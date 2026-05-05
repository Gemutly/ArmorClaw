#!/bin/bash
# ArmorClaw Production Hardening
# Post-setup security hardening for production deployments
# Version: 0.3.1
#
# Usage: sudo ./deploy/armorclaw-harden.sh [--non-interactive]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# Global variables
CONFIG_DIR="/etc/armorclaw"
CONFIG_FILE="$CONFIG_DIR/config.toml"
LOG_DIR="/var/log/armorclaw"

# Non-interactive mode
NON_INTERACTIVE=false
if [[ "$1" == "--non-interactive" || "$1" == "-y" ]]; then
    NON_INTERACTIVE=true
fi

# Track what was configured
FIREWALL_CONFIGURED=false
SSH_CONFIGURED=false
FAIL2BAN_CONFIGURED=false
UPDATES_CONFIGURED=false
LOGGING_CONFIGURED=false

#=============================================================================
# Helper Functions
#=============================================================================

print_header() {
    clear 2>/dev/null || true
    echo -e "${CYAN}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC}            ${BOLD}ArmorClaw Production Hardening${NC}                   ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}            ${BOLD}Security Configuration${NC}                            ${CYAN}║${NC}"
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

fail() {
    print_error "$1"
    exit 1
}

prompt_yes_no() {
    if $NON_INTERACTIVE; then
        return 0
    fi

    local prompt="$1"
    local default="${2:-n}"

    echo ""
    echo -ne "  ${CYAN}$prompt [${default^^}/$(echo $default | tr 'yn' 'ny')]${NC}: "
    read -r response
    response=${response:-$default}

    case "$response" in
        [Yy]|[Yy][Ee][Ss]) return 0 ;;
        [Nn]|[Nn][Oo]) return 1 ;;
    esac

    [[ "$default" == "y" ]]
}

#=============================================================================
# Check Prerequisites
#=============================================================================

check_prerequisites() {
    print_step "Checking prerequisites..."

    # Check root
    if [[ $EUID -ne 0 ]]; then
        fail "This script must be run as root (use sudo)"
    fi
    print_success "Running as root"

    # Check OS
    if [[ -f /etc/os-release ]]; then
        source /etc/os-release
        print_success "OS: $PRETTY_NAME"

        if [[ "$ID" != "ubuntu" ]] && [[ "$ID" != "debian" ]]; then
            print_warning "This script is designed for Ubuntu/Debian"
            if ! prompt_yes_no "Continue anyway?" "n"; then
                exit 1
            fi
        fi
    else
        fail "Cannot detect OS"
    fi
}

#=============================================================================
# 1. Firewall Configuration (UFW)
#=============================================================================

