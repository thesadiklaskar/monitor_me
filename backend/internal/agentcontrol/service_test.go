package agentcontrol

import (
	"testing"

	monitorv1 "github.com/WorkRCS/monitor_me/backend/gen/monitor/v1"
)

func TestNormalizeCapabilities(t *testing.T) {
	got, err := normalizeCapabilities([]string{"host.cpu", "host.memory", "host.cpu"})
	if err != nil {
		t.Fatalf("normalizeCapabilities() error = %v", err)
	}
	if len(got) != 2 || got[0] != "host.cpu" || got[1] != "host.memory" {
		t.Fatalf("unexpected normalized capabilities: %#v", got)
	}
}

func TestNormalizeCapabilitiesRejectsInvalidInput(t *testing.T) {
	if _, err := normalizeCapabilities([]string{"contains spaces"}); err == nil {
		t.Fatal("expected invalid capability to be rejected")
	}
	tooMany := make([]string, 65)
	for index := range tooMany {
		tooMany[index] = "feature"
	}
	if _, err := normalizeCapabilities(tooMany); err == nil {
		t.Fatal("expected excessive capability count to be rejected")
	}
}

func TestValidEnrollmentRequest(t *testing.T) {
	request := &monitorv1.EnrollRequest{
		EnrollmentToken: "0123456789012345678901234567890123456789012",
		AgentVersion:    "0.1.0",
		OperatingSystem: "linux",
		Architecture:    "amd64",
		Hostname:        "node-01.example",
	}
	if !validEnrollmentRequest(request) {
		t.Fatal("expected supported Linux enrollment request to be accepted")
	}
	request.OperatingSystem = "unknown"
	if validEnrollmentRequest(request) {
		t.Fatal("expected unsupported operating system to be rejected")
	}
}
