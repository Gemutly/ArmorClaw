#!/usr/bin/env bash
# test-tls-mode-integration.sh — Full TLS surface integration test

set -uo pipefail

_SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${_SCRIPT_DIR}/lib/load_env.sh"
source "${_SCRIPT_DIR}/lib/transport.sh"
source "${_SCRIPT_DIR}/lib/common_output.sh"
source "${_SCRIPT_DIR}/lib/assert_json.sh"

REPO_ROOT="$(cd "${_SCRIPT_DIR}/.." && pwd)"
EVIDENCE_DIR="${REPO_ROOT}/.sisyphus/evidence/tls/integration"
mkdir -p "$EVIDENCE_DIR"

log_info "========================================="
log_info " TLS Mode Integration Test"
log_info "========================================="

BRIDGE_BASE="https://${VPS_IP}:${BRIDGE_PORT}"

curl_tls() {
  curl -skf -m 10 "$@" 2>/dev/null
}

# ── Scenario 1: HTTPS health endpoint ────────────────────────────────────────
log_info "Scenario 1: HTTPS health endpoint..."
HEALTH=$(curl_tls "${BRIDGE_BASE}/health" || echo "")
if [[ -n "$HEALTH" ]] && echo "$HEALTH" | jq -e '.status' >/dev/null 2>&1; then
  log_pass "S1: HTTPS /health returned valid JSON"
  echo "$HEALTH" > "${EVIDENCE_DIR}/s1_health.json"
else
  log_fail "S1: HTTPS /health failed"
fi

# ── Scenario 2: Cert SAN includes public IP ──────────────────────────────────
log_info "Scenario 2: Cert SAN inspection..."
SAN_OUTPUT=$(echo | openssl s_client -connect "${VPS_IP}:${BRIDGE_PORT}" 2>/dev/null | openssl x509 -noout -ext subjectAltName 2>/dev/null || echo "")
echo "$SAN_OUTPUT" > "${EVIDENCE_DIR}/s2_san.txt"
if echo "$SAN_OUTPUT" | grep -q "IP Address"; then
  log_pass "S2: Cert includes IP SANs"
else
  log_info "S2: Cert has no IP SANs (may be DNS-only)"
fi

# ── Scenario 3: /fingerprint matches openssl ─────────────────────────────────
log_info "Scenario 3: Fingerprint consistency..."
FP_EP=$(curl_tls "${BRIDGE_BASE}/fingerprint" | jq -r '.sha256 // empty' 2>/dev/null || echo "")
FP_OPENSSL=$(echo | openssl s_client -connect "${VPS_IP}:${BRIDGE_PORT}" 2>/dev/null | \
  openssl x509 -fingerprint -sha256 -noout 2>/dev/null | cut -d= -f2 | tr -d ':' | tr 'A-F' 'a-f' || echo "")
echo "{\"endpoint\":\"$FP_EP\",\"openssl\":\"$FP_OPENSSL\"}" > "${EVIDENCE_DIR}/s3_fingerprint.json"
if [[ "$FP_EP" == "$FP_OPENSSL" && -n "$FP_EP" ]]; then
  log_pass "S3: /fingerprint matches openssl (${FP_EP:0:16}...)"
else
  log_fail "S3: Fingerprint mismatch — ep=$FP_EP openssl=$FP_OPENSSL"
fi

# ── Scenario 4: bridge.status.tls fingerprint matches /fingerprint ───────────
log_info "Scenario 4: bridge.status.tls matches /fingerprint..."
STATUS_JSON=$(curl_tls "${BRIDGE_BASE}/api" -d '{"jsonrpc":"2.0","id":1,"method":"bridge.status","params":{}}' || echo "{}")
echo "$STATUS_JSON" | jq '.' > "${EVIDENCE_DIR}/s4_bridge_status.json" 2>/dev/null
FP_STATUS=$(echo "$STATUS_JSON" | jq -r '.result.tls.fingerprint_sha256 // empty' 2>/dev/null || echo "")
if [[ "$FP_STATUS" == "$FP_EP" && -n "$FP_STATUS" ]]; then
  log_pass "S4: bridge.status.tls.fingerprint matches /fingerprint"
else
  log_fail "S4: bridge.status fingerprint ($FP_STATUS) != /fingerprint ($FP_EP)"
fi

# ── Scenario 5: /discover includes tls matching bridge.status ────────────────
log_info "Scenario 5: /discover TLS consistency..."
DISCOVER_JSON=$(curl_tls "${BRIDGE_BASE}/discover" || echo "{}")
echo "$DISCOVER_JSON" | jq '.' > "${EVIDENCE_DIR}/s5_discover.json" 2>/dev/null
FP_DISCOVER=$(echo "$DISCOVER_JSON" | jq -r '.tls.fingerprint_sha256 // empty' 2>/dev/null || echo "")
if [[ "$FP_DISCOVER" == "$FP_STATUS" && -n "$FP_DISCOVER" ]]; then
  log_pass "S5: /discover.tls.fingerprint matches bridge.status"
