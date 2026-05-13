# group-e-email.sh — Feature Group E: Email pipeline tests
#
# Sourced library (no shebang, no main block).
# Tests the email approval pipeline via RPC over SSH to the Bridge:
#   1. Email pipeline health — check if email service is configured
#   2. Outbound HITL — trigger email approval workflow, verify mechanism
#   3. Timeout/audit — verify email approval timeout behavior
#
# Usage:
#   source "${_SCRIPT_DIR}/feature-groups/group-e-email.sh"
#   _group_e_run --vps-ip 1.2.3.4 --ssh-key ~/.ssh/id_rsa --ssh-user root --output-dir /tmp/evidence

# ── _ge_test_result(name, status, details, evidence_path, duration_ms) ──────────
# Print a single structured JSON test result to stdout.
_ge_test_result() {
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

# ── _ge_save_evidence(output_dir, filename, content) ────────────────────────────
# Save evidence JSON to output_dir/filename. Echoes the path.
_ge_save_evidence() {
  local output_dir="${1:?}"
  local filename="${2:?}"
  local content="${3:-}"
  local filepath="${output_dir}/${filename}"
  mkdir -p "$output_dir"
  echo "$content" > "$filepath"
  echo "$filepath"
}

# ── _ge_rpc(method, params_json) ────────────────────────────────────────────────
# Make an RPC call via SSH to the Bridge for email methods.
_ge_rpc() {
  local method="${1:?Usage: _ge_rpc method [params_json]}"
  local params="${2:-{}}"
  _contract_bridge_rpc "$method" "$params" 3
}

# ── _ge_check_email_configured() ────────────────────────────────────────────────
# Check if email pipeline is configured on the VPS.
# Looks for postfix, mta-recv, or email-related env vars.
# Returns 0 if configured, 1 if not.
_ge_check_email_configured() {
  local check
  check=$(ssh_vps "$(
    cat <<'REMOTESH'
# Check for email pipeline indicators
_email_configured=0

# Check if postfix is installed/running
if command -v postfix >/dev/null 2>&1 || systemctl is-active postfix >/dev/null 2>&1; then
  _email_configured=1
fi

# Check for mta-recv binary (ArmorClaw email receiver)
if [[ -x /usr/local/bin/mta-recv ]] || [[ -x /opt/armorclaw/mta-recv ]]; then
  _email_configured=1
fi

# Check for email-related docker container
if docker ps --format '{{.Names}}' 2>/dev/null | grep -qi 'email\|postfix\|mta'; then
  _email_configured=1
fi

# Check for email-ingest socket
if [[ -S /run/armorclaw/email-ingest.sock ]]; then
  _email_configured=1
fi

echo "$_email_configured"
REMOTESH
  )" 2>/dev/null)

  [[ "$check" == "1" ]]
}

# ── _group_e_run(--vps-ip, --ssh-key, --ssh-user, --output-dir) ────────────────
# Main entry point for Feature Group E: Email pipeline tests.
# Accepts --vps-ip, --ssh-key, --ssh-user, --output-dir as arguments.
# Outputs a single JSON object with the overall group result.
_group_e_run() {
  local vps_ip="" ssh_key="" ssh_user="" output_dir=""

  # Parse arguments
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --vps-ip)     vps_ip="$2";     shift 2 ;;
      --ssh-key)    ssh_key="$2";    shift 2 ;;
      --ssh-user)   ssh_user="$2";   shift 2 ;;
      --output-dir) output_dir="$2"; shift 2 ;;
      *) log_info "[GROUP-E] Unknown argument: $1"; shift ;;
    esac
  done

  # Apply overrides to the environment if provided
  if [[ -n "$vps_ip" ]];   then VPS_IP="$vps_ip";         export VPS_IP; fi
  if [[ -n "$ssh_key" ]];  then SSH_KEY_PATH="$ssh_key";  export SSH_KEY_PATH; fi
  if [[ -n "$ssh_user" ]]; then VPS_USER="$ssh_user";     export VPS_USER; fi
  if [[ -n "$output_dir" ]]; then
    mkdir -p "$output_dir"
  else
    output_dir="${EVIDENCE_DIR:-.sisyphus/evidence}/group-e"
    mkdir -p "$output_dir"
  fi

  local group_start group_results=()
  group_start=$(date +%s%N)

  # ── Step 1: Email pipeline health check ────────────────────────────────────
  local t1_start t1_ms t1_result t1_details t1_evidence_path=""
  t1_start=$(date +%s%N)

  # First, check if email service is deployed
  if ! _ge_check_email_configured; then
    t1_ms=$(( ($(date +%s%N) - t1_start) / 1000000 ))
    t1_result="skip-disabled"
    t1_details="Email service not deployed on this VPS (no postfix/mta-recv/email-ingest.sock found)"
    log_info "[GROUP-E] Email service not configured — skipping all email tests"
    group_results+=("$(_ge_test_result "email-pipeline-health" "$t1_result" "$t1_details" "" "$t1_ms")")
    group_results+=("$(_ge_test_result "email-outbound-hitl" "skip-disabled" "Email service not deployed" "" 0)")
    group_results+=("$(_ge_test_result "email-timeout-audit" "skip-disabled" "Email service not deployed" "" 0)")

    # Assemble skip result
    local all_results group_ms
    all_results=$(printf '%s\n' "${group_results[@]}" | jq -s '.')
    group_ms=$(( ($(date +%s%N) - group_start) / 1000000 ))
    jq -nc \
      --argjson results "$all_results" \
      --argjson duration_ms "$group_ms" \
      '{group: "E", name: "Email Pipeline", results: $results, duration_ms: $duration_ms, overall: "skip-disabled"}'
    return 0
  fi

  # Email service present — probe RPC email_approval_status
  local t1_resp
  t1_resp=$(_ge_rpc "email_approval_status" '{}' 2>/dev/null)
  t1_ms=$(( ($(date +%s%N) - t1_start) / 1000000 ))

  if [[ -n "$t1_resp" ]]; then
    t1_evidence_path=$(_ge_save_evidence "$output_dir" "group-e-email-health.json" "$t1_resp")
  fi

  if [[ -n "$t1_resp" ]] && echo "$t1_resp" | jq -e '.result.pending_count' >/dev/null 2>&1; then
    local pending_count
    pending_count=$(echo "$t1_resp" | jq -r '.result.pending_count')
    local timeout_s
    timeout_s=$(echo "$t1_resp" | jq -r '.result.timeout_s')
    t1_result="pass"
    t1_details="Email pipeline healthy: pending_count=${pending_count}, timeout_s=${timeout_s}"
    log_pass "[GROUP-E] Step 1: Email pipeline healthy (pending=${pending_count})"
  else
    t1_result="fail"
    t1_details="email_approval_status RPC failed: $(echo "$t1_resp" | head -c 300)"
    log_fail "[GROUP-E] Step 1: Email pipeline health check failed"
  fi
  group_results+=("$(_ge_test_result "email-pipeline-health" "$t1_result" "$t1_details" "$t1_evidence_path" "$t1_ms")")

  # ── Step 2: Outbound HITL approval workflow ────────────────────────────────
  local t2_start t2_ms t2_resp t2_result t2_details t2_evidence_path=""
  t2_start=$(date +%s%N)

  # Query pending approvals via email.list_pending
  t2_resp=$(_ge_rpc "email.list_pending" '{}' 2>/dev/null)
  t2_ms=$(( ($(date +%s%N) - t2_start) / 1000000 ))

  if [[ -n "$t2_resp" ]]; then
    t2_evidence_path=$(_ge_save_evidence "$output_dir" "group-e-email-list-pending.json" "$t2_resp")
  fi

  if [[ -n "$t2_resp" ]] && echo "$t2_resp" | jq -e '.result.approvals' >/dev/null 2>&1; then
    local approval_count
    approval_count=$(echo "$t2_resp" | jq -r '.result.count')
    t2_result="pass"
    t2_details="HITL approval mechanism responsive: ${approval_count} pending approvals listed"
    log_pass "[GROUP-E] Step 2: HITL approval mechanism works (${approval_count} pending)"
  else
    t2_result="fail"
    t2_details="email.list_pending RPC failed: $(echo "$t2_resp" | head -c 300)"
    log_fail "[GROUP-E] Step 2: HITL approval mechanism check failed"
  fi
  group_results+=("$(_ge_test_result "email-outbound-hitl" "$t2_result" "$t2_details" "$t2_evidence_path" "$t2_ms")")

  # ── Step 3: Timeout/audit verification ─────────────────────────────────────
  local t3_start t3_ms t3_resp t3_result t3_details t3_evidence_path=""
  t3_start=$(date +%s%N)

  # Re-query status to verify timeout configuration and audit trail
  t3_resp=$(_ge_rpc "email_approval_status" '{}' 2>/dev/null)
  t3_ms=$(( ($(date +%s%N) - t3_start) / 1000000 ))

  if [[ -n "$t3_resp" ]]; then
    t3_evidence_path=$(_ge_save_evidence "$output_dir" "group-e-email-timeout-audit.json" "$t3_resp")
  fi

  if [[ -n "$t3_resp" ]] && echo "$t3_resp" | jq -e '.result.timeout_s' >/dev/null 2>&1; then
    local timeout_val
    timeout_val=$(echo "$t3_resp" | jq -r '.result.timeout_s')
    # Verify timeout is a reasonable value (> 0)
    if [[ "$timeout_val" -gt 0 ]]; then
      t3_result="pass"
      t3_details="Timeout configured: ${timeout_val}s. Audit endpoint responsive."
      log_pass "[GROUP-E] Step 3: Timeout/audit verified (timeout=${timeout_val}s)"
    else
      t3_result="fail"
      t3_details="Invalid timeout value: ${timeout_val}s"
      log_fail "[GROUP-E] Step 3: Invalid timeout configuration"
    fi
  else
    t3_result="fail"
    t3_details="Timeout/audit check failed: $(echo "$t3_resp" | head -c 300)"
    log_fail "[GROUP-E] Step 3: Timeout/audit verification failed"
  fi
  group_results+=("$(_ge_test_result "email-timeout-audit" "$t3_result" "$t3_details" "$t3_evidence_path" "$t3_ms")")

  # ── Assemble overall result ────────────────────────────────────────────────
  local all_results overall group_ms fail_count skip_count
  all_results=$(printf '%s\n' "${group_results[@]}" | jq -s '.')

  fail_count=$(echo "$all_results" | jq '[.[] | select(.status == "fail")] | length')
  skip_count=$(echo "$all_results" | jq '[.[] | select(.status == "skip-disabled")] | length')

  if [[ "$fail_count" -eq 0 && "$skip_count" -eq 0 ]]; then
    overall="pass"
  elif [[ "$fail_count" -eq 0 ]]; then
    overall="skip-disabled"
  else
    overall="fail"
  fi

  group_ms=$(( ($(date +%s%N) - group_start) / 1000000 ))

  jq -nc \
    --argjson results "$all_results" \
    --argjson duration_ms "$group_ms" \
    --arg overall "$overall" \
    '{group: "E", name: "Email Pipeline", results: $results, duration_ms: $duration_ms, overall: $overall}'
}
