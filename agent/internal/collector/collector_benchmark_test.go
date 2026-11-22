package collector

import (
	"context"
	"testing"
	"time"
)

func BenchmarkProcCollectorCollect(b *testing.B) {
	hostCollector := NewProcCollector("/proc")
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := hostCollector.Collect(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCounterRate(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := CounterRate(1000, 1125, time.Second); !ok {
			b.Fatal("valid counter delta was rejected")
		}
	}
}