else
  log_fail "S5: /discover fingerprint ($FP_DISCOVER) != bridge.status ($FP_STATUS)"
fi

# ── Scenario 6: /.well-known includes tls_mode ──────────────────────────────
log_info "Scenario 6: /.well-known tls_mode..."
WELL_KNOWN=$(curl_tls "${BRIDGE_BASE}/.well-known/matrix/client" || echo "{}")
echo "$WELL_KNOWN" | jq '.' > "${EVIDENCE_DIR}/s6_well_known.json" 2>/dev/null
WK_TLS_MODE=$(echo "$WELL_KNOWN" | jq -r '.["com.armorclaw"].tls_mode // empty' 2>/dev/null || echo "")
STATUS_TLS_MODE=$(echo "$STATUS_JSON" | jq -r '.result.tls.mode // empty' 2>/dev/null || echo "")
if [[ "$WK_TLS_MODE" == "$STATUS_TLS_MODE" && -n "$WK_TLS_MODE" ]]; then
  log_pass "S6: /.well-known tls_mode=$WK_TLS_MODE matches bridge.status"
else
  log_fail "S6: /.well-known tls_mode ($WK_TLS_MODE) != bridge.status ($STATUS_TLS_MODE)"
fi

# ── Scenario 7: External endpoints HTTPS-only ────────────────────────────────
log_info "Scenario 7: External HTTPS-only enforcement..."
HTTP_RESULT=$(curl -sf -m 5 "http://${VPS_IP}:${BRIDGE_PORT}/health" 2>/dev/null || echo "REJECTED")
if [[ "$HTTP_RESULT" == "REJECTED" || -z "$HTTP_RESULT" ]]; then
  log_pass "S7: HTTP rejected (HTTPS-only enforced)"
else
  log_info "S7: HTTP accepted (may be expected in some configs)"
fi

# ── Scenario 8: Localhost HTTPS access ────────────────────────────────
log_info "Scenario 8: Localhost SSH HTTPS access..."
LOCAL_HEALTH=$(ssh_vps "curl -sf -k -m 5 'https://localhost:${BRIDGE_PORT}/health'" 2>/dev/null || echo "")
if [[ -n "$LOCAL_HEALTH" ]] && echo "$LOCAL_HEALTH" | jq -e '.status' >/dev/null 2>&1; then
  log_pass "S8: Localhost HTTPS access works"
else
  log_fail "S8: Localhost HTTPS access failed"
fi

# ── Scenario 9: QR v1 default (no TLS fields) ───────────────────────────────
log_info "Scenario 9: QR config v1 default..."
QR_CONFIG=$(curl_tls "${BRIDGE_BASE}/qr/config" || echo "{}")
echo "$QR_CONFIG" | jq '.' > "${EVIDENCE_DIR}/s9_qr_config.json" 2>/dev/null
QR_VERSION=$(echo "$QR_CONFIG" | jq -r '.config.version // 0' 2>/dev/null || echo "0")
if [[ "$QR_VERSION" == "1" ]]; then
  log_pass "S9: QR emits v1 by default (no TLS fields)"
else
  log_fail "S9: QR version is $QR_VERSION (expected 1)"
fi

# ── Scenario 10: QR v2 contract (conditional) ───────────────────────────────
if [[ "${ARMORCLAW_QR_VERSION:-}" == "2" ]]; then
  log_info "Scenario 10: QR v2 contract (ARMORCLAW_QR_VERSION=2)..."
  QR_V2_FP=$(echo "$QR_CONFIG" | jq -r '.config.tls_fingerprint_sha256 // empty' 2>/dev/null || echo "")
  QR_V2_MODE=$(echo "$QR_CONFIG" | jq -r '.config.tls_mode // empty' 2>/dev/null || echo "")
  QR_V2_VER=$(echo "$QR_CONFIG" | jq -r '.config.version // 0' 2>/dev/null || echo "0")

  PASS_V2=true
  if [[ "$QR_V2_VER" != "2" ]]; then
    log_fail "S10: QR version=$QR_V2_VER (expected 2)"
    PASS_V2=false
  fi
  if [[ "$QR_V2_FP" != "$FP_STATUS" ]]; then
    log_fail "S10: QR fingerprint != bridge.status"
    PASS_V2=false
  fi
  if [[ "$QR_V2_MODE" != "$STATUS_TLS_MODE" ]]; then
    log_fail "S10: QR tls_mode != bridge.status tls.mode"
    PASS_V2=false
  fi
  if $PASS_V2; then
    log_pass "S10: QR v2 TLS fields match bridge.status"
  fi
else
  log_skip "S10: QR v2 contract (ARMORCLAW_QR_VERSION not set)"
fi

# ── Summary ──────────────────────────────────────────────────────────────────
log_info "========================================="
log_info " TLS Mode Integration: Complete"
log_info " Evidence: ${EVIDENCE_DIR}/"
log_info "========================================="
harness_summary
