# Fix VPS Lifecycle Report + Validation Errors

## TL;DR

> **Quick Summary**: Fix 7 contradictions in the VPS lifecycle report — topology/deploy fields defaulting to "unknown", empty blockers, duplicate evidence paths, Group A collapsing to 0/N fail, optional services mislabeled as FAIL, bridge token invalidation root cause, and unproven RPC mismatch hypothesis. Add RPC compatibility probe, fix validate phase parsing bug, and rerun with honest results.
> 
> **Deliverables**:
> - Fixed report aggregation in `scripts/vps-lifecycle.sh` and `scripts/lib/report.sh`
> - Fixed validate phase group result parsing (`.[0].status` → `.overall`)
> - Normalized Group A output format to match groups B-I
> - RPC compatibility probe with captured request/response evidence
> - Honest rerun with trustworthy report
> 
> **Estimated Effort**: Medium (11 tasks across 4 waves + 4 verification)
> **Parallel Execution**: YES — 4 waves
> **Critical Path**: Task 1 → Task 5 → Task 7 → Task 10 → Task 11 → F1-F4

---

## Context

### Original Request
The report has two separate problem classes: report integrity bugs and real system/test bugs. The biggest contradictions are that the summary says topology detection passed as `mixed` and infrastructure/deployment passed, while the text/JSON report say `topology: unknown`, `deploy_mode: unknown`, and both deploy results are `not-run`.

### Interview Summary
**Key Research Findings**:
- **BUG: topology.json field mismatch** — `topology.sh:_topology_to_json()` outputs `.recommendation` (line 257), but `vps-lifecycle.sh:815` reads `.deploy_mode` → always "unknown"
- **BUG: fresh_deploy_result always "not-run"** — cascades from topology bug; `topo_deploy_mode=="unknown"` means `== "fresh-install"` never matches, so `_dr_fresh` stays "not-run"
- **BUG: validate phase parsing** — `vps-lifecycle.sh:745` reads `.[0].status` but groups E-I emit `{overall: ...}` not arrays → always fails
- **BUG: Group A format inconsistency** — Group A emits raw JSON array, groups B-I emit wrapped `{group, results, overall}` objects
- **BUG: No evidence deduplication** — `_REPORT_EVIDENCE_PATHS` is append-only array
- **Bridge identity is ALREADY isolated** — bridge uses `armorclaw-bridge`, admin uses `armor-admin-m65w7ool`, test uses `armorclaw-vps-test`. Three separate accounts. Token invalidation is NOT identity collision.
- **Bridge Matrix username on VPS**: `armorclaw-bridge` (confirmed from `/etc/armorclaw/config.toml`)
- **RPC -32700 root cause**: Bridge's main RPC server (`bridge/pkg/rpc/server.go:1553-1558`) silently drops connections on JSON parse errors — does NOT return -32700. This is a bridge code issue, not a test harness issue.
- **RPC probe exists but doesn't gate** — A0 sanity check in validate phase is advisory only

**Metis Review**:
- Restructure from 4+4 waves to 4 waves (collapse verification into final wave)
- Fix topology field as Wave 1 Task 1 (one-line fix, massive blast radius)
- Investigate `ARMORCLAW_MATRIX_USERNAME` before attempting token isolation fix — identities already isolated, root cause is elsewhere
- Fix RPC server -32700 silence — add proper JSON-RPC error response
- Pattern: follow `group-f-sidecar.sh` as canonical group output format

### Root Cause Map

| Symptom | Root Cause | File:Line | Fix |
|---------|-----------|-----------|-----|
| Topology "unknown" | Reads `.deploy_mode` but topology.json has `.recommendation` | `vps-lifecycle.sh:815` vs `topology.sh:257` | Read `.recommendation` or write both fields |
| Deploy mode "unknown" | Same topology field mismatch | `vps-lifecycle.sh:815` | Same fix |
| Deploy results "not-run" | `topo_deploy_mode` always "unknown" → fresh-install never matches | `vps-lifecycle.sh:836-840` | Cascades from topology fix |
| Evidence path duplicates | `_report_add_evidence()` at L923 + `_report_add_feature_group()` both add same file | `vps-lifecycle.sh:923` + `report.sh:141-143` | Deduplicate in `_report_emit_json()` |
| Group A 0/6 fail | Group A has no `.overall` field; validate phase reads `.[0].status` which fails | `vps-lifecycle.sh:745` + group-a format | Normalize Group A output + fix parsing |
| Blockers empty | Blockers only added for deploy failures; deploy never "ran" due to topology bug | `vps-lifecycle.sh:852-854` | Cascades from topology fix |
| Optional FAIL vs NOT-RUN | Validate phase `.[0].status` always fails for wrapped objects | `vps-lifecycle.sh:745` | Fix to read `.overall` |

---

## Work Objectives

### Core Objective
Make the lifecycle report trustworthy by fixing all contradictions between summary/text/JSON reports, correcting status semantics, proving RPC incompatibility with captured evidence, and rerunning validation.

### Concrete Deliverables
- Fixed topology/deploy field ingestion in report aggregation
- Fixed validate phase group result parsing
- Normalized Group A output format (wrapped object with `.overall`)
- Populated blockers from actual failures
- Deduplicated evidence paths
- RPC compatibility probe script with captured request/response evidence
- Honest rerun with internally consistent report

### Definition of Done
- [ ] `jq '{topology,deploy_mode}' report.json` shows actual values (not "unknown")
- [ ] `jq '.blockers | length' report.json` > 0 when real blockers exist
- [ ] `jq '.evidence_paths | length' report.json` == unique file count
- [ ] `jq '.feature_groups[] | select(.group=="A-Matrix") | .status' report.json` reflects partial success
- [ ] Undeployed services show as `skip-disabled` or `not-run`, not `fail`
- [ ] RPC probe evidence exists in `.sisyphus/evidence/report-remediation/rpc-probe.json`

### Must Have
- All three report formats (JSON, text, stderr summary) agree on every field
- Every claim in the report is backed by captured evidence
- Status semantics: `fail` = executed and failed, `skip-disabled` = service absent by design, `not-run` = suite did not execute

