package rpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveIdentityRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/rpc", nil)
	req.RemoteAddr = "192.168.1.100:54321"

	id := resolveClientIdentity(req, "sentinel")
	if id != "192.168.1.100:54321" {
		t.Errorf("expected RemoteAddr, got %q", id)
	}
}

func TestResolveIdentityForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/rpc", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 10.0.0.1")

	id := resolveClientIdentity(req, "sentinel")
	if id != "203.0.113.50" {
		t.Errorf("expected first X-Forwarded-For IP, got %q", id)
	}
}

func TestResolveIdentityUnixPeer(t *testing.T) {
	cred := &PeerCredentials{UID: 1000, GID: 1000, PID: 4242}
	ctx := ContextWithPeerCred(context.Background(), cred)

	req := httptest.NewRequest(http.MethodPost, "/rpc", nil)
	req = req.WithContext(ctx)
	req.RemoteAddr = "@"

	id := resolveClientIdentity(req, "native")
	if id != "uid:1000" {
		t.Errorf("expected uid:1000, got %q", id)
	}
}

func TestResolveIdentityUnixPeerMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/rpc", nil)
	req.RemoteAddr = "@"

	id := resolveClientIdentity(req, "native")
	if id != "@" {
		t.Errorf("expected fallback RemoteAddr, got %q", id)
	}
}
