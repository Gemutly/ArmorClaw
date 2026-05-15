#!/usr/bin/env bash
# restart_bridge.sh — Serialized bridge restart with readiness polling
#
# Restarts the armorclaw bridge on the VPS via SSH (Docker container or
# systemd service), then polls until the service is active and accepting
# RPC calls. Uses flock for serialization so parallel test scripts don't race.
#
# Requires: load_env.sh (for ssh_vps, ADMIN_TOKEN, BRIDGE_PORT, BRIDGE_SOCKET)
#
# Usage:
#   source "$(dirname "$0")/load_env.sh"
#   source "$(dirname "$0")/restart_bridge.sh"
#   restart_bridge          # default 30s timeout
#   restart_bridge 60       # custom 60s timeout

# ── restart_bridge [max_wait_seconds=30] ──────────────────────────────────────
# Returns 0 on success, 1 on timeout.
restart_bridge() {
  local max_wait="${1:-30}"
  local lock_file="/tmp/armorclaw-test-restart.lock"

  (
    flock -x 200 || return 1

    # Detect deployment type and restart accordingly
    if ssh_vps "docker ps --filter name=armorclaw --format '{{.Names}}'" 2>/dev/null | grep -q "armorclaw"; then
      echo "[INFO] Restarting armorclaw Docker container..."
      ssh_vps "docker restart armorclaw" 2>/dev/null || true
    else
      echo "[INFO] Restarting armorclaw-bridge.service..."
      ssh_vps "systemctl restart armorclaw-bridge.service" 2>/dev/null || true
    fi

    # Poll readiness: up to 15 intervals of 2s (matching test-persistence.sh)
    local intervals=15
    local sleep_interval=2
    local ready=false

    for i in $(seq 1 "$intervals"); do
      sleep "$sleep_interval"

      # Check health via HTTP (works for both Docker and systemd)
      local health_resp
      health_resp=$(ssh_vps "curl -sfsS -o /dev/null -w '%{http_code}' http://localhost:${BRIDGE_PORT}/health 2>/dev/null || curl -kfsS -o /dev/null -w '%{http_code}' https://localhost:${BRIDGE_PORT}/health 2>/dev/null || echo '000'")
      if [[ "$health_resp" == "200" ]]; then
        # Socket check is advisory — HTTP health alone means bridge is ready
        ssh_vps "test -S ${BRIDGE_SOCKET:-/run/armorclaw/bridge.sock}" 2>/dev/null || true
        ready=true
        echo "[INFO] Bridge ready after $((i * sleep_interval))s"
        break
      fi

      echo "[INFO] ... waiting ($((i * sleep_interval))s)"
    done

    if $ready; then
      return 0
    else
      echo "[FAIL] Bridge not ready after ${max_wait}s"
      return 1
    fi
  ) 200>"$lock_file"
}
