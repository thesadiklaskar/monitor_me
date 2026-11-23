package agentcontrol

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	monitorv1 "github.com/WorkRCS/monitor_me/backend/gen/monitor/v1"
	"github.com/WorkRCS/monitor_me/backend/internal/telemetry"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	agentSessionTTL = 30 * 24 * time.Hour
	heartbeatPeriod = 15
)

var (
	agentIDPattern    = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$")
	capabilityPattern = regexp.MustCompile("^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$")
)

// Service implements the versioned control-plane RPCs used during Phase 2.
type Service struct {
	monitorv1.UnimplementedAgentControlServer
	db        *sql.DB
	publisher telemetry.EventPublisher
}

// NewService creates an agent-control service backed by PostgreSQL.
func NewService(db *sql.DB, publishers ...telemetry.EventPublisher) *Service {
	service := &Service{db: db}
	if len(publishers) > 0 {
		service.publisher = publishers[0]
	}
	return service
}

// Enroll atomically consumes a scoped enrollment token, binds the agent to its
// pre-created server, and returns a separate revocable agent session secret.
func (s *Service) Enroll(ctx context.Context, request *monitorv1.EnrollRequest) (*monitorv1.EnrollResponse, error) {
	if request == nil || !validEnrollmentRequest(request) {
		return nil, status.Error(codes.InvalidArgument, "enrollment request is invalid")
	}

	capabilities, err := normalizeCapabilities(request.GetCapabilities())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "agent capabilities are invalid")
	}
	capabilityJSON, err := json.Marshal(capabilities)
	if err != nil {
		return nil, status.Error(codes.Internal, "could not encode agent capabilities")
	}

	enrollmentHash := sha256.Sum256([]byte(request.GetEnrollmentToken()))
	sessionToken, err := newSessionToken()
	if err != nil {
		return nil, status.Error(codes.Internal, "could not create agent session")
	}
	sessionHash := sha256.Sum256([]byte(sessionToken))
	sessionExpiry := time.Now().UTC().Add(agentSessionTTL)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, "could not begin enrollment")
	}
	defer tx.Rollback()

	var serverID, organizationID string
	err = tx.QueryRowContext(ctx, `
		SELECT server_id::text, organization_id::text
		FROM agent_enrollment_tokens
		WHERE token_hash = $1
		  AND consumed_at IS NULL
		  AND revoked_at IS NULL
		  AND expires_at > now()
		  AND server_id IS NOT NULL
		FOR UPDATE
	`, enrollmentHash[:]).Scan(&serverID, &organizationID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, status.Error(codes.Unauthenticated, "enrollment credential is invalid, expired, revoked, or already consumed")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "could not validate enrollment credential")
	}

	var agentID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO agents (server_id, version, status, last_seen_at, capabilities, updated_at)
		VALUES ($1, $2, 'online', now(), $3::jsonb, now())
		ON CONFLICT (server_id) DO UPDATE
		  SET version = EXCLUDED.version,
		      status = 'online',
		      last_seen_at = now(),
		      capabilities = EXCLUDED.capabilities,
		      updated_at = now()
		RETURNING id::text
	`, serverID, request.GetAgentVersion(), string(capabilityJSON)).Scan(&agentID)
	if err != nil {
		return nil, status.Error(codes.Internal, "could not bind agent identity to server")
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_sessions SET revoked_at = COALESCE(revoked_at, now())
		WHERE agent_id = $1 AND revoked_at IS NULL
	`, agentID); err != nil {
		return nil, status.Error(codes.Internal, "could not rotate agent session")
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE servers
		SET status = 'online',
		    hostname = COALESCE(NULLIF($2, ''), hostname),
		    operating_system = $3,
		    architecture = $4,
		    updated_at = now()
		WHERE id = $1 AND organization_id = $5
	`, serverID, request.GetHostname(), request.GetOperatingSystem(), request.GetArchitecture(), organizationID); err != nil {
		return nil, status.Error(codes.Internal, "could not update enrolled server")
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE agent_enrollment_tokens
		SET consumed_at = now()
		WHERE token_hash = $1 AND consumed_at IS NULL AND revoked_at IS NULL AND expires_at > now()
	`, enrollmentHash[:])
	if err != nil {
		return nil, status.Error(codes.Internal, "could not consume enrollment credential")
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return nil, status.Error(codes.Unauthenticated, "enrollment credential is no longer valid")
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO agent_sessions (agent_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, agentID, sessionHash[:], sessionExpiry); err != nil {
		return nil, status.Error(codes.Internal, "could not create agent session")
	}

	if err := tx.Commit(); err != nil {
		return nil, status.Error(codes.Internal, "could not commit agent enrollment")
	}

	return &monitorv1.EnrollResponse{
		AgentId:                    agentID,
		ServerId:                   serverID,
		SessionToken:               sessionToken,
		HeartbeatIntervalSeconds:   heartbeatPeriod,
		ProtocolVersion:            "monitor.v1",
		SessionExpiresAtUnixMillis: sessionExpiry.UnixMilli(),
	}, nil
}

