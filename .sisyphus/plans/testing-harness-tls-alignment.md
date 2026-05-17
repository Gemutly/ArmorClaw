# Testing & Harness TLS Alignment

## TL;DR

> ✅ **Post-Execution Defect RESOLVED**: T1 and T2 were executed and verified. T3 was partially executed (3a–3c applied) but T3d (adjacent stale-reference scan) was not performed. A stale `full-system-{task-name}` evidence path was discovered at line 3815 of `doc/armorclaw.md` in the "Test Results Location" section. **T3d was rerun**: line 3815 fixed to dual-root format, T3-QA scenario 5 passed (zero stale refs), F1 rerun with corrected backtick-wrapped table-row patterns — 23/23 checks PASS. Plan is now fully complete.

> **Quick Summary**: Fix 4 documentation/harness inconsistencies identified in the TLS review: wire TLS test suites into the Plan A harness SUITE_MAP, correct scenario counts in testing.md, add evidence-root boundary note, and tighten F6 fingerprint verification. Sync armorclaw.md to prevent drift.
> 
> **Deliverables**:
> - `scripts/a4_harness.sh` — 2 new SUITE_MAP entries (`tls-mode`, `tls-restart`)
> - `doc/testing.md` — v1.1 with corrected counts, evidence boundary, tightened F6, TLS health values table, native zero-value shell example
> - `doc/armorclaw.md` — Synced harness loop with TLS scripts + updated Tier A comment + harness SUITE_MAP note + aligned evidence path
> 
> **Estimated Effort**: Quick
> **Parallel Execution**: YES - 2 waves
> **Critical Path**: (T1 + T2 parallel) → T3 → F1

---

## Context

### Original Request
Apply a review document's fixes as a "paired documentation + harness alignment change" — not a doc-only edit. The review found that TLS test suites exist but aren't wired into the Plan A harness abstraction, scenario counts are wrong, F6 is vague, and armorclaw.md has drifted from testing.md.

### Interview Summary
**Key Discussions**:
- The review provided exact wording for most changes — minimal ambiguity
- The harness gap (missing SUITE_MAP entries) is an execution gap, not just wording drift
- armorclaw.md must be updated simultaneously to prevent doc divergence
- Both doc files are in `.gitignore` — local-only, not committed

**Research Findings**:
- `a4_harness.sh` SUITE_MAP: 17 entries, zero TLS entries — gap confirmed
- `test-tls-restart-safety.sh`: 3 core preservation assertions (fingerprint, mode, QR state) + restart + checkpoint — review says "3 scenarios"
- `test-tls-mode-integration.sh`: exactly 10 scenarios (S1-S10, S10 conditional)
- armorclaw.md harness loop in "Running the Full Harness" section: missing TLS scripts
- armorclaw.md evidence path (`.sisyphus/evidence/full-system-{task-name}/`) differs from testing.md (`.sisyphus/evidence/armorclaw/`)

### Self-Review (Metis substitute — 50-descendant limit)
**Identified Gaps** (addressed):
- Evidence path divergence between docs → resolved by T3 (sync armorclaw.md)
- Harness TLS entries must use correct test filenames → verified from filesystem
- F6 wording must specify exact 3 fingerprint sources → review provided exact wording
- Version bump from 1.0.0 to 1.1.0 → included in T2

---

## Work Objectives

### Core Objective
Align TLS testing documentation and harness wiring so that: (1) TLS suites are runnable through `a4_harness.sh`, (2) testing.md accurately reflects scenario counts and evidence boundaries, and (3) armorclaw.md doesn't drift from testing.md.

### Concrete Deliverables
- `scripts/a4_harness.sh` with `tls-mode` and `tls-restart` in SUITE_MAP
- `doc/testing.md` v1.1 with corrected scenario counts, Evidence Boundary, tightened F6, TLS health values table, native zero-value shell example
- `doc/armorclaw.md` with TLS scripts in harness loop, updated Tier A comment, Plan A harness note (`tls-mode` / `tls-restart`), aligned evidence path

