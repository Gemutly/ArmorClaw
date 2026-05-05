#!/bin/bash
# =============================================================================
# ArmorClaw Bridge Installation Script
# Purpose: Install and configure the ArmorClaw bridge on a Linux system
# Supported: Ubuntu 22.04/24.04, Debian 12
# =============================================================================

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Version and configuration
BRIDGE_VERSION="${ARMORCLAW_VERSION:-v1.0.0}"
INSTALL_DIR="/opt/armorclaw"
BIN_DIR="/usr/local/bin"
CONFIG_DIR="/etc/armorclaw"
DATA_DIR="/var/lib/armorclaw"
RUN_DIR="/run/armorclaw"
USER="armorclaw"
GROUP="armorclaw"
SIGNING_KEY_FPR="A1482657223EAFE1C481B74A8F535F90685749E0"
REPO="Gemutly/ArmorClaw"
VERSION="${BRIDGE_VERSION}"
USE_BINARY="${USE_BINARY:-true}"

# =============================================================================
# Helper Functions
# =============================================================================

# Helper for interactive prompts (handles curl | bash and non-interactive envs)
prompt_read() {
    if [ -t 0 ] || [ -c /dev/tty ]; then
        read "$@" < /dev/tty
    fi
}

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
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
            log_warning "Unsupported architecture for binary distribution: $arch"
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
    
    # Use latest release if VERSION is main or v1.0.0
    if [[ "$VERSION" == "main" || "$VERSION" == "v1.0.0" ]]; then
        base_url="https://github.com/$REPO/releases/latest/download"
    fi

    log_info "Downloading prebuilt binary: $bin_name"
    
    local tmp_dir=$(mktemp -d)
    local bin_path="$tmp_dir/$bin_name"
    local checksum_path="$tmp_dir/checksums.txt"
    local sig_path="$tmp_dir/checksums.txt.sig"
    local key_path="$tmp_dir/armorclaw-signing-key.asc"
    
    # Use strict curl flags
    local curl_base="curl --proto =https --tlsv1.2 --fail --silent --show-error --location"

    if ! retry $curl_base "$base_url/$bin_name" -o "$bin_path"; then
        log_warning "Failed to download binary"
        rm -rf "$tmp_dir"
        return 1
    fi
    
    if ! retry $curl_base "$base_url/checksums.txt" -o "$checksum_path"; then
        log_warning "Failed to download checksums"
        rm -rf "$tmp_dir"
        return 1
    fi
    
    if ! retry $curl_base "$base_url/checksums.txt.sig" -o "$sig_path"; then
        log_warning "Failed to download signature"
        rm -rf "$tmp_dir"
        return 1
    fi
    
    retry $curl_base "https://raw.githubusercontent.com/$REPO/main/deploy/armorclaw-signing-key.asc" -o "$key_path"

    # Verify Checksum
    log_info "Verifying binary checksum..."
    if ! grep "$bin_name" "$checksum_path" | (cd "$tmp_dir" && sha256sum -c - >/dev/null 2>&1); then
        log_error "Binary checksum verification failed!"
        rm -rf "$tmp_dir"
        return 1
    fi
    log_success "Checksum OK"

    # Verify Signature
    log_info "Verifying release signature..."
    local gnupg_home="$tmp_dir/gnupg"
    mkdir -p "$gnupg_home"
    chmod 700 "$gnupg_home"
    
    gpg --homedir "$gnupg_home" --batch --import "$key_path" >/dev/null 2>&1
    
    # Verify fingerprint
    local fpr_check=$(gpg --homedir "$gnupg_home" --with-colons --fingerprint releases@armorclaw.ai | grep "^fpr" | cut -d: -f10)
    if [[ "$fpr_check" != "$SIGNING_KEY_FPR" ]]; then
        log_error "Unauthorized signing key detected!"
        rm -rf "$tmp_dir"
        return 1
    fi

    if ! gpg --homedir "$gnupg_home" --batch --verify "$sig_path" "$checksum_path" >/dev/null 2>&1; then
        log_error "GPG signature verification failed for checksums.txt!"
        rm -rf "$tmp_dir"
        return 1
    fi
    log_success "Signature Verified"

    # Move to final location
    $SUDO mkdir -p "$INSTALL_DIR"
    $SUDO install -m 755 "$bin_path" "$INSTALL_DIR/armorclaw-bridge"
    $SUDO ln -sf "$INSTALL_DIR/armorclaw-bridge" /usr/local/bin/armorclaw-bridge
    
    rm -rf "$tmp_dir"
    return 0
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

