// Package crypto provides cryptographic interfaces for E2EE support
package crypto

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	_ "github.com/mutecomm/go-sqlcipher/v4"
)

// currentSchemaVersion is the latest schema version for the crypto store.
// Each new table addition increments this version.
const currentSchemaVersion = 2

// KeystoreBackedStore implements Store using the encrypted keystore database
// This provides persistent, encrypted storage for Megolm session keys
type KeystoreBackedStore struct {
	db   *sql.DB
	mu   sync.RWMutex
	path string
}

// NewKeystoreBackedStore creates a new crypto store backed by SQLCipher
// The dbPath should point to the same encrypted database used by the keystore
func NewKeystoreBackedStore(dbPath string) (*KeystoreBackedStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open crypto store database: %w", err)
	}

	store := &KeystoreBackedStore{
		db:   db,
		path: dbPath,
	}

	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize crypto store schema: %w", err)
	}

	return store, nil
}

// NewKeystoreBackedStoreWithDB creates a store from an existing database connection
// Use this when sharing the same database with the keystore
func NewKeystoreBackedStoreWithDB(db *sql.DB) (*KeystoreBackedStore, error) {
	store := &KeystoreBackedStore{
		db: db,
	}

	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize crypto store schema: %w", err)
	}

	return store, nil
}

// initSchema creates the base tables and runs migrations
func (s *KeystoreBackedStore) initSchema() error {
	// Create base inbound_group_sessions table (always present)
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS inbound_group_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			room_id TEXT NOT NULL,
			sender_key TEXT NOT NULL,
			session_id TEXT NOT NULL,
			session_key BLOB NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(room_id, sender_key, session_id)
		);

		CREATE INDEX IF NOT EXISTS idx_inbound_sessions_room
			ON inbound_group_sessions(room_id);

		CREATE INDEX IF NOT EXISTS idx_inbound_sessions_sender
			ON inbound_group_sessions(sender_key);
	`)
	if err != nil {
		return fmt.Errorf("failed to create base schema: %w", err)
	}

	return s.runMigrations()
}

// getSchemaVersion returns the current schema version, or 0 if not set
func (s *KeystoreBackedStore) getSchemaVersion() (int, error) {
	var version int
	err := s.db.QueryRow(`SELECT version FROM schema_version ORDER BY version DESC LIMIT 1`).Scan(&version)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		// Table might not exist yet
		return 0, nil
	}
	return version, nil
}

// runMigrations applies all pending schema migrations idempotently
func (s *KeystoreBackedStore) runMigrations() error {
	currentVersion, err := s.getSchemaVersion()
	if err != nil {
		return fmt.Errorf("failed to get schema version: %w", err)
	}

	if currentVersion >= currentSchemaVersion {
		return nil
	}

	// Migration v1: Add Olm account, outbound sessions, device keys, Olm sessions, next batch
	if currentVersion < 1 {
		if err := s.migrateV1(); err != nil {
			return fmt.Errorf("migration v1 failed: %w", err)
		}
	}

	// Migration v2: Add cross-signing keys, signatures, message indices, withheld sessions, tracked users
	if currentVersion < 2 {
		if err := s.migrateV2(); err != nil {
			return fmt.Errorf("migration v2 failed: %w", err)
		}
	}

	return nil
}

// migrateV1 adds tables for Olm accounts, outbound group sessions,
// device keys, Olm sessions, next batch tracking, and schema versioning.
// All DDL uses IF NOT EXISTS for idempotency.
func (s *KeystoreBackedStore) migrateV1() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			applied_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS olm_accounts (
			device_id TEXT PRIMARY KEY,
			account_pickle BLOB NOT NULL,
			shared INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
			updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
		);

		CREATE TABLE IF NOT EXISTS outbound_group_sessions (
			room_id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			session_pickle BLOB NOT NULL,
			expires_at INTEGER,
			created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
			updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
		);

		CREATE TABLE IF NOT EXISTS device_keys (
			user_id TEXT NOT NULL,
			device_id TEXT NOT NULL,
			key_data BLOB NOT NULL,
			uploaded_at INTEGER NOT NULL,
			PRIMARY KEY (user_id, device_id)
		);

		CREATE TABLE IF NOT EXISTS olm_sessions (
			session_id TEXT PRIMARY KEY,
			sender_key TEXT NOT NULL,
			session_pickle BLOB NOT NULL,
			created_at INTEGER NOT NULL,
			last_used INTEGER NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_olm_sessions_sender
			ON olm_sessions(sender_key);

		CREATE TABLE IF NOT EXISTS next_batch (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			batch_token TEXT NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create v1 tables: %w", err)
	}

	// Record migration
	_, err = s.db.Exec(`INSERT OR IGNORE INTO schema_version (version, applied_at) VALUES (1, ?)`, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to record schema version: %w", err)
	}

	return nil
}

func (s *KeystoreBackedStore) migrateV2() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS cross_signing_keys (
			user_id TEXT NOT NULL,
			usage TEXT NOT NULL,
			key_data TEXT NOT NULL,
			updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
			PRIMARY KEY (user_id, usage)
		);

		CREATE TABLE IF NOT EXISTS signatures (
			signed_key_id TEXT NOT NULL,
			signer_user_id TEXT NOT NULL,
			signer_key_id TEXT NOT NULL,
			signature TEXT NOT NULL,
			PRIMARY KEY (signed_key_id, signer_user_id, signer_key_id)
		);

		CREATE TABLE IF NOT EXISTS message_indices (
			session_id TEXT NOT NULL,
			message_index TEXT NOT NULL,
			event_id TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			PRIMARY KEY (session_id, message_index)
		);

		CREATE TABLE IF NOT EXISTS withheld_sessions (
			room_id TEXT NOT NULL,
			sender_key TEXT NOT NULL,
			session_id TEXT NOT NULL,
			code TEXT NOT NULL,
			reason TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (room_id, sender_key, session_id)
		);

		CREATE TABLE IF NOT EXISTS tracked_users (
			user_id TEXT PRIMARY KEY,
			outdated INTEGER NOT NULL DEFAULT 0
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create v2 tables: %w", err)
	}

	_, err = s.db.Exec(`INSERT OR IGNORE INTO schema_version (version, applied_at) VALUES (2, ?)`, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to record schema version v2: %w", err)
	}

	return nil
}

// --- Existing methods (unchanged) ---

// AddInboundGroupSession stores an inbound Megolm session
func (s *KeystoreBackedStore) AddInboundGroupSession(ctx context.Context, roomID, senderKey, sessionID string, sessionKey []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	encodedKey := base64.StdEncoding.EncodeToString(sessionKey)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO inbound_group_sessions (room_id, sender_key, session_id, session_key, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(room_id, sender_key, session_id) DO UPDATE SET
			session_key = excluded.session_key,
			updated_at = CURRENT_TIMESTAMP
	`, roomID, senderKey, sessionID, encodedKey)

	if err != nil {
		return fmt.Errorf("failed to store inbound group session: %w", err)
	}

	return nil
}

