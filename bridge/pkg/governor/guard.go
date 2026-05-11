// Package governor provides per-RPC-method rate limiting through the Guard type,
// PII interception through SkillGate, and Shadow Mapping 2.0.
package governor

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/armorclaw/bridge/pkg/audit"
)

// RPCMethodRateLimited is the error code returned when a call is rate-limited.
// Uses -32006 (same as KeystoreRateLimited in rpc package) per spec.
const RPCMethodRateLimited = -32006

// AuditLogger is a minimal interface for logging guard decisions.
type AuditLogger interface {
	LogEvent(eventType audit.EventType, sessionID, roomID, userID string, details interface{}) error
}

// MethodGroup defines a named group of RPC methods sharing a rate limit.
type MethodGroup struct {
	Name    string   // Group identifier (e.g., "default", "container", "ai", "health")
	RPS     float64  // Requests per second allowed for this group
	Methods []string // RPC method names in this group (prefix-matched)
}

// GuardConfig holds configuration for creating a new Guard.
type GuardConfig struct {
	// Groups defines method groups and their RPS limits.
	// At minimum, "default" group should be defined as fallback.
	Groups []MethodGroup

	// DefaultRPS is the rate limit for any method not in a defined group.
	DefaultRPS float64

	// HealthExempt marks the "health" group as exempt from rate limiting.
	// Health check endpoints should never be rate-limited.
	HealthExempt bool

	// AuditLog is an optional audit logger. When nil, decisions are logged via stdlib log.
	AuditLog AuditLogger

	// Mode is the server mode ("native", "sentinel", "cloudflare") — used for logging context.
	Mode string
}

// tokenBucket implements a simple token bucket rate limiter.
type tokenBucket struct {
	rate       float64 // tokens per second
	maxTokens  float64 // bucket capacity
	tokens     float64 // current token count
	lastRefill time.Time
	mu         sync.Mutex
}

func newTokenBucket(rps float64) *tokenBucket {
	return &tokenBucket{
		rate:       rps,
		maxTokens:  rps, // burst = 1 second worth
		tokens:     rps, // start full
		lastRefill: time.Now(),
	}
}

// Allow returns true if a token is available.
func (tb *tokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastRefill = now

	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}
	return false
}

// Guard provides per-RPC-method rate limiting with mode-aware initialization.
// It maps RPC method names to method groups, each with independent token buckets.
type Guard struct {
	groups     map[string]*methodGroupEntry
	defaultRPS float64
	auditLog   AuditLogger
	mode       string
	mu         sync.RWMutex
}

type methodGroupEntry struct {
	name   string
	rps    float64
	exempt bool
	bucket *tokenBucket
}

// NewGuard creates a new Guard with the given configuration.
// Returns an error if the configuration is invalid (e.g., no groups, non-positive RPS).
func NewGuard(cfg GuardConfig) (*Guard, error) {
	if cfg.DefaultRPS <= 0 {
		return nil, fmt.Errorf("governor: DefaultRPS must be positive, got %f", cfg.DefaultRPS)
	}

	g := &Guard{
		groups:     make(map[string]*methodGroupEntry),
		defaultRPS: cfg.DefaultRPS,
		auditLog:   cfg.AuditLog,
		mode:       cfg.Mode,
	}

	// Process defined groups
	for _, mg := range cfg.Groups {
		if mg.RPS < 0 {
			return nil, fmt.Errorf("governor: group %q has negative RPS %f", mg.Name, mg.RPS)
		}

		rps := mg.RPS
		if rps == 0 {
			rps = cfg.DefaultRPS
		}

		isHealth := mg.Name == "health" && cfg.HealthExempt

		entry := &methodGroupEntry{
			name:   mg.Name,
			rps:    rps,
			exempt: isHealth,
			bucket: newTokenBucket(rps),
		}

		for _, method := range mg.Methods {
			g.groups[method] = entry
		}

		// Also register by group name for lookup
		g.groups[mg.Name] = entry
	}

	if len(g.groups) == 0 {
		return nil, fmt.Errorf("governor: at least one method group must be defined")
	}

	// Ensure default bucket exists
	if _, ok := g.groups["default"]; !ok {
		g.groups["default"] = &methodGroupEntry{
			name:   "default",
			rps:    cfg.DefaultRPS,
			exempt: false,
			bucket: newTokenBucket(cfg.DefaultRPS),
		}
	}

	log.Printf("[INFO] Guard initialized: %d groups, mode=%s, default_rps=%.1f",
		len(cfg.Groups), cfg.Mode, cfg.DefaultRPS)

	return g, nil
}

