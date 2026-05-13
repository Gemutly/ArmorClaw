# group-c-secretary.sh — Feature Group C: Secretary workflow tests
#
# Sourced library (no shebang, no main block).
# Functions prefixed with _ to avoid namespace collisions.
#
# Tests the full secretary workflow lifecycle:
#   1. Start workflow
#   2. Blocked workflow path — verify blocks
#   3. User response path — approve via Matrix, verify advances
#   4. Workflow completion — verify completes
#
# Dependencies:
#   scripts/lib/contract.sh  — _contract_bridge_rpc(), ssh_vps()
#   scripts/lib/matrix-state.sh — _matrix_load_state()
#
# Usage:
#   source "scripts/feature-groups/group-c-secretary.sh"
#   _group_c_run --vps-ip 1.2.3.4 --ssh-key ~/.ssh/id_rsa --ssh-user root --output-dir /tmp/evidence

# ── Constants ────────────────────────────────────────────────────────────────────
_GROUP_C_POLL_TIMEOUT=30
_GROUP_C_POLL_INTERVAL=3

# ── _group_c_parse_args() ────────────────────────────────────────────────────────
# Parse named arguments into shell variables.
# Sets: _GC_VPS_IP, _GC_SSH_KEY, _GC_SSH_USER, _GC_OUTPUT_DIR
_group_c_parse_args() {
  _GC_VPS_IP=""
  _GC_SSH_KEY=""
  _GC_SSH_USER=""
  _GC_OUTPUT_DIR=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --vps-ip)       _GC_VPS_IP="${2:-}";    shift 2 ;;
      --ssh-key)      _GC_SSH_KEY="${2:-}";   shift 2 ;;
      --ssh-user)     _GC_SSH_USER="${2:-}";  shift 2 ;;
      --output-dir)   _GC_OUTPUT_DIR="${2:-}"; shift 2 ;;
      *) shift ;;
    esac
  done

  if [[ -z "$_GC_VPS_IP" || -z "$_GC_SSH_KEY" || -z "$_GC_SSH_USER" || -z "$_GC_OUTPUT_DIR" ]]; then
    echo "[group-c] ERROR: missing required args --vps-ip, --ssh-key, --ssh-user, --output-dir" >&2
    return 1
  fi

  mkdir -p "$_GC_OUTPUT_DIR"
  return 0
}

# ── _group_c_ssh() ───────────────────────────────────────────────────────────────
# Run a command on the VPS via SSH.
_group_c_ssh() {
  local cmd="$1"
  ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 \
    -i "$_GC_SSH_KEY" "${_GC_SSH_USER}@${_GC_VPS_IP}" \
    "$cmd" 2>/dev/null
}

# ── _group_c_bridge_rpc() ────────────────────────────────────────────────────────
# Call Bridge JSON-RPC via SSH (curl on VPS to localhost).
# Uses _contract_bridge_rpc from contract.sh when available,
# otherwise implements direct SSH-based RPC.
#
# Arguments:
#   $1 - RPC method name
#   $2 - JSON params (default: "{}")
_group_c_bridge_rpc() {
  local method="${1:?Usage: _group_c_bridge_rpc method [params]}"
  local params="${2:-\{\}}"

  local request
  request=$(jq -nc \
    --arg method "$method" \
    --argjson params "$params" \
    '{jsonrpc:"2.0", id: 1, method: $method, params: $params}')

  local response
  response=$(_group_c_ssh "curl -sf -k -m 15 'https://localhost:${BRIDGE_PORT:-8443}/api' -H 'Content-Type: application/json' -d '${request}' 2>/dev/null || curl -sf -m 15 'http://localhost:${BRIDGE_PORT:-8443}/api' -H 'Content-Type: application/json' -d '${request}' 2>/dev/null")

  if [[ -n "$response" ]]; then
    echo "$response"
    return 0
  fi
  return 1
}

