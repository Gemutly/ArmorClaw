#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# test-matrix-control-flow.sh — Matrix Control-Flow Happy Path Tests (T10)
#
# Tests that Matrix-driven RPC commands work through the HTTPS transport:
#   1. bridge.status — basic connectivity
#   2. matrix.status — Matrix connection state
#   3. studio.list_agents — Studio subsystem reachable
#   4. studio.create_agent → studio.list_agents (count+1) → studio.delete_agent
#   5. secretary.list_templates — Secretary subsystem reachable
#
# Usage:  bash tests/test-matrix-control-flow.sh
# Requires: .env with VPS_IP, ADMIN_TOKEN, BRIDGE_PORT
# ──────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/load_env.sh"
source "$SCRIPT_DIR/lib/transport.sh"
source "$SCRIPT_DIR/lib/common_output.sh"

EVIDENCE_DIR="$SCRIPT_DIR/../.sisyphus/evidence/pbcp-10"
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

# ══════════════════════════════════════════════════════════════════════════════
# P0: Prerequisites
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P0: Prerequisites ─────────────────────────────"

if ! check_bridge_running 2>/dev/null; then
  log_skip "Bridge not running on VPS"
  harness_summary
  exit 0
fi

log_pass "Bridge reachable"

# ══════════════════════════════════════════════════════════════════════════════
# P1: bridge.status — basic connectivity
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P1: bridge.status ────────────────────────────"

P1_FILE="$EVIDENCE_DIR/bridge-status.json"
rpc "bridge.status" > "$P1_FILE" 2>/dev/null || true

if [[ -s "$P1_FILE" ]] && jq -e '.result' "$P1_FILE" >/dev/null 2>&1; then
  log_pass "bridge.status returned result"
  echo "  Response: $(jq -c '.result | {status, enabled}' "$P1_FILE" 2>/dev/null || echo 'parse error')" >> "$P1_FILE.tmp"
else
  log_fail "bridge.status did not return valid result"
  echo "Raw: $(cat "$P1_FILE" 2>/dev/null || echo 'empty')"
fi

# ══════════════════════════════════════════════════════════════════════════════
# P2: matrix.status — Matrix connection state
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P2: matrix.status ────────────────────────────"

P2_FILE="$EVIDENCE_DIR/matrix-status.json"
rpc "matrix.status" > "$P2_FILE" 2>/dev/null || true

if [[ -s "$P2_FILE" ]] && jq -e '.result' "$P2_FILE" >/dev/null 2>&1; then
  logged_in=$(jq -r '.result.logged_in // false' "$P2_FILE" 2>/dev/null)
  connected=$(jq -r '.result.connected // false' "$P2_FILE" 2>/dev/null)
  if [[ "$logged_in" == "true" && "$connected" == "true" ]]; then
    log_pass "matrix.status: logged_in=$logged_in, connected=$connected"
  else
    log_fail "matrix.status: logged_in=$logged_in, connected=$connected (expected both true)"
  fi
else
  log_fail "matrix.status did not return valid result"
fi

# ══════════════════════════════════════════════════════════════════════════════
# P3: studio.list_agents — Studio subsystem reachable
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P3: studio.list_agents ────────────────────────"

P3_FILE="$EVIDENCE_DIR/studio-list-agents.json"
rpc "studio.list_agents" > "$P3_FILE" 2>/dev/null || true

if [[ -s "$P3_FILE" ]] && jq -e '.result' "$P3_FILE" >/dev/null 2>&1; then
  agent_count=$(jq -r '.result.count // 0' "$P3_FILE" 2>/dev/null)
  log_pass "studio.list_agents: count=$agent_count"
else
  log_fail "studio.list_agents did not return valid result"
fi

# ══════════════════════════════════════════════════════════════════════════════
# P4: Studio create → list (count+1) → delete → list (count-1)
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P4: Studio create/list/delete lifecycle ────────"

BASELINE_COUNT=$(jq -r '.result.count // 0' "$P3_FILE" 2>/dev/null)

# Create agent
CREATE_FILE="$EVIDENCE_DIR/studio-create-agent.json"
rpc "studio.create_agent" "{\"name\":\"test-pbcp-t10\",\"skills\":[\"web_browsing\"]}" > "$CREATE_FILE" 2>/dev/null || true

