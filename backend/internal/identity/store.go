package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/WorkRCS/monitor_me/backend/internal/security"
)

var (
	ErrBootstrapComplete  = errors.New("initial account is already configured")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("authentication required")
	ErrForbidden          = errors.New("operation is not permitted")
	ErrNotFound           = errors.New("resource not found")
	ErrInvalidInput       = errors.New("invalid input")
)

const (
	userSessionTTL     = 24 * time.Hour
	enrollmentTokenTTL = 15 * time.Minute
	liveTicketTTL      = 30 * time.Second
)

var (
	slugPattern        = regexp.MustCompile("^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$")
	uuidPattern        = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$")
	alertMetricPattern = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_.-]{0,127}$")
)

// Store contains the durable identity and authorization operations.
type Store struct {
	db *sql.DB
}

// NewStore creates an identity store backed by PostgreSQL.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Principal is the authenticated user's organization-scoped authorization context.
type Principal struct {
	UserID         string `json:"user_id"`
	SessionID      string `json:"-"`
	OrganizationID string `json:"organization_id"`
	Email          string `json:"email"`
	DisplayName    string `json:"display_name"`
	Organization   string `json:"organization_name"`
	Role           string `json:"role"`
}

// RegisterInput is the one-time initial owner registration request.
type RegisterInput struct {
	Email            string
	DisplayName      string
	Password         string
	OrganizationName string
	OrganizationSlug string
}

// User is the safe public subset of an account.
type User struct {
	ID               string `json:"id"`
	Email            string `json:"email"`
	DisplayName      string `json:"display_name"`
	OrganizationID   string `json:"organization_id"`
	OrganizationName string `json:"organization_name"`
	Role             string `json:"role"`
}

// AuthSession contains a new opaque bearer credential and its expiry.
type AuthSession struct {
	AccessToken string
	ExpiresAt   time.Time
	User        User
}

// Server is an organization-scoped server inventory record.
type Server struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Hostname  string     `json:"hostname,omitempty"`
	Status    string     `json:"status"`
	AgentID   *string    `json:"agent_id,omitempty"`
	AgentSeen *time.Time `json:"agent_last_seen_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Project is a project owned by the authenticated organization.
type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}

// ServerProjectMapping assigns one workload selector on a server to a project.
type ServerProjectMapping struct {
	ID                string    `json:"id"`
	ServerID          string    `json:"server_id"`
	ProjectID         string    `json:"project_id"`
	ProjectName       string    `json:"project_name"`
	ProjectSlug       string    `json:"project_slug"`
	WorkloadType      string    `json:"workload_type"`
	WorkloadSelector  string    `json:"workload_selector"`
	AttributionMethod string    `json:"attribution_method"`
	CreatedAt         time.Time `json:"created_at"`
}


type AlertRule struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	ServerID   *string   `json:"server_id,omitempty"`
	ProjectID  *string   `json:"project_id,omitempty"`
	MetricName string    `json:"metric_name"`
	Operator   string    `json:"operator"`
	Threshold  float64   `json:"threshold"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
}

