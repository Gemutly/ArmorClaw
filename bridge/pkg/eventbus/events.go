// Package eventbus provides event types for ArmorClaw bridge events
// These events are broadcast to WebSocket clients for real-time updates
package eventbus

import (
	"encoding/json"
	"fmt"
	"time"
)

// EventType constants for all bridge events
const (
	// Matrix events (existing)
	EventTypeMatrixMessage  = "matrix.message"
	EventTypeMatrixReceipt  = "matrix.receipt"
	EventTypeMatrixTyping   = "matrix.typing"
	EventTypeMatrixPresence = "matrix.presence"

	// Agent events (new)
	EventTypeAgentStarted       = "agent.started"
	EventTypeAgentStopped       = "agent.stopped"
	EventTypeAgentStatusChanged = "agent.status_changed"
	EventTypeAgentCommand       = "agent.command"
	EventTypeAgentError         = "agent.error"

	// Workflow events (new)
	EventTypeWorkflowStarted   = "workflow.started"
	EventTypeWorkflowProgress  = "workflow.progress"
	EventTypeWorkflowCompleted = "workflow.completed"
	EventTypeWorkflowFailed    = "workflow.failed"
	EventTypeWorkflowCancelled = "workflow.cancelled"
	EventTypeWorkflowPaused    = "workflow.paused"
	EventTypeWorkflowResumed   = "workflow.resumed"

	// HITL events (new)
	EventTypeHitlPending   = "hitl.pending"
	EventTypeHitlApproved  = "hitl.approved"
	EventTypeHitlRejected  = "hitl.rejected"
	EventTypeHitlExpired   = "hitl.expired"
	EventTypeHitlEscalated = "hitl.escalated"

	// Budget events (new)
	EventTypeBudgetAlert   = "budget.alert"
	EventTypeBudgetLimit   = "budget.limit"
	EventTypeBudgetUpdated = "budget.updated"

	// Platform events (new)
	EventTypePlatformConnected    = "platform.connected"
	EventTypePlatformDisconnected = "platform.disconnected"
	EventTypePlatformMessage      = "platform.message"
	EventTypePlatformError        = "platform.error"

	// Email events
	EventTypeEmailReceived = "email.received"

	// Bridge events
	EventTypeBridgeStatus   = "bridge.status"
	EventTypeSessionExpired = "session.expired"
)

// BridgeEvent is the base event interface
type BridgeEvent interface {
	EventType() string
	Timestamp() time.Time
	ToJSON() ([]byte, error)
}

// BaseEvent provides common fields for all events
type BaseEvent struct {
	Type string    `json:"type"`
	Ts   time.Time `json:"timestamp"`
}

// EventType returns the event type
func (e *BaseEvent) EventType() string {
	return e.Type
}

// Timestamp returns the event timestamp
func (e *BaseEvent) Timestamp() time.Time {
	return e.Ts
}

