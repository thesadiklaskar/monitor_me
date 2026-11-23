package operations

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteRequiresLocalAllowlist(t *testing.T) {
	result := Execute(context.Background(), "restart_service", "sshd.service", []string{"monitor-me.service"})
	if result.Status != "failed" || !strings.Contains(result.Output, "allowlist") {
		t.Fatalf("unallowlisted service should be rejected: %+v", result)
	}
}

func TestExecuteInvokesOnlyTheFixedSystemctlArguments(t *testing.T) {
	directory := t.TempDir()
	logPath := filepath.Join(directory, "arguments.txt")
	binary := filepath.Join(directory, "systemctl")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SYSTEMCTL_ARGUMENT_LOG\"\nprintf 'service action accepted\\n'\n"
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	t.Setenv("SYSTEMCTL_ARGUMENT_LOG", logPath)

	result := Execute(context.Background(), "restart_service", "monitor-me-worker.service", []string{"monitor-me-worker.service"})
	if result.Status != "succeeded" {
		t.Fatalf("allowlisted systemctl operation failed: %+v", result)
	}
	arguments, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(arguments) != "restart\nmonitor-me-worker.service\n" {
		t.Fatalf("unexpected systemctl arguments: %q", arguments)
	}
}

func TestExecuteRejectsUnsupportedOperationAndUnsafeUnit(t *testing.T) {
	for _, item := range []struct{ operation, unit string }{
		{operation: "shell", unit: "monitor-me.service"},
		{operation: "restart_service", unit: "monitor-me.service;id"},
		{operation: "stop_service", unit: "/tmp/evil.service"},
	} {
		result := Execute(context.Background(), item.operation, item.unit, []string{"monitor-me.service"})
		if result.Status != "failed" {
			t.Errorf("unsafe operation was not rejected: %#v", item)
		}
	}
}
