# matrix-state.sh — Persist and load Matrix test state across runs
#
# Sourced library (no shebang, no main block).
# Functions prefixed with _ to avoid namespace collisions.
#
# Persists Matrix tokens and test session state to a JSON file so that
# test runs can reuse existing sessions instead of re-bootstrapping.
# State file has chmod 600 because it contains access/refresh tokens.

_lib_ssh() {
  local _ssh_args=(-o StrictHostKeyChecking=no -o ConnectTimeout=10 -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR)
  [[ -n "${SSH_KEY_PATH:-}" ]] && _ssh_args+=(-i "$SSH_KEY_PATH")
  ssh "${_ssh_args[@]}" "$@"
}
#
# Usage:
#   source "${_SCRIPT_DIR}/lib/matrix-state.sh"
#   _matrix_load_state
#   if _matrix_state_is_valid; then
#     echo "Reusing session for ${_MATRIX_USER_ID}"
#   else
#     _matrix_refresh_token || _test_session_bootstrap ...
#   fi

# ── Constants ────────────────────────────────────────────────────────────────────
_MATRIX_STATE_DIR="${_SCRIPT_DIR}/../.sisyphus/evidence/vps-lifecycle"
_MATRIX_STATE_FILE="${_MATRIX_STATE_DIR}/matrix-state.json"

# ── _matrix_save_state() ────────────────────────────────────────────────────────
# Save current Matrix session state to JSON file.
# Reads from caller-visible variables set by test-session-bootstrap.sh:
#   _TEST_ACCESS_TOKEN, _TEST_DEVICE_ID, _TEST_USER_ID, _TEST_ROOM_ID,
#   _TEST_CRYPTO_VERIFIED
# Also persists conduit_base_url and bootstrap_timestamp.
#
# Sets file permissions to 600 (owner-only, tokens are sensitive).
# Returns 0 on success, 1 on failure.
_matrix_save_state() {
  # Require access_token at minimum
  if [[ -z "${_TEST_ACCESS_TOKEN:-}" ]]; then
    echo "[matrix-state] ERROR: _TEST_ACCESS_TOKEN not set — nothing to save" >&2
    return 1
  fi

  mkdir -p "$_MATRIX_STATE_DIR"

  local state_json
  state_json=$(jq -n \
    --arg access_token "$_TEST_ACCESS_TOKEN" \
    --arg refresh_token "${_TEST_REFRESH_TOKEN:-}" \
    --arg device_id "${_TEST_DEVICE_ID:-}" \
    --arg user_id "${_TEST_USER_ID:-}" \
    --arg test_room_id "${_TEST_ROOM_ID:-}" \
    --arg conduit_base_url "${_TEST_CONDUIT_URL:-http://localhost:6167}" \
    --arg bootstrap_timestamp "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg crypto_state_verified "${_TEST_CRYPTO_VERIFIED:-false}" \
    '{
      access_token:          $access_token,
      refresh_token:         $refresh_token,
      device_id:             $device_id,
      user_id:               $user_id,
      test_room_id:          $test_room_id,
      conduit_base_url:      $conduit_base_url,
      bootstrap_timestamp:   $bootstrap_timestamp,
      crypto_state_verified: $crypto_state_verified
    }')

  if [[ -z "$state_json" ]]; then
    echo "[matrix-state] ERROR: jq failed to produce state JSON" >&2
    return 1
  fi

  echo "$state_json" > "$_MATRIX_STATE_FILE"
  chmod 600 "$_MATRIX_STATE_FILE"

  echo "[matrix-state] State saved to $_MATRIX_STATE_FILE (chmod 600)" >&2
  return 0
}

# ── _matrix_load_state() ────────────────────────────────────────────────────────
# Load persisted Matrix state from JSON file into shell variables.
# Sets caller-visible variables:
#   _TEST_ACCESS_TOKEN, _TEST_REFRESH_TOKEN, _TEST_DEVICE_ID,
#   _TEST_USER_ID, _TEST_ROOM_ID, _MATRIX_CONDUIT_BASE_URL,
#   _MATRIX_BOOTSTRAP_TIMESTAMP, _TEST_CRYPTO_VERIFIED
#
# Returns 0 if state file exists and was loaded, 1 otherwise.
# Returns 2 if state file exists but is invalid JSON.
_matrix_load_state() {
  if [[ ! -f "$_MATRIX_STATE_FILE" ]]; then
    echo "[matrix-state] No state file found at $_MATRIX_STATE_FILE" >&2
    return 1
  fi

  # Verify file is readable (permissions check)
  if [[ ! -r "$_MATRIX_STATE_FILE" ]]; then
    echo "[matrix-state] ERROR: State file not readable: $_MATRIX_STATE_FILE" >&2
    return 1
  fi

  local state_json
  state_json=$(cat "$_MATRIX_STATE_FILE")

  # Validate JSON
  if ! echo "$state_json" | jq -e '.' >/dev/null 2>&1; then
    echo "[matrix-state] ERROR: State file contains invalid JSON" >&2
    return 2
  fi

  # Extract fields into shell variables
  _TEST_ACCESS_TOKEN=$(echo "$state_json" | jq -r '.access_token // empty')
  _TEST_REFRESH_TOKEN=$(echo "$state_json" | jq -r '.refresh_token // empty')
  _TEST_DEVICE_ID=$(echo "$state_json" | jq -r '.device_id // empty')
  _TEST_USER_ID=$(echo "$state_json" | jq -r '.user_id // empty')
  _TEST_ROOM_ID=$(echo "$state_json" | jq -r '.test_room_id // empty')
  _MATRIX_CONDUIT_BASE_URL=$(echo "$state_json" | jq -r '.conduit_base_url // empty')
  _MATRIX_BOOTSTRAP_TIMESTAMP=$(echo "$state_json" | jq -r '.bootstrap_timestamp // empty')
  _TEST_CRYPTO_VERIFIED=$(echo "$state_json" | jq -r '.crypto_state_verified // "false"')

  # Minimum viable check: access_token must be present
  if [[ -z "$_TEST_ACCESS_TOKEN" ]]; then
    echo "[matrix-state] WARN: Loaded state has no access_token — session unusable" >&2
    return 1
  fi

  echo "[matrix-state] State loaded: user=${_TEST_USER_ID:-?} room=${_TEST_ROOM_ID:-?}" >&2
  return 0
}

