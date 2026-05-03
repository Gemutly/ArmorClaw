// Package crypto provides cryptographic interfaces for E2EE support
package crypto

import (
	"context"
	"errors"
)

// Extension constraint: Task 9.5 will add cross-signing and verification methods.
// All new methods will be additive — no existing method signatures will be changed.

// OlmAccountData represents stored Olm account data for E2EE.
// Compatible with mautrix-go's crypto.OlmAccount persistence model.
type OlmAccountData struct {
	DeviceID      string
	AccountPickle []byte
	Shared        bool
}

// OlmSessionData represents stored Olm session data for E2EE.
// Compatible with mautrix-go's crypto.OlmSession persistence model.
type OlmSessionData struct {
	SessionID     string
	SenderKey     string
	SessionPickle []byte
	CreatedAt     int64
	LastUsed      int64
}

// InboundGroupSessionDetail represents a full inbound Megolm session with key data.
// Used for room-level session listing and key export.
type InboundGroupSessionDetail struct {
	RoomID     string
	SenderKey  string
	SessionID  string
	SessionKey []byte
	CreatedAt  string
}

// Store defines the interface for cryptographic key storage.
// Used by the Bridge AppService to store ingested room keys and Olm account data.
//
// The interface is designed to be compatible with mautrix-go's crypto.Store interface
// without directly importing mautrix-go. An adapter in a future task will bridge
// between this interface and mautrix-go's type-safe version.
//
// General implementation details (matching mautrix-go conventions):
//   - Get methods should not return errors if the requested data does not exist,
//     they should simply return nil/zero values.
//   - Update methods may assume that the pointer/data has been previously stored.
type Store interface {
	// AddInboundGroupSession stores an inbound Megolm session
	AddInboundGroupSession(ctx context.Context, roomID, senderKey, sessionID string, sessionKey []byte) error

	// GetInboundGroupSession retrieves an inbound Megolm session
	GetInboundGroupSession(ctx context.Context, roomID, senderKey, sessionID string) ([]byte, error)

	// HasInboundGroupSession checks if a session exists
	HasInboundGroupSession(ctx context.Context, roomID, senderKey, sessionID string) bool

	// Clear removes all stored sessions
	Clear(ctx context.Context) error

	// --- Olm Account management ---

	// PutOlmAccount stores or updates the Olm account for the given device.
	// There is typically one Olm account per device.
	PutOlmAccount(ctx context.Context, deviceID string, accountPickle []byte, shared bool) error

	// GetOlmAccount retrieves the stored Olm account.
	// Returns nil if no account exists (not an error).
	GetOlmAccount(ctx context.Context) (*OlmAccountData, error)

	// --- Inbound Group Session (extended) ---

	// UpdateInboundGroupSession updates an existing inbound Megolm session.
	// If the session does not exist, it is created.
	UpdateInboundGroupSession(ctx context.Context, roomID, senderKey, sessionID string, sessionKey []byte) error

	// GetGroupSessionsForRoom returns all inbound Megolm sessions for a room.
	// Used for key export and session forwarding.
	GetGroupSessionsForRoom(ctx context.Context, roomID string) ([]InboundGroupSessionDetail, error)

	// --- Outbound Group Session management ---

	// PutOutboundGroupSession stores an outbound Megolm session for a room.
	// There is at most one outbound session per room at a time.
	PutOutboundGroupSession(ctx context.Context, roomID, sessionID string, sessionPickle []byte, expiresAt int64) error

	// GetOutboundGroupSession retrieves the outbound Megolm session for a room.
	// Returns nil if no session exists (not an error).
	GetOutboundGroupSession(ctx context.Context, roomID string) (sessionID string, sessionPickle []byte, expiresAt int64, err error)

	// RemoveOutboundGroupSession removes the outbound Megolm session for a room.
	RemoveOutboundGroupSession(ctx context.Context, roomID string) error

	// --- Device Keys ---

	// PutDeviceKeys stores device keys for a user's device.
	// Replaces existing keys for the same (user, device) pair.
	PutDeviceKeys(ctx context.Context, userID, deviceID string, keyData []byte, uploadedAt int64) error

	// GetDeviceKeys retrieves device keys for a user's device.
	// Returns nil if not found (not an error).
	GetDeviceKeys(ctx context.Context, userID, deviceID string) ([]byte, error)

	// --- Cross-signing (stub for Task 9.5) ---

	// PutCrossSigningKey stores a cross-signing key for a user.
	// Stub implementation — will be fully implemented in Task 9.5.
	PutCrossSigningKey(ctx context.Context, userID, usage string, key string) error

	// --- Olm Session management ---

	// PutSession stores an Olm session for a sender key.
	PutSession(ctx context.Context, senderKey, sessionID string, sessionPickle []byte, createdAt int64) error

	// GetSession retrieves a specific Olm session by sender key and session ID.
	// Returns nil if not found (not an error).
	GetSession(ctx context.Context, senderKey, sessionID string) (*OlmSessionData, error)

	// GetSessions retrieves all Olm sessions for a given sender key.
	// Returns empty slice if none exist (not an error).
	GetSessions(ctx context.Context, senderKey string) ([]OlmSessionData, error)

	// --- Next batch / sync token ---

	// PutNextBatch stores the next-batch sync token for the crypto processor.
	PutNextBatch(ctx context.Context, batchToken string) error

	// GetNextBatch retrieves the stored next-batch sync token.
	// Returns empty string if not set (not an error).
	GetNextBatch(ctx context.Context) (string, error)

	GetCrossSigningKeys(ctx context.Context, userID string) ([]CrossSigningKey, error)
	PutSignature(ctx context.Context, signedKeyID, signerUserID, signerKeyID, signature string) error
	GetSignatures(ctx context.Context, signedKeyID string) ([]Signature, error)
	GetMessageIndex(ctx context.Context, sessionID, messageIndex string) (*MessageIndex, error)
	PutMessageIndex(ctx context.Context, sessionID, messageIndex string, mi MessageIndex) error
	GetWithheldSession(ctx context.Context, roomID, senderKey, sessionID string) (*WithheldSession, error)
	PutWithheldSession(ctx context.Context, roomID, senderKey, sessionID string, ws WithheldSession) error
	IsOutdated(ctx context.Context, userID string) (bool, error)
	MarkOutdated(ctx context.Context, userID string, outdated bool) error

	// --- Lifecycle ---

	// Flush ensures that everything in the store is persisted to disk.
	// This doesn't have to do anything for database-backed implementations
	// that persist everything immediately.
	Flush(ctx context.Context) error
}

