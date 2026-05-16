package rpc

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func newTestMiddleware(group RPCGroup, adminToken string) *SafetyMiddleware {
	limiters := map[string]*RateLimiter{
		group.Name: NewRateLimiter(group.RateLimit),
	}
	return NewSafetyMiddleware(SafetyConfig{
		Group:      group,
		AdminToken: adminToken,
		Limiters:   limiters,
		AuditLogger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})),
	})
}

func fakeHandler(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	return map[string]string{"status": "ok"}, nil
}

func slowHandler(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	select {
	case <-time.After(5 * time.Second):
		return map[string]string{"status": "slow_ok"}, nil
	case <-ctx.Done():
		return nil, &ErrorObj{Code: InternalError, Message: ctx.Err().Error()}
	}
}

func TestRPCSafetyAuthMissingToken(t *testing.T) {
	mw := newTestMiddleware(BrowserRPCGroup, "valid-admin-token")
	wrapped := mw.Wrap(fakeHandler)

	req := &Request{
		Method: "browser.navigate",
		Params: json.RawMessage(`{"url":"http://example.com"}`),
	}

	_, errObj := wrapped(context.Background(), req)
	if errObj == nil {
		t.Fatal("expected error for missing token, got nil")
	}
	if errObj.Code != RPCAuthRequired {
		t.Errorf("expected RPCAuthRequired (%d), got %d", RPCAuthRequired, errObj.Code)
	}
	if !strings.Contains(errObj.Message, "missing token") {
		t.Errorf("expected 'missing token' in message, got: %s", errObj.Message)
	}
}

func TestRPCSafetyAuthInvalidToken(t *testing.T) {
	mw := newTestMiddleware(BrowserRPCGroup, "valid-admin-token")
	wrapped := mw.Wrap(fakeHandler)

	req := &Request{
		Method: "browser.navigate",
		Params: json.RawMessage(`{"url":"http://example.com","token":"wrong-token"}`),
	}

	_, errObj := wrapped(context.Background(), req)
	if errObj == nil {
		t.Fatal("expected error for invalid token, got nil")
	}
	if errObj.Code != RPCAuthForbidden {
		t.Errorf("expected RPCAuthForbidden (%d), got %d", RPCAuthForbidden, errObj.Code)
	}
}

func TestRPCSafetyAuthValidToken(t *testing.T) {
	mw := newTestMiddleware(BrowserRPCGroup, "valid-admin-token")
	wrapped := mw.Wrap(fakeHandler)

	req := &Request{
		Method: "browser.navigate",
		Params: json.RawMessage(`{"url":"http://example.com","token":"valid-admin-token"}`),
	}

	result, errObj := wrapped(context.Background(), req)
	if errObj != nil {
		t.Fatalf("expected success, got error: %v", errObj)
	}
	m, ok := result.(map[string]string)
	if !ok || m["status"] != "ok" {
		t.Errorf("expected status=ok, got: %v", result)
	}
}

func TestRPCSafetyAuthSkippedWhenNoToken(t *testing.T) {
	mw := newTestMiddleware(BrowserRPCGroup, "")
	wrapped := mw.Wrap(fakeHandler)

	req := &Request{
		Method: "browser.navigate",
		Params: json.RawMessage(`{"url":"http://example.com"}`),
	}

	result, errObj := wrapped(context.Background(), req)
	if errObj != nil {
		t.Fatalf("expected success with no token config, got error: %v", errObj)
	}
	m, ok := result.(map[string]string)
	if !ok || m["status"] != "ok" {
		t.Errorf("expected status=ok, got: %v", result)
	}
}

func TestRPCSafetyTimeoutEnforced(t *testing.T) {
	group := RPCGroup{
		Name:      "test-timeout",
		Timeout:   100 * time.Millisecond,
		RateLimit: 100,
	}
	mw := newTestMiddleware(group, "")
	wrapped := mw.Wrap(slowHandler)

	req := &Request{
		Method: "test.slow",
		Params: json.RawMessage(`{}`),
	}

	start := time.Now()
	_, errObj := wrapped(context.Background(), req)
	elapsed := time.Since(start)

	if errObj == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(errObj.Message, "context deadline") && !strings.Contains(errObj.Message, "context canceled") {
		t.Errorf("expected context deadline error, got: %s", errObj.Message)
	}
	if elapsed > 2*time.Second {
		t.Errorf("handler should have timed out quickly, took %v", elapsed)
	}
}

func TestRPCSafetyRateLimitEnforced(t *testing.T) {
	group := RPCGroup{
		Name:      "test-ratelimit",
		Timeout:   30 * time.Second,
		RateLimit: 3,
	}
	mw := newTestMiddleware(group, "")
	wrapped := mw.Wrap(fakeHandler)

	for i := 0; i < 3; i++ {
		req := &Request{
			Method: "test.rate",
			Params: json.RawMessage(`{}`),
		}
		_, errObj := wrapped(context.Background(), req)
		if errObj != nil {
			t.Fatalf("request %d should have succeeded, got: %v", i+1, errObj)
		}
	}

	req := &Request{
		Method: "test.rate",
		Params: json.RawMessage(`{}`),
	}
	_, errObj := wrapped(context.Background(), req)
	if errObj == nil {
		t.Fatal("expected rate limit error on 4th request, got nil")
	}
	if errObj.Code != RPCRateExceeded {
		t.Errorf("expected RPCRateExceeded (%d), got %d", RPCRateExceeded, errObj.Code)
	}
}

