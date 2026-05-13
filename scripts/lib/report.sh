# report.sh — Report generation infrastructure for VPS lifecycle
#
# Sourced library — no shebang, no main block. Functions prefixed with _.
# Provides structured report generation with per-feature-group breakdown,
# JSON and text output, and atomic file writes.
#
# Usage:
#   source "$(dirname "$0")/lib/report.sh"
#   _report_init --output-dir /path/to/evidence
#   _report_add_phase "deploy" "pass" "Bridge deployed successfully"
#   _report_add_feature_group "A-Matrix" "pass" "All Matrix tests passed" "/evidence/group-a.txt"
#   _report_set_verdict
#   _report_emit_json
#   _report_emit_text

set -uo pipefail

# ── Locate repo root ─────────────────────────────────────────────────────────
_REPORT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_REPORT_REPO_ROOT="$(cd "${_REPORT_DIR}/../.." && pwd)"

# ── Internal state ────────────────────────────────────────────────────────────
_REPORT_OUTPUT_DIR=""
_REPORT_TOPOLOGY="unknown"
_REPORT_DEPLOY_MODE="unknown"
_REPORT_FRESH_DEPLOY_RESULT="not-run"
_REPORT_EXISTING_INSTALL_RESULT="not-run"
_REPORT_MATRIX_BOOTSTRAP_RESULT="not-run"
_REPORT_PHASES=()            # JSON objects: {name, status, details}
_REPORT_FEATURE_GROUPS=()    # JSON objects: {group, status, details, evidence_path}
_REPORT_BLOCKERS=()          # JSON objects: {phase, message, severity}
_REPORT_EVIDENCE_PATHS=()    # Raw file paths
_REPORT_OVERALL_VERDICT="not-run"
_REPORT_INIT_TIME=""

# ── _report_init [--output-dir dir] ───────────────────────────────────────────
# Initialize the report state. Accepts --output-dir to set evidence directory.
# Creates the output directory if it doesn't exist.
#
# Arguments:
#   --output-dir  - Directory for report output (default: .sisyphus/evidence/vps-lifecycle)
_report_init() {
  local output_dir="${_REPORT_REPO_ROOT}/.sisyphus/evidence/vps-lifecycle"

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --output-dir)
        output_dir="${2:?--output-dir requires a value}"
        shift 2
        ;;
      *)
        echo "[REPORT] Unknown argument: $1" >&2
        shift
        ;;
    esac
  done

  _REPORT_OUTPUT_DIR="$output_dir"
  _REPORT_TOPOLOGY="unknown"
  _REPORT_DEPLOY_MODE="unknown"
  _REPORT_FRESH_DEPLOY_RESULT="not-run"
  _REPORT_EXISTING_INSTALL_RESULT="not-run"
  _REPORT_MATRIX_BOOTSTRAP_RESULT="not-run"
  _REPORT_PHASES=()
  _REPORT_FEATURE_GROUPS=()
  _REPORT_BLOCKERS=()
  _REPORT_EVIDENCE_PATHS=()
  _REPORT_OVERALL_VERDICT="not-run"
  _REPORT_INIT_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

  mkdir -p "$_REPORT_OUTPUT_DIR"
}

# ── _report_set_topology(topology, deploy_mode) ───────────────────────────────
# Set the topology classification and deployment mode.
#
# Arguments:
#   $1 - Topology string (e.g. "fresh", "native-systemd", "docker-conduit")
#   $2 - Deploy mode string (e.g. "fresh-install", "replace-existing")
_report_set_topology() {
  _REPORT_TOPOLOGY="${1:-unknown}"
  _REPORT_DEPLOY_MODE="${2:-unknown}"
}

# ── _report_add_phase(phase_name, status, details) ───────────────────────────
# Record a lifecycle phase result.
#
# Arguments:
#   $1 - Phase name (e.g. "deploy", "bootstrap", "validate")
#   $2 - Status: pass, fail, skip, not-run
#   $3 - Details string
_report_add_phase() {
  local name="${1:?Usage: _report_add_phase name status [details]}"
  local status="${2:-not-run}"
  local details="${3:-}"
  local entry
  entry=$(jq -nc \
    --arg name "$name" \
    --arg status "$status" \
    --arg details "$details" \
    '{name: $name, status: $status, details: $details}')
  _REPORT_PHASES+=("$entry")
}

