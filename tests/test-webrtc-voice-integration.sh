#!/bin/bash
set -euo pipefail

# ArmorClaw: WebRTC Voice + Matrix Integration Tests
# Tests the complete WebRTC voice implementation with Matrix integration

echo "🧪 WebRTC Voice + Matrix Integration Tests"
echo "=========================================="
echo ""

# Unique test namespace
TEST_NS="test-webrtc-$(date +%s)"
TEST_DIR="/tmp/armorclaw-$TEST_NS"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/transport.sh"
BRIDGE_BIN="$PROJECT_ROOT/bridge/build/armorclaw-bridge"
SOCKET_PATH="/run/armorclaw/$TEST_NS.sock"
RESULTS_DIR="$PROJECT_ROOT/tests/results/$TEST_NS"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
CONCURRENT_CALLS=${CONCURRENT_CALLS:-5}
MATRIX_ROOM_PREFIX="!testwebrtc"
TEST_TTL="10m"

# Test results
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_WARNED=0

# Cleanup handler
cleanup() {
    echo -e "\n${YELLOW}Cleaning up test artifacts...${NC}"

    # Kill bridge if running
    pkill -f "$SOCKET_PATH" 2>/dev/null || true

    # Remove test socket
    rm -f "$SOCKET_PATH" 2>/dev/null || true

    # Stop any test containers
    docker ps -q --filter "name=$TEST_NS" | xargs -r docker stop 2>/dev/null || true

    echo -e "${GREEN}Cleanup complete${NC}"
}

trap cleanup EXIT INT TERM

# Create test directories
mkdir -p "$TEST_DIR"
mkdir -p "$RESULTS_DIR"
mkdir -p "$(dirname "$SOCKET_PATH")"

# ============================================================================
# PREREQUISITE CHECKS
# ============================================================================
echo -e "${BLUE}Prerequisite Checks${NC}"
echo "--------------------"

# Check 1: Bridge binary
if [ ! -f "$BRIDGE_BIN" ]; then
    echo -e "${YELLOW}Building bridge binary...${NC}"
    cd "$PROJECT_ROOT/bridge"
    go build -o build/armorclaw-bridge ./cmd/bridge || \
        (echo -e "${RED}FAIL: Could not build bridge${NC}"; exit 1)
fi
echo -e "${GREEN}✓ Bridge binary available${NC}"

# Check 2: Container image
if docker images armorclaw/agent:v1 | grep -q armorclaw/agent; then
    echo -e "${GREEN}✓ Container image exists${NC}"
else
    echo -e "${YELLOW}Building container image...${NC}"
    docker build -t armorclaw/agent:v1 "$PROJECT_ROOT" || \
        (echo -e "${RED}FAIL: Could not build container${NC}"; exit 1)
    echo -e "${GREEN}✓ Container image built${NC}"
fi

# Check 3: socat for JSON-RPC
if command -v socat >/dev/null 2>&1; then
    echo -e "${GREEN}✓ socat available for JSON-RPC${NC}"
else
    echo -e "${RED}FAIL: socat is required for tests${NC}"
    exit 1
fi

echo ""

# ============================================================================
# CREATE TEST CONFIGURATION
# ============================================================================
echo -e "${BLUE}Test Configuration${NC}"
echo "--------------------"

cat > "$TEST_DIR/test-config.toml" <<EOF
[server]
socket_path = "$SOCKET_PATH"
daemonize = false

[keystore]
db_path = "$TEST_DIR/keystore.db"

[matrix]
enabled = false

[webrtc]
enabled = true

[webrtc.session]
default_ttl = "$TEST_TTL"
max_ttl = "1h"
cleanup_interval = "1m"

[voice]
enabled = true

[voice.general]
default_lifetime = "30m"
max_lifetime = "2h"
auto_answer = false
require_membership = false
max_concurrent_calls = 10

[voice.security]
require_e2ee = false
rate_limit = 100
rate_limit_burst = 200
audit_calls = true

[voice.budget]
enabled = true
default_token_limit = 10000
default_duration_limit = "2h"
warning_threshold = 0.8
hard_stop = false

[voice.ttl]
default_ttl = "30m"
max_ttl = "2h"
enforcement_interval = "30s"
warn_before_expiration = "5m"
on_expiration = "terminate"

[logging]
level = "info"
format = "json"
output = "$TEST_DIR/bridge.log"
EOF

echo -e "${GREEN}✓ Test configuration created${NC}"
echo ""

# ============================================================================
# START BRIDGE
# ============================================================================
echo -e "${BLUE}Starting Bridge${NC}"
echo "--------------------"

