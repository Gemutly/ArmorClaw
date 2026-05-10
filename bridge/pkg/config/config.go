// Package config provides configuration management for ArmorClaw bridge.
// Supports TOML configuration files with environment variable overrides.
package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/armorclaw/bridge/internal/adapter"
	"github.com/armorclaw/bridge/pkg/budget"
	"github.com/armorclaw/bridge/pkg/keystore"
	"github.com/armorclaw/bridge/pkg/rpc"
)

// errors.Config alias for type compatibility (imported in main.go to avoid circular dependency)
// This is a placeholder - the actual type is in bridge/pkg/errors/init.go

// Helper function to validate directory exists or can be created
func validateDirectoryWritable(dir string) error {
	// Check if directory exists
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// Try to create it
			if err := os.MkdirAll(dir, 0750); err != nil {
				return fmt.Errorf("cannot create directory: %w", err)
			}
			return nil
		}
		return fmt.Errorf("cannot access directory: %w", err)
	}

	// Check if it's actually a directory
	if !info.IsDir() {
		return fmt.Errorf("not a directory")
	}

	// Check if we can write to it
	testFile := filepath.Join(dir, ".write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("cannot write to directory: %w", err)
	}
	f.Close()
	os.Remove(testFile)

	return nil
}

var (
	ErrInvalidConfig = errors.New("invalid configuration")
	ErrMissingValue  = errors.New("missing required configuration value")
)

// FeatureFlags holds runtime feature toggles for v1.0 phases.
// All flags default to false/off unless explicitly enabled via config or env var.
type FeatureFlags struct {
	// ZeroTrustKeystore enables zero-trust keystore validation
	ZeroTrustKeystore bool `toml:"feature_zero_trust_keystore" env:"ARMORCLAW_FEATURE_ZERO_TRUST_KEYSTORE"`

	// VoicePipeline controls voice processing: "cloud" or "off"
	VoicePipeline string `toml:"feature_voice_pipeline" env:"ARMORCLAW_FEATURE_VOICE_PIPELINE"`

	// MultiTabReplay enables multi-tab browser replay capability
	MultiTabReplay bool `toml:"feature_multi_tab_replay" env:"ARMORCLAW_FEATURE_MULTI_TAB_REPLAY"`

	// E2EEBackup enables end-to-end encrypted backup
	E2EEBackup bool `toml:"feature_e2ee_backup" env:"ARMORCLAW_FEATURE_E2EE_BACKUP"`
}

// BudgetConfig holds budget-related configuration
type BudgetConfig struct {
	// DailyLimitUSD is the maximum daily spend in USD (0 = no limit)
	DailyLimitUSD float64 `toml:"daily_limit_usd" env:"ARMORCLAW_DAILY_LIMIT"`

	// MonthlyLimitUSD is the maximum monthly spend in USD (0 = no limit)
	MonthlyLimitUSD float64 `toml:"monthly_limit_usd" env:"ARMORCLAW_MONTHLY_LIMIT"`

	// AlertThreshold is the percentage (0-100) at which to alert (e.g., 80 = warn at 80% of limit)
	AlertThreshold float64 `toml:"alert_threshold" env:"ARMORCLAW_ALERT_THRESHOLD"`

	// HardStop prevents new sessions when limits are exceeded
	HardStop bool `toml:"hard_stop" env:"ARMORCLAW_HARD_STOP"`

	// ProviderCosts allows custom token costs per model
	ProviderCosts map[string]float64 `toml:"provider_costs"`
}

// Config holds all bridge configuration
type Config struct {
	// Server configuration
	Server ServerConfig `toml:"server"`

	// Keystore configuration
	Keystore KeystoreConfig `toml:"keystore"`

	// Matrix configuration
	Matrix MatrixConfig `toml:"matrix"`

	// Budget configuration
	Budget BudgetConfig `toml:"budget"`

	// WebRTC configuration
	WebRTC WebRTCConfig `toml:"webrtc"`

	// Voice configuration
	Voice VoiceConfig `toml:"voice"`

	// Notifications configuration
	Notifications NotificationsConfig `toml:"notifications"`

	// Browser service configuration
	Browser BrowserConfig `toml:"browser"`

	// Event bus configuration
	EventBus EventBusConfig `toml:"eventbus"`

	// Discovery configuration (mDNS)
	Discovery DiscoveryConfig `toml:"discovery"`

	// Error system configuration
	ErrorSystem ErrorSystemConfig `toml:"errors"`

	// Compliance configuration (PII/PHI scrubbing)
	Compliance ComplianceConfig `toml:"compliance"`

	// Provisioning configuration (ArmorChat first-boot claim)
	Provisioning ProvisioningConfig `toml:"provisioning"`

	// Logging configuration
	Logging LoggingConfig `toml:"logging"`

	// Vault configuration (Rust Vault governance)
	Vault VaultConfig `toml:"vault"`

	// HTTP server configuration
	HTTP HTTPConfig `toml:"http"`

	// Java sidecar configuration (legacy .doc/.ppt extraction)
	SidecarJava SidecarJavaConfig `toml:"sidecar_java"`

	// Feature flags for v1.0 phases
	Features FeatureFlags `toml:"features"`

	// Governance configuration (room ID for governance/voting)
	Governance GovernanceConfig `toml:"governance"`
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	// SocketPath is the path to the Unix domain socket
	SocketPath string `toml:"socket_path" env:"ARMORCLAW_SOCKET"`

	// PidFile is the path to the PID file for daemon mode
	PidFile string `toml:"pid_file" env:"ARMORCLAW_PID_FILE"`

	// Daemonize runs the server as a background daemon
	Daemonize bool `toml:"daemonize" env:"ARMORCLAW_DAEMONIZE"`

	// Auth specifies the authentication mode for RPC calls
	// Valid values: "token" (Matrix token-based authentication)
	// Deprecated: "none" is not allowed for production deployments
	Auth string `toml:"auth" env:"ARMORCLAW_AUTH"`

	// Mode is the server deployment mode: "native" (Unix sockets) or "sentinel" (TCP + TLS)
	Mode string `toml:"mode" env:"ARMORCLAW_SERVER_MODE"`

	// RPCTransport is the RPC transport type: "unix" or "tcp"
	RPCTransport string `toml:"rpc_transport" env:"ARMORCLAW_RPC_TRANSPORT"`

	// ListenAddr is the TCP listen address for sentinel mode (e.g., "0.0.0.0:8080")
	ListenAddr string `toml:"listen_addr" env:"ARMORCLAW_LISTEN_ADDR"`

	// PublicBaseURL is the public URL for sentinel mode (e.g., "https://bridge.example.com")
	PublicBaseURL string `toml:"public_base_url" env:"ARMORCLAW_PUBLIC_BASE_URL"`

	// AdminToken is the generated admin token for Sentinel mode
	AdminToken string `toml:"admin_token" env:"ARMORCLAW_ADMIN_TOKEN"`
}

// KeystoreConfig holds keystore-specific configuration
type KeystoreConfig struct {
	// DBPath is the path to the encrypted keystore database
	DBPath string `toml:"db_path" env:"ARMORCLAW_KEYSTORE_DB"`

	// MasterKey is an optional master key (if not provided, derived from hardware)
	MasterKey string `toml:"master_key" env:"ARMORCLAW_MASTER_KEY"`

	// Provider configuration
	Providers []ProviderConfig `toml:"providers"`
}

