package telemetry

import (
	"testing"
	"time"
)

func TestRetentionPolicyDefaultsAndRejectsUnsafeValues(t *testing.T) {
	policy := DefaultRetentionPolicy()
	if err := policy.Validate(); err != nil {
		t.Fatalf("default retention policy should be valid: %v", err)
	}
	if policy.SampleAge != 30*24*time.Hour || policy.IdempotencyAge != 90*24*time.Hour {
		t.Fatalf("unexpected default retention periods: %+v", policy)
	}
	policy.IdempotencyAge = policy.SampleAge
	if err := policy.Validate(); err == nil {
		t.Fatal("idempotency records must outlive raw samples by at least 2x")
	}
	policy = DefaultRetentionPolicy()
	policy.BatchSize = 0
	if err := policy.Validate(); err == nil {
		t.Fatal("retention batch size must be bounded above zero")
	}
}
