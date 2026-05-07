package rpc

import (
	"context"
	"encoding/json"
	"testing"
)

func TestE2EECreateBackup_FlagOff(t *testing.T) {
	s := &Server{e2eeBackupEnabled: false}

	params, _ := json.Marshal(map[string]interface{}{
		"recovery_phrase": make([]string, 24),
		"encrypted_key":   "dGVzdA==",
	})
	req := &Request{JSONRPC: "2.0", ID: 1, Method: "e2ee.create_backup", Params: params}

	result, err := s.handleE2EECreateBackup(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error when E2EEBackup flag is off")
	}
	if result != nil {
		t.Error("Expected nil result when flag is off")
	}
	if err.Code != MethodNotFound {
		t.Errorf("Expected code %d (MethodNotFound), got %d", MethodNotFound, err.Code)
	}
}

func TestE2EEDeleteBackup_FlagOff(t *testing.T) {
	s := &Server{e2eeBackupEnabled: false}

	params, _ := json.Marshal(map[string]interface{}{
		"backup_id": "test-backup-123",
	})
	req := &Request{JSONRPC: "2.0", ID: 2, Method: "e2ee.delete_backup", Params: params}

	result, err := s.handleE2EEDeleteBackup(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error when E2EEBackup flag is off")
	}
	if result != nil {
		t.Error("Expected nil result when flag is off")
	}
	if err.Code != MethodNotFound {
		t.Errorf("Expected code %d (MethodNotFound), got %d", MethodNotFound, err.Code)
	}
}

func TestE2EEBackupExists_FlagOff(t *testing.T) {
	s := &Server{e2eeBackupEnabled: false}

	params, _ := json.Marshal(map[string]interface{}{
		"backup_id": "test-backup-123",
	})
	req := &Request{JSONRPC: "2.0", ID: 3, Method: "e2ee.backup_exists", Params: params}

	result, err := s.handleE2EEBackupExists(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error when E2EEBackup flag is off")
	}
	if result != nil {
		t.Error("Expected nil result when flag is off")
	}
	if err.Code != MethodNotFound {
		t.Errorf("Expected code %d (MethodNotFound), got %d", MethodNotFound, err.Code)
	}
}

func TestE2EEBackupHandlers_Registered(t *testing.T) {
	s, _ := New(Config{})
	methods := []string{
		"e2ee.create_backup",
		"e2ee.delete_backup",
		"e2ee.backup_exists",
	}
	for _, m := range methods {
		if _, ok := s.handlers[m]; !ok {
			t.Errorf("handler %q not registered", m)
		}
	}
}
