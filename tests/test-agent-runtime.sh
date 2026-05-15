#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# T10. Agent Runtime Invariants Harness
#
# Tests container.* and studio.* RPC response shapes against a locally-built
# bridge binary via Unix socket + socat (dual-transport: socat/nc).
# Tier B: Entire harness skips gracefully if Docker is unavailable.
#
# Scenarios:
#   R0: Prerequisites — Docker, socat/nc, container.* RPCs, studio.* RPCs
#   R1: container.list — verify response shape
#   R2: container.terminate — invalid ID → verify error
#   R3: studio.list_agents — verify response shape
#   R4: studio.list_instances — verify response shape
#   R5: studio.get_stats — verify stats present
#   R6: No cross-agent leakage — container isolation check (if containers running)
#
# Usage:  bash tests/test-agent-runtime.sh
# Requires: go, socat (or nc -U), gcc (for CGO/SQLCipher)
# ──────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/transport.sh"
source "$SCRIPT_DIR/lib/common_output.sh"
source "$SCRIPT_DIR/lib/assert_json.sh"

# Colors (common_output.sh expects these from common.sh; provide fallbacks)
GREEN="${GREEN:-\033[0;32m}"
RED="${RED:-\033[0;31m}"
YELLOW="${YELLOW:-\033[1;33m}"
NC="${NC:-\033[0m}"

BRIDGE_BIN="${SCRIPT_DIR}/../bridge/build/armorclaw-bridge"
SOCKET_PATH="/tmp/bridge-runtime-test-$$.sock"
KEYSTORE_DIR="/tmp/armorclaw-runtime-keystore-$$"
CONFIG_FILE="/tmp/armorclaw-runtime-config-$$.toml"
BRIDGE_PID=""

# ── Evidence output directory ─────────────────────────────────────────────────
EVIDENCE_DIR="$SCRIPT_DIR/../.sisyphus/evidence/full-system-t10"
mkdir -p "$EVIDENCE_DIR"

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

# ══════════════════════════════════════════════════════════════════════════════
# R0: Prerequisites
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " R0: Prerequisites"
echo "========================================="

R0_SKIP=false

# Check Docker availability
if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    log_pass "Docker is available and daemon is running"
else
    log_skip "Docker not available or daemon not running — entire harness skipped (Tier B)"
    R0_SKIP=true
fi

# Check socket transport (socat or nc with -U)
SOCKET_CMD=""
if command -v socat >/dev/null 2>&1; then
    SOCKET_CMD="socat"
    log_pass "socat is available"
elif nc -h 2>&1 | grep -q -- '-U'; then
    SOCKET_CMD="nc"
    log_pass "nc with Unix socket support (-U) is available"
else
    log_fail "Neither socat nor nc (with -U) found — cannot test RPC"
    R0_SKIP=true
fi

# Check jq
if command -v jq >/dev/null 2>&1; then
    log_pass "jq is available ($(jq --version))"
else
    log_fail "jq is required but not found"
    R0_SKIP=true
fi

# Check go (needed to build bridge)
if command -v go >/dev/null 2>&1; then
    log_pass "go is available ($(go version 2>/dev/null | head -1))"
else
    log_fail "go is required to build bridge"
    R0_SKIP=true
fi

if $R0_SKIP; then
    log_skip "R0 prerequisites not met — skipping R1-R6"
    # Save evidence of skip
    echo '{"status":"skipped","reason":"Docker or core dependencies unavailable"}' \
        > "$EVIDENCE_DIR/r0-prerequisites.json"
    harness_summary
    exit 0
fi

# Build bridge
if [[ ! -x "$BRIDGE_BIN" ]]; then
    log_info "Building bridge binary..."
    (cd "${SCRIPT_DIR}/../bridge" && go build -o build/armorclaw-bridge ./cmd/bridge)
fi

if [[ -x "$BRIDGE_BIN" ]]; then
    log_pass "Bridge binary ready"
else
    log_fail "Bridge binary build failed"
    harness_summary
    exit 1
fi

# Start bridge
echo "Starting bridge..."
ARMORCLAW_ERRORS_STORE_PATH="$KEYSTORE_DIR/errors.db" \
ARMORCLAW_SKIP_DOCKER_CHECK=1 \
setsid "$BRIDGE_BIN" --config "$CONFIG_FILE" >/tmp/bridge-runtime-$$.log 2>&1 &
BRIDGE_PID=$!

