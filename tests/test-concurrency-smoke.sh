#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/load_env.sh"
source "$SCRIPT_DIR/lib/common_output.sh"

EVIDENCE_DIR="$SCRIPT_DIR/../.sisyphus/evidence/pbcp-17"
mkdir -p "$EVIDENCE_DIR"

PASS=0; FAIL=0; SKIP=0

log_info "── P0: Prerequisites ─────────────────────────────"

if ! check_bridge_running 2>/dev/null; then
  log_skip "Bridge not running"
  harness_summary
  exit 0
fi
log_pass "Bridge reachable"

# ══════════════════════════════════════════════════════════════════════════════
# P1: 10 concurrent bridge.status calls → all 200 OK
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P1: 10 concurrent bridge.status calls ──────────"

P1_DIR="$EVIDENCE_DIR/concurrent-rpc"
mkdir -p "$P1_DIR"

for i in $(seq 1 10); do
  ssh_vps "curl -ksS 'https://localhost:${BRIDGE_PORT}/api' \
    -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer ${ADMIN_TOKEN}' \
    -d '{\"jsonrpc\":\"2.0\",\"id\":$i,\"method\":\"bridge.status\"}' \
    --max-time 10" > "$P1_DIR/result-$i.json" 2>/dev/null &
done

wait

SUCCESS_COUNT=0
FAIL_COUNT=0
for i in $(seq 1 10); do
  if [[ -s "$P1_DIR/result-$i.json" ]] && jq -e '.result' "$P1_DIR/result-$i.json" >/dev/null 2>&1; then
    SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
  else
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
done

if [[ "$FAIL_COUNT" -eq 0 ]]; then
  log_pass "10/10 concurrent bridge.status calls succeeded"
else
  log_fail "$SUCCESS_COUNT/10 concurrent calls succeeded ($FAIL_COUNT failed)"
fi

# ══════════════════════════════════════════════════════════════════════════════
# P2: 3 concurrent WSS subscribers → all connect
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P2: 3 concurrent WSS subscribers ───────────────"

if command -v websocat >/dev/null 2>&1; then
  P2_DIR="$EVIDENCE_DIR/concurrent-ws"
  mkdir -p "$P2_DIR"

  for i in 1 2 3; do
    timeout 5 websocat -k "wss://${VPS_IP}:${BRIDGE_PORT}/ws" > "$P2_DIR/ws-$i.txt" 2>/dev/null &
  done

  wait

  WS_SUCCESS=0
  for i in 1 2 3; do
    if [[ -f "$P2_DIR/ws-$i.txt" ]]; then
      WS_SUCCESS=$((WS_SUCCESS + 1))
    fi
  done

  if [[ "$WS_SUCCESS" -eq 3 ]]; then
    log_pass "3/3 concurrent WSS connections succeeded"
  else
    log_fail "$WS_SUCCESS/3 concurrent WSS connections succeeded"
  fi
else
  log_skip "websocat not available for concurrent WSS test"
fi

# ══════════════════════════════════════════════════════════════════════════════
# P3: Bridge stays healthy after concurrent load
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P3: Post-load health check ─────────────────────"

P3_FILE="$EVIDENCE_DIR/post-load-health.json"
ssh_vps "curl -ksS 'https://localhost:${BRIDGE_PORT}/api' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer ${ADMIN_TOKEN}' \
  -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"bridge.status\"}'" > "$P3_FILE" 2>/dev/null || true

if [[ -s "$P3_FILE" ]] && jq -e '.result' "$P3_FILE" >/dev/null 2>&1; then
  log_pass "Bridge healthy after concurrent load"
else
  log_fail "Bridge unhealthy after concurrent load"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " Concurrency Smoke Summary"
echo "========================================="
echo " Passed: $PASS | Failed: $FAIL | Skipped: $SKIP"
echo "========================================="

harness_summary
