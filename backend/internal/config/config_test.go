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
