// Package agent provides the JetskiStateEventSubscriber which connects to
// a Jetski SSE event stream, parses CDP events, and wires them to the
// Bridge-side state inference engine.
//
// Lifecycle:
//   - Start(ctx) → POST registration handshake → read SSE stream → convert
//     SubscriberEvent → CDPEvent → InferAgentState → ApplyInferredState
//   - Stop() → cancel context, tear down connection
//   - Auto-reconnect with exponential backoff on connection loss
package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// JetskiSubscriberConfig configures the Jetski SSE subscriber.
type JetskiSubscriberConfig struct {
	// JetskiURL is the base URL of the Jetski RPC server (e.g. "http://localhost:9223").
	JetskiURL string

	// DeviceID is the identifier used in the SSE registration handshake.
	// Convention: "bridge" for the primary Bridge subscriber.
	DeviceID string

	// Coordinator is the AgentCoordinator that manages agent state machines.
	// Events are routed to the appropriate agent's state machine.
	Coordinator *AgentCoordinator
}

// JetskiStateEventSubscriber connects to a Jetski CDP event SSE stream,
// parses incoming CDP events, and applies state inference to registered agents.
type JetskiStateEventSubscriber struct {
	cfg    JetskiSubscriberConfig
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	// HTTP client (exported for testing overrides)
	HTTPClient *http.Client

	// connected tracks whether we have an active SSE connection.
	connected bool

	// reconnectBackoff tracks the current backoff duration.
	reconnectBackoff time.Duration
}

// NewJetskiStateEventSubscriber creates a new subscriber with the given config.
func NewJetskiStateEventSubscriber(cfg JetskiSubscriberConfig) (*JetskiStateEventSubscriber, error) {
	if cfg.JetskiURL == "" {
		return nil, fmt.Errorf("jetski_subscriber: JetskiURL is required")
	}
	if cfg.DeviceID == "" {
		return nil, fmt.Errorf("jetski_subscriber: DeviceID is required")
	}
	if cfg.Coordinator == nil {
		return nil, fmt.Errorf("jetski_subscriber: Coordinator is required")
	}

	return &JetskiStateEventSubscriber{
		cfg:             cfg,
		HTTPClient:      &http.Client{Timeout: 0}, // no timeout for SSE
		reconnectBackoff: time.Second,
	}, nil
}

// subscribeRequest matches Jetski's expected registration format.
type subscribeRequest struct {
	Type    string `json:"type"`
	Payload struct {
		DeviceID string `json:"device_id"`
	} `json:"payload"`
}

// subscriberEvent mirrors Jetski's cdp.SubscriberEvent JSON structure.
// We define it locally to avoid importing the Jetski package.
type subscriberEvent struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// Start begins the SSE subscription loop. It blocks until the context is
// cancelled. Use Stop() to shut down cleanly.
//
// The subscription flow:
//  1. POST registration handshake to /rpc/events.subscribe
//  2. Read SSE stream: parse "event: cdp\ndata: {...}\n\n" lines
//  3. Unmarshal each data payload as subscriberEvent
//  4. Convert to CDPEvent (direct mapping — same structure)
//  5. Route to agent via session correlation
//  6. Call ApplyInferredState on the agent's state machine
func (s *JetskiStateEventSubscriber) Start(ctx context.Context) error {
	s.mu.Lock()
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		default:
		}

		err := s.connectAndServe()
		if err != nil {
			log.Printf("[JETSKI SUBSCRIBER]: connection error: %v", err)
		}

		// Check if we should stop before reconnecting.
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		default:
		}

		s.mu.Lock()
		backoff := s.reconnectBackoff
		s.reconnectBackoff = time.Duration(math.Min(float64(s.reconnectBackoff*2), float64(30*time.Second)))
		s.connected = false
		s.mu.Unlock()

		log.Printf("[JETSKI SUBSCRIBER]: reconnecting in %v...", backoff)

		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-time.After(backoff):
			// Continue to reconnect
		}
	}
}

// Stop cancels the subscription context and tears down the connection.
func (s *JetskiStateEventSubscriber) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	s.connected = false
}

// Connected returns whether the subscriber has an active SSE connection.
func (s *JetskiStateEventSubscriber) Connected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