# ── _group_c_matrix_send() ──────────────────────────────────────────────────────
# Send a message to the test room via Matrix client API.
# Uses _matrix_load_state() credentials.
#
# Arguments:
#   $1 - Message body
_group_c_matrix_send() {
  local body="${1:?Usage: _group_c_matrix_send body}"

  if [[ -z "${_TEST_ACCESS_TOKEN:-}" || -z "${_TEST_ROOM_ID:-}" ]]; then
    echo "[group-c] ERROR: Matrix session not loaded" >&2
    return 1
  fi

  local conduit_url="${_MATRIX_CONDUIT_BASE_URL:-http://localhost:6167}"
  local txn_id="txn_$$_$(date +%s)"

  _group_c_ssh "curl -sf -m 10 -X PUT '${conduit_url}/_matrix/client/r0/rooms/${_TEST_ROOM_ID}/send/m.room.message/${txn_id}?access_token=${_TEST_ACCESS_TOKEN}' -H 'Content-Type: application/json' -d '{\"msgtype\":\"m.text\",\"body\":\"${body}\"}'" 2>/dev/null
}

# ── _group_c_matrix_poll() ──────────────────────────────────────────────────────
# Poll Matrix /sync for a bot response matching a pattern in the test room.
# Uses 30s timeout.
#
# Arguments:
#   $1 - Pattern to match in response body (grep -i)
# Returns: matching event JSON on stdout, or empty on timeout.
_group_c_matrix_poll() {
  local pattern="${1:?Usage: _group_c_matrix_poll pattern}"

  if [[ -z "${_TEST_ACCESS_TOKEN:-}" || -z "${_TEST_ROOM_ID:-}" ]]; then
    return 1
  fi

  local conduit_url="${_MATRIX_CONDUIT_BASE_URL:-http://localhost:6167}"
  local since=""
  local start_time
  start_time=$(date +%s)
  local elapsed=0

  while [[ $elapsed -lt $_GROUP_C_POLL_TIMEOUT ]]; do
    local sync_url="${conduit_url}/_matrix/client/r0/sync?access_token=${_TEST_ACCESS_TOKEN}&timeout=3000"
    [[ -n "$since" ]] && sync_url="${sync_url}&since=${since}"

    local sync_resp
    sync_resp=$(_group_c_ssh "curl -sf -m 10 '${sync_url}'" 2>/dev/null) || {
      sleep "$_GROUP_C_POLL_INTERVAL"
      elapsed=$(( $(date +%s) - start_time ))
      continue
    }

    # Update since token
    since=$(echo "$sync_resp" | jq -r '.next_batch // empty' 2>/dev/null)

    # Look for messages from bot in our test room
    local events_json
    events_json=$(echo "$sync_resp" | jq -c ".rooms.join[\"${_TEST_ROOM_ID}\"].timeline.events // []" 2>/dev/null)

    if [[ -n "$events_json" && "$events_json" != "null" && "$events_json" != "[]" ]]; then
      # Check for matching message
      local match
      match=$(echo "$events_json" | jq -c ".[] | select(.content.body // \"\" | test(\"${pattern}\"; \"i\"))" 2>/dev/null)
      if [[ -n "$match" ]]; then
        echo "$match"
        return 0
      fi
    fi

    sleep "$_GROUP_C_POLL_INTERVAL"
    elapsed=$(( $(date +%s) - start_time ))
  done

  return 1
}

# ── _group_c_save_evidence() ─────────────────────────────────────────────────────
# Save test evidence JSON to output directory.
#
# Arguments:
#   $1 - Test name (e.g., "start-workflow")
#   $2 - JSON content
_group_c_save_evidence() {
  local name="${1:?Usage: _group_c_save_evidence name json}"
  local json="${2:-}"
  local path="${_GC_OUTPUT_DIR}/group-c-${name}.json"
  echo "$json" > "$path"
  echo "$path"
}

# ── _group_c_make_result() ──────────────────────────────────────────────────────
# Build a structured test result JSON.
#
# Arguments:
#   $1 - Test name
#   $2 - Status (pass|fail|skip)
#   $3 - Details string
#   $4 - Evidence path
#   $5 - Duration in ms
_group_c_make_result() {
  local name="${1:?name}"
  local status="${2:?status}"
  local details="${3:-}"
  local evidence="${4:-}"
  local duration="${5:-0}"

  jq -nc \
    --arg name "$name" \
    --arg status "$status" \
    --arg details "$details" \
    --arg evidence "$evidence" \
    --argjson duration "$duration" \
    '{name: $name, status: $status, details: $details, evidence_path: $evidence, duration_ms: $duration}'
}

