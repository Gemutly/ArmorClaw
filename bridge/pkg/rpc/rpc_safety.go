// Package rpc provides safety middleware for BEATO RPC methods.
// These helpers add authentication, timeout, rate-limiting, audit logging,
// and fail-closed behavior to browser.*, document.*, and email.queue_* methods.
// They are applied ONLY to new BEATO RPCs — existing methods are untouched.
package rpc

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Error codes for RPC safety (custom application codes in the -32000 range)
// ---------------------------------------------------------------------------

const (
	RPCAuthRequired    = -32010
	RPCAuthForbidden   = -32011
	RPCTimeoutExceeded = -32012
	RPCRateExceeded    = -32013
	RPCBodyTooLarge    = -32014
	RPCFailClosed      = -32015
)

// ---------------------------------------------------------------------------
// RPCGroup configures per-group safety parameters.
// ---------------------------------------------------------------------------

// RPCGroup defines the safety profile for a group of related RPC methods.
type RPCGroup struct {
	Name       string        // e.g. "browser", "document", "email"
	Timeout    time.Duration // per-request context deadline
	RateLimit  int           // max requests per minute per key
	MaxBodyLen int           // max request params length in bytes (0 = unlimited)
}

// Default BEATO RPC group profiles.
var (
	BrowserRPCGroup = RPCGroup{
		Name:       "browser",
		Timeout:    30 * time.Second,
		RateLimit:  20,      // 20/min
		MaxBodyLen: 1 << 20, // 1 MB
	}
	JetskiRPCGroup = RPCGroup{
		Name:       "jetski",
		Timeout:    10 * time.Second,
		RateLimit:  30, // 30/min
		MaxBodyLen: 1 << 20,
	}
	DocumentRPCGroup = RPCGroup{
		Name:       "document",
		Timeout:    60 * time.Second,
		RateLimit:  10, // 10/min
		MaxBodyLen: 1 << 20,
	}
	EmailRPCGroup = RPCGroup{
		Name:       "email",
		Timeout:    30 * time.Second,
		RateLimit:  20, // 20/min
		MaxBodyLen: 1 << 20,
	}
)

// ---------------------------------------------------------------------------
// TokenBucket — per-key rate limiter
// ---------------------------------------------------------------------------

// tokenBucket implements a sliding-window rate limiter.
type tokenBucket struct {
	count   int
	resetAt time.Time
}

// RateLimiter provides per-key in-memory rate limiting.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	limit   int // requests per minute
	window  time.Duration
}

// NewRateLimiter creates a new in-memory rate limiter.
func NewRateLimiter(limitPerMin int) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*tokenBucket),
		limit:   limitPerMin,
		window:  time.Minute,
	}
}

// Allow checks if a request from the given key is within rate limits.
// Returns true if allowed, false if rate limit exceeded.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok || now.After(b.resetAt) {
		rl.buckets[key] = &tokenBucket{
			count:   1,
			resetAt: now.Add(rl.window),
		}
		return true
	}

	if b.count >= rl.limit {
		return false
	}

	b.count++
	return true
}

// Reset clears all rate limit state (useful for testing).
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.buckets = make(map[string]*tokenBucket)
}

// ---------------------------------------------------------------------------
// SafetyMiddleware wraps handlers with safety checks.
// ---------------------------------------------------------------------------

// SafetyConfig holds the configuration for the safety middleware.
type SafetyConfig struct {
	Group       RPCGroup
	AdminToken  string                  // if empty, auth checks are skipped
	Limiters    map[string]*RateLimiter // keyed by group name
	AuditLogger *slog.Logger            // if nil, uses default slog
}

// SafetyMiddleware provides the Wrap function to add safety layers to a handler.
type SafetyMiddleware struct {
	cfg SafetyConfig
}

// NewSafetyMiddleware creates a new middleware with the given config.
func NewSafetyMiddleware(cfg SafetyConfig) *SafetyMiddleware {
	if cfg.AuditLogger == nil {
		cfg.AuditLogger = slog.Default()
	}
	return &SafetyMiddleware{cfg: cfg}
}