// connectAndServe performs a single SSE connection attempt: registration
// handshake followed by event processing. Returns on connection loss or
// context cancellation.
func (s *JetskiStateEventSubscriber) connectAndServe() error {
	// Build registration request.
	reqBody := subscribeRequest{
		Type: "register",
	}
	reqBody.Payload.DeviceID = s.cfg.DeviceID

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal subscribe request: %w", err)
	}

	endpoint := strings.TrimRight(s.cfg.JetskiURL, "/") + "/rpc/events.subscribe"

	req, err := http.NewRequestWithContext(s.ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create subscribe request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("subscribe request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("subscribe rejected (status %d): %s", resp.StatusCode, string(respBody))
	}

	s.mu.Lock()
	s.connected = true
	s.reconnectBackoff = time.Second // reset backoff on successful connect
	s.mu.Unlock()

	log.Printf("[JETSKI SUBSCRIBER]: connected to %s as device %s", endpoint, s.cfg.DeviceID)

	// Process SSE stream.
	return s.processSSEStream(resp.Body)
}

// processSSEStream reads the SSE response body, parses events, and routes
// them to the state inference engine.
func (s *JetskiStateEventSubscriber) processSSEStream(reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line

	var currentEvent string
	var currentData string

	for scanner.Scan() {
		line := scanner.Text()

		// Empty line = end of SSE event
		if line == "" {
			if currentEvent != "" && currentData != "" {
				if err := s.handleSSEEvent(currentEvent, currentData); err != nil {
					log.Printf("[JETSKI SUBSCRIBER]: error handling event: %v", err)
				}
			}
			currentEvent = ""
			currentData = ""
			continue
		}

		// Parse SSE fields.
		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			currentData = strings.TrimPrefix(line, "data: ")
		}
		// Ignore "id:", "retry:", and comment lines (starting with ":")
	}

	if err := scanner.Err(); err != nil {
		// Context cancellation is expected on Stop().
		if s.ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("SSE stream read error: %w", err)
	}

	return nil
}

// handleSSEEvent processes a single parsed SSE event.
func (s *JetskiStateEventSubscriber) handleSSEEvent(eventType, data string) error {
	switch eventType {
	case "registered":
		log.Printf("[JETSKI SUBSCRIBER]: registration confirmed: %s", data)
		return nil
	case "cdp":
		return s.handleCDPEvent(data)
	default:
		// Ignore unknown event types
		return nil
	}
}

// handleCDPEvent parses a CDP SSE data payload, converts it to a CDPEvent,
// and applies state inference to all registered agents.
func (s *JetskiStateEventSubscriber) handleCDPEvent(data string) error {
	var subEvt subscriberEvent
	if err := json.Unmarshal([]byte(data), &subEvt); err != nil {
		return fmt.Errorf("unmarshal CDP event: %w", err)
	}

	// Convert SubscriberEvent → CDPEvent (same structure, zero-copy).
	cdpEvt := CDPEvent{
		Method: subEvt.Method,
		Params: subEvt.Params,
	}

	// Route to all active agents.
	// In the current architecture, events are broadcast to all agents
	// because session correlation is not yet available. Each agent's
	// InferAgentState handles event filtering (unknown events = no-op).
	statuses := s.cfg.Coordinator.GetAllStatuses()
	for _, status := range statuses {
		integration, err := s.cfg.Coordinator.GetAgent(status.AgentID)
		if err != nil {
			continue // agent may have been unregistered
		}

		// ApplyInferredState combines InferAgentState + ForceTransition.
		// It returns true if the state changed.
		changed := ApplyInferredState(
			integration.stateMachine,
			[]CDPEvent{cdpEvt},
			WorkflowStatus{}, // no workflow side-channel yet
		)

		if changed {
			log.Printf("[JETSKI SUBSCRIBER]: agent %s state changed via CDP event %s",
				status.AgentID, cdpEvt.Method)
		}
	}

	return nil
}

// ParseSSEEvent is a helper function for parsing SSE text data into a CDPEvent.
// Exported for testing.
func ParseSSEEvent(data string) (*CDPEvent, error) {
	var subEvt subscriberEvent
	if err := json.Unmarshal([]byte(data), &subEvt); err != nil {
		return nil, fmt.Errorf("parse SSE event: %w", err)
	}

	return &CDPEvent{
		Method: subEvt.Method,
		Params: subEvt.Params,
	}, nil
}

// BuildRegistrationBody creates the JSON body for the SSE registration handshake.
// Exported for testing.
func BuildRegistrationBody(deviceID string) ([]byte, error) {
	req := subscribeRequest{
		Type: "register",
	}
	req.Payload.DeviceID = deviceID
	return json.Marshal(req)
}
