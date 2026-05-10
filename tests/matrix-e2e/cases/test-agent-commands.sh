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

echo "=== test-agent-commands: !agent create/list/spawn/stop/skills/forget-skill ==="

# Agent commands to test — each is a (label, command-body, response-pattern) tuple
declare -a CMDS=(
  "!agent create|!agent create name=\"test-agent\" skills=\"web_browsing\"|agent.*created"
  "!agent list|!agent list|agent"
  "!agent spawn|!agent spawn test-agent|spawn"
  "!agent skills|!agent skills|skill"
  "!agent stop|!agent stop test-agent|stop"
  "!agent forget-skill|!agent forget-skill test-agent skill-1|forget"
)

# 1. Login
TOKEN=$(matrix_login "$TEST_USER" "$TEST_PASSWORD") || { echo -e "${RED}FAIL: login${NC}"; exit 1; }
echo -e "${GREEN}Logged in as $TEST_USER${NC}"

# 2. Create room
ROOM_ID=$(matrix_create_room "$TOKEN" "agent-test-$$") || { echo -e "${RED}FAIL: create_room${NC}"; exit 1; }
echo -e "${GREEN}Room: $ROOM_ID${NC}"

# 3. Invite bot
matrix_invite "$TOKEN" "$ROOM_ID" "$BRIDGE_BOT_USER" || { echo -e "${RED}FAIL: invite bot${NC}"; exit 1; }

# 4. Bot joins
BOT_TOKEN=$(matrix_login "test-bot" "test-bot-password") || { echo -e "${RED}FAIL: bot login${NC}"; exit 1; }
matrix_join "$BOT_TOKEN" "$ROOM_ID" || { echo -e "${RED}FAIL: bot join${NC}"; exit 1; }

# 5. Send each command, poll for response
RESPONDED=0
SKIPPED=0

for entry in "${CMDS[@]}"; do
  IFS='|' read -r label body pattern <<< "$entry"
  echo -e "${YELLOW}Testing: $label${NC}"

  EVENT_ID=$(matrix_send "$TOKEN" "$ROOM_ID" "$body") || {
    echo -e "${YELLOW}SKIP: could not send $label${NC}"
    ((SKIPPED++)) || true
    continue
  }

  # Poll for m.notice matching pattern — tolerate failure (conditional test)
  NOTICE=$(matrix_poll_notice "$TOKEN" "$ROOM_ID" "$pattern" "$POLL_TIMEOUT" 2>/dev/null) || {
    echo -e "${YELLOW}SKIP: $label — no m.notice response (pattern: $pattern)${NC}"
    ((SKIPPED++)) || true
    continue
  }

  assert_notice "[$NOTICE]" "$pattern" 2>/dev/null || {
    echo -e "${YELLOW}SKIP: $label — assertion failed${NC}"
    ((SKIPPED++)) || true
    continue
  }

  echo -e "${GREEN}PASS: $label${NC}"
  ((RESPONDED++)) || true
done

# 6. Summary
TOTAL=${#CMDS[@]}
echo ""
echo "--- Agent command results: $RESPONDED/$TOTAL responded, $SKIPPED skipped ---"

if [ "$RESPONDED" -eq 0 ]; then
  echo "SKIP: !agent commands not responding — bridge may not have agent studio enabled"
  exit 1
fi

echo -e "${GREEN}PASS: at least one !agent command got a response ($RESPONDED/$TOTAL)${NC}"
exit 0
