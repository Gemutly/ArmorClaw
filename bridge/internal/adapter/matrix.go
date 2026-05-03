// Package adapter provides Matrix client functionality for ArmorClaw bridge.
// This enables agents to communicate via Matrix while keeping credentials isolated.
package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/armorclaw/bridge/internal/events"
	"github.com/armorclaw/bridge/pkg/audit"
	errsys "github.com/armorclaw/bridge/pkg/errors"
	"github.com/armorclaw/bridge/pkg/logger"
	"github.com/armorclaw/bridge/pkg/pii"
	"github.com/armorclaw/bridge/pkg/trust"
)

// matrixTracker tracks component events for the Matrix adapter
var matrixTracker = errsys.GetComponentTracker("matrix")

// MatrixAdapter implements Matrix client protocol
type MatrixAdapter struct {
	homeserverURL    string
	userID           string
	password         string
	accessToken      string
	refreshToken     string
	deviceID         string
	syncToken        string
	trustedSenders   []string
	trustedRooms     []string
	rejectUntrusted  bool
	httpClient       *http.Client
	syncClient       *http.Client
	eventQueue       chan *MatrixEvent
	mu               sync.RWMutex
	tokenMutex       sync.Mutex // Prevents race conditions during token refresh
	ctx              context.Context
	cancel           context.CancelFunc
	piiScrubber      *pii.Scrubber
	lastExpiryCheck  time.Time               // P1-HIGH-1: Track last token expiry check
	commandHandler   *CommandHandler         // Command handler for admin operations
	studioCmdHandler StudioCommandHandler    // Studio command handler for Agent Factory
	trustVerifier    *TrustVerifier          // Zero-trust verification
	auditLog         *audit.TamperEvidentLog // Audit logging
	minTrustLevel    trust.TrustScore        // Minimum trust level required
	syncFilterID     string                  // Server-side sync filter ID for performance
	syncMetrics      SyncMetrics             // Sync performance metrics
	eventBus         *events.MatrixEventBus  // High-throughput event bus for agent streaming
	keyIngestion     *KeyIngestionManager    // E2EE key ingestion for to_device events
}

// StudioCommandHandler handles Agent Studio commands via Matrix
type StudioCommandHandler interface {
	HandleMatrixMessage(ctx context.Context, roomID, userID, eventID, text string) bool
}

// SyncMetrics tracks sync performance for monitoring
type SyncMetrics struct {
	mu                 sync.Mutex
	LastSyncDurationMs int64 `json:"last_sync_duration_ms"`
	EventsProcessed    int64 `json:"events_processed"`
	RoomsActive        int   `json:"rooms_active"`
	TotalSyncs         int64 `json:"total_syncs"`
}

// bridgeSyncFilter is the default sync filter for the bridge.
// Limits timeline to 10 events per room, enables lazy-load members,
// and only subscribes to event types the bridge processes.
var bridgeSyncFilter = map[string]interface{}{
	"room": map[string]interface{}{
		"timeline": map[string]interface{}{
			"limit": 10,
			"types": []string{
				"m.room.message",
				"m.room.member",
				"m.room.bridge",
				"m.room.encrypted",
				"app.armorclaw.alert",
				"com.armorclaw.agent.status",
			},
		},
		"state": map[string]interface{}{
			"lazy_load_members": true,
			"types": []string{
				"m.room.member",
				"m.room.bridge",
				"m.room.name",
				"m.room.power_levels",
			},
		},
		"ephemeral": map[string]interface{}{
			"types": []string{
				"m.typing",
				"m.receipt",
			},
		},
	},
	"presence": map[string]interface{}{
		"types": []string{},
	},
	"to_device": map[string]interface{}{
		"limit": 100,
	},
}

// MatrixEvent represents a Matrix event
type MatrixEvent struct {
	Type    string                 `json:"type"`
	RoomID  string                 `json:"room_id"`
	Sender  string                 `json:"sender"`
	Content map[string]interface{} `json:"content"`
	Origin  string                 `json:"origin_server"`
	EventID string                 `json:"event_id"`
}

// MatrixMessage is a simplified message structure
type MatrixMessage struct {
	RoomID  string `json:"room_id"`
	Message string `json:"message"`
	MsgType string `json:"msgtype"`
}

// ToDevice represents the to_device section of a /sync response
type ToDevice struct {
	Events []json.RawMessage `json:"events"`
}

// DeviceLists represents the device_lists section of a /sync response
type DeviceLists struct {
	Changed []string `json:"changed,omitempty"`
	Left    []string `json:"left,omitempty"`
}

// SyncResponse represents Matrix sync response
type SyncResponse struct {
	NextBatch             string              `json:"next_batch"`
	ToDevice              *ToDevice           `json:"to_device,omitempty"`
	DeviceLists           *DeviceLists        `json:"device_lists,omitempty"`
	DeviceOneTimeKeysCount map[string]int     `json:"device_one_time_keys_count,omitempty"`
	Rooms                 struct {
		Join map[string]struct {
			Timeline struct {
				Events []json.RawMessage `json:"events"`
			} `json:"timeline"`
		} `json:"join"`
	} `json:"rooms"`
}

// MatrixError represents a Matrix API error response
type MatrixError struct {
	ErrCode string `json:"errcode"`
	Error   string `json:"error"`
}

// Config holds Matrix adapter configuration
type Config struct {
	HomeserverURL string
	Username      string
	Password      string
	DeviceID      string
	TokenFile     string

	// Zero-trust configuration
	TrustedSenders  []string // Allowed Matrix user IDs (@user:domain.com, *@trusted.domain, *:domain)
	TrustedRooms    []string // Allowed room IDs (!roomid:domain.com)
	RejectUntrusted bool     // If true, return error to sender; if false, drop silently
	RefreshToken    string   // P1-HIGH-1: Encrypted refresh token from login

	// Trust verification settings
	MinTrustLevel trust.TrustScore // Minimum trust level required for message processing
}

// New creates a new Matrix adapter
func New(cfg Config) (*MatrixAdapter, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Set default trust level if not specified
	minTrustLevel := cfg.MinTrustLevel
	if minTrustLevel == 0 {
		minTrustLevel = trust.TrustScoreLow // Default: allow low trust and above
	}

	return &MatrixAdapter{
		homeserverURL:   cfg.HomeserverURL,
		deviceID:        cfg.DeviceID,
		password:        cfg.Password,
		trustedSenders:  cfg.TrustedSenders,
		trustedRooms:    cfg.TrustedRooms,
		rejectUntrusted: cfg.RejectUntrusted,
		refreshToken:    cfg.RefreshToken,
		minTrustLevel:   minTrustLevel,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		syncClient: &http.Client{
			Timeout: 90 * time.Second,
		},
		eventQueue:  make(chan *MatrixEvent, 100),
		ctx:         ctx,
		cancel:      cancel,
		piiScrubber: pii.New(),
	}, nil
}

