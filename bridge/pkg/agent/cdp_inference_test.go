package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/armorclaw/bridge/internal/events"
)

func TestCDPInference_BroadcastsOnStateChange(t *testing.T) {
	bus := events.NewMatrixEventBus(64)
	sub := bus.Subscribe()

	coordinator := NewAgentCoordinator()
	coordinator.SetEventBus(bus)

	sm := NewStateMachine(StateMachineConfig{AgentID: "agent-browse"})
	integration, err := coordinator.RegisterAgent("agent-browse", sm)
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}
	integration.SetRoomID("!room-browse:example.com")

	sm.ForceTransition(StatusIdle)

	sseData := "event: registered\ndata: {\"device_id\":\"bridge\"}\n\n" +
		"event: cdp\ndata: {\"method\":\"Page.frameNavigated\",\"params\":{\"url\":\"https://example.com\"}}\n\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseData)
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	jetskiSub, err := NewJetskiStateEventSubscriber(JetskiSubscriberConfig{
		JetskiURL:   server.URL,
		DeviceID:    "bridge",
		Coordinator: coordinator,
	})
	if err != nil {
		t.Fatalf("NewJetskiStateEventSubscriber: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- jetskiSub.Start(ctx)
	}()

	time.Sleep(300 * time.Millisecond)

	finalState := sm.Current()
	cancel()
	<-done

	if finalState != StatusBrowsing {
		t.Fatalf("expected agent state BROWSING, got %v", finalState)
	}

	select {
	case published := <-sub:
		if published.Type != "com.armorclaw.agent.status" {
			t.Errorf("expected event type 'com.armorclaw.agent.status', got %q", published.Type)
		}
		content, ok := published.Content.(map[string]interface{})
		if !ok {
			t.Fatal("expected map content")
		}
		if status := content["status"].(string); status != "BROWSING" {
			t.Errorf("expected status BROWSING, got %q", status)
		}
		if agentID := content["agent_id"].(string); agentID != "agent-browse" {
			t.Errorf("expected agent_id 'agent-browse', got %q", agentID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for broadcast event")
	}
}

func TestCDPInference_AwaitingApprovalProtected(t *testing.T) {
	bus := events.NewMatrixEventBus(64)
	sub := bus.Subscribe()

	coordinator := NewAgentCoordinator()
	coordinator.SetEventBus(bus)

	sm := NewStateMachine(StateMachineConfig{AgentID: "agent-approval"})
	integration, err := coordinator.RegisterAgent("agent-approval", sm)
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}
	integration.SetRoomID("!room-approval:example.com")

	sm.ForceTransition(StatusFormFilling)
	sm.ForceTransition(StatusAwaitingApproval, StatusMetadata{
		FieldsRequested: []string{"credit_card_number"},
	})

	sseData := "event: registered\ndata: {\"device_id\":\"bridge\"}\n\n" +
		"event: cdp\ndata: {\"method\":\"Page.frameNavigated\",\"params\":{\"url\":\"https://example.com\"}}\n\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseData)
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	jetskiSub, err := NewJetskiStateEventSubscriber(JetskiSubscriberConfig{
		JetskiURL:   server.URL,
		DeviceID:    "bridge",
		Coordinator: coordinator,
	})
	if err != nil {
		t.Fatalf("NewJetskiStateEventSubscriber: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- jetskiSub.Start(ctx)
	}()

	time.Sleep(300 * time.Millisecond)

	finalState := sm.Current()
	cancel()
	<-done

	if finalState != StatusAwaitingApproval {
		t.Fatalf("expected AWAITING_APPROVAL to be preserved, got %v", finalState)
	}

	select {
	case <-sub:
		t.Fatal("expected NO broadcast when AWAITING_APPROVAL is preserved")
	default:
	}
}

func TestCDPInference_NoBroadcastOnNoChange(t *testing.T) {
	bus := events.NewMatrixEventBus(64)
	sub := bus.Subscribe()

	coordinator := NewAgentCoordinator()
	coordinator.SetEventBus(bus)

	sm := NewStateMachine(StateMachineConfig{AgentID: "agent-same"})
	integration, err := coordinator.RegisterAgent("agent-same", sm)
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}
	integration.SetRoomID("!room-same:example.com")

	sm.ForceTransition(StatusIdle)
	sm.ForceTransition(StatusInitializing)
	sm.ForceTransition(StatusBrowsing, StatusMetadata{URL: "https://example.com"})

	sseData := "event: registered\ndata: {\"device_id\":\"bridge\"}\n\n" +
		"event: cdp\ndata: {\"method\":\"Page.frameNavigated\",\"params\":{\"url\":\"https://example.com/page2\"}}\n\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseData)
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	jetskiSub, err := NewJetskiStateEventSubscriber(JetskiSubscriberConfig{
		JetskiURL:   server.URL,
		DeviceID:    "bridge",
		Coordinator: coordinator,
	})
	if err != nil {
		t.Fatalf("NewJetskiStateEventSubscriber: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- jetskiSub.Start(ctx)
	}()

	time.Sleep(300 * time.Millisecond)

	finalState := sm.Current()
	cancel()
	<-done

	if finalState != StatusBrowsing {
		t.Fatalf("expected BROWSING (unchanged), got %v", finalState)
	}

	select {
	case <-sub:
		t.Fatal("expected NO broadcast when state does not change")
	default:
	}
}

func TestCDPInference_FormFillingBroadcast(t *testing.T) {
	bus := events.NewMatrixEventBus(64)
	sub := bus.Subscribe()

	coordinator := NewAgentCoordinator()
	coordinator.SetEventBus(bus)

	sm := NewStateMachine(StateMachineConfig{AgentID: "agent-form"})
	integration, err := coordinator.RegisterAgent("agent-form", sm)
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}
	integration.SetRoomID("!room-form:example.com")

	sm.ForceTransition(StatusIdle)
	sm.ForceTransition(StatusInitializing)
	sm.ForceTransition(StatusBrowsing)

	sseData := "event: registered\ndata: {\"device_id\":\"bridge\"}\n\n" +
		"event: cdp\ndata: {\"method\":\"DOM.focus\",\"params\":{\"nodeName\":\"INPUT\",\"type\":\"text\"}}\n\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseData)
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	jetskiSub, err := NewJetskiStateEventSubscriber(JetskiSubscriberConfig{
		JetskiURL:   server.URL,
		DeviceID:    "bridge",
		Coordinator: coordinator,
	})
	if err != nil {
		t.Fatalf("NewJetskiStateEventSubscriber: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- jetskiSub.Start(ctx)
	}()

	time.Sleep(300 * time.Millisecond)

	finalState := sm.Current()
	cancel()
	<-done

	if finalState != StatusFormFilling {
		t.Fatalf("expected FORM_FILLING, got %v", finalState)
	}

	select {
	case published := <-sub:
		if published.Type != "com.armorclaw.agent.status" {
			t.Errorf("expected event type 'com.armorclaw.agent.status', got %q", published.Type)
		}
		content, ok := published.Content.(map[string]interface{})
		if !ok {
			t.Fatal("expected map content")
		}
		if status := content["status"].(string); status != "FORM_FILLING" {
			t.Errorf("expected status FORM_FILLING, got %q", status)
		}
		if agentID := content["agent_id"].(string); agentID != "agent-form" {
			t.Errorf("expected agent_id 'agent-form', got %q", agentID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for broadcast event")
	}
}
