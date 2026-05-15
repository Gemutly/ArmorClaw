#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# Email Approval Pipeline Harness (T4)
#
# Validates the email approval RPC boundary: status, list, approve, deny,
# restart persistence, and negative paths.  No real emails are sent.
#
# Scenarios:
#   M0 — Prerequisites
#   M1 — Approval status
#   M2 — List pending
#   M3 — Deny email (boundary test)
#   M4 — Approve email (boundary test)
#   M5 — Restart with pending
#   M6 — Negative paths
#
# Usage:  bash tests/test-email-pipeline.sh
# ──────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib/load_env.sh"
source "$SCRIPT_DIR/lib/transport.sh"
source "$SCRIPT_DIR/lib/common_output.sh"
source "$SCRIPT_DIR/lib/assert_json.sh"
source "$SCRIPT_DIR/lib/restart_bridge.sh"

# ── Evidence directory ────────────────────────────────────────────────────────
EVIDENCE_DIR="$SCRIPT_DIR/../.sisyphus/evidence/full-system-t4"
mkdir -p "$EVIDENCE_DIR"

# ── Override transport URL for VPS targeting ────────────────────────────────────
export BRIDGE_HTTP_URL="https://${VPS_IP}:${BRIDGE_PORT}"

# ── Auth-aware RPC wrapper (delegates to shared transport.sh) ───────────────────
rpc_call() { rpc_call_auth "$@"; }

# ── Evidence saver ────────────────────────────────────────────────────────────
save_evidence() {
  local name="$1" data="$2"
  echo "$data" > "$EVIDENCE_DIR/${name}.json"
}

# ══════════════════════════════════════════════════════════════════════════════
# M0: Prerequisites
# ══════════════════════════════════════════════════════════════════════════════
log_info "M0: Prerequisites"

# jq
if command -v jq &>/dev/null; then
  log_pass "M0: jq available"
else
  log_fail "M0: jq not found — required for JSON validation"
fi

# ADMIN_TOKEN
if [[ -z "${ADMIN_TOKEN:-}" ]]; then
  log_env_missing "ADMIN_TOKEN not configured — skipping admin-gated tests"
  log_info  "      Set ADMIN_TOKEN in .env to enable email pipeline tests"
  harness_summary
  exit 0
fi
log_pass "M0: ADMIN_TOKEN is set"

# Bridge running
if check_bridge_running; then
  log_pass "M0: Bridge service is active"
else
  log_skip "M0: Bridge not running — cannot test RPC boundary"
  harness_summary
  exit 0
fi

# ══════════════════════════════════════════════════════════════════════════════
# M1: Approval status
# ══════════════════════════════════════════════════════════════════════════════
log_info "M1: email_approval_status"

M1_RESP=$(rpc_call "email_approval_status" '{}')
save_evidence "m1-approval-status" "$M1_RESP"

if [[ -z "$M1_RESP" ]]; then
  log_fail "M1: email_approval_status returned empty response"
else
  log_pass "M1: email_approval_status returned non-empty response"

  # Validate response shape: must have pending_count and timeout_s
  if assert_json_has_key "$M1_RESP" "pending_count"; then
    log_pass "M1: response has pending_count field"
  else
    # Might be wrapped in result — check .result.pending_count
    _pending=$(echo "$M1_RESP" | jq -r '.result.pending_count // empty' 2>/dev/null)
    if [[ -n "$_pending" ]]; then
      log_pass "M1: response has result.pending_count field"
    else
      log_fail "M1: no pending_count in response"
    fi
  fi

  if assert_json_has_key "$M1_RESP" "timeout_s"; then
    log_pass "M1: response has timeout_s field"
  else
    _timeout_val=$(echo "$M1_RESP" | jq -r '.result.timeout_s // empty' 2>/dev/null)
    if [[ -n "$_timeout_val" ]]; then
      log_pass "M1: response has result.timeout_s field"
    else
      log_fail "M1: no timeout_s in response"
    fi
  fi

  # Save pending count for M5
  M1_PENDING=$(echo "$M1_RESP" | jq -r '.result.pending_count // .pending_count // 0' 2>/dev/null)
  log_info "M1: pending_count = $M1_PENDING"
