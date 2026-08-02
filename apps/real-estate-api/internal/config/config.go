package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ApplicationName        string
	ApplicationBuild       string
	Environment            string
	Address                string
	DatabaseURL            string
	DatabaseMaxConnections int32
	AllowedOrigins         []string
	LogLevel               string
	ReadHeaderTimeout      time.Duration
	ReadTimeout            time.Duration
	WriteTimeout           time.Duration
	IdleTimeout            time.Duration
	ShutdownTimeout        time.Duration
}

func Load() (Config, error) {
	maxConnections, err := int32Env("DATABASE_MAX_CONNECTIONS", 10)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		ApplicationName:        env("API_NAME", "real-estate-api"),
		ApplicationBuild:       env("API_BUILD", "development"),
		Environment:            env("API_ENV", "development"),
		Address:                env("API_ADDRESS", ":8080"),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		DatabaseMaxConnections: maxConnections,
		AllowedOrigins:         csvEnv("API_ALLOWED_ORIGINS"),
		LogLevel:               env("LOG_LEVEL", "info"),
		ReadHeaderTimeout:      5 * time.Second,
		ReadTimeout:            15 * time.Second,
		WriteTimeout:           30 * time.Second,
		IdleTimeout:            60 * time.Second,
		ShutdownTimeout:        10 * time.Second,
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.DatabaseMaxConnections < 1 {
		return Config{}, errors.New("DATABASE_MAX_CONNECTIONS must be positive")
	}
	if !oneOf(cfg.LogLevel, "debug", "info", "warn", "error") {
		return Config{}, fmt.Errorf("unsupported LOG_LEVEL %q", cfg.LogLevel)
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func csvEnv(name string) []string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func int32Env(name string, fallback int32) (int32, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	return int32(parsed), nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
