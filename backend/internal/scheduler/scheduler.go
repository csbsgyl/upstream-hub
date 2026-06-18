// Package scheduler 用 robfig/cron 触发周期性扫描。
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/worryzyy/upstream-hub/internal/config"
	"github.com/worryzyy/upstream-hub/internal/monitor"
	"github.com/worryzyy/upstream-hub/internal/storage"
)

type Scheduler struct {
	cfg      config.SchedulerConfig
	log      *slog.Logger
	cron     *cron.Cron
	monitor  *monitor.Service
	monLogs  *storage.MonitorLogs
	rates    *storage.Rates
	notifies *storage.Notifications
}

func New(
	cfg config.SchedulerConfig,
	m *monitor.Service,
	monLogs *storage.MonitorLogs,
	rates *storage.Rates,
	notifies *storage.Notifications,
	log *slog.Logger,
) *Scheduler {
	return &Scheduler{
		cfg:      cfg,
		log:      log,
		cron:     cron.New(cron.WithSeconds()),
		monitor:  m,
		monLogs:  monLogs,
		rates:    rates,
		notifies: notifies,
	}
}

func (s *Scheduler) Start() error {
	if s.cfg.SyncCron != "" {
		if _, err := s.cron.AddFunc(s.cfg.SyncCron, s.runSync); err != nil {
			return err
		}
	}
	if s.cfg.BalanceCron != "" {
		if _, err := s.cron.AddFunc(s.cfg.BalanceCron, s.runBalance); err != nil {
			return err
		}
	}
	if s.cfg.RateCron != "" {
		if _, err := s.cron.AddFunc(s.cfg.RateCron, s.runRates); err != nil {
			return err
		}
	}
	if s.cfg.Retention.Cron != "" && s.hasRetention() {
		if _, err := s.cron.AddFunc(s.cfg.Retention.Cron, s.runRetention); err != nil {
			return err
		}
	}
	s.cron.Start()
	s.log.Info("scheduler started",
		"syncCron", s.cfg.SyncCron,
		"balanceCron", s.cfg.BalanceCron,
		"rateCron", s.cfg.RateCron,
		"retentionCron", s.cfg.Retention.Cron,
		"concurrency", s.cfg.Concurrency,
	)
	return nil
}

func (s *Scheduler) Stop() {
	if s.cron != nil {
		<-s.cron.Stop().Done()
	}
}

func (s *Scheduler) runSync() {
	s.runScan("sync", func(ctx context.Context) {
		s.monitor.ScanAllSyncConcurrent(ctx, s.cfg.Concurrency)
	})
}

func (s *Scheduler) runBalance() {
	s.runScan("balance", func(ctx context.Context) {
		s.monitor.ScanAllBalancesConcurrent(ctx, s.cfg.Concurrency)
	})
}

func (s *Scheduler) runRates() {
	s.runScan("rates", func(ctx context.Context) {
		s.monitor.ScanAllRatesConcurrent(ctx, s.cfg.Concurrency)
	})
}

func (s *Scheduler) runScan(job string, run func(context.Context)) {
	release, ok := s.monitor.TryBeginScan(job)
	if !ok {
		return
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	run(ctx)
}

func (s *Scheduler) hasRetention() bool {
	r := s.cfg.Retention
	return r.MonitorLogsDays > 0 || r.BalanceSnapshotsDays > 0 || r.NotificationLogsDays > 0
}

// runRetention 按配置删除过期历史。任一表失败不影响其它，全部错误写日志。
func (s *Scheduler) runRetention() {
	r := s.cfg.Retention
	now := time.Now()

	if r.MonitorLogsDays > 0 {
		cutoff := now.AddDate(0, 0, -r.MonitorLogsDays)
		n, err := s.monLogs.DeleteBefore(cutoff)
		if err != nil {
			s.log.Warn("retention monitor_logs failed", "err", err)
		} else if n > 0 {
			s.log.Info("retention monitor_logs deleted", "rows", n, "before", cutoff)
		}
	}

	if r.BalanceSnapshotsDays > 0 {
		cutoff := now.AddDate(0, 0, -r.BalanceSnapshotsDays)
		n, err := s.rates.DeleteBalanceSnapshotsBefore(cutoff)
		if err != nil {
			s.log.Warn("retention balance_snapshots failed", "err", err)
		} else if n > 0 {
			s.log.Info("retention balance_snapshots deleted", "rows", n, "before", cutoff)
		}
	}

	if r.NotificationLogsDays > 0 {
		cutoff := now.AddDate(0, 0, -r.NotificationLogsDays)
		n, err := s.notifies.DeleteLogsBefore(cutoff)
		if err != nil {
			s.log.Warn("retention notification_logs failed", "err", err)
		} else if n > 0 {
			s.log.Info("retention notification_logs deleted", "rows", n, "before", cutoff)
		}
	}
}
