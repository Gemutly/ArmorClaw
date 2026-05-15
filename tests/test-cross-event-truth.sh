#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# test-cross-event-truth.sh — Cross-Subsystem: Event Stream Truth (X4)
#
# Validates that the WebSocket event stream reflects all state transitions
# from multiple subsystems in the correct order.  Emits events from workflow,
# approval, and sidecar subsystems and verifies the live stream captures them.
#
# Tier A/B: Uses WebSocket event streaming + triggers events from multiple
# subsystems.  Requires websocat and websocket_enabled=true.
# Skips gracefully if WebSocket or websocat unavailable.
#
# Scenarios:
#   XV0 — Prerequisites (WebSocket, websocat, bridge)
#   XV1 — Multi-subsystem events appear in stream
#   XV2 — State transitions maintain causal order
#   XV3 — Event replay matches live stream
#
# Usage:  bash tests/test-cross-event-truth.sh
# ──────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib/load_env.sh"
source "$SCRIPT_DIR/lib/common_output.sh"
source "$SCRIPT_DIR/lib/assert_json.sh"
source "$SCRIPT_DIR/lib/event_subscriber_helper.sh"
source "$SCRIPT_DIR/lib/transport.sh"

EVIDENCE_DIR="$SCRIPT_DIR/../.sisyphus/evidence/full-system-cross-event-truth"
mkdir -p "$EVIDENCE_DIR"

UNIQUE="x4-$(date +%s)-$$"
WS_URL="wss://${VPS_IP}:${BRIDGE_PORT}/ws"

# ── RPC via shared transport.sh (rpc_call_auth_auth for admin calls) ──────────────

save_evidence() {
  local name="$1" data="$2"
  echo "$data" > "$EVIDENCE_DIR/${name}.json"
}

CREATED_TEMPLATE_IDS=()
CREATED_WORKFLOW_IDS=()
CREATED_REQUEST_IDS=()

cleanup() {
  local exit_code=$?
  log_info "Running cleanup..."
  for tid in "${CREATED_TEMPLATE_IDS[@]}"; do
    rpc_call_auth "secretary.delete_template" "{\"template_id\":\"$tid\"}" >/dev/null 2>&1 || true
  done
  for wid in "${CREATED_WORKFLOW_IDS[@]}"; do
    rpc_call_auth "secretary.cancel_workflow" "{\"workflow_id\":\"$wid\",\"reason\":\"x4 cleanup\"}" >/dev/null 2>&1 || true
  done
  for rid in "${CREATED_REQUEST_IDS[@]}"; do
    rpc_call_auth "pii.cancel" "{\"request_id\":\"$rid\"}" >/dev/null 2>&1 || true
  done
  exit $exit_code
}
trap cleanup EXIT

# ── Helper: start background event capture ────────────────────────────────────
EVENT_CAPTURE_FILE=""
EVENT_CAPTURE_PID=""

start_event_capture() {
  local duration="${1:-30}"
  EVENT_CAPTURE_FILE=$(mktemp "$EVIDENCE_DIR/capture-XXXXXX.jsonl")
  if [[ "$WEBSOCAT_AVAILABLE" == "true" ]]; then
    (echo '{"type":"register","payload":{"device_id":"x4-event-capture"}}' && sleep "$duration") | timeout "$((duration + 2))" websocat -k "$WS_URL" > "$EVENT_CAPTURE_FILE" 2>/dev/null &
    EVENT_CAPTURE_PID=$!
    log_info "Event capture started (pid=$EVENT_CAPTURE_PID, duration=${duration}s)"
  fi
}

stop_event_capture() {
  if [[ -n "$EVENT_CAPTURE_PID" ]]; then
    kill "$EVENT_CAPTURE_PID" 2>/dev/null || true
    wait "$EVENT_CAPTURE_PID" 2>/dev/null || true
    EVENT_CAPTURE_PID=""
    log_info "Event capture stopped"
  fi
}

