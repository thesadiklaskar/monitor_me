package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"monitor-me.internal/backend/internal/identity"
)

func TestInteractiveAuthenticationRoutesAreNotRegistered(t *testing.T) {
	handler := NewRouter("test-version", nil, identity.NewStore(nil))

	for _, route := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/v1/auth/login"},
		{method: http.MethodPost, path: "/v1/auth/register"},
		{method: http.MethodPost, path: "/v1/auth/logout"},
		{method: http.MethodGet, path: "/v1/auth/me"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("expected HTTP %d, got %d: %s", http.StatusNotFound, recorder.Code, recorder.Body.String())
			}
		})
	}
}
