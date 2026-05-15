package security

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// AllCategories
// ---------------------------------------------------------------------------

func TestAllCategories_Count(t *testing.T) {
	cats := AllCategories()
	if len(cats) != 8 {
		t.Fatalf("expected 8 categories, got %d", len(cats))
	}
}

func TestAllCategories_AllHaveRequiredFields(t *testing.T) {
	for _, c := range AllCategories() {
		if c.Name == "" {
			t.Error("category with empty Name")
		}
		if c.DisplayName == "" {
			t.Errorf("category %s missing DisplayName", c.Name)
		}
		if c.Description == "" {
			t.Errorf("category %s missing Description", c.Name)
		}
		if len(c.Examples) == 0 {
			t.Errorf("category %s has no Examples", c.Name)
		}
		switch c.RiskLevel {
		case "high", "medium", "low":
			// ok
		default:
			t.Errorf("category %s has unexpected RiskLevel %q", c.Name, c.RiskLevel)
		}
	}
}

func TestAllCategories_KnownConstants(t *testing.T) {
	names := map[DataCategory]bool{}
	for _, c := range AllCategories() {
		names[c.Name] = true
	}
	for _, want := range []DataCategory{
		CategoryBanking, CategoryPII, CategoryMedical, CategoryResidential,
		CategoryNetwork, CategoryIdentity, CategoryLocation, CategoryCredentials,
	} {
		if !names[want] {
			t.Errorf("missing category %q", want)
		}
	}
}

// ---------------------------------------------------------------------------
// NewSecurityConfig / defaults
// ---------------------------------------------------------------------------

func TestNewSecurityConfigDefaults(t *testing.T) {
	cfg := NewSecurityConfig()
	if cfg.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", cfg.Version)
	}
	if cfg.Tier != TierBalanced {
		t.Errorf("expected tier %s, got %s", TierBalanced, cfg.Tier)
	}
	if cfg.Categories == nil {
		t.Error("Categories map is nil")
	}
	if cfg.Adapters == nil {
		t.Error("Adapters map is nil")
	}
	if cfg.Websites == nil {
		t.Error("Websites map is nil")
	}
	if cfg.Skills == nil {
		t.Error("Skills map is nil")
	}
}

// ---------------------------------------------------------------------------
// SetTier variants
// ---------------------------------------------------------------------------

func TestSetTierParanoid(t *testing.T) {
	cfg := NewSecurityConfig()
	if err := cfg.SetTier(TierParanoid); err != nil {
		// SetTier tries to save when configFile is set, but NewSecurityConfig leaves it empty => nil save
		t.Fatalf("SetTier paranoid: %v", err)
	}
	for _, cat := range AllCategories() {
		cc := cfg.GetCategory(cat.Name)
		if cc.Permission != PermissionDeny {
			t.Errorf("paranoid: category %s should be deny, got %s", cat.Name, cc.Permission)
		}
		if !cc.RequiresApproval {
			t.Errorf("paranoid: category %s should require approval", cat.Name)
		}
		if cc.AuditLevel != AuditVerbose {
			t.Errorf("paranoid: category %s should have verbose audit, got %s", cat.Name, cc.AuditLevel)
		}
	}
}

func TestSetTierOpen(t *testing.T) {
	cfg := NewSecurityConfig()
	if err := cfg.SetTier(TierOpen); err != nil {
		t.Fatalf("SetTier open: %v", err)
	}
	for _, cat := range AllCategories() {
		cc := cfg.GetCategory(cat.Name)
		if cc.Permission != PermissionAllowAll {
			t.Errorf("open: category %s should be allow_all, got %s", cat.Name, cc.Permission)
		}
		if cc.RequiresApproval {
			t.Errorf("open: category %s should not require approval", cat.Name)
		}
	}
}

func TestSetTierBalanced(t *testing.T) {
	cfg := NewSecurityConfig()
	if err := cfg.SetTier(TierBalanced); err != nil {
		t.Fatalf("SetTier balanced: %v", err)
	}
	// Low-risk (location) should be allowed
	loc := cfg.GetCategory(CategoryLocation)
	if loc.Permission != PermissionAllow {
		t.Errorf("balanced: location should be allow, got %s", loc.Permission)
	}
	// High-risk categories should be denied
	bank := cfg.GetCategory(CategoryBanking)
	if bank.Permission != PermissionDeny {
		t.Errorf("balanced: banking should be deny, got %s", bank.Permission)
	}
}

