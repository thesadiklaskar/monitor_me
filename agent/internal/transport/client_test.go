package transport

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	monitorv1 "example.com/monitor-me/agent/gen/monitor/v1"
	"example.com/monitor-me/agent/internal/collector"
	"example.com/monitor-me/agent/internal/spool"
	"example.com/monitor-me/agent/internal/state"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type streamAcknowledgement func(*monitorv1.TelemetryBatch) (*monitorv1.BatchAcknowledgement, error)

type streamTestServer struct {
	monitorv1.UnimplementedAgentControlServer
	acknowledge streamAcknowledgement
}

func (server *streamTestServer) Stream(stream monitorv1.AgentControl_StreamServer) error {
	for {
		message, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		batch := message.GetTelemetryBatch()
		if batch == nil {
			return status.Error(codes.InvalidArgument, "telemetry batch required")
		}
		ack, err := server.acknowledge(batch)
		if err != nil {
			return err
		}
		if err := stream.Send(&monitorv1.ServerToAgentMessage{
			Payload: &monitorv1.ServerToAgentMessage_Acknowledgement{Acknowledgement: ack},
		}); err != nil {
			return err
		}
	}
}

func newStreamTestClient(t *testing.T, acknowledge streamAcknowledgement) *Client {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	monitorv1.RegisterAgentControlServer(server, &streamTestServer{acknowledge: acknowledge})
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	connection, err := grpc.NewClient(
		"passthrough:///monitor-me-test",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("create test gRPC client: %v", err)
	}
	t.Cleanup(func() {
		_ = connection.Close()
	})
	return &Client{
		connection: connection,
		rpc:        monitorv1.NewAgentControlClient(connection),
	}
}

func newPendingTestBatch(t *testing.T, store *spool.Store) spool.Batch {
	t.Helper()
	now := time.Now().UTC().UnixNano()
	batch, err := spool.NewBatch([]collector.Sample{{
		Name: "host.cpu.utilization", Value: 42, Unit: "percent",
		ObservedAtUnixNanos: now,
	}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append(batch); err != nil {
		t.Fatal(err)
	}
	return batch
}

func TestSendPendingRetainsWALUntilMatchingAcceptedAcknowledgement(t *testing.T) {
	tests := []struct {
		name    string
		ack     streamAcknowledgement
		wantErr bool
	}{
		{
			name: "backend returns an error before acknowledgement",
			ack: func(*monitorv1.TelemetryBatch) (*monitorv1.BatchAcknowledgement, error) {
				return nil, status.Error(codes.Unavailable, "temporary database outage")
			},
			wantErr: true,
		},
		{
			name: "backend rejects the batch",
			ack: func(batch *monitorv1.TelemetryBatch) (*monitorv1.BatchAcknowledgement, error) {
				return &monitorv1.BatchAcknowledgement{
					BatchId: batch.GetBatchId(), Accepted: false, RejectionCode: "invalid_batch",
				}, nil
			},
			wantErr: true,
		},
		{
			name: "backend acknowledges another batch",
			ack: func(batch *monitorv1.TelemetryBatch) (*monitorv1.BatchAcknowledgement, error) {
				return &monitorv1.BatchAcknowledgement{
					BatchId: "ffffffffffffffffffffffffffffffff", Accepted: true,
				}, nil
			},
			wantErr: true,
		},
		{
			name: "matching durable acknowledgement",
			ack: func(batch *monitorv1.TelemetryBatch) (*monitorv1.BatchAcknowledgement, error) {
				return &monitorv1.BatchAcknowledgement{
					BatchId: batch.GetBatchId(), Accepted: true,
				}, nil
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, err := spool.Open(t.TempDir(), 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			batch := newPendingTestBatch(t, store)
			client := newStreamTestClient(t, test.ack)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			sent, sendErr := client.SendPending(ctx, state.Credentials{
				AgentID:      "e9e3d6df-8cbe-4d48-92e6-450fb7c132a1",
				SessionToken: "01234567890123456789012345678901",
			}, store)
			if test.wantErr && sendErr == nil {
				t.Fatal("SendPending() should reject a missing, negative, or mismatched acknowledgement")
			}
			if !test.wantErr && sendErr != nil {
				t.Fatalf("SendPending() error = %v", sendErr)
			}
			if !test.wantErr && sent != 1 {
				t.Fatalf("SendPending() sent %d batches, want 1", sent)
			}
			pending, err := store.Peek(10)
			if err != nil {
				t.Fatal(err)
			}
			if test.wantErr {
				if len(pending) != 1 || pending[0].ID != batch.ID {
					t.Fatalf("unacknowledged WAL batch was lost: %#v", pending)
				}
			} else if len(pending) != 0 {
				t.Fatalf("durably acknowledged WAL batch was retained: %#v", pending)
			}
		})
	}
}
