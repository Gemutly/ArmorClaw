package rpc

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/armorclaw/bridge/pkg/keystore"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// testPasswordSealedKS creates a real password-gated SealedKeystore backed by
// a temp SQLCipher database. Skips the test if SQLCipher is unavailable.
func testPasswordSealedKS(t *testing.T) (*keystore.SealedKeystore, string) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "ks-contract-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	dbPath := filepath.Join(tmpDir, "keystore.db")

	baseKS, err := keystore.New(keystore.Config{DBPath: dbPath})
	if err != nil {
		t.Skipf("keystore.New failed (SQLCipher unavailable?): %v", err)
	}
	if err := baseKS.Open(); err != nil {
		t.Skipf("keystore.Open failed (SQLCipher unavailable?): %v", err)
	}
	t.Cleanup(func() { baseKS.Close() })

	password := "TestPass1"
	kd, err := keystore.NewKeyDerivation(keystore.DefaultKeyDerivationParams)
	if err != nil {
		t.Fatalf("key derivation: %v", err)
	}

	salt := make([]byte, keystore.DefaultKeyDerivationParams.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		t.Fatalf("salt: %v", err)
	}

	verifier, err := kd.DeriveKey([]byte(password), salt)
	if err != nil {
		t.Fatalf("derive verifier: %v", err)
	}

	vaultKey := make([]byte, 32)
	if _, err := rand.Read(vaultKey); err != nil {
		t.Fatalf("vault key: %v", err)
	}

	wrapped, err := kd.WrapKey(vaultKey, []byte(password))
	if err != nil {
		t.Fatalf("wrap vault key: %v", err)
	}

	sk, err := keystore.NewSealedKeystoreWithPassword(baseKS, keystore.SealedStoreConfig{
		PasswordVerifier: verifier.Key,
		VerifySalt:       salt,
		WrappedVaultKey:  *wrapped,
	})
	if err != nil {
		t.Fatalf("NewSealedKeystoreWithPassword: %v", err)
	}

	sk.SetAutoSealDuration(1 * time.Hour)

	return sk, password
}

func contractReq(t *testing.T, method string, params interface{}) *Request {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return &Request{JSONRPC: "2.0", ID: 1, Method: method, Params: raw}
}

// ---------------------------------------------------------------------------
// Dispatch helper
// ---------------------------------------------------------------------------

func dispatchKeystoreHandler(s *Server, method string, ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	switch method {
	case "keystore.unseal":
		return s.handleKeystoreUnseal(ctx, req)
	case "keystore.sealed":
		return s.handleKeystoreSealed(ctx, req)
	case "keystore.seal":
		return s.handleKeystoreSeal(ctx, req)
	case "keystore.extend_session":
		return s.handleKeystoreExtendSession(ctx, req)
	case "keystore.session_status":
		return s.handleKeystoreSessionStatus(ctx, req)
	case "keystore.list_keys":
		return s.handleKeystoreListKeys(ctx, req)
	case "keystore.delete_key":
		return s.handleKeystoreDeleteKey(ctx, req)
	default:
		return nil, &ErrorObj{Code: MethodNotFound, Message: fmt.Sprintf("unknown method: %s", method)}
	}
}

// ---------------------------------------------------------------------------
// State 1: DISABLED (zeroTrustKS=false)
// ---------------------------------------------------------------------------

