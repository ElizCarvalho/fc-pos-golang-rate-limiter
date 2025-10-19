package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
	Redis     RedisConfig     `mapstructure:"redis"`
}

type ServerConfig struct {
	Port   string `mapstructure:"port"`
	AppEnv string `mapstructure:"app_env"`
}

type RateLimitConfig struct {
	IPLimit              int `mapstructure:"ip_limit"`
	WindowSeconds        int `mapstructure:"window_seconds"`
	BlockDurationSeconds int `mapstructure:"block_duration_seconds"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// LoadConfig carrega configurações da aplicação usando viper com suporte a .env e defaults
func LoadConfig() (*Config, error) {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./configs")

	viper.SetDefault("SERVER_PORT", "8080")
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("RATE_LIMIT_IP", 10)
	viper.SetDefault("RATE_LIMIT_WINDOW_SECONDS", 1)
	viper.SetDefault("RATE_LIMIT_BLOCK_DURATION_SECONDS", 300)
	viper.SetDefault("REDIS_HOST", "localhost")
	viper.SetDefault("REDIS_PORT", "6379")
	viper.SetDefault("REDIS_PASSWORD", "")
	viper.SetDefault("REDIS_DB", 0)

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	viper.Set("server.port", viper.GetString("SERVER_PORT"))
	viper.Set("server.app_env", viper.GetString("APP_ENV"))
	viper.Set("rate_limit.ip_limit", viper.GetInt("RATE_LIMIT_IP"))
	viper.Set("rate_limit.window_seconds", viper.GetInt("RATE_LIMIT_WINDOW_SECONDS"))
	viper.Set("rate_limit.block_duration_seconds", viper.GetInt("RATE_LIMIT_BLOCK_DURATION_SECONDS"))
	viper.Set("redis.host", viper.GetString("REDIS_HOST"))
	viper.Set("redis.port", viper.GetString("REDIS_PORT"))
	viper.Set("redis.password", viper.GetString("REDIS_PASSWORD"))
	viper.Set("redis.db", viper.GetInt("REDIS_DB"))

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

func (c *RateLimitConfig) GetWindowDuration() time.Duration {
	return time.Duration(c.WindowSeconds) * time.Second
}

func (c *RateLimitConfig) GetBlockDuration() time.Duration {
	return time.Duration(c.BlockDurationSeconds) * time.Second
}

func (c *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}