// CheckRateLimit checks if the given RPC method is allowed to proceed.
// Returns nil if allowed, or an error with code -32006 if rate-limited.
// Health methods are always allowed when HealthExempt is true.
func (g *Guard) CheckRateLimit(method string) error {
	g.mu.RLock()
	entry := g.resolveGroup(method)
	g.mu.RUnlock()

	if entry == nil {
		return nil // no rate limit configured, allow
	}

	if entry.exempt {
		return nil // health methods are always allowed
	}

	if !entry.bucket.Allow() {
		g.logDecision(method, entry.name, entry.rps, false)
		return fmt.Errorf("rate limit exceeded for method %s (limit: %.0f rps, group: %s)",
			method, entry.rps, entry.name)
	}

	g.logDecision(method, entry.name, entry.rps, true)
	return nil
}

// resolveGroup finds the method group for a given RPC method name.
// Looks for exact match first, then falls back to "default".
func (g *Guard) resolveGroup(method string) *methodGroupEntry {
	// Exact match
	if entry, ok := g.groups[method]; ok {
		return entry
	}

	// Prefix match: e.g., "browser.navigate" matches "browser" group
	// Check if any group name is a prefix of the method
	parts := splitMethod(method)
	for i := len(parts) - 1; i >= 1; i-- {
		prefix := parts[0]
		for j := 1; j < i; j++ {
			prefix += "." + parts[j]
		}
		if entry, ok := g.groups[prefix]; ok {
			return entry
		}
	}

	// Fall back to "default" group
	if entry, ok := g.groups["default"]; ok {
		return entry
	}

	return nil
}

// splitMethod splits "browser.navigate" into ["browser", "navigate"]
func splitMethod(method string) []string {
	parts := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(method); i++ {
		if method[i] == '.' {
			if i > start {
				parts = append(parts, method[start:i])
			}
			start = i + 1
		}
	}
	if start < len(method) {
		parts = append(parts, method[start:])
	}
	return parts
}

// logDecision logs rate limiting decisions to AuditLog when available,
// otherwise logs via stdlib log.
func (g *Guard) logDecision(method, groupName string, rps float64, allowed bool) {
	if g.auditLog != nil {
		status := "allowed"
		if !allowed {
			status = "rate_limited"
		}
		_ = g.auditLog.LogEvent("guard.rate_limit_check", "", "", "", map[string]interface{}{
			"method":     method,
			"group":      groupName,
			"rps_limit":  rps,
			"status":     status,
			"server_mode": g.mode,
		})
		return
	}

	if !allowed {
		log.Printf("[WARN] Guard: rate limited method=%s group=%s limit=%.0frps",
			method, groupName, rps)
	}
}

// DefaultMethodGroups returns the standard method groups used by ArmorClaw.
func DefaultMethodGroups() []MethodGroup {
	return []MethodGroup{
		{
			Name:    "health",
			RPS:     100, // high limit, exempt anyway
			Methods: []string{"health.check"},
		},
		{
			Name: "container",
			RPS:  10,
			Methods: []string{
				"container.terminate",
				"container.list",
			},
		},
		{
			Name: "ai",
			RPS:  5,
			Methods: []string{
				"ai.chat",
			},
		},
		{
			Name: "browser",
			RPS:  20,
			Methods: []string{
				"browser.navigate",
				"browser.fill",
				"browser.click",
				"browser.status",
				"browser.wait_for_element",
				"browser.wait_for_captcha",
				"browser.wait_for_2fa",
				"browser.complete",
				"browser.fail",
				"browser.list",
				"browser.cancel",
				"browser.replay_diagnostics",
			},
		},
		{
			Name: "skills",
			RPS:  10,
			Methods: []string{
				"skills.execute",
				"skills.list",
				"skills.get_schema",
				"skills.allow",
				"skills.block",
				"skills.allowlist_add",
				"skills.allowlist_remove",
				"skills.allowlist_list",
				"skills.web_search",
				"skills.web_extract",
				"skills.email_send",
				"skills.slack_message",
				"skills.file_read",
				"skills.data_analyze",
			},
		},
		{
			Name: "keystore",
			RPS:  5,
			Methods: []string{
				"keystore.unseal",
				"keystore.sealed",
				"keystore.seal",
				"keystore.extend_session",
				"keystore.session_status",
				"keystore.list_keys",
				"keystore.delete_key",
			},
		},
		{
			Name: "default",
			RPS:  30,
			Methods: []string{
				// Catch-all group — any method not in a specific group
				// falls through to this group at resolve time
			},
		},
	}
}
