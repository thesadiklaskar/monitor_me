package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/WorkRCS/monitor_me/backend/internal/identity"
)

func (api *API) createServer(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Name     string `json:"name"`
		Hostname string `json:"hostname"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}

	server, err := api.identities.CreateServer(request.Context(), principalFromRequest(request), input.Name, input.Hostname)
	switch {
	case errors.Is(err, identity.ErrInvalidInput):
		writeAPIError(writer, http.StatusBadRequest, "invalid_input", "Server name or hostname is invalid.")
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Your organization role cannot create servers.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "server_creation_failed", "The server and enrollment token could not be created.")
	default:
		writeJSON(writer, http.StatusCreated, map[string]any{
			"server": map[string]any{
				"id": server.ID, "name": server.Name, "hostname": server.Hostname,
				"status": server.Status, "created_at": server.CreatedAt,
			},
			"enrollment": map[string]any{
				"token_id":   server.EnrollmentTokenID,
				"token":      server.EnrollmentToken,
				"expires_at": server.TokenExpiresAt,
				"protocol":   "monitor.v1",
			},
		})
	}
}

func (api *API) reissueEnrollmentToken(writer http.ResponseWriter, request *http.Request) {
	serverID := strings.TrimSpace(request.PathValue("serverID"))
	issued, err := api.identities.ReissueEnrollmentToken(request.Context(), principalFromRequest(request), serverID)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Your organization role cannot issue enrollment credentials.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The server was not found in this organization.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "token_issue_failed", "A replacement enrollment credential could not be created.")
	default:
		writeJSON(writer, http.StatusCreated, map[string]any{
			"server": map[string]any{"id": issued.ID, "name": issued.Name, "status": issued.Status},
			"enrollment": map[string]any{
				"token_id":   issued.EnrollmentTokenID,
				"token":      issued.EnrollmentToken,
				"expires_at": issued.TokenExpiresAt,
				"protocol": "monitor.v1",
			},
		})
	}
}

func (api *API) listServers(writer http.ResponseWriter, request *http.Request) {
	servers, err := api.identities.ListServers(request.Context(), principalFromRequest(request))
	if err != nil {
		writeAPIError(writer, http.StatusInternalServerError, "server_list_failed", "Server inventory could not be loaded.")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"servers": servers})
}

func (api *API) revokeEnrollmentToken(writer http.ResponseWriter, request *http.Request) {
	tokenID := strings.TrimSpace(request.PathValue("tokenID"))
	err := api.identities.RevokeEnrollmentToken(request.Context(), principalFromRequest(request), tokenID)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Only organization owners and administrators can revoke enrollment tokens.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The unused enrollment token was not found.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "token_revocation_failed", "The enrollment token could not be revoked.")
	default:
		writer.WriteHeader(http.StatusNoContent)
	}
}

func (api *API) revokeAgent(writer http.ResponseWriter, request *http.Request) {
	agentID := strings.TrimSpace(request.PathValue("agentID"))
	err := api.identities.RevokeAgent(request.Context(), principalFromRequest(request), agentID)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Only organization owners and administrators can revoke agents.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The agent was not found in this organization.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "agent_revocation_failed", "The agent could not be revoked.")
	default:
		writer.WriteHeader(http.StatusNoContent)
	}
}
