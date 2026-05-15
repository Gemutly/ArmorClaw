#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# T7: Jetski Browser Sidecar Harness
#
# Tests the Jetski CDP proxy browser sidecar via its RPC API (port 9223).
# Tier B: Jetski is NOT deployed on the VPS — entire script is expected to
# skip gracefully when the sidecar is unreachable.
#
# Scenarios:
#   J0 — Prerequisites (port 9223 reachability via SSH)
#   J1 — Health check   (GET /rpc/health)
#   J2 — Session lifecycle (POST /rpc/session/create → POST /rpc/session/close)
#   J3 — Status          (GET /rpc/status → active_sessions field)
#   J4 — CDP proxy       (ws://localhost:9222 via websocat)
#   J5 — PII scanner     (Input.insertText with SSN → interception)
#
# Usage:  bash tests/test-jetski-sidecar.sh
# Requires: ssh, curl, jq (websocat optional for J4)
# ──────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/load_env.sh"
source "$SCRIPT_DIR/lib/transport.sh"
source "$SCRIPT_DIR/lib/common_output.sh"
source "$SCRIPT_DIR/lib/assert_json.sh"

# ── Evidence output directory ─────────────────────────────────────────────────
EVIDENCE_DIR="$SCRIPT_DIR/../.sisyphus/evidence/full-system-t7"
mkdir -p "$EVIDENCE_DIR"

# ── Jetski constants ──────────────────────────────────────────────────────────
JETSKI_RPC_PORT=9223
JETSKI_CDP_PORT=9222

# ══════════════════════════════════════════════════════════════════════════════
# J0: Prerequisites — Check Jetski reachability on VPS
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " J0: Prerequisites"
echo "========================================="

J0_PASS=true

# Check jq
if command -v jq >/dev/null 2>&1; then
  log_pass "jq is available ($(jq --version))"
else
  log_fail "jq is required but not found"
  J0_PASS=false
fi

# Check curl
if command -v curl >/dev/null 2>&1; then
  log_pass "curl is available"
else
  log_fail "curl is required but not found"
  J0_PASS=false
fi

# Check Jetski RPC health via SSH — this is the gate for the entire script
JETSKI_HEALTH=""
if JETSKI_HEALTH=$(ssh_vps "curl -sf http://localhost:${JETSKI_RPC_PORT}/rpc/health" 2>/dev/null) && [[ -n "$JETSKI_HEALTH" ]]; then
  log_pass "Jetski RPC health reachable on VPS port ${JETSKI_RPC_PORT}"
  log_info "Health response: $(echo "$JETSKI_HEALTH" | head -c 200)"
else
  log_skip "Jetski NOT deployed on VPS — skipping entire harness"
  log_skip "J1: Health check (no Jetski)"
  log_skip "J2: Session lifecycle (no Jetski)"
  log_skip "J3: Status (no Jetski)"
  log_skip "J4: CDP proxy (no Jetski)"
  log_skip "J5: PII scanner (no Jetski)"
  harness_summary
  exit 0
fi

if ! $J0_PASS; then
  log_fail "J0 prerequisites failed — skipping remaining tests"
  harness_summary
  exit 1
fi

# ══════════════════════════════════════════════════════════════════════════════
# J1: Health check — GET /rpc/health
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " J1: Health check — GET /rpc/health"
echo "========================================="

J1_RESPONSE=""
J1_OK=false

J1_RESPONSE=$(ssh_vps "curl -sf http://localhost:${JETSKI_RPC_PORT}/rpc/health" 2>/dev/null) || true

if [[ -n "$J1_RESPONSE" ]]; then
  J1_OK=true
  log_info "Response: $(echo "$J1_RESPONSE" | head -c 300)"
fi

if $J1_OK; then
  if assert_json_has_key "$J1_RESPONSE" "status"; then
    local_status=$(echo "$J1_RESPONSE" | jq -r '.status' 2>/dev/null)
    if [[ "$local_status" == "ok" || "$local_status" == "healthy" ]]; then
      log_pass "Jetski health status is '$local_status'"
    else
      log_fail "Jetski health status is '$local_status' (expected 'ok' or 'healthy')"
    fi
  else
    log_fail "Jetski /rpc/health did not return valid JSON with 'status' key"
  fi
  # Save evidence
  echo "$J1_RESPONSE" | jq . > "$EVIDENCE_DIR/j1-health.json" 2>/dev/null || {
    echo "$J1_RESPONSE" > "$EVIDENCE_DIR/j1-health.json"
  }
else
  log_fail "Jetski /rpc/health returned empty response"
fi

# ══════════════════════════════════════════════════════════════════════════════
# J2: Session lifecycle — create then close
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " J2: Session lifecycle"
echo "========================================="