type AlertEvent struct {
	ID           string     `json:"id"`
	RuleID       string     `json:"rule_id"`
	RuleName     string     `json:"rule_name"`
	ServerID     string     `json:"server_id"`
	ServerName   string     `json:"server_name"`
	ProjectID    *string    `json:"project_id,omitempty"`
	MetricName   string     `json:"metric_name"`
	Status       string     `json:"status"`
	LastValue    float64    `json:"last_value"`
	TriggeredAt  time.Time  `json:"triggered_at"`
	LastObserved time.Time  `json:"last_observed_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
}

type AlertNotification struct {
	ID        string     `json:"id"`
	EventID   string     `json:"event_id"`
	Kind      string     `json:"kind"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// IssuedServer returns a server and a one-time enrollment credential.
type IssuedServer struct {
	Server
	EnrollmentTokenID string    `json:"enrollment_token_id"`
	EnrollmentToken   string    `json:"enrollment_token"`
	TokenExpiresAt    time.Time `json:"token_expires_at"`
}

// RegisterFirst creates the first user, organization, and owner membership atomically.
// A transaction-scoped advisory lock prevents concurrent first-user registrations.
func (s *Store) RegisterFirst(ctx context.Context, input RegisterInput) (User, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	displayName := strings.TrimSpace(input.DisplayName)
	organizationName := strings.TrimSpace(input.OrganizationName)
	slug := strings.ToLower(strings.TrimSpace(input.OrganizationSlug))

	if !validEmail(email) || len(displayName) < 1 || len(displayName) > 120 ||
		len(organizationName) < 2 || len(organizationName) > 120 ||
		!slugPattern.MatchString(slug) || len(input.Password) < 12 || len(input.Password) > 128 {
		return User{}, ErrInvalidInput
	}

	passwordHash, err := security.HashPassword(input.Password)
	if err != nil {
		return User{}, fmt.Errorf("hash initial password: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("begin initial registration: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(722041102)`); err != nil {
		return User{}, fmt.Errorf("lock initial registration: %w", err)
	}

	var hasUsers bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&hasUsers); err != nil {
		return User{}, fmt.Errorf("check initial account state: %w", err)
	}
	if hasUsers {
		return User{}, ErrBootstrapComplete
	}

	var organizationID, userID string
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO organizations (name, slug) VALUES ($1, $2) RETURNING id::text`,
		organizationName, slug,
	).Scan(&organizationID); err != nil {
		return User{}, fmt.Errorf("create initial organization: %w", err)
	}

	if err := tx.QueryRowContext(ctx,
		`INSERT INTO users (email, display_name, password_hash) VALUES ($1, $2, $3) RETURNING id::text`,
		email, displayName, passwordHash,
	).Scan(&userID); err != nil {
		return User{}, fmt.Errorf("create initial user: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO organization_memberships (organization_id, user_id, role) VALUES ($1, $2, 'owner')`,
		organizationID, userID,
	); err != nil {
		return User{}, fmt.Errorf("create initial owner membership: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("commit initial registration: %w", err)
	}

	return User{
		ID: userID, Email: email, DisplayName: displayName,
		OrganizationID: organizationID, OrganizationName: organizationName, Role: "owner",
	}, nil
}

// Login verifies a password and creates a 24-hour opaque bearer session.
// Only the SHA-256 digest of the bearer secret is persisted.
func (s *Store) Login(ctx context.Context, email, password string) (AuthSession, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !validEmail(email) || len(password) == 0 || len(password) > 128 {
		return AuthSession{}, ErrInvalidCredentials
	}

	var user User
	var passwordHash string
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id::text, u.email, u.display_name, o.id::text, o.name, m.role, u.password_hash
		FROM users u
		JOIN organization_memberships m ON m.user_id = u.id
		JOIN organizations o ON o.id = m.organization_id
		WHERE lower(u.email) = $1
		ORDER BY m.created_at
		LIMIT 1
	`, email).Scan(&user.ID, &user.Email, &user.DisplayName, &user.OrganizationID, &user.OrganizationName, &user.Role, &passwordHash)
	if err != nil || !security.VerifyPassword(passwordHash, password) {
		return AuthSession{}, ErrInvalidCredentials
	}

	token, err := newOpaqueToken()
	if err != nil {
		return AuthSession{}, fmt.Errorf("create user session: %w", err)
	}
	expiresAt := time.Now().UTC().Add(userSessionTTL)
	hash := sha256.Sum256([]byte(token))

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO user_sessions (user_id, organization_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, user.ID, user.OrganizationID, hash[:], expiresAt); err != nil {
		return AuthSession{}, fmt.Errorf("persist user session: %w", err)
	}

	return AuthSession{AccessToken: token, ExpiresAt: expiresAt, User: user}, nil
}

// Authenticate resolves an opaque bearer token to an active user session.
func (s *Store) Authenticate(ctx context.Context, token string) (Principal, error) {
	if len(token) < 32 || len(token) > 128 {
		return Principal{}, ErrUnauthorized
	}
	hash := sha256.Sum256([]byte(token))

	var principal Principal
	err := s.db.QueryRowContext(ctx, `
		SELECT us.id::text, u.id::text, us.organization_id::text, u.email,
		       u.display_name, o.name, m.role
		FROM user_sessions us
		JOIN users u ON u.id = us.user_id
		JOIN organizations o ON o.id = us.organization_id
		JOIN organization_memberships m
		  ON m.organization_id = us.organization_id AND m.user_id = us.user_id
		WHERE us.token_hash = $1
		  AND us.revoked_at IS NULL
		  AND us.expires_at > now()
	`, hash[:]).Scan(
		&principal.SessionID, &principal.UserID, &principal.OrganizationID,
		&principal.Email, &principal.DisplayName, &principal.Organization, &principal.Role,
	)
	if err != nil {
		return Principal{}, ErrUnauthorized
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE user_sessions SET last_seen_at = now() WHERE id = $1`, principal.SessionID)
	return principal, nil
}

// Logout revokes the current bearer session.
func (s *Store) Logout(ctx context.Context, principal Principal) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE user_sessions
		SET revoked_at = COALESCE(revoked_at, now())
		WHERE id = $1 AND user_id = $2
	`, principal.SessionID, principal.UserID)
	if err != nil {
		return fmt.Errorf("revoke user session: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect session revocation: %w", err)
	}
	if affected == 0 {
		return ErrUnauthorized
	}
	return nil
}

type LiveTicket struct {
	Token     string
	ExpiresAt time.Time
}

// CreateLiveTicket issues a short-lived, one-use ticket bound to the current session.
func (s *Store) CreateLiveTicket(ctx context.Context, principal Principal) (LiveTicket, error) {
	token, err := newOpaqueToken()
	if err != nil {
		return LiveTicket{}, fmt.Errorf("create live connection ticket: %w", err)
	}
	hash := sha256.Sum256([]byte(token))
	expiresAt := time.Now().UTC().Add(liveTicketTTL)
	var ticketID string
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO live_stream_tickets (user_session_id, organization_id, token_hash, expires_at)
		SELECT session.id, session.organization_id, $3, $4
		FROM user_sessions session
		JOIN organization_memberships membership
		  ON membership.organization_id = session.organization_id
		 AND membership.user_id = session.user_id
		WHERE session.id = $1 AND session.user_id = $2 AND session.organization_id = $5
		  AND session.revoked_at IS NULL AND session.expires_at > now()
		RETURNING id::text
	`, principal.SessionID, principal.UserID, hash[:], expiresAt, principal.OrganizationID).Scan(&ticketID)
	if errors.Is(err, sql.ErrNoRows) {
		return LiveTicket{}, ErrUnauthorized
	}
	if err != nil {
		return LiveTicket{}, fmt.Errorf("persist live connection ticket: %w", err)
	}
	return LiveTicket{Token: token, ExpiresAt: expiresAt}, nil
}

// ConsumeLiveTicket atomically spends a ticket and revalidates the parent session.
func (s *Store) ConsumeLiveTicket(ctx context.Context, token string) (Principal, error) {
	if len(token) < 32 || len(token) > 128 {
		return Principal{}, ErrUnauthorized
	}
	hash := sha256.Sum256([]byte(token))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Principal{}, fmt.Errorf("begin live ticket exchange: %w", err)
	}
	defer tx.Rollback()

	var principal Principal
	err = tx.QueryRowContext(ctx, `
		UPDATE live_stream_tickets ticket
		SET consumed_at = now()
		FROM user_sessions session
		JOIN users user_account ON user_account.id = session.user_id
		JOIN organizations organization ON organization.id = session.organization_id
		JOIN organization_memberships membership
		  ON membership.organization_id = session.organization_id
		 AND membership.user_id = session.user_id
		WHERE ticket.token_hash = $1
		  AND ticket.user_session_id = session.id
		  AND ticket.organization_id = session.organization_id
		  AND ticket.consumed_at IS NULL
		  AND ticket.expires_at > now()
		  AND session.revoked_at IS NULL
		  AND session.expires_at > now()
		RETURNING session.id::text, session.user_id::text,
		          session.organization_id::text, user_account.email,
		          user_account.display_name, organization.name, membership.role
	`, hash[:]).Scan(
		&principal.SessionID, &principal.UserID, &principal.OrganizationID,
		&principal.Email, &principal.DisplayName, &principal.Organization, &principal.Role,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, ErrUnauthorized
	}
	if err != nil {
		return Principal{}, fmt.Errorf("consume live connection ticket: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Principal{}, fmt.Errorf("commit live ticket exchange: %w", err)
	}
	return principal, nil
}

// CreateServer creates a pending server and its first scoped enrollment token atomically.
func (s *Store) CreateServer(ctx context.Context, principal Principal, name, hostname string) (IssuedServer, error) {
	name = strings.TrimSpace(name)
	hostname = strings.TrimSpace(hostname)
	if len(name) < 2 || len(name) > 120 || len(hostname) > 253 {
		return IssuedServer{}, ErrInvalidInput
	}
	if !canManageServers(principal.Role) {
		return IssuedServer{}, ErrForbidden
	}

	token, err := newOpaqueToken()
	if err != nil {
		return IssuedServer{}, fmt.Errorf("create enrollment credential: %w", err)
	}
	tokenHash := sha256.Sum256([]byte(token))
	expiresAt := time.Now().UTC().Add(enrollmentTokenTTL)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return IssuedServer{}, fmt.Errorf("begin server creation: %w", err)
	}
	defer tx.Rollback()

	var serverID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO servers (organization_id, name, hostname, status)
		VALUES ($1, $2, NULLIF($3, ''), 'pending')
		RETURNING id::text
	`, principal.OrganizationID, name, hostname).Scan(&serverID)
	if err != nil {
		return IssuedServer{}, fmt.Errorf("create server inventory record: %w", err)
	}

	var tokenID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO agent_enrollment_tokens (organization_id, server_id, token_hash, created_by, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text
	`, principal.OrganizationID, serverID, tokenHash[:], principal.UserID, expiresAt).Scan(&tokenID)
	if err != nil {
		return IssuedServer{}, fmt.Errorf("create scoped enrollment token: %w", err)
	}

	var createdAt time.Time
	err = tx.QueryRowContext(ctx, `SELECT created_at FROM servers WHERE id = $1`, serverID).Scan(&createdAt)
	if err != nil {
		return IssuedServer{}, fmt.Errorf("read created server: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return IssuedServer{}, fmt.Errorf("commit server creation: %w", err)
	}

	return IssuedServer{
		Server:            Server{ID: serverID, Name: name, Hostname: hostname, Status: "pending", CreatedAt: createdAt},
		EnrollmentTokenID: tokenID, EnrollmentToken: token, TokenExpiresAt: expiresAt,
	}, nil
}