// ProviderConfig holds credentials for a specific provider
type ProviderConfig struct {
	ID          string            `toml:"id"`
	Provider    keystore.Provider `toml:"provider"`
	Token       string            `toml:"token" env:"ARMORCLAW_PROVIDER_TOKEN"`
	DisplayName string            `toml:"display_name"`
	ExpiresAt   int64             `toml:"expires_at"`
	Tags        []string          `toml:"tags"`
}

// E2EEConfig is a kill switch for Matrix E2EE crypto operations.
// When Enabled is false, no E2EE crypto runs. Default: false (unimplemented).
// Runtime toggle: bridge.e2ee_enable / bridge.e2ee_disable RPC methods.
type E2EEConfig struct {
	Enabled bool `toml:"enabled" env:"ARMORCLAW_E2EE_ENABLED"`
}

// MatrixConfig holds Matrix-specific configuration
type MatrixConfig struct {
	// Enabled enables Matrix communication
	Enabled bool `toml:"enabled" env:"ARMORCLAW_MATRIX_ENABLED"`

	// HomeserverURL is the Matrix homeserver URL
	HomeserverURL string `toml:"homeserver_url" env:"ARMORCLAW_MATRIX_HOMESERVER"`

	// Username for auto-login
	Username string `toml:"username" env:"ARMORCLAW_MATRIX_USERNAME"`

	// Password for auto-login
	Password string `toml:"password" env:"ARMORCLAW_MATRIX_PASSWORD"`

	// DeviceID for the Matrix client
	DeviceID string `toml:"device_id" env:"ARMORCLAW_MATRIX_DEVICE_ID"`

	// SyncInterval is the interval between syncs in seconds
	SyncInterval int `toml:"sync_interval" env:"ARMORCLAW_MATRIX_SYNC_INTERVAL"`

	// AutoRooms are rooms to automatically join on login
	AutoRooms []string `toml:"auto_rooms"`

	// EventBufferSize is the size of the high-throughput event bus ring buffer
	EventBufferSize int `toml:"event_buffer_size" env:"ARMORCLAW_MATRIX_EVENT_BUFFER_SIZE"`

	// Retry configuration
	Retry RetryConfig `toml:"retry"`

	ZeroTrust ZeroTrustConfig `toml:"zero_trust"`

	E2EE E2EEConfig `toml:"e2ee"`
}

// RetryConfig holds retry configuration for Matrix operations
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int `toml:"max_retries"`

	// RetryDelay is the delay between retries in seconds
	RetryDelay int `toml:"retry_delay"`

	// BackoffMultiplier multiplies the delay after each retry
	BackoffMultiplier float64 `toml:"backoff_multiplier"`
}

// ZeroTrustConfig holds zero-trust security settings
type ZeroTrustConfig struct {
	// TrustedSenders is a list of allowed Matrix user IDs
	// Supports wildcards: @user:domain.com, *@trusted.domain.com, *:domain.com
	TrustedSenders []string `toml:"trusted_senders" env:"ARMORCLAW_TRUSTED_SENDERS"`

	// TrustedRooms restricts message processing to specific rooms
	TrustedRooms []string `toml:"trusted_rooms" env:"ARMORCLAW_TRUSTED_ROOMS"`

	// RejectUntrusted controls behavior for untrusted messages
	// true: send rejection back to sender, false: drop silently with log
	RejectUntrusted bool `toml:"reject_untrusted" env:"ARMORCLAW_REJECT_UNTRUSTED"`
}

// LoggingConfig holds logging-specific configuration
type LoggingConfig struct {
	// Level is the log level (debug, info, warn, error)
	Level string `toml:"level" env:"ARMORCLAW_LOG_LEVEL"`

	// Format is the log format (json, text)
	Format string `toml:"format" env:"ARMORCLAW_LOG_FORMAT"`

	// Output is the log output (stdout, stderr, or file path)
	Output string `toml:"output" env:"ARMORCLAW_LOG_OUTPUT"`

	// File is the log file path when output is "file"
	File string `toml:"file" env:"ARMORCLAW_LOG_FILE"`
}

// WebRTCConfig holds WebRTC engine configuration
type WebRTCConfig struct {
	// DefaultLifetime is the default session lifetime
	DefaultLifetime string `toml:"default_lifetime" env:"ARMORCLAW_WEBRTC_DEFAULT_LIFETIME"`

	// MaxLifetime is the maximum session lifetime
	MaxLifetime string `toml:"max_lifetime" env:"ARMORCLAW_WEBRTC_MAX_LIFETIME"`

	// TURNSharedSecret is the shared secret for TURN credentials
	TURNSharedSecret string `toml:"turn_shared_secret" env:"ARMORCLAW_TURN_SHARED_SECRET"`

	// TURNServerURL is the TURN server URL
	TURNServerURL string `toml:"turn_server_url" env:"ARMORCLAW_TURN_SERVER_URL"`

	// ICEServers is a list of ICE servers (STUN/TURN)
	ICEServers []ICEServerConfig `toml:"ice_servers"`

	// AudioCodec configuration
	AudioCodec AudioCodecConfig `toml:"audio_codec"`

	// Signaling server configuration
	SignalingEnabled bool   `toml:"signaling_enabled" env:"ARMORCLAW_SIGNALING_ENABLED"`
	SignalingAddr    string `toml:"signaling_addr" env:"ARMORCLAW_SIGNALING_ADDR"`
	SignalingPath    string `toml:"signaling_path" env:"ARMORCLAW_SIGNALING_PATH"`
	SignalingTLSCert string `toml:"signaling_tls_cert" env:"ARMORCLAW_SIGNALING_TLS_CERT"`
	SignalingTLSKey  string `toml:"signaling_tls_key" env:"ARMORCLAW_SIGNALING_TLS_KEY"`
}

// ICEServerConfig represents an ICE server configuration
type ICEServerConfig struct {
	// URLs is a list of ICE server URLs
	URLs []string `toml:"urls"`

	// Username for TURN authentication (optional)
	Username string `toml:"username" env:"ARMORCLAW_ICE_USERNAME"`

	// Credential for TURN authentication (optional)
	Credential string `toml:"credential" env:"ARMORCLAW_ICE_CREDENTIAL"`
}

// AudioCodecConfig holds audio codec configuration
type AudioCodecConfig struct {
	// SampleRate is the audio sample rate in Hz
	SampleRate uint32 `toml:"sample_rate"`

	// Channels is the number of audio channels (1=mono, 2=stereo)
	Channels uint8 `toml:"channels"`

	// Bitrate is the target bitrate in bps
	Bitrate uint32 `toml:"bitrate"`

	// PayloadType is the RTP payload type
	PayloadType uint8 `toml:"payload_type"`
}

// VoiceConfig holds voice call configuration
type VoiceConfig struct {
	// DefaultLifetime is the default call lifetime
	DefaultLifetime string `toml:"default_lifetime" env:"ARMORCLAW_VOICE_DEFAULT_LIFETIME"`

	// MaxLifetime is the maximum call lifetime
	MaxLifetime string `toml:"max_lifetime" env:"ARMORCLAW_VOICE_MAX_LIFETIME"`

	// AutoAnswer automatically answers incoming calls
	AutoAnswer bool `toml:"auto_answer" env:"ARMORCLAW_VOICE_AUTO_ANSWER"`

	// RequireMembership requires room membership for calls
	RequireMembership bool `toml:"require_membership" env:"ARMORCLAW_VOICE_REQUIRE_MEMBERSHIP"`

	// AllowedRooms is a list of allowed rooms (empty = all allowed)
	AllowedRooms []string `toml:"allowed_rooms" env:"ARMORCLAW_VOICE_ALLOWED_ROOMS"`

	// BlockedRooms is a list of blocked rooms
	BlockedRooms []string `toml:"blocked_rooms" env:"ARMORCLAW_VOICE_BLOCKED_ROOMS"`

	// Security configuration
	Security VoiceSecurityConfig `toml:"security"`

	// Budget configuration
	Budget VoiceBudgetConfig `toml:"budget"`

	// TTL configuration
	TTL VoiceTTLConfig `toml:"ttl"`

	// VAD configuration
	VAD VoiceVADConfig `toml:"vad"`
}

