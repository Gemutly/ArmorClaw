#!/usr/bin/env bash
# vps-matrix-cli-test.sh — Two-mode VPS Matrix CLI validation script
#
# Validates that Matrix CLI commands work correctly against a live ArmorClaw VPS
# deployment. Self-contained — does not source any project files.
#
# Modes (via MODE env var):
#   smoke         — Safe for live: SSH, service health, login, read-only commands
#   fixture-admin — Isolated state-changing tests: /claim_admin, /verify, /approve, /reject
#                   FAILS CLOSED if fixture env vars are not provided.
#
# Usage:
#   MODE=smoke VPS_IP=1.2.3.4 MATRIX_BASE_URL=https://... ./scripts/vps-matrix-cli-test.sh
#   MODE=fixture-admin FIXTURE_ROOM_ID='!x:y' FIXTURE_TEST_USER='@test:y' ./scripts/vps-matrix-cli-test.sh

set -euo pipefail

# ── Configuration ──────────────────────────────────────────────────────────────

MODE="${MODE:-smoke}"
VPS_IP="${VPS_IP:-}"
SSH_KEY_PATH="${SSH_KEY_PATH:-}"
SSH_USER="${SSH_USER:-root}"
MATRIX_BASE_URL="${MATRIX_BASE_URL:-}"
MATRIX_USER="${MATRIX_USER:-}"
MATRIX_PASSWORD="${MATRIX_PASSWORD:-}"
MATRIX_ROOM_ID="${MATRIX_ROOM_ID:-}"
EVIDENCE_DIR=".sisyphus/evidence"

# Fixture-mode specific (only used in fixture-admin mode)
FIXTURE_ROOM_ID="${FIXTURE_ROOM_ID:-}"
FIXTURE_APPROVAL_TARGET="${FIXTURE_APPROVAL_TARGET:-}"
FIXTURE_VERIFY_TARGET="${FIXTURE_VERIFY_TARGET:-}"
FIXTURE_TEST_USER="${FIXTURE_TEST_USER:-}"

# Polling defaults
POLL_TIMEOUT="${POLL_TIMEOUT:-20}"
POLL_INTERVAL="${POLL_INTERVAL:-2}"

# Counters
PASS=0
FAIL=0
SKIP=0

# Runtime state (set during login)
ACCESS_TOKEN=""
SINCE_TOKEN=""

# ── Helpers ────────────────────────────────────────────────────────────────────

log_pass() {
  PASS=$((PASS + 1))
  echo "PASS: $1"
}

log_fail() {
  FAIL=$((FAIL + 1))
  echo "FAIL: $1"
}

log_skip() {
  SKIP=$((SKIP + 1))
  echo "SKIP: $1"
}

redact() {
  # Strip access_token values from strings to prevent secret leakage in logs
  echo "$1" | sed -E 's/(access_token=)[^&" ]*/\1REDACTED/g' | sed -E 's/("access_token"\s*:\s*")[^"]*/\1REDACTED/g'
}

save_evidence() {
  local name="$1"
  local data="$2"
  mkdir -p "$EVIDENCE_DIR"
  echo "$data" > "$EVIDENCE_DIR/task-4-${name}.json"
}

ssh_cmd() {
  # Execute a command on the VPS via SSH. Args: command...
  ssh -i "$SSH_KEY_PATH" \
    -o ConnectTimeout=5 \
    -o BatchMode=yes \
    -o StrictHostKeyChecking=accept-new \
    "${SSH_USER}@${VPS_IP}" \
    "$@"
}

# ── Matrix API Functions (self-contained, replicates matrix-client.sh patterns) ─

matrix_login() {
  # Login to Matrix via m.login.password. Sets ACCESS_TOKEN on success.
  local resp
  resp=$(curl -sf -X POST "${MATRIX_BASE_URL}/_matrix/client/v3/login" \
    -H "Content-Type: application/json" \
    -d "{
      \"type\": \"m.login.password\",
      \"identifier\": { \"type\": \"m.id.user\", \"user\": \"${MATRIX_USER}\" },
      \"password\": \"${MATRIX_PASSWORD}\"
    }" 2>/dev/null) || {
    echo "ERROR: Matrix login request failed" >&2
    return 1
  }

  ACCESS_TOKEN=$(echo "$resp" | jq -r '.access_token // empty' 2>/dev/null)
  if [[ -z "$ACCESS_TOKEN" ]]; then
    local err
    err=$(echo "$resp" | jq -r '.error // "unknown"' 2>/dev/null)
    echo "ERROR: Matrix login failed: $err" >&2
    return 1
  fi

  # Perform initial sync to get a since token for incremental polling
  local sync_resp
  sync_resp=$(curl -sf "${MATRIX_BASE_URL}/_matrix/client/v3/sync?access_token=${ACCESS_TOKEN}&timeout=0" 2>/dev/null) || true
  SINCE_TOKEN=$(echo "$sync_resp" | jq -r '.next_batch // empty' 2>/dev/null)

  echo "OK: logged in as ${MATRIX_USER}"
  return 0
}

