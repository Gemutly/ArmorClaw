#!/usr/bin/env bash
# a0_discover.sh — Phase A0: Contract discovery for ArmorClaw E2E
#
# Probes the live Bridge on VPS to discover RPC methods, HTTP endpoints,
# event types, env vars, and deep links. Generates contract_manifest.json.
# If Bridge is not running, records deployment_required=true and exits cleanly.

set -uo pipefail

_SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${_SCRIPT_DIR}/lib/contract.sh"

log_info "========================================="
log_info " Phase A0: Contract Discovery"
log_info "========================================="

# ── A0.1: Verify VPS SSH connectivity ────────────────────────────────────────
log_info "A0.1: Verifying SSH connectivity to ${VPS_USER}@${VPS_IP}..."
if ! _contract_ssh_test; then
  log_fail "A0.1: SSH connectivity failed — cannot proceed"
  exit 1
fi

# ── A0.2: Check if Bridge already running ────────────────────────────────────
log_info "A0.2: Checking if Bridge is running on port ${BRIDGE_PORT}..."
BRIDGE_RUNNING=false

# Check via systemd
if ssh_vps "systemctl is-active armorclaw-bridge.service 2>/dev/null" 2>/dev/null | grep -q "active"; then
  BRIDGE_RUNNING=true
  log_pass "A0.2: Bridge is running (systemd service active)"
fi

# Check via Docker
if [[ "$BRIDGE_RUNNING" == "false" ]]; then
  if ssh_vps "docker ps --format '{{.Names}}' 2>/dev/null" 2>/dev/null | grep -qi "armorclaw"; then
    BRIDGE_RUNNING=true
    log_pass "A0.2: Bridge is running (Docker container active)"
  fi
fi

# Check via port probe
if [[ "$BRIDGE_RUNNING" == "false" ]]; then
  if ssh_vps "curl -sf -k -o /dev/null -m 5 'https://localhost:${BRIDGE_PORT}/health'" 2>/dev/null; then
    BRIDGE_RUNNING=true
    log_pass "A0.2: Bridge is running (port probe successful)"
  fi
fi

if [[ "$BRIDGE_RUNNING" == "false" ]]; then
  log_info "A0.2: Bridge is NOT running. Recording deployment_required=true"
  _contract_update_manifest '.runtime_flags.deployment_required' 'true'
  _contract_save "a0_discovery_status.json" "$(jq -nc '{
    phase: "A0",
    status: "bridge_not_running",
    deployment_required: true,
    timestamp: (now | todate)
  }')"
  log_info "A0.2: Exiting discovery cleanly — deployment required before further probing"
  harness_summary
  exit 0
fi

_contract_update_manifest '.runtime_flags.deployment_required' 'false'

# ── A0.3: Discover HTTP endpoints ────────────────────────────────────────────
log_info "A0.3: Discovering HTTP endpoints..."
ENDPOINTS_JSON="[]"

probe_endpoint() {
  local path="$1"
  local description="${2:-$path}"
  local status_code
  status_code=$(ssh_vps "curl -sf -k -o /dev/null -w '%{http_code}' -m 5 'https://localhost:${BRIDGE_PORT}${path}'" 2>/dev/null || echo "000")

  local entry
  if [[ "$status_code" != "000" ]]; then
    local body_preview
    body_preview=$(ssh_vps "curl -sf -k -m 5 'https://localhost:${BRIDGE_PORT}${path}' 2>/dev/null | head -c 200" 2>/dev/null || echo "")
    local response_keys="[]"
    if [[ -n "$body_preview" ]] && echo "$body_preview" | jq -e . >/dev/null 2>&1; then
      response_keys=$(echo "$body_preview" | jq 'keys' 2>/dev/null || echo "[]")
    fi
    entry=$(jq -nc \
      --arg path "$path" \
      --arg status "$status_code" \
      --arg desc "$description" \
      --argjson keys "$response_keys" \
      '{path: $path, status_code: ($status | tonumber), description: $desc, response_keys: $keys}')
    log_pass "A0.3: ${path} → ${status_code}"
  else
    entry=$(jq -nc \
      --arg path "$path" \
      --arg desc "$description" \
      '{path: $path, status_code: 0, description: $desc, response_keys: []}')
    log_info "A0.3: ${path} → no response"
  fi
  ENDPOINTS_JSON=$(echo "$ENDPOINTS_JSON" | jq --argjson e "$entry" '. + [$e]')
}

for ep_path in "/health" "/api" "/.well-known/matrix/client" "/qr/config" "/metrics" "/version" "/status"; do
  probe_endpoint "$ep_path"
done

