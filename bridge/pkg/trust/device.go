// Package trust provides device trust verification for ArmorClaw.
// New devices must be verified by an admin before gaining access.
package trust

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/armorclaw/bridge/pkg/crypto"
	"github.com/armorclaw/bridge/pkg/securerandom"
)

// TrustState represents the verification state of a device
type TrustState string

const (
	// StateUnverified - Device connected but not verified
	StateUnverified TrustState = "unverified"
	// StatePendingApproval - Awaiting admin approval
	StatePendingApproval TrustState = "pending_approval"
	// StateAwaitingSecondFactor - Waiting for existing device confirmation
	StateAwaitingSecondFactor TrustState = "awaiting_second_factor"
	// StateVerified - Device is trusted
	StateVerified TrustState = "verified"
	// StateRejected - Device was rejected by admin
	StateRejected TrustState = "rejected"
	// StateExpired - Verification request expired
	StateExpired TrustState = "expired"
)

// VerificationMethod defines how devices can be verified
type VerificationMethod string

const (
	// MethodAdminApproval - Admin must manually approve
	MethodAdminApproval VerificationMethod = "admin_approval"
	// MethodSecondFactor - Existing device must confirm
	MethodSecondFactor VerificationMethod = "second_factor"
	// MethodWaitPeriod - Auto-approve after wait period
	MethodWaitPeriod VerificationMethod = "wait_period"
	// MethodAutomatic - Auto-approve (not recommended)
	MethodAutomatic VerificationMethod = "automatic"
)

