#!/usr/bin/env bash
# a0_investigate.sh — Investigate A0 103/0 mismatch
#
# Probes every registered RPC method with multiple timeout budgets and
# both HTTP/HTTPS transports, classifies each method's root cause, and
# produces a structured JSON report with dominant_root_cause.
#
# Usage:
#   bash scripts/a0_investigate.sh
#   bash scripts/a0_investigate.sh --vps-ip 1.2.3.4 --ssh-key ~/.ssh/mykey
#
# Environment (or .env):
#   VPS_IP, VPS_USER, BRIDGE_PORT, MATRIX_PORT, SSH_KEY_PATH
#
# Output:
#   .sisyphus/evidence/a0-investigate/{method}.json   per-method findings
#   .sisyphus/evidence/task-6-investigation-summary.json  final report

set -uo pipefail

# ── Script directory & repo root ─────────────────────────────────────────────
_SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
_REPO_ROOT="$(cd "${_SCRIPT_DIR}/.." && pwd)"

# ── Parse CLI args (override env) ────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --vps-ip)       VPS_IP="$2";       shift 2 ;;
    --vps-user)     VPS_USER="$2";     shift 2 ;;
    --bridge-port)  BRIDGE_PORT="$2";  shift 2 ;;
    --ssh-key)      SSH_KEY_PATH="$2"; shift 2 ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

# ── Source infrastructure (loads .env, exports VPS_*, provides ssh_vps()) ────
source "${_REPO_ROOT}/tests/lib/load_env.sh"
source "${_REPO_ROOT}/tests/lib/common_output.sh"

# ── Constants ────────────────────────────────────────────────────────────────
EVIDENCE_DIR="${_REPO_ROOT}/.sisyphus/evidence/a0-investigate"
SUMMARY_PATH="${_REPO_ROOT}/.sisyphus/evidence/task-6-investigation-summary.json"
TIMEOUTS=(5 10 30)
TRANSPORTS=("https" "http")
RPC_PATH="/api"
PARAMS_EMPTY='{}'

mkdir -p "$EVIDENCE_DIR"

# ── Ground-truth RPC methods from bridge/pkg/rpc/server.go registerHandlers() ─
# Extracted from lines 1219-1330 (109 methods total).
# Includes methods missing from a0_discover.sh KNOWN_METHODS:
#   secretary.create_workflow, secretary.is_running, secretary.get_active_count,
#   secretary.shutdown, bridge.e2ee_enable, bridge.e2ee_disable
ALL_RPC_METHODS=(
  "ai.chat"
  "browser.navigate" "browser.fill" "browser.click" "browser.status"
  "browser.wait_for_element" "browser.wait_for_captcha" "browser.wait_for_2fa"
  "browser.complete" "browser.fail" "browser.list" "browser.cancel"
  "browser.replay_diagnostics"
  "bridge.start" "bridge.stop" "bridge.status" "bridge.channel"
  "bridge.unchannel" "bridge.list" "bridge.ghost_list" "bridge.appservice_status"
  "bridge.e2ee_enable" "bridge.e2ee_disable"
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
  "secretary.create_workflow" "secretary.advance_workflow"
  "secretary.list_templates" "secretary.create_template"
  "secretary.get_template" "secretary.delete_template" "secretary.update_template"
  "secretary.is_running" "secretary.get_active_count" "secretary.shutdown"
  "task.create" "task.list" "task.cancel" "task.get"
  "device.list" "device.get" "device.approve" "device.reject"
  "invite.list" "invite.create" "invite.revoke" "invite.validate"
  "keystore.unseal" "keystore.sealed" "keystore.seal" "keystore.extend_session"
  "keystore.session_status" "keystore.list_keys" "keystore.delete_key"
  "e2ee.create_backup" "e2ee.delete_backup" "e2ee.backup_exists"
  "voice.start_session" "voice.stop_session" "voice.status"
)

log_info "========================================="
log_info " A0 Investigation: 103/0 Mismatch"
log_info " Methods: ${#ALL_RPC_METHODS[@]}"
log_info " Timeouts: ${TIMEOUTS[*]}s"
log_info " Transports: ${TRANSPORTS[*]}"
log_info "========================================="

# ── SSH connectivity check ───────────────────────────────────────────────────
log_info "Verifying SSH connectivity to ${VPS_USER}@${VPS_IP}..."
if ! ssh_vps "echo SSH_OK" 2>/dev/null | grep -q "SSH_OK"; then
  log_fail "Cannot SSH to ${VPS_USER}@${VPS_IP} — aborting"
  exit 1
fi
log_pass "SSH connectivity verified"

