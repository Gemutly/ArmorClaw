#!/bin/bash
# =============================================================================
# ArmorClaw Production Installer v4.0
# Purpose: Self-aware, deterministic, hardened deployment with blue/green support
# Supported: Ubuntu 22.04/24.04, Debian 12
# Version: 0.3.1
# =============================================================================

set -euo pipefail

# =============================================================================
# Section 1: Constants and Configuration
# =============================================================================

readonly SCRIPT_VERSION="0.3.1"
readonly SCRIPT_NAME="installer-v4.sh"
readonly MIN_RAM_MB=4096
readonly MIN_CPU_CORES=2
readonly MIN_DISK_GB=20

# Installation paths
readonly INSTALL_DIR="/opt/armorclaw"
readonly CONFIG_DIR="/etc/armorclaw"
readonly DATA_DIR="/var/lib/armorclaw"
readonly RUN_DIR="/run/armorclaw"
readonly LOG_DIR="/var/log/armorclaw"
readonly NGINX_SITES_DIR="/etc/nginx/sites-available"
readonly NGINX_ENABLED_DIR="/etc/nginx/sites-enabled"

# Bridge configuration
readonly BRIDGE_USER="armorclaw"
readonly BRIDGE_GROUP="armorclaw"
readonly BRIDGE_SOCKET="/run/armorclaw/bridge.sock"
readonly BRIDGE_DEFAULT_PORT=9000
readonly SIGNING_KEY_FPR="A1482657223EAFE1C481B74A8F535F90685749E0"
readonly REPO="Gemutly/ArmorClaw"
readonly VERSION="${VERSION:-main}"

# Release URLs
readonly RELEASE_BASE_URL="https://github.com/${REPO}/releases/download"
readonly CHECKSUM_URL="checksums.txt"

# State file for rollback
readonly STATE_FILE="${DATA_DIR}/installer-state.json"

# Error codes
readonly E001="E001: Must run as root"
readonly E002="E002: Container environment without systemd detected"
readonly E003="E003: No public IPv4 address found"
readonly E004="E004: Rootless Docker not supported"
readonly E005="E005: Insufficient RAM (minimum ${MIN_RAM_MB}MB required)"
readonly E006="E006: Insufficient CPU (minimum ${MIN_CPU_CORES} cores required)"
readonly E007="E007: Insufficient disk space (minimum ${MIN_DISK_GB}GB required)"
readonly E008="E008: Domain does not resolve to this server"
readonly E009="E009: Nginx configuration test failed"
readonly E010="E010: Bridge health check failed"
readonly E011="E011: Systemd service start failed"

# =============================================================================
# Section 2: Color Output and Logging
# =============================================================================

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
BOLD='\033[1m'
NC='\033[0m'

# Logging
LOG_FILE="/var/log/armorclaw/installer-$(date +%Y%m%d-%H%M%S).log"

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
    echo "[INFO] $(date '+%Y-%m-%d %H:%M:%S') $1" >> "$LOG_FILE" 2>/dev/null || true
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
    echo "[SUCCESS] $(date '+%Y-%m-%d %H:%M:%S') $1" >> "$LOG_FILE" 2>/dev/null || true
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
    echo "[WARNING] $(date '+%Y-%m-%d %H:%M:%S') $1" >> "$LOG_FILE" 2>/dev/null || true
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
    echo "[ERROR] $(date '+%Y-%m-%d %H:%M:%S') $1" >> "$LOG_FILE" 2>/dev/null || true
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

abort() {
    local code="$1"
    local message="$2"
    log_error "$code - $message"
    echo ""
    echo -e "${RED}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${RED}║ Installation Aborted                                           ║${NC}"
    echo -e "${RED}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Error: $code"
    echo "Message: $message"
    echo ""
    echo "Please fix the issue and run the installer again."
    echo "Log file: $LOG_FILE"
    exit 1
}

# =============================================================================
# Section 3: Helper Functions
# =============================================================================

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
            log_warning "Unsupported architecture for binary distribution: $arch"
            BIN_ARCH=""
            ;;
    esac
}

prompt_yes_no() {
    local prompt="$1"
    local default="${2:-n}"

    if [[ "$NON_INTERACTIVE" == "true" ]]; then
        echo "$default"
        return
    fi

    while true; do
        if [[ "$default" == "y" ]]; then
            echo -ne "${CYAN}$prompt [Y/n]: ${NC}"
        else
            echo -ne "${CYAN}$prompt [y/N]: ${NC}"
        fi

        prompt_read -r response
        response="${response:-$default}"

        case "$response" in
            [Yy]|[Yy][Ee][Ss])
                echo "y"
                return 0
                ;;
            [Nn]|[Nn][Oo])
                echo "n"
                return 1
                ;;
        esac

        echo -e "${YELLOW}Please answer yes or no.${NC}"
    done
}

prompt_input() {
    local prompt="$1"
    local default="$2"

    if [[ "$NON_INTERACTIVE" == "true" ]]; then
        echo "$default"
        return
    fi

    local result
    if [[ -n "$default" ]]; then
        echo -ne "${CYAN}$prompt [$default]: ${NC}"
    else
        echo -ne "${CYAN}$prompt: ${NC}"
    fi

    prompt_read -r result
    echo "${result:-$default}"
}

check_command() {
    local cmd="$1"
    command -v "$cmd" &>/dev/null
}

