package chatrelay

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/armorclaw/bridge/internal/ai"
)

// ---------------------------------------------------------------------------
// Mock: MessageSender
// ---------------------------------------------------------------------------

type sentMessage struct {
	roomID  string
	message string
	msgType string
}

type mockSender struct {
	mu       sync.Mutex
	messages []sentMessage
}

func (m *mockSender) SendMessageWithRetry(roomID, message, msgType string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, sentMessage{roomID, message, msgType})
	return "fake-event-id", nil
}

func (m *mockSender) getMessages() []sentMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]sentMessage(nil), m.messages...)
}

// ---------------------------------------------------------------------------
// Mock: AIChatFunc
// ---------------------------------------------------------------------------

type mockAIChat struct {
	response *ai.ChatResponse
	err      error
	called   bool
	mu       sync.Mutex
}

func (m *mockAIChat) call(ctx context.Context, req ai.ChatRequest, keyID string) (*ai.ChatResponse, error) {
	m.mu.Lock()
	m.called = true
	m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockAIChat) wasCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.called
}

// ---------------------------------------------------------------------------
// Helper: create a test handler with sensible defaults
// ---------------------------------------------------------------------------

func newTestHandler(aiChat AIChatFunc) (*Handler, *mockSender) {
	sender := &mockSender{}
	cfg := Config{
		Enabled:     true,
		RoomIDs:     []string{"!test:server"},
		MaxTokens:   256,
		Timeout:     5 * time.Second,
		MaxInFlight: 4,
	}
	h := NewHandler(cfg, aiChat, sender, "test-key-id", "@bridge:server")
	return h, sender
}

// ===========================================================================
// Tests
// ===========================================================================

// TestHandlerDisabled verifies that a disabled relay returns false for all messages.
func TestHandlerDisabled(t *testing.T) {
	aiMock := &mockAIChat{
		response: &ai.ChatResponse{Content: "hi"},
	}
	sender := &mockSender{}
	cfg := Config{
		Enabled:     false,
		RoomIDs:     []string{"!test:server"},
		MaxTokens:   256,
		Timeout:     5 * time.Second,
		MaxInFlight: 4,
	}
	h := NewHandler(cfg, aiMock.call, sender, "test-key-id", "@bridge:server")

	got := h.HandleMatrixMessage(context.Background(), "!test:server", "@user:server", "$event1", "hello")
	if got {
		t.Fatal("expected false when relay is disabled")
	}
	if aiMock.wasCalled() {
		t.Fatal("AI should not be called when relay is disabled")
	}
	if msgs := sender.getMessages(); len(msgs) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(msgs))
	}
}

// TestHandlerRoomNotEnabled verifies messages in non-allowlist rooms are rejected.
func TestHandlerRoomNotEnabled(t *testing.T) {
	aiMock := &mockAIChat{
		response: &ai.ChatResponse{Content: "hi"},
	}
	h, sender := newTestHandler(aiMock.call)

	got := h.HandleMatrixMessage(context.Background(), "!other:server", "@user:server", "$event1", "hello")
	if got {
		t.Fatal("expected false for non-allowlist room")
	}
	if aiMock.wasCalled() {
		t.Fatal("AI should not be called for non-allowlist room")
	}
	if msgs := sender.getMessages(); len(msgs) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(msgs))
	}
}

// TestHandlerCommandPrefix verifies messages starting with ! or / are rejected.
func TestHandlerCommandPrefix(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"bang prefix", "!command do something"},
		{"slash prefix", "/command do something"},
		{"bang only", "!"},
		{"slash only", "/"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Fresh handler per subtest to avoid dedup interference
			aiMock := &mockAIChat{response: &ai.ChatResponse{Content: "hi"}}
			h, sender := newTestHandler(aiMock.call)

			got := h.HandleMatrixMessage(context.Background(), "!test:server", "@user:server", "$evt_"+tc.name, tc.text)
			if got {
				t.Fatalf("expected false for command-prefixed text %q", tc.text)
			}
			if aiMock.wasCalled() {
				t.Fatal("AI should not be called for command-prefixed text")
			}
			if msgs := sender.getMessages(); len(msgs) != 0 {
				t.Fatalf("expected 0 messages, got %d", len(msgs))
			}
		})
	}
}

