package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Browser  BrowserConfig  `yaml:"browser"`
	Security SecurityConfig `yaml:"security"`
	Network  NetworkConfig  `yaml:"network"`
	Logging  LoggingConfig  `yaml:"logging"`
	Approval ApprovalConfig `yaml:"approval"`
}

type ServerConfig struct {
	Port         string        `yaml:"port"`
	Host         string        `yaml:"host"`
	ReadTimeout  time.Duration `yaml:"readTimeout"`
	WriteTimeout time.Duration `yaml:"writeTimeout"`
}

type BrowserConfig struct {
	EnginePath    string         `yaml:"enginePath"`
	EnginePort    string         `yaml:"enginePort"`
	HealthCheck   bool           `yaml:"healthCheck"`
	CheckInterval time.Duration  `yaml:"checkInterval"`
	Watchdog      WatchdogConfig `yaml:"watchdog"`
}

type WatchdogConfig struct {
	Enabled       bool          `yaml:"enabled"`
	CheckInterval time.Duration `yaml:"checkInterval"`
	MaxFailures   int           `yaml:"maxFailures"`
	AutoRestart   bool          `yaml:"autoRestart"`
	RestartDelay  time.Duration `yaml:"restartDelay"`
}

type SecurityConfig struct {
	Passphrase       string `yaml:"passphrase"`
	SessionDir       string `yaml:"sessionDir"`
	PIIScanning      bool   `yaml:"piiScanning"`
	EncryptSession   bool   `yaml:"encryptSession"`
	EmitStateEvents  bool   `yaml:"emitStateEvents"`
}

type NetworkConfig struct {
	ProxyList           []string             `yaml:"proxyList"`
	ProxyEnabled        bool                 `yaml:"proxyEnabled"`
	ProxyHealthCheckURL string               `yaml:"proxyHealthCheckURL"`
	ProxyHealthInterval time.Duration        `yaml:"proxyHealthInterval"`
	CircuitBreaker      CircuitBreakerConfig `yaml:"circuitBreaker"`
}

type CircuitBreakerConfig struct {
	FailureThreshold  int           `yaml:"failureThreshold"`
	ResetTimeout      time.Duration `yaml:"resetTimeout"`
	HalfOpenThreshold int           `yaml:"halfOpenThreshold"`
}

type LoggingConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	Output     string `yaml:"output"`
	Structured bool   `yaml:"structured"`
}

type ApprovalConfig struct {
	Enabled             bool          `yaml:"enabled"`
	BridgeURL           string        `yaml:"bridgeURL"`
	RoomID              string        `yaml:"roomID"`
	Timeout             time.Duration `yaml:"timeout"`
	SensitiveOperations []string      `yaml:"sensitiveOperations"`
}

var defaultConfig = Config{
	Server: ServerConfig{
		Port:         "9222",
		Host:         "0.0.0.0",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	},
	Browser: BrowserConfig{
		EnginePath:    "/usr/local/bin/lightpanda",
		EnginePort:    "9333",
		HealthCheck:   true,
		CheckInterval: 5 * time.Second,
		Watchdog: WatchdogConfig{
			Enabled:       true,
			CheckInterval: 5 * time.Second,
			MaxFailures:   3,
			AutoRestart:   true,
			RestartDelay:  5 * time.Second,
		},
	},
	Security: SecurityConfig{
		Passphrase:     "",
		SessionDir:     "./sessions",
		PIIScanning:    true,
		EncryptSession: false,
	},
	Network: NetworkConfig{
		ProxyEnabled:        false,
		ProxyHealthCheckURL: "http://www.google.com",
		ProxyHealthInterval: 60 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			FailureThreshold:  3,
			ResetTimeout:      30 * time.Second,
			HalfOpenThreshold: 1,
		},
	},
	Logging: LoggingConfig{
		Level:      "INFO",
		Format:     "text",
		Output:     "stdout",
		Structured: false,
	},
	Approval: ApprovalConfig{
		Enabled:   false,
		BridgeURL: "http://127.0.0.1:8080",
		Timeout:   60 * time.Second,
		SensitiveOperations: []string{
			"session_create",
			"navigation",
			"file_download",
		},
	},
}

