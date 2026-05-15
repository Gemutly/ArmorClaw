#!/usr/bin/env bash
# transport.sh — Docker-aware bridge transport detector for ArmorClaw test scripts
#
# Detects whether the ArmorClaw Bridge is reachable via Unix socket, HTTP, or both.
# Provides rpc_call() / rpc_call_auth() that auto-select the correct transport.
#
# Usage:
#   source tests/lib/transport.sh
#   detect_transport          # sets TRANSPORT_MODE, HAS_SOCKET, HAS_HTTP
#   rpc_call status           # auto-selects socket or HTTP
#   require_bridge            # exits if no transport available
#   optional_bridge           # returns 1 but does NOT exit
#
# Environment overrides:
#   BRIDGE_TRANSPORT  — force "socket"|"http"|"both"|"none" (skip detection)
#   BRIDGE_SOCKET     — Unix socket path (default: /run/armorclaw/bridge.sock)
#   BRIDGE_PORT       — HTTP port (default: 8080)
#   ADMIN_TOKEN       — auth token for rpc_call_auth()
#
# Requires: socat, curl (already in CI image)

# ── Defaults ──────────────────────────────────────────────────────────────────
export BRIDGE_SOCKET="${BRIDGE_SOCKET:-/run/armorclaw/bridge.sock}"
export BRIDGE_PORT="${BRIDGE_PORT:-8080}"
export BRIDGE_HTTP_URL="${BRIDGE_HTTP_URL:-http://localhost:${BRIDGE_PORT}}"
export BRIDGE_HTTPS_URL="${BRIDGE_HTTPS_URL:-https://localhost:${BRIDGE_PORT}}"
export BRIDGE_HTTPS_MODE="${BRIDGE_HTTPS_MODE:-auto}"  # auto|https|http
export BRIDGE_CURL_INSECURE="${BRIDGE_CURL_INSECURE:-}"
export TRANSPORT_MODE="${TRANSPORT_MODE:-}"
export HAS_SOCKET=false
export HAS_HTTP=false
export RUNNING_IN_DOCKER=false
export BRIDGE_AVAILABLE=false
_TRANSPORT_DETECTED=false

# ── detect_transport ──────────────────────────────────────────────────────────
# Detects bridge transport availability.  Sets TRANSPORT_MODE, HAS_SOCKET,
# HAS_HTTP, RUNNING_IN_DOCKER.  Prints detected mode on first call only.
detect_transport() {
  # Skip re-detection if already run (unless forced via BRIDGE_TRANSPORT)
  if [[ "$_TRANSPORT_DETECTED" == true && -z "$BRIDGE_TRANSPORT" ]]; then
    return 0
  fi

  HAS_SOCKET=false
  HAS_HTTP=false
  RUNNING_IN_DOCKER=false

  # 1. Explicit env override
  if [[ -n "${BRIDGE_TRANSPORT:-}" ]]; then
    TRANSPORT_MODE="$BRIDGE_TRANSPORT"
    case "$TRANSPORT_MODE" in
      socket|http|both|none) ;;
      *) echo "[WARN] transport: invalid BRIDGE_TRANSPORT='$TRANSPORT_MODE' (expected socket|http|both|none)" ;;
    esac
    _TRANSPORT_DETECTED=true
    echo "[INFO] transport: BRIDGE_TRANSPORT override=$TRANSPORT_MODE"
    return 0
  fi

  # 2. Running inside Docker?
  if test -f /.dockerenv; then
    RUNNING_IN_DOCKER=true
  fi

  # 3. Socket check
  if test -S "$BRIDGE_SOCKET"; then
    HAS_SOCKET=true
  fi

  # 4. Docker health (advisory only — container may be up but transport differs)
  local _docker_up=false
  if command -v docker &>/dev/null; then
    local _docker_status
    _docker_status=$(docker ps --filter name=armorclaw --format '{{.Status}}' 2>/dev/null || true)
    if [[ "$_docker_status" == *"Up"* || "$_docker_status" == *"healthy"* ]]; then
      _docker_up=true
    fi
  fi

  # 5. HTTPS health (try before HTTP if mode is auto or https)
  if [[ "$BRIDGE_HTTPS_MODE" != "http" ]]; then
    if curl -ksSf "${BRIDGE_HTTPS_URL}/health" &>/dev/null; then
      HAS_HTTP=true
      BRIDGE_HTTP_URL="${BRIDGE_HTTPS_URL}"
      BRIDGE_CURL_INSECURE="-k"
    fi
  fi

  # 6. HTTP health (fallback, only if HTTPS didn't work)
  if ! $HAS_HTTP && [[ "$BRIDGE_HTTPS_MODE" != "https" ]]; then
    if curl -sf "${BRIDGE_HTTP_URL}/health" &>/dev/null; then
      HAS_HTTP=true
    fi
  fi

  # 7. Determine mode
  if $HAS_SOCKET && $HAS_HTTP; then
    TRANSPORT_MODE="both"
  elif $HAS_SOCKET; then
    TRANSPORT_MODE="socket"
  elif $HAS_HTTP; then
    TRANSPORT_MODE="http"
  else
    TRANSPORT_MODE="none"
  fi

  _TRANSPORT_DETECTED=true

  # 8. Print detected mode on first call
  echo "[INFO] transport: mode=$TRANSPORT_MODE socket=$HAS_SOCKET http=$HAS_HTTP docker=$_docker_up in_docker=$RUNNING_IN_DOCKER"
}

