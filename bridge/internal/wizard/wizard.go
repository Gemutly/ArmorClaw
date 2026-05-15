// Package wizard provides an interactive TUI setup wizard for ArmorClaw
// using charmbracelet/huh forms. It collects configuration through multi-page
// forms with validation, then outputs results for the container setup script.
//
// Security: Secrets (API keys, passwords) are kept in memory only and passed
// to the container setup script via environment variables — never written to
// the JSON output file.
package wizard

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/armorclaw/bridge/pkg/setup"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"
)

// Version is the wizard version, matching the container setup version.
// Update this when releasing new versions - should match VERSION file in repo root.
const Version = "0.3.6"

// Profile represents a deployment profile.
const (
	ProfileQuick      = "quick"
	ProfileEnterprise = "enterprise"
)

// WizardConfig holds non-secret configuration values that are safe to write
// to a JSON file for the container setup script to consume.
type WizardConfig struct {
	Profile       string `json:"profile"`
	APIProvider   string `json:"api_provider"`
	APIBaseURL    string `json:"api_base_url"`
	DefaultModel  string `json:"default_model,omitempty"`
	AdminUser     string `json:"admin_user"`
	ServerName    string `json:"server_name,omitempty"`
	MatrixServer  string `json:"matrix_server,omitempty"`
	MatrixURL     string `json:"matrix_url,omitempty"`
	LogLevel      string `json:"log_level,omitempty"`
	SocketPath    string `json:"socket_path,omitempty"`
	SecurityTier  string `json:"security_tier,omitempty"`
	HIPAAEnabled  bool   `json:"hipaa_enabled,omitempty"`
	Quarantine    bool   `json:"quarantine_enabled,omitempty"`
	AuditRetDays  int    `json:"audit_retention_days,omitempty"`
	WizardVersion string `json:"wizard_version"`
}

// WizardSecrets holds sensitive values that must never be written to disk.
// These are passed to the container setup script via environment variables
// and then injected directly into the SQLCipher-encrypted keystore.
type WizardSecrets struct {
	APIKey         string
	AdminPassword  string
	BridgePassword string
}

// WizardResult combines config and secrets from a completed wizard run.
type WizardResult struct {
	Config  WizardConfig
	Secrets WizardSecrets
}

// TerminalCheckResult contains information about terminal capabilities.
type TerminalCheckResult struct {
	IsTTY         bool
	StdinIsTTY    bool
	StdoutIsTTY   bool
	Width         int
	Height        int
	StdinOpen     bool
	SupportsColor bool
	TERM          string
	CanRunHuh     bool // Combined check for all requirements
}

// checkTerminal verifies terminal capabilities for TUI operation.
// Huh? requires BOTH stdin AND stdout to be TTYs for interactive forms.
func checkTerminal() TerminalCheckResult {
	result := TerminalCheckResult{
		StdoutIsTTY:   term.IsTerminal(int(os.Stdout.Fd())),
		StdinIsTTY:    term.IsTerminal(int(os.Stdin.Fd())),
		StdinOpen:     true,
		SupportsColor: true,
	}

	// IsTTY is true only if BOTH stdin and stdout are TTYs
	// This is required for interactive Huh? forms
	result.IsTTY = result.StdinIsTTY && result.StdoutIsTTY

	// Check if stdin is open (not closed or piped)
	stat, err := os.Stdin.Stat()
	if err != nil {
		result.StdinOpen = false
	} else {
		// Check if stdin is a pipe or redirect (not interactive)
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			result.StdinOpen = false
		}
	}

	// Get terminal size if available (use stdout fd)
	if result.StdoutIsTTY {
		result.Width, result.Height, _ = term.GetSize(int(os.Stdout.Fd()))
	}

	// Check for color/ANSI support via TERM env
	result.TERM = os.Getenv("TERM")
	if result.TERM == "dumb" || result.TERM == "" {
		result.SupportsColor = false
	}

	// Determine if Huh? can run properly
	// Requirements:
	// 1. Both stdin and stdout must be TTYs
	// 2. Stdin must be open
	// 3. Terminal width must be >= 60 (if detectable)
	// 4. TERM should not be "dumb"
	result.CanRunHuh = result.IsTTY &&
		result.StdinOpen &&
		(result.Width == 0 || result.Width >= 60) &&
		result.TERM != "dumb"

	return result
}

