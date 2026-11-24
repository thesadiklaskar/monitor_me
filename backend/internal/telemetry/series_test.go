package telemetry

import (
	"testing"
	"time"
)

func TestValidateRangeQueryBounds(t *testing.T) {
	now := time.Now().UTC()
	base := RangeQuery{
		OrganizationID: "org", ServerID: "server", MetricName: "host.cpu.total",
		From: now.Add(-time.Hour), To: now, Step: time.Second, Limit: 5000,
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
	invalid = base
	invalid.Step = 100 * time.Millisecond
	invalid.Limit = 10
	if err := ValidateRangeQuery(invalid, now); err == nil {
		t.Fatal("bucket grids larger than the output limit must be rejected rather than silently truncated")
	}
}

func TestSeriesBucketCountUsesEpochAlignedBoundaries(t *testing.T) {
	step := time.Second
	tests := []struct {
		name string
		from time.Time
		to   time.Time
		want int64
	}{
		{
			name: "aligned one bucket",
			from: time.Unix(0, 0),
			to:   time.Unix(1, 0),
			want: 1,
		},
		{
			name: "offset range crosses two buckets",
			from: time.Unix(0, 100_000_000),
			to:   time.Unix(1, 100_000_000),
			want: 2,
		},
		{
			name: "exclusive endpoint on next boundary",
			from: time.Unix(0, 0),
			to:   time.Unix(1, 1),
			want: 2,
		},
		{
			name: "negative epoch floor division",
			from: time.Unix(-2, 0),
			to:   time.Unix(-1, 0),
			want: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := seriesBucketCount(test.from, test.to, step); got != test.want {
				t.Fatalf("seriesBucketCount() = %d, want %d", got, test.want)
			}
		})
	}

	query := RangeQuery{
		OrganizationID: "org", ServerID: "server", MetricName: "host.cpu.utilization",
		From: time.Unix(0, 100_000_000),
		To:   time.Unix(1, 100_000_000),
		Step: step, Limit: 1,
	}
	if err := ValidateRangeQuery(query, time.Unix(2, 0)); err == nil {
		t.Fatal("offset two-bucket range must not pass a one-point limit")
	}
	query.Limit = 2
	if err := ValidateRangeQuery(query, time.Unix(2, 0)); err != nil {
		t.Fatalf("two-point limit should accept the offset range: %v", err)
	}
}

