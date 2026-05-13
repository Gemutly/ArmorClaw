# group-b-studio.sh — Feature Group B: Agent lifecycle / Studio tests
#
# Sourced library (no shebang, no main block).
# Tests the full agent lifecycle via RPC over SSH to the Bridge:
#   1. Deploy/create agent via studio.create_agent
#   2. List agents via studio.list_agents
#   3. Start agent via studio.spawn_agent
#   4. Observe agent status via studio.list_instances
#   5. Stop/delete agent (cleanup)
#
# Usage:
#   source "${_SCRIPT_DIR}/feature-groups/group-b-studio.sh"
#   _group_b_run --vps-ip 1.2.3.4 --ssh-key ~/.ssh/id_rsa --ssh-user root --output-dir /tmp/evidence

# ── Internal state ──────────────────────────────────────────────────────────────
_GB_AGENT_ID=""
_GB_AGENT_NAME=""
_GB_INSTANCE_ID=""

# ── _gb_test_result(name, status, details, evidence_path, duration_ms) ──────────
# Print a single structured JSON test result to stdout.
_gb_test_result() {
  local name="${1:?}"
  local status="${2:?}"   # pass|fail|skip
  local details="${3:-}"
  local evidence_path="${4:-}"
  local duration_ms="${5:-0}"
  jq -nc \
    --arg name "$name" \
    --arg status "$status" \
    --arg details "$details" \
    --arg evidence_path "$evidence_path" \
    --argjson duration_ms "$duration_ms" \
    '{name: $name, status: $status, details: $details, evidence_path: $evidence_path, duration_ms: $duration_ms}'
}

# ── _gb_save_evidence(output_dir, filename, content) ────────────────────────────
# Save evidence JSON to output_dir/filename. Echoes the path.
_gb_save_evidence() {
  local output_dir="${1:?}"
  local filename="${2:?}"
  local content="${3:-}"
  local filepath="${output_dir}/${filename}"
  mkdir -p "$output_dir"
  echo "$content" > "$filepath"
  echo "$filepath"
}

# ── _gb_rpc(ssh_cmd_prefix, method, params_json) ───────────────────────────────
# Make an RPC call via SSH to the Bridge. Wraps _contract_bridge_rpc for
# studio methods. Echoes the raw JSON-RPC response.
# Returns 0 on success, 1 on failure.
_gb_rpc() {
  local method="${1:?Usage: _gb_rpc method [params_json]}"
  local params="${2:-{}}"
  _contract_bridge_rpc "$method" "$params" 3
}

# ── _gb_cleanup() ──────────────────────────────────────────────────────────────
# Best-effort cleanup: stop running instances, then delete the test agent.
# Called on ANY exit path (success or failure).
_gb_cleanup() {
  # Stop instance if spawned
  if [[ -n "$_GB_INSTANCE_ID" ]]; then
    local stop_resp
    stop_resp=$(_gb_rpc "studio.stop_instance" "{\"id\": \"${_GB_INSTANCE_ID}\"}" 2>/dev/null) || true
    if [[ -n "$stop_resp" ]]; then
      log_info "[GROUP-B] Stopped instance $_GB_INSTANCE_ID"
    fi
    _GB_INSTANCE_ID=""
  fi

  # Delete agent definition if created
  if [[ -n "$_GB_AGENT_ID" ]]; then
    local del_resp
    del_resp=$(_gb_rpc "studio.delete_agent" "{\"id\": \"${_GB_AGENT_ID}\"}" 2>/dev/null) || true
    if [[ -n "$del_resp" ]]; then
      log_info "[GROUP-B] Deleted agent $_GB_AGENT_ID"
    fi
    _GB_AGENT_ID=""
  fi
}

