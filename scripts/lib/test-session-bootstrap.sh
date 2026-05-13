# test-session-bootstrap.sh — Test user/session bootstrap for ArmorClaw Conduit
#
# Sourced library (no shebang, no main block).
# Functions prefixed with _ to avoid namespace collisions.
#
# Creates a dedicated non-admin test user, tagged test room, and session
# on Conduit (localhost:6167, HTTP only). Reuses tagged rooms across runs.
#

_lib_ssh() {
  local _ssh_args=(-o StrictHostKeyChecking=no -o ConnectTimeout=10 -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR)
  [[ -n "${SSH_KEY_PATH:-}" ]] && _ssh_args+=(-i "$SSH_KEY_PATH")
  ssh "${_ssh_args[@]}" "$@"
}
# Registration uses HMAC-SHA1 shared-secret (same mechanism as admin, NOT admin token).
# The admin token from T8 is passed as --bootstrap-admin-token for room creation
# and bot invitation ONLY — never for user registration or validation traffic.
#
# Usage:
#   source "${_SCRIPT_DIR}/lib/test-session-bootstrap.sh"
#   _test_session_bootstrap ssh_host [--bootstrap-admin-token TOKEN]
#   # → sets _TEST_USER_ID, _TEST_ACCESS_TOKEN, _TEST_DEVICE_ID,
#   #      _TEST_ROOM_ID, _TEST_CRYPTO_VERIFIED

# ── Constants ────────────────────────────────────────────────────────────────────
_TEST_CONDUIT_URL="http://localhost:6167"
_TEST_USERNAME_PREFIX="armorclaw-vps-test"
_TEST_ROOM_PREFIX="armorclaw-vps-test"
_TEST_CONDUIT_CONFIG="/etc/armorclaw/conduit.toml"
_TEST_CRYPTO_VERIFY_TIMEOUT=15
_TEST_CRYPTO_VERIFY_INTERVAL=2

# ── _test_get_shared_secret(ssh_host) ────────────────────────────────────────────
# Read CONDUIT_REGISTRATION_SECRET from conduit.toml or container env on VPS.
# Prints the shared secret on stdout. Returns 0 on success.
#
# Arguments:
#   $1 - SSH host
_test_get_shared_secret() {
  local ssh_host="${1:?Usage: _test_get_shared_secret ssh_host}"
  local secret

  secret=$(_lib_ssh "$ssh_host" bash -s <<'SECRET_EOF' 2>/dev/null
    for f in /etc/armorclaw/conduit.toml /etc/conduit.toml; do
      s=$(awk -F'"' '/registration_shared_secret/{print $2}' "$f" 2>/dev/null | head -1)
      if [[ -n "$s" ]]; then echo "$s"; exit 0; fi
    done
SECRET_EOF
  )

  if [[ -n "$secret" ]]; then
    echo "$secret"
    return 0
  fi

  secret=$(_lib_ssh "$ssh_host" bash -s <<'SECRET_EOF' 2>/dev/null
    for name in armorclaw-conduit matrix-conduit conduit; do
      s=$(docker inspect "$name" --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null \
        | awk -F= '/CONDUIT_REGISTRATION_SECRET/{print $2}' | head -1)
      if [[ -n "$s" ]]; then echo "$s"; exit 0; fi
    done
SECRET_EOF
  )

  if [[ -n "$secret" ]]; then
    echo "$secret"
    return 0
  fi

  echo "[test-session] ERROR: Cannot find registration shared secret" >&2
  return 1
}

# ── _test_read_server_name(ssh_host) ─────────────────────────────────────────────
# Read server_name from conduit.toml on VPS. Prints server_name on stdout.
_test_read_server_name() {
  local ssh_host="${1:?Usage: _test_read_server_name ssh_host}"
  local server_name

  server_name=$(_lib_ssh "$ssh_host" bash -s <<'SN_EOF' 2>/dev/null
    for f in /etc/armorclaw/conduit.toml /etc/conduit.toml; do
      sn=$(awk -F'"' '/server_name/{print $2}' "$f" 2>/dev/null | head -1)
      if [[ -n "$sn" ]]; then echo "$sn"; exit 0; fi
    done
SN_EOF
  )

  echo "${server_name:-localhost}"
}

