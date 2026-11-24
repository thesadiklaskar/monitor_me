package telemetry

import (
	"testing"
	"time"
)

func TestValidateRangeQueryBounds(t *testing.T) {
	now := time.Now().UTC()
	base := RangeQuery{
		OrganizationID: "org", ServerID: "2b6a8d10-9b4d-4e40-9a9c-0c8fc3af3b7f", MetricName: "host.cpu.total",
		From: now.Add(-time.Hour), To: now, Step: time.Second, Limit: 5000,
	}
	if err := ValidateRangeQuery(base, now); err != nil {
		t.Fatalf("valid range should pass: %v", err)
	}
	invalid := base
	invalid.ServerID = "not-a-uuid"
	if err := ValidateRangeQuery(invalid, now); err == nil {
		t.Fatal("invalid server ID must be rejected")
	}
	invalid = base
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
		OrganizationID: "org", ServerID: "2b6a8d10-9b4d-4e40-9a9c-0c8fc3af3b7f", MetricName: "host.cpu.utilization",
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


func TestValidateRangeQueryLabelFilters(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	base := RangeQuery{
		OrganizationID: "org", ServerID: "00000000-0000-4000-8000-000000000001",
		MetricName: "host.disk.used.percent", From: now.Add(-time.Hour), To: now,
		Step: time.Minute, Limit: 120,
	}
	valid := base
	valid.LabelKey = "mountpoint"
	valid.LabelValue = "/"
	if err := ValidateRangeQuery(valid, now); err != nil {
		t.Fatalf("valid label filter rejected: %v", err)
	}
	interfaceFilter := base
	interfaceFilter.LabelKey = "interface"
	interfaceFilter.LabelValue = "enp0s31f6"
	if err := ValidateRangeQuery(interfaceFilter, now); err != nil {
		t.Fatalf("valid network interface filter rejected: %v", err)
	}
	invalid := []RangeQuery{
		{OrganizationID: base.OrganizationID, ServerID: base.ServerID, MetricName: base.MetricName, From: base.From, To: base.To, Step: base.Step, Limit: base.Limit, LabelKey: "mountpoint"},
		{OrganizationID: base.OrganizationID, ServerID: base.ServerID, MetricName: base.MetricName, From: base.From, To: base.To, Step: base.Step, Limit: base.Limit, LabelValue: "/"},
		{OrganizationID: base.OrganizationID, ServerID: base.ServerID, MetricName: base.MetricName, From: base.From, To: base.To, Step: base.Step, Limit: base.Limit, LabelKey: "mountpoint = 'x'", LabelValue: "/"},
		{OrganizationID: base.OrganizationID, ServerID: base.ServerID, MetricName: base.MetricName, From: base.From, To: base.To, Step: base.Step, Limit: base.Limit, LabelKey: "mountpoint", LabelValue: "line1\nline2"},
	}
	for index, query := range invalid {
		if err := ValidateRangeQuery(query, now); err == nil {
			t.Errorf("invalid label filter %d was accepted", index)
		}
	}
}