// Login authenticates with the Matrix homeserver
func (m *MatrixAdapter) Login(username, password string) error {
	matrixTracker.Event("login", map[string]any{"user": username})

	payload := map[string]interface{}{
		"type": "m.login.password",
		"identifier": map[string]string{
			"type": "m.id.user",
			"user": username,
		},
		"password":  password,
		"device_id": m.deviceID,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("failed to marshal login request: %w", err)).
			WithFunction("Login").
			WithInputs(map[string]any{"user": username}).
			Build()
		matrixTracker.Failure("login", err, map[string]any{"reason": "marshal_failed"})
		return err
	}

	req, err := http.NewRequestWithContext(
		m.ctx,
		"POST",
		m.homeserverURL+"/_matrix/client/v3/login",
		bytes.NewReader(body),
	)
	if err != nil {
		err := errsys.NewBuilder("MAT-001").
			Wrap(fmt.Errorf("failed to create request: %w", err)).
			WithFunction("Login").
			WithInputs(map[string]any{"user": username, "homeserver": m.homeserverURL}).
			Build()
		matrixTracker.Failure("login", err, map[string]any{"reason": "request_create_failed"})
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		err := errsys.NewBuilder("MAT-001").
			Wrap(fmt.Errorf("login request failed: %w", err)).
			WithFunction("Login").
			WithInputs(map[string]any{"user": username, "homeserver": m.homeserverURL}).
			Build()
		matrixTracker.Failure("login", err, map[string]any{"reason": "request_failed"})
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("login failed: status %d, response: %s", resp.StatusCode, string(body))).
			WithFunction("Login").
			WithInputs(map[string]any{"user": username, "status": resp.StatusCode}).
			Build()
		matrixTracker.Failure("login", err, map[string]any{"reason": "auth_failed", "status": resp.StatusCode})
		return err
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		DeviceID     string `json:"device_id"`
		UserID       string `json:"user_id"`
		RefreshToken string `json:"refresh_token"` // P1-HIGH-1: Capture refresh token
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("failed to decode login response: %w", err)).
			WithFunction("Login").
			WithInputs(map[string]any{"user": username}).
			Build()
		matrixTracker.Failure("login", err, map[string]any{"reason": "decode_failed"})
		return err
	}

	m.mu.Lock()
	m.accessToken = result.AccessToken
	m.userID = result.UserID
	m.refreshToken = result.RefreshToken // P1-HIGH-1: Store refresh token for long-lived sessions
	if result.DeviceID != "" {
		m.deviceID = result.DeviceID
	}
	m.mu.Unlock()

	matrixTracker.Success("login", map[string]any{"user_id": result.UserID})
	return nil
}

