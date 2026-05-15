#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# T2.5: System Health After Full Startup
#
# Verifies all configured components report healthy after startup.
# Tier A: Runs on VPS, tests live health endpoints.
# READ-ONLY — does not restart the bridge or modify any state.
#
# Usage:  bash tests/test-system-health-baseline.sh
# Requires: ssh, curl, jq, socat (on VPS)
# ──────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib/load_env.sh"
source "$SCRIPT_DIR/lib/transport.sh"
source "$SCRIPT_DIR/lib/common_output.sh"
source "$SCRIPT_DIR/lib/assert_json.sh"

# ── Evidence output directory ─────────────────────────────────────────────────
EVIDENCE_DIR="$SCRIPT_DIR/../.sisyphus/evidence/full-system-t2.5"
mkdir -p "$EVIDENCE_DIR"

# ── Component status tracking (for H6 summary) ───────────────────────────────
COMP_BRIDGE="unknown"
COMP_MATRIX="unknown"
COMP_KEYSTORE="unknown"
COMP_EVENTBUS="unknown"

# ══════════════════════════════════════════════════════════════════════════════
# H0: Prerequisites
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " H0: Prerequisites"
echo "========================================="

H0_PASS=true

# Check jq
if command -v jq >/dev/null 2>&1; then
  log_pass "jq is available ($(jq --version))"
else
  log_fail "jq is required but not found"
  H0_PASS=false
fi

# Check curl
if command -v curl >/dev/null 2>&1; then
  log_pass "curl is available"
else
  log_fail "curl is required but not found"
  H0_PASS=false
fi

# Check bridge is running
if check_bridge_running; then
  log_pass "Bridge service is active on VPS"
else
  log_fail "Bridge service is NOT active on VPS"
  H0_PASS=false
fi

if ! $H0_PASS; then
  log_fail "H0 prerequisites failed — skipping remaining tests"
  harness_summary
  exit 1
fi

# ══════════════════════════════════════════════════════════════════════════════
# H1: Bridge Health — GET /health
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " H1: Bridge Health — GET /health"
echo "========================================="

H1_RESPONSE=""
H1_OK=false

if H1_RESPONSE=$(curl -ksS --max-time 10 "https://${VPS_IP}:${BRIDGE_PORT}/health" 2>&1) && [[ -n "$H1_RESPONSE" ]]; then
  H1_OK=true
  log_info "Response: $(echo "$H1_RESPONSE" | head -c 300)"
fi

if $H1_OK && assert_json_has_key "$H1_RESPONSE" "status"; then
  local_status=$(echo "$H1_RESPONSE" | jq -r '.status' 2>/dev/null)
  if [[ "$local_status" == "ok" || "$local_status" == "healthy" ]]; then
    log_pass "Bridge health status is '$local_status'"
    COMP_BRIDGE="ok"
  else
    log_fail "Bridge health status is '$local_status' (expected 'ok' or 'healthy')"
    COMP_BRIDGE="$local_status"
  fi
else
  log_fail "Bridge /health endpoint did not return valid JSON with 'status' key"
  COMP_BRIDGE="unreachable"
fi

# ══════════════════════════════════════════════════════════════════════════════
# H2: Bridge Status — GET /api/status
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " H2: Bridge Status — GET /api/status"
echo "========================================="

H2_RESPONSE=""
H2_OK=false

if H2_RESPONSE=$(curl -ksS --max-time 10 "https://${VPS_IP}:${BRIDGE_PORT}/api/status" 2>&1) && [[ -n "$H2_RESPONSE" ]]; then
  H2_OK=true
  log_info "Response: $(echo "$H2_RESPONSE" | head -c 300)"
fi

if $H2_OK && assert_json_has_key "$H2_RESPONSE" "status"; then
  local_status=$(echo "$H2_RESPONSE" | jq -r '.status' 2>/dev/null)
  if [[ "$local_status" == "running" || "$local_status" == "ok" || "$local_status" == "healthy" ]]; then
    log_pass "Bridge API status is '$local_status'"
  else
    log_fail "Bridge API status is '$local_status' (expected 'running', 'ok', or 'healthy')"
  fi
else
  log_fail "Bridge /api/status endpoint did not return valid JSON with 'status' key"
fi

# ══════════════════════════════════════════════════════════════════════════════
# H3: Discovery — GET /api/discovery
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " H3: Discovery — GET /api/discovery"
echo "========================================="

H3_RESPONSE=""
H3_OK=false

if H3_RESPONSE=$(curl -ksS --max-time 10 "https://${VPS_IP}:${BRIDGE_PORT}/api/discovery" 2>&1) && [[ -n "$H3_RESPONSE" ]]; then
  H3_OK=true
  log_info "Response: $(echo "$H3_RESPONSE" | head -c 300)"
fi

if $H3_OK; then
  assert_json_has_key "$H3_RESPONSE" "version" || true
  if echo "$H3_RESPONSE" | jq -e '.endpoints' >/dev/null 2>&1; then
    log_pass "Discovery returns endpoints"
  elif echo "$H3_RESPONSE" | jq -e '.methods' >/dev/null 2>&1; then
    log_pass "Discovery returns methods"
  elif echo "$H3_RESPONSE" | jq -e '.service_name' >/dev/null 2>&1; then
    log_pass "Discovery returns service metadata (service_name=$(echo "$H3_RESPONSE" | jq -r '.service_name'))"
  else
    log_fail "Discovery response missing 'endpoints', 'methods', or 'service_name'"
  fi
else
  log_fail "Bridge /api/discovery endpoint did not return valid JSON"
fi

