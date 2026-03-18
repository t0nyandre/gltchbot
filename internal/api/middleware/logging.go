package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/t0nyandre/gltchbot/internal/logging"
	"log/slog"
)

// Logging returns a middleware that logs HTTP requests.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = logging.L()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Generate or extract request ID
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = logging.GenerateRequestID()
			}
			
			// Create request-scoped logger
			reqLogger := logger.With(
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)
			
			// Add request ID to response headers
			w.Header().Set("X-Request-ID", requestID)
			
			// Create response writer to capture status code
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			
			// Add logger to request context
			ctx := logging.WithContext(r.Context(), reqLogger)
			r = r.WithContext(ctx)
			
			// Log request start
			reqLogger.Info("request started")
			
			// Process request
			next.ServeHTTP(rw, r)
			
			// Calculate duration
			duration := time.Since(start)
			
			// Log request completion
			reqLogger.Info("request completed",
				"status", rw.statusCode,
				"duration_ms", duration.Milliseconds(),
				"bytes_written", rw.bytesWritten,
			)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code and bytes written.
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
	wroteHeader  bool
}

// WriteHeader captures the status code.
func (rw *responseWriter) WriteHeader(statusCode int) {
	if !rw.wroteHeader {
		rw.statusCode = statusCode
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(statusCode)
	}
}

// Write captures the number of bytes written.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)
	return n, err
}

// RequestLoggerFromContext returns the request-scoped logger from the context.
func RequestLoggerFromContext(ctx context.Context) *slog.Logger {
	return logging.FromContext(ctx)
}