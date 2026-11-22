package httpapi

import "net/http"

// NewRouter registers the initial public health endpoints for the control plane.
// Business APIs are added only after their contracts and authorization rules
// have been defined.
func NewRouter(version string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health/live", healthHandler("ok", version))
	mux.HandleFunc("/health/ready", healthHandler("ready", version))

	return mux
}
