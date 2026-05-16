package email

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func tempOutboxDB(t *testing.T) (*OutboxStore, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test_outbox.db")
	key := "test_encryption_key_32bytes_long_ok!"

	err := os.WriteFile(dbPath, nil, 0600)
	if err != nil {
		t.Fatalf("create db file: %v", err)
	}

	store, err := NewOutboxStore(dbPath + "?_pragma_key=" + key + "&_pragma_cipher_page_size=4096")
	if err != nil {
		t.Fatalf("NewOutboxStore: %v", err)
	}

	return store, func() {
		store.Close()
	}
}

func TestOutboxEnqueueAndGetByID(t *testing.T) {
	store, cleanup := tempOutboxDB(t)
	defer cleanup()
	ctx := context.Background()

	entry := &OutboxEntry{
		ID:            "email-001",
		WorkflowID:    "wf-001",
		RecipientHash: "sha256:abc123",
		SubjectHash:   "sha256:def456",
	}

	if err := store.Enqueue(ctx, entry); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	got, err := store.GetByID(ctx, "email-001")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil")
	}
	if got.ID != "email-001" {
		t.Errorf("ID = %q, want %q", got.ID, "email-001")
	}
	if got.Status != StatusQueued {
		t.Errorf("Status = %q, want %q", got.Status, StatusQueued)
	}
	if got.WorkflowID != "wf-001" {
		t.Errorf("WorkflowID = %q, want %q", got.WorkflowID, "wf-001")
	}
	if got.RecipientHash != "sha256:abc123" {
		t.Errorf("RecipientHash = %q, want %q", got.RecipientHash, "sha256:abc123")
	}
	if got.CreatedAt == 0 {
		t.Error("CreatedAt should be set")
	}
}

func TestOutboxGetByIDNotFound(t *testing.T) {
	store, cleanup := tempOutboxDB(t)
	defer cleanup()
	ctx := context.Background()

	got, err := store.GetByID(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestOutboxUpdateStatus(t *testing.T) {
	store, cleanup := tempOutboxDB(t)
	defer cleanup()
	ctx := context.Background()

	entry := &OutboxEntry{
		ID:            "email-002",
		WorkflowID:    "wf-002",
		RecipientHash: "sha256:test",
	}
	store.Enqueue(ctx, entry)

	err := store.UpdateStatus(ctx, "email-002", StatusAwaitingApproval)
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	got, _ := store.GetByID(ctx, "email-002")
	if got.Status != StatusAwaitingApproval {
		t.Errorf("Status = %q, want %q", got.Status, StatusAwaitingApproval)
	}
}

func TestOutboxInvalidTransition(t *testing.T) {
	store, cleanup := tempOutboxDB(t)
	defer cleanup()
	ctx := context.Background()

	entry := &OutboxEntry{
		ID:            "email-003",
		WorkflowID:    "wf-003",
		RecipientHash: "sha256:test",
	}
	store.Enqueue(ctx, entry)

	err := store.UpdateStatus(ctx, "email-003", StatusSent)
	if err == nil {
		t.Fatal("expected error for invalid transition queued -> sent")
	}
}

func TestOutboxIncrementAttempt(t *testing.T) {
	store, cleanup := tempOutboxDB(t)
	defer cleanup()
	ctx := context.Background()

	entry := &OutboxEntry{
		ID:            "email-004",
		WorkflowID:    "wf-004",
		RecipientHash: "sha256:test",
	}
	store.Enqueue(ctx, entry)

	store.IncrementAttempt(ctx, "email-004", "SMTP_TIMEOUT")
	store.IncrementAttempt(ctx, "email-004", "CONNECTION_REFUSED")

	got, _ := store.GetByID(ctx, "email-004")
	if got.AttemptCount != 2 {
		t.Errorf("AttemptCount = %d, want 2", got.AttemptCount)
	}
	if got.LastErrorCode != "CONNECTION_REFUSED" {
		t.Errorf("LastErrorCode = %q, want %q", got.LastErrorCode, "CONNECTION_REFUSED")
	}
}

func TestOutboxMarkDeadLetter(t *testing.T) {
	store, cleanup := tempOutboxDB(t)
	defer cleanup()
	ctx := context.Background()

	entry := &OutboxEntry{
		ID:            "email-005",
		WorkflowID:    "wf-005",
		RecipientHash: "sha256:test",
	}
	store.Enqueue(ctx, entry)
	store.UpdateStatus(ctx, "email-005", StatusAwaitingApproval)
	store.UpdateStatus(ctx, "email-005", StatusFailed)

	err := store.MarkDeadLetter(ctx, "email-005")
	if err != nil {
		t.Fatalf("MarkDeadLetter: %v", err)
	}

	got, _ := store.GetByID(ctx, "email-005")
	if got.Status != StatusDeadLetter {
		t.Errorf("Status = %q, want %q", got.Status, StatusDeadLetter)
	}
}

func TestOutboxListByStatus(t *testing.T) {
	store, cleanup := tempOutboxDB(t)
	defer cleanup()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		entry := &OutboxEntry{
			ID:            fmt.Sprintf("email-%03d", i),
			WorkflowID:    "wf-list",
			RecipientHash: fmt.Sprintf("sha256:recip%d", i),
		}
		store.Enqueue(ctx, entry)
	}

	entries, err := store.ListByStatus(ctx, StatusQueued, 3, 0)
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("got %d entries, want 3", len(entries))
	}

	allEntries, err := store.ListByStatus(ctx, "", 10, 0)
	if err != nil {
		t.Fatalf("ListByStatus (all): %v", err)
	}
	if len(allEntries) != 5 {
		t.Errorf("got %d entries, want 5", len(allEntries))
	}
}

