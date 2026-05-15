#!/usr/bin/env bash
# test-tls-restart-safety.sh — Verify cert + provisioning state survives restart

set -uo pipefail

_SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${_SCRIPT_DIR}/lib/load_env.sh"
source "${_SCRIPT_DIR}/lib/transport.sh"
source "${_SCRIPT_DIR}/lib/common_output.sh"
source "${_SCRIPT_DIR}/lib/restart_bridge.sh"

REPO_ROOT="$(cd "${_SCRIPT_DIR}/.." && pwd)"
EVIDENCE_DIR="${REPO_ROOT}/.sisyphus/evidence/tls"
mkdir -p "$EVIDENCE_DIR"

log_info "========================================="
log_info " TLS Restart Safety Test"
log_info "========================================="

capture_state() {
  local label="$1"
  local output_file="$2"

  local fingerprint status_tls qr_config

  fingerprint=$(ssh_vps "curl -sf -k -m 5 'https://localhost:${BRIDGE_PORT}/fingerprint'" 2>/dev/null || echo "{}")
  status_tls=$(ssh_vps "curl -sf -k -m 5 'https://localhost:${BRIDGE_PORT}/api' \
    -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"bridge.status\",\"params\":{}}'" 2>/dev/null | jq '.result.tls // {}' || echo "{}")
  qr_config=$(ssh_vps "curl -sf -k -m 5 'https://localhost:${BRIDGE_PORT}/qr/config'" 2>/dev/null || echo "{}")

  jq -nc \
    --argjson fp "$fingerprint" \
    --argjson tls "$status_tls" \
    --argjson qr "$qr_config" \
    --arg label "$label" \
    '{label: $label, fingerprint: $fp, tls: $tls, qr_config: $qr, timestamp: (now | todate)}' > "$output_file"
}

# ── Capture pre-restart state ─────────────────────────────────────────────────
log_info "Capturing pre-restart state..."
capture_state "pre-restart" "${EVIDENCE_DIR}/restart-pre-state.json"

PRE_FP=$(jq -r '.fingerprint.sha256 // empty' "${EVIDENCE_DIR}/restart-pre-state.json")
PRE_MODE=$(jq -r '.tls.mode // empty' "${EVIDENCE_DIR}/restart-pre-state.json")
PRE_QR_HOMESERVER=$(jq -r '.qr_config.config.matrix_homeserver // empty' "${EVIDENCE_DIR}/restart-pre-state.json")
PRE_QR_RPC=$(jq -r '.qr_config.config.rpc_url // empty' "${EVIDENCE_DIR}/restart-pre-state.json")

if [[ -z "$PRE_FP" ]]; then
  log_fail "Pre-restart: fingerprint not captured (bridge may not be running)"
  harness_summary
  exit 1
fi
log_pass "Pre-restart state captured — fp=${PRE_FP:0:16}... mode=$PRE_MODE"

# ── Restart bridge ────────────────────────────────────────────────────────────
log_info "Restarting bridge..."
if ! restart_bridge 60; then
  log_fail "Bridge restart failed"
  harness_summary
  exit 1
fi
log_pass "Bridge restarted successfully"

# ── Capture post-restart state ────────────────────────────────────────────────
log_info "Capturing post-restart state..."
capture_state "post-restart" "${EVIDENCE_DIR}/restart-post-state.json"

POST_FP=$(jq -r '.fingerprint.sha256 // empty' "${EVIDENCE_DIR}/restart-post-state.json")
POST_MODE=$(jq -r '.tls.mode // empty' "${EVIDENCE_DIR}/restart-post-state.json")
POST_QR_HOMESERVER=$(jq -r '.qr_config.config.matrix_homeserver // empty' "${EVIDENCE_DIR}/restart-post-state.json")
POST_QR_RPC=$(jq -r '.qr_config.config.rpc_url // empty' "${EVIDENCE_DIR}/restart-post-state.json")

# ── Compare states ────────────────────────────────────────────────────────────
PASS=true

if [[ "$PRE_FP" != "$POST_FP" ]]; then
  log_fail "Fingerprint changed: pre=${PRE_FP:0:16}... post=${POST_FP:0:16}..."
  PASS=false
else
  log_pass "Fingerprint preserved across restart"
fi

if [[ "$PRE_MODE" != "$POST_MODE" ]]; then
  log_fail "TLS mode changed: pre=$PRE_MODE post=$POST_MODE"
  PASS=false
else
  log_pass "TLS mode preserved: $POST_MODE"
fi

if [[ "$PRE_QR_HOMESERVER" != "$POST_QR_HOMESERVER" ]]; then
  log_fail "QR homeserver changed: pre=$PRE_QR_HOMESERVER post=$POST_QR_HOMESERVER"
  PASS=false
else
  log_pass "QR homeserver preserved"
fi

if [[ "$PRE_QR_RPC" != "$POST_QR_RPC" ]]; then
  log_fail "QR RPC URL changed: pre=$PRE_QR_RPC post=$POST_QR_RPC"
  PASS=false
else
  log_pass "QR RPC URL preserved"
fi

# ── Update checkpoint files on pass ──────────────────────────────────────────
if $PASS; then
  CHECKPOINT_DIR="${REPO_ROOT}/.sisyphus/checkpoints/tls"
  mkdir -p "$CHECKPOINT_DIR"
  cp "${EVIDENCE_DIR}/restart-post-state.json" "$CHECKPOINT_DIR/last_good_bridge_status.json" 2>/dev/null && chmod 600 "$CHECKPOINT_DIR/last_good_bridge_status.json"
  jq '.qr_config' "${EVIDENCE_DIR}/restart-post-state.json" > "$CHECKPOINT_DIR/last_good_qr_config.json" 2>/dev/null && chmod 600 "$CHECKPOINT_DIR/last_good_qr_config.json"
  log_pass "Checkpoint files updated"
fi

# ── Summary ──────────────────────────────────────────────────────────────────
_contract_save() { true; }
log_info "========================================="
if $PASS; then
  log_pass "TLS Restart Safety: ALL CHECKS PASSED"
else
  log_fail "TLS Restart Safety: SOME CHECKS FAILED"
fi
log_info "========================================="
harness_summary
$PASS
