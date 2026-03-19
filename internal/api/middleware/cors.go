package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/t0nyandre/gltchbot/internal/logging"
)

// CORS returns a middleware that adds CORS headers to HTTP responses.
// If allowedOrigins is empty or "*", all origins are allowed (wildcard).
// If allowedMethods is empty, default methods are used.
// If allowedHeaders is empty, default headers are used.
// If exposedHeaders is empty, no headers are exposed.
// If maxAge <= 0, default max age is used (86400 seconds).
// If allowCredentials is true, Access-Control-Allow-Credentials header is set.
func CORS(
	allowedOrigins string,
	allowedMethods string,
	allowedHeaders string,
	exposedHeaders string,
	maxAge int,
	allowCredentials bool,
) func(http.Handler) http.Handler {
	logger := logging.L()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				// Not a CORS request
				next.ServeHTTP(w, r)
				return
			}

			// Parse allowed origins
			origins := parseList(allowedOrigins)
			allowed := false
			if len(origins) == 0 || (len(origins) == 1 && origins[0] == "*") {
				// Any origin allowed (wildcard)
				allowed = true
			} else {
				for _, o := range origins {
					if o == origin {
						allowed = true
						break
					}
				}
			}

			if !allowed {
				// Origin not allowed, skip CORS headers
				logger.Debug("CORS origin not allowed",
					"origin", origin,
					"allowed_origins", allowedOrigins,
				)
				next.ServeHTTP(w, r)
				return
			}

			// Origin allowed: set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", origin)
			if allowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				// Parse allowed methods
				methods := parseList(allowedMethods)
				if len(methods) == 0 {
					methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
				}
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))

				// Parse allowed headers
				headers := parseList(allowedHeaders)
				if len(headers) == 0 {
					headers = []string{"Content-Type", "X-API-Key", "Authorization"}
				}
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(headers, ", "))

				// Exposed headers
				if exposedHeaders != "" {
					exposed := parseList(exposedHeaders)
					if len(exposed) > 0 {
						w.Header().Set("Access-Control-Expose-Headers", strings.Join(exposed, ", "))
					}
				}

				// Max age
				preflightMaxAge := maxAge
				if preflightMaxAge <= 0 {
					preflightMaxAge = 86400
				}
				w.Header().Set("Access-Control-Max-Age", strconv.Itoa(preflightMaxAge))

				// Respond to preflight with 200 OK
				w.WriteHeader(http.StatusOK)
				return
			}

			// Regular request: add exposed headers if any
			if exposedHeaders != "" {
				exposed := parseList(exposedHeaders)
				if len(exposed) > 0 {
					w.Header().Set("Access-Control-Expose-Headers", strings.Join(exposed, ", "))
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// parseList splits a comma-separated string into trimmed non-empty parts.
func parseList(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