func TestSetTierPermissive(t *testing.T) {
	cfg := NewSecurityConfig()
	if err := cfg.SetTier(TierPermissive); err != nil {
		t.Fatalf("SetTier permissive: %v", err)
	}
	for _, cat := range AllCategories() {
		cc := cfg.GetCategory(cat.Name)
		if cc.Permission != PermissionAllow {
			t.Errorf("permissive: category %s should be allow, got %s", cat.Name, cc.Permission)
		}
	}
}

// ---------------------------------------------------------------------------
// CategoryConfig.IsAllowed
// ---------------------------------------------------------------------------

func TestCategoryConfig_IsAllowed(t *testing.T) {
	tests := []struct {
		name       string
		permission PermissionLevel
		want       bool
	}{
		{"deny", PermissionDeny, false},
		{"allow", PermissionAllow, true},
		{"allow_all", PermissionAllowAll, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CategoryConfig{Permission: tt.permission}
			if got := c.IsAllowed(); got != tt.want {
				t.Errorf("IsAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CategoryConfig.IsWebsiteAllowed — blocked takes priority
// ---------------------------------------------------------------------------

func TestIsWebsiteAllowed_BlockedTakesPriority(t *testing.T) {
	c := &CategoryConfig{
		Permission:      PermissionAllow,
		AllowedWebsites: []string{"evil.com"},
		BlockedWebsites: []string{"evil.com"},
	}
	if c.IsWebsiteAllowed("evil.com") {
		t.Error("blocked domain should not be allowed even if in allowlist")
	}
}

func TestIsWebsiteAllowed_AllowlistMatch(t *testing.T) {
	c := &CategoryConfig{
		Permission:      PermissionAllow,
		AllowedWebsites: []string{"safe.com", "*.example.com"},
	}
	if !c.IsWebsiteAllowed("safe.com") {
		t.Error("exact allowlist match should be allowed")
	}
}

func TestIsWebsiteAllowed_Wildcard(t *testing.T) {
	c := &CategoryConfig{
		Permission:      PermissionAllow,
		AllowedWebsites: []string{"*.example.com"},
	}
	if !c.IsWebsiteAllowed("sub.example.com") {
		t.Error("wildcard *.example.com should match sub.example.com")
	}
}

func TestIsWebsiteAllowed_EmptyAllowlist(t *testing.T) {
	c := &CategoryConfig{
		Permission:      PermissionAllow,
		AllowedWebsites: nil,
	}
	if c.IsWebsiteAllowed("any.com") {
		t.Error("empty allowlist should deny by default")
	}
}

func TestIsWebsiteAllowed_PermissionAllowAll(t *testing.T) {
	c := &CategoryConfig{
		Permission:      PermissionAllowAll,
		BlockedWebsites: []string{"evil.com"},
	}
	// allow_all bypasses blocked list too (based on implementation order)
	if !c.IsWebsiteAllowed("evil.com") {
		t.Error("PermissionAllowAll should bypass all checks")
	}
}

// ---------------------------------------------------------------------------
// CategoryConfig.IsAdapterAllowed
// ---------------------------------------------------------------------------

func TestIsAdapterAllowed(t *testing.T) {
	c := &CategoryConfig{
		Permission:      PermissionAllow,
		AllowedAdapters: []string{"matrix", "web"},
	}
	if !c.IsAdapterAllowed("matrix") {
		t.Error("matrix adapter should be allowed")
	}
	if c.IsAdapterAllowed("slack") {
		t.Error("slack adapter should not be allowed")
	}
}

func TestIsAdapterAllowed_AllowAll(t *testing.T) {
	c := &CategoryConfig{Permission: PermissionAllowAll}
	if !c.IsAdapterAllowed("anything") {
		t.Error("allow_all should permit any adapter")
	}
}

// ---------------------------------------------------------------------------
// CategoryConfig.IsSubsetAllowed
// ---------------------------------------------------------------------------

func TestIsSubsetAllowed(t *testing.T) {
	c := &CategoryConfig{
		Permission: PermissionAllow,
		DataSubsets: map[string]SubsetConfig{
			"partial": {Permission: PermissionAllow},
			"denied":  {Permission: PermissionDeny},
		},
	}
	if !c.IsSubsetAllowed("partial") {
		t.Error("allowed subset should return true")
	}
	if c.IsSubsetAllowed("denied") {
		t.Error("denied subset should return false")
	}
	// unknown subset falls back to category permission
	if !c.IsSubsetAllowed("unknown") {
		t.Error("unknown subset should fall back to category permission (allow)")
	}
}

func TestIsSubsetAllowed_NilSubsets(t *testing.T) {
	c := &CategoryConfig{
		Permission:  PermissionAllow,
		DataSubsets: nil,
	}
	if !c.IsSubsetAllowed("anything") {
		t.Error("nil subsets with allow should permit")
	}
}

// ---------------------------------------------------------------------------
// matchDomain (unexported helper)
// ---------------------------------------------------------------------------

func TestMatchDomain(t *testing.T) {
	tests := []struct {
		pattern string
		domain  string
		want    bool
	}{
		{"*", "anything.com", true},
		{"exact.com", "exact.com", true},
		{"exact.com", "other.com", false},
		{"*.example.com", "sub.example.com", true},
		{"*.example.com", "example.com", false},
		{"*.example.com", "deep.sub.example.com", true},
	}
	for _, tt := range tests {
		got := matchDomain(tt.pattern, tt.domain)
		if got != tt.want {
			t.Errorf("matchDomain(%q, %q) = %v, want %v", tt.pattern, tt.domain, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// SecurityConfig.IsDataAllowed
// ---------------------------------------------------------------------------

func TestIsDataAllowed_DeniedCategory(t *testing.T) {
	cfg := NewSecurityConfig()
	cfg.SetTier(TierParanoid) // deny all
	err := cfg.IsDataAllowed(CategoryBanking, DataUsageContext{Approved: true})
	if err == nil {
		t.Error("expected error for denied category")
	}
}

func TestIsDataAllowed_RequiresApproval(t *testing.T) {
	cfg := NewSecurityConfig()
	cfg.SetTier(TierBalanced)
	// banking is high-risk, requires approval
	err := cfg.IsDataAllowed(CategoryBanking, DataUsageContext{Approved: false})
	if err == nil {
		t.Error("expected error when approval required but not given")
	}
}

func TestIsDataAllowed_WebsiteBlocked(t *testing.T) {
	cfg := NewSecurityConfig()
	cfg.SetTier(TierPermissive) // allow all categories
	cat := cfg.GetCategory(CategoryBanking)
	cat.AllowedWebsites = []string{"safe.com"}
	cat.BlockedWebsites = []string{"evil.com"}
	cfg.SetCategory(CategoryBanking, cat)

	err := cfg.IsDataAllowed(CategoryBanking, DataUsageContext{Website: "evil.com", Approved: true})
	if err == nil {
		t.Error("expected error for blocked website")
	}
}

// ---------------------------------------------------------------------------
// Clone
// ---------------------------------------------------------------------------

func TestSecurityConfig_Clone(t *testing.T) {
	cfg := NewSecurityConfig()
	cfg.SetTier(TierPermissive)
	cfg.AdminID = "admin-1"

	clone := cfg.Clone()
	if clone.AdminID != "admin-1" {
		t.Errorf("clone admin mismatch: %s", clone.AdminID)
	}
	if clone.Tier != TierPermissive {
		t.Errorf("clone tier mismatch: %s", clone.Tier)
	}

	// Mutate clone, verify original unchanged
	clone.AdminID = "admin-2"
	if cfg.AdminID != "admin-1" {
		t.Error("mutating clone affected original")
	}
}

func TestSecurityConfig_Clone_DeepCopyCategories(t *testing.T) {
	cfg := NewSecurityConfig()
	cfg.SetTier(TierPermissive)

	clone := cfg.Clone()
	cat := clone.GetCategory(CategoryBanking)
	cat.AllowedWebsites = append(cat.AllowedWebsites, "injected.com")

	orig := cfg.GetCategory(CategoryBanking)
	for _, w := range orig.AllowedWebsites {
		if w == "injected.com" {
			t.Error("mutating cloned category affected original (not a deep copy)")
		}
	}
}

// ---------------------------------------------------------------------------
// LoadSecurityConfig
// ---------------------------------------------------------------------------

func TestLoadSecurityConfig_MissingFile(t *testing.T) {
	cfg, err := LoadSecurityConfig("/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Tier != TierBalanced {
		t.Errorf("expected default balanced tier, got %s", cfg.Tier)
	}
}

func TestLoadSecurityConfig_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := []byte(`{"version":"2.0.0","tier":"strict"}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := LoadSecurityConfig(path)
	if err != nil {
		t.Fatalf("LoadSecurityConfig: %v", err)
	}
	if cfg.Version != "2.0.0" {
		t.Errorf("expected version 2.0.0, got %s", cfg.Version)
	}
	if cfg.Tier != TierStrict {
		t.Errorf("expected strict tier, got %s", cfg.Tier)
	}
}

func TestLoadSecurityConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadSecurityConfig(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// ToJSON
// ---------------------------------------------------------------------------

func TestToJSON(t *testing.T) {
	cfg := NewSecurityConfig()
	cfg.SetTier(TierOpen)
	data, err := cfg.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("ToJSON returned empty data")
	}
	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("ToJSON produced invalid JSON: %v", err)
	}
	if parsed["tier"] != "open" {
		t.Errorf("expected tier 'open', got %v", parsed["tier"])
	}
}

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------

func TestSummary(t *testing.T) {
	cfg := NewSecurityConfig()
	cfg.SetTier(TierOpen)
	s := cfg.Summary()
	if s["tier"] != TierOpen {
		t.Errorf("expected tier open, got %v", s["tier"])
	}
}

// ---------------------------------------------------------------------------
// Adapter / Skill helpers
// ---------------------------------------------------------------------------

func TestGetAdapter_Default(t *testing.T) {
	cfg := NewSecurityConfig()
	a := cfg.GetAdapter("nonexistent")
	if a.Enabled {
		t.Error("default adapter should be disabled")
	}
	if !a.RequireApproval {
		t.Error("default adapter should require approval")
	}
}

func TestSetAndGetAdapter(t *testing.T) {
	cfg := NewSecurityConfig()
	ac := AdapterConfig{Enabled: true, RateLimit: 100}
	if err := cfg.SetAdapter("matrix", ac); err != nil {
		t.Fatalf("SetAdapter: %v", err)
	}
	got := cfg.GetAdapter("matrix")
	if !got.Enabled {
		t.Error("adapter should be enabled")
	}
	if got.RateLimit != 100 {
		t.Errorf("expected rate limit 100, got %d", got.RateLimit)
	}
}

func TestGetSkill_Default(t *testing.T) {
	cfg := NewSecurityConfig()
	s := cfg.GetSkill("nonexistent")
	if s.Enabled {
		t.Error("default skill should be disabled")
	}
}

func TestSetAndGetSkill(t *testing.T) {
	cfg := NewSecurityConfig()
	sc := SkillConfig{Enabled: true, RequiresApproval: false}
	if err := cfg.SetSkill("browse", sc); err != nil {
		t.Fatalf("SetSkill: %v", err)
	}
	got := cfg.GetSkill("browse")
	if !got.Enabled {
		t.Error("skill should be enabled")
	}
}

// ---------------------------------------------------------------------------
// SetCategory persists via save() — no configFile means no-op save
// ---------------------------------------------------------------------------

func TestSetCategory_NoConfigFile(t *testing.T) {
	cfg := NewSecurityConfig()
	cc := &CategoryConfig{Permission: PermissionAllow}
	if err := cfg.SetCategory(CategoryBanking, cc); err != nil {
		t.Fatalf("SetCategory without configFile: %v", err)
	}
	got := cfg.GetCategory(CategoryBanking)
	if got.Permission != PermissionAllow {
		t.Errorf("expected allow, got %s", got.Permission)
	}
}

// ---------------------------------------------------------------------------
// countEnabledAdapters / countEnabledSkills (exercised via Summary)
// ---------------------------------------------------------------------------

func TestCountEnabledAdapters(t *testing.T) {
	m := map[string]AdapterConfig{
		"a": {Enabled: true},
		"b": {Enabled: false},
		"c": {Enabled: true},
	}
	if n := countEnabledAdapters(m); n != 2 {
		t.Errorf("expected 2 enabled adapters, got %d", n)
	}
}

func TestCountEnabledSkills(t *testing.T) {
	m := map[string]SkillConfig{
		"x": {Enabled: true},
		"y": {Enabled: false},
	}
	if n := countEnabledSkills(m); n != 1 {
		t.Errorf("expected 1 enabled skill, got %d", n)
	}
}
