package middleware

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCORS(t *testing.T) {
	tests := []struct {
		name               string
		allowedOrigins     string
		allowedMethods     string
		allowedHeaders     string
		exposedHeaders     string
		maxAge             int
		allowCredentials   bool
		requestOrigin      string
		requestMethod      string
		requestHeaders     map[string]string
		expectedHeaders    map[string]string
		expectedStatusCode int
		expectNextHandler  bool
	}{
		// No Origin header => no CORS headers
		{
			name:              "no origin header",
			allowedOrigins:    "*",
			requestOrigin:     "",
			requestMethod:     "GET",
			expectedHeaders:   map[string]string{},
			expectNextHandler: true,
		},
		// Wildcard origin allowed (empty allowedOrigins)
		{
			name:               "wildcard empty origins",
			allowedOrigins:     "",
			requestOrigin:      "https://example.com",
			requestMethod:      "GET",
			expectedHeaders:    map[string]string{"Access-Control-Allow-Origin": "https://example.com"},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  true,
		},
		// Wildcard origin allowed (single "*")
		{
			name:               "wildcard star origins",
			allowedOrigins:     "*",
			requestOrigin:      "https://example.com",
			requestMethod:      "GET",
			expectedHeaders:    map[string]string{"Access-Control-Allow-Origin": "https://example.com"},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  true,
		},
		// Specific origin allowed
		{
			name:               "specific origin allowed",
			allowedOrigins:     "https://example.com,https://foo.com",
			requestOrigin:      "https://example.com",
			requestMethod:      "GET",
			expectedHeaders:    map[string]string{"Access-Control-Allow-Origin": "https://example.com"},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  true,
		},
		// Multiple origins, second matches
		{
			name:               "multiple origins second matches",
			allowedOrigins:     "https://foo.com,https://example.com",
			requestOrigin:      "https://example.com",
			requestMethod:      "GET",
			expectedHeaders:    map[string]string{"Access-Control-Allow-Origin": "https://example.com"},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  true,
		},
		// Origin not allowed => no CORS headers
		{
			name:               "origin not allowed",
			allowedOrigins:     "https://allowed.com",
			requestOrigin:      "https://evil.com",
			requestMethod:      "GET",
			expectedHeaders:    map[string]string{},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  true,
		},
		// Allowed origin with credentials
		{
			name:             "allow credentials",
			allowedOrigins:   "https://example.com",
			allowCredentials: true,
			requestOrigin:    "https://example.com",
			requestMethod:    "GET",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":      "https://example.com",
				"Access-Control-Allow-Credentials": "true",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  true,
		},
		// Preflight request with default methods/headers
		{
			name:           "preflight default",
			allowedOrigins: "*",
			requestOrigin:  "https://example.com",
			requestMethod:  "OPTIONS",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, X-API-Key, Authorization",
				"Access-Control-Max-Age":       "86400",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  false,
		},
		// Preflight with custom allowed methods
		{
			name:           "preflight custom methods",
			allowedOrigins: "*",
			allowedMethods: "GET, POST",
			requestOrigin:  "https://example.com",
			requestMethod:  "OPTIONS",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "GET, POST",
				"Access-Control-Allow-Headers": "Content-Type, X-API-Key, Authorization",
				"Access-Control-Max-Age":       "86400",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  false,
		},
		// Preflight with custom allowed headers
		{
			name:           "preflight custom headers",
			allowedOrigins: "*",
			allowedHeaders: "X-Custom, X-Other",
			requestOrigin:  "https://example.com",
			requestMethod:  "OPTIONS",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
				"Access-Control-Allow-Headers": "X-Custom, X-Other",
				"Access-Control-Max-Age":       "86400",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  false,
		},
		// Preflight with exposed headers (should also be set in preflight per implementation)
		{
			name:           "preflight exposed headers",
			allowedOrigins: "*",
			exposedHeaders: "X-Exposed, X-Other",
			requestOrigin:  "https://example.com",
			requestMethod:  "OPTIONS",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":   "https://example.com",
				"Access-Control-Allow-Methods":  "GET, POST, PUT, PATCH, DELETE, OPTIONS",
				"Access-Control-Allow-Headers":  "Content-Type, X-API-Key, Authorization",
				"Access-Control-Expose-Headers": "X-Exposed, X-Other",
				"Access-Control-Max-Age":        "86400",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  false,
		},
		// Preflight with max age > 0
		{
			name:           "preflight custom max age",
			allowedOrigins: "*",
			maxAge:         3600,
			requestOrigin:  "https://example.com",
			requestMethod:  "OPTIONS",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, X-API-Key, Authorization",
				"Access-Control-Max-Age":       "3600",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  false,
		},
		// Preflight with max age <= 0 uses default
		{
			name:           "preflight zero max age uses default",
			allowedOrigins: "*",
			maxAge:         0,
			requestOrigin:  "https://example.com",
			requestMethod:  "OPTIONS",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, X-API-Key, Authorization",
				"Access-Control-Max-Age":       "86400",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  false,
		},
		// Regular request with exposed headers
		{
			name:           "regular request exposed headers",
			allowedOrigins: "*",
			exposedHeaders: "X-Exposed",
			requestOrigin:  "https://example.com",
			requestMethod:  "GET",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":   "https://example.com",
				"Access-Control-Expose-Headers": "X-Exposed",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  true,
		},
		// Regular request without exposed headers
		{
			name:           "regular request no exposed headers",
			allowedOrigins: "*",
			exposedHeaders: "",
			requestOrigin:  "https://example.com",
			requestMethod:  "GET",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin": "https://example.com",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  true,
		},
		// Origin with spaces in allowed list
		{
			name:           "origins with spaces",
			allowedOrigins: "https://example.com, https://foo.com",
			requestOrigin:  "https://foo.com",
			requestMethod:  "GET",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin": "https://foo.com",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  true,
		},
		// Empty allowedMethods and allowedHeaders strings (should default)
		{
			name:           "empty methods and headers defaults",
			allowedOrigins: "*",
			allowedMethods: "",
			allowedHeaders: "",
			requestOrigin:  "https://example.com",
			requestMethod:  "OPTIONS",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, X-API-Key, Authorization",
				"Access-Control-Max-Age":       "86400",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  false,
		},
		// new test cases start here
		{
			name:               "origin not allowed with OPTIONS",
			allowedOrigins:     "https://allowed.com",
			requestOrigin:      "https://evil.com",
			requestMethod:      "OPTIONS",
			expectedHeaders:    map[string]string{},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  true,
		},
		{
			name:             "preflight with credentials",
			allowedOrigins:   "https://example.com",
			allowCredentials: true,
			requestOrigin:    "https://example.com",
			requestMethod:    "OPTIONS",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":      "https://example.com",
				"Access-Control-Allow-Credentials": "true",
				"Access-Control-Allow-Methods":     "GET, POST, PUT, PATCH, DELETE, OPTIONS",
				"Access-Control-Allow-Headers":     "Content-Type, X-API-Key, Authorization",
				"Access-Control-Max-Age":           "86400",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  false,
		},
		{
			name:           "preflight exposed headers empty string",
			allowedOrigins: "*",
			exposedHeaders: "",
			requestOrigin:  "https://example.com",
			requestMethod:  "OPTIONS",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, X-API-Key, Authorization",
				"Access-Control-Max-Age":       "86400",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  false,
		},
		{
			name:           "preflight exposed headers empty parse list",
			allowedOrigins: "*",
			exposedHeaders: ",,",
			requestOrigin:  "https://example.com",
			requestMethod:  "OPTIONS",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, X-API-Key, Authorization",
				"Access-Control-Max-Age":       "86400",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  false,
		},
		{
			name:           "preflight max age negative",
			allowedOrigins: "*",
			maxAge:         -1,
			requestOrigin:  "https://example.com",
			requestMethod:  "OPTIONS",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, X-API-Key, Authorization",
				"Access-Control-Max-Age":       "86400",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  false,
		},
		{
			name:               "allowed origins star with spaces",
			allowedOrigins:     " * ",
			requestOrigin:      "https://example.com",
			requestMethod:      "GET",
			expectedHeaders:    map[string]string{"Access-Control-Allow-Origin": "https://example.com"},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  true,
		},
		{
			name:               "allowed origins empty parse list wildcard",
			allowedOrigins:     ",,",
			requestOrigin:      "https://example.com",
			requestMethod:      "GET",
			expectedHeaders:    map[string]string{"Access-Control-Allow-Origin": "https://example.com"},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  true,
		},
		{
			name:           "allowed methods empty parse list defaults",
			allowedOrigins: "*",
			allowedMethods: ",,",
			requestOrigin:  "https://example.com",
			requestMethod:  "OPTIONS",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, X-API-Key, Authorization",
				"Access-Control-Max-Age":       "86400",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  false,
		},
		{
			name:           "allowed headers empty parse list defaults",
			allowedOrigins: "*",
			allowedHeaders: ",,",
			requestOrigin:  "https://example.com",
			requestMethod:  "OPTIONS",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, X-API-Key, Authorization",
				"Access-Control-Max-Age":       "86400",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  false,
		},
		{
			name:               "regular request exposed headers empty parse list",
			allowedOrigins:     "*",
			exposedHeaders:     ",,",
			requestOrigin:      "https://example.com",
			requestMethod:      "GET",
			expectedHeaders:    map[string]string{"Access-Control-Allow-Origin": "https://example.com"},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  true,
		},
		{
			name:             "regular request credentials with wildcard origins",
			allowedOrigins:   "*",
			allowCredentials: true,
			requestOrigin:    "https://example.com",
			requestMethod:    "GET",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":      "https://example.com",
				"Access-Control-Allow-Credentials": "true",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  true,
		},
		{
			name:             "preflight with wildcard origins and credentials",
			allowedOrigins:   "*",
			allowCredentials: true,
			requestOrigin:    "https://example.com",
			requestMethod:    "OPTIONS",
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":      "https://example.com",
				"Access-Control-Allow-Credentials": "true",
				"Access-Control-Allow-Methods":     "GET, POST, PUT, PATCH, DELETE, OPTIONS",
				"Access-Control-Allow-Headers":     "Content-Type, X-API-Key, Authorization",
				"Access-Control-Max-Age":           "86400",
			},
			expectedStatusCode: http.StatusOK,
			expectNextHandler:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled := false
			handler := CORS(
				tt.allowedOrigins,
				tt.allowedMethods,
				tt.allowedHeaders,
				tt.exposedHeaders,
				tt.maxAge,
				tt.allowCredentials,
			)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(tt.requestMethod, "/", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}
			for k, v := range tt.requestHeaders {
				req.Header.Set(k, v)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			// Check status code
			if tt.expectedStatusCode != 0 && rr.Code != tt.expectedStatusCode {
				t.Errorf("status code = %d, want %d", rr.Code, tt.expectedStatusCode)
			}

			// Check handler invocation
			if handlerCalled != tt.expectNextHandler {
				t.Errorf("next handler called = %v, want %v", handlerCalled, tt.expectNextHandler)
			}

			// Check expected headers presence
			for key, expected := range tt.expectedHeaders {
				got := rr.Header().Get(key)
				if got != expected {
					t.Errorf("header %s = %q, want %q", key, got, expected)
				}
			}

			// Ensure no extra CORS headers beyond expected
			// We'll check a known set of CORS headers
			corsHeaders := []string{
				"Access-Control-Allow-Origin",
				"Access-Control-Allow-Credentials",
				"Access-Control-Allow-Methods",
				"Access-Control-Allow-Headers",
				"Access-Control-Expose-Headers",
				"Access-Control-Max-Age",
			}
			for _, key := range corsHeaders {
				if _, expected := tt.expectedHeaders[key]; !expected {
					if val := rr.Header().Get(key); val != "" {
						t.Errorf("unexpected CORS header %s = %q", key, val)
					}
				}
			}
		})
	}
}

func TestParseList(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{}},
		{"*", []string{"*"}},
		{"GET,POST", []string{"GET", "POST"}},
		{"GET, POST", []string{"GET", "POST"}},
		{"GET , POST , PUT", []string{"GET", "POST", "PUT"}},
		{"GET,", []string{"GET"}},
		{",GET", []string{"GET"}},
		{"GET,,POST", []string{"GET", "POST"}},
		{"   GET   ,   POST   ", []string{"GET", "POST"}},
		{"single", []string{"single"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseList(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseList(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
