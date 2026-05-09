package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsFreshInstall_TrueWhenNothingExists(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	if _, err := os.Stat("/var/lib/armorclaw"); err == nil {
		t.Skip("skipping: /var/lib/armorclaw exists on this machine")
	}

	if !IsFreshInstall() {
		t.Fatal("expected IsFreshInstall() = true when nothing exists")
	}
}

func TestIsFreshInstall_FalseWhenConfigFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	configDir := filepath.Join(tmpDir, ".armorclaw")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("# test"), 0644); err != nil {
		t.Fatal(err)
	}

	if IsFreshInstall() {
		t.Fatal("expected IsFreshInstall() = false when config file exists")
	}
}

func TestIsFreshInstall_FalseWhenKeystoreDirExists(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	keystoreDir := "/var/lib/armorclaw"
	if err := os.MkdirAll(keystoreDir, 0755); err != nil {
		t.Skipf("skipping: cannot create %s: %v", keystoreDir, err)
	}
	defer func() { _ = os.RemoveAll(keystoreDir) }()

	if IsFreshInstall() {
		t.Fatal("expected IsFreshInstall() = false when keystore dir exists")
	}
}
