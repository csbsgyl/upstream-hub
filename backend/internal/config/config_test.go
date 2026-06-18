package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestDefaultNotificationsAvoidRetryDuplicates(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		t.Fatalf("unmarshal defaults error = %v", err)
	}
	normalizeNotificationsConfig(&cfg.Notifications)
	if cfg.Notifications.SendMaxAttempts != 1 {
		t.Fatalf("SendMaxAttempts default = %d, want 1", cfg.Notifications.SendMaxAttempts)
	}
}

func TestDefaultSchedulerUsesUnifiedSync(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		t.Fatalf("unmarshal defaults error = %v", err)
	}
	if cfg.Scheduler.SyncCron != defaultSchedulerSyncCron {
		t.Fatalf("SyncCron default = %q, want 3 minute unified sync", cfg.Scheduler.SyncCron)
	}
	if cfg.Scheduler.BalanceCron != "" || cfg.Scheduler.RateCron != "" {
		t.Fatalf("legacy cron defaults = balance %q rate %q, want both empty", cfg.Scheduler.BalanceCron, cfg.Scheduler.RateCron)
	}
}

func TestLegacyDefaultSchedulerMigratesToThreeMinutes(t *testing.T) {
	t.Setenv("UPSTREAMHUB_SCHEDULER_SYNC_CRON", legacyDefaultSchedulerSyncCron)
	t.Setenv("UPSTREAMHUB_SCHEDULER_BALANCE_CRON", "")
	t.Setenv("UPSTREAMHUB_SCHEDULER_RATE_CRON", "")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load config error = %v", err)
	}
	if cfg.Scheduler.SyncCron != defaultSchedulerSyncCron {
		t.Fatalf("SyncCron = %q, want migrated default %q", cfg.Scheduler.SyncCron, defaultSchedulerSyncCron)
	}
}

func TestSchedulerConcurrencyEnvOverride(t *testing.T) {
	t.Setenv("UPSTREAMHUB_SCHEDULER_CONCURRENCY", "2")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load config error = %v", err)
	}
	if cfg.Scheduler.Concurrency != 2 {
		t.Fatalf("Concurrency = %d, want env override 2", cfg.Scheduler.Concurrency)
	}
}

func TestEmptySchedulerEnvCanDisableUnifiedSync(t *testing.T) {
	t.Setenv("UPSTREAMHUB_SCHEDULER_SYNC_CRON", "")
	t.Setenv("UPSTREAMHUB_SCHEDULER_BALANCE_CRON", "37 */15 * * * *")
	t.Setenv("UPSTREAMHUB_SCHEDULER_RATE_CRON", "13 */30 * * * *")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load config error = %v", err)
	}
	if cfg.Scheduler.SyncCron != "" {
		t.Fatalf("SyncCron = %q, want empty from env override", cfg.Scheduler.SyncCron)
	}
	if cfg.Scheduler.BalanceCron != "37 */15 * * * *" {
		t.Fatalf("BalanceCron = %q, want env override", cfg.Scheduler.BalanceCron)
	}
	if cfg.Scheduler.RateCron != "13 */30 * * * *" {
		t.Fatalf("RateCron = %q, want env override", cfg.Scheduler.RateCron)
	}
}

func TestLegacySchedulerEnvAliasesStillWork(t *testing.T) {
	t.Setenv("UPSTREAMHUB_SCHEDULER_SYNCCRON", "7 */10 * * * *")
	t.Setenv("UPSTREAMHUB_SCHEDULER_BALANCECRON", "11 */20 * * * *")
	t.Setenv("UPSTREAMHUB_SCHEDULER_RATECRON", "17 */30 * * * *")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load config error = %v", err)
	}
	if cfg.Scheduler.SyncCron != "7 */10 * * * *" {
		t.Fatalf("SyncCron = %q, want legacy alias override", cfg.Scheduler.SyncCron)
	}
	if cfg.Scheduler.BalanceCron != "11 */20 * * * *" {
		t.Fatalf("BalanceCron = %q, want legacy alias override", cfg.Scheduler.BalanceCron)
	}
	if cfg.Scheduler.RateCron != "17 */30 * * * *" {
		t.Fatalf("RateCron = %q, want legacy alias override", cfg.Scheduler.RateCron)
	}
}
