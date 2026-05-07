package keystore

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNewSealedKeystore(t *testing.T) {
	baseKS := createTestKeystore(t)
	defer baseKS.Close()

	cfg := SealedKeystoreConfig{
		BaseKeystore: baseKS,
		DefaultTTL:   5 * time.Minute,
		Policy:       PolicyMobileApproval,
	}

	sk, err := NewSealedKeystore(cfg)
	if err != nil {
		t.Fatalf("failed to create sealed keystore: %v", err)
	}

	if sk.policy != PolicyMobileApproval {
		t.Errorf("expected policy %s, got %s", PolicyMobileApproval, sk.policy)
	}
}

func TestNewSealedKeystoreDefaults(t *testing.T) {
	baseKS := createTestKeystore(t)
	defer baseKS.Close()

	cfg := SealedKeystoreConfig{
		BaseKeystore: baseKS,
	}

	sk, err := NewSealedKeystore(cfg)
	if err != nil {
		t.Fatalf("failed to create sealed keystore: %v", err)
	}

	if sk.defaultTTL != 5*time.Minute {
		t.Errorf("expected default TTL 5m, got %v", sk.defaultTTL)
	}
	if sk.policy != PolicyMobileApproval {
		t.Errorf("expected default policy %s, got %s", PolicyMobileApproval, sk.policy)
	}
}

func TestIsSealed(t *testing.T) {
	baseKS := createTestKeystore(t)
	defer baseKS.Close()

	sk, _ := NewSealedKeystore(SealedKeystoreConfig{BaseKeystore: baseKS})

	// Initially sealed
	if !sk.IsSealed("agent-001") {
		t.Error("expected keystore to be sealed initially")
	}

	// Create a session directly
	sk.mu.Lock()
	sk.createSessionLocked("agent-001", PolicyAuto, "", "")
	sk.mu.Unlock()

	// Now unsealed
	if sk.IsSealed("agent-001") {
		t.Error("expected keystore to be unsealed after session creation")
	}

	// Different agent still sealed
	if !sk.IsSealed("agent-002") {
		t.Error("expected different agent to still be sealed")
	}
}

func TestGetStatus(t *testing.T) {
	baseKS := createTestKeystore(t)
	defer baseKS.Close()

	sk, _ := NewSealedKeystore(SealedKeystoreConfig{BaseKeystore: baseKS})

	// Sealed status
	status := sk.GetStatus("agent-001")
	if !status.IsSealed {
		t.Error("expected sealed status")
	}

	// Create session
	sk.mu.Lock()
	sk.createSessionLocked("agent-001", PolicyMobileApproval, "@user:example.com", "device-001")
	sk.mu.Unlock()

	// Unsealed status
	status = sk.GetStatus("agent-001")
	if status.IsSealed {
		t.Error("expected unsealed status")
	}
	if status.SessionID == "" {
		t.Error("expected session ID")
	}
	if status.AgentID != "agent-001" {
		t.Errorf("expected agent_id 'agent-001', got %s", status.AgentID)
	}
}

func TestRequestUnsealMobileApproval(t *testing.T) {
	baseKS := createTestKeystore(t)
	defer baseKS.Close()

	sk, _ := NewSealedKeystore(SealedKeystoreConfig{
		BaseKeystore: baseKS,
		Policy:       PolicyMobileApproval,
	})

	ctx := context.Background()
	req, err := sk.RequestUnseal(ctx, "agent-001", "test reason", []string{"name", "email"}, "task-001")
	if err != nil {
		t.Fatalf("failed to request unseal: %v", err)
	}

	if req.ID == "" {
		t.Error("expected request ID")
	}
	if req.AgentID != "agent-001" {
		t.Errorf("expected agent_id 'agent-001', got %s", req.AgentID)
	}
	if req.Reason != "test reason" {
		t.Errorf("expected reason 'test reason', got %s", req.Reason)
	}
	if len(req.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(req.Fields))
	}

	// Still sealed until approved
	if !sk.IsSealed("agent-001") {
		t.Error("expected keystore to still be sealed before approval")
	}
}

