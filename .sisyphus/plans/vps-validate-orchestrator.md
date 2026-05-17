# VPS Validate Orchestrator

## TL;DR

> **Quick Summary**: Create a thin orchestrator script (`scripts/vps-validate.sh`) that combines the existing `vps-matrix-cli-test.sh` (infrastructure + Matrix CLI) and the Plan-A harness (`a4_harness.sh`) into a single agent-callable entry point with unified JSON output.
> 
> **Deliverables**:
> - `scripts/vps-validate.sh` — ~150-line orchestrator (new file)
> - Unified JSON report at `.sisyphus/evidence/vps-validate-report.json`
> - Per-layer scores + overall health score (0-100)
> 
> **Estimated Effort**: Quick (3 tasks, ~2-3 hours)
> **Parallel Execution**: YES — 2 waves
> **Critical Path**: Task 1 (skeleton+smoke) → Task 2 (full mode) → Task 3 (JSON report)

---

## Context

### Original Request
Create a single, agent-driven way to validate that ArmorClaw features work on a VPS.

### Interview Summary
**Key Discussions**:
- `vps-matrix-cli-test.sh` (625 lines) already validates infrastructure + Matrix CLI commands (L1-L2)
- Plan-A pipeline (`a_run_all.sh` + `a4_harness.sh`) already validates 20 feature suites (L3-L6)
- Neither is agent-callable through a single entry point
- No unified JSON report exists

**Research Findings**:
- A3 (event validation) requires `a2_matrix_session.json` from A2 provisioning — degrades gracefully if missing (SKIP)
- A4 (harness) runs independently — no A2/A3 dependency
- The two scripts use different env var conventions (Plan-A uses `.env`, vps-matrix-cli-test.sh reads env vars)
- Evidence paths differ: `task-4-*.json` vs `.sisyphus/evidence/armorclaw/`

### Metis Review
**Identified Gaps** (addressed):
- A3+A4 dependency: Resolved — full mode will run **A4 only** (harness). A3 skipped because it needs A2 session file.
- Text parsing fragility: Accepted — output format is stable (`PASS:`, `FAIL:`, `SKIP:`, `Results: N PASS | N FAIL | N SKIP`)
- Concurrent evidence writes: Not an issue — scripts write to different subdirectories
- 150-line estimate: Realistic — pure orchestration, no test logic

---

## Work Objectives

### Core Objective
Create `scripts/vps-validate.sh` that orchestrates existing validation tools into a single agent-callable command with JSON output.

### Concrete Deliverables
- `scripts/vps-validate.sh` — new file, ~150 lines
- `.sisyphus/evidence/vps-validate/report.json` — generated at runtime
- `.sisyphus/evidence/vps-validate/matrix-cli-output.txt` — captured stdout from smoke mode
- `.sisyphus/evidence/vps-validate/a4-summary.json` — copied from Plan-A evidence dir

### Definition of Done
- [ ] `bash -n scripts/vps-validate.sh` passes (syntax check)
- [ ] `MODE=smoke bash scripts/vps-validate.sh` runs `vps-matrix-cli-test.sh` and captures results
- [ ] `MODE=full bash scripts/vps-validate.sh` runs both `vps-matrix-cli-test.sh` + `a4_harness.sh`
- [ ] JSON report is valid jq-parsable output with overall_score and per-layer breakdown
- [ ] No changes to existing scripts (`vps-matrix-cli-test.sh`, `a_run_all.sh`, `a4_harness.sh`)

### Scope Decision: Why A3 Is Excluded
> **Decision**: Full mode runs **A4 only** (harness). A3 (event validation) is excluded because it depends on `a2_matrix_session.json` from the provisioning phase (A2). Running A3 without A2 would produce unreliable SKIP results. A4 runs independently — no A2/A3 dependency.

### Assumptions
- VPS is already running (no deployment needed)
- `.env` file exists with required variables (or they are passed via environment)
- Bridge is accessible at `VPS_IP:BRIDGE_PORT`
- Matrix Conduit is accessible at `VPS_IP:MATRIX_PORT`

