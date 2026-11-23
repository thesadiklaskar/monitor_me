package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Config contains bounded runtime settings for collection, local durability,
// and optional authenticated delivery to the control plane.
type Config struct {
	Version            string
	CollectionInterval time.Duration
	ProcRoot           string
	SpoolDirectory     string
	SpoolMaxBytes      int64
	LogLevel           string
	GRPCAddress        string
	TLSCAFile          string
	EnrollmentToken    string
	StateFile          string
	AllowedSystemdUnits []string
}

// Load reads and validates environment configuration without echoing secrets.
func Load() (Config, error) {
	cfg := Config{
		Version:            env("MONITOR_AGENT_VERSION", "0.1.0-dev"),
		CollectionInterval: 500 * time.Millisecond,
		ProcRoot:           env("MONITOR_PROC_ROOT", "/proc"),
		SpoolDirectory:     env("MONITOR_SPOOL_DIR", "./.monitor-me/spool"),
		SpoolMaxBytes:      64 << 20,
		LogLevel:           strings.ToLower(env("MONITOR_LOG_LEVEL", "info")),
		GRPCAddress:        os.Getenv("MONITOR_GRPC_ADDRESS"),
		TLSCAFile:          os.Getenv("MONITOR_TLS_CA_FILE"),
		EnrollmentToken:    os.Getenv("MONITOR_ENROLLMENT_TOKEN"),
	}
	cfg.StateFile = env("MONITOR_AGENT_STATE_FILE", filepath.Join(cfg.SpoolDirectory, "agent-state.json"))
	cfg.AllowedSystemdUnits, err = parseAllowedSystemdUnits(os.Getenv("MONITOR_ALLOWED_SYSTEMD_UNITS"))
	if err != nil {
		return Config{}, err
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
	if cfg.ProcRoot == "" || cfg.SpoolDirectory == "" || cfg.StateFile == "" {
		return Config{}, fmt.Errorf("MONITOR_PROC_ROOT, MONITOR_SPOOL_DIR, and MONITOR_AGENT_STATE_FILE must not be empty")
	}
	if (cfg.GRPCAddress == "") != (cfg.TLSCAFile == "") {
		return Config{}, fmt.Errorf("MONITOR_GRPC_ADDRESS and MONITOR_TLS_CA_FILE must be configured together")
	}
	if cfg.GRPCAddress == "" && cfg.EnrollmentToken != "" {
		return Config{}, fmt.Errorf("MONITOR_ENROLLMENT_TOKEN requires MONITOR_GRPC_ADDRESS and MONITOR_TLS_CA_FILE")
	}
	if len(cfg.EnrollmentToken) > 128 {
		return Config{}, fmt.Errorf("MONITOR_ENROLLMENT_TOKEN exceeds the supported length")
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


var allowedSystemdUnitPattern = regexp.MustCompile("^[A-Za-z0-9_.:@-]{1,180}\\.service$")

func parseAllowedSystemdUnits(raw string) ([]string, error) {
	seen := make(map[string]struct{})
	units := make([]string, 0)
	for _, item := range strings.Split(raw, ",") {
		unit := strings.TrimSpace(item)
		if unit == "" {
			continue
		}
		if !allowedSystemdUnitPattern.MatchString(unit) {
			return nil, fmt.Errorf("MONITOR_ALLOWED_SYSTEMD_UNITS must contain comma-separated systemd .service unit names")
		}
		if _, exists := seen[unit]; exists {
			continue
		}
		seen[unit] = struct{}{}
		units = append(units, unit)
	}
	return units, nil
}
