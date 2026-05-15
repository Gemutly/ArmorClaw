#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# Persistence Integration Test — Invite State Survival Across Bridge Restart
#
# Creates an invite, restarts the bridge, validates the invite survived,
# then revokes it. Runs against a live VPS via SSH.
#
# Phases: P0-Prerequisites, P1-Create, P2-Pre-restart,
#         P3-Restart, P4-Post-restart, P5-Revoke
#
# Usage:  bash tests/test-persistence.sh
# Requires: .env with VPS_IP, ADMIN_TOKEN (or exported in env)
# ──────────────────────────────────────────────────────────────────────────────

# Auto-source .env
set -a
source "$(dirname "$0")/../.env" 2>/dev/null || true
set +a

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/transport.sh"

# ── Environment ────────────────────────────────────────────────────────────────
VPS_IP="${VPS_IP:?VPS_IP required}"
VPS_USER="${VPS_USER:-root}"
SSH_KEY_PATH="${SSH_KEY_PATH:-~/.ssh/openclaw_win}"
ADMIN_TOKEN="${ADMIN_TOKEN:?ADMIN_TOKEN required}"
BRIDGE_PORT="${BRIDGE_PORT:-8080}"
BRIDGE_SERVICE="${BRIDGE_SERVICE:-armorclaw-bridge}"
BRIDGE_SOCKET="${BRIDGE_SOCKET:-/run/armorclaw/bridge.sock}"

command -v jq >/dev/null 2>&1 || { echo "FAIL: jq is required"; exit 1; }

# ── Counters ──────────────────────────────────────────────────────────────────
TOTAL=0 PASSED=0 FAILED=0
P0_PASSED=0 P0_FAILED=0
P1_PASSED=0 P1_FAILED=0
P2_PASSED=0 P2_FAILED=0
P3_PASSED=0 P3_FAILED=0
P4_PASSED=0 P4_FAILED=0
P5_PASSED=0 P5_FAILED=0
FAILURES=""

# Unique run ID
TEST_RUN_ID="persist-$(date +%s)-$$"

# ── Helpers ───────────────────────────────────────────────────────────────────

ssh_vps() {
  ssh -i "${SSH_KEY_PATH}" -o ConnectTimeout=10 -o StrictHostKeyChecking=no "${VPS_USER}@${VPS_IP}" "$@"
}

# RPC via Unix socket (preferred)
rpc_socket() {
  local method="$1" params="${2:-}"
  if [ -z "$params" ]; then params='{}'; fi
  local payload="{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"${method}\",\"params\":${params},\"auth\":\"${ADMIN_TOKEN}\"}"
  ssh_vps "echo '${payload}' | socat - UNIX-CONNECT:${BRIDGE_SOCKET}"
}

# RPC via HTTP (fallback)
rpc_http() {
  local method="$1" params="${2:-}"
  if [ -z "$params" ]; then params='{}'; fi
  local payload="{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"${method}\",\"params\":${params}}"
  ssh_vps "curl -kfsS -X POST https://localhost:${BRIDGE_PORT}/api -H 'Content-Type: application/json' -H 'Authorization: Bearer ${ADMIN_TOKEN}' -d '${payload}'"
}

# Auto-detect transport
RPC_CMD=""
TRANSPORT=""

detect_transport() {
  if ssh_vps "command -v socat >/dev/null 2>&1 && test -S ${BRIDGE_SOCKET}" 2>/dev/null; then
    RPC_CMD=rpc_socket
    TRANSPORT="socket"
  elif ssh_vps "curl -kfsS -o /dev/null https://localhost:${BRIDGE_PORT}/health 2>/dev/null"; then
    RPC_CMD=rpc_http
    TRANSPORT="http"
  else
    echo "FATAL: No transport available (socket and HTTP both failed)"
    exit 1
  fi
  echo "[INFO] Transport: $TRANSPORT"
}

# Generic RPC call using detected transport
rpc() {
  $RPC_CMD "$@"
}