### Definition of Done
- [ ] `bash -n scripts/a4_harness.sh` passes
- [ ] `grep -Eq '^\s*\["tls-mode"\]="test-tls-mode-integration\.sh"$' scripts/a4_harness.sh` → exit 0 (exact SUITE_MAP line)
- [ ] `grep -Eq '^\s*\["tls-restart"\]="test-tls-restart-safety\.sh"$' scripts/a4_harness.sh` → exit 0 (exact SUITE_MAP line)
- [ ] `grep -Ec '^\s*\["tls-(mode|restart)"\]=' scripts/a4_harness.sh` → exactly 2
- [ ] `grep -E '^> \*\*Version\*\*: 1\.1\.0$' doc/testing.md` → exit 0 (exact version header)
- [ ] `grep -F '| \`test-tls-restart-safety.sh\` | TLS | 3 | Fingerprint, TLS mode, QR state preservation across bridge restart |' doc/testing.md` → exit 0 (exact table row)
- [ ] `grep -F '| \`test-tls-mode-integration.sh\` | TLS | 10 | Full TLS surface: fingerprint consistency, mode detection, HTTPS enforcement, QR v1/v2 |' doc/testing.md` → exit 0 (exact table row)
- [ ] `grep -A15 '### Evidence Boundary' doc/testing.md | grep 'evidence/armorclaw'` → match AND `grep -A15 '### Evidence Boundary' doc/testing.md | grep 'evidence/tls'` → match
- [ ] `grep -F '| F6 | TLS fingerprint identical across `/fingerprint`, `bridge.status.tls.fingerprint_sha256`, and `openssl x509 -fingerprint -sha256` | `bash tests/test-tls-mode-integration.sh` → PASS |' doc/testing.md` → exit 0 (exact F6 row)
- [ ] `grep '### TLS health values' doc/testing.md` → match AND section contains `ok` / `degraded` entries
- [ ] `grep 'Native mode — skip trust flow' doc/testing.md` → match
- [ ] `grep -A10 'Run all Tier A scripts' doc/armorclaw.md | grep 'test-tls-mode-integration.sh'` → ≥1 AND `grep -A10 'Run all Tier A scripts' doc/armorclaw.md | grep 'test-tls-restart-safety.sh'` → ≥1
- [ ] `grep -A5 'Run all Tier A scripts' doc/armorclaw.md | grep 'includes TLS verification'` → match
- [ ] `grep -A10 'TLS suite integration' doc/armorclaw.md | grep 'a4_harness.sh tls-mode'` → match AND `grep -A10 'TLS suite integration' doc/armorclaw.md | grep 'a4_harness.sh tls-restart'` → match
- [ ] `grep -A15 'Evidence path' doc/armorclaw.md | grep 'evidence/armorclaw'` → match AND `grep -A15 'Evidence path' doc/armorclaw.md | grep 'evidence/tls'` → match
- [ ] No remaining `full-system-{task-name}` evidence path references anywhere in armorclaw.md

### Must Have
- TLS suite entries in SUITE_MAP with exact filenames `test-tls-mode-integration.sh` and `test-tls-restart-safety.sh`
- Scenario counts: restart-safety = "3 scenarios", mode-integration = "10 scenarios"
- Evidence boundary note clearly distinguishing Plan A root from TLS root
- F6 with 3 explicit fingerprint sources
- TLS health values table with `ok` and `degraded` entries in testing.md
- Native zero-value shell example in testing.md
- armorclaw.md harness loop updated to include TLS scripts
- armorclaw.md Tier A loop comment updated to mention TLS verification
- armorclaw.md harness note documenting `a4_harness.sh tls-mode` / `tls-restart`
- armorclaw.md evidence path aligned with testing.md (both roots documented)
- testing.md version bump to 1.1.0

### Must NOT Have (Guardrails)
- Do NOT change test script logic — only documentation and harness wiring
- Do NOT change evidence paths in existing test scripts
- Do NOT add new test scenarios
- Do NOT commit doc files (they are in `.gitignore`)
- Do NOT modify files outside the 3 targets (`a4_harness.sh`, `testing.md`, `armorclaw.md`)
- Do NOT alter Plan A script logic or contract.sh
- Do NOT change the SUITE_MAP key naming convention (lowercase, hyphenated)

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: N/A (script/doc changes)
- **Automated tests**: `bash -n` syntax verification
- **Framework**: bash syntax check

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/tls-harness-alignment/`.

> **Note**: `.sisyphus/evidence/tls-harness-alignment/` is plan-execution working storage for this alignment task. It is NOT one of the runtime evidence roots documented in testing.md (`.sisyphus/evidence/armorclaw/` and `.sisyphus/evidence/tls/`). Do not conflate the plan's working folder with the documented system evidence roots.

### F1-Canonical Rule
The Final Verification Wave (F1) is the **single authoritative source** for all verification commands. Task-level QA scenarios and the spot-check section must reuse F1 commands verbatim — no paraphrasing, no simplification, no alternative patterns. If a task QA command diverges from F1, F1 wins. This prevents drift between "quick checks" and the real gate.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately):
├── Task 1: Add TLS entries to a4_harness.sh SUITE_MAP [quick]
└── Task 2: Update testing.md v1.1 [quick]

Wave 2 (After Wave 1 — verify consistency):
└── Task 3: Sync armorclaw.md (harness loop, Tier A comment, harness note, evidence path) [quick]

Wave FINAL:
└── Cross-file consistency check [quick]
```

