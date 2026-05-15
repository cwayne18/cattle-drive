package api

import (
	"crypto/subtle"
	"net/http"
	"slices"
	"strings"
)

// ServerOptions configures the HTTP API server.
type ServerOptions struct {
	// APIToken, if non-empty, requires every non-OPTIONS request to carry
	// "Authorization: Bearer <token>". The /healthz endpoint is exempt.
	APIToken string
	// DefaultKubeconfig is used when request bodies omit "kubeconfig".
	DefaultKubeconfig string
	// AllowedOrigins controls which cross-origin browser requests receive CORS
	// response headers. Leave empty to rely on same-origin Rancher proxy calls.
	AllowedOrigins []string
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
		return corsMiddleware(opts.AllowedOrigins, h)
	}

	mux.HandleFunc("/api/clusters", wrap(func(w http.ResponseWriter, r *http.Request) {
		handleClustersWithDefault(opts.DefaultKubeconfig, w, r)
	}))
	mux.HandleFunc("/api/status", wrap(func(w http.ResponseWriter, r *http.Request) {
		handleStatusWithDefault(opts.DefaultKubeconfig, w, r)
	}))
	mux.HandleFunc("/api/migrate", wrap(func(w http.ResponseWriter, r *http.Request) {
		handleMigrateWithDefault(opts.DefaultKubeconfig, w, r)
	}))

	// Health-check: no auth required.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return mux
}

// corsMiddleware adds CORS headers for explicitly allowed origins and handles
// pre-flight OPTIONS requests. When no origins are configured the API relies on
// same-origin Rancher proxy calls and emits no Access-Control-Allow-Origin
// header, which keeps the default deployment closed to arbitrary browsers.
func corsMiddleware(allowedOrigins []string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Add("Vary", "Origin")
		}
		if allowOrigin(origin, allowedOrigins) {
			if slices.Contains(allowedOrigins, "*") {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func allowOrigin(origin string, allowedOrigins []string) bool {
	if origin == "" || len(allowedOrigins) == 0 {
		return false
	}
	return slices.Contains(allowedOrigins, "*") || slices.Contains(allowedOrigins, origin)
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
