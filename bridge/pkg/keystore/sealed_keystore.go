// Package keystore provides sealed keystore functionality for Mobile Secretary.
// The sealed keystore wraps the base keystore with an additional security layer
// that requires explicit unsealing before sensitive operations can be performed.
package keystore

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// PasswordUnsealError provides structured error information for password-gated unseal.
type PasswordUnsealError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *PasswordUnsealError) Error() string { return e.Message }

// Sealed state errors
var (
	ErrKeystoreSealed     = errors.New("keystore is sealed - unseal required")
	ErrInvalidUnsealToken = errors.New("invalid unseal token")
	ErrUnsealExpired      = errors.New("unseal token has expired")
	ErrSessionNotFound    = errors.New("session not found")
	ErrAgentNotAuthorized = errors.New("agent not authorized for this session")

	// Password policy errors
	ErrInvalidPassword = &PasswordUnsealError{Code: -32001, Message: "invalid password"}
	ErrAlreadyUnsealed = &PasswordUnsealError{Code: -32003, Message: "keystore already unsealed"}
	ErrPasswordSealed  = &PasswordUnsealError{Code: -32005, Message: "keystore is sealed"}
)

// UnsealPolicy defines how the keystore can be unsealed
type UnsealPolicy string

const (
	// PolicyMobileApproval requires approval from paired mobile device
	PolicyMobileApproval UnsealPolicy = "mobile_approval"
	// PolicyChallenge requires Ed25519 challenge-response verification
	PolicyChallenge UnsealPolicy = "challenge"
	// PolicyTimeLimited allows unseal for a limited time without approval (for testing)
	PolicyTimeLimited UnsealPolicy = "time_limited"
	// PolicyAuto allows automatic unseal (for development/testing only)
	PolicyAuto UnsealPolicy = "auto"
	// PolicyPassword requires Argon2id password verification to unseal
	PolicyPassword UnsealPolicy = "password"
)

// SealedSession represents an unsealed session with an expiration
type SealedSession struct {
	ID           string       `json:"id"`
	AgentID      string       `json:"agent_id"`
	UnsealedAt   time.Time    `json:"unsealed_at"`
	ExpiresAt    time.Time    `json:"expires_at"`
	UnsealPolicy UnsealPolicy `json:"unseal_policy"`
	ApprovedBy   string       `json:"approved_by,omitempty"` // Matrix user ID who approved
	DeviceID     string       `json:"device_id,omitempty"`   // Mobile device that approved
	Operations   int          `json:"operations"`            // Number of operations performed
	LastAccess   time.Time    `json:"last_access"`
}

