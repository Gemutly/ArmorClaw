package rpc

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/armorclaw/bridge/pkg/invite"
	"github.com/armorclaw/bridge/pkg/trust"
)

// mockEventMatrixAdapter tracks SendEvent calls for test assertions.
type mockEventMatrixAdapter struct {
	mu     sync.Mutex
	events []mockSentEvent
}

type mockSentEvent struct {
	RoomID    string
	EventType string
	Content   []byte
}

func (m *mockEventMatrixAdapter) SendMessage(_, _, _ string) (string, error) {
	return "", nil
}

func (m *mockEventMatrixAdapter) SendEvent(roomID, eventType string, content []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, mockSentEvent{
		RoomID:    roomID,
		EventType: eventType,
		Content:   content,
	})
	return nil
}

func (m *mockEventMatrixAdapter) Login(_, _ string) error                        { return nil }
func (m *mockEventMatrixAdapter) JoinRoom(_ context.Context, _ string, _ []string, _ string) (string, error) {
	return "", nil
}
func (m *mockEventMatrixAdapter) GetUserID() string   { return "@bridge:test.com" }
func (m *mockEventMatrixAdapter) IsLoggedIn() bool    { return true }
func (m *mockEventMatrixAdapter) GetHomeserver() string { return "https://test.com" }
func (m *mockEventMatrixAdapter) GetStatus() string     { return "connected" }

func (m *mockEventMatrixAdapter) getEvents() []mockSentEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]mockSentEvent, len(m.events))
	copy(out, m.events)
	return out
}

const testGovernanceRoom = "!governance:test.com"

func newServerWithEvents(t *testing.T) (*Server, *mockEventMatrixAdapter) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ds, err := trust.NewDeviceStore(db)
	if err != nil {
		t.Fatalf("new device store: %v", err)
	}
	is, err := invite.NewInviteStore(db)
	if err != nil {
		t.Fatalf("new invite store: %v", err)
	}

	mock := &mockEventMatrixAdapter{}
	s := &Server{
		handlers:        make(map[string]HandlerFunc, 32),
		deviceStore:     ds,
		inviteStore:     is,
		matrix:          mock,
		governanceRoomID: testGovernanceRoom,
	}
	return s, mock
}

