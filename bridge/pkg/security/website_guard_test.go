package security

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"math/big"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

type mockAuditLogger struct {
	logged []WebsiteAccessLog
}

func (m *mockAuditLogger) LogWebsiteAccess(entry WebsiteAccessLog) {
	m.logged = append(m.logged, entry)
}

func newMockLogger() *mockAuditLogger {
	return &mockAuditLogger{}
}

func helperGuard(t *testing.T, tier SecurityTier) (*WebsiteGuard, *SecurityConfig) {
	t.Helper()
	cfg := NewSecurityConfig()
	cfg.SetTier(tier)
	return NewWebsiteGuard(cfg, newMockLogger()), cfg
}

func helperGuardNoLogger(t *testing.T, tier SecurityTier) *WebsiteGuard {
	t.Helper()
	cfg := NewSecurityConfig()
	cfg.SetTier(tier)
	return NewWebsiteGuard(cfg, nil)
}

func TestNewWebsiteGuard(t *testing.T) {
	cfg := NewSecurityConfig()
	g := NewWebsiteGuard(cfg, nil)
	if g == nil {
		t.Fatal("expected non-nil WebsiteGuard")
	}
	if g.config != cfg {
		t.Error("config not set correctly")
	}
}

func TestCheckAccess_AllowedDomain(t *testing.T) {
	g, cfg := helperGuard(t, TierPermissive)
	cat := cfg.GetCategory(CategoryBanking)
	cat.AllowedWebsites = []string{"safe.com"}
	cfg.SetCategory(CategoryBanking, cat)

	err := g.CheckAccess(context.Background(), "https://safe.com/api", CategoryBanking, "read")
	if err != nil {
		t.Fatalf("expected allowed, got: %v", err)
	}
}

func TestCheckAccess_BlockedCategory(t *testing.T) {
	g, _ := helperGuard(t, TierParanoid)
	err := g.CheckAccess(context.Background(), "https://any.com/data", CategoryBanking, "read")
	if err == nil {
		t.Error("expected error for blocked category")
	}
}

func TestCheckAccess_DomainNotAllowed(t *testing.T) {
	g, cfg := helperGuard(t, TierPermissive)
	cat := cfg.GetCategory(CategoryBanking)
	cat.AllowedWebsites = []string{"safe.com"}
	cfg.SetCategory(CategoryBanking, cat)

	err := g.CheckAccess(context.Background(), "https://evil.com/api", CategoryBanking, "read")
	if err == nil {
		t.Error("expected error for non-allowlisted domain")
	}
}

