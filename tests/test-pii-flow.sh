#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# test-pii-flow.sh — PII Trust Flow Harness
#
# Validates the complete PII trust lifecycle through bridge RPC:
#   request → list_pending → status → approve → fulfill → stats
#
# Usage:  bash tests/test-pii-flow.sh
# Tier:   A (VPS — calls bridge RPC via socket/HTTPS)
# ──────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/load_env.sh"
source "${SCRIPT_DIR}/lib/transport.sh"
source "${SCRIPT_DIR}/lib/common_output.sh"

# ── Test configuration ─────────────────────────────────────────────────────────
TEST_ID="pii-flow-$$-$(date +%s)"
TEST_AGENT_ID="pii-flow-agent-${TEST_ID}"
TEST_SKILL_ID="pii-flow-skill-${TEST_ID}"
TEST_PROFILE_ID="pii-flow-profile-${TEST_ID}"
EVIDENCE_DIR="${SCRIPT_DIR}/../.sisyphus/evidence/pii-flow"
mkdir -p "$EVIDENCE_DIR"

# ── Track request IDs for cleanup ──────────────────────────────────────────────
CREATED_REQUEST_IDS=()
cleanup_requests() {
  for rid in "${CREATED_REQUEST_IDS[@]:-}"; do
    rpc_call "pii.cancel" "{\"request_id\":\"$rid\"}" >/dev/null 2>&1 || true
  done
}
trap cleanup_requests EXIT

# ── Dependency check ───────────────────────────────────────────────────────────
command -v jq >/dev/null 2>&1 || { log_fail "jq is required"; harness_summary; exit 1; }

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

# ── RPC helpers ────────────────────────────────────────────────────────────────

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

# ── Helper: extract request_id flexibly ────────────────────────────────────────
# Tries common field names: result.request_id, result.id, result.req_id
extract_request_id() {
  local json="$1"
  local rid=""
  # Try result.request_id first (standard)
  rid=$(echo "$json" | jq -r '.result.request_id' 2>/dev/null) || true
  if [[ -n "$rid" && "$rid" != "null" ]]; then
    echo "$rid"
    return 0
  fi
  # Fallback: result.id
  rid=$(echo "$json" | jq -r '.result.id' 2>/dev/null) || true
  if [[ -n "$rid" && "$rid" != "null" ]]; then
    echo "$rid"
    return 0
  fi
  # Fallback: result.req_id
  rid=$(echo "$json" | jq -r '.result.req_id' 2>/dev/null) || true
  if [[ -n "$rid" && "$rid" != "null" ]]; then
    echo "$rid"
    return 0
  fi
  return 1
}

# ══════════════════════════════════════════════════════════════════════════════
# Pre-flight
# ══════════════════════════════════════════════════════════════════════════════

log_info "Detecting transport..."
detect_transport

if ! $HAS_SOCKET && ! $HAS_HTTP; then
  log_fail "No transport available (neither socket nor HTTP)"
  harness_summary
  exit 1
fi

if [ -z "${ADMIN_TOKEN:-}" ]; then
  log_fail "ADMIN_TOKEN is empty — required for RPC authentication"
  harness_summary
  exit 1
fi

echo ""
echo "========================================="
echo " PII Flow Test"
echo " Test ID: ${TEST_ID}"
echo "========================================="
echo ""

# ── Session state ──────────────────────────────────────────────────────────────
REQ_ID=""
PII_FLOW_PASS=0
PII_FLOW_TOTAL=6

# ══════════════════════════════════════════════════════════════════════════════
# Test 1: pii.request — request PII access
# ══════════════════════════════════════════════════════════════════════════════
log_info "── T1: pii.request ─────────────────────────────"

