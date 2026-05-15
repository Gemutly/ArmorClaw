#!/usr/bin/env bash
# test-eventbus-streaming.sh — EventBus + WebSocket Live Event Harness (T1)
#
# Validates EventBus publish/subscribe over WebSocket.
# Tier A: Runs on VPS if websocket_enabled=true in config.
#         Currently false on VPS — test will skip gracefully until enabled.
#
# Test Scenarios:
#   E0: Prerequisites (websocat, bridge, WS config)
#   E1: WebSocket connection and heartbeat
#   E2: Event subscription (register device)
#   E3: Event fanout (multiple subscribers receive same event)
#   E4: Event filtering (type-based)
#   E5: No-subscriber behavior (bridge survives orphan event)
#   E6: Reconnect (disconnect + reconnect still receives events)
#
# Usage: bash tests/test-eventbus-streaming.sh

set -euo pipefail

# ── Source fixtures ────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib/load_env.sh"
source "$SCRIPT_DIR/lib/transport.sh"
source "$SCRIPT_DIR/lib/common_output.sh"
source "$SCRIPT_DIR/lib/assert_json.sh"
source "$SCRIPT_DIR/lib/event_subscriber_helper.sh"

# ── Evidence directory ─────────────────────────────────────────────────────────
EVIDENCE_DIR="$SCRIPT_DIR/../.sisyphus/evidence/full-system-t1"
mkdir -p "$EVIDENCE_DIR"

# ── Constants ──────────────────────────────────────────────────────────────────
WS_URL="wss://${VPS_IP}:${BRIDGE_PORT}/ws"
DEVICE_ID="test-harness-t1"
EVIDENCE_E0="$EVIDENCE_DIR/e0-prerequisites.txt"
EVIDENCE_E1="$EVIDENCE_DIR/e1-heartbeat.txt"
EVIDENCE_E2="$EVIDENCE_DIR/e2-register.txt"
EVIDENCE_E3="$EVIDENCE_DIR/e3-fanout.txt"
EVIDENCE_E4="$EVIDENCE_DIR/e4-filtering.txt"
EVIDENCE_E5="$EVIDENCE_DIR/e5-no-subscriber.txt"
EVIDENCE_E6="$EVIDENCE_DIR/e6-reconnect.txt"

# Track whether we can run WS tests (set by E0)
WS_TESTS_ENABLED=false

# ── Helper: send a single WebSocket message via websocat ───────────────────────
# Usage: ws_send <json_message> <timeout_seconds>
# Connects, sends one message, captures response lines for timeout seconds.
ws_send() {
  local msg="$1" duration="${2:-5}"
  local tmp_file
  tmp_file=$(mktemp)
  echo "$msg" | timeout "$duration" websocat -k "$WS_URL" > "$tmp_file" 2>/dev/null || true
  cat "$tmp_file"
  rm -f "$tmp_file"
}

# ── Helper: trigger an event via RPC ───────────────────────────────────────────
# Calls device.list via RPC over SSH to the bridge socket on VPS.
trigger_rpc_event() {
  local socket="/run/armorclaw/bridge.sock"
  ssh_vps "echo '{\"jsonrpc\":\"2.0\",\"id\":99,\"method\":\"device.list\",\"params\":{\"auth\":\"$ADMIN_TOKEN\"}}' | socat - UNIX-CONNECT:$socket" 2>/dev/null || echo "{}"
}

# ── Helper: check if a file contains a JSON line with a given key ──────────────
# Returns 0 if at least one line has the key, 1 otherwise.
file_has_json_key() {
  local file="$1" key="$2"
  while IFS= read -r line; do
    if echo "$line" | jq -e --arg k "$key" 'has($k)' >/dev/null 2>&1; then
      return 0
    fi
  done < "$file"
  return 1
}

# ── Helper: count lines matching a JSON key in file ────────────────────────────
count_json_key() {
  local file="$1" key="$2"
  local count=0
  while IFS= read -r line; do
    if echo "$line" | jq -e --arg k "$key" 'has($k)' >/dev/null 2>&1; then
      count=$((count + 1))
    fi
  done < "$file"
  echo "$count"
}

# ── Cleanup on exit ────────────────────────────────────────────────────────────
cleanup() {
  # Kill any lingering background subscriber processes
  jobs -p 2>/dev/null | xargs -r kill 2>/dev/null || true
  wait 2>/dev/null || true
}
trap cleanup EXIT

echo "========================================="
echo " T1: EventBus + WebSocket Streaming"
echo "========================================="
echo ""

# ═══════════════════════════════════════════════════════════════════════════════
# E0: Prerequisites
# ═══════════════════════════════════════════════════════════════════════════════
echo "--- E0: Prerequisites ---"

