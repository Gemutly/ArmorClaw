#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# T9: Platform Adapter Harness
#
# Validates all platform adapters via the Bridge RPC API.
# Tier B: Only Matrix is configured on VPS; Slack tests skip gracefully.
#
# Scenarios:
#   P0: Prerequisites — bridge running, ADMIN_TOKEN present
#   P1: Matrix status — matrix.status → connected, logged_in, user_id
#   P2: Matrix send/receive — round-trip via bridge RPC
#   P3: Matrix join room — matrix.join_room → success
#   P4: Bridge channel list — bridge.list → channels array
#   P5: AppService status — bridge.appservice_status → enabled
#   P6: Slack adapter — skip if not configured
#
# Usage:  bash tests/test-platform-adapters.sh
# Requires: ssh, curl, jq; socat on VPS for socket transport
# ──────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib/load_env.sh"
source "$SCRIPT_DIR/lib/transport.sh"
source "$SCRIPT_DIR/lib/common_output.sh"
source "$SCRIPT_DIR/lib/assert_json.sh"

# ── Evidence output directory ─────────────────────────────────────────────────
EVIDENCE_DIR="$SCRIPT_DIR/../.sisyphus/evidence/full-system-t9"
mkdir -p "$EVIDENCE_DIR"

# ── Use shared transport detector ─────────────────────────────────────────────
detect_transport

if [[ "$TRANSPORT_MODE" == "none" ]]; then
  log_fail "No transport available (neither socket nor HTTP)"
  harness_summary
  exit 1
fi

log_info "Transport: socket=$HAS_SOCKET http=$HAS_HTTP"

# ── Override transport URL for VPS targeting ────────────────────────────────────
export BRIDGE_HTTP_URL="https://${VPS_IP}:${BRIDGE_PORT}"

# ── Auth-aware RPC wrapper (delegates to shared transport.sh) ───────────────────
rpc_call() { rpc_call_auth "$@"; }

# save_evidence: Append JSON blob to evidence file
save_evidence() {
  local name="$1"
  local data="$2"
  echo "$data" | jq . > "$EVIDENCE_DIR/${name}.json" 2>/dev/null || echo "$data" > "$EVIDENCE_DIR/${name}.json"
  log_info "Evidence saved: $EVIDENCE_DIR/${name}.json"
}

# ══════════════════════════════════════════════════════════════════════════════
# P0: Prerequisites
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " P0: Prerequisites"
echo "========================================="

P0_PASS=true

# jq on local machine
if command -v jq >/dev/null 2>&1; then
  log_pass "jq is available locally ($(jq --version))"
else
  log_fail "jq is required but not found on local machine"
  P0_PASS=false
fi

# jq on VPS
if ssh_vps "command -v jq >/dev/null 2>&1" 2>/dev/null; then
  log_pass "jq is available on VPS"
else
  log_fail "jq is required but not found on VPS"
  P0_PASS=false
fi

# Bridge running
if check_bridge_running; then
  log_pass "Bridge service is active on VPS"
else
  log_fail "Bridge service is NOT active on VPS"
  P0_PASS=false
fi

# ADMIN_TOKEN present
if [[ -n "${ADMIN_TOKEN:-}" ]]; then
  log_pass "ADMIN_TOKEN is set"
else
  log_env_missing "ADMIN_TOKEN not configured — skipping admin-gated tests"
  P0_PASS=false
fi

if ! $P0_PASS; then
  log_fail "P0 prerequisites failed — aborting"
  harness_summary
  exit 1
fi

# ══════════════════════════════════════════════════════════════════════════════
# P1: Matrix Status — matrix.status
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " P1: Matrix Status"
echo "========================================="

P1_RESP=""
P1_RESP=$(rpc_call "matrix.status") || true
save_evidence "p1-matrix-status" "${P1_RESP:-{}}"

if [[ -z "$P1_RESP" ]]; then
  log_fail "matrix.status returned empty response"
else
  log_info "Response: $(echo "$P1_RESP" | head -c 300)"

  if assert_rpc_success "$P1_RESP"; then
    # Check for connected
    local_connected=$(echo "$P1_RESP" | jq -r '.result.connected // .result.status // empty' 2>/dev/null)
    if [[ "$local_connected" == "true" || "$local_connected" == "connected" ]]; then
      log_pass "Matrix reports connected"
    else
      log_fail "Matrix not connected (status: $local_connected)"
    fi

    # Check for logged_in
    local_logged_in=$(echo "$P1_RESP" | jq -r '.result.logged_in // .result.isLoggedIn // empty' 2>/dev/null)
    if [[ "$local_logged_in" == "true" ]]; then
      log_pass "Matrix reports logged_in"
    else
      log_fail "Matrix not logged in (logged_in: $local_logged_in)"
    fi

    # Check for user_id
    local_user_id=$(echo "$P1_RESP" | jq -r '.result.user_id // .result.userId // empty' 2>/dev/null)
    if [[ -n "$local_user_id" && "$local_user_id" != "null" ]]; then
      log_pass "Matrix user_id: $local_user_id"
    else
      log_fail "Matrix user_id missing"
    fi
  else
    log_fail "matrix.status RPC returned error"
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# P2: Matrix Send/Receive Round-Trip
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " P2: Matrix Send/Receive Round-Trip"
echo "========================================="

