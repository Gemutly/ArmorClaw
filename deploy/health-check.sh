#!/bin/bash
# ArmorClaw Health Check Script
# Part of Topology Separation (G-06)
#
# Usage: ./deploy/health-check.sh [--verbose]

set -e

VERBOSE=false
if [[ "$1" == "--verbose" || "$1" == "-v" ]]; then
    VERBOSE=true
fi

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
PASS=0
FAIL=0
WARN=0

log() {
    if $VERBOSE; then
        echo -e "$1"
    fi
}

check_pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((PASS++)) || true
}

check_fail() {
    echo -e "${RED}✗${NC} $1"
    ((FAIL++)) || true
}

check_warn() {
    echo -e "${YELLOW}!${NC} $1"
    ((WARN++)) || true
}

check_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

echo "========================================"
echo "ArmorClaw Health Check"
echo "========================================"
echo ""

# ========================================
# Docker Checks
# ========================================
echo "--- Docker ---"

# Check Docker daemon
if docker info >/dev/null 2>&1; then
    check_pass "Docker daemon is running"
else
    check_fail "Docker daemon is not running"
    echo "  Run: sudo systemctl start docker"
fi

# Check ArmorClaw containers
if docker ps --format '{{.Names}}' | grep -q '^armorclaw-'; then
    check_pass "ArmorClaw containers are running"
else
    check_warn "No ArmorClaw containers detected"
fi

echo ""

# ========================================
# Matrix Stack Checks
# ========================================
echo "--- Matrix Stack ---"

# Check Matrix Conduit
if curl -sf http://localhost:6167/_matrix/client/versions > /dev/null 2>&1; then
    check_pass "Matrix Conduit is responding"
else
    check_fail "Matrix Conduit is not responding"
fi

# Check Matrix Federation
if curl -sf http://localhost:8448/_matrix/federation/v1/version > /dev/null 2>&1; then
    check_pass "Matrix Federation endpoint is available"
else
    check_warn "Matrix Federation endpoint not available (may be disabled)"
fi

# Check Nginx
if curl -sf http://localhost/_matrix/client/versions > /dev/null 2>&1; then
    check_pass "Nginx proxy is routing Matrix requests"
else
    check_fail "Nginx proxy is not responding"
fi

# Check Nginx health endpoint
if curl -sf http://localhost/health > /dev/null 2>&1; then
    check_pass "Nginx health endpoint is responding"
else
    check_warn "Nginx health endpoint not configured"
fi

# ========================================
# Bridge Stack Checks
# ========================================
echo ""
echo "--- Bridge Stack ---"

# Check Sygnal Push Gateway
if curl -sf http://localhost:5000/_matrix/push/v1/notify > /dev/null 2>&1; then
    check_pass "Sygnal push gateway is responding"
else
    check_fail "Sygnal push gateway is not responding"
fi

# ========================================
# AI Stack Checks
# ========================================
echo ""
echo "--- AI Stack ---"

# Check Catwalk
if curl -sf http://localhost:8080/healthz > /dev/null 2>&1; then
    check_pass "Catwalk AI service is responding"
else
    check_warn "Catwalk AI service is not responding (AI provider discovery unavailable)"
fi

# ========================================
# Bridge Checks
# ========================================
echo ""
echo "--- Bridge ---"

# Check Bridge RPC (HTTP)
if curl -sf http://localhost:8443/health > /dev/null 2>&1; then
    check_pass "Bridge RPC HTTP endpoint is responding"
else
    check_fail "Bridge RPC HTTP endpoint is not responding"
fi

# Check ArmorClaw Bridge (Unix Socket)
BRIDGE_SOCK="/run/armorclaw/bridge.sock"
if [[ -S "$BRIDGE_SOCK" ]]; then
    check_pass "Bridge socket exists at $BRIDGE_SOCK"

    # Try RPC status command
    if echo '{"jsonrpc":"2.0","id":1,"method":"bridge.status"}' | socat - UNIX-CONNECT:"$BRIDGE_SOCK" > /dev/null 2>&1; then
        check_pass "Bridge RPC is responding"
    else
        check_fail "Bridge RPC is not responding"
    fi
else
    check_warn "Bridge socket not found (bridge may not be running)"
fi

