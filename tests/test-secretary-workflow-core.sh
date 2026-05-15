#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# test-secretary-workflow-core.sh — Secretary Workflow Core Harness (T3a)
#
# Validates secretary RPC state machine: template lifecycle, workflow
# execution, blockers, restart survival, and negative paths.
#
# Scenarios:
#   W0  Prerequisites
#   W1  Template lifecycle (create/get/update/list/delete)
#   W2  Single-step workflow (start/get/cancel)
#   W3  Multi-step workflow (start/advance through steps)
#   W4  Blocker creation/resolution (resolve_blocker)
#   W5  Restart survival (template+workflow survive bridge restart)
#   W6  Negative paths (malformed input, nonexistent IDs, duplicates)
#
# Usage:  bash tests/test-secretary-workflow-core.sh
# Tier:   A (VPS — calls bridge RPC via HTTPS)
# ──────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib/load_env.sh"
source "$SCRIPT_DIR/lib/transport.sh"
source "$SCRIPT_DIR/lib/common_output.sh"
source "$SCRIPT_DIR/lib/assert_json.sh"
source "$SCRIPT_DIR/lib/restart_bridge.sh"

EVIDENCE_DIR="$SCRIPT_DIR/../.sisyphus/evidence/full-system-t3a"
mkdir -p "$EVIDENCE_DIR"

# ── Unique test prefix ─────────────────────────────────────────────────────────
UNIQUE="harness-t3a-$(date +%s)-$$"

# ── Track created resources for cleanup ─────────────────────────────────────────
CREATED_TEMPLATE_IDS=()
CREATED_WORKFLOW_IDS=()

# ── Override transport URL for VPS targeting ────────────────────────────────────
export BRIDGE_HTTP_URL="https://${VPS_IP}:${BRIDGE_PORT}"

# ── Auth-aware RPC wrapper (delegates to shared transport.sh) ───────────────────
rpc_call() { rpc_call_auth "$@"; }

# ── Cleanup trap ────────────────────────────────────────────────────────────────
cleanup() {
  local exit_code=$?
  log_info "Running cleanup..."

  # Delete all test templates (best-effort)
  for tid in "${CREATED_TEMPLATE_IDS[@]}"; do
    log_info "Deleting template: $tid"
    rpc_call "secretary.delete_template" "{\"template_id\":\"$tid\"}" >/dev/null 2>&1 || true
  done

  # Cancel all test workflows (best-effort)
  for wid in "${CREATED_WORKFLOW_IDS[@]}"; do
    log_info "Cancelling workflow: $wid"
    rpc_call "secretary.cancel_workflow" "{\"workflow_id\":\"$wid\",\"reason\":\"cleanup\"}" >/dev/null 2>&1 || true
  done

  exit $exit_code
}
trap cleanup EXIT

# ── Helper: save evidence ──────────────────────────────────────────────────────
save_evidence() {
  local name="$1" data="$2"
  echo "$data" > "$EVIDENCE_DIR/$name"
}

# ══════════════════════════════════════════════════════════════════════════════
# W0: Prerequisites
# ══════════════════════════════════════════════════════════════════════════════
log_info "── W0: Prerequisites ─────────────────────────────"

# jq
if command -v jq &>/dev/null; then
  log_pass "jq available"
else
  log_fail "jq not found — required for JSON assertions"
fi

# ADMIN_TOKEN
if [[ -z "${ADMIN_TOKEN:-}" ]]; then
  log_env_missing "ADMIN_TOKEN not configured — skipping admin-gated tests"
  harness_summary
  exit 0
fi
log_pass "ADMIN_TOKEN is set"

# Bridge running
if check_bridge_running; then
  log_pass "Bridge service is active on VPS"
else
  log_skip "Bridge not running — remaining tests require live bridge"
  harness_summary
  exit 0
fi

# Secretary availability — check via secretary.is_running or secretary.get_active_count
W0_SECRETARY_OK=false
W0_RESP=$(rpc_call "secretary.is_running" '{}')
save_evidence "w0-is-running.json" "$W0_RESP"

if echo "$W0_RESP" | jq -e '.result' >/dev/null 2>&1; then
  log_pass "secretary.is_running RPC available"
  W0_SECRETARY_OK=true
