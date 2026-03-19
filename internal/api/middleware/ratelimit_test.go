package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// testHandler is a simple handler that records the number of times it's called.
type testHandler struct {
	calls int
}

func (h *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.calls++
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// makeRequest is a helper to create a request with optional headers and remote address.
func makeRequest(method, path, remoteAddr string, headers map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

// parseErrorResponse parses the error response body and returns the error code and message.
func parseErrorResponse(body []byte) (code, message string) {
	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", ""
	}
	return resp.Error.Code, resp.Error.Message
}

func TestRateLimit_ClientIPExtraction(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		wantIP     string
	}{
		{
			name:       "RemoteAddr only",
			remoteAddr: "192.168.1.1:12345",
			headers:    nil,
			wantIP:     "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For single IP",
			remoteAddr: "10.0.0.1:80",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.5"},
			wantIP:     "203.0.113.5",
		},
		{
			name:       "X-Forwarded-For multiple IPs",
			remoteAddr: "10.0.0.1:80",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.5, 198.51.100.1"},
			wantIP:     "203.0.113.5",
		},
		{
			name:       "X-Forwarded-For with spaces",
			remoteAddr: "10.0.0.1:80",
			headers:    map[string]string{"X-Forwarded-For": " 203.0.113.5 , 198.51.100.1 "},
			wantIP:     "203.0.113.5",
		},
		{
			name:       "Invalid X-Forwarded-For falls back to RemoteAddr",
			remoteAddr: "192.168.1.2:12345",
			headers:    map[string]string{"X-Forwarded-For": "invalid-ip"},
			wantIP:     "192.168.1.2",
		},
		{
			name:       "Empty X-Forwarded-For falls back to RemoteAddr",
			remoteAddr: "192.168.1.3:12345",
			headers:    map[string]string{"X-Forwarded-For": ""},
			wantIP:     "192.168.1.3",
		},
		{
			name:       "RemoteAddr without port",
			remoteAddr: "192.168.1.4",
			headers:    nil,
			wantIP:     "192.168.1.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a per‑IP rate limit to detect different IPs.
			// Set a low per‑IP rate (1 request per second) and send two requests.
			// If IP extraction is correct, the second request from the same extracted IP
			// should be blocked.
			rl := RateLimit(0, 1, 1, 1) // global off, auth/unauth 1 req/sec, burst 1
			handler := &testHandler{}
			middleware := rl(handler)

			req := makeRequest("GET", "/", tt.remoteAddr, tt.headers)
			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Errorf("first request failed: %d", rr.Code)
			}
			if handler.calls != 1 {
				t.Errorf("handler calls = %d, want 1", handler.calls)
			}

			// Second request from same extracted IP should be rate limited.
			req2 := makeRequest("GET", "/", tt.remoteAddr, tt.headers)
			rr2 := httptest.NewRecorder()
			middleware.ServeHTTP(rr2, req2)
			if rr2.Code != http.StatusTooManyRequests {
				t.Errorf("second request status = %d, want %d", rr2.Code, http.StatusTooManyRequests)
			}
			// Ensure Retry-After header is set.
			if retryAfter := rr2.Header().Get("Retry-After"); retryAfter != "1" {
				t.Errorf("Retry-After header = %q, want %q", retryAfter, "1")
			}
			// Verify error response.
			code, _ := parseErrorResponse(rr2.Body.Bytes())
			if code != "rate_limit_exceeded" {
				t.Errorf("error code = %q, want %q", code, "rate_limit_exceeded")
			}
		})
	}
}

func TestRateLimit_HealthEndpointExemption(t *testing.T) {
	// Health endpoint is always considered unauthenticated (line 78-80).
	// Use a high authenticated rate but zero unauthenticated rate; health should still pass.
	rl := RateLimit(0, 100, 0, 1) // global off, auth 100 req/sec, unauth 0 (no limit)
	handler := &testHandler{}
	middleware := rl(handler)

	// Request to /health without X-API-Key (unauthenticated) should succeed.
	req := makeRequest("GET", "/health", "192.168.1.1:12345", nil)
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("health endpoint failed: %d", rr.Code)
	}
	if handler.calls != 1 {
		t.Errorf("handler calls = %d, want 1", handler.calls)
	}

	// Multiple health requests should all succeed (unauth rate is zero, meaning no limit).
	for i := 0; i < 5; i++ {
		req := makeRequest("GET", "/health", "192.168.1.1:12345", nil)
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("health request %d failed: %d", i, rr.Code)
		}
	}
	if handler.calls != 6 {
		t.Errorf("handler calls = %d, want 6", handler.calls)
	}
}