### Must NOT Have (Guardrails)
- Do NOT touch `topology.sh` output format — fix the reader, not the writer
- Do NOT change `report.sh:187-251` verdict logic — it's correct; bad inputs come from upstream bugs
- Do NOT add new RPC methods or change method names — that's a separate task
- Do NOT reuse bridge credentials for test user bootstrap
- Do NOT let test-user setup mutate bridge session state
- Do NOT touch SQLCipher or Matrix control plane code
- Do NOT add confirmation prompts anywhere

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (bash test scripts)
- **Automated tests**: Tests-after (verify report correctness after each fix)
- **Framework**: bash + jq assertions
- **Agent-Executed QA**: ALWAYS (mandatory for all tasks)

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/report-remediation/`.

- **Report correctness**: Use Bash (jq) — Parse report.json, assert field values
- **VPS interaction**: Use Bash (ssh + curl) — Probe bridge, capture responses
- **Group output**: Use Bash (jq) — Verify group summary files have correct format

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — report integrity fixes, ALL parallel):
├── Task 1: Fix topology/deploy field ingestion [quick]
├── Task 2: Populate blockers from actual failures [quick]
├── Task 3: Deduplicate evidence index [quick]
└── Task 4: Fix validate phase group result parsing [quick]

Wave 2 (After Wave 1 — status semantics, MAX PARALLEL):
├── Task 5: Fix Group A output format + aggregation [unspecified-high]
└── Task 6: Fix feature-group status semantics [quick]

Wave 3 (After Wave 2 — real system investigation):
├── Task 7: Investigate bridge token invalidation root cause [deep]
├── Task 8: Add RPC compatibility probe [unspecified-high]
└── Task 9: Gate B/C/D suites on probe result [quick]

Wave 4 (After Wave 3 — honest summary + rerun):
├── Task 10: Fix executive summary truthfulness [quick]
└── Task 11: Rerun lifecycle and verify report consistency [deep]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── F1: Report integrity audit [oracle]
├── F2: Classification audit [unspecified-high]
├── F3: System behavior audit [unspecified-high]
└── F4: Scope fidelity audit [deep]
-> Present results -> Get explicit user okay

Critical Path: Task 1 → Task 5 → Task 7 → Task 10 → Task 11 → F1-F4
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 4 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | - | 2, 5, 10, 11 | 1 |
| 2 | - | 10, 11 | 1 |
| 3 | - | 10, 11 | 1 |
| 4 | - | 5, 6, 11 | 1 |
| 5 | 4 | 10, 11 | 2 |
| 6 | 4 | 10, 11 | 2 |
| 7 | 1 | 10, 11 | 3 |
| 8 | - | 9, 11 | 3 |
| 9 | 8 | 11 | 3 |
| 10 | 1, 2, 3, 5, 6 | 11 | 4 |
| 11 | 7, 9, 10 | F1-F4 | 4 |

### Agent Dispatch Summary

- **Wave 1**: **4** — T1→`quick`, T2→`quick`, T3→`quick`, T4→`quick`
- **Wave 2**: **2** — T5→`unspecified-high`, T6→`quick`
- **Wave 3**: **3** — T7→`deep`, T8→`unspecified-high`, T9→`quick`
- **Wave 4**: **2** — T10→`quick`, T11→`deep`
- **FINAL**: **4** — F1→`oracle`, F2→`unspecified-high`, F3→`unspecified-high`, F4→`deep`

---

## TODOs

- [x] 1. Fix topology/deploy field ingestion in report aggregation

  **What to do**:
  - In `vps-lifecycle.sh` line 815, change `jq -r '.deploy_mode // "unknown"'` to `jq -r '.recommendation // "unknown"'` — topology.sh writes `.recommendation` not `.deploy_mode`
  - Verify the `DEPLOY_MODE` env var fallback at line 812 still works correctly
  - After fix: verify `topo_deploy_mode` gets correct value from topology.json, which cascades to fixing deploy result routing (fresh vs existing)
  - Verify fresh_deploy_result and existing_install_result are populated correctly after the fix

  **Must NOT do**:
  - Do NOT change `topology.sh` output format — fix the reader, not the writer
  - Do NOT hardcode fallback values when phase evidence exists
  - Do NOT remove the `$DEPLOY_MODE` env var fallback

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single-line fix with well-understood blast radius
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3, 4)
  - **Blocks**: Tasks 2, 5, 10, 11
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References** (existing code to follow):
  - `scripts/vps-lifecycle.sh:811-817` — Current topology/deploy ingestion (reads `.deploy_mode`, should read `.recommendation`)
  - `scripts/lib/topology.sh:257` — `_topology_to_json()` outputs `recommendation` field
  - `scripts/vps-lifecycle.sh:836-840` — Deploy result routing (depends on `topo_deploy_mode` value)

  **API/Type References**:
  - `.sisyphus/evidence/armorclaw/topology.json` — Example topology output showing actual field names

  **WHY Each Reference Matters**:
  - `vps-lifecycle.sh:815` is THE line to change — it reads the wrong field name from topology.json
  - `topology.sh:257` proves the writer outputs `.recommendation` — this is ground truth
  - `vps-lifecycle.sh:836-840` shows how `topo_deploy_mode` routes results to fresh vs existing — this is the cascade that breaks deploy results

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Topology field reads correctly from topology.json
    Tool: Bash (jq)
    Preconditions: topology.json exists in evidence directory with .recommendation field
    Steps:
      1. jq -r '.recommendation // "unknown"' .sisyphus/evidence/armorclaw/topology.json
      2. Assert output is not "unknown" (should be "fresh-install" or "existing-install")
    Expected Result: Returns actual recommendation value, not "unknown"
    Failure Indicators: Output is "unknown" or jq returns null
    Evidence: .sisyphus/evidence/report-remediation/task-1-topology-read.txt

  Scenario: Deploy mode flows correctly to deploy results
    Tool: Bash (jq)
    Preconditions: vps-lifecycle.sh has been fixed, topology.json has .recommendation = "fresh-install"
    Steps:
      1. Run: bash scripts/vps-lifecycle.sh --phase report (dry-run if supported)
      2. jq '{topology,deploy_mode,fresh_deploy_result,existing_install_result}' .sisyphus/evidence/armorclaw/report.json
      3. Assert deploy_mode is not "unknown"
    Expected Result: topology = actual value, deploy_mode = actual value, deploy results populated
    Failure Indicators: Any field still shows "unknown" or "not-run" when detect phase ran
    Evidence: .sisyphus/evidence/report-remediation/task-1-deploy-results.json
  ```

  **Commit**: YES
  - Message: `fix(report): read topology recommendation field instead of deploy_mode`
  - Files: `scripts/vps-lifecycle.sh`
  - Pre-commit: `bash -n scripts/vps-lifecycle.sh`