// SendMessage sends a message to a Matrix room
func (m *MatrixAdapter) SendMessage(roomID, message, msgType string) (string, error) {
	matrixTracker.Event("send_message", map[string]any{"room_id": roomID, "msg_type": msgType})

	// P1-HIGH-1: Ensure token is valid before sending
	if err := m.ensureValidToken(); err != nil {
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("token validation failed: %w", err)).
			WithFunction("SendMessage").
			WithInputs(map[string]any{"room_id": roomID}).
			Build()
		matrixTracker.Failure("send_message", err, map[string]any{"reason": "token_validation_failed"})
		return "", err
	}

	m.mu.RLock()
	token := m.accessToken
	m.mu.RUnlock()

	if token == "" {
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("not logged in")).
			WithFunction("SendMessage").
			WithInputs(map[string]any{"room_id": roomID}).
			Build()
		matrixTracker.Failure("send_message", err, map[string]any{"reason": "not_authenticated"})
		return "", err
	}

	// Scrub PII from outgoing message
	scrubbedMessage, redactions := m.piiScrubber.Scrub(message)
	if len(redactions) > 0 {
		logger.Global().Info("PII scrubbed from outgoing message",
			"room_id", roomID,
			"redaction_count", len(redactions),
		)
	}

	payload := map[string]interface{}{
		"msgtype": msgType,
		"body":    scrubbedMessage,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		err := errsys.NewBuilder("MAT-021").
			Wrap(fmt.Errorf("failed to marshal message: %w", err)).
			WithFunction("SendMessage").
			WithInputs(map[string]any{"room_id": roomID}).
			Build()
		matrixTracker.Failure("send_message", err, map[string]any{"reason": "marshal_failed"})
		return "", err
	}

	fmt.Printf("[matrix] SendMessage: room=%s body=%s\n", roomID, string(body))

	// Generate transaction ID
	txnID := fmt.Sprintf("m%d", time.Now().UnixNano())

	u, err := url.Parse(m.homeserverURL)
	if err != nil {
		err := errsys.NewBuilder("MAT-001").
			Wrap(fmt.Errorf("failed to parse homeserver URL: %w", err)).
			WithFunction("SendMessage").
			WithInputs(map[string]any{"room_id": roomID, "homeserver": m.homeserverURL}).
			Build()
		matrixTracker.Failure("send_message", err, map[string]any{"reason": "url_parse_failed"})
		return "", err
	}

	u.Path = fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.room.message/%s",
		roomID, txnID)

	req, err := http.NewRequestWithContext(
		m.ctx,
		"PUT",
		u.String(),
		bytes.NewReader(body),
	)
	if err != nil {
		err := errsys.NewBuilder("MAT-021").
			Wrap(fmt.Errorf("failed to create request: %w", err)).
			WithFunction("SendMessage").
			WithInputs(map[string]any{"room_id": roomID}).
			Build()
		matrixTracker.Failure("send_message", err, map[string]any{"reason": "request_create_failed"})
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		err := errsys.NewBuilder("MAT-021").
			Wrap(fmt.Errorf("send request failed: %w", err)).
			WithFunction("SendMessage").
			WithInputs(map[string]any{"room_id": roomID}).
			Build()
		matrixTracker.Failure("send_message", err, map[string]any{"reason": "request_failed"})
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := errsys.NewBuilder("MAT-021").
			Wrap(fmt.Errorf("send failed: status %d, response: %s", resp.StatusCode, string(body))).
			WithFunction("SendMessage").
			WithInputs(map[string]any{"room_id": roomID, "status": resp.StatusCode}).
			Build()
		matrixTracker.Failure("send_message", err, map[string]any{"reason": "send_failed", "status": resp.StatusCode})
		return "", err
	}

	var result struct {
		EventID string `json:"event_id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		err := errsys.NewBuilder("MAT-021").
			Wrap(fmt.Errorf("failed to decode send response: %w", err)).
			WithFunction("SendMessage").
			WithInputs(map[string]any{"room_id": roomID}).
			Build()
		matrixTracker.Failure("send_message", err, map[string]any{"reason": "decode_failed"})
		return "", err
	}

	matrixTracker.Success("send_message", map[string]any{"room_id": roomID, "event_id": result.EventID})
	return result.EventID, nil
}

// Sync performs a long-poll sync with the homeserver
func (m *MatrixAdapter) Sync(timeout int) error {
	fmt.Printf("[matrix] Sync: starting sync timeout=%d\n", timeout)
	matrixTracker.Event("sync", map[string]any{"timeout": timeout})

	// P1-HIGH-1: Ensure token is valid before syncing
	if err := m.ensureValidToken(); err != nil {
		fmt.Printf("[matrix] Sync: token validation failed: %v\n", err)
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("token validation failed: %w", err)).
			WithFunction("Sync").
			WithInputs(map[string]any{"timeout": timeout}).
			Build()
		matrixTracker.Failure("sync", err, map[string]any{"reason": "token_validation_failed"})
		return err
	}

	m.mu.RLock()
	token := m.accessToken
	syncToken := m.syncToken
	m.mu.RUnlock()

	if token == "" {
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("not logged in")).
			WithFunction("Sync").
			WithInputs(map[string]any{"timeout": timeout}).
			Build()
		matrixTracker.Failure("sync", err, map[string]any{"reason": "not_authenticated"})
		return err
	}

	u, err := url.Parse(m.homeserverURL)
	if err != nil {
		err := errsys.NewBuilder("MAT-001").
			Wrap(fmt.Errorf("failed to parse homeserver URL: %w", err)).
			WithFunction("Sync").
			WithInputs(map[string]any{"timeout": timeout, "homeserver": m.homeserverURL}).
			Build()
		matrixTracker.Failure("sync", err, map[string]any{"reason": "url_parse_failed"})
		return err
	}

	syncStart := time.Now()

	u.Path = "/_matrix/client/v3/sync"
	query := url.Values{}
	query.Set("timeout", fmt.Sprintf("%d", timeout*1000))
	if syncToken != "" {
		query.Set("since", syncToken)
	}

	// Apply sync filter for performance: limits timeline, enables lazy-load,
	// filters to only event types the bridge processes
	if m.syncFilterID != "" {
		query.Set("filter", m.syncFilterID)
	} else {
		// Inline filter as JSON (fallback if server-side filter not registered)
		filterJSON, err := json.Marshal(bridgeSyncFilter)
		if err == nil {
			query.Set("filter", string(filterJSON))
		}
	}
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(
		m.ctx,
		"GET",
		u.String(),
		nil,
	)
	if err != nil {
		err := errsys.NewBuilder("MAT-003").
			Wrap(fmt.Errorf("failed to create request: %w", err)).
			WithFunction("Sync").
			WithInputs(map[string]any{"timeout": timeout}).
			Build()
		matrixTracker.Failure("sync", err, map[string]any{"reason": "request_create_failed"})
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := m.syncClient.Do(req)
	if err != nil {
		err := errsys.NewBuilder("MAT-003").
			Wrap(fmt.Errorf("sync request failed: %w", err)).
			WithFunction("Sync").
			WithInputs(map[string]any{"timeout": timeout}).
			Build()
		matrixTracker.Failure("sync", err, map[string]any{"reason": "request_failed"})
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusUnauthorized {
			m.mu.Lock()
			m.syncToken = ""
			m.mu.Unlock()
		}
		err := errsys.NewBuilder("MAT-003").
			Wrap(fmt.Errorf("sync failed: status %d, response: %s", resp.StatusCode, string(body))).
			WithFunction("Sync").
			WithInputs(map[string]any{"timeout": timeout, "status": resp.StatusCode}).
			Build()
		matrixTracker.Failure("sync", err, map[string]any{"reason": "sync_failed", "status": resp.StatusCode})
		return err
	}

	var syncResp SyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncResp); err != nil {
		err := errsys.NewBuilder("MAT-003").
			Wrap(fmt.Errorf("failed to decode sync response: %w", err)).
			WithFunction("Sync").
			WithInputs(map[string]any{"timeout": timeout}).
			Build()
		matrixTracker.Failure("sync", err, map[string]any{"reason": "decode_failed"})
		return err
	}

	fmt.Printf("[matrix] Sync: decoded response next_batch=%s rooms_count=%d\n", syncResp.NextBatch, len(syncResp.Rooms.Join))

	// Process events and queue them
	eventsProcessed := m.processEvents(&syncResp)

	// Update sync token
	m.mu.Lock()
	m.syncToken = syncResp.NextBatch
	m.lastExpiryCheck = time.Now() // Update expiry check on successful sync
	m.mu.Unlock()

	// Track sync performance metrics
	syncDuration := time.Since(syncStart)
	m.syncMetrics.mu.Lock()
	m.syncMetrics.LastSyncDurationMs = syncDuration.Milliseconds()
	m.syncMetrics.EventsProcessed += int64(eventsProcessed)
	m.syncMetrics.RoomsActive = len(syncResp.Rooms.Join)
	m.syncMetrics.TotalSyncs++
	m.syncMetrics.mu.Unlock()

	matrixTracker.Success("sync", map[string]any{
		"next_batch":       syncResp.NextBatch,
		"duration_ms":      syncDuration.Milliseconds(),
		"events_processed": eventsProcessed,
		"rooms_active":     len(syncResp.Rooms.Join),
	})
	fmt.Printf("[matrix] Sync: complete next_batch=%s events=%d rooms=%d\n", syncResp.NextBatch, eventsProcessed, len(syncResp.Rooms.Join))
	return nil
}

// processEvents processes events from sync response and returns the count of events processed
func (m *MatrixAdapter) processEvents(syncResp *SyncResponse) int {
	processed := 0
	for roomID, room := range syncResp.Rooms.Join {
		eventCount := len(room.Timeline.Events)
		if eventCount > 0 {
			fmt.Printf("[matrix] processEvents: room=%s event_count=%d\n", roomID, eventCount)
		}
		for _, rawEvent := range room.Timeline.Events {
			processed++
			var event MatrixEvent
			if err := json.Unmarshal(rawEvent, &event); err != nil {
				fmt.Printf("[matrix] processEvents: unmarshal error: %v\n", err)
				continue
			}

			// Only queue message events
			if event.Type == "m.room.message" {
				// Validate sender (zero-trust)
				if !m.isTrustedSender(event.Sender) {
					// Log security event
					secLog := logger.NewSecurityLogger(logger.Global().WithComponent("matrix"))
					secLog.LogAuthRejected(context.Background(), event.Sender, "sender_not_in_allowlist",
						logger.LogAttr("room_id", roomID),
					)

					// Log to audit if configured
					m.logTrustEvent("sender_rejected", event.Sender, roomID, map[string]interface{}{
						"reason": "sender_not_in_allowlist",
					})

					// Optionally send rejection back to sender
					if m.rejectUntrusted {
						m.sendRejectionMessage(roomID, event.Sender)
					}
					continue
				}

				// Validate room
				if !m.isTrustedRoom(roomID) {
					secLog := logger.NewSecurityLogger(logger.Global().WithComponent("matrix"))
					secLog.LogAccessDenied(context.Background(), "matrix_message", event.Sender, "room_not_in_allowlist",
						logger.LogAttr("room_id", roomID),
						logger.LogAttr("event_id", event.EventID),
					)

					// Log to audit if configured
					m.logTrustEvent("room_rejected", event.Sender, roomID, map[string]interface{}{
						"reason":   "room_not_in_allowlist",
						"event_id": event.EventID,
					})
					continue
				}

				// Perform zero-trust verification if configured
				m.mu.RLock()
				trustVerifier := m.trustVerifier
				minTrustLevel := m.minTrustLevel
				m.mu.RUnlock()

				if trustVerifier != nil {
					// Create device fingerprint from event context
					fingerprint := DeviceFingerprintFromMatrix(event.Sender, "", "matrix")

					// Verify the event
					result, err := trustVerifier.VerifyEvent(context.Background(), &event, fingerprint, "")
					if err != nil {
						logger.Global().Error("trust verification error",
							"error", err,
							"event_id", event.EventID,
							"sender", event.Sender,
						)
						continue
					}

					// Enforce trust level
					enforcement := trustVerifier.ShouldEnforceTrust(result, minTrustLevel)
					if !enforcement.Allowed {
						logger.Global().Warn("event blocked by trust enforcement",
							"event_id", event.EventID,
							"sender", event.Sender,
							"trust_level", enforcement.TrustLevel.String(),
							"min_required", minTrustLevel.String(),
							"risk_score", enforcement.RiskScore,
							"anomaly_flags", enforcement.AnomalyFlags,
						)

						// Log to audit if configured
						m.logTrustEvent("trust_blocked", event.Sender, roomID, map[string]interface{}{
							"trust_level":   enforcement.TrustLevel.String(),
							"risk_score":    enforcement.RiskScore,
							"anomaly_flags": enforcement.AnomalyFlags,
							"message":       enforcement.Message,
						})

						// Send rejection if configured
						if m.rejectUntrusted {
							m.sendTrustRejectionMessage(roomID, event.Sender, enforcement)
						}
						continue
					}

					// Log successful verification with anomalies if any
					if len(enforcement.AnomalyFlags) > 0 {
						logger.Global().Info("event passed with anomalies",
							"event_id", event.EventID,
							"sender", event.Sender,
							"trust_level", enforcement.TrustLevel.String(),
							"anomaly_flags", enforcement.AnomalyFlags,
						)
					}
				}

				// Scrub PII from message content
				event.Content = m.piiScrubber.ScrubMap(event.Content)

				// Log redaction summary if any PII was found
				if body, ok := event.Content["body"].(string); ok {
					scrubbed, redactions := m.piiScrubber.Scrub(body)
					if len(redactions) > 0 {
						event.Content["body"] = scrubbed
						logger.Global().Info("PII scrubbed from message",
							"sender", event.Sender,
							"room_id", roomID,
							"redaction_count", len(redactions),
						)
					}
				}

				event.RoomID = roomID

				// Check for studio commands before queuing
				m.mu.RLock()
				studioHandler := m.studioCmdHandler
				m.mu.RUnlock()

				if studioHandler != nil {
					if body, ok := event.Content["body"].(string); ok {
						fmt.Printf("[matrix] calling studioHandler: room=%s sender=%s body=%s\n", roomID, event.Sender, body)
						if studioHandler.HandleMatrixMessage(context.Background(), roomID, event.Sender, event.EventID, body) {
							fmt.Printf("[matrix] event handled by studioHandler: event_id=%s\n", event.EventID)
							continue
						}
					}
				} else {
					fmt.Printf("[matrix] WARNING: studioHandler is nil\n")
				}

				select {
				case m.eventQueue <- &event:
				default:
					// Queue full, drop event
				}

				// Publish to high-throughput event bus if configured
				m.mu.RLock()
				bus := m.eventBus
				m.mu.RUnlock()

				if bus != nil {
					bus.Publish(events.MatrixEvent{
						ID:      event.EventID,
						RoomID:  roomID,
						Sender:  event.Sender,
						Type:    event.Type,
						Content: event.Content,
					})
				}
			}

			// Handle custom ArmorClaw event types (workflow.*, agent.*, blocker.*)
			switch {
			case strings.HasPrefix(event.Type, "workflow."):
				// workflow.progress, workflow.step_progress, workflow.step_error,
				// workflow.blocker_warning, workflow.blocked, workflow.timeline
				fmt.Printf("[matrix] processEvents: workflow event type=%s room=%s sender=%s\n",
					event.Type, roomID, event.Sender)
				m.publishCustomEvent(&event, roomID)

			case strings.HasPrefix(event.Type, "agent."):
				// agent.comment, etc.
				fmt.Printf("[matrix] processEvents: agent event type=%s room=%s sender=%s\n",
					event.Type, roomID, event.Sender)
				m.publishCustomEvent(&event, roomID)

			case strings.HasPrefix(event.Type, "blocker."):
				// blocker.required, etc.
				fmt.Printf("[matrix] processEvents: blocker event type=%s room=%s sender=%s\n",
					event.Type, roomID, event.Sender)
				m.publishCustomEvent(&event, roomID)

			case strings.HasPrefix(event.Type, "com.armorclaw."):
				// com.armorclaw.agent.status, com.armorclaw.agent.*, etc.
				fmt.Printf("[matrix] processEvents: armorclaw event type=%s room=%s sender=%s\n",
					event.Type, roomID, event.Sender)
				m.publishCustomEvent(&event, roomID)

			default:
				// Unknown custom event types - log at debug but don't silently drop
				if strings.Contains(event.Type, ".") && !strings.HasPrefix(event.Type, "m.") {
					fmt.Printf("[matrix] processEvents: unrecognized custom event type=%s room=%s\n",
						event.Type, roomID)
				}
			}
		}
	}

	if syncResp.ToDevice != nil {
		m.processToDeviceEvents(syncResp.ToDevice.Events)
	}

	return processed
}

func (m *MatrixAdapter) processToDeviceEvents(events []json.RawMessage) {
	m.mu.RLock()
	ki := m.keyIngestion
	m.mu.RUnlock()

	if ki == nil {
		return
	}

	for _, rawEvent := range events {
		var toDeviceEvent struct {
			Type    string          `json:"type"`
			Sender  string          `json:"sender"`
			Content json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(rawEvent, &toDeviceEvent); err != nil {
			fmt.Printf("[matrix] processToDeviceEvents: unmarshal error: %v\n", err)
			continue
		}

		if err := ki.HandleKeyEvent(m.ctx, toDeviceEvent.Type, toDeviceEvent.Content, toDeviceEvent.Sender); err != nil {
			fmt.Printf("[matrix] processToDeviceEvents: HandleKeyEvent error: type=%s err=%v\n", toDeviceEvent.Type, err)
		}
	}
}

func (m *MatrixAdapter) publishCustomEvent(event *MatrixEvent, roomID string) {
	m.mu.RLock()
	bus := m.eventBus
	m.mu.RUnlock()

	if bus != nil {
		bus.Publish(events.MatrixEvent{
			ID:      event.EventID,
			RoomID:  roomID,
			Sender:  event.Sender,
			Type:    event.Type,
			Content: event.Content,
		})
	}
}

// GetSyncMetrics returns current sync performance metrics
func (m *MatrixAdapter) GetSyncMetrics() SyncMetrics {
	m.syncMetrics.mu.Lock()
	defer m.syncMetrics.mu.Unlock()
	return SyncMetrics{
		LastSyncDurationMs: m.syncMetrics.LastSyncDurationMs,
		EventsProcessed:    m.syncMetrics.EventsProcessed,
		RoomsActive:        m.syncMetrics.RoomsActive,
		TotalSyncs:         m.syncMetrics.TotalSyncs,
	}
}

// logTrustEvent logs a trust-related event to the audit log
func (m *MatrixAdapter) logTrustEvent(eventType, sender, roomID string, details map[string]interface{}) {
	m.mu.RLock()
	auditLog := m.auditLog
	m.mu.RUnlock()

	if auditLog == nil {
		return
	}

	actor := audit.Actor{
		Type: "matrix_user",
		ID:   sender,
	}
	resource := audit.Resource{
		Type: "matrix_room",
		ID:   roomID,
	}
	compliance := audit.ComplianceFlags{
		Category:      "trust_verification",
		Severity:      "medium",
		AuditRequired: true,
	}

	_, _ = auditLog.LogEntry(eventType, actor, "verify", resource, details, compliance)
}

// sendTrustRejectionMessage sends a trust-related rejection message
func (m *MatrixAdapter) sendTrustRejectionMessage(roomID, sender string, enforcement TrustEnforcementResult) {
	msg := "🔒 **Message Rejected - Trust Verification Failed**\n\n"
	msg += "Your message was not processed due to trust verification requirements.\n\n"
	msg += fmt.Sprintf("**Trust Level:** %s (Required: %s)\n", enforcement.TrustLevel.String(), m.minTrustLevel.String())
	msg += fmt.Sprintf("**Risk Score:** %d/100\n", enforcement.RiskScore)

	if len(enforcement.AnomalyFlags) > 0 {
		msg += "**Anomalies Detected:**\n"
		for _, flag := range enforcement.AnomalyFlags {
			msg += fmt.Sprintf("- %s\n", flag)
		}
	}

	if len(enforcement.RequiredActions) > 0 {
		msg += "\n**Required Actions:**\n"
		for _, action := range enforcement.RequiredActions {
			msg += fmt.Sprintf("- %s\n", action)
		}
	}

	m.SendMessageWithRetry(roomID, msg, "m.notice")
}

// sendRejectionMessage sends a rejection message back to the sender
func (m *MatrixAdapter) sendRejectionMessage(roomID, sender string) {
	// Send rejection notice
	rejectionMsg := "🔒 **Message Rejected**\n\nYour message was not processed because you are not on the trusted senders list for this ArmorClaw instance."
	m.SendMessageWithRetry(roomID, rejectionMsg, "m.notice")
}

// ReceiveEvents returns the event channel for receiving messages
func (m *MatrixAdapter) ReceiveEvents() <-chan *MatrixEvent {
	return m.eventQueue
}

// StartSync begins the background sync loop
func (m *MatrixAdapter) StartSync() {
	go m.syncLoop()
}

// syncLoop runs the continuous sync loop
func (m *MatrixAdapter) syncLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.Sync(30); err != nil {
				// Log error but continue
			}
		}
	}
}

// GetUserID returns the current user ID
func (m *MatrixAdapter) GetUserID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.userID
}

// GetAccessToken returns the current access token
func (m *MatrixAdapter) GetAccessToken() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.accessToken
}

// isTrustedSender checks if a sender is in the trusted list
func (m *MatrixAdapter) isTrustedSender(sender string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// If no allowlist configured, allow all (backward compatible)
	if len(m.trustedSenders) == 0 {
		return true
	}

	// Check against allowlist
	for _, pattern := range m.trustedSenders {
		if m.matchSenderPattern(sender, pattern) {
			return true
		}
	}
	return false
}

// matchSenderPattern supports wildcards for sender matching
func (m *MatrixAdapter) matchSenderPattern(sender, pattern string) bool {
	// Exact match
	if sender == pattern {
		return true
	}

	// Wildcard: *@domain.com - any user from domain
	if len(pattern) > 1 && pattern[0] == '*' && pattern[1] == '@' {
		domain := pattern[2:]
		if len(sender) > len(domain)+1 {
			atIndex := len(sender) - len(domain)
			if sender[atIndex:] == domain && sender[atIndex-1] == '@' {
				return true
			}
		}
	}

	// Wildcard: *:domain.com - any user from domain (alternative format)
	if len(pattern) > 1 && pattern[0] == '*' && pattern[1] == ':' {
		domain := pattern[2:]
		if len(sender) > len(domain)+1 {
			atIndex := len(sender) - len(domain)
			if sender[atIndex:] == domain && sender[atIndex-1] == ':' {
				return true
			}
		}
	}

	return false
}

// isTrustedRoom checks if a room is in the trusted list
func (m *MatrixAdapter) isTrustedRoom(roomID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// If no allowlist configured, allow all rooms
	if len(m.trustedRooms) == 0 {
		return true
	}

	for _, trusted := range m.trustedRooms {
		if roomID == trusted {
			return true
		}
	}
	return false
}

// SetTrustedSenders sets the list of trusted senders
func (m *MatrixAdapter) SetTrustedSenders(senders []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trustedSenders = senders
}

// SetTrustedRooms sets the list of trusted rooms
func (m *MatrixAdapter) SetTrustedRooms(rooms []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trustedRooms = rooms
}

// GetTrustedSenders returns the list of trusted senders
func (m *MatrixAdapter) GetTrustedSenders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string{}, m.trustedSenders...)
}

// GetTrustedRooms returns the list of trusted rooms
func (m *MatrixAdapter) GetTrustedRooms() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string{}, m.trustedRooms...)
}

// GetRoomMembers retrieves the list of members in a Matrix room
// This implements the errsys.MatrixAdminAdapter interface
func (m *MatrixAdapter) GetRoomMembers(ctx context.Context, roomID string) ([]errsys.RoomMember, error) {
	matrixTracker.Event("get_room_members", map[string]any{"room_id": roomID})

	// P1-HIGH-1: Ensure token is valid before API call
	if err := m.ensureValidToken(); err != nil {
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("token validation failed: %w", err)).
			WithFunction("GetRoomMembers").
			WithInputs(map[string]any{"room_id": roomID}).
			Build()
		matrixTracker.Failure("get_room_members", err, map[string]any{"reason": "token_validation_failed"})
		return nil, err
	}

	m.mu.RLock()
	accessToken := m.accessToken
	m.mu.RUnlock()

	// Build URL for room members API
	url := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/members", m.homeserverURL, roomID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		err := errsys.NewBuilder("MAT-020").
			Wrap(fmt.Errorf("failed to create request: %w", err)).
			WithFunction("GetRoomMembers").
			WithInputs(map[string]any{"room_id": roomID}).
			Build()
		matrixTracker.Failure("get_room_members", err, map[string]any{"reason": "request_create_failed"})
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		err := errsys.NewBuilder("MAT-001").
			Wrap(fmt.Errorf("failed to get room members: %w", err)).
			WithFunction("GetRoomMembers").
			WithInputs(map[string]any{"room_id": roomID}).
			Build()
		matrixTracker.Failure("get_room_members", err, map[string]any{"reason": "request_failed"})
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := errsys.NewBuilder("MAT-020").
			Wrap(fmt.Errorf("failed to get room members: %s - %s", resp.Status, string(body))).
			WithFunction("GetRoomMembers").
			WithInputs(map[string]any{"room_id": roomID, "status": resp.StatusCode}).
			Build()
		matrixTracker.Failure("get_room_members", err, map[string]any{"reason": "api_failed", "status": resp.StatusCode})
		return nil, err
	}

	// Parse response
	var membersResp struct {
		Chunk []struct {
			Content struct {
				Membership  string `json:"membership"`
				Displayname string `json:"displayname"`
			} `json:"content"`
			Sender   string `json:"sender"`
			StateKey string `json:"state_key"`
		} `json:"chunk"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&membersResp); err != nil {
		err := errsys.NewBuilder("MAT-020").
			Wrap(fmt.Errorf("failed to parse room members response: %w", err)).
			WithFunction("GetRoomMembers").
			WithInputs(map[string]any{"room_id": roomID}).
			Build()
		matrixTracker.Failure("get_room_members", err, map[string]any{"reason": "decode_failed"})
		return nil, err
	}

	// Convert to errsys.RoomMember slice
	var members []errsys.RoomMember
	for _, event := range membersResp.Chunk {
		if event.Content.Membership == "join" {
			members = append(members, errsys.RoomMember{
				UserID:     event.StateKey,
				PowerLevel: 0, // Default power level, would need separate state event for actual value
				Display:    event.Content.Displayname,
			})
		}
	}

	matrixTracker.Success("get_room_members", map[string]any{"room_id": roomID, "member_count": len(members)})
	return members, nil
}