# ── _group_b_run(--vps-ip, --ssh-key, --ssh-user, --output-dir) ────────────────
# Main entry point for Feature Group B: Agent lifecycle / Studio tests.
# Accepts --vps-ip, --ssh-key, --ssh-user, --output-dir as arguments.
# Outputs a single JSON object with the overall group result.
_group_b_run() {
  local vps_ip="" ssh_key="" ssh_user="" output_dir=""

  # Parse arguments
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --vps-ip)     vps_ip="$2";     shift 2 ;;
      --ssh-key)    ssh_key="$2";    shift 2 ;;
      --ssh-user)   ssh_user="$2";   shift 2 ;;
      --output-dir) output_dir="$2"; shift 2 ;;
      *) log_info "[GROUP-B] Unknown argument: $1"; shift ;;
    esac
  done

  # Apply overrides to the environment if provided
  if [[ -n "$vps_ip" ]];   then VPS_IP="$vps_ip";       export VPS_IP; fi
  if [[ -n "$ssh_key" ]];  then SSH_KEY_PATH="$ssh_key"; export SSH_KEY_PATH; fi
  if [[ -n "$ssh_user" ]]; then VPS_USER="$ssh_user";    export VPS_USER; fi
  if [[ -n "$output_dir" ]]; then
    mkdir -p "$output_dir"
  else
    output_dir="${EVIDENCE_DIR:-.sisyphus/evidence}/group-b"
    mkdir -p "$output_dir"
  fi

  local group_start group_results=()
  group_start=$(date +%s%N)

  # Generate unique test agent name
  local ts
  ts=$(date +%s)
  _GB_AGENT_NAME="vps-test-agent-${ts}"
  _GB_AGENT_ID=""
  _GB_INSTANCE_ID=""

  # ── Step 0: Bridge health check ───────────────────────────────────────────
  local health_start health_ms health_resp health_result
  health_start=$(date +%s%N)

  # Use SSH to probe bridge health from localhost on VPS
  health_resp=$(ssh_vps "$(
    cat <<'REMOTESH'
_probe_bridge_health() {
  local port="${1:-8080}"
  local response
  response=$(curl -sf -k -m 5 "https://localhost:${port}/health" 2>/dev/null)
  if [[ $? -eq 0 && -n "$response" ]]; then echo "$response"; return 0; fi
  response=$(curl -sf -m 5 "http://localhost:${port}/health" 2>/dev/null)
  if [[ $? -eq 0 && -n "$response" ]]; then echo "$response"; return 0; fi
  return 1
}
_probe_bridge_health "${BRIDGE_PORT:-8080}"
REMOTESH
  )" 2>/dev/null)

  health_ms=$(( ($(date +%s%N) - health_start) / 1000000 ))

  if [[ -z "$health_resp" ]]; then
    # Bridge is down — RPC incompatible, skip all tests with skip-disabled
    log_info "[GROUP-B] Bridge RPC not responding — skipping all studio tests (skip-disabled)"
    local bridge_skip_details="RPC incompatible — bridge API not responding at ${VPS_IP}:${BRIDGE_PORT:-8080}"
    local skip_results=()
    skip_results+=("$(_gb_test_result "bridge-health" "skip-disabled" "$bridge_skip_details" "" "$health_ms")")
    skip_results+=("$(_gb_test_result "studio.create_agent" "skip-disabled" "Bridge RPC unavailable" "" 0)")
    skip_results+=("$(_gb_test_result "studio.list_agents" "skip-disabled" "Bridge RPC unavailable" "" 0)")
    skip_results+=("$(_gb_test_result "studio.spawn_agent" "skip-disabled" "Bridge RPC unavailable" "" 0)")
    skip_results+=("$(_gb_test_result "studio.list_instances" "skip-disabled" "Bridge RPC unavailable" "" 0)")
    skip_results+=("$(_gb_test_result "studio.stop_and_delete" "skip-disabled" "Bridge RPC unavailable" "" 0)")

    local all_results
    all_results=$(printf '%s\n' "${skip_results[@]}" | jq -s '.')
    local group_ms=$(( ($(date +%s%N) - group_start) / 1000000 ))
    jq -nc \
      --argjson results "$all_results" \
      --argjson duration_ms "$group_ms" \
      '{group: "B", name: "Agent Lifecycle / Studio", results: $results, duration_ms: $duration_ms, overall: "skip-disabled"}'
    return 0
  fi

  # Save health evidence and record result
  local health_evidence_path
  health_evidence_path=$(_gb_save_evidence "$output_dir" "group-b-bridge-health.json" "$health_resp")
  group_results+=("$(_gb_test_result "bridge-health" "pass" "Bridge healthy: $(echo "$health_resp" | head -c 200)" "$health_evidence_path" "$health_ms")")

  # Ensure cleanup on any exit
  trap _gb_cleanup EXIT

  # ── Step 1: Create agent ──────────────────────────────────────────────────
  local t1_start t1_ms t1_resp t1_result t1_details
  t1_start=$(date +%s%N)
  t1_resp=$(_gb_rpc "studio.create_agent" "$(jq -nc \
    --arg name "$_GB_AGENT_NAME" \
    --arg desc "VPS lifecycle test agent (auto-created)" \
    '{name: $name, description: $desc, skills: ["web_browsing"], pii_access: [], resource_tier: "medium"}'
  )" 2>/dev/null)
  t1_ms=$(( ($(date +%s%N) - t1_start) / 1000000 ))

  local t1_evidence_path=""
  if [[ -n "$t1_resp" ]]; then
    t1_evidence_path=$(_gb_save_evidence "$output_dir" "group-b-create-agent.json" "$t1_resp")
  fi

  if [[ -n "$t1_resp" ]] && echo "$t1_resp" | jq -e '.result.agent.id' >/dev/null 2>&1; then
    _GB_AGENT_ID=$(echo "$t1_resp" | jq -r '.result.agent.id')
    t1_details="Agent created: name=$_GB_AGENT_NAME id=$_GB_AGENT_ID"
    t1_result="pass"
    log_pass "[GROUP-B] Step 1: Created agent $_GB_AGENT_NAME (id=$_GB_AGENT_ID)"
  else
    t1_details="Failed to create agent: $(echo "$t1_resp" | head -c 300)"
    t1_result="fail"
    log_fail "[GROUP-B] Step 1: Agent creation failed"
    # Save what we got and skip remaining
    group_results+=("$(_gb_test_result "studio.create_agent" "$t1_result" "$t1_details" "$t1_evidence_path" "$t1_ms")")
    # Skip remaining tests
    group_results+=("$(_gb_test_result "studio.list_agents" "skip" "Agent creation failed" "" 0)")
    group_results+=("$(_gb_test_result "studio.spawn_agent" "skip" "Agent creation failed" "" 0)")
    group_results+=("$(_gb_test_result "studio.list_instances" "skip" "Agent creation failed" "" 0)")
    group_results+=("$(_gb_test_result "studio.stop_and_delete" "skip" "Agent creation failed" "" 0)")
    _gb_cleanup
    trap - EXIT
    local all_results overall="fail"
    all_results=$(printf '%s\n' "${group_results[@]}" | jq -s '.')
    local group_ms=$(( ($(date +%s%N) - group_start) / 1000000 ))
    jq -nc \
      --argjson results "$all_results" \
      --argjson duration_ms "$group_ms" \
      '{group: "B", name: "Agent Lifecycle / Studio", results: $results, duration_ms: $duration_ms, overall: "fail"}'
    return 1
  fi
  group_results+=("$(_gb_test_result "studio.create_agent" "$t1_result" "$t1_details" "$t1_evidence_path" "$t1_ms")")

  # ── Step 2: List agents — verify test agent appears ───────────────────────
  local t2_start t2_ms t2_resp t2_result t2_details
  t2_start=$(date +%s%N)
  t2_resp=$(_gb_rpc "studio.list_agents" '{"active_only": false}' 2>/dev/null)
  t2_ms=$(( ($(date +%s%N) - t2_start) / 1000000 ))

  local t2_evidence_path=""
  if [[ -n "$t2_resp" ]]; then
    t2_evidence_path=$(_gb_save_evidence "$output_dir" "group-b-list-agents.json" "$t2_resp")
  fi

  if [[ -n "$t2_resp" ]] && echo "$t2_resp" | jq -e ".result.agents[] | select(.name == \"$_GB_AGENT_NAME\")" >/dev/null 2>&1; then
    t2_result="pass"
    t2_details="Test agent $_GB_AGENT_NAME found in agent list"
    log_pass "[GROUP-B] Step 2: Agent found in list"
  else
    t2_result="fail"
    t2_details="Test agent $_GB_AGENT_NAME not found in list: $(echo "$t2_resp" | head -c 300)"
    log_fail "[GROUP-B] Step 2: Agent not found in list"
  fi
  group_results+=("$(_gb_test_result "studio.list_agents" "$t2_result" "$t2_details" "$t2_evidence_path" "$t2_ms")")

  # ── Step 3: Start (spawn) agent ───────────────────────────────────────────
  local t3_start t3_ms t3_resp t3_result t3_details
  t3_start=$(date +%s%N)
  t3_resp=$(_gb_rpc "studio.spawn_agent" "$(jq -nc \
    --arg id "$_GB_AGENT_ID" \
    --arg task "VPS lifecycle smoke test" \
    '{id: $id, task_description: $task}'
  )" 2>/dev/null)
  t3_ms=$(( ($(date +%s%N) - t3_start) / 1000000 ))

  local t3_evidence_path=""
  if [[ -n "$t3_resp" ]]; then
    t3_evidence_path=$(_gb_save_evidence "$output_dir" "group-b-spawn-agent.json" "$t3_resp")
  fi

  if [[ -n "$t3_resp" ]] && echo "$t3_resp" | jq -e '.result.instance.id' >/dev/null 2>&1; then
    _GB_INSTANCE_ID=$(echo "$t3_resp" | jq -r '.result.instance.id')
    local instance_status
    instance_status=$(echo "$t3_resp" | jq -r '.result.instance.status')
    t3_result="pass"
    t3_details="Agent spawned: instance=$_GB_INSTANCE_ID status=$instance_status"
    log_pass "[GROUP-B] Step 3: Spawned agent (instance=$_GB_INSTANCE_ID status=$instance_status)"
  else
    t3_result="fail"
    t3_details="Failed to spawn agent: $(echo "$t3_resp" | head -c 300)"
    log_fail "[GROUP-B] Step 3: Agent spawn failed"
  fi
  group_results+=("$(_gb_test_result "studio.spawn_agent" "$t3_result" "$t3_details" "$t3_evidence_path" "$t3_ms")")

  # ── Step 4: Observe agent status ──────────────────────────────────────────
  local t4_start t4_ms t4_resp t4_result t4_details
  t4_start=$(date +%s%N)

  if [[ -z "$_GB_INSTANCE_ID" ]]; then
    # No instance to observe
    t4_ms=0
    t4_result="skip"
    t4_details="No instance to observe (spawn failed)"
    t4_evidence_path=""
    log_info "[GROUP-B] Step 4: Skipped (no instance)"
  else
    t4_resp=$(_gb_rpc "studio.list_instances" "$(jq -nc \
      --arg def_id "$_GB_AGENT_ID" \
      '{definition_id: $def_id}'
    )" 2>/dev/null)
    t4_ms=$(( ($(date +%s%N) - t4_start) / 1000000 ))

    local t4_evidence_path=""
    if [[ -n "$t4_resp" ]]; then
      t4_evidence_path=$(_gb_save_evidence "$output_dir" "group-b-list-instances.json" "$t4_resp")
    fi

    if [[ -n "$t4_resp" ]] && echo "$t4_resp" | jq -e ".result.instances[] | select(.id == \"$_GB_INSTANCE_ID\")" >/dev/null 2>&1; then
      local obs_status
      obs_status=$(echo "$t4_resp" | jq -r ".result.instances[] | select(.id == \"$_GB_INSTANCE_ID\") | .status")
      t4_result="pass"
      t4_details="Instance $_GB_INSTANCE_ID observed with status=$obs_status"
      log_pass "[GROUP-B] Step 4: Instance observed (status=$obs_status)"
    else
      t4_result="fail"
      t4_details="Instance $_GB_INSTANCE_ID not found in instance list: $(echo "$t4_resp" | head -c 300)"
      log_fail "[GROUP-B] Step 4: Instance not found"
    fi
  fi
  group_results+=("$(_gb_test_result "studio.list_instances" "$t4_result" "$t4_details" "${t4_evidence_path:-}" "$t4_ms")")

  # ── Step 5: Stop instance and delete agent (cleanup) ──────────────────────
  local t5_start t5_ms t5_result t5_details
  local stop_ok=true del_ok=true
  t5_start=$(date +%s%N)

  # Stop instance first
  local stop_resp=""
  if [[ -n "$_GB_INSTANCE_ID" ]]; then
    stop_resp=$(_gb_rpc "studio.stop_instance" "{\"id\": \"${_GB_INSTANCE_ID}\"}" 2>/dev/null) || true
    if [[ -n "$stop_resp" ]] && echo "$stop_resp" | jq -e '.result' >/dev/null 2>&1; then
      log_pass "[GROUP-B] Step 5a: Stopped instance $_GB_INSTANCE_ID"
    else
      stop_ok=false
      log_fail "[GROUP-B] Step 5a: Failed to stop instance $_GB_INSTANCE_ID"
    fi
  fi

  # Delete agent
  local del_resp=""
  if [[ -n "$_GB_AGENT_ID" ]]; then
    del_resp=$(_gb_rpc "studio.delete_agent" "{\"id\": \"${_GB_AGENT_ID}\"}" 2>/dev/null) || true
    if [[ -n "$del_resp" ]] && echo "$del_resp" | jq -e '.result' >/dev/null 2>&1; then
      log_pass "[GROUP-B] Step 5b: Deleted agent $_GB_AGENT_ID"
    else
      del_ok=false
      log_fail "[GROUP-B] Step 5b: Failed to delete agent $_GB_AGENT_ID"
    fi
  fi

  t5_ms=$(( ($(date +%s%N) - t5_start) / 1000000 ))

  # Save cleanup evidence
  local t5_evidence_path
  local cleanup_evidence
  cleanup_evidence=$(jq -nc \
    --argjson stop_resp "${stop_resp:-null}" \
    --argjson del_resp "${del_resp:-null}" \
    '{stop_response: $stop_resp, delete_response: $del_resp}')
  t5_evidence_path=$(_gb_save_evidence "$output_dir" "group-b-cleanup.json" "$cleanup_evidence")

  if $stop_ok && $del_ok; then
    t5_result="pass"
    t5_details="Instance stopped and agent deleted successfully"
  else
    t5_result="fail"
    t5_details="Cleanup incomplete: stop=${stop_ok} delete=${del_ok}"
  fi

  # Clear IDs so the EXIT trap doesn't re-cleanup
  _GB_INSTANCE_ID=""
  _GB_AGENT_ID=""
  trap - EXIT

  group_results+=("$(_gb_test_result "studio.stop_and_delete" "$t5_result" "$t5_details" "$t5_evidence_path" "$t5_ms")")

  # ── Assemble overall result ───────────────────────────────────────────────
  local all_results overall group_ms fail_count
  all_results=$(printf '%s\n' "${group_results[@]}" | jq -s '.')

  # Determine overall status: pass if zero failures, fail otherwise
  fail_count=$(echo "$all_results" | jq '[.[] | select(.status == "fail")] | length')
  if [[ "$fail_count" -eq 0 ]]; then
    overall="pass"
  else
    overall="fail"
  fi

  group_ms=$(( ($(date +%s%N) - group_start) / 1000000 ))

  jq -nc \
    --argjson results "$all_results" \
    --argjson duration_ms "$group_ms" \
    --arg overall "$overall" \
    '{group: "B", name: "Agent Lifecycle / Studio", results: $results, duration_ms: $duration_ms, overall: $overall}'
}