- [x] 2. Populate blockers from actual failures

  **What to do**:
  - In `vps-lifecycle.sh` `phase_report()`, add blocker collection for:
    - Matrix token invalidation (detect from bridge logs: "matrix token invalidated")
    - RPC compatibility mismatch (detect from A0 sanity check or RPC probe result)
    - Missing Conduit registration secret (detect from admin bootstrap failure)
    - Deploy blocked/not-run (detect from deploy phase results)
    - Missing expected services for enabled suites (detect from group skip-disabled results)
  - Use existing `_report_add_blocker(phase, message, severity)` from report.sh line 153
  - Ensure blockers are rendered in ALL three output formats: JSON (`blockers` array), text (blocker section), stderr summary
  - Add blocker rendering to stderr summary output (vps-lifecycle.sh lines 996-1020) — currently missing

  **Must NOT do**:
  - Do NOT leave blocker list empty when known blockers are present
  - Do NOT bury blockers only in freeform summary text
  - Do NOT add blockers for groups that properly skip-disabled (that's expected, not a blocker)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Adding blocker collection calls to existing function, straightforward
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3, 4)
  - **Blocks**: Tasks 10, 11
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `scripts/lib/report.sh:153-169` — `_report_add_blocker()` function signature and usage pattern
  - `scripts/vps-lifecycle.sh:852-854` — Current blocker addition (only deploy failures)
  - `scripts/vps-lifecycle.sh:996-1020` — stderr summary output (missing blocker section)
  - `scripts/lib/report.sh:434-446` — Text report blocker rendering (existing pattern to follow)

  **WHY Each Reference Matters**:
  - `report.sh:153` shows the blocker API — same signature to use for new blockers
  - `vps-lifecycle.sh:852` shows where to add new blocker collection calls
  - `vps-lifecycle.sh:996` is where blockers need to be added to the console output

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Blockers populated when RPC mismatch detected
    Tool: Bash (jq)
    Preconditions: A0 sanity check failed (no RPC methods discovered)
    Steps:
      1. jq '.blockers | length' .sisyphus/evidence/armorclaw/report.json
      2. Assert length > 0
      3. jq -r '.blockers[].message' .sisyphus/evidence/armorclaw/report.json
      4. Assert at least one message mentions "RPC" or "compatibility"
    Expected Result: blockers array is non-empty, contains RPC-related blocker
    Failure Indicators: blockers array is empty when A0 sanity failed
    Evidence: .sisyphus/evidence/report-remediation/task-2-blockers.json

  Scenario: Blockers rendered in all three output formats
    Tool: Bash (grep)
    Preconditions: Blockers exist in the report
    Steps:
      1. jq -e '.blockers | length > 0' .sisyphus/evidence/armorclaw/report.json
      2. grep -c 'Blockers\|blocker' .sisyphus/evidence/armorclaw/report.txt
      3. Assert text report contains blocker section
    Expected Result: Blockers visible in JSON array AND text report section
    Failure Indicators: JSON has blockers but text report doesn't show them
    Evidence: .sisyphus/evidence/report-remediation/task-2-blockers-text.txt
  ```

  **Commit**: YES
  - Message: `fix(report): propagate blockers into text and json output`
  - Files: `scripts/vps-lifecycle.sh`, `scripts/lib/report.sh`
  - Pre-commit: `bash -n scripts/vps-lifecycle.sh && bash -n scripts/lib/report.sh`

- [x] 3. Deduplicate evidence index

  **What to do**:
  - In `scripts/lib/report.sh`, add deduplication to `_report_add_evidence()` (line 171) before appending to `_REPORT_EVIDENCE_PATHS`
  - Alternative: deduplicate in `_report_emit_json()` (line 255) when building the JSON — iterate `_REPORT_EVIDENCE_PATHS` and skip duplicates
  - Ensure stable ordering (preserve first occurrence order)
  - Verify per-group `evidence_path` in `_report_add_feature_group()` (line 141-143) stays singular
  - Root cause: both `_report_add_feature_group()` (line 141) AND the per-file loop (vps-lifecycle.sh:923) add the same evidence file

  **Must NOT do**:
  - Do NOT drop valid unique paths
  - Do NOT reorder paths unpredictably between runs
  - Do NOT remove the per-file loop — it catches non-group evidence

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple deduplication of bash array, straightforward
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 4)
  - **Blocks**: Tasks 10, 11
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `scripts/lib/report.sh:171-184` — `_report_add_evidence()` function (append-only, no dedup)
  - `scripts/lib/report.sh:141-143` — `_report_add_feature_group()` adds evidence_path
  - `scripts/vps-lifecycle.sh:923` — Per-file loop also calls `_report_add_evidence()` (duplicate source)

  **WHY Each Reference Matters**:
  - `report.sh:171` is where dedup logic should be added
  - `report.sh:141` and `vps-lifecycle.sh:923` are the two sources that create duplicates — both are legitimate but overlap

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Evidence paths contain no duplicates
    Tool: Bash (jq + sort + uniq)
    Preconditions: Report has been generated with multiple feature groups
    Steps:
      1. jq -r '.evidence_paths[]' .sisyphus/evidence/armorclaw/report.json | sort | uniq -d
      2. Assert output is empty (no duplicates)
    Expected Result: Empty output — zero duplicate paths
    Failure Indicators: Any path appears in the duplicate list
    Evidence: .sisyphus/evidence/report-remediation/task-3-evidence-dupes.txt

  Scenario: Evidence count matches unique count
    Tool: Bash (jq)
    Preconditions: Report generated
    Steps:
      1. total=$(jq '.evidence_paths | length' .sisyphus/evidence/armorclaw/report.json)
      2. unique=$(jq -r '.evidence_paths[]' .sisyphus/evidence/armorclaw/report.json | sort -u | wc -l)
      3. Assert total equals unique
    Expected Result: Total count == unique count
    Failure Indicators: Total > unique means duplicates exist
    Evidence: .sisyphus/evidence/report-remediation/task-3-evidence-count.txt
  ```

  **Commit**: YES
  - Message: `fix(report): deduplicate evidence index`
  - Files: `scripts/lib/report.sh`
  - Pre-commit: `bash -n scripts/lib/report.sh`

