package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"github.com/worryzyy/upstream-hub/internal/storage"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Security  SecurityConfig  `mapstructure:"security"`
	Auth      AuthConfig      `mapstructure:"auth"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
	Log       LogConfig       `mapstructure:"log"`
}

type ServerConfig struct {
	Port           int      `mapstructure:"port"`
	Mode           string   `mapstructure:"mode"`
	TrustedProxies []string `mapstructure:"trustedProxies"`
	BaseURL        string   `mapstructure:"baseURL"`
}

type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	Name         string `mapstructure:"name"`
	SSLMode      string `mapstructure:"sslMode"`
	Timezone     string `mapstructure:"timezone"`
	MaxOpenConns int    `mapstructure:"maxOpenConns"`
	MaxIdleConns int    `mapstructure:"maxIdleConns"`
}

func (d DatabaseConfig) ToStorageConfig() storage.DBConfig {
	return storage.DBConfig{
		Host:         d.Host,
		Port:         d.Port,
		User:         d.User,
		Password:     d.Password,
		Name:         d.Name,
		SSLMode:      d.SSLMode,
		Timezone:     d.Timezone,
		MaxOpenConns: d.MaxOpenConns,
		MaxIdleConns: d.MaxIdleConns,
	}
}

type SecurityConfig struct {
	// AppSecret 主密钥，用于 AES-GCM。优先从 APP_SECRET 环境变量读取。
	AppSecret string `mapstructure:"appSecret"`
}

// AuthConfig 后台单用户登录配置。
// Username / Password 是写死的管理员凭据，TokenSecret 用于签发 HMAC token。
// 如果 TokenSecret 为空，会回退使用 Security.AppSecret，保证有合理默认。
type AuthConfig struct {
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	TokenSecret     string `mapstructure:"tokenSecret"`
	SessionTTLHours int    `mapstructure:"sessionTTLHours"`
}

type SchedulerConfig struct {
	BalanceCron string          `mapstructure:"balanceCron"`
	RateCron    string          `mapstructure:"rateCron"`
	Concurrency int             `mapstructure:"concurrency"`
	Retention   RetentionConfig `mapstructure:"retention"`
}

// RetentionConfig 历史数据保留策略。
//
// 字段为 0 表示该表不清理，永久保留（默认 rate_change_logs 永远保留，是核心业务数据）。
// Cron 为空时不启动清理任务。
type RetentionConfig struct {
	Cron                 string `mapstructure:"cron"`
	MonitorLogsDays      int    `mapstructure:"monitorLogsDays"`
	BalanceSnapshotsDays int    `mapstructure:"balanceSnapshotsDays"`
	NotificationLogsDays int    `mapstructure:"notificationLogsDays"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// Load 读取 config.yaml（可选）+ APP_SECRET / UPSTREAMHUB_* 环境变量覆盖。
//
// 关键映射：
//
//	APP_SECRET                       -> security.appSecret
//	UPSTREAMHUB_DATABASE_HOST        -> database.host
//	UPSTREAMHUB_SERVER_PORT          -> server.port
//	UPSTREAMHUB_SCHEDULER_BALANCECRON-> scheduler.balanceCron
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")

	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.AddConfigPath(".")
		v.AddConfigPath("./backend")
		v.AddConfigPath("/etc/upstream-hub")
	}

	setDefaults(v)

	v.SetEnvPrefix("UPSTREAMHUB")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	// APP_SECRET / ADMIN_USERNAME / ADMIN_PASSWORD 是独立约定的环境变量名，不带前缀。
	_ = v.BindEnv("security.appSecret", "APP_SECRET")
	_ = v.BindEnv("auth.username", "ADMIN_USERNAME")
	_ = v.BindEnv("auth.password", "ADMIN_PASSWORD")
	_ = v.BindEnv("auth.tokenSecret", "AUTH_TOKEN_SECRET")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 8418)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.baseURL", "http://localhost:8418")

	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 54329)
	v.SetDefault("database.sslMode", "disable")
	v.SetDefault("database.timezone", "Asia/Shanghai")
	v.SetDefault("database.maxOpenConns", 20)
	v.SetDefault("database.maxIdleConns", 5)

	// CLAUDE.md 默认建议：余额 15 分钟，倍率 30 分钟。
	v.SetDefault("scheduler.balanceCron", "37 */15 * * * *")
	v.SetDefault("scheduler.rateCron", "13 */30 * * * *")
	v.SetDefault("scheduler.concurrency", 4)

	// 历史清理：每天凌晨 3:17 跑一次（6 字段 cron 含秒），
	// monitor 30 天 / balance 90 天 / notify 90 天。rate_change_logs 不清理（业务核心数据）。
	v.SetDefault("scheduler.retention.cron", "0 17 3 * * *")
	v.SetDefault("scheduler.retention.monitorLogsDays", 30)
	v.SetDefault("scheduler.retention.balanceSnapshotsDays", 90)
	v.SetDefault("scheduler.retention.notificationLogsDays", 90)

	v.SetDefault("auth.username", "admin")
	v.SetDefault("auth.sessionTTLHours", 168) // 7 天

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")
}
