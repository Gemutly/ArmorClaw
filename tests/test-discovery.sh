#!/bin/bash
# Test mDNS discovery of ArmorClaw bridge
#
# This script tests the mDNS discovery functionality for ArmorClaw.
# Run this on the same network where clients will discover the bridge.
#
# Usage:
#   ./tests/test-discovery.sh
#
# Requirements:
#   - avahi-tools (for avahi-browse)
#   - socat (for Unix socket testing)
#   - jq (for JSON formatting)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/transport.sh"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================"
echo "ArmorClaw mDNS Discovery Test"
echo "========================================"
echo ""

PASS=0
FAIL=0
WARN=0

pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((PASS++))
}

fail() {
    echo -e "${RED}✗${NC} $1"
    ((FAIL++))
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
    ((WARN++))
}

# 1. Check if mDNS port is open
echo "1. Checking UDP port 5353 (mDNS)..."
if ss -ulnp 2>/dev/null | grep -q 5353 || netstat -ulnp 2>/dev/null | grep -q 5353; then
    pass "mDNS port 5353 is open"
else
    # Check if avahi daemon is running
    if pgrep -x avahi-daemon > /dev/null || systemctl is-active --quiet avahi-daemon 2>/dev/null; then
        pass "Avahi daemon is running (mDNS port will be opened on demand)"
    else
        warn "mDNS port 5353 not detected - ensure firewall allows UDP 5353"
        echo "   Fix: sudo ufw allow 5353/udp"
    fi
fi
echo ""

# 2. Check if bridge is running
echo "2. Checking if ArmorClaw Bridge is running..."
if pgrep -f "armorclaw-bridge" > /dev/null; then
    pass "ArmorClaw Bridge process is running"
else
    warn "ArmorClaw Bridge not running - start with: ./bridge/build/armorclaw-bridge"
fi
echo ""

# 3. Check bridge socket
echo "3. Checking bridge Unix socket..."
SOCKET_PATH="/run/armorclaw/bridge.sock"
if [ -S "$SOCKET_PATH" ]; then
    pass "Bridge socket exists at $SOCKET_PATH"

    # Test RPC connection via transport
    echo "   Testing RPC connection..."
    detect_transport
    if [[ "$TRANSPORT_MODE" != "none" ]]; then
        RESPONSE=$(rpc_call "bridge.status" '{}' 3 2>/dev/null || echo '{"error":"connection failed"}')

        if echo "$RESPONSE" | grep -q '"result"'; then
            pass "RPC connection successful"
            if command -v jq &> /dev/null; then
                echo "   Response:"
                echo "$RESPONSE" | jq -C . | sed 's/^/   /'
            fi
        else
            fail "RPC connection failed"
        fi
    else
        warn "socat not installed - cannot test RPC"
    fi
else
    warn "Bridge socket not found at $SOCKET_PATH"
fi
echo ""

# 4. Test mDNS discovery
echo "4. Testing mDNS service discovery..."
if command -v avahi-browse &> /dev/null; then
    echo "   Browsing for _armorclaw._tcp services (5s timeout)..."

    DISCOVERY_RESULT=$(timeout 5 avahi-browse -t -r _armorclaw._tcp 2>&1 || true)

    if echo "$DISCOVERY_RESULT" | grep -q "armorclaw"; then
        pass "Found ArmorClaw service via mDNS"
        echo "$DISCOVERY_RESULT" | grep -A 10 "armorclaw" | sed 's/^/   /'

        # Check for TXT records
        if echo "$DISCOVERY_RESULT" | grep -q "matrix_homeserver"; then
            pass "Matrix homeserver URL found in TXT records"
        else
            warn "Matrix homeserver URL not in TXT records - clients will need QR setup"
        fi
    else
        warn "No ArmorClaw services found via mDNS"
        echo "   Discovery output:"
        echo "$DISCOVERY_RESULT" | sed 's/^/   /'
    fi
else
    warn "avahi-browse not installed"
    echo "   Install with: sudo apt install avahi-tools"
    echo ""
    echo "   Alternative test using dns-sd (macOS):"
    echo "   dns-sd -B _armorclaw._tcp"
fi
echo ""

# 5. Check firewall status
echo "5. Checking firewall configuration..."
if command -v ufw &> /dev/null; then
    UFW_STATUS=$(sudo ufw status 2>/dev/null || echo "Status: inactive")
    if echo "$UFW_STATUS" | grep -q "active"; then
        if echo "$UFW_STATUS" | grep -q "5353"; then
            pass "Firewall allows mDNS (5353/udp)"
        else
            warn "Firewall active but mDNS port not explicitly allowed"
            echo "   Fix: sudo ufw allow 5353/udp"
        fi
    else
        pass "Firewall is inactive (mDNS should work)"
    fi
elif command -v firewall-cmd &> /dev/null; then
    if sudo firewall-cmd --list-ports 2>/dev/null | grep -q "5353"; then
        pass "Firewall allows mDNS (5353/udp)"
    else
        warn "Firewall may block mDNS"
        echo "   Fix: sudo firewall-cmd --add-port=5353/udp --permanent"
    fi
else
    warn "Could not detect firewall status"
fi
echo ""

# 6. Test network connectivity
echo "6. Checking network configuration..."
DEFAULT_IFACE=$(ip route | grep default | awk '{print $5}' | head -1)
if [ -n "$DEFAULT_IFACE" ]; then
    IP_ADDR=$(ip addr show "$DEFAULT_IFACE" | grep "inet " | awk '{print $2}' | cut -d/ -f1)
    if [ -n "$IP_ADDR" ]; then
        pass "Network interface: $DEFAULT_IFACE ($IP_ADDR)"
        echo "   Clients should discover bridge at: $IP_ADDR"
    else
        warn "Could not determine IP address"
    fi
else
    warn "Could not determine default network interface"
fi
echo ""

# 7. Check Matrix homeserver configuration
echo "7. Checking Matrix configuration..."
if [ -f "$HOME/.armorclaw/config.toml" ]; then
    MATRIX_URL=$(grep -E "^homeserver_url" "$HOME/.armorclaw/config.toml" 2>/dev/null | head -1 | awk -F'= ' '{print $2}' | tr -d '"')
    if [ -n "$MATRIX_URL" ]; then
        pass "Matrix homeserver configured: $MATRIX_URL"
    else
        warn "Matrix homeserver not configured in config.toml"
    fi
else
    warn "Config file not found at ~/.armorclaw/config.toml"
fi
echo ""

# Summary
echo "========================================"
echo "Test Summary"
echo "========================================"
echo -e "${GREEN}Passed:${NC} $PASS"
echo -e "${RED}Failed:${NC} $FAIL"
echo -e "${YELLOW}Warnings:${NC} $WARN"
echo ""

if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}Discovery system appears to be configured correctly!${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Install ArmorChat or ArmorTerminal on a device on the same network"
    echo "2. The app should auto-discover the bridge"
    echo "3. If Matrix URL not advertised, scan QR code from bridge admin UI"
else
    echo -e "${RED}Some issues detected. Please review the warnings above.${NC}"
fi

exit $FAIL