matrix_send() {
  # Send a message to a room. Args: room_id body [msgtype]
  # Returns event_id on stdout.
  local room_id="$1"
  local body="$2"
  local msgtype="${3:-m.text}"

  local txn_id="txn-$$-$(date +%s%N)"
  local escaped_body
  escaped_body=$(echo "$body" | jq -Rs . 2>/dev/null || echo "\"$body\"")

  local resp
  resp=$(curl -sf -X PUT "${MATRIX_BASE_URL}/_matrix/client/v3/rooms/${room_id}/send/m.room.message/${txn_id}?access_token=${ACCESS_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"msgtype\": \"${msgtype}\", \"body\": ${escaped_body}}" 2>/dev/null) || {
    echo "ERROR: Matrix send failed for: $body" >&2
    return 1
  }

  local event_id
  event_id=$(echo "$resp" | jq -r '.event_id // empty' 2>/dev/null)
  echo "${event_id}"
  return 0
}

matrix_poll_notice() {
  # Poll sync for m.notice events in a room. Args: room_id [expected_substring]
  # Returns matching event JSON on stdout, empty on timeout.
  local room_id="$1"
  local expected="${2:-}"
  local elapsed=0

  while [[ $elapsed -lt $POLL_TIMEOUT ]]; do
    local endpoint="${MATRIX_BASE_URL}/_matrix/client/v3/sync?access_token=${ACCESS_TOKEN}&timeout=1000"
    if [[ -n "$SINCE_TOKEN" ]]; then
      endpoint="${endpoint}&since=${SINCE_TOKEN}"
    fi

    local resp
    resp=$(curl -sf "$endpoint" 2>/dev/null) || true

    if [[ -n "$resp" ]]; then
      SINCE_TOKEN=$(echo "$resp" | jq -r '.next_batch // empty' 2>/dev/null)

      # Extract timeline events for the target room
      local events
      events=$(echo "$resp" | jq -c --arg room "$room_id" \
        '.rooms.join[$room].timeline.events // []' 2>/dev/null)

      local match
      if [[ -n "$expected" ]]; then
        match=$(echo "$events" | jq -c --arg sub "$expected" \
          '[.[] | select(.type == "m.notice" and ((.content.body // "") | test($sub; "i")))][0]' 2>/dev/null)
      else
        match=$(echo "$events" | jq -c \
          '[.[] | select(.type == "m.notice")][0]' 2>/dev/null)
      fi

      if [[ -n "$match" && "$match" != "null" ]]; then
        echo "$match"
        return 0
      fi
    fi

    sleep "$POLL_INTERVAL"
    elapsed=$((elapsed + POLL_INTERVAL))
  done

  echo "ERROR: poll_notice timed out after ${POLL_TIMEOUT}s" >&2
  return 1
}

# ── Smoke Mode Tests ───────────────────────────────────────────────────────────

test_ssh_connectivity() {
  echo "--- test_ssh_connectivity ---"
  if [[ -z "$VPS_IP" || -z "$SSH_KEY_PATH" ]]; then
    log_skip "ssh_connectivity (VPS_IP or SSH_KEY_PATH not set)"
    return 0
  fi

  local result
  result=$(ssh_cmd "echo ok" 2>&1) || {
    log_fail "ssh_connectivity: SSH failed — $(redact "$result")"
    return 0
  }

  if [[ "$result" == "ok" ]]; then
    log_pass "ssh_connectivity"
  else
    log_fail "ssh_connectivity: unexpected output — $result"
  fi
}