# Test runner
run_test() {
  local name="$1" expected="$2" actual="$3"
  TOTAL=$((TOTAL + 1))
  if echo "$actual" | grep -q "$expected"; then
    PASSED=$((PASSED + 1))
    echo "  [PASS] $name"
  else
    FAILED=$((FAILED + 1))
    FAILURES="$FAILURES\n  - $name: expected '$expected' in '$(echo "$actual" | head -c 200)'"
    echo "  [FAIL] $name"
    echo "    Expected: $expected"
    echo "    Got: $(echo "$actual" | head -c 200)"
  fi
}

# Phase-scoped test runner
run_phase_test() {
  local phase="$1" name="$2" expected="$3" actual="$4"
  run_test "$name" "$expected" "$actual"
  local idx=$((TOTAL - 1))
  if echo "$actual" | grep -q "$expected"; then
    case "$phase" in
      P0) P0_PASSED=$((P0_PASSED + 1)) ;;
      P1) P1_PASSED=$((P1_PASSED + 1)) ;;
      P2) P2_PASSED=$((P2_PASSED + 1)) ;;
      P3) P3_PASSED=$((P3_PASSED + 1)) ;;
      P4) P4_PASSED=$((P4_PASSED + 1)) ;;
      P5) P5_PASSED=$((P5_PASSED + 1)) ;;
    esac
  else
    case "$phase" in
      P0) P0_FAILED=$((P0_FAILED + 1)) ;;
      P1) P1_FAILED=$((P1_FAILED + 1)) ;;
      P2) P2_FAILED=$((P2_FAILED + 1)) ;;
      P3) P3_FAILED=$((P3_FAILED + 1)) ;;
      P4) P4_FAILED=$((P4_FAILED + 1)) ;;
      P5) P5_FAILED=$((P5_FAILED + 1)) ;;
    esac
  fi
}

# ══════════════════════════════════════════════════════════════════════════════
# P0: Prerequisites
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " P0: Prerequisites (3 tests)"
echo "========================================="

SSH_RESULT=$(ssh_vps "echo OK" 2>/dev/null || echo "FAIL")
run_phase_test P0 "SSH connectivity" "OK" "$SSH_RESULT"

SERVICE_RESULT=$(ssh_vps "systemctl is-active ${BRIDGE_SERVICE}" 2>/dev/null || echo "FAIL")
run_phase_test P0 "Bridge service active" "active" "$SERVICE_RESULT"

detect_transport
if [ -n "$TRANSPORT" ]; then
  run_phase_test P0 "Transport detection" "$TRANSPORT" "transport=$TRANSPORT"
else
  run_phase_test P0 "Transport detection" "FAIL" "no transport"
fi

# ══════════════════════════════════════════════════════════════════════════════
# P1: Create Invite
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " P1: Create Invite (2 tests)"
echo "========================================="

CREATE_RESULT=$(rpc invite.create "{\"role\":\"admin\",\"expiration\":\"1d\",\"max_uses\":1,\"created_by\":\"persistence-test\",\"welcome_message\":\"test invite ${TEST_RUN_ID}\"}")
run_phase_test P1 "invite.create returns code" '"code"' "$CREATE_RESULT"

INVITE_ID=$(echo "$CREATE_RESULT" | jq -r '.result.id // empty' 2>/dev/null || true)
INVITE_CODE=$(echo "$CREATE_RESULT" | jq -r '.result.code // empty' 2>/dev/null || true)

if [ -n "$INVITE_ID" ] && [ -n "$INVITE_CODE" ]; then
  run_phase_test P1 "Captured invite_id and code" "captured" "captured id=$INVITE_ID"
else
  run_phase_test P1 "Captured invite_id and code" "captured" "id=[$INVITE_ID] code=[$INVITE_CODE]"
fi

echo "  Created: id=$INVITE_ID code=$INVITE_CODE"

# ══════════════════════════════════════════════════════════════════════════════
# P2: Pre-Restart Validation
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " P2: Pre-Restart Validation (2 tests)"
echo "========================================="