### Dependency Matrix
| Task | Depends On | Blocks |
|------|-----------|--------|
| T1 | - | T3 |
| T2 | - | T3 |
| T3 | T1, T2 | Final |

### Agent Dispatch Summary
- **Wave 1**: T1 → `quick`, T2 → `quick`
- **Wave 2**: T3 → `quick`
- **Final**: F1 → `quick`

---

## TODOs

- [x] 1. Add TLS entries to a4_harness.sh SUITE_MAP

  **What to do**:
  - Add two entries to the `SUITE_MAP` associative array in `scripts/a4_harness.sh` (inside the `declare -A SUITE_MAP=( ... )` block, before the closing `)`):
    ```bash
    ["tls-mode"]="test-tls-mode-integration.sh"
    ["tls-restart"]="test-tls-restart-safety.sh"
    ```
  - These entries follow the existing naming convention: lowercase keys with hyphens, values are exact test filenames in `tests/`
  - No other changes to the harness script — the existing runner logic (the `run_suite` function and case dispatcher after SUITE_MAP) handles new suite names automatically

  **Must NOT do**:
  - Do NOT change the existing runner loop logic
  - Do NOT alter existing SUITE_MAP entries
  - Do NOT add any new functions or imports

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single associative array edit, 2 lines added
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T2)
  - **Parallel Group**: Wave 1
  - **Blocks**: T3
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `scripts/a4_harness.sh` — SUITE_MAP declaration block (`declare -A SUITE_MAP=( ... )`). New entries follow same format: `["key"]="filename.sh"`. Insert before the closing `)` of the SUITE_MAP block.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: TLS suite entries present in SUITE_MAP (exact lines)
    Tool: Bash (grep -E)
    Preconditions: scripts/a4_harness.sh exists
    Steps:
      1. Run: grep -Eq '^\s*\["tls-mode"\]="test-tls-mode-integration\.sh"$' scripts/a4_harness.sh
      2. Assert exit 0 (exact tls-mode SUITE_MAP line found)
      3. Run: grep -Eq '^\s*\["tls-restart"\]="test-tls-restart-safety\.sh"$' scripts/a4_harness.sh
      4. Assert exit 0 (exact tls-restart SUITE_MAP line found)
      5. Run: grep -Ec '^\s*\["tls-(mode|restart)"\]=' scripts/a4_harness.sh
      6. Assert count equals 2
    Expected Result: Exactly 2 SUITE_MAP entries with correct key-filename mapping
    Failure Indicators: Count ≠ 2
    Evidence: .sisyphus/evidence/tls-harness-alignment/task1-suite-map-grep.txt

  Scenario: Harness syntax valid after edit
    Tool: Bash (bash -n)
    Preconditions: Edit applied
    Steps:
      1. Run: bash -n scripts/a4_harness.sh
      2. Assert exit code 0 (no output)
    Expected Result: No syntax errors
    Failure Indicators: Non-zero exit or syntax error output
    Evidence: .sisyphus/evidence/tls-harness-alignment/task1-syntax-check.txt
  ```

  **Commit**: YES
  - Message: `fix(tests): wire TLS suites into Plan A harness SUITE_MAP`
  - Files: `scripts/a4_harness.sh`

- [x] 2. Update testing.md to v1.1 (counts, evidence boundary, F6, health values table, native-mode example)

  **What to do**:
  Apply these specific edits to `doc/testing.md`:

  **2a. Version bump**: In the header block (near the top of the file), change the version line from `> **Version**: 1.0.0` to `> **Version**: 1.1.0`

  **2b. Fix scenario counts in Tier A table** (in the "Tier A" subsection of the test scripts table):
  - Find the `test-tls-restart-safety.sh` row. Change the count from `4` to `3` and the description to `Fingerprint, TLS mode, QR state preservation across bridge restart`
  - Find the `test-tls-mode-integration.sh` row. Change the count from `9+1` to `10` (description unchanged)

  **2c. Add Evidence Boundary note** (insert immediately after the first paragraph of the Evidence System section):
  The target paragraph is the one beginning with "All evidence files are written to" under the `## Evidence System` heading. Insert the block directly after this paragraph, before the `### Directory Structure` subsection:
  Insert:
  ```markdown
  ### Evidence Boundary

  Two evidence roots exist for different purposes:

  | Root | Purpose | Written By |
  |------|---------|-----------|
  | `.sisyphus/evidence/armorclaw/` | Plan A pipeline outputs (A0–A4) | `scripts/lib/contract.sh` (`_contract_save()`), called by `a0_discover.sh`, `a1_deploy.sh`, `a2_provision.sh`, etc. |
  | `.sisyphus/evidence/tls/` | TLS verification artifacts | `tests/test-tls-*.sh` and TLS plan execution |

  Agents and scripts must write to the correct root. Plan A scripts always use `.sisyphus/evidence/armorclaw/`. TLS-specific verification uses `.sisyphus/evidence/tls/`.
  ```

  **2d. Tighten F6** (in the verification table, the F6 row):
  Find the row starting with `| F6 | TLS verification (fingerprint consistent across sources) |` and replace with:
  ```
  | F6 | TLS fingerprint identical across `/fingerprint`, `bridge.status.tls.fingerprint_sha256`, and `openssl x509 -fingerprint -sha256` | `bash tests/test-tls-mode-integration.sh` → PASS |
  ```

  **2e. Add TLS health values table** (in the "TLS Mode Derivation" section, after the topology table):
  Find the existing paragraph starting with `**TLS health field**:` and replace it with:
  ```markdown
  ### TLS health values

  | Value | Meaning |
  |-------|---------|
  | `ok` | Bridge has direct access to the active shared certificate material |
  | `degraded` | TLS topology is unchanged, but Bridge cannot read the active cert directly (for example `cert_source="proxy_only"`) |

  Mode never changes due to health — scripts that only check topology (`private`/`public`/`none`) are stable.

  **Shared-cert model**: Bridge and Caddy read from `/etc/armorclaw/certs/server.crt` and `server.key`. `cert_source="shared_cert"` when the cert file is present; `cert_source="proxy_only"` when missing (degraded state).

  **Native mode zero-value semantics**: `bridge.status.tls` is always present. In native mode:
  `mode="none"`, `fingerprint_sha256=""`, `trust_type=""`, `expires_at=0`,
  `san_includes_public_ip=false`.

  Example:
  ```bash
  if [[ "$(jq -r '.tls.mode' status.json)" == "none" ]]; then
      echo "Native mode — skip trust flow"
  fi
  ```
  ```

  **Must NOT do**:
  - Do NOT change test script logic
  - Do NOT add new test scenarios
  - Do NOT change evidence paths in existing scripts
  - Do NOT modify the contract manifest schema
  - Do NOT remove existing sections

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Multiple targeted edits to a single documentation file
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T1)
  - **Parallel Group**: Wave 1
  - **Blocks**: T3
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `doc/testing.md` — Tier A table subsection: the `test-tls-restart-safety.sh` and `test-tls-mode-integration.sh` rows have wrong scenario counts
  - `doc/testing.md` — F6 row in the verification table: currently vague "fingerprint consistent across sources"
  - `doc/testing.md` — "TLS Mode Derivation" section: contains the `**TLS health field**:` paragraph to replace

  **API/Type References**:
  - `tests/test-tls-mode-integration.sh` — Contains exactly 10 scenarios (S1-S10, S10 conditional) to verify the correct count
  - `tests/test-tls-restart-safety.sh` — Contains exactly 3 preservation checks to verify the correct count

  **WHY Each Reference Matters**:
  - Tier A table rows: These are the specific rows with wrong counts that need correcting to 3 and 10
  - F6 row: The vague description needs tightening to specify all 3 fingerprint sources
  - TLS Mode Derivation section: Needs the health values table and shell example added after the topology table

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Version bumped to 1.1.0 (exact header line)
    Tool: Bash (grep -E)
    Steps:
      1. grep -E '^> \*\*Version\*\*: 1\.1\.0$' doc/testing.md
      2. Assert exit 0 (exact version header line)
    Expected Result: Version header line is exactly "1.1.0"
    Evidence: .sisyphus/evidence/tls-harness-alignment/task2-version.txt

  Scenario: Scenario counts corrected (exact Tier A table rows)
    Tool: Bash (grep -F)
    Steps:
      1. grep -F '| `test-tls-restart-safety.sh` | TLS | 3 | Fingerprint, TLS mode, QR state preservation across bridge restart |' doc/testing.md
      2. Assert exit 0 (exact restart-safety row with count 3)
      3. grep -F '| `test-tls-mode-integration.sh` | TLS | 10 | Full TLS surface: fingerprint consistency, mode detection, HTTPS enforcement, QR v1/v2 |' doc/testing.md
      4. Assert exit 0 (exact mode-integration row with count 10)
    Expected Result: Both Tier A table rows match exactly
    Evidence: .sisyphus/evidence/tls-harness-alignment/task2-counts.txt

  Scenario: Evidence boundary section exists
    Tool: Bash (grep)
    Steps:
      1. grep '### Evidence Boundary' doc/testing.md
      2. Assert heading found
      3. grep -A15 '### Evidence Boundary' doc/testing.md | grep 'evidence/armorclaw'
      4. Assert Plan A root present within Evidence Boundary section
      5. grep -A15 '### Evidence Boundary' doc/testing.md | grep 'evidence/tls'
      6. Assert TLS root present within Evidence Boundary section
    Expected Result: Section heading present, both evidence roots verified within that section
    Evidence: .sisyphus/evidence/tls-harness-alignment/task2-evidence-boundary.txt

  Scenario: F6 tightened with 3 fingerprint sources (exact row)
    Tool: Bash (grep -F)
    Steps:
      1. grep -F '| F6 | TLS fingerprint identical across `/fingerprint`, `bridge.status.tls.fingerprint_sha256`, and `openssl x509 -fingerprint -sha256` | `bash tests/test-tls-mode-integration.sh` → PASS |' doc/testing.md
      2. Assert exit 0 (exact F6 row with all 3 sources specified)
    Expected Result: Exact F6 table row present
    Evidence: .sisyphus/evidence/tls-harness-alignment/task2-f6.txt

  Scenario: TLS health values table and native-mode example present
    Tool: Bash (grep)
    Steps:
      1. grep '### TLS health values' doc/testing.md
      2. Assert heading found
      3. grep -A20 '### TLS health values' doc/testing.md | grep -F 'ok' | grep -i 'Bridge has direct access'
      4. Assert exit 0 (ok value in health table)
      5. grep -A20 '### TLS health values' doc/testing.md | grep -F 'degraded' | grep -i 'cannot read'
      6. Assert exit 0 (degraded value in health table)
      7. grep 'Native mode — skip trust flow' doc/testing.md
      8. Assert match found
    Expected Result: Health table heading present, both ok and degraded values verified within that section, shell example present
    Evidence: .sisyphus/evidence/tls-harness-alignment/task2-health-values.txt
  ```

  **Commit**: NO (gitignored — local-only documentation)
  - Files: `doc/testing.md`

- [x] 3. Sync armorclaw.md (harness loop, Tier A comment, harness note, evidence path)

  **What to do**:
  Apply these specific edits to `doc/armorclaw.md`:

  **3a. Add TLS scripts to harness loop + update comment** (in the "Running the Full Harness" / "Run all Tier A scripts" block):
  Change the comment from:
  ```bash
  # Run all Tier A scripts (requires VPS with bridge running)
  ```
  To:
  ```bash
  # Run all Tier A scripts (requires VPS with bridge running) — includes TLS verification
  ```
  And change the Tier A `for` loop to append the two TLS test scripts:
  ```bash
  for f in tests/test-eventbus-streaming.sh tests/test-trust-layer.sh \
           tests/test-system-health-baseline.sh tests/test-secretary-workflow-core.sh \
           tests/test-email-pipeline.sh \
           tests/test-tls-restart-safety.sh tests/test-tls-mode-integration.sh; do
  ```

  **3b. Add harness SUITE_MAP note** (after the harness syntax-check block — the `bash -n` loop, near the end of the "Running the Full Harness" section):
  Add after the syntax check block:
  ```markdown
  **TLS suite integration**: TLS tests are also available through the Plan A harness via:
  ```bash
  bash scripts/a4_harness.sh tls-mode
  bash scripts/a4_harness.sh tls-restart
  ```
  ```

  **3c. Align evidence path** (in the "Evidence and Results" subsection immediately following the harness syntax-check block):
  Find the line `- **Evidence path**: `.sisyphus/evidence/full-system-{task-name}/`` and change to:
  ```
  - **Evidence path**: `.sisyphus/evidence/armorclaw/` (Plan A pipeline); `.sisyphus/evidence/tls/` (TLS verification)
  ```

  **3d. Scan for adjacent stale references**: After completing 3a–3c, search the nearby sections of armorclaw.md for any other occurrences of the old patterns:
  - `grep -n 'full-system-{task-name}' doc/armorclaw.md` — any hit is a stale evidence path that must be updated to match the new dual-root format
  - `grep -n 'test-email-pipeline.sh; do$' doc/armorclaw.md` — any hit outside the targeted block is a stale Tier A loop that must include the TLS scripts
  - Check the "Test Results Location" section and any other harness-related subsections for the same stale patterns
  - Fix any stale references found to be consistent with the changes in 3a–3c

  **Must NOT do**:
  - Do NOT modify Tier B loop (TLS tests are Tier A only)
  - Do NOT change cross-subsystem scenarios
  - Do NOT modify the architecture section or deployment mode docs
  - Do NOT add TLS Mode Detection content — that already exists (added in TLS plan T12)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Targeted edits to sync harness section
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (after T1+T2)
  - **Blocks**: Final
  - **Blocked By**: T1, T2

  **References**:
  **Pattern References**:
  - `doc/armorclaw.md` — "Run all Tier A scripts" block: the Tier A `for` loop and its comment. This is the loop that needs TLS scripts appended.
  - `doc/armorclaw.md` — "Evidence and Results" subsection (immediately after the syntax-check block): contains the stale evidence path `full-system-{task-name}/`
  - `doc/testing.md` (T2 output) — Authoritative source for evidence path language

  **WHY Each Reference Matters**:
  - The "Run all Tier A scripts" block is the primary harness example in armorclaw.md — must match the actual Tier A test list
  - The "Evidence and Results" evidence path must match testing.md's documented roots
  - Adjacent sections (e.g., "Test Results Location") may duplicate these patterns and must be checked for consistency

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Harness loop includes both TLS scripts independently
    Tool: Bash (grep + count)
    Steps:
      1. grep -A10 'Run all Tier A scripts' doc/armorclaw.md | grep -c 'test-tls-mode-integration.sh'
      2. Assert count >= 1
      3. grep -A10 'Run all Tier A scripts' doc/armorclaw.md | grep -c 'test-tls-restart-safety.sh'
      4. Assert count >= 1
    Expected Result: Each TLS test script independently confirmed within the Tier A harness loop
    Evidence: .sisyphus/evidence/tls-harness-alignment/task3-harness-loop.txt

  Scenario: Evidence path aligned with testing.md
    Tool: Bash (grep)
    Steps:
      1. grep -A15 'Evidence path' doc/armorclaw.md | grep 'evidence/armorclaw'
      2. Assert Plan A root present in evidence-path section
      3. grep -A15 'Evidence path' doc/armorclaw.md | grep 'evidence/tls'
      4. Assert TLS root present in evidence-path section
    Expected Result: Both evidence roots documented within the evidence-path section
    Evidence: .sisyphus/evidence/tls-harness-alignment/task3-evidence-path.txt

  Scenario: Tier A loop comment updated to mention TLS
    Tool: Bash (grep)
    Steps:
      1. grep -A5 'Run all Tier A scripts' doc/armorclaw.md | grep 'includes TLS verification'
      2. Assert match found within the Tier A loop section
    Expected Result: Comment on Tier A loop mentions TLS verification
    Evidence: .sisyphus/evidence/tls-harness-alignment/task3-comment.txt

  Scenario: Harness SUITE_MAP note added
    Tool: Bash (grep)
    Steps:
      1. grep -A10 'TLS suite integration' doc/armorclaw.md | grep 'a4_harness.sh tls-mode'
      2. Assert match found within the TLS suite integration block
      3. grep -A10 'TLS suite integration' doc/armorclaw.md | grep 'a4_harness.sh tls-restart'
      4. Assert match found within the TLS suite integration block
    Expected Result: Both harness suite commands documented
    Evidence: .sisyphus/evidence/tls-harness-alignment/task3-harness-note.txt

  Scenario: No stale references remain in armorclaw.md (T3d enforcement)
    Tool: Bash (grep)
    Preconditions: T3a–T3d edits applied
    Steps:
      1. grep -c 'full-system-{task-name}' doc/armorclaw.md
      2. Assert count equals 0 (no stale evidence-path references remain)
      3. grep -c 'test-email-pipeline.sh; do$' doc/armorclaw.md
      4. Assert count equals 0 (no stale Tier A loops without TLS scripts remain)
    Expected Result: Zero matches for both stale patterns — all adjacent sections updated
    Failure Indicators: Any match count > 0 means a stale reference was missed
    Evidence: .sisyphus/evidence/tls-harness-alignment/task3-stale-scan.txt
  ```

  **Commit**: NO (gitignored — local-only documentation)
  - Files: `doc/armorclaw.md`

