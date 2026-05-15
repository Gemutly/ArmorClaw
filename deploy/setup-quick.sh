#!/bin/bash
# ArmorClaw Quick Setup - Express Installation
# Version: 1.0
# Idempotent: Yes
# Safe to re-run: Yes
#
# Usage: sudo ./deploy/setup-quick.sh [--non-interactive]

# Handle --help and --version flags early (before any checks)
case "${1:-}" in
    --help|-h)
        echo "ArmorClaw Quick Setup - Express Installation"
        echo ""
        echo "Usage: setup-quick.sh [OPTIONS]"
        echo ""
        echo "Options:"
        echo "  --non-interactive, -y  Run without prompts (use defaults)"
        echo "  --help, -h             Show this help message"
        echo "  --version, -v          Show version"
        echo ""
        echo "Environment Variables:"
        echo "  OPENROUTER_API_KEY     OpenRouter API key (recommended)"
        echo "  OPEN_AI_KEY            OpenAI API key"
        echo "  ZAI_API_KEY            xAI (Grok) API key"
        echo "  ARMORCLAW_SERVER_NAME  Server hostname or IP"
        echo ""
        exit 0
        ;;
    --version|-v)
        echo "ArmorClaw Quick Setup v0.4.1"
        exit 0
        ;;
esac

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# Source Cloudflare functions library
CLOUDFLARE_FUNCTIONS_SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/cloudflare-functions.sh"
if [[ -f "$CLOUDFLARE_FUNCTIONS_SCRIPT" ]]; then
    source "$CLOUDFLARE_FUNCTIONS_SCRIPT"
else
    echo -e "${RED}WARNING: Cloudflare functions library not found at $CLOUDFLARE_FUNCTIONS_SCRIPT${NC}"
    echo -e "${YELLOW}Cloudflare HTTPS features will be unavailable${NC}"
fi

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

# Non-interactive mode
NON_INTERACTIVE=false
if [[ "${1:-}" == "--non-interactive" || "${1:-}" == "-y" ]]; then
    NON_INTERACTIVE=true
fi

# CI mode detection (for smoke tests)
CI_MODE=false
if [[ "${GITHUB_ACTIONS:-false}" == "true" ]] || [[ "${CI:-false}" == "true" ]] || [[ "${ARMORCLAW_SKIP_DOCKER_CHECK:-false}" == "true" ]]; then
    CI_MODE=true
    NON_INTERACTIVE=true
fi

# Prefer binary distribution (disabled until release is created)
USE_BINARY="${USE_BINARY:-false}"

# Smart defaults for quick setup
DEFAULT_LOG_LEVEL="info"
DEFAULT_LOG_FORMAT="text"
DEFAULT_DAILY_BUDGET="5.00"
DEFAULT_MONTHLY_BUDGET="100.00"
DEFAULT_HARD_STOP="true"

# Track Matrix installation state
MATRIX_ENABLED="false"
MATRIX_ADMIN_USER="${ARMORCLAW_ADMIN_USER:-admin}"
MATRIX_ADMIN_PASSWORD=""
MATRIX_BRIDGE_USER="${ARMORCLAW_BRIDGE_USER:-bridge}"
MATRIX_BRIDGE_PASSWORD=""
MATRIX_DOMAIN=""
TUNNEL_URL=""
LOCKFILE="/tmp/armorclaw-quick.lock"

# Cleanup handler
cleanup() {
    # Release lock
    flock -u 200 2>/dev/null || true
    
    [ -n "${WORK_DIR:-}" ] && [ -d "$WORK_DIR" ] && rm -rf "$WORK_DIR"
}
trap cleanup EXIT

# Acquire lock
exec 200>"$LOCKFILE"
if ! flock -n 200; then
    echo -e "${RED}ERROR:${NC} Another instance of setup-quick.sh is already running."
    exit 1
fi

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
        ((n++)) || true || true
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
    
    # Use strict curl flags in an array for safety with retry
    local curl_opts=(--proto "=https" --tlsv1.2 --fail --silent --show-error --location)

    if ! retry curl "${curl_opts[@]}" "$base_url/$bin_name" -o "$bin_path"; then
        print_warning "Failed to download binary from $base_url/$bin_name"
        rm -rf "$tmp_dir"
        return 1
    fi
    
    if ! retry curl "${curl_opts[@]}" "$base_url/checksums.txt" -o "$checksum_path"; then
        print_warning "Failed to download checksums from $base_url/checksums.txt"
        rm -rf "$tmp_dir"
        return 1
    fi
    
    if ! retry curl "${curl_opts[@]}" "$base_url/checksums.txt.sig" -o "$sig_path"; then
        print_warning "Failed to download signature from $base_url/checksums.txt.sig"
        rm -rf "$tmp_dir"
        return 1
    fi
    
    retry curl "${curl_opts[@]}" "https://raw.githubusercontent.com/$REPO/main/deploy/armorclaw-signing-key.asc" -o "$key_path" || true

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
    local fpr_check=$(gpg --homedir "$gnupg_home" --with-colons --fingerprint releases@armorclaw.ai | grep "^fpr" | head -n1 | cut -d: -f10)
    if [[ "${fpr_check:-}" != "$SIGNING_KEY_FPR" ]]; then
        print_error "Unauthorized signing key detected!"
        echo "Expected: $SIGNING_KEY_FPR"
        echo "Actual:   ${fpr_check:-MISSING}"
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
    clear 2>/dev/null || true
    local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local version_file="$script_dir/../VERSION"
    local ver="0.4.1"
    if [[ -f "$version_file" ]]; then
        ver=$(cat "$version_file" | tr -d '[:space:]')
    fi
    echo -e "${CYAN}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC}            ${BOLD}ArmorClaw Quick Setup${NC}  ${YELLOW}v${ver}${NC}                      ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}            ${BOLD}Express Installation (2-3 min)${NC}                   ${CYAN}║${NC}"
    echo -e "${CYAN}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

print_step() {
    echo -e "\n${BLUE}▶${NC} ${BOLD}$1${NC}"
    echo -e "${BLUE}  ─────────────────────────────────────${NC}"
}

print_success() {
    echo -e "  ${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "  ${RED}✗${NC} ${BOLD}ERROR:${NC} $1" >&2
}

print_warning() {
    echo -e "  ${YELLOW}⚠${NC} $1"
}

print_info() {
    echo -e "  ${CYAN}ℹ${NC} $1"
}

print_done() {
    echo -e "  ${GREEN}✓${NC} $1"
}

show_spinner() {
    local pid=$1
    local message="$2"
    local spin='-\|/'
    local i=0
    while kill -0 "$pid" 2>/dev/null; do
        i=$(( (i+1) % 4 ))
        printf "\r  ${YELLOW}⏳${NC} $message... ${spin:$i:1}"
        sleep .2
    done
    printf "\r"
}

fail() {
    print_error "$1"
    exit 1
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

prompt_yes_no() {
    if $NON_INTERACTIVE; then
        return 0  # Default to yes in non-interactive mode
    fi

    local prompt="$1"
    local default="${2:-n}"

    echo ""
    echo -ne "  ${CYAN}$prompt [${default^^}/$(echo $default | tr 'yn' 'ny')]${NC}: "
    prompt_read -r response
    response=${response:-$default}

    case "$response" in
        [Yy]|[Yy][Ee][Ss]) return 0 ;;
        [Nn]|[Nn][Oo]) return 1 ;;
    esac

    # Default behavior
    [[ "$default" == "y" ]]
}

#=============================================================================
# Matrix Helpers
#=============================================================================

