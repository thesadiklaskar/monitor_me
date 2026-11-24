package agentcontrol

import (
	"strings"
	"testing"
	"time"

	monitorv1 "monitor-me.internal/backend/gen/monitor/v1"
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

func TestValidateTelemetryBatchRequiresCanonicalLowercaseBatchID(t *testing.T) {
	now := time.Now().UTC().UnixNano()
	newBatch := func(id string) *monitorv1.TelemetryBatch {
		return &monitorv1.TelemetryBatch{
			BatchId: id, CollectedAtUnixNanos: now, SchemaVersion: 1,
			Samples: []*monitorv1.MetricSample{{
				MetricName: "host.cpu.utilization", Value: 42, Unit: "percent",
				ObservedAtUnixNanos: now,
			}},
		}
	}

	const validID = "abcdef0123456789abcdef0123456789"
	if err := validateTelemetryBatch(newBatch(validID)); err != nil {
		t.Fatalf("canonical lowercase ID should pass validation: %v", err)
	}
	if err := validateTelemetryBatch(newBatch(strings.ToUpper(validID))); err == nil {
		t.Fatal("uppercase hexadecimal IDs must be rejected before they reach the lowercase PostgreSQL constraint")
	}
	if err := validateTelemetryBatch(newBatch("abcdef0123456789abcdef012345678g")); err == nil {
		t.Fatal("non-hexadecimal batch IDs must be rejected")
	}
}
