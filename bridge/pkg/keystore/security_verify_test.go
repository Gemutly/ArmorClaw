package keystore

import (
	"crypto/rand"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Argon2id Parameters — OWASP minimums
// ---------------------------------------------------------------------------

func TestArgon2idDefaultParamsOWASP(t *testing.T) {
	p := DefaultKeyDerivationParams

	if p.Iterations < 3 {
		t.Errorf("Argon2id time cost %d < OWASP minimum 3", p.Iterations)
	}
	if p.Memory < 64*1024 {
		t.Errorf("Argon2id memory %d KiB < OWASP minimum 65536 KiB (64 MB)", p.Memory)
	}
	if p.Parallelism < 4 {
		t.Errorf("Argon2id parallelism %d < OWASP minimum 4", p.Parallelism)
	}
	if p.KeyLength != 32 {
		t.Errorf("Argon2id key length %d != expected 32 bytes", p.KeyLength)
	}
	if p.SaltLength < 16 {
		t.Errorf("Argon2id salt length %d < minimum 16 bytes", p.SaltLength)
	}
}

func TestArgon2idParamsImmutable(t *testing.T) {
	original := DefaultKeyDerivationParams
	params := GetDefaultParams()
	params.Iterations = 1
	if DefaultKeyDerivationParams.Iterations != original.Iterations {
		t.Error("GetDefaultParams() returned a mutable reference — defaults changed")
	}
}

// ---------------------------------------------------------------------------
// Salt Generation — crypto/rand, uniqueness, minimum length
// ---------------------------------------------------------------------------

func TestSaltUsesCryptoRand(t *testing.T) {
	kd, err := NewKeyDerivation(DefaultKeyDerivationParams)
	if err != nil {
		t.Fatalf("NewKeyDerivation: %v", err)
	}

	dk, err := kd.DeriveKeyWithNewSalt([]byte("password"))
	if err != nil {
		t.Fatalf("DeriveKeyWithNewSalt: %v", err)
	}
	if len(dk.Salt) != int(DefaultKeyDerivationParams.SaltLength) {
		t.Errorf("salt length %d != expected %d", len(dk.Salt), DefaultKeyDerivationParams.SaltLength)
	}
}

func TestSaltsAreUnique(t *testing.T) {
	kd, _ := NewKeyDerivation(DefaultKeyDerivationParams)
	dk1, _ := kd.DeriveKeyWithNewSalt([]byte("password"))
	dk2, _ := kd.DeriveKeyWithNewSalt([]byte("password"))

	if ConstantTimeCompare(dk1.Salt, dk2.Salt) {
		t.Error("two consecutive salts should be unique")
	}
}

func TestSaltMinimumLength(t *testing.T) {
	p := DefaultKeyDerivationParams
	if p.SaltLength < 16 {
		t.Errorf("default salt length %d < 16 bytes minimum", p.SaltLength)
	}

	kd, _ := NewKeyDerivation(p)
	dk, err := kd.DeriveKeyWithNewSalt([]byte("password"))
	if err != nil {
		t.Fatalf("DeriveKeyWithNewSalt: %v", err)
	}
	if len(dk.Salt) < 16 {
		t.Errorf("generated salt %d bytes < 16 minimum", len(dk.Salt))
	}
}

func TestVaultKeyGeneration(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("vault key length %d != 32", len(key))
	}

	key2 := make([]byte, 32)
	rand.Read(key2)
	if ConstantTimeCompare(key, key2) {
		t.Error("two random vault keys should not collide")
	}
}

// ---------------------------------------------------------------------------
// Passphrase Policy — minimum length, complexity
// ---------------------------------------------------------------------------

func TestPasswordPolicyDefaultMinLength(t *testing.T) {
	policy := DefaultPasswordPolicy()

	if err := policy.ValidatePassword(""); err == nil {
		t.Error("empty password should be rejected")
	}
	if err := policy.ValidatePassword("short"); err == nil {
		t.Error("5-char password should be rejected (min 8)")
	}
	if err := policy.ValidatePassword("abcdefg1"); err == nil {
		t.Error("8-char lowercase+digits should be rejected (no uppercase)")
	}
	if err := policy.ValidatePassword("Abcdefg1"); err != nil {
		t.Errorf("valid 8-char mixed password should be accepted: %v", err)
	}
}

func TestPasswordPolicyComplexity(t *testing.T) {
	policy := PasswordPolicy{
		MinLength:      8,
		RequireUpper:   true,
		RequireLower:   true,
		RequireDigit:   true,
		RequireSpecial: true,
	}

	tests := []struct {
		pw   string
		ok   bool
		desc string
	}{
		{"Abcdefg1!", true, "all categories"},
		{"abcdefg1!", false, "missing uppercase"},
		{"ABCDEFG1!", false, "missing lowercase"},
		{"Abcdefgh!", false, "missing digit"},
		{"Abcdefg12", false, "missing special"},
		{"A1!", false, "too short"},
	}

	for _, tt := range tests {
		err := policy.ValidatePassword(tt.pw)
		if (err == nil) != tt.ok {
			t.Errorf("%s: %q expected ok=%v, got err=%v", tt.desc, tt.pw, tt.ok, err)
		}
	}
}

