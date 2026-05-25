package chatrelay

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/armorclaw/bridge/internal/adapter"
	"github.com/armorclaw/bridge/internal/ai"
)

// Compile-time check that Handler implements StudioCommandHandler.
var _ adapter.StudioCommandHandler = (*Handler)(nil)

// Handler receives plain-text Matrix messages, filters them, and spawns
// async goroutines to call the AI and reply back to the room.
type Handler struct {
	config       Config
	aiChat       AIChatFunc
	sender       MessageSender
	defaultKeyID string
	botMXID      string
	seenEvents   map[string]bool
	mu           sync.Mutex
	sem          chan struct{} // bounded semaphore for MaxInFlight
	logger       *slog.Logger
}

// NewHandler creates a ready-to-use chat relay handler.
func NewHandler(config Config, aiChat AIChatFunc, sender MessageSender, defaultKeyID string, botMXID string) *Handler {
	return &Handler{
		config:       config,
		aiChat:       aiChat,
		sender:       sender,
		defaultKeyID: defaultKeyID,
		botMXID:      botMXID,
		seenEvents:   make(map[string]bool),
		sem:          make(chan struct{}, config.MaxInFlight),
		logger:       slog.Default().With("component", "chatrelay"),
	}
}

// HandleMatrixMessage implements StudioCommandHandler. It filters the message
// synchronously and returns true immediately when consumed, then processes
// the AI call asynchronously in a bounded goroutine.
func (h *Handler) HandleMatrixMessage(ctx context.Context, roomID, userID, eventID, text string) bool {
	// Step 1: Check enabled
	if !h.config.Enabled {
		return false
	}

	// Step 2: Check room allowlist
	if !h.config.IsRoomEnabled(roomID) {
		return false
	}

	// Step 3: Self-message filtering (prevent bot loop)
	if userID == h.botMXID {
		return false
	}

	// Step 4: Event dedup (protect against Matrix retry)
	h.mu.Lock()
	if h.seenEvents[eventID] {
		h.mu.Unlock()
		return false
	}
	h.seenEvents[eventID] = true
	h.mu.Unlock()

	// Step 5: Filter commands (!prefix or /prefix)
	if strings.HasPrefix(text, "!") || strings.HasPrefix(text, "/") {
		return false
	}

	// Step 6: Filter empty/whitespace
	if strings.TrimSpace(text) == "" {
		return false
	}

	// Step 7: Return true immediately to consume the event.
	// Step 8: Spawn async goroutine (bounded by semaphore).
	go h.processMessage(roomID, eventID, text)

	return true
}

// processMessage runs in its own goroutine, bounded by the semaphore.
func (h *Handler) processMessage(roomID, eventID, text string) {
	// Try to acquire semaphore (non-blocking)
	select {
	case h.sem <- struct{}{}:
		// Got a slot
		defer func() { <-h.sem }()
	default:
		// Backpressure: max in-flight reached
		corrID := generateCorrelationID()
		h.logger.Warn("relay backpressure: max_in_flight reached, dropping message",
			"room_id", roomID, "event_id", eventID, "correlation_id", corrID)
		errMsg := "ArmorClaw is busy, please try again later. Reference: relay_" + corrID
		h.sender.SendMessageWithRetry(roomID, errMsg, "m.notice")
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), h.config.Timeout)
	defer cancel()

	start := time.Now()

	// Build AI request
	req := ai.ChatRequest{
		Model:     h.config.Model, // empty = AIService resolves default
		Messages:  []ai.Message{{Role: "user", Content: text}},
		MaxTokens: h.config.MaxTokens,
	}

	// Call AI
	resp, err := h.aiChat(ctx, req, h.defaultKeyID)
	latency := time.Since(start)

	if err != nil {
		corrID := generateCorrelationID()
		h.logger.Error("chat relay AI error",
			"room_id", roomID, "event_id", eventID, "correlation_id", corrID,
			"latency_ms", latency.Milliseconds(), "error", err)
		errMsg := "ArmorClaw received your message, but the Secretary could not generate a reply right now. Reference: relay_" + corrID
		h.sender.SendMessageWithRetry(roomID, errMsg, "m.notice")
		return
	}

	// Success: send as m.text (chat message, not system notification)
	h.logger.Info("chat relay response sent",
		"room_id", roomID, "event_id", eventID,
		"latency_ms", latency.Milliseconds(), "model", resp.Model)
	h.sender.SendMessageWithRetry(roomID, resp.Content, "m.text")
}

// generateCorrelationID returns an 8-character hex string for error correlation.
func generateCorrelationID() string {
	b := make([]byte, 4) // 4 bytes = 8 hex chars
	rand.Read(b)
	return hex.EncodeToString(b)
}