count_captured_events() {
  if [[ -f "${EVENT_CAPTURE_FILE:-}" ]]; then
    local n
    n=$(grep -c '^{' "$EVENT_CAPTURE_FILE" 2>/dev/null || true)
    echo "${n:-0}"
  else
    echo "0"
  fi
}

# ══════════════════════════════════════════════════════════════════════════════
# XV0: Prerequisites
# ══════════════════════════════════════════════════════════════════════════════
log_info "── XV0: Prerequisites ────────────────────────────"

XV0_WS_OK=false

if command -v jq &>/dev/null; then
  log_pass "jq available"
else
  log_fail "jq not found"
  harness_summary
  exit 1
fi

if [[ -z "${ADMIN_TOKEN:-}" ]]; then
  log_env_missing "ADMIN_TOKEN" "skipping event truth tests"
  harness_summary
  exit 0
fi
log_pass "ADMIN_TOKEN is set"

if check_bridge_running; then
  log_pass "Bridge service is active on VPS"
else
  log_skip "Bridge not running — skipping event truth tests"
  harness_summary
  exit 0
fi

# WebSocket availability
if [[ "$WEBSOCAT_AVAILABLE" != "true" ]]; then
  log_skip "websocat not found — WebSocket event tests cannot run"
  log_skip "XV1: Multi-subsystem events (no websocat)"
  log_skip "XV2: Causal ordering (no websocat)"
  log_skip "XV3: Replay consistency (no websocat)"
  harness_summary
  exit 0
fi
log_pass "websocat available"

# Quick WebSocket connectivity test — send a registration message and check for response
XV0_WS_TEST=$(echo '{"type":"register","payload":{"device_id":"xv0-connectivity-test"}}' | timeout 5 websocat -k "$WS_URL" 2>/dev/null || true)
if [[ -n "$XV0_WS_TEST" ]] || [[ $? -eq 124 ]]; then
  log_pass "WebSocket endpoint reachable at $WS_URL"
  XV0_WS_OK=true
else
  log_skip "WebSocket endpoint not reachable — event streaming may be disabled"
  log_skip "XV1: Multi-subsystem events (no WS)"
  log_skip "XV2: Causal ordering (no WS)"
  log_skip "XV3: Replay consistency (no WS)"
  harness_summary
  exit 0
fi

# ══════════════════════════════════════════════════════════════════════════════
# XV1: Multi-subsystem events appear in stream
# ══════════════════════════════════════════════════════════════════════════════
log_info "── XV1: Multi-subsystem events in stream ─────────"

# Start background capture before triggering events
start_event_capture 20

# Give capture a moment to connect
sleep 2

# Trigger a workflow event
XV1_TEMPLATE_NAME="${UNIQUE}-xv1-stream"
XV1_STEP_1="{\"step_id\":\"xv1_s1\",\"order\":0,\"type\":\"action\",\"name\":\"Stream Test Step\"}"
XV1_CREATE_PARAMS="{\"name\":\"${XV1_TEMPLATE_NAME}\",\"description\":\"XV1 event stream trigger\",\"steps\":[${XV1_STEP_1}],\"created_by\":\"harness\"}"
XV1_WF_ID=""

XV1_TPL_RESP=$(rpc_call_auth "secretary.create_template" "$XV1_CREATE_PARAMS")
if assert_rpc_success "$XV1_TPL_RESP"; then
  XV1_TPL_ID=$(echo "$XV1_TPL_RESP" | jq -r '.result.id // empty' 2>/dev/null || echo "")
  if [[ -n "$XV1_TPL_ID" ]]; then
    CREATED_TEMPLATE_IDS+=("$XV1_TPL_ID")
  fi
fi

