package api

import (
	"net/http"
)

// NewServer returns an http.Handler that mounts all cattle-drive API routes.
//
// Routes:
//
//	POST /api/clusters  – list downstream clusters from a kubeconfig
//	POST /api/status    – compare source/target cluster objects
//	POST /api/migrate   – migrate source objects to target cluster
func NewServer() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/clusters", corsMiddleware(handleClusters))
	mux.HandleFunc("/api/status", corsMiddleware(handleStatus))
	mux.HandleFunc("/api/migrate", corsMiddleware(handleMigrate))

	// health-check endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return mux
}

// corsMiddleware adds CORS headers and handles pre-flight OPTIONS requests so
// that the Rancher Dashboard (running on a different origin during development)
// can call the API.
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}
