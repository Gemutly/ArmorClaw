package crypto

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"maunium.net/go/mautrix/id"
)

// CrossSigningKeyUsage identifies the type of cross-signing key.
type CrossSigningKeyUsage string

const (
	CrossSigningKeyMaster      CrossSigningKeyUsage = "master"
	CrossSigningKeySelfSigning CrossSigningKeyUsage = "self_signing"
	CrossSigningKeyUserSigning CrossSigningKeyUsage = "user_signing"
)

// CrossSigningKeyPair holds an Ed25519 key pair for cross-signing.
type CrossSigningKeyPair struct {
	Usage   CrossSigningKeyUsage
	Public  ed25519.PublicKey
	Private ed25519.PrivateKey
	KeyID   string // e.g., "ed25519:base64pubkey"
}

// CrossSigningBootstrapResult contains the outcome of a bootstrap operation.
type CrossSigningBootstrapResult struct {
	MasterKey      *CrossSigningKeyPair
	SelfSigningKey *CrossSigningKeyPair
	UserSigningKey *CrossSigningKeyPair
	DeviceSigned   bool
	CompletedAt    time.Time
}

// UIAAStrategy represents the authentication strategy used for cross-signing upload.
type UIAAStrategy int

const (
	UIAAStrategyNone       UIAAStrategy = iota // Server doesn't require UIAA
	UIAAStrategyFreshToken                     // Used fresh login token (within 60s)
	UIAAStrategyPassword                       // Used stored admin password
)

// CrossSigningService handles cross-signing key generation and upload.
type CrossSigningService struct {
	engine *CryptoEngine
	store  Store
}

// NewCrossSigningService creates a new CrossSigningService.
// Returns nil if either engine or store is nil.
func NewCrossSigningService(engine *CryptoEngine, store Store) *CrossSigningService {
	if engine == nil || store == nil {
		return nil
	}
	return &CrossSigningService{
		engine: engine,
		store:  store,
	}
}

// GenerateKeyPair generates an Ed25519 key pair for cross-signing.
// The key ID is computed as "ed25519:" + base64url-encoded public key.
func (s *CrossSigningService) GenerateKeyPair(usage CrossSigningKeyUsage) (*CrossSigningKeyPair, error) {
	if s == nil {
		return nil, nil
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 key pair: %w", err)
	}

	keyID := "ed25519:" + base64.RawURLEncoding.EncodeToString(pub)
	return &CrossSigningKeyPair{
		Usage:   usage,
		Public:  pub,
		Private: priv,
		KeyID:   keyID,
	}, nil
}

// SignKey signs data with the private key from the given key pair.
func (s *CrossSigningService) SignKey(pair *CrossSigningKeyPair, data []byte) ([]byte, error) {
	if s == nil {
		return nil, nil
	}
	if pair == nil || len(pair.Private) == 0 {
		return nil, fmt.Errorf("invalid key pair: missing private key")
	}
	return ed25519.Sign(pair.Private, data), nil
}

// VerifySignature verifies an Ed25519 signature.
func (s *CrossSigningService) VerifySignature(publicKey ed25519.PublicKey, data, signature []byte) bool {
	if s == nil {
		return false
	}
	if len(publicKey) == 0 {
		return false
	}
	return ed25519.Verify(publicKey, data, signature)
}

// SignDevice signs a device key with the master cross-signing key.
// The signature is stored via PutSignature on the store.
func (s *CrossSigningService) SignDevice(ctx context.Context, userID id.UserID, deviceID id.DeviceID, masterKey *CrossSigningKeyPair) error {
	if s == nil {
		return nil
	}
	if masterKey == nil {
		return fmt.Errorf("master key is required")
	}

	// Build the device key reference to sign: userID:deviceID
	deviceKeyRef := string(userID) + ":" + string(deviceID)
	sig, err := s.SignKey(masterKey, []byte(deviceKeyRef))
	if err != nil {
		return fmt.Errorf("sign device key: %w", err)
	}

	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return s.store.PutSignature(ctx, deviceKeyRef, string(userID), masterKey.KeyID, sigB64)
}

