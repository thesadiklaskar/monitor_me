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
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	monitorv1 "example.com/monitor-me/backend/gen/monitor/v1"
	"example.com/monitor-me/backend/internal/identity"
	"example.com/monitor-me/backend/internal/security"
	"github.com/gorilla/websocket"
	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
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
	// A user with memberships in two organizations must not be silently
	// authenticated into whichever organization happens to have the oldest row.
	if _, err := database.ExecContext(ctx, `
		INSERT INTO organization_memberships (organization_id, user_id, role)
		VALUES ($1, $2, 'viewer')
	`, otherOrganizationID, registered.User.ID); err != nil {
		t.Fatalf("create second membership for ambiguous-login test: %v", err)
	}
	ambiguousLogin := requestJSON(t, ctx, http.MethodPost, baseURL+"/v1/auth/login", "", map[string]any{
		"email": registerBody["email"], "password": registerBody["password"],
	})
	if ambiguousLogin.StatusCode != http.StatusConflict || !strings.Contains(ambiguousLogin.Body, "organization_selection_required") {
		t.Fatalf("multi-organization login must fail closed with 409: status=%d body=%s",
			ambiguousLogin.StatusCode, ambiguousLogin.Body)
	}
	if _, err := database.ExecContext(ctx, `
		DELETE FROM organization_memberships WHERE organization_id = $1 AND user_id = $2
	`, otherOrganizationID, registered.User.ID); err != nil {
		t.Fatalf("restore single-organization owner membership after test: %v", err)
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

	// Phase 4 project administration and server workload mapping authorization.
	projectCreate := requestJSON(t, ctx, http.MethodPost, baseURL+"/v1/projects", bearer, map[string]any{
		"name": "Payments API", "slug": "payments-api",
	})
	if projectCreate.StatusCode != http.StatusCreated {
		t.Fatalf("owner project creation: expected 201, got %d: %s", projectCreate.StatusCode, projectCreate.Body)
	}
	var createdProject struct {
		Project struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
	}
	decodeResponse(t, projectCreate.Body, &createdProject)
	if createdProject.Project.ID == "" || createdProject.Project.Name != "Payments API" {
		t.Fatalf("project creation returned an incomplete project: %#v", createdProject.Project)
	}

	viewerProjectCreate := requestJSON(t, ctx, http.MethodPost, baseURL+"/v1/projects", viewerSession.AccessToken, map[string]any{
		"name": "Forbidden Project", "slug": "forbidden-project",
	})
	if viewerProjectCreate.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer project creation: expected 403, got %d: %s", viewerProjectCreate.StatusCode, viewerProjectCreate.Body)
	}

	mappingCreate := requestJSON(t, ctx, http.MethodPost,
		baseURL+"/v1/servers/"+serverID+"/project-mappings", bearer, map[string]any{
			"project_id":         createdProject.Project.ID,
			"workload_type":      "systemd",
			"workload_selector":  "payments-api.service",
			"attribution_method": "systemd",
		})
	if mappingCreate.StatusCode != http.StatusCreated {
		t.Fatalf("owner workload mapping creation: expected 201, got %d: %s", mappingCreate.StatusCode, mappingCreate.Body)
	}
	var createdMapping struct {
		Mapping struct {
			ID               string `json:"id"`
			ProjectID        string `json:"project_id"`
			WorkloadSelector string `json:"workload_selector"`
		} `json:"mapping"`
	}
	decodeResponse(t, mappingCreate.Body, &createdMapping)
	if createdMapping.Mapping.ID == "" || createdMapping.Mapping.ProjectID != createdProject.Project.ID ||
		createdMapping.Mapping.WorkloadSelector != "payments-api.service" {
		t.Fatalf("mapping creation returned an incomplete mapping: %#v", createdMapping.Mapping)
	}

	mappingList := requestJSON(t, ctx, http.MethodGet,
		baseURL+"/v1/servers/"+serverID+"/project-mappings", viewerSession.AccessToken, nil)
	if mappingList.StatusCode != http.StatusOK {
		t.Fatalf("viewer workload mapping list: expected 200, got %d: %s", mappingList.StatusCode, mappingList.Body)
	}
	var listedMappings struct {
		Mappings []struct {
			ID        string `json:"id"`
			ProjectID string `json:"project_id"`
		} `json:"mappings"`
	}
	decodeResponse(t, mappingList.Body, &listedMappings)
	if len(listedMappings.Mappings) != 1 || listedMappings.Mappings[0].ID != createdMapping.Mapping.ID {
		t.Fatalf("server workload mapping was not returned in organization scope: %#v", listedMappings.Mappings)
	}

	viewerMappingCreate := requestJSON(t, ctx, http.MethodPost,
		baseURL+"/v1/servers/"+serverID+"/project-mappings", viewerSession.AccessToken, map[string]any{
			"project_id":         createdProject.Project.ID,
			"workload_type":      "systemd",
			"workload_selector":  "forbidden.service",
			"attribution_method": "systemd",
		})
	if viewerMappingCreate.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer workload mapping creation: expected 403, got %d: %s", viewerMappingCreate.StatusCode, viewerMappingCreate.Body)
	}

	mappingDelete := requestJSON(t, ctx, http.MethodDelete,
		baseURL+"/v1/project-mappings/"+createdMapping.Mapping.ID, bearer, map[string]any{})
	if mappingDelete.StatusCode != http.StatusNoContent {
		t.Fatalf("owner workload mapping deletion: expected 204, got %d: %s", mappingDelete.StatusCode, mappingDelete.Body)
	}
	projectDelete := requestJSON(t, ctx, http.MethodDelete,
		baseURL+"/v1/projects/"+createdProject.Project.ID, bearer, map[string]any{})
	if projectDelete.StatusCode != http.StatusNoContent {
		t.Fatalf("owner project deletion: expected 204, got %d: %s", projectDelete.StatusCode, projectDelete.Body)
	}

	otherProjects := requestJSON(t, ctx, http.MethodGet, baseURL+"/v1/projects", otherSession.AccessToken, nil)
	if otherProjects.StatusCode != http.StatusOK {
		t.Fatalf("second organization project list: expected 200, got %d", otherProjects.StatusCode)
	}
	var otherProjectList struct {
		Projects []json.RawMessage `json:"projects"`
	}
	decodeResponse(t, otherProjects.Body, &otherProjectList)
	if len(otherProjectList.Projects) != 0 {
		t.Fatalf("cross-organization project leak: received %d foreign projects", len(otherProjectList.Projects))
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

	// Rotating a still-pending server credential invalidates the previous unused token.
	pendingServer := createServer(t, ctx, baseURL, bearer, "pending-token-rotation-node")
	pendingHash := sha256.Sum256([]byte(pendingServer.Enrollment.Token))
	pendingRotation := requestJSON(t, ctx, http.MethodPost,
		baseURL+"/v1/servers/"+pendingServer.Server.ID+"/enrollment-token", bearer, map[string]any{})
	if pendingRotation.StatusCode != http.StatusCreated {
		t.Fatalf("rotate pending server token: expected 201, got %d: %s", pendingRotation.StatusCode, pendingRotation.Body)
	}
	var pendingRotationResult apiResponse
	decodeResponse(t, pendingRotation.Body, &pendingRotationResult)
	var revokedAt sql.NullTime
	if err := database.QueryRowContext(ctx,
		"SELECT revoked_at FROM agent_enrollment_tokens WHERE token_hash = $1", pendingHash[:],
	).Scan(&revokedAt); err != nil {
		t.Fatalf("read replaced enrollment-token revocation: %v", err)
	}
	if !revokedAt.Valid {
		t.Fatal("reissuing an enrollment token must revoke the previous unused credential")
	}
	if _, err := agentClient.Enroll(ctx, &monitorv1.EnrollRequest{
		EnrollmentToken: pendingServer.Enrollment.Token,
		AgentVersion:    "0.1.0-test", OperatingSystem: "linux", Architecture: "amd64",
		Hostname: "pending-token-rotation-node",
	}); err == nil || status.Code(err) != codes.Unauthenticated {
		t.Fatalf("replaced enrollment token must be rejected, got %v", err)
	}
	if _, err := agentClient.Enroll(ctx, &monitorv1.EnrollRequest{
		EnrollmentToken: pendingRotationResult.Enrollment.Token,
		AgentVersion:    "0.1.0-test", OperatingSystem: "linux", Architecture: "amd64",
		Hostname: "pending-token-rotation-node",
	}); err != nil {
		t.Fatalf("replacement enrollment token should be accepted: %v", err)
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

	// A ticket is invalid as soon as its parent user session is logged out.
	revokedTicketResponse := requestJSON(t, ctx, http.MethodPost,
		baseURL+"/v1/live/ticket", viewerSession.AccessToken, map[string]any{})
	if revokedTicketResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create live ticket for logout test: expected 201, got %d: %s",
			revokedTicketResponse.StatusCode, revokedTicketResponse.Body)
	}
	var revokedTicket struct {
		Ticket string `json:"ticket"`
	}
	decodeResponse(t, revokedTicketResponse.Body, &revokedTicket)
	logoutViewer := requestJSON(t, ctx, http.MethodPost,
		baseURL+"/v1/auth/logout", viewerSession.AccessToken, map[string]any{})
	if logoutViewer.StatusCode != http.StatusNoContent {
		t.Fatalf("revoke viewer session: expected 204, got %d: %s", logoutViewer.StatusCode, logoutViewer.Body)
	}
	revokedTicketURL := "ws" + strings.TrimPrefix(baseURL, "http") +
		"/v1/live/ws?ticket=" + url.QueryEscape(revokedTicket.Ticket)
	revokedTicketConnection, revokedTicketResult, revokedTicketErr :=
		websocket.DefaultDialer.DialContext(ctx, revokedTicketURL, liveHeaders)
	if revokedTicketErr == nil {
		_ = revokedTicketConnection.Close()
		t.Fatal("a ticket must not outlive its revoked parent user session")
	}
	if revokedTicketResult == nil || revokedTicketResult.StatusCode != http.StatusUnauthorized {
		t.Fatalf("ticket from a revoked session should return 401: response=%v error=%v",
			revokedTicketResult, revokedTicketErr)
	}
	if revokedTicketResult.Body != nil {
		revokedTicketResult.Body.Close()
	}

	// A separate organization's valid ticket may subscribe successfully, but it
	// must never receive telemetry events published for this organization.
	foreignTicketResponse := requestJSON(t, ctx, http.MethodPost,
		baseURL+"/v1/live/ticket", otherSession.AccessToken, map[string]any{})
	if foreignTicketResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create foreign organization live ticket: expected 201, got %d: %s",
			foreignTicketResponse.StatusCode, foreignTicketResponse.Body)
	}
	var foreignTicket struct {
		Ticket string `json:"ticket"`
	}
	decodeResponse(t, foreignTicketResponse.Body, &foreignTicket)
	foreignLiveURL := "ws" + strings.TrimPrefix(baseURL, "http") +
		"/v1/live/ws?ticket=" + url.QueryEscape(foreignTicket.Ticket)
	foreignLiveConnection, foreignLiveResponse, err :=
		websocket.DefaultDialer.DialContext(ctx, foreignLiveURL, liveHeaders)
	if err != nil {
		if foreignLiveResponse != nil && foreignLiveResponse.Body != nil {
			foreignLiveResponse.Body.Close()
		}
		t.Fatalf("connect foreign organization live stream: response=%v error=%v",
			foreignLiveResponse, err)
	}
	defer foreignLiveConnection.Close()

	// Threshold rules are organization-scoped and can only be managed by owners/admins.
	alertRuleResponse := requestJSON(t, ctx, http.MethodPost, baseURL+"/v1/alert-rules", bearer, map[string]any{
		"name":        "High host memory",
		"server_id":   serverID,
		"metric_name": "host.memory.used",
		"operator":    "gt",
		"threshold":   500000,
	})
	if alertRuleResponse.StatusCode != http.StatusCreated {
		t.Fatalf("owner alert rule creation: expected 201, got %d: %s", alertRuleResponse.StatusCode, alertRuleResponse.Body)
	}
	var createdAlertRule struct {
		Rule struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"rule"`
	}
	decodeResponse(t, alertRuleResponse.Body, &createdAlertRule)
	if createdAlertRule.Rule.ID == "" || createdAlertRule.Rule.Name != "High host memory" {
		t.Fatalf("alert rule creation returned an incomplete rule: %#v", createdAlertRule.Rule)
	}
	viewerAlertRule := requestJSON(t, ctx, http.MethodPost, baseURL+"/v1/alert-rules", viewerSession.AccessToken, map[string]any{
		"name": "Viewer forbidden rule", "metric_name": "host.memory.used", "operator": "gt", "threshold": 100,
	})
	if viewerAlertRule.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer alert rule creation: expected 403, got %d: %s", viewerAlertRule.StatusCode, viewerAlertRule.Body)
	}
	foreignAlertRules := requestJSON(t, ctx, http.MethodGet, baseURL+"/v1/alert-rules", otherSession.AccessToken, nil)
	if foreignAlertRules.StatusCode != http.StatusOK {
		t.Fatalf("second organization alert rule list: expected 200, got %d", foreignAlertRules.StatusCode)
	}
	var foreignRules struct {
		Rules []json.RawMessage `json:"rules"`
	}
	decodeResponse(t, foreignAlertRules.Body, &foreignRules)
	if len(foreignRules.Rules) != 0 {
		t.Fatalf("cross-organization alert-rule leak: received %d foreign rules", len(foreignRules.Rules))
	}

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
						{
							MetricName: "process.memory.rss", Value: 512, Unit: "bytes",
							Labels: map[string]string{
								"pid": "124", "process": "unmapped-worker",
								"attribution_method": "unclassified",
							},
							ObservedAtUnixNanos: observedAt,
						},
						{
							MetricName: "host.network.receive.bytes_per_second", Value: 0, Unit: "bytes/second",
							Labels: map[string]string{
								"interface":          "shared-bridge",
								"attribution_method": "shared",
							},
							ObservedAtUnixNanos: observedAt,
						},
					},
				},
			},
		}
	}
	// A notification-store failure must roll back telemetry and let the
	// agent replay the same batch ID without losing the firing alert.
	if _, err := database.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION test_fail_firing_alert_notification()
		RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.kind = 'firing' THEN
				RAISE EXCEPTION 'injected alert notification failure';
			END IF;
			RETURN NEW;
		END;
		$$
	`); err != nil {
		t.Fatalf("create alert notification failure fixture: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		CREATE TRIGGER test_fail_firing_alert_notification
		BEFORE INSERT ON alert_notifications
		FOR EACH ROW EXECUTE FUNCTION test_fail_firing_alert_notification()
	`); err != nil {
		t.Fatalf("install alert notification failure fixture: %v", err)
	}
	failureStream, err := agentClient.Stream(ctx)
	if err != nil {
		t.Fatalf("open stream for injected alert storage failure: %v", err)
	}
	if err := failureStream.Send(telemetryMessage(1048576)); err != nil {
		t.Fatalf("send telemetry during injected alert storage failure: %v", err)
	}
	_, failureErr := failureStream.Recv()
	if status.Code(failureErr) != codes.Unavailable {
		t.Fatalf("alert notification storage failure must reject the telemetry batch for retry, got %v", failureErr)
	}
	_ = failureStream.CloseSend()
	var rolledBackBatches, rolledBackSamples, rolledBackEvents int
	if err := database.QueryRowContext(ctx, `
		SELECT count(*) FROM telemetry_batches WHERE agent_id = $1 AND batch_id = $2
	`, enrolled.GetAgentId(), batchID).Scan(&rolledBackBatches); err != nil {
		t.Fatalf("verify telemetry batch rollback after alert failure: %v", err)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT count(*) FROM telemetry_samples WHERE agent_id = $1 AND batch_id = $2
	`, enrolled.GetAgentId(), batchID).Scan(&rolledBackSamples); err != nil {
		t.Fatalf("verify telemetry sample rollback after alert failure: %v", err)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT count(*) FROM alert_events WHERE rule_id = $1 AND status = 'firing'
	`, createdAlertRule.Rule.ID).Scan(&rolledBackEvents); err != nil {
		t.Fatalf("verify alert-event rollback after notification failure: %v", err)
	}
	if rolledBackBatches != 0 || rolledBackSamples != 0 || rolledBackEvents != 0 {
		t.Fatalf("alert failure left partial monitoring data: batches=%d samples=%d events=%d",
			rolledBackBatches, rolledBackSamples, rolledBackEvents)
	}
	if _, err := database.ExecContext(ctx,
		"DROP TRIGGER test_fail_firing_alert_notification ON alert_notifications"); err != nil {
		t.Fatalf("remove alert notification failure trigger: %v", err)
	}
	if _, err := database.ExecContext(ctx,
		"DROP FUNCTION test_fail_firing_alert_notification()"); err != nil {
		t.Fatalf("remove alert notification failure function: %v", err)
	}

	if err := stream.Send(telemetryMessage(1048576)); err != nil {
		t.Fatalf("send first telemetry batch: %v", err)
	}
	firstAck, err := stream.Recv()
	if err != nil || firstAck.GetAcknowledgement() == nil || !firstAck.GetAcknowledgement().GetAccepted() {
		t.Fatalf("committed telemetry batch was not acknowledged: response=%v error=%v", firstAck, err)
	}

	firingAlerts := requestJSON(t, ctx, http.MethodGet, baseURL+"/v1/alerts", bearer, nil)
	if firingAlerts.StatusCode != http.StatusOK {
		t.Fatalf("firing alert list: expected 200, got %d: %s", firingAlerts.StatusCode, firingAlerts.Body)
	}
	var firingAlertList struct {
		Events []struct {
			RuleID string `json:"rule_id"`
			Status string `json:"status"`
		} `json:"events"`
	}
	decodeResponse(t, firingAlerts.Body, &firingAlertList)
	if len(firingAlertList.Events) != 1 || firingAlertList.Events[0].RuleID != createdAlertRule.Rule.ID || firingAlertList.Events[0].Status != "firing" {
		t.Fatalf("threshold breach did not create one firing alert event: %#v", firingAlertList.Events)
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
	if err := foreignLiveConnection.SetReadDeadline(time.Now().Add(250 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	_, _, foreignReadErr := foreignLiveConnection.ReadMessage()
	if foreignReadErr == nil {
		t.Fatal("telemetry from one organization must not reach another organization's subscriber")
	}
	var networkError net.Error
	if !errors.As(foreignReadErr, &networkError) || !networkError.Timeout() {
		t.Fatalf("foreign live subscription ended unexpectedly instead of remaining isolated: %v", foreignReadErr)
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

	// The transport accepts one bounded batch at a time; oversized metric lists
	// are rejected without inserting partial telemetry.
	oversizedSamples := make([]*monitorv1.MetricSample, 4097)
	oversizedObservedAt := time.Now().UTC().UnixNano()
	for index := range oversizedSamples {
		oversizedSamples[index] = &monitorv1.MetricSample{
			MetricName: "host.cpu.utilization", Value: 50, Unit: "percent",
			ObservedAtUnixNanos: oversizedObservedAt,
		}
	}
	oversizedMessage := &monitorv1.AgentToServerMessage{
		AgentId: enrolled.GetAgentId(), SessionToken: enrolled.GetSessionToken(),
		Payload: &monitorv1.AgentToServerMessage_TelemetryBatch{
			TelemetryBatch: &monitorv1.TelemetryBatch{
				BatchId:              "ffffffffffffffffffffffffffffffff",
				CollectedAtUnixNanos: oversizedObservedAt,
				SchemaVersion:        1,
				Samples:              oversizedSamples,
			},
		},
	}
	if err := stream.Send(oversizedMessage); err != nil {
		t.Fatalf("send oversized telemetry batch: %v", err)
	}
	oversizedAck, err := stream.Recv()
	if err != nil || oversizedAck.GetAcknowledgement() == nil ||
		oversizedAck.GetAcknowledgement().GetAccepted() ||
		oversizedAck.GetAcknowledgement().GetRejectionCode() != "invalid_batch" {
		t.Fatalf("oversized batch must be rejected with a bounded validation response: response=%v error=%v", oversizedAck, err)
	}
	secondBatchID := "fedcba9876543210fedcba9876543210"
	secondMessage := &monitorv1.AgentToServerMessage{
		AgentId: enrolled.GetAgentId(), SessionToken: enrolled.GetSessionToken(),
		Payload: &monitorv1.AgentToServerMessage_TelemetryBatch{
			TelemetryBatch: &monitorv1.TelemetryBatch{
				BatchId: secondBatchID, CollectedAtUnixNanos: time.Now().UTC().UnixNano(), SchemaVersion: 1,
				Samples: []*monitorv1.MetricSample{{
					MetricName: "host.memory.used", Value: 2097152, Unit: "bytes",
					Labels:              map[string]string{"source": "phase3-reconnect"},
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

	// A fresh below-threshold committed sample must resolve the firing alert
	// and create one recovery notification for each organization member.
	recoveryObservedAt := time.Now().UTC().UnixNano()
	recoveryMessage := &monitorv1.AgentToServerMessage{
		AgentId: enrolled.GetAgentId(), SessionToken: enrolled.GetSessionToken(),
		Payload: &monitorv1.AgentToServerMessage_TelemetryBatch{
			TelemetryBatch: &monitorv1.TelemetryBatch{
				BatchId:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				CollectedAtUnixNanos: recoveryObservedAt,
				SchemaVersion:        1,
				Samples: []*monitorv1.MetricSample{{
					MetricName: "host.memory.used", Value: 1024, Unit: "bytes",
					Labels:              map[string]string{"source": "alert-recovery-integration"},
					ObservedAtUnixNanos: recoveryObservedAt,
				}},
			},
		},
	}
	if err := stream.Send(recoveryMessage); err != nil {
		t.Fatalf("send recovery telemetry batch: %v", err)
	}
	recoveryAck, err := stream.Recv()
	if err != nil || recoveryAck.GetAcknowledgement() == nil || !recoveryAck.GetAcknowledgement().GetAccepted() {
		t.Fatalf("recovery telemetry batch was not acknowledged: response=%v error=%v", recoveryAck, err)
	}
	resolvedAlerts := requestJSON(t, ctx, http.MethodGet, baseURL+"/v1/alerts", bearer, nil)
	if resolvedAlerts.StatusCode != http.StatusOK {
		t.Fatalf("resolved alert list: expected 200, got %d: %s", resolvedAlerts.StatusCode, resolvedAlerts.Body)
	}
	var resolvedAlertList struct {
		Events []struct {
			RuleID string `json:"rule_id"`
			Status string `json:"status"`
		} `json:"events"`
	}
	decodeResponse(t, resolvedAlerts.Body, &resolvedAlertList)
	if len(resolvedAlertList.Events) != 1 || resolvedAlertList.Events[0].RuleID != createdAlertRule.Rule.ID || resolvedAlertList.Events[0].Status != "resolved" {
		t.Fatalf("below-threshold telemetry did not resolve the alert: %#v", resolvedAlertList.Events)
	}
	notificationResponse := requestJSON(t, ctx, http.MethodGet, baseURL+"/v1/notifications", bearer, nil)
	if notificationResponse.StatusCode != http.StatusOK {
		t.Fatalf("owner notification list: expected 200, got %d: %s", notificationResponse.StatusCode, notificationResponse.Body)
	}
	var notificationList struct {
		Notifications []struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		} `json:"notifications"`
	}
	decodeResponse(t, notificationResponse.Body, &notificationList)
	if len(notificationList.Notifications) != 2 {
		t.Fatalf("expected firing and recovery notifications, got %#v", notificationList.Notifications)
	}
	var firingNotificationID string
	seenKinds := map[string]bool{}
	for _, notification := range notificationList.Notifications {
		seenKinds[notification.Kind] = true
		if notification.Kind == "firing" {
			firingNotificationID = notification.ID
		}
	}
	if !seenKinds["firing"] || !seenKinds["recovered"] || firingNotificationID == "" {
		t.Fatalf("firing and recovery notifications were not both persisted: %#v", notificationList.Notifications)
	}
	markRead := requestJSON(t, ctx, http.MethodPost, baseURL+"/v1/notifications/"+firingNotificationID+"/read", bearer, map[string]any{})
	if markRead.StatusCode != http.StatusNoContent {
		t.Fatalf("mark notification as read: expected 204, got %d: %s", markRead.StatusCode, markRead.Body)
	}
	viewerNotifications := requestJSON(t, ctx, http.MethodGet, baseURL+"/v1/notifications", viewerSession.AccessToken, nil)
	if viewerNotifications.StatusCode != http.StatusOK {
		t.Fatalf("viewer notification list: expected 200, got %d", viewerNotifications.StatusCode)
	}
	var viewerNotificationList struct {
		Notifications []json.RawMessage `json:"notifications"`
	}
	decodeResponse(t, viewerNotifications.Body, &viewerNotificationList)
	if len(viewerNotificationList.Notifications) != 2 {
		t.Fatalf("alert notifications should be delivered to each organization member, got %d", len(viewerNotificationList.Notifications))
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
		Points []struct {
			Average     float64 `json:"average"`
			Minimum     float64 `json:"minimum"`
			Maximum     float64 `json:"maximum"`
			SampleCount int64   `json:"sample_count"`
		} `json:"points"`
	}
	decodeResponse(t, seriesResponse.Body, &seriesResult)
	if len(seriesResult.Points) == 0 {
		t.Fatal("expected the persisted telemetry to be queryable from PostgreSQL history")
	}
	var aggregateCount int64
	var weightedSum float64
	minimum, maximum := seriesResult.Points[0].Minimum, seriesResult.Points[0].Maximum
	for _, point := range seriesResult.Points {
		aggregateCount += point.SampleCount
		weightedSum += point.Average * float64(point.SampleCount)
		if point.Minimum < minimum {
			minimum = point.Minimum
		}
		if point.Maximum > maximum {
			maximum = point.Maximum
		}
	}
	if aggregateCount != 2 || minimum != 1048576 || maximum != 2097152 ||
		weightedSum/float64(aggregateCount) != 1572864 {
		t.Fatalf("historical buckets did not preserve expected aggregation: count=%d min=%v max=%v average=%v",
			aggregateCount, minimum, maximum, weightedSum/float64(aggregateCount))
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
		WHERE agent_id = $1 AND batch_id = $2
		  AND metric_name = 'process.memory.rss' AND labels->>'pid' = '123'
	`, enrolled.GetAgentId(), batchID).Scan(&mappedProject, &attributionMethod); err != nil {
		t.Fatalf("query workload-attributed telemetry: %v", err)
	}
	if mappedProject != projectID || attributionMethod != "systemd" {
		t.Fatalf("unexpected workload attribution: project=%s method=%s", mappedProject, attributionMethod)
	}
	var unclassifiedCount, sharedCount, hostTotalCount int
	if err := database.QueryRowContext(ctx, `
		SELECT
			count(*) FILTER (WHERE attribution_method = 'unclassified'),
			count(*) FILTER (WHERE attribution_method = 'shared'),
			count(*) FILTER (WHERE attribution_method = 'host_total')
		FROM telemetry_samples
		WHERE agent_id = $1 AND batch_id = $2
	`, enrolled.GetAgentId(), batchID).Scan(&unclassifiedCount, &sharedCount, &hostTotalCount); err != nil {
		t.Fatalf("query explicit attribution categories: %v", err)
	}
	if unclassifiedCount != 1 || sharedCount != 1 || hostTotalCount != 1 {
		t.Fatalf("shared, unclassified, and host-total resources must stay distinct: unclassified=%d shared=%d host_total=%d",
			unclassifiedCount, sharedCount, hostTotalCount)
	}

	if err := stream.CloseSend(); err != nil {
		t.Fatalf("close telemetry stream: %v", err)
	}
	var firstBatchSamples, secondBatchSamples int
	if err := database.QueryRowContext(ctx, `
		SELECT count(*) FROM telemetry_samples
		WHERE agent_id = $1 AND batch_id = $2
	`, enrolled.GetAgentId(), batchID).Scan(&firstBatchSamples); err != nil {
		t.Fatalf("query persisted first telemetry batch: %v", err)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT count(*) FROM telemetry_samples
		WHERE agent_id = $1 AND batch_id = $2
	`, enrolled.GetAgentId(), secondBatchID).Scan(&secondBatchSamples); err != nil {
		t.Fatalf("query persisted second telemetry batch: %v", err)
	}
	if firstBatchSamples != 4 || secondBatchSamples != 1 {
		t.Fatalf("idempotent batch must persist four samples and unique retry batch one: first=%d second=%d",
			firstBatchSamples, secondBatchSamples)
	}

	// A failed audit insert must roll back the queued command as well.
	// Inventory status is derived from the last authenticated heartbeat,
	// not a stale status field left behind by a disconnected agent.
	if _, err := database.ExecContext(ctx, `
		UPDATE agents SET last_seen_at = now() - interval '60 seconds' WHERE id = $1
	`, enrolled.GetAgentId()); err != nil {
		t.Fatalf("age agent heartbeat for stale-state test: %v", err)
	}
	readInventoryStatus := func() string {
		response := requestJSON(t, ctx, http.MethodGet, baseURL+"/v1/servers", bearer, nil)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("read server inventory status: expected 200, got %d: %s", response.StatusCode, response.Body)
		}
		var inventory struct {
			Servers []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"servers"`
		}
		decodeResponse(t, response.Body, &inventory)
		for _, item := range inventory.Servers {
			if item.ID == serverID {
				return item.Status
			}
		}
		t.Fatalf("server %s is missing from its owning organization inventory", serverID)
		return ""
	}
	if actual := readInventoryStatus(); actual != "stale" {
		t.Fatalf("a 60-second-old heartbeat must be stale, got %q", actual)
	}
	if _, err := database.ExecContext(ctx, `
		UPDATE agents SET last_seen_at = now() - interval '120 seconds' WHERE id = $1
	`, enrolled.GetAgentId()); err != nil {
		t.Fatalf("age agent heartbeat for offline-state test: %v", err)
	}
	if actual := readInventoryStatus(); actual != "offline" {
		t.Fatalf("a 120-second-old heartbeat must be offline, got %q", actual)
	}
	if _, err := database.ExecContext(ctx, `
		UPDATE agents SET last_seen_at = now() WHERE id = $1
	`, enrolled.GetAgentId()); err != nil {
		t.Fatalf("restore fresh agent heartbeat after liveness test: %v", err)
	}

	identityStore := identity.NewStore(database)
	operationPrincipal := identity.Principal{
		UserID: registered.User.ID, OrganizationID: registered.User.OrganizationID, Role: "owner",
	}
	const operationTarget = "monitor-me-agent.service"
	if _, err := database.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION test_fail_queued_operation_audit()
		RETURNS trigger LANGUAGE plpgsql AS $
		BEGIN
			IF NEW.action = 'queued' THEN
				RAISE EXCEPTION 'injected queued audit failure';
			END IF;
			RETURN NEW;
		END;
		$
	`); err != nil {
		t.Fatalf("create remote operation audit failure fixture: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		CREATE TRIGGER test_fail_queued_operation_audit
		BEFORE INSERT ON remote_operation_audit
		FOR EACH ROW EXECUTE FUNCTION test_fail_queued_operation_audit()
	`); err != nil {
		t.Fatalf("install remote operation audit failure fixture: %v", err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = database.ExecContext(cleanupContext,
			"DROP TRIGGER IF EXISTS test_fail_queued_operation_audit ON remote_operation_audit")
		_, _ = database.ExecContext(cleanupContext,
			"DROP FUNCTION IF EXISTS test_fail_queued_operation_audit()")
	})
	if _, err := identityStore.CreateRemoteOperation(
		ctx, operationPrincipal, serverID, "restart_service", operationTarget, []string{operationTarget},
	); err == nil {
		t.Fatal("remote operation should fail when its required audit insert fails")
	}
	var uncommittedOperations int
	if err := database.QueryRowContext(ctx, `
		SELECT count(*) FROM remote_operations
		WHERE organization_id = $1 AND server_id = $2 AND target = $3
	`, operationPrincipal.OrganizationID, serverID, operationTarget).Scan(&uncommittedOperations); err != nil {
		t.Fatalf("verify remote operation rollback: %v", err)
	}
	if uncommittedOperations != 0 {
		t.Fatalf("remote operation escaped its failed audit transaction: found %d queued rows", uncommittedOperations)
	}
	if _, err := database.ExecContext(ctx,
		"DROP TRIGGER test_fail_queued_operation_audit ON remote_operation_audit"); err != nil {
		t.Fatalf("remove remote operation audit failure fixture: %v", err)
	}
	if _, err := database.ExecContext(ctx,
		"DROP FUNCTION test_fail_queued_operation_audit()"); err != nil {
		t.Fatalf("remove remote operation audit failure function: %v", err)
	}
	queuedOperation, err := identityStore.CreateRemoteOperation(
		ctx, operationPrincipal, serverID, "restart_service", operationTarget, []string{operationTarget},
	)
	if err != nil {
		t.Fatalf("queue audited remote operation: %v", err)
	}
	var queuedAuditCount int
	if err := database.QueryRowContext(ctx, `
		SELECT count(*) FROM remote_operation_audit
		WHERE organization_id = $1 AND remote_operation_id = $2 AND action = 'queued'
	`, operationPrincipal.OrganizationID, queuedOperation.ID).Scan(&queuedAuditCount); err != nil {
		t.Fatalf("verify queued remote operation audit: %v", err)
	}
	if queuedAuditCount != 1 || queuedOperation.Status != "queued" {
		t.Fatalf("remote operation and audit were not committed together: status=%s audit_count=%d",
			queuedOperation.Status, queuedAuditCount)
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