### Must Have
- Two modes: `smoke` (L1-L2 only) and `full` (L1-L2 + A4 harness — smoke always runs first)
- JSON structured output with per-layer scores
- `.env` file sourcing (consistent with Plan-A)
- Env var translation from `.env` → `vps-matrix-cli-test.sh` format
- Evidence aggregation
- Idempotent and safe for re-runs

### Must NOT Have (Guardrails)
- No modifications to `vps-matrix-cli-test.sh` (brownfield — don't touch working code)
- No modifications to Plan-A pipeline scripts (`a_run_all.sh`, `a4_harness.sh`, `contract.sh`)
- No new test cases or test logic
- No interactive prompts
- No hardcoded credentials or VPS IPs
- No deployment or provisioning steps (VPS assumed already running)
- No dependency on A2 session (A3 is intentionally excluded from full mode)

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: N/A (no Go/unit tests for bash scripts)
- **Automated tests**: bash -n syntax check + dry-run execution
- **Framework**: bash

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately):
└── Task 1: Script skeleton + .env sourcing + env var translation + smoke mode [quick]

Wave 2 (After Wave 1):
├── Task 2: Full mode — A4 harness integration + result capture [quick]
└── Task 3: JSON report generation + scoring + evidence aggregation [quick]

Wave FINAL (After ALL tasks):
├── F1: Plan compliance audit (oracle)
├── F2: Syntax + shellcheck + dry-run (unspecified-high)
├── F3: End-to-end execution QA (unspecified-high)
└── F4: Scope fidelity check (deep)

Critical Path: Task 1 → Task 2/3 → F1-F4
Parallel Speedup: Wave 2 has 2 parallel tasks
Max Concurrent: 2 (Wave 2)
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| 1 | - | 2, 3, F1-F4 |
| 2 | 1 | F1-F4 |
| 3 | 1 | F1-F4 |

### Agent Dispatch Summary

