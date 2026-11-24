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
	"regexp"
	"strings"
	"time"

	"monitor-me.internal/backend/internal/security"
)

var (
	ErrUnauthorized       = errors.New("installation session is invalid")
	ErrForbidden          = errors.New("operation is not permitted")
	ErrNotFound           = errors.New("resource not found")
	ErrInvalidInput       = errors.New("invalid input")
)

const (
	installationSessionTTL = 24 * time.Hour
	enrollmentTokenTTL = 15 * time.Minute
	liveTicketTTL      = 30 * time.Second
)

var (
	uuidPattern        = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$")
	slugPattern        = regexp.MustCompile("^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$")
	alertMetricPattern = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_.-]{0,127}$")
	dashboardLabelKeyPattern = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_.-]{0,63}$")
)

// Store contains the durable identity and authorization operations.
type Store struct {
	db *sql.DB
}

// NewStore creates an identity store backed by PostgreSQL.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Principal is the private installation scope retained for legacy database relationships.
type Principal struct {
	UserID         string `json:"user_id"`
	SessionID      string `json:"-"`
	OrganizationID string `json:"organization_id"`
	Email          string `json:"email"`
	DisplayName    string `json:"display_name"`
	Organization   string `json:"organization_name"`
	Role           string `json:"role"`
}

// Server is a monitored host within the private installation scope.
type Server struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Hostname  string     `json:"hostname,omitempty"`
	Status    string     `json:"status"`
	AgentID   *string    `json:"agent_id,omitempty"`
	AgentSeen *time.Time `json:"agent_last_seen_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type Dashboard struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Description      string           `json:"description"`
	IsDefault        bool             `json:"is_default"`
	DefaultServerID  *string          `json:"default_server_id,omitempty"`
	DefaultTimeRange string           `json:"default_time_range"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	Panels           []DashboardPanel `json:"panels"`
}

type DashboardInput struct {
	Name             string
	Description      string
	IsDefault        bool
	DefaultServerID  *string
	DefaultTimeRange string
}

type DashboardPanel struct {
	ID             string    `json:"id"`
	DashboardID    string    `json:"dashboard_id"`
	Title          string    `json:"title"`
	Visualization  string    `json:"visualization"`
	ServerID       string    `json:"server_id"`
	MetricName     string    `json:"metric_name"`
	TimeRange      string    `json:"time_range"`
	Position       int       `json:"position"`
	GridWidth      int       `json:"grid_width"`
	LabelKey       string    `json:"label_key"`
	LabelValue     string    `json:"label_value"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type DashboardPanelInput struct {
	Title          string
	Visualization  string
	ServerID       string
	MetricName     string
	TimeRange      string
	Position       int
	GridWidth      int
	LabelKey       string
	LabelValue     string
}


type AlertRule struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	ServerID   *string   `json:"server_id,omitempty"`
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

// createInstallationIdentity provisions private compatibility rows for the
// legacy relational schema. It does not create an interactive account: the
// random password is discarded and the application exposes no login endpoint.
func (s *Store) createInstallationIdentity(ctx context.Context) error {
	randomPassword, err := newOpaqueToken()
	if err != nil {
		return fmt.Errorf("generate installation-only secret: %w", err)
	}
	passwordHash, err := security.HashPassword(randomPassword)
	if err != nil {
		return fmt.Errorf("hash installation-only secret: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin installation identity transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(722041102)`); err != nil {
		return fmt.Errorf("lock installation identity creation: %w", err)
	}

	var hasUsers bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&hasUsers); err != nil {
		return fmt.Errorf("check legacy identity state: %w", err)
	}
	if hasUsers {
		return nil
	}

	var organizationID, userID string
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Monitor Me Installation', 'monitor-me-installation') RETURNING id::text`,
	).Scan(&organizationID); err != nil {
		return fmt.Errorf("create private installation scope: %w", err)
	}

	if err := tx.QueryRowContext(ctx,
		`INSERT INTO users (email, display_name, password_hash) VALUES ('local@monitor.me', 'Monitor Me Installation', $1) RETURNING id::text`,
		passwordHash,
	).Scan(&userID); err != nil {
		return fmt.Errorf("create private installation identity: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO organization_memberships (organization_id, user_id, role) VALUES ($1, $2, 'owner')`,
		organizationID, userID,
	); err != nil {
		return fmt.Errorf("link private installation scope: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit installation identity transaction: %w", err)
	}
	return nil
}

