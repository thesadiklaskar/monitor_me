package telemetry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	minimumSeriesStep   = 100 * time.Millisecond
	maximumSeriesStep   = 24 * time.Hour
	maximumSeriesRange  = 30 * 24 * time.Hour
	maximumSeriesPoints = 5000
)

var (
	seriesUUID       = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$")
	seriesMetricName = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_.-]{0,127}$")
	seriesLabelKey  = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_.-]{0,63}$")
)

type RangeQuery struct {
	OrganizationID string
	ServerID       string
	MetricName     string
	LabelKey       string
	LabelValue     string
	From           time.Time
	To             time.Time
	Step           time.Duration
	Limit          int
}

type SeriesPoint struct {
	Bucket      time.Time `json:"bucket"`
	Average     float64   `json:"average"`
	Minimum     float64   `json:"minimum"`
	Maximum     float64   `json:"maximum"`
	SampleCount int64     `json:"sample_count"`
}

func ValidateRangeQuery(query RangeQuery, now time.Time) error {
	if query.OrganizationID == "" || !seriesUUID.MatchString(query.ServerID) || !seriesMetricName.MatchString(query.MetricName) {
		return errors.New("organization, server, and valid metric name are required")
	}
	if (query.LabelKey == "") != (query.LabelValue == "") ||
		(query.LabelKey != "" && (!seriesLabelKey.MatchString(query.LabelKey) ||
			len(query.LabelValue) > 256 || strings.ContainsAny(query.LabelValue, "\\r\\n\\x00"))) {
		return errors.New("metric label filters require a valid key and a bounded single-line value")
	}
	if query.From.IsZero() || query.To.IsZero() || !query.From.Before(query.To) ||
		query.To.After(now.Add(time.Minute)) || query.To.Sub(query.From) > maximumSeriesRange {
		return errors.New("series time range is invalid or exceeds 30 days")
	}
	if query.Step < minimumSeriesStep || query.Step > maximumSeriesStep {
		return errors.New("series step must be between 100ms and 24h")
	}
	if query.Limit < 1 || query.Limit > maximumSeriesPoints {
		return errors.New("series limit must be between 1 and 5000")
	}
	// Reject oversized bucket grids instead of scanning a long range and silently
	// truncating the response at LIMIT.
	requiredPoints := seriesBucketCount(query.From, query.To, query.Step)
	if requiredPoints > int64(query.Limit) {
		return errors.New("series range and step would exceed the requested point limit")
	}
	return nil
}

// seriesBucketCount mirrors PostgreSQL date_bin's epoch-aligned bucket boundaries.
func seriesBucketCount(from, to time.Time, step time.Duration) int64 {
	stepNanos := int64(step)
	firstBucket := floorDivision(from.UnixNano(), stepNanos)
	lastBucket := floorDivision(to.Add(-time.Nanosecond).UnixNano(), stepNanos)
	return lastBucket - firstBucket + 1
}

func floorDivision(value, divisor int64) int64 {
	quotient, remainder := value/divisor, value%divisor
	if remainder != 0 && value < 0 {
		quotient--
	}
	return quotient
}

// QueryRange uses PostgreSQL date_bin for bounded server-side aggregation.
// Tenant and server scopes are required so callers cannot query by metric name alone.
func QueryRange(ctx context.Context, db *sql.DB, query RangeQuery) ([]SeriesPoint, error) {
	if err := ValidateRangeQuery(query, time.Now().UTC()); err != nil {
		return nil, err
	}
	interval := fmt.Sprintf("%d microseconds", query.Step.Microseconds())
	rows, err := db.QueryContext(ctx, `
		SELECT
			date_bin($1::interval, observed_at, to_timestamp(0)) AS bucket,
			avg(value), min(value), max(value), count(*)
		FROM telemetry_samples
		WHERE organization_id = $2
		  AND server_id = $3
		  AND metric_name = $4
		  AND observed_at >= $5
		  AND observed_at < $6
		  AND ($7 = '' OR labels ->> $7 = $8)
		GROUP BY bucket
		ORDER BY bucket
		LIMIT $9
	`, interval, query.OrganizationID, query.ServerID, query.MetricName,
		query.From.UTC(), query.To.UTC(), query.LabelKey, query.LabelValue, query.Limit)
	if err != nil {
		return nil, fmt.Errorf("query aggregated telemetry series: %w", err)
	}
	defer rows.Close()

	points := make([]SeriesPoint, 0)
	for rows.Next() {
		var point SeriesPoint
		if err := rows.Scan(&point.Bucket, &point.Average, &point.Minimum, &point.Maximum, &point.SampleCount); err != nil {
			return nil, fmt.Errorf("scan aggregated telemetry point: %w", err)
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate aggregated telemetry series: %w", err)
	}
	return points, nil
}

