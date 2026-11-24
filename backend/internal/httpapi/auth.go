package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"example.com/monitor-me/backend/internal/identity"
	"example.com/monitor-me/backend/internal/logstore"
	"example.com/monitor-me/backend/internal/telemetry"
	redisv9 "github.com/redis/go-redis/v9"
)

type API struct {
	identities          *identity.Store
	cache               *redisv9.Client
	telemetry           *telemetry.Store
	logs                *logstore.Store
	allowedLiveOrigins  map[string]struct{}
	allowedRemoteUnits  []string
	localPrincipalMu    sync.Mutex
	localPrincipal      identity.Principal
	localPrincipalExpiresAt time.Time
}

type principalContextKey struct{}

func (api *API) installationPrincipal(ctx context.Context) (identity.Principal, error) {
	api.localPrincipalMu.Lock()
	defer api.localPrincipalMu.Unlock()

	if api.localPrincipal.SessionID != "" && time.Until(api.localPrincipalExpiresAt) > time.Minute {
		return api.localPrincipal, nil
	}

	principal, expiresAt, err := api.identities.CreateInstallationPrincipal(ctx)
	if err != nil {
		return identity.Principal{}, err
	}
	api.localPrincipal = principal
	api.localPrincipalExpiresAt = expiresAt
	return principal, nil
}

// installation reports the local self-hosted installation, not a user or organization.
func (api *API) installation(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{
		"installation": map[string]string{
			"name": "Monitor Me",
			"mode": "single-install",
		},
	})
}

// requireInstallation binds requests to the one implicit installation principal. It does
// not accept browser credentials; remote access is restricted at the network edge.
func (api *API) requireInstallation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		principal, err := api.installationPrincipal(request.Context())
		if err != nil {
			writeAPIError(writer, http.StatusServiceUnavailable, "installation_unavailable", "The Monitor Me installation could not be initialized.")
			return
		}
		ctx := context.WithValue(request.Context(), principalContextKey{}, principal)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func principalFromRequest(request *http.Request) identity.Principal {
	principal, _ := request.Context().Value(principalContextKey{}).(identity.Principal)
	return principal
}

func decodeJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAPIError(writer, http.StatusBadRequest, "invalid_json", "The request body must contain one JSON object.")
		return false
	}
	return true
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeAPIError(writer http.ResponseWriter, status int, code, message string) {
	writeJSON(writer, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
