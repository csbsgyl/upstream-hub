// Package monitor 周期性扫描渠道，采集余额 / 倍率并写入快照、变化日志和通知。
package monitor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/worryzyy/upstream-hub/internal/channel"
	"github.com/worryzyy/upstream-hub/internal/connector"
	"github.com/worryzyy/upstream-hub/internal/notify"
	"github.com/worryzyy/upstream-hub/internal/progress"
	"github.com/worryzyy/upstream-hub/internal/storage"
)

// Service 监控扫描服务。
type Service struct {
	channels    *storage.Channels
	rates       *storage.Rates
	monitorLogs *storage.MonitorLogs
	channelSvc  *channel.Service
	dispatcher  *notify.Dispatcher
	log         *slog.Logger
	scanMu      sync.Mutex
}

func NewService(
	channels *storage.Channels,
	rates *storage.Rates,
	monitorLogs *storage.MonitorLogs,
	channelSvc *channel.Service,
	dispatcher *notify.Dispatcher,
	log *slog.Logger,
) *Service {
	return &Service{
		channels:    channels,
		rates:       rates,
		monitorLogs: monitorLogs,
		channelSvc:  channelSvc,
		dispatcher:  dispatcher,
		log:         log,
	}
}

type RefreshAllError struct {
	BalanceErr error
	RatesErr   error
}

func (e *RefreshAllError) Error() string {
	switch {
	case e == nil:
		return ""
	case e.BalanceErr != nil && e.RatesErr != nil:
		return e.BalanceErr.Error() + " | " + e.RatesErr.Error()
	case e.BalanceErr != nil:
		return e.BalanceErr.Error()
	case e.RatesErr != nil:
		return e.RatesErr.Error()
	default:
		return ""
	}
}

func (e *RefreshAllError) Unwrap() []error {
	if e == nil {
		return nil
	}
	errs := make([]error, 0, 2)
	if e.BalanceErr != nil {
		errs = append(errs, e.BalanceErr)
	}
	if e.RatesErr != nil {
		errs = append(errs, e.RatesErr)
	}
	return errs
}

func SplitRefreshAllError(err error) (balanceErr error, ratesErr error, ok bool) {
	var refreshErr *RefreshAllError
	if !errors.As(err, &refreshErr) || refreshErr == nil {
		return nil, nil, false
	}
	return refreshErr.BalanceErr, refreshErr.RatesErr, true
}

// TryBeginScan starts a global monitor scan section. Callers that kick off
// scheduler/manual scans use this shared guard to avoid overlapping refreshes
// and duplicate notifications.
func (s *Service) TryBeginScan(job string) (func(), bool) {
	if !s.scanMu.TryLock() {
		if s.log != nil {
			s.log.Warn("skip scan because another scan is still running", "job", job)
		}
		return nil, false
	}
	return func() {
		s.scanMu.Unlock()
	}, true
}

// ScanAllBalances 扫描所有启用监控的渠道余额。单个失败不影响其他。
func (s *Service) ScanAllBalances(ctx context.Context) {
	s.ScanAllBalancesConcurrent(ctx, 1)
}

// ScanAllBalancesConcurrent 按并发上限扫描所有启用监控的渠道余额。
func (s *Service) ScanAllBalancesConcurrent(ctx context.Context, concurrency int) {
	s.scanAll(ctx, concurrency, "balance", func(c *storage.Channel) error {
		return s.RefreshBalance(ctx, c)
	})
}

func (s *Service) scanAll(ctx context.Context, concurrency int, job string, run func(*storage.Channel) error) {
	list, err := s.channels.ListMonitorEnabled()
	if err != nil {
		s.log.Error("list channels", "err", err)
		return
	}
	if len(list) == 0 {
		s.log.Info("no monitor-enabled channels", "job", job)
		return
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(list) {
		concurrency = len(list)
	}

	jobs := make(chan storage.Channel)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range jobs {
				if err := run(&c); err != nil {
					s.log.Warn("refresh failed", "job", job, "channel", c.Name, "err", err)
				}
			}
		}()
	}
sendLoop:
	for i := range list {
		select {
		case <-ctx.Done():
			break sendLoop
		case jobs <- list[i]:
		}
	}
	close(jobs)
	wg.Wait()
}