func TestRPCSafetyRateLimitReset(t *testing.T) {
	limiter := NewRateLimiter(2)

	if !limiter.Allow("key1") {
		t.Error("first request should be allowed")
	}
	if !limiter.Allow("key1") {
		t.Error("second request should be allowed")
	}
	if limiter.Allow("key1") {
		t.Error("third request should be rate limited")
	}

	limiter.Reset()

	if !limiter.Allow("key1") {
		t.Error("request after reset should be allowed")
	}
}

func TestRPCSafetyRateLimitPerKey(t *testing.T) {
	limiter := NewRateLimiter(1)

	if !limiter.Allow("user-a") {
		t.Error("first request from user-a should be allowed")
	}
	if !limiter.Allow("user-b") {
		t.Error("first request from user-b should be allowed (different key)")
	}
	if limiter.Allow("user-a") {
		t.Error("second request from user-a should be rate limited")
	}
	if limiter.Allow("user-b") {
		t.Error("second request from user-b should be rate limited")
	}
}

func TestRPCSafetyBodyTooLarge(t *testing.T) {
	group := RPCGroup{
		Name:       "test-body",
		Timeout:    30 * time.Second,
		RateLimit:  100,
		MaxBodyLen: 10,
	}
	mw := newTestMiddleware(group, "")
	wrapped := mw.Wrap(fakeHandler)

	req := &Request{
		Method: "test.body",
		Params: json.RawMessage(`{"data":"this is way more than 10 bytes"}`),
	}

	_, errObj := wrapped(context.Background(), req)
	if errObj == nil {
		t.Fatal("expected body too large error, got nil")
	}
	if errObj.Code != RPCBodyTooLarge {
		t.Errorf("expected RPCBodyTooLarge (%d), got %d", RPCBodyTooLarge, errObj.Code)
	}
}

func TestRPCSafetyFailClosed(t *testing.T) {
	mw := newTestMiddleware(BrowserRPCGroup, "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	wrapped := mw.Wrap(fakeHandler)
	req := &Request{
		Method: "browser.navigate",
		Params: json.RawMessage(`{}`),
	}

	_, errObj := wrapped(ctx, req)
	if errObj == nil {
		t.Fatal("expected fail-closed error, got nil")
	}
	if errObj.Code != RPCFailClosed {
		t.Errorf("expected RPCFailClosed (%d), got %d", RPCFailClosed, errObj.Code)
	}
}

func TestRPCSafetyGroupProfiles(t *testing.T) {
	profiles := []struct {
		name      string
		group     RPCGroup
		timeoutGT time.Duration
		rateLimit int
	}{
		{"browser", BrowserRPCGroup, 25 * time.Second, 20},
		{"jetski", JetskiRPCGroup, 5 * time.Second, 30},
		{"document", DocumentRPCGroup, 50 * time.Second, 10},
		{"email", EmailRPCGroup, 25 * time.Second, 20},
	}

	for _, p := range profiles {
		t.Run(p.name, func(t *testing.T) {
			if p.group.Timeout < p.timeoutGT {
				t.Errorf("timeout too low: %v (expected >= %v)", p.group.Timeout, p.timeoutGT)
			}
			if p.group.RateLimit != p.rateLimit {
				t.Errorf("rate limit mismatch: got %d, expected %d", p.group.RateLimit, p.rateLimit)
			}
			if p.group.MaxBodyLen != 1<<20 {
				t.Errorf("max body length should be 1MB, got %d", p.group.MaxBodyLen)
			}
		})
	}
}

func TestRPCSafetySanitizeKey(t *testing.T) {
	mw := &SafetyMiddleware{}

	tests := []struct {
		input    string
		expected string
	}{
		{"short", "****"},
		{"", "****"},
		{"12345678", "****"},
		{"abcdefghijk", "abcd****hijk"},
	}

	for _, tt := range tests {
		got := mw.sanitizeKey(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeKey(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestRPCSafetyWrapForGroup(t *testing.T) {
	mw := NewBEATOSafetyMiddleware("test-token")

	wrapped := mw.WrapForGroup(DocumentRPCGroup, fakeHandler)

	req := &Request{
		Method: "document.extract_text",
		Params: json.RawMessage(`{"token":"test-token"}`),
	}

	result, errObj := wrapped(context.Background(), req)
	if errObj != nil {
		t.Fatalf("expected success, got: %v", errObj)
	}
	m, ok := result.(map[string]string)
	if !ok || m["status"] != "ok" {
		t.Errorf("expected status=ok, got: %v", result)
	}
}

func TestRPCSafetyExistingTestsUnchanged(t *testing.T) {
	server := &Server{}
	server.registerHandlers()

	critical := []string{
		"ai.chat", "browser.navigate", "browser.fill", "browser.click",
		"browser.status", "matrix.status", "matrix.login", "matrix.send",
		"matrix.join_room", "health.check",
	}
	for _, method := range critical {
		if _, ok := server.handlers[method]; !ok {
			t.Errorf("existing method %q still registered after safety middleware addition", method)
		}
	}
}
