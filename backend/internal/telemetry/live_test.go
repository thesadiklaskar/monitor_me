package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"
)

type capturedPublisher struct {
	channel string
	payload string
}

func (publisher *capturedPublisher) Publish(_ context.Context, channel string, payload any) error {
	publisher.channel = channel
	publisher.payload = payload.(string)
	return nil
}

func TestPublishCommittedBatchBoundsAndScopesLivePayload(t *testing.T) {
	publisher := &capturedPublisher{}
	samples := make([]LiveSample, 0, 200)
	for index := 0; index < 200; index++ {
		samples = append(samples, LiveSample{
			MetricName: "process.memory.rss",
			Value: float64(index),
			Unit: "bytes",
			Labels: map[string]string{"pid": "123", "process": "worker"},
			ObservedAtUnixNanos: time.Now().UnixNano(),
		})
	}
	if err := PublishCommittedBatch(context.Background(), publisher,
		"organization-1", "server-1", "agent-1", "0123456789abcdef0123456789abcdef", samples); err != nil {
		t.Fatalf("PublishCommittedBatch() error = %v", err)
	}
	var event LiveEvent
	if err := json.Unmarshal([]byte(publisher.payload), &event); err != nil {
		t.Fatalf("decode live event: %v", err)
	}
	if publisher.channel != LiveChannel("organization-1") {
		t.Fatalf("unexpected Redis channel %q", publisher.channel)
	}
	if event.OrganizationID != "organization-1" || event.ServerID != "server-1" ||
		event.AgentID != "agent-1" || event.BatchID != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("live payload escaped its organization or batch scope: %+v", event)
	}
	if len(event.Samples) != 1 || event.Samples[0].Value != 199 {
		t.Fatalf("latest duplicate series should collapse to one bounded point: %+v", event.Samples)
	}
}

func BenchmarkPublishCommittedBatch(b *testing.B) {
	publisher := &capturedPublisher{}
	samples := make([]LiveSample, 0, 96)
	for index := 0; index < 96; index++ {
		samples = append(samples, LiveSample{
			MetricName: "host.cpu.core.utilization",
			Value: float64(index),
			Unit: "percent",
			Labels: map[string]string{"cpu": "cpu" + strconv.Itoa(index)},
			ObservedAtUnixNanos: time.Now().UnixNano(),
		})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batchID := fmt.Sprintf("%032x", i+1)
		if err := PublishCommittedBatch(context.Background(), publisher,
			"organization-1", "server-1", "agent-1", batchID, samples); err != nil {
			b.Fatal(err)
		}
	}
}
