package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/armorclaw/bridge/pkg/secretary"
)

func TestMethodRegistration(t *testing.T) {
	criticalMethods := []string{
		"matrix.status",
		"matrix.login",
		"matrix.send",
		"matrix.join_room",
		"ai.chat",
		"health.check",
	}

	server := &Server{}
	server.registerHandlers()

	for _, method := range criticalMethods {
		t.Run(method, func(t *testing.T) {
			if _, exists := server.handlers[method]; !exists {
				t.Errorf("critical method %q not registered in handlers map", method)
			}
		})
	}
}

func TestMethodRegistrationCompleteness(t *testing.T) {
	expectedMethods := []string{
		"ai.chat",
		"browser.navigate",
		"browser.fill",
		"browser.click",
		"browser.status",
		"matrix.status",
		"matrix.login",
		"matrix.send",
		"matrix.join_room",
		"health.check",
	}

	server := &Server{}
	server.registerHandlers()

	for _, method := range expectedMethods {
		t.Run(method, func(t *testing.T) {
			if _, exists := server.handlers[method]; !exists {
				t.Errorf("expected method %q not registered in handlers map", method)
			}
		})
	}
}

func TestMatrixJoinRoomMissingRoomID(t *testing.T) {
	server := &Server{}
	server.registerHandlers()

	handler := server.handlers["matrix.join_room"]
	if handler == nil {
		t.Fatalf("matrix.join_room handler not registered")
	}

	// Test with empty room_id
	req := &Request{
		Params: json.RawMessage(`{"via_servers": [], "reason": "test"}`),
	}

	_, errObj := handler(context.Background(), req)
	if errObj == nil {
		t.Error("expected error for missing room_id, got nil")
	}

	if errObj.Code != InvalidParams {
		t.Errorf("expected InvalidParams code, got %d", errObj.Code)
	}

	if errObj.Message != "room_id is required" {
		t.Errorf("expected 'room_id is required' message, got '%s'", errObj.Message)
	}
}

func TestHealthCheckHandler(t *testing.T) {
	server := &Server{}
	server.registerHandlers()

	handler := server.handlers["health.check"]
	if handler == nil {
		t.Fatalf("health.check handler not registered")
	}

	req := &Request{
		Params: json.RawMessage(`{}`),
	}

	result, errObj := handler(context.Background(), req)
	if errObj != nil {
		t.Fatalf("unexpected error: %v", errObj)
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var healthResp HealthCheckResponse
	if err := json.Unmarshal(resultBytes, &healthResp); err != nil {
		t.Fatalf("failed to unmarshal HealthCheckResponse: %v", err)
	}

	if healthResp.Status == "" {
		t.Error("expected status to be set, got empty string")
	}

	if healthResp.Components == nil {
		t.Error("expected components to be set, got nil")
	}

	if _, ok := healthResp.Components["bridge"]; !ok {
		t.Error("expected components to contain 'bridge' key")
	}

	if _, ok := healthResp.Components["matrix"]; !ok {
		t.Error("expected components to contain 'matrix' key")
	}

	if _, ok := healthResp.Components["keystore"]; !ok {
		t.Error("expected components to contain 'keystore' key")
	}

	if !healthResp.BridgeReady {
		t.Error("expected bridge_ready to be true")
	}

	if healthResp.ProvisioningAvailable {
		t.Error("expected provisioning_available to be false when provisioningMgr is nil")
	}

	if !healthResp.IsNewServer {
		t.Error("expected is_new_server to be true when no owner claimed")
	}

	if healthResp.Version == "" {
		t.Error("expected version to be set")
	}

	if healthResp.Timestamp == "" {
		t.Error("expected timestamp to be set")
	}

	validStatuses := []string{"healthy", "degraded", "unhealthy"}
	valid := false
	for _, vs := range validStatuses {
		if healthResp.Status == vs {
			valid = true
			break
		}
	}
	if !valid {
		t.Errorf("expected status to be one of %v, got %s", validStatuses, healthResp.Status)
	}
}

func TestResolveBlocker_DeliversResponse(t *testing.T) {
	server := &Server{}
	server.registerHandlers()

	handler := server.handlers["resolve_blocker"]
	if handler == nil {
		t.Fatalf("resolve_blocker handler not registered")
	}

	workflowID := "test-wf-deliver"
	stepID := "step-1"
	key := "blocker:" + workflowID + ":" + stepID

	ch := make(chan secretary.BlockerResponse, 1)
	secretary.PendingBlockersMap().Store(key, ch)
	defer secretary.PendingBlockersMap().Delete(key)

	req := &Request{
		Params: json.RawMessage(`{"workflow_id":"test-wf-deliver","step_id":"step-1","input":"my-secret-input","note":"optional note"}`),
	}

	result, errObj := handler(context.Background(), req)
	if errObj != nil {
		t.Fatalf("unexpected error: %v", errObj)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if resultMap["status"] != "delivered" {
		t.Errorf("expected status 'delivered', got %v", resultMap["status"])
	}

	select {
	case resp := <-ch:
		if resp.Input != "my-secret-input" {
			t.Errorf("expected input 'my-secret-input', got %q", resp.Input)
		}
		if resp.Note != "optional note" {
			t.Errorf("expected note 'optional note', got %q", resp.Note)
		}
		if resp.ProvidedAt == 0 {
			t.Error("expected ProvidedAt to be set")
		}
	default:
		t.Error("expected response on channel, but channel was empty")
	}
}

func TestResolveBlocker_NoPendingBlocker(t *testing.T) {
	server := &Server{}
	server.registerHandlers()

	handler := server.handlers["resolve_blocker"]
	if handler == nil {
		t.Fatalf("resolve_blocker handler not registered")
	}

	req := &Request{
		Params: json.RawMessage(`{"workflow_id":"no-such-wf","step_id":"no-step","input":"whatever"}`),
	}

	_, errObj := handler(context.Background(), req)
	if errObj == nil {
		t.Fatal("expected error for no pending blocker, got nil")
	}
	if errObj.Code != InvalidParams {
		t.Errorf("expected InvalidParams code, got %d", errObj.Code)
	}
}

func TestResolveBlocker_MissingFields(t *testing.T) {
	server := &Server{}
	server.registerHandlers()

	handler := server.handlers["resolve_blocker"]
	if handler == nil {
		t.Fatalf("resolve_blocker handler not registered")
	}

	tests := []struct {
		name   string
		params string
	}{
		{"empty workflow_id", `{"workflow_id":"","step_id":"s1","input":"val"}`},
		{"empty step_id", `{"workflow_id":"wf1","step_id":"","input":"val"}`},
		{"empty input", `{"workflow_id":"wf1","step_id":"s1","input":""}`},
		{"missing workflow_id", `{"step_id":"s1","input":"val"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &Request{Params: json.RawMessage(tt.params)}
			_, errObj := handler(context.Background(), req)
			if errObj == nil {
				t.Error("expected error for missing fields, got nil")
			}
			if errObj.Code != InvalidParams {
				t.Errorf("expected InvalidParams code, got %d", errObj.Code)
			}
		})
	}
}
