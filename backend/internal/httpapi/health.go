package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type healthResponse struct {
	Status       string            `json:"status"`
	Service      string            `json:"service"`
	Version      string            `json:"version"`
	CheckedAt    string            `json:"checked_at"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

func healthHandler(status, version string, dependencies []Dependency) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writer.Header().Set("Allow", http.MethodGet)
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		response := healthResponse{
			Status:    status,
			Service:   "monitor-me-api",
			Version:   version,
			CheckedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}

		statusCode := http.StatusOK
		if status == "ready" {
			response.Dependencies = make(map[string]string, len(dependencies))
			for _, dependency := range dependencies {
				dependencyStatus := "ok"
				checkContext, cancel := context.WithTimeout(request.Context(), 2*time.Second)
				err := dependency.Check(checkContext)
				cancel()

				if err != nil {
					dependencyStatus = "unavailable"
					response.Status = "not_ready"
					statusCode = http.StatusServiceUnavailable
				}
				response.Dependencies[dependency.Name] = dependencyStatus
			}
		}

		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(statusCode)

		// Health responses expose dependency state, never connection strings or driver errors.
		_ = json.NewEncoder(writer).Encode(response)
	}
}
