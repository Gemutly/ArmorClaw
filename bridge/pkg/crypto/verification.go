// Package crypto provides cryptographic interfaces for E2EE support
package crypto

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"maunium.net/go/mautrix/id"
)

// SASVerificationService handles Matrix SAS verification flows.
// All methods are nil-safe: calling on a nil service returns zero values.
type SASVerificationService struct {
	engine  *CryptoEngine
	mu      sync.RWMutex
	active  map[string]*SASVerificationTransaction // key: transactionID
	timeout time.Duration
	logger  *slog.Logger
}

// SASVerificationTransaction tracks an in-progress verification.
type SASVerificationTransaction struct {
	TransactionID    string
	TheirUserID      id.UserID
	TheirDeviceID    id.DeviceID
	OurDeviceID      id.DeviceID
	State            SASState
	StartedAt        time.Time
	KeyAgreementData []byte   // ECDH shared secret
	Commitment       []byte   // SHA256 of our key + their key
	OurKey           []byte   // Our ephemeral public key
	TheirKey         []byte   // Their ephemeral public key
	TheirMAC         []byte   // Their MAC for verification
	OurMAC           []byte   // Our MAC for verification
	Emoji            []string // Generated emoji for display
	CancelReason     string
}

// SASState represents the state of a SAS verification transaction.
type SASState int

const (
	SASStateCreated      SASState = iota
	SASStateStarted
	SASStateAccepted
	SASStateKeyExchanged
	SASStateConfirmed
	SASStateMACExchanged
	SASStateDone
	SASStateCancelled
)

// sasEmojis is the fixed set of 64 emoji used for SAS verification display.
var sasEmojis = [64]string{
	"🐶", "🐱", "🦁", "🐎", "🦄", "🐷", "🐘", "🐰",
	"🐼", "🐓", "🐧", "🐟", "🐙", "🦋", "🌷", "🌹",
	"🌻", "🌲", "🌵", "🍄", "🌍", "🌙", "☁️", "🔥",
	"🍌", "🍎", "🍓", "🌽", "🍕", "🎂", "❤️", "💛",
	"💚", "💙", "💜", "🧡", "🔔", "♩", "🎸", "🎺",
	"🏀", "⚽", "🎯", "🏆", "🏁", "🚗", "🚌", "🚀",
	"✈️", "🛳️", "🏠", "⏰", "📱", "💻", "💿", "📷",
	"🔑", "💡", "📖", "✏️", "📎", "✂️", "🔒", "🔔",
}

// NewSASVerificationService creates a new SAS verification service.
// Returns nil if engine is nil.
func NewSASVerificationService(engine *CryptoEngine) *SASVerificationService {
	if engine == nil {
		return nil
	}
	return &SASVerificationService{
		engine:  engine,
		active:  make(map[string]*SASVerificationTransaction),
		timeout: 10 * time.Minute,
		logger:  slog.Default(),
	}
}

// StartVerification creates a new SAS verification transaction.
// Generates a random transaction ID and commitment, returns the transaction in Started state.
func (s *SASVerificationService) StartVerification(ctx context.Context, theirUserID id.UserID, theirDeviceID id.DeviceID) (*SASVerificationTransaction, error) {
	if s == nil {
		return nil, nil
	}

	// Generate random transaction ID (32 bytes, base64url)
	txBuf := make([]byte, 32)
	if _, err := rand.Read(txBuf); err != nil {
		return nil, fmt.Errorf("crypto/verification: failed to generate transaction ID: %w", err)
	}
	txID := base64.RawURLEncoding.EncodeToString(txBuf)

	// Generate our ephemeral key (32 bytes random for this simplified implementation)
	ourKey := make([]byte, 32)
	if _, err := rand.Read(ourKey); err != nil {
		return nil, fmt.Errorf("crypto/verification: failed to generate ephemeral key: %w", err)
	}

	ourDeviceID := s.engine.GetDeviceID()

	tx := &SASVerificationTransaction{
		TransactionID: txID,
		TheirUserID:   theirUserID,
		TheirDeviceID: theirDeviceID,
		OurDeviceID:   ourDeviceID,
		State:         SASStateStarted,
		StartedAt:     time.Now(),
		OurKey:        ourKey,
		Emoji:         []string{},
	}

	s.mu.Lock()
	s.active[txID] = tx
	s.mu.Unlock()

	s.logger.Debug("SAS verification started",
		"transaction_id", txID,
		"their_user_id", theirUserID,
		"their_device_id", theirDeviceID,
	)

	return tx, nil
}

