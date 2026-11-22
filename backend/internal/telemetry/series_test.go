package telemetry

import (
	"testing"
	"time"
)

func TestValidateRangeQueryBounds(t *testing.T) {
	now := time.Now().UTC()
	base := RangeQuery{
		OrganizationID: "org", ServerID: "server", MetricName: "host.cpu.total",
		From: now.Add(-time.Hour), To: now, Step: time.Second, Limit: 1000,
	}
	if err := ValidateRangeQuery(base, now); err != nil {
		t.Fatalf("valid range should pass: %v", err)
	}
	invalid := base
	invalid.Step = time.Millisecond
	if err := ValidateRangeQuery(invalid, now); err == nil {
		t.Fatal("sub-100ms aggregation step must be rejected")
	}
	invalid = base
	invalid.To = now.Add(2 * time.Hour)
	if err := ValidateRangeQuery(invalid, now); err == nil {
		t.Fatal("future range must be rejected")
	}
	invalid = base
	invalid.From = now.Add(-31 * 24 * time.Hour)
	if err := ValidateRangeQuery(invalid, now); err == nil {
		t.Fatal("range beyond raw retention must be rejected")
	}
}
