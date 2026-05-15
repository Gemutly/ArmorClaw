// Package budget provides token budget tracking and enforcement for ArmorClaw
// persistence.go handles durable storage with Write-Ahead Log (WAL) support
package budget

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PersistenceMode controls how data is persisted
type PersistenceMode int

const (
	// PersistenceSync writes to WAL synchronously before returning (safest)
	PersistenceSync PersistenceMode = iota
	// PersistenceAsync writes to WAL in background (faster, small data loss window)
	PersistenceAsync
	// PersistenceDisabled no persistence (in-memory only)
	PersistenceDisabled
)

// PersistenceConfig configures the persistence behavior
type PersistenceConfig struct {
	// Mode controls sync vs async persistence
	Mode PersistenceMode

	// StateDir is the directory for state files
	StateDir string

	// SyncInterval controls how often async mode flushes (milliseconds)
	SyncInterval int

	// MaxWALSize limits WAL file size before compaction (bytes)
	MaxWALSize int64
}

// DefaultPersistenceConfig returns safe defaults
func DefaultPersistenceConfig() PersistenceConfig {
	return PersistenceConfig{
		Mode:         PersistenceSync,
		StateDir:     "/var/lib/armorclaw",
		SyncInterval: 1000,  // 1 second
		MaxWALSize:   10 * 1024 * 1024, // 10MB
	}
}

// walEntry represents a single entry in the Write-Ahead Log
type walEntry struct {
	Sequence   uint64      `json:"seq"`
	Type       string      `json:"type"` // "usage", "reset", "config"
	Timestamp  time.Time   `json:"ts"`
	Data       interface{} `json:"data"`
	Checksum   uint32      `json:"checksum"` // Simple checksum for integrity
}

// persistentStore handles durable storage with WAL
type persistentStore struct {
	config      PersistenceConfig
	walFile     *os.File
	walWriter   *bufio.Writer
	walPath     string
	statePath   string
	sequence    uint64
	mu          sync.Mutex
	flushChan   chan struct{}
	doneChan    chan struct{}
	wg          sync.WaitGroup
}

// newPersistentStore creates a new persistent store with WAL
func newPersistentStore(config PersistenceConfig) (*persistentStore, error) {
	// Ensure directory exists
	if err := os.MkdirAll(config.StateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	ps := &persistentStore{
		config:    config,
		walPath:   filepath.Join(config.StateDir, "budget.wal"),
		statePath: filepath.Join(config.StateDir, "budget_state.json"),
		flushChan: make(chan struct{}, 1),
		doneChan:  make(chan struct{}),
	}

	// Recover from existing WAL/state
	if err := ps.recover(); err != nil {
		return nil, fmt.Errorf("failed to recover state: %w", err)
	}

	// Open WAL for appending
	if config.Mode != PersistenceDisabled {
		if err := ps.openWAL(); err != nil {
			return nil, fmt.Errorf("failed to open WAL: %w", err)
		}

		// Start async flusher if needed
		if config.Mode == PersistenceAsync {
			ps.startAsyncFlusher()
		}
	}

	return ps, nil
}

// openWAL opens the WAL file for appending
func (ps *persistentStore) openWAL() error {
	f, err := os.OpenFile(ps.walPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open WAL file: %w", err)
	}

	ps.walFile = f
	ps.walWriter = bufio.NewWriterSize(f, 4096)
	return nil
}

// AppendUsage appends a usage record to the WAL
// Returns error if persistence fails (caller should decide whether to proceed)
func (ps *persistentStore) AppendUsage(record UsageRecord) error {
	if ps.config.Mode == PersistenceDisabled {
		return nil
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.sequence++
	entry := walEntry{
		Sequence:  ps.sequence,
		Type:      "usage",
		Timestamp: time.Now(),
		Data:      record,
		Checksum:  simpleChecksum(record),
	}

	return ps.writeEntry(entry)
}

// AppendReset logs a reset operation
func (ps *persistentStore) AppendReset() error {
	if ps.config.Mode == PersistenceDisabled {
		return nil
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.sequence++
	entry := walEntry{
		Sequence:  ps.sequence,
		Type:      "reset",
		Timestamp: time.Now(),
		Data:      nil,
	}

	return ps.writeEntry(entry)
}

// writeEntry writes an entry to the WAL
func (ps *persistentStore) writeEntry(entry walEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal WAL entry: %w", err)
	}

	// Write entry as single line with newline delimiter
	if _, err := ps.walWriter.Write(data); err != nil {
		return fmt.Errorf("failed to write WAL entry: %w", err)
	}
	if _, err := ps.walWriter.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write WAL newline: %w", err)
	}

	// In sync mode, flush immediately
	if ps.config.Mode == PersistenceSync {
		if err := ps.syncWAL(); err != nil {
			return err
		}
	}
	// In async mode, signal flusher
	if ps.config.Mode == PersistenceAsync {
		select {
		case ps.flushChan <- struct{}{}:
		default: // Already signaled
		}
	}

	return nil
}

// syncWAL flushes and syncs the WAL to disk
func (ps *persistentStore) syncWAL() error {
	if ps.walWriter == nil {
		return nil
	}

	// Flush buffer
	if err := ps.walWriter.Flush(); err != nil {
		return fmt.Errorf("failed to flush WAL: %w", err)
	}

	// Sync to disk (fsync)
	if err := ps.walFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync WAL: %w", err)
	}

	return nil
}

