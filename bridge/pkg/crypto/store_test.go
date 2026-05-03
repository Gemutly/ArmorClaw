package crypto

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// openTestDB creates an in-memory SQLite database for testing
func openTestDB(t *testing.T) *KeystoreBackedStore {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test_crypto.db")

	// Use regular file path — go-sqlcipher accepts unencrypted databases too
	store, err := NewKeystoreBackedStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}

	return store
}

func TestStoreOlmAccountCRUD(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()
	ctx := context.Background()

	// Get on empty store returns nil
	acct, err := store.GetOlmAccount(ctx)
	if err != nil {
		t.Fatalf("GetOlmAccount on empty store: %v", err)
	}
	if acct != nil {
		t.Fatal("expected nil account on empty store")
	}

	// Put
	pickle := []byte("olm-account-pickle-data-12345")
	err = store.PutOlmAccount(ctx, "DEVICE_ABC", pickle, false)
	if err != nil {
		t.Fatalf("PutOlmAccount: %v", err)
	}

	// Get
	acct, err = store.GetOlmAccount(ctx)
	if err != nil {
		t.Fatalf("GetOlmAccount after put: %v", err)
	}
	if acct == nil {
		t.Fatal("expected account, got nil")
	}
	if acct.DeviceID != "DEVICE_ABC" {
		t.Errorf("DeviceID = %q, want %q", acct.DeviceID, "DEVICE_ABC")
	}
	if string(acct.AccountPickle) != string(pickle) {
		t.Errorf("AccountPickle mismatch")
	}
	if acct.Shared {
		t.Error("expected Shared=false")
	}

	// Update shared status
	err = store.PutOlmAccount(ctx, "DEVICE_ABC", pickle, true)
	if err != nil {
		t.Fatalf("PutOlmAccount (update): %v", err)
	}

	acct, err = store.GetOlmAccount(ctx)
	if err != nil {
		t.Fatalf("GetOlmAccount after update: %v", err)
	}
	if !acct.Shared {
		t.Error("expected Shared=true after update")
	}
}

func TestStoreOutboundGroupSessionCRUD(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()
	ctx := context.Background()

	// Get on empty store returns zero values
	sessID, pickle, expiresAt, err := store.GetOutboundGroupSession(ctx, "!room:test")
	if err != nil {
		t.Fatalf("GetOutboundGroupSession on empty: %v", err)
	}
	if sessID != "" || pickle != nil || expiresAt != 0 {
		t.Fatal("expected zero values for non-existent session")
	}

	// Put
	sessionPickle := []byte("outbound-session-pickle")
	expires := time.Now().Add(24 * time.Hour).Unix()
	err = store.PutOutboundGroupSession(ctx, "!room:test", "sess_123", sessionPickle, expires)
	if err != nil {
		t.Fatalf("PutOutboundGroupSession: %v", err)
	}

	// Get
	sessID, pickle, expiresAt, err = store.GetOutboundGroupSession(ctx, "!room:test")
	if err != nil {
		t.Fatalf("GetOutboundGroupSession after put: %v", err)
	}
	if sessID != "sess_123" {
		t.Errorf("SessionID = %q, want %q", sessID, "sess_123")
	}
	if string(pickle) != string(sessionPickle) {
		t.Error("pickle mismatch")
	}
	if expiresAt != expires {
		t.Errorf("ExpiresAt = %d, want %d", expiresAt, expires)
	}

	// Update (upsert)
	newPickle := []byte("updated-pickle")
	err = store.PutOutboundGroupSession(ctx, "!room:test", "sess_456", newPickle, expires+100)
	if err != nil {
		t.Fatalf("PutOutboundGroupSession (update): %v", err)
	}

	sessID, _, _, err = store.GetOutboundGroupSession(ctx, "!room:test")
	if err != nil {
		t.Fatalf("GetOutboundGroupSession after update: %v", err)
	}
	if sessID != "sess_456" {
		t.Errorf("SessionID after update = %q, want %q", sessID, "sess_456")
	}

	// Remove
	err = store.RemoveOutboundGroupSession(ctx, "!room:test")
	if err != nil {
		t.Fatalf("RemoveOutboundGroupSession: %v", err)
	}

	sessID, _, _, err = store.GetOutboundGroupSession(ctx, "!room:test")
	if err != nil {
		t.Fatalf("GetOutboundGroupSession after remove: %v", err)
	}
	if sessID != "" {
		t.Errorf("SessionID after remove = %q, want empty", sessID)
	}
}

