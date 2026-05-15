package websocket

import (
	"strings"
	"testing"
	"time"
)

type mockBroadcaster struct {
	calls []broadcastCall
}
type broadcastCall struct {
	eventType string
	payload   []byte
}

func (m *mockBroadcaster) BroadcastEvent(eventType string, payload []byte) {
	m.calls = append(m.calls, broadcastCall{eventType: eventType, payload: payload})
}

func TestNewServer(t *testing.T) {
	cfg := Config{
		Addr:              ":9090",
		Path:              "/ws",
		AllowedOrigins:    []string{"*"},
		MaxConnections:    10,
		InactivityTimeout: 30 * time.Second,
	}
	s := NewServer(cfg)

	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.addr != ":9090" {
		t.Errorf("addr = %q, want %q", s.addr, ":9090")
	}
	if s.config.Path != "/ws" {
		t.Errorf("config.Path = %q, want %q", s.config.Path, "/ws")
	}
	if s.started {
		t.Error("started should be false initially")
	}
	if s.broadcaster != nil {
		t.Error("broadcaster should be nil initially")
	}
}

func TestStart_NoBroadcaster_ReturnsError(t *testing.T) {
	s := NewServer(Config{Addr: ":8080"})

	err := s.Start()
	if err == nil {
		t.Fatal("expected error when starting without broadcaster")
	}

	want := "no EventBroadcaster wired"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want to contain %q", err.Error(), want)
	}
}

func TestStart_WithBroadcaster_Success(t *testing.T) {
	s := NewServer(Config{Addr: ":8080"})
	s.SetBroadcaster(&mockBroadcaster{})

	err := s.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s.mu.RLock()
	started := s.started
	s.mu.RUnlock()

	if !started {
		t.Error("started should be true after Start()")
	}
}

func TestBroadcast_NoBroadcaster_ReturnsError(t *testing.T) {
	s := NewServer(Config{Addr: ":8080"})

	err := s.Broadcast([]byte("hello"))
	if err == nil {
		t.Fatal("expected error when broadcasting without broadcaster")
	}

	want := "no EventBroadcaster wired"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want to contain %q", err.Error(), want)
	}
}

func TestBroadcast_WithBroadcaster_Delegates(t *testing.T) {
	mb := &mockBroadcaster{}
	s := NewServer(Config{Addr: ":8080"})
	s.SetBroadcaster(mb)

	msg := []byte(`{"type":"test"}`)
	err := s.Broadcast(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mb.calls) != 1 {
		t.Fatalf("expected 1 broadcast call, got %d", len(mb.calls))
	}
	if string(mb.calls[0].payload) != string(msg) {
		t.Errorf("payload = %q, want %q", mb.calls[0].payload, msg)
	}
}

func TestStop_SetsStartedFalse(t *testing.T) {
	s := NewServer(Config{Addr: ":8080"})
	s.SetBroadcaster(&mockBroadcaster{})

	if err := s.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	s.mu.RLock()
	if !s.started {
		t.Fatal("should be started before Stop()")
	}
	s.mu.RUnlock()

	if err := s.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	s.mu.RLock()
	started := s.started
	s.mu.RUnlock()

	if started {
		t.Error("started should be false after Stop()")
	}
}

func TestAddr(t *testing.T) {
	s := NewServer(Config{Addr: "192.168.1.1:9999"})

	if got := s.Addr(); got != "192.168.1.1:9999" {
		t.Errorf("Addr() = %q, want %q", got, "192.168.1.1:9999")
	}
}

func TestPath(t *testing.T) {
	s := NewServer(Config{Path: "/custom-ws"})

	if got := s.Path(); got != "/custom-ws" {
		t.Errorf("Path() = %q, want %q", got, "/custom-ws")
	}
}

func TestSetBroadcaster_NilThenStartFails(t *testing.T) {
	s := NewServer(Config{Addr: ":8080"})

	err := s.Start()
	if err == nil {
		t.Fatal("expected error without broadcaster")
	}
}

func TestErrNoBroadcaster_Type(t *testing.T) {
	err := errNoBroadcaster()
	if err == nil {
		t.Fatal("errNoBroadcaster returned nil")
	}

	var target *noBroadcasterError
	if !isNoBroadcasterError(err) {
		t.Error("error should be *noBroadcasterError")
	}
	_ = target
}

func isNoBroadcasterError(err error) bool {
	_, ok := err.(*noBroadcasterError)
	return ok
}