// GetInboundGroupSession retrieves an inbound Megolm session
func (s *KeystoreBackedStore) GetInboundGroupSession(ctx context.Context, roomID, senderKey, sessionID string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var encodedKey string
	err := s.db.QueryRowContext(ctx, `
		SELECT session_key FROM inbound_group_sessions
		WHERE room_id = ? AND sender_key = ? AND session_id = ?
	`, roomID, senderKey, sessionID).Scan(&encodedKey)

	if err == sql.ErrNoRows {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve inbound group session: %w", err)
	}

	sessionKey, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode session key: %w", err)
	}

	return sessionKey, nil
}

// HasInboundGroupSession checks if a session exists
func (s *KeystoreBackedStore) HasInboundGroupSession(ctx context.Context, roomID, senderKey, sessionID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM inbound_group_sessions
		WHERE room_id = ? AND sender_key = ? AND session_id = ?
	`, roomID, senderKey, sessionID).Scan(&count)

	return err == nil && count > 0
}

// Clear removes all stored sessions
func (s *KeystoreBackedStore) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tables := []string{
		"inbound_group_sessions",
		"olm_accounts",
		"outbound_group_sessions",
		"device_keys",
		"olm_sessions",
		"next_batch",
		"cross_signing_keys",
		"signatures",
		"message_indices",
		"withheld_sessions",
		"tracked_users",
	}
	for _, table := range tables {
		_, err := s.db.ExecContext(ctx, "DELETE FROM "+table)
		if err != nil {
			return fmt.Errorf("failed to clear table %s: %w", table, err)
		}
	}

	return nil
}

// --- Olm Account ---

// PutOlmAccount stores or updates the Olm account for the given device
func (s *KeystoreBackedStore) PutOlmAccount(ctx context.Context, deviceID string, accountPickle []byte, shared bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sharedInt := 0
	if shared {
		sharedInt = 1
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO olm_accounts (device_id, account_pickle, shared, updated_at)
		VALUES (?, ?, ?, strftime('%s','now'))
		ON CONFLICT(device_id) DO UPDATE SET
			account_pickle = excluded.account_pickle,
			shared = excluded.shared,
			updated_at = strftime('%s','now')
	`, deviceID, accountPickle, sharedInt)

	if err != nil {
		return fmt.Errorf("failed to store olm account: %w", err)
	}
	return nil
}

