#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# Governance RPC Integration Tests
#
# Tests all governance methods (device.*, invite.*) against a locally-built
# bridge binary via Unix socket + socat.
#
# Usage:  bash tests/test-governance-rpc.sh
# Requires: go, socat, gcc (for CGO/SQLCipher)
# ──────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/transport.sh"
BRIDGE_BIN="${SCRIPT_DIR}/../bridge/build/armorclaw-bridge"
SOCKET_PATH="/tmp/bridge-gov-test-$$.sock"
KEYSTORE_DIR="/tmp/armorclaw-gov-keystore-$$"
CONFIG_FILE="/tmp/armorclaw-gov-config-$$.toml"
BRIDGE_PID=""

# ── Counters ──────────────────────────────────────────────────────────────────
TOTAL=0 PASSED=0 FAILED=0
G1_TOTAL=0 G1_PASS=0  # Registration
G2_TOTAL=0 G2_PASS=0  # Invites happy path
G3_TOTAL=0 G3_PASS=0  # Parameter validation
G4_TOTAL=0 G4_PASS=0  # Protocol errors
G5_TOTAL=0 G5_PASS=0  # Edge cases
FAILURES=""

# ── Cleanup ───────────────────────────────────────────────────────────────────
cleanup() {
    if [[ -n "$BRIDGE_PID" ]]; then
        kill "$BRIDGE_PID" 2>/dev/null || true
        sleep 2
        kill -9 "$BRIDGE_PID" 2>/dev/null || true
    fi
    rm -f "$SOCKET_PATH" "$CONFIG_FILE" 2>/dev/null || true
    rm -rf "$KEYSTORE_DIR" 2>/dev/null || true
}
trap cleanup EXIT

# ── Temp config & keystore ────────────────────────────────────────────────────
mkdir -p "$KEYSTORE_DIR"

cat > "$CONFIG_FILE" << EOF
[keystore]
db_path = "$KEYSTORE_DIR/keystore.db"

[server]
socket_path = "$SOCKET_PATH"

[error_system]
enabled = false
store_enabled = false

[discovery]
enabled = false
EOF

# ── Dependency check ──────────────────────────────────────────────────────────
SOCKET_CMD=""
if command -v socat &>/dev/null; then
    SOCKET_CMD="socat"
elif nc -h 2>&1 | grep -q -- '-U'; then
    SOCKET_CMD="nc"
else
    echo "ERROR: socat or nc (with -U) not installed."
    exit 1
fi

# ── Build bridge ──────────────────────────────────────────────────────────────
if [[ ! -x "$BRIDGE_BIN" ]]; then
    echo "Building bridge binary..."
    (cd "${SCRIPT_DIR}/../bridge" && go build -o build/armorclaw-bridge ./cmd/bridge)
fi

# ── Start bridge ──────────────────────────────────────────────────────────────
echo "Starting bridge..."
ARMORCLAW_ERRORS_STORE_PATH="$KEYSTORE_DIR/errors.db" \
ARMORCLAW_SKIP_DOCKER_CHECK=1 \
setsid "$BRIDGE_BIN" --config "$CONFIG_FILE" >/tmp/bridge-gov-$$.log 2>&1 &
BRIDGE_PID=$!

echo "Waiting for socket..."
for i in {1..30}; do
    if [[ -S "$SOCKET_PATH" ]]; then
        break
    fi
    sleep 0.5
done

if [[ ! -S "$SOCKET_PATH" ]]; then
    echo "ERROR: Socket not created after 15 seconds"
    exit 1
fi
echo "Socket ready at $SOCKET_PATH"

# ── RPC helper ────────────────────────────────────────────────────────────────
rpc_call() {
    local method="$1"
    local params="${2:-{\}}"
    local timeout_sec="${3:-5}"
    local payload="{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$method\",\"params\":$params}"

    if [[ "$SOCKET_CMD" == "socat" ]]; then
        timeout "$timeout_sec" bash -c \
            "echo '$payload' | socat - UNIX-CONNECT:$SOCKET_PATH" 2>/dev/null || echo ""
    else
        timeout "$timeout_sec" bash -c \
            "printf '%s\n' '$payload' | nc -w 2 -U $SOCKET_PATH" 2>/dev/null || echo ""
    fi
}

raw_rpc() {
    local payload="$1"
    local timeout_sec="${2:-5}"

    if [[ "$SOCKET_CMD" == "socat" ]]; then
        timeout "$timeout_sec" bash -c \
            "echo '$payload' | socat - UNIX-CONNECT:$SOCKET_PATH" 2>/dev/null || echo ""
    else
        timeout "$timeout_sec" bash -c \
            "printf '%s\n' '$payload' | nc -q1 -U $SOCKET_PATH" 2>/dev/null || echo ""
    fi
}

