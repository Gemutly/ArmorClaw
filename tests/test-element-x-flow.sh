#!/bin/bash
# ArmorClaw Element X Config Flow - End-to-End Test Suite
#
# Tests the complete flow from Matrix message to agent receiving config:
# 1. Bridge startup and health check
# 2. Matrix connection verification
# 3. Config attachment via RPC (simulating Matrix command)
# 4. Config file creation and verification
# 5. Container access to configs
# 6. Error handling and edge cases
#
# Usage:
#   ./test-element-x-flow.sh [--with-container]
#
# Requirements:
#   - Bridge built: bridge/build/armorclaw-bridge
#   - Matrix server running (optional, uses mock if not available)
#   - Docker (optional, for container tests)

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BRIDGE_BIN="${PROJECT_ROOT}/bridge/build/armorclaw-bridge"
BRIDGE_SOCK="/run/armorclaw/bridge.sock"
CONFIG_DIR="/run/armorclaw/configs"
WITH_CONTAINER="${1:-}"

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# Helper functions
log_test() {
    echo -e "\n${BLUE}═══════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}Test $1: $2${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════${NC}"
}

log_pass() {
    echo -e "${GREEN}✅ PASS: $1${NC}"
    ((TESTS_PASSED++))
}

log_fail() {
    echo -e "${RED}❌ FAIL: $1${NC}"
    ((TESTS_FAILED++))
}

log_skip() {
    echo -e "${YELLOW}⏭️  SKIP: $1${NC}"
    ((TESTS_SKIPPED++))
}

log_info() {
    echo -e "   $1"
}

rpc_call() {
    local method="$1"
    local params="$2"
    echo "{\"jsonrpc\":\"2.0\",\"method\":\"$method\",\"params\":$params,\"id\":$(date +%s%N)}" | \
        socat - UNIX-CONNECT:"$BRIDGE_SOCK" 2>/dev/null
}

check_jq() {
    if ! command -v jq &>/dev/null; then
        echo -e "${RED}Error: jq is required for this test suite${NC}"
        echo "Install with: sudo apt install jq"
        exit 1
    fi
}

check_socat() {
    if ! command -v socat &>/dev/null; then
        echo -e "${RED}Error: socat is required for this test suite${NC}"
        echo "Install with: sudo apt install socat"
        exit 1
    fi
}

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up test artifacts...${NC}"

    # Remove test configs
    sudo rm -rf "${CONFIG_DIR}/test-"* 2>/dev/null || true

    echo -e "${GREEN}Cleanup complete${NC}"
}
trap cleanup EXIT

# ============================================================================
# Print header
# ============================================================================
echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║     ArmorClaw Element X Config Flow - E2E Test Suite          ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Check dependencies
check_jq
check_socat

# ============================================================================
# TEST 1: Prerequisites Check
# ============================================================================
log_test "1" "Prerequisites Check"

# Check bridge binary
if [ -x "$BRIDGE_BIN" ]; then
    log_pass "Bridge binary exists at $BRIDGE_BIN"
else
    log_fail "Bridge binary not found. Build with: cd bridge && go build -o build/armorclaw-bridge ./cmd/bridge"
fi

# Check bridge socket
if [ -S "$BRIDGE_SOCK" ]; then
    log_pass "Bridge socket exists at $BRIDGE_SOCK"
else
    log_fail "Bridge socket not found. Start bridge with: sudo $BRIDGE_BIN"
fi

# Check config directory
if [ -d "$CONFIG_DIR" ]; then
    log_pass "Config directory exists at $CONFIG_DIR"
else
    log_info "Config directory will be created on first use"
fi

# Skip remaining tests if bridge not running
if [ ! -S "$BRIDGE_SOCK" ]; then
    echo -e "\n${RED}Bridge not running. Start bridge and re-run tests.${NC}"
    exit 1
fi

# ============================================================================
# TEST 2: Bridge Health Check
# ============================================================================
log_test "2" "Bridge Health Check"

RESPONSE=$(rpc_call "health" "{}")
log_info "Response: $RESPONSE"

