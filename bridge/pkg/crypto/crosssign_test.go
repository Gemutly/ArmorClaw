package crypto

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"

	"maunium.net/go/mautrix/id"
)

func TestNewCrossSigningService_NilEngine(t *testing.T) {
	store := NewMemoryStore()
	svc := NewCrossSigningService(nil, store)
	if svc != nil {
		t.Fatal("expected nil when engine is nil")
	}
}

func TestCrossSigning_GenerateKeyPair(t *testing.T) {
	store := NewMemoryStore()
	engine := &CryptoEngine{}
	svc := NewCrossSigningService(engine, store)
	if svc == nil {
		t.Fatal("service should not be nil")
	}

	pair, err := svc.GenerateKeyPair(CrossSigningKeyMaster)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	if pair == nil {
		t.Fatal("pair should not be nil")
	}

	if pair.Usage != CrossSigningKeyMaster {
		t.Fatalf("usage = %q, want %q", pair.Usage, CrossSigningKeyMaster)
	}
	if len(pair.Public) != ed25519.PublicKeySize {
		t.Fatalf("public key len = %d, want %d", len(pair.Public), ed25519.PublicKeySize)
	}
	if len(pair.Private) != ed25519.PrivateKeySize {
		t.Fatalf("private key len = %d, want %d", len(pair.Private), ed25519.PrivateKeySize)
	}

	wantPrefix := "ed25519:"
	if !strings.HasPrefix(pair.KeyID, wantPrefix) {
		t.Fatalf("keyID = %q, want prefix %q", pair.KeyID, wantPrefix)
	}

	encoded := strings.TrimPrefix(pair.KeyID, wantPrefix)
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("keyID base64 decode: %v", err)
	}
	if string(decoded) != string(pair.Public) {
		t.Fatal("keyID does not match public key")
	}
}

func TestCrossSigning_SignAndVerify(t *testing.T) {
	store := NewMemoryStore()
	engine := &CryptoEngine{}
	svc := NewCrossSigningService(engine, store)

	pair, err := svc.GenerateKeyPair(CrossSigningKeySelfSigning)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	data := []byte("test data for signing")
	sig, err := svc.SignKey(pair, data)
	if err != nil {
		t.Fatalf("SignKey: %v", err)
	}

	if !svc.VerifySignature(pair.Public, data, sig) {
		t.Fatal("VerifySignature should pass for correct data")
	}

	wrongData := []byte("wrong data")
	if svc.VerifySignature(pair.Public, wrongData, sig) {
		t.Fatal("VerifySignature should fail for wrong data")
	}
}

func TestCrossSigning_Bootstrap(t *testing.T) {
	store := NewMemoryStore()
	engine := &CryptoEngine{}
	svc := NewCrossSigningService(engine, store)

	ctx := context.Background()
	userID := id.UserID("@alice:example.com")
	deviceID := id.DeviceID("DEVICE1")

	result, err := svc.Bootstrap(ctx, userID, deviceID)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}

	if result.MasterKey == nil {
		t.Fatal("master key should not be nil")
	}
	if result.SelfSigningKey == nil {
		t.Fatal("self-signing key should not be nil")
	}
	if result.UserSigningKey == nil {
		t.Fatal("user-signing key should not be nil")
	}

	if result.MasterKey.Usage != CrossSigningKeyMaster {
		t.Fatalf("master usage = %q, want %q", result.MasterKey.Usage, CrossSigningKeyMaster)
	}
	if result.SelfSigningKey.Usage != CrossSigningKeySelfSigning {
		t.Fatalf("ssk usage = %q, want %q", result.SelfSigningKey.Usage, CrossSigningKeySelfSigning)
	}
	if result.UserSigningKey.Usage != CrossSigningKeyUserSigning {
		t.Fatalf("usk usage = %q, want %q", result.UserSigningKey.Usage, CrossSigningKeyUserSigning)
	}

	if !result.DeviceSigned {
		t.Fatal("DeviceSigned should be true")
	}
	if result.CompletedAt.IsZero() {
		t.Fatal("CompletedAt should not be zero")
	}

	keys, err := store.GetCrossSigningKeys(ctx, string(userID))
	if err != nil {
		t.Fatalf("GetCrossSigningKeys: %v", err)
	}
	if len(keys) != 3 {
		t.Fatalf("stored %d keys, want 3", len(keys))
	}

	usages := map[string]bool{}
	for _, k := range keys {
		usages[k.Usage] = true
	}
	for _, want := range []string{"master", "self_signing", "user_signing"} {
		if !usages[want] {
			t.Fatalf("missing key with usage %q", want)
		}
	}

	deviceKeyRef := string(userID) + ":" + string(deviceID)
	sigs, err := store.GetSignatures(ctx, deviceKeyRef)
	if err != nil {
		t.Fatalf("GetSignatures: %v", err)
	}
	if len(sigs) == 0 {
		t.Fatal("device should have a signature from master key")
	}
}

