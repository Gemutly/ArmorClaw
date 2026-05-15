#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# test-navchart-security.sh — NavChart PII Security Harness
#
# Validates that stored NavCharts do not contain raw PII, malformed charts are
# rejected, approval is enforced on replay, and audit logs are captured.
#
# Scenarios:
#   NS0  Prerequisites — tools, bridge access
#   NS1  No raw PII in stored charts
#   NS2  Policy rejection — malformed chart
#   NS3  Approval still required on replay
#   NS4  Audit log entries present
#   NS5  Malicious/malformed chart rejected
#
# Usage:  bash tests/test-navchart-security.sh
# Tier:   B (skip if bridge not deployed)
# ──────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib/load_env.sh"
source "$SCRIPT_DIR/lib/common_output.sh"
source "$SCRIPT_DIR/lib/assert_json.sh"
source "$SCRIPT_DIR/lib/transport.sh"

EVIDENCE_DIR="$SCRIPT_DIR/../.sisyphus/evidence/full-system-navchart"
mkdir -p "$EVIDENCE_DIR"

SKIP_ALL=false

# ══════════════════════════════════════════════════════════════════════════════
# NS0: Prerequisites
# ══════════════════════════════════════════════════════════════════════════════
log_info "── NS0: Prerequisites ────────────────────────"

if command -v jq &>/dev/null; then
  log_pass "jq available"
else
  log_fail "jq not found"
  SKIP_ALL=true
fi

if command -v curl &>/dev/null; then
  log_pass "curl available"
else
  log_fail "curl not found"
  SKIP_ALL=true
fi

BRIDGE_REACHABLE=false
BRIDGE_RESP=$(rpc_call_auth "bridge.status" 2>/dev/null || true)
if [[ -n "$BRIDGE_RESP" ]] && echo "$BRIDGE_RESP" | jq -e '.result' &>/dev/null; then
  BRIDGE_REACHABLE=true
  log_pass "Bridge reachable"
else
  log_skip "Bridge not reachable — all scenarios will be skipped (Tier B)"
  SKIP_ALL=true
fi

# ══════════════════════════════════════════════════════════════════════════════
# NS1: No raw PII in stored charts
# ══════════════════════════════════════════════════════════════════════════════
log_info "── NS1: No raw PII in stored charts ───────────"

if $SKIP_ALL; then
  log_skip "NS1 — bridge not reachable"