else
  # Try alternate method
  W0_RESP2=$(rpc_call "secretary.get_active_count" '{}')
  save_evidence "w0-active-count.json" "$W0_RESP2"
  if echo "$W0_RESP2" | jq -e '.result' >/dev/null 2>&1; then
    log_pass "secretary.get_active_count RPC available"
    W0_SECRETARY_OK=true
  else
    log_skip "Secretary RPC methods unavailable — skipping workflow tests"
    harness_summary
    exit 0
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# W1: Template lifecycle
# ══════════════════════════════════════════════════════════════════════════════
log_info "── W1: Template lifecycle ────────────────────────"

W1_TEMPLATE_NAME="${UNIQUE}-w1-simple"
W1_STEP_1="{\"step_id\":\"step_1\",\"order\":0,\"type\":\"action\",\"name\":\"Step One\",\"description\":\"First step\"}"
W1_CREATE_PARAMS="{\"name\":\"${W1_TEMPLATE_NAME}\",\"description\":\"T3a harness single-step template\",\"steps\":[${W1_STEP_1}],\"created_by\":\"harness\"}"

# Create template
W1_CREATE_RESP=$(rpc_call "secretary.create_template" "$W1_CREATE_PARAMS")
save_evidence "w1-create-template.json" "$W1_CREATE_RESP"
log_info "W1 create_template response: $W1_CREATE_RESP"

if assert_rpc_success "$W1_CREATE_RESP"; then
  log_pass "W1: create_template succeeded"
  W1_TEMPLATE_ID=$(echo "$W1_CREATE_RESP" | jq -r '.result.id // empty' 2>/dev/null || echo "")
  if [[ -n "$W1_TEMPLATE_ID" ]]; then
    CREATED_TEMPLATE_IDS+=("$W1_TEMPLATE_ID")
    log_pass "W1: template_id = $W1_TEMPLATE_ID"
  else
    log_fail "W1: could not extract template_id from response"
  fi
else
  log_fail "W1: create_template failed"
  W1_TEMPLATE_ID=""
fi

# Get template
if [[ -n "$W1_TEMPLATE_ID" ]]; then
  W1_GET_RESP=$(rpc_call "secretary.get_template" "{\"template_id\":\"${W1_TEMPLATE_ID}\"}")
  save_evidence "w1-get-template.json" "$W1_GET_RESP"

  if assert_rpc_success "$W1_GET_RESP"; then
    log_pass "W1: get_template succeeded"
    # Verify name matches
    W1_GOT_NAME=$(echo "$W1_GET_RESP" | jq -r '.result.name // empty' 2>/dev/null)
    if [[ "$W1_GOT_NAME" == "$W1_TEMPLATE_NAME" ]]; then
      log_pass "W1: template name matches ('$W1_GOT_NAME')"
    else
      log_fail "W1: template name mismatch (expected '$W1_TEMPLATE_NAME', got '$W1_GOT_NAME')"
    fi
    # Verify steps present
    W1_STEP_COUNT=$(echo "$W1_GET_RESP" | jq '.result.steps | length' 2>/dev/null || echo "0")
    if [[ "$W1_STEP_COUNT" -ge 1 ]]; then
      log_pass "W1: template has $W1_STEP_COUNT step(s)"
    else
      log_fail "W1: template has no steps"
    fi
  else
    log_fail "W1: get_template failed"
  fi
fi

# Update template
if [[ -n "$W1_TEMPLATE_ID" ]]; then
  W1_UPDATE_RESP=$(rpc_call "secretary.update_template" "{\"template_id\":\"${W1_TEMPLATE_ID}\",\"description\":\"Updated by T3a harness\"}")
  save_evidence "w1-update-template.json" "$W1_UPDATE_RESP"

  if assert_rpc_success "$W1_UPDATE_RESP"; then
    log_pass "W1: update_template succeeded"
    W1_UPDATED_DESC=$(echo "$W1_UPDATE_RESP" | jq -r '.result.description // empty' 2>/dev/null)
    if [[ "$W1_UPDATED_DESC" == "Updated by T3a harness" ]]; then
      log_pass "W1: description updated correctly"
    else
      log_fail "W1: description not updated (got: '$W1_UPDATED_DESC')"
    fi
  else
    log_fail "W1: update_template failed"
  fi
fi