// MemoryStore is an in-memory implementation of Store for testing
type MemoryStore struct {
	sessions          map[string][]byte
	olmAccount        *OlmAccountData
	outboundSessions  map[string]*outboundGroupSessionEntry
	deviceKeys        map[string][]byte
	olmSessions       map[string][]OlmSessionData
	nextBatch         string
	crossSigningKeys  map[string]string
	inboundByRoom     map[string][]InboundGroupSessionDetail
	signatures        map[string][]Signature
	messageIndices    map[string]MessageIndex
	withheldSessions  map[string]WithheldSession
	trackedUsers      map[string]bool
}

type outboundGroupSessionEntry struct {
	SessionID     string
	SessionPickle []byte
	ExpiresAt     int64
}

// NewMemoryStore creates a new in-memory crypto store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions:         make(map[string][]byte),
		outboundSessions: make(map[string]*outboundGroupSessionEntry),
		deviceKeys:       make(map[string][]byte),
		olmSessions:      make(map[string][]OlmSessionData),
		crossSigningKeys: make(map[string]string),
		inboundByRoom:    make(map[string][]InboundGroupSessionDetail),
		signatures:       make(map[string][]Signature),
		messageIndices:   make(map[string]MessageIndex),
		withheldSessions: make(map[string]WithheldSession),
		trackedUsers:     make(map[string]bool),
	}
}

func (s *MemoryStore) sessionKey(roomID, senderKey, sessionID string) string {
	return roomID + ":" + senderKey + ":" + sessionID
}

// AddInboundGroupSession stores an inbound Megolm session
func (s *MemoryStore) AddInboundGroupSession(ctx context.Context, roomID, senderKey, sessionID string, sessionKey []byte) error {
	key := s.sessionKey(roomID, senderKey, sessionID)
	s.sessions[key] = sessionKey
	s.inboundByRoom[roomID] = append(s.inboundByRoom[roomID], InboundGroupSessionDetail{
		RoomID:     roomID,
		SenderKey:  senderKey,
		SessionID:  sessionID,
		SessionKey: sessionKey,
	})
	return nil
}