T1_RESP=$(rpc_call "pii.request" "{
  \"agent_id\": \"${TEST_AGENT_ID}\",
  \"skill_id\": \"${TEST_SKILL_ID}\",
  \"skill_name\": \"pii_flow_test\",
  \"profile_id\": \"${TEST_PROFILE_ID}\",
  \"room_id\": \"\",
  \"context\": \"PII flow test: request approve fulfill cycle\",
  \"variables\": [
    {\"key\": \"api_key\", \"display_name\": \"API Key\", \"required\": true, \"sensitive\": true},
    {\"key\": \"user_email\", \"display_name\": \"User Email\", \"required\": false, \"sensitive\": true}
  ],
  \"ttl\": 300
}")

save_evidence "t1-request" "$T1_RESP"

# Extract request ID using flexible helper
REQ_ID=$(extract_request_id "$T1_RESP") || true

if [[ -n "$REQ_ID" && "$REQ_ID" != "null" ]]; then
  CREATED_REQUEST_IDS+=("$REQ_ID")
  log_pass "T1: pii.request returned request_id=$REQ_ID"
  PII_FLOW_PASS=$((PII_FLOW_PASS + 1))
else
  log_fail "T1: pii.request did not return a request_id — response: $(echo "$T1_RESP" | jq -c . 2>/dev/null || echo "$T1_RESP")"
fi

# Verify status is pending
T1_STATUS=$(echo "$T1_RESP" | jq -r '.result.status' 2>/dev/null || echo "")
if [[ "$T1_STATUS" == "pending" ]]; then
  log_pass "T1: Initial status is 'pending'"
else
  log_fail "T1: Expected status 'pending', got '$T1_STATUS'"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# Test 2: pii.list_pending — verify request appears in pending list
# ══════════════════════════════════════════════════════════════════════════════
log_info "── T2: pii.list_pending ─────────────────────────"

T2_RESP=$(rpc_call "pii.list_pending" "{}")
save_evidence "t2-list-pending" "$T2_RESP"

if [[ -n "$REQ_ID" ]]; then
  # Try to find our request in the pending list — check common response shapes
  T2_FOUND=false

  # Shape 1: result.requests[] — array of request objects
  if echo "$T2_RESP" | jq -e --arg rid "$REQ_ID" '.result.requests[] | select(.request_id == $rid)' >/dev/null 2>&1; then
    T2_FOUND=true
  fi

  # Shape 2: result[] — top-level array
  if ! $T2_FOUND && echo "$T2_RESP" | jq -e --arg rid "$REQ_ID" '.result[] | select(.request_id == $rid)' >/dev/null 2>&1; then
    T2_FOUND=true
  fi

  # Shape 3: result.pending[] — nested pending array
  if ! $T2_FOUND && echo "$T2_RESP" | jq -e --arg rid "$REQ_ID" '.result.pending[] | select(.request_id == $rid)' >/dev/null 2>&1; then
    T2_FOUND=true
  fi

  # Shape 4: result.items[] — items array
  if ! $T2_FOUND && echo "$T2_RESP" | jq -e --arg rid "$REQ_ID" '.result.items[] | select(.request_id == $rid)' >/dev/null 2>&1; then
    T2_FOUND=true
  fi

  if $T2_FOUND; then
    log_pass "T2: Request $REQ_ID found in pending list"
    PII_FLOW_PASS=$((PII_FLOW_PASS + 1))
  else
    log_fail "T2: Request $REQ_ID not found in pending list — response: $(echo "$T2_RESP" | jq -c . 2>/dev/null || echo "$T2_RESP")"
  fi
else
  log_skip "T2: Skipped — no request_id from T1"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# Test 3: pii.status — check request status is pending
# ══════════════════════════════════════════════════════════════════════════════
log_info "── T3: pii.status ──────────────────────────────"

T3_RESP=$(rpc_call "pii.status" "{\"request_id\":\"${REQ_ID}\"}")
save_evidence "t3-status-pending" "$T3_RESP"

T3_STATUS=$(echo "$T3_RESP" | jq -r '.result.status' 2>/dev/null || echo "")

if [[ "$T3_STATUS" == "pending" ]]; then
  log_pass "T3: pii.status confirms 'pending' (request=$REQ_ID)"
  PII_FLOW_PASS=$((PII_FLOW_PASS + 1))
else
  log_fail "T3: Expected status 'pending', got '$T3_STATUS'"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# Test 4: pii.approve — approve the request
# ══════════════════════════════════════════════════════════════════════════════
log_info "── T4: pii.approve ─────────────────────────────"

T4_RESP=$(rpc_call "pii.approve" "{
  \"request_id\": \"${REQ_ID}\",
  \"user_id\": \"pii-flow-test-user\",
  \"approved_fields\": [\"api_key\", \"user_email\"]
}")
save_evidence "t4-approve" "$T4_RESP"

T4_STATUS=$(echo "$T4_RESP" | jq -r '.result.status' 2>/dev/null || echo "")

if [[ "$T4_STATUS" == "approved" ]]; then
  log_pass "T4: pii.approve — status is 'approved'"
  PII_FLOW_PASS=$((PII_FLOW_PASS + 1))
else
  log_fail "T4: Expected status 'approved', got '$T4_STATUS' — response: $(echo "$T4_RESP" | jq -c . 2>/dev/null || echo "$T4_RESP")"
fi

# Verify approval metadata
if echo "$T4_RESP" | jq -e '.result.approved_by' >/dev/null 2>&1; then
  log_pass "T4: approved_by field present"
else
  log_fail "T4: approved_by field missing"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# Test 5: pii.fulfill — fulfill the approved request
# ══════════════════════════════════════════════════════════════════════════════
log_info "── T5: pii.fulfill ─────────────────────────────"

T5_RESP=$(rpc_call "pii.fulfill" "{
  \"request_id\": \"${REQ_ID}\",
  \"resolved_vars\": {
    \"api_key\": \"{{VAULT:pii_flow_test_key}}\",
    \"user_email\": \"{{VAULT:pii_flow_test_email}}\"
  }
}")
save_evidence "t5-fulfill" "$T5_RESP"

