//go:build integration

package integration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/WorkRCS/monitor_me/backend/internal/telemetry"
)

func TestPostgresRetentionKeepsIdempotencyRecordsLongerThanSamples(t *testing.T) {
	databaseURL := requiredEnv(t, "TEST_DATABASE_URL")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open retention integration database: %v", err)
	}
	defer database.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var organizationID, serverID, agentID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO organizations (name, slug)
		VALUES ('Phase Three Retention Test', $1)
		RETURNING id::text
	`, "phase3-retention-"+suffix).Scan(&organizationID); err != nil {
		t.Fatalf("create retention organization: %v", err)
	}
	defer database.ExecContext(context.Background(),
		`DELETE FROM organizations WHERE id = $1`, organizationID)

	if err := database.QueryRowContext(ctx, `
		INSERT INTO servers (organization_id, name, hostname, status)
		VALUES ($1, 'retention-node', 'retention-node', 'pending')
		RETURNING id::text
	`, organizationID).Scan(&serverID); err != nil {
		t.Fatalf("create retention server: %v", err)
	}
	if err := database.QueryRowContext(ctx, `
		INSERT INTO agents (server_id, version, status)
		VALUES ($1, 'retention-test', 'online')
		RETURNING id::text
	`, serverID).Scan(&agentID); err != nil {
		t.Fatalf("create retention agent: %v", err)
	}

	now := time.Now().UTC()
	retainedBatchID := "11111111111111111111111111111111"
	expiredBatchID := "22222222222222222222222222222222"
	insertBatch := func(batchID string, age time.Duration) {
		t.Helper()
		collectedAt := now.Add(-age)
		payloadHash := sha256.Sum256([]byte(batchID))
		if _, err := database.ExecContext(ctx, `
			INSERT INTO telemetry_batches (
				agent_id, batch_id, schema_version, payload_hash, collected_at,
				received_at, sample_count
			)
			VALUES ($1, $2, 1, $3, $4, $4, 1)
		`, agentID, batchID, payloadHash[:], collectedAt); err != nil {
			t.Fatalf("insert telemetry batch %s: %v", batchID, err)
		}
		if _, err := database.ExecContext(ctx, `
			INSERT INTO telemetry_samples (
				organization_id, server_id, agent_id, batch_id, metric_name,
				value, unit, labels, observed_at, attribution_method
			)
			VALUES ($1, $2, $3, $4, 'host.cpu.utilization',
			        42, 'percent', '{}'::jsonb, $5, 'host_total')
		`, organizationID, serverID, agentID, batchID, collectedAt); err != nil {
			t.Fatalf("insert telemetry sample for batch %s: %v", batchID, err)
		}
	}

	insertBatch(retainedBatchID, 31*24*time.Hour)
	insertBatch(expiredBatchID, 100*24*time.Hour)

	policy := telemetry.DefaultRetentionPolicy()
	policy.BatchSize = 100
	result, err := telemetry.PruneBatch(ctx, database, now, policy)
	if err != nil {
		t.Fatalf("prune expired raw telemetry: %v", err)
	}
	if result.SamplesDeleted != 2 || result.BatchesDeleted != 1 {
		t.Fatalf("unexpected first retention pass: %+v", result)
	}

	var sampleCount, batchCount int
	if err := database.QueryRowContext(ctx, `
		SELECT count(*) FROM telemetry_samples WHERE agent_id = $1
	`, agentID).Scan(&sampleCount); err != nil {
		t.Fatalf("count retained metric samples: %v", err)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT count(*) FROM telemetry_batches WHERE agent_id = $1
	`, agentID).Scan(&batchCount); err != nil {
		t.Fatalf("count retained idempotency records: %v", err)
	}
	if sampleCount != 0 || batchCount != 1 {
		t.Fatalf("raw samples should expire before idempotency keys: samples=%d batches=%d",
			sampleCount, batchCount)
	}

	result, err = telemetry.PruneBatch(ctx, database, now.Add(91*24*time.Hour), policy)
	if err != nil {
		t.Fatalf("prune expired idempotency record: %v", err)
	}
	if result.BatchesDeleted != 1 {
		t.Fatalf("90-day idempotency record was not removed after its retention age: %+v", result)
	}
}
