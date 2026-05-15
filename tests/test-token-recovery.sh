#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# Token Recovery Test — validates the bridge handles Matrix token expiry
# gracefully using a deliberately invalid token.
#
# Tests: matrix.login, matrix.status/matrix.send with valid/invalid tokens,
#        health.check resilience, re-login recovery
# Uses Tier A pattern: SSH to VPS, run RPC calls against bridge.
#
# Usage:  bash tests/test-token-recovery.sh
# Requires: ssh, curl, jq, socat (optional for socket transport)
# ──────────────────────────────────────────────────────────────────────────────

# ── Source test libraries ──────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/load_env.sh"
source "${SCRIPT_DIR}/lib/common_output.sh"
source "${SCRIPT_DIR}/lib/assert_json.sh"

# ── Dependency check ───────────────────────────────────────────────────────────
command -v jq >/dev/null 2>&1 || { echo "FAIL: jq is required"; exit 1; }

# ── Test configuration ─────────────────────────────────────────────────────────
MATRIX_USER="bridge"
MATRIX_PASS="bridgepassword123"
INVALID_TOKEN="deliberately_invalid_token_12345"
TEST_ID="token-rec-$$-$(date +%s)"
EVIDENCE_DIR="${SCRIPT_DIR}/../.sisyphus/evidence/post-deploy"

# ── Ensure evidence directory exists ───────────────────────────────────────────
mkdir -p "$EVIDENCE_DIR"

# ── Transport detection (matching test-matrix-e2e-rpc.sh pattern) ──────────────
HAS_SOCKET=false
HAS_HTTP=false

check_socat() {
  ssh_vps "command -v socat >/dev/null 2>&1" 2>/dev/null
}

detect_transport() {
  if check_socat; then
    if ssh_vps "test -S /run/armorclaw/bridge.sock" 2>/dev/null; then
      HAS_SOCKET=true
    fi
  else
    log_info "socat not available on VPS — socket transport skipped"
  fi

  local http_code
  http_code=$(ssh_vps "curl -kfsS -o /dev/null -w '%{http_code}' https://localhost:${BRIDGE_PORT}/health 2>/dev/null || echo 000")
  if [ "$http_code" = "200" ]; then
    HAS_HTTP=true
  fi

  log_info "Transport: socket=$HAS_SOCKET http=$HAS_HTTP"
}

# ── RPC helpers (matching test-vps-smoke.sh patterns) ──────────────────────────

rpc_vps() {
  local method="$1" params="${2:-}"
  if [ -z "$params" ]; then
    params='{}'
  fi
  ssh_vps "curl -kfsS -H 'Authorization: Bearer ${ADMIN_TOKEN}' -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"${method}\",\"params\":${params}}' https://localhost:${BRIDGE_PORT}/api"
}

rpc_socket() {
  local method="$1"
  local params="${2:-{\}}"
  if [ "$params" = "\{\}" ] || [ "$params" = "{}" ]; then
    params='{}'
  fi
  local payload="{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"${method}\",\"params\":${params},\"auth\":\"${ADMIN_TOKEN}\"}"
  ssh_vps "echo '${payload}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock" 2>/dev/null
}

# Unified RPC call: tries socket first, falls back to HTTP
rpc_call() {
  local method="$1" params="${2:-}"
  if $HAS_SOCKET; then
    rpc_socket "$method" "$params"
  elif $HAS_HTTP; then
    rpc_vps "$method" "$params"
  else
    echo '{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"no transport available"}}'
  fi
}

# ── Save evidence ──────────────────────────────────────────────────────────────
save_evidence() {
  local name="$1" data="$2"
  echo "$data" > "${EVIDENCE_DIR}/${TEST_ID}-${name}.json"
}

# ══════════════════════════════════════════════════════════════════════════════
# Pre-flight checks
# ══════════════════════════════════════════════════════════════════════════════

log_info "Detecting transport..."
detect_transport

if ! $HAS_SOCKET && ! $HAS_HTTP; then
  log_fail "No transport available (neither socket nor HTTP)"
  harness_summary
  exit 1
fi

# Verify ADMIN_TOKEN is set
if [ -z "${ADMIN_TOKEN:-}" ]; then
  log_fail "ADMIN_TOKEN is empty — required for RPC authentication"
  harness_summary
  exit 1
fi

# Session state
MATRIX_ACCESS_TOKEN=""

echo ""
echo "========================================="
echo " Token Recovery Test"
echo " Test ID: ${TEST_ID}"
echo "========================================="
echo ""

# ══════════════════════════════════════════════════════════════════════════════
# Test 1: matrix.login — authenticate with valid credentials
# ══════════════════════════════════════════════════════════════════════════════

log_info "Test 1/5: matrix.login with valid credentials"

LOGIN_RESP=$(rpc_call "matrix.login" "{\"username\":\"${MATRIX_USER}\",\"password\":\"${MATRIX_PASS}\"}")
save_evidence "01-login" "$LOGIN_RESP"
log_info "Login response: $(echo "$LOGIN_RESP" | head -c 300)"

