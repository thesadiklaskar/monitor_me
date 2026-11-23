package httpapi

import (
	"context"
	"net/http"

	"github.com/WorkRCS/monitor_me/backend/internal/identity"
	"github.com/WorkRCS/monitor_me/backend/internal/telemetry"
	redisv9 "github.com/redis/go-redis/v9"
)

// Dependency represents a backend service that must be reachable before the
// API is considered ready to receive application traffic.
type Dependency struct {
	Name  string
	Check func(context.Context) error
}

type LiveDependencies struct {
	Cache          *redisv9.Client
	Telemetry      *telemetry.Store
	AllowedOrigins []string
}

// NewRouter registers health, identity, inventory, telemetry, and live routes.
func NewRouter(version string, dependencies []Dependency, identities *identity.Store, live ...LiveDependencies) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health/live", healthHandler("ok", version, nil))
	mux.HandleFunc("GET /health/ready", healthHandler("ready", version, dependencies))

	if identities != nil {
		api := &API{identities: identities}
		if len(live) > 0 {
			api.cache = live[0].Cache
			api.telemetry = live[0].Telemetry
			api.allowedLiveOrigins = make(map[string]struct{}, len(live[0].AllowedOrigins))
			for _, origin := range live[0].AllowedOrigins {
				api.allowedLiveOrigins[origin] = struct{}{}
			}
		}
		mux.HandleFunc("POST /v1/auth/register", api.register)
		mux.HandleFunc("POST /v1/auth/login", api.login)
		mux.Handle("GET /v1/auth/me", api.requireAuth(http.HandlerFunc(api.me)))
		mux.Handle("POST /v1/auth/logout", api.requireAuth(http.HandlerFunc(api.logout)))
		mux.Handle("GET /v1/alert-rules", api.requireAuth(http.HandlerFunc(api.listAlertRules)))
		mux.Handle("POST /v1/alert-rules", api.requireAuth(http.HandlerFunc(api.createAlertRule)))
		mux.Handle("DELETE /v1/alert-rules/{ruleID}", api.requireAuth(http.HandlerFunc(api.deleteAlertRule)))
		mux.Handle("GET /v1/alerts", api.requireAuth(http.HandlerFunc(api.listAlertEvents)))
		mux.Handle("GET /v1/notifications", api.requireAuth(http.HandlerFunc(api.listAlertNotifications)))
		mux.Handle("POST /v1/notifications/{notificationID}/read", api.requireAuth(http.HandlerFunc(api.markAlertNotificationRead)))
		mux.Handle("GET /v1/projects", api.requireAuth(http.HandlerFunc(api.listProjects)))
		mux.Handle("POST /v1/projects", api.requireAuth(http.HandlerFunc(api.createProject)))
		mux.Handle("DELETE /v1/projects/{projectID}", api.requireAuth(http.HandlerFunc(api.deleteProject)))
		mux.Handle("GET /v1/servers/{serverID}/project-mappings", api.requireAuth(http.HandlerFunc(api.listServerProjectMappings)))
		mux.Handle("POST /v1/servers/{serverID}/project-mappings", api.requireAuth(http.HandlerFunc(api.createServerProjectMapping)))
		mux.Handle("DELETE /v1/project-mappings/{mappingID}", api.requireAuth(http.HandlerFunc(api.deleteServerProjectMapping)))
		mux.Handle("GET /v1/servers", api.requireAuth(http.HandlerFunc(api.listServers)))
		mux.Handle("POST /v1/servers", api.requireAuth(http.HandlerFunc(api.createServer)))
		mux.Handle("POST /v1/servers/{serverID}/enrollment-token", api.requireAuth(http.HandlerFunc(api.reissueEnrollmentToken)))
		mux.Handle("POST /v1/enrollment-tokens/{tokenID}/revoke", api.requireAuth(http.HandlerFunc(api.revokeEnrollmentToken)))
		mux.Handle("POST /v1/agents/{agentID}/revoke", api.requireAuth(http.HandlerFunc(api.revokeAgent)))

		if api.telemetry != nil {
			mux.Handle("GET /v1/servers/{serverID}/metrics", api.requireAuth(http.HandlerFunc(api.metrics)))
			mux.Handle("GET /v1/servers/{serverID}/metrics/latest", api.requireAuth(http.HandlerFunc(api.latestMetrics)))
			mux.Handle("GET /v1/servers/{serverID}/metrics/latest", api.requireAuth(http.HandlerFunc(api.latestMetrics)))
		}
		if api.cache != nil {
			mux.Handle("POST /v1/live/ticket", api.requireAuth(http.HandlerFunc(api.createLiveTicket)))
			mux.HandleFunc("GET /v1/live/ws", api.liveStream)
		}
	}

	return mux
}
