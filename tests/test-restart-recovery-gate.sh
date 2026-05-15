#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/load_env.sh"
source "$SCRIPT_DIR/lib/common_output.sh"

EVIDENCE_DIR="$SCRIPT_DIR/../.sisyphus/evidence/pbcp-15"
mkdir -p "$EVIDENCE_DIR"

PASS=0; FAIL=0; SKIP=0

rpc() {
  local method="$1"
  local params="${2:-{\}}"
  ssh_vps "curl -ksS 'https://localhost:${BRIDGE_PORT}/api' \
    -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer ${ADMIN_TOKEN}' \
    -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"${method}\",\"params\":${params}}'" 2>/dev/null
}

log_info "── P0: Prerequisites ─────────────────────────────"

if ! check_bridge_running 2>/dev/null; then
  log_skip "Bridge not running"
  harness_summary
  exit 0
fi
log_pass "Bridge reachable"

# ══════════════════════════════════════════════════════════════════════════════
# P1: Record pre-restart state
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P1: Pre-restart state ─────────────────────────"

PRE_FILE="$EVIDENCE_DIR/pre-restart-status.json"
rpc "matrix.status" > "$PRE_FILE" 2>/dev/null || true

if [[ -s "$PRE_FILE" ]] && jq -e '.result' "$PRE_FILE" >/dev/null 2>&1; then
  pre_login=$(jq -r '.result.logged_in // false' "$PRE_FILE" 2>/dev/null)
  pre_connected=$(jq -r '.result.connected // false' "$PRE_FILE" 2>/dev/null)
  log_pass "Pre-restart matrix.status: logged_in=$pre_login, connected=$pre_connected"
else
  log_skip "Pre-restart matrix.status unavailable (will still test restart)"
fi

# ══════════════════════════════════════════════════════════════════════════════
# P2: Restart bridge container
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P2: Restarting bridge ─────────────────────────"

RESTART_LOG="$EVIDENCE_DIR/restart.log"
START_EPOCH=$(date +%s)

ssh_vps "docker restart armorclaw" > "$RESTART_LOG" 2>&1 || true
log_pass "docker restart issued"

# ══════════════════════════════════════════════════════════════════════════════
# P3: Poll /health until 200 OK (timeout 30s, target ≤10s)
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P3: Health recovery polling ────────────────────"

HEALTH_OK=false
RECOVERY_TIME=""

for i in $(seq 1 15); do
  sleep 2
  NOW_EPOCH=$(date +%s)
  ELAPSED=$(( NOW_EPOCH - START_EPOCH ))

  health_check=$(ssh_vps "curl -ksSf 'https://localhost:${BRIDGE_PORT}/health' --max-time 2" 2>/dev/null || true)
  if [[ -n "$health_check" ]]; then
    HEALTH_OK=true
    RECOVERY_TIME="${ELAPSED}s"
    break
  fi
done

if $HEALTH_OK; then
  if [[ "$ELAPSED" -le 10 ]]; then
    log_pass "Health recovered in $RECOVERY_TIME (≤10s target)"
  else
    log_fail "Health recovered in $RECOVERY_TIME (>10s target)"
  fi
else
  log_fail "Health did not recover within 30s"
  harness_summary
  exit 1
fi

# ══════════════════════════════════════════════════════════════════════════════
# P4: Verify matrix.status → logged_in + connected
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P4: Matrix reconnect after restart ─────────────"

sleep 3

POST_FILE="$EVIDENCE_DIR/post-restart-status.json"
rpc "matrix.status" > "$POST_FILE" 2>/dev/null || true

if [[ -s "$POST_FILE" ]] && jq -e '.result' "$POST_FILE" >/dev/null 2>&1; then
  post_login=$(jq -r '.result.logged_in // false' "$POST_FILE" 2>/dev/null)
  post_connected=$(jq -r '.result.connected // false' "$POST_FILE" 2>/dev/null)
  if [[ "$post_login" == "true" && "$post_connected" == "true" ]]; then
    log_pass "Post-restart matrix.status: logged_in=$post_login, connected=$post_connected"
  else
    log_fail "Post-restart matrix.status: logged_in=$post_login, connected=$post_connected (expected both true)"
  fi
else
  log_fail "Post-restart matrix.status unavailable"
fi

# ══════════════════════════════════════════════════════════════════════════════
# P5: Verify RPC still works
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P5: RPC post-restart ──────────────────────────"

P5_FILE="$EVIDENCE_DIR/post-restart-rpc.json"
rpc "bridge.status" > "$P5_FILE" 2>/dev/null || true

if [[ -s "$P5_FILE" ]] && jq -e '.result' "$P5_FILE" >/dev/null 2>&1; then
  log_pass "RPC works post-restart: $(jq -c '.result | {status}' "$P5_FILE" 2>/dev/null)"
else
  log_fail "RPC not working post-restart"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " Restart Recovery Gate Summary"
echo "========================================="
echo " Passed: $PASS | Failed: $FAIL | Skipped: $SKIP"
echo " Recovery time: ${RECOVERY_TIME:-N/A}"
echo "========================================="

harness_summary