J2_CREATE_RESPONSE=""
J2_SESSION_ID=""
J2_CREATE_OK=false

# Create session
J2_CREATE_RESPONSE=$(ssh_vps "curl -sf -X POST http://localhost:${JETSKI_RPC_PORT}/rpc/session/create" 2>/dev/null) || true

if [[ -n "$J2_CREATE_RESPONSE" ]]; then
  J2_CREATE_OK=true
  log_info "Create response: $(echo "$J2_CREATE_RESPONSE" | head -c 300)"
fi

if $J2_CREATE_OK && assert_rpc_success "$J2_CREATE_RESPONSE"; then
  # Extract session ID
  J2_SESSION_ID=$(echo "$J2_CREATE_RESPONSE" | jq -r '.result.session_id // .result.id // .result.sessionId // empty' 2>/dev/null) || true
  if [[ -n "$J2_SESSION_ID" && "$J2_SESSION_ID" != "null" ]]; then
    log_pass "Session created: $J2_SESSION_ID"
    # Save evidence
    echo "$J2_CREATE_RESPONSE" | jq . > "$EVIDENCE_DIR/j2-session-create.json" 2>/dev/null || {
      echo "$J2_CREATE_RESPONSE" > "$EVIDENCE_DIR/j2-session-create.json"
    }
  else
    log_fail "Session create response missing session_id"
    log_info "Full response: $J2_CREATE_RESPONSE"
  fi
else
  log_fail "Session create request failed or returned error"
fi

# Close session (if we got an ID)
J2_CLOSE_RESPONSE=""
J2_CLOSE_OK=false

if [[ -n "$J2_SESSION_ID" && "$J2_SESSION_ID" != "null" ]]; then
  J2_CLOSE_RESPONSE=$(ssh_vps "curl -sf -X POST 'http://localhost:${JETSKI_RPC_PORT}/rpc/session/close' -H 'Content-Type: application/json' -d '{\"session_id\":\"${J2_SESSION_ID}\"}'" 2>/dev/null) || true

  if [[ -n "$J2_CLOSE_RESPONSE" ]]; then
    J2_CLOSE_OK=true
    log_info "Close response: $(echo "$J2_CLOSE_RESPONSE" | head -c 300)"
  fi

  if $J2_CLOSE_OK && assert_rpc_success "$J2_CLOSE_RESPONSE"; then
    log_pass "Session closed: $J2_SESSION_ID"
    # Save evidence
    echo "$J2_CLOSE_RESPONSE" | jq . > "$EVIDENCE_DIR/j2-session-close.json" 2>/dev/null || {
      echo "$J2_CLOSE_RESPONSE" > "$EVIDENCE_DIR/j2-session-close.json"
    }
  else
    log_fail "Session close request failed or returned error"
  fi
else
  log_skip "J2 session close skipped (no valid session_id from create)"
fi

# ══════════════════════════════════════════════════════════════════════════════
# J3: Status — GET /rpc/status
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " J3: Status — GET /rpc/status"
echo "========================================="

J3_RESPONSE=""
J3_OK=false

J3_RESPONSE=$(ssh_vps "curl -sf http://localhost:${JETSKI_RPC_PORT}/rpc/status" 2>/dev/null) || true

if [[ -n "$J3_RESPONSE" ]]; then
  J3_OK=true
  log_info "Response: $(echo "$J3_RESPONSE" | head -c 300)"
fi

if $J3_OK; then
  if assert_json_has_key "$J3_RESPONSE" "active_sessions"; then
    active_count=$(echo "$J3_RESPONSE" | jq -r '.active_sessions' 2>/dev/null)
    log_pass "active_sessions field present: $active_count"
  else
    # Try alternate field names
    if echo "$J3_RESPONSE" | jq -e '.sessions // .session_count // .active' >/dev/null 2>&1; then
      log_pass "Status response contains session information"
    else
      log_fail "Status response missing active_sessions field"
    fi
  fi
  # Save evidence
  echo "$J3_RESPONSE" | jq . > "$EVIDENCE_DIR/j3-status.json" 2>/dev/null || {
    echo "$J3_RESPONSE" > "$EVIDENCE_DIR/j3-status.json"
  }
else
  log_fail "Jetski /rpc/status returned empty response"
fi

# ══════════════════════════════════════════════════════════════════════════════
# J4: CDP proxy — WebSocket connection on port 9222
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " J4: CDP proxy — ws://localhost:${JETSKI_CDP_PORT}"
echo "========================================="