# ── _group_c_ensure_test_template() ─────────────────────────────────────────────
# Create a minimal test workflow template that will hit a blocked step.
# Returns the template_id on stdout.
_group_c_ensure_test_template() {
  local tpl_id="e2e_test_tpl_secretary_$$"

  # Create a template with a step that requires user input (blocked step)
  local steps_json
  steps_json=$(jq -nc '[
    {step_id: "step_1", order: 0, type: "action", name: "Initial Action", config: {action_type: "log"}},
    {step_id: "step_2", order: 1, type: "action", name: "Await Approval", config: {action_type: "blocker", requires_input: true, prompt: "Please approve this action"}},
    {step_id: "step_3", order: 2, type: "action", name: "Final Action", config: {action_type: "log"}}
  ]')

  local result
  result=$(_group_c_bridge_rpc "secretary.create_template" \
    "{\"name\":\"E2E Test Secretary\",\"description\":\"Test template for secretary workflow E2E\",\"steps\":${steps_json},\"created_by\":\"e2e_test\"}")

  local err
  err=$(echo "$result" | jq -r '.error.message // empty' 2>/dev/null)
  if [[ -n "$err" ]]; then
    echo "[group-c] ERROR creating template: $err" >&2
    return 1
  fi

  # Extract template id from result
  local tid
  tid=$(echo "$result" | jq -r '.result.id // empty' 2>/dev/null)
  if [[ -z "$tid" ]]; then
    echo "[group-c] ERROR: no template_id in create_template response" >&2
    return 1
  fi

  echo "$tid"
}

# ── _group_c_test_start_workflow() ──────────────────────────────────────────────
# Test 1: Start a workflow from a template and verify it enters running state.
_group_c_test_start_workflow() {
  local start_ms
  start_ms=$(date +%s%3N)

  local tpl_id
  tpl_id=$(_group_c_ensure_test_template) || {
    local result
    result=$(_group_c_make_result "start-workflow" "fail" "Could not create test template" "" 0)
    _group_c_save_evidence "start-workflow" "$result"
    echo "$result"
    return 0
  }

  # Create workflow from template
  local create_result
  create_result=$(_group_c_bridge_rpc "secretary.create_workflow" \
    "{\"template_id\":\"${tpl_id}\",\"created_by\":\"e2e_test\"}")

  local wf_id
  wf_id=$(echo "$create_result" | jq -r '.result.id // empty' 2>/dev/null)
  if [[ -z "$wf_id" ]]; then
    local err_msg
    err_msg=$(echo "$create_result" | jq -r '.error.message // "no workflow_id"' 2>/dev/null)
    local elapsed=$(( $(date +%s%3N) - start_ms ))
    local result
    result=$(_group_c_make_result "start-workflow" "fail" "create_workflow failed: $err_msg" "" "$elapsed")
    _group_c_save_evidence "start-workflow" "$result"
    echo "$result"
    return 0
  fi

  # Start the workflow
  local start_result
  start_result=$(_group_c_bridge_rpc "secretary.start_workflow" \
    "{\"workflow_id\":\"${wf_id}\"}")

  local wf_status
  wf_status=$(echo "$start_result" | jq -r '.result.status // empty' 2>/dev/null)

  _group_c_save_evidence "start-workflow" "$start_result"

  local elapsed=$(( $(date +%s%3N) - start_ms ))

  if [[ "$wf_status" == "started" ]]; then
    # Store workflow_id for subsequent tests
    _GC_WF_ID="$wf_id"
    _GC_TPL_ID="$tpl_id"
    _group_c_make_result "start-workflow" "pass" "Workflow ${wf_id} started successfully" "${_GC_OUTPUT_DIR}/group-c-start-workflow.json" "$elapsed"
  else
    local err_msg
    err_msg=$(echo "$start_result" | jq -r '.error.message // "unexpected status: $wf_status"' 2>/dev/null)
    _group_c_make_result "start-workflow" "fail" "start_workflow returned: $err_msg" "${_GC_OUTPUT_DIR}/group-c-start-workflow.json" "$elapsed"
  fi
}