func TestRateLimit_AuthenticatedVsUnauthenticated(t *testing.T) {
	tests := []struct {
		name        string
		authRate    float64
		unauthRate  float64
		header      map[string]string
		wantLimited bool
		description string
	}{
		{
			name:        "authenticated with key, auth rate limited",
			authRate:    1,
			unauthRate:  100,
			header:      map[string]string{"X-API-Key": "secret"},
			wantLimited: true,
			description: "auth rate 1 req/sec, second request should be limited",
		},
		{
			name:        "unauthenticated, unauth rate limited",
			authRate:    100,
			unauthRate:  1,
			header:      nil,
			wantLimited: true,
			description: "unauth rate 1 req/sec, second request should be limited",
		},
		{
			name:        "authenticated with key, auth rate unlimited",
			authRate:    0,
			unauthRate:  1,
			header:      map[string]string{"X-API-Key": "secret"},
			wantLimited: false,
			description: "auth rate 0 (no limit), second request should pass",
		},
		{
			name:        "unauthenticated, unauth rate unlimited",
			authRate:    1,
			unauthRate:  0,
			header:      nil,
			wantLimited: false,
			description: "unauth rate 0 (no limit), second request should pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := RateLimit(0, tt.authRate, tt.unauthRate, 1)
			handler := &testHandler{}
			middleware := rl(handler)

			// First request should always succeed.
			req := makeRequest("GET", "/api/test", "192.168.1.1:12345", tt.header)
			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Errorf("first request failed: %d", rr.Code)
			}
			if handler.calls != 1 {
				t.Errorf("handler calls = %d, want 1", handler.calls)
			}

			// Second request from same IP.
			req2 := makeRequest("GET", "/api/test", "192.168.1.1:12345", tt.header)
			rr2 := httptest.NewRecorder()
			middleware.ServeHTTP(rr2, req2)

			if tt.wantLimited {
				if rr2.Code != http.StatusTooManyRequests {
					t.Errorf("second request status = %d, want %d", rr2.Code, http.StatusTooManyRequests)
				}
			} else {
				if rr2.Code != http.StatusOK {
					t.Errorf("second request status = %d, want %d", rr2.Code, http.StatusOK)
				}
				if handler.calls != 2 {
					t.Errorf("handler calls = %d, want 2", handler.calls)
				}
			}
		})
	}
}

func TestRateLimit_GlobalLimit(t *testing.T) {
	// Enable global limit (1 req/sec) and no per‑IP limit.
	rl := RateLimit(1, 0, 0, 1)
	handler := &testHandler{}
	middleware := rl(handler)

	// First request from any IP should pass.
	req1 := makeRequest("GET", "/", "192.168.1.1:12345", nil)
	rr1 := httptest.NewRecorder()
	middleware.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Errorf("first request failed: %d", rr1.Code)
	}
	if handler.calls != 1 {
		t.Errorf("handler calls = %d, want 1", handler.calls)
	}

	// Second request from a different IP should still be blocked by global limit.
	req2 := makeRequest("GET", "/", "10.0.0.2:54321", nil)
	rr2 := httptest.NewRecorder()
	middleware.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("second request status = %d, want %d", rr2.Code, http.StatusTooManyRequests)
	}
	// Verify global limit log? Not needed.
	// Ensure per‑IP limit didn't block (since rate is zero).
	// Use same IP again after waiting? Not needed.
}