configure_firewall() {
    print_step "Firewall Configuration (UFW)"

    cat <<'EOF'
  UFW (Uncomplicated Firewall) provides:
  • Deny-all default policy
  • Simple port management
  • Tailscale VPN auto-detection
  • Rate limiting for SSH

  Recommended ports:
  • 22/tcp   - SSH (rate limited)
  • 80/tcp   - HTTP (for Let's Encrypt)
  • 443/tcp  - HTTPS
  • 41641/udp - Tailscale (if installed)
EOF

    echo ""

    if ! prompt_yes_no "Configure UFW firewall?" "y"; then
        print_info "Skipping firewall configuration"
        return 0
    fi

    # Install UFW if needed
    if ! command -v ufw &>/dev/null; then
        print_info "Installing UFW..."
        apt-get update -qq
        apt-get install -y -qq ufw
    fi

    # Reset to clean state (with confirmation)
    print_info "Setting default policies..."
    ufw default deny incoming
    ufw default allow outgoing

    # Allow SSH first (critical!)
    print_info "Allowing SSH (rate limited)..."
    ufw limit 22/tcp comment 'SSH rate limited'

    # Allow HTTP/HTTPS
    print_info "Allowing web ports..."
    ufw allow 80/tcp comment 'HTTP'
    ufw allow 443/tcp comment 'HTTPS'

    # Check for Tailscale
    if command -v tailscale &>/dev/null; then
        print_info "Detected Tailscale - allowing VPN port..."
        ufw allow 41641/udp comment 'Tailscale VPN'
    fi

    # Allow Matrix federation port if configured
    if grep -q 'enabled = true' "$CONFIG_FILE" 2>/dev/null; then
        if prompt_yes_no "Allow Matrix federation port (8448/tcp)?" "y"; then
            ufw allow 8448/tcp comment 'Matrix federation'
        fi
    fi

    # Enable logging
    print_info "Enabling firewall logging..."
    ufw logging on

    # Show status
    echo ""
    print_info "Firewall rules preview:"
    ufw status numbered | head -20

    echo ""
    print_warning "${BOLD}IMPORTANT:${NC} You are about to enable the firewall!"
    print_warning "Make sure you have an active SSH connection."

    if prompt_yes_no "Enable UFW firewall now?" "y"; then
        # Enable with echo "y" to avoid interactive prompt
        echo "y" | ufw enable
        print_success "UFW firewall enabled"
        FIREWALL_CONFIGURED=true
    else
        print_warning "Firewall not enabled"
        print_info "Enable later with: ufw enable"
    fi
}

#=============================================================================
# 2. SSH Hardening
#=============================================================================

configure_ssh() {
    print_step "SSH Hardening"

    cat <<'EOF'
  SSH hardening includes:
  • Key-only authentication (password disabled)
  • Root login disabled
  • X11 forwarding disabled
  • Reduced login grace time
  • Stronger ciphers

  WARNING: Ensure you have SSH keys configured before enabling!
EOF

    echo ""

    # Check for SSH keys
    local has_keys=false
    if [[ -d /root/.ssh ]] && [[ -n "$(ls -A /root/.ssh/*.pub 2>/dev/null)" ]]; then
        has_keys=true
    fi

    if [[ -d /home ]] && ls /home/*/.ssh/*.pub &>/dev/null; then
        has_keys=true
    fi

    if ! $has_keys; then
        print_warning "No SSH keys detected in ~/.ssh/"
        print_info "Configure SSH keys first: ssh-copy-id user@host"
        echo ""

        if ! prompt_yes_no "Continue anyway (may lock you out!)?" "n"; then
            print_info "Skipping SSH hardening"
            return 0
        fi
    fi

    if ! prompt_yes_no "Harden SSH configuration?" "y"; then
        print_info "Skipping SSH hardening"
        return 0
    fi

    local sshd_config="/etc/ssh/sshd_config"

    # Backup
    cp "$sshd_config" "${sshd_config}.bak"
    print_info "Backup saved to ${sshd_config}.bak"

    # Apply hardening
    print_info "Applying SSH hardening..."

    # Function to set or add SSH option
    set_ssh_option() {
        local key="$1"
        local value="$2"

        if grep -q "^[[:space:]]*${key}" "$sshd_config"; then
            sed -i "s/^[[:space:]]*${key}.*/${key} ${value}/" "$sshd_config"
        else
            echo "${key} ${value}" >> "$sshd_config"
        fi
    }

    # Disable password authentication
    set_ssh_option "PasswordAuthentication" "no"
    set_ssh_option "PubkeyAuthentication" "yes"

    # Disable root login
    set_ssh_option "PermitRootLogin" "prohibit-password"

    # Disable X11 forwarding
    set_ssh_option "X11Forwarding" "no"

    # Reduce grace time
    set_ssh_option "LoginGraceTime" "30"

    # Limit authentication attempts
    set_ssh_option "MaxAuthTries" "3"

    # Use stronger ciphers
    set_ssh_option "KexAlgorithms" "diffie-hellman-group-exchange-sha256"
    set_ssh_option "Ciphers" "aes256-gcm@openssh.com,chacha20-poly1305@openssh.com"
    set_ssh_option "MACs" "hmac-sha2-512,hmac-sha2-256"

    # Validate config
    if sshd -t; then
        print_success "SSH configuration validated"

        # Restart SSH
        if prompt_yes_no "Restart SSH service now?" "y"; then
            systemctl restart sshd || systemctl restart ssh
            print_success "SSH service restarted"
            SSH_CONFIGURED=true
        fi
    else
        print_error "SSH configuration invalid - restoring backup"
        mv "${sshd_config}.bak" "$sshd_config"
    fi
}

#=============================================================================
# 3. Fail2Ban Setup
#=============================================================================