func TestStoreDeviceKeysCRUD(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()
	ctx := context.Background()

	// Get on empty returns nil
	keys, err := store.GetDeviceKeys(ctx, "@alice:test", "DEV1")
	if err != nil {
		t.Fatalf("GetDeviceKeys on empty: %v", err)
	}
	if keys != nil {
		t.Fatal("expected nil for non-existent keys")
	}

	// Put
	keyData := []byte(`{"ed25519":"key123","curve25519":"key456"}`)
	err = store.PutDeviceKeys(ctx, "@alice:test", "DEV1", keyData, time.Now().Unix())
	if err != nil {
		t.Fatalf("PutDeviceKeys: %v", err)
	}

	// Get
	keys, err = store.GetDeviceKeys(ctx, "@alice:test", "DEV1")
	if err != nil {
		t.Fatalf("GetDeviceKeys after put: %v", err)
	}
	if string(keys) != string(keyData) {
		t.Errorf("key data mismatch: got %q, want %q", string(keys), string(keyData))
	}

	// Update (upsert)
	newKeyData := []byte(`{"ed25519":"key789","curve25519":"key012"}`)
	err = store.PutDeviceKeys(ctx, "@alice:test", "DEV1", newKeyData, time.Now().Unix())
	if err != nil {
		t.Fatalf("PutDeviceKeys (update): %v", err)
	}

	keys, err = store.GetDeviceKeys(ctx, "@alice:test", "DEV1")
	if err != nil {
		t.Fatalf("GetDeviceKeys after update: %v", err)
	}
	if string(keys) != string(newKeyData) {
		t.Error("key data not updated")
	}

	// Different device for same user
	err = store.PutDeviceKeys(ctx, "@alice:test", "DEV2", keyData, time.Now().Unix())
	if err != nil {
		t.Fatalf("PutDeviceKeys (second device): %v", err)
	}

	keys, err = store.GetDeviceKeys(ctx, "@alice:test", "DEV2")
	if err != nil {
		t.Fatalf("GetDeviceKeys (second device): %v", err)
	}
	if string(keys) != string(keyData) {
		t.Error("second device key data mismatch")
	}

	// Original device still intact
	keys, err = store.GetDeviceKeys(ctx, "@alice:test", "DEV1")
	if err != nil {
		t.Fatalf("GetDeviceKeys (original device): %v", err)
	}
	if string(keys) != string(newKeyData) {
		t.Error("original device key data lost after adding second device")
	}
}

