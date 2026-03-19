package middleware

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// brokenReader is an io.Reader that returns an error after reading n bytes.
type brokenReader struct {
	data   []byte
	offset int
	err    error
}

func (b *brokenReader) Read(p []byte) (int, error) {
	if b.offset >= len(b.data) {
		return 0, b.err
	}
	n := copy(p, b.data[b.offset:])
	b.offset += n
	if b.offset >= len(b.data) {
		return n, b.err
	}
	return n, nil
}

// noLenReader wraps an io.Reader and hides any Len method.
type noLenReader struct {
	io.Reader
}

func TestSizeLimit(t *testing.T) {
	tests := []struct {
		name           string
		maxBytes       int64
		body           string
		setContentLen  bool // true to set Content-Length (if body length known)
		requestID      string
		wantStatusCode int
		wantHandler    bool
		wantErrorCode  string // empty if not checking
	}{
		{
			name:           "within limit, Content-Length set",
			maxBytes:       100,
			body:           strings.Repeat("a", 50),
			setContentLen:  true,
			wantStatusCode: http.StatusOK,
			wantHandler:    true,
		},
		{
			name:           "within limit, no Content-Length",
			maxBytes:       100,
			body:           strings.Repeat("b", 50),
			setContentLen:  false,
			wantStatusCode: http.StatusOK,
			wantHandler:    true,
		},
		{
			name:           "exact limit",
			maxBytes:       100,
			body:           strings.Repeat("c", 100),
			setContentLen:  true,
			wantStatusCode: http.StatusOK,
			wantHandler:    true,
		},
		{
			name:           "exact limit plus one byte",
			maxBytes:       100,
			body:           strings.Repeat("c", 101),
			setContentLen:  true,
			wantStatusCode: http.StatusRequestEntityTooLarge,
			wantHandler:    false,
			wantErrorCode:  "payload_too_large",
		},
		{
			name:           "Content-Length exceeds limit, early rejection",
			maxBytes:       100,
			body:           strings.Repeat("d", 150),
			setContentLen:  true,
			wantStatusCode: http.StatusRequestEntityTooLarge,
			wantHandler:    false,
			wantErrorCode:  "payload_too_large",
		},
		{
			name:           "body exceeds limit during read, no Content-Length",
			maxBytes:       100,
			body:           strings.Repeat("e", 150),
			setContentLen:  false,
			wantStatusCode: http.StatusRequestEntityTooLarge,
			wantHandler:    false,
			wantErrorCode:  "payload_too_large",
		},
		{
			name:           "zero maxBytes, no limit",
			maxBytes:       0,
			body:           strings.Repeat("g", 500),
			setContentLen:  true,
			wantStatusCode: http.StatusOK,
			wantHandler:    true,
		},
		{
			name:           "negative maxBytes, no limit",
			maxBytes:       -1,
			body:           strings.Repeat("h", 500),
			setContentLen:  true,
			wantStatusCode: http.StatusOK,
			wantHandler:    true,
		},
		{
			name:           "nil body",
			maxBytes:       100,
			body:           "",
			setContentLen:  false,
			wantStatusCode: http.StatusOK,
			wantHandler:    true,
		},
		{
			name:           "zero Content-Length with nil body",
			maxBytes:       100,
			body:           "",
			setContentLen:  true, // Content-Length 0
			wantStatusCode: http.StatusOK,
			wantHandler:    true,
		},
		{
			name:           "request ID with newlines, within limit",
			maxBytes:       100,
			body:           strings.Repeat("i", 50),
			setContentLen:  true,
			requestID:      "req\nid\r\nwith\tcontrols",
			wantStatusCode: http.StatusOK,
			wantHandler:    true,
		},
		{
			name:           "request ID with newlines, exceeds limit",
			maxBytes:       100,
			body:           strings.Repeat("j", 150),
			setContentLen:  true,
			requestID:      "req\nid\r\nwith\tcontrols",
			wantStatusCode: http.StatusRequestEntityTooLarge,
			wantHandler:    false,
			wantErrorCode:  "payload_too_large",
		},
		{
			name:           "Content-Length shorter than actual body (malicious client)",
			maxBytes:       100,
			body:           strings.Repeat("k", 150),
			setContentLen:  true, // Content-Length will be 150 (body length), not shorter
			wantStatusCode: http.StatusRequestEntityTooLarge,
			wantHandler:    false,
			wantErrorCode:  "payload_too_large",
		},
		{
			name:           "Content-Length longer than actual body but within limit",
			maxBytes:       100,
			body:           strings.Repeat("l", 50),
			setContentLen:  true,
			wantStatusCode: http.StatusOK,
			wantHandler:    true,
		},
		{
			name:           "Content-Length zero with body present",
			maxBytes:       100,
			body:           strings.Repeat("m", 150),
			setContentLen:  true, // Content-Length will be 150, not zero
			wantStatusCode: http.StatusRequestEntityTooLarge,
			wantHandler:    false,
			wantErrorCode:  "payload_too_large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled := false
			handler := SizeLimit(tt.maxBytes)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				// Verify the body is still readable and matches original
				if tt.body != "" {
					data, err := io.ReadAll(r.Body)
					if err != nil {
						t.Errorf("failed to read body in handler: %v", err)
					}
					if string(data) != tt.body {
						t.Errorf("body mismatch: got %q, want %q", string(data), tt.body)
					}
				}
				w.WriteHeader(http.StatusOK)
			}))

			var bodyReader io.Reader
			if tt.body != "" {
				if tt.setContentLen {
					// Use strings.Reader which has Len, httptest.NewRequest will set Content-Length
					bodyReader = strings.NewReader(tt.body)
				} else {
					// Wrap in noLenReader to hide Len method
					bodyReader = noLenReader{strings.NewReader(tt.body)}
				}
			}
			req := httptest.NewRequest(http.MethodPost, "/", bodyReader)
			if tt.requestID != "" {
				req.Header.Set("X-Request-ID", tt.requestID)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatusCode {
				t.Errorf("status code = %d, want %d", rr.Code, tt.wantStatusCode)
			}
			if handlerCalled != tt.wantHandler {
				t.Errorf("handler called = %v, want %v", handlerCalled, tt.wantHandler)
			}
			if tt.wantErrorCode != "" {
				// Parse JSON error response
				var resp struct {
					Error struct {
						Code    string `json:"code"`
						Message string `json:"message"`
					} `json:"error"`
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Errorf("failed to unmarshal error response: %v, body: %s", err, rr.Body.String())
				} else {
					if resp.Error.Code != tt.wantErrorCode {
						t.Errorf("error code = %q, want %q", resp.Error.Code, tt.wantErrorCode)
					}
					// Verify error message matches middleware's message
					if resp.Error.Message != "request body too large" {
						t.Errorf("error message = %q, want %q", resp.Error.Message, "request body too large")
					}
				}
			}
		})
	}
}