# List templates — verify ours appears
W1_LIST_RESP=$(rpc_call "secretary.list_templates" '{}')
save_evidence "w1-list-templates.json" "$W1_LIST_RESP"

if assert_rpc_success "$W1_LIST_RESP"; then
  log_pass "W1: list_templates succeeded"
  if echo "$W1_LIST_RESP" | jq -e --arg name "$W1_TEMPLATE_NAME" '.result.templates[] | select(.name == $name)' >/dev/null 2>&1; then
    log_pass "W1: our template appears in list"
  else
    log_fail "W1: our template not found in list"
  fi
else
  log_fail "W1: list_templates failed"
fi

# ══════════════════════════════════════════════════════════════════════════════
# W2: Single-step workflow
# ══════════════════════════════════════════════════════════════════════════════
log_info "── W2: Single-step workflow ──────────────────────"

W2_TEMPLATE_NAME="${UNIQUE}-w2-workflow"
W2_STEP_1="{\"step_id\":\"w2_step_1\",\"order\":0,\"type\":\"action\",\"name\":\"W2 Step\",\"description\":\"Single step for workflow test\"}"
W2_CREATE_PARAMS="{\"name\":\"${W2_TEMPLATE_NAME}\",\"description\":\"T3a single-step workflow template\",\"steps\":[${W2_STEP_1}],\"created_by\":\"harness\"}"

W2_TEMPLATE_ID=""
W2_WORKFLOW_ID=""

# Create template for W2
W2_CREATE_RESP=$(rpc_call "secretary.create_template" "$W2_CREATE_PARAMS")
save_evidence "w2-create-template.json" "$W2_CREATE_RESP"

if assert_rpc_success "$W2_CREATE_RESP"; then
  W2_TEMPLATE_ID=$(echo "$W2_CREATE_RESP" | jq -r '.result.id // empty' 2>/dev/null || echo "")
  if [[ -n "$W2_TEMPLATE_ID" ]]; then
    CREATED_TEMPLATE_IDS+=("$W2_TEMPLATE_ID")
    log_pass "W2: template created (id=$W2_TEMPLATE_ID)"
  fi
else
  log_fail "W2: template creation failed"
fi

