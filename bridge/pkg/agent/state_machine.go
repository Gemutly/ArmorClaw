package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// TransitionRecord captures a single state transition for observability.
type TransitionRecord struct {
	From         AgentStatus `json:"from"`
	To           AgentStatus `json:"to"`
	Timestamp    int64       `json:"timestamp"`
	Reason       string      `json:"reason,omitempty"`
	InferredFrom string      `json:"inferred_from,omitempty"` // "CDP", "workflow", "manual", or ""
}

const transitionLogMaxSize = 100

// StateMachine manages agent state transitions with validation and event emission.
// It is safe for concurrent use.
type StateMachine struct {
	mu        sync.RWMutex
	current   AgentStatus
	agentID   string
	metadata  StatusMetadata
	startTime time.Time

	// Event handling
	eventChan   chan StatusEvent
	subscribers []chan StatusEvent

	// History for reconnection support
	history     []StatusEvent
	historySize int

	// Transition log for observability (ring buffer of last 100)
	transitionLog []TransitionRecord

	// Context for shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// StateMachineConfig holds configuration for creating a state machine.
type StateMachineConfig struct {
	AgentID     string
	HistorySize int
}

// NewStateMachine creates a new state machine for an agent.
func NewStateMachine(cfg StateMachineConfig) *StateMachine {
	if cfg.HistorySize == 0 {
		cfg.HistorySize = 100
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &StateMachine{
		current:     StatusOffline,
		agentID:     cfg.AgentID,
		historySize: cfg.HistorySize,
		eventChan:   make(chan StatusEvent, 100),
		history:     make([]StatusEvent, 0, cfg.HistorySize),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Transition attempts to change state. Returns error if invalid.
// On success, emits a StatusEvent to all subscribers.
func (sm *StateMachine) Transition(newStatus AgentStatus, metadata ...StatusMetadata) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Validate transition
	if err := ValidateTransition(sm.current, newStatus); err != nil {
		return err
	}

	// Record transition
	prev := sm.current
	sm.current = newStatus
	var meta StatusMetadata
	if len(metadata) > 0 {
		meta = metadata[0]
		sm.metadata = meta
	} else {
		// Clear sensitive metadata on terminal states
		if newStatus.IsTerminal() || newStatus == StatusIdle || newStatus == StatusComplete {
			sm.metadata = StatusMetadata{}
		}
	}

	// Create event
	event := StatusEvent{
		AgentID:   sm.agentID,
		Status:    newStatus,
		Previous:  prev,
		Metadata:  sm.metadata,
		Timestamp: time.Now().UnixMilli(),
	}

	// Add to history
	sm.history = append(sm.history, event)
	if len(sm.history) > sm.historySize {
		sm.history = sm.history[1:]
	}

	inferredFrom := meta.InferredFrom
	if inferredFrom == "" {
		inferredFrom = "manual"
	}

	sm.transitionLog = append(sm.transitionLog, TransitionRecord{
		From:         prev,
		To:           newStatus,
		Timestamp:    event.Timestamp,
		InferredFrom: inferredFrom,
	})
	if len(sm.transitionLog) > transitionLogMaxSize {
		sm.transitionLog = sm.transitionLog[len(sm.transitionLog)-transitionLogMaxSize:]
	}

	slog.Info("[STATE TRANSITION]",
		"agent_id", sm.agentID,
		"from", string(prev),
		"to", string(newStatus),
		"inferred_from", inferredFrom,
		"timestamp", event.Timestamp,
	)

	// Emit to channel (non-blocking)
	select {
	case sm.eventChan <- event:
	default:
		// Channel full, drop oldest event
		select {
		case <-sm.eventChan:
			sm.eventChan <- event
		default:
		}
	}

	// Notify subscribers
	for _, sub := range sm.subscribers {
		select {
		case sub <- event:
		default:
			// Subscriber channel full, skip
		}
	}

	return nil
}

// Current returns the current state.
func (sm *StateMachine) Current() AgentStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.current
}

// Metadata returns the current metadata.
func (sm *StateMachine) Metadata() StatusMetadata {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.metadata
}

// Events returns the event channel for consuming status changes.
// The channel is closed when Shutdown() is called.
func (sm *StateMachine) Events() <-chan StatusEvent {
	return sm.eventChan
}

// Subscribe creates a new subscriber channel for status events.
// The caller must call Unsubscribe to clean up.
func (sm *StateMachine) Subscribe() <-chan StatusEvent {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	ch := make(chan StatusEvent, 50)
	sm.subscribers = append(sm.subscribers, ch)
	return ch
}

// Unsubscribe removes a subscriber channel.
func (sm *StateMachine) Unsubscribe(ch <-chan StatusEvent) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for i, sub := range sm.subscribers {
		// Compare the receive-only channel with the send channel
		if sub == ch {
			sm.subscribers = append(sm.subscribers[:i], sm.subscribers[i+1:]...)
			close(sub)
			return
		}
	}
}

// History returns recent status events for reconnection support.
func (sm *StateMachine) History(limit int) []StatusEvent {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if limit <= 0 || limit > len(sm.history) {
		limit = len(sm.history)
	}

	// Return copy to avoid races
	result := make([]StatusEvent, limit)
	copy(result, sm.history[len(sm.history)-limit:])
	return result
}

// LastEvent returns the most recent status event.
func (sm *StateMachine) LastEvent() *StatusEvent {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.history) == 0 {
		return nil
	}

	event := sm.history[len(sm.history)-1]
	return &event
}