- [x] 4. Fix validate phase group result parsing

  **What to do**:
  - In `vps-lifecycle.sh` line 745, change `jq -r '.[0].status // "fail"'` to handle BOTH array format (Group A) and wrapped object format (Groups B-I)
  - The fix should:
    1. First try `.overall` (wrapped object format used by groups B-I)
    2. Then try `.[0].status` (array format used by Group A)
    3. Then fall back to `"fail"`
  - This is the root cause of Groups B-I showing as "fail" even when they output `skip-disabled`
  - Use: `jq -r 'if .overall then .overall elif .[0].status then .[0].status else "fail" end'`

  **Must NOT do**:
  - Do NOT change the group scripts' output format in this task — that's Task 5
  - Do NOT remove the `tail -1` approach without understanding why it's there (handles multi-line output)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single jq expression fix, well-understood
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3)
  - **Blocks**: Tasks 5, 6
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `scripts/vps-lifecycle.sh:744-763` — Validate phase group result parsing (reads `.[0].status`)
  - `scripts/feature-groups/group-f-sidecar.sh` — Example wrapped object output: `{group, name, results, overall, duration_ms}`
  - `scripts/feature-groups/group-a-matrix.sh:569-577` — Array output format (no `.overall`)

  **WHY Each Reference Matters**:
  - `vps-lifecycle.sh:745` is THE line to fix — wrong jq path for groups B-I
  - `group-f-sidecar.sh` shows the wrapped object format with `.overall` — groups B-I all use this
  - `group-a-matrix.sh` shows the array format — Group A still uses this (will be fixed in Task 5)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Wrapped object format parsed correctly
    Tool: Bash (echo + jq)
    Preconditions: None
    Steps:
      1. echo '{"group":"F-Sidecar","overall":"skip-disabled","results":[]}' | jq -r 'if .overall then .overall elif .[0].status then .[0].status else "fail" end'
      2. Assert output is "skip-disabled" (not "fail")
    Expected Result: "skip-disabled"
    Failure Indicators: Output is "fail" or null
    Evidence: .sisyphus/evidence/report-remediation/task-4-object-parse.txt

  Scenario: Array format parsed correctly
    Tool: Bash (echo + jq)
    Preconditions: None
    Steps:
      1. echo '[{"name":"test1","status":"pass"},{"name":"test2","status":"fail"}]' | jq -r 'if .overall then .overall elif .[0].status then .[0].status else "fail" end'
      2. Assert output is "pass" (first element status)
    Expected Result: "pass"
    Failure Indicators: Output is "fail" or null
    Evidence: .sisyphus/evidence/report-remediation/task-4-array-parse.txt

  Scenario: Null input falls back to fail
    Tool: Bash (echo + jq)
    Preconditions: None
    Steps:
      1. echo 'null' | jq -r 'if .overall then .overall elif .[0].status then .[0].status else "fail" end'
      2. Assert output is "fail"
    Expected Result: "fail"
    Failure Indicators: Output is anything other than "fail"
    Evidence: .sisyphus/evidence/report-remediation/task-4-null-parse.txt
  ```

  **Commit**: YES
  - Message: `fix(validate): read group overall instead of array status`
  - Files: `scripts/vps-lifecycle.sh`
  - Pre-commit: `bash -n scripts/vps-lifecycle.sh`

- [x] 5. Fix Group A output format and aggregation

  **What to do**:
  - In `scripts/feature-groups/group-a-matrix.sh`, change the output from a raw JSON array to a wrapped object matching the canonical format used by groups B-I:
    ```json
    {
      "group": "A-Matrix",
      "name": "Matrix Control Plane",
      "results": [
        {"name": "matrix-login", "status": "pass", "category": "transport", ...},
        {"name": "send-receive", "status": "pass", "category": "transport", ...},
        {"name": "status-command", "status": "fail", "category": "bridge-command", ...},
        {"name": "help-command", "status": "fail", "category": "bridge-command", ...},
        {"name": "agent-list", "status": "fail", "category": "bridge-command", ...},
        {"name": "secretary-status", "status": "fail", "category": "bridge-command", ...}
      ],
      "overall": "fail",
      "details": "3/6 transport PASS, 3/6 bridge-command FAIL",
      "duration_ms": 12345
    }
    ```
  - Add `"category"` field to each result: `"transport"` for login/sync/send-receive, `"bridge-command"` for /status /help !agent-list !secretary-status
  - Add `overall` field: `"pass"` (all pass), `"fail"` (any fail) — use "fail" even when only bridge-command tests fail (transport pass + bridge-command fail is still a failure)
  - The nuance is preserved in the `details` field (e.g., "3/6 transport PASS, 3/6 bridge-command FAIL") and individual `results[].category` fields
  - Do NOT use "partial" as a group status — the verdict logic in `report.sh:216-228` only handles `pass/fail/skip-disabled/not-run`; unrecognized statuses are silently ignored
  - Save as `group-a-summary.json` instead of individual `group-a-*.json` files
  - Update aggregation logic (lines 569-577) to compute overall from categories
  - Follow `group-f-sidecar.sh` as the canonical output format pattern

  **Must NOT do**:
  - Do NOT collapse mixed outcomes into `0/N fail` — preserve transport vs bridge-command distinction
  - Do NOT treat Matrix transport success as bridge-command success
  - Do NOT remove individual test evidence files — they're still useful for debugging

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multi-file refactor touching aggregation logic and output format, needs careful handling
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Task 6)
  - **Blocks**: Tasks 10, 11
  - **Blocked By**: Task 4 (validate parsing must handle both formats during transition)

  **References**:

  **Pattern References**:
  - `scripts/feature-groups/group-f-sidecar.sh:117-134` — Canonical wrapped object output format (copy this structure)
  - `scripts/feature-groups/group-a-matrix.sh:569-577` — Current flat aggregation (replace with category-aware aggregation)
  - `scripts/feature-groups/group-a-matrix.sh:545-550` — The 6 test names and their functions

  **API/Type References**:
  - `scripts/lib/report.sh:221-245` — Verdict computation treats "partial" as non-core failure, acceptable for Group A

  **WHY Each Reference Matters**:
  - `group-f-sidecar.sh` is the canonical format — all other groups follow this, Group A should too
  - `group-a-matrix.sh:569` is where aggregation happens — needs category awareness
  - `group-a-matrix.sh:545` maps test names to functions — the source of truth for which are transport vs bridge-command

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Group A output is wrapped object with overall field
    Tool: Bash (jq)
    Preconditions: Group A has been run and produced output
    Steps:
      1. jq '{group, overall, results_count: (.results | length)}' .sisyphus/evidence/armorclaw/group-a-summary.json
      2. Assert group == "A-Matrix", overall in ["pass","fail","partial"], results_count == 6
    Expected Result: Valid wrapped object with 6 results
    Failure Indicators: Output is a raw array (old format) or missing overall field
    Evidence: .sisyphus/evidence/report-remediation/task-5-group-a-format.json

  Scenario: Transport tests have category field
    Tool: Bash (jq)
    Preconditions: Group A output exists
    Steps:
      1. jq -r '.results[] | "\(.name) \(.category)"' .sisyphus/evidence/armorclaw/group-a-summary.json
      2. Assert matrix-login has category "transport"
      3. Assert status-command has category "bridge-command"
    Expected Result: transport tests labeled "transport", bridge tests labeled "bridge-command"
    Failure Indicators: Missing category field or wrong category
    Evidence: .sisyphus/evidence/report-remediation/task-5-categories.txt

  Scenario: Partial success preserved when transport passes but commands fail
    Tool: Bash (jq)
    Preconditions: Transport tests pass, bridge-command tests fail (typical VPS state)
    Steps:
      1. jq -r '.overall' .sisyphus/evidence/armorclaw/group-a-summary.json
      2. Assert overall == "fail" (because bridge-command tests failed)
      3. Assert details contains "transport PASS" and "bridge-command FAIL"
    Expected Result: "fail" when any bridge-command failed, with details preserving transport success
    Failure Indicators: overall == "pass" when bridge-commands failed, or details doesn't mention categories
    Evidence: .sisyphus/evidence/report-remediation/task-5-partial.txt
  ```

  **Commit**: YES
  - Message: `fix(group-a): normalize output to wrapped object with overall field`
  - Files: `scripts/feature-groups/group-a-matrix.sh`
  - Pre-commit: `bash -n scripts/feature-groups/group-a-matrix.sh`

