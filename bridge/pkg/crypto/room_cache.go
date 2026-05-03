package crypto

import (
	"encoding/json"
	"sync"
)

// RoomEncryptionCache tracks which rooms are encrypted by observing
// m.room.encryption state events from sync responses.
// It is safe for concurrent use.
type RoomEncryptionCache struct {
	mu      sync.RWMutex
	rooms   map[string]bool // room_id → encrypted
	enabled bool            // mirrors config.E2EE.Enabled
}

// NewRoomEncryptionCache creates a new cache.
// When e2eeEnabled is false, IsEncrypted always returns false.
func NewRoomEncryptionCache(e2eeEnabled bool) *RoomEncryptionCache {
	return &RoomEncryptionCache{
		rooms:   make(map[string]bool),
		enabled: e2eeEnabled,
	}
}

// IsEncrypted returns whether a room is known to be encrypted.
// Always returns false when E2EE is disabled globally.
func (c *RoomEncryptionCache) IsEncrypted(roomID string) bool {
	if c == nil || !c.enabled {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rooms[roomID]
}

// SetEncrypted updates the encryption status for a room.
func (c *RoomEncryptionCache) SetEncrypted(roomID string, encrypted bool) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rooms[roomID] = encrypted
}

// ProcessStateEvents scans raw state events for m.room.encryption type
// and updates the cache accordingly.
func (c *RoomEncryptionCache) ProcessStateEvents(events []json.RawMessage) {
	if c == nil || !c.enabled {
		return
	}
	for _, raw := range events {
		var evt struct {
			Type     string `json:"type"`
			RoomID   string `json:"room_id"`
			StateKey string `json:"state_key"`
			Content  struct {
				Algorithm string `json:"algorithm"`
			} `json:"content"`
		}
		if err := json.Unmarshal(raw, &evt); err != nil {
			continue
		}
		if evt.Type == "m.room.encryption" && evt.RoomID != "" {
			c.SetEncrypted(evt.RoomID, true)
		}
	}
}

// Clear removes all cached room encryption status.
// Primarily for testing.
func (c *RoomEncryptionCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rooms = make(map[string]bool)
}

// IsEnabled returns whether E2EE is globally enabled.
func (c *RoomEncryptionCache) IsEnabled() bool {
	if c == nil {
		return false
	}
	return c.enabled
}
