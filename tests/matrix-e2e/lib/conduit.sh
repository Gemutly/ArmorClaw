#!/usr/bin/env bash
# conduit.sh — Conduit Matrix homeserver lifecycle management
#
# Provides functions to start/stop a Conduit container, check health,
# register users, create rooms, and invite the bridge bot.
#
# Uses: docker, curl, jq
# Requires: CONDUIT_PORT, CONDUIT_CONTAINER, CONDUIT_URL set by harness

# ── conduit_start [image] ──────────────────────────────────────────────────────
# Starts a Conduit container. Defaults to girlbossceo/conduit:latest.
# Sets CONDUIT_CONTAINER on success.
conduit_start() {
  local image="${1:-girlbossceo/conduit:latest}"
  local port="${CONDUIT_PORT:-6167}"
  local name="${CONDUIT_CONTAINER:-matrix-e2e-conduit-$$}"
  local config_dir="${CONDUIT_CONFIG_DIR:-$(dirname "$0")/../fixtures}"

  # Resolve absolute path for config mount
  config_dir="$(cd "$config_dir" && pwd)"

  echo -e "${YELLOW}[conduit] Starting container: $name${NC}"

  CONDUIT_CONTAINER=$(
    docker run -d --rm \
      --name "$name" \
      -p "${port}:6167" \
      -v "${config_dir}/test-config.toml:/data/conduit.toml:ro" \
      "$image" 2>/dev/null
  )

  if [[ -z "$CONDUIT_CONTAINER" ]]; then
    echo -e "${RED}[conduit] Failed to start container${NC}"
    return 1
  fi

  export CONDUIT_CONTAINER
  echo -e "${GREEN}[conduit] Started: ${CONDUIT_CONTAINER:0:12}${NC}"
  return 0
}

# ── conduit_stop ───────────────────────────────────────────────────────────────
# Stops and removes the Conduit container.
conduit_stop() {
  if [[ -z "${CONDUIT_CONTAINER:-}" ]]; then
    echo -e "${YELLOW}[conduit] No container to stop${NC}"
    return 0
  fi

  echo -e "${YELLOW}[conduit] Stopping: ${CONDUIT_CONTAINER:0:12}${NC}"
  docker stop "$CONDUIT_CONTAINER" 2>/dev/null || true
  docker rm -f "$CONDUIT_CONTAINER" 2>/dev/null || true
  unset CONDUIT_CONTAINER
  echo -e "${GREEN}[conduit] Stopped${NC}"
  return 0
}

# ── conduit_health [timeout_secs] ──────────────────────────────────────────────
# Polls Conduit health endpoint until ready. Default timeout: 30s.
conduit_health() {
  local timeout="${1:-30}"
  local url="${CONDUIT_URL:-http://localhost:${CONDUIT_PORT:-6167}}"
  local count=0

  echo -e "${YELLOW}[conduit] Waiting for health (${timeout}s)...${NC}"

  while [[ $count -lt $timeout ]]; do
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" "$url/_matrix/client/versions" 2>/dev/null || echo "000")
    if [[ "$code" == "200" ]]; then
      echo -e "${GREEN}[conduit] Healthy${NC}"
      return 0
    fi
    sleep 1
    ((count++)) || true
  done

  echo -e "${RED}[conduit] Not healthy after ${timeout}s (last code: ${code:-none})${NC}"
  return 1
}

