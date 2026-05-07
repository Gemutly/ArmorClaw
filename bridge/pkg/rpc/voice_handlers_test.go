package rpc

import (
	"context"
	"encoding/json"
	"testing"
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
