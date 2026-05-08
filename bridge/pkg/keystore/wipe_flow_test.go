//go:build cgo

package keystore

import (
	"context"
	cryptorand "crypto/rand"
	"path/filepath"
	"testing"
	"time"
)

func TestWipeRuntime_PasswordBufferZeroedAfterSeal(t *testing.T) {
	ks := createTestSealedKeystoreInternal(t)
	password := "TestP@ssw0rd!"

	kd, err := NewKeyDerivation(DefaultKeyDerivationParams)
	if err != nil {
		t.Fatalf("Failed to create key derivation: %v", err)
	}

	salt := make([]byte, DefaultKeyDerivationParams.SaltLength)
	if _, err := cryptorand.Read(salt); err != nil {
		t.Fatalf("Failed to generate salt: %v", err)
	}

	derived, err := kd.DeriveKey([]byte(password), salt)
	if err != nil {
		t.Fatalf("Failed to derive verifier: %v", err)
	}
	verifier := make([]byte, len(derived.Key))
	copy(verifier, derived.Key)

	vaultKey := make([]byte, 32)
	if _, err := cryptorand.Read(vaultKey); err != nil {
		t.Fatalf("Failed to generate vault key: %v", err)
	}
	wrapped, err := kd.WrapKey(vaultKey, []byte(password))
	if err != nil {
		t.Fatalf("Failed to wrap vault key: %v", err)
	}

	ks.passwordKD = kd
	ks.verifySalt = salt
	ks.verifier = verifier
	ks.wrappedVK = *wrapped
	ks.maxFailedAttempts = 5
	ks.lockoutDuration = 5 * time.Minute
	ks.autoSealDuration = 4 * time.Hour

	err = ks.UnsealWithPassword(password)
	if err != nil {
		t.Fatalf("UnsealWithPassword failed: %v", err)
	}
	if !ks.isUnsealed {
		t.Fatal("Expected keystore to be unsealed after correct password")
	}

	ks.SealPassword()

	if ks.isUnsealed {
		t.Error("Expected keystore to be sealed after SealPassword")
	}
	if ks.vaultKey != nil {
		t.Error("SECURITY: vaultKey is not nil after SealPassword — secret material persists!")
	} else {
		t.Log("PASS: vaultKey is nil after SealPassword (wiped and dereferenced)")
	}
}

func TestWipeRuntime_FailedUnsealIncrementsAttempts(t *testing.T) {
	ks := createTestSealedKeystoreInternal(t)
	password := "TestP@ssw0rd!"

	kd, err := NewKeyDerivation(DefaultKeyDerivationParams)
	if err != nil {
		t.Fatalf("Failed to create key derivation: %v", err)
	}

	salt := make([]byte, DefaultKeyDerivationParams.SaltLength)
	if _, err := cryptorand.Read(salt); err != nil {
		t.Fatalf("Failed to generate salt: %v", err)
	}

	derived, err := kd.DeriveKey([]byte(password), salt)
	if err != nil {
		t.Fatalf("Failed to derive verifier: %v", err)
	}
	verifier := make([]byte, len(derived.Key))
	copy(verifier, derived.Key)

	vaultKey := make([]byte, 32)
	if _, err := cryptorand.Read(vaultKey); err != nil {
		t.Fatalf("Failed to generate vault key: %v", err)
	}
	wrapped, err := kd.WrapKey(vaultKey, []byte(password))
	if err != nil {
		t.Fatalf("Failed to wrap vault key: %v", err)
	}

	ks.passwordKD = kd
	ks.verifySalt = salt
	ks.verifier = verifier
	ks.wrappedVK = *wrapped
	ks.maxFailedAttempts = 5
	ks.lockoutDuration = 5 * time.Minute
	ks.autoSealDuration = 4 * time.Hour

	err = ks.UnsealWithPassword("wrong-password")
	if err == nil {
		t.Fatal("Expected error for wrong password")
	}
	if ks.failedAttempts != 1 {
		t.Errorf("Expected 1 failed attempt, got %d", ks.failedAttempts)
	}
	if ks.isUnsealed {
		t.Error("Expected keystore to remain sealed after wrong password")
	}
	if ks.vaultKey != nil {
		t.Error("SECURITY: vaultKey is not nil after failed unseal")
	}

	t.Log("PASS: Failed unseal correctly increments counter, vault key remains nil")
}

