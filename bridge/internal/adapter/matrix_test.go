package adapter

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/armorclaw/bridge/internal/events"
	errsys "github.com/armorclaw/bridge/pkg/errors"
)

func buildTestSyncResponse(roomID string, eventMaps []map[string]interface{}) *SyncResponse {
	rawEvents := make([]json.RawMessage, len(eventMaps))
	for i, evt := range eventMaps {
		data, _ := json.Marshal(evt)
		rawEvents[i] = data
	}

	syncJSON := map[string]interface{}{
		"next_batch": "test_batch",
		"rooms": map[string]interface{}{
			"join": map[string]interface{}{
				roomID: map[string]interface{}{
					"timeline": map[string]interface{}{
						"events": rawEvents,
					},
				},
			},
		},
	}
	data, _ := json.Marshal(syncJSON)
	var resp SyncResponse
	json.Unmarshal(data, &resp)
	return &resp
}

func TestProcessEvents_WorkflowEventHandled(t *testing.T) {
	m, err := New(Config{
		HomeserverURL: "http://localhost:6167",
	})
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	bus := events.NewMatrixEventBus(64)
	m.SetEventBus(bus)
	sub := bus.Subscribe()

	syncResp := buildTestSyncResponse("!test:localhost", []map[string]interface{}{
		{
			"type":          "workflow.progress",
			"room_id":       "!test:localhost",
			"sender":        "@agent:localhost",
			"content":       map[string]interface{}{"step": 1, "total": 5},
			"event_id":      "$wf1",
			"origin_server": "matrix",
		},
	})

	processed := m.processEvents(syncResp)
	if processed != 1 {
		t.Errorf("expected 1 event processed, got %d", processed)
	}

	select {
	case evt := <-sub:
		if evt.Type != "workflow.progress" {
			t.Errorf("expected workflow.progress event, got %s", evt.Type)
		}
		if evt.RoomID != "!test:localhost" {
			t.Errorf("expected room !test:localhost, got %s", evt.RoomID)
		}
	default:
		t.Error("workflow event was not published to event bus")
	}
}

func TestProcessEvents_AgentEventHandled(t *testing.T) {
	m, err := New(Config{
		HomeserverURL: "http://localhost:6167",
	})
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	bus := events.NewMatrixEventBus(64)
	m.SetEventBus(bus)
	sub := bus.Subscribe()

	syncResp := buildTestSyncResponse("!test:localhost", []map[string]interface{}{
		{
			"type":          "agent.comment",
			"room_id":       "!test:localhost",
			"sender":        "@bot:localhost",
			"content":       map[string]interface{}{"body": "note"},
			"event_id":      "$ag1",
			"origin_server": "matrix",
		},
	})

	processed := m.processEvents(syncResp)
	if processed != 1 {
		t.Errorf("expected 1 event processed, got %d", processed)
	}

	select {
	case evt := <-sub:
		if evt.Type != "agent.comment" {
			t.Errorf("expected agent.comment event, got %s", evt.Type)
		}
	default:
		t.Error("agent event was not published to event bus")
	}
}

func TestProcessEvents_BlockerEventHandled(t *testing.T) {
	m, err := New(Config{
		HomeserverURL: "http://localhost:6167",
	})
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	bus := events.NewMatrixEventBus(64)
	m.SetEventBus(bus)
	sub := bus.Subscribe()

	syncResp := buildTestSyncResponse("!test:localhost", []map[string]interface{}{
		{
			"type":          "blocker.required",
			"room_id":       "!test:localhost",
			"sender":        "@system:localhost",
			"content":       map[string]interface{}{"reason": "approval needed"},
			"event_id":      "$bl1",
			"origin_server": "matrix",
		},
	})

	processed := m.processEvents(syncResp)
	if processed != 1 {
		t.Errorf("expected 1 event processed, got %d", processed)
	}

	select {
	case evt := <-sub:
		if evt.Type != "blocker.required" {
			t.Errorf("expected blocker.required event, got %s", evt.Type)
		}
	default:
		t.Error("blocker event was not published to event bus")
	}
}

func TestProcessEvents_MessageUnchanged(t *testing.T) {
	m, err := New(Config{
		HomeserverURL: "http://localhost:6167",
	})
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	bus := events.NewMatrixEventBus(64)
	m.SetEventBus(bus)
	sub := bus.Subscribe()

	syncResp := buildTestSyncResponse("!test:localhost", []map[string]interface{}{
		{
			"type":          "m.room.message",
			"room_id":       "!test:localhost",
			"sender":        "@user:localhost",
			"content":       map[string]interface{}{"msgtype": "m.text", "body": "hello"},
			"event_id":      "$msg1",
			"origin_server": "matrix",
		},
	})

	processed := m.processEvents(syncResp)
	if processed != 1 {
		t.Errorf("expected 1 event processed, got %d", processed)
	}

	select {
	case evt := <-sub:
		if evt.Type != "m.room.message" {
			t.Errorf("expected m.room.message event, got %s", evt.Type)
		}
	default:
		t.Error("m.room.message event was not published to event bus")
	}

	select {
	case evt := <-m.ReceiveEvents():
		if evt.Type != "m.room.message" {
			t.Errorf("expected m.room.message in queue, got %s", evt.Type)
		}
	default:
		t.Error("m.room.message event was not queued")
	}
}

