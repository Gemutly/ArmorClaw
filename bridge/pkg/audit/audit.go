package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type EventType string

const (
	EventCallCreated       EventType = "call_created"
	EventCallEnded         EventType = "call_ended"
	EventCallRejected      EventType = "call_rejected"
	EventBudgetWarning     EventType = "budget_warning"
	EventSecurityViolation EventType = "security_violation"

	// PII access events
	EventPIIAccessRequest  EventType = "pii_access_request"
	EventPIIAccessGranted  EventType = "pii_access_granted"
	EventPIIAccessRejected EventType = "pii_access_rejected"
	EventPIIAccessExpired  EventType = "pii_access_expired"
	EventPIIInjected       EventType = "pii_injected"

	// PII profile management events
	EventPIIProfileCreated EventType = "pii_profile_created"
	EventPIIProfileUpdated EventType = "pii_profile_updated"
	EventPIIProfileDeleted EventType = "pii_profile_deleted"

	// Sidecar operation events
	EventSidecarHealthCheck     EventType = "sidecar_health_check"
	EventSidecarUploadBlob      EventType = "sidecar_upload_blob"
	EventSidecarDownloadBlob    EventType = "sidecar_download_blob"
	EventSidecarListBlobs       EventType = "sidecar_list_blobs"
	EventSidecarDeleteBlob      EventType = "sidecar_delete_blob"
	EventSidecarExtractText     EventType = "sidecar_extract_text"
	EventSidecarProcessDocument EventType = "sidecar_process_document"
	EventSidecarQueued          EventType = "sidecar_queued"
	EventSidecarRetry           EventType = "sidecar_retry"

	// Keystore lifecycle events
	EventKeystoreUnseal        EventType = "keystore.unseal"
	EventKeystoreSeal          EventType = "keystore.seal"
	EventKeystoreExtendSession EventType = "keystore.extend_session"

	// Backup events
	EventBackupCreate EventType = "backup.create"
	EventBackupDelete EventType = "backup.delete"

	// Voice session events
	EventVoiceStartSession EventType = "voice.start_session"
	EventVoiceStopSession  EventType = "voice.stop_session"
)

const (
	// Governance / capability broker events
	EventCapabilityRequested  EventType = "capability_requested"
	EventCapabilityGranted    EventType = "capability_granted"
	EventCapabilityDenied     EventType = "capability_denied"
	EventCapabilityDeferred   EventType = "capability_deferred"
	EventBrokerIntercept      EventType = "broker_intercept"
	EventArtifactCreated      EventType = "artifact_created"
	EventArtifactTransformed  EventType = "artifact_transformed"
	EventArtifactLineageQuery EventType = "artifact_lineage_query"
	EventTeamCreated          EventType = "team_created"
	EventTeamDissolved        EventType = "team_dissolved"
	EventMemberAdded          EventType = "member_added"
	EventMemberRemoved        EventType = "member_removed"
	EventRoleAssigned         EventType = "role_assigned"

	// Governance mutation events (device & invite)
	EventDeviceApproved EventType = "device.approved"
	EventDeviceRejected EventType = "device.rejected"
	EventInviteCreated  EventType = "invite.created"
	EventInviteRevoked  EventType = "invite.revoked"
)

type Entry struct {
	Timestamp time.Time   `json:"timestamp"`
	EventType EventType   `json:"event_type"`
	SessionID string      `json:"session_id"`
	RoomID    string      `json:"room_id"`
	UserID    string      `json:"user_id"`
	Details   interface{} `json:"details,omitempty"`
}

type AuditLog struct {
	mu     sync.RWMutex
	path   string
	events []Entry
	maxLen int
}

type Config struct {
	Path   string
	MaxLen int
}

func DefaultConfig() Config {
	return Config{
		Path:   "/var/lib/armorclaw/audit.db",
		MaxLen: 10000,
	}
}

func NewAuditLog(cfg Config) (*AuditLog, error) {
	if cfg.MaxLen == 0 {
		cfg.MaxLen = 10000
	}

	al := &AuditLog{
		path:   cfg.Path,
		events: make([]Entry, 0, 1000),
		maxLen: cfg.MaxLen,
	}

	if err := al.loadFromFile(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load audit log: %w", err)
	}

	return al, nil
}

func (al *AuditLog) Log(entry Entry) error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	al.events = append(al.events, entry)

	if len(al.events) > al.maxLen {
		al.events = al.events[len(al.events)-al.maxLen:]
	}

	return al.saveToFile()
}

func (al *AuditLog) LogEvent(eventType EventType, sessionID, roomID, userID string, details interface{}) error {
	return al.Log(Entry{
		Timestamp: time.Now(),
		EventType: eventType,
		SessionID: sessionID,
		RoomID:    roomID,
		UserID:    userID,
		Details:   details,
	})
}

type QueryParams struct {
	Limit     int
	EventType EventType
	SessionID string
	RoomID    string
	Since     time.Time
}

func (al *AuditLog) Query(params QueryParams) ([]Entry, error) {
	al.mu.RLock()
	defer al.mu.RUnlock()

	if params.Limit <= 0 {
		params.Limit = 100
	}
	if params.Limit > 1000 {
		params.Limit = 1000
	}

	var result []Entry
	for i := len(al.events) - 1; i >= 0 && len(result) < params.Limit; i-- {
		entry := al.events[i]

		if params.EventType != "" && entry.EventType != params.EventType {
			continue
		}
		if params.SessionID != "" && entry.SessionID != params.SessionID {
			continue
		}
		if params.RoomID != "" && entry.RoomID != params.RoomID {
			continue
		}
		if !params.Since.IsZero() && entry.Timestamp.Before(params.Since) {
			continue
		}

		result = append(result, entry)
	}

	return result, nil
}

func (al *AuditLog) Count() int {
	al.mu.RLock()
	defer al.mu.RUnlock()
	return len(al.events)
}

func (al *AuditLog) Clear() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	al.events = make([]Entry, 0)
	return al.saveToFile()
}

func (al *AuditLog) loadFromFile() error {
	if al.path == "" {
		return nil
	}

	dir := filepath.Dir(al.path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	data, err := os.ReadFile(al.path)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return nil
	}

	return json.Unmarshal(data, &al.events)
}

func (al *AuditLog) saveToFile() error {
	if al.path == "" {
		return nil
	}

	dir := filepath.Dir(al.path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	data, err := json.MarshalIndent(al.events, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(al.path, data, 0640)
}

func (al *AuditLog) ExportJSON() ([]byte, error) {
	al.mu.RLock()
	defer al.mu.RUnlock()

	return json.MarshalIndent(al.events, "", "  ")
}

func (al *AuditLog) ImportJSON(data []byte) error {
	al.mu.Lock()
	defer al.mu.Unlock()

	var events []Entry
	if err := json.Unmarshal(data, &events); err != nil {
		return err
	}

	al.events = events
	return al.saveToFile()
}