// VoiceSecurityConfig holds voice security settings
type VoiceSecurityConfig struct {
	// MaxConcurrentCalls is the maximum number of concurrent calls
	MaxConcurrentCalls int `toml:"max_concurrent_calls" env:"ARMORCLAW_VOICE_MAX_CONCURRENT"`

	// MaxCallDuration is the maximum call duration
	MaxCallDuration string `toml:"max_call_duration" env:"ARMORCLAW_VOICE_MAX_CALL_DURATION"`

	// RateLimitCalls is the maximum calls per time window
	RateLimitCalls int `toml:"rate_limit_calls" env:"ARMORCLAW_VOICE_RATE_LIMIT_CALLS"`

	// RateLimitWindow is the rate limit time window
	RateLimitWindow string `toml:"rate_limit_window" env:"ARMORCLAW_VOICE_RATE_LIMIT_WINDOW"`

	// RequireE2EE requires end-to-end encryption
	RequireE2EE bool `toml:"require_e2ee" env:"ARMORCLAW_VOICE_REQUIRE_E2EE"`

	// RequireSignalingTLS requires TLS for signaling
	RequireSignalingTLS bool `toml:"require_signaling_tls" env:"ARMORCLAW_VOICE_REQUIRE_SIGNALING_TLS"`
}

// VoiceBudgetConfig holds voice budget settings
type VoiceBudgetConfig struct {
	// DefaultTokenLimit is the default token limit per call
	DefaultTokenLimit uint64 `toml:"default_token_limit" env:"ARMORCLAW_VOICE_DEFAULT_TOKEN_LIMIT"`

	// DefaultDurationLimit is the default duration limit per call
	DefaultDurationLimit string `toml:"default_duration_limit" env:"ARMORCLAW_VOICE_DEFAULT_DURATION_LIMIT"`

	// WarningThreshold is the percentage at which to warn (0.0-1.0)
	WarningThreshold float64 `toml:"warning_threshold" env:"ARMORCLAW_VOICE_WARNING_THRESHOLD"`

	// HardStop enforces hard limits when exceeded
	HardStop bool `toml:"hard_stop" env:"ARMORCLAW_VOICE_HARD_STOP"`
}

// VoiceTTLConfig holds voice TTL settings
type VoiceTTLConfig struct {
	// DefaultTTL is the default TTL for voice sessions
	DefaultTTL string `toml:"default_ttl" env:"ARMORCLAW_VOICE_DEFAULT_TTL"`

	// MaxTTL is the maximum allowed TTL
	MaxTTL string `toml:"max_ttl" env:"ARMORCLAW_VOICE_MAX_TTL"`

	// EnforcementInterval is how often to check TTL
	EnforcementInterval string `toml:"enforcement_interval" env:"ARMORCLAW_VOICE_ENFORCEMENT_INTERVAL"`

	// WarningThreshold is the percentage at which to warn
	WarningThreshold float64 `toml:"warning_threshold" env:"ARMORCLAW_VOICE_TTL_WARNING_THRESHOLD"`

	// HardStop enforces hard TTL expiration
	HardStop bool `toml:"hard_stop" env:"ARMORCLAW_VOICE_TTL_HARD_STOP"`
}

// VoiceVADConfig holds voice activity detection settings for the energy-threshold VAD.
type VoiceVADConfig struct {
	EnergyThreshold  float64 `toml:"energy_threshold" env:"ARMORCLAW_VOICE_VAD_ENERGY_THRESHOLD"`
	FrameDurationMs  int     `toml:"frame_duration_ms" env:"ARMORCLAW_VOICE_VAD_FRAME_DURATION_MS"`
	SilenceDurationMs int   `toml:"silence_duration_ms" env:"ARMORCLAW_VOICE_VAD_SILENCE_DURATION_MS"`
	SampleRate       uint32  `toml:"sample_rate" env:"ARMORCLAW_VOICE_VAD_SAMPLE_RATE"`
}

// NotificationsConfig holds notification system configuration
type NotificationsConfig struct {
	// AdminRoomID is the Matrix room ID for admin notifications
	AdminRoomID string `toml:"admin_room_id" env:"ARMORCLAW_ADMIN_ROOM"`

	// Enabled controls whether notifications are sent
	Enabled bool `toml:"enabled" env:"ARMORCLAW_NOTIFICATIONS_ENABLED"`

	// AlertThreshold is the percentage at which to send alerts (0.0-1.0)
	AlertThreshold float64 `toml:"alert_threshold" env:"ARMORCLAW_ALERT_THRESHOLD"`
}

// BrowserConfig holds browser service configuration
type BrowserConfig struct {
	// Enabled controls whether browser automation is available
	Enabled bool `toml:"enabled" env:"ARMORCLAW_BROWSER_ENABLED"`

	// ServiceURL is the URL of the browser-service HTTP API (legacy, used as default for LegacyURL)
	ServiceURL string `toml:"service_url" env:"ARMORCLAW_BROWSER_SERVICE_URL"`

	// CDPUrl is the CDP WebSocket URL for browser execution (e.g., ws://localhost:9222)
	CDPUrl string `toml:"cdp_url" env:"ARMORCLAW_BROWSER_CDP_URL"`

	// RPCUrl is the JSON-RPC HTTP URL for browser management (e.g., http://localhost:9223)
	RPCUrl string `toml:"rpc_url" env:"ARMORCLAW_BROWSER_RPC_URL"`

	// LegacyURL is the legacy browser-service HTTP API URL (e.g., http://localhost:3002)
	LegacyURL string `toml:"legacy_url" env:"ARMORCLAW_BROWSER_LEGACY_URL"`

	// Backend selects the browser backend: "jetski" or "legacy".
	//
	// Deprecated: The "legacy" (Playwright-based) backend is deprecated but
	// still supported as a fallback. Use "jetski" (CDP proxy) instead.
	// Legacy support will be removed in a future version.
	Backend string `toml:"backend" env:"ARMORCLAW_BROWSER_BACKEND"`

	// Fallback enables automatic fallback from jetski to legacy on failure
	Fallback bool `toml:"fallback" env:"ARMORCLAW_BROWSER_FALLBACK"`

	// Timeout is the default timeout for browser operations in seconds
	Timeout int `toml:"timeout" env:"ARMORCLAW_BROWSER_TIMEOUT"`

	// MaxRetries is the maximum number of retries for failed operations
	MaxRetries int `toml:"max_retries" env:"ARMORCLAW_BROWSER_MAX_RETRIES"`

	// RetryDelay is the delay between retries in seconds
	RetryDelay int `toml:"retry_delay" env:"ARMORCLAW_BROWSER_RETRY_DELAY"`

	// Stealth holds anti-detection configuration
	Stealth BrowserStealthConfig `toml:"stealth"`

	// Queue holds browser job queue configuration
	Queue BrowserQueueConfig `toml:"queue"`
}

