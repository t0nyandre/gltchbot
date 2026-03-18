package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/t0nyandre/gltchbot/internal/api/response"
	"github.com/t0nyandre/gltchbot/internal/api/validation"
	"github.com/t0nyandre/gltchbot/internal/logging"
)

// SizeLimit returns a middleware that limits the size of incoming request bodies.
// If maxBytes <= 0, no limit is applied.
func SizeLimit(maxBytes int64) func(http.Handler) http.Handler {
	logger := logging.L()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip limiting for requests with no body
			if r.Body == nil || r.ContentLength == 0 {
				next.ServeHTTP(w, r)
				return
			}

			// If maxBytes <= 0, no limit
			if maxBytes <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			// Early rejection based on Content-Length header
			if cl := r.ContentLength; cl > 0 && cl > maxBytes {
				requestID := r.Header.Get("X-Request-ID")
				logger.Warn("request body too large by Content-Length",
					"request_id", validation.SanitizeForLog(requestID),
					"method", r.Method,
					"path", validation.SanitizeForLog(r.URL.Path),
					"content_length", cl,
					"max_bytes", maxBytes,
				)
				// Close the request body to free resources
				r.Body.Close()
				response.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}

			// Read the body up to maxBytes+1 to detect overflow
			limitedReader := io.LimitReader(r.Body, maxBytes+1)
			data, err := io.ReadAll(limitedReader)
			if err != nil {
				// Read error (e.g., connection closed)
				logger.Error("failed to read request body", "error", err)
				r.Body.Close()
				response.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			// Close the original body (we have consumed it)
			r.Body.Close()

			// Check if the body exceeds the limit
			if int64(len(data)) > maxBytes {
				requestID := r.Header.Get("X-Request-ID")
				logger.Warn("request body too large",
					"request_id", validation.SanitizeForLog(requestID),
					"method", r.Method,
					"path", validation.SanitizeForLog(r.URL.Path),
					"body_size", len(data),
					"max_bytes", maxBytes,
				)
				response.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}

			// Replace the body with a reader over the buffered data
			r.Body = io.NopCloser(bytes.NewReader(data))

			next.ServeHTTP(w, r)
		})
	}
}
