package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains bounded, non-secret runtime settings for the native agent.
type Config struct {
	Version            string
	CollectionInterval time.Duration
	ProcRoot            string
	SpoolDirectory      string
	SpoolMaxBytes       int64
	LogLevel            string
}

// Load reads and validates environment configuration without echoing values.
func Load() (Config, error) {
	cfg := Config{
		Version:            env("MONITOR_AGENT_VERSION", "0.1.0-dev"),
		CollectionInterval: 500 * time.Millisecond,
		ProcRoot:           env("MONITOR_PROC_ROOT", "/proc"),
		SpoolDirectory:     env("MONITOR_SPOOL_DIR", "./.monitor-me/spool"),
		SpoolMaxBytes:      64 << 20,
		LogLevel:           strings.ToLower(env("MONITOR_LOG_LEVEL", "info")),
	}
	if raw := os.Getenv("MONITOR_COLLECTION_INTERVAL"); raw != "" {
		interval, err := time.ParseDuration(raw)
		if err != nil || interval < 100*time.Millisecond || interval > time.Minute {
			return Config{}, fmt.Errorf("MONITOR_COLLECTION_INTERVAL must be between 100ms and 1m")
		}
		cfg.CollectionInterval = interval
	}
	if raw := os.Getenv("MONITOR_SPOOL_MAX_BYTES"); raw != "" {
		size, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || size < 1<<20 || size > 10<<30 {
			return Config{}, fmt.Errorf("MONITOR_SPOOL_MAX_BYTES must be between 1MiB and 10GiB")
		}
		cfg.SpoolMaxBytes = size
	}
	if cfg.Version == "" || len(cfg.Version) > 64 {
		return Config{}, fmt.Errorf("MONITOR_AGENT_VERSION must contain 1 to 64 characters")
	}
	if cfg.ProcRoot == "" || cfg.SpoolDirectory == "" {
		return Config{}, fmt.Errorf("MONITOR_PROC_ROOT and MONITOR_SPOOL_DIR must not be empty")
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("MONITOR_LOG_LEVEL must be debug, info, warn, or error")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