configure_fail2ban() {
    print_step "Fail2Ban Setup"

    cat <<'EOF'
  Fail2Ban protects against brute-force attacks:
  • Monitors log files for failed login attempts
  • Bans suspicious IPs automatically
  • Customizable ban duration and thresholds

  Default protection for:
  • SSH (sshd)
  • Matrix (if enabled)
EOF

    echo ""

    if ! prompt_yes_no "Install and configure Fail2Ban?" "y"; then
        print_info "Skipping Fail2Ban setup"
        return 0
    fi

    # Install fail2ban
    if ! command -v fail2ban-server &>/dev/null; then
        print_info "Installing Fail2Ban..."
        apt-get update -qq
        apt-get install -y -qq fail2ban
    fi

    # Create jail.local (overrides jail.d)
    local jail_file="/etc/fail2ban/jail.local"

    cat > "$jail_file" <<'EOF'
[DEFAULT]
# Ban duration (10 minutes)
bantime = 10m

# Search window
findtime = 10m

# Max attempts before ban
maxretry = 5

# Ban action
banaction = iptables-multiport

# Email for alerts (optional)
# destemail = your@email.com
# sendername = Fail2Ban
# mta = sendmail

[sshd]
enabled = true
port = ssh
filter = sshd
logpath = /var/log/auth.log
maxretry = 3
bantime = 1h
EOF

    # Add Matrix jail if enabled
    if grep -q 'enabled = true' "$CONFIG_FILE" 2>/dev/null; then
        cat >> "$jail_file" <<'EOF'

[matrix]
enabled = true
port = 443,8448
filter = matrix
logpath = /var/log/armorclaw/bridge.log
maxretry = 5
bantime = 1h
EOF

        # Create Matrix filter
        cat > /etc/fail2ban/filter.d/matrix.conf <<'EOF'
[Definition]
failregex = ^.*authentication failed.*from <HOST>.*$
            ^.*invalid signature.*from <HOST>.*$
ignoreregex =
EOF
    fi

    # Enable and start fail2ban
    systemctl enable fail2ban
    systemctl restart fail2ban

    print_success "Fail2Ban configured and started"
    FAIL2BAN_CONFIGURED=true

    # Show status
    print_info "Fail2Ban status:"
    fail2ban-client status 2>/dev/null || true
}

#=============================================================================
# 4. Automatic Security Updates
#=============================================================================

configure_updates() {
    print_step "Automatic Security Updates"

    cat <<'EOF'
  Unattended-upgrades provides:
  • Automatic security patch installation
  • Configurable update schedule
  • Optional reboot for kernel updates

  Recommended for production servers.
EOF

    echo ""

    if ! prompt_yes_no "Configure automatic security updates?" "y"; then
        print_info "Skipping automatic updates"
        return 0
    fi

    # Install unattended-upgrades
    if ! command -v unattended-upgrade &>/dev/null; then
        print_info "Installing unattended-upgrades..."
        apt-get update -qq
        apt-get install -y -qq unattended-upgrades apt-listchanges
    fi

    # Configure
    local config_file="/etc/apt/apt.conf.d/20auto-upgrades"

    cat > "$config_file" <<'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::Download-Upgradeable-Packages "1";
APT::Periodic::AutocleanInterval "7";
EOF

    # Configure unattended-upgrades
    local upgrade_config="/etc/apt/apt.conf.d/50unattended-upgrades"

    # Enable security updates
    sed -i 's|//.*"${distro_id}:${distro_codename}-security";|"${distro_id}:${distro_codename}-security";|' "$upgrade_config"

    # Ask about automatic reboot
    if prompt_yes_no "Enable automatic reboot for kernel updates?" "n"; then
        sed -i 's|//Unattended-Upgrade::Automatic-Reboot "false";|Unattended-Upgrade::Automatic-Reboot "true";|' "$upgrade_config"
        sed -i 's|//Unattended-Upgrade::Automatic-Reboot-Time "02:00";|Unattended-Upgrade::Automatic-Reboot-Time "02:00";|' "$upgrade_config"
    fi

    print_success "Automatic security updates configured"
    UPDATES_CONFIGURED=true
}

#=============================================================================
# 5. Production Logging
#=============================================================================