# ── Test runner ───────────────────────────────────────────────────────────────
run_test() {
    local name="$1" method="$2" params="$3" expected="$4"
    TOTAL=$((TOTAL + 1))
    local result
    result=$(rpc_call "$method" "$params")
    if echo "$result" | grep -q "$expected"; then
        PASSED=$((PASSED + 1))
        echo "  [PASS] $name"
    else
        FAILED=$((FAILED + 1))
        FAILURES="$FAILURES\n  - $name: expected '$expected' in: $(echo "$result" | head -c 200)"
        echo "  [FAIL] $name"
        echo "    Expected: $expected"
        echo "    Got: $(echo "$result" | head -c 200)"
    fi
}

run_group_test() {
    local group="$1" name="$2" method="$3" params="$4" expected="$5"
    run_test "$name" "$method" "$params" "$expected"
    case "$group" in
        1) G1_TOTAL=$((G1_TOTAL + 1)) ;;
        2) G2_TOTAL=$((G2_TOTAL + 1)) ;;
        3) G3_TOTAL=$((G3_TOTAL + 1)) ;;
        4) G4_TOTAL=$((G4_TOTAL + 1)) ;;
        5) G5_TOTAL=$((G5_TOTAL + 1)) ;;
    esac
}

# Track pass per group — must be called right after run_test when expected passed
# We simply recount from PASSED vs group totals at the end; but for live tracking:
pass_group() {
    case "$1" in
        1) G1_PASS=$((G1_PASS + 1)) ;;
        2) G2_PASS=$((G2_PASS + 1)) ;;
        3) G3_PASS=$((G3_PASS + 1)) ;;
        4) G4_PASS=$((G4_PASS + 1)) ;;
        5) G5_PASS=$((G5_PASS + 1)) ;;
    esac
}

# Convenience: run_test that also tracks group pass
gt() {
    local group="$1" name="$2" method="$3" params="$4" expected="$5"
    local prev_failed=$FAILED
    run_group_test "$group" "$name" "$method" "$params" "$expected"
    # If FAILED didn't increase, it passed
    if [[ $FAILED -eq $prev_failed ]]; then
        pass_group "$group"
    fi
}

# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " GROUP 1: Method Registration (8 tests)"
echo "========================================="

gt 1 "device.list returns result"       "device.list"    '{}'        '"result"'
gt 1 "device.get empty returns error"   "device.get"     '{}'        '"error"'
gt 1 "device.approve empty returns error" "device.approve" '{}'      '"error"'
gt 1 "device.reject empty returns error" "device.reject"  '{}'       '"error"'
gt 1 "invite.list returns result"        "invite.list"    '{}'       '"result"'
gt 1 "invite.create valid returns result" "invite.create" '{"role":"admin","expiration":"1d","max_uses":5,"created_by":"test-admin"}' '"code"'
gt 1 "invite.revoke empty returns error" "invite.revoke"  '{}'       '"error"'
gt 1 "invite.validate empty returns error" "invite.validate" '{}'     '"error"'

# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " GROUP 2: Invite Happy Path (5 tests)"
echo "========================================="