func TestCrossSigning_IsBootstrapped(t *testing.T) {
	store := NewMemoryStore()
	engine := &CryptoEngine{}
	svc := NewCrossSigningService(engine, store)

	ctx := context.Background()
	userID := id.UserID("@alice:example.com")

	bootstrapped, err := svc.IsBootstrapped(ctx, string(userID))
	if err != nil {
		t.Fatalf("IsBootstrapped before: %v", err)
	}
	if bootstrapped {
		t.Fatal("should not be bootstrapped before Bootstrap()")
	}

	deviceID := id.DeviceID("DEVICE1")
	_, err = svc.Bootstrap(ctx, userID, deviceID)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	bootstrapped, err = svc.IsBootstrapped(ctx, string(userID))
	if err != nil {
		t.Fatalf("IsBootstrapped after: %v", err)
	}
	if !bootstrapped {
		t.Fatal("should be bootstrapped after Bootstrap()")
	}
}

func TestCrossSigning_NilSafeMethods(t *testing.T) {
	var svc *CrossSigningService

	pair, err := svc.GenerateKeyPair(CrossSigningKeyMaster)
	if pair != nil || err != nil {
		t.Fatalf("GenerateKeyPair on nil: pair=%v err=%v", pair, err)
	}

	sig, err := svc.SignKey(nil, nil)
	if sig != nil || err != nil {
		t.Fatalf("SignKey on nil: sig=%v err=%v", sig, err)
	}

	if svc.VerifySignature(nil, nil, nil) {
		t.Fatal("VerifySignature on nil should return false")
	}

	ctx := context.Background()
	result, err := svc.Bootstrap(ctx, "@alice:example.com", "DEV1")
	if result != nil || err != nil {
		t.Fatalf("Bootstrap on nil: result=%v err=%v", result, err)
	}

	strategy, err := svc.UploadKeys(ctx, "@alice:example.com", nil)
	if strategy != UIAAStrategyNone || err != nil {
		t.Fatalf("UploadKeys on nil: strategy=%v err=%v", strategy, err)
	}

	bootstrapped, err := svc.IsBootstrapped(ctx, "@alice:example.com")
	if bootstrapped || err != nil {
		t.Fatalf("IsBootstrapped on nil: bootstrapped=%v err=%v", bootstrapped, err)
	}

	if err := svc.SignDevice(ctx, "@alice:example.com", "DEV1", nil); err != nil {
		t.Fatalf("SignDevice on nil: err=%v", err)
	}
}

func TestCrossSigning_SignDevice(t *testing.T) {
	store := NewMemoryStore()
	engine := &CryptoEngine{}
	svc := NewCrossSigningService(engine, store)

	ctx := context.Background()
	userID := id.UserID("@bob:example.com")
	deviceID := id.DeviceID("DEVICE42")

	msk, err := svc.GenerateKeyPair(CrossSigningKeyMaster)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	err = svc.SignDevice(ctx, userID, deviceID, msk)
	if err != nil {
		t.Fatalf("SignDevice: %v", err)
	}

	deviceKeyRef := string(userID) + ":" + string(deviceID)
	sigs, err := store.GetSignatures(ctx, deviceKeyRef)
	if err != nil {
		t.Fatalf("GetSignatures: %v", err)
	}
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signature, got %d", len(sigs))
	}

	sig := sigs[0]
	if sig.SignerUserID != string(userID) {
		t.Fatalf("signer user ID = %q, want %q", sig.SignerUserID, string(userID))
	}
	if sig.SignerKeyID != msk.KeyID {
		t.Fatalf("signer key ID = %q, want %q", sig.SignerKeyID, msk.KeyID)
	}

	decodedSig, err := base64.RawURLEncoding.DecodeString(sig.Signature)
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}

	deviceKeyRefBytes := []byte(deviceKeyRef)
	if !ed25519.Verify(msk.Public, deviceKeyRefBytes, decodedSig) {
		t.Fatal("signature verification failed")
	}
}