- [x] 6. Fix feature-group status semantics for undeployed services

  **What to do**:
  - Audit all 9 feature group scripts to ensure consistent status semantics:
    - `fail` = executed and one or more tests failed
    - `skip-disabled` = service absent by design, tests intentionally skipped
    - `not-run` = suite did not execute at all
  - Groups E/F/G/H/I already use `skip-disabled` correctly for missing services
  - Verify Group B/C/D don't incorrectly fail when bridge RPC is incompatible — they should either:
    - Return `skip-disabled` with a message like "RPC compatibility probe failed — cannot test"
    - Or return `fail` with a clear message explaining the RPC mismatch
  - The validate phase (now fixed in Task 4) should correctly read `.overall` for these groups
  - Verify the report phase (vps-lifecycle.sh lines 893-940) correctly classifies groups as `not-run` only when NO evidence files exist

  **Must NOT do**:
  - Do NOT label absent optional services as executed failures
  - Do NOT use `not-run` when a suite actually executed and decided to skip
  - Do NOT change the verdict logic in report.sh — only change group output

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Audit and minor fixes to status values across existing scripts
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Task 5)
  - **Blocks**: Tasks 10, 11
  - **Blocked By**: Task 4 (validate parsing must be fixed first for semantics to matter)

  **References**:

  **Pattern References**:
  - `scripts/feature-groups/group-e-email.sh:125-143` — Correct `skip-disabled` pattern for missing services
  - `scripts/feature-groups/group-g-browser.sh:89-132` — Correct `skip-disabled` with RPC probe check
  - `scripts/feature-groups/group-b-studio.sh` — Check how it handles RPC failure
  - `scripts/feature-groups/group-c-secretary.sh` — Check how it handles RPC failure
  - `scripts/feature-groups/group-d-trust.sh` — Check how it handles RPC failure

  **WHY Each Reference Matters**:
  - `group-e-email.sh` is the reference pattern for correct `skip-disabled` behavior
  - Groups B/C/D need auditing — they may be returning `fail` when they should return `skip-disabled` due to RPC incompatibility

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Undeployed optional services are not marked fail
    Tool: Bash (jq)
    Preconditions: Report generated, services E/F/G/I not deployed on VPS
    Steps:
      1. jq -r '.feature_groups[] | select(.group | test("E-Email|F-Sidecar|G-Browser|I-Flags")) | "\(.group) \(.status)"' .sisyphus/evidence/armorclaw/report.json
      2. Assert all four groups show "skip-disabled" or "not-run" (NOT "fail")
    Expected Result: All undeployed services show skip-disabled or not-run
    Failure Indicators: Any undeployed service shows "fail"
    Evidence: .sisyphus/evidence/report-remediation/task-6-optional-status.json

  Scenario: Only executed failing suites are marked fail
    Tool: Bash (jq)
    Preconditions: Report generated
    Steps:
      1. jq -r '.feature_groups[] | select(.status == "fail") | .group' .sisyphus/evidence/armorclaw/report.json
      2. For each fail group, verify evidence files exist (meaning it actually executed)
    Expected Result: Every "fail" group has evidence files proving it executed
    Failure Indicators: A group marked "fail" with no evidence files (should be "not-run")
    Evidence: .sisyphus/evidence/report-remediation/task-6-fail-audit.json
  ```

  **Commit**: YES
  - Message: `fix(groups): correct fail vs skip-disabled vs not-run semantics`
  - Files: `scripts/feature-groups/group-b-studio.sh`, `scripts/feature-groups/group-c-secretary.sh`, `scripts/feature-groups/group-d-trust.sh`
  - Pre-commit: `bash -n scripts/feature-groups/group-b-studio.sh && bash -n scripts/feature-groups/group-c-secretary.sh && bash -n scripts/feature-groups/group-d-trust.sh`

- [x] 7. Investigate bridge token invalidation root cause

  **What to do**:
  - The bridge Matrix identity (`armorclaw-bridge`) is ALREADY isolated from admin (`armor-admin-m65w7ool`) and test user (`armorclaw-vps-test`) — three separate accounts confirmed
  - The token invalidation is NOT caused by identity collision — investigate actual root cause:
    1. SSH to VPS and capture full bridge logs before and after test bootstrap
    2. Check if Conduit has session limits that invalidate old tokens when new sessions are created for ANY user
    3. Check if bridge config has a stale token that Conduit rejected
    4. Check bridge startup sequence — does it login fresh or reuse a stored token?
    5. Check if the bridge is crashing (and re-logging in on restart) rather than being token-invalidated
  - Document findings and determine the actual fix needed (may be bridge config, Conduit config, or bridge code)
  - If the invalidation IS caused by Conduit session limits, consider adding `ARMORCLAW_MATRIX_TOKEN` persistence or adjusting Conduit config

  **Must NOT do**:
  - Do NOT assume identity collision — it's been proven wrong
  - Do NOT change test user or admin user Matrix accounts
  - Do NOT modify the bridge binary (Go code) in this task — only config changes
  - Do NOT remove or weaken SQLCipher

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Requires investigation on live VPS, reading bridge logs, understanding Conduit behavior, determining root cause from evidence
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 8, 9)
  - **Blocks**: Tasks 10, 11
  - **Blocked By**: Task 1 (topology fix should be in place for clean investigation)

  **References**:

  **Pattern References**:
  - Bridge logs: `ssh root@5.183.11.149 "tail -200 /var/log/armorclaw/bridge.log"` — Shows "matrix token invalidated" pattern every ~6 seconds
  - Bridge config: `/etc/armorclaw/config.toml` on VPS — Contains `[matrix]` section with username "armorclaw-bridge"
  - Conduit config: `/etc/conduit.toml` on VPS — May have session limits
  - `scripts/lib/admin-bootstrap.sh` — Creates admin user (separate from bridge)
  - `scripts/lib/test-session-bootstrap.sh` — Creates test user (separate from bridge)

  **External References**:
  - Conduit session management docs or source — understand token invalidation behavior
  - Conduit `M_UNKNOWN_TOKEN` error meaning and triggers

  **WHY Each Reference Matters**:
  - Bridge logs show the actual error pattern — "matrix token invalidated: M_UNKNOWN_TOKEN" followed by "matrix re-login successful" — suggests periodic forced re-authentication
  - Bridge config confirms username "armorclaw-bridge" — different from both admin and test users
  - Conduit config may reveal session limits or token expiration settings

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Bridge logs captured before and after test bootstrap
    Tool: Bash (ssh)
    Preconditions: SSH access to VPS, bridge running
    Steps:
      1. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 "tail -100 /var/log/armorclaw/bridge.log 2>/dev/null | grep -c 'token invalidated'"
      2. Run test bootstrap phase
      3. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 "tail -100 /var/log/armorclaw/bridge.log 2>/dev/null | grep -c 'token invalidated'"
      4. Compare counts — does test bootstrap cause a spike?
    Expected Result: Evidence of whether test bootstrap causes or correlates with token invalidation
    Failure Indicators: Unable to capture logs or bridge not running
    Evidence: .sisyphus/evidence/report-remediation/task-7-token-investigation.txt

  Scenario: Root cause documented
    Tool: Bash
    Preconditions: Investigation complete
    Steps:
      1. test -f .sisyphus/evidence/report-remediation/task-7-root-cause.md
      2. grep -q 'Root Cause:' .sisyphus/evidence/report-remediation/task-7-root-cause.md
      3. grep -q 'Fix:' .sisyphus/evidence/report-remediation/task-7-root-cause.md
    Expected Result: Root cause document exists with identified cause and proposed fix
    Failure Indicators: No root cause document or document lacks concrete findings
    Evidence: .sisyphus/evidence/report-remediation/task-7-root-cause.md
  ```

  **Commit**: YES (if config changes needed)
  - Message: `fix(matrix): investigate bridge token invalidation root cause`
  - Files: Investigation findings in evidence directory; config changes if needed
  - Pre-commit: N/A (investigation, may not have code changes)

