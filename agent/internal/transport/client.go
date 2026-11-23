package transport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	monitorv1 "github.com/WorkRCS/monitor_me/agent/gen/monitor/v1"
	"github.com/WorkRCS/monitor_me/agent/internal/config"
	"github.com/WorkRCS/monitor_me/agent/internal/operations"
	"github.com/WorkRCS/monitor_me/agent/internal/spool"
	"github.com/WorkRCS/monitor_me/agent/internal/state"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const maxRPCMessageBytes = 1 << 20

type Client struct {
	connection *grpc.ClientConn
	rpc        monitorv1.AgentControlClient
}

func Dial(address, caFile string) (*Client, error) {
	certificatePEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read configured gRPC CA certificate: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certificatePEM) {
		return nil, errors.New("configured gRPC CA file contains no valid PEM certificate")
	}
	connection, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			RootCAs: roots, MinVersion: tls.VersionTLS12,
		})),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxRPCMessageBytes),
			grpc.MaxCallSendMsgSize(maxRPCMessageBytes),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize TLS gRPC client: %w", err)
	}
	return &Client{connection: connection, rpc: monitorv1.NewAgentControlClient(connection)}, nil
}

func (c *Client) LoadOrEnroll(
	ctx context.Context,
	cfg config.Config,
	hostname string,
	capabilities []string,
) (state.Credentials, error) {
	existing, err := state.Load(cfg.StateFile)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, state.ErrExpired) {
		return state.Credentials{}, err
	}
	if cfg.EnrollmentToken == "" {
		return state.Credentials{}, errors.New("no valid local agent session exists; a fresh enrollment token is required")
	}
	if errors.Is(err, state.ErrExpired) {
		if removeErr := state.Remove(cfg.StateFile); removeErr != nil {
			return state.Credentials{}, removeErr
		}
	}

	response, err := c.rpc.Enroll(ctx, &monitorv1.EnrollRequest{
		EnrollmentToken: cfg.EnrollmentToken,
		AgentVersion:    cfg.Version,
		OperatingSystem: runtime.GOOS,
		Architecture:    runtime.GOARCH,
		Hostname:        hostname,
		Capabilities:    capabilities,
	})
	if err != nil {
		return state.Credentials{}, fmt.Errorf("enroll native agent over TLS: %w", err)
	}
	credentials := state.Credentials{
		AgentID: response.GetAgentId(), ServerID: response.GetServerId(),
		SessionToken: response.GetSessionToken(), ProtocolVersion: response.GetProtocolVersion(),
		ExpiresAtUnixMillis:      response.GetSessionExpiresAtUnixMillis(),
		HeartbeatIntervalSeconds: response.GetHeartbeatIntervalSeconds(),
	}
	if err := state.Save(cfg.StateFile, credentials); err != nil {
		return state.Credentials{}, err
	}
	return credentials, nil
}

func (c *Client) ReportHeartbeat(
	ctx context.Context,
	session state.Credentials,
	version string,
	capabilities []string,
) error {
	response, err := c.rpc.ReportHeartbeat(ctx, &monitorv1.HeartbeatRequest{
		AgentId: session.AgentID, SessionToken: session.SessionToken,
		AgentVersion: version, Capabilities: capabilities,
		SentAtUnixMillis: time.Now().UTC().UnixMilli(),
	})
	if err != nil {
		return fmt.Errorf("report agent heartbeat: %w", err)
	}
	if !response.GetAccepted() {
		return errors.New("control plane did not accept the agent heartbeat")
	}
	return nil
}