if echo "$RESPONSE" | jq -e '.result.status == "ok"' >/dev/null 2>&1; then
    log_pass "Bridge health check passed"
    log_info "Status: $(echo "$RESPONSE" | jq -r '.result.status')"
    log_info "Version: $(echo "$RESPONSE" | jq -r '.result.version // "unknown"')"
else
    log_fail "Bridge health check failed"
fi

# ============================================================================
# TEST 3: Bridge Status Check
# ============================================================================
log_test "3" "Bridge Status Check"

RESPONSE=$(rpc_call "bridge.status" "{}")
log_info "Response: $(echo "$RESPONSE" | jq -c '.')"

if echo "$RESPONSE" | jq -e '.result' >/dev/null 2>&1; then
    log_pass "Bridge status check passed"
    log_info "State: $(echo "$RESPONSE" | jq -r '.result.state // "unknown"')"
    log_info "Socket: $(echo "$RESPONSE" | jq -r '.result.socket // "unknown"')"
else
    log_fail "Bridge status check failed"
fi

# ============================================================================
# TEST 4: Simple Config Attachment
# ============================================================================
log_test "4" "Simple Config Attachment (env file)"

RESPONSE=$(rpc_call "attach_config" '{
    "name": "test-simple.env",
    "content": "KEY1=value1\nKEY2=value2\nKEY3=value3",
    "encoding": "raw",
    "type": "env"
}')

log_info "Response: $(echo "$RESPONSE" | jq -c '.')"

if echo "$RESPONSE" | jq -e '.result.config_id' >/dev/null 2>&1; then
    CONFIG_ID=$(echo "$RESPONSE" | jq -r '.result.config_id')
    CONFIG_PATH=$(echo "$RESPONSE" | jq -r '.result.path')

    log_pass "Config attached successfully"
    log_info "Config ID: $CONFIG_ID"
    log_info "Path: $CONFIG_PATH"
    log_info "Size: $(echo "$RESPONSE" | jq -r '.result.size') bytes"

    # Verify file exists
    if [ -f "$CONFIG_PATH" ]; then
        log_pass "Config file created at $CONFIG_PATH"
        log_info "Content: $(cat "$CONFIG_PATH" | tr '\n' ' ')"
    else
        log_fail "Config file not created at $CONFIG_PATH"
    fi
else
    log_fail "Config attachment failed: $(echo "$RESPONSE" | jq -r '.error.message // "unknown error"')"
fi

# ============================================================================
# TEST 5: Base64 Encoded Config
# ============================================================================
log_test "5" "Base64 Encoded Config Attachment"

# Encode test content
TEST_CONTENT="SECRET_KEY=supersecret123\nAPI_TOKEN=token-abc-xyz"
BASE64_CONTENT=$(echo -ne "$TEST_CONTENT" | base64)

RESPONSE=$(rpc_call "attach_config" "{
    \"name\": \"test-secret.env\",
    \"content\": \"$BASE64_CONTENT\",
    \"encoding\": \"base64\",
    \"type\": \"env\"
}")

log_info "Response: $(echo "$RESPONSE" | jq -c '.')"

if echo "$RESPONSE" | jq -e '.result.config_id' >/dev/null 2>&1; then
    CONFIG_PATH=$(echo "$RESPONSE" | jq -r '.result.path')

    log_pass "Base64 config attached successfully"

    # Verify decoded content
    if [ -f "$CONFIG_PATH" ]; then
        DECODED=$(cat "$CONFIG_PATH")
        if echo "$DECODED" | grep -q "SECRET_KEY=supersecret123"; then
            log_pass "Content decoded correctly"
        else
            log_fail "Content not decoded correctly: $DECODED"
        fi
    fi
else
    log_fail "Base64 config attachment failed"
fi

# ============================================================================
# TEST 6: TOML Configuration
# ============================================================================
log_test "6" "TOML Configuration Attachment"

TOML_CONTENT='[agent]
model = "gpt-4"
temperature = 0.7

[limits]
max_tokens = 4096
timeout = 30'

# Escape for JSON
TOML_JSON=$(echo "$TOML_CONTENT" | jq -Rs .)