// CreateInstallationPrincipal resolves the private compatibility identity and creates an internal
// short-lived session used only to bind live-stream tickets. No browser credential is created.
func (s *Store) CreateInstallationPrincipal(ctx context.Context) (Principal, time.Time, error) {
	// A legacy database may contain data for several previously supported
	// organizations. Never pick one implicitly: that could hide or expose the
	// wrong tenant's monitoring data after switching to single-install mode.
	var organizationCount int
	if err := s.db.QueryRowContext(ctx, `
		SELECT count(DISTINCT organization_id) FROM organization_memberships
	`).Scan(&organizationCount); err != nil {
		return Principal{}, time.Time{}, fmt.Errorf("inspect legacy installation scopes: %w", err)
	}
	if organizationCount > 1 {
		return Principal{}, time.Time{}, fmt.Errorf(
			"single-install mode found %d legacy organizations; migrate or isolate the database before startup",
			organizationCount,
		)
	}

	loadPrincipal := func() (Principal, error) {
		var principal Principal
		err := s.db.QueryRowContext(ctx, `
			SELECT u.id::text, o.id::text, u.email, u.display_name, o.name, m.role
			FROM users u
			JOIN organization_memberships m ON m.user_id = u.id
			JOIN organizations o ON o.id = m.organization_id
			ORDER BY m.created_at
			LIMIT 1
		`).Scan(
			&principal.UserID, &principal.OrganizationID, &principal.Email,
			&principal.DisplayName, &principal.Organization, &principal.Role,
		)
		return principal, err
	}

	principal, err := loadPrincipal()
	if errors.Is(err, sql.ErrNoRows) {
		if createErr := s.createInstallationIdentity(ctx); createErr != nil {
			return Principal{}, time.Time{}, fmt.Errorf("bootstrap private installation scope: %w", createErr)
		}
		principal, err = loadPrincipal()
	}
	if err != nil {
		return Principal{}, time.Time{}, fmt.Errorf("load local workspace owner: %w", err)
	}

	internalToken, err := newOpaqueToken()
	if err != nil {
		return Principal{}, time.Time{}, fmt.Errorf("create local live session secret: %w", err)
	}
	hash := sha256.Sum256([]byte(internalToken))
	expiresAt := time.Now().UTC().Add(installationSessionTTL)
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO user_sessions (user_id, organization_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text
	`, principal.UserID, principal.OrganizationID, hash[:], expiresAt).Scan(&principal.SessionID)
	if err != nil {
		return Principal{}, time.Time{}, fmt.Errorf("persist local live session: %w", err)
	}
	return principal, expiresAt, nil
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

// CreateServer creates a pending host and its first one-time enrollment credential atomically.
func (s *Store) CreateServer(ctx context.Context, principal Principal, name, hostname string) (IssuedServer, error) {
	name = strings.TrimSpace(name)
	hostname = strings.TrimSpace(hostname)
	if len(name) < 2 || len(name) > 120 || len(hostname) > 253 {
		return IssuedServer{}, ErrInvalidInput
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
		SELECT s.id::text, s.name, COALESCE(s.hostname, ''),
		       CASE
		           WHEN s.status = 'disabled' THEN 'disabled'
		           WHEN a.status = 'pending' THEN 'pending'
		           WHEN a.status IN ('revoked', 'offline') THEN 'offline'
		           WHEN a.status = 'stale' THEN 'stale'
		           WHEN a.last_seen_at IS NULL THEN s.status
		           WHEN a.last_seen_at <= now() - interval '90 seconds' THEN 'offline'
		           WHEN a.last_seen_at <= now() - interval '45 seconds' THEN 'stale'
		           ELSE 'online'
		       END AS effective_status,
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

// ListDashboards returns organization-owned dashboards and their configured panels.
func (s *Store) ListDashboards(ctx context.Context, principal Principal) ([]Dashboard, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id::text, d.name, d.description, d.is_default,
		       d.default_server_id::text, d.default_time_range,
		       d.created_at, d.updated_at,
		       panel.id::text, panel.title, panel.visualization, panel.server_id::text,
		       panel.metric_name, panel.time_range, panel.position, panel.grid_width,
		       panel.label_key, panel.label_value, panel.created_at, panel.updated_at
		FROM dashboards d
		LEFT JOIN dashboard_panels panel
		  ON panel.dashboard_id = d.id
		 AND panel.organization_id = d.organization_id
		WHERE d.organization_id = $1
		ORDER BY d.is_default DESC, lower(d.name), d.id, panel.position, panel.id
	`, principal.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("list installation dashboards: %w", err)
	}
	defer rows.Close()

	dashboards := make([]Dashboard, 0)
	indexByID := make(map[string]int)
	for rows.Next() {
		var dashboard Dashboard
		var defaultServerID sql.NullString
		var defaultTimeRange string
		var panelID, panelTitle, visualization, serverID, metricName, timeRange sql.NullString
		var labelKey, labelValue sql.NullString
		var position, gridWidth sql.NullInt64
		var panelCreatedAt, panelUpdatedAt sql.NullTime
		if err := rows.Scan(
			&dashboard.ID, &dashboard.Name, &dashboard.Description, &dashboard.IsDefault,
			&defaultServerID, &defaultTimeRange, &dashboard.CreatedAt, &dashboard.UpdatedAt,
			&panelID, &panelTitle, &visualization, &serverID,
			&metricName, &timeRange, &position, &gridWidth, &labelKey, &labelValue, &panelCreatedAt, &panelUpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan installation dashboard: %w", err)
		}
		if index, exists := indexByID[dashboard.ID]; exists {
			dashboard = dashboards[index]
		} else {
			if defaultServerID.Valid {
				value := defaultServerID.String
				dashboard.DefaultServerID = &value
			}
			dashboard.DefaultTimeRange = defaultTimeRange
			dashboard.Panels = make([]DashboardPanel, 0)
			indexByID[dashboard.ID] = len(dashboards)
			dashboards = append(dashboards, dashboard)
		}
		if panelID.Valid {
			panel := DashboardPanel{
				ID: panelID.String, DashboardID: dashboard.ID, Title: panelTitle.String,
				Visualization: visualization.String, ServerID: serverID.String,
				MetricName: metricName.String, TimeRange: timeRange.String,
				Position: int(position.Int64), GridWidth: int(gridWidth.Int64),
				LabelKey: labelKey.String, LabelValue: labelValue.String,
				CreatedAt: panelCreatedAt.Time, UpdatedAt: panelUpdatedAt.Time,
			}
			dashboards[indexByID[dashboard.ID]].Panels = append(dashboards[indexByID[dashboard.ID]].Panels, panel)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate installation dashboards: %w", err)
	}
	return dashboards, nil
}

// CreateDashboard persists a dashboard and its saved filter defaults.
func (s *Store) CreateDashboard(ctx context.Context, principal Principal, input DashboardInput) (Dashboard, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if input.DefaultTimeRange == "" {
		input.DefaultTimeRange = "panel"
	}
	if len(input.Name) < 2 || len(input.Name) > 120 || len(input.Description) > 500 ||
		!validDashboardTimeRange(input.DefaultTimeRange) {
		return Dashboard{}, ErrInvalidInput
	}
	if input.DefaultServerID != nil && !validUUID(*input.DefaultServerID) {
		return Dashboard{}, ErrInvalidInput
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Dashboard{}, fmt.Errorf("begin dashboard creation: %w", err)
	}
	defer tx.Rollback()

	if input.DefaultServerID != nil {
		var exists bool
		if err := tx.QueryRowContext(ctx,
			"SELECT EXISTS (SELECT 1 FROM servers WHERE id = $1 AND organization_id = $2)",
			*input.DefaultServerID, principal.OrganizationID).Scan(&exists); err != nil {
			return Dashboard{}, fmt.Errorf("validate default dashboard machine: %w", err)
		}
		if !exists {
			return Dashboard{}, ErrNotFound
		}
	}
	if input.IsDefault {
		if _, err := tx.ExecContext(ctx,
			"UPDATE dashboards SET is_default = false, updated_at = now() WHERE organization_id = $1 AND is_default = true",
			principal.OrganizationID); err != nil {
			return Dashboard{}, fmt.Errorf("clear previous default dashboard: %w", err)
		}
	}

	var dashboard Dashboard
	var defaultServerID sql.NullString
	err = tx.QueryRowContext(ctx, `
		INSERT INTO dashboards (
			organization_id, name, description, is_default, default_server_id,
			default_time_range, created_by, updated_by
		)
		VALUES ($1, $2, $3, $4, $5::uuid, $6, $7, $7)
		RETURNING id::text, name, description, is_default, default_server_id::text,
		          default_time_range, created_at, updated_at
	`, principal.OrganizationID, input.Name, input.Description, input.IsDefault,
		nullableUUID(input.DefaultServerID), input.DefaultTimeRange, principal.UserID).Scan(
		&dashboard.ID, &dashboard.Name, &dashboard.Description, &dashboard.IsDefault,
		&defaultServerID, &dashboard.DefaultTimeRange, &dashboard.CreatedAt, &dashboard.UpdatedAt,
	)
	if err != nil {
		return Dashboard{}, fmt.Errorf("create installation dashboard: %w", err)
	}
	if defaultServerID.Valid {
		value := defaultServerID.String
		dashboard.DefaultServerID = &value
	}
	dashboard.Panels = make([]DashboardPanel, 0)
	if err := tx.Commit(); err != nil {
		return Dashboard{}, fmt.Errorf("commit installation dashboard: %w", err)
	}
	return dashboard, nil
}

// UpdateDashboard edits dashboard details, default status and saved machine/time filters.
func (s *Store) UpdateDashboard(ctx context.Context, principal Principal, dashboardID string, input DashboardInput) (Dashboard, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if !validUUID(dashboardID) {
		return Dashboard{}, ErrNotFound
	}
	if input.DefaultTimeRange == "" {
		input.DefaultTimeRange = "panel"
	}
	if len(input.Name) < 2 || len(input.Name) > 120 || len(input.Description) > 500 ||
		!validDashboardTimeRange(input.DefaultTimeRange) {
		return Dashboard{}, ErrInvalidInput
	}
	if input.DefaultServerID != nil && !validUUID(*input.DefaultServerID) {
		return Dashboard{}, ErrInvalidInput
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Dashboard{}, fmt.Errorf("begin dashboard update: %w", err)
	}
	defer tx.Rollback()

	if input.DefaultServerID != nil {
		var exists bool
		if err := tx.QueryRowContext(ctx,
			"SELECT EXISTS (SELECT 1 FROM servers WHERE id = $1 AND organization_id = $2)",
			*input.DefaultServerID, principal.OrganizationID).Scan(&exists); err != nil {
			return Dashboard{}, fmt.Errorf("validate dashboard default machine: %w", err)
		}
		if !exists {
			return Dashboard{}, ErrNotFound
		}
	}
	if input.IsDefault {
		if _, err := tx.ExecContext(ctx,
			"UPDATE dashboards SET is_default = false, updated_at = now() WHERE organization_id = $1 AND is_default = true AND id <> $2",
			principal.OrganizationID, dashboardID); err != nil {
			return Dashboard{}, fmt.Errorf("clear previous default dashboard: %w", err)
		}
	}

	var dashboard Dashboard
	var defaultServerID sql.NullString
	err = tx.QueryRowContext(ctx, `
		UPDATE dashboards
		SET name = $3, description = $4, is_default = $5,
		    default_server_id = $6::uuid, default_time_range = $7,
		    updated_by = $8, updated_at = now()
		WHERE id = $1 AND organization_id = $2
		RETURNING id::text, name, description, is_default, default_server_id::text,
		          default_time_range, created_at, updated_at
	`, dashboardID, principal.OrganizationID, input.Name, input.Description, input.IsDefault,
		nullableUUID(input.DefaultServerID), input.DefaultTimeRange, principal.UserID).Scan(
		&dashboard.ID, &dashboard.Name, &dashboard.Description, &dashboard.IsDefault,
		&defaultServerID, &dashboard.DefaultTimeRange, &dashboard.CreatedAt, &dashboard.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Dashboard{}, ErrNotFound
	}
	if err != nil {
		return Dashboard{}, fmt.Errorf("update installation dashboard: %w", err)
	}
	if defaultServerID.Valid {
		value := defaultServerID.String
		dashboard.DefaultServerID = &value
	}
	dashboard.Panels = make([]DashboardPanel, 0)
	if err := tx.Commit(); err != nil {
		return Dashboard{}, fmt.Errorf("commit dashboard update: %w", err)
	}
	return dashboard, nil
}

func validDashboardTimeRange(value string) bool {
	switch value {
	case "panel", "1h", "6h", "24h", "7d", "30d":
		return true
	default:
		return false
	}
}

// DeleteDashboard removes only a dashboard owned by the caller's organization.
func (s *Store) DeleteDashboard(ctx context.Context, principal Principal, dashboardID string) error {
	if !validUUID(dashboardID) {
		return ErrNotFound
	}
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM dashboards WHERE id = $1 AND organization_id = $2",
		dashboardID, principal.OrganizationID)
	if err != nil {
		return fmt.Errorf("delete organization dashboard: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect dashboard deletion: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateDashboardPanel validates every referenced entity in the dashboard's organization.
func (s *Store) CreateDashboardPanel(ctx context.Context, principal Principal, dashboardID string, input DashboardPanelInput) (DashboardPanel, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.GridWidth == 0 {
		input.GridWidth = 6
	}
	if !validUUID(dashboardID) || !validUUID(input.ServerID) ||
		len(input.Title) < 2 || len(input.Title) > 120 ||
		!alertMetricPattern.MatchString(input.MetricName) ||
		!validPanelVisualization(input.Visualization) ||
		!validPanelLabelSelector(input.LabelKey, input.LabelValue) ||
		(input.TimeRange != "1h" && input.TimeRange != "6h" && input.TimeRange != "24h" &&
			input.TimeRange != "7d" && input.TimeRange != "30d") ||
		!validPanelGridWidth(input.GridWidth) ||
		input.Position < 0 || input.Position > 1000 {
		return DashboardPanel{}, ErrInvalidInput
	}
	var panel DashboardPanel
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO dashboard_panels (
			organization_id, dashboard_id, title, visualization, server_id,
			metric_name, time_range, position, grid_width, label_key, label_value, created_by, updated_by
		)
		SELECT d.organization_id, d.id, $4, $5, server.id, $6, $7, $8, $9, $10, $11, $12, $12
		FROM dashboards d
		JOIN servers server
		  ON server.id = $3
		 AND server.organization_id = d.organization_id
		WHERE d.id = $1
		  AND d.organization_id = $2
		RETURNING id::text, dashboard_id::text, title, visualization, server_id::text,
		          metric_name, time_range, position, grid_width, label_key, label_value, created_at, updated_at
	`, dashboardID, principal.OrganizationID, input.ServerID, input.Title, input.Visualization,
		input.MetricName, input.TimeRange, input.Position, input.GridWidth, input.LabelKey, input.LabelValue, principal.UserID).Scan(
		&panel.ID, &panel.DashboardID, &panel.Title, &panel.Visualization,
		&panel.ServerID, &panel.MetricName, &panel.TimeRange, &panel.Position, &panel.GridWidth,
		&panel.LabelKey, &panel.LabelValue, &panel.CreatedAt, &panel.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return DashboardPanel{}, ErrNotFound
	}
	if err != nil {
		return DashboardPanel{}, fmt.Errorf("create dashboard panel: %w", err)
	}
	return panel, nil
}

// UpdateDashboardPanel replaces a panel's saved query and presentation settings.
// The panel, parent dashboard and selected server must all belong to the installation.
func (s *Store) UpdateDashboardPanel(ctx context.Context, principal Principal, panelID string, input DashboardPanelInput) (DashboardPanel, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.GridWidth == 0 {
		input.GridWidth = 6
	}
	if !validUUID(panelID) || !validUUID(input.ServerID) ||
		len(input.Title) < 2 || len(input.Title) > 120 ||
		!alertMetricPattern.MatchString(input.MetricName) ||
		!validPanelVisualization(input.Visualization) ||
		!validPanelLabelSelector(input.LabelKey, input.LabelValue) ||
		(input.TimeRange != "1h" && input.TimeRange != "6h" && input.TimeRange != "24h" &&
			input.TimeRange != "7d" && input.TimeRange != "30d") ||
		!validPanelGridWidth(input.GridWidth) {
		return DashboardPanel{}, ErrInvalidInput
	}

	var panel DashboardPanel
	err := s.db.QueryRowContext(ctx, `
		UPDATE dashboard_panels panel
		SET title = $3,
		    visualization = $4,
		    server_id = server.id,
		    metric_name = $6,
		    time_range = $7,
		    grid_width = $8, label_key = $9, label_value = $10,
		    updated_by = $11,
		    updated_at = now()
		FROM dashboards dashboard
		JOIN servers server
		  ON server.id = $5
		 AND server.organization_id = dashboard.organization_id
		WHERE panel.id = $1
		  AND panel.organization_id = $2
		  AND dashboard.id = panel.dashboard_id
		  AND dashboard.organization_id = panel.organization_id
		RETURNING panel.id::text, panel.dashboard_id::text, panel.title,
		          panel.visualization, panel.server_id::text, panel.metric_name,
		          panel.time_range, panel.position, panel.grid_width,
		          panel.label_key, panel.label_value, panel.created_at, panel.updated_at
	`, panelID, principal.OrganizationID, input.Title, input.Visualization,
		input.ServerID, input.MetricName, input.TimeRange, input.GridWidth, input.LabelKey, input.LabelValue, principal.UserID).Scan(
		&panel.ID, &panel.DashboardID, &panel.Title, &panel.Visualization,
		&panel.ServerID, &panel.MetricName, &panel.TimeRange, &panel.Position,
		&panel.GridWidth, &panel.LabelKey, &panel.LabelValue, &panel.CreatedAt, &panel.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return DashboardPanel{}, ErrNotFound
	}
	if err != nil {
		return DashboardPanel{}, fmt.Errorf("update dashboard panel: %w", err)
	}
	return panel, nil
}

func validPanelVisualization(value string) bool {
	switch value {
	case "timeseries", "stat", "table", "bar", "area", "gauge", "heatmap", "logs":
		return true
	default:
		return false
	}
}

func validPanelLabelSelector(key, value string) bool {
	if key == "" && value == "" {
		return true
	}
	return key != "" && value != "" &&
		dashboardLabelKeyPattern.MatchString(key) &&
		len(value) <= 256 && !strings.ContainsAny(value, "\r\n\x00")
}

func validPanelGridWidth(value int) bool {
	return value == 4 || value == 6 || value == 8 || value == 12
}

// MoveDashboardPanel updates the persisted ordering of panels within one dashboard.
// The dashboard row is locked so concurrent reorder requests cannot interleave.
func (s *Store) MoveDashboardPanel(ctx context.Context, principal Principal, panelID string, position int) error {
	if !validUUID(panelID) || position < 0 || position > 1000 {
		return ErrInvalidInput
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin dashboard panel reorder: %w", err)
	}
	defer tx.Rollback()

	var dashboardID string
	if err := tx.QueryRowContext(ctx, `
		SELECT dashboard_id::text
		FROM dashboard_panels
		WHERE id = $1 AND organization_id = $2
	`, panelID, principal.OrganizationID).Scan(&dashboardID); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("load dashboard for panel reorder: %w", err)
	}

	// Lock the parent first so concurrent moves of different panels in the
	// same dashboard serialize without acquiring panel locks in opposite order.
	var lockedDashboardID string
	err = tx.QueryRowContext(ctx, `
		SELECT id::text FROM dashboards
		WHERE id = $1 AND organization_id = $2
		FOR UPDATE
	`, dashboardID, principal.OrganizationID).Scan(&lockedDashboardID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock dashboard for panel reorder: %w", err)
	}

	var currentPosition int
	err = tx.QueryRowContext(ctx, `
		SELECT position
		FROM dashboard_panels
		WHERE id = $1 AND dashboard_id = $2 AND organization_id = $3
		FOR UPDATE
	`, panelID, lockedDashboardID, principal.OrganizationID).Scan(&currentPosition)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("load dashboard panel position: %w", err)
	}

	var panelCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT count(*)
		FROM dashboard_panels
		WHERE dashboard_id = $1 AND organization_id = $2
	`, dashboardID, principal.OrganizationID).Scan(&panelCount); err != nil {
		return fmt.Errorf("count dashboard panels: %w", err)
	}
	if position >= panelCount {
		return ErrInvalidInput
	}
	if currentPosition == position {
		return tx.Commit()
	}

	if position < currentPosition {
		_, err = tx.ExecContext(ctx, `
			UPDATE dashboard_panels
			SET position = position + 1, updated_at = now()
			WHERE dashboard_id = $1 AND organization_id = $2
			  AND position >= $3 AND position < $4
		`, dashboardID, principal.OrganizationID, position, currentPosition)
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE dashboard_panels
			SET position = position - 1, updated_at = now()
			WHERE dashboard_id = $1 AND organization_id = $2
			  AND position > $3 AND position <= $4
		`, dashboardID, principal.OrganizationID, currentPosition, position)
	}
	if err != nil {
		return fmt.Errorf("shift dashboard panel positions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE dashboard_panels
		SET position = $3, updated_at = now()
		WHERE id = $1 AND organization_id = $2
	`, panelID, principal.OrganizationID, position); err != nil {
		return fmt.Errorf("save dashboard panel position: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit dashboard panel reorder: %w", err)
	}
	return nil
}

// DeleteDashboardPanel removes a panel only when it belongs to the caller's organization.
func (s *Store) DeleteDashboardPanel(ctx context.Context, principal Principal, panelID string) error {
	if !validUUID(panelID) {
		return ErrNotFound
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin dashboard panel deletion: %w", err)
	}
	defer tx.Rollback()

	var dashboardID string
	if err := tx.QueryRowContext(ctx, `
		SELECT dashboard_id::text
		FROM dashboard_panels
		WHERE id = $1 AND organization_id = $2
	`, panelID, principal.OrganizationID).Scan(&dashboardID); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("load dashboard for panel deletion: %w", err)
	}

	var lockedDashboardID string
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text FROM dashboards
		WHERE id = $1 AND organization_id = $2
		FOR UPDATE
	`, dashboardID, principal.OrganizationID).Scan(&lockedDashboardID); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("lock dashboard for panel deletion: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM dashboard_panels
		WHERE id = $1 AND dashboard_id = $2 AND organization_id = $3
	`, panelID, lockedDashboardID, principal.OrganizationID); err != nil {
		return fmt.Errorf("delete dashboard panel: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		WITH ranked AS (
			SELECT id, row_number() OVER (ORDER BY position, id) - 1 AS next_position
			FROM dashboard_panels
			WHERE dashboard_id = $1 AND organization_id = $2
		)
		UPDATE dashboard_panels panel
		SET position = ranked.next_position, updated_at = now()
		FROM ranked
		WHERE panel.id = ranked.id
		  AND panel.position <> ranked.next_position
	`, lockedDashboardID, principal.OrganizationID); err != nil {
		return fmt.Errorf("compact dashboard panel positions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit dashboard panel deletion: %w", err)
	}
	return nil
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
		SELECT id::text, name, server_id::text, metric_name, operator, threshold, enabled, created_at
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
		var serverID sql.NullString
		if err := rows.Scan(&item.ID, &item.Name, &serverID, &item.MetricName,
			&item.Operator, &item.Threshold, &item.Enabled, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan alert rule: %w", err)
		}
		if serverID.Valid {
			item.ServerID = &serverID.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert rules: %w", err)
	}
	return items, nil
}

func (s *Store) CreateAlertRule(ctx context.Context, principal Principal, input AlertRule) (AlertRule, error) {
	input.Name = strings.TrimSpace(input.Name)
	if len(input.Name) < 2 || len(input.Name) > 120 || !alertMetricPattern.MatchString(input.MetricName) ||
		(input.Operator != "gt" && input.Operator != "gte" && input.Operator != "lt" && input.Operator != "lte") ||
		math.IsNaN(input.Threshold) || math.IsInf(input.Threshold, 0) {
		return AlertRule{}, ErrInvalidInput
	}
	if input.ServerID != nil && !validUUID(*input.ServerID) {
		return AlertRule{}, ErrInvalidInput
	}
	if input.ServerID != nil {
		var exists bool
		if err := s.db.QueryRowContext(ctx,
			"SELECT EXISTS (SELECT 1 FROM servers WHERE id = $1 AND organization_id = $2)",
			*input.ServerID, principal.OrganizationID).Scan(&exists); err != nil {
			return AlertRule{}, fmt.Errorf("validate alert rule server: %w", err)
		}
		if !exists {
			return AlertRule{}, ErrNotFound
		}
	}
	var serverID sql.NullString
	var result AlertRule
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO alert_rules (organization_id, name, server_id, metric_name, operator, threshold, enabled, created_by)
		VALUES ($1, $2, $3::uuid, $4, $5, $6, true, $7)
		RETURNING id::text, name, server_id::text, metric_name, operator, threshold, enabled, created_at
	`, principal.OrganizationID, input.Name, nullableUUID(input.ServerID),
		input.MetricName, input.Operator, input.Threshold, principal.UserID).Scan(
		&result.ID, &result.Name, &serverID, &result.MetricName,
		&result.Operator, &result.Threshold, &result.Enabled, &result.CreatedAt)
	if err != nil {
		return AlertRule{}, fmt.Errorf("create alert rule: %w", err)
	}
	if serverID.Valid {
		result.ServerID = &serverID.String
	}
	return result, nil
}