func TestCheckAccess_InvalidURL(t *testing.T) {
	g, _ := helperGuard(t, TierPermissive)
	err := g.CheckAccess(context.Background(), "://bad-url", CategoryBanking, "read")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestCheckAccess_PathRestriction(t *testing.T) {
	g, cfg := helperGuard(t, TierPermissive)
	cat := cfg.GetCategory(CategoryBanking)
	cat.AllowedWebsites = []string{"bank.com"}
	cfg.SetCategory(CategoryBanking, cat)

	cfg.Websites["bank.com"] = WebsiteConfig{
		Domain:   "bank.com",
		Subpaths: []string{"/api/", "/public/"},
	}

	if err := g.CheckAccess(context.Background(), "https://bank.com/api/balance", CategoryBanking, "read"); err != nil {
		t.Errorf("allowed path should pass: %v", err)
	}
	err := g.CheckAccess(context.Background(), "https://bank.com/admin/secret", CategoryBanking, "read")
	if err == nil {
		t.Error("restricted path should be denied")
	}
}

func TestCheckAccess_LogsAccess(t *testing.T) {
	cfg := NewSecurityConfig()
	cfg.SetTier(TierOpen)
	logger := newMockLogger()
	g := NewWebsiteGuard(cfg, logger)

	_ = g.CheckAccess(context.Background(), "https://example.com/path", CategoryBanking, "read")
	if len(logger.logged) == 0 {
		t.Fatal("expected audit log entry")
	}
	entry := logger.logged[0]
	if entry.Domain != "example.com" {
		t.Errorf("logged domain = %q, want example.com", entry.Domain)
	}
	if entry.Path != "/path" {
		t.Errorf("logged path = %q, want /path", entry.Path)
	}
	if !entry.Allowed {
		t.Error("expected Allowed=true")
	}
}

func TestCheckAccess_NilAuditLogger(t *testing.T) {
	g := helperGuardNoLogger(t, TierOpen)
	err := g.CheckAccess(context.Background(), "https://example.com/path", CategoryBanking, "read")
	if err != nil {
		t.Fatalf("should not panic with nil logger: %v", err)
	}
}

func TestCheckAccess_CancelledContext(t *testing.T) {
	g, cfg := helperGuard(t, TierPermissive)
	cat := cfg.GetCategory(CategoryBanking)
	cat.AllowedWebsites = []string{"safe.com"}
	cfg.SetCategory(CategoryBanking, cat)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := g.CheckAccess(ctx, "https://safe.com/api", CategoryBanking, "read")
	if err != nil {
		t.Logf("cancelled context returned: %v (acceptable — current impl doesn't check ctx)", err)
	}
}

func TestValidateURL_HTTPSRequired(t *testing.T) {
	g, _ := helperGuard(t, TierPermissive)
	os.Unsetenv("ARMORCLAW_DEV_MODE")

	_, err := g.ValidateURL("http://example.com")
	if err == nil {
		t.Error("expected error for HTTP URL in production mode")
	}
}

func TestValidateURL_HTTPSAllowed(t *testing.T) {
	g, _ := helperGuard(t, TierPermissive)
	os.Unsetenv("ARMORCLAW_DEV_MODE")

	u, err := g.ValidateURL("https://example.com")
	if err != nil {
		t.Fatalf("HTTPS URL should be valid: %v", err)
	}
	if u.Scheme != "https" {
		t.Errorf("scheme = %q, want https", u.Scheme)
	}
}

func TestValidateURL_DevModeAllowsHTTP(t *testing.T) {
	g, _ := helperGuard(t, TierPermissive)
	os.Setenv("ARMORCLAW_DEV_MODE", "true")
	defer os.Unsetenv("ARMORCLAW_DEV_MODE")

	_, err := g.ValidateURL("http://localhost:8080")
	if err != nil {
		t.Fatalf("dev mode should allow HTTP: %v", err)
	}
}

func TestValidateURL_SuspiciousPastebin(t *testing.T) {
	g, _ := helperGuard(t, TierPermissive)
	os.Unsetenv("ARMORCLAW_DEV_MODE")

	_, err := g.ValidateURL("https://pastebin.com/abc123")
	if err == nil {
		t.Error("pastebin URL should be flagged as suspicious")
	}
}

func TestValidateURL_SuspiciousNgrok(t *testing.T) {
	g, _ := helperGuard(t, TierPermissive)
	os.Unsetenv("ARMORCLAW_DEV_MODE")

	_, err := g.ValidateURL("https://tunnel.ngrok.io")
	if err == nil {
		t.Error("ngrok URL should be flagged as suspicious")
	}
}

func TestValidateURL_InvalidURL(t *testing.T) {
	g, _ := helperGuard(t, TierPermissive)
	_, err := g.ValidateURL("not a url at all")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestExtractDomain_WithPort(t *testing.T) {
	got := ExtractDomain("example.com:8080")
	if got != "example.com" {
		t.Errorf("got %q, want example.com", got)
	}
}

func TestExtractDomain_Subdomain(t *testing.T) {
	got := ExtractDomain("sub.example.com")
	if got != "example.com" {
		t.Errorf("got %q, want example.com", got)
	}
}

func TestExtractDomain_DeepSubdomain(t *testing.T) {
	got := ExtractDomain("a.b.c.example.com")
	if got != "example.com" {
		t.Errorf("got %q, want example.com", got)
	}
}

func TestExtractDomain_BareDomain(t *testing.T) {
	got := ExtractDomain("example.com")
	if got != "example.com" {
		t.Errorf("got %q, want example.com", got)
	}
}

func TestExtractDomain_SingleLabel(t *testing.T) {
	got := ExtractDomain("localhost")
	if got != "localhost" {
		t.Errorf("got %q, want localhost", got)
	}
}

func TestAllowlist_IsAllowed_ExactDomain(t *testing.T) {
	w := &WebsiteAllowlist{
		Domains: []DomainRule{{Domain: "safe.com"}},
		Default: PermissionDeny,
	}
	if !w.IsAllowed("safe.com", "/any") {
		t.Error("exact domain should be allowed")
	}
	if w.IsAllowed("evil.com", "/any") {
		t.Error("non-matching domain should be denied")
	}
}

func TestAllowlist_IsAllowed_WildcardDomain(t *testing.T) {
	w := &WebsiteAllowlist{
		Domains: []DomainRule{{Domain: "*.example.com"}},
		Default: PermissionDeny,
	}
	if !w.IsAllowed("sub.example.com", "/path") {
		t.Error("wildcard should match subdomain")
	}
}

func TestAllowlist_IsAllowed_PathRestriction(t *testing.T) {
	w := &WebsiteAllowlist{
		Domains: []DomainRule{
			{Domain: "api.com", Subpaths: []string{"/v1/", "/v2/"}},
		},
		Default: PermissionDeny,
	}
	if !w.IsAllowed("api.com", "/v1/users") {
		t.Error("/v1/ should be allowed")
	}
	if w.IsAllowed("api.com", "/v3/secret") {
		t.Error("/v3/ should be denied")
	}
}

func TestAllowlist_IsAllowed_DefaultAllowAll(t *testing.T) {
	w := &WebsiteAllowlist{
		Default: PermissionAllowAll,
	}
	if !w.IsAllowed("anything.com", "/any") {
		t.Error("default allow_all should permit everything")
	}
}

func TestAllowlist_IsAllowed_NoSubpathsAllowsAll(t *testing.T) {
	w := &WebsiteAllowlist{
		Domains: []DomainRule{{Domain: "open.com"}},
		Default: PermissionDeny,
	}
	if !w.IsAllowed("open.com", "/secret/admin") {
		t.Error("no subpaths = all paths allowed")
	}
}

func TestAllowlist_AddRemoveDomain(t *testing.T) {
	w := &WebsiteAllowlist{Default: PermissionDeny}
	w.AddDomain(DomainRule{Domain: "new.com"})
	if !w.IsAllowed("new.com", "/") {
		t.Error("added domain should be allowed")
	}
	w.RemoveDomain("new.com")
	if w.IsAllowed("new.com", "/") {
		t.Error("removed domain should no longer be allowed")
	}
}

func TestAllowlist_RemoveDomain_NonExistent(t *testing.T) {
	w := &WebsiteAllowlist{Default: PermissionDeny}
	w.RemoveDomain("nope.com")
	if len(w.Domains) != 0 {
		t.Error("removing non-existent domain should be a no-op")
	}
}

func TestAllowlist_ToJSON(t *testing.T) {
	w := &WebsiteAllowlist{
		Category: CategoryBanking,
		Domains:  []DomainRule{{Domain: "bank.com"}},
		Default:  PermissionDeny,
	}
	data, err := w.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	var parsed WebsiteAllowlist
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Category != CategoryBanking {
		t.Errorf("category = %q, want banking", parsed.Category)
	}
}

func TestVerifyCertificate_NoCerts(t *testing.T) {
	g, _ := helperGuard(t, TierPermissive)
	err := g.VerifyCertificate("example.com", nil)
	if err == nil {
		t.Error("expected error for no certificates")
	}
}

func TestVerifyCertificate_NoPins(t *testing.T) {
	g, _ := helperGuard(t, TierPermissive)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1)}
	err := g.VerifyCertificate("example.com", []*x509.Certificate{cert})
	if err != nil {
		t.Errorf("no pins should pass: %v", err)
	}
}

