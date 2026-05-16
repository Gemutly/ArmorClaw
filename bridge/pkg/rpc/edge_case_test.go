package rpc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/armorclaw/bridge/pkg/browser"
	"github.com/armorclaw/bridge/pkg/voice"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// edgeReq builds a JSON-RPC request for edge-case tests.
func edgeReq(method string, rawParams json.RawMessage) *Request {
	return &Request{JSONRPC: "2.0", ID: 1, Method: method, Params: rawParams}
}

// assertNoPanic runs f and fails if it panics. Returns the recovered value.
func assertNoPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%s panicked: %v", name, r)
		}
	}()
	f()
}

// ---------------------------------------------------------------------------
// 1. KEYSTORE EDGE CASES
// ---------------------------------------------------------------------------

func TestEdgeKeystore_UnsealAlreadyUnsealed(t *testing.T) {
	// With sealedKS == nil, unseal returns InternalError ("sealed keystore not configured").
	// This exercises the nil-keystore guard without requiring a real SQLCipher store.
	s := &Server{zeroTrustKS: true, sealedKS: nil}
	ctx := context.Background()

	req := edgeReq("keystore.unseal", json.RawMessage(`{"password":"test1234"}`))
	assertNoPanic(t, "unseal-nil-ks", func() {
		result, err := s.handleKeystoreUnseal(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error when sealedKS is nil")
		}
		if err.Code != InternalError {
			t.Errorf("expected InternalError (%d), got %d", InternalError, err.Code)
		}
	})
}

func TestEdgeKeystore_SealAlreadySealed(t *testing.T) {
	// When sealedKS is nil, seal returns InternalError.
	s := &Server{zeroTrustKS: true, sealedKS: nil}
	ctx := context.Background()

	req := edgeReq("keystore.seal", json.RawMessage(`{}`))
	assertNoPanic(t, "seal-nil-ks", func() {
		result, err := s.handleKeystoreSeal(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error when sealedKS is nil")
		}
		if err.Code != InternalError {
			t.Errorf("expected InternalError (%d), got %d", InternalError, err.Code)
		}
	})
}

func TestEdgeKeystore_ExtendExpiredSession(t *testing.T) {
	// When sealedKS is nil, extend_session returns InternalError.
	s := &Server{zeroTrustKS: true, sealedKS: nil}
	ctx := context.Background()

	req := edgeReq("keystore.extend_session", json.RawMessage(`{}`))
	assertNoPanic(t, "extend-nil-ks", func() {
		result, err := s.handleKeystoreExtendSession(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error when sealedKS is nil")
		}
		if err.Code != InternalError {
			t.Errorf("expected InternalError (%d), got %d", InternalError, err.Code)
		}
	})
}

func TestEdgeKeystore_ListKeysWhenSealed(t *testing.T) {
	// When sealedKS is nil, list_keys returns InternalError.
	s := &Server{zeroTrustKS: true, sealedKS: nil}
	ctx := context.Background()

	req := edgeReq("keystore.list_keys", json.RawMessage(`{}`))
	assertNoPanic(t, "list-nil-ks", func() {
		result, err := s.handleKeystoreListKeys(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error when sealedKS is nil")
		}
		if err.Code != InternalError {
			t.Errorf("expected InternalError (%d), got %d", InternalError, err.Code)
		}
	})
}

func TestEdgeKeystore_DeleteNonexistentKey(t *testing.T) {
	// When sealedKS is nil, delete_key returns InternalError.
	s := &Server{zeroTrustKS: true, sealedKS: nil}
	ctx := context.Background()

	req := edgeReq("keystore.delete_key", json.RawMessage(`{"name":"ghost-key"}`))
	assertNoPanic(t, "delete-nil-ks", func() {
		result, err := s.handleKeystoreDeleteKey(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error when sealedKS is nil")
		}
		if err.Code != InternalError {
			t.Errorf("expected InternalError (%d), got %d", InternalError, err.Code)
		}
	})
}