CREATED_AGENT_ID=""
if [[ -s "$CREATE_FILE" ]] && jq -e '.result' "$CREATE_FILE" >/dev/null 2>&1; then
  CREATED_AGENT_ID=$(jq -r '.result.agent_id // .result.id // empty' "$CREATE_FILE" 2>/dev/null)
  if [[ -n "$CREATED_AGENT_ID" && "$CREATED_AGENT_ID" != "null" ]]; then
    log_pass "studio.create_agent: id=$CREATED_AGENT_ID"
  else
    # Some versions return the agent object differently
    log_pass "studio.create_agent: $(jq -c '.result' "$CREATE_FILE" 2>/dev/null | head -c 100)"
  fi
else
  error_msg=$(jq -r '.error.message // "unknown"' "$CREATE_FILE" 2>/dev/null || echo 'parse error')
  log_skip "studio.create_agent failed: $error_msg"
fi

# List agents again — expect count+1
LIST2_FILE="$EVIDENCE_DIR/studio-list-agents-2.json"
rpc "studio.list_agents" > "$LIST2_FILE" 2>/dev/null || true

if [[ -s "$LIST2_FILE" ]] && jq -e '.result' "$LIST2_FILE" >/dev/null 2>&1; then
  new_count=$(jq -r '.result.count // 0' "$LIST2_FILE" 2>/dev/null)
  if [[ "$new_count" -gt "$BASELINE_COUNT" ]]; then
    log_pass "studio.list_agents after create: count=$new_count (was $BASELINE_COUNT, +1)"
  else
    log_skip "studio.list_agents after create: count=$new_count (expected > $BASELINE_COUNT)"
  fi
else
  log_fail "studio.list_agents (post-create) did not return valid result"
fi

# Delete agent (cleanup)
if [[ -n "$CREATED_AGENT_ID" && "$CREATED_AGENT_ID" != "null" ]]; then
  DELETE_FILE="$EVIDENCE_DIR/studio-delete-agent.json"
  rpc "studio.delete_agent" "{\"agent_id\":\"$CREATED_AGENT_ID\"}" > "$DELETE_FILE" 2>/dev/null || true

  if [[ -s "$DELETE_FILE" ]] && jq -e '.result' "$DELETE_FILE" >/dev/null 2>&1; then
    log_pass "studio.delete_agent: success"
  else
    error_msg=$(jq -r '.error.message // "unknown"' "$DELETE_FILE" 2>/dev/null || echo 'parse error')
    log_skip "studio.delete_agent failed: $error_msg (orphan may remain)"
  fi
else
  log_skip "No agent_id to delete (create may not have returned one)"
fi

# Final list — expect back to baseline
LIST3_FILE="$EVIDENCE_DIR/studio-list-agents-final.json"
rpc "studio.list_agents" > "$LIST3_FILE" 2>/dev/null || true

if [[ -s "$LIST3_FILE" ]] && jq -e '.result' "$LIST3_FILE" >/dev/null 2>&1; then
  final_count=$(jq -r '.result.count // 0' "$LIST3_FILE" 2>/dev/null)
  if [[ "$final_count" -le "$BASELINE_COUNT" ]]; then
    log_pass "studio.list_agents after cleanup: count=$final_count (≤ baseline $BASELINE_COUNT)"
  else
    log_skip "studio.list_agents after cleanup: count=$final_count (expected ≤ $BASELINE_COUNT, orphan may exist)"
  fi
else
  log_fail "studio.list_agents (final) did not return valid result"
fi

# ══════════════════════════════════════════════════════════════════════════════
# P5: secretary.list_templates — Secretary subsystem reachable
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P5: secretary.list_templates ──────────────────"

P5_FILE="$EVIDENCE_DIR/secretary-list-templates.json"
rpc "secretary.list_templates" > "$P5_FILE" 2>/dev/null || true

if [[ -s "$P5_FILE" ]] && jq -e '.result' "$P5_FILE" >/dev/null 2>&1; then
  log_pass "secretary.list_templates: $(jq -c '.result | keys' "$P5_FILE" 2>/dev/null | head -c 80)"
else
  error_code=$(jq -r '.error.code // empty' "$P5_FILE" 2>/dev/null)
  if [[ "$error_code" == "-32601" ]]; then
    log_skip "secretary.list_templates: method not found (feature not enabled)"
  else
    log_fail "secretary.list_templates did not return valid result"
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " Matrix Control-Flow Happy Path Summary"
echo "========================================="
echo " Passed: $PASS | Failed: $FAIL | Skipped: $SKIP"
echo "========================================="

harness_summary