test_service_health() {
  echo "--- test_service_health ---"
  if [[ -z "$VPS_IP" || -z "$SSH_KEY_PATH" ]]; then
    log_skip "service_health (VPS_IP or SSH_KEY_PATH not set)"
    return 0
  fi

  local result
  result=$(ssh_cmd "docker compose ps --format '{{.Name}} {{.Status}}' 2>/dev/null || docker ps --format '{{.Names}} {{.Status}}'" 2>&1) || {
    log_fail "service_health: docker command failed — $(redact "$result")"
    return 0
  }

  save_evidence "service-health" "$result"

  # Check that bridge and matrix/conduit containers are running
  local bridge_up=0
  local matrix_up=0

  if echo "$result" | grep -qi "bridge.*Up\|bridge.*running"; then
    bridge_up=1
  fi
  if echo "$result" | grep -qi "conduit\|matrix.*Up\|matrix.*running"; then
    matrix_up=1
  fi

  if [[ $bridge_up -eq 1 && $matrix_up -eq 1 ]]; then
    log_pass "service_health (bridge + matrix running)"
  elif [[ $bridge_up -eq 1 ]]; then
    log_fail "service_health: bridge running but matrix/conduit not detected"
  elif [[ $matrix_up -eq 1 ]]; then
    log_fail "service_health: matrix/conduit running but bridge not detected"
  else
    log_fail "service_health: neither bridge nor matrix/conduit detected as running"
  fi
}

test_matrix_login() {
  echo "--- test_matrix_login ---"
  if [[ -z "$MATRIX_BASE_URL" || -z "$MATRIX_USER" || -z "$MATRIX_PASSWORD" ]]; then
    log_skip "matrix_login (MATRIX_BASE_URL, MATRIX_USER, or MATRIX_PASSWORD not set)"
    return 0
  fi

  local result
  result=$(matrix_login 2>&1) || {
    log_fail "matrix_login: $(redact "$result")"
    return 0
  }

  if [[ -n "$ACCESS_TOKEN" ]]; then
    log_pass "matrix_login (${MATRIX_USER})"
  else
    log_fail "matrix_login: no access token obtained"
  fi
}

test_status_command() {
  echo "--- test_status_command ---"
  if [[ -z "$ACCESS_TOKEN" || -z "$MATRIX_ROOM_ID" ]]; then
    log_skip "status_command (not logged in or MATRIX_ROOM_ID not set)"
    return 0
  fi

  # Send /status command
  local event_id
  event_id=$(matrix_send "$MATRIX_ROOM_ID" "/status" 2>/dev/null) || {
    log_fail "status_command: failed to send /status"
    return 0
  }

  save_evidence "status-send" "{\"event_id\": \"${event_id}\"}"

  # Poll for m.notice response containing "status" or "bridge"
  local match
  match=$(matrix_poll_notice "$MATRIX_ROOM_ID" "status|bridge" 2>/dev/null) || {
    log_fail "status_command: no m.notice response received"
    return 0
  }

  save_evidence "status-response" "$match"

  local body
  body=$(echo "$match" | jq -r '.content.body // empty' 2>/dev/null)
  if [[ -n "$body" ]]; then
    log_pass "status_command (m.notice received, body length: ${#body})"
  else
    log_fail "status_command: m.notice received but body is empty"
  fi
}

test_help_command() {
  echo "--- test_help_command ---"
  if [[ -z "$ACCESS_TOKEN" || -z "$MATRIX_ROOM_ID" ]]; then
    log_skip "help_command (not logged in or MATRIX_ROOM_ID not set)"
    return 0
  fi

  local event_id
  event_id=$(matrix_send "$MATRIX_ROOM_ID" "/help" 2>/dev/null) || {
    log_fail "help_command: failed to send /help"
    return 0
  }

  save_evidence "help-send" "{\"event_id\": \"${event_id}\"}"

  local match
  match=$(matrix_poll_notice "$MATRIX_ROOM_ID" "help" 2>/dev/null) || {
    log_fail "help_command: no m.notice response received"
    return 0
  }

  save_evidence "help-response" "$match"

  local msgtype
  msgtype=$(echo "$match" | jq -r '.content.msgtype // empty' 2>/dev/null)
  if [[ "$msgtype" == "m.notice" ]]; then
    log_pass "help_command (m.notice confirmed)"
  else
    log_fail "help_command: response msgtype is '${msgtype}', expected m.notice"
  fi
}

test_studio_command() {
  echo "--- test_studio_command (!agent list) ---"
  if [[ -z "$ACCESS_TOKEN" || -z "$MATRIX_ROOM_ID" ]]; then
    log_skip "studio_command (not logged in or MATRIX_ROOM_ID not set)"
    return 0
  fi

  local event_id
  event_id=$(matrix_send "$MATRIX_ROOM_ID" "!agent list" 2>/dev/null) || {
    log_fail "studio_command: failed to send !agent list"
    return 0
  }

  save_evidence "studio-send" "{\"event_id\": \"${event_id}\"}"

  # Poll for any m.notice (don't filter by content — just verify routing)
  local match
  match=$(matrix_poll_notice "$MATRIX_ROOM_ID" "" 2>/dev/null) || {
    log_fail "studio_command: no m.notice response received"
    return 0
  }

  save_evidence "studio-response" "$match"

  local msgtype
  msgtype=$(echo "$match" | jq -r '.content.msgtype // empty' 2>/dev/null)
  if [[ "$msgtype" == "m.notice" ]]; then
    log_pass "studio_command (m.notice received from !agent list)"
  else
    log_fail "studio_command: response msgtype is '${msgtype}', expected m.notice"
  fi
}