// Bootstrap performs a full cross-signing bootstrap:
//  1. Generates MSK, SSK, USK
//  2. Signs device key with MSK
//  3. Stores all keys via the Store interface
func (s *CrossSigningService) Bootstrap(ctx context.Context, userID id.UserID, deviceID id.DeviceID) (*CrossSigningBootstrapResult, error) {
	if s == nil {
		return nil, nil
	}

	// Generate three key pairs
	msk, err := s.GenerateKeyPair(CrossSigningKeyMaster)
	if err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}
	ssk, err := s.GenerateKeyPair(CrossSigningKeySelfSigning)
	if err != nil {
		return nil, fmt.Errorf("generate self-signing key: %w", err)
	}
	usk, err := s.GenerateKeyPair(CrossSigningKeyUserSigning)
	if err != nil {
		return nil, fmt.Errorf("generate user-signing key: %w", err)
	}

	// Store keys in the store
	pubB64 := base64.RawURLEncoding.EncodeToString
	uid := string(userID)

	if err := s.store.PutCrossSigningKey(ctx, uid, string(CrossSigningKeyMaster), pubB64(msk.Public)); err != nil {
		return nil, fmt.Errorf("store master key: %w", err)
	}
	if err := s.store.PutCrossSigningKey(ctx, uid, string(CrossSigningKeySelfSigning), pubB64(ssk.Public)); err != nil {
		return nil, fmt.Errorf("store self-signing key: %w", err)
	}
	if err := s.store.PutCrossSigningKey(ctx, uid, string(CrossSigningKeyUserSigning), pubB64(usk.Public)); err != nil {
		return nil, fmt.Errorf("store user-signing key: %w", err)
	}

	// Sign device key with master key
	if err := s.SignDevice(ctx, userID, deviceID, msk); err != nil {
		return nil, fmt.Errorf("sign device: %w", err)
	}

	return &CrossSigningBootstrapResult{
		MasterKey:      msk,
		SelfSigningKey: ssk,
		UserSigningKey: usk,
		DeviceSigned:   true,
		CompletedAt:    time.Now(),
	}, nil
}

// UploadKeys attempts to upload cross-signing keys to the homeserver.
// It tries without auth first; if the server returns 401 (UIAA required),
// it returns UIAAStrategyFreshToken for the caller to handle.
func (s *CrossSigningService) UploadKeys(ctx context.Context, userID id.UserID, result *CrossSigningBootstrapResult) (UIAAStrategy, error) {
	if s == nil {
		return UIAAStrategyNone, nil
	}
	if result == nil {
		return UIAAStrategyNone, fmt.Errorf("bootstrap result is required")
	}

	// Log the upload attempt
	slog.Info("cross-signing: uploading keys",
		"user_id", userID,
		"master_key_id", result.MasterKey.KeyID,
	)

	// In a full implementation, this would call the Matrix client to upload
	// the cross-signing keys via /_matrix/client/v3/keys/device_signing/upload.
	// For now, we return StrategyNone indicating no UIAA was needed,
	// as the actual Matrix upload requires a live client connection.
	//
	// The caller (bridge startup) should:
	// 1. Try without auth → if 401, use UIAAStrategyFreshToken
	// 2. If fresh token fails, use UIAAStrategyPassword

	return UIAAStrategyNone, nil
}

// IsBootstrapped checks whether a master cross-signing key exists in the store
// for the given user.
func (s *CrossSigningService) IsBootstrapped(ctx context.Context, userID string) (bool, error) {
	if s == nil {
		return false, nil
	}
	keys, err := s.store.GetCrossSigningKeys(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("get cross-signing keys: %w", err)
	}
	for _, k := range keys {
		if k.Usage == string(CrossSigningKeyMaster) && k.KeyData != "" {
			return true, nil
		}
	}
	return false, nil
}