---

## Final Verification Wave

- [x] F1. **Cross-File Consistency Check**
  Verify all 3 files are consistent across ALL required outputs from T1, T2, and T3:
  
  **Harness (a4_harness.sh)**:
  - `grep -Eq '^\s*\["tls-mode"\]="test-tls-mode-integration\.sh"$' scripts/a4_harness.sh` → exit 0 (exact tls-mode line)
  - `grep -Eq '^\s*\["tls-restart"\]="test-tls-restart-safety\.sh"$' scripts/a4_harness.sh` → exit 0 (exact tls-restart line)
  - `grep -Ec '^\s*\["tls-(mode|restart)"\]=' scripts/a4_harness.sh` → exactly 2
  - `bash -n scripts/a4_harness.sh` → exit 0
  
  **testing.md**:
  - `grep -E '^> \*\*Version\*\*: 1\.1\.0$' doc/testing.md` → exit 0 (exact version header)
  - `grep -F '| \`test-tls-restart-safety.sh\` | TLS | 3 | Fingerprint, TLS mode, QR state preservation across bridge restart |' doc/testing.md` → exit 0 (exact restart-safety row)
  - `grep -F '| \`test-tls-mode-integration.sh\` | TLS | 10 | Full TLS surface: fingerprint consistency, mode detection, HTTPS enforcement, QR v1/v2 |' doc/testing.md` → exit 0 (exact mode-integration row)
  - `grep '### TLS health values' doc/testing.md` → match
  - `grep -A20 '### TLS health values' doc/testing.md | grep -F 'ok' | grep -i 'Bridge has direct access'` → match (ok value in health section)
  - `grep -A20 '### TLS health values' doc/testing.md | grep -F 'degraded' | grep -i 'cannot read'` → match (degraded value in health section)
  - `grep 'Native mode — skip trust flow' doc/testing.md` → match
  - `grep -A15 '### Evidence Boundary' doc/testing.md | grep 'evidence/armorclaw'` → match (Plan A root in Evidence Boundary section)
  - `grep -A15 '### Evidence Boundary' doc/testing.md | grep 'evidence/tls'` → match (TLS root in Evidence Boundary section)
  - `grep -F '| F6 | TLS fingerprint identical across `/fingerprint`, `bridge.status.tls.fingerprint_sha256`, and `openssl x509 -fingerprint -sha256` | `bash tests/test-tls-mode-integration.sh` → PASS |' doc/testing.md` → exit 0 (exact full F6 row)
  
  **armorclaw.md**:
  - `grep -A10 'Run all Tier A scripts' doc/armorclaw.md | grep -c 'test-tls-mode-integration.sh'` → ≥1
  - `grep -A10 'Run all Tier A scripts' doc/armorclaw.md | grep -c 'test-tls-restart-safety.sh'` → ≥1
  - `grep -A5 'Run all Tier A scripts' doc/armorclaw.md | grep 'includes TLS verification'` → match (Tier A comment scoped)
  - `grep -A10 'TLS suite integration' doc/armorclaw.md | grep 'a4_harness.sh tls-mode'` → match (harness note scoped)
  - `grep -A10 'TLS suite integration' doc/armorclaw.md | grep 'a4_harness.sh tls-restart'` → match (harness note scoped)
  - `grep -A15 'Evidence path' doc/armorclaw.md | grep 'evidence/armorclaw'` → match (Plan A root in evidence-path section)
  - `grep -A15 'Evidence path' doc/armorclaw.md | grep 'evidence/tls'` → match (TLS root in evidence-path section)
  - `grep -c 'full-system-{task-name}' doc/armorclaw.md` → 0 (no stale evidence-path references remain anywhere)
  - `grep -c 'test-email-pipeline.sh; do$' doc/armorclaw.md` → 0 (no stale Tier A loops without TLS scripts remain)
  
  Output: `Harness [2/2 TLS entries, syntax OK] | testing.md [version/counts/health/evidence/F6] | armorclaw.md [loop/comment/note/paths/stale-clean] | VERDICT: APPROVE/REJECT`

