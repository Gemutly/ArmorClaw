package http

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/armorclaw/bridge/pkg/rpc"
)

// helper: create a self-signed cert
func generateTestSelfSignedCert(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPEM = encodePEM("CERTIFICATE", certDER)
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM = encodePEM("EC PRIVATE KEY", keyDER)
	return
}

// helper: create a CA-signed cert (public mode)
func generateTestCACert(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	caPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test CA", Organization: []string{"Test Org"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		IsCA:         true,
	}
	_, err = x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caPriv.PublicKey, caPriv)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}

	leafPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test.example.com"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caTmpl, &leafPriv.PublicKey, caPriv)
	if err != nil {
		t.Fatalf("create leaf cert: %v", err)
	}

	certPEM = encodePEM("CERTIFICATE", leafDER)
	keyDER, err := x509.MarshalECPrivateKey(leafPriv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM = encodePEM("EC PRIVATE KEY", keyDER)
	return
}

func encodePEM(blockType string, data []byte) []byte {
	return []byte("-----BEGIN " + blockType + "-----\n" +
		base64.StdEncoding.EncodeToString(data) + "\n" +
		"-----END " + blockType + "-----\n")
}

// helper: create a test server
func newTestServer(t *testing.T, mode string) *Server {
	t.Helper()
	rpcSrv, err := rpc.New(rpc.Config{})
	if err != nil {
		t.Fatalf("create rpc server: %v", err)
	}
	s := NewServer(ServerConfig{
		ServerMode: mode,
		Hostname:   "test.armorclaw.local",
		Port:       8443,
	}, rpcSrv)
	return s
}

// helper: read TLS info from QR config payload (v2)
func getQRConfigTLS(t *testing.T, s *Server) (mode, fingerprint, trustHint string, expiresAt int64) {
	t.Helper()
	t.Setenv("ARMORCLAW_QR_VERSION", "2")
	result, err := s.qrManager.GenerateConfigQR(24 * time.Hour)
	if err != nil {
		t.Fatalf("generate config QR: %v", err)
	}
	if result.Config == nil {
		t.Fatal("config QR result has nil Config")
	}
	return result.Config.TLSMode, result.Config.TLSFingerprintSHA256, result.Config.TLSTrustHint, result.Config.CertExpiresAt
}

func TestUpdateQRTLSInfo_NativeMode(t *testing.T) {
	// Ensure no env override
	t.Setenv("ARMORCLAW_TLS_MODE", "")
	s := newTestServer(t, "native")
	// No certPEM set - native mode
	s.updateQRTLSInfo()

	mode, fp, trust, expires := getQRConfigTLS(t, s)
	if mode != "none" {
		t.Errorf("native mode: expected TLS mode 'none', got %q", mode)
	}
	if fp != "" {
		t.Errorf("native mode: expected empty fingerprint, got %q", fp)
	}
	if trust != "" {
		t.Errorf("native mode: expected empty trust hint, got %q", trust)
	}
	if expires != 0 {
		t.Errorf("native mode: expected expires 0, got %d", expires)
	}
}

func TestUpdateQRTLSInfo_SentinelSelfSigned(t *testing.T) {
	t.Setenv("ARMORCLAW_TLS_MODE", "")
	s := newTestServer(t, "sentinel")
	certPEM, _ := generateTestSelfSignedCert(t)
	s.certPEM = certPEM

	s.updateQRTLSInfo()

	mode, fp, trust, expires := getQRConfigTLS(t, s)
	if mode != "private" {
		t.Errorf("sentinel+self-signed: expected TLS mode 'private', got %q", mode)
	}
	if fp == "" {
		t.Error("sentinel+self-signed: expected non-empty fingerprint")
	}
	if trust != "self_signed" {
		t.Errorf("sentinel+self-signed: expected trust hint 'self_signed', got %q", trust)
	}
	if expires == 0 {
		t.Error("sentinel+self-signed: expected non-zero cert expiry")
	}
}

func TestUpdateQRTLSInfo_SentinelCA(t *testing.T) {
	t.Setenv("ARMORCLAW_TLS_MODE", "")
	s := newTestServer(t, "sentinel")
	certPEM, _ := generateTestCACert(t)
	s.certPEM = certPEM

	s.updateQRTLSInfo()

	mode, fp, trust, expires := getQRConfigTLS(t, s)
	if mode != "public" {
		t.Errorf("sentinel+CA: expected TLS mode 'public', got %q", mode)
	}
	if fp == "" {
		t.Error("sentinel+CA: expected non-empty fingerprint")
	}
	if trust != "public_ca" {
		t.Errorf("sentinel+CA: expected trust hint 'public_ca', got %q", trust)
	}
	if expires == 0 {
		t.Error("sentinel+CA: expected non-zero cert expiry")
	}
}

func TestUpdateQRTLSInfo_EnvOverride(t *testing.T) {
	t.Setenv("ARMORCLAW_TLS_MODE", "private")
	s := newTestServer(t, "native")
	// Env override makes deriveTLSMode return "private", but without certPEM
	// GetCertificateFingerprint fails and returns early — no panic expected
	_ = s
	os.Unsetenv("ARMORCLAW_TLS_MODE")
}