RESPONSE=$(rpc_call "attach_config" "{
    \"name\": \"test-agent.toml\",
    \"content\": $TOML_JSON,
    \"encoding\": \"raw\",
    \"type\": \"toml\"
}")

log_info "Response: $(echo "$RESPONSE" | jq -c '.')"

if echo "$RESPONSE" | jq -e '.result.config_id' >/dev/null 2>&1; then
    CONFIG_PATH=$(echo "$RESPONSE" | jq -r '.result.path')

    log_pass "TOML config attached successfully"

    # Verify TOML content
    if [ -f "$CONFIG_PATH" ]; then
        if grep -q "\[agent\]" "$CONFIG_PATH" && grep -q "model = \"gpt-4\"" "$CONFIG_PATH"; then
            log_pass "TOML content valid"
        else
            log_fail "TOML content corrupted"
        fi
    fi
else
    log_fail "TOML config attachment failed"
fi

# ============================================================================
# TEST 7: List Configs
# ============================================================================
log_test "7" "List Attached Configs"

RESPONSE=$(rpc_call "list_configs" "{}")
log_info "Response: $(echo "$RESPONSE" | jq -c '.')"

if echo "$RESPONSE" | jq -e '.result.configs' >/dev/null 2>&1; then
    CONFIG_COUNT=$(echo "$RESPONSE" | jq -r '.result.count')
    log_pass "List configs successful"
    log_info "Config count: $CONFIG_COUNT"

    # Verify our test configs are in the list
    if echo "$RESPONSE" | jq -e '.result.configs[] | select(.name | startswith("test-"))' >/dev/null 2>&1; then
        log_pass "Test configs found in list"
    fi
else
    log_fail "List configs failed"
fi

# ============================================================================
# TEST 8: Path Traversal Protection
# ============================================================================
log_test "8" "Path Traversal Protection"

RESPONSE=$(rpc_call "attach_config" '{
    "name": "../../../etc/passwd",
    "content": "malicious",
    "encoding": "raw"
}')

log_info "Response: $(echo "$RESPONSE" | jq -c '.')"

if echo "$RESPONSE" | jq -e '.error' >/dev/null 2>&1; then
    ERROR_MSG=$(echo "$RESPONSE" | jq -r '.error.message')
    if echo "$ERROR_MSG" | grep -qi "traversal\|invalid\|path"; then
        log_pass "Path traversal blocked correctly"
    else
        log_fail "Path traversal blocked but with unexpected error: $ERROR_MSG"
    fi
else
    log_fail "Path traversal NOT blocked - security issue!"
fi

# ============================================================================
# TEST 9: Absolute Path Rejection
# ============================================================================
log_test "9" "Absolute Path Rejection"

RESPONSE=$(rpc_call "attach_config" '{
    "name": "/absolute/path.env",
    "content": "test",
    "encoding": "raw"
}')

log_info "Response: $(echo "$RESPONSE" | jq -c '.')"

if echo "$RESPONSE" | jq -e '.error' >/dev/null 2>&1; then
    log_pass "Absolute path rejected correctly"
else
    log_fail "Absolute path NOT rejected"
fi

# ============================================================================
# TEST 10: Missing Required Parameters
# ============================================================================
log_test "10" "Missing Required Parameters"

RESPONSE=$(rpc_call "attach_config" '{"name": "test.env"}')

log_info "Response: $(echo "$RESPONSE" | jq -c '.')"

if echo "$RESPONSE" | jq -e '.error' >/dev/null 2>&1; then
    log_pass "Missing parameters rejected correctly"
else
    log_fail "Missing parameters NOT rejected"
fi

# ============================================================================
# TEST 11: Config Overwrite
# ============================================================================
log_test "11" "Config Overwrite (Same Name)"

# First attachment
RESPONSE1=$(rpc_call "attach_config" '{
    "name": "test-overwrite.env",
    "content": "VERSION=1",
    "encoding": "raw"
}')

# Second attachment with same name
RESPONSE2=$(rpc_call "attach_config" '{
    "name": "test-overwrite.env",
    "content": "VERSION=2\nNEW_KEY=value",
    "encoding": "raw"
}')

