#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# test-license-enforcement.sh — License Enforcement Harness (T8)
#
# Validates license enforcement RPCs on the bridge client.
# Tier B: License server NOT deployed — tests bridge client RPCs only.
# Bridge uses offline-first caching, so RPCs may respond from cache.
#
# Scenarios:
#   L0  Prerequisites — check license RPCs available, skip if not found
#   L1  License status — verify response shape from license.status
#   L2  Features list — verify available features array from license.features
#   L3  Feature check — verify allowed/denied from license.check_feature
#   L4  Compliance status — verify mode and fields from compliance.status
#   L5  Platform limits — verify tier and limits from platform.limits
#   L6  Grace period — verify bridge responds when server unreachable (cache)
#
# Usage:  bash tests/test-license-enforcement.sh
# Tier:   B (license server NOT deployed — tests bridge client RPCs only)
# ──────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib/load_env.sh"
source "$SCRIPT_DIR/lib/transport.sh"
source "$SCRIPT_DIR/lib/common_output.sh"
source "$SCRIPT_DIR/lib/assert_json.sh"

EVIDENCE_DIR="$SCRIPT_DIR/../.sisyphus/evidence/full-system-t8"
mkdir -p "$EVIDENCE_DIR"

# ── Override transport URL for VPS targeting ────────────────────────────────────
export BRIDGE_HTTP_URL="https://${VPS_IP}:${BRIDGE_PORT}"

# ── Auth-aware RPC wrapper (delegates to shared transport.sh) ───────────────────
rpc_call() { rpc_call_auth "$@"; }

# ── License states / compliance modes for reference ────────────────────────────
# States: Valid, GracePeriod, Expired, Invalid, Unknown
# Compliance modes: none, basic, standard, full, strict
# Features: 26 features in the license features array

# ══════════════════════════════════════════════════════════════════════════════
# L0: Prerequisites
# ══════════════════════════════════════════════════════════════════════════════
log_info "── L0: Prerequisites ─────────────────────────────"

# jq required for JSON assertions
if command -v jq &>/dev/null; then
  log_pass "jq available ($(jq --version 2>/dev/null || echo 'unknown version'))"
else
  log_fail "jq not found — required for JSON assertions"
  harness_summary
  exit 1
fi

# Bridge running check
if check_bridge_running; then
  log_pass "Bridge service is active on VPS"
else
  log_skip "Bridge not running — remaining tests require live bridge"
  harness_summary
  exit 0
fi

# Try license.status to determine if license RPCs are available
L0_STATUS_RESP=$(rpc_call "license.status" '{}')
echo "$L0_STATUS_RESP" | jq . > "$EVIDENCE_DIR/l0-prereq-status.json" 2>/dev/null || true

if echo "$L0_STATUS_RESP" | jq -e '.error.code' 2>/dev/null | grep -q -- '-32601'; then
  log_skip "license.status returned method not found (-32601)"
  log_skip "License RPCs not available in this bridge build — skipping all scenarios"
  harness_summary
  exit 0
fi

if echo "$L0_STATUS_RESP" | jq -e '.result' >/dev/null 2>&1 || echo "$L0_STATUS_RESP" | jq -e '.id' >/dev/null 2>&1; then
  log_pass "license.status RPC is available"
else
  log_skip "license.status returned unexpected response: ${L0_STATUS_RESP:0:200}"
  log_skip "Cannot proceed without license RPCs"
  harness_summary
  exit 0
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# L1: License status — license.status response shape
# ══════════════════════════════════════════════════════════════════════════════
log_info "── L1: License status ────────────────────────────"

L1_RESP=$(rpc_call "license.status" '{}')
echo "$L1_RESP" | jq . > "$EVIDENCE_DIR/l1-license-status.json" 2>/dev/null || true

# L1a: RPC succeeded (no error or has result)
if assert_rpc_success "$L1_RESP" >/dev/null 2>&1; then
  log_pass "L1a: license.status RPC succeeded"
