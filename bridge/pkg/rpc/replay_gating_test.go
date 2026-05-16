package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/armorclaw/jetski/navchart"
)

// TestReplayGating_FlagOn_ReturnsData verifies that when MultiTabReplay is enabled,
// the handler returns replay diagnostics data (success response) instead of an error.
func TestReplayGating_FlagOn_ReturnsData(t *testing.T) {
	store := navchart.NewMultiTabStore()
	s := &Server{
		navChartStore: store,
	}
	s.replayFlags.MultiTabReplay = true
	s.registerHandlers()

	// Store a chart so the handler has data to return.
	chart := navchart.NavChart{
		Version:      1,
		TargetDomain: "example.com",
		TabID:        "tab-gating-on",
		ActionMap: map[string]navchart.ChartAction{
			"step1": {ActionType: navchart.ActionNavigate, URL: "https://example.com"},
		},
	}
	if err := store.StoreChart("tab-gating-on", chart); err != nil {
		t.Fatalf("StoreChart: %v", err)
	}

	handler, ok := s.handlers["browser.replay_diagnostics"]
	if !ok {
		t.Fatal("browser.replay_diagnostics not registered")
	}

	req := &Request{
		Params: json.RawMessage(`{"tab_id":"tab-gating-on"}`),
	}

	result, errObj := handler(context.Background(), req)
	if errObj != nil {
		t.Fatalf("expected success, got error: code=%d message=%s", errObj.Code, errObj.Message)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if resultMap["tab_id"] != "tab-gating-on" {
		t.Errorf("expected tab_id 'tab-gating-on', got %v", resultMap["tab_id"])
	}
}

// TestReplayGating_FlagOff_ReturnsFeatureDisabled verifies that when MultiTabReplay is
// disabled, the handler returns a feature-disabled error (MethodNotFound code -32601).
// Critically, the method IS registered (not -32601 from the dispatcher), but the
// handler itself returns the disabled error.
func TestReplayGating_FlagOff_ReturnsFeatureDisabled(t *testing.T) {
	s := &Server{}
	// Default: replayFlags.MultiTabReplay is false (zero value)
	s.registerHandlers()

	// Confirm handler is registered (handler-gated, not registration-gated).
	handler, ok := s.handlers["browser.replay_diagnostics"]
	if !ok {
		t.Fatal("browser.replay_diagnostics must be registered even when flag is off")
	}

	req := &Request{
		Params: json.RawMessage(`{"tab_id":"tab-1"}`),
	}

	result, errObj := handler(context.Background(), req)
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
	if errObj == nil {
		t.Fatal("expected error when MultiTabReplay flag is off")
	}

	// The handler uses MethodNotFound code for feature-disabled, which is the
	// handler-gated pattern. The method is always registered; the handler
	// returns -32601 internally to signal "this feature is off".
	if errObj.Code != MethodNotFound {
		t.Errorf("expected code %d (MethodNotFound), got %d", MethodNotFound, errObj.Code)
	}
	if errObj.Message != "Feature disabled: multi_tab_replay" {
		t.Errorf("unexpected message: %s", errObj.Message)
	}
}

// TestReplayGating_Discovery verifies that the handler map size is
// stable regardless of the replay flag state. replay_diagnostics is
// always registered (handler-gated, not registration-gated), so toggling the flag
// does not change the method count.
func TestReplayGating_Discovery(t *testing.T) {
	const expectedMethods = 142

	cases := []struct {
		name      string
		flagOn    bool
	}{
		{"flag_off", false},
		{"flag_on", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{}
			s.replayFlags.MultiTabReplay = tc.flagOn
			s.registerHandlers()

			count := len(s.handlers)
			if count != expectedMethods {
				t.Errorf("expected %d registered methods, got %d", expectedMethods, count)
			}

			// replay_diagnostics must always be present regardless of flag.
			if _, ok := s.handlers["browser.replay_diagnostics"]; !ok {
				t.Error("browser.replay_diagnostics missing from handler map")
			}
		})
	}
}
