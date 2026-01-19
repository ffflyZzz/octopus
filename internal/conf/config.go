package conf

import (
	"fmt"
	"os"
	"strings"

	"octopus/internal/utils/log"
	"github.com/spf13/viper"
)

type Server struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type Log struct {
	Level string `mapstructure:"level"`
}

type Database struct {
	Type string `mapstructure:"type"`
	Path string `mapstructure:"path"`
}

type AmpCode struct {
	Enabled                       bool   `mapstructure:"enabled"`
	UpstreamURL                   string `mapstructure:"upstream_url"`
	UpstreamAPIKey                string `mapstructure:"upstream_api_key"`
	RestrictManagementToLocalhost bool   `mapstructure:"restrict_management_to_localhost"`
}

type RateLimit struct {
	MaxConcurrentRequests int `mapstructure:"max_concurrent_requests"` // 兼容旧配置
	FastMaxConcurrent     int `mapstructure:"fast_max_concurrent"`     // 快池并发
	SlowMaxConcurrent     int `mapstructure:"slow_max_concurrent"`     // 慢池并发
	MigrateAfterSeconds   int `mapstructure:"migrate_after_seconds"`   // 快池转慢池阈值
	RateLimitPerSecond    int `mapstructure:"rate_limit_per_second"`   // 每秒新请求数
	RateLimitBurst        int `mapstructure:"rate_limit_burst"`        // 速率突发容量
	MaxQueueSize          int `mapstructure:"max_queue_size"`          // 排队上限
	MaxQueueWaitSeconds   int `mapstructure:"max_queue_wait_seconds"`  // 最大排队时间
}

type Config struct {
	Server    Server    `mapstructure:"server"`
	Log       Log       `mapstructure:"log"`
	Database  Database  `mapstructure:"database"`
	AmpCode   AmpCode   `mapstructure:"ampcode"`
	RateLimit RateLimit `mapstructure:"ratelimit"`
}

var AppConfig Config

func Load(path string) error {
	if path != "" {
		viper.SetConfigFile(path)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("json")
		viper.AddConfigPath("data")
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix(APP_NAME)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	setDefaults()

	if err := viper.ReadInConfig(); err == nil {
		log.Infof("Using config file: %s", viper.ConfigFileUsed())
	} else {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Infof("Config file not found, creating default config")
			if err := os.MkdirAll("data", 0755); err != nil {
				log.Errorf("Failed to create data directory: %v", err)
			}
			if err := viper.SafeWriteConfigAs("data/config.json"); err != nil {
				log.Errorf("Failed to create default config: %v", err)
			}
		} else {
			return fmt.Errorf("error reading config file: %w", err)
		}
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		return fmt.Errorf("unable to decode config into struct: %w", err)
	}
	return nil
}

func setDefaults() {
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("database.type", "sqlite")
	viper.SetDefault("database.path", "data/data.db")
	viper.SetDefault("log.level", "info")
	// AmpCode defaults
	viper.SetDefault("ampcode.enabled", false)
	viper.SetDefault("ampcode.upstream_url", "https://ampcode.com")
	viper.SetDefault("ampcode.restrict_management_to_localhost", false)
	// RateLimit defaults
	viper.SetDefault("ratelimit.max_concurrent_requests", 0) // 默认不限制并发
	viper.SetDefault("ratelimit.fast_max_concurrent", 0)
	viper.SetDefault("ratelimit.slow_max_concurrent", 0)
	viper.SetDefault("ratelimit.migrate_after_seconds", 10)
	viper.SetDefault("ratelimit.rate_limit_per_second", 0)
	viper.SetDefault("ratelimit.rate_limit_burst", 0)
	viper.SetDefault("ratelimit.max_queue_size", 1000)
	viper.SetDefault("ratelimit.max_queue_wait_seconds", 120)
}