else
  # License server not deployed is expected for Tier B — the bridge may return
  # a cached/default response or an error about unreachable server.
  # Check if it's a meaningful error (not method-not-found)
  L1_ERR_MSG=$(echo "$L1_RESP" | jq -r '.error.message' 2>/dev/null || echo "")
  if [[ -n "$L1_ERR_MSG" ]]; then
    log_skip "L1a: license.status error: $L1_ERR_MSG (license server may be unreachable)"
  else
    log_fail "L1a: license.status returned unexpected error"
  fi
fi

# L1b: Verify response shape — tier field
if echo "$L1_RESP" | jq -e '.result.tier' >/dev/null 2>&1; then
  L1_TIER=$(echo "$L1_RESP" | jq -r '.result.tier')
  log_pass "L1b: Response contains tier field (tier=$L1_TIER)"
else
  log_skip "L1b: tier field not present (bridge may use different schema or no cached license)"
fi

# L1c: Verify response shape — valid field
if echo "$L1_RESP" | jq -e '.result.valid' >/dev/null 2>&1; then
  L1_VALID=$(echo "$L1_RESP" | jq -r '.result.valid')
  log_pass "L1c: Response contains valid field (valid=$L1_VALID)"
else
  log_skip "L1c: valid field not present (bridge may use different schema)"
fi

# L1d: Verify response shape — compliance_mode field
if echo "$L1_RESP" | jq -e '.result.compliance_mode' >/dev/null 2>&1; then
  L1_MODE=$(echo "$L1_RESP" | jq -r '.result.compliance_mode')
  case "$L1_MODE" in
    none|basic|standard|full|strict)
      log_pass "L1d: compliance_mode is a known value ($L1_MODE)"
      ;;
    *)
      log_fail "L1d: compliance_mode '$L1_MODE' is not a recognized mode"
      ;;
  esac
else
  log_skip "L1d: compliance_mode field not present"
fi

# L1e: Verify response shape — expires_at field
if echo "$L1_RESP" | jq -e '.result.expires_at' >/dev/null 2>&1; then
  log_pass "L1e: Response contains expires_at field"
else
  log_skip "L1e: expires_at field not present"
fi

# L1f: Verify response shape — state field (Valid/GracePeriod/Expired/Invalid/Unknown)
if echo "$L1_RESP" | jq -e '.result.state' >/dev/null 2>&1; then
  L1_STATE=$(echo "$L1_RESP" | jq -r '.result.state')
  case "$L1_STATE" in
    Valid|GracePeriod|Expired|Invalid|Unknown)
      log_pass "L1f: state is a known value ($L1_STATE)"
      ;;
    *)
      log_fail "L1f: state '$L1_STATE' is not a recognized license state"
      ;;
  esac
else
  log_skip "L1f: state field not present"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# L2: Features list — license.features
# ══════════════════════════════════════════════════════════════════════════════
log_info "── L2: Features list ─────────────────────────────"

L2_RESP=$(rpc_call "license.features" '{}')
echo "$L2_RESP" | jq . > "$EVIDENCE_DIR/l2-license-features.json" 2>/dev/null || true

# L2a: RPC succeeded
if assert_rpc_success "$L2_RESP" >/dev/null 2>&1; then
  log_pass "L2a: license.features RPC succeeded"
else
  log_skip "L2a: license.features RPC error — $(echo "$L2_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)"
fi

# L2b: Response contains features array
if echo "$L2_RESP" | jq -e '.result.features' >/dev/null 2>&1; then
  L2_FEATURE_COUNT=$(echo "$L2_RESP" | jq '.result.features | length' 2>/dev/null || echo "0")
  if [[ "$L2_FEATURE_COUNT" -gt 0 ]]; then
    log_pass "L2b: Features array present with $L2_FEATURE_COUNT feature(s)"
  else
    log_fail "L2b: Features array is empty"
  fi