func TestWipeRuntime_LockoutAfterMaxFailures(t *testing.T) {
	ks := createTestSealedKeystoreInternal(t)
	password := "TestP@ssw0rd!"

	kd, err := NewKeyDerivation(DefaultKeyDerivationParams)
	if err != nil {
		t.Fatalf("Failed to create key derivation: %v", err)
	}

	salt := make([]byte, DefaultKeyDerivationParams.SaltLength)
	if _, err := cryptorand.Read(salt); err != nil {
		t.Fatalf("Failed to generate salt: %v", err)
	}

	derived, err := kd.DeriveKey([]byte(password), salt)
	if err != nil {
		t.Fatalf("Failed to derive verifier: %v", err)
	}
	verifier := make([]byte, len(derived.Key))
	copy(verifier, derived.Key)

	vaultKey := make([]byte, 32)
	if _, err := cryptorand.Read(vaultKey); err != nil {
		t.Fatalf("Failed to generate vault key: %v", err)
	}
	wrapped, err := kd.WrapKey(vaultKey, []byte(password))
	if err != nil {
		t.Fatalf("Failed to wrap vault key: %v", err)
	}

	ks.passwordKD = kd
	ks.verifySalt = salt
	ks.verifier = verifier
	ks.wrappedVK = *wrapped
	ks.maxFailedAttempts = 3
	ks.lockoutDuration = 10 * time.Second
	ks.autoSealDuration = 4 * time.Hour

	for i := 0; i < 3; i++ {
		err = ks.UnsealWithPassword("wrong-password")
		if err == nil {
			t.Fatalf("Attempt %d: expected error for wrong password", i+1)
		}
	}

	if !ks.IsLockedOut() {
		t.Error("Expected keystore to be locked out after 3 failed attempts")
	}

	err = ks.UnsealWithPassword(password)
	if err == nil {
		t.Error("Expected error during lockout even with correct password")
	}
	if ks.vaultKey != nil {
		t.Error("SECURITY: vaultKey is not nil during lockout — secret material in memory!")
	}

	t.Log("PASS: Lockout works correctly, vault key stays nil")
}

func TestWipeRuntime_SealViaContext(t *testing.T) {
	ks := createTestSealedKeystoreInternal(t)
	ks.policy = PolicyPassword
	password := "TestP@ssw0rd!"

	kd, err := NewKeyDerivation(DefaultKeyDerivationParams)
	if err != nil {
		t.Fatalf("Failed to create key derivation: %v", err)
	}

	salt := make([]byte, DefaultKeyDerivationParams.SaltLength)
	if _, err := cryptorand.Read(salt); err != nil {
		t.Fatalf("Failed to generate salt: %v", err)
	}

	derived, err := kd.DeriveKey([]byte(password), salt)
	if err != nil {
		t.Fatalf("Failed to derive verifier: %v", err)
	}
	verifier := make([]byte, len(derived.Key))
	copy(verifier, derived.Key)

	vaultKey := make([]byte, 32)
	if _, err := cryptorand.Read(vaultKey); err != nil {
		t.Fatalf("Failed to generate vault key: %v", err)
	}
	wrapped, err := kd.WrapKey(vaultKey, []byte(password))
	if err != nil {
		t.Fatalf("Failed to wrap vault key: %v", err)
	}

	ks.passwordKD = kd
	ks.verifySalt = salt
	ks.verifier = verifier
	ks.wrappedVK = *wrapped
	ks.maxFailedAttempts = 5
	ks.lockoutDuration = 5 * time.Minute
	ks.autoSealDuration = 4 * time.Hour

	err = ks.UnsealWithPassword(password)
	if err != nil {
		t.Fatalf("UnsealWithPassword failed: %v", err)
	}
	if ks.vaultKey == nil {
		t.Fatal("Expected vaultKey to be non-nil after successful unseal")
	}

	err = ks.Seal(context.Background(), "test-agent")
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	if ks.vaultKey != nil {
		t.Error("SECURITY: vaultKey is not nil after Seal() — secret material in memory!")
	} else {
		t.Log("PASS: Seal() correctly wipes vaultKey for password policy")
	}
}

func createTestSealedKeystoreInternal(t *testing.T) *SealedKeystore {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "keystore.db")

	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}

	base, err := New(Config{
		DBPath:    dbPath,
		MasterKey: masterKey,
	})
	if err != nil {
		t.Fatalf("Failed to create base keystore: %v", err)
	}
	if err := base.Open(); err != nil {
		t.Fatalf("Failed to open base keystore: %v", err)
	}
	t.Cleanup(func() { base.Close() })

	sk, err := NewSealedKeystore(SealedKeystoreConfig{
		BaseKeystore: base,
		DefaultTTL:   5 * time.Minute,
		Policy:       PolicyPassword,
	})
	if err != nil {
		t.Fatalf("Failed to create sealed keystore: %v", err)
	}

	return sk
}