// SealedStatus represents the current sealed state
type SealedStatus struct {
	IsSealed     bool   `json:"is_sealed"`
	SessionID    string `json:"session_id,omitempty"`
	AgentID      string `json:"agent_id,omitempty"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
	UnsealPolicy string `json:"unseal_policy,omitempty"`
}

// PendingUnsealRequest represents a pending unseal request awaiting approval
type PendingUnsealRequest struct {
	ID          string       `json:"id"`
	AgentID     string       `json:"agent_id"`
	RequestedAt time.Time    `json:"requested_at"`
	ExpiresAt   time.Time    `json:"expires_at"`
	Policy      UnsealPolicy `json:"policy"`
	Reason      string       `json:"reason"`
	Fields      []string     `json:"fields,omitempty"` // Fields being requested
	TaskID      string       `json:"task_id,omitempty"`
	ChallengeID string       `json:"challenge_id,omitempty"` // For challenge-response policy
}

// SealedKeystore wraps a Keystore with sealed/unsealed state management
type SealedKeystore struct {
	base         *Keystore
	mu           sync.RWMutex
	sessions     map[string]*SealedSession        // session_id -> session
	agentSession map[string]string                // agent_id -> session_id
	pending      map[string]*PendingUnsealRequest // request_id -> request
	defaultTTL   time.Duration
	policy       UnsealPolicy
	challengeMgr *ChallengeManager // Challenge manager for challenge-response policy

	// Password policy fields (protected by passwordMu, NOT mu)
	passwordMu   sync.Mutex
	vaultKey     []byte         // decrypted vault key in memory when unsealed
	passwordKD   *KeyDerivation // key derivation for password verification
	isUnsealed   bool
	unsealedAt   time.Time
	verifySalt   []byte     // salt for password verification
	verifier     []byte     // stored password verifier
	wrappedVK    WrappedKey // wrapped (encrypted) vault key

	// Auto-seal timer fields (protected by passwordMu)
	autoSealTimer    *time.Timer
	sessionExpiresAt time.Time
	lastActivityAt   time.Time
	autoSealDuration time.Duration // configurable, default 4 hours

	// Feature flag check for ZeroTrustKeystore (nil = feature off)
	featureCheck func() bool
}

// SealedKeystoreConfig holds configuration for the sealed keystore
type SealedKeystoreConfig struct {
	BaseKeystore *Keystore
	DefaultTTL   time.Duration // Default unseal duration (default: 5 minutes)
	Policy       UnsealPolicy  // Default unseal policy
}

// SealedStoreConfig holds configuration for password-gated sealed keystore.
type SealedStoreConfig struct {
	PasswordVerifier []byte    // Argon2id-derived verifier for password comparison
	VerifySalt       []byte    // Salt used to derive the verifier
	WrappedVaultKey  WrappedKey // Vault key encrypted with the user's password
}

// SessionStatusResult holds the current password session status.
type SessionStatusResult struct {
	Sealed           bool
	RemainingSeconds float64
	LastActivityAt   time.Time
}

// NewSealedKeystore creates a new sealed keystore wrapper
func NewSealedKeystore(cfg SealedKeystoreConfig) (*SealedKeystore, error) {
	if cfg.BaseKeystore == nil {
		return nil, errors.New("base keystore is required")
	}

	if cfg.DefaultTTL == 0 {
		cfg.DefaultTTL = 5 * time.Minute
	}

	if cfg.Policy == "" {
		cfg.Policy = PolicyMobileApproval
	}

	return &SealedKeystore{
		base:         cfg.BaseKeystore,
		sessions:     make(map[string]*SealedSession),
		agentSession: make(map[string]string),
		pending:      make(map[string]*PendingUnsealRequest),
		defaultTTL:   cfg.DefaultTTL,
		policy:       cfg.Policy,
	}, nil
}

// IsSealed returns true if the keystore is sealed for the given agent
func (sk *SealedKeystore) IsSealed(agentID string) bool {
	sk.mu.RLock()
	defer sk.mu.RUnlock()

	sessionID, hasSession := sk.agentSession[agentID]
	if !hasSession {
		return true
	}

	session, exists := sk.sessions[sessionID]
	if !exists {
		return true
	}

	// Check if session has expired
	if time.Now().After(session.ExpiresAt) {
		return true
	}

	return false
}

// GetStatus returns the current sealed status for an agent
func (sk *SealedKeystore) GetStatus(agentID string) SealedStatus {
	sk.mu.RLock()
	defer sk.mu.RUnlock()

	sessionID, hasSession := sk.agentSession[agentID]
	if !hasSession {
		return SealedStatus{IsSealed: true}
	}

	session, exists := sk.sessions[sessionID]
	if !exists || time.Now().After(session.ExpiresAt) {
		return SealedStatus{IsSealed: true}
	}

	return SealedStatus{
		IsSealed:     false,
		SessionID:    sessionID,
		AgentID:      session.AgentID,
		ExpiresAt:    session.ExpiresAt.Unix(),
		UnsealPolicy: string(session.UnsealPolicy),
	}
}

// SetChallengeManager sets the challenge manager for challenge-response policy
func (sk *SealedKeystore) SetChallengeManager(cm *ChallengeManager) {
	sk.mu.Lock()
	defer sk.mu.Unlock()
	sk.challengeMgr = cm
}

// GetChallengeManager returns the challenge manager
func (sk *SealedKeystore) GetChallengeManager() *ChallengeManager {
	sk.mu.RLock()
	defer sk.mu.RUnlock()
	return sk.challengeMgr
}

// RequestUnseal creates a pending unseal request
// For mobile approval policy, this creates a request that must be approved
// For time_limited policy, this auto-approves and returns the session
func (sk *SealedKeystore) RequestUnseal(ctx context.Context, agentID, reason string, fields []string, taskID string) (*PendingUnsealRequest, error) {
	sk.mu.Lock()
	defer sk.mu.Unlock()

	// Check if already unsealed
	if sessionID, hasSession := sk.agentSession[agentID]; hasSession {
		if session, exists := sk.sessions[sessionID]; exists && time.Now().Before(session.ExpiresAt) {
			// Already unsealed, return nil to indicate no request needed
			return nil, nil
		}
	}

	requestID := generateRequestID()
	now := time.Now()

	request := &PendingUnsealRequest{
		ID:          requestID,
		AgentID:     agentID,
		RequestedAt: now,
		ExpiresAt:   now.Add(60 * time.Second), // Request expires in 60 seconds
		Policy:      sk.policy,
		Reason:      reason,
		Fields:      fields,
		TaskID:      taskID,
	}

	switch sk.policy {
	case PolicyAuto:
		// Auto-approve
		_ = sk.createSessionLocked(agentID, PolicyAuto, "", "")
		sk.pending[requestID] = request
		return request, nil

	case PolicyTimeLimited:
		// Auto-approve with time limit
		_ = sk.createSessionLocked(agentID, PolicyTimeLimited, "", "")
		sk.pending[requestID] = request
		return request, nil

	case PolicyMobileApproval:
		// Store pending request for mobile approval
		sk.pending[requestID] = request
		return request, nil

	case PolicyChallenge:
		// Generate challenge for Ed25519 verification
		if sk.challengeMgr == nil {
			return nil, errors.New("challenge manager not configured for challenge policy")
		}
		challenge, err := sk.challengeMgr.GenerateChallenge(agentID, reason, fields)
		if err != nil {
			return nil, fmt.Errorf("failed to generate challenge: %w", err)
		}
		request.ChallengeID = challenge.ID
		sk.pending[requestID] = request
		return request, nil

	default:
		return nil, fmt.Errorf("unknown unseal policy: %s", sk.policy)
	}
}

// ApproveUnseal approves a pending unseal request (called from mobile device)
func (sk *SealedKeystore) ApproveUnseal(ctx context.Context, requestID, approvedBy, deviceID string) (*SealedSession, error) {
	sk.mu.Lock()
	defer sk.mu.Unlock()

	request, exists := sk.pending[requestID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	// Check if request has expired
	if time.Now().After(request.ExpiresAt) {
		delete(sk.pending, requestID)
		return nil, ErrUnsealExpired
	}

	// Create the unsealed session
	session := sk.createSessionLocked(request.AgentID, PolicyMobileApproval, approvedBy, deviceID)

	// Remove the pending request
	delete(sk.pending, requestID)

	return session, nil
}

// RejectUnseal rejects a pending unseal request
func (sk *SealedKeystore) RejectUnseal(ctx context.Context, requestID, rejectedBy string) error {
	sk.mu.Lock()
	defer sk.mu.Unlock()

	request, exists := sk.pending[requestID]
	if !exists {
		return ErrSessionNotFound
	}

	// Log rejection (in production, this would go to audit log)
	_ = rejectedBy // Use for logging
	_ = request

	delete(sk.pending, requestID)
	return nil
}

// VerifyChallengeAndUnseal verifies a challenge response and completes unseal
// This is used with PolicyChallenge - the mobile device must sign the challenge
func (sk *SealedKeystore) VerifyChallengeAndUnseal(ctx context.Context, response *ChallengeResponse) (*SealedSession, error) {
	sk.mu.Lock()
	defer sk.mu.Unlock()

	if sk.challengeMgr == nil {
		return nil, errors.New("challenge manager not configured")
	}

	// Verify the challenge response
	verified, err := sk.challengeMgr.VerifyChallenge(response)
	if err != nil {
		return nil, fmt.Errorf("challenge verification failed: %w", err)
	}

	// Find the pending request with this challenge ID
	var pendingRequest *PendingUnsealRequest
	var requestID string
	for id, req := range sk.pending {
		if req.ChallengeID == verified.ID {
			pendingRequest = req
			requestID = id
			break
		}
	}

	if pendingRequest == nil {
		return nil, ErrSessionNotFound
	}

	// Check if request has expired
	if time.Now().After(pendingRequest.ExpiresAt) {
		delete(sk.pending, requestID)
		return nil, ErrUnsealExpired
	}

	// Create the unsealed session
	session := sk.createSessionLocked(
		pendingRequest.AgentID,
		PolicyChallenge,
		"", // approvedBy - verified via signature
		response.DeviceID,
	)

	// Remove the pending request
	delete(sk.pending, requestID)

	return session, nil
}

// GetChallengeForRequest returns the challenge for a pending request
func (sk *SealedKeystore) GetChallengeForRequest(requestID string) (*Challenge, error) {
	sk.mu.RLock()
	defer sk.mu.RUnlock()

	request, exists := sk.pending[requestID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	if request.ChallengeID == "" {
		return nil, errors.New("request does not have an associated challenge")
	}

	if sk.challengeMgr == nil {
		return nil, errors.New("challenge manager not configured")
	}

	return sk.challengeMgr.GetChallenge(request.ChallengeID)
}

// GetPendingRequests returns all pending unseal requests for an agent
func (sk *SealedKeystore) GetPendingRequests(agentID string) []*PendingUnsealRequest {
	sk.mu.RLock()
	defer sk.mu.RUnlock()

	var requests []*PendingUnsealRequest
	for _, req := range sk.pending {
		if agentID == "" || req.AgentID == agentID {
			// Skip expired requests
			if time.Now().Before(req.ExpiresAt) {
				requests = append(requests, req)
			}
		}
	}
	return requests
}

// createSessionLocked creates a new unsealed session (must hold lock)
func (sk *SealedKeystore) createSessionLocked(agentID string, policy UnsealPolicy, approvedBy, deviceID string) *SealedSession {
	now := time.Now()
	sessionID := generateSessionID()

	session := &SealedSession{
		ID:           sessionID,
		AgentID:      agentID,
		UnsealedAt:   now,
		ExpiresAt:    now.Add(sk.defaultTTL),
		UnsealPolicy: policy,
		ApprovedBy:   approvedBy,
		DeviceID:     deviceID,
		Operations:   0,
		LastAccess:   now,
	}

	sk.sessions[sessionID] = session
	sk.agentSession[agentID] = sessionID

	return session
}

// ExtendSession extends the unsealed session duration
func (sk *SealedKeystore) ExtendSession(ctx context.Context, agentID string, additionalTTL time.Duration) error {
	sk.mu.Lock()
	defer sk.mu.Unlock()

	sessionID, hasSession := sk.agentSession[agentID]
	if !hasSession {
		return ErrSessionNotFound
	}

	session, exists := sk.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	// Check if session has expired
	if time.Now().After(session.ExpiresAt) {
		delete(sk.sessions, sessionID)
		delete(sk.agentSession, agentID)
		return ErrUnsealExpired
	}

	if additionalTTL == 0 {
		additionalTTL = sk.defaultTTL
	}

	session.ExpiresAt = session.ExpiresAt.Add(additionalTTL)
	session.LastAccess = time.Now()

	return nil
}

// Seal explicitly seals the keystore for an agent.
// For password policy, also zeros the vault key.
func (sk *SealedKeystore) Seal(ctx context.Context, agentID string) error {
	sk.mu.Lock()
	defer sk.mu.Unlock()

	sessionID, hasSession := sk.agentSession[agentID]
	if hasSession {
		delete(sk.sessions, sessionID)
		delete(sk.agentSession, agentID)
	}

	// Also remove any pending requests for this agent
	for reqID, req := range sk.pending {
		if req.AgentID == agentID {
			delete(sk.pending, reqID)
		}
	}

	// For password policy, seal the vault as well
	if sk.policy == PolicyPassword {
		sk.passwordMu.Lock()
		if sk.vaultKey != nil {
			ZeroBytes(sk.vaultKey)
			sk.vaultKey = nil
		}
		sk.isUnsealed = false
		sk.passwordMu.Unlock()
	}

	return nil
}

// SealAll seals all active sessions
func (sk *SealedKeystore) SealAll(ctx context.Context) error {
	sk.mu.Lock()
	defer sk.mu.Unlock()

	sk.sessions = make(map[string]*SealedSession)
	sk.agentSession = make(map[string]string)
	sk.pending = make(map[string]*PendingUnsealRequest)

	return nil
}

// RetrieveProfile retrieves a profile if the keystore is unsealed for the agent
func (sk *SealedKeystore) RetrieveProfile(ctx context.Context, agentID, profileID string) (*UserProfileData, error) {
	// Check if unsealed
	if sk.IsSealed(agentID) {
		return nil, ErrKeystoreSealed
	}

	// Update session operation count
	sk.recordOperation(agentID)

	// Delegate to base keystore
	return sk.base.RetrieveProfile(profileID)
}

// GetDefaultProfile retrieves the default profile if unsealed
func (sk *SealedKeystore) GetDefaultProfile(ctx context.Context, agentID, profileType string) (*UserProfileData, error) {
	// Check if unsealed
	if sk.IsSealed(agentID) {
		return nil, ErrKeystoreSealed
	}

	// Update session operation count
	sk.recordOperation(agentID)

	// Delegate to base keystore
	return sk.base.GetDefaultProfile(profileType)
}

// ListProfiles lists profiles (does not require unseal - only shows metadata)
func (sk *SealedKeystore) ListProfiles(ctx context.Context, profileType string) ([]UserProfileData, error) {
	// Listing profiles does not require unsealing
	return sk.base.ListProfiles(profileType)
}

// Retrieve retrieves a credential if unsealed.
// When ZeroTrustKeystore flag is on and policy is PolicyPassword, checks
// the password-gated sealed state and resets the auto-seal timer on success
// (credential read is an activity operation).
func (sk *SealedKeystore) Retrieve(ctx context.Context, agentID, keyID string) (*Credential, error) {
	if sk.IsSealed(agentID) {
		return nil, ErrKeystoreSealed
	}

	if sk.requirePasswordGate() {
		sk.passwordMu.Lock()
		if !sk.isUnsealed {
			sk.passwordMu.Unlock()
			return nil, ErrPasswordSealed
		}
		sk.passwordMu.Unlock()
	}

	sk.recordOperation(agentID)

	cred, err := sk.base.Retrieve(keyID)
	if err != nil {
		return nil, err
	}

	if sk.requirePasswordGate() {
		sk.resetActivityTimer()
	}

	return cred, nil
}

// Store stores a credential if unsealed.
// When ZeroTrustKeystore flag is on and policy is PolicyPassword, checks
// the password-gated sealed state. Store is a non-activity operation —
// it does NOT reset the auto-seal timer.
func (sk *SealedKeystore) Store(ctx context.Context, agentID string, cred Credential) error {
	if sk.IsSealed(agentID) {
		return ErrKeystoreSealed
	}

	if sk.requirePasswordGate() {
		sk.passwordMu.Lock()
		if !sk.isUnsealed {
			sk.passwordMu.Unlock()
			return ErrPasswordSealed
		}
		sk.passwordMu.Unlock()
	}

	sk.recordOperation(agentID)

	return sk.base.Store(cred)
}

// requirePasswordGate returns true when the ZeroTrustKeystore feature flag
// is enabled and the current policy is PolicyPassword.
func (sk *SealedKeystore) requirePasswordGate() bool {
	return sk.featureCheck != nil && sk.featureCheck() && sk.policy == PolicyPassword
}

// resetActivityTimer resets the auto-seal timer. Safe to call only when
// already holding no lock on passwordMu; acquires passwordMu internally.
func (sk *SealedKeystore) resetActivityTimer() {
	sk.passwordMu.Lock()
	defer sk.passwordMu.Unlock()
	if sk.isUnsealed {
		sk.resetTimerLocked()
	}
}

// SetFeatureCheck sets the closure that returns true when the
// ZeroTrustKeystore feature flag is enabled at runtime.
func (sk *SealedKeystore) SetFeatureCheck(fn func() bool) {
	sk.featureCheck = fn
}

// SealedCheckFunc returns a closure that checks sealed state.
// Returns ErrPasswordSealed (-32005) when the password vault is sealed
// and the ZeroTrustKeystore feature is enabled. Nil when no gate applies.
func (sk *SealedKeystore) SealedCheckFunc() func() error {
	return func() error {
		if !sk.requirePasswordGate() {
			return nil
		}
		sk.passwordMu.Lock()
		sealed := !sk.isUnsealed
		sk.passwordMu.Unlock()
		if sealed {
			return ErrPasswordSealed
		}
		return nil
	}
}

// ResetTimerFunc returns a closure that resets the auto-seal timer.
// Safe to hand to external components (PII scrubber, provider registry).
func (sk *SealedKeystore) ResetTimerFunc() func() {
	return func() {
		sk.resetActivityTimer()
	}
}

// recordOperation updates the operation count for a session
func (sk *SealedKeystore) recordOperation(agentID string) {
	sk.mu.Lock()
	defer sk.mu.Unlock()

	if sessionID, hasSession := sk.agentSession[agentID]; hasSession {
		if session, exists := sk.sessions[sessionID]; exists {
			session.Operations++
			session.LastAccess = time.Now()
		}
	}
}

// CleanupExpired removes expired sessions and pending requests
func (sk *SealedKeystore) CleanupExpired() int {
	sk.mu.Lock()
	defer sk.mu.Unlock()

	now := time.Now()
	count := 0

	// Clean expired sessions
	for sessionID, session := range sk.sessions {
		if now.After(session.ExpiresAt) {
			delete(sk.sessions, sessionID)
			delete(sk.agentSession, session.AgentID)
			count++
		}
	}

	// Clean expired pending requests
	for requestID, request := range sk.pending {
		if now.After(request.ExpiresAt) {
			delete(sk.pending, requestID)
			count++
		}
	}

	return count
}

// GetSession returns the session for an agent
func (sk *SealedKeystore) GetSession(agentID string) (*SealedSession, error) {
	sk.mu.RLock()
	defer sk.mu.RUnlock()

	sessionID, hasSession := sk.agentSession[agentID]
	if !hasSession {
		return nil, ErrSessionNotFound
	}

	session, exists := sk.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	// Check if expired
	if time.Now().After(session.ExpiresAt) {
		return nil, ErrUnsealExpired
	}

	// Return a copy to avoid races
	sessionCopy := *session
	return &sessionCopy, nil
}

// StartCleanupRoutine starts a background goroutine to clean up expired sessions
func (sk *SealedKeystore) StartCleanupRoutine(ctx context.Context, interval time.Duration) context.CancelFunc {
	if interval == 0 {
		interval = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sk.CleanupExpired()
			}
		}
	}()

	return cancel
}

// GetBaseKeystore returns the underlying keystore (use with caution)
func (sk *SealedKeystore) GetBaseKeystore() *Keystore {
	return sk.base
}

// SetPolicy changes the unseal policy
func (sk *SealedKeystore) SetPolicy(policy UnsealPolicy) {
	sk.mu.Lock()
	defer sk.mu.Unlock()
	sk.policy = policy
}

// GetPolicy returns the current unseal policy
func (sk *SealedKeystore) GetPolicy() UnsealPolicy {
	sk.mu.RLock()
	defer sk.mu.RUnlock()
	return sk.policy
}

// IsPasswordUnsealed returns whether the password-gated vault is currently unsealed.
func (sk *SealedKeystore) IsPasswordUnsealed() bool {
	sk.passwordMu.Lock()
	defer sk.passwordMu.Unlock()
	return sk.isUnsealed
}

// NewSealedKeystoreWithPassword creates a SealedKeystore that uses Argon2id
// password verification as its unseal policy.
func NewSealedKeystoreWithPassword(base *Keystore, cfg SealedStoreConfig) (*SealedKeystore, error) {
	if base == nil {
		return nil, errors.New("base keystore is required")
	}
	if len(cfg.PasswordVerifier) == 0 {
		return nil, errors.New("password verifier is required")
	}
	if len(cfg.VerifySalt) == 0 {
		return nil, errors.New("verify salt is required")
	}
	if len(cfg.WrappedVaultKey.Ciphertext) == 0 {
		return nil, errors.New("wrapped vault key is required")
	}

	kd, err := NewKeyDerivation(DefaultKeyDerivationParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create key derivation: %w", err)
	}

	return &SealedKeystore{
		base:             base,
		sessions:         make(map[string]*SealedSession),
		agentSession:     make(map[string]string),
		pending:          make(map[string]*PendingUnsealRequest),
		defaultTTL:       5 * time.Minute,
		policy:           PolicyPassword,
		passwordKD:       kd,
		verifySalt:       append([]byte(nil), cfg.VerifySalt...),
		verifier:         append([]byte(nil), cfg.PasswordVerifier...),
		wrappedVK:        cfg.WrappedVaultKey,
		autoSealDuration: 4 * time.Hour,
	}, nil
}

// UnsealWithPassword verifies the password via constant-time comparison of the
// Argon2id-derived verifier, then decrypts the vault key.
func (sk *SealedKeystore) UnsealWithPassword(password string) error {
	sk.passwordMu.Lock()
	defer sk.passwordMu.Unlock()

	if sk.isUnsealed {
		return ErrAlreadyUnsealed
	}

	// Derive verifier candidate from the supplied password + stored salt.
	candidate, err := sk.passwordKD.DeriveKey([]byte(password), sk.verifySalt)
	if err != nil {
		return fmt.Errorf("key derivation failed: %w", err)
	}
	candidateBytes := candidate.Key

	if subtle.ConstantTimeCompare(candidateBytes, sk.verifier) != 1 {
		ZeroBytes(candidateBytes)
		return ErrInvalidPassword
	}
	ZeroBytes(candidateBytes)

	// Decrypt the vault key using the password.
	vk, err := sk.passwordKD.UnwrapKey(&sk.wrappedVK, []byte(password))
	if err != nil {
		return ErrInvalidPassword
	}

	sk.vaultKey = vk
	sk.isUnsealed = true
	sk.unsealedAt = time.Now()
	sk.resetTimerLocked()
	return nil
}

// SealPassword zeros the in-memory vault key and marks the keystore sealed.
func (sk *SealedKeystore) SealPassword() {
	sk.passwordMu.Lock()
	defer sk.passwordMu.Unlock()

	if sk.autoSealTimer != nil {
		sk.autoSealTimer.Stop()
		sk.autoSealTimer = nil
	}

	if sk.vaultKey != nil {
		ZeroBytes(sk.vaultKey)
		sk.vaultKey = nil
	}
	sk.isUnsealed = false
}

// VaultKey returns a copy of the decrypted vault key or nil if sealed.
func (sk *SealedKeystore) VaultKey() []byte {
	sk.passwordMu.Lock()
	defer sk.passwordMu.Unlock()
	if !sk.isUnsealed || sk.vaultKey == nil {
		return nil
	}
	cp := make([]byte, len(sk.vaultKey))
	copy(cp, sk.vaultKey)
	return cp
}

// resetTimerLocked resets the auto-seal timer. Caller must hold passwordMu.
func (sk *SealedKeystore) resetTimerLocked() {
	if sk.autoSealTimer != nil {
		sk.autoSealTimer.Stop()
	}
	sk.lastActivityAt = time.Now()
	sk.sessionExpiresAt = sk.lastActivityAt.Add(sk.autoSealDuration)
	sk.autoSealTimer = time.AfterFunc(sk.autoSealDuration, sk.sealCallback)
}

// sealCallback is invoked by the auto-seal timer.
func (sk *SealedKeystore) sealCallback() {
	sk.SealPassword()
}

// SessionStatus returns the current password session status.
func (sk *SealedKeystore) SessionStatus() SessionStatusResult {
	sk.passwordMu.Lock()
	defer sk.passwordMu.Unlock()

	if !sk.isUnsealed {
		return SessionStatusResult{Sealed: true}
	}

	remaining := sk.sessionExpiresAt.Sub(time.Now()).Seconds()
	if remaining < 0 {
		remaining = 0
	}
	return SessionStatusResult{
		Sealed:           false,
		RemainingSeconds: remaining,
		LastActivityAt:   sk.lastActivityAt,
	}
}

// ExtendPasswordSession explicitly extends the auto-seal timer.
// Returns an error if the keystore is sealed.
func (sk *SealedKeystore) ExtendPasswordSession() error {
	sk.passwordMu.Lock()
	defer sk.passwordMu.Unlock()

	if !sk.isUnsealed {
		return ErrPasswordSealed
	}

	sk.resetTimerLocked()
	return nil
}

// SetAutoSealDuration configures the auto-seal timeout. For testing only.
func (sk *SealedKeystore) SetAutoSealDuration(d time.Duration) {
	sk.passwordMu.Lock()
	defer sk.passwordMu.Unlock()
	sk.autoSealDuration = d
}

// ListKeys returns the names of all stored credentials.
// Requires the password-gated vault to be unsealed.
// This is a non-activity operation — it does NOT reset the auto-seal timer.
func (sk *SealedKeystore) ListKeys(ctx context.Context) ([]string, error) {
	sk.passwordMu.Lock()
	if !sk.isUnsealed {
		sk.passwordMu.Unlock()
		return nil, ErrPasswordSealed
	}
	sk.passwordMu.Unlock()

	infos, err := sk.base.List("")
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(infos))
	for _, info := range infos {
		names = append(names, info.ID)
	}
	return names, nil
}

// DeleteKey removes a credential by name.
// Requires the password-gated vault to be unsealed.
// This is a non-activity operation — it does NOT reset the auto-seal timer.
func (sk *SealedKeystore) DeleteKey(ctx context.Context, name string) error {
	sk.passwordMu.Lock()
	if !sk.isUnsealed {
		sk.passwordMu.Unlock()
		return ErrPasswordSealed
	}
	sk.passwordMu.Unlock()

	return sk.base.Delete(name)
}

// ToMatrixEvent converts a pending unseal request to a Matrix event for mobile notification
func (r *PendingUnsealRequest) ToMatrixEvent() map[string]interface{} {
	return map[string]interface{}{
		"type":         "com.armorclaw.sealed_keystore.unseal_request",
		"request_id":   r.ID,
		"agent_id":     r.AgentID,
		"reason":       r.Reason,
		"fields":       r.Fields,
		"task_id":      r.TaskID,
		"requested_at": r.RequestedAt.UnixMilli(),
		"expires_at":   r.ExpiresAt.UnixMilli(),
	}
}

// ToMatrixEvent converts a sealed session to a Matrix event for status updates
func (s *SealedSession) ToMatrixEvent() map[string]interface{} {
	return map[string]interface{}{
		"type":        "com.armorclaw.sealed_keystore.session",
		"session_id":  s.ID,
		"agent_id":    s.AgentID,
		"unsealed_at": s.UnsealedAt.UnixMilli(),
		"expires_at":  s.ExpiresAt.UnixMilli(),
		"policy":      string(s.UnsealPolicy),
		"approved_by": s.ApprovedBy,
		"device_id":   s.DeviceID,
	}
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	b := make([]byte, 16)
	cryptorand.Read(b)
	hash := sha256.Sum256(append(b, []byte(time.Now().String())...))
	return "sess_" + base64.RawURLEncoding.EncodeToString(hash[:12])
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	b := make([]byte, 16)
	cryptorand.Read(b)
	hash := sha256.Sum256(append(b, []byte(time.Now().String())...))
	return "req_" + base64.RawURLEncoding.EncodeToString(hash[:12])
}

// MarshalJSON implements json.Marshaler for SealedSession
func (s *SealedSession) MarshalJSON() ([]byte, error) {
	type Alias SealedSession
	return json.Marshal(&struct {
		*Alias
		UnsealedAt int64 `json:"unsealed_at"`
		ExpiresAt  int64 `json:"expires_at"`
		LastAccess int64 `json:"last_access"`
	}{
		Alias:      (*Alias)(s),
		UnsealedAt: s.UnsealedAt.Unix(),
		ExpiresAt:  s.ExpiresAt.Unix(),
		LastAccess: s.LastAccess.Unix(),
	})
}