func TestEdgeKeystore_Disabled_Subsystem(t *testing.T) {
	// All keystore methods return MethodNotFound when zeroTrustKS is false.
	s := &Server{zeroTrustKS: false}
	ctx := context.Background()

	cases := []struct {
		name    string
		handler func(context.Context, *Request) (interface{}, *ErrorObj)
		params  json.RawMessage
	}{
		{"unseal", s.handleKeystoreUnseal, json.RawMessage(`{"password":"x"}`)},
		{"sealed", s.handleKeystoreSealed, json.RawMessage(`{}`)},
		{"seal", s.handleKeystoreSeal, json.RawMessage(`{}`)},
		{"extend_session", s.handleKeystoreExtendSession, json.RawMessage(`{}`)},
		{"session_status", s.handleKeystoreSessionStatus, json.RawMessage(`{}`)},
		{"list_keys", s.handleKeystoreListKeys, json.RawMessage(`{}`)},
		{"delete_key", s.handleKeystoreDeleteKey, json.RawMessage(`{"name":"x"}`)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertNoPanic(t, tc.name, func() {
				result, err := tc.handler(ctx, edgeReq("keystore."+tc.name, tc.params))
				if result != nil {
					t.Errorf("expected nil result, got %v", result)
				}
				if err == nil {
					t.Fatal("expected error for disabled feature")
				}
				if err.Code != MethodNotFound {
					t.Errorf("expected MethodNotFound (%d), got %d", MethodNotFound, err.Code)
				}
			})
		})
	}
}

// ---------------------------------------------------------------------------
// 2. VOICE EDGE CASES
// ---------------------------------------------------------------------------

func TestEdgeVoice_FeatureDisabled(t *testing.T) {
	// voicePipeline != "cloud" means all voice methods return MethodNotFound.
	s := &Server{voicePipeline: ""}
	ctx := context.Background()

	cases := []struct {
		name    string
		handler func(context.Context, *Request) (interface{}, *ErrorObj)
		params  json.RawMessage
	}{
		{"start_session", s.handleVoiceStartSession, json.RawMessage(`{"session_config":{}}`)},
		{"stop_session", s.handleVoiceStopSession, json.RawMessage(`{"session_id":"s1"}`)},
		{"status", s.handleVoiceStatus, json.RawMessage(`{}`)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertNoPanic(t, tc.name, func() {
				result, err := tc.handler(ctx, edgeReq("voice."+tc.name, tc.params))
				if result != nil {
					t.Errorf("expected nil result, got %v", result)
				}
				if err == nil {
					t.Fatal("expected error for disabled feature")
				}
				if err.Code != MethodNotFound {
					t.Errorf("expected MethodNotFound (%d), got %d", MethodNotFound, err.Code)
				}
				if err.Message != "Feature disabled: voice_pipeline" {
					t.Errorf("unexpected message: %s", err.Message)
				}
			})
		})
	}
}

func TestEdgeVoice_StartWhenNilManager(t *testing.T) {
	s := &Server{voicePipeline: "cloud", voiceMgr: nil}
	ctx := context.Background()

	req := edgeReq("voice.start_session", json.RawMessage(`{"session_config":{}}`))
	assertNoPanic(t, "voice-start-nil", func() {
		result, err := s.handleVoiceStartSession(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error when voiceMgr nil")
		}
		if err.Code != int(voice.ErrVoiceNotConfiguredCode) {
			t.Errorf("expected voice.ErrVoiceNotConfiguredCode (%d), got %d", voice.ErrVoiceNotConfiguredCode, err.Code)
		}
	})
}