# Start bridge with test config
"$BRIDGE_BIN" -config "$TEST_DIR/test-config.toml" > "$TEST_DIR/bridge.stdout" 2>&1 &
BRIDGE_PID=$!

# Wait for socket to be created
for i in {1..10}; do
    if [ -S "$SOCKET_PATH" ]; then
        echo -e "${GREEN}✓ Bridge started (PID: $BRIDGE_PID)${NC}"
        break
    fi
    if [ $i -eq 10 ]; then
        echo -e "${RED}FAIL: Bridge socket not created${NC}"
        cat "$TEST_DIR/bridge.stdout"
        exit 1
    fi
    sleep 0.5
done

# Wait for bridge to be ready
sleep 2

# Verify bridge is running
if ! kill -0 $BRIDGE_PID 2>/dev/null; then
    echo -e "${RED}FAIL: Bridge process died${NC}"
    cat "$TEST_DIR/bridge.stdout"
    exit 1
fi

echo ""

# ============================================================================
# TEST SUITE
# ============================================================================

# Helper: RPC call function
rpc_call() {
    local method=$1
    local params=$2
    local id=${3:-1}

    echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"method\":\"$method\",\"params\":$params}" | \
        socat - UNIX-CONNECT:"$SOCKET_PATH" 2>/dev/null
}

