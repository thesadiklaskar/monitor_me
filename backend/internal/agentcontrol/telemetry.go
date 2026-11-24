package agentcontrol

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"math"
	"regexp"
	"strings"
	"time"

	monitorv1 "example.com/monitor-me/backend/gen/monitor/v1"
	"example.com/monitor-me/backend/internal/telemetry"
	"example.com/monitor-me/backend/internal/logstore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const maxTelemetryBatchSamples = 4096

var (
	batchIDPattern    = regexp.MustCompile("^[0-9a-f]{32}$")
	metricNamePattern = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_.-]{0,127}$")
	labelKeyPattern   = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_.-]{0,63}$")
)

// Stream acknowledges telemetry only after its database transaction commits.
// A lost acknowledgement is safe because retries use a stable per-agent batch ID.
func (s *Service) Stream(stream monitorv1.AgentControl_StreamServer) error {
	for {
		message, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if message == nil || !agentIDPattern.MatchString(message.GetAgentId()) ||
			len(message.GetSessionToken()) < 32 || len(message.GetSessionToken()) > 128 {
			return status.Error(codes.Unauthenticated, "agent stream identity is invalid")
		}
		if event := message.GetEvent(); event != nil {
			response, eventErr := s.handleAgentEvent(stream.Context(), message.GetAgentId(), message.GetSessionToken(), event)
			if eventErr != nil {
				return eventErr
			}
			if err := stream.Send(response); err != nil {
				return err
			}
			continue
		}
		logBatch := message.GetLogBatch()
		if logBatch == nil {
			batch := message.GetTelemetryBatch()
			if batch == nil {
				return status.Error(codes.InvalidArgument, "unsupported agent stream payload")
			}
			accepted, rejectionCode, persistErr := s.persistTelemetryBatch(
				stream.Context(), message.GetAgentId(), message.GetSessionToken(), batch,
			)
			if persistErr != nil {
				if status.Code(persistErr) == codes.InvalidArgument {
					accepted = false
					rejectionCode = "invalid_batch"
				} else {
					return persistErr
				}
			}
			ack := &monitorv1.BatchAcknowledgement{
				BatchId: batch.GetBatchId(), Accepted: accepted, RejectionCode: rejectionCode,
			}
			if accepted {
				ack.Message = "telemetry batch committed"
			} else if rejectionCode == "idempotency_conflict" {
				ack.Message = "batch ID was already used for different telemetry content"
			} else {
				ack.Message = "telemetry batch failed validation"
			}
			if err := stream.Send(&monitorv1.ServerToAgentMessage{
				Payload: &monitorv1.ServerToAgentMessage_Acknowledgement{Acknowledgement: ack},
			}); err != nil {
				return err
			}
			continue
		}

		if s.logs == nil {
			return status.Error(codes.Unavailable, "durable log storage is not configured")
		}
		batch := logstore.Batch{
			ID: logBatch.GetBatchId(), CollectedAtUnixNanos: logBatch.GetCollectedAtUnixNanos(),
			SchemaVersion: logBatch.GetSchemaVersion(), Events: make([]logstore.Event, 0, len(logBatch.GetEvents())),
		}
		for _, event := range logBatch.GetEvents() {
			if event == nil {
				return status.Error(codes.InvalidArgument, "log batch contains an empty event")
			}
			batch.Events = append(batch.Events, logstore.Event{
				ID: event.GetEventId(), EventTimeUnixNanos: event.GetEventTimeUnixNanos(),
				ObservedAtUnixNanos: event.GetObservedAtUnixNanos(), Source: event.GetSource(),
				Severity: event.GetSeverity(), Body: event.GetBody(), Attributes: event.GetAttributes(),
			})
		}
		accepted, rejectionCode, ingestErr := s.logs.Ingest(
			stream.Context(), message.GetAgentId(), message.GetSessionToken(), batch,
		)
		if ingestErr != nil {
			switch {
			case errors.Is(ingestErr, logstore.ErrUnauthorized):
				return status.Error(codes.Unauthenticated, "agent session is invalid, expired, or revoked")
			case errors.Is(ingestErr, logstore.ErrInvalidBatch):
				accepted = false
				rejectionCode = "invalid_log_batch"
			case errors.Is(ingestErr, logstore.ErrIdempotencyConflict):
				accepted = false
				rejectionCode = "idempotency_conflict"
			default:
				return status.Error(codes.Unavailable, "durable log storage is temporarily unavailable")
			}
		}
		ack := &monitorv1.BatchAcknowledgement{
			BatchId: batch.ID, Accepted: accepted, RejectionCode: rejectionCode,
		}
		if accepted {
			ack.Message = "log batch durably accepted"
		} else if rejectionCode == "idempotency_conflict" {
			ack.Message = "log batch ID was already used for different event content"
		} else {
			ack.Message = "log batch failed validation"
		}
		if err := stream.Send(&monitorv1.ServerToAgentMessage{
			Payload: &monitorv1.ServerToAgentMessage_Acknowledgement{Acknowledgement: ack},
		}); err != nil {
			return err
		}

	}
}

