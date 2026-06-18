package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/worryzyy/upstream-hub/internal/config"
)

func registerOps(g *gin.RouterGroup, d *Deps) {
	g.GET("/ops/status", func(c *gin.Context) { opsStatus(c, d) })
	g.GET("/ops/diagnostics", func(c *gin.Context) { diagnostics(c, d) })
	g.GET("/ops/audit-logs", func(c *gin.Context) { auditLogs(c, d) })
	g.POST("/ops/backups", func(c *gin.Context) { createBackup(c, d) })
	g.GET("/ops/backups/:name/download", func(c *gin.Context) { downloadBackup(c, d) })
	g.POST("/ops/retention/run", func(c *gin.Context) { runRetentionNow(c, d) })
	g.POST("/ops/scan/sync", func(c *gin.Context) { startOpsScan(c, d, "sync") })
	g.POST("/ops/scan/balances", func(c *gin.Context) { startOpsScan(c, d, "balances") })
	g.POST("/ops/scan/rates", func(c *gin.Context) { startOpsScan(c, d, "rates") })
}

type backupState struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
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

type retentionRunResponse struct {
	MonitorLogsDeleted      int64     `json:"monitor_logs_deleted"`
	BalanceSnapshotsDeleted int64     `json:"balance_snapshots_deleted"`
	NotificationLogsDeleted int64     `json:"notification_logs_deleted"`
	MonitorLogsDays         int       `json:"monitor_logs_days"`
	BalanceSnapshotsDays    int       `json:"balance_snapshots_days"`
	NotificationLogsDays    int       `json:"notification_logs_days"`
	RanAt                   time.Time `json:"ran_at"`
}

type opsScanResponse struct {
	OK        bool      `json:"ok"`
	Started   bool      `json:"started"`
	Job       string    `json:"job"`
	Channels  int64     `json:"channels"`
	Message   string    `json:"message"`
	StartedAt time.Time `json:"started_at"`
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
			"sync_cron":    d.Config.Scheduler.SyncCron,
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
			"failure_cooldown_minutes":     d.Config.Notifications.FailureCooldownMinutes,
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

func createBackup(c *gin.Context, d *Deps) {
	backup, err := writeDatabaseBackup(c.Request.Context(), d)
	if err != nil {
		audit(c, d, "ops.backup", "ops", 0, "created database backup", gin.H{
			"ok":    false,
			"error": err.Error(),
		})
		fail(c, http.StatusInternalServerError, err)
		return
	}
	audit(c, d, "ops.backup", "ops", 0, "created database backup "+backup.Name, gin.H{
		"ok":     true,
		"name":   backup.Name,
		"source": backup.Source,
		"size":   backup.Size,
	})
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"backup":  backup,
			"backups": listBackups(),
		},
	})
}

func downloadBackup(c *gin.Context, d *Deps) {
	name, err := safeBackupName(c.Param("name"))
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	path := filepath.Join(backupDir(), name)
	info, err := os.Stat(path)
	if err != nil {
		fail(c, http.StatusNotFound, errors.New("backup file not found"))
		return
	}
	if info.IsDir() {
		fail(c, http.StatusNotFound, errors.New("backup file not found"))
		return
	}
	audit(c, d, "ops.backup_download", "ops", 0, "downloaded database backup "+name, gin.H{
		"name": name,
		"size": info.Size(),
	})
	c.FileAttachment(path, name)
}

func runRetentionNow(c *gin.Context, d *Deps) {
	res, err := applyRetention(d)
	if err != nil {
		audit(c, d, "ops.retention", "ops", 0, "ran retention cleanup", gin.H{
			"ok":    false,
			"error": err.Error(),
		})
		fail(c, http.StatusInternalServerError, err)
		return
	}
	audit(c, d, "ops.retention", "ops", 0, "ran retention cleanup", gin.H{
		"ok":                        true,
		"monitor_logs_deleted":      res.MonitorLogsDeleted,
		"balance_snapshots_deleted": res.BalanceSnapshotsDeleted,
		"notification_logs_deleted": res.NotificationLogsDeleted,
		"monitor_logs_days":         res.MonitorLogsDays,
		"balance_snapshots_days":    res.BalanceSnapshotsDays,
		"notification_logs_days":    res.NotificationLogsDays,
	})
	c.JSON(http.StatusOK, gin.H{"data": res})
}

