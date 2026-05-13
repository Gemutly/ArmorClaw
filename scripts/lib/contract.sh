#!/usr/bin/env bash
# contract.sh — Contract discovery helper library for ArmorClaw E2E plan
#
# Sourced by all Phase A scripts (a0_discover.sh, a1_deploy.sh, etc.).
# Extends tests/lib/ with contract-specific helpers for RPC probing,
# HTTP waiting, manifest management, and evidence persistence.
#
# Usage:
#   source "$(dirname "$0")/lib/contract.sh"
#   _contract_bridge_rpc "status" "{}"
#   _contract_save "evidence.json" "$result"

set -uo pipefail

# ── Locate repo root and script directory ──────────────────────────────────────
_CONTRACT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_REPO_ROOT="$(cd "${_CONTRACT_DIR}/../.." && pwd)"

# ── Source existing infrastructure ────────────────────────────────────────────
# tests/lib/load_env.sh: sources .env, exports VPS_IP/BRIDGE_PORT/etc, provides ssh_vps()
source "${_REPO_ROOT}/tests/lib/load_env.sh"

# tests/lib/common_output.sh: FULL_SYSTEM_PASSED/FAILED/SKIPPED counters, color vars
source "${_REPO_ROOT}/tests/lib/common_output.sh"

# ── Evidence directory ────────────────────────────────────────────────────────
export EVIDENCE_DIR="${_REPO_ROOT}/.sisyphus/evidence/armorclaw"
mkdir -p "$EVIDENCE_DIR"

# ── _contract_bridge_rpc(method, params_json, max_retries) ────────────────────
# Call Bridge JSON-RPC via ssh_vps (curl on VPS to localhost).
# Returns response body on stdout. Returns 1 on failure after retries.
#
# Arguments:
#   $1 - RPC method name (e.g. "status", "rpc.discover")
#   $2 - JSON params (default: "{}")
#   $3 - Max retries (default: 3)
_contract_bridge_rpc() {
  local method="${1:?Usage: _contract_bridge_rpc method [params_json] [max_retries]}"
  local params="${2:-{}}"
  local max_retries="${3:-3}"
  local base_delay=2
  local attempt=1
  local response=""

  while [[ $attempt -le $max_retries ]]; do
    # Build JSON-RPC request
    local request
    request=$(jq -nc \
      --arg method "$method" \
      --argjson params "$params" \
      '{jsonrpc:"2.0", id: 1, method: $method, params: $params}')

    # Call bridge via SSH
    response=$(ssh_vps "curl -sf -k -m 5 'https://localhost:${BRIDGE_PORT}/api' -H 'Content-Type: application/json' -d '${request}'" 2>/dev/null) && {
      echo "$response"
      return 0
    }

    local delay=$(( base_delay * (2 ** (attempt - 1)) ))
    log_info "[RPC] Attempt $attempt/$max_retries for '$method' failed, retrying in ${delay}s..."
    sleep "$delay"
    attempt=$((attempt + 1))
  done

  log_fail "[RPC] '$method' failed after $max_retries attempts"
  return 1
}

# ── _contract_wait_http(port, path, timeout_seconds) ──────────────────────────
# Poll an HTTP endpoint on the VPS until it returns 200 or timeout.
# Prints elapsed time and last status code on each attempt.
#
# Arguments:
#   $1 - Port number (e.g. 8080)
#   $2 - URL path (e.g. "/health")
#   $3 - Timeout in seconds (default: 180)
_contract_wait_http() {
  local port="${1:?Usage: _contract_wait_http port path [timeout]}"
  local path="${2:-/}"
  local timeout="${3:-180}"
  local start_time
  start_time=$(date +%s)
  local last_status="N/A"
  local poll_interval=5

  while true; do
    local now
    now=$(date +%s)
    local elapsed=$(( now - start_time ))

    if [[ $elapsed -ge $timeout ]]; then
      log_fail "[WAIT] http://$VPS_IP:$port$path timed out after ${elapsed}s (last status: $last_status)"
      return 1
    fi

    # Try HTTP request via SSH (avoids local firewall issues)
    last_status=$(ssh_vps "curl -sf -o /dev/null -w '%{http_code}' -m 5 'http://localhost:${port}${path}'" 2>/dev/null || echo "000")

    if [[ "$last_status" == "200" ]]; then
      log_pass "[WAIT] http://$VPS_IP:$port$path responded 200 in ${elapsed}s"
      return 0
    fi

    log_info "[WAIT] http://$VPS_IP:$port$path → $last_status (${elapsed}s/${timeout}s)"
    sleep "$poll_interval"
  done
}

# ── _contract_save(filename, content) ─────────────────────────────────────────
# Save content to .sisyphus/evidence/armorclaw/<filename>.
# Creates parent directory if needed. Echoes the saved path.
#
# Arguments:
#   $1 - Filename (e.g. "contract_manifest.json")
#   $2 - Content to write
_contract_save() {
  local filename="${1:?Usage: _contract_save filename content}"
  local content="${2:-}"
  local filepath="${EVIDENCE_DIR}/${filename}"

  mkdir -p "$(dirname "$filepath")"
  echo "$content" > "$filepath"
  log_info "[SAVE] Evidence saved to $filepath"
  echo "$filepath"
}

# ── _contract_save_raw(filename) ──────────────────────────────────────────────
# Save stdin to .sisyphus/evidence/armorclaw/<filename>.
# Use when content contains special chars or is piped.
#
# Arguments:
#   $1 - Filename
_contract_save_raw() {
  local filename="${1:?Usage: _contract_save_raw filename}"
  local filepath="${EVIDENCE_DIR}/${filename}"

  mkdir -p "$(dirname "$filepath")"
  cat > "$filepath"
  log_info "[SAVE] Evidence saved to $filepath"
  echo "$filepath"
}

