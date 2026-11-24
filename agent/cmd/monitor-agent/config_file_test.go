package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFileArgument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.env")
	if err := os.WriteFile(path, []byte(
		"MONITOR_GRPC_ADDRESS="control.example:8443"\n"+
			"MONITOR_AGENT_STATE_FILE="C:\\\\ProgramData\\\\MonitorMe\\\\agent-state.json"\n"+
			"MONITOR_ENROLLMENT_TOKEN="token-value"\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}

	keys := []string{"MONITOR_GRPC_ADDRESS", "MONITOR_AGENT_STATE_FILE", "MONITOR_ENROLLMENT_TOKEN"}
	previous := make(map[string]string, len(keys))
	wasSet := make(map[string]bool, len(keys))
	for _, key := range keys {
		previous[key], wasSet[key] = os.LookupEnv(key)
		_ = os.Unsetenv(key)
	}
	t.Cleanup(func() {
		for _, key := range keys {
			if wasSet[key] {
				_ = os.Setenv(key, previous[key])
			} else {
				_ = os.Unsetenv(key)
			}
		}
	})

	oldArgs := os.Args
	os.Args = []string{"monitor-agent", "--config-file", path}
	t.Cleanup(func() { os.Args = oldArgs })

	if err := loadConfigFileArgument(); err != nil {
		t.Fatalf("loadConfigFileArgument() error = %v", err)
	}
	if got := os.Getenv("MONITOR_GRPC_ADDRESS"); got != "control.example:8443" {
		t.Fatalf("MONITOR_GRPC_ADDRESS = %q", got)
	}
	if got := os.Getenv("MONITOR_AGENT_STATE_FILE"); got != `C:\ProgramData\MonitorMe\agent-state.json` {
		t.Fatalf("MONITOR_AGENT_STATE_FILE = %q", got)
	}
	if got := os.Getenv("MONITOR_ENROLLMENT_TOKEN"); got != "token-value" {
		t.Fatalf("MONITOR_ENROLLMENT_TOKEN = %q", got)
	}
}

func TestLoadConfigFileArgumentRejectsMissingPath(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"monitor-agent", "--config-file"}
	t.Cleanup(func() { os.Args = oldArgs })
	if err := loadConfigFileArgument(); err == nil {
		t.Fatal("expected --config-file without a path to be rejected")
	}
}

func TestLoadConfigFileArgumentRejectsMalformedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.env")
	if err := os.WriteFile(path, []byte("NOT_A_VALID_LINE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldArgs := os.Args
	os.Args = []string{"monitor-agent", "--config-file", path}
	t.Cleanup(func() { os.Args = oldArgs })
	if err := loadConfigFileArgument(); err == nil {
		t.Fatal("expected malformed config line to be rejected")
	}
}