func TestEmitDeviceEventOnApprove(t *testing.T) {
	s, mock := newServerWithEvents(t)
	seedDevice(t, s.deviceStore, "dev-emit-1", trust.StateUnverified)

	_, rpcErr := s.handleDeviceApprove(context.Background(), &Request{
		Params: json.RawMessage(`{"device_id":"dev-emit-1","approved_by":"@admin:test.com"}`),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	events := mock.getEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != EventDeviceApproved {
		t.Fatalf("expected event type %s, got %s", EventDeviceApproved, events[0].EventType)
	}
	if events[0].RoomID != testGovernanceRoom {
		t.Fatalf("expected room %s, got %s", testGovernanceRoom, events[0].RoomID)
	}

	var content map[string]interface{}
	if err := json.Unmarshal(events[0].Content, &content); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	if content["device_id"] != "dev-emit-1" {
		t.Fatalf("expected device_id dev-emit-1, got %v", content["device_id"])
	}
	if content["actor"] != "@admin:test.com" {
		t.Fatalf("expected actor @admin:test.com, got %v", content["actor"])
	}
}

func TestEmitDeviceEventOnReject(t *testing.T) {
	s, mock := newServerWithEvents(t)
	seedDevice(t, s.deviceStore, "dev-emit-2", trust.StateUnverified)

	_, rpcErr := s.handleDeviceReject(context.Background(), &Request{
		Params: json.RawMessage(`{"device_id":"dev-emit-2","rejected_by":"@admin:test.com","reason":"untrusted"}`),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	events := mock.getEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != EventDeviceRejected {
		t.Fatalf("expected event type %s, got %s", EventDeviceRejected, events[0].EventType)
	}
}

func TestEmitDeviceEventSkippedOnIdempotentApprove(t *testing.T) {
	s, mock := newServerWithEvents(t)
	seedDevice(t, s.deviceStore, "dev-emit-3", trust.StateVerified)

	_, rpcErr := s.handleDeviceApprove(context.Background(), &Request{
		Params: json.RawMessage(`{"device_id":"dev-emit-3","approved_by":"@admin:test.com"}`),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	events := mock.getEvents()
	if len(events) != 0 {
		t.Fatalf("expected 0 events for idempotent approve, got %d", len(events))
	}
}

func TestEmitDeviceEventSkippedOnIdempotentReject(t *testing.T) {
	s, mock := newServerWithEvents(t)
	seedDevice(t, s.deviceStore, "dev-emit-4", trust.StateRejected)

	_, rpcErr := s.handleDeviceReject(context.Background(), &Request{
		Params: json.RawMessage(`{"device_id":"dev-emit-4","rejected_by":"@admin:test.com"}`),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	events := mock.getEvents()
	if len(events) != 0 {
		t.Fatalf("expected 0 events for idempotent reject, got %d", len(events))
	}
}

func TestNoEventWhenMatrixNil(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	ds, _ := trust.NewDeviceStore(db)
	s := &Server{
		handlers:        make(map[string]HandlerFunc, 32),
		deviceStore:     ds,
		matrix:          nil,
		governanceRoomID: testGovernanceRoom,
	}
	seedDevice(t, ds, "dev-nil", trust.StateUnverified)

	_, rpcErr := s.handleDeviceApprove(context.Background(), &Request{
		Params: json.RawMessage(`{"device_id":"dev-nil","approved_by":"@admin:test.com"}`),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}
}

func TestNoEventWhenRoomIDEmpty(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	ds, _ := trust.NewDeviceStore(db)
	s := &Server{
		handlers:        make(map[string]HandlerFunc, 32),
		deviceStore:     ds,
		matrix:          &mockEventMatrixAdapter{},
		governanceRoomID: "",
	}
	seedDevice(t, ds, "dev-noroom", trust.StateUnverified)

	_, rpcErr := s.handleDeviceApprove(context.Background(), &Request{
		Params: json.RawMessage(`{"device_id":"dev-noroom","approved_by":"@admin:test.com"}`),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}
}

func TestEmitInviteEventOnCreate(t *testing.T) {
	s, mock := newServerWithEvents(t)

	_, rpcErr := s.handleInviteCreate(context.Background(), &Request{
		Params: json.RawMessage(`{"role":"user","expiration":"7d","created_by":"@admin:test.com"}`),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	events := mock.getEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != EventInviteCreated {
		t.Fatalf("expected event type %s, got %s", EventInviteCreated, events[0].EventType)
	}
	if events[0].RoomID != testGovernanceRoom {
		t.Fatalf("expected room %s, got %s", testGovernanceRoom, events[0].RoomID)
	}

	var content map[string]interface{}
	if err := json.Unmarshal(events[0].Content, &content); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	if content["actor"] != "@admin:test.com" {
		t.Fatalf("expected actor @admin:test.com, got %v", content["actor"])
	}
	if content["code"] == nil || content["code"] == "" {
		t.Fatal("expected non-empty code in event content")
	}
}

func TestEmitInviteEventOnRevoke(t *testing.T) {
	s, mock := newServerWithEvents(t)
	record := seedInvite(t, s.inviteStore, invite.RoleUser, "")

	_, rpcErr := s.handleInviteRevoke(context.Background(), &Request{
		Params: json.RawMessage(`{"invite_id":"` + record.ID + `","revoked_by":"@admin:test.com"}`),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	events := mock.getEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != EventInviteRevoked {
		t.Fatalf("expected event type %s, got %s", EventInviteRevoked, events[0].EventType)
	}

	var content map[string]interface{}
	if err := json.Unmarshal(events[0].Content, &content); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	if content["invite_id"] != record.ID {
		t.Fatalf("expected invite_id %s, got %v", record.ID, content["invite_id"])
	}
	if content["code"] != record.Code {
		t.Fatalf("expected code %s, got %v", record.Code, content["code"])
	}
}

func TestEmitInviteEventSkippedOnIdempotentRevoke(t *testing.T) {
	s, mock := newServerWithEvents(t)
	record := seedInvite(t, s.inviteStore, invite.RoleUser, invite.StatusRevoked)

	_, rpcErr := s.handleInviteRevoke(context.Background(), &Request{
		Params: json.RawMessage(`{"invite_id":"` + record.ID + `","revoked_by":"@admin:test.com"}`),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	events := mock.getEvents()
	if len(events) != 0 {
		t.Fatalf("expected 0 events for idempotent revoke, got %d", len(events))
	}
}

func TestNoEventOnInviteValidate(t *testing.T) {
	s, mock := newServerWithEvents(t)
	record := seedInvite(t, s.inviteStore, invite.RoleUser, "")

	_, rpcErr := s.handleInviteValidate(context.Background(), &Request{
		Params: json.RawMessage(`{"code":"` + record.Code + `"}`),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	events := mock.getEvents()
	if len(events) != 0 {
		t.Fatalf("expected 0 events for validate (read operation), got %d", len(events))
	}
}

func TestNoEventOnDeviceList(t *testing.T) {
	s, mock := newServerWithEvents(t)

	_, rpcErr := s.handleDeviceList(context.Background(), &Request{
		Params: json.RawMessage(`{}`),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	events := mock.getEvents()
	if len(events) != 0 {
		t.Fatalf("expected 0 events for list (read operation), got %d", len(events))
	}
}

func TestNoEventOnDeviceGet(t *testing.T) {
	s, mock := newServerWithEvents(t)
	seedDevice(t, s.deviceStore, "dev-get", trust.StateUnverified)

	_, rpcErr := s.handleDeviceGet(context.Background(), &Request{
		Params: json.RawMessage(`{"device_id":"dev-get"}`),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	events := mock.getEvents()
	if len(events) != 0 {
		t.Fatalf("expected 0 events for get (read operation), got %d", len(events))
	}
}

func TestNoEventOnInviteList(t *testing.T) {
	s, mock := newServerWithEvents(t)

	_, rpcErr := s.handleInviteList(context.Background(), &Request{
		Params: json.RawMessage(`{}`),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	events := mock.getEvents()
	if len(events) != 0 {
		t.Fatalf("expected 0 events for list (read operation), got %d", len(events))
	}
}

func TestEventConstants(t *testing.T) {
	if EventDeviceApproved != "app.armorclaw.device.approved" {
		t.Fatalf("unexpected EventDeviceApproved: %s", EventDeviceApproved)
	}
	if EventDeviceRejected != "app.armorclaw.device.rejected" {
		t.Fatalf("unexpected EventDeviceRejected: %s", EventDeviceRejected)
	}
	if EventInviteCreated != "app.armorclaw.invite.created" {
		t.Fatalf("unexpected EventInviteCreated: %s", EventInviteCreated)
	}
	if EventInviteRevoked != "app.armorclaw.invite.revoked" {
		t.Fatalf("unexpected EventInviteRevoked: %s", EventInviteRevoked)
	}
}
