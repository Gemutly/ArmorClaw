package browser

import (
	"fmt"
	"sync"
)

// MultiTabStore manages NavCharts per browser tab in a thread-safe manner.
// An empty tabID denotes the global (single-tab) scope.
type MultiTabStore struct {
	mu     sync.RWMutex
	charts map[string][]NavChart
}

// NewMultiTabStore creates an initialized MultiTabStore.
func NewMultiTabStore() *MultiTabStore {
	return &MultiTabStore{
		charts: make(map[string][]NavChart),
	}
}

// StoreChart appends a NavChart for the given tabID.
// The chart's TabID field is set to the provided tabID.
func (s *MultiTabStore) StoreChart(tabID string, chart NavChart) error {
	if tabID == "" {
		return fmt.Errorf("navchart: tabID must not be empty")
	}
	chart.TabID = tabID

	s.mu.Lock()
	s.charts[tabID] = append(s.charts[tabID], chart)
	s.mu.Unlock()
	return nil
}

// GetCharts returns all NavCharts stored for the given tabID.
// Returns an empty slice (not nil) if the tab has no charts.
func (s *MultiTabStore) GetCharts(tabID string) ([]NavChart, error) {
	s.mu.RLock()
	charts := s.charts[tabID]
	s.mu.RUnlock()

	if charts == nil {
		return []NavChart{}, nil
	}
	out := make([]NavChart, len(charts))
	copy(out, charts)
	return out, nil
}

// RemoveTab deletes all charts associated with the given tabID.
// It is safe to call for a tab that has no stored charts.
func (s *MultiTabStore) RemoveTab(tabID string) error {
	s.mu.Lock()
	delete(s.charts, tabID)
	s.mu.Unlock()
	return nil
}

// TabIDs returns all tab IDs that currently have stored charts.
func (s *MultiTabStore) TabIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.charts))
	for id := range s.charts {
		ids = append(ids, id)
	}
	return ids
}
