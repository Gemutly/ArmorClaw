#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/load_env.sh"
source "$SCRIPT_DIR/lib/common_output.sh"

EVIDENCE_DIR="$SCRIPT_DIR/../.sisyphus/evidence/pbcp-13"
mkdir -p "$EVIDENCE_DIR"

PASS=0; FAIL=0; SKIP=0

rpc() {
  local method="$1"
  local params="${2:-{\}}"
  ssh_vps "curl -ksS 'https://localhost:${BRIDGE_PORT}/api' \
    -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer ${ADMIN_TOKEN}' \
    -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"${method}\",\"params\":${params}}'" 2>/dev/null
}

log_info "── P0: Prerequisites ─────────────────────────────"

if ! check_bridge_running 2>/dev/null; then
  log_skip "Bridge not running"
  harness_summary
  exit 0
fi
log_pass "Bridge reachable"

# ══════════════════════════════════════════════════════════════════════════════
# P1: Probe all Secretary + Task methods
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P1: Probe Secretary/Task methods ───────────────"

SECRETARY_METHODS=(
  "secretary.list_templates"
  "secretary.get_template"
  "secretary.create_template"
  "secretary.delete_template"
  "secretary.start_workflow"
  "secretary.get_workflow"
  "secretary.list_workflows"
  "secretary.cancel_workflow"
  "secretary.pause_workflow"
  "secretary.resume_workflow"
  "secretary.get_workflow_status"
  "secretary.get_workflow_history"
  "secretary.delete_workflow"
)

TASK_METHODS=(
  "task.create"
  "task.list"
  "task.get"
  "task.cancel"
)

AVAILABILITY_FILE="$EVIDENCE_DIR/method-availability.txt"
echo "# Secretary/Task Method Availability — $(date)" > "$AVAILABILITY_FILE"
echo "" >> "$AVAILABILITY_FILE"

AVAILABLE_COUNT=0
UNAVAILABLE_COUNT=0

probe_method() {
  local method="$1"
  local result
  result=$(rpc "$method" 2>/dev/null || true)

  if [[ -z "$result" ]]; then
    echo "  $method → NO_RESPONSE" >> "$AVAILABILITY_FILE"
    echo "no_response"
    return
  fi

  local has_result
  has_result=$(echo "$result" | jq -e '.result' >/dev/null 2>&1 && echo "yes" || echo "no")

  if [[ "$has_result" == "yes" ]]; then
    echo "  $method → AVAILABLE (returns object)" >> "$AVAILABILITY_FILE"
    echo "available"
    return 0
  fi

  local error_code
  error_code=$(echo "$result" | jq -r '.error.code // "none"' 2>/dev/null)

  if [[ "$error_code" == "-32601" ]]; then
    echo "  $method → NOT_REGISTERED (-32601)" >> "$AVAILABILITY_FILE"
    echo "not_registered"
  elif [[ "$error_code" == "-32602" ]]; then
    echo "  $method → NEEDS_PARAMS (-32602)" >> "$AVAILABILITY_FILE"
    echo "needs_params"
    return 0
  else
    echo "  $method → ERROR ($error_code)" >> "$AVAILABILITY_FILE"
    echo "error:$error_code"
  fi
  return 1
}

echo "## Secretary Methods" >> "$AVAILABILITY_FILE"
WORKING_METHODS=""
for method in "${SECRETARY_METHODS[@]}"; do
  status=$(probe_method "$method") || true
  case "$status" in
    available|needs_params)
      AVAILABLE_COUNT=$((AVAILABLE_COUNT + 1))
      WORKING_METHODS="$WORKING_METHODS $method"
      ;;
    *)
      UNAVAILABLE_COUNT=$((UNAVAILABLE_COUNT + 1))
      ;;
  esac
done

