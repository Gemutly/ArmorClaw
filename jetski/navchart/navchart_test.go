package navchart

import (
	"testing"
)

func TestMultiTabStore(t *testing.T) {
	store := NewMultiTabStore()

	chart1 := NavChart{
		Version:      1,
		TargetDomain: "example.com",
		ActionMap: map[string]ChartAction{
			"step1": {ActionType: ActionClick, Selector: &ChartSelector{PrimaryCSS: "#btn"}},
		},
	}
	chart2 := NavChart{
		Version:      1,
		TargetDomain: "example.com",
		ActionMap: map[string]ChartAction{
			"step1": {ActionType: ActionNavigate, URL: "https://example.com/page2"},
		},
	}

	err := store.StoreChart("tab-a", chart1)
	if err != nil {
		t.Fatalf("StoreChart tab-a: %v", err)
	}
	err = store.StoreChart("tab-b", chart2)
	if err != nil {
		t.Fatalf("StoreChart tab-b: %v", err)
	}

	chartsA, err := store.GetCharts("tab-a")
	if err != nil {
		t.Fatalf("GetCharts tab-a: %v", err)
	}
	if len(chartsA) != 1 {
		t.Fatalf("expected 1 chart for tab-a, got %d", len(chartsA))
	}
	if chartsA[0].TabID != "tab-a" {
		t.Errorf("expected TabID=tab-a, got %q", chartsA[0].TabID)
	}

	chartsB, err := store.GetCharts("tab-b")
	if err != nil {
		t.Fatalf("GetCharts tab-b: %v", err)
	}
	if len(chartsB) != 1 {
		t.Fatalf("expected 1 chart for tab-b, got %d", len(chartsB))
	}

	unknown, err := store.GetCharts("tab-unknown")
	if err != nil {
		t.Fatalf("GetCharts unknown: %v", err)
	}
	if len(unknown) != 0 {
		t.Errorf("expected empty slice for unknown tab, got %d", len(unknown))
	}
}

func TestStoreChartRejectsEmptyTabID(t *testing.T) {
	store := NewMultiTabStore()
	err := store.StoreChart("", NavChart{})
	if err == nil {
		t.Fatal("expected error for empty tabID")
	}
}

func TestRemoveTab(t *testing.T) {
	store := NewMultiTabStore()

	chart := NavChart{
		Version:      1,
		TargetDomain: "example.com",
		ActionMap: map[string]ChartAction{
			"step1": {ActionType: ActionClick},
		},
	}

	store.StoreChart("tab-x", chart)
	store.StoreChart("tab-x", chart)

	charts, _ := store.GetCharts("tab-x")
	if len(charts) != 2 {
		t.Fatalf("expected 2 charts, got %d", len(charts))
	}

	err := store.RemoveTab("tab-x")
	if err != nil {
		t.Fatalf("RemoveTab: %v", err)
	}

	charts, _ = store.GetCharts("tab-x")
	if len(charts) != 0 {
		t.Fatalf("expected 0 charts after remove, got %d", len(charts))
	}
}

func TestDiffCharts(t *testing.T) {
	expected := NavChart{
		ActionMap: map[string]ChartAction{
			"step1": {ActionType: ActionClick, Selector: &ChartSelector{PrimaryCSS: "#btn"}},
			"step2": {ActionType: ActionInput, Value: "hello"},
		},
	}
	actual := NavChart{
		ActionMap: map[string]ChartAction{
			"step1": {ActionType: ActionClick, Selector: &ChartSelector{PrimaryCSS: "#btn"}},
			"step2": {ActionType: ActionNavigate, URL: "https://other.com"},
		},
	}

	diffs := DiffCharts(expected, actual)
	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d", len(diffs))
	}

	for _, d := range diffs {
		if d.Action == "step1" && !d.Match {
			t.Error("step1 should match")
		}
		if d.Action == "step2" && d.Match {
			t.Error("step2 should not match")
		}
	}
}

