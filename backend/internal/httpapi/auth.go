package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"example.com/monitor-me/backend/internal/identity"
	"example.com/monitor-me/backend/internal/telemetry"
	redisv9 "github.com/redis/go-redis/v9"
)

type API struct {
	identities                *identity.Store
	cache                     *redisv9.Client
	telemetry                 *telemetry.Store
	allowedLiveOrigins        map[string]struct{}
	allowedRemoteUnits        []string
	disableAuthentication     bool
	localPrincipalMu          sync.Mutex
	localPrincipal            identity.Principal
	localPrincipalExpiresAt   time.Time
}

type principalContextKey struct{}

func (api *API) anonymousLocalPrincipal(ctx context.Context) (identity.Principal, error) {
	api.localPrincipalMu.Lock()
	defer api.localPrincipalMu.Unlock()

	if api.localPrincipal.SessionID != "" && time.Until(api.localPrincipalExpiresAt) > time.Minute {
		return api.localPrincipal, nil
	}

	principal, expiresAt, err := api.identities.CreateLocalPrincipal(ctx)
	if err != nil {
		return identity.Principal{}, err
	}
	api.localPrincipal = principal
	api.localPrincipalExpiresAt = expiresAt
	return principal, nil
}

func (api *API) authenticationDisabled(writer http.ResponseWriter, request *http.Request) {
	writeAPIError(writer, http.StatusNotFound, "authentication_disabled", "Interactive login is disabled in local anonymous mode.")
}

func (api *API) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if api.disableAuthentication {
			principal, err := api.anonymousLocalPrincipal(request.Context())
			if err != nil {
				writeAPIError(writer, http.StatusServiceUnavailable, "local_workspace_unavailable", "The local workspace could not be initialized.")
				return
			}
			ctx := context.WithValue(request.Context(), principalContextKey{}, principal)
			next.ServeHTTP(writer, request.WithContext(ctx))
			return
		}
		header := request.Header.Get("Authorization")
		scheme, token, ok := strings.Cut(header, " ")
		if !ok || scheme != "Bearer" || token == "" || strings.ContainsAny(token, " \t\r\n") {
			writeAPIError(writer, http.StatusUnauthorized, "unauthorized", "A valid bearer token is required.")
			return
		}

		principal, err := api.identities.Authenticate(request.Context(), token)
		if err != nil {
			writeAPIError(writer, http.StatusUnauthorized, "unauthorized", "The bearer token is invalid, expired, or revoked.")
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

func (api *API) register(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Email            string `json:"email"`
		DisplayName      string `json:"display_name"`
		Password         string `json:"password"`
		OrganizationName string `json:"organization_name"`
		OrganizationSlug string `json:"organization_slug"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}

	user, err := api.identities.RegisterFirst(request.Context(), identity.RegisterInput{
		Email: input.Email, DisplayName: input.DisplayName, Password: input.Password,
		OrganizationName: input.OrganizationName, OrganizationSlug: input.OrganizationSlug,
	})
	switch {
	case errors.Is(err, identity.ErrInvalidInput):
		writeAPIError(writer, http.StatusBadRequest, "invalid_input", "Registration fields are invalid.")
	case errors.Is(err, identity.ErrBootstrapComplete):
		writeAPIError(writer, http.StatusForbidden, "bootstrap_complete", "Initial registration is disabled because an account already exists.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "registration_failed", "The initial account could not be created.")
	default:
		writeJSON(writer, http.StatusCreated, map[string]any{"user": user})
	}
}

func (api *API) login(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}

	session, err := api.identities.Login(request.Context(), input.Email, input.Password)
	if err != nil {
		if errors.Is(err, identity.ErrInvalidCredentials) {
			writeAPIError(writer, http.StatusUnauthorized, "invalid_credentials", "Email or password is incorrect.")
			return
		}
		writeAPIError(writer, http.StatusInternalServerError, "login_failed", "The session could not be created.")
		return
	}

	writeJSON(writer, http.StatusOK, map[string]any{
		"access_token": session.AccessToken,
		"token_type":   "Bearer",
		"expires_at":   session.ExpiresAt,
		"user":         session.User,
	})
}

func (api *API) me(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{"user": principalFromRequest(request)})
}

func (api *API) logout(writer http.ResponseWriter, request *http.Request) {
	if api.disableAuthentication {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	if err := api.identities.Logout(request.Context(), principalFromRequest(request)); err != nil {
		writeAPIError(writer, http.StatusInternalServerError, "logout_failed", "The session could not be revoked.")
		return
	}
	writer.WriteHeader(http.StatusNoContent)
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