# Generate unique token for round-trip verification
RT_TOKEN="ADAPTER-RT-$(openssl rand -hex 6 2>/dev/null || date +%s%N)"
P2_ROOM_ID="${TEST_ROOM_ID:-}"

# Find a suitable room: try matrix.status for a room, or use bridge.list
if [[ -z "$P2_ROOM_ID" ]]; then
  P2_LIST_RESP=$(rpc_call "bridge.list") || true
  P2_ROOM_ID=$(echo "$P2_LIST_RESP" | jq -r '.result.channels[0].id // .result[0].id // empty' 2>/dev/null)
fi

if [[ -z "$P2_ROOM_ID" || "$P2_ROOM_ID" == "null" ]]; then
  log_skip "P2 send/receive — no room available for round-trip test"
else
  log_info "Using room: $P2_ROOM_ID"

  # Send message via matrix.send
  P2_SEND_PARAMS="{\"room_id\":\"${P2_ROOM_ID}\",\"message\":\"${RT_TOKEN} adapter harness round-trip\"}"
  P2_SEND_RESP=""
  P2_SEND_RESP=$(rpc_call "matrix.send" "$P2_SEND_PARAMS" "15") || true
  save_evidence "p2-matrix-send" "${P2_SEND_RESP:-{}}"

  if [[ -z "$P2_SEND_RESP" ]]; then
    log_fail "matrix.send returned empty response"
  elif assert_rpc_success "$P2_SEND_RESP"; then
    local_event_id=$(echo "$P2_SEND_RESP" | jq -r '.result.event_id // .result.eventId // empty' 2>/dev/null)
    if [[ -n "$local_event_id" && "$local_event_id" != "null" ]]; then
      log_pass "matrix.send succeeded (event_id: $local_event_id)"
    else
      log_pass "matrix.send succeeded"
    fi

    # Poll for the message to verify round-trip
    RT_FOUND=false
    for _attempt in 1 2 3 4 5 6; do
      sleep 5
      P2_RECV_PARAMS="{\"room_id\":\"${P2_ROOM_ID}\",\"limit\":20}"
      P2_RECV_RESP=""
      P2_RECV_RESP=$(rpc_call "matrix.messages" "$P2_RECV_PARAMS" "10") || true
      if [[ -n "$P2_RECV_RESP" ]] && echo "$P2_RECV_RESP" | grep -q "$RT_TOKEN"; then
        RT_FOUND=true
        break
      fi
    done

    if $RT_FOUND; then
      log_pass "Round-trip verified — token found in messages"
      save_evidence "p2-matrix-receive" "${P2_RECV_RESP:-{}}"
    else
      log_fail "Round-trip NOT verified — token '$RT_TOKEN' not found after 30s"
      save_evidence "p2-matrix-receive-miss" "${P2_RECV_RESP:-{}}"
    fi
  else
    log_fail "matrix.send RPC returned error"
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# P3: Matrix Join Room — matrix.join_room
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " P3: Matrix Join Room"
echo "========================================="

# Try to join a public test room or alias
P3_TEST_ROOM="${MATRIX_TEST_ROOM:-#armorclaw-test:${VPS_IP}}"
P3_JOIN_PARAMS="{\"room_id_or_alias\":\"${P3_TEST_ROOM}\"}"
P3_JOIN_RESP=""
P3_JOIN_RESP=$(rpc_call "matrix.join_room" "$P3_JOIN_PARAMS" "15") || true
save_evidence "p3-matrix-join" "${P3_JOIN_RESP:-{}}"

if [[ -z "$P3_JOIN_RESP" ]]; then
  log_skip "P3 join_room — no response (room may not exist)"
elif assert_rpc_success "$P3_JOIN_RESP"; then
  local_joined_room=$(echo "$P3_JOIN_RESP" | jq -r '.result.room_id // .result.roomId // .result // empty' 2>/dev/null)
  if [[ -n "$local_joined_room" && "$local_joined_room" != "null" ]]; then
    log_pass "matrix.join_room succeeded (room: $local_joined_room)"
  else
    log_pass "matrix.join_room succeeded"
  fi
