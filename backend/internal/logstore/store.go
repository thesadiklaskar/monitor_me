package logstore

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	maxBatchEvents = 32
	maxEventBody = 16 << 10
	maxBatchBytes = 768 << 10
	maxQueryRange = 7 * 24 * time.Hour
	maxQueryRows = 200
)

var (
	batchIDPattern = regexp.MustCompile("^[0-9a-f]{32}$")
	eventIDPattern = regexp.MustCompile("^[0-9a-f]{64}$")
	attributeKeyPattern = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_.-]{0,63}$")
)

var (
	ErrInvalidBatch = errors.New("invalid log batch")
	ErrInvalidQuery = errors.New("invalid log query")
	ErrUnauthorized = errors.New("agent session is invalid, expired, or revoked")
	ErrIdempotencyConflict = errors.New("log batch ID was reused with different content")
)

type Event struct {
	ID string `json:"event_id"`
	EventTimeUnixNanos int64 `json:"event_time_unix_nanos"`
	ObservedAtUnixNanos int64 `json:"observed_at_unix_nanos"`
	Source string `json:"source"`
	Severity string `json:"severity"`
	Body string `json:"body"`
	Attributes map[string]string `json:"attributes"`
}

type Batch struct {
	ID string `json:"batch_id"`
	CollectedAtUnixNanos int64 `json:"collected_at_unix_nanos"`
	SchemaVersion uint32 `json:"schema_version"`
	Events []Event `json:"events"`
}

type Query struct {
	OrganizationID string
	ServerID string
	ProjectID string
	From time.Time
	To time.Time
	Search string
	Severity string
	Source string
	Limit int
}

type Result struct {
	EventID string `json:"event_id"`
	EventTimeUnixNanos int64 `json:"event_time_unix_nanos"`
	Source string `json:"source"`
	Severity string `json:"severity"`
	Body string `json:"body"`
	Attributes map[string]string `json:"attributes"`
	ProjectID string `json:"project_id,omitempty"`
}

type Config struct {
	URL string
	Username string
	Password string
	Database string
	RequestTimeout time.Duration
}

type Store struct {
	db *sql.DB
	endpoint *url.URL
	username string
	password string
	database string
	client *http.Client
}

func New(ctx context.Context, db *sql.DB, cfg Config) (*Store, error) {
	if db == nil || cfg.URL == "" {
		return nil, errors.New("PostgreSQL and ClickHouse endpoints are required for log storage")
	}
	parsed, err := url.Parse(cfg.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("CLICKHOUSE_URL must be an HTTP(S) endpoint without credentials, query or fragment")
	}
	database := cfg.Database
	if database == "" {
		database = "monitor_me"
	}
	if !regexp.MustCompile("^[A-Za-z_][A-Za-z0-9_]{0,63}$").MatchString(database) {
		return nil, errors.New("ClickHouse database name is invalid")
	}
	timeout := cfg.RequestTimeout
	if timeout < time.Second || timeout > 30*time.Second {
		timeout = 8 * time.Second
	}
	store := &Store{
		db: db, endpoint: parsed, username: cfg.Username, password: cfg.Password,
		database: database, client: &http.Client{Timeout: timeout},
	}
	if err := store.execute(ctx, "CREATE DATABASE IF NOT EXISTS "+database, nil); err != nil {
		return nil, fmt.Errorf("initialize ClickHouse database: %w", err)
	}
	createTable := "CREATE TABLE IF NOT EXISTS "+database+".logs ("+
		"organization_id UUID, server_id UUID, agent_id UUID, batch_id String, event_id String, "+
		"event_time DateTime64(6, 'UTC'), observed_at DateTime64(6, 'UTC'), "+
		"source LowCardinality(String), severity LowCardinality(String), "+
		"body String CODEC(ZSTD(3)), attributes Map(String, String), project_id Nullable(UUID), "+
		"received_at DateTime64(3, 'UTC') DEFAULT now64(3)"+
		") ENGINE = ReplacingMergeTree(received_at) "+
		"PARTITION BY toYYYYMM(event_time) "+
		"ORDER BY (organization_id, server_id, event_time, event_id) "+
		"TTL event_time + INTERVAL 30 DAY DELETE "+
		"SETTINGS index_granularity = 8192"
	if err := store.execute(ctx, createTable, nil); err != nil {
		return nil, fmt.Errorf("initialize ClickHouse logs table: %w", err)
	}
	return store, nil
}