# ── Bridge liveness check ────────────────────────────────────────────────────
log_info "Checking if bridge is alive on port ${BRIDGE_PORT}..."
BRIDGE_ALIVE=false
for transport in "${TRANSPORTS[@]}"; do
  local_extra=()
  if [[ "$transport" == "https" ]]; then
    local_extra=(-k)
  fi
  if ssh_vps "curl -sf ${local_extra[*]} -o /dev/null -m 5 '${transport}://localhost:${BRIDGE_PORT}/health'" 2>/dev/null; then
    BRIDGE_ALIVE=true
    WORKING_TRANSPORT="$transport"
    log_pass "Bridge alive on ${transport}://localhost:${BRIDGE_PORT}"
    break
  fi
done

if [[ "$BRIDGE_ALIVE" != "true" ]]; then
  log_fail "Bridge not responding on any transport — cannot investigate"
  # Still write a summary with bridge_down status
  jq -nc \
    --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    '{status:"bridge_down",timestamp:$ts,methods_tested:0,dominant_root_cause:"transport",note:"Bridge not responding on any transport"}' \
    > "$SUMMARY_PATH"
  exit 1
fi

# ── Per-method investigation ─────────────────────────────────────────────────
log_info "Starting per-method investigation..."

# Arrays for summary counters
declare -A CAUSE_COUNTS
CAUSE_COUNTS[timeout]=0
CAUSE_COUNTS[parsing]=0
CAUSE_COUNTS[transport]=0
CAUSE_COUNTS[method-missing]=0
CAUSE_COUNTS[param-required]=0
CAUSE_COUNTS[ok-error]=0
CAUSE_COUNTS[ok-result]=0

METHODS_TESTED=0
METHODS_JSON="[]"