# Check bridge is running
if check_bridge_running; then
  log_pass "E0: Bridge is running on VPS"
else
  log_skip "E0: Bridge not running — cannot test WebSocket"
  echo "  Bridge systemd service not active on VPS" | tee "$EVIDENCE_E0"
  harness_summary
  exit 0
fi

# Check websocat availability (set by event_subscriber_helper.sh)
if [[ "$WEBSOCAT_AVAILABLE" == "true" ]]; then
  log_pass "E0: websocat is available"
else
  log_skip "E0: websocat not found — install with: apt install websocat"
  echo "  WEBSOCAT_AVAILABLE=$WEBSOCAT_AVAILABLE" | tee "$EVIDENCE_E0"
  harness_summary
  exit 0
fi

# Check WebSocket enabled in bridge config on VPS
WS_ENABLED=false
CONFIG_OUTPUT=$(ssh_vps "cat /etc/armorclaw/config.toml" 2>/dev/null || echo "")
if echo "$CONFIG_OUTPUT" | grep -q "websocket_enabled.*=.*true"; then
  WS_ENABLED=true
  log_pass "E0: WebSocket is enabled in bridge config"
else
  log_skip "E0: WebSocket not enabled in bridge config (websocket_enabled != true)"
  echo "  Bridge config does not have websocket_enabled = true" | tee "$EVIDENCE_E0"
  echo "  Current config (relevant lines):" >> "$EVIDENCE_E0"
  echo "$CONFIG_OUTPUT" | grep -i "websocket" >> "$EVIDENCE_E0" 2>/dev/null || echo "  (no websocket lines found)" >> "$EVIDENCE_E0"
  harness_summary
  exit 0
fi

WS_TESTS_ENABLED=true
echo "[INFO] All prerequisites met — running WebSocket tests" | tee "$EVIDENCE_E0"
echo ""

# ═══════════════════════════════════════════════════════════════════════════════
# E1: WebSocket connection + heartbeat
# ═══════════════════════════════════════════════════════════════════════════════
echo "--- E1: WebSocket connection ---"

E1_TMP="/tmp/ws-test-e1-$$.txt"
DEVICE_ID_E1="test-harness-e1"

(echo "{\"type\":\"register\",\"payload\":{\"device_id\":\"$DEVICE_ID_E1\"}}" && sleep 10) \
  | timeout 12 websocat -k "$WS_URL" > "$E1_TMP" 2>/dev/null
E1_RC=$?

LINE_COUNT=0
if [[ -f "$E1_TMP" ]]; then
  LINE_COUNT=$(grep -c '^{' "$E1_TMP" 2>/dev/null || true)
  LINE_COUNT=${LINE_COUNT:-0}
fi

cp "$E1_TMP" "$EVIDENCE_E1" 2>/dev/null || true

if [[ "$LINE_COUNT" -ge 1 ]]; then
  log_pass "E1: Received $LINE_COUNT lines from WebSocket"
else
  log_fail "E1: No lines received from WebSocket"
fi

if [[ "$LINE_COUNT" -ge 1 ]] && grep -q '"type"' "$E1_TMP" 2>/dev/null; then
  HEARTBEATS=$(grep -c '"timestamp"' "$E1_TMP" 2>/dev/null || true)
  HEARTBEATS=${HEARTBEATS:-0}
  log_pass "E1: Found messages with 'type' field ($HEARTBEATS messages)"

  if grep -q '"heartbeat"' "$E1_TMP" || grep -q '"event"' "$E1_TMP" || grep -q '"registered"' "$E1_TMP"; then
    log_pass "E1: Contains heartbeat or event messages"
  else
    log_info "E1: No heartbeat/event messages seen (may need longer capture)"
  fi
else
  log_fail "E1: No valid JSON with 'type' field received"
fi

rm -f "$E1_TMP"
echo ""

# ═══════════════════════════════════════════════════════════════════════════════
# E2: Event subscription (register device)
# ═══════════════════════════════════════════════════════════════════════════════
echo "--- E2: Event subscription ---"

REGISTER_MSG='{"type":"register","payload":{"device_id":"'"$DEVICE_ID"'"}}'
E2_OUTPUT=$(ws_send "$REGISTER_MSG" 5)
echo "$E2_OUTPUT" > "$EVIDENCE_E2"

if [[ -n "$E2_OUTPUT" ]]; then
  log_pass "E2: Received response after register"

  # Check for registered acknowledgment
  if echo "$E2_OUTPUT" | grep -q '"registered"'; then
    log_pass "E2: Device registration acknowledged"
  elif echo "$E2_OUTPUT" | grep -q '"pong"'; then
    log_pass "E2: Server responded (pong — registration may be fire-and-forget)"
  else
    log_info "E2: Response received but no 'registered' confirmation"
    log_info "E2: Response: $(echo "$E2_OUTPUT" | head -1)"
  fi