func TestRequestUnsealAutoPolicy(t *testing.T) {
	baseKS := createTestKeystore(t)
	defer baseKS.Close()

	sk, _ := NewSealedKeystore(SealedKeystoreConfig{
		BaseKeystore: baseKS,
		Policy:       PolicyAuto,
	})

	ctx := context.Background()
	req, err := sk.RequestUnseal(ctx, "agent-001", "test reason", nil, "")
	if err != nil {
		t.Fatalf("failed to request unseal: %v", err)
	}

	_ = req // Request is created

	// Auto policy should unseal immediately
	if sk.IsSealed("agent-001") {
		t.Error("expected keystore to be unsealed with auto policy")
	}
}

func TestApproveUnseal(t *testing.T) {
	baseKS := createTestKeystore(t)
	defer baseKS.Close()

	sk, _ := NewSealedKeystore(SealedKeystoreConfig{
		BaseKeystore: baseKS,
		Policy:       PolicyMobileApproval,
	})

	ctx := context.Background()
	req, _ := sk.RequestUnseal(ctx, "agent-001", "test", nil, "")

	// Approve the request
	session, err := sk.ApproveUnseal(ctx, req.ID, "@user:example.com", "device-001")
	if err != nil {
		t.Fatalf("failed to approve unseal: %v", err)
	}

	if session.AgentID != "agent-001" {
		t.Errorf("expected agent_id 'agent-001', got %s", session.AgentID)
	}
	if session.ApprovedBy != "@user:example.com" {
		t.Errorf("expected approved_by '@user:example.com', got %s", session.ApprovedBy)
	}

	// Now unsealed
	if sk.IsSealed("agent-001") {
		t.Error("expected keystore to be unsealed after approval")
	}
}

func TestApproveUnsealExpiredRequest(t *testing.T) {
	baseKS := createTestKeystore(t)
	defer baseKS.Close()

	sk, _ := NewSealedKeystore(SealedKeystoreConfig{
		BaseKeystore: baseKS,
		Policy:       PolicyMobileApproval,
	})

	ctx := context.Background()
	req, _ := sk.RequestUnseal(ctx, "agent-001", "test", nil, "")

	// Manually expire the request
	sk.mu.Lock()
	if pending, exists := sk.pending[req.ID]; exists {
		pending.ExpiresAt = time.Now().Add(-1 * time.Hour)
	}
	sk.mu.Unlock()

	// Try to approve expired request
	_, err := sk.ApproveUnseal(ctx, req.ID, "@user:example.com", "device-001")
	if err != ErrUnsealExpired {
		t.Errorf("expected ErrUnsealExpired, got %v", err)
	}
}

func TestRejectUnseal(t *testing.T) {
	baseKS := createTestKeystore(t)
	defer baseKS.Close()

	sk, _ := NewSealedKeystore(SealedKeystoreConfig{
		BaseKeystore: baseKS,
		Policy:       PolicyMobileApproval,
	})

	ctx := context.Background()
	req, _ := sk.RequestUnseal(ctx, "agent-001", "test", nil, "")

	// Reject the request
	err := sk.RejectUnseal(ctx, req.ID, "@user:example.com")
	if err != nil {
		t.Fatalf("failed to reject unseal: %v", err)
	}

	// Still sealed
	if !sk.IsSealed("agent-001") {
		t.Error("expected keystore to still be sealed after rejection")
	}

	// Request should be removed
	pending := sk.GetPendingRequests("agent-001")
	if len(pending) != 0 {
		t.Errorf("expected no pending requests, got %d", len(pending))
	}
}

func TestGetPendingRequests(t *testing.T) {
	baseKS := createTestKeystore(t)
	defer baseKS.Close()

	sk, _ := NewSealedKeystore(SealedKeystoreConfig{
		BaseKeystore: baseKS,
		Policy:       PolicyMobileApproval,
	})

	ctx := context.Background()
	sk.RequestUnseal(ctx, "agent-001", "reason 1", nil, "")
	sk.RequestUnseal(ctx, "agent-002", "reason 2", nil, "")

	// Get all pending
	all := sk.GetPendingRequests("")
	if len(all) != 2 {
		t.Errorf("expected 2 pending requests, got %d", len(all))
	}

	// Get for specific agent
	agent1 := sk.GetPendingRequests("agent-001")
	if len(agent1) != 1 {
		t.Errorf("expected 1 pending request for agent-001, got %d", len(agent1))
	}
}

