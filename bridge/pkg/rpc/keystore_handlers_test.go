package rpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/armorclaw/bridge/pkg/keystore"
)

func testKeystoreParams(t *testing.T, method string, params interface{}) *Request {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	return &Request{JSONRPC: "2.0", ID: 1, Method: method, Params: raw}
}

func TestKeystoreHandlers_Disabled(t *testing.T) {
	s := &Server{zeroTrustKS: false}
	ctx := context.Background()

	disabledCases := []struct {
		name    string
		method  string
		handler func(context.Context, *Request) (interface{}, *ErrorObj)
		params  interface{}
	}{
		{"unseal", "keystore.unseal", s.handleKeystoreUnseal, map[string]string{"password": "x"}},
		{"sealed", "keystore.sealed", s.handleKeystoreSealed, nil},
		{"seal", "keystore.seal", s.handleKeystoreSeal, nil},
		{"extend_session", "keystore.extend_session", s.handleKeystoreExtendSession, nil},
		{"session_status", "keystore.session_status", s.handleKeystoreSessionStatus, nil},
		{"list_keys", "keystore.list_keys", s.handleKeystoreListKeys, nil},
		{"delete_key", "keystore.delete_key", s.handleKeystoreDeleteKey, map[string]string{"name": "test"}},
	}

	for _, tc := range disabledCases {
		t.Run(tc.name+"_disabled", func(t *testing.T) {
			var req *Request
			if tc.params != nil {
				req = testKeystoreParams(t, tc.method, tc.params)
			} else {
				req = &Request{JSONRPC: "2.0", ID: 1, Method: tc.method, Params: json.RawMessage(`{}`)}
			}

			result, err := tc.handler(ctx, req)
			if result != nil {
				t.Errorf("expected nil result, got %v", result)
			}
			if err == nil {
				t.Fatal("expected error when feature disabled")
			}
			if err.Code != MethodNotFound {
				t.Errorf("expected code %d, got %d", MethodNotFound, err.Code)
			}
			if err.Message != "Feature disabled: zero_trust_keystore" {
				t.Errorf("unexpected message: %s", err.Message)
			}
		})
	}
}

func TestKeystoreUnseal_RateLimited(t *testing.T) {
	limiter := keystore.NewRateLimiter(1, time.Minute)
	s := &Server{
		zeroTrustKS:    true,
		keystoreLimiter: limiter,
	}

	ctx := context.Background()

	limiter.Record("rpc:unknown")
	limiter.Record("rpc:unknown")

	req := testKeystoreParams(t, "keystore.unseal", map[string]string{"password": "test"})
	result, err := s.handleKeystoreUnseal(ctx, req)
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	if err.Code != KeystoreRateLimited {
		t.Errorf("expected code %d, got %d", KeystoreRateLimited, err.Code)
	}
}

func TestKeystoreUnseal_MissingPassword(t *testing.T) {
	s := &Server{zeroTrustKS: true}
	ctx := context.Background()

	req := testKeystoreParams(t, "keystore.unseal", map[string]string{"password": ""})
	result, err := s.handleKeystoreUnseal(ctx, req)
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
	if err == nil {
		t.Fatal("expected error for missing password")
	}
	if err.Code != InvalidParams {
		t.Errorf("expected code %d, got %d", InvalidParams, err.Code)
	}
}

func TestKeystoreDeleteKey_MissingName(t *testing.T) {
	s := &Server{zeroTrustKS: true}
	ctx := context.Background()

	req := testKeystoreParams(t, "keystore.delete_key", map[string]string{"name": ""})
	result, err := s.handleKeystoreDeleteKey(ctx, req)
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if err.Code != InvalidParams {
		t.Errorf("expected code %d, got %d", InvalidParams, err.Code)
	}
}

func TestKeystoreSealed_NilKS(t *testing.T) {
	s := &Server{zeroTrustKS: true, sealedKS: nil}
	ctx := context.Background()

	req := &Request{JSONRPC: "2.0", ID: 1, Method: "keystore.sealed", Params: json.RawMessage(`{}`)}
	result, err := s.handleKeystoreSealed(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}
	if m["sealed"] != true {
		t.Errorf("expected sealed=true when KS is nil, got %v", m["sealed"])
	}
}

func TestKeystoreSessionStatus_NilKS(t *testing.T) {
	s := &Server{zeroTrustKS: true, sealedKS: nil}
	ctx := context.Background()

	req := &Request{JSONRPC: "2.0", ID: 1, Method: "keystore.session_status", Params: json.RawMessage(`{}`)}
	result, err := s.handleKeystoreSessionStatus(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}
	if m["sealed"] != true {
		t.Errorf("expected sealed=true, got %v", m["sealed"])
	}
	if m["remaining_seconds"] != float64(0) {
		t.Errorf("expected remaining_seconds=0, got %v", m["remaining_seconds"])
	}
}

func TestKeystoreSeal_NilKS(t *testing.T) {
	s := &Server{zeroTrustKS: true, sealedKS: nil}
	ctx := context.Background()

	req := &Request{JSONRPC: "2.0", ID: 1, Method: "keystore.seal", Params: json.RawMessage(`{}`)}
	result, err := s.handleKeystoreSeal(ctx, req)
	if err == nil {
		t.Fatal("expected error when sealedKS nil")
	}
	if err.Code != InternalError {
		t.Errorf("expected code %d, got %d", InternalError, err.Code)
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestKeystoreHandlers_Registered(t *testing.T) {
	s, err := New(Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	methods := []string{
		"keystore.unseal",
		"keystore.sealed",
		"keystore.seal",
		"keystore.extend_session",
		"keystore.session_status",
		"keystore.list_keys",
		"keystore.delete_key",
	}

	for _, m := range methods {
		if _, ok := s.handlers[m]; !ok {
			t.Errorf("handler %q not registered", m)
		}
	}
}

func TestKeystoreHandlers_InvalidParams(t *testing.T) {
	s := &Server{zeroTrustKS: true}
	ctx := context.Background()

	req := &Request{JSONRPC: "2.0", ID: 1, Method: "keystore.unseal", Params: json.RawMessage(`invalid json`)}
	result, err := s.handleKeystoreUnseal(ctx, req)
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
	if err == nil || err.Code != InvalidParams {
		t.Errorf("expected InvalidParams, got %v", err)
	}

	req2 := &Request{JSONRPC: "2.0", ID: 2, Method: "keystore.delete_key", Params: json.RawMessage(`invalid json`)}
	result2, err2 := s.handleKeystoreDeleteKey(ctx, req2)
	if result2 != nil {
		t.Errorf("expected nil result, got %v", result2)
	}
	if err2 == nil || err2.Code != InvalidParams {
		t.Errorf("expected InvalidParams, got %v", err2)
	}
}