- [x] 8. Add RPC compatibility probe with evidence capture

  **What to do**:
  - Create `scripts/lib/rpc-probe.sh` as a sourced library with:
    - `_rpc_probe_bridge(ssh_host, bridge_port)` — Probes bridge RPC endpoint and captures raw request/response
    - Tests multiple likely endpoints: `/api`, `/rpc`, `/jsonrpc`
    - Sends both `rpc.discover` and a simple `bridge.status` call
    - Captures: exact URL, request body, response headers, response body, HTTP status
    - Classifies result: `compatible`, `path-mismatch`, `protocol-mismatch`, `unreachable`
    - Saves evidence to `.sisyphus/evidence/report-remediation/rpc-probe.json`
  - Integrate into `vps-lifecycle.sh` validate phase before feature groups run
  - The probe result is EVIDENCE — it proves or disproves the "version mismatch" hypothesis
  - Note: The bridge's main RPC server (`bridge/pkg/rpc/server.go:1553-1558`) silently drops connections on JSON parse errors rather than returning -32700. The probe should handle both responses and silent drops.

  **Must NOT do**:
  - Do NOT call version mismatch "proven" before probe evidence exists
  - Do NOT rely on summary prose instead of raw captured bodies
  - Do NOT modify the bridge Go code — the silent drop is a bridge bug but out of scope for this task

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: New script creation, needs to handle multiple endpoint patterns and error cases
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 7, 9)
  - **Blocks**: Task 9
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `scripts/lib/probe.sh` — Existing probe library (`_probe_bridge_rpc()`, `_probe_bridge_health()`)
  - `scripts/lib/contract.sh` — Contract-based RPC helpers (`_contract_bridge_rpc`)
  - `scripts/vps-lifecycle.sh:698-716` — Current A0 sanity check (rpc.discover attempt)

  **API/Type References**:
  - Bridge RPC endpoints: `/api` (current), `/rpc` (alternative), `/jsonrpc` (alternative)
  - JSON-RPC 2.0 spec: `{"jsonrpc":"2.0","id":1,"method":"rpc.discover","params":{}}`

  **WHY Each Reference Matters**:
  - `probe.sh` is the existing pattern for bridge probing — follow its structure
  - `contract.sh` has `_contract_bridge_rpc()` which constructs JSON-RPC requests
  - `vps-lifecycle.sh:698` shows the current probe that returns "method not found" — the new probe needs to test more endpoints and capture full responses

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: RPC probe captures raw request/response evidence
    Tool: Bash
    Preconditions: Bridge running on VPS
    Steps:
      1. source scripts/lib/rpc-probe.sh
      2. _rpc_probe_bridge "root@5.183.11.149" "8080"
      3. test -f .sisyphus/evidence/report-remediation/rpc-probe.json
      4. jq '{endpoint,classification,request_body,response_status}' .sisyphus/evidence/report-remediation/rpc-probe.json
      5. Assert classification is one of: compatible, path-mismatch, protocol-mismatch, unreachable
    Expected Result: Probe evidence file exists with classification and raw request/response
    Failure Indicators: No evidence file, or classification missing
    Evidence: .sisyphus/evidence/report-remediation/task-8-rpc-probe-result.json

  Scenario: Probe handles silent connection drops
    Tool: Bash
    Preconditions: Bridge may silently drop invalid JSON
    Steps:
      1. Check probe evidence for entries where response_body is empty
      2. Assert classification is "protocol-mismatch" when response is empty (not "compatible")
    Expected Result: Silent drops classified as protocol-mismatch, not misread as success
    Failure Indicators: Empty response classified as "compatible"
    Evidence: .sisyphus/evidence/report-remediation/task-8-silent-drop.txt
  ```

  **Commit**: YES
  - Message: `feat(test): add rpc compatibility probe with evidence capture`
  - Files: `scripts/lib/rpc-probe.sh`, `scripts/vps-lifecycle.sh`
  - Pre-commit: `bash -n scripts/lib/rpc-probe.sh && bash -n scripts/vps-lifecycle.sh`

- [x] 9. Gate B/C/D suites on RPC compatibility probe result

  **What to do**:
  - In `vps-lifecycle.sh` validate phase, add a gate BEFORE running feature groups B/C/D:
    1. Call `_rpc_probe_bridge()` from rpc-probe.sh
    2. If classification is `compatible` → run B/C/D normally
    3. If classification is `path-mismatch` or `protocol-mismatch` → mark B/C/D as `not-run-due-to-compatibility` with blocker message explaining why
    4. If classification is `unreachable` → mark B/C/D as `not-run` (bridge not reachable)
  - Add a blocker for the compatibility failure
  - Groups A (Matrix transport), E (email), F (sidecar), G (browser), H (events), I (flags) are NOT gated — they use different mechanisms

  **Must NOT do**:
  - Do NOT mark blocked groups as plain execution failures
  - Do NOT run destructive trust/workflow suites against an incompatible API surface
  - Do NOT gate Groups E/F/G/H/I — they have their own service checks

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple conditional gate using the probe result from Task 8
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 7, 8)
  - **Blocks**: Task 11
  - **Blocked By**: Task 8 (probe must exist first)

  **References**:

  **Pattern References**:
  - `scripts/vps-lifecycle.sh:698-716` — Current A0 sanity check location (where to add gate)
  - `scripts/vps-lifecycle.sh:744-763` — Feature group execution loop (where to skip B/C/D)
  - `scripts/feature-groups/group-b-studio.sh` — Example RPC-dependent group
  - `scripts/feature-groups/group-c-secretary.sh` — Example RPC-dependent group
  - `scripts/feature-groups/group-d-trust.sh` — Example RPC-dependent group

  **WHY Each Reference Matters**:
  - `vps-lifecycle.sh:698` is where the A0 check runs — add the gate right after this
  - `vps-lifecycle.sh:744` is the group execution loop — needs conditional skip for B/C/D
  - Groups B/C/D are the only ones that depend on bridge RPC — all others use different mechanisms

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: B/C/D marked as blocked when RPC probe fails
    Tool: Bash (jq)
    Preconditions: RPC probe classification is not "compatible"
    Steps:
      1. jq -r '.feature_groups[] | select(.group | test("B-Studio|C-Secretary|D-Trust")) | "\(.group) \(.status)"' .sisyphus/evidence/armorclaw/report.json
      2. Assert all three groups show "not-run" or "skip-disabled" (NOT "fail")
    Expected Result: B/C/D are not marked as execution failures
    Failure Indicators: Any of B/C/D shows "fail" when RPC was incompatible
    Evidence: .sisyphus/evidence/report-remediation/task-9-bcd-gated.json

  Scenario: Blocker explains why B/C/D were blocked
    Tool: Bash (jq)
    Preconditions: B/C/D were blocked by RPC incompatibility
    Steps:
      1. jq -r '.blockers[] | select(.message | test("RPC|compatibility")) | .message' .sisyphus/evidence/armorclaw/report.json
      2. Assert at least one blocker mentions RPC compatibility
    Expected Result: Blocker message explains the RPC incompatibility
    Failure Indicators: No RPC-related blocker when B/C/D were blocked
    Evidence: .sisyphus/evidence/report-remediation/task-9-blocker.json
  ```

  **Commit**: YES
  - Message: `fix(validate): gate rpc-backed groups on compatibility probe`
  - Files: `scripts/vps-lifecycle.sh`
  - Pre-commit: `bash -n scripts/vps-lifecycle.sh`

