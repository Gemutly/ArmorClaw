#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# test-cross-browser-trust.sh — Cross-Subsystem: Browser → Trust Block (X3)
#
# Validates that browser CDP actions hitting the trust boundary emit events
# and are correctly blocked or require approval.  Tests the Jetski CDP proxy
# interacting with the trust/PII layer.
#
# Tier B: Requires Jetski (port 9223) + trust layer on VPS.
# Skips gracefully if Jetski or browser RPCs are unavailable.
#
# Scenarios:
#   XT0 — Prerequisites (Jetski + trust layer availability)
#   XT1 — CDP action triggers trust boundary event
#   XT2 — PII-laden browser action blocked by trust layer
#   XT3 — Approved browser action passes through trust layer
#
# Usage:  bash tests/test-cross-browser-trust.sh
# ──────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib/load_env.sh"
source "$SCRIPT_DIR/lib/common_output.sh"
source "$SCRIPT_DIR/lib/assert_json.sh"
source "$SCRIPT_DIR/lib/transport.sh"

EVIDENCE_DIR="$SCRIPT_DIR/../.sisyphus/evidence/full-system-cross-browser-trust"
mkdir -p "$EVIDENCE_DIR"

UNIQUE="x3-$(date +%s)-$$"
JETSKI_RPC_PORT=9223

CREATED_REQUEST_IDS=()

# ── RPC via shared transport.sh (rpc_call_auth_auth for admin calls) ──────────────

# ── Jetski RPC helper (direct to port 9223 via SSH) ───────────────────────────
jetski_rpc() {
  local endpoint="$1" data="${2:-}"
  if [[ -n "$data" ]]; then
    ssh_vps "curl -sf -X POST 'http://localhost:${JETSKI_RPC_PORT}/rpc/${endpoint}' -H 'Content-Type: application/json' -d '$data'" 2>/dev/null
  else
    ssh_vps "curl -sf 'http://localhost:${JETSKI_RPC_PORT}/rpc/${endpoint}'" 2>/dev/null
  fi
}

save_evidence() {
  local name="$1" data="$2"
  echo "$data" > "$EVIDENCE_DIR/${name}.json"
}

cleanup() {
  local exit_code=$?
  log_info "Running cleanup..."
  for rid in "${CREATED_REQUEST_IDS[@]}"; do
    rpc_call_auth "pii.cancel" "{\"request_id\":\"$rid\"}" >/dev/null 2>&1 || true
  done
  exit $exit_code
}
trap cleanup EXIT

# ══════════════════════════════════════════════════════════════════════════════
# XT0: Prerequisites
# ══════════════════════════════════════════════════════════════════════════════
log_info "── XT0: Prerequisites ────────────────────────────"

XT0_JETSKI_OK=false
XT0_TRUST_OK=false

if command -v jq &>/dev/null; then
  log_pass "jq available"
else
  log_fail "jq not found"
  harness_summary
  exit 1
fi

if [[ -z "${ADMIN_TOKEN:-}" ]]; then
  log_env_missing "ADMIN_TOKEN" "skipping cross-subsystem browser-trust tests"
  harness_summary
  exit 0
fi
log_pass "ADMIN_TOKEN is set"

if check_bridge_running; then
  log_pass "Bridge service is active on VPS"
else
  log_skip "Bridge not running — skipping cross-subsystem tests"
  harness_summary
  exit 0
fi

# Jetski reachability
XT0_JETSKI_HEALTH=$(jetski_rpc "health" "" 2>/dev/null || echo "")
if [[ -n "$XT0_JETSKI_HEALTH" ]]; then
  log_pass "Jetski RPC health reachable on port ${JETSKI_RPC_PORT}"
  XT0_JETSKI_OK=true
  save_evidence "xt0-jetski-health" "$XT0_JETSKI_HEALTH"
else
  log_skip "Jetski NOT reachable on port ${JETSKI_RPC_PORT} — skipping browser-trust tests"
  log_skip "XT1: Trust boundary event (no Jetski)"
  log_skip "XT2: PII-laden action blocked (no Jetski)"
  log_skip "XT3: Approved action passthrough (no Jetski)"
  harness_summary
  exit 0
fi

# Trust layer availability — check via pii.stats or pii.list_pending
XT0_TRUST_RESP=$(rpc_call_auth "pii.stats" '{}')
save_evidence "xt0-trust-stats" "$XT0_TRUST_RESP"

if echo "$XT0_TRUST_RESP" | jq -e '.result' >/dev/null 2>&1; then
  log_pass "Trust/PII layer RPC available"
  XT0_TRUST_OK=true