// GetOlmAccount retrieves the stored Olm account
func (s *KeystoreBackedStore) GetOlmAccount(ctx context.Context) (*OlmAccountData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data OlmAccountData
	var sharedInt int
	err := s.db.QueryRowContext(ctx, `
		SELECT device_id, account_pickle, shared FROM olm_accounts LIMIT 1
	`).Scan(&data.DeviceID, &data.AccountPickle, &sharedInt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get olm account: %w", err)
	}

	data.Shared = sharedInt != 0
	return &data, nil
}

// --- Inbound Group Sessions (extended) ---

// UpdateInboundGroupSession updates an existing inbound Megolm session
func (s *KeystoreBackedStore) UpdateInboundGroupSession(ctx context.Context, roomID, senderKey, sessionID string, sessionKey []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	encodedKey := base64.StdEncoding.EncodeToString(sessionKey)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO inbound_group_sessions (room_id, sender_key, session_id, session_key, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(room_id, sender_key, session_id) DO UPDATE SET
			session_key = excluded.session_key,
			updated_at = CURRENT_TIMESTAMP
	`, roomID, senderKey, sessionID, encodedKey)

	if err != nil {
		return fmt.Errorf("failed to update inbound group session: %w", err)
	}
	return nil
}

// GetGroupSessionsForRoom returns all inbound Megolm sessions for a room
func (s *KeystoreBackedStore) GetGroupSessionsForRoom(ctx context.Context, roomID string) ([]InboundGroupSessionDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT room_id, sender_key, session_id, session_key, created_at
		FROM inbound_group_sessions
		WHERE room_id = ?
		ORDER BY created_at DESC
	`, roomID)
	if err != nil {
		return nil, fmt.Errorf("failed to query group sessions for room: %w", err)
	}
	defer rows.Close()

	var result []InboundGroupSessionDetail
	for rows.Next() {
		var detail InboundGroupSessionDetail
		var encodedKey string
		if err := rows.Scan(&detail.RoomID, &detail.SenderKey, &detail.SessionID, &encodedKey, &detail.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		detail.SessionKey, err = base64.StdEncoding.DecodeString(encodedKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decode session key: %w", err)
		}
		result = append(result, detail)
	}

	if result == nil {
		result = []InboundGroupSessionDetail{}
	}
	return result, nil
}

// --- Outbound Group Sessions ---

// PutOutboundGroupSession stores an outbound Megolm session for a room
func (s *KeystoreBackedStore) PutOutboundGroupSession(ctx context.Context, roomID, sessionID string, sessionPickle []byte, expiresAt int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO outbound_group_sessions (room_id, session_id, session_pickle, expires_at, updated_at)
		VALUES (?, ?, ?, ?, strftime('%s','now'))
		ON CONFLICT(room_id) DO UPDATE SET
			session_id = excluded.session_id,
			session_pickle = excluded.session_pickle,
			expires_at = excluded.expires_at,
			updated_at = strftime('%s','now')
	`, roomID, sessionID, sessionPickle, expiresAt)

	if err != nil {
		return fmt.Errorf("failed to store outbound group session: %w", err)
	}
	return nil
}

// GetOutboundGroupSession retrieves the outbound Megolm session for a room
func (s *KeystoreBackedStore) GetOutboundGroupSession(ctx context.Context, roomID string) (string, []byte, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sessionID string
	var sessionPickle []byte
	var expiresAt sql.NullInt64

	err := s.db.QueryRowContext(ctx, `
		SELECT session_id, session_pickle, expires_at
		FROM outbound_group_sessions
		WHERE room_id = ?
	`, roomID).Scan(&sessionID, &sessionPickle, &expiresAt)

	if err == sql.ErrNoRows {
		return "", nil, 0, nil
	}
	if err != nil {
		return "", nil, 0, fmt.Errorf("failed to get outbound group session: %w", err)
	}

	var exp int64
	if expiresAt.Valid {
		exp = expiresAt.Int64
	}
	return sessionID, sessionPickle, exp, nil
}

// RemoveOutboundGroupSession removes the outbound Megolm session for a room
func (s *KeystoreBackedStore) RemoveOutboundGroupSession(ctx context.Context, roomID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `DELETE FROM outbound_group_sessions WHERE room_id = ?`, roomID)
	if err != nil {
		return fmt.Errorf("failed to remove outbound group session: %w", err)
	}
	return nil
}

// --- Device Keys ---

// PutDeviceKeys stores device keys for a user's device
func (s *KeystoreBackedStore) PutDeviceKeys(ctx context.Context, userID, deviceID string, keyData []byte, uploadedAt int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO device_keys (user_id, device_id, key_data, uploaded_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, device_id) DO UPDATE SET
			key_data = excluded.key_data,
			uploaded_at = excluded.uploaded_at
	`, userID, deviceID, keyData, uploadedAt)

	if err != nil {
		return fmt.Errorf("failed to store device keys: %w", err)
	}
	return nil
}