# ── _matrix_state_is_valid(ssh_host) ────────────────────────────────────────────
# Verify the persisted access_token is still valid by calling whoami.
# Requires ssh_host to reach the Conduit instance.
#
# Arguments:
#   $1 - SSH host (to reach Conduit)
#
# Returns 0 if token is valid, 1 if invalid or expired.
_matrix_state_is_valid() {
  local ssh_host="${1:?Usage: _matrix_state_is_valid ssh_host}"

  if [[ -z "${_TEST_ACCESS_TOKEN:-}" ]]; then
    echo "[matrix-state] No access_token loaded — state invalid" >&2
    return 1
  fi

  local conduit_url="${_MATRIX_CONDUIT_BASE_URL:-http://localhost:6167}"

  local whoami_resp
  whoami_resp=$(_lib_ssh "$ssh_host" \
    "curl -s '${conduit_url}/_matrix/client/r0/account/whoami?access_token=${_TEST_ACCESS_TOKEN}' 2>/dev/null" \
    2>/dev/null)

  local whoami_user
  whoami_user=$(echo "$whoami_resp" | jq -r '.user_id // empty' 2>/dev/null)

  if [[ -n "$whoami_user" && "$whoami_user" != "null" ]]; then
    echo "[matrix-state] Token valid — whoami: ${whoami_user}" >&2
    return 0
  fi

  local errcode
  errcode=$(echo "$whoami_resp" | jq -r '.errcode // empty' 2>/dev/null)

  echo "[matrix-state] Token invalid — errcode: ${errcode:-unknown}" >&2
  return 1
}

# ── _matrix_refresh_token(ssh_host) ─────────────────────────────────────────────
# Refresh an expired access_token using the Matrix v3 refresh endpoint.
# Uses POST /_matrix/client/v3/refresh with the stored refresh_token.
# On success, updates shell variables and persists the new state.
#
# Arguments:
#   $1 - SSH host (to reach Conduit)
#
# Returns 0 on success, 1 on failure.
_matrix_refresh_token() {
  local ssh_host="${1:?Usage: _matrix_refresh_token ssh_host}"

  if [[ -z "${_TEST_REFRESH_TOKEN:-}" ]]; then
    echo "[matrix-state] ERROR: No refresh_token available — cannot refresh" >&2
    return 1
  fi

  local conduit_url="${_MATRIX_CONDUIT_BASE_URL:-http://localhost:6167}"

  echo "[matrix-state] Refreshing token via ${conduit_url}/_matrix/client/v3/refresh" >&2

  local refresh_resp
  refresh_resp=$(_lib_ssh "$ssh_host" \
    "curl -s -X POST '${conduit_url}/_matrix/client/v3/refresh' \
      -H 'Content-Type: application/json' \
      -d '{\"refresh_token\":\"${_TEST_REFRESH_TOKEN}\"}'" \
    2>/dev/null)

  local new_access_token
  new_access_token=$(echo "$refresh_resp" | jq -r '.access_token // empty' 2>/dev/null)

  if [[ -z "$new_access_token" ]]; then
    local err_msg
    err_msg=$(echo "$refresh_resp" | jq -r '.error // empty' 2>/dev/null)
    echo "[matrix-state] ERROR: Token refresh failed: ${err_msg:-unknown}" >&2
    return 1
  fi

  # Update shell variables with new tokens
  _TEST_ACCESS_TOKEN="$new_access_token"

  # Server may rotate the refresh token
  local new_refresh_token
  new_refresh_token=$(echo "$refresh_resp" | jq -r '.refresh_token // empty' 2>/dev/null)
  if [[ -n "$new_refresh_token" ]]; then
    _TEST_REFRESH_TOKEN="$new_refresh_token"
  fi

  # Persist the updated state
  _matrix_save_state

  echo "[matrix-state] Token refreshed successfully" >&2
  return 0
}