for method in "${ALL_RPC_METHODS[@]}"; do
  METHODS_TESTED=$((METHODS_TESTED + 1))
  log_info "[${METHODS_TESTED}/${#ALL_RPC_METHODS[@]}] Investigating: ${method}"

  method_findings="[]"
  best_cause="unknown"
  best_timeout="none"
  best_transport="none"
  got_response=false
  got_error=false
  got_result=false
  first_valid_response=""
  response_time_ms=-1

  for timeout in "${TIMEOUTS[@]}"; do
    for transport in "${TRANSPORTS[@]}"; do
      # Build curl flags
      curl_flags="-sf -m ${timeout}"
      url="${transport}://localhost:${BRIDGE_PORT}${RPC_PATH}"
      if [[ "$transport" == "https" ]]; then
        curl_flags="${curl_flags} -k"
      fi

      # Build JSON-RPC request
      request=$(jq -nc \
        --arg method "$method" \
        '{jsonrpc:"2.0", id: 1, method: $method, params: {}}')

      # Execute with timing
      start_ms=$(date +%s%3N 2>/dev/null || python3 -c "import time; print(int(time.time()*1000))")
      raw_response=""
      curl_exit=0

      raw_response=$(ssh_vps "curl ${curl_flags} -w '\n%{http_code} %{time_total}' '${url}' -H 'Content-Type: application/json' -d '${request}'" 2>/dev/null) || curl_exit=$?
      end_ms=$(date +%s%3N 2>/dev/null || python3 -c "import time; print(int(time.time()*1000))")

      elapsed_ms=$(( end_ms - start_ms ))

      # Separate HTTP status / time_total from body
      http_code="000"
      time_total="0.000"
      body=""

      if [[ -n "$raw_response" ]]; then
        # Last line is "http_code time_total"
        meta_line=$(echo "$raw_response" | tail -1)
        body=$(echo "$raw_response" | sed '$d')

        http_code=$(echo "$meta_line" | awk '{print $1}')
        time_total=$(echo "$meta_line" | awk '{print $2}')
      fi

      # Classify this probe
      probe_cause="unknown"
      is_valid_json=false
      response_type="none"

      if [[ $curl_exit -ne 0 ]]; then
        # curl itself failed (exit != 0)
        if [[ $elapsed_ms -ge $((timeout * 1000 - 500)) ]]; then
          probe_cause="timeout"
        else
          probe_cause="transport"
        fi
      elif [[ -z "$body" ]]; then
        probe_cause="transport"
      elif echo "$body" | jq -e . >/dev/null 2>&1; then
        # Valid JSON response
        is_valid_json=true
        if echo "$body" | jq -e '.error' >/dev/null 2>&1; then
          response_type="error"
          error_code=$(echo "$body" | jq -r '.error.code // "unknown"' 2>/dev/null)
          error_msg=$(echo "$body" | jq -r '.error.message // "unknown"' 2>/dev/null)

          # Check if error is about missing/invalid params (method exists but needs params)
          if echo "$error_msg" | grep -qi "param\|argument\|required\|missing\|invalid\|schema"; then
            probe_cause="param-required"
          else
            probe_cause="ok-error"
          fi
          got_error=true
          if [[ -z "$first_valid_response" ]]; then
            first_valid_response="$body"
          fi
        elif echo "$body" | jq -e '.result' >/dev/null 2>&1; then
          response_type="result"
          probe_cause="ok-result"
          got_result=true
          if [[ -z "$first_valid_response" ]]; then
            first_valid_response="$body"
          fi
        else
          probe_cause="parsing"
        fi
      else
        # Non-JSON response
        probe_cause="parsing"
      fi

      # Track best result
      if [[ "$probe_cause" == "ok-result" || "$probe_cause" == "ok-error" || "$probe_cause" == "param-required" ]]; then
        got_response=true
        if [[ "$best_cause" == "unknown" || "$best_cause" == "timeout" || "$best_cause" == "transport" ]]; then
          best_cause="$probe_cause"
          best_timeout="$timeout"
          best_transport="$transport"
          response_time_ms=$(echo "$time_total" | awk '{printf "%.0f", $1 * 1000}')
        fi
      fi

      # Append probe result
      probe_entry=$(jq -nc \
        --arg transport "$transport" \
        --argjson timeout "$timeout" \
        --arg curl_exit "$curl_exit" \
        --arg http_code "$http_code" \
        --arg time_total "$time_total" \
        --argjson elapsed_ms "$elapsed_ms" \
        --arg cause "$probe_cause" \
        --arg response_type "$response_type" \
        --argjson is_valid_json "$is_valid_json" \
        '{transport:$transport,timeout:$timeout,curl_exit:($curl_exit|tonumber),http_code:$http_code,time_total:$time_total,elapsed_ms:$elapsed_ms,cause:$cause,response_type:$response_type,is_valid_json:$is_valid_json}')
      method_findings=$(echo "$method_findings" | jq --argjson p "$probe_entry" '. + [$p]')
    done
  done

  # Final classification for this method
  final_cause="$best_cause"
  if [[ "$got_response" == "false" ]]; then
    if [[ "$got_error" == "true" ]]; then
      final_cause="ok-error"
    elif [[ "$got_result" == "true" ]]; then
      final_cause="ok-result"
    else
      # Check probes to determine why
      # If any probe got a non-timeout transport issue, it's transport
      # If all probes timed out, it's timeout
      all_timed_out=true
      for i in $(echo "$method_findings" | jq -r '.[] | .cause' 2>/dev/null); do
        if [[ "$i" != "timeout" && "$i" != "unknown" ]]; then
          all_timed_out=false
          break
        fi
      done
      if [[ "$all_timed_out" == "true" ]]; then
        final_cause="timeout"
      else
        final_cause="transport"
      fi
    fi
  fi

  # Update cause counters
  CAUSE_COUNTS[$final_cause]=$((CAUSE_COUNTS[$final_cause] + 1))

  # Build method summary
  error_preview=""
  if [[ -n "$first_valid_response" ]]; then
    error_preview=$(echo "$first_valid_response" | jq -c '.' 2>/dev/null | head -c 500)
  fi

  method_entry=$(jq -nc \
    --arg name "$method" \
    --arg cause "$final_cause" \
    --arg best_timeout "$best_timeout" \
    --arg best_transport "$best_transport" \
    --argjson response_time_ms "$response_time_ms" \
    --argjson probes "$method_findings" \
    --arg error_preview "$error_preview" \
    '{name:$name,root_cause:$cause,best_timeout:$best_timeout,best_transport:$best_transport,response_time_ms:$response_time_ms,probes:$probes,error_preview:$error_preview}')

  # Save per-method JSON
  safe_name=$(echo "$method" | tr '.' '_')
  echo "$method_entry" > "${EVIDENCE_DIR}/${safe_name}.json"

  METHODS_JSON=$(echo "$METHODS_JSON" | jq --argjson e "$method_entry" '. + [$e]')

  # Log status
  case "$final_cause" in
    ok-result)   log_pass "  ${method} → ${final_cause} (${response_time_ms}ms @ ${best_transport}/${best_timeout}s)" ;;
    ok-error)    log_info "  ${method} → ${final_cause} (${response_time_ms}ms @ ${best_transport}/${best_timeout}s)" ;;
    param-required) log_info "  ${method} → ${final_cause} (${response_time_ms}ms @ ${best_transport}/${best_timeout}s)" ;;
    timeout)     log_fail "  ${method} → ${final_cause} (all probes timed out)" ;;
    transport)   log_fail "  ${method} → ${final_cause} (connection error)" ;;
    parsing)     log_fail "  ${method} → ${final_cause} (invalid response)" ;;
    *)           log_fail "  ${method} → ${final_cause}" ;;
  esac