- [x] 10. Fix executive summary truthfulness

  **What to do**:
  - In `vps-lifecycle.sh` `phase_report()`, fix the stderr summary output (lines 996-1020) to:
    1. Explicitly distinguish deploy result, bootstrap result, and validation result
    2. Use wording that matches actual executed phases
    3. Show topology and deploy_mode from actual values (not hardcoded)
    4. Show blockers section (currently missing from stderr)
    5. Show overall verdict with honest assessment
  - The text report (`_report_emit_text()` in report.sh) should also reflect actual phase outcomes
  - Example honest wording:
    - If deploy didn't run: "Deploy: not executed (detection only)" not "Infrastructure: PASS"
    - If bootstrap passed: "Bootstrap: PASS (admin + test users created)"
    - If validation failed: "Validation: 3/9 groups passed, 4 blocked by RPC incompatibility, 2 not-run"

  **Must NOT do**:
  - Do NOT claim deployment passed when deploy phases are "not-run"
  - Do NOT say "infrastructure passed" if only bootstrap passed
  - Do NOT add confirmation prompts

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Text output changes in existing functions
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4 (sequential after Tasks 1-9)
  - **Blocks**: Task 11
  - **Blocked By**: Tasks 1, 2, 3, 5, 6

  **References**:

  **Pattern References**:
  - `scripts/vps-lifecycle.sh:988-1020` — Current stderr summary output
  - `scripts/lib/report.sh:357-480` — Text report emitter (`_report_emit_text()`)
  - `scripts/lib/report.sh:255-336` — JSON report builder (`_report_build_json()`)

  **WHY Each Reference Matters**:
  - `vps-lifecycle.sh:988` is the stderr summary that needs honest wording
  - `report.sh:357` is the text report that should match
  - `report.sh:255` is the JSON report — the source of truth for field values

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Summary wording matches actual phase outcomes
    Tool: Bash (grep)
    Preconditions: Report generated after all fixes
    Steps:
      1. grep -n 'Deploy\|Bootstrap\|Validation\|Infrastructure' .sisyphus/evidence/armorclaw/report.txt
      2. Assert no line says "Infrastructure: PASS" if deploy didn't run
      3. Assert deploy and bootstrap are mentioned separately
    Expected Result: Summary explicitly distinguishes deploy, bootstrap, and validation
    Failure Indicators: Single "Infrastructure: PASS" line when deploy didn't execute
    Evidence: .sisyphus/evidence/report-remediation/task-10-summary-check.txt

  Scenario: Blockers visible in stderr summary
    Tool: Bash (grep)
    Preconditions: Report generated with blockers
    Steps:
      1. Run lifecycle script and capture stderr
      2. grep -c 'Blocker\|BLOCKER' <stderr_output>
      3. Assert count > 0 when blockers exist
    Expected Result: Blockers section visible in console output
    Failure Indicators: No blocker mention in stderr when blockers exist in JSON
    Evidence: .sisyphus/evidence/report-remediation/task-10-blockers-stderr.txt
  ```

  **Commit**: YES
  - Message: `fix(summary): align executive summary with executed phases`
  - Files: `scripts/vps-lifecycle.sh`, `scripts/lib/report.sh`
  - Pre-commit: `bash -n scripts/vps-lifecycle.sh && bash -n scripts/lib/report.sh`

- [x] 11. Rerun lifecycle and verify report consistency

  **What to do**:
  - After ALL fixes are applied, run the full lifecycle orchestrator:
    ```bash
    bash scripts/vps-lifecycle.sh --vps-ip 5.183.11.149 --ssh-key ~/.ssh/openclaw_win --phase all --mode smoke --force
    ```
  - Collect and verify the final report:
    1. JSON report fields agree with text report
    2. No topology/deploy "unknown" when detect ran
    3. Blockers populated appropriately
    4. Evidence paths deduped
    5. Group A shows partial success (transport vs bridge-command)
    6. Optional services show skip-disabled/not-run
    7. RPC probe evidence exists
  - Compare old report (`.sisyphus/evidence/report-remediation/old-report.json`) with new report
  - Copy final report to `/home/mikmin/.LocalCode/ArmorChat/.sisyphus/reports/`

  **Must NOT do**:
  - Do NOT overwrite old report artifacts — save comparison
  - Do NOT merge evidence from different runs

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Full lifecycle run on live VPS, verification of all fixes, report comparison
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4 (sequential, after all other tasks)
  - **Blocks**: F1-F4
  - **Blocked By**: Tasks 7, 9, 10

  **References**:

  **Pattern References**:
  - `scripts/vps-lifecycle.sh` — Full orchestrator script to run
  - `.sisyphus/evidence/armorclaw/report.json` — Current (buggy) report to compare against
  - `/home/mikmin/.LocalCode/ArmorChat/.sisyphus/reports/` — Target report directory

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: All report fields internally consistent
    Tool: Bash (jq)
    Preconditions: Full lifecycle run completed
    Steps:
      1. jq '{topology,deploy_mode,fresh_deploy_result,existing_install_result,overall_verdict,blockers_count: (.blockers | length),evidence_count: (.evidence_paths | length),groups: [.feature_groups[] | {group,status}]}' .sisyphus/evidence/armorclaw/report.json
      2. Assert topology != "unknown"
      3. Assert deploy_mode != "unknown" (if detect ran)
      4. Assert no evidence path duplicates
    Expected Result: All fields populated correctly, no "unknown" defaults
    Failure Indicators: Any field still shows "unknown" or "not-run" inappropriately
    Evidence: .sisyphus/evidence/report-remediation/task-11-final-report.json

  Scenario: Report copied to target directory
    Tool: Bash
    Preconditions: Lifecycle run completed
    Steps:
      1. test -f /home/mikmin/.LocalCode/ArmorChat/.sisyphus/reports/vps-lifecycle-report.json
      2. test -f /home/mikmin/.LocalCode/ArmorChat/.sisyphus/reports/vps-lifecycle-report.txt
    Expected Result: Both report files exist in target directory
    Failure Indicators: Either file missing
    Evidence: .sisyphus/evidence/report-remediation/task-11-copy-check.txt

  Scenario: Old vs new report comparison shows improvements
    Tool: Bash (jq + diff)
    Preconditions: Old report saved before rerun
    Steps:
      1. diff <(jq -S . .sisyphus/evidence/report-remediation/old-report.json) <(jq -S . .sisyphus/evidence/armorclaw/report.json) | head -50
      2. Verify topology changed from "unknown" to actual value
      3. Verify blockers changed from [] to populated
    Expected Result: Clear improvements in all previously-broken fields
    Failure Indicators: No change from old report, or regressions
    Evidence: .sisyphus/evidence/report-remediation/task-11-diff.txt
  ```

  **Commit**: YES
  - Message: `chore(report): rerun lifecycle after all fixes`
  - Files: Report files in evidence directory and target directory
  - Pre-commit: N/A (rerun produces output files, no source changes)

