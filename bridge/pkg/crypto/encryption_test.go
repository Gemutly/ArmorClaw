package crypto

import (
	"context"
	"encoding/json"
	"testing"
)

func TestEncryptionService_NilSafe(t *testing.T) {
	var s *EncryptionService

	if s.ShouldEncrypt("!room:test") {
		t.Error("nil service ShouldEncrypt should return false")
	}

	result, err := s.EncryptMessage(context.Background(), "!room:test", "hello")
	if err == nil {
		t.Error("nil service EncryptMessage should return error")
	}
	if result != nil {
		t.Error("nil service EncryptMessage should return nil result")
	}

	decrypted, err := s.DecryptEvent(context.Background(), nil)
	if err == nil {
		t.Error("nil service DecryptEvent should return error")
	}
	if decrypted != "" {
		t.Error("nil service DecryptEvent should return empty string")
	}

	placeholder := s.HandleDecryptionFailure(context.Background(), "!room:test", "evt1", "@user:test", nil)
	if placeholder == "" {
		t.Error("nil service HandleDecryptionFailure should return placeholder")
	}

	s.ProcessRetryQueue(context.Background())

	s.OnRoomEncryptionEvent("!room:test")

	if s.RetryCount() != 0 {
		t.Error("nil service RetryCount should return 0")
	}
}

func TestRoomEncryptionCache_SetAndGet(t *testing.T) {
	cache := NewRoomEncryptionCache(true)

	if cache.IsEncrypted("!room1:test") {
		t.Error("room should not be encrypted initially")
	}

	cache.SetEncrypted("!room1:test", true)
	if !cache.IsEncrypted("!room1:test") {
		t.Error("room should be encrypted after SetEncrypted(true)")
	}

	cache.SetEncrypted("!room1:test", false)
	if cache.IsEncrypted("!room1:test") {
		t.Error("room should not be encrypted after SetEncrypted(false)")
	}
}

func TestRoomEncryptionCache_DisabledGlobally(t *testing.T) {
	cache := NewRoomEncryptionCache(false)

	cache.SetEncrypted("!room1:test", true)
	if cache.IsEncrypted("!room1:test") {
		t.Error("IsEncrypted should return false when E2EE is disabled globally")
	}
}

func TestRoomEncryptionCache_NilSafe(t *testing.T) {
	var cache *RoomEncryptionCache

	if cache.IsEncrypted("!room1:test") {
		t.Error("nil cache IsEncrypted should return false")
	}
	cache.SetEncrypted("!room1:test", true)
	cache.ProcessStateEvents(nil)
	cache.Clear()
	if cache.IsEnabled() {
		t.Error("nil cache IsEnabled should return false")
	}
}

func TestRoomEncryptionCache_ProcessStateEvents(t *testing.T) {
	cache := NewRoomEncryptionCache(true)

	events := []json.RawMessage{
		[]byte(`{"type":"m.room.encryption","room_id":"!enc:test","content":{"algorithm":"m.megolm.v1.aes-sha2"}}`),
		[]byte(`{"type":"m.room.member","room_id":"!plain:test","content":{"membership":"join"}}`),
		[]byte(`{"type":"m.room.encryption","room_id":"!enc2:test","content":{"algorithm":"m.megolm.v1.aes-sha2"}}`),
		[]byte(`not valid json`),
	}

	cache.ProcessStateEvents(events)

	if !cache.IsEncrypted("!enc:test") {
		t.Error("!enc:test should be encrypted")
	}
	if !cache.IsEncrypted("!enc2:test") {
		t.Error("!enc2:test should be encrypted")
	}
	if cache.IsEncrypted("!plain:test") {
		t.Error("!plain:test should not be encrypted")
	}
}

func TestRoomEncryptionCache_Clear(t *testing.T) {
	cache := NewRoomEncryptionCache(true)
	cache.SetEncrypted("!room1:test", true)

	if !cache.IsEncrypted("!room1:test") {
		t.Error("room should be encrypted before clear")
	}

	cache.Clear()

	if cache.IsEncrypted("!room1:test") {
		t.Error("room should not be encrypted after clear")
	}
}

func TestEncryptionService_ShouldEncrypt(t *testing.T) {
	cache := NewRoomEncryptionCache(true)
	// engine is nil but cache has encryption status — ShouldEncrypt returns false
	// because without an engine, encryption is impossible
	svc := NewEncryptionService(nil, cache, nil)

	cache.SetEncrypted("!enc:test", true)
	if svc.ShouldEncrypt("!enc:test") {
		t.Error("ShouldEncrypt should return false when engine is nil even if cache says encrypted")
	}
	if svc.ShouldEncrypt("!plain:test") {
		t.Error("ShouldEncrypt should return false for non-encrypted room")
	}
}

func TestEncryptionService_DecryptionFailure_Placeholder(t *testing.T) {
	svc := NewEncryptionService(nil, nil, nil)

	content := json.RawMessage(`{"algorithm":"m.megolm.v1.aes-sha2"}`)
	placeholder := svc.HandleDecryptionFailure(
		context.Background(), "!room:test", "$evt1", "@user:test", content)

	if placeholder == "" {
		t.Error("placeholder should not be empty")
	}
	if svc.RetryCount() != 1 {
		t.Errorf("retry count should be 1, got %d", svc.RetryCount())
	}
}

func TestEncryptionService_DecryptionFailure_RetryQueue(t *testing.T) {
	svc := NewEncryptionService(nil, nil, nil)

	content := json.RawMessage(`{"algorithm":"m.megolm.v1.aes-sha2"}`)

	// Queue 2 failures
	svc.HandleDecryptionFailure(context.Background(), "!room:test", "$evt1", "@user:test", content)
	svc.HandleDecryptionFailure(context.Background(), "!room:test", "$evt2", "@user:test", content)

	if svc.RetryCount() != 2 {
		t.Errorf("retry count should be 2, got %d", svc.RetryCount())
	}

	// ProcessRetryQueue with nil engine will fail to decrypt, incrementing attempts
	results := svc.ProcessRetryQueue(context.Background())
	if len(results) != 0 {
		t.Errorf("results should be empty (no successful decryption), got %d", len(results))
	}

	// After max retries (3), events should be removed
	svc.ProcessRetryQueue(context.Background())
	svc.ProcessRetryQueue(context.Background())

	if svc.RetryCount() != 0 {
		t.Errorf("retry count should be 0 after max retries, got %d", svc.RetryCount())
	}
}

func TestEncryptionService_OnRoomEncryptionEvent(t *testing.T) {
	cache := NewRoomEncryptionCache(true)
	svc := NewEncryptionService(nil, cache, nil)

	svc.OnRoomEncryptionEvent("!room:test")

	if !cache.IsEncrypted("!room:test") {
		t.Error("room should be encrypted after OnRoomEncryptionEvent")
	}
}

func TestNewEncryptionService_NilLogger(t *testing.T) {
	svc := NewEncryptionService(nil, nil, nil)
	if svc == nil {
		t.Error("service should not be nil even with nil logger")
	}
}