test_secretary_command() {
  echo "--- test_secretary_command (!secretary status) ---"
  if [[ -z "$ACCESS_TOKEN" || -z "$MATRIX_ROOM_ID" ]]; then
    log_skip "secretary_command (not logged in or MATRIX_ROOM_ID not set)"
    return 0
  fi

  local event_id
  event_id=$(matrix_send "$MATRIX_ROOM_ID" "!secretary status" 2>/dev/null) || {
    log_fail "secretary_command: failed to send !secretary status"
    return 0
  }

  save_evidence "secretary-send" "{\"event_id\": \"${event_id}\"}"

  # Poll for any m.notice (just verify command routes)
  local match
  match=$(matrix_poll_notice "$MATRIX_ROOM_ID" "" 2>/dev/null) || {
    log_fail "secretary_command: no m.notice response received"
    return 0
  }

  save_evidence "secretary-response" "$match"

  local msgtype
  msgtype=$(echo "$match" | jq -r '.content.msgtype // empty' 2>/dev/null)
  if [[ "$msgtype" == "m.notice" ]]; then
    log_pass "secretary_command (m.notice received from !secretary status)"
  else
    log_fail "secretary_command: response msgtype is '${msgtype}', expected m.notice"
  fi
}

# ── Fixture-Admin Mode Tests ──────────────────────────────────────────────────

test_claim_admin() {
  echo "--- test_claim_admin (fixture) ---"
  local room="${FIXTURE_ROOM_ID}"

  local event_id
  event_id=$(matrix_send "$room" "/claim_admin" 2>/dev/null) || {
    log_fail "claim_admin: failed to send /claim_admin"
    return 0
  }

  save_evidence "fixture-claim-admin-send" "{\"event_id\": \"${event_id}\"}"

  local match
  match=$(matrix_poll_notice "$room" "admin|claim" 2>/dev/null) || {
    log_fail "claim_admin: no m.notice response received"
    return 0
  }

  save_evidence "fixture-claim-admin-response" "$match"

  local msgtype
  msgtype=$(echo "$match" | jq -r '.content.msgtype // empty' 2>/dev/null)
  if [[ "$msgtype" == "m.notice" ]]; then
    log_pass "claim_admin (m.notice confirmed)"
  else
    log_fail "claim_admin: response msgtype is '${msgtype}', expected m.notice"
  fi
}

test_verify() {
  echo "--- test_verify (fixture) ---"
  local room="${FIXTURE_ROOM_ID}"
  local target="${FIXTURE_VERIFY_TARGET}"

  local cmd="/verify"
  if [[ -n "$target" ]]; then
    cmd="/verify ${target}"
  fi

  local event_id
  event_id=$(matrix_send "$room" "$cmd" 2>/dev/null) || {
    log_fail "verify: failed to send $cmd"
    return 0
  }

  save_evidence "fixture-verify-send" "{\"event_id\": \"${event_id}\", \"command\": \"${cmd}\"}"

  local match
  match=$(matrix_poll_notice "$room" "verif" 2>/dev/null) || {
    log_fail "verify: no m.notice response received"
    return 0
  }

  save_evidence "fixture-verify-response" "$match"

  local msgtype
  msgtype=$(echo "$match" | jq -r '.content.msgtype // empty' 2>/dev/null)
  if [[ "$msgtype" == "m.notice" ]]; then
    log_pass "verify (m.notice confirmed)"
  else
    log_fail "verify: response msgtype is '${msgtype}', expected m.notice"
  fi
}

test_approve() {
  echo "--- test_approve (fixture) ---"
  local room="${FIXTURE_ROOM_ID}"
  local target="${FIXTURE_APPROVAL_TARGET}"

  local cmd="/approve"
  if [[ -n "$target" ]]; then
    cmd="/approve ${target}"
  fi

  local event_id
  event_id=$(matrix_send "$room" "$cmd" 2>/dev/null) || {
    log_fail "approve: failed to send $cmd"
    return 0
  }

  save_evidence "fixture-approve-send" "{\"event_id\": \"${event_id}\", \"command\": \"${cmd}\"}"

  local match
  match=$(matrix_poll_notice "$room" "approv" 2>/dev/null) || {
    log_fail "approve: no m.notice response received"
    return 0
  }

  save_evidence "fixture-approve-response" "$match"

  local msgtype
  msgtype=$(echo "$match" | jq -r '.content.msgtype // empty' 2>/dev/null)
  if [[ "$msgtype" == "m.notice" ]]; then
    log_pass "approve (m.notice confirmed)"
  else
    log_fail "approve: response msgtype is '${msgtype}', expected m.notice"
  fi
}

