package rpc

import (
	"context"
	"encoding/json"
	"testing"
)

// authMockHandler returns a success response. If the handler is reached,
// the auth middleware allowed the call through.
func authMockHandler(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	return map[string]string{"status": "ok"}, nil
}

// assertAuthError verifies that a wrapped handler returns the expected auth error code.
func assertAuthError(t *testing.T, method string, errObj *ErrorObj, expectedCode int) {
	t.Helper()
	if errObj == nil {
		t.Fatalf("%s: expected auth error, got nil", method)
	}
	if errObj.Code != expectedCode {
		t.Errorf("%s: expected error code %d, got %d (%s)", method, expectedCode, errObj.Code, errObj.Message)
	}
}

// ---------------------------------------------------------------------------
// Test 1: Browser handlers require auth
// ---------------------------------------------------------------------------

func TestBrowserHandlersRequireAuth(t *testing.T) {
	mw := NewBEATOSafetyMiddleware("test-token")

	methods := []struct {
		method string
		group  RPCGroup
	}{
		{"browser.navigate", BrowserRPCGroup},
		{"browser.fill", BrowserRPCGroup},
		{"browser.click", BrowserRPCGroup},
		{"browser.status", BrowserRPCGroup},
		{"browser.wait_for_element", BrowserRPCGroup},
		{"browser.wait_for_captcha", BrowserRPCGroup},
		{"browser.wait_for_2fa", BrowserRPCGroup},
		{"browser.complete", BrowserRPCGroup},
		{"browser.fail", BrowserRPCGroup},
		{"browser.list", BrowserRPCGroup},
		{"browser.cancel", BrowserRPCGroup},
		{"browser.replay_diagnostics", BrowserRPCGroup},
	}

	for _, tc := range methods {
		t.Run(tc.method, func(t *testing.T) {
			wrapped := mw.WrapForGroup(tc.group, authMockHandler)
			req := &Request{
				Method: tc.method,
				Params: json.RawMessage(`{}`),
			}
			_, errObj := wrapped(context.Background(), req)
			assertAuthError(t, tc.method, errObj, RPCAuthRequired)
		})
	}
}

// ---------------------------------------------------------------------------
// Test 2: Document handlers require auth
// ---------------------------------------------------------------------------

func TestDocumentHandlersRequireAuth(t *testing.T) {
	mw := NewBEATOSafetyMiddleware("test-token")

	methods := []struct {
		method string
		group  RPCGroup
	}{
		{"document.extract_text", DocumentRPCGroup},
		{"document.status", DocumentRPCGroup},
		{"document.list_jobs", DocumentRPCGroup},
	}

	for _, tc := range methods {
		t.Run(tc.method, func(t *testing.T) {
			wrapped := mw.WrapForGroup(tc.group, authMockHandler)
			req := &Request{
				Method: tc.method,
				Params: json.RawMessage(`{}`),
			}
			_, errObj := wrapped(context.Background(), req)
			assertAuthError(t, tc.method, errObj, RPCAuthRequired)
		})
	}
}

// ---------------------------------------------------------------------------
// Test 3: Email handlers require auth
// ---------------------------------------------------------------------------

func TestEmailHandlersRequireAuth(t *testing.T) {
	mw := NewBEATOSafetyMiddleware("test-token")

	methods := []struct {
		method string
		group  RPCGroup
	}{
		{"email.queue_status", EmailRPCGroup},
		{"email.get", EmailRPCGroup},
		{"email.retry", EmailRPCGroup},
		{"email.list", EmailRPCGroup},
	}

	for _, tc := range methods {
		t.Run(tc.method, func(t *testing.T) {
			wrapped := mw.WrapForGroup(tc.group, authMockHandler)
			req := &Request{
				Method: tc.method,
				Params: json.RawMessage(`{}`),
			}
			_, errObj := wrapped(context.Background(), req)
			assertAuthError(t, tc.method, errObj, RPCAuthRequired)
		})
	}
}

// ---------------------------------------------------------------------------
// Test 4: Email approval handlers require auth
// ---------------------------------------------------------------------------