- **Wave 1**: 1 task — T1 → `quick`
- **Wave 2**: 2 tasks — T2 → `quick`, T3 → `quick`
- **FINAL**: 4 tasks — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Script Skeleton + .env Sourcing + Env Var Translation + Smoke Mode

  **What to do**:
  - Create `scripts/vps-validate.sh` with `set -euo pipefail`
  - Source `.env` file (use same pattern as `tests/lib/load_env.sh` lines 14-20: `set -a; source .env; set +a`)
  - Define required env vars with defaults: `VPS_IP`, `VPS_USER=root`, `BRIDGE_PORT=8080`, `MATRIX_PORT=6167`, `SSH_KEY_PATH`
  - Hard-fail if `VPS_IP` is empty (matches `load_env.sh` line 23 pattern)
  - **Env var translation**: Export Matrix-specific vars for `vps-matrix-cli-test.sh`:

    | From `.env`                  | To `vps-matrix-cli-test.sh`     | Notes |
    |-----------------------------|----------------------------------|-------|
    | `VPS_IP` + `MATRIX_PORT`    | `MATRIX_BASE_URL`                | Construct `http://$VPS_IP:$MATRIX_PORT` (https if port 443) |
    | `ARMORCLAW_ADMIN_USERNAME`  | `MATRIX_USER`                    | Fallback to `admin` |
    | `ARMORCLAW_ADMIN_PASSWORD`  | `MATRIX_PASSWORD`                | Required |
    | `MATRIX_ROOM_ID`            | `MATRIX_ROOM_ID`                 | Direct pass-through |
    | `VPS_IP`                    | `VPS_IP`                         | Direct pass-through |
    | `SSH_KEY_PATH`              | `SSH_KEY_PATH`                   | Direct pass-through |
  - Parse `MODE` env var: `smoke` (default) or `full`
  - Implement `run_smoke()` function:
    - Call `bash "$(dirname "$0")/vps-matrix-cli-test.sh"` with MODE=smoke
    - Capture stdout and exit code
    - Parse output: grep for `^PASS:`, `^FAIL:`, `^SKIP:` lines and `Results: (\d+) PASS \| (\d+) FAIL \| (\d+) SKIP`
    - Store counts in variables
  - **Error handling**: Capture exit code from `vps-matrix-cli-test.sh`. If non-zero, still continue (don't abort) — set `infra_and_cli.status = "FAIL"` and generate partial JSON report. Exit with code 1 at the end only after report generation.
  - Implement main routing: `case "$MODE" in smoke) run_smoke ;; full) run_smoke && run_full ;; *) usage ;; esac`
  - Add usage/help output
  - Evidence directory: ensure `.sisyphus/evidence/vps-validate/` exists
  - Save captured stdout from `vps-matrix-cli-test.sh` to `.sisyphus/evidence/vps-validate/matrix-cli-output.txt`

  **Must NOT do**:
  - Do not modify `vps-matrix-cli-test.sh`
  - Do not add test logic — only orchestrate
  - Do not implement `run_full()` yet (Task 2)
  - Do not implement JSON report yet (Task 3)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1 (sequential)
  - **Blocks**: Tasks 2, 3, F1-F4
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `tests/lib/load_env.sh:14-20` — .env sourcing pattern (`set -a; source .env; set +a`)
  - `tests/lib/load_env.sh:23-31` — env var definition with defaults and exports
  - `scripts/vps-matrix-cli-test.sh:50-64` — PASS/FAIL/SKIP output format (log_pass, log_fail, log_skip functions)
  - `scripts/vps-matrix-cli-test.sh:617-618` — Results summary line format: `Results: N PASS | N FAIL | N SKIP`

  **API/Type References**:
  - `scripts/vps-matrix-cli-test.sh:20-34` — All env vars it reads (MODE, VPS_IP, SSH_KEY_PATH, MATRIX_BASE_URL, etc.)
  - `tests/lib/load_env.sh:23-29` — All env vars Plan-A reads (VPS_IP, VPS_USER, BRIDGE_PORT, MATRIX_PORT, SSH_KEY_PATH, ADMIN_TOKEN)

  **WHY Each Reference Matters**:
  - `load_env.sh` pattern: Copy the exact .env sourcing approach so the orchestrator behaves identically to Plan-A scripts
  - `vps-matrix-cli-test.sh` output format: Must match grep patterns exactly for reliable parsing
  - Env var lists: The translation layer must map every required var between the two conventions

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Syntax check passes
    Tool: Bash
    Steps:
      1. Run: bash -n scripts/vps-validate.sh
    Expected Result: Exit code 0, no output
    Failure Indicators: Syntax errors printed to stderr
    Evidence: .sisyphus/evidence/task-1-syntax-check.txt

  Scenario: Smoke mode runs vps-matrix-cli-test.sh
    Tool: Bash
    Preconditions: .env file exists with VPS_IP (can be dummy for syntax check)
    Steps:
      1. Run: MODE=smoke bash scripts/vps-validate.sh --help (or without args)
      2. Verify usage output is printed
    Expected Result: Usage message with MODE=smoke|full options
    Failure Indicators: Script exits with error, no usage output
    Evidence: .sisyphus/evidence/task-1-usage-check.txt

  Scenario: Missing VPS_IP causes hard failure
    Tool: Bash
    Steps:
      1. Run: VPS_IP="" bash scripts/vps-validate.sh
    Expected Result: Exit code 1, error message about missing VPS_IP
    Failure Indicators: Script proceeds without VPS_IP
    Evidence: .sisyphus/evidence/task-1-missing-vps-ip.txt
  ```

  **Commit**: YES
  - Message: `feat(scripts): add vps-validate.sh orchestrator — skeleton + smoke mode`
  - Files: `scripts/vps-validate.sh`
  - Pre-commit: `bash -n scripts/vps-validate.sh`

- [x] 2. Full Mode — A4 Harness Integration + Result Capture

  **What to do**:
  - Add `run_full()` function to `scripts/vps-validate.sh`
  - Source `scripts/lib/contract.sh` (same as `a_run_all.sh` line 7) — this gives `ssh_vps()`, `_contract_bridge_rpc()`, etc.
  - Call `bash "${_SCRIPT_DIR}/a4_harness.sh" "${SUITES}"` where SUITES defaults to `health,eventbus,trust,workflow-core,email,workflow-deep,sidecar-docs,voice,jetski,license,platform,agent-runtime` (all Tier A + Tier B suites, excluding destructive ones)
  - Allow `SUITES` env var override (e.g., `SUITES=health,trust`)
  - Capture exit code and stdout
  - Read `a4_summary.json` from `.sisyphus/evidence/armorclaw/` for structured results
  - Copy/symlink `a4_summary.json` to `.sisyphus/evidence/vps-validate/a4-summary.json`
  - Extract pass/fail/skip counts from `a4_summary.json` using jq
  - Store results in variables for Task 3 JSON report
  - Update main routing: `full) run_smoke && run_full ;;` — full mode **always runs smoke first** to validate baseline, then runs A4 harness

  **Must NOT do**:
  - Do not modify `a4_harness.sh` or `a_run_all.sh`
  - Do not call A0, A1, A2, or A3 (no deployment/provisioning)
  - Do not add new test suites
  - Do not implement JSON report yet (Task 3)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 3)
  - **Parallel Group**: Wave 2 (with Task 3)
  - **Blocks**: F1-F4
  - **Blocked By**: Task 1

  **References**:

  **Pattern References**:
  - `scripts/a_run_all.sh:16-36` — `run_phase()` pattern for calling phase scripts
  - `scripts/a_run_all.sh:52` — A4 invocation: `run_phase "a4_prepare" && run_phase "a4_harness"`
  - `scripts/a4_harness.sh:13` — Suite arg: `SUITES="${1:-health}"` (comma-separated)
  - `scripts/a4_harness.sh:20-41` — SUITE_MAP with all 20 suite names

  **API/Type References**:
  - `scripts/a4_harness.sh:81-92` — `a4_summary.json` structure: `{phase, pass, fail, skip, total, suites: {name: {status}}, timestamp}`
  - `scripts/lib/contract.sh:7` — Must source this for Plan-A infrastructure (`load_env.sh`, `ssh_vps()`, etc.)

  **WHY Each Reference Matters**:
  - `a_run_all.sh` pattern: Shows how Plan-A scripts call each other — follow the same pattern
  - `a4_harness.sh` SUITE_MAP: Defines all valid suite names — the orchestrator must only pass valid names
  - `a4_summary.json` structure: The JSON report (Task 3) needs to parse this exact shape

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Full mode calls a4_harness.sh
    Tool: Bash
    Preconditions: Task 1 complete, scripts/vps-validate.sh exists
    Steps:
      1. Grep scripts/vps-validate.sh for "a4_harness.sh"
    Expected Result: Match found — a4_harness.sh is called in run_full()
    Failure Indicators: No reference to a4_harness.sh
    Evidence: .sisyphus/evidence/task-2-harness-call.txt

  Scenario: a4_summary.json is read after harness run
    Tool: Bash
    Steps:
      1. Grep scripts/vps-validate.sh for "a4_summary.json"
    Expected Result: Match found — JSON evidence file is read with jq
    Failure Indicators: No reference to a4_summary.json
    Evidence: .sisyphus/evidence/task-2-summary-read.txt

  Scenario: A4 harness receives correct suite names
    Tool: Bash
    Steps:
      1. Grep scripts/vps-validate.sh for SUITE_MAP or suite name list
      2. Verify suite names match those in scripts/a4_harness.sh lines 20-41
    Expected Result: Default suites are a subset of valid SUITE_MAP keys
    Failure Indicators: Unknown suite names that aren't in SUITE_MAP
    Evidence: .sisyphus/evidence/task-2-suite-names.txt
  ```

  **Commit**: YES
  - Message: `feat(scripts): add full mode — A4 harness integration`
  - Files: `scripts/vps-validate.sh`
  - Pre-commit: `bash -n scripts/vps-validate.sh`

