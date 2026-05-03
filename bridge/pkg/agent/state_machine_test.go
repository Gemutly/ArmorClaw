package agent

import (
	"bytes"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestNewStateMachine(t *testing.T) {
	cfg := StateMachineConfig{
		AgentID:     "test-agent-001",
		HistorySize: 50,
	}

	sm := NewStateMachine(cfg)
	if sm == nil {
		t.Fatal("expected state machine to be created")
	}

	if sm.Current() != StatusOffline {
		t.Errorf("expected initial state to be OFFLINE, got %s", sm.Current())
	}

	if sm.AgentID() != "test-agent-001" {
		t.Errorf("expected agent ID 'test-agent-001', got %s", sm.AgentID())
	}
}

func TestNewStateMachineDefaultHistorySize(t *testing.T) {
	cfg := StateMachineConfig{
		AgentID: "test-agent",
	}

	sm := NewStateMachine(cfg)
	if sm.historySize != 100 {
		t.Errorf("expected default history size 100, got %d", sm.historySize)
	}
}

func TestTransitionValid(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{AgentID: "test"})

	// OFFLINE -> INITIALIZING (valid)
	err := sm.Transition(StatusInitializing)
	if err != nil {
		t.Errorf("expected valid transition OFFLINE -> INITIALIZING, got error: %v", err)
	}

	if sm.Current() != StatusInitializing {
		t.Errorf("expected state INITIALIZING, got %s", sm.Current())
	}
}

func TestTransitionInvalid(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{AgentID: "test"})

	// OFFLINE -> IDLE (invalid - can only go to INITIALIZING)
	err := sm.Transition(StatusIdle)
	if err == nil {
		t.Error("expected error for invalid transition OFFLINE -> IDLE")
	}
}

func TestTransitionWithMetadata(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{AgentID: "test"})
	sm.ForceTransition(StatusBrowsing)

	metadata := StatusMetadata{
		URL:      "https://example.com",
		Step:     "1/5",
		Progress: 20,
	}

	err := sm.Transition(StatusFormFilling, metadata)
	if err != nil {
		t.Fatalf("transition failed: %v", err)
	}

	meta := sm.Metadata()
	if meta.URL != "https://example.com" {
		t.Errorf("expected URL 'https://example.com', got %s", meta.URL)
	}
	if meta.Step != "1/5" {
		t.Errorf("expected Step '1/5', got %s", meta.Step)
	}
	if meta.Progress != 20 {
		t.Errorf("expected Progress 20, got %d", meta.Progress)
	}
}

func TestForceTransition(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{AgentID: "test"})

	// Force transition bypasses validation
	sm.ForceTransition(StatusBrowsing)
	if sm.Current() != StatusBrowsing {
		t.Errorf("expected state BROWSING after force, got %s", sm.Current())
	}
}

func TestHelperMethods(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{AgentID: "test"})

	// Initialize (OFFLINE -> INITIALIZING)
	if err := sm.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if sm.Current() != StatusInitializing {
		t.Errorf("expected INITIALIZING, got %s", sm.Current())
	}

	// SetReady (INITIALIZING -> IDLE)
	if err := sm.SetReady(); err != nil {
		t.Fatalf("SetReady failed: %v", err)
	}
	if sm.Current() != StatusIdle {
		t.Errorf("expected IDLE, got %s", sm.Current())
	}

	// To browse from IDLE, we need to go through INITIALIZING first
	// (IDLE can only go to INITIALIZING or ERROR)
	if err := sm.Transition(StatusInitializing); err != nil {
		t.Fatalf("Transition to INITIALIZING failed: %v", err)
	}
	if err := sm.StartBrowsing("https://example.com"); err != nil {
		t.Fatalf("StartBrowsing failed: %v", err)
	}
	if sm.Current() != StatusBrowsing {
		t.Errorf("expected BROWSING, got %s", sm.Current())
	}

	// StartFormFilling
	if err := sm.StartFormFilling("step-1", 25); err != nil {
		t.Fatalf("StartFormFilling failed: %v", err)
	}
	if sm.Current() != StatusFormFilling {
		t.Errorf("expected FORM_FILLING, got %s", sm.Current())
	}

	// RequestApproval
	if err := sm.RequestApproval([]string{"name", "email"}); err != nil {
		t.Fatalf("RequestApproval failed: %v", err)
	}
	meta := sm.Metadata()
	if len(meta.FieldsRequested) != 2 {
		t.Errorf("expected 2 fields requested, got %d", len(meta.FieldsRequested))
	}

	// Complete via force
	sm.ForceTransition(StatusComplete)
	if sm.Current() != StatusComplete {
		t.Errorf("expected COMPLETE, got %s", sm.Current())
	}
}

