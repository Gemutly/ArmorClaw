#!/bin/bash
# ArmorClaw Setup Wizard
# Interactive guided installation and configuration
# Version: 2.0.0 - Added mode selection

set -euo pipefail

# Configuration
REPO="Gemutly/ArmorClaw"
VERSION="${VERSION:-main}"
LOCKFILE="/tmp/armorclaw-wizard.lock"

# Cleanup handler
cleanup() {
    # Release lock
    flock -u 200 2>/dev/null || true
    
    # Reset terminal state
    stty sane 2>/dev/null || true
    tput cnorm 2>/dev/null || true
}
trap cleanup EXIT

# Acquire lock
exec 200>"$LOCKFILE"
flock -n 200 || {
    echo -e "\n${RED}ERROR:${NC} Another instance of setup-wizard.sh is already running."
    echo "Please wait for it to finish or remove $LOCKFILE if you are sure it is stale."
    exit 1
}
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
BOLD='\033[1m'
NC='\033[0m'

# Global variables
export REPO="Gemutly/ArmorClaw"
export VERSION="${VERSION:-main}"
CONFIG_DIR="/etc/armorclaw"
DATA_DIR="/var/lib/armorclaw"
RUN_DIR="/run/armorclaw"
SOCKET_PATH="$RUN_DIR/bridge.sock"
LOG_FILE="/var/log/armorclaw-setup.log"
INSTALL_DIR="/opt/armorclaw"
SIGNING_KEY_FPR="A1482657223EAFE1C481B74A8F535F90685749E0"

# Setup mode (quick/standard/expert)
SETUP_MODE="standard"

# Import state flags
SETTINGS_IMPORTED="false"
IMPORT_CONFIG="false"
IMPORT_KEYSTORE="false"
IMPORT_SALT="false"
IMPORT_AGENT_CONFIGS="false"
IMPORT_MATRIX="false"

# Required commands
REQUIRED_COMMANDS=("docker" "docker compose" "go" "tar" "systemctl")

#=============================================================================
# Helper Functions
#=============================================================================

# Helper for interactive prompts (handles curl | bash and non-interactive envs)
prompt_read() {
    if [ -t 0 ] || [ -c /dev/tty ]; then
        read "$@" < /dev/tty
    fi
}

# Helper: Retry logic for network operations
retry() {
    local n=1
    local max=3
    local delay=2
    while true; do
        "$@" && return 0
        if (( n == max )); then
            return 1
        fi
        echo -e "  [armorclaw] Retrying in ${delay}s... ($n/$max)"
        sleep $delay
        ((n++)) || true
    done
}

detect_arch() {
    local arch=$(uname -m)
    case "$arch" in
        x86_64|amd64) BIN_ARCH="linux-amd64" ;;
        aarch64|arm64) BIN_ARCH="linux-arm64" ;;
        *)
            print_warning "Unsupported architecture for binary distribution: $arch"
            BIN_ARCH=""
            ;;
    esac
}

download_binary() {
    if [[ -z "$BIN_ARCH" ]]; then
        return 1
    fi

    local bin_name="armorclaw-bridge-$BIN_ARCH"
    local base_url="https://github.com/$REPO/releases/download/$VERSION"
    
    # Use latest release if VERSION is main
    if [[ "$VERSION" == "main" ]]; then
        base_url="https://github.com/$REPO/releases/latest/download"
    fi

    print_info "Downloading prebuilt binary: $bin_name"
    
    local tmp_dir=$(mktemp -d)
    local bin_path="$tmp_dir/$bin_name"
    local checksum_path="$tmp_dir/checksums.txt"
    local sig_path="$tmp_dir/checksums.txt.sig"
    local key_path="$tmp_dir/armorclaw-signing-key.asc"
    
    # Use strict curl flags
    local curl_base="curl --proto =https --tlsv1.2 --fail --silent --show-error --location"

    if ! retry $curl_base "$base_url/$bin_name" -o "$bin_path"; then
        print_warning "Failed to download binary"
        rm -rf "$tmp_dir"
        return 1
    fi
    
    if ! retry $curl_base "$base_url/checksums.txt" -o "$checksum_path"; then
        print_warning "Failed to download checksums"
        rm -rf "$tmp_dir"
        return 1
    fi
    
    if ! retry $curl_base "$base_url/checksums.txt.sig" -o "$sig_path"; then
        print_warning "Failed to download signature"
        rm -rf "$tmp_dir"
        return 1
    fi
    
    retry $curl_base "https://raw.githubusercontent.com/$REPO/main/deploy/armorclaw-signing-key.asc" -o "$key_path"

    # Verify Checksum
    print_info "Verifying binary checksum..."
    if ! grep "$bin_name" "$checksum_path" | (cd "$tmp_dir" && sha256sum -c - >/dev/null 2>&1); then
        print_error "Binary checksum verification failed!"
        rm -rf "$tmp_dir"
        return 1
    fi
    print_done "Checksum OK"

    # Verify Signature
    print_info "Verifying release signature..."
    local gnupg_home="$tmp_dir/gnupg"
    mkdir -p "$gnupg_home"
    chmod 700 "$gnupg_home"
    
    gpg --homedir "$gnupg_home" --batch --import "$key_path" >/dev/null 2>&1
    
    # Verify fingerprint
    local fpr_check=$(gpg --homedir "$gnupg_home" --with-colons --fingerprint releases@armorclaw.ai | grep "^fpr" | cut -d: -f10)
    if [[ "$fpr_check" != "$SIGNING_KEY_FPR" ]]; then
        print_error "Unauthorized signing key detected!"
        rm -rf "$tmp_dir"
        return 1
    fi

    if ! gpg --homedir "$gnupg_home" --batch --verify "$sig_path" "$checksum_path" >/dev/null 2>&1; then
        print_error "GPG signature verification failed for checksums.txt!"
        rm -rf "$tmp_dir"
        return 1
    fi
    print_done "Signature Verified"

    # Move to final location
    $SUDO mkdir -p "$INSTALL_DIR"
    $SUDO install -m 755 "$bin_path" "$INSTALL_DIR/armorclaw-bridge"
    $SUDO ln -sf "$INSTALL_DIR/armorclaw-bridge" /usr/local/bin/armorclaw-bridge
    
    rm -rf "$tmp_dir"
    return 0
}

print_header() {
    echo -e "${CYAN}╔══════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC}        ${BOLD}ArmorClaw Setup Wizard${NC}                      ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}        ${BOLD}Version 1.0.0${NC}                                  ${CYAN}║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════════╝${NC}"
    echo ""
}

print_step() {
    local step="$1"
    local total="$2"
    echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  Step $step of $total: ${BOLD}$3${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
}

print_success() {
    echo -e "\n${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "\n${RED}✗ ERROR: $1${NC}" >&2
}

print_warning() {
    echo -e "\n${YELLOW}⚠ WARNING: $1${NC}"
}

print_info() {
    echo -e "\n${CYAN}ℹ $1${NC}"
}

prompt_yes_no() {
    local prompt="$1"
    local default="${2:-n}"

    while true; do
        if [ "$default" = "y" ]; then
            echo -ne "${CYAN}$prompt [Y/n]: ${NC}"
        else
            echo -ne "${CYAN}$prompt [y/N]: ${NC}"
        fi

        prompt_read -r response
        response=${response:-$default}

        case "$response" in
            [Yy]|[Yy][Ee][Ss]) return 0 ;;
            [Nn]|[Nn][Oo]) return 1 ;;
        esac

        echo -e "${YELLOW}Please answer yes or no.${NC}"
    done
}

prompt_input() {
    local prompt="$1"
    local default="$2"
    local result

    if [ -n "$default" ]; then
        echo -ne "${CYAN}$prompt [$default]: ${NC}"
    else
        echo -ne "${CYAN}$prompt: ${NC}"
    fi

    prompt_read -r result
    echo "${result:-$default}"
}