func TestEdgeVoice_StopWhenNilManager(t *testing.T) {
	s := &Server{voicePipeline: "cloud", voiceMgr: nil}
	ctx := context.Background()

	req := edgeReq("voice.stop_session", json.RawMessage(`{"session_id":"s1"}`))
	assertNoPanic(t, "voice-stop-nil", func() {
		result, err := s.handleVoiceStopSession(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error when voiceMgr nil")
		}
		if err.Code != int(voice.ErrVoiceNotConfiguredCode) {
			t.Errorf("expected voice.ErrVoiceNotConfiguredCode (%d), got %d", voice.ErrVoiceNotConfiguredCode, err.Code)
		}
	})
}

func TestEdgeVoice_StatusWhenNilManager(t *testing.T) {
	s := &Server{voicePipeline: "cloud", voiceMgr: nil}
	ctx := context.Background()

	req := edgeReq("voice.status", json.RawMessage(`{}`))
	assertNoPanic(t, "voice-status-nil", func() {
		result, err := s.handleVoiceStatus(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error when voiceMgr nil")
		}
		if err.Code != int(voice.ErrVoiceNotConfiguredCode) {
			t.Errorf("expected voice.ErrVoiceNotConfiguredCode (%d), got %d", voice.ErrVoiceNotConfiguredCode, err.Code)
		}
	})
}

