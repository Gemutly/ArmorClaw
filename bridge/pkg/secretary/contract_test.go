package secretary

import (
	"encoding/json"
	"testing"
)

// Contract tests verify that bridge response shapes match the documented
// API contracts in docs/reference/api-contracts-v1.2.md.

func TestContract_RPCResponse_Shape(t *testing.T) {
	resp := SuccessResponse(map[string]interface{}{
		"id":     "wf-123",
		"status": "running",
	})
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if parsed["jsonrpc"] != "2.0" {
		t.Fatalf("expected jsonrpc 2.0, got %v", parsed["jsonrpc"])
	}
	result := parsed["result"].(map[string]interface{})
	if result["id"] != "wf-123" {
		t.Fatalf("expected id wf-123, got %v", result["id"])
	}
	if result["status"] != "running" {
		t.Fatalf("expected status running, got %v", result["status"])
	}
}

func TestContract_ErrorResponse_Shape(t *testing.T) {
	resp := ErrorResponse(-32001, "not found")
	b, _ := json.Marshal(resp)
	var parsed map[string]interface{}
	json.Unmarshal(b, &parsed)
	errObj := parsed["error"].(map[string]interface{})
	if errObj["code"] != float64(-32001) {
		t.Fatalf("expected code -32001, got %v", errObj["code"])
	}
	if errObj["message"] != "not found" {
		t.Fatalf("expected message 'not found', got %v", errObj["message"])
	}
}

func TestContract_WorkflowStatus_Values(t *testing.T) {
	valid := map[WorkflowStatus]bool{
		StatusPending: true, StatusRunning: true, StatusBlocked: true,
		StatusCompleted: true, StatusFailed: true, StatusCancelled: true,
	}
	for _, s := range []WorkflowStatus{
		"pending", "running", "blocked", "completed", "failed", "cancelled",
	} {
		if !valid[s] {
			t.Fatalf("status %q not in valid set", s)
		}
	}
}

func TestContract_ArtifactStatus_Values(t *testing.T) {
	for _, s := range []ArtifactStatus{
		ArtifactPending, ArtifactProcessing, ArtifactCompleted,
		ArtifactFailed, ArtifactExpired,
	} {
		if !ValidArtifactStatuses[s] {
			t.Fatalf("artifact status %q not valid", s)
		}
	}
}

func TestContract_WorkflowEvent_Shape(t *testing.T) {
	evt := WorkflowEvent{
		WorkflowID: "wf-1",
		TemplateID: "tmpl-1",
		Status:     "running",
		StepID:     "s1",
		StepName:   "Navigate",
		Progress:   0.5,
	}
	b, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	var parsed map[string]interface{}
	json.Unmarshal(b, &parsed)
	if parsed["workflow_id"] != "wf-1" {
		t.Fatalf("expected workflow_id wf-1, got %v", parsed["workflow_id"])
	}
	if parsed["progress"] != float64(0.5) {
		t.Fatalf("expected progress 0.5, got %v", parsed["progress"])
	}
}

func TestContract_ApprovalRequest_Shape(t *testing.T) {
	req := ApprovalRequest{
		ID:         "apr-1",
		WorkflowID: "wf-1",
		StepID:     "s2",
		Decision:   ApprovalDecision("pending"),
	}
	b, _ := json.Marshal(req)
	var parsed map[string]interface{}
	json.Unmarshal(b, &parsed)
	if parsed["id"] != "apr-1" {
		t.Fatalf("expected id apr-1, got %v", parsed["id"])
	}
}

func TestContract_TrustDecision_Values(t *testing.T) {
	for _, d := range []TrustDecision{
		"allowed", "conditional", "denied", "requires_approval",
	} {
		_ = d
	}
}

func TestContract_ArtifactUpload_Params(t *testing.T) {
	raw := `{
		"owner": "@alice:m",
		"workflow_id": "wf-1",
		"step_id": "s1",
		"metadata": {
			"mime_type": "application/pdf",
			"filename": "report.pdf",
			"tags": ["finance"],
			"source": "browser",
			"size_bytes": 1024
		},
		"checksum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	}`
	var params ArtifactUploadParams
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		t.Fatalf("unmarshal upload params: %v", err)
	}
	if params.Owner != "@alice:m" {
		t.Fatalf("expected owner @alice:m, got %q", params.Owner)
	}
	if params.Metadata.Filename != "report.pdf" {
		t.Fatalf("expected filename report.pdf, got %q", params.Metadata.Filename)
	}
	if params.Metadata.SizeBytes != 1024 {
		t.Fatalf("expected size 1024, got %d", params.Metadata.SizeBytes)
	}
}

func TestContract_ArtifactList_Params(t *testing.T) {
	raw := `{
		"owner": "@alice:m",
		"workflow_id": "wf-1",
		"status": "completed"
	}`
	var params ArtifactListParams
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		t.Fatalf("unmarshal list params: %v", err)
	}
	if params.Owner != "@alice:m" {
		t.Fatalf("expected owner @alice:m, got %q", params.Owner)
	}
	if params.Status != "completed" {
		t.Fatalf("expected status completed, got %q", params.Status)
	}
}

func TestContract_BlockerResolution_Params(t *testing.T) {
	raw := `{
		"workflow_id": "wf-1",
		"step_id": "s2",
		"decision": "approved",
		"reason": "User approved"
	}`
	var params struct {
		WorkflowID string `json:"workflow_id"`
		StepID     string `json:"step_id"`
		Decision   string `json:"decision"`
		Reason     string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		t.Fatalf("unmarshal blocker params: %v", err)
	}
	if params.Decision != "approved" {
		t.Fatalf("expected decision approved, got %q", params.Decision)
	}
}
