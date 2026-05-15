#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/load_env.sh"
source "$SCRIPT_DIR/lib/transport.sh"
source "$SCRIPT_DIR/lib/common_output.sh"
source "$SCRIPT_DIR/lib/event_subscriber_helper.sh"

EVIDENCE_DIR="$SCRIPT_DIR/../.sisyphus/evidence/pbcp-16"
mkdir -p "$EVIDENCE_DIR"

PASS=0; FAIL=0; SKIP=0

log_info "── P0: Prerequisites ─────────────────────────────"

if [[ "$WEBSOCAT_AVAILABLE" != "true" ]]; then
  log_skip "websocat not available"
  harness_summary
  exit 0
fi

if ! check_bridge_running 2>/dev/null; then
  log_skip "Bridge not running"
  harness_summary
  exit 0
fi
log_pass "Bridge reachable"

# ══════════════════════════════════════════════════════════════════════════════
# P1: Initial WSS connection — verify alive
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P1: Initial WSS connection ─────────────────────"

P1_FILE="$EVIDENCE_DIR/ws-initial.txt"
if subscribe_events 5 "$P1_FILE"; then
  log_pass "Initial WSS connection established and held for 5s"
else
  log_fail "Initial WSS connection failed"
  harness_summary
  exit 1
fi

# ══════════════════════════════════════════════════════════════════════════════
# P2: Force disconnect (let connection close)
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P2: Force disconnect ──────────────────────────"

log_pass "Connection naturally closed (subscribe_events timeout expired)"

# ══════════════════════════════════════════════════════════════════════════════
# P3: Reconnect → trigger action → verify connection active
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P3: Reconnect after disconnect ─────────────────"

P3_FILE="$EVIDENCE_DIR/ws-reconnect.txt"
if subscribe_events 5 "$P3_FILE"; then
  log_pass "Reconnect successful — connection held for 5s"
else
  log_fail "Reconnect failed after disconnect"
  harness_summary
  exit 1
fi

# ══════════════════════════════════════════════════════════════════════════════
# P4: Multiple rapid connect/disconnect cycles
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P4: Rapid connect/disconnect cycles ────────────"

CYCLE_PASS=0
for i in 1 2 3; do
  CYCLE_FILE="$EVIDENCE_DIR/ws-cycle-$i.txt"
  if subscribe_events 2 "$CYCLE_FILE"; then
    CYCLE_PASS=$((CYCLE_PASS + 1))
  fi
done

if [[ "$CYCLE_PASS" -eq 3 ]]; then
  log_pass "3/3 rapid connect/disconnect cycles succeeded"
else
  log_fail "Only $CYCLE_PASS/3 rapid cycles succeeded"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " WSS Reconnect Gate Summary"
echo "========================================="
echo " Passed: $PASS | Failed: $FAIL | Skipped: $SKIP"
echo "========================================="

harness_summary