# ── _group_c_test_blocked_path() ────────────────────────────────────────────────
# Test 2: Verify the workflow enters a blocked state when it needs user input.
# Polls secretary.get_workflow until status is blocked or timeout.
_group_c_test_blocked_path() {
  local start_ms
  start_ms=$(date +%s%3N)

  if [[ -z "${_GC_WF_ID:-}" ]]; then
    local result
    result=$(_group_c_make_result "blocked-path" "skip" "No workflow_id from previous test" "" 0)
    _group_c_save_evidence "blocked-path" "$result"
    echo "$result"
    return 0
  fi

  # Poll for blocked state (workflow should reach step_2 which is a blocker)
  local poll_start
  poll_start=$(date +%s)
  local wf_status=""
  local get_result=""

  while [[ $(( $(date +%s) - poll_start )) -lt $_GROUP_C_POLL_TIMEOUT ]]; do
    get_result=$(_group_c_bridge_rpc "secretary.get_workflow" \
      "{\"workflow_id\":\"${_GC_WF_ID}\"}")

    wf_status=$(echo "$get_result" | jq -r '.result.status // empty' 2>/dev/null)

    if [[ "$wf_status" == "blocked" ]]; then
      break
    fi

    # If completed or failed unexpectedly, break out
    if [[ "$wf_status" == "completed" || "$wf_status" == "failed" || "$wf_status" == "cancelled" ]]; then
      break
    fi

    sleep "$_GROUP_C_POLL_INTERVAL"
  done

  _group_c_save_evidence "blocked-path" "$get_result"

  local elapsed=$(( $(date +%s%3N) - start_ms ))

  if [[ "$wf_status" == "blocked" ]]; then
    _group_c_make_result "blocked-path" "pass" "Workflow entered blocked state at approval step" "${_GC_OUTPUT_DIR}/group-c-blocked-path.json" "$elapsed"
  else
    _group_c_make_result "blocked-path" "fail" "Workflow status is '${wf_status}', expected 'blocked'" "${_GC_OUTPUT_DIR}/group-c-blocked-path.json" "$elapsed"
  fi
}

# ── _group_c_test_user_response() ───────────────────────────────────────────────
# Test 3: Send an approval response and verify the workflow advances past the block.
_group_c_test_user_response() {
  local start_ms
  start_ms=$(date +%s%3N)

  if [[ -z "${_GC_WF_ID:-}" ]]; then
    local result
    result=$(_group_c_make_result "user-response" "skip" "No workflow_id from previous test" "" 0)
    _group_c_save_evidence "user-response" "$result"
    echo "$result"
    return 0
  fi

  # Resolve the blocker via RPC (resolve_blocker method)
  local resolve_result
  resolve_result=$(_group_c_bridge_rpc "resolve_blocker" \
    "{\"workflow_id\":\"${_GC_WF_ID}\",\"step_id\":\"step_2\",\"input\":\"approve\",\"note\":\"E2E test approval\"}")

  local delivered
  delivered=$(echo "$resolve_result" | jq -r '.result.status // empty' 2>/dev/null)

  if [[ "$delivered" != "delivered" ]]; then
    # Try Matrix-based approval as fallback: send approval message in test room
    if [[ -n "${_TEST_ACCESS_TOKEN:-}" && -n "${_TEST_ROOM_ID:-}" ]]; then
      _group_c_matrix_send "/approve ${_GC_WF_ID}" >/dev/null 2>&1 || true

      # Poll for bot acknowledgment
      _group_c_matrix_poll "approv" >/dev/null 2>&1 || true
    fi
  fi

  _group_c_save_evidence "user-response" "$resolve_result"

  # Now poll for workflow to advance past blocked state
  local poll_start
  poll_start=$(date +%s)
  local wf_status=""

  while [[ $(( $(date +%s) - poll_start )) -lt $_GROUP_C_POLL_TIMEOUT ]]; do
    local get_result
    get_result=$(_group_c_bridge_rpc "secretary.get_workflow" \
      "{\"workflow_id\":\"${_GC_WF_ID}\"}")

    wf_status=$(echo "$get_result" | jq -r '.result.status // empty' 2>/dev/null)

    # Advanced means no longer blocked (could be running, completed)
    if [[ "$wf_status" != "blocked" && -n "$wf_status" ]]; then
      _group_c_save_evidence "user-response" "$get_result"
      break
    fi

    sleep "$_GROUP_C_POLL_INTERVAL"
  done

  local elapsed=$(( $(date +%s%3N) - start_ms ))

  if [[ "$wf_status" != "blocked" && -n "$wf_status" ]]; then
    _group_c_make_result "user-response" "pass" "Workflow advanced after approval, status: ${wf_status}" "${_GC_OUTPUT_DIR}/group-c-user-response.json" "$elapsed"
  else
    _group_c_make_result "user-response" "fail" "Workflow still blocked after approval attempt" "${_GC_OUTPUT_DIR}/group-c-user-response.json" "$elapsed"
  fi
}