// ListServers returns only the current user's organization inventory.
func (s *Store) ListServers(ctx context.Context, principal Principal) ([]Server, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id::text, s.name, COALESCE(s.hostname, ''), s.status,
		       a.id::text, a.last_seen_at, s.created_at
		FROM servers s
		LEFT JOIN agents a ON a.server_id = s.id
		WHERE s.organization_id = $1
		ORDER BY s.created_at DESC
	`, principal.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("list organization servers: %w", err)
	}
	defer rows.Close()

	servers := make([]Server, 0)
	for rows.Next() {
		var server Server
		var agentID sql.NullString
		var lastSeen sql.NullTime
		if err := rows.Scan(&server.ID, &server.Name, &server.Hostname, &server.Status, &agentID, &lastSeen, &server.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan server inventory: %w", err)
		}
		if agentID.Valid {
			server.AgentID = &agentID.String
		}
		if lastSeen.Valid {
			server.AgentSeen = &lastSeen.Time
		}
		servers = append(servers, server)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate server inventory: %w", err)
	}
	return servers, nil
}

// ReissueEnrollmentToken creates a fresh single-use credential for an existing server.
// Only operators and administrators in the owning organization may rotate installation credentials.
func (s *Store) ReissueEnrollmentToken(ctx context.Context, principal Principal, serverID string) (IssuedServer, error) {
	if !canManageServers(principal.Role) {
		return IssuedServer{}, ErrForbidden
	}
	if !validUUID(serverID) {
		return IssuedServer{}, ErrNotFound
	}
	token, err := newOpaqueToken()
	if err != nil {
		return IssuedServer{}, fmt.Errorf("create enrollment credential: %w", err)
	}
	tokenHash := sha256.Sum256([]byte(token))
	expiresAt := time.Now().UTC().Add(enrollmentTokenTTL)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return IssuedServer{}, fmt.Errorf("begin enrollment token rotation: %w", err)
	}
	defer tx.Rollback()

	var server Server
	err = tx.QueryRowContext(ctx, `
		UPDATE servers
		SET status = CASE WHEN status = 'disabled' THEN 'pending' ELSE status END,
		    updated_at = now()
		WHERE id = $1 AND organization_id = $2
		RETURNING id::text, name, COALESCE(hostname, ''), status, created_at
	`, serverID, principal.OrganizationID).Scan(
		&server.ID, &server.Name, &server.Hostname, &server.Status, &server.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return IssuedServer{}, ErrNotFound
	}
	if err != nil {
		return IssuedServer{}, fmt.Errorf("load organization server for token rotation: %w", err)
	}

	// A rotation replaces all unused credentials so only the newly returned
	// secret can enroll this server after the transaction commits.
	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_enrollment_tokens
		SET revoked_at = COALESCE(revoked_at, now())
		WHERE organization_id = $1 AND server_id = $2
		  AND consumed_at IS NULL AND revoked_at IS NULL
	`, principal.OrganizationID, serverID); err != nil {
		return IssuedServer{}, fmt.Errorf("revoke previous unused enrollment tokens: %w", err)
	}

	var tokenID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO agent_enrollment_tokens (organization_id, server_id, token_hash, created_by, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text
	`, principal.OrganizationID, serverID, tokenHash[:], principal.UserID, expiresAt).Scan(&tokenID)
	if err != nil {
		return IssuedServer{}, fmt.Errorf("persist replacement enrollment token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return IssuedServer{}, fmt.Errorf("commit enrollment token rotation: %w", err)
	}

	return IssuedServer{
		Server: server, EnrollmentTokenID: tokenID,
		EnrollmentToken: token, TokenExpiresAt: expiresAt,
	}, nil
}

// RevokeEnrollmentToken revokes an unused token within the caller's organization.
func (s *Store) RevokeEnrollmentToken(ctx context.Context, principal Principal, tokenID string) error {
	if !canAdminister(principal.Role) {
		return ErrForbidden
	}
	if !validUUID(tokenID) {
		return ErrNotFound
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE agent_enrollment_tokens
		SET revoked_at = COALESCE(revoked_at, now())
		WHERE id = $1 AND organization_id = $2
		  AND consumed_at IS NULL AND revoked_at IS NULL
	`, tokenID, principal.OrganizationID)
	if err != nil {
		return fmt.Errorf("revoke enrollment token: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect enrollment token revocation: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeAgent revokes the agent and every outstanding session in the caller's organization.
func (s *Store) RevokeAgent(ctx context.Context, principal Principal, agentID string) error {
	if !canAdminister(principal.Role) {
		return ErrForbidden
	}
	if !validUUID(agentID) {
		return ErrNotFound
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin agent revocation: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE agents a
		SET status = 'revoked', updated_at = now()
		FROM servers s
		WHERE a.server_id = s.id
		  AND a.id = $1 AND s.organization_id = $2
	`, agentID, principal.OrganizationID)
	if err != nil {
		return fmt.Errorf("revoke agent identity: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect agent revocation: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_sessions SET revoked_at = COALESCE(revoked_at, now())
		WHERE agent_id = $1 AND revoked_at IS NULL
	`, agentID); err != nil {
		return fmt.Errorf("revoke agent sessions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE servers SET status = 'disabled', updated_at = now()
		WHERE id = (SELECT server_id FROM agents WHERE id = $1)
	`, agentID); err != nil {
		return fmt.Errorf("disable revoked server: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit agent revocation: %w", err)
	}
	return nil
}

// ListProjects returns only projects belonging to the authenticated organization.
func (s *Store) ListProjects(ctx context.Context, principal Principal) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id::text, name, slug, created_at
		FROM projects
		WHERE organization_id = $1
		ORDER BY lower(name), id
	`, principal.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("list organization projects: %w", err)
	}
	defer rows.Close()
	projects := make([]Project, 0)
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.ID, &project.Name, &project.Slug, &project.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan organization project: %w", err)
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate organization projects: %w", err)
	}
	return projects, nil
}

// CreateProject restricts project administration to organization owners/admins.
func (s *Store) CreateProject(ctx context.Context, principal Principal, name, slug string) (Project, error) {
	if !canAdminister(principal.Role) {
		return Project{}, ErrForbidden
	}
	name = strings.TrimSpace(name)
	slug = strings.ToLower(strings.TrimSpace(slug))
	if len(name) < 2 || len(name) > 120 || !slugPattern.MatchString(slug) {
		return Project{}, ErrInvalidInput
	}
	var project Project
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO projects (organization_id, name, slug)
		VALUES ($1, $2, $3)
		ON CONFLICT (organization_id, slug) DO NOTHING
		RETURNING id::text, name, slug, created_at
	`, principal.OrganizationID, name, slug).Scan(
		&project.ID, &project.Name, &project.Slug, &project.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrInvalidInput
	}
	if err != nil {
		return Project{}, fmt.Errorf("create organization project: %w", err)
	}
	return project, nil
}

// DeleteProject removes a project in the caller's organization. Telemetry samples
// retain their history; the schema clears only their project attribution.
func (s *Store) DeleteProject(ctx context.Context, principal Principal, projectID string) error {
	if !canAdminister(principal.Role) {
		return ErrForbidden
	}
	if !validUUID(projectID) {
		return ErrNotFound
	}
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM projects WHERE id = $1 AND organization_id = $2
	`, projectID, principal.OrganizationID)
	if err != nil {
		return fmt.Errorf("delete organization project: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect project deletion: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// ListServerProjectMappings returns workload mappings for one organization-owned server.
func (s *Store) ListServerProjectMappings(ctx context.Context, principal Principal, serverID string) ([]ServerProjectMapping, error) {
	if !validUUID(serverID) {
		return nil, ErrNotFound
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM servers WHERE id = $1 AND organization_id = $2)
	`, serverID, principal.OrganizationID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("verify server mapping scope: %w", err)
	}
	if !exists {
		return nil, ErrNotFound
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT mapping.id::text, mapping.server_id::text, mapping.project_id::text,
		       project.name, project.slug, mapping.workload_type,
		       mapping.workload_selector, mapping.attribution_method, mapping.created_at
		FROM server_project_mappings mapping
		JOIN projects project
		  ON project.id = mapping.project_id
		 AND project.organization_id = mapping.organization_id
		WHERE mapping.organization_id = $1 AND mapping.server_id = $2
		ORDER BY mapping.workload_type, mapping.workload_selector
	`, principal.OrganizationID, serverID)
	if err != nil {
		return nil, fmt.Errorf("list server workload mappings: %w", err)
	}
	defer rows.Close()
	mappings := make([]ServerProjectMapping, 0)
	for rows.Next() {
		var mapping ServerProjectMapping
		if err := rows.Scan(
			&mapping.ID, &mapping.ServerID, &mapping.ProjectID, &mapping.ProjectName,
			&mapping.ProjectSlug, &mapping.WorkloadType, &mapping.WorkloadSelector,
			&mapping.AttributionMethod, &mapping.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan server workload mapping: %w", err)
		}
		mappings = append(mappings, mapping)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate server workload mappings: %w", err)
	}
	return mappings, nil
}

// CreateServerProjectMapping assigns an existing cgroup/container/Systemd selector.
// The compound organization foreign keys prevent cross-tenant mappings.
func (s *Store) CreateServerProjectMapping(
	ctx context.Context,
	principal Principal,
	serverID, projectID, workloadType, selector, attributionMethod string,
) (ServerProjectMapping, error) {
	if !canManageServers(principal.Role) {
		return ServerProjectMapping{}, ErrForbidden
	}
	workloadType = strings.TrimSpace(workloadType)
	selector = strings.TrimSpace(selector)
	attributionMethod = strings.TrimSpace(attributionMethod)
	if !validUUID(serverID) || !validUUID(projectID) ||
		(len(workloadType) == 0 || len(workloadType) > 32) ||
		(workloadType != "cgroup" && workloadType != "container" && workloadType != "systemd") ||
		len(selector) == 0 || len(selector) > 512 ||
		strings.ContainsAny(selector, "\r\n\x00") ||
		(attributionMethod != "cgroup" && attributionMethod != "container" &&
			attributionMethod != "systemd" && attributionMethod != "explicit" &&
			attributionMethod != "process_tree") {
		return ServerProjectMapping{}, ErrInvalidInput
	}
	var allowed bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			EXISTS (SELECT 1 FROM servers WHERE id = $1 AND organization_id = $3)
			AND EXISTS (SELECT 1 FROM projects WHERE id = $2 AND organization_id = $3)
	`, serverID, projectID, principal.OrganizationID).Scan(&allowed); err != nil {
		return ServerProjectMapping{}, fmt.Errorf("verify workload mapping scope: %w", err)
	}
	if !allowed {
		return ServerProjectMapping{}, ErrNotFound
	}
	var mapping ServerProjectMapping
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO server_project_mappings (
			organization_id, server_id, project_id, workload_type,
			workload_selector, attribution_method
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (server_id, workload_type, workload_selector) DO NOTHING
		RETURNING id::text, server_id::text, project_id::text, workload_type,
		          workload_selector, attribution_method, created_at
	`, principal.OrganizationID, serverID, projectID, workloadType, selector, attributionMethod).Scan(
		&mapping.ID, &mapping.ServerID, &mapping.ProjectID, &mapping.WorkloadType,
		&mapping.WorkloadSelector, &mapping.AttributionMethod, &mapping.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ServerProjectMapping{}, ErrInvalidInput
	}
	if err != nil {
		return ServerProjectMapping{}, fmt.Errorf("create server workload mapping: %w", err)
	}
	mapping.ProjectName, mapping.ProjectSlug = "", ""
	if err := s.db.QueryRowContext(ctx, `
		SELECT name, slug FROM projects WHERE id = $1 AND organization_id = $2
	`, projectID, principal.OrganizationID).Scan(&mapping.ProjectName, &mapping.ProjectSlug); err != nil {
		return ServerProjectMapping{}, fmt.Errorf("load mapped project details: %w", err)
	}
	return mapping, nil
}

// DeleteServerProjectMapping removes a mapping only within the caller's organization.
func (s *Store) DeleteServerProjectMapping(ctx context.Context, principal Principal, mappingID string) error {
	if !canManageServers(principal.Role) {
		return ErrForbidden
	}
	if !validUUID(mappingID) {
		return ErrNotFound
	}
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM server_project_mappings
		WHERE id = $1 AND organization_id = $2
	`, mappingID, principal.OrganizationID)
	if err != nil {
		return fmt.Errorf("delete server workload mapping: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect workload mapping deletion: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func canManageServers(role string) bool {
	return role == "owner" || role == "admin" || role == "operator"
}

func canAdminister(role string) bool {
	return role == "owner" || role == "admin"
}

func validEmail(email string) bool {
	if len(email) < 5 || len(email) > 254 {
		return false
	}
	parsed, err := mail.ParseAddress(email)
	return err == nil && strings.EqualFold(parsed.Address, email) && !strings.ContainsAny(email, " <>")
}

func validUUID(value string) bool {
	return uuidPattern.MatchString(value)
}

func newOpaqueToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}


func (s *Store) ListAlertRules(ctx context.Context, principal Principal) ([]AlertRule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id::text, name, server_id::text, project_id::text, metric_name,
		       operator, threshold, enabled, created_at
		FROM alert_rules
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT 200
	`, principal.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("list alert rules: %w", err)
	}
	defer rows.Close()

	items := make([]AlertRule, 0)
	for rows.Next() {
		var item AlertRule
		var serverID, projectID sql.NullString
		if err := rows.Scan(&item.ID, &item.Name, &serverID, &projectID, &item.MetricName,
			&item.Operator, &item.Threshold, &item.Enabled, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan alert rule: %w", err)
		}
		if serverID.Valid {
			item.ServerID = &serverID.String
		}
		if projectID.Valid {
			item.ProjectID = &projectID.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert rules: %w", err)
	}
	return items, nil
}

func (s *Store) CreateAlertRule(ctx context.Context, principal Principal, input AlertRule) (AlertRule, error) {
	if !canAdminister(principal.Role) {
		return AlertRule{}, ErrForbidden
	}
	input.Name = strings.TrimSpace(input.Name)
	if len(input.Name) < 2 || len(input.Name) > 120 || !alertMetricPattern.MatchString(input.MetricName) ||
		(input.Operator != "gt" && input.Operator != "gte" && input.Operator != "lt" && input.Operator != "lte") ||
		math.IsNaN(input.Threshold) || math.IsInf(input.Threshold, 0) {
		return AlertRule{}, ErrInvalidInput
	}
	if input.ServerID != nil && !validUUID(*input.ServerID) {
		return AlertRule{}, ErrInvalidInput
	}
	if input.ProjectID != nil && !validUUID(*input.ProjectID) {
		return AlertRule{}, ErrInvalidInput
	}
	if input.ServerID != nil || input.ProjectID != nil {
		var allowed bool
		err := s.db.QueryRowContext(ctx, `
			SELECT
				($2::uuid IS NULL OR EXISTS (SELECT 1 FROM servers WHERE id = $2::uuid AND organization_id = $1))
				AND ($3::uuid IS NULL OR EXISTS (SELECT 1 FROM projects WHERE id = $3::uuid AND organization_id = $1))
		`, principal.OrganizationID, nullableUUID(input.ServerID), nullableUUID(input.ProjectID)).Scan(&allowed)
		if err != nil {
			return AlertRule{}, fmt.Errorf("validate alert rule scope: %w", err)
		}
		if !allowed {
			return AlertRule{}, ErrNotFound
		}
	}
	var serverID, projectID sql.NullString
	var result AlertRule
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO alert_rules (organization_id, name, server_id, project_id, metric_name, operator, threshold, enabled, created_by)
		VALUES ($1, $2, $3::uuid, $4::uuid, $5, $6, $7, true, $8)
		RETURNING id::text, name, server_id::text, project_id::text, metric_name, operator, threshold, enabled, created_at
	`, principal.OrganizationID, input.Name, nullableUUID(input.ServerID), nullableUUID(input.ProjectID),
		input.MetricName, input.Operator, input.Threshold, principal.UserID).Scan(
		&result.ID, &result.Name, &serverID, &projectID, &result.MetricName,
		&result.Operator, &result.Threshold, &result.Enabled, &result.CreatedAt)
	if err != nil {
		return AlertRule{}, fmt.Errorf("create alert rule: %w", err)
	}
	if serverID.Valid {
		result.ServerID = &serverID.String
	}
	if projectID.Valid {
		result.ProjectID = &projectID.String
	}
	return result, nil
}

func nullableUUID(value *string) any {
	if value == nil || *value == "" {
		return nil
	}
	return *value
}

func (s *Store) DeleteAlertRule(ctx context.Context, principal Principal, ruleID string) error {
	if !canAdminister(principal.Role) {
		return ErrForbidden
	}
	if !validUUID(ruleID) {
		return ErrNotFound
	}
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM alert_rules WHERE id = $1 AND organization_id = $2`, ruleID, principal.OrganizationID)
	if err != nil {
		return fmt.Errorf("delete alert rule: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect alert rule deletion: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListAlertEvents(ctx context.Context, principal Principal) ([]AlertEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT event.id::text, event.rule_id::text, rule.name, event.server_id::text,
		       server.name, event.project_id::text, event.metric_name, event.status,
		       event.last_value, event.triggered_at, event.last_observed_at, event.resolved_at
		FROM alert_events event
		JOIN alert_rules rule ON rule.id = event.rule_id AND rule.organization_id = event.organization_id
		JOIN servers server ON server.id = event.server_id AND server.organization_id = event.organization_id
		WHERE event.organization_id = $1
		ORDER BY event.triggered_at DESC
		LIMIT 200
	`, principal.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("list alert events: %w", err)
	}
	defer rows.Close()

	items := make([]AlertEvent, 0)
	for rows.Next() {
		var item AlertEvent
		var projectID sql.NullString
		var resolvedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.RuleID, &item.RuleName, &item.ServerID, &item.ServerName,
			&projectID, &item.MetricName, &item.Status, &item.LastValue, &item.TriggeredAt,
			&item.LastObserved, &resolvedAt); err != nil {
			return nil, fmt.Errorf("scan alert event: %w", err)
		}
		if projectID.Valid {
			item.ProjectID = &projectID.String
		}
		if resolvedAt.Valid {
			item.ResolvedAt = &resolvedAt.Time
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert events: %w", err)
	}
	return items, nil
}

func (s *Store) ListAlertNotifications(ctx context.Context, principal Principal) ([]AlertNotification, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id::text, event_id::text, kind, title, message, read_at, created_at
		FROM alert_notifications
		WHERE organization_id = $1 AND user_id = $2
		ORDER BY read_at NULLS FIRST, created_at DESC
		LIMIT 100
	`, principal.OrganizationID, principal.UserID)
	if err != nil {
		return nil, fmt.Errorf("list alert notifications: %w", err)
	}
	defer rows.Close()

	items := make([]AlertNotification, 0)
	for rows.Next() {
		var item AlertNotification
		var readAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.EventID, &item.Kind, &item.Title, &item.Message, &readAt, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan alert notification: %w", err)
		}
		if readAt.Valid {
			item.ReadAt = &readAt.Time
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert notifications: %w", err)
	}
	return items, nil
}

func (s *Store) MarkAlertNotificationRead(ctx context.Context, principal Principal, notificationID string) error {
	if !validUUID(notificationID) {
		return ErrNotFound
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE alert_notifications SET read_at = COALESCE(read_at, now())
		WHERE id = $1 AND organization_id = $2 AND user_id = $3
	`, notificationID, principal.OrganizationID, principal.UserID)
	if err != nil {
		return fmt.Errorf("mark alert notification read: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect notification update: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}