// Device represents a connected device awaiting or having trust
type Device struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Fingerprint    string         `json:"fingerprint"`
	TrustState     TrustState     `json:"trust_state"`
	UserID         string         `json:"user_id"`
	UserDisplayName string        `json:"user_display_name"`
	FirstSeen      time.Time      `json:"first_seen"`
	LastSeen       time.Time      `json:"last_seen"`
	VerifiedAt     *time.Time     `json:"verified_at,omitempty"`
	VerifiedBy     string         `json:"verified_by,omitempty"`
	RejectedAt     *time.Time     `json:"rejected_at,omitempty"`
	RejectedBy     string         `json:"rejected_by,omitempty"`
	RejectionReason string        `json:"rejection_reason,omitempty"`
	ExpiresAt      *time.Time     `json:"expires_at,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// VerificationRequest represents a pending verification
type VerificationRequest struct {
	ID           string            `json:"id"`
	DeviceID     string            `json:"device_id"`
	UserID       string            `json:"user_id"`
	UserName     string            `json:"user_name"`
	DeviceName   string            `json:"device_name"`
	Method       VerificationMethod `json:"method"`
	CreatedAt    time.Time         `json:"created_at"`
	ExpiresAt    time.Time         `json:"expires_at"`
	ApprovedBy   string            `json:"approved_by,omitempty"`
	ApprovedAt   *time.Time        `json:"approved_at,omitempty"`
	RejectedBy   string            `json:"rejected_by,omitempty"`
	RejectedAt   *time.Time        `json:"rejected_at,omitempty"`
	NotifyAdmins bool              `json:"notify_admins"`
}

// TrustConfig configures the trust verification behavior
type TrustConfig struct {
	// Method is the default verification method
	Method VerificationMethod `json:"method"`

	// WaitPeriod is the auto-approval wait duration (for MethodWaitPeriod)
	WaitPeriod time.Duration `json:"wait_period"`

	// RequestExpiry is how long verification requests are valid
	RequestExpiry time.Duration `json:"request_expiry"`

	// RequireSecondFactor requires existing device confirmation
	RequireSecondFactor bool `json:"require_second_factor"`

	// NotifyAdmins sends notifications for new devices
	NotifyAdmins bool `json:"notify_admins"`

	// MaxPendingRequests limits concurrent pending requests
	MaxPendingRequests int `json:"max_pending_requests"`
}

// DefaultTrustConfig returns the recommended trust configuration
func DefaultTrustConfig() *TrustConfig {
	return &TrustConfig{
		Method:              MethodAdminApproval,
		WaitPeriod:          24 * time.Hour,
		RequestExpiry:       7 * 24 * time.Hour, // 7 days
		RequireSecondFactor: false,
		NotifyAdmins:        true,
		MaxPendingRequests:  100,
	}
}

// Manager handles device trust verification
type Manager struct {
	mu           sync.RWMutex
	config       *TrustConfig
	devices      map[string]*Device
	pending      map[string]*VerificationRequest
	admins       []string // Admin user IDs for notifications
	crossSigning *crypto.CrossSigningService
}

// NewManager creates a new trust manager.
// The optional crossSigning parameter enables cross-signing-based
// bridge verification via IsBridgeVerified.
func NewManager(config *TrustConfig, crossSigning ...*crypto.CrossSigningService) *Manager {
	if config == nil {
		config = DefaultTrustConfig()
	}
	m := &Manager{
		config:  config,
		devices: make(map[string]*Device),
		pending: make(map[string]*VerificationRequest),
		admins:  make([]string, 0),
	}
	if len(crossSigning) > 0 && crossSigning[0] != nil {
		m.crossSigning = crossSigning[0]
	}
	return m
}

// RegisterDevice registers a new device and creates verification request
func (m *Manager) RegisterDevice(userID, userName, deviceName, fingerprint string) (*Device, *VerificationRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if device already exists
	for _, device := range m.devices {
		if device.Fingerprint == fingerprint && device.UserID == userID {
			// Update last seen
			device.LastSeen = time.Now()
			return device, nil, nil
		}
	}

	// Check pending request limit
	if len(m.pending) >= m.config.MaxPendingRequests {
		return nil, nil, errors.New("too many pending verification requests")
	}

	// Create device
	deviceID := generateID("device")
	device := &Device{
		ID:              deviceID,
		Name:            deviceName,
		Fingerprint:     fingerprint,
		TrustState:      StateUnverified,
		UserID:          userID,
		UserDisplayName: userName,
		FirstSeen:       time.Now(),
		LastSeen:        time.Now(),
	}

	m.devices[deviceID] = device

	// Handle automatic approval
	if m.config.Method == MethodAutomatic {
		now := time.Now()
		device.TrustState = StateVerified
		device.VerifiedAt = &now
		device.VerifiedBy = "automatic"
		return device, nil, nil
	}

	// Handle wait period auto-approval
	if m.config.Method == MethodWaitPeriod {
		device.TrustState = StatePendingApproval
		expiresAt := time.Now().Add(m.config.WaitPeriod)
		device.ExpiresAt = &expiresAt
	}

	// Create verification request
	request := m.createVerificationRequest(device)

	return device, request, nil
}

// createVerificationRequest creates a new verification request
func (m *Manager) createVerificationRequest(device *Device) *VerificationRequest {
	requestID := generateID("verify")
	request := &VerificationRequest{
		ID:           requestID,
		DeviceID:     device.ID,
		UserID:       device.UserID,
		UserName:     device.UserDisplayName,
		DeviceName:   device.Name,
		Method:       m.config.Method,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(m.config.RequestExpiry),
		NotifyAdmins: m.config.NotifyAdmins,
	}

	device.TrustState = StatePendingApproval
	m.pending[requestID] = request

	return request
}

// ApproveDevice approves a device verification request
func (m *Manager) ApproveDevice(requestID, adminID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	request, exists := m.pending[requestID]
	if !exists {
		return errors.New("verification request not found")
	}

	if time.Now().After(request.ExpiresAt) {
		delete(m.pending, requestID)
		return errors.New("verification request expired")
	}

	device, exists := m.devices[request.DeviceID]
	if !exists {
		return errors.New("device not found")
	}

	now := time.Now()
	device.TrustState = StateVerified
	device.VerifiedAt = &now
	device.VerifiedBy = adminID
	device.ExpiresAt = nil

	request.ApprovedBy = adminID
	request.ApprovedAt = &now

	delete(m.pending, requestID)

	return nil
}

// RejectDevice rejects a device verification request
func (m *Manager) RejectDevice(requestID, adminID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	request, exists := m.pending[requestID]
	if !exists {
		return errors.New("verification request not found")
	}

	device, exists := m.devices[request.DeviceID]
	if !exists {
		return errors.New("device not found")
	}

	now := time.Now()
	device.TrustState = StateRejected
	device.RejectedAt = &now
	device.RejectedBy = adminID
	device.RejectionReason = reason

	request.RejectedBy = adminID
	request.RejectedAt = &now

	delete(m.pending, requestID)

	return nil
}

// ConfirmSecondFactor confirms a device via existing device
func (m *Manager) ConfirmSecondFactor(requestID, confirmingDeviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if method requires second factor
	if !m.config.RequireSecondFactor && m.config.Method != MethodSecondFactor {
		return errors.New("second factor not required")
	}

	request, exists := m.pending[requestID]
	if !exists {
		return errors.New("verification request not found")
	}

	// Verify confirming device is trusted
	confirmingDevice, exists := m.devices[confirmingDeviceID]
	if !exists || confirmingDevice.TrustState != StateVerified {
		return errors.New("confirming device is not trusted")
	}

	// Verify confirming device belongs to same user
	if confirmingDevice.UserID != request.UserID {
		return errors.New("confirming device belongs to different user")
	}

	// Approve the request
	device, exists := m.devices[request.DeviceID]
	if !exists {
		return errors.New("device not found")
	}

	now := time.Now()
	device.TrustState = StateVerified
	device.VerifiedAt = &now
	device.VerifiedBy = "second_factor:" + confirmingDeviceID

	request.ApprovedBy = "second_factor:" + confirmingDeviceID
	request.ApprovedAt = &now

	delete(m.pending, requestID)

	return nil
}

// GetDevice returns a device by ID
func (m *Manager) GetDevice(deviceID string) (*Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return nil, errors.New("device not found")
	}
	return device, nil
}

// GetDeviceByFingerprint returns a device by fingerprint
func (m *Manager) GetDeviceByFingerprint(fingerprint string) (*Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, device := range m.devices {
		if device.Fingerprint == fingerprint {
			return device, nil
		}
	}
	return nil, errors.New("device not found")
}

// IsDeviceTrusted checks if a device is verified
func (m *Manager) IsDeviceTrusted(deviceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return false
	}
	return device.TrustState == StateVerified
}

// IsBridgeVerified checks if the bridge device for a given user is verified
// via cross-signing. If cross-signing is bootstrapped for the user, the device
// is considered verified. If cross-signing is not configured or not yet
// bootstrapped, the device is unverified but not blocked.
func (m *Manager) IsBridgeVerified(ctx context.Context, userID string) (bool, error) {
	if m.crossSigning == nil {
		slog.Debug("cross_signing_not_configured", "user_id", userID)
		return false, nil
	}

	bootstrapped, err := m.crossSigning.IsBootstrapped(ctx, userID)
	if err != nil {
		return false, err
	}

	if bootstrapped {
		m.mu.Lock()
		defer m.mu.Unlock()
		for _, device := range m.devices {
			if device.UserID == userID && device.TrustState == StateVerified {
				return true, nil
			}
		}
	}

	return false, nil
}

// ListPendingRequests returns all pending verification requests
func (m *Manager) ListPendingRequests() []*VerificationRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	requests := make([]*VerificationRequest, 0, len(m.pending))
	for _, request := range m.pending {
		requests = append(requests, request)
	}
	return requests
}

// ListUserDevices returns all devices for a user
func (m *Manager) ListUserDevices(userID string) []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*Device, 0)
	for _, device := range m.devices {
		if device.UserID == userID {
			devices = append(devices, device)
		}
	}
	return devices
}

// ListAllDevices returns all devices
func (m *Manager) ListAllDevices() []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*Device, 0, len(m.devices))
	for _, device := range m.devices {
		devices = append(devices, device)
	}
	return devices
}

// RevokeDevice revokes trust for a device
func (m *Manager) RevokeDevice(deviceID, adminID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return errors.New("device not found")
	}

	now := time.Now()
	device.TrustState = StateRejected
	device.RejectedAt = &now
	device.RejectedBy = adminID
	device.RejectionReason = reason

	return nil
}

// AddAdmin adds an admin for notifications
func (m *Manager) AddAdmin(adminID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.admins {
		if existing == adminID {
			return
		}
	}
	m.admins = append(m.admins, adminID)
}

// GetAdmins returns all registered admins
func (m *Manager) GetAdmins() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	admins := make([]string, len(m.admins))
	copy(admins, m.admins)
	return admins
}

// CleanupExpired removes expired verification requests
func (m *Manager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	count := 0

	for id, request := range m.pending {
		if now.After(request.ExpiresAt) {
			if device, exists := m.devices[request.DeviceID]; exists {
				device.TrustState = StateExpired
			}
			delete(m.pending, id)
			count++
		}
	}

	return count
}

// SetConfig updates the trust configuration
func (m *Manager) SetConfig(config *TrustConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// ToJSON returns the trust state as JSON
func (m *Manager) ToJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return json.Marshal(map[string]interface{}{
		"config":        m.config,
		"devices_count": len(m.devices),
		"pending_count": len(m.pending),
		"admins":        m.admins,
	})
}

// Summary returns a summary of trust state
func (m *Manager) Summary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	verified := 0
	unverified := 0
	rejected := 0

	for _, device := range m.devices {
		switch device.TrustState {
		case StateVerified:
			verified++
		case StateRejected, StateExpired:
			rejected++
		default:
			unverified++
		}
	}

	return map[string]interface{}{
		"total_devices":    len(m.devices),
		"verified_devices": verified,
		"unverified_devices": unverified,
		"rejected_devices": rejected,
		"pending_requests": len(m.pending),
		"verification_method": string(m.config.Method),
	}
}

// generateID creates a unique identifier
func generateID(prefix string) string {
	return prefix + "_" + securerandom.MustID(16)
}

// VerificationNotifier interface for sending notifications
type VerificationNotifier interface {
	NotifyAdmins(request *VerificationRequest) error
	NotifyUser(userID string, state TrustState, message string) error
}

// Notify sends notifications for a verification request
func (m *Manager) Notify(notifier VerificationNotifier, request *VerificationRequest) error {
	if !request.NotifyAdmins {
		return nil
	}
	return notifier.NotifyAdmins(request)
}
