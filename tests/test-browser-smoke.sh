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
# B4: browser.fill — fill a form field
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " B4: browser.fill"
echo "========================================="

B4_RESP=""
B4_RESP=$(rpc_call "browser.fill" '{"selector":"#test","value":"test"}') || true

log_info "browser.fill response: $(echo "$B4_RESP" | head -c 300)"
save_evidence "b4-browser-fill" "$B4_RESP"

if echo "$B4_RESP" | jq -e '.error.code == -32601' >/dev/null 2>&1; then
  log_skip "browser.fill: method not found (-32601)"
elif echo "$B4_RESP" | jq -e '.result' >/dev/null 2>&1; then
  log_pass "browser.fill: bridge recognizes method"
elif echo "$B4_RESP" | jq -e '.error' >/dev/null 2>&1; then
  local_err_msg=$(echo "$B4_RESP" | jq -r '.error.message' 2>/dev/null || echo "")
  if echo "$local_err_msg" | grep -qi "no active session\|no session\|not initialized"; then
    log_pass "browser.fill: bridge recognizes method (no active session — expected)"
  else
    log_fail "browser.fill: unexpected error — $local_err_msg"
  fi
else
  log_fail "browser.fill: malformed response"
fi

# ══════════════════════════════════════════════════════════════════════════════
# B5: browser.click — click an element
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " B5: browser.click"
echo "========================================="

B5_RESP=""
B5_RESP=$(rpc_call "browser.click" '{"selector":"#test"}') || true

log_info "browser.click response: $(echo "$B5_RESP" | head -c 300)"
save_evidence "b5-browser-click" "$B5_RESP"

if echo "$B5_RESP" | jq -e '.error.code == -32601' >/dev/null 2>&1; then
  log_skip "browser.click: method not found (-32601)"
elif echo "$B5_RESP" | jq -e '.result' >/dev/null 2>&1; then
  log_pass "browser.click: bridge recognizes method"
elif echo "$B5_RESP" | jq -e '.error' >/dev/null 2>&1; then
  local_err_msg=$(echo "$B5_RESP" | jq -r '.error.message' 2>/dev/null || echo "")
  if echo "$local_err_msg" | grep -qi "no active session\|no session\|not initialized"; then
    log_pass "browser.click: bridge recognizes method (no active session — expected)"
  else
    log_fail "browser.click: unexpected error — $local_err_msg"
  fi
else
  log_fail "browser.click: malformed response"
fi

# ══════════════════════════════════════════════════════════════════════════════
# B6: browser.wait_for_element — wait for a DOM element
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " B6: browser.wait_for_element"
echo "========================================="

B6_RESP=""
B6_RESP=$(rpc_call "browser.wait_for_element" '{"selector":"#test","timeout_ms":1000}') || true

log_info "browser.wait_for_element response: $(echo "$B6_RESP" | head -c 300)"
save_evidence "b6-browser-wait-for-element" "$B6_RESP"

if echo "$B6_RESP" | jq -e '.error.code == -32601' >/dev/null 2>&1; then
  log_skip "browser.wait_for_element: method not found (-32601)"
elif echo "$B6_RESP" | jq -e '.result' >/dev/null 2>&1; then
  log_pass "browser.wait_for_element: bridge recognizes method"
elif echo "$B6_RESP" | jq -e '.error' >/dev/null 2>&1; then
  local_err_msg=$(echo "$B6_RESP" | jq -r '.error.message' 2>/dev/null || echo "")
  if echo "$local_err_msg" | grep -qi "no active session\|no session\|not initialized"; then
    log_pass "browser.wait_for_element: bridge recognizes method (no active session — expected)"
  else
    log_fail "browser.wait_for_element: unexpected error — $local_err_msg"
  fi
else
  log_fail "browser.wait_for_element: malformed response"
fi

# ══════════════════════════════════════════════════════════════════════════════
# B7: browser.wait_for_captcha — wait for CAPTCHA resolution
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " B7: browser.wait_for_captcha"
echo "========================================="

B7_RESP=""
B7_RESP=$(rpc_call "browser.wait_for_captcha" '{"timeout_ms":1000}') || true