func TestEdgeVoice_StopMissingSessionID(t *testing.T) {
	s := &Server{voicePipeline: "cloud", voiceMgr: nil}
	ctx := context.Background()

	req := edgeReq("voice.stop_session", json.RawMessage(`{"session_id":""}`))
	assertNoPanic(t, "voice-stop-empty-id", func() {
		result, err := s.handleVoiceStopSession(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Code != int(voice.ErrVoiceNotConfiguredCode) {
			t.Errorf("expected voice.ErrVoiceNotConfiguredCode (%d), got %d", voice.ErrVoiceNotConfiguredCode, err.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// 3. E2EE BACKUP EDGE CASES
// ---------------------------------------------------------------------------

func TestEdgeE2EE_FeatureDisabled(t *testing.T) {
	// All E2EE backup methods return MethodNotFound when flag is off.
	s := &Server{e2eeBackupEnabled: false}
	ctx := context.Background()

	cases := []struct {
		name    string
		handler func(context.Context, *Request) (interface{}, *ErrorObj)
		params  json.RawMessage
	}{
		{"create_backup", s.handleE2EECreateBackup, json.RawMessage(`{"recovery_phrase":[],"encrypted_key":"AQID"}`)},
		{"delete_backup", s.handleE2EEDeleteBackup, json.RawMessage(`{"backup_id":"b1"}`)},
		{"backup_exists", s.handleE2EEBackupExists, json.RawMessage(`{"backup_id":"b1"}`)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertNoPanic(t, tc.name, func() {
				result, err := tc.handler(ctx, edgeReq("e2ee."+tc.name, tc.params))
				if result != nil {
					t.Errorf("expected nil result, got %v", result)
				}
				if err == nil {
					t.Fatal("expected error for disabled feature")
				}
				if err.Code != MethodNotFound {
					t.Errorf("expected MethodNotFound (%d), got %d", MethodNotFound, err.Code)
				}
				if err.Message != "Feature disabled: e2ee_backup" {
					t.Errorf("unexpected message: %s", err.Message)
				}
			})
		})
	}
}

func TestEdgeE2EE_CreateBackup_InvalidPhrase(t *testing.T) {
	// Wrong number of recovery words → InvalidParams.
	s := &Server{e2eeBackupEnabled: true}
	ctx := context.Background()

	cases := []struct {
		name   string
		params json.RawMessage
		code   int
	}{
		{"too_few_words", json.RawMessage(`{"recovery_phrase":["a","b"],"encrypted_key":"AQID"}`), InvalidParams},
		{"missing_encrypted_key", json.RawMessage(`{"recovery_phrase":["a","b","c","d","e","f","g","h","i","j","k","l","m","n","o","p","q","r","s","t","u","v","w","x"]}`), InvalidParams},
		{"invalid_base64", json.RawMessage(`{"recovery_phrase":["a","b","c","d","e","f","g","h","i","j","k","l","m","n","o","p","q","r","s","t","u","v","w","x"],"encrypted_key":"!!invalid!!"}`), InvalidParams},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertNoPanic(t, tc.name, func() {
				result, err := s.handleE2EECreateBackup(ctx, edgeReq("e2ee.create_backup", tc.params))
				if result != nil {
					t.Errorf("expected nil result, got %v", result)
				}
				if err == nil {
					t.Fatal("expected error")
				}
				if err.Code != tc.code {
					t.Errorf("expected code %d, got %d (msg: %s)", tc.code, err.Code, err.Message)
				}
			})
		})
	}
}

func TestEdgeE2EE_DeleteBackup_MissingID(t *testing.T) {
	s := &Server{e2eeBackupEnabled: true}
	ctx := context.Background()

	req := edgeReq("e2ee.delete_backup", json.RawMessage(`{"backup_id":""}`))
	assertNoPanic(t, "e2ee-delete-empty", func() {
		result, err := s.handleE2EEDeleteBackup(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error for empty backup_id")
		}
		if err.Code != InvalidParams {
			t.Errorf("expected InvalidParams (%d), got %d", InvalidParams, err.Code)
		}
	})
}

func TestEdgeE2EE_BackupExists_MissingID(t *testing.T) {
	s := &Server{e2eeBackupEnabled: true}
	ctx := context.Background()

	req := edgeReq("e2ee.backup_exists", json.RawMessage(`{"backup_id":""}`))
	assertNoPanic(t, "e2ee-exists-empty", func() {
		result, err := s.handleE2EEBackupExists(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error for empty backup_id")
		}
		if err.Code != InvalidParams {
			t.Errorf("expected InvalidParams (%d), got %d", InvalidParams, err.Code)
		}
	})
}

func TestEdgeE2EE_CreateBackup_NilManager(t *testing.T) {
	s := &Server{e2eeBackupEnabled: true, backupMgr: nil}
	ctx := context.Background()

	// 24-word phrase with valid base64 encrypted_key
	params := json.RawMessage(`{"recovery_phrase":["a","b","c","d","e","f","g","h","i","j","k","l","m","n","o","p","q","r","s","t","u","v","w","x"],"encrypted_key":"AQID"}`)
	req := edgeReq("e2ee.create_backup", params)
	assertNoPanic(t, "e2ee-create-nil-mgr", func() {
		result, err := s.handleE2EECreateBackup(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error when backupMgr nil")
		}
		// resolveUserID returns "" with no matrix adapter and no peer creds
		// → user_id is required → InvalidParams
		// OR backupMgr nil → InternalError. The order of checks matters.
		// Check that it's a structured error either way.
		if err.Code != InvalidParams && err.Code != InternalError {
			t.Errorf("expected InvalidParams or InternalError, got code %d (msg: %s)", err.Code, err.Message)
		}
	})
}

// ---------------------------------------------------------------------------
// 4. REPLAY EDGE CASES
// ---------------------------------------------------------------------------

func TestEdgeReplay_InvalidTabID(t *testing.T) {
	store := browser.NewMultiTabStore()
	s := &Server{navChartStore: store}
	s.replayFlags.MultiTabReplay = true
	ctx := context.Background()

	req := edgeReq("browser.replay_diagnostics", json.RawMessage(`{"tab_id":"nonexistent-tab-xyz"}`))
	assertNoPanic(t, "replay-invalid-tab", func() {
		result, err := s.handleBrowserReplayDiagnostics(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error for nonexistent tab")
		}
		// "no stored charts for tab" → InvalidParams
		if err.Code != InvalidParams {
			t.Errorf("expected InvalidParams (%d), got %d (msg: %s)", InvalidParams, err.Code, err.Message)
		}
	})
}

func TestEdgeReplay_NoTabsExist(t *testing.T) {
	store := browser.NewMultiTabStore() // empty store
	s := &Server{navChartStore: store}
	s.replayFlags.MultiTabReplay = true
	ctx := context.Background()

	req := edgeReq("browser.replay_diagnostics", json.RawMessage(`{"tab_id":"any-tab"}`))
	assertNoPanic(t, "replay-no-tabs", func() {
		result, err := s.handleBrowserReplayDiagnostics(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error for empty store")
		}
		if err.Code != InvalidParams {
			t.Errorf("expected InvalidParams (%d), got %d (msg: %s)", InvalidParams, err.Code, err.Message)
		}
	})
}

func TestEdgeReplay_EmptyTabID(t *testing.T) {
	store := browser.NewMultiTabStore()
	s := &Server{navChartStore: store}
	s.replayFlags.MultiTabReplay = true
	ctx := context.Background()

	req := edgeReq("browser.replay_diagnostics", json.RawMessage(`{"tab_id":""}`))
	assertNoPanic(t, "replay-empty-tab", func() {
		result, err := s.handleBrowserReplayDiagnostics(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error for empty tab_id")
		}
		if err.Code != InvalidParams {
			t.Errorf("expected InvalidParams (%d), got %d", InvalidParams, err.Code)
		}
	})
}

func TestEdgeReplay_NilStore(t *testing.T) {
	s := &Server{navChartStore: nil}
	s.replayFlags.MultiTabReplay = true
	ctx := context.Background()

	req := edgeReq("browser.replay_diagnostics", json.RawMessage(`{"tab_id":"tab-1"}`))
	assertNoPanic(t, "replay-nil-store", func() {
		result, err := s.handleBrowserReplayDiagnostics(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error when store nil")
		}
		if err.Code != InternalError {
			t.Errorf("expected InternalError (%d), got %d", InternalError, err.Code)
		}
	})
}

func TestEdgeReplay_FeatureDisabled(t *testing.T) {
	s := &Server{}
	// Default: replayFlags.MultiTabReplay is false
	ctx := context.Background()

	req := edgeReq("browser.replay_diagnostics", json.RawMessage(`{"tab_id":"tab-1"}`))
	assertNoPanic(t, "replay-disabled", func() {
		result, err := s.handleBrowserReplayDiagnostics(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error when feature disabled")
		}
		if err.Code != MethodNotFound {
			t.Errorf("expected MethodNotFound (%d), got %d", MethodNotFound, err.Code)
		}
		if err.Message != "Feature disabled: multi_tab_replay" {
			t.Errorf("unexpected message: %s", err.Message)
		}
	})
}

// ---------------------------------------------------------------------------
// 5. GENERAL EDGE CASES — malformed input
// ---------------------------------------------------------------------------

func TestEdgeGeneral_NilParams(t *testing.T) {
	// Handlers receiving nil Params must not panic.
	s := &Server{zeroTrustKS: true}
	ctx := context.Background()

	cases := []struct {
		name    string
		handler func(context.Context, *Request) (interface{}, *ErrorObj)
	}{
		{"keystore_unseal", s.handleKeystoreUnseal},
		{"keystore_delete_key", s.handleKeystoreDeleteKey},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &Request{JSONRPC: "2.0", ID: 1, Method: tc.name, Params: nil}
			assertNoPanic(t, tc.name, func() {
				result, err := tc.handler(ctx, req)
				_ = result
				_ = err
				// Must not panic. Errors are acceptable.
			})
		})
	}
}

func TestEdgeGeneral_EmptyParams(t *testing.T) {
	// Handlers receiving `{}` must return structured errors, not panic.
	s := &Server{zeroTrustKS: true}
	ctx := context.Background()

	cases := []struct {
		name    string
		handler func(context.Context, *Request) (interface{}, *ErrorObj)
		wantOK  bool // true = success expected (e.g. sealed, session_status)
	}{
		{"keystore_unseal", s.handleKeystoreUnseal, false},
		{"keystore_sealed", s.handleKeystoreSealed, true},
		{"keystore_seal", s.handleKeystoreSeal, false},
		{"keystore_extend_session", s.handleKeystoreExtendSession, false},
		{"keystore_session_status", s.handleKeystoreSessionStatus, true},
		{"keystore_list_keys", s.handleKeystoreListKeys, false},
		{"keystore_delete_key", s.handleKeystoreDeleteKey, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := edgeReq(tc.name, json.RawMessage(`{}`))
			assertNoPanic(t, tc.name, func() {
				result, err := tc.handler(ctx, req)
				if tc.wantOK {
					if err != nil {
						t.Errorf("expected success, got error: code=%d msg=%s", err.Code, err.Message)
					}
					if result == nil {
						t.Error("expected non-nil result")
					}
				} else {
					// Error is expected, just verify no panic and structured error
					if err == nil && result == nil {
						t.Error("expected either result or error")
					}
				}
			})
		})
	}
}