// GetDeviceKeys retrieves device keys for a user's device
func (s *KeystoreBackedStore) GetDeviceKeys(ctx context.Context, userID, deviceID string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keyData []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT key_data FROM device_keys
		WHERE user_id = ? AND device_id = ?
	`, userID, deviceID).Scan(&keyData)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get device keys: %w", err)
	}
	return keyData, nil
}

// --- Cross-signing ---

func (s *KeystoreBackedStore) PutCrossSigningKey(ctx context.Context, userID, usage string, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO cross_signing_keys (user_id, usage, key_data, updated_at)
		VALUES (?, ?, ?, strftime('%s','now'))
		ON CONFLICT(user_id, usage) DO UPDATE SET
			key_data = excluded.key_data,
			updated_at = strftime('%s','now')
	`, userID, usage, key)

	if err != nil {
		return fmt.Errorf("failed to store cross-signing key: %w", err)
	}
	return nil
}

func (s *KeystoreBackedStore) GetCrossSigningKeys(ctx context.Context, userID string) ([]CrossSigningKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, usage, key_data FROM cross_signing_keys WHERE user_id = ?
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query cross-signing keys: %w", err)
	}
	defer rows.Close()

	var result []CrossSigningKey
	for rows.Next() {
		var key CrossSigningKey
		if err := rows.Scan(&key.UserID, &key.Usage, &key.KeyData); err != nil {
			return nil, fmt.Errorf("failed to scan cross-signing key: %w", err)
		}
		result = append(result, key)
	}
	if result == nil {
		result = []CrossSigningKey{}
	}
	return result, nil
}

func (s *KeystoreBackedStore) PutSignature(ctx context.Context, signedKeyID, signerUserID, signerKeyID, signature string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO signatures (signed_key_id, signer_user_id, signer_key_id, signature)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(signed_key_id, signer_user_id, signer_key_id) DO UPDATE SET
			signature = excluded.signature
	`, signedKeyID, signerUserID, signerKeyID, signature)

	if err != nil {
		return fmt.Errorf("failed to store signature: %w", err)
	}
	return nil
}

func (s *KeystoreBackedStore) GetSignatures(ctx context.Context, signedKeyID string) ([]Signature, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT signed_key_id, signer_user_id, signer_key_id, signature
		FROM signatures WHERE signed_key_id = ?
	`, signedKeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to query signatures: %w", err)
	}
	defer rows.Close()

	var result []Signature
	for rows.Next() {
		var sig Signature
		if err := rows.Scan(&sig.SignedKeyID, &sig.SignerUserID, &sig.SignerKeyID, &sig.Signature); err != nil {
			return nil, fmt.Errorf("failed to scan signature: %w", err)
		}
		result = append(result, sig)
	}
	if result == nil {
		result = []Signature{}
	}
	return result, nil
}