func TestDiffChartsSurplusActions(t *testing.T) {
	expected := NavChart{
		ActionMap: map[string]ChartAction{
			"step1": {ActionType: ActionClick},
		},
	}
	actual := NavChart{
		ActionMap: map[string]ChartAction{
			"step1": {ActionType: ActionClick},
			"step2": {ActionType: ActionNavigate, URL: "/extra"},
		},
	}

	diffs := DiffCharts(expected, actual)
	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d", len(diffs))
	}

	found := false
	for _, d := range diffs {
		if d.Action == "step2" {
			found = true
			if d.Expected != "" {
				t.Error("step2 expected should be empty (surplus in actual)")
			}
			if d.Match {
				t.Error("step2 should not match")
			}
		}
	}
	if !found {
		t.Error("step2 diff not found")
	}
}

func TestDiffReplay(t *testing.T) {
	store := NewMultiTabStore()

	stored := NavChart{
		ActionMap: map[string]ChartAction{
			"step1": {ActionType: ActionClick, Selector: &ChartSelector{PrimaryCSS: "#btn"}},
		},
	}
	store.StoreChart("tab-1", stored)

	replayed := NavChart{
		ActionMap: map[string]ChartAction{
			"step1": {ActionType: ActionClick, Selector: &ChartSelector{PrimaryCSS: "#btn"}},
		},
	}

	diffs, err := DiffReplay("tab-1", store, replayed)
	if err != nil {
		t.Fatalf("DiffReplay: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if !diffs[0].Match {
		t.Error("expected match")
	}
}

func TestDiffReplayNoStoredCharts(t *testing.T) {
	store := NewMultiTabStore()
	_, err := DiffReplay("tab-missing", store, NavChart{})
	if err == nil {
		t.Fatal("expected error for missing tab")
	}
}

func TestReplayMultiTabRequiresTabID(t *testing.T) {
	cfg := ReplayConfig{
		MultiTabReplay: true,
		TabID:          "",
		Chart: NavChart{
			ActionMap: map[string]ChartAction{
				"step1": {ActionType: ActionClick},
			},
		},
	}
	_, err := Replay(cfg)
	if err == nil {
		t.Fatal("expected error when multi-tab replay has no tabID")
	}
}

func TestReplaySuccess(t *testing.T) {
	cfg := ReplayConfig{
		MultiTabReplay: false,
		Chart: NavChart{
			ActionMap: map[string]ChartAction{
				"step1": {ActionType: ActionClick},
			},
		},
	}
	result, err := Replay(cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.Actions != 1 {
		t.Errorf("expected 1 action, got %d", result.Actions)
	}
}

func TestReplayEmptyActionMap(t *testing.T) {
	cfg := ReplayConfig{
		Chart: NavChart{ActionMap: map[string]ChartAction{}},
	}
	result, err := Replay(cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if result.Success {
		t.Error("expected failure for empty action map")
	}
}

func TestReplayWithStore(t *testing.T) {
	store := NewMultiTabStore()

	stored := NavChart{
		ActionMap: map[string]ChartAction{
			"step1": {ActionType: ActionClick, Selector: &ChartSelector{PrimaryCSS: "#go"}},
		},
	}
	store.StoreChart("tab-1", stored)

	cfg := ReplayConfig{
		TabID:          "tab-1",
		MultiTabReplay: true,
		Chart: NavChart{
			ActionMap: map[string]ChartAction{
				"step1": {ActionType: ActionClick, Selector: &ChartSelector{PrimaryCSS: "#go"}},
			},
		},
	}

	result, err := ReplayWithStore(cfg, store)
	if err != nil {
		t.Fatalf("ReplayWithStore: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if len(result.Diffs) != 1 || !result.Diffs[0].Match {
		t.Error("expected matching diff")
	}
}

func TestGetChartsReturnsCopy(t *testing.T) {
	store := NewMultiTabStore()
	store.StoreChart("tab-c", NavChart{
		ActionMap: map[string]ChartAction{"s": {ActionType: ActionClick}},
	})

	charts, _ := store.GetCharts("tab-c")
	charts[0].TargetDomain = "mutated"

	original, _ := store.GetCharts("tab-c")
	if original[0].TargetDomain == "mutated" {
		t.Error("GetCharts should return a copy, not a reference")
	}
}