// BrowserStealthConfig holds anti-detection settings
type BrowserStealthConfig struct {
	// Enabled controls whether stealth mode is active
	Enabled bool `toml:"enabled" env:"ARMORCLAW_BROWSER_STEALTH_ENABLED"`

	// FingerprintSeed is the seed for deterministic fingerprint generation
	FingerprintSeed string `toml:"fingerprint_seed" env:"ARMORCLAW_BROWSER_FINGERPRINT_SEED"`

	// HumanLikeTyping enables human-like typing delays
	HumanLikeTyping bool `toml:"human_like_typing" env:"ARMORCLAW_BROWSER_HUMAN_TYPING"`

	// HumanLikeMouse enables human-like mouse movements
	HumanLikeMouse bool `toml:"human_like_mouse" env:"ARMORCLAW_BROWSER_HUMAN_MOUSE"`

	// WebGLNoise enables WebGL fingerprint noise
	WebGLNoise bool `toml:"webgl_noise" env:"ARMORCLAW_BROWSER_WEBGL_NOISE"`

	// CanvasNoise enables canvas fingerprint noise
	CanvasNoise bool `toml:"canvas_noise" env:"ARMORCLAW_BROWSER_CANVAS_NOISE"`
}

// BrowserQueueConfig holds browser job queue settings
type BrowserQueueConfig struct {
	// MaxWorkers is the maximum number of concurrent browser workers
	MaxWorkers int `toml:"max_workers" env:"ARMORCLAW_BROWSER_MAX_WORKERS"`

	// MaxQueueSize is the maximum number of pending jobs
	MaxQueueSize int `toml:"max_queue_size" env:"ARMORCLAW_BROWSER_MAX_QUEUE_SIZE"`

	// JobTimeout is the maximum time a job can run in seconds
	JobTimeout int `toml:"job_timeout" env:"ARMORCLAW_BROWSER_JOB_TIMEOUT"`

	// PriorityLevels is the number of priority levels (1-10)
	PriorityLevels int `toml:"priority_levels" env:"ARMORCLAW_BROWSER_PRIORITY_LEVELS"`
}

// EventBusConfig holds event bus configuration for real-time event push
type EventBusConfig struct {
	// WebSocketEnabled enables the WebSocket server for event push
	WebSocketEnabled bool `toml:"websocket_enabled" env:"ARMORCLAW_EVENTBUS_WEBSOCKET_ENABLED"`

	// WebSocketAddr is the WebSocket listen address
	WebSocketAddr string `toml:"websocket_addr" env:"ARMORCLAW_EVENTBUS_WEBSOCKET_ADDR"`

	// WebSocketPath is the WebSocket path
	WebSocketPath string `toml:"websocket_path" env:"ARMORCLAW_EVENTBUS_WEBSOCKET_PATH"`

	// MaxSubscribers is the maximum concurrent subscribers
	MaxSubscribers int `toml:"max_subscribers" env:"ARMORCLAW_EVENTBUS_MAX_SUBSCRIBERS"`

	// InactivityTimeout is the timeout for inactive subscribers
	InactivityTimeout string `toml:"inactivity_timeout" env:"ARMORCLAW_EVENTBUS_INACTIVITY_TIMEOUT"`

	// EnableDurableLog enables the append-only event log
	EnableDurableLog bool `toml:"enable_durable_log" env:"ARMORCLAW_EVENTBUS_DURABLE_LOG_ENABLED"`

	// DurableLogDir is the directory where event log segments are stored
	DurableLogDir string `toml:"durable_log_dir" env:"ARMORCLAW_EVENTBUS_DURABLE_LOG_DIR"`

	// MaxLogFileSize is the maximum size of a single log segment
	MaxLogFileSize int64 `toml:"max_log_file_size" env:"ARMORCLAW_EVENTBUS_MAX_LOG_FILE_SIZE"`
}

// HTTPConfig holds the HTTPS bridge server configuration
type HTTPConfig struct {
	Enabled  bool   `toml:"enabled" env:"ARMORCLAW_HTTP_ENABLED"`
	Hostname string `toml:"hostname" env:"ARMORCLAW_HTTP_HOSTNAME"`
	Port     int    `toml:"port" env:"ARMORCLAW_HTTP_PORT"`
	CertDir  string `toml:"cert_dir" env:"ARMORCLAW_HTTP_CERT_DIR"`
}

// DiscoveryConfig holds mDNS/Bonjour discovery configuration
type DiscoveryConfig struct {
	// Enabled controls whether mDNS discovery is active
	Enabled bool `toml:"enabled" env:"ARMORCLAW_DISCOVERY_ENABLED"`

	// InstanceName is the service instance name (defaults to hostname)
	InstanceName string `toml:"instance_name" env:"ARMORCLAW_DISCOVERY_NAME"`

	// Port is the HTTP API port to advertise
	Port int `toml:"port" env:"ARMORCLAW_DISCOVERY_PORT"`

	// TLS indicates whether HTTPS is enabled
	TLS bool `toml:"tls" env:"ARMORCLAW_DISCOVERY_TLS"`

	// APIPath is the API endpoint path (default: /api)
	APIPath string `toml:"api_path" env:"ARMORCLAW_DISCOVERY_API_PATH"`

	// WSPath is the WebSocket path (default: /ws)
	WSPath string `toml:"ws_path" env:"ARMORCLAW_DISCOVERY_WS_PATH"`

	// MatrixHomeserver is the Matrix homeserver URL to advertise
	// If empty, uses the Matrix config's homeserver URL
	MatrixHomeserver string `toml:"matrix_homeserver" env:"ARMORCLAW_DISCOVERY_MATRIX_URL"`

	// PushGateway is the push gateway URL to advertise
	// If empty, derived from the API URL
	PushGateway string `toml:"push_gateway" env:"ARMORCLAW_DISCOVERY_PUSH_URL"`

	// Hardware describes the hardware platform (optional)
	Hardware string `toml:"hardware" env:"ARMORCLAW_DISCOVERY_HARDWARE"`
}

// ComplianceConfig holds PII/PHI compliance settings
type ComplianceConfig struct {
	// Enabled controls whether PII/PHI scrubbing is active
	// When enabled, switches from streaming to buffered responses for full scrubbing
	Enabled bool `toml:"enabled" env:"ARMORCLAW_COMPLIANCE_ENABLED"`

	// StreamingMode allows partial streaming with chunk-level scrubbing
	// WARNING: May miss cross-chunk patterns. Not recommended for HIPAA compliance.
	// Default: false (buffered mode when compliance enabled)
	StreamingMode bool `toml:"streaming_mode" env:"ARMORCLAW_COMPLIANCE_STREAMING"`

	// QuarantineEnabled blocks messages containing critical PHI for admin review
	QuarantineEnabled bool `toml:"quarantine_enabled" env:"ARMORCLAW_COMPLIANCE_QUARANTINE"`

	// NotifyOnQuarantine sends notification to user when their message is quarantined
	NotifyOnQuarantine bool `toml:"notify_on_quarantine" env:"ARMORCLAW_COMPLIANCE_NOTIFY_QUARANTINE"`

	// AuditEnabled logs all PII/PHI detections for compliance auditing
	AuditEnabled bool `toml:"audit_enabled" env:"ARMORCLAW_COMPLIANCE_AUDIT"`

	// AuditRetentionDays is how long to keep compliance audit logs
	AuditRetentionDays int `toml:"audit_retention_days" env:"ARMORCLAW_COMPLIANCE_AUDIT_DAYS"`

	// Tier is the compliance tier (basic, standard, full)
	Tier string `toml:"tier" env:"ARMORCLAW_COMPLIANCE_TIER"`

	// Patterns controls which patterns to check
	Patterns PIIPatternConfig `toml:"patterns"`
}

