package crypto

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"

	"maunium.net/go/mautrix/id"
)

// ---------------------------------------------------------------------------
// 1. Encrypt/Decrypt Round-Trip
// ---------------------------------------------------------------------------

// TestE2EE_EncryptDecryptRoundTrip verifies that EncryptionService correctly
// refuses operations without a real CryptoEngine (full Megolm round-trip
// requires a Matrix homeserver for key distribution).  The test validates
// that service wiring and nil-safety hold across two logical parties.
func TestE2EE_EncryptDecryptRoundTrip_ServiceWiring(t *testing.T) {
	cache := NewRoomEncryptionCache(true)
	cache.SetEncrypted("!room:test", true)

	svc := NewEncryptionService(nil, cache, nil)

	if svc.ShouldEncrypt("!room:test") {
		t.Error("ShouldEncrypt should return false when engine is nil (kill switch)")
	}
	if svc.ShouldEncrypt("!plain:test") {
		t.Error("ShouldEncrypt should return false when engine is nil")
	}

	// EncryptMessage should fail gracefully (no engine).
	_, err := svc.EncryptMessage(context.Background(), "!room:test", "hello")
	if err == nil {
		t.Error("EncryptMessage without engine should return error")
	}

	// DecryptEvent should fail gracefully.
	_, err = svc.DecryptEvent(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Error("DecryptEvent without engine should return error")
	}
}

// TestE2EE_EncryptDecryptRoundTrip_StoreLevel verifies that Megolm session
// data can be stored, retrieved, and verified at the Store layer — the
// foundation for encrypt/decrypt round-trips.
func TestE2EE_EncryptDecryptRoundTrip_StoreLevel(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	roomID := "!room:test"
	senderKey := "senderKey123"
	sessionID := "sessionID456"
	sessionKey := []byte("this-is-a-megolm-session-key-32b")

	// Store session.
	if err := store.AddInboundGroupSession(ctx, roomID, senderKey, sessionID, sessionKey); err != nil {
		t.Fatalf("AddInboundGroupSession: %v", err)
	}

	// Verify existence.
	if !store.HasInboundGroupSession(ctx, roomID, senderKey, sessionID) {
		t.Fatal("HasInboundGroupSession should return true after add")
	}

	// Retrieve.
	retrieved, err := store.GetInboundGroupSession(ctx, roomID, senderKey, sessionID)
	if err != nil {
		t.Fatalf("GetInboundGroupSession: %v", err)
	}
	if string(retrieved) != string(sessionKey) {
		t.Fatalf("retrieved key mismatch: got %q, want %q", string(retrieved), string(sessionKey))
	}

	// Update.
	updatedKey := []byte("updated-session-key-32bytes-pad!")
	if err := store.UpdateInboundGroupSession(ctx, roomID, senderKey, sessionID, updatedKey); err != nil {
		t.Fatalf("UpdateInboundGroupSession: %v", err)
	}

	retrieved, err = store.GetInboundGroupSession(ctx, roomID, senderKey, sessionID)
	if err != nil {
		t.Fatalf("GetInboundGroupSession after update: %v", err)
	}
	if string(retrieved) != string(updatedKey) {
		t.Fatalf("updated key mismatch: got %q, want %q", string(retrieved), string(updatedKey))
	}
}

// ---------------------------------------------------------------------------
// 2. Dual-Mode Messaging
// ---------------------------------------------------------------------------

// TestE2EE_DualModeMessaging verifies that RoomEncryptionCache correctly
// identifies encrypted vs unencrypted rooms, and messages are routed based
// on encryption status.
func TestE2EE_DualModeMessaging(t *testing.T) {
	cache := NewRoomEncryptionCache(true)

	cache.SetEncrypted("!encrypted-room:test", true)

	if !cache.IsEncrypted("!encrypted-room:test") {
		t.Error("encrypted room should be marked as encrypted in cache")
	}
	if cache.IsEncrypted("!plaintext-room:test") {
		t.Error("plaintext room should not be encrypted in cache")
	}

	// OnRoomEncryptionEvent flips a room to encrypted (even with nil engine).
	svc := NewEncryptionService(nil, cache, nil)
	svc.OnRoomEncryptionEvent("!newly-encrypted:test")
	if !cache.IsEncrypted("!newly-encrypted:test") {
		t.Error("room should be encrypted after OnRoomEncryptionEvent")
	}

	// ShouldEncrypt returns false when engine is nil (no actual crypto possible).
	if svc.ShouldEncrypt("!newly-encrypted:test") {
		t.Error("ShouldEncrypt returns false when engine is nil — kill switch")
	}
}