ensure_error_store_config() {
    local config_file="${CONFIG_DIR}/config.toml"

    $SUDO mkdir -p "$CONFIG_DIR"
    touch "$config_file"

    if ! grep -q "^\[errors\]" "$config_file" 2>/dev/null; then
        cat >> "$config_file" <<EOF

[errors]
store_path = "$DATA_DIR/errors.db"
EOF
    fi
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root"
        exit 1
    fi
}

check_os() {
    if [[ ! -f /etc/os-release ]]; then
        log_error "Cannot detect OS"
        exit 1
    fi

    local os_id=$(grep "^ID=" /etc/os-release | cut -d= -f2 | tr -d '"')
    local os_pretty_name=$(grep "^PRETTY_NAME=" /etc/os-release | cut -d= -f2 | tr -d '"')

    if [[ "$os_id" != "ubuntu" ]] && [[ "$os_id" != "debian" ]]; then
        log_warning "This script is designed for Ubuntu/Debian. Detected: $os_id"
        echo -n "Continue anyway? (y/N) "
        prompt_read -n 1 -r REPLY
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi

    log_success "OS detected: $os_pretty_name"
}

verify_checksum() {
    local file="$1"
    local expected_checksum="$2"

    if [[ ! -f "$file" ]]; then
        log_error "File not found: $file"
        return 1
    fi

    # Calculate SHA256 checksum
    local actual_checksum=$(sha256sum "$file" | awk '{print $1}')

    if [[ "$actual_checksum" == "$expected_checksum" ]]; then
        log_success "Checksum verified for $(basename "$file")"
        return 0
    else
        log_error "Checksum mismatch for $(basename "$file")"
        log_error "Expected: $expected_checksum"
        log_error "Actual:   $actual_checksum"
        return 1
    fi
}

# =============================================================================
# Installation Steps
# =============================================================================

create_user() {
    log_info "Creating ArmorClaw user..."

    if id "$USER" &>/dev/null; then
        log_warning "User $USER already exists"
    else
        $SUDO useradd --system --user-group --shell /bin/false --home "$INSTALL_DIR" "$USER"
        log_success "Created user: $USER"
    fi
}

create_directories() {
    log_info "Creating directories..."

    $SUDO mkdir -p "$INSTALL_DIR"
    $SUDO mkdir -p "$CONFIG_DIR"
    $SUDO mkdir -p "$RUN_DIR"
    $SUDO mkdir -p "$INSTALL_DIR/containers"

    $SUDO chown -R "$USER:$GROUP" "$INSTALL_DIR"
    $SUDO chown -R "$USER:$GROUP" "$RUN_DIR"
    $SUDO chmod 755 "$CONFIG_DIR"
    $SUDO chmod 770 "$RUN_DIR"

    log_success "Directories created"
}

install_binary() {
    log_info "Installing bridge binary..."

    # Try binary install first if enabled
    if [[ "${USE_BINARY:-true}" == "true" ]]; then
        detect_arch
        if [[ -n "$BIN_ARCH" ]]; then
            if download_binary; then
                log_success "Bridge installed via binary distribution"
                return 0
            else
                log_warning "Binary installation failed, falling back to local source"
            fi
        fi
    fi

    local binary_source=""

    # Check if binary is provided locally
    if [[ -f "./armorclaw-bridge" ]]; then
        binary_source="./armorclaw-bridge"
        log_info "Using local binary"
    elif [[ -f "./bin/bridge" ]]; then
        binary_source="./bin/bridge"
        log_info "Using local binary from bin/"
    else
        log_error "Bridge binary not found"
        log_error "Please provide armorclaw-bridge in the current directory"
        exit 1
    fi

    # Verify binary exists and is executable
    if [[ ! -x "$binary_source" ]]; then
        chmod +x "$binary_source"
    fi

    # Copy to installation directory
    $SUDO cp "$binary_source" "$INSTALL_DIR/armorclaw-bridge"
    $SUDO chmod 755 "$INSTALL_DIR/armorclaw-bridge"
    $SUDO chown "$USER:$GROUP" "$INSTALL_DIR/armorclaw-bridge"

    # Create symlink in BIN_DIR
    $SUDO ln -sf "$INSTALL_DIR/armorclaw-bridge" "$BIN_DIR/armorclaw-bridge"

    # Verify installation
    if "$INSTALL_DIR/armorclaw-bridge" --version &>/dev/null; then
        log_success "Bridge binary installed"
    else
        log_warning "Binary installed but version check failed"
    fi
}