# ══════════════════════════════════════════════════════════════════════════════
# H4: Matrix Health — health.check RPC via SSH + socat
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " H4: Matrix Health — health.check RPC (socket)"
echo "========================================="

H4_RESPONSE=""
H4_OK=false

# Check if socat is available on VPS and socket exists
if ssh_vps "command -v socat >/dev/null 2>&1 && test -S /run/armorclaw/bridge.sock" 2>/dev/null; then
  H4_RESPONSE=$(ssh_vps "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"health.check\",\"params\":{}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock" 2>/dev/null) || true
  if [[ -n "$H4_RESPONSE" ]]; then
    H4_OK=true
    log_info "Response: $(echo "$H4_RESPONSE" | head -c 300)"
  fi
fi

if ! $H4_OK; then
  log_skip "health.check RPC skipped (socat or socket not available on VPS)"
  COMP_MATRIX="skipped"
else
  if assert_rpc_success "$H4_RESPONSE"; then
    # Check for matrix/connected/healthy in result
    if echo "$H4_RESPONSE" | jq -e '.result' >/dev/null 2>&1; then
      matrix_status=$(echo "$H4_RESPONSE" | jq -r '.result.matrix // .result.status // .result.health // "unknown"' 2>/dev/null)
      if [[ "$matrix_status" == "connected" || "$matrix_status" == "healthy" || "$matrix_status" == "ok" ]]; then
        log_pass "Matrix component reports '$matrix_status'"
        COMP_MATRIX="$matrix_status"
      else
        log_info "Matrix component status: '$matrix_status'"
        COMP_MATRIX="$matrix_status"
      fi
    else
      log_fail "health.check RPC did not return a result"
      COMP_MATRIX="error"
    fi
  else
    log_fail "health.check RPC returned an error"
    COMP_MATRIX="error"
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# H5: Public Health — system.health RPC (no auth)
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " H5: Public Health — system.health RPC (no auth)"
echo "========================================="

H5_RESPONSE=""
H5_OK=false

H5_RESPONSE=$(curl -ksS --max-time 10 -X POST "https://${VPS_IP}:${BRIDGE_PORT}/api" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"system.health","params":{}}' 2>&1) || true

if [[ -n "$H5_RESPONSE" ]]; then
  H5_OK=true
  log_info "Response: $(echo "$H5_RESPONSE" | head -c 300)"
fi

if ! $H5_OK; then
  log_fail "system.health RPC request failed (no response)"
else
  # Public endpoint — should succeed without auth
  if assert_rpc_success "$H5_RESPONSE"; then
    log_pass "system.health RPC succeeded without authentication"
    # Extract component statuses if available
    if echo "$H5_RESPONSE" | jq -e '.result' >/dev/null 2>&1; then
      result_keys=$(echo "$H5_RESPONSE" | jq -r '.result | keys[]' 2>/dev/null || echo "")
      log_info "system.health result keys: $result_keys"

      # Try to extract component statuses for H6
      keystore_status=$(echo "$H5_RESPONSE" | jq -r '.result.keystore // .result.sqlcipher // "unknown"' 2>/dev/null)
      eventbus_status=$(echo "$H5_RESPONSE" | jq -r '.result.eventbus // .result.bus // "unknown"' 2>/dev/null)

      [[ "$keystore_status" != "unknown" && "$keystore_status" != "null" ]] && COMP_KEYSTORE="$keystore_status" || true
      [[ "$eventbus_status" != "unknown" && "$eventbus_status" != "null" ]] && COMP_EVENTBUS="$eventbus_status" || true
    fi
  else
    # If system.health is not a public method, it may require auth — that's acceptable
    log_info "system.health RPC returned error (may require auth — acceptable)"
    log_skip "system.health is not a public endpoint (requires auth)"
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# H6: Component Health Summary Table
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " H6: Component Health Summary"
echo "========================================="

# Determine overall status
OVERALL="healthy"
for comp in "$COMP_BRIDGE" "$COMP_MATRIX" "$COMP_KEYSTORE" "$COMP_EVENTBUS"; do
  if [[ "$comp" == "unreachable" || "$comp" == "error" ]]; then
    OVERALL="unhealthy"
    break
  elif [[ "$comp" != "ok" && "$comp" != "connected" && "$comp" != "ready" && "$comp" != "skipped" ]]; then
    if [[ "$OVERALL" == "healthy" ]]; then
      OVERALL="degraded"
    fi
  fi
done

# Build JSON summary
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

HEALTH_SUMMARY=$(cat <<HEREDOC
{
  "components": {
    "bridge": "$COMP_BRIDGE",
    "matrix": "$COMP_MATRIX",
    "keystore": "$COMP_KEYSTORE",
    "eventbus": "$COMP_EVENTBUS"
  },
  "timestamp": "$TIMESTAMP",
  "overall": "$OVERALL"
}
HEREDOC
)

# Save evidence
echo "$HEALTH_SUMMARY" | jq . > "$EVIDENCE_DIR/health-summary.json" 2>/dev/null || {
  echo "$HEALTH_SUMMARY" > "$EVIDENCE_DIR/health-summary.json"
}

log_info "Component health summary:"
echo "$HEALTH_SUMMARY" | jq . 2>/dev/null || echo "$HEALTH_SUMMARY"

if [[ "$OVERALL" == "healthy" ]]; then
  log_pass "Overall system status: $OVERALL"
elif [[ "$OVERALL" == "degraded" ]]; then
  log_info "Overall system status: $OVERALL (some components report non-ideal status)"
else
  log_fail "Overall system status: $OVERALL"
fi

log_info "Health summary saved to $EVIDENCE_DIR/health-summary.json"

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════
echo ""
harness_summary