is_matrix_running() {
    # Check if conduit service is active
    if systemctl is-active --quiet conduit 2>/dev/null; then
        return 0
    fi

    # Check if conduit container is running
    if command -v docker >/dev/null 2>&1; then
        if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "conduit"; then
            return 0
        fi
    fi

    return 1
}

create_matrix_admin_user() {
    local server_name="${1:-localhost}"
    local admin_user="${2:-admin}"
    local admin_pass="${3:-}"

    if [[ -z "$admin_pass" ]]; then
        print_error "No password provided for admin user"
        return 1
    fi

    print_info "Creating Matrix admin user: $admin_user..."

    local max_attempts=30
    local attempt=0
    while [[ $attempt -lt $max_attempts ]]; do
        if curl -sf "http://localhost:6167/_matrix/client/versions" >/dev/null 2>&1; then
            break
        fi
        sleep 1
        ((attempt++)) || true
    done

    if [[ $attempt -eq $max_attempts ]]; then
        print_error "Matrix API not responding"
        return 1
    fi

    local register_response
    register_response=$(curl -sf "http://localhost:6167/_matrix/client/v3/register" \
        -H "Content-Type: application/json" \
        -d '{
            "username": "'"${admin_user}"'",
            "password": "'"${admin_pass}"'",
            "auth": {"type": "m.login.dummy"}
        }' 2>/dev/null) || true

    if echo "$register_response" | grep -q '"user_id"'; then
        print_done "Matrix admin user created: $admin_user"
        return 0
    elif echo "$register_response" | grep -q '"errcode".*"M_USER_IN_USE"'; then
        print_info "Matrix admin user already exists: $admin_user"
        return 0
    else
        print_warning "Could not create Matrix admin user (registration may be disabled)"
        print_info "Response: ${register_response:-no response}"
        return 0
    fi
}

create_matrix_bridge_user() {
    local server_name="${1:-localhost}"
    local bridge_user="${2:-bridge}"
    local bridge_pass="${3:-}"

    if [[ -z "$bridge_pass" ]]; then
        print_error "No password provided for bridge user"
        return 1
    fi

    print_info "Creating Matrix bridge user: $bridge_user..."

    local register_response
    register_response=$(curl -sf "http://localhost:6167/_matrix/client/v3/register" \
        -H "Content-Type: application/json" \
        -d '{
            "username": "'"${bridge_user}"'",
            "password": "'"${bridge_pass}"'",
            "auth": {"type": "m.login.dummy"}
        }' 2>/dev/null) || true

    if echo "$register_response" | grep -q '"user_id"'; then
        print_done "Matrix bridge user created: $bridge_user"
        return 0
    elif echo "$register_response" | grep -q '"errcode".*"M_USER_IN_USE"'; then
        print_info "Matrix bridge user already exists: $bridge_user"
        return 0
    else
        print_warning "Could not create Matrix bridge user (registration may be disabled)"
        print_info "Response: ${register_response:-no response}"
        return 1
    fi
}

ensure_matrix() {
    if is_matrix_running; then
        # Matrix already running - check if we have credentials
        if [[ -z "$MATRIX_ADMIN_PASSWORD" ]]; then
            print_info "Matrix already running - generating admin credentials..."
            MATRIX_ADMIN_PASSWORD=$(openssl rand -base64 16 | tr -d '/+=' 2>/dev/null || head -c 16 /dev/urandom | xxd -p)
            MATRIX_BRIDGE_PASSWORD=$(openssl rand -base64 16 | tr -d '/+=' 2>/dev/null || head -c 16 /dev/urandom | xxd -p)
            MATRIX_DOMAIN=$(hostname -I | awk '{print $1}')
            create_matrix_admin_user "$MATRIX_DOMAIN" "$MATRIX_ADMIN_USER" "$MATRIX_ADMIN_PASSWORD" || true
            create_matrix_bridge_user "$MATRIX_DOMAIN" "$MATRIX_BRIDGE_USER" "$MATRIX_BRIDGE_PASSWORD" || true
            local config_file="$CONFIG_DIR/config.toml"
            if [[ -f "$config_file" ]]; then
                $SUDO sed -i 's/^enabled = false/enabled = true/' "$config_file" 2>/dev/null || true
                $SUDO sed -i 's|homeserver_url = ""|homeserver_url = "http://localhost:6167"|' "$config_file" 2>/dev/null || true
                $SUDO sed -i "s/^username = \"\"/username = \"$MATRIX_BRIDGE_USER\"/" "$config_file" 2>/dev/null || true
                $SUDO sed -i "s/^password = \"\"/password = \"$MATRIX_BRIDGE_PASSWORD\"/" "$config_file" 2>/dev/null || true
                print_done "Bridge Matrix credentials configured"
            fi
        fi
        return 0
    fi

    print_info "Matrix server not detected — installing Conduit..."

    # Check Docker
    if ! command -v docker >/dev/null 2>&1; then
        print_error "Docker is required for Matrix"
        return 1
    fi

    if ! docker info >/dev/null 2>&1; then
        print_error "Docker daemon is not running"
        return 1
    fi

    MATRIX_DOMAIN=$(hostname -I | awk '{print $1}')
    print_info "Server IP: $MATRIX_DOMAIN"

    MATRIX_ADMIN_PASSWORD=$(openssl rand -base64 16 | tr -d '/+=' 2>/dev/null || head -c 16 /dev/urandom | xxd -p)
    print_info "Generated admin password"

    MATRIX_BRIDGE_PASSWORD=$(openssl rand -base64 16 | tr -d '/+=' 2>/dev/null || head -c 16 /dev/urandom | xxd -p)
    print_info "Generated bridge password"

    local CONDUIT_DATA_DIR="/var/lib/conduit"
    local CONDUIT_CONFIG_FILE="/etc/conduit.toml"
    $SUDO mkdir -p "$CONDUIT_DATA_DIR"
    $SUDO chown 1000:1000 "$CONDUIT_DATA_DIR" 2>/dev/null || true

    $SUDO tee "$CONDUIT_CONFIG_FILE" >/dev/null <<EOF
[global]
server_name = "${MATRIX_DOMAIN}"
database_backend = "rocksdb"
database_path = "/data"
address = "0.0.0.0"
port = 6167
max_request_size = 20000000
allow_registration = true
allow_federation = true
allow_check_for_updates = false
trusted_servers = ["matrix.org"]
EOF
    print_info "Created Conduit config: $CONDUIT_CONFIG_FILE"

    if docker ps -a --format '{{.Names}}' | grep -q "^armorclaw-conduit$"; then
        docker rm -f armorclaw-conduit 2>/dev/null || true
    fi

    docker run -d \
        --name armorclaw-conduit \
        --restart unless-stopped \
        -p 6167:6167 \
        -p 8448:8448 \
        -v "$CONDUIT_DATA_DIR:/data" \
        -v "$CONDUIT_CONFIG_FILE:/etc/conduit.toml:ro" \
        -e CONDUIT_CONFIG="/etc/conduit.toml" \
        matrixconduit/matrix-conduit:latest 2>/dev/null

    print_info "Waiting for Matrix server..."
    local wait_count=0
    while [[ $wait_count -lt 30 ]]; do
        if curl -sf "http://localhost:6167/_matrix/client/versions" >/dev/null 2>&1; then
            break
        fi
        sleep 1
        ((wait_count++)) || true
    done

    if ! is_matrix_running; then
        print_error "Matrix server failed to start"
        print_info "Check logs: docker logs armorclaw-conduit"
        return 1
    fi
    print_done "Matrix server started"

    create_matrix_admin_user "$MATRIX_DOMAIN" "$MATRIX_ADMIN_USER" "$MATRIX_ADMIN_PASSWORD" || true

    $SUDO sed -i 's/allow_registration = true/allow_registration = false/' "$CONDUIT_CONFIG_FILE"
    docker restart armorclaw-conduit 2>/dev/null || true
    print_info "Registration disabled for security"

    local config_file="$CONFIG_DIR/config.toml"
    if [[ -f "$config_file" ]]; then
        if grep -q '^enabled = false' "$config_file" 2>/dev/null; then
            $SUDO sed -i 's/enabled = false/enabled = true/' "$config_file"
        fi
        if grep -q 'homeserver_url = ""' "$config_file"; then
            $SUDO sed -i 's|homeserver_url = ""|homeserver_url = "http://localhost:6167"|' "$config_file"
        fi
        if grep -q '^username = ""' "$config_file" 2>/dev/null; then
            $SUDO sed -i "s/^username = \"\"/username = \"$MATRIX_BRIDGE_USER\"/" "$config_file"
        fi
        if grep -q '^password = ""' "$config_file" 2>/dev/null; then
            $SUDO sed -i "s/^password = \"\"/password = \"$MATRIX_BRIDGE_PASSWORD\"/" "$config_file"
        fi
        print_done "Bridge Matrix credentials configured"
    fi

    print_done "Matrix installed and running"
    return 0
}