prompt_password() {
    local prompt="$1"
    local password
    local confirm

    while true; do
        prompt_read -s -p "$prompt: " password
        echo
        prompt_read -s -p "Confirm: " confirm
        echo

        if [ "$password" = "$confirm" ]; then
            if [ ${#password} -ge 8 ]; then
                echo "$password"
                return 0
            else
                print_error "Password must be at least 8 characters"
            fi
        else
            print_error "Passwords do not match"
        fi
    done
}

check_command() {
    local cmd="$1"
    if ! command -v "$cmd" &> /dev/null; then
        return 1
    fi
    return 0
}

ensure_error_store_config() {
    local config_file="${CONFIG_DIR}/config.toml"

    $SUDO mkdir -p "$CONFIG_DIR"
    $SUDO touch "$config_file"

    if ! grep -q "^\[errors\]" "$config_file" 2>/dev/null; then
        $SUDO tee -a "$config_file" > /dev/null <<EOF

[errors]
store_path = "$DATA_DIR/errors.db"
EOF
    fi
}

#=============================================================================
# Logging
#=============================================================================

init_logging() {
    mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null || true
    exec > >(tee -a "$LOG_FILE")
    exec 2>&1
}

#=============================================================================
# Mode Selection
#=============================================================================

select_setup_mode() {
    clear 2>/dev/null || true

    echo -e "${CYAN}╔══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC}              ${BOLD}ArmorClaw Setup Wizard${NC}                              ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}              ${BOLD}Version 2.0.0${NC}                                        ${CYAN}║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    echo -e "${BOLD}Choose your setup experience:${NC}"
    echo ""

    echo -e "  ${GREEN}[1]${NC} ${BOLD}Quick Setup${NC} ${CYAN}(Recommended)${NC}"
    echo "      → 2-3 minutes, secure defaults, minimal prompts"
    echo "      → Best for: First-time users, local development"
    echo ""

    echo -e "  ${YELLOW}[2]${NC} ${BOLD}Standard Setup${NC}"
    echo "      → 5-10 minutes, guided configuration with explanations"
    echo "      → Best for: Production deployments with custom needs"
    echo ""

    echo -e "  ${MAGENTA}[3]${NC} ${BOLD}Expert Setup${NC}"
    echo "      → 10-15 minutes, full control over all settings"
    echo "      → Best for: Advanced users, complex configurations"
    echo ""

    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    local choice
    while true; do
        echo -ne "${CYAN}Enter choice [1/2/3] (default: 1): ${NC}"
        read -r choice
        choice=${choice:-1}

        case "$choice" in
            1|quick)
                SETUP_MODE="quick"
                echo ""
                echo -e "${GREEN}✓ Quick Setup selected${NC}"
                echo ""
                return 0
                ;;
            2|standard)
                SETUP_MODE="standard"
                echo ""
                echo -e "${YELLOW}✓ Standard Setup selected${NC}"
                echo ""
                return 0
                ;;
            3|expert)
                SETUP_MODE="expert"
                echo ""
                echo -e "${MAGENTA}✓ Expert Setup selected${NC}"
                echo ""
                return 0
                ;;
            *)
                echo -e "${YELLOW}Please enter 1, 2, or 3${NC}"
                ;;
        esac
    done
}

run_quick_setup() {
    print_info "Launching Quick Setup..."
    echo ""

    # Find the quick setup script
    local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local quick_script="$script_dir/setup-quick.sh"

    if [[ -x "$quick_script" ]]; then
        exec "$quick_script" "$@"
    else
        print_error "Quick setup script not found: $quick_script"
        print_info "Continuing with standard setup..."
        SETUP_MODE="standard"
    fi
}

#=============================================================================
# Import Agent Settings from Backup
#=============================================================================

import_agent_settings() {
    local zip_path="$1"
    local temp_dir
    temp_dir=$(mktemp -d)

    print_info "Extracting backup from: $zip_path"

    # Validate zip file
    if [ ! -f "$zip_path" ]; then
        print_error "Backup file not found: $zip_path"
        rm -rf "$temp_dir"
        return 1
    fi

    # Extract the zip
    if ! unzip -q "$zip_path" -d "$temp_dir"; then
        print_error "Failed to extract backup file. Ensure it's a valid .zip archive."
        rm -rf "$temp_dir"
        return 1
    fi

    # Validate backup structure
    local backup_manifest="$temp_dir/armorclaw-backup/manifest.json"
    if [ ! -f "$backup_manifest" ]; then
        print_error "Invalid backup: missing manifest.json"
        print_info "Expected structure: armorclaw-backup/manifest.json"
        rm -rf "$temp_dir"
        return 1
    fi

    # Read backup info
    local backup_version backup_date
    if command -v jq &> /dev/null; then
        backup_version=$(jq -r '.version // "unknown"' "$backup_manifest" 2>/dev/null)
        backup_date=$(jq -r '.created_at // "unknown"' "$backup_manifest" 2>/dev/null)
    else
        # Fallback parsing without jq
        backup_version=$(grep -o '"version"[[:space:]]*:[[:space:]]*"[^"]*"' "$backup_manifest" | head -1 | sed 's/.*: *"\([^"]*\)".*/\1/')
        backup_date=$(grep -o '"created_at"[[:space:]]*:[[:space:]]*"[^"]*"' "$backup_manifest" | head -1 | sed 's/.*: *"\([^"]*\)".*/\1/')
    fi

    print_success "Backup found: version $backup_version, created $backup_date"

    # List what will be imported
    echo ""
    print_info "${BOLD}Backup Contents:${NC}"

    if [ -f "$temp_dir/armorclaw-backup/config/config.toml" ]; then
        echo "  ${GREEN}✓${NC} Configuration file (config.toml)"
        IMPORT_CONFIG="true"
    fi

    if [ -f "$temp_dir/armorclaw-backup/keystore/keystore.db" ]; then
        echo "  ${GREEN}✓${NC} Encrypted keystore database"
        IMPORT_KEYSTORE="true"
    fi

    if [ -f "$temp_dir/armorclaw-backup/keystore/keystore.db.salt" ]; then
        echo "  ${GREEN}✓${NC} Keystore salt file"
        IMPORT_SALT="true"
    fi

    if [ -d "$temp_dir/armorclaw-backup/agent-configs" ]; then
        local config_count
        config_count=$(find "$temp_dir/armorclaw-backup/agent-configs" -name "*.json" 2>/dev/null | wc -l)
        if [ "$config_count" -gt 0 ]; then
            echo "  ${GREEN}✓${NC} Agent configurations ($config_count files)"
            IMPORT_AGENT_CONFIGS="true"
        fi
    fi

    if [ -f "$temp_dir/armorclaw-backup/matrix/session.json" ]; then
        echo "  ${GREEN}✓${NC} Matrix session data"
        IMPORT_MATRIX="true"
    fi

    echo ""

    # Security warning
    print_warning "${BOLD}Security Notice:${NC}"
    echo "  • The keystore is hardware-bound to the original machine"
    echo "  • API keys encrypted on another system cannot be decrypted here"
    echo "  • You will need to re-add API keys after import"
    echo "  • Configuration and agent settings will be restored"
    echo ""

    if ! prompt_yes_no "Proceed with import?"; then
        print_info "Import cancelled"
        rm -rf "$temp_dir"
        return 1
    fi

    # Create target directories
    $SUDO mkdir -p "$CONFIG_DIR"
    $SUDO mkdir -p "$DATA_DIR"
    $SUDO mkdir -p "$DATA_DIR/agent-configs"

    # Import configuration
    if [ "$IMPORT_CONFIG" = "true" ]; then
        print_info "Importing configuration..."
        $SUDO cp "$temp_dir/armorclaw-backup/config/config.toml" "$CONFIG_DIR/config.toml"
        $SUDO chown armorclaw:armorclaw "$CONFIG_DIR/config.toml" 2>/dev/null || true
        $SUDO chmod 640 "$CONFIG_DIR/config.toml"
        print_success "Configuration imported"
        SETTINGS_IMPORTED="true"
    fi

    # Import keystore (structure only, keys need re-adding)
    if [ "$IMPORT_KEYSTORE" = "true" ]; then
        print_info "Importing keystore structure..."
        # Note: The encrypted keys won't work on new hardware
        # We import the structure but user must re-add keys
        print_warning "Keystore imported but keys are hardware-bound"
        print_info "You will need to re-add your API keys in Step 9"
        SETTINGS_IMPORTED="true"
    fi

    # Import agent configurations
    if [ "$IMPORT_AGENT_CONFIGS" = "true" ]; then
        print_info "Importing agent configurations..."
        $SUDO cp -r "$temp_dir/armorclaw-backup/agent-configs/"* "$DATA_DIR/agent-configs/" 2>/dev/null || true
        $SUDO chown -R armorclaw:armorclaw "$DATA_DIR/agent-configs" 2>/dev/null || true
        print_success "Agent configurations imported"
        SETTINGS_IMPORTED="true"
    fi

    # Import Matrix session (if applicable)
    if [ "$IMPORT_MATRIX" = "true" ]; then
        print_info "Importing Matrix session..."
        $SUDO mkdir -p "$DATA_DIR/matrix"
        $SUDO cp "$temp_dir/armorclaw-backup/matrix/session.json" "$DATA_DIR/matrix/" 2>/dev/null || true
        $SUDO chown -R armorclaw:armorclaw "$DATA_DIR/matrix" 2>/dev/null || true
        print_success "Matrix session imported"
        SETTINGS_IMPORTED="true"
    fi

    # Cleanup
    rm -rf "$temp_dir"

    print_success "Import complete!"
    echo ""
    print_info "${BOLD}What was restored:${NC}"
    echo "  • Bridge configuration"
    echo "  • Agent configuration profiles"
    if [ "$IMPORT_MATRIX" = "true" ]; then
        echo "  • Matrix session data"
    fi
    echo ""
    print_info "${BOLD}What needs attention:${NC}"
    echo "  • API keys must be re-added (hardware-bound encryption)"
    echo "  • Verify Matrix credentials if changed"
    echo ""

    return 0
}