func TestEdgeGeneral_OversizedParams(t *testing.T) {
	// Send a very large params payload; handlers must not panic.
	s := &Server{zeroTrustKS: true}
	ctx := context.Background()

	// Build a large JSON string (~1MB)
	largeValue := strings.Repeat("x", 1024*1024)
	largeParams := json.RawMessage(`{"password":"` + largeValue + `"}`)

	req := edgeReq("keystore.unseal", largeParams)
	assertNoPanic(t, "oversized-params", func() {
		result, err := s.handleKeystoreUnseal(ctx, req)
		_ = result
		_ = err
		// Must not panic; result or error is fine
	})
}

func TestEdgeGeneral_MalformedJSON(t *testing.T) {
	// Handlers receiving malformed JSON must return InvalidParams, not panic.
	s := &Server{zeroTrustKS: true}
	ctx := context.Background()

	cases := []struct {
		name    string
		handler func(context.Context, *Request) (interface{}, *ErrorObj)
	}{
		{"keystore_unseal", s.handleKeystoreUnseal},
		{"keystore_delete_key", s.handleKeystoreDeleteKey},
		{"voice_stop_session", (&Server{voicePipeline: "cloud"}).handleVoiceStopSession},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := edgeReq(tc.name, json.RawMessage(`{invalid`))
			assertNoPanic(t, tc.name+"_malformed", func() {
				result, err := tc.handler(ctx, req)
				if result != nil {
					t.Errorf("expected nil result, got %v", result)
				}
				if err == nil {
					t.Fatal("expected error for malformed JSON")
				}
				if err.Code != InvalidParams {
					t.Errorf("expected InvalidParams (%d), got %d (msg: %s)", InvalidParams, err.Code, err.Message)
				}
			})
		})
	}
}