# Step 1: Create invite and capture id + code
CREATE_RESULT=$(rpc_call "invite.create" '{"role":"admin","expiration":"1d","max_uses":5,"created_by":"test-admin"}')
INVITE_ID=$(echo "$CREATE_RESULT" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
INVITE_CODE=$(echo "$CREATE_RESULT" | grep -o '"code":"[^"]*"' | head -1 | cut -d'"' -f4)

echo "  Created invite: id=$INVITE_ID code=$INVITE_CODE"

# Step 2: Validate the active code
G2_TOTAL=$((G2_TOTAL + 1)); TOTAL=$((TOTAL + 1))
VALIDATE_RESULT=$(rpc_call "invite.validate" "{\"code\":\"$INVITE_CODE\"}")
if echo "$VALIDATE_RESULT" | grep -q '"status":"active"'; then
    PASSED=$((PASSED + 1)); G2_PASS=$((G2_PASS + 1))
    echo "  [PASS] invite.validate returns active"
else
    FAILED=$((FAILED + 1))
    FAILURES="$FAILURES\n  - invite.validate active: expected 'active' in: $(echo "$VALIDATE_RESULT" | head -c 200)"
    echo "  [FAIL] invite.validate returns active"
    echo "    Expected: active"
    echo "    Got: $(echo "$VALIDATE_RESULT" | head -c 200)"
fi

# Step 3: List invites contains our invite id
G2_TOTAL=$((G2_TOTAL + 1)); TOTAL=$((TOTAL + 1))
LIST_RESULT=$(rpc_call "invite.list" '{}')
if echo "$LIST_RESULT" | grep -q "$INVITE_ID"; then
    PASSED=$((PASSED + 1)); G2_PASS=$((G2_PASS + 1))
    echo "  [PASS] invite.list contains created invite_id"
else
    FAILED=$((FAILED + 1))
    FAILURES="$FAILURES\n  - invite.list contains id: expected '$INVITE_ID' in: $(echo "$LIST_RESULT" | head -c 200)"
    echo "  [FAIL] invite.list contains created invite_id"
    echo "    Expected: $INVITE_ID"
    echo "    Got: $(echo "$LIST_RESULT" | head -c 200)"
fi

# Step 4: Revoke the invite
G2_TOTAL=$((G2_TOTAL + 1)); TOTAL=$((TOTAL + 1))
REVOKE_RESULT=$(rpc_call "invite.revoke" "{\"invite_id\":\"$INVITE_ID\",\"revoked_by\":\"test-admin\"}")
if echo "$REVOKE_RESULT" | grep -q '"success"'; then
    PASSED=$((PASSED + 1)); G2_PASS=$((G2_PASS + 1))
    echo "  [PASS] invite.revoke returns success"
else
    FAILED=$((FAILED + 1))
    FAILURES="$FAILURES\n  - invite.revoke: expected 'success' in: $(echo "$REVOKE_RESULT" | head -c 200)"
    echo "  [FAIL] invite.revoke returns success"
    echo "    Expected: success"
    echo "    Got: $(echo "$REVOKE_RESULT" | head -c 200)"
fi

# Step 5: Validate revoked code returns error with "revoked"
G2_TOTAL=$((G2_TOTAL + 1)); TOTAL=$((TOTAL + 1))
REVOKED_RESULT=$(rpc_call "invite.validate" "{\"code\":\"$INVITE_CODE\"}")
if echo "$REVOKED_RESULT" | grep -q "revoked"; then
    PASSED=$((PASSED + 1)); G2_PASS=$((G2_PASS + 1))
    echo "  [PASS] invite.validate revoked code returns revoked"
else
    FAILED=$((FAILED + 1))
    FAILURES="$FAILURES\n  - invite.validate revoked: expected 'revoked' in: $(echo "$REVOKED_RESULT" | head -c 200)"
    echo "  [FAIL] invite.validate revoked code returns revoked"
    echo "    Expected: revoked"
    echo "    Got: $(echo "$REVOKED_RESULT" | head -c 200)"
fi

# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " GROUP 3: Parameter Validation (~16 tests)"
echo "========================================="

# Device parameter validation
gt 3 "device.get no device_id" \
    "device.get" '{}' 'device_id is required'

gt 3 "device.get empty device_id" \
    "device.get" '{"device_id":""}' 'device_id is required'

gt 3 "device.get non-existent device" \
    "device.get" '{"device_id":"nonexistent-device-xyz"}' 'device not found'

gt 3 "device.approve no params" \
    "device.approve" '{}' 'device_id is required'

gt 3 "device.approve missing approved_by" \
    "device.approve" '{"device_id":"test-device-1"}' 'approved_by is required'

gt 3 "device.approve non-existent device (with approved_by)" \
    "device.approve" '{"device_id":"nonexistent-xyz","approved_by":"admin"}' 'device not found'

gt 3 "device.reject no params" \
    "device.reject" '{}' 'device_id is required'

gt 3 "device.reject empty device_id" \
    "device.reject" '{"device_id":""}' 'device_id is required'

# device.reject with non-existent device_id returns "device not found"
# (rejected_by is NOT validated — function goes straight to GetDevice)
gt 3 "device.reject non-existent device (no rejected_by check)" \
    "device.reject" '{"device_id":"nonexistent-device-xyz"}' 'device not found'

# Invite parameter validation
gt 3 "invite.create no role" \
    "invite.create" '{"expiration":"1d","created_by":"admin"}' 'role is required'

gt 3 "invite.create invalid role" \
    "invite.create" '{"role":"superuser","expiration":"1d","created_by":"admin"}' 'invalid role'

gt 3 "invite.create no expiration" \
    "invite.create" '{"role":"admin","created_by":"admin"}' 'expiration is required'

gt 3 "invite.create invalid expiration" \
    "invite.create" '{"role":"admin","expiration":"2w","created_by":"admin"}' 'unsupported expiration'

gt 3 "invite.create no created_by" \
    "invite.create" '{"role":"admin","expiration":"1d"}' 'created_by is required'

gt 3 "invite.revoke no invite_id" \
    "invite.revoke" '{"revoked_by":"admin"}' 'invite_id is required'

gt 3 "invite.validate no code" \
    "invite.validate" '{}' 'code is required'

gt 3 "invite.validate invalid code" \
    "invite.validate" '{"code":"totally-invalid-code-12345"}' 'invite not found'

# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " GROUP 4: Protocol Errors (4 tests)"
echo "========================================="

# Method not found
gt 4 "unknown method returns -32601" \
    "device.delete" '{}' '-32601'

# Wrong jsonrpc version
G4_TOTAL=$((G4_TOTAL + 1)); TOTAL=$((TOTAL + 1))
PROTO_RESULT=$(raw_rpc '{"jsonrpc":"1.0","id":1,"method":"device.list","params":{}}')
if echo "$PROTO_RESULT" | grep -q '\-32600'; then
    PASSED=$((PASSED + 1)); G4_PASS=$((G4_PASS + 1))
    echo "  [PASS] jsonrpc 1.0 returns -32600"
else
    FAILED=$((FAILED + 1))
    FAILURES="$FAILURES\n  - jsonrpc 1.0: expected '-32600' in: $(echo "$PROTO_RESULT" | head -c 200)"
    echo "  [FAIL] jsonrpc 1.0 returns -32600"
    echo "    Expected: -32600"
    echo "    Got: $(echo "$PROTO_RESULT" | head -c 200)"
fi

# Missing method field
G4_TOTAL=$((G4_TOTAL + 1)); TOTAL=$((TOTAL + 1))
PROTO_RESULT=$(raw_rpc '{"jsonrpc":"2.0","id":1,"params":{}}')
if echo "$PROTO_RESULT" | grep -q '\-32600'; then
    PASSED=$((PASSED + 1)); G4_PASS=$((G4_PASS + 1))
    echo "  [PASS] missing method returns -32600"
else
    FAILED=$((FAILED + 1))
    FAILURES="$FAILURES\n  - missing method: expected '-32600' in: $(echo "$PROTO_RESULT" | head -c 200)"
    echo "  [FAIL] missing method returns -32600"
    echo "    Expected: -32600"
    echo "    Got: $(echo "$PROTO_RESULT" | head -c 200)"
fi

# Invalid JSON
G4_TOTAL=$((G4_TOTAL + 1)); TOTAL=$((TOTAL + 1))
PROTO_RESULT=$(raw_rpc 'this is not json at all!!!')
if echo "$PROTO_RESULT" | grep -q '\-32700'; then
    PASSED=$((PASSED + 1)); G4_PASS=$((G4_PASS + 1))
    echo "  [PASS] invalid JSON returns -32700"
else
    FAILED=$((FAILED + 1))
    FAILURES="$FAILURES\n  - invalid JSON: expected '-32700' in: $(echo "$PROTO_RESULT" | head -c 200)"
    echo "  [FAIL] invalid JSON returns -32700"
    echo "    Expected: -32700"
    echo "    Got: $(echo "$PROTO_RESULT" | head -c 200)"
fi

# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " GROUP 5: Edge Cases (4 tests)"
echo "========================================="

# invite.create with max_uses:0 (unlimited uses)
gt 5 "invite.create max_uses:0 succeeds" \
    "invite.create" '{"role":"user","expiration":"1d","max_uses":0,"created_by":"edge-test"}' '"code"'

# invite.create with expiration:"never"
gt 5 "invite.create expiration:never succeeds" \
    "invite.create" '{"role":"user","expiration":"never","created_by":"edge-test"}' '"code"'

# invite.create with welcome_message
gt 5 "invite.create with welcome_message succeeds" \
    "invite.create" '{"role":"user","expiration":"1d","max_uses":1,"welcome_message":"Welcome to ArmorClaw!","created_by":"edge-test"}' '"code"'

# Validate already-revoked code returns revoked (reuse code from Group 2)
gt 5 "invite.validate revoked code (re-check)" \
    "invite.validate" "{\"code\":\"$INVITE_CODE\"}" 'revoked'

# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo "GOVERNANCE RPC TEST SUMMARY"
echo "========================================="
echo "Total: $TOTAL | Passed: $PASSED | Failed: $FAILED"
echo "Groups: Registration($G1_PASS/$G1_TOTAL) Invites($G2_PASS/$G2_TOTAL) Validation($G3_PASS/$G3_TOTAL) Protocol($G4_PASS/$G4_TOTAL) EdgeCases($G5_PASS/$G5_TOTAL)"
echo "========================================="
if [ -n "$FAILURES" ]; then
    echo "FAILURES:"
    echo -e "$FAILURES"
    echo "========================================="
fi

if [ "$FAILED" -gt 0 ]; then
    exit 1
fi