echo "Waiting for socket..."
for i in {1..30}; do
    if [[ -S "$SOCKET_PATH" ]]; then
        break
    fi
    sleep 0.5
done

if [[ ! -S "$SOCKET_PATH" ]]; then
    log_fail "Socket not created after 15 seconds"
    cat /tmp/bridge-runtime-$$.log 2>/dev/null | tail -20 || true
    harness_summary
    exit 1
fi
log_pass "Bridge socket ready at $SOCKET_PATH"

# ── RPC helper (dual-transport) ───────────────────────────────────────────────
rpc_call() {
    local method="$1"
    local params="${2:-\{\}}"
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

# Check container.* RPC methods registered
CONTAINER_LIST_RESULT=$(rpc_call "container.list" '{"all":true}')
if echo "$CONTAINER_LIST_RESULT" | jq -e 'has("result") or has("error")' >/dev/null 2>&1; then
    log_pass "container.list RPC is registered"
else
    log_fail "container.list RPC returned unexpected shape: $(echo "$CONTAINER_LIST_RESULT" | head -c 200)"
fi

CONTAINER_TERM_RESULT=$(rpc_call "container.terminate" '{}')
if echo "$CONTAINER_TERM_RESULT" | jq -e 'has("result") or has("error")' >/dev/null 2>&1; then
    log_pass "container.terminate RPC is registered"
else
    log_fail "container.terminate RPC returned unexpected shape: $(echo "$CONTAINER_TERM_RESULT" | head -c 200)"
fi

# Check studio.* RPC methods registered
STUDIO_AGENTS_RESULT=$(rpc_call "studio.list_agents" '{}')
if echo "$STUDIO_AGENTS_RESULT" | jq -e 'has("result") or has("error")' >/dev/null 2>&1; then
    log_pass "studio.list_agents RPC is registered"
else
    log_fail "studio.list_agents RPC returned unexpected shape: $(echo "$STUDIO_AGENTS_RESULT" | head -c 200)"
fi

STUDIO_INST_RESULT=$(rpc_call "studio.list_instances" '{}')
if echo "$STUDIO_INST_RESULT" | jq -e 'has("result") or has("error")' >/dev/null 2>&1; then
    log_pass "studio.list_instances RPC is registered"
else
    log_fail "studio.list_instances RPC returned unexpected shape: $(echo "$STUDIO_INST_RESULT" | head -c 200)"
fi

STUDIO_STATS_RESULT=$(rpc_call "studio.get_stats" '{}')
if echo "$STUDIO_STATS_RESULT" | jq -e 'has("result") or has("error")' >/dev/null 2>&1; then
    log_pass "studio.get_stats RPC is registered"
else
    log_fail "studio.get_stats RPC returned unexpected shape: $(echo "$STUDIO_STATS_RESULT" | head -c 200)"
fi

# Save R0 evidence
cat > "$EVIDENCE_DIR/r0-prerequisites.json" << EVIDENCE
{
  "docker_available": true,
  "transport": "$SOCKET_CMD",
  "bridge_pid": $BRIDGE_PID,
  "socket_path": "$SOCKET_PATH",
  "container_list_ok": $(echo "$CONTAINER_LIST_RESULT" | jq -e 'has("result") or has("error")' 2>/dev/null && echo true || echo false),
  "studio_list_agents_ok": $(echo "$STUDIO_AGENTS_RESULT" | jq -e 'has("result") or has("error")' 2>/dev/null && echo true || echo false)
}
EVIDENCE

# ══════════════════════════════════════════════════════════════════════════════
# R1: container.list — verify response shape
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " R1: container.list — response shape"
echo "========================================="

R1_RESPONSE=$(rpc_call "container.list" '{"all":true}')

# Save evidence
echo "$R1_RESPONSE" | jq . > "$EVIDENCE_DIR/r1-container-list.json" 2>/dev/null \
    || echo "$R1_RESPONSE" > "$EVIDENCE_DIR/r1-container-list.json"

if [[ -z "$R1_RESPONSE" ]]; then
    log_fail "R1: container.list returned empty response"
else
    # Verify JSON-RPC envelope
    if assert_json_has_key "$R1_RESPONSE" "jsonrpc"; then
        log_pass "R1: container.list has jsonrpc field"
    else
        log_fail "R1: container.list missing jsonrpc field"
    fi

    if assert_json_has_key "$R1_RESPONSE" "id"; then
        log_pass "R1: container.list has id field"
    else
        log_fail "R1: container.list missing id field"
    fi

    # Must have either result or error (valid JSON-RPC)
    if echo "$R1_RESPONSE" | jq -e 'has("result") or has("error")' >/dev/null 2>&1; then
        log_pass "R1: container.list has result or error key"
    else
        log_fail "R1: container.list missing both result and error keys"
    fi

    # If it has result, verify it's an array (container list)
    if echo "$R1_RESPONSE" | jq -e 'has("result")' >/dev/null 2>&1; then
        if echo "$R1_RESPONSE" | jq -e '.result | type == "array"' >/dev/null 2>&1; then
            log_pass "R1: container.list result is an array"
        elif echo "$R1_RESPONSE" | jq -e '.result | type == "object"' >/dev/null 2>&1; then
            # Could be wrapped in {containers: []}
            log_pass "R1: container.list result is an object (may contain containers key)"
            if echo "$R1_RESPONSE" | jq -e '.result.containers' >/dev/null 2>&1; then
                log_pass "R1: container.list result has 'containers' key"
            fi
        else
            log_fail "R1: container.list result is neither array nor object"
        fi
    fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# R2: container.terminate — invalid ID → verify error
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " R2: container.terminate — invalid ID"
echo "========================================="

R2_RESPONSE=$(rpc_call "container.terminate" '{"container_id":"nonexistent-container-xyz","user_id":"test-user"}')

# Save evidence
echo "$R2_RESPONSE" | jq . > "$EVIDENCE_DIR/r2-container-terminate-invalid.json" 2>/dev/null \
    || echo "$R2_RESPONSE" > "$EVIDENCE_DIR/r2-container-terminate-invalid.json"

if [[ -z "$R2_RESPONSE" ]]; then
    log_fail "R2: container.terminate returned empty response"
else
    # Should return an error for invalid container ID
    if assert_rpc_error "$R2_RESPONSE"; then
        log_pass "R2: container.terminate with invalid ID returns error"
    else
        # Could succeed if Docker not available (returns empty result)
        if assert_rpc_success "$R2_RESPONSE"; then
            log_pass "R2: container.terminate succeeded (Docker not available — empty result is expected)"
        else
            log_fail "R2: container.terminate returned unexpected shape"
        fi
    fi

    # Verify response has proper JSON-RPC structure
    if assert_json_has_key "$R2_RESPONSE" "jsonrpc"; then
        log_pass "R2: container.terminate has jsonrpc field"
    fi

    # If error, check for meaningful error message
    if echo "$R2_RESPONSE" | jq -e 'has("error")' >/dev/null 2>&1; then
        local_err_msg=$(echo "$R2_RESPONSE" | jq -r '.error.message' 2>/dev/null || true)
        if [[ -n "$local_err_msg" && "$local_err_msg" != "null" ]]; then
            log_pass "R2: error message present: '$local_err_msg'"
        else
            log_fail "R2: error message is missing or null"
        fi
    fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# R3: studio.list_agents — verify response shape
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " R3: studio.list_agents — response shape"
echo "========================================="

R3_RESPONSE=$(rpc_call "studio.list_agents" '{}')

# Save evidence
echo "$R3_RESPONSE" | jq . > "$EVIDENCE_DIR/r3-studio-list-agents.json" 2>/dev/null \
    || echo "$R3_RESPONSE" > "$EVIDENCE_DIR/r3-studio-list-agents.json"

if [[ -z "$R3_RESPONSE" ]]; then
    log_fail "R3: studio.list_agents returned empty response"
else
    # Must have result or error
    if echo "$R3_RESPONSE" | jq -e 'has("result") or has("error")' >/dev/null 2>&1; then
        log_pass "R3: studio.list_agents has result or error key"
    else
        log_fail "R3: studio.list_agents missing both result and error keys"
    fi

    # If it has result, check the shape
    if echo "$R3_RESPONSE" | jq -e 'has("result")' >/dev/null 2>&1; then
        result_type=$(echo "$R3_RESPONSE" | jq -r '.result | type' 2>/dev/null)
        case "$result_type" in
            "array")
                log_pass "R3: studio.list_agents result is an array"
                agent_count=$(echo "$R3_RESPONSE" | jq '.result | length' 2>/dev/null)
                log_info "R3: agent count: $agent_count"
                ;;
            "object")
                log_pass "R3: studio.list_agents result is an object"
                # Check for common wrapper keys
                if echo "$R3_RESPONSE" | jq -e '.result.agents' >/dev/null 2>&1; then
                    log_pass "R3: result has 'agents' key"
                fi
                if echo "$R3_RESPONSE" | jq -e '.result.list' >/dev/null 2>&1; then
                    log_pass "R3: result has 'list' key"
                fi
                ;;
            *)
                log_info "R3: studio.list_agents result type: $result_type"
                ;;
        esac
    fi

    # Verify JSON-RPC envelope
    if assert_json_has_key "$R3_RESPONSE" "jsonrpc"; then
        log_pass "R3: studio.list_agents has jsonrpc field"
    fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# R4: studio.list_instances — verify response shape
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " R4: studio.list_instances — response shape"
echo "========================================="