type databaseMetricSample struct {
	MetricName          string            `json:"metric_name"`
	Value               float64           `json:"value"`
	Unit                string            `json:"unit"`
	Labels              map[string]string `json:"labels"`
	ObservedAtUnixNanos int64             `json:"observed_at_unix_nanos"`
}

func (s *Service) persistTelemetryBatch(ctx context.Context, agentID, sessionToken string, batch *monitorv1.TelemetryBatch) (bool, string, error) {
	if err := validateTelemetryBatch(batch); err != nil {
		return false, "", status.Error(codes.InvalidArgument, err.Error())
	}

	sessionHash := sha256.Sum256([]byte(sessionToken))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, "", status.Error(codes.Unavailable, "telemetry storage is temporarily unavailable")
	}
	defer tx.Rollback()

	var serverID, organizationID string
	err = tx.QueryRowContext(ctx, `
		SELECT a.server_id::text, s.organization_id::text
		FROM agents a
		JOIN servers s ON s.id = a.server_id
		JOIN agent_sessions agent_session ON agent_session.agent_id = a.id
		WHERE a.id = $1
		  AND a.status <> 'revoked'
		  AND s.status <> 'disabled'
		  AND agent_session.token_hash = $2
		  AND agent_session.revoked_at IS NULL
		  AND agent_session.expires_at > now()
		FOR UPDATE OF a, agent_session
	`, agentID, sessionHash[:]).Scan(&serverID, &organizationID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", status.Error(codes.Unauthenticated, "agent session is invalid, expired, or revoked")
	}
	if err != nil {
		return false, "", status.Error(codes.Unavailable, "could not validate telemetry session")
	}

	canonical, err := (proto.MarshalOptions{Deterministic: true}).Marshal(batch)
	if err != nil {
		return false, "", status.Error(codes.InvalidArgument, "telemetry batch cannot be encoded")
	}
	payloadHash := sha256.Sum256(canonical)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO telemetry_batches (
			agent_id, batch_id, schema_version, payload_hash, collected_at, sample_count
		)
		VALUES ($1, $2, $3, $4, to_timestamp($5 / 1000000000.0), $6)
		ON CONFLICT (agent_id, batch_id) DO NOTHING
	`, agentID, batch.GetBatchId(), batch.GetSchemaVersion(), payloadHash[:],
		batch.GetCollectedAtUnixNanos(), len(batch.GetSamples()))
	if err != nil {
		return false, "", status.Error(codes.Unavailable, "could not reserve telemetry idempotency key")
	}
	insertedBatch, err := result.RowsAffected()
	if err != nil {
		return false, "", status.Error(codes.Unavailable, "could not inspect telemetry idempotency key")
	}
	if insertedBatch == 0 {
		var previousHash []byte
		if err := tx.QueryRowContext(ctx, `
			SELECT payload_hash FROM telemetry_batches WHERE agent_id = $1 AND batch_id = $2
		`, agentID, batch.GetBatchId()).Scan(&previousHash); err != nil {
			return false, "", status.Error(codes.Unavailable, "could not verify prior telemetry batch")
		}
		if !bytes.Equal(previousHash, payloadHash[:]) {
			return false, "idempotency_conflict", nil
		}
		if err := tx.Commit(); err != nil {
			return false, "", status.Error(codes.Unavailable, "could not confirm duplicate telemetry batch")
		}
		return true, "", nil
	}

	samples := make([]databaseMetricSample, 0, len(batch.GetSamples()))
	for _, sample := range batch.GetSamples() {
		labels := sample.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		samples = append(samples, databaseMetricSample{
			MetricName: sample.GetMetricName(), Value: sample.GetValue(), Unit: sample.GetUnit(),
			Labels: labels, ObservedAtUnixNanos: sample.GetObservedAtUnixNanos(),
		})
	}
	payloadJSON, err := json.Marshal(samples)
	if err != nil {
		return false, "", status.Error(codes.InvalidArgument, "telemetry samples cannot be encoded")
	}
	result, err = tx.ExecContext(ctx, `
		INSERT INTO telemetry_samples (
			organization_id, server_id, agent_id, batch_id, metric_name, value,
			unit, labels, observed_at, project_id, attribution_method
		)
		SELECT
			$1, $2, $3, $4, sample.metric_name, sample.value, sample.unit,
			sample.labels,
			to_timestamp(sample.observed_at_unix_nanos / 1000000000.0),
			mapping.project_id,
			CASE
				WHEN sample.labels->>'attribution_method' = 'shared' THEN 'shared'
				WHEN sample.labels->>'attribution_method' = 'unclassified' THEN 'unclassified'
				WHEN sample.labels ? 'workload_selector' THEN COALESCE(mapping.attribution_method, 'unclassified')
				ELSE 'host_total'
			END
		FROM jsonb_to_recordset($5::jsonb) AS sample(
			metric_name text, value double precision, unit text, labels jsonb,
			observed_at_unix_nanos bigint
		)
		LEFT JOIN LATERAL (
			SELECT mapping.project_id, mapping.attribution_method
			FROM server_project_mappings mapping
			WHERE mapping.organization_id = $1
			  AND mapping.server_id = $2
			  AND mapping.workload_type = sample.labels->>'workload_type'
			  AND mapping.workload_selector = sample.labels->>'workload_selector'
			LIMIT 1
		) mapping ON true
	`, organizationID, serverID, agentID, batch.GetBatchId(), string(payloadJSON))
	if err != nil {
		return false, "", status.Error(codes.Unavailable, "could not persist telemetry samples")
	}
	sampleCount, err := result.RowsAffected()
	if err != nil || sampleCount != int64(len(batch.GetSamples())) {
		return false, "", status.Error(codes.Unavailable, "persisted sample count did not match the submitted batch")
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_sessions SET last_seen_at = now()
		WHERE agent_id = $1 AND token_hash = $2
	`, agentID, sessionHash[:]); err != nil {
		return false, "", status.Error(codes.Unavailable, "could not refresh agent session liveness")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE agents SET status = 'online', last_seen_at = now(), updated_at = now()
		WHERE id = $1
	`, agentID); err != nil {
		return false, "", status.Error(codes.Unavailable, "could not refresh agent liveness")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE servers SET status = 'online', updated_at = now() WHERE id = $1
	`, serverID); err != nil {
		return false, "", status.Error(codes.Unavailable, "could not refresh server liveness")
	}
	// Evaluate alert rules before commit. A transient alert-storage failure
	// rolls back the metrics batch too, allowing a safe replay from agent WAL.
	alertRows, err := tx.QueryContext(ctx, `
		SELECT metric_name, value, unit, labels, COALESCE(project_id::text, ''),
		       (EXTRACT(EPOCH FROM observed_at) * 1000000000)::bigint
		FROM telemetry_samples
		WHERE agent_id = $1 AND batch_id = $2
		ORDER BY id
	`, agentID, batch.GetBatchId())
	if err != nil {
		return false, "", status.Error(codes.Unavailable, "could not load telemetry samples for alert evaluation")
	}
	alertSamples := make([]telemetry.LiveSample, 0, len(batch.GetSamples()))
	for alertRows.Next() {
		var sample telemetry.LiveSample
		var labelsJSON []byte
		if err := alertRows.Scan(
			&sample.MetricName, &sample.Value, &sample.Unit, &labelsJSON,
			&sample.ProjectID, &sample.ObservedAtUnixNanos,
		); err != nil {
			_ = alertRows.Close()
			return false, "", status.Error(codes.Unavailable, "could not read telemetry samples for alert evaluation")
		}
		if len(labelsJSON) > 0 {
			if err := json.Unmarshal(labelsJSON, &sample.Labels); err != nil {
				_ = alertRows.Close()
				return false, "", status.Error(codes.Unavailable, "could not decode telemetry labels for alert evaluation")
			}
		}
		alertSamples = append(alertSamples, sample)
	}
	alertRowsErr := alertRows.Err()
	alertRowsCloseErr := alertRows.Close()
	if alertRowsErr != nil || alertRowsCloseErr != nil || len(alertSamples) != len(batch.GetSamples()) {
		return false, "", status.Error(codes.Unavailable, "telemetry samples could not be verified for alert evaluation")
	}
	if err := evaluateCommittedSamples(ctx, tx, organizationID, serverID, alertSamples); err != nil {
		return false, "", status.Error(codes.Unavailable, "alert evaluation could not be committed with telemetry")
	}

	if err := tx.Commit(); err != nil {
		return false, "", status.Error(codes.Unavailable, "could not commit telemetry batch")
	}
	s.publishCommittedBatch(ctx, organizationID, serverID, agentID, batch)
	return true, "", nil
}

