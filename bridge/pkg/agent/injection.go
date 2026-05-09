package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/armorclaw/bridge/pkg/trust"
)

type PIIRequest struct {
	Hash   string `json:"hash"`
	Secret string `json:"secret"`
}

type PIIResponse struct {
	Value string `json:"value,omitempty"`
	Error string `json:"error,omitempty"`
}

type PIIInjector struct {
	socketPath string
	mapping    *PIIMapping
	secret     string
	listener   net.Listener
	mu         sync.Mutex
	stopped    bool
	guard      *trust.TrustedProxyGuard
}

func NewPIIInjector(socketPath, secret string, mapping *PIIMapping) *PIIInjector {
	return &PIIInjector{
		socketPath: socketPath,
		mapping:    mapping,
		secret:     secret,
	}
}

func (i *PIIInjector) Start() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.listener != nil {
		return fmt.Errorf("injector already started")
	}

	if err := os.MkdirAll(filepath.Dir(i.socketPath), 0755); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}
	_ = os.Remove(i.socketPath)

	listener, err := net.Listen("unix", i.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket listener: %w", err)
	}

	if err := os.Chmod(i.socketPath, 0660); err != nil {
		listener.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	i.listener = listener
	i.stopped = false

	go i.handleConnections()

	return nil
}

func (i *PIIInjector) Stop() {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.stopped || i.listener == nil {
		return
	}

	i.stopped = true
	i.listener.Close()
	os.Remove(i.socketPath)
}

func (i *PIIInjector) handleConnections() {
	for {
		i.mu.Lock()
		if i.stopped || i.listener == nil {
			i.mu.Unlock()
			return
		}
		i.mu.Unlock()

		conn, err := i.listener.Accept()
		if err != nil {
			i.mu.Lock()
			if i.stopped {
				i.mu.Unlock()
				return
			}
			i.mu.Unlock()
			continue
		}

		go i.handleConnection(conn)
	}
}

func (i *PIIInjector) handleConnection(conn net.Conn) {
	if i.guard != nil {
		if err := i.guard.Check(conn.RemoteAddr()); err != nil {
			slog.Warn("trusted_proxy_guard_rejected", "error", err)
			conn.Close()
			return
		}
	}

	defer conn.Close()

	var req PIIRequest
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&req); err != nil {
		return
	}

	resp := PIIResponse{}
	if req.Secret != i.secret {
		resp.Error = "authentication failed"
	} else {
		if val, ok := i.mapping.Get(req.Hash); ok {
			resp.Value = val
		} else {
			resp.Error = "hash not found"
		}
	}

	encoder := json.NewEncoder(conn)
	encoder.Encode(resp)

	time.Sleep(10 * time.Millisecond)
}
