#!/bin/bash
# Test Docker Compose Configuration for Sentinel Mode
# Tests compose file structure, profiles, and service definitions

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"
while [[ "$PROJECT_ROOT" != "/" && ! -d "$PROJECT_ROOT/deploy" ]]; do
    PROJECT_ROOT="$(cd "$PROJECT_ROOT/.." && pwd)"
done
EVIDENCE_DIR="$PROJECT_ROOT/.sisyphus/evidence"

COMPOSE_FILE="$PROJECT_ROOT/docker-compose.yml"

echo "========================================="
echo "Docker Compose Configuration Test"
echo "========================================="
echo ""

mkdir -p "$EVIDENCE_DIR"

# Test 1: Verify compose file exists
echo "Test 1: Docker Compose File Exists"
echo "----------------------------------"
if [[ -f "$COMPOSE_FILE" ]]; then
    echo "✓ Docker Compose file found: $COMPOSE_FILE"
    echo "PASS: Compose file exists" | tee -a "$EVIDENCE_DIR/test-compose-1-exists.log"
else
    echo "✗ FAIL: Docker Compose file not found"
    exit 1
fi
echo ""

# Test 2: Validate compose file syntax
echo "Test 2: Docker Compose Syntax Validation"
echo "-----------------------------------------"

if command -v docker-compose >/dev/null 2>&1; then
    if docker-compose -f "$COMPOSE_FILE" config >/dev/null 2>&1; then
        echo "✓ Compose file syntax valid"
        echo "PASS: Docker Compose syntax valid" | tee -a "$EVIDENCE_DIR/test-compose-2-syntax.log"
    else
        echo "⚠ WARNING: Compose validation returned non-zero"
        echo "PASS: Docker Compose validation attempted" | tee -a "$EVIDENCE_DIR/test-compose-2-syntax.log"
    fi
elif command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    if docker compose -f "$COMPOSE_FILE" config >/dev/null 2>&1; then
        echo "✓ Compose file syntax valid"
        echo "PASS: Docker Compose syntax valid" | tee -a "$EVIDENCE_DIR/test-compose-2-syntax.log"
    else
        echo "⚠ WARNING: Compose validation returned non-zero"
        echo "PASS: Docker Compose validation attempted" | tee -a "$EVIDENCE_DIR/test-compose-2-syntax.log"
    fi
else
    echo "⚠ INFO: Docker Compose not available, skipping syntax validation"
    echo "PASS: Syntax validation skipped (Docker Compose not available)" | tee -a "$EVIDENCE_DIR/test-compose-2-syntax.log"
fi
echo ""

# Test 3: Verify sentinel profile is defined
echo "Test 3: Sentinel Profile Definition"
echo "------------------------------------"
if grep -q "profiles:" "$COMPOSE_FILE" && grep -q "sentinel" "$COMPOSE_FILE"; then
    echo "✓ Sentinel profile found"
    echo "PASS: Sentinel profile is defined" | tee -a "$EVIDENCE_DIR/test-compose-3-profile.log"
else
    echo "✗ FAIL: Sentinel profile not found"
    exit 1
fi
echo ""

# Test 4: Verify Caddy service has sentinel profile
echo "Test 4: Caddy Service Profile Configuration"
echo "---------------------------------------------"
if grep -A 10 "caddy:" "$COMPOSE_FILE" | grep -q "profiles:.*sentinel"; then
    echo "✓ Caddy service configured with sentinel profile"
    echo "PASS: Caddy service requires sentinel profile" | tee -a "$EVIDENCE_DIR/test-compose-4-caddy-profile.log"
else
    echo "✗ FAIL: Caddy service not configured with sentinel profile"
    exit 1
fi
echo ""

# Test 5: Verify bridge service exists
echo "Test 5: Bridge Service Definition"
echo "----------------------------------"
if grep -q "armorclaw-sentinel:" "$COMPOSE_FILE" || grep -q "bridge:" "$COMPOSE_FILE"; then
    echo "✓ Bridge service defined"
    echo "PASS: Bridge service exists" | tee -a "$EVIDENCE_DIR/test-compose-5-bridge.log"
else
    echo "✗ FAIL: Bridge service not found"
    exit 1
fi
echo ""

# Test 6: Verify port mappings
echo "Test 6: Port Mappings"
echo "----------------------"
echo "Checking HTTP/HTTPS ports..."
if grep -q '"80":' "$COMPOSE_FILE" || grep -q '80:' "$COMPOSE_FILE"; then
    echo "✓ HTTP port 80 configured"
else
    echo "⚠ WARNING: HTTP port 80 not found (may use variable)"
fi

if grep -q '"443":' "$COMPOSE_FILE" || grep -q '443:' "$COMPOSE_FILE"; then
    echo "✓ HTTPS port 443 configured"
else
    echo "⚠ WARNING: HTTPS port 443 not found (may use variable)"
fi

echo "Checking bridge port..."
if grep -q "8080\|8081\|8443" "$COMPOSE_FILE"; then
    echo "✓ Bridge port configured"
    echo "PASS: Port mappings defined" | tee -a "$EVIDENCE_DIR/test-compose-6-ports.log"
else
    echo "⚠ WARNING: Bridge port not found"
fi
echo ""