# ========================================
# Docker Containers Checks
# ========================================
echo ""
echo "--- Docker Containers ---"

# Check if Docker is running
if docker info > /dev/null 2>&1; then
    check_pass "Docker daemon is running"
else
    check_fail "Docker daemon is not running"
fi

# Check expected containers
CONTAINERS=("armorclaw-conduit" "armorclaw-nginx" "armorclaw-coturn" "armorclaw-sygnal")

for container in "${CONTAINERS[@]}"; do
    if docker ps --format '{{.Names}}' | grep -q "^${container}$"; then
        STATUS=$(docker inspect -f '{{.State.Health.Status}}' "$container" 2>/dev/null || echo "unknown")
        if [[ "$STATUS" == "healthy" ]]; then
            check_pass "Container $container is running and healthy"
        elif [[ "$STATUS" == "unknown" ]]; then
            check_pass "Container $container is running (no health check)"
        else
            check_warn "Container $container is running but status: $STATUS"
        fi
    else
        log "${YELLOW}Container $container is not running${NC}"
    fi
done

# ========================================
# Network Checks
# ========================================
echo ""
echo "--- Network Topology ---"

# Check matrix-net
if docker network ls --format '{{.Name}}' | grep -q "^armorclaw-matrix$"; then
    check_pass "matrix-net network exists"
else
    check_warn "matrix-net network not found"
fi

# Check bridge-net
if docker network ls --format '{{.Name}}' | grep -q "^armorclaw-bridge$"; then
    check_pass "bridge-net network exists"
else
    check_warn "bridge-net network not found"
fi

# ========================================
# Volume Checks
# ========================================
echo ""
echo "--- Volumes ---"

VOLUMES=("armorclaw-conduit-data" "armorclaw-coturn-data" "armorclaw-sygnal-data")

for volume in "${VOLUMES[@]}"; do
    if docker volume ls --format '{{.Name}}' | grep -q "^${volume}$"; then
        check_pass "Volume $volume exists"
    else
        log "${YELLOW}Volume $volume not found${NC}"
    fi
done

# ========================================
# Configuration Checks
# ========================================
echo ""
echo "--- Configuration ---"

CONFIGS=("configs/conduit.toml" "configs/nginx.conf" "configs/sygnal.yaml" "configs/turnserver.conf")

for config in "${CONFIGS[@]}"; do
    if [[ -f "$config" ]]; then
        check_pass "Config file $config exists"
    else
        check_warn "Config file $config not found"
    fi
done

# ========================================
# Security Checks
# ========================================
echo ""
echo "--- Security ---"

# Check if bridge socket has correct permissions
if [[ -S "$BRIDGE_SOCK" ]]; then
    SOCK_PERMS=$(stat -c '%a' "$BRIDGE_SOCK" 2>/dev/null || stat -f '%Lp' "$BRIDGE_SOCK" 2>/dev/null)
    if [[ "$SOCK_PERMS" == "600" || "$SOCK_PERMS" == "660" ]]; then
        check_pass "Bridge socket has restricted permissions ($SOCK_PERMS)"
    else
        check_warn "Bridge socket permissions: $SOCK_PERMS (recommend 600 or 660)"
    fi
fi

# Check if registration is disabled
if grep -q 'allow_registration.*false\|allow_registration.*=.*false' configs/conduit.toml 2>/dev/null; then
    check_pass "Matrix registration is disabled"
else
    check_warn "Matrix registration may be enabled (check configs/conduit.toml)"
fi

# ========================================
# Firewall Checks
# ========================================
echo ""
echo "--- Firewall ---"

# Check if UFW is available
if command -v ufw &> /dev/null; then
    UFW_STATUS=$(ufw status 2>/dev/null | head -1)
    if echo "$UFW_STATUS" | grep -q "Status: active"; then
        check_pass "UFW firewall is active"

        # Check required ports
        REQUIRED_PORTS=("22/tcp" "80/tcp" "443/tcp" "8448/tcp")
        for port in "${REQUIRED_PORTS[@]}"; do
            if ufw status | grep -q "$port"; then
                log "${GREEN}  Port $port is allowed${NC}"
            else
                check_warn "Port $port may not be allowed in UFW"
            fi
        done
    elif echo "$UFW_STATUS" | grep -q "Status: inactive"; then
        check_fail "UFW firewall is inactive"
    else
        check_warn "Could not determine UFW status"
    fi
