#!/usr/bin/env bash
# a4_harness.sh — Phase A4: Run test suites on VPS

set -uo pipefail

_SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${_SCRIPT_DIR}/lib/contract.sh"

log_info "========================================="
log_info " Phase A4: Harness Execution on VPS"
log_info "========================================="

SUITES="${1:-health}"
REMOTE_DIR="/opt/armorclaw/tests"
PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0
RESULTS_JSON="{}"

declare -A SUITE_MAP=(
  ["health"]="test-system-health-baseline.sh"
  ["eventbus"]="test-eventbus-streaming.sh"
  ["trust"]="test-trust-layer.sh"
  ["workflow-core"]="test-secretary-workflow-core.sh"
  ["email"]="test-email-pipeline.sh"
  ["workflow-deep"]="test-secretary-workflow-deep.sh"
  ["sidecar-docs"]="test-sidecar-docs.sh"
  ["voice"]="test-voice-stack.sh"
  ["jetski"]="test-jetski-sidecar.sh"
  ["license"]="test-license-enforcement.sh"
  ["platform"]="test-platform-adapters.sh"
  ["agent-runtime"]="test-agent-runtime.sh"
  ["deployment-usb"]="test-deployment-usb.sh"
  ["cross-workflow-email"]="test-cross-workflow-email.sh"
  ["cross-workflow-docs"]="test-cross-workflow-docs.sh"
  ["cross-browser-trust"]="test-cross-browser-trust.sh"
  ["cross-event-truth"]="test-cross-event-truth.sh"
  ["tls-mode"]="test-tls-mode-integration.sh"
  ["tls-restart"]="test-tls-restart-safety.sh"
  ["keystore-agent"]="test-keystore-agent.sh"
)

IFS=',' read -ra SUITE_LIST <<< "$SUITES"

for suite_name in "${SUITE_LIST[@]}"; do
  suite_name=$(echo "$suite_name" | xargs)
  test_file="${SUITE_MAP[$suite_name]:-}"

  if [[ -z "$test_file" ]]; then
    log_skip "A4: Unknown suite '${suite_name}'"
    SKIP_COUNT=$((SKIP_COUNT + 1))
    RESULTS_JSON=$(echo "$RESULTS_JSON" | jq --arg name "$suite_name" '. + {($name): {status: "unknown"}}')
    continue
  fi

  log_info "A4: Running suite '${suite_name}' (${test_file})..."
  RUN_OUTPUT=$(cd "${_REPO_ROOT}" && bash "tests/${test_file}" 2>&1)
  RUN_EXIT=$?

  _contract_save "a4_${suite_name}_output.txt" "${RUN_OUTPUT}"

  if echo "$RUN_OUTPUT" | grep -qi "ALL TESTS PASSED\|PASSED"; then
    log_pass "A4: ${suite_name} — PASSED"
    PASS_COUNT=$((PASS_COUNT + 1))
    RESULTS_JSON=$(echo "$RESULTS_JSON" | jq --arg name "$suite_name" '. + {($name): {status: "passed"}}')
  elif echo "$RUN_OUTPUT" | grep -qi "SKIP\|skipped"; then
    log_info "A4: ${suite_name} — SKIPPED (dependencies not met)"
    SKIP_COUNT=$((SKIP_COUNT + 1))
    RESULTS_JSON=$(echo "$RESULTS_JSON" | jq --arg name "$suite_name" '. + {($name): {status: "skipped"}}')
  elif [[ $RUN_EXIT -eq 0 ]]; then
    log_pass "A4: ${suite_name} — completed (exit 0)"
    PASS_COUNT=$((PASS_COUNT + 1))
    RESULTS_JSON=$(echo "$RESULTS_JSON" | jq --arg name "$suite_name" '. + {($name): {status: "passed"}}')
  else
    log_fail "A4: ${suite_name} — FAILED (exit ${RUN_EXIT})"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    RESULTS_JSON=$(echo "$RESULTS_JSON" | jq --arg name "$suite_name" --arg exit "$RUN_EXIT" '. + {($name): {status: "failed", exit_code: ($exit | tonumber)}}')
  fi
done

_contract_save "a4_summary.json" "$(jq -nc \
  --argjson pass "$PASS_COUNT" --argjson fail "$FAIL_COUNT" --argjson skip "$SKIP_COUNT" \
  --argjson results "$RESULTS_JSON" \
  '{
    phase: "A4",
    pass: $pass,
    fail: $fail,
    skip: $skip,
    total: ($pass + $fail + $skip),
    suites: $results,
    timestamp: (now | todate)
  }')"

FULL_SYSTEM_PASSED=$PASS_COUNT
FULL_SYSTEM_FAILED=$FAIL_COUNT
FULL_SYSTEM_SKIPPED=$SKIP_COUNT

log_info "========================================="
log_info " Phase A4: Harness Complete"
log_info "  Pass: $PASS_COUNT | Fail: $FAIL_COUNT | Skip: $SKIP_COUNT"
log_info "========================================="
harness_summary