func startOpsScan(c *gin.Context, d *Deps, job string) {
	if d == nil || d.Monitor == nil {
		fail(c, http.StatusInternalServerError, errors.New("monitor service is unavailable"))
		return
	}
	enabled, err := d.Channels.CountMonitorEnabled()
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	if enabled == 0 {
		c.JSON(http.StatusOK, gin.H{"data": opsScanResponse{
			OK:        false,
			Started:   false,
			Job:       job,
			Channels:  enabled,
			Message:   "没有启用监控的渠道",
			StartedAt: time.Now(),
		}})
		return
	}

	startedAt := time.Now()
	releaseScan, ok := d.Monitor.TryBeginScan("ops." + job)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"data": opsScanResponse{
			OK:        false,
			Started:   false,
			Job:       job,
			Channels:  enabled,
			Message:   "这个任务已经在运行",
			StartedAt: startedAt,
		}})
		return
	}

	action := "ops.scan_balances"
	summary := "started balance scan"
	if job == "sync" {
		action = "ops.scan_sync"
		summary = "started sync scan"
	} else if job == "rates" {
		action = "ops.scan_rates"
		summary = "started rate scan"
	}
	audit(c, d, action, "ops", 0, summary, gin.H{
		"ok":       true,
		"channels": enabled,
	})

	go func() {
		defer releaseScan()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		switch job {
		case "sync":
			d.Monitor.ScanAllSyncConcurrent(ctx, d.Config.Scheduler.Concurrency)
		case "balances":
			d.Monitor.ScanAllBalancesConcurrent(ctx, d.Config.Scheduler.Concurrency)
		case "rates":
			d.Monitor.ScanAllRatesConcurrent(ctx, d.Config.Scheduler.Concurrency)
		}
	}()

	message := "已开始后台扫描"
	if job == "sync" {
		message = "已开始后台同步余额和倍率"
	} else if job == "balances" {
		message = "已开始后台扫描余额"
	} else if job == "rates" {
		message = "已开始后台扫描倍率"
	}
	c.JSON(http.StatusAccepted, gin.H{"data": opsScanResponse{
		OK:        true,
		Started:   true,
		Job:       job,
		Channels:  enabled,
		Message:   message,
		StartedAt: startedAt,
	}})
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

func writeDatabaseBackup(ctx context.Context, d *Deps) (backupState, error) {
	if d == nil || d.Config == nil {
		return backupState{}, errors.New("config is unavailable")
	}
	db := d.Config.Database
	if strings.TrimSpace(db.Host) == "" || strings.TrimSpace(db.User) == "" || strings.TrimSpace(db.Name) == "" {
		return backupState{}, errors.New("database connection config is incomplete")
	}
	pgDump, err := exec.LookPath("pg_dump")
	if err != nil {
		return backupState{}, errors.New("pg_dump is not available in the app runtime; rebuild the image with the PostgreSQL client installed")
	}

	dir := backupDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return backupState{}, fmt.Errorf("create backup directory: %w", err)
	}

	now := time.Now()
	name := fmt.Sprintf("upstream-hub-%s.sql.gz", now.Format("20060102-150405"))
	finalPath := filepath.Join(dir, name)
	tmpPath := finalPath + ".tmp"
	_ = os.Remove(tmpPath)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	args := []string{"--dbname", dbDSN(db), "--no-owner", "--no-privileges"}
	cmd := exec.CommandContext(ctx, pgDump, args...)
	cmd.Env = os.Environ()
	if db.Password != "" {
		cmd.Env = append(cmd.Env, "PGPASSWORD="+db.Password)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return backupState{}, err
	}

	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return backupState{}, fmt.Errorf("create backup file: %w", err)
	}
	cleanup := true
	defer func() {
		_ = file.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	gz := gzip.NewWriter(file)
	if err := cmd.Start(); err != nil {
		_ = gz.Close()
		return backupState{}, fmt.Errorf("start pg_dump: %w", err)
	}
	copyErr := copyAndClose(gz, stdout)
	fileErr := file.Close()
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return backupState{}, ctx.Err()
	}
	if copyErr != nil {
		return backupState{}, fmt.Errorf("write backup stream: %w", copyErr)
	}
	if fileErr != nil {
		return backupState{}, fmt.Errorf("close backup file: %w", fileErr)
	}
	if waitErr != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = waitErr.Error()
		}
		return backupState{}, fmt.Errorf("pg_dump failed: %s", msg)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return backupState{}, fmt.Errorf("finalize backup file: %w", err)
	}
	cleanup = false

	info, err := os.Stat(finalPath)
	if err != nil {
		return backupState{}, err
	}
	backup := backupState{
		Name:      name,
		Source:    "database",
		Size:      info.Size(),
		UpdatedAt: info.ModTime().Format(time.RFC3339),
	}
	_ = writeBackupMetadata(dir, backup)
	return backup, nil
}