else
    check_warn "UFW not available (may be using different firewall)"
fi

# ========================================
# HTTPS / TLS Checks
# ========================================
echo ""
echo "--- HTTPS / TLS ---"

# Check if HTTPS is configured for Matrix
MATRIX_DOMAIN=$(grep 'server_name' configs/conduit.toml 2>/dev/null | head -1 | sed 's/.*= *"\([^"]*\)".*/\1/' || echo "")

if [[ -n "$MATRIX_DOMAIN" ]]; then
    # Check local HTTPS
    if curl -sf https://localhost/_matrix/client/versions > /dev/null 2>&1; then
        check_pass "HTTPS is configured and working"

        # Check certificate validity
        CERT_INFO=$(echo | openssl s_client -servername localhost -connect localhost:443 2>/dev/null | openssl x509 -noout -dates 2>/dev/null || echo "")
        if [[ -n "$CERT_INFO" ]]; then
            CERT_EXPIRY=$(echo "$CERT_INFO" | grep "notAfter" | sed 's/notAfter=//')
            check_pass "SSL certificate valid until: $CERT_EXPIRY"

            # Check if certificate expires soon
            EXPIRY_EPOCH=$(date -d "$CERT_EXPIRY" +%s 2>/dev/null || date -j -f "%b %d %T %Y %Z" "$CERT_EXPIRY" +%s 2>/dev/null || echo "0")
            NOW_EPOCH=$(date +%s)
            DAYS_LEFT=$(( (EXPIRY_EPOCH - NOW_EPOCH) / 86400 ))
            if [[ $DAYS_LEFT -lt 7 ]]; then
                check_fail "SSL certificate expires in $DAYS_LEFT days!"
            elif [[ $DAYS_LEFT -lt 30 ]]; then
                check_warn "SSL certificate expires in $DAYS_LEFT days"
            fi
        else
            check_warn "Could not verify SSL certificate details"
        fi
    else
        # HTTPS not working - check if it's a development setup
        if curl -sf http://localhost/_matrix/client/versions > /dev/null 2>&1; then
            check_fail "HTTPS not configured - HTTP only (not production-ready)"
        else
            check_fail "Neither HTTP nor HTTPS is responding"
        fi
    fi

    # Check certificate chain
    CHAIN_VERIFY=$(echo | openssl s_client -servername localhost -connect localhost:443 -verify_return_error 2>&1 | grep -i "verify" || echo "")
    if echo "$CHAIN_VERIFY" | grep -qi "error\|fail"; then
        check_warn "SSL certificate chain verification issue"
    fi
else
    check_warn "Could not determine Matrix domain from config"
fi

# ========================================
# Production Readiness
# ========================================
echo ""
echo "--- Production Readiness ---"

# Check for default/secrets in config
if grep -q 'change-me\|your-password\|changeme' configs/*.toml configs/*.yaml 2>/dev/null; then
    check_fail "Default/placeholder values found in config files"
else
    check_pass "No default values in config files"
fi

# Check if running as non-root (bridge)
if [[ -S "$BRIDGE_SOCK" ]]; then
    SOCK_OWNER=$(stat -c '%U' "$BRIDGE_SOCK" 2>/dev/null || stat -f '%Su' "$BRIDGE_SOCK" 2>/dev/null)
    if [[ "$SOCK_OWNER" == "root" ]]; then
        check_warn "Bridge socket owned by root (consider using dedicated user)"
    else
        check_pass "Bridge socket owned by: $SOCK_OWNER"
    fi
fi

# ========================================
# Summary
# ========================================
echo ""
echo "========================================"
echo "Summary"
echo "========================================"
echo -e "${GREEN}Passed:${NC} $PASS"
echo -e "${RED}Failed:${NC} $FAIL"
echo -e "${YELLOW}Warnings:${NC} $WARN"
echo ""

if [[ $FAIL -gt 0 ]]; then
    echo -e "${RED}Status: UNHEALTHY${NC}"
    exit 1
elif [[ $WARN -gt 0 ]]; then
    echo -e "${YELLOW}Status: DEGRADED${NC}"
    exit 0
else
    echo -e "${GREEN}Status: HEALTHY${NC}"
    exit 0
fi