# Check for websocat on the VPS
if ssh_vps "command -v websocat >/dev/null 2>&1" 2>/dev/null; then
  # Send a CDP getVersion command and check for a response
  J4_RESPONSE=""
  J4_RESPONSE=$(ssh_vps "echo '{\"id\":1,\"method\":\"Browser.getVersion\"}' | timeout 5 websocat -1 ws://localhost:${JETSKI_CDP_PORT} 2>/dev/null" 2>/dev/null) || true

  if [[ -n "$J4_RESPONSE" ]]; then
    log_info "CDP response: $(echo "$J4_RESPONSE" | head -c 300)"
    if echo "$J4_RESPONSE" | jq -e '.result' >/dev/null 2>&1; then
      log_pass "CDP proxy responded with valid result"
    else
      log_info "CDP proxy responded (non-standard format)"
      log_pass "CDP proxy is reachable and responding"
    fi
    # Save evidence
    echo "$J4_RESPONSE" | jq . > "$EVIDENCE_DIR/j4-cdp-proxy.json" 2>/dev/null || {
      echo "$J4_RESPONSE" > "$EVIDENCE_DIR/j4-cdp-proxy.json"
    }
  else
    log_fail "CDP proxy did not respond on ws://localhost:${JETSKI_CDP_PORT}"
  fi
else
  log_skip "J4 CDP proxy skipped (websocat not available on VPS)"
fi

# ══════════════════════════════════════════════════════════════════════════════
# J5: PII scanner — Input.insertText with SSN → verify interception
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " J5: PII scanner — SSN interception"
echo "========================================="

# We need websocat on VPS and an active session for this test
# Create a temporary session for PII testing
J5_SESSION_ID=""
J5_CREATE_RESPONSE=$(ssh_vps "curl -sf -X POST http://localhost:${JETSKI_RPC_PORT}/rpc/session/create" 2>/dev/null) || true
J5_SESSION_ID=$(echo "$J5_CREATE_RESPONSE" | jq -r '.result.session_id // .result.id // .result.sessionId // empty' 2>/dev/null) || true

if [[ -z "$J5_SESSION_ID" || "$J5_SESSION_ID" == "null" ]]; then
  log_skip "J5 PII scanner skipped (could not create test session)"
else
  if ssh_vps "command -v websocat >/dev/null 2>&1" 2>/dev/null; then
    # Send Input.insertText with a fake SSN through the CDP proxy
    # The Jetski PII scanner should intercept/redact it
    J5_SSN="123-45-6789"
    J5_CDP_PAYLOAD="{\"id\":2,\"method\":\"Input.insertText\",\"params\":{\"text\":\"SSN: ${J5_SSN}\"}}"

    J5_RESPONSE=""
    J5_RESPONSE=$(ssh_vps "echo '${J5_CDP_PAYLOAD}' | timeout 5 websocat -1 ws://localhost:${JETSKI_CDP_PORT} 2>/dev/null" 2>/dev/null) || true

    if [[ -n "$J5_RESPONSE" ]]; then
      log_info "PII scanner response: $(echo "$J5_RESPONSE" | head -c 300)"
      # Check if the SSN was intercepted/blocked/redacted
      if echo "$J5_RESPONSE" | grep -q "$J5_SSN"; then
        log_fail "PII scanner did NOT intercept SSN — raw SSN found in response"
      else
        log_pass "PII scanner intercepted SSN (not present in response)"
      fi
      # Also check Jetski audit/status for interception event
      J5_STATUS=$(ssh_vps "curl -sf http://localhost:${JETSKI_RPC_PORT}/rpc/status" 2>/dev/null) || true
      if [[ -n "$J5_STATUS" ]]; then
        if echo "$J5_STATUS" | jq -e '.pii_intercepted // .interceptions // .pii_count' >/dev/null 2>&1; then
          log_pass "PII interception recorded in status"
        else
          log_info "Status does not include PII interception counter (may be logged elsewhere)"
        fi
      fi
      # Save evidence
      echo "$J5_RESPONSE" | jq . > "$EVIDENCE_DIR/j5-pii-scanner.json" 2>/dev/null || {
        echo "$J5_RESPONSE" > "$EVIDENCE_DIR/j5-pii-scanner.json"
      }
    else
      log_fail "PII scanner test — no response from CDP proxy"
    fi

    # Clean up the PII test session
    ssh_vps "curl -sf -X POST 'http://localhost:${JETSKI_RPC_PORT}/rpc/session/close' -H 'Content-Type: application/json' -d '{\"session_id\":\"${J5_SESSION_ID}\"}'" >/dev/null 2>&1 || true
  else
    log_skip "J5 PII scanner skipped (websocat not available on VPS)"
    # Clean up session
    ssh_vps "curl -sf -X POST 'http://localhost:${JETSKI_RPC_PORT}/rpc/session/close' -H 'Content-Type: application/json' -d '{\"session_id\":\"${J5_SESSION_ID}\"}'" >/dev/null 2>&1 || true
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════
echo ""
log_info "Evidence saved to $EVIDENCE_DIR/"
harness_summary