start_cloudflare_tunnel() {
    print_step "HTTPS Setup (Tunnel)"

    echo ""
    echo "  Element X requires HTTPS to connect."
    echo "  Tunnels provide free, instant HTTPS URLs."
    echo ""
    echo "  Options:"
    echo "  1) Cloudflare Quick Tunnel (Free, instant)"
    echo "  2) ngrok Tunnel (Free account needed at ngrok.com)"
    echo "  3) Skip (configure manually later)"
    echo ""

    if $NON_INTERACTIVE; then
        print_info "Non-interactive mode - skipping tunnel setup"
        print_info "Run manually: docker run -d --name armorclaw-tunnel cloudflare/cloudflared:latest tunnel --url http://host.docker.internal:6167"
        return 0
    fi

    echo -ne "  Select [1/2/3]: "
    prompt_read -r tunnel_choice

    if [[ "$tunnel_choice" == "3" ]] || [[ -z "$tunnel_choice" ]]; then
        print_info "Skipped. Run later to enable remote Element X access."
        return 0
    fi

    if [[ "$tunnel_choice" == "2" ]]; then
        start_ngrok_tunnel
        return $?
    fi

    if [[ "$tunnel_choice" != "1" ]]; then
        print_error "Invalid selection"
        return 1
    fi

    if docker ps -a --format '{{.Names}}' | grep -q "^armorclaw-tunnel$"; then
        print_info "Removing existing tunnel container..."
        docker rm -f armorclaw-tunnel 2>/dev/null || true
    fi

    local max_retries=2
    local retry=1

    while [[ $retry -le $max_retries ]]; do
        print_info "Starting Cloudflare Quick Tunnel (attempt $retry/$max_retries)..."

        local container_id
        container_id=$(docker run -d \
            --name armorclaw-tunnel \
            --restart unless-stopped \
            --add-host=host.docker.internal:host-gateway \
            cloudflare/cloudflared:latest \
            tunnel --url http://host.docker.internal:6167 2>&1)

        if [[ $? -ne 0 ]]; then
            print_error "Failed to start tunnel container"
            print_error "$container_id"
            print_info "Check Docker is running: docker info"
            return 1
        fi

        print_info "Waiting for tunnel to establish (up to 30s)..."

        local max_attempts=15
        local attempt=1
        local tunnel_logs=""

        while [[ $attempt -le $max_attempts ]]; do
            sleep 2
            tunnel_logs=$(docker logs armorclaw-tunnel 2>&1)

            if echo "$tunnel_logs" | grep -qE "https://[a-z0-9-]+\.trycloudflare\.com"; then
                TUNNEL_URL=$(echo "$tunnel_logs" | grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' | tail -1)
                break
            fi

            if echo "$tunnel_logs" | grep -qiE "500 Internal Server Error|failed to unmarshal"; then
                print_warning "Cloudflare service error (500)"
                docker rm -f armorclaw-tunnel 2>/dev/null || true
                break
            fi

            if echo "$tunnel_logs" | grep -qiE "error|failed|unable"; then
                print_warning "Tunnel error detected"
                break
            fi

            echo -n "."
            ((attempt++)) || true
        done
        echo ""

        if [[ -n "$TUNNEL_URL" ]]; then
            break
        fi

        if [[ $retry -lt $max_retries ]]; then
            print_info "Retrying in 5 seconds..."
            sleep 5
        fi

        ((retry++)) || true
    done

    if [[ -z "$TUNNEL_URL" ]]; then
        print_error "Cloudflare Tunnel failed after $max_retries attempts"
        echo ""
        echo -e "${YELLOW}Cloudflare's free tunnel service may be rate-limiting or experiencing issues.${NC}"
        echo ""
        echo -e "${BOLD}Alternatives:${NC}"
        echo "  1) Try ngrok (option 2) - requires free account at ngrok.com"
        echo "  2) Wait 10-15 minutes and re-run setup"
        echo "  3) Use local network only (HTTP) - Element Web works, mobile needs HTTPS"
        echo ""
        print_info "To retry later: docker rm -f armorclaw-tunnel 2>/dev/null; then re-run this script"
        return 1
    fi

    TUNNEL_DOMAIN=$(echo "$TUNNEL_URL" | sed 's|https://||')

    if [[ -f "$CONDUIT_CONFIG_FILE" ]]; then
        print_info "Updating Matrix server_name to tunnel domain..."
        $SUDO sed -i "s/^server_name = .*/server_name = \"$TUNNEL_DOMAIN\"/" "$CONDUIT_CONFIG_FILE"
        print_info "Restarting Matrix to apply changes..."
        docker restart armorclaw-conduit 2>/dev/null || true
        sleep 3
    fi

    print_success "Cloudflare Tunnel active!"
    echo ""
    echo -e "  ${BOLD}${GREEN}Tunnel URL:${NC} ${TUNNEL_URL}"
    echo ""
}

start_ngrok_tunnel() {
    print_info "Starting ngrok Tunnel..."

    if ! command -v ngrok &>/dev/null; then
        print_info "ngrok not installed. Installing..."
        curl -s https://ngrok-agent.s3.amazonaws.com/ngrok.asc | sudo tee /etc/apt/trusted.gpg.d/ngrok.asc >/dev/null
        echo "deb https://ngrok-agent.s3.amazonaws.com buster main" | sudo tee /etc/apt/sources.list.d/ngrok.list
        sudo apt-get update -qq && sudo apt-get install -y ngrok
    fi

    if ! ngrok config check &>/dev/null; then
        echo ""
        echo -e "${YELLOW}ngrok requires a free account.${NC}"
        echo ""
        echo -e "  ${BOLD}Setup steps:${NC}"
        echo "  1) Go to https://ngrok.com and sign up (free)"
        echo "  2) Get authtoken: https://dashboard.ngrok.com/get-started/your-authtoken"
        echo "  3) Run: ${CYAN}ngrok config add-authtoken YOUR_TOKEN${NC}"
        echo "  4) Re-run this setup"
        echo ""
        return 1
    fi

    print_info "ngrok is configured. Starting tunnel..."

    if docker ps -a --format '{{.Names}}' | grep -q "^armorclaw-tunnel$"; then
        docker rm -f armorclaw-tunnel 2>/dev/null || true
    fi

    pkill -f "ngrok http" 2>/dev/null || true
    sleep 1

    nohup ngrok http 6167 --log=stdout > /tmp/ngrok.log 2>&1 &
    local ngrok_pid=$!
    sleep 4

    if ! kill -0 $ngrok_pid 2>/dev/null; then
        print_error "ngrok failed to start"
        cat /tmp/ngrok.log 2>/dev/null | tail -10
        return 1
    fi

    print_info "Detecting ngrok URL..."

    local max_attempts=15
    local attempt=1
    local ngrok_url=""

    while [[ $attempt -le $max_attempts ]]; do
        ngrok_url=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null | grep -oE 'https://[a-zA-Z0-9.-]+\.(ngrok\.io|ngrok-free\.app|ngrok\.app)' | head -1)
        if [[ -n "$ngrok_url" ]]; then
            break
        fi
        sleep 1
        echo -n "."
        ((attempt++)) || true
    done
    echo ""

    if [[ -z "$ngrok_url" ]]; then
        print_error "Could not detect ngrok URL automatically"
        echo ""
        echo -e "${YELLOW}ngrok web interface: http://localhost:4040${NC}"
        echo ""
        ngrok_url=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null)
        if [[ -n "$ngrok_url" ]]; then
            echo -e "${YELLOW}API response:${NC}"
            echo "$ngrok_url" | head -c 500
            echo ""
        fi
        echo ""
        print_info "To complete setup manually:"
        print_info "  1) Open http://localhost:4040 in a browser"
        print_info "  2) Copy the HTTPS URL (e.g., https://abc-123.ngrok-free.app)"
        print_info "  3) Run these commands:"
        echo ""
        echo "     sudo sed -i 's/^server_name = .*/server_name = \"YOUR-NGROK-URL\"/' /etc/armorclaw/conduit.toml"
        echo "     docker restart armorclaw-conduit"
        echo ""
        print_info "Then connect Element X to: https://YOUR-NGROK-URL"
        return 1
    fi

    TUNNEL_URL="$ngrok_url"
    TUNNEL_DOMAIN=$(echo "$TUNNEL_URL" | sed 's|https://||')

    print_success "ngrok Tunnel active!"
    echo ""
    echo -e "  ${BOLD}${GREEN}Tunnel URL:${NC} ${TUNNEL_URL}"
    echo ""
    echo -e "  ${YELLOW}⚠ WARNING: ngrok free tier domains change on restart${NC}"
    echo -e "  ${YELLOW}  Mobile clients (Element X) will need reconfiguration${NC}"
    echo -e "  ${YELLOW}  For production use, consider Cloudflare or a static domain${NC}"
    echo ""

    if [[ -f "$CONDUIT_CONFIG_FILE" ]]; then
        print_info "Updating Matrix server_name to tunnel domain..."
        $SUDO sed -i "s/^server_name = .*/server_name = \"$TUNNEL_DOMAIN\"/" "$CONDUIT_CONFIG_FILE"
        print_info "Restarting Matrix to apply changes..."
        docker restart armorclaw-conduit 2>/dev/null || true
        sleep 3
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
# Step 1: Prerequisites Check (Auto)
#=============================================================================

check_prerequisites() {
    print_step "Checking prerequisites..."

    local errors=0

    # Determine sudo usage
    if [[ $EUID -ne 0 ]]; then
        if command -v sudo >/dev/null 2>&1; then
            SUDO="sudo"
            print_done "Sudo detected (elevation will be used when needed)"
        else
            print_error "This script requires root privileges or sudo to be installed."
            exit 1
        fi
    else
        SUDO=""
        print_warning "Running as root is not recommended. Consider running as a normal user."
    fi

    # Check OS
    if [[ -f /etc/os-release ]]; then
        local os_pretty_name=$(grep "^PRETTY_NAME=" /etc/os-release | cut -d= -f2 | tr -d '"')
        print_done "OS: $os_pretty_name"
    else
        print_error "Cannot detect OS"
        ((errors++)) || true
    fi

    if [[ "${ARMORCLAW_SKIP_DOCKER_CHECK:-}" == "true" ]]; then
        print_info "Docker check skipped (ARMORCLAW_SKIP_DOCKER_CHECK=true)"
    elif command -v docker &>/dev/null; then
        if docker info &>/dev/null; then
            print_done "Docker: $(docker --version | awk '{print $3}' | tr -d ',')"
        else
            print_error "Docker is installed but not running"
            print_info "Start with: systemctl start docker"
            ((errors++)) || true
        fi
    else
        print_error "Docker not installed"
        print_info "Install with: curl -fsSL https://get.docker.com | sh"
        ((errors++)) || true
    fi

    # Check memory (minimum 1GB)
    local total_mem=$(free -m | awk '/^Mem:/{print $2}')
    if [[ $total_mem -ge 1024 ]]; then
        print_done "Memory: ${total_mem}MB"
    else
        print_error "Memory: ${total_mem}MB (minimum 1GB required)"
        ((errors++)) || true
    fi

    # Check disk space (minimum 2GB)
    local avail_space=$(df -m / | awk 'NR==2 {print $4}')
    if [[ $avail_space -ge 2048 ]]; then
        print_done "Disk: ${avail_space}MB available"
    else
        print_warning "Disk: ${avail_space}MB available (2GB+ recommended)"
    fi

    if ! command -v qrencode &>/dev/null; then
        print_info "Installing qrencode for QR code display..."
        $SUDO apt-get update -qq && $SUDO apt-get install -y -qq qrencode 2>/dev/null || true
        if command -v qrencode &>/dev/null; then
            print_done "qrencode installed"
        else
            print_warning "Could not install qrencode - QR codes will not display"
        fi
    else
        print_done "qrencode available"
    fi

    if command -v go &>/dev/null; then
        local go_ver=$(go version 2>/dev/null | sed 's/go\([0-9.]*\).*/\1/')
        print_done "Go: $go_ver"
    else
        print_info "Go: not installed (will be installed if needed)"
    fi

    if [[ $errors -gt 0 ]]; then
        fail "Prerequisites check failed. Fix the issues above and run again."
    fi

    print_success "All prerequisites met!"
}

#=============================================================================
# Step 2: Build/Install Bridge
#=============================================================================

install_bridge() {
    print_step "Installing ArmorClaw Bridge..."

    # Check if already installed
    if [[ -x "$INSTALL_DIR/armorclaw-bridge" ]]; then
        print_done "Bridge already installed at $INSTALL_DIR/armorclaw-bridge"

        if ! $NON_INTERACTIVE && prompt_yes_no "Reinstall bridge?" "n"; then
            $SUDO rm -f "$INSTALL_DIR/armorclaw-bridge"
        else
            return 0
        fi
    fi

    # Try binary install first if enabled
    if [[ "${USE_BINARY:-true}" == "true" ]]; then
        detect_arch
        if [[ -n "$BIN_ARCH" ]]; then
            if download_binary; then
                print_success "Bridge installed via binary distribution"
                return 0
            else
                print_warning "Binary installation failed, falling back to source build"
            fi
        fi
    fi

    # Check for dependencies
    if ! command -v go &>/dev/null || ! command -v git &>/dev/null || ! command -v gcc &>/dev/null; then
        print_info "Installing dependencies (Go, Git, Build-essential)..."
        $SUDO apt-get update -qq
        $SUDO apt-get install -y -qq golang-go git build-essential
    fi
    
    if command -v go &>/dev/null; then
        local go_major=$(go version | sed 's/.*go\([0-9]*\)\.\([0-9]*\).*/\1/')
        local go_minor=$(go version | sed 's/.*go\([0-9]*\)\.\([0-9]*\).*/\2/')
        
        if [[ "$go_major" -lt 1 ]] || ([[ "$go_major" -eq 1 ]] && [[ "$go_minor" -lt 22 ]]); then
            print_info "Upgrading Go (need 1.22+ for CGO/SQLCipher)..."
            $SUDO rm -rf /usr/local/go 2>/dev/null || true
            curl -fsSL https://go.dev/dl/go1.26.1.linux-amd64.tar.gz | $SUDO tar -C /usr/local -xzf -
            export PATH="/usr/local/go/bin:$PATH"
            print_done "Go upgraded"
        fi
    fi

    print_info "Building bridge from source..."


    local build_dir="/tmp/armorclaw-src"
    rm -rf "$build_dir"
    mkdir -p "$build_dir"

    git clone --depth 1 https://github.com/Gemutly/ArmorClaw "$build_dir" &>/dev/null &
    show_spinner $! "Fetching source"
    wait $!
    if [[ $? -ne 0 ]]; then
        fail "Failed to clone source"
    fi

    cd "$build_dir/bridge" || fail "Bridge source missing"

    # Build with CGO enabled for SQLCipher
    CGO_ENABLED=1 go build -o armorclaw-bridge ./cmd/bridge >/dev/null 2>&1 &
    show_spinner $! "Building bridge"

    if [[ -f "armorclaw-bridge" ]]; then
        print_done "Bridge built successfully"
    else
        fail "Bridge build failed"
    fi

    # Install to system location
    $SUDO mkdir -p "$INSTALL_DIR"
    $SUDO mv armorclaw-bridge "$INSTALL_DIR/"
    $SUDO chmod +x "$INSTALL_DIR/armorclaw-bridge"
    $SUDO ln -sf "$INSTALL_DIR/armorclaw-bridge" /usr/local/bin/armorclaw-bridge

    print_success "Bridge installed to $INSTALL_DIR/armorclaw-bridge"
}

#=============================================================================
# Step 3: Create System User
#=============================================================================

create_user() {
    print_step "Creating system user..."

    if id "armorclaw" &>/dev/null; then
        print_done "User 'armorclaw' already exists"
    else
        $SUDO useradd -r -s /bin/false -d "$DATA_DIR" armorclaw
        print_done "User 'armorclaw' created"
    fi
}

#=============================================================================
# Step 4: Generate Configuration (Smart Defaults)
#=============================================================================

generate_config() {
    print_step "Generating configuration..."

    # Ensure armorclaw user exists
    id -u armorclaw >/dev/null 2>&1 || $SUDO useradd --system --no-create-home --shell /bin/false armorclaw

    # Create directories
    $SUDO mkdir -p "$CONFIG_DIR"
    $SUDO mkdir -p "$DATA_DIR"
    $SUDO mkdir -p "$RUN_DIR"
    $SUDO chown armorclaw:armorclaw "$RUN_DIR" "$DATA_DIR" 2>/dev/null || true
    $SUDO chmod 700 "$DATA_DIR"
    $SUDO chmod 770 "$RUN_DIR"

    local config_file="$CONFIG_DIR/config.toml"

    # Guard: preserve existing config unless FORCE_REGEN is set
    if [[ -f "$config_file" ]] && [[ "${FORCE_REGEN:-false}" != "true" ]]; then
        print_done "Config already exists, preserving: $config_file"
        return 0
    fi

    # Generate provisioning secret for QR codes
    local provisioning_secret=$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | xxd -p -c 32)

    # Create config with smart defaults
    cat > "$config_file" <<EOF
# ArmorClaw Bridge Configuration
# Generated by quick setup on $(date)
#
# Quick setup uses secure defaults. Customize later in this file.
# See: docs/guides/configuration.md for all options.

[server]
socket_path = "$SOCKET_PATH"
daemonize = false

[keystore]
db_path = "$DATA_DIR/keystore.db"

# Matrix disabled by default - enable with: ./deploy/setup-matrix.sh
[matrix]
enabled = false
homeserver_url = ""
username = ""
password = ""
device_id = "armorclaw-bridge"

# Budget tracking with sensible defaults
[budget]
daily_limit_usd = $DEFAULT_DAILY_BUDGET
monthly_limit_usd = $DEFAULT_MONTHLY_BUDGET
alert_threshold = 80.0
hard_stop = true

# Logging configuration
[logging]
level = "$DEFAULT_LOG_LEVEL"
format = "$DEFAULT_LOG_FORMAT"
output = "stdout"

# Provisioning for secure device setup (QR codes)
[provisioning]
signing_secret = "$provisioning_secret"
default_expiry_seconds = 60
max_expiry_seconds = 300
one_time_use = true

# Voice disabled by default
[voice]
enabled = false

# WebRTC signaling disabled by default
[webrtc]
signaling_enabled = false

# Notifications disabled by default
[notifications]
enabled = false

# Event bus disabled by default
[eventbus]
websocket_enabled = false

# Discovery enabled for local network
[discovery]
enabled = true
port = 8080
tls = false
EOF

    # Ensure error store path to config (idempotent)
    ensure_error_store_config

    $SUDO chown armorclaw:armorclaw "$config_file"
    $SUDO chmod 640 "$config_file"

    # Config sanity check
    grep -q "store_path" "$config_file" || echo "Warning: errors.store_path not configured"

    print_success "Configuration generated: $config_file"
    print_info "Provisioning secret generated for secure QR code setup"
}