// PIIPatternConfig controls individual PII pattern detection
type PIIPatternConfig struct {
	// SSN detection (US Social Security Numbers)
	SSN bool `toml:"ssn" env:"ARMORCLAW_PII_SSN"`

	// CreditCard detection with Luhn validation
	CreditCard bool `toml:"credit_card" env:"ARMORCLAW_PII_CREDIT_CARD"`

	// MedicalRecord detection (MRN, Patient ID)
	MedicalRecord bool `toml:"medical_record" env:"ARMORCLAW_PII_MEDICAL_RECORD"`

	// HealthPlan detection (Medicare, Medicaid numbers)
	HealthPlan bool `toml:"health_plan" env:"ARMORCLAW_PII_HEALTH_PLAN"`

	// DeviceID detection (Medical device identifiers)
	DeviceID bool `toml:"device_id" env:"ARMORCLAW_PII_DEVICE_ID"`

	// Biometric detection
	Biometric bool `toml:"biometric" env:"ARMORCLAW_PII_BIOMETRIC"`

	// LabResult detection
	LabResult bool `toml:"lab_result" env:"ARMORCLAW_PII_LAB_RESULT"`

	// Diagnosis detection (ICD codes)
	Diagnosis bool `toml:"diagnosis" env:"ARMORCLAW_PII_DIAGNOSIS"`

	// Prescription detection
	Prescription bool `toml:"prescription" env:"ARMORCLAW_PII_PRESCRIPTION"`

	// Email detection
	Email bool `toml:"email" env:"ARMORCLAW_PII_EMAIL"`

	// Phone detection
	Phone bool `toml:"phone" env:"ARMORCLAW_PII_PHONE"`

	// IPAddress detection
	IPAddress bool `toml:"ip_address" env:"ARMORCLAW_PII_IP"`

	// APIToken detection
	APIToken bool `toml:"api_token" env:"ARMORCLAW_PII_API_TOKEN"`
}

// ComplianceMode represents the response processing mode
type ComplianceMode string

const (
	// ComplianceModeStreaming processes chunks as they arrive (may miss patterns)
	ComplianceModeStreaming ComplianceMode = "streaming"
	// ComplianceModeBuffered collects full response before scrubbing (recommended)
	ComplianceModeBuffered ComplianceMode = "buffered"
)

// GetMode returns the compliance processing mode
func (c *ComplianceConfig) GetMode() ComplianceMode {
	if !c.Enabled {
		return ComplianceModeStreaming // No scrubbing needed, streaming OK
	}
	if c.StreamingMode {
		return ComplianceModeStreaming
	}
	return ComplianceModeBuffered
}

// IsHIPAAEnabled returns true if HIPAA-compliant patterns are enabled
func (c *ComplianceConfig) IsHIPAAEnabled() bool {
	return c.Enabled && (c.Tier == "full" || c.Tier == "standard")
}

// IsStrictMode returns true if strict (quarantine) mode is enabled
func (c *ComplianceConfig) IsStrictMode() bool {
	return c.Enabled && c.QuarantineEnabled
}

// GetEnabledPatterns returns a list of enabled pattern names
func (c *ComplianceConfig) GetEnabledPatterns() []string {
	patterns := []string{}

	// Standard PII patterns
	if c.Patterns.SSN {
		patterns = append(patterns, "ssn")
	}
	if c.Patterns.CreditCard {
		patterns = append(patterns, "credit_card")
	}
	if c.Patterns.Email {
		patterns = append(patterns, "email")
	}
	if c.Patterns.Phone {
		patterns = append(patterns, "phone")
	}
	if c.Patterns.IPAddress {
		patterns = append(patterns, "ip_address")
	}
	if c.Patterns.APIToken {
		patterns = append(patterns, "api_token")
	}

	// HIPAA/PHI patterns
	if c.Patterns.MedicalRecord {
		patterns = append(patterns, "medical_record")
	}
	if c.Patterns.HealthPlan {
		patterns = append(patterns, "health_plan")
	}
	if c.Patterns.DeviceID {
		patterns = append(patterns, "device_id")
	}
	if c.Patterns.Biometric {
		patterns = append(patterns, "biometric")
	}
	if c.Patterns.LabResult {
		patterns = append(patterns, "lab_result")
	}
	if c.Patterns.Diagnosis {
		patterns = append(patterns, "diagnosis")
	}
	if c.Patterns.Prescription {
		patterns = append(patterns, "prescription")
	}

	return patterns
}

// ComplianceTierDefaults returns default compliance settings for a license tier
func ComplianceTierDefaults(tier string) ComplianceConfig {
	switch tier {
	case "ent", "enterprise", "maximum":
		// Enterprise/Maximum: Full HIPAA compliance, buffered mode, quarantine
		return ComplianceConfig{
			Enabled:            true,
			StreamingMode:      false, // Buffered for full compliance
			QuarantineEnabled:  true,
			NotifyOnQuarantine: true,
			AuditEnabled:       true,
			AuditRetentionDays: 90,
			Tier:               "full",
			Patterns: PIIPatternConfig{
				SSN:           true,
				CreditCard:    true,
				MedicalRecord: true,
				HealthPlan:    true,
				DeviceID:      true,
				Biometric:     true,
				LabResult:     true,
				Diagnosis:     true,
				Prescription:  true,
				Email:         true,
				Phone:         true,
				IPAddress:     true,
				APIToken:      true,
			},
		}
	case "pro", "professional":
		// Professional: Basic PII only, streaming mode, no quarantine
		return ComplianceConfig{
			Enabled:            false, // Disabled by default for performance
			StreamingMode:      true,
			QuarantineEnabled:  false,
			NotifyOnQuarantine: false,
			AuditEnabled:       false,
			AuditRetentionDays: 30,
			Tier:               "basic",
			Patterns: PIIPatternConfig{
				SSN:        true,
				CreditCard: true,
				Email:      false,
				Phone:      false,
				IPAddress:  false,
				APIToken:   true,
			},
		}
	default:
		// Essential/Free: Disabled for maximum performance
		return ComplianceConfig{
			Enabled:            false,
			StreamingMode:      true,
			QuarantineEnabled:  false,
			NotifyOnQuarantine: false,
			AuditEnabled:       false,
			AuditRetentionDays: 7,
			Tier:               "basic",
			Patterns:           PIIPatternConfig{},
		}
	}
}

// ProvisioningConfig holds provisioning system configuration (ArmorChat first-boot claim)
type ProvisioningConfig struct {
	// SigningSecret is the HMAC key for signing provisioning QR data
	SigningSecret string `toml:"signing_secret" env:"ARMORCLAW_PROVISIONING_SECRET"`

	// DefaultExpirySeconds is the default token lifetime
	DefaultExpirySeconds int `toml:"default_expiry_seconds" env:"ARMORCLAW_PROVISIONING_DEFAULT_EXPIRY"`

	// MaxExpirySeconds is the maximum allowed token lifetime
	MaxExpirySeconds int `toml:"max_expiry_seconds" env:"ARMORCLAW_PROVISIONING_MAX_EXPIRY"`

	// OneTimeUse determines if tokens can only be used once
	OneTimeUse bool `toml:"one_time_use" env:"ARMORCLAW_PROVISIONING_ONE_TIME_USE"`

	// DataDir is the directory for persisting roles (empty = in-memory only)
	DataDir string `toml:"data_dir" env:"ARMORCLAW_PROVISIONING_DATA_DIR"`
}

