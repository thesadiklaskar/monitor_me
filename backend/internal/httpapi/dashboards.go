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
		Name             string  `json:"name"`
		Description      string  `json:"description"`
		IsDefault        bool    `json:"is_default"`
		DefaultServerID  *string `json:"default_server_id"`
		DefaultTimeRange string  `json:"default_time_range"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	dashboard, err := api.identities.CreateDashboard(
		request.Context(), principalFromRequest(request), identity.DashboardInput{
			Name: input.Name, Description: input.Description, IsDefault: input.IsDefault,
			DefaultServerID: input.DefaultServerID, DefaultTimeRange: input.DefaultTimeRange,
		},
	)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Dashboard changes are not permitted.")
	case errors.Is(err, identity.ErrInvalidInput):
		writeAPIError(writer, http.StatusBadRequest, "invalid_dashboard", "Dashboard name or description is invalid.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The selected machine was not found.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "dashboard_create_failed", "The dashboard could not be created.")
	default:
		writeJSON(writer, http.StatusCreated, map[string]any{"dashboard": dashboard})
	}
}

func (api *API) updateDashboard(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Name             string  `json:"name"`
		Description      string  `json:"description"`
		IsDefault        bool    `json:"is_default"`
		DefaultServerID  *string `json:"default_server_id"`
		DefaultTimeRange string  `json:"default_time_range"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	dashboard, err := api.identities.UpdateDashboard(
		request.Context(), principalFromRequest(request), strings.TrimSpace(request.PathValue("dashboardID")),
		identity.DashboardInput{
			Name: input.Name, Description: input.Description, IsDefault: input.IsDefault,
			DefaultServerID: input.DefaultServerID, DefaultTimeRange: input.DefaultTimeRange,
		},
	)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Dashboard changes are not permitted.")
	case errors.Is(err, identity.ErrInvalidInput):
		writeAPIError(writer, http.StatusBadRequest, "invalid_dashboard", "Dashboard details or saved filters are invalid.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The dashboard or selected machine was not found.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "dashboard_update_failed", "The dashboard could not be updated.")
	default:
		writeJSON(writer, http.StatusOK, map[string]any{"dashboard": dashboard})
	}
}

func (api *API) deleteDashboard(writer http.ResponseWriter, request *http.Request) {
	dashboardID := strings.TrimSpace(request.PathValue("dashboardID"))
	err := api.identities.DeleteDashboard(request.Context(), principalFromRequest(request), dashboardID)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Dashboard changes are not permitted.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The dashboard was not found.")
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
		MetricName    string `json:"metric_name"`
		TimeRange     string `json:"time_range"`
		Position      int    `json:"position"`
		GridWidth     int    `json:"grid_width"`
		LabelKey      string `json:"label_key"`
		LabelValue    string `json:"label_value"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	panel, err := api.identities.CreateDashboardPanel(
		request.Context(), principalFromRequest(request), strings.TrimSpace(request.PathValue("dashboardID")),
		identity.DashboardPanelInput{
			Title: input.Title, Visualization: input.Visualization, ServerID: input.ServerID,
			MetricName: input.MetricName, TimeRange: input.TimeRange, Position: input.Position, GridWidth: input.GridWidth,
			LabelKey: input.LabelKey, LabelValue: input.LabelValue,
		},
	)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Dashboard changes are not permitted.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The dashboard or machine was not found.")
	case errors.Is(err, identity.ErrInvalidInput):
		writeAPIError(writer, http.StatusBadRequest, "invalid_panel", "The panel configuration is invalid.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "panel_create_failed", "The dashboard panel could not be created.")
	default:
		writeJSON(writer, http.StatusCreated, map[string]any{"panel": panel})
	}
}

func (api *API) updateDashboardPanel(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Title         string `json:"title"`
		Visualization string `json:"visualization"`
		ServerID      string `json:"server_id"`
		MetricName    string `json:"metric_name"`
		TimeRange     string `json:"time_range"`
		GridWidth     int    `json:"grid_width"`
		LabelKey      string `json:"label_key"`
		LabelValue    string `json:"label_value"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	panel, err := api.identities.UpdateDashboardPanel(
		request.Context(), principalFromRequest(request), strings.TrimSpace(request.PathValue("panelID")),
		identity.DashboardPanelInput{
			Title: input.Title, Visualization: input.Visualization, ServerID: input.ServerID,
			MetricName: input.MetricName, TimeRange: input.TimeRange, GridWidth: input.GridWidth,
			LabelKey: input.LabelKey, LabelValue: input.LabelValue,
		},
	)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Dashboard changes are not permitted.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The dashboard panel or selected machine was not found.")
	case errors.Is(err, identity.ErrInvalidInput):
		writeAPIError(writer, http.StatusBadRequest, "invalid_panel", "The panel configuration is invalid.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "panel_update_failed", "The dashboard panel could not be updated.")
	default:
		writeJSON(writer, http.StatusOK, map[string]any{"panel": panel})
	}
}

func (api *API) deleteDashboardPanel(writer http.ResponseWriter, request *http.Request) {
	panelID := strings.TrimSpace(request.PathValue("panelID"))
	err := api.identities.DeleteDashboardPanel(request.Context(), principalFromRequest(request), panelID)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Dashboard changes are not permitted.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The dashboard panel was not found.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "panel_delete_failed", "The dashboard panel could not be deleted.")
	default:
		writer.WriteHeader(http.StatusNoContent)
	}
}

func (api *API) moveDashboardPanel(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Position int `json:"position"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	err := api.identities.MoveDashboardPanel(
		request.Context(),
		principalFromRequest(request),
		strings.TrimSpace(request.PathValue("panelID")),
		input.Position,
	)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Dashboard changes are not permitted.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The dashboard panel no longer exists.")
	case errors.Is(err, identity.ErrInvalidInput):
		writeAPIError(writer, http.StatusBadRequest, "invalid_position", "The requested panel position is outside the dashboard.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "panel_reorder_failed", "The dashboard panel could not be moved.")
	default:
		writer.WriteHeader(http.StatusNoContent)
	}
}
