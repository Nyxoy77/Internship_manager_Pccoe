package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost              string
	DBPort              string
	DBUser              string
	DBPassword          string
	DBName              string
	JWTSecret           string
	ServerPort          string
	AppEnv              string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	LoginRateLimit      int
	LoginRateWindowSecs int
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Load .env file if it exists (ignore error in production)
	_ = godotenv.Load()

	config := &Config{
		DBHost:              getEnv("DB_HOST", "localhost"),
		DBPort:              getEnv("DB_PORT", "5432"),
		DBUser:              getEnv("DB_USER", "postgres"),
		DBPassword:          getEnv("DB_PASSWORD", ""),
		DBName:              getEnv("DB_NAME", "internship_db"),
		JWTSecret:           getEnv("JWT_SECRET", ""),
		ServerPort:          getEnv("SERVER_PORT", "8080"),
		AppEnv:              strings.ToLower(getEnv("APP_ENV", "development")),
		AccessTokenTTL:      getDurationEnv("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:     getDurationEnv("REFRESH_TOKEN_TTL", 7*24*time.Hour),
		LoginRateLimit:      getIntEnv("LOGIN_RATE_LIMIT", 5),
		LoginRateWindowSecs: getIntEnv("LOGIN_RATE_WINDOW_SECS", 60),
	}

	// Validate required fields
	if config.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable is required")
	}
	if len(config.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if config.AppEnv == "production" {
		weak := map[string]struct{}{
			"dev-secret-change-this": {},
			"secret":                 {},
			"password":               {},
			"changeme":               {},
		}
		if _, exists := weak[strings.ToLower(config.JWTSecret)]; exists {
			return nil, fmt.Errorf("weak JWT_SECRET is not allowed in production")
		}
	}
	if config.AccessTokenTTL <= 0 || config.RefreshTokenTTL <= 0 {
		return nil, fmt.Errorf("token TTL values must be positive")
	}
	if config.LoginRateLimit <= 0 || config.LoginRateWindowSecs <= 0 {
		return nil, fmt.Errorf("login rate limiting values must be positive")
	}

	return config, nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}
	return d
}

func getIntEnv(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return n
}