// Close closes the adapter and stops sync
func (m *MatrixAdapter) Close() error {
	m.cancel()
	return nil
}

// isRetryableHTTPError checks if an HTTP error is retryable
func isRetryableHTTPError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context-related errors
	if containsAny(err.Error(), "context deadline exceeded", "context canceled") {
		return true
	}

	// Check for network-related errors
	if containsAny(err.Error(), "connection refused", "connection reset",
		"connection timed out", "temporary failure", "network is unreachable") {
		return true
	}

	return false
}

// isRetryableStatusCode checks if an HTTP status code is retryable
func isRetryableStatusCode(statusCode int) bool {
	// 5xx server errors are typically retryable
	return statusCode >= 500 && statusCode < 600
}

// SendMessageWithRetry sends a message with retry logic for transient failures
func (m *MatrixAdapter) SendMessageWithRetry(roomID, message, msgType string) (string, error) {
	const maxAttempts = 3
	const baseDelay = 1 * time.Second

	matrixTracker.Event("send_message_retry", map[string]any{"room_id": roomID, "max_attempts": maxAttempts})

	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		eventID, err := m.SendMessage(roomID, message, msgType)
		if err == nil {
			return eventID, nil
		}

		// Check if error is retryable
		if !isRetryableHTTPError(err) {
			return "", err // Non-retryable error
		}

		lastErr = err

		// Don't wait after the last attempt
		if attempt < maxAttempts-1 {
			backoff := baseDelay * time.Duration(1<<uint(attempt))

			select {
			case <-time.After(backoff):
			case <-m.ctx.Done():
				err := errsys.NewBuilder("MAT-021").
					Wrap(m.ctx.Err()).
					WithFunction("SendMessageWithRetry").
					WithInputs(map[string]any{"room_id": roomID, "attempt": attempt}).
					Build()
				matrixTracker.Failure("send_message_retry", err, map[string]any{"reason": "context_canceled"})
				return "", err
			}
		}
	}

	err := errsys.NewBuilder("MAT-021").
		Wrap(fmt.Errorf("send message failed after %d attempts: %w", maxAttempts, lastErr)).
		WithFunction("SendMessageWithRetry").
		WithInputs(map[string]any{"room_id": roomID, "attempts": maxAttempts}).
		Build()
	matrixTracker.Failure("send_message_retry", err, map[string]any{"reason": "max_retries_exceeded"})
	return "", err
}