func TestFail(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{AgentID: "test"})
	sm.ForceTransition(StatusBrowsing)

	err := sm.FailWithString("test error")
	if err != nil {
		t.Fatalf("FailWithString failed: %v", err)
	}

	if sm.Current() != StatusError {
		t.Errorf("expected ERROR state, got %s", sm.Current())
	}

	meta := sm.Metadata()
	if meta.Error != "test error" {
		t.Errorf("expected error message 'test error', got %s", meta.Error)
	}
}

func TestReset(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{AgentID: "test"})
	sm.ForceTransition(StatusError)

	err := sm.Reset()
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	if sm.Current() != StatusIdle {
		t.Errorf("expected IDLE after reset, got %s", sm.Current())
	}
}

func TestEvents(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{AgentID: "test"})

	// Subscribe to events first
	sub := sm.Subscribe()
	defer sm.Unsubscribe(sub)

	// Small delay to ensure subscriber is registered
	time.Sleep(10 * time.Millisecond)

	// Force a transition (uses non-blocking send, event goes to main eventChan)
	sm.ForceTransition(StatusInitializing)

	// Check the main Events() channel instead of subscriber
	// (ForceTransition only emits to main channel, not all subscribers)
	select {
	case event := <-sm.Events():
		if event.Status != StatusInitializing {
			t.Errorf("expected event status INITIALIZING, got %s", event.Status)
		}
		if event.Previous != StatusOffline {
			t.Errorf("expected previous status OFFLINE, got %s", event.Previous)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestHistory(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{AgentID: "test"})

	// Create some history
	sm.ForceTransition(StatusInitializing)
	sm.ForceTransition(StatusBrowsing)
	sm.ForceTransition(StatusFormFilling)

	history := sm.History(10)
	if len(history) < 3 {
		t.Errorf("expected at least 3 history entries, got %d", len(history))
	}

	// Last entry should be FORM_FILLING
	last := history[len(history)-1]
	if last.Status != StatusFormFilling {
		t.Errorf("expected last status FORM_FILLING, got %s", last.Status)
	}
}

func TestHistoryLimit(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{
		AgentID:     "test",
		HistorySize: 5,
	})

	// Create more than 5 transitions
	for i := 0; i < 10; i++ {
		sm.ForceTransition(StatusBrowsing)
		sm.ForceTransition(StatusFormFilling)
	}

	history := sm.History(100)
	if len(history) > 5 {
		t.Errorf("expected max 5 history entries, got %d", len(history))
	}
}

func TestLastEvent(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{AgentID: "test"})

	// No events yet
	if sm.LastEvent() != nil {
		t.Error("expected nil LastEvent with no history")
	}

	// Add event
	sm.ForceTransition(StatusInitializing)

	last := sm.LastEvent()
	if last == nil {
		t.Fatal("expected LastEvent to return event")
	}
	if last.Status != StatusInitializing {
		t.Errorf("expected status INITIALIZING, got %s", last.Status)
	}
}

func TestString(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{AgentID: "test"})

	// No metadata
	sm.ForceTransition(StatusIdle)
	if sm.String() != "IDLE" {
		t.Errorf("expected 'IDLE', got %s", sm.String())
	}

	// With URL metadata
	sm.ForceTransition(StatusBrowsing, StatusMetadata{URL: "https://example.com"})
	if sm.String() != "BROWSING (https://example.com)" {
		t.Errorf("expected 'BROWSING (https://example.com)', got %s", sm.String())
	}

	// With step metadata
	sm.ForceTransition(StatusFormFilling, StatusMetadata{Step: "2/5"})
	if sm.String() != "FORM_FILLING (step: 2/5)" {
		t.Errorf("expected 'FORM_FILLING (step: 2/5)', got %s", sm.String())
	}
}

