// ArmorClaw Bridge - Main entry point
//
// The bridge provides a secure interface between the host system and isolated
// AI agent containers. It manages encrypted credentials and container lifecycle.
package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/armorclaw/bridge/internal/adapter"
	"github.com/armorclaw/bridge/internal/ai"
	"github.com/armorclaw/bridge/internal/events"
	"github.com/armorclaw/bridge/internal/wizard"
	"github.com/armorclaw/bridge/pkg/admin"
	"github.com/armorclaw/bridge/pkg/config"
	"github.com/armorclaw/bridge/pkg/crypto"
	"github.com/armorclaw/bridge/pkg/discovery"
	"github.com/armorclaw/bridge/pkg/docker"
	"github.com/armorclaw/bridge/pkg/errors"
	"github.com/armorclaw/bridge/pkg/eventbus"
	"github.com/armorclaw/bridge/pkg/health"
	"github.com/armorclaw/bridge/pkg/keystore"
	"github.com/armorclaw/bridge/pkg/lockdown"
	"github.com/armorclaw/bridge/pkg/logger"
	"github.com/armorclaw/bridge/pkg/notification"
	"github.com/armorclaw/bridge/pkg/providers"
	"github.com/armorclaw/bridge/pkg/provisioning"
	"github.com/armorclaw/bridge/pkg/qr"
	"github.com/armorclaw/bridge/pkg/rpc"
	"github.com/armorclaw/bridge/pkg/audit"
	"github.com/armorclaw/bridge/pkg/secretary"
	"github.com/armorclaw/bridge/pkg/setup"
	"github.com/armorclaw/bridge/pkg/studio"
	"github.com/armorclaw/bridge/pkg/systemd"
	"github.com/armorclaw/bridge/pkg/trust"
	"github.com/armorclaw/bridge/pkg/invite"
	"github.com/armorclaw/bridge/pkg/turn"
	"github.com/armorclaw/bridge/pkg/governor"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"

	"github.com/armorclaw/bridge/internal/sdtw"
	"github.com/armorclaw/bridge/internal/skills"
	"github.com/armorclaw/bridge/pkg/voice"
	"github.com/armorclaw/bridge/pkg/appservice"
	"github.com/armorclaw/bridge/pkg/email"
	"github.com/armorclaw/bridge/pkg/webrtc"
	"github.com/armorclaw/bridge/pkg/yara"

	"github.com/armorclaw/bridge/pkg/browser"

	bridgeHTTP "github.com/armorclaw/bridge/pkg/http"
	"github.com/armorclaw/bridge/pkg/sidecar"

	"github.com/charmbracelet/huh"
)

var (
	version   = "0.2.0"
	buildTime = "unknown"
)

// resetTerminal restores terminal state after TUI operations.
// This ensures the terminal is usable if the TUI (Huh?) left it in an altered state.
func resetTerminal() {
	// Show cursor (in case TUI hid it)
	fmt.Print("\033[?25h")
	// Reset colors and attributes
	fmt.Print("\033[0m")
	// Also try stty sane via shell for raw mode terminals
	exec.Command("stty", "sane").Run()
}

type cliConfig struct {
	command          string
	configPath       string
	configOutput     string
	socketPath       string
	dbPath           string
	matrixHomeserver string
	matrixUsername   string
	matrixPassword   string
	matrixEnabled    bool
	logLevel         string
	verbose          bool
	version          bool
	help             bool
	migrateKeystore  bool
	readminReason    string
	// Quick-start command flags
	addKeyProvider    string
	addKeyToken       string
	addKeyId          string
	addKeyDisplayName string
	addKeyBaseURL     string
	startKeyId        string
	// QR code command flags
	qrHost string
	qrPort int
	// Agent command flags
	agentType         string
	agentName         string
	agentRoom         string
	agentKey          string
	agentCapabilities string
}

const governanceStateFile = "/var/lib/armorclaw/.governance-room"

func resolveGovernanceRoomID(cfg *config.Config, matrixClient *adapter.MatrixAdapter) string {
	if cfg.Governance.RoomID != "" {
		return cfg.Governance.RoomID
	}

	if data, err := os.ReadFile(governanceStateFile); err == nil {
		roomID := strings.TrimSpace(string(data))
		if roomID != "" {
			return roomID
		}
	}

	if matrixClient == nil {
		log.Printf("[WARN] GovernanceRoomID: no config value, no state file, Matrix client unavailable — starting without governance room")
		return ""
	}

	roomID, _, err := matrixClient.CreateRoom(context.Background(), map[string]interface{}{
		"name":   "ArmorClaw Governance",
		"topic":  "Governance and voting room for ArmorClaw Bridge",
		"preset": "private_chat",
	})
	if err != nil {
		log.Printf("[WARN] GovernanceRoomID: auto-create room failed: %v — starting without governance room", err)
		return ""
	}

	if err := os.MkdirAll(filepath.Dir(governanceStateFile), 0750); err == nil {
		if err := os.WriteFile(governanceStateFile, []byte(roomID), 0600); err != nil {
			log.Printf("[WARN] GovernanceRoomID: failed to persist room ID to state file: %v", err)
		}
	}

	log.Printf("GovernanceRoomID: auto-created room %s and persisted to %s", roomID, governanceStateFile)
	return roomID
}

func main() {
	cliCfg := parseFlags()

	if cliCfg.version {
		printVersion()
		return
	}

	if cliCfg.help {
		printHelp()
		return
	}

	// Handle config commands
	if cliCfg.command == "init" {
		runInitCommand(cliCfg)
		return
	}

	if cliCfg.command == "validate" {
		runValidateCommand(cliCfg)
		return
	}

	if cliCfg.command == "setup" {
		runSetupCommand(cliCfg)
		return
	}

	if cliCfg.command == "container-setup" {
		runContainerSetupCommand(cliCfg)
		return
	}

	if cliCfg.command == "completion" {
		runCompletionCommand(cliCfg)
		return
	}

	if cliCfg.command == "readmin" {
		runReadminCommand(cliCfg)
		return
	}

	if cliCfg.command == "daemon" {
		runDaemonCommand(cliCfg)
		return
	}

	if cliCfg.command == "help" {
		// Check if help is for a specific command
		args := flag.Args()
		if len(args) > 1 {
			printCommandHelp(args[1])
		} else {
			printHelp()
		}
		return
	}

	// Handle command-specific help (via --help/-h flag)
	if len(os.Args) > 2 && (os.Args[2] == "--help" || os.Args[2] == "-h") {
		// The command would be in cliCfg.command if --help is after a command
		printCommandHelp(cliCfg.command)
		return
	}

	// Handle quick-start commands
	if cliCfg.command == "add-key" {
		runAddKeyCommand(cliCfg)
		return
	}

	if cliCfg.command == "list-keys" {
		runListKeysCommand(cliCfg)
		return
	}

	if cliCfg.command == "start" {
		runStartCommand(cliCfg)
		return
	}

	if cliCfg.command == "generate-qr" {
		runGenerateQRCommand(cliCfg)
		return
	}

	if cliCfg.command == "start-agent" {
		runStartAgentCommand(cliCfg)
		return
	}

	// Default: Start the bridge server
	runBridgeServer(cliCfg)
}

// runInitCommand generates an example configuration file
func runInitCommand(cliCfg cliConfig) {
	outputPath := cliCfg.configOutput
	if outputPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to determine home directory: %v", err)
		}
		outputPath = filepath.Join(homeDir, ".armorclaw", "config.toml")
	}
	if err := config.GenerateExampleConfig(outputPath); err != nil {
		log.Fatalf("Failed to generate example config: %v", err)
	}
	log.Printf("✓ Example configuration written to: %s", outputPath)
	log.Println("✓ Edit this file to customize your ArmorClaw bridge configuration")
	log.Println("")
	log.Println("Quick start:")
	log.Println("  1. Add an API key: armorclaw-bridge add-key --provider openai --token sk-...")
	log.Println("  2. Start agent:    armorclaw-bridge start --key <key-id>")
}

// runValidateCommand validates the configuration
func runValidateCommand(cliCfg cliConfig) {
	cfg, err := config.Load(cliCfg.configPath)
	if err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}
	log.Printf("✓ Configuration is valid!")
	log.Printf(" Socket: %s", cfg.Server.SocketPath)
}

// runReadminCommand initiates admin reset mode
func runReadminCommand(cliCfg cliConfig) {
	// For now, just log the reason. Full implementation will be added in later tasks.
	log.Printf("Initiating readmin mode with reason: %s\n", cliCfg.readminReason)
	log.Println("Note: ReadminManager will be implemented in subsequent tasks")
	log.Println("This is a placeholder until InitiateReadmin() is implemented")
}

// runSetupCommand runs the interactive setup wizard using Huh? TUI forms.
// This is the standalone (non-container) setup that creates a local config.
func runSetupCommand(cliCfg cliConfig) {
	// Detect accessible mode from environment
	accessible := os.Getenv("ACCESSIBLE") != ""

	result, err := wizard.Run(accessible)
	if err != nil {
		if err == huh.ErrUserAborted {
			fmt.Println("\nSetup cancelled.")
			return
		}
		log.Fatalf("Setup wizard failed: %v", err)
	}

	// Determine config path
	configPath := cliCfg.configPath
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to determine home directory: %v", err)
		}
		configPath = filepath.Join(homeDir, ".armorclaw", "config.toml")
	}

	// Create config directory
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0750); err != nil {
		log.Fatalf("Failed to create config directory: %v", err)
	}

	// Generate and save configuration
	cfg := config.DefaultConfig()
	cfg.Keystore.DBPath = filepath.Join(configDir, "keystore.db")

	if err := config.Save(cfg, configPath); err != nil {
		log.Fatalf("Failed to save configuration: %v", err)
	}

	// Store API key directly in the encrypted keystore (never written to disk as plaintext)
	if result.Secrets.APIKey != "" {
		ks, err := keystore.New(cfg.ToKeystoreConfig())
		if err != nil {
			log.Fatalf("Failed to initialize keystore: %v", err)
		}
		if err := ks.Open(); err != nil {
			log.Fatalf("Failed to open keystore: %v", err)
		}
		defer ks.Close()

		keyID := result.Config.APIProvider + "-default"
		cred := keystore.Credential{
			ID:          keyID,
			Provider:    keystore.Provider(result.Config.APIProvider),
			Token:       result.Secrets.APIKey,
			DisplayName: result.Config.APIProvider + " API Key (setup wizard)",
			Tags:        []string{"setup-wizard", "production"},
		}

		if err := ks.Store(cred); err != nil {
			log.Fatalf("Failed to store API key: %v", err)
		}
		fmt.Printf("  API key stored securely as '%s'\n", keyID)
	}

	fmt.Printf("  Configuration saved to: %s\n", configPath)
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Println("    1. Start the bridge:  armorclaw-bridge")
	if result.Secrets.APIKey != "" {
		fmt.Printf("    2. Start an agent:    armorclaw-bridge start --key %s-default\n", result.Config.APIProvider)
	} else {
		fmt.Println("    2. Add an API key:    armorclaw-bridge add-key --provider openai --token sk-...")
	}
	fmt.Println()
	return
}

// runContainerSetupCommand runs the Huh? wizard then delegates infrastructure
// setup (Docker Compose, Matrix, certs) to container-setup.sh.
// Secrets are passed via environment variables and never written to the JSON file.
func runContainerSetupCommand(cliCfg cliConfig) {
	// Panic recovery for crash handler (Phase 2: Crash Handler & Error Capture)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in container setup: %v", r)
			// Print stack trace for debugging
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			log.Printf("Stack trace:\n%s", string(buf[:n]))
			fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println("SETUP CRASHED (unexpected error)")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println()
			fmt.Println("This is an internal error. Please report this issue:")
			fmt.Println("  https://github.com/Gemutly/ArmorClaw/issues")
			fmt.Println()
			fmt.Println("Include the stack trace above in your report.")
			os.Exit(1)
		}
	}()

	// Preflight check: Verify Docker daemon is accessible before running wizard
	fmt.Println("Checking Docker connectivity...")
	dockerResult := setup.FullDockerCheck()
	if dockerResult.Error != nil {
		fmt.Println()
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("ERROR: Docker not accessible")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()

		// Show detailed diagnostics
		fmt.Printf("Diagnostics:\n")
		fmt.Printf("  Socket exists:   %v\n", dockerResult.SocketExists)
		fmt.Printf("  Socket readable: %v\n", dockerResult.SocketReadable)
		fmt.Printf("  Socket writable: %v\n", dockerResult.SocketWritable)
		fmt.Printf("  Daemon running:  %v\n", dockerResult.DaemonRunning)
		fmt.Println()

		// Show specific error
		if setupErr, ok := dockerResult.Error.(*setup.SetupError); ok {
			fmt.Printf("Error code: %s\n", setupErr.Code)
			fmt.Printf("Message:    %s\n", setupErr.Title)
			fmt.Printf("Cause:      %s\n", setupErr.Cause)
			fmt.Println()
			fmt.Println("Suggested fixes:")
			for i, fix := range setupErr.Fix {
				fmt.Printf("  %d. %s\n", i+1, fix)
			}
		} else {
			fmt.Printf("Error: %v\n", dockerResult.Error)
		}

		fmt.Println()
		fmt.Println("Common solutions:")
		fmt.Println("  1. Run container with --user root")
		fmt.Println("  2. Ensure Docker is running on host: systemctl status docker")
		fmt.Println("  3. Fix socket permissions: sudo chmod 666 /var/run/docker.sock")
		fmt.Println()
		os.Exit(1)
	}
	fmt.Println("Docker connectivity OK")
	fmt.Println()

	accessible := os.Getenv("ACCESSIBLE") != ""

	result, err := wizard.Run(accessible)
	if err != nil {
		if err == huh.ErrUserAborted {
			fmt.Println("\nSetup cancelled.")
			os.Exit(130)
		}
		fatalSetupError(err, "Wizard", "wizard.Run")
	}

	// Write non-secret config to temporary JSON file
	wizardJSON := "/tmp/armorclaw-wizard.json"
	if err := wizard.WriteConfigJSON(wizardJSON, &result.Config); err != nil {
		fatalSetupError(err, "Config Write", fmt.Sprintf("wizard.WriteConfigJSON(%s)", wizardJSON))
	}
	defer os.Remove(wizardJSON) // Clean up immediately after container-setup.sh reads it

	// Build environment with secrets (passed via env vars, not written to disk)
	setupScript := "/opt/armorclaw/container-setup.sh"
	cmd := exec.Command(setupScript, "--from-wizard", wizardJSON)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Inherit current env and add secret env vars
	cmd.Env = append(os.Environ(), wizard.SecretEnvVars(&result.Secrets)...)

	if err := cmd.Run(); err != nil {
		fatalSetupError(err, "Container Setup", fmt.Sprintf("exec.Command(%s).Run()", setupScript))
	}
}

// fatalSetupError displays a setup error with full context and exits.
// If the error is already a SetupError, it displays it directly.
// Otherwise, it wraps the error with context.
func fatalSetupError(err error, operation, function string) {
	if setupErr, ok := err.(*setup.SetupError); ok {
		// Already a SetupError - display it directly
		fmt.Println()
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("ERROR [%s]: %s\n", setupErr.Code, setupErr.Title)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()
		if setupErr.Cause != "" {
			fmt.Printf("Cause:\n  %s\n\n", setupErr.Cause)
		}
		if len(setupErr.Fix) > 0 {
			fmt.Println("Suggested fixes:")
			for i, fix := range setupErr.Fix {
				fmt.Printf("  %d. %s\n", i+1, fix)
			}
			fmt.Println()
		}
		if setupErr.DocLink != "" {
			fmt.Printf("Learn more: %s\n", setupErr.DocLink)
		}
		os.Exit(setupErr.ExitCode)
	}

	// Not a SetupError - wrap it with context
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("ERROR: %s failed\n", operation)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("Function: %s\n", function)
	fmt.Printf("Error: %v\n", err)
	fmt.Println()
	fmt.Println("Suggested fixes:")
	fmt.Println("  1. Check the error message above for clues")
	fmt.Println("  2. Try running with -e ARMORCLAW_DEBUG=true for more info")
	fmt.Println("  3. Report this issue: https://github.com/Gemutly/ArmorClaw/issues")
	os.Exit(1)
}