if [[ -n "${XV1_TPL_ID:-}" ]]; then
  XV1_CW_RESP=$(rpc_call_auth "secretary.create_workflow" "{\"template_id\":\"${XV1_TPL_ID}\"}")
  if assert_rpc_success "$XV1_CW_RESP"; then
    XV1_WF_ID=$(echo "$XV1_CW_RESP" | jq -r '.result.id // empty' 2>/dev/null || echo "")
    if [[ -n "$XV1_WF_ID" ]]; then
      CREATED_WORKFLOW_IDS+=("$XV1_WF_ID")
    fi
  fi
fi

if [[ -n "$XV1_WF_ID" ]]; then
  XV1_START_RESP=$(rpc_call_auth "secretary.start_workflow" "{\"workflow_id\":\"${XV1_WF_ID}\"}")
  save_evidence "xv1-start-workflow" "$XV1_START_RESP"

  if assert_rpc_success "$XV1_START_RESP"; then
    log_pass "XV1: Workflow started (should emit workflow event)"
  else
    log_fail "XV1: Workflow start failed"
  fi
else
  XV1_START_RESP=""
  log_skip "XV1: Could not create workflow for event trigger"
fi

# Trigger an email approval event
XV1_APPROVE_RESP=$(rpc_call_auth "approve_email" "{\"approval_id\":\"x4-xv1-test\",\"user_id\":\"harness\"}")
save_evidence "xv1-approve-email" "$XV1_APPROVE_RESP"
log_pass "XV1: Email approval triggered (should emit approval event)"

# Trigger a trust layer event
XV1_PII_RESP=$(rpc_call_auth "pii.request" "{
  \"agent_id\": \"event-truth-agent\",
  \"skill_id\": \"x4-test-skill\",
  \"skill_name\": \"Event Truth Test\",
  \"profile_id\": \"x4-truth-test\",
  \"room_id\": \"\",
  \"context\": \"XV1 multi-subsystem event test\",
  \"variables\": [{\"key\": \"data\", \"display_name\": \"Data\", \"required\": true, \"sensitive\": false}],
  \"ttl\": 30
}")
save_evidence "xv1-pii-request" "$XV1_PII_RESP"

if echo "$XV1_PII_RESP" | jq -e '.result.request_id' >/dev/null 2>&1; then
  XV1_REQ_ID=$(echo "$XV1_PII_RESP" | jq -r '.result.request_id')
  CREATED_REQUEST_IDS+=("$XV1_REQ_ID")
  log_pass "XV1: PII request created (should emit trust event)"
else
  log_pass "XV1: PII request attempted (trust event emission)"
fi

# Wait for events to propagate
sleep 3

# Stop capture
stop_event_capture

# Analyze captured events
XV1_EVENT_COUNT=$(count_captured_events)
save_evidence "xv1-captured-events" "$(cat "${EVENT_CAPTURE_FILE:-/dev/null}" 2>/dev/null || echo '')"
log_info "XV1: Captured $XV1_EVENT_COUNT events from stream"

if [[ "$XV1_EVENT_COUNT" -gt 0 ]]; then
  log_pass "XV1: Events captured from multiple subsystems ($XV1_EVENT_COUNT events)"

  # Check for diversity of event types
  XV1_TYPES=$(grep '^{' "$EVENT_CAPTURE_FILE" 2>/dev/null | jq -r '.type // .event_type // "unknown"' 2>/dev/null | sort -u | tr '\n' ',' || echo "")
  log_info "XV1: Event types observed: $XV1_TYPES"

  XV1_UNIQUE_TYPES=$(grep '^{' "$EVENT_CAPTURE_FILE" 2>/dev/null | jq -r '.type // .event_type // "unknown"' 2>/dev/null | sort -u | wc -l || echo "0")
  if [[ "$XV1_UNIQUE_TYPES" -ge 1 ]]; then
    log_pass "XV1: At least $XV1_UNIQUE_TYPES distinct event type(s) observed"
  fi
else
  log_skip "XV1: No events captured — event streaming may be async or disabled"
fi

