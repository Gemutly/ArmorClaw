#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BRIDGE_BIN="${SCRIPT_DIR}/../bridge/build/armorclaw-bridge"
SOCKET_PATH="/tmp/bridge-test-$$.sock"
KEYSTORE_DIR="/tmp/armorclaw-keystore-$$"
CONFIG_FILE="/tmp/armorclaw-config-$$.toml"
BRIDGE_LOG="/tmp/bridge-test-$$.log"
BRIDGE_PID=""
FAILED=0

cleanup() {
    if [[ -n "$BRIDGE_PID" ]]; then
        # SIGTERM first, then SIGKILL after 2s — prevents CI hang on deadlocked Go shutdown
        kill "$BRIDGE_PID" 2>/dev/null || true
        sleep 2
        kill -9 "$BRIDGE_PID" 2>/dev/null || true
    fi
    rm -f "$SOCKET_PATH" "$CONFIG_FILE" 2>/dev/null || true
    rm -rf "$KEYSTORE_DIR" 2>/dev/null || true
}
trap cleanup EXIT

mkdir -p "$KEYSTORE_DIR"

cat > "$CONFIG_FILE" << EOF
[keystore]
db_path = "$KEYSTORE_DIR/keystore.db"

[server]
socket_path = "$SOCKET_PATH"

[error_system]
enabled = false
store_enabled = false
EOF

if ! command -v socat &>/dev/null; then
    echo "❌ socat not installed. Install with: apt-get install socat"
    exit 1
fi

if [[ ! -x "$BRIDGE_BIN" ]]; then    echo "Building bridge binary..."
    cd "${SCRIPT_DIR}/../bridge"
    go build -o build/armorclaw-bridge ./cmd/bridge
fi

echo "Starting bridge (log: $BRIDGE_LOG)..."
ARMORCLAW_ERRORS_STORE_PATH="$KEYSTORE_DIR/errors.db" \
ARMORCLAW_SKIP_DOCKER_CHECK=1 \
"$BRIDGE_BIN" --config "$CONFIG_FILE" > "$BRIDGE_LOG" 2>&1 &
BRIDGE_PID=$!

echo "Waiting for socket (PID=$BRIDGE_PID)..."
for i in {1..60}; do
    if [[ -S "$SOCKET_PATH" ]]; then
        break
    fi
    if ! kill -0 "$BRIDGE_PID" 2>/dev/null; then
        echo "❌ FAILED: Bridge process exited unexpectedly"
        echo "--- Bridge log ---"
        cat "$BRIDGE_LOG" 2>/dev/null || echo "(no log)"
        echo "--- End log ---"
        exit 1
    fi
    sleep 0.5
done

if [[ ! -S "$SOCKET_PATH" ]]; then
    echo "❌ FAILED: Socket not created after 30 seconds"
    echo "--- Bridge log (last 40 lines) ---"
    tail -40 "$BRIDGE_LOG" 2>/dev/null || echo "(no log)"
    echo "--- End log ---"
    exit 1
fi
echo "✅ Socket created at $SOCKET_PATH"

rpc_call() {
    local method="$1"
    local params="${2:-{\}}"
    local timeout="${3:-5}"
    
    timeout "$timeout" bash -c \
        "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$method\",\"params\":$params}' | \
         socat - UNIX-CONNECT:$SOCKET_PATH" 2>/dev/null || echo ""
}

# Test function
test_method() {
    local method="$1"
    local expected="$2"
    local params="${3:-{}}"
    
    result=$(rpc_call "$method" "$params")
    
    if [[ -z "$result" ]]; then
        echo "❌ FAILED: $method - no response"
        return 1
    fi
    
    if echo "$result" | tr -d '\n' | grep -q "$expected"; then
        echo "✅ PASSED: $method"
        return 0
    else
        echo "❌ FAILED: $method - expected '$expected' in response"
        echo "   Got: $result"
        return 1
    fi
}

echo ""
echo "Testing critical RPC methods..."
echo ""

# Critical RPC methods to test
# Format: "method" "expected_in_response" "params"
test_method "health.check" '"status"' || ((FAILED++)) || true

# Test error handling
echo ""
echo "Testing error handling..."

# Invalid method should return error code -32601
test_method "invalid.method" '"code": -32601' || ((FAILED++)) || true

# Summary
echo ""
echo "========================================"
if [[ $FAILED -eq 0 ]]; then
    echo "🎉 All integration tests passed"
    exit 0
else
    echo "❌ $FAILED test(s) failed"
    exit 1
fi