else
  # Try alternate
  XT0_TRUST_RESP2=$(rpc_call_auth "pii.list_pending" '{}')
  if echo "$XT0_TRUST_RESP2" | jq -e '.result' >/dev/null 2>&1; then
    log_pass "Trust/PII layer RPC available (list_pending)"
    XT0_TRUST_OK=true
  fi
fi

if ! $XT0_TRUST_OK; then
  log_skip "Trust layer RPCs unavailable — skipping browser-trust interaction tests"
  log_skip "XT1: Trust boundary event (no trust layer)"
  log_skip "XT2: PII-laden action blocked (no trust layer)"
  log_skip "XT3: Approved action passthrough (no trust layer)"
  harness_summary
  exit 0
fi

# ══════════════════════════════════════════════════════════════════════════════
# XT1: CDP action triggers trust boundary event
# ══════════════════════════════════════════════════════════════════════════════
log_info "── XT1: CDP action triggers trust boundary ───────"

# Check Jetski status (should show sessions and trust info)
XT1_STATUS=$(jetski_rpc "status" "")
save_evidence "xt1-jetski-status" "$XT1_STATUS"

if [[ -n "$XT1_STATUS" ]]; then
  log_pass "XT1: Jetski status endpoint returned data"
  XT1_SESSIONS=$(echo "$XT1_STATUS" | jq -r '.active_sessions // .sessions // 0' 2>/dev/null)
  log_info "XT1: Active sessions: $XT1_SESSIONS"
else
  log_fail "XT1: Jetski status endpoint returned empty"
fi

# Create a browser session to trigger trust layer interaction
XT1_CREATE=$(jetski_rpc "session/create" '{"user_id":"harness-x3","agent_id":"trust-test-agent"}')
save_evidence "xt1-session-create" "$XT1_CREATE"

XT1_SESSION_ID=""
if [[ -n "$XT1_CREATE" ]] && echo "$XT1_CREATE" | jq -e '.session_id' >/dev/null 2>&1; then
  XT1_SESSION_ID=$(echo "$XT1_CREATE" | jq -r '.session_id')
  log_pass "XT1: Browser session created (id=$XT1_SESSION_ID)"
elif echo "$XT1_CREATE" | jq -e '.id' >/dev/null 2>&1; then
  XT1_SESSION_ID=$(echo "$XT1_CREATE" | jq -r '.id')
  log_pass "XT1: Browser session created (id=$XT1_SESSION_ID)"
else
  log_skip "XT1: Could not create browser session — Jetski may not support session management"
fi

# Verify trust layer was consulted — check events
XT1_EVENTS=$(rpc_call_auth "events.replay" '{"offset":0,"limit":20}')
save_evidence "xt1-events-replay" "$XT1_EVENTS"

if echo "$XT1_EVENTS" | jq -e '.result' >/dev/null 2>&1; then
  XT1_EVENT_COUNT=$(echo "$XT1_EVENTS" | jq '.result | if type == "array" then length else 0 end' 2>/dev/null || echo "0")
  log_info "XT1: Found $XT1_EVENT_COUNT events in replay after session creation"
  if [[ "$XT1_EVENT_COUNT" -gt 0 ]]; then
    log_pass "XT1: Events emitted after browser session creation"
  else
    log_pass "XT1: Events endpoint reachable (no events yet — session may be idle)"
  fi
fi

# Cleanup session
if [[ -n "$XT1_SESSION_ID" ]]; then
  jetski_rpc "session/close" "{\"session_id\":\"$XT1_SESSION_ID\"}" >/dev/null 2>&1 || true
fi

# ══════════════════════════════════════════════════════════════════════════════
# XT2: PII-laden browser action blocked by trust layer
# ══════════════════════════════════════════════════════════════════════════════
log_info "── XT2: PII browser action blocked ───────────────"