func TestShutdown(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{AgentID: "test"})

	// Subscribe before shutdown
	sub := sm.Subscribe()

	// Shutdown should close channels
	sm.Shutdown()

	// Check Done channel is closed
	select {
	case <-sm.Done():
		// Expected - channel closed
	default:
		t.Error("expected Done channel to be closed after shutdown")
	}

	// Check subscriber channel is closed
	select {
	case _, ok := <-sub:
		if ok {
			t.Error("expected subscriber channel to be closed")
		}
	default:
		// Channel might not have been written to, but should be closed
	}
}

func TestConcurrentTransitions(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{AgentID: "test"})

	// Set to a state with multiple valid transitions
	sm.ForceTransition(StatusFormFilling)

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// Try concurrent transitions
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := sm.Transition(StatusBrowsing)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Count errors - should have 9 failures (only one transition succeeds)
	errorCount := 0
	for range errors {
		errorCount++
	}

	if errorCount != 9 {
		t.Errorf("expected 9 transition failures (concurrent), got %d", errorCount)
	}
}

func TestStatusEventToMatrix(t *testing.T) {
	event := StatusEvent{
		AgentID:   "agent-001",
		Status:    StatusBrowsing,
		Previous:  StatusIdle,
		Timestamp: 1234567890,
		Metadata: StatusMetadata{
			URL: "https://example.com",
		},
	}

	// Test event type
	if event.EventType() != "com.armorclaw.agent.status" {
		t.Errorf("expected event type 'com.armorclaw.agent.status', got %s", event.EventType())
	}

	// Test Matrix content
	content := event.ToMatrixContent()
	if content["agent_id"] != "agent-001" {
		t.Errorf("expected agent_id 'agent-001', got %v", content["agent_id"])
	}
	if content["status"] != "BROWSING" {
		t.Errorf("expected status 'BROWSING', got %v", content["status"])
	}
}

func TestStatusEventFromMatrix(t *testing.T) {
	content := map[string]interface{}{
		"agent_id":  "agent-001",
		"status":    "BROWSING",
		"previous":  "IDLE",
		"timestamp": float64(1234567890),
		"metadata": map[string]interface{}{
			"url":      "https://example.com",
			"step":     "1/5",
			"progress": float64(20),
		},
	}

	event, err := StatusEventFromMatrix(content)
	if err != nil {
		t.Fatalf("StatusEventFromMatrix failed: %v", err)
	}

	if event.AgentID != "agent-001" {
		t.Errorf("expected agent_id 'agent-001', got %s", event.AgentID)
	}
	if event.Status != StatusBrowsing {
		t.Errorf("expected status BROWSING, got %s", event.Status)
	}
	if event.Metadata.URL != "https://example.com" {
		t.Errorf("expected URL 'https://example.com', got %s", event.Metadata.URL)
	}
	if event.Metadata.Progress != 20 {
		t.Errorf("expected progress 20, got %d", event.Metadata.Progress)
	}
}

func TestStatusEventFromMatrixWithFieldsRequested(t *testing.T) {
	content := map[string]interface{}{
		"agent_id":  "agent-001",
		"status":    "AWAITING_APPROVAL",
		"timestamp": float64(1234567890),
		"metadata": map[string]interface{}{
			"fields_requested": []interface{}{"name", "email", "phone"},
		},
	}

	event, err := StatusEventFromMatrix(content)
	if err != nil {
		t.Fatalf("StatusEventFromMatrix failed: %v", err)
	}

	if len(event.Metadata.FieldsRequested) != 3 {
		t.Errorf("expected 3 fields requested, got %d", len(event.Metadata.FieldsRequested))
	}
}

