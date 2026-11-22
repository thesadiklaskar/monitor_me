package spool

import (
	"testing"
	"time"

	"github.com/WorkRCS/monitor_me/agent/internal/collector"
)

func BenchmarkSpoolAppend(b *testing.B) {
	store, err := Open(b.TempDir(), 1<<30)
	if err != nil {
		b.Fatal(err)
	}
	sample := collector.Sample{
		Name:                "host.cpu.utilization",
		Value:               42.5,
		Unit:                "percent",
		Labels:              map[string]string{"cpu": "cpu0"},
		ObservedAtUnixNanos: time.Now().UnixNano(),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch, err := NewBatch([]collector.Sample{sample}, time.Now().UnixNano())
		if err != nil {
			b.Fatal(err)
		}
		if err := store.Append(batch); err != nil {
			b.Fatal(err)
		}
	}
}