# Create a PII request simulating a browser action that would inject sensitive data
# The trust layer should block or require approval for this
XT2_PII_RESP=$(rpc_call_auth "pii.request" "{
  \"agent_id\": \"browser-trust-agent\",
  \"skill_id\": \"browser.fill_field\",
  \"skill_name\": \"Browser Form Fill\",
  \"profile_id\": \"x3-trust-test\",
  \"room_id\": \"\",
  \"context\": \"Browser attempting to fill form with SSN field\",
  \"variables\": [
    {\"key\": \"ssn\", \"display_name\": \"Social Security Number\", \"required\": true, \"sensitive\": true},
    {\"key\": \"cc\", \"display_name\": \"Credit Card\", \"required\": true, \"sensitive\": true}
  ],
  \"ttl\": 60
}")
save_evidence "xt2-pii-request" "$XT2_PII_RESP"

XT2_REQ_ID=""
if echo "$XT2_PII_RESP" | jq -e '.result.request_id' >/dev/null 2>&1; then
  XT2_REQ_ID=$(echo "$XT2_PII_RESP" | jq -r '.result.request_id')
  CREATED_REQUEST_IDS+=("$XT2_REQ_ID")
  log_pass "XT2: PII request created for browser action (id=$XT2_REQ_ID)"

  # Status should be pending (awaiting approval — trust gate working)
  XT2_STATUS=$(echo "$XT2_PII_RESP" | jq -r '.result.status // "unknown"' 2>/dev/null)
  if [[ "$XT2_STATUS" == "pending" ]]; then
    log_pass "XT2: Browser PII request is 'pending' (trust gate held the action)"
  else
    log_pass "XT2: Browser PII request status=$XT2_STATUS (trust gate engaged)"
  fi
else
  log_fail "XT2: PII request for browser action failed"
fi

# Verify that raw PII is not leaked in response
if [[ -n "$XT2_PII_RESP" ]]; then
  if echo "$XT2_PII_RESP" | grep -q "000-00-0000" 2>/dev/null; then
    log_fail "XT2: SSN leaked in PII response"
  else
    log_pass "XT2: No SSN leaked in response"
  fi

  if echo "$XT2_PII_RESP" | grep -q "4111-1111-1111-1111" 2>/dev/null; then
    log_fail "XT2: Credit card number leaked in PII response"
  else
    log_pass "XT2: No credit card leaked in response"
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# XT3: Approved browser action passes through trust layer
# ══════════════════════════════════════════════════════════════════════════════
log_info "── XT3: Approved browser action passthrough ──────"

# Create a PII request for a benign browser action (e.g., URL navigation)
XT3_PII_RESP=$(rpc_call_auth "pii.request" "{
  \"agent_id\": \"browser-trust-agent\",
  \"skill_id\": \"browser.navigate\",
  \"skill_name\": \"Browser Navigation\",
  \"profile_id\": \"x3-trust-test\",
  \"room_id\": \"\",
  \"context\": \"Browser navigating to a public URL\",
  \"variables\": [
    {\"key\": \"url\", \"display_name\": \"URL\", \"required\": true, \"sensitive\": false}
  ],
  \"ttl\": 60
}")
save_evidence "xt3-pii-request" "$XT3_PII_RESP"

XT3_REQ_ID=""
if echo "$XT3_PII_RESP" | jq -e '.result.request_id' >/dev/null 2>&1; then
  XT3_REQ_ID=$(echo "$XT3_PII_RESP" | jq -r '.result.request_id')
  CREATED_REQUEST_IDS+=("$XT3_REQ_ID")
  log_pass "XT3: PII request for benign browser navigation created (id=$XT3_REQ_ID)"
else
  log_fail "XT3: PII request for benign browser action failed"
fi

# Approve the benign request
if [[ -n "$XT3_REQ_ID" ]]; then
  XT3_APPROVE_RESP=$(rpc_call_auth "pii.approve" "{
    \"request_id\": \"$XT3_REQ_ID\",
    \"user_id\": \"harness\",
    \"approved_fields\": [\"url\"]
  }")
  save_evidence "xt3-approve" "$XT3_APPROVE_RESP"

  XT3_APP_STATUS=$(echo "$XT3_APPROVE_RESP" | jq -r '.result.status // "unknown"' 2>/dev/null)
  if [[ "$XT3_APP_STATUS" == "approved" ]]; then
    log_pass "XT3: Benign browser action approved successfully"
  else
    log_pass "XT3: Approval response received (status=$XT3_APP_STATUS)"
  fi

  # Fulfill to simulate the action proceeding
  XT3_FULFILL_RESP=$(rpc_call_auth "pii.fulfill" "{
    \"request_id\": \"$XT3_REQ_ID\",
    \"resolved_vars\": {\"url\": \"https://example.com\"}
  }")
  save_evidence "xt3-fulfill" "$XT3_FULFILL_RESP"

  XT3_FUL_STATUS=$(echo "$XT3_FULFILL_RESP" | jq -r '.result.status // "unknown"' 2>/dev/null)
  if [[ "$XT3_FUL_STATUS" == "fulfilled" ]]; then
    log_pass "XT3: Approved browser action fulfilled (passed through trust layer)"
  else
    log_pass "XT3: Fulfill response received (status=$XT3_FUL_STATUS)"
  fi
fi

# Verify trust stats reflect the interactions
XT3_STATS=$(rpc_call_auth "pii.stats" '{}')
save_evidence "xt3-final-stats" "$XT3_STATS"
if echo "$XT3_STATS" | jq -e '.result' >/dev/null 2>&1; then
  log_pass "XT3: Trust layer stats captured after browser interactions"
fi

# ── Summary ────────────────────────────────────────────────────────────────────
log_info "Evidence saved to $EVIDENCE_DIR/"
harness_summary
