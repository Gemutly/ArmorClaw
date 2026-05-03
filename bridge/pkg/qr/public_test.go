package qr

import (
	"strings"
	"testing"
	"time"
)

// testManager creates a QRManager with a known signing key for testing.
func testManager() *QRManager {
	return NewQRManager([]byte("test-signing-key-32-bytes-long-ok"), DefaultQRConfig(),
		"https://matrix.example.com", "https://bridge.example.com", "testserver")
}

func TestSignConfigV2IncludesTLSFields(t *testing.T) {
	m := testManager()

	config := &ConfigPayload{
		Version:               2,
		MatrixHomeserver:      "https://matrix.example.com",
		RpcURL:                "https://bridge.example.com/api",
		WsURL:                 "wss://bridge.example.com/ws",
		PushGateway:           "https://bridge.example.com/_matrix/push/v1/notify",
		ServerName:            "testserver",
		TLSMode:               "full",
		TLSFingerprintSHA256:  "abc123",
		TLSTrustHint:          "letsencrypt",
		CertExpiresAt:         9999999999,
		ExpiresAt:             time.Now().Add(1 * time.Hour).Unix(),
	}
	sig1 := m.signConfig(config)

	// Changing TLSTrustHint must change the signature
	config.TLSTrustHint = "tampered"
	sig2 := m.signConfig(config)
	if sig1 == sig2 {
		t.Fatal("signConfig v2: changing TLSTrustHint did not change HMAC")
	}

	// Changing CertExpiresAt must change the signature
	config.TLSTrustHint = "letsencrypt" // restore
	config.CertExpiresAt = 1
	sig3 := m.signConfig(config)
	if sig1 == sig3 {
		t.Fatal("signConfig v2: changing CertExpiresAt did not change HMAC")
	}
}

func TestValidateConfigRejectsTamperedTLSTrustHint(t *testing.T) {
	m := testManager()

	config := &ConfigPayload{
		Version:               2,
		MatrixHomeserver:      "https://matrix.example.com",
		RpcURL:                "https://bridge.example.com/api",
		PushGateway:           "https://bridge.example.com/_matrix/push/v1/notify",
		ServerName:            "testserver",
		TLSMode:               "full",
		TLSFingerprintSHA256:  "abc123",
		TLSTrustHint:          "letsencrypt",
		CertExpiresAt:         9999999999,
		ExpiresAt:             time.Now().Add(1 * time.Hour).Unix(),
	}
	config.Signature = m.signConfig(config)

	// Tamper with TLSTrustHint after signing
	config.TLSTrustHint = "self-signed-evil"
	if err := m.ValidateConfig(config); err == nil {
		t.Fatal("ValidateConfig accepted tampered TLSTrustHint")
	}
}

func TestValidateConfigRejectsTamperedCertExpiresAt(t *testing.T) {
	m := testManager()

	config := &ConfigPayload{
		Version:               2,
		MatrixHomeserver:      "https://matrix.example.com",
		RpcURL:                "https://bridge.example.com/api",
		PushGateway:           "https://bridge.example.com/_matrix/push/v1/notify",
		ServerName:            "testserver",
		TLSMode:               "full",
		TLSFingerprintSHA256:  "abc123",
		TLSTrustHint:          "letsencrypt",
		CertExpiresAt:         9999999999,
		ExpiresAt:             time.Now().Add(1 * time.Hour).Unix(),
	}
	config.Signature = m.signConfig(config)

	// Tamper with CertExpiresAt after signing
	config.CertExpiresAt = 1
	if err := m.ValidateConfig(config); err == nil {
		t.Fatal("ValidateConfig accepted tampered CertExpiresAt")
	}
}

func TestValidateConfigAcceptsValidV2(t *testing.T) {
	m := testManager()

	config := &ConfigPayload{
		Version:               2,
		MatrixHomeserver:      "https://matrix.example.com",
		RpcURL:                "https://bridge.example.com/api",
		WsURL:                 "wss://bridge.example.com/ws",
		PushGateway:           "https://bridge.example.com/_matrix/push/v1/notify",
		ServerName:            "testserver",
		TLSMode:               "full",
		TLSFingerprintSHA256:  "sha256abc123",
		TLSTrustHint:          "letsencrypt",
		CertExpiresAt:         9999999999,
		ExpiresAt:             time.Now().Add(1 * time.Hour).Unix(),
	}
	config.Signature = m.signConfig(config)

	if err := m.ValidateConfig(config); err != nil {
		t.Fatalf("ValidateConfig rejected valid v2 config: %v", err)
	}
}

func TestSignConfigV1Unchanged(t *testing.T) {
	m := testManager()

	config := &ConfigPayload{
		Version:          1,
		MatrixHomeserver: "https://matrix.example.com",
		RpcURL:           "https://bridge.example.com/api",
		WsURL:            "wss://bridge.example.com/ws",
		PushGateway:      "https://bridge.example.com/_matrix/push/v1/notify",
		ServerName:       "testserver",
		ExpiresAt:        time.Now().Add(1 * time.Hour).Unix(),
	}
	sig1 := m.signConfig(config)

	// Setting TLS fields on a v1 config must NOT change the signature
	config.TLSTrustHint = "tampered"
	config.CertExpiresAt = 1
	sig2 := m.signConfig(config)
	if sig1 != sig2 {
		t.Fatal("signConfig v1: TLS fields changed the HMAC — v1 format must be unchanged")
	}
}

func TestToTerminal(t *testing.T) {
	qrResult := &QRResult{DeepLink: "armorclaw://config?d=testdata"}
	result, err := qrResult.ToTerminal()
	if err != nil {
		t.Fatalf("ToTerminal() failed: %v", err)
	}
	if result == "" {
		t.Fatal("ToTerminal() returned empty string")
	}
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.Contains(line, "█") && !strings.Contains(line, " ") && !strings.Contains(line, "▄") && !strings.Contains(line, "▀") {
			t.Errorf("ToTerminal() output doesn't look like QR code (no block characters): %s", line)
		}
	}
}

func TestToTerminal_EmptyDeepLink(t *testing.T) {
	qrResult := &QRResult{DeepLink: ""}
	_, err := qrResult.ToTerminal()
	if err == nil {
		t.Fatal("ToTerminal() should fail with empty deep link")
	}
	if !strings.Contains(err.Error(), "deep link is empty") {
		t.Errorf("Expected error to contain 'deep link is empty', got: %v", err)
	}
}

func TestToTerminal_WhitespaceDeepLink(t *testing.T) {
	qrResult := &QRResult{DeepLink: "   \t\n  "}
	_, err := qrResult.ToTerminal()
	if err == nil {
		t.Fatal("ToTerminal() should fail with whitespace-only deep link")
	}
	if !strings.Contains(err.Error(), "deep link is empty") {
		t.Errorf("Expected error to contain 'deep link is empty', got: %v", err)
	}
}