type GovernanceConfig struct {
	RoomID string `toml:"room_id" env:"ARMORCLAW_GOVERNANCE_ROOM"`
}

// SidecarJavaConfig holds configuration for the Java sidecar (legacy .doc/.ppt extraction)
type SidecarJavaConfig struct {
	// Enabled controls whether the Java sidecar is used for .doc/.ppt extraction.
	// When false, OLE .doc/.ppt files continue routing to the Python sidecar (current behavior).
	Enabled bool `toml:"enabled" env:"ARMORCLAW_SIDECAR_JAVA_ENABLED" envDefault:"false"`

	// SocketPath is the path to the Java sidecar Unix domain socket
	SocketPath string `toml:"socket_path" env:"ARMORCLAW_SIDECAR_JAVA_SOCKET_PATH" envDefault:"/run/armorclaw/sidecar-java/sidecar-java.sock"`
}

// VaultConfig holds configuration for the Rust Vault governance integration
type VaultConfig struct {
	// V6Microkernel enables the v6 microkernel architecture with vault governance,
	// ephemeral tokens, and ToolSidecar lifecycle hooks.
	// Tool execution is real (Docker exec-based) and vaultClient wiring is complete
	// (setupMCPRouter receives and sets VaultClient on mcp.Config).
	// Feature remains gated: V6Microkernel defaults to false.
	// When false, legacy execution paths are used (default).
	V6Microkernel bool `toml:"v6_microkernel" env:"ARMORCLAW_V6_MICROKERNEL"`

	// V6AuditMode enables audit-only mode for the v6 microkernel.
	// When true AND v6_microkernel=true, every action that would be taken
	// (PII interception, governance checks, ToolSidecar spawns, token issuance)
	// is logged but NOT executed. Tool calls pass through unmodified.
	// Useful for pre-deployment validation without enforcement.
	V6AuditMode bool `toml:"v6_audit_mode" env:"ARMORCLAW_V6_AUDIT_MODE"`

	// SocketPath is the path to the Rust Vault governance gRPC socket
	SocketPath string `toml:"socket_path" env:"ARMORCLAW_VAULT_SOCKET"`
}

