package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFreshInstallE2EEEnabled validates that a brand-new installation
// (no config file, no keystore directory) defaults E2EE to enabled.
func TestFreshInstallE2EEEnabled(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	// Skip if system has /var/lib/armorclaw (not a fresh-machine scenario)
	if _, err := os.Stat("/var/lib/armorclaw"); err == nil {
		t.Skip("skipping: /var/lib/armorclaw exists on this machine")
	}

	// Ensure no config.toml in CWD either
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err == nil {
		defer func() { _ = os.Chdir(origDir) }()
	}

	cfg := DefaultConfig()
	if !cfg.Matrix.E2EE.Enabled {
		t.Fatal("expected E2EE.Enabled = true for fresh install (no config, no keystore)")
	}
	if !cfg.IsE2EEEnabled() {
		t.Fatal("expected IsE2EEEnabled() = true for fresh install")
	}
}

// TestLegacyInstallE2EEDisabled validates that when a config file already
// exists, E2EE remains off by default (preserving legacy behavior).
func TestLegacyInstallE2EEDisabled(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	// Create config file to simulate legacy install
	configDir := filepath.Join(tmpDir, ".armorclaw")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configFile, []byte("# legacy config"), 0644); err != nil {
		t.Fatal(err)
	}

	// Skip if system has /var/lib/armorclaw (would make it non-fresh regardless)
	if _, err := os.Stat("/var/lib/armorclaw"); err == nil {
		t.Skip("skipping: /var/lib/armorclaw exists on this machine")
	}

	cfg := DefaultConfig()
	if cfg.Matrix.E2EE.Enabled {
		t.Fatal("expected E2EE.Enabled = false for legacy install (config file exists)")
	}
	if cfg.IsE2EEEnabled() {
		t.Fatal("expected IsE2EEEnabled() = false for legacy install")
	}
}

// TestFreshInstallConfigPropagatesE2EE verifies the full DefaultConfig pipeline:
// IsFreshInstall() returns true → E2EE.Enabled is set → IsE2EEEnabled() reflects it.
// This is the config-side equivalent of the RPC server atomic store validation.
func TestFreshInstallConfigPropagatesE2EE(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	if _, err := os.Stat("/var/lib/armorclaw"); err == nil {
		t.Skip("skipping: /var/lib/armorclaw exists on this machine")
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err == nil {
		defer func() { _ = os.Chdir(origDir) }()
	}

	// Step 1: Verify fresh install detection
	if !IsFreshInstall() {
		t.Fatal("expected IsFreshInstall() = true")
	}

	// Step 2: Verify DefaultConfig propagates E2EE
	cfg := DefaultConfig()
	if !cfg.Matrix.E2EE.Enabled {
		t.Fatal("expected DefaultConfig().Matrix.E2EE.Enabled = true")
	}

	// Step 3: Verify accessor method
	if !cfg.IsE2EEEnabled() {
		t.Fatal("expected IsE2EEEnabled() = true")
	}

	// Step 4: Verify validation does not break E2EE
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() should not fail with E2EE enabled: %v", err)
	}

	// Step 5: E2EE should still be true after validation
	if !cfg.IsE2EEEnabled() {
		t.Fatal("expected IsE2EEEnabled() = true after Validate()")
	}
}
