package rpc

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/armorclaw/bridge/pkg/voice"
)

func TestVoiceHandlers_Disabled(t *testing.T) {
	s := &Server{voicePipeline: "off"}
	ctx := context.Background()

	disabledCases := []struct {
		name    string
		method  string
		handler func(context.Context, *Request) (interface{}, *ErrorObj)
		params  interface{}
	}{
		{"start_session", "voice.start_session", s.handleVoiceStartSession, map[string]interface{}{"session_config": map[string]string{}}},
		{"stop_session", "voice.stop_session", s.handleVoiceStopSession, map[string]string{"session_id": "test"}},
		{"status", "voice.status", s.handleVoiceStatus, nil},
	}

	for _, tc := range disabledCases {
		t.Run(tc.name+"_disabled", func(t *testing.T) {
			var req *Request
			if tc.params != nil {
				raw, err := json.Marshal(tc.params)
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				req = &Request{JSONRPC: "2.0", ID: 1, Method: tc.method, Params: raw}
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
			if err.Message != "Feature disabled: voice_pipeline" {
				t.Errorf("unexpected message: %s", err.Message)
			}
		})
	}
}

func TestVoiceStopSession_MissingSessionID(t *testing.T) {
	s := &Server{voicePipeline: "off"}
	ctx := context.Background()

	req := &Request{JSONRPC: "2.0", ID: 1, Method: "voice.stop_session", Params: json.RawMessage(`{"session_id": ""}`)}
	result, err := s.handleVoiceStopSession(ctx, req)
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
	if err == nil {
		t.Fatal("expected error when feature disabled")
	}
	if err.Code != MethodNotFound {
		t.Errorf("expected code %d, got %d", MethodNotFound, err.Code)
	}
}

func TestVoiceHandlers_Registered(t *testing.T) {
	s, err := New(Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	methods := []string{
		"voice.start_session",
		"voice.stop_session",
		"voice.status",
	}

	for _, m := range methods {
		if _, ok := s.handlers[m]; !ok {
			t.Errorf("handler %q not registered", m)
		}
	}
}

func TestVoiceHandlers_InvalidParams(t *testing.T) {
	s := &Server{voicePipeline: "off"}
	ctx := context.Background()

	req := &Request{JSONRPC: "2.0", ID: 1, Method: "voice.stop_session", Params: json.RawMessage(`invalid json`)}
	result, err := s.handleVoiceStopSession(ctx, req)
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
	if err == nil || err.Code != MethodNotFound {
		t.Errorf("expected MethodNotFound for disabled feature, got %v", err)
	}
}

func TestVoiceHandlers_NilManager_ReturnsVoiceNotConfigured(t *testing.T) {
	// When voicePipeline is "cloud" but voiceMgr is nil, handlers must
	// return voice.ErrVoiceNotConfiguredCode (-32007), NOT InternalError (-32603).
	s := &Server{voicePipeline: "cloud", voiceMgr: nil}
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
			req := &Request{JSONRPC: "2.0", ID: 1, Method: "voice." + tc.name, Params: tc.params}
			result, err := tc.handler(ctx, req)
			if result != nil {
				t.Errorf("expected nil result, got %v", result)
			}
			if err == nil {
				t.Fatal("expected error when voiceMgr is nil")
			}
			if err.Code != voice.ErrVoiceNotConfiguredCode {
				t.Errorf("expected voice.ErrVoiceNotConfiguredCode (%d), got %d",
					voice.ErrVoiceNotConfiguredCode, err.Code)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// E2E voice pipeline tests: flag-off, prereq-fail, manager-nil, success path
// ---------------------------------------------------------------------------

func TestVoiceE2E_FlagOff_ReturnsMethodNotFound(t *testing.T) {
	s := &Server{voicePipeline: "off"}
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
			req := &Request{JSONRPC: "2.0", ID: 1, Method: "voice." + tc.name, Params: tc.params}
			result, err := tc.handler(ctx, req)
			if result != nil {
				t.Errorf("expected nil result, got %v", result)
			}
			if err == nil {
				t.Fatal("expected error when flag is off")
			}
			if err.Code != MethodNotFound {
				t.Errorf("expected -32601 (MethodNotFound), got %d", err.Code)
			}
		})
	}
}

func TestVoiceE2E_PrereqFail_TableDriven(t *testing.T) {
	prereqCases := []struct {
		name          string
		turnSecret    string
		openAIKey     string
		matrixWired   bool
		wantReason    voice.VoicePrereqReason
	}{
		{
			name:        "TURN_SECRET_missing",
			turnSecret:  "",
			openAIKey:   "sk-test-key",
			matrixWired: true,
			wantReason:  voice.PrereqTurnSecretMissing,
		},
		{
			name:        "OPENAI_API_KEY_missing",
			turnSecret:  "test-turn-secret",
			openAIKey:   "",
			matrixWired: true,
			wantReason:  voice.PrereqOpenAIKeyMissing,
		},
		{
			name:        "Matrix_unavailable",
			turnSecret:  "test-turn-secret",
			openAIKey:   "sk-test-key",
			matrixWired: false,
			wantReason:  voice.PrereqMatrixUnwired,
		},
	}

	for _, pc := range prereqCases {
		t.Run(pc.name, func(t *testing.T) {
			if pc.turnSecret != "" {
				t.Setenv("TURN_SECRET", pc.turnSecret)
			} else {
				t.Setenv("TURN_SECRET", "")
			}
			if pc.openAIKey != "" {
				t.Setenv("OPENAI_API_KEY", pc.openAIKey)
			} else {
				t.Setenv("OPENAI_API_KEY", "")
			}

			matrix := &mockMatrixAdapter{loggedIn: pc.matrixWired}
			s := &Server{
				voicePipeline: "cloud",
				voiceMgr:      &voice.Manager{},
				matrix:        matrix,
			}
			ctx := context.Background()

			handlers := []struct {
				name    string
				handler func(context.Context, *Request) (interface{}, *ErrorObj)
				params  json.RawMessage
			}{
				{"start_session", s.handleVoiceStartSession, json.RawMessage(`{"session_config":{}}`)},
				{"status", s.handleVoiceStatus, json.RawMessage(`{}`)},
			}

			for _, h := range handlers {
				t.Run(h.name, func(t *testing.T) {
					req := &Request{JSONRPC: "2.0", ID: 1, Method: "voice." + h.name, Params: h.params}
					result, err := h.handler(ctx, req)
					if result != nil {
						t.Errorf("expected nil result, got %v", result)
					}
					if err == nil {
						t.Fatal("expected prereq failure error")
					}
					if err.Code != voice.ErrVoiceNotConfiguredCode {
						t.Errorf("expected code -32007, got %d", err.Code)
					}
					if err.Data == nil {
						t.Fatal("expected error.data with prereq failures")
					}

					failures, ok := err.Data.([]voice.VoicePrereqFailure)
					if !ok {
						t.Fatalf("expected []voice.VoicePrereqFailure in data, got %T", err.Data)
					}

					found := false
					for _, f := range failures {
						if f.Reason == pc.wantReason {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected reason %q in failures: %+v", pc.wantReason, failures)
					}
				})
			}
		})
	}
}

func TestVoiceE2E_ManagerNil_ReturnsNotConfigured(t *testing.T) {
	s := &Server{voicePipeline: "cloud", voiceMgr: nil}
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
			req := &Request{JSONRPC: "2.0", ID: 1, Method: "voice." + tc.name, Params: tc.params}
			result, err := tc.handler(ctx, req)
			if result != nil {
				t.Errorf("expected nil result, got %v", result)
			}
			if err == nil {
				t.Fatal("expected error when voiceMgr is nil")
			}
			if err.Code != voice.ErrVoiceNotConfiguredCode {
				t.Errorf("expected -32007 (not -32603), got %d", err.Code)
			}
			if err.Code == InternalError {
				t.Error("should NOT return InternalError (-32603) for nil manager")
			}
		})
	}
}