// printTerminalError shows a helpful message when terminal is not suitable for TUI.
func printTerminalError(check TerminalCheckResult) {
	log.Println()
	log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("  Interactive Terminal Required")
	log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println()

	// Show diagnostic information
	log.Println("  Terminal Diagnostics:")
	log.Printf("    Stdin is TTY:  %v\n", check.StdinIsTTY)
	log.Printf("    Stdout is TTY: %v\n", check.StdoutIsTTY)
	log.Printf("    Stdin open:    %v\n", check.StdinOpen)
	log.Printf("    TERM:          %s\n", func() string {
		if check.TERM == "" {
			return "(not set)"
		}
		return check.TERM
	}())
	if check.Width > 0 {
		log.Printf("    Terminal size: %dx%d\n", check.Width, check.Height)
	}
	log.Println()

	if !check.StdinIsTTY || !check.StdoutIsTTY {
		log.Println("  The TUI wizard requires BOTH stdin and stdout to be terminals.")
		log.Println()
		log.Println("  Common causes:")
		if !check.StdinIsTTY {
			log.Println("    • Stdin is piped or redirected")
		}
		if !check.StdoutIsTTY {
			log.Println("    • Stdout is piped or redirected")
		}
		log.Println()
		log.Println("  Solutions:")
		log.Println("    1. Run with -it flags: docker run -it ...")
		log.Println("    2. Use environment variables (non-interactive):")
		log.Println("       -e ARMORCLAW_API_KEY=sk-your-key")
		log.Println("       -e ARMORCLAW_PROFILE=quick")
	}

	if check.TERM == "dumb" {
		log.Println("  TERM is set to 'dumb' - terminal does not support ANSI codes.")
		log.Println()
		log.Println("  Solutions:")
		log.Println("    1. Use a terminal that supports ANSI escape codes")
		log.Println("    2. Use environment variables for non-interactive setup")
	}

	if check.Width > 0 && check.Width < 60 {
		log.Printf("  Terminal is too narrow (%d columns, need 60+).\n", check.Width)
		log.Println("  Please resize your terminal window.")
	}

	log.Println()
	log.Println("  For non-interactive setup, set these environment variables:")
	log.Println("    ARMORCLAW_PROFILE=quick|enterprise")
	log.Println("    ARMORCLAW_API_KEY=your-api-key")
	log.Println("    ARMORCLAW_SERVER_NAME=your-server (optional, auto-detected)")
	log.Println()
}

