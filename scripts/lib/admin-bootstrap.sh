# admin-bootstrap.sh — Admin identity bootstrap for ArmorClaw Conduit
#
# Sourced library (no shebang, no main block).
# Functions prefixed with _ to avoid namespace collisions.
#
# Ensures an admin user exists on Conduit (localhost:6167, HTTP only).
# Two strategies:
#   1. If cmd/bootstrap-admin binary exists on VPS: invoke it over SSH

_lib_ssh() {
  local _ssh_args=(-o StrictHostKeyChecking=no -o ConnectTimeout=10 -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR)
  [[ -n "${SSH_KEY_PATH:-}" ]] && _ssh_args+=(-i "$SSH_KEY_PATH")
  ssh "${_ssh_args[@]}" "$@"
}
#   2. Otherwise: emulate via HMAC-SHA1 shared-secret registration
#
# After registration, performs an EXPLICIT Matrix login to obtain access_token.
#
# Usage:
#   source "${_SCRIPT_DIR}/lib/admin-bootstrap.sh"
#   _admin_bootstrap ssh_host                 # → sets _ADMIN_USER_ID, _ADMIN_PASSWORD, _ADMIN_ACCESS_TOKEN
#   _admin_is_bootstrapped ssh_host           # → 0 if bootstrapped, 1 otherwise

# ── Constants ────────────────────────────────────────────────────────────────────
_ADMIN_CONDUIT_URL="http://localhost:6167"
_ADMIN_GUARD_FILE="/var/lib/armorclaw/.bootstrapped"
_ADMIN_CONDUIT_CONFIG="/etc/armorclaw/conduit.toml"
_ADMIN_USERNAME_FILE="/var/lib/armorclaw/.admin_username"
_ADMIN_PASSWORD_FILE="/var/lib/armorclaw/.admin_password"
_ADMIN_CONDUIT_WAIT_TIMEOUT=120
_ADMIN_CONDUIT_WAIT_INTERVAL=2

# ── _admin_is_bootstrapped(ssh_host) ─────────────────────────────────────────────
# Check whether admin bootstrap has already completed on the VPS.
# Returns 0 if guard file exists, 1 otherwise.
#
# Arguments:
#   $1 - SSH host — used to check guard file on VPS
_admin_is_bootstrapped() {
  local ssh_host="${1:?Usage: _admin_is_bootstrapped ssh_host}"

  _lib_ssh "$ssh_host" "test -f ${_ADMIN_GUARD_FILE}" 2>/dev/null
}

# ── _admin_wait_conduit(ssh_host) ────────────────────────────────────────────────
# Wait for Conduit to become ready on localhost:6167 over SSH.
# Returns 0 when Conduit responds, 1 on timeout.
#
# Arguments:
#   $1 - SSH host
_admin_wait_conduit() {
  local ssh_host="${1:?Usage: _admin_wait_conduit ssh_host}"
  local elapsed=0

  while (( elapsed < _ADMIN_CONDUIT_WAIT_TIMEOUT )); do
    local http_code
    http_code=$(_lib_ssh "$ssh_host" \
      "curl -s -o /dev/null -w '%{http_code}' -m 5 ${_ADMIN_CONDUIT_URL}/_matrix/client/versions" \
      2>/dev/null)

    if [[ "$http_code" == "200" ]]; then
      return 0
    fi

    sleep "$_ADMIN_CONDUIT_WAIT_INTERVAL"
    (( elapsed += _ADMIN_CONDUIT_WAIT_INTERVAL ))
  done

  echo "[admin-bootstrap] ERROR: Conduit not ready after ${_ADMIN_CONDUIT_WAIT_TIMEOUT}s" >&2
  return 1
}

# ── _admin_get_shared_secret(ssh_host) ───────────────────────────────────────────
# Read CONDUIT_REGISTRATION_SECRET from conduit.toml or container env on VPS.
# Prints the shared secret on stdout.
#
# Arguments:
#   $1 - SSH host
_admin_get_shared_secret() {
  local ssh_host="${1:?Usage: _admin_get_shared_secret ssh_host}"
  local secret

  # Strategy 1: Try conduit.toml (multiple known locations)
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

  # Strategy 2: Try container environment variables (multiple container names)
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

  echo "[admin-bootstrap] ERROR: Cannot find registration shared secret" >&2
  return 1
}

# ── _admin_generate_password() ───────────────────────────────────────────────────
# Generate a secure random password. Prints password on stdout.
_admin_generate_password() {
  local password
  password=$(tr -dc 'a-zA-Z0-9' </dev/urandom 2>/dev/null | head -c 32)
  if [[ -z "$password" ]]; then
    password=$(openssl rand -hex 16 2>/dev/null | tr -d '\n')
  fi
  echo "$password"
}