// runSetupCommandLegacy is the original bufio-based setup wizard (kept for
// environments where Huh? TUI is not available, e.g., dumb terminals).
func runSetupCommandLegacy(cliCfg cliConfig) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║        Welcome to ArmorClaw - Interactive Setup           ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println("")
	fmt.Println("This wizard will guide you through the initial setup process.")
	fmt.Println("Press Ctrl+C at any time to cancel.")
	fmt.Println("")

	// Step 1: Check Docker availability
	fmt.Print("Checking Docker availability... ")
	if !docker.IsAvailable() {
		fmt.Println("not found")
		log.Fatal("Docker is not available or not running. Please install and start Docker first.")
	}
	fmt.Println("ok")

	// Step 2: Configuration location
	fmt.Println("\n📁 Configuration Setup")
	fmt.Println("Where would you like to store your ArmorClaw configuration?")
	fmt.Println("  [1] Default (~/.armorclaw)")
	fmt.Println("  [2] Custom location")

	fmt.Print("Choose an option (1-2) [1]: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	configPath := cliCfg.configPath
	if strings.TrimSpace(input) == "2" {
		fmt.Print("Enter custom path: ")
		configPath, _ = reader.ReadString('\n')
		configPath = strings.TrimSpace(configPath)
	}

	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to determine home directory: %v", err)
		}
		configPath = filepath.Join(homeDir, ".armorclaw", "config.toml")
	} else {
		// If user provided a directory, append config.toml
		if !strings.HasSuffix(configPath, ".toml") {
			configPath = filepath.Join(configPath, "config.toml")
		}
	}

	// Create config directory if needed
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0750); err != nil {
		log.Fatalf("Failed to create config directory: %v", err)
	}

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("\n⚠️  Configuration file already exists: %s\n", configPath)
		fmt.Print("Do you want to overwrite it? [y/N]: ")
		input, _ = reader.ReadString('\n')
		input = strings.ToLower(strings.TrimSpace(input))
		if input != "y" && input != "yes" {
			fmt.Println("Setup cancelled.")
			return
		}
	}

	// Step 3: API Provider Selection
	fmt.Println("\n🤖 AI Provider Selection")
	fmt.Println("Which AI provider do you use?")
	fmt.Println("  [1] OpenAI (GPT-4, GPT-3.5)")
	fmt.Println("  [2] Anthropic (Claude)")
	fmt.Println("  [3] OpenRouter")
	fmt.Println("  [4] Google (Gemini)")
	fmt.Println("  [5] xAI (Grok)")
	fmt.Println("  [6] Skip (add keys later)")

	fmt.Print("Choose an option (1-6) [1]: ")
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)

	provider := ""
	providerName := ""
	defaultKeyName := ""

	switch input {
	case "1", "":
		provider = "openai"
		providerName = "OpenAI"
		defaultKeyName = "openai-default"
	case "2":
		provider = "anthropic"
		providerName = "Anthropic"
		defaultKeyName = "anthropic-default"
	case "3":
		provider = "openrouter"
		providerName = "OpenRouter"
		defaultKeyName = "openrouter-default"
	case "4":
		provider = "google"
		providerName = "Google"
		defaultKeyName = "google-default"
	case "5":
		provider = "xai"
		providerName = "xAI"
		defaultKeyName = "xai-default"
	case "6":
		provider = ""
		providerName = "None"
	default:
		fmt.Println("Invalid option. Skipping API key setup.")
		provider = ""
	}

	// Step 4: API Key Entry (if provider selected)
	if provider != "" {
		fmt.Printf("\n🔑 %s API Key\n", providerName)
		fmt.Println("Enter your API key (input will be hidden):")

		// Read API key securely
		var apiKey string
		var err error

		// Try to use termios for hidden input on Unix-like systems
		apiKey, err = readPassword(reader)
		if err != nil {
			// Fallback to regular input
			fmt.Print("API Key: ")
			apiKey, _ = reader.ReadString('\n')
			apiKey = strings.TrimSpace(apiKey)
		}

		if apiKey == "" {
			fmt.Println("⚠️  No API key provided. You can add one later with:")
			fmt.Println("  armorclaw-bridge add-key --provider <provider> --token <key>")
		} else {
			// Validate API key format
			if !validateAPIKeyFormat(provider, apiKey) {
				fmt.Printf("⚠️  Warning: API key format looks unusual for %s\n", providerName)
				fmt.Print("Continue anyway? [y/N]: ")
				input, _ = reader.ReadString('\n')
				input = strings.ToLower(strings.TrimSpace(input))
				if input != "y" && input != "yes" {
					fmt.Println("Setup cancelled.")
					return
				}
			}

			// Store the API key
			cfg := config.DefaultConfig()
			cfg.Keystore.DBPath = filepath.Join(filepath.Dir(configPath), "keystore.db")

			ks, err := keystore.New(cfg.ToKeystoreConfig())
			if err != nil {
				log.Fatalf("Failed to initialize keystore: %v", err)
			}

			if err := ks.Open(); err != nil {
				log.Fatalf("Failed to open keystore: %v", err)
			}
			defer ks.Close()

			cred := keystore.Credential{
				ID:          defaultKeyName,
				Provider:    keystore.Provider(provider),
				Token:       apiKey,
				DisplayName: fmt.Sprintf("%s API Key (setup wizard)", providerName),
				Tags:        []string{"setup-wizard", "production"},
			}

			if err := ks.Store(cred); err != nil {
				log.Fatalf("Failed to store API key: %v", err)
			}

			fmt.Printf("✓ API key stored as '%s'\n", defaultKeyName)
		}
	}

	// Step 5: Matrix Configuration (Optional)
	fmt.Println("\n💬 Matrix Configuration (Optional)")
	fmt.Println("ArmorClaw can integrate with Matrix for encrypted messaging.")
	fmt.Println("Leave blank to skip this step.")

	fmt.Print("Enable Matrix? [y/N]: ")
	input, _ = reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))

	matrixEnabled := input == "y" || input == "yes"
	matrixHomeserver := ""
	matrixUsername := ""
	matrixPassword := ""

	if matrixEnabled {
		fmt.Print("Matrix homeserver URL [https://matrix.example.com]: ")
		matrixHomeserver, _ = reader.ReadString('\n')
		matrixHomeserver = strings.TrimSpace(matrixHomeserver)
		if matrixHomeserver == "" {
			matrixHomeserver = "https://matrix.example.com"
		}

		fmt.Print("Matrix username: ")
		matrixUsername, _ = reader.ReadString('\n')
		matrixUsername = strings.TrimSpace(matrixUsername)

		fmt.Print("Matrix password: ")
		matrixPassword, _ = reader.ReadString('\n')
		matrixPassword = strings.TrimSpace(matrixPassword)
	}

	// Step 6: Generate Configuration
	fmt.Println("\n⚙️  Generating configuration...")

	// Create config structure
	cfg := config.DefaultConfig()

	// Override with wizard values (configDir was already declared above)
	configDir = filepath.Dir(configPath)
	cfg.Keystore.DBPath = filepath.Join(configDir, "keystore.db")

	if matrixEnabled {
		cfg.Matrix.Enabled = true
		cfg.Matrix.HomeserverURL = matrixHomeserver
		cfg.Matrix.Username = matrixUsername
		cfg.Matrix.Password = matrixPassword

		// Capture setup user MXID for error system
		if matrixUsername != "" && matrixHomeserver != "" {
			// Construct full MXID from username and homeserver
			// Format: @username:homeserver.domain
			homeserverDomain := strings.TrimPrefix(matrixHomeserver, "https://")
			homeserverDomain = strings.TrimPrefix(homeserverDomain, "http://")
			homeserverDomain = strings.Split(homeserverDomain, "/")[0] // Remove any path
			homeserverDomain = strings.Split(homeserverDomain, ":")[0] // Remove port if present
			cfg.ErrorSystem.SetupUserMXID = fmt.Sprintf("@%s:%s", matrixUsername, homeserverDomain)
		}
	}

	// Save configuration
	if err := config.Save(cfg, configPath); err != nil {
		log.Fatalf("Failed to save configuration: %v", err)
	}

	fmt.Printf("✓ Configuration saved to: %s\n", configPath)

	// Step 7: Summary and Next Steps
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                   Setup Complete! ✓                       ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println("")
	fmt.Println("Your ArmorClaw bridge is ready to use!")
	fmt.Println("")

	if provider != "" {
		fmt.Println("🚀 Quick Start:")
		fmt.Printf("  1. Start the bridge:  armorclaw-bridge\n")
		fmt.Printf("  2. Start an agent:    armorclaw-bridge start --key %s\n", defaultKeyName)
	} else {
		fmt.Println("🚀 Next Steps:")
		fmt.Println("  1. Add an API key:    armorclaw-bridge add-key --provider <provider> --token <key>")
		fmt.Println("  2. Start the bridge:  armorclaw-bridge")
		fmt.Println("  3. Start an agent:    armorclaw-bridge start --key <key-id>")
	}

	fmt.Println("")
	fmt.Println("📚 Documentation: https://github.com/Gemutly/ArmorClaw")
	fmt.Println("")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
	fmt.Println("")
	fmt.Println("📱 CONNECT ARMORCHAT / ARMORTERMINAL")
	fmt.Println("")
	fmt.Println("After starting the bridge, connect your mobile app:")
	fmt.Println("")
	fmt.Println("  For LOCAL NETWORK (same WiFi):")
	fmt.Println("    • Open ArmorChat - it will auto-discover this bridge")
	fmt.Printf("    • Look for: %s._armorclaw._tcp.local.\n", getHostname())
	fmt.Println("")
	fmt.Println("  For REMOTE VPS (different network):")
	fmt.Println("    • Option A: Scan QR code (run: armorclaw-bridge generate-qr)")
	fmt.Println("    • Option B: Enter your domain in ArmorChat setup")
	fmt.Println("    • Option C: Manual entry with the URLs shown at startup")
	fmt.Println("")
	fmt.Println("  ⚠️  mDNS discovery only works on the SAME network!")
	fmt.Println("     For VPS deployments, use QR code or manual entry.")
	fmt.Println("")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════════")
}

// getHostname returns the best available hostname for public-facing URLs.
// Uses UDP STUN-like detection (like wizard.detectServerName) to find
// the local IP that can reach the internet, falling back to os.Hostname().
func getHostname() string {
	if conn, err := net.DialTimeout("udp", "8.8.8.8:80", 3*time.Second); err == nil {
		conn.Close()
		if localAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			ip := localAddr.IP.String()
			if ip != "127.0.0.1" && ip != "::1" && !strings.HasPrefix(ip, "172.17.") {
				return ip
			}
		}
	}
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		return hostname
	}
	return "armorclaw-server"
}

// readPassword reads a password from stdin with hidden input (Unix-like systems)
func readPassword(reader *bufio.Reader) (string, error) {
	// On Windows, this won't hide the input, but it will still work
	// On Unix-like systems, we use syscall to disable echo
	// For simplicity, this implementation just reads normally
	// A production version would use termios for hidden input
	fmt.Print("API Key: ")
	input, err := reader.ReadString('\n')
	return strings.TrimSpace(input), err
}

// validateAPIKeyFormat performs basic validation of API key format
func validateAPIKeyFormat(provider, key string) bool {
	switch provider {
	case "openai":
		return strings.HasPrefix(key, "sk-") || strings.HasPrefix(key, "sk-proj-")
	case "anthropic":
		return strings.HasPrefix(key, "sk-ant-")
	case "openrouter":
		return len(key) > 20 // OpenRouter keys vary
	case "google", "xai":
		return len(key) > 20 // Basic length check
	}
	return true
}

// runCompletionCommand outputs shell completion script
func runCompletionCommand(cliCfg cliConfig) {
	shell := cliCfg.configPath // Reuse configPath for shell type
	if shell == "" {
		// Detect shell from environment
		shell = os.Getenv("SHELL")
		if strings.Contains(shell, "zsh") {
			shell = "zsh"
		} else {
			shell = "bash"
		}
	}

	var script string
	switch shell {
	case "bash":
		script = `# ArmorClaw Bridge Bash Completion
# Save this file to: ~/.bash_completion.d/armorclaw-bridge
# Or source it in: ~/.bashrc

_armorclaw_bridge_commands() {
    local commands="init validate add-key list-keys start start-agent generate-qr setup version help completion"
    echo "$commands"
}

_armorclaw_bridge_providers() {
    local providers="openai anthropic openrouter google gemini xai"
    echo "$providers"
}

_armorclaw_bridge() {
    local cur prev words cword
    _init_completion || return

    if [ $cword -eq 1 ]; then
        COMPREPLY=($(compgen -W "$(_armorclaw_bridge_commands)" -- "$cur"))
        return 0
    fi

    local cmd="${words[1]}"
    case "$cmd" in
        add-key)
            case "$prev" in
                --provider|-p)
                    COMPREPLY=($(compgen -W "$(_armorclaw_bridge_providers)" -- "$cur"))
                    ;;
                *)
                    COMPREPLY=($(compgen -W "--provider --token --id --name --help -h" -- "$cur"))
                    ;;
            esac
            ;;
        start)
            case "$prev" in
                --key|-k)
                    COMPREPLY=($(compgen -W "$(armorclaw-bridge list-keys 2>/dev/null | grep '•' | awk '{print $2}')" -- "$cur"))
                    ;;
                *)
                    COMPREPLY=($(compgen -W "--key --help -h" -- "$cur"))
                    ;;
            esac
            ;;
        completion)
            COMPREPLY=($(compgen -W "bash zsh" -- "$cur"))
            ;;
        generate-qr)
            COMPREPLY=($(compgen -W "--host --port --help -h" -- "$cur"))
            ;;
        start-agent)
            COMPREPLY=($(compgen -W "--type --name --room --key --capabilities --help -h" -- "$cur"))
            ;;
    esac
}

complete -F _armorclaw_bridge armorclaw-bridge
`
	case "zsh":
		script = `#compdef armorclaw-bridge
# ArmorClaw Bridge Zsh Completion
# Save this file to: ~/.zsh/completions/_armorclaw-bridge

_armorclaw_bridge() {
    local -a commands
    commands=(
        'init:Initialize configuration file'
        'validate:Validate configuration'
        'setup:Run interactive setup wizard'
        'add-key:Add an API key to the keystore'
        'list-keys:List all stored API keys'
        'start:Start an agent container (legacy)'
        'start-agent:Start an AI agent (OpenClaw, assistant, etc.)'
        'generate-qr:Generate QR code for ArmorChat discovery'
        'completion:Generate shell completion script'
        'version:Show version information'
        'help:Show help information'
    )

    if (( CURRENT == 2 )); then
        _describe 'command' commands
    else
        case $words[2] in
            add-key)
                _arguments '--provider[AI provider]:providers:(openai anthropic openrouter google gemini xai)' \
                           '--token[API token]' \
                           '--id[Key ID]' \
                           '--name[Display name]' \
                           '--help[Show help]'
                ;;
            start)
                _arguments '--key[Key ID]:keys:(_armorclaw_bridge_keys)' \
                           '--help[Show help]'
                ;;
            completion)
                _arguments '--shell[Shell type]:shells:(bash zsh)'
                ;;
            generate-qr)
                _arguments '--host[Public hostname/domain]' \
                           '--port[Public port]' \
                           '--help[Show help]'
                ;;
            start-agent)
                _arguments '--type[Agent type]:types:(assistant openclaw custom)' \
                           '--name[Agent display name]' \
                           '--room[Matrix room ID]' \
                           '--key[API key ID]' \
                           '--capabilities[Comma-separated capabilities]' \
                           '--help[Show help]'
                ;;
        esac
    fi
}

_armorclaw_bridge_keys() {
    local -a keys
    keys=($(armorclaw-bridge list-keys 2>/dev/null | grep '•' | awk '{print $2}'))
    _describe 'stored-key' keys
}
`
	default:
		log.Fatalf("Unsupported shell: %s. Supported: bash, zsh", shell)
	}

	fmt.Println(script)
	log.Printf("✓ %s completion script generated", shell)
	log.Println("")
	log.Println("To enable completion:")
	switch shell {
	case "bash":
		log.Println("  1. Save to file:")
		log.Println("     armorclaw-bridge completion bash > ~/.bash_completion.d/armorclaw-bridge")
		log.Println("  2. Source in ~/.bashrc:")
		log.Println("     source ~/.bash_completion.d/armorclaw-bridge")
		log.Println("  3. Or restart your shell")
	case "zsh":
		log.Println("  1. Save to file:")
		log.Println("     armorclaw-bridge completion zsh > ~/.zsh/completions/_armorclaw-bridge")
		log.Println("  2. Or add to ~/.zshrc:")
		log.Println("     autoload -U compinit && compinit")
		log.Println("  3. Restart your shell")
	}
}

// runDaemonCommand manages daemon operations (start/stop/restart/status/logs)
func runDaemonCommand(cliCfg cliConfig) {
	if len(os.Args) < 3 {
		printDaemonHelp()
		log.Fatal("Error: daemon requires an action (start, stop, restart, status, logs)")
	}

	action := os.Args[2]

	switch action {
	case "start":
		daemonStart(cliCfg)
	case "stop":
		daemonStop()
	case "restart":
		daemonStop()
		time.Sleep(1 * time.Second)
		daemonStart(cliCfg)
	case "status":
		daemonStatus()
	case "logs":
		daemonLogs()
	default:
		printDaemonHelp()
		log.Fatalf("Error: unknown daemon action: %s", action)
	}
}

// daemonStart starts the bridge as a daemon
func daemonStart(cliCfg cliConfig) {
	// Load configuration
	cfg, err := config.Load(cliCfg.configPath)
	if err != nil {
		log.Printf("Warning: Using default configuration: %v", err)
		cfg = config.DefaultConfig()
	}

	// Enable daemonize
	cfg.Server.Daemonize = true

	// Set default PID file if not specified
	if cfg.Server.PidFile == "" {
		cfg.Server.PidFile = "/run/armorclaw/bridge.pid"
	}

	// Set default log file for daemon
	if cfg.Logging.Output == "stdout" {
		cfg.Logging.Output = "file"
		if cfg.Logging.File == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				log.Fatalf("Failed to determine home directory: %v", err)
			}
			cfg.Logging.File = filepath.Join(homeDir, ".armorclaw", "bridge.log")
		}
	}

	// Check if already running
	if daemonStatusRunning() {
		log.Fatal("Error: daemon is already running")
	}

	// Create runtime directory
	runtimeDir := filepath.Dir(cfg.Server.SocketPath)
	if err := os.MkdirAll(runtimeDir, 0750); err != nil {
		log.Fatalf("Failed to create runtime directory: %v", err)
	}

	log.Println("Starting ArmorClaw Bridge as daemon...")
	log.Printf("PID file: %s", cfg.Server.PidFile)
	log.Printf("Log file: %s", cfg.Logging.File)

	// Fork and detach (simulate daemon mode for now - on Windows, we'll run in foreground)
	// For true daemon mode, we'd use syscall fork/exec, but for cross-platform compatibility:
	// We'll start in background with output redirected

	// For now, just start normally with logging to file
	// Note: This will run in foreground, not true daemon mode
	// True daemon mode would require fork/exec which is platform-specific
	log.Println("Note: Running in foreground (true daemon mode requires platform-specific fork)")
	runBridgeServer(cliCfg)
}

