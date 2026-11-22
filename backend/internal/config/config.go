package config

import (
	"fmt"
	"net/url"
	"os"
	"time"
)

// Config contains runtime settings for the control-plane API and agent RPC server.
type Config struct {
	APIAddress           string
	ServiceVersion       string
	DatabaseURL          string
	RedisAddress         string
	RedisPassword        string
	AgentGRPCAddress     string
	AgentTLSCertFile     string
	AgentTLSKeyFile      string
	DBMaxOpenConnections int
	DBMaxIdleConnections int
	DBConnectionLifetime time.Duration
	StartupTimeout       time.Duration
}

// Load reads and validates environment-based configuration.
// Secret values are never included in configuration errors or logs.
func Load() (Config, error) {
	cfg := Config{
		APIAddress:           getEnv("API_ADDR", ":8080"),
		ServiceVersion:       getEnv("SERVICE_VERSION", "0.1.0-dev"),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://monitor_me:local_only_change_me@localhost:5432/monitor_me?sslmode=disable"),
		RedisAddress:         getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:        os.Getenv("REDIS_PASSWORD"),
		AgentGRPCAddress:     getEnv("AGENT_GRPC_ADDR", ":8443"),
		AgentTLSCertFile:     getEnv("AGENT_TLS_CERT_FILE", ".data/certs/server.crt"),
		AgentTLSKeyFile:      getEnv("AGENT_TLS_KEY_FILE", ".data/certs/server.key"),
		DBMaxOpenConnections: 25,
		DBMaxIdleConnections: 10,
		DBConnectionLifetime: 30 * time.Minute,
		StartupTimeout:       10 * time.Second,
	}

	if err := validateDatabaseURL(cfg.DatabaseURL); err != nil {
		return Config{}, err
	}
	if cfg.RedisAddress == "" {
		return Config{}, fmt.Errorf("REDIS_ADDR must not be empty")
	}
	if cfg.AgentGRPCAddress == "" || cfg.AgentTLSCertFile == "" || cfg.AgentTLSKeyFile == "" {
		return Config{}, fmt.Errorf("agent gRPC address and TLS certificate/key files are required")
	}
	if cfg.AgentTLSCertFile == cfg.AgentTLSKeyFile {
		return Config{}, fmt.Errorf("agent TLS certificate and key paths must be different")
	}

	return cfg, nil
}

func validateDatabaseURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return fmt.Errorf("DATABASE_URL must use the postgres or postgresql scheme")
	}
	if parsed.Host == "" || parsed.Path == "" || parsed.Path == "/" {
		return fmt.Errorf("DATABASE_URL must include a host and database name")
	}
	if parsed.User == nil || parsed.User.Username() == "" {
		return fmt.Errorf("DATABASE_URL must include a database username")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