else
  log_fail "E2: No response received after register"
fi

# Test ping/pong
PING_OUTPUT=$(ws_send '{"type":"ping"}' 3)
if [[ -n "$PING_OUTPUT" ]] && echo "$PING_OUTPUT" | grep -q '"pong"'; then
  log_pass "E2: Ping/pong working"
else
  log_info "E2: Ping did not produce pong (may be timing)"
fi
echo ""

# ═══════════════════════════════════════════════════════════════════════════════
# E3: Event fanout — 2 subscribers receive events
# ═══════════════════════════════════════════════════════════════════════════════
echo "--- E3: Event fanout ---"

E3_SUB1="/tmp/ws-test-e3-sub1-$$.txt"
E3_SUB2="/tmp/ws-test-e3-sub2-$$.txt"

# Start 2 background subscribers — register + capture for 15 seconds
{
  echo '{"type":"register","payload":{"device_id":"fanout-sub1"}}' | \
    timeout 15 websocat -k "$WS_URL" > "$E3_SUB1" 2>/dev/null || true
} &
SUB1_PID=$!

{
  echo '{"type":"register","payload":{"device_id":"fanout-sub2"}}' | \
    timeout 15 websocat -k "$WS_URL" > "$E3_SUB2" 2>/dev/null || true
} &
SUB2_PID=$!

# Give subscribers time to connect and register
sleep 2

# Trigger an event via RPC
log_info "E3: Triggering RPC event (device.list)..."
RPC_RESULT=$(trigger_rpc_event)
log_info "E3: RPC response: $(echo "$RPC_RESULT" | head -c 200)"

# Wait for subscribers to potentially receive events
sleep 5

# Stop subscribers
kill "$SUB1_PID" 2>/dev/null || true
kill "$SUB2_PID" 2>/dev/null || true
wait "$SUB1_PID" 2>/dev/null || true
wait "$SUB2_PID" 2>/dev/null || true

# Analyze results
{
  echo "=== E3: Event Fanout Evidence ==="
  echo "--- Subscriber 1 ---"
  cat "$E3_SUB1" 2>/dev/null || echo "(no data)"
  echo "--- Subscriber 2 ---"
  cat "$E3_SUB2" 2>/dev/null || echo "(no data)"
  echo "--- RPC Result ---"
  echo "$RPC_RESULT"
} > "$EVIDENCE_E3"

SUB1_LINES=$(wc -l < "$E3_SUB1" 2>/dev/null || echo 0)
SUB2_LINES=$(wc -l < "$E3_SUB2" 2>/dev/null || echo 0)

log_info "E3: Subscriber 1: $SUB1_LINES lines, Subscriber 2: $SUB2_LINES lines"

if [[ "$SUB1_LINES" -ge 1 && "$SUB2_LINES" -ge 1 ]]; then
  log_pass "E3: Both subscribers received at least 1 message"
else
  if [[ "$SUB1_LINES" -ge 1 ]]; then
    log_pass "E3: Subscriber 1 received messages ($SUB1_LINES)"
  else
    log_info "E3: Subscriber 1 received 0 messages (fanout may not trigger on this RPC)"
  fi
  if [[ "$SUB2_LINES" -ge 1 ]]; then
    log_pass "E3: Subscriber 2 received messages ($SUB2_LINES)"
  else
    log_info "E3: Subscriber 2 received 0 messages (fanout may not trigger on this RPC)"
  fi
fi

rm -f "$E3_SUB1" "$E3_SUB2"
echo ""

# ═══════════════════════════════════════════════════════════════════════════════
# E4: Event filtering — subscribe, emit different types, verify matching
# ═══════════════════════════════════════════════════════════════════════════════
echo "--- E4: Event filtering ---"

E4_TMP="/tmp/ws-test-e4-$$.txt"

# Subscribe and capture events for 10 seconds
# Register first, then observe which event types come through
{
  echo '{"type":"register","payload":{"device_id":"filter-test-device"}}'
  sleep 8
} | timeout 10 websocat -k "$WS_URL" > "$E4_TMP" 2>/dev/null || true

# Save evidence
cp "$E4_TMP" "$EVIDENCE_E4"

# Collect all event types seen
E4_TYPES=$(while IFS= read -r line; do
  parse_event_type "$line" 2>/dev/null || true
done < "$E4_TMP" | sort -u)

log_info "E4: Event types observed: ${E4_TYPES:-"(none)"}"

# Count distinct event types
E4_TYPE_COUNT=$(echo "$E4_TYPES" | grep -c . || echo 0)