_contract_update_manifest_merge ".live_discovered.endpoints = \$ENDPOINTS" --argjson ENDPOINTS "$ENDPOINTS_JSON"

# ── A0.4: Discover RPC methods ──────────────────────────────────────────────
log_info "A0.4: Discovering RPC methods..."

# Try rpc.discover / system.listMethods first
DISCOVER_METHODS=""
DISCOVER_METHODS=$(_contract_bridge_rpc "rpc.discover" "{}" 1 2>/dev/null || echo "")
if [[ -z "$DISCOVER_METHODS" ]] || echo "$DISCOVER_METHODS" | jq -e '.error' >/dev/null 2>&1; then
  DISCOVER_METHODS=$(_contract_bridge_rpc "system.listMethods" "{}" 1 2>/dev/null || echo "")
fi

# Known RPC method list from bridge/pkg/rpc/server.go registerHandlers()
KNOWN_METHODS=(
  "ai.chat"
  "browser.navigate" "browser.fill" "browser.click" "browser.status"
  "browser.wait_for_element" "browser.wait_for_captcha" "browser.wait_for_2fa"
  "browser.complete" "browser.fail" "browser.list" "browser.cancel"
  "bridge.start" "bridge.stop" "bridge.status" "bridge.channel"
  "bridge.unchannel" "bridge.list" "bridge.ghost_list" "bridge.appservice_status"
  "pii.request" "pii.approve" "pii.deny" "pii.status" "pii.list_pending"
  "pii.stats" "pii.cancel" "pii.fulfill" "pii.wait_for_approval"
  "skills.execute" "skills.list" "skills.get_schema" "skills.allow" "skills.block"
  "skills.allowlist_add" "skills.allowlist_remove" "skills.allowlist_list"
  "skills.web_search" "skills.web_extract" "skills.email_send"
  "skills.slack_message" "skills.file_read" "skills.data_analyze"
  "matrix.status" "matrix.login" "matrix.send" "matrix.receive" "matrix.join_room"
  "events.replay" "events.stream"
  "studio.deploy" "studio.stats"
  "store_key"
  "provisioning.start" "provisioning.claim"
  "hardening.status" "hardening.ack" "hardening.rotate_password"
  "health.check" "mobile.heartbeat"
  "container.terminate" "container.list"
  "resolve_blocker"
  "approve_email" "deny_email" "email_approval_status" "email.list_pending"
  "account.delete"
  "secretary.start_workflow" "secretary.get_workflow" "secretary.cancel_workflow"
  "secretary.advance_workflow" "secretary.list_templates" "secretary.create_template"
  "secretary.get_template" "secretary.delete_template" "secretary.update_template"
  "task.create" "task.list" "task.cancel" "task.get"
  "device.list" "device.get" "device.approve" "device.reject"
  "invite.list" "invite.create" "invite.revoke" "invite.validate"
  "keystore.unseal" "keystore.sealed" "keystore.seal" "keystore.extend_session" "keystore.session_status" "keystore.list_keys" "keystore.delete_key"
)

RPC_METHODS_JSON="[]"
METHODS_FOUND=0
METHODS_RESPONDING=0

for method in "${KNOWN_METHODS[@]}"; do
  local_result=""
  local_status="unknown"
  local_error=""
  local_notes=""

  local_result=$(_contract_bridge_rpc "$method" "{}" 1 2>/dev/null) && {
    if echo "$local_result" | jq -e '.error' >/dev/null 2>&1; then
      local_status="error"
      local_error=$(echo "$local_result" | jq -r '.error.message // .error.code // "unknown error"' 2>/dev/null)
      local_notes="responds with error: ${local_error}"
      METHODS_FOUND=$((METHODS_FOUND + 1))
    elif echo "$local_result" | jq -e '.result' >/dev/null 2>&1; then
      local_status="responds"
      local_notes="responds with result"
      METHODS_FOUND=$((METHODS_FOUND + 1))
      METHODS_RESPONDING=$((METHODS_RESPONDING + 1))
    else
      local_status="unknown"
      local_notes="unexpected response format"
    fi
  } || {
    local_status="timeout"
    local_notes="no response or connection error"
  }

  entry=$(jq -nc \
    --arg name "$method" \
    --arg status "$local_status" \
    --arg result "$local_error" \
    --arg notes "$local_notes" \
    '{name: $name, status: $status, empty_params_result: $result, notes: $notes}')

  RPC_METHODS_JSON=$(echo "$RPC_METHODS_JSON" | jq --argjson e "$entry" '. + [$e]')
done

_contract_update_manifest_merge ".live_discovered.rpc_methods = \$METHODS" --argjson METHODS "$RPC_METHODS_JSON"
log_pass "A0.4: Discovered ${METHODS_FOUND} methods, ${METHODS_RESPONDING} responding"