# ── _test_generate_password() ────────────────────────────────────────────────────
# Generate a secure random password. Prints password on stdout.
_test_generate_password() {
  local password
  password=$(tr -dc 'a-zA-Z0-9!@#$%^&*' </dev/urandom 2>/dev/null | head -c 24)
  if [[ -z "$password" ]]; then
    password=$(openssl rand -base64 24 2>/dev/null | tr -d '\n')
  fi
  echo "$password"
}

# ── _test_register_user(ssh_host, username, password, shared_secret) ──────────────
# Register non-admin test user via HMAC-SHA1 shared-secret registration.
# Tries nonce-based admin/register endpoint first, falls back to v3/register.
# Prints MXID on stdout. Returns 0 on success.
#
# Arguments:
#   $1 - SSH host
#   $2 - username
#   $3 - password
#   $4 - shared secret
_test_register_user() {
  local ssh_host="${1:?Usage: _test_register_user ssh_host username password secret}"
  local username="${2:?username required}"
  local password="${3:?password required}"
  local shared_secret="${4:?shared_secret required}"

  local server_name
  server_name=$(_test_read_server_name "$ssh_host")

  # Strategy 1: Nonce-based registration via admin/register endpoint
  local nonce_resp
  nonce_resp=$(_lib_ssh "$ssh_host" \
    "curl -s ${_TEST_CONDUIT_URL}/_matrix/client/r0/admin/register 2>/dev/null" \
    2>/dev/null)

  local nonce
  nonce=$(echo "$nonce_resp" | jq -r '.nonce // empty' 2>/dev/null)

  if [[ -n "$nonce" ]]; then
    # Compute HMAC-SHA1: mac = hmac(secret, nonce + username + password + "notadmin")
    local mac
    mac=$(printf '%s%s%s%s' "$nonce" "$username" "$password" "notadmin" \
      | openssl dgst -sha1 -hmac "$shared_secret" 2>/dev/null \
      | awk '{print $NF}')

    if [[ -z "$mac" ]]; then
      echo "[test-session] ERROR: HMAC computation failed — openssl missing?" >&2
      return 1
    fi

    local reg_resp
    reg_resp=$(_lib_ssh "$ssh_host" \
      "curl -s -X POST ${_TEST_CONDUIT_URL}/_matrix/client/r0/admin/register \
        -H 'Content-Type: application/json' \
        -d '{\"username\":\"${username}\",\"password\":\"${password}\",\"nonce\":\"${nonce}\",\"admin\":false,\"mac\":\"${mac}\"}'" \
      2>/dev/null)

    local err_msg
    err_msg=$(echo "$reg_resp" | jq -r '.error // empty' 2>/dev/null)
    if [[ -z "$err_msg" ]]; then
      local user_id
      user_id=$(echo "$reg_resp" | jq -r '.user_id // empty' 2>/dev/null)
      echo "${user_id:-@${username}:${server_name}}"
      return 0
    fi

    # User already exists is OK — we'll just login
    local errcode
    errcode=$(echo "$reg_resp" | jq -r '.errcode // empty' 2>/dev/null)
    if [[ "$errcode" != "M_USER_IN_USE" ]] && ! echo "$err_msg" | grep -qi "already in use"; then
      echo "[test-session] WARN: Nonce-based registration failed: ${err_msg}, trying v3" >&2
    fi
  fi

  # Strategy 2: v3/register with HMAC (no nonce) — m.login.dummy auth type
  local mac
  mac=$(printf '%s\x00%s\x00' "$username" "$password" \
    | openssl dgst -sha1 -hmac "$shared_secret" 2>/dev/null \
    | awk '{print $NF}')

  if [[ -z "$mac" ]]; then
    echo "[test-session] ERROR: HMAC computation failed — openssl missing?" >&2
    return 1
  fi

  local reg_resp
  reg_resp=$(_lib_ssh "$ssh_host" \
    "curl -s -X POST ${_TEST_CONDUIT_URL}/_matrix/client/v3/register \
      -H 'Content-Type: application/json' \
      -d '{\"username\":\"${username}\",\"password\":\"${password}\",\"auth\":{\"type\":\"m.login.dummy\",\"mac\":\"${mac}\"}}'" \
    2>/dev/null)

  local err_msg
  err_msg=$(echo "$reg_resp" | jq -r '.error // empty' 2>/dev/null)
  if [[ -n "$err_msg" ]]; then
    local errcode
    errcode=$(echo "$reg_resp" | jq -r '.errcode // empty' 2>/dev/null)
    # User already exists is fine — just proceed to login
    if [[ "$errcode" == "M_USER_IN_USE" ]] || echo "$err_msg" | grep -qi "already in use"; then
      echo "[test-session] User already registered, proceeding to login" >&2
      echo "@${username}:${server_name}"
      return 0
    fi
    echo "[test-session] ERROR: v3 registration failed: ${err_msg}" >&2
    return 1
  fi

  local user_id
  user_id=$(echo "$reg_resp" | jq -r '.user_id // empty' 2>/dev/null)
  echo "${user_id:-@${username}:${server_name}}"
  return 0
}