func TestSeal(t *testing.T) {
	baseKS := createTestKeystore(t)
	defer baseKS.Close()

	sk, _ := NewSealedKeystore(SealedKeystoreConfig{
		BaseKeystore: baseKS,
		Policy:       PolicyAuto,
	})

	ctx := context.Background()
	sk.RequestUnseal(ctx, "agent-001", "test", nil, "")

	// Should be unsealed
	if sk.IsSealed("agent-001") {
		t.Fatal("expected keystore to be unsealed")
	}

	// Seal it
	err := sk.Seal(ctx, "agent-001")
	if err != nil {
		t.Fatalf("failed to seal: %v", err)
	}

	// Now sealed
	if !sk.IsSealed("agent-001") {
		t.Error("expected keystore to be sealed after Seal()")
	}
}

func TestSealAll(t *testing.T) {
	baseKS := createTestKeystore(t)
	defer baseKS.Close()

	sk, _ := NewSealedKeystore(SealedKeystoreConfig{
		BaseKeystore: baseKS,
		Policy:       PolicyAuto,
	})

	ctx := context.Background()
	sk.RequestUnseal(ctx, "agent-001", "test", nil, "")
	sk.RequestUnseal(ctx, "agent-002", "test", nil, "")

	// Seal all
	err := sk.SealAll(ctx)
	if err != nil {
		t.Fatalf("failed to seal all: %v", err)
	}

	// Both should be sealed
	if !sk.IsSealed("agent-001") || !sk.IsSealed("agent-002") {
		t.Error("expected both agents to be sealed")
	}
}

func TestExtendSession(t *testing.T) {
	baseKS := createTestKeystore(t)
	defer baseKS.Close()

	sk, _ := NewSealedKeystore(SealedKeystoreConfig{
		BaseKeystore: baseKS,
		Policy:       PolicyAuto,
		DefaultTTL:   1 * time.Minute,
	})

	ctx := context.Background()
	sk.RequestUnseal(ctx, "agent-001", "test", nil, "")

	// Get original expiry
	session, _ := sk.GetSession("agent-001")
	originalExpiry := session.ExpiresAt

	// Extend session
	err := sk.ExtendSession(ctx, "agent-001", 5*time.Minute)
	if err != nil {
		t.Fatalf("failed to extend session: %v", err)
	}

	// Check new expiry
	session, _ = sk.GetSession("agent-001")
	if !session.ExpiresAt.After(originalExpiry) {
		t.Error("expected expiry to be extended")
	}
}

func TestRetrieveProfileSealed(t *testing.T) {
	baseKS := createTestKeystore(t)
	defer baseKS.Close()

	sk, _ := NewSealedKeystore(SealedKeystoreConfig{
		BaseKeystore: baseKS,
		Policy:       PolicyMobileApproval,
	})

	ctx := context.Background()

	// Try to retrieve while sealed
	_, err := sk.RetrieveProfile(ctx, "agent-001", "profile-001")
	if err != ErrKeystoreSealed {
		t.Errorf("expected ErrKeystoreSealed, got %v", err)
	}
}

func TestRetrieveProfileUnsealed(t *testing.T) {
	baseKS := createTestKeystore(t)
	defer baseKS.Close()

	// Store a test profile
	profileData := []byte(`{"name":"John","email":"john@example.com"}`)
	err := baseKS.StoreProfile("profile-001", "Personal", "personal", profileData, `{"name":"text","email":"email"}`, true)
	if err != nil {
		t.Fatalf("failed to store profile: %v", err)
	}

	sk, _ := NewSealedKeystore(SealedKeystoreConfig{
		BaseKeystore: baseKS,
		Policy:       PolicyAuto,
	})

	ctx := context.Background()
	sk.RequestUnseal(ctx, "agent-001", "test", nil, "")

	// Now retrieve should work
	profile, err := sk.RetrieveProfile(ctx, "agent-001", "profile-001")
	if err != nil {
		t.Fatalf("failed to retrieve profile: %v", err)
	}

	if profile.ID != "profile-001" {
		t.Errorf("expected profile ID 'profile-001', got %s", profile.ID)
	}
}