install_container_image() {
    log_info "Checking for container image..."

    if [[ -f "./armorclaw-agent-v1.tar.gz" ]]; then
        log_info "Loading container image..."

        if command -v docker &>/dev/null; then
            docker load -i "./armorclaw-agent-v1.tar.gz" 2>/dev/null || {
                log_warning "Failed to load container image (Docker may not be running)"
            }
        else
            log_warning "Docker not found, skipping container image load"
        fi
    else
        log_info "No container image found (this is OK if using remote registry)"
    fi
}

install_config() {
    log_info "Installing configuration..."

    # Ensure armorclaw user exists
    id -u "$USER" >/dev/null 2>&1 || $SUDO useradd --system --user-group --shell /bin/false --home "$INSTALL_DIR" "$USER"

    # Create data directory
    $SUDO mkdir -p "$DATA_DIR"
    $SUDO chown "$USER:$GROUP" "$DATA_DIR"
    $SUDO chmod 700 "$DATA_DIR"

    if [[ -f "./config.example.toml" ]]; then
        $SUDO cp "./config.example.toml" "$CONFIG_DIR/config.toml"
    elif [[ -f "./bridge/config.example.toml" ]]; then
        $SUDO cp "./bridge/config.example.toml" "$CONFIG_DIR/config.toml"
    else
        # Create minimal default config
        $SUDO tee "$CONFIG_DIR/config.toml" > /dev/null <<EOF
[server]
socket_path = "/run/armorclaw/bridge.sock"
pid_file = "/run/armorclaw/bridge.pid"
daemonize = false

[keystore]
db_path = "/var/lib/armorclaw/keystore.db"
master_key = ""

[matrix]
enabled = false
homeserver_url = "https://matrix.example.com"
username = "bridge-bot"
password = "change-me"
device_id = "armorclaw-bridge"

[logging]
level = "info"
format = "text"
output = "stdout"
EOF
    fi

    # Ensure error store path to config (idempotent)
    ensure_error_store_config

    $SUDO chown "$USER:$GROUP" "$CONFIG_DIR/config.toml"
    $SUDO chmod 640 "$CONFIG_DIR/config.toml"

    # Config sanity check
    grep -q "store_path" "$CONFIG_DIR/config.toml" || echo "Warning: errors.store_path not configured"

    log_success "Configuration installed at $CONFIG_DIR/config.toml"
}

install_systemd_service() {
    log_info "Installing systemd service..."

    $SUDO tee "/etc/systemd/system/armorclaw-bridge.service" > /dev/null <<EOF
[Unit]
Description=ArmorClaw Bridge - Secure AI Agent Container Manager
Documentation=https://github.com/Gemutly/ArmorClaw
After=network-online.target docker.socket
Wants=network-online.target docker.socket

StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=notify
NotifyAccess=main
User=$USER
Group=$GROUP

# Configuration
Environment="ARMORCLAW_CONFIG=$CONFIG_DIR/config.toml"

# Execution
ExecStart=$INSTALL_DIR/armorclaw-bridge --config $CONFIG_DIR/config.toml
ExecReload=/bin/kill -HUP \$MAINPID

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectControlGroups=true

RuntimeDirectory=armorclaw
RuntimeDirectoryMode=0755

# Resource limits
MemoryMax=1G
LimitNOFILE=65536

# Allowed paths
ReadWritePaths=$DATA_DIR

# Process management
Restart=always
RestartSec=5s
# Watchdog: bridge sends WATCHDOG=1 every 30s via sd_notify
# If stuck for 60s, systemd restarts it automatically
WatchdogSec=60
TimeoutStopSec=30s

StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    $SUDO systemctl daemon-reload
    $SUDO systemctl enable armorclaw-bridge.service

    log_success "Systemd service installed and enabled"
}

