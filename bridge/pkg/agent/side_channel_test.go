package agent

import (
	"context"
	"testing"
	"time"

	"github.com/armorclaw/bridge/internal/events"
)

func setupSideChannelTest(t *testing.T, agentID string, initialState AgentStatus) (*AgentCoordinator, *events.MatrixEventBus, <-chan events.MatrixEvent, *StateMachine) {
	t.Helper()

	bus := events.NewMatrixEventBus(64)
	sub := bus.Subscribe()

	coordinator := NewAgentCoordinator()
	coordinator.SetEventBus(bus)

	sm := NewStateMachine(StateMachineConfig{AgentID: agentID})
	integration, err := coordinator.RegisterAgent(agentID, sm)
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}
	integration.SetRoomID("!room-test:example.com")

	sm.ForceTransition(StatusIdle)
	sm.ForceTransition(StatusInitializing)
	if initialState != StatusIdle && initialState != StatusInitializing {
		sm.ForceTransition(initialState)
	}

	return coordinator, bus, sub, sm
}

func TestSideChannel_CaptchaSignal(t *testing.T) {
	coordinator, _, sub, sm := setupSideChannelTest(t, "agent-captcha", StatusBrowsing)

	changed, err := coordinator.EmitSideChannelSignal(context.Background(), "agent-captcha", "captcha")
	if err != nil {
		t.Fatalf("EmitSideChannelSignal: %v", err)
	}
	if !changed {
		t.Fatal("expected state change")
	}

	if sm.Current() != StatusAwaitingCaptcha {
		t.Fatalf("expected AWAITING_CAPTCHA, got %v", sm.Current())
	}

	select {
	case published := <-sub:
		content, ok := published.Content.(map[string]interface{})
		if !ok {
			t.Fatal("expected map content")
		}
		if status := content["status"].(string); status != "AWAITING_CAPTCHA" {
			t.Errorf("expected status AWAITING_CAPTCHA, got %q", status)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for broadcast event")
	}
}

func TestSideChannel_2FASignal(t *testing.T) {
	coordinator, _, sub, sm := setupSideChannelTest(t, "agent-2fa", StatusBrowsing)

	changed, err := coordinator.EmitSideChannelSignal(context.Background(), "agent-2fa", "twofa")
	if err != nil {
		t.Fatalf("EmitSideChannelSignal: %v", err)
	}
	if !changed {
		t.Fatal("expected state change")
	}

	if sm.Current() != StatusAwaiting2FA {
		t.Fatalf("expected AWAITING_2FA, got %v", sm.Current())
	}

	select {
	case published := <-sub:
		content, ok := published.Content.(map[string]interface{})
		if !ok {
			t.Fatal("expected map content")
		}
		if status := content["status"].(string); status != "AWAITING_2FA" {
			t.Errorf("expected status AWAITING_2FA, got %q", status)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for broadcast event")
	}
}

func TestSideChannel_PaymentSignal(t *testing.T) {
	coordinator, _, sub, sm := setupSideChannelTest(t, "agent-payment", StatusFormFilling)

	changed, err := coordinator.EmitSideChannelSignal(context.Background(), "agent-payment", "payment")
	if err != nil {
		t.Fatalf("EmitSideChannelSignal: %v", err)
	}
	if !changed {
		t.Fatal("expected state change")
	}

	if sm.Current() != StatusProcessingPayment {
		t.Fatalf("expected PROCESSING_PAYMENT, got %v", sm.Current())
	}

	select {
	case published := <-sub:
		content, ok := published.Content.(map[string]interface{})
		if !ok {
			t.Fatal("expected map content")
		}
		if status := content["status"].(string); status != "PROCESSING_PAYMENT" {
			t.Errorf("expected status PROCESSING_PAYMENT, got %q", status)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for broadcast event")
	}
}

func TestSideChannel_OfflineSignal(t *testing.T) {
	coordinator, _, sub, sm := setupSideChannelTest(t, "agent-offline", StatusBrowsing)

	changed, err := coordinator.EmitSideChannelSignal(context.Background(), "agent-offline", "offline")
	if err != nil {
		t.Fatalf("EmitSideChannelSignal: %v", err)
	}
	if !changed {
		t.Fatal("expected state change")
	}

	if sm.Current() != StatusOffline {
		t.Fatalf("expected OFFLINE, got %v", sm.Current())
	}

	select {
	case published := <-sub:
		content, ok := published.Content.(map[string]interface{})
		if !ok {
			t.Fatal("expected map content")
		}
		if status := content["status"].(string); status != "OFFLINE" {
			t.Errorf("expected status OFFLINE, got %q", status)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for broadcast event")
	}
}

func TestSideChannel_OverridesCDPState(t *testing.T) {
	coordinator, _, _, sm := setupSideChannelTest(t, "agent-override", StatusBrowsing)

	changed, err := coordinator.EmitSideChannelSignal(context.Background(), "agent-override", "captcha")
	if err != nil {
		t.Fatalf("EmitSideChannelSignal: %v", err)
	}
	if !changed {
		t.Fatal("expected state change")
	}

	if sm.Current() != StatusAwaitingCaptcha {
		t.Fatalf("expected AWAITING_CAPTCHA (priority-1 override), got %v", sm.Current())
	}
}

func TestSideChannel_UnknownSignalIgnored(t *testing.T) {
	coordinator, _, sub, sm := setupSideChannelTest(t, "agent-unknown", StatusBrowsing)

	changed, err := coordinator.EmitSideChannelSignal(context.Background(), "agent-unknown", "unknown_signal")
	if err != nil {
		t.Fatalf("EmitSideChannelSignal: %v", err)
	}
	if changed {
		t.Fatal("expected no state change for unknown signal")
	}

	if sm.Current() != StatusBrowsing {
		t.Fatalf("expected BROWSING (unchanged), got %v", sm.Current())
	}

	select {
	case <-sub:
		t.Fatal("expected NO broadcast for unknown signal")
	default:
	}
}

func TestSideChannel_AgentNotFound(t *testing.T) {
	coordinator := NewAgentCoordinator()

	changed, err := coordinator.EmitSideChannelSignal(context.Background(), "nonexistent", "captcha")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
	if changed {
		t.Fatal("expected no state change for nonexistent agent")
	}
}