func TestRateLimit_GlobalAndPerIP(t *testing.T) {
	// Global limit 2 req/sec, per‑IP limit 1 req/sec.
	// Burst multiplier 1 (burst = rate).
	rl := RateLimit(2, 1, 1, 1)
	handler := &testHandler{}
	middleware := rl(handler)

	// First request from IP1 passes both global and per‑IP.
	req1 := makeRequest("GET", "/", "192.168.1.1:12345", nil)
	rr1 := httptest.NewRecorder()
	middleware.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Errorf("first request failed: %d", rr1.Code)
	}
	if handler.calls != 1 {
		t.Errorf("handler calls = %d, want 1", handler.calls)
	}

	// Second request from same IP should be blocked by per‑IP limit.
	req2 := makeRequest("GET", "/", "192.168.1.1:12345", nil)
	rr2 := httptest.NewRecorder()
	middleware.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("second request status = %d, want %d", rr2.Code, http.StatusTooManyRequests)
	}
	// Verify error is per‑IP (can't differentiate, but we can check logs? skip)

	// Request from different IP should be blocked by global limit (global tokens exhausted by first two requests).
	req3 := makeRequest("GET", "/", "10.0.0.2:54321", nil)
	rr3 := httptest.NewRecorder()
	middleware.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusTooManyRequests {
		t.Errorf("third request status = %d, want %d", rr3.Code, http.StatusTooManyRequests)
	}
	// Handler calls remain 1 (only first request succeeded).
	if handler.calls != 1 {
		t.Errorf("handler calls = %d, want 1", handler.calls)
	}

	// Next request from any IP should be blocked by global limit (global tokens exhausted).
	req4 := makeRequest("GET", "/", "192.168.1.3:12345", nil)
	rr4 := httptest.NewRecorder()
	middleware.ServeHTTP(rr4, req4)
	if rr4.Code != http.StatusTooManyRequests {
		t.Errorf("fourth request status = %d, want %d", rr4.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimit_BurstMultiplier(t *testing.T) {
	// Rate 1 req/sec, burst multiplier 3 => burst = 3.
	rl := RateLimit(0, 1, 1, 3)
	handler := &testHandler{}
	middleware := rl(handler)

	// First 3 requests should succeed (burst size).
	for i := 0; i < 3; i++ {
		req := makeRequest("GET", "/", "192.168.1.1:12345", nil)
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("request %d failed: %d", i, rr.Code)
		}
	}
	if handler.calls != 3 {
		t.Errorf("handler calls = %d, want 3", handler.calls)
	}

	// Fourth request should be limited.
	req := makeRequest("GET", "/", "192.168.1.1:12345", nil)
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("fourth request status = %d, want %d", rr.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimit_NoLimitWhenRatesZero(t *testing.T) {
	// All rates zero => no limiting at all.
	rl := RateLimit(0, 0, 0, 1)
	handler := &testHandler{}
	middleware := rl(handler)

	// Send many requests from same IP.
	for i := 0; i < 10; i++ {
		req := makeRequest("GET", "/", "192.168.1.1:12345", nil)
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("request %d failed: %d", i, rr.Code)
		}
	}
	if handler.calls != 10 {
		t.Errorf("handler calls = %d, want 10", handler.calls)
	}
}

func TestRateLimit_ResponseHeaders(t *testing.T) {
	// Ensure 429 responses include Retry-After header.
	rl := RateLimit(0, 1, 1, 1)
	handler := &testHandler{}
	middleware := rl(handler)

	// Exhaust the token.
	req1 := makeRequest("GET", "/", "192.168.1.1:12345", nil)
	rr1 := httptest.NewRecorder()
	middleware.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first request failed: %d", rr1.Code)
	}

	req2 := makeRequest("GET", "/", "192.168.1.1:12345", nil)
	rr2 := httptest.NewRecorder()
	middleware.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rr2.Code)
	}
	if retryAfter := rr2.Header().Get("Retry-After"); retryAfter != "1" {
		t.Errorf("Retry-After header = %q, want %q", retryAfter, "1")
	}
	// Verify JSON error structure.
	code, message := parseErrorResponse(rr2.Body.Bytes())
	if code != "rate_limit_exceeded" {
		t.Errorf("error code = %q, want %q", code, "rate_limit_exceeded")
	}
	if message != "rate limit exceeded" {
		t.Errorf("error message = %q, want %q", message, "rate limit exceeded")
	}
}

func TestRateLimit_HealthEndpointWithAPIKey(t *testing.T) {
	// Health endpoint is always considered unauthenticated, even with X-API-Key header.
	// Set auth rate limit 1 req/sec, unauth rate unlimited.
	rl := RateLimit(0, 1, 0, 1)
	handler := &testHandler{}
	middleware := rl(handler)

	// Request to /health with X-API-Key header should be considered unauthenticated.
	headers := map[string]string{"X-API-Key": "secret"}
	// First request should pass (unauth rate unlimited).
	req1 := makeRequest("GET", "/health", "192.168.1.1:12345", headers)
	rr1 := httptest.NewRecorder()
	middleware.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Errorf("first health request failed: %d", rr1.Code)
	}
	if handler.calls != 1 {
		t.Errorf("handler calls = %d, want 1", handler.calls)
	}
	// Second request should also pass (no limit).
	req2 := makeRequest("GET", "/health", "192.168.1.1:12345", headers)
	rr2 := httptest.NewRecorder()
	middleware.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Errorf("second health request failed: %d", rr2.Code)
	}
	if handler.calls != 2 {
		t.Errorf("handler calls = %d, want 2", handler.calls)
	}
}