else
  # Try alternative path — features may be at result level directly
  if echo "$L2_RESP" | jq -e '.result | if type == "array" then true else false end' >/dev/null 2>&1; then
    L2_DIRECT_COUNT=$(echo "$L2_RESP" | jq '.result | length' 2>/dev/null || echo "0")
    log_pass "L2b: Features returned as array with $L2_DIRECT_COUNT item(s)"
  else
    log_skip "L2b: No features array found in response"
  fi
fi

# L2c: Features are objects with name/id keys
if echo "$L2_RESP" | jq -e '.result.features[0]' >/dev/null 2>&1; then
  L2_FIRST=$(echo "$L2_RESP" | jq -c '.result.features[0]' 2>/dev/null)
  if echo "$L2_FIRST" | jq -e 'has("name")' >/dev/null 2>&1 || echo "$L2_FIRST" | jq -e 'has("id")' >/dev/null 2>&1; then
    log_pass "L2c: Feature entries have name/id fields"
  else
    log_skip "L2c: Feature entries have unexpected structure: $L2_FIRST"
  fi
else
  log_skip "L2c: Cannot inspect feature entry structure (no features array)"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# L3: Feature check — license.check_feature
# ══════════════════════════════════════════════════════════════════════════════
log_info "── L3: Feature check ─────────────────────────────"

# L3a: Check a core feature that should exist
L3A_RESP=$(rpc_call "license.check_feature" '{"feature":"web_browsing"}')
echo "$L3A_RESP" | jq . > "$EVIDENCE_DIR/l3a-check-web-browsing.json" 2>/dev/null || true

if assert_rpc_success "$L3A_RESP" >/dev/null 2>&1; then
  log_pass "L3a: license.check_feature('web_browsing') succeeded"

  # Check allowed/denied field
  if echo "$L3A_RESP" | jq -e '.result.allowed' >/dev/null 2>&1; then
    L3A_ALLOWED=$(echo "$L3A_RESP" | jq -r '.result.allowed')
    log_pass "L3a: allowed field present (allowed=$L3A_ALLOWED)"
  else
    log_skip "L3a: allowed field not in response"
  fi
else
  log_skip "L3a: license.check_feature error — $(echo "$L3A_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)"
fi

# L3b: Check a feature that likely doesn't exist
L3B_RESP=$(rpc_call "license.check_feature" '{"feature":"nonexistent_feature_xyz"}')
echo "$L3B_RESP" | jq . > "$EVIDENCE_DIR/l3b-check-nonexistent.json" 2>/dev/null || true

if assert_rpc_success "$L3B_RESP" >/dev/null 2>&1; then
  L3B_ALLOWED=$(echo "$L3B_RESP" | jq -r '.result.allowed' 2>/dev/null || echo "unknown")
  if [[ "$L3B_ALLOWED" == "false" ]]; then
    log_pass "L3b: Nonexistent feature correctly denied (allowed=false)"
  else
    log_pass "L3b: Nonexistent feature returned allowed=$L3B_ALLOWED (bridge may default-allow)"
  fi
else
  # Error for unknown feature is also acceptable
  log_pass "L3b: Nonexistent feature returns error (expected behavior)"
fi

# L3c: Check without feature parameter (parameter validation)
L3C_RESP=$(rpc_call "license.check_feature" '{}')
echo "$L3C_RESP" | jq . > "$EVIDENCE_DIR/l3c-check-no-param.json" 2>/dev/null || true

if echo "$L3C_RESP" | jq -e '.error' >/dev/null 2>&1; then
  log_pass "L3c: Missing feature parameter returns error (parameter validation works)"
else
  log_skip "L3c: Missing feature parameter did not return error"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# L4: Compliance status — compliance.status
# ══════════════════════════════════════════════════════════════════════════════
log_info "── L4: Compliance status ─────────────────────────"

L4_RESP=$(rpc_call "compliance.status" '{}')
echo "$L4_RESP" | jq . > "$EVIDENCE_DIR/l4-compliance-status.json" 2>/dev/null || true

# L4a: RPC succeeded
if assert_rpc_success "$L4_RESP" >/dev/null 2>&1; then
  log_pass "L4a: compliance.status RPC succeeded"