func TestCleanupExpired(t *testing.T) {
	baseKS := createTestKeystore(t)
	defer baseKS.Close()

	sk, _ := NewSealedKeystore(SealedKeystoreConfig{
		BaseKeystore: baseKS,
		Policy:       PolicyAuto,
		DefaultTTL:   1 * time.Nanosecond, // Very short TTL
	})

	ctx := context.Background()
	sk.RequestUnseal(ctx, "agent-001", "test", nil, "")

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	// Cleanup
	count := sk.CleanupExpired()
	if count < 1 {
		t.Errorf("expected at least 1 expired session cleaned, got %d", count)
	}

	// Should be sealed now
	if !sk.IsSealed("agent-001") {
		t.Error("expected keystore to be sealed after cleanup")
	}
}

func TestSetPolicy(t *testing.T) {
	baseKS := createTestKeystore(t)
	defer baseKS.Close()

	sk, _ := NewSealedKeystore(SealedKeystoreConfig{
		BaseKeystore: baseKS,
		Policy:       PolicyMobileApproval,
	})

	if sk.GetPolicy() != PolicyMobileApproval {
		t.Error("expected initial policy to be mobile_approval")
	}

	sk.SetPolicy(PolicyAuto)

	if sk.GetPolicy() != PolicyAuto {
		t.Error("expected policy to be changed to auto")
	}
}

func TestToMatrixEvent(t *testing.T) {
	req := &PendingUnsealRequest{
		ID:          "req_123",
		AgentID:     "agent-001",
		Reason:      "test",
		Fields:      []string{"name", "email"},
		TaskID:      "task-001",
		RequestedAt: time.Now(),
		ExpiresAt:   time.Now().Add(60 * time.Second),
	}

	event := req.ToMatrixEvent()
	if event["type"] != "com.armorclaw.sealed_keystore.unseal_request" {
		t.Errorf("unexpected event type: %v", event["type"])
	}
	if event["request_id"] != "req_123" {
		t.Errorf("unexpected request_id: %v", event["request_id"])
	}

	session := &SealedSession{
		ID:           "sess_456",
		AgentID:      "agent-001",
		UnsealedAt:   time.Now(),
		ExpiresAt:    time.Now().Add(5 * time.Minute),
		UnsealPolicy: PolicyMobileApproval,
		ApprovedBy:   "@user:example.com",
	}

	sessionEvent := session.ToMatrixEvent()
	if sessionEvent["type"] != "com.armorclaw.sealed_keystore.session" {
		t.Errorf("unexpected event type: %v", sessionEvent["type"])
	}
	if sessionEvent["session_id"] != "sess_456" {
		t.Errorf("unexpected session_id: %v", sessionEvent["session_id"])
	}
}

// Helper function to create a test keystore
func createTestKeystore(t *testing.T) *Keystore {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "keystore-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	dbPath := filepath.Join(tmpDir, "keystore.db")

	ks, err := New(Config{DBPath: dbPath})
	if err != nil {
		t.Fatalf("failed to create keystore: %v", err)
	}

	if err := ks.Open(); err != nil {
		t.Fatalf("failed to open keystore: %v", err)
	}

	return ks
}

func newTestPasswordSealedKeystore(t *testing.T) (*SealedKeystore, string) {
	return newTestPasswordSealedKeystoreWithAutoSeal(t, 0)
}

