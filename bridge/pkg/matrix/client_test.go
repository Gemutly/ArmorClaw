package matrix

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// helper: creates a valid Config pointing at the given server URL.
func testConfig(serverURL string) Config {
	return Config{
		HomeserverURL: serverURL,
		AccessToken:   "test-token",
		DeviceID:      "DEVDEVICE",
		RoomID:        "!room:test-server",
		StorePath:     "/tmp/matrix-test",
	}
}

// --------------------------------------------------------------------------
// 1. New – validation
// --------------------------------------------------------------------------

func TestNew_ValidatesRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name:    "empty homeserver URL",
			cfg:     Config{HomeserverURL: "", AccessToken: "tok", RoomID: "!r:s"},
			wantErr: "homeserver URL is required",
		},
		{
			name:    "empty access token",
			cfg:     Config{HomeserverURL: "http://localhost", AccessToken: "", RoomID: "!r:s"},
			wantErr: "access token is required",
		},
		{
			name:    "empty room ID",
			cfg:     Config{HomeserverURL: "http://localhost", AccessToken: "tok", RoomID: ""},
			wantErr: "room ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestNew_Success(t *testing.T) {
	c, err := New(testConfig("http://localhost"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.GetRoomID() != "!room:test-server" {
		t.Fatalf("expected room ID !room:test-server, got %q", c.GetRoomID())
	}
}

// --------------------------------------------------------------------------
// 2. Login
// --------------------------------------------------------------------------

func TestLogin_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/_matrix/client/v3/login" {
			t.Errorf("expected path /_matrix/client/v3/login, got %s", r.URL.Path)
		}

		// Validate request body
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("failed to parse login body: %v", err)
		}
		if payload["type"] != "m.login.password" {
			t.Errorf("expected type m.login.password, got %v", payload["type"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "new-access-token",
			"user_id":      "@alice:test-server",
			"device_id":    "NEWDEVICE",
		})
	}))
	defer ts.Close()

	c, err := New(testConfig(ts.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = c.Login(context.Background(), "alice", "password123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if c.GetUserID() != "@alice:test-server" {
		t.Fatalf("expected userID @alice:test-server, got %q", c.GetUserID())
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"errcode": "M_FORBIDDEN",
			"error":   "Invalid username/password",
		})
	}))
	defer ts.Close()

	c, err := New(testConfig(ts.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = c.Login(context.Background(), "alice", "wrong")
	if err == nil {
		t.Fatal("expected error for invalid credentials, got nil")
	}
	if !strings.Contains(err.Error(), "status 403") {
		t.Fatalf("expected status 403 in error, got %q", err.Error())
	}
}

// --------------------------------------------------------------------------
// 3. SendMessage
// --------------------------------------------------------------------------

func TestSendMessage_Success(t *testing.T) {
	var receivedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path

		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer auth header, got %q", r.Header.Get("Authorization"))
		}

		// Validate message body
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		json.Unmarshal(body, &payload)
		if payload["msgtype"] != "m.text" {
			t.Errorf("expected msgtype m.text, got %v", payload["msgtype"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"event_id": "$event123",
		})
	}))
	defer ts.Close()

	c, err := New(testConfig(ts.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = c.SendMessage(context.Background(), "Hello, Matrix!")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	expectedPrefix := "/_matrix/client/r0/rooms/!room:test-server/send/m.room.message/"
	if !strings.HasPrefix(receivedPath, expectedPrefix) {
		t.Fatalf("expected path prefix %q, got %q", expectedPrefix, receivedPath)
	}
}

func TestSendMessage_TooLarge(t *testing.T) {
	// Build a client with a server that would fail if contacted.
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	c, err := New(testConfig(ts.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bigMessage := strings.Repeat("x", 65537)
	err = c.SendMessage(context.Background(), bigMessage)
	if err == nil {
		t.Fatal("expected ErrMessageTooLarge, got nil")
	}
	if !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("expected ErrMessageTooLarge, got %v", err)
	}
	if called {
		t.Fatal("expected no HTTP call for oversized message")
	}
}

func TestSendMessage_NotLoggedIn(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	c, err := New(testConfig(ts.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Clear the access token to simulate not logged in
	c.accessToken = ""

	err = c.SendMessage(context.Background(), "hello")
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("expected ErrNotLoggedIn, got %v", err)
	}
}

// --------------------------------------------------------------------------
// 4. GetMessages
// --------------------------------------------------------------------------

func TestGetMessages_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		expectedPath := "/_matrix/client/r0/rooms/!room:test-server/messages"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %q, got %q", expectedPath, r.URL.Path)
		}
		if r.URL.Query().Get("dir") != "b" {
			t.Errorf("expected dir=b, got %q", r.URL.Query().Get("dir"))
		}
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("expected limit=10, got %q", r.URL.Query().Get("limit"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"chunk": []map[string]interface{}{
				{
					"type": "m.room.message",
					"content": map[string]string{
						"msgtype": "m.text",
						"body":    "Hello from Matrix",
					},
				},
				{
					"type": "m.room.message",
					"content": map[string]string{
						"msgtype": "m.text",
						"body":    "Second message",
					},
				},
			},
			"start": "t0",
			"end":   "t1",
		})
	}))
	defer ts.Close()

	c, err := New(testConfig(ts.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs, err := c.GetMessages(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Content.Body != "Hello from Matrix" {
		t.Fatalf("expected body 'Hello from Matrix', got %q", msgs[0].Content.Body)
	}
	if msgs[1].Content.Body != "Second message" {
		t.Fatalf("expected body 'Second message', got %q", msgs[1].Content.Body)
	}
}

func TestGetMessages_NotLoggedIn(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	c, err := New(testConfig(ts.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c.accessToken = ""

	_, err = c.GetMessages(context.Background(), 10)
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("expected ErrNotLoggedIn, got %v", err)
	}
}

// --------------------------------------------------------------------------
// 5. JoinRoom
// --------------------------------------------------------------------------

func TestJoinRoom_Success(t *testing.T) {
	var receivedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path

		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer auth header, got %q", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"room_id": "!newroom:test-server",
		})
	}))
	defer ts.Close()

	c, err := New(testConfig(ts.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = c.JoinRoom(context.Background(), "!newroom:test-server")
	if err != nil {
		t.Fatalf("JoinRoom failed: %v", err)
	}

	expected := "/_matrix/client/r0/rooms/!newroom:test-server/join"
	if receivedPath != expected {
		t.Fatalf("expected path %q, got %q", expected, receivedPath)
	}
}

func TestJoinRoom_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"errcode": "M_NOT_FOUND",
			"error":   "Room not found",
		})
	}))
	defer ts.Close()

	c, err := New(testConfig(ts.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = c.JoinRoom(context.Background(), "!badroom:server")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if !strings.Contains(err.Error(), "status 404") {
		t.Fatalf("expected status 404 in error, got %q", err.Error())
	}
}

// --------------------------------------------------------------------------
// 6. Sync
// --------------------------------------------------------------------------

func TestSync_Success(t *testing.T) {
	var receivedFilter string
	var receivedSince string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/_matrix/client/v3/sync" {
			t.Errorf("expected path /_matrix/client/v3/sync, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer auth header, got %q", r.Header.Get("Authorization"))
		}

		receivedFilter = r.URL.Query().Get("filter")
		receivedSince = r.URL.Query().Get("since")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"next_batch": "s1_2_3",
			"rooms": map[string]interface{}{
				"!room:test-server": map[string]interface{}{
					"timeline": map[string]interface{}{
						"events": []interface{}{},
						"limited": false,
					},
				},
			},
		})
	}))
	defer ts.Close()

	c, err := New(testConfig(ts.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	filter := DefaultSyncFilter()
	resp, err := c.Sync(context.Background(), "s0_1_0", filter, 5000)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if resp.NextBatch != "s1_2_3" {
		t.Fatalf("expected next_batch s1_2_3, got %q", resp.NextBatch)
	}
	if receivedSince != "s0_1_0" {
		t.Fatalf("expected since=s0_1_0, got %q", receivedSince)
	}
	// Verify the filter was sent
	if receivedFilter == "" {
		t.Fatal("expected filter to be sent")
	}
	// Parse the filter back and verify it matches
	var sentFilter SyncFilter
	if err := json.Unmarshal([]byte(receivedFilter), &sentFilter); err != nil {
		t.Fatalf("failed to parse sent filter: %v", err)
	}
	if sentFilter.Room.Timeline.Limit != 50 {
		t.Fatalf("expected filter limit 50, got %d", sentFilter.Room.Timeline.Limit)
	}
	if sentFilter.Room.State.LazyLoadMembers != true {
		t.Fatal("expected lazy_load_members=true in filter")
	}
}

func TestSync_NotLoggedIn(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	c, err := New(testConfig(ts.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c.accessToken = ""

	_, err = c.Sync(context.Background(), "", DefaultSyncFilter(), 30000)
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("expected ErrNotLoggedIn, got %v", err)
	}
}

func TestSync_DefaultTimeout(t *testing.T) {
	var receivedTimeout string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTimeout = r.URL.Query().Get("timeout")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"next_batch": "batch1",
		})
	}))
	defer ts.Close()

	c, err := New(testConfig(ts.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Pass timeout=0, should default to 30000
	_, err = c.Sync(context.Background(), "", DefaultSyncFilter(), 0)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if receivedTimeout != "30000" {
		t.Fatalf("expected timeout=30000 (default), got %q", receivedTimeout)
	}
}

// --------------------------------------------------------------------------
// 7. DefaultSyncFilter
// --------------------------------------------------------------------------

func TestDefaultSyncFilter(t *testing.T) {
	f := DefaultSyncFilter()

	if f.Room.Timeline.Limit != 50 {
		t.Fatalf("expected timeline limit 50, got %d", f.Room.Timeline.Limit)
	}
	if !f.Room.State.LazyLoadMembers {
		t.Fatal("expected lazy_load_members=true")
	}
}

// --------------------------------------------------------------------------
// 8. GetUserID / GetRoomID accessors
// --------------------------------------------------------------------------

func TestGetUserID(t *testing.T) {
	c, err := New(testConfig("http://localhost"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Initially empty
	if c.GetUserID() != "" {
		t.Fatalf("expected empty userID, got %q", c.GetUserID())
	}
}

func TestGetRoomID(t *testing.T) {
	c, err := New(testConfig("http://localhost"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.GetRoomID() != "!room:test-server" {
		t.Fatalf("expected room ID !room:test-server, got %q", c.GetRoomID())
	}
}
