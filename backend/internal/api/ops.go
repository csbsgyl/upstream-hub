package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func registerOps(g *gin.RouterGroup, d *Deps) {
	g.GET("/ops/status", func(c *gin.Context) { opsStatus(c, d) })
	g.GET("/ops/diagnostics", func(c *gin.Context) { diagnostics(c, d) })
	g.GET("/ops/audit-logs", func(c *gin.Context) { auditLogs(c, d) })
}

type backupState struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	UpdatedAt string `json:"updated_at"`
}

type opsStatusResponse struct {
	Database           string           `json:"database"`
	AuthEnabled        bool             `json:"auth_enabled"`
	AppSecretReady     bool             `json:"app_secret_ready"`
	Scheduler          map[string]any   `json:"scheduler"`
	Notifications      map[string]any   `json:"notifications"`
	Channels           map[string]any   `json:"channels"`
	Captchas           map[string]any   `json:"captchas"`
	Backups            []backupState    `json:"backups"`
	RecentAuditLogs    []map[string]any `json:"recent_audit_logs"`
	RecentMonitorLogs  []map[string]any `json:"recent_monitor_logs"`
	RecentRateChanges  []map[string]any `json:"recent_rate_changes"`
	RecentNotification []map[string]any `json:"recent_notification_logs"`
	GeneratedAt        time.Time        `json:"generated_at"`
}

func opsStatus(c *gin.Context, d *Deps) {
	resp, err := buildOpsStatus(d)
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func buildOpsStatus(d *Deps) (*opsStatusResponse, error) {
	if d == nil || d.DB == nil {
		return nil, errors.New("database is unavailable")
	}
	sqlDB, err := d.DB.DB()
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}

	channelsCount, _ := d.Channels.Count()
	channelsEnabled, _ := d.Channels.CountMonitorEnabled()
	notifyCount, _ := d.Notifies.CountChannels()
	notifyEnabled, _ := d.Notifies.CountEnabledChannels()
	notifyFailed, _ := d.Notifies.CountFailedLogs()
	captchaCount, _ := d.Captchas.Count()
	captchaEnabled, _ := d.Captchas.CountEnabled()
	rateChanges, _ := d.Rates.CountChanges()
	rateSnapshots, _ := d.Rates.CountSnapshots()
	monitorFailures, _ := d.MonLogs.CountFailures()
	audits, _ := d.AuditLogs.List(10)
	monLogs, _ := d.MonLogs.List(0, 10)
	rateLogs, _ := d.Rates.ListChanges(0, 10)
	notifLogs, _ := d.Notifies.ListLogs(10)

	status := &opsStatusResponse{
		Database:       "ok",
		AuthEnabled:    d.Auth != nil,
		AppSecretReady: strings.TrimSpace(d.Config.Security.AppSecret) != "",
		Scheduler: map[string]any{
			"balance_cron": d.Config.Scheduler.BalanceCron,
			"rate_cron":    d.Config.Scheduler.RateCron,
			"retention":    d.Config.Scheduler.Retention,
			"concurrency":  d.Config.Scheduler.Concurrency,
		},
		Notifications: map[string]any{
			"total":                        notifyCount,
			"enabled":                      notifyEnabled,
			"batch_rate_changes":           d.Config.Notifications.BatchRateChanges,
			"min_change_pct":               d.Config.Notifications.MinChangePct,
			"rate_change_direction":        d.Config.Notifications.RateChangeDirection,
			"rate_change_quiet_groups":     d.Config.Notifications.RateChangeQuietGroups,
			"balance_low_cooldown_minutes": d.Config.Notifications.BalanceLowCooldownMinutes,
			"send_max_attempts":            d.Config.Notifications.SendMaxAttempts,
			"failed_notification_logs":     notifyFailed,
		},
		Channels: map[string]any{
			"total":           channelsCount,
			"monitor_enabled": channelsEnabled,
			"failed":          monitorFailures,
			"rate_changes":    rateChanges,
			"rate_snapshots":  rateSnapshots,
		},
		Captchas: map[string]any{
			"total":   captchaCount,
			"enabled": captchaEnabled,
		},
		Backups:            listBackups(),
		RecentAuditLogs:    toMapSlice(audits),
		RecentMonitorLogs:  toMapSlice(monLogs),
		RecentRateChanges:  toMapSlice(rateLogs),
		RecentNotification: toMapSlice(notifLogs),
		GeneratedAt:        time.Now(),
	}
	return status, nil
}

func auditLogs(c *gin.Context, d *Deps) {
	limit := queryIntClamped(c, "limit", 100, 1, 500)
	list, err := d.AuditLogs.List(limit)
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

func diagnostics(c *gin.Context, d *Deps) {
	status, err := buildOpsStatus(d)
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	sqlDB, _ := d.DB.DB()
	info := gin.H{
		"generated_at": time.Now(),
		"database": gin.H{
			"ping": dbPing(sqlDB),
		},
		"config": gin.H{
			"server": gin.H{
				"port":            d.Config.Server.Port,
				"mode":            d.Config.Server.Mode,
				"trusted_proxies": d.Config.Server.TrustedProxies,
				"base_url":        d.Config.Server.BaseURL,
			},
			"database": gin.H{
				"host":           d.Config.Database.Host,
				"port":           d.Config.Database.Port,
				"user":           d.Config.Database.User,
				"name":           d.Config.Database.Name,
				"ssl_mode":       d.Config.Database.SSLMode,
				"timezone":       d.Config.Database.Timezone,
				"max_open_conns": d.Config.Database.MaxOpenConns,
				"max_idle_conns": d.Config.Database.MaxIdleConns,
			},
			"auth": gin.H{
				"enabled":           d.Config.Auth.Enabled,
				"username":          d.Config.Auth.Username,
				"session_ttl_hours": d.Config.Auth.SessionTTLHours,
			},
			"scheduler":     d.Config.Scheduler,
			"notifications": d.Config.Notifications,
		},
		"summary": status,
		"counts": gin.H{
			"channels":      status.Channels,
			"captcha":       status.Captchas,
			"notifications": status.Notifications,
			"audit_logs":    len(status.RecentAuditLogs),
			"monitor_logs":  len(status.RecentMonitorLogs),
			"rate_changes":  len(status.RecentRateChanges),
		},
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=upstream-hub-diagnostics-%s.json", time.Now().Format("20060102-150405")))
	c.JSON(http.StatusOK, info)
}

func dbPing(sqlDB interface{ Ping() error }) string {
	if sqlDB == nil {
		return "unknown"
	}
	if err := sqlDB.Ping(); err != nil {
		return err.Error()
	}
	return "ok"
}

func listBackups() []backupState {
	dir := backupDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []backupState
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".sql.gz") && !strings.HasSuffix(strings.ToLower(e.Name()), ".json") && !strings.HasSuffix(strings.ToLower(e.Name()), ".env") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, backupState{
			Path:      filepath.Join(dir, e.Name()),
			Name:      e.Name(),
			Size:      info.Size(),
			UpdatedAt: info.ModTime().Format(time.RFC3339),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out
}

func backupDir() string {
	if v := strings.TrimSpace(os.Getenv("UPSTREAMHUB_BACKUP_DIR")); v != "" {
		return v
	}
	return filepath.Join(".", "backups")
}

func toMapSlice[T any](items []T) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		b, err := json.Marshal(item)
		if err != nil {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			continue
		}
		out = append(out, m)
	}
	return out
}