T5_STATUS=$(echo "$T5_RESP" | jq -r '.result.status' 2>/dev/null || echo "")

if [[ "$T5_STATUS" == "fulfilled" ]]; then
  log_pass "T5: pii.fulfill — status is 'fulfilled'"
  PII_FLOW_PASS=$((PII_FLOW_PASS + 1))
else
  log_fail "T5: Expected status 'fulfilled', got '$T5_STATUS' — response: $(echo "$T5_RESP" | jq -c . 2>/dev/null || echo "$T5_RESP")"
fi

# Verify final status via pii.status
T5_FINAL_RESP=$(rpc_call "pii.status" "{\"request_id\":\"${REQ_ID}\"}")
save_evidence "t5-final-status" "$T5_FINAL_RESP"

T5_FINAL_STATUS=$(echo "$T5_FINAL_RESP" | jq -r '.result.status' 2>/dev/null || echo "")
if [[ "$T5_FINAL_STATUS" == "fulfilled" ]]; then
  log_pass "T5: Final pii.status confirms 'fulfilled'"
else
  log_fail "T5: Final status expected 'fulfilled', got '$T5_FINAL_STATUS'"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# Test 6: pii.stats — verify stats reflect the flow
# ══════════════════════════════════════════════════════════════════════════════
log_info "── T6: pii.stats ───────────────────────────────"

T6_RESP=$(rpc_call "pii.stats" "{}")
save_evidence "t6-stats" "$T6_RESP"

# Verify stats endpoint returned valid data
if echo "$T6_RESP" | jq -e '.result' >/dev/null 2>&1; then
  log_pass "T6: pii.stats returned result data"
  PII_FLOW_PASS=$((PII_FLOW_PASS + 1))
else
  log_fail "T6: pii.stats did not return valid result — response: $(echo "$T6_RESP" | jq -c . 2>/dev/null || echo "$T6_RESP")"
fi

# Log key stats for observability
T6_TOTAL=$(echo "$T6_RESP" | jq -r '.result.total // .result.total_requests // "N/A"' 2>/dev/null || echo "N/A")
T6_APPROVED=$(echo "$T6_RESP" | jq -r '.result.approved // .result.approved_count // "N/A"' 2>/dev/null || echo "N/A")
T6_FULFILLED=$(echo "$T6_RESP" | jq -r '.result.fulfilled // .result.fulfilled_count // "N/A"' 2>/dev/null || echo "N/A")
log_info "T6: Stats — total=$T6_TOTAL approved=$T6_APPROVED fulfilled=$T6_FULFILLED"

echo ""

# ── Summary ────────────────────────────────────────────────────────────────────
log_info "── Evidence saved to $EVIDENCE_DIR/ ─────────────"

echo ""
echo "========================================="
echo " PII Flow: ${PII_FLOW_PASS}/${PII_FLOW_TOTAL} PASS"
echo "========================================="
echo ""

harness_summary