CONFIG_PATH=$(echo "$RESPONSE2" | jq -r '.result.path')

if [ -f "$CONFIG_PATH" ]; then
    CONTENT=$(cat "$CONFIG_PATH")
    if echo "$CONTENT" | grep -q "VERSION=2" && echo "$CONTENT" | grep -q "NEW_KEY=value"; then
        log_pass "Config overwrite successful"
    else
        log_fail "Config not overwritten correctly"
    fi
else
    log_fail "Config file not found after overwrite"
fi

# ============================================================================
# TEST 12: Matrix Status (if available)
# ============================================================================
log_test "12" "Matrix Integration Status"

RESPONSE=$(rpc_call "matrix.status" "{}")
log_info "Response: $(echo "$RESPONSE" | jq -c '.')"

if echo "$RESPONSE" | jq -e '.result' >/dev/null 2>&1; then
    CONNECTED=$(echo "$RESPONSE" | jq -r '.result.connected // false')

    if [ "$CONNECTED" = "true" ]; then
        log_pass "Matrix is connected"
        log_info "User ID: $(echo "$RESPONSE" | jq -r '.result.user_id // "unknown"')"
    else
        log_skip "Matrix not connected (requires Matrix server)"
    fi
elif echo "$RESPONSE" | jq -e '.error.message | contains("not enabled")' >/dev/null 2>&1; then
    log_skip "Matrix not enabled in bridge configuration"
else
    log_skip "Matrix status unavailable"
fi

# ============================================================================
# TEST 13: Container Access (optional, requires Docker)
# ============================================================================
log_test "13" "Container Config Access"

if [ "$WITH_CONTAINER" = "--with-container" ]; then
    if command -v docker &>/dev/null; then
        # Create a test container
        CONTAINER_NAME="test-config-access-$$"

        docker run -d --name "$CONTAINER_NAME" \
            -v "${CONFIG_DIR}:/run/armorclaw/configs:ro" \
            alpine:latest sleep 60 >/dev/null 2>&1 || true

        if docker ps | grep -q "$CONTAINER_NAME"; then
            # Test config access from container
            if docker exec "$CONTAINER_NAME" ls /run/armorclaw/configs/ 2>/dev/null | grep -q "test-"; then
                log_pass "Container can access config directory"
            else
                log_fail "Container cannot access config directory"
            fi

            # Cleanup
            docker stop "$CONTAINER_NAME" >/dev/null 2>&1 || true
            docker rm "$CONTAINER_NAME" >/dev/null 2>&1 || true
        else
            log_skip "Could not start test container"
        fi
    else
        log_skip "Docker not available"
    fi
else
    log_skip "Container tests disabled (use --with-container to enable)"
fi

# ============================================================================
# TEST 14: Concurrent Config Attachments
# ============================================================================
log_test "14" "Concurrent Config Attachments"

# Attach multiple configs in parallel
for i in {1..5}; do
    rpc_call "attach_config" "{
        \"name\": \"test-concurrent-$i.env\",
        \"content\": \"INDEX=$i\",
        \"encoding\": \"raw\"
    }" &
done
wait

# Verify all configs were created
SUCCESS_COUNT=0
for i in {1..5}; do
    if [ -f "${CONFIG_DIR}/test-concurrent-$i.env" ]; then
        ((SUCCESS_COUNT++))
    fi
done

if [ $SUCCESS_COUNT -eq 5 ]; then
    log_pass "All 5 concurrent attachments successful"
else
    log_fail "Only $SUCCESS_COUNT/5 concurrent attachments succeeded"
fi

# ============================================================================
# Test Summary
# ============================================================================
echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║                      Test Summary                              ║"
echo "╠════════════════════════════════════════════════════════════════╣"
echo -e "║  ${GREEN}Passed: $TESTS_PASSED${NC}                                                        ║"
echo -e "║  ${RED}Failed: $TESTS_FAILED${NC}                                                        ║"
echo -e "║  ${YELLOW}Skipped: $TESTS_SKIPPED${NC}                                                       ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Exit with appropriate code
if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}Some tests failed. Please review the output above.${NC}"
    exit 1
else
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
fi