// startAsyncFlusher starts the background flush goroutine
func (ps *persistentStore) startAsyncFlusher() {
	ps.wg.Add(1)
	go func() {
		defer ps.wg.Done()

		ticker := time.NewTicker(time.Duration(ps.config.SyncInterval) * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ps.doneChan:
				// Final flush before shutdown
				ps.mu.Lock()
				ps.syncWAL()
				ps.mu.Unlock()
				return
			case <-ticker.C:
				ps.mu.Lock()
				ps.syncWAL()
				ps.mu.Unlock()
			case <-ps.flushChan:
				ps.mu.Lock()
				ps.syncWAL()
				ps.mu.Unlock()
			}
		}
	}()
}

// recover replays WAL and loads state
func (ps *persistentStore) recover() error {
	// First, try to load snapshot
	state, err := ps.loadSnapshot()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load snapshot: %w", err)
	}

	// Set sequence from snapshot
	if state != nil {
		ps.sequence = state.LastSequence
	}

	// Replay WAL entries after snapshot
	_, err = ps.replayWAL()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to replay WAL: %w", err)
	}

	return nil
}

// snapshotState is the full state saved to disk
type snapshotState struct {
	LastSequence uint64              `json:"last_sequence"`
	Config       BudgetConfig        `json:"config"`
	UsageHistory []UsageRecord       `json:"usage_history"`
	DailyUsage   map[string]float64  `json:"daily_usage"`
	MonthlyUsage map[string]float64  `json:"monthly_usage"`
	SessionUsage map[string]float64  `json:"session_usage"`
	SavedAt      time.Time           `json:"saved_at"`
}

// loadSnapshot loads the state from the snapshot file
func (ps *persistentStore) loadSnapshot() (*snapshotState, error) {
	data, err := os.ReadFile(ps.statePath)
	if err != nil {
		return nil, err
	}

	var state snapshotState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	return &state, nil
}

// SaveSnapshot saves current state to snapshot file atomically
func (ps *persistentStore) SaveSnapshot(tracker *BudgetTracker) error {
	if ps.config.Mode == PersistenceDisabled {
		return nil
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Get current state from tracker
	state := snapshotState{
		LastSequence: ps.sequence,
		Config:       tracker.config,
		UsageHistory: tracker.usageHistory,
		DailyUsage:   tracker.dailyUsage,
		MonthlyUsage: tracker.monthlyUsage,
		SessionUsage: tracker.sessionUsage,
		SavedAt:      time.Now(),
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	// Write atomically: temp file -> rename
	tempPath := ps.statePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp snapshot: %w", err)
	}

	// Sync temp file
	if f, err := os.Open(tempPath); err == nil {
		f.Sync()
		f.Close()
	}

	// Atomic rename
	if err := os.Rename(tempPath, ps.statePath); err != nil {
		return fmt.Errorf("failed to rename snapshot: %w", err)
	}

	// Compact WAL after successful snapshot
	if err := ps.compactWAL(); err != nil {
		// Log but don't fail - WAL compaction is optimization
		log.Printf("[BUDGET] Warning: WAL compaction failed: %v\n", err)
	}

	return nil
}

// replayWAL replays WAL entries to recover state
func (ps *persistentStore) replayWAL() ([]UsageRecord, error) {
	f, err := os.Open(ps.walPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var records []UsageRecord
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry walEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			// Corrupted entry, skip
			continue
		}

		// Only replay entries after our current sequence
		if entry.Sequence <= ps.sequence {
			continue
		}

		ps.sequence = entry.Sequence

		switch entry.Type {
		case "usage":
			// Parse usage record from data
			dataBytes, err := json.Marshal(entry.Data)
			if err != nil {
				continue
			}
			var record UsageRecord
			if err := json.Unmarshal(dataBytes, &record); err != nil {
				continue
			}
			records = append(records, record)
		case "reset":
			// Reset clears all usage
			records = nil
		}
	}

	return records, scanner.Err()
}

// compactWAL creates a fresh WAL after snapshot
func (ps *persistentStore) compactWAL() error {
	// Close current WAL
	if ps.walWriter != nil {
		ps.walWriter.Flush()
	}
	if ps.walFile != nil {
		ps.walFile.Close()
	}

	// Remove old WAL
	if err := os.Remove(ps.walPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Open fresh WAL
	return ps.openWAL()
}

// Close flushes and closes the persistent store
func (ps *persistentStore) Close() error {
	if ps.config.Mode == PersistenceDisabled {
		return nil
	}

	// Stop async flusher
	close(ps.doneChan)
	ps.wg.Wait()

	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Final sync
	if err := ps.syncWAL(); err != nil {
		return err
	}

	// Close WAL file
	if ps.walFile != nil {
		return ps.walFile.Close()
	}

	return nil
}

// simpleChecksum generates a simple checksum for data integrity
func simpleChecksum(data interface{}) uint32 {
	// Simple FNV-1a hash
	const (
		offset32 uint32 = 2166136261
		prime32  uint32 = 16777619
	)

	var hash uint32 = offset32

	// Convert to JSON bytes for hashing
	b, _ := json.Marshal(data)
	for _, c := range b {
		hash ^= uint32(c)
		hash *= prime32
	}

	return hash
}

// ReadFrom reads state from an io.Reader (for testing/loading)
func ReadFrom(r io.Reader) (*snapshotState, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var state snapshotState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// WriteTo writes state to an io.Writer (for testing/saving)
func WriteTo(w io.Writer, state *snapshotState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	_, err = w.Write(data)
	return err
}