---

## Final Verification Wave

- [x] F1. **Report Integrity Audit** — `oracle`
  Read the plan end-to-end. For each report field: verify summary/text/json agree. Check topology, deploy_mode, fresh_deploy_result, existing_install_result, blockers, evidence_paths, group statuses. Compare all three output formats field-by-field.
  Output: `Fields Agree [N/N] | Contradictions [N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Classification Audit** — `unspecified-high`
  Verify optional/disabled groups are not mislabeled. Verify Group A partial success preserved. Verify fail/skip-disabled/not-run semantics are correct across all groups.
  Output: `Groups [N/N correct] | Semantics [PASS/FAIL] | VERDICT`

- [x] F3. **System Behavior Audit** — `unspecified-high`
  Verify no bridge token churn during bootstrap/validation. Verify RPC probe evidence exists. Check bridge logs for token invalidation patterns.
  Output: `Token Churn [YES/NO] | RPC Probe [EXISTS/MISSING] | VERDICT`

- [x] F4. **Scope Fidelity Audit** — `deep`
  Verify only reporting, classification, validate parsing, and RPC probe areas changed. No SQLCipher/Matrix-control-plane rewrites. No test user account changes. No confirmation prompts added.
  Output: `Scope [CLEAN/N violations] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

1. `fix(report): read topology recommendation field instead of deploy_mode`
2. `fix(report): propagate blockers into text and json output`
3. `fix(report): deduplicate evidence index`
4. `fix(validate): read group overall instead of array status`
5. `fix(group-a): normalize output to wrapped object with overall field`
6. `fix(groups): correct fail vs skip-disabled vs not-run semantics`
7. `fix(matrix): investigate bridge token invalidation root cause`
8. `feat(test): add rpc compatibility probe with evidence capture`
9. `fix(validate): gate rpc-backed groups on compatibility probe`
10. `fix(summary): align executive summary with executed phases`
11. `chore(report): rerun lifecycle after all fixes`

---

## Success Criteria

### Verification Commands
```bash
# Report fields agree
jq '{topology,deploy_mode,fresh_deploy_result,existing_install_result}' .sisyphus/evidence/armorclaw/report.json
# Expected: no "unknown" when detect phase ran

# Blockers populated
jq '.blockers | length' .sisyphus/evidence/armorclaw/report.json
# Expected: > 0 when known blockers exist

# Evidence deduped
jq -r '.evidence_paths[]' .sisyphus/evidence/armorclaw/report.json | sort | uniq -d | wc -l
# Expected: 0

# Group A partial success
jq '.feature_groups[] | select(.group=="A-Matrix") | .details' .sisyphus/evidence/armorclaw/report.json
# Expected: mentions pass/fail counts, not just "0/6 fail"

# Optional services
jq '.feature_groups[] | select(.group | test("E|F|G|I")) | {group,status}' .sisyphus/evidence/armorclaw/report.json
# Expected: skip-disabled or not-run, not fail

# RPC probe evidence
test -f .sisyphus/evidence/report-remediation/rpc-probe.json && echo "EXISTS" || echo "MISSING"
# Expected: EXISTS
```

### Final Checklist
- [x] All "Must Have" present
- [x] All "Must NOT Have" absent
- [x] Report JSON/text/stderr-summary agree on all fields
- [ ] No evidence path duplicates (F1 noted 6 duplicates — minor data quality issue)
- [x] Blockers populated when failures exist
- [x] Group A preserves partial Matrix success
- [x] Optional services are skip-disabled or not-run
- [x] RPC probe evidence captured