func TestEmailApprovalHandlersRequireAuth(t *testing.T) {
	mw := NewBEATOSafetyMiddleware("test-token")

	methods := []struct {
		method string
		group  RPCGroup
	}{
		{"approve_email", EmailRPCGroup},
		{"deny_email", EmailRPCGroup},
		{"email_approval_status", EmailRPCGroup},
		{"email.list_pending", EmailRPCGroup},
	}

	for _, tc := range methods {
		t.Run(tc.method, func(t *testing.T) {
			wrapped := mw.WrapForGroup(tc.group, authMockHandler)
			req := &Request{
				Method: tc.method,
				Params: json.RawMessage(`{}`),
			}
			_, errObj := wrapped(context.Background(), req)
			assertAuthError(t, tc.method, errObj, RPCAuthRequired)
		})
	}
}

// ---------------------------------------------------------------------------
// Test 5: Excluded handlers do NOT require auth
// ---------------------------------------------------------------------------

func TestExcludedHandlersDoNotRequireAuth(t *testing.T) {
	// Use a real Server to verify that excluded handlers were NOT wrapped
	// with SafetyMiddleware during registerHandlers().
	server := &Server{}
	server.registerHandlers()

	excludedMethods := []string{
		"health.check",
		"hardening.status",
		"hardening.ack",
		"hardening.rotate_password",
		"matrix.status",
		"ai.chat",
		"provisioning.start",
		"provisioning.claim",
	}

	for _, method := range excludedMethods {
		t.Run(method, func(t *testing.T) {
			handler, ok := server.handlers[method]
			if !ok {
				t.Fatalf("%s: handler not registered", method)
			}

			// Call without token. If the handler was wrapped with auth middleware,
			// it would return RPCAuthRequired (-32010). Excluded handlers should
			// NOT return auth errors — they may return business errors but not -32010.
			req := &Request{
				Method: method,
				Params: json.RawMessage(`{}`),
			}
			_, errObj := handler(context.Background(), req)
			if errObj != nil && errObj.Code == RPCAuthRequired {
				t.Errorf("%s: excluded handler returned auth error (was wrapped), got code %d", method, errObj.Code)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test 6: Valid token allows BEATO handlers through
// ---------------------------------------------------------------------------

func TestValidTokenAllowsBEATOHandlers(t *testing.T) {
	mw := NewBEATOSafetyMiddleware("test-token")

	methods := []struct {
		method string
		group  RPCGroup
	}{
		{"browser.navigate", BrowserRPCGroup},
		{"document.extract_text", DocumentRPCGroup},
		{"email.queue_status", EmailRPCGroup},
		{"approve_email", EmailRPCGroup},
	}

	for _, tc := range methods {
		t.Run(tc.method, func(t *testing.T) {
			wrapped := mw.WrapForGroup(tc.group, authMockHandler)
			req := &Request{
				Method: tc.method,
				Params: json.RawMessage(`{"token":"test-token"}`),
			}
			result, errObj := wrapped(context.Background(), req)
			if errObj != nil {
				t.Fatalf("%s: expected success with valid token, got error: code=%d msg=%s",
					tc.method, errObj.Code, errObj.Message)
			}
			m, ok := result.(map[string]string)
			if !ok || m["status"] != "ok" {
				t.Errorf("%s: expected status=ok, got: %v", tc.method, result)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test 7: Invalid token is rejected
// ---------------------------------------------------------------------------

func TestInvalidTokenRejected(t *testing.T) {
	mw := NewBEATOSafetyMiddleware("test-token")

	methods := []struct {
		method string
		group  RPCGroup
	}{
		{"browser.navigate", BrowserRPCGroup},
		{"document.extract_text", DocumentRPCGroup},
		{"email.queue_status", EmailRPCGroup},
		{"approve_email", EmailRPCGroup},
	}

	for _, tc := range methods {
		t.Run(tc.method, func(t *testing.T) {
			wrapped := mw.WrapForGroup(tc.group, authMockHandler)
			req := &Request{
				Method: tc.method,
				Params: json.RawMessage(`{"token":"wrong-token"}`),
			}
			_, errObj := wrapped(context.Background(), req)
			assertAuthError(t, tc.method, errObj, RPCAuthForbidden)
		})
	}
}
