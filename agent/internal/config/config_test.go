package config

import (
	"testing"
	"time"
)

func TestLoadDefaultsAndOverrides(t *testing.T) {
	t.Setenv("MONITOR_COLLECTION_INTERVAL", "250ms")
	t.Setenv("MONITOR_SPOOL_MAX_BYTES", "2097152")
	t.Setenv("MONITOR_LOG_LEVEL", "debug")
	t.Setenv("MONITOR_PROC_ROOT", "/fixture/proc")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.CollectionInterval != 250*time.Millisecond {
		t.Fatalf("unexpected interval: %s", cfg.CollectionInterval)
	}
	if cfg.SpoolMaxBytes != 2<<20 || cfg.LogLevel != "debug" || cfg.ProcRoot != "/fixture/proc" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadAcceptsOfflineModeAndOptionalEnrollment(t *testing.T) {
	t.Setenv("MONITOR_GRPC_ADDRESS", "")
	t.Setenv("MONITOR_TLS_CA_FILE", "")
	t.Setenv("MONITOR_ENROLLMENT_TOKEN", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("offline agent configuration should be valid: %v", err)
	}
	if cfg.GRPCAddress != "" || cfg.TLSCAFile != "" {
		t.Fatalf("offline configuration unexpectedly enables transport: %+v", cfg)
	}

	t.Setenv("MONITOR_GRPC_ADDRESS", "localhost:8443")
	t.Setenv("MONITOR_TLS_CA_FILE", "/run/secrets/agent-ca.crt")
	t.Setenv("MONITOR_ENROLLMENT_TOKEN", "scoped-enrollment-secret")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("connected agent configuration should be valid: %v", err)
	}
	if cfg.GRPCAddress != "localhost:8443" || cfg.TLSCAFile != "/run/secrets/agent-ca.crt" ||
		cfg.EnrollmentToken != "scoped-enrollment-secret" {
		t.Fatalf("unexpected transport settings: %+v", cfg)
	}
}

func TestLoadRejectsPartialTransportConfiguration(t *testing.T) {
	t.Setenv("MONITOR_GRPC_ADDRESS", "localhost:8443")
	t.Setenv("MONITOR_TLS_CA_FILE", "")
	if _, err := Load(); err == nil {
		t.Fatal("gRPC without an explicit CA must be rejected")
	}
	t.Setenv("MONITOR_GRPC_ADDRESS", "")
	t.Setenv("MONITOR_TLS_CA_FILE", "")
	t.Setenv("MONITOR_ENROLLMENT_TOKEN", "not-usable-offline")
	if _, err := Load(); err == nil {
		t.Fatal("an enrollment token must not be accepted in offline mode")
	}
}

func TestLoadRejectsUnsafeBounds(t *testing.T) {
	t.Setenv("MONITOR_COLLECTION_INTERVAL", "10ms")
	if _, err := Load(); err == nil {
		t.Fatal("expected too-frequent collection interval to be rejected")
	}
	t.Setenv("MONITOR_COLLECTION_INTERVAL", "500ms")
	t.Setenv("MONITOR_SPOOL_MAX_BYTES", "128")
	if _, err := Load(); err == nil {
		t.Fatal("expected too-small spool quota to be rejected")
	}
}
