package telemetry

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

type RetentionPolicy struct {
	SampleAge      time.Duration
	IdempotencyAge time.Duration
	BatchSize      int
}

type RetentionResult struct {
	SamplesDeleted     int64
	BatchesDeleted     int64
	LogBatchesDeleted  int64
	LiveTicketsDeleted int64
}

func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		SampleAge:      30 * 24 * time.Hour,
		IdempotencyAge: 90 * 24 * time.Hour,
		BatchSize:      5000,
	}
}

func (policy RetentionPolicy) Validate() error {
	if policy.SampleAge < 24*time.Hour || policy.SampleAge > 3650*24*time.Hour {
		return fmt.Errorf("sample retention must be between 1 and 3650 days")
	}
	if policy.IdempotencyAge < policy.SampleAge*2 || policy.IdempotencyAge > 10*365*24*time.Hour {
		return fmt.Errorf("idempotency retention must be at least twice sample retention and at most 10 years")
	}
	if policy.BatchSize < 1 || policy.BatchSize > 100000 {
		return fmt.Errorf("retention batch size must be between 1 and 100000")
	}
	return nil
}

// PruneBatch deletes a bounded number of expired samples and old idempotency
// records. Keeping batch hashes longer than raw samples prevents late retries
// from reintroducing data that has already passed the retention window.
func PruneBatch(ctx context.Context, db *sql.DB, now time.Time, policy RetentionPolicy) (RetentionResult, error) {
	if err := policy.Validate(); err != nil {
		return RetentionResult{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return RetentionResult{}, fmt.Errorf("begin telemetry retention transaction: %w", err)
	}
	defer tx.Rollback()

	sampleResult, err := tx.ExecContext(ctx, `
		WITH expired AS (
			SELECT id
			FROM telemetry_samples
			WHERE observed_at < $1
			ORDER BY observed_at
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		DELETE FROM telemetry_samples sample
		USING expired
		WHERE sample.id = expired.id
	`, now.Add(-policy.SampleAge), policy.BatchSize)
	if err != nil {
		return RetentionResult{}, fmt.Errorf("delete expired telemetry samples: %w", err)
	}
	samplesDeleted, err := sampleResult.RowsAffected()
	if err != nil {
		return RetentionResult{}, fmt.Errorf("count expired telemetry samples: %w", err)
	}

	batchResult, err := tx.ExecContext(ctx, `
		WITH expired AS (
			SELECT batch.agent_id, batch.batch_id
			FROM telemetry_batches batch
			WHERE batch.received_at < $1
			  AND NOT EXISTS (
				SELECT 1 FROM telemetry_samples sample
				WHERE sample.agent_id = batch.agent_id AND sample.batch_id = batch.batch_id
			  )
			ORDER BY batch.received_at
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		DELETE FROM telemetry_batches batch
		USING expired
		WHERE batch.agent_id = expired.agent_id AND batch.batch_id = expired.batch_id
	`, now.Add(-policy.IdempotencyAge), policy.BatchSize)
	if err != nil {
		return RetentionResult{}, fmt.Errorf("delete expired telemetry idempotency records: %w", err)
	}
	batchesDeleted, err := batchResult.RowsAffected()
	if err != nil {
		return RetentionResult{}, fmt.Errorf("count expired telemetry idempotency records: %w", err)
	}
	logBatchResult, err := tx.ExecContext(ctx, `
		WITH expired AS (
			SELECT agent_id, batch_id FROM agent_log_batches
			WHERE received_at < $1 ORDER BY received_at
			LIMIT $2 FOR UPDATE SKIP LOCKED
		)
		DELETE FROM agent_log_batches batch USING expired
		WHERE batch.agent_id = expired.agent_id AND batch.batch_id = expired.batch_id
	`, now.Add(-policy.IdempotencyAge), policy.BatchSize)
	if err != nil {
		return RetentionResult{}, fmt.Errorf("delete expired log idempotency records: %w", err)
	}
	logBatchesDeleted, err := logBatchResult.RowsAffected()
	if err != nil {
		return RetentionResult{}, fmt.Errorf("count expired log idempotency records: %w", err)
	}
	liveTicketResult, err := tx.ExecContext(ctx, `
		WITH expired AS (
			SELECT id
			FROM live_stream_tickets
			WHERE expires_at < $1
			ORDER BY expires_at
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		DELETE FROM live_stream_tickets ticket
		USING expired
		WHERE ticket.id = expired.id
	`, now, policy.BatchSize)
	if err != nil {
		return RetentionResult{}, fmt.Errorf("delete expired live stream tickets: %w", err)
	}
	liveTicketsDeleted, err := liveTicketResult.RowsAffected()
	if err != nil {
		return RetentionResult{}, fmt.Errorf("count expired live stream tickets: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return RetentionResult{}, fmt.Errorf("commit telemetry retention transaction: %w", err)
	}
	return RetentionResult{
		SamplesDeleted:     samplesDeleted,
		BatchesDeleted:     batchesDeleted,
		LogBatchesDeleted:  logBatchesDeleted,
		LiveTicketsDeleted: liveTicketsDeleted,
	}, nil
}

// RunRetention performs a bounded startup sweep and then runs hourly. It uses
// short transactions so normal ingest is not blocked by a large cleanup.
func RunRetention(ctx context.Context, db *sql.DB, policy RetentionPolicy, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	runSweep := func() {
		for {
			result, err := PruneBatch(ctx, db, time.Now().UTC(), policy)
			if err != nil {
				if ctx.Err() == nil {
					logger.Warn("telemetry retention sweep failed", "error", err)
				}
				return
			}
			if result.SamplesDeleted > 0 || result.BatchesDeleted > 0 || result.LogBatchesDeleted > 0 || result.LiveTicketsDeleted > 0 {
				logger.Info("telemetry retention sweep completed",
					"samples_deleted", result.SamplesDeleted,
					"batch_records_deleted", result.BatchesDeleted,
					"log_batch_records_deleted", result.LogBatchesDeleted,
					"live_tickets_deleted", result.LiveTicketsDeleted,
				)
			}
			if result.SamplesDeleted < int64(policy.BatchSize) &&
				result.BatchesDeleted < int64(policy.BatchSize) &&
				result.LogBatchesDeleted < int64(policy.BatchSize) &&
				result.LiveTicketsDeleted < int64(policy.BatchSize) {
				return
			}
		}
	}

	runSweep()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runSweep()
		}
	}
}