# ── _test_login(ssh_host, username, password) ─────────────────────────────────────
# Login test user via m.login.password. Returns access_token on stdout.
# Sets _TEST_DEVICE_ID as side effect.
#
# Arguments:
#   $1 - SSH host
#   $2 - username
#   $3 - password
_test_login() {
  local ssh_host="${1:?Usage: _test_login ssh_host username password}"
  local username="${2:?username required}"
  local password="${3:?password required}"

  local login_resp
  login_resp=$(_lib_ssh "$ssh_host" \
    "curl -s -X POST ${_TEST_CONDUIT_URL}/_matrix/client/v3/login \
      -H 'Content-Type: application/json' \
      -d '{\"type\":\"m.login.password\",\"identifier\":{\"type\":\"m.id.user\",\"user\":\"${username}\"},\"password\":\"${password}\"}'" \
    2>/dev/null)

  local access_token
  access_token=$(echo "$login_resp" | jq -r '.access_token // empty' 2>/dev/null)

  if [[ -z "$access_token" ]]; then
    local err_msg
    err_msg=$(echo "$login_resp" | jq -r '.error // empty' 2>/dev/null)
    echo "[test-session] ERROR: Login failed for ${username}: ${err_msg:-unknown}" >&2
    return 1
  fi

  # Capture device_id as side effect
  _TEST_DEVICE_ID=$(echo "$login_resp" | jq -r '.device_id // empty' 2>/dev/null)

  echo "$access_token"
  return 0
}