// ReportHeartbeat validates the agent session and atomically refreshes liveness
// and capability state. The raw session token is never written to the database.
func (s *Service) ReportHeartbeat(ctx context.Context, request *monitorv1.HeartbeatRequest) (*monitorv1.HeartbeatResponse, error) {
	if request == nil || !agentIDPattern.MatchString(request.GetAgentId()) ||
		len(request.GetSessionToken()) < 32 || len(request.GetSessionToken()) > 128 ||
		len(request.GetAgentVersion()) < 1 || len(request.GetAgentVersion()) > 64 {
		return nil, status.Error(codes.InvalidArgument, "heartbeat request is invalid")
	}
	if sentAt := request.GetSentAtUnixMillis(); sentAt > 0 && time.UnixMilli(sentAt).After(time.Now().Add(5*time.Minute)) {
		return nil, status.Error(codes.InvalidArgument, "heartbeat timestamp is too far in the future")
	}

	capabilities, err := normalizeCapabilities(request.GetCapabilities())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "agent capabilities are invalid")
	}
	capabilityJSON, err := json.Marshal(capabilities)
	if err != nil {
		return nil, status.Error(codes.Internal, "could not encode agent capabilities")
	}
	sessionHash := sha256.Sum256([]byte(request.GetSessionToken()))

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, "could not begin heartbeat update")
	}
	defer tx.Rollback()

	var serverID string
	err = tx.QueryRowContext(ctx, `
		SELECT a.server_id::text
		FROM agents a
		JOIN agent_sessions s ON s.agent_id = a.id
		WHERE a.id = $1
		  AND a.status <> 'revoked'
		  AND s.token_hash = $2
		  AND s.revoked_at IS NULL
		  AND s.expires_at > now()
		FOR UPDATE OF a, s
	`, request.GetAgentId(), sessionHash[:]).Scan(&serverID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, status.Error(codes.Unauthenticated, "agent session is invalid, expired, or revoked")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "could not validate agent session")
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_sessions SET last_seen_at = now()
		WHERE agent_id = $1 AND token_hash = $2
	`, request.GetAgentId(), sessionHash[:]); err != nil {
		return nil, status.Error(codes.Internal, "could not update agent session")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE agents
		SET version = $2, capabilities = $3::jsonb, status = 'online',
		    last_seen_at = now(), updated_at = now()
		WHERE id = $1
	`, request.GetAgentId(), request.GetAgentVersion(), string(capabilityJSON)); err != nil {
		return nil, status.Error(codes.Internal, "could not update agent heartbeat")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE servers SET status = 'online', updated_at = now() WHERE id = $1
	`, serverID); err != nil {
		return nil, status.Error(codes.Internal, "could not update server liveness")
	}
	if err := tx.Commit(); err != nil {
		return nil, status.Error(codes.Internal, "could not commit agent heartbeat")
	}

	return &monitorv1.HeartbeatResponse{
		Accepted:             true,
		ServerTimeUnixMillis: time.Now().UTC().UnixMilli(),
	}, nil
}

func (s *Service) publishCommittedBatch(ctx context.Context, organizationID, serverID, agentID string, batch *monitorv1.TelemetryBatch) {
	publishContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()

	// Read committed rows so live events carry the exact project attribution
	// assigned by the PostgreSQL transaction used for historical queries.
	rows, err := s.db.QueryContext(publishContext, `
		SELECT metric_name, value, unit, labels, COALESCE(project_id::text, ''),
		       (EXTRACT(EPOCH FROM observed_at) * 1000000000)::bigint
		FROM telemetry_samples
		WHERE agent_id = $1 AND batch_id = $2
		ORDER BY id
	`, agentID, batch.GetBatchId())
	if err != nil {
		slog.Default().Warn("committed telemetry could not be loaded for live delivery",
			"server_id", serverID, "batch_id", batch.GetBatchId(), "error", err)
		return
	}
	defer rows.Close()

	samples := make([]telemetry.LiveSample, 0, len(batch.GetSamples()))
	for rows.Next() {
		var sample telemetry.LiveSample
		var labelsJSON []byte
		if err := rows.Scan(
			&sample.MetricName, &sample.Value, &sample.Unit, &labelsJSON,
			&sample.ProjectID, &sample.ObservedAtUnixNanos,
		); err != nil {
			slog.Default().Warn("committed telemetry row could not be read for live delivery",
				"server_id", serverID, "batch_id", batch.GetBatchId(), "error", err)
			return
		}
		if len(labelsJSON) > 0 {
			if err := json.Unmarshal(labelsJSON, &sample.Labels); err != nil {
				slog.Default().Warn("committed telemetry labels could not be decoded for live delivery",
					"server_id", serverID, "batch_id", batch.GetBatchId(), "error", err)
				return
			}
		}
		samples = append(samples, sample)
	}
	if err := rows.Err(); err != nil {
		slog.Default().Warn("committed telemetry rows could not be iterated for live delivery",
			"server_id", serverID, "batch_id", batch.GetBatchId(), "error", err)
		return
	}
	evaluateCommittedSamples(publishContext, s.db, organizationID, serverID, samples)
	if s.publisher != nil {
		if err := telemetry.PublishCommittedBatch(
			publishContext, s.publisher, organizationID, serverID, agentID, batch.GetBatchId(), samples,
		); err != nil {
			slog.Default().Warn("durable telemetry committed but live notification failed",
				"server_id", serverID, "batch_id", batch.GetBatchId(), "error", err)
		}
	}
}

func validEnrollmentRequest(request *monitorv1.EnrollRequest) bool {
	return len(request.GetEnrollmentToken()) >= 32 &&
		len(request.GetEnrollmentToken()) <= 128 &&
		len(request.GetAgentVersion()) >= 1 &&
		len(request.GetAgentVersion()) <= 64 &&
		len(request.GetHostname()) >= 1 &&
		len(request.GetHostname()) <= 253 &&
		!strings.ContainsAny(request.GetHostname(), " \t\r\n") &&
		(request.GetOperatingSystem() == "linux" ||
			request.GetOperatingSystem() == "windows" ||
			request.GetOperatingSystem() == "darwin") &&
		(request.GetArchitecture() == "amd64" ||
			request.GetArchitecture() == "arm64" ||
			request.GetArchitecture() == "386" ||
			request.GetArchitecture() == "arm")
}

func normalizeCapabilities(values []string) ([]string, error) {
	if len(values) > 64 {
		return nil, fmt.Errorf("too many capabilities")
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, capability := range values {
		capability = strings.TrimSpace(capability)
		if !capabilityPattern.MatchString(capability) {
			return nil, fmt.Errorf("invalid capability")
		}
		if _, exists := seen[capability]; exists {
			continue
		}
		seen[capability] = struct{}{}
		result = append(result, capability)
	}
	return result, nil
}

func newSessionToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
