package rpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/armorclaw/jetski/internal/cdp"
)

func TestRPCStatusReturnsSessionInfo(t *testing.T) {
	srv := NewServer(nil, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/rpc/status")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if _, ok := body["active_sessions"]; !ok {
		t.Error("response missing 'active_sessions' field")
	}
	if _, ok := body["engine_health"]; !ok {
		t.Error("response missing 'engine_health' field")
	}
}

func TestRPCSessionCreateReturnsID(t *testing.T) {
	srv := NewServer(nil, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/rpc/session/create", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	id, ok := body["id"].(string)
	if !ok || id == "" {
		t.Error("response missing non-empty 'id' string field")
	}
	if !strings.HasPrefix(id, "session-") {
		t.Errorf("session ID should start with 'session-', got %q", id)
	}
}

func TestRPCSessionCreateIncrementsActiveCount(t *testing.T) {
	srv := NewServer(nil, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Create a session
	resp, _ := http.Post(ts.URL+"/rpc/session/create", "application/json", strings.NewReader("{}"))
	resp.Body.Close()

	// Check status shows 1 active session
	resp, err := http.Get(ts.URL + "/rpc/status")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	active := body["active_sessions"].(float64)
	if active != 1 {
		t.Errorf("expected 1 active session, got %.0f", active)
	}
}

func TestRPCSessionCloseRemovesSession(t *testing.T) {
	srv := NewServer(nil, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Create a session first
	createResp, _ := http.Post(ts.URL+"/rpc/session/create", "application/json", strings.NewReader("{}"))
	var createBody map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&createBody)
	createResp.Body.Close()
	sessionID := createBody["id"].(string)

	// Close the session
	closeBody := `{"id":"` + sessionID + `"}`
	closeResp, err := http.Post(ts.URL+"/rpc/session/close", "application/json", strings.NewReader(closeBody))
	if err != nil {
		t.Fatalf("close request failed: %v", err)
	}
	defer closeResp.Body.Close()

	if closeResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", closeResp.StatusCode)
	}

	var closeResult map[string]interface{}
	json.NewDecoder(closeResp.Body).Decode(&closeResult)
	if closeResult["status"] != "closed" {
		t.Errorf("expected status 'closed', got %v", closeResult["status"])
	}

	// Verify session count is 0
	statusResp, _ := http.Get(ts.URL + "/rpc/status")
	var statusBody map[string]interface{}
	json.NewDecoder(statusResp.Body).Decode(&statusBody)
	statusResp.Body.Close()

	active := statusBody["active_sessions"].(float64)
	if active != 0 {
		t.Errorf("expected 0 active sessions after close, got %.0f", active)
	}
}

func TestRPCSessionCloseInvalidID(t *testing.T) {
	srv := NewServer(nil, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	closeResp, _ := http.Post(ts.URL+"/rpc/session/close", "application/json",
		strings.NewReader(`{"id":"session-nonexistent"}`))
	defer closeResp.Body.Close()

	if closeResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent session, got %d", closeResp.StatusCode)
	}
}

func TestRPCHealthReturnsDetailedInfo(t *testing.T) {
	srv := NewServer(nil, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/rpc/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if _, ok := body["uptime"]; !ok {
		t.Error("response missing 'uptime' field")
	}
	if _, ok := body["status"]; !ok {
		t.Error("response missing 'status' field")
	}
}

func TestRPCHealthUptimeIncreases(t *testing.T) {
	srv := NewServer(nil, nil)
	time.Sleep(10 * time.Millisecond)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/rpc/health")
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()

	uptime := body["uptime"].(float64)
	if uptime <= 0 {
		t.Errorf("uptime should be positive, got %v", uptime)
	}
}

func TestRPCUnknownPathReturns404(t *testing.T) {
	srv := NewServer(nil, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/rpc/nonexistent")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRPCEventsSubscribeDisabledReturns503(t *testing.T) {
	em := cdp.NewEventEmitter(false)
	srv := NewServer(nil, em)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/rpc/events.subscribe", "application/json",
		strings.NewReader(`{"type":"register","payload":{"device_id":"dev-1"}}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when emitter disabled, got %d", resp.StatusCode)
	}
}

func TestRPCEventsSubscribeNilEmitterReturns503(t *testing.T) {
	srv := NewServer(nil, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/rpc/events.subscribe", "application/json",
		strings.NewReader(`{"type":"register","payload":{"device_id":"dev-1"}}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when emitter is nil, got %d", resp.StatusCode)
	}
}

func TestRPCEventsSubscribeBadHandshake(t *testing.T) {
	em := cdp.NewEventEmitter(true)
	srv := NewServer(nil, em)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/rpc/events.subscribe", "application/json",
		strings.NewReader(`{"type":"wrong","payload":{"device_id":"dev-1"}}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad handshake type, got %d", resp.StatusCode)
	}
}

func TestRPCEventsSubscribeGETNotAllowed(t *testing.T) {
	em := cdp.NewEventEmitter(true)
	srv := NewServer(nil, em)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/rpc/events.subscribe")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for GET, got %d", resp.StatusCode)
	}
}
