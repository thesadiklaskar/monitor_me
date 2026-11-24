package logstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const (
	testOrganizationID = "11111111-1111-4111-8111-111111111111"
	testServerID       = "22222222-2222-4222-8222-222222222222"
	testProjectID      = "33333333-3333-4333-8333-333333333333"
)

func testQuery() Query {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return Query{
		OrganizationID: testOrganizationID,
		ServerID: testServerID,
		From: now.Add(-time.Hour),
		To: now,
		Limit: 200,
	}
}

func testStore(server *httptest.Server) *Store {
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		panic(err)
	}
	return &Store{
		endpoint: endpoint,
		username: "monitor_me",
		password: "test-password",
		database: "monitor_me",
		client: server.Client(),
	}
}

func TestValidateBatchRejectsDuplicateEventIDs(t *testing.T) {
	now := time.Now().UTC()
	event := Event{
		ID: strings.Repeat("a", 64),
		EventTimeUnixNanos: now.UnixNano(),
		ObservedAtUnixNanos: now.UnixNano(),
		Source: "sshd.service",
		Severity: "warning",
		Body: "authentication failed",
	}
	batch := Batch{
		ID: strings.Repeat("b", 32),
		CollectedAtUnixNanos: now.UnixNano(),
		SchemaVersion: 1,
		Events: []Event{event, event},
	}
	if err := validateBatch(batch, now); !errors.Is(err, ErrInvalidBatch) {
		t.Fatalf("validateBatch() error = %v, want ErrInvalidBatch", err)
	}
}

func TestValidateBatchAcceptsBoundedValidBatch(t *testing.T) {
	now := time.Now().UTC()
	batch := Batch{
		ID: strings.Repeat("b", 32),
		CollectedAtUnixNanos: now.UnixNano(),
		SchemaVersion: 1,
		Events: []Event{{
			ID: strings.Repeat("a", 64),
			EventTimeUnixNanos: now.Add(-time.Second).UnixNano(),
			ObservedAtUnixNanos: now.UnixNano(),
			Source: "sshd.service",
			Severity: "warning",
			Body: "authentication failed",
			Attributes: map[string]string{"systemd_unit": "sshd.service"},
		}},
	}
	if err := validateBatch(batch, now); err != nil {
		t.Fatalf("validateBatch() unexpected error: %v", err)
	}
}

func TestQueryAlwaysScopesOrganizationAndServerAndBindsFilters(t *testing.T) {
	var receivedQuery string
	var receivedParams url.Values
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", request.Method)
		}
		if request.Header.Get("Content-Type") != "text/plain" {
			t.Errorf("content type = %q", request.Header.Get("Content-Type"))
		}
		if username, password, ok := request.BasicAuth(); !ok || username != "monitor_me" || password != "test-password" {
			t.Errorf("unexpected ClickHouse basic auth: %q / %v", username, ok)
		}
		receivedParams = request.URL.Query()
		body, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			t.Errorf("read ClickHouse query body: %v", readErr)
		}
		receivedQuery = string(body)
		_, _ = fmt.Fprintln(writer, `{"event_id":"`+strings.Repeat("a", 64)+`","event_time_unix_nanos":1795524000000000000,"source":"sshd.service","severity":"warning","body":"authentication failed","attributes":{"systemd_unit":"sshd.service"},"project_id":"`+testProjectID+`"}`)
	}))
	defer server.Close()

	query := testQuery()
	query.ProjectID = testProjectID
	query.Search = "authentication"
	query.Severity = "warning"
	query.Source = "sshd.service"
	results, err := testStore(server).Query(context.Background(), query)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(results) != 1 || results[0].ProjectID != testProjectID {
		t.Fatalf("Query() results = %#v", results)
	}
	for _, required := range []string{
		"organization_id = {org_id:UUID}",
		"server_id = {server_id:UUID}",
		"project_id = {project_id:UUID}",
		"positionCaseInsensitiveUTF8(body, {search:String})",
		"severity = {severity:String}",
		"source = {source:String}",
	} {
		if !strings.Contains(receivedQuery, required) {
			t.Errorf("ClickHouse query missing %q: %s", required, receivedQuery)
		}
	}
	for key, want := range map[string]string{
		"param_org_id": testOrganizationID,
		"param_server_id": testServerID,
		"param_project_id": testProjectID,
		"param_search": "authentication",
		"param_severity": "warning",
		"param_source": "sshd.service",
		"param_limit": "200",
	} {
		if got := receivedParams.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestQueryUsesKeysetCursorAndBindsItsParameters(t *testing.T) {
	var receivedQuery string
	var receivedParams url.Values
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedParams = request.URL.Query()
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read ClickHouse query body: %v", err)
		}
		receivedQuery = string(body)
		_, _ = fmt.Fprintln(writer, `{"event_id":"`+strings.Repeat("a", 64)+`","event_time_unix_nanos":1795524000000000000,"event_time_unix_micros":1795524000000000,"source":"sshd.service","severity":"warning","body":"authentication failed","attributes":{},"project_id":""}`)
	}))
	defer server.Close()

	query := testQuery()
	query.Limit = 201
	query.BeforeTimeUnixMicros = query.To.Add(-time.Minute).UnixMicro()
	query.BeforeEventID = strings.Repeat("b", 64)
	results, err := testStore(server).Query(context.Background(), query)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(results) != 1 || results[0].EventTimeUnixMicros != 1795524000000000 {
		t.Fatalf("Query() results = %#v", results)
	}
	if !strings.Contains(receivedQuery, "tuple(event_time, event_id) < tuple(fromUnixTimestamp64Micro({before_time:Int64}), {before_event_id:String})") {
		t.Fatalf("ClickHouse query missing keyset cursor predicate: %s", receivedQuery)
	}
	if receivedParams.Get("param_before_time") != strconv.FormatInt(query.BeforeTimeUnixMicros, 10) ||
		receivedParams.Get("param_before_event_id") != query.BeforeEventID {
		t.Fatalf("cursor parameters were not bound correctly: %#v", receivedParams)
	}
}

