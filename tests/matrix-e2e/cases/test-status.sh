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

echo "=== test-status: /status command ==="

# 1. Login
TOKEN=$(matrix_login "$TEST_USER" "$TEST_PASSWORD") || { echo -e "${RED}FAIL: login${NC}"; exit 1; }
echo -e "${GREEN}Logged in as $TEST_USER${NC}"

# 2. Create room
ROOM_ID=$(matrix_create_room "$TOKEN" "status-test-$$") || { echo -e "${RED}FAIL: create_room${NC}"; exit 1; }
echo -e "${GREEN}Room: $ROOM_ID${NC}"

# 3. Invite bot
matrix_invite "$TOKEN" "$ROOM_ID" "$BRIDGE_BOT_USER" || { echo -e "${RED}FAIL: invite bot${NC}"; exit 1; }

# 4. Bot joins
BOT_TOKEN=$(matrix_login "test-bot" "test-bot-password") || { echo -e "${RED}FAIL: bot login${NC}"; exit 1; }
matrix_join "$BOT_TOKEN" "$ROOM_ID" || { echo -e "${RED}FAIL: bot join${NC}"; exit 1; }

# 5. Send /status
EVENT_ID=$(matrix_send "$TOKEN" "$ROOM_ID" "/status") || { echo -e "${RED}FAIL: send /status${NC}"; exit 1; }
echo -e "${GREEN}Sent /status (event: $EVENT_ID)${NC}"

# 6. Poll for m.notice with status/bridge/running
NOTICE=$(matrix_poll_notice "$TOKEN" "$ROOM_ID" "status|bridge|running" "$POLL_TIMEOUT") || {
  echo -e "${RED}FAIL: no m.notice response with status content${NC}"
  exit 1
}

# 7. Assert the notice
assert_notice "[$NOTICE]" "status|bridge|running" || exit 1

echo -e "${GREEN}PASS: /status E2E test${NC}"
exit 0