# ── _admin_read_server_name(ssh_host) ────────────────────────────────────────────
# Read server_name from conduit.toml on VPS. Prints server_name on stdout.
_admin_read_server_name() {
  local ssh_host="${1:?Usage: _admin_read_server_name ssh_host}"
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

# ── _admin_register_shell(ssh_host, username, password, shared_secret) ────────────
# Register admin user via HMAC-SHA1 shared-secret registration.
# Tries nonce-based admin endpoint first, falls back to v3 register.
# Prints the MXID on stdout. Returns 0 on success.
#
# Arguments:
#   $1 - SSH host
#   $2 - admin username
#   $3 - admin password
#   $4 - shared secret
_admin_register_shell() {
  local ssh_host="${1:?Usage: _admin_register_shell ssh_host username password secret}"
  local username="${2:?username required}"
  local password="${3:?password required}"
  local shared_secret="${4:?shared_secret required}"

  local max_retries=3
  local attempt_username="$username"
  local server_name

  server_name=$(_admin_read_server_name "$ssh_host")

  for (( retry = 0; retry < max_retries; retry++ )); do
    if (( retry > 0 )); then
      local suffix
      suffix=$(tr -dc 'a-z0-9' </dev/urandom 2>/dev/null | head -c 6)
      attempt_username="armor-admin-${suffix}"
      echo "[admin-bootstrap] Retrying with username: ${attempt_username}" >&2
    fi

    # Get nonce from Conduit admin register endpoint
    local nonce_resp
    nonce_resp=$(_lib_ssh "$ssh_host" \
      "curl -s ${_ADMIN_CONDUIT_URL}/_matrix/client/r0/admin/register 2>/dev/null" \
      2>/dev/null)

    local nonce
    nonce=$(echo "$nonce_resp" | jq -r '.nonce // empty' 2>/dev/null)

    if [[ -z "$nonce" ]]; then
      # Conduit v3 register with m.login.dummy (allow_registration must be true)
      local reg_resp
      reg_resp=$(_lib_ssh "$ssh_host" \
        "curl -s -X POST ${_ADMIN_CONDUIT_URL}/_matrix/client/v3/register \
          -H 'Content-Type: application/json' \
          -d '{\"username\":\"${attempt_username}\",\"password\":\"${password}\",\"auth\":{\"type\":\"m.login.dummy\"}}'" \
        2>/dev/null)

      local err_msg
      err_msg=$(echo "$reg_resp" | jq -r '.error // empty' 2>/dev/null)
      if [[ -n "$err_msg" ]]; then
        local errcode
        errcode=$(echo "$reg_resp" | jq -r '.errcode // empty' 2>/dev/null)
        if [[ "$errcode" == "M_USER_IN_USE" ]] || echo "$err_msg" | grep -qi "already in use"; then
          continue
        fi
        echo "[admin-bootstrap] ERROR: v3 registration failed: ${err_msg}" >&2
        return 1
      fi

      local user_id
      user_id=$(echo "$reg_resp" | jq -r '.user_id // empty' 2>/dev/null)
      if [[ -z "$user_id" ]]; then
        echo "[admin-bootstrap] ERROR: v3 registration returned no user_id" >&2
        return 1
      fi
      echo "${user_id}"
      return 0
    fi

    # Nonce-based HMAC-SHA1 — admin user
    local mac
    mac=$(printf '%s%s%s%s' "$nonce" "$attempt_username" "$password" "admin" \
      | openssl dgst -sha1 -hmac "$shared_secret" 2>/dev/null \
      | awk '{print $NF}')

    if [[ -z "$mac" ]]; then
      echo "[admin-bootstrap] ERROR: HMAC computation failed - openssl missing?" >&2
      return 1
    fi

    local reg_resp
    reg_resp=$(_lib_ssh "$ssh_host" \
      "curl -s -X POST ${_ADMIN_CONDUIT_URL}/_matrix/client/r0/admin/register \
        -H 'Content-Type: application/json' \
        -d '{\"username\":\"${attempt_username}\",\"password\":\"${password}\",\"nonce\":\"${nonce}\",\"admin\":true,\"mac\":\"${mac}\"}'" \
      2>/dev/null)

    local err_msg
    err_msg=$(echo "$reg_resp" | jq -r '.error // empty' 2>/dev/null)
    if [[ -n "$err_msg" ]]; then
      local errcode
      errcode=$(echo "$reg_resp" | jq -r '.errcode // empty' 2>/dev/null)
      if [[ "$errcode" == "M_USER_IN_USE" ]] || echo "$err_msg" | grep -qi "already in use"; then
        continue
      fi
      echo "[admin-bootstrap] ERROR: Registration failed: ${err_msg}" >&2
      return 1
    fi

    local user_id
    user_id=$(echo "$reg_resp" | jq -r '.user_id // empty' 2>/dev/null)
    echo "${user_id:-@${attempt_username}:${server_name}}"
    return 0
  done

  echo "[admin-bootstrap] ERROR: Failed to register admin after ${max_retries} attempts" >&2
  return 1
}

# ── _admin_login(ssh_host, username, password) ────────────────────────────────────
# Explicit Matrix login to obtain access_token. Prints access_token on stdout.
# Returns 0 on success, 1 on failure.
#
# Arguments:
#   $1 - SSH host
#   $2 - username
#   $3 - password
_admin_login() {
  local ssh_host="${1:?Usage: _admin_login ssh_host username password}"
  local username="${2:?username required}"
  local password="${3:?password required}"

  local login_resp
  login_resp=$(_lib_ssh "$ssh_host" \
    "curl -s -X POST ${_ADMIN_CONDUIT_URL}/_matrix/client/v3/login \
      -H 'Content-Type: application/json' \
      -d '{\"type\":\"m.login.password\",\"identifier\":{\"type\":\"m.id.user\",\"user\":\"${username}\"},\"password\":\"${password}\"}'" \
    2>/dev/null)

  local access_token
  access_token=$(echo "$login_resp" | jq -r '.access_token // empty' 2>/dev/null)

  if [[ -z "$access_token" ]]; then
    local err_msg
    err_msg=$(echo "$login_resp" | jq -r '.error // empty' 2>/dev/null)
    echo "[admin-bootstrap] ERROR: Login failed: ${err_msg:-unknown error}" >&2
    return 1
  fi

  echo "$access_token"
  return 0
}

# ── _admin_bootstrap(ssh_host) ────────────────────────────────────────────────────
# Ensure admin user exists on Conduit. Entry point for admin identity bootstrap.
#
# Strategy:
#   1. Check if Conduit is running — HTTP localhost:6167
#   2. Check if admin already bootstrapped — guard file
#   3. If cmd/bootstrap-admin binary exists on VPS: invoke it directly
#   4. If not: emulate via shell — HMAC-SHA1 registration, write guard file
#   5. Always perform explicit login to obtain access_token
#
# Sets caller-visible variables:
#   _ADMIN_USER_ID      — full MXID
#   _ADMIN_PASSWORD     — admin password
#   _ADMIN_ACCESS_TOKEN — Matrix access_token from explicit login
#
# Arguments:
#   $1 - SSH host
#
# Returns 0 on success, 1 on failure.
_admin_bootstrap() {
  local ssh_host="${1:?Usage: _admin_bootstrap ssh_host}"

  _ADMIN_USER_ID=""
  _ADMIN_PASSWORD=""
  _ADMIN_ACCESS_TOKEN=""

  # Step 1: Wait for Conduit
  if ! _admin_wait_conduit "$ssh_host"; then
    return 1
  fi

  # Step 2: Already bootstrapped? Read credentials and login
  if _admin_is_bootstrapped "$ssh_host"; then
    echo "[admin-bootstrap] Admin already bootstrapped - guard file exists" >&2

    _ADMIN_PASSWORD=$(_lib_ssh "$ssh_host" "cat ${_ADMIN_PASSWORD_FILE} 2>/dev/null" 2>/dev/null)
    local stored_username
    stored_username=$(_lib_ssh "$ssh_host" "cat ${_ADMIN_USERNAME_FILE} 2>/dev/null" 2>/dev/null)

    if [[ -n "$stored_username" && -n "$_ADMIN_PASSWORD" ]]; then
      local server_name
      server_name=$(_admin_read_server_name "$ssh_host")

      _ADMIN_USER_ID="@${stored_username}:${server_name}"

      _ADMIN_ACCESS_TOKEN=$(_admin_login "$ssh_host" "$stored_username" "$_ADMIN_PASSWORD")
      if [[ $? -eq 0 && -n "$_ADMIN_ACCESS_TOKEN" ]]; then
        echo "[admin-bootstrap] Re-authenticated existing admin: ${_ADMIN_USER_ID}" >&2
        return 0
      fi
      echo "[admin-bootstrap] WARN: Login with stored credentials failed, re-bootstrapping" >&2
    else
      echo "[admin-bootstrap] WARN: Stored credentials incomplete, re-bootstrapping" >&2
    fi
  fi

  # Step 3: Prepare credentials
  local admin_username="${ARMORCLAW_ADMIN_USERNAME:-}"
  local admin_password="${ARMORCLAW_ADMIN_PASSWORD:-}"
  if [[ -z "$admin_password" ]]; then
    admin_password=$(_admin_generate_password)
  fi
  _ADMIN_PASSWORD="$admin_password"

  if [[ -n "$admin_username" ]]; then
    if ! [[ "$admin_username" =~ ^[a-z0-9._-]+$ ]]; then
      echo "[admin-bootstrap] WARN: Invalid username, generating random" >&2
      local suffix
      suffix=$(tr -dc 'a-z0-9' </dev/urandom 2>/dev/null | head -c 8)
      admin_username="armor-admin-${suffix}"
    fi
  else
    local suffix
    suffix=$(tr -dc 'a-z0-9' </dev/urandom 2>/dev/null | head -c 8)
    admin_username="armor-admin-${suffix}"
  fi

  # Step 3b: Try cmd/bootstrap-admin binary on VPS
  if _lib_ssh "$ssh_host" "command -v bootstrap-admin >/dev/null 2>&1 || test -x /usr/local/bin/bootstrap-admin" 2>/dev/null; then
    echo "[admin-bootstrap] Found cmd/bootstrap-admin binary on VPS, invoking it" >&2

    local binary_path
    binary_path=$(_lib_ssh "$ssh_host" "command -v bootstrap-admin 2>/dev/null || echo /usr/local/bin/bootstrap-admin" 2>/dev/null)

    local server_name
    server_name=$(_admin_read_server_name "$ssh_host")

    local bootstrap_output
    bootstrap_output=$(_lib_ssh "$ssh_host" \
      "ARMORCLAW_ADMIN_USERNAME=${admin_username} ARMORCLAW_ADMIN_PASSWORD=${admin_password} ARMORCLAW_SERVER_NAME=${server_name} ${binary_path}" 2>&1)

    if [[ $? -eq 0 ]]; then
      echo "[admin-bootstrap] cmd/bootstrap-admin succeeded" >&2
      _ADMIN_USER_ID=$(echo "$bootstrap_output" | grep -o '@[a-z0-9._-]\+:[a-z0-9._-]\+' | head -1)
      if [[ -z "$_ADMIN_USER_ID" ]]; then
        _ADMIN_USER_ID="@${admin_username}:${server_name}"
      fi
    else
      echo "[admin-bootstrap] WARN: cmd/bootstrap-admin failed, falling back to shell registration" >&2
      _ADMIN_USER_ID=""
    fi
  fi

  # Step 4: Shell-based HMAC-SHA1 registration — fallback
  if [[ -z "$_ADMIN_USER_ID" ]]; then
    echo "[admin-bootstrap] Using shell-based HMAC-SHA1 registration" >&2

    local shared_secret
    shared_secret=$(_admin_get_shared_secret "$ssh_host")
    if [[ $? -ne 0 || -z "$shared_secret" ]]; then
      return 1
    fi

    _ADMIN_USER_ID=$(_admin_register_shell "$ssh_host" "$admin_username" "$admin_password" "$shared_secret")
    if [[ $? -ne 0 || -z "$_ADMIN_USER_ID" ]]; then
      return 1
    fi

    # Write guard file and credentials on VPS
    _lib_ssh "$ssh_host" "mkdir -p /var/lib/armorclaw && touch ${_ADMIN_GUARD_FILE}" 2>/dev/null

    _lib_ssh "$ssh_host" "cat > ${_ADMIN_USERNAME_FILE} <<EOU
${admin_username}
EOU
chmod 644 ${_ADMIN_USERNAME_FILE}" 2>/dev/null

    _lib_ssh "$ssh_host" "cat > ${_ADMIN_PASSWORD_FILE} <<EOP
${admin_password}
EOP
chmod 600 ${_ADMIN_PASSWORD_FILE}" 2>/dev/null

    echo "[admin-bootstrap] Shell registration complete: ${_ADMIN_USER_ID}" >&2
  fi

  # Step 5: Explicit Matrix login to obtain access_token
  local login_user
  login_user="${_ADMIN_USER_ID#@}"
  login_user="${login_user%%:*}"

  echo "[admin-bootstrap] Performing explicit Matrix login for ${login_user}" >&2
  _ADMIN_ACCESS_TOKEN=$(_admin_login "$ssh_host" "$login_user" "$_ADMIN_PASSWORD")
  if [[ $? -ne 0 || -z "$_ADMIN_ACCESS_TOKEN" ]]; then
    echo "[admin-bootstrap] ERROR: Post-registration login failed" >&2
    return 1
  fi

  echo "[admin-bootstrap] Admin bootstrap complete: ${_ADMIN_USER_ID}" >&2
  return 0
}
