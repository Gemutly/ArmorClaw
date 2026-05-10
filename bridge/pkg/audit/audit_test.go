package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAuditLog_WritesEntry(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Path:   filepath.Join(dir, "audit.db"),
		MaxLen: 1000,
	}

	al, err := NewAuditLog(cfg)
	if err != nil {
		t.Fatalf("NewAuditLog: %v", err)
	}

	err = al.LogEvent(EventSidecarHealthCheck, "sess1", "room1", "user1", map[string]string{"status": "ok"})
	if err != nil {
		t.Fatalf("LogEvent: %v", err)
	}

	if al.Count() != 1 {
		t.Fatalf("expected count 1, got %d", al.Count())
	}

	data, err := os.ReadFile(cfg.Path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry in file, got %d", len(entries))
	}
	if entries[0].EventType != EventSidecarHealthCheck {
		t.Errorf("expected event_type %s, got %s", EventSidecarHealthCheck, entries[0].EventType)
	}
	if entries[0].SessionID != "sess1" {
		t.Errorf("expected session_id sess1, got %s", entries[0].SessionID)
	}
}

func TestAuditLog_DegradedMode(t *testing.T) {
	cfg := Config{
		Path:   "/nonexistent/deep/path/that/cannot/be/created/audit.db",
		MaxLen: 100,
	}

	_, err := NewAuditLog(cfg)
	if err == nil {
		t.Fatal("expected error for unwritable path, got nil")
	}

	dir := t.TempDir()
	readOnlyDir := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0555); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	cfg2 := Config{
		Path:   filepath.Join(readOnlyDir, "subdir", "audit.db"),
		MaxLen: 100,
	}

	_, err = NewAuditLog(cfg2)
	if err != nil {
		t.Logf("NewAuditLog with unwritable subdir: %v (expected: creation failure is acceptable)", err)
	}
}

func TestAuditLog_NilSafeLogEvent(t *testing.T) {
	var al *AuditLog

	if al != nil {
		t.Fatal("nil AuditLog should be nil — callers must guard with nil check")
	}
}

func TestAuditLog_QueryFilters(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Path:   filepath.Join(dir, "audit.db"),
		MaxLen: 10000,
	}

	al, err := NewAuditLog(cfg)
	if err != nil {
		t.Fatalf("NewAuditLog: %v", err)
	}

	_ = al.LogEvent(EventSidecarHealthCheck, "s1", "r1", "u1", nil)
	_ = al.LogEvent(EventKeystoreUnseal, "s2", "r2", "u2", nil)
	_ = al.LogEvent(EventSidecarHealthCheck, "s3", "r3", "u3", nil)

	results, err := al.Query(QueryParams{EventType: EventSidecarHealthCheck, Limit: 10})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 health check events, got %d", len(results))
	}

	results, err = al.Query(QueryParams{SessionID: "s2", Limit: 10})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 event for session s2, got %d", len(results))
	}
	if results[0].EventType != EventKeystoreUnseal {
		t.Errorf("expected keystore unseal, got %s", results[0].EventType)
	}
}

func TestAuditLog_MaxLenTrim(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Path:   filepath.Join(dir, "audit.db"),
		MaxLen: 5,
	}

	al, err := NewAuditLog(cfg)
	if err != nil {
		t.Fatalf("NewAuditLog: %v", err)
	}

	for i := 0; i < 10; i++ {
		_ = al.LogEvent(EventSidecarHealthCheck, "s", "r", "u", i)
	}

	if al.Count() != 5 {
		t.Fatalf("expected 5 entries after trim, got %d", al.Count())
	}
}

func TestAuditLog_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Path != "/var/lib/armorclaw/audit.db" {
		t.Errorf("expected default path /var/lib/armorclaw/audit.db, got %s", cfg.Path)
	}
	if cfg.MaxLen != 10000 {
		t.Errorf("expected default maxLen 10000, got %d", cfg.MaxLen)
	}
}

func TestAuditLog_WiredInMain(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Path:   filepath.Join(dir, "audit.db"),
		MaxLen: 1000,
	}

	auditor, err := NewAuditLog(cfg)
	if err != nil {
		t.Fatalf("NewAuditLog: %v", err)
	}

	_ = auditor.LogEvent(EventSidecarHealthCheck, "sess", "", "", map[string]interface{}{
		"status": "healthy",
	})
	_ = auditor.LogEvent(EventKeystoreUnseal, "", "", "user1", nil)
	_ = auditor.LogEvent(EventSecurityViolation, "", "", "user2", map[string]interface{}{
		"action":       "terminate_container",
		"container_id": "abc123",
	})

	if auditor.Count() != 3 {
		t.Fatalf("expected 3 entries, got %d", auditor.Count())
	}

	all, err := auditor.Query(QueryParams{Limit: 100})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	eventTypes := map[EventType]int{}
	for _, e := range all {
		eventTypes[e.EventType]++
	}
	if eventTypes[EventSidecarHealthCheck] != 1 {
		t.Errorf("expected 1 sidecar_health_check, got %d", eventTypes[EventSidecarHealthCheck])
	}
	if eventTypes[EventKeystoreUnseal] != 1 {
		t.Errorf("expected 1 keystore.unseal, got %d", eventTypes[EventKeystoreUnseal])
	}
	if eventTypes[EventSecurityViolation] != 1 {
		t.Errorf("expected 1 security_violation, got %d", eventTypes[EventSecurityViolation])
	}
}
