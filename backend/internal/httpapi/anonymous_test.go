package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/monitor-me/backend/internal/identity"
)

func TestAnonymousModeDisablesInteractiveAuthenticationEndpoints(t *testing.T) {
	handler := NewRouter(
		"test-version",
		nil,
		identity.NewStore(nil),
		LiveDependencies{DisableAuthentication: true},
	)

	for _, path := range []string{"/v1/auth/login", "/v1/auth/register"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusNotFound {
				t.Fatalf("expected HTTP %d, got %d: %s", http.StatusNotFound, recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "authentication_disabled") {
				t.Fatalf("expected authentication_disabled response, got %s", recorder.Body.String())
			}
		})
	}
}
