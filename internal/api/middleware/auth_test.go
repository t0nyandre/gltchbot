package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/t0nyandre/gltchbot/internal/audit"
)

// mockAuditor implements audit.Auditor and records logged events.
type mockAuditor struct {
	events []auditEvent
}

type auditEvent struct {
	ctx       context.Context
	eventType audit.EventType
	details   map[string]any
}

func (m *mockAuditor) Log(ctx context.Context, event audit.Event) {
	// Convert event to details similar to what LogEvent does
	details := make(map[string]any)
	if event.Details != nil {
		details = event.Details
	}
	m.events = append(m.events, auditEvent{
		ctx:       ctx,
		eventType: event.Type,
		details:   details,
	})
}

func (m *mockAuditor) LogEvent(ctx context.Context, eventType audit.EventType, details map[string]any) {
	m.events = append(m.events, auditEvent{
		ctx:       ctx,
		eventType: eventType,
		details:   details,
	})
}

func (m *mockAuditor) reset() {
	m.events = nil
}

func (m *mockAuditor) lastEvent() *auditEvent {
	if len(m.events) == 0 {
		return nil
	}
	return &m.events[len(m.events)-1]
}

// TestAPIKeyAuthentication tests the API key authentication middleware with various scenarios.
func TestAPIKeyAuthentication(t *testing.T) {
	validKeys := []string{"current-key-123", "current-key-456"}
	oldKeys := []string{"old-key-789"}

	tests := []struct {
		name            string
		apiKeyHeader    string
		requestID       string
		forwardedFor    string
		userAgent       string
		method          string
		path            string
		wantStatusCode  int
		wantEventType   audit.EventType
		wantKeySource   string   // "current", "old", or ""
		wantLogContains []string // substrings expected in log details
	}{
		{
			name:            "missing API key",
			apiKeyHeader:    "",
			requestID:       "req-missing",
			forwardedFor:    "10.0.0.1",
			userAgent:       "TestAgent",
			method:          "GET",
			path:            "/api/test",
			wantStatusCode:  http.StatusUnauthorized,
			wantEventType:   audit.EventAuthMissingKey,
			wantLogContains: []string{"request_id", "ip_address", "user_agent", "method", "path"},
		},
		{
			name:            "invalid API key",
			apiKeyHeader:    "invalid-key",
			requestID:       "req-invalid",
			forwardedFor:    "",
			userAgent:       "TestAgent2",
			method:          "POST",
			path:            "/api/other",
			wantStatusCode:  http.StatusUnauthorized,
			wantEventType:   audit.EventAuthInvalidKey,
			wantLogContains: []string{"request_id", "ip_address", "user_agent", "method", "path", "api_key"},
		},
		{
			name:            "valid current key",
			apiKeyHeader:    "current-key-123",
			requestID:       "req-valid",
			forwardedFor:    "192.168.1.1",
			userAgent:       "TestAgent3",
			method:          "PUT",
			path:            "/api/resource",
			wantStatusCode:  http.StatusOK,
			wantEventType:   audit.EventAuthSuccess,
			wantKeySource:   "current",
			wantLogContains: []string{"request_id", "ip_address", "user_agent", "method", "path", "api_key", "key_source"},
		},
		{
			name:            "valid old key",
			apiKeyHeader:    "old-key-789",
			requestID:       "req-old",
			forwardedFor:    "",
			userAgent:       "TestAgent4",
			method:          "DELETE",
			path:            "/api/old",
			wantStatusCode:  http.StatusOK,
			wantEventType:   audit.EventAuthSuccess,
			wantKeySource:   "old",
			wantLogContains: []string{"request_id", "ip_address", "user_agent", "method", "path", "api_key", "key_source"},
		},
		{
			name:            "valid second current key",
			apiKeyHeader:    "current-key-456",
			requestID:       "",
			forwardedFor:    "203.0.113.5",
			userAgent:       "TestAgent5",
			method:          "GET",
			path:            "/",
			wantStatusCode:  http.StatusOK,
			wantEventType:   audit.EventAuthSuccess,
			wantKeySource:   "current",
			wantLogContains: []string{"request_id", "ip_address", "user_agent", "method", "path", "api_key", "key_source"},
		},
		{
			name:            "valid key with X-Forwarded-For",
			apiKeyHeader:    "current-key-123",
			requestID:       "req-forward",
			forwardedFor:    "10.1.2.3, 10.2.3.4",
			userAgent:       "ForwardedAgent",
			method:          "GET",
			path:            "/forward",
			wantStatusCode:  http.StatusOK,
			wantEventType:   audit.EventAuthSuccess,
			wantKeySource:   "current",
			wantLogContains: []string{"request_id", "ip_address", "user_agent", "method", "path", "api_key", "key_source"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAud := &mockAuditor{}
			ctx := audit.WithAuditor(context.Background(), mockAud)

			// Create request with context containing our mock auditor
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req = req.WithContext(ctx)
			if tt.apiKeyHeader != "" {
				req.Header.Set("X-API-Key", tt.apiKeyHeader)
			}
			if tt.requestID != "" {
				req.Header.Set("X-Request-ID", tt.requestID)
			}
			if tt.forwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.forwardedFor)
			}
			if tt.userAgent != "" {
				req.Header.Set("User-Agent", tt.userAgent)
			}
			// Set RemoteAddr to a known value for IP extraction
			req.RemoteAddr = "192.168.0.1:12345"

			rr := httptest.NewRecorder()
			handlerCalled := false
			handler := APIKey(validKeys, oldKeys)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(rr, req)

			// Verify response status code
			if rr.Code != tt.wantStatusCode {
				t.Errorf("status code = %d, want %d", rr.Code, tt.wantStatusCode)
			}

			// Verify handler was called only when authorized
			expectedHandlerCalled := tt.wantStatusCode == http.StatusOK
			if handlerCalled != expectedHandlerCalled {
				t.Errorf("handler called = %v, want %v", handlerCalled, expectedHandlerCalled)
			}

			// Verify audit events
			if len(mockAud.events) != 1 {
				t.Errorf("expected exactly 1 audit event, got %d", len(mockAud.events))
				// Continue to examine the first event if present
			}
			if len(mockAud.events) > 0 {
				event := mockAud.events[0]
				if event.eventType != tt.wantEventType {
					t.Errorf("event type = %q, want %q", event.eventType, tt.wantEventType)
				}
				// Check details contain required fields
				for _, key := range tt.wantLogContains {
					if _, ok := event.details[key]; !ok {
						t.Errorf("missing expected detail key %q in %v", key, event.details)
					}
				}
				// Verify request ID
				if tt.requestID != "" {
					if got, ok := event.details["request_id"].(string); !ok || got != tt.requestID {
						t.Errorf("request_id = %v, want %s", event.details["request_id"], tt.requestID)
					}
				} else {
					// request ID should be generated
					if got, ok := event.details["request_id"].(string); !ok || got == "" {
						t.Error("request_id missing or empty")
					}
				}
				// Verify IP address
				expectedIP := tt.forwardedFor
				if expectedIP == "" {
					expectedIP = req.RemoteAddr // middleware uses RemoteAddr if no X-Forwarded-For
				}
				if got, ok := event.details["ip_address"].(string); !ok || got != expectedIP {
					t.Errorf("ip_address = %v, want %s", event.details["ip_address"], expectedIP)
				}
				// Verify user agent
				if got, ok := event.details["user_agent"].(string); !ok || got != tt.userAgent {
					t.Errorf("user_agent = %v, want %s", event.details["user_agent"], tt.userAgent)
				}
				// Verify method
				if got, ok := event.details["method"].(string); !ok || got != tt.method {
					t.Errorf("method = %v, want %s", event.details["method"], tt.method)
				}
				// Verify path
				if got, ok := event.details["path"].(string); !ok || got != tt.path {
					t.Errorf("path = %v, want %s", event.details["path"], tt.path)
				}
				// Verify API key masking (if present)
				if tt.apiKeyHeader != "" && tt.wantEventType != audit.EventAuthMissingKey {
					if got, ok := event.details["api_key"].(string); !ok {
						t.Error("api_key missing from details")
					} else {
						// Should be masked (first 4 chars + ***)
						if len(tt.apiKeyHeader) > 4 {
							wantMasked := tt.apiKeyHeader[:4] + "***"
							if got != wantMasked {
								t.Errorf("api_key masked = %q, want %q", got, wantMasked)
							}
						} else {
							if got != "***" {
								t.Errorf("api_key masked = %q, want ***", got)
							}
						}
					}
				}
				// Verify key_source for successful auth
				if tt.wantKeySource != "" {
					if got, ok := event.details["key_source"].(string); !ok || got != tt.wantKeySource {
						t.Errorf("key_source = %v, want %s", event.details["key_source"], tt.wantKeySource)
					}
				}
			}
		})
	}
}