if assert_rpc_success "$LOGIN_RESP" >/dev/null 2>&1; then
  MATRIX_ACCESS_TOKEN=$(echo "$LOGIN_RESP" | jq -r '.result.access_token // .result.token // empty' 2>/dev/null || true)

  if [ -n "$MATRIX_ACCESS_TOKEN" ]; then
    log_pass "Test 1/5: matrix.login — authenticated, got access token"
  else
    log_fail "Test 1/5: matrix.login — succeeded but no access_token in response"
  fi
else
  local_err=$(echo "$LOGIN_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
  log_fail "Test 1/5: matrix.login — RPC error: $local_err"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# Test 2: matrix.status with valid token — should succeed
# ══════════════════════════════════════════════════════════════════════════════

log_info "Test 2/5: matrix.status with valid token"

if [ -n "$MATRIX_ACCESS_TOKEN" ]; then
  STATUS_PARAMS="{\"access_token\":\"${MATRIX_ACCESS_TOKEN}\"}"
  STATUS_RESP=$(rpc_call "matrix.status" "$STATUS_PARAMS")
  save_evidence "02-status-valid" "$STATUS_RESP"
  log_info "Status response: $(echo "$STATUS_RESP" | head -c 300)"

  if assert_rpc_success "$STATUS_RESP" >/dev/null 2>&1; then
    log_pass "Test 2/5: matrix.status with valid token — succeeded"
  else
    local_err=$(echo "$STATUS_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
    log_fail "Test 2/5: matrix.status with valid token — error: $local_err"
  fi
else
  log_skip "Test 2/5: matrix.status — skipped (no token from Test 1)"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# Test 3: matrix.status with invalid token — should return error, NOT crash
# ══════════════════════════════════════════════════════════════════════════════

log_info "Test 3/5: matrix.status with invalid token (expect error, not crash)"

BAD_PARAMS="{\"access_token\":\"${INVALID_TOKEN}\"}"
BAD_RESP=$(rpc_call "matrix.status" "$BAD_PARAMS")
save_evidence "03-status-invalid" "$BAD_RESP"
log_info "Invalid token response: $(echo "$BAD_RESP" | head -c 300)"

# We expect an error response — the bridge must NOT crash or hang
# A valid JSON-RPC response with an error object is the correct behavior
if echo "$BAD_RESP" | jq -e '.' >/dev/null 2>&1; then
  # Response is valid JSON — check if it's an error response
  if echo "$BAD_RESP" | jq -e 'has("error")' >/dev/null 2>&1; then
    local_err=$(echo "$BAD_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
    log_pass "Test 3/5: invalid token returned error (not crash) — message: $local_err"
  else
    # Got a success response despite invalid token — that's wrong
    log_fail "Test 3/5: invalid token returned success (expected error)"
  fi
else
  log_fail "Test 3/5: response is not valid JSON (bridge may have crashed)"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# Test 4: health.check — bridge should still be responsive after token error
# ══════════════════════════════════════════════════════════════════════════════

log_info "Test 4/5: health.check after token error"

HEALTH_RESP=$(rpc_call "health.check")
save_evidence "04-health-check" "$HEALTH_RESP"
log_info "Health response: $(echo "$HEALTH_RESP" | head -c 300)"

if assert_rpc_success "$HEALTH_RESP" >/dev/null 2>&1; then
  log_pass "Test 4/5: health.check — bridge still healthy after token error"
else
  local_err=$(echo "$HEALTH_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
  log_fail "Test 4/5: health.check — bridge NOT healthy after token error: $local_err"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# Test 5: Re-login after token error — should succeed (recovery)
# ══════════════════════════════════════════════════════════════════════════════

log_info "Test 5/5: re-login after token error"

RELOGIN_RESP=$(rpc_call "matrix.login" "{\"username\":\"${MATRIX_USER}\",\"password\":\"${MATRIX_PASS}\"}")
save_evidence "05-relogin" "$RELOGIN_RESP"
log_info "Re-login response: $(echo "$RELOGIN_RESP" | head -c 300)"

if assert_rpc_success "$RELOGIN_RESP" >/dev/null 2>&1; then
  NEW_TOKEN=$(echo "$RELOGIN_RESP" | jq -r '.result.access_token // .result.token // empty' 2>/dev/null || true)

  if [ -n "$NEW_TOKEN" ]; then
    log_pass "Test 5/5: re-login — authenticated, got new access token (recovery successful)"
  else
    log_fail "Test 5/5: re-login — succeeded but no access_token in response"
  fi
else
  local_err=$(echo "$RELOGIN_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
  log_fail "Test 5/5: re-login — RPC error: $local_err"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════

TOTAL_TESTS=$FULL_SYSTEM_PASSED
TOTAL_TESTS=$((TOTAL_TESTS + FULL_SYSTEM_FAILED))
echo "Token Recovery: ${FULL_SYSTEM_PASSED}/${TOTAL_TESTS} PASS"
echo ""

harness_summary
