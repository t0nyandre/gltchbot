package middleware

import (
	"net/http"

	"github.com/t0nyandre/gltchbot/internal/api/response"
)

// APIKey returns a middleware that validates the X-API-Key header.
func APIKey(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" || key != apiKey {
				response.Unauthorized(w, "unauthorized")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