#=============================================================================
# Step 5: Initialize Keystore
#=============================================================================

init_keystore() {
    print_step "Initializing keystore..."

    local keystore_db="$DATA_DIR/keystore.db"

    if [[ -f "$keystore_db" ]]; then
        print_done "Keystore already exists"
        return 0
    fi

    # Initialize by running bridge with --init flag
    if $SUDO -u armorclaw "$INSTALL_DIR/armorclaw-bridge" --init 2>/dev/null; then
        print_done "Keystore initialized"
    else
        # Fallback: keystore will be created on first start
        print_info "Keystore will be created on first bridge start"
    fi
}

#=============================================================================
# Step 6: Setup Systemd Service
#=============================================================================

setup_systemd() {
    print_step "Setting up systemd service..."

    if [[ "${ARMORCLAW_SKIP_SYSTEMD:-}" == "true" ]] || ! command -v systemctl &>/dev/null; then
        print_info "Systemd setup skipped (ARMORCLAW_SKIP_SYSTEMD=${ARMORCLAW_SKIP_SYSTEMD:-false}, systemctl=$(command -v systemctl || echo 'not found'))"
        return 0
    fi

    local service_file="/etc/systemd/system/armorclaw-bridge.service"

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

ExecStart=$INSTALL_DIR/armorclaw-bridge -config $CONFIG_DIR/config.toml

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

    $SUDO systemctl daemon-reload
    print_done "Systemd service configured"
}