// UpdateAlertRule replaces a rule's threshold, scope and active state.
// When a rule's condition changes, any prior firing events are closed and a recovery
// notification is recorded before the updated rule becomes visible to collectors.
func (s *Store) UpdateAlertRule(ctx context.Context, principal Principal, ruleID string, input AlertRule) (AlertRule, error) {
	input.Name = strings.TrimSpace(input.Name)
	if !validUUID(ruleID) || len(input.Name) < 2 || len(input.Name) > 120 ||
		!alertMetricPattern.MatchString(input.MetricName) ||
		(input.Operator != "gt" && input.Operator != "gte" && input.Operator != "lt" && input.Operator != "lte") ||
		math.IsNaN(input.Threshold) || math.IsInf(input.Threshold, 0) {
		return AlertRule{}, ErrInvalidInput
	}
	if input.ServerID != nil && !validUUID(*input.ServerID) {
		return AlertRule{}, ErrInvalidInput
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AlertRule{}, fmt.Errorf("begin alert rule update: %w", err)
	}
	defer tx.Rollback()

	var previous AlertRule
	var previousServerID sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT id::text, name, server_id::text, metric_name, operator, threshold, enabled, created_at
		FROM alert_rules
		WHERE id = $1 AND organization_id = $2
		FOR UPDATE
	`, ruleID, principal.OrganizationID).Scan(
		&previous.ID, &previous.Name, &previousServerID, &previous.MetricName,
		&previous.Operator, &previous.Threshold, &previous.Enabled, &previous.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return AlertRule{}, ErrNotFound
	}
	if err != nil {
		return AlertRule{}, fmt.Errorf("load alert rule for update: %w", err)
	}
	if previousServerID.Valid {
		previous.ServerID = &previousServerID.String
	}

	if input.ServerID != nil {
		var exists bool
		if err := tx.QueryRowContext(ctx,
			"SELECT EXISTS (SELECT 1 FROM servers WHERE id = $1 AND organization_id = $2)",
			*input.ServerID, principal.OrganizationID).Scan(&exists); err != nil {
			return AlertRule{}, fmt.Errorf("validate updated alert rule machine: %w", err)
		}
		if !exists {
			return AlertRule{}, ErrNotFound
		}
	}

	conditionChanged := previous.MetricName != input.MetricName ||
		previous.Operator != input.Operator ||
		previous.Threshold != input.Threshold ||
		previous.Enabled != input.Enabled ||
		nullableUUID(previous.ServerID) != nullableUUID(input.ServerID)

	var updated AlertRule
	var updatedServerID sql.NullString
	err = tx.QueryRowContext(ctx, `
		UPDATE alert_rules
		SET name = $3, server_id = $4::uuid, metric_name = $5,
		    operator = $6, threshold = $7, enabled = $8
		WHERE id = $1 AND organization_id = $2
		RETURNING id::text, name, server_id::text, metric_name, operator, threshold, enabled, created_at
	`, ruleID, principal.OrganizationID, input.Name, nullableUUID(input.ServerID),
		input.MetricName, input.Operator, input.Threshold, input.Enabled).Scan(
		&updated.ID, &updated.Name, &updatedServerID, &updated.MetricName,
		&updated.Operator, &updated.Threshold, &updated.Enabled, &updated.CreatedAt,
	)
	if err != nil {
		return AlertRule{}, fmt.Errorf("save alert rule update: %w", err)
	}
	if updatedServerID.Valid {
		updated.ServerID = &updatedServerID.String
	}

	if conditionChanged {
		rows, err := tx.QueryContext(ctx, `
			UPDATE alert_events
			SET status = 'resolved', last_observed_at = now(), resolved_at = now()
			WHERE rule_id = $1 AND organization_id = $2 AND status = 'firing'
			RETURNING id::text
		`, ruleID, principal.OrganizationID)
		if err != nil {
			return AlertRule{}, fmt.Errorf("close events for changed alert rule: %w", err)
		}
		eventIDs := make([]string, 0)
		for rows.Next() {
			var eventID string
			if err := rows.Scan(&eventID); err != nil {
				_ = rows.Close()
				return AlertRule{}, fmt.Errorf("scan resolved alert event: %w", err)
			}
			eventIDs = append(eventIDs, eventID)
		}
		rowsErr := rows.Err()
		closeErr := rows.Close()
		if rowsErr != nil {
			return AlertRule{}, fmt.Errorf("iterate resolved alert events: %w", rowsErr)
		}
		if closeErr != nil {
			return AlertRule{}, fmt.Errorf("close resolved alert event rows: %w", closeErr)
		}
		for _, eventID := range eventIDs {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO alert_notifications (organization_id, event_id, user_id, kind, title, message)
				SELECT $1, $2, membership.user_id, 'recovered', $3, $4
				FROM organization_memberships membership
				WHERE membership.organization_id = $1
				ON CONFLICT (event_id, user_id, kind) DO NOTHING
			`, principal.OrganizationID, eventID,
					"Alert updated: "+previous.Name,
					"The alert condition or scope changed. The previous firing event was closed.")
			if err != nil {
				return AlertRule{}, fmt.Errorf("notify changed alert rule recovery: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return AlertRule{}, fmt.Errorf("commit alert rule update: %w", err)
	}
	return updated, nil
}

func nullableUUID(value *string) any {
	if value == nil || *value == "" {
		return nil
	}
	return *value
}

func (s *Store) DeleteAlertRule(ctx context.Context, principal Principal, ruleID string) error {
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
		       server.name, event.metric_name, event.status,
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
		var resolvedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.RuleID, &item.RuleName, &item.ServerID, &item.ServerName,
			&item.MetricName, &item.Status, &item.LastValue, &item.TriggeredAt,
			&item.LastObserved, &resolvedAt); err != nil {
			return nil, fmt.Errorf("scan alert event: %w", err)
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

