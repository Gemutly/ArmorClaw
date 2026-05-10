#!/usr/bin/env bash
# matrix-client.sh — Matrix Client-Server API helpers for E2E tests
#
# Wraps common Matrix v3 Client-Server endpoints with curl + jq.
# Requires: CONDUIT_URL, curl, jq

# ── matrix_login <username> <password> ─────────────────────────────────────────
# Returns access token on stdout.
matrix_login() {
  local username="$1"
  local password="$2"
  local url="${CONDUIT_URL:-http://localhost:6167}"

  if [[ -z "$username" || -z "$password" ]]; then
    echo -e "${RED}[matrix] login requires username and password${NC}" >&2
    return 1
  fi

  local resp
  resp=$(curl -sf -X POST "$url/_matrix/client/v3/login" \
    -H "Content-Type: application/json" \
    -d "{
      \"type\": \"m.login.password\",
      \"identifier\": { \"type\": \"m.id.user\", \"user\": \"$username\" },
      \"password\": \"$password\"
    }" 2>/dev/null)

  if [[ -z "$resp" ]]; then
    echo -e "${RED}[matrix] login request failed${NC}" >&2
    return 1
  fi

  local token
  token=$(echo "$resp" | jq -r '.access_token // empty' 2>/dev/null)
  if [[ -z "$token" ]]; then
    local err
    err=$(echo "$resp" | jq -r '.error // "unknown"' 2>/dev/null)
    echo -e "${RED}[matrix] login failed: $err${NC}" >&2
    return 1
  fi

  echo "$token"
}

# ── matrix_send <access_token> <room_id> <body> [msgtype] ──────────────────────
# Returns event_id on stdout.
matrix_send() {
  local access_token="$1"
  local room_id="$2"
  local body="$3"
  local msgtype="${4:-m.text}"
  local url="${CONDUIT_URL:-http://localhost:6167}"

  if [[ -z "$access_token" || -z "$room_id" || -z "$body" ]]; then
    echo -e "${RED}[matrix] send requires access_token, room_id, body${NC}" >&2
    return 1
  fi

  local txn_id="txn-$$-$(date +%s%N)"
  local escaped_body
  escaped_body=$(echo "$body" | jq -Rs . 2>/dev/null || echo "\"$body\"")

  local resp
  resp=$(curl -sf -X PUT "$url/_matrix/client/v3/rooms/${room_id}/send/m.room.message/${txn_id}?access_token=${access_token}" \
    -H "Content-Type: application/json" \
    -d "{\"msgtype\": \"$msgtype\", \"body\": $escaped_body}" 2>/dev/null)

  if [[ -z "$resp" ]]; then
    echo -e "${RED}[matrix] send request failed${NC}" >&2
    return 1
  fi

  local event_id
  event_id=$(echo "$resp" | jq -r '.event_id // empty' 2>/dev/null)
  if [[ -z "$event_id" ]]; then
    local err
    err=$(echo "$resp" | jq -r '.error // "unknown"' 2>/dev/null)
    echo -e "${RED}[matrix] send failed: $err${NC}" >&2
    return 1
  fi

  echo "$event_id"
}

# ── matrix_sync <access_token> [since] [timeout_ms] ────────────────────────────
# Returns full sync JSON on stdout.
matrix_sync() {
  local access_token="$1"
  local since="${2:-}"
  local timeout_ms="${3:-0}"
  local url="${CONDUIT_URL:-http://localhost:6167}"

  if [[ -z "$access_token" ]]; then
    echo -e "${RED}[matrix] sync requires access_token${NC}" >&2
    return 1
  fi

  local endpoint="$url/_matrix/client/v3/sync?access_token=${access_token}&timeout=${timeout_ms}"
  if [[ -n "$since" ]]; then
    endpoint="${endpoint}&since=${since}"
  fi

  local resp
  resp=$(curl -sf "$endpoint" 2>/dev/null)

  if [[ -z "$resp" ]]; then
    echo -e "${RED}[matrix] sync request failed${NC}" >&2
    return 1
  fi

  echo "$resp"
}