#=============================================================================
# Step 1: Welcome and Overview
#=============================================================================

step_welcome() {
    local total_steps=13
    if [[ "$SETUP_MODE" == "expert" ]]; then
        total_steps=14
    fi

    print_step 1 $total_steps "Welcome and Overview"

    cat <<EOF
${BOLD}Welcome to ArmorClaw!${NC}

${BOLD}Setup Mode:${NC} ${SETUP_MODE^}

ArmorClaw is a secure containerization system for AI agents. This wizard will guide you through:

  1. System requirements check
  2. Docker installation/verification
  3. Container image setup
  4. Bridge installation
  5. Budget confirmation (FINANCIAL RESPONSIBILITY)
  6. Configuration file creation
  7. Keystore initialization
  8. First API key setup
  9. Systemd service setup
  10. Post-installation verification
EOF

    if [[ "$SETUP_MODE" == "expert" ]]; then
        echo "  11. Advanced features (WebRTC Voice, Host Hardening, Production Logging)"
        echo "  12. Start first agent (optional)"
    else
        echo "  11. Start first agent (optional)"
        echo ""
        echo -e "  ${CYAN}(Advanced features can be configured later with separate scripts)${NC}"
    fi

    cat <<EOF

${BOLD}Estimated time:${NC} $([[ "$SETUP_MODE" == "expert" ]] && echo "10-15" || echo "5-10") minutes
${BOLD}Configuration directory:${NC} /etc/armorclaw
${BOLD}Data directory:${NC} /var/lib/armorclaw

${YELLOW}Note:${NC} You can cancel at any time by pressing Ctrl+C
EOF

    echo ""

    # Check for unzip command
    if ! check_command unzip; then
        print_info "Installing unzip for backup import support..."
        $SUDO apt-get update -qq && sudo apt-get install -y -qq unzip 2>/dev/null || \
        $SUDO yum install -y -q unzip 2>/dev/null || \
        print_warning "Could not install unzip; import feature may be limited"
    fi

    # Offer import option
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BOLD}Import Existing Settings${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo "If you have a backup from a previous ArmorClaw installation, you can"
    echo "import your settings now. This will restore:"
    echo ""
    echo "  ${GREEN}✓${NC} Configuration file (config.toml)"
    echo "  ${GREEN}✓${NC} Agent configuration profiles"
    echo "  ${GREEN}✓${NC} Matrix session data"
    echo "  ${GREEN}✓${NC} Budget and logging preferences"
    echo ""
    echo "  ${YELLOW}⚠${NC} API keys cannot be transferred (hardware-bound encryption)"
    echo ""

    if prompt_yes_no "Import settings from a backup (.zip)?"; then
        local backup_path
        echo ""
        echo -ne "${CYAN}Enter path to backup file: ${NC}"
        read -r backup_path

        # Expand ~ to home directory
        backup_path="${backup_path/#\~/$HOME}"

        if [ -n "$backup_path" ]; then
            if import_agent_settings "$backup_path"; then
                print_success "Settings imported successfully"
                print_info "Setup will skip configuration steps for imported items"
            else
                print_warning "Import failed or cancelled, continuing with fresh setup"
            fi
        else
            print_info "No backup path provided, continuing with fresh setup"
        fi
        echo ""
    fi

    echo ""
    if ! prompt_yes_no "Continue with setup?"; then
        print_info "Setup cancelled by user"
        exit 0
    fi
}

#=============================================================================
# Step 2: System Requirements Check
#=============================================================================

step_prerequisites() {
    print_step 2 13 "System Requirements Check"

    local all_ok=true

    # Check OS
    print_info "Checking operating system..."
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        print_success "OS: $PRETTY_NAME"

        case "$ID" in
            ubuntu|debian)
                print_success "Supported OS detected"
                ;;
            *)
                print_warning "You are using $PRETTY_NAME. Official support is for Ubuntu 22.04+ and Debian 12+."
                if ! prompt_yes_no "Continue anyway?"; then
                    exit 1
                fi
                ;;
        esac
    else
        print_error "Cannot detect OS. /etc/os-release not found."
        exit 1
    fi

    # Check RAM
    print_info "Checking memory..."
    local total_mem=$(free -m | awk '/^Mem:/{print $2}')
    if [ "$total_mem" -ge 2048 ]; then
        print_success "Memory: ${total_mem}MB (recommended: 2048MB+)"
    elif [ "$total_mem" -ge 1024 ]; then
        print_warning "Memory: ${total_mem}MB (below 2048MB recommendation, but should work)"
    else
        print_error "Memory: ${total_mem}MB (minimum 1024MB required)"
        all_ok=false
    fi

    # Check disk space
    print_info "Checking disk space..."
    local avail_space=$(df -m / | awk 'NR==2 {print $4}')
    if [ "$avail_space" -ge 5120 ]; then
        print_success "Disk space: ${avail_space}MB available"
    else
        print_warning "Disk space: ${avail_space}MB available (5GB+ recommended)"
    fi

    # Check CPU
    print_info "Checking CPU..."
    local cpu_cores=$(nproc)
    if [ "$cpu_cores" -ge 2 ]; then
        print_success "CPU: $cpu_cores cores"
    else
        print_warning "CPU: $cpu_cores core (2+ cores recommended)"
    fi

    # Determine sudo usage
    if [ "$EUID" -ne 0 ]; then
        if command -v sudo >/dev/null 2>&1; then
            SUDO="sudo"
            print_success "Sudo detected (elevation will be used when needed)"
        else
            print_error "This script requires root or sudo access"
            all_ok=false
        fi
    else
        SUDO=""
        print_warning "Running as root is not recommended. Consider running as a normal user."
    fi

    if [ "$all_ok" = false ]; then
        print_error "Prerequisites check failed. Please fix the issues above and run again."
        exit 1
    fi

    print_success "All prerequisites met!"
}

#=============================================================================
# Step 3: Docker Verification
#=============================================================================

step_docker() {
    print_step 3 13 "Docker Verification"

    if check_command docker; then
        local docker_version=$(docker --version | awk '{print $3}' | sed 's/,//')
        print_success "Docker installed: $docker_version"
    else
        print_info "Docker not found. Would you like to install it now?"
        if prompt_yes_no "Install Docker?" "y"; then
            print_info "Installing Docker..."
            curl -fsSL https://get.docker.com -o get-docker.sh
            $SUDO sh get-docker.sh
            rm get-docker.sh

            # Add user to docker group
            local user="$SUDO_USER"
            if [ -z "$user" ]; then
                user="$USER"
            fi
            if [ -n "$user" ]; then
                $SUDO usermod -aG docker "$user"
                print_warning "You will need to log out and log back in for group changes to take effect"
            fi

            print_success "Docker installed"
        else
            print_error "Docker is required for ArmorClaw"
            exit 1
        fi
    fi

    # Check if Docker is running
    print_info "Checking if Docker daemon is running..."
    if $SUDO docker info &> /dev/null; then
        print_success "Docker daemon is running"
    else
        print_error "Docker daemon is not running"
        print_info "Start it with: $SUDO systemctl start docker"
        exit 1
    fi

    # Check Docker Compose
    if check_command "docker compose" || check_command docker-compose; then
        print_success "Docker Compose available"
    else
        print_warning "Docker Compose not found (optional, recommended for full stack deployment)"
    fi

    # Test Docker with a simple container
    print_info "Testing Docker with hello-world container..."
    if $SUDO docker run --rm hello-world &> /dev/null; then
        print_success "Docker is working correctly"
    else
        print_error "Docker test failed"
        exit 1
    fi
}

