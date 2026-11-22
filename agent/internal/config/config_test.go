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
