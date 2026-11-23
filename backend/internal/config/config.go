package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
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
	WebAllowedOrigins     []string
	RemoteAllowedUnits    []string
	DisableAuthentication bool
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

	disableAuthentication, parseErr := strconv.ParseBool(getEnv("MONITOR_ME_AUTH_DISABLED", "false"))
	if parseErr != nil {
		return Config{}, fmt.Errorf("MONITOR_ME_AUTH_DISABLED must be true or false")
	}
	cfg.DisableAuthentication = disableAuthentication

	var err error
	cfg.WebAllowedOrigins, err = parseWebAllowedOrigins(getEnv("WEB_ALLOWED_ORIGINS", "http://localhost:3000"))
	if err != nil {
		return Config{}, err
	}
	if err := validateDatabaseURL(cfg.DatabaseURL); err != nil {
		return Config{}, err
	}
	cfg.RemoteAllowedUnits, err = parseRemoteAllowedUnits(os.Getenv("REMOTE_ALLOWED_SYSTEMD_UNITS"))
	if err != nil {
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

func parseWebAllowedOrigins(raw string) ([]string, error) {
	seen := make(map[string]struct{})
	origins := make([]string, 0)
	for _, item := range strings.Split(raw, ",") {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		parsed, err := url.Parse(value)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.Host == "" || parsed.User != nil || parsed.Path != "" ||
			parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("WEB_ALLOWED_ORIGINS must contain comma-separated HTTP(S) origins without paths or credentials")
		}
		origin := parsed.Scheme + "://" + parsed.Host
		if _, exists := seen[origin]; exists {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	if len(origins) == 0 {
		return nil, fmt.Errorf("WEB_ALLOWED_ORIGINS must contain at least one HTTP(S) origin")
	}
	return origins, nil
}

var remoteSystemdUnitPattern = regexp.MustCompile("^[A-Za-z0-9_.:@-]{1,180}\\.service$")

func parseRemoteAllowedUnits(raw string) ([]string, error) {
	seen := make(map[string]struct{})
	units := make([]string, 0)
	for _, item := range strings.Split(raw, ",") {
		unit := strings.TrimSpace(item)
		if unit == "" {
			continue
		}
		if !remoteSystemdUnitPattern.MatchString(unit) {
			return nil, fmt.Errorf("REMOTE_ALLOWED_SYSTEMD_UNITS must contain comma-separated systemd .service unit names")
		}
		if _, exists := seen[unit]; exists {
			continue
		}
		seen[unit] = struct{}{}
		units = append(units, unit)
	}
	return units, nil
}
