package rpc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/armorclaw/bridge/pkg/lockdown"
)

func newTestLockdownServer(t *testing.T) (*Server, *lockdown.Manager, *lockdown.BondingManager) {
	t.Helper()
	stateFile := filepath.Join(t.TempDir(), "lockdown.json")
	mgr, err := lockdown.NewManager(lockdown.Config{StateFile: stateFile})
	if err != nil {
		t.Fatalf("create lockdown manager: %v", err)
	}
	bondingMgr := lockdown.NewBondingManager(mgr)

	server := &Server{
		lockdownMgr: mgr,
		bondingMgr:  bondingMgr,
	}
	server.registerHandlers()
	return server, mgr, bondingMgr
}

func TestLockdownStatus(t *testing.T) {
	server, _, _ := newTestLockdownServer(t)

	handler := server.handlers["lockdown.status"]
	if handler == nil {
		t.Fatal("lockdown.status handler not registered")
	}

	result, errObj := handler(context.Background(), &Request{})
	if errObj != nil {
		t.Fatalf("unexpected error: %v", errObj)
	}

	status, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result)
	}

	for _, key := range []string{"mode", "admin_established", "single_device_mode", "setup_complete", "security_configured", "keystore_initialized"} {
		if _, exists := status[key]; !exists {
			t.Errorf("missing key %q in status response", key)
		}
	}
}

func TestLockdownGetChallenge(t *testing.T) {
	server, _, _ := newTestLockdownServer(t)

	handler := server.handlers["lockdown.get_challenge"]
	if handler == nil {
		t.Fatal("lockdown.get_challenge handler not registered")
	}

	result, errObj := handler(context.Background(), &Request{})
	if errObj != nil {
		t.Fatalf("unexpected error: %v", errObj)
	}

	challenge, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result)
	}

	for _, key := range []string{"nonce", "created_at", "expires_at"} {
		if _, exists := challenge[key]; !exists {
			t.Errorf("missing key %q in challenge response", key)
		}
	}
}

func TestLockdownClaimOwnership(t *testing.T) {
	server, _, _ := newTestLockdownServer(t)

	handler := server.handlers["lockdown.claim_ownership"]
	if handler == nil {
		t.Fatal("lockdown.claim_ownership handler not registered")
	}

	params := map[string]string{
		"display_name":           "TestAdmin",
		"device_name":            "TestDevice",
		"device_fingerprint":     "abc123def456",
		"passphrase_commitment":  "sha256hash_of_passphrase_at_least_16ch",
		"challenge_response":     "",
	}
	raw, _ := json.Marshal(params)

	result, errObj := handler(context.Background(), &Request{Params: raw})
	if errObj != nil {
		t.Fatalf("unexpected error: %+v", errObj)
	}

	resp, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result)
	}

	for _, key := range []string{"status", "admin_id", "session_token", "next_step"} {
		if _, exists := resp[key]; !exists {
			t.Errorf("missing key %q in claim response", key)
		}
	}
}

func TestLockdownClaimOwnershipAlreadyClaimed(t *testing.T) {
	server, _, _ := newTestLockdownServer(t)

	handler := server.handlers["lockdown.claim_ownership"]

	params := map[string]string{
		"display_name":          "Admin1",
		"device_name":           "Device1",
		"device_fingerprint":    "fp1",
		"passphrase_commitment": "sha256hash_long_enough_value",
	}
	raw, _ := json.Marshal(params)

	// First claim succeeds
	_, errObj := handler(context.Background(), &Request{Params: raw})
	if errObj != nil {
		t.Fatalf("first claim should succeed: %+v", errObj)
	}

	// Second claim fails
	params["display_name"] = "Admin2"
	raw2, _ := json.Marshal(params)
	result, errObj := handler(context.Background(), &Request{Params: raw2})
	if result != nil {
		t.Errorf("expected nil result on second claim, got %v", result)
	}
	if errObj == nil {
		t.Fatal("expected error on second claim")
	}
	if errObj.Code != InvalidParams {
		t.Errorf("expected InvalidParams code, got %d", errObj.Code)
	}
}

func TestLockdownTransitionRequiresAdmin(t *testing.T) {
	server, _, _ := newTestLockdownServer(t)

	handler := server.handlers["lockdown.transition"]
	if handler == nil {
		t.Fatal("lockdown.transition handler not registered")
	}

	raw, _ := json.Marshal(map[string]string{"target": "operational"})
	result, errObj := handler(context.Background(), &Request{Params: raw})
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
	if errObj == nil {
		t.Fatal("expected error when admin not established")
	}
	if errObj.Code != InvalidRequest {
		t.Errorf("expected InvalidRequest code, got %d", errObj.Code)
	}
	if errObj.Message != "admin not established" {
		t.Errorf("expected 'admin not established', got '%s'", errObj.Message)
	}
}

func TestLockdownNotConfigured(t *testing.T) {
	server := &Server{}
	server.registerHandlers()

	methods := []struct {
		name   string
		params json.RawMessage
	}{
		{"lockdown.status", nil},
		{"lockdown.get_challenge", nil},
		{"lockdown.transition", json.RawMessage(`{"target":"operational"}`)},
	}

	for _, m := range methods {
		t.Run(m.name, func(t *testing.T) {
			handler := server.handlers[m.name]
			if handler == nil {
				t.Fatalf("%s handler not registered", m.name)
			}
			result, errObj := handler(context.Background(), &Request{Params: m.params})
			if result != nil {
				t.Errorf("expected nil result, got %v", result)
			}
			if errObj == nil {
				t.Fatal("expected error when lockdown not configured")
			}
			if errObj.Code != InternalError {
				t.Errorf("expected InternalError code, got %d", errObj.Code)
			}
		})
	}
}

func TestLockdownNotConfiguredClaim(t *testing.T) {
	server := &Server{}
	server.registerHandlers()

	handler := server.handlers["lockdown.claim_ownership"]
	if handler == nil {
		t.Fatal("lockdown.claim_ownership handler not registered")
	}

	raw, _ := json.Marshal(map[string]string{
		"display_name":          "Test",
		"device_name":           "Dev",
		"device_fingerprint":    "fp",
		"passphrase_commitment": "sha256hashlongenough",
	})
	result, errObj := handler(context.Background(), &Request{Params: raw})
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
	if errObj == nil {
		t.Fatal("expected error when bonding not configured")
	}
	if errObj.Code != InternalError {
		t.Errorf("expected InternalError code, got %d", errObj.Code)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