configure_logging() {
    print_step "Production Logging"

    cat <<'EOF'
  Production logging provides:
  • JSON structured logs for aggregation
  • Persistent file storage
  • Log rotation
  • Security event tracking

  Compatible with: ELK Stack, Splunk, Grafana Loki, etc.
EOF

    echo ""

    if ! prompt_yes_no "Configure production logging?" "y"; then
        print_info "Skipping logging configuration"
        return 0
    fi

    # Create log directory
    mkdir -p "$LOG_DIR"
    chmod 750 "$LOG_DIR"
    chown armorclaw:armorclaw "$LOG_DIR" 2>/dev/null || true

    # Update bridge config
    if [[ -f "$CONFIG_FILE" ]]; then
        print_info "Updating bridge logging configuration..."

        # Backup
        cp "$CONFIG_FILE" "$CONFIG_FILE.bak"

        # Update logging section
        if grep -q '^\[logging\]' "$CONFIG_FILE"; then
            sed -i 's/^format = "text"/format = "json"/' "$CONFIG_FILE"
            sed -i 's/^output = "stdout"/output = "file"/' "$CONFIG_FILE"

            # Set log file if not present
            if ! grep -q '^file = ' "$CONFIG_FILE"; then
                sed -i '/^\[logging\]/a file = "'$LOG_DIR'/bridge.log"' "$CONFIG_FILE"
            fi
        else
            # Add logging section
            cat >> "$CONFIG_FILE" <<EOF

[logging]
level = "info"
format = "json"
output = "file"
file = "$LOG_DIR/bridge.log"
EOF
        fi

        print_success "Logging configuration updated"
    fi

    # Configure logrotate
    local logrotate_conf="/etc/logrotate.d/armorclaw"

    cat > "$logrotate_conf" <<EOF
$log_dir/*.log {
    daily
    missingok
    rotate 14
    compress
    delaycompress
    notifempty
    create 0640 armorclaw armorclaw
    sharedscripts
    postrotate
        systemctl reload armorclaw-bridge > /dev/null 2>&1 || true
    endscript
}
EOF

    print_success "Log rotation configured"
    LOGGING_CONFIGURED=true
}

#=============================================================================
# 6. Monitoring Setup (Optional)
#=============================================================================

configure_monitoring() {
    print_step "Monitoring Setup (Optional)"

    cat <<'EOF'
  Monitoring is handled by systemd watchdog:
  • Bridge sends WATCHDOG=1 every 30s via sd_notify
  • systemd restarts the bridge if stuck for 60s (WatchdogSec=60)
  • Status visible via: systemctl status armorclaw-bridge
  • Optional: Prometheus node exporter

  For advanced monitoring, consider:
  • Prometheus + Grafana
  • Datadog / New Relic
  • Custom health endpoints
EOF

    echo ""

    if ! prompt_yes_no "Configure basic monitoring?" "n"; then
        print_info "Skipping monitoring setup"
        return 0
    fi

    # Note: Health monitoring is now handled by systemd watchdog.
    # The bridge sends WATCHDOG=1 every 30s via sd_notify (Type=notify service).
    # If the bridge is stuck for 60s, systemd automatically restarts it.
    # The cron-based health check is no longer needed.
    print_info "Health monitoring: systemd watchdog (WatchdogSec=60, bridge pings every 30s)"

    # Ask about node exporter
    if prompt_yes_no "Install Prometheus node exporter?" "n"; then
        apt-get install -y -qq prometheus-node-exporter 2>/dev/null || {
            print_warning "prometheus-node-exporter not in repos"
            print_info "Install manually if needed"
        }
    fi
}

#=============================================================================
# Summary
#=============================================================================

print_summary() {
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║${NC}                 ${BOLD}Hardening Complete!${NC}                               ${GREEN}║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    echo -e "${BOLD}Configuration Summary:${NC}"
    echo ""

    if $FIREWALL_CONFIGURED; then
        echo -e "  ${GREEN}✓${NC} UFW Firewall: Enabled"
        ufw status | head -3
    else
        echo -e "  ${YELLOW}○${NC} UFW Firewall: Skipped"
    fi
    echo ""

    if $SSH_CONFIGURED; then
        echo -e "  ${GREEN}✓${NC} SSH Hardening: Key-only, no root"
    else
        echo -e "  ${YELLOW}○${NC} SSH Hardening: Skipped"
    fi
    echo ""

    if $FAIL2BAN_CONFIGURED; then
        echo -e "  ${GREEN}✓${NC} Fail2Ban: Active"
    else
        echo -e "  ${YELLOW}○${NC} Fail2Ban: Skipped"
    fi
    echo ""

    if $UPDATES_CONFIGURED; then
        echo -e "  ${GREEN}✓${NC} Auto Updates: Security only"
    else
        echo -e "  ${YELLOW}○${NC} Auto Updates: Skipped"
    fi
    echo ""

    if $LOGGING_CONFIGURED; then
        echo -e "  ${GREEN}✓${NC} Logging: JSON format, rotating"
    else
        echo -e "  ${YELLOW}○${NC} Logging: Skipped"
    fi

    echo ""
    echo -e "${BOLD}Important Commands:${NC}"
    echo ""
    echo -e "  Firewall status:    ${CYAN}ufw status verbose${NC}"
    echo -e "  Fail2Ban status:    ${CYAN}fail2ban-client status${NC}"
    echo -e "  View logs:          ${CYAN}tail -f $LOG_DIR/bridge.log${NC}"
    echo -e "  Check updates:      ${CYAN}unattended-upgrade --dry-run${NC}"
    echo ""

    if $LOGGING_CONFIGURED; then
        echo -e "${BOLD}Restart bridge to apply logging changes:${NC}"
        echo -e "  ${CYAN}systemctl restart armorclaw-bridge${NC}"
        echo ""
    fi

    echo -e "${BOLD}Security Checklist:${NC}"
    echo ""
    echo "  [ ] Verify SSH key login works before closing session"
    echo "  [ ] Test firewall doesn't block required services"
    echo "  [ ] Review Fail2Ban logs: journalctl -u fail2ban"
    echo "  [ ] Schedule regular security audits"
    echo ""
}

#=============================================================================
# Main
#=============================================================================

main() {
    print_header
    check_prerequisites
    configure_firewall
    configure_ssh
    configure_fail2ban
    configure_updates
    configure_logging
    configure_monitoring
    print_summary
}

main "$@"
