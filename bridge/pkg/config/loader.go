// Package config provides configuration loading and management for ArmorClaw bridge.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/armorclaw/bridge/pkg/logger"
)

// Load loads configuration from a file path
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// If path is empty, search for default config files
	if path == "" {
		for _, p := range ConfigPaths() {
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}
	}

	// If no config file found, warn and return defaults
	if path == "" {
		log := logger.Global().WithComponent("config")
		log.Warn("No configuration file found in default locations")
		log.Warn("Default locations checked:")
		for _, p := range ConfigPaths() {
			log.Warn("  - " + p)
		}
		log.Warn("Using default configuration")
		log.Warn("Create a config with: armorclaw-bridge init")
		return cfg, nil
	}

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse TOML using BurntSushi/toml library
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.Server.Auth == "none" {
		log := logger.Global().WithComponent("config")
		log.Warn("WARNING: auth: none is deprecated and will be removed in future versions. Use token-based authentication for production deployments")
	}

	// Apply environment variable overrides
	if err := applyEnvOverrides(cfg); err != nil {
		return nil, fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// LoadOrDie loads configuration or exits on error
func LoadOrDie(path string) *Config {
	cfg, err := Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return cfg
}

// applyEnvOverrides applies environment variable overrides to the configuration
func applyEnvOverrides(cfg *Config) error {
	// Server overrides
	if v := os.Getenv("ARMORCLAW_SOCKET"); v != "" {
		cfg.Server.SocketPath = v
	}
	if v := os.Getenv("ARMORCLAW_PID_FILE"); v != "" {
		cfg.Server.PidFile = v
	}
	if v := os.Getenv("ARMORCLAW_DAEMONIZE"); v != "" {
		cfg.Server.Daemonize = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_SERVER_MODE"); v != "" {
		cfg.Server.Mode = v
	}
	if v := os.Getenv("ARMORCLAW_RPC_TRANSPORT"); v != "" {
		cfg.Server.RPCTransport = v
	}
	if v := os.Getenv("ARMORCLAW_LISTEN_ADDR"); v != "" {
		cfg.Server.ListenAddr = v
	}
	if v := os.Getenv("ARMORCLAW_PUBLIC_BASE_URL"); v != "" {
		cfg.Server.PublicBaseURL = v
	}
	if v := os.Getenv("ARMORCLAW_ADMIN_TOKEN"); v != "" {
		cfg.Server.AdminToken = v
	}
	if v := os.Getenv("ARMORCLAW_AUTH"); v != "" {
		cfg.Server.Auth = v
	}

	// Keystore overrides
	if v := os.Getenv("ARMORCLAW_KEYSTORE_DB"); v != "" {
		cfg.Keystore.DBPath = v
	}
	if v := os.Getenv("ARMORCLAW_MASTER_KEY"); v != "" {
		cfg.Keystore.MasterKey = v
	}

	// Matrix overrides
	if v := os.Getenv("ARMORCLAW_MATRIX_ENABLED"); v != "" {
		cfg.Matrix.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_MATRIX_HOMESERVER"); v != "" {
		cfg.Matrix.HomeserverURL = v
	}
	if v := os.Getenv("ARMORCLAW_MATRIX_USERNAME"); v != "" {
		cfg.Matrix.Username = v
	}
	if v := os.Getenv("ARMORCLAW_MATRIX_PASSWORD"); v != "" {
		cfg.Matrix.Password = v
	}
	if v := os.Getenv("ARMORCLAW_MATRIX_DEVICE_ID"); v != "" {
		cfg.Matrix.DeviceID = v
	}

	if v := os.Getenv("ARMORCLAW_MATRIX_SYNC_INTERVAL"); v != "" {
		var interval int
		if _, err := fmt.Sscanf(v, "%d", &interval); err == nil {
			cfg.Matrix.SyncInterval = interval
		}
	}

	// E2EE kill switch override
	if v := os.Getenv("ARMORCLAW_E2EE_ENABLED"); v != "" {
		cfg.Matrix.E2EE.Enabled = v == "true" || v == "1"
	}

	// WebRTC overrides
	if v := os.Getenv("ARMORCLAW_WEBRTC_STUN_SERVER"); v != "" {
		// Override the default STUN server
		if len(cfg.WebRTC.ICEServers) > 0 {
			cfg.WebRTC.ICEServers[0].URLs = []string{v}
		}
	}
	if v := os.Getenv("ARMORCLAW_WEBRTC_TURN_URL"); v != "" {
		cfg.WebRTC.TURNServerURL = v
	}
	if v := os.Getenv("ARMORCLAW_WEBRTC_TURN_SECRET"); v != "" {
		cfg.WebRTC.TURNSharedSecret = v
	}

	// Logging overrides
	if v := os.Getenv("ARMORCLAW_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("ARMORCLAW_LOG_FORMAT"); v != "" {
		cfg.Logging.Format = v
	}
	if v := os.Getenv("ARMORCLAW_LOG_OUTPUT"); v != "" {
		cfg.Logging.Output = v
	}
	if v := os.Getenv("ARMORCLAW_LOG_FILE"); v != "" {
		cfg.Logging.File = v
	}

	// Error system overrides
	if v := os.Getenv("ARMORCLAW_ERRORS_ENABLED"); v != "" {
		cfg.ErrorSystem.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_ERRORS_STORE_ENABLED"); v != "" {
		cfg.ErrorSystem.StoreEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_ERRORS_NOTIFY_ENABLED"); v != "" {
		cfg.ErrorSystem.NotifyEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_ERRORS_STORE_PATH"); v != "" {
		cfg.ErrorSystem.StorePath = v
	}
	if v := os.Getenv("ARMORCLAW_ERRORS_RETENTION_DAYS"); v != "" {
		var days int
		if _, err := fmt.Sscanf(v, "%d", &days); err == nil {
			cfg.ErrorSystem.RetentionDays = days
		}
	}
	if v := os.Getenv("ARMORCLAW_ERRORS_RATE_LIMIT_WINDOW"); v != "" {
		cfg.ErrorSystem.RateLimitWindow = v
	}
	if v := os.Getenv("ARMORCLAW_ERRORS_RETENTION_PERIOD"); v != "" {
		cfg.ErrorSystem.RetentionPeriod = v
	}
	if v := os.Getenv("ARMORCLAW_ERRORS_ADMIN_MXID"); v != "" {
		cfg.ErrorSystem.AdminMXID = v
	}
	if v := os.Getenv("ARMORCLAW_ERRORS_SETUP_USER_MXID"); v != "" {
		cfg.ErrorSystem.SetupUserMXID = v
	}
	if v := os.Getenv("ARMORCLAW_ERRORS_ADMIN_ROOM_ID"); v != "" {
		cfg.ErrorSystem.AdminRoomID = v
	}

	// Compliance/PII scrubbing overrides
	if v := os.Getenv("ARMORCLAW_COMPLIANCE_ENABLED"); v != "" {
		cfg.Compliance.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_COMPLIANCE_STREAMING"); v != "" {
		cfg.Compliance.StreamingMode = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_COMPLIANCE_QUARANTINE"); v != "" {
		cfg.Compliance.QuarantineEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_COMPLIANCE_NOTIFY_QUARANTINE"); v != "" {
		cfg.Compliance.NotifyOnQuarantine = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_COMPLIANCE_AUDIT"); v != "" {
		cfg.Compliance.AuditEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_COMPLIANCE_AUDIT_DAYS"); v != "" {
		var days int
		if _, err := fmt.Sscanf(v, "%d", &days); err == nil {
			cfg.Compliance.AuditRetentionDays = days
		}
	}
	if v := os.Getenv("ARMORCLAW_COMPLIANCE_TIER"); v != "" {
		cfg.Compliance.Tier = v
	}

	// PII pattern overrides
	if v := os.Getenv("ARMORCLAW_PII_SSN"); v != "" {
		cfg.Compliance.Patterns.SSN = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_PII_CREDIT_CARD"); v != "" {
		cfg.Compliance.Patterns.CreditCard = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_PII_MEDICAL_RECORD"); v != "" {
		cfg.Compliance.Patterns.MedicalRecord = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_PII_HEALTH_PLAN"); v != "" {
		cfg.Compliance.Patterns.HealthPlan = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_PII_DEVICE_ID"); v != "" {
		cfg.Compliance.Patterns.DeviceID = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_PII_BIOMETRIC"); v != "" {
		cfg.Compliance.Patterns.Biometric = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_PII_LAB_RESULT"); v != "" {
		cfg.Compliance.Patterns.LabResult = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_PII_DIAGNOSIS"); v != "" {
		cfg.Compliance.Patterns.Diagnosis = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_PII_PRESCRIPTION"); v != "" {
		cfg.Compliance.Patterns.Prescription = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_PII_EMAIL"); v != "" {
		cfg.Compliance.Patterns.Email = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_PII_PHONE"); v != "" {
		cfg.Compliance.Patterns.Phone = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_PII_IP"); v != "" {
		cfg.Compliance.Patterns.IPAddress = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_PII_API_TOKEN"); v != "" {
		cfg.Compliance.Patterns.APIToken = v == "true" || v == "1"
	}

	if v := os.Getenv("ARMORCLAW_HTTP_ENABLED"); v != "" {
		cfg.HTTP.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("ARMORCLAW_HTTP_HOSTNAME"); v != "" {
		cfg.HTTP.Hostname = v
	}
	if v := os.Getenv("ARMORCLAW_HTTP_PORT"); v != "" {
		var port int
		if _, err := fmt.Sscanf(v, "%d", &port); err == nil {
			cfg.HTTP.Port = port
		}
	}
	if v := os.Getenv("ARMORCLAW_HTTP_CERT_DIR"); v != "" {
		cfg.HTTP.CertDir = v
	}

	return nil
}

// Save saves the configuration to a file
func Save(cfg *Config, path string) error {
	// Validate before saving
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid configuration: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Normalize paths for TOML compatibility (forward slashes, no backslashes)
	// This fixes Windows path parsing issues where \U is interpreted as Unicode escape
	cfgCopy := *cfg // Make a shallow copy
	cfgCopy.Keystore.DBPath = filepath.ToSlash(cfg.Keystore.DBPath)
	cfgCopy.Server.SocketPath = filepath.ToSlash(cfg.Server.SocketPath)
	if cfgCopy.Server.PidFile != "" {
		cfgCopy.Server.PidFile = filepath.ToSlash(cfgCopy.Server.PidFile)
	}
	if cfg.Matrix.DeviceID != "" {
		cfgCopy.Matrix.DeviceID = filepath.ToSlash(cfgCopy.Matrix.DeviceID)
	}

	// Marshal to TOML using BurntSushi/toml library
	data, err := toml.Marshal(&cfgCopy)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GenerateExampleConfig generates an example configuration file
func GenerateExampleConfig(path string) error {
	cfg := DefaultConfig()

	// Add example values
	cfg.Matrix.Enabled = true
	cfg.Matrix.HomeserverURL = "https://matrix.example.com"
	cfg.Matrix.Username = "bridge-bot"
	cfg.Matrix.Password = "change-me"
	cfg.Logging.Level = "info"

	return Save(cfg, path)
}