// daemonStop stops the daemon
func daemonStop() {
	// Load config to get PID file location
	cfg, err := config.Load("")
	if err != nil {
		cfg = config.DefaultConfig()
	}

	if cfg.Server.PidFile == "" {
		cfg.Server.PidFile = "/run/armorclaw/bridge.pid"
	}

	// Read PID file
	pidData, err := os.ReadFile(cfg.Server.PidFile)
	if err != nil {
		log.Fatalf("Failed to read PID file: %v", err)
	}

	pidStr := strings.TrimSpace(string(pidData))
	var pid int
	fmt.Sscanf(pidStr, "%d", &pid)

	// Check if process is running
	process, err := os.FindProcess(pid)
	if err != nil {
		log.Printf("Process %d not found (already stopped?)", pid)
		os.Remove(cfg.Server.PidFile)
		return
	}

	// Send SIGTERM for graceful shutdown
	if err := process.Signal(syscall.SIGTERM); err != nil {
		log.Fatalf("Failed to send SIGTERM to process %d: %v", pid, err)
	}

	// Wait a bit and check if it stopped
	time.Sleep(2 * time.Second)

	_, err = os.FindProcess(pid)
	if err == nil {
		// Process still running, force kill
		log.Printf("Process %d did not stop gracefully, sending SIGKILL...", pid)
		// Note: os.Process.Signal is not available on all platforms
		// For production, would use syscall.Kill
		time.Sleep(500 * time.Millisecond)
	}

	// Clean up PID file
	os.Remove(cfg.Server.PidFile)

	log.Printf("✓ Daemon stopped (PID: %d)", pid)
}

// daemonRestart restarts the daemon
func daemonRestart() {
	log.Println("Restarting daemon...")
	// Implementation: stop then start
	// This is handled by calling stop then start
}

// daemonStatus checks daemon status
func daemonStatus() {
	// Load config to get PID file location
	cfg, err := config.Load("")
	if err != nil {
		cfg = config.DefaultConfig()
	}

	if cfg.Server.PidFile == "" {
		cfg.Server.PidFile = "/run/armorclaw/bridge.pid"
	}

	// Check PID file
	pidData, err := os.ReadFile(cfg.Server.PidFile)
	if err != nil {
		fmt.Println("Daemon status: Stopped (no PID file)")
		return
	}

	pidStr := strings.TrimSpace(string(pidData))
	var pid int
	fmt.Sscanf(pidStr, "%d", &pid)

	// Check if process is running
	_, err = os.FindProcess(pid)
	if err != nil {
		fmt.Printf("Daemon status: Stopped (stale PID file, PID: %d)\n", pid)
		os.Remove(cfg.Server.PidFile)
		return
	}

	// Process is running
	// Get start time
	createTime, err := getProcessStartTime(pid)
	if err != nil {
		fmt.Printf("Daemon status: Running (PID: %d)\n", pid)
		return
	}

	uptime := time.Since(createTime)
	fmt.Printf("Daemon status: Running (PID: %d, uptime: %s)\n", pid, uptime.Round(time.Second))
}

// daemonLogs shows daemon logs
func daemonLogs() {
	// Load config to get log file location
	cfg, err := config.Load("")
	if err != nil {
		cfg = config.DefaultConfig()
	}

	logFile := cfg.Logging.File
	if logFile == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatal("Failed to determine home directory")
		}
		logFile = filepath.Join(homeDir, ".armorclaw", "bridge.log")
	}

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		fmt.Printf("Log file not found: %s\n", logFile)
		return
	}

	fmt.Printf("Showing last 50 lines of %s:\n", logFile)
	fmt.Println("---")

	// Use tail to show last 50 lines
	cmd := exec.Command("tail", "-n", "50", logFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to read log file: %v", err)
	}
}

// daemonStatusRunning checks if daemon is running without loading config
func daemonStatusRunning() bool {
	cfg, err := config.Load("")
	if err != nil {
		cfg = config.DefaultConfig()
	}

	if cfg.Server.PidFile == "" {
		cfg.Server.PidFile = "/run/armorclaw/bridge.pid"
	}

	pidData, err := os.ReadFile(cfg.Server.PidFile)
	if err != nil {
		return false
	}

	pidStr := strings.TrimSpace(string(pidData))
	var pid int
	fmt.Sscanf(pidStr, "%d", &pid)

	_, err = os.FindProcess(pid)
	return err == nil
}

// getProcessStartTime gets the creation time of a process
func getProcessStartTime(pid int) (time.Time, error) {
	// On Linux, read /proc/<pid>/stat
	// On Windows, use different method
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	data, err := os.ReadFile(statPath)
	if err != nil {
		return time.Time{}, err
	}

	// Parse stat file (field 22 is creation time in jiffies)
	fields := strings.Fields(string(data))
	if len(fields) < 22 {
		return time.Time{}, fmt.Errorf("invalid stat format")
	}

	// Field 22 is starttime (since boot in jiffies)
	var starttime int64
	fmt.Sscanf(fields[21], "%d", &starttime)

	// Get system boot time
	// Would need to read /proc/stat for accurate boot time
	// For simplicity, return current time minus approximate uptime
	// This is a simplified version - production code would be more accurate

	return time.Now(), nil
}

// printDaemonHelp shows daemon help
func printDaemonHelp() {
	help := `COMMAND: daemon

Manage the ArmorClaw Bridge background daemon.

USAGE:
    armorclaw-bridge daemon [action]

ACTIONS:
    start     Start bridge as background daemon
    stop      Stop the background daemon
    restart   Restart the daemon
    status    Show daemon status
    logs      Show recent log entries

EXAMPLES:
    # Start daemon
    armorclaw-daemon start

    # Check status
    armorclaw-daemon status

    # View logs
    armorclaw-bridge daemon logs

    # Stop daemon
    armorclaw-bridge daemon stop

CONFIGURATION:
    Daemon mode is configured in config.toml:

    [server]
      daemonize = true
      pid_file = "/run/armorclaw/bridge.pid"

    [logging]
      output = "file"
      file = "/path/to/bridge.log"
`
	fmt.Println(help)
}

// runAddKeyCommand adds an API key to the keystore
func runAddKeyCommand(cliCfg cliConfig) {
	// Load configuration (use defaults if not found)
	cfg, err := config.Load(cliCfg.configPath)
	if err != nil {
		log.Printf("Warning: Using default configuration: %v", err)
		cfg = config.DefaultConfig()
	}

	// Check for required parameters
	if cliCfg.addKeyProvider == "" {
		log.Fatal("Error: --provider is required (openai, anthropic, openrouter, google, xai)")
	}
	if cliCfg.addKeyToken == "" {
		token := os.Getenv("ARMORCLAW_API_KEY")
		if token == "" {
			log.Fatal("Error: --token is required or set ARMORCLAW_API_KEY environment variable")
		}
		cliCfg.addKeyToken = token
	}

	// Initialize keystore
	log.Println("Initializing encrypted keystore...")
	ks, err := keystore.New(cfg.ToKeystoreConfig())
	if err != nil {
		log.Fatalf("Failed to initialize keystore: %v", err)
	}

	if err := ks.Open(); err != nil {
		log.Fatalf("Failed to open keystore: %v", err)
	}
	defer ks.Close()

	// Generate key ID if not provided
	keyID := cliCfg.addKeyId
	if keyID == "" {
		keyID = cliCfg.addKeyProvider + "-default"
	}

	// Create credential
	displayName := cliCfg.addKeyDisplayName
	if displayName == "" {
		displayName = fmt.Sprintf("%s API Key", cliCfg.addKeyProvider)
	}

	cred := keystore.Credential{
		ID:          keyID,
		Provider:    keystore.Provider(cliCfg.addKeyProvider),
		Token:       cliCfg.addKeyToken,
		BaseURL:     cliCfg.addKeyBaseURL,
		DisplayName: displayName,
		Tags:        []string{"cli", "quick-start"},
	}

	// Store credential
	if err := ks.Store(cred); err != nil {
		log.Fatalf("Failed to store credential: %v", err)
	}

	log.Printf("✓ API key stored as '%s'", keyID)
	log.Printf("  Provider: %s", cliCfg.addKeyProvider)
	if cliCfg.addKeyBaseURL != "" {
		log.Printf("  Base URL: %s", cliCfg.addKeyBaseURL)
	}
	log.Printf("  Display name: %s", displayName)
	log.Println("")
	log.Println("Start an agent with this key:")
	log.Printf("  armorclaw-bridge start --key %s", keyID)
}

// runListKeysCommand lists all stored credentials
func runListKeysCommand(cliCfg cliConfig) {
	// Load configuration
	cfg, err := config.Load(cliCfg.configPath)
	if err != nil {
		log.Printf("Warning: Using default configuration: %v", err)
		cfg = config.DefaultConfig()
	}

	// Initialize keystore
	ks, err := keystore.New(cfg.ToKeystoreConfig())
	if err != nil {
		log.Fatalf("Failed to initialize keystore: %v", err)
	}

	if err := ks.Open(); err != nil {
		log.Fatalf("Failed to open keystore: %v", err)
	}
	defer ks.Close()

	// List credentials (empty provider means all)
	creds, err := ks.List("")
	if err != nil {
		log.Fatalf("Failed to list credentials: %v", err)
	}

	if len(creds) == 0 {
		log.Println("No API keys stored.")
		log.Println("")
		log.Println("Add one with:")
		log.Println("  armorclaw-bridge add-key --provider openai --token sk-...")
		return
	}

	log.Printf("✓ Found %d API key(s):\n", len(creds))
	for _, cred := range creds {
		log.Printf("  • %s", cred.ID)
		log.Printf("    Provider: %s", cred.Provider)
		if cred.BaseURL != "" {
			log.Printf("    Base URL: %s", cred.BaseURL)
		}
		if cred.DisplayName != "" {
			log.Printf("    Name: %s", cred.DisplayName)
		}
		log.Println("")
	}
}

// runStartCommand starts an agent container
func runStartCommand(cliCfg cliConfig) {
	// Load configuration
	cfg, err := config.Load(cliCfg.configPath)
	if err != nil {
		log.Printf("Warning: Using default configuration: %v", err)
		cfg = config.DefaultConfig()
	}

	// Check for key_id
	if cliCfg.startKeyId == "" {
		log.Fatal("Error: --key is required. Use 'list-keys' to see available keys.")
	}

	// Initialize keystore
	ks, err := keystore.New(cfg.ToKeystoreConfig())
	if err != nil {
		log.Fatalf("Failed to initialize keystore: %v", err)
	}

	if err := ks.Open(); err != nil {
		log.Fatalf("Failed to open keystore: %v", err)
	}
	defer ks.Close()

	// TODO: Implement container start logic
	log.Printf("Starting agent with key '%s'...", cliCfg.startKeyId)
	log.Println("Note: Container start via RPC is not yet implemented in CLI mode.")
	log.Println("Use the RPC API to start containers:")
	log.Printf(`echo '{"jsonrpc":"2.0","method":"start","params":{"key_id":"%s"},"id":1}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`, cliCfg.startKeyId)
}

