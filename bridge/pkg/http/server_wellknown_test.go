package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// helper: call handleWellKnown and parse JSON response
func wellKnownResponse(t *testing.T, s *Server) map[string]interface{} {
	t.Helper()
	t.Setenv("ARMORCLAW_TLS_MODE", "")
	req := httptest.NewRequest(http.MethodGet, "/.well-known/matrix/client", nil)
	rec := httptest.NewRecorder()
	s.handleWellKnown(rec, req)

	var result map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("parse well-known JSON: %v", err)
	}
	return result
}

func TestWellKnownIncludesTLSMetadata(t *testing.T) {
	// Sentinel mode with self-signed cert → all 4 TLS fields populated
	s := newTestServer(t, "sentinel")
	certPEM, _ := generateTestSelfSignedCert(t)
	s.certPEM = certPEM

	resp := wellKnownResponse(t, s)

	ac, ok := resp["com.armorclaw"].(map[string]interface{})
	if !ok {
		t.Fatal("com.armorclaw section missing or not a map")
	}

	// Verify all 4 TLS fields exist and are non-zero
	if ac["tls_mode"] != "private" {
		t.Errorf("sentinel+self-signed: expected tls_mode 'private', got %v", ac["tls_mode"])
	}
	if fp, _ := ac["tls_fingerprint_sha256"].(string); fp == "" {
		t.Error("expected non-empty tls_fingerprint_sha256")
	}
	if trust, _ := ac["tls_trust_hint"].(string); trust != "self_signed" {
		t.Errorf("expected tls_trust_hint 'self_signed', got %v", trust)
	}
	if exp, _ := ac["cert_expires_at"].(float64); exp == 0 {
		t.Error("expected non-zero cert_expires_at")
	}
}

func TestWellKnownNativeModeZeroValues(t *testing.T) {
	// Native mode → all TLS fields present but zero-valued
	s := newTestServer(t, "native")

	resp := wellKnownResponse(t, s)

	ac, ok := resp["com.armorclaw"].(map[string]interface{})
	if !ok {
		t.Fatal("com.armorclaw section missing or not a map")
	}

	// tls_mode should be "none"
	if ac["tls_mode"] != "none" {
		t.Errorf("native mode: expected tls_mode 'none', got %v", ac["tls_mode"])
	}
	// Other fields should be present but zero-valued (not omitted)
	if fp, _ := ac["tls_fingerprint_sha256"].(string); fp != "" {
		t.Errorf("native mode: expected empty tls_fingerprint_sha256, got %q", fp)
	}
	if trust, _ := ac["tls_trust_hint"].(string); trust != "" {
		t.Errorf("native mode: expected empty tls_trust_hint, got %q", trust)
	}
	if exp, _ := ac["cert_expires_at"].(float64); exp != 0 {
		t.Errorf("native mode: expected cert_expires_at 0, got %v", exp)
	}
}

func TestWellKnownHomeserverUnchanged(t *testing.T) {
	s := newTestServer(t, "native")

	resp := wellKnownResponse(t, s)

	// m.homeserver must be present and correct
	hs, ok := resp["m.homeserver"].(map[string]interface{})
	if !ok {
		t.Fatal("m.homeserver section missing or not a map")
	}
	if hs["base_url"] != "https://matrix.armorclaw.app" {
		t.Errorf("m.homeserver.base_url: expected 'https://matrix.armorclaw.app', got %v", hs["base_url"])
	}

	// m.identity_server must be present and correct
	is, ok := resp["m.identity_server"].(map[string]interface{})
	if !ok {
		t.Fatal("m.identity_server section missing or not a map")
	}
	if is["base_url"] != "https://matrix.armorclaw.app" {
		t.Errorf("m.identity_server.base_url: expected 'https://matrix.armorclaw.app', got %v", is["base_url"])
	}
}