// GetInboundGroupSession retrieves an inbound Megolm session
func (s *MemoryStore) GetInboundGroupSession(ctx context.Context, roomID, senderKey, sessionID string) ([]byte, error) {
	key := s.sessionKey(roomID, senderKey, sessionID)
	session, exists := s.sessions[key]
	if !exists {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

// HasInboundGroupSession checks if a session exists
func (s *MemoryStore) HasInboundGroupSession(ctx context.Context, roomID, senderKey, sessionID string) bool {
	key := s.sessionKey(roomID, senderKey, sessionID)
	_, exists := s.sessions[key]
	return exists
}

// Clear removes all stored sessions
func (s *MemoryStore) Clear(ctx context.Context) error {
	s.sessions = make(map[string][]byte)
	s.olmAccount = nil
	s.outboundSessions = make(map[string]*outboundGroupSessionEntry)
	s.deviceKeys = make(map[string][]byte)
	s.olmSessions = make(map[string][]OlmSessionData)
	s.nextBatch = ""
	s.crossSigningKeys = make(map[string]string)
	s.inboundByRoom = make(map[string][]InboundGroupSessionDetail)
	return nil
}

// PutOlmAccount stores or updates the Olm account
func (s *MemoryStore) PutOlmAccount(ctx context.Context, deviceID string, accountPickle []byte, shared bool) error {
	s.olmAccount = &OlmAccountData{
		DeviceID:      deviceID,
		AccountPickle: accountPickle,
		Shared:        shared,
	}
	return nil
}

// GetOlmAccount retrieves the stored Olm account
func (s *MemoryStore) GetOlmAccount(ctx context.Context) (*OlmAccountData, error) {
	return s.olmAccount, nil
}

// UpdateInboundGroupSession updates an existing inbound Megolm session
func (s *MemoryStore) UpdateInboundGroupSession(ctx context.Context, roomID, senderKey, sessionID string, sessionKey []byte) error {
	key := s.sessionKey(roomID, senderKey, sessionID)
	s.sessions[key] = sessionKey
	return nil
}

// GetGroupSessionsForRoom returns all inbound Megolm sessions for a room
func (s *MemoryStore) GetGroupSessionsForRoom(ctx context.Context, roomID string) ([]InboundGroupSessionDetail, error) {
	details, ok := s.inboundByRoom[roomID]
	if !ok {
		return []InboundGroupSessionDetail{}, nil
	}
	return details, nil
}

// PutOutboundGroupSession stores an outbound Megolm session for a room
func (s *MemoryStore) PutOutboundGroupSession(ctx context.Context, roomID, sessionID string, sessionPickle []byte, expiresAt int64) error {
	s.outboundSessions[roomID] = &outboundGroupSessionEntry{
		SessionID:     sessionID,
		SessionPickle: sessionPickle,
		ExpiresAt:     expiresAt,
	}
	return nil
}

// GetOutboundGroupSession retrieves the outbound Megolm session for a room
func (s *MemoryStore) GetOutboundGroupSession(ctx context.Context, roomID string) (string, []byte, int64, error) {
	entry, ok := s.outboundSessions[roomID]
	if !ok {
		return "", nil, 0, nil
	}
	return entry.SessionID, entry.SessionPickle, entry.ExpiresAt, nil
}

// RemoveOutboundGroupSession removes the outbound Megolm session for a room
func (s *MemoryStore) RemoveOutboundGroupSession(ctx context.Context, roomID string) error {
	delete(s.outboundSessions, roomID)
	return nil
}

// PutDeviceKeys stores device keys for a user's device
func (s *MemoryStore) PutDeviceKeys(ctx context.Context, userID, deviceID string, keyData []byte, uploadedAt int64) error {
	key := userID + ":" + deviceID
	s.deviceKeys[key] = keyData
	return nil
}

// GetDeviceKeys retrieves device keys for a user's device
func (s *MemoryStore) GetDeviceKeys(ctx context.Context, userID, deviceID string) ([]byte, error) {
	key := userID + ":" + deviceID
	data, ok := s.deviceKeys[key]
	if !ok {
		return nil, nil
	}
	return data, nil
}

// PutCrossSigningKey stores a cross-signing key (stub for Task 9.5)
func (s *MemoryStore) PutCrossSigningKey(ctx context.Context, userID, usage string, key string) error {
	k := userID + ":" + usage
	s.crossSigningKeys[k] = key
	return nil
}

func (s *MemoryStore) GetCrossSigningKeys(ctx context.Context, userID string) ([]CrossSigningKey, error) {
	var result []CrossSigningKey
	for k, v := range s.crossSigningKeys {
		prefix := userID + ":"
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			result = append(result, CrossSigningKey{
				UserID:  userID,
				Usage:   k[len(prefix):],
				KeyData: v,
			})
		}
	}
	if result == nil {
		result = []CrossSigningKey{}
	}
	return result, nil
}

func (s *MemoryStore) PutSignature(ctx context.Context, signedKeyID, signerUserID, signerKeyID, signature string) error {
	s.signatures[signedKeyID] = append(s.signatures[signedKeyID], Signature{
		SignedKeyID:  signedKeyID,
		SignerUserID: signerUserID,
		SignerKeyID:  signerKeyID,
		Signature:    signature,
	})
	return nil
}

func (s *MemoryStore) GetSignatures(ctx context.Context, signedKeyID string) ([]Signature, error) {
	sigs := s.signatures[signedKeyID]
	if sigs == nil {
		return []Signature{}, nil
	}
	return sigs, nil
}

func (s *MemoryStore) GetMessageIndex(ctx context.Context, sessionID, messageIndex string) (*MessageIndex, error) {
	key := sessionID + ":" + messageIndex
	mi, ok := s.messageIndices[key]
	if !ok {
		return nil, nil
	}
	return &mi, nil
}

func (s *MemoryStore) PutMessageIndex(ctx context.Context, sessionID, messageIndex string, mi MessageIndex) error {
	key := sessionID + ":" + messageIndex
	s.messageIndices[key] = mi
	return nil
}

func (s *MemoryStore) GetWithheldSession(ctx context.Context, roomID, senderKey, sessionID string) (*WithheldSession, error) {
	key := roomID + ":" + senderKey + ":" + sessionID
	ws, ok := s.withheldSessions[key]
	if !ok {
		return nil, nil
	}
	return &ws, nil
}

func (s *MemoryStore) PutWithheldSession(ctx context.Context, roomID, senderKey, sessionID string, ws WithheldSession) error {
	key := roomID + ":" + senderKey + ":" + sessionID
	s.withheldSessions[key] = ws
	return nil
}

func (s *MemoryStore) IsOutdated(ctx context.Context, userID string) (bool, error) {
	return s.trackedUsers[userID], nil
}

func (s *MemoryStore) MarkOutdated(ctx context.Context, userID string, outdated bool) error {
	s.trackedUsers[userID] = outdated
	return nil
}

// PutSession stores an Olm session for a sender key
func (s *MemoryStore) PutSession(ctx context.Context, senderKey, sessionID string, sessionPickle []byte, createdAt int64) error {
	sessions := s.olmSessions[senderKey]
	// Update if exists, otherwise append
	for i, sess := range sessions {
		if sess.SessionID == sessionID {
			sessions[i] = OlmSessionData{
				SessionID:     sessionID,
				SenderKey:     senderKey,
				SessionPickle: sessionPickle,
				CreatedAt:     createdAt,
				LastUsed:      createdAt,
			}
			return nil
		}
	}
	s.olmSessions[senderKey] = append(sessions, OlmSessionData{
		SessionID:     sessionID,
		SenderKey:     senderKey,
		SessionPickle: sessionPickle,
		CreatedAt:     createdAt,
		LastUsed:      createdAt,
	})
	return nil
}

// GetSession retrieves a specific Olm session by sender key and session ID
func (s *MemoryStore) GetSession(ctx context.Context, senderKey, sessionID string) (*OlmSessionData, error) {
	sessions, ok := s.olmSessions[senderKey]
	if !ok {
		return nil, nil
	}
	for _, sess := range sessions {
		if sess.SessionID == sessionID {
			return &sess, nil
		}
	}
	return nil, nil
}

// GetSessions retrieves all Olm sessions for a given sender key
func (s *MemoryStore) GetSessions(ctx context.Context, senderKey string) ([]OlmSessionData, error) {
	sessions, ok := s.olmSessions[senderKey]
	if !ok {
		return []OlmSessionData{}, nil
	}
	return sessions, nil
}

// PutNextBatch stores the next-batch sync token
func (s *MemoryStore) PutNextBatch(ctx context.Context, batchToken string) error {
	s.nextBatch = batchToken
	return nil
}

// GetNextBatch retrieves the stored next-batch sync token
func (s *MemoryStore) GetNextBatch(ctx context.Context) (string, error) {
	return s.nextBatch, nil
}

// Flush is a no-op for in-memory store
func (s *MemoryStore) Flush(ctx context.Context) error {
	return nil
}

// CrossSigningKey represents a stored cross-signing key for a user.
type CrossSigningKey struct {
	UserID  string
	Usage   string // "master", "self_signing", "user_signing"
	KeyData string
}

// Signature represents a key signature in the cross-signing verification chain.
type Signature struct {
	SignedKeyID  string
	SignerUserID string
	SignerKeyID  string
	Signature    string
}

// MessageIndex records the first known occurrence of a Megolm message index
// to detect replay attacks.
type MessageIndex struct {
	SessionID    string
	MessageIndex string
	EventID      string
	Timestamp    int64
}

// WithheldSession stores information about a withheld Megolm session key.
type WithheldSession struct {
	RoomID    string
	SenderKey string
	SessionID string
	Code      string
	Reason    string
}

// ErrSessionNotFound is returned when a session is not found
var ErrSessionNotFound = errors.New("session not found")