func TestAgentStatusMethods(t *testing.T) {
	// Test IsTerminal
	if !StatusAwaitingCaptcha.IsTerminal() {
		t.Error("AWAITING_CAPTCHA should be terminal")
	}
	if !StatusAwaiting2FA.IsTerminal() {
		t.Error("AWAITING_2FA should be terminal")
	}
	if !StatusAwaitingApproval.IsTerminal() {
		t.Error("AWAITING_APPROVAL should be terminal")
	}
	if StatusBrowsing.IsTerminal() {
		t.Error("BROWSING should not be terminal")
	}

	// Test IsActive
	if !StatusBrowsing.IsActive() {
		t.Error("BROWSING should be active")
	}
	if !StatusFormFilling.IsActive() {
		t.Error("FORM_FILLING should be active")
	}
	if !StatusProcessingPayment.IsActive() {
		t.Error("PROCESSING_PAYMENT should be active")
	}
	if StatusIdle.IsActive() {
		t.Error("IDLE should not be active")
	}

	// Test NeedsUserAction
	if !StatusAwaitingCaptcha.NeedsUserAction() {
		t.Error("AWAITING_CAPTCHA should need user action")
	}
	if !StatusAwaiting2FA.NeedsUserAction() {
		t.Error("AWAITING_2FA should need user action")
	}
	if !StatusAwaitingApproval.NeedsUserAction() {
		t.Error("AWAITING_APPROVAL should need user action")
	}
	if StatusBrowsing.NeedsUserAction() {
		t.Error("BROWSING should not need user action")
	}
}

func TestValidateTransition(t *testing.T) {
	// Valid transition
	err := ValidateTransition(StatusOffline, StatusInitializing)
	if err != nil {
		t.Errorf("expected valid transition OFFLINE -> INITIALIZING, got error: %v", err)
	}

	// Invalid transition
	err = ValidateTransition(StatusOffline, StatusIdle)
	if err == nil {
		t.Error("expected error for invalid transition OFFLINE -> IDLE")
	}

	// Unknown current state
	err = ValidateTransition(AgentStatus("UNKNOWN"), StatusIdle)
	if err == nil {
		t.Error("expected error for unknown current state")
	}
}

func TestNewStatusEvent(t *testing.T) {
	event := NewStatusEvent("agent-001", StatusBrowsing, StatusIdle, StatusMetadata{
		URL: "https://example.com",
	})

	if event.AgentID != "agent-001" {
		t.Errorf("expected agent_id 'agent-001', got %s", event.AgentID)
	}
	if event.Status != StatusBrowsing {
		t.Errorf("expected status BROWSING, got %s", event.Status)
	}
	if event.Timestamp == 0 {
		t.Error("expected non-zero timestamp")
	}
}

func TestTransitionLog_ForceTransition(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	slog.SetDefault(logger)

	sm := NewStateMachine(StateMachineConfig{AgentID: "test-agent-log"})

	sm.ForceTransition(StatusBrowsing, StatusMetadata{InferredFrom: "CDP"})

	logOutput := buf.String()
	if logOutput == "" {
		t.Fatal("expected log output from ForceTransition, got empty")
	}

	agentID := "agent_id=test-agent-log"
	from := "from=OFFLINE"
	to := "to=BROWSING"
	inferredFrom := "inferred_from=CDP"

	for _, field := range []string{agentID, from, to, inferredFrom} {
		if !bytes.Contains(buf.Bytes(), []byte(field)) {
			t.Errorf("expected log to contain %q, got: %s", field, logOutput)
		}
	}
}

func TestTransitionLog_ForceTransitionDefaultManual(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	slog.SetDefault(logger)

	sm := NewStateMachine(StateMachineConfig{AgentID: "test-manual"})
	sm.ForceTransition(StatusInitializing)

	if !bytes.Contains(buf.Bytes(), []byte("inferred_from=manual")) {
		t.Errorf("expected inferred_from=manual when no metadata provided, got: %s", buf.String())
	}
}