// ToJSON serializes the event to JSON
func (e *BaseEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ============================================================================
// Agent Events
// ============================================================================

// AgentStartedEvent is emitted when an agent starts
type AgentStartedEvent struct {
	BaseEvent
	AgentID      string            `json:"agent_id"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	RoomID       string            `json:"room_id,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	KeyID        string            `json:"key_id,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// NewAgentStartedEvent creates a new agent started event
func NewAgentStartedEvent(agentID, name, agentType string, opts ...AgentEventOption) *AgentStartedEvent {
	event := &AgentStartedEvent{
		BaseEvent: BaseEvent{
			Type: EventTypeAgentStarted,
			Ts:   time.Now(),
		},
		AgentID: agentID,
		Name:    name,
		Type:    agentType,
	}
	for _, opt := range opts {
		opt(event)
	}
	return event
}

// AgentEventOption is a functional option for agent events
type AgentEventOption func(*AgentStartedEvent)

// WithRoomID sets the room ID
func WithRoomID(roomID string) AgentEventOption {
	return func(e *AgentStartedEvent) {
		e.RoomID = roomID
	}
}

// WithCapabilities sets the capabilities
func WithCapabilities(capabilities []string) AgentEventOption {
	return func(e *AgentStartedEvent) {
		e.Capabilities = capabilities
	}
}

// WithKeyID sets the key ID
func WithKeyID(keyID string) AgentEventOption {
	return func(e *AgentStartedEvent) {
		e.KeyID = keyID
	}
}

// WithMetadata sets the metadata
func WithMetadata(metadata map[string]string) AgentEventOption {
	return func(e *AgentStartedEvent) {
		e.Metadata = metadata
	}
}

// AgentStoppedEvent is emitted when an agent stops
type AgentStoppedEvent struct {
	BaseEvent
	AgentID string `json:"agent_id"`
	Reason  string `json:"reason,omitempty"`
}

// NewAgentStoppedEvent creates a new agent stopped event
func NewAgentStoppedEvent(agentID, reason string) *AgentStoppedEvent {
	return &AgentStoppedEvent{
		BaseEvent: BaseEvent{
			Type: EventTypeAgentStopped,
			Ts:   time.Now(),
		},
		AgentID: agentID,
		Reason:  reason,
	}
}

// AgentStatusChangedEvent is emitted when agent status changes
type AgentStatusChangedEvent struct {
	BaseEvent
	AgentID     string `json:"agent_id"`
	OldStatus   string `json:"old_status"`
	NewStatus   string `json:"new_status"`
	Uptime      int64  `json:"uptime_seconds"`
	MessagesIn  int64  `json:"messages_in"`
	MessagesOut int64  `json:"messages_out"`
}

// NewAgentStatusChangedEvent creates a new status changed event
func NewAgentStatusChangedEvent(agentID, oldStatus, newStatus string, uptime, msgsIn, msgsOut int64) *AgentStatusChangedEvent {
	return &AgentStatusChangedEvent{
		BaseEvent: BaseEvent{
			Type: EventTypeAgentStatusChanged,
			Ts:   time.Now(),
		},
		AgentID:     agentID,
		OldStatus:   oldStatus,
		NewStatus:   newStatus,
		Uptime:      uptime,
		MessagesIn:  msgsIn,
		MessagesOut: msgsOut,
	}
}

// ============================================================================
// Workflow Events
// ============================================================================

// WorkflowStartedEvent is emitted when a workflow starts
type WorkflowStartedEvent struct {
	BaseEvent
	WorkflowID   string            `json:"workflow_id"`
	TemplateName string            `json:"template_name"`
	AgentID      string            `json:"agent_id,omitempty"`
	TotalSteps   int               `json:"total_steps"`
	Params       map[string]string `json:"params,omitempty"`
}

// NewWorkflowStartedEvent creates a new workflow started event
func NewWorkflowStartedEvent(workflowID, templateName string, totalSteps int, params map[string]string) *WorkflowStartedEvent {
	return &WorkflowStartedEvent{
		BaseEvent: BaseEvent{
			Type: EventTypeWorkflowStarted,
			Ts:   time.Now(),
		},
		WorkflowID:   workflowID,
		TemplateName: templateName,
		TotalSteps:   totalSteps,
		Params:       params,
	}
}

// WorkflowProgressEvent is emitted when workflow progress updates
type WorkflowProgressEvent struct {
	BaseEvent
	WorkflowID  string    `json:"workflow_id"`
	StepNumber  int       `json:"step_number"`
	TotalSteps  int       `json:"total_steps"`
	StepName    string    `json:"step_name"`
	StepStatus  string    `json:"step_status"` // running, completed, failed
	Progress    float64   `json:"progress"`    // 0.0 to 1.0
	Message     string    `json:"message,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// NewWorkflowProgressEvent creates a new workflow progress event
func NewWorkflowProgressEvent(workflowID string, stepNum, totalSteps int, stepName, stepStatus string, progress float64) *WorkflowProgressEvent {
	return &WorkflowProgressEvent{
		BaseEvent: BaseEvent{
			Type: EventTypeWorkflowProgress,
			Ts:   time.Now(),
		},
		WorkflowID: workflowID,
		StepNumber: stepNum,
		TotalSteps: totalSteps,
		StepName:   stepName,
		StepStatus: stepStatus,
		Progress:   progress,
		StartedAt:  time.Now(),
	}
}

// WorkflowCompletedEvent is emitted when a workflow completes
type WorkflowCompletedEvent struct {
	BaseEvent
	WorkflowID     string        `json:"workflow_id"`
	Success        bool          `json:"success"`
	Result         string        `json:"result,omitempty"`
	Duration       time.Duration `json:"duration"`
	StepsCompleted int           `json:"steps_completed"`
	TotalSteps     int           `json:"total_steps"`
}

// NewWorkflowCompletedEvent creates a new workflow completed event
func NewWorkflowCompletedEvent(workflowID string, success bool, result string, duration time.Duration, stepsCompleted, totalSteps int) *WorkflowCompletedEvent {
	return &WorkflowCompletedEvent{
		BaseEvent: BaseEvent{
			Type: EventTypeWorkflowCompleted,
			Ts:   time.Now(),
		},
		WorkflowID:     workflowID,
		Success:        success,
		Result:         result,
		Duration:       duration,
		StepsCompleted: stepsCompleted,
		TotalSteps:     totalSteps,
	}
}

// WorkflowFailedEvent is emitted when a workflow fails
type WorkflowFailedEvent struct {
	BaseEvent
	WorkflowID  string `json:"workflow_id"`
	StepNumber  int    `json:"step_number"`
	Error       string `json:"error"`
	Recoverable bool   `json:"recoverable"`
}

// NewWorkflowFailedEvent creates a new workflow failed event
func NewWorkflowFailedEvent(workflowID string, stepNum int, errMsg string, recoverable bool) *WorkflowFailedEvent {
	return &WorkflowFailedEvent{
		BaseEvent: BaseEvent{
			Type: EventTypeWorkflowFailed,
			Ts:   time.Now(),
		},
		WorkflowID:  workflowID,
		StepNumber:  stepNum,
		Error:       errMsg,
		Recoverable: recoverable,
	}
}

// ============================================================================
// HITL Events
// ============================================================================

// HitlPendingEvent is emitted when a HITL approval is pending
type HitlPendingEvent struct {
	BaseEvent
	GateID      string    `json:"gate_id"`
	WorkflowID  string    `json:"workflow_id,omitempty"`
	AgentID     string    `json:"agent_id,omitempty"`
	RequestType string    `json:"request_type"` // file_access, command, data_export, etc.
	Description string    `json:"description"`
	ExpiresAt   time.Time `json:"expires_at"`
	Priority    string    `json:"priority"` // low, medium, high, critical
	Context     string    `json:"context,omitempty"`
}

// NewHitlPendingEvent creates a new HITL pending event
func NewHitlPendingEvent(gateID, requestType, description string, expiresAt time.Time, priority string) *HitlPendingEvent {
	return &HitlPendingEvent{
		BaseEvent: BaseEvent{
			Type: EventTypeHitlPending,
			Ts:   time.Now(),
		},
		GateID:      gateID,
		RequestType: requestType,
		Description: description,
		ExpiresAt:   expiresAt,
		Priority:    priority,
	}
}

// HitlApprovedEvent is emitted when a HITL request is approved
type HitlApprovedEvent struct {
	BaseEvent
	GateID     string    `json:"gate_id"`
	ApprovedBy string    `json:"approved_by"`
	ApprovedAt time.Time `json:"approved_at"`
	Notes      string    `json:"notes,omitempty"`
}

// NewHitlApprovedEvent creates a new HITL approved event
func NewHitlApprovedEvent(gateID, approvedBy string, notes string) *HitlApprovedEvent {
	return &HitlApprovedEvent{
		BaseEvent: BaseEvent{
			Type: EventTypeHitlApproved,
			Ts:   time.Now(),
		},
		GateID:     gateID,
		ApprovedBy: approvedBy,
		ApprovedAt: time.Now(),
		Notes:      notes,
	}
}

// HitlRejectedEvent is emitted when a HITL request is rejected
type HitlRejectedEvent struct {
	BaseEvent
	GateID     string `json:"gate_id"`
	RejectedBy string `json:"rejected_by"`
	Reason     string `json:"reason,omitempty"`
}

// NewHitlRejectedEvent creates a new HITL rejected event
func NewHitlRejectedEvent(gateID, rejectedBy, reason string) *HitlRejectedEvent {
	return &HitlRejectedEvent{
		BaseEvent: BaseEvent{
			Type: EventTypeHitlRejected,
			Ts:   time.Now(),
		},
		GateID:     gateID,
		RejectedBy: rejectedBy,
		Reason:     reason,
	}
}

// ============================================================================
// Budget Events
// ============================================================================

// BudgetAlertEvent is emitted when budget threshold is reached
type BudgetAlertEvent struct {
	BaseEvent
	Provider     string  `json:"provider"`
	UsagePercent float64 `json:"usage_percent"`
	Limit        float64 `json:"limit"`
	Current      float64 `json:"current"`
	AlertType    string  `json:"alert_type"` // warning, critical, exceeded
}

// NewBudgetAlertEvent creates a new budget alert event
func NewBudgetAlertEvent(provider string, usagePercent, limit, current float64, alertType string) *BudgetAlertEvent {
	return &BudgetAlertEvent{
		BaseEvent: BaseEvent{
			Type: EventTypeBudgetAlert,
			Ts:   time.Now(),
		},
		Provider:     provider,
		UsagePercent: usagePercent,
		Limit:        limit,
		Current:      current,
		AlertType:    alertType,
	}
}

// ============================================================================
// Platform Events
// ============================================================================

// PlatformConnectedEvent is emitted when a platform bridge connects
type PlatformConnectedEvent struct {
	BaseEvent
	Platform string `json:"platform"` // slack, discord, teams, etc.
	Status   string `json:"status"`
}

// NewPlatformConnectedEvent creates a new platform connected event
func NewPlatformConnectedEvent(platform, status string) *PlatformConnectedEvent {
	return &PlatformConnectedEvent{
		BaseEvent: BaseEvent{
			Type: EventTypePlatformConnected,
			Ts:   time.Now(),
		},
		Platform: platform,
		Status:   status,
	}
}

// PlatformDisconnectedEvent is emitted when a platform bridge disconnects
type PlatformDisconnectedEvent struct {
	BaseEvent
	Platform string `json:"platform"`
	Reason   string `json:"reason,omitempty"`
}

// NewPlatformDisconnectedEvent creates a new platform disconnected event
func NewPlatformDisconnectedEvent(platform, reason string) *PlatformDisconnectedEvent {
	return &PlatformDisconnectedEvent{
		BaseEvent: BaseEvent{
			Type: EventTypePlatformDisconnected,
			Ts:   time.Now(),
		},
		Platform: platform,
		Reason:   reason,
	}
}

// ============================================================================
// Bridge Status Events
// ============================================================================

// BridgeStatusEvent is emitted when a bridge component changes status
// (e.g., Matrix reconnection, subsystem health changes)
type BridgeStatusEvent struct {
	BaseEvent
	Component string `json:"component"`
	Status    string `json:"status"`
}

// NewBridgeStatusEvent creates a new bridge status event
func NewBridgeStatusEvent(component, status string) *BridgeStatusEvent {
	return &BridgeStatusEvent{
		BaseEvent: BaseEvent{
			Type: EventTypeBridgeStatus,
			Ts:   time.Now(),
		},
		Component: component,
		Status:    status,
	}
}

// ============================================================================
// Event Wrapper for WebSocket Transmission
// ============================================================================

// EventWrapper wraps any event for JSON serialization
type EventWrapper struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// WrapEvent wraps a BridgeEvent for transmission
func WrapEvent(event BridgeEvent) (*EventWrapper, error) {
	data, err := event.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize event: %w", err)
	}

	return &EventWrapper{
		Type:      event.EventType(),
		Timestamp: event.Timestamp(),
		Data:      data,
	}, nil
}

// ToJSON serializes the EventWrapper
func (w *EventWrapper) ToJSON() ([]byte, error) {
	return json.Marshal(w)
}
