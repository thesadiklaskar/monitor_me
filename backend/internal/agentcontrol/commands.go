package agentcontrol

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	monitorv1 "example.com/monitor-me/backend/gen/monitor/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type remoteCommandPayload struct {
	Unit string `json:"unit"`
}

type remoteCommandResult struct {
	Status string `json:"status"`
	Output string `json:"output"`
}

type leasedRemoteCommand struct {
	ID        string
	Operation string
	Target    string
	ExpiresAt time.Time
}

// handleAgentEvent processes only command polling and command results. Agent
// identity and the opaque session are revalidated for every event.
func (s *Service) handleAgentEvent(
	ctx context.Context,
	agentID, sessionToken string,
	event *monitorv1.AgentEvent,
) (*monitorv1.ServerToAgentMessage, error) {
	if event == nil || len(event.GetMessage()) > 4096 {
		return nil, status.Error(codes.InvalidArgument, "agent event is invalid or exceeds the message limit")
	}
	organizationID, serverID, err := s.authenticateAgent(ctx, agentID, sessionToken)
	if err != nil {
		return nil, err
	}

	switch event.GetEventType() {
	case "command_poll":
		command, err := s.leaseRemoteCommand(ctx, organizationID, serverID, agentID, sessionToken)
		if err != nil {
			return nil, err
		}
		if command == nil {
			return commandAcknowledgement("command-poll", "no pending operations"), nil
		}
		payload, err := json.Marshal(remoteCommandPayload{Unit: command.Target})
		if err != nil {
			return nil, status.Error(codes.Internal, "could not encode the allowlisted remote operation")
		}
		return &monitorv1.ServerToAgentMessage{
			Payload: &monitorv1.ServerToAgentMessage_Command{Command: &monitorv1.AgentCommand{
				CommandId: command.ID, CommandType: command.Operation,
				Payload: payload, ExpiresAtUnixMillis: command.ExpiresAt.UnixMilli(),
			}},
		}, nil
	case "command_result":
		if !validCommandUUID(event.GetEventId()) {
			return nil, status.Error(codes.InvalidArgument, "remote operation result ID is invalid")
		}
		var result remoteCommandResult
		if err := json.Unmarshal([]byte(event.GetMessage()), &result); err != nil ||
			(result.Status != "succeeded" && result.Status != "failed") {
			return nil, status.Error(codes.InvalidArgument, "remote operation result is invalid")
		}
		result.Output = strings.TrimSpace(result.Output)
		if len(result.Output) > 2048 {
			result.Output = result.Output[:2048]
		}
		if err := s.completeRemoteCommand(ctx, organizationID, serverID, agentID, sessionToken, event.GetEventId(), result); err != nil {
			return nil, err
		}
		return commandAcknowledgement(event.GetEventId(), "remote operation result recorded"), nil
	default:
		return nil, status.Error(codes.InvalidArgument, "agent event type is not supported")
	}
}

func commandAcknowledgement(id, message string) *monitorv1.ServerToAgentMessage {
	return &monitorv1.ServerToAgentMessage{
		Payload: &monitorv1.ServerToAgentMessage_Acknowledgement{Acknowledgement: &monitorv1.BatchAcknowledgement{
			BatchId: id, Accepted: true, Message: message,
		}},
	}
}

func (s *Service) authenticateAgent(ctx context.Context, agentID, sessionToken string) (string, string, error) {
	if !agentIDPattern.MatchString(agentID) || len(sessionToken) < 32 || len(sessionToken) > 128 {
		return "", "", status.Error(codes.Unauthenticated, "agent stream identity is invalid")
	}
	hash := sha256.Sum256([]byte(sessionToken))
	var organizationID, serverID string
	err := s.db.QueryRowContext(ctx, `
		SELECT server.organization_id::text, server.id::text
		FROM agents agent
		JOIN servers server ON server.id = agent.server_id
		JOIN agent_sessions session ON session.agent_id = agent.id
		WHERE agent.id = $1
		  AND agent.status <> 'revoked'
		  AND server.status <> 'disabled'
		  AND session.token_hash = $2
		  AND session.revoked_at IS NULL
		  AND session.expires_at > now()
	`, agentID, hash[:]).Scan(&organizationID, &serverID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", status.Error(codes.Unauthenticated, "agent session is invalid, expired, or revoked")
	}
	if err != nil {
		return "", "", status.Error(codes.Unavailable, "could not validate agent session")
	}
	return organizationID, serverID, nil
}

