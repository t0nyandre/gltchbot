package middleware

import (
	"net/http"
	"strconv"

	"github.com/t0nyandre/gltchbot/internal/logging"
)

// Security returns a middleware that adds security headers to HTTP responses.
// If csp is empty, a default CSP will be used.
// If permissionsPolicy is empty, a default policy will be used.
func Security(hstsMaxAge int, csp string, permissionsPolicy string) func(http.Handler) http.Handler {
	logger := logging.L()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Strict‑Transport‑Security
			if hstsMaxAge > 0 {
				w.Header().Set("Strict-Transport-Security", "max-age="+strconv.Itoa(hstsMaxAge))
			}

			// X‑Frame‑Options
			w.Header().Set("X-Frame-Options", "DENY")

			// X‑Content‑Type‑Options
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// X‑XSS‑Protection (disabled, rely on CSP)
			w.Header().Set("X-XSS-Protection", "0")

			// Referrer‑Policy
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Permissions‑Policy
			if permissionsPolicy == "" {
				permissionsPolicy = "camera=(), microphone=(), geolocation=()"
			}
			w.Header().Set("Permissions-Policy", permissionsPolicy)

			// Content‑Security‑Policy
			if csp == "" {
				csp = "default-src 'self'; style-src 'self' 'unsafe-inline'"
			}
			w.Header().Set("Content-Security-Policy", csp)

			// Log at debug level
			logger.Debug("security headers added",
				"hsts_max_age", hstsMaxAge,
				"csp_configured", csp != "",
				"permissions_policy_configured", permissionsPolicy != "",
			)

			next.ServeHTTP(w, r)
		})
	}
}