// SendEvent sends a custom event type to a Matrix room
// This is used for sending non-message events like call signaling (m.call.invite, etc.)
func (m *MatrixAdapter) SendEvent(roomID, eventType string, content []byte) error {
	matrixTracker.Event("send_event", map[string]any{"room_id": roomID, "event_type": eventType})

	// P1-HIGH-1: Ensure token is valid before sending
	if err := m.ensureValidToken(); err != nil {
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("token validation failed: %w", err)).
			WithFunction("SendEvent").
			WithInputs(map[string]any{"room_id": roomID, "event_type": eventType}).
			Build()
		matrixTracker.Failure("send_event", err, map[string]any{"reason": "token_validation_failed"})
		return err
	}

	m.mu.RLock()
	token := m.accessToken
	m.mu.RUnlock()

	if token == "" {
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("not logged in")).
			WithFunction("SendEvent").
			WithInputs(map[string]any{"room_id": roomID, "event_type": eventType}).
			Build()
		matrixTracker.Failure("send_event", err, map[string]any{"reason": "not_authenticated"})
		return err
	}

	// Generate transaction ID
	txnID := fmt.Sprintf("m%d", time.Now().UnixNano())

	u, err := url.Parse(m.homeserverURL)
	if err != nil {
		err := errsys.NewBuilder("MAT-001").
			Wrap(fmt.Errorf("failed to parse homeserver URL: %w", err)).
			WithFunction("SendEvent").
			WithInputs(map[string]any{"room_id": roomID, "event_type": eventType, "homeserver": m.homeserverURL}).
			Build()
		matrixTracker.Failure("send_event", err, map[string]any{"reason": "url_parse_failed"})
		return err
	}

	u.Path = fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/%s/%s",
		url.PathEscape(roomID), url.PathEscape(eventType), txnID)

	req, err := http.NewRequestWithContext(
		m.ctx,
		"PUT",
		u.String(),
		bytes.NewReader(content),
	)
	if err != nil {
		err := errsys.NewBuilder("MAT-021").
			Wrap(fmt.Errorf("failed to create request: %w", err)).
			WithFunction("SendEvent").
			WithInputs(map[string]any{"room_id": roomID, "event_type": eventType}).
			Build()
		matrixTracker.Failure("send_event", err, map[string]any{"reason": "request_create_failed"})
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		err := errsys.NewBuilder("MAT-021").
			Wrap(fmt.Errorf("send request failed: %w", err)).
			WithFunction("SendEvent").
			WithInputs(map[string]any{"room_id": roomID, "event_type": eventType}).
			Build()
		matrixTracker.Failure("send_event", err, map[string]any{"reason": "request_failed"})
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := errsys.NewBuilder("MAT-021").
			Wrap(fmt.Errorf("send failed: status %d, response: %s", resp.StatusCode, string(body))).
			WithFunction("SendEvent").
			WithInputs(map[string]any{"room_id": roomID, "event_type": eventType, "status": resp.StatusCode}).
			Build()
		matrixTracker.Failure("send_event", err, map[string]any{"reason": "send_failed", "status": resp.StatusCode})
		return err
	}

	matrixTracker.Success("send_event", map[string]any{"room_id": roomID, "event_type": eventType})
	return nil
}