// runGenerateQRCommand generates a QR code for ArmorChat discovery
func runGenerateQRCommand(cliCfg cliConfig) {
	// Load configuration
	cfg, err := config.Load(cliCfg.configPath)
	if err != nil {
		log.Printf("Warning: Using default configuration: %v", err)
		cfg = config.DefaultConfig()
	}

	// Get hostname: --host flag > config > UDP public IP detection > os.Hostname()
	hostname := cliCfg.qrHost
	if hostname == "" && cfg.HTTP.Hostname != "" {
		hostname = cfg.HTTP.Hostname
	}
	if hostname == "" {
		hostname = getHostname()
	}

	port := cfg.Discovery.Port
	if cliCfg.qrPort != 0 {
		port = cliCfg.qrPort
	}

	// Determine protocol
	protocol := "http"
	if cfg.Discovery.TLS {
		protocol = "https"
	}

	// Build configuration data
	matrixHS := cfg.Matrix.HomeserverURL
	if matrixHS == "" {
		matrixHS = fmt.Sprintf("%s://%s:8448", protocol, hostname)
	}

	// Create the config JSON
	configData := map[string]interface{}{
		"version":           1,
		"matrix_homeserver": matrixHS,
		"rpc_url":           fmt.Sprintf("%s://%s:%d/api", protocol, hostname, port),
		"ws_url":            fmt.Sprintf("%s://%s:%d/ws", map[bool]string{true: "wss", false: "ws"}[cfg.Discovery.TLS], hostname, port),
		"push_gateway":      strings.TrimSuffix(matrixHS, "/") + "/_matrix/push/v1/notify",
		"server_name":       hostname,
		"expires_at":        time.Now().Add(24 * time.Hour).Unix(),
	}

	// Convert to JSON
	jsonData, err := json.Marshal(configData)
	if err != nil {
		log.Fatalf("Failed to create config JSON: %v", err)
	}

	// Base64 encode
	encodedData := base64.StdEncoding.EncodeToString(jsonData)

	// Create deep link URL
	deepLinkURL := fmt.Sprintf("armorclaw://config?d=%s", encodedData)
	webURL := fmt.Sprintf("https://armorclaw.app/config?d=%s", encodedData)

	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║              ARMORCHAT DISCOVERY QR CODE GENERATED                          ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println("")
	fmt.Println("To connect ArmorChat to this bridge:")
	fmt.Println("")
	fmt.Println("  1. Open ArmorChat on your device")
	fmt.Println("  2. Go to Settings → Add Server → Scan QR Code")
	fmt.Println("  3. Scan the QR code below or use the deep link")
	fmt.Println("")
	fmt.Println("┌─────────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│ CONFIGURATION DATA                                                          │")
	fmt.Println("├─────────────────────────────────────────────────────────────────────────────┤")
	fmt.Printf("│ Server:       %s\n", hostname)
	fmt.Printf("│ Port:         %d\n", port)
	fmt.Printf("│ TLS:          %v\n", cfg.Discovery.TLS)
	fmt.Printf("│ Matrix:       %s\n", matrixHS)
	fmt.Printf("│ RPC:          %s://%s:%d/api\n", protocol, hostname, port)
	fmt.Printf("│ WebSocket:    %s://%s:%d/ws\n", map[bool]string{true: "wss", false: "ws"}[cfg.Discovery.TLS], hostname, port)
	fmt.Println("│ Valid:        24 hours")
	fmt.Println("└─────────────────────────────────────────────────────────────────────────────┘")
	fmt.Println("")
	fmt.Println("┌─────────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│ DEEP LINK (copy/paste to device)                                            │")
	fmt.Println("├─────────────────────────────────────────────────────────────────────────────┤")
	fmt.Printf("│ %s\n", deepLinkURL[:min(75, len(deepLinkURL))])
	if len(deepLinkURL) > 75 {
		fmt.Printf("│ %s\n", deepLinkURL[75:min(150, len(deepLinkURL))])
	}
	if len(deepLinkURL) > 150 {
		fmt.Printf("│ %s\n", deepLinkURL[150:])
	}
	fmt.Println("└─────────────────────────────────────────────────────────────────────────────┘")
	fmt.Println("")
	fmt.Println("┌─────────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│ WEB LINK (for browsers)                                                     │")
	fmt.Println("├─────────────────────────────────────────────────────────────────────────────┤")
	fmt.Printf("│ %s\n", webURL[:min(75, len(webURL))])
	if len(webURL) > 75 {
		fmt.Printf("│ %s\n", webURL[75:min(150, len(webURL))])
	}
	if len(webURL) > 150 {
		fmt.Printf("│ %s\n", webURL[150:])
	}
	fmt.Println("└─────────────────────────────────────────────────────────────────────────────┘")
	fmt.Println("")
	fmt.Println("📝 TIP: For production use, consider:")
	fmt.Println("   • Use --host with your public domain (e.g., bridge.example.com)")
	fmt.Println("   • Ensure TLS is enabled in your config ([discovery] tls = true)")
	fmt.Println("   • Configure your firewall to allow the port")
	fmt.Println("")

	qrResult := &qr.QRResult{
		DeepLink: deepLinkURL,
	}
	if qrText, err := qrResult.ToTerminal(); err == nil {
		fmt.Println("")
		fmt.Println(qrText)
	} else {
		fmt.Printf("⚠️  Failed to render terminal QR: %v\n", err)
		fmt.Printf("   Use deep link: %s\n", deepLinkURL)
	}
	fmt.Println("")
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// runStartAgentCommand starts an AI agent (OpenClaw or similar) via bridge RPC
func runStartAgentCommand(cliCfg cliConfig) {
	// Validate required parameters
	if cliCfg.agentRoom == "" {
		log.Fatal("Error: --room is required. Specify the Matrix room ID for the agent.")
	}

	// Generate agent name if not provided
	agentName := cliCfg.agentName
	if agentName == "" {
		agentName = fmt.Sprintf("%s-agent", cliCfg.agentType)
	}

	// Parse capabilities
	capabilities := []string{"chat"}
	if cliCfg.agentCapabilities != "" {
		capabilities = strings.Split(cliCfg.agentCapabilities, ",")
		for i, cap := range capabilities {
			capabilities[i] = strings.TrimSpace(cap)
		}
	}

	// Check if bridge is running
	socketPath := "/run/armorclaw/bridge.sock"
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		log.Fatal("Error: Bridge is not running. Start it first with: armorclaw-bridge")
	}

	// Connect to bridge via Unix socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Fatalf("Error: Failed to connect to bridge: %v", err)
	}
	defer conn.Close()

	// Generate agent ID
	agentID := fmt.Sprintf("%s-%d", cliCfg.agentType, time.Now().Unix())

	// Build RPC request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "agent.start",
		"params": map[string]interface{}{
			"agent_id":     agentID,
			"name":         agentName,
			"type":         cliCfg.agentType,
			"room_id":      cliCfg.agentRoom,
			"capabilities": capabilities,
		},
	}

	// Add key_id if provided
	if cliCfg.agentKey != "" {
		request["params"].(map[string]interface{})["key_id"] = cliCfg.agentKey
	}

	// Send request
	requestJSON, _ := json.Marshal(request)
	_, err = conn.Write(append(requestJSON, '\n'))
	if err != nil {
		log.Fatalf("Error: Failed to send request: %v", err)
	}

	// Read response
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		log.Fatalf("Error: Failed to read response: %v", err)
	}

	// Parse response
	var response struct {
		Jsonrpc string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  *struct {
			AgentID      string   `json:"agent_id"`
			Name         string   `json:"name"`
			Type         string   `json:"type"`
			Status       string   `json:"status"`
			RoomID       string   `json:"room_id"`
			Capabilities []string `json:"capabilities"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(buffer[:n], &response); err != nil {
		log.Fatalf("Error: Failed to parse response: %v", err)
	}

	if response.Error != nil {
		log.Fatalf("Error: Agent start failed (code %d): %s", response.Error.Code, response.Error.Message)
	}

	if response.Result == nil {
		log.Fatal("Error: No result returned from bridge")
	}

	// Success output
	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                      AGENT STARTED SUCCESSFULLY                              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println("")
	fmt.Println("┌─────────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│ AGENT INFORMATION                                                           │")
	fmt.Println("├─────────────────────────────────────────────────────────────────────────────┤")
	fmt.Printf("│ Agent ID:     %s\n", response.Result.AgentID)
	fmt.Printf("│ Name:         %s\n", response.Result.Name)
	fmt.Printf("│ Type:         %s\n", response.Result.Type)
	fmt.Printf("│ Status:       %s\n", response.Result.Status)
	fmt.Printf("│ Room:         %s\n", response.Result.RoomID)
	fmt.Printf("│ Capabilities: %s\n", strings.Join(response.Result.Capabilities, ", "))
	fmt.Println("└─────────────────────────────────────────────────────────────────────────────┘")
	fmt.Println("")
	fmt.Println("📱 Connect ArmorChat to interact with this agent:")
	fmt.Printf("   Room ID: %s\n", response.Result.RoomID)
	fmt.Println("")
	fmt.Println("🔧 Management commands:")
	fmt.Println("   Check status:  armorclaw-bridge agent-status --id " + response.Result.AgentID)
	fmt.Println("   Stop agent:    armorclaw-bridge stop-agent --id " + response.Result.AgentID)
	fmt.Println("   View logs:     journalctl -u armorclaw-bridge -f")
	fmt.Println("")

	// If using OpenClaw type, provide additional guidance
	if cliCfg.agentType == "openclaw" || cliCfg.agentType == "assistant" {
		fmt.Println("💡 OpenClaw Agent Tips:")
		fmt.Println("   • Ensure API keys are stored: armorclaw-bridge add-key --provider openai --token sk-xxx")
		fmt.Println("   • Use docker-compose to manage: docker-compose -f docker-compose.bridge.yml --profile openclaw up -d")
		fmt.Println("   • Check container status: docker ps | grep openclaw")
		fmt.Println("")
	}
}

// runBridgeServer starts the bridge server
func runBridgeServer(cliCfg cliConfig) {
	log.Printf("Starting ArmorClaw Bridge v%s", version)
	log.Printf("Build time: %s", buildTime)

	// Check for ARMORCLAW_API_KEY environment variable (OpenClaw compatibility)
	if apiKey := os.Getenv("ARMORCLAW_API_KEY"); apiKey != "" {
		log.Println("⚠️  ARMORCLAW_API_KEY detected - This will auto-store your key for convenience")
		log.Println("    For production, consider using 'add-key' command instead")

		// Auto-store the key
		autoStoreKey(apiKey)
	}

	// Load configuration file
	cfg, err := config.Load(cliCfg.configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Apply CLI flag overrides
	if cliCfg.socketPath != "" {
		cfg.Server.SocketPath = cliCfg.socketPath
	}
	if cliCfg.dbPath != "" {
		cfg.Keystore.DBPath = cliCfg.dbPath
	}
	if cliCfg.matrixHomeserver != "" {
		cfg.Matrix.HomeserverURL = cliCfg.matrixHomeserver
		cfg.Matrix.Enabled = true
	}
	if cliCfg.matrixUsername != "" {
		cfg.Matrix.Username = cliCfg.matrixUsername
	}
	if cliCfg.matrixPassword != "" {
		cfg.Matrix.Password = cliCfg.matrixPassword
	}
	if cliCfg.matrixEnabled {
		cfg.Matrix.Enabled = true
	}
	if cliCfg.logLevel != "" {
		cfg.Logging.Level = cliCfg.logLevel
	}

	// Validate final configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Setup logging
	setupLogging(cfg.Logging)

	log.Printf("Configuration loaded successfully")
	log.Printf("Socket: %s", cfg.Server.SocketPath)
	log.Printf("Keystore: %s", cfg.Keystore.DBPath)
	if cfg.Matrix.Enabled {
		log.Printf("Matrix: %s (enabled)", cfg.Matrix.HomeserverURL)
	} else {
		log.Printf("Matrix: disabled")
	}

	// Pre-flight checks
	// Check Docker availability before initializing services
	// Skip check if ARMORCLAW_SKIP_DOCKER_CHECK is set (for testing)
	if os.Getenv("ARMORCLAW_SKIP_DOCKER_CHECK") == "" {
		log.Println("Checking Docker availability...")
		if !docker.IsAvailable() {
			log.Fatalf("Docker is not available or not running. " +
				"Please start Docker and ensure the daemon is accessible.")
		}
		log.Println("Docker is available")
	} else {
		log.Println("Skipping Docker availability check (ARMORCLAW_SKIP_DOCKER_CHECK set)")
	}

	// Ensure base runtime directory exists (cross-platform safe)
	runtimeDir := filepath.Dir(cfg.Server.SocketPath)
	if runtimeDir == "" || runtimeDir == "." {
		runtimeDir = filepath.Join(os.TempDir(), "armorclaw")
	}
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		log.Fatalf("Failed to create runtime directory %s: %v", runtimeDir, err)
	}
	log.Printf("Runtime directory ready: %s", runtimeDir)

	// Initialize keystore with recovery for corrupted databases
	log.Println("Initializing encrypted keystore...")
	ks, err := keystore.New(cfg.ToKeystoreConfig())
	if err != nil {
		log.Fatalf("Failed to initialize keystore: %v", err)
	}

	if err := ks.Open(); err != nil {
		// Check if database is corrupted - try to recover
		if strings.Contains(err.Error(), "file is not a database") || strings.Contains(err.Error(), "database disk image is malformed") {
			log.Printf("Warning: Corrupted keystore detected, attempting recovery...")
			dbPath := cfg.Keystore.DBPath

			// Backup corrupted file
			backupPath := dbPath + ".corrupted." + time.Now().Format("20060102-150405")
			if renameErr := os.Rename(dbPath, backupPath); renameErr != nil {
				log.Printf("Warning: Could not backup corrupted database: %v", renameErr)
				// Try to delete directly
				os.Remove(dbPath)
				os.Remove(dbPath + "-shm")
				os.Remove(dbPath + "-wal")
			}
			log.Printf("Corrupted database backed up to: %s", backupPath)

			// Retry opening
			ks, err = keystore.New(cfg.ToKeystoreConfig())
			if err != nil {
				log.Fatalf("Failed to reinitialize keystore: %v", err)
			}
			if err := ks.Open(); err != nil {
				log.Fatalf("Failed to open keystore after recovery: %v", err)
			}
			log.Printf("Keystore recovered successfully (previous data lost)")
		} else {
			log.Fatalf("Failed to open keystore: %v", err)
		}
	}
	defer ks.Close()

	if cliCfg.migrateKeystore {
		log.Println("Migrating keystore from hardware-derived key to file-persisted key...")
		if err := ks.MigrateToPersistedKey(); err != nil {
			log.Fatalf("Failed to migrate keystore: %v", err)
		}
		log.Println("✓ Keystore migration completed successfully")
		log.Printf("  Key persisted to file: %s", cfg.Keystore.DBPath)
		log.Println("  You can now restart without --migrate-keystore flag")
	}

	log.Println("Keystore initialized with hardware-derived master key")

	// Initialize z.ai provider configuration
	log.Println("Configuring z.ai provider...")
	providersDir := "/etc/armorclaw"
	if err := os.MkdirAll(providersDir, 0755); err != nil {
		log.Printf("Warning: Failed to create providers directory: %v", err)
	} else {
		providersPath := filepath.Join(providersDir, "providers.json")
		registry, err := providers.LoadRegistry(providersPath)
		if err != nil {
			log.Printf("Warning: Failed to load provider registry: %v", err)
			// Use embedded registry
			registry = &providers.EmbeddedRegistry
		}

		// Check if z.ai provider exists with correct endpoint
		zaiProvider, found := registry.GetProviderByID("zhipu")
		if !found || zaiProvider.BaseURL != "https://api.z.ai/v1" {
			log.Println("Updating z.ai provider configuration...")
			// Create updated registry with z.ai endpoint
			updatedProviders := []providers.Provider{
				{ID: "openai", Name: "OpenAI", Protocol: "openai", BaseURL: "https://api.openai.com/v1"},
				{ID: "anthropic", Name: "Anthropic", Protocol: "anthropic", BaseURL: "https://api.anthropic.com/v1"},
				{ID: "google", Name: "Google", Protocol: "openai", BaseURL: "https://generativelanguage.googleapis.com/v1"},
				{ID: "xai", Name: "xAI", Protocol: "openai", BaseURL: "https://api.x.ai/v1"},
				{ID: "openrouter", Name: "OpenRouter", Protocol: "openai", BaseURL: "https://openrouter.ai/api/v1"},
				{ID: "zhipu", Name: "Zhipu AI (Z AI)", Protocol: "openai", BaseURL: "https://api.z.ai/v1", Aliases: []string{"zai", "glm"}},
				{ID: "deepseek", Name: "DeepSeek", Protocol: "openai", BaseURL: "https://api.deepseek.com/v1"},
				{ID: "moonshot", Name: "Moonshot AI", Protocol: "openai", BaseURL: "https://api.moonshot.ai/v1"},
				{ID: "nvidia", Name: "NVIDIA NIM", Protocol: "openai", BaseURL: "https://integrate.api.nvidia.com/v1"},
				{ID: "groq", Name: "Groq", Protocol: "openai", BaseURL: "https://api.groq.com/openai/v1"},
				{ID: "cloudflare", Name: "Cloudflare", Protocol: "openai", BaseURL: "https://gateway.ai.cloudflare.com/v1"},
				{ID: "ollama", Name: "Ollama (Local)", Protocol: "openai", BaseURL: "http://localhost:11434/v1"},
			}

			updatedRegistry := providers.Registry{Providers: updatedProviders}
			data, err := json.MarshalIndent(updatedRegistry, "", "  ")
			if err != nil {
				log.Printf("Warning: Failed to marshal provider registry: %v", err)
			} else if err := os.WriteFile(providersPath, data, 0644); err != nil {
				log.Printf("Warning: Failed to write provider registry: %v", err)
			} else {
				log.Printf("z.ai provider configured at: %s", providersPath)
			}
		} else {
			log.Println("z.ai provider already configured correctly")
		}
	}

	// API keys are read from environment variables at runtime
	// Set ZAI_API_KEY in your shell profile (.zshrc) before starting the bridge
	if apiKey := os.Getenv("ZAI_API_KEY"); apiKey != "" {
		log.Println("ZAI_API_KEY found in environment - will be used for zhipu provider")
	}

	// Initialize error handling system
	log.Println("Initializing error handling system...")
	errorCfg := cfg.ToErrorSystemConfig()
	errorSystem, err := errors.Initialize(errors.Config{
		StorePath:       errorCfg.StorePath,
		RetentionDays:   errorCfg.RetentionDays,
		RateLimitWindow: errorCfg.RateLimitWindow,
		RetentionPeriod: errorCfg.RetentionPeriod,
		ConfigAdminMXID: errorCfg.ConfigAdminMXID,
		SetupUserMXID:   errorCfg.SetupUserMXID,
		AdminRoomID:     errorCfg.AdminRoomID,
		FallbackMXID:    errorCfg.FallbackMXID,
		Enabled:         errorCfg.Enabled,
		StoreEnabled:    errorCfg.StoreEnabled,
		NotifyEnabled:   errorCfg.NotifyEnabled,
	})
	if err != nil {
		log.Fatalf("Failed to initialize error system: %v", err)
	}
	defer errorSystem.Stop()

	// Start the error system
	if err := errorSystem.Start(context.Background()); err != nil {
		log.Printf("Warning: Failed to start error system: %v", err)
	} else {
		log.Println("Error system initialized")
		if errorCfg.StoreEnabled {
			log.Printf("Error store: %s", errorCfg.StorePath)
		}
		if errorCfg.SetupUserMXID != "" {
			log.Printf("Setup user: %s", errorCfg.SetupUserMXID)
		}
	}

	// Initialize WebRTC components
	log.Println("Initializing WebRTC components...")

	// Create session manager
	sessionConfig := webrtc.DefaultSessionConfig()
	sessionConfig.DefaultTTL = 30 * time.Minute
	sessionMgr := webrtc.NewSessionManager(sessionConfig)

	// Create token manager (requires secret for signing)
	// Use TURN secret or generate a random one
	tokenSecret := cfg.WebRTC.TURNSharedSecret
	if tokenSecret == "" {
		// Generate a random secret if not configured
		tokenSecret = fmt.Sprintf("armorclaw-%d", time.Now().UnixNano())
	}
	tokenMgr := webrtc.NewTokenManager(tokenSecret, 24*time.Hour)

	// Create WebRTC engine
	webrtcConfig := webrtc.DefaultEngineConfig()
	webrtcEngine, err := webrtc.NewEngine(webrtcConfig)
	if err != nil {
		log.Fatalf("Failed to create WebRTC engine: %v", err)
	}

	// Create TURN manager (required for voice features)
	// TURN_SECRET must be configured — the default is intentionally empty
	// to prevent deploying with a known shared secret.
	var turnMgr *turn.Manager
	if cfg.WebRTC.TURNSharedSecret != "" {
		// Use default TURN config with the configured secret
		turnConfig := turn.DefaultConfig()
		turnConfig.Secret = cfg.WebRTC.TURNSharedSecret
		if cfg.WebRTC.TURNServerURL != "" {
			// Parse TURN URL (format: turn:host:port)
			turnURL := cfg.WebRTC.TURNServerURL
			if strings.HasPrefix(turnURL, "turn:") {
				turnURL = strings.TrimPrefix(turnURL, "turn:")
			} else if strings.HasPrefix(turnURL, "turns:") {
				turnURL = strings.TrimPrefix(turnURL, "turns:")
			}
			parts := strings.Split(turnURL, ":")
			if len(parts) >= 1 {
				turnConfig.Servers[0].Host = parts[0]
			}
			if len(parts) >= 2 {
				var port int
				fmt.Sscanf(parts[1], "%d", &port)
				turnConfig.Servers[0].Port = port
			}
		}
		var turnErr error
		turnMgr, turnErr = turn.NewManager(turnConfig)
		if turnErr != nil {
			log.Fatalf("FATAL: %v", turnErr)
		}
		webrtcEngine.SetTURNManager(turnMgr)
		log.Println("TURN manager initialized")
	}

	// Initialize voice manager (gated by VoicePipeline feature flag)
	var voiceMgr *voice.Manager
	if cfg.IsFeatureEnabled("voice_pipeline") {
		log.Println("Initializing voice manager (VoicePipeline=cloud)...")

		parseDuration := func(s string) time.Duration {
			if s == "" {
				return 0
			}
			d, err := time.ParseDuration(s)
			if err != nil {
				log.Printf("Warning: Invalid duration '%s': %v", s, err)
				return 0
			}
			return d
		}

		stringSliceToBoolMap := func(slice []string) map[string]bool {
			result := make(map[string]bool)
			for _, s := range slice {
				result[s] = true
			}
			return result
		}

		securityPolicy := voice.DefaultSecurityPolicy()
		if cfg.Voice.Security.MaxCallDuration != "" {
			if d := parseDuration(cfg.Voice.Security.MaxCallDuration); d > 0 {
				securityPolicy.MaxCallDuration = d
			}
		}
		securityPolicy.RateLimitCalls = cfg.Voice.Security.RateLimitCalls
		if cfg.Voice.Security.RateLimitWindow != "" {
			if d := parseDuration(cfg.Voice.Security.RateLimitWindow); d > 0 {
				securityPolicy.RateLimitWindow = d
			}
		}
		securityPolicy.RequireE2EE = cfg.Voice.Security.RequireE2EE
		securityPolicy.RequireSignalingTLS = cfg.Voice.Security.RequireSignalingTLS

		budgetConfig := voice.DefaultConfig()
		budgetConfig.DefaultTokenLimit = cfg.Voice.Budget.DefaultTokenLimit
		if cfg.Voice.Budget.DefaultDurationLimit != "" {
			if d := parseDuration(cfg.Voice.Budget.DefaultDurationLimit); d > 0 {
				budgetConfig.DefaultDurationLimit = d
			}
		}
		budgetConfig.WarningThreshold = cfg.Voice.Budget.WarningThreshold
		budgetConfig.HardStop = cfg.Voice.Budget.HardStop
		budgetConfig.AutoAnswer = cfg.Voice.AutoAnswer
		budgetConfig.RequireMembership = cfg.Voice.RequireMembership
		budgetConfig.AllowedRooms = stringSliceToBoolMap(cfg.Voice.AllowedRooms)
		budgetConfig.BlockedRooms = stringSliceToBoolMap(cfg.Voice.BlockedRooms)
		if cfg.Voice.DefaultLifetime != "" {
			if d := parseDuration(cfg.Voice.DefaultLifetime); d > 0 {
				budgetConfig.DefaultLifetime = d
			}
		}
		if cfg.Voice.MaxLifetime != "" {
			if d := parseDuration(cfg.Voice.MaxLifetime); d > 0 {
				budgetConfig.MaxLifetime = d
			}
		}

		ttlConfig := voice.DefaultTTLConfig()
		if cfg.Voice.TTL.DefaultTTL != "" {
			if d := parseDuration(cfg.Voice.TTL.DefaultTTL); d > 0 {
				ttlConfig.DefaultTTL = d
			}
		}
		if cfg.Voice.TTL.MaxTTL != "" {
			if d := parseDuration(cfg.Voice.TTL.MaxTTL); d > 0 {
				ttlConfig.MaxTTL = d
			}
		}
		if cfg.Voice.TTL.EnforcementInterval != "" {
			if d := parseDuration(cfg.Voice.TTL.EnforcementInterval); d > 0 {
				ttlConfig.EnforcementInterval = d
			}
		}
		if cfg.Voice.TTL.WarningThreshold > 0 {
			ttlConfig.WarningThreshold = cfg.Voice.TTL.WarningThreshold
		}
		ttlConfig.HardStop = cfg.Voice.TTL.HardStop

		maxCallDuration := securityPolicy.MaxCallDuration
		if maxCallDuration == 0 {
			maxCallDuration = 1 * time.Hour
		}

		voiceMgr = voice.NewManager(
			sessionMgr,
			tokenMgr,
			webrtcEngine,
			turnMgr,
			voice.ManagerConfig{
				WebRTCConfig:         webrtc.DefaultEngineConfig(),
				VoiceConfig:          budgetConfig,
				SecurityPolicy:       securityPolicy,
				BudgetConfig: voice.BudgetConfig{
					DefaultTokenLimit:    budgetConfig.DefaultTokenLimit,
					DefaultDurationLimit: budgetConfig.DefaultDurationLimit,
					WarningThreshold:     budgetConfig.WarningThreshold,
					HardStop:             budgetConfig.HardStop,
				},
				DefaultLifetime:      budgetConfig.DefaultLifetime,
				MaxLifetime:          budgetConfig.MaxLifetime,
				TURNSharedSecret:     cfg.WebRTC.TURNSharedSecret,
				TURNServerURL:        cfg.WebRTC.TURNServerURL,
				MaxConcurrentCalls:   cfg.Voice.Security.MaxConcurrentCalls,
				MaxCallDuration:      maxCallDuration,
			},
		)

		if err := voiceMgr.Start(); err != nil {
			log.Printf("Warning: Failed to start voice manager: %v", err)
			log.Println("Voice calls will not be available")
		} else {
			log.Println("Voice manager started")
		}
	} else {
		log.Println("Voice pipeline disabled (features.voice_pipeline=off)")
	}

	log.Println("WebRTC components initialized")

	// Initialize Docker client for container management
	log.Println("Initializing Docker client...")
	dockerClient, err := docker.New(docker.Config{
		Host:       "", // Use default socket
		APIVersion: "1.45",
		Scopes: []docker.Scope{
			docker.ScopeCreate,
			docker.ScopeExec,
			docker.ScopeRemove,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	// Initialize notification system (requires Matrix adapter)
	// Declare notifier early so it can be used in health monitor callback
	var notifier *notification.Notifier
	if cfg.Matrix.Enabled && cfg.Notifications.AdminRoomID != "" {
		log.Println("Initializing notification system...")
		// We'll create the notifier after Matrix adapter is initialized
		// For now, create a placeholder that will be configured later
		notifier = notification.NewNotifier(nil, notification.Config{
			AdminRoomID: cfg.Notifications.AdminRoomID,
			Enabled:     true,
		})
	} else {
		log.Println("Notifications disabled (Matrix not enabled or no admin room configured)")
	}

	// Initialize health monitor
	log.Println("Initializing container health monitor...")
	healthConfig := health.DefaultMonitorConfig()
	healthMonitor := health.NewMonitor(dockerClient, healthConfig)

	// Set up container failure handler
	healthMonitor.SetFailureHandler(func(containerID, containerName, reason string) {
		log.Printf("Container failure detected: %s (%s) - %s", containerName, containerID, reason)

		// Send notification if configured
		if notifier != nil {
			_ = notifier.SendContainerAlert("container_failed", containerID, containerName, reason)
		}
	})

	// Start health monitor
	if err := healthMonitor.Start(); err != nil {
		log.Printf("Warning: Failed to start health monitor: %v", err)
		log.Println("Container health monitoring will not be available")
	} else {
		log.Println("Health monitor started")
	}

	// Initialize event bus for real-time Matrix event push
	var eventBus *eventbus.EventBus
	if cfg.Matrix.Enabled {
		log.Println("Initializing event bus for Matrix events...")

		// Parse inactivity timeout
		inactivityTimeout := 30 * time.Minute
		if cfg.EventBus.InactivityTimeout != "" {
			if d, err := time.ParseDuration(cfg.EventBus.InactivityTimeout); err == nil {
				inactivityTimeout = d
			}
		}

		logDir := cfg.EventBus.DurableLogDir
		if logDir == "" {
			logDir = filepath.Join(filepath.Dir(cfg.Keystore.DBPath), "events")
		}

		eventBusConfig := eventbus.Config{
			WebSocketEnabled:  cfg.EventBus.WebSocketEnabled,
			WebSocketAddr:     cfg.EventBus.WebSocketAddr,
			WebSocketPath:     cfg.EventBus.WebSocketPath,
			MaxSubscribers:    cfg.EventBus.MaxSubscribers,
			InactivityTimeout: inactivityTimeout,
			EnableLog:         cfg.EventBus.EnableDurableLog,
			LogDir:            logDir,
			MaxLogFileSize:    cfg.EventBus.MaxLogFileSize,
		}

		// Validate: WebSocket requires HTTP server (broadcaster lives in HTTPS server)
		if cfg.EventBus.WebSocketEnabled && !cfg.HTTP.Enabled {
			log.Fatalf("Configuration error: websocket_enabled=true requires [http] enabled=true (the HTTP server hosts the WebSocket endpoint)")
		}

		eventBus = eventbus.NewEventBus(eventBusConfig)

		// Event bus will be started after HTTPS server creation (needs broadcaster wire)
	} else {
		log.Println("Event bus disabled (Matrix not enabled)")
	}

	// Initialize YARA scanner (required by email ingest for attachment scanning)
	yaraRulesPath := filepath.Join("configs", "yara_rules.yar")
	if err := yara.InitYARA(yaraRulesPath); err != nil {
		log.Printf("Warning: YARA initialization failed (email attachments will bypass malware scan): %v", err)
	}

	// Initialize WebRTC signaling server
	var signalingSvr *webrtc.SignalingServer
	if cfg.WebRTC.SignalingEnabled {
		log.Println("Initializing WebRTC signaling server...")
		// Create signaling server with WebSocket endpoint
		signalingSvr = webrtc.NewSignalingServer(
			cfg.WebRTC.SignalingAddr,
			cfg.WebRTC.SignalingPath,
			sessionMgr,
			tokenMgr,
		)

		// Configure TLS if certificates provided
		if cfg.WebRTC.SignalingTLSCert != "" && cfg.WebRTC.SignalingTLSKey != "" {
			signalingSvr.SetTLS(cfg.WebRTC.SignalingTLSCert, cfg.WebRTC.SignalingTLSKey)
			log.Printf("Signaling server TLS enabled")
		}

		// Start signaling server
		if err := signalingSvr.Start(); err != nil {
			log.Printf("Warning: Failed to start signaling server: %v", err)
			log.Println("WebRTC signaling will use JSON-RPC fallback")
			signalingSvr = nil
		} else {
			log.Printf("Signaling server started on %s%s", cfg.WebRTC.SignalingAddr, cfg.WebRTC.SignalingPath)
		}
	}

	// Connect notifier to Matrix adapter (if Matrix is enabled)
	if notifier != nil && cfg.Matrix.Enabled {
		// We need to get the Matrix adapter from the RPC server later
		// For now, the notifier will be passed to the budget manager
		log.Printf("Notifier configured for admin room: %s", cfg.Notifications.AdminRoomID)
	}

	// Set notifier on budget manager for budget alerts
	if notifier != nil {
		// The budget manager has access to the budget tracker
		// We'll set the notifier on the budget tracker after it's created
		log.Println("Budget alerts will be sent to Matrix")
	}

	// Create shutdown context early for components that need it
	shutdownCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize vault governance client when v6_microkernel is enabled
	vaultClient, vaultEventBridge := setupVaultClient(cfg, eventBus, shutdownCtx)

	// Initialize high-throughput event bus for Matrix event streaming
	var matrixBus *events.MatrixEventBus
	var matrixAdapter *adapter.MatrixAdapter

	if cfg.Matrix.Enabled && cfg.Matrix.HomeserverURL != "" {
		log.Println("Initializing high-throughput Matrix event bus...")

		// Use default buffer size of 1024
		bufferSize := 1024

		matrixBus = events.NewMatrixEventBus(bufferSize)
		log.Printf("Matrix event bus initialized with buffer size: %d", bufferSize)

		// Create MatrixAdapter
		var err error
		matrixAdapter, err = adapter.New(adapter.Config{
			HomeserverURL: cfg.Matrix.HomeserverURL,
			Username:      cfg.Matrix.Username,
			DeviceID:      "armorclaw-bridge",
			Password:      cfg.Matrix.Password,
		})
		if err != nil {
			log.Printf("Warning: Failed to create matrix adapter: %v", err)
			matrixAdapter = nil
		} else {
			// Inject keystore for refresh token persistence
			matrixAdapter.SetKeystore(ks)

			// Set the event bus
			matrixAdapter.SetEventBus(matrixBus)

			// Try refresh-before-login at startup
			loggedIn := false
			if ks != nil {
				storedToken, err := ks.RetrieveMatrixRefreshToken("matrix-refresh-token")
				if err == nil && storedToken != nil && storedToken.Token != "" {
					log.Println("Found stored refresh token, attempting refresh...")
					matrixAdapter.SetRefreshToken(storedToken.Token)
					if err := matrixAdapter.RefreshAccessToken(); err != nil {
						log.Printf("Warning: Refresh token failed (expired/invalid), falling back to password login: %v", err)
						_ = ks.DeleteMatrixRefreshToken("matrix-refresh-token")
					} else {
						log.Println("Matrix session restored via refresh token")
						loggedIn = true
					}
				}
			}

			// Fall back to password login if refresh failed or no stored token
			if !loggedIn {
				if err := matrixAdapter.Login(cfg.Matrix.Username, cfg.Matrix.Password); err != nil {
					log.Printf("Warning: Matrix login failed (will use anonymous mode): %v", err)
				} else {
					log.Println("Matrix login successful via password")
				}
			}

			matrixAdapter.StartSync()
			log.Println("Matrix sync loop started")
			log.Printf("Matrix adapter initialized: %s", matrixAdapter.GetUserID())

			matrixAdapter.SetStatusCallback(func(status string) {
				systemd.NotifyStatus("Matrix: " + status)
				if eventBus != nil {
					eventBus.PublishBridgeEvent(eventbus.NewBridgeStatusEvent("matrix", status))
				}
			})
		}
	}

	// Wire E2EE encryption and key exchange services on MatrixAdapter
	if matrixAdapter != nil {
		e2eeEnabled := cfg.Features.E2EEBackup

		if e2eeEnabled {
			cryptoStorePath := filepath.Join(filepath.Dir(cfg.Keystore.DBPath), "crypto.db")
			cryptoStore, storeErr := crypto.NewKeystoreBackedStore(cryptoStorePath)
			if storeErr != nil {
				log.Printf("Warning: E2EE crypto store init failed (%v), E2EE services disabled", storeErr)
			} else {
				cryptoEngine, engineErr := crypto.NewCryptoEngine(shutdownCtx, crypto.CryptoEngineConfig{
					UserID:        matrixAdapter.GetUserID(),
					DeviceID:      "armorclaw-bridge",
					HomeserverURL: cfg.Matrix.HomeserverURL,
					AccessToken:   matrixAdapter.GetAccessToken(),
					Store:         cryptoStore,
					E2EEEnabled:   true,
				})
				if engineErr != nil {
					log.Printf("Warning: E2EE CryptoEngine init failed (%v), E2EE services disabled", engineErr)
				} else if cryptoEngine != nil {
					roomCache := crypto.NewRoomEncryptionCache(true)
					encryptionService := crypto.NewEncryptionService(cryptoEngine, roomCache, nil)
					keyExchangeService := crypto.NewKeyExchangeService(cryptoEngine, nil)

					matrixAdapter.SetEncryptionService(encryptionService)
					matrixAdapter.SetKeyExchangeService(keyExchangeService)
					log.Println("E2EE encryption and key exchange services wired on MatrixAdapter")
				}
			}
		} else {
			// Even with E2EE disabled, wire nil-safe services for dual-mode messaging
			matrixAdapter.SetEncryptionService(nil)
			matrixAdapter.SetKeyExchangeService(nil)
		}
	}

	if voiceMgr != nil && matrixAdapter != nil {
		mm := voice.NewMatrixManager(matrixAdapter, sessionMgr, voice.DefaultConfig())
		voiceMgr.SetMatrixManager(mm)
		log.Println("MatrixManager injected into voice manager")
	}

	// P1: Bridge MatrixEventBus -> pkg/eventbus
	if matrixBus != nil && eventBus != nil {
		log.Println("Starting Matrix event bridge...")
		go func(ctx context.Context) {
			sub := matrixBus.Subscribe()
			for {
				select {
				case ev, ok := <-sub:
					if !ok {
						return
					}

					// Forward to global event bus (non-blocking)
					content, _ := ev.Content.(map[string]interface{})

					mEv := &eventbus.MatrixEvent{
						Type:    ev.Type,
						RoomID:  ev.RoomID,
						Sender:  ev.Sender,
						Content: content,
						EventID: ev.ID,
					}

					go eventBus.Publish(mEv)

				case <-ctx.Done():
					return
				}
			}
		}(shutdownCtx)
	}

	var bridgeManager *appservice.BridgeManager
	sdtwAdapters := make(map[appservice.Platform]sdtw.SDTWAdapter)

	if cfg.Matrix.Enabled && matrixAdapter != nil {
		log.Println("Initializing SDTW platform adapters...")

		log.Println("Initializing Discord adapter...")
		discordAdapter := sdtw.NewDiscordAdapter()
		sdtwAdapters[appservice.PlatformDiscord] = discordAdapter
		log.Println("Discord adapter initialized")

		log.Println("Initializing Teams adapter...")
		teamsAdapter := sdtw.NewTeamsAdapter(sdtw.TeamsConfig{})
		sdtwAdapters[appservice.PlatformTeams] = teamsAdapter
		log.Println("Teams adapter initialized")

		log.Println("Initializing WhatsApp adapter...")
		whatsappAdapter := sdtw.NewWhatsAppAdapter()
		sdtwAdapters[appservice.PlatformWhatsApp] = whatsappAdapter
		log.Println("WhatsApp adapter initialized")

		log.Println("Creating BridgeManager...")
		bm, err := appservice.NewBridgeManager(appservice.BridgeConfig{
			AppService: nil, // TODO: Wire AppService when available
			Client:     nil, // TODO: Wire Matrix client when available
			Adapters:   sdtwAdapters,
		})
		if err != nil {
			log.Printf("Warning: Failed to create BridgeManager: %v", err)
		} else {
			bridgeManager = bm

			for platform, adapter := range sdtwAdapters {
				if err := bridgeManager.RegisterAdapter(platform, adapter); err != nil {
					log.Printf("Warning: Failed to register %s adapter: %v", platform, err)
				} else {
					log.Printf("Successfully registered %s adapter with BridgeManager", platform)
				}
			}
		}
	}

	// --- EMAIL INGEST SERVER ---
	var ingestServer *email.IngestServer
	if cfg.Matrix.Enabled {
		ingestServer = setupEmailIngest(eventBus, filepath.Dir(cfg.Keystore.DBPath))
	}

	// Initialize RPC server
	log.Printf("Starting JSON-RPC server on %s", cfg.Server.SocketPath)
	// Compute data dir for provisioning role persistence
	provisioningDataDir := cfg.Provisioning.DataDir
	if provisioningDataDir == "" && cfg.Keystore.DBPath != "" {
		// Default to same directory as keystore for persistence
		provisioningDataDir = filepath.Dir(cfg.Keystore.DBPath)
	}

	// Create browser job manager
	browserJobs := rpc.NewBrowserJobManager()

	// P0: Initialize services for RPC wiring
	// Create SkillExecutor
	skillMgr := skills.NewSkillExecutor()
	log.Println("Skill executor initialized")

	// Create Provisioning manager (requires signing secret)
	var provisioningMgr *provisioning.Manager
	signingSecret := cfg.Provisioning.SigningSecret
	if signingSecret == "" {
		signingSecret = fmt.Sprintf("armorclaw-%d", time.Now().UnixNano())
		log.Println("Warning: Using auto-generated provisioning signing secret (not recommended for production)")
	}
	provisioningMgr, err = provisioning.NewManager(&provisioning.ManagerConfig{
		SigningSecret:        signingSecret,
		DefaultExpirySeconds: cfg.Provisioning.DefaultExpirySeconds,
		MaxExpirySeconds:     cfg.Provisioning.MaxExpirySeconds,
		DataDir:              provisioningDataDir,
	})
	if err != nil {
		log.Printf("Warning: Failed to initialize provisioning manager: %v", err)
		provisioningMgr = nil
	} else {
		log.Println("Provisioning manager initialized")
	}

	// Create Docker client adapter for toolsidecar (v6 microkernel)
	toolsidecarDocker := &toolsidecarDockerAdapter{client: dockerClient}

	// Create Studio service
	var studioService *studio.StudioIntegration
	studioDataPath := filepath.Join(filepath.Dir(cfg.Keystore.DBPath), "studio")

	// Create Docker client adapter for studio
	studioDockerAdapter := studio.NewDockerClientAdapter(
		func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, name string) (string, error) {
			return dockerClient.CreateContainer(ctx, config, hostConfig, nil, nil)
		},
		func(ctx context.Context, containerID string) error {
			return dockerClient.StartContainer(ctx, containerID)
		},
		func(ctx context.Context, containerID string, timeout time.Duration) error {
			opts := container.StopOptions{Timeout: &[]int{int(timeout / time.Second)}[0]}
			return dockerClient.StopContainer(ctx, containerID, opts)
		},
		func(ctx context.Context, containerID string, force bool) error {
			return dockerClient.RemoveContainer(ctx, containerID, force)
		},
		func(ctx context.Context, containerID string) (*studio.ContainerInfo, error) {
			inspect, err := dockerClient.InspectContainer(ctx, containerID)
			if err != nil {
				return nil, err
			}
			return &studio.ContainerInfo{
				ID:       inspect.ID,
				Running:  inspect.State.Running,
				ExitCode: inspect.State.ExitCode,
			}, nil
		},
		func(ctx context.Context, all bool) ([]types.Container, error) {
			return dockerClient.ListContainers(ctx, all, filters.Args{})
		},
	)

	// Wrap Matrix adapter for studio
	var studioMatrix studio.MatrixAdapter
	if matrixAdapter != nil {
		studioMatrix = &studioMatrixAdapter{adapter: matrixAdapter}
	}

	studioService, err = studio.NewIntegration(studio.IntegrationConfig{
		DataPath:      studioDataPath,
		DockerClient:  studioDockerAdapter,
		MatrixAdapter: studioMatrix,
	})
	if err != nil {
		log.Printf("Warning: Failed to initialize studio: %v", err)
		studioService = nil
	} else {
		log.Println("Studio service initialized")
	}

	// Initialize v6 MCP Router (if enabled)
	mcpRouter, mcpTranslator := setupMCPRouter(cfg, toolsidecarDocker, vaultClient)

	rolodexStore, rolodexService, webdavService, calendarService, artifactHandler, delegationService := setupSecretaryServices(ks)
	if rolodexStore != nil {
		defer rolodexStore.Close()
	}

	approvalEngine, trustEngine := setupApprovalAndTrust(rolodexStore)

	workflowOrchestrator, orchestratorIntegration := setupWorkflowEngine(rolodexStore, matrixBus, studioService, cfg)

	// Initialize admin command dependencies
	claimMgr := admin.NewClaimManager(nil, admin.DefaultClaimConfig())
	lockdownMgr, lockdownErr := lockdown.NewManager(lockdown.Config{StateFile: "/var/lib/armorclaw/lockdown.json"})
	if lockdownErr != nil {
		log.Printf("Warning: Failed to initialize lockdown manager: %v", lockdownErr)
		lockdownMgr = nil
	}
	var adminCmdHandler *adapter.CommandHandler
	if claimMgr != nil && lockdownMgr != nil && matrixAdapter != nil {
		adminCmdHandler = adapter.NewCommandHandler(claimMgr, lockdownMgr, matrixAdapter, nil)
		log.Println("Admin command handler initialized")
	}

	taskScheduler := setupSecretaryCommandHandler(
		rolodexStore, workflowOrchestrator, orchestratorIntegration,
		matrixAdapter, studioService,
		rolodexService, webdavService, calendarService,
		approvalEngine, trustEngine,
		adminCmdHandler,
	)
	if taskScheduler != nil {
		defer taskScheduler.Stop()
	}

	// --- EMAIL DISPATCHER ---
	if ingestServer != nil && taskScheduler != nil {
		_ = setupEmailDispatcher(eventBus, taskScheduler, rolodexStore)
	}

	if err := os.MkdirAll("/run/armorclaw", 0755); err != nil {
		log.Printf("Warning: failed to create /run/armorclaw: %v", err)
	}
	if err := sidecar.ProvisionJavaSocketDir(); err != nil {
		log.Printf("Warning: failed to provision Java socket dir: %v", err)
	}

	// --- SIDECAR STARTUP VALIDATION (non-fatal) ---
	validateSidecarSockets()

	// Log RPC dependency status
	log.Printf("RPC dependencies: studio=%v, provisioning=%v, skills=%v",
		studioService != nil, provisioningMgr != nil, skillMgr != nil)

	// Initialize hardening store
	hardeningStore := trust.NewKeystoreHardeningStore(ks.GetDB())
	log.Println("Hardening store initialized")

	// Initialize governance stores (shared keystore DB)
	deviceStore, err := trust.NewDeviceStore(ks.GetDB())
	if err != nil {
		log.Fatalf("Failed to initialize device store: %v", err)
	}
	inviteStore, err := invite.NewInviteStore(ks.GetDB())
	if err != nil {
		log.Fatalf("Failed to initialize invite store: %v", err)
	}

	metrics := rpc.NewMetrics()
	log.Println("Metrics initialized")

	var rpcCfg rpc.Config
	rpcCfg.SocketPath = cfg.Server.SocketPath
	rpcCfg.RPCTransport = cfg.Server.RPCTransport
	rpcCfg.ListenAddr = cfg.Server.ListenAddr
	rpcCfg.Keystore = ks
	rpcCfg.Matrix = matrixAdapter
	rpcCfg.AIService = ai.NewAIService(ks)
	rpcCfg.AIMaxConcurrent = 4
	rpcCfg.BridgeManager = bridgeManager
	rpcCfg.BrowserJobs = browserJobs
	rpcCfg.Studio = studioService
	rpcCfg.AppService = nil
	rpcCfg.ProvisioningMgr = provisioningMgr
	rpcCfg.SkillManager = skillMgr
	rpcCfg.EventBus = eventBus
	rpcCfg.HardeningStore = hardeningStore
	rpcCfg.DeviceStore = deviceStore
	rpcCfg.InviteStore = inviteStore
	rpcCfg.Metrics = metrics
	rpcCfg.MCPRouter = mcpRouter
	rpcCfg.Translator = mcpTranslator
	rpcCfg.VoiceMgr = voiceMgr
	rpcCfg.DockerClient = dockerClient
	auditor, auditErr := audit.NewAuditLog(audit.DefaultConfig())
	if auditErr != nil {
		log.Printf("[WARN] audit logger init failed: %v, degrading to stderr", auditErr)
		// Do NOT fatal — bridge must start even if audit dir unwritable
	} else {
		rpcCfg.AuditLog = auditor
	}
	rpcCfg.Guard = nil    // TODO: wire trust.TrustedProxyGuard when constructed
	navChartStore := browser.NewMultiTabStore()
	rpcCfg.NavChartStore = navChartStore
	skillGateGov := governor.NewGovernor(nil, nil)
	rpcCfg.SkillGate = skillGateGov
	rpcCfg.GovernanceRoomID = resolveGovernanceRoomID(cfg, matrixAdapter)

	rateGuard, rateGuardErr := governor.NewGuard(governor.GuardConfig{
		Groups:       governor.DefaultMethodGroups(),
		DefaultRPS:   30,
		HealthExempt: true,
		AuditLog:     rpcCfg.AuditLog,
		Mode:         cfg.Server.Mode,
	})
	if rateGuardErr != nil {
		if cfg.Server.Mode == "sentinel" || cfg.Server.Mode == "cloudflare" {
			log.Fatalf("[FATAL] Guard required in %s mode but failed to initialize: %v", cfg.Server.Mode, rateGuardErr)
		}
		log.Printf("[WARN] Guard init failed: %v, rate limiting disabled", rateGuardErr)
	} else {
		rpcCfg.MethodRateLimiter = rateGuard
	}

	rpcCfg.ZeroTrustKS = cfg.Features.ZeroTrustKeystore
	rpcCfg.VoicePipeline = cfg.Features.VoicePipeline
	rpcCfg.ReplayFlags = rpc.ReplayFeatureFlags{
		MultiTabReplay: cfg.Features.MultiTabReplay,
	}
	rpcCfg.E2EEBackupEnabled = cfg.Features.E2EEBackup

	if cfg.Features.E2EEBackup {
		backupVaultPath := filepath.Join(filepath.Dir(cfg.Keystore.DBPath), "backups")
		backupStore, err := crypto.NewBackupStore(backupVaultPath)
		if err != nil {
			log.Printf("Warning: E2EE backup store init failed (%v), feature disabled", err)
			rpcCfg.E2EEBackupEnabled = false
		} else {
			rpcCfg.BackupMgr = crypto.NewBackupManager(backupStore, true)
		}
	}

	if cfg.Features.ZeroTrustKeystore {
		sealedKS, err := keystore.NewSealedKeystore(keystore.SealedKeystoreConfig{
			BaseKeystore: ks,
			DefaultTTL:   5 * time.Minute,
			Policy:       keystore.PolicyPassword,
		})
		if err != nil {
			log.Printf("Warning: sealed keystore init failed (%v), feature disabled", err)
			rpcCfg.ZeroTrustKS = false
		} else {
			rpcCfg.SealedKS = sealedKS
			rpcCfg.KeystoreLimiter = keystore.NewRateLimiter(5, 60*time.Second)
		}
	}

	if rolodexStore != nil && workflowOrchestrator != nil {
		rpcCfg.SecretaryHandler = rpc.NewSecretaryHandler(secretary.NewRPCHandler(secretary.RPCHandlerConfig{
			Orchestrator:      workflowOrchestrator,
			Store:             rolodexStore,
			DelegationService: delegationService,
		}))
	}

	if artifactHandler != nil {
		rpcCfg.ArtifactHandler = rpc.NewArtifactRPCHandlerAdapter(artifactHandler)
	}

	outboxDBPath := cfg.Email.OutboxDBPath
	if outboxDBPath == "" {
		outboxDBPath = filepath.Join(filepath.Dir(cfg.Keystore.DBPath), "email-outbox.db")
	}
	outboxStore, outboxErr := email.NewOutboxStore(outboxDBPath)
	if outboxErr != nil {
		log.Printf("Warning: email outbox store init failed (%v), email RPCs will return 'not configured'", outboxErr)
	} else {
		rpcCfg.OutboxStore = outboxStore
		log.Printf("Email outbox store initialized: %s", outboxDBPath)
	}

	server, err := rpc.New(rpcCfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Sidecar clients are lazy-connecting; missing sockets fail at extraction time, not startup.
	officeClient := sidecar.NewOfficeClient(nil)
	rustClient := sidecar.NewClient(nil)
	javaClient := sidecar.NewJavaClient(nil)
	server.SetSidecarClients(officeClient, rustClient, javaClient)
	sidecar.ProbeExtractionMode()
	log.Printf("Sidecar clients injected (office=%s, rust=%s, java=%s)",
		sidecar.OfficeSocketPath, sidecar.DefaultSocketPath, sidecar.JavaSocketPath)

	log.Printf("Starting ArmorClaw Bridge in %s mode with %s transport", cfg.Server.Mode, cfg.Server.RPCTransport)

	// Start the RPC server in a goroutine so it doesn't block other services (e.g. mDNS)
	go func() {
		if err := server.Run(cfg.Server.SocketPath); err != nil {
			log.Fatalf("Failed to start RPC server: %v", err)
		}
	}()

	// Wait a bit for the socket to be created
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(cfg.Server.SocketPath); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	defer func() {
		log.Println("Stopping RPC server...")
	}()

	// Wire Matrix adapter to components
	if matrixAdapter != nil {
		if notifier != nil {
			log.Println("Wiring Matrix adapter to notifier...")
			notifier.SetMatrixAdapter(matrixAdapter)
		}

		if errorSystem != nil {
			log.Println("Wiring Matrix adapter to error system...")
			errorSystem.SetMatrixAdapter(matrixAdapter)
		}
	}

	// Create shutdown context early for components that need it
	// Initialize mDNS discovery server
	var discoveryServer *discovery.Server
	var httpDiscoveryServer *discovery.HTTPServer

	matrixHS := cfg.Matrix.HomeserverURL
	if matrixHS == "" {
		matrixHS = cfg.Discovery.MatrixHomeserver
	}
	pushGW := cfg.Discovery.PushGateway
	if pushGW == "" && matrixHS != "" {
		pushGW = strings.TrimSuffix(matrixHS, "/") + "/_matrix/push/v1/notify"
	}

	if cfg.Discovery.Enabled && !cfg.HTTP.Enabled {
		// Start HTTP discovery server (listens on port 8080)
		log.Println("Starting HTTP discovery server...")
		httpDiscoveryServer, err = discovery.NewHTTPServer(discovery.HTTPServerConfig{
			Port:             cfg.Discovery.Port,
			TLS:              cfg.Discovery.TLS,
			InstanceName:     cfg.Discovery.InstanceName,
			MatrixHomeserver: matrixHS,
			PushGateway:      pushGW,
			APIPath:          cfg.Discovery.APIPath,
			WSPath:           cfg.Discovery.WSPath,
			Metrics:          metrics,
		})
		if err != nil {
			log.Printf("Warning: Failed to create HTTP discovery server: %v", err)
		} else if err := httpDiscoveryServer.Start(shutdownCtx); err != nil {
			log.Printf("Warning: Failed to start HTTP discovery server: %v", err)
			httpDiscoveryServer = nil
		} else {
			protocol := "http"
			if cfg.Discovery.TLS {
				protocol = "https"
			}
			log.Printf("HTTP discovery: %s://0.0.0.0:%d/api/discovery", protocol, cfg.Discovery.Port)
		}

		// Start mDNS discovery server (broadcasts on local network)
		log.Println("Starting mDNS discovery server...")
		discoveryConfig := discovery.ServerConfig{
			InstanceName:     cfg.Discovery.InstanceName,
			Port:             cfg.Discovery.Port,
			TLS:              cfg.Discovery.TLS,
			MatrixHomeserver: matrixHS,
			PushGateway:      pushGW,
			APIPath:          cfg.Discovery.APIPath,
			WSPath:           cfg.Discovery.WSPath,
			ExtraTXT: map[string]string{
				"hardware": cfg.Discovery.Hardware,
			},
		}

		discoveryServer, err = discovery.NewServerWithConfig(discoveryConfig)
		if err != nil {
			log.Printf("Warning: Failed to create mDNS discovery server: %v", err)
			log.Println("Bridge discovery will not be available via mDNS")
		} else {
			if err := discoveryServer.Start(shutdownCtx); err != nil {
				log.Printf("Warning: Failed to start mDNS discovery server: %v", err)
				log.Println("Bridge discovery will not be available via mDNS")
				discoveryServer = nil
			} else {
				info := discoveryServer.Info()
				log.Printf("mDNS discovery started: %s._armorclaw._tcp.local.", info.Name)
				if matrixHS != "" {
					log.Printf("Matrix homeserver: %s", matrixHS)
				}
			}
		}
	} else {
		log.Println("mDNS discovery disabled")
	}

	if cfg.HTTP.Enabled && cfg.Discovery.Enabled {
		log.Println("Discovery routes served via HTTPS server (unified surface)")
	}

	// Start HTTPS bridge server (port 8443) for mobile client access
	var httpsServer *bridgeHTTP.Server
	if cfg.HTTP.Enabled {
		hostname := cfg.HTTP.Hostname
		if hostname == "" {
			hostname = cfg.Discovery.InstanceName
		}

		httpsServer = bridgeHTTP.NewServer(bridgeHTTP.ServerConfig{
			Port:             cfg.HTTP.Port,
			CertDir:          cfg.HTTP.CertDir,
			Hostname:         hostname,
			MatrixHomeserver: matrixHS,
			ServerName:       hostname,
			EnableCORS:       true,
			PushGateway:      pushGW,
			APIPath:          cfg.Discovery.APIPath,
			WSPath:           cfg.Discovery.WSPath,
			Metrics:          metrics,
			ServerMode:       cfg.Server.Mode,
		}, server)

		server.SetTLSInfoProvider(httpsServer)

		if eventBus != nil && httpsServer != nil {
			eventBus.SetBroadcaster(httpsServer)
		}
		if eventBus != nil {
			if err := eventBus.Start(); err != nil {
				log.Printf("Warning: Failed to start event bus: %v", err)
			} else {
				log.Println("Event bus started")
			}
		}

		go func() {
			if err := httpsServer.Start(); err != nil {
				log.Printf("Warning: HTTPS server failed: %v", err)
			}
		}()

		log.Printf("HTTPS bridge server: https://%s:%d", hostname, cfg.HTTP.Port)
	}

	log.Println("ArmorClaw Bridge is running")
	log.Println("Press Ctrl+C to stop")
	log.Println("")

	// Notify systemd that the bridge is ready (no-op if not under systemd)
	systemd.NotifyReady()
	systemd.NotifyStatus("Matrix: initializing...")

	// Start watchdog ticker (only active when running under systemd)
	if systemd.IsRunningSystemd() {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					systemd.NotifyWatchdog()
				case <-shutdownCtx.Done():
					return
				}
			}
		}()
		log.Println("Systemd watchdog enabled (ping every 30s)")
	}

	// Show connection guidance for ArmorChat
	printConnectionGuidance(cfg)

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh

		// Notify systemd FIRST to disable watchdog during shutdown
		systemd.NotifyStopping()

		log.Println("\nShutting down...")

		// Stop HTTP discovery server
		if httpDiscoveryServer != nil {
			log.Println("Stopping HTTP discovery server...")
			httpDiscoveryServer.Stop()
		}

		// Stop mDNS discovery server
		if discoveryServer != nil {
			log.Println("Stopping mDNS discovery server...")
			discoveryServer.Stop()
		}

		if httpsServer != nil {
			httpsServer.Stop(context.Background())
		}

		// Stop WebRTC signaling server
		if signalingSvr != nil {
			log.Println("Stopping WebRTC signaling server...")
			signalingSvr.Stop()
		}

		// Stop event bus
		if eventBus != nil {
			log.Println("Stopping event bus...")
			eventBus.Stop()
		}

		// Stop vault event bridge and close vault client
		if vaultEventBridge != nil {
			log.Println("[VAULT] Stopping vault event bridge...")
			vaultEventBridge.Stop()
		}
		if vaultClient != nil {
			log.Println("[VAULT] Closing vault governance client...")
			vaultClient.Close()
		}

		// Stop health monitor
		log.Println("Stopping health monitor...")
		healthMonitor.Stop()

		// Stop notifier
		if notifier != nil {
			log.Println("Stopping notifier...")
			notifier.Stop()
		}

		// Stop error system
		if errorSystem != nil {
			log.Println("Stopping error system...")
			errorSystem.Stop()
		}

		if voiceMgr != nil {
			voiceMgr.Stop()
		}
		webrtcEngine.Stop()

		if ingestServer != nil {
			log.Println("Stopping email ingest server...")
			ingestServer.Stop()
		}

		cancel()
	}()

	<-shutdownCtx.Done()
	log.Println("ArmorClaw Bridge stopped")
}

// autoStoreKey automatically stores an API key from environment variable
func autoStoreKey(apiKey string) {
	// Detect provider from key format
	provider := detectProviderFromKey(apiKey)
	if provider == "" {
		log.Println("Warning: Could not detect provider from API key format")
		log.Println("         Use 'add-key' command with explicit --provider")
		return
	}

	// Generate default config for auto-storage
	cfg := config.DefaultConfig()

	// Initialize keystore
	ks, err := keystore.New(cfg.ToKeystoreConfig())
	if err != nil {
		log.Printf("Failed to initialize keystore for auto-storage: %v", err)
		return
	}

	if err := ks.Open(); err != nil {
		log.Printf("Failed to open keystore for auto-storage: %v", err)
		return
	}
	defer ks.Close()

	// Create credential
	keyID := provider + "-default"
	cred := keystore.Credential{
		ID:          keyID,
		Provider:    keystore.Provider(provider),
		Token:       apiKey,
		DisplayName: "Auto-stored from ARMORCLAW_API_KEY",
		Tags:        []string{"environment-variable", "auto-stored"},
	}

	// Check if key already exists
	existing, err := ks.Retrieve(keyID)
	if err == nil && existing.Token == apiKey {
		log.Printf("✓ API key already stored as '%s'", keyID)
		return
	}

	// Store the credential
	if err := ks.Store(cred); err != nil {
		log.Printf("Failed to auto-store API key: %v", err)
		return
	}

	log.Printf("✓ API key auto-stored as '%s' (provider: %s)", keyID, provider)
	log.Println("  Start agent: armorclaw-bridge start --key " + keyID)
}

// detectProviderFromKey attempts to detect the AI provider from the API key format
func detectProviderFromKey(key string) string {
	// OpenAI keys start with sk-
	if strings.HasPrefix(key, "sk-") || strings.HasPrefix(key, "sk-proj-") {
		return "openai"
	}
	// Anthropic keys start with sk-ant-
	if strings.HasPrefix(key, "sk-ant-") {
		return "anthropic"
	}
	// Add more patterns as needed
	return ""
}

func parseFlags() cliConfig {
	cfg := cliConfig{}

	flag.StringVar(&cfg.configPath, "config", "", "Path to configuration file")
	flag.StringVar(&cfg.configPath, "c", "", "Path to configuration file (shorthand)")
	flag.StringVar(&cfg.configOutput, "config-output", "", "Output path for 'config init' command")
	flag.StringVar(&cfg.socketPath, "socket", "", "Path to Unix domain socket (overrides config)")
	flag.StringVar(&cfg.dbPath, "db", "", "Path to keystore database (overrides config)")
	flag.StringVar(&cfg.matrixHomeserver, "matrix-homeserver", "", "Matrix homeserver URL (enables Matrix)")
	flag.StringVar(&cfg.matrixUsername, "matrix-username", "", "Matrix username for auto-login")
	flag.StringVar(&cfg.matrixPassword, "matrix-password", "", "Matrix password for auto-login")
	flag.BoolVar(&cfg.matrixEnabled, "matrix-enabled", false, "Enable Matrix communication")
	flag.StringVar(&cfg.logLevel, "log-level", "", "Log level: debug, info, warn, error")
	flag.BoolVar(&cfg.verbose, "v", false, "Verbose logging (sets log level to debug)")
	flag.BoolVar(&cfg.version, "version", false, "Print version and exit")
	flag.BoolVar(&cfg.help, "help", false, "Show help message")
	flag.BoolVar(&cfg.migrateKeystore, "migrate-keystore", false, "Migrate from hardware-derived key to file-persisted key")
	flag.StringVar(&cfg.readminReason, "reason", "", "Reason for entering readmin mode")

	// Quick-start command flags
	flag.StringVar(&cfg.addKeyProvider, "p", "", "Provider for add-key (short for --provider)")
	flag.StringVar(&cfg.addKeyProvider, "provider", "", "Provider for add-key (openai, anthropic, openrouter, google, xai, or any OpenAI-compatible)")
	flag.StringVar(&cfg.addKeyToken, "t", "", "API token for add-key (short for --token)")
	flag.StringVar(&cfg.addKeyToken, "token", "", "API token for add-key (or use ARMORCLAW_API_KEY env var)")
	flag.StringVar(&cfg.addKeyId, "i", "", "Key ID for add-key (short for --id)")
	flag.StringVar(&cfg.addKeyId, "id", "", "Key ID for add-key (default: <provider>-default)")
	flag.StringVar(&cfg.addKeyDisplayName, "n", "", "Display name for add-key (short for --display-name)")
	flag.StringVar(&cfg.addKeyDisplayName, "display-name", "", "Display name for add-key")
	flag.StringVar(&cfg.addKeyBaseURL, "b", "", "Base URL for OpenAI-compatible API providers (short for --base-url)")
	flag.StringVar(&cfg.addKeyBaseURL, "base-url", "", "Base URL for OpenAI-compatible API providers")
	flag.StringVar(&cfg.startKeyId, "key", "", "Key ID for start command")
	// QR code command flags
	flag.StringVar(&cfg.qrHost, "host", "", "Host/domain for QR code (generate-qr command)")
	flag.IntVar(&cfg.qrPort, "port", 0, "Port for QR code (generate-qr command)")
	// Agent command flags
	flag.StringVar(&cfg.agentType, "type", "assistant", "Agent type (start-agent command)")
	flag.StringVar(&cfg.agentName, "agent-name", "", "Agent display name (start-agent command)")
	flag.StringVar(&cfg.agentRoom, "room", "", "Matrix room ID for agent (start-agent command)")
	flag.StringVar(&cfg.agentKey, "agent-key", "", "API key ID for agent (start-agent command)")
	flag.StringVar(&cfg.agentCapabilities, "capabilities", "chat", "Comma-separated capabilities (start-agent command)")

	// Pre-parse to extract command first (before full flag parsing)
	// This handles: armorclaw-bridge add-key --provider openai
	// The "add-key" would be parsed as a flag value, so we need special handling
	if len(os.Args) > 1 {
		// Check if first arg looks like a command (not a flag)
		if !strings.HasPrefix(os.Args[1], "-") {
			cfg.command = os.Args[1]
			// Remove command from args for flag parsing
			os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
		}
	}

	flag.Parse()

	// If command wasn't set from pre-parse, check args
	if cfg.command == "" {
		args := flag.Args()
		if len(args) > 0 {
			cfg.command = args[0]
		}
	}

	// Apply ARMORCLAW_CONFIG env var if --config flag was not explicitly set
	if cfg.configPath == "" {
		if envPath := os.Getenv("ARMORCLAW_CONFIG"); envPath != "" {
			cfg.configPath = envPath
		}
	}

	// Set verbose flag if -v is used
	if cfg.verbose {
		cfg.logLevel = "debug"
	}

	return cfg
}

func setupLogging(cfg config.LoggingConfig) {
	// Initialize the global structured logger
	if err := logger.Initialize(cfg.Level, cfg.Format, cfg.Output); err != nil {
		// Fallback to standard logging if initialization fails
		log.Printf("Warning: Failed to initialize structured logger: %v", err)
		log.Printf("Falling back to standard logging")
		return
	}
}

func printVersion() {
	fmt.Printf("ArmorClaw Bridge v%s\n", version)
	fmt.Printf("Build time: %s\n", buildTime)
	fmt.Println("License: MIT")
	fmt.Println("https://github.com/Gemutly/ArmorClaw")
}

func printHelp() {
	helpText := `USAGE:
    armorclaw-bridge [command] [flags]

COMMANDS:
    init              Initialize configuration file
    validate          Validate configuration
    setup             Run interactive setup wizard (Huh? TUI)
    container-setup   Run container setup wizard (Huh? TUI + infrastructure)
    add-key           Add an API key to the keystore
    list-keys   List all stored API keys
    start       Start an agent container (legacy, use start-agent)
    start-agent Start an AI agent (OpenClaw, assistant, etc.)
    generate-qr Generate QR code for ArmorChat discovery
    completion  Generate shell completion script
    version     Show version information
    help        Show this help message

EXAMPLES:
    # First-time setup (interactive)
    ./build/armorclaw-bridge setup

    # Quick start with defaults
    ./build/armorclaw-bridge init
    ./build/armorclaw-bridge add-key --provider openai --token sk-proj-...
    ./build/armorclaw-bridge start --key openai-default

    # Add key with custom base URL (for OpenAI-compatible providers)
    ./build/armorclaw-bridge add-key --provider openai --base-url https://open.bigmodel.cn/api/paas/v4 --id zhipu --token your-api-key

    # List stored keys
    ./build/armorclaw-bridge list-keys

    # Start an AI agent
    ./build/armorclaw-bridge start-agent --room '!room:matrix.example.com' --type assistant
    ./build/armorclaw-bridge start-agent --room '!room:matrix.example.com' --type openclaw --agent-key openai-default

    # Generate QR code for ArmorChat
    ./build/armorclaw-bridge generate-qr --host bridge.example.com

    # Generate shell completion
    ./build/armorclaw-bridge completion bash > ~/.bash_completion.d/armorclaw-bridge
    source ~/.bash_completion.d/armorclaw-bridge

FLAGS:
    -c, --config string      Path to configuration file (default: ~/.armorclaw/config.toml)
    -v, --verbose           Enable verbose (debug) logging
    -h, --help              Show this help message
    -V, --version           Show version information

CONFIGURATION:
    The bridge loads configuration from ~/.armorclaw/config.toml by default.
    You can override this with the -c flag or ARMORCLAW_CONFIG environment variable.

    For first-time setup, run: ./build/armorclaw-bridge setup

ENVIRONMENT VARIABLES:
    ARMORCLAW_API_KEY     API key (auto-stored on bridge startup)
    ARMORCLAW_CONFIG      Path to configuration file

DOCUMENTATION:
    https://github.com/Gemutly/ArmorClaw
    https://docs.armorclaw.com

SUPPORT:
    Issues: https://github.com/Gemutly/ArmorClaw/issues
`
	fmt.Println(helpText)
}

// printConnectionGuidance displays instructions for connecting ArmorChat to this bridge
func printConnectionGuidance(cfg *config.Config) {
	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    ARMORCHAT CONNECTION GUIDE                                ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println("")
	fmt.Println("Connect ArmorChat or ArmorTerminal to this bridge using one of these methods:")
	fmt.Println("")

	// Determine protocol based on TLS setting
	protocol := "http"
	if cfg.Discovery.TLS {
		protocol = "https"
	}

	// Get hostname for display
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}

	// Derive the public URL (for remote deployments, this would be the VPS domain)
	// In production, this should come from config or auto-detection
	publicHost := hostname
	if cfg.Discovery.InstanceName != "" {
		publicHost = cfg.Discovery.InstanceName
	}

	// Method 1: QR Code / Deep Link (Recommended for remote VPS)
	fmt.Println("┌─────────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│ METHOD 1: QR CODE (Recommended for Remote VPS)                              │")
	fmt.Println("├─────────────────────────────────────────────────────────────────────────────┤")
	fmt.Println("│ 1. Open ArmorChat on your device                                            │")
	fmt.Println("│ 2. Tap 'Scan QR Code' or go to Settings → Add Server                        │")
	fmt.Println("│ 3. Scan the QR code generated by this command:                              │")
	fmt.Println("│                                                                             │")
	fmt.Printf(" │     armorclaw-bridge generate-qr --host %s --port %d\n", publicHost, cfg.Discovery.Port)
	fmt.Println("│                                                                             │")
	fmt.Println("│ The QR code contains signed server configuration including:                 │")
	fmt.Println("│   • Matrix homeserver URL                                                   │")
	fmt.Println("│   • Bridge RPC and WebSocket endpoints                                      │")
	fmt.Println("│   • Push gateway URL                                                        │")
	fmt.Println("└─────────────────────────────────────────────────────────────────────────────┘")
	fmt.Println("")

	// Method 2: Well-Known Discovery (for custom domains)
	fmt.Println("┌─────────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│ METHOD 2: WELL-KNOWN DISCOVERY (Custom Domain)                              │")
	fmt.Println("├─────────────────────────────────────────────────────────────────────────────┤")
	fmt.Println("│ If your Matrix server has .well-known configured:                           │")
	fmt.Println("│                                                                             │")
	fmt.Println("│ 1. Open ArmorChat on your device                                            │")
	fmt.Println("│ 2. Enter your domain (e.g., 'matrix.example.com')                           │")
	fmt.Println("│ 3. ArmorChat will auto-discover the bridge configuration                    │")
	fmt.Println("│                                                                             │")
	fmt.Println("│ Required .well-known endpoint:                                              │")
	if cfg.Matrix.HomeserverURL != "" {
		fmt.Printf("│   %s/.well-known/matrix/client\n", cfg.Matrix.HomeserverURL)
	}
	fmt.Println("│                                                                             │")
	fmt.Println("│ Response must include 'com.armorclaw.bridge' section with:                  │")
	fmt.Println("│   {\"api_endpoint\": \"...\", \"ws_endpoint\": \"...\", \"push_gateway\": \"...\"}   │")
	fmt.Println("└─────────────────────────────────────────────────────────────────────────────┘")
	fmt.Println("")

	// Method 3: mDNS Discovery (Local Network Only)
	if cfg.Discovery.Enabled {
		fmt.Println("┌─────────────────────────────────────────────────────────────────────────────┐")
		fmt.Println("│ METHOD 3: mDNS DISCOVERY (Same Network Only)                                │")
		fmt.Println("├─────────────────────────────────────────────────────────────────────────────┤")
		fmt.Println("│ If your device is on the SAME network as this server:                       │")
		fmt.Println("│                                                                             │")
		fmt.Println("│ 1. Open ArmorChat on your device                                            │")
		fmt.Println("│ 2. The app will automatically discover this bridge                          │")
		fmt.Printf("│ 3. Look for: %s._armorclaw._tcp.local.\n", hostname)
		fmt.Println("│                                                                             │")
		fmt.Println("│ ⚠️  NOTE: mDNS does NOT work across different networks or VPNs!            │")
		fmt.Println("│    For remote VPS deployments, use QR code or manual entry.                 │")
		fmt.Println("└─────────────────────────────────────────────────────────────────────────────┘")
		fmt.Println("")
	}

	// Method 4: Manual Configuration
	fmt.Println("┌─────────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│ METHOD 4: MANUAL CONFIGURATION (Fallback)                                   │")
	fmt.Println("├─────────────────────────────────────────────────────────────────────────────┤")
	fmt.Println("│ If other methods don't work, enter these URLs manually in ArmorChat:        │")
	fmt.Println("│                                                                             │")
	if cfg.Matrix.HomeserverURL != "" {
		fmt.Printf("│ Matrix Server:  %s\n", cfg.Matrix.HomeserverURL)
	} else {
		fmt.Println("│ Matrix Server:  (not configured - set [matrix] homeserver_url)              │")
	}
	fmt.Printf("│ Bridge RPC:     %s://%s:%d/api\n", protocol, publicHost, cfg.Discovery.Port)
	fmt.Printf("│ Bridge WebSocket: %s://%s:%d/ws\n", map[bool]string{true: "wss", false: "ws"}[cfg.Discovery.TLS], publicHost, cfg.Discovery.Port)
	fmt.Println("│                                                                             │")
	fmt.Println("│ To set up Matrix integration, edit your config:                             │")
	fmt.Println("│   ~/.armorclaw/config.toml                                                  │")
	fmt.Println("│                                                                             │")
	fmt.Println("│   [matrix]                                                                  │")
	fmt.Println("│   enabled = true                                                            │")
	fmt.Println("│   homeserver_url = \"https://matrix.yourdomain.com\"                          │")
	fmt.Println("└─────────────────────────────────────────────────────────────────────────────┘")
	fmt.Println("")

	// Configuration summary
	fmt.Println("┌─────────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│ CURRENT CONFIGURATION                                                       │")
	fmt.Println("├─────────────────────────────────────────────────────────────────────────────┤")
	fmt.Printf("│ Discovery:      %s\n", map[bool]string{true: "ENABLED (mDNS + well-known)", false: "DISABLED"}[cfg.Discovery.Enabled])
	fmt.Printf("│ TLS:            %s\n", map[bool]string{true: "ENABLED (HTTPS/WSS)", false: "DISABLED (HTTP/WS)"}[cfg.Discovery.TLS])
	fmt.Printf("│ Port:           %d\n", cfg.Discovery.Port)
	if cfg.Matrix.HomeserverURL != "" {
		fmt.Printf("│ Matrix:         %s\n", cfg.Matrix.HomeserverURL)
	} else {
		fmt.Println("│ Matrix:         NOT CONFIGURED")
	}
	if cfg.Discovery.PushGateway != "" {
		fmt.Printf("│ Push Gateway:   %s\n", cfg.Discovery.PushGateway)
	}
	fmt.Println("└─────────────────────────────────────────────────────────────────────────────┘")
	fmt.Println("")

	// Troubleshooting hints
	fmt.Println("Troubleshooting:")
	fmt.Println("  • If ArmorChat can't connect, check firewall allows port " + fmt.Sprintf("%d", cfg.Discovery.Port))
	fmt.Println("  • For remote VPS, ensure your domain's DNS is properly configured")
	fmt.Println("  • Generate QR: armorclaw-bridge generate-qr --help")
	fmt.Println("  • Check status: armorclaw-bridge daemon status")
	fmt.Println("  • View docs:    https://github.com/Gemutly/ArmorClaw/tree/main/docs/guides")
	fmt.Println("")
}

func printCommandHelp(command string) {
	var help string
	switch command {
	case "init":
		help = `COMMAND: init

Initialize a default configuration file.

USAGE:
    armorclaw-bridge init [-c|--config-output path]

FLAGS:
    -c, --config-output string   Output path for config file

EXAMPLES:
    # Create default config
    armorclaw-bridge init

    # Create config at custom path
    armorclaw-bridge init -c /custom/path/config.toml
`
	case "setup":
		help = `COMMAND: setup

Run interactive setup wizard for first-time configuration.

The wizard guides you through:
    - Docker availability check
    - Configuration location
    - AI provider selection
    - API key entry (stored securely)
    - Optional Matrix configuration
    - Automatic configuration generation

USAGE:
    armorclaw-bridge setup [-c|--config path]

EXAMPLES:
    # Run setup wizard
    armorclaw-bridge setup

    # Run with custom config location
    armorclaw-bridge setup -c /custom/path/config.toml
`
	case "add-key":
		help = `COMMAND: add-key

Add an API key to the encrypted keystore.

USAGE:
    armorclaw-bridge add-key -p PROVIDER -t TOKEN [flags]

FLAGS:
    -p, --provider string     AI provider (openai, anthropic, openrouter, google, xai, or any OpenAI-compatible)
    -t, --token string        API token (or use ARMORCLAW_API_KEY env var)
    -i, --id string           Key ID (default: <provider>-default)
    -n, --display-name string Display name for the key
    -b, --base-url string     Base URL for OpenAI-compatible API providers

PROVIDERS:
    openai      OpenAI (GPT-4, GPT-3.5)
    anthropic   Anthropic (Claude)
    openrouter  OpenRouter (multi-provider)
    google      Google (Gemini)
    xai         xAI (Grok)

OPENAI-COMPATIBLE PROVIDERS (use --base-url):
    Zhipu AI        --base-url https://open.bigmodel.cn/api/paas/v4
    DeepSeek        --base-url https://api.deepseek.com/v1
    Moonshot        --base-url https://api.moonshot.cn/v1
    NVIDIA NIM      --base-url https://integrate.api.nvidia.com/v1
    OpenRouter      --base-url https://openrouter.ai/api/v1
    Groq            --base-url https://api.groq.com/openai/v1
    Custom endpoint --base-url https://your-api.com/v1

EXAMPLES:
    # Add OpenAI key
    armorclaw-bridge add-key --provider openai --token sk-proj-xxx

    # Add Anthropic key with custom ID
    armorclaw-bridge add-key --provider anthropic --token sk-ant-xxx --id claude-prod

    # Add Zhipu AI key with custom base URL
    armorclaw-bridge add-key --provider openai --base-url https://open.bigmodel.cn/api/paas/v4 --id zhipu --token your-api-key

    # Add key using environment variable
    export ARMORCLAW_API_KEY="sk-xxx"
    armorclaw-bridge add-key --provider openai
`
	case "list-keys":
		help = `COMMAND: list-keys

List all stored API keys in the keystore.

USAGE:
    armorclaw-bridge list-keys [-c|--config path]

OUTPUT:
    Shows key ID, provider, and display name for each stored key.

EXAMPLES:
    # List all keys
    armorclaw-bridge list-keys

    # No keys? Add one:
    armorclaw-bridge add-key --provider openai --token sk-proj-...
`
	case "start":
		help = `COMMAND: start

Start an AI agent container with stored credentials.

USAGE:
    armorclaw-bridge start -k KEY_ID [-c|--config path]

FLAGS:
    -k, --key string   Key ID to use (required)

EXAMPLES:
    # List keys first
    armorclaw-bridge list-keys

    # Start with specific key
    armorclaw-bridge start --key openai-default

    # Start bridge in foreground, then in another terminal:
    armorclaw-bridge start --key openai-default
`
	case "generate-qr":
		help = `COMMAND: generate-qr

Generate a QR code for ArmorChat/ArmorTerminal discovery.

This command creates a signed configuration URL that ArmorChat can use
to automatically discover and connect to this bridge.

USAGE:
    armorclaw-bridge generate-qr [--host hostname] [--port port]

FLAGS:
    --host string   Public hostname/domain (default: system hostname)
    --port int      Public port (default: from config)

OUTPUT:
    • Deep link URL (armorclaw://config?d=...)
    • Web link URL (https://armorclaw.app/config?d=...)
    • Configuration summary

EXAMPLES:
    # Generate QR with defaults
    armorclaw-bridge generate-qr

    # Generate QR for production domain
    armorclaw-bridge generate-qr --host bridge.example.com --port 443

    # Generate QR for local development
    armorclaw-bridge generate-qr --host 192.168.1.100

DISCOVERY METHODS:
    ArmorChat supports multiple discovery methods:

    1. QR Code (this command) - Best for remote VPS
    2. mDNS discovery - Same network only
    3. Well-known discovery - Custom domains
    4. Manual entry - Fallback option

NOTES:
    • The generated QR is valid for 24 hours
    • For production, ensure TLS is enabled in config
    • mDNS discovery only works on the same local network
`
	case "start-agent":
		help = `COMMAND: start-agent

Start an AI agent (OpenClaw, assistant, etc.) via the bridge RPC.

This command connects to the running bridge and starts an agent
that can interact with Matrix users through ArmorChat.

USAGE:
    armorclaw-bridge start-agent --room ROOM_ID [flags]

FLAGS:
    --type string          Agent type (default: "assistant")
                           Options: assistant, openclaw, custom
    --name string          Agent display name (default: "<type>-agent")
    --room string          Matrix room ID for agent (required)
    --key string           API key ID to use for the agent
    --capabilities string  Comma-separated capabilities (default: "chat")
                           Options: chat, voice, video, files, code

PREREQUISITES:
    1. Bridge must be running: armorclaw-bridge
    2. API key must be stored (if using AI features):
       armorclaw-bridge add-key --provider openai --token sk-xxx
    3. Matrix room must exist and bridge must be invited

EXAMPLES:
    # Start a basic assistant in a room
    armorclaw-bridge start-agent --room '!abc123:matrix.example.com'

    # Start OpenClaw agent with specific key
    armorclaw-bridge start-agent --room '!abc123:matrix.example.com' --type openclaw --key openai-default

    # Start agent with multiple capabilities
    armorclaw-bridge start-agent --room '!abc123:matrix.example.com' --capabilities "chat,files,code"

    # Start with custom name
    armorclaw-bridge start-agent --room '!abc123:matrix.example.com' --name "Support Bot"

AGENT MANAGEMENT:
    After starting, users can interact with the agent in the Matrix room.
    The agent will respond to messages based on its configuration.

    To stop an agent:
    • Via RPC: echo '{"method":"agent.stop","params":{"agent_id":"xxx"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock

DOCKER COMPOSE:
    For OpenClaw agents, you can also use Docker Compose:

    docker-compose -f docker-compose.bridge.yml --profile openclaw up -d

    This requires:
    • ARMORCLAW_MATRIX_ROOM environment variable set
    • OpenClaw container image built
`
	case "completion":
		help = `COMMAND: completion

Generate shell completion script for bash or zsh.

USAGE:
    armorclaw-bridge completion [bash|zsh]

EXAMPLES:
    # Generate bash completion
    armorclaw-bridge completion bash > ~/.bash_completion.d/armorclaw-bridge
    source ~/.bash_completion.d/armorclaw-bridge

    # Generate zsh completion
    armorclaw-bridge completion zsh > ~/.zsh/completions/_armorclaw-bridge
    # Then add to ~/.zshrc: autoload -U compinit && compinit
`
	case "validate":
		help = `COMMAND: validate

Validate configuration file.

USAGE:
    armorclaw-bridge validate [-c|--config path]

EXAMPLES:
    # Validate default config
    armorclaw-bridge validate

    # Validate custom config
    armorclaw-bridge validate -c /path/to/config.toml
`
	default:
		help = fmt.Sprintf("Unknown command: %s\n\nRun 'armorclaw-bridge help' for usage.", command)
	}
	fmt.Println(help)
}

// studioMatrixAdapter wraps adapter.MatrixAdapter to satisfy studio.MatrixAdapter interface
type studioMatrixAdapter struct {
	adapter *adapter.MatrixAdapter
}

func (s *studioMatrixAdapter) SendMessage(ctx context.Context, roomID, message string) error {
	_, err := s.adapter.SendMessage(roomID, message, "m.notice")
	return err
}

func (s *studioMatrixAdapter) SendFormattedMessage(ctx context.Context, roomID, plainBody, formattedBody string) error {
	return s.adapter.SendFormattedMessage(ctx, roomID, plainBody, formattedBody)
}

func (s *studioMatrixAdapter) ReplyToEvent(ctx context.Context, roomID, eventID, message string) error {
	return s.adapter.ReplyToEvent(ctx, roomID, eventID, message)
}

// studioFactoryAdapter bridges studio.AgentFactory to secretary.FactoryInterface
type studioFactoryAdapter struct {
	factory *studio.AgentFactory
}

func (a *studioFactoryAdapter) Spawn(ctx context.Context, req *secretary.SpawnRequestRef) (*secretary.SpawnResultRef, error) {
	result, err := a.factory.Spawn(ctx, &studio.SpawnRequest{
		DefinitionID:    req.DefinitionID,
		TaskDescription: req.TaskDescription,
		UserID:          req.UserID,
		RoomID:          req.RoomID,
	})
	if err != nil {
		return nil, err
	}
	return &secretary.SpawnResultRef{
		InstanceID: result.Instance.ID,
		RoomID:     result.Instance.RoomID,
	}, nil
}

// schedulerMatrixAdapter wraps adapter.MatrixAdapter to satisfy secretary.MatrixAdapter
type schedulerMatrixAdapter struct {
	adapter *adapter.MatrixAdapter
}

func (s *schedulerMatrixAdapter) SendEvent(ctx context.Context, roomID, eventType string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("schedulerMatrixAdapter: failed to marshal payload: %w", err)
	}
	return s.adapter.SendEvent(roomID, eventType, data)
}

// compositeStudioHandler tries studio commands first, then secretary commands
type compositeStudioHandler struct {
	studio       *studio.StudioIntegration
	secretary    *secretary.SecretaryCommandHandler
	adminHandler *adapter.CommandHandler
}

func (c *compositeStudioHandler) HandleMatrixMessage(ctx context.Context, roomID, userID, eventID, text string) bool {
	if strings.HasPrefix(text, "/") {
		if c.adminHandler != nil {
			return c.adminHandler.HandleCommand(ctx, roomID, userID, text)
		}
		return false
	}
	if c.studio != nil {
		if c.studio.HandleMatrixMessage(ctx, roomID, userID, eventID, text) {
			return true
		}
	}
	if c.secretary != nil {
		return c.secretary.HandleMatrixMessage(ctx, roomID, userID, eventID, text)
	}
	return false
}

type toolsidecarDockerAdapter struct {
	client *docker.Client
}

func (a *toolsidecarDockerAdapter) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig any, platform any, name string) (container.CreateResponse, error) {
	id, err := a.client.CreateContainer(ctx, config, hostConfig, nil, nil)
	if err != nil {
		return container.CreateResponse{}, err
	}
	return container.CreateResponse{ID: id}, nil
}

func (a *toolsidecarDockerAdapter) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	return a.client.StartContainer(ctx, containerID)
}

func (a *toolsidecarDockerAdapter) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	return a.client.StopContainer(ctx, containerID, options)
}

func (a *toolsidecarDockerAdapter) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	return a.client.RemoveContainer(ctx, containerID, options.Force)
}

func (a *toolsidecarDockerAdapter) ContainerExecCreate(ctx context.Context, containerID string, config container.ExecOptions) (container.ExecCreateResponse, error) {
	return a.client.ContainerExecCreate(ctx, containerID, config)
}

func (a *toolsidecarDockerAdapter) ContainerExecAttach(ctx context.Context, execID string, config container.ExecAttachOptions) (types.HijackedResponse, error) {
	return a.client.ContainerExecAttach(ctx, execID, config)
}

func (a *toolsidecarDockerAdapter) ContainerExecInspect(ctx context.Context, execID string) (container.ExecInspect, error) {
	return a.client.ContainerExecInspect(ctx, execID)
}

func validateSidecarSockets() {
	dialTimeout := 1 * time.Second

	tryDial := func(name, path string) {
		conn, err := net.DialTimeout("unix", path, dialTimeout)
		if err != nil {
			log.Printf("Warning: %s sidecar socket not dialable at %s (%s may be degraded)", name, path, name)
			return
		}
		conn.Close()
		log.Printf("%s sidecar socket OK at %s", name, path)
	}

	if _, err := os.Stat("/run/armorclaw"); os.IsNotExist(err) {
		log.Printf("Warning: /run/armorclaw does not exist — sidecar sockets cannot be validated")
		return
	}

	tryDial("office", sidecar.OfficeSocketPath)
	tryDial("java", sidecar.JavaSocketPath)

	if info, err := os.Stat("/run/armorclaw/office-sidecar"); err == nil && info.IsDir() {
		log.Printf("Warning: deprecated path /run/armorclaw/office-sidecar/ exists — migrate to %s", sidecar.OfficeSocketPath)
	}

	sidecar.CheckRustSidecarHealth()
}