# ── matrix_poll_notice <access_token> <room_id> [expected_substring] [timeout] ─
# Polls sync for m.notice events. Returns matching event JSON on stdout.
# Default timeout: 15s. Polls every 2s.
matrix_poll_notice() {
  local access_token="$1"
  local room_id="$2"
  local expected="${3:-}"
  local timeout="${4:-15}"
  local url="${CONDUIT_URL:-http://localhost:6167}"

  if [[ -z "$access_token" || -z "$room_id" ]]; then
    echo -e "${RED}[matrix] poll_notice requires access_token, room_id${NC}" >&2
    return 1
  fi

  local since=""
  local elapsed=0

  while [[ $elapsed -lt $timeout ]]; do
    local endpoint="$url/_matrix/client/v3/sync?access_token=${access_token}&timeout=1000"
    if [[ -n "$since" ]]; then
      endpoint="${endpoint}&since=${since}"
    fi

    local resp
    resp=$(curl -sf "$endpoint" 2>/dev/null) || true

    if [[ -n "$resp" ]]; then
      since=$(echo "$resp" | jq -r '.next_batch // empty' 2>/dev/null)

      # Extract timeline events for the target room
      local events
      events=$(echo "$resp" | jq -c --arg room "$room_id" '
        .rooms.join[$room].timeline.events // []
      ' 2>/dev/null)

      local match
      if [[ -n "$expected" ]]; then
        match=$(echo "$events" | jq -c --arg sub "$expected" '
          [.[] | select(.type == "m.notice" and (.content.body // "" | test($sub; "i")))][0]
        ' 2>/dev/null)
      else
        match=$(echo "$events" | jq -c '
          [.[] | select(.type == "m.notice")][0]
        ' 2>/dev/null)
      fi

      if [[ -n "$match" && "$match" != "null" ]]; then
        echo "$match"
        return 0
      fi
    fi

    sleep 2
    elapsed=$((elapsed + 2))
  done

  echo -e "${RED}[matrix] poll_notice timed out after ${timeout}s${NC}" >&2
  return 1
}

# ── matrix_create_room <access_token> [room_name] [preset] ─────────────────────
# Returns room_id on stdout.
matrix_create_room() {
  local access_token="$1"
  local room_name="${2:-test-room-$$}"
  local preset="${3:-private_chat}"
  local url="${CONDUIT_URL:-http://localhost:6167}"

  if [[ -z "$access_token" ]]; then
    echo -e "${RED}[matrix] create_room requires access_token${NC}" >&2
    return 1
  fi

  local resp
  resp=$(curl -sf -X POST "$url/_matrix/client/v3/createRoom?access_token=${access_token}" \
    -H "Content-Type: application/json" \
    -d "{\"name\": \"$room_name\", \"visibility\": \"private\", \"preset\": \"$preset\"}" 2>/dev/null)

  if [[ -z "$resp" ]]; then
    echo -e "${RED}[matrix] create_room request failed${NC}" >&2
    return 1
  fi

  local room_id
  room_id=$(echo "$resp" | jq -r '.room_id // empty' 2>/dev/null)
  if [[ -z "$room_id" ]]; then
    local err
    err=$(echo "$resp" | jq -r '.error // "unknown"' 2>/dev/null)
    echo -e "${RED}[matrix] create_room failed: $err${NC}" >&2
    return 1
  fi

  echo "$room_id"
}

# ── matrix_invite <access_token> <room_id> <user_id> ───────────────────────────
# Returns 0 on success.
matrix_invite() {
  local access_token="$1"
  local room_id="$2"
  local user_id="$3"
  local url="${CONDUIT_URL:-http://localhost:6167}"

  if [[ -z "$access_token" || -z "$room_id" || -z "$user_id" ]]; then
    echo -e "${RED}[matrix] invite requires access_token, room_id, user_id${NC}" >&2
    return 1
  fi

  local resp
  resp=$(curl -sf -X POST "$url/_matrix/client/v3/rooms/${room_id}/invite?access_token=${access_token}" \
    -H "Content-Type: application/json" \
    -d "{\"user_id\": \"${user_id}\"}" 2>/dev/null)

  if [[ -z "$resp" ]]; then
    echo -e "${RED}[matrix] invite request failed${NC}" >&2
    return 1
  fi

  local err
  err=$(echo "$resp" | jq -r '.error // empty' 2>/dev/null)
  if [[ -n "$err" ]]; then
    echo -e "${RED}[matrix] invite failed: $err${NC}" >&2
    return 1
  fi

  return 0
}

# ── matrix_join <access_token> <room_id_or_alias> ──────────────────────────────
# Returns room_id on stdout.
matrix_join() {
  local access_token="$1"
  local room_id="$2"
  local url="${CONDUIT_URL:-http://localhost:6167}"

  if [[ -z "$access_token" || -z "$room_id" ]]; then
    echo -e "${RED}[matrix] join requires access_token, room_id${NC}" >&2
    return 1
  fi

  local resp
  resp=$(curl -sf -X POST "$url/_matrix/client/v3/rooms/${room_id}/join?access_token=${access_token}" \
    -H "Content-Type: application/json" \
    -d "{}" 2>/dev/null)

  if [[ -z "$resp" ]]; then
    echo -e "${RED}[matrix] join request failed${NC}" >&2
    return 1
  fi

  local err
  err=$(echo "$resp" | jq -r '.error // empty' 2>/dev/null)
  if [[ -n "$err" ]]; then
    echo -e "${RED}[matrix] join failed: $err${NC}" >&2
    return 1
  fi

  local joined_id
  joined_id=$(echo "$resp" | jq -r '.room_id // empty' 2>/dev/null)
  echo "${joined_id:-$room_id}"
}

# ── matrix_get_messages <access_token> <room_id> [from] [dir] [limit] ──────────
# Returns messages JSON on stdout.
matrix_get_messages() {
  local access_token="$1"
  local room_id="$2"
  local from="${3:-}"
  local dir="${4:-b}"
  local limit="${5:-50}"
  local url="${CONDUIT_URL:-http://localhost:6167}"

  if [[ -z "$access_token" || -z "$room_id" ]]; then
    echo -e "${RED}[matrix] get_messages requires access_token, room_id${NC}" >&2
    return 1
  fi

  local endpoint="$url/_matrix/client/v3/rooms/${room_id}/messages?access_token=${access_token}&dir=${dir}&limit=${limit}"
  if [[ -n "$from" ]]; then
    endpoint="${endpoint}&from=${from}"
  fi

  local resp
  resp=$(curl -sf "$endpoint" 2>/dev/null)

  if [[ -z "$resp" ]]; then
    echo -e "${RED}[matrix] get_messages request failed${NC}" >&2
    return 1
  fi

  echo "$resp"
}