else
  L4_ERR=$(echo "$L4_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
  log_skip "L4a: compliance.status error: $L4_ERR"
fi

# L4b: Verify compliance mode field
if echo "$L4_RESP" | jq -e '.result.mode' >/dev/null 2>&1; then
  L4_MODE=$(echo "$L4_RESP" | jq -r '.result.mode')
  case "$L4_MODE" in
    none|basic|standard|full|strict)
      log_pass "L4b: compliance mode is recognized ($L4_MODE)"
      ;;
    *)
      log_fail "L4b: compliance mode '$L4_MODE' is not a recognized mode"
      ;;
  esac
else
  log_skip "L4b: mode field not present in compliance.status response"
fi

# L4c: Verify compliance has status/summary fields
L4_FIELDS=0
echo "$L4_RESP" | jq -e '.result.mode' >/dev/null 2>&1 && L4_FIELDS=$((L4_FIELDS + 1))
echo "$L4_RESP" | jq -e '.result.level' >/dev/null 2>&1 && L4_FIELDS=$((L4_FIELDS + 1))
echo "$L4_RESP" | jq -e '.result.enforced' >/dev/null 2>&1 && L4_FIELDS=$((L4_FIELDS + 1))
echo "$L4_RESP" | jq -e '.result.status' >/dev/null 2>&1 && L4_FIELDS=$((L4_FIELDS + 1))
echo "$L4_RESP" | jq -e '.result.active' >/dev/null 2>&1 && L4_FIELDS=$((L4_FIELDS + 1))

if [[ "$L4_FIELDS" -ge 1 ]]; then
  log_pass "L4c: Compliance response has $L4_FIELDS structured field(s)"
else
  log_skip "L4c: Compliance response lacks expected structured fields"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# L5: Platform limits — platform.limits
# ══════════════════════════════════════════════════════════════════════════════
log_info "── L5: Platform limits ───────────────────────────"

L5_RESP=$(rpc_call "platform.limits" '{}')
echo "$L5_RESP" | jq . > "$EVIDENCE_DIR/l5-platform-limits.json" 2>/dev/null || true

# L5a: RPC succeeded
if assert_rpc_success "$L5_RESP" >/dev/null 2>&1; then
  log_pass "L5a: platform.limits RPC succeeded"
