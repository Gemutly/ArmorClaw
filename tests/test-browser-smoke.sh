#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# Browser Smoke Test — validates browser automation through bridge RPC
#
# Tests browser RPC methods (browser.navigate, browser.status, browser.complete)
# via the bridge's JSON-RPC interface. Graceful SKIP if Jetski/browser sidecar
# is not deployed on the VPS.
#
# Scenarios:
#   B0 — Prerequisites (jq, curl, bridge reachability)
#   B1 — browser.status  (probes whether browser RPC methods exist)
#   B2 — browser.navigate (navigate to a test URL)
#   B3 — browser.complete (mark session complete)
#
# Skip semantics:
#   -32601 (method not found) → SKIP (browser methods not registered)
#   "no active session"       → PASS (bridge knows browser, just idle)
#   Unexpected error           → FAIL
#
# Usage:  bash tests/test-browser-smoke.sh
# Requires: ssh, curl, jq
# ──────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/load_env.sh"
source "${SCRIPT_DIR}/lib/transport.sh"
source "${SCRIPT_DIR}/lib/common_output.sh"

# ── Evidence output directory ─────────────────────────────────────────────────
EVIDENCE_DIR="${SCRIPT_DIR}/../.sisyphus/evidence/browser-smoke"
mkdir -p "$EVIDENCE_DIR"

# ── Test configuration ────────────────────────────────────────────────────────
TEST_URL="https://example.com"

# ══════════════════════════════════════════════════════════════════════════════
# B0: Prerequisites — check dependencies and bridge reachability
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " B0: Prerequisites"
echo "========================================="

B0_PASS=true

# Check jq
if command -v jq >/dev/null 2>&1; then
  log_pass "jq is available ($(jq --version))"
else
  log_fail "jq is required but not found"
  B0_PASS=false
fi

# Check curl
if command -v curl >/dev/null 2>&1; then
  log_pass "curl is available"
else
  log_fail "curl is required but not found"
  B0_PASS=false
fi

# Check bridge reachability
if check_bridge_running; then
  log_pass "Bridge service is active on VPS"
else
  log_fail "Bridge service is not active on VPS"
  B0_PASS=false
fi

if ! $B0_PASS; then
  log_fail "B0 prerequisites failed — cannot continue"
  harness_summary
  exit 1
fi

# ── Transport detection (matching test-matrix-e2e-rpc.sh pattern) ─────────────
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

# ── RPC helpers (matching test-matrix-e2e-rpc.sh patterns) ────────────────────

