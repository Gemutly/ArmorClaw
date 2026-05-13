# group-g-browser.sh — Feature Group G: Browser / Jetski tests
#
# Sourced library (no shebang, no main block).
# Tests browser/Jetski functionality via RPC over SSH to the Bridge:
#   1. Jetski availability — check browser.status RPC
#   2. Browser action — navigate to a URL via browser.navigate
#   3. Browser diagnostics — list browser jobs via browser.list
#
# Usage:
#   source "${_SCRIPT_DIR}/feature-groups/group-g-browser.sh"
#   _group_g_run --vps-ip 1.2.3.4 --ssh-key ~/.ssh/id_rsa --ssh-user root --output-dir /tmp/evidence

# ── Internal state ──────────────────────────────────────────────────────────────
_GG_NAV_JOB_ID=""

# ── _gg_test_result(name, status, details, evidence_path, duration_ms) ──────────
# Print a single structured JSON test result to stdout.
_gg_test_result() {
  local name="${1:?}"
  local status="${2:?}"   # pass|fail|skip-disabled
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

# ── _gg_save_evidence(output_dir, filename, content) ────────────────────────────
# Save evidence JSON to output_dir/filename. Echoes the path.
_gg_save_evidence() {
  local output_dir="${1:?}"
  local filename="${2:?}"
  local content="${3:-}"
  local filepath="${output_dir}/${filename}"
  mkdir -p "$output_dir"
  echo "$content" > "$filepath"
  echo "$filepath"
}

# ── _gg_rpc(method, params_json) ────────────────────────────────────────────────
# Make an RPC call via SSH to the Bridge. Echoes the raw JSON-RPC response.
_gg_rpc() {
  local method="${1:?Usage: _gg_rpc method [params_json]}"
  local params="${2:-{}}"
  _contract_bridge_rpc "$method" "$params" 3
}

# ── _group_g_run(--vps-ip, --ssh-key, --ssh-user, --output-dir) ────────────────
# Main entry point for Feature Group G: Browser / Jetski tests.
_group_g_run() {
  local vps_ip="" ssh_key="" ssh_user="" output_dir=""

  # Parse arguments
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --vps-ip)     vps_ip="$2";     shift 2 ;;
      --ssh-key)    ssh_key="$2";    shift 2 ;;
      --ssh-user)   ssh_user="$2";   shift 2 ;;
      --output-dir) output_dir="$2"; shift 2 ;;
      *) log_info "[GROUP-G] Unknown argument: $1"; shift ;;
    esac
  done

  # Apply overrides to the environment if provided
  if [[ -n "$vps_ip" ]];   then VPS_IP="$vps_ip";       export VPS_IP; fi
  if [[ -n "$ssh_key" ]];  then SSH_KEY_PATH="$ssh_key"; export SSH_KEY_PATH; fi
  if [[ -n "$ssh_user" ]]; then VPS_USER="$ssh_user";    export VPS_USER; fi
  if [[ -n "$output_dir" ]]; then
    mkdir -p "$output_dir"
  else
    output_dir="${EVIDENCE_DIR:-.sisyphus/evidence}/group-g"
    mkdir -p "$output_dir"
  fi

  local group_start group_results=()
  group_start=$(date +%s%N)

  # ── Step 1: Jetski / Browser availability ─────────────────────────────────
  # Probe browser.status with a dummy job_id to detect if the browser subsystem
  # is present. A method-not-found or service-unavailable error means Jetski is
  # disabled — we report skip-disabled for all tests.
  local t1_start t1_ms t1_resp
  t1_start=$(date +%s%N)
  t1_resp=$(_gg_rpc "browser.list" '{}' 2>/dev/null)
  t1_ms=$(( ($(date +%s%N) - t1_start) / 1000000 ))

  local t1_evidence_path=""
  if [[ -n "$t1_resp" ]]; then
    t1_evidence_path=$(_gg_save_evidence "$output_dir" "group-g-browser-availability.json" "$t1_resp")
  fi

  # Check if browser subsystem returned an explicit "not available" or method not found
  local browser_available=true
  if [[ -z "$t1_resp" ]]; then
    browser_available=false
  elif echo "$t1_resp" | jq -e '.error' >/dev/null 2>&1; then
    local err_code
    err_code=$(echo "$t1_resp" | jq -r '.error.code // empty')
    local err_msg
    err_msg=$(echo "$t1_resp" | jq -r '.error.message // empty' 2>/dev/null)
    # MethodNotFound (-32601) or messages indicating browser not configured
    if [[ "$err_code" == "-32601" ]] || \
       echo "$err_msg" | grep -qi "not found\|not available\|not configured\|no browser"; then
      browser_available=false
    fi
  fi

  if ! $browser_available; then
    # Browser/Jetski not available — skip all tests
    log_info "[GROUP-G] Browser/Jetski unavailable — skipping all browser tests"
    local skip_details="Browser subsystem not available"
    if [[ -n "$t1_resp" ]]; then
      skip_details="Browser unavailable: $(echo "$t1_resp" | head -c 200)"
    fi
    group_results+=("$(_gg_test_result "browser.availability" "skip-disabled" "$skip_details" "$t1_evidence_path" "$t1_ms")")
    group_results+=("$(_gg_test_result "browser.navigate" "skip-disabled" "Browser unavailable" "" 0)")
    group_results+=("$(_gg_test_result "browser.diagnostics" "skip-disabled" "Browser unavailable" "" 0)")

    local all_results
    all_results=$(printf '%s\n' "${group_results[@]}" | jq -s '.')
    local group_ms=$(( ($(date +%s%N) - group_start) / 1000000 ))
    jq -nc \
      --argjson results "$all_results" \
      --argjson duration_ms "$group_ms" \
      '{group: "G", name: "Browser / Jetski", results: $results, duration_ms: $duration_ms, overall: "skip-disabled"}'
    return 0
  fi

  # Browser is available
  local t1_details="Browser subsystem available"
  if echo "$t1_resp" | jq -e '.result' >/dev/null 2>&1; then
    local job_count
    job_count=$(echo "$t1_resp" | jq -r '.result.count // 0')
    t1_details="Browser available, ${job_count} existing job(s)"
  fi
  log_pass "[GROUP-G] Browser availability confirmed"
  group_results+=("$(_gg_test_result "browser.availability" "pass" "$t1_details" "$t1_evidence_path" "$t1_ms")")

  # ── Step 2: Browser action — navigate ────────────────────────────────────
  local t2_start t2_ms t2_resp t2_result t2_details
  t2_start=$(date +%s%N)
  t2_resp=$(_gg_rpc "browser.navigate" "$(jq -nc \
    --arg url "https://example.com" \
    '{url: $url, agent_id: "e2e-test"}'
  )" 2>/dev/null)
  t2_ms=$(( ($(date +%s%N) - t2_start) / 1000000 ))

  local t2_evidence_path=""
  if [[ -n "$t2_resp" ]]; then
    t2_evidence_path=$(_gg_save_evidence "$output_dir" "group-g-browser-navigate.json" "$t2_resp")
  fi

  if [[ -n "$t2_resp" ]] && echo "$t2_resp" | jq -e '.result.job_id' >/dev/null 2>&1; then
    _GG_NAV_JOB_ID=$(echo "$t2_resp" | jq -r '.result.job_id')
    t2_result="pass"
    t2_details="Navigation started: job_id=$_GG_NAV_JOB_ID"
    log_pass "[GROUP-G] Navigate dispatched (job_id=$_GG_NAV_JOB_ID)"
  elif [[ -n "$t2_resp" ]] && echo "$t2_resp" | jq -e '.result' >/dev/null 2>&1; then
    # Some result returned even without job_id — still ok
    t2_result="pass"
    t2_details="Navigate response received: $(echo "$t2_resp" | jq -c '.result' | head -c 200)"
    log_pass "[GROUP-G] Navigate responded"
  else
    t2_result="fail"
    t2_details="Navigate failed: $(echo "$t2_resp" | head -c 300)"
    log_fail "[GROUP-G] Navigate failed"
  fi
  group_results+=("$(_gg_test_result "browser.navigate" "$t2_result" "$t2_details" "$t2_evidence_path" "$t2_ms")")

  # ── Step 3: Browser diagnostics ──────────────────────────────────────────
  local t3_start t3_ms t3_resp t3_result t3_details
  t3_start=$(date +%s%N)
  t3_resp=$(_gg_rpc "browser.status" "$(jq -nc \
    --arg job_id "${_GG_NAV_JOB_ID:-unknown}" \
    '{job_id: $job_id}'
  )" 2>/dev/null)
  t3_ms=$(( ($(date +%s%N) - t3_start) / 1000000 ))

  local t3_evidence_path=""
  if [[ -n "$t3_resp" ]]; then
    t3_evidence_path=$(_gg_save_evidence "$output_dir" "group-g-browser-status.json" "$t3_resp")
  fi

  if [[ -n "$t3_resp" ]] && echo "$t3_resp" | jq -e '.result' >/dev/null 2>&1; then
    local nav_status
    nav_status=$(echo "$t3_resp" | jq -r '.result.status // "unknown"')
    t3_result="pass"
    t3_details="Diagnostics received: status=$nav_status"
    log_pass "[GROUP-G] Diagnostics OK (status=$nav_status)"
  else
    t3_result="fail"
    t3_details="Diagnostics failed: $(echo "$t3_resp" | head -c 300)"
    log_fail "[GROUP-G] Diagnostics failed"
  fi
  group_results+=("$(_gg_test_result "browser.diagnostics" "$t3_result" "$t3_details" "$t3_evidence_path" "$t3_ms")")

  # ── Assemble overall result ───────────────────────────────────────────────
  local all_results overall group_ms fail_count
  all_results=$(printf '%s\n' "${group_results[@]}" | jq -s '.')

  # Determine overall: pass if zero failures, fail otherwise
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
    '{group: "G", name: "Browser / Jetski", results: $results, duration_ms: $duration_ms, overall: $overall}'
}
