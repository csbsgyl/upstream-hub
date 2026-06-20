package storage

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Rates struct{ db *gorm.DB }

func NewRates(db *gorm.DB) *Rates { return &Rates{db: db} }

// ListByChannel 返回渠道当前所有倍率快照。
func (r *Rates) ListByChannel(channelID uint) ([]RateSnapshot, error) {
	var list []RateSnapshot
	if err := r.db.Where("channel_id = ?", channelID).Order("model_name ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Upsert 更新或插入倍率快照，返回此前的记录（若有），调用方据此判断是否变化。
func (r *Rates) Upsert(snapshot *RateSnapshot) (*RateSnapshot, error) {
	var prev RateSnapshot
	err := r.db.
		Where("channel_id = ? AND model_name = ?", snapshot.ChannelID, snapshot.ModelName).
		First(&prev).Error
	switch {
	case err == nil:
		old := prev
		prev.Ratio = snapshot.Ratio
		prev.CompletionRatio = snapshot.CompletionRatio
		prev.Description = snapshot.Description
		prev.LastSeenAt = snapshot.LastSeenAt
		if err := r.db.Save(&prev).Error; err != nil {
			return nil, err
		}
		return &old, nil
	case err == gorm.ErrRecordNotFound:
		snapshot.FirstSeenAt = snapshot.LastSeenAt
		if err := r.db.Create(snapshot).Error; err != nil {
			return nil, err
		}
		return nil, nil
	default:
		return nil, err
	}
}

func (r *Rates) AppendChange(log *RateChangeLog) error {
	if log.ChangedAt.IsZero() {
		log.ChangedAt = time.Now()
	}
	return r.db.Create(log).Error
}

// ListChanges 倒序拉取倍率变化日志。channelID 为 0 表示不过滤。
func (r *Rates) ListChanges(channelID uint, limit int) ([]RateChangeLog, error) {
	if limit <= 0 {
		limit = 50
	}
	q := r.db.Model(&RateChangeLog{}).Order("changed_at DESC").Limit(limit)
	if channelID != 0 {
		q = q.Where("channel_id = ?", channelID)
	}
	var list []RateChangeLog
	if err := q.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *Rates) CountChanges() (int64, error) {
	var n int64
	err := r.db.Model(&RateChangeLog{}).Count(&n).Error
	return n, err
}

func (r *Rates) CountSnapshots() (int64, error) {
	var n int64
	err := r.db.Model(&RateSnapshot{}).Count(&n).Error
	return n, err
}

func (r *Rates) DeleteRateSnapshotsNotIn(channelID uint, keepModelNames []string) (int64, error) {
	var existing []RateSnapshot
	if err := r.db.Select("model_name").Where("channel_id = ?", channelID).Find(&existing).Error; err != nil {
		return 0, err
	}
	stale := staleRateSnapshotModelNames(existing, keepModelNames)
	if len(stale) == 0 {
		return 0, nil
	}
	res := r.db.Where("channel_id = ? AND model_name IN ?", channelID, stale).Delete(&RateSnapshot{})
	return res.RowsAffected, res.Error
}

func staleRateSnapshotModelNames(existing []RateSnapshot, keepModelNames []string) []string {
	keep := make(map[string]struct{}, len(keepModelNames))
	for _, name := range keepModelNames {
		keep[name] = struct{}{}
	}
	stale := make([]string, 0, len(existing))
	seen := make(map[string]struct{}, len(existing))
	for _, snapshot := range existing {
		if _, ok := keep[snapshot.ModelName]; ok {
			continue
		}
		if _, ok := seen[snapshot.ModelName]; ok {
			continue
		}
		seen[snapshot.ModelName] = struct{}{}
		stale = append(stale, snapshot.ModelName)
	}
	return stale
}

func (r *Rates) AppendBalance(s *BalanceSnapshot) error {
	if s.SampledAt.IsZero() {
		s.SampledAt = time.Now()
	}
	return r.db.Create(s).Error
}

// DeleteBalanceSnapshotsBefore 删除 sampled_at < cutoff 的余额快照，返回删除行数。
func (r *Rates) DeleteBalanceSnapshotsBefore(cutoff time.Time) (int64, error) {
	res := r.db.Where("sampled_at < ?", cutoff).Delete(&BalanceSnapshot{})
	return res.RowsAffected, res.Error
}

// BalanceHistory 倒序拉取余额历史。
func (r *Rates) BalanceHistory(channelID uint, limit int) ([]BalanceSnapshot, error) {
	if limit <= 0 {
		limit = 100
	}
	var list []BalanceSnapshot
	if err := r.db.
		Where("channel_id = ?", channelID).
		Order("sampled_at DESC").
		Limit(limit).
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// BalanceTrendPoint 是首页余额趋势的聚合点。
//
// At 是当前前端使用的采样桶时间；Day 保留给旧前端缓存兼容。
type BalanceTrendPoint struct {
	At      time.Time `json:"at"`
	Day     time.Time `json:"day"`
	Balance float64   `json:"balance"`
}

type BalanceTrendSpec struct {
	Range      string
	Since      time.Time
	BucketExpr string
}

func BalanceTrendSpecForRange(raw string) (BalanceTrendSpec, bool) {
	return balanceTrendSpecForRange(raw, time.Now())
}

func balanceTrendSpecForRange(raw string, now time.Time) (BalanceTrendSpec, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "24h":
		return BalanceTrendSpec{
			Range:      "24h",
			Since:      now.Add(-24 * time.Hour).Truncate(3 * time.Minute),
			BucketExpr: "date_bin(interval '3 minutes', sampled_at, '2000-01-01 00:00:00+00'::timestamptz)",
		}, true
	case "7d":
		return BalanceTrendSpec{
			Range:      "7d",
			Since:      startOfDay(now).AddDate(0, 0, -6),
			BucketExpr: "date_trunc('hour', sampled_at)",
		}, true
	case "30d":
		return BalanceTrendSpec{
			Range:      "30d",
			Since:      startOfDay(now).AddDate(0, 0, -29),
			BucketExpr: "date_trunc('day', sampled_at)",
		}, true
	default:
		return BalanceTrendSpec{}, false
	}
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// AggregateBalanceTrend 按时间范围聚合总余额趋势。
//
// 24h 使用 3 分钟桶对齐后台默认同步频率；7d 使用小时桶；30d 使用天桶。
// 每个桶内先取单个渠道最后一次余额，再把所有渠道相加，避免同一渠道在同一桶内重复计入。
func (r *Rates) AggregateBalanceTrend(spec BalanceTrendSpec) ([]BalanceTrendPoint, error) {
	type row struct {
		At      time.Time
		Balance float64
	}
	var rows []row
	query := fmt.Sprintf(`
		WITH sampled AS (
			SELECT id, channel_id, sampled_at, balance, %[1]s AS bucket
			FROM balance_snapshots
			WHERE sampled_at >= ?
		),
		latest AS (
			SELECT DISTINCT ON (channel_id, bucket) channel_id, bucket, balance
			FROM sampled
			ORDER BY channel_id, bucket, sampled_at DESC, id DESC
		)
		SELECT bucket AS at, SUM(balance) AS balance
		FROM latest
		GROUP BY bucket
		ORDER BY bucket ASC
	`, spec.BucketExpr)
	if err := r.db.Raw(query, spec.Since).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]BalanceTrendPoint, 0, len(rows))
	for _, row := range rows {
		out = append(out, BalanceTrendPoint{At: row.At, Day: row.At, Balance: row.Balance})
	}
	return out, nil
}