// ScanAllSync 同一轮里同步扫描余额和倍率，复用一次登录会话。
// 适合默认调度路径，减少重复登录和重复失败通知。
func (s *Service) ScanAllSync(ctx context.Context) {
	s.ScanAllSyncConcurrent(ctx, 1)
}

// ScanAllSyncConcurrent 按并发上限同一轮同步余额和倍率。
func (s *Service) ScanAllSyncConcurrent(ctx context.Context, concurrency int) {
	s.scanAll(ctx, concurrency, "sync", func(c *storage.Channel) error {
		return s.RefreshAll(ctx, c)
	})
}

// ScanAllRates 扫描所有启用监控的渠道倍率。
func (s *Service) ScanAllRates(ctx context.Context) {
	s.ScanAllRatesConcurrent(ctx, 1)
}

// ScanAllRatesConcurrent 按并发上限扫描所有启用监控的渠道倍率。
func (s *Service) ScanAllRatesConcurrent(ctx context.Context, concurrency int) {
	s.scanAll(ctx, concurrency, "rates", func(c *storage.Channel) error {
		return s.RefreshRates(ctx, c)
	})
}

// RefreshAll 先刷新余额，再刷新倍率，并复用同一个登录会话。
// 单个渠道的任一步失败只影响该渠道，不影响其他渠道。
func (s *Service) RefreshAll(ctx context.Context, c *storage.Channel) error {
	resolved, conn, session, err := s.prepare(ctx, c)
	if err != nil {
		s.notifyError(ctx, c, storage.EventLoginFailed, "登录失败", err)
		return err
	}
	balanceErr := s.refreshBalanceWithSession(ctx, c, resolved, conn, session)
	ratesErr := s.refreshRatesWithSession(ctx, c, resolved, conn, session)
	if balanceErr == nil && ratesErr == nil {
		return nil
	}
	return &RefreshAllError{BalanceErr: balanceErr, RatesErr: ratesErr}
}

// RefreshBalance 单个渠道余额刷新，可被 API 手动触发。
func (s *Service) RefreshBalance(ctx context.Context, c *storage.Channel) error {
	resolved, conn, session, err := s.prepare(ctx, c)
	if err != nil {
		s.notifyError(ctx, c, storage.EventLoginFailed, "登录失败", err)
		return err
	}
	return s.refreshBalanceWithSession(ctx, c, resolved, conn, session)
}

func (s *Service) refreshBalanceWithSession(ctx context.Context, c *storage.Channel, resolved *connector.Channel, conn connector.Connector, session *connector.AuthSession) error {
	progress.Start(ctx, progress.StageBalance, "拉取余额…")
	started := time.Now()
	res, err := conn.GetBalance(ctx, resolved, session)
	finished := time.Now()
	_ = s.monitorLogs.Append(&storage.MonitorLog{
		ChannelID:    c.ID,
		Job:          storage.MonitorJobBalance,
		Success:      err == nil,
		ErrorMessage: errString(err),
		StartedAt:    started,
		FinishedAt:   finished,
	})
	if err != nil {
		progress.Fail(ctx, progress.StageBalance, err.Error())
		s.notifyError(ctx, c, storage.EventMonitorFailed, "余额采集失败", err)
		return err
	}

	sampledAt := res.SampledAt
	if sampledAt.IsZero() {
		sampledAt = time.Now()
	}
	if err := s.channels.UpdateBalance(c.ID, res.Balance, &sampledAt, ""); err != nil {
		return err
	}
	_ = s.rates.AppendBalance(&storage.BalanceSnapshot{
		ChannelID: c.ID,
		Balance:   res.Balance,
		SampledAt: sampledAt,
	})
	progress.OK(ctx, progress.StageBalance, fmt.Sprintf("当前余额 %.4f", res.Balance),
		map[string]any{"balance": res.Balance})

	if c.BalanceThreshold > 0 && res.Balance < c.BalanceThreshold {
		_ = s.dispatcher.Dispatch(ctx, notify.BuildBalanceLowMessage(c, res.Balance, c.BalanceThreshold, sampledAt))
	}
	return nil
}

// RefreshRates 单个渠道倍率刷新，可被 API 手动触发。
func (s *Service) RefreshRates(ctx context.Context, c *storage.Channel) error {
	resolved, conn, session, err := s.prepare(ctx, c)
	if err != nil {
		s.notifyError(ctx, c, storage.EventLoginFailed, "登录失败", err)
		return err
	}
	return s.refreshRatesWithSession(ctx, c, resolved, conn, session)
}

