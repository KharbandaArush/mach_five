package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the trading system
type Config struct {
	GoogleSheets GoogleSheetsConfig
	Redis        RedisConfig
	Broker       BrokerConfig
	Logging      LoggingConfig
	Trigger      TriggerConfig
}

// GoogleSheetsConfig holds Google Sheets API configuration
type GoogleSheetsConfig struct {
	CredentialsPath string
	SheetID         string
	BuyRange        string
	SellRange       string
	RefreshInterval time.Duration
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// BrokerConfig holds broker configuration
type BrokerConfig struct {
	ConfigPath     string
	Type           string
	APIKey         string
	APISecret      string
	BaseURL        string
	RateLimit      RateLimitConfig
	OrderSplitting OrderSplittingConfig
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	RequestsPerSecond int
	BurstSize         int
}

// OrderSplittingConfig holds order splitting configuration
type OrderSplittingConfig struct {
	MaxOrderSize int // Maximum quantity per order before splitting
	Enabled      bool // Whether order splitting is enabled
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level      string
	ReadLog    string
	TriggerLog string
	BrokerLog  string
}

// TriggerConfig holds trigger module configuration
type TriggerConfig struct {
	WorkerPoolSize int
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	cfg := &Config{}

	// Google Sheets config
	cfg.GoogleSheets.CredentialsPath = getEnv("GOOGLE_SHEETS_CREDENTIALS_PATH", "./config/google-credentials.json")
	cfg.GoogleSheets.SheetID = getEnv("GOOGLE_SHEET_ID", "")
	cfg.GoogleSheets.BuyRange = getEnv("GOOGLE_SHEET_BUY_RANGE", "to_buy!B3:J")
	cfg.GoogleSheets.SellRange = getEnv("GOOGLE_SHEET_SELL_RANGE", "to_sell!B3:J")
	refreshInterval := getEnv("GOOGLE_SHEETS_REFRESH_INTERVAL", "1m")
	var err error
	cfg.GoogleSheets.RefreshInterval, err = time.ParseDuration(refreshInterval)
	if err != nil {
		cfg.GoogleSheets.RefreshInterval = 1 * time.Minute
	}

	// Redis config
	cfg.Redis.Addr = getEnv("REDIS_ADDR", "localhost:6379")
	cfg.Redis.Password = getEnv("REDIS_PASSWORD", "")
	cfg.Redis.DB, _ = strconv.Atoi(getEnv("REDIS_DB", "0"))

	// Broker config
	cfg.Broker.ConfigPath = getEnv("BROKER_CONFIG_PATH", "./config/broker-config.json")
	cfg.Broker.Type = getEnv("BROKER_TYPE", "mock")
	cfg.Broker.APIKey = getEnv("BROKER_API_KEY", "")
	cfg.Broker.APISecret = getEnv("BROKER_API_SECRET", "")
	cfg.Broker.BaseURL = getEnv("BROKER_BASE_URL", "")

	// Rate limit config
	cfg.Broker.RateLimit.RequestsPerSecond, _ = strconv.Atoi(getEnv("BROKER_RATE_LIMIT_RPS", "10"))
	cfg.Broker.RateLimit.BurstSize, _ = strconv.Atoi(getEnv("BROKER_RATE_LIMIT_BURST", "20"))

	// Order splitting config
	cfg.Broker.OrderSplitting.MaxOrderSize, _ = strconv.Atoi(getEnv("MAX_ORDER_SIZE", "1000"))
	cfg.Broker.OrderSplitting.Enabled = getEnv("ORDER_SPLITTING_ENABLED", "true") == "true"
	if cfg.Broker.OrderSplitting.MaxOrderSize <= 0 {
		cfg.Broker.OrderSplitting.MaxOrderSize = 1000 // Default to 1000
	}

	// Logging config
	cfg.Logging.Level = getEnv("LOG_LEVEL", "INFO")
	cfg.Logging.ReadLog = getEnv("READ_LOG_PATH", "./logs/read-module.log")
	cfg.Logging.TriggerLog = getEnv("TRIGGER_LOG_PATH", "./logs/trigger-module.log")
	cfg.Logging.BrokerLog = getEnv("BROKER_LOG_PATH", "./logs/broker-module.log")

	// Trigger config
	cfg.Trigger.WorkerPoolSize, _ = strconv.Atoi(getEnv("WORKER_POOL_SIZE", "5"))
	if cfg.Trigger.WorkerPoolSize <= 0 {
		cfg.Trigger.WorkerPoolSize = 5
	}

	// Load broker config from file if path is provided
	if cfg.Broker.ConfigPath != "" {
		if err := cfg.loadBrokerConfigFromFile(); err != nil {
			return nil, fmt.Errorf("failed to load broker config: %w", err)
		}
	}

	return cfg, nil
}

func (c *Config) loadBrokerConfigFromFile() error {
	data, err := os.ReadFile(c.Broker.ConfigPath)
	if err != nil {
		// File doesn't exist, use env vars
		return nil
	}

	var fileConfig struct {
		Type           string               `json:"type"`
		APIKey         string               `json:"api_key"`
		APISecret      string               `json:"api_secret"`
		BaseURL        string               `json:"base_url"`
		RateLimit      RateLimitConfig      `json:"rate_limit"`
		OrderSplitting OrderSplittingConfig `json:"order_splitting"`
	}

	if err := json.Unmarshal(data, &fileConfig); err != nil {
		return err
	}

	// Override with file config if not set in env
	if fileConfig.Type != "" {
		c.Broker.Type = fileConfig.Type
	}
	if fileConfig.APIKey != "" {
		c.Broker.APIKey = fileConfig.APIKey
	}
	if fileConfig.APISecret != "" {
		c.Broker.APISecret = fileConfig.APISecret
	}
	if fileConfig.BaseURL != "" {
		c.Broker.BaseURL = fileConfig.BaseURL
	}
	if fileConfig.RateLimit.RequestsPerSecond > 0 {
		c.Broker.RateLimit = fileConfig.RateLimit
	}
	if fileConfig.OrderSplitting.MaxOrderSize > 0 {
		c.Broker.OrderSplitting = fileConfig.OrderSplitting
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