install_packages() {
    local packages=("$@")
    log_info "Installing packages: ${packages[*]}"

    if check_command apt-get; then
        DEBIAN_FRONTEND=noninteractive apt-get update -qq
        DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "${packages[@]}"
    elif check_command yum; then
        yum install -y -q "${packages[@]}"
    else
        abort "E000" "No supported package manager found"
    fi
}

# =============================================================================
# Global Flags
# =============================================================================

NON_INTERACTIVE="false"
DRY_RUN="false"
DOMAIN_MODE="false"
DOMAIN=""
BRIDGE_PORT="$BRIDGE_DEFAULT_PORT"
TELEMETRY="false"
UPGRADE_MODE="false"
ROLLBACK_MODE="false"
PROVIDER="unknown"
PUBLIC_IP=""
PRIVATE_IP=""
HOSTNAME_FQDN=""

# Detection results
declare -A DETECTION_RESULTS

# =============================================================================
# Section 4: Argument Parsing and Help
# =============================================================================

show_help() {
    cat <<EOF
ArmorClaw Production Installer v${SCRIPT_VERSION}

Usage: $SCRIPT_NAME [OPTIONS]

Options:
  --yes, --non-interactive   Skip confirmation prompts
  --dry-run                  Validate only, no changes
  --domain=DOMAIN            Force domain mode with specified domain
  --bridge-port=PORT         Custom bridge base port (default: 9000)
  --upgrade                  Trigger blue/green upgrade
  --rollback                 Restore last backup
  --telemetry                Enable anonymous telemetry
  --help, -h                 Show this help message

Examples:
  # Fresh install with domain
  $SCRIPT_NAME --yes --domain=example.com

  # Dry run to validate environment
  $SCRIPT_NAME --dry-run

  # Upgrade existing installation
  $SCRIPT_NAME --yes --upgrade

  # Rollback to previous version
  $SCRIPT_NAME --yes --rollback

Non-negotiable Constraints:
  - NEVER bind bridge to public interface (localhost only)
  - NEVER run bridge as root (uses armorclaw user)
  - NEVER git clone during installation (binary download only)
  - NEVER naive health checks (real JSON-RPC validation)
  - ALWAYS require explicit consent for telemetry

EOF
    exit 0
}

parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --yes|--non-interactive)
                NON_INTERACTIVE="true"
                shift
                ;;
            --dry-run)
                DRY_RUN="true"
                shift
                ;;
            --domain=*)
                DOMAIN="${1#*=}"
                DOMAIN_MODE="true"
                shift
                ;;
            --bridge-port=*)
                BRIDGE_PORT="${1#*=}"
                shift
                ;;
            --upgrade)
                UPGRADE_MODE="true"
                shift
                ;;
            --rollback)
                ROLLBACK_MODE="true"
                shift
                ;;
            --telemetry)
                TELEMETRY="true"
                shift
                ;;
            --help|-h)
                show_help
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                ;;
        esac
    done
}

# =============================================================================
# Section 4: 13 Environment Detection Modules
# =============================================================================

# Module 1: System Environment Detection
detect_system_environment() {
    log_info "Detecting system environment..."

    local result="ok"
    local details=""

    # Check if running as root
    if [[ $EUID -ne 0 ]]; then
        DETECTION_RESULTS["system_environment"]="FAIL:$E001"
        return 1
    fi

    # Check if systemd is available
    if [[ ! -d /run/systemd/system ]]; then
        # Check if we're in a container
        if [[ -f /.dockerenv ]] || [[ -f /run/.containerenv ]]; then
            DETECTION_RESULTS["system_environment"]="FAIL:$E002"
            return 1
        fi
        result="warn"
        details="systemd not available"
    fi

    # Check OS
    if [[ -f /etc/os-release ]]; then
        local os_pretty_name=$(grep "^PRETTY_NAME=" /etc/os-release | cut -d= -f2 | tr -d '"')
        local os_id=$(grep "^ID=" /etc/os-release | cut -d= -f2 | tr -d '"')
        details="$os_pretty_name"
        case "$os_id" in
            ubuntu|debian)
                result="ok"
                ;;
            *)
                result="warn"
                details="$os_pretty_name (untested)"
                ;;
        esac
    else
        DETECTION_RESULTS["system_environment"]="FAIL:Cannot detect OS"
        return 1
    fi

    DETECTION_RESULTS["system_environment"]="OK:$details"
    log_success "System: $details"
    return 0
}