func TestOutboxGetQueueStats(t *testing.T) {
	store, cleanup := tempOutboxDB(t)
	defer cleanup()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		entry := &OutboxEntry{
			ID:            fmt.Sprintf("email-stat-%d", i),
			WorkflowID:    "wf-stats",
			RecipientHash: "sha256:test",
		}
		store.Enqueue(ctx, entry)
	}

	store.UpdateStatus(ctx, "email-stat-0", StatusAwaitingApproval)

	stats, err := store.GetQueueStats(ctx)
	if err != nil {
		t.Fatalf("GetQueueStats: %v", err)
	}

	byStatus, ok := stats["by_status"].(map[string]int)
	if !ok {
		t.Fatalf("by_status type = %T, want map[string]int", stats["by_status"])
	}

	if byStatus[StatusQueued] != 2 {
		t.Errorf("queued count = %d, want 2", byStatus[StatusQueued])
	}
	if byStatus[StatusAwaitingApproval] != 1 {
		t.Errorf("awaiting_approval count = %d, want 1", byStatus[StatusAwaitingApproval])
	}
	if stats["total"].(int) != 3 {
		t.Errorf("total = %d, want 3", stats["total"])
	}
}

func TestOutboxRetry(t *testing.T) {
	store, cleanup := tempOutboxDB(t)
	defer cleanup()
	ctx := context.Background()

	entry := &OutboxEntry{
		ID:            "email-retry",
		WorkflowID:    "wf-retry",
		RecipientHash: "sha256:test",
	}
	store.Enqueue(ctx, entry)
	store.UpdateStatus(ctx, "email-retry", StatusAwaitingApproval)
	store.UpdateStatus(ctx, "email-retry", StatusFailed)

	err := store.Retry(ctx, "email-retry")
	if err != nil {
		t.Fatalf("Retry: %v", err)
	}

	got, _ := store.GetByID(ctx, "email-retry")
	if got.Status != StatusQueued {
		t.Errorf("Status after retry = %q, want %q", got.Status, StatusQueued)
	}
	if got.AttemptCount != 0 {
		t.Errorf("AttemptCount after retry = %d, want 0", got.AttemptCount)
	}
}

func TestOutboxRetryWrongStatus(t *testing.T) {
	store, cleanup := tempOutboxDB(t)
	defer cleanup()
	ctx := context.Background()

	entry := &OutboxEntry{
		ID:            "email-retry-wrong",
		WorkflowID:    "wf-retry",
		RecipientHash: "sha256:test",
	}
	store.Enqueue(ctx, entry)

	err := store.Retry(ctx, "email-retry-wrong")
	if err == nil {
		t.Fatal("expected error when retrying queued email")
	}
}

func TestOutboxStatusTransitionChain(t *testing.T) {
	store, cleanup := tempOutboxDB(t)
	defer cleanup()
	ctx := context.Background()

	entry := &OutboxEntry{
		ID:            "email-chain",
		WorkflowID:    "wf-chain",
		RecipientHash: "sha256:test",
	}
	store.Enqueue(ctx, entry)

	transitions := []string{
		StatusAwaitingApproval,
		StatusApproved,
		StatusSending,
		StatusSent,
	}

	for _, status := range transitions {
		if err := store.UpdateStatus(ctx, "email-chain", status); err != nil {
			t.Fatalf("transition to %s: %v", status, err)
		}
	}

	got, _ := store.GetByID(ctx, "email-chain")
	if got.Status != StatusSent {
		t.Errorf("final status = %q, want %q", got.Status, StatusSent)
	}
}