# ── rpc_call <method> [params_json] [timeout] ─────────────────────────────────
# Auto-selects socket or HTTP based on detected transport.
# Returns raw JSON-RPC response on stdout.
rpc_call() {
  local method="$1"
  local params="${2:-{\}}"
  local timeout="${3:-5}"

  if [[ -z "$method" ]]; then
    echo "[ERROR] transport: rpc_call requires a method argument" >&2
    return 1
  fi

  detect_transport

  if [[ "$TRANSPORT_MODE" == "none" ]]; then
    echo "[ERROR] transport: no bridge transport available for rpc_call" >&2
    return 1
  fi

  # Prefer socket when available
  if [[ "$TRANSPORT_MODE" == "socket" || "$TRANSPORT_MODE" == "both" ]] && $HAS_SOCKET; then
    if command -v socat &>/dev/null; then
      local response
      response=$(timeout "$timeout" bash -c \
        "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$method\",\"params\":$params}' | \
         socat - UNIX-CONNECT:$BRIDGE_SOCKET" 2>/dev/null || true)
      if [[ -n "$response" ]]; then
        echo "$response"
        return 0
      fi
      # Socket failed — fall through to HTTP if available
      if ! $HAS_HTTP; then
        return 1
      fi
    fi
  fi

  # HTTP transport
  if [[ "$TRANSPORT_MODE" == "http" || "$TRANSPORT_MODE" == "both" ]] && $HAS_HTTP; then
    curl ${BRIDGE_CURL_INSECURE} -sf "${BRIDGE_HTTP_URL}/api" \
      -H "Content-Type: application/json" \
      -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$method\",\"params\":$params}" \
      --max-time "$timeout" 2>/dev/null
    return $?
  fi

  return 1
}

# ── rpc_call_auth <method> [params_json] [timeout] ────────────────────────────
# Same as rpc_call but injects ADMIN_TOKEN auth per transport:
#   socket → "auth":"$ADMIN_TOKEN" in JSON-RPC params
#   HTTP   → Authorization: Bearer $ADMIN_TOKEN header
rpc_call_auth() {
  local method="$1"
  local params="${2:-{\}}"
  local timeout="${3:-5}"

  if [[ -z "${ADMIN_TOKEN:-}" ]]; then
    echo "[WARN] transport: rpc_call_auth called but ADMIN_TOKEN is empty" >&2
    return 1
  fi

  if [[ -z "$method" ]]; then
    echo "[ERROR] transport: rpc_call_auth requires a method argument" >&2
    return 1
  fi

  detect_transport

  if [[ "$TRANSPORT_MODE" == "none" ]]; then
    echo "[ERROR] transport: no bridge transport available for rpc_call_auth" >&2
    return 1
  fi

  # Prefer socket when available
  if [[ "$TRANSPORT_MODE" == "socket" || "$TRANSPORT_MODE" == "both" ]] && $HAS_SOCKET; then
    if command -v socat &>/dev/null; then
      local response
      response=$(timeout "$timeout" bash -c \
        "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$method\",\"params\":$params,\"auth\":\"$ADMIN_TOKEN\"}' | \
         socat - UNIX-CONNECT:$BRIDGE_SOCKET" 2>/dev/null || true)
      if [[ -n "$response" ]]; then
        echo "$response"
        return 0
      fi
      if ! $HAS_HTTP; then
        return 1
      fi
    fi
  fi

  # HTTP transport
  if [[ "$TRANSPORT_MODE" == "http" || "$TRANSPORT_MODE" == "both" ]] && $HAS_HTTP; then
    curl ${BRIDGE_CURL_INSECURE} -sf "${BRIDGE_HTTP_URL}/api" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $ADMIN_TOKEN" \
      -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$method\",\"params\":$params}" \
      --max-time "$timeout" 2>/dev/null
    return $?
  fi

  return 1
}

# ── require_bridge ────────────────────────────────────────────────────────────
# Calls detect_transport; exits 1 if TRANSPORT_MODE == "none".
require_bridge() {
  detect_transport
  if [[ "$TRANSPORT_MODE" == "none" ]]; then
    echo "[ERROR] transport: bridge not available (ENV_MISSING). Set BRIDGE_TRANSPORT or start the bridge." >&2
    exit 1
  fi
  BRIDGE_AVAILABLE=true
  return 0
}

# ── optional_bridge ───────────────────────────────────────────────────────────
# Calls detect_transport; returns 1 if no transport but does NOT exit.
# Caller decides what to do (skip, mock, etc).
optional_bridge() {
  detect_transport
  if [[ "$TRANSPORT_MODE" == "none" ]]; then
    BRIDGE_AVAILABLE=false
    return 1
  fi
  BRIDGE_AVAILABLE=true
  return 0
}

# ── Export all functions ──────────────────────────────────────────────────────
export -f detect_transport
export -f rpc_call
export -f rpc_call_auth
export -f require_bridge
export -f optional_bridge
