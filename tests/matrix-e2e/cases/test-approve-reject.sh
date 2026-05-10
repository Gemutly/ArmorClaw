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

echo "=== test-approve-reject: /approve and /reject commands ==="

# 1. Login
TOKEN=$(matrix_login "$TEST_USER" "$TEST_PASSWORD") || { echo -e "${RED}FAIL: login${NC}"; exit 1; }
echo -e "${GREEN}Logged in as $TEST_USER${NC}"

# 2. Create room
ROOM_ID=$(matrix_create_room "$TOKEN" "approve-reject-test-$$") || { echo -e "${RED}FAIL: create_room${NC}"; exit 1; }
echo -e "${GREEN}Room: $ROOM_ID${NC}"

# 3. Invite bot
matrix_invite "$TOKEN" "$ROOM_ID" "$BRIDGE_BOT_USER" || { echo -e "${RED}FAIL: invite bot${NC}"; exit 1; }

# 4. Bot joins
BOT_TOKEN=$(matrix_login "test-bot" "test-bot-password") || { echo -e "${RED}FAIL: bot login${NC}"; exit 1; }
matrix_join "$BOT_TOKEN" "$ROOM_ID" || { echo -e "${RED}FAIL: bot join${NC}"; exit 1; }

# --- Test /approve ---
echo -e "${YELLOW}--- /approve ---${NC}"

# 5a. Send /approve
EVENT_ID=$(matrix_send "$TOKEN" "$ROOM_ID" "/approve test-id-123") || { echo -e "${RED}FAIL: send /approve${NC}"; exit 1; }
echo -e "${GREEN}Sent /approve test-id-123 (event: $EVENT_ID)${NC}"

# 6a. Poll for m.notice confirming approval
NOTICE=$(matrix_poll_notice "$TOKEN" "$ROOM_ID" "approv" "$POLL_TIMEOUT") || {
  echo -e "${RED}FAIL: no m.notice for /approve${NC}"
  exit 1
}
assert_notice "[$NOTICE]" "approv" || { echo -e "${RED}FAIL: /approve assertion${NC}"; exit 1; }
echo -e "${GREEN}/approve response OK${NC}"

# --- Test /reject ---
echo -e "${YELLOW}--- /reject ---${NC}"

# 5b. Send /reject
EVENT_ID=$(matrix_send "$TOKEN" "$ROOM_ID" "/reject test-id-456") || { echo -e "${RED}FAIL: send /reject${NC}"; exit 1; }
echo -e "${GREEN}Sent /reject test-id-456 (event: $EVENT_ID)${NC}"

# 6b. Poll for m.notice confirming rejection
NOTICE=$(matrix_poll_notice "$TOKEN" "$ROOM_ID" "reject" "$POLL_TIMEOUT") || {
  echo -e "${RED}FAIL: no m.notice for /reject${NC}"
  exit 1
}
assert_notice "[$NOTICE]" "reject" || { echo -e "${RED}FAIL: /reject assertion${NC}"; exit 1; }
echo -e "${GREEN}/reject response OK${NC}"

echo -e "${GREEN}PASS: /approve /reject E2E test${NC}"
exit 0
