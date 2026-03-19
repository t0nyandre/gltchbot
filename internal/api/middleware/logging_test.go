package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/t0nyandre/gltchbot/internal/api/validation"
)

// loggingHandler is a simple handler that records the number of times it's called.
type loggingHandler struct {
	calls int
	delay time.Duration // optional delay to simulate processing time
}

func (h *loggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.calls++
	if h.delay > 0 {
		time.Sleep(h.delay)
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// captureLogger creates a logger that writes JSON logs to a buffer.
// It returns the logger and a function that returns the captured log lines.
func captureLogger() (*slog.Logger, func() []map[string]any) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	return logger, func() []map[string]any {
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		entries := make([]map[string]any, 0, len(lines))
		for _, line := range lines {
			if line == "" {
				continue
			}
			var entry map[string]any
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				// If unmarshal fails, ignore the line (should not happen)
				continue
			}
			entries = append(entries, entry)
		}
		return entries
	}
}

func TestLogging_RequestIDGeneration(t *testing.T) {
	tests := []struct {
		name          string
		requestID     string // value of X-Request-ID header in request
		wantGenerated bool   // true if a new request ID should be generated
	}{
		{
			name:          "no X-Request-ID header",
			requestID:     "",
			wantGenerated: true,
		},
		{
			name:          "existing X-Request-ID header",
			requestID:     "custom-req-123",
			wantGenerated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, getLogs := captureLogger()
			middleware := Logging(logger)
			handler := &loggingHandler{}
			middlewareHandler := middleware(handler)

			headers := make(map[string]string)
			if tt.requestID != "" {
				headers["X-Request-ID"] = tt.requestID
			}
			req := makeRequest("GET", "/test", "192.168.1.1:12345", headers)
			rr := httptest.NewRecorder()
			middlewareHandler.ServeHTTP(rr, req)

			// Verify response header contains request ID
			respID := rr.Header().Get("X-Request-ID")
			if respID == "" {
				t.Error("response missing X-Request-ID header")
			}
			if tt.requestID != "" && respID != tt.requestID {
				t.Errorf("response X-Request-ID = %q, want %q", respID, tt.requestID)
			}
			if tt.wantGenerated && respID == "" {
				t.Error("expected generated request ID but got empty")
			}

			// Verify logs contain request_id field
			logs := getLogs()
			if len(logs) < 2 {
				t.Fatalf("expected at least 2 log entries, got %d", len(logs))
			}
			for _, entry := range logs {
				if id, ok := entry["request_id"].(string); !ok || id == "" {
					t.Errorf("log entry missing request_id: %v", entry)
				} else if tt.requestID != "" && id != tt.requestID {
					t.Errorf("log request_id = %q, want %q", id, tt.requestID)
				}
			}
		})
	}
}

func TestLogging_RequestStartAndComplete(t *testing.T) {
	logger, getLogs := captureLogger()
	middleware := Logging(logger)
	handler := &loggingHandler{delay: 10 * time.Millisecond}
	middlewareHandler := middleware(handler)

	req := makeRequest("POST", "/api/users", "10.0.0.1:8080", nil)
	rr := httptest.NewRecorder()
	middlewareHandler.ServeHTTP(rr, req)

	logs := getLogs()
	if len(logs) != 2 {
		t.Fatalf("expected exactly 2 log entries (start and complete), got %d", len(logs))
	}

	// First log should be "request started"
	start := logs[0]
	if msg, ok := start["msg"].(string); !ok || msg != "request started" {
		t.Errorf("first log msg = %v, want 'request started'", msg)
	}
	// Second log should be "request completed"
	complete := logs[1]
	if msg, ok := complete["msg"].(string); !ok || msg != "request completed" {
		t.Errorf("second log msg = %v, want 'request completed'", msg)
	}

	// Verify common fields
	for _, entry := range logs {
		if method, ok := entry["method"].(string); !ok || method != "POST" {
			t.Errorf("log missing or incorrect method: %v", entry)
		}
		if path, ok := entry["path"].(string); !ok || path != "/api/users" {
			t.Errorf("log missing or incorrect path: %v", entry)
		}
		if remote, ok := entry["remote_addr"].(string); !ok || remote != validation.SanitizeForLog("10.0.0.1:8080") {
			t.Errorf("log missing or incorrect remote_addr: %v", entry)
		}
	}

	// Verify completion fields
	if status, ok := complete["status"].(float64); !ok || int(status) != http.StatusOK {
		t.Errorf("completion log missing or incorrect status: %v", complete)
	}
	if dur, ok := complete["duration_ms"].(float64); !ok || dur <= 0 {
		t.Errorf("completion log missing or non-positive duration_ms: %v", complete)
	}
	if bytes, ok := complete["bytes_written"].(float64); !ok || int(bytes) != len("OK") {
		t.Errorf("completion log missing or incorrect bytes_written: %v", complete)
	}
}