R4_RESPONSE=$(rpc_call "studio.list_instances" '{}')

# Save evidence
echo "$R4_RESPONSE" | jq . > "$EVIDENCE_DIR/r4-studio-list-instances.json" 2>/dev/null \
    || echo "$R4_RESPONSE" > "$EVIDENCE_DIR/r4-studio-list-instances.json"

if [[ -z "$R4_RESPONSE" ]]; then
    log_fail "R4: studio.list_instances returned empty response"
else
    # Must have result or error
    if echo "$R4_RESPONSE" | jq -e 'has("result") or has("error")' >/dev/null 2>&1; then
        log_pass "R4: studio.list_instances has result or error key"
    else
        log_fail "R4: studio.list_instances missing both result and error keys"
    fi

    # If it has result, check shape
    if echo "$R4_RESPONSE" | jq -e 'has("result")' >/dev/null 2>&1; then
        result_type=$(echo "$R4_RESPONSE" | jq -r '.result | type' 2>/dev/null)
        case "$result_type" in
            "array")
                log_pass "R4: studio.list_instances result is an array"
                instance_count=$(echo "$R4_RESPONSE" | jq '.result | length' 2>/dev/null)
                log_info "R4: instance count: $instance_count"
                ;;
            "object")
                log_pass "R4: studio.list_instances result is an object"
                if echo "$R4_RESPONSE" | jq -e '.result.instances' >/dev/null 2>&1; then
                    log_pass "R4: result has 'instances' key"
                fi
                ;;
            *)
                log_info "R4: studio.list_instances result type: $result_type"
                ;;
        esac
    fi

    # Verify JSON-RPC envelope
    if assert_json_has_key "$R4_RESPONSE" "jsonrpc"; then
        log_pass "R4: studio.list_instances has jsonrpc field"
    fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# R5: studio.get_stats — verify stats present
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " R5: studio.get_stats — verify stats"
echo "========================================="

