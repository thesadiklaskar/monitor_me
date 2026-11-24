package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"monitor-me.internal/backend/internal/telemetry"
)

func (api *API) metrics(writer http.ResponseWriter, request *http.Request) {
	queryValues := request.URL.Query()
	from, err := time.Parse(time.RFC3339Nano, queryValues.Get("from"))
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_time_range", "The from parameter must be an RFC3339 timestamp.")
		return
	}
	to, err := time.Parse(time.RFC3339Nano, queryValues.Get("to"))
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_time_range", "The to parameter must be an RFC3339 timestamp.")
		return
	}
	step, err := time.ParseDuration(queryValues.Get("step"))
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_step", "The step parameter must be a Go duration such as 1s or 250ms.")
		return
	}
	limit := 1000
	if raw := queryValues.Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			writeAPIError(writer, http.StatusBadRequest, "invalid_limit", "The limit parameter must be an integer.")
			return
		}
	}
	principal := principalFromRequest(request)
	query := telemetry.RangeQuery{
		OrganizationID: principal.OrganizationID,
		ServerID:       request.PathValue("serverID"),
		MetricName:     queryValues.Get("metric"),
		From:           from,
		To:             to,
		Step:           step,
		Limit:          limit,
	}
	points, err := api.telemetry.QueryRange(request.Context(), query)
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_series_query", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"server_id":  query.ServerID,
		"metric":     query.MetricName,
		"from":       query.From,
		"to":         query.To,
		"step":       query.Step.String(),
		"points":     points,
	})
}

func (api *API) latestMetrics(writer http.ResponseWriter, request *http.Request) {
	principal := principalFromRequest(request)
	query := telemetry.LatestQuery{
		OrganizationID: principal.OrganizationID,
		ServerID:       request.PathValue("serverID"),
	}
	samples, err := api.telemetry.QueryLatest(request.Context(), query)
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_latest_query", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"server_id":      query.ServerID,
		"window_seconds": 900,
		"samples":        samples,
	})
}
