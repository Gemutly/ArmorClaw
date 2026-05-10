#!/usr/bin/env bash
# run-test.sh — Test runner for Matrix E2E test cases
#
# Usage: ./run-test.sh [case_file ...]
#   If no case files given, runs all cases/case-*.sh files.
#   Cleans up Conduit + Bridge on exit.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

source "$SCRIPT_DIR/harness.sh"

# Cleanup trap — always tear down on exit
trap harness_stop EXIT

# ── run_case <case_file> ──────────────────────────────────────────────────────
run_case() {
  local case_file="$1"
  local case_name
  case_name="$(basename "$case_file" .sh)"

  echo ""
  echo -e "${YELLOW}─── Running: $case_name ───${NC}"

  if ! bash "$case_file"; then
    echo -e "${RED}═══ FAILED: $case_name ═══${NC}"
    return 1
  fi

  echo -e "${GREEN}═══ PASSED: $case_name ═══${NC}"
  return 0
}

# ── main ──────────────────────────────────────────────────────────────────────
main() {
  check_dependencies || exit 1

  echo -e "${YELLOW}Matrix E2E Test Runner${NC}"
  echo "  Script dir: $SCRIPT_DIR"

  # Start the harness
  if ! harness_start; then
    echo -e "${RED}FATAL: Harness failed to start${NC}"
    exit 1
  fi

  # Determine case files
  local -a cases=()
  if [[ $# -gt 0 ]]; then
    cases=("$@")
  else
    while IFS= read -r f; do
      cases+=("$f")
    done < <(find "$SCRIPT_DIR/cases" -name 'case-*.sh' -type f 2>/dev/null | sort)
  fi

  if [[ ${#cases[@]} -eq 0 ]]; then
    echo -e "${YELLOW}No test cases found in $SCRIPT_DIR/cases/${NC}"
    exit 0
  fi

  echo ""
  echo -e "${YELLOW}Running ${#cases[@]} test case(s)${NC}"

  local failed=0
  for case_file in "${cases[@]}"; do
    if ! run_case "$case_file"; then
      ((failed++)) || true
    fi
  done

  echo ""
  echo "══════════════════════════════════════════"
  echo -e "  Total: ${#cases[@]}"
  echo -e "  ${GREEN}Passed: $(( ${#cases[@]} - failed ))${NC}"
  echo -e "  ${RED}Failed: $failed${NC}"
  echo "══════════════════════════════════════════"

  if [[ $failed -gt 0 ]]; then
    return 1
  fi
  return 0
}

main "$@"