#=============================================================================
# Step 7: Start Bridge
#=============================================================================

start_bridge() {
    print_step "Starting bridge..."

    if [[ "${ARMORCLAW_SKIP_SYSTEMD:-}" == "true" ]] || ! command -v systemctl &>/dev/null; then
        print_info "Systemd start skipped (ARMORCLAW_SKIP_SYSTEMD=${ARMORCLAW_SKIP_SYSTEMD:-false})"
        if [[ -x "$INSTALL_DIR/armorclaw-bridge" ]]; then
            print_info "Starting bridge directly..."
            if [[ -f /.dockerenv ]] || [[ -f /run/.containerenv ]] || [[ "${container:-}" == "docker" ]]; then
                print_info "Docker environment detected — bridge will run as PID 1"
                print_info "Note: setup script will complete, then exec bridge as final process"
                DOCKER_MODE=true
                $INSTALL_DIR/armorclaw-bridge -config $CONFIG_DIR/config.toml &
            else
                $INSTALL_DIR/armorclaw-bridge -config $CONFIG_DIR/config.toml &
            fi
            local wait_count=0
            while [[ ! -S "$SOCKET_PATH" ]] && [[ $wait_count -lt 30 ]]; do
                sleep 0.5
                ((wait_count++)) || true
            done
            if [[ -S "$SOCKET_PATH" ]]; then
                print_done "Bridge socket ready: $SOCKET_PATH"
            else
                print_warning "Bridge socket not ready after 15s"
            fi
        fi
        return 0
    fi

    if $SUDO systemctl start armorclaw-bridge; then
        print_done "Bridge service started"
    else
        fail "Failed to start bridge service"
    fi

    # Wait for socket
    print_info "Waiting for bridge to be ready..."
    local wait_count=0
    while [[ ! -S "$SOCKET_PATH" ]] && [[ $wait_count -lt 30 ]]; do
        sleep 0.5
        ((wait_count++)) || true
    done

    if [[ -S "$SOCKET_PATH" ]]; then
        print_done "Bridge socket ready: $SOCKET_PATH"
    else
        print_warning "Bridge socket not ready after 15s"
        print_info "Check logs: journalctl -u armorclaw-bridge -n 50"
    fi
}