func TestLogging_StatusCodeCapture(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantStatus int
	}{
		{"OK", http.StatusOK, http.StatusOK},
		{"Created", http.StatusCreated, http.StatusCreated},
		{"BadRequest", http.StatusBadRequest, http.StatusBadRequest},
		{"InternalServerError", http.StatusInternalServerError, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, getLogs := captureLogger()
			middleware := Logging(logger)
			// Custom handler that writes specific status code
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte("error"))
			})
			middlewareHandler := middleware(handler)

			req := makeRequest("GET", "/", "127.0.0.1:80", nil)
			rr := httptest.NewRecorder()
			middlewareHandler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("response status = %d, want %d", rr.Code, tt.wantStatus)
			}

			logs := getLogs()
			if len(logs) != 2 {
				t.Fatalf("expected 2 log entries, got %d", len(logs))
			}
			complete := logs[1]
			if status, ok := complete["status"].(float64); !ok || int(status) != tt.wantStatus {
				t.Errorf("log status = %v, want %d", status, tt.wantStatus)
			}
		})
	}
}

func TestLogging_BytesWrittenCapture(t *testing.T) {
	tests := []struct {
		name          string
		writeBody     string
		writeMultiple bool // write multiple times
		wantBytes     int
	}{
		{"empty body", "", false, 0},
		{"simple body", "hello", false, len("hello")},
		{"multiple writes", "hello world", true, len("hello world")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, getLogs := captureLogger()
			middleware := Logging(logger)
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				if tt.writeMultiple {
					w.Write([]byte(tt.writeBody[:5]))
					w.Write([]byte(tt.writeBody[5:]))
				} else {
					w.Write([]byte(tt.writeBody))
				}
			})
			middlewareHandler := middleware(handler)

			req := makeRequest("GET", "/", "127.0.0.1:80", nil)
			rr := httptest.NewRecorder()
			middlewareHandler.ServeHTTP(rr, req)

			logs := getLogs()
			complete := logs[1]
			if bytes, ok := complete["bytes_written"].(float64); !ok || int(bytes) != tt.wantBytes {
				t.Errorf("log bytes_written = %v, want %d", bytes, tt.wantBytes)
			}
		})
	}
}

func TestLogging_ContextLoggerInjection(t *testing.T) {
	logger, getLogs := captureLogger()
	middleware := Logging(logger)
	// Handler that retrieves logger from context and logs something extra
	var capturedLogger *slog.Logger
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Retrieve logger from context via RequestLoggerFromContext
		ctxLogger := RequestLoggerFromContext(r.Context())
		capturedLogger = ctxLogger
		// Log a test message to verify logger works
		ctxLogger.Info("handler log")
		w.WriteHeader(http.StatusOK)
	})
	middlewareHandler := middleware(handler)

	req := makeRequest("GET", "/", "127.0.0.1:80", nil)
	rr := httptest.NewRecorder()
	middlewareHandler.ServeHTTP(rr, req)

	if capturedLogger == nil {
		t.Error("failed to retrieve logger from context")
	}
	// Verify that three log entries were produced: request started, handler log, request completed
	logs := getLogs()
	if len(logs) != 3 {
		t.Fatalf("expected 3 log entries, got %d", len(logs))
	}
	// Check order
	if msg, ok := logs[0]["msg"].(string); !ok || msg != "request started" {
		t.Errorf("first log msg = %v, want 'request started'", logs[0]["msg"])
	}
	if msg, ok := logs[1]["msg"].(string); !ok || msg != "handler log" {
		t.Errorf("second log msg = %v, want 'handler log'", logs[1]["msg"])
	}
	if msg, ok := logs[2]["msg"].(string); !ok || msg != "request completed" {
		t.Errorf("third log msg = %v, want 'request completed'", logs[2]["msg"])
	}
	// Verify all logs have request_id
	for i, entry := range logs {
		if id, ok := entry["request_id"].(string); !ok || id == "" {
			t.Errorf("log entry %d missing request_id: %v", i, entry)
		}
	}
}