# Module 2: Cloud Provider Detection
detect_provider() {
    log_info "Detecting cloud provider..."

    local provider="unknown"
    local metadata=""

    # Check for cloud provider metadata endpoints
    # AWS
    if curl -sf --connect-timeout 2 http://169.254.169.254/latest/meta-data/instance-id &>/dev/null; then
        provider="aws"
        metadata=$(curl -sf http://169.254.169.254/latest/meta-data/instance-type 2>/dev/null || echo "unknown")
    # GCP
    elif curl -sf --connect-timeout 2 -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/id &>/dev/null; then
        provider="gcp"
        metadata=$(curl -sf -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/machine-type 2>/dev/null | xargs basename || echo "unknown")
    # DigitalOcean
    elif curl -sf --connect-timeout 2 http://169.254.169.254/metadata/v1/instance-id &>/dev/null; then
        provider="digitalocean"
        metadata=$(curl -sf http://169.254.169.254/metadata/v1/instance/slug 2>/dev/null || echo "unknown")
    # Hetzner
    elif curl -sf --connect-timeout 2 http://169.254.169.254/hetzner/v1/metadata/instance-id &>/dev/null; then
        provider="hetzner"
        metadata=$(curl -sf http://169.254.169.254/hetzner/v1/metadata/instance-type 2>/dev/null || echo "unknown")
    # Vultr
    elif [[ -f /etc/vultr/instance-id ]]; then
        provider="vultr"
        metadata=$(cat /etc/vultr/instance-type 2>/dev/null || echo "unknown")
    # Hostinger
    elif [[ -f /etc/hpanel/metadata ]]; then
        provider="hostinger"
        metadata="vps"
    # Check for common VPS indicators
    elif [[ -d /etc/digitalocean ]] || [[ -f /etc/digitalocean ]]; then
        provider="digitalocean"
        metadata="vps"
    elif dmidecode -s system-manufacturer 2>/dev/null | grep -qi "linode"; then
        provider="linode"
        metadata="vps"
    fi

    PROVIDER="$provider"
    DETECTION_RESULTS["provider"]="OK:$provider ($metadata)"
    log_success "Provider: $provider ${metadata:+($metadata)}"
    return 0
}

# Module 3: Public IP Detection
detect_public_ip() {
    log_info "Detecting public IP address..."

    local ip=""

    # Try multiple services for redundancy
    ip=$(curl -sf --connect-timeout 5 https://api.ipify.org 2>/dev/null) ||
    ip=$(curl -sf --connect-timeout 5 https://ifconfig.me 2>/dev/null) ||
    ip=$(curl -sf --connect-timeout 5 https://icanhazip.com 2>/dev/null) ||
    ip=$(curl -sf --connect-timeout 5 https://ipecho.net/plain 2>/dev/null) ||
    ip=$(curl -sf --connect-timeout 5 https://checkip.amazonaws.com 2>/dev/null)

    if [[ -z "$ip" ]]; then
        DETECTION_RESULTS["public_ip"]="FAIL:$E003"
        return 1
    fi

    # Validate IP format
    if [[ ! "$ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        DETECTION_RESULTS["public_ip"]="FAIL:Invalid IP format: $ip"
        return 1
    fi

    PUBLIC_IP="$ip"
    DETECTION_RESULTS["public_ip"]="OK:$ip"
    log_success "Public IP: $ip"
    return 0
}

# Module 4: NAT/Private IP Detection
detect_nat_private_ip_trap() {
    log_info "Checking for NAT/private IP configuration..."

    # Get all private IPs
    local private_ips
    private_ips=$(ip -4 addr show | grep -oP '(?<=inet\s)10\.[0-9.]+|172\.(1[6-9]|2[0-9]|3[01])\.[0-9.]+|192\.168\.[0-9.]+' | head -1)

    if [[ -n "$private_ips" ]]; then
        PRIVATE_IP="$private_ips"

        # Check if this is behind NAT (public IP differs from interface IPs)
        local has_public_interface
        has_public_interface=$(ip -4 addr show | grep -c "$PUBLIC_IP" || true)

        if [[ "$has_public_interface" -eq 0 ]]; then
            DETECTION_RESULTS["nat"]="WARN:Behind NAT (private: $private_ips, public: $PUBLIC_IP)"
            log_warning "NAT detected: private=$private_ips, public=$PUBLIC_IP"
            log_warning "Ensure your firewall forwards required ports"
        else
            DETECTION_RESULTS["nat"]="OK:Direct connection"
        fi
    else
        DETECTION_RESULTS["nat"]="OK:Direct connection"
    fi

    return 0
}

# Module 5: Docker Mode Detection
detect_docker_mode() {
    log_info "Checking Docker installation..."

    if ! check_command docker; then
        DETECTION_RESULTS["docker"]="WARN:Not installed"
        log_warning "Docker not installed"
        return 0
    fi

    local docker_version
    docker_version=$(docker --version 2>/dev/null | awk '{print $3}' | tr -d ',')
    log_info "Docker version: $docker_version"

    # Check if Docker daemon is running
    if ! docker info &>/dev/null; then
        DETECTION_RESULTS["docker"]="WARN:Docker daemon not running"
        log_warning "Docker daemon not running"
        return 0
    fi

    # Check for rootless Docker
    if [[ -n "${DOCKER_HOST:-}" ]] && [[ "$DOCKER_HOST" == *"/run/user/"* ]]; then
        DETECTION_RESULTS["docker"]="FAIL:$E004"
        return 1
    fi

    # Check Docker socket permissions
    if [[ ! -S /var/run/docker.sock ]]; then
        DETECTION_RESULTS["docker"]="WARN:Docker socket not found"
        return 0
    fi

    DETECTION_RESULTS["docker"]="OK:$docker_version"
    log_success "Docker: $docker_version"
    return 0
}

# Module 6: Firewall Detection
detect_firewall() {
    log_info "Detecting firewall..."

    local firewall="none"

    if check_command ufw; then
        local ufw_status
        ufw_status=$(ufw status 2>/dev/null | head -1)
        if echo "$ufw_status" | grep -q "Status: active"; then
            firewall="ufw (active)"
        elif echo "$ufw_status" | grep -q "Status: inactive"; then
            firewall="ufw (inactive)"
        fi
    elif check_command firewall-cmd; then
        if firewall-cmd --state &>/dev/null; then
            firewall="firewalld (active)"
        else
            firewall="firewalld (inactive)"
        fi
    elif check_command iptables; then
        local rules
        rules=$(iptables -S 2>/dev/null | grep -cv "^-" || echo "0")
        if [[ "$rules" -gt 0 ]]; then
            firewall="iptables ($rules rules)"
        fi
    fi

    DETECTION_RESULTS["firewall"]="OK:$firewall"
    log_success "Firewall: $firewall"
    return 0
}

# Module 7: Resource Detection
detect_resources() {
    log_info "Checking system resources..."

    local failed=false

    # RAM check
    local total_mem
    total_mem=$(free -m | awk '/^Mem:/{print $2}')
    if [[ "$total_mem" -lt "$MIN_RAM_MB" ]]; then
        DETECTION_RESULTS["ram"]="FAIL:$E005 (${total_mem}MB < ${MIN_RAM_MB}MB)"
        failed=true
    else
        DETECTION_RESULTS["ram"]="OK:${total_mem}MB"
        log_success "RAM: ${total_mem}MB"
    fi

    # CPU check
    local cpu_cores
    cpu_cores=$(nproc)
    if [[ "$cpu_cores" -lt "$MIN_CPU_CORES" ]]; then
        DETECTION_RESULTS["cpu"]="FAIL:$E006 (${cpu_cores} < ${MIN_CPU_CORES})"
        failed=true
    else
        DETECTION_RESULTS["cpu"]="OK:${cpu_cores} cores"
        log_success "CPU: ${cpu_cores} cores"
    fi

    # Disk check
    local avail_disk
    avail_disk=$(df -BG / | awk 'NR==2 {print $4}' | tr -d 'G')
    if [[ "$avail_disk" -lt "$MIN_DISK_GB" ]]; then
        DETECTION_RESULTS["disk"]="FAIL:$E007 (${avail_disk}GB < ${MIN_DISK_GB}GB)"
        failed=true
    else
        DETECTION_RESULTS["disk"]="OK:${avail_disk}GB available"
        log_success "Disk: ${avail_disk}GB available"
    fi

    if [[ "$failed" == "true" ]]; then
        return 1
    fi
    return 0
}

# Module 8: Reverse DNS Check
check_reverse_dns() {
    log_info "Checking reverse DNS..."

    if [[ -z "$PUBLIC_IP" ]]; then
        DETECTION_RESULTS["rdns"]="WARN:No public IP"
        return 0
    fi

    local rdns
    rdns=$(dig +short -x "$PUBLIC_IP" @8.8.8.8 2>/dev/null | head -1 | sed 's/\.$//')

    if [[ -n "$rdns" ]]; then
        HOSTNAME_FQDN="$rdns"
        DETECTION_RESULTS["rdns"]="OK:$rdns"
        log_success "Reverse DNS: $rdns"
    else
        DETECTION_RESULTS["rdns"]="WARN:No PTR record"
        log_warning "No reverse DNS (PTR) record found"
        log_warning "Matrix federation may have deliverability issues"
    fi

    return 0
}

# Module 9: Domain vs IP Mode Detection
detect_domain_vs_ip_mode() {
    log_info "Determining deployment mode (domain vs IP)..."

    if [[ "$DOMAIN_MODE" == "true" ]] && [[ -n "$DOMAIN" ]]; then
        # Verify domain resolves to this server
        local domain_ip
        domain_ip=$(dig +short "$DOMAIN" @8.8.8.8 2>/dev/null | head -1)

        if [[ "$domain_ip" != "$PUBLIC_IP" ]]; then
            DETECTION_RESULTS["domain_mode"]="FAIL:$E008 ($DOMAIN resolves to $domain_ip, expected $PUBLIC_IP)"
            return 1
        fi

        DETECTION_RESULTS["domain_mode"]="OK:Domain mode ($DOMAIN)"
        log_success "Domain mode: $DOMAIN"
    else
        # Use IP-only mode
        if [[ -n "$HOSTNAME_FQDN" ]]; then
            DOMAIN="$HOSTNAME_FQDN"
        else
            DOMAIN="$PUBLIC_IP"
        fi
        DETECTION_RESULTS["domain_mode"]="OK:IP mode ($DOMAIN)"
        log_success "IP-only mode: $DOMAIN"
        log_warning "Self-signed certificates will be used (not recommended for production)"
    fi

    return 0
}

# Module 10: Combined Environment Validation
validate_environment() {
    log_info "Validating complete environment..."

    local errors=0
    local warnings=0

    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║              Environment Validation Summary                     ║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    printf "%-25s %-10s %s\n" "Check" "Status" "Details"
    printf "%-25s %-10s %s\n" "─────────────────────" "──────────" "─────────────────────────────────"

    for key in "${!DETECTION_RESULTS[@]}"; do
        local value="${DETECTION_RESULTS[$key]}"
        local status details

        if [[ "$value" == FAIL:* ]]; then
            status="${RED}FAIL${NC}"
            details="${value#FAIL:}"
            ((errors++)) || true
        elif [[ "$value" == WARN:* ]]; then
            status="${YELLOW}WARN${NC}"
            details="${value#WARN:}"
            ((warnings++)) || true
        else
            status="${GREEN}OK${NC}"
            details="${value#OK:}"
        fi

        printf "%-25s %-10s %s\n" "$key" "$status" "$details"
    done

    echo ""
    printf "Total: ${GREEN}%d passed${NC}, ${YELLOW}%d warnings${NC}, ${RED}%d errors${NC}\n" \
        $((${#DETECTION_RESULTS[@]} - errors - warnings)) "$warnings" "$errors"
    echo ""

    if [[ "$errors" -gt 0 ]]; then
        abort "E000" "Environment validation failed with $errors error(s)"
    fi

    if [[ "$warnings" -gt 0 ]]; then
        if [[ "$(prompt_yes_no "Continue with $warnings warning(s)?" "y")" != "y" ]]; then
            abort "E000" "Installation cancelled by user"
        fi
    fi

    return 0
}

# Module 11: Reverse Proxy Installation and Configuration
enforce_reverse_proxy() {
    log_info "Installing and configuring nginx reverse proxy..."

    # Install nginx if needed
    if ! check_command nginx; then
        log_info "Installing nginx..."
        install_packages nginx certbot
    fi

    # Create nginx configuration
    local nginx_conf="${NGINX_SITES_DIR}/armorclaw.conf"

    log_info "Creating nginx configuration..."
    $SUDO tee "$nginx_conf" > /dev/null <<NGINX_CONF
# ArmorClaw Nginx Configuration
# Generated by installer-v4.sh on $(date)
# WARNING: Bridge is bound to localhost ONLY

# Rate limiting zones
limit_req_zone \$binary_remote_addr zone=general:10m rate=10r/s;
limit_req_zone \$binary_remote_addr zone=matrix:10m rate=5r/s;

# Blue-green upstream for bridge
upstream armorclaw_active {
    server 127.0.0.1:${BRIDGE_PORT};        # blue (active)
    server 127.0.0.1:$((BRIDGE_PORT + 1)) backup;  # green (standby)
    keepalive 32;
}

# HTTP server - redirect to HTTPS
server {
    listen 80;
    listen [::]:80;
    server_name ${DOMAIN};

    # Let's Encrypt challenge
    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    # Redirect to HTTPS
    location / {
        return 301 https://\$host\$request_uri;
    }
}

# HTTPS server
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name ${DOMAIN};

    # SSL certificates
$(if [[ "$DOMAIN_MODE" == "true" ]]; then
    echo "    ssl_certificate /etc/letsencrypt/live/${DOMAIN}/fullchain.pem;"
    echo "    ssl_certificate_key /etc/letsencrypt/live/${DOMAIN}/privkey.pem;"
else
    echo "    ssl_certificate /etc/ssl/armorclaw/selfsigned.crt;"
    echo "    ssl_certificate_key /etc/ssl/armorclaw/selfsigned.key;"
fi)

    # SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers 'ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384';
    ssl_prefer_server_ciphers off;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    # Security headers
    add_header Strict-Transport-Security "max-age=63072000" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Health check endpoint
    location /health {
        access_log off;
        return 200 "OK\n";
        add_header Content-Type text/plain;
    }

    # Bridge API - LOCALHOST ONLY!
    # This location is only accessible from the server itself
    location /bridge/ {
        # CRITICAL: Only allow localhost connections
        allow 127.0.0.1;
        deny all;

        limit_req zone=general burst=20 nodelay;

        proxy_pass http://armorclaw_active/;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # Matrix client API
    location /_matrix/ {
        limit_req zone=matrix burst=50 nodelay;

        # CORS headers
        add_header Access-Control-Allow-Origin '*' always;
        add_header Access-Control-Allow-Methods 'GET, POST, PUT, DELETE, OPTIONS' always;
        add_header Access-Control-Allow-Headers 'Authorization, Content-Type' always;

        if (\$request_method = 'OPTIONS') {
            add_header Access-Control-Allow-Origin '*';
            add_header Access-Control-Allow-Methods 'GET, POST, PUT, DELETE, OPTIONS';
            add_header Access-Control-Allow-Headers 'Authorization, Content-Type';
            add_header Content-Length 0;
            return 204;
        }

        proxy_pass http://127.0.0.1:6167;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_buffering off;
    }

    # Matrix federation
    location /_matrix/federation {
        proxy_pass http://127.0.0.1:8448;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    # Well-known endpoints
    location /.well-known/matrix/server {
        default_type application/json;
        add_header Access-Control-Allow-Origin '*' always;
        return 200 '{"m.server": "${DOMAIN}:443"}';
    }

    location /.well-known/matrix/client {
        default_type application/json;
        add_header Access-Control-Allow-Origin '*' always;
        return 200 '{"m.homeserver": {"base_url": "https://${DOMAIN}"}}';
    }

    # Default location
    location / {
        return 404;
    }
}
NGINX_CONF

    # Create SSL directory for self-signed certs if needed
    if [[ "$DOMAIN_MODE" != "true" ]]; then
        $SUDO mkdir -p /etc/ssl/armorclaw
        $SUDO openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
            -keyout /etc/ssl/armorclaw/selfsigned.key \
            -out /etc/ssl/armorclaw/selfsigned.crt \
            -subj "/CN=${DOMAIN}" 2>/dev/null
        log_info "Self-signed certificate generated"
    fi

    # Enable site
    $SUDO ln -sf "$nginx_conf" "${NGINX_ENABLED_DIR}/armorclaw.conf"

    # Remove default site
    $SUDO rm -f "${NGINX_ENABLED_DIR}/default"

    # Test nginx configuration
    log_info "Testing nginx configuration..."
    if ! nginx -t 2>&1; then
        DETECTION_RESULTS["nginx"]="FAIL:$E009"
        abort "$E009" "nginx -t failed"
    fi

    DETECTION_RESULTS["nginx"]="OK:Configured"
    log_success "Nginx configured successfully"

    # Reload nginx
    $SUDO systemctl reload nginx || $SUDO systemctl start nginx

    # Obtain Let's Encrypt certificate if domain mode
    if [[ "$DOMAIN_MODE" == "true" ]]; then
        log_info "Obtaining Let's Encrypt certificate..."
        certbot certonly --nginx --non-interactive --agree-tos --email "admin@${DOMAIN}" -d "$DOMAIN" || {
            log_warning "Let's Encrypt certificate request failed"
            log_warning "Using self-signed certificate temporarily"
        }
    fi

    # Update state file for rollback
    save_state "nginx_config" "created"

    return 0
}

# Module 12: Blue-Green Deployment
deploy_blue_green() {
    log_info "Setting up blue-green deployment..."

    # Create bridge user
    if ! id "$BRIDGE_USER" &>/dev/null; then
        $SUDO useradd -r -s /bin/false -d "$INSTALL_DIR" "$BRIDGE_USER"
        log_success "Created user: $BRIDGE_USER"
    fi

    # Create directories
    mkdir -p "$INSTALL_DIR" "$CONFIG_DIR" "$DATA_DIR" "$RUN_DIR" "$LOG_DIR"
     $SUDO chown -R "$BRIDGE_USER:$BRIDGE_GROUP" "$INSTALL_DIR" "$DATA_DIR" "$RUN_DIR" "$LOG_DIR"
    $SUDO chmod 770 "$RUN_DIR"

    # Download and verify binary
    download_and_verify_binary

    # Create default configuration if not exists
    create_default_config

    # Create blue service
    create_systemd_service "blue" "$BRIDGE_PORT"

    # Create green service (standby)
    create_systemd_service "green" "$((BRIDGE_PORT + 1))"

    # Enable and start blue (active)
    log_info "Starting blue (active) service..."
    $SUDO systemctl daemon-reload
    $SUDO systemctl enable armorclaw-bridge-blue

    if ! $SUDO systemctl start armorclaw-bridge-blue; then
        abort "$E011" "Failed to start blue bridge service"
    fi

    # Update state file
    save_state "systemd_services" "created"
    save_state "active_slot" "blue"

    log_success "Blue-green deployment complete"
    return 0
}

# Create systemd service file
create_systemd_service() {
    local slot="$1"
    local port="$2"
    local service_file="/etc/systemd/system/armorclaw-bridge-${slot}.service"

    log_info "Creating systemd service for $slot slot (port $port)..."

    $SUDO tee "$service_file" > /dev/null <<EOF
[Unit]
Description=ArmorClaw Bridge (${slot^})
Documentation=https://github.com/Gemutly/ArmorClaw
After=network-online.target docker.socket
Wants=network-online.target docker.socket

StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=notify
NotifyAccess=main
User=${BRIDGE_USER}
Group=${BRIDGE_GROUP}

# Environment
Environment="ARMORCLAW_CONFIG=${CONFIG_DIR}/config.toml"
Environment="ARMORCLAW_PORT=${port}"

# Execution
ExecStart=${INSTALL_DIR}/armorclaw-bridge --config ${CONFIG_DIR}/config.toml --port ${port}
ExecReload=/bin/kill -HUP \$MAINPID

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectControlGroups=true
RestrictRealtime=true
RestrictSUIDSGID=true
LockPersonality=true

RuntimeDirectory=armorclaw
RuntimeDirectoryMode=0755

# Resource limits
MemoryMax=512M
CPUQuota=50%
LimitNOFILE=65536

# Allowed paths
ReadWritePaths=${DATA_DIR}

# Process management
Restart=always
RestartSec=5
# Bridge sends WATCHDOG=1 every 30s. If stuck for 60s, systemd restarts it.
WatchdogSec=60
TimeoutStopSec=30s

StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    log_success "Created: $service_file"
}

# Module 13: Smoke Test RPC Health
smoke_test_rpc_health() {
    log_info "Running RPC health check..."

    local max_attempts=30
    local attempt=1

    while [[ $attempt -le $max_attempts ]]; do
        # Check socket exists
        if [[ ! -S "$BRIDGE_SOCKET" ]]; then
            log_info "Waiting for socket... (attempt $attempt/$max_attempts)"
            sleep 1
            ((attempt++)) || true
            continue
        fi

        # Real JSON-RPC health check (not naive curl)
        local response
        response=$(echo '{"jsonrpc":"2.0","id":1,"method":"health"}' | \
            socat - UNIX-CONNECT:"$BRIDGE_SOCKET" 2>/dev/null) || true

        if echo "$response" | grep -q '"status".*"healthy"'; then
            DETECTION_RESULTS["health"]="OK:Bridge healthy"
            log_success "Bridge health check passed"
            return 0
        fi

        log_info "Waiting for healthy bridge... (attempt $attempt/$max_attempts)"
        sleep 1
        ((attempt++)) || true
    done

    DETECTION_RESULTS["health"]="FAIL:$E010"
    abort "$E010" "Bridge did not become healthy within ${max_attempts} seconds"
}

# =============================================================================
# Section 5: Binary Download and Verification
# =============================================================================

download_binary() {
    if [[ -z "${BIN_ARCH:-}" ]]; then
        detect_arch
    fi
    if [[ -z "${BIN_ARCH:-}" ]]; then
        return 1
    fi

    local version="${VERSION:-latest}"
    local bin_name="armorclaw-bridge-$BIN_ARCH"
    local base_url="${RELEASE_BASE_URL}/$version"
    
    # Use latest release if VERSION is main or latest
    if [[ "$version" == "main" || "$version" == "latest" ]]; then
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
        log_error "Failed to download binary from $base_url/$bin_name"
        rm -rf "$tmp_dir"
        return 1
    fi
    
    if ! retry $curl_base "$base_url/checksums.txt" -o "$checksum_path"; then
        log_error "Failed to download checksums"
        rm -rf "$tmp_dir"
        return 1
    fi
    
    if ! retry $curl_base "$base_url/checksums.txt.sig" -o "$sig_path"; then
        log_error "Failed to download signature"
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
    $SUDO mkdir -p "${INSTALL_DIR}"
    $SUDO install -m 755 "$bin_path" "${INSTALL_DIR}/armorclaw-bridge"
    $SUDO ln -sf "${INSTALL_DIR}/armorclaw-bridge" /usr/local/bin/armorclaw-bridge
    
    rm -rf "$tmp_dir"
    return 0
}

download_and_verify_binary() {
    log_info "Downloading and verifying ArmorClaw bridge binary..."

    if download_binary; then
        log_success "Bridge installed via binary distribution"
    else
        abort "E000" "Binary installation failed"
    fi

    # Update state
    save_state "binary_install" "created"
}

# Create default configuration file
create_default_config() {
    local config_file="${CONFIG_DIR}/config.toml"

    # Ensure armorclaw user exists
    id -u "$BRIDGE_USER" >/dev/null 2>&1 || $SUDO useradd -r -s /bin/false -d "$INSTALL_DIR" "$BRIDGE_USER"

    # Create data directory
    $SUDO mkdir -p "$DATA_DIR"
    chown "$BRIDGE_USER:$BRIDGE_GROUP" "$DATA_DIR"
    $SUDO chmod 700 "$DATA_DIR"

    if [[ -f "$config_file" ]]; then
        log_info "Configuration file already exists: $config_file"
        # Ensure error store path to config (idempotent)
        ensure_error_store_config
        return 0
    fi

    log_info "Creating default configuration..."

    # Create minimal working config
    $SUDO tee "$config_file" > /dev/null <<TOML_CONFIG
# ArmorClaw Bridge Configuration
# Generated by installer-v4.sh
# Full example: https://github.com/Gemutly/ArmorClaw/blob/main/bridge/config.example.toml

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
format = "json"
output = "stdout"

[webrtc]
enabled = false

[webrtc.signaling]
enabled = false
listen_address = "127.0.0.1:8443"
path = "/webrtc"
tls_cert = ""
tls_key = ""

[voice]
enabled = false

[eventbus]
websocket_enabled = false

[notifications]
enabled = false
admin_room_id = ""
TOML_CONFIG

    # Ensure error store path to config (idempotent)
    ensure_error_store_config

    # Set ownership and permissions
    $SUDO chown "$BRIDGE_USER:$BRIDGE_GROUP" "$config_file"
    $SUDO chmod 640 "$config_file"

    # Config sanity check
    grep -q "store_path" "$config_file" || echo "Warning: errors.store_path not configured"

    save_state "config_file" "created"
    log_success "Configuration created: $config_file"
    log_warning "IMPORTANT: Edit $config_file to add your Matrix credentials and API keys"
}

# =============================================================================
# Section 6: Rollback and Recovery
# =============================================================================

save_state() {
    local key="$1"
    local value="$2"

    $SUDO mkdir -p "$(dirname "$STATE_FILE")"

    # Initialize state file if needed
    if [[ ! -f "$STATE_FILE" ]]; then
        echo '{"version":"1.0","steps":[]}' > "$STATE_FILE"
    fi

    # Update state (using simple text append for bash compatibility)
    local tmp_file
    tmp_file=$(mktemp)
    if command -v jq &>/dev/null; then
        jq ".steps += [{\"$key\": \"$value\", \"timestamp\": \"$(date -Iseconds)\"}]" "$STATE_FILE" > "$tmp_file"
        mv "$tmp_file" "$STATE_FILE"
    else
        # Fallback without jq - use simple line-based format
        echo "${key}=${value} $(date -Iseconds)" >> "$STATE_FILE"
    fi
}

perform_rollback() {
    log_warning "Starting rollback..."

    if [[ ! -f "$STATE_FILE" ]]; then
        log_error "No state file found, cannot rollback"
        return 1
    fi

    # Stop services
    systemctl stop armorclaw-bridge-blue 2>/dev/null || true
    systemctl stop armorclaw-bridge-green 2>/dev/null || true

    # Disable and remove services
    systemctl disable armorclaw-bridge-blue 2>/dev/null || true
    systemctl disable armorclaw-bridge-green 2>/dev/null || true
    rm -f /etc/systemd/system/armorclaw-bridge-*.service
    $SUDO systemctl daemon-reload

    # Remove nginx config
    rm -f "${NGINX_ENABLED_DIR}/armorclaw.conf" "${NGINX_SITES_DIR}/armorclaw.conf"
    $SUDO systemctl reload nginx || true

    # Remove binary
    rm -f "${INSTALL_DIR}/armorclaw-bridge" /usr/local/bin/armorclaw-bridge

    # Remove directories (optional, keep data)
    local remove_dirs="n"
    if [[ "$NON_INTERACTIVE" == "true" ]]; then
        remove_dirs="n"  # Safe default in non-interactive mode
        log_info "Non-interactive mode: preserving config and data directories"
    else
        read -p "Remove configuration and data directories? [y/N] " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            remove_dirs="y"
        fi
    fi

    if [[ "$remove_dirs" == "y" ]]; then
        rm -rf "$CONFIG_DIR" "$DATA_DIR" "$RUN_DIR" "$LOG_DIR"
        userdel "$BRIDGE_USER" 2>/dev/null || true
    fi

    rm -f "$STATE_FILE"

    log_success "Rollback complete"
}

# =============================================================================
# Section 7: Telemetry
# =============================================================================

send_telemetry() {
    if [[ "$TELEMETRY" != "true" ]]; then
        return 0
    fi

    log_info "Sending anonymous telemetry..."

    local telemetry_data
    telemetry_data=$(cat <<EOF
{
    "event": "install",
    "version": "$SCRIPT_VERSION",
    "provider": "$PROVIDER",
    "os": "$(cat /etc/os-release 2>/dev/null | grep '^ID=' | cut -d= -f2 || echo 'unknown')",
    "domain_mode": $DOMAIN_MODE,
    "timestamp": "$(date -Iseconds)"
}
EOF
)

    curl -sf -X POST \
        -H "Content-Type: application/json" \
        -d "$telemetry_data" \
        "https://telemetry.armorclaw.com/install" 2>/dev/null || true

    log_success "Telemetry sent (thank you!)"
}

# =============================================================================
# Section 8: Main Entry Point
# =============================================================================

print_banner() {
    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC}         ${BOLD}ArmorClaw Production Installer v${SCRIPT_VERSION}${NC}               ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}                                                                ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}  ${GREEN}Secure${NC} • ${YELLOW}Deterministic${NC} • ${BLUE}Hardened${NC}                        ${CYAN}║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

print_summary() {
    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║${NC}              ${BOLD}Installation Complete${NC}                              ${GREEN}║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${BOLD}Installation Details:${NC}"
    echo "  • Domain:        $DOMAIN"
    echo "  • Public IP:     $PUBLIC_IP"
    echo "  • Bridge Port:   $BRIDGE_PORT (localhost only)"
    echo "  • Socket:        $BRIDGE_SOCKET"
    echo "  • Config:        $CONFIG_DIR/config.toml"
    echo ""
    echo -e "${BOLD}Service Commands:${NC}"
    echo "  • Status:        systemctl status armorclaw-bridge-blue"
    echo "  • Logs:          journalctl -u armorclaw-bridge-blue -f"
    echo "  • Restart:       systemctl restart armorclaw-bridge-blue"
    echo ""
    echo -e "${BOLD}Next Steps:${NC}"
    echo "  1. Add API keys:    armorclaw-bridge add-key --provider openai"
    echo "  2. Configure Matrix: Edit $CONFIG_DIR/config.toml"
    echo "  3. Start agent:     armorclaw-bridge start --key <key-id>"
    echo ""
    echo -e "${GREEN}Security Notice:${NC}"
    echo "  • Bridge is bound to localhost ONLY"
    echo "  • Running as non-root user: $BRIDGE_USER"
    echo "  • Socket permissions: 0660"
    echo ""
}

main() {
    # Initialize logging
    mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null || LOG_FILE="/tmp/armorclaw-installer.log"

    # Parse arguments
    parse_arguments "$@"

    # Print banner
    print_banner

    # Check for rollback mode
    if [[ "$ROLLBACK_MODE" == "true" ]]; then
        perform_rollback
        exit 0
    fi

    # Run detection modules (abort on failure)
    detect_system_environment || abort "${DETECTION_RESULTS[system_environment]#FAIL:}" "System environment check failed"
    detect_provider
    detect_public_ip || abort "${DETECTION_RESULTS[public_ip]#FAIL:}" "Public IP detection failed"
    detect_nat_private_ip_trap
    detect_docker_mode || abort "${DETECTION_RESULTS[docker]#FAIL:}" "Docker check failed"
    detect_firewall
    detect_resources || abort "E005" "Resource requirements not met"
    check_reverse_dns
    detect_domain_vs_ip_mode || abort "${DETECTION_RESULTS[domain_mode]#FAIL:}" "Domain check failed"

    # Validate environment
    validate_environment

    # Dry run check
    if [[ "$DRY_RUN" == "true" ]]; then
        log_success "Dry run complete - no changes made"
        exit 0
    fi

    # Confirm installation
    if [[ "$NON_INTERACTIVE" != "true" ]]; then
        if [[ "$(prompt_yes_no "Proceed with installation?" "n")" != "y" ]]; then
            log_info "Installation cancelled"
            exit 0
        fi
    fi

    # Telemetry consent
    if [[ "$TELEMETRY" != "true" ]] && [[ "$NON_INTERACTIVE" != "true" ]]; then
        if [[ "$(prompt_yes_no "Enable anonymous telemetry to help improve ArmorClaw?" "n")" == "y" ]]; then
            TELEMETRY="true"
        fi
    fi

    # Install required packages
    install_packages curl socat jq

    # Run installation modules
    enforce_reverse_proxy
    deploy_blue_green

    # Verify installation
    smoke_test_rpc_health

    # Send telemetry
    send_telemetry

    # Print summary
    print_summary

    # Play completion sound (if available)
    if command -v afplay &>/dev/null; then
        afplay /System/Library/Sounds/Glass.aiff 2>/dev/null || true
    elif command -v paplay &>/dev/null; then
        paplay /usr/share/sounds/freedesktop/stereo/complete.oga 2>/dev/null || true
    fi

    exit 0
}

# Run main function
main "$@"