#=============================================================================
# Step 4: Container Image Setup
#=============================================================================

step_container() {
    print_step 4 13 "Container Image Setup"

    # Check if image exists locally
    print_info "Checking for ArmorClaw agent container image..."
    if $SUDO docker images | grep -q "armorclaw/agent"; then
        print_success "Container image found locally"
        $SUDO docker images | grep "armorclaw/agent"
    else
        print_info "Container image not found locally"

        if [ -f "Dockerfile" ]; then
            print_info "Dockerfile found in current directory"
            if prompt_yes_no "Build container image now? (takes ~5 minutes)"; then
                print_info "Building container image..."
                print_info "This may take several minutes on first build..."

                if $SUDO docker build -t armorclaw/agent:v1 .; then
                    print_success "Container image built successfully"
                else
                    print_error "Container build failed"
                    exit 1
                fi
            fi
        else
            print_info "No Dockerfile found. You can build the image later:"
            echo "  cd armorclaw"
            echo "  docker build -t armorclaw/agent:v1 ."
        fi
    fi

    # Show container info
    if $SUDO docker images | grep -q "armorclaw/agent"; then
        echo ""
        echo -e "${BOLD}Container Image Details:${NC}"
        $SUDO docker images armorclaw/agent:v1
        echo ""
    fi
}

#=============================================================================
# Step 5: Bridge Installation
#=============================================================================

step_bridge_install() {
    print_step 5 13 "Bridge Installation"

    # Check if already installed
    if [ -f "/usr/local/bin/armorclaw-bridge" ] || [ -f "/opt/armorclaw/armorclaw-bridge" ]; then
        print_info "Bridge binary found"
        if prompt_yes_no "Reinstall bridge?"; then
            $SUDO rm -f /usr/local/bin/armorclaw-bridge /opt/armorclaw/armorclaw-bridge
        else
            print_success "Using existing bridge installation"
            return
        fi
    fi

    # Check if we need to build or use pre-built binary
    print_info "Bridge binary installation options:"
    echo "  1. Build from source (requires Go 1.21+)"
    echo "  2. Use pre-built binary (if available)"
    echo ""

    local choice=$(prompt_input "Choose option [1/2]" "2")

    case "$choice" in
        1)
            if check_command go; then
                local go_version=$(go version | awk '{print $3}')
                print_success "Go installed: $go_version"
            else
                print_error "Go is not installed. Please install Go 1.21+ or use option 2."
                exit 1
            fi

            print_info "Building bridge from source..."
            cd bridge
            if go build -o armorclaw-bridge ./cmd/bridge; then
                print_success "Bridge built successfully"

                # Install to system location
                print_info "Installing to /opt/armorclaw/..."
                $SUDO mkdir -p /opt/armorclaw
                $SUDO mv armorclaw-bridge /opt/armorclaw/
                $SUDO chmod +x /opt/armorclaw/armorclaw-bridge
                $SUDO ln -sf /opt/armorclaw/armorclaw-bridge /usr/local/bin/armorclaw-bridge
                print_success "Bridge installed"
            else
                print_error "Bridge build failed"
                exit 1
            fi
            cd ..
            ;;
        2)
            print_info "Installing pre-built binary..."
            detect_arch
            if [[ -n "$BIN_ARCH" ]]; then
                if download_binary; then
                    print_success "Bridge installed via binary distribution"
                else
                    print_error "Binary installation failed"
                    print_info "Please try building from source (option 1)"
                    exit 1
                fi
            else
                print_error "Unsupported architecture for binary distribution"
                exit 1
            fi
            ;;
        *)
            print_error "Invalid option"
            exit 1
            ;;
    esac

    # Create system user
    print_info "Creating system user..."
    if id "armorclaw" &>/dev/null; then
        print_success "User 'armorclaw' already exists"
    else
        $SUDO useradd -r -s /bin/false -d /var/lib/armorclaw armorclaw
        print_success "User 'armorclaw' created"
    fi
}

#=============================================================================
# Step 6: Budget Confirmation
#=============================================================================

step_budget_confirmation() {
    print_step 6 13 "Budget Confirmation"

    cat <<'EOF'
${BOLD}CRITICAL: Financial Responsibility${NC}

ArmorClaw provides token budget tracking, but you MUST set hard limits
in your AI provider dashboard to prevent unexpected charges.

${YELLOW}Please verify you have set (or will set) the following:${NC}

EOF

    echo -e "  1. ${CYAN}OpenAI${NC}: https://platform.openai.com/settings/limits"
    echo -e "  2. ${CYAN}Anthropic${NC}: https://console.anthropic.com/settings/limits"
    echo ""
    echo -e "${BOLD}Provider Cost Configuration${NC}"
    echo ""
    echo "ArmorClaw tracks token usage with default provider costs:"
    echo "  • gpt-4:         \$30.00 per 1M tokens"
    echo "  • gpt-3.5-turbo:  \$2.00 per 1M tokens"
    echo "  • claude-3-opus:  \$15.00 per 1M tokens"
    echo "  • claude-3-sonnet: \$3.00 per 1M tokens"
    echo ""
    echo "You can customize these later in config.toml under [budget.provider_costs]"

    print_info "Set your hard monthly limit to a reasonable amount (e.g., \$100)"
    echo ""

    if ! prompt_yes_no "I have set (or will set) hard limits in my provider dashboard"; then
        print_error "Budget confirmation required"
        print_info "Please set hard limits before continuing"
        exit 1
    fi

    print_success "Budget confirmation complete"
}

#=============================================================================
# Step 7: Configuration File Creation
#=============================================================================

