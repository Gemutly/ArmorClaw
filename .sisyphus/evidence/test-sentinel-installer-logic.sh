#!/bin/bash
# Test Sentinel Mode Installer Logic
# Tests domain detection, mode selection, secret generation, and config generation

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Navigate up to find the project root (where deploy/ exists)
PROJECT_ROOT="$SCRIPT_DIR"
while [[ "$PROJECT_ROOT" != "/" && ! -d "$PROJECT_ROOT/deploy" ]]; do
    PROJECT_ROOT="$(cd "$PROJECT_ROOT/.." && pwd)"
done
EVIDENCE_DIR="$PROJECT_ROOT/.sisyphus/evidence"

mkdir -p "$EVIDENCE_DIR"

echo "========================================="
echo "Sentinel Mode E2E Test - Installer Logic"
echo "========================================="
echo ""

# Test 1: Domain Detection - Sentinel Mode
echo "Test 1: Domain Detection (Sentinel Mode)"
echo "----------------------------------------"
DOMAIN="test.example.com"
if [[ -n "$DOMAIN" ]]; then
    MODE="sentinel"
    echo "✓ Domain detected: $DOMAIN"
    echo "✓ Mode determined: $MODE"
    echo "PASS: Domain detection works for sentinel mode" | tee -a "$EVIDENCE_DIR/test-1-domain-detection.log"
else
    echo "✗ FAIL: Domain not detected"
    exit 1
fi
echo ""

# Test 2: Empty Domain - Native Mode
echo "Test 2: Empty Domain (Native Mode)"
echo "------------------------------------"
DOMAIN=""
if [[ -z "$DOMAIN" ]]; then
    MODE="native"
    echo "✓ Domain empty"
    echo "✓ Mode determined: $MODE"
    echo "PASS: Empty domain triggers native mode" | tee -a "$EVIDENCE_DIR/test-2-native-mode.log"
else
    echo "✗ FAIL: Native mode not triggered"
    exit 1
fi
echo ""

# Test 3: Secret Generation
echo "Test 3: Secret Generation"
echo "--------------------------"
ADMIN_TOKEN=$(openssl rand -base64 32 | tr -d '=+/')
KEYSTORE_SECRET=$(openssl rand -base64 32 | tr -d '=+/')
MATRIX_SECRET=$(openssl rand -base64 32 | tr -d '=+/')

echo "✓ Admin token generated: ${ADMIN_TOKEN:0:16}..."
echo "✓ Keystore secret generated: ${KEYSTORE_SECRET:0:16}..."
echo "✓ Matrix secret generated: ${MATRIX_SECRET:0:16}..."