func TestVoiceE2E_SuccessPath_ReturnsSessionID(t *testing.T) {
	t.Setenv("TURN_SECRET", "test-turn-secret")
	t.Setenv("OPENAI_API_KEY", "sk-test-key")

	matrix := &mockMatrixAdapter{loggedIn: true}
	s := &Server{
		voicePipeline: "cloud",
		voiceMgr:      &voice.Manager{},
		matrix:        matrix,
	}
	ctx := context.Background()

	req := &Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "voice.start_session",
		Params:  json.RawMessage(`{"session_config":{"mode":"cloud"}}`),
	}

	result, err := s.handleVoiceStartSession(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result)
	}

	sessionID, ok := resultMap["session_id"].(string)
	if !ok {
		t.Fatal("expected session_id string in result")
	}
	if sessionID == "" {
		t.Error("session_id should not be empty")
	}
	if len(sessionID) < 6 {
		t.Errorf("session_id looks too short: %q", sessionID)
	}
}

func TestVoiceE2E_SuccessPath_StatusReturnsSessions(t *testing.T) {
	t.Setenv("TURN_SECRET", "test-turn-secret")
	t.Setenv("OPENAI_API_KEY", "sk-test-key")

	matrix := &mockMatrixAdapter{loggedIn: true}
	s := &Server{
		voicePipeline: "cloud",
		voiceMgr:      &voice.Manager{},
		matrix:        matrix,
	}
	ctx := context.Background()

	req := &Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "voice.status",
		Params:  json.RawMessage(`{}`),
	}

	result, err := s.handleVoiceStatus(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result)
	}

	if _, hasSessions := resultMap["sessions"]; !hasSessions {
		t.Error("expected 'sessions' key in status result")
	}
	if _, hasActive := resultMap["active"]; !hasActive {
		t.Error("expected 'active' key in status result")
	}
}