func TestRateLimit_BurstMultiplierEdgeCases(t *testing.T) {
	// Test that burst is at least 1 even when rate * multiplier < 1.
	// rate 0.5, multiplier 1 => burst should be 1.
	rl := RateLimit(0, 0.5, 0.5, 1)
	handler := &testHandler{}
	middleware := rl(handler)

	// First request should succeed (burst 1).
	req1 := makeRequest("GET", "/", "192.168.1.1:12345", nil)
	rr1 := httptest.NewRecorder()
	middleware.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Errorf("first request failed: %d", rr1.Code)
	}
	// Second request should be limited (only one token in bucket).
	req2 := makeRequest("GET", "/", "192.168.1.1:12345", nil)
	rr2 := httptest.NewRecorder()
	middleware.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("second request status = %d, want %d", rr2.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimit_GlobalBurstMultiplier(t *testing.T) {
	// Global rate 1 req/sec, burst multiplier 5 => burst 5.
	rl := RateLimit(1, 0, 0, 5)
	handler := &testHandler{}
	middleware := rl(handler)

	// First 5 requests should succeed.
	for i := 0; i < 5; i++ {
		req := makeRequest("GET", "/", "192.168.1.1:12345", nil)
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("request %d failed: %d", i, rr.Code)
		}
	}
	if handler.calls != 5 {
		t.Errorf("handler calls = %d, want 5", handler.calls)
	}
	// Sixth request should be limited.
	req := makeRequest("GET", "/", "192.168.1.1:12345", nil)
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("sixth request status = %d, want %d", rr.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimit_GlobalLimitDisabled(t *testing.T) {
	// Global rate 0 means no global limit.
	rl := RateLimit(0, 1, 1, 1)
	handler := &testHandler{}
	middleware := rl(handler)

	// Exhaust per‑IP limit.
	req1 := makeRequest("GET", "/", "192.168.1.1:12345", nil)
	rr1 := httptest.NewRecorder()
	middleware.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Errorf("first request failed: %d", rr1.Code)
	}
	// Second request from same IP blocked by per‑IP limit.
	req2 := makeRequest("GET", "/", "192.168.1.1:12345", nil)
	rr2 := httptest.NewRecorder()
	middleware.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("second request status = %d, want %d", rr2.Code, http.StatusTooManyRequests)
	}
	// Request from different IP should pass (global limit disabled).
	req3 := makeRequest("GET", "/", "10.0.0.2:54321", nil)
	rr3 := httptest.NewRecorder()
	middleware.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusOK {
		t.Errorf("third request status = %d, want %d", rr3.Code, http.StatusOK)
	}
}

func TestRateLimit_PerIPLimitOnly(t *testing.T) {
	// Only per‑IP limit enabled, global disabled.
	// Authenticated vs unauthenticated rates.
	// Use auth rate 1, unauth rate 100.
	rl := RateLimit(0, 1, 100, 1)
	handler := &testHandler{}
	middleware := rl(handler)

	// Authenticated request.
	headers := map[string]string{"X-API-Key": "secret"}
	req1 := makeRequest("GET", "/api", "192.168.1.1:12345", headers)
	rr1 := httptest.NewRecorder()
	middleware.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Errorf("first authenticated request failed: %d", rr1.Code)
	}
	// Second authenticated request from same IP should be limited.
	req2 := makeRequest("GET", "/api", "192.168.1.1:12345", headers)
	rr2 := httptest.NewRecorder()
	middleware.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("second authenticated request status = %d, want %d", rr2.Code, http.StatusTooManyRequests)
	}
	// Unauthenticated request from same IP should also be limited because per‑IP limiter is shared across authentication states.
	// The limiter was created with auth rate 1 (first request), so only one token per second.
	req3 := makeRequest("GET", "/api", "192.168.1.1:12345", nil)
	rr3 := httptest.NewRecorder()
	middleware.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusTooManyRequests {
		t.Errorf("unauthenticated request status = %d, want %d (shared limiter)", rr3.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimit_PerIPLimiterShared(t *testing.T) {
	// Verify that per‑IP limiter is shared across authentication states.
	// If first request is unauthenticated, the limiter is created with unauth rate.
	rl := RateLimit(0, 100, 1, 1) // auth 100, unauth 1 (so unauth rate is limiting)
	handler := &testHandler{}
	middleware := rl(handler)

	// First request unauthenticated (unauth rate 1, burst 1).
	req1 := makeRequest("GET", "/api", "192.168.1.1:12345", nil)
	rr1 := httptest.NewRecorder()
	middleware.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Errorf("first unauthenticated request failed: %d", rr1.Code)
	}
	// Second unauthenticated request should be limited (burst 1).
	req2 := makeRequest("GET", "/api", "192.168.1.1:12345", nil)
	rr2 := httptest.NewRecorder()
	middleware.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("second unauthenticated request status = %d, want %d", rr2.Code, http.StatusTooManyRequests)
	}
	// Authenticated request should also be limited (same limiter with rate 1).
	headers := map[string]string{"X-API-Key": "secret"}
	req3 := makeRequest("GET", "/api", "192.168.1.1:12345", headers)
	rr3 := httptest.NewRecorder()
	middleware.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusTooManyRequests {
		t.Errorf("authenticated request status = %d, want %d (shared limiter)", rr3.Code, http.StatusTooManyRequests)
	}
}

// TestRateLimit_CleanupGoroutine is not needed for unit tests; integration test may verify.
// The goroutine is started by the middleware and stopped when the rateLimiter is garbage collected.
// We can skip testing it.