- [x] 3. JSON Report Generation + Scoring + Evidence Aggregation

  **What to do**:
  - Add `generate_report()` function to `scripts/vps-validate.sh`
  - Calculate per-layer scores:
    - `infra_and_cli`: from parsed `vps-matrix-cli-test.sh` output (pass/(pass+fail+skip) * 100)
    - `feature_suites`: from `a4_summary.json` (pass/(total) * 100)
    - `overall`: weighted average (infra_and_cli * 0.4 + feature_suites * 0.6 for full mode, or infra_and_cli for smoke mode)
  - Generate JSON report using `jq -nc` with this structure:
    ```json
    {
      "mode": "smoke|full",
      "vps_ip": "1.2.3.4",
      "overall_score": 87,
      "duration_seconds": 45,
      "timestamp": "2026-05-11T...",
      "evidence_paths": [
        ".sisyphus/evidence/vps-validate/matrix-cli-output.txt",
        ".sisyphus/evidence/vps-validate/a4-summary.json"
      ],
      "layers": {
        "infra_and_cli": {
          "score": 100,
          "status": "PASS",
          "pass": 7,
          "fail": 0,
          "skip": 0,
          "source": "vps-matrix-cli-test.sh"
        },
        "feature_suites": {
          "score": 82,
          "status": "PASS",
          "pass": 14,
          "fail": 2,
          "skip": 4,
          "suites": { "health": "passed", "trust": "failed", ... },
          "source": "a4_harness.sh"
        }
      },
      "recommendations": ["trust suite failed — check PII detection logs"]
    }
    ```
  - For smoke mode: `feature_suites` is omitted (or `"status": "not_run"`)
  - Write to `.sisyphus/evidence/vps-validate/report.json`
  - Print human-readable summary to stdout:
    ```
    ========================================
     VPS Validation Report
     Mode: smoke | Score: 100/100
    ========================================
     Infrastructure + CLI: 7 PASS, 0 FAIL
     Feature Suites: not run (smoke mode)
    ========================================
     Overall: PASS
     Report: .sisyphus/evidence/vps-validate-report.json
    ========================================
    ```
  - Add simple recommendation generation: if any layer has failures, add "check logs at .sisyphus/evidence/armorclaw/a4_{suite}_output.txt"
  - Exit 0 if overall_score >= 80 and no hard failures, exit 1 otherwise

  **Must NOT do**:
  - Do not add complex recommendation logic — keep it simple (just point to evidence)
  - Do not modify existing scripts
  - Do not add AI-based analysis

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 2)
  - **Parallel Group**: Wave 2 (with Task 2)
  - **Blocks**: F1-F4
  - **Blocked By**: Task 1

  **References**:

  **Pattern References**:
  - `scripts/a_run_all.sh:75-84` — JSON summary generation pattern using `jq -nc`
  - `scripts/a4_harness.sh:81-92` — `a4_summary.json` structure to parse

  **API/Type References**:
  - `scripts/vps-matrix-cli-test.sh:617-618` — Text output format for parsing: `Results: N PASS | N FAIL | N SKIP`

  **WHY Each Reference Matters**:
  - `a_run_all.sh` JSON pattern: Follow the same jq -nc approach for consistency with Plan-A
  - `a4_summary.json` structure: Must parse this exact JSON shape to extract suite results
  - Text format: Must match grep patterns to reliably extract PASS/FAIL/SKIP counts

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: JSON report is valid jq output
    Tool: Bash
    Preconditions: Task 1+2+3 complete
    Steps:
      1. Grep scripts/vps-validate.sh for "vps-validate-report.json"
      2. Verify jq -nc is used to generate JSON
    Expected Result: Report file path referenced, JSON generation via jq
    Failure Indicators: No report file path, JSON built via string concatenation
    Evidence: .sisyphus/evidence/task-3-json-generation.txt

  Scenario: Score calculation is correct
    Tool: Bash
    Steps:
      1. Grep scripts/vps-validate.sh for "overall_score" or score calculation
      2. Verify formula: pass/(total) * 100 pattern exists
    Expected Result: Score formula present and uses integer arithmetic
    Failure Indicators: No score calculation, or division by zero risk
    Evidence: .sisyphus/evidence/task-3-score-calc.txt

  Scenario: Human-readable summary printed
    Tool: Bash
    Steps:
      1. Grep scripts/vps-validate.sh for "Validation Report" or "Overall:"
    Expected Result: Summary header and pass/fail counts printed to stdout
    Failure Indicators: Only JSON output, no human-readable format
    Evidence: .sisyphus/evidence/task-3-summary.txt

  Scenario: Exit code reflects pass/fail
    Tool: Bash
    Steps:
      1. Grep scripts/vps-validate.sh for exit code logic (exit 0, exit 1)
    Expected Result: Exit 0 on pass, exit 1 on failure (score < 80 or hard failures)
    Failure Indicators: Always exits 0 regardless of results
    Evidence: .sisyphus/evidence/task-3-exit-code.txt
  ```

  **Commit**: YES
  - Message: `feat(scripts): add JSON report generation + scoring`
  - Files: `scripts/vps-validate.sh`
  - Pre-commit: `bash -n scripts/vps-validate.sh`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns. Check evidence files exist. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Syntax + Shellcheck + Dry-Run** — `unspecified-high`
  Run `bash -n scripts/vps-validate.sh`. Run `shellcheck scripts/vps-validate.sh` if available. Run `MODE=smoke bash scripts/vps-validate.sh --dry-run` (if --dry-run implemented). Check for: unquoted variables, missing error handling, unsafe pipes.
  Output: `Syntax [PASS/FAIL] | Shellcheck [PASS/FAIL/WARN] | Dry-run [PASS/FAIL] | VERDICT`

- [x] F3. **End-to-End Execution QA** — `unspecified-high`
  Read the script. Trace execution paths for both modes (smoke, full). Verify: env var translation is correct (MATRIX_BASE_URL derived from VPS_IP+MATRIX_PORT), .env sourcing works, sub-script calls use correct paths, JSON output is valid jq. Check error paths: missing .env, missing VPS_IP, script failures.
  Output: `Smoke path [PASS/FAIL] | Full path [PASS/FAIL] | Error paths [PASS/FAIL] | JSON valid [PASS/FAIL] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual file content. Verify no changes were made to existing scripts (`vps-matrix-cli-test.sh`, `a_run_all.sh`, `a4_harness.sh`, `contract.sh`). Verify new file only. Detect scope creep beyond orchestration.
  Output: `Tasks [N/N compliant] | Existing files [UNCHANGED/N modified] | Scope [CLEAN/CREPT] | VERDICT`

---

## Commit Strategy

- **1**: `feat(scripts): add vps-validate.sh orchestrator — skeleton + smoke mode` - scripts/vps-validate.sh
- **2**: `feat(scripts): add full mode — A4 harness integration` - scripts/vps-validate.sh
- **3**: `feat(scripts): add JSON report generation + scoring` - scripts/vps-validate.sh

---

## Success Criteria

### Verification Commands
```bash
bash -n scripts/vps-validate.sh           # Expected: no output (syntax OK)
shellcheck scripts/vps-validate.sh        # Expected: no errors (warnings OK)
jq . .sisyphus/evidence/vps-validate/report.json  # Expected: valid JSON with overall_score
```

### Final Checklist
- [x] All "Must Have" present
- [x] All "Must NOT Have" absent (no changes to existing scripts)
- [x] JSON report is valid and parseable
- [x] Both modes (smoke, full) route correctly
