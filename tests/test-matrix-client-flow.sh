#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# Matrix Client Flow Integration Test
#
# Validates the REAL user-facing Matrix messaging path via direct Conduit API
# (the path ArmorChat uses via Matrix SDK /sync).
#
# Covers:
#   1. Login via m.login.password
#   2. Create room
#   3. Send message
#   4. Sync and verify message appears
#   5. Cleanup (leave room)
#
# Usage:
#   bash tests/test-matrix-client-flow.sh
#
# Requires: jq, openssl, SSH access to VPS with Conduit running
# ──────────────────────────────────────────────────────────────────────────────

# ── Source shared helpers ──────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/load_env.sh"
source "${SCRIPT_DIR}/lib/common_output.sh"

# ── Check prerequisites ───────────────────────────────────────────────────────
command -v jq >/dev/null 2>&1 || { echo "FAIL: jq required"; exit 1; }
command -v openssl >/dev/null 2>&1 || { echo "FAIL: openssl required"; exit 1; }

# ── Matrix credentials ────────────────────────────────────────────────────────
MATRIX_USER="bridge"
MATRIX_PASS="bridgepassword123"
CONDUIT_BASE="http://localhost:${MATRIX_PORT}"

# ── State ──────────────────────────────────────────────────────────────────────
TOKEN=""
USER_ID=""
ROOM_ID=""
TEST_MESSAGE="armorclaw-test-$(openssl rand -hex 8)"
TOTAL_TESTS=5

# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " Matrix Client Flow Test"
echo "========================================="

# ── Test 1: Login ──────────────────────────────────────────────────────────────
echo ""
log_info "Test 1/5: Login via m.login.password"

LOGIN_RESP=$(ssh_vps "curl -s -X POST '${CONDUIT_BASE}/_matrix/client/v3/login' \
  -H 'Content-Type: application/json' \
  -d '{\"type\":\"m.login.password\",\"user\":\"${MATRIX_USER}\",\"password\":\"${MATRIX_PASS}\"}'" 2>&1) || true

TOKEN=$(echo "$LOGIN_RESP" | jq -r '.access_token // empty')
USER_ID=$(echo "$LOGIN_RESP" | jq -r '.user_id // empty')

if [[ -n "$TOKEN" && -n "$USER_ID" ]]; then
  log_pass "Login succeeded (user: $USER_ID)"
else
  log_fail "Login failed — response: $(echo "$LOGIN_RESP" | head -c 300)"
fi

# ── Test 2: Create room ───────────────────────────────────────────────────────
echo ""
log_info "Test 2/5: Create room"

ROOM_NAME="test-client-flow-$(openssl rand -hex 4)"
CREATE_RESP=$(ssh_vps "curl -s -X POST '${CONDUIT_BASE}/_matrix/client/v3/createRoom' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer ${TOKEN}' \
  -d '{\"name\":\"${ROOM_NAME}\",\"visibility\":\"private\"}'" 2>&1) || true

ROOM_ID=$(echo "$CREATE_RESP" | jq -r '.room_id // empty')

if [[ -n "$ROOM_ID" ]]; then
  log_pass "Room created: $ROOM_ID"
else
  log_fail "Create room failed — response: $(echo "$CREATE_RESP" | head -c 300)"
fi

# ── Test 3: Send message ──────────────────────────────────────────────────────
echo ""
log_info "Test 3/5: Send message"

TXN_ID=$(openssl rand -hex 8)
# Escape the room ID for URL path — Matrix room IDs contain ! and : which are safe in path
SEND_RESP=$(ssh_vps "curl -s -X PUT '${CONDUIT_BASE}/_matrix/client/v3/rooms/${ROOM_ID}/send/m.room.message/${TXN_ID}' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer ${TOKEN}' \
  -d '{\"msgtype\":\"m.text\",\"body\":\"${TEST_MESSAGE}\"}'" 2>&1) || true

EVENT_ID=$(echo "$SEND_RESP" | jq -r '.event_id // empty')

if [[ -n "$EVENT_ID" ]]; then
  log_pass "Message sent (event: $EVENT_ID)"
else
  log_fail "Send message failed — response: $(echo "$SEND_RESP" | head -c 300)"
fi

# ── Test 4: Sync and verify message ───────────────────────────────────────────
echo ""
log_info "Test 4/5: Sync and verify message appears"

FOUND=false
for attempt in 1 2 3 4 5 6; do
  sleep 5
  SYNC_RESP=$(ssh_vps "curl -s '${CONDUIT_BASE}/_matrix/client/v3/sync?timeout=5000' \
    -H 'Authorization: Bearer ${TOKEN}'" 2>&1) || true

  # Check if sync returned valid structure
  if ! echo "$SYNC_RESP" | jq -e '.rooms' >/dev/null 2>&1; then
    log_info "  Attempt $attempt: sync returned no rooms yet, retrying..."
    continue
  fi

  # Search for our test message in the sync response
  if echo "$SYNC_RESP" | jq -r --arg msg "$TEST_MESSAGE" '
    [.rooms.join // {} | to_entries[] |
      .value.timeline.events // [] | .[] |
      select(.content.body == $msg or (.content.body // "" | test($msg)))
    ] | length' 2>/dev/null | grep -q '[1-9]'; then
    FOUND=true
    break
  fi

  log_info "  Attempt $attempt: message not yet visible, retrying..."
done

if $FOUND; then
  log_pass "Message '$TEST_MESSAGE' found via /sync"
else
  log_fail "Message '$TEST_MESSAGE' NOT found after 30s of polling"
fi

# ── Test 5: Cleanup — leave room ──────────────────────────────────────────────
echo ""
log_info "Test 5/5: Cleanup — leave room"

LEAVE_RESP=$(ssh_vps "curl -s -X POST '${CONDUIT_BASE}/_matrix/client/v3/rooms/${ROOM_ID}/leave' \
  -H 'Authorization: Bearer ${TOKEN}'" 2>&1) || true

# A successful leave returns {} (empty object)
LEAVE_EMPTY=$(echo "$LEAVE_RESP" | jq -r 'if . == {} then "ok" else .error // "unknown" end' 2>/dev/null || echo "parse_error")

if [[ "$LEAVE_EMPTY" == "ok" ]]; then
  log_pass "Left room $ROOM_ID"
else
  # Conduit may also return 200 with empty body on leave
  if echo "$LEAVE_RESP" | jq -e 'keys | length == 0' >/dev/null 2>&1; then
    log_pass "Left room $ROOM_ID"
  else
    log_fail "Leave room failed — response: $(echo "$LEAVE_RESP" | head -c 300)"
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " Matrix Client Flow: ${FULL_SYSTEM_PASSED}/${TOTAL_TESTS} PASS"
echo "========================================="

if [[ $FULL_SYSTEM_FAILED -gt 0 ]]; then
  exit 1
fi
