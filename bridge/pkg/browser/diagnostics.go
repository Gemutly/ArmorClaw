package browser

import (
	"fmt"
	"sort"
)

// ChartDiff represents a single difference between expected and actual
// replay actions.
type ChartDiff struct {
	Action   string `json:"action"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Match    bool   `json:"match"`
}

// DiffCharts compares two NavCharts action-by-action and returns
// a slice of ChartDiff entries. Actions are compared in key-sorted
// order so results are deterministic.
func DiffCharts(expected, actual NavChart) []ChartDiff {
	var diffs []ChartDiff

	eKeys := sortedKeys(expected.ActionMap)
	aKeys := sortedKeys(actual.ActionMap)

	allKeys := make(map[string]struct{})
	for _, k := range eKeys {
		allKeys[k] = struct{}{}
	}
	for _, k := range aKeys {
		allKeys[k] = struct{}{}
	}

	sorted := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	for _, key := range sorted {
		eAction, eOK := expected.ActionMap[key]
		aAction, aOK := actual.ActionMap[key]

		if !eOK {
			diffs = append(diffs, ChartDiff{
				Action:   key,
				Expected: "",
				Actual:   actionSummary(aAction),
				Match:    false,
			})
			continue
		}
		if !aOK {
			diffs = append(diffs, ChartDiff{
				Action:   key,
				Expected: actionSummary(eAction),
				Actual:   "",
				Match:    false,
			})
			continue
		}

		eSummary := actionSummary(eAction)
		aSummary := actionSummary(aAction)
		diffs = append(diffs, ChartDiff{
			Action:   key,
			Expected: eSummary,
			Actual:   aSummary,
			Match:    eSummary == aSummary,
		})
	}

	return diffs
}

// DiffReplay compares the most recently stored chart for a tab against
// the provided replayed chart.
func DiffReplay(tabID string, store *MultiTabStore, replayed NavChart) ([]ChartDiff, error) {
	charts, err := store.GetCharts(tabID)
	if err != nil {
		return nil, fmt.Errorf("navchart diff: %w", err)
	}
	if len(charts) == 0 {
		return nil, fmt.Errorf("navchart diff: no stored charts for tab %q", tabID)
	}

	expected := charts[len(charts)-1]
	return DiffCharts(expected, replayed), nil
}

func actionSummary(a ChartAction) string {
	s := string(a.ActionType)
	if a.URL != "" {
		s += " " + a.URL
	}
	if a.Value != "" {
		s += " value=" + a.Value
	}
	if a.Selector != nil && a.Selector.PrimaryCSS != "" {
		s += " sel=" + a.Selector.PrimaryCSS
	}
	return s
}

func sortedKeys(m map[string]ChartAction) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