// Run executes the interactive setup wizard and returns the collected
// configuration and secrets. Returns huh.ErrUserAborted if the user cancels.
func Run(accessible bool) (*WizardResult, error) {
	// Check for ACCESSIBLE env var
	if os.Getenv("ARMORCLAW_ACCESSIBLE") == "true" {
		accessible = true
	}

	// ALWAYS check env vars first - prefer non-interactive if configured
	// This allows users to provide env vars even with proper terminal
	if result := tryNonInteractive(); result != nil {
		return result, nil
	}

	// No env vars - check terminal capabilities before launching TUI
	termCheck := checkTerminal()

	// If terminal is not suitable for Huh? TUI, try accessible mode first
	if !termCheck.CanRunHuh && !accessible {
		// If we have a TTY but just missing some capabilities, try accessible mode
		if termCheck.IsTTY {
			log.Println("  Terminal has limited capabilities - using accessible mode")
			log.Println("  (Arrow keys and colors may not work properly)")
			log.Println()
			accessible = true
		} else {
			// No TTY at all - show error and exit
			printTerminalError(termCheck)
			return nil, setup.ErrTerminalNotTTY
		}
	}

	// Check terminal width (only if we could detect it)
	if termCheck.Width > 0 && termCheck.Width < 60 {
		printTerminalError(termCheck)
		return nil, setup.ErrTerminalTooNarrow
	}

	// Setup signal handling for graceful terminal cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		// Clean up terminal state before exit
		fmt.Print("\033[?25h") // Show cursor
		fmt.Print("\033[0m")   // Reset colors
		log.Println("\nSetup cancelled.")
		os.Exit(130)
	}()

	printBanner()

	result := &WizardResult{
		Config: WizardConfig{
			WizardVersion: Version,
			LogLevel:      "info",
			SocketPath:    "/run/armorclaw/bridge.sock",
			SecurityTier:  "enhanced",
			AdminUser:     "admin",
		},
	}

	// Page 1: Profile selection
	profile, err := runProfileForm(accessible)
	if err != nil {
		return nil, err
	}
	result.Config.Profile = profile

	// Route to profile-specific forms
	switch profile {
	case ProfileQuick:
		if err := runQuickStartForms(result, accessible); err != nil {
			return nil, err
		}
	case ProfileEnterprise:
		// Enterprise wizard is a future addition; for now fall through
		// to quick start with a note.
		log.Println("  Enterprise profile will be available in a future update.")
		log.Println("  Running Quick Start for now.")
		log.Println()
		result.Config.Profile = ProfileQuick
		if err := runQuickStartForms(result, accessible); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// tryNonInteractive attempts to build a WizardResult from environment variables.
// Returns nil if required env vars are missing.
func tryNonInteractive() *WizardResult {
	apiKey := strings.TrimSpace(os.Getenv("ARMORCLAW_API_KEY"))
	if apiKey == "" {
		return nil
	}

	profile := os.Getenv("ARMORCLAW_PROFILE")
	if profile == "" {
		profile = ProfileQuick
	}

	// Determine provider from API base URL
	baseURL := os.Getenv("ARMORCLAW_API_BASE_URL")
	provider := "openai"
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	// Detect provider from URL patterns
	switch {
	case strings.Contains(baseURL, "anthropic"):
		provider = "anthropic"
	case strings.Contains(baseURL, "z.ai"):
		provider = "openai" // Z.ai uses OpenAI-compatible API
	}

	// Get server name from env or auto-detect
	serverName := os.Getenv("ARMORCLAW_SERVER_NAME")
	if serverName == "" {
		serverName = detectServerName()
	}

	// Get or generate admin password
	adminPassword := os.Getenv("ARMORCLAW_ADMIN_PASSWORD")
	if adminPassword == "" {
		generated, err := generatePassword(16)
		if err != nil {
			// Log error but don't fail - use a fallback
			fmt.Fprintf(os.Stderr, "Warning: password generation failed: %v\n", err)
			generated = "ChangeMe123!" // Fallback password
		}
		adminPassword = generated
	}

	// Generate bridge password
	bridgePassword, err := generatePassword(16)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: bridge password generation failed: %v\n", err)
		bridgePassword = "BridgePass123!"
	}

	log.Println()
	log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("  Non-Interactive Mode (Environment Variables)")
	log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("  Profile:     %s\n", profile)
	log.Printf("  Server Name: %s\n", serverName)
	log.Printf("  Provider:    %s\n", provider)
	log.Printf("  Base URL:    %s\n", baseURL)
	log.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println()

	return &WizardResult{
		Config: WizardConfig{
			WizardVersion: Version,
			Profile:       profile,
			APIProvider:   provider,
			APIBaseURL:    baseURL,
			ServerName:    serverName,
			LogLevel:      "info",
			SocketPath:    "/run/armorclaw/bridge.sock",
			SecurityTier:  "enhanced",
			AdminUser:     "admin",
		},
		Secrets: WizardSecrets{
			APIKey:         apiKey,
			AdminPassword:  adminPassword,
			BridgePassword: bridgePassword,
		},
	}
}

// detectServerName attempts to auto-detect the server's public hostname or IP.
// Falls back to localhost if detection fails.
func detectServerName() string {
	// Try to get public IP (most reliable for VPS deployments)
	if conn, err := net.Dial("udp", "8.8.8.8:80"); err == nil {
		defer conn.Close()
		if localAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			return localAddr.IP.String()
		}
	}

	// Try hostname
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		return hostname
	}

	// Fallback to localhost
	return "localhost"
}

// WriteConfigJSON writes the non-secret configuration to a JSON file.
// Secrets are intentionally excluded — they are passed via environment
// variables by the caller.
func WriteConfigJSON(path string, cfg *WizardConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal wizard config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return setup.WrapError(err, setup.ErrConfigWriteFailed)
	}

	return nil
}

// SecretEnvVars returns the environment variable assignments for secrets.
// These are used to pass secrets to the container setup script without
// writing them to disk.
func SecretEnvVars(s *WizardSecrets) []string {
	vars := make([]string, 0, 3)
	if s.APIKey != "" {
		vars = append(vars, "ARMORCLAW_WIZARD_API_KEY="+s.APIKey)
	}
	if s.AdminPassword != "" {
		vars = append(vars, "ARMORCLAW_WIZARD_ADMIN_PASSWORD="+s.AdminPassword)
	}
	if s.BridgePassword != "" {
		vars = append(vars, "ARMORCLAW_WIZARD_BRIDGE_PASSWORD="+s.BridgePassword)
	}
	return vars
}

func printBanner() {
	log.Println()
	log.Println("  ╔══════════════════════════════════════════════════════╗")
	log.Println("  ║        ArmorClaw Container Setup                    ║")
	log.Printf("  ║        Version %-40s║\n", Version)
	log.Println("  ╚══════════════════════════════════════════════════════╝")
	log.Println()
}

// formOpts returns common form options (accessibility, theme).
func formOpts(f *huh.Form, accessible bool) *huh.Form {
	return f.
		WithTheme(ArmorClawTheme()).
		WithAccessible(accessible)
}
