package config

import (
	"os"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestFeatureFlagsDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Features.ZeroTrustKeystore {
		t.Error("ZeroTrustKeystore should default to false")
	}
	if cfg.Features.VoicePipeline != "off" {
		t.Errorf("VoicePipeline should default to 'off', got %s", cfg.Features.VoicePipeline)
	}
	if cfg.Features.MultiTabReplay {
		t.Error("MultiTabReplay should default to false")
	}
	if cfg.Features.E2EEBackup {
		t.Error("E2EEBackup should default to false")
	}
}

func TestFeatureFlagsOffReturnsFalse(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.IsFeatureEnabled("zero_trust_keystore") {
		t.Error("IsFeatureEnabled should return false for zero_trust_keystore when off")
	}
	if cfg.IsFeatureEnabled("voice_pipeline") {
		t.Error("IsFeatureEnabled should return false for voice_pipeline when off")
	}
	if cfg.IsFeatureEnabled("multi_tab_replay") {
		t.Error("IsFeatureEnabled should return false for multi_tab_replay when off")
	}
	if cfg.IsFeatureEnabled("e2ee_backup") {
		t.Error("IsFeatureEnabled should return false for e2ee_backup when off")
	}
}

func TestFeatureFlagsOnReturnsTrue(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Features.ZeroTrustKeystore = true
	cfg.Features.VoicePipeline = "cloud"
	cfg.Features.MultiTabReplay = true
	cfg.Features.E2EEBackup = true

	if !cfg.IsFeatureEnabled("zero_trust_keystore") {
		t.Error("IsFeatureEnabled should return true for zero_trust_keystore when on")
	}
	if !cfg.IsFeatureEnabled("voice_pipeline") {
		t.Error("IsFeatureEnabled should return true for voice_pipeline when cloud")
	}
	if !cfg.IsFeatureEnabled("multi_tab_replay") {
		t.Error("IsFeatureEnabled should return true for multi_tab_replay when on")
	}
	if !cfg.IsFeatureEnabled("e2ee_backup") {
		t.Error("IsFeatureEnabled should return true for e2ee_backup when on")
	}
}

func TestFeatureFlagsUnknownName(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.IsFeatureEnabled("nonexistent_flag") {
		t.Error("IsFeatureEnabled should return false for unknown flag")
	}
}

func TestFeatureFlagsVoicePipelineValidation(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"off is valid", "off", false},
		{"cloud is valid", "cloud", false},
		{"invalid value", "onnx", true},
		{"invalid value 2", "local", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ff := FeatureFlags{VoicePipeline: tt.value}
			err := ff.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFeatureFlagsVoicePipelineOnlyCloudReturnsTrue(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Features.VoicePipeline = "off"

	if cfg.IsFeatureEnabled("voice_pipeline") {
		t.Error("voice_pipeline should return false when set to 'off'")
	}

	cfg.Features.VoicePipeline = "cloud"
	if !cfg.IsFeatureEnabled("voice_pipeline") {
		t.Error("voice_pipeline should return true when set to 'cloud'")
	}
}

func TestFeatureFlagsTOMLParsing(t *testing.T) {
	input := `
[features]
feature_zero_trust_keystore = true
feature_voice_pipeline = "cloud"
feature_multi_tab_replay = true
feature_e2ee_backup = true
`

	cfg := DefaultConfig()
	if _, err := toml.Decode(input, cfg); err != nil {
		t.Fatalf("failed to parse features TOML: %v", err)
	}

	if !cfg.Features.ZeroTrustKeystore {
		t.Error("expected zero_trust_keystore = true from TOML")
	}
	if cfg.Features.VoicePipeline != "cloud" {
		t.Errorf("expected voice_pipeline = 'cloud', got %s", cfg.Features.VoicePipeline)
	}
	if !cfg.Features.MultiTabReplay {
		t.Error("expected multi_tab_replay = true from TOML")
	}
	if !cfg.Features.E2EEBackup {
		t.Error("expected e2ee_backup = true from TOML")
	}
}

func TestFeatureFlagsEnvOverride(t *testing.T) {
	t.Setenv("ARMORCLAW_FEATURE_ZERO_TRUST_KEYSTORE", "true")
	t.Setenv("ARMORCLAW_FEATURE_VOICE_PIPELINE", "cloud")
	t.Setenv("ARMORCLAW_FEATURE_MULTI_TAB_REPLAY", "1")
	t.Setenv("ARMORCLAW_FEATURE_E2EE_BACKUP", "true")

	cfg := DefaultConfig()
	if err := applyEnvOverrides(cfg); err != nil {
		t.Fatalf("applyEnvOverrides failed: %v", err)
	}

	if !cfg.Features.ZeroTrustKeystore {
		t.Error("env override should enable ZeroTrustKeystore")
	}
	if cfg.Features.VoicePipeline != "cloud" {
		t.Errorf("env override should set VoicePipeline to 'cloud', got %s", cfg.Features.VoicePipeline)
	}
	if !cfg.Features.MultiTabReplay {
		t.Error("env override should enable MultiTabReplay with '1'")
	}
	if !cfg.Features.E2EEBackup {
		t.Error("env override should enable E2EEBackup")
	}
}

func TestFeatureFlagsEnvOverrideFalse(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Features.ZeroTrustKeystore = true

	t.Setenv("ARMORCLAW_FEATURE_ZERO_TRUST_KEYSTORE", "false")
	if err := applyEnvOverrides(cfg); err != nil {
		t.Fatalf("applyEnvOverrides failed: %v", err)
	}

	if cfg.Features.ZeroTrustKeystore {
		t.Error("env override 'false' should disable ZeroTrustKeystore")
	}
}

func TestFeatureFlagsValidateInConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Features.VoicePipeline = "invalid"

	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for invalid voice_pipeline value")
	}
}

func TestFeatureFlagsValidateEmptyStringPasses(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Features.VoicePipeline = ""

	err := cfg.Validate()
	if err != nil {
		t.Errorf("empty voice_pipeline should pass validation, got: %v", err)
	}
}

func TestFeatureFlagsEnvClearsOnEmpty(t *testing.T) {
	os.Unsetenv("ARMORCLAW_FEATURE_ZERO_TRUST_KEYSTORE")
	os.Unsetenv("ARMORCLAW_FEATURE_VOICE_PIPELINE")
	os.Unsetenv("ARMORCLAW_FEATURE_MULTI_TAB_REPLAY")
	os.Unsetenv("ARMORCLAW_FEATURE_E2EE_BACKUP")

	cfg := DefaultConfig()
	cfg.Features.ZeroTrustKeystore = true

	if err := applyEnvOverrides(cfg); err != nil {
		t.Fatalf("applyEnvOverrides failed: %v", err)
	}

	if !cfg.Features.ZeroTrustKeystore {
		t.Error("unset env var should not change existing config value")
	}
}