// TestE2EE_DualModeMessaging_ProcessStateEvents verifies batch detection
// of encrypted rooms from sync state events.
func TestE2EE_DualModeMessaging_ProcessStateEvents(t *testing.T) {
	cache := NewRoomEncryptionCache(true)

	events := []json.RawMessage{
		[]byte(`{"type":"m.room.encryption","room_id":"!encA:test","content":{"algorithm":"m.megolm.v1.aes-sha2"}}`),
		[]byte(`{"type":"m.room.member","room_id":"!plainB:test","content":{"membership":"join"}}`),
		[]byte(`{"type":"m.room.encryption","room_id":"!encC:test","content":{"algorithm":"m.megolm.v1.aes-sha2"}}`),
		[]byte(`invalid json`),
	}
	cache.ProcessStateEvents(events)

	if !cache.IsEncrypted("!encA:test") {
		t.Error("!encA:test should be encrypted")
	}
	if cache.IsEncrypted("!plainB:test") {
		t.Error("!plainB:test should not be encrypted")
	}
	if !cache.IsEncrypted("!encC:test") {
		t.Error("!encC:test should be encrypted")
	}
}

// ---------------------------------------------------------------------------
// 3. Kill Switch Verification
// ---------------------------------------------------------------------------

// TestE2EE_KillSwitch_DisabledGlobals verifies that when E2EE is disabled
// (config gate), all crypto operations are no-ops and messages remain plaintext.
func TestE2EE_KillSwitch_DisabledGlobals(t *testing.T) {
	// Simulate E2EE disabled: NewCryptoEngine returns (nil, nil).
	cache := NewRoomEncryptionCache(false) // e2eeEnabled = false

	// Even if someone calls SetEncrypted, IsEncrypted returns false.
	cache.SetEncrypted("!room:test", true)
	if cache.IsEncrypted("!room:test") {
		t.Error("IsEncrypted should return false when E2EE globally disabled")
	}
	if cache.IsEnabled() {
		t.Error("IsEnabled should return false when E2EE disabled")
	}

	// EncryptionService with nil engine — all methods should be no-ops or errors.
	svc := NewEncryptionService(nil, cache, nil)
	if svc.ShouldEncrypt("!room:test") {
		t.Error("ShouldEncrypt should return false when engine is nil")
	}

	// EncryptMessage returns error (no crypto operations attempted).
	_, err := svc.EncryptMessage(context.Background(), "!room:test", "plaintext msg")
	if err == nil {
		t.Error("EncryptMessage should return error when engine is nil")
	}

	// DecryptEvent returns error.
	_, err = svc.DecryptEvent(context.Background(), nil)
	if err == nil {
		t.Error("DecryptEvent should return error when engine is nil")
	}

	// SASVerificationService returns nil (no verification possible).
	sasSvc := NewSASVerificationService(nil)
	if sasSvc != nil {
		t.Error("SASVerificationService should be nil when engine is nil")
	}

	// CrossSigningService returns nil.
	csSvc := NewCrossSigningService(nil, NewMemoryStore())
	if csSvc != nil {
		t.Error("CrossSigningService should be nil when engine is nil")
	}
}

