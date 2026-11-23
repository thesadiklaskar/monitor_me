package telemetry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	redisv9 "github.com/redis/go-redis/v9"
)

const maxLiveMetricPoints = 128

type EventPublisher interface {
	Publish(context.Context, string, any) error
}

type RedisPublisher struct {
	Client redisv9.Cmdable
}

func (publisher RedisPublisher) Publish(ctx context.Context, channel string, payload any) error {
	if publisher.Client == nil {
		return fmt.Errorf("live event Redis publisher is not configured")
	}
	return publisher.Client.Publish(ctx, channel, payload).Err()
}

type LiveSample struct {
	MetricName          string            `json:"metric_name"`
	Value               float64           `json:"value"`
	Unit                string            `json:"unit"`
	Labels              map[string]string `json:"labels,omitempty"`
	ProjectID           string            `json:"project_id,omitempty"`
	ObservedAtUnixNanos int64             `json:"observed_at_unix_nanos"`
}

type LiveEvent struct {
	Type                string       `json:"type"`
	OrganizationID      string       `json:"organization_id"`
	ServerID            string       `json:"server_id"`
	AgentID             string       `json:"agent_id"`
	BatchID             string       `json:"batch_id"`
	ReceivedAtUnixNanos int64        `json:"received_at_unix_nanos"`
	Samples             []LiveSample `json:"samples"`
}

// PublishCommittedBatch emits latest metric points only after PostgreSQL commit.
// Redis Pub/Sub is an ephemeral notification path; durable history remains in PostgreSQL.
func PublishCommittedBatch(ctx context.Context, publisher EventPublisher, organizationID, serverID, agentID, batchID string, samples []LiveSample) error {
	if publisher == nil {
		return nil
	}
	latest := make(map[string]LiveSample)
	for _, sample := range samples {
		keys := make([]string, 0, len(sample.Labels))
		for key := range sample.Labels {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		labels := make(map[string]string)
		for index, key := range keys {
			if index >= 12 {
				break
			}
			value := sample.Labels[key]
			if len(value) > 128 {
				value = value[:128]
			}
			labels[key] = value
		}
		labelIdentity, err := json.Marshal(labels)
		if err != nil {
			return fmt.Errorf("encode live metric labels: %w", err)
		}
		sample.Labels = labels
		latest[sample.MetricName+sample.ProjectID+string(labelIdentity)] = sample
	}
	keys := make([]string, 0, len(latest))
	for key := range latest {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	points := make([]LiveSample, 0, maxLiveMetricPoints)
	for _, key := range keys {
		if len(points) >= maxLiveMetricPoints {
			break
		}
		points = append(points, latest[key])
	}
	event := LiveEvent{
		Type: "telemetry.batch", OrganizationID: organizationID,
		ServerID: serverID, AgentID: agentID, BatchID: batchID,
		ReceivedAtUnixNanos: time.Now().UTC().UnixNano(), Samples: points,
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode live telemetry event: %w", err)
	}
	if len(encoded) > 256<<10 {
		return fmt.Errorf("live telemetry event exceeds the 256KiB publication limit")
	}
	return publisher.Publish(ctx, LiveChannel(organizationID), string(encoded))
}

// LiveChannel isolates Redis Pub/Sub events by organization without exposing
// the raw organization identifier in Redis channel names.
func LiveChannel(organizationID string) string {
	digest := sha256.Sum256([]byte(organizationID))
	return "monitor:live:organization:" + hex.EncodeToString(digest[:16])
}