func newTestPasswordSealedKeystoreWithAutoSeal(t *testing.T, autoSeal time.Duration) (*SealedKeystore, string) {
	t.Helper()

	baseKS := createTestKeystore(t)
	t.Cleanup(func() { baseKS.Close() })

	password := "test-password"
	kd, err := NewKeyDerivation(DefaultKeyDerivationParams)
	if err != nil {
		t.Fatalf("failed to create key derivation: %v", err)
	}

	salt := make([]byte, DefaultKeyDerivationParams.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		t.Fatalf("failed to generate salt: %v", err)
	}

	verifier, err := kd.DeriveKey([]byte(password), salt)
	if err != nil {
		t.Fatalf("failed to derive verifier: %v", err)
	}

	vaultKey := make([]byte, 32)
	if _, err := rand.Read(vaultKey); err != nil {
		t.Fatalf("failed to generate vault key: %v", err)
	}

	wrapped, err := kd.WrapKey(vaultKey, []byte(password))
	if err != nil {
		t.Fatalf("failed to wrap vault key: %v", err)
	}

	cfg := SealedStoreConfig{
		PasswordVerifier: verifier.Key,
		VerifySalt:       salt,
		WrappedVaultKey:  *wrapped,
	}

	sk, err := NewSealedKeystoreWithPassword(baseKS, cfg)
	if err != nil {
		t.Fatalf("failed to create password sealed keystore: %v", err)
	}

	if autoSeal > 0 {
		sk.SetAutoSealDuration(autoSeal)
	}

	return sk, password
}

func TestPasswordUnsealCorrect(t *testing.T) {
	sk, password := newTestPasswordSealedKeystore(t)

	if sk.IsPasswordUnsealed() {
		t.Error("expected keystore to start sealed")
	}

	err := sk.UnsealWithPassword(password)
	if err != nil {
		t.Fatalf("expected successful unseal, got: %v", err)
	}

	if !sk.IsPasswordUnsealed() {
		t.Error("expected keystore to be unsealed after correct password")
	}

	vk := sk.VaultKey()
	if vk == nil {
		t.Error("expected vault key to be available after unseal")
	}
	if len(vk) != 32 {
		t.Errorf("expected vault key length 32, got %d", len(vk))
	}
}

func TestPasswordUnsealWrong(t *testing.T) {
	sk, _ := newTestPasswordSealedKeystore(t)

	err := sk.UnsealWithPassword("wrong-password")
	if err == nil {
		t.Fatal("expected error with wrong password")
	}

	if err != ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword, got: %v", err)
	}

	if sk.IsPasswordUnsealed() {
		t.Error("expected keystore to stay sealed after wrong password")
	}

	if sk.VaultKey() != nil {
		t.Error("expected nil vault key after failed unseal")
	}
}

func TestPasswordUnsealAlreadyUnsealed(t *testing.T) {
	sk, password := newTestPasswordSealedKeystore(t)

	err := sk.UnsealWithPassword(password)
	if err != nil {
		t.Fatalf("first unseal failed: %v", err)
	}

	err = sk.UnsealWithPassword(password)
	if err == nil {
		t.Fatal("expected error on second unseal")
	}

	if err != ErrAlreadyUnsealed {
		t.Errorf("expected ErrAlreadyUnsealed, got: %v", err)
	}
}

func TestPasswordSealZerosVaultKey(t *testing.T) {
	sk, password := newTestPasswordSealedKeystore(t)

	err := sk.UnsealWithPassword(password)
	if err != nil {
		t.Fatalf("unseal failed: %v", err)
	}

	if sk.VaultKey() == nil {
		t.Fatal("expected vault key after unseal")
	}

	sk.SealPassword()

	if sk.IsPasswordUnsealed() {
		t.Error("expected keystore to be sealed after SealPassword()")
	}

	if sk.VaultKey() != nil {
		t.Error("expected nil vault key after seal")
	}
}