#=============================================================================
# Step 8: Verify Health
#=============================================================================

verify_health() {
    print_step "Verifying installation..."

    local all_ok=true

    # Check directories
    [[ -d "$CONFIG_DIR" ]] && print_done "Config directory" || { print_error "Config directory missing"; all_ok=false; }
    [[ -d "$DATA_DIR" ]] && print_done "Data directory" || { print_error "Data directory missing"; all_ok=false; }
    [[ -d "$RUN_DIR" ]] && print_done "Run directory" || { print_error "Run directory missing"; all_ok=false; }

    # Check binary
    [[ -x "$INSTALL_DIR/armorclaw-bridge" ]] && print_done "Bridge binary" || { print_error "Bridge binary missing"; all_ok=false; }

    # Check config
    [[ -f "$CONFIG_DIR/config.toml" ]] && print_done "Configuration file" || { print_error "Configuration file missing"; all_ok=false; }

    # Check service
    systemctl is-active armorclaw-bridge &>/dev/null && print_done "Service running" || { print_warning "Service not running"; }

    # Check socket
    [[ -S "$SOCKET_PATH" ]] && print_done "Bridge socket" || { print_warning "Bridge socket not available"; }

    if $all_ok; then
        print_success "Installation verified!"
    else
        print_warning "Some checks failed - review above"
    fi
}

#=============================================================================
# Step 9: Matrix Server (Optional but recommended)
#=============================================================================

# (Handled in main flow)

#=============================================================================
# Step 10: Optional API Key
#=============================================================================

prompt_api_key() {
    if $NON_INTERACTIVE; then
        print_info "Skipping API key setup (non-interactive mode)"
        print_info "Add keys later with: armorclaw-bridge add-key"
        return 0
    fi

    print_step "API Key Setup (Optional)"

    echo ""
    echo "  Would you like to add an API key now?"
    echo "  You can skip this and add keys later."
    echo ""

    if ! prompt_yes_no "Add an API key now?" "n"; then
        print_info "You can add API keys later using:"
        echo "    sudo armorclaw-bridge add-key --provider <provider> --token <token>"
        return 0
    fi

    echo ""
    echo "  Available AI Providers:"
    echo "  ┌──────────────────────────────────────────────────────────────────┐"
    echo "  │  1) openai        - OpenAI (GPT-4, GPT-3.5, o1)                  │"
    echo "  │  2) anthropic      - Anthropic (Claude)                          │"
    echo "  │  3) google         - Google Gemini (Pro, Ultra, Flash)           │"
    echo "  │  4) xai            - xAI (Grok)                                  │"
    echo "  │  5) openrouter     - OpenRouter (Multi-provider aggregator)      │"
    echo "  │  6) zhipu          - Zhipu AI (Z AI) [aliases: zai, glm]         │"
    echo "  │  7) deepseek       - DeepSeek (R1, V3)                           │"
    echo "  │  8) moonshot       - Moonshot AI                                 │"
    echo "  │  9) nvidia         - NVIDIA NIM                                  │"
    echo "  │ 10) groq           - Groq (Fast inference)                       │"
    echo "  │ 11) cloudflare     - Cloudflare AI Gateway                       │"
    echo "  │ 12) ollama         - Local Ollama instance                       │"
    echo "  └──────────────────────────────────────────────────────────────────┘"

    echo ""
    echo -ne "  Select provider number [1-12]: "
    prompt_read -r provider_choice

    # Provider base URLs and keys
    declare -A PROVIDERS=(
        ["1"]="openai"
        ["2"]="anthropic"
        ["3"]="google"
        ["4"]="xai"
        ["5"]="openrouter"
        ["6"]="zhipu"
        ["7"]="deepseek"
        ["8"]="moonshot"
        ["9"]="nvidia"
        ["10"]="groq"
        ["11"]="cloudflare"
        ["12"]="ollama"
    )

    # Validate provider choice
    if [[ -z "${PROVIDERS[$provider_choice]}" ]]; then
        print_error "Invalid provider selection"
        return 1
    fi

    provider_key="${PROVIDERS[$provider_choice]}"

    echo ""
    echo -ne "  API Key for ${provider_key}: "
    prompt_read -s key_token
    echo ""

    if [[ -z "$key_token" ]]; then
        print_warning "No key provided, skipping"
        return 0
    fi

    # Add key via RPC (if bridge is running)
    if [[ -S "$SOCKET_PATH" ]]; then
        local key_id="${provider_key}-main"
        local rpc_cmd='{"jsonrpc":"2.0","method":"add_key","params":{"id":"'"$key_id"'","provider":"'"$provider_key"'","token":"'"$key_token"'","display_name":"'"$provider_key"' API Key"},"id":1}'

        if echo "$rpc_cmd" | socat - UNIX-CONNECT:"$SOCKET_PATH" &>/dev/null; then
            print_done "API key added: $key_id"
        else
            print_warning "Could not add key via RPC - add manually"
            print_info "Command: echo '$rpc_cmd' | socat - UNIX-CONNECT:$SOCKET_PATH"
        fi
    else
        print_warning "Bridge not ready - add key manually after setup"
    fi
}

#=============================================================================
# Step 11: Generate QR Code for Provisioning
#=============================================================================