func Load(path string) (*Config, error) {
	cfg := defaultConfig

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := applyEnvOverrides(&cfg); err != nil {
		return nil, fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	return &cfg, nil
}

func applyEnvOverrides(cfg *Config) error {
	overrides := []struct {
		envVar string
		setter func(string)
	}{
		{"JETSKI_PORT", func(v string) { cfg.Server.Port = v }},
		{"JETSKI_HOST", func(v string) { cfg.Server.Host = v }},
		{"JETSKI_ENGINE_PATH", func(v string) { cfg.Browser.EnginePath = v }},
		{"JETSKI_ENGINE_PORT", func(v string) { cfg.Browser.EnginePort = v }},
		{"JETSKI_PASSPHRASE", func(v string) { cfg.Security.Passphrase = v }},
		{"JETSKI_SESSION_DIR", func(v string) { cfg.Security.SessionDir = v }},
		{"JETSKI_LOG_LEVEL", func(v string) { cfg.Logging.Level = v }},
		{"JETSKI_LOG_FORMAT", func(v string) { cfg.Logging.Format = v }},
		{"JETSKI_LOG_OUTPUT", func(v string) { cfg.Logging.Output = v }},
	}

	for _, override := range overrides {
		if value := os.Getenv(override.envVar); value != "" {
			override.setter(value)
		}
	}

	if proxyList := os.Getenv("JETSKI_PROXY_LIST"); proxyList != "" {
		proxies := strings.Split(proxyList, ",")
		cfg.Network.ProxyList = proxies
		cfg.Network.ProxyEnabled = true
	}

	if piiScanning := os.Getenv("JETSKI_PII_SCANNING"); piiScanning != "" {
		cfg.Security.PIIScanning = piiScanning == "true" || piiScanning == "1"
	}

	if encryptSession := os.Getenv("JETSKI_ENCRYPT_SESSION"); encryptSession != "" {
		cfg.Security.EncryptSession = encryptSession == "true" || encryptSession == "1"
	}

	if emitStateEvents := os.Getenv("JETSKI_EMIT_STATE_EVENTS"); emitStateEvents != "" {
		cfg.Security.EmitStateEvents = emitStateEvents == "true" || emitStateEvents == "1"
	}

	if approvalEnabled := os.Getenv("JETSKI_APPROVAL_ENABLED"); approvalEnabled != "" {
		cfg.Approval.Enabled = approvalEnabled == "true" || approvalEnabled == "1"
	}

	if bridgeURL := os.Getenv("JETSKI_BRIDGE_URL"); bridgeURL != "" {
		cfg.Approval.BridgeURL = bridgeURL
	}

	if roomID := os.Getenv("JETSKI_ROOM_ID"); roomID != "" {
		cfg.Approval.RoomID = roomID
	}

	if approvalTimeout := os.Getenv("JETSKI_APPROVAL_TIMEOUT"); approvalTimeout != "" {
		if seconds, err := strconv.Atoi(approvalTimeout); err == nil && seconds > 0 {
			cfg.Approval.Timeout = time.Duration(seconds) * time.Second
		}
	}

	return nil
}

func (c *Config) Validate() error {
	if c.Server.Port == "" {
		return fmt.Errorf("server port cannot be empty")
	}

	if c.Browser.EnginePath == "" {
		return fmt.Errorf("browser engine path cannot be empty")
	}

	if c.Browser.EnginePort == "" {
		return fmt.Errorf("browser engine port cannot be empty")
	}

	if c.Security.EncryptSession && c.Security.Passphrase == "" {
		return fmt.Errorf("passphrase is required when session encryption is enabled")
	}

	if c.Network.ProxyEnabled && len(c.Network.ProxyList) == 0 {
		return fmt.Errorf("proxy list cannot be empty when proxy is enabled")
	}

	validLevels := map[string]bool{
		"DEBUG": true,
		"INFO":  true,
		"WARN":  true,
		"ERROR": true,
	}

	if !validLevels[strings.ToUpper(c.Logging.Level)] {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	validFormats := map[string]bool{
		"text": true,
		"json": true,
	}

	if !validFormats[strings.ToLower(c.Logging.Format)] {
		return fmt.Errorf("invalid log format: %s", c.Logging.Format)
	}

	if c.Approval.Enabled {
		if c.Approval.BridgeURL == "" {
			return fmt.Errorf("approval bridge URL cannot be empty when approval is enabled")
		}
		if c.Approval.RoomID == "" {
			return fmt.Errorf("approval room ID cannot be empty when approval is enabled")
		}
	}

	return nil
}

func LoadDefault() (*Config, error) {
	cfg := defaultConfig
	if err := applyEnvOverrides(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