// SyncWithRetry performs sync with retry logic
func (m *MatrixAdapter) SyncWithRetry(timeout int) error {
	const maxAttempts = 3
	const baseDelay = 1 * time.Second

	matrixTracker.Event("sync_retry", map[string]any{"timeout": timeout, "max_attempts": maxAttempts})

	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		err := m.Sync(timeout)
		if err == nil {
			return nil
		}

		// Check if error is retryable
		if !isRetryableHTTPError(err) {
			return err // Non-retryable error
		}

		lastErr = err

		// Don't wait after the last attempt
		if attempt < maxAttempts-1 {
			backoff := baseDelay * time.Duration(1<<uint(attempt))

			select {
			case <-time.After(backoff):
			case <-m.ctx.Done():
				err := errsys.NewBuilder("MAT-003").
					Wrap(m.ctx.Err()).
					WithFunction("SyncWithRetry").
					WithInputs(map[string]any{"timeout": timeout, "attempt": attempt}).
					Build()
				matrixTracker.Failure("sync_retry", err, map[string]any{"reason": "context_canceled"})
				return err
			}
		}
	}

	err := errsys.NewBuilder("MAT-003").
		Wrap(fmt.Errorf("sync failed after %d attempts: %w", maxAttempts, lastErr)).
		WithFunction("SyncWithRetry").
		WithInputs(map[string]any{"timeout": timeout, "attempts": maxAttempts}).
		Build()
	matrixTracker.Failure("sync_retry", err, map[string]any{"reason": "max_retries_exceeded"})
	return err
}

// isTokenExpired checks if the access token might be expired
// P1-HIGH-1: Tracks token expiry using lastExpiryCheck timestamp
// Matrix access tokens expire after 7 days of inactivity
func (m *MatrixAdapter) isTokenExpired() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// No token at all means not logged in
	if m.accessToken == "" {
		return true
	}

	// P1-HIGH-1: Check if we have a refresh token to support long-lived sessions
	if m.refreshToken != "" {
		// With refresh token support, check if 7 days have passed since last check
		if m.lastExpiryCheck.IsZero() {
			// First check - if we have a sync token, we've successfully synced
			return m.syncToken == ""
		}
		// Check if 7 days have passed since last expiry check
		expiryThreshold := 7 * 24 * time.Hour
		return time.Since(m.lastExpiryCheck) > expiryThreshold
	}

	// Without refresh token: if we have a token but no sync token, we haven't successfully synced
	return m.syncToken == ""
}