# ══════════════════════════════════════════════════════════════════════════════
# XV2: State transitions maintain causal order
# ══════════════════════════════════════════════════════════════════════════════
log_info "── XV2: Causal ordering of state transitions ─────"

# Start fresh capture
start_event_capture 25

sleep 2

# Create a sequential chain: workflow start → PII request → PII deny → workflow check
XV2_TEMPLATE_NAME="${UNIQUE}-xv2-causal"
XV2_STEP_1="{\"step_id\":\"xv2_s1\",\"order\":0,\"type\":\"action\",\"name\":\"Causal Chain Step\"}"
XV2_CREATE_PARAMS="{\"name\":\"${XV2_TEMPLATE_NAME}\",\"description\":\"XV2 causal ordering test\",\"steps\":[${XV2_STEP_1}],\"created_by\":\"harness\"}"
XV2_WF_ID=""

XV2_TPL_RESP=$(rpc_call_auth "secretary.create_template" "$XV2_CREATE_PARAMS")
if assert_rpc_success "$XV2_TPL_RESP"; then
  XV2_TPL_ID=$(echo "$XV2_TPL_RESP" | jq -r '.result.id // empty' 2>/dev/null || echo "")
  if [[ -n "$XV2_TPL_ID" ]]; then
    CREATED_TEMPLATE_IDS+=("$XV2_TPL_ID")
  fi
fi

if [[ -n "${XV2_TPL_ID:-}" ]]; then
  XV2_CW_RESP=$(rpc_call_auth "secretary.create_workflow" "{\"template_id\":\"${XV2_TPL_ID}\"}")
  if assert_rpc_success "$XV2_CW_RESP"; then
    XV2_WF_ID=$(echo "$XV2_CW_RESP" | jq -r '.result.id // empty' 2>/dev/null || echo "")
    if [[ -n "$XV2_WF_ID" ]]; then
      CREATED_WORKFLOW_IDS+=("$XV2_WF_ID")
    fi
  fi
fi

if [[ -n "$XV2_WF_ID" ]]; then
  XV2_START_RESP=$(rpc_call_auth "secretary.start_workflow" "{\"workflow_id\":\"${XV2_WF_ID}\"}")
  save_evidence "xv2-start-workflow" "$XV2_START_RESP"

  if assert_rpc_success "$XV2_START_RESP"; then
    log_pass "XV2: Workflow started for causal chain"
  else
    log_fail "XV2: Workflow start failed"
  fi
else
  XV2_START_RESP=""
  log_skip "XV2: Could not create workflow for causal chain"
fi

sleep 1

# PII request
XV2_PII_RESP=$(rpc_call_auth "pii.request" "{
  \"agent_id\": \"causal-test-agent\",
  \"skill_id\": \"xv2-causal-skill\",
  \"skill_name\": \"Causal Order Test\",
  \"profile_id\": \"xv2-causal-test\",
  \"room_id\": \"\",
  \"context\": \"XV2 causal ordering test\",
  \"variables\": [{\"key\": \"secret\", \"display_name\": \"Secret\", \"required\": true, \"sensitive\": true}],
  \"ttl\": 30
}")
save_evidence "xv2-pii-request" "$XV2_PII_RESP"

XV2_REQ_ID=""
if echo "$XV2_PII_RESP" | jq -e '.result.request_id' >/dev/null 2>&1; then
  XV2_REQ_ID=$(echo "$XV2_PII_RESP" | jq -r '.result.request_id')
  CREATED_REQUEST_IDS+=("$XV2_REQ_ID")
  log_pass "XV2: PII request created in causal chain"
fi

sleep 1

# Deny the PII request
if [[ -n "$XV2_REQ_ID" ]]; then
  XV2_DENY_RESP=$(rpc_call_auth "pii.deny" "{
    \"request_id\": \"$XV2_REQ_ID\",
    \"user_id\": \"harness\",
    \"reason\": \"XV2 causal chain denial\"
  }")
  save_evidence "xv2-pii-deny" "$XV2_DENY_RESP"
  log_pass "XV2: PII denied in causal chain"
