package secretary

import (
	"context"
	"testing"
	"time"
)

func newTestArtifactStore(t *testing.T) *ArtifactStore {
	t.Helper()
	store, err := NewArtifactStore(ArtifactStoreConfig{
		TTL: 1 * time.Hour,
	})
	if err != nil {
		t.Fatalf("create artifact store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestArtifactStore_StoreAndGetEnvelope(t *testing.T) {
	store := newTestArtifactStore(t)
	ctx := context.Background()

	meta := ArtifactMetadata{
		MIMEType:  "application/pdf",
		Filename:  "report.pdf",
		SizeBytes: 2048,
	}
	env, err := NewArtifactEnvelope("@alice:matrix.org", "wf-001", meta)
	if err != nil {
		t.Fatalf("create envelope: %v", err)
	}

	if err := store.StoreEnvelope(ctx, env); err != nil {
		t.Fatalf("store envelope: %v", err)
	}

	got, err := store.GetEnvelope(ctx, env.ID)
	if err != nil {
		t.Fatalf("get envelope: %v", err)
	}
	if got == nil {
		t.Fatal("expected envelope, got nil")
	}
	if got.ID != env.ID {
		t.Fatalf("expected ID %q, got %q", env.ID, got.ID)
	}
	if got.Owner != "@alice:matrix.org" {
		t.Fatalf("expected owner @alice:matrix.org, got %q", got.Owner)
	}
	if got.WorkflowID != "wf-001" {
		t.Fatalf("expected workflow wf-001, got %q", got.WorkflowID)
	}
	if got.Metadata.Filename != "report.pdf" {
		t.Fatalf("expected filename report.pdf, got %q", got.Metadata.Filename)
	}
}

func TestArtifactStore_GetEnvelope_NotFound(t *testing.T) {
	store := newTestArtifactStore(t)
	ctx := context.Background()

	got, err := store.GetEnvelope(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for nonexistent envelope")
	}
}

func TestArtifactStore_UpdateStatus(t *testing.T) {
	store := newTestArtifactStore(t)
	ctx := context.Background()

	env, err := NewArtifactEnvelope("@u:m", "wf-1", ArtifactMetadata{})
	if err != nil {
		t.Fatalf("create envelope: %v", err)
	}
	if err := store.StoreEnvelope(ctx, env); err != nil {
		t.Fatalf("store envelope: %v", err)
	}

	if err := store.UpdateStatus(ctx, env.ID, ArtifactProcessing); err != nil {
		t.Fatalf("update status: %v", err)
	}

	got, err := store.GetEnvelope(ctx, env.ID)
	if err != nil {
		t.Fatalf("get envelope: %v", err)
	}
	if got == nil {
		t.Fatal("expected envelope, got nil")
	}
	if got.Status != ArtifactProcessing {
		t.Fatalf("expected processing, got %q", got.Status)
	}
}

func TestArtifactStore_UpdateStatus_NotFound(t *testing.T) {
	store := newTestArtifactStore(t)
	ctx := context.Background()

	err := store.UpdateStatus(ctx, "nonexistent", ArtifactProcessing)
	if err == nil {
		t.Fatal("expected error for nonexistent envelope")
	}
}

func TestArtifactStore_ListEnvelopes_ByOwner(t *testing.T) {
	store := newTestArtifactStore(t)
	ctx := context.Background()

	env1, err := NewArtifactEnvelope("@alice:m", "wf-1", ArtifactMetadata{Filename: "a.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	env2, err := NewArtifactEnvelope("@bob:m", "wf-2", ArtifactMetadata{Filename: "b.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	env3, err := NewArtifactEnvelope("@alice:m", "wf-3", ArtifactMetadata{Filename: "c.pdf"})
	if err != nil {
		t.Fatal(err)
	}

	store.StoreEnvelope(ctx, env1)
	store.StoreEnvelope(ctx, env2)
	store.StoreEnvelope(ctx, env3)

	results, err := store.ListEnvelopes(ctx, ArtifactFilter{Owner: "@alice:m"})
	if err != nil {
		t.Fatalf("list by owner: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for @alice:m, got %d", len(results))
	}
}

func TestArtifactStore_ListEnvelopes_ByWorkflowID(t *testing.T) {
	store := newTestArtifactStore(t)
	ctx := context.Background()

	env1, err := NewArtifactEnvelope("@u:m", "wf-target", ArtifactMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	env2, err := NewArtifactEnvelope("@u:m", "wf-other", ArtifactMetadata{})
	if err != nil {
		t.Fatal(err)
	}

	store.StoreEnvelope(ctx, env1)
	store.StoreEnvelope(ctx, env2)

	results, err := store.ListEnvelopes(ctx, ArtifactFilter{WorkflowID: "wf-target"})
	if err != nil {
		t.Fatalf("list by workflow: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for wf-target, got %d", len(results))
	}
}

func TestArtifactStore_ListEnvelopes_ByStatus(t *testing.T) {
	store := newTestArtifactStore(t)
	ctx := context.Background()

	env1, err := NewArtifactEnvelope("@u:m", "wf-1", ArtifactMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	env2, err := NewArtifactEnvelope("@u:m", "wf-2", ArtifactMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	env2.Status = ArtifactCompleted

	store.StoreEnvelope(ctx, env1)
	store.StoreEnvelope(ctx, env2)

	pending := ArtifactPending
	results, err := store.ListEnvelopes(ctx, ArtifactFilter{Status: &pending})
	if err != nil {
		t.Fatalf("list by status: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 pending result, got %d", len(results))
	}
}

func TestArtifactStore_CleanupExpired(t *testing.T) {
	store, err := NewArtifactStore(ArtifactStoreConfig{
		TTL: 1 * time.Nanosecond,
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	env, err := NewArtifactEnvelope("@u:m", "wf-1", ArtifactMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	store.StoreEnvelope(ctx, env)

	time.Sleep(10 * time.Millisecond)

	removed, err := store.CleanupExpired(ctx)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 expired removed, got %d", removed)
	}

	got, _ := store.GetEnvelope(ctx, env.ID)
	if got != nil {
		t.Fatal("expected envelope to be cleaned up")
	}
}

func TestArtifactStore_CleanupExpired_NoneExpired(t *testing.T) {
	store := newTestArtifactStore(t)
	ctx := context.Background()

	env, err := NewArtifactEnvelope("@u:m", "wf-1", ArtifactMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	store.StoreEnvelope(ctx, env)

	removed, err := store.CleanupExpired(ctx)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed, got %d", removed)
	}
}

func TestArtifactStore_ListEnvelopes_Empty(t *testing.T) {
	store := newTestArtifactStore(t)
	ctx := context.Background()

	results, err := store.ListEnvelopes(ctx, ArtifactFilter{})
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}