done

# ── Determine dominant root cause ────────────────────────────────────────────
dominant_cause="unknown"
dominant_count=0
for cause in timeout parsing transport method-missing param-required ok-error ok-result; do
  count=${CAUSE_COUNTS[$cause]:-0}
  if [[ $count -gt $dominant_count ]]; then
    dominant_count=$count
    dominant_cause="$cause"
  fi
done

# ── Write summary JSON ──────────────────────────────────────────────────────
log_info "Writing investigation summary..."

CAUSE_SUMMARY_JSON=$(jq -nc \
  --argjson timeout "${CAUSE_COUNTS[timeout]:-0}" \
  --argjson parsing "${CAUSE_COUNTS[parsing]:-0}" \
  --argjson transport "${CAUSE_COUNTS[transport]:-0}" \
  --argjson method_missing "${CAUSE_COUNTS[method-missing]:-0}" \
  --argjson param_required "${CAUSE_COUNTS[param-required]:-0}" \
  --argjson ok_error "${CAUSE_COUNTS[ok-error]:-0}" \
  --argjson ok_result "${CAUSE_COUNTS[ok-result]:-0}" \
  '{
    timeout:$timeout,
    parsing:$parsing,
    transport:$transport,
    method_missing:$method_missing,
    param_required:$param_required,
    ok_error:$ok_error,
    ok_result:$ok_result
  }')

# Build analysis note based on findings
analysis_note=""
if [[ "$dominant_cause" == "ok-error" ]]; then
  analysis_note="Most methods respond with JSON-RPC errors when called with empty params {}. This is CORRECT behavior — a0_discover.sh classification bug: it counts error-responding methods as 'found' but not 'responding'. The 5s timeout is not the primary issue."
elif [[ "$dominant_cause" == "timeout" ]]; then
  analysis_note="Most methods time out even at 30s budget. The 5s timeout in contract.sh is NOT the sole issue — bridge may not be handling these methods or SSH tunnel adds significant latency."
elif [[ "$dominant_cause" == "param-required" ]]; then
  analysis_note="Most methods require specific parameters. Empty params {} triggers parameter validation errors. Methods ARE registered and responding — the a0_discover.sh classification treats these as non-responding."
elif [[ "$dominant_cause" == "transport" ]]; then
  analysis_note="Transport-level failures dominate. Check bridge TLS configuration, port binding, and firewall rules."
fi

SUMMARY=$(jq -nc \
  --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --argjson methods_tested "$METHODS_TESTED" \
  --argjson methods_total "${#ALL_RPC_METHODS[@]}" \
  --arg dominant_root_cause "$dominant_cause" \
  --argjson cause_counts "$CAUSE_SUMMARY_JSON" \
  --argjson methods "$METHODS_JSON" \
  --arg analysis "$analysis_note" \
  --arg working_transport "$WORKING_TRANSPORT" \
  '{
    status:"complete",
    timestamp:$ts,
    methods_tested:$methods_tested,
    methods_total:$methods_total,
    dominant_root_cause:$dominant_root_cause,
    cause_counts:$cause_counts,
    analysis:$analysis,
    working_transport:$working_transport,
    time_budgets_tested:[5,10,30],
    transports_tested:["http","https"],
    methods:$methods
  }')

echo "$SUMMARY" > "$SUMMARY_PATH"

# ── Final report ─────────────────────────────────────────────────────────────
log_info "========================================="
log_info " A0 Investigation Complete"
log_info "========================================="
log_info " Methods tested:  ${METHODS_TESTED}/${#ALL_RPC_METHODS[@]}"
log_info " Dominant cause:  ${dominant_cause} (${dominant_count} methods)"
log_info ""
log_info " Cause breakdown:"
for cause in timeout parsing transport method-missing param-required ok-error ok-result; do
  count=${CAUSE_COUNTS[$cause]:-0}
  [[ $count -gt 0 ]] && log_info "   ${cause}: ${count}"
done
log_info ""
log_info " Analysis: ${analysis_note}"
log_info ""
log_info " Evidence dir:   ${EVIDENCE_DIR}/"
log_info " Summary file:   ${SUMMARY_PATH}"
log_info "========================================="