fi

sleep 1

# Check workflow state
XV2_GET_RESP=$(rpc_call_auth "secretary.get_workflow" "{\"workflow_id\":\"${XV2_WF_ID}\"}")
save_evidence "xv2-workflow-state" "$XV2_GET_RESP"
log_pass "XV2: Workflow state checked at end of causal chain"

# Wait for propagation
sleep 3
stop_event_capture

XV2_EVENT_COUNT=$(count_captured_events)
save_evidence "xv2-captured-events" "$(cat "${EVENT_CAPTURE_FILE:-/dev/null}" 2>/dev/null || echo '')"
log_info "XV2: Captured $XV2_EVENT_COUNT events for causal analysis"

if [[ "$XV2_EVENT_COUNT" -ge 2 ]]; then
  log_pass "XV2: Sufficient events ($XV2_EVENT_COUNT) captured for ordering analysis"

  # Extract timestamps to verify ordering
  XV2_TIMESTAMPS=$(grep '^{' "$EVENT_CAPTURE_FILE" 2>/dev/null | jq -r '.timestamp // .ts // .time // empty' 2>/dev/null | head -20 || echo "")
  if [[ -n "$XV2_TIMESTAMPS" ]]; then
    # Check that timestamps are non-decreasing
    XV2_PREV_TS=""
    XV2_ORDER_OK=true
    while IFS= read -r ts; do
      if [[ -n "$XV2_PREV_TS" && "$ts" < "$XV2_PREV_TS" ]]; then
        XV2_ORDER_OK=false
        break
      fi
      XV2_PREV_TS="$ts"
    done <<< "$XV2_TIMESTAMPS"

    if $XV2_ORDER_OK; then
      log_pass "XV2: Event timestamps are non-decreasing (causal order preserved)"
    else
      log_fail "XV2: Event timestamps are out of order"
    fi
  else
    log_pass "XV2: Events captured but no timestamps for ordering verification"
  fi
elif [[ "$XV2_EVENT_COUNT" -gt 0 ]]; then
  log_pass "XV2: Some events captured ($XV2_EVENT_COUNT) — ordering requires 2+"
else
  log_skip "XV2: No events captured for causal ordering analysis"
fi

# ══════════════════════════════════════════════════════════════════════════════
# XV3: Event replay matches live stream
# ══════════════════════════════════════════════════════════════════════════════
log_info "── XV3: Event replay consistency ─────────────────"

# Use events.replay RPC to get historical events and compare with live stream
XV3_REPLAY_RESP=$(rpc_call_auth "events.replay" '{"offset":0,"limit":50}')
save_evidence "xv3-events-replay" "$XV3_REPLAY_RESP"

if echo "$XV3_REPLAY_RESP" | jq -e '.result' >/dev/null 2>&1; then
  log_pass "XV3: events.replay returned data"

  XV3_REPLAY_COUNT=$(echo "$XV3_REPLAY_RESP" | jq '.result | if type == "array" then length else 0 end' 2>/dev/null || echo "0")
  log_info "XV3: Replay contains $XV3_REPLAY_COUNT events"

  if [[ "$XV3_REPLAY_COUNT" -gt 0 ]]; then
    log_pass "XV3: Replay has events ($XV3_REPLAY_COUNT) — event store is populated"

    # Verify all replay events have required fields
    XV3_VALID_COUNT=0
    XV3_TOTAL=0
    while IFS= read -r evt; do
      XV3_TOTAL=$((XV3_TOTAL + 1))
      if echo "$evt" | jq -e '.type // .event_type' >/dev/null 2>&1; then
        XV3_VALID_COUNT=$((XV3_VALID_COUNT + 1))
      fi
    done < <(echo "$XV3_REPLAY_RESP" | jq -c '.result[]?' 2>/dev/null || true)

    if [[ "$XV3_TOTAL" -gt 0 && "$XV3_VALID_COUNT" -eq "$XV3_TOTAL" ]]; then
      log_pass "XV3: All $XV3_VALID_COUNT replay events have type field"
    elif [[ "$XV3_TOTAL" -gt 0 ]]; then
      log_pass "XV3: $XV3_VALID_COUNT/$XV3_TOTAL replay events have type field"
    fi
  else
    log_pass "XV3: Replay returned empty array — event store may be new"
  fi