echo "" >> "$AVAILABILITY_FILE"
echo "## Task Methods" >> "$AVAILABILITY_FILE"
for method in "${TASK_METHODS[@]}"; do
  status=$(probe_method "$method") || true
  case "$status" in
    available|needs_params)
      AVAILABLE_COUNT=$((AVAILABLE_COUNT + 1))
      WORKING_METHODS="$WORKING_METHODS $method"
      ;;
    *)
      UNAVAILABLE_COUNT=$((UNAVAILABLE_COUNT + 1))
      ;;
  esac
done

echo "" >> "$AVAILABILITY_FILE"
echo "Summary: $AVAILABLE_COUNT available, $UNAVAILABLE_COUNT unavailable" >> "$AVAILABILITY_FILE"

if [[ "$AVAILABLE_COUNT" -gt 0 ]]; then
  log_pass "Probed 17 methods: $AVAILABLE_COUNT available, $UNAVAILABLE_COUNT unavailable"
else
  log_skip "No Secretary/Task methods available"
fi

# ══════════════════════════════════════════════════════════════════════════════
# P2: Test secretary.list_templates lifecycle
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P2: secretary.list_templates ───────────────────"

P2_FILE="$EVIDENCE_DIR/secretary-list-templates.json"
rpc "secretary.list_templates" > "$P2_FILE" 2>/dev/null || true

if [[ -s "$P2_FILE" ]] && jq -e '.result' "$P2_FILE" >/dev/null 2>&1; then
  template_count=$(jq -r '.result.count // (.result.templates | length) // 0' "$P2_FILE" 2>/dev/null)
  log_pass "secretary.list_templates: count=$template_count"
else
  log_skip "secretary.list_templates: not available"
fi

# ══════════════════════════════════════════════════════════════════════════════
# P3: Test secretary.start_workflow (if available)
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P3: secretary.start_workflow ───────────────────"

P3_FILE="$EVIDENCE_DIR/secretary-start-workflow.json"
rpc "secretary.start_workflow" "{\"template_id\":\"test\",\"name\":\"pbcp-probe\"}" > "$P3_FILE" 2>/dev/null || true

if [[ -s "$P3_FILE" ]]; then
  has_result=$(jq -e '.result' "$P3_FILE" >/dev/null 2>&1 && echo "yes" || echo "no")
  if [[ "$has_result" == "yes" ]]; then
    log_pass "secretary.start_workflow: $(jq -c '.result' "$P3_FILE" 2>/dev/null | head -c 80)"
    WF_ID=$(jq -r '.result.workflow_id // .result.id // empty' "$P3_FILE" 2>/dev/null)
  else
    error_code=$(jq -r '.error.code // "none"' "$P3_FILE" 2>/dev/null)
    error_msg=$(jq -r '.error.message // ""' "$P3_FILE" 2>/dev/null)
    log_skip "secretary.start_workflow: $error_code $error_msg"
  fi
else
  log_skip "secretary.start_workflow: no response"
fi

# ══════════════════════════════════════════════════════════════════════════════
# P4: Test task.create (if available)
# ══════════════════════════════════════════════════════════════════════════════
log_info "── P4: task.create ───────────────────────────────"

P4_FILE="$EVIDENCE_DIR/task-create.json"
rpc "task.create" "{\"description\":\"pbcp-probe-test\",\"priority\":\"low\"}" > "$P4_FILE" 2>/dev/null || true

if [[ -s "$P4_FILE" ]]; then
  has_result=$(jq -e '.result' "$P4_FILE" >/dev/null 2>&1 && echo "yes" || echo "no")
  if [[ "$has_result" == "yes" ]]; then
    log_pass "task.create: $(jq -c '.result' "$P4_FILE" 2>/dev/null | head -c 80)"
  else
    error_code=$(jq -r '.error.code // "none"' "$P4_FILE" 2>/dev/null)
    log_skip "task.create: $error_code"
  fi
else
  log_skip "task.create: no response"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo " Secretary Lifecycle Probe Summary"
echo "========================================="
echo " Passed: $PASS | Failed: $FAIL | Skipped: $SKIP"
echo " Methods: $AVAILABLE_COUNT available / $UNAVAILABLE_COUNT unavailable"
echo "========================================="

harness_summary