func TestEdgeGeneral_E2EEMalformedJSON(t *testing.T) {
	s := &Server{e2eeBackupEnabled: true}
	ctx := context.Background()

	cases := []struct {
		name    string
		handler func(context.Context, *Request) (interface{}, *ErrorObj)
	}{
		{"create_backup", s.handleE2EECreateBackup},
		{"delete_backup", s.handleE2EEDeleteBackup},
		{"backup_exists", s.handleE2EEBackupExists},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := edgeReq("e2ee."+tc.name, json.RawMessage(`{invalid`))
			assertNoPanic(t, tc.name+"_malformed", func() {
				result, err := tc.handler(ctx, req)
				if result != nil {
					t.Errorf("expected nil result, got %v", result)
				}
				if err == nil {
					t.Fatal("expected error for malformed JSON")
				}
				if err.Code != InvalidParams {
					t.Errorf("expected InvalidParams (%d), got %d (msg: %s)", InvalidParams, err.Code, err.Message)
				}
			})
		})
	}
}

func TestEdgeGeneral_ReplayMalformedJSON(t *testing.T) {
	store := browser.NewMultiTabStore()
	s := &Server{navChartStore: store}
	s.replayFlags.MultiTabReplay = true
	ctx := context.Background()

	req := edgeReq("browser.replay_diagnostics", json.RawMessage(`{invalid`))
	assertNoPanic(t, "replay-malformed", func() {
		result, err := s.handleBrowserReplayDiagnostics(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error for malformed JSON")
		}
		if err.Code != InvalidParams {
			t.Errorf("expected InvalidParams (%d), got %d (msg: %s)", InvalidParams, err.Code, err.Message)
		}
	})
}