func validateTelemetryBatch(batch *monitorv1.TelemetryBatch) error {
	if batch == nil || !batchIDPattern.MatchString(batch.GetBatchId()) {
		return errors.New("telemetry batch ID must be 32 lowercase hexadecimal characters")
	}
	if batch.GetSchemaVersion() != 1 {
		return errors.New("unsupported telemetry schema version")
	}
	if len(batch.GetSamples()) == 0 || len(batch.GetSamples()) > maxTelemetryBatchSamples {
		return errors.New("telemetry batch sample count is outside the allowed range")
	}
	now := time.Now()
	if batch.GetCollectedAtUnixNanos() <= 0 ||
		time.Unix(0, batch.GetCollectedAtUnixNanos()).After(now.Add(5*time.Minute)) {
		return errors.New("telemetry batch timestamp is invalid")
	}
	for _, sample := range batch.GetSamples() {
		if sample == nil || !metricNamePattern.MatchString(sample.GetMetricName()) ||
			len(sample.GetUnit()) == 0 || len(sample.GetUnit()) > 64 ||
			strings.ContainsAny(sample.GetUnit(), "\r\n\x00") ||
			math.IsNaN(sample.GetValue()) || math.IsInf(sample.GetValue(), 0) ||
			sample.GetObservedAtUnixNanos() <= 0 ||
			time.Unix(0, sample.GetObservedAtUnixNanos()).After(now.Add(5*time.Minute)) ||
			len(sample.GetLabels()) > 32 {
			return errors.New("telemetry sample is invalid")
		}
		for key, value := range sample.GetLabels() {
			if !labelKeyPattern.MatchString(key) || len(value) > 256 ||
				strings.ContainsAny(value, "\r\n\x00") {
				return errors.New("telemetry sample labels are invalid")
			}
		}
	}
	return nil
}