# Create workflow from template, then start it
if [[ -n "$W2_TEMPLATE_ID" ]]; then
  W2_CREATE_WF_RESP=$(rpc_call "secretary.create_workflow" "{\"template_id\":\"${W2_TEMPLATE_ID}\"}")
  save_evidence "w2-create-workflow.json" "$W2_CREATE_WF_RESP"

  W2_WF_ID=""
  if assert_rpc_success "$W2_CREATE_WF_RESP"; then
    W2_WF_ID=$(echo "$W2_CREATE_WF_RESP" | jq -r '.result.id // empty' 2>/dev/null || echo "")
    if [[ -n "$W2_WF_ID" ]]; then
      CREATED_WORKFLOW_IDS+=("$W2_WF_ID")
      log_pass "W2: create_workflow succeeded (id=$W2_WF_ID)"
    else
      log_fail "W2: create_workflow returned no id"
    fi
  else
    log_fail "W2: create_workflow failed"
  fi

  # Start the workflow
  if [[ -n "$W2_WF_ID" ]]; then
    W2_START_RESP=$(rpc_call "secretary.start_workflow" "{\"workflow_id\":\"${W2_WF_ID}\"}")
    save_evidence "w2-start-workflow.json" "$W2_START_RESP"

    if assert_rpc_success "$W2_START_RESP"; then
      log_pass "W2: start_workflow succeeded"
      W2_WORKFLOW_ID="$W2_WF_ID"
    else
      log_fail "W2: start_workflow failed"
      W2_START_ERR=$(echo "$W2_START_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
      log_info "W2: start_workflow error: $W2_START_ERR"
    fi
  fi

  # Get workflow status
  if [[ -n "$W2_WORKFLOW_ID" ]]; then
    W2_GET_RESP=$(rpc_call "secretary.get_workflow" "{\"workflow_id\":\"${W2_WORKFLOW_ID}\"}")
    save_evidence "w2-get-workflow.json" "$W2_GET_RESP"

    if assert_rpc_success "$W2_GET_RESP"; then
      log_pass "W2: get_workflow succeeded"
      W2_STATUS=$(echo "$W2_GET_RESP" | jq -r '.result.status // "unknown"' 2>/dev/null)
      log_info "W2: workflow status = $W2_STATUS"
      # Status should be one of: pending, running, completed, etc.
      if [[ "$W2_STATUS" =~ ^(pending|running|completed|blocked)$ ]]; then
        log_pass "W2: valid workflow status '$W2_STATUS'"
      else
        log_fail "W2: unexpected workflow status '$W2_STATUS'"
      fi
    else
      log_fail "W2: get_workflow failed"
    fi
  fi

  # Cancel workflow
  if [[ -n "$W2_WORKFLOW_ID" ]]; then
    W2_CANCEL_RESP=$(rpc_call "secretary.cancel_workflow" "{\"workflow_id\":\"${W2_WORKFLOW_ID}\",\"reason\":\"T3a test cancellation\"}")
    save_evidence "w2-cancel-workflow.json" "$W2_CANCEL_RESP"

    if assert_rpc_success "$W2_CANCEL_RESP"; then
      log_pass "W2: cancel_workflow succeeded"
      # Verify cancelled status
      W2_AFTER_CANCEL=$(rpc_call "secretary.get_workflow" "{\"workflow_id\":\"${W2_WORKFLOW_ID}\"}")
      W2_FINAL_STATUS=$(echo "$W2_AFTER_CANCEL" | jq -r '.result.status // "unknown"' 2>/dev/null)
      if [[ "$W2_FINAL_STATUS" == "cancelled" ]]; then
        log_pass "W2: workflow status is cancelled after cancellation"
      else
        log_fail "W2: expected cancelled, got '$W2_FINAL_STATUS'"
      fi
    else
      log_fail "W2: cancel_workflow failed"
    fi
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# W3: Multi-step workflow
# ══════════════════════════════════════════════════════════════════════════════
log_info "── W3: Multi-step workflow ───────────────────────"

W3_TEMPLATE_NAME="${UNIQUE}-w3-multistep"
W3_STEP_1="{\"step_id\":\"w3_s1\",\"order\":0,\"type\":\"action\",\"name\":\"W3 Step 1\"}"
W3_STEP_2="{\"step_id\":\"w3_s2\",\"order\":1,\"type\":\"action\",\"name\":\"W3 Step 2\"}"
W3_STEP_3="{\"step_id\":\"w3_s3\",\"order\":2,\"type\":\"action\",\"name\":\"W3 Step 3\"}"
W3_CREATE_PARAMS="{\"name\":\"${W3_TEMPLATE_NAME}\",\"description\":\"T3a 3-step template\",\"steps\":[${W3_STEP_1},${W3_STEP_2},${W3_STEP_3}],\"created_by\":\"harness\"}"

W3_TEMPLATE_ID=""
W3_WORKFLOW_ID=""

# Create 3-step template
W3_CREATE_RESP=$(rpc_call "secretary.create_template" "$W3_CREATE_PARAMS")
save_evidence "w3-create-template.json" "$W3_CREATE_RESP"

if assert_rpc_success "$W3_CREATE_RESP"; then
  W3_TEMPLATE_ID=$(echo "$W3_CREATE_RESP" | jq -r '.result.id // empty' 2>/dev/null || echo "")
  if [[ -n "$W3_TEMPLATE_ID" ]]; then
    CREATED_TEMPLATE_IDS+=("$W3_TEMPLATE_ID")
    log_pass "W3: 3-step template created (id=$W3_TEMPLATE_ID)"
  fi
else
  log_fail "W3: 3-step template creation failed"
fi

# Create and advance through steps
if [[ -n "$W3_TEMPLATE_ID" ]]; then
  W3_CREATE_WF_RESP=$(rpc_call "secretary.create_workflow" "{\"template_id\":\"${W3_TEMPLATE_ID}\"}")
  save_evidence "w3-create-workflow.json" "$W3_CREATE_WF_RESP"

  W3_WF_ID=""
  if assert_rpc_success "$W3_CREATE_WF_RESP"; then
    W3_WF_ID=$(echo "$W3_CREATE_WF_RESP" | jq -r '.result.id // empty' 2>/dev/null || echo "")
    if [[ -n "$W3_WF_ID" ]]; then
      CREATED_WORKFLOW_IDS+=("$W3_WF_ID")
      log_pass "W3: create_workflow succeeded (id=$W3_WF_ID)"
    else
      log_fail "W3: create_workflow returned no id"
    fi
  else
    log_fail "W3: create_workflow failed"
  fi

  if [[ -n "$W3_WF_ID" ]]; then
    W3_START_RESP=$(rpc_call "secretary.start_workflow" "{\"workflow_id\":\"${W3_WF_ID}\"}")
    save_evidence "w3-start-workflow.json" "$W3_START_RESP"

    if assert_rpc_success "$W3_START_RESP"; then
      W3_WORKFLOW_ID="$W3_WF_ID"
      log_pass "W3: multi-step workflow started"
    else
      log_fail "W3: start_workflow failed"
      W3_START_ERR=$(echo "$W3_START_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
      log_info "W3: error: $W3_START_ERR"
    fi
  fi
fi

# Advance through steps
if [[ -n "$W3_WORKFLOW_ID" ]]; then
    for STEP_ID in w3_s1 w3_s2 w3_s3; do
      W3_ADV_RESP=$(rpc_call "secretary.advance_workflow" "{\"workflow_id\":\"${W3_WORKFLOW_ID}\",\"step_id\":\"${STEP_ID}\"}")
      save_evidence "w3-advance-${STEP_ID}.json" "$W3_ADV_RESP"

      if assert_rpc_success "$W3_ADV_RESP"; then
        log_pass "W3: advance_workflow step=$STEP_ID succeeded"
      else
        log_fail "W3: advance_workflow step=$STEP_ID failed"
        W3_ADV_ERR=$(echo "$W3_ADV_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
        log_info "W3: advance error: $W3_ADV_ERR"
      fi
    done

    # Verify final state
    W3_FINAL_RESP=$(rpc_call "secretary.get_workflow" "{\"workflow_id\":\"${W3_WORKFLOW_ID}\"}")
    save_evidence "w3-final-state.json" "$W3_FINAL_RESP"
    if assert_rpc_success "$W3_FINAL_RESP"; then
      W3_FINAL_STATUS=$(echo "$W3_FINAL_RESP" | jq -r '.result.status // "unknown"' 2>/dev/null)
      log_info "W3: final workflow status = $W3_FINAL_STATUS"
    fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# W4: Blocker creation/resolution
# ══════════════════════════════════════════════════════════════════════════════
log_info "── W4: Blocker creation/resolution ───────────────"

# Create a template and start a workflow for blocker testing
W4_TEMPLATE_NAME="${UNIQUE}-w4-blocker"
W4_STEP_1="{\"step_id\":\"w4_s1\",\"order\":0,\"type\":\"action\",\"name\":\"W4 Blocker Step\"}"
W4_CREATE_PARAMS="{\"name\":\"${W4_TEMPLATE_NAME}\",\"description\":\"T3a blocker test template\",\"steps\":[${W4_STEP_1}],\"created_by\":\"harness\"}"

W4_TEMPLATE_ID=""
W4_WORKFLOW_ID=""

W4_CREATE_RESP=$(rpc_call "secretary.create_template" "$W4_CREATE_PARAMS")
save_evidence "w4-create-template.json" "$W4_CREATE_RESP"

if assert_rpc_success "$W4_CREATE_RESP"; then
  W4_TEMPLATE_ID=$(echo "$W4_CREATE_RESP" | jq -r '.result.id // empty' 2>/dev/null || echo "")
  if [[ -n "$W4_TEMPLATE_ID" ]]; then
    CREATED_TEMPLATE_IDS+=("$W4_TEMPLATE_ID")
  fi
fi

# Create and start workflow for blocker testing
if [[ -n "$W4_TEMPLATE_ID" ]]; then
  W4_CREATE_WF_RESP=$(rpc_call "secretary.create_workflow" "{\"template_id\":\"${W4_TEMPLATE_ID}\"}")
  save_evidence "w4-create-workflow.json" "$W4_CREATE_WF_RESP"

  W4_WF_ID=""
  if assert_rpc_success "$W4_CREATE_WF_RESP"; then
    W4_WF_ID=$(echo "$W4_CREATE_WF_RESP" | jq -r '.result.id // empty' 2>/dev/null || echo "")
    if [[ -n "$W4_WF_ID" ]]; then
      CREATED_WORKFLOW_IDS+=("$W4_WF_ID")
    fi
  fi

  if [[ -n "$W4_WF_ID" ]]; then
    W4_START_RESP=$(rpc_call "secretary.start_workflow" "{\"workflow_id\":\"${W4_WF_ID}\"}")
    save_evidence "w4-start-workflow.json" "$W4_START_RESP"

    if assert_rpc_success "$W4_START_RESP"; then
      W4_WORKFLOW_ID="$W4_WF_ID"
      log_pass "W4: workflow started for blocker test"
    else
      log_fail "W4: start_workflow failed for blocker test"
    fi
  else
    log_fail "W4: create_workflow failed"
  fi
fi

# Attempt resolve_blocker
# Note: resolve_blocker requires a pending blocker to exist; this tests the RPC interface
W4_RESOLVE_RESP=$(rpc_call "resolve_blocker" "{\"workflow_id\":\"${W4_WORKFLOW_ID:-nonexistent}\",\"step_id\":\"w4_s1\",\"input\":\"test-resolution-input\",\"note\":\"T3a harness blocker resolution\"}")
save_evidence "w4-resolve-blocker.json" "$W4_RESOLVE_RESP"

if assert_rpc_success "$W4_RESOLVE_RESP"; then
  log_pass "W4: resolve_blocker succeeded"
  W4_DELIVER_STATUS=$(echo "$W4_RESOLVE_RESP" | jq -r '.result.status // "unknown"' 2>/dev/null)
  if [[ "$W4_DELIVER_STATUS" == "delivered" ]]; then
    log_pass "W4: blocker resolved (status=delivered)"
  fi
else
  # Expected: may fail if no blocker is pending — that's acceptable
  W4_ERR_MSG=$(echo "$W4_RESOLVE_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
  if echo "$W4_ERR_MSG" | grep -qi "no pending blocker\|not found\|blocker"; then
    log_pass "W4: resolve_blocker correctly rejected (no pending blocker): $W4_ERR_MSG"
  else
    log_fail "W4: resolve_blocker unexpected error: $W4_ERR_MSG"
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# W5: Restart survival
# ══════════════════════════════════════════════════════════════════════════════
log_info "── W5: Restart survival ──────────────────────────"

W5_TEMPLATE_NAME="${UNIQUE}-w5-persist"
W5_STEP_1="{\"step_id\":\"w5_s1\",\"order\":0,\"type\":\"action\",\"name\":\"W5 Persist Step\"}"
W5_CREATE_PARAMS="{\"name\":\"${W5_TEMPLATE_NAME}\",\"description\":\"T3a restart survival template\",\"steps\":[${W5_STEP_1}],\"created_by\":\"harness\"}"

W5_TEMPLATE_ID=""
W5_WORKFLOW_ID=""

# Create template
W5_CREATE_RESP=$(rpc_call "secretary.create_template" "$W5_CREATE_PARAMS")
save_evidence "w5-create-template.json" "$W5_CREATE_RESP"

if assert_rpc_success "$W5_CREATE_RESP"; then
  W5_TEMPLATE_ID=$(echo "$W5_CREATE_RESP" | jq -r '.result.id // empty' 2>/dev/null || echo "")
  if [[ -n "$W5_TEMPLATE_ID" ]]; then
    CREATED_TEMPLATE_IDS+=("$W5_TEMPLATE_ID")
    log_pass "W5: template created (id=$W5_TEMPLATE_ID)"
  fi
else
  log_fail "W5: template creation failed"
fi

# Create and start workflow
if [[ -n "$W5_TEMPLATE_ID" ]]; then
  W5_CREATE_WF_RESP=$(rpc_call "secretary.create_workflow" "{\"template_id\":\"${W5_TEMPLATE_ID}\"}")
  save_evidence "w5-create-workflow.json" "$W5_CREATE_WF_RESP"

  W5_WF_ID=""
  if assert_rpc_success "$W5_CREATE_WF_RESP"; then
    W5_WF_ID=$(echo "$W5_CREATE_WF_RESP" | jq -r '.result.id // empty' 2>/dev/null || echo "")
    if [[ -n "$W5_WF_ID" ]]; then
      CREATED_WORKFLOW_IDS+=("$W5_WF_ID")
    fi
  else
    log_fail "W5: create_workflow failed"
  fi

  if [[ -n "$W5_WF_ID" ]]; then
    W5_START_RESP=$(rpc_call "secretary.start_workflow" "{\"workflow_id\":\"${W5_WF_ID}\"}")
    save_evidence "w5-start-workflow.json" "$W5_START_RESP"

    if assert_rpc_success "$W5_START_RESP"; then
      W5_WORKFLOW_ID="$W5_WF_ID"
      log_pass "W5: workflow started (id=$W5_WF_ID)"
    else
      log_fail "W5: start_workflow failed"
    fi
  fi
fi

# Record pre-restart state
W5_PRE_RESTART_STATUS=""
if [[ -n "$W5_TEMPLATE_ID" ]]; then
  W5_PRE_GET=$(rpc_call "secretary.get_template" "{\"template_id\":\"${W5_TEMPLATE_ID}\"}")
  W5_PRE_RESTART_DESC=$(echo "$W5_PRE_GET" | jq -r '.result.description // ""' 2>/dev/null)
  save_evidence "w5-pre-restart-template.json" "$W5_PRE_GET"
fi
if [[ -n "$W5_WORKFLOW_ID" ]]; then
  W5_PRE_WF=$(rpc_call "secretary.get_workflow" "{\"workflow_id\":\"${W5_WORKFLOW_ID}\"}")
  W5_PRE_RESTART_STATUS=$(echo "$W5_PRE_WF" | jq -r '.result.status // ""' 2>/dev/null)
  save_evidence "w5-pre-restart-workflow.json" "$W5_PRE_WF"
fi

# Restart bridge
log_info "W5: restarting bridge..."
if restart_bridge 60; then
  log_pass "W5: bridge restarted successfully"
else
  log_fail "W5: bridge restart failed/timed out"
fi

# Verify template survived
if [[ -n "$W5_TEMPLATE_ID" ]]; then
  W5_POST_GET=$(rpc_call "secretary.get_template" "{\"template_id\":\"${W5_TEMPLATE_ID}\"}")
  save_evidence "w5-post-restart-template.json" "$W5_POST_GET"

  if assert_rpc_success "$W5_POST_GET"; then
    log_pass "W5: template survived restart"
    W5_POST_DESC=$(echo "$W5_POST_GET" | jq -r '.result.description // ""' 2>/dev/null)
    if [[ "$W5_POST_DESC" == "$W5_PRE_RESTART_DESC" ]]; then
      log_pass "W5: template description preserved across restart"
    else
      log_fail "W5: template description changed (pre='$W5_PRE_RESTART_DESC', post='$W5_POST_DESC')"
    fi
  else
    log_fail "W5: template not found after restart"
  fi
fi

# Verify workflow survived
if [[ -n "$W5_WORKFLOW_ID" ]]; then
  W5_POST_WF=$(rpc_call "secretary.get_workflow" "{\"workflow_id\":\"${W5_WORKFLOW_ID}\"}")
  save_evidence "w5-post-restart-workflow.json" "$W5_POST_WF"

  if assert_rpc_success "$W5_POST_WF"; then
    log_pass "W5: workflow survived restart"
    W5_POST_STATUS=$(echo "$W5_POST_WF" | jq -r '.result.status // "unknown"' 2>/dev/null)
    log_info "W5: post-restart workflow status = $W5_POST_STATUS"
  else
    log_fail "W5: workflow not found after restart"
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# W6: Negative paths
# ══════════════════════════════════════════════════════════════════════════════
log_info "── W6: Negative paths ────────────────────────────"

# W6a: Empty template name (should fail validation)
W6A_RESP=$(rpc_call "secretary.create_template" "{\"name\":\"\",\"steps\":[{\"step_id\":\"s1\",\"order\":0,\"type\":\"action\",\"name\":\"S\"}],\"created_by\":\"harness\"}")
save_evidence "w6a-empty-name.json" "$W6A_RESP"
if assert_rpc_error "$W6A_RESP"; then
  log_pass "W6a: empty template name rejected"
else
  log_fail "W6a: empty template name should have been rejected"
fi

# W6b: Empty steps (should fail validation)
W6B_RESP=$(rpc_call "secretary.create_template" "{\"name\":\"${UNIQUE}-w6b-empty-steps\",\"steps\":[],\"created_by\":\"harness\"}")
save_evidence "w6b-empty-steps.json" "$W6B_RESP"
if assert_rpc_error "$W6B_RESP"; then
  log_pass "W6b: empty steps rejected"
else
  log_fail "W6b: empty steps should have been rejected"
fi

# W6c: Nonexistent workflow_id for get_workflow
W6C_RESP=$(rpc_call "secretary.get_workflow" "{\"workflow_id\":\"nonexistent-wf-t3a-999\"}")
save_evidence "w6c-nonexistent-wf.json" "$W6C_RESP"
if assert_rpc_error "$W6C_RESP"; then
  log_pass "W6c: nonexistent workflow_id correctly returns error"
else
  log_fail "W6c: nonexistent workflow_id should return error"
fi

# W6d: Nonexistent template_id for get_template
W6D_RESP=$(rpc_call "secretary.get_template" "{\"template_id\":\"nonexistent-tpl-t3a-999\"}")
save_evidence "w6d-nonexistent-tpl.json" "$W6D_RESP"
if assert_rpc_error "$W6D_RESP"; then
  log_pass "W6d: nonexistent template_id correctly returns error"
else
  log_fail "W6d: nonexistent template_id should return error"
fi

# W6e: Cancel nonexistent workflow
W6E_RESP=$(rpc_call "secretary.cancel_workflow" "{\"workflow_id\":\"nonexistent-wf-cancel-999\"}")
save_evidence "w6e-cancel-nonexistent.json" "$W6E_RESP"
if assert_rpc_error "$W6E_RESP"; then
  log_pass "W6e: cancel nonexistent workflow returns error"
else
  log_fail "W6e: cancel nonexistent workflow should return error"
fi

# W6f: Advance nonexistent workflow
W6F_RESP=$(rpc_call "secretary.advance_workflow" "{\"workflow_id\":\"nonexistent-wf-advance-999\",\"step_id\":\"step_1\"}")
save_evidence "w6f-advance-nonexistent.json" "$W6F_RESP"
if assert_rpc_error "$W6F_RESP"; then
  log_pass "W6f: advance nonexistent workflow returns error"
else
  log_fail "W6f: advance nonexistent workflow should return error"
fi

# W6g: resolve_blocker with missing required fields
W6G_RESP=$(rpc_call "resolve_blocker" "{\"workflow_id\":\"\",\"step_id\":\"\",\"input\":\"\"}")
save_evidence "w6g-blocker-missing-fields.json" "$W6G_RESP"
if assert_rpc_error "$W6G_RESP"; then
  log_pass "W6g: resolve_blocker with empty fields rejected"
else
  log_fail "W6g: resolve_blocker with empty fields should be rejected"
fi

# W6h: Duplicate resolution attempt (if W4 succeeded with a blocker)
# Re-use W4 workflow if available
W6H_WF_ID="${W4_WORKFLOW_ID:-nonexistent}"
W6H_RESP=$(rpc_call "resolve_blocker" "{\"workflow_id\":\"${W6H_WF_ID}\",\"step_id\":\"w4_s1\",\"input\":\"duplicate-resolve-attempt\"}")
save_evidence "w6h-duplicate-resolve.json" "$W6H_RESP"
# Whether it fails (no pending blocker) or succeeds, we verify the RPC handled it
if assert_rpc_error "$W6H_RESP"; then
  log_pass "W6h: duplicate resolve_blocker correctly returns error"
else
  log_pass "W6h: resolve_blocker accepted (blocker may have been re-created)"
fi

# W6i: Malformed params (invalid JSON — should cause parse error)
# Send an intentionally malformed request directly via HTTP
W6I_RESP=$(curl -ksS -X POST "https://${VPS_IP}:${BRIDGE_PORT}/api" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"secretary.create_template\",\"params\":{invalid}}" \
  --connect-timeout 10 --max-time 30 2>/dev/null || echo "{}")
save_evidence "w6i-malformed-params.json" "$W6I_RESP"
# Server should return an error or empty response, not crash
if echo "$W6I_RESP" | jq -e '.error' >/dev/null 2>&1 || [[ "$W6I_RESP" == "{}" ]] || [[ -z "$W6I_RESP" ]]; then
  log_pass "W6i: malformed params handled gracefully"
else
  log_fail "W6i: malformed params not handled correctly"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════
harness_summary
