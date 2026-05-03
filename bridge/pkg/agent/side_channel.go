package agent

import (
	"context"
	"fmt"
	"log"
	"time"
)

// EmitSideChannelSignal routes workflow side-channel signals (captcha, 2FA, payment, offline)
// to the state inference engine. These signals are priority-1 and override CDP-based inference.
//
// Recognized signals: "captcha", "twofa", "payment", "offline".
// Unknown signals are ignored (no state change).
//
// Returns true if the state changed, false otherwise.
func (c *AgentCoordinator) EmitSideChannelSignal(ctx context.Context, agentID string, signal string, metadata ...StatusMetadata) (bool, error) {
	integration, err := c.GetAgent(agentID)
	if err != nil {
		return false, fmt.Errorf("EmitSideChannelSignal: %w", err)
	}

	workflowStatus := WorkflowStatus{State: signal}

	changed := ApplyInferredState(integration.stateMachine, nil, workflowStatus)

	if changed {
		newStatus := integration.stateMachine.Current()
		lastEvent := integration.stateMachine.LastEvent()

		var previousStatus AgentStatus
		if lastEvent != nil {
			previousStatus = lastEvent.Previous
		}

		var meta StatusMetadata
		if len(metadata) > 0 {
			meta = metadata[0]
		}
		meta.InferredFrom = "workflow"

		broadcastEvent := StatusEvent{
			AgentID:   agentID,
			Status:    newStatus,
			Previous:  previousStatus,
			Timestamp: time.Now().UnixMilli(),
			Metadata:  meta,
		}

		if err := c.BroadcastStatus(ctx, broadcastEvent); err != nil {
			log.Printf("[SIDE CHANNEL]: failed to broadcast status for agent %s: %v", agentID, err)
		}
	}

	return changed, nil
}
