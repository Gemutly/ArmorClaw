// Package crypto provides cryptographic interfaces for E2EE support
package crypto

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// CryptoEngine wraps mautrix-go's OlmMachine for Bridge E2EE.
// When E2EE is disabled (config gate), CryptoEngine is nil and all
// operations are no-ops. Callers should check IsE2EEEnabled() or
// simply call methods on a nil CryptoEngine (all methods are nil-safe).
type CryptoEngine struct {
	machine  *crypto.OlmMachine
	store    *MautrixStoreAdapter
	client   *mautrix.Client
	deviceID id.DeviceID
	userID   id.UserID
	logger   *slog.Logger
	mu       sync.RWMutex
}

// CryptoEngineConfig holds configuration for creating a CryptoEngine.
type CryptoEngineConfig struct {
	// UserID is the Matrix user ID (e.g., "@bridge:example.com")
	UserID string
	// DeviceID is the Matrix device ID
	DeviceID string
	// HomeserverURL is the Matrix homeserver URL
	HomeserverURL string
	// AccessToken is the Matrix access token
	AccessToken string
	// Store is our SQLCipher-backed crypto store
	Store Store
	// Logger is an optional structured logger
	Logger *slog.Logger
	// E2EEEnabled gates all crypto operations. When false, NewCryptoEngine returns nil.
	E2EEEnabled bool
	// PickleKey is used to encrypt Olm pickles in the store (32 bytes recommended)
	PickleKey []byte
}