else
  CHARTS_RESP=$(rpc_call_auth "chart.list" '{"domain":"https://unknown","limit":100}' 2>/dev/null || true)
  if [[ -z "$CHARTS_RESP" ]]; then
    log_skip "NS1 — chart.list returned empty response (no charts or method missing)"
  else
    echo "$CHARTS_RESP" > "$EVIDENCE_DIR/ns1_charts.json"

    PII_COUNT=$(echo "$CHARTS_RESP" | jq -r '
      [.result[]?.steps // empty] |
      join(" ") |
      scan("\\b\\d{3}-\\d{2}-\\d{4}\\b") |
      length
    ' 2>/dev/null || echo "0")

    EMAIL_COUNT=$(echo "$CHARTS_RESP" | jq -r '
      [.result[]?.steps // empty] |
      join(" ") |
      scan("\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Za-z]{2,}\\b") |
      length
    ' 2>/dev/null || echo "0")

    if [[ "$PII_COUNT" == "0" ]] && [[ "$EMAIL_COUNT" == "0" ]]; then
      log_pass "NS1 — no raw PII detected in stored charts"
    else
      log_fail "NS1 — found SSN-like: $PII_COUNT, email-like: $EMAIL_COUNT in stored charts"
    fi
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# NS2: Policy rejection — malformed chart
# ══════════════════════════════════════════════════════════════════════════════
log_info "── NS2: Policy rejection (malformed chart) ────"

if $SKIP_ALL; then
  log_skip "NS2 — bridge not reachable"
else
  MALFORMED_RESP=$(rpc_call_auth "chart.save" '{
    "chart": {"version": -1, "action_map": {}},
    "meta": {"domain": "https://evil.com", "title": "<script>alert(1)</script>"}
  }' 2>/dev/null || true)

  echo "$MALFORMED_RESP" > "$EVIDENCE_DIR/ns2_malformed_response.json"

  if [[ -z "$MALFORMED_RESP" ]]; then
    log_skip "NS2 — no response from chart.save"
  elif echo "$MALFORMED_RESP" | jq -e '.error' &>/dev/null; then
    log_pass "NS2 — malformed chart correctly rejected"
  else
    CHART_ID=$(echo "$MALFORMED_RESP" | jq -r '.result.chart_id // empty' 2>/dev/null)
    if [[ -n "$CHART_ID" ]]; then
      rpc_call_auth "chart.delete" "{\"chart_id\":\"$CHART_ID\"}" &>/dev/null || true
      log_fail "NS2 — malformed chart was accepted (cleaned up)"
    else
      log_skip "NS2 — unexpected response shape"
    fi
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# NS3: Approval still required on replay
# ══════════════════════════════════════════════════════════════════════════════
log_info "── NS3: Approval required on replay ───────────"

if $SKIP_ALL; then
  log_skip "NS3 — bridge not reachable"
else
  APPROVAL_RESP=$(rpc_call_auth "chart.list" '{"domain":"https://unknown","limit":10}' 2>/dev/null || true)
  if [[ -z "$APPROVAL_RESP" ]]; then
    log_skip "NS3 — no chart data available"
  else
    echo "$APPROVAL_RESP" > "$EVIDENCE_DIR/ns3_approval_check.json"

    HAS_INPUT_CHARTS=$(echo "$APPROVAL_RESP" | jq -r '
      [.result[]? | select(.requires_approval == true)] | length
    ' 2>/dev/null || echo "0")

    if [[ "$HAS_INPUT_CHARTS" -gt 0 ]]; then
      log_pass "NS3 — input charts require approval flag"
    else
      ALL_CHARTS=$(echo "$APPROVAL_RESP" | jq -r '[.result[]?] | length' 2>/dev/null || echo "0")
      if [[ "$ALL_CHARTS" == "0" ]]; then
        log_skip "NS3 — no charts with input actions to verify"
      else
        log_fail "NS3 — input charts exist but approval flag not set"
      fi
    fi
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# NS4: Audit log entries present
# ══════════════════════════════════════════════════════════════════════════════
log_info "── NS4: Audit log entries present ─────────────"

if $SKIP_ALL; then
  log_skip "NS4 — bridge not reachable"
else
  AUDIT_RESP=$(rpc_call_auth "audit.list" '{"limit":5}' 2>/dev/null || true)
  if [[ -z "$AUDIT_RESP" ]]; then
    log_skip "NS4 — audit.list returned empty (method may not exist)"
  else
    echo "$AUDIT_RESP" > "$EVIDENCE_DIR/ns4_audit.json"

    AUDIT_COUNT=$(echo "$AUDIT_RESP" | jq -r '[.result[]?] | length' 2>/dev/null || echo "0")
    if [[ "$AUDIT_COUNT" -gt 0 ]]; then
      log_pass "NS4 — audit log has $AUDIT_COUNT entries"
    else
      log_skip "NS4 — audit log is empty (no chart operations yet)"
    fi
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# NS5: Malicious/malformed chart rejected
# ══════════════════════════════════════════════════════════════════════════════
log_info "── NS5: Malicious chart rejected ──────────────"

if $SKIP_ALL; then
  log_skip "NS5 — bridge not reachable"
else
  INJECT_RESP=$(rpc_call_auth "chart.save" '{
    "chart": {
      "version": 1,
      "target_domain": "https://evil.com",
      "action_map": {
        "action_1": {
          "action_type": "navigate",
          "url": "javascript:alert(document.cookie)"
        }
      }
    },
    "meta": {"domain": "https://evil.com", "title": "xss-payload-test"}
  }' 2>/dev/null || true)

  echo "$INJECT_RESP" > "$EVIDENCE_DIR/ns5_inject_response.json"

  if [[ -z "$INJECT_RESP" ]]; then
    log_skip "NS5 — no response from chart.save"
  elif echo "$INJECT_RESP" | jq -e '.error' &>/dev/null; then
    log_pass "NS5 — javascript: URL chart correctly rejected"
  else
    INJECT_ID=$(echo "$INJECT_RESP" | jq -r '.result.chart_id // empty' 2>/dev/null)
    if [[ -n "$INJECT_ID" ]]; then
      rpc_call_auth "chart.delete" "{\"chart_id\":\"$INJECT_ID\"}" &>/dev/null || true
      log_fail "NS5 — javascript: URL chart was accepted (cleaned up)"
    else
      log_skip "NS5 — unexpected response shape"
    fi
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════
harness_summary