# ── conduit_register <username> <password> ─────────────────────────────────────
# Registers a user via Conduit's shared-secret admin API.
# Returns the full MXID on stdout.
conduit_register() {
  local username="$1"
  local password="$2"
  local url="${CONDUIT_URL:-http://localhost:${CONDUIT_PORT:-6167}}"
  local server_name="${CONDUIT_SERVER_NAME:-armorclaw.test}"
  local shared_secret="${CONDUIT_REGISTRATION_SECRET:-test-registration-secret-change-in-production}"

  if [[ -z "$username" || -z "$password" ]]; then
    echo -e "${RED}[conduit] register requires username and password${NC}" >&2
    return 1
  fi

  echo -e "${YELLOW}[conduit] Registering user: $username${NC}"

  # Use the admin register endpoint with shared secret nonce
  local nonce_resp
  nonce_resp=$(curl -s "$url/_matrix/client/r0/admin/register" 2>/dev/null)

  local nonce
  nonce=$(echo "$nonce_resp" | jq -r '.nonce // empty' 2>/dev/null)

  if [[ -z "$nonce" ]]; then
    echo -e "${RED}[conduit] Failed to get registration nonce${NC}" >&2
    return 1
  fi

  # Compute HMAC-SHA1 for Conduit registration (mac = hmac(shared_secret, nonce+username+password+admin))
  local mac
  if command -v openssl &>/dev/null; then
    mac=$(printf '%s%s%s%s' "$nonce" "$username" "$password" "notadmin" \
      | openssl dgst -sha1 -hmac "$shared_secret" 2>/dev/null \
      | awk '{print $NF}')
  else
    echo -e "${RED}[conduit] openssl required for HMAC computation${NC}" >&2
    return 1
  fi

  local reg_resp
  reg_resp=$(curl -s -X POST "$url/_matrix/client/r0/admin/register" \
    -H "Content-Type: application/json" \
    -d "{
      \"username\": \"$username\",
      \"password\": \"$password\",
      \"nonce\": \"$nonce\",
      \"admin\": false,
      \"mac\": \"$mac\"
    }" 2>/dev/null)

  # Check for error
  local err_msg
  err_msg=$(echo "$reg_resp" | jq -r '.error // empty' 2>/dev/null)
  if [[ -n "$err_msg" ]]; then
    echo -e "${RED}[conduit] Registration failed: $err_msg${NC}" >&2
    return 1
  fi

  local mxid="@${username}:${server_name}"
  echo -e "${GREEN}[conduit] Registered: $mxid${NC}"
  echo "$mxid"
  return 0
}

# ── conduit_login <username> <password> ────────────────────────────────────────
# Logs in a user and returns the access token on stdout.
conduit_login() {
  local username="$1"
  local password="$2"
  local url="${CONDUIT_URL:-http://localhost:${CONDUIT_PORT:-6167}}"

  local resp
  resp=$(curl -s -X POST "$url/_matrix/client/r0/login" \
    -H "Content-Type: application/json" \
    -d "{
      \"type\": \"m.login.password\",
      \"user\": \"$username\",
      \"password\": \"$password\"
    }" 2>/dev/null)

  local token
  token=$(echo "$resp" | jq -r '.access_token // empty' 2>/dev/null)

  if [[ -z "$token" ]]; then
    echo -e "${RED}[conduit] Login failed for $username${NC}" >&2
    return 1
  fi

  echo "$token"
  return 0
}

# ── conduit_create_room <access_token> [room_name] ─────────────────────────────
# Creates a room and returns the room ID on stdout.
conduit_create_room() {
  local access_token="$1"
  local room_name="${2:-test-room-$$}"
  local url="${CONDUIT_URL:-http://localhost:${CONDUIT_PORT:-6167}}"

  local resp
  resp=$(curl -s -X POST "$url/_matrix/client/r0/createRoom?access_token=$access_token" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"$room_name\",
      \"visibility\": \"private\",
      \"preset\": \"private_chat\"
    }" 2>/dev/null)

  local room_id
  room_id=$(echo "$resp" | jq -r '.room_id // empty' 2>/dev/null)

  if [[ -z "$room_id" ]]; then
    echo -e "${RED}[conduit] Room creation failed${NC}" >&2
    return 1
  fi

  echo -e "${GREEN}[conduit] Room created: $room_id${NC}"
  echo "$room_id"
  return 0
}

# ── conduit_invite_bot <access_token> <room_id> <bot_mxid> ─────────────────────
# Invites the bridge bot to a room.
conduit_invite_bot() {
  local access_token="$1"
  local room_id="$2"
  local bot_mxid="$3"
  local url="${CONDUIT_URL:-http://localhost:${CONDUIT_PORT:-6167}}"

  local resp
  resp=$(curl -s -X POST "$url/_matrix/client/r0/rooms/${room_id}/invite?access_token=${access_token}" \
    -H "Content-Type: application/json" \
    -d "{\"user_id\": \"${bot_mxid}\"}" 2>/dev/null)

  local err_msg
  err_msg=$(echo "$resp" | jq -r '.error // empty' 2>/dev/null)
  if [[ -n "$err_msg" ]]; then
    echo -e "${RED}[conduit] Invite failed: $err_msg${NC}" >&2
    return 1
  fi

  echo -e "${GREEN}[conduit] Invited $bot_mxid to $room_id${NC}"
  return 0
}

# ── conduit_get_event <access_token> <room_id> <event_id> ──────────────────────
# Retrieves a single event. Returns the full event JSON on stdout.
conduit_get_event() {
  local access_token="$1"
  local room_id="$2"
  local event_id="$3"
  local url="${CONDUIT_URL:-http://localhost:${CONDUIT_PORT:-6167}}"

  curl -s "$url/_matrix/client/r0/rooms/${room_id}/event/${event_id}?access_token=${access_token}" 2>/dev/null
}
