package httpapi

import (
	"context"
	"net/http"

	"monitor-me.internal/backend/internal/identity"
	"monitor-me.internal/backend/internal/logstore"
	"monitor-me.internal/backend/internal/telemetry"
	redisv9 "github.com/redis/go-redis/v9"
)

// Dependency represents a backend service that must be reachable before the
// API is considered ready to receive application traffic.
type Dependency struct {
	Name  string
	Check func(context.Context) error
}

type LiveDependencies struct {
	Cache                  *redisv9.Client
	Telemetry              *telemetry.Store
	AllowedOrigins         []string
	Logs                   *logstore.Store
}

// NewRouter registers health, installation, inventory, telemetry, and live routes.
func NewRouter(version string, dependencies []Dependency, identities *identity.Store, live ...LiveDependencies) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health/live", healthHandler("ok", version, nil))
	mux.HandleFunc("GET /health/ready", healthHandler("ready", version, dependencies))

	if identities != nil {
		api := &API{identities: identities}
		if len(live) > 0 {
			api.cache = live[0].Cache
			api.telemetry = live[0].Telemetry
			api.logs = live[0].Logs
			api.allowedLiveOrigins = make(map[string]struct{}, len(live[0].AllowedOrigins))
			for _, origin := range live[0].AllowedOrigins {
				api.allowedLiveOrigins[origin] = struct{}{}
			}
		}
		mux.Handle("GET /v1/installation", api.requireInstallation(http.HandlerFunc(api.installation)))
		mux.Handle("GET /v1/alert-rules", api.requireInstallation(http.HandlerFunc(api.listAlertRules)))
		mux.Handle("POST /v1/alert-rules", api.requireInstallation(http.HandlerFunc(api.createAlertRule)))
		mux.Handle("PATCH /v1/alert-rules/{ruleID}", api.requireInstallation(http.HandlerFunc(api.updateAlertRule)))
		mux.Handle("DELETE /v1/alert-rules/{ruleID}", api.requireInstallation(http.HandlerFunc(api.deleteAlertRule)))
		mux.Handle("GET /v1/alerts", api.requireInstallation(http.HandlerFunc(api.listAlertEvents)))
		mux.Handle("GET /v1/notifications", api.requireInstallation(http.HandlerFunc(api.listAlertNotifications)))
		mux.Handle("POST /v1/notifications/{notificationID}/read", api.requireInstallation(http.HandlerFunc(api.markAlertNotificationRead)))
		mux.Handle("GET /v1/dashboards", api.requireInstallation(http.HandlerFunc(api.listDashboards)))
		mux.Handle("POST /v1/dashboards", api.requireInstallation(http.HandlerFunc(api.createDashboard)))
		mux.Handle("PATCH /v1/dashboards/{dashboardID}", api.requireInstallation(http.HandlerFunc(api.updateDashboard)))
		mux.Handle("DELETE /v1/dashboards/{dashboardID}", api.requireInstallation(http.HandlerFunc(api.deleteDashboard)))
		mux.Handle("POST /v1/dashboards/{dashboardID}/panels", api.requireInstallation(http.HandlerFunc(api.createDashboardPanel)))
		mux.Handle("PATCH /v1/dashboard-panels/{panelID}", api.requireInstallation(http.HandlerFunc(api.updateDashboardPanel)))
		mux.Handle("DELETE /v1/dashboard-panels/{panelID}", api.requireInstallation(http.HandlerFunc(api.deleteDashboardPanel)))
		mux.Handle("PATCH /v1/dashboard-panels/{panelID}/position", api.requireInstallation(http.HandlerFunc(api.moveDashboardPanel)))
		mux.Handle("GET /v1/servers", api.requireInstallation(http.HandlerFunc(api.listServers)))
		mux.Handle("POST /v1/servers", api.requireInstallation(http.HandlerFunc(api.createServer)))
		mux.Handle("POST /v1/servers/{serverID}/enrollment-token", api.requireInstallation(http.HandlerFunc(api.reissueEnrollmentToken)))
		mux.Handle("POST /v1/enrollment-tokens/{tokenID}/revoke", api.requireInstallation(http.HandlerFunc(api.revokeEnrollmentToken)))
		mux.Handle("POST /v1/agents/{agentID}/revoke", api.requireInstallation(http.HandlerFunc(api.revokeAgent)))

		if api.logs != nil {
			mux.Handle("GET /v1/servers/{serverID}/logs", api.requireInstallation(http.HandlerFunc(api.logsSearch)))
		}
		if api.telemetry != nil {
			mux.Handle("GET /v1/servers/{serverID}/metrics", api.requireInstallation(http.HandlerFunc(api.metrics)))
			mux.Handle("GET /v1/servers/{serverID}/metrics/latest", api.requireInstallation(http.HandlerFunc(api.latestMetrics)))
		}
		if api.cache != nil {
			mux.Handle("POST /v1/live/ticket", api.requireInstallation(http.HandlerFunc(api.createLiveTicket)))
			mux.HandleFunc("GET /v1/live/ws", api.liveStream)
		}
	}

	return mux
}