func (s *Service) refreshRatesWithSession(ctx context.Context, c *storage.Channel, resolved *connector.Channel, conn connector.Connector, session *connector.AuthSession) error {
	progress.Start(ctx, progress.StageRates, "拉取分组倍率…")
	started := time.Now()
	results, err := conn.GetRates(ctx, resolved, session)
	finished := time.Now()
	_ = s.monitorLogs.Append(&storage.MonitorLog{
		ChannelID:    c.ID,
		Job:          storage.MonitorJobRates,
		Success:      err == nil,
		ErrorMessage: errString(err),
		StartedAt:    started,
		FinishedAt:   finished,
	})
	if err != nil {
		progress.Fail(ctx, progress.StageRates, err.Error())
		s.notifyError(ctx, c, storage.EventMonitorFailed, "倍率采集失败", err)
		return err
	}

	now := time.Now()
	keepModelNames := make([]string, 0, len(results))
	changes := make([]notify.RateChange, 0, len(results))
	for _, r := range results {
		keepModelNames = append(keepModelNames, r.ModelName)
		prev, err := s.rates.Upsert(&storage.RateSnapshot{
			ChannelID:       c.ID,
			ModelName:       r.ModelName,
			Description:     r.Description,
			Ratio:           r.Ratio,
			CompletionRatio: r.CompletionRatio,
			LastSeenAt:      now,
		})
		if err != nil {
			s.log.Warn("rate upsert failed", "channel", c.Name, "model", r.ModelName, "err", err)
			continue
		}
		if prev == nil {
			continue
		}
		if prev.Ratio == r.Ratio && prev.CompletionRatio == r.CompletionRatio {
			continue
		}
		oldRatio := prev.Ratio
		oldComp := prev.CompletionRatio
		_ = s.rates.AppendChange(&storage.RateChangeLog{
			ChannelID:          c.ID,
			ModelName:          r.ModelName,
			OldRatio:           &oldRatio,
			NewRatio:           r.Ratio,
			OldCompletionRatio: &oldComp,
			NewCompletionRatio: r.CompletionRatio,
			ChangedAt:          now,
		})
		changes = append(changes, notify.RateChange{
			GroupName: r.ModelName,
			OldRatio:  oldRatio,
			NewRatio:  r.Ratio,
			OldComp:   oldComp,
			NewComp:   r.CompletionRatio,
			ChangedAt: now,
		})
	}
	// 一次扫描的所有变化打包推送：去抖策略（合并 / 涨跌幅过滤）由 Dispatcher.Policy 决定。
	if _, err := s.rates.DeleteRateSnapshotsNotIn(c.ID, keepModelNames); err != nil {
		progress.Fail(ctx, progress.StageRates, err.Error())
		if s.log != nil {
			s.log.Warn("rate stale snapshot cleanup failed", "channel", c.Name, "err", err)
		}
		return err
	}
	if len(changes) > 0 {
		if err := s.dispatcher.DispatchRateBatch(ctx, c, changes); err != nil && s.log != nil {
			s.log.Warn("dispatch rate changes failed", "channel", c.Name, "err", err)
		}
	}
	progress.OK(ctx, progress.StageRates, fmt.Sprintf("拉到 %d 个分组", len(results)),
		map[string]any{"count": len(results)})
	return nil
}

func (s *Service) prepare(ctx context.Context, c *storage.Channel) (*connector.Channel, connector.Connector, *connector.AuthSession, error) {
	resolved, err := s.channelSvc.Resolve(ctx, c)
	if err != nil {
		return nil, nil, nil, err
	}
	conn, err := connector.For(resolved.Type)
	if err != nil {
		return nil, nil, nil, err
	}
	session, err := s.channelSvc.EnsureSession(ctx, c, resolved, conn)
	if err != nil {
		return nil, nil, nil, err
	}
	return resolved, conn, session, nil
}

func (s *Service) notifyError(ctx context.Context, c *storage.Channel, event storage.NotificationEvent, subject string, err error) {
	_ = s.dispatcher.Dispatch(ctx, notify.BuildFailureMessage(c, event, subject, err))
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