generate_qr_code() {
    print_step "Device Provisioning"

    # Check both the flag AND actual running state
    if [[ "$MATRIX_ENABLED" == "false" ]] && ! is_matrix_running; then
        echo ""
        print_warning "Matrix server not installed or running."
        echo "QR connection requires a Matrix server (Conduit)."
        echo ""
        echo "To enable:"
        echo "  sudo ./deploy/setup-matrix.sh"
        echo ""
        return 0
    fi

    local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local provision_script="$script_dir/armorclaw-provision.sh"

    if [[ -x "$provision_script" ]]; then
        # Use dedicated provisioning script
        "$provision_script" --expiry 300
    else
        # Fallback inline generation
        print_info "Generating provisioning QR code..."

        local hostname=$(hostname)
        local ip_address=$(hostname -I | awk '{print $1}')

        # Generate config JSON matching ArmorChat's expected format
        local expiry=$(($(date +%s) + 300))  # 5 minutes
        local matrix_url="http://${ip_address}:8448"
        local rpc_url="http://${ip_address}:8443/api"
        local ws_url="ws://${ip_address}:8443/ws"
        local push_url="http://${ip_address}:5000"

        # Try to register token via bridge RPC so it's recognized during claim
        local setup_token=""
        if [[ -S "$SOCKET_PATH" ]]; then
            local rpc_cmd='{"jsonrpc":"2.0","method":"provisioning.start","params":{"expiry_seconds":300},"id":1}'
            local rpc_result
            rpc_result=$(echo "$rpc_cmd" | socat - UNIX-CONNECT:"$SOCKET_PATH" 2>/dev/null) || true
            if [[ -n "$rpc_result" ]]; then
                setup_token=$(echo "$rpc_result" | grep -oP '"setup_token":\s*"\K[^"]+' 2>/dev/null || true)
            fi
        fi
        if [[ -z "$setup_token" ]]; then
            setup_token="stp_$(openssl rand -hex 24)"
            print_warning "Bridge not reachable — token may not be recognized"
        fi

        local config_json=$(cat <<EOF
{"matrix_homeserver":"${matrix_url}","rpc_url":"${rpc_url}","ws_url":"${ws_url}","push_gateway":"${push_url}","server_name":"${hostname}","setup_token":"${setup_token}","expires_at":${expiry}}
EOF
)
        # Base64 encode (URL-safe, single-line)
        local config_b64=$(echo -n "$config_json" | base64 -w0 2>/dev/null || echo -n "$config_json" | base64 | tr -d '\n')
        config_b64=$(echo -n "$config_b64" | tr '+/' '-_' | tr -d '=')
        local provision_url="armorclaw://config?d=${config_b64}"

        echo ""
        echo -e "  ${BOLD}Scan this QR code with ArmorChat to connect:${NC}"
        echo ""

        # Install qrencode if not available
        if ! command -v qrencode &>/dev/null; then
            print_info "Installing qrencode for QR display..."
            $SUDO apt-get update -qq && $SUDO apt-get install -y -qq qrencode 2>/dev/null || true
        fi

        # Try to generate QR code with qrencode
        if command -v qrencode &>/dev/null; then
            qrencode -t UTF8 "$provision_url" 2>/dev/null || \
            qrencode -t ASCII "$provision_url" 2>/dev/null
        else
            # ASCII fallback - show URL
            echo -e "  ${CYAN}──────────────────────────────────────${NC}"
            echo -e "  ${CYAN}│${NC} Install qrencode for QR display:   ${CYAN}│${NC}"
            echo -e "  ${CYAN}│${NC}   sudo apt install qrencode         ${CYAN}│${NC}"
            echo -e "  ${CYAN}──────────────────────────────────────${NC}"
            echo ""
            echo -e "  ${BOLD}Manual connection:${NC}"
            echo -e "  ${CYAN}$provision_url${NC}"
        fi

        echo ""
        echo -e "  ${YELLOW}Note:${NC} QR code expires in 5 minutes"
    fi

    echo ""
    print_info "Generate new codes anytime with: sudo ./deploy/armorclaw-provision.sh"
}

generate_element_connection_info() {
    local tunnel_url="$1"
    
    [[ -z "$tunnel_url" ]] && return 0
    
    local element_web_login="https://app.element.io/#/login?hs_url=${tunnel_url}"
    
    echo ""
    echo -e "${CYAN}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC}            ${BOLD}Element X / Matrix Connection${NC}                   ${CYAN}║${NC}"
    echo -e "${CYAN}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${BOLD}Element Web Quick Login:${NC}"
    echo -e "  ${CYAN}$element_web_login${NC}"
    echo ""
    
    if command -v qrencode &>/dev/null; then
        echo -e "${BOLD}QR for Element Web:${NC}"
        qrencode -t UTF8 "$element_web_login" 2>/dev/null || \
        qrencode -t ASCII "$element_web_login" 2>/dev/null || true
        echo ""
    fi
    
    echo -e "${BOLD}Element X Mobile:${NC}"
    echo -e "  Homeserver: ${GREEN}${tunnel_url}${NC}"
    echo -e "  Username:   ${MATRIX_ADMIN_USER}"
    echo -e "${YELLOW}  (Element X requires manual entry — no QR login)${NC}"
    echo ""
}

#=============================================================================
# Service Verification
#=============================================================================

verify_services() {
    echo ""
    echo -e "${BOLD}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BOLD}║                    SERVICE STATUS                             ║${NC}"
    echo -e "${BOLD}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    local bridge_ok=false
    local matrix_ok=false
    local tunnel_ok=false
    local tunnel_type=""
    local tunnel_url=""
    
    echo -e "  ${BOLD}Service${NC}              ${BOLD}Status${NC}         ${BOLD}Details${NC}"
    echo -e "  ─────────────────────────────────────────────────────────────"
    
    if systemctl is-active --quiet armorclaw-bridge 2>/dev/null; then
        echo -e "  Bridge (ArmorClaw)    ${GREEN}● running${NC}      port 8443"
        bridge_ok=true
    else
        echo -e "  Bridge (ArmorClaw)    ${RED}● stopped${NC}      check: journalctl -u armorclaw-bridge"
    fi
    
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "^armorclaw-conduit$"; then
        echo -e "  Matrix (Conduit)      ${GREEN}● running${NC}      port 6167"
        matrix_ok=true
    else
        echo -e "  Matrix (Conduit)      ${RED}● stopped${NC}      docker start armorclaw-conduit"
    fi
    
    if pgrep -f "ngrok http" &>/dev/null; then
        tunnel_url=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null | grep -oE 'https://[a-z0-9-]+\.(ngrok\.io|ngrok-free\.app)' | head -1)
        if [[ -n "$tunnel_url" ]]; then
            echo -e "  Tunnel (ngrok)        ${GREEN}● active${NC}       $tunnel_url"
            tunnel_ok=true
            tunnel_type="ngrok"
            TUNNEL_URL="${TUNNEL_URL:-$tunnel_url}"
        else
            echo -e "  Tunnel (ngrok)        ${YELLOW}● starting${NC}     waiting for URL..."
            tunnel_type="ngrok"
        fi
    elif docker ps --format '{{.Names}}' 2>/dev/null | grep -q "^armorclaw-tunnel$"; then
        tunnel_url=$(docker logs armorclaw-tunnel 2>&1 | grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' | tail -1)
        if [[ -n "$tunnel_url" ]]; then
            echo -e "  Tunnel (Cloudflare)   ${GREEN}● active${NC}       $tunnel_url"
            tunnel_ok=true
            tunnel_type="cloudflare"
            TUNNEL_URL="${TUNNEL_URL:-$tunnel_url}"
        else
            echo -e "  Tunnel (Cloudflare)   ${YELLOW}● starting${NC}     waiting for URL..."
            tunnel_type="cloudflare"
        fi
    elif [[ -n "$TUNNEL_URL" ]]; then
        echo -e "  Tunnel                ${GREEN}● active${NC}       $TUNNEL_URL"
        tunnel_ok=true
    else
        echo -e "  Tunnel                ${YELLOW}● not setup${NC}    optional for HTTPS"
    fi
    
    echo ""
    
    if $bridge_ok && $matrix_ok; then
        echo -e "  ${GREEN}✓ All core services running${NC}"
        return 0
    elif $bridge_ok; then
        echo -e "  ${YELLOW}⚠ Matrix not running - Element X will not connect${NC}"
        return 1
    else
        echo -e "  ${RED}✗ Bridge not running${NC}"
        return 2
    fi
}

#=============================================================================
# Completion Message
#=============================================================================