// Wrap wraps a HandlerFunc with all safety checks for the configured RPC group.
// The wrapped handler enforces: body size → auth → rate limit → timeout → fail-closed → handler.
func (m *SafetyMiddleware) Wrap(handler HandlerFunc) HandlerFunc {
	return func(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
		// 1. Body size check
		if m.cfg.Group.MaxBodyLen > 0 && len(req.Params) > m.cfg.Group.MaxBodyLen {
			m.audit("deny", req, "body_too_large", "")
			return nil, &ErrorObj{
				Code:    RPCBodyTooLarge,
				Message: "request body exceeds maximum allowed size",
			}
		}

		// 2. Authentication check
		authKey := m.extractAuthKey(req)
		if m.cfg.AdminToken != "" {
			if authKey == "" {
				m.audit("deny", req, "auth_missing", "")
				return nil, &ErrorObj{
					Code:    RPCAuthRequired,
					Message: "authentication required: missing token",
				}
			}
			if authKey != m.cfg.AdminToken {
				m.audit("deny", req, "auth_invalid", m.sanitizeKey(authKey))
				return nil, &ErrorObj{
					Code:    RPCAuthForbidden,
					Message: "authentication failed: invalid token",
				}
			}
		}

		// 3. Rate limiting
		if limiter, ok := m.cfg.Limiters[m.cfg.Group.Name]; ok {
			if !limiter.Allow(authKey) {
				m.audit("deny", req, "rate_limited", "")
				return nil, &ErrorObj{
					Code:    RPCRateExceeded,
					Message: "rate limit exceeded for " + m.cfg.Group.Name + " RPC group",
				}
			}
		}

		// 4. Timeout
		if m.cfg.Group.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, m.cfg.Group.Timeout)
			defer cancel()
		}

		// 5. Fail-closed: check if context already cancelled (dependency unavailable)
		if err := ctx.Err(); err != nil {
			m.audit("error", req, "fail_closed", err.Error())
			return nil, &ErrorObj{
				Code:    RPCFailClosed,
				Message: "service unavailable: " + err.Error(),
			}
		}

		// 6. Execute handler
		result, rpcErr := handler(ctx, req)
		if rpcErr != nil {
			m.audit("error", req, "handler_error", rpcErr.Message)
			return result, rpcErr
		}

		m.audit("allow", req, "success", "")
		return result, nil
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// extractAuthKey extracts the authentication token from the request.
// It checks the "token" field in the params JSON.
func (m *SafetyMiddleware) extractAuthKey(req *Request) string {
	if req.Params == nil {
		return ""
	}
	var p struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return ""
	}
	return p.Token
}

// sanitizeKey returns a truncated/masked version for logging.
// Prevents logging full auth tokens in audit trails.
func (m *SafetyMiddleware) sanitizeKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// audit logs a structured audit event. No raw PII is logged.
func (m *SafetyMiddleware) audit(action string, req *Request, reason string, detail string) {
	attrs := []any{
		"action", action,
		"method", req.Method,
		"reason", reason,
	}
	if detail != "" {
		attrs = append(attrs, "detail", detail)
	}
	m.cfg.AuditLogger.Info("rpc_safety_audit", attrs...)
}

// ---------------------------------------------------------------------------
// NewBEATOSafetyMiddleware creates a pre-configured SafetyMiddleware for all
// BEATO RPC groups with in-memory rate limiters.
// ---------------------------------------------------------------------------

// NewBEATOSafetyMiddleware creates safety middleware for all BEATO RPC groups.
// adminToken may be empty to skip auth checks (e.g. in tests).
func NewBEATOSafetyMiddleware(adminToken string) *SafetyMiddleware {
	limiters := map[string]*RateLimiter{
		BrowserRPCGroup.Name:  NewRateLimiter(BrowserRPCGroup.RateLimit),
		JetskiRPCGroup.Name:   NewRateLimiter(JetskiRPCGroup.RateLimit),
		DocumentRPCGroup.Name: NewRateLimiter(DocumentRPCGroup.RateLimit),
		EmailRPCGroup.Name:    NewRateLimiter(EmailRPCGroup.RateLimit),
	}

	return &SafetyMiddleware{
		cfg: SafetyConfig{
			AdminToken:  adminToken,
			Limiters:    limiters,
			AuditLogger: slog.Default(),
		},
	}
}

// WrapForGroup wraps a handler with safety checks for a specific RPC group.
// This is a convenience function that creates per-group middleware on the fly.
func (m *SafetyMiddleware) WrapForGroup(group RPCGroup, handler HandlerFunc) HandlerFunc {
	groupMiddleware := &SafetyMiddleware{
		cfg: SafetyConfig{
			Group:       group,
			AdminToken:  m.cfg.AdminToken,
			Limiters:    m.cfg.Limiters,
			AuditLogger: m.cfg.AuditLogger,
		},
	}
	return groupMiddleware.Wrap(handler)
}
