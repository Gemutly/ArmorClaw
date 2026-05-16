#!/usr/bin/env bash
# bridge.sh — ArmorClaw Bridge lifecycle management for Matrix E2E tests
#
# Uses: docker, curl, jq

# ── bridge_start <binary_path> <config_path> ───────────────────────────────────
# Starts the Bridge binary with the given config.
# Sets BRIDGE_PID on success.
bridge_start() {
  local binary="${1:-$BRIDGE_BIN}"
  local config="${2:-$BRIDGE_CONFIG}"

  if [[ -z "$binary" || ! -f "$binary" ]]; then
    echo -e "${RED}[bridge] Binary not found: $binary${NC}"
    return 1
  fi

  if [[ -z "$config" || ! -f "$config" ]]; then
    echo -e "${RED}[bridge] Config not found: $config${NC}"
    return 1
  fi

  echo -e "${YELLOW}[bridge] Starting with config: $config${NC}"

  mkdir -p "$KEYSTORE_DIR"

  ARMORCLAW_ERRORS_STORE_PATH="$KEYSTORE_DIR/errors.db" \
    ARMORCLAW_SKIP_DOCKER_CHECK=1 \
    "$binary" --config "$config" &
  BRIDGE_PID=$!

  echo -e "${GREEN}[bridge] Started (PID: $BRIDGE_PID)${NC}"
  return 0
}

# ── bridge_stop ────────────────────────────────────────────────────────────────
# Stops the Bridge process.
bridge_stop() {
  if [[ -n "${BRIDGE_PID:-}" ]]; then
    echo -e "${YELLOW}[bridge] Stopping PID $BRIDGE_PID${NC}"
    kill "$BRIDGE_PID" 2>/dev/null || true
    wait "$BRIDGE_PID" 2>/dev/null || true
    BRIDGE_PID=""
    echo -e "${GREEN}[bridge] Stopped${NC}"
    return 0
  fi

  if [[ -n "${BRIDGE_BIN:-}" && -f "$BRIDGE_BIN" ]]; then
    pkill -f "$BRIDGE_BIN" 2>/dev/null || true
    echo -e "${GREEN}[bridge] Killed remaining processes${NC}"
    return 0
  fi

  echo -e "${YELLOW}[bridge] No process to stop${NC}"
  return 0
}

# ── bridge_health [timeout_secs] ───────────────────────────────────────────────
# Polls Bridge health via Unix socket or TCP until ready.
bridge_health() {
  local timeout="${1:-30}"
  local count=0

  # TCP health check (for sentinel mode)
  if [[ "${BRIDGE_TRANSPORT:-unix}" == "tcp" ]]; then
    local addr="${BRIDGE_LISTEN_ADDR:-localhost:8443}"
    echo -e "${YELLOW}[bridge] Waiting for TCP health (${timeout}s): $addr${NC}"

    while [[ $count -lt $timeout ]]; do
      local code
      code=$(curl -s -o /dev/null -w "%{http_code}" "http://${addr}/health" 2>/dev/null || echo "000")
      if [[ "$code" == "200" ]]; then
        echo -e "${GREEN}[bridge] Healthy (TCP)${NC}"
        return 0
      fi
      sleep 1
      ((count++)) || true
    done

    echo -e "${RED}[bridge] Not healthy after ${timeout}s via TCP${NC}"
    return 1
  fi

  # Unix socket health check
  local socket="${BRIDGE_SOCKET:-/run/armorclaw/bridge.sock}"
  echo -e "${YELLOW}[bridge] Waiting for socket (${timeout}s): $socket${NC}"

  while [[ $count -lt $timeout ]]; do
    if [[ -S "$socket" ]]; then
      echo -e "${GREEN}[bridge] Healthy (socket)${NC}"
      return 0
    fi
    sleep 1
    ((count++)) || true
  done

  echo -e "${RED}[bridge] Socket not ready after ${timeout}s${NC}"
  return 1
}