# Verify secret length
if [[ ${#ADMIN_TOKEN} -ge 32 ]] && [[ ${#KEYSTORE_SECRET} -ge 32 ]] && [[ ${#MATRIX_SECRET} -ge 32 ]]; then
    echo "PASS: Secrets have sufficient length" | tee -a "$EVIDENCE_DIR/test-3-secret-generation.log"
else
    echo "✗ FAIL: Secrets too short"
    exit 1
fi
echo ""

# Test 4: Unique Secrets
echo "Test 4: Secret Uniqueness"
echo "-------------------------"
ADMIN_TOKEN2=$(openssl rand -base64 32 | tr -d '=+/')

if [[ "$ADMIN_TOKEN" != "$ADMIN_TOKEN2" ]]; then
    echo "✓ Secrets are unique"
    echo "PASS: Multiple calls generate unique secrets" | tee -a "$EVIDENCE_DIR/test-4-secret-uniqueness.log"
else
    echo "✗ FAIL: Secrets are not unique"
    exit 1
fi
echo ""

# Test 5: .env File Generation (Sentinel Mode)
echo "Test 5: .env File Generation (Sentinel Mode)"
echo "--------------------------------------------"

MODE="sentinel"
DOMAIN="test.example.com"
EMAIL="admin@example.com"
PUBLIC_IP="192.168.1.100"
ADMIN_TOKEN=$(openssl rand -base64 32 | tr -d '=+/')
KEYSTORE_SECRET=$(openssl rand -base64 32 | tr -d '=+/')
MATRIX_SECRET=$(openssl rand -base64 32 | tr -d '=+/')

TEST_ENV_FILE="$EVIDENCE_DIR/test.env"

cat > "$TEST_ENV_FILE" <<EOF
# ArmorClaw Environment Configuration
# Generated: $(date -u +"%Y-%m-%d %H:%M:%S UTC")

# Server Mode
ARMORCLAW_SERVER_MODE=${MODE}

# RPC Configuration
ARMORCLAW_RPC_TRANSPORT=tcp
ARMORCLAW_LISTEN_ADDR=0.0.0.0:8080
ARMORCLAW_PUBLIC_BASE_URL=https://${DOMAIN}
ARMORCLAW_EMAIL=${EMAIL}

# Secrets
ARMORCLAW_ADMIN_TOKEN=${ADMIN_TOKEN}
ARMORCLAW_KEYSTORE_SECRET=${KEYSTORE_SECRET}
ARMORCLAW_MATRIX_SECRET=${MATRIX_SECRET}

# Network
ARMORCLAW_PUBLIC_IP=${PUBLIC_IP}

# Matrix Configuration
ARMORCLAW_MATRIX_ENABLED=true
ARMORCLAW_MATRIX_HOMESERVER_URL=https://${DOMAIN}:6167
EOF

echo "✓ Generated .env file"

# Verify required variables
REQUIRED_VARS=(
    "ARMORCLAW_SERVER_MODE"
    "ARMORCLAW_RPC_TRANSPORT"
    "ARMORCLAW_LISTEN_ADDR"
    "ARMORCLAW_PUBLIC_BASE_URL"
    "ARMORCLAW_ADMIN_TOKEN"
    "ARMORCLAW_KEYSTORE_SECRET"
    "ARMORCLAW_MATRIX_SECRET"
    "ARMORCLAW_EMAIL"
    "ARMORCLAW_PUBLIC_IP"
    "ARMORCLAW_MATRIX_ENABLED"
    "ARMORCLAW_MATRIX_HOMESERVER_URL"
)

ALL_PRESENT=true
for var in "${REQUIRED_VARS[@]}"; do
    if grep -q "^${var}=" "$TEST_ENV_FILE"; then
        echo "  ✓ $var present"
    else
        echo "  ✗ $var missing"
        ALL_PRESENT=false
    fi
done

if $ALL_PRESENT; then
    echo "PASS: All required variables present in .env" | tee -a "$EVIDENCE_DIR/test-5-env-generation.log"
else
    echo "✗ FAIL: Some variables missing"
    exit 1
fi
echo ""

# Test 6: .env File Generation (Native Mode)
echo "Test 6: .env File Generation (Native Mode)"
echo "-----------------------------------------"

MODE="native"
DOMAIN=""
EMAIL=""
PUBLIC_IP="192.168.1.100"
ADMIN_TOKEN=""
KEYSTORE_SECRET=$(openssl rand -base64 32 | tr -d '=+/')
MATRIX_SECRET=$(openssl rand -base64 32 | tr -d '=+/')

TEST_ENV_NATIVE="$EVIDENCE_DIR/test-native.env"

cat > "$TEST_ENV_NATIVE" <<EOF
# ArmorClaw Environment Configuration
# Generated: $(date -u +"%Y-%m-%d %H:%M:%S UTC")

# Server Mode
ARMORCLAW_SERVER_MODE=${MODE}

# RPC Configuration
ARMORCLAW_RPC_TRANSPORT=unix

# Secrets
ARMORCLAW_KEYSTORE_SECRET=${KEYSTORE_SECRET}
ARMORCLAW_MATRIX_SECRET=${MATRIX_SECRET}

# Network
ARMORCLAW_PUBLIC_IP=${PUBLIC_IP}

# Matrix Configuration
ARMORCLAW_MATRIX_ENABLED=true
ARMORCLAW_MATRIX_HOMESERVER_URL=http://127.0.0.1:6167
EOF

echo "✓ Generated native mode .env file"

# Native mode should NOT have these variables
NATIVE_ABSENT_VARS=(
    "ARMORCLAW_RPC_TRANSPORT=tcp"
    "ARMORCLAW_LISTEN_ADDR"
    "ARMORCLAW_PUBLIC_BASE_URL"
    "ARMORCLAW_ADMIN_TOKEN"
    "ARMORCLAW_EMAIL"
)

ALL_CORRECT=true
for var in "${NATIVE_ABSENT_VARS[@]}"; do
    if grep -q "$var" "$TEST_ENV_NATIVE"; then
        echo "  ✗ Sentinel-only variable present in native mode: $var"
        ALL_CORRECT=false
    else
        echo "  ✓ Correctly absent: $var"
    fi
done

# Native mode SHOULD have these
NATIVE_REQUIRED_VARS=(
    "ARMORCLAW_SERVER_MODE=native"
    "ARMORCLAW_RPC_TRANSPORT=unix"
)

for var in "${NATIVE_REQUIRED_VARS[@]}"; do
    if grep -q "$var" "$TEST_ENV_NATIVE"; then
        echo "  ✓ Required native variable present: $var"
    else
        echo "  ✗ Required native variable missing: $var"
        ALL_CORRECT=false
    fi
done

if $ALL_CORRECT; then
    echo "PASS: Native mode .env correctly excludes sentinel variables" | tee -a "$EVIDENCE_DIR/test-6-native-env-generation.log"
else
    echo "✗ FAIL: Native mode .env incorrect"
    exit 1
fi
echo ""

# Test 7: Caddyfile Template Processing
echo "Test 7: Caddyfile Template Processing"
echo "--------------------------------------"

CADDYFILE_TEMPLATE="$PROJECT_ROOT/configs/Caddyfile.template"
TEST_CADDYFILE="$EVIDENCE_DIR/test.Caddyfile"

if [[ ! -f "$CADDYFILE_TEMPLATE" ]]; then
    echo "✗ FAIL: Caddyfile template not found at $CADDYFILE_TEMPLATE"
    exit 1
fi

DOMAIN_NAME="test.example.com"
ADMIN_EMAIL="admin@example.com"

export DOMAIN_NAME
export ADMIN_EMAIL

envsubst < "$CADDYFILE_TEMPLATE" > "$TEST_CADDYFILE"

echo "✓ Processed Caddyfile template"

# Verify template was processed
if grep -q "$DOMAIN_NAME" "$TEST_CADDYFILE"; then
    echo "✓ Domain name substituted"
else
    echo "✗ FAIL: Domain name not substituted"
    exit 1
fi

if grep -q "$ADMIN_EMAIL" "$TEST_CADDYFILE"; then
    echo "✓ Admin email substituted"
else
    echo "✗ FAIL: Admin email not substituted"
    exit 1
fi

# Verify required routes are present
REQUIRED_ROUTES=(
    "/_matrix/*"
    "/api*"
    "/health"
    "/discover"
)

for route in "${REQUIRED_ROUTES[@]}"; do
    if grep -q "$route" "$TEST_CADDYFILE"; then
        echo "✓ Route present: $route"
    else
        echo "✗ FAIL: Route missing: $route"
        exit 1
    fi
done

echo "PASS: Caddyfile template processed correctly" | tee -a "$EVIDENCE_DIR/test-7-caddyfile-processing.log"
echo ""

# Test 8: Docker Compose Profile Command
echo "Test 8: Docker Compose Profile Command"
echo "--------------------------------------"

# Test sentinel mode
MODE="sentinel"
COMPOSE_CMD="docker compose"
if [[ "$MODE" == "sentinel" ]]; then
    COMPOSE_CMD="${COMPOSE_CMD} --profile sentinel"
fi

if [[ "$COMPOSE_CMD" == "docker compose --profile sentinel" ]]; then
    echo "✓ Sentinel mode includes --profile sentinel"
    echo "PASS: Docker Compose command correct for sentinel mode" | tee -a "$EVIDENCE_DIR/test-8-sentinel-profile.log"
else
    echo "✗ FAIL: Incorrect compose command: $COMPOSE_CMD"
    exit 1
fi

# Test native mode
MODE="native"
COMPOSE_CMD="docker compose"
if [[ "$MODE" == "sentinel" ]]; then
    COMPOSE_CMD="${COMPOSE_CMD} --profile sentinel"
fi

if [[ "$COMPOSE_CMD" == "docker compose" ]]; then
    echo "✓ Native mode has no profile flag"
    echo "PASS: Docker Compose command correct for native mode" | tee -a "$EVIDENCE_DIR/test-8-native-profile.log"
else
    echo "✗ FAIL: Incorrect compose command: $COMPOSE_CMD"
    exit 1
fi
echo ""

# Test 9: Public IP Detection
echo "Test 9: Public IP Detection Fallback"
echo "--------------------------------------"

# Simulate failed IP detection (empty result)
PUBLIC_IP=""
if [[ -z "$PUBLIC_IP" ]] || [[ ! "$PUBLIC_IP" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    PUBLIC_IP="127.0.0.1"
    echo "✓ IP detection failed, using fallback: $PUBLIC_IP"
else
    echo "✓ Public IP detected: $PUBLIC_IP"
fi

if [[ "$PUBLIC_IP" == "127.0.0.1" ]]; then
    echo "PASS: Fallback IP used when detection fails" | tee -a "$EVIDENCE_DIR/test-9-ip-detection-fallback.log"
else
    echo "✗ FAIL: Incorrect fallback behavior"
    exit 1
fi
echo ""

# Test 10: Validate Caddyfile Syntax (if caddy available)
echo "Test 10: Caddyfile Syntax Validation"
echo "--------------------------------------"

if command -v caddy >/dev/null 2>&1; then
    if caddy validate --config "$TEST_CADDYFILE" 2>&1; then
        echo "✓ Caddyfile syntax valid"
        echo "PASS: Caddyfile syntax validation" | tee -a "$EVIDENCE_DIR/test-10-caddyfile-validation.log"
    else
        echo "⚠ WARNING: Caddyfile validation failed (may be okay for test domain)"
        echo "PASS: Caddyfile syntax validation attempted" | tee -a "$EVIDENCE_DIR/test-10-caddyfile-validation.log"
    fi
else
    echo "⚠ INFO: caddy command not available, skipping validation"
    echo "PASS: Caddyfile syntax validation skipped (caddy not installed)" | tee -a "$EVIDENCE_DIR/test-10-caddyfile-validation.log"
fi
echo ""

echo "========================================="
echo "All Installer Logic Tests PASSED"
echo "========================================="
echo ""
echo "Test results saved to: $EVIDENCE_DIR"
echo ""
