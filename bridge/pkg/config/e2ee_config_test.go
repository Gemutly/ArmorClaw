package config

import (
	"os"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestE2EEConfigDefaultFalse(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Matrix.E2EE.Enabled {
		t.Error("E2EE should default to false")
	}
	if cfg.IsE2EEEnabled() {
		t.Error("IsE2EEEnabled() should return false for default config")
	}
}

func TestE2EEConfigEnvOverride(t *testing.T) {
	t.Setenv("ARMORCLAW_E2EE_ENABLED", "true")
	cfg := DefaultConfig()
	applyEnvOverrides(cfg)

	if !cfg.Matrix.E2EE.Enabled {
		t.Error("E2EE should be enabled when ARMORCLAW_E2EE_ENABLED=true")
	}
	if !cfg.IsE2EEEnabled() {
		t.Error("IsE2EEEnabled() should return true after env override")
	}
}

func TestE2EEConfigEnvOverrideFalse(t *testing.T) {
	t.Setenv("ARMORCLAW_E2EE_ENABLED", "false")
	cfg := DefaultConfig()
	applyEnvOverrides(cfg)

	if cfg.Matrix.E2EE.Enabled {
		t.Error("E2EE should be disabled when ARMORCLAW_E2EE_ENABLED=false")
	}
}

func TestE2EEConfigEnvOverrideNumeric(t *testing.T) {
	t.Setenv("ARMORCLAW_E2EE_ENABLED", "1")
	cfg := DefaultConfig()
	applyEnvOverrides(cfg)

	if !cfg.Matrix.E2EE.Enabled {
		t.Error("E2EE should be enabled when ARMORCLAW_E2EE_ENABLED=1")
	}

	t.Setenv("ARMORCLAW_E2EE_ENABLED", "0")
	cfg2 := DefaultConfig()
	applyEnvOverrides(cfg2)

	if cfg2.Matrix.E2EE.Enabled {
		t.Error("E2EE should be disabled when ARMORCLAW_E2EE_ENABLED=0")
	}
}

func TestE2EEConfigTOMLParsing(t *testing.T) {
	tomlData := `
[matrix]
enabled = true
homeserver_url = "https://matrix.example.com"

[matrix.e2ee]
enabled = true
`
	cfg := DefaultConfig()
	if _, err := toml.Decode(tomlData, cfg); err != nil {
		t.Fatalf("Failed to parse TOML: %v", err)
	}

	if !cfg.Matrix.E2EE.Enabled {
		t.Error("E2EE should be true from TOML config")
	}
}

func TestE2EEConfigTOMLDefaultOmitted(t *testing.T) {
	tomlData := `
[matrix]
enabled = true
homeserver_url = "https://matrix.example.com"
`
	cfg := DefaultConfig()
	cfg.Matrix.E2EE.Enabled = true
	if _, err := toml.Decode(tomlData, cfg); err != nil {
		t.Fatalf("Failed to parse TOML: %v", err)
	}

	if cfg.Matrix.E2EE.Enabled {
		t.Error("E2EE should be false when [matrix.e2ee] section is omitted")
	}
}

func TestE2EEConfigEnvOverridesTOML(t *testing.T) {
	t.Setenv("ARMORCLAW_E2EE_ENABLED", "true")
	tomlData := `
[matrix]
enabled = true
homeserver_url = "https://matrix.example.com"

[matrix.e2ee]
enabled = false
`
	cfg := DefaultConfig()
	if _, err := toml.Decode(tomlData, cfg); err != nil {
		t.Fatalf("Failed to parse TOML: %v", err)
	}
	applyEnvOverrides(cfg)

	if !cfg.Matrix.E2EE.Enabled {
		t.Error("Env var should override TOML value")
	}
}

func TestE2EEIsE2EEEnabledMethod(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Matrix.E2EE.Enabled = true
	if !cfg.IsE2EEEnabled() {
		t.Error("IsE2EEEnabled() should return true when E2EE.Enabled is true")
	}

	cfg.Matrix.E2EE.Enabled = false
	if cfg.IsE2EEEnabled() {
		t.Error("IsE2EEEnabled() should return false when E2EE.Enabled is false")
	}
}

func TestE2EERuntimeToggle(t *testing.T) {
	cfg := DefaultConfig()
	initial := cfg.Matrix.E2EE.Enabled
	cfg.Matrix.E2EE.Enabled = !initial
	if cfg.Matrix.E2EE.Enabled == initial {
		t.Error("Toggle should change E2EE state")
	}
}

func TestE2EEDoesNotBreakValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Matrix.E2EE.Enabled = true
	if err := cfg.Validate(); err != nil {
		t.Errorf("E2EE enabled should not break validation: %v", err)
	}
}

func TestE2EEEmptyEnvNoEffect(t *testing.T) {
	os.Unsetenv("ARMORCLAW_E2EE_ENABLED")
	cfg := DefaultConfig()
	cfg.Matrix.E2EE.Enabled = false
	applyEnvOverrides(cfg)
	if cfg.Matrix.E2EE.Enabled {
		t.Error("Empty env var should not change E2EE state")
	}

	cfg.Matrix.E2EE.Enabled = true
	applyEnvOverrides(cfg)
	if !cfg.Matrix.E2EE.Enabled {
		t.Error("Empty env var should not change E2EE state to false")
	}
}