// TestAPIKeyEdgeCases tests edge cases not covered by the main table.
func TestAPIKeyEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		validKeys      []string
		oldKeys        []string
		apiKeyHeader   string
		wantStatusCode int
		wantEventType  audit.EventType
		expectHandler  bool
	}{
		{
			name:           "no valid keys, old key present",
			validKeys:      nil,
			oldKeys:        []string{"old-key"},
			apiKeyHeader:   "old-key",
			wantStatusCode: http.StatusOK,
			wantEventType:  audit.EventAuthSuccess,
			expectHandler:  true,
		},
		{
			name:           "no valid keys, invalid key",
			validKeys:      []string{},
			oldKeys:        []string{},
			apiKeyHeader:   "any-key",
			wantStatusCode: http.StatusUnauthorized,
			wantEventType:  audit.EventAuthInvalidKey,
			expectHandler:  false,
		},
		{
			name:           "duplicate keys in valid list",
			validKeys:      []string{"key1", "key1"},
			oldKeys:        nil,
			apiKeyHeader:   "key1",
			wantStatusCode: http.StatusOK,
			wantEventType:  audit.EventAuthSuccess,
			expectHandler:  true,
		},
		{
			name:           "key matches both valid and old (should be valid)",
			validKeys:      []string{"shared-key"},
			oldKeys:        []string{"shared-key"},
			apiKeyHeader:   "shared-key",
			wantStatusCode: http.StatusOK,
			wantEventType:  audit.EventAuthSuccess,
			expectHandler:  true,
		},
		{
			name:           "empty API key with empty valid keys",
			validKeys:      []string{},
			oldKeys:        []string{},
			apiKeyHeader:   "",
			wantStatusCode: http.StatusUnauthorized,
			wantEventType:  audit.EventAuthMissingKey,
			expectHandler:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAud := &mockAuditor{}
			ctx := audit.WithAuditor(context.Background(), mockAud)
			req := httptest.NewRequest("GET", "/", nil)
			req = req.WithContext(ctx)
			if tt.apiKeyHeader != "" {
				req.Header.Set("X-API-Key", tt.apiKeyHeader)
			}
			rr := httptest.NewRecorder()
			handlerCalled := false
			handler := APIKey(tt.validKeys, tt.oldKeys)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatusCode {
				t.Errorf("status code = %d, want %d", rr.Code, tt.wantStatusCode)
			}
			if handlerCalled != tt.expectHandler {
				t.Errorf("handler called = %v, want %v", handlerCalled, tt.expectHandler)
			}
			if len(mockAud.events) != 1 {
				t.Errorf("expected exactly 1 audit event, got %d", len(mockAud.events))
			} else {
				if mockAud.events[0].eventType != tt.wantEventType {
					t.Errorf("event type = %q, want %q", mockAud.events[0].eventType, tt.wantEventType)
				}
			}
		})
	}
}

// TestAPIKeyMasking tests the MaskAPIKey function directly.
func TestAPIKeyMasking(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "***"},
		{"a", "***"},
		{"ab", "***"},
		{"abc", "***"},
		{"abcd", "***"},
		{"abcde", "abcd***"},
		{"abcdefgh", "abcd***"},
		{"secretapikey", "secr***"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := MaskAPIKey(tt.input)
			if got != tt.expected {
				t.Errorf("MaskAPIKey(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestAPIKeyMultipleEvents ensures that only one audit event is logged per request.
func TestAPIKeyMultipleEvents(t *testing.T) {
	mockAud := &mockAuditor{}
	ctx := audit.WithAuditor(context.Background(), mockAud)

	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(ctx)
	req.Header.Set("X-API-Key", "current-key-123")
	req.Header.Set("X-Request-ID", "req-multi")

	rr := httptest.NewRecorder()
	handler := APIKey([]string{"current-key-123"}, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if len(mockAud.events) != 1 {
		t.Errorf("expected exactly 1 audit event, got %d", len(mockAud.events))
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rr.Code, http.StatusOK)
	}
}
