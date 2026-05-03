package crypto

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

const maxRetries = 3

// retryEntry tracks a failed decryption for later retry.
type retryEntry struct {
	EventID   string
	RoomID    string
	Sender    string
	Content   json.RawMessage
	Attempts  int
	LastRetry time.Time
}

// EncryptionService handles encrypt/decrypt operations using CryptoEngine.
// All methods are nil-safe — they are no-ops when the service is nil
// (i.e., when E2EE is disabled).
type EncryptionService struct {
	engine *CryptoEngine
	cache  *RoomEncryptionCache
	logger *slog.Logger

	mu      sync.Mutex
	retries map[string]*retryEntry // event_id → retry info
}

// NewEncryptionService creates a new EncryptionService.
// Any of engine, cache, or logger may be nil (all methods become no-ops).
func NewEncryptionService(engine *CryptoEngine, cache *RoomEncryptionCache, logger *slog.Logger) *EncryptionService {
	if logger == nil {
		logger = slog.Default()
	}
	return &EncryptionService{
		engine:  engine,
		cache:   cache,
		logger:  logger,
		retries: make(map[string]*retryEntry),
	}
}

// ShouldEncrypt returns true if the room is known to be encrypted
// and E2EE is enabled.
func (s *EncryptionService) ShouldEncrypt(roomID string) bool {
	if s == nil || s.cache == nil || s.engine == nil {
		return false
	}
	return s.cache.IsEncrypted(roomID)
}

// EncryptMessage encrypts a plaintext message for the given room using Megolm.
// Returns the encrypted event content as a serializable map.
func (s *EncryptionService) EncryptMessage(ctx context.Context, roomID, plaintext string) (map[string]interface{}, error) {
	if s == nil || s.engine == nil {
		return nil, fmt.Errorf("encryption service not available")
	}

	machine := s.engine.GetOlmMachine()
	if machine == nil {
		return nil, fmt.Errorf("olm machine not initialized")
	}

	content := map[string]interface{}{
		"msgtype": "m.text",
		"body":    plaintext,
	}

	encrypted, err := machine.EncryptMegolmEvent(ctx, id.RoomID(roomID), event.EventMessage, content)
	if err != nil {
		return nil, fmt.Errorf("megolm encrypt failed: %w", err)
	}

	// Serialize EncryptedEventContent to a generic map for HTTP transport
	raw, err := json.Marshal(encrypted)
	if err != nil {
		return nil, fmt.Errorf("marshal encrypted content: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal encrypted content: %w", err)
	}

	return result, nil
}

// DecryptEvent decrypts an m.room.encrypted event.
// Returns the decrypted plaintext body string.
func (s *EncryptionService) DecryptEvent(ctx context.Context, rawContent json.RawMessage) (string, error) {
	if s == nil || s.engine == nil {
		return "", fmt.Errorf("encryption service not available")
	}

	machine := s.engine.GetOlmMachine()
	if machine == nil {
		return "", fmt.Errorf("olm machine not initialized")
	}

	// Build a mautrix event.Event from the raw content
	var evt event.Event
	if err := json.Unmarshal(rawContent, &evt); err != nil {
		return "", fmt.Errorf("unmarshal encrypted event: %w", err)
	}

	// Ensure content is parsed as EncryptedEventContent
	if evt.Content.Parsed == nil {
		var encryptedContent event.EncryptedEventContent
		if err := json.Unmarshal(evt.Content.VeryRaw, &encryptedContent); err != nil {
			return "", fmt.Errorf("parse encrypted content: %w", err)
		}
		evt.Content.Parsed = &encryptedContent
	}

	decrypted, err := machine.DecryptMegolmEvent(ctx, &evt)
	if err != nil {
		return "", fmt.Errorf("megolm decrypt failed: %w", err)
	}

	// Extract the plaintext body
	if body, ok := decrypted.Content.Raw["body"].(string); ok {
		return body, nil
	}

	raw, err := json.Marshal(decrypted.Content.Raw)
	if err != nil {
		return "", fmt.Errorf("marshal decrypted content: %w", err)
	}
	return string(raw), nil
}

// HandleDecryptionFailure creates a placeholder message for a decryption failure
// and queues the event for retry.
func (s *EncryptionService) HandleDecryptionFailure(ctx context.Context, roomID, eventID, sender string, rawContent json.RawMessage) string {
	if s == nil {
		return "[Decryption failed - E2EE not available]"
	}

	s.logger.Warn("decryption failed, emitting placeholder",
		"event_id", eventID,
		"room_id", roomID,
		"sender", sender,
	)

	// Queue for retry
	s.mu.Lock()
	entry, exists := s.retries[eventID]
	if !exists {
		s.retries[eventID] = &retryEntry{
			EventID:  eventID,
			RoomID:   roomID,
			Sender:   sender,
			Content:  rawContent,
			Attempts: 1,
		}
	} else {
		entry.Attempts++
		entry.LastRetry = time.Now()
	}
	s.mu.Unlock()

	return fmt.Sprintf("[Unable to decrypt message - waiting for key material (attempt %d/%d)]",
		s.retries[eventID].Attempts, maxRetries)
}

// ProcessRetryQueue retries failed decryptions after new key material
// may have arrived via a sync response.
func (s *EncryptionService) ProcessRetryQueue(ctx context.Context) map[string]string {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	entries := make([]*retryEntry, 0, len(s.retries))
	for _, entry := range s.retries {
		entries = append(entries, entry)
	}
	s.mu.Unlock()

	results := make(map[string]string)
	for _, entry := range entries {
		if entry.Attempts >= maxRetries {
			s.mu.Lock()
			delete(s.retries, entry.EventID)
			s.mu.Unlock()
			s.logger.Warn("max retries exceeded for decryption",
				"event_id", entry.EventID,
				"room_id", entry.RoomID,
			)
			continue
		}

		decrypted, err := s.DecryptEvent(ctx, entry.Content)
		if err != nil {
			s.mu.Lock()
			if e, ok := s.retries[entry.EventID]; ok {
				e.Attempts++
				e.LastRetry = time.Now()
			}
			s.mu.Unlock()
			continue
		}

		// Success — remove from retry queue
		s.mu.Lock()
		delete(s.retries, entry.EventID)
		s.mu.Unlock()
		results[entry.EventID] = decrypted
	}

	return results
}

// OnRoomEncryptionEvent marks a room as encrypted in the cache.
func (s *EncryptionService) OnRoomEncryptionEvent(roomID string) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.SetEncrypted(roomID, true)
}

// RetryCount returns the number of events in the retry queue (for testing).
func (s *EncryptionService) RetryCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.retries)
}
