package crypto

import (
	"context"
	"fmt"
	"log/slog"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

// KeyExchangeService handles Matrix key exchange operations:
//   - Upload device keys and one-time keys to the homeserver
//   - Query device keys for other users
//   - Claim one-time keys for Olm session establishment
//   - Process device list change notifications from sync
//
// All methods are nil-safe — they are no-ops when the service is nil.
type KeyExchangeService struct {
	engine *CryptoEngine
	client *mautrix.Client
	logger *slog.Logger
}

// NewKeyExchangeService creates a new KeyExchangeService.
// Returns nil if engine is nil (E2EE disabled).
func NewKeyExchangeService(engine *CryptoEngine, logger *slog.Logger) *KeyExchangeService {
	if engine == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default().With("component", "key_exchange")
	}
	return &KeyExchangeService{
		engine: engine,
		client: engine.client,
		logger: logger,
	}
}

// UploadKeys uploads device keys and one-time keys via OlmMachine.ShareKeys.
func (s *KeyExchangeService) UploadKeys(ctx context.Context) error {
	if s == nil {
		return nil
	}
	machine := s.engine.GetOlmMachine()
	if machine == nil {
		return fmt.Errorf("olm machine not initialized")
	}
	s.logger.Debug("uploading device keys and OTKs")
	if err := machine.ShareKeys(ctx, -1); err != nil {
		s.logger.Error("failed to upload keys", "error", err)
		return fmt.Errorf("key upload failed: %w", err)
	}
	s.logger.Debug("keys uploaded successfully")
	return nil
}

// QueryKeys fetches device keys for given users via OlmMachine.FetchKeys,
// which calls the homeserver and caches results in the crypto store.
func (s *KeyExchangeService) QueryKeys(ctx context.Context, userIDs []string) error {
	if s == nil || len(userIDs) == 0 {
		return nil
	}
	machine := s.engine.GetOlmMachine()
	if machine == nil {
		return fmt.Errorf("olm machine not initialized")
	}
	mxUserIDs := make([]id.UserID, len(userIDs))
	for i, uid := range userIDs {
		mxUserIDs[i] = id.UserID(uid)
	}
	s.logger.Debug("querying device keys", "user_count", len(mxUserIDs))
	if _, err := machine.FetchKeys(ctx, mxUserIDs, true); err != nil {
		s.logger.Error("failed to query keys", "error", err)
		return fmt.Errorf("key query failed: %w", err)
	}
	s.logger.Debug("key query completed", "user_count", len(mxUserIDs))
	return nil
}

// ClaimKeys claims one-time keys for establishing Olm sessions.
// targets maps user IDs to device ID lists: map["@user:domain"][]{"DEVICEID"}.
// The claimed keys are returned; session establishment happens via
// OlmMachine.ShareGroupSession or on the next ProcessSyncResponse.
func (s *KeyExchangeService) ClaimKeys(ctx context.Context, targets map[string][]string) error {
	if s == nil || len(targets) == 0 {
		return nil
	}
	if s.client == nil {
		return fmt.Errorf("mautrix client not available")
	}
	claimReq := &mautrix.ReqClaimKeys{
		OneTimeKeys: make(mautrix.OneTimeKeysRequest),
	}
	for uid, devices := range targets {
		deviceMap := make(map[id.DeviceID]id.KeyAlgorithm, len(devices))
		for _, devID := range devices {
			deviceMap[id.DeviceID(devID)] = id.KeyAlgorithmSignedCurve25519
		}
		claimReq.OneTimeKeys[id.UserID(uid)] = deviceMap
	}
	s.logger.Debug("claiming OTKs", "user_count", len(claimReq.OneTimeKeys))
	if _, err := s.client.ClaimKeys(ctx, claimReq); err != nil {
		s.logger.Error("failed to claim keys", "error", err)
		return fmt.Errorf("key claim failed: %w", err)
	}
	s.logger.Debug("key claim completed")
	return nil
}

// ProcessDeviceListChanges handles device_lists from sync responses.
// Re-queries device keys for users whose device lists changed.
func (s *KeyExchangeService) ProcessDeviceListChanges(ctx context.Context, changed []string, left []string) error {
	if s == nil || (len(changed) == 0 && len(left) == 0) {
		return nil
	}
	s.logger.Debug("processing device list changes",
		"changed_count", len(changed), "left_count", len(left))
	if len(changed) > 0 {
		if err := s.QueryKeys(ctx, changed); err != nil {
			return fmt.Errorf("device list change processing failed: %w", err)
		}
	}
	s.logger.Debug("device list changes processed")
	return nil
}