log_info "browser.wait_for_captcha response: $(echo "$B7_RESP" | head -c 300)"
save_evidence "b7-browser-wait-for-captcha" "$B7_RESP"

if echo "$B7_RESP" | jq -e '.error.code == -32601' >/dev/null 2>&1; then
  log_skip "browser.wait_for_captcha: method not found (-32601)"
elif echo "$B7_RESP" | jq -e '.result' >/dev/null 2>&1; then
  log_pass "browser.wait_for_captcha: bridge recognizes method"
elif echo "$B7_RESP" | jq -e '.error' >/dev/null 2>&1; then
  local_err_msg=$(echo "$B7_RESP" | jq -r '.error.message' 2>/dev/null || echo "")
  if echo "$local_err_msg" | grep -qi "no active session\|no session\|not initialized"; then
    log_pass "browser.wait_for_captcha: bridge recognizes method (no active session — expected)"
  else
    log_fail "browser.wait_for_captcha: unexpected error — $local_err_msg"
  fi
else
  log_fail "browser.wait_for_captcha: malformed response"
fi

# ══════════════════════════════════════════════════════════════════════════════
# B8: browser.wait_for_2fa — wait for 2FA code entry
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " B8: browser.wait_for_2fa"
echo "========================================="

B8_RESP=""
B8_RESP=$(rpc_call "browser.wait_for_2fa" '{"timeout_ms":1000}') || true

log_info "browser.wait_for_2fa response: $(echo "$B8_RESP" | head -c 300)"
save_evidence "b8-browser-wait-for-2fa" "$B8_RESP"

if echo "$B8_RESP" | jq -e '.error.code == -32601' >/dev/null 2>&1; then
  log_skip "browser.wait_for_2fa: method not found (-32601)"
elif echo "$B8_RESP" | jq -e '.result' >/dev/null 2>&1; then
  log_pass "browser.wait_for_2fa: bridge recognizes method"
elif echo "$B8_RESP" | jq -e '.error' >/dev/null 2>&1; then
  local_err_msg=$(echo "$B8_RESP" | jq -r '.error.message' 2>/dev/null || echo "")
  if echo "$local_err_msg" | grep -qi "no active session\|no session\|not initialized"; then
    log_pass "browser.wait_for_2fa: bridge recognizes method (no active session — expected)"
  else
    log_fail "browser.wait_for_2fa: unexpected error — $local_err_msg"
  fi
else
  log_fail "browser.wait_for_2fa: malformed response"
fi

# ══════════════════════════════════════════════════════════════════════════════
# B9: browser.fail — mark session as failed
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " B9: browser.fail"
echo "========================================="

B9_RESP=""
B9_RESP=$(rpc_call "browser.fail" '{"reason":"smoke test"}') || true

log_info "browser.fail response: $(echo "$B9_RESP" | head -c 300)"
save_evidence "b9-browser-fail" "$B9_RESP"

if echo "$B9_RESP" | jq -e '.error.code == -32601' >/dev/null 2>&1; then
  log_skip "browser.fail: method not found (-32601)"
elif echo "$B9_RESP" | jq -e '.result' >/dev/null 2>&1; then
  log_pass "browser.fail: bridge recognizes method"
elif echo "$B9_RESP" | jq -e '.error' >/dev/null 2>&1; then
  local_err_msg=$(echo "$B9_RESP" | jq -r '.error.message' 2>/dev/null || echo "")
  if echo "$local_err_msg" | grep -qi "no active session\|no session\|not initialized"; then
    log_pass "browser.fail: bridge recognizes method (no active session — expected)"
  else
    log_fail "browser.fail: unexpected error — $local_err_msg"
  fi
else
  log_fail "browser.fail: malformed response"
fi

# ══════════════════════════════════════════════════════════════════════════════
# B10: browser.list — list browser sessions
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " B10: browser.list"
echo "========================================="

B10_RESP=""
B10_RESP=$(rpc_call "browser.list" '{}') || true

