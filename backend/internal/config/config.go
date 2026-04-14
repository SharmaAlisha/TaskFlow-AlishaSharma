package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv       string
	AppPort      string
	AppHost      string
	BodyLimitMB  int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	DBHost    string
	DBPort    string
	DBUser    string
	DBPass    string
	DBName    string
	DBSSLMode string
	DBMaxConn int32
	DBMinConn int32

	JWTSecret        string
	JWTAccessExpiry  time.Duration
	JWTRefreshExpiry time.Duration

	CORSAllowedOrigins []string

	RateLimitAuthMax    int
	RateLimitAuthWindow time.Duration
	RateLimitAPIMax     int
	RateLimitAPIWindow  time.Duration

	WebhookMaxRetries    int
	WebhookTimeout       time.Duration
	WebhookWorkerPoolSz  int

	SeedDB bool

	LogLevel  string
	LogFormat string
}

func (c *Config) DatabaseURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.DBUser, c.DBPass, c.DBHost, c.DBPort, c.DBName, c.DBSSLMode,
	)
}

func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:       getEnv("APP_ENV", "development"),
		AppPort:      getEnv("APP_PORT", "8080"),
		AppHost:      getEnv("APP_HOST", "0.0.0.0"),
		BodyLimitMB:  getEnvInt("APP_BODY_LIMIT_MB", 1),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,

		DBHost:    getEnv("DB_HOST", "localhost"),
		DBPort:    getEnv("DB_PORT", "5432"),
		DBUser:    getEnv("DB_USER", "taskflow"),
		DBPass:    getEnv("DB_PASSWORD", ""),
		DBName:    getEnv("DB_NAME", "taskflow"),
		DBSSLMode: getEnv("DB_SSL_MODE", "disable"),
		DBMaxConn: int32(getEnvInt("DB_MAX_CONNS", 25)),
		DBMinConn: int32(getEnvInt("DB_MIN_CONNS", 5)),

		JWTSecret:        getEnv("JWT_SECRET", ""),
		JWTAccessExpiry:  getEnvDuration("JWT_ACCESS_EXPIRY", 24*time.Hour),
		JWTRefreshExpiry: getEnvDuration("JWT_REFRESH_EXPIRY", 168*time.Hour),

		CORSAllowedOrigins: strings.Split(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000"), ","),

		RateLimitAuthMax:    getEnvInt("RATE_LIMIT_AUTH_MAX", 5),
		RateLimitAuthWindow: getEnvDuration("RATE_LIMIT_AUTH_WINDOW", 15*time.Minute),
		RateLimitAPIMax:     getEnvInt("RATE_LIMIT_API_MAX", 100),
		RateLimitAPIWindow:  getEnvDuration("RATE_LIMIT_API_WINDOW", 1*time.Minute),

		WebhookMaxRetries:   getEnvInt("WEBHOOK_MAX_RETRIES", 3),
		WebhookTimeout:      getEnvDuration("WEBHOOK_TIMEOUT", 10*time.Second),
		WebhookWorkerPoolSz: getEnvInt("WEBHOOK_WORKER_POOL_SIZE", 10),

		SeedDB: getEnv("SEED_DB", "false") == "true",

		LogLevel:  getEnv("LOG_LEVEL", "info"),
		LogFormat: getEnv("LOG_FORMAT", "json"),
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if len(c.JWTSecret) < 16 {
		return fmt.Errorf("JWT_SECRET must be at least 16 characters")
	}
	if c.DBPass == "" {
		return fmt.Errorf("DB_PASSWORD is required")
	}
	if c.DBHost == "" {
		return fmt.Errorf("DB_HOST is required")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