func validateBatch(batch Batch, now time.Time) error {
	if !batchIDPattern.MatchString(batch.ID) || batch.SchemaVersion != 1 ||
		batch.CollectedAtUnixNanos <= 0 || len(batch.Events) == 0 || len(batch.Events) > maxBatchEvents {
		return ErrInvalidBatch
	}
	collectedAt := time.Unix(0, batch.CollectedAtUnixNanos)
	if collectedAt.After(now.Add(5*time.Minute)) {
		return ErrInvalidBatch
	}
	var estimatedBytes int
	for _, event := range batch.Events {
		if !eventIDPattern.MatchString(event.ID) || event.EventTimeUnixNanos <= 0 ||
			event.ObservedAtUnixNanos <= 0 || len(event.Body) == 0 || len(event.Body) > maxEventBody ||
			len(event.Source) == 0 || len(event.Source) > 128 ||
			!validSeverity(event.Severity) || len(event.Attributes) > 16 {
			return ErrInvalidBatch
		}
		if time.Unix(0, event.EventTimeUnixNanos).After(now.Add(5*time.Minute)) ||
			time.Unix(0, event.ObservedAtUnixNanos).After(now.Add(5*time.Minute)) {
			return ErrInvalidBatch
		}
		for key, value := range event.Attributes {
			if !attributeKeyPattern.MatchString(key) || len(value) > 256 ||
				strings.ContainsAny(value, "\r\n\x00") {
				return ErrInvalidBatch
			}
		}
		estimatedBytes += len(event.Body) + len(event.Source) + len(event.ID) + 512
	}
	if estimatedBytes > maxBatchBytes {
		return ErrInvalidBatch
	}
	return nil
}

func validSeverity(value string) bool {
	switch value {
	case "emergency", "alert", "critical", "error", "warning", "notice", "info", "debug", "unknown":
		return true
	default:
		return false
	}
}

