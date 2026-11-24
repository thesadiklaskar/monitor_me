package health

import (
	"sync/atomic"
	"time"

	"monitor-me.internal/agent/internal/collector"
)

// State stores process-lifetime delivery state shared by collection and
// transport goroutines. An atomic timestamp avoids introducing a data race.
type State struct {
	lastSuccessfulSendUnixNanos atomic.Int64
}

func New() *State { return &State{} }

func (state *State) MarkSuccessfulSend(at time.Time) {
	if state != nil && !at.IsZero() {
		state.lastSuccessfulSendUnixNanos.Store(at.UTC().UnixNano())
	}
}

// CollectorErrorCount counts leaf failures in wrapped and joined errors.
func CollectorErrorCount(err error) int {
	if err == nil {
		return 0
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		total := 0
		for _, child := range joined.Unwrap() {
			total += CollectorErrorCount(child)
		}
		return total
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return CollectorErrorCount(wrapped.Unwrap())
	}
	return 1
}

// Samples builds health metrics. Queue depth is the count of unacknowledged
// batches in the persistent WAL; there is no separate in-memory queue.
func (state *State) Samples(now time.Time, collectorErrors int, spoolBytes int64, pendingBatches int) []collector.Sample {
	if now.IsZero() {
		return nil
	}
	observedAt := now.UTC().UnixNano()
	samples := []collector.Sample{
		{Name: "agent.collector.errors", Value: float64(collectorErrors), Unit: "errors/interval", ObservedAtUnixNanos: observedAt},
		{Name: "agent.queue.depth", Value: float64(pendingBatches), Unit: "batches", ObservedAtUnixNanos: observedAt},
		{Name: "agent.spool.bytes", Value: float64(spoolBytes), Unit: "bytes", ObservedAtUnixNanos: observedAt},
	}
	if state != nil {
		lastSuccessfulSend := state.lastSuccessfulSendUnixNanos.Load()
		if lastSuccessfulSend > 0 {
			samples = append(samples, collector.Sample{
				Name:                "agent.last_successful_send",
				Value:               float64(lastSuccessfulSend) / float64(time.Second),
				Unit:                "unix_seconds",
				ObservedAtUnixNanos: observedAt,
			})
		}
	}
	return samples
}
