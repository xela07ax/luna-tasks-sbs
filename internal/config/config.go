package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Auth      AuthConfig      `mapstructure:"auth"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
	Metrics   MetricsConfig   `mapstructure:"metrics"`
}

type ServerConfig struct {
	Port               int           `mapstructure:"port"`
	ReadTimeout        time.Duration `mapstructure:"read_timeout"`
	WriteTimeout       time.Duration `mapstructure:"write_timeout"`
	CorsAllowedOrigins []string      `mapstructure:"cors_allowed_origins"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"dbname"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type AuthConfig struct {
	PublicKeyPath   string        `mapstructure:"public_key_path"`
	PrivateKeyPath  string        `mapstructure:"private_key_path"`
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
	PublicKey       []byte
	PrivateKey      []byte
}

type RateLimitConfig struct {
	RequestsPerMinute float64 `mapstructure:"requests_per_minute"`
	Burst             int     `mapstructure:"burst"`
}

type MetricsConfig struct {
	Enabled bool `mapstructure:"enabled"`
	Port    int  `mapstructure:"port"`
}

func LoadConfig(path string) (*Config, error) {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(path)
	v.AddConfigPath(".")

	// Поддержка переменных окружения (например, LUNA_DATABASE_HOST переопределит database.host)
	v.SetEnvPrefix("LUNA")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Дефолтные значения
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "10s")
	v.SetDefault("server.write_timeout", "10s")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 3306)
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("auth.access_token_ttl", "15m")
	v.SetDefault("auth.refresh_token_ttl", "168h")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Если файла нет, работаем на дефолтах и ENV
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %w", err)
	}

	// Загрузка ключей из Файла ИЛИ из ENV
	cfg.Auth.PublicKey = loadKeyResource(cfg.Auth.PublicKeyPath, "LUNA_AUTH_PUBLIC_KEY_DATA")
	cfg.Auth.PrivateKey = loadKeyResource(cfg.Auth.PrivateKeyPath, "LUNA_AUTH_PRIVATE_KEY_DATA")

	return &cfg, nil
}

// loadKeyResource — читает ключ из ENV (приоритет) или из файла
func loadKeyResource(path string, envDataKey string) []byte {
	if data := os.Getenv(envDataKey); data != "" {
		return []byte(data)
	}
	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			return data
		}
	}
	return nil
}