// P1-HIGH-1: RefreshAccessToken uses refresh_token to get a new access token
// This enables long-lived sessions without requiring re-login with credentials
func (m *MatrixAdapter) RefreshAccessToken() error {
	matrixTracker.Event("refresh_token", nil)

	// P1-HIGH-1: Acquire token mutex to prevent concurrent refreshes
	m.tokenMutex.Lock()
	defer m.tokenMutex.Unlock()

	m.mu.RLock()
	refreshToken := m.refreshToken
	m.mu.RUnlock()

	if refreshToken == "" {
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("no refresh token available")).
			WithFunction("RefreshAccessToken").
			Build()
		matrixTracker.Failure("refresh_token", err, map[string]any{"reason": "no_refresh_token"})
		return err
	}

	// Prepare refresh request according to Matrix spec
	// https://spec.matrix.org/v1.4/client-server-api/#post_matrixclientv3refresh
	payload := map[string]interface{}{
		"refresh_token": refreshToken,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("failed to marshal refresh request: %w", err)).
			WithFunction("RefreshAccessToken").
			Build()
		matrixTracker.Failure("refresh_token", err, map[string]any{"reason": "marshal_failed"})
		return err
	}

	req, err := http.NewRequestWithContext(
		m.ctx,
		"POST",
		m.homeserverURL+"/_matrix/client/v3/refresh",
		bytes.NewReader(body),
	)
	if err != nil {
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("failed to create refresh request: %w", err)).
			WithFunction("RefreshAccessToken").
			WithInputs(map[string]any{"homeserver": m.homeserverURL}).
			Build()
		matrixTracker.Failure("refresh_token", err, map[string]any{"reason": "request_create_failed"})
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		err := errsys.NewBuilder("MAT-001").
			Wrap(fmt.Errorf("refresh request failed: %w", err)).
			WithFunction("RefreshAccessToken").
			Build()
		matrixTracker.Failure("refresh_token", err, map[string]any{"reason": "request_failed"})
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("token refresh failed: status %d, response: %s", resp.StatusCode, string(body))).
			WithFunction("RefreshAccessToken").
			WithInputs(map[string]any{"status": resp.StatusCode}).
			Build()
		matrixTracker.Failure("refresh_token", err, map[string]any{"reason": "refresh_failed", "status": resp.StatusCode})
		return err
	}

	// Parse refresh response
	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"` // May be rotated
		ExpiresIn    int64  `json:"expires_in_ms"` // Token lifetime in milliseconds
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("failed to decode refresh response: %w", err)).
			WithFunction("RefreshAccessToken").
			Build()
		matrixTracker.Failure("refresh_token", err, map[string]any{"reason": "decode_failed"})
		return err
	}

	// Update tokens and reset expiry check timer
	m.mu.Lock()
	m.accessToken = result.AccessToken
	if result.RefreshToken != "" {
		m.refreshToken = result.RefreshToken // Update if rotated
	}
	m.lastExpiryCheck = time.Now() // Reset expiry check timer
	m.mu.Unlock()

	logger.Global().Info("Matrix access token refreshed via refresh_token",
		"expires_in_ms", result.ExpiresIn,
	)

	matrixTracker.Success("refresh_token", map[string]any{"expires_in_ms": result.ExpiresIn})
	return nil
}

// ensureValidToken checks and refreshes access token if needed
// P1-HIGH-1: Call this before API operations to ensure valid credentials
func (m *MatrixAdapter) ensureValidToken() error {
	if m.isTokenExpired() {
		// Try to refresh using refresh token if available
		m.mu.RLock()
		hasRefreshToken := m.refreshToken != ""
		password := m.password
		username := m.userID
		m.mu.RUnlock()

		if hasRefreshToken {
			if err := m.RefreshAccessToken(); err != nil {
				// Refresh failed, try password login as fallback
				if password != "" && username != "" {
					if loginErr := m.Login(username, password); loginErr != nil {
						return errsys.NewBuilder("MAT-002").
							Wrap(fmt.Errorf("failed to refresh expired token and password login also failed: refresh=%w, login=%w", err, loginErr)).
							WithFunction("ensureValidToken").
							Build()
					}
					return nil
				}
				return errsys.NewBuilder("MAT-002").
					Wrap(fmt.Errorf("failed to refresh expired token: %w", err)).
					WithFunction("ensureValidToken").
					Build()
			}
		} else {
			// No refresh token, try password login
			if password != "" && username != "" {
				if err := m.Login(username, password); err != nil {
					return errsys.NewBuilder("MAT-002").
						Wrap(fmt.Errorf("access token expired, password login failed: %w", err)).
						WithFunction("ensureValidToken").
						Build()
				}
				return nil
			}
			return errsys.NewBuilder("MAT-002").
				Wrap(fmt.Errorf("access token expired and no password available for re-login")).
				WithFunction("ensureValidToken").
				Build()
		}
	}
	return nil
}

// containsAny checks if the string contains any of the substrings
func containsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if len(s) >= len(sub) && findSubstring(s, sub) >= 0 {
			return true
		}
	}
	return false
}

// findSubstring finds the index of a substring
func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// SetCommandHandler sets the command handler for admin operations via Matrix
func (m *MatrixAdapter) SetCommandHandler(h *CommandHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commandHandler = h
}

// SetStudioCommandHandler sets the studio command handler for Agent Factory commands
func (m *MatrixAdapter) SetStudioCommandHandler(h StudioCommandHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.studioCmdHandler = h
}

// SetEventBus sets the high-throughput event bus for agent streaming
// This replaces the old polling-based event queue
func (m *MatrixAdapter) SetEventBus(bus *events.MatrixEventBus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventBus = bus
}

// ============================================================================
// ArmorChat Compatibility Methods
// ============================================================================

// SyncWithParams performs a Matrix sync with full parameters
// This is the enhanced version for ArmorChat compatibility
func (m *MatrixAdapter) SyncWithParams(ctx context.Context, since string, timeout int, fullState bool, setPresence string) (map[string]interface{}, error) {
	if err := m.ensureValidToken(); err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	// Build sync URL with parameters
	syncURL := fmt.Sprintf("%s/_matrix/client/v3/sync?access_token=%s", m.homeserverURL, m.accessToken)

	if since != "" {
		syncURL += fmt.Sprintf("&since=%s", since)
	}
	if timeout > 0 {
		syncURL += fmt.Sprintf("&timeout=%d", timeout)
	}
	if fullState {
		syncURL += "&full_state=true"
	}
	if setPresence != "" {
		syncURL += fmt.Sprintf("&set_presence=%s", setPresence)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", syncURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sync request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sync failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode sync response: %w", err)
	}

	return result, nil
}