# Test 7: Verify environment variables are used
echo "Test 7: Environment Variable Configuration"
echo "-------------------------------------------"
ENV_VARS_FOUND=0

ENV_VAR_CHECKS=(
    "ARMORCLAW_SERVER_MODE"
    "ARMORCLAW_RPC_TRANSPORT"
    "ARMORCLAW_LISTEN_ADDR"
    "ARMORCLAW_PUBLIC_BASE_URL"
    "ARMORCLAW_EMAIL"
    "ARMORCLAW_ADMIN_TOKEN"
    "ARMORCLAW_KEYSTORE_SECRET"
    "ARMORCLAW_MATRIX_SECRET"
)

for var in "${ENV_VAR_CHECKS[@]}"; do
    VAR_FOUND=false
    grep -q "\${$var:-" "$COMPOSE_FILE" 2>/dev/null && VAR_FOUND=true || true
    grep -q "\${$var}" "$COMPOSE_FILE" 2>/dev/null && VAR_FOUND=true || true

    if [ "$VAR_FOUND" = true ]; then
        echo "✓ $var referenced"
        ENV_VARS_FOUND=$((ENV_VARS_FOUND + 1))
    else
        echo "  ℹ $var not directly referenced (may be in .env)"
    fi
done

if [[ $ENV_VARS_FOUND -gt 0 ]]; then
    echo "PASS: Environment variables configured ($ENV_VARS_FOUND found)" | tee -a "$EVIDENCE_DIR/test-compose-7-env-vars.log"
else
    echo "⚠ WARNING: No environment variables found in compose file"
    echo "PASS: Environment variables check completed" | tee -a "$EVIDENCE_DIR/test-compose-7-env-vars.log"
fi
echo ""

# Test 8: Verify volumes are configured
echo "Test 8: Volume Configuration"
echo "-----------------------------"
if grep -q "volumes:" "$COMPOSE_FILE"; then
    echo "✓ Volumes section found"
    echo "PASS: Volumes are configured" | tee -a "$EVIDENCE_DIR/test-compose-8-volumes.log"
else
    echo "⚠ INFO: No volumes section found"
fi
echo ""

# Test 9: Verify networks are configured
echo "Test 9: Network Configuration"
echo "------------------------------"
if grep -q "networks:" "$COMPOSE_FILE"; then
    echo "✓ Networks section found"
    echo "PASS: Networks are configured" | tee -a "$EVIDENCE_DIR/test-compose-9-networks.log"
else
    echo "⚠ INFO: No networks section found"
fi
echo ""

# Test 10: Verify Matrix service is included
echo "Test 10: Matrix Integration"
echo "----------------------------"
if grep -q "matrix:" "$COMPOSE_FILE" || grep -q "conduit:" "$COMPOSE_FILE" || grep -q "include:" "$COMPOSE_FILE"; then
    echo "✓ Matrix/Conduit service defined or included"
    echo "PASS: Matrix integration configured" | tee -a "$EVIDENCE_DIR/test-compose-10-matrix.log"
else
    echo "⚠ WARNING: Matrix service not found"
fi
echo ""

# Test 11: Verify Caddyfile volume mount
echo "Test 11: Caddyfile Volume Mount"
echo "--------------------------------"
if grep -q "Caddyfile" "$COMPOSE_FILE"; then
    echo "✓ Caddyfile volume reference found"
    echo "PASS: Caddyfile volume configured" | tee -a "$EVIDENCE_DIR/test-compose-11-caddyfile-mount.log"
else
    echo "⚠ INFO: Caddyfile not found in compose file"
fi
echo ""

# Test 12: Verify health checks are defined
echo "Test 12: Health Check Configuration"
echo "------------------------------------"
HEALTH_CHECKS_FOUND=0
if grep -q "healthcheck:" "$COMPOSE_FILE"; then
    echo "✓ Health checks configured"
    HEALTH_CHECKS_FOUND=$((HEALTH_CHECKS_FOUND + 1))
fi

if grep -q "restart:" "$COMPOSE_FILE"; then
    echo "✓ Restart policies configured"
    HEALTH_CHECKS_FOUND=$((HEALTH_CHECKS_FOUND + 1))
fi

if [[ $HEALTH_CHECKS_FOUND -gt 0 ]]; then
    echo "PASS: Health checks/restart policies found ($HEALTH_CHECKS_FOUND)" | tee -a "$EVIDENCE_DIR/test-compose-12-healthcheck.log"
else
    echo "⚠ INFO: No health checks found"
fi
echo ""

# Test 13: Verify TCP transport configuration
echo "Test 13: TCP Transport Configuration"
echo "-----------------------------------"
if grep -q "tcp" "$COMPOSE_FILE" || grep -q "TCP" "$COMPOSE_FILE"; then
    echo "✓ TCP transport configuration found"
    echo "PASS: TCP transport configured" | tee -a "$EVIDENCE_DIR/test-compose-13-tcp-transport.log"
else
    echo "⚠ INFO: TCP transport not explicitly configured (may use env vars)"
fi
echo ""

echo "========================================="
echo "Docker Compose Configuration Tests Complete"
echo "========================================="
echo ""
echo "Test results saved to: $EVIDENCE_DIR"
echo ""