test_reject() {
  echo "--- test_reject (fixture) ---"
  local room="${FIXTURE_ROOM_ID}"
  local target="${FIXTURE_APPROVAL_TARGET}"

  local cmd="/reject"
  if [[ -n "$target" ]]; then
    cmd="/reject ${target}"
  fi

  local event_id
  event_id=$(matrix_send "$room" "$cmd" 2>/dev/null) || {
    log_fail "reject: failed to send $cmd"
    return 0
  }

  save_evidence "fixture-reject-send" "{\"event_id\": \"${event_id}\", \"command\": \"${cmd}\"}"

  local match
  match=$(matrix_poll_notice "$room" "reject" 2>/dev/null) || {
    log_fail "reject: no m.notice response received"
    return 0
  }

  save_evidence "fixture-reject-response" "$match"

  local msgtype
  msgtype=$(echo "$match" | jq -r '.content.msgtype // empty' 2>/dev/null)
  if [[ "$msgtype" == "m.notice" ]]; then
    log_pass "reject (m.notice confirmed)"
  else
    log_fail "reject: response msgtype is '${msgtype}', expected m.notice"
  fi
}

# ── Mode Routing ───────────────────────────────────────────────────────────────

run_smoke() {
  echo "========================================="
  echo " MODE: smoke (safe for live deployment)"
  echo "========================================="
  echo ""

  test_ssh_connectivity
  test_service_health
  test_matrix_login
  test_status_command
  test_help_command
  test_studio_command
  test_secretary_command
}

run_fixture_admin() {
  echo "========================================="
  echo " MODE: fixture-admin (isolated state-changing)"
  echo "========================================="
  echo ""

  # Fail closed if fixture vars not set
  if [[ -z "$FIXTURE_ROOM_ID" || -z "$FIXTURE_TEST_USER" ]]; then
    echo "FATAL: fixture-admin mode requires FIXTURE_ROOM_ID and FIXTURE_TEST_USER"
    echo "  FIXTURE_ROOM_ID='${FIXTURE_ROOM_ID}'"
    echo "  FIXTURE_TEST_USER='${FIXTURE_TEST_USER}'"
    exit 1
  fi

  if [[ -z "$MATRIX_BASE_URL" || -z "$MATRIX_USER" || -z "$MATRIX_PASSWORD" ]]; then
    echo "FATAL: fixture-admin mode requires MATRIX_BASE_URL, MATRIX_USER, MATRIX_PASSWORD"
    exit 1
  fi

  # Login first
  test_matrix_login

  if [[ -z "$ACCESS_TOKEN" ]]; then
    echo "FATAL: login failed, cannot proceed with fixture-admin tests"
    exit 1
  fi

  test_claim_admin
  test_verify
  test_approve
  test_reject
}

# ── Main ───────────────────────────────────────────────────────────────────────

mkdir -p "$EVIDENCE_DIR"

case "$MODE" in
  smoke)
    run_smoke
    ;;
  fixture-admin)
    run_fixture_admin
    ;;
  *)
    echo "Usage: MODE=smoke|fixture-admin $0"
    echo ""
    echo "Modes:"
    echo "  smoke         — Safe read-only tests (SSH, health, login, /status, /help, studio, secretary)"
    echo "  fixture-admin — State-changing tests in isolated fixtures (/claim_admin, /verify, /approve, /reject)"
    echo ""
    echo "Required env vars for smoke:"
    echo "  VPS_IP, SSH_KEY_PATH, MATRIX_BASE_URL, MATRIX_USER, MATRIX_PASSWORD, MATRIX_ROOM_ID"
    echo ""
    echo "Additional env vars for fixture-admin:"
    echo "  FIXTURE_ROOM_ID, FIXTURE_TEST_USER, FIXTURE_APPROVAL_TARGET, FIXTURE_VERIFY_TARGET"
    exit 1
    ;;
esac

echo ""
echo "Results: ${PASS} PASS | ${FAIL} FAIL | ${SKIP} SKIP"
echo "Evidence saved to: ${EVIDENCE_DIR}/task-4-*.json"

if [[ $FAIL -eq 0 ]]; then
  exit 0
else
  exit 1
fi