# ── _report_set_deploy_results(fresh, existing, matrix_bootstrap) ────────────
# Set the deployment phase results.
#
# Arguments:
#   $1 - Fresh deploy result: pass, fail, not-run
#   $2 - Existing install result: pass, fail, skip, not-run
#   $3 - Matrix bootstrap result: pass, fail, not-run
_report_set_deploy_results() {
  _REPORT_FRESH_DEPLOY_RESULT="${1:-not-run}"
  _REPORT_EXISTING_INSTALL_RESULT="${2:-not-run}"
  _REPORT_MATRIX_BOOTSTRAP_RESULT="${3:-not-run}"
}

# ── _report_add_feature_group(group_name, status, details, evidence_path) ────
# Record a feature group result.
#
# Arguments:
#   $1 - Group name (e.g. "A-Matrix", "B-Studio")
#   $2 - Status: pass, fail, skip-disabled, not-run
#   $3 - Details string
#   $4 - Evidence file path (optional)
_report_add_feature_group() {
  local group="${1:?Usage: _report_add_feature_group group status [details] [evidence_path]}"
  local status="${2:-not-run}"
  local details="${3:-}"
  local evidence="${4:-}"
  local entry
  entry=$(jq -nc \
    --arg group "$group" \
    --arg status "$status" \
    --arg details "$details" \
    --arg evidence "$evidence" \
    '{group: $group, status: $status, details: $details, evidence_path: $evidence}')
  _REPORT_FEATURE_GROUPS+=("$entry")

  # Track evidence path
  if [[ -n "$evidence" ]]; then
    _REPORT_EVIDENCE_PATHS+=("$evidence")
  fi
}

# ── _report_add_blocker(phase, message, severity) ───────────────────────────
# Record a blocker that prevents progress.
#
# Arguments:
#   $1 - Phase or group where the blocker occurred
#   $2 - Blocker message
#   $3 - Severity: high, medium, low
_report_add_blocker() {
  local phase="${1:?Usage: _report_add_blocker phase message [severity]}"
  local message="${2:-}"
  local severity="${3:-high}"
  local entry
  entry=$(jq -nc \
    --arg phase "$phase" \
    --arg message "$message" \
    --arg severity "$severity" \
    '{phase: $phase, message: $message, severity: $severity}')
  _REPORT_BLOCKERS+=("$entry")
}

# ── _report_add_evidence(path) ───────────────────────────────────────────────
# Add an evidence file path to the evidence index.
#
# Arguments:
#   $1 - File path
_report_add_evidence() {
  local path="${1:?Usage: _report_add_evidence path}"
  _REPORT_EVIDENCE_PATHS+=("$path")
}

# ── _report_set_verdict ──────────────────────────────────────────────────────
# Compute overall verdict based on collected results.
#
# Verdict logic:
#   blocked       - Deploy failed (fresh or existing deploy result is fail)
#   fail          - Core group (A/B/C) failures exist
#   partial       - Some non-core groups failed
#   pass          - All tested groups pass (at least one group tested)
#   not-run       - No groups tested at all
#
# Verdict is NOT "pass" when groups are not-run.
_report_set_verdict() {
  # Check for blockers first (deploy failures)
  local blocker
  for blocker in "${_REPORT_BLOCKERS[@]}"; do
    local b_phase
    b_phase=$(echo "$blocker" | jq -r '.phase')
    # Deploy-phase blockers mean blocked
    if [[ "$b_phase" == "deploy" || "$b_phase" == "bootstrap" ]]; then
      _REPORT_OVERALL_VERDICT="blocked"
      return 0
    fi
  done

  # Check deploy results
  if [[ "$_REPORT_FRESH_DEPLOY_RESULT" == "fail" || "$_REPORT_EXISTING_INSTALL_RESULT" == "fail" ]]; then
    _REPORT_OVERALL_VERDICT="blocked"
    return 0
  fi

  # Count group statuses
  local n_total=0 n_pass=0 n_fail=0 n_skip_disabled=0 n_not_run=0
  local core_fail=false non_core_fail=false
  local entry
  for entry in "${_REPORT_FEATURE_GROUPS[@]}"; do
    local g_status g_group
    g_status=$(echo "$entry" | jq -r '.status')
    g_group=$(echo "$entry" | jq -r '.group')
    n_total=$((n_total + 1))

    case "$g_status" in
      pass)         n_pass=$((n_pass + 1)) ;;
      fail)         n_fail=$((n_fail + 1))
                    # Core groups: A (Matrix), B (Studio), C (Secretary)
                    if [[ "$g_group" == A* || "$g_group" == B* || "$g_group" == C* ]]; then
                      core_fail=true
                    else
                      non_core_fail=true
                    fi
                    ;;
      skip-disabled) n_skip_disabled=$((n_skip_disabled + 1)) ;;
      not-run)       n_not_run=$((n_not_run + 1)) ;;
    esac
  done

  # No groups tested at all
  if [[ $n_total -eq 0 || $((n_pass + n_fail)) -eq 0 ]]; then
    _REPORT_OVERALL_VERDICT="not-run"
    return 0
  fi

  # Core failures → fail
  if [[ "$core_fail" == "true" ]]; then
    _REPORT_OVERALL_VERDICT="fail"
    return 0
  fi

  # Any failures → partial
  if [[ "$non_core_fail" == "true" ]]; then
    _REPORT_OVERALL_VERDICT="partial"
    return 0
  fi

  # All tested groups pass (remaining are skip-disabled or not-run)
  _REPORT_OVERALL_VERDICT="pass"
}