func TestVoiceE2E_ContractShape_ErrorDataJSON(t *testing.T) {
	t.Setenv("TURN_SECRET", "")
	t.Setenv("OPENAI_API_KEY", "")

	matrix := &mockMatrixAdapter{loggedIn: false}
	s := &Server{
		voicePipeline: "cloud",
		voiceMgr:      &voice.Manager{},
		matrix:        matrix,
	}
	ctx := context.Background()

	req := &Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "voice.start_session",
		Params:  json.RawMessage(`{"session_config":{}}`),
	}

	_, rpcErr := s.handleVoiceStartSession(ctx, req)
	if rpcErr == nil {
		t.Fatal("expected error")
	}

	resp := Response{
		JSONRPC: "2.0",
		ID:      1,
		Error:   rpcErr,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var parsed struct {
		Error struct {
			Code  int              `json:"code"`
			Data  []voice.VoicePrereqFailure `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if parsed.Error.Code != voice.ErrVoiceNotConfiguredCode {
		t.Errorf("error.code: got %d, want %d", parsed.Error.Code, voice.ErrVoiceNotConfiguredCode)
	}
	if len(parsed.Error.Data) < 3 {
		t.Errorf("expected >= 3 prereq failures, got %d", len(parsed.Error.Data))
	}

	reasons := make(map[voice.VoicePrereqReason]bool)
	for _, f := range parsed.Error.Data {
		reasons[f.Reason] = true
		if f.Message == "" {
			t.Errorf("failure with reason %q has empty message", f.Reason)
		}
	}
	for _, want := range []voice.VoicePrereqReason{
		voice.PrereqTurnSecretMissing,
		voice.PrereqOpenAIKeyMissing,
		voice.PrereqMatrixUnwired,
	} {
		if !reasons[want] {
			t.Errorf("missing expected reason %q in error.data", want)
		}
	}
}

func TestVoiceE2E_FlagOff_DoesNotDependOnEnv(t *testing.T) {
	os.Setenv("TURN_SECRET", "has-value")
	os.Setenv("OPENAI_API_KEY", "has-value")
	defer os.Unsetenv("TURN_SECRET")
	defer os.Unsetenv("OPENAI_API_KEY")

	s := &Server{voicePipeline: "off"}
	ctx := context.Background()

	req := &Request{JSONRPC: "2.0", ID: 1, Method: "voice.start_session", Params: json.RawMessage(`{}`)}
	_, err := s.handleVoiceStartSession(ctx, req)
	if err == nil {
		t.Fatal("expected error when flag is off regardless of env vars")
	}
	if err.Code != MethodNotFound {
		t.Errorf("expected -32601 even with env vars set, got %d", err.Code)
	}
}
