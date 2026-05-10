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

PASS=0
TOTAL=0

echo "=== test-mnotice-responses: m.notice contract validation ==="

TOKEN=$(matrix_login "$TEST_USER" "$TEST_PASSWORD") || { echo -e "${RED}FAIL: login${NC}"; exit 1; }
echo -e "${GREEN}Logged in as $TEST_USER${NC}"

ROOM_ID=$(matrix_create_room "$TOKEN" "mnotice-test-$$") || { echo -e "${RED}FAIL: create_room${NC}"; exit 1; }
echo -e "${GREEN}Room: $ROOM_ID${NC}"

matrix_invite "$TOKEN" "$ROOM_ID" "$BRIDGE_BOT_USER" || { echo -e "${RED}FAIL: invite bot${NC}"; exit 1; }

BOT_TOKEN=$(matrix_login "test-bot" "test-bot-password") || { echo -e "${RED}FAIL: bot login${NC}"; exit 1; }
matrix_join "$BOT_TOKEN" "$ROOM_ID" || { echo -e "${RED}FAIL: bot join${NC}"; exit 1; }

validate_notice() {
  local label="$1"
  local cmd="$2"
  local pattern="${3:-}"

  echo ""
  echo "--- $label ---"
  TOTAL=$((TOTAL + 1))

  local event_id
  event_id=$(matrix_send "$TOKEN" "$ROOM_ID" "$cmd") || {
    echo -e "${RED}FAIL: send $label${NC}"
    return
  }

  local notice
  notice=$(matrix_poll_notice "$TOKEN" "$ROOM_ID" "" "$POLL_TIMEOUT") || {
    echo -e "${RED}FAIL: no m.notice for $label${NC}"
    return
  }

  local msgtype body
  msgtype=$(echo "$notice" | jq -r '.content.msgtype // ""' 2>/dev/null)
  body=$(echo "$notice" | jq -r '.content.body // ""' 2>/dev/null)

  local msgtype_ok="false" body_ok="false" pattern_ok="false"
  [[ "$msgtype" == "m.notice" ]] && msgtype_ok="true"
  [[ -n "$body" ]] && body_ok="true"

  if [[ -n "$pattern" ]]; then
    echo "$body" | grep -qiE "$pattern" && pattern_ok="true"
    echo "  msgtype=$msgtype valid=$msgtype_ok body_nonempty=$body_ok body_matches_pattern=$pattern_ok"
  else
    echo "  msgtype=$msgtype valid=$msgtype_ok body_nonempty=$body_ok"
  fi

  if [[ "$msgtype_ok" == "true" && "$body_ok" == "true" ]]; then
    if [[ -z "$pattern" || "$pattern_ok" == "true" ]]; then
      PASS=$((PASS + 1))
      echo -e "  ${GREEN}PASS${NC}"
      return
    fi
  fi
  echo -e "  ${RED}FAIL${NC}"
}

validate_notice "/status" "/status" "status|bridge|running"
validate_notice "!agent skills test-agent-1" "!agent skills test-agent-1" ""
validate_notice "!agent forget-skill test-agent-1 ls_xxx_123" "!agent forget-skill test-agent-1 ls_xxx_123" ""

echo ""
echo "=== Summary: $PASS/$TOTAL validated ==="

if [[ "$PASS" -eq "$TOTAL" ]]; then
  echo -e "${GREEN}PASS: all m.notice responses valid${NC}"
  exit 0
else
  echo -e "${RED}FAIL: $((TOTAL - PASS)) response(s) invalid${NC}"
  exit 1
fi