// AcceptVerification moves a transaction to Accepted state and generates key agreement data.
func (s *SASVerificationService) AcceptVerification(ctx context.Context, transactionID string) error {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, ok := s.active[transactionID]
	if !ok {
		return fmt.Errorf("crypto/verification: transaction %s not found", transactionID)
	}

	if tx.State != SASStateStarted {
		return fmt.Errorf("crypto/verification: transaction %s in state %d, expected Started", transactionID, tx.State)
	}

	// Generate key agreement data (random 32 bytes as shared secret material)
	keyData := make([]byte, 32)
	if _, err := rand.Read(keyData); err != nil {
		return fmt.Errorf("crypto/verification: failed to generate key agreement data: %w", err)
	}
	tx.KeyAgreementData = keyData
	tx.State = SASStateAccepted

	s.logger.Debug("SAS verification accepted", "transaction_id", transactionID)
	return nil
}

// ExchangeKeys exchanges key agreement, computes shared secret, generates emoji,
// and moves to KeyExchanged state.
// Returns our public key bytes for transmission to the other party.
func (s *SASVerificationService) ExchangeKeys(ctx context.Context, transactionID string, theirKey []byte) ([]byte, error) {
	if s == nil {
		return nil, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, ok := s.active[transactionID]
	if !ok {
		return nil, fmt.Errorf("crypto/verification: transaction %s not found", transactionID)
	}

	if tx.State != SASStateAccepted {
		return nil, fmt.Errorf("crypto/verification: transaction %s in state %d, expected Accepted", transactionID, tx.State)
	}

	// Store their key
	tx.TheirKey = make([]byte, len(theirKey))
	copy(tx.TheirKey, theirKey)

	// Compute commitment: SHA256(ourKey + theirKey)
	h := sha256.New()
	h.Write(tx.OurKey)
	h.Write(theirKey)
	tx.Commitment = h.Sum(nil)

	// Derive shared secret from key agreement data + both keys
	// For this simplified implementation, derive from HMAC-SHA256(keyAgreementData, ourKey||theirKey)
	mac := hmac.New(sha256.New, tx.KeyAgreementData)
	mac.Write(tx.OurKey)
	mac.Write(theirKey)
	sharedSecret := mac.Sum(nil)

	// Generate 6 emoji from shared secret bytes
	// Take 6 bytes, use lower 7 bits of each as index into sasEmojis[64]
	emoji := make([]string, 6)
	for i := 0; i < 6 && i < len(sharedSecret); i++ {
		idx := sharedSecret[i] & 0x3F // lower 6 bits → 0..63
		emoji[i] = sasEmojis[idx]
	}
	tx.Emoji = emoji
	tx.KeyAgreementData = sharedSecret
	tx.State = SASStateKeyExchanged

	s.logger.Debug("SAS verification keys exchanged",
		"transaction_id", transactionID,
		"emoji_count", len(emoji),
	)

	// Return our key for transmission
	ourKey := make([]byte, len(tx.OurKey))
	copy(ourKey, tx.OurKey)
	return ourKey, nil
}

// ConfirmVerification confirms the emoji match, computes our MAC, and moves to Confirmed state.
// Returns our MAC for transmission.
func (s *SASVerificationService) ConfirmVerification(ctx context.Context, transactionID string) ([]byte, error) {
	if s == nil {
		return nil, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, ok := s.active[transactionID]
	if !ok {
		return nil, fmt.Errorf("crypto/verification: transaction %s not found", transactionID)
	}

	if tx.State != SASStateKeyExchanged {
		return nil, fmt.Errorf("crypto/verification: transaction %s in state %d, expected KeyExchanged", transactionID, tx.State)
	}

	// Compute MAC: HMAC-SHA256(sharedSecret, ourUserID + ourDeviceID + theirUserID + theirDeviceID + ourKey + theirKey)
	mac := hmac.New(sha256.New, tx.KeyAgreementData)
	mac.Write([]byte(s.engine.GetUserID()))
	mac.Write([]byte(tx.OurDeviceID))
	mac.Write([]byte(tx.TheirUserID))
	mac.Write([]byte(tx.TheirDeviceID))
	mac.Write(tx.OurKey)
	mac.Write(tx.TheirKey)
	ourMAC := mac.Sum(nil)

	tx.OurMAC = ourMAC
	tx.State = SASStateConfirmed

	s.logger.Debug("SAS verification confirmed", "transaction_id", transactionID)

	result := make([]byte, len(ourMAC))
	copy(result, ourMAC)
	return result, nil
}

// VerifyMAC verifies the other party's MAC and moves to Done state.
func (s *SASVerificationService) VerifyMAC(ctx context.Context, transactionID string, theirMAC []byte) error {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, ok := s.active[transactionID]
	if !ok {
		return fmt.Errorf("crypto/verification: transaction %s not found", transactionID)
	}

	if tx.State != SASStateConfirmed {
		return fmt.Errorf("crypto/verification: transaction %s in state %d, expected Confirmed", transactionID, tx.State)
	}

	// Compute expected MAC from their perspective:
	// HMAC-SHA256(sharedSecret, theirUserID + theirDeviceID + ourUserID + ourDeviceID + theirKey + ourKey)
	expectedMAC := hmac.New(sha256.New, tx.KeyAgreementData)
	expectedMAC.Write([]byte(tx.TheirUserID))
	expectedMAC.Write([]byte(tx.TheirDeviceID))
	expectedMAC.Write([]byte(s.engine.GetUserID()))
	expectedMAC.Write([]byte(tx.OurDeviceID))
	expectedMAC.Write(tx.TheirKey)
	expectedMAC.Write(tx.OurKey)
	expectedSum := expectedMAC.Sum(nil)

	if !hmac.Equal(expectedSum, theirMAC) {
		tx.State = SASStateCancelled
		tx.CancelReason = "MAC verification failed"
		s.logger.Warn("SAS verification MAC mismatch",
			"transaction_id", transactionID,
		)
		return fmt.Errorf("crypto/verification: MAC verification failed for transaction %s", transactionID)
	}

	tx.TheirMAC = make([]byte, len(theirMAC))
	copy(tx.TheirMAC, theirMAC)
	tx.State = SASStateDone

	s.logger.Debug("SAS verification completed successfully", "transaction_id", transactionID)
	return nil
}

// CancelVerification cancels an active verification with a reason.
func (s *SASVerificationService) CancelVerification(ctx context.Context, transactionID string, reason string) error {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, ok := s.active[transactionID]
	if !ok {
		return fmt.Errorf("crypto/verification: transaction %s not found", transactionID)
	}

	tx.State = SASStateCancelled
	tx.CancelReason = reason

	s.logger.Debug("SAS verification cancelled",
		"transaction_id", transactionID,
		"reason", reason,
	)
	return nil
}

// GetTransaction returns an active transaction by ID.
func (s *SASVerificationService) GetTransaction(transactionID string) *SASVerificationTransaction {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.active[transactionID]
}

// GetEmoji returns the emoji for display for a given transaction.
func (s *SASVerificationService) GetEmoji(transactionID string) []string {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	tx, ok := s.active[transactionID]
	if !ok {
		return nil
	}

	result := make([]string, len(tx.Emoji))
	copy(result, tx.Emoji)
	return result
}

// CleanupExpired removes transactions older than the timeout duration.
func (s *SASVerificationService) CleanupExpired() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, tx := range s.active {
		if now.Sub(tx.StartedAt) > s.timeout {
			s.logger.Debug("SAS verification expired, cleaning up",
				"transaction_id", id,
				"age", now.Sub(tx.StartedAt),
			)
			delete(s.active, id)
		}
	}
}