func TestLogging_Sanitization(t *testing.T) {
	logger, getLogs := captureLogger()
	middleware := Logging(logger)
	handler := &loggingHandler{}
	middlewareHandler := middleware(handler)

	// Use path with newline and control characters
	path := "/api/\nusers\r\t"
	remoteAddr := "192.168.1.1:12345\n"
	userAgent := "TestAgent\r\nX-Header: evil"
	// Create a request with a valid path, then modify fields to contain control characters
	req := makeRequest("GET", "/api/users", remoteAddr, nil)
	req.URL.Path = path
	req.RemoteAddr = remoteAddr
	req.Header.Set("User-Agent", userAgent)

	rr := httptest.NewRecorder()
	middlewareHandler.ServeHTTP(rr, req)

	logs := getLogs()
	for _, entry := range logs {
		// Check that path, remote_addr, user_agent are sanitized (no newlines)
		if pathVal, ok := entry["path"].(string); ok {
			if strings.ContainsAny(pathVal, "\n\r\t") {
				t.Errorf("path contains control characters after sanitization: %q", pathVal)
			}
			if pathVal != validation.SanitizeForLog(path) {
				t.Errorf("path mismatch: got %q, want %q", pathVal, validation.SanitizeForLog(path))
			}
		}
		if remoteVal, ok := entry["remote_addr"].(string); ok {
			if strings.ContainsAny(remoteVal, "\n\r\t") {
				t.Errorf("remote_addr contains control characters after sanitization: %q", remoteVal)
			}
			if remoteVal != validation.SanitizeForLog(remoteAddr) {
				t.Errorf("remote_addr mismatch: got %q, want %q", remoteVal, validation.SanitizeForLog(remoteAddr))
			}
		}
		if agentVal, ok := entry["user_agent"].(string); ok {
			if strings.ContainsAny(agentVal, "\n\r\t") {
				t.Errorf("user_agent contains control characters after sanitization: %q", agentVal)
			}
			if agentVal != validation.SanitizeForLog(userAgent) {
				t.Errorf("user_agent mismatch: got %q, want %q", agentVal, validation.SanitizeForLog(userAgent))
			}
		}
	}
}

func TestLogging_DurationCalculation(t *testing.T) {
	logger, getLogs := captureLogger()
	middleware := Logging(logger)
	// Handler with a known delay
	delay := 50 * time.Millisecond
	handler := &loggingHandler{delay: delay}
	middlewareHandler := middleware(handler)

	req := makeRequest("GET", "/", "127.0.0.1:80", nil)
	rr := httptest.NewRecorder()
	start := time.Now()
	middlewareHandler.ServeHTTP(rr, req)
	elapsed := time.Since(start)

	logs := getLogs()
	complete := logs[1]
	if dur, ok := complete["duration_ms"].(float64); !ok {
		t.Errorf("duration_ms missing")
	} else {
		// Duration should be within reasonable tolerance (maybe 10ms)
		got := time.Duration(dur) * time.Millisecond
		// Allow 20ms tolerance for CI variability
		if got < delay || got > elapsed+20*time.Millisecond {
			t.Errorf("duration_ms = %v, want between %v and %v", got, delay, elapsed+20*time.Millisecond)
		}
	}
}

func TestLogging_DefaultLogger(t *testing.T) {
	// When nil logger is passed, default logger should be used.
	// We can't easily capture logs from default logger, but we can ensure it doesn't panic.
	middleware := Logging(nil)
	handler := &loggingHandler{}
	middlewareHandler := middleware(handler)

	req := makeRequest("GET", "/", "127.0.0.1:80", nil)
	rr := httptest.NewRecorder()
	// Should not panic
	middlewareHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("request failed with status %d", rr.Code)
	}
	// Verify request ID header is set
	if rr.Header().Get("X-Request-ID") == "" {
		t.Error("missing X-Request-ID header")
	}
}

func TestLogging_ResponseWriterInterface(t *testing.T) {
	// Ensure the wrapped responseWriter implements http.Flusher, http.Hijacker, etc.
	// The middleware should preserve the underlying ResponseWriter's additional interfaces.
	// We'll test by using a httptest.ResponseRecorder which does not implement those.
	// That's fine; we just need to ensure the wrapper doesn't break standard behavior.
	logger, _ := captureLogger()
	middleware := Logging(logger)
	handler := &loggingHandler{}
	middlewareHandler := middleware(handler)

	req := makeRequest("GET", "/", "127.0.0.1:80", nil)
	rr := httptest.NewRecorder()
	middlewareHandler.ServeHTTP(rr, req)

	// Verify that writing after header works
	if rr.Body.String() != "OK" {
		t.Errorf("body = %q, want 'OK'", rr.Body.String())
	}
}

func TestLogging_GeneratedRequestID(t *testing.T) {
	logger, getLogs := captureLogger()
	middleware := Logging(logger)
	handler := &loggingHandler{}
	middlewareHandler := middleware(handler)

	req := makeRequest("GET", "/", "127.0.0.1:80", nil)
	rr := httptest.NewRecorder()
	middlewareHandler.ServeHTTP(rr, req)

	// Verify generated request ID matches expected pattern
	respID := rr.Header().Get("X-Request-ID")
	if respID == "" {
		t.Fatal("missing X-Request-ID header")
	}
	// Expected pattern: "req_<pid>"
	expectedPrefix := "req_"
	if !strings.HasPrefix(respID, expectedPrefix) {
		t.Errorf("request ID %q does not start with %q", respID, expectedPrefix)
	}
	// The suffix should be the process ID
	pid := os.Getpid()
	want := "req_" + strconv.Itoa(pid)
	if respID != want {
		t.Errorf("request ID %q does not match expected %q", respID, want)
	}
	// Verify logs also contain the same request ID
	logs := getLogs()
	for _, entry := range logs {
		if id, ok := entry["request_id"].(string); ok && id != respID {
			t.Errorf("log request_id %q differs from header %q", id, respID)
		}
	}
}