---

## Commit Strategy

- **Commit**: `scripts/a4_harness.sh` only
- Message: `fix(tests): wire TLS suites into Plan A harness SUITE_MAP`
- Files: `scripts/a4_harness.sh`
- **Not committed**: `doc/testing.md` and `doc/armorclaw.md` are in `.gitignore` — these are local-only reference material changes, applied but not tracked by git

---

## Success Criteria

### Verification Commands

> These are minimal spot checks. The authoritative verification is F1 (Final Verification Wave), which covers all required outputs comprehensively. Commands below are a subset copied from F1.

```bash
# Harness
bash -n scripts/a4_harness.sh  # Expected: no output (success)
grep -Ec '^\s*\["tls-(mode|restart)"\]=' scripts/a4_harness.sh  # Expected: 2

# testing.md
grep -E '^> \*\*Version\*\*: 1\.1\.0$' doc/testing.md  # Expected: exact version header
grep -F '| `test-tls-restart-safety.sh` | TLS | 3 |' doc/testing.md  # Expected: exact row
grep -F '| `test-tls-mode-integration.sh` | TLS | 10 |' doc/testing.md  # Expected: exact row
grep -A15 '### Evidence Boundary' doc/testing.md | grep 'evidence/armorclaw'  # Expected: Plan A root in section
grep -A15 '### Evidence Boundary' doc/testing.md | grep 'evidence/tls'  # Expected: TLS root in section
grep '### TLS health values' doc/testing.md  # Expected: match
grep 'Native mode — skip trust flow' doc/testing.md  # Expected: match

# armorclaw.md
grep -A10 'Run all Tier A scripts' doc/armorclaw.md | grep 'test-tls-mode-integration.sh'  # Expected: match in loop
grep -A10 'Run all Tier A scripts' doc/armorclaw.md | grep 'test-tls-restart-safety.sh'  # Expected: match in loop
grep -A5 'Run all Tier A scripts' doc/armorclaw.md | grep 'includes TLS verification'  # Expected: Tier A comment
grep -A10 'TLS suite integration' doc/armorclaw.md | grep 'a4_harness.sh tls-mode'  # Expected: harness note
grep -A10 'TLS suite integration' doc/armorclaw.md | grep 'a4_harness.sh tls-restart'  # Expected: harness note
grep -A15 'Evidence path' doc/armorclaw.md | grep 'evidence/armorclaw'  # Expected: Plan A root in section
grep -A15 'Evidence path' doc/armorclaw.md | grep 'evidence/tls'  # Expected: TLS root in section
! grep 'full-system-{task-name}' doc/armorclaw.md  # Expected: zero stale evidence-path refs
! grep 'test-email-pipeline.sh; do$' doc/armorclaw.md  # Expected: zero stale Tier A loops
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] `bash -n` passes on a4_harness.sh
- [ ] `a4_harness.sh` contains both `tls-mode` and `tls-restart` SUITE_MAP entries
- [ ] `doc/testing.md` version is `1.1.0`
- [ ] Scenario counts correct in testing.md
- [ ] TLS health values section present in testing.md
- [ ] Native-mode shell example present in testing.md
- [ ] Evidence Boundary section present with both roots
- [ ] F6 specifies 3 fingerprint sources
- [ ] armorclaw.md harness loop includes TLS scripts
- [ ] armorclaw.md Tier A loop comment mentions TLS verification
- [ ] armorclaw.md has harness note (a4_harness.sh tls-mode / tls-restart)
- [ ] armorclaw.md evidence paths aligned with testing.md
- [ ] Zero stale `full-system-{task-name}` evidence-path references in armorclaw.md
- [ ] Zero stale Tier A loop patterns (without TLS scripts) in armorclaw.md