rpc_vps() {
  local method="$1" params="${2:-}"
  if [ -z "$params" ]; then
    params='{}'
  fi
  ssh_vps "curl -kfsS -H 'Authorization: Bearer ${ADMIN_TOKEN}' -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"${method}\",\"params\":${params}}' https://localhost:${BRIDGE_PORT}/api" 2>/dev/null
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

# ── Save evidence ─────────────────────────────────────────────────────────────
save_evidence() {
  local name="$1" data="$2"
  echo "$data" > "${EVIDENCE_DIR}/${name}.json"
}

# ── Detect transport ──────────────────────────────────────────────────────────
detect_transport

if ! $HAS_SOCKET && ! $HAS_HTTP; then
  log_fail "No RPC transport available (neither socket nor HTTP)"
  harness_summary
  exit 1
fi

# ══════════════════════════════════════════════════════════════════════════════
# B1: browser.status — probe whether browser RPC methods are registered
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " B1: browser.status — probe browser RPC"
echo "========================================="

B1_RESP=""
B1_RESP=$(rpc_call "browser.status") || true

log_info "browser.status response: $(echo "$B1_RESP" | head -c 300)"
save_evidence "b1-browser-status" "$B1_RESP"

# Check for -32601 method not found → browser not deployed
if echo "$B1_RESP" | jq -e '.error.code == -32601' >/dev/null 2>&1; then
  log_skip "browser.status: browser RPC methods not registered (-32601)"
  log_skip "B2: browser.navigate (browser not deployed)"
  log_skip "B3: browser.complete (browser not deployed)"
  log_info "Browser sidecar not deployed — all browser tests skipped gracefully"
  harness_summary
  exit 0
fi

# Check for no active session → PASS (bridge knows browser, just idle)
if echo "$B1_RESP" | jq -e '.result' >/dev/null 2>&1; then
  log_pass "browser.status: bridge recognizes browser RPC (result returned)"
elif echo "$B1_RESP" | jq -e '.error' >/dev/null 2>&1; then
  # Check if error is "no active session" or similar → still PASS
  local_err_msg=$(echo "$B1_RESP" | jq -r '.error.message' 2>/dev/null || echo "")
  if echo "$local_err_msg" | grep -qi "no active session\|no session\|not initialized\|idle"; then
    log_pass "browser.status: bridge recognizes browser RPC (no active session — expected)"
  else
    log_fail "browser.status: unexpected error — $local_err_msg"
  fi
else
  log_fail "browser.status: malformed response (no result or error)"
fi

# ══════════════════════════════════════════════════════════════════════════════
# B2: browser.navigate — navigate to test URL
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " B2: browser.navigate — ${TEST_URL}"
echo "========================================="

B2_RESP=""
B2_RESP=$(rpc_call "browser.navigate" "{\"url\":\"${TEST_URL}\"}") || true

log_info "browser.navigate response: $(echo "$B2_RESP" | head -c 300)"
save_evidence "b2-browser-navigate" "$B2_RESP"

# Check for method not found (shouldn't happen after B1 gate, but be safe)
if echo "$B2_RESP" | jq -e '.error.code == -32601' >/dev/null 2>&1; then
  log_skip "browser.navigate: method not found (-32601)"
else
  # Success if navigate returned a result (even with warnings)
  if echo "$B2_RESP" | jq -e '.result' >/dev/null 2>&1; then
    log_pass "browser.navigate: navigation initiated successfully"
  elif echo "$B2_RESP" | jq -e '.error' >/dev/null 2>&1; then
    local_nav_err=$(echo "$B2_RESP" | jq -r '.error.message' 2>/dev/null || echo "")
    # "no active session" for navigate is also acceptable — browser module exists
    if echo "$local_nav_err" | grep -qi "no active session\|no session\|not initialized"; then
      log_pass "browser.navigate: bridge recognizes method (no active session — expected)"
    else
      log_fail "browser.navigate: unexpected error — $local_nav_err"
    fi
  else
    log_fail "browser.navigate: malformed response (no result or error)"
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# B3: browser.complete — mark session complete
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " B3: browser.complete"
echo "========================================="

B3_RESP=""
B3_RESP=$(rpc_call "browser.complete") || true

log_info "browser.complete response: $(echo "$B3_RESP" | head -c 300)"
save_evidence "b3-browser-complete" "$B3_RESP"

# Check for method not found
if echo "$B3_RESP" | jq -e '.error.code == -32601' >/dev/null 2>&1; then
  log_skip "browser.complete: method not found (-32601)"
else
  # Success if complete returned a result
  if echo "$B3_RESP" | jq -e '.result' >/dev/null 2>&1; then
    log_pass "browser.complete: session completion acknowledged"
  elif echo "$B3_RESP" | jq -e '.error' >/dev/null 2>&1; then
    local_comp_err=$(echo "$B3_RESP" | jq -r '.error.message' 2>/dev/null || echo "")
    # "no active session" is acceptable — browser module exists, nothing to complete
    if echo "$local_comp_err" | grep -qi "no active session\|no session\|not initialized\|nothing to complete"; then
      log_pass "browser.complete: bridge recognizes method (no active session — expected)"
    else
      log_fail "browser.complete: unexpected error — $local_comp_err"
    fi
  else
    log_fail "browser.complete: malformed response (no result or error)"
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════
echo ""
log_info "Evidence saved to $EVIDENCE_DIR/"
TOTAL=$((FULL_SYSTEM_PASSED + FULL_SYSTEM_FAILED + FULL_SYSTEM_SKIPPED))
echo -e "Browser Smoke: ${FULL_SYSTEM_PASSED}/${TOTAL} PASS (${FULL_SYSTEM_SKIPPED} SKIP)"
harness_summary
