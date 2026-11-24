package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"monitor-me.internal/backend/internal/identity"
)

func (api *API) listDashboards(writer http.ResponseWriter, request *http.Request) {
	dashboards, err := api.identities.ListDashboards(request.Context(), principalFromRequest(request))
	if err != nil {
		writeAPIError(writer, http.StatusInternalServerError, "dashboard_list_failed", "Dashboards could not be loaded.")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"dashboards": dashboards})
}

func (api *API) createDashboard(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ProjectID   string `json:"project_id"`
		IsDefault   bool   `json:"is_default"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	dashboard, err := api.identities.CreateDashboard(
		request.Context(), principalFromRequest(request), input.Name, input.Description, input.ProjectID, input.IsDefault,
	)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Only organization owners and administrators can create dashboards.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The selected project does not belong to this organization.")
	case errors.Is(err, identity.ErrInvalidInput):
		writeAPIError(writer, http.StatusBadRequest, "invalid_dashboard", "Dashboard name, description or project is invalid.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "dashboard_create_failed", "The dashboard could not be created.")
	default:
		writeJSON(writer, http.StatusCreated, map[string]any{"dashboard": dashboard})
	}
}

func (api *API) deleteDashboard(writer http.ResponseWriter, request *http.Request) {
	dashboardID := strings.TrimSpace(request.PathValue("dashboardID"))
	err := api.identities.DeleteDashboard(request.Context(), principalFromRequest(request), dashboardID)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Only organization owners and administrators can delete dashboards.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The dashboard was not found in this organization.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "dashboard_delete_failed", "The dashboard could not be deleted.")
	default:
		writer.WriteHeader(http.StatusNoContent)
	}
}

func (api *API) createDashboardPanel(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Title         string `json:"title"`
		Visualization string `json:"visualization"`
		ServerID      string `json:"server_id"`
		ProjectID     string `json:"project_id"`
		MetricName    string `json:"metric_name"`
		TimeRange     string `json:"time_range"`
		Position      int    `json:"position"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	panel, err := api.identities.CreateDashboardPanel(
		request.Context(), principalFromRequest(request), strings.TrimSpace(request.PathValue("dashboardID")),
		identity.DashboardPanelInput{
			Title: input.Title, Visualization: input.Visualization, ServerID: input.ServerID,
			ProjectID: input.ProjectID, MetricName: input.MetricName, TimeRange: input.TimeRange,
			Position: input.Position,
		},
	)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Only organization owners and administrators can add dashboard panels.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The dashboard, server or project was not found in this organization.")
	case errors.Is(err, identity.ErrInvalidInput):
		writeAPIError(writer, http.StatusBadRequest, "invalid_panel", "The panel configuration is invalid.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "panel_create_failed", "The dashboard panel could not be created.")
	default:
		writeJSON(writer, http.StatusCreated, map[string]any{"panel": panel})
	}
}

func (api *API) deleteDashboardPanel(writer http.ResponseWriter, request *http.Request) {
	panelID := strings.TrimSpace(request.PathValue("panelID"))
	err := api.identities.DeleteDashboardPanel(request.Context(), principalFromRequest(request), panelID)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Only organization owners and administrators can remove dashboard panels.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The dashboard panel was not found in this organization.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "panel_delete_failed", "The dashboard panel could not be deleted.")
	default:
		writer.WriteHeader(http.StatusNoContent)
	}
}