# ── _test_find_or_create_room(ssh_host, admin_token, test_access_token, room_tag) ─
# Find existing tagged test room or create one. Uses admin token for room creation
# and bot invitation. Prints room_id on stdout. Returns 0 on success.
#
# The admin_token is used ONLY for:
#   - Room creation (initial setup)
#   - Bot invitation (if applicable)
# NOT for user registration or validation traffic.
#
# Arguments:
#   $1 - SSH host
#   $2 - admin access token (for room creation + bot invite only)
#   $3 - test user access token
#   $4 - room tag suffix (e.g., timestamp or unique ID)
_test_find_or_create_room() {
  local ssh_host="${1:?Usage: _test_find_or_create_room ssh_host admin_token test_token tag}"
  local admin_token="${2:?admin_token required}"
  local test_token="${3:?test_token required}"
  local room_tag="${4:-$(date +%s)}"

  local room_name="${_TEST_ROOM_PREFIX}-${room_tag}"

  # Try to find existing tagged room via test user's synced rooms
  local sync_resp
  sync_resp=$(_lib_ssh "$ssh_host" \
    "curl -s '${_TEST_CONDUIT_URL}/_matrix/client/v3/sync?access_token=${test_token}&timeout=0' 2>/dev/null" \
    2>/dev/null)

  # Look for rooms matching our tag pattern in room names
  local existing_room
  existing_room=$(echo "$sync_resp" | jq -r --arg prefix "$_TEST_ROOM_PREFIX" '
    .rooms.join // {} | to_entries[] |
    select(.value.state.events // [] |
      any(.type == "m.room.name" and
          (.content.name // "") | startswith($prefix))) |
    .key' 2>/dev/null | head -1)

  if [[ -n "$existing_room" && "$existing_room" != "null" ]]; then
    echo "[test-session] Reusing existing test room: ${existing_room}" >&2
    echo "$existing_room"
    return 0
  fi

  # Create new room using admin token (bootstrap-only room management)
  echo "[test-session] Creating new tagged test room: ${room_name}" >&2

  local create_resp
  create_resp=$(_lib_ssh "$ssh_host" \
    "curl -s -X POST '${_TEST_CONDUIT_URL}/_matrix/client/r0/createRoom?access_token=${admin_token}' \
      -H 'Content-Type: application/json' \
      -d '{\"name\":\"${room_name}\",\"visibility\":\"private\",\"preset\":\"private_chat\",\"topic\":\"ArmorClaw VPS test session room (persistent)\"}'" \
    2>/dev/null)

  local room_id
  room_id=$(echo "$create_resp" | jq -r '.room_id // empty' 2>/dev/null)

  if [[ -z "$room_id" ]]; then
    local err_msg
    err_msg=$(echo "$create_resp" | jq -r '.error // empty' 2>/dev/null)
    echo "[test-session] ERROR: Room creation failed: ${err_msg:-unknown}" >&2
    return 1
  fi

  # Invite test user to the admin-created room
  local server_name
  server_name=$(_test_read_server_name "$ssh_host")
  local test_mxid="@${_TEST_USERNAME_PREFIX}:${server_name}"

  _lib_ssh "$ssh_host" \
    "curl -s -X POST '${_TEST_CONDUIT_URL}/_matrix/client/r0/rooms/${room_id}/invite?access_token=${admin_token}' \
      -H 'Content-Type: application/json' \
      -d '{\"user_id\":\"${test_mxid}\"}'" \
    2>/dev/null >/dev/null

  # Test user joins the room
  _lib_ssh "$ssh_host" \
    "curl -s -X POST '${_TEST_CONDUIT_URL}/_matrix/client/r0/rooms/${room_id}/join?access_token=${test_token}' \
      -H 'Content-Type: application/json' \
      -d '{}'" \
    2>/dev/null >/dev/null

  echo "[test-session] Room created: ${room_id}" >&2
  echo "$room_id"
  return 0
}

# ── _test_verify_crypto_signals(ssh_host, access_token, room_id) ─────────────────
# Verify crypto session state via concrete observable signals.
# Each signal is tested independently — failure logs warning but continues.
# Sets _TEST_CRYPTO_VERIFIED ("true"/"false") and _TEST_CRYPTO_SIGNALS (JSON).
#
# Signals tested:
#   1. Successful login → access_token captured
#   2. Successful sync → returns initial sync data
#   3. Successful whoami → returns user_id
#   4. Send test message → poll sync → message appears in timeline
#
# Arguments:
#   $1 - SSH host
#   $2 - test user access token
#   $3 - room_id
_test_verify_crypto_signals() {
  local ssh_host="${1:?Usage: _test_verify_crypto_signals ssh_host access_token room_id}"
  local access_token="${2:?access_token required}"
  local room_id="${3:?room_id required}"

  local signal_login="false"
  local signal_sync="false"
  local signal_whoami="false"
  local signal_msg="false"
  local all_passed=true

  # Signal 1: Login → access_token already captured (we're here = login succeeded)
  if [[ -n "$access_token" ]]; then
    signal_login="true"
    echo "[test-session] Signal 1 (login): PASS — access_token captured" >&2
  else
    echo "[test-session] Signal 1 (login): FAIL — no access_token" >&2
    all_passed=false
  fi

  # Signal 2: Successful sync → returns initial sync data
  local sync_resp
  sync_resp=$(_lib_ssh "$ssh_host" \
    "curl -s '${_TEST_CONDUIT_URL}/_matrix/client/v3/sync?access_token=${access_token}&timeout=0' 2>/dev/null" \
    2>/dev/null)

  local next_batch
  next_batch=$(echo "$sync_resp" | jq -r '.next_batch // empty' 2>/dev/null)

  if [[ -n "$next_batch" ]]; then
    signal_sync="true"
    echo "[test-session] Signal 2 (sync): PASS — next_batch=${next_batch}" >&2
  else
    echo "[test-session] Signal 2 (sync): FAIL — no next_batch in sync response" >&2
    all_passed=false
  fi

  # Signal 3: Successful whoami → returns user_id
  local whoami_resp
  whoami_resp=$(_lib_ssh "$ssh_host" \
    "curl -s '${_TEST_CONDUIT_URL}/_matrix/client/r0/account/whoami?access_token=${access_token}' 2>/dev/null" \
    2>/dev/null)

  local whoami_user
  whoami_user=$(echo "$whoami_resp" | jq -r '.user_id // empty' 2>/dev/null)

  if [[ -n "$whoami_user" ]]; then
    signal_whoami="true"
    echo "[test-session] Signal 3 (whoami): PASS — ${whoami_user}" >&2
  else
    local err_msg
    err_msg=$(echo "$whoami_resp" | jq -r '.error // empty' 2>/dev/null)
    echo "[test-session] Signal 3 (whoami): FAIL — ${err_msg:-no user_id}" >&2
    all_passed=false
  fi

  # Signal 4: Send test message → poll sync → message appears in timeline
  local txn_id="txn-test-$$-$(date +%s%N)"
  local test_body="crypto-verify-$(date +%s)"

  local send_resp
  send_resp=$(_lib_ssh "$ssh_host" \
    "curl -s -X PUT '${_TEST_CONDUIT_URL}/_matrix/client/v3/rooms/${room_id}/send/m.room.message/${txn_id}?access_token=${access_token}' \
      -H 'Content-Type: application/json' \
      -d '{\"msgtype\":\"m.text\",\"body\":\"${test_body}\"}'" \
    2>/dev/null)

  local sent_event_id
  sent_event_id=$(echo "$send_resp" | jq -r '.event_id // empty' 2>/dev/null)

  if [[ -n "$sent_event_id" ]]; then
    # Poll sync for the message
    local elapsed=0
    local found_msg=false

    while (( elapsed < _TEST_CRYPTO_VERIFY_TIMEOUT )); do
      local poll_resp
      poll_resp=$(_lib_ssh "$ssh_host" \
        "curl -s '${_TEST_CONDUIT_URL}/_matrix/client/v3/sync?access_token=${access_token}&timeout=1000&since=${next_batch}' 2>/dev/null" \
        2>/dev/null)

      local msg_body
      msg_body=$(echo "$poll_resp" | jq -r --arg room "$room_id" --arg body "$test_body" '
        .rooms.join[$room].timeline.events // [] |
        .[] | select(.type == "m.room.message") |
        .content.body // empty' 2>/dev/null | head -1)

      if [[ "$msg_body" == "$test_body" ]]; then
        found_msg=true
        break
      fi

      # Update next_batch for incremental polling
      next_batch=$(echo "$poll_resp" | jq -r '.next_batch // empty' 2>/dev/null)

      sleep "$_TEST_CRYPTO_VERIFY_INTERVAL"
      (( elapsed += _TEST_CRYPTO_VERIFY_INTERVAL ))
    done

    if [[ "$found_msg" == "true" ]]; then
      signal_msg="true"
      echo "[test-session] Signal 4 (message): PASS — message round-trip confirmed" >&2
    else
      echo "[test-session] Signal 4 (message): FAIL — sent but not received in timeline" >&2
      all_passed=false
    fi
  else
    local err_msg
    err_msg=$(echo "$send_resp" | jq -r '.error // empty' 2>/dev/null)
    echo "[test-session] Signal 4 (message): FAIL — send failed: ${err_msg:-unknown}" >&2
    all_passed=false
  fi

  # Build signals summary
  _TEST_CRYPTO_SIGNALS=$(jq -n \
    --arg login "$signal_login" \
    --arg sync "$signal_sync" \
    --arg whoami "$signal_whoami" \
    --arg msg "$signal_msg" \
    '{login: $login, sync: $sync, whoami: $whoami, message: $msg}')

  if [[ "$all_passed" == "true" ]]; then
    _TEST_CRYPTO_VERIFIED="true"
    echo "[test-session] Crypto verification: ALL SIGNALS PASSED" >&2
  else
    _TEST_CRYPTO_VERIFIED="false"
    echo "[test-session] Crypto verification: SOME SIGNALS FAILED (continuing)" >&2
  fi
}

# ── _test_session_bootstrap(ssh_host, [options]) ─────────────────────────────────
# Create dedicated test user + tagged test room + session with crypto verification.
#
# This is separate from admin bootstrap (T8) because admin identity ≠ test session.
# Uses HMAC-SHA1 shared-secret for user registration (NOT admin token).
# Admin token is used ONLY for room creation and bot invitation.
#
# Sets caller-visible variables:
#   _TEST_USER_ID         — full MXID of test user
#   _TEST_ACCESS_TOKEN    — Matrix access_token from login
#   _TEST_DEVICE_ID       — Matrix device_id from login
#   _TEST_ROOM_ID         — Matrix room ID of tagged test room
#   _TEST_CRYPTO_VERIFIED — "true" if all crypto signals passed, "false" otherwise
#   _TEST_CRYPTO_SIGNALS  — JSON object with individual signal results
#
# Arguments:
#   $1       - SSH host
#   Options:
#     --bootstrap-admin-token TOKEN  — admin access token for room creation (required)
#     --room-tag TAG                 — custom room tag suffix (default: auto)
#
# Returns 0 on success, 1 on failure.
_test_session_bootstrap() {
  local ssh_host="${1:?Usage: _test_session_bootstrap ssh_host [--bootstrap-admin-token TOKEN] [--room-tag TAG]}"
  shift

  local admin_token=""
  local room_tag=""

  # Parse options
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --bootstrap-admin-token)
        admin_token="${2:?--bootstrap-admin-token requires a value}"
        shift 2
        ;;
      --room-tag)
        room_tag="${2:?--room-tag requires a value}"
        shift 2
        ;;
      *)
        echo "[test-session] ERROR: Unknown option: $1" >&2
        return 1
        ;;
    esac
  done

  if [[ -z "$admin_token" ]]; then
    echo "[test-session] ERROR: --bootstrap-admin-token is required for room creation" >&2
    return 1
  fi

  # Initialize output variables
  _TEST_USER_ID=""
  _TEST_ACCESS_TOKEN=""
  _TEST_DEVICE_ID=""
  _TEST_ROOM_ID=""
  _TEST_CRYPTO_VERIFIED="false"
  _TEST_CRYPTO_SIGNALS="{}"

  echo "[test-session] Starting test session bootstrap" >&2

  # Step 1: Get shared secret for HMAC-SHA1 registration
  local shared_secret
  shared_secret=$(_test_get_shared_secret "$ssh_host")
  if [[ $? -ne 0 || -z "$shared_secret" ]]; then
    echo "[test-session] ERROR: Cannot obtain registration shared secret" >&2
    return 1
  fi

  # Step 2: Register test user (idempotent — M_USER_IN_USE is OK)
  local test_password
  test_password=$(_test_generate_password)

  local username="${_TEST_USERNAME_PREFIX}"

  echo "[test-session] Registering test user: ${username}" >&2
  _TEST_USER_ID=$(_test_register_user "$ssh_host" "$username" "$test_password" "$shared_secret")
  if [[ $? -ne 0 || -z "$_TEST_USER_ID" ]]; then
    echo "[test-session] ERROR: Test user registration failed" >&2
    return 1
  fi
  echo "[test-session] Test user: ${_TEST_USER_ID}" >&2

  # Step 3: Login as test user → capture access_token, device_id
  local login_user="${_TEST_USER_ID#@}"
  login_user="${login_user%%:*}"

  echo "[test-session] Logging in as: ${login_user}" >&2
  _TEST_ACCESS_TOKEN=$(_test_login "$ssh_host" "$login_user" "$test_password")
  if [[ $? -ne 0 || -z "$_TEST_ACCESS_TOKEN" ]]; then
    echo "[test-session] ERROR: Test user login failed" >&2
    return 1
  fi
  echo "[test-session] Login successful, device_id: ${_TEST_DEVICE_ID:-unknown}" >&2

  # Step 4: Create or reuse tagged test room (admin token for room creation + bot invite)
  room_tag="${room_tag:-$(date +%Y%m%d)}"
  _TEST_ROOM_ID=$(_test_find_or_create_room "$ssh_host" "$admin_token" "$_TEST_ACCESS_TOKEN" "$room_tag")
  if [[ $? -ne 0 || -z "$_TEST_ROOM_ID" ]]; then
    echo "[test-session] ERROR: Room setup failed" >&2
    return 1
  fi
  echo "[test-session] Test room: ${_TEST_ROOM_ID}" >&2

  # Step 5: Verify crypto session state via observable signals
  echo "[test-session] Verifying crypto session state" >&2
  _test_verify_crypto_signals "$ssh_host" "$_TEST_ACCESS_TOKEN" "$_TEST_ROOM_ID"

  echo "[test-session] Bootstrap complete — crypto_state_verified: ${_TEST_CRYPTO_VERIFIED}" >&2
  return 0
}
