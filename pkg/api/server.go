package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// ServerOptions configures the HTTP API server.
type ServerOptions struct {
	// APIToken, if non-empty, requires every non-OPTIONS request to carry
	// "Authorization: Bearer <token>". The /healthz endpoint is exempt.
	APIToken string
}

// NewServer returns an http.Handler that mounts all cattle-drive API routes.
//
// Routes:
//
//	POST /api/clusters  – list downstream clusters from a kubeconfig
//	POST /api/status    – compare source/target cluster objects
//	POST /api/migrate   – migrate source objects to target cluster
//	GET  /healthz       – health check (no auth required)
func NewServer(opts ServerOptions) http.Handler {
	mux := http.NewServeMux()

	// wrap applies CORS and, when a token is configured, Bearer-token auth.
	// Ordering: CORS is outermost so that pre-flight OPTIONS responses always
	// carry the right headers even when auth rejects the actual request.
	wrap := func(h http.HandlerFunc) http.HandlerFunc {
		if opts.APIToken != "" {
			h = bearerAuth(opts.APIToken, h)
		}
		return corsMiddleware(h)
	}

	mux.HandleFunc("/api/clusters", wrap(handleClusters))
	mux.HandleFunc("/api/status", wrap(handleStatus))
	mux.HandleFunc("/api/migrate", wrap(handleMigrate))

	// Health-check: no auth required.
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

// bearerAuth returns middleware that enforces "Authorization: Bearer <token>"
// on every request. Pre-flight OPTIONS requests bypass the check so that CORS
// works correctly from browser clients. The comparison is performed in constant
// time to prevent timing-based token oracle attacks.
func bearerAuth(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Let CORS pre-flights through – the browser sends these without auth.
		if r.Method == http.MethodOptions {
			next(w, r)
			return
		}
		const prefix = "Bearer "
		auth := r.Header.Get("Authorization")
		// Extract only the token portion after confirming the prefix is present.
		var provided string
		if strings.HasPrefix(auth, prefix) {
			provided = auth[len(prefix):]
		}
		// Constant-time comparison prevents timing-based token-oracle attacks.
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized: missing or invalid token")
			return
		}
		next(w, r)
	}
}