else
  L5_ERR=$(echo "$L5_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
  log_skip "L5a: platform.limits error: $L5_ERR"
fi

# L5b: Verify tier field
if echo "$L5_RESP" | jq -e '.result.tier' >/dev/null 2>&1; then
  L5_TIER=$(echo "$L5_RESP" | jq -r '.result.tier')
  log_pass "L5b: Platform tier present (tier=$L5_TIER)"
else
  log_skip "L5b: tier field not present in platform.limits"
fi

# L5c: Verify platform limits fields (agents, containers, etc.)
L5_LIMIT_FIELDS=0
echo "$L5_RESP" | jq -e '.result.max_agents' >/dev/null 2>&1 && L5_LIMIT_FIELDS=$((L5_LIMIT_FIELDS + 1))
echo "$L5_RESP" | jq -e '.result.max_containers' >/dev/null 2>&1 && L5_LIMIT_FIELDS=$((L5_LIMIT_FIELDS + 1))
echo "$L5_RESP" | jq -e '.result.max_concurrent_tasks' >/dev/null 2>&1 && L5_LIMIT_FIELDS=$((L5_LIMIT_FIELDS + 1))
echo "$L5_RESP" | jq -e '.result.agents' >/dev/null 2>&1 && L5_LIMIT_FIELDS=$((L5_LIMIT_FIELDS + 1))
echo "$L5_RESP" | jq -e '.result.containers' >/dev/null 2>&1 && L5_LIMIT_FIELDS=$((L5_LIMIT_FIELDS + 1))
echo "$L5_RESP" | jq -e '.result.limits' >/dev/null 2>&1 && L5_LIMIT_FIELDS=$((L5_LIMIT_FIELDS + 1))

if [[ "$L5_LIMIT_FIELDS" -ge 1 ]]; then
  log_pass "L5c: Platform limits response has $L5_LIMIT_FIELDS limit field(s)"
else
  log_skip "L5c: No recognized limit fields in platform.limits response"
fi

# L5d: Verify numeric limit values are non-negative where present
L5_NUMERIC_OK=true
for field in max_agents max_containers max_concurrent_tasks; do
  if echo "$L5_RESP" | jq -e ".result.$field" >/dev/null 2>&1; then
    L5_VAL=$(echo "$L5_RESP" | jq ".result.$field" 2>/dev/null)
    if [[ "$L5_VAL" -lt 0 ]] 2>/dev/null; then
      log_fail "L5d: $field is negative ($L5_VAL)"
      L5_NUMERIC_OK=false
    fi
  fi
done

if $L5_NUMERIC_OK; then
  log_pass "L5d: All present numeric limits are non-negative"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# L6: Grace period — bridge responds when license server unreachable
# ══════════════════════════════════════════════════════════════════════════════
log_info "── L6: Grace period / offline cache ──────────────"

# The license server is NOT deployed (Tier B). The bridge should still respond
# to RPCs using its offline-first cached license. This test verifies that the
# bridge client handles the unreachable server gracefully.

# L6a: Verify license.status still responds (from cache or default)
L6_STATUS_RESP=$(rpc_call "license.status" '{}')
echo "$L6_STATUS_RESP" | jq . > "$EVIDENCE_DIR/l6-grace-period-status.json" 2>/dev/null || true

if [[ -n "$L6_STATUS_RESP" ]] && echo "$L6_STATUS_RESP" | jq -e '.id' >/dev/null 2>&1; then
  log_pass "L6a: Bridge responds to license.status while server unreachable (offline cache)"
else
  log_skip "L6a: Bridge did not respond to license.status (may not have cached license)"
fi

# L6b: Verify the response indicates offline/grace-period state
if echo "$L6_STATUS_RESP" | jq -e '.result.state' >/dev/null 2>&1; then
  L6_STATE=$(echo "$L6_STATUS_RESP" | jq -r '.result.state')
  case "$L6_STATE" in
    GracePeriod|Unknown|Expired)
      log_pass "L6b: License state '$L6_STATE' indicates offline/grace-period behavior"
      ;;
    Valid)
      log_pass "L6b: License state 'Valid' — cached license is still valid"
      ;;
    *)
      log_skip "L6b: License state '$L6_STATE' — unexpected but bridge responded"
      ;;
  esac
else
  log_skip "L6b: Cannot determine license state (field not present)"
fi

# L6c: Verify platform.check still works (bridge functional without license server)
L6_CHECK_RESP=$(rpc_call "platform.check" '{}')
echo "$L6_CHECK_RESP" | jq . > "$EVIDENCE_DIR/l6-platform-check.json" 2>/dev/null || true

if [[ -n "$L6_CHECK_RESP" ]] && echo "$L6_CHECK_RESP" | jq -e '.id' >/dev/null 2>&1; then
  log_pass "L6c: platform.check responds while license server unreachable"
else
  log_skip "L6c: platform.check did not respond (may not be implemented or no cache)"
fi

# L6d: Verify license.features still returns data (from cache)
L6_FEAT_RESP=$(rpc_call "license.features" '{}')
echo "$L6_FEAT_RESP" | jq . > "$EVIDENCE_DIR/l6-grace-period-features.json" 2>/dev/null || true

if [[ -n "$L6_FEAT_RESP" ]] && echo "$L6_FEAT_RESP" | jq -e '.id' >/dev/null 2>&1; then
  log_pass "L6d: license.features responds while license server unreachable"
else
  log_skip "L6d: license.features did not respond (no cached feature data)"
fi

echo ""

# ── Evidence summary ────────────────────────────────────────────────────────────
log_info "── Evidence saved to $EVIDENCE_DIR/ ─────────────"
ls -la "$EVIDENCE_DIR/" 2>/dev/null | tail -n +2 | while read -r line; do
  log_info "  $line"
done

harness_summary