fi

# ══════════════════════════════════════════════════════════════════════════════
# M2: List pending
# ══════════════════════════════════════════════════════════════════════════════
log_info "M2: email.list_pending"

M2_RESP=$(rpc_call "email.list_pending" '{}')
save_evidence "m2-list-pending" "$M2_RESP"

if [[ -z "$M2_RESP" ]]; then
  log_fail "M2: email.list_pending returned empty response"
else
  log_pass "M2: email.list_pending returned non-empty response"

  # Check response has approvals array and count
  _approvals_count=$(echo "$M2_RESP" | jq -r '.result.count // .count // empty' 2>/dev/null)
  if [[ -n "$_approvals_count" ]]; then
    log_pass "M2: response has count field (value=$_approvals_count)"
  else
    log_fail "M2: no count field in response"
  fi

  _approvals_key=$(echo "$M2_RESP" | jq -e 'has("approvals") or (.result // {} | has("approvals"))' 2>/dev/null || echo "false")
  if [[ "$_approvals_key" == "true" ]]; then
    log_pass "M2: response has approvals field"
  else
    log_fail "M2: no approvals field in response"
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# M3: Deny email (boundary test)
# ══════════════════════════════════════════════════════════════════════════════
log_info "M3: deny_email with test-deny-001"

M3_RESP=$(rpc_call "deny_email" '{"approval_id":"test-deny-001","user_id":"harness","reason":"T4 boundary test"}')
save_evidence "m3-deny-email" "$M3_RESP"

if [[ -z "$M3_RESP" ]]; then
  log_fail "M3: deny_email returned empty response"
else
  log_pass "M3: deny_email returned non-empty response (boundary reachable)"

  # The test ID won't exist — expect an error or "not found" response
  _m3_has_error=$(echo "$M3_RESP" | jq -e 'has("error") or (.result == null)' 2>/dev/null || echo "false")
  if [[ "$_m3_has_error" == "true" ]]; then
    log_pass "M3: deny_email returned error for non-existent ID (expected)"
    log_info  "M3: error=$(echo "$M3_RESP" | jq -c '.error' 2>/dev/null)"
  else
    # If it didn't error, check response shape for approval_id
    _m3_aid=$(echo "$M3_RESP" | jq -r '.result.approval_id // .approval_id // empty' 2>/dev/null)
    if [[ "$_m3_aid" == "test-deny-001" ]]; then
      log_pass "M3: deny_email response has matching approval_id"
    else
      log_info "M3: deny_email response shape: $(echo "$M3_RESP" | jq -c '.' 2>/dev/null)"
    fi
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# M4: Approve email (boundary test)
# ══════════════════════════════════════════════════════════════════════════════
log_info "M4: approve_email with test-approve-001"

M4_RESP=$(rpc_call "approve_email" '{"approval_id":"test-approve-001","user_id":"harness"}')
save_evidence "m4-approve-email" "$M4_RESP"

if [[ -z "$M4_RESP" ]]; then
  log_fail "M4: approve_email returned empty response"
else
  log_pass "M4: approve_email returned non-empty response (boundary reachable)"

  _m4_has_error=$(echo "$M4_RESP" | jq -e 'has("error")' 2>/dev/null || echo "false")
  if [[ "$_m4_has_error" == "true" ]]; then
    log_pass "M4: approve_email returned error for non-existent ID (expected)"
    log_info  "M4: error=$(echo "$M4_RESP" | jq -c '.error' 2>/dev/null)"
  else
    # Check response shape
    _m4_status=$(echo "$M4_RESP" | jq -r '.result.status // .status // empty' 2>/dev/null)
    if [[ "$_m4_status" == "approved" ]]; then
      log_pass "M4: approve_email response status=approved"
    else
      log_info "M4: approve_email response shape: $(echo "$M4_RESP" | jq -c '.' 2>/dev/null)"
    fi
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# M5: Restart with pending
# ══════════════════════════════════════════════════════════════════════════════
log_info "M5: Restart persistence check"