print_completion() {
    verify_services
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║${NC}                 ${BOLD}Setup Complete!${NC}                              ${GREEN}║${NC}"
    echo -e "${GREEN}║${NC}             ${BOLD}ArmorClaw is ready to use.${NC}                       ${GREEN}║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════════╝${NC}"

    if [[ "$MATRIX_ENABLED" == "true" ]] || is_matrix_running; then
        echo ""
        echo -e "${BOLD}╔═══════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${BOLD}║                  ELEMENT X CREDENTIALS                        ║${NC}"
        echo -e "${BOLD}╚═══════════════════════════════════════════════════════════════╝${NC}"
        echo ""
        
        if [[ -n "$TUNNEL_URL" ]]; then
            echo -e "  ${BOLD}Homeserver URL:${NC}"
            echo -e "    ${CYAN}${TUNNEL_URL}${NC}"
        else
            local local_ip=$(hostname -I | awk '{print $1}')
            echo -e "  ${BOLD}Homeserver URL:${NC}"
            echo -e "    ${CYAN}http://${MATRIX_DOMAIN:-$local_ip}:6167${NC}"
            echo ""
            echo -e "  ${YELLOW}⚠ Element X mobile requires HTTPS${NC}"
            echo -e "  ${YELLOW}  Run tunnel: docker run -d --name armorclaw-tunnel \\${NC}"
            echo -e "  ${YELLOW}    cloudflare/cloudflared:latest tunnel --url http://host.docker.internal:6167${NC}"
        fi
        echo ""
        echo -e "  ${BOLD}Username:${NC}  @${MATRIX_ADMIN_USER}:${TUNNEL_DOMAIN:-${MATRIX_DOMAIN:-$(hostname -I | awk '{print $1}')}}"
        echo -e "  ${BOLD}Password:${NC}  ${MATRIX_ADMIN_PASSWORD}"
        echo ""
        
        if [[ -n "$TUNNEL_URL" ]]; then
            echo -e "  ${GREEN}📱 Connect from anywhere via Cloudflare Tunnel${NC}"
        else
            echo -e "  ${YELLOW}📱 Local network only - setup tunnel for remote access${NC}"
        fi
    fi

    echo -e "${BOLD}Quick Start:${NC}"
    echo ""
    if [[ "$MATRIX_ENABLED" == "true" ]] || is_matrix_running; then
        echo -e "  1. ${CYAN}Connect Element X${NC} (see credentials above)"
        echo -e "  2. ${CYAN}Or scan QR${NC} with ArmorChat (if displayed)"
    else
        echo -e "  1. ${CYAN}Enable Matrix${NC} to get Element X connection:"
        echo "     sudo ./deploy/setup-matrix.sh"
    fi
    echo -e "  3. ${CYAN}Add API key${NC}:"
    echo "     sudo armorclaw-bridge add-key --provider openai --token sk-..."
    echo -e "  4. ${CYAN}Start an agent${NC}:"
    echo "     sudo armorclaw-bridge start --key openai-main"
    echo ""

    echo -e "${BOLD}Service Commands:${NC}"
    echo ""
    echo -e "  Status:  ${CYAN}sudo systemctl status armorclaw-bridge${NC}"
    echo -e "  Logs:    ${CYAN}sudo journalctl -u armorclaw-bridge -f${NC}"
    echo -e "  Restart: ${CYAN}sudo systemctl restart armorclaw-bridge${NC}"
    echo ""

    echo -e "${BOLD}Next Steps:${NC}"
    echo ""
    echo -e "  ${CYAN}• Harden security:${NC}     ./deploy/armorclaw-harden.sh"
    echo -e "  ${CYAN}• New device QR:${NC}       ./deploy/armorclaw-provision.sh"
    echo -e "  ${CYAN}• Full configuration:${NC}  nano $CONFIG_DIR/config.toml"
    echo ""

    echo -e "${BOLD}Documentation:${NC}"
    echo ""
    echo "  Element X Guide: docs/guides/element-x-quickstart.md"
    echo "  Configuration:   docs/guides/configuration.md"
    echo "  Full Docs:       docs/index.md"
    echo ""

    if [[ -f "$LOG_FILE" ]]; then
        echo -e "${BOLD}Setup log:${NC} $LOG_FILE"
    fi

    echo ""
}

#=============================================================================
# Main Flow
#=============================================================================

main() {
    # Initialize logging
    init_logging

    # Print header
    print_header

    local bootstrap_flag="$INSTALL_DIR/.bootstrapped"
    if [[ -f "$bootstrap_flag" ]]; then
        print_done "Already bootstrapped — starting bridge directly"
        start_bridge

        if $CI_MODE; then
            print_success "CI smoke test passed"
            exec tail -f /dev/null
        fi

        if [[ -f /.dockerenv ]] || [[ -f /run/.containerenv ]] || [[ "${container:-}" == "docker" ]]; then
            local bridge_pid=$(pgrep -f "armorclaw-bridge")
            if [[ -n "$bridge_pid" ]]; then
                kill "$bridge_pid" 2>/dev/null || true
                sleep 1
            fi
            exec $INSTALL_DIR/armorclaw-bridge -config $CONFIG_DIR/config.toml
        fi

        exit 0
    fi

    # Run setup steps
    check_prerequisites

    # Network detection and Cloudflare mode selection
    if command -v detect_network_environment >/dev/null 2>&1; then
        detect_network_environment
    else
        print_warning "Cloudflare functions not available - skipping network detection"
    fi

    install_bridge
    create_user
    generate_config
    init_keystore
    setup_systemd
    start_bridge
    verify_health

    touch "$INSTALL_DIR/.bootstrapped"

    # Step 9: Matrix Server (auto-install in quickstart)
    print_step "Matrix Server"
    if is_matrix_running; then
        print_done "Matrix already running"
        MATRIX_ENABLED="true"
    else
        echo ""
        echo "  Installing Matrix server for ArmorChat connections..."
        if ensure_matrix; then
            MATRIX_ENABLED="true"
            print_success "Matrix installed and running"
        else
            print_warning "Matrix installation failed."
            echo ""
            echo "  To install manually later:"
            echo "    sudo ./deploy/setup-matrix.sh"
            echo ""
        fi
    fi

    if $CI_MODE; then
        print_success "CI smoke test passed - basic setup complete"
        exec tail -f /dev/null
    fi

    prompt_api_key
    generate_qr_code

    # Cloudflare HTTPS setup (if Matrix is running)
    if [[ "$MATRIX_ENABLED" == "true" ]] || is_matrix_running; then
        if command -v prompt_cloudflare_mode >/dev/null 2>&1 && ! $NON_INTERACTIVE; then
            prompt_cloudflare_mode

            # Route to appropriate Cloudflare setup based on mode
            case "${CF_MODE:-}" in
                tunnel)
                    if command -v setup_cloudflare_tunnel >/dev/null 2>&1; then
                        setup_cloudflare_tunnel
                    else
                        print_warning "Cloudflare Tunnel setup function not available - using legacy tunnel"
                        start_cloudflare_tunnel
                    fi
                    ;;
                proxy)
                    if command -v setup_cloudflare_proxy >/dev/null 2>&1; then
                        setup_cloudflare_proxy
                    else
                        print_warning "Cloudflare Proxy setup function not available - using legacy tunnel"
                        start_cloudflare_tunnel
                    fi
                    ;;
                *)
                    # Default to legacy tunnel for backward compatibility
                    start_cloudflare_tunnel
                    ;;
            esac
        else
            # Default behavior - use legacy tunnel
            start_cloudflare_tunnel
        fi
    fi

    print_completion

    if [[ "${DOCKER_MODE:-false}" == "true" ]] && ! $CI_MODE; then
        print_info "Handing off to bridge as PID 1..."
        local bridge_pid=$(pgrep -f "armorclaw-bridge")
        if [[ -n "$bridge_pid" ]]; then
            kill "$bridge_pid" 2>/dev/null || true
            sleep 1
        fi
        exec $INSTALL_DIR/armorclaw-bridge -config $CONFIG_DIR/config.toml
    fi
}

# Run main function
main "$@"