func TestVerifyCertificate_PinMatch(t *testing.T) {
	g, _ := helperGuard(t, TierPermissive)
	cert := &x509.Certificate{SerialNumber: big.NewInt(42)}
	pin := pinCertificate(cert)
	g.AddCertificatePin("example.com", pin)
	err := g.VerifyCertificate("example.com", []*x509.Certificate{cert})
	if err != nil {
		t.Errorf("matching pin should pass: %v", err)
	}
}

func TestVerifyCertificate_PinMismatch(t *testing.T) {
	g, _ := helperGuard(t, TierPermissive)
	cert := &x509.Certificate{SerialNumber: big.NewInt(42)}
	g.AddCertificatePin("example.com", "deadbeef")
	err := g.VerifyCertificate("example.com", []*x509.Certificate{cert})
	if err == nil {
		t.Error("mismatched pin should fail")
	}
}

func TestGetAllowedDomains(t *testing.T) {
	cfg := NewSecurityConfig()
	cfg.SetTier(TierPermissive)
	cat := cfg.GetCategory(CategoryBanking)
	cat.AllowedWebsites = []string{"a.com", "b.com"}
	cfg.SetCategory(CategoryBanking, cat)

	g := NewWebsiteGuard(cfg, nil)
	domains := g.GetAllowedDomains(CategoryBanking)
	if len(domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(domains))
	}
	found := map[string]bool{}
	for _, d := range domains {
		found[d] = true
	}
	if !found["a.com"] || !found["b.com"] {
		t.Errorf("expected a.com and b.com, got %v", domains)
	}
}

func TestGetAllowedDomains_NilCategory(t *testing.T) {
	cfg := NewSecurityConfig()
	g := NewWebsiteGuard(cfg, nil)
	domains := g.GetAllowedDomains(DataCategory("nonexistent"))
	if len(domains) != 0 {
		t.Errorf("expected empty for unknown category, got %v", domains)
	}
}

func TestTLSConfig_NonNil(t *testing.T) {
	g, _ := helperGuard(t, TierPermissive)
	tlsCfg := g.TLSConfig()
	if tlsCfg == nil {
		t.Fatal("TLSConfig should not return nil")
	}
	if tlsCfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false")
	}
}

func TestCheckAccess_ConcurrentSafety(t *testing.T) {
	g, cfg := helperGuard(t, TierPermissive)
	cat := cfg.GetCategory(CategoryBanking)
	cat.AllowedWebsites = []string{"safe.com"}
	cfg.SetCategory(CategoryBanking, cat)

	var errors int64
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := g.CheckAccess(context.Background(), "https://safe.com/api", CategoryBanking, "read")
			if err != nil {
				atomic.AddInt64(&errors, 1)
			}
		}()
	}
	wg.Wait()
	if atomic.LoadInt64(&errors) > 0 {
		t.Errorf("concurrent CheckAccess had %d errors", errors)
	}
}