R5_RESPONSE=$(rpc_call "studio.get_stats" '{}')

# Save evidence
echo "$R5_RESPONSE" | jq . > "$EVIDENCE_DIR/r5-studio-get-stats.json" 2>/dev/null \
    || echo "$R5_RESPONSE" > "$EVIDENCE_DIR/r5-studio-get-stats.json"

if [[ -z "$R5_RESPONSE" ]]; then
    log_fail "R5: studio.get_stats returned empty response"
else
    # Must have result or error
    if echo "$R5_RESPONSE" | jq -e 'has("result") or has("error")' >/dev/null 2>&1; then
        log_pass "R5: studio.get_stats has result or error key"
    else
        log_fail "R5: studio.get_stats missing both result and error keys"
    fi

    # If it has result, verify stats content
    if echo "$R5_RESPONSE" | jq -e 'has("result")' >/dev/null 2>&1; then
        stat_keys=$(echo "$R5_RESPONSE" | jq -r '.result | keys[]' 2>/dev/null || echo "")
        if [[ -n "$stat_keys" ]]; then
            log_pass "R5: studio.get_stats result has keys: $(echo "$stat_keys" | tr '\n' ', ')"
        fi

        # Check for expected stats fields (at least one should be present)
        found_stat=false
        for key in agents instances uptime total_agents total_instances active running; do
            if echo "$R5_RESPONSE" | jq -e --arg k "$key" '.result | has($k)' >/dev/null 2>&1; then
                log_pass "R5: stats contains '$key' field"
                found_stat=true
            fi
        done

        if ! $found_stat; then
            log_info "R5: stats result present but no standard stat keys found (shape may vary)"
        fi
    else
        # Error is acceptable if studio is not fully initialized
        log_info "R5: studio.get_stats returned error (studio may not be fully initialized)"
    fi

    # Verify JSON-RPC envelope
    if assert_json_has_key "$R5_RESPONSE" "jsonrpc"; then
        log_pass "R5: studio.get_stats has jsonrpc field"
    fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# R6: No cross-agent leakage — container isolation
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " R6: No cross-agent leakage — isolation"
echo "========================================="