step_configuration() {
    print_step 7 13 "Configuration File Creation"

    print_info "Creating configuration directory..."
    $SUDO mkdir -p "$CONFIG_DIR"
    $SUDO mkdir -p "$DATA_DIR"
    $SUDO mkdir -p "$RUN_DIR"

    # Ensure armorclaw user exists
    id -u armorclaw >/dev/null 2>&1 || $SUDO useradd --system --no-create-home --shell /usr/sbin/nologin armorclaw

    $SUDO chown armorclaw:armorclaw "$RUN_DIR" "$DATA_DIR" 2>/dev/null || true
    $SUDO chmod 700 "$DATA_DIR"
    $SUDO chmod 770 "$RUN_DIR"

    local config_file="$CONFIG_DIR/config.toml"

    # Skip if config was imported from backup
    if [ "$IMPORT_CONFIG" = "true" ] && [ -f "$config_file" ]; then
        print_success "Configuration imported from backup"
        print_info "Reviewing imported configuration..."
        echo ""
        cat "$config_file"
        echo ""
        if prompt_yes_no "Use imported configuration as-is?"; then
            print_success "Using imported configuration"
            return
        fi
        print_info "You can customize the imported configuration"
    fi

    # Get configuration values
    print_info "${BOLD}Configuration Options:${NC}"

    local socket_path=$(prompt_input "Socket path" "$SOCKET_PATH")

    # Enhanced logging configuration
    echo ""
    print_info "${BOLD}Logging Configuration${NC}"
    echo -e "  ${CYAN}Level:${NC}   debug, info, warn, error"
    echo -e "  ${CYAN}Format:${NC}  text (human-readable) or json (structured for log aggregation)"
    echo -e "  ${CYAN}Output:${NC}  stdout (console) or file:/path/to/log (persistent)"
    echo ""
    print_info "${CYAN}Recommendation:${NC} Use JSON format for production with log aggregation"
    print_info "Security events are always logged regardless of level"

    local log_level=$(prompt_input "Log level [debug/info/warn/error]" "info")
    local log_format=$(prompt_input "Log format [text/json]" "text")
    local log_output=$(prompt_input "Log output [stdout]" "stdout")

    # Ask about file logging if JSON format selected
    local log_file=""
    if [ "$log_format" = "json" ]; then
        if prompt_yes_no "Enable persistent file logging for production?"; then
            log_file="/var/log/armorclaw/bridge.log"
            log_output=$(prompt_input "Log file path" "$log_file")
        fi
    fi

    local daemonize=$(prompt_input "Run as daemon [true/false]" "false")

    # Matrix configuration (optional)
    echo ""
    print_info "Matrix Configuration (optional - for remote communication)"
    local matrix_enabled="false"
    local matrix_url=""
    local matrix_user=""
    local matrix_pass=""

    if prompt_yes_no "Enable Matrix communication?"; then
        matrix_enabled="true"
        local matrix_url=$(prompt_input "Matrix homeserver URL" "https://matrix.example.com")
        local matrix_user=$(prompt_input "Matrix bridge username" "bridge")
        local matrix_pass=""
        echo ""
        print_info "Enter Matrix password (leave empty to set later):"
        matrix_pass=$(prompt_password "  Password")

        # Zero-trust configuration
        echo ""
        print_info "${BOLD}Zero-Trust Security Configuration${NC}"
        print_info "ArmorClaw can restrict Matrix communication to trusted senders and rooms."
        print_info "This provides an additional layer of security for remote agent control."
        echo ""
        print_info "${BOLD}Wildcards for Trusted Senders:${NC}"
        echo "  • @alice:example.com       - Specific user only"
        echo "  • *@admin.example.com     - All users from admin domain"
        echo "  • *:example.com            - Everyone on homeserver"
        echo ""
        echo -e "${YELLOW}Security:${NC} Empty allowlist = allow all (default for testing)"
        echo -e "${GREEN}Recommendation:${NC} Start with specific users, expand as needed"
        echo ""

        local enable_trust="false"
        local trusted_senders=""
        local trusted_rooms=""
        local reject_untrusted="false"

        if prompt_yes_no "Enable zero-trust sender/room filtering?"; then
            enable_trust="true"
            echo ""
            print_info "Enter trusted Matrix user IDs (one per line, empty line to finish):"
            print_info "${CYAN}Format:${NC} @user:domain.com, *@trusted.domain.com, or *:domain.com for wildcards"
            echo ""
            while IFS= read -r line; do
                [ -z "$line" ] && break
                trusted_senders="$trusted_senders  \"$line\","
            done

            echo ""
            print_info "Enter trusted room IDs (one per line, empty line to finish):"
            print_info "${CYAN}Format:${NC} !roomid:domain.com"
            echo ""
            while IFS= read -r line; do
                [ -z "$line" ] && break
                trusted_rooms="$trusted_rooms  \"$line\","
            done

            echo ""
            if prompt_yes_no "Send rejection message to untrusted senders?"; then
                reject_untrusted="true"
            fi
        fi
    fi

    # Budget configuration
    echo ""
    print_info "${BOLD}Budget Guardrails Configuration${NC}"
    print_info "ArmorClaw provides token budget tracking to prevent unexpected API costs."
    echo ""
    print_info "IMPORTANT: You MUST ALSO set hard limits in your AI provider dashboard!"
    print_info "  • OpenAI: https://platform.openai.com/settings/limits"
    print_info "  • Anthropic: https://console.anthropic.com/settings/limits"
    echo ""

    local budget_daily=$(prompt_input "Daily budget limit in USD (0 = no limit)" "5.00")
    local budget_monthly=$(prompt_input "Monthly budget limit in USD (0 = no limit)" "100.00")
    local budget_hardstop="true"

    if ! prompt_yes_no "Enable hard-stop when budget exceeded? (recommended)"; then
        budget_hardstop="false"
    fi

    # Create configuration file
    print_info "Creating configuration file: $config_file"
    $SUDO tee "$config_file" > /dev/null <<EOF
# ArmorClaw Bridge Configuration
# Generated by setup wizard on $(date)

[server]
socket_path = "$socket_path"
daemonize = $daemonize

[keystore]
db_path = "$DATA_DIR/keystore.db"

[matrix]
enabled = $matrix_enabled
EOF

    if [ "$matrix_enabled" = "true" ]; then
        $SUDO tee -a "$config_file" > /dev/null <<EOF
homeserver_url = "$matrix_url"
username = "$matrix_user"
password = "$matrix_pass"
EOF
    fi

    # Zero-trust configuration
    if [ "$matrix_enabled" = "true" ] && [ "$enable_trust" = "true" ]; then
        $SUDO tee -a "$config_file" > /dev/null <<EOF

[matrix.zero_trust]
trusted_senders = [
$trusted_senders
]
trusted_rooms = [
$trusted_rooms
]
reject_untrusted = $reject_untrusted
EOF
    fi

    # Budget configuration
    $SUDO tee -a "$config_file" > /dev/null <<EOF

[budget]
daily_limit_usd = $budget_daily
monthly_limit_usd = $budget_monthly
alert_threshold = 80.0
hard_stop = $budget_hardstop
EOF

    $SUDO tee -a "$config_file" > /dev/null <<EOF

[logging]
level = "$log_level"
format = "$log_format"
output = "$log_output"
EOF

    if [ -n "$log_file" ]; then
        $SUDO tee -a "$config_file" > /dev/null <<EOF
file = "$log_file"
EOF
    fi

    # Voice and WebRTC configuration
    if [ -n "$VOICE_CONFIG" ]; then
        $SUDO tee -a "$config_file" > /dev/null <<EOF

[voice]
$VOICE_CONFIG

[voice.general]
default_lifetime = "30m"
max_lifetime = "2h"
auto_answer = false
require_membership = true
max_concurrent_calls = 5

[voice.security]
require_e2ee = true
min_e2ee_algorithm = "megolm.v1.aes-sha2"
rate_limit = 10
rate_limit_burst = 20
require_approval = false

[voice.budget]
enabled = true
global_token_limit = 0
global_duration_limit = "0s"
enforcement_interval = "30s"
EOF
    fi

    # WebRTC signaling configuration
    if [ -n "$WEBRTC_SIGNALING" ]; then
        $SUDO tee -a "$config_file" > /dev/null <<EOF

[webrtc.signaling]
$WEBRTC_SIGNALING
enabled = false
addr = "0.0.0.0:8443"
path = "/webrtc"
tls_cert = ""
tls_key = ""
EOF
    fi

    # Notifications configuration
    if [ "$notifications_enabled" = "true" ]; then
        $SUDO tee -a "$config_file" > /dev/null <<EOF

[notifications]
enabled = true
$notifications_config
EOF
    else
        $SUDO tee -a "$config_file" > /dev/null <<EOF

[notifications]
enabled = false
admin_room_id = ""
alert_threshold = 0.8
EOF
    fi

    # Event bus configuration
    if [ "$eventbus_enabled" = "true" ]; then
        $SUDO tee -a "$config_file" > /dev/null <<EOF

[eventbus]
$eventbus_config
EOF
    else
        $SUDO tee -a "$config_file" > /dev/null <<EOF

[eventbus]
websocket_enabled = false
websocket_addr = "0.0.0.0:8444"
websocket_path = "/events"
max_subscribers = 100
inactivity_timeout = "30m"
EOF
    fi

    # Provisioning configuration (for secure device setup)
    PROVISIONING_SECRET=$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | xxd -p -c 32)
    $SUDO tee -a "$config_file" > /dev/null <<EOF

[provisioning]
# Secret key for signing QR code configurations
# Used for secure ArmorChat/ArmorTerminal device provisioning
signing_secret = "$PROVISIONING_SECRET"
# Default token expiry in seconds (60 = 1 minute window)
default_expiry_seconds = 60
# Maximum token expiry (300 = 5 minutes max)
max_expiry_seconds = 300
# Enable one-time-use tokens (recommended for security)
one_time_use = true
EOF

    print_info "Generated provisioning secret for secure device setup"

    # Ensure error store path to config (idempotent)
    ensure_error_store_config

    # Set permissions
    $SUDO chown armorclaw:armorclaw "$config_file"
    $SUDO chmod 640 "$config_file"

    # Config sanity check
    grep -q "store_path" "$config_file" || echo "Warning: errors.store_path not configured"

    print_success "Configuration file created"
    echo ""
    cat "$config_file"
}

#=============================================================================
# Step 7: Keystore Initialization
#=============================================================================

