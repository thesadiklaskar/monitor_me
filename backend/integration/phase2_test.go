//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	monitorv1 "github.com/WorkRCS/monitor_me/backend/gen/monitor/v1"
	"github.com/WorkRCS/monitor_me/backend/internal/security"
	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc/status"
)

type apiResponse struct {
	AccessToken string `json:"access_token"`
	User        struct {
		ID             string `json:"id"`
		Role           string `json:"role"`
		OrganizationID string `json:"organization_id"`
	} `json:"user"`
	Server struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"server"`
	Enrollment struct {
		TokenID   string    `json:"token_id"`
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	} `json:"enrollment"`
}

type httpResult struct {
	StatusCode int
	Body       string
}

func TestPhase2AuthenticationAndAgentLifecycle(t *testing.T) {
	baseURL := requiredEnv(t, "API_BASE_URL")
	grpcAddress := requiredEnv(t, "AGENT_GRPC_ADDRESS")
	rootCAPath := requiredEnv(t, "AGENT_GRPC_ROOT_CA")
	databaseURL := requiredEnv(t, "TEST_DATABASE_URL")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer database.Close()

	registerBody := map[string]any{
		"email":             "phase2-owner@example.invalid",
		"display_name":      "Phase Two Owner",
		"password":          "a-strong-initial-password-2026",
		"organization_name": "Phase Two Test Organization",
		"organization_slug": "phase-two-test-org",
	}
	registerResponse := requestJSON(t, ctx, http.MethodPost, baseURL+"/v1/auth/register", "", registerBody)
	if registerResponse.StatusCode != http.StatusCreated {
		t.Fatalf("initial owner registration: expected 201, got %d: %s", registerResponse.StatusCode, registerResponse.Body)
	}
	var registered apiResponse
	decodeResponse(t, registerResponse.Body, &registered)
	if registered.User.Role != "owner" || registered.User.ID == "" || registered.User.OrganizationID == "" {
		t.Fatalf("unexpected bootstrap user: %#v", registered.User)
	}

	duplicateRegister := requestJSON(t, ctx, http.MethodPost, baseURL+"/v1/auth/register", "", registerBody)
	if duplicateRegister.StatusCode != http.StatusForbidden {
		t.Fatalf("second bootstrap registration: expected 403, got %d", duplicateRegister.StatusCode)
	}
	wrongPassword := requestJSON(t, ctx, http.MethodPost, baseURL+"/v1/auth/login", "", map[string]any{
		"email": registerBody["email"], "password": "incorrect-password",
	})
	if wrongPassword.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password login: expected 401, got %d", wrongPassword.StatusCode)
	}

	loginResponse := requestJSON(t, ctx, http.MethodPost, baseURL+"/v1/auth/login", "", map[string]any{
		"email": registerBody["email"], "password": registerBody["password"],
	})
	if loginResponse.StatusCode != http.StatusOK {
		t.Fatalf("owner login: expected 200, got %d: %s", loginResponse.StatusCode, loginResponse.Body)
	}
	var session apiResponse
	decodeResponse(t, loginResponse.Body, &session)
	if len(session.AccessToken) < 32 {
		t.Fatal("login did not return an opaque bearer session")
	}
	bearer := session.AccessToken

	me := requestJSON(t, ctx, http.MethodGet, baseURL+"/v1/auth/me", bearer, nil)
	if me.StatusCode != http.StatusOK {
		t.Fatalf("authenticated /me: expected 200, got %d", me.StatusCode)
	}

	// A viewer can inspect the organization but cannot provision infrastructure.
	viewerPassword := "viewer-password-for-phase-two"
	viewerHash, err := security.HashPassword(viewerPassword)
	if err != nil {
		t.Fatalf("hash viewer password: %v", err)
	}
	var viewerID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO users (email, display_name, password_hash)
		VALUES ('phase2-viewer@example.invalid', 'Phase Two Viewer', $1)
		RETURNING id::text
	`, viewerHash).Scan(&viewerID); err != nil {
		t.Fatalf("create viewer account for authorization test: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO organization_memberships (organization_id, user_id, role)
		VALUES ($1, $2, 'viewer')
	`, registered.User.OrganizationID, viewerID); err != nil {
		t.Fatalf("create viewer membership: %v", err)
	}
	viewerLogin := requestJSON(t, ctx, http.MethodPost, baseURL+"/v1/auth/login", "", map[string]any{
		"email": "phase2-viewer@example.invalid", "password": viewerPassword,
	})
	if viewerLogin.StatusCode != http.StatusOK {
		t.Fatalf("viewer login: expected 200, got %d: %s", viewerLogin.StatusCode, viewerLogin.Body)
	}
	var viewerSession apiResponse
	decodeResponse(t, viewerLogin.Body, &viewerSession)
	viewerCreate := requestJSON(t, ctx, http.MethodPost, baseURL+"/v1/servers", viewerSession.AccessToken, map[string]any{"name": "forbidden-node"})
	if viewerCreate.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer server creation: expected 403, got %d", viewerCreate.StatusCode)
	}
	viewerList := requestJSON(t, ctx, http.MethodGet, baseURL+"/v1/servers", viewerSession.AccessToken, nil)
	if viewerList.StatusCode != http.StatusOK {
		t.Fatalf("viewer inventory read: expected 200, got %d", viewerList.StatusCode)
	}

	serverResponse := createServer(t, ctx, baseURL, bearer, "integration-node")
	serverID, enrollmentToken := serverResponse.Server.ID, serverResponse.Enrollment.Token
	if serverID == "" || len(enrollmentToken) < 32 {
		t.Fatalf("server creation did not return a scoped enrollment token: %#v", serverResponse)
	}
	if time.Until(serverResponse.Enrollment.ExpiresAt) <= 0 || time.Until(serverResponse.Enrollment.ExpiresAt) > 16*time.Minute {
		t.Fatalf("unexpected enrollment token expiry: %s", serverResponse.Enrollment.ExpiresAt)
	}

	// A viewer cannot revoke an enrollment credential created by an owner.
	viewerRevoke := requestJSON(t, ctx, http.MethodPost,
		baseURL+"/v1/enrollment-tokens/"+serverResponse.Enrollment.TokenID+"/revoke",
		viewerSession.AccessToken, map[string]any{})
	if viewerRevoke.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer token revocation: expected 403, got %d", viewerRevoke.StatusCode)
	}

	// A member of a different organization must never see this server.
	otherPassword := "second-organization-owner-password"
	otherHash, err := security.HashPassword(otherPassword)
	if err != nil {
		t.Fatalf("hash second organization password: %v", err)
	}
	var otherOrganizationID, otherUserID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO organizations (name, slug)
		VALUES ('Second Test Organization', 'second-test-organization')
		RETURNING id::text
	`).Scan(&otherOrganizationID); err != nil {
		t.Fatalf("create second organization: %v", err)
	}
	if err := database.QueryRowContext(ctx, `
		INSERT INTO users (email, display_name, password_hash)
		VALUES ('second-owner@example.invalid', 'Second Organization Owner', $1)
		RETURNING id::text
	`, otherHash).Scan(&otherUserID); err != nil {
		t.Fatalf("create second organization user: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO organization_memberships (organization_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, otherOrganizationID, otherUserID); err != nil {
		t.Fatalf("create second organization membership: %v", err)
	}
	otherLogin := requestJSON(t, ctx, http.MethodPost, baseURL+"/v1/auth/login", "", map[string]any{
		"email": "second-owner@example.invalid", "password": otherPassword,
	})
	if otherLogin.StatusCode != http.StatusOK {
		t.Fatalf("second organization login: expected 200, got %d: %s", otherLogin.StatusCode, otherLogin.Body)
	}
	var otherSession apiResponse
	decodeResponse(t, otherLogin.Body, &otherSession)
	otherInventoryResponse := requestJSON(t, ctx, http.MethodGet, baseURL+"/v1/servers", otherSession.AccessToken, nil)
	if otherInventoryResponse.StatusCode != http.StatusOK {
		t.Fatalf("second organization inventory: expected 200, got %d", otherInventoryResponse.StatusCode)
	}
	var otherInventory struct {
		Servers []json.RawMessage `json:"servers"`
	}
	decodeResponse(t, otherInventoryResponse.Body, &otherInventory)
	if len(otherInventory.Servers) != 0 {
		t.Fatalf("cross-organization inventory leak: received %d foreign servers", len(otherInventory.Servers))
	}

	certificatePEM, err := os.ReadFile(rootCAPath)
	if err != nil {
		t.Fatalf("read gRPC root certificate: %v", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certificatePEM) {
		t.Fatal("could not add development CA certificate")
	}
	connection, err := grpc.NewClient(
		grpcAddress,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			RootCAs: roots, ServerName: "localhost", MinVersion: tls.VersionTLS12,
		})),
	)
	if err != nil {
		t.Fatalf("create TLS gRPC client: %v", err)
	}
	defer connection.Close()
	agentClient := monitorv1.NewAgentControlClient(connection)

	invalidEnrollment, err := agentClient.Enroll(ctx, &monitorv1.EnrollRequest{
		EnrollmentToken: strings.Repeat("x", 43),
		AgentVersion:    "0.1.0-test", OperatingSystem: "linux", Architecture: "amd64", Hostname: "invalid-node",
	})
	if err == nil || status.Code(err) != codes.Unauthenticated {
		t.Fatalf("invalid token should be unauthenticated; response=%v error=%v", invalidEnrollment, err)
	}

	enrolled, err := agentClient.Enroll(ctx, &monitorv1.EnrollRequest{
		EnrollmentToken: enrollmentToken,
		AgentVersion:    "0.1.0-test",
		OperatingSystem: "linux",
		Architecture:    "amd64",
		Hostname:        "integration-node",
		Capabilities:    []string{"host.cpu", "host.memory", "host.disk", "host.cpu"},
	})
	if err != nil {
		t.Fatalf("valid TLS enrollment failed: %v", err)
	}
	if enrolled.GetAgentId() == "" || enrolled.GetServerId() != serverID || len(enrolled.GetSessionToken()) < 32 {
		t.Fatalf("enrollment response is incomplete: %#v", enrolled)
	}
	if enrolled.GetHeartbeatIntervalSeconds() != 15 || enrolled.GetProtocolVersion() != "monitor.v1" {
		t.Fatalf("unexpected protocol settings: %#v", enrolled)
	}

	heartbeat, err := agentClient.ReportHeartbeat(ctx, &monitorv1.HeartbeatRequest{
		AgentId:          enrolled.GetAgentId(),
		SessionToken:     enrolled.GetSessionToken(),
		AgentVersion:     "0.1.0-test",
		Capabilities:     []string{"host.cpu", "host.memory"},
		SentAtUnixMillis: time.Now().UnixMilli(),
	})
	if err != nil || !heartbeat.GetAccepted() {
		t.Fatalf("valid heartbeat was not accepted: response=%v error=%v", heartbeat, err)
	}

	// A process restart or network reconnect must reuse the persisted session credential.
	if err := connection.Close(); err != nil {
		t.Fatalf("close first gRPC connection: %v", err)
	}
	reconnectedConnection, err := grpc.NewClient(
		grpcAddress,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			RootCAs: roots, ServerName: "localhost", MinVersion: tls.VersionTLS12,
		})),
	)
	if err != nil {
		t.Fatalf("create reconnected TLS gRPC client: %v", err)
	}
	defer reconnectedConnection.Close()
	agentClient = monitorv1.NewAgentControlClient(reconnectedConnection)
	reconnectedHeartbeat, err := agentClient.ReportHeartbeat(ctx, &monitorv1.HeartbeatRequest{
		AgentId:          enrolled.GetAgentId(),
		SessionToken:     enrolled.GetSessionToken(),
		AgentVersion:     "0.1.0-test",
		Capabilities:     []string{"host.cpu", "host.memory"},
		SentAtUnixMillis: time.Now().UnixMilli(),
	})
	if err != nil || !reconnectedHeartbeat.GetAccepted() {
		t.Fatalf("heartbeat after reconnect was not accepted: response=%v error=%v", reconnectedHeartbeat, err)
	}

	// Reissue a single-use token for the same server and rotate its agent session.
	rotationResponse := requestJSON(t, ctx, http.MethodPost,
		baseURL+"/v1/servers/"+serverID+"/enrollment-token", bearer, map[string]any{})
	if rotationResponse.StatusCode != http.StatusCreated {
		t.Fatalf("reissue enrollment token: expected 201, got %d: %s", rotationResponse.StatusCode, rotationResponse.Body)
	}
	var rotationResult apiResponse
	decodeResponse(t, rotationResponse.Body, &rotationResult)
	oldSessionToken := enrolled.GetSessionToken()
	rotatedAgent, err := agentClient.Enroll(ctx, &monitorv1.EnrollRequest{
		EnrollmentToken: rotationResult.Enrollment.Token,
		AgentVersion:    "0.1.1-test", OperatingSystem: "linux", Architecture: "amd64",
		Hostname: "integration-node", Capabilities: []string{"host.cpu"},
	})
	if err != nil {
		t.Fatalf("re-enrollment with replacement token failed: %v", err)
	}
	if rotatedAgent.GetAgentId() != enrolled.GetAgentId() {
		t.Fatalf("re-enrollment must preserve stable agent identity: old=%s new=%s", enrolled.GetAgentId(), rotatedAgent.GetAgentId())
	}
	if _, err := agentClient.ReportHeartbeat(ctx, &monitorv1.HeartbeatRequest{
		AgentId: enrolled.GetAgentId(), SessionToken: oldSessionToken, AgentVersion: "0.1.0-test",
	}); err == nil || status.Code(err) != codes.Unauthenticated {
		t.Fatalf("rotated agent session must be invalidated, got %v", err)
	}
	enrolled = rotatedAgent

	reusedEnrollment, err := agentClient.Enroll(ctx, &monitorv1.EnrollRequest{
		EnrollmentToken: enrollmentToken,
		AgentVersion:    "0.1.0-test", OperatingSystem: "linux", Architecture: "amd64", Hostname: "integration-node",
	})
	if err == nil || status.Code(err) != codes.Unauthenticated {
		t.Fatalf("consumed token should not be reusable; response=%v error=%v", reusedEnrollment, err)
	}

	expiredServer := createServer(t, ctx, baseURL, bearer, "expired-token-node")
	expiredHash := sha256.Sum256([]byte(expiredServer.Enrollment.Token))
	if _, err := database.ExecContext(ctx,
		"UPDATE agent_enrollment_tokens SET expires_at = now() - interval '1 minute' WHERE token_hash = $1",
		expiredHash[:],
	); err != nil {
		t.Fatalf("expire enrollment token for boundary test: %v", err)
	}
	expiredResponse, err := agentClient.Enroll(ctx, &monitorv1.EnrollRequest{
		EnrollmentToken: expiredServer.Enrollment.Token,
		AgentVersion:    "0.1.0-test", OperatingSystem: "linux", Architecture: "amd64", Hostname: "expired-node",
	})
	if err == nil || status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expired token should be unauthenticated; response=%v error=%v", expiredResponse, err)
	}

	revokedServer := createServer(t, ctx, baseURL, bearer, "revoked-token-node")
	revokeToken := requestJSON(t, ctx, http.MethodPost,
		baseURL+"/v1/enrollment-tokens/"+revokedServer.Enrollment.TokenID+"/revoke", bearer, map[string]any{})
	if revokeToken.StatusCode != http.StatusNoContent {
		t.Fatalf("enrollment token revocation: expected 204, got %d: %s", revokeToken.StatusCode, revokeToken.Body)
	}
	revokedResponse, err := agentClient.Enroll(ctx, &monitorv1.EnrollRequest{
		EnrollmentToken: revokedServer.Enrollment.Token,
		AgentVersion:    "0.1.0-test", OperatingSystem: "linux", Architecture: "amd64", Hostname: "revoked-node",
	})
	if err == nil || status.Code(err) != codes.Unauthenticated {
		t.Fatalf("revoked token should be unauthenticated; response=%v error=%v", revokedResponse, err)
	}

	stream, err := agentClient.Stream(ctx)
	if err != nil {
		t.Fatalf("open authenticated telemetry stream: %v", err)
	}

	// Create a project mapping so ingestion proves that workload selectors are
	// attributed only through an organization-scoped mapping.
	var projectID string
	if err := database.QueryRowContext(ctx, `
		INSERT INTO projects (organization_id, name, slug)
		VALUES ($1, 'Phase Three Project', 'phase-three-project')
		RETURNING id::text
	`, registered.User.OrganizationID).Scan(&projectID); err != nil {
		t.Fatalf("create project for workload attribution: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO server_project_mappings (
			organization_id, server_id, project_id, workload_type, workload_selector, attribution_method
		)
		VALUES ($1, $2, $3, 'systemd', '/system.slice/example.service', 'systemd')
	`, registered.User.OrganizationID, serverID, projectID); err != nil {
		t.Fatalf("create Systemd project mapping: %v", err)
	}

	ticketResponse := requestJSON(t, ctx, http.MethodPost, baseURL+"/v1/live/ticket", bearer, map[string]any{})
	if ticketResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create live ticket: expected 201, got %d: %s", ticketResponse.StatusCode, ticketResponse.Body)
	}
	var liveTicket struct {
		Ticket string `json:"ticket"`
	}
	decodeResponse(t, ticketResponse.Body, &liveTicket)
	liveURL := "ws" + strings.TrimPrefix(baseURL, "http") + "/v1/live/ws?ticket=" + url.QueryEscape(liveTicket.Ticket)
	liveHeaders := http.Header{"Origin": []string{baseURL}}
	liveConnection, liveResponse, err := websocket.DefaultDialer.DialContext(ctx, liveURL, liveHeaders)
	if err != nil {
		if liveResponse != nil && liveResponse.Body != nil {
			liveResponse.Body.Close()
		}
		t.Fatalf("connect authorized organization live stream: response=%v error=%v", liveResponse, err)
	}
	defer liveConnection.Close()

	collectedAt := time.Now().UTC().UnixNano()
	observedAt := collectedAt
	batchID := "0123456789abcdef0123456789abcdef"
	telemetryMessage := func(value float64) *monitorv1.AgentToServerMessage {
		return &monitorv1.AgentToServerMessage{
			AgentId: enrolled.GetAgentId(), SessionToken: enrolled.GetSessionToken(),
			Payload: &monitorv1.AgentToServerMessage_TelemetryBatch{
				TelemetryBatch: &monitorv1.TelemetryBatch{
					BatchId: batchID, CollectedAtUnixNanos: collectedAt, SchemaVersion: 1,
					Samples: []*monitorv1.MetricSample{
						{
							MetricName: "host.memory.used", Value: value, Unit: "bytes",
							Labels:              map[string]string{"source": "phase3-integration"},
							ObservedAtUnixNanos: observedAt,
						},
						{
							MetricName: "process.memory.rss", Value: 2048, Unit: "bytes",
							Labels: map[string]string{
								"pid": "123", "process": "fixture-worker",
								"workload_type":      "systemd",
								"workload_selector":  "/system.slice/example.service",
								"attribution_method": "systemd",
							},
							ObservedAtUnixNanos: observedAt,
						},
					},
				},
			},
		}
	}
	if err := stream.Send(telemetryMessage(1048576)); err != nil {
		t.Fatalf("send first telemetry batch: %v", err)
	}
	firstAck, err := stream.Recv()
	if err != nil || firstAck.GetAcknowledgement() == nil || !firstAck.GetAcknowledgement().GetAccepted() {
		t.Fatalf("committed telemetry batch was not acknowledged: response=%v error=%v", firstAck, err)
	}
	if err := liveConnection.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var liveEvent struct {
		Type           string `json:"type"`
		OrganizationID string `json:"organization_id"`
		ServerID       string `json:"server_id"`
		BatchID        string `json:"batch_id"`
	}
	if err := liveConnection.ReadJSON(&liveEvent); err != nil {
		t.Fatalf("read post-commit live event: %v", err)
	}
	if liveEvent.Type != "telemetry.batch" || liveEvent.OrganizationID != registered.User.OrganizationID ||
		liveEvent.ServerID != serverID || liveEvent.BatchID != batchID {
		t.Fatalf("live event has incorrect tenant or batch scope: %+v", liveEvent)
	}
	_ = liveConnection.Close()

	// A ticket is single-use. Replaying it cannot reopen a stream.
	replayConnection, replayResponse, replayErr := websocket.DefaultDialer.DialContext(ctx, liveURL, liveHeaders)
	if replayErr == nil {
		_ = replayConnection.Close()
		t.Fatal("a consumed live ticket must not be reusable")
	}
	if replayResponse == nil || replayResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("replayed live ticket should return 401, got response=%v error=%v", replayResponse, replayErr)
	}
	if replayResponse.Body != nil {
		replayResponse.Body.Close()
	}

	// A fresh ticket restores the live stream after a normal reconnect.
	reconnectTicketResponse := requestJSON(t, ctx, http.MethodPost, baseURL+"/v1/live/ticket", bearer, map[string]any{})
	if reconnectTicketResponse.StatusCode != http.StatusCreated {
		t.Fatalf("issue reconnect ticket: expected 201, got %d: %s", reconnectTicketResponse.StatusCode, reconnectTicketResponse.Body)
	}
	var reconnectTicket struct {
		Ticket string `json:"ticket"`
	}
	decodeResponse(t, reconnectTicketResponse.Body, &reconnectTicket)
	reconnectURL := "ws" + strings.TrimPrefix(baseURL, "http") + "/v1/live/ws?ticket=" + url.QueryEscape(reconnectTicket.Ticket)
	reconnectedLive, reconnectResponse, err := websocket.DefaultDialer.DialContext(ctx, reconnectURL, liveHeaders)
	if err != nil {
		if reconnectResponse != nil && reconnectResponse.Body != nil {
			reconnectResponse.Body.Close()
		}
		t.Fatalf("reconnect to organization live stream: response=%v error=%v", reconnectResponse, err)
	}
	defer reconnectedLive.Close()
	if err := stream.Send(telemetryMessage(1048576)); err != nil {
		t.Fatalf("send retry of telemetry batch: %v", err)
	}
	retryAck, err := stream.Recv()
	if err != nil || retryAck.GetAcknowledgement() == nil || !retryAck.GetAcknowledgement().GetAccepted() {
		t.Fatalf("idempotent telemetry retry was not acknowledged: response=%v error=%v", retryAck, err)
	}
	if err := stream.Send(telemetryMessage(2097152)); err != nil {
		t.Fatalf("send conflicting telemetry batch: %v", err)
	}
	conflictAck, err := stream.Recv()
	if err != nil || conflictAck.GetAcknowledgement() == nil ||
		conflictAck.GetAcknowledgement().GetAccepted() ||
		conflictAck.GetAcknowledgement().GetRejectionCode() != "idempotency_conflict" {
		t.Fatalf("reused idempotency key with changed data must be rejected: response=%v error=%v", conflictAck, err)
	}
	secondBatchID := "fedcba9876543210fedcba9876543210"
	secondMessage := &monitorv1.AgentToServerMessage{
		AgentId: enrolled.GetAgentId(), SessionToken: enrolled.GetSessionToken(),
		Payload: &monitorv1.AgentToServerMessage_TelemetryBatch{
			TelemetryBatch: &monitorv1.TelemetryBatch{
				BatchId: secondBatchID, CollectedAtUnixNanos: time.Now().UTC().UnixNano(), SchemaVersion: 1,
				Samples: []*monitorv1.MetricSample{{
					MetricName: "host.memory.used", Value: 2097152, Unit: "bytes",
					Labels: map[string]string{"source": "phase3-reconnect"},
					ObservedAtUnixNanos: time.Now().UTC().UnixNano(),
				}},
			},
		},
	}
	if err := stream.Send(secondMessage); err != nil {
		t.Fatalf("send telemetry after websocket reconnect: %v", err)
	}
	secondAck, err := stream.Recv()
	if err != nil || secondAck.GetAcknowledgement() == nil || !secondAck.GetAcknowledgement().GetAccepted() {
		t.Fatalf("telemetry after websocket reconnect was not stored: response=%v error=%v", secondAck, err)
	}
	if err := reconnectedLive.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var reconnectedEvent struct {
		Type    string `json:"type"`
		BatchID string `json:"batch_id"`
	}
	if err := reconnectedLive.ReadJSON(&reconnectedEvent); err != nil {
		t.Fatalf("read live event after reconnect: %v", err)
	}
	if reconnectedEvent.Type != "telemetry.batch" || reconnectedEvent.BatchID != secondBatchID {
		t.Fatalf("unexpected event after reconnect: %+v", reconnectedEvent)
	}

	// The history endpoint is organization-scoped and uses bounded aggregation.
	seriesValues := url.Values{
		"metric": []string{"host.memory.used"},
		"from":   []string{time.Now().Add(-30 * time.Second).Format(time.RFC3339Nano)},
		"to":     []string{time.Now().Add(time.Second).Format(time.RFC3339Nano)},
		"step":   []string{"10s"},
		"limit":  []string{"10"},
	}
	seriesURL := baseURL + "/v1/servers/" + serverID + "/metrics?" + seriesValues.Encode()
	seriesResponse := requestJSON(t, ctx, http.MethodGet, seriesURL, bearer, nil)
	if seriesResponse.StatusCode != http.StatusOK {
		t.Fatalf("server metrics history: expected 200, got %d: %s", seriesResponse.StatusCode, seriesResponse.Body)
	}
	var seriesResult struct {
		Points []json.RawMessage `json:"points"`
	}
	decodeResponse(t, seriesResponse.Body, &seriesResult)
	if len(seriesResult.Points) == 0 {
		t.Fatal("expected the persisted telemetry to be queryable from PostgreSQL history")
	}
	foreignSeries := requestJSON(t, ctx, http.MethodGet, seriesURL, otherSession.AccessToken, nil)
	if foreignSeries.StatusCode != http.StatusOK {
		t.Fatalf("foreign organization metrics query: expected 200, got %d", foreignSeries.StatusCode)
	}
	var foreignSeriesResult struct {
		Points []json.RawMessage `json:"points"`
	}
	decodeResponse(t, foreignSeries.Body, &foreignSeriesResult)
	if len(foreignSeriesResult.Points) != 0 {
		t.Fatalf("cross-organization telemetry leak: received %d metric points", len(foreignSeriesResult.Points))
	}

	var mappedProject, attributionMethod string
	if err := database.QueryRowContext(ctx, `
		SELECT project_id::text, attribution_method
		FROM telemetry_samples
		WHERE agent_id = $1 AND batch_id = $2 AND metric_name = 'process.memory.rss'
	`, enrolled.GetAgentId(), batchID).Scan(&mappedProject, &attributionMethod); err != nil {
		t.Fatalf("query workload-attributed telemetry: %v", err)
	}
	if mappedProject != projectID || attributionMethod != "systemd" {
		t.Fatalf("unexpected workload attribution: project=%s method=%s", mappedProject, attributionMethod)
	}

	if err := stream.CloseSend(); err != nil {
		t.Fatalf("close telemetry stream: %v", err)
	}
	var storedSamples int
	if err := database.QueryRowContext(ctx, `
		SELECT count(*) FROM telemetry_samples
		WHERE agent_id = $1 AND batch_id = $2
	`, enrolled.GetAgentId(), batchID).Scan(&storedSamples); err != nil {
		t.Fatalf("query persisted telemetry samples: %v", err)
	}
	if storedSamples != 3 {
		t.Fatalf("the first idempotent batch and second unique batch must persist three samples, got %d", storedSamples)
	}

	revokeAgent := requestJSON(t, ctx, http.MethodPost,
		baseURL+"/v1/agents/"+enrolled.GetAgentId()+"/revoke", bearer, map[string]any{})
	if revokeAgent.StatusCode != http.StatusNoContent {
		t.Fatalf("agent revocation: expected 204, got %d: %s", revokeAgent.StatusCode, revokeAgent.Body)
	}
	revokedHeartbeat, err := agentClient.ReportHeartbeat(ctx, &monitorv1.HeartbeatRequest{
		AgentId:      enrolled.GetAgentId(),
		SessionToken: enrolled.GetSessionToken(),
		AgentVersion: "0.1.0-test",
	})
	if err == nil || status.Code(err) != codes.Unauthenticated {
		t.Fatalf("revoked agent session should be rejected; response=%v error=%v", revokedHeartbeat, err)
	}

	servers := requestJSON(t, ctx, http.MethodGet, baseURL+"/v1/servers", bearer, nil)
	if servers.StatusCode != http.StatusOK {
		t.Fatalf("server inventory: expected 200, got %d: %s", servers.StatusCode, servers.Body)
	}
	logout := requestJSON(t, ctx, http.MethodPost, baseURL+"/v1/auth/logout", bearer, map[string]any{})
	if logout.StatusCode != http.StatusNoContent {
		t.Fatalf("logout: expected 204, got %d: %s", logout.StatusCode, logout.Body)
	}
	afterLogout := requestJSON(t, ctx, http.MethodGet, baseURL+"/v1/auth/me", bearer, nil)
	if afterLogout.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked user session: expected 401, got %d", afterLogout.StatusCode)
	}
}

func createServer(t *testing.T, ctx context.Context, baseURL, bearer, name string) apiResponse {
	t.Helper()
	response := requestJSON(t, ctx, http.MethodPost, baseURL+"/v1/servers", bearer, map[string]any{
		"name": name, "hostname": name,
	})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create server %q: expected 201, got %d: %s", name, response.StatusCode, response.Body)
	}
	var decoded apiResponse
	decodeResponse(t, response.Body, &decoded)
	return decoded
}

func requestJSON(t *testing.T, ctx context.Context, method, endpoint, bearer string, body any) httpResult {
	t.Helper()
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("create HTTP request: %v", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("HTTP %s %s failed: %v", method, endpoint, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read HTTP response: %v", err)
	}
	return httpResult{StatusCode: response.StatusCode, Body: string(responseBody)}
}

func decodeResponse(t *testing.T, body string, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), target); err != nil {
		t.Fatalf("decode API response: %v; body=%s", err, body)
	}
}

func requiredEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("required environment variable %s is not set", name)
	}
	return value
}
