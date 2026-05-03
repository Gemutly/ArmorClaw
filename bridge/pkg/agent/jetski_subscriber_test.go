package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestJetskiSubscriberParseSSEEvent(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		wantMethod string
		wantParams map[string]interface{}
		wantErr    bool
	}{
		{
			name:       "Page.frameNavigated",
			data:       `{"method":"Page.frameNavigated","params":{"url":"https://example.com"}}`,
			wantMethod: "Page.frameNavigated",
			wantParams: map[string]interface{}{"url": "https://example.com"},
		},
		{
			name:       "DOM.focus with nodeName",
			data:       `{"method":"DOM.focus","params":{"nodeName":"INPUT","type":"text"}}`,
			wantMethod: "DOM.focus",
			wantParams: map[string]interface{}{"nodeName": "INPUT", "type": "text"},
		},
		{
			name:       "Runtime.executionContextCreated",
			data:       `{"method":"Runtime.executionContextCreated","params":{}}`,
			wantMethod: "Runtime.executionContextCreated",
			wantParams: map[string]interface{}{},
		},
		{
			name:      "invalid JSON",
			data:      `{not json}`,
			wantErr:   true,
		},
		{
			name:       "empty params omitted",
			data:       `{"method":"Page.loadEventFired"}`,
			wantMethod: "Page.loadEventFired",
			wantParams: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt, err := ParseSSEEvent(tt.data)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if evt.Method != tt.wantMethod {
				t.Errorf("Method = %q, want %q", evt.Method, tt.wantMethod)
			}
			if tt.wantParams == nil && evt.Params != nil {
				t.Errorf("Params = %v, want nil", evt.Params)
			}
		})
	}
}

func TestJetskiSubscriberRegistrationHandshake(t *testing.T) {
	body, err := BuildRegistrationBody("bridge")
	if err != nil {
		t.Fatalf("BuildRegistrationBody: %v", err)
	}

	var req subscribeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}

	if req.Type != "register" {
		t.Errorf("Type = %q, want %q", req.Type, "register")
	}
	if req.Payload.DeviceID != "bridge" {
		t.Errorf("DeviceID = %q, want %q", req.Payload.DeviceID, "bridge")
	}
}

func TestJetskiSubscriberReconnectOnDisconnect(t *testing.T) {
	var connectCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&connectCount, 1)

		if count <= 2 {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "event: registered\ndata: {\"device_id\":\"bridge\"}\n\n")
			w.(http.Flusher).Flush()
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: registered\ndata: {\"device_id\":\"bridge\"}\n\n")
		w.(http.Flusher).Flush()

		fmt.Fprintf(w, "event: cdp\ndata: {\"method\":\"Page.frameNavigated\",\"params\":{}}\n\n")
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	coord := NewAgentCoordinator()
	sm := NewStateMachine(StateMachineConfig{AgentID: "test-agent"})
	coord.RegisterAgent("test-agent", sm)

	sub, err := NewJetskiStateEventSubscriber(JetskiSubscriberConfig{
		JetskiURL:  server.URL,
		DeviceID:   "bridge",
		Coordinator: coord,
	})
	if err != nil {
		t.Fatalf("NewJetskiStateEventSubscriber: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- sub.Start(ctx)
	}()

	time.Sleep(3 * time.Second)
	cancel()

	<-done

	count := atomic.LoadInt32(&connectCount)
	if count < 2 {
		t.Errorf("expected at least 2 connections (reconnect), got %d", count)
	}
}