func TestKeystoreContract_Disabled(t *testing.T) {
	s := &Server{zeroTrustKS: false}
	ctx := context.Background()

	methods := []struct {
		name   string
		method string
		params interface{}
	}{
		{"unseal", "keystore.unseal", map[string]string{"password": "x"}},
		{"sealed", "keystore.sealed", nil},
		{"seal", "keystore.seal", nil},
		{"extend_session", "keystore.extend_session", nil},
		{"session_status", "keystore.session_status", nil},
		{"list_keys", "keystore.list_keys", nil},
		{"delete_key", "keystore.delete_key", map[string]string{"name": "test"}},
	}

	for _, tc := range methods {
		t.Run(tc.name+"_disabled", func(t *testing.T) {
			var req *Request
			if tc.params != nil {
				req = contractReq(t, tc.method, tc.params)
			} else {
				req = &Request{JSONRPC: "2.0", ID: 1, Method: tc.method, Params: json.RawMessage(`{}`)}
			}

			result, err := dispatchKeystoreHandler(s, tc.method, ctx, req)

			if result != nil {
				t.Errorf("expected nil result, got %v", result)
			}
			if err == nil {
				t.Fatal("expected error for disabled feature")
			}
			if err.Code != MethodNotFound {
				t.Errorf("expected code %d (MethodNotFound), got %d", MethodNotFound, err.Code)
			}
			if err.Message != "Feature disabled: zero_trust_keystore" {
				t.Errorf("unexpected message: %s", err.Message)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// State 2: SEALED (vault is sealed, password not yet provided)
// ---------------------------------------------------------------------------

func TestKeystoreContract_Sealed(t *testing.T) {
	sk, _ := testPasswordSealedKS(t)
	limiter := keystore.NewRateLimiter(100, time.Minute)

	s := &Server{
		zeroTrustKS:     true,
		sealedKS:        sk,
		keystoreLimiter: limiter,
	}
	ctx := context.Background()

	cases := []struct {
		name         string
		method       string
		params       interface{}
		wantErrCode  int
		wantErrMatch string
	}{
		{
			name: "unseal_missing_password", method: "keystore.unseal",
			params: map[string]string{"password": ""},
			wantErrCode: InvalidParams,
		},
		{
			name: "sealed_returns_true", method: "keystore.sealed",
			params: map[string]string{},
			wantErrCode: 0,
		},
		{
			name: "seal_succeeds_idempotent", method: "keystore.seal",
			params: map[string]string{},
			wantErrCode: 0,
		},
		{
			name: "extend_session_sealed_error", method: "keystore.extend_session",
			params: map[string]string{},
			wantErrCode: -32005, wantErrMatch: "sealed",
		},
		{
			name: "session_status_sealed", method: "keystore.session_status",
			params: map[string]string{},
			wantErrCode: 0,
		},
		{
			name: "list_keys_sealed_error", method: "keystore.list_keys",
			params: map[string]string{},
			wantErrCode: -32005, wantErrMatch: "sealed",
		},
		{
			name: "delete_key_sealed_error", method: "keystore.delete_key",
			params: map[string]string{"name": "nonexistent"},
			wantErrCode: -32005, wantErrMatch: "sealed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req *Request
			if tc.params != nil {
				req = contractReq(t, tc.method, tc.params)
			} else {
				req = &Request{JSONRPC: "2.0", ID: 1, Method: tc.method, Params: json.RawMessage(`{}`)}
			}

			result, err := dispatchKeystoreHandler(s, tc.method, ctx, req)

			if tc.wantErrCode != 0 {
				if err == nil {
					t.Fatalf("expected error code %d, got nil (result=%v)", tc.wantErrCode, result)
				}
				if err.Code != tc.wantErrCode {
					t.Errorf("expected code %d, got %d (msg: %s)", tc.wantErrCode, err.Code, err.Message)
				}
				if tc.wantErrMatch != "" && !containsSubstring(err.Message, tc.wantErrMatch) {
					t.Errorf("error message %q should contain %q", err.Message, tc.wantErrMatch)
				}
			} else {
				if err != nil {
					t.Fatalf("expected success, got error: code=%d msg=%s", err.Code, err.Message)
				}
				if result == nil {
					t.Fatal("expected non-nil result")
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// State 3: UNSEALED (after successful password unseal)
// ---------------------------------------------------------------------------

func TestKeystoreContract_Unsealed(t *testing.T) {
	sk, password := testPasswordSealedKS(t)
	limiter := keystore.NewRateLimiter(100, time.Minute)

	s := &Server{
		zeroTrustKS:     true,
		sealedKS:        sk,
		keystoreLimiter: limiter,
	}
	ctx := context.Background()

	if err := sk.UnsealWithPassword(password); err != nil {
		t.Fatalf("setup unseal failed: %v", err)
	}

	cases := []struct {
		name        string
		method      string
		params      interface{}
		wantErrCode int
		resultCheck func(t *testing.T, result interface{})
	}{
		{
			name: "unseal_already_unsealed", method: "keystore.unseal",
			params: map[string]string{"password": password},
			wantErrCode: -32003,
		},
		{
			name: "sealed_returns_false", method: "keystore.sealed",
			params: map[string]string{},
			resultCheck: func(t *testing.T, result interface{}) {
				m := result.(map[string]interface{})
				if m["sealed"] == true {
					t.Error("expected sealed=false when unsealed")
				}
			},
		},
		{
			name: "seal_succeeds", method: "keystore.seal",
			params: map[string]string{},
			resultCheck: func(t *testing.T, result interface{}) {
				m := result.(map[string]interface{})
				if m["sealed"] != true {
					t.Error("expected sealed=true after seal")
				}
			},
		},
		{
			name: "extend_session_succeeds", method: "keystore.extend_session",
			params: map[string]string{},
			resultCheck: func(t *testing.T, result interface{}) {
				m := result.(map[string]interface{})
				if m["extended"] != true {
					t.Error("expected extended=true")
				}
			},
		},
		{
			name: "session_status_unsealed", method: "keystore.session_status",
			params: map[string]string{},
			resultCheck: func(t *testing.T, result interface{}) {
				m := result.(map[string]interface{})
				if m["sealed"] == true {
					t.Error("expected sealed=false in session_status")
				}
			},
		},
		{
			name: "list_keys_empty", method: "keystore.list_keys",
			params: map[string]string{},
			resultCheck: func(t *testing.T, result interface{}) {
				m := result.(map[string]interface{})
				if _, ok := m["keys"]; !ok {
					t.Fatal("expected 'keys' field")
				}
			},
		},
		{
			name: "delete_key_nonexistent", method: "keystore.delete_key",
			params: map[string]string{"name": "nonexistent-key"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "seal_succeeds" && !sk.IsPasswordUnsealed() {
				if err := sk.UnsealWithPassword(password); err != nil {
					t.Fatalf("re-unseal: %v", err)
				}
			}

			var req *Request
			if tc.params != nil {
				req = contractReq(t, tc.method, tc.params)
			} else {
				req = &Request{JSONRPC: "2.0", ID: 1, Method: tc.method, Params: json.RawMessage(`{}`)}
			}

			result, err := dispatchKeystoreHandler(s, tc.method, ctx, req)

			if tc.wantErrCode != 0 {
				if err == nil {
					t.Fatalf("expected error code %d, got nil (result=%v)", tc.wantErrCode, result)
				}
				if err.Code != tc.wantErrCode {
					t.Errorf("expected code %d, got %d (msg: %s)", tc.wantErrCode, err.Code, err.Message)
				}
			} else {
				if err != nil {
					t.Fatalf("expected success, got error: code=%d msg=%s", err.Code, err.Message)
				}
				if result == nil {
					t.Fatal("expected non-nil result")
				}
			}

			if tc.resultCheck != nil {
				tc.resultCheck(t, result)
			}

			if tc.name == "seal_succeeds" {
				if err := sk.UnsealWithPassword(password); err != nil {
					t.Fatalf("re-unseal after seal: %v", err)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Password-specific subtests
// ---------------------------------------------------------------------------

func TestKeystoreContract_UnsealCorrectPassword(t *testing.T) {
	sk, password := testPasswordSealedKS(t)
	limiter := keystore.NewRateLimiter(100, time.Minute)

	s := &Server{
		zeroTrustKS:     true,
		sealedKS:        sk,
		keystoreLimiter: limiter,
	}
	ctx := context.Background()

	req := contractReq(t, "keystore.unseal", map[string]string{"password": password})
	result, err := s.handleKeystoreUnseal(ctx, req)

	if err != nil {
		t.Fatalf("expected successful unseal, got: code=%d msg=%s", err.Code, err.Message)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}
	if m["unsealed"] != true {
		t.Error("expected unsealed=true")
	}
	if !sk.IsPasswordUnsealed() {
		t.Error("keystore should report unsealed")
	}
}

func TestKeystoreContract_UnsealWrongPassword(t *testing.T) {
	sk, _ := testPasswordSealedKS(t)
	limiter := keystore.NewRateLimiter(100, time.Minute)

	s := &Server{
		zeroTrustKS:     true,
		sealedKS:        sk,
		keystoreLimiter: limiter,
	}
	ctx := context.Background()

	req := contractReq(t, "keystore.unseal", map[string]string{"password": "WrongPass1"})
	result, err := s.handleKeystoreUnseal(ctx, req)

	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	if err.Code != -32001 {
		t.Errorf("expected code -32001 (ErrInvalidPassword), got %d (msg: %s)", err.Code, err.Message)
	}
	if sk.IsPasswordUnsealed() {
		t.Error("keystore should remain sealed after wrong password")
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