// TestHandlerEmptyBody verifies empty/whitespace messages are rejected.
func TestHandlerEmptyBody(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"empty string", ""},
		{"spaces only", "   "},
		{"tabs only", "\t\t"},
		{"newlines only", "\n\n"},
		{"mixed whitespace", " \t\n "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			aiMock := &mockAIChat{response: &ai.ChatResponse{Content: "hi"}}
			h, sender := newTestHandler(aiMock.call)

			got := h.HandleMatrixMessage(context.Background(), "!test:server", "@user:server", "$evt_"+tc.name, tc.text)
			if got {
				t.Fatalf("expected false for empty body %q", tc.text)
			}
			if aiMock.wasCalled() {
				t.Fatal("AI should not be called for empty body")
			}
			if msgs := sender.getMessages(); len(msgs) != 0 {
				t.Fatalf("expected 0 messages, got %d", len(msgs))
			}
		})
	}
}

// TestHandlerPlainText verifies the success path: plain text in an enabled room
// triggers AI call and response is sent as m.text.
func TestHandlerPlainText(t *testing.T) {
	aiMock := &mockAIChat{
		response: &ai.ChatResponse{Content: "Hello from AI!"},
	}
	h, sender := newTestHandler(aiMock.call)

	got := h.HandleMatrixMessage(context.Background(), "!test:server", "@user:server", "$event1", "hello")
	if !got {
		t.Fatal("expected true for valid plain-text message")
	}

	// Wait for async goroutine to finish
	time.Sleep(200 * time.Millisecond)

	if !aiMock.wasCalled() {
		t.Fatal("expected AI to be called")
	}

	msgs := sender.getMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].msgType != "m.text" {
		t.Fatalf("expected msgType m.text, got %q", msgs[0].msgType)
	}
	if msgs[0].message != "Hello from AI!" {
		t.Fatalf("expected message 'Hello from AI!', got %q", msgs[0].message)
	}
	if msgs[0].roomID != "!test:server" {
		t.Fatalf("expected roomID '!test:server', got %q", msgs[0].roomID)
	}
}

// TestHandlerAIError verifies that when AI returns an error, a safe m.notice is sent
// with a relay_ correlation ID and no secrets are leaked.
func TestHandlerAIError(t *testing.T) {
	aiMock := &mockAIChat{
		err: context.DeadlineExceeded,
	}
	h, sender := newTestHandler(aiMock.call)

	got := h.HandleMatrixMessage(context.Background(), "!test:server", "@user:server", "$event1", "hello")
	if !got {
		t.Fatal("expected true even when AI errors (message is consumed)")
	}

	// Wait for async goroutine
	time.Sleep(200 * time.Millisecond)

	msgs := sender.getMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	msg := msgs[0]

	if msg.msgType != "m.notice" {
		t.Fatalf("expected msgType m.notice for error, got %q", msg.msgType)
	}

	if !strings.Contains(msg.message, "relay_") {
		t.Fatalf("expected error message to contain 'relay_' correlation ID, got %q", msg.message)
	}

	// Verify no secrets are leaked
	forbidden := []string{"api_key", "api key", "token", "password"}
	lowered := strings.ToLower(msg.message)
	for _, word := range forbidden {
		if strings.Contains(lowered, word) {
			t.Fatalf("error message must not contain %q, got: %q", word, msg.message)
		}
	}
}

// TestHandlerSelfMessage verifies that messages from the bot itself are rejected
// to prevent infinite loops.
func TestHandlerSelfMessage(t *testing.T) {
	aiMock := &mockAIChat{
		response: &ai.ChatResponse{Content: "hi"},
	}
	h, sender := newTestHandler(aiMock.call)

	got := h.HandleMatrixMessage(context.Background(), "!test:server", "@bridge:server", "$event1", "hello")
	if got {
		t.Fatal("expected false for self-message (bot loop prevention)")
	}
	if aiMock.wasCalled() {
		t.Fatal("AI should not be called for self-messages")
	}
	if msgs := sender.getMessages(); len(msgs) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(msgs))
	}
}

