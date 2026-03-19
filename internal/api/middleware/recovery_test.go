package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecovery_NormalRequest(t *testing.T) {
	logger, getLogs := captureLogger()
	middleware := Recovery(logger)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	middlewareHandler := middleware(handler)

	req := makeRequest("GET", "/test", "192.168.1.1:12345", nil)
	rr := httptest.NewRecorder()
	middlewareHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("response status = %d, want %d", rr.Code, http.StatusOK)
	}
	if body := rr.Body.String(); body != "OK" {
		t.Errorf("response body = %q, want %q", body, "OK")
	}
	// No panic logs should be generated
	logs := getLogs()
	if len(logs) > 0 {
		t.Errorf("expected no logs, got %d logs", len(logs))
	}
}

func TestRecovery_PanicRecovery(t *testing.T) {
	tests := []struct {
		name       string
		panicValue any
		wantPanic  any
	}{
		{
			name:       "string panic",
			panicValue: "something went wrong",
			wantPanic:  "something went wrong",
		},
		{
			name:       "error panic",
			panicValue: http.ErrAbortHandler,
			wantPanic:  http.ErrAbortHandler.Error(),
		},
		{
			name:       "nil panic",
			panicValue: nil,
			wantPanic:  "panic called with nil argument",
		},
		{
			name:       "int panic",
			panicValue: 42,
			wantPanic:  float64(42),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, getLogs := captureLogger()
			middleware := Recovery(logger)
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic(tt.panicValue)
			})
			middlewareHandler := middleware(handler)

			req := makeRequest("POST", "/api/users", "10.0.0.1:8080", nil)
			rr := httptest.NewRecorder()
			// Should not panic
			middlewareHandler.ServeHTTP(rr, req)

			// Verify response is 500 Internal Server Error
			if rr.Code != http.StatusInternalServerError {
				t.Errorf("response status = %d, want %d", rr.Code, http.StatusInternalServerError)
			}
			// Verify JSON error response
			var resp struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if resp.Error.Code != "internal_server_error" {
				t.Errorf("error code = %q, want %q", resp.Error.Code, "internal_server_error")
			}
			if resp.Error.Message != "internal server error" {
				t.Errorf("error message = %q, want %q", resp.Error.Message, "internal server error")
			}

			// Verify logs
			logs := getLogs()
			if len(logs) != 1 {
				t.Fatalf("expected exactly 1 log entry, got %d", len(logs))
			}
			entry := logs[0]
			if msg, ok := entry["msg"].(string); !ok || msg != "panic recovered" {
				t.Errorf("log msg = %v, want 'panic recovered'", msg)
			}
			panicVal, ok := entry["panic"]
			if !ok {
				t.Errorf("log missing panic field")
			} else {
				// Compare values, considering JSON number conversion
				switch v := panicVal.(type) {
				case string:
					if tt.wantPanic == nil {
						t.Errorf("log panic = %q, want nil", v)
					} else if want, ok := tt.wantPanic.(string); !ok || v != want {
						t.Errorf("log panic = %q, want %v", v, tt.wantPanic)
					}
				case float64:
					if want, ok := tt.wantPanic.(float64); !ok || v != want {
						t.Errorf("log panic = %v, want %v", v, tt.wantPanic)
					}
				case nil:
					if tt.wantPanic != nil {
						t.Errorf("log panic = nil, want %v", tt.wantPanic)
					}
				default:
					t.Errorf("unexpected panic field type: %T", v)
				}
			}
			if stack, ok := entry["stack"].(string); !ok || stack == "" {
				t.Errorf("log stack missing or empty")
			}
			if method, ok := entry["method"].(string); !ok || method != "POST" {
				t.Errorf("log method = %v, want 'POST'", method)
			}
			if path, ok := entry["path"].(string); !ok || path != "/api/users" {
				t.Errorf("log path = %v, want '/api/users'", path)
			}
			if remote, ok := entry["remote_addr"].(string); !ok || remote != "10.0.0.1:8080" {
				t.Errorf("log remote_addr = %v, want '10.0.0.1:8080'", remote)
			}
		})
	}
}

func TestRecovery_LoggerInjection(t *testing.T) {
	// Test with nil logger (should use default logger)
	t.Run("nil logger", func(t *testing.T) {
		middleware := Recovery(nil)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		})
		middlewareHandler := middleware(handler)

		req := makeRequest("GET", "/", "127.0.0.1:80", nil)
		rr := httptest.NewRecorder()
		// Should not panic, default logger should be used
		middlewareHandler.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("response status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	// Test with provided logger (should use provided logger)
	t.Run("provided logger", func(t *testing.T) {
		logger, getLogs := captureLogger()
		middleware := Recovery(logger)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		})
		middlewareHandler := middleware(handler)

		req := makeRequest("GET", "/", "127.0.0.1:80", nil)
		rr := httptest.NewRecorder()
		middlewareHandler.ServeHTTP(rr, req)

		logs := getLogs()
		if len(logs) != 1 {
			t.Errorf("expected exactly 1 log entry, got %d", len(logs))
		}
	})
}

func TestRecovery_StackTracePresent(t *testing.T) {
	logger, getLogs := captureLogger()
	middleware := Recovery(logger)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("stack trace test")
	})
	middlewareHandler := middleware(handler)

	req := makeRequest("GET", "/", "127.0.0.1:80", nil)
	rr := httptest.NewRecorder()
	middlewareHandler.ServeHTTP(rr, req)

	logs := getLogs()
	if len(logs) != 1 {
		t.Fatalf("expected exactly 1 log entry, got %d", len(logs))
	}
	entry := logs[0]
	stack, ok := entry["stack"].(string)
	if !ok {
		t.Fatal("stack field missing from log entry")
	}
	// Stack trace should contain goroutine and file info
	if !strings.Contains(stack, "goroutine") {
		t.Errorf("stack trace missing goroutine info: %s", stack[:100])
	}
	if !strings.Contains(stack, ".go") {
		t.Errorf("stack trace missing .go file reference: %s", stack[:100])
	}
}
