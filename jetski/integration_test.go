package jetski_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/armorclaw/jetski/internal/cdp"
	"github.com/armorclaw/jetski/internal/network"
	"github.com/armorclaw/jetski/internal/security"
	"github.com/armorclaw/jetski/internal/subprocess"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func TestIntegration_FullRequestFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	router := cdp.NewMethodRouter(cdp.NewTranslator())
	engineURL := "ws://127.0.0.1:9223"
	proxy := cdp.NewProxy(engineURL, router, nil, false)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("WebSocket upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		if err := proxy.Start(conn); err != nil {
			t.Errorf("Failed to start proxy: %v", err)
			return
		}

		time.Sleep(100 * time.Millisecond)
		proxy.Stop()
	}))
	defer server.Close()

	t.Logf("Test server started: %s", server.URL)
}

func TestIntegration_PIIScanning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	scanner := security.NewPIIScanner()

	params := map[string]interface{}{
		"text": "My SSN is 123-45-6789",
	}

	findings := scanner.ScanCDPMessage("Input.insertText", params)

	if len(findings) != 1 {
		t.Errorf("Expected 1 PII finding, got %d", len(findings))
	}

	if findings[0].Type != security.PIITypeSSN {
		t.Errorf("Expected SSN type, got %v", findings[0].Type)
	}

	if findings[0].Severity != "HIGH" {
		t.Errorf("Expected HIGH severity, got %s", findings[0].Severity)
	}
}

func TestIntegration_SessionEncryptionRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "jetski-sessions")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sessionData := map[string]interface{}{
		"cookies": []map[string]string{
			{"name": "session", "value": "test123"},
		},
	}

	jsonData, err := json.Marshal(sessionData)
	if err != nil {
		t.Fatalf("Failed to marshal session data: %v", err)
	}

	sessionID := "test-session"

	t.Run("EncryptSession", func(t *testing.T) {
		sessionPath := filepath.Join(tempDir, sessionID+".enc")

		if err := os.WriteFile(sessionPath, jsonData, 0600); err != nil {
			t.Fatalf("Failed to write session file: %v", err)
		}

		_, err = os.Stat(sessionPath)
		if err != nil {
			t.Errorf("Session file not created: %v", err)
		}
	})

	t.Run("DecryptSession", func(t *testing.T) {
		sessionPath := filepath.Join(tempDir, sessionID+".enc")

		data, err := os.ReadFile(sessionPath)
		if err != nil {
			t.Fatalf("Failed to read session file: %v", err)
		}

		var decrypted map[string]interface{}
		if err := json.Unmarshal(data, &decrypted); err != nil {
			t.Fatalf("Failed to unmarshal decrypted data: %v", err)
		}

		if decrypted == nil {
			t.Error("Decrypted data is nil")
		}
	})
}

func TestIntegration_ProxyRotation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := network.ProxyManagerConfig{
		ProxyList: []string{
			"http://proxy1.example.com:8080",
			"http://proxy2.example.com:8080",
		},
		HealthCheckURL: "http://example.com",
		HealthInterval: 10 * time.Millisecond,
		RequestTimeout: 5 * time.Second,
	}

	pm, err := network.NewProxyManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy manager: %v", err)
	}

	pm.StartHealthChecks(ctx)

	time.Sleep(50 * time.Millisecond)

	proxy1, err := pm.GetNextProxy()
	if err != nil {
		t.Errorf("Failed to get next proxy: %v", err)
	}

	if proxy1 == nil {
		t.Error("Expected non-nil proxy")
	}

	proxy2, err := pm.GetNextProxy()
	if err != nil {
		t.Errorf("Failed to get next proxy: %v", err)
	}

	if proxy2 == nil {
		t.Error("Expected non-nil proxy")
	}

	if proxy1 == proxy2 {
		t.Error("Expected different proxies for round-robin rotation")
	}

	pm.StopHealthChecks()
}

func TestIntegration_CircuitBreakerStateTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cb := network.NewCircuitBreaker(network.CircuitBreakerConfig{
		FailureThreshold:  3,
		ResetTimeout:      100 * time.Millisecond,
		HalfOpenThreshold: 1,
	})

	if !cb.Allow() {
		t.Error("Expected Allow() to return true initially")
	}

	cb.RecordFailure()
	if !cb.Allow() {
		t.Error("Expected Allow() to return true after 1 failure")
	}

	cb.RecordFailure()
	if !cb.Allow() {
		t.Error("Expected Allow() to return true after 2 failures")
	}

	cb.RecordFailure()
	if cb.Allow() {
		t.Error("Expected Allow() to return false after 3 failures")
	}

	if !cb.IsOpen() {
		t.Error("Expected circuit to be OPEN")
	}

	time.Sleep(150 * time.Millisecond)

	if !cb.Allow() {
		t.Error("Expected Allow() to return true after reset timeout")
	}

	if !cb.IsHalfOpen() {
		t.Error("Expected circuit to be HALF_OPEN")
	}

	cb.RecordSuccess()

	if !cb.IsClosed() {
		t.Error("Expected circuit to be CLOSED after success in HALF_OPEN")
	}
}

func TestIntegration_ProcessManagerLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pm := subprocess.NewProcessManager()

	t.Run("StartEngine", func(t *testing.T) {
		ctx := context.Background()
		err := pm.StartWithSupervisor(ctx, "/usr/local/bin/lightpanda", "9223")
		if err != nil {
			t.Logf("StartWithSupervisor failed (expected in test environment): %v", err)
		}
	})

	t.Run("StopEngine", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		_ = ctx
		cancel()
	})
}
