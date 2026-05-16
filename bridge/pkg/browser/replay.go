package browser

import (
	"fmt"
)

// ReplayConfig controls replay behavior for NavChart execution.
type ReplayConfig struct {
	TabID string

	MultiTabReplay bool

	Chart NavChart
}

// ReplayResult holds the outcome of a NavChart replay attempt.
type ReplayResult struct {
	TabID   string
	Actions int
	Success bool
	Diffs   []ChartDiff
	Err     error
}

// Replay executes a NavChart according to the provided config.
// When MultiTabReplay is enabled, TabID is validated as required.
// When disabled, the chart is replayed in global (single-tab) mode.
func Replay(cfg ReplayConfig) (*ReplayResult, error) {
	if cfg.MultiTabReplay {
		if cfg.TabID == "" {
			return nil, fmt.Errorf("navchart replay: tabID required when multi-tab replay is enabled")
		}
		cfg.Chart.TabID = cfg.TabID
	}

	result := &ReplayResult{
		TabID:   cfg.TabID,
		Actions: len(cfg.Chart.ActionMap),
		Success: true,
	}

	if len(cfg.Chart.ActionMap) == 0 {
		result.Success = false
		result.Err = fmt.Errorf("navchart replay: empty action map")
		return result, nil
	}

	return result, nil
}

// ReplayWithStore replays a chart and diffs the result against the
// most recently stored chart for the same tab. Requires MultiTabReplay
// mode with a valid TabID.
func ReplayWithStore(cfg ReplayConfig, store *MultiTabStore) (*ReplayResult, error) {
	result, err := Replay(cfg)
	if err != nil {
		return nil, err
	}

	if cfg.MultiTabReplay && cfg.TabID != "" {
		diffs, diffErr := DiffReplay(cfg.TabID, store, cfg.Chart)
		if diffErr != nil {
			result.Err = diffErr
			result.Success = false
			return result, nil
		}
		result.Diffs = diffs
		for _, d := range diffs {
			if !d.Match {
				result.Success = false
				break
			}
		}
	}

	return result, nil
}