func (s *KeystoreBackedStore) GetMessageIndex(ctx context.Context, sessionID, messageIndex string) (*MessageIndex, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var mi MessageIndex
	err := s.db.QueryRowContext(ctx, `
		SELECT session_id, message_index, event_id, timestamp
		FROM message_indices WHERE session_id = ? AND message_index = ?
	`, sessionID, messageIndex).Scan(&mi.SessionID, &mi.MessageIndex, &mi.EventID, &mi.Timestamp)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get message index: %w", err)
	}
	return &mi, nil
}

func (s *KeystoreBackedStore) PutMessageIndex(ctx context.Context, sessionID, messageIndex string, mi MessageIndex) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO message_indices (session_id, message_index, event_id, timestamp)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(session_id, message_index) DO UPDATE SET
			event_id = excluded.event_id,
			timestamp = excluded.timestamp
	`, sessionID, messageIndex, mi.EventID, mi.Timestamp)

	if err != nil {
		return fmt.Errorf("failed to store message index: %w", err)
	}
	return nil
}

func (s *KeystoreBackedStore) GetWithheldSession(ctx context.Context, roomID, senderKey, sessionID string) (*WithheldSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ws WithheldSession
	err := s.db.QueryRowContext(ctx, `
		SELECT room_id, sender_key, session_id, code, reason
		FROM withheld_sessions WHERE room_id = ? AND sender_key = ? AND session_id = ?
	`, roomID, senderKey, sessionID).Scan(&ws.RoomID, &ws.SenderKey, &ws.SessionID, &ws.Code, &ws.Reason)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get withheld session: %w", err)
	}
	return &ws, nil
}

func (s *KeystoreBackedStore) PutWithheldSession(ctx context.Context, roomID, senderKey, sessionID string, ws WithheldSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO withheld_sessions (room_id, sender_key, session_id, code, reason)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(room_id, sender_key, session_id) DO UPDATE SET
			code = excluded.code,
			reason = excluded.reason
	`, roomID, senderKey, sessionID, ws.Code, ws.Reason)

	if err != nil {
		return fmt.Errorf("failed to store withheld session: %w", err)
	}
	return nil
}

func (s *KeystoreBackedStore) IsOutdated(ctx context.Context, userID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var outdated int
	err := s.db.QueryRowContext(ctx, `
		SELECT outdated FROM tracked_users WHERE user_id = ?
	`, userID).Scan(&outdated)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check outdated: %w", err)
	}
	return outdated != 0, nil
}

func (s *KeystoreBackedStore) MarkOutdated(ctx context.Context, userID string, outdated bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	outdatedInt := 0
	if outdated {
		outdatedInt = 1
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tracked_users (user_id, outdated) VALUES (?, ?)
		ON CONFLICT(user_id) DO UPDATE SET outdated = excluded.outdated
	`, userID, outdatedInt)

	if err != nil {
		return fmt.Errorf("failed to mark outdated: %w", err)
	}
	return nil
}

// --- Olm Sessions ---

// PutSession stores an Olm session for a sender key
func (s *KeystoreBackedStore) PutSession(ctx context.Context, senderKey, sessionID string, sessionPickle []byte, createdAt int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO olm_sessions (session_id, sender_key, session_pickle, created_at, last_used)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			session_pickle = excluded.session_pickle,
			last_used = excluded.last_used
	`, sessionID, senderKey, sessionPickle, createdAt, createdAt)

	if err != nil {
		return fmt.Errorf("failed to store olm session: %w", err)
	}
	return nil
}

// GetSession retrieves a specific Olm session by sender key and session ID
func (s *KeystoreBackedStore) GetSession(ctx context.Context, senderKey, sessionID string) (*OlmSessionData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data OlmSessionData
	err := s.db.QueryRowContext(ctx, `
		SELECT session_id, sender_key, session_pickle, created_at, last_used
		FROM olm_sessions
		WHERE sender_key = ? AND session_id = ?
	`, senderKey, sessionID).Scan(&data.SessionID, &data.SenderKey, &data.SessionPickle, &data.CreatedAt, &data.LastUsed)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get olm session: %w", err)
	}
	return &data, nil
}