func TestStoreOlmSessionCRUD(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()
	ctx := context.Background()

	senderKey := "sender_curve25519_key"
	sessionID := "olm_session_id_1"
	pickle := []byte("olm-session-pickle-data")
	createdAt := time.Now().Unix()

	// Get on empty returns nil
	sess, err := store.GetSession(ctx, senderKey, sessionID)
	if err != nil {
		t.Fatalf("GetSession on empty: %v", err)
	}
	if sess != nil {
		t.Fatal("expected nil for non-existent session")
	}

	// GetSessions on empty returns empty slice
	sessions, err := store.GetSessions(ctx, senderKey)
	if err != nil {
		t.Fatalf("GetSessions on empty: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}

	// Put
	err = store.PutSession(ctx, senderKey, sessionID, pickle, createdAt)
	if err != nil {
		t.Fatalf("PutSession: %v", err)
	}

	// Get
	sess, err = store.GetSession(ctx, senderKey, sessionID)
	if err != nil {
		t.Fatalf("GetSession after put: %v", err)
	}
	if sess == nil {
		t.Fatal("expected session, got nil")
	}
	if sess.SessionID != sessionID {
		t.Errorf("SessionID = %q, want %q", sess.SessionID, sessionID)
	}
	if sess.SenderKey != senderKey {
		t.Errorf("SenderKey = %q, want %q", sess.SenderKey, senderKey)
	}
	if string(sess.SessionPickle) != string(pickle) {
		t.Error("session pickle mismatch")
	}

	// GetSessions
	sessions, err = store.GetSessions(ctx, senderKey)
	if err != nil {
		t.Fatalf("GetSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	// Add second session for same sender
	sessionID2 := "olm_session_id_2"
	pickle2 := []byte("olm-session-pickle-data-2")
	err = store.PutSession(ctx, senderKey, sessionID2, pickle2, createdAt+100)
	if err != nil {
		t.Fatalf("PutSession (second): %v", err)
	}

	sessions, err = store.GetSessions(ctx, senderKey)
	if err != nil {
		t.Fatalf("GetSessions (two): %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	// Update existing session
	updatedPickle := []byte("updated-olm-session-pickle")
	err = store.PutSession(ctx, senderKey, sessionID, updatedPickle, createdAt)
	if err != nil {
		t.Fatalf("PutSession (update): %v", err)
	}

	sess, err = store.GetSession(ctx, senderKey, sessionID)
	if err != nil {
		t.Fatalf("GetSession after update: %v", err)
	}
	if string(sess.SessionPickle) != string(updatedPickle) {
		t.Error("session pickle not updated")
	}

	// Still 2 sessions after update
	sessions, err = store.GetSessions(ctx, senderKey)
	if err != nil {
		t.Fatalf("GetSessions after update: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions after update, got %d", len(sessions))
	}

	// Different sender key
	senderKey2 := "other_sender_key"
	err = store.PutSession(ctx, senderKey2, "session_3", pickle, createdAt)
	if err != nil {
		t.Fatalf("PutSession (different sender): %v", err)
	}

	sessions, err = store.GetSessions(ctx, senderKey2)
	if err != nil {
		t.Fatalf("GetSessions (different sender): %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session for other sender, got %d", len(sessions))
	}
}

func TestStoreNextBatchCRUD(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()
	ctx := context.Background()

	// Get on empty returns empty string
	token, err := store.GetNextBatch(ctx)
	if err != nil {
		t.Fatalf("GetNextBatch on empty: %v", err)
	}
	if token != "" {
		t.Errorf("expected empty token, got %q", token)
	}

	// Put
	err = store.PutNextBatch(ctx, "batch_token_12345")
	if err != nil {
		t.Fatalf("PutNextBatch: %v", err)
	}

	// Get
	token, err = store.GetNextBatch(ctx)
	if err != nil {
		t.Fatalf("GetNextBatch after put: %v", err)
	}
	if token != "batch_token_12345" {
		t.Errorf("token = %q, want %q", token, "batch_token_12345")
	}

	// Update
	err = store.PutNextBatch(ctx, "batch_token_67890")
	if err != nil {
		t.Fatalf("PutNextBatch (update): %v", err)
	}

	token, err = store.GetNextBatch(ctx)
	if err != nil {
		t.Fatalf("GetNextBatch after update: %v", err)
	}
	if token != "batch_token_67890" {
		t.Errorf("token after update = %q, want %q", token, "batch_token_67890")
	}
}

func TestStoreCrossSigningKeyStub(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()
	ctx := context.Background()

	// PutCrossSigningKey should not error (stub)
	err := store.PutCrossSigningKey(ctx, "@alice:test", "master", "ed25519:abc123")
	if err != nil {
		t.Fatalf("PutCrossSigningKey (stub): %v", err)
	}
}

func TestStoreGetGroupSessionsForRoom(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()
	ctx := context.Background()

	// Empty room returns empty slice
	details, err := store.GetGroupSessionsForRoom(ctx, "!room1:test")
	if err != nil {
		t.Fatalf("GetGroupSessionsForRoom on empty: %v", err)
	}
	if len(details) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(details))
	}

	// Add sessions to multiple rooms
	sessionKey1 := []byte("session_key_room1_sess1")
	sessionKey2 := []byte("session_key_room1_sess2")
	sessionKey3 := []byte("session_key_room2_sess1")

	err = store.AddInboundGroupSession(ctx, "!room1:test", "sender1", "sess1", sessionKey1)
	if err != nil {
		t.Fatalf("AddInboundGroupSession: %v", err)
	}
	err = store.AddInboundGroupSession(ctx, "!room1:test", "sender2", "sess2", sessionKey2)
	if err != nil {
		t.Fatalf("AddInboundGroupSession: %v", err)
	}
	err = store.AddInboundGroupSession(ctx, "!room2:test", "sender1", "sess3", sessionKey3)
	if err != nil {
		t.Fatalf("AddInboundGroupSession: %v", err)
	}

	// Get sessions for room1
	details, err = store.GetGroupSessionsForRoom(ctx, "!room1:test")
	if err != nil {
		t.Fatalf("GetGroupSessionsForRoom: %v", err)
	}
	if len(details) != 2 {
		t.Fatalf("expected 2 sessions for room1, got %d", len(details))
	}

	// Verify session data
	foundSess1 := false
	foundSess2 := false
	for _, d := range details {
		if d.SessionID == "sess1" {
			foundSess1 = true
			if string(d.SessionKey) != string(sessionKey1) {
				t.Error("sess1 key data mismatch")
			}
		}
		if d.SessionID == "sess2" {
			foundSess2 = true
			if string(d.SessionKey) != string(sessionKey2) {
				t.Error("sess2 key data mismatch")
			}
		}
	}
	if !foundSess1 || !foundSess2 {
		t.Error("missing sessions in result")
	}

	// Room2 has only 1 session
	details, err = store.GetGroupSessionsForRoom(ctx, "!room2:test")
	if err != nil {
		t.Fatalf("GetGroupSessionsForRoom (room2): %v", err)
	}
	if len(details) != 1 {
		t.Fatalf("expected 1 session for room2, got %d", len(details))
	}
}

func TestStoreUpdateInboundGroupSession(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()
	ctx := context.Background()

	// Add a session
	originalKey := []byte("original_key")
	err := store.AddInboundGroupSession(ctx, "!room:test", "sender1", "sess1", originalKey)
	if err != nil {
		t.Fatalf("AddInboundGroupSession: %v", err)
	}

	// Update it
	updatedKey := []byte("updated_key")
	err = store.UpdateInboundGroupSession(ctx, "!room:test", "sender1", "sess1", updatedKey)
	if err != nil {
		t.Fatalf("UpdateInboundGroupSession: %v", err)
	}

	// Verify update
	key, err := store.GetInboundGroupSession(ctx, "!room:test", "sender1", "sess1")
	if err != nil {
		t.Fatalf("GetInboundGroupSession: %v", err)
	}
	if string(key) != string(updatedKey) {
		t.Errorf("key after update = %q, want %q", string(key), string(updatedKey))
	}

	// UpdateInboundGroupSession on non-existent session should create it
	newKey := []byte("new_session_key")
	err = store.UpdateInboundGroupSession(ctx, "!room:test", "sender2", "sess_new", newKey)
	if err != nil {
		t.Fatalf("UpdateInboundGroupSession (new): %v", err)
	}

	key, err = store.GetInboundGroupSession(ctx, "!room:test", "sender2", "sess_new")
	if err != nil {
		t.Fatalf("GetInboundGroupSession (new): %v", err)
	}
	if string(key) != string(newKey) {
		t.Errorf("key for new session = %q, want %q", string(key), string(newKey))
	}
}

func TestStoreFlush(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()
	ctx := context.Background()

	err := store.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
}

func TestStoreClear(t *testing.T) {
	store := openTestDB(t)
	defer store.Close()
	ctx := context.Background()

	// Populate all tables
	_ = store.PutOlmAccount(ctx, "DEV1", []byte("pickle"), false)
	_ = store.AddInboundGroupSession(ctx, "!room:test", "sender", "sess1", []byte("key"))
	_ = store.PutOutboundGroupSession(ctx, "!room:test", "out1", []byte("out_pickle"), 0)
	_ = store.PutDeviceKeys(ctx, "@user:test", "DEV1", []byte("keys"), time.Now().Unix())
	_ = store.PutSession(ctx, "sender1", "olm1", []byte("olm_pickle"), time.Now().Unix())
	_ = store.PutNextBatch(ctx, "token123")

	// Clear
	err := store.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}

	// Verify all cleared
	acct, _ := store.GetOlmAccount(ctx)
	if acct != nil {
		t.Error("olm account not cleared")
	}

	_, err = store.GetInboundGroupSession(ctx, "!room:test", "sender", "sess1")
	if err != ErrSessionNotFound {
		t.Errorf("inbound session not cleared: %v", err)
	}

	sessID, _, _, _ := store.GetOutboundGroupSession(ctx, "!room:test")
	if sessID != "" {
		t.Error("outbound session not cleared")
	}

	keys, _ := store.GetDeviceKeys(ctx, "@user:test", "DEV1")
	if keys != nil {
		t.Error("device keys not cleared")
	}

	sess, _ := store.GetSession(ctx, "sender1", "olm1")
	if sess != nil {
		t.Error("olm session not cleared")
	}

	token, _ := store.GetNextBatch(ctx)
	if token != "" {
		t.Error("next batch not cleared")
	}
}

func TestSchemaMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration_test.db")

	// Phase 1: Create store with base schema and add data
	store1, err := NewKeystoreBackedStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	ctx := context.Background()

	// Add inbound session to base table
	err = store1.AddInboundGroupSession(ctx, "!room:test", "sender1", "sess1", []byte("key_data"))
	if err != nil {
		t.Fatalf("AddInboundGroupSession: %v", err)
	}
	store1.Close()

	// Phase 2: Reopen — migrations should run and preserve data
	store2, err := NewKeystoreBackedStore(dbPath)
	if err != nil {
		t.Fatalf("failed to reopen store: %v", err)
	}
	defer store2.Close()

	// Verify existing data preserved
	key, err := store2.GetInboundGroupSession(ctx, "!room:test", "sender1", "sess1")
	if err != nil {
		t.Fatalf("GetInboundGroupSession after migration: %v", err)
	}
	if string(key) != "key_data" {
		t.Errorf("key data after migration = %q, want %q", string(key), "key_data")
	}

	// Verify new tables work
	err = store2.PutOlmAccount(ctx, "DEV1", []byte("account_pickle"), true)
	if err != nil {
		t.Fatalf("PutOlmAccount after migration: %v", err)
	}

	acct, err := store2.GetOlmAccount(ctx)
	if err != nil {
		t.Fatalf("GetOlmAccount after migration: %v", err)
	}
	if acct == nil || string(acct.AccountPickle) != "account_pickle" {
		t.Error("olm account not working after migration")
	}

	// Verify schema version recorded
	var version int
	err = store2.db.QueryRow("SELECT version FROM schema_version LIMIT 1").Scan(&version)
	if err != nil {
		t.Fatalf("failed to query schema_version: %v", err)
	}
	if version != currentSchemaVersion {
		t.Errorf("schema version = %d, want %d", version, currentSchemaVersion)
	}
}

func TestSchemaMigrationIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "idempotent_test.db")

	// Create and populate
	store1, err := NewKeystoreBackedStore(dbPath)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	ctx := context.Background()

	_ = store1.PutOlmAccount(ctx, "DEV1", []byte("pickle"), false)
	_ = store1.PutSession(ctx, "sender", "sess1", []byte("olm_pickle"), time.Now().Unix())
	_ = store1.PutNextBatch(ctx, "token1")
	store1.Close()

	// Reopen multiple times — each triggers migration check
	for i := 0; i < 3; i++ {
		store, err := NewKeystoreBackedStore(dbPath)
		if err != nil {
			t.Fatalf("reopen %d: %v", i+1, err)
		}

		// Verify data intact
		acct, err := store.GetOlmAccount(ctx)
		if err != nil {
			t.Fatalf("reopen %d GetOlmAccount: %v", i+1, err)
		}
		if acct == nil || string(acct.AccountPickle) != "pickle" {
			t.Errorf("reopen %d: account data lost", i+1)
		}

		sess, err := store.GetSession(ctx, "sender", "sess1")
		if err != nil {
			t.Fatalf("reopen %d GetSession: %v", i+1, err)
		}
		if sess == nil || string(sess.SessionPickle) != "olm_pickle" {
			t.Errorf("reopen %d: session data lost", i+1)
		}

		token, err := store.GetNextBatch(ctx)
		if err != nil {
			t.Fatalf("reopen %d GetNextBatch: %v", i+1, err)
		}
		if token != "token1" {
			t.Errorf("reopen %d: token = %q, want %q", i+1, token, "token1")
		}

		store.Close()
	}
}

