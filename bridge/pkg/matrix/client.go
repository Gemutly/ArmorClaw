// Package matrix provides Matrix client integration for ArmorClaw.
// This enables encrypted communication through the Matrix homeserver.
package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

var (
	ErrNotLoggedIn      = errors.New("not logged in")
	ErrRoomNotFound     = errors.New("room not found")
	ErrMessageTooLarge  = errors.New("message exceeds size limit")
	ErrEncryptionFailed = errors.New("encryption failed")
)

// Client represents a Matrix client with E2EE support
type Client struct {
	homeserver  string
	accessToken string
	userID      string
	deviceID    string
	roomID      string
	dbPath      string
	httpClient  *http.Client
}

// Config holds Matrix client configuration
type Config struct {
	HomeserverURL string
	AccessToken   string
	DeviceID      string
	RoomID        string
	StorePath     string
}

// MessageEvent represents a Matrix message event
type MessageEvent struct {
	Type    string `json:"type"`
	Content struct {
		MsgType string `json:"msgtype"`
		Body    string `json:"body"`
	} `json:"content"`
}

// New creates a new Matrix client
func New(cfg Config) (*Client, error) {
	if cfg.HomeserverURL == "" {
		return nil, errors.New("homeserver URL is required")
	}
	if cfg.AccessToken == "" {
		return nil, errors.New("access token is required")
	}
	if cfg.RoomID == "" {
		return nil, errors.New("room ID is required")
	}

	return &Client{
		homeserver:  cfg.HomeserverURL,
		accessToken: cfg.AccessToken,
		deviceID:    cfg.DeviceID,
		roomID:      cfg.RoomID,
		dbPath:      cfg.StorePath,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Login authenticates with the Matrix homeserver
func (c *Client) Login(ctx context.Context, username, password string) error {
	// Implement password-based login using the Matrix Client API
	// POST /_matrix/client/v3/login
	payload := map[string]interface{}{
		"type": "m.login.password",
		"identifier": map[string]string{
			"type": "m.id.user",
			"user": username,
		},
		"password": password,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal login request: %w", err)
	}

	url := fmt.Sprintf("%s/_matrix/client/v3/login", c.homeserver)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ArmorClaw-Bridge/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		UserID      string `json:"user_id"`
		DeviceID    string `json:"device_id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode login response: %w", err)
	}

	c.accessToken = result.AccessToken
	c.userID = result.UserID
	c.deviceID = result.DeviceID

	return nil
}

// SendMessage sends a text message to the configured room
func (c *Client) SendMessage(ctx context.Context, message string) error {
	if c.accessToken == "" {
		return ErrNotLoggedIn
	}

	if len(message) > 65536 {
		return ErrMessageTooLarge
	}

	// Build transaction ID
	txnID := fmt.Sprintf("m%d", time.Now().UnixMilli())

	// PUT /_matrix/client/r0/rooms/{roomId}/send/m.room.message/{txnId}
	url := fmt.Sprintf("%s/_matrix/client/r0/rooms/%s/send/m.room.message/%s",
		c.homeserver, c.roomID, txnID)

	payload := map[string]interface{}{
		"msgtype": "m.text",
		"body":    message,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("User-Agent", "ArmorClaw-Bridge/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send message failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send message failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		EventID string `json:"event_id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// GetMessages syncs messages from the room
func (c *Client) GetMessages(ctx context.Context, limit int) ([]MessageEvent, error) {
	if c.accessToken == "" {
		return nil, ErrNotLoggedIn
	}

	// GET /_matrix/client/r0/rooms/{roomId}/messages
	url := fmt.Sprintf("%s/_matrix/client/r0/rooms/%s/messages?limit=%d&dir=b",
		c.homeserver, c.roomID, limit)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("User-Agent", "ArmorClaw-Bridge/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get messages failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get messages failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Chunk []MessageEvent `json:"chunk"`
		Start string         `json:"start"`
		End   string         `json:"end"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Chunk, nil
}

// JoinRoom joins a room
func (c *Client) JoinRoom(ctx context.Context, roomID string) error {
	// POST /_matrix/client/r0/rooms/{roomId}/join
	url := fmt.Sprintf("%s/_matrix/client/r0/rooms/%s/join", c.homeserver, roomID)

	payload := map[string]interface{}{}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("User-Agent", "ArmorClaw-Bridge/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("join room failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("join room failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// SyncFilter configures the /sync endpoint to reduce payload size at scale.
// See: https://spec.matrix.org/v1.9/client-server-api/#filtering
type SyncFilter struct {
	Room RoomFilter `json:"room"`
}

// RoomFilter filters room-level events in sync responses
type RoomFilter struct {
	Timeline EventFilter `json:"timeline"`
	State    StateFilter `json:"state"`
}

// EventFilter limits timeline events
type EventFilter struct {
	Limit int `json:"limit"`
}

// StateFilter controls state event delivery
type StateFilter struct {
	LazyLoadMembers bool `json:"lazy_load_members"`
}

type ToDevice struct {
	Events []json.RawMessage `json:"events"`
}

type DeviceLists struct {
	Changed []string `json:"changed,omitempty"`
	Left    []string `json:"left,omitempty"`
}

type SyncResponse struct {
	NextBatch             string              `json:"next_batch"`
	ToDevice              *ToDevice           `json:"to_device,omitempty"`
	DeviceLists           *DeviceLists        `json:"device_lists,omitempty"`
	DeviceOneTimeKeysCount map[string]int     `json:"device_one_time_keys_count,omitempty"`
	Rooms                 map[string]json.RawMessage `json:"rooms,omitempty"`
}

// DefaultSyncFilter returns a filter suitable for production use
// - Limits timeline events to 50 per room
// - Enables lazy loading of member events to reduce payload size
func DefaultSyncFilter() SyncFilter {
	return SyncFilter{
		Room: RoomFilter{
			Timeline: EventFilter{Limit: 50},
			State:    StateFilter{LazyLoadMembers: true},
		},
	}
}

// Sync performs an incremental sync with the homeserver.
// Uses the provided filter to limit response size and latency.
// Pass an empty sinceToken for the initial sync.
func (c *Client) Sync(ctx context.Context, sinceToken string, filter SyncFilter, timeout int) (*SyncResponse, error) {
	if c.accessToken == "" {
		return nil, ErrNotLoggedIn
	}

	if timeout <= 0 {
		timeout = 30000 // 30s long-poll default
	}

	// Serialize the filter
	filterJSON, err := json.Marshal(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sync filter: %w", err)
	}

	// Build URL with query params
	url := fmt.Sprintf("%s/_matrix/client/v3/sync?timeout=%d&filter=%s",
		c.homeserver, timeout, string(filterJSON))

	if sinceToken != "" {
		url += "&since=" + sinceToken
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("User-Agent", "ArmorClaw-Bridge/1.0")

	// Use a longer HTTP timeout to accommodate the long-poll
	syncClient := &http.Client{
		Timeout: time.Duration(timeout+10000) * time.Millisecond,
	}

	resp, err := syncClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sync request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sync failed (status %d): %s", resp.StatusCode, string(body))
	}

	var syncResp SyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncResp); err != nil {
		return nil, fmt.Errorf("failed to decode sync response: %w", err)
	}

	return &syncResp, nil
}

// GetUserID returns the current user ID
func (c *Client) GetUserID() string {
	return c.userID
}

// GetRoomID returns the current room ID
func (c *Client) GetRoomID() string {
	return c.roomID
}