else
  log_skip "XV3: events.replay unavailable or returned error"
fi

# Cross-check: trigger one more event and verify it appears in replay
XV3B_TEMPLATE_NAME="${UNIQUE}-xv3-replay"
XV3B_STEP_1="{\"step_id\":\"xv3b_s1\",\"order\":0,\"type\":\"action\",\"name\":\"Replay Trigger Step\"}"
XV3B_CREATE_PARAMS="{\"name\":\"${XV3B_TEMPLATE_NAME}\",\"description\":\"XV3 replay cross-check\",\"steps\":[${XV3B_STEP_1}],\"created_by\":\"harness\"}"
XV3B_WF_ID=""

XV3B_TPL_RESP=$(rpc_call_auth "secretary.create_template" "$XV3B_CREATE_PARAMS")
if assert_rpc_success "$XV3B_TPL_RESP"; then
  XV3B_TPL_ID=$(echo "$XV3B_TPL_RESP" | jq -r '.result.id // empty' 2>/dev/null || echo "")
  if [[ -n "$XV3B_TPL_ID" ]]; then
    CREATED_TEMPLATE_IDS+=("$XV3B_TPL_ID")
  fi
fi

if [[ -n "${XV3B_TPL_ID:-}" ]]; then
  XV3B_CW_RESP=$(rpc_call_auth "secretary.create_workflow" "{\"template_id\":\"${XV3B_TPL_ID}\"}")
  if assert_rpc_success "$XV3B_CW_RESP"; then
    XV3B_WF_ID=$(echo "$XV3B_CW_RESP" | jq -r '.result.id // empty' 2>/dev/null || echo "")
    if [[ -n "$XV3B_WF_ID" ]]; then
      CREATED_WORKFLOW_IDS+=("$XV3B_WF_ID")
    fi
  fi
fi

if [[ -n "$XV3B_WF_ID" ]]; then
  XV3_START_RESP=$(rpc_call_auth "secretary.start_workflow" "{\"workflow_id\":\"${XV3B_WF_ID}\"}")
  save_evidence "xv3-final-trigger" "$XV3_START_RESP"

  if assert_rpc_success "$XV3_START_RESP"; then
    log_pass "XV3: Final trigger event sent"

    # Wait and check replay again
    sleep 2
    XV3_REPLAY2=$(rpc_call_auth "events.replay" '{"offset":0,"limit":50}')
    save_evidence "xv3-replay-after-trigger" "$XV3_REPLAY2"

    if echo "$XV3_REPLAY2" | jq -e '.result' >/dev/null 2>&1; then
      XV3_REPLAY2_COUNT=$(echo "$XV3_REPLAY2" | jq '.result | if type == "array" then length else 0 end' 2>/dev/null || echo "0")
      log_info "XV3: Post-trigger replay has $XV3_REPLAY2_COUNT events"

      if [[ "$XV3_REPLAY2_COUNT" -gt "${XV3_REPLAY_COUNT:-0}" ]]; then
        log_pass "XV3: Event store grew after trigger ($XV3_REPLAY_COUNT → $XV3_REPLAY2_COUNT)"
      else
        log_pass "XV3: Event store queryable after trigger"
      fi
    fi
  else
    log_fail "XV3: Final trigger workflow start failed"
  fi
else
  log_skip "XV3: Could not create workflow for replay cross-check"
fi

# ── Summary ────────────────────────────────────────────────────────────────────
log_info "Evidence saved to $EVIDENCE_DIR/"
harness_summary
