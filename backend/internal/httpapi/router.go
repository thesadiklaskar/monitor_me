package httpapi

import (
	"context"
	"net/http"
)

// Dependency represents a backend service that must be reachable before the
// API is considered ready to receive application traffic.
type Dependency struct {
	Name  string
	Check func(context.Context) error
}

// NewRouter registers public health endpoints.
// Business APIs are added only after their authorization contracts are defined.
func NewRouter(version string, dependencies ...Dependency) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health/live", healthHandler("ok", version, nil))
	mux.HandleFunc("/health/ready", healthHandler("ready", version, dependencies))

	return mux
}
