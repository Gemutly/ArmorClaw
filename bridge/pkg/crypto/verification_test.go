package crypto

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"

	"maunium.net/go/mautrix/id"
)

func TestNewSASVerificationService_NilEngine(t *testing.T) {
	svc := NewSASVerificationService(nil)
	if svc != nil {
		t.Fatal("expected nil service when engine is nil")
	}
}

func TestSASVerification_FullFlow(t *testing.T) {
	engine := &CryptoEngine{
		deviceID: id.DeviceID("DEVICE_ABC"),
		userID:   id.UserID("@alice:example.com"),
	}
	svc := NewSASVerificationService(engine)
	ctx := context.Background()

	theirUserID := id.UserID("@bob:example.com")
	theirDeviceID := id.DeviceID("DEVICE_XYZ")

	tx, err := svc.StartVerification(ctx, theirUserID, theirDeviceID)
	if err != nil {
		t.Fatalf("StartVerification failed: %v", err)
	}
	if tx == nil {
		t.Fatal("StartVerification returned nil transaction")
	}
	if tx.State != SASStateStarted {
		t.Fatalf("expected Started state, got %d", tx.State)
	}
	if tx.TheirUserID != theirUserID {
		t.Fatalf("expected their user ID %s, got %s", theirUserID, tx.TheirUserID)
	}
	if tx.TheirDeviceID != theirDeviceID {
		t.Fatalf("expected their device ID %s, got %s", theirDeviceID, tx.TheirDeviceID)
	}
	txID := tx.TransactionID

	err = svc.AcceptVerification(ctx, txID)
	if err != nil {
		t.Fatalf("AcceptVerification failed: %v", err)
	}
	tx = svc.GetTransaction(txID)
	if tx.State != SASStateAccepted {
		t.Fatalf("expected Accepted state, got %d", tx.State)
	}

	theirKey := make([]byte, 32)
	rand.Read(theirKey)

	ourKey, err := svc.ExchangeKeys(ctx, txID, theirKey)
	if err != nil {
		t.Fatalf("ExchangeKeys failed: %v", err)
	}
	if len(ourKey) != 32 {
		t.Fatalf("expected 32-byte our key, got %d bytes", len(ourKey))
	}
	tx = svc.GetTransaction(txID)
	if tx.State != SASStateKeyExchanged {
		t.Fatalf("expected KeyExchanged state, got %d", tx.State)
	}

	emoji := svc.GetEmoji(txID)
	if len(emoji) != 6 {
		t.Fatalf("expected 6 emoji, got %d", len(emoji))
	}

	ourMAC, err := svc.ConfirmVerification(ctx, txID)
	if err != nil {
		t.Fatalf("ConfirmVerification failed: %v", err)
	}
	if len(ourMAC) != 32 {
		t.Fatalf("expected 32-byte MAC, got %d bytes", len(ourMAC))
	}
	tx = svc.GetTransaction(txID)
	if tx.State != SASStateConfirmed {
		t.Fatalf("expected Confirmed state, got %d", tx.State)
	}

	theirMAC := computeTheirMAC(tx, engine)
	err = svc.VerifyMAC(ctx, txID, theirMAC)
	if err != nil {
		t.Fatalf("VerifyMAC failed: %v", err)
	}
	tx = svc.GetTransaction(txID)
	if tx.State != SASStateDone {
		t.Fatalf("expected Done state, got %d", tx.State)
	}
}

func TestSASVerification_Cancel(t *testing.T) {
	engine := &CryptoEngine{
		deviceID: id.DeviceID("DEVICE_ABC"),
		userID:   id.UserID("@alice:example.com"),
	}
	svc := NewSASVerificationService(engine)
	ctx := context.Background()

	tx, err := svc.StartVerification(ctx, "@bob:example.com", "DEVICE_XYZ")
	if err != nil {
		t.Fatalf("StartVerification failed: %v", err)
	}

	reason := "user declined"
	err = svc.CancelVerification(ctx, tx.TransactionID, reason)
	if err != nil {
		t.Fatalf("CancelVerification failed: %v", err)
	}

	tx = svc.GetTransaction(tx.TransactionID)
	if tx.State != SASStateCancelled {
		t.Fatalf("expected Cancelled state, got %d", tx.State)
	}
	if tx.CancelReason != reason {
		t.Fatalf("expected cancel reason %q, got %q", reason, tx.CancelReason)
	}
}

func TestSASVerification_Timeout(t *testing.T) {
	engine := &CryptoEngine{
		deviceID: id.DeviceID("DEVICE_ABC"),
		userID:   id.UserID("@alice:example.com"),
	}
	svc := NewSASVerificationService(engine)
	ctx := context.Background()

	tx, _ := svc.StartVerification(ctx, "@bob:example.com", "DEVICE_XYZ")
	txID := tx.TransactionID

	svc.mu.Lock()
	svc.active[txID].StartedAt = time.Now().Add(-15 * time.Minute)
	svc.mu.Unlock()

	svc.CleanupExpired()

	if svc.GetTransaction(txID) != nil {
		t.Fatal("expected expired transaction to be cleaned up")
	}
}