// NewCryptoEngine creates a new CryptoEngine.
// Returns (nil, nil) if E2EE is disabled — callers should treat nil as "no crypto".
func NewCryptoEngine(ctx context.Context, cfg CryptoEngineConfig) (*CryptoEngine, error) {
	if !cfg.E2EEEnabled {
		if cfg.Logger != nil {
			cfg.Logger.Info("E2EE disabled, CryptoEngine not initialized")
		}
		return nil, nil
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if err := initOlmBackend(); err != nil {
		return nil, fmt.Errorf("olm backend init failed: %w", err)
	}

	pickleKey := cfg.PickleKey
	if len(pickleKey) == 0 {
		pickleKey = []byte("armorclaw-bridge-default-pickle!")
	}

	userID := id.UserID(cfg.UserID)
	deviceID := id.DeviceID(cfg.DeviceID)

	// Create mautrix client for crypto operations
	client, err := mautrix.NewClient(cfg.HomeserverURL, userID, cfg.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create mautrix client for crypto: %w", err)
	}
	client.DeviceID = deviceID

	// Create our store adapter (wraps crypto.MemoryStore with our Store persistence)
	var storeAdapter *MautrixStoreAdapter
	if cfg.Store != nil {
		storeAdapter = NewMautrixStoreAdapter(cfg.Store, client, pickleKey, logger)
	} else {
		// Use bare MemoryStore if no backing store provided
		storeAdapter = NewMautrixStoreAdapter(NewMemoryStore(), client, pickleKey, logger)
	}

	// mautrix-go uses zerolog; we use slog. Bridge with a no-op zerolog.
	zlog := zerolog.Nop()

	// Create OlmMachine
	machine := crypto.NewOlmMachine(client, &zlog, storeAdapter, &noOpStateStore{})

	return &CryptoEngine{
		machine:  machine,
		store:    storeAdapter,
		client:   client,
		deviceID: deviceID,
		userID:   userID,
		logger:   logger,
	}, nil
}

// Initialize loads or creates the Olm account and uploads device keys + OTKs.
// Must be called once after NewCryptoEngine, before any sync processing.
func (ce *CryptoEngine) Initialize(ctx context.Context) error {
	if ce == nil {
		return nil
	}

	ce.logger.Info("initializing CryptoEngine",
		"user_id", ce.userID,
		"device_id", ce.deviceID,
	)

	// Load existing crypto state from our backing store into MemoryStore
	if err := ce.store.LoadFromBackingStore(ctx); err != nil {
		ce.logger.Warn("failed to load existing crypto state (starting fresh)", "error", err)
	}

	// Load creates the Olm account if none exists, loads it otherwise
	if err := ce.machine.Load(ctx); err != nil {
		return fmt.Errorf("failed to load OlmMachine: %w", err)
	}

	// Share keys uploads device keys and one-time keys to the server.
	// -1 means "generate and upload as many OTKs as the server wants".
	if err := ce.machine.ShareKeys(ctx, -1); err != nil {
		ce.logger.Warn("failed to share keys (will retry on next sync)", "error", err)
		// Non-fatal: keys will be re-shared on next sync if needed
	}

	ce.logger.Info("CryptoEngine initialized successfully")
	return nil
}

// ProcessSyncResponse feeds a Bridge sync response to OlmMachine.
// Converts Bridge's SyncResponse to mautrix format and processes:
// - to_device events (key exchange, room keys)
// - device_lists changes (tracks new/removed devices)
// - device_one_time_keys_count (triggers OTK generation)
func (ce *CryptoEngine) ProcessSyncResponse(ctx context.Context, bridgeResp *SyncResponse) error {
	if ce == nil || bridgeResp == nil {
		return nil
	}

	mautrixResp := BridgeSyncToMautrix(bridgeResp)

	ce.mu.Lock()
	ce.machine.ProcessSyncResponse(ctx, mautrixResp, "")
	ce.mu.Unlock()

	// Persist crypto state after processing sync
	if err := ce.store.Flush(ctx); err != nil {
		ce.logger.Warn("failed to flush crypto store after sync", "error", err)
	}

	return nil
}

// IsE2EEEnabled returns whether E2EE crypto is active.
func (ce *CryptoEngine) IsE2EEEnabled() bool {
	return ce != nil
}

// GetOlmMachine returns the underlying OlmMachine for advanced usage.
// Returns nil if E2EE is disabled.
func (ce *CryptoEngine) GetOlmMachine() *crypto.OlmMachine {
	if ce == nil {
		return nil
	}
	return ce.machine
}

// GetUserID returns the Matrix user ID associated with this engine.
func (ce *CryptoEngine) GetUserID() id.UserID {
	if ce == nil {
		return ""
	}
	return ce.userID
}

// GetDeviceID returns the Matrix device ID associated with this engine.
func (ce *CryptoEngine) GetDeviceID() id.DeviceID {
	if ce == nil {
		return ""
	}
	return ce.deviceID
}

// noOpStateStore provides safe defaults for StateStore methods
// until a proper implementation is wired in.
type noOpStateStore struct{}

func (n *noOpStateStore) IsEncrypted(_ context.Context, _ id.RoomID) (bool, error) {
	return false, nil
}

func (n *noOpStateStore) GetEncryptionEvent(_ context.Context, _ id.RoomID) (*event.EncryptionEventContent, error) {
	return nil, nil
}

func (n *noOpStateStore) FindSharedRooms(_ context.Context, _ id.UserID) ([]id.RoomID, error) {
	return nil, nil
}

// SyncResponse contains crypto-relevant fields from a Matrix /sync response.
// This mirrors the fields from internal/adapter.SyncResponse that are needed
// for E2EE processing, avoiding a direct import of the adapter package.
type SyncResponse struct {
	ToDevice               *ToDeviceSync     `json:"to_device,omitempty"`
	DeviceLists            *DeviceListsSync  `json:"device_lists,omitempty"`
	DeviceOneTimeKeysCount map[string]int    `json:"device_one_time_keys_count,omitempty"`
}

// ToDeviceSync represents the to_device section of a sync response.
type ToDeviceSync struct {
	Events []json.RawMessage `json:"events,omitempty"`
}

// DeviceListsSync represents the device_lists section of a sync response.
type DeviceListsSync struct {
	Changed []string `json:"changed,omitempty"`
	Left    []string `json:"left,omitempty"`
}
