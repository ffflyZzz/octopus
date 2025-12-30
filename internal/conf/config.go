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
	RPS int `mapstructure:"rps"` // 每秒请求数，0=不限流
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
	viper.SetDefault("ratelimit.rps", 0) // 默认不限流
}
