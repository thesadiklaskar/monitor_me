package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"monitor-me.internal/backend/internal/identity"
)

func (api *API) remoteOperationConfig(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{
		"allowed_systemd_units": api.allowedRemoteUnits,
		"operations":            []string{"restart_service", "stop_service"},
	})
}

func (api *API) createRemoteOperation(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Operation string `json:"operation"`
		Target    string `json:"target"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	serverID := strings.TrimSpace(request.PathValue("serverID"))
	operation, err := api.identities.CreateRemoteOperation(
		request.Context(), principalFromRequest(request), serverID,
		input.Operation, input.Target, api.allowedRemoteUnits,
	)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Only organization owners and administrators can run remote operations.")
	case errors.Is(err, identity.ErrInvalidInput):
		writeAPIError(writer, http.StatusBadRequest, "remote_operation_not_allowed", "The operation or target is not in the control-plane systemd allowlist.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusConflict, "server_or_agent_unavailable", "The server must be online with a recent authenticated agent heartbeat.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "remote_operation_failed", "The remote operation could not be queued.")
	default:
		writeJSON(writer, http.StatusAccepted, map[string]any{"operation": operation})
	}
}

func (api *API) listRemoteOperations(writer http.ResponseWriter, request *http.Request) {
	principal := principalFromRequest(request)
	operations, err := api.identities.ListRemoteOperations(request.Context(), principal)
	if err != nil {
		writeAPIError(writer, http.StatusInternalServerError, "remote_operations_failed", "Remote operation history could not be loaded.")
		return
	}
	audit, err := api.identities.ListRemoteOperationAudit(request.Context(), principal)
	if err != nil {
		writeAPIError(writer, http.StatusInternalServerError, "remote_audit_failed", "Remote operation audit history could not be loaded.")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"operations": operations, "audit": audit})
}

// listRemoteOperationAudit returns the organization-scoped audit history.
func (api *API) listRemoteOperationAudit(writer http.ResponseWriter, request *http.Request) {
	audit, err := api.identities.ListRemoteOperationAudit(request.Context(), principalFromRequest(request))
	if err != nil {
		writeAPIError(writer, http.StatusInternalServerError, "remote_audit_failed", "Remote operation audit history could not be loaded.")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"audit": audit})
}
