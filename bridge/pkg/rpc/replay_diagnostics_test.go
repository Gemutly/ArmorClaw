package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/armorclaw/jetski/navchart"
)

func TestBrowserReplayDiagnostics_FlagOff(t *testing.T) {
	s := &Server{}
	s.registerHandlers()

	handler, ok := s.handlers["browser.replay_diagnostics"]
	if !ok {
		t.Fatal("browser.replay_diagnostics not registered")
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
	if errObj.Code != MethodNotFound {
		t.Errorf("expected code %d (-32601), got %d", MethodNotFound, errObj.Code)
	}
	if errObj.Message != "Feature disabled: multi_tab_replay" {
		t.Errorf("unexpected message: %s", errObj.Message)
	}
}

func TestBrowserReplayDiagnostics_MissingTabID(t *testing.T) {
	store := navchart.NewMultiTabStore()
	s := &Server{
		navChartStore: store,
	}
	s.replayFlags.MultiTabReplay = true
	s.registerHandlers()

	handler := s.handlers["browser.replay_diagnostics"]

	req := &Request{
		Params: json.RawMessage(`{"session_id":"sess-1"}`),
	}

	_, errObj := handler(context.Background(), req)
	if errObj == nil {
		t.Fatal("expected error for missing tab_id")
	}
	if errObj.Code != InvalidParams {
		t.Errorf("expected InvalidParams, got %d", errObj.Code)
	}
}

func TestBrowserReplayDiagnostics_NoStoredCharts(t *testing.T) {
	store := navchart.NewMultiTabStore()
	s := &Server{
		navChartStore: store,
	}
	s.replayFlags.MultiTabReplay = true
	s.registerHandlers()

	handler := s.handlers["browser.replay_diagnostics"]

	req := &Request{
		Params: json.RawMessage(`{"tab_id":"tab-nocharts"}`),
	}

	_, errObj := handler(context.Background(), req)
	if errObj == nil {
		t.Fatal("expected error for no stored charts")
	}
	if errObj.Code != InvalidParams {
		t.Errorf("expected InvalidParams, got %d", errObj.Code)
	}
}

func TestBrowserReplayDiagnostics_SingleChart(t *testing.T) {
	store := navchart.NewMultiTabStore()
	s := &Server{
		navChartStore: store,
	}
	s.replayFlags.MultiTabReplay = true
	s.registerHandlers()

	chart := navchart.NavChart{
		Version:      1,
		TargetDomain: "example.com",
		TabID:        "tab-1",
		ActionMap: map[string]navchart.ChartAction{
			"step1": {ActionType: navchart.ActionNavigate, URL: "https://example.com"},
		},
	}
	if err := store.StoreChart("tab-1", chart); err != nil {
		t.Fatalf("StoreChart: %v", err)
	}

	handler := s.handlers["browser.replay_diagnostics"]
	req := &Request{
		Params: json.RawMessage(`{"tab_id":"tab-1"}`),
	}

	result, errObj := handler(context.Background(), req)
	if errObj != nil {
		t.Fatalf("unexpected error: %v", errObj)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}

	if resultMap["tab_id"] != "tab-1" {
		t.Errorf("expected tab_id 'tab-1', got %v", resultMap["tab_id"])
	}
	if resultMap["match_percentage"] != 100.0 {
		t.Errorf("expected match_percentage 100.0, got %v", resultMap["match_percentage"])
	}

	diffs, ok := resultMap["diffs"].([]navchart.ChartDiff)
	if !ok {
		t.Fatalf("expected []ChartDiff, got %T", resultMap["diffs"])
	}
	if len(diffs) != 0 {
		t.Errorf("expected 0 diffs for single chart, got %d", len(diffs))
	}
}

func TestBrowserReplayDiagnostics_TwoChartsWithDiff(t *testing.T) {
	store := navchart.NewMultiTabStore()
	s := &Server{
		navChartStore: store,
	}
	s.replayFlags.MultiTabReplay = true
	s.registerHandlers()

	chart1 := navchart.NavChart{
		Version:      1,
		TargetDomain: "example.com",
		TabID:        "tab-2",
		ActionMap: map[string]navchart.ChartAction{
			"step1": {ActionType: navchart.ActionNavigate, URL: "https://example.com"},
			"step2": {ActionType: navchart.ActionClick, Selector: &navchart.ChartSelector{PrimaryCSS: "#btn"}},
		},
	}
	chart2 := navchart.NavChart{
		Version:      1,
		TargetDomain: "example.com",
		TabID:        "tab-2",
		ActionMap: map[string]navchart.ChartAction{
			"step1": {ActionType: navchart.ActionNavigate, URL: "https://example.com"},
			"step2": {ActionType: navchart.ActionInput, Value: "hello"},
		},
	}
	if err := store.StoreChart("tab-2", chart1); err != nil {
		t.Fatalf("StoreChart 1: %v", err)
	}
	if err := store.StoreChart("tab-2", chart2); err != nil {
		t.Fatalf("StoreChart 2: %v", err)
	}

	handler := s.handlers["browser.replay_diagnostics"]
	req := &Request{
		Params: json.RawMessage(`{"tab_id":"tab-2","session_id":"sess-abc"}`),
	}

	result, errObj := handler(context.Background(), req)
	if errObj != nil {
		t.Fatalf("unexpected error: %v", errObj)
	}

	resultMap := result.(map[string]interface{})
	diffs := resultMap["diffs"].([]navchart.ChartDiff)

	if len(diffs) == 0 {
		t.Fatal("expected diffs for mismatched charts, got 0")
	}

	matchPct := resultMap["match_percentage"].(float64)
	if matchPct >= 100.0 {
		t.Errorf("expected match_percentage < 100 for mismatched charts, got %v", matchPct)
	}
}

func TestBrowserReplayDiagnostics_NilStore(t *testing.T) {
	s := &Server{}
	s.replayFlags.MultiTabReplay = true
	s.registerHandlers()

	handler := s.handlers["browser.replay_diagnostics"]
	req := &Request{
		Params: json.RawMessage(`{"tab_id":"tab-1"}`),
	}

	_, errObj := handler(context.Background(), req)
	if errObj == nil {
		t.Fatal("expected error when navChartStore is nil")
	}
	if errObj.Code != InternalError {
		t.Errorf("expected InternalError, got %d", errObj.Code)
	}
}

func TestBrowserReplayDiagnostics_Registration(t *testing.T) {
	s := &Server{}
	s.registerHandlers()

	if _, ok := s.handlers["browser.replay_diagnostics"]; !ok {
		t.Error("browser.replay_diagnostics not registered in handlers map")
	}
}
