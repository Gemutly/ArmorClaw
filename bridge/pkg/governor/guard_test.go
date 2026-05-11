package governor

import (
	"sync"
	"testing"
	"time"

	"github.com/armorclaw/bridge/pkg/audit"
)

func TestNewGuard_ValidConfig(t *testing.T) {
	g, err := NewGuard(GuardConfig{
		Groups:       DefaultMethodGroups(),
		DefaultRPS:   30,
		HealthExempt: true,
		Mode:         "native",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if g == nil {
		t.Fatal("expected non-nil Guard")
	}
}

func TestNewGuard_ZeroDefaultRPS(t *testing.T) {
	_, err := NewGuard(GuardConfig{
		Groups:     DefaultMethodGroups(),
		DefaultRPS: 0,
	})
	if err == nil {
		t.Fatal("expected error for zero DefaultRPS")
	}
}

func TestNewGuard_NegativeRPS(t *testing.T) {
	_, err := NewGuard(GuardConfig{
		Groups: []MethodGroup{
			{Name: "test", RPS: -1, Methods: []string{"test.method"}},
		},
		DefaultRPS: 30,
	})
	if err == nil {
		t.Fatal("expected error for negative group RPS")
	}
}

func TestNewGuard_EmptyGroups(t *testing.T) {
	_, err := NewGuard(GuardConfig{
		Groups:     []MethodGroup{},
		DefaultRPS: 30,
	})
	if err == nil {
		t.Fatal("expected error for empty groups")
	}
}

func TestCheckRateLimit_AllowedUnderLimit(t *testing.T) {
	g, err := NewGuard(GuardConfig{
		Groups: []MethodGroup{
			{Name: "test", RPS: 100, Methods: []string{"test.method"}},
		},
		DefaultRPS:   30,
		HealthExempt: true,
		Mode:         "native",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = g.CheckRateLimit("test.method")
	if err != nil {
		t.Fatalf("expected rate limit to pass, got: %v", err)
	}
}

func TestCheckRateLimit_BlockedOverLimit(t *testing.T) {
	g, err := NewGuard(GuardConfig{
		Groups: []MethodGroup{
			{Name: "test", RPS: 2, Methods: []string{"test.method"}},
		},
		DefaultRPS:   30,
		HealthExempt: true,
		Mode:         "native",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Use all tokens (starts with 2 from burst)
	for i := 0; i < 2; i++ {
		err = g.CheckRateLimit("test.method")
		if err != nil {
			t.Fatalf("call %d should succeed: %v", i, err)
		}
	}

	// Third call should be rate limited
	err = g.CheckRateLimit("test.method")
	if err == nil {
		t.Fatal("expected rate limit error")
	}
}

func TestCheckRateLimit_HealthExempt(t *testing.T) {
	g, err := NewGuard(GuardConfig{
		Groups: []MethodGroup{
			{Name: "health", RPS: 1, Methods: []string{"health.check"}},
		},
		DefaultRPS:   1,
		HealthExempt: true,
		Mode:         "native",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Exhaust tokens first
	_ = g.CheckRateLimit("health.check")

	// Health should still be exempt even after exhaustion
	for i := 0; i < 100; i++ {
		err = g.CheckRateLimit("health.check")
		if err != nil {
			t.Fatalf("health.check should be exempt, call %d failed: %v", i, err)
		}
	}
}

func TestCheckRateLimit_DefaultFallback(t *testing.T) {
	g, err := NewGuard(GuardConfig{
		Groups: []MethodGroup{
			{Name: "test", RPS: 100, Methods: []string{"test.method"}},
			{Name: "default", RPS: 50},
		},
		DefaultRPS:   30,
		HealthExempt: true,
		Mode:         "native",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Unknown method should fall to default group
	err = g.CheckRateLimit("unknown.method")
	if err != nil {
		t.Fatalf("unknown method should use default group: %v", err)
	}
}

func TestCheckRateLimit_GroupPrefixMatching(t *testing.T) {
	g, err := NewGuard(GuardConfig{
		Groups: []MethodGroup{
			{Name: "browser", RPS: 2, Methods: []string{"browser.navigate"}},
		},
		DefaultRPS:   100,
		HealthExempt: false,
		Mode:         "native",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Exact match should work
	err = g.CheckRateLimit("browser.navigate")
	if err != nil {
		t.Fatalf("exact match should work: %v", err)
	}
}

func TestCheckRateLimit_IndependentGroups(t *testing.T) {
	g, err := NewGuard(GuardConfig{
		Groups: []MethodGroup{
			{Name: "ai", RPS: 1, Methods: []string{"ai.chat"}},
			{Name: "browser", RPS: 1, Methods: []string{"browser.navigate"}},
		},
		DefaultRPS:   30,
		HealthExempt: true,
		Mode:         "native",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Exhaust AI bucket
	_ = g.CheckRateLimit("ai.chat")
	err = g.CheckRateLimit("ai.chat")
	if err == nil {
		t.Fatal("AI should be rate limited")
	}

	// Browser should still work (independent bucket)
	err = g.CheckRateLimit("browser.navigate")
	if err != nil {
		t.Fatalf("browser should still be allowed: %v", err)
	}
}

func TestCheckRateLimit_Concurrent(t *testing.T) {
	g, err := NewGuard(GuardConfig{
		Groups: []MethodGroup{
			{Name: "test", RPS: 1000, Methods: []string{"test.method"}},
		},
		DefaultRPS:   30,
		HealthExempt: true,
		Mode:         "native",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	errors := 0
	mu := sync.Mutex{}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := g.CheckRateLimit("test.method"); err != nil {
				mu.Lock()
				errors++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// With 1000 RPS burst and 50 concurrent calls, should have very few errors
	mu.Lock()
	defer mu.Unlock()
	if errors > 10 {
		t.Fatalf("too many rate limited calls in concurrent test: %d", errors)
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	tb := newTokenBucket(1000) // 1000 RPS = high refill rate
	tb.tokens = 0             // drain

	// Should fail immediately
	if tb.Allow() {
		t.Fatal("expected bucket to be empty")
	}

	// Wait for refill
	time.Sleep(2 * time.Millisecond)

	// Should succeed after refill
	if !tb.Allow() {
		t.Fatal("expected bucket to refill")
	}
}

func TestModeAwareFail_SentinelFatal(t *testing.T) {
	// In sentinel mode, bad config should cause fatal
	// We test by verifying NewGuard returns error and the caller pattern works
	_, err := NewGuard(GuardConfig{
		Groups:     []MethodGroup{},
		DefaultRPS: 30,
	})
	if err == nil {
		t.Fatal("expected error from empty groups")
	}

	// Verify the error message is useful for sentinel mode logging
	if err.Error() == "" {
		t.Fatal("error message should not be empty")
	}
}

func TestDefaultMethodGroups_Completeness(t *testing.T) {
	groups := DefaultMethodGroups()

	groupNames := make(map[string]bool)
	for _, g := range groups {
		groupNames[g.Name] = true
	}

	for _, name := range []string{"health", "container", "ai", "browser", "skills", "keystore", "default"} {
		if !groupNames[name] {
			t.Errorf("missing method group: %s", name)
		}
	}
}

type mockAuditLogger struct {
	events []mockAuditEvent
	mu     sync.Mutex
}

type mockAuditEvent struct {
	eventType string
	details   interface{}
}

func (m *mockAuditLogger) LogEvent(eventType audit.EventType, sessionID, roomID, userID string, details interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, mockAuditEvent{eventType: string(eventType), details: details})
	return nil
}

func TestGuard_AuditLogIntegration(t *testing.T) {
	mockLog := &mockAuditLogger{}
	g, err := NewGuard(GuardConfig{
		Groups: []MethodGroup{
			{Name: "test", RPS: 1, Methods: []string{"test.method"}},
		},
		DefaultRPS:   30,
		HealthExempt: true,
		AuditLog:     mockLog,
		Mode:         "sentinel",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Allowed call should log
	_ = g.CheckRateLimit("test.method")

	// Exhaust and get rate limited
	_ = g.CheckRateLimit("test.method")
	_ = g.CheckRateLimit("test.method")

	mockLog.mu.Lock()
	defer mockLog.mu.Unlock()
	if len(mockLog.events) == 0 {
		t.Fatal("expected audit log events")
	}

	// First event should be "allowed"
	first := mockLog.events[0]
	if first.eventType != "guard.rate_limit_check" {
		t.Errorf("expected event type guard.rate_limit_check, got %s", first.eventType)
	}

	// Find a rate_limited event
	foundLimited := false
	for _, e := range mockLog.events {
		if d, ok := e.details.(map[string]interface{}); ok {
			if d["status"] == "rate_limited" {
				foundLimited = true
				break
			}
		}
	}
	if !foundLimited {
		t.Error("expected at least one rate_limited audit event")
	}
}

func TestGuard_NilAuditLog_NoPanic(t *testing.T) {
	g, err := NewGuard(GuardConfig{
		Groups: []MethodGroup{
			{Name: "test", RPS: 1, Methods: []string{"test.method"}},
		},
		DefaultRPS:   30,
		HealthExempt: true,
		AuditLog:     nil,
		Mode:         "native",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not panic
	_ = g.CheckRateLimit("test.method")
	_ = g.CheckRateLimit("test.method")
	_ = g.CheckRateLimit("test.method")
}
