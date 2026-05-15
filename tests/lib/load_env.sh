#!/usr/bin/env bash
# load_env.sh — Shared environment loader for ArmorClaw test scripts
#
# Sources .env for VPS connection details, exports key variables with
# sensible defaults, sources tests/e2e/common.sh (so callers get
# rpc_call, log_result, etc.), and provides ssh_vps() and
# check_bridge_running() helpers.
#
# Usage:
#   source "$(dirname "$0")/../lib/load_env.sh"
#   # or from tests/ root:
#   source "tests/lib/load_env.sh"

# ── Source .env (matching test-vps-smoke.sh pattern) ──────────────────────────
set -a
# Use _REPO_ROOT when sourced from scripts/lib/contract.sh, fall back to $0-relative
_ROOT="${_REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")/../.." && pwd)}"
source "${_ROOT}/.env" 2>/dev/null || true
source .env 2>/dev/null || true
set +a

# ── Environment variables with defaults ───────────────────────────────────────
: "${VPS_IP:?VPS_IP is required — set in .env or export manually}"
: "${VPS_USER:=root}"
: "${BRIDGE_PORT:=8080}"
: "${MATRIX_PORT:=6167}"
: "${SSH_KEY_PATH:=~/.ssh/openclaw_win}"
# ADMIN_TOKEN may be empty — tests that need it should check and skip gracefully
export ADMIN_TOKEN="${ADMIN_TOKEN:-}"

export VPS_IP VPS_USER BRIDGE_PORT MATRIX_PORT SSH_KEY_PATH

: "${BRIDGE_SOCKET:=/run/armorclaw/bridge.sock}"
: "${BRIDGE_TRANSPORT:=}"  # auto-detect if empty; set to socket|http|both|none to override
export BRIDGE_SOCKET BRIDGE_TRANSPORT

# ── Source common.sh AFTER .env so its defaults don't override ────────────────
COMMON_SH="${_ROOT}/tests/e2e/common.sh"
if [[ -f "$COMMON_SH" ]]; then
  source "$COMMON_SH"
else
  echo "[WARN] tests/e2e/common.sh not found at $COMMON_SH — skipping"
fi

# ── ssh_vps helper ────────────────────────────────────────────────────────────
# Runs a command on the VPS via SSH. Mirrors the pattern from test-persistence.sh.
#
# Usage:
#   ssh_vps "systemctl status armorclaw-bridge"
#   ssh_vps "ls /run/armorclaw/"
ssh_vps() {
  ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=no -o ConnectTimeout=10 "${VPS_USER}@${VPS_IP}" "$@"
}

# ── check_bridge_running helper ───────────────────────────────────────────────
# Returns 0 if the bridge is running (Docker container OR systemd service),
# 1 otherwise.  Checks in order:
#   1. Docker container "armorclaw" is Up
#   2. systemd service armorclaw-bridge.service is active
#   3. Socket file exists at $BRIDGE_SOCKET
check_bridge_running() {
  # 1. Check Docker container
  local docker_status
  docker_status=$(ssh_vps "docker ps --filter name=armorclaw --format '{{.Status}}'" 2>/dev/null || echo "")
  if [[ "$docker_status" == *"Up"* ]]; then
    return 0
  fi

  # 2. Check systemd service
  local svc_status
  svc_status=$(ssh_vps "systemctl is-active armorclaw-bridge.service" 2>/dev/null || echo "")
  if [[ "$svc_status" == "active" ]]; then
    return 0
  fi

  # 3. Check socket file exists
  if ssh_vps "test -S ${BRIDGE_SOCKET}" 2>/dev/null; then
    return 0
  fi

  return 1
}
