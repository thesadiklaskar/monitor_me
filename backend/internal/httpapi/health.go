package httpapi

import (
	"encoding/json"
	"net/http"
	"time"
)

type healthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	CheckedAt string `json:"checked_at"`
}

func healthHandler(status, version string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writer.Header().Set("Allow", http.MethodGet)
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(http.StatusOK)

		response := healthResponse{
			Status:    status,
			Service:   "monitor-me-api",
			Version:   version,
			CheckedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}

		// Health responses are intentionally small and contain no infrastructure secrets.
		_ = json.NewEncoder(writer).Encode(response)
	}
}
