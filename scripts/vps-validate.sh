#!/usr/bin/env bash
# vps-validate.sh — Orchestrator for VPS validation phases
#
# Phases:
#   smoke — Infrastructure + Matrix CLI validation (safe, read-only)
#   full  — Smoke + A4 harness feature tests (assumes VPS already running)
#
# Usage:
#   MODE=smoke bash scripts/vps-validate.sh
#   MODE=full bash scripts/vps-validate.sh

set -euo pipefail

# ── Paths ─────────────────────────────────────────────────────────────────────
_SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
_REPO_ROOT="$(cd "${_SCRIPT_DIR}/.." && pwd)"

# ── Source .env ───────────────────────────────────────────────────────────────
set -a
source "${_REPO_ROOT}/.env" 2>/dev/null || true
set +a

# ── Environment variables with defaults ───────────────────────────────────────
: "${VPS_IP:?VPS_IP is required — set in .env or export manually}"
: "${VPS_USER:=root}"
: "${BRIDGE_PORT:=8080}"
: "${MATRIX_PORT:=6167}"
: "${SSH_KEY_PATH:=~/.ssh/id_ed25519}"

# ── Mode ──────────────────────────────────────────────────────────────────────
MODE="${MODE:-smoke}"

# ── Evidence directory ────────────────────────────────────────────────────────
EVIDENCE_DIR="${_REPO_ROOT}/.sisyphus/evidence/vps-validate"
mkdir -p "${EVIDENCE_DIR}"
START_TIME=$(date +%s)

# ── Env var translation for vps-matrix-cli-test.sh ────────────────────────────
if [[ "$MATRIX_PORT" == "443" ]]; then
  MATRIX_BASE_URL="https://${VPS_IP}:${MATRIX_PORT}"
else
  MATRIX_BASE_URL="http://${VPS_IP}:${MATRIX_PORT}"
fi
export MATRIX_BASE_URL
export MATRIX_USER="${ARMORCLAW_ADMIN_USERNAME:-admin}"
export MATRIX_PASSWORD="${ARMORCLAW_ADMIN_PASSWORD:-}"
export MATRIX_ROOM_ID="${MATRIX_ROOM_ID:-}"
export VPS_IP
export SSH_KEY_PATH

# ── Result accumulators ───────────────────────────────────────────────────────
SMOKE_PASS=0
SMOKE_FAIL=0
SMOKE_SKIP=0
SMOKE_STATUS=""
FULL_PASS=0
FULL_FAIL=0
FULL_SKIP=0
FULL_STATUS=""

# ── Usage ─────────────────────────────────────────────────────────────────────
usage() {
  echo "Usage: MODE=smoke|full bash scripts/vps-validate.sh"
  echo ""
  echo "Modes:"
  echo "  smoke  — Infrastructure + Matrix CLI validation (safe, read-only)"
  echo "  full   — Smoke + A4 harness feature tests (assumes VPS already running)"
  echo ""
  echo "Required env vars (or set in .env):"
  echo "  VPS_IP              — VPS IP address"
  echo "  SSH_KEY_PATH        — Path to SSH private key"
  echo "  ARMORCLAW_ADMIN_PASSWORD — Matrix admin password"
  echo ""
  echo "Optional env vars:"
  echo "  VPS_USER            — SSH user (default: root)"
  echo "  BRIDGE_PORT         — Bridge HTTP port (default: 8080)"
  echo "  MATRIX_PORT         — Matrix Conduit port (default: 6167)"
  echo "  MATRIX_ROOM_ID      — Matrix room ID for testing"
  echo "  SUITES              — A4 test suites to run (default: all)"
  exit 1
}