func TestListKeysReturnsNames(t *testing.T) {
	sk, password := newTestPasswordSealedKeystore(t)

	err := sk.UnsealWithPassword(password)
	if err != nil {
		t.Fatalf("unseal failed: %v", err)
	}

	base := sk.GetBaseKeystore()
	now := time.Now().Unix()
	base.Store(Credential{ID: "key-alpha", Provider: ProviderOpenAI, Token: "tok-a", DisplayName: "Alpha", CreatedAt: now})
	base.Store(Credential{ID: "key-beta", Provider: ProviderAnthropic, Token: "tok-b", DisplayName: "Beta", CreatedAt: now})

	ctx := context.Background()
	names, err := sk.ListKeys(ctx)
	if err != nil {
		t.Fatalf("ListKeys failed: %v", err)
	}

	if len(names) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(names))
	}

	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	if !found["key-alpha"] || !found["key-beta"] {
		t.Errorf("expected key-alpha and key-beta, got %v", names)
	}
}

func TestListKeysSealed(t *testing.T) {
	sk, _ := newTestPasswordSealedKeystore(t)

	ctx := context.Background()
	_, err := sk.ListKeys(ctx)
	if err != ErrPasswordSealed {
		t.Errorf("expected ErrPasswordSealed, got %v", err)
	}
}

func TestDeleteKeyRemovesKey(t *testing.T) {
	sk, password := newTestPasswordSealedKeystore(t)

	err := sk.UnsealWithPassword(password)
	if err != nil {
		t.Fatalf("unseal failed: %v", err)
	}

	base := sk.GetBaseKeystore()
	now := time.Now().Unix()
	base.Store(Credential{ID: "key-gone", Provider: ProviderOpenAI, Token: "tok-x", DisplayName: "Gone", CreatedAt: now})

	ctx := context.Background()
	err = sk.DeleteKey(ctx, "key-gone")
	if err != nil {
		t.Fatalf("DeleteKey failed: %v", err)
	}

	names, err := sk.ListKeys(ctx)
	if err != nil {
		t.Fatalf("ListKeys after delete failed: %v", err)
	}
	for _, n := range names {
		if n == "key-gone" {
			t.Error("key-gone should have been deleted")
		}
	}
}

func TestDeleteKeySealed(t *testing.T) {
	sk, _ := newTestPasswordSealedKeystore(t)

	ctx := context.Background()
	err := sk.DeleteKey(ctx, "any-key")
	if err != ErrPasswordSealed {
		t.Errorf("expected ErrPasswordSealed, got %v", err)
	}
}

func TestPasswordConcurrent(t *testing.T) {
	sk, password := newTestPasswordSealedKeystore(t)

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// Concurrent unseal attempts
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Add(-1)
			_ = sk.UnsealWithPassword(password)
		}()
	}

	// Concurrent seal attempts
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Add(-1)
			sk.SealPassword()
		}()
	}

	// Concurrent reads
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Add(-1)
			_ = sk.VaultKey()
		}()
	}

	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Errorf("unexpected error in concurrent test: %v", err)
		}
	}
}

func TestAutoSealTimer(t *testing.T) {
	sk, password := newTestPasswordSealedKeystoreWithAutoSeal(t, 100*time.Millisecond)

	err := sk.UnsealWithPassword(password)
	if err != nil {
		t.Fatalf("unseal failed: %v", err)
	}

	if !sk.IsPasswordUnsealed() {
		t.Fatal("expected keystore to be unsealed immediately after unseal")
	}

	time.Sleep(200 * time.Millisecond)

	if sk.IsPasswordUnsealed() {
		t.Error("expected keystore to be auto-sealed after 100ms inactivity")
	}

	if sk.VaultKey() != nil {
		t.Error("expected vault key to be nil after auto-seal")
	}
}