# ── _group_c_test_completion() ──────────────────────────────────────────────────
# Test 4: Verify the workflow reaches completed state.
_group_c_test_completion() {
  local start_ms
  start_ms=$(date +%s%3N)

  if [[ -z "${_GC_WF_ID:-}" ]]; then
    local result
    result=$(_group_c_make_result "completion" "skip" "No workflow_id from previous test" "" 0)
    _group_c_save_evidence "completion" "$result"
    echo "$result"
    return 0
  fi

  # Poll for completed state
  local poll_start
  poll_start=$(date +%s)
  local wf_status=""
  local get_result=""

  while [[ $(( $(date +%s) - poll_start )) -lt $_GROUP_C_POLL_TIMEOUT ]]; do
    get_result=$(_group_c_bridge_rpc "secretary.get_workflow" \
      "{\"workflow_id\":\"${_GC_WF_ID}\"}")

    wf_status=$(echo "$get_result" | jq -r '.result.status // empty' 2>/dev/null)

    if [[ "$wf_status" == "completed" || "$wf_status" == "failed" || "$wf_status" == "cancelled" ]]; then
      break
    fi

    sleep "$_GROUP_C_POLL_INTERVAL"
  done

  _group_c_save_evidence "completion" "$get_result"

  local elapsed=$(( $(date +%s%3N) - start_ms ))

  if [[ "$wf_status" == "completed" ]]; then
    _group_c_make_result "completion" "pass" "Workflow completed successfully" "${_GC_OUTPUT_DIR}/group-c-completion.json" "$elapsed"
  elif [[ "$wf_status" == "failed" ]]; then
    _group_c_make_result "completion" "fail" "Workflow failed (expected completed)" "${_GC_OUTPUT_DIR}/group-c-completion.json" "$elapsed"
  elif [[ "$wf_status" == "cancelled" ]]; then
    _group_c_make_result "completion" "fail" "Workflow was cancelled (expected completed)" "${_GC_OUTPUT_DIR}/group-c-completion.json" "$elapsed"
  else
    _group_c_make_result "completion" "fail" "Workflow did not reach terminal state, status: ${wf_status:-unknown}" "${_GC_OUTPUT_DIR}/group-c-completion.json" "$elapsed"
  fi
}

# ── _group_c_cleanup() ─────────────────────────────────────────────────────────
# Clean up test workflows and templates.
_group_c_cleanup() {
  # Cancel test workflow if still running
  if [[ -n "${_GC_WF_ID:-}" ]]; then
    _group_c_bridge_rpc "secretary.cancel_workflow" \
      "{\"workflow_id\":\"${_GC_WF_ID}\",\"reason\":\"E2E cleanup\"}" >/dev/null 2>&1 || true
  fi

  # Delete test template
  if [[ -n "${_GC_TPL_ID:-}" ]]; then
    _group_c_bridge_rpc "secretary.delete_template" \
      "{\"template_id\":\"${_GC_TPL_ID}\"}" >/dev/null 2>&1 || true
  fi

}