// Ingest stores a batch idempotently. PostgreSQL serializes each agent's batch
// acceptance while ClickHouse stores the high-volume event rows. Stable event IDs
// plus FINAL reads make retries safe if the ClickHouse insert succeeds but the
// PostgreSQL acknowledgement ledger cannot commit.
func (s *Store) Ingest(ctx context.Context, agentID, sessionToken string, batch Batch) (bool, string, error) {
	if err := validateBatch(batch, time.Now().UTC()); err != nil {
		return false, "invalid_batch", ErrInvalidBatch
	}
	payload, err := json.Marshal(batch)
	if err != nil {
		return false, "", fmt.Errorf("encode log batch for idempotency: %w", err)
	}
	payloadHash := sha256.Sum256(payload)
	sessionHash := sha256.Sum256([]byte(sessionToken))

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, "", fmt.Errorf("begin log batch transaction: %w", err)
	}
	defer tx.Rollback()

	var serverID, organizationID string
	err = tx.QueryRowContext(ctx, `
		SELECT a.server_id::text, server.organization_id::text
		FROM agents a
		JOIN servers server ON server.id = a.server_id
		JOIN agent_sessions session ON session.agent_id = a.id
		WHERE a.id = $1
		  AND a.status <> 'revoked'
		  AND server.status <> 'disabled'
		  AND session.token_hash = $2
		  AND session.revoked_at IS NULL
		  AND session.expires_at > now()
		FOR UPDATE OF a, session
	`, agentID, sessionHash[:]).Scan(&serverID, &organizationID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", ErrUnauthorized
	}
	if err != nil {
		return false, "", fmt.Errorf("validate log delivery session: %w", err)
	}

	var priorHash []byte
	err = tx.QueryRowContext(ctx,
		"SELECT payload_hash FROM agent_log_batches WHERE agent_id = $1 AND batch_id = $2",
		agentID, batch.ID).Scan(&priorHash)
	if err == nil {
		if !bytes.Equal(priorHash, payloadHash[:]) {
			return false, "idempotency_conflict", ErrIdempotencyConflict
		}
		if err := tx.Commit(); err != nil {
			return false, "", fmt.Errorf("confirm duplicate log batch: %w", err)
		}
		return true, "", nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, "", fmt.Errorf("check log batch idempotency: %w", err)
	}

	projectByUnit, err := s.projectMappings(ctx, tx, organizationID, serverID)
	if err != nil {
		return false, "", err
	}
	rows := make([]clickHouseRow, 0, len(batch.Events))
	for _, event := range batch.Events {
		projectID := projectByUnit[event.Attributes["systemd_unit"]]
		var nullableProject *string
		if projectID != "" {
			nullableProject = &projectID
		}
		rows = append(rows, clickHouseRow{
			OrganizationID: organizationID, ServerID: serverID, AgentID: agentID, BatchID: batch.ID,
			EventID: event.ID, EventTime: clickHouseTime(event.EventTimeUnixNanos),
			ObservedAt: clickHouseTime(event.ObservedAtUnixNanos), Source: event.Source,
			Severity: event.Severity, Body: event.Body, Attributes: event.Attributes, ProjectID: nullableProject,
		})
	}
	if err := s.insertRows(ctx, rows); err != nil {
		return false, "", fmt.Errorf("persist log batch to ClickHouse: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO agent_log_batches (organization_id, server_id, agent_id, batch_id, payload_hash, event_count)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, organizationID, serverID, agentID, batch.ID, payloadHash[:], len(batch.Events)); err != nil {
		return false, "", fmt.Errorf("record durable log batch acknowledgement: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_sessions SET last_seen_at = now()
		WHERE agent_id = $1 AND token_hash = $2
	`, agentID, sessionHash[:]); err != nil {
		return false, "", fmt.Errorf("refresh log sender session: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE agents SET status = 'online', last_seen_at = now(), updated_at = now()
		WHERE id = $1
	`, agentID); err != nil {
		return false, "", fmt.Errorf("refresh log sender liveness: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE servers SET status = 'online', updated_at = now() WHERE id = $1", serverID); err != nil {
		return false, "", fmt.Errorf("refresh log sender server liveness: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, "", fmt.Errorf("commit log batch acknowledgement: %w", err)
	}
	return true, "", nil
}

func (s *Store) projectMappings(ctx context.Context, tx *sql.Tx, organizationID, serverID string) (map[string]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT workload_selector, project_id::text
		FROM server_project_mappings
		WHERE organization_id = $1 AND server_id = $2 AND workload_type = 'systemd'
	`, organizationID, serverID)
	if err != nil {
		return nil, fmt.Errorf("load log project mappings: %w", err)
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var selector, projectID string
		if err := rows.Scan(&selector, &projectID); err != nil {
			return nil, fmt.Errorf("scan log project mapping: %w", err)
		}
		result[selector] = projectID
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate log project mappings: %w", err)
	}
	return result, nil
}

type clickHouseRow struct {
	OrganizationID string `json:"organization_id"`
	ServerID string `json:"server_id"`
	AgentID string `json:"agent_id"`
	BatchID string `json:"batch_id"`
	EventID string `json:"event_id"`
	EventTime string `json:"event_time"`
	ObservedAt string `json:"observed_at"`
	Source string `json:"source"`
	Severity string `json:"severity"`
	Body string `json:"body"`
	Attributes map[string]string `json:"attributes"`
	ProjectID *string `json:"project_id"`
}

func clickHouseTime(unixNanos int64) string {
	return time.Unix(0, unixNanos).UTC().Format("2006-01-02 15:04:05.000000")
}

func (s *Store) insertRows(ctx context.Context, rows []clickHouseRow) error {
	var body bytes.Buffer
	writer := bufio.NewWriter(&body)
	encoder := json.NewEncoder(writer)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			return fmt.Errorf("encode ClickHouse JSONEachRow: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	if body.Len() > maxBatchBytes {
		return errors.New("encoded ClickHouse log batch exceeds the configured size limit")
	}
	return s.execute(ctx, "INSERT INTO "+s.database+".logs FORMAT JSONEachRow", &body)
}

func (s *Store) execute(ctx context.Context, query string, body io.Reader) error {
	values := url.Values{}
	values.Set("query", query)
	requestURL := *s.endpoint
	requestURL.RawQuery = values.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), body)
	if err != nil {
		return fmt.Errorf("create ClickHouse request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if s.username != "" {
		request.SetBasicAuth(s.username, s.password)
	}
	response, err := s.client.Do(request)
	if err != nil {
		return fmt.Errorf("send ClickHouse request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return fmt.Errorf("ClickHouse returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	_, err = io.Copy(io.Discard, response.Body)
	return err
}

func (s *Store) Query(ctx context.Context, query Query) ([]Result, error) {
	if !validUUID(query.OrganizationID) || !validUUID(query.ServerID) ||
		(query.ProjectID != "" && !validUUID(query.ProjectID)) ||
		query.From.IsZero() || query.To.IsZero() || !query.From.Before(query.To) ||
		query.To.After(time.Now().UTC().Add(time.Minute)) ||
		query.To.Sub(query.From) > maxQueryRange ||
		query.Limit < 1 || query.Limit > maxQueryRows ||
		len(query.Search) > 256 || len(query.Source) > 128 ||
		(query.Severity != "" && !validSeverity(query.Severity)) {
		return nil, ErrInvalidQuery
	}
	sqlText := "SELECT event_id, toUnixTimestamp64Nano(event_time) AS event_time_unix_nanos, "+
		"source, severity, body, attributes, ifNull(toString(project_id), '') AS project_id "+
		"FROM "+s.database+".logs FINAL "+
		"WHERE organization_id = {org_id:UUID} AND server_id = {server_id:UUID} "+
		"AND event_time >= {from:DateTime64(6, 'UTC')} AND event_time < {to:DateTime64(6, 'UTC')} "+
		"AND ({search:String} = '' OR positionCaseInsensitiveUTF8(body, {search:String}) > 0) "+
		"AND ({severity:String} = '' OR severity = {severity:String}) "+
		"AND ({source:String} = '' OR source = {source:String})"
	params := url.Values{
		"param_org_id": {query.OrganizationID},
		"param_server_id": {query.ServerID},
		"param_from": {query.From.UTC().Format("2006-01-02 15:04:05.000000")},
		"param_to": {query.To.UTC().Format("2006-01-02 15:04:05.000000")},
		"param_search": {query.Search},
		"param_severity": {query.Severity},
		"param_source": {query.Source},
		"param_limit": {strconv.Itoa(query.Limit)},
	}
	if query.ProjectID != "" {
		sqlText += " AND project_id = {project_id:UUID}"
		params.Set("param_project_id", query.ProjectID)
	}
	sqlText += " ORDER BY event_time DESC, event_id DESC LIMIT {limit:UInt16} FORMAT JSONEachRow"

	requestURL := *s.endpoint
	requestURL.RawQuery = params.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), strings.NewReader(sqlText))
	if err != nil {
		return nil, fmt.Errorf("create ClickHouse log query: %w", err)
	}
	request.Header.Set("Content-Type", "text/plain")
	if s.username != "" {
		request.SetBasicAuth(s.username, s.password)
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("query ClickHouse logs: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return nil, fmt.Errorf("ClickHouse log query returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	results := make([]Result, 0)
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		var row Result
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return nil, fmt.Errorf("decode ClickHouse log row: %w", err)
		}
		results = append(results, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read ClickHouse log rows: %w", err)
	}
	return results, nil
}

func validUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		switch index {
		case 8, 13, 18, 23:
			if character != '-' { return false }
		default:
			if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
				return false
			}
		}
	}
	return true
}