func TestTimerActivityReset(t *testing.T) {
	sk, password := newTestPasswordSealedKeystoreWithAutoSeal(t, 200*time.Millisecond)

	err := sk.UnsealWithPassword(password)
	if err != nil {
		t.Fatalf("unseal failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	err = sk.ExtendPasswordSession()
	if err != nil {
		t.Fatalf("ExtendPasswordSession failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if !sk.IsPasswordUnsealed() {
		t.Error("expected keystore to still be unsealed after ExtendPasswordSession reset the timer")
	}

	time.Sleep(150 * time.Millisecond)

	if sk.IsPasswordUnsealed() {
		t.Error("expected keystore to be sealed after timer expired post-extend")
	}
}

func TestSessionStatus(t *testing.T) {
	sk, password := newTestPasswordSealedKeystoreWithAutoSeal(t, 5*time.Second)

	status := sk.SessionStatus()
	if !status.Sealed {
		t.Error("expected sealed status before unseal")
	}

	err := sk.UnsealWithPassword(password)
	if err != nil {
		t.Fatalf("unseal failed: %v", err)
	}

	status = sk.SessionStatus()
	if status.Sealed {
		t.Error("expected unsealed status after unseal")
	}
	if status.RemainingSeconds <= 0 {
		t.Errorf("expected positive remaining seconds, got %f", status.RemainingSeconds)
	}
	if status.RemainingSeconds > 5.0 {
		t.Errorf("expected remaining <= 5s, got %f", status.RemainingSeconds)
	}
	if time.Since(status.LastActivityAt) > time.Second {
		t.Error("expected LastActivityAt to be recent")
	}

	sk.SealPassword()

	status = sk.SessionStatus()
	if !status.Sealed {
		t.Error("expected sealed status after SealPassword")
	}
	if status.RemainingSeconds != 0 {
		t.Errorf("expected 0 remaining seconds when sealed, got %f", status.RemainingSeconds)
	}
}

func TestTimerResetOnExtend(t *testing.T) {
	sk, password := newTestPasswordSealedKeystoreWithAutoSeal(t, 300*time.Millisecond)

	err := sk.UnsealWithPassword(password)
	if err != nil {
		t.Fatalf("unseal failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	err = sk.ExtendPasswordSession()
	if err != nil {
		t.Fatalf("ExtendPasswordSession failed: %v", err)
	}

	status := sk.SessionStatus()
	if status.Sealed {
		t.Error("expected unsealed after extend")
	}
	if status.RemainingSeconds < 0.2 {
		t.Errorf("expected timer to be reset, remaining should be near 0.3s, got %f", status.RemainingSeconds)
	}

	time.Sleep(400 * time.Millisecond)

	if sk.IsPasswordUnsealed() {
		t.Error("expected auto-seal after extended timer expired")
	}
}

func TestExtendPasswordSessionWhenSealed(t *testing.T) {
	sk, _ := newTestPasswordSealedKeystore(t)

	err := sk.ExtendPasswordSession()
	if err == nil {
		t.Fatal("expected error when extending sealed keystore")
	}
	if err != ErrPasswordSealed {
		t.Errorf("expected ErrPasswordSealed, got: %v", err)
	}
}

func TestSealedGetBlocked(t *testing.T) {
	sk, _ := newTestPasswordSealedKeystore(t)
	sk.SetFeatureCheck(func() bool { return true })

	base := sk.GetBaseKeystore()
	now := time.Now().Unix()
	base.Store(Credential{ID: "test-key", Provider: ProviderOpenAI, Token: "tok-123", DisplayName: "Test", CreatedAt: now})

	ctx := context.Background()
	sk.mu.Lock()
	sk.createSessionLocked("agent-001", PolicyPassword, "", "")
	sk.mu.Unlock()

	_, err := sk.Retrieve(ctx, "agent-001", "test-key")
	if err == nil {
		t.Fatal("expected error when retrieving from sealed keystore")
	}
	if err != ErrPasswordSealed {
		t.Errorf("expected ErrPasswordSealed, got: %v", err)
	}
}

func TestSealedGetAllowed(t *testing.T) {
	sk, password := newTestPasswordSealedKeystore(t)
	sk.SetFeatureCheck(func() bool { return true })

	err := sk.UnsealWithPassword(password)
	if err != nil {
		t.Fatalf("unseal failed: %v", err)
	}

	base := sk.GetBaseKeystore()
	now := time.Now().Unix()
	base.Store(Credential{ID: "test-key", Provider: ProviderOpenAI, Token: "tok-123", DisplayName: "Test", CreatedAt: now})

	ctx := context.Background()
	sk.mu.Lock()
	sk.createSessionLocked("agent-001", PolicyPassword, "", "")
	sk.mu.Unlock()

	cred, err := sk.Retrieve(ctx, "agent-001", "test-key")
	if err != nil {
		t.Fatalf("expected successful retrieve, got: %v", err)
	}
	if cred.ID != "test-key" {
		t.Errorf("expected credential ID 'test-key', got %s", cred.ID)
	}
}

func TestSealedSetBlocked(t *testing.T) {
	sk, _ := newTestPasswordSealedKeystore(t)
	sk.SetFeatureCheck(func() bool { return true })

	ctx := context.Background()
	sk.mu.Lock()
	sk.createSessionLocked("agent-001", PolicyPassword, "", "")
	sk.mu.Unlock()

	err := sk.Store(ctx, "agent-001", Credential{
		ID:        "new-key",
		Provider:  ProviderOpenAI,
		Token:     "tok-new",
		CreatedAt: time.Now().Unix(),
	})
	if err == nil {
		t.Fatal("expected error when storing to sealed keystore")
	}
	if err != ErrPasswordSealed {
		t.Errorf("expected ErrPasswordSealed, got: %v", err)
	}
}

func TestSealedSetAllowed(t *testing.T) {
	sk, password := newTestPasswordSealedKeystore(t)
	sk.SetFeatureCheck(func() bool { return true })

	err := sk.UnsealWithPassword(password)
	if err != nil {
		t.Fatalf("unseal failed: %v", err)
	}

	ctx := context.Background()
	sk.mu.Lock()
	sk.createSessionLocked("agent-001", PolicyPassword, "", "")
	sk.mu.Unlock()

	err = sk.Store(ctx, "agent-001", Credential{
		ID:        "new-key",
		Provider:  ProviderOpenAI,
		Token:     "tok-new",
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("expected successful store, got: %v", err)
	}
}

func TestRetrieveNoFlagUnchanged(t *testing.T) {
	sk, _ := newTestPasswordSealedKeystore(t)
	// No SetFeatureCheck — feature flag off

	base := sk.GetBaseKeystore()
	now := time.Now().Unix()
	base.Store(Credential{ID: "test-key", Provider: ProviderOpenAI, Token: "tok-123", DisplayName: "Test", CreatedAt: now})

	ctx := context.Background()
	sk.mu.Lock()
	sk.createSessionLocked("agent-001", PolicyPassword, "", "")
	sk.mu.Unlock()

	// Should succeed even though password vault is sealed — flag is off
	cred, err := sk.Retrieve(ctx, "agent-001", "test-key")
	if err != nil {
		t.Fatalf("expected success with flag off, got: %v", err)
	}
	if cred.ID != "test-key" {
		t.Errorf("expected credential ID 'test-key', got %s", cred.ID)
	}
}

func TestSealedCheckFunc(t *testing.T) {
	sk, password := newTestPasswordSealedKeystore(t)
	sk.SetFeatureCheck(func() bool { return true })

	check := sk.SealedCheckFunc()

	// Sealed — should error
	if err := check(); err != ErrPasswordSealed {
		t.Errorf("expected ErrPasswordSealed, got: %v", err)
	}

	// Unseal
	if err := sk.UnsealWithPassword(password); err != nil {
		t.Fatalf("unseal failed: %v", err)
	}

	// Unsealed — should return nil
	if err := check(); err != nil {
		t.Errorf("expected nil after unseal, got: %v", err)
	}
}

func TestResetTimerFunc(t *testing.T) {
	sk, password := newTestPasswordSealedKeystoreWithAutoSeal(t, 200*time.Millisecond)
	sk.SetFeatureCheck(func() bool { return true })

	err := sk.UnsealWithPassword(password)
	if err != nil {
		t.Fatalf("unseal failed: %v", err)
	}

	reset := sk.ResetTimerFunc()

	time.Sleep(100 * time.Millisecond)
	reset()
	time.Sleep(100 * time.Millisecond)

	if !sk.IsPasswordUnsealed() {
		t.Error("expected keystore to still be unsealed after timer reset")
	}

	time.Sleep(250 * time.Millisecond)

	if sk.IsPasswordUnsealed() {
		t.Error("expected keystore to be sealed after timer expired post-reset")
	}
}