// RecentTransitions returns the last n transition records for debugging.
// If n > available records, all records are returned.
func (sm *StateMachine) RecentTransitions(n int) []TransitionRecord {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if n <= 0 || n > len(sm.transitionLog) {
		n = len(sm.transitionLog)
	}

	result := make([]TransitionRecord, n)
	copy(result, sm.transitionLog[len(sm.transitionLog)-n:])
	return result
}

// ForceTransition bypasses validation (for recovery scenarios).
// Use with caution - prefer Transition() for normal operations.
func (sm *StateMachine) ForceTransition(newStatus AgentStatus, metadata ...StatusMetadata) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	prev := sm.current
	sm.current = newStatus
	var meta StatusMetadata
	if len(metadata) > 0 {
		meta = metadata[0]
		sm.metadata = meta
	}

	event := StatusEvent{
		AgentID:   sm.agentID,
		Status:    newStatus,
		Previous:  prev,
		Metadata:  sm.metadata,
		Timestamp: time.Now().UnixMilli(),
	}

	sm.history = append(sm.history, event)
	if len(sm.history) > sm.historySize {
		sm.history = sm.history[1:]
	}

	inferredFrom := meta.InferredFrom
	if inferredFrom == "" {
		inferredFrom = "manual"
	}

	sm.transitionLog = append(sm.transitionLog, TransitionRecord{
		From:         prev,
		To:           newStatus,
		Timestamp:    event.Timestamp,
		InferredFrom: inferredFrom,
	})
	if len(sm.transitionLog) > transitionLogMaxSize {
		sm.transitionLog = sm.transitionLog[len(sm.transitionLog)-transitionLogMaxSize:]
	}

	slog.Info("[STATE TRANSITION]",
		"agent_id", sm.agentID,
		"from", string(prev),
		"to", string(newStatus),
		"inferred_from", inferredFrom,
		"timestamp", event.Timestamp,
	)

	// Emit non-blocking
	select {
	case sm.eventChan <- event:
	default:
	}
}

// Initialize transitions from OFFLINE to INITIALIZING.
func (sm *StateMachine) Initialize() error {
	return sm.Transition(StatusInitializing)
}

// SetReady transitions from INITIALIZING to IDLE.
func (sm *StateMachine) SetReady() error {
	return sm.Transition(StatusIdle)
}

// StartBrowsing transitions to BROWSING with URL metadata.
func (sm *StateMachine) StartBrowsing(url string) error {
	return sm.Transition(StatusBrowsing, StatusMetadata{
		URL: url,
	})
}

// StartFormFilling transitions to FORM_FILLING with step info.
func (sm *StateMachine) StartFormFilling(step string, progress int) error {
	return sm.Transition(StatusFormFilling, StatusMetadata{
		Step:     step,
		Progress: progress,
	})
}

// RequestApproval transitions to AWAITING_APPROVAL with field list.
func (sm *StateMachine) RequestApproval(fields []string) error {
	return sm.Transition(StatusAwaitingApproval, StatusMetadata{
		FieldsRequested: fields,
	})
}

// WaitForCaptcha transitions to AWAITING_CAPTCHA.
func (sm *StateMachine) WaitForCaptcha() error {
	return sm.Transition(StatusAwaitingCaptcha)
}

// WaitFor2FA transitions to AWAITING_2FA.
func (sm *StateMachine) WaitFor2FA() error {
	return sm.Transition(StatusAwaiting2FA)
}

// StartPayment transitions to PROCESSING_PAYMENT.
func (sm *StateMachine) StartPayment() error {
	return sm.Transition(StatusProcessingPayment)
}

// Complete transitions to COMPLETE.
func (sm *StateMachine) Complete() error {
	return sm.Transition(StatusComplete)
}

// Fail transitions to ERROR with an error message.
func (sm *StateMachine) Fail(err error) error {
	return sm.Transition(StatusError, StatusMetadata{
		Error: err.Error(),
	})
}

// FailWithString transitions to ERROR with a string message.
func (sm *StateMachine) FailWithString(msg string) error {
	return sm.Transition(StatusError, StatusMetadata{
		Error: msg,
	})
}

// Reset transitions from ERROR to IDLE.
func (sm *StateMachine) Reset() error {
	return sm.Transition(StatusIdle)
}

// Shutdown stops the state machine and closes all channels.
func (sm *StateMachine) Shutdown() {
	sm.cancel()

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Close main event channel
	close(sm.eventChan)

	// Close subscriber channels
	for _, sub := range sm.subscribers {
		close(sub)
	}
	sm.subscribers = nil
}

// Done returns a channel that's closed when the state machine is shut down.
func (sm *StateMachine) Done() <-chan struct{} {
	return sm.ctx.Done()
}

// AgentID returns the agent ID for this state machine.
func (sm *StateMachine) AgentID() string {
	return sm.agentID
}

// String returns a human-readable status string.
func (sm *StateMachine) String() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.metadata.URL != "" {
		return fmt.Sprintf("%s (%s)", sm.current, sm.metadata.URL)
	}
	if sm.metadata.Step != "" {
		return fmt.Sprintf("%s (step: %s)", sm.current, sm.metadata.Step)
	}
	return string(sm.current)
}
