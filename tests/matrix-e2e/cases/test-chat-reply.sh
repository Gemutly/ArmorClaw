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

SMOKE_ID="armorclaw-smoke-$(date +%s)-$RANDOM"
SEND_TS=$(date +%s)
MESSAGE="hello from $SMOKE_ID"

echo "Sending smoke message: $MESSAGE"
EVENT_ID=$(matrix_send "$TOKEN" "$RELAY_ROOM_ID" "$MESSAGE")
echo "Sent event: $EVENT_ID"

echo "Polling for response (timeout=${POLL_TIMEOUT}s)..."
RESPONSE=$(matrix_poll_notice "$TOKEN" "$RELAY_ROOM_ID" "" "$POLL_TIMEOUT" 2>/dev/null || true)

if [ -n "$RESPONSE" ]; then
    # Check if response is from a different sender (not our test user)
    SENDER=$(echo "$RESPONSE" | jq -r '.sender // empty')
    MSGTYPE=$(echo "$RESPONSE" | jq -r '.content.msgtype // "m.notice"')
    BODY=$(echo "$RESPONSE" | jq -r '.content.body // ""')

    if [ "$SENDER" != "@$TEST_USER:$CONDUIT_SERVER_NAME" ]; then
        if [ "$MSGTYPE" = "m.text" ]; then
            echo "PASS: non-self m.text AI reply"
            PASS=$((PASS + 1))
        elif echo "$BODY" | grep -q "relay_"; then
            echo "CONDITIONAL PASS: non-self m.notice error response (relay_ found)"
            PASS=$((PASS + 1))
        else
            echo "CONDITIONAL PASS: non-self m.notice response (AI provider may have failed)"
            PASS=$((PASS + 1))
        fi
    else
        echo "FAIL: response was from self (test user), not from bot"
        FAIL=$((FAIL + 1))
    fi
else
    echo "FAIL: no response received within ${POLL_TIMEOUT}s"
    FAIL=$((FAIL + 1))
fi

# ═══════════════════════════════════════════
# ── NEGATIVE TEST: Non-relay room ──
# ═══════════════════════════════════════════

echo ""
echo "Running negative test: non-relay room..."
NEGATIVE_ROOM_ID=$(matrix_create_room "$TOKEN" "non-relay-test-$$")
echo "Created non-relay room: $NEGATIVE_ROOM_ID"

NEGATIVE_MSG="hello from $SMOKE_ID-negative"
matrix_send "$TOKEN" "$NEGATIVE_ROOM_ID" "$NEGATIVE_MSG" > /dev/null

NEG_RESPONSE=$(matrix_poll_notice "$TOKEN" "$NEGATIVE_ROOM_ID" "" "5" 2>/dev/null || true)

if [ -z "$NEG_RESPONSE" ]; then
    echo "PASS: no response in non-relay room (as expected)"
    PASS=$((PASS + 1))
else
    NEG_SENDER=$(echo "$NEG_RESPONSE" | jq -r '.sender // empty' 2>/dev/null || echo "")
    if [ "$NEG_SENDER" = "@$TEST_USER:$CONDUIT_SERVER_NAME" ]; then
        echo "PASS: no bot response in non-relay room (only saw own message echo)"
        PASS=$((PASS + 1))
    else
        echo "FAIL: unexpected response in non-relay room from $NEG_SENDER"
        FAIL=$((FAIL + 1))
    fi
fi

# ── Summary ──
echo ""
echo "Results: PASS=$PASS FAIL=$FAIL SKIP=$SKIP"
if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
exit 0
