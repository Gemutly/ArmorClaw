package rpc

import (
	"context"
	"testing"
)

func TestE2EERuntimeToggle(t *testing.T) {
	s := &Server{}
	if s.IsE2EERuntimeEnabled() {
		t.Error("E2EE runtime should default to false")
	}

	s.e2eeEnabled.Store(true)
	if !s.IsE2EERuntimeEnabled() {
		t.Error("E2EE runtime should be true after enable")
	}

	s.e2eeEnabled.Store(false)
	if s.IsE2EERuntimeEnabled() {
		t.Error("E2EE runtime should be false after disable")
	}
}

func TestE2EEToggleRequiresAuth(t *testing.T) {
	s := &Server{}

	req := &Request{JSONRPC: "2.0", ID: 1, Method: "bridge.e2ee_enable"}

	result, err := s.handleE2EEEnable(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error when not authenticated")
	}
	if result != nil {
		t.Error("Expected nil result when not authenticated")
	}
	if err.Message != "Not authenticated" {
		t.Errorf("Expected 'Not authenticated', got %s", err.Message)
	}

	req2 := &Request{JSONRPC: "2.0", ID: 2, Method: "bridge.e2ee_disable"}
	result2, err2 := s.handleE2EEDisable(context.Background(), req2)
	if err2 == nil {
		t.Fatal("Expected error when not authenticated for disable")
	}
	if result2 != nil {
		t.Error("Expected nil result when not authenticated for disable")
	}
}

func TestE2EEToggleWithNilMatrix(t *testing.T) {
	s := &Server{matrix: nil}

	req := &Request{JSONRPC: "2.0", ID: 1, Method: "bridge.e2ee_enable"}
	_, err := s.handleE2EEEnable(context.Background(), req)
	if err == nil || err.Message != "Not authenticated" {
		t.Errorf("Expected 'Not authenticated' with nil matrix, got %v", err)
	}
}