# ── Phase 1: Smoke ───────────────────────────────────────────────────────────
run_smoke() {
  echo "========================================="
  echo " Phase 1: Infrastructure + Matrix CLI (Smoke)"
  echo "========================================="

  local cli_output
  local cli_exit=0

  MODE=smoke cli_output=$(bash "${_SCRIPT_DIR}/vps-matrix-cli-test.sh" 2>&1) || cli_exit=$?

  # Save captured output
  echo "$cli_output" > "${EVIDENCE_DIR}/matrix-cli-output.txt"

  # Parse results: grep for "Results: N PASS | N FAIL | N SKIP"
  SMOKE_PASS=$(echo "$cli_output" | grep -oP 'Results: \K\d+' | head -1 || echo "0")
  SMOKE_FAIL=$(echo "$cli_output" | grep -oP 'Results: \d+ PASS \| \K\d+' | head -1 || echo "0")
  SMOKE_SKIP=$(echo "$cli_output" | grep -oP 'Results: \d+ PASS \| \d+ FAIL \| \K\d+' | head -1 || echo "0")

  # Also count from individual lines if Results line not found
  if [[ "$SMOKE_PASS" == "0" && "$SMOKE_FAIL" == "0" ]]; then
    SMOKE_PASS=$(echo "$cli_output" | grep -c "^PASS:" || echo "0")
    SMOKE_FAIL=$(echo "$cli_output" | grep -c "^FAIL:" || echo "0")
    SMOKE_SKIP=$(echo "$cli_output" | grep -c "^SKIP:" || echo "0")
  fi

  echo "$cli_output"
  echo ""

  if [[ $cli_exit -ne 0 ]]; then
    SMOKE_STATUS="FAIL"
  else
    SMOKE_STATUS="PASS"
  fi
}

# ── Phase 2: Full (A4 harness integration) ────────────────────────────────────
run_full() {
  echo "========================================="
  echo " Phase 2: A4 Harness Feature Tests (Full)"
  echo "========================================="

  # Source contract.sh inside run_full (not top-level) — contract.sh sources
  # load_env.sh which hard-fails on missing VPS_IP, but that's already checked at line 24
  source "${_SCRIPT_DIR}/lib/contract.sh"

  local suites="${SUITES:-health,eventbus,trust,workflow-core,email,workflow-deep,sidecar-docs,voice,jetski,license,platform,agent-runtime}"

  local harness_output
  local harness_exit=0
  harness_output=$(bash "${_SCRIPT_DIR}/a4_harness.sh" "${suites}" 2>&1) || harness_exit=$?

  echo "$harness_output"
  echo ""

  local summary_file="${_REPO_ROOT}/.sisyphus/evidence/armorclaw/a4_summary.json"
  if [[ -f "$summary_file" ]]; then
    FULL_PASS=$(jq -r '.pass // 0' "$summary_file")
    FULL_FAIL=$(jq -r '.fail // 0' "$summary_file")
    FULL_SKIP=$(jq -r '.skip // 0' "$summary_file")
    cp "$summary_file" "${EVIDENCE_DIR}/a4-summary.json"
  fi
  if [[ $harness_exit -ne 0 ]]; then
    FULL_STATUS="FAIL"
  else
    FULL_STATUS="PASS"
  fi

  echo "Full phase complete. Status: ${FULL_STATUS}"
}

