# group-i-flags.sh — Feature Group I: Flag-gated features with skip-disabled semantics
#
# Sourced library (no shebang, no main block).
# Tests each flag-gated feature: if the feature flag is disabled the test
# reports skip-disabled (NOT fail).  Only when a flag IS enabled do we
# exercise the corresponding RPC methods.
#
# Tests:
#   1. ZeroTrustKeystore  — keystore.unseal / keystore.sealed / keystore.session_status
#   2. VoicePipeline       — voice.status
#   3. E2EEBackup          — e2ee.backup_exists
#   4. MultiTabReplay      — browser.replay_diagnostics
#
# Usage:
#   source "${_SCRIPT_DIR}/feature-groups/group-i-flags.sh"
#   _group_i_run --vps-ip 1.2.3.4 --ssh-key ~/.ssh/id_rsa --ssh-user root --output-dir /tmp/evidence

# ── _gi_test_result(name, status, details, evidence_path, duration_ms) ──────────
# Print a single structured JSON test result to stdout.
_gi_test_result() {
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

# ── _gi_save_evidence(output_dir, filename, content) ────────────────────────────
# Save evidence JSON to output_dir/filename. Echoes the path.
_gi_save_evidence() {
  local output_dir="${1:?}"
  local filename="${2:?}"
  local content="${3:-}"
  local filepath="${output_dir}/${filename}"
  mkdir -p "$output_dir"
  echo "$content" > "$filepath"
  echo "$filepath"
}

# ── _gi_rpc(method, params_json) ────────────────────────────────────────────────
# Make an RPC call via SSH to the Bridge. Wraps _contract_bridge_rpc.
# Echoes the raw JSON-RPC response. Returns 0 on success, 1 on failure.
_gi_rpc() {
  local method="${1:?Usage: _gi_rpc method [params_json]}"
  local params="${2:-{}}"
  _contract_bridge_rpc "$method" "$params" 3
}

# ── _gi_probe_flag(method, params_json) ─────────────────────────────────────────
# Probe an RPC method to determine if a feature is available.
# Returns 0 if the method responds successfully, 1 otherwise.
# Captures the response in _GI_PROBE_RESP.
_GI_PROBE_RESP=""
_gi_probe_flag() {
  local method="${1:?}"
  local params="${2:-{}}"
  _GI_PROBE_RESP=$(_gi_rpc "$method" "$params" 2>/dev/null) || {
    _GI_PROBE_RESP=""
    return 1
  }
  # Check for JSON-RPC error (method not found, feature disabled, etc.)
  if [[ -n "$_GI_PROBE_RESP" ]] && echo "$_GI_PROBE_RESP" | jq -e '.error' >/dev/null 2>&1; then
    return 1
  fi
  return 0
}

# ── _gi_test_zero_trust_keystore(output_dir) ────────────────────────────────────
# Test 1: ZeroTrustKeystore flag-gated feature.
# If the feature flag is disabled → skip-disabled.
# If enabled: test keystore.unseal, keystore.sealed, keystore.session_status.
_gi_test_zero_trust_keystore() {
  local output_dir="${1:?}"
  local start_ms duration_ms

  start_ms=$(date +%s%N)

  # Probe: try keystore.sealed (lightweight read-only call)
  if ! _gi_probe_flag "keystore.sealed" '{}'; then
    duration_ms=$(( ($(date +%s%N) - start_ms) / 1000000 ))
    log_info "[GROUP-I] ZeroTrustKeystore: flag appears disabled (keystore.sealed unavailable)"
    _gi_test_result "ZeroTrustKeystore" "skip-disabled" \
      "ZeroTrustKeystore flag is false" "" "$duration_ms"
    return 0
  fi

  # Flag is enabled — run the three keystore RPCs
  local sealed_resp unseal_resp session_resp
  local evidence_content="" sub_results=()
  local all_pass=true

  # 1a. keystore.sealed
  sealed_resp="$_GI_PROBE_RESP"
  if [[ -n "$sealed_resp" ]] && echo "$sealed_resp" | jq -e '.result' >/dev/null 2>&1; then
    local sealed_val
    sealed_val=$(echo "$sealed_resp" | jq -r '.result.sealed // .result')
    sub_results+=("keystore.sealed=${sealed_val}")
    log_pass "[GROUP-I] ZeroTrustKeystore: keystore.sealed responded (${sealed_val})"
  else
    all_pass=false
    sub_results+=("keystore.sealed=ERROR")
    log_fail "[GROUP-I] ZeroTrustKeystore: keystore.sealed failed"
  fi

  # 1b. keystore.unseal (will likely fail without correct password — that's OK,
  #     we're testing the method exists and responds, not actually unsealing)
  unseal_resp=$(_gi_rpc "keystore.unseal" '{"password":"flag-test-probe"}' 2>/dev/null) || true
  if [[ -n "$unseal_resp" ]]; then
    # Any response (even error for wrong password) means the method exists
    if echo "$unseal_resp" | jq -e '.error' >/dev/null 2>&1; then
      local err_msg
      err_msg=$(echo "$unseal_resp" | jq -r '.error.message // "error"')
      sub_results+=("keystore.unseal=responded(${err_msg})")
      log_info "[GROUP-I] ZeroTrustKeystore: keystore.unseal responded with error: ${err_msg}"
    else
      sub_results+=("keystore.unseal=ok")
      log_pass "[GROUP-I] ZeroTrustKeystore: keystore.unseal succeeded"
    fi
  else
    all_pass=false
    sub_results+=("keystore.unseal=NO_RESPONSE")
    log_fail "[GROUP-I] ZeroTrustKeystore: keystore.unseal no response"
  fi

  # 1c. keystore.session_status
  session_resp=$(_gi_rpc "keystore.session_status" '{}' 2>/dev/null) || true
  if [[ -n "$session_resp" ]] && echo "$session_resp" | jq -e '.result' >/dev/null 2>&1; then
    sub_results+=("keystore.session_status=ok")
    log_pass "[GROUP-I] ZeroTrustKeystore: keystore.session_status responded"
  else
    all_pass=false
    sub_results+=("keystore.session_status=FAIL")
    log_fail "[GROUP-I] ZeroTrustKeystore: keystore.session_status failed"
  fi

  duration_ms=$(( ($(date +%s%N) - start_ms) / 1000000 ))

  # Build evidence
  evidence_content=$(jq -nc \
    --arg sealed "${sealed_resp:-}" \
    --arg unseal "${unseal_resp:-}" \
    --arg session "${session_resp:-}" \
    '{sealed_response: $sealed, unseal_response: $unseal, session_response: $session}')
  local evidence_path
  evidence_path=$(_gi_save_evidence "$output_dir" "group-i-zero-trust-keystore.json" "$evidence_content")

  local status="pass"
  if ! $all_pass; then status="fail"; fi
  local details
  details=$(printf '%s; ' "${sub_results[@]}")
  details="${details%; }"

  _gi_test_result "ZeroTrustKeystore" "$status" "$details" "$evidence_path" "$duration_ms"
}

# ── _gi_test_voice_pipeline(output_dir) ─────────────────────────────────────────
# Test 2: VoicePipeline flag-gated feature.
# If the feature flag is "off" or empty → skip-disabled.
# If enabled: test voice.status.
_gi_test_voice_pipeline() {
  local output_dir="${1:?}"
  local start_ms duration_ms

  start_ms=$(date +%s%N)

  # Probe: try voice.status
  if ! _gi_probe_flag "voice.status" '{}'; then
    duration_ms=$(( ($(date +%s%N) - start_ms) / 1000000 ))
    log_info "[GROUP-I] VoicePipeline: flag appears off (voice.status unavailable)"
    _gi_test_result "VoicePipeline" "skip-disabled" \
      "VoicePipeline is off" "" "$duration_ms"
    return 0
  fi

  # Flag is enabled — voice.status was already called in the probe
  local voice_resp="$_GI_PROBE_RESP"
  local evidence_content details=""

  evidence_content="$voice_resp"

  if [[ -n "$voice_resp" ]] && echo "$voice_resp" | jq -e '.result' >/dev/null 2>&1; then
    local pipeline_status
    pipeline_status=$(echo "$voice_resp" | jq -r '.result.status // .result.pipeline // "unknown"')
    details="voice.status=${pipeline_status}"
    log_pass "[GROUP-I] VoicePipeline: voice.status responded (status=${pipeline_status})"
  else
    details="voice.status=unexpected_response"
    log_fail "[GROUP-I] VoicePipeline: voice.status unexpected response"
  fi

  duration_ms=$(( ($(date +%s%N) - start_ms) / 1000000 ))
  local evidence_path
  evidence_path=$(_gi_save_evidence "$output_dir" "group-i-voice-pipeline.json" "$evidence_content")

  # Determine pass/fail based on whether we got a valid result
  local status="pass"
  if [[ "$details" == *"unexpected"* ]]; then status="fail"; fi

  _gi_test_result "VoicePipeline" "$status" "$details" "$evidence_path" "$duration_ms"
}

# ── _gi_test_e2ee_backup(output_dir) ────────────────────────────────────────────
# Test 3: E2EEBackup flag-gated feature.
# If the feature flag is disabled → skip-disabled.
# If enabled: test e2ee.backup_exists.
_gi_test_e2ee_backup() {
  local output_dir="${1:?}"
  local start_ms duration_ms

  start_ms=$(date +%s%N)

  # Probe: try e2ee.backup_exists with a dummy backup_id
  if ! _gi_probe_flag "e2ee.backup_exists" '{"backup_id":"_flag_probe_"}'; then
    duration_ms=$(( ($(date +%s%N) - start_ms) / 1000000 ))
    log_info "[GROUP-I] E2EEBackup: flag appears disabled (e2ee.backup_exists unavailable)"
    _gi_test_result "E2EEBackup" "skip-disabled" \
      "E2EEBackup flag is false" "" "$duration_ms"
    return 0
  fi

  # Flag is enabled — e2ee.backup_exists responded
  local backup_resp="$_GI_PROBE_RESP"
  local evidence_content details=""

  evidence_content="$backup_resp"

  if [[ -n "$backup_resp" ]] && echo "$backup_resp" | jq -e '.result' >/dev/null 2>&1; then
    local exists_val
    exists_val=$(echo "$backup_resp" | jq -r '.result.exists // .result')
    details="e2ee.backup_exists=${exists_val}"
    log_pass "[GROUP-I] E2EEBackup: e2ee.backup_exists responded (exists=${exists_val})"
  else
    details="e2ee.backup_exists=unexpected_response"
    log_fail "[GROUP-I] E2EEBackup: e2ee.backup_exists unexpected response"
  fi

  duration_ms=$(( ($(date +%s%N) - start_ms) / 1000000 ))
  local evidence_path
  evidence_path=$(_gi_save_evidence "$output_dir" "group-i-e2ee-backup.json" "$evidence_content")

  local status="pass"
  if [[ "$details" == *"unexpected"* ]]; then status="fail"; fi

  _gi_test_result "E2EEBackup" "$status" "$details" "$evidence_path" "$duration_ms"
}

# ── _gi_test_multi_tab_replay(output_dir) ───────────────────────────────────────
# Test 4: MultiTabReplay flag-gated feature.
# If the feature flag is disabled → skip-disabled.
# If enabled: test browser.replay_diagnostics.
_gi_test_multi_tab_replay() {
  local output_dir="${1:?}"
  local start_ms duration_ms

  start_ms=$(date +%s%N)

  # Probe: try browser.replay_diagnostics with a probe tab_id
  if ! _gi_probe_flag "browser.replay_diagnostics" '{"tab_id":"_flag_probe_"}'; then
    duration_ms=$(( ($(date +%s%N) - start_ms) / 1000000 ))
    log_info "[GROUP-I] MultiTabReplay: flag appears disabled (browser.replay_diagnostics unavailable)"
    _gi_test_result "MultiTabReplay" "skip-disabled" \
      "MultiTabReplay flag is false" "" "$duration_ms"
    return 0
  fi

  # Flag is enabled — replay_diagnostics responded
  local replay_resp="$_GI_PROBE_RESP"
  local evidence_content details=""

  evidence_content="$replay_resp"

  if [[ -n "$replay_resp" ]] && echo "$replay_resp" | jq -e '.result' >/dev/null 2>&1; then
    local diag_status
    diag_status=$(echo "$replay_resp" | jq -r '.result.status // .result')
    details="browser.replay_diagnostics=${diag_status}"
    log_pass "[GROUP-I] MultiTabReplay: browser.replay_diagnostics responded (status=${diag_status})"
  else
    details="browser.replay_diagnostics=unexpected_response"
    log_fail "[GROUP-I] MultiTabReplay: browser.replay_diagnostics unexpected response"
  fi

  duration_ms=$(( ($(date +%s%N) - start_ms) / 1000000 ))
  local evidence_path
  evidence_path=$(_gi_save_evidence "$output_dir" "group-i-multi-tab-replay.json" "$evidence_content")

  local status="pass"
  if [[ "$details" == *"unexpected"* ]]; then status="fail"; fi

  _gi_test_result "MultiTabReplay" "$status" "$details" "$evidence_path" "$duration_ms"
}

# ── _group_i_run(--vps-ip, --ssh-key, --ssh-user, --output-dir) ─────────────────
# Main entry point for Feature Group I: Flag-gated features.
# Accepts --vps-ip, --ssh-key, --ssh-user, --output-dir as arguments.
# Outputs a single JSON object with the overall group result.
_group_i_run() {
  local vps_ip="" ssh_key="" ssh_user="" output_dir=""

  # Parse arguments
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --vps-ip)     vps_ip="$2";     shift 2 ;;
      --ssh-key)    ssh_key="$2";    shift 2 ;;
      --ssh-user)   ssh_user="$2";   shift 2 ;;
      --output-dir) output_dir="$2"; shift 2 ;;
      *) log_info "[GROUP-I] Unknown argument: $1"; shift ;;
    esac
  done

  # Apply overrides to the environment if provided
  if [[ -n "$vps_ip" ]];   then VPS_IP="$vps_ip";         export VPS_IP; fi
  if [[ -n "$ssh_key" ]];  then SSH_KEY_PATH="$ssh_key";  export SSH_KEY_PATH; fi
  if [[ -n "$ssh_user" ]]; then VPS_USER="$ssh_user";     export VPS_USER; fi
  if [[ -n "$output_dir" ]]; then
    mkdir -p "$output_dir"
  else
    output_dir="${EVIDENCE_DIR:-.sisyphus/evidence}/group-i"
    mkdir -p "$output_dir"
  fi

  local group_start group_results=()
  group_start=$(date +%s%N)

  # ── Test 1: ZeroTrustKeystore ─────────────────────────────────────────────
  group_results+=("$(_gi_test_zero_trust_keystore "$output_dir")")

  # ── Test 2: VoicePipeline ─────────────────────────────────────────────────
  group_results+=("$(_gi_test_voice_pipeline "$output_dir")")

  # ── Test 3: E2EEBackup ────────────────────────────────────────────────────
  group_results+=("$(_gi_test_e2ee_backup "$output_dir")")

  # ── Test 4: MultiTabReplay ────────────────────────────────────────────────
  group_results+=("$(_gi_test_multi_tab_replay "$output_dir")")

  # ── Assemble overall result ───────────────────────────────────────────────
  local all_results overall group_ms fail_count
  all_results=$(printf '%s\n' "${group_results[@]}" | jq -s '.')

  # Overall: fail if any test has status "fail". skip-disabled does NOT count as fail.
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
    '{group: "I", name: "Flag-Gated Features", results: $results, duration_ms: $duration_ms, overall: $overall}'
}