log_info "browser.list response: $(echo "$B10_RESP" | head -c 300)"
save_evidence "b10-browser-list" "$B10_RESP"

if echo "$B10_RESP" | jq -e '.error.code == -32601' >/dev/null 2>&1; then
  log_skip "browser.list: method not found (-32601)"
elif echo "$B10_RESP" | jq -e '.result' >/dev/null 2>&1; then
  log_pass "browser.list: bridge recognizes method"
elif echo "$B10_RESP" | jq -e '.error' >/dev/null 2>&1; then
  local_err_msg=$(echo "$B10_RESP" | jq -r '.error.message' 2>/dev/null || echo "")
  if echo "$local_err_msg" | grep -qi "no active session\|no session\|not initialized"; then
    log_pass "browser.list: bridge recognizes method (no active session — expected)"
  else
    log_fail "browser.list: unexpected error — $local_err_msg"
  fi
else
  log_fail "browser.list: malformed response"
fi

# ══════════════════════════════════════════════════════════════════════════════
# B11: browser.replay_diagnostics — replay diagnostics for a session
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " B11: browser.replay_diagnostics"
echo "========================================="

B11_RESP=""
B11_RESP=$(rpc_call "browser.replay_diagnostics" '{}') || true

log_info "browser.replay_diagnostics response: $(echo "$B11_RESP" | head -c 300)"
save_evidence "b11-browser-replay-diagnostics" "$B11_RESP"

if echo "$B11_RESP" | jq -e '.error.code == -32601' >/dev/null 2>&1; then
  log_skip "browser.replay_diagnostics: method not found (-32601)"
elif echo "$B11_RESP" | jq -e '.result' >/dev/null 2>&1; then
  log_pass "browser.replay_diagnostics: bridge recognizes method"
elif echo "$B11_RESP" | jq -e '.error' >/dev/null 2>&1; then
  local_err_msg=$(echo "$B11_RESP" | jq -r '.error.message' 2>/dev/null || echo "")
  if echo "$local_err_msg" | grep -qi "no active session\|no session\|not initialized"; then
    log_pass "browser.replay_diagnostics: bridge recognizes method (no active session — expected)"
  else
    log_fail "browser.replay_diagnostics: unexpected error — $local_err_msg"
  fi
else
  log_fail "browser.replay_diagnostics: malformed response"
fi

# ══════════════════════════════════════════════════════════════════════════════
# B12: browser.cancel — cancel active browser session
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " B12: browser.cancel"
echo "========================================="

B12_RESP=""
B12_RESP=$(rpc_call "browser.cancel" '{}') || true

log_info "browser.cancel response: $(echo "$B12_RESP" | head -c 300)"
save_evidence "b12-browser-cancel" "$B12_RESP"

if echo "$B12_RESP" | jq -e '.error.code == -32601' >/dev/null 2>&1; then
  log_skip "browser.cancel: method not found (-32601)"
elif echo "$B12_RESP" | jq -e '.result' >/dev/null 2>&1; then
  log_pass "browser.cancel: bridge recognizes method"
elif echo "$B12_RESP" | jq -e '.error' >/dev/null 2>&1; then
  local_err_msg=$(echo "$B12_RESP" | jq -r '.error.message' 2>/dev/null || echo "")
  if echo "$local_err_msg" | grep -qi "no active session\|no session\|not initialized"; then
    log_pass "browser.cancel: bridge recognizes method (no active session — expected)"
  else
    log_fail "browser.cancel: unexpected error — $local_err_msg"
  fi
else
  log_fail "browser.cancel: malformed response"
fi

# ══════════════════════════════════════════════════════════════════════════════
# B13: browser.screenshot — capture screenshot
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " B13: browser.screenshot"
echo "========================================="

B13_RESP=""
B13_RESP=$(rpc_call "browser.screenshot" '{"format":"png"}') || true

log_info "browser.screenshot response: $(echo "$B13_RESP" | head -c 300)"
save_evidence "b13-browser-screenshot" "$B13_RESP"

if echo "$B13_RESP" | jq -e '.error.code == -32601' >/dev/null 2>&1; then
  log_skip "browser.screenshot: method not found (-32601)"
