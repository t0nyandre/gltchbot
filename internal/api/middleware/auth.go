package middleware

import (
	"net/http"

	"github.com/t0nyandre/gltchbot/internal/api/response"
	"github.com/t0nyandre/gltchbot/internal/api/validation"
	"github.com/t0nyandre/gltchbot/internal/audit"
	"github.com/t0nyandre/gltchbot/internal/logging"
)

// APIKey returns a middleware that validates the X-API-Key header.
// Accepts current and old API keys for rotation support.
// Logs authentication attempts to audit log.
func APIKey(validKeys, oldKeys []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")

			// Extract request info for audit logging
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = logging.GenerateRequestID()
			}
			ipAddress := r.RemoteAddr
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				ipAddress = forwarded
			}

			if key == "" {
				// Missing API key
				audit.LogEvent(r.Context(), audit.EventAuthMissingKey, validation.SanitizeLogDetails(map[string]any{
					"request_id": requestID,
					"ip_address": ipAddress,
					"user_agent": r.UserAgent(),
					"method":     r.Method,
					"path":       r.URL.Path,
				}))
				response.Unauthorized(w, "unauthorized")
				return
			}

			// Check against current and old keys
			matched := false
			keySource := ""
			for _, validKey := range validKeys {
				if key == validKey {
					matched = true
					keySource = "current"
					break
				}
			}
			if !matched {
				for _, oldKey := range oldKeys {
					if key == oldKey {
						matched = true
						keySource = "old"
						break
					}
				}
			}
			if !matched {
				// Invalid API key
				audit.LogEvent(r.Context(), audit.EventAuthInvalidKey, validation.SanitizeLogDetails(map[string]any{
					"request_id": requestID,
					"ip_address": ipAddress,
					"user_agent": r.UserAgent(),
					"method":     r.Method,
					"path":       r.URL.Path,
					"api_key":    audit.MaskAPIKey(key),
				}))
				response.Unauthorized(w, "unauthorized")
				return
			}

			// Authentication successful
			details := map[string]any{
				"request_id": requestID,
				"ip_address": ipAddress,
				"user_agent": r.UserAgent(),
				"method":     r.Method,
				"path":       r.URL.Path,
				"api_key":    audit.MaskAPIKey(key),
			}
			if keySource != "" {
				details["key_source"] = keySource
			}
			audit.LogEvent(r.Context(), audit.EventAuthSuccess, validation.SanitizeLogDetails(details))

			next.ServeHTTP(w, r)
		})
	}
}

// MaskAPIKey masks the API key for logging (shows first 4 characters).
func MaskAPIKey(apiKey string) string {
	if len(apiKey) <= 4 {
		return "***"
	}
	return apiKey[:4] + "***"
}