step_keystore() {
    print_step 8 13 "Keystore Initialization"

    local keystore_db="$DATA_DIR/keystore.db"

    # If settings were imported, remind user about hardware-bound keys
    if [ "$SETTINGS_IMPORTED" = "true" ]; then
        print_warning "${BOLD}Important: Imported keystore requires fresh initialization${NC}"
        echo ""
        echo "  • API keys from your backup are encrypted with hardware-bound keys"
        echo "  • They cannot be decrypted on this machine"
        echo "  • A new keystore will be created for this system"
        echo ""
        if prompt_yes_no "Initialize new keystore for this system?"; then
            $SUDO rm -f "$keystore_db" "$keystore_db.salt" 2>/dev/null || true
        else
            print_info "Keystore will be created on first bridge start"
            return
        fi
    fi

    if [ -f "$keystore_db" ]; then
        print_info "Keystore already exists at $keystore_db"
        if prompt_yes_no "Reinitialize keystore? (WARNING: This will delete all stored credentials)"; then
            $SUDO rm -f "$keystore_db" "$keystore_db.salt"
        else
            print_success "Using existing keystore"
            return
        fi
    fi

    print_info "${BOLD}Keystore Security Features:${NC}"
    cat <<'EOF'
  • Double-layer encryption (SQLCipher + XChaCha20-Poly1305)
  • Hardware-derived master key (machine-id + DMI UUID + MAC)
  • Zero-touch reboot (salt persistence)
  • Theft protection (database unusable on different hardware)
EOF

    echo ""
    print_info "Initializing keystore..."

    # Initialize keystore by running bridge with init command
    if $SUDO -u armorclaw /opt/armorclaw/armorclaw-bridge --init; then
        print_success "Keystore initialized"
        print_info "Keystore database: $keystore_db"
        print_info "Salt file: $keystore_db.salt"
    else
        print_warning "Init command not available, keystore will be created on first start"
    fi
}

#=============================================================================
# Step 8: First API Key Setup
#=============================================================================

step_api_key() {
    print_step 9 13 "First API Key Setup"

    echo ""

    # Special messaging if settings were imported
    if [ "$SETTINGS_IMPORTED" = "true" ]; then
        print_info "${BOLD}Re-add Your API Keys${NC}"
        echo ""
        echo "Your configuration was imported, but API keys are hardware-bound"
        echo "and cannot be transferred. Please re-add your API keys now."
        echo ""
    else
        print_info "${BOLD}You can now add your first API key for AI agents.${NC}"
    fi
    echo ""
    print_info "Supported providers: openai, anthropic, openrouter, google, xai"
    echo ""

    if ! prompt_yes_no "Add an API key now?"; then
        print_info "You can add API keys later using the bridge CLI"
        return
    fi

    local provider=$(prompt_input "Provider (e.g., openai)" "")
    if [ -z "$provider" ]; then
        print_warning "No provider specified, skipping API key setup"
        return
    fi

    local key_id=$(prompt_input "Key ID (e.g., $provider-main)" "$provider-main")
    local key_token=""
    echo ""
    print_info "Enter your API key:"
    prompt_read -s -p "  " key_token
    echo

    if [ -z "$key_token" ]; then
        print_error "API key cannot be empty"
        return
    fi

    local display_name=$(prompt_input "Display name (e.g., 'Production Key')" "$provider API Key")

    print_info "Adding API key to keystore..."
    print_info "  Provider: $provider"
    print_info "  Key ID: $key_id"
    print_info "  Display name: $display_name"

    # Use bridge CLI to add key (if available) or show manual command
    echo ""
    echo -e "${CYAN}To add this key later, use:${NC}"
    echo "  echo '{\"jsonrpc\":\"2.0\",\"method\":\"add_key\",\"params\":{"
    echo "    \"id\":\"$key_id\","
    echo "    \"provider\":\"$provider\","
    echo "    \"token\":\"YOUR_TOKEN_HERE\","
    echo "    \"display_name\":\"$display_name\""
    echo "  },\"id\":1}' | sudo socat - UNIX-CONNECT:$socket_path"
}

#=============================================================================
# Step 9: Systemd Service Setup
#=============================================================================

step_systemd() {
    print_step 10 13 "Systemd Service Setup"

    local service_file="/etc/systemd/system/armorclaw-bridge.service"

    if [ -f "$service_file" ]; then
        print_info "Service file already exists"
        if prompt_yes_no "Recreate service file?"; then
            $SUDO rm -f "$service_file"
        else
            print_success "Using existing service file"
            return
        fi
    fi

    print_info "Creating systemd service..."

    $SUDO tee "$service_file" > /dev/null <<EOF
[Unit]
Description=ArmorClaw Bridge Service
After=network-online.target docker.service
Wants=network-online.target docker.service

StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=notify
NotifyAccess=main
User=armorclaw
Group=armorclaw

ExecStart=/opt/armorclaw/armorclaw-bridge -config $CONFIG_DIR/config.toml

Restart=always
RestartSec=5
# Bridge sends WATCHDOG=1 every 30s. If stuck for 60s, systemd restarts it.
WatchdogSec=60

RuntimeDirectory=armorclaw
RuntimeDirectoryMode=0755

LimitNOFILE=65536
ProtectKernelTunables=true
ProtectControlGroups=true

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$DATA_DIR

StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    print_success "Service file created"

    # Reload systemd
    print_info "Reloading systemd daemon..."
    $SUDO systemctl daemon-reload

    print_success "Systemd service configured"
    echo ""
    echo -e "${CYAN}Commands to control the service:${NC}"
    echo "  Start:   sudo systemctl start armorclaw-bridge"
    echo "  Stop:    sudo systemctl stop armorclaw-bridge"
    echo "  Status:  sudo systemctl status armorclaw-bridge"
    echo "  Logs:    sudo journalctl -u armorclaw-bridge -f"
    echo "  Enable:  sudo systemctl enable armorclaw-bridge"
}

#=============================================================================
# Final Verification
#=============================================================================

step_verify() {
    print_step 11 13 "Final Verification"

    print_info "Running verification checks..."

    local all_ok=true

    # Check directories
    print_info "Checking directories..."
    if [ -d "$CONFIG_DIR" ]; then
        print_success "Config directory: $CONFIG_DIR"
    else
        print_error "Config directory missing: $CONFIG_DIR"
        all_ok=false
    fi

    if [ -d "$DATA_DIR" ]; then
        print_success "Data directory: $DATA_DIR"
    else
        print_error "Data directory missing: $DATA_DIR"
        all_ok=false
    fi

    if [ -d "$RUN_DIR" ]; then
        print_success "Run directory: $RUN_DIR"
    else
        print_error "Run directory missing: $RUN_DIR"
        all_ok=false
    fi

    # Check binary
    print_info "Checking bridge binary..."
    if [ -x "/opt/armorclaw/armorclaw-bridge" ]; then
        print_success "Bridge binary: /opt/armorclaw/armorclaw-bridge"
    else
        print_error "Bridge binary not found"
        all_ok=false
    fi

    # Check symlink
    if [ -L "/usr/local/bin/armorclaw-bridge" ]; then
        print_success "Symlink: /usr/local/bin/armorclaw-bridge"
    else
        print_warning "Symlink missing (not critical)"
    fi

    # Check service file
    print_info "Checking systemd service..."
    if [ -f "/etc/systemd/system/armorclaw-bridge.service" ]; then
        print_success "Service file installed"
    else
        print_warning "Service file not found"
    fi

    # Check config
    print_info "Checking configuration..."
    if [ -f "$CONFIG_DIR/config.toml" ]; then
        print_success "Configuration file exists"
    else
        print_error "Configuration file missing"
        all_ok=false
    fi

    if [ "$all_ok" = true ]; then
        print_success "All verification checks passed!"
    else
        print_error "Some verification checks failed"
    fi
}

#=============================================================================
# Completion Message
#=============================================================================

