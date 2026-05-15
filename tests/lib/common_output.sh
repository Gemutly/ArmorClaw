#!/usr/bin/env bash
# common_output.sh — Test output and counter helpers for full-system test harness
#
# Provides PASS/FAIL/SKIP counters and formatted output functions.
# These are separate from tests/e2e/common.sh counters (TESTS_RUN/PASSED/FAILED)
# so full-system tests can track their own results independently.
#
# Requires: GREEN, RED, YELLOW, NC color variables (from tests/e2e/common.sh)

# ── Counters ──────────────────────────────────────────────────────────────────
export FULL_SYSTEM_PASSED=0
export FULL_SYSTEM_FAILED=0
export FULL_SYSTEM_SKIPPED=0
export FULL_SYSTEM_GATED_EXPECTED=0
export FULL_SYSTEM_ENV_MISSING=0

# Fallback color definitions (may already be set by tests/e2e/common.sh)
CYAN="${CYAN:-\033[36m}"
MAGENTA="${MAGENTA:-\033[35m}"

# ── log_pass <message> ────────────────────────────────────────────────────────
log_pass() {
  local msg="$1"
  FULL_SYSTEM_PASSED=$((FULL_SYSTEM_PASSED + 1))
  echo -e "${GREEN}[PASS]${NC} $msg"
}

# ── log_fail <message> ────────────────────────────────────────────────────────
log_fail() {
  local msg="$1"
  FULL_SYSTEM_FAILED=$((FULL_SYSTEM_FAILED + 1))
  echo -e "${RED}[FAIL]${NC} $msg"
}

# ── log_skip <message> ────────────────────────────────────────────────────────
log_skip() {
  local msg="$1"
  FULL_SYSTEM_SKIPPED=$((FULL_SYSTEM_SKIPPED + 1))
  echo -e "${YELLOW}[SKIP]${NC} $msg"
}

# ── log_gated_expected <message> ────────────────────────────────────────────────
log_gated_expected() {
  local msg="$1"
  FULL_SYSTEM_GATED_EXPECTED=$((FULL_SYSTEM_GATED_EXPECTED + 1))
  echo -e "${CYAN}[GATED]${NC} $msg"
}

# ── log_env_missing <message> ────────────────────────────────────────────────────
log_env_missing() {
  local msg="$1"
  FULL_SYSTEM_ENV_MISSING=$((FULL_SYSTEM_ENV_MISSING + 1))
  echo -e "${MAGENTA}[ENV_MISSING]${NC} $msg"
}

# ── log_info <message> ────────────────────────────────────────────────────────
log_info() {
  local msg="$1"
  echo "[INFO] $msg"
}

# ── harness_summary ───────────────────────────────────────────────────────────
# Prints final PASS/FAIL/SKIP counts.  Returns 0 if no failures, 1 otherwise.
harness_summary() {
  echo ""
  echo "========================================="
  echo " Full-System Test Summary"
  echo "========================================="
  echo -e " ${GREEN}Passed:${NC}  $FULL_SYSTEM_PASSED"
  echo -e " ${RED}Failed:${NC}  $FULL_SYSTEM_FAILED"
  echo -e " ${YELLOW}Skipped:${NC} $FULL_SYSTEM_SKIPPED"
  echo -e " ${CYAN}Gated:${NC}  $FULL_SYSTEM_GATED_EXPECTED"
  echo -e " ${MAGENTA}Env Missing:${NC} $FULL_SYSTEM_ENV_MISSING"
  echo "========================================="

  if [[ $FULL_SYSTEM_FAILED -eq 0 ]]; then
    echo -e "${GREEN}ALL TESTS PASSED${NC}"
    return 0
  else
    echo -e "${RED}$FULL_SYSTEM_FAILED TEST(S) FAILED${NC}"
    return 1
  fi
}
