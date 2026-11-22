package httpapi

import (
	"context"
	"net/http"

	"github.com/WorkRCS/monitor_me/backend/internal/identity"
)

// Dependency represents a backend service that must be reachable before the
// API is considered ready to receive application traffic.
type Dependency struct {
	Name  string
	Check func(context.Context) error
}

// NewRouter registers health, authentication, and organization-scoped inventory routes.
func NewRouter(version string, dependencies []Dependency, identities *identity.Store) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health/live", healthHandler("ok", version, nil))
	mux.HandleFunc("GET /health/ready", healthHandler("ready", version, dependencies))

	if identities != nil {
		api := &API{identities: identities}
		mux.HandleFunc("POST /v1/auth/register", api.register)
		mux.HandleFunc("POST /v1/auth/login", api.login)
		mux.Handle("GET /v1/auth/me", api.requireAuth(http.HandlerFunc(api.me)))
		mux.Handle("POST /v1/auth/logout", api.requireAuth(http.HandlerFunc(api.logout)))
		mux.Handle("GET /v1/servers", api.requireAuth(http.HandlerFunc(api.listServers)))
		mux.Handle("POST /v1/servers", api.requireAuth(http.HandlerFunc(api.createServer)))
		mux.Handle("POST /v1/servers/{serverID}/enrollment-token", api.requireAuth(http.HandlerFunc(api.reissueEnrollmentToken)))
		mux.Handle("POST /v1/enrollment-tokens/{tokenID}/revoke", api.requireAuth(http.HandlerFunc(api.revokeEnrollmentToken)))
		mux.Handle("POST /v1/agents/{agentID}/revoke", api.requireAuth(http.HandlerFunc(api.revokeAgent)))
	}

	return mux
}