# ── A0.5: Discover RPC parameter schemas ─────────────────────────────────────
log_info "A0.5: Probing RPC parameter schemas for responding methods..."

RESPONDING_METHODS=$(echo "$RPC_METHODS_JSON" | jq -r '.[] | select(.status=="responds" or .status=="error") | .name' 2>/dev/null)
SCHEMA_JSON="{}"

while IFS= read -r method; do
  [[ -z "$method" ]] && continue
  result=$(_contract_bridge_rpc "$method" "{}" 1 2>/dev/null || echo "")
  if [[ -n "$result" ]]; then
    error_msg=$(echo "$result" | jq -r '.error.message // empty' 2>/dev/null)
    if [[ -n "$error_msg" ]]; then
      SCHEMA_JSON=$(echo "$SCHEMA_JSON" | jq --arg m "$method" --arg e "$error_msg" '. + {($m): {hint: $e}}')
    fi
  fi
done <<< "$RESPONDING_METHODS"

_contract_save "a0_rpc_schemas.json" "$SCHEMA_JSON"
log_pass "A0.5: Parameter schema hints saved"

# ── A0.6: Discover Matrix event types ────────────────────────────────────────
log_info "A0.6: Checking Matrix homeserver..."

MATRIX_ENDPOINTS_JSON="[]"
MATRIX_VERSION=""
MATRIX_WELL_KNOWN=""

# Check Matrix version
MATRIX_VERSION=$(ssh_vps "curl -sf -m 5 'http://localhost:${MATRIX_PORT}/_matrix/client/versions'" 2>/dev/null || echo "")
if [[ -n "$MATRIX_VERSION" ]] && echo "$MATRIX_VERSION" | jq -e . >/dev/null 2>&1; then
  log_pass "A0.6: Matrix /versions responded"
  MATRIX_ENDPOINTS_JSON=$(echo "$MATRIX_ENDPOINTS_JSON" | jq '. + [{"endpoint": "/_matrix/client/versions", "status": "ok"}]')
else
  log_info "A0.6: Matrix /versions not responding"
fi

# Check .well-known
MATRIX_WELL_KNOWN=$(ssh_vps "curl -sf -k -m 5 'https://localhost:${BRIDGE_PORT}/.well-known/matrix/client'" 2>/dev/null || echo "")
if [[ -n "$MATRIX_WELL_KNOWN" ]] && echo "$MATRIX_WELL_KNOWN" | jq -e . >/dev/null 2>&1; then
  log_pass "A0.6: /.well-known/matrix/client responded"
  MATRIX_ENDPOINTS_JSON=$(echo "$MATRIX_ENDPOINTS_JSON" | jq '. + [{"endpoint": "/.well-known/matrix/client", "status": "ok"}]')
else
  log_info "A0.6: /.well-known/matrix/client not responding (may be on Matrix port)"
  MATRIX_WELL_KNOWN=$(ssh_vps "curl -sf -m 5 'http://localhost:${MATRIX_PORT}/.well-known/matrix/client'" 2>/dev/null || echo "")
  if [[ -n "$MATRIX_WELL_KNOWN" ]] && echo "$MATRIX_WELL_KNOWN" | jq -e . >/dev/null 2>&1; then
    log_pass "A0.6: Matrix /.well-known/matrix/client responded"
  fi
fi

# Note: Full event type discovery requires authenticated Matrix session (done in A2/A3)
_contract_save "a0_matrix_status.json" "$(jq -nc \
  --arg versions "$MATRIX_VERSION" \
  --arg well_known "$MATRIX_WELL_KNOWN" \
  --argjson endpoints "$MATRIX_ENDPOINTS_JSON" \
  '{versions_response: $versions, well_known: $well_known, endpoints: $endpoints, note: "Full event discovery requires Matrix session from A2"}')"

# ── A0.6b: Discover TLS metadata ──────────────────────────────────────────────
log_info "A0.6b: Probing TLS metadata from bridge.status..."
TLS_STATUS=""
TLS_STATUS=$(_contract_bridge_rpc "bridge.status" "{}" 1 2>/dev/null || echo "")

TLS_INFO_JSON='{"mode":"unknown","health":"unknown"}'
if [[ -n "$TLS_STATUS" ]] && echo "$TLS_STATUS" | jq -e '.result.tls' >/dev/null 2>&1; then
  TLS_INFO_JSON=$(echo "$TLS_STATUS" | jq '.result.tls')
  log_pass "A0.6b: TLS metadata retrieved — mode=$(echo "$TLS_INFO_JSON" | jq -r '.mode'), health=$(echo "$TLS_INFO_JSON" | jq -r '.health')"
