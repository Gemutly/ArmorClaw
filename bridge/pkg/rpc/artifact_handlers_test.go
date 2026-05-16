package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/armorclaw/bridge/pkg/secretary"
)

func TestArtifactMethodRegistration(t *testing.T) {
	cfg := Config{
		SecretaryHandler: NewSecretaryHandler(nil),
	}
	s, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	expected := []string{
		"secretary.artifact_upload",
		"secretary.artifact_download",
		"secretary.artifact_list",
		"secretary.artifact_update_status",
	}

	for _, method := range expected {
		if _, ok := s.handlers[method]; !ok {
			t.Errorf("method %q not registered in handler map", method)
		}
	}
}

func TestArtifactMethodNotInitialized(t *testing.T) {
	cfg := Config{
		ArtifactHandler: nil,
	}
	s, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	handler := s.handlers["secretary.artifact_upload"]
	if handler == nil {
		t.Fatal("artifact_upload handler not found")
	}

	_, errObj := handler(context.Background(), &Request{
		Method: "secretary.artifact_upload",
		Params: json.RawMessage(`{}`),
	})

	if errObj == nil {
		t.Fatal("expected error when artifact handler not initialized")
	}
	if errObj.Message != "artifact service not initialized" {
		t.Fatalf("unexpected error message: %s", errObj.Message)
	}
}

func TestArtifactHandlerAdapterUpload(t *testing.T) {
	store, err := secretary.NewArtifactStore(secretary.ArtifactStoreConfig{Path: ""})
	if err != nil {
		t.Fatalf("failed to create artifact store: %v", err)
	}
	handler := secretary.NewArtifactRPCHandler(store)
	adapter := NewArtifactRPCHandlerAdapter(handler)

	result, err := adapter.Handle("secretary.artifact_list", json.RawMessage(`{"user_id":"test-user"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}
	count, ok := resultMap["count"].(int)
	if !ok {
		t.Fatalf("expected count to be int, got %T", resultMap["count"])
	}
	if count != 0 {
		t.Fatalf("expected count 0, got %d", count)
	}
}