func TestProcessEvents_UnknownCustomLogged(t *testing.T) {
	m, err := New(Config{
		HomeserverURL: "http://localhost:6167",
	})
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	bus := events.NewMatrixEventBus(64)
	m.SetEventBus(bus)
	sub := bus.Subscribe()

	syncResp := buildTestSyncResponse("!test:localhost", []map[string]interface{}{
		{
			"type":          "custom.unknown",
			"room_id":       "!test:localhost",
			"sender":        "@test:localhost",
			"content":       map[string]interface{}{},
			"event_id":      "$unk1",
			"origin_server": "matrix",
		},
	})

	processed := m.processEvents(syncResp)
	if processed != 1 {
		t.Errorf("expected 1 event processed (not silently dropped), got %d", processed)
	}

	select {
	case evt := <-sub:
		t.Errorf("unknown custom event should not be published to event bus, got type=%s", evt.Type)
	default:
	}
}

func TestProcessEvents_MatrixStateEventNotCustom(t *testing.T) {
	m, err := New(Config{
		HomeserverURL: "http://localhost:6167",
	})
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	syncResp := buildTestSyncResponse("!test:localhost", []map[string]interface{}{
		{
			"type":          "m.room.member",
			"room_id":       "!test:localhost",
			"sender":        "@user:localhost",
			"content":       map[string]interface{}{"membership": "join"},
			"event_id":      "$mem1",
			"origin_server": "matrix",
			"state_key":     "@user:localhost",
		},
	})

	processed := m.processEvents(syncResp)
	if processed != 1 {
		t.Errorf("expected 1 event processed, got %d", processed)
	}
}

func TestProcessSyncResult(t *testing.T) {
	tests := []struct {
		name                string
		err                 error
		consecutiveFailures int
		backoff             time.Duration
		wantFailures        int
		wantBackoff         time.Duration
		wantAction          syncAction
	}{
		{
			name:                "nil error resets backoff",
			err:                 nil,
			consecutiveFailures: 5,
			backoff:             8 * time.Second,
			wantFailures:        0,
			wantBackoff:         syncInitialBackoff,
			wantAction:          actionResetBackoff,
		},
		{
			name:                "generic error first failure continues",
			err:                 errors.New("connection refused"),
			consecutiveFailures: 0,
			backoff:             syncInitialBackoff,
			wantFailures:        1,
			wantBackoff:         2 * time.Second,
			wantAction:          actionContinue,
		},
		{
			name:                "third failure triggers relogin",
			err:                 errors.New("server error"),
			consecutiveFailures: 2,
			backoff:             4 * time.Second,
			wantFailures:        3,
			wantBackoff:         8 * time.Second,
			wantAction:          actionRelogin,
		},
		{
			name:                "ErrTokenInvalidated triggers immediate relogin",
			err:                 ErrTokenInvalidated,
			consecutiveFailures: 0,
			backoff:             syncInitialBackoff,
			wantFailures:        1,
			wantBackoff:         syncInitialBackoff,
			wantAction:          actionRelogin,
		},
		{
			name:                "success after failures resets everything",
			err:                 nil,
			consecutiveFailures: 7,
			backoff:             30 * time.Second,
			wantFailures:        0,
			wantBackoff:         syncInitialBackoff,
			wantAction:          actionResetBackoff,
		},
		{
			name:                "backoff caps at syncMaxBackoff",
			err:                 errors.New("slow down"),
			consecutiveFailures: 1,
			backoff:             20 * time.Second,
			wantFailures:        2,
			wantBackoff:         30 * time.Second,
			wantAction:          actionContinue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFailures, gotBackoff, gotAction := processSyncResult(tt.err, tt.consecutiveFailures, tt.backoff)
			if gotFailures != tt.wantFailures {
				t.Errorf("failures = %d, want %d", gotFailures, tt.wantFailures)
			}
			if gotBackoff != tt.wantBackoff {
				t.Errorf("backoff = %v, want %v", gotBackoff, tt.wantBackoff)
			}
			if gotAction != tt.wantAction {
				t.Errorf("action = %v, want %v", gotAction, tt.wantAction)
			}
		})
	}
}

func TestExtractHTTPStatus(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "nil error returns 0",
			err:      nil,
			wantCode: 0,
		},
		{
			name:     "plain error returns 0",
			err:      errors.New("something failed"),
			wantCode: 0,
		},
		{
			name: "TracedError with status 502",
			err: &errsys.TracedError{
				Code:    "SYNC_502",
				Message: "bad gateway",
				Inputs:  map[string]interface{}{"status": 502},
			},
			wantCode: 502,
		},
		{
			name: "TracedError with status 429",
			err: &errsys.TracedError{
				Code:    "SYNC_429",
				Message: "rate limited",
				Inputs:  map[string]interface{}{"status": 429},
			},
			wantCode: 429,
		},
		{
			name: "TracedError with nil Inputs returns 0",
			err: &errsys.TracedError{
				Code:    "SYNC_ERR",
				Message: "no inputs",
			},
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractHTTPStatus(tt.err)
			if got != tt.wantCode {
				t.Errorf("extractHTTPStatus() = %d, want %d", got, tt.wantCode)
			}
		})
	}
}

func TestGetStatus(t *testing.T) {
	// Test the initial empty state
	ma := &MatrixAdapter{}
	if got := ma.GetStatus(); got != "" {
		t.Errorf("initial GetStatus() = %q, want empty string", got)
	}

	// Test after notifyStatus
	ma.notifyStatus("connected")
	if got := ma.GetStatus(); got != "connected" {
		t.Errorf("after notifyStatus, GetStatus() = %q, want %q", got, "connected")
	}

	// Test status update
	ma.notifyStatus("reconnecting")
	if got := ma.GetStatus(); got != "reconnecting" {
		t.Errorf("after update, GetStatus() = %q, want %q", got, "reconnecting")
	}
}