func (s *Service) leaseRemoteCommand(
	ctx context.Context,
	organizationID, serverID, agentID, sessionToken string,
) (*leasedRemoteCommand, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "could not begin remote operation lease")
	}
	defer tx.Rollback()

	hash := sha256.Sum256([]byte(sessionToken))
	var active bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM agents agent
			JOIN servers server ON server.id = agent.server_id
			JOIN agent_sessions session ON session.agent_id = agent.id
			WHERE agent.id = $1 AND server.id = $2 AND server.organization_id = $3
			  AND agent.status <> 'revoked' AND server.status <> 'disabled'
			  AND session.token_hash = $4 AND session.revoked_at IS NULL AND session.expires_at > now()
		)
	`, agentID, serverID, organizationID, hash[:]).Scan(&active); err != nil || !active {
		return nil, status.Error(codes.Unauthenticated, "agent session is no longer active")
	}

	// Expire queued commands and write their audit records in one statement.
	if _, err := tx.ExecContext(ctx, `
		WITH expired AS (
			UPDATE remote_operations
			SET status = 'expired', completed_at = now(), result = 'Command expired before dispatch.'
			WHERE organization_id = $1 AND server_id = $2 AND agent_id = $3
			  AND status = 'queued' AND expires_at <= now()
			RETURNING id, created_by, operation, target
		)
		INSERT INTO remote_operation_audit (
			organization_id, remote_operation_id, actor_user_id, actor_agent_id, action, details
		)
		SELECT $1, expired.id, expired.created_by, $3, 'expired',
		       jsonb_build_object('operation', expired.operation, 'target', expired.target)
		FROM expired
	`, organizationID, serverID, agentID); err != nil {
		return nil, status.Error(codes.Unavailable, "could not expire pending remote operations")
	}

	var command leasedRemoteCommand
	err = tx.QueryRowContext(ctx, `
		SELECT id::text, operation, target, expires_at
		FROM remote_operations
		WHERE organization_id = $1 AND server_id = $2 AND agent_id = $3
		  AND status = 'queued' AND expires_at > now()
		ORDER BY created_at
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, organizationID, serverID, agentID).Scan(&command.ID, &command.Operation, &command.Target, &command.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.Commit(); err != nil {
			return nil, status.Error(codes.Unavailable, "could not commit remote operation poll")
		}
		return nil, nil
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "could not lease remote operation")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE remote_operations SET status = 'dispatched', dispatched_at = now()
		WHERE id = $1 AND status = 'queued'
	`, command.ID); err != nil {
		return nil, status.Error(codes.Unavailable, "could not dispatch remote operation")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO remote_operation_audit (organization_id, remote_operation_id, actor_agent_id, action, details)
		VALUES ($1, $2, $3, 'dispatched', jsonb_build_object('operation', $4, 'target', $5))
	`, organizationID, command.ID, agentID, command.Operation, command.Target); err != nil {
		return nil, status.Error(codes.Unavailable, "could not audit remote operation dispatch")
	}
	if err := tx.Commit(); err != nil {
		return nil, status.Error(codes.Unavailable, "could not commit remote operation dispatch")
	}
	return &command, nil
}

func (s *Service) completeRemoteCommand(
	ctx context.Context,
	organizationID, serverID, agentID, sessionToken, commandID string,
	result remoteCommandResult,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return status.Error(codes.Unavailable, "could not begin remote operation result transaction")
	}
	defer tx.Rollback()

	hash := sha256.Sum256([]byte(sessionToken))
	var active bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM agents agent
			JOIN servers server ON server.id = agent.server_id
			JOIN agent_sessions session ON session.agent_id = agent.id
			WHERE agent.id = $1 AND server.id = $2 AND server.organization_id = $3
			  AND agent.status <> 'revoked' AND server.status <> 'disabled'
			  AND session.token_hash = $4 AND session.revoked_at IS NULL AND session.expires_at > now()
		)
	`, agentID, serverID, organizationID, hash[:]).Scan(&active); err != nil || !active {
		return status.Error(codes.Unauthenticated, "agent session is no longer active")
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return status.Error(codes.InvalidArgument, "remote operation result could not be encoded")
	}
	updated, err := tx.ExecContext(ctx, `
		UPDATE remote_operations
		SET status = $1, result = $2, completed_at = now()
		WHERE id = $3 AND organization_id = $4 AND server_id = $5 AND agent_id = $6
		  AND status = 'dispatched'
	`, result.Status, result.Output, commandID, organizationID, serverID, agentID)
	if err != nil {
		return status.Error(codes.Unavailable, "could not record remote operation result")
	}
	count, err := updated.RowsAffected()
	if err != nil || count != 1 {
		return status.Error(codes.NotFound, "remote operation is no longer awaiting a result")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO remote_operation_audit (organization_id, remote_operation_id, actor_agent_id, action, details)
		VALUES ($1, $2, $3, $4, $5::jsonb)
	`, organizationID, commandID, agentID, result.Status, string(encoded)); err != nil {
		return status.Error(codes.Unavailable, "could not audit remote operation result")
	}
	if err := tx.Commit(); err != nil {
		return status.Error(codes.Unavailable, "could not commit remote operation result")
	}
	return nil
}

var remoteOperationUUIDPattern = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$")

func validCommandUUID(value string) bool {
	return remoteOperationUUIDPattern.MatchString(value)
}
