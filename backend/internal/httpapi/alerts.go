package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"example.com/monitor-me/backend/internal/identity"
)

func (api *API) listAlertRules(writer http.ResponseWriter, request *http.Request) {
	rules, err := api.identities.ListAlertRules(request.Context(), principalFromRequest(request))
	if err != nil {
		writeAPIError(writer, http.StatusInternalServerError, "alert_rules_failed", "Alert rules could not be loaded.")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"rules": rules})
}

func (api *API) createAlertRule(writer http.ResponseWriter, request *http.Request) {
	var input identity.AlertRule
	if !decodeJSON(writer, request, &input) {
		return
	}
	rule, err := api.identities.CreateAlertRule(request.Context(), principalFromRequest(request), input)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Only organization owners and administrators can manage alert rules.")
	case errors.Is(err, identity.ErrInvalidInput):
		writeAPIError(writer, http.StatusBadRequest, "invalid_alert_rule", "The alert rule fields are invalid.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "alert_scope_not_found", "The selected server or project was not found in this organization.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "alert_rule_create_failed", "The alert rule could not be created.")
	default:
		writeJSON(writer, http.StatusCreated, map[string]any{"rule": rule})
	}
}

func (api *API) deleteAlertRule(writer http.ResponseWriter, request *http.Request) {
	ruleID := strings.TrimSpace(request.PathValue("ruleID"))
	err := api.identities.DeleteAlertRule(request.Context(), principalFromRequest(request), ruleID)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Only organization owners and administrators can delete alert rules.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The alert rule was not found.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "alert_rule_delete_failed", "The alert rule could not be deleted.")
	default:
		writer.WriteHeader(http.StatusNoContent)
	}
}

func (api *API) listAlertEvents(writer http.ResponseWriter, request *http.Request) {
	events, err := api.identities.ListAlertEvents(request.Context(), principalFromRequest(request))
	if err != nil {
		writeAPIError(writer, http.StatusInternalServerError, "alerts_failed", "Alert events could not be loaded.")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"events": events})
}

func (api *API) listAlertNotifications(writer http.ResponseWriter, request *http.Request) {
	notifications, err := api.identities.ListAlertNotifications(request.Context(), principalFromRequest(request))
	if err != nil {
		writeAPIError(writer, http.StatusInternalServerError, "notifications_failed", "Notifications could not be loaded.")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"notifications": notifications})
}

func (api *API) markAlertNotificationRead(writer http.ResponseWriter, request *http.Request) {
	id := strings.TrimSpace(request.PathValue("notificationID"))
	err := api.identities.MarkAlertNotificationRead(request.Context(), principalFromRequest(request), id)
	switch {
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The notification was not found.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "notification_update_failed", "The notification could not be updated.")
	default:
		writer.WriteHeader(http.StatusNoContent)
	}
}