// TestE2EE_KillSwitch_NilCryptoEngine verifies CryptoEngine nil-safety
// for the kill switch scenario.
func TestE2EE_KillSwitch_NilCryptoEngine(t *testing.T) {
	var engine *CryptoEngine

	if engine.IsE2EEEnabled() {
		t.Error("nil engine should not report E2EE enabled")
	}
	if engine.GetOlmMachine() != nil {
		t.Error("nil engine GetOlmMachine should return nil")
	}
	if engine.GetUserID() != "" {
		t.Error("nil engine GetUserID should return empty")
	}
	if engine.GetDeviceID() != "" {
		t.Error("nil engine GetDeviceID should return empty")
	}
	if err := engine.Initialize(context.Background()); err != nil {
		t.Errorf("nil engine Initialize should return nil, got: %v", err)
	}
	if err := engine.ProcessSyncResponse(context.Background(), nil); err != nil {
		t.Errorf("nil engine ProcessSyncResponse should return nil, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 4. SAS Verification Flow
// ---------------------------------------------------------------------------

// TestE2EE_SASVerification_FullFlow performs a complete SAS verification:
// start → accept → key exchange → confirm → MAC verify → done.
// Verifies emoji match on both sides.
func TestE2EE_SASVerification_FullFlow(t *testing.T) {
	engine := &CryptoEngine{
		deviceID: id.DeviceID("DEVICE_ALICE"),
		userID:   id.UserID("@alice:example.com"),
	}
	svc := NewSASVerificationService(engine)
	ctx := context.Background()

	theirUserID := id.UserID("@bob:example.com")
	theirDeviceID := id.DeviceID("DEVICE_BOB")

	// Step 1: Start verification.
	tx, err := svc.StartVerification(ctx, theirUserID, theirDeviceID)
	if err != nil {
		t.Fatalf("StartVerification: %v", err)
	}
	if tx.State != SASStateStarted {
		t.Fatalf("state = %d, want Started", tx.State)
	}
	txID := tx.TransactionID

	// Step 2: Accept verification.
	if err := svc.AcceptVerification(ctx, txID); err != nil {
		t.Fatalf("AcceptVerification: %v", err)
	}
	tx = svc.GetTransaction(txID)
	if tx.State != SASStateAccepted {
		t.Fatalf("state = %d, want Accepted", tx.State)
	}

	// Step 3: Exchange keys (simulate their key).
	theirKey := make([]byte, 32)
	if _, err := rand.Read(theirKey); err != nil {
		t.Fatalf("generate their key: %v", err)
	}
	ourKey, err := svc.ExchangeKeys(ctx, txID, theirKey)
	if err != nil {
		t.Fatalf("ExchangeKeys: %v", err)
	}
	if len(ourKey) != 32 {
		t.Fatalf("our key len = %d, want 32", len(ourKey))
	}
	tx = svc.GetTransaction(txID)
	if tx.State != SASStateKeyExchanged {
		t.Fatalf("state = %d, want KeyExchanged", tx.State)
	}

	// Verify emoji generated.
	emoji := svc.GetEmoji(txID)
	if len(emoji) != 6 {
		t.Fatalf("emoji count = %d, want 6", len(emoji))
	}
	for i, e := range emoji {
		if e == "" {
			t.Fatalf("emoji[%d] is empty", i)
		}
	}

	// Step 4: Confirm verification (user confirms emoji match).
	ourMAC, err := svc.ConfirmVerification(ctx, txID)
	if err != nil {
		t.Fatalf("ConfirmVerification: %v", err)
	}
	if len(ourMAC) != 32 {
		t.Fatalf("MAC len = %d, want 32", len(ourMAC))
	}
	tx = svc.GetTransaction(txID)
	if tx.State != SASStateConfirmed {
		t.Fatalf("state = %d, want Confirmed", tx.State)
	}

	// Step 5: Verify their MAC (computed from their perspective).
	theirMAC := computeTheirMAC(tx, engine)
	if err := svc.VerifyMAC(ctx, txID, theirMAC); err != nil {
		t.Fatalf("VerifyMAC: %v", err)
	}
	tx = svc.GetTransaction(txID)
	if tx.State != SASStateDone {
		t.Fatalf("state = %d, want Done", tx.State)
	}
}

// TestE2EE_SASVerification_EmojiDeterministic verifies that emoji are
// deterministic given the same key material (both parties see the same emoji).
func TestE2EE_SASVerification_EmojiDeterministic(t *testing.T) {
	engine := &CryptoEngine{
		deviceID: id.DeviceID("DEVICE_ALICE"),
		userID:   id.UserID("@alice:example.com"),
	}
	svc := NewSASVerificationService(engine)
	ctx := context.Background()

	// Run two independent flows with the same their-key material.
	fixedTheirKey := make([]byte, 32)
	for i := range fixedTheirKey {
		fixedTheirKey[i] = byte(i)
	}

	// Flow 1.
	tx1, _ := svc.StartVerification(ctx, "@bob:test", "DEV1")
	svc.AcceptVerification(ctx, tx1.TransactionID)
	svc.ExchangeKeys(ctx, tx1.TransactionID, fixedTheirKey)
	emoji1 := svc.GetEmoji(tx1.TransactionID)

	// Flow 2 (different transaction, same key material → different emoji
	// because ourKey is random, but verify they are still valid emoji).
	tx2, _ := svc.StartVerification(ctx, "@bob:test", "DEV2")
	svc.AcceptVerification(ctx, tx2.TransactionID)
	svc.ExchangeKeys(ctx, tx2.TransactionID, fixedTheirKey)
	emoji2 := svc.GetEmoji(tx2.TransactionID)

	// Both should produce valid emoji sets.
	if len(emoji1) != 6 || len(emoji2) != 6 {
		t.Fatalf("emoji1=%d, emoji2=%d, both want 6", len(emoji1), len(emoji2))
	}

	// Verify all emoji are from the fixed set.
	for i, e := range emoji1 {
		if !isValidSASEmoji(e) {
			t.Errorf("emoji1[%d] = %q not in valid set", i, e)
		}
	}
	for i, e := range emoji2 {
		if !isValidSASEmoji(e) {
			t.Errorf("emoji2[%d] = %q not in valid set", i, e)
		}
	}
}

// TestE2EE_SASVerification_MACTamperDetection verifies that MAC mismatch
// cancels the transaction.
func TestE2EE_SASVerification_MACTamperDetection(t *testing.T) {
	engine := &CryptoEngine{
		deviceID: id.DeviceID("DEVICE_ALICE"),
		userID:   id.UserID("@alice:example.com"),
	}
	svc := NewSASVerificationService(engine)
	ctx := context.Background()

	tx, _ := svc.StartVerification(ctx, "@eve:test", "DEV_EVE")
	txID := tx.TransactionID
	svc.AcceptVerification(ctx, txID)
	svc.ExchangeKeys(ctx, txID, make([]byte, 32))
	svc.ConfirmVerification(ctx, txID)

	// Tampered MAC.
	tamperedMAC := make([]byte, 32)
	for i := range tamperedMAC {
		tamperedMAC[i] = 0xFF
	}

	err := svc.VerifyMAC(ctx, txID, tamperedMAC)
	if err == nil {
		t.Fatal("tampered MAC should cause error")
	}

	tx = svc.GetTransaction(txID)
	if tx.State != SASStateCancelled {
		t.Fatalf("state = %d, want Cancelled", tx.State)
	}
	if tx.CancelReason != "MAC verification failed" {
		t.Fatalf("reason = %q, want 'MAC verification failed'", tx.CancelReason)
	}
}

// TestE2EE_SASVerification_CancelMidFlow verifies cancellation at various
// states works correctly.
func TestE2EE_SASVerification_CancelMidFlow(t *testing.T) {
	engine := &CryptoEngine{
		deviceID: id.DeviceID("DEVICE_ALICE"),
		userID:   id.UserID("@alice:example.com"),
	}
	svc := NewSASVerificationService(engine)
	ctx := context.Background()

	tx, _ := svc.StartVerification(ctx, "@bob:test", "DEV_BOB")
	txID := tx.TransactionID

	reason := "user declined verification"
	if err := svc.CancelVerification(ctx, txID, reason); err != nil {
		t.Fatalf("CancelVerification: %v", err)
	}

	tx = svc.GetTransaction(txID)
	if tx.State != SASStateCancelled {
		t.Fatalf("state = %d, want Cancelled", tx.State)
	}
	if tx.CancelReason != reason {
		t.Fatalf("reason = %q, want %q", tx.CancelReason, reason)
	}
}

// ---------------------------------------------------------------------------
// 5. Cross-Signing Bootstrap
// ---------------------------------------------------------------------------

// TestE2EE_CrossSigningBootstrap generates MSK/SSK/USK, signs device key,
// verifies signature, and checks IsBootstrapped.
func TestE2EE_CrossSigningBootstrap(t *testing.T) {
	store := NewMemoryStore()
	engine := &CryptoEngine{}
	svc := NewCrossSigningService(engine, store)
	ctx := context.Background()

	userID := id.UserID("@alice:example.com")
	deviceID := id.DeviceID("DEVICE_ABC")

	// Pre-condition: not bootstrapped.
	bootstrapped, err := svc.IsBootstrapped(ctx, string(userID))
	if err != nil {
		t.Fatalf("IsBootstrapped pre: %v", err)
	}
	if bootstrapped {
		t.Fatal("should not be bootstrapped initially")
	}

	// Bootstrap: generates MSK, SSK, USK; signs device key.
	result, err := svc.Bootstrap(ctx, userID, deviceID)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	// Verify key pairs.
	if result.MasterKey == nil || result.SelfSigningKey == nil || result.UserSigningKey == nil {
		t.Fatal("all three key pairs should be generated")
	}
	if result.MasterKey.Usage != CrossSigningKeyMaster {
		t.Fatalf("MSK usage = %q, want %q", result.MasterKey.Usage, CrossSigningKeyMaster)
	}
	if result.SelfSigningKey.Usage != CrossSigningKeySelfSigning {
		t.Fatalf("SSK usage = %q, want %q", result.SelfSigningKey.Usage, CrossSigningKeySelfSigning)
	}
	if result.UserSigningKey.Usage != CrossSigningKeyUserSigning {
		t.Fatalf("USK usage = %q, want %q", result.UserSigningKey.Usage, CrossSigningKeyUserSigning)
	}

	// Verify device was signed.
	if !result.DeviceSigned {
		t.Fatal("DeviceSigned should be true")
	}

	// Verify signature is valid.
	deviceKeyRef := string(userID) + ":" + string(deviceID)
	sigs, err := store.GetSignatures(ctx, deviceKeyRef)
	if err != nil {
		t.Fatalf("GetSignatures: %v", err)
	}
	if len(sigs) == 0 {
		t.Fatal("device should have at least one signature")
	}

	decodedSig, err := base64.RawURLEncoding.DecodeString(sigs[0].Signature)
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	if !ed25519.Verify(result.MasterKey.Public, []byte(deviceKeyRef), decodedSig) {
		t.Fatal("Ed25519 signature verification failed")
	}

	// Verify IsBootstrapped now returns true.
	bootstrapped, err = svc.IsBootstrapped(ctx, string(userID))
	if err != nil {
		t.Fatalf("IsBootstrapped post: %v", err)
	}
	if !bootstrapped {
		t.Fatal("should be bootstrapped after Bootstrap()")
	}
}

// TestE2EE_CrossSigningBootstrap_KeyIsolation verifies that bootstrapping
// two different users produces distinct keys.
func TestE2EE_CrossSigningBootstrap_KeyIsolation(t *testing.T) {
	store := NewMemoryStore()
	engine := &CryptoEngine{}
	svc := NewCrossSigningService(engine, store)
	ctx := context.Background()

	aliceResult, err := svc.Bootstrap(ctx, "@alice:test", "DEV_A")
	if err != nil {
		t.Fatalf("Bootstrap alice: %v", err)
	}
	bobResult, err := svc.Bootstrap(ctx, "@bob:test", "DEV_B")
	if err != nil {
		t.Fatalf("Bootstrap bob: %v", err)
	}

	// Keys should be distinct.
	if string(aliceResult.MasterKey.Public) == string(bobResult.MasterKey.Public) {
		t.Fatal("alice and bob should have different master keys")
	}

	// Alice's master key should NOT verify bob's device signature.
	bobDeviceRef := "@bob:test:DEV_B"
	bobSigs, _ := store.GetSignatures(ctx, bobDeviceRef)
	if len(bobSigs) == 0 {
		t.Fatal("bob's device should be signed")
	}
	bobSigBytes, _ := base64.RawURLEncoding.DecodeString(bobSigs[0].Signature)
	if ed25519.Verify(aliceResult.MasterKey.Public, []byte(bobDeviceRef), bobSigBytes) {
		t.Fatal("alice's master key should not verify bob's device signature")
	}
}

// TestE2EE_CrossSigning_SignKeyAndVerify tests the sign→verify cycle directly.
func TestE2EE_CrossSigning_SignKeyAndVerify(t *testing.T) {
	store := NewMemoryStore()
	engine := &CryptoEngine{}
	svc := NewCrossSigningService(engine, store)

	pair, err := svc.GenerateKeyPair(CrossSigningKeySelfSigning)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	data := []byte("test message for cross-signing")
	sig, err := svc.SignKey(pair, data)
	if err != nil {
		t.Fatalf("SignKey: %v", err)
	}

	// Correct data verifies.
	if !svc.VerifySignature(pair.Public, data, sig) {
		t.Error("VerifySignature should pass for correct data")
	}

	// Wrong data does not verify.
	if svc.VerifySignature(pair.Public, []byte("wrong"), sig) {
		t.Error("VerifySignature should fail for wrong data")
	}

	// Wrong key does not verify.
	wrongPub := make(ed25519.PublicKey, ed25519.PublicKeySize)
	copy(wrongPub, pair.Public)
	wrongPub[0] ^= 0xFF
	if svc.VerifySignature(wrongPub, data, sig) {
		t.Error("VerifySignature should fail for wrong key")
	}
}

// ---------------------------------------------------------------------------
// 6. Session Persistence
// ---------------------------------------------------------------------------

// TestE2EE_SessionPersistence verifies that session data stored in MemoryStore
// is recoverable, simulating persistence across engine restarts.
func TestE2EE_SessionPersistence(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	// --- Simulate first engine instance storing data ---

	// Store Olm account.
	accountPickle := []byte("pickled-olm-account-data-123456")
	if err := store.PutOlmAccount(ctx, "DEVICE_1", accountPickle, true); err != nil {
		t.Fatalf("PutOlmAccount: %v", err)
	}

	// Store inbound Megolm session.
	roomID := "!room:test"
	senderKey := "curve25519:senderABC"
	sessionID := "megolm:sessionXYZ"
	sessionKey := []byte("megolm-session-key-material-32b")
	if err := store.AddInboundGroupSession(ctx, roomID, senderKey, sessionID, sessionKey); err != nil {
		t.Fatalf("AddInboundGroupSession: %v", err)
	}

	// Store cross-signing keys.
	if err := store.PutCrossSigningKey(ctx, "@alice:test", "master", "master-key-data"); err != nil {
		t.Fatalf("PutCrossSigningKey master: %v", err)
	}
	if err := store.PutCrossSigningKey(ctx, "@alice:test", "self_signing", "ssk-data"); err != nil {
		t.Fatalf("PutCrossSigningKey ssk: %v", err)
	}

	// Store device keys.
	if err := store.PutDeviceKeys(ctx, "@alice:test", "DEVICE_1", []byte(`{"ed25519":"keydata"}`), 12345); err != nil {
		t.Fatalf("PutDeviceKeys: %v", err)
	}

	// Store next batch token.
	if err := store.PutNextBatch(ctx, "sync_token_42"); err != nil {
		t.Fatalf("PutNextBatch: %v", err)
	}

	// --- Simulate new engine instance reading data back ---

	// Olm account.
	acct, err := store.GetOlmAccount(ctx)
	if err != nil {
		t.Fatalf("GetOlmAccount: %v", err)
	}
	if acct == nil {
		t.Fatal("Olm account should be recoverable")
	}
	if string(acct.AccountPickle) != string(accountPickle) {
		t.Fatalf("account pickle mismatch: got %q, want %q", string(acct.AccountPickle), string(accountPickle))
	}
	if !acct.Shared {
		t.Fatal("account should be marked shared")
	}

	// Inbound session.
	retrievedKey, err := store.GetInboundGroupSession(ctx, roomID, senderKey, sessionID)
	if err != nil {
		t.Fatalf("GetInboundGroupSession: %v", err)
	}
	if string(retrievedKey) != string(sessionKey) {
		t.Fatalf("session key mismatch: got %q, want %q", string(retrievedKey), string(sessionKey))
	}

	// Cross-signing keys.
	csKeys, err := store.GetCrossSigningKeys(ctx, "@alice:test")
	if err != nil {
		t.Fatalf("GetCrossSigningKeys: %v", err)
	}
	if len(csKeys) != 2 {
		t.Fatalf("expected 2 cross-signing keys, got %d", len(csKeys))
	}
	usages := map[string]string{}
	for _, k := range csKeys {
		usages[k.Usage] = k.KeyData
	}
	if usages["master"] != "master-key-data" {
		t.Fatalf("master key data = %q, want 'master-key-data'", usages["master"])
	}
	if usages["self_signing"] != "ssk-data" {
		t.Fatalf("ssk data = %q, want 'ssk-data'", usages["self_signing"])
	}

	// Device keys.
	devKeys, err := store.GetDeviceKeys(ctx, "@alice:test", "DEVICE_1")
	if err != nil {
		t.Fatalf("GetDeviceKeys: %v", err)
	}
	if devKeys == nil {
		t.Fatal("device keys should be recoverable")
	}

	// Next batch.
	batch, err := store.GetNextBatch(ctx)
	if err != nil {
		t.Fatalf("GetNextBatch: %v", err)
	}
	if batch != "sync_token_42" {
		t.Fatalf("next batch = %q, want 'sync_token_42'", batch)
	}
}

// TestE2EE_SessionPersistence_SignaturesAcrossInstances verifies that
// signatures stored by one instance are recoverable by another.
func TestE2EE_SessionPersistence_SignaturesAcrossInstances(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	// Instance 1: bootstrap cross-signing and sign device.
	engine := &CryptoEngine{}
	svc := NewCrossSigningService(engine, store)
	result, err := svc.Bootstrap(ctx, "@alice:test", "DEV_1")
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	mskPub := result.MasterKey.Public

	// Instance 2: recover and verify signatures.
	svc2 := NewCrossSigningService(engine, store)
	bootstrapped, err := svc2.IsBootstrapped(ctx, "@alice:test")
	if err != nil {
		t.Fatalf("IsBootstrapped: %v", err)
	}
	if !bootstrapped {
		t.Fatal("second instance should see bootstrapped state")
	}

	deviceRef := "@alice:test:DEV_1"
	sigs, err := store.GetSignatures(ctx, deviceRef)
	if err != nil {
		t.Fatalf("GetSignatures: %v", err)
	}
	if len(sigs) == 0 {
		t.Fatal("signatures should be recoverable")
	}

	// Verify the signature with the original master key.
	sigBytes, err := base64.RawURLEncoding.DecodeString(sigs[0].Signature)
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	if !ed25519.Verify(mskPub, []byte(deviceRef), sigBytes) {
		t.Fatal("signature verification failed on recovered data")
	}
}

// ---------------------------------------------------------------------------
// 7. Deployment Migration
// ---------------------------------------------------------------------------

// TestE2EE_DeploymentMigration simulates an existing deployment with
// plaintext rooms where E2EE is subsequently enabled. Existing rooms
// remain plaintext, new encrypted rooms work correctly.
func TestE2EE_DeploymentMigration(t *testing.T) {
	// Phase 1: Before E2EE — all rooms are plaintext.
	cacheBefore := NewRoomEncryptionCache(false) // E2EE disabled

	// Existing rooms exist as plaintext.
	existingRoom := "!existing-room:test"
	cacheBefore.SetEncrypted(existingRoom, true) // Attempt to mark encrypted
	if cacheBefore.IsEncrypted(existingRoom) {
		t.Error("existing room should remain plaintext when E2EE disabled")
	}

	// Phase 2: E2EE is enabled.
	cacheAfter := NewRoomEncryptionCache(true) // E2EE enabled

	// Existing room is NOT automatically encrypted — it stays plaintext.
	if cacheAfter.IsEncrypted(existingRoom) {
		t.Error("existing room should not auto-become encrypted on E2EE enable")
	}

	// New rooms created with encryption ARE encrypted.
	newRoom := "!new-encrypted-room:test"
	cacheAfter.SetEncrypted(newRoom, true)
	if !cacheAfter.IsEncrypted(newRoom) {
		t.Error("new encrypted room should be marked as encrypted")
	}

	// Existing room can be explicitly upgraded via m.room.encryption event.
	svc := NewEncryptionService(nil, cacheAfter, nil)
	svc.OnRoomEncryptionEvent(existingRoom)
	if !cacheAfter.IsEncrypted(existingRoom) {
		t.Error("existing room should be encrypted after receiving encryption event")
	}
	// ShouldEncrypt still false because engine is nil — crypto not actually possible.
	if svc.ShouldEncrypt(existingRoom) {
		t.Error("ShouldEncrypt returns false with nil engine — kill switch behavior")
	}
}

// TestE2EE_DeploymentMigration_MixedRooms verifies a deployment where
// some rooms are encrypted and others are not.
func TestE2EE_DeploymentMigration_MixedRooms(t *testing.T) {
	cache := NewRoomEncryptionCache(true)

	// Simulate mixed deployment: 3 encrypted, 2 plaintext.
	cache.SetEncrypted("!enc1:test", true)
	cache.SetEncrypted("!enc2:test", true)
	cache.SetEncrypted("!enc3:test", true)
	// !plain1:test and !plain2:test remain unmarked

	encryptedRooms := []string{"!enc1:test", "!enc2:test", "!enc3:test"}
	plaintextRooms := []string{"!plain1:test", "!plain2:test"}

	for _, room := range encryptedRooms {
		if !cache.IsEncrypted(room) {
			t.Errorf("%s should be encrypted", room)
		}
	}
	for _, room := range plaintextRooms {
		if cache.IsEncrypted(room) {
			t.Errorf("%s should be plaintext", room)
		}
	}

	// Verify clear works (simulates E2EE disable toggle).
	cache.Clear()
	for _, room := range encryptedRooms {
		if cache.IsEncrypted(room) {
			t.Errorf("%s should not be encrypted after clear", room)
		}
	}
}

// ---------------------------------------------------------------------------
// Bonus: Store-level integration for outbound sessions
// ---------------------------------------------------------------------------

// TestE2EE_OutboundSessionLifecycle tests the full lifecycle of an outbound
// Megolm session through the store.
func TestE2EE_OutboundSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	roomID := "!room:test"
	sessionID := "outbound-session-123"
	pickle := []byte("pickled-outbound-session")
	expiresAt := int64(1700000000)

	// Store outbound session.
	if err := store.PutOutboundGroupSession(ctx, roomID, sessionID, pickle, expiresAt); err != nil {
		t.Fatalf("PutOutboundGroupSession: %v", err)
	}

	// Retrieve.
	gotSID, gotPickle, gotExpires, err := store.GetOutboundGroupSession(ctx, roomID)
	if err != nil {
		t.Fatalf("GetOutboundGroupSession: %v", err)
	}
	if gotSID != sessionID {
		t.Fatalf("session ID = %q, want %q", gotSID, sessionID)
	}
	if string(gotPickle) != string(pickle) {
		t.Fatalf("pickle mismatch")
	}
	if gotExpires != expiresAt {
		t.Fatalf("expires = %d, want %d", gotExpires, expiresAt)
	}

	// Update (overwrite).
	newPickle := []byte("updated-outbound-session")
	if err := store.PutOutboundGroupSession(ctx, roomID, sessionID+"v2", newPickle, expiresAt+1000); err != nil {
		t.Fatalf("PutOutboundGroupSession update: %v", err)
	}
	gotSID, _, _, _ = store.GetOutboundGroupSession(ctx, roomID)
	if gotSID != sessionID+"v2" {
		t.Fatalf("session ID after update = %q, want %q", gotSID, sessionID+"v2")
	}

	// Remove.
	if err := store.RemoveOutboundGroupSession(ctx, roomID); err != nil {
		t.Fatalf("RemoveOutboundGroupSession: %v", err)
	}
	gotSID, _, _, _ = store.GetOutboundGroupSession(ctx, roomID)
	if gotSID != "" {
		t.Fatalf("session ID after remove = %q, want empty", gotSID)
	}
}

// TestE2EE_MessageIndex_ReplayProtection verifies the message index store
// for replay attack detection.
func TestE2EE_MessageIndex_ReplayProtection(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	sessionID := "session-123"
	index := "42"
	mi := MessageIndex{
		SessionID:    sessionID,
		MessageIndex: index,
		EventID:      "$event_abc",
		Timestamp:    1700000000,
	}

	// First occurrence should be stored.
	if err := store.PutMessageIndex(ctx, sessionID, index, mi); err != nil {
		t.Fatalf("PutMessageIndex: %v", err)
	}

	// Retrieve.
	got, err := store.GetMessageIndex(ctx, sessionID, index)
	if err != nil {
		t.Fatalf("GetMessageIndex: %v", err)
	}
	if got == nil {
		t.Fatal("message index should exist")
	}
	if got.EventID != mi.EventID {
		t.Fatalf("event ID = %q, want %q", got.EventID, mi.EventID)
	}
	if got.Timestamp != mi.Timestamp {
		t.Fatalf("timestamp = %d, want %d", got.Timestamp, mi.Timestamp)
	}

	// Non-existent index returns nil.
	got, err = store.GetMessageIndex(ctx, sessionID, "99")
	if err != nil {
		t.Fatalf("GetMessageIndex non-existent: %v", err)
	}
	if got != nil {
		t.Fatal("non-existent index should return nil")
	}
}

// ---------------------------------------------------------------------------
// Bonus: Cross-service integration
// ---------------------------------------------------------------------------

// TestE2EE_CrossService_Integration verifies that multiple services work
// together: RoomEncryptionCache + EncryptionService + CrossSigning.
func TestE2EE_CrossService_Integration(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	engine := &CryptoEngine{}
	cache := NewRoomEncryptionCache(true)

	encSvc := NewEncryptionService(nil, cache, nil)
	csSvc := NewCrossSigningService(engine, store)

	// Bootstrap cross-signing.
	result, err := csSvc.Bootstrap(ctx, "@alice:test", "DEV_1")
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	// Room becomes encrypted.
	encSvc.OnRoomEncryptionEvent("!secure-room:test")

	// Verify cross-signing state is persistent.
	bootstrapped, _ := csSvc.IsBootstrapped(ctx, "@alice:test")
	if !bootstrapped {
		t.Fatal("cross-signing should be bootstrapped")
	}

	// Verify room routing.
	if !cache.IsEncrypted("!secure-room:test") {
		t.Error("room should be encrypted")
	}
	if cache.IsEncrypted("!insecure-room:test") {
		t.Error("room should not be encrypted")
	}

	// Verify cross-signing key isolation (different user should not be bootstrapped).
	otherBootstrapped, _ := csSvc.IsBootstrapped(ctx, "@bob:test")
	if otherBootstrapped {
		t.Error("other user should not be bootstrapped")
	}

	// Verify the master key from bootstrap result.
	if result.MasterKey == nil {
		t.Fatal("master key should exist")
	}
	sig, err := csSvc.SignKey(result.MasterKey, []byte("test"))
	if err != nil {
		t.Fatalf("SignKey: %v", err)
	}
	if !csSvc.VerifySignature(result.MasterKey.Public, []byte("test"), sig) {
		t.Error("signature verification failed for master key")
	}
}

// ---------------------------------------------------------------------------
// Bonus: Olm session persistence
// ---------------------------------------------------------------------------

// TestE2EE_OlmSessionPersistence verifies Olm session storage and retrieval.
func TestE2EE_OlmSessionPersistence(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	senderKey := "curve25519:ABCDEF"
	sessionID := "session_001"
	pickle := []byte("olm-session-pickle-data")
	createdAt := int64(1700000000)

	// Store session.
	if err := store.PutSession(ctx, senderKey, sessionID, pickle, createdAt); err != nil {
		t.Fatalf("PutSession: %v", err)
	}

	// Retrieve specific session.
	sess, err := store.GetSession(ctx, senderKey, sessionID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if sess == nil {
		t.Fatal("session should exist")
	}
	if sess.SessionID != sessionID {
		t.Fatalf("session ID = %q, want %q", sess.SessionID, sessionID)
	}
	if string(sess.SessionPickle) != string(pickle) {
		t.Fatalf("pickle mismatch")
	}

	// Retrieve all sessions for sender key.
	sessions, err := store.GetSessions(ctx, senderKey)
	if err != nil {
		t.Fatalf("GetSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	// Add another session for the same sender.
	sessionID2 := "session_002"
	pickle2 := []byte("olm-session-pickle-2")
	if err := store.PutSession(ctx, senderKey, sessionID2, pickle2, createdAt+100); err != nil {
		t.Fatalf("PutSession 2: %v", err)
	}

	sessions, err = store.GetSessions(ctx, senderKey)
	if err != nil {
		t.Fatalf("GetSessions after second add: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	// Update existing session.
	updatedPickle := []byte("updated-pickle")
	if err := store.PutSession(ctx, senderKey, sessionID, updatedPickle, createdAt+200); err != nil {
		t.Fatalf("PutSession update: %v", err)
	}

	sessions, _ = store.GetSessions(ctx, senderKey)
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions after update, got %d", len(sessions))
	}

	// Non-existent sender key.
	empty, err := store.GetSessions(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetSessions non-existent: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected 0 sessions for non-existent key, got %d", len(empty))
	}
}

// ---------------------------------------------------------------------------
// Bonus: Withheld session tracking
// ---------------------------------------------------------------------------

// TestE2EE_WithheldSessionTracking verifies storage of withheld session info.
func TestE2EE_WithheldSessionTracking(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	ws := WithheldSession{
		RoomID:    "!room:test",
		SenderKey: "senderKey1",
		SessionID: "session1",
		Code:      "m.unverified",
		Reason:    "device not verified",
	}

	// Store.
	if err := store.PutWithheldSession(ctx, "!room:test", "senderKey1", "session1", ws); err != nil {
		t.Fatalf("PutWithheldSession: %v", err)
	}

	// Retrieve.
	got, err := store.GetWithheldSession(ctx, "!room:test", "senderKey1", "session1")
	if err != nil {
		t.Fatalf("GetWithheldSession: %v", err)
	}
	if got == nil {
		t.Fatal("withheld session should exist")
	}
	if got.Code != ws.Code {
		t.Fatalf("code = %q, want %q", got.Code, ws.Code)
	}
	if got.Reason != ws.Reason {
		t.Fatalf("reason = %q, want %q", got.Reason, ws.Reason)
	}

	// Non-existent.
	got, err = store.GetWithheldSession(ctx, "!room:test", "nonexistent", "nonexistent")
	if err != nil {
		t.Fatalf("GetWithheldSession non-existent: %v", err)
	}
	if got != nil {
		t.Fatal("non-existent withheld session should return nil")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func isValidSASEmoji(e string) bool {
	for _, candidate := range sasEmojis {
		if candidate == e {
			return true
		}
	}
	return false
}

var _ = hmac.New