func TestQueryRejectsPartialOrOutOfRangeCursor(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	tests := []struct {
		name string
		set func(*Query)
	}{
		{name: "time without event ID", set: func(query *Query) {
			query.BeforeTimeUnixMicros = query.To.Add(-time.Minute).UnixMicro()
		}},
		{name: "event ID without time", set: func(query *Query) {
			query.BeforeEventID = strings.Repeat("b", 64)
		}},
		{name: "timestamp outside query window", set: func(query *Query) {
			query.BeforeTimeUnixMicros = query.From.Add(-time.Second).UnixMicro()
			query.BeforeEventID = strings.Repeat("b", 64)
		}},
		{name: "invalid event ID", set: func(query *Query) {
			query.BeforeTimeUnixMicros = query.To.Add(-time.Minute).UnixMicro()
			query.BeforeEventID = "invalid"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query := testQuery()
			test.set(&query)
			if _, err := testStore(server).Query(context.Background(), query); !errors.Is(err, ErrInvalidQuery) {
				t.Fatalf("Query() error = %v, want ErrInvalidQuery", err)
			}
		})
	}
}

func TestQueryRejectsInvalidRangeBeforeClickHouseRequest(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		called = true
	}))
	defer server.Close()

	query := testQuery()
	query.To = query.From
	_, err := testStore(server).Query(context.Background(), query)
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("Query() error = %v, want ErrInvalidQuery", err)
	}
	if called {
		t.Fatal("ClickHouse was called for an invalid query")
	}
}

func TestQueryReturnsClickHouseErrorsWithoutHidingFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "table unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := testStore(server).Query(context.Background(), testQuery())
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("Query() error = %v, want HTTP 503 error", err)
	}
}

func TestQueryRejectsMalformedClickHouseRows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = fmt.Fprintln(writer, `{"event_id":`)
	}))
	defer server.Close()

	_, err := testStore(server).Query(context.Background(), testQuery())
	if err == nil || !strings.Contains(err.Error(), "decode ClickHouse log row") {
		t.Fatalf("Query() error = %v, want malformed-row error", err)
	}
}

func TestValidUUID(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{
		{testOrganizationID, true},
		{"AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA", true},
		{"not-a-uuid", false},
		{"11111111111141118111111111111111", false},
		{"11111111-1111-4111-8111-11111111111g", false},
	} {
		if got := validUUID(test.value); got != test.valid {
			t.Errorf("validUUID(%q) = %v, want %v", test.value, got, test.valid)
		}
	}
}
