package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	tests := []struct {
		name              string
		hstsMaxAge        int
		csp               string
		permissionsPolicy string
		expectedHeaders   map[string]string
	}{
		{
			name:              "default headers",
			hstsMaxAge:        31536000,
			csp:               "",
			permissionsPolicy: "",
			expectedHeaders: map[string]string{
				"Strict-Transport-Security": "max-age=31536000",
				"X-Frame-Options":           "DENY",
				"X-Content-Type-Options":    "nosniff",
				"X-XSS-Protection":          "0",
				"Referrer-Policy":           "strict-origin-when-cross-origin",
				"Permissions-Policy":        "camera=(), microphone=(), geolocation=()",
				"Content-Security-Policy":   "default-src 'self'; style-src 'self' 'unsafe-inline'",
			},
		},
		{
			name:              "zero HSTS max-age omits header",
			hstsMaxAge:        0,
			csp:               "",
			permissionsPolicy: "",
			expectedHeaders: map[string]string{
				"X-Frame-Options":         "DENY",
				"X-Content-Type-Options":  "nosniff",
				"X-XSS-Protection":        "0",
				"Referrer-Policy":         "strict-origin-when-cross-origin",
				"Permissions-Policy":      "camera=(), microphone=(), geolocation=()",
				"Content-Security-Policy": "default-src 'self'; style-src 'self' 'unsafe-inline'",
			},
		},
		{
			name:              "custom CSP and Permissions-Policy",
			hstsMaxAge:        3600,
			csp:               "default-src 'none'",
			permissionsPolicy: "camera=(self), microphone=()",
			expectedHeaders: map[string]string{
				"Strict-Transport-Security": "max-age=3600",
				"X-Frame-Options":           "DENY",
				"X-Content-Type-Options":    "nosniff",
				"X-XSS-Protection":          "0",
				"Referrer-Policy":           "strict-origin-when-cross-origin",
				"Permissions-Policy":        "camera=(self), microphone=()",
				"Content-Security-Policy":   "default-src 'none'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := Security(tt.hstsMaxAge, tt.csp, tt.permissionsPolicy)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}),
			)

			req := httptest.NewRequest("GET", "/", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			for key, expected := range tt.expectedHeaders {
				got := rr.Header().Get(key)
				if got != expected {
					t.Errorf("header %s = %q, want %q", key, got, expected)
				}
			}

			// Ensure HSTS header is absent when max-age <= 0
			if tt.hstsMaxAge <= 0 {
				if hsts := rr.Header().Get("Strict-Transport-Security"); hsts != "" {
					t.Errorf("expected no HSTS header when max-age <= 0, got %q", hsts)
				}
			}
		})
	}
}
