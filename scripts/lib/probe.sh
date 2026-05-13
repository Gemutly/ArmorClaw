# probe.sh — HTTPS-first health probe library for ArmorClaw Bridge
#
# Sourced library (no shebang, no main block).
# Provides HTTPS-first probing for Bridge ONLY.
# Matrix/Conduit localhost checks always use plain HTTP.
#
# Usage:
#   source "${_SCRIPT_DIR}/lib/probe.sh"
#   _probe_bridge_health 8080        # → response body + sets _PROBE_SCHEME
#   _probe_bridge_rpc 8080 "status" "{}"
#   _probe_scheme_detect localhost 8080  # → "https" or "http"

# ── _probe_bridge_health(port) ──────────────────────────────────────────────────
# Try HTTPS first, fall back to HTTP for Bridge /health endpoint.
# Sets _PROBE_SCHEME to "https" or "http" so callers know which worked.
# Prints the response body on stdout (empty on failure).
# Returns 0 on success (200 response), 1 on failure.
#
# Arguments:
#   $1 - Port number (e.g. 8080)
_probe_bridge_health() {
  local port="${1:?Usage: _probe_bridge_health port}"
  local response

  # Try HTTPS first (self-signed OK)
  response=$(curl -sf -k -m 5 "https://localhost:${port}/health" 2>/dev/null)
  if [[ $? -eq 0 && -n "$response" ]]; then
    _PROBE_SCHEME="https"
    echo "$response"
    return 0
  fi

  # Fallback to HTTP
  response=$(curl -sf -m 5 "http://localhost:${port}/health" 2>/dev/null)
  if [[ $? -eq 0 && -n "$response" ]]; then
    _PROBE_SCHEME="http"
    echo "$response"
    return 0
  fi

  _PROBE_SCHEME=""
  return 1
}

# ── _probe_bridge_rpc(port, method, params_json) ────────────────────────────────
# Try HTTPS-first JSON-RPC call to Bridge.
# Sets _PROBE_SCHEME to "https" or "http".
# Prints the response body on stdout.
# Returns 0 on success, 1 on failure.
#
# Arguments:
#   $1 - Port number (e.g. 8080)
#   $2 - RPC method name (e.g. "status")
#   $3 - JSON params string (e.g. "{}")
_probe_bridge_rpc() {
  local port="${1:?Usage: _probe_bridge_rpc port method params}"
  local method="${2:?Usage: _probe_bridge_rpc port method params}"
  local params="${3:-{}}"
  local payload
  local response

  payload=$(jq -nc --arg m "$method" --argjson p "$params" \
    '{jsonrpc:"2.0", id:1, method:$m, params:$p}')

  # Try HTTPS first
  response=$(curl -sf -k -m 5 -X POST \
    -H "Content-Type: application/json" \
    -d "$payload" \
    "https://localhost:${port}/rpc" 2>/dev/null)
  if [[ $? -eq 0 && -n "$response" ]]; then
    _PROBE_SCHEME="https"
    echo "$response"
    return 0
  fi

  # Fallback to HTTP
  response=$(curl -sf -m 5 -X POST \
    -H "Content-Type: application/json" \
    -d "$payload" \
    "http://localhost:${port}/rpc" 2>/dev/null)
  if [[ $? -eq 0 && -n "$response" ]]; then
    _PROBE_SCHEME="http"
    echo "$response"
    return 0
  fi

  _PROBE_SCHEME=""
  return 1
}

# ── _probe_scheme_detect(host, port) ────────────────────────────────────────────
# Detect whether a service speaks HTTPS or HTTP by probing /health.
# Prints "https" or "http" on stdout.
# Returns 0 on success, 1 if neither works.
#
# Arguments:
#   $1 - Hostname or IP (e.g. "localhost")
#   $2 - Port number (e.g. 8080)
_probe_scheme_detect() {
  local host="${1:?Usage: _probe_scheme_detect host port}"
  local port="${2:?Usage: _probe_scheme_detect host port}"

  # Try HTTPS first
  if curl -sf -k -o /dev/null -m 5 "https://${host}:${port}/health" 2>/dev/null; then
    echo "https"
    return 0
  fi

  # Fallback to HTTP
  if curl -sf -o /dev/null -m 5 "http://${host}:${port}/health" 2>/dev/null; then
    echo "http"
    return 0
  fi

  return 1
}
