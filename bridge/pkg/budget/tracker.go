// Package budget provides token budget tracking and enforcement for ArmorClaw
package budget

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/armorclaw/bridge/pkg/notification"
)

// TokenCosts defines approximate costs per 1M tokens (input + output average)
// Prices are in USD as of 2024
var TokenCosts = map[string]float64{
	"gpt-4":          30.00,
	"gpt-4-turbo":    10.00,
	"gpt-3.5-turbo":  0.50,
	"claude-3-opus":  15.00,
	"claude-3-sonnet": 3.00,
	"claude-3-haiku":  0.25,
}

// WorkflowState represents the operational state of a workflow/session
// considering budget constraints
type WorkflowState int

const (
	// WorkflowRunning means the workflow is actively processing
	WorkflowRunning WorkflowState = iota
	// WorkflowPaused means temporarily paused (user-initiated)
	WorkflowPaused
	// WorkflowPausedInsufficientFunds means paused due to budget exhaustion
	WorkflowPausedInsufficientFunds
	// WorkflowCompleted means finished successfully
	WorkflowCompleted
	// WorkflowFailed means terminated due to error
	WorkflowFailed
)

// String returns the string representation of the workflow state
func (s WorkflowState) String() string {
	switch s {
	case WorkflowRunning:
		return "running"
	case WorkflowPaused:
		return "paused"
	case WorkflowPausedInsufficientFunds:
		return "paused_insufficient_funds"
	case WorkflowCompleted:
		return "completed"
	case WorkflowFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// IsPaused returns true if the workflow is in any paused state
func (s WorkflowState) IsPaused() bool {
	return s == WorkflowPaused || s == WorkflowPausedInsufficientFunds
}

// CanResume returns true if the workflow can be resumed
func (s WorkflowState) CanResume() bool {
	return s == WorkflowPaused
	// Note: WorkflowPausedInsufficientFunds requires budget top-up before resuming
}

// UsageRecord tracks token usage for a session
type UsageRecord struct {
	SessionID    string    `json:"session_id"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	Timestamp    time.Time `json:"timestamp"`
	CostUSD      float64   `json:"cost_usd"`
}

// BudgetConfig defines budget limits
type BudgetConfig struct {
	DailyLimitUSD   float64 `toml:"daily_limit_usd" env:"ARMORCLAW_DAILY_LIMIT"`
	MonthlyLimitUSD float64 `toml:"monthly_limit_usd" env:"ARMORCLAW_MONTHLY_LIMIT"`
	AlertThreshold  float64 `toml:"alert_threshold" env:"ARMORCLAW_ALERT_THRESHOLD"` // % of limit
	HardStop        bool    `toml:"hard_stop" env:"ARMORCLAW_HARD_STOP"`
	ProviderCosts   map[string]float64 `toml:"provider_costs"`
}

// BudgetTracker monitors and enforces token budgets
// Thread-safe with Write-Ahead Log persistence for crash recovery
type BudgetTracker struct {
	config        BudgetConfig
	usageHistory  []UsageRecord
	dailyUsage    map[string]float64
	monthlyUsage  map[string]float64
	sessionUsage  map[string]float64
	mutex         sync.RWMutex
	costs         map[string]float64
	notifier      *notification.Notifier

	// Persistence layer (WAL-based)
	persistence   *persistentStore
	persistConfig PersistenceConfig
}

// BudgetTrackerOption configures the budget tracker
type BudgetTrackerOption func(*BudgetTracker)

// WithPersistence sets the persistence configuration
func WithPersistence(config PersistenceConfig) BudgetTrackerOption {
	return func(b *BudgetTracker) {
		b.persistConfig = config
	}
}

// WithNotifier sets the notification system
func WithNotifier(notifier *notification.Notifier) BudgetTrackerOption {
	return func(b *BudgetTracker) {
		b.notifier = notifier
	}
}

// NewBudgetTracker creates a new budget tracker with optional persistence
func NewBudgetTracker(config BudgetConfig, opts ...BudgetTrackerOption) (*BudgetTracker, error) {
	// Merge custom provider costs with defaults
	costs := make(map[string]float64)
	for k, v := range TokenCosts {
		costs[k] = v
	}
	if config.ProviderCosts != nil {
		for k, v := range config.ProviderCosts {
			costs[k] = v
		}
	}

	// Default persistence config (disabled for backward compatibility)
	persistConfig := PersistenceConfig{
		Mode:     PersistenceDisabled,
		StateDir: "/var/lib/armorclaw",
	}

	b := &BudgetTracker{
		config:        config,
		usageHistory:  make([]UsageRecord, 0),
		dailyUsage:    make(map[string]float64),
		monthlyUsage:  make(map[string]float64),
		sessionUsage:  make(map[string]float64),
		costs:         costs,
		persistConfig: persistConfig,
	}

	// Apply options
	for _, opt := range opts {
		opt(b)
	}

	// Initialize persistence if enabled
	if b.persistConfig.Mode != PersistenceDisabled {
		ps, err := newPersistentStore(b.persistConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize persistence: %w", err)
		}
		b.persistence = ps

		// Recover state from WAL/snapshot
		if err := b.recoverFromPersistence(); err != nil {
			return nil, fmt.Errorf("failed to recover state: %w", err)
		}
	}

	return b, nil
}

// recoverFromPersistence replays WAL and restores state
func (b *BudgetTracker) recoverFromPersistence() error {
	if b.persistence == nil {
		return nil
	}

	// Load snapshot first
	state, err := b.persistence.loadSnapshot()
	if err == nil && state != nil {
		b.config = state.Config
		b.usageHistory = state.UsageHistory
		b.dailyUsage = state.DailyUsage
		b.monthlyUsage = state.MonthlyUsage
		b.sessionUsage = state.SessionUsage
	}

	// Replay any WAL entries after snapshot
	records, err := b.persistence.replayWAL()
	if err != nil {
		return err
	}

	// Apply replayed records
	for _, record := range records {
		b.applyRecord(record)
	}

	return nil
}

// applyRecord applies a usage record to the in-memory state (without persistence)
func (b *BudgetTracker) applyRecord(record UsageRecord) {
	date := record.Timestamp.Format("2006-01-02")
	month := record.Timestamp.Format("2006-01")
	b.dailyUsage[date] += record.CostUSD
	b.monthlyUsage[month] += record.CostUSD
	b.sessionUsage[record.SessionID] += record.CostUSD
	b.usageHistory = append(b.usageHistory, record)
}

// RecordUsage records token usage for a session
// With synchronous persistence enabled, this writes to WAL before returning
// to guarantee no data loss on crash
func (b *BudgetTracker) RecordUsage(record UsageRecord) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	// Calculate cost
	costPer1M := b.costs[record.Model]
	if costPer1M == 0 {
		costPer1M = 0.001 // Default fallback
	}

	totalTokens := record.InputTokens + record.OutputTokens
	record.CostUSD = float64(totalTokens) / 1000000.0 * costPer1M
	record.Timestamp = time.Now()

	// Persist to WAL FIRST (before memory update) for durability
	// If persistence fails, we don't update memory state
	if b.persistence != nil {
		if err := b.persistence.AppendUsage(record); err != nil {
			return fmt.Errorf("failed to persist usage record: %w", err)
		}
	}

	// Now update in-memory state
	b.usageHistory = append(b.usageHistory, record)

	// Update tracking maps
	date := record.Timestamp.Format("2006-01-02")
	month := record.Timestamp.Format("2006-01")
	b.dailyUsage[date] += record.CostUSD
	b.monthlyUsage[month] += record.CostUSD
	b.sessionUsage[record.SessionID] += record.CostUSD

	// Check limits
	return b.checkLimits(&record)
}

// checkLimits verifies if usage is within budget limits
func (b *BudgetTracker) checkLimits(record *UsageRecord) error {
	date := record.Timestamp.Format("2006-01-02")
	month := record.Timestamp.Format("2006-01")

	dailyCost := b.dailyUsage[date]
	monthlyCost := b.monthlyUsage[month]

	// Check daily limit
	if b.config.DailyLimitUSD > 0 && dailyCost >= b.config.DailyLimitUSD {
		if b.config.HardStop {
			return fmt.Errorf("daily budget limit exceeded: $%.2f / $%.2f",
				dailyCost, b.config.DailyLimitUSD)
		}
		// Send alert
		b.sendAlert("daily_limit_exceeded", dailyCost, b.config.DailyLimitUSD)
	}

	// Check monthly limit
	if b.config.MonthlyLimitUSD > 0 && monthlyCost >= b.config.MonthlyLimitUSD {
		if b.config.HardStop {
			return fmt.Errorf("monthly budget limit exceeded: $%.2f / $%.2f",
				monthlyCost, b.config.MonthlyLimitUSD)
		}
		b.sendAlert("monthly_limit_exceeded", monthlyCost, b.config.MonthlyLimitUSD)
	}

	// Check alert threshold
	if b.config.AlertThreshold > 0 {
		if b.config.DailyLimitUSD > 0 {
			dailyPercent := (dailyCost / b.config.DailyLimitUSD) * 100
			if dailyPercent >= b.config.AlertThreshold {
				b.sendAlert("daily_budget_warning", dailyCost, b.config.DailyLimitUSD)
			}
		}

		if b.config.MonthlyLimitUSD > 0 {
			monthlyPercent := (monthlyCost / b.config.MonthlyLimitUSD) * 100
			if monthlyPercent >= b.config.AlertThreshold {
				b.sendAlert("monthly_budget_warning", monthlyCost, b.config.MonthlyLimitUSD)
			}
		}
	}

	return nil
}

// CanStartSession checks if a new session can be started based on budget
func (b *BudgetTracker) CanStartSession() error {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	now := time.Now()
	date := now.Format("2006-01-02")
	month := now.Format("2006-01")

	dailyCost := b.dailyUsage[date]
	monthlyCost := b.monthlyUsage[month]

	// Check if we've already hit the limits
	if b.config.HardStop {
		if b.config.DailyLimitUSD > 0 && dailyCost >= b.config.DailyLimitUSD {
			return fmt.Errorf("daily budget limit reached: $%.2f / $%.2f",
				dailyCost, b.config.DailyLimitUSD)
		}

		if b.config.MonthlyLimitUSD > 0 && monthlyCost >= b.config.MonthlyLimitUSD {
			return fmt.Errorf("monthly budget limit reached: $%.2f / $%.2f",
				monthlyCost, b.config.MonthlyLimitUSD)
		}
	}

	return nil
}

// GetWorkflowState returns the current workflow state based on budget status
// This is used to determine if active sessions should be paused
func (b *BudgetTracker) GetWorkflowState() WorkflowState {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	now := time.Now()
	date := now.Format("2006-01-02")
	month := now.Format("2006-01")

	dailyCost := b.dailyUsage[date]
	monthlyCost := b.monthlyUsage[month]

	// Check if budget is exhausted
	if b.config.DailyLimitUSD > 0 && dailyCost >= b.config.DailyLimitUSD {
		return WorkflowPausedInsufficientFunds
	}

	if b.config.MonthlyLimitUSD > 0 && monthlyCost >= b.config.MonthlyLimitUSD {
		return WorkflowPausedInsufficientFunds
	}

	return WorkflowRunning
}

// CanResumeWorkflow checks if a paused workflow can be resumed
// Returns error if budget is still exhausted
func (b *BudgetTracker) CanResumeWorkflow() error {
	state := b.GetWorkflowState()
	if state == WorkflowPausedInsufficientFunds {
		now := time.Now()
		date := now.Format("2006-01-02")
		month := now.Format("2006-01")

		b.mutex.RLock()
		dailyCost := b.dailyUsage[date]
		monthlyCost := b.monthlyUsage[month]
		b.mutex.RUnlock()

		limit := b.config.DailyLimitUSD
		current := dailyCost
		limitType := "daily"

		if b.config.MonthlyLimitUSD > 0 && monthlyCost >= b.config.MonthlyLimitUSD {
			limit = b.config.MonthlyLimitUSD
			current = monthlyCost
			limitType = "monthly"
		}

		return fmt.Errorf("cannot resume: %s budget exhausted ($%.2f of $%.2f)",
			limitType, current, limit)
	}

	return nil
}

// GetDailyUsage returns current daily usage in USD
func (b *BudgetTracker) GetDailyUsage() float64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	date := time.Now().Format("2006-01-02")
	return b.dailyUsage[date]
}

// GetMonthlyUsage returns current monthly usage in USD
func (b *BudgetTracker) GetMonthlyUsage() float64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	month := time.Now().Format("2006-01")
	return b.monthlyUsage[month]
}

// GetSessionUsage returns usage for a specific session
func (b *BudgetTracker) GetSessionUsage(sessionID string) float64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.sessionUsage[sessionID]
}

// GetDailyLimit returns the configured daily limit
func (b *BudgetTracker) GetDailyLimit() float64 {
	return b.config.DailyLimitUSD
}

// GetMonthlyLimit returns the configured monthly limit
func (b *BudgetTracker) GetMonthlyLimit() float64 {
	return b.config.MonthlyLimitUSD
}

// sendAlert sends a budget alert via the notification system
func (b *BudgetTracker) sendAlert(alertType string, current, limit float64) {
	// Try to send via notification system first
	if b.notifier != nil {
		// Determine session ID (use "global" for budget-wide alerts)
		sessionID := "global"
		if alertType == "session_limit" || alertType == "session_warning" {
			// For session-specific alerts, we'd need to track the current session
			// For now, use "session" as a placeholder
			sessionID = "session"
		}

		err := b.notifier.SendBudgetAlert(alertType, sessionID, current, limit)
		if err == nil {
			return // Successfully sent via Matrix
		}
		// Fall through to logging if Matrix send fails
	}

	// Fallback to console logging
	log.Printf("[BUDGET ALERT] %s - Current: $%.2f, Limit: $%.2f\n", alertType, current, limit)
}

// SetNotifier sets the notification system for budget alerts
func (b *BudgetTracker) SetNotifier(notifier *notification.Notifier) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.notifier = notifier
}

// Reset clears all usage data
// Persists the reset to WAL for crash recovery
func (b *BudgetTracker) Reset() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	// Persist reset to WAL first
	if b.persistence != nil {
		if err := b.persistence.AppendReset(); err != nil {
			return fmt.Errorf("failed to persist reset: %w", err)
		}
	}

	b.usageHistory = make([]UsageRecord, 0)
	b.dailyUsage = make(map[string]float64)
	b.monthlyUsage = make(map[string]float64)
	b.sessionUsage = make(map[string]float64)

	return nil
}

// Close flushes pending writes and closes the tracker
// MUST be called for graceful shutdown to ensure all data is persisted
func (b *BudgetTracker) Close() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.persistence != nil {
		// Save final snapshot
		if err := b.persistence.SaveSnapshot(b); err != nil {
			return fmt.Errorf("failed to save final snapshot: %w", err)
		}
		// Close persistence layer
		if err := b.persistence.Close(); err != nil {
			return fmt.Errorf("failed to close persistence: %w", err)
		}
	}

	return nil
}

// SaveState persists budget state to disk (creates a snapshot)
// This creates a full snapshot and compacts the WAL
func (b *BudgetTracker) SaveState() error {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	if b.persistence == nil {
		return fmt.Errorf("persistence not enabled")
	}

	return b.persistence.SaveSnapshot(b)
}

// LoadState restores budget state from disk
// Note: State is automatically loaded on NewBudgetTracker when persistence is enabled
func (b *BudgetTracker) LoadState() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.persistence == nil {
		return fmt.Errorf("persistence not enabled")
	}

	return b.recoverFromPersistence()
}

// GetUsageHistory returns the usage history
func (b *BudgetTracker) GetUsageHistory() []UsageRecord {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	// Return a copy to prevent external modification
	history := make([]UsageRecord, len(b.usageHistory))
	copy(history, b.usageHistory)
	return history
}

// SetCost sets a custom cost for a model
func (b *BudgetTracker) SetCost(model string, costPer1M float64) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.costs[model] = costPer1M
}

// GetCost returns the cost per 1M tokens for a model
func (b *BudgetTracker) GetCost(model string) float64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	if cost, ok := b.costs[model]; ok {
		return cost
	}
	return 0.001 // Default fallback
}

// GetPersistenceMode returns the current persistence mode
func (b *BudgetTracker) GetPersistenceMode() PersistenceMode {
	return b.persistConfig.Mode
}

// IsPersistent returns true if persistence is enabled
func (b *BudgetTracker) IsPersistent() bool {
	return b.persistConfig.Mode != PersistenceDisabled
}