elif echo "$B13_RESP" | jq -e '.result' >/dev/null 2>&1; then
  log_pass "browser.screenshot: bridge recognizes method"
elif echo "$B13_RESP" | jq -e '.error' >/dev/null 2>&1; then
  local_err_msg=$(echo "$B13_RESP" | jq -r '.error.message' 2>/dev/null || echo "")
  if echo "$local_err_msg" | grep -qi "no active session\|no session\|not initialized"; then
    log_pass "browser.screenshot: bridge recognizes method (no active session — expected)"
  else
    log_fail "browser.screenshot: unexpected error — $local_err_msg"
  fi
else
  log_fail "browser.screenshot: malformed response"
fi

# ══════════════════════════════════════════════════════════════════════════════
# B14: browser.close — close browser session
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " B14: browser.close"
echo "========================================="

B14_RESP=""
B14_RESP=$(rpc_call "browser.close" '{}') || true

log_info "browser.close response: $(echo "$B14_RESP" | head -c 300)"
save_evidence "b14-browser-close" "$B14_RESP"

if echo "$B14_RESP" | jq -e '.error.code == -32601' >/dev/null 2>&1; then
  log_skip "browser.close: method not found (-32601)"
elif echo "$B14_RESP" | jq -e '.result' >/dev/null 2>&1; then
  log_pass "browser.close: bridge recognizes method"
elif echo "$B14_RESP" | jq -e '.error' >/dev/null 2>&1; then
  local_err_msg=$(echo "$B14_RESP" | jq -r '.error.message' 2>/dev/null || echo "")
  if echo "$local_err_msg" | grep -qi "no active session\|no session\|not initialized"; then
    log_pass "browser.close: bridge recognizes method (no active session — expected)"
  else
    log_fail "browser.close: unexpected error — $local_err_msg"
  fi
else
  log_fail "browser.close: malformed response"
fi

# ══════════════════════════════════════════════════════════════════════════════
# AUTH: Auth enforcement check on browser methods
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " AUTH: Auth enforcement on browser RPCs"
echo "========================================="

AUTH_METHODS=("browser.navigate" "browser.fill" "browser.click" "browser.status" "browser.complete" "browser.cancel" "browser.fail" "browser.list" "browser.wait_for_element" "browser.wait_for_captcha" "browser.wait_for_2fa" "browser.replay_diagnostics" "browser.screenshot" "browser.close")

for method in "${AUTH_METHODS[@]}"; do
  # Call without auth token — use raw transport without Authorization header
  AUTH_RESP=""
  if $HAS_SOCKET; then
    AUTH_RESP=$(ssh_vps "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"${method}\",\"params\":{}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock" 2>/dev/null) || true
  elif $HAS_HTTP; then
    AUTH_RESP=$(ssh_vps "curl -kfsS -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"${method}\",\"params\":{}}' https://localhost:${BRIDGE_PORT}/api" 2>/dev/null) || true
  fi

  if echo "$AUTH_RESP" | jq -e '.error.code == -32010 or .error.code == -32011' >/dev/null 2>&1; then
    log_pass "${method}: auth enforced (unauthenticated request rejected)"
  elif echo "$AUTH_RESP" | jq -e '.error.code == -32601' >/dev/null 2>&1; then
    log_skip "${method}: method not registered — auth check N/A"
  elif echo "$AUTH_RESP" | jq -e '.result' >/dev/null 2>&1; then
    log_fail "${method}: SECURITY — unauthenticated request succeeded!"
  else
    log_info "${method}: unexpected auth response — $(echo "$AUTH_RESP" | head -c 200)"
  fi
done

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════
echo ""
log_info "Evidence saved to $EVIDENCE_DIR/"
echo "14 browser methods tested"
TOTAL=$((FULL_SYSTEM_PASSED + FULL_SYSTEM_FAILED + FULL_SYSTEM_SKIPPED))
echo -e "Browser Smoke: ${FULL_SYSTEM_PASSED}/${TOTAL} PASS (${FULL_SYSTEM_SKIPPED} SKIP)"
harness_summary
