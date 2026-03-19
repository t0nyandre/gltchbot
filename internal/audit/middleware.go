package audit

import (
	"context"
	"net/http"

	"github.com/t0nyandre/gltchbot/internal/logging"
)

// Middleware returns an HTTP middleware that adds request information and an auditor to the context.
// This middleware should be placed after authentication middleware so that the API key is available.
func Middleware(auditor Auditor) func(http.Handler) http.Handler {
	if auditor == nil {
		auditor = New(nil)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract request ID from header or generate
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = logging.GenerateRequestID()
			}

			// Extract API key from header (masked)
			apiKey := r.Header.Get("X-API-Key")

			// Get client IP (respect X-Forwarded-For if behind proxy)
			ipAddress := r.RemoteAddr
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				ipAddress = forwarded
			}

			// Store request info in context
			info := RequestInfo{
				RequestID: requestID,
				APIKey:    apiKey,
				IPAddress: ipAddress,
				UserAgent: r.UserAgent(),
				Method:    r.Method,
				Path:      r.URL.Path,
			}
			ctx := WithRequestInfo(r.Context(), info)

			// Add auditor to context
			ctx = WithAuditor(ctx, auditor)

			// Add request ID to response header
			w.Header().Set("X-Request-ID", requestID)

			// Call next handler with enriched context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// auditorKey is the key used to store the auditor in context.
type auditorKey struct{}

// WithAuditor adds the auditor to the context.
func WithAuditor(ctx context.Context, auditor Auditor) context.Context {
	return context.WithValue(ctx, auditorKey{}, auditor)
}

// FromContext returns the auditor from the context, or nil if not found.
func FromContext(ctx context.Context) Auditor {
	if auditor, ok := ctx.Value(auditorKey{}).(Auditor); ok {
		return auditor
	}
	return nil
}

// LogEvent logs an audit event using the auditor from the context.
// If no auditor is found, the event is logged using the default audit logger.
func LogEvent(ctx context.Context, eventType EventType, details map[string]any) {
	auditor := FromContext(ctx)
	if auditor == nil {
		auditor = New(nil)
	}
	auditor.LogEvent(ctx, eventType, details)
}