func TestSASVerification_NilSafeMethods(t *testing.T) {
	var svc *SASVerificationService
	ctx := context.Background()

	tx, err := svc.StartVerification(ctx, "@bob:example.com", "DEVICE_XYZ")
	if tx != nil || err != nil {
		t.Fatalf("expected nil/nil from StartVerification on nil service, got tx=%v err=%v", tx, err)
	}

	err = svc.AcceptVerification(ctx, "any")
	if err != nil {
		t.Fatalf("expected nil error from AcceptVerification on nil service, got %v", err)
	}

	key, err := svc.ExchangeKeys(ctx, "any", []byte{1, 2, 3})
	if key != nil || err != nil {
		t.Fatalf("expected nil/nil from ExchangeKeys on nil service, got key=%v err=%v", key, err)
	}

	mac, err := svc.ConfirmVerification(ctx, "any")
	if mac != nil || err != nil {
		t.Fatalf("expected nil/nil from ConfirmVerification on nil service, got mac=%v err=%v", mac, err)
	}

	err = svc.VerifyMAC(ctx, "any", []byte{1, 2, 3})
	if err != nil {
		t.Fatalf("expected nil error from VerifyMAC on nil service, got %v", err)
	}

	err = svc.CancelVerification(ctx, "any", "reason")
	if err != nil {
		t.Fatalf("expected nil error from CancelVerification on nil service, got %v", err)
	}

	if svc.GetTransaction("any") != nil {
		t.Fatal("expected nil from GetTransaction on nil service")
	}

	if svc.GetEmoji("any") != nil {
		t.Fatal("expected nil from GetEmoji on nil service")
	}

	svc.CleanupExpired()
}

func TestSASVerification_EmojiGeneration(t *testing.T) {
	engine := &CryptoEngine{
		deviceID: id.DeviceID("DEVICE_ABC"),
		userID:   id.UserID("@alice:example.com"),
	}
	svc := NewSASVerificationService(engine)
	ctx := context.Background()

	tx, _ := svc.StartVerification(ctx, "@bob:example.com", "DEVICE_XYZ")
	svc.AcceptVerification(ctx, tx.TransactionID)
	svc.ExchangeKeys(ctx, tx.TransactionID, make([]byte, 32))

	emoji := svc.GetEmoji(tx.TransactionID)
	if len(emoji) != 6 {
		t.Fatalf("expected 6 emoji, got %d", len(emoji))
	}
	for i, e := range emoji {
		if e == "" {
			t.Fatalf("emoji[%d] is empty", i)
		}
		found := false
		for _, candidate := range sasEmojis {
			if candidate == e {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("emoji[%d] = %q not in sasEmojis", i, e)
		}
	}
}

func TestSASVerification_MACTamperDetection(t *testing.T) {
	engine := &CryptoEngine{
		deviceID: id.DeviceID("DEVICE_ABC"),
		userID:   id.UserID("@alice:example.com"),
	}
	svc := NewSASVerificationService(engine)
	ctx := context.Background()

	tx, _ := svc.StartVerification(ctx, "@bob:example.com", "DEVICE_XYZ")
	txID := tx.TransactionID

	svc.AcceptVerification(ctx, txID)
	svc.ExchangeKeys(ctx, txID, make([]byte, 32))
	svc.ConfirmVerification(ctx, txID)

	tamperedMAC := make([]byte, 32)
	for i := range tamperedMAC {
		tamperedMAC[i] = 0xFF
	}

	err := svc.VerifyMAC(ctx, txID, tamperedMAC)
	if err == nil {
		t.Fatal("expected error for tampered MAC, got nil")
	}

	tx = svc.GetTransaction(txID)
	if tx.State != SASStateCancelled {
		t.Fatalf("expected Cancelled state after MAC failure, got %d", tx.State)
	}
	if tx.CancelReason != "MAC verification failed" {
		t.Fatalf("expected 'MAC verification failed' reason, got %q", tx.CancelReason)
	}
}

func computeTheirMAC(tx *SASVerificationTransaction, engine *CryptoEngine) []byte {
	mac := hmac.New(sha256.New, tx.KeyAgreementData)
	mac.Write([]byte(tx.TheirUserID))
	mac.Write([]byte(tx.TheirDeviceID))
	mac.Write([]byte(engine.GetUserID()))
	mac.Write([]byte(tx.OurDeviceID))
	mac.Write(tx.TheirKey)
	mac.Write(tx.OurKey)
	return mac.Sum(nil)
}