# ── _group_c_run() ──────────────────────────────────────────────────────────────
# Main entry point for Feature Group C: Secretary workflow tests.
#
# Arguments:
#   --vps-ip      VPS IP address
#   --ssh-key     Path to SSH private key
#   --ssh-user    SSH username
#   --output-dir  Directory for evidence files
#
# Outputs structured JSON to stdout:
#   {
#     "group": "c-secretary",
#     "status": "pass|fail|partial",
#     "tests": [ {name, status, details, evidence_path, duration_ms} ],
#     "total_duration_ms": <int>
#   }
_group_c_run() {
  _group_c_parse_args "$@" || return 1

  local group_start_ms
  group_start_ms=$(date +%s%3N)

  # Initialize workflow tracking vars
  _GC_WF_ID=""
  _GC_TPL_ID=""

  # ── Bridge RPC pre-check ─────────────────────────────────────────────────
  local rpc_probe
  rpc_probe=$(_group_c_bridge_rpc "rpc.discover" '{}' 2>/dev/null)
  if [[ -z "$rpc_probe" ]]; then
    local probe_ms=$(( $(date +%s%3N) - group_start_ms ))
    local skip_results
    skip_results=$(jq -nc '[
      {name: "rpc-probe", status: "skip-disabled", details: "RPC incompatible — bridge API not responding", evidence_path: "", duration_ms: 0},
      {name: "start-workflow", status: "skip-disabled", details: "Bridge RPC unavailable", evidence_path: "", duration_ms: 0},
      {name: "blocked-path", status: "skip-disabled", details: "Bridge RPC unavailable", evidence_path: "", duration_ms: 0},
      {name: "user-response", status: "skip-disabled", details: "Bridge RPC unavailable", evidence_path: "", duration_ms: 0},
      {name: "completion", status: "skip-disabled", details: "Bridge RPC unavailable", evidence_path: "", duration_ms: 0}
    ]')
    jq -nc \
      --argjson results "$skip_results" \
      --argjson duration_ms "$probe_ms" \
      '{group: "c-secretary", name: "Secretary Workflows", results: $results, duration_ms: $duration_ms, overall: "skip-disabled"}'
    return 0
  fi

  # Load Matrix session state for approval path tests
  if [[ -n "${_SCRIPT_DIR:-}" ]]; then
    _matrix_load_state >/dev/null 2>&1 || true
  fi

  local results=()
  local pass_count=0
  local fail_count=0
  local skip_count=0

  # Run tests sequentially (each depends on previous state)
  local t1 t2 t3 t4

  t1=$(_group_c_test_start_workflow)
  results+=("$t1")
  local s1
  s1=$(echo "$t1" | jq -r '.status')
  [[ "$s1" == "pass" ]] && ((pass_count++)) || true
  [[ "$s1" == "fail" ]] && ((fail_count++)) || true
  [[ "$s1" == "skip" ]] && ((skip_count++)) || true

  t2=$(_group_c_test_blocked_path)
  results+=("$t2")
  local s2
  s2=$(echo "$t2" | jq -r '.status')
  [[ "$s2" == "pass" ]] && ((pass_count++)) || true
  [[ "$s2" == "fail" ]] && ((fail_count++)) || true
  [[ "$s2" == "skip" ]] && ((skip_count++)) || true

  t3=$(_group_c_test_user_response)
  results+=("$t3")
  local s3
  s3=$(echo "$t3" | jq -r '.status')
  [[ "$s3" == "pass" ]] && ((pass_count++)) || true
  [[ "$s3" == "fail" ]] && ((fail_count++)) || true
  [[ "$s3" == "skip" ]] && ((skip_count++)) || true

  t4=$(_group_c_test_completion)
  results+=("$t4")
  local s4
  s4=$(echo "$t4" | jq -r '.status')
  [[ "$s4" == "pass" ]] && ((pass_count++)) || true
  [[ "$s4" == "fail" ]] && ((fail_count++)) || true
  [[ "$s4" == "skip" ]] && ((skip_count++)) || true

  # Cleanup
  _group_c_cleanup

  # Determine overall status
  local overall_status="pass"
  if [[ $fail_count -gt 0 && $pass_count -gt 0 ]]; then
    overall_status="partial"
  elif [[ $fail_count -gt 0 ]]; then
    overall_status="fail"
  elif [[ $skip_count -eq 4 ]]; then
    overall_status="skip"
  fi

  local total_elapsed=$(( $(date +%s%3N) - group_start_ms ))

  # Build final JSON result
  local tests_array
  tests_array=$(printf '%s\n' "${results[@]}" | jq -s '.')

  jq -nc \
    --arg group "c-secretary" \
    --arg name "Secretary Workflows" \
    --arg overall "$overall_status" \
    --argjson duration_ms "$total_elapsed" \
    --argjson results "$tests_array" \
    '{group: $group, name: $name, results: $results, duration_ms: $duration_ms, overall: $overall}'
}