# ── _report_build_json ───────────────────────────────────────────────────────
# Build the full JSON report structure. Echoes JSON to stdout.
_report_build_json() {
  # Build phases array
  local phases_json="[]"
  local entry
  for entry in "${_REPORT_PHASES[@]}"; do
    phases_json=$(echo "$phases_json" | jq --argjson e "$entry" '. + [$e]')
  done

  # Build feature_groups array with per-group matrix
  local groups_json="[]"
  for entry in "${_REPORT_FEATURE_GROUPS[@]}"; do
    local g_group g_status g_details g_evidence
    g_group=$(echo "$entry" | jq -r '.group')
    g_status=$(echo "$entry" | jq -r '.status')
    g_details=$(echo "$entry" | jq -r '.details')
    g_evidence=$(echo "$entry" | jq -r '.evidence_path')

    local group_obj
    group_obj=$(jq -nc \
      --arg group "$g_group" \
      --arg status "$g_status" \
      --arg details "$g_details" \
      --arg evidence "$g_evidence" \
      '{
        group: $group,
        status: $status,
        details: $details,
        evidence_path: $evidence,
        matrix: {
          pass:         (if $status == "pass" then true else false end),
          fail:         (if $status == "fail" then true else false end),
          skip_disabled:(if $status == "skip-disabled" then true else false end),
          not_run:      (if $status == "not-run" then true else false end)
        }
      }')
    groups_json=$(echo "$groups_json" | jq --argjson g "$group_obj" '. + [$g]')
  done

  # Build blockers array
  local blockers_json="[]"
  for entry in "${_REPORT_BLOCKERS[@]}"; do
    blockers_json=$(echo "$blockers_json" | jq --argjson e "$entry" '. + [$e]')
  done

  # Build evidence_paths array
  local evidence_json="[]"
  for entry in "${_REPORT_EVIDENCE_PATHS[@]}"; do
    evidence_json=$(echo "$evidence_json" | jq --arg p "$entry" '. + [$p]')
  done

  # Compose full report
  jq -nc \
    --arg topology "$_REPORT_TOPOLOGY" \
    --arg deploy_mode "$_REPORT_DEPLOY_MODE" \
    --arg fresh_deploy "$_REPORT_FRESH_DEPLOY_RESULT" \
    --arg existing_install "$_REPORT_EXISTING_INSTALL_RESULT" \
    --arg matrix_bootstrap "$_REPORT_MATRIX_BOOTSTRAP_RESULT" \
    --argjson phases "$phases_json" \
    --argjson feature_groups "$groups_json" \
    --argjson blockers "$blockers_json" \
    --argjson evidence_paths "$evidence_json" \
    --arg verdict "$_REPORT_OVERALL_VERDICT" \
    --arg init_time "$_REPORT_INIT_TIME" \
    '{
      topology: $topology,
      deploy_mode: $deploy_mode,
      fresh_deploy_result: $fresh_deploy,
      existing_install_result: $existing_install,
      matrix_bootstrap_result: $matrix_bootstrap,
      phases: $phases,
      feature_groups: $feature_groups,
      blockers: $blockers,
      evidence_paths: $evidence_paths,
      overall_verdict: $verdict,
      timestamp: (now | todate),
      init_time: $init_time
    }'
}

# ── _report_emit_json ────────────────────────────────────────────────────────
# Write report.json to the output directory atomically (temp file + mv).
# Uses the same atomic write pattern as jetski/internal/sonar/reporter.go.
_report_emit_json() {
  if [[ -z "$_REPORT_OUTPUT_DIR" ]]; then
    echo "[REPORT] ERROR: _report_init not called" >&2
    return 1
  fi

  local report_json
  report_json=$(_report_build_json)

  local final_path="${_REPORT_OUTPUT_DIR}/report.json"
  local tmp_path="${_REPORT_OUTPUT_DIR}/.report.json.tmp"

  # Atomic write: write to temp, then rename
  echo "$report_json" > "$tmp_path"
  mv "$tmp_path" "$final_path"
}