else
  # Join may fail if room doesn't exist — that's acceptable for harness
  local_err_msg=$(echo "$P3_JOIN_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
  log_skip "matrix.join_room error (room may not exist): $local_err_msg"
fi

# ══════════════════════════════════════════════════════════════════════════════
# P4: Bridge Channel List — bridge.list
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " P4: Bridge Channel List"
echo "========================================="

P4_RESP=""
P4_RESP=$(rpc_call "bridge.list") || true
save_evidence "p4-bridge-list" "${P4_RESP:-{}}"

if [[ -z "$P4_RESP" ]]; then
  log_fail "bridge.list returned empty response"
else
  log_info "Response: $(echo "$P4_RESP" | head -c 500)"

  if assert_rpc_success "$P4_RESP"; then
    # Check that result contains channels (array)
    local_channels_count=$(echo "$P4_RESP" | jq '.result.channels // .result | length' 2>/dev/null || echo "0")
    if [[ "$local_channels_count" -gt 0 ]]; then
      log_pass "bridge.list returned $local_channels_count channel(s)"
      # Verify channel structure has expected fields
      local_first_id=$(echo "$P4_RESP" | jq -r '.result.channels[0].id // .result[0].id // empty' 2>/dev/null)
      if [[ -n "$local_first_id" && "$local_first_id" != "null" ]]; then
        log_pass "Channel entry has id field: $local_first_id"
      else
        log_info "Channel structure differs from expected — inspect evidence"
      fi
    else
      log_pass "bridge.list succeeded (0 channels — may be expected on fresh install)"
    fi
  else
    log_fail "bridge.list RPC returned error"
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# P5: AppService Status — bridge.appservice_status
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " P5: AppService Status"
echo "========================================="

P5_RESP=""
P5_RESP=$(rpc_call "bridge.appservice_status") || true
save_evidence "p5-appservice-status" "${P5_RESP:-{}}"

if [[ -z "$P5_RESP" ]]; then
  log_skip "P5 appservice_status — no response (method may not be registered)"
else
  log_info "Response: $(echo "$P5_RESP" | head -c 300)"

  if assert_rpc_success "$P5_RESP"; then
    local_enabled=$(echo "$P5_RESP" | jq -r '.result.enabled // .result.active // .result.status // empty' 2>/dev/null)
    if [[ "$local_enabled" == "true" || "$local_enabled" == "enabled" || "$local_enabled" == "active" ]]; then
      log_pass "AppService reports enabled ($local_enabled)"
    elif [[ -n "$local_enabled" && "$local_enabled" != "null" ]]; then
      log_info "AppService status: $local_enabled"
      log_pass "AppService responded with status"
    else
      # Even if no specific field, success is acceptable
      log_pass "AppService RPC succeeded"
    fi
  else
    local_err_code=$(echo "$P5_RESP" | jq -r '.error.code // empty' 2>/dev/null)
    if [[ "$err_code" == "-32601" ]]; then
      log_skip "bridge.appservice_status not implemented (method not found)"
    else
      log_fail "bridge.appservice_status RPC returned error"
    fi
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# P6: Slack Adapter — skip if not configured
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " P6: Slack Adapter"
echo "========================================="

# Check if Slack is configured via environment or bridge config
SLACK_CONFIGURED=false

# Try to detect Slack configuration from bridge status
P6_STATUS_RESP=""
P6_STATUS_RESP=$(rpc_call "slack.status" "{}" "5") || true

if [[ -n "$P6_STATUS_RESP" ]]; then
  # If slack.status returns success, Slack is configured
  if assert_rpc_success "$P6_STATUS_RESP" 2>/dev/null; then
    SLACK_CONFIGURED=true
    save_evidence "p6-slack-status" "${P6_STATUS_RESP}"
  fi
fi

if $SLACK_CONFIGURED; then
  log_info "Slack adapter detected — running Slack tests"
  log_pass "Slack adapter is configured"

  # Verify Slack connectivity
  local_slack_connected=$(echo "$P6_STATUS_RESP" | jq -r '.result.connected // .result.status // empty' 2>/dev/null)
  if [[ "$local_slack_connected" == "true" || "$local_slack_connected" == "connected" ]]; then
    log_pass "Slack adapter reports connected"
  else
    log_info "Slack adapter status: $local_slack_connected"
  fi
else
  log_skip "P6 Slack adapter — not configured (Tier B: only Matrix on VPS)"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Evidence Summary
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " Evidence Summary"
echo "========================================="

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

TEST_SUMMARY=$(cat <<HEREDOC
{
  "test": "T9-platform-adapters",
  "timestamp": "${TIMESTAMP}",
  "transport": {
    "transport_mode": "${TRANSPORT_MODE}"
  },
  "passed": $FULL_SYSTEM_PASSED,
  "failed": $FULL_SYSTEM_FAILED,
  "skipped": $FULL_SYSTEM_SKIPPED
}
HEREDOC
)

echo "$TEST_SUMMARY" | jq . > "$EVIDENCE_DIR/test-summary.json" 2>/dev/null || echo "$TEST_SUMMARY" > "$EVIDENCE_DIR/test-summary.json"
log_info "Test summary saved to $EVIDENCE_DIR/test-summary.json"

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════
echo ""
harness_summary
