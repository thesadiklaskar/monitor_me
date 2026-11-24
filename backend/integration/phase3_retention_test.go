//go:build integration

package integration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"example.com/monitor-me/backend/internal/telemetry"
	_ "github.com/jackc/pgx/v5/stdlib"
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
	insertLogBatch := func(batchID string, age time.Duration) {
		t.Helper()
		payloadHash := sha256.Sum256([]byte("log-" + batchID))
		if _, err := database.ExecContext(ctx, `
			INSERT INTO agent_log_batches (
				organization_id, server_id, agent_id, batch_id, payload_hash, event_count, received_at
			) VALUES ($1, $2, $3, $4, $5, 1, $6)
		`, organizationID, serverID, agentID, batchID, payloadHash[:], now.Add(-age)); err != nil {
			t.Fatalf("insert log idempotency batch %s: %v", batchID, err)
		}
	}
	retainedLogBatchID := "33333333333333333333333333333333"
	expiredLogBatchID := "44444444444444444444444444444444"
	insertLogBatch(retainedLogBatchID, 31*24*time.Hour)
	insertLogBatch(expiredLogBatchID, 100*24*time.Hour)

	policy := telemetry.DefaultRetentionPolicy()
	policy.BatchSize = 100
	result, err := telemetry.PruneBatch(ctx, database, now, policy)
	if err != nil {
		t.Fatalf("prune expired raw telemetry: %v", err)
	}
	if result.SamplesDeleted != 2 || result.BatchesDeleted != 1 || result.LogBatchesDeleted != 1 {
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
	var retainedCount, expiredCount int
	if err := database.QueryRowContext(ctx, `
		SELECT
			count(*) FILTER (WHERE batch_id = $2),
			count(*) FILTER (WHERE batch_id = $3)
		FROM telemetry_batches
		WHERE agent_id = $1
	`, agentID, retainedBatchID, expiredBatchID).Scan(&retainedCount, &expiredCount); err != nil {
		t.Fatalf("verify which idempotency record survived the first sweep: %v", err)
	}
	if retainedCount != 1 || expiredCount != 0 {
		t.Fatalf("first sweep should retain only the 31-day batch: retained=%d expired=%d",
			retainedCount, expiredCount)
	}
	var retainedLogCount, expiredLogCount int
	if err := database.QueryRowContext(ctx, `
		SELECT count(*) FILTER (WHERE batch_id = $2), count(*) FILTER (WHERE batch_id = $3)
		FROM agent_log_batches WHERE agent_id = $1
	`, agentID, retainedLogBatchID, expiredLogBatchID).Scan(&retainedLogCount, &expiredLogCount); err != nil {
		t.Fatalf("verify retained log idempotency records: %v", err)
	}
	if retainedLogCount != 1 || expiredLogCount != 0 {
		t.Fatalf("first sweep should retain only the 31-day log batch: retained=%d expired=%d", retainedLogCount, expiredLogCount)
	}

	// The retention sweep is global, so unrelated telemetry created by other
	// tests may also expire when the simulated clock advances. Assert only this
	// test agent's remaining idempotency records rather than the global delete count.
	result, err = telemetry.PruneBatch(ctx, database, now.Add(91*24*time.Hour), policy)
	if err != nil {
		t.Fatalf("prune expired idempotency record: %v", err)
	}
	if result.BatchesDeleted < 1 || result.LogBatchesDeleted < 1 {
		t.Fatalf("expected the simulated 91-day sweep to delete at least one batch: %+v", result)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT count(*) FROM telemetry_batches WHERE agent_id = $1
	`, agentID).Scan(&batchCount); err != nil {
		t.Fatalf("count expired test idempotency records: %v", err)
	}
	if batchCount != 0 {
		t.Fatalf("the retained 90-day idempotency record should expire after 91 days, got %d remaining",
			batchCount)
	}
	var logBatchCount int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM agent_log_batches WHERE agent_id = $1`, agentID).Scan(&logBatchCount); err != nil {
		t.Fatalf("count expired log idempotency records: %v", err)
	}
	if logBatchCount != 0 {
		t.Fatalf("the retained 90-day log idempotency record should expire after 91 days, got %d remaining", logBatchCount)
	}
}