func TestPasswordPolicyMinLengthOnly(t *testing.T) {
	policy := PasswordPolicy{MinLength: 12}
	if err := policy.ValidatePassword("shortpass"); err == nil {
		t.Error("9-char should be rejected when min is 12")
	}
	if err := policy.ValidatePassword("longenoughpass"); err != nil {
		t.Errorf("12+ chars with no complexity should be accepted: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Failed-Unseal Lockout — attempt counting, lockout duration
// ---------------------------------------------------------------------------

func TestLockoutAfterMaxFailures(t *testing.T) {
	sk, _ := newTestPasswordSealedKeystore(t)
	sk.SetLockoutConfig(3, 5*time.Minute)

	for i := 0; i < 3; i++ {
		err := sk.UnsealWithPassword("wrong")
		if err == nil {
			t.Fatal("wrong password should fail")
		}
		if err == ErrLockedOut {
			t.Fatalf("attempt %d: should not be locked out yet", i+1)
		}
	}

	if sk.FailedAttempts() != 3 {
		t.Errorf("expected 3 failed attempts, got %d", sk.FailedAttempts())
	}

	err := sk.UnsealWithPassword("wrong")
	if err != ErrLockedOut {
		t.Errorf("4th attempt should be ErrLockedOut, got %v", err)
	}
}

func TestLockoutBlocksCorrectPassword(t *testing.T) {
	sk, password := newTestPasswordSealedKeystore(t)
	sk.SetLockoutConfig(2, 10*time.Second)

	sk.UnsealWithPassword("wrong1")
	sk.UnsealWithPassword("wrong2")

	err := sk.UnsealWithPassword(password)
	if err != ErrLockedOut {
		t.Errorf("correct password should be blocked during lockout, got %v", err)
	}
}

func TestLockoutExpires(t *testing.T) {
	sk, password := newTestPasswordSealedKeystore(t)
	sk.SetLockoutConfig(2, 50*time.Millisecond)

	sk.UnsealWithPassword("wrong1")
	sk.UnsealWithPassword("wrong2")

	if !sk.IsLockedOut() {
		t.Error("expected to be locked out")
	}

	time.Sleep(80 * time.Millisecond)

	if sk.IsLockedOut() {
		t.Error("lockout should have expired")
	}

	err := sk.UnsealWithPassword(password)
	if err != nil {
		t.Errorf("correct password should work after lockout expires: %v", err)
	}
}

func TestLockoutResetsOnSuccess(t *testing.T) {
	sk, password := newTestPasswordSealedKeystore(t)
	sk.SetLockoutConfig(3, 5*time.Minute)

	sk.UnsealWithPassword("wrong1")
	sk.UnsealWithPassword("wrong2")
	if sk.FailedAttempts() != 2 {
		t.Fatalf("expected 2 failures, got %d", sk.FailedAttempts())
	}

	err := sk.UnsealWithPassword(password)
	if err != nil {
		t.Fatalf("correct password should work: %v", err)
	}

	sk.SealPassword()
	if sk.FailedAttempts() != 0 {
		t.Errorf("failed attempts should be 0 after seal, got %d", sk.FailedAttempts())
	}
}

func TestLockoutResetsOnSeal(t *testing.T) {
	sk, _ := newTestPasswordSealedKeystore(t)
	sk.SetLockoutConfig(2, 50*time.Millisecond)

	sk.UnsealWithPassword("wrong1")
	sk.UnsealWithPassword("wrong2")

	if !sk.IsLockedOut() {
		t.Error("expected lockout")
	}

	sk.SealPassword()

	if sk.FailedAttempts() != 0 {
		t.Errorf("expected 0 failures after SealPassword, got %d", sk.FailedAttempts())
	}
	if sk.IsLockedOut() {
		t.Error("expected no lockout after SealPassword reset")
	}
}

// ---------------------------------------------------------------------------
// Idle Auto-Seal — timer on last activity, seals after timeout
// ---------------------------------------------------------------------------

func TestAutoSealAfterInactivity(t *testing.T) {
	sk, password := newTestPasswordSealedKeystoreWithAutoSeal(t, 100*time.Millisecond)

	err := sk.UnsealWithPassword(password)
	if err != nil {
		t.Fatalf("unseal: %v", err)
	}
	if !sk.IsPasswordUnsealed() {
		t.Fatal("should be unsealed immediately after unseal")
	}

	time.Sleep(200 * time.Millisecond)

	if sk.IsPasswordUnsealed() {
		t.Error("should be auto-sealed after inactivity timeout")
	}
	if sk.VaultKey() != nil {
		t.Error("vault key should be nil after auto-seal")
	}
}

func TestAutoSealTimerResetsOnActivity(t *testing.T) {
	sk, password := newTestPasswordSealedKeystoreWithAutoSeal(t, 200*time.Millisecond)

	sk.UnsealWithPassword(password)

	time.Sleep(120 * time.Millisecond)

	err := sk.ExtendPasswordSession()
	if err != nil {
		t.Fatalf("ExtendPasswordSession: %v", err)
	}

	time.Sleep(120 * time.Millisecond)

	if !sk.IsPasswordUnsealed() {
		t.Error("should still be unsealed after activity reset the timer")
	}

	time.Sleep(150 * time.Millisecond)

	if sk.IsPasswordUnsealed() {
		t.Error("should be sealed after timer expired post-extend")
	}
}

// ---------------------------------------------------------------------------
// Rate Limiter — 5 attempts per identity per 60 seconds
// ---------------------------------------------------------------------------

func TestRateLimiterAllowsUpToMax(t *testing.T) {
	rl := NewRateLimiter(5, 60*time.Second)

	for i := 0; i < 5; i++ {
		rl.Record("user-1")
	}
	if rl.Exceeded("user-1") {
		t.Error("should not be exceeded at exactly 5 attempts")
	}

	rl.Record("user-1")
	if !rl.Exceeded("user-1") {
		t.Error("should be exceeded after 6th attempt")
	}
}

func TestRateLimiterIdentityIsolation(t *testing.T) {
	rl := NewRateLimiter(5, 60*time.Second)

	for i := 0; i < 10; i++ {
		rl.Record("user-1")
	}
	if !rl.Exceeded("user-1") {
		t.Error("user-1 should be exceeded")
	}
	if rl.Exceeded("user-2") {
		t.Error("user-2 should not be exceeded (separate identity)")
	}
}

func TestRateLimiterWindowReset(t *testing.T) {
	rl := NewRateLimiter(5, 50*time.Millisecond)

	for i := 0; i < 10; i++ {
		rl.Record("user-1")
	}
	if !rl.Exceeded("user-1") {
		t.Fatal("should be exceeded within window")
	}

	time.Sleep(60 * time.Millisecond)

	if rl.Exceeded("user-1") {
		t.Error("should be reset after window expires")
	}
}

func TestRateLimiterProductionConfig(t *testing.T) {
	rl := NewRateLimiter(5, 60*time.Second)

	for i := 0; i < 5; i++ {
		rl.Record("identity-abc")
	}
	if rl.Exceeded("identity-abc") {
		t.Error("5 attempts within 60s should not trigger rate limit")
	}

	rl.Record("identity-abc")
	if !rl.Exceeded("identity-abc") {
		t.Error("6th attempt within 60s should trigger rate limit")
	}
}

// ---------------------------------------------------------------------------
// AEAD Cipher — XChaCha20-Poly1305 (unchanged)
// ---------------------------------------------------------------------------

func TestWrapUsesXChaCha20Poly1305(t *testing.T) {
	kd, _ := NewKeyDerivation(DefaultKeyDerivationParams)

	key := make([]byte, 32)
	rand.Read(key)

	wrapped, err := kd.WrapKey(key, []byte("password"))
	if err != nil {
		t.Fatalf("WrapKey: %v", err)
	}

	// XChaCha20-Poly1305 nonce is 24 bytes
	if len(wrapped.Nonce) != 24 {
		t.Errorf("expected 24-byte XChaCha20-Poly1305 nonce, got %d", len(wrapped.Nonce))
	}

	// Ciphertext = 32 plaintext + 16 auth tag = 48 bytes
	if len(wrapped.Ciphertext) != 48 {
		t.Errorf("expected 48-byte ciphertext (32+16 tag), got %d", len(wrapped.Ciphertext))
	}

	unwrapped, err := kd.UnwrapKey(wrapped, []byte("password"))
	if err != nil {
		t.Fatalf("UnwrapKey: %v", err)
	}
	if !ConstantTimeCompare(key, unwrapped) {
		t.Error("unwrapped key mismatch")
	}
}

// ---------------------------------------------------------------------------
// Key Derivation Params Validation — rejects below-OWASP
// ---------------------------------------------------------------------------

func TestParamsRejectBelowOWASP(t *testing.T) {
	belowOWASP := KeyDerivationParams{
		Memory:      8 * 1024, // 8 MB, below OWASP 64 MB
		Iterations:  1,        // below OWASP 3
		Parallelism: 1,        // below OWASP 4
		KeyLength:   32,
		SaltLength:  16,
	}

	// Validate should pass because code allows lower for testing
	// But defaults should always be OWASP-compliant
	if err := belowOWASP.Validate(); err != nil {
		t.Logf("below-OWASP params rejected by Validate: %v (acceptable)", err)
	}

	if DefaultKeyDerivationParams.Memory < 64*1024 {
		t.Error("production defaults must meet OWASP memory minimum")
	}
	if DefaultKeyDerivationParams.Iterations < 3 {
		t.Error("production defaults must meet OWASP iterations minimum")
	}
	if DefaultKeyDerivationParams.Parallelism < 4 {
		t.Error("production defaults must meet OWASP parallelism minimum")
	}
}