func TestMemoryStoreInterface(t *testing.T) {
	// Verify MemoryStore satisfies the Store interface (compile-time check already done)
	// Quick smoke test of new methods
	store := NewMemoryStore()
	ctx := context.Background()

	// Olm Account
	err := store.PutOlmAccount(ctx, "DEV1", []byte("pickle"), true)
	if err != nil {
		t.Fatalf("PutOlmAccount: %v", err)
	}
	acct, err := store.GetOlmAccount(ctx)
	if err != nil || acct == nil || acct.DeviceID != "DEV1" {
		t.Fatalf("GetOlmAccount: err=%v acct=%v", err, acct)
	}

	// Next batch
	err = store.PutNextBatch(ctx, "token")
	if err != nil {
		t.Fatalf("PutNextBatch: %v", err)
	}
	token, err := store.GetNextBatch(ctx)
	if err != nil || token != "token" {
		t.Fatalf("GetNextBatch: err=%v token=%q", err, token)
	}

	// Device Keys
	err = store.PutDeviceKeys(ctx, "@user:test", "DEV1", []byte("keys"), 123)
	if err != nil {
		t.Fatalf("PutDeviceKeys: %v", err)
	}
	keys, err := store.GetDeviceKeys(ctx, "@user:test", "DEV1")
	if err != nil || string(keys) != "keys" {
		t.Fatalf("GetDeviceKeys: err=%v keys=%q", err, string(keys))
	}

	// Flush
	err = store.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