func copyAndClose(gz *gzip.Writer, r io.Reader) error {
	if _, err := io.Copy(gz, r); err != nil {
		_ = gz.Close()
		return err
	}
	return gz.Close()
}

func dbDSN(db config.DatabaseConfig) string {
	u := &url.URL{
		Scheme: "postgresql",
		User:   url.User(db.User),
		Host:   net.JoinHostPort(db.Host, fmt.Sprint(db.Port)),
		Path:   "/" + db.Name,
	}
	q := u.Query()
	sslMode := strings.TrimSpace(db.SSLMode)
	if sslMode == "" {
		sslMode = "disable"
	}
	q.Set("sslmode", sslMode)
	u.RawQuery = q.Encode()
	return u.String()
}

func writeBackupMetadata(dir string, backup backupState) error {
	meta := map[string]any{
		"created_at": time.Now().Format(time.RFC3339),
		"sql":        backup.Name,
		"source":     backup.Source,
		"size":       backup.Size,
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "latest.json"), append(b, '\n'), 0600)
}

func safeBackupName(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if strings.ContainsAny(raw, `/\`) {
		return "", errors.New("invalid backup name")
	}
	name := filepath.Base(raw)
	if name == "." || name == "" || name != strings.TrimSpace(raw) {
		return "", errors.New("invalid backup name")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".sql.gz") {
		return "", errors.New("only .sql.gz database backups can be downloaded")
	}
	return name, nil
}

func applyRetention(d *Deps) (*retentionRunResponse, error) {
	if d == nil || d.Config == nil {
		return nil, errors.New("config is unavailable")
	}
	r := d.Config.Scheduler.Retention
	now := time.Now()
	resp := &retentionRunResponse{
		MonitorLogsDays:      r.MonitorLogsDays,
		BalanceSnapshotsDays: r.BalanceSnapshotsDays,
		NotificationLogsDays: r.NotificationLogsDays,
		RanAt:                now,
	}

	var errs []string
	if r.MonitorLogsDays > 0 {
		n, err := d.MonLogs.DeleteBefore(now.AddDate(0, 0, -r.MonitorLogsDays))
		if err != nil {
			errs = append(errs, "monitor_logs: "+err.Error())
		} else {
			resp.MonitorLogsDeleted = n
		}
	}
	if r.BalanceSnapshotsDays > 0 {
		n, err := d.Rates.DeleteBalanceSnapshotsBefore(now.AddDate(0, 0, -r.BalanceSnapshotsDays))
		if err != nil {
			errs = append(errs, "balance_snapshots: "+err.Error())
		} else {
			resp.BalanceSnapshotsDeleted = n
		}
	}
	if r.NotificationLogsDays > 0 {
		n, err := d.Notifies.DeleteLogsBefore(now.AddDate(0, 0, -r.NotificationLogsDays))
		if err != nil {
			errs = append(errs, "notification_logs: "+err.Error())
		} else {
			resp.NotificationLogsDeleted = n
		}
	}
	if len(errs) > 0 {
		return resp, errors.New(strings.Join(errs, "; "))
	}
	return resp, nil
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
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".sql.gz") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, backupState{
			Source:    "database",
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
