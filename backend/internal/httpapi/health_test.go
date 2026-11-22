package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthEndpoints(t *testing.T) {
	dependencies := []Dependency{
		{Name: "postgres", Check: func(context.Context) error { return nil }},
		{Name: "redis", Check: func(context.Context) error { return nil }},
	}

	tests := []struct {
		name           string
		path           string
		expectedStatus string
	}{
		{name: "liveness", path: "/health/live", expectedStatus: "ok"},
		{name: "readiness", path: "/health/ready", expectedStatus: "ready"},
	}

	handler := NewRouter("test-version", dependencies, nil)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("expected HTTP %d, got %d", http.StatusOK, recorder.Code)
			}

			var response healthResponse
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatalf("decode health response: %v", err)
			}

			if response.Status != test.expectedStatus {
				t.Fatalf("expected status %q, got %q", test.expectedStatus, response.Status)
			}
			if response.Service != "monitor-me-api" {
				t.Fatalf("unexpected service name %q", response.Service)
			}
			if response.Version != "test-version" {
				t.Fatalf("expected test version, got %q", response.Version)
			}
			if response.CheckedAt == "" {
				t.Fatal("expected checked_at to be populated")
			}
		})
	}
}

func TestReadinessFailsWhenDependencyIsUnavailable(t *testing.T) {
	handler := NewRouter("test-version", []Dependency{{
		Name: "postgres",
		Check: func(context.Context) error {
			return errors.New("connection details must not be exposed")
		}}}, nil)

	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected HTTP %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}

	var response healthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode readiness response: %v", err)
	}
	if response.Status != "not_ready" {
		t.Fatalf("expected not_ready status, got %q", response.Status)
	}
	if response.Dependencies["postgres"] != "unavailable" {
		t.Fatalf("expected postgres unavailable, got %q", response.Dependencies["postgres"])
	}
	if strings.Contains(recorder.Body.String(), "connection details") {
		t.Fatal("readiness response exposed an internal connection error")
	}
}

func TestHealthEndpointRejectsNonGetMethods(t *testing.T) {
	handler := NewRouter("test-version", nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/health/live", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected HTTP %d, got %d", http.StatusMethodNotAllowed, recorder.Code)
	}
	if allow := recorder.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("expected Allow header %q, got %q", http.MethodGet, allow)
	}
}