VALIDATE_PRE=$(rpc invite.validate "{\"code\":\"${INVITE_CODE}\"}")
run_phase_test P2 "invite.validate (pre-restart) active" '"active"' "$VALIDATE_PRE"

LIST_PRE=$(rpc invite.list '{}')
run_phase_test P2 "invite.list (pre-restart) contains id" "$INVITE_ID" "$LIST_PRE"

# ══════════════════════════════════════════════════════════════════════════════
# P3: Restart Bridge
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " P3: Restart Bridge (1 test)"
echo "========================================="

echo "  Restarting ${BRIDGE_SERVICE}..."
ssh_vps "systemctl restart ${BRIDGE_SERVICE}" 2>/dev/null || true

# Poll for readiness (max 30s, 2s intervals)
READY=false
for i in $(seq 1 15); do
  sleep 2
  if ssh_vps "systemctl is-active ${BRIDGE_SERVICE}" 2>/dev/null | grep -q "active"; then
    # Re-detect transport after restart (socket may take a moment)
    sleep 1
    detect_transport 2>/dev/null || true
    if [ -n "$RPC_CMD" ]; then
      # Quick health check
      HEALTH=$(rpc invite.list '{}' 2>/dev/null || echo "")
      if [ -n "$HEALTH" ]; then
        READY=true
        echo "  Bridge ready after $((i * 2))s"
        break
      fi
    fi
  fi
  echo "  ... waiting ($((i * 2))s)"
done

if $READY; then
  run_phase_test P3 "Bridge active after restart" "active" "active"
else
  run_phase_test P3 "Bridge active after restart" "active" "not-ready"
fi

# ══════════════════════════════════════════════════════════════════════════════
# P4: Post-Restart Validation
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " P4: Post-Restart Validation (2 tests)"
echo "========================================="

VALIDATE_POST=$(rpc invite.validate "{\"code\":\"${INVITE_CODE}\"}")
run_phase_test P4 "invite.validate (post-restart) active" '"active"' "$VALIDATE_POST"

LIST_POST=$(rpc invite.list '{}')
run_phase_test P4 "invite.list (post-restart) contains id" "$INVITE_ID" "$LIST_POST"

# ══════════════════════════════════════════════════════════════════════════════
# P5: Revoke After Restart
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " P5: Revoke After Restart (2 tests)"
echo "========================================="

REVOKE_RESULT=$(rpc invite.revoke "{\"invite_id\":\"${INVITE_ID}\"}")
run_phase_test P5 "invite.revoke returns success" '"success"' "$REVOKE_RESULT"

VALIDATE_REVOKED=$(rpc invite.validate "{\"code\":\"${INVITE_CODE}\"}")
if echo "$VALIDATE_REVOKED" | grep -q "revoked"; then
  run_phase_test P5 "invite.validate revoked code" "revoked" "$VALIDATE_REVOKED"
elif echo "$VALIDATE_REVOKED" | grep -q "error"; then
  run_phase_test P5 "invite.validate revoked code" "error" "$VALIDATE_REVOKED"
else
  run_phase_test P5 "invite.validate revoked code" "revoked" "$VALIDATE_REVOKED"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " Results"
echo "========================================="
echo "  P0 Prerequisites:  $P0_PASSED passed, $P0_FAILED failed"
echo "  P1 Create:         $P1_PASSED passed, $P1_FAILED failed"
echo "  P2 Pre-restart:    $P2_PASSED passed, $P2_FAILED failed"
echo "  P3 Restart:        $P3_PASSED passed, $P3_FAILED failed"
echo "  P4 Post-restart:   $P4_PASSED passed, $P4_FAILED failed"
echo "  P5 Revoke:         $P5_PASSED passed, $P5_FAILED failed"
echo ""
echo "  TOTAL: $PASSED / $TOTAL passed"

if [ -n "$FAILURES" ]; then
  echo ""
  echo "  Failures:"
  echo -e "$FAILURES"
fi

if [ "$FAILED" -gt 0 ]; then
  exit 1
fi

exit 0