// GetSessions retrieves all Olm sessions for a given sender key
func (s *KeystoreBackedStore) GetSessions(ctx context.Context, senderKey string) ([]OlmSessionData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT session_id, sender_key, session_pickle, created_at, last_used
		FROM olm_sessions
		WHERE sender_key = ?
		ORDER BY created_at DESC
	`, senderKey)
	if err != nil {
		return nil, fmt.Errorf("failed to query olm sessions: %w", err)
	}
	defer rows.Close()

	var result []OlmSessionData
	for rows.Next() {
		var data OlmSessionData
		if err := rows.Scan(&data.SessionID, &data.SenderKey, &data.SessionPickle, &data.CreatedAt, &data.LastUsed); err != nil {
			return nil, fmt.Errorf("failed to scan olm session: %w", err)
		}
		result = append(result, data)
	}

	if result == nil {
		result = []OlmSessionData{}
	}
	return result, nil
}

// --- Next batch ---

// PutNextBatch stores the next-batch sync token
func (s *KeystoreBackedStore) PutNextBatch(ctx context.Context, batchToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO next_batch (id, batch_token) VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET batch_token = excluded.batch_token
	`, batchToken)

	if err != nil {
		return fmt.Errorf("failed to store next batch: %w", err)
	}
	return nil
}

// GetNextBatch retrieves the stored next-batch sync token
func (s *KeystoreBackedStore) GetNextBatch(ctx context.Context) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var token string
	err := s.db.QueryRowContext(ctx, `SELECT batch_token FROM next_batch WHERE id = 1`).Scan(&token)

	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get next batch: %w", err)
	}
	return token, nil
}

// --- Lifecycle ---

// Flush is a no-op for SQLCipher-backed store (writes are immediate)
func (s *KeystoreBackedStore) Flush(ctx context.Context) error {
	return nil
}

// Close closes the database connection
func (s *KeystoreBackedStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// GetStats returns statistics about the crypto store
func (s *KeystoreBackedStore) GetStats(ctx context.Context) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]interface{})

	tableCounts := map[string]string{
		"session_count": "SELECT COUNT(*) FROM inbound_group_sessions",
		"room_count":    "SELECT COUNT(DISTINCT room_id) FROM inbound_group_sessions",
		"sender_count":  "SELECT COUNT(DISTINCT sender_key) FROM inbound_group_sessions",
		"olm_sessions":  "SELECT COUNT(*) FROM olm_sessions",
		"devices":       "SELECT COUNT(*) FROM device_keys",
	}

	for key, query := range tableCounts {
		var count int
		err := s.db.QueryRowContext(ctx, query).Scan(&count)
		if err != nil {
			return nil, fmt.Errorf("failed to get %s: %w", key, err)
		}
		stats[key] = count
	}

	return stats, nil
}

// DeleteSessionsForRoom removes all sessions for a specific room
func (s *KeystoreBackedStore) DeleteSessionsForRoom(ctx context.Context, roomID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `DELETE FROM inbound_group_sessions WHERE room_id = ?`, roomID)
	if err != nil {
		return fmt.Errorf("failed to delete sessions for room: %w", err)
	}

	return nil
}

// ListSessions returns all session IDs for a room
func (s *KeystoreBackedStore) ListSessions(ctx context.Context, roomID string) ([]SessionInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT session_id, sender_key, created_at
		FROM inbound_group_sessions
		WHERE room_id = ?
		ORDER BY created_at DESC
	`, roomID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []SessionInfo
	for rows.Next() {
		var info SessionInfo
		info.RoomID = roomID
		if err := rows.Scan(&info.SessionID, &info.SenderKey, &info.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, info)
	}

	return sessions, nil
}

// SessionInfo contains metadata about a stored session
type SessionInfo struct {
	RoomID    string
	SessionID string
	SenderKey string
	CreatedAt string
}

// Verify the implementation satisfies the Store interface
var _ Store = (*KeystoreBackedStore)(nil)
var _ Store = (*MemoryStore)(nil)