# ── Report Generation ────────────────────────────────────────────────────────
generate_report() {
  local total_smoke=$(( SMOKE_PASS + SMOKE_FAIL + SMOKE_SKIP ))
  local infra_score=0
  if [[ $total_smoke -gt 0 ]]; then
    infra_score=$(( (SMOKE_PASS * 100) / total_smoke ))
  fi

  local total_full=$(( FULL_PASS + FULL_FAIL + FULL_SKIP ))
  local feature_score=0
  if [[ $total_full -gt 0 ]]; then
    feature_score=$(( (FULL_PASS * 100) / total_full ))
  fi

  local overall_score=$infra_score
  if [[ "$MODE" == "full" ]]; then
    overall_score=$(( (infra_score * 40 + feature_score * 60) / 100 ))
  fi

  local infra_status="PASS"
  [[ $SMOKE_FAIL -gt 0 ]] && infra_status="FAIL"

  local feature_status="NOT_RUN"
  if [[ "$MODE" == "full" ]]; then
    feature_status="PASS"
    [[ $FULL_FAIL -gt 0 ]] && feature_status="FAIL"
  fi

  local overall_status="PASS"
  [[ $overall_score -lt 80 ]] && overall_status="FAIL"

  # Build evidence_paths array
  local evidence_paths="[\"${EVIDENCE_DIR}/matrix-cli-output.txt\"]"
  if [[ "$MODE" == "full" && -f "${EVIDENCE_DIR}/a4-summary.json" ]]; then
    evidence_paths="[\"${EVIDENCE_DIR}/matrix-cli-output.txt\", \"${EVIDENCE_DIR}/a4-summary.json\"]"
  fi

  # Generate JSON report
  local report
  if [[ "$MODE" == "full" ]]; then
    report=$(jq -nc \
      --arg mode "$MODE" \
      --arg vps_ip "$VPS_IP" \
      --argjson overall "$overall_score" \
      --argjson duration "$(($(date +%s) - START_TIME))" \
      --arg infra_status "$infra_status" \
      --argjson infra_score "$infra_score" \
      --argjson s_pass "$SMOKE_PASS" \
      --argjson s_fail "$SMOKE_FAIL" \
      --argjson s_skip "$SMOKE_SKIP" \
      --arg feature_status "$feature_status" \
      --argjson feature_score "$feature_score" \
      --argjson f_pass "$FULL_PASS" \
      --argjson f_fail "$FULL_FAIL" \
      --argjson f_skip "$FULL_SKIP" \
      --argjson evidence "$evidence_paths" \
      '{
        mode: $mode,
        vps_ip: $vps_ip,
        overall_score: $overall,
        duration_seconds: $duration,
        timestamp: (now | todate),
        evidence_paths: $evidence,
        layers: {
          infra_and_cli: {
            score: $infra_score,
            status: $infra_status,
            pass: $s_pass,
            fail: $s_fail,
            skip: $s_skip,
            source: "vps-matrix-cli-test.sh"
          },
          feature_suites: {
            score: $feature_score,
            status: $feature_status,
            pass: $f_pass,
            fail: $f_fail,
            skip: $f_skip,
            source: "a4_harness.sh"
          }
        },
        recommendations: []
      }')
  else
    report=$(jq -nc \
      --arg mode "$MODE" \
      --arg vps_ip "$VPS_IP" \
      --argjson overall "$overall_score" \
      --argjson duration "$(($(date +%s) - START_TIME))" \
      --arg infra_status "$infra_status" \
      --argjson infra_score "$infra_score" \
      --argjson s_pass "$SMOKE_PASS" \
      --argjson s_fail "$SMOKE_FAIL" \
      --argjson s_skip "$SMOKE_SKIP" \
      --argjson evidence "$evidence_paths" \
      '{
        mode: $mode,
        vps_ip: $vps_ip,
        overall_score: $overall,
        duration_seconds: $duration,
        timestamp: (now | todate),
        evidence_paths: $evidence,
        layers: {
          infra_and_cli: {
            score: $infra_score,
            status: $infra_status,
            pass: $s_pass,
            fail: $s_fail,
            skip: $s_skip,
            source: "vps-matrix-cli-test.sh"
          },
          feature_suites: {
            status: "not_run"
          }
        },
        recommendations: []
      }')
  fi

  echo "$report" > "${EVIDENCE_DIR}/report.json"

  # Print human-readable summary
  echo ""
  echo "========================================="
  echo " VPS Validation Report"
  echo " Mode: ${MODE} | Score: ${overall_score}/100"
  echo "========================================="
  echo " Infrastructure + CLI: ${SMOKE_PASS} PASS, ${SMOKE_FAIL} FAIL, ${SMOKE_SKIP} SKIP"
  if [[ "$MODE" == "full" ]]; then
    echo " Feature Suites: ${FULL_PASS} PASS, ${FULL_FAIL} FAIL, ${FULL_SKIP} SKIP"
  else
    echo " Feature Suites: not run (smoke mode)"
  fi
  echo "========================================="
  echo " Overall: ${overall_status}"
  echo " Report: ${EVIDENCE_DIR}/report.json"
  echo "========================================="

  # Exit based on score
  if [[ $overall_score -lt 80 ]]; then
    exit 1
  fi
}

# ── Main ──────────────────────────────────────────────────────────────────────
case "$MODE" in
  smoke) run_smoke ;;
  full)  run_smoke && run_full ;;
  *)     usage ;;
esac

generate_report
