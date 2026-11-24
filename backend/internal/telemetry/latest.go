package telemetry

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	latestMetricWindow = 15 * time.Minute
	latestMetricLimit  = 512
)

type LatestQuery struct {
	OrganizationID string
	ServerID       string
}

type LatestSample struct {
	MetricName string            `json:"metric_name"`
	Value      float64           `json:"value"`
	Unit       string            `json:"unit"`
	Labels     map[string]string `json:"labels,omitempty"`
	ObservedAt time.Time         `json:"observed_at"`
}

func ValidateLatestQuery(query LatestQuery) error {
	if query.OrganizationID == "" || !seriesUUID.MatchString(query.ServerID) {
		return errors.New("organization and valid server ID are required")
	}
	return nil
}

// QueryLatest returns a bounded recent observation per metric/label identity.
// This supports point-in-time process, TCP state and listening-port inventory.
func QueryLatest(ctx context.Context, db *sql.DB, query LatestQuery) ([]LatestSample, error) {
	if err := ValidateLatestQuery(query); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT ON (metric_name, labels)
			metric_name, value, unit, labels, observed_at
		FROM telemetry_samples
		WHERE organization_id = $1
		  AND server_id = $2
		  AND observed_at >= now() - $3::interval
		ORDER BY metric_name, labels, observed_at DESC
		LIMIT $4
	`, query.OrganizationID, query.ServerID, fmt.Sprintf("%d seconds", int(latestMetricWindow.Seconds())),
		latestMetricLimit)
	if err != nil {
		return nil, fmt.Errorf("query latest telemetry inventory: %w", err)
	}
	defer rows.Close()

	samples := make([]LatestSample, 0)
	for rows.Next() {
		var sample LatestSample
		var labelsJSON []byte
		if err := rows.Scan(&sample.MetricName, &sample.Value, &sample.Unit, &labelsJSON, &sample.ObservedAt); err != nil {
			return nil, fmt.Errorf("scan latest telemetry inventory: %w", err)
		}
		if len(labelsJSON) > 0 {
			if err := json.Unmarshal(labelsJSON, &sample.Labels); err != nil {
				return nil, fmt.Errorf("decode latest telemetry labels: %w", err)
			}
		}
		samples = append(samples, sample)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latest telemetry inventory: %w", err)
	}
	return samples, nil
}
