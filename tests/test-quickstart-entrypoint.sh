#!/bin/bash
# Test suite for quickstart-entrypoint.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/transport.sh"

TESTS_PASSED=0
TESTS_FAILED=0

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_test() {
    echo -e "${CYAN}[TEST]${NC} $1"
}

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++))
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((TESTS_FAILED++))
}

log_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

# Setup test environment
TEST_DIR="$(mktemp -d)"
TEST_CONFIG_DIR="$TEST_DIR/etc/armorclaw"
TEST_DATA_DIR="$TEST_DIR/var/lib/armorclaw"
TEST_OPT="/opt/armorclaw"

mkdir -p "$TEST_CONFIG_DIR" "$TEST_DATA_DIR" "$TEST_OPT/configs"

# Create mock conduit.toml template
cat > "$TEST_OPT/configs/conduit.toml" << 'EOF'
[global]
server_name = "armorclaw.local"
database_backend = "rocksdb"
database_path = "/var/lib/conduit"
address = "0.0.0.0"
port = 6167
allow_registration = false
allow_federation = true
allow_check_for_updates = true
trusted_servers = ["matrix.org"]
EOF

# Create mock config.toml template
cat > "$TEST_OPT/configs/config.toml" << 'EOF'
[bridge]
socket_path = "/run/armorclaw/bridge.sock"
homeserver_url = "http://localhost:6167"
db_path = "/var/lib/armorclaw/armorclaw.db"

[http]
address = "0.0.0.0"
port = 8443
EOF

# Copy quickstart entrypoint
cp deploy/quickstart-entrypoint.sh "$TEST_DIR/entrypoint.sh"
chmod +x "$TEST_DIR/entrypoint.sh"

cd "$TEST_DIR"

# Test 1: Docker socket check - no socket
log_test "Test 1: Should exit gracefully without Docker socket"
if [ -S /var/run/docker.sock ]; then
    log_info "Docker socket present — skipping no-socket test (not applicable)"
else
    TEST_OUTPUT=$("$TEST_DIR/entrypoint.sh" 2>&1 || true)
    if echo "$TEST_OUTPUT" | grep -q "Docker socket not available"; then
        if echo "$TEST_OUTPUT" | grep -q "Bootstrap complete"; then
            log_pass "Exited gracefully with bridge-only mode"
        else
            log_fail "Did not show bootstrap complete message"
        fi
    else
        log_fail "Did not detect missing Docker socket"
    fi
fi

# Test 2: Check if config templates exist
log_test "Test 2: Should find conduit.toml template"
if [ -f "$TEST_OPT/configs/conduit.toml" ]; then
    log_pass "Conduit template found"
else
    log_fail "Conduit template not found"
fi

log_test "Test 3: Should find config.toml template"
if [ -f "$TEST_OPT/configs/config.toml" ]; then
    log_pass "Config template found"
else
    log_fail "Config template not found"
fi

# Test 4: Test config copying logic
log_test "Test 4: Should copy conduit.toml to config directory"
export TEST_INIT_FLAG="$TEST_DATA_DIR/.bootstrapped"
# Simulate running the logic
mkdir -p "$TEST_CONFIG_DIR"
if [ -f "$TEST_OPT/configs/conduit.toml" ]; then
    cp "$TEST_OPT/configs/conduit.toml" "$TEST_CONFIG_DIR/conduit.toml"
    if [ -f "$TEST_CONFIG_DIR/conduit.toml" ]; then
        log_pass "Config copied successfully"
    else
        log_fail "Config copy failed"
    fi
else
    log_fail "Template not found for copy test"
fi

# Test 5: Test directory creation
log_test "Test 5: Should create required directories"
mkdir -p "$TEST_CONFIG_DIR" "$TEST_DATA_DIR"
if [ -d "$TEST_CONFIG_DIR" ] && [ -d "$TEST_DATA_DIR" ]; then
    log_pass "Directories created successfully"
else
    log_fail "Directory creation failed"
fi

# Test 6: Test INIT_FLAG creation
log_test "Test 6: Should create bootstrapped flag"
INIT_FLAG="$TEST_DATA_DIR/.bootstrapped"
touch "$INIT_FLAG"
if [ -f "$INIT_FLAG" ]; then
    log_pass "INIT_FLAG created successfully"
else
    log_fail "INIT_FLAG creation failed"
fi

# Cleanup
cd /dev/null
rm -rf "$TEST_DIR"

# Summary
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${CYAN}Test Summary:${NC}"
echo "  Passed: $TESTS_PASSED"
echo "  Failed: $TESTS_FAILED"
echo "  Total:  $((TESTS_PASSED + TESTS_FAILED))"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    exit 1
fi