func TestJetskiSubscriberEventMapping(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		params         map[string]interface{}
		initialState   AgentStatus
		expectedState  AgentStatus
	}{
		{
			name:          "Page.frameNavigated → BROWSING",
			method:        "Page.frameNavigated",
			params:        map[string]interface{}{"url": "https://example.com"},
			initialState:  StatusIdle,
			expectedState: StatusBrowsing,
		},
		{
			name:          "Page.frameNavigated from FORM_FILLING → BROWSING",
			method:        "Page.frameNavigated",
			params:        map[string]interface{}{"url": "https://example.com/page2"},
			initialState:  StatusFormFilling,
			expectedState: StatusBrowsing,
		},
		{
			name:          "DOM.focus on INPUT → FORM_FILLING",
			method:        "DOM.focus",
			params:        map[string]interface{}{"nodeName": "INPUT"},
			initialState:  StatusBrowsing,
			expectedState: StatusFormFilling,
		},
		{
			name:          "DOM.focus on non-input stays same",
			method:        "DOM.focus",
			params:        map[string]interface{}{"nodeName": "DIV"},
			initialState:  StatusBrowsing,
			expectedState: StatusBrowsing,
		},
		{
			name:          "AWAITING_APPROVAL is not overridden by CDP",
			method:        "Page.frameNavigated",
			params:        map[string]interface{}{"url": "https://example.com"},
			initialState:  StatusAwaitingApproval,
			expectedState: StatusAwaitingApproval,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewStateMachine(StateMachineConfig{AgentID: "test-agent"})

			if tt.initialState != StatusOffline {
				sm.ForceTransition(tt.initialState)
			}

			cdpEvents := []CDPEvent{
				{Method: tt.method, Params: tt.params},
			}

			changed := ApplyInferredState(sm, cdpEvents, WorkflowStatus{})

			result := sm.Current()
			if result != tt.expectedState {
				t.Errorf("state after %s = %v, want %v (changed=%v)",
					tt.method, result, tt.expectedState, changed)
			}
		})
	}
}

func TestJetskiSubscriberDOMFocusToFormFilling(t *testing.T) {
	tests := []struct {
		name       string
		params     map[string]interface{}
		wantForm   bool
	}{
		{
			name:     "INPUT element",
			params:   map[string]interface{}{"nodeName": "INPUT"},
			wantForm: true,
		},
		{
			name:     "TEXTAREA element",
			params:   map[string]interface{}{"nodeName": "TEXTAREA"},
			wantForm: true,
		},
		{
			name:     "SELECT element",
			params:   map[string]interface{}{"nodeName": "SELECT"},
			wantForm: true,
		},
		{
			name:     "input type=password",
			params:   map[string]interface{}{"nodeName": "INPUT", "type": "password"},
			wantForm: true,
		},
		{
			name:     "input type=email",
			params:   map[string]interface{}{"nodeName": "INPUT", "type": "email"},
			wantForm: true,
		},
		{
			name:     "DIV element (not form)",
			params:   map[string]interface{}{"nodeName": "DIV"},
			wantForm: false,
		},
		{
			name:     "BUTTON element (not form)",
			params:   map[string]interface{}{"nodeName": "BUTTON"},
			wantForm: false,
		},
		{
			name:     "nil params",
			params:   nil,
			wantForm: false,
		},
		{
			name:     "type=text without nodeName",
			params:   map[string]interface{}{"type": "text"},
			wantForm: true,
		},
		{
			name:     "type=tel",
			params:   map[string]interface{}{"type": "tel"},
			wantForm: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewStateMachine(StateMachineConfig{AgentID: "test-agent"})
			sm.ForceTransition(StatusBrowsing)

			changed := ApplyInferredState(sm, []CDPEvent{
				{Method: "DOM.focus", Params: tt.params},
			}, WorkflowStatus{})

			if tt.wantForm {
				if sm.Current() != StatusFormFilling {
					t.Errorf("expected FORM_FILLING, got %v (changed=%v)", sm.Current(), changed)
				}
			} else {
				if sm.Current() != StatusBrowsing {
					t.Errorf("expected BROWSING (unchanged), got %v (changed=%v)", sm.Current(), changed)
				}
			}
		})
	}
}