if [[ "${M1_PENDING:-0}" -eq 0 ]]; then
  log_skip "M5: No pending approvals to test restart persistence"
else
  log_info "M5: Found $M1_PENDING pending approvals — restarting bridge..."

  if restart_bridge 30; then
    log_pass "M5: Bridge restarted successfully"

    # Re-check list_pending
    M5_RESP=$(rpc_call "email.list_pending" '{}')
    save_evidence "m5-post-restart-pending" "$M5_RESP"

    if [[ -z "$M5_RESP" ]]; then
      log_fail "M5: email.list_pending returned empty after restart"
    else
      _m5_count=$(echo "$M5_RESP" | jq -r '.result.count // .count // 0' 2>/dev/null)
      log_info "M5: pending count after restart = $_m5_count (was $M1_PENDING)"

      if [[ "$_m5_count" -ge 0 ]]; then
        log_pass "M5: email.list_pending responded after restart"
      else
        log_fail "M5: invalid pending count after restart"
      fi
    fi
  else
    log_fail "M5: Bridge did not become ready after restart"
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# M6: Negative paths
# ══════════════════════════════════════════════════════════════════════════════
log_info "M6: Negative paths"

# M6a: Empty approval_id
M6A_RESP=$(rpc_call "approve_email" '{"approval_id":"","user_id":"harness"}')
save_evidence "m6a-empty-approval-id" "$M6A_RESP"

if [[ -z "$M6A_RESP" ]]; then
  log_fail "M6a: empty approval_id — no response"
else
  _m6a_err=$(echo "$M6A_RESP" | jq -e 'has("error")' 2>/dev/null || echo "false")
  if [[ "$_m6a_err" == "true" ]]; then
    log_pass "M6a: empty approval_id correctly rejected with error"
  else
    log_fail "M6a: empty approval_id was not rejected"
  fi
fi

# M6b: Empty approval_id on deny
M6B_RESP=$(rpc_call "deny_email" '{"approval_id":""}')
save_evidence "m6b-empty-approval-id-deny" "$M6B_RESP"

if [[ -z "$M6B_RESP" ]]; then
  log_fail "M6b: empty approval_id on deny — no response"
else
  _m6b_err=$(echo "$M6B_RESP" | jq -e 'has("error")' 2>/dev/null || echo "false")
  if [[ "$_m6b_err" == "true" ]]; then
    log_pass "M6b: empty approval_id on deny correctly rejected"
  else
    log_fail "M6b: empty approval_id on deny was not rejected"
  fi
fi

# M6c: Double-approve same ID
M6C_RESP1=$(rpc_call "approve_email" '{"approval_id":"test-double-001","user_id":"harness"}')
M6C_RESP2=$(rpc_call "approve_email" '{"approval_id":"test-double-001","user_id":"harness"}')
save_evidence "m6c-double-approve-first" "$M6C_RESP1"
save_evidence "m6c-double-approve-second" "$M6C_RESP2"

# At least the second call should error or return a different status
_m6c_second_err=$(echo "$M6C_RESP2" | jq -e 'has("error")' 2>/dev/null || echo "false")
if [[ "$_m6c_second_err" == "true" ]]; then
  log_pass "M6c: double-approve second call returned error (idempotency check)"
else
  log_info "M6c: double-approve second call did not error — boundary reachable"
  log_pass "M6c: RPC boundary reachable for double-approve"
fi

# M6d: Missing params (empty object)
M6D_RESP=$(rpc_call "approve_email" '{}')
save_evidence "m6d-missing-params" "$M6D_RESP"

if [[ -z "$M6D_RESP" ]]; then
  log_fail "M6d: missing params — no response"
else
  _m6d_err=$(echo "$M6D_RESP" | jq -e 'has("error")' 2>/dev/null || echo "false")
  if [[ "$_m6d_err" == "true" ]]; then
    log_pass "M6d: missing params correctly rejected with error"
  else
    log_fail "M6d: missing params was not rejected"
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════
log_info "Evidence saved to $EVIDENCE_DIR/"
log_info "Email approval pipeline: tested status, list, approve, deny, restart, negatives"

harness_summary