// CreateRoom creates a new Matrix room
func (m *MatrixAdapter) CreateRoom(ctx context.Context, params map[string]interface{}) (roomID, roomAlias string, err error) {
	if err := m.ensureValidToken(); err != nil {
		return "", "", fmt.Errorf("token validation failed: %w", err)
	}

	createURL := fmt.Sprintf("%s/_matrix/client/v3/createRoom?access_token=%s", m.homeserverURL, m.accessToken)

	bodyBytes, err := json.Marshal(params)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal create room request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", createURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("create room request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("create room failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		RoomID    string `json:"room_id"`
		RoomAlias string `json:"room_alias,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("failed to decode create room response: %w", err)
	}

	return result.RoomID, result.RoomAlias, nil
}

// JoinRoom joins an existing Matrix room
func (m *MatrixAdapter) JoinRoom(ctx context.Context, roomIDOrAlias string, viaServers []string, reason string) (string, error) {
	if err := m.ensureValidToken(); err != nil {
		return "", fmt.Errorf("token validation failed: %w", err)
	}

	// URL encode the room ID/alias
	encodedRoomID := url.PathEscape(roomIDOrAlias)
	joinURL := fmt.Sprintf("%s/_matrix/client/v3/join/%s?access_token=%s", m.homeserverURL, encodedRoomID, m.accessToken)

	// Build request body with optional via servers and reason
	reqBody := map[string]interface{}{}
	if len(viaServers) > 0 {
		reqBody["via_servers"] = viaServers
	}
	if reason != "" {
		reqBody["reason"] = reason
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal join room request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", joinURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("join room request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("join room failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		RoomID string `json:"room_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode join room response: %w", err)
	}

	return result.RoomID, nil
}

// LeaveRoom leaves a Matrix room
func (m *MatrixAdapter) LeaveRoom(ctx context.Context, roomID string, reason string) error {
	if err := m.ensureValidToken(); err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	leaveURL := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/leave?access_token=%s", m.homeserverURL, roomID, m.accessToken)

	reqBody := map[string]interface{}{}
	if reason != "" {
		reqBody["reason"] = reason
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal leave room request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", leaveURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("leave room request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("leave room failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// InviteUser invites a user to a Matrix room
func (m *MatrixAdapter) InviteUser(ctx context.Context, roomID, userID, reason string) error {
	if err := m.ensureValidToken(); err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	inviteURL := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/invite?access_token=%s", m.homeserverURL, roomID, m.accessToken)

	reqBody := map[string]interface{}{
		"user_id": userID,
	}
	if reason != "" {
		reqBody["reason"] = reason
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal invite request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", inviteURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("invite request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("invite failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// SendTyping sends a typing notification to a room
func (m *MatrixAdapter) SendTyping(ctx context.Context, roomID string, typing bool, timeout int) error {
	if err := m.ensureValidToken(); err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	userID := m.GetUserID()
	typingURL := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/typing/%s?access_token=%s",
		m.homeserverURL, roomID, userID, m.accessToken)

	reqBody := map[string]interface{}{
		"typing": typing,
	}
	if typing && timeout > 0 {
		reqBody["timeout"] = timeout
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal typing request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", typingURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("typing request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("typing notification failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// SendReadReceipt sends a read receipt for an event
func (m *MatrixAdapter) SendReadReceipt(ctx context.Context, roomID, eventID, receiptType string) error {
	if err := m.ensureValidToken(); err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	// Default to m.read if not specified
	if receiptType == "" {
		receiptType = "m.read"
	}

	receiptURL := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/receipt/%s/%s?access_token=%s",
		m.homeserverURL, roomID, receiptType, eventID, m.accessToken)

	reqBody := map[string]interface{}{
		"data": map[string]interface{}{
			"thread_id": "main", // Optional: indicate which thread the receipt applies to
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal receipt request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", receiptURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("receipt request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("read receipt failed with status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// ============================================================================
// RPC Interface Methods
// ============================================================================

// IsLoggedIn returns true if the adapter has a valid access token
func (m *MatrixAdapter) IsLoggedIn() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.accessToken != ""
}

// GetHomeserver returns the homeserver URL
func (m *MatrixAdapter) GetHomeserver() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.homeserverURL
}

// GetEventBus returns the event bus if configured
func (m *MatrixAdapter) GetEventBus() *events.MatrixEventBus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.eventBus
}

// ChangePassword changes the user's password via Matrix API
func (m *MatrixAdapter) ChangePassword(ctx context.Context, newPassword string, logoutDevices bool) error {
	matrixTracker.Event("change_password", nil)

	// Ensure token is valid before password change
	if err := m.ensureValidToken(); err != nil {
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("token validation failed: %w", err)).
			WithFunction("ChangePassword").
			Build()
		matrixTracker.Failure("change_password", err, map[string]any{"reason": "token_validation_failed"})
		return err
	}

	m.mu.RLock()
	token := m.accessToken
	m.mu.RUnlock()

	if token == "" {
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("not logged in")).
			WithFunction("ChangePassword").
			Build()
		matrixTracker.Failure("change_password", err, map[string]any{"reason": "not_authenticated"})
		return err
	}

	payload := map[string]interface{}{
		"new_password":   newPassword,
		"logout_devices": logoutDevices,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		err := errsys.NewBuilder("MAT-002").
			Wrap(fmt.Errorf("failed to marshal password change request: %w", err)).
			WithFunction("ChangePassword").
			Build()
		matrixTracker.Failure("change_password", err, map[string]any{"reason": "marshal_failed"})
		return err
	}

	url := m.homeserverURL + "/_matrix/client/v3/account/password"

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		url,
		bytes.NewReader(body),
	)
	if err != nil {
		err := errsys.NewBuilder("MAT-001").
			Wrap(fmt.Errorf("failed to create password change request: %w", err)).
			WithFunction("ChangePassword").
			Build()
		matrixTracker.Failure("change_password", err, map[string]any{"reason": "request_create_failed"})
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		err := errsys.NewBuilder("MAT-001").
			Wrap(fmt.Errorf("password change request failed: %w", err)).
			WithFunction("ChangePassword").
			Build()
		matrixTracker.Failure("change_password", err, map[string]any{"reason": "request_failed"})
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var matrixErr MatrixError
		if err := json.Unmarshal(body, &matrixErr); err != nil {
			matrixErr = MatrixError{Error: string(body)}
		}

		var builder *errsys.ErrorBuilder
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			builder = errsys.NewBuilder("MAT-002").
				Wrap(fmt.Errorf("unauthorized: %s", matrixErr.Error)).
				WithFunction("ChangePassword")
		case http.StatusBadRequest:
			builder = errsys.NewBuilder("MAT-002").
				Wrap(fmt.Errorf("bad request: %s", matrixErr.Error)).
				WithFunction("ChangePassword")
		default:
			builder = errsys.NewBuilder("MAT-001").
				Wrap(fmt.Errorf("password change failed: status %d, error: %s", resp.StatusCode, matrixErr.Error)).
				WithFunction("ChangePassword")
		}
		matrixTracker.Failure("change_password", builder.Build(), map[string]any{"reason": "api_failed", "status": resp.StatusCode})
		return builder.Build()
	}

	matrixTracker.Success("change_password", nil)
	return nil
}

// SendFormattedMessage sends a message with HTML formatting
func (m *MatrixAdapter) SendFormattedMessage(ctx context.Context, roomID, plainBody, formattedBody string) error {
	payload := map[string]interface{}{
		"msgtype":        "m.text",
		"body":           plainBody,
		"format":         "org.matrix.custom.html",
		"formatted_body": formattedBody,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return m.SendEvent(roomID, "m.room.message", data)
}

// ReplyToEvent sends a reply to a specific event
func (m *MatrixAdapter) ReplyToEvent(ctx context.Context, roomID, eventID, message string) error {
	payload := map[string]interface{}{
		"msgtype": "m.text",
		"body":    message,
		"m.relates_to": map[string]interface{}{
			"m.in_reply_to": map[string]interface{}{
				"event_id": eventID,
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return m.SendEvent(roomID, "m.room.message", data)
}
