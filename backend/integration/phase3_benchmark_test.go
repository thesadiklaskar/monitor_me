//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	monitorv1 "github.com/WorkRCS/monitor_me/backend/gen/monitor/v1"
	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// BenchmarkPhase3PostgresIngestion measures the real TLS gRPC -> validated
// batch -> PostgreSQL transaction path on the Docker Compose CI runner.
func BenchmarkPhase3PostgresIngestion(b *testing.B) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	grpcAddress := os.Getenv("AGENT_GRPC_ADDRESS")
	rootCAPath := os.Getenv("AGENT_GRPC_ROOT_CA")
	if databaseURL == "" || grpcAddress == "" || rootCAPath == "" {
		b.Fatal("TEST_DATABASE_URL, AGENT_GRPC_ADDRESS, and AGENT_GRPC_ROOT_CA are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		b.Fatal(err)
	}
	defer database.Close()

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	var organizationID, serverID, agentID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO organizations (name, slug)
		VALUES ('Telemetry Benchmark', $1) RETURNING id::text
	`, "telemetry-benchmark-"+suffix).Scan(&organizationID); err != nil {
		b.Fatal(err)
	}
	defer database.ExecContext(context.Background(), `DELETE FROM organizations WHERE id = $1`, organizationID)
	if err := database.QueryRowContext(ctx, `
		INSERT INTO servers (organization_id, name, hostname, status)
		VALUES ($1, 'benchmark-node', 'benchmark-node', 'pending')
		RETURNING id::text
	`, organizationID).Scan(&serverID); err != nil {
		b.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, `
		INSERT INTO agents (server_id, version, status)
		VALUES ($1, 'benchmark', 'online') RETURNING id::text
	`, serverID).Scan(&agentID); err != nil {
		b.Fatal(err)
	}
	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		b.Fatal(err)
	}
	sessionToken := base64.RawURLEncoding.EncodeToString(rawToken)
	sessionHash := sha256.Sum256([]byte(sessionToken))
	if _, err := database.ExecContext(ctx, `
		INSERT INTO agent_sessions (agent_id, token_hash, expires_at)
		VALUES ($1, $2, now() + interval '1 hour')
	`, agentID, sessionHash[:]); err != nil {
		b.Fatal(err)
	}

	certificatePEM, err := os.ReadFile(rootCAPath)
	if err != nil {
		b.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certificatePEM) {
		b.Fatal("configured gRPC root certificate is invalid")
	}
	connection, err := grpc.NewClient(
		grpcAddress,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			RootCAs: roots, ServerName: "localhost", MinVersion: tls.VersionTLS12,
		})),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer connection.Close()
	client := monitorv1.NewAgentControlClient(connection)
	stream, err := client.Stream(ctx)
	if err != nil {
		b.Fatal(err)
	}

	samples := make([]*monitorv1.MetricSample, 64)
	for index := range samples {
		samples[index] = &monitorv1.MetricSample{
			MetricName:          "host.cpu.utilization",
			Value:               float64(index),
			Unit:                "percent",
			Labels:              map[string]string{"cpu": "cpu" + strconv.Itoa(index%8), "benchmark": "true"},
			ObservedAtUnixNanos: time.Now().UnixNano(),
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		var randomID [16]byte
		if _, err := rand.Read(randomID[:]); err != nil {
			b.Fatal(err)
		}
		batch := &monitorv1.TelemetryBatch{
			BatchId:              hex.EncodeToString(randomID[:]),
			CollectedAtUnixNanos: time.Now().UnixNano(),
			SchemaVersion:        1,
			Samples:              samples,
		}
		message := &monitorv1.AgentToServerMessage{
			AgentId:      agentID,
			SessionToken: sessionToken,
			Payload:      &monitorv1.AgentToServerMessage_TelemetryBatch{TelemetryBatch: batch},
		}
		if err := stream.Send(message); err != nil {
			b.Fatal(err)
		}
		ack, err := stream.Recv()
		if err != nil {
			b.Fatal(err)
		}
		if ack.GetAcknowledgement() == nil || !ack.GetAcknowledgement().GetAccepted() {
			b.Fatalf("ingestion batch was not durably acknowledged: %v", ack)
		}
	}
	b.StopTimer()
	elapsed := b.Elapsed().Seconds()
	if elapsed > 0 {
		b.ReportMetric(float64(b.N*len(samples))/elapsed, "samples/sec")
	}
	if err := stream.CloseSend(); err != nil {
		b.Fatal(fmt.Errorf("close benchmark stream: %w", err))
	}
}