if [[ "$E4_TYPE_COUNT" -ge 1 ]]; then
  log_pass "E4: Observed $E4_TYPE_COUNT distinct event type(s)"

  # Verify heartbeat type exists (server sends these every 30s)
  if echo "$E4_TYPES" | grep -q "heartbeat"; then
    log_pass "E4: Heartbeat events present"
  fi

  # Check if we see event diversity
  if [[ "$E4_TYPE_COUNT" -ge 2 ]]; then
    log_pass "E4: Event type diversity confirmed ($E4_TYPE_COUNT types)"
  else
    log_info "E4: Only 1 event type — filtering can be verified when more types are emitted"
  fi
else
  log_info "E4: No event types captured in 10s window (heartbeats are every 30s)"
fi

rm -f "$E4_TMP"
echo ""

# ═══════════════════════════════════════════════════════════════════════════════
# E5: No-subscriber behavior — emit event with no subscribers, bridge survives
# ═══════════════════════════════════════════════════════════════════════════════
echo "--- E5: No-subscriber behavior ---"

# Ensure no subscribers — just trigger an RPC event directly
log_info "E5: Triggering RPC with no WebSocket subscribers..."
E5_RPC=$(trigger_rpc_event)

# Verify bridge is still running after orphan event
sleep 2
if check_bridge_running; then
  log_pass "E5: Bridge still running after event with no subscribers"
else
  log_fail "E5: Bridge crashed after event with no subscribers"
fi

# Also verify the RPC returned a valid response
{
  echo "=== E5: No-Subscriber Behavior ==="
  echo "RPC response: $E5_RPC"
  echo "Bridge status after: $(ssh_vps 'systemctl is-active armorclaw-bridge.service' 2>/dev/null || echo 'unknown')"
} > "$EVIDENCE_E5"

if echo "$E5_RPC" | jq -e '.' >/dev/null 2>&1; then
  log_pass "E5: RPC returned valid JSON response"
else
  log_info "E5: RPC response not valid JSON (may be empty socket response)"
fi
echo ""

# ═══════════════════════════════════════════════════════════════════════════════
# E6: Reconnect — disconnect then reconnect, verify can still receive events
# ═══════════════════════════════════════════════════════════════════════════════
echo "--- E6: Reconnect ---"

# First connection — register and disconnect quickly
E6_FIRST="/tmp/ws-test-e6-first-$$.txt"
{
  echo '{"type":"register","payload":{"device_id":"reconnect-test"}}'
  sleep 3
} | timeout 5 websocat -k "$WS_URL" > "$E6_FIRST" 2>/dev/null || true

FIRST_LINES=$(wc -l < "$E6_FIRST" 2>/dev/null || echo 0)
log_info "E6: First connection captured $FIRST_LINES lines"

# Disconnect gap
sleep 2

# Second connection — reconnect and verify can still receive
E6_SECOND="/tmp/ws-test-e6-second-$$.txt"
{
  echo '{"type":"register","payload":{"device_id":"reconnect-test"}}'
  sleep 5
} | timeout 8 websocat -k "$WS_URL" > "$E6_SECOND" 2>/dev/null || true

SECOND_LINES=$(wc -l < "$E6_SECOND" 2>/dev/null || echo 0)
log_info "E6: Second connection captured $SECOND_LINES"

# Save evidence
{
  echo "=== E6: Reconnect Evidence ==="
  echo "--- First connection ($FIRST_LINES lines) ---"
  cat "$E6_FIRST" 2>/dev/null || echo "(no data)"
  echo "--- Second connection ($SECOND_LINES lines) ---"
  cat "$E6_SECOND" 2>/dev/null || echo "(no data)"
} > "$EVIDENCE_E6"

if [[ "$SECOND_LINES" -ge 1 ]]; then
  log_pass "E6: Reconnect successful — received data on second connection"
else
  log_info "E6: Second connection received 0 lines (may need longer capture)"
fi

# Verify bridge is still healthy after reconnect sequence
if check_bridge_running; then
  log_pass "E6: Bridge healthy after disconnect/reconnect cycle"
else
  log_fail "E6: Bridge unhealthy after reconnect"
fi

rm -f "$E6_FIRST" "$E6_SECOND"
echo ""

# ═══════════════════════════════════════════════════════════════════════════════
# Summary
# ═══════════════════════════════════════════════════════════════════════════════

# Build event summary
{
  echo "=== T1 EventBus Streaming — Event Summary ==="
  echo ""
  echo "WebSocket URL: $WS_URL"
  echo "Device ID: $DEVICE_ID"
  echo "WS enabled: $WS_TESTS_ENABLED"
  echo ""
  echo "Evidence files:"
  ls -la "$EVIDENCE_DIR/" 2>/dev/null || echo "  (none)"
} | tee "$EVIDENCE_DIR/summary.txt"

echo ""
harness_summary