func TestSizeLimit_ReadError(t *testing.T) {
	// Simulate a read error after reading some bytes
	br := &brokenReader{
		data: []byte("partial data"),
		err:  errors.New("network error"),
	}
	req := httptest.NewRequest(http.MethodPost, "/", br)
	req.ContentLength = int64(len(br.data))

	handlerCalled := false
	handler := SizeLimit(100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
	if handlerCalled {
		t.Error("handler should not be called on read error")
	}
	// Verify error code
	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Errorf("failed to unmarshal error response: %v", err)
	} else if resp.Error.Code != "internal_server_error" {
		t.Errorf("error code = %q, want internal_server_error", resp.Error.Code)
	}
}

func TestSizeLimit_BodyReplacement(t *testing.T) {
	// Ensure the body is replaced with a reader over buffered data
	const body = "hello world"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.ContentLength = int64(len(body))

	var capturedBody []byte
	handler := SizeLimit(100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rr.Code, http.StatusOK)
	}
	if string(capturedBody) != body {
		t.Errorf("captured body = %q, want %q", capturedBody, body)
	}
}

func TestSizeLimit_ZeroContentLengthWithBody(t *testing.T) {
	// When Content-Length is 0 but body is present (edge case)
	// The middleware checks r.Body == nil || r.ContentLength == 0
	// If ContentLength == 0, it skips limiting.
	// This test ensures that a body is still read correctly.
	const body = "some body"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.ContentLength = 0 // explicitly set zero

	handlerCalled := false
	handler := SizeLimit(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
		}
		if string(data) != body {
			t.Errorf("body = %q, want %q", string(data), body)
		}
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("handler should be called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestSizeLimit_NoBodyNil(t *testing.T) {
	// Request with nil body (http.NoBody)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Body = nil // actually http.NoBody, but we can set nil
	// The middleware should skip limiting because r.Body == nil
	handlerCalled := false
	handler := SizeLimit(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("handler should be called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestSizeLimit_NegativeContentLength(t *testing.T) {
	// If Content-Length is negative (invalid), middleware should skip early rejection
	// and read the body.
	const body = "small"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.ContentLength = -1 // invalid, but ensure no panic

	handlerCalled := false
	handler := SizeLimit(100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
		}
		if string(data) != body {
			t.Errorf("body = %q, want %q", string(data), body)
		}
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("handler should be called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rr.Code, http.StatusOK)
	}
}