create_uninstall_script() {
    log_info "Creating uninstall script..."

    cat > "$INSTALL_DIR/uninstall.sh" <<'EOF'
#!/bin/bash
# ArmorClaw Bridge Uninstall Script

set -euo pipefail

RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}Error: This script must be run as root${NC}"
    exit 1
fi

echo -e "${YELLOW}This will remove ArmorClaw Bridge from your system.${NC}"
echo ""
read -p "Continue? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Uninstall cancelled"
    exit 0
fi

# Stop and disable service
systemctl stop armorclaw-bridge.service || true
systemctl disable armorclaw-bridge.service || true

# Remove service file
rm -f /etc/systemd/system/armorclaw-bridge.service
$SUDO systemctl daemon-reload

# Remove symlink
rm -f /usr/local/bin/armorclaw-bridge

# Preserve or remove config
read -p "Remove configuration files in /etc/armorclaw? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    rm -rf /etc/armorclaw
fi

# Preserve or remove keystore
read -p "Remove keystore database? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    rm -f /etc/armorclaw/keystore.db
    rm -f /etc/armorclaw/keystore.db.salt
fi

# Remove installation directory
rm -rf /opt/armorclaw

# Remove user (only if no processes remain)
if ! pgrep -u armorclaw &>/dev/null; then
    userdel armorclaw || true
fi

echo "ArmorClaw Bridge removed successfully"
EOF

    $SUDO chmod +x "$INSTALL_DIR/uninstall.sh"
    $SUDO chown "$USER:$GROUP" "$INSTALL_DIR/uninstall.sh"

    log_success "Uninstall script created at $INSTALL_DIR/uninstall.sh"
}

print_post_install() {
    cat <<EOF

${GREEN}╔════════════════════════════════════════════════════════════════╗
║           ArmorClaw Bridge Installation Complete                    ║
╚══════════════════════════════════════════════════════════════════════╝${NC}

${BLUE}Installation Details:${NC}
  • Binary:       $INSTALL_DIR/armorclaw-bridge
  • Config:       $CONFIG_DIR/config.toml
  • Socket:       $RUN_DIR/bridge.sock
  • User:         $USER

${BLUE}Service Commands:${NC}
  • Start:        sudo systemctl start armorclaw-bridge
  • Stop:         sudo systemctl stop armorclaw-bridge
  • Restart:      sudo systemctl restart armorclaw-bridge
  • Status:       sudo systemctl status armorclaw-bridge
  • Logs:         sudo journalctl -u armorclaw-bridge -f

${BLUE}Next Steps:${NC}
  1. Edit configuration: sudo nano $CONFIG_DIR/config.toml
  2. Start the service: sudo systemctl start armorclaw-bridge
  3. Check status: sudo systemctl status armorclaw-bridge
  4. View logs: sudo journalctl -u armorclaw-bridge -n 50

${YELLOW}Configuration Required:${NC}
  • Matrix settings: Enable and configure in config.toml if needed
  • Docker: Ensure Docker is installed and running
  • Container: Pull or build armorclaw/agent:v1 container image

${YELLOW}Important:${NC}
  • The keystore database will be created on first start
  • Database is encrypted with hardware-derived master key
  • Keep backups of the database and salt file

${GREEN}To uninstall: $INSTALL_DIR/uninstall.sh${NC}

EOF
}

# =============================================================================
# Main Execution
# =============================================================================

main() {
    echo -e "${BLUE}"
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║           ArmorClaw Bridge Installation Script                 ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
    echo "Version: $BRIDGE_VERSION"
    echo ""

    # Prerequisites check
    check_root
    check_os

    # Confirm installation
    log_warning "This will install ArmorClaw Bridge on your system"
    echo ""
    echo -n "Continue? (y/N) "; prompt_read -n 1 -r REPLY
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Installation cancelled"
        exit 0
    fi

    echo ""
    log_info "Starting installation..."
    echo ""

    # Execute installation steps
    create_user
    create_directories
    install_binary
    install_container_image
    install_config
    install_systemd_service
    create_uninstall_script

    echo ""
    print_post_install
}

# Run main function
main "$@"