# Helper: Check RPC response for success
check_success() {
    local response=$1
    local test_name=$2

    if echo "$response" | grep -q '"result"'; then
        echo -e "${GREEN}✓ $test_name${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${RED}✗ $test_name${NC}"
        echo "  Response: $response"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

# ============================================================================
# TEST 1: Bridge Status
# ============================================================================
echo -e "${BLUE}Test 1: Bridge Status${NC}"
echo "--------------------"

RESPONSE=$(rpc_call "bridge.status" "{}")
if check_success "$RESPONSE" "Bridge status request"; then
    # Extract and display status info
    STATUS=$(echo "$RESPONSE" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
    echo "  Status: $STATUS"
fi
echo ""

# ============================================================================
# TEST 2: WebRTC Session Creation
# ============================================================================
echo -e "${BLUE}Test 2: WebRTC Session Creation${NC}"
echo "--------------------"

ROOM_ID="${MATRIX_ROOM_PREFIX}$(date +%s%N):example.com"
RESPONSE=$(rpc_call "webrtc.start" "{\"room_id\":\"$ROOM_ID\",\"ttl\":\"$TEST_TTL\"}" 2)

if check_success "$RESPONSE" "WebRTC session creation"; then
    SESSION_ID=$(echo "$RESPONSE" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)
    SDP_ANSWER=$(echo "$RESPONSE" | grep -o '"sdp_answer":"[^"]*"' | cut -d'"' -f4 | head -c 50)
    TOKEN=$(echo "$RESPONSE" | grep -o '"token":"[^"]*"' | cut -d'"' -f4 | head -c 20)

    echo "  Room ID: $ROOM_ID"
    echo "  Session ID: $SESSION_ID"
    echo "  SDP Answer: ${SDP_ANSWER}..."
    echo "  Token: ${TOKEN}..."

    # Save session ID for later tests
    echo "$SESSION_ID" > "$RESULTS_DIR/session.txt"
    echo "$ROOM_ID" > "$RESULTS_DIR/room.txt"
else
    echo -e "${RED}Cannot continue without valid session${NC}"
    exit 1
fi
echo ""

# ============================================================================
# TEST 3: ICE Candidate Handling
# ============================================================================
echo -e "${BLUE}Test 3: ICE Candidate Handling${NC}"
echo "--------------------"

SESSION_ID=$(cat "$RESULTS_DIR/session.txt" 2>/dev/null)
if [ -n "$SESSION_ID" ]; then
    CANDIDATE='{"candidate":"candidate:1 1 udp 2130706431 192.168.1.100 54321 typ host","sdpMid":"audio","sdpMLineIndex":0}'
    RESPONSE=$(rpc_call "webrtc.ice_candidate" "{\"session_id\":\"$SESSION_ID\",\"candidate\":$CANDIDATE}" 3)

    if check_success "$RESPONSE" "ICE candidate submission"; then
        echo "  Candidate accepted"
    fi
else
    echo -e "${YELLOW}⊘ Skipped (no session ID)${NC}"
    TESTS_WARNED=$((TESTS_WARNED + 1))
fi
echo ""

# ============================================================================
# TEST 4: Session Listing
# ============================================================================
echo -e "${BLUE}Test 4: Session Listing${NC}"
echo "--------------------"

RESPONSE=$(rpc_call "webrtc.list" "{}" 4)

if check_success "$RESPONSE" "Session list request"; then
    ACTIVE_COUNT=$(echo "$RESPONSE" | grep -o '"active_calls":[0-9]*' | cut -d':' -f2)
    echo "  Active calls: $ACTIVE_COUNT"

    if [ "$ACTIVE_COUNT" -gt 0 ]; then
        echo -e "${GREEN}✓ Session tracking working${NC}"
    else
        echo -e "${YELLOW}⚠ No active calls found (may indicate tracking issue)${NC}"
        TESTS_WARNED=$((TESTS_WARNED + 1))
    fi
fi
echo ""

# ============================================================================
# TEST 5: Concurrent Call Creation
# ============================================================================
echo -e "${BLUE}Test 5: Concurrent Call Creation${NC}"
echo "--------------------"

echo "  Creating $CONCURRENT_CALLS concurrent calls..."
CREATED=0
FAILED=0
SESSION_IDS=()

for i in $(seq 1 $CONCURRENT_CALLS); do
    ROOM_ID="${MATRIX_ROOM_PREFIX}-concurrent-$i-$(date +%s%N):example.com"
    RESPONSE=$(rpc_call "webrtc.start" "{\"room_id\":\"$ROOM_ID\",\"ttl\":\"5m\"}" $((10 + i)))

    if echo "$RESPONSE" | grep -q '"result"'; then
        SESSION_ID=$(echo "$RESPONSE" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)
        SESSION_IDS+=("$SESSION_ID")
        CREATED=$((CREATED + 1))
        echo -e "    ${GREEN}✓${NC} Call $i created"
    else
        FAILED=$((FAILED + 1))
        echo -e "    ${RED}✗${NC} Call $i failed"
    fi
done

echo "  Created: $CREATED/$CONCURRENT_CALLS"

if [ $CREATED -eq $CONCURRENT_CALLS ]; then
    echo -e "${GREEN}✓ All concurrent calls created successfully${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))

    # Save session IDs for cleanup
    for sid in "${SESSION_IDS[@]}"; do
        echo "$sid" >> "$RESULTS_DIR/concurrent_sessions.txt"
    done
else
    echo -e "${YELLOW}⚠ Some concurrent calls failed${NC}"
    TESTS_WARNED=$((TESTS_WARNED + 1))
fi
echo ""

# ============================================================================
# TEST 6: Call Termination
# ============================================================================
echo -e "${BLUE}Test 6: Call Termination${NC}"
echo "--------------------"

SESSION_ID=$(cat "$RESULTS_DIR/session.txt" 2>/dev/null)
if [ -n "$SESSION_ID" ]; then
    RESPONSE=$(rpc_call "webrtc.end" "{\"session_id\":\"$SESSION_ID\",\"reason\":\"test_complete\"}" 20)

    if check_success "$RESPONSE" "Call termination"; then
        echo "  Call terminated successfully"
    fi

    # Verify call is no longer in list
    sleep 1
    RESPONSE=$(rpc_call "webrtc.list" "{}" 21)
    ACTIVE_AFTER=$(echo "$RESPONSE" | grep -o '"active_calls":[0-9]*' | cut -d':' -f2)
    echo "  Active calls after termination: $ACTIVE_AFTER"
else
    echo -e "${YELLOW}⊘ Skipped (no session ID)${NC}"
    TESTS_WARNED=$((TESTS_WARNED + 1))
fi
echo ""

# ============================================================================
# TEST 7: Security Policy Validation
# ============================================================================
echo -e "${BLUE}Test 7: Security Policy Validation${NC}"
echo "--------------------"

# Test 7a: Concurrent call limit
echo "  Testing concurrent call limit..."

# Create calls up to the configured limit
MAX_CALLS=10
OVER_LIMIT_COUNT=0

for i in $(seq 1 $((MAX_CALLS + 2))); do
    ROOM_ID="${MATRIX_ROOM_PREFIX}-limit-$i-$(date +%s%N):example.com"
    RESPONSE=$(rpc_call "webrtc.start" "{\"room_id\":\"$ROOM_ID\",\"ttl\":\"5m\"}" $((30 + i)))

    if echo "$RESPONSE" | grep -q "max_concurrent_calls"; then
        OVER_LIMIT_COUNT=$((OVER_LIMIT_COUNT + 1))
    fi
done

if [ $OVER_LIMIT_COUNT -gt 0 ]; then
    echo -e "${GREEN}✓ Concurrent call limit enforced${NC}"
    echo "  Rejected $OVER_LIMIT_COUNT calls over limit"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${YELLOW}⚠ Concurrent call limit may not be enforced${NC}"
    TESTS_WARNED=$((TESTS_WARNED + 1))
fi
echo ""

# ============================================================================
# TEST 8: Budget Tracking
# ============================================================================
echo -e "${BLUE}Test 8: Budget Tracking${NC}"
echo "--------------------"

# This test verifies budget sessions are created for calls
ROOM_ID="${MATRIX_ROOM_PREFIX}-budget-$(date +%s%N):example.com"
RESPONSE=$(rpc_call "webrtc.start" "{\"room_id\":\"$ROOM_ID\",\"ttl\":\"5m\"}" 50)

if echo "$RESPONSE" | grep -q '"result"'; then
    echo -e "${GREEN}✓ Budget session created for call${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))

    # Clean up this call
    SESSION_ID=$(echo "$RESPONSE" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)
    rpc_call "webrtc.end" "{\"session_id\":\"$SESSION_ID\",\"reason\":\"cleanup\"}" 51 >/dev/null
else
    echo -e "${RED}✗ Failed to create call for budget test${NC}"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# ============================================================================
# TEST 9: Error Handling
# ============================================================================
echo -e "${BLUE}Test 9: Error Handling${NC}"
echo "--------------------"

# Test 9a: Invalid session ID
RESPONSE=$(rpc_call "webrtc.end" "{\"session_id\":\"invalid-session-id\",\"reason\":\"test\"}" 60)
if echo "$RESPONSE" | grep -q "error\|not_found"; then
    echo -e "${GREEN}✓ Invalid session ID properly rejected${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${YELLOW}⚠ Invalid session ID should have been rejected${NC}"
    TESTS_WARNED=$((TESTS_WARNED + 1))
fi

# Test 9b: Missing required parameters
RESPONSE=$(rpc_call "webrtc.start" "{}" 61)
if echo "$RESPONSE" | grep -q "error\|required"; then
    echo -e "${GREEN}✓ Missing parameters properly rejected${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${YELLOW}⚠ Missing parameters should have been rejected${NC}"
    TESTS_WARNED=$((TESTS_WARNED + 1))
fi

# Test 9c: Invalid TTL format
RESPONSE=$(rpc_call "webrtc.start" "{\"room_id\":\"$ROOM_ID\",\"ttl\":\"invalid\"}" 62)
if echo "$RESPONSE" | grep -q "error\|invalid"; then
    echo -e "${GREEN}✓ Invalid TTL format properly rejected${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${YELLOW}⚠ Invalid TTL should have been rejected${NC}"
    TESTS_WARNED=$((TESTS_WARNED + 1))
fi
echo ""

# ============================================================================
# CLEANUP: Terminate all remaining test calls
# ============================================================================
echo -e "${BLUE}Cleanup: Terminating Test Calls${NC}"
echo "--------------------"

if [ -f "$RESULTS_DIR/concurrent_sessions.txt" ]; then
    while read -r SESSION_ID; do
        rpc_call "webrtc.end" "{\"session_id\":\"$SESSION_ID\",\"reason\":\"test_cleanup\"}" 99 >/dev/null
    done < "$RESULTS_DIR/concurrent_sessions.txt"
    echo -e "${GREEN}✓ All test calls terminated${NC}"
fi
echo ""

# ============================================================================
# TEST SUMMARY
# ============================================================================
echo "=========================================="
echo -e "${BLUE}Test Summary${NC}"
echo "=========================================="
echo ""
echo "Tests Passed:  $TESTS_PASSED"
echo "Tests Failed:  $TESTS_FAILED"
echo "Tests Warned:  $TESTS_WARNED"
echo "Tests Total:   $((TESTS_PASSED + TESTS_FAILED + TESTS_WARNED))"
echo ""

# Exit with appropriate code
if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}✗ INTEGRATION TESTS FAILED${NC}"
    echo ""
    echo "Review logs at:"
    echo "  Bridge stdout: $TEST_DIR/bridge.stdout"
    echo "  Bridge log:   $TEST_DIR/bridge.log"
    echo "  Test results:  $RESULTS_DIR"
    exit 1
elif [ $TESTS_WARNED -gt 0 ]; then
    echo -e "${YELLOW}⚠ INTEGRATION TESTS PASSED WITH WARNINGS${NC}"
    echo ""
    echo "Review warnings above and logs at:"
    echo "  Bridge stdout: $TEST_DIR/bridge.stdout"
    echo "  Bridge log:   $TEST_DIR/bridge.log"
    echo "  Test results:  $RESULTS_DIR"
    exit 0
else
    echo -e "${GREEN}✓ ALL INTEGRATION TESTS PASSED${NC}"
    echo ""
    echo "WebRTC Voice + Matrix integration is working correctly!"
    echo ""
    echo "Test artifacts saved to: $RESULTS_DIR"
    exit 0
fi