func TestJetskiSubscriberSSEStreamParsing(t *testing.T) {
	sseData := "event: registered\ndata: {\"device_id\":\"bridge\"}\n\n" +
		"event: cdp\ndata: {\"method\":\"Page.frameNavigated\",\"params\":{\"url\":\"https://example.com\"}}\n\n" +
		"event: cdp\ndata: {\"method\":\"DOM.focus\",\"params\":{\"nodeName\":\"INPUT\"}}\n\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req subscribeRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Type != "register" || req.Payload.DeviceID != "bridge" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseData)
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	coord := NewAgentCoordinator()
	sm := NewStateMachine(StateMachineConfig{AgentID: "test-agent"})
	coord.RegisterAgent("test-agent", sm)

	sub, err := NewJetskiStateEventSubscriber(JetskiSubscriberConfig{
		JetskiURL:   server.URL,
		DeviceID:    "bridge",
		Coordinator: coord,
	})
	if err != nil {
		t.Fatalf("NewJetskiStateEventSubscriber: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- sub.Start(ctx)
	}()

	// Wait for events to be processed (server sends immediately, reader processes quickly)
	time.Sleep(200 * time.Millisecond)

	finalState := sm.Current()

	cancel()
	<-done

	if finalState != StatusFormFilling {
		t.Errorf("expected FORM_FILLING after Page.frameNavigated + DOM.focus(INPUT), got %v", finalState)
	}
}

func TestJetskiSubscriberProcessSSEStream(t *testing.T) {
	sseInput := "event: registered\ndata: {\"device_id\":\"bridge\"}\n\n" +
		"event: cdp\ndata: {\"method\":\"Page.frameNavigated\",\"params\":{\"url\":\"https://test.com\"}}\n\n" +
		": this is a comment\n\n" +
		"event: unknown\ndata: {}\n\n"

	coord := NewAgentCoordinator()
	sm := NewStateMachine(StateMachineConfig{AgentID: "test-agent"})
	coord.RegisterAgent("test-agent", sm)

	sub, _ := NewJetskiStateEventSubscriber(JetskiSubscriberConfig{
		JetskiURL:   "http://localhost:9223",
		DeviceID:    "bridge",
		Coordinator: coord,
	})
	sub.ctx, sub.cancel = context.WithCancel(context.Background())

	reader := bufio.NewReader(strings.NewReader(sseInput))
	err := sub.processSSEStream(reader)
	if err != nil {
		t.Fatalf("processSSEStream error: %v", err)
	}

	if sm.Current() != StatusBrowsing {
		t.Errorf("expected BROWSING after Page.frameNavigated, got %v", sm.Current())
	}
}

func TestJetskiSubscriberValidation(t *testing.T) {
	coord := NewAgentCoordinator()

	_, err := NewJetskiStateEventSubscriber(JetskiSubscriberConfig{
		DeviceID:    "bridge",
		Coordinator: coord,
	})
	if err == nil {
		t.Error("expected error for missing JetskiURL")
	}

	_, err = NewJetskiStateEventSubscriber(JetskiSubscriberConfig{
		JetskiURL:   "http://localhost:9223",
		Coordinator: coord,
	})
	if err == nil {
		t.Error("expected error for missing DeviceID")
	}

	_, err = NewJetskiStateEventSubscriber(JetskiSubscriberConfig{
		JetskiURL: "http://localhost:9223",
		DeviceID:  "bridge",
	})
	if err == nil {
		t.Error("expected error for missing Coordinator")
	}
}

func TestJetskiSubscriberConnectionRefused(t *testing.T) {
	coord := NewAgentCoordinator()
	sub, err := NewJetskiStateEventSubscriber(JetskiSubscriberConfig{
		JetskiURL:   "http://127.0.0.1:1",
		DeviceID:    "bridge",
		Coordinator: coord,
	})
	if err != nil {
		t.Fatalf("NewJetskiStateEventSubscriber: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sub.Start(ctx)

	if sub.Connected() {
		t.Error("should not be connected to invalid port")
	}
}

func TestJetskiSubscriberStop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "event: registered\ndata: {\"device_id\":\"bridge\"}\n\n")
		w.(http.Flusher).Flush()

		<-r.Context().Done()
	}))
	defer server.Close()

	coord := NewAgentCoordinator()
	sm := NewStateMachine(StateMachineConfig{AgentID: "test-agent"})
	coord.RegisterAgent("test-agent", sm)

	sub, _ := NewJetskiStateEventSubscriber(JetskiSubscriberConfig{
		JetskiURL:   server.URL,
		DeviceID:    "bridge",
		Coordinator: coord,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- sub.Start(ctx)
	}()

	time.Sleep(200 * time.Millisecond)

	if !sub.Connected() {
		t.Fatal("expected subscriber to be connected before stop")
	}

	sub.Stop()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after Stop")
	}

	if sub.Connected() {
		t.Error("expected subscriber to be disconnected after stop")
	}
}