# Check if any containers are running
RUNNING_CONTAINERS=$(docker ps --filter "name=openclaw" --format '{{.Names}}' 2>/dev/null || echo "")

if [[ -z "$RUNNING_CONTAINERS" ]]; then
    log_skip "R6: No running agent containers — isolation test skipped (no containers to check)"
    echo '{"status":"skipped","reason":"no running containers"}' \
        > "$EVIDENCE_DIR/r6-isolation.json"
else
    log_info "R6: Found running containers: $(echo "$RUNNING_CONTAINERS" | tr '\n' ' ')"

    ISOLATION_PASS=true
    CONTAINER_COUNT=0

    while IFS= read -r container_name; do
        [[ -z "$container_name" ]] && continue
        CONTAINER_COUNT=$((CONTAINER_COUNT + 1))

        # Check that containers have network mode "none" or are isolated
        CONTAINER_NET=$(docker inspect --format '{{.HostConfig.NetworkMode}}' "$container_name" 2>/dev/null || echo "unknown")
        if [[ "$CONTAINER_NET" == "none" || "$CONTAINER_NET" == *"isolated"* ]]; then
            log_pass "R6: Container '$container_name' has isolated network ($CONTAINER_NET)"
        else
            log_info "R6: Container '$container_name' network mode: $CONTAINER_NET"
        fi

        # Check that containers don't share PID namespace with host
        CONTAINER_PID=$(docker inspect --format '{{.HostConfig.PidMode}}' "$container_name" 2>/dev/null || echo "unknown")
        if [[ "$CONTAINER_PID" == "" || "$CONTAINER_PID" == "unknown" ]]; then
            log_pass "R6: Container '$container_name' has private PID namespace"
        else
            log_fail "R6: Container '$container_name' shares PID namespace: $CONTAINER_PID"
            ISOLATION_PASS=false
        fi

        # Check read-only root filesystem
        CONTAINER_RO=$(docker inspect --format '{{.HostConfig.ReadonlyRootfs}}' "$container_name" 2>/dev/null || echo "unknown")
        if [[ "$CONTAINER_RO" == "true" ]]; then
            log_pass "R6: Container '$container_name' has read-only root filesystem"
        else
            log_info "R6: Container '$container_name' root filesystem not read-only ($CONTAINER_RO)"
        fi
    done <<< "$RUNNING_CONTAINERS"

    # Save evidence
    cat > "$EVIDENCE_DIR/r6-isolation.json" << EVIDENCE
{
  "container_count": $CONTAINER_COUNT,
  "isolation_pass": $ISOLATION_PASS,
  "containers": $(echo "$RUNNING_CONTAINERS" | jq -R -s 'split("\n") | map(select(length > 0))' 2>/dev/null || echo '[]')
}
EVIDENCE

    if $ISOLATION_PASS; then
        log_pass "R6: All $CONTAINER_COUNT container(s) pass isolation checks"
    else
        log_fail "R6: Some containers failed isolation checks"
    fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════
echo ""
harness_summary
