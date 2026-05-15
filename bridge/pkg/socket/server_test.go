package socket

import (
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/armorclaw/bridge/pkg/keystore"
)

// skipIfNoSQLCipher skips the test if SQLCipher is unavailable.
func skipIfNoSQLCipher(t *testing.T) {
	t.Helper()
	ks, err := keystore.New(keystore.Config{
		DBPath:    filepath.Join(t.TempDir(), "check.db"),
		MasterKey: make([]byte, 32),
	})
	if err != nil {
		t.Skip("skipping: SQLCipher unavailable")
	}
	ks.Close()
}

// newTestKeystore creates a keystore for testing (requires SQLCipher).
func newTestKeystore(t *testing.T) *keystore.Keystore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}
	ks, err := keystore.New(keystore.Config{
		DBPath:    dbPath,
		MasterKey: masterKey,
	})
	if err != nil {
		t.Fatalf("create keystore: %v", err)
	}
	if err := ks.Open(); err != nil {
		t.Fatalf("open keystore: %v", err)
	}
	t.Cleanup(func() { ks.Close() })
	return ks
}

// newTestServer creates a Server with a temp socket path and real keystore.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	skipIfNoSQLCipher(t)
	ks := newTestKeystore(t)
	socketPath := filepath.Join(t.TempDir(), "test.sock")
	srv, err := New(Config{
		SocketPath: socketPath,
		Keystore:   ks,
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	return srv
}

func TestNew_NilKeystore_ReturnsError(t *testing.T) {
	_, err := New(Config{Keystore: nil})
	if err == nil {
		t.Fatal("expected error for nil keystore")
	}
	if err.Error() != "keystore is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNew_DefaultSocketPath(t *testing.T) {
	skipIfNoSQLCipher(t)
	ks := newTestKeystore(t)
	srv, err := New(Config{Keystore: ks})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	if srv.GetSocketPath() != DefaultSocketPath {
		t.Fatalf("expected default socket path %q, got %q", DefaultSocketPath, srv.GetSocketPath())
	}
}

func TestServer_StartStop(t *testing.T) {
	srv := newTestServer(t)

	if err := srv.Start(); err != nil {
		t.Fatalf("Start(): %v", err)
	}

	if _, err := filepath.Glob(srv.GetSocketPath()); err != nil {
		t.Fatalf("socket file check: %v", err)
	}

	conn, err := net.DialTimeout("unix", srv.GetSocketPath(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn.Close()

	if err := srv.Stop(); err != nil {
		t.Fatalf("Stop(): %v", err)
	}

	if _, err := net.DialTimeout("unix", srv.GetSocketPath(), 100*time.Millisecond); err == nil {
		t.Fatal("expected dial to fail after Stop()")
	}
}

func TestHandleMessage_InvalidVersion(t *testing.T) {
	srv := newTestServer(t)

	msg := &Message{
		JSONRPC: "1.0",
		ID:      float64(1),
		Method:  "status",
	}

	resp := srv.handleMessage(msg)

	if resp.JSONRPC != "2.0" {
		t.Fatalf("expected jsonrpc 2.0, got %q", resp.JSONRPC)
	}
	if resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != CodeInvalidRequest {
		t.Fatalf("expected code %d, got %d", CodeInvalidRequest, resp.Error.Code)
	}
	if resp.Error.Message != "invalid JSON-RPC version" {
		t.Fatalf("unexpected message: %q", resp.Error.Message)
	}
}

func TestHandleMessage_UnknownMethod(t *testing.T) {
	srv := newTestServer(t)

	msg := &Message{
		JSONRPC: "2.0",
		ID:      float64(42),
		Method:  "nonexistent_method",
	}

	resp := srv.handleMessage(msg)

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != CodeMethodNotFound {
		t.Fatalf("expected code %d, got %d", CodeMethodNotFound, resp.Error.Code)
	}
}

func TestHandleStatus(t *testing.T) {
	srv := newTestServer(t)

	msg := &Message{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "status",
	}

	resp := srv.handleMessage(msg)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}
	if result["state"] != "running" {
		t.Fatalf("expected state=running, got %v", result["state"])
	}
	if result["version"] != "1.0.0" {
		t.Fatalf("expected version=1.0.0, got %v", result["version"])
	}
	if result["socket"] != srv.GetSocketPath() {
		t.Fatalf("socket mismatch: %v", result["socket"])
	}
}

func TestHandleHealth(t *testing.T) {
	srv := newTestServer(t)

	msg := &Message{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "health",
	}

	resp := srv.handleMessage(msg)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}
	if result["status"] != "healthy" {
		t.Fatalf("expected status=healthy, got %v", result["status"])
	}
}

func TestHandleStop_MissingContainerID(t *testing.T) {
	srv := newTestServer(t)

	msg := &Message{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "stop",
		Params:  json.RawMessage(`{}`),
	}

	resp := srv.handleMessage(msg)

	if resp.Error == nil {
		t.Fatal("expected error for missing container_id")
	}
	if resp.Error.Code != CodeInvalidParams {
		t.Fatalf("expected code %d, got %d", CodeInvalidParams, resp.Error.Code)
	}
}

func TestHandleStart_MissingKeyID(t *testing.T) {
	srv := newTestServer(t)

	msg := &Message{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "start",
		Params:  json.RawMessage(`{"agent_type":"browser","image":"test"}`),
	}

	resp := srv.handleMessage(msg)

	if resp.Error == nil {
		t.Fatal("expected error for missing key_id")
	}
	if resp.Error.Code != CodeInvalidParams {
		t.Fatalf("expected code %d, got %d", CodeInvalidParams, resp.Error.Code)
	}
}

func TestHandleGetCredential_MissingID(t *testing.T) {
	srv := newTestServer(t)

	msg := &Message{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "get_credential",
	}

	resp := srv.handleMessage(msg)

	if resp.Error == nil {
		t.Fatal("expected error for missing id param")
	}
	if resp.Error.Code != CodeInvalidParams {
		t.Fatalf("expected code %d, got %d", CodeInvalidParams, resp.Error.Code)
	}
}

func TestHandleListCredentials_InvalidProvider(t *testing.T) {
	srv := newTestServer(t)

	msg := &Message{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "list_credentials",
		Params:  json.RawMessage(`{"provider":"bogus_provider"}`),
	}

	resp := srv.handleMessage(msg)

	if resp.Error == nil {
		t.Fatal("expected error for invalid provider")
	}
	if resp.Error.Code != CodeInvalidParams {
		t.Fatalf("expected code %d, got %d", CodeInvalidParams, resp.Error.Code)
	}
}

func TestAcceptConnections_Concurrency(t *testing.T) {
	srv := newTestServer(t)

	if err := srv.Start(); err != nil {
		t.Fatalf("Start(): %v", err)
	}
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	const numClients = 10
	var wg sync.WaitGroup
	errCh := make(chan error, numClients)

	for i := range numClients {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn, err := net.DialTimeout("unix", srv.GetSocketPath(), 2*time.Second)
			if err != nil {
				errCh <- err
				return
			}
			defer conn.Close()

			req := Message{
				JSONRPC: "2.0",
				ID:      float64(id),
				Method:  "health",
			}
			encoder := json.NewEncoder(conn)
			decoder := json.NewDecoder(conn)

			if err := encoder.Encode(&req); err != nil {
				errCh <- err
				return
			}

			var resp Message
			if err := decoder.Decode(&resp); err != nil {
				errCh <- err
				return
			}

			result, ok := resp.Result.(map[string]interface{})
			if !ok {
				errCh <- errors.New("result is not a map")
				return
			}
			if result["status"] != "healthy" {
				errCh <- errors.New("unexpected health status")
				return
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("client error: %v", err)
	}
}

func TestSendError_NilEncoder(t *testing.T) {
	srv := newTestServer(t)

	err := srv.sendError(nil, float64(1), CodeInternalError, "test", nil)
	if err == nil {
		t.Fatal("expected error for nil encoder")
	}
}

func TestErrorConstants(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{ErrServerClosed, "server closed"},
		{ErrInvalidMessage, "invalid message format"},
		{ErrUnauthorized, "unauthorized access"},
		{ErrContainerNotFound, "container not found"},
	}
	for _, tt := range tests {
		if tt.err.Error() != tt.want {
			t.Errorf("got %q, want %q", tt.err.Error(), tt.want)
		}
	}
}
