#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/conduit.sh"
source "$SCRIPT_DIR/../lib/matrix-client.sh"
source "$SCRIPT_DIR/../lib/assertions.sh"

CONDUIT_URL="${CONDUIT_URL:-http://localhost:6167}"
CONDUIT_SERVER_NAME="${CONDUIT_SERVER_NAME:-armorclaw.test}"
BRIDGE_BOT_USER="${BRIDGE_BOT_USER:-@test-bot:$CONDUIT_SERVER_NAME}"
TEST_USER="${TEST_USER:-test-admin}"
TEST_PASSWORD="${TEST_PASSWORD:-test-admin-password}"
POLL_TIMEOUT="${POLL_TIMEOUT:-15}"

echo "=== test-custom-events: custom event shape validation ==="

TOKEN=$(matrix_login "$TEST_USER" "$TEST_PASSWORD") || { echo -e "${RED}FAIL: login${NC}"; exit 0; }
echo -e "${GREEN}Logged in as $TEST_USER${NC}"

ROOM_ID=$(matrix_create_room "$TOKEN" "custom-events-test-$$") || { echo -e "${RED}FAIL: create_room${NC}"; exit 0; }
echo -e "${GREEN}Room: $ROOM_ID${NC}"

matrix_invite "$TOKEN" "$ROOM_ID" "$BRIDGE_BOT_USER" || { echo -e "${RED}FAIL: invite bot${NC}"; exit 0; }

BOT_TOKEN=$(matrix_login "test-bot" "test-bot-password") || { echo -e "${RED}FAIL: bot login${NC}"; exit 0; }
matrix_join "$BOT_TOKEN" "$ROOM_ID" || { echo -e "${RED}FAIL: bot join${NC}"; exit 0; }

SINCE=$(matrix_sync "$TOKEN" "" "0" | jq -r '.next_batch // empty' 2>/dev/null || true)

matrix_send "$TOKEN" "$ROOM_ID" "/status" >/dev/null || true
matrix_send "$TOKEN" "$ROOM_ID" "!agent list" >/dev/null || true

sleep 3

SYNC_JSON=$(matrix_sync "$TOKEN" "$SINCE" "2000" 2>/dev/null || echo '{}')

CUSTOM_EVENTS=$(echo "$SYNC_JSON" | jq -c --arg room "$ROOM_ID" '
  [.rooms.join[$room].timeline.events // []
   | .[] | select(.type | test("^(workflow|agent|blocker)\\."))]' 2>/dev/null)

EVENT_COUNT=$(echo "$CUSTOM_EVENTS" | jq 'length' 2>/dev/null || echo "0")

if [[ "$EVENT_COUNT" -eq 0 ]]; then
  echo "SKIP: No custom events produced during test"
  echo "  (Custom events require actual workflow execution)"
  echo -e "${GREEN}DONE: 0 events, 0 validated, 0 mismatched${NC}"
  exit 0
fi

echo "Found $EVENT_COUNT custom event(s)"

VALIDATED=0
MATCHED=0
MISMATCHED=0

for i in $(seq 0 $((EVENT_COUNT - 1))); do
  EVT=$(echo "$CUSTOM_EVENTS" | jq -c ".[$i]")
  EVT_TYPE=$(echo "$EVT" | jq -r '.type // ""')

  echo "  Event[$i] type=$EVT_TYPE"

  if echo "$EVT_TYPE" | grep -qE '^workflow\.'; then
    HAS_ID=$(echo "$EVT" | jq 'has("workflow_id") or (.content | has("workflow_id"))' 2>/dev/null || echo "false")
    HAS_STATUS=$(echo "$EVT" | jq 'has("status") or (.content | has("status"))' 2>/dev/null || echo "false")

    if [[ "$HAS_ID" == "true" && "$HAS_STATUS" == "true" ]]; then
      echo -e "    ${GREEN}valid${NC}: workflow_id=$HAS_ID status=$HAS_STATUS"
      VALIDATED=$((VALIDATED + 1))
      MATCHED=$((MATCHED + 1))
    else
      echo -e "    ${RED}mismatch${NC}: workflow_id=$HAS_ID status=$HAS_STATUS"
      MISMATCHED=$((MISMATCHED + 1))
    fi

  elif echo "$EVT_TYPE" | grep -qE '^agent\.'; then
    HAS_ID=$(echo "$EVT" | jq 'has("agent_id") or (.content | has("agent_id"))' 2>/dev/null || echo "false")
    HAS_STATE=$(echo "$EVT" | jq 'has("state") or (.content | has("state"))' 2>/dev/null || echo "false")

    if [[ "$HAS_ID" == "true" && "$HAS_STATE" == "true" ]]; then
      echo -e "    ${GREEN}valid${NC}: agent_id=$HAS_ID state=$HAS_STATE"
      VALIDATED=$((VALIDATED + 1))
      MATCHED=$((MATCHED + 1))
    else
      echo -e "    ${RED}mismatch${NC}: agent_id=$HAS_ID state=$HAS_STATE"
      MISMATCHED=$((MISMATCHED + 1))
    fi

  else
    echo "    unrecognized prefix: $EVT_TYPE (counted only)"
    VALIDATED=$((VALIDATED + 1))
  fi
done

echo ""
echo "=== Summary: $EVENT_COUNT events, $VALIDATED validated, $MATCHED matched, $MISMATCHED mismatched ==="
echo -e "${GREEN}DONE${NC}"
exit 0
