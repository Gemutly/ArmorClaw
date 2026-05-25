#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/conduit.sh"
source "$SCRIPT_DIR/../lib/matrix-client.sh"
source "$SCRIPT_DIR/../lib/assertions.sh"

# ── Environment defaults ──
CONDUIT_URL="${CONDUIT_URL:-http://localhost:${CONDUIT_PORT:-6167}}"
CONDUIT_SERVER_NAME="${CONDUIT_SERVER_NAME:-armorclaw.test}"
TEST_USER="${TEST_USER:-test-admin}"
TEST_PASSWORD="${TEST_PASSWORD:-test-admin-password}"
BRIDGE_BOT_USER="${BRIDGE_BOT_USER:-@test-bot:$CONDUIT_SERVER_NAME}"
RELAY_ROOM_ID="${ARMORCLAW_CHAT_RELAY_ROOM_IDS:-}"  # Pre-configured room (VPS smoke)
POLL_TIMEOUT="${POLL_TIMEOUT:-30}"

# ── Skip condition ──
# Skip if no AI API key is configured (the relay needs an AI provider to respond)
if [ -z "${OPEN_AI_KEY:-}" ] && [ -z "${OPENROUTER_API_KEY:-}" ] && [ -z "${ZAI_API_KEY:-}" ] && [ -z "${ANTHROPIC_API_KEY:-}" ]; then
    echo "SKIP: no AI API key configured (set OPEN_AI_KEY, OPENROUTER_API_KEY, ZAI_API_KEY, or ANTHROPIC_API_KEY)"
    exit 0
fi

# ── Counters ──
PASS=0
FAIL=0
SKIP=0

# ── Login ──
echo "Logging in as $TEST_USER..."
TOKEN=$(matrix_login "$TEST_USER" "$TEST_PASSWORD")
echo "Logged in. Token: ${TOKEN:0:8}..."

# ── Use pre-created relay room ──
# The relay room is pre-configured via ARMORCLAW_CHAT_RELAY_ROOM_IDS before Bridge starts.
# For VPS smoke tests, this is set to a known room like "!roomid:5.183.11.149".
if [ -z "$RELAY_ROOM_ID" ]; then
    echo "SKIP: ARMORCLAW_CHAT_RELAY_ROOM_IDS not set (no relay room configured)"
    exit 0
fi
echo "Using relay room: $RELAY_ROOM_ID"

# ═══════════════════════════════════════════
# ── MAIN TEST: Unique smoke message ──
# ═══════════════════════════════════════════

# TODO (Task 6): Generate unique smoke ID
# SMOKE_ID="armorclaw-smoke-$(date +%s)-$RANDOM"
# SEND_TS=$(date +%s)
# MESSAGE="hello from $SMOKE_ID"

# TODO (Task 6): Send message
# EVENT_ID=$(matrix_send "$TOKEN" "$RELAY_ROOM_ID" "$MESSAGE")

# TODO (Task 6): Poll for non-self m.text response
# RESPONSE=$(matrix_poll_notice "$TOKEN" "$RELAY_ROOM_ID" "" "$POLL_TIMEOUT")

# TODO (Task 6): Evaluate result
# - If non-self m.text response → PASS: non-self m.text AI reply
# - If non-self m.notice containing "relay_" → CONDITIONAL PASS: error response
# - If no response → FAIL: no reply

# ═══════════════════════════════════════════
# ── NEGATIVE TEST: Non-relay room ──
# ═══════════════════════════════════════════

# TODO (Task 6): Create a room NOT in the allowlist
# NEGATIVE_ROOM_ID=$(matrix_create_room "$TOKEN" "non-relay-test")
# Send "hello" → poll for 5s → assert NO response

# ── Summary ──
echo ""
echo "Results: PASS=$PASS FAIL=$FAIL SKIP=$SKIP"
if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
exit 0