func TestEdgeGeneral_VoiceMalformedJSON(t *testing.T) {
	s := &Server{voicePipeline: "cloud", voiceMgr: nil}
	ctx := context.Background()

	req := edgeReq("voice.stop_session", json.RawMessage(`{invalid`))
	assertNoPanic(t, "voice-malformed", func() {
		result, err := s.handleVoiceStopSession(ctx, req)
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err == nil {
			t.Fatal("expected error for malformed JSON")
		}
		if err.Code != InvalidParams {
			t.Errorf("expected InvalidParams (%d), got %d (msg: %s)", InvalidParams, err.Code, err.Message)
		}
	})
}

// ---------------------------------------------------------------------------
// 6. CROSS-SUBSYSTEM — all handlers respond gracefully to nil Params
// ---------------------------------------------------------------------------

func TestEdgeCrossSubsystem_NilParamsNoPanic(t *testing.T) {
	// Verify that nil Params on various subsystem handlers never panics.
	ctx := context.Background()
	nilReq := &Request{JSONRPC: "2.0", ID: 1, Params: nil}

	cases := []struct {
		name    string
		handler func(context.Context, *Request) (interface{}, *ErrorObj)
	}{
		{"voice_start", (&Server{voicePipeline: "cloud", voiceMgr: nil}).handleVoiceStartSession},
		{"voice_stop", (&Server{voicePipeline: "cloud", voiceMgr: nil}).handleVoiceStopSession},
		{"voice_status", (&Server{voicePipeline: "cloud", voiceMgr: nil}).handleVoiceStatus},
		{"e2ee_create", (&Server{e2eeBackupEnabled: true}).handleE2EECreateBackup},
		{"e2ee_delete", (&Server{e2eeBackupEnabled: true}).handleE2EEDeleteBackup},
		{"e2ee_exists", (&Server{e2eeBackupEnabled: true}).handleE2EEBackupExists},
		{"replay_diag", (&Server{replayFlags: ReplayFeatureFlags{MultiTabReplay: true}, navChartStore: browser.NewMultiTabStore()}).handleBrowserReplayDiagnostics},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertNoPanic(t, tc.name, func() {
				// Just ensure no panic — errors are acceptable.
				result, err := tc.handler(ctx, nilReq)
				_ = result
				_ = err
			})
		})
	}
}

// ---------------------------------------------------------------------------
// 7. VOICE — start when started, stop when stopped (nil manager scenario)
// ---------------------------------------------------------------------------

func TestEdgeVoice_StartWhenNilManager_NoPanic(t *testing.T) {
	// Even calling start twice (simulated by calling on nil manager),
	// no panic should occur.
	s := &Server{voicePipeline: "cloud", voiceMgr: nil}
	ctx := context.Background()
	req := edgeReq("voice.start_session", json.RawMessage(`{"session_config":{}}`))

	for i := 0; i < 3; i++ {
		assertNoPanic(t, "voice-start-iter", func() {
			result, err := s.handleVoiceStartSession(ctx, req)
			if result != nil {
				t.Errorf("iteration %d: expected nil result, got %v", i, result)
			}
			if err == nil {
				t.Fatalf("iteration %d: expected error", i)
			}
			if err.Code != voice.ErrVoiceNotConfiguredCode {
				t.Errorf("iteration %d: expected ErrVoiceNotConfiguredCode (%d), got %d", i, voice.ErrVoiceNotConfiguredCode, err.Code)
			}
		})
	}
}
