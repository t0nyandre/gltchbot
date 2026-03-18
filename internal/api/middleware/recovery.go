package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/t0nyandre/gltchbot/internal/api/response"
	"github.com/t0nyandre/gltchbot/internal/logging"
	"log/slog"
)

// Recovery returns a middleware that recovers from panics and logs the error.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = logging.L()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					// Log the panic with stack trace
					logger.Error("panic recovered",
						"panic", rec,
						"stack", string(debug.Stack()),
						"method", r.Method,
						"path", r.URL.Path,
						"remote_addr", r.RemoteAddr,
					)

					// Return 500 Internal Server Error
					response.InternalServerError(w, "internal server error")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}