// SendPending acknowledges a local WAL batch only after the control plane
// confirms its PostgreSQL transaction has committed.
func (c *Client) SendPending(ctx context.Context, session state.Credentials, store *spool.Store, allowlists ...[]string) (int, error) {
	batches, err := store.Peek(16)
	if err != nil {
		return 0, err
	}
	var allowedUnits []string
	if len(allowlists) > 0 {
		allowedUnits = allowlists[0]
	}
	if len(batches) == 0 && len(allowedUnits) == 0 {
		return 0, nil
	}
	stream, err := c.rpc.Stream(ctx)
	if err != nil {
		return 0, fmt.Errorf("open telemetry stream: %w", err)
	}
	sent := 0
	for _, batch := range batches {
		samples := make([]*monitorv1.MetricSample, 0, len(batch.Samples))
		for _, sample := range batch.Samples {
			samples = append(samples, &monitorv1.MetricSample{
				MetricName: sample.Name, Value: sample.Value, Unit: sample.Unit,
				Labels: sample.Labels, ObservedAtUnixNanos: sample.ObservedAtUnixNanos,
			})
		}
		message := &monitorv1.AgentToServerMessage{
			AgentId: session.AgentID, SessionToken: session.SessionToken,
			Payload: &monitorv1.AgentToServerMessage_TelemetryBatch{
				TelemetryBatch: &monitorv1.TelemetryBatch{
					BatchId: batch.ID, CollectedAtUnixNanos: batch.CollectedAtUnixNanos,
					SchemaVersion: batch.SchemaVersion, Samples: samples,
				},
			},
		}
		if err := stream.Send(message); err != nil {
			return sent, fmt.Errorf("send telemetry batch %s: %w", batch.ID, err)
		}
		response, err := stream.Recv()
		if err != nil {
			return sent, fmt.Errorf("receive telemetry acknowledgement for %s: %w", batch.ID, err)
		}
		acknowledgement := response.GetAcknowledgement()
		if acknowledgement == nil || acknowledgement.GetBatchId() != batch.ID {
			return sent, fmt.Errorf("control plane returned an invalid acknowledgement for batch %s", batch.ID)
		}
		if !acknowledgement.GetAccepted() {
			return sent, fmt.Errorf("control plane rejected batch %s: %s (%s)",
				batch.ID, acknowledgement.GetMessage(), acknowledgement.GetRejectionCode())
		}
		if err := store.Ack([]string{batch.ID}); err != nil {
			return sent, fmt.Errorf("acknowledge durable local batch %s: %w", batch.ID, err)
		}
		sent++
	}

	if runtime.GOOS == "linux" && len(allowedUnits) > 0 {
		if err := c.pollRemoteOperation(ctx, stream, session, allowedUnits); err != nil {
			return sent, err
		}
	}
	if err := stream.CloseSend(); err != nil {
		return sent, fmt.Errorf("close telemetry stream: %w", err)
	}
	return sent, nil
}

func (c *Client) pollRemoteOperation(
	ctx context.Context,
	stream monitorv1.AgentControl_StreamClient,
	session state.Credentials,
	allowedUnits []string,
) error {
	poll := &monitorv1.AgentToServerMessage{
		AgentId:      session.AgentID,
		SessionToken: session.SessionToken,
		Payload: &monitorv1.AgentToServerMessage_Event{Event: &monitorv1.AgentEvent{
			EventId: "command-poll", EventType: "command_poll", OccurredAtUnixNanos: time.Now().UTC().UnixNano(),
		}},
	}
	if err := stream.Send(poll); err != nil {
		return fmt.Errorf("request remote operation: %w", err)
	}
	response, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receive remote operation: %w", err)
	}
	command := response.GetCommand()
	if command == nil {
		ack := response.GetAcknowledgement()
		if ack == nil || ack.GetBatchId() != "command-poll" || !ack.GetAccepted() {
			return errors.New("control plane returned an invalid remote operation poll response")
		}
		return nil
	}

	var payload struct {
		Unit string `json:"unit"`
	}
	result := operations.Result{Status: "failed", Output: "Remote operation command was malformed."}
	if err := json.Unmarshal(command.GetPayload(), &payload); err == nil {
		if command.GetExpiresAtUnixMillis() > time.Now().UTC().UnixMilli() {
			result = operations.Execute(ctx, command.GetCommandType(), payload.Unit, allowedUnits)
		} else {
			result.Output = "Remote operation expired before agent execution."
		}
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode remote operation result: %w", err)
	}
	resultMessage := &monitorv1.AgentToServerMessage{
		AgentId:      session.AgentID,
		SessionToken: session.SessionToken,
		Payload: &monitorv1.AgentToServerMessage_Event{Event: &monitorv1.AgentEvent{
			EventId: command.GetCommandId(), EventType: "command_result",
			Message: string(encoded), OccurredAtUnixNanos: time.Now().UTC().UnixNano(),
		}},
	}
	if err := stream.Send(resultMessage); err != nil {
		return fmt.Errorf("send remote operation result: %w", err)
	}
	acknowledgement, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receive remote operation acknowledgement: %w", err)
	}
	ack := acknowledgement.GetAcknowledgement()
	if ack == nil {
		return errors.New("control plane returned an invalid remote operation result acknowledgement")
	}
	if ack.GetBatchId() != command.GetCommandId() || !ack.GetAccepted() {
		return fmt.Errorf("control plane rejected remote operation result: %s", ack.GetMessage())
	}
	return nil
}

func (c *Client) Close() error {
	if c.connection == nil {
		return nil
	}
	return c.connection.Close()
}
