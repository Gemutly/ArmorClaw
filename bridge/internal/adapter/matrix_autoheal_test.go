package adapter

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestIsRetryableHTTPErrorWithStatus verifies retryable error detection
// combining transport errors and HTTP status codes.
func TestIsRetryableHTTPErrorWithStatus(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		statusCode int
		want       bool
	}{
		{
			name:       "502 is retryable via status code",
			err:        errors.New("server error"),
			statusCode: 502,
			want:       true,
		},
		{
			name:       "503 is retryable via status code",
			err:        errors.New("service unavailable"),
			statusCode: 503,
			want:       true,
		},
		{
			name:       "500 is retryable via status code",
			err:        errors.New("internal server error"),
			statusCode: 500,
			want:       true,
		},
		{
			name:       "429 is retryable via status code",
			err:        errors.New("rate limited"),
			statusCode: 429,
			want:       true,
		},
		{
			name:       "401 is not retryable",
			err:        errors.New("unauthorized"),
			statusCode: 401,
			want:       false,
		},
		{
			name:       "200 is not retryable",
			err:        errors.New("ok"),
			statusCode: 200,
			want:       false,
		},
		{
			name:       "connection refused is retryable via transport error",
			err:        errors.New("connection refused"),
			statusCode: 0,
			want:       true,
		},
		{
			name:       "context canceled is retryable via transport error",
			err:        errors.New("context canceled"),
			statusCode: 0,
			want:       true,
		},
		{
			name:       "nil error with no status is not retryable",
			err:        nil,
			statusCode: 0,
			want:       false,
		},
		{
			name:       "non-retryable error with 403 status",
			err:        errors.New("forbidden"),
			statusCode: 403,
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableHTTPErrorWithStatus(tt.err, tt.statusCode); got != tt.want {
				t.Errorf("isRetryableHTTPErrorWithStatus(%v, %d) = %v, want %v", tt.err, tt.statusCode, got, tt.want)
			}
		})
	}
}

// TestIsRetryableStatusCode verifies which HTTP status codes are retryable.
func TestIsRetryableStatusCode(t *testing.T) {
	retryable := []int{429, 500, 502, 503, 504, 599}
	for _, code := range retryable {
		t.Run(fmt.Sprintf("retryable_%d", code), func(t *testing.T) {
			if !isRetryableStatusCode(code) {
				t.Errorf("status %d should be retryable", code)
			}
		})
	}

	notRetryable := []int{200, 400, 401, 403, 404, 409, 418}
	for _, code := range notRetryable {
		t.Run(fmt.Sprintf("not_retryable_%d", code), func(t *testing.T) {
			if isRetryableStatusCode(code) {
				t.Errorf("status %d should NOT be retryable", code)
			}
		})
	}
}

// TestIsRetryableHTTPError verifies transport-level error detection.
func TestIsRetryableHTTPError(t *testing.T) {
	retryableErrors := []string{
		"connection refused",
		"connection reset by peer",
		"connection timed out",
		"context deadline exceeded",
		"context canceled",
		"temporary failure in name resolution",
		"network is unreachable",
	}
	for _, msg := range retryableErrors {
		t.Run("retryable_"+msg, func(t *testing.T) {
			if !isRetryableHTTPError(errors.New(msg)) {
				t.Errorf("error containing %q should be retryable", msg)
			}
		})
	}

	nonRetryable := []string{
		"invalid character",
		"json: cannot unmarshal",
		"unexpected end of JSON",
	}
	for _, msg := range nonRetryable {
		t.Run("not_retryable_"+msg, func(t *testing.T) {
			if isRetryableHTTPError(errors.New(msg)) {
				t.Errorf("error %q should NOT be retryable", msg)
			}
		})
	}

	if isRetryableHTTPError(nil) {
		t.Error("nil error should not be retryable")
	}
}

// TestErrTokenInvalidatedSentinel verifies the sentinel error works with errors.Is.
func TestErrTokenInvalidatedSentinel(t *testing.T) {
	if !errors.Is(ErrTokenInvalidated, ErrTokenInvalidated) {
		t.Error("ErrTokenInvalidated should match itself via errors.Is")
	}

	wrapped := fmt.Errorf("sync failed: %w", ErrTokenInvalidated)
	if !errors.Is(wrapped, ErrTokenInvalidated) {
		t.Error("wrapped ErrTokenInvalidated should match via errors.Is")
	}

	doubleWrapped := fmt.Errorf("after retry: %w", wrapped)
	if !errors.Is(doubleWrapped, ErrTokenInvalidated) {
		t.Error("double-wrapped ErrTokenInvalidated should match via errors.Is")
	}

	unrelated := fmt.Errorf("something else: %w", errors.New("different"))
	if errors.Is(unrelated, ErrTokenInvalidated) {
		t.Error("unrelated error should NOT match ErrTokenInvalidated")
	}
}

// TestSyncDetectsMUnknownTokenInBody verifies M_UNKNOWN_TOKEN detection
// in sync response bodies without requiring a live Matrix server.
func TestSyncDetectsMUnknownTokenInBody(t *testing.T) {
	tokenBody := `{"errcode":"M_UNKNOWN_TOKEN","error":"Unrecognized access token"}`
	if !strings.Contains(tokenBody, "M_UNKNOWN_TOKEN") {
		t.Error("detection should find M_UNKNOWN_TOKEN in error body")
	}

	normalBody := `{"errcode":"M_UNKNOWN","error":"Unknown error"}`
	if strings.Contains(normalBody, "M_UNKNOWN_TOKEN") {
		t.Error("should not detect M_UNKNOWN_TOKEN in normal error body")
	}

	emptyBody := `{}`
	if strings.Contains(emptyBody, "M_UNKNOWN_TOKEN") {
		t.Error("should not detect M_UNKNOWN_TOKEN in empty response")
	}

	successBody := `{"next_batch":"s1","rooms":{"join":{}}}`
	if strings.Contains(successBody, "M_UNKNOWN_TOKEN") {
		t.Error("should not detect M_UNKNOWN_TOKEN in successful response")
	}
}

// TestSetStatusCallback verifies the status callback wiring.
func TestSetStatusCallback(t *testing.T) {
	m, err := New(Config{
		HomeserverURL: "http://localhost:6167",
	})
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	var received string
	m.SetStatusCallback(func(status string) {
		received = status
	})

	if m.statusCallback == nil {
		t.Fatal("status callback should be set after SetStatusCallback")
	}

	m.notifyStatus("Matrix: connected")
	if received != "Matrix: connected" {
		t.Errorf("expected 'Matrix: connected', got '%s'", received)
	}

	m.notifyStatus("Matrix: reconnecting (backoff: 5s)")
	if received != "Matrix: reconnecting (backoff: 5s)" {
		t.Errorf("expected 'Matrix: reconnecting (backoff: 5s)', got '%s'", received)
	}
}

// TestSetStatusCallbackNil verifies that notifyStatus handles nil callback safely.
func TestSetStatusCallbackNil(t *testing.T) {
	m, err := New(Config{
		HomeserverURL: "http://localhost:6167",
	})
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	m.notifyStatus("should not panic")
}
