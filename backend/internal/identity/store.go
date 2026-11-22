package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
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
)

var (
	slugPattern = regexp.MustCompile("^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$")
	uuidPattern = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$")
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
