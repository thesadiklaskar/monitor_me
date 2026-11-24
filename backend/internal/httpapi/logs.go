package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"example.com/monitor-me/backend/internal/logstore"
)

func (api *API) logsSearch(writer http.ResponseWriter, request *http.Request) {
	values := request.URL.Query()
	from, err := time.Parse(time.RFC3339Nano, values.Get("from"))
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_time_range", "The from parameter must be an RFC3339 timestamp.")
		return
	}
	to, err := time.Parse(time.RFC3339Nano, values.Get("to"))
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_time_range", "The to parameter must be an RFC3339 timestamp.")
		return
	}
	limit := 100
	if raw := values.Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			writeAPIError(writer, http.StatusBadRequest, "invalid_limit", "The limit parameter must be an integer.")
			return
		}
	}
	principal := principalFromRequest(request)
	query := logstore.Query{
		OrganizationID: principal.OrganizationID,
		ServerID: request.PathValue("serverID"),
		ProjectID: values.Get("project_id"),
		From: from, To: to, Search: values.Get("search"),
		Severity: values.Get("severity"), Source: values.Get("source"), Limit: limit,
	}
	events, err := api.logs.Query(request.Context(), query)
	switch {
	case errors.Is(err, logstore.ErrInvalidQuery):
		writeAPIError(writer, http.StatusBadRequest, "invalid_log_query", "The log query is invalid or outside the supported time and result limits.")
	case err != nil:
		writeAPIError(writer, http.StatusServiceUnavailable, "log_store_unavailable", "The log store could not complete this search.")
	default:
		writeJSON(writer, http.StatusOK, map[string]any{
			"server_id": query.ServerID, "project_id": query.ProjectID,
			"from": query.From, "to": query.To, "events": events, "count": len(events),
		})
	}
}