else
  log_info "A0.6b: bridge.status.tls not available (older bridge version or not running)"
fi

_contract_update_manifest_merge '.live_discovered.tls = $TLS' --argjson TLS "$TLS_INFO_JSON"
_contract_save "a0_tls_status.json" "$TLS_INFO_JSON"

# Also probe /fingerprint endpoint for cross-check
FINGERPRINT_EP=""
FINGERPRINT_EP=$(ssh_vps "curl -sf -k -m 5 'https://localhost:${BRIDGE_PORT}/fingerprint'" 2>/dev/null || echo "")
if [[ -n "$FINGERPRINT_EP" ]] && echo "$FINGERPRINT_EP" | jq -e . >/dev/null 2>&1; then
  _contract_update_manifest_merge '.live_discovered.tls.fingerprint_endpoint = $FP' --argjson FP "$FINGERPRINT_EP"
fi

_contract_update_manifest_merge '.live_discovered.tls.external_scheme = "https"'

# ── A0.7: Document env var names ─────────────────────────────────────────────
log_info "A0.7: Documenting env var names..."
ENV_VARS_JSON=$(jq -nc '[
  "VPS_IP", "VPS_USER", "BRIDGE_PORT", "MATRIX_PORT", "SSH_KEY_PATH",
  "ADMIN_TOKEN", "OPENROUTER_API_KEY", "OPEN_AI_KEY", "ZAI_API_KEY",
  "TURN_SECRET", "KEYSTORE_SECRET", "ARMORCLAW_SERVER_MODE",
  "ARMORCLAW_RPC_TRANSPORT", "ARMORCLAW_LISTEN_ADDR",
  "ARMORCLAW_PUBLIC_BASE_URL", "ARMORCLAW_EMAIL"
]')
_contract_update_manifest_merge '.documented_reference.env_vars = $VARS' --argjson VARS "$ENV_VARS_JSON"
log_pass "A0.7: $(echo "$ENV_VARS_JSON" | jq 'length') env var names documented"

# ── A0.8: Document deep link formats ─────────────────────────────────────────
log_info "A0.8: Documenting deep link formats..."
DEEP_LINKS_JSON=$(jq -nc '[
  "armorclaw://config?d={base64_config}",
  "armorclaw://approve?token={approval_token}",
  "https://{domain}/qr/config?expiry={seconds}"
]')
_contract_update_manifest_merge '.documented_reference.deep_links = $LINKS' --argjson LINKS "$DEEP_LINKS_JSON"
log_pass "A0.8: Deep link formats documented"

# ── A0.9: Generate contract_manifest.json ─────────────────────────────────────
log_info "A0.9: Finalizing contract_manifest.json..."

# Add metadata
_contract_update_manifest_merge '.metadata = {
  generated_at: (now | todate),
  bridge_version: "4.6.0",
  vps_ip: $VPS_IP,
  bridge_port: ($BRIDGE_PORT | tonumber),
  methods_discovered: $METHODS,
  methods_responding: $RESPONDING
}' \
  --arg VPS_IP "$VPS_IP" \
  --arg BRIDGE_PORT "$BRIDGE_PORT" \
  --argjson METHODS "$METHODS_FOUND" \
  --argjson RESPONDING "$METHODS_RESPONDING"

# Save final manifest copy for easy verification
MANIFEST=$(_contract_load_manifest)
_contract_save "contract_manifest.json" "$MANIFEST"

log_pass "A0.9: contract_manifest.json generated"
log_info "  Methods found: ${METHODS_FOUND}/${#KNOWN_METHODS[@]}"
log_info "  Methods responding: ${METHODS_RESPONDING}"
log_info "  Endpoints probed: $(echo "$ENDPOINTS_JSON" | jq 'length')"

# ── Summary ──────────────────────────────────────────────────────────────────
_contract_save "a0_summary.json" "$(jq -nc \
  --arg phase "A0" \
  --argjson methods_found "$METHODS_FOUND" \
  --argjson methods_responding "$METHODS_RESPONDING" \
  --argjson methods_total "${#KNOWN_METHODS[@]}" \
  --arg bridge_running "$BRIDGE_RUNNING" \
  '{
    phase: $phase,
    status: "complete",
    methods_found: $methods_found,
    methods_responding: $methods_responding,
    methods_total: ($methods_total | tonumber),
    bridge_running: ($bridge_running == "true"),
    timestamp: (now | todate)
  }')"

log_info "========================================="
log_info " Phase A0: Discovery Complete"
log_info "========================================="
harness_summary
