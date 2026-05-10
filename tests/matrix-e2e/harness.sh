#!/usr/bin/env bash
# harness.sh — Conduit lifecycle harness for Matrix E2E tests
#
# Orchestrates Conduit + Bridge startup/shutdown for test suites.
# Usage: source this file, then call harness_start / harness_stop / harness_status.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

source "$SCRIPT_DIR/lib/conduit.sh"
source "$SCRIPT_DIR/lib/bridge.sh"
source "$SCRIPT_DIR/lib/assertions.sh"
source "$SCRIPT_DIR/../e2e/common.sh"

# Exported defaults
export CONDUIT_PORT="${CONDUIT_PORT:-6167}"
export CONDUIT_URL="${CONDUIT_URL:-http://localhost:${CONDUIT_PORT}}"
export CONDUIT_SERVER_NAME="${CONDUIT_SERVER_NAME:-armorclaw.test}"
export CONDUIT_REGISTRATION_SECRET="${CONDUIT_REGISTRATION_SECRET:-test-registration-secret-change-in-production}"
export CONDUIT_CONFIG_DIR="${CONDUIT_CONFIG_DIR:-$SCRIPT_DIR/fixtures}"

export BRIDGE_SOCKET="${BRIDGE_SOCKET:-/tmp/bridge-e2e-$$.sock}"
export BRIDGE_CONFIG="${BRIDGE_CONFIG:-$SCRIPT_DIR/fixtures/test-config.toml}"
export BRIDGE_BIN="${BRIDGE_BIN:-}"
export KEYSTORE_DIR="${KEYSTORE_DIR:-/tmp/armorclaw-keystore-e2e-$$}"

# ── harness_start ─────────────────────────────────────────────────────────────
# Starts Conduit, waits for health, then starts Bridge.
harness_start() {
  echo -e "${YELLOW}═══ Starting Conduit Lifecycle Harness ═══${NC}"

  conduit_start || return 1
  conduit_health 30 || { conduit_stop; return 1; }

  if [[ -n "${BRIDGE_BIN:-}" ]]; then
    bridge_start "$BRIDGE_BIN" "$BRIDGE_CONFIG" || { conduit_stop; return 1; }
    bridge_health 30 || { bridge_stop; conduit_stop; return 1; }
  fi

  echo -e "${GREEN}═══ Harness Ready ═══${NC}"
  return 0
}

# ── harness_stop ──────────────────────────────────────────────────────────────
# Stops Bridge then Conduit in reverse order.
harness_stop() {
  echo -e "${YELLOW}═══ Stopping Conduit Lifecycle Harness ═══${NC}"
  bridge_stop
  conduit_stop
  rm -f "$BRIDGE_SOCKET" 2>/dev/null || true
  rm -rf "$KEYSTORE_DIR" 2>/dev/null || true
  echo -e "${GREEN}═══ Harness Stopped ═══${NC}"
  return 0
}

# ── harness_status ────────────────────────────────────────────────────────────
# Prints status of all harness components.
harness_status() {
  echo "═══ Harness Status ═══"
  echo "  Conduit container: ${CONDUIT_CONTAINER:-not started}"
  echo "  Conduit URL:       $CONDUIT_URL"
  echo "  Bridge PID:        ${BRIDGE_PID:-not started}"
  echo "  Bridge socket:     $BRIDGE_SOCKET"
  echo "  Keystore dir:      $KEYSTORE_DIR"

  if [[ -n "${CONDUIT_CONTAINER:-}" ]]; then
    local conduit_up
    conduit_up=$(docker ps -q -f "id=$CONDUIT_CONTAINER" 2>/dev/null || true)
    if [[ -n "$conduit_up" ]]; then
      echo -e "  Conduit health:    ${GREEN}running${NC}"
    else
      echo -e "  Conduit health:    ${RED}stopped${NC}"
    fi
  fi

  if [[ -n "${BRIDGE_PID:-}" ]] && kill -0 "$BRIDGE_PID" 2>/dev/null; then
    echo -e "  Bridge health:     ${GREEN}running${NC}"
  elif [[ -n "${BRIDGE_PID:-}" ]]; then
    echo -e "  Bridge health:     ${RED}exited${NC}"
  fi
}
