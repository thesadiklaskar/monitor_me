package health

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestCollectorErrorCount(t *testing.T) {
	joined := errors.Join(
		errors.New("procfs memory read failed"),
		fmt.Errorf("disk collector: %w", errors.New("malformed disk counters")),
		nil,
	)
	if got := CollectorErrorCount(nil); got != 0 {
		t.Fatalf("nil collector error count = %d, want 0", got)
	}
	if got := CollectorErrorCount(joined); got != 2 {
		t.Fatalf("joined collector error count = %d, want 2", got)
	}
	if got := CollectorErrorCount(fmt.Errorf("wrapped: %w", errors.New("network read failed"))); got != 1 {
		t.Fatalf("wrapped collector error count = %d, want 1", got)
	}
}

func TestHealthSamplesUseDurableQueueStatsAndOmitUnknownLastSend(t *testing.T) {
	state := New()
	now := time.Unix(1_800_000_000, 0).UTC()
	samples := state.Samples(now, 2, 4096, 3)
	if len(samples) != 3 {
		t.Fatalf("health samples before first successful send = %d, want 3", len(samples))
	}
	got := make(map[string]float64, len(samples))
	for _, sample := range samples {
		got[sample.Name] = sample.Value
		if sample.ObservedAtUnixNanos != now.UnixNano() {
			t.Fatalf("sample %s has wrong observation timestamp", sample.Name)
		}
	}
	if got["agent.collector.errors"] != 2 || got["agent.spool.bytes"] != 4096 || got["agent.queue.depth"] != 3 {
		t.Fatalf("unexpected health sample values: %v", got)
	}
	if _, exists := got["agent.last_successful_send"]; exists {
		t.Fatal("last successful send must be missing until a backend ACK is accepted")
	}

	lastSend := now.Add(-5 * time.Second)
	state.MarkSuccessfulSend(lastSend)
	samples = state.Samples(now, 0, 2048, 1)
	for _, sample := range samples {
		if sample.Name == "agent.last_successful_send" {
			if sample.Unit != "unix_seconds" || sample.Value != float64(lastSend.UnixNano())/float64(time.Second) {
				t.Fatalf("unexpected last successful send sample: %+v", sample)
			}
			return
		}
	}
	t.Fatal("last successful send sample was not emitted after a durable ACK")
}