print_completion() {
    echo ""
    echo -e "${GREEN}╔══════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║${NC}          ${BOLD}Setup Complete!${NC}                           ${GREEN}║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════╝${NC}"
    echo ""

    # Show import summary if applicable
    if [ "$SETTINGS_IMPORTED" = "true" ]; then
        echo -e "${BOLD}Import Summary:${NC}"
        echo "  ${GREEN}✓${NC} Settings imported from backup"
        if [ "$IMPORT_AGENT_CONFIGS" = "true" ]; then
            echo "  ${GREEN}✓${NC} Agent configurations restored"
        fi
        if [ "$IMPORT_MATRIX" = "true" ]; then
            echo "  ${GREEN}✓${NC} Matrix session restored"
        fi
        echo "  ${YELLOW}⚠${NC} API keys need to be re-added"
        echo ""
    fi

    echo -e "${BOLD}Next Steps:${NC}"
    echo ""
    echo "1. Start the bridge:"
    echo -e "   ${CYAN}sudo systemctl start armorclaw-bridge${NC}"
    echo ""
    echo "2. Enable auto-start on boot:"
    echo -e "   ${CYAN}sudo systemctl enable armorclaw-bridge${NC}"
    echo ""
    echo "3. Check status:"
    echo -e "   ${CYAN}sudo systemctl status armorclaw-bridge${NC}"
    echo ""
    echo "4. View logs:"
    echo -e "   ${CYAN}sudo journalctl -u armorclaw-bridge -f${NC}"
    echo ""
    echo "5. Test the bridge:"
    echo -e "   ${CYAN}echo '{\"jsonrpc\":\"2.0\",\"method\":\"health\",\"id\":1}' | sudo socat - UNIX-CONNECT:$SOCKET_PATH${NC}"
    echo ""

    echo -e "${BOLD}Configuration:${NC}"
    echo "  Config:  $CONFIG_DIR/config.toml"
    echo "  Data:    $DATA_DIR"
    echo "  Socket:  $SOCKET_PATH"
    echo "  Log:     $LOG_FILE"
    echo ""

    echo -e "${BOLD}Backup & Migration:${NC}"
    echo -e "  Create backup:  ${CYAN}sudo ./deploy/backup-settings.sh${NC}"
    echo -e "  Restore backup: ${CYAN}sudo ./deploy/setup-wizard.sh${NC} (choose import option)"
    echo ""

    echo -e "${BOLD}Documentation:${NC}"
    echo "  Element X Quick Start: docs/guides/element-x-quickstart.md ⭐"
    echo "  Setup Guide:           docs/guides/setup-guide.md"
    echo "  Full Docs:              docs/index.md"
    echo "  GitHub:                 https://github.com/Gemutly/ArmorClaw"
    echo ""

    if [ -f "$LOG_FILE" ]; then
        echo -e "${BOLD}Setup log saved to:${NC} $LOG_FILE"
    fi
}

#=============================================================================
# Step 13: Advanced Features (Optional)
#=============================================================================

step_advanced_features() {
    print_step 12 14 "Advanced Features (Optional)"

    cat <<'EOF'
${BOLD}Advanced Security & Features${NC}

This step configures optional advanced features for production deployments:

  • ${CYAN}WebRTC Voice Calling${NC} - Secure audio calls via Matrix
  • ${CYAN}Notifications${NC} - Matrix-based system alerts
  • ${CYAN}Event Bus${NC} - Real-time event push via WebSocket
  • ${CYAN}Host Hardening${NC} - Firewall + SSH hardening scripts
  • ${CYAN}Production Logging${NC} - JSON structured logging with file output

${YELLOW}Note:${NC} All features are optional and can be configured later
EOF

    echo ""

    # WebRTC Voice Calling
    echo -e "${BOLD}══════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}WebRTC Voice Calling${NC}"
    echo -e "${BOLD}══════════════════════════════════════════════════════${NC}"
    echo ""
    cat <<'EOF'
Enable secure voice calls through Matrix with:
  • End-to-end encrypted audio via Opus codec
  • Budget-controlled calls with token/duration limits
  • NAT traversal via TURN/STUN
  • E2EE requirement enforcement
  • Rate limiting and concurrent call limits

${CYAN}Documentation:${NC} docs/guides/webrtc-voice-guide.md
EOF
    echo ""
    if prompt_yes_no "Enable WebRTC Voice Calling?"; then
        print_info "WebRTC Voice will be enabled in configuration"

        # Voice configuration
        local voice_default_lifetime=$(prompt_input "Default call lifetime [e.g., 30m]" "30m")
        local voice_max_lifetime=$(prompt_input "Maximum call lifetime [e.g., 2h]" "2h")
        local voice_max_concurrent=$(prompt_input "Maximum concurrent calls" "5")
        local voice_require_e2ee="true"
        if ! prompt_yes_no "Require end-to-end encryption for calls?"; then
            voice_require_e2ee="false"
        fi

        VOICE_CONFIG="enabled = true
default_lifetime = \"$voice_default_lifetime\"
max_lifetime = \"$voice_max_lifetime\"
max_concurrent_calls = $voice_max_concurrent
require_e2ee = $voice_require_e2ee"

        # WebRTC signaling configuration
        echo ""
        print_info "WebRTC Signaling enables browser-based voice clients"
        if prompt_yes_no "Enable WebRTC signaling server?"; then
            local signaling_addr=$(prompt_input "Signaling server address" "0.0.0.0:8443")
            local signaling_path=$(prompt_input "Signaling WebSocket path" "/webrtc")

            WEBRTC_SIGNALING="signaling_enabled = true
signaling_addr = \"$signaling_addr\"
signaling_path = \"$signaling_path\""
        else
            WEBRTC_SIGNALING="signaling_enabled = false"
        fi

        print_info "You can customize additional voice settings in config.toml"
    else
        VOICE_ENABLED="false"
        VOICE_CONFIG="enabled = false"
        WEBRTC_SIGNALING="signaling_enabled = false"
        print_info "WebRTC Voice disabled (can be enabled later)"
    fi

    echo ""
    echo -e "${BOLD}══════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}Notification System${NC}"
    echo -e "${BOLD}══════════════════════════════════════════════════════${NC}"
    echo ""
    cat <<'EOF'
Enable Matrix-based notifications for system events:
  • ${CYAN}Budget alerts${NC} - Warning when approaching/exceeding limits
  • ${CYAN}Security events${NC} - Authentication failures, access denied
  • ${CYAN}Container events${NC} - Started, stopped, failed, restarted
  • ${CYAN}System alerts${NC} - Startup, shutdown

Notifications are sent to a Matrix admin room of your choice.
EOF
    echo ""
    local notifications_enabled="false"
    local notifications_config=""
    if [ "$matrix_enabled" = "true" ]; then
        if prompt_yes_no "Enable notification system?"; then
            notifications_enabled="true"
            local admin_room=$(prompt_input "Admin room ID for notifications" "")
            local alert_threshold=$(prompt_input "Alert threshold (percentage, e.g., 80)" "80")

            if [ -n "$admin_room" ]; then
                notifications_config="admin_room_id = \"$admin_room\"
alert_threshold = $alert_threshold"
            else
                print_warning "No admin room specified, using default notifications"
                notifications_config=""
            fi
        fi
    else
        print_info "Notifications require Matrix to be enabled"
    fi

    echo ""
    echo -e "${BOLD}══════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}Event Bus (Real-Time Event Push)${NC}"
    echo -e "${BOLD}══════════════════════════════════════════════════════${NC}"
    echo ""
    cat <<'EOF'
Enable real-time Matrix event push via WebSocket:
  • ${CYAN}Real-time delivery${NC} - No polling delay for events
  • ${CYAN}Event filtering${NC} - Subscribe by room, sender, or event type
  • ${CYAN}WebSocket support${NC} - Standard protocol for browser/clients
  • ${CYAN}Reduced bandwidth${NC} - Only relevant events delivered

${CYAN}Documentation:${NC} docs/guides/websocket-client-guide.md
EOF
    echo ""
    local eventbus_enabled="false"
    local eventbus_config=""
    if prompt_yes_no "Enable event bus for real-time event push?"; then
        eventbus_enabled="true"

        # WebSocket server configuration
        if prompt_yes_no "Enable WebSocket server for event push?"; then
            local ws_addr=$(prompt_input "WebSocket listen address" "0.0.0.0:8444")
            local ws_path=$(prompt_input "WebSocket path" "/events")
            local max_subs=$(prompt_input "Maximum concurrent subscribers" "100")
            local inactive_timeout=$(prompt_input "Inactivity timeout" "30m")

            eventbus_config="websocket_enabled = true
websocket_addr = \"$ws_addr\"
websocket_path = \"$ws_path\"
max_subscribers = $max_subs
inactivity_timeout = \"$inactive_timeout\""
        else
            eventbus_config="websocket_enabled = false"
        fi
    fi

    echo ""
    echo -e "${BOLD}══════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}Host Security Hardening${NC}"
    echo -e "${BOLD}══════════════════════════════════════════════════════${NC}"
    echo ""
    cat <<'EOF'
Configure host-level security to protect your VPS:
  • ${CYAN}Firewall (UFW)${NC} - Deny-all default with Tailscale VPN auto-detection
  • ${CYAN}SSH Hardening${NC} - Key-only authentication, root login disabled

${YELLOW}Warning:${NC} These scripts modify system firewall and SSH configuration
${CYAN}Documentation:${NC} docs/guides/security-configuration.md
EOF
    echo ""
    if prompt_yes_no "Configure host security hardening?"; then
        print_info "Checking for security scripts..."

        if [ -f "./deploy/setup-firewall.sh" ] && [ -f "./deploy/harden-ssh.sh" ]; then
            print_info "Running firewall configuration..."
            $SUDO bash ./deploy/setup-firewall.sh
            print_success "Firewall configured"

            print_info "Running SSH hardening..."
            $SUDO bash ./deploy/harden-ssh.sh
            print_success "SSH hardened"

            print_warning "You may need to reconnect SSH after hardening"
        else
            print_warning "Security scripts not found in ./deploy/"
            print_info "You can run them later:"
            echo "  sudo ./deploy/setup-firewall.sh"
            echo "  sudo ./deploy/harden-ssh.sh"
        fi
    else
        print_info "Host hardening skipped (can be configured later)"
    fi

    echo ""
    echo -e "${BOLD}══════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}Production Logging${NC}"
    echo -e "${BOLD}══════════════════════════════════════════════════════${NC}"
    echo ""
    cat <<'EOF'
Enable production-ready logging with:
  • ${CYAN}JSON format${NC} - Structured logs for log aggregation systems
  • ${CYAN}File output${NC} - Persistent logs for audit trails
  • ${CYAN}Security events${NC} - Automatic logging of auth, containers, budgets, PII

${GREEN}Benefits:${NC}
  • Machine-parseable JSON for ELK/Splunk/other log aggregators
  • Persistent audit trails for compliance
  • Detailed security event tracking
EOF
    echo ""

    # Check if user already selected JSON logging in configuration step
    if [ "$log_format" = "json" ]; then
        print_success "Production logging already configured (JSON format)"
    elif prompt_yes_no "Enable production logging now?"; then
        print_info "Production logging will be configured in config.toml"
        print_info "You can customize logging in config.toml under [logging] section"
    else
        print_info "Production logging skipped (can be enabled in config.toml)"
    fi

    echo ""
    print_success "Advanced features configuration complete"
    echo ""
    print_info "All advanced features can be configured later in:"
    echo "  $CONFIG_DIR/config.toml"
}