func TestTransitionLog_RecentTransitions(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{AgentID: "test"})

	sm.ForceTransition(StatusInitializing)
	sm.ForceTransition(StatusBrowsing, StatusMetadata{InferredFrom: "CDP"})
	sm.ForceTransition(StatusFormFilling, StatusMetadata{InferredFrom: "workflow"})

	records := sm.RecentTransitions(10)
	if len(records) != 3 {
		t.Fatalf("expected 3 transition records, got %d", len(records))
	}

	if records[0].From != StatusOffline || records[0].To != StatusInitializing {
		t.Errorf("first record: expected OFFLINE -> INITIALIZING, got %s -> %s", records[0].From, records[0].To)
	}
	if records[0].InferredFrom != "manual" {
		t.Errorf("first record: expected inferred_from=manual, got %s", records[0].InferredFrom)
	}

	if records[1].From != StatusInitializing || records[1].To != StatusBrowsing {
		t.Errorf("second record: expected INITIALIZING -> BROWSING, got %s -> %s", records[1].From, records[1].To)
	}
	if records[1].InferredFrom != "CDP" {
		t.Errorf("second record: expected inferred_from=CDP, got %s", records[1].InferredFrom)
	}

	if records[2].From != StatusBrowsing || records[2].To != StatusFormFilling {
		t.Errorf("third record: expected BROWSING -> FORM_FILLING, got %s -> %s", records[2].From, records[2].To)
	}
	if records[2].InferredFrom != "workflow" {
		t.Errorf("third record: expected inferred_from=workflow, got %s", records[2].InferredFrom)
	}

	for i, r := range records {
		if r.Timestamp == 0 {
			t.Errorf("record %d: expected non-zero timestamp", i)
		}
	}
}

func TestTransitionLog_RecentTransitionsLimit(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{AgentID: "test"})

	sm.ForceTransition(StatusInitializing)
	sm.ForceTransition(StatusBrowsing)
	sm.ForceTransition(StatusFormFilling)

	records := sm.RecentTransitions(2)
	if len(records) != 2 {
		t.Fatalf("expected 2 records when limit=2, got %d", len(records))
	}
	if records[0].To != StatusBrowsing {
		t.Errorf("expected first limited record to=BROWSING, got %s", records[0].To)
	}
	if records[1].To != StatusFormFilling {
		t.Errorf("expected second limited record to=FORM_FILLING, got %s", records[1].To)
	}
}

func TestTransitionLog_RingBuffer(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{AgentID: "test"})

	for i := 0; i < 150; i++ {
		sm.ForceTransition(StatusBrowsing)
		sm.ForceTransition(StatusFormFilling)
	}

	records := sm.RecentTransitions(0)
	if len(records) > transitionLogMaxSize {
		t.Errorf("expected max %d transition records (ring buffer), got %d", transitionLogMaxSize, len(records))
	}

	if len(records) != transitionLogMaxSize {
		t.Errorf("expected exactly %d records after overflow, got %d", transitionLogMaxSize, len(records))
	}

	lastRecord := records[len(records)-1]
	if lastRecord.To != StatusFormFilling {
		t.Errorf("expected last record to=FORM_FILLING, got %s", lastRecord.To)
	}
}

func TestTransitionLog_TransitionPath(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	slog.SetDefault(logger)

	sm := NewStateMachine(StateMachineConfig{AgentID: "transition-path"})

	err := sm.Transition(StatusInitializing)
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	logOutput := buf.String()
	if logOutput == "" {
		t.Fatal("expected log output from Transition, got empty")
	}

	if !bytes.Contains(buf.Bytes(), []byte("agent_id=transition-path")) {
		t.Errorf("expected log to contain agent_id, got: %s", logOutput)
	}
	if !bytes.Contains(buf.Bytes(), []byte("from=OFFLINE")) {
		t.Errorf("expected log to contain from=OFFLINE, got: %s", logOutput)
	}
	if !bytes.Contains(buf.Bytes(), []byte("to=INITIALIZING")) {
		t.Errorf("expected log to contain to=INITIALIZING, got: %s", logOutput)
	}

	records := sm.RecentTransitions(10)
	if len(records) != 1 {
		t.Fatalf("expected 1 transition record, got %d", len(records))
	}
	if records[0].InferredFrom != "manual" {
		t.Errorf("expected inferred_from=manual, got %s", records[0].InferredFrom)
	}
}

func TestTransitionLog_ConcurrentTransitions(t *testing.T) {
	sm := NewStateMachine(StateMachineConfig{AgentID: "concurrent"})
	sm.ForceTransition(StatusBrowsing)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sm.ForceTransition(StatusFormFilling)
			sm.ForceTransition(StatusBrowsing)
		}()
	}
	wg.Wait()

	records := sm.RecentTransitions(0)
	if len(records) != transitionLogMaxSize {
		t.Errorf("expected exactly %d records after concurrent writes, got %d", transitionLogMaxSize, len(records))
	}
}