// TestHandlerEventDedup verifies that duplicate eventIDs are rejected after first consumption.
func TestHandlerEventDedup(t *testing.T) {
	aiMock := &mockAIChat{
		response: &ai.ChatResponse{Content: "hi"},
	}
	h, sender := newTestHandler(aiMock.call)

	// First call should be consumed
	got1 := h.HandleMatrixMessage(context.Background(), "!test:server", "@user:server", "$dup-event", "hello")
	if !got1 {
		t.Fatal("first call should return true")
	}

	// Second call with same eventID should be rejected
	got2 := h.HandleMatrixMessage(context.Background(), "!test:server", "@user:server", "$dup-event", "hello")
	if got2 {
		t.Fatal("duplicate eventID should return false")
	}

	// Wait for async goroutine from first call
	time.Sleep(200 * time.Millisecond)

	msgs := sender.getMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected exactly 1 message (no dedup replay), got %d", len(msgs))
	}
}

// TestHandlerConcurrentLimit verifies the handler doesn't panic under concurrent calls.
func TestHandlerConcurrentLimit(t *testing.T) {
	aiMock := &mockAIChat{
		response: &ai.ChatResponse{Content: "ok"},
	}
	h, _ := newTestHandler(aiMock.call)

	var wg sync.WaitGroup
	const numGoroutines = 10
	panicCh := make(chan interface{}, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCh <- r
				}
			}()
			h.HandleMatrixMessage(context.Background(), "!test:server", "@user:server",
				"$concurrent-event-"+string(rune('0'+idx)),
				"message")
		}(i)
	}

	wg.Wait()
	close(panicCh)

	for r := range panicCh {
		t.Fatalf("handler panicked under concurrent calls: %v", r)
	}
}

// TestHandlerBackpressure verifies that when MaxInFlight goroutines are active,
// the next message gets a safe m.notice "busy" response.
func TestHandlerBackpressure(t *testing.T) {
	blockCh := make(chan struct{})

	blockingAI := func(ctx context.Context, req ai.ChatRequest, keyID string) (*ai.ChatResponse, error) {
		<-blockCh // Block until test releases
		return &ai.ChatResponse{Content: "delayed response"}, nil
	}

	sender := &mockSender{}
	cfg := Config{
		Enabled:     true,
		RoomIDs:     []string{"!test:server"},
		MaxTokens:   256,
		Timeout:     5 * time.Second,
		MaxInFlight: 1, // Only one concurrent AI call allowed
	}
	h := NewHandler(cfg, blockingAI, sender, "test-key-id", "@bridge:server")

	// First message: will block in AI goroutine, consuming the semaphore slot
	got1 := h.HandleMatrixMessage(context.Background(), "!test:server", "@user:server", "$bp-event-1", "first message")
	if !got1 {
		t.Fatal("first message should be consumed")
	}

	// Give the goroutine time to acquire the semaphore
	time.Sleep(100 * time.Millisecond)

	// Second message: semaphore full → backpressure response
	got2 := h.HandleMatrixMessage(context.Background(), "!test:server", "@user:server", "$bp-event-2", "second message")
	if !got2 {
		t.Fatal("second message should still be consumed (returns true, sends busy notice)")
	}

	// Wait for the busy notice to be sent
	time.Sleep(200 * time.Millisecond)

	msgs := sender.getMessages()
	if len(msgs) == 0 {
		t.Fatal("expected at least one message (busy notice)")
	}

	// Find the busy notice
	var busyMsg *sentMessage
	for i := range msgs {
		if strings.Contains(msgs[i].message, "busy") {
			busyMsg = &msgs[i]
			break
		}
	}
	if busyMsg == nil {
		t.Fatalf("expected a 'busy' m.notice message, got: %v", msgs)
	}
	if busyMsg.msgType != "m.notice" {
		t.Fatalf("expected busy message msgType m.notice, got %q", busyMsg.msgType)
	}
	if !strings.Contains(busyMsg.message, "relay_") {
		t.Fatalf("expected busy message to contain 'relay_' correlation ID, got %q", busyMsg.message)
	}

	// Release the blocked AI goroutine and wait for it to complete
	close(blockCh)
	time.Sleep(200 * time.Millisecond)

	msgs = sender.getMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages total (busy notice + delayed response), got %d", len(msgs))
	}
}