#=============================================================================
# Step 14: Start First Agent
#=============================================================================

step_start_agent() {
    local total_steps=13
    if [[ "$SETUP_MODE" == "expert" ]]; then
        total_steps=14
    fi
    print_step $((total_steps - 1)) $total_steps "Start First Agent (Optional)"

    echo ""
    echo "Would you like to start your first agent now?"
    echo ""
    echo "This will:"
    echo "  - Start the ArmorClaw Bridge"
    echo "  - Launch an agent container with your API key"
    echo "  - Verify everything is working"
    echo ""

    if ! prompt_yes_no "Start first agent now?"; then
        echo ""
        echo -e "${YELLOW}⊘ Skipping agent startup${NC}"
        echo "You can start an agent later with:"
        echo -e "  ${CYAN}sudo $INSTALL_DIR/armorclaw-bridge start --key <key-id>${NC}"
        return
    fi

    echo ""
    echo -e "${BLUE}Starting ArmorClaw Bridge...${NC}"

    # Start the bridge
    if ! $SUDO systemctl start armorclaw-bridge 2>/dev/null; then
        echo ""
        echo -e "${RED}✗ Failed to start bridge${NC}"
        echo "Start it manually with:"
        echo -e "  ${CYAN}sudo systemctl start armorclaw-bridge${NC}"
        echo ""
        echo "Then start an agent with:"
        echo -e "  ${CYAN}sudo $INSTALL_DIR/armorclaw-bridge start --key <key-id>${NC}"
        return
    fi

    echo -e "${GREEN}✓ Bridge started${NC}"

    # Wait for bridge to be ready
    echo ""
    echo -e "${BLUE}Waiting for bridge to be ready...${NC}"
    sleep 3

    # Get the first key ID
    local key_id
    key_id=$($SUDO "$INSTALL_DIR/armorclaw-bridge" list-keys 2>/dev/null | grep -m1 '•' | awk '{print $2}' || echo "")

    if [ -z "$key_id" ]; then
        echo ""
        echo -e "${YELLOW}⚠ No API keys found in keystore${NC}"
        echo "Add a key with:"
        echo -e "  ${CYAN}sudo $INSTALL_DIR/armorclaw-bridge add-key --provider <provider> --token <token>${NC}"
        return
    fi

    echo ""
    echo -e "${BLUE}Starting agent with key: ${key_id}${NC}"

    # Start the agent
    local output
    output=$($SUDO "$INSTALL_DIR/armorclaw-bridge" start --key "$key_id" 2>&1)
    local exit_code=$?

    if [ $exit_code -ne 0 ]; then
        echo ""
        echo -e "${RED}✗ Failed to start agent${NC}"
        echo "$output"
        echo ""
        echo "Check bridge logs with:"
        echo -e "  ${CYAN}sudo journalctl -u armorclaw-bridge -n 50${NC}"
        return
    fi

    # Parse container ID from output
    local container_id
    container_id=$(echo "$output" | grep -oP 'Container ID: \K[a-f0-9]+' || echo "")

    echo -e "${GREEN}✓ Agent started successfully${NC}"

    if [ -n "$container_id" ]; then
        echo ""
        echo -e "  Container ID: ${CYAN}$container_id${NC}"
    fi

    echo ""
    echo -e "${GREEN}╔══════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║${NC}     ${BOLD}🎉 First Agent Running Successfully!${NC}            ${GREEN}║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════╝${NC}"
    echo ""

    echo -e "${BOLD}Check agent status:${NC}"
    echo -e "  ${CYAN}sudo $INSTALL_DIR/armorclaw-bridge status${NC}"
    echo ""

    echo -e "${BOLD}View agent logs:${NC}"
    if [ -n "$container_id" ]; then
        echo -e "  ${CYAN}docker logs -f $container_id${NC}"
    else
        echo -e "  ${CYAN}docker logs -f <container-id>${NC}"
    fi
    echo ""

    echo -e "${BOLD}Stop the agent:${NC}"
    echo -e "  ${CYAN}sudo $INSTALL_DIR/armorclaw-bridge stop --name my-agent${NC}"
    echo ""

    echo -e "${BOLD}Next Steps:${NC}"
    echo "  1. Connect via Element X (see docs/guides/element-x-quickstart.md)"
    echo "  2. Send commands to your agent"
    echo "  3. Configure agent with /attach_config"
    echo ""
}

#=============================================================================
# Main Setup Flow
#=============================================================================

main() {
    # Initialize logging
    init_logging

    # Mode selection (NEW in v2.0)
    select_setup_mode

    # Route to quick setup if selected
    if [[ "$SETUP_MODE" == "quick" ]]; then
        run_quick_setup "$@"
        # If run_quick_setup returns, quick setup failed - fall through to standard
    fi

    # Print welcome for standard/expert modes
    print_header
    step_welcome

    # Run all steps
    step_prerequisites
    step_docker
    step_container
    step_bridge_install
    step_budget_confirmation
    step_configuration
    step_keystore
    step_api_key
    step_systemd
    step_verify

    # Advanced features only in expert mode
    if [[ "$SETUP_MODE" == "expert" ]]; then
        step_advanced_features
    else
        print_info "Skipping advanced features (use Expert mode or run scripts later)"
        echo "  • Matrix setup:     ./deploy/setup-matrix.sh"
        echo "  • Host hardening:   ./deploy/armorclaw-harden.sh"
        echo ""
    fi

    step_start_agent

    # Print completion message
    print_completion
}

# Run main function
main "$@"