# ── _contract_load_manifest() ─────────────────────────────────────────────────
# Load contract_manifest.json from evidence directory.
# If file doesn't exist, creates a minimal manifest and returns it.
# Echoes the JSON content.
_contract_load_manifest() {
  local manifest_path="${EVIDENCE_DIR}/contract_manifest.json"

  if [[ -f "$manifest_path" ]]; then
    cat "$manifest_path"
    return 0
  fi

  # Create minimal manifest
  local minimal
  minimal=$(jq -nc '{
    live_discovered: { rpc_methods: [], event_types: [], endpoints: [] },
    documented_reference: { env_vars: [], deep_links: [] },
    runtime_flags: { deployment_required: false },
    provisioning: {}
  }')
  echo "$minimal" > "$manifest_path"
  log_info "[MANIFEST] Created minimal manifest at $manifest_path"
  echo "$minimal"
}

# ── _contract_update_manifest(jq_filter, jq_value) ───────────────────────────
# Update contract_manifest.json atomically using jq.
# Reads existing manifest, applies update, writes back.
#
# Arguments:
#   $1 - jq filter path (e.g. '.runtime_flags.deployment_required')
#   $2 - jq value expression (e.g. 'true', '"string"', '[1,2,3]')
_contract_update_manifest() {
  local filter="${1:?Usage: _contract_update_manifest jq_filter jq_value}"
  local value="${2:-null}"
  local manifest_path="${EVIDENCE_DIR}/contract_manifest.json"
  local tmp_path="${manifest_path}.tmp"

  # Ensure manifest exists
  _contract_load_manifest > /dev/null

  # Read, update, write atomically
  jq "${filter} = ${value}" "$manifest_path" > "$tmp_path" && \
    mv "$tmp_path" "$manifest_path"

  log_info "[MANIFEST] Updated ${filter} = ${value}"
}

# ── _contract_update_manifest_merge(jq_expression) ────────────────────────────
# Apply a full jq expression to the manifest (for complex updates).
# Reads existing manifest, applies expression, writes back.
#
# Arguments:
#   $1 - jq expression (e.g. '.live_discovered.rpc_methods += [$new_method]')
_contract_update_manifest_merge() {
  local expression="${1:?Usage: _contract_update_manifest_merge jq_expression [jq_args...]}"
  local manifest_path="${EVIDENCE_DIR}/contract_manifest.json"
  local tmp_path="${manifest_path}.tmp"

  # Ensure manifest exists
  _contract_load_manifest > /dev/null

  # Read, apply expression with any extra jq args, write atomically
  jq "$expression" "${@:2}" "$manifest_path" > "$tmp_path" && \
    mv "$tmp_path" "$manifest_path"

  log_info "[MANIFEST] Applied merge: $expression"
}

# ── _contract_ssh_test() ──────────────────────────────────────────────────────
# Test SSH connectivity to VPS. Returns 0 on success, 1 on failure.
_contract_ssh_test() {
  local result
  result=$(ssh_vps "echo 'SSH_OK'" 2>/dev/null)
  if [[ "$result" == "SSH_OK" ]]; then
    log_pass "[SSH] Connectivity to ${VPS_USER}@${VPS_IP} verified"
    return 0
  else
    log_fail "[SSH] Cannot connect to ${VPS_USER}@${VPS_IP}"
    return 1
  fi
}

# ── _contract_check_bridge_port() ─────────────────────────────────────────────
# Check if the bridge port is listening on the VPS.
# Returns 0 if port is open, 1 if not.
_contract_check_bridge_port() {
  local port="${1:-$BRIDGE_PORT}"
  local result
  result=$(ssh_vps "ss -tlnp | grep ':${port}' | head -1" 2>/dev/null)
  if [[ -n "$result" ]]; then
    return 0
  else
    return 1
  fi
}

# ── _get_api_key() ────────────────────────────────────────────────────────────
# Resolve API key from environment: OPENROUTER_API_KEY → OPEN_AI_KEY → ZAI_API_KEY.
# Echoes the key value on stdout. Returns 1 if no key is set.
_get_api_key() {
  local key="${OPENROUTER_API_KEY:-${OPEN_AI_KEY:-${ZAI_API_KEY:-${API_KEY:-}}}}"
  if [[ -z "$key" ]]; then
    echo "ERROR: OPENROUTER_API_KEY, OPEN_AI_KEY, ZAI_API_KEY, or API_KEY must be set" >&2
    return 1
  fi
  echo "$key"
}

# ── _get_api_key_var() ────────────────────────────────────────────────────────
# Return the name of the env var that provided the API key.
# Useful for logging which key is in use. Returns 1 if no key is set.
_get_api_key_var() {
  if [[ -n "${OPENROUTER_API_KEY:-}" ]]; then
    echo "OPENROUTER_API_KEY"
  elif [[ -n "${OPEN_AI_KEY:-}" ]]; then
    echo "OPEN_AI_KEY"
  elif [[ -n "${ZAI_API_KEY:-}" ]]; then
    echo "ZAI_API_KEY"
  elif [[ -n "${API_KEY:-}" ]]; then
    echo "API_KEY"
  else
    echo "ERROR: No API key env var set" >&2
    return 1
  fi
}

# ── Initialization log ────────────────────────────────────────────────────────
log_info "[CONTRACT] Library loaded. Evidence dir: $EVIDENCE_DIR"
log_info "[CONTRACT] VPS: ${VPS_USER}@${VPS_IP}, Bridge: ${BRIDGE_PORT}, Matrix: ${MATRIX_PORT}"