# ── _report_emit_text ────────────────────────────────────────────────────────
# Write report.txt to the output directory with human-readable formatting.
# Includes visual bar chart per group (████░░░░), blockers section, evidence index.
_report_emit_text() {
  if [[ -z "$_REPORT_OUTPUT_DIR" ]]; then
    echo "[REPORT] ERROR: _report_init not called" >&2
    return 1
  fi

  local text_path="${_REPORT_OUTPUT_DIR}/report.txt"
  local tmp_path="${_REPORT_OUTPUT_DIR}/.report.txt.tmp"

  {
    echo "========================================="
    echo " VPS Lifecycle Report"
    echo " $(date -u +"%Y-%m-%d %H:%M:%S UTC")"
    echo "========================================="
    echo ""
    echo "Topology:        ${_REPORT_TOPOLOGY}"
    echo "Deploy Mode:     ${_REPORT_DEPLOY_MODE}"
    echo "Verdict:         ${_REPORT_OVERALL_VERDICT}"
    echo ""
    echo "─── Deploy Results ─────────────────────"
    echo "  Fresh Deploy:       ${_REPORT_FRESH_DEPLOY_RESULT}"
    echo "  Existing Install:   ${_REPORT_EXISTING_INSTALL_RESULT}"
    echo "  Matrix Bootstrap:   ${_REPORT_MATRIX_BOOTSTRAP_RESULT}"
    echo ""

    # ── Feature Group Bar Chart ────────────────────────────
    echo "─── Feature Groups ─────────────────────"
    local entry
    for entry in "${_REPORT_FEATURE_GROUPS[@]}"; do
      local g_group g_status g_details
      g_group=$(echo "$entry" | jq -r '.group')
      g_status=$(echo "$entry" | jq -r '.status')
      g_details=$(echo "$entry" | jq -r '.details')

      # Visual bar: 8 chars
      local bar=""
      case "$g_status" in
        pass)
          bar="████████"
          ;;
        fail)
          bar="████░░░░"
          ;;
        skip-disabled)
          bar="░░░░░░░░"
          ;;
        not-run)
          bar="────────"
          ;;
        *)
          bar="????????"
          ;;
      esac

      # Status label with padding
      local status_label
      case "$g_status" in
        pass)         status_label="PASS" ;;
        fail)         status_label="FAIL" ;;
        skip-disabled) status_label="SKIP" ;;
        not-run)       status_label="NOT RUN" ;;
        *)             status_label="$g_status" ;;
      esac

      echo "  ${bar} ${g_group}  [${status_label}]"
      if [[ -n "$g_details" && "$g_details" != "null" ]]; then
        echo "              ${g_details}"
      fi
    done

    # Handle empty groups
    if [[ ${#_REPORT_FEATURE_GROUPS[@]} -eq 0 ]]; then
      echo "  (no feature groups recorded)"
    fi
    echo ""

    # ── Blockers ───────────────────────────────────────────
    echo "─── Blockers ───────────────────────────"
    if [[ ${#_REPORT_BLOCKERS[@]} -eq 0 ]]; then
      echo "  (none)"
    else
      local entry
      for entry in "${_REPORT_BLOCKERS[@]}"; do
        local b_phase b_message b_severity
        b_phase=$(echo "$entry" | jq -r '.phase')
        b_message=$(echo "$entry" | jq -r '.message')
        b_severity=$(echo "$entry" | jq -r '.severity')
        echo "  [${b_severity^^}] ${b_phase}: ${b_message}"
      done
    fi
    echo ""

    # ── Evidence Index ─────────────────────────────────────
    echo "─── Evidence Index ─────────────────────"
    if [[ ${#_REPORT_EVIDENCE_PATHS[@]} -eq 0 ]]; then
      echo "  (no evidence files)"
    else
      local i=1
      local entry
      for entry in "${_REPORT_EVIDENCE_PATHS[@]}"; do
        echo "  ${i}. ${entry}"
        i=$((i + 1))
      done
    fi
    echo ""

    echo "========================================="
    echo " Verdict: ${_REPORT_OVERALL_VERDICT}"
    echo " Report:  ${_REPORT_OUTPUT_DIR}/report.json"
    echo "========================================="
  } > "$tmp_path"

  mv "$tmp_path" "$text_path"
}

# ── _report_get_verdict ──────────────────────────────────────────────────────
# Echo the current overall verdict to stdout.
_report_get_verdict() {
  echo "${_REPORT_OVERALL_VERDICT}"
}

# ── _report_get_output_dir ───────────────────────────────────────────────────
# Echo the configured output directory to stdout.
_report_get_output_dir() {
  echo "${_REPORT_OUTPUT_DIR}"
}
