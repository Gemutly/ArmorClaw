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

echo "=== test-agent-skills: !agent skills command ==="

# 1. Login
TOKEN=$(matrix_login "$TEST_USER" "$TEST_PASSWORD") || { echo -e "${RED}FAIL: login${NC}"; exit 1; }
echo -e "${GREEN}Logged in as $TEST_USER${NC}"

# 2. Create room
ROOM_ID=$(matrix_create_room "$TOKEN" "agent-skills-test-$$") || { echo -e "${RED}FAIL: create_room${NC}"; exit 1; }
echo -e "${GREEN}Room: $ROOM_ID${NC}"

# 3. Invite bot
matrix_invite "$TOKEN" "$ROOM_ID" "$BRIDGE_BOT_USER" || { echo -e "${RED}FAIL: invite bot${NC}"; exit 1; }

# 4. Bot joins
BOT_TOKEN=$(matrix_login "test-bot" "test-bot-password") || { echo -e "${RED}FAIL: bot login${NC}"; exit 1; }
matrix_join "$BOT_TOKEN" "$ROOM_ID" || { echo -e "${RED}FAIL: bot join${NC}"; exit 1; }

# 5. Send !agent skills
EVENT_ID=$(matrix_send "$TOKEN" "$ROOM_ID" "!agent skills test-agent-1") || { echo -e "${RED}FAIL: send !agent skills${NC}"; exit 1; }
echo -e "${GREEN}Sent !agent skills test-agent-1 (event: $EVENT_ID)${NC}"

# 6. Poll for any m.notice (skill list or "no skills" message)
NOTICE=$(matrix_poll_notice "$TOKEN" "$ROOM_ID" "" "$POLL_TIMEOUT") || {
  echo -e "${RED}FAIL: no m.notice response received${NC}"
  exit 1
}

# 7. Assert a notice was received
body=$(echo "$NOTICE" | jq -r '.content.body // empty' 2>/dev/null)
if [[ -n "$body" ]]; then
  echo -e "${GREEN}PASS: !agent skills E2E test — received notice: ${body:0:80}${NC}"
  exit 0
fi

echo -e "${RED}FAIL: m.notice had no body${NC}"
exit 1
