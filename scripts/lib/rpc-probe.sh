# rpc-probe.sh — RPC endpoint probe library for ArmorClaw Bridge
#
# Sourced library (no shebang, no main block).
# Probes bridge RPC endpoint via SSH and captures raw request/response evidence.
#
# Usage:
#   source "${_SCRIPT_DIR}/lib/rpc-probe.sh"
#   _rpc_probe_bridge "root@5.183.11.149" 8080
#   # prints classification to stdout: compatible|path-mismatch|protocol-mismatch|unreachable

# ── Endpoints to try (in order) ──────────────────────────────────────────────
_RPC_PROBE_ENDPOINTS=("/api" "/rpc" "/jsonrpc")

# ── JSON-RPC discover payload ────────────────────────────────────────────────
_RPC_PROBE_DISCOVER='{"jsonrpc":"2.0","id":1,"method":"rpc.discover","params":{}}'

# ── _rpc_probe_bridge(ssh_host, bridge_port) ─────────────────────────────────
# Probe bridge RPC via SSH and capture raw request/response evidence.
# Tests multiple endpoints and classifies the result.
# Saves evidence to ${EVIDENCE_DIR}/report-remediation/rpc-probe.json.
# Prints classification to stdout for callers to use.
#
# Arguments:
#   $1 - SSH host string (e.g. "root@5.183.11.149")
#   $2 - Bridge port (e.g. 8080)
#
# Returns 0 if classification is "compatible", 1 otherwise.
_rpc_probe_bridge() {
  local ssh_host="${1:?Usage: _rpc_probe_bridge ssh_host bridge_port}"
  local bridge_port="${2:?Usage: _rpc_probe_bridge ssh_host bridge_port}"
  local ssh_key="${SSH_KEY:-}"
  local evidence_dir="${EVIDENCE_DIR:-${_REPO_ROOT:-.}/.sisyphus/evidence/vps-lifecycle}"
  local report_dir="${evidence_dir}/report-remediation"
  local evidence_file="${report_dir}/rpc-probe.json"

  mkdir -p "$report_dir"

  local classification="unreachable"
  local results=()
  local found_compatible=false
  local scheme_used=""

  # Try HTTPS first, then HTTP for each endpoint
  for scheme in https http; do
    if [[ "$found_compatible" == "true" ]]; then
      break
    fi

    for endpoint in "${_RPC_PROBE_ENDPOINTS[@]}"; do
      local url="${scheme}://localhost:${bridge_port}${endpoint}"
      local request="$_RPC_PROBE_DISCOVER"
      local response=""
      local http_code=""

      # Build SSH command
      local ssh_cmd="curl -s -k -m 5 -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' -d '${request}' '${url}' 2>/dev/null"

      # Run via SSH
      if [[ -n "$ssh_key" ]]; then
        http_code=$(ssh -i "$ssh_key" -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$ssh_host" "$ssh_cmd" 2>/dev/null) || http_code="000"
      else
        http_code=$(ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$ssh_host" "$ssh_cmd" 2>/dev/null) || http_code="000"
      fi

      # If endpoint responded (non-zero status code), fetch the actual response body
      if [[ "$http_code" != "000" ]]; then
        local body_cmd="curl -s -k -m 5 -X POST -H 'Content-Type: application/json' -d '${request}' '${url}' 2>/dev/null"
        if [[ -n "$ssh_key" ]]; then
          response=$(ssh -i "$ssh_key" -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$ssh_host" "$body_cmd" 2>/dev/null) || response=""
        else
          response=$(ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$ssh_host" "$body_cmd" 2>/dev/null) || response=""
        fi

        # Classify this endpoint result
        local ep_class
        ep_class=$(_rpc_probe_classify_response "$http_code" "$response")

        results+=($(jq -nc \
          --arg url "$url" \
          --arg request "$request" \
          --arg response "${response:-<empty>}" \
          --arg http_code "$http_code" \
          --arg classification "$ep_class" \
          '{endpoint:$url, request:$request, response:$response, http_status:$http_code, classification:$classification}'))

        # Track best classification
        if [[ "$ep_class" == "compatible" ]]; then
          classification="compatible"
          found_compatible=true
          scheme_used="$scheme"
          break 2
        elif [[ "$ep_class" == "protocol-mismatch" && "$classification" == "unreachable" ]]; then
          classification="protocol-mismatch"
          scheme_used="$scheme"
        elif [[ "$ep_class" == "path-mismatch" && "$classification" == "unreachable" ]]; then
          classification="path-mismatch"
          scheme_used="$scheme"
        fi
      else
        # No response at all for this endpoint/scheme
        results+=($(jq -nc \
          --arg url "$url" \
          --arg request "$request" \
          --arg response "<empty>" \
          --arg http_code "000" \
          --arg classification "unreachable" \
          '{endpoint:$url, request:$request, response:$response, http_status:$http_code, classification:$classification}'))
      fi
    done
  done

  # Build evidence JSON
  local results_json
  results_json=$(printf '%s\n' "${results[@]}" | jq -s '.')
  local timestamp
  timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

  local evidence
  evidence=$(jq -nc \
    --arg classification "$classification" \
    --arg scheme "$scheme_used" \
    --arg port "$bridge_port" \
    --arg timestamp "$timestamp" \
    --argjson results "$results_json" \
    '{classification:$classification, scheme:$scheme, bridge_port:$port, timestamp:$timestamp, probes:$results}')

  # Save evidence
  echo "$evidence" > "$evidence_file"

  # Print classification to stdout for callers
  echo "$classification"

  # Return 0 only if compatible
  [[ "$classification" == "compatible" ]]
}

# ── _rpc_probe_classify_response(http_code, response_body) ──────────────────
# Classify a single RPC response.
# Returns one of: compatible, path-mismatch, protocol-mismatch
#
# Arguments:
#   $1 - HTTP status code (e.g. "200", "404")
#   $2 - Response body (may be empty if bridge silently drops)
_rpc_probe_classify_response() {
  local http_code="${1:-000}"
  local response="${2:-}"

  # 404 = path doesn't exist
  if [[ "$http_code" == "404" ]]; then
    echo "path-mismatch"
    return 0
  fi

  # Empty response — bridge silently dropped (not 404, so endpoint exists
  # but didn't respond to our payload)
  if [[ -z "$response" || "$response" == "<empty>" ]]; then
    echo "protocol-mismatch"
    return 0
  fi

  # Check for valid JSON-RPC response with methods
  local has_methods
  has_methods=$(echo "$response" | jq -r '.result.methods // empty' 2>/dev/null)

  if [[ -n "$has_methods" ]]; then
    echo "compatible"
    return 0
  fi

  # Check for any valid JSON-RPC response (has jsonrpc field or result/error)
  local is_jsonrpc
  is_jsonrpc=$(echo "$response" | jq -r 'if (.jsonrpc // .result // .error) then "yes" else "no" end' 2>/dev/null)

  if [[ "$is_jsonrpc" == "yes" ]]; then
    echo "compatible"
    return 0
  fi

  # Got a response but not valid JSON-RPC
  echo "protocol-mismatch"
}
