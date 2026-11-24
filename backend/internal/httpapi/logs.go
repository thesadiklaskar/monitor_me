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
	// One lookahead row tells the browser whether another bounded page exists.
	limit := 201
	if raw := values.Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit != 201 {
			writeAPIError(writer, http.StatusBadRequest, "invalid_limit", "The limit parameter must be 201 for bounded keyset pagination.")
			return
		}
	}
	var beforeTime int64
	beforeID := values.Get("before_id")
	if raw := values.Get("before_time"); raw != "" {
		beforeTime, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || beforeTime <= 0 {
			writeAPIError(writer, http.StatusBadRequest, "invalid_cursor", "The before_time parameter must be a positive Unix-microsecond timestamp.")
			return
		}
	} else if beforeID != "" {
		writeAPIError(writer, http.StatusBadRequest, "invalid_cursor", "before_time and before_id must be provided together.")
		return
	}
	if beforeTime > 0 && beforeID == "" {
		writeAPIError(writer, http.StatusBadRequest, "invalid_cursor", "before_time and before_id must be provided together.")
		return
	}
	principal := principalFromRequest(request)
	query := logstore.Query{
		OrganizationID: principal.OrganizationID,
		ServerID: request.PathValue("serverID"),
		ProjectID: values.Get("project_id"),
		From: from, To: to, Search: values.Get("search"),
		Severity: values.Get("severity"), Source: values.Get("source"), Limit: limit,
		BeforeTimeUnixMicros: beforeTime, BeforeEventID: beforeID,
	}
	events, err := api.logs.Query(request.Context(), query)
	switch {
	case errors.Is(err, logstore.ErrInvalidQuery):
		writeAPIError(writer, http.StatusBadRequest, "invalid_log_query", "The log query is invalid or outside the supported time and result limits.")
	case err != nil:
		writeAPIError(writer, http.StatusServiceUnavailable, "log_store_unavailable", "The log store could not complete this search.")
	default:
		hasMore := len(events) > 200
		if hasMore {
			events = events[:200]
		}
		writeJSON(writer, http.StatusOK, map[string]any{
			"server_id": query.ServerID, "project_id": query.ProjectID,
			"from": query.From, "to": query.To, "events": events,
			"count": len(events), "has_more": hasMore,
		})
	}
}