// ErrorSystemConfig holds error handling system configuration
type ErrorSystemConfig struct {
	// Enabled controls whether the error system is active
	Enabled bool `toml:"enabled" env:"ARMORCLAW_ERRORS_ENABLED"`

	// StoreEnabled controls whether errors are persisted to SQLite
	StoreEnabled bool `toml:"store_enabled" env:"ARMORCLAW_ERRORS_STORE_ENABLED"`

	// NotifyEnabled controls whether Matrix notifications are sent
	NotifyEnabled bool `toml:"notify_enabled" env:"ARMORCLAW_ERRORS_NOTIFY_ENABLED"`

	// StorePath is the path to the SQLite error database
	StorePath string `toml:"store_path" env:"ARMORCLAW_ERRORS_STORE_PATH"`

	// RetentionDays is how long to keep resolved errors
	RetentionDays int `toml:"retention_days" env:"ARMORCLAW_ERRORS_RETENTION_DAYS"`

	// RateLimitWindow is the window for rate-limiting notifications (e.g., "5m")
	RateLimitWindow string `toml:"rate_limit_window" env:"ARMORCLAW_ERRORS_RATE_LIMIT_WINDOW"`

	// RetentionPeriod is how long to keep error counts for sampling
	RetentionPeriod string `toml:"retention_period" env:"ARMORCLAW_ERRORS_RETENTION_PERIOD"`

	// AdminMXID is the configured admin Matrix ID (highest priority)
	AdminMXID string `toml:"admin_mxid" env:"ARMORCLAW_ERRORS_ADMIN_MXID"`

	// SetupUserMXID is the MXID of the user who ran setup (second priority)
	SetupUserMXID string `toml:"setup_user_mxid" env:"ARMORCLAW_ERRORS_SETUP_USER_MXID"`

	// AdminRoomID is the room ID to search for admins (third priority)
	AdminRoomID string `toml:"admin_room_id" env:"ARMORCLAW_ERRORS_ADMIN_ROOM_ID"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	cfg := &Config{
		Server: ServerConfig{
			Mode:         "native", // Default to native mode (Unix socket)
			RPCTransport: "unix",   // Default to Unix socket
			SocketPath:   "/run/armorclaw/bridge.sock",
			PidFile:      filepath.Join(os.TempDir(), "armorclaw", "bridge.pid"),
			Daemonize:    false,
			Auth:         "token",
		},
		Keystore: KeystoreConfig{
			DBPath:    "/var/lib/armorclaw/keystore.db",
			MasterKey: "",
			Providers: []ProviderConfig{},
		},
		Matrix: MatrixConfig{
			Enabled:       false,
			HomeserverURL: "",
			Username:      "",
			Password:      "",
			DeviceID:      "armorclaw-bridge",
			SyncInterval:  5,
			AutoRooms:     []string{},
			Retry: RetryConfig{
				MaxRetries:        3,
				RetryDelay:        5,
				BackoffMultiplier: 2.0,
			},
			ZeroTrust: ZeroTrustConfig{
				TrustedSenders:  []string{}, // Empty = allow all (backward compatible)
				TrustedRooms:    []string{}, // Empty = allow all rooms
				RejectUntrusted: false,      // Silent drop by default
			},
			E2EE: E2EEConfig{
				Enabled: false,
			},
		},
		Budget: BudgetConfig{
			DailyLimitUSD:   5.00,   // $5/day default
			MonthlyLimitUSD: 100.00, // $100/month default
			AlertThreshold:  80.0,   // Warn at 80%
			HardStop:        true,   // Prevent overages by default
			ProviderCosts:   make(map[string]float64),
		},
		WebRTC: WebRTCConfig{
			DefaultLifetime:  "30m",
			MaxLifetime:      "2h",
			TURNSharedSecret: "",
			TURNServerURL:    "",
			ICEServers: []ICEServerConfig{
				{
					URLs: []string{"stun:stun.l.google.com:19302"},
				},
			},
			AudioCodec: AudioCodecConfig{
				SampleRate:  48000,
				Channels:    1,
				Bitrate:     64000,
				PayloadType: 111,
			},
			// Signaling server configuration
			SignalingEnabled: false,
			SignalingAddr:    "0.0.0.0:8443",
			SignalingPath:    "/webrtc",
			SignalingTLSCert: "",
			SignalingTLSKey:  "",
		},
		Voice: VoiceConfig{
			DefaultLifetime:   "30m",
			MaxLifetime:       "2h",
			AutoAnswer:        false,
			RequireMembership: true,
			AllowedRooms:      []string{},
			BlockedRooms:      []string{},
			Security: VoiceSecurityConfig{
				MaxConcurrentCalls:  10,
				MaxCallDuration:     "1h",
				RateLimitCalls:      10,
				RateLimitWindow:     "1h",
				RequireE2EE:         false, // resolved in Validate() based on E2EE state
				RequireSignalingTLS: true,
			},
			Budget: VoiceBudgetConfig{
				DefaultTokenLimit:    100000,
				DefaultDurationLimit: "30m",
				WarningThreshold:     0.8,
				HardStop:             true,
			},
			TTL: VoiceTTLConfig{
				DefaultTTL:          "10m",
				MaxTTL:              "1h",
				EnforcementInterval: "30s",
				WarningThreshold:    0.9,
				HardStop:            true,
			},
			VAD: VoiceVADConfig{
				EnergyThreshold:   0.01,
				FrameDurationMs:   20,
				SilenceDurationMs: 300,
				SampleRate:        16000,
			},
		},
		Notifications: NotificationsConfig{
			AdminRoomID:    "",
			Enabled:        false,
			AlertThreshold: 0.8,
		},
		Browser: BrowserConfig{
			Enabled:    false,
			ServiceURL: "http://localhost:3002",
			CDPUrl:     "ws://localhost:9222",
			RPCUrl:     "http://localhost:9223",
			LegacyURL:  "http://localhost:3002",
			Backend:    "jetski",
			Fallback:   true,
			Timeout:    120,
			MaxRetries: 3,
			RetryDelay: 5,
			Stealth: BrowserStealthConfig{
				Enabled:         true,
				FingerprintSeed: "armorclaw-default",
				HumanLikeTyping: true,
				HumanLikeMouse:  true,
				WebGLNoise:      true,
				CanvasNoise:     true,
			},
			Queue: BrowserQueueConfig{
				MaxWorkers:     3,
				MaxQueueSize:   100,
				JobTimeout:     300, // 5 minutes
				PriorityLevels: 3,
			},
		},
		HTTP: HTTPConfig{
			Enabled:  false,
			Hostname: "",
			Port:     8443,
			CertDir:  "/etc/armorclaw/certs",
		},
		EventBus: EventBusConfig{
			WebSocketEnabled:  false,
			WebSocketAddr:     "0.0.0.0:8444",
			WebSocketPath:     "/events",
			MaxSubscribers:    100,
			InactivityTimeout: "30m",
		},
		Discovery: DiscoveryConfig{
			Enabled:          true,  // Enable mDNS discovery by default
			InstanceName:     "",    // Will use hostname if empty
			Port:             8080,  // Default HTTP port
			TLS:              false, // Default to HTTP for local development
			APIPath:          "/api",
			WSPath:           "/ws",
			MatrixHomeserver: "", // Will use Matrix config if empty
			PushGateway:      "", // Will derive from API URL if empty
			Hardware:         "",
		},
		ErrorSystem: ErrorSystemConfig{
			Enabled:         true,
			StoreEnabled:    true,
			NotifyEnabled:   true,
			StorePath:       "/var/lib/armorclaw/errors.db",
			RetentionDays:   30,
			RateLimitWindow: "5m",
			RetentionPeriod: "24h",
			AdminMXID:       "",
			SetupUserMXID:   "",
			AdminRoomID:     "",
		},
		Provisioning: ProvisioningConfig{
			SigningSecret:        "", // Generated during container-setup.sh
			DefaultExpirySeconds: 60,
			MaxExpirySeconds:     300,
			OneTimeUse:           true,
			DataDir:              "", // Set by container-setup.sh for persistence
		},
		Compliance: ComplianceConfig{
			Enabled:            false, // Disabled by default for performance
			StreamingMode:      true,  // Allow streaming when disabled
			QuarantineEnabled:  false,
			NotifyOnQuarantine: false,
			AuditEnabled:       false,
			AuditRetentionDays: 30,
			Tier:               "basic",
			Patterns: PIIPatternConfig{
				// Basic PII patterns enabled by default
				SSN:        true,
				CreditCard: true,
				APIToken:   true,
				// PHI patterns disabled by default (enable for Enterprise)
				MedicalRecord: false,
				HealthPlan:    false,
				DeviceID:      false,
				Biometric:     false,
				LabResult:     false,
				Diagnosis:     false,
				Prescription:  false,
				Email:         false,
				Phone:         false,
				IPAddress:     false,
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			Output: "stdout",
			File:   "",
		},
		Vault: VaultConfig{
			V6Microkernel: false,
			V6AuditMode:   false,
			SocketPath:    "/run/armorclaw/vault/keystore.sock",
		},
		Features: FeatureFlags{
			ZeroTrustKeystore: false,
			VoicePipeline:     "off",
			MultiTabReplay:    false,
			E2EEBackup:        false,
		},
		Governance: GovernanceConfig{
			RoomID: "",
		},
	}

	// T17: Default E2EE enabled for fresh installs only.
	// Existing installations preserve their legacy default (false).
	if IsFreshInstall() {
		cfg.Matrix.E2EE.Enabled = true
	}

	return cfg
}

// IsFreshInstall returns true when no config file exists at any default path
// and no keystore directory exists. Used by T17 to default E2EE on for fresh installs only.
func IsFreshInstall() bool {
	for _, p := range ConfigPaths() {
		if _, err := os.Stat(p); err == nil {
			return false
		}
	}

	defaults := DefaultConfig()
	keystoreDir := filepath.Dir(defaults.Keystore.DBPath)
	if _, err := os.Stat(keystoreDir); err == nil {
		return false
	}

	return true
}

// ConfigPaths returns the list of default configuration file paths to check
func ConfigPaths() []string {
	homeDir, _ := os.UserHomeDir()
	return []string{
		filepath.Join(homeDir, ".armorclaw", "config.toml"),
		filepath.Join("/etc", "armorclaw", "config.toml"),
		"./config.toml",
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate server configuration
	if c.Server.Mode == "" {
		c.Server.Mode = "native"
	}

	validModes := map[string]bool{
		"native":   true,
		"sentinel": true,
	}
	if !validModes[c.Server.Mode] {
		return fmt.Errorf("%w: server.mode must be 'native' or 'sentinel', got '%s'", ErrInvalidConfig, c.Server.Mode)
	}

	if c.Server.Mode == "sentinel" {
		if c.Server.ListenAddr == "" {
			return fmt.Errorf("%w: server.listen_addr is required when server.mode is 'sentinel'", ErrInvalidConfig)
		}
		if c.Server.PublicBaseURL == "" {
			return fmt.Errorf("%w: server.public_base_url is required when server.mode is 'sentinel'", ErrInvalidConfig)
		}
	} else {
		if c.Server.SocketPath == "" {
			return fmt.Errorf("%w: server.socket_path is required when server.mode is 'native'", ErrInvalidConfig)
		}

		socketDir := filepath.Dir(c.Server.SocketPath)
		if err := validateDirectoryWritable(socketDir); err != nil {
			return fmt.Errorf("%w: socket directory %s: %w", ErrInvalidConfig, socketDir, err)
		}
	}

	// Validate authentication mode
	if c.Server.Auth == "" {
		c.Server.Auth = "token"
	}

	validAuthModes := map[string]bool{
		"token": true,
	}

	if !validAuthModes[c.Server.Auth] {
		return fmt.Errorf("%w: server.auth must be 'token', got '%s'. auth: none is deprecated and not allowed for production", ErrInvalidConfig, c.Server.Auth)
	}

	// Validate keystore configuration
	if c.Keystore.DBPath == "" {
		return fmt.Errorf("%w: keystore.db_path is required", ErrInvalidConfig)
	}

	// Validate keystore directory exists or can be created
	keystoreDir := filepath.Dir(c.Keystore.DBPath)
	if err := validateDirectoryWritable(keystoreDir); err != nil {
		return fmt.Errorf("%w: keystore directory %s: %w", ErrInvalidConfig, keystoreDir, err)
	}

	// Validate Matrix configuration if enabled
	if c.Matrix.Enabled {
		if c.Matrix.HomeserverURL == "" {
			return fmt.Errorf("%w: matrix.homeserver_url is required when matrix is enabled", ErrInvalidConfig)
		}

		// Validate sync interval
		if c.Matrix.SyncInterval < 1 {
			return fmt.Errorf("%w: matrix.sync_interval must be at least 1 second", ErrInvalidConfig)
		}

		// Validate retry configuration
		if c.Matrix.Retry.MaxRetries < 0 {
			return fmt.Errorf("%w: matrix.retry.max_retries cannot be negative", ErrInvalidConfig)
		}

		if c.Matrix.Retry.RetryDelay < 0 {
			return fmt.Errorf("%w: matrix.retry.retry_delay cannot be negative", ErrInvalidConfig)
		}

		if c.Matrix.Retry.BackoffMultiplier < 1.0 {
			return fmt.Errorf("%w: matrix.retry.backoff_multiplier must be at least 1.0", ErrInvalidConfig)
		}
	}

	// Validate logging configuration
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("%w: logging.level must be one of: debug, info, warn, error", ErrInvalidConfig)
	}

	validFormats := map[string]bool{
		"json": true,
		"text": true,
	}
	if !validFormats[c.Logging.Format] {
		return fmt.Errorf("%w: logging.format must be one of: json, text", ErrInvalidConfig)
	}

	validOutputs := map[string]bool{
		"stdout": true,
		"stderr": true,
		"file":   true,
	}
	if !validOutputs[c.Logging.Output] {
		return fmt.Errorf("%w: logging.output must be one of: stdout, stderr, file", ErrInvalidConfig)
	}

	if c.Logging.Output == "file" && c.Logging.File == "" {
		return fmt.Errorf("%w: logging.file is required when logging.output is 'file'", ErrInvalidConfig)
	}

	// Validate budget configuration
	if c.Budget.DailyLimitUSD < 0 {
		return fmt.Errorf("%w: budget.daily_limit_usd cannot be negative", ErrInvalidConfig)
	}

	if c.Budget.MonthlyLimitUSD < 0 {
		return fmt.Errorf("%w: budget.monthly_limit_usd cannot be negative", ErrInvalidConfig)
	}

	if c.Budget.AlertThreshold < 0 || c.Budget.AlertThreshold > 100 {
		return fmt.Errorf("%w: budget.alert_threshold must be between 0 and 100", ErrInvalidConfig)
	}

	if err := c.Features.Validate(); err != nil {
		return err
	}

	c.Voice.Security.RequireE2EE = c.IsE2EEEnabled()

	return nil
}

// ToRPCConfig converts the Config to rpc.Config
func (c *Config) ToRPCConfig() rpc.Config {
	return rpc.Config{
		SocketPath: c.Server.SocketPath,
	}
}

// ToKeystoreConfig converts the Config to keystore.Config
func (c *Config) ToKeystoreConfig() keystore.Config {
	cfg := keystore.Config{
		DBPath: c.Keystore.DBPath,
	}

	// Parse master key if provided
	if c.Keystore.MasterKey != "" {
		// Master key should be hex-encoded
		cfg.MasterKey = []byte(c.Keystore.MasterKey)
	}

	return cfg
}

// ToMatrixConfig converts the Config to adapter.Config
func (c *Config) ToMatrixConfig() adapter.Config {
	return adapter.Config{
		HomeserverURL:   c.Matrix.HomeserverURL,
		DeviceID:        c.Matrix.DeviceID,
		TokenFile:       filepath.Join(filepath.Dir(c.Keystore.DBPath), "matrix_token.json"),
		TrustedSenders:  c.Matrix.ZeroTrust.TrustedSenders,
		TrustedRooms:    c.Matrix.ZeroTrust.TrustedRooms,
		RejectUntrusted: c.Matrix.ZeroTrust.RejectUntrusted,
		Username:        c.Matrix.Username,
	}
}

// MatrixCredentials returns the Matrix username and password for auto-login
func (c *Config) MatrixCredentials() (username, password string) {
	return c.Matrix.Username, c.Matrix.Password
}

// IsMatrixEnabled returns true if Matrix communication is enabled
func (c *Config) IsMatrixEnabled() bool {
	return c.Matrix.Enabled
}

func (c *Config) IsE2EEEnabled() bool {
	return c.Matrix.E2EE.Enabled
}

func (c *Config) IsFeatureEnabled(flagName string) bool {
	switch flagName {
	case "zero_trust_keystore":
		return c.Features.ZeroTrustKeystore
	case "voice_pipeline":
		return c.Features.VoicePipeline == "cloud"
	case "multi_tab_replay":
		return c.Features.MultiTabReplay
	case "e2ee_backup":
		return c.Features.E2EEBackup
	default:
		return false
	}
}

func (c *FeatureFlags) Validate() error {
	if c.VoicePipeline != "" && c.VoicePipeline != "cloud" && c.VoicePipeline != "off" {
		return fmt.Errorf("%w: features.voice_pipeline must be 'cloud' or 'off', got '%s'", ErrInvalidConfig, c.VoicePipeline)
	}
	return nil
}

// GetSyncInterval returns the Matrix sync interval as a Duration
func (c *Config) GetSyncInterval() time.Duration {
	return time.Duration(c.Matrix.SyncInterval) * time.Second
}

// GetLegacyURL returns the effective legacy browser-service URL.
// LegacyURL takes precedence; falls back to ServiceURL for backward compatibility.
func (c *Config) GetLegacyURL() string {
	if c.Browser.LegacyURL != "" {
		return c.Browser.LegacyURL
	}
	return c.Browser.ServiceURL
}

// WarnDeprecatedBrowserBackend logs a warning if the legacy browser backend is
// configured. Call this once during startup after config is loaded.
func (c *Config) WarnDeprecatedBrowserBackend() {
	if c.Browser.Backend == "legacy" {
		log.Printf("[WARN] Legacy browser backend is deprecated. Consider switching to Jetski (jetski) backend. Legacy support will be removed in a future version.")
	}
}

// ToBudgetConfig converts the Config to budget.BudgetConfig
func (c *Config) ToBudgetConfig() budget.BudgetConfig {
	return budget.BudgetConfig{
		DailyLimitUSD:   c.Budget.DailyLimitUSD,
		MonthlyLimitUSD: c.Budget.MonthlyLimitUSD,
		AlertThreshold:  c.Budget.AlertThreshold,
		HardStop:        c.Budget.HardStop,
		ProviderCosts:   c.Budget.ProviderCosts,
	}
}

// ErrorSystemConfigResult holds the converted error system config
// This mirrors errors.Config to avoid import cycles
type ErrorSystemConfigResult struct {
	StorePath       string
	RetentionDays   int
	RateLimitWindow string
	RetentionPeriod string
	ConfigAdminMXID string
	SetupUserMXID   string
	AdminRoomID     string
	FallbackMXID    string
	Enabled         bool
	StoreEnabled    bool
	NotifyEnabled   bool
}

// ToErrorSystemConfig converts the Config to error system config
func (c *Config) ToErrorSystemConfig() ErrorSystemConfigResult {
	return ErrorSystemConfigResult{
		StorePath:       c.ErrorSystem.StorePath,
		RetentionDays:   c.ErrorSystem.RetentionDays,
		RateLimitWindow: c.ErrorSystem.RateLimitWindow,
		RetentionPeriod: c.ErrorSystem.RetentionPeriod,
		ConfigAdminMXID: c.ErrorSystem.AdminMXID,
		SetupUserMXID:   c.ErrorSystem.SetupUserMXID,
		AdminRoomID:     c.ErrorSystem.AdminRoomID,
		FallbackMXID:    "",
		Enabled:         c.ErrorSystem.Enabled,
		StoreEnabled:    c.ErrorSystem.StoreEnabled,
		NotifyEnabled:   c.ErrorSystem.NotifyEnabled,
	}
}
