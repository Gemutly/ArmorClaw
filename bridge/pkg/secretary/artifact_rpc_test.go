package secretary

import (
	"context"
	"encoding/json"
	"testing"
)

func newTestArtifactRPCHandler(t *testing.T) (*ArtifactRPCHandler, *ArtifactStore) {
	t.Helper()
	store, err := NewArtifactStore(ArtifactStoreConfig{})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	return NewArtifactRPCHandler(store), store
}

func TestArtifactRPC_Upload_Success(t *testing.T) {
	handler, _ := newTestArtifactRPCHandler(t)

	params, _ := json.Marshal(ArtifactUploadParams{
		Owner:      "@alice:m",
		WorkflowID: "wf-1",
		Metadata: ArtifactMetadata{
			MIMEType:  "application/pdf",
			Filename:  "doc.pdf",
			SizeBytes: 1024,
		},
	})

	resp := handler.Handle(&RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "secretary.artifact_upload",
		Params:  params,
		UserID:  "@alice:m",
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	result := resp.Result.(map[string]interface{})
	if result["id"] == "" {
		t.Fatal("expected non-empty artifact ID")
	}
	if result["status"] != ArtifactPending {
		t.Fatalf("expected pending status, got %v", result["status"])
	}
}

func TestArtifactRPC_Upload_RejectNoAuth(t *testing.T) {
	handler, _ := newTestArtifactRPCHandler(t)

	params, _ := json.Marshal(ArtifactUploadParams{
		Owner:      "@alice:m",
		WorkflowID: "wf-1",
	})

	resp := handler.Handle(&RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "secretary.artifact_upload",
		Params:  params,
		UserID:  "",
	})

	if resp.Error == nil {
		t.Fatal("expected auth error")
	}
}

func TestArtifactRPC_Upload_RejectNoWorkflow(t *testing.T) {
	handler, _ := newTestArtifactRPCHandler(t)

	params, _ := json.Marshal(ArtifactUploadParams{
		Owner: "@alice:m",
	})

	resp := handler.Handle(&RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "secretary.artifact_upload",
		Params:  params,
		UserID:  "@alice:m",
	})

	if resp.Error == nil {
		t.Fatal("expected validation error")
	}
}

func TestArtifactRPC_Download_Success(t *testing.T) {
	handler, store := newTestArtifactRPCHandler(t)
	ctx := context.Background()

	env, _ := NewArtifactEnvelope("@alice:m", "wf-1", ArtifactMetadata{Filename: "doc.pdf"})
	store.StoreEnvelope(ctx, env)

	params, _ := json.Marshal(ArtifactDownloadParams{ArtifactID: env.ID})

	resp := handler.Handle(&RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "secretary.artifact_download",
		Params:  params,
		UserID:  "@alice:m",
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestArtifactRPC_Download_RejectWrongOwner(t *testing.T) {
	handler, store := newTestArtifactRPCHandler(t)

	env, _ := NewArtifactEnvelope("@alice:m", "wf-1", ArtifactMetadata{})
	store.StoreEnvelope(context.Background(), env)

	params, _ := json.Marshal(ArtifactDownloadParams{ArtifactID: env.ID})

	resp := handler.Handle(&RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "secretary.artifact_download",
		Params:  params,
		UserID:  "@eve:m",
	})

	if resp.Error == nil {
		t.Fatal("expected authorization error")
	}
}

func TestArtifactRPC_Download_NotFound(t *testing.T) {
	handler, _ := newTestArtifactRPCHandler(t)

	params, _ := json.Marshal(ArtifactDownloadParams{ArtifactID: "nonexistent"})

	resp := handler.Handle(&RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "secretary.artifact_download",
		Params:  params,
		UserID:  "@alice:m",
	})

	if resp.Error == nil {
		t.Fatal("expected not found error")
	}
}

func TestArtifactRPC_List_Success(t *testing.T) {
	handler, store := newTestArtifactRPCHandler(t)

	env1, _ := NewArtifactEnvelope("@alice:m", "wf-1", ArtifactMetadata{})
	env2, _ := NewArtifactEnvelope("@bob:m", "wf-2", ArtifactMetadata{})
	store.StoreEnvelope(context.Background(), env1)
	store.StoreEnvelope(context.Background(), env2)

	params, _ := json.Marshal(ArtifactListParams{Owner: "@alice:m"})

	resp := handler.Handle(&RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "secretary.artifact_list",
		Params:  params,
		UserID:  "@alice:m",
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	result := resp.Result.(map[string]interface{})
	if result["count"].(int) != 1 {
		t.Fatalf("expected count 1, got %v", result["count"])
	}
}

func TestArtifactRPC_UpdateStatus_Success(t *testing.T) {
	handler, store := newTestArtifactRPCHandler(t)

	env, _ := NewArtifactEnvelope("@alice:m", "wf-1", ArtifactMetadata{})
	store.StoreEnvelope(context.Background(), env)

	params, _ := json.Marshal(ArtifactUpdateStatusParams{
		ArtifactID: env.ID,
		Status:     "processing",
	})

	resp := handler.Handle(&RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "secretary.artifact_update_status",
		Params:  params,
		UserID:  "@alice:m",
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestArtifactRPC_UpdateStatus_InvalidTransition(t *testing.T) {
	handler, store := newTestArtifactRPCHandler(t)

	env, _ := NewArtifactEnvelope("@alice:m", "wf-1", ArtifactMetadata{})
	store.StoreEnvelope(context.Background(), env)

	params, _ := json.Marshal(ArtifactUpdateStatusParams{
		ArtifactID: env.ID,
		Status:     "completed",
	})

	resp := handler.Handle(&RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "secretary.artifact_update_status",
		Params:  params,
		UserID:  "@alice:m",
	})

	if resp.Error == nil {
		t.Fatal("expected transition error")
	}
}

func TestArtifactRPC_UpdateStatus_RejectWrongOwner(t *testing.T) {
	handler, store := newTestArtifactRPCHandler(t)

	env, _ := NewArtifactEnvelope("@alice:m", "wf-1", ArtifactMetadata{})
	store.StoreEnvelope(context.Background(), env)

	params, _ := json.Marshal(ArtifactUpdateStatusParams{
		ArtifactID: env.ID,
		Status:     "processing",
	})

	resp := handler.Handle(&RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "secretary.artifact_update_status",
		Params:  params,
		UserID:  "@eve:m",
	})

	if resp.Error == nil {
		t.Fatal("expected authorization error")
	}
}

func TestArtifactRPC_UnknownMethod(t *testing.T) {
	handler, _ := newTestArtifactRPCHandler(t)

	resp := handler.Handle(&RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "secretary.artifact_nonexistent",
		Params:  json.RawMessage(`{}`),
		UserID:  "@alice:m",
	})

	if resp.Error == nil {
		t.Fatal("expected not found error")
	}
}
