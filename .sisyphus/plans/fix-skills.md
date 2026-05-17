# Fix ArmorClaw Skills System — Validated Findings Only

## TL;DR

> **Quick Summary**: Fix the 7 remaining broken findings in the skills system (S1, S4, S5, S12, S13, S16, F3 + newly discovered parseSkillFile field extraction gap). Write confirmation tests for the 6 already-fixed findings (S2, S3, S6, S7, S8, F1). Add security-critical test coverage.
> 
> **Deliverables**:
> - Fixed `containsDangerousChars` with context-aware allowlist
> - Pinned `deploy.yaml` curl to release tags with SHA256
> - Fixed `status.yaml` return 0 → exit 0
> - Fixed zero timeout for SKILL.md-loaded skills
> - Fixed timeout detection using correct context
> - Fixed `provision.yaml` sudo + automation level
> - Completed `parseSkillFile` field extraction (timeout, version, parameters, enabled)
> - Confirmation tests for 6 already-fixed findings
> - Expanded test suite from 4 to ~30 test functions
> 
> **Estimated Effort**: Medium
> **Parallel Execution**: YES — 5 waves
> **Critical Path**: T1 (confirm fixes) → T2 (dangerous chars) → T6 (parseSkillFile) → T8 (SSRF gap) → F1-F4 (verification)

---

## Context

### Original Request
User asked to "make a plan to fix skills" based on the deployment skills analysis that surfaced 46 findings from SKILLS_REVIEW.md.

### Interview Summary
**Key Discussions**:
- Initial analysis identified 3 CRITICAL, 10 HIGH, 15 MEDIUM, 18 LOW findings
- Metis consultation revealed **SKILLS_REVIEW.md is substantially stale** — 6 of 10 top-priority findings already fixed
- Re-validation against current HEAD confirmed: S2, S3, S6, S7, S8, F1 are fixed; S1, S4, S5, S12, S13, S16, F3 remain broken
- Metis discovered unstated issue: `parseSkillFile` only extracts 3 fields (name, description, homepage), silently drops timeout, version, parameters, enabled

**Research Findings**:
- `registry.go:282-284`: Already uses `yaml.v3` (not hand-rolled)
- `executor.go:373`: Already deny-by-default
- `ssrf.go:22-36`: IPv6 ranges present (only `ff00::/8` multicast missing)
- `executor.go:55`: Governor installed as default SkillGate
- `containsDangerousChars` blocks `|`, `;`, `` ` ``, `$(`, `${` — NOT the `&`, `{}`, `()`, `<>` the review claimed
- `parseSkillFile` at registry.go:141 initializes parameters to empty map and never populates
- Only 1 test file exists: `executor_authorizer_test.go` (130 lines, 4 functions)

### Metis Review
**Identified Gaps** (addressed):
- Stale review findings → Re-validated every finding against current HEAD
- S1 mischaracterized → Using actual blocked chars (`|`, `;`, `` ` ``, `$(`, `${`)
- `parseSkillFile` field extraction gap → Added as new task (T6)
- Scope creep risk (mock implementations, router keywords) → Explicitly excluded

---

## Work Objectives

### Core Objective
Fix the 7 remaining broken findings and add confirmation tests for the 6 already-fixed findings. Expand test coverage from 1 file/4 functions to ~8 files/~30 functions.

### Concrete Deliverables
- `bridge/internal/skills/executor.go` — Fixed dangerous chars, timeout handling
- `bridge/internal/skills/registry.go` — Completed parseSkillFile field extraction
- `bridge/internal/skills/ssrf.go` — Added ff00::/8 multicast CIDR
- `.skills/deploy.yaml` — Pinned curl to release tag
- `.skills/status.yaml` — Fixed return 0 → exit 0
- `.skills/provision.yaml` — Quoted sudo, changed to confirm automation
- 7 new/updated test files in `bridge/internal/skills/`

### Definition of Done
- [x] `cd bridge && go test -v ./internal/skills/...` → ALL PASS, ~34 test functions
- [x] `bash tests/test-deployment-skills.sh` → ALL PASS
- [ ] `cd bridge && go build -o /dev/null ./cmd/bridge` → clean build *(pre-existing yara CGO dep issue, not skills scope)*
- [x] Zero instances of `return 0` in `.skills/status.yaml`
- [x] Zero instances of `raw.githubusercontent.com/.../main/` in `.skills/deploy.yaml`

### Must Have
- All 7 broken findings fixed with targeted minimal patches
- Confirmation tests proving the 6 already-fixed findings stay fixed (regression guard)
- `parseSkillFile` extracts timeout, version, enabled from SKILL.md frontmatter
- Every test runnable with `go test` — no external dependencies, no live VPS

### Must NOT Have (Guardrails)
- Do NOT re-fix already-fixed items (S2, S3, S6, S7, S8, F1) — only write confirmation tests
- Do NOT change mock implementations (email, slack, calendar) — those are intentional stubs
- Do NOT add router keyword maps (F4) or schema generation (F5) — defer to separate phase
- Do NOT add PowerShell implementations for P1-P4 platform claims — defer to separate phase
- Do NOT implement DNS rebinding protection (S11) — architectural change, separate phase
- Do NOT replace the custom `contains()` function with `strings.Contains()` yet — test first, then replace
- Do NOT touch `bridge/pkg/skills/` (has excellent 1.8:1 test ratio already)
- Do NOT modify any file outside `bridge/internal/skills/` and `.skills/` (brownfield, minimal patches)

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (Go testing, `go test`)
- **Automated tests**: YES (tests-after for fixes, confirmation tests for already-fixed)
- **Framework**: Go standard `testing` package + `testify/assert` (if already in go.mod, else stdlib only)

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go code**: Use `go test -v -run TestName ./internal/skills/...` — assert output, coverage
- **YAML files**: Use `grep` — assert no bad patterns, confirm good patterns
- **Build**: Use `go build -o /dev/null ./cmd/bridge` — assert clean compilation

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — confirmation tests + quick YAML fixes):
├── Task 1: Confirmation tests for 6 already-fixed findings [deep]
├── Task 2: Fix containsDangerousChars (S1) + tests [deep]
├── Task 3: Fix deploy.yaml curl pinning (S4) [quick]
├── Task 4: Fix status.yaml return 0 → exit 0 (S5) [quick]
├── Task 5: Fix provision.yaml sudo + automation (S16) [quick]
└── Task 6: Fix zero timeout + wrong ctx (S12, S13) + tests [deep]

Wave 2 (After Wave 1 — field extraction + SSRF gap + expand tests):
├── Task 7: Complete parseSkillFile field extraction (F3+) + tests [deep]
├── Task 8: Add ff00::/8 to SSRF (S6 remaining) + tests [quick]
└── Task 9: Expand policy + registry tests [unspecified-high]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay

Critical Path: T1 → T2 → T7 → F1-F4 → user okay
Max Concurrent: 6 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | — | 7, 9 | 1 |
| 2 | — | F1-F4 | 1 |
| 3 | — | F1-F4 | 1 |
| 4 | — | F1-F4 | 1 |
| 5 | — | F1-F4 | 1 |
| 6 | — | F1-F4 | 1 |
| 7 | 1 | 9 | 2 |
| 8 | — | F1-F4 | 2 |
| 9 | 1, 7 | F1-F4 | 2 |
| F1-F4 | ALL | user okay | FINAL |

### Agent Dispatch Summary

- **Wave 1**: **6** — T1 → `deep`, T2 → `deep`, T3 → `quick`, T4 → `quick`, T5 → `quick`, T6 → `deep`
- **Wave 2**: **3** — T7 → `deep`, T8 → `quick`, T9 → `unspecified-high`
- **FINAL**: **4** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Confirmation Tests for Already-Fixed Findings (S2, S3, S6, S7, S8, F1)

  **What to do**:
  - Create `bridge/internal/skills/confirmation_test.go`
  - Write test `TestDenyByDefault_S2` — verify `IsAllowed("unknown.skill")` returns `false`
  - Write test `TestYAMLv3Parser_S3` — parse a SKILL.md with nested `metadata.openclaw` object, verify fields extracted
  - Write test `TestIPv6Blocking_S6` — verify `::1`, `fc00::`, `fe80::`, `::ffff:0:0`, `::/128`, `64:ff9b::` are all blocked
  - Write test `TestExtractHostStripsUserinfo_S7` — verify `https://attacker@169.254.169.254/` resolves to `169.254.169.254`
  - Write test `TestEmailSendPolicy_S8` — verify `email.send` has policy with risk "high" and AutoExecute false
  - Write test `TestDefaultGovernorInstalled_F1` — verify `NewSkillExecutor()` installs a non-nil SkillGate
  - All tests must PASS against current code (no code changes — just proving the fixes exist)

  **Must NOT do**:
  - Do NOT modify any production code
  - Do NOT modify any existing test files

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Requires understanding the executor pipeline and writing precise assertions
  - **Skills**: []
  - **Skills Evaluated but Omitted**: None needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2-6)
  - **Blocks**: Tasks 7, 9
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/internal/skills/executor_authorizer_test.go` — Existing test file showing test conventions and imports used
  - `bridge/pkg/skills/learned_store_test.go` — Reference for Go test patterns in this project

  **API/Type References**:
  - `bridge/internal/skills/executor.go:49-64` — `NewSkillExecutorWithConfig` showing Governor installation (F1)
  - `bridge/internal/skills/executor.go:361-375` — `IsAllowed` showing deny-by-default (S2)
  - `bridge/internal/skills/registry.go:282-284` — `parseYAMLFrontmatter` showing yaml.v3 usage (S3)
  - `bridge/internal/skills/ssrf.go:22-36` — IPv6 CIDRs (S6)
  - `bridge/internal/skills/ssrf.go:119-125` — `extractHost` using url.Parse (S7)
  - `bridge/internal/skills/policy.go:85-91` — email.send policy (S8)

  **WHY Each Reference Matters**:
  - executor_authorizer_test.go: Copy import patterns, assertion style, test structure
  - Each function reference: Verify the fix exists and write an assertion that proves it

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Confirmation tests pass
    Tool: Bash (go test)
    Preconditions: bridge/ directory, Go toolchain available
    Steps:
      1. Run: cd bridge && go test -v -run "TestDenyByDefault_S2|TestYAMLv3Parser_S3|TestIPv6Blocking_S6|TestExtractHostStripsUserinfo_S7|TestEmailSendPolicy_S8|TestDefaultGovernorInstalled_F1" ./internal/skills/...
      2. Assert: All 6 tests PASS with explicit pass lines in output
    Expected Result: 6/6 tests PASS, 0 failures
    Failure Indicators: Any test shows FAIL, panic, or compilation error
    Evidence: .sisyphus/evidence/task-1-confirmation-tests.txt

  Scenario: No production code changed
    Tool: Bash (git diff)
    Preconditions: Git clean state before task
    Steps:
      1. Run: git diff --stat -- ':(exclude)*_test.go'
      2. Assert: Empty output (no non-test files changed)
    Expected Result: Zero production files modified
    Failure Indicators: Any file listed in diff output
    Evidence: .sisyphus/evidence/task-1-no-prod-changes.txt
  ```

  **Commit**: YES
  - Message: `test(skills): add confirmation tests for already-fixed findings S2,S3,S6,S7,S8,F1`
  - Files: `bridge/internal/skills/confirmation_test.go`
  - Pre-commit: `cd bridge && go test -v ./internal/skills/...`

- [x] 2. Fix containsDangerousChars (S1) + Tests

  **What to do**:
  - The current implementation blocks `|`, `;`, `` ` ``, `$(`, `${` on ALL string parameters via `preExecutionChecks`
  - Replace with context-aware validation:
    - For URL parameters: allow `|`, `;`, `` ` ``, `$(`, `${` (these are command injection chars, not relevant for URLs that won't be shell-executed). Actually — re-think: URLs ARE passed to handlers, not shell. The check should only block on parameters that will be used in shell contexts.
    - Simpler approach: Replace the blanket check with a check that only applies to parameters destined for shell execution. Since skills execute via Go handlers (not shell), remove the blanket check entirely and add targeted validation in individual skill handlers that DO shell out (currently: none in production code).
    - **Minimal patch approach**: Replace `containsDangerousChars` with a no-op that logs a warning, OR remove the call from `preExecutionChecks` entirely since skills execute via Go HTTP handlers, not shell commands.
  - Write `bridge/internal/skills/executor_dangerous_test.go`:
    - Test `TestContainsDangerousChars_URLWithQueryParams` — `"https://api.example.com/search?q=test&page=1"` should NOT be blocked
    - Test `TestContainsDangerousChars_CommandSubstitution` — `"$(whoami)"` and `"${PATH}"` SHOULD be flagged
    - Test `TestContainsDangerousChars_PipeInjection` — `"cat file | rm -rf"` SHOULD be flagged
    - Test `TestPreExecutionChecks_LegitimateData` — verify JSON bodies, math expressions, HTML content pass through
  - Run `lsp_find_references` on `containsDangerousChars` before changing to map all callers

  **Must NOT do**:
  - Do NOT add new shell-execution paths
  - Do NOT change the `contains()` helper yet (separate concern)
  - Do NOT modify any skill handler files

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Security-critical change requiring understanding of the PETG pipeline
  - **Skills**: []
  - **Skills Evaluated but Omitted**: None needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3-6)
  - **Blocks**: F1-F4
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/internal/skills/executor.go:240-280` — `preExecutionChecks` and `containsDangerousChars` (the code to change)
  - `bridge/internal/skills/executor.go:115-120` — Where `preExecutionChecks` is called in the Execute pipeline
  - `bridge/internal/skills/executor_authorizer_test.go` — Test conventions

  **API/Type References**:
  - `bridge/internal/skills/executor.go:272-279` — Current dangerous chars: `|`, `;`, `` ` ``, `$(`, `${`

  **WHY Each Reference Matters**:
  - executor.go:240-280: The actual code to modify
  - executor.go:115-120: Where the check is called — need to understand the execution context
  - executor_authorizer_test.go: Test patterns to follow

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Dangerous chars tests pass
    Tool: Bash (go test)
    Preconditions: bridge/ directory
    Steps:
      1. Run: cd bridge && go test -v -run "TestContainsDangerousChars|TestPreExecutionChecks" ./internal/skills/...
      2. Assert: All tests PASS
    Expected Result: Tests for URL params, JSON bodies, HTML content all PASS (not blocked)
    Failure Indicators: Any test FAIL, especially URL/query param tests
    Evidence: .sisyphus/evidence/task-2-dangerous-chars-tests.txt

  Scenario: Build still clean
    Tool: Bash (go build)
    Preconditions: Changes applied
    Steps:
      1. Run: cd bridge && CGO_ENABLED=0 go vet ./internal/skills/...
      2. Assert: No errors
    Expected Result: Clean vet output
    Failure Indicators: Any vet warnings or errors
    Evidence: .sisyphus/evidence/task-2-vet.txt
  ```

  **Commit**: YES
  - Message: `fix(skills): replace blanket dangerousChars with context-aware validation (S1)`
  - Files: `bridge/internal/skills/executor.go`, `bridge/internal/skills/executor_dangerous_test.go`
  - Pre-commit: `cd bridge && go test -v ./internal/skills/...`

- [x] 3. Fix deploy.yaml Curl Pinning (S4)

  **What to do**:
  - In `.skills/deploy.yaml`, lines 163 and 166 reference `https://raw.githubusercontent.com/Gemutly/ArmorClaw/main/deploy/install.sh`
  - Pin to a specific release tag: replace `/main/` with `/v0.7.0/` (current release per review.md)
  - Add SHA256 checksum verification step after the curl download
  - The step should: download script → verify checksum → then execute
  - If no SHA256SUMS entry exists for the current release, add a comment with instructions for manual verification

  **Must NOT do**:
  - Do NOT change the install.sh script itself
  - Do NOT add new deployment modes
  - Do NOT modify the SSH command structure

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 2-line change in YAML file
  - **Skills**: []
  - **Skills Evaluated but Omitted**: None needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-2, 4-6)
  - **Blocks**: F1-F4
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `.skills/deploy.yaml:163,166` — The two curl|bash lines to pin
  - `deploy/SHA256SUMS` — Existing checksum file for reference

  **WHY Each Reference Matters**:
  - deploy.yaml:163,166: Exact lines to change
  - SHA256SUMS: Shows the existing checksum pattern to follow

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: No unversioned main references
    Tool: Bash (grep)
    Preconditions: Changes applied
    Steps:
      1. Run: grep 'raw.githubusercontent.com.*ArmorClaw.*main' .skills/deploy.yaml
      2. Assert: Exit code 1 (no matches found)
    Expected Result: Zero matches for unversioned main branch URLs
    Failure Indicators: Any line matches the grep pattern
    Evidence: .sisyphus/evidence/task-3-curl-pinned.txt

  Scenario: YAML still valid
    Tool: Bash (python3)
    Preconditions: Changes applied
    Steps:
      1. Run: python3 -c "import yaml; yaml.safe_load(open('.skills/deploy.yaml'))"
      2. Assert: Exit code 0
    Expected Result: YAML parses without errors
    Failure Indicators: Python traceback
    Evidence: .sisyphus/evidence/task-3-yaml-valid.txt
  ```

  **Commit**: YES
  - Message: `fix(deploy): pin curl|bash to release tag with integrity check (S4)`
  - Files: `.skills/deploy.yaml`
  - Pre-commit: `python3 -c "import yaml; yaml.safe_load(open('.skills/deploy.yaml'))"`

- [x] 4. Fix status.yaml return 0 → exit 0 (S5)

  **What to do**:
  - In `.skills/status.yaml`, line 150 has `return 0` outside a function scope
  - Replace `return 0` with `exit 0`
  - Verify the surrounding context — the `return 0` is inside an SSH command string that runs remotely, so `exit 0` is correct

  **Must NOT do**:
  - Do NOT change any other lines in status.yaml
  - Do NOT change the step structure

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 1-line change in YAML file
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-3, 5-6)
  - **Blocks**: F1-F4
  - **Blocked By**: None

  **References**:
  - `.skills/status.yaml:150` — The `return 0` line to fix

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: No return 0 in status.yaml
    Tool: Bash (grep)
    Steps:
      1. Run: grep 'return 0' .skills/status.yaml
      2. Assert: Exit code 1 (no matches)
    Expected Result: Zero instances of "return 0"
    Evidence: .sisyphus/evidence/task-4-no-return0.txt

  Scenario: exit 0 present
    Tool: Bash (grep)
    Steps:
      1. Run: grep 'exit 0' .skills/status.yaml
      2. Assert: At least 1 match
    Expected Result: "exit 0" present at the fixed line
    Evidence: .sisyphus/evidence/task-4-exit0-present.txt
  ```

  **Commit**: YES
  - Message: `fix(status): replace return 0 with exit 0 (S5)`
  - Files: `.skills/status.yaml`
  - Pre-commit: `python3 -c "import yaml; yaml.safe_load(open('.skills/status.yaml'))"`

- [x] 5. Fix provision.yaml sudo + Automation (S16)

  **What to do**:
  - In `.skills/provision.yaml`:
    - Line 42: `sudo $CMD` → quote as `sudo "$CMD"`
    - Line 48: `sudo ./deploy/armorclaw-provision.sh --show-url` → already safe (no variable), no quoting needed
    - Lines 35 and 45: Change `automation: "auto"` to `automation: "confirm"` for steps that use `sudo`
  - This ensures privilege escalation requires user confirmation before executing

  **Must NOT do**:
  - Do NOT remove `sudo` — the script likely requires it for system-level operations
  - Do NOT change the `manual_entry` guide step

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 3-line change in YAML file
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-4, 6)
  - **Blocks**: F1-F4
  - **Blocked By**: None

  **References**:
  - `.skills/provision.yaml:35,42,45,48` — The sudo and automation lines to fix

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: sudo commands quoted
    Tool: Bash (grep)
    Steps:
      1. Run: grep 'sudo \$CMD' .skills/provision.yaml
      2. Assert: Exit code 1 (no unquoted $CMD with sudo)
      3. Run: grep 'sudo "\$CMD"' .skills/provision.yaml
      4. Assert: At least 1 match (quoted $CMD)
    Expected Result: Unquoted variable eliminated, quoted variable present
    Evidence: .sisyphus/evidence/task-5-sudo-quoted.txt

  Scenario: sudo steps use confirm automation
    Tool: Bash (grep)
    Steps:
      1. Run: grep -B2 'sudo' .skills/provision.yaml | grep 'automation'
      2. Assert: All automation lines near sudo show "confirm" (not "auto")
    Expected Result: All sudo steps gated behind confirm
    Evidence: .sisyphus/evidence/task-5-confirm-automation.txt
  ```

  **Commit**: YES
  - Message: `fix(provision): quote sudo commands and change to confirm automation (S16)`
  - Files: `.skills/provision.yaml`
  - Pre-commit: `python3 -c "import yaml; yaml.safe_load(open('.skills/provision.yaml'))"`

- [x] 6. Fix Zero Timeout + Wrong Timeout Context (S12, S13)

  **What to do**:
  - **S12**: In `executor.go:132`, `context.WithTimeout(ctx, skill.Timeout)` creates immediate expiry when `skill.Timeout` is zero (SKILL.md-loaded skills via `parseSkillFile` don't extract timeout)
    - Add a default: if `skill.Timeout <= 0`, use `30 * time.Second`
    - This goes in the `Execute` method before creating the context
  - **S13**: In `executor.go:138`, `ctx.Err() == context.DeadlineExceeded` checks the parent context, not the execution context
    - Change to `executionCtx.Err() == context.DeadlineExceeded` to correctly detect skill-level timeouts
  - Write `bridge/internal/skills/executor_timeout_test.go`:
    - Test `TestDefaultTimeout_S12` — execute a skill with zero timeout, verify it gets 30s default
    - Test `TestTimeoutDetection_S13` — execute a skill that exceeds deadline, verify error type is "timeout"
    - Test `TestParentCancellationNotMisreported_S13` — cancel parent context, verify error type is NOT "timeout" (it's "context canceled")

  **Must NOT do**:
  - Do NOT change the timeout values of programmatic skills (they already have timeouts set)
  - Do NOT modify `parseSkillFile` (that's Task 7)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Concurrency and context handling requires careful understanding
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-5)
  - **Blocks**: F1-F4
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/internal/skills/executor.go:128-145` — The timeout execution block (lines to change)
  - `bridge/internal/skills/executor_authorizer_test.go` — Test conventions

  **API/Type References**:
  - `bridge/internal/skills/registry.go:140-141` — Where SKILL.md skills get empty parameters (no timeout extracted)

  **WHY Each Reference Matters**:
  - executor.go:128-145: Exact code block to modify
  - registry.go:140-141: Root cause of zero timeouts — parseSkillFile doesn't extract this field

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Timeout tests pass
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v -run "TestDefaultTimeout_S12|TestTimeoutDetection_S13|TestParentCancellationNotMisreported" ./internal/skills/...
      2. Assert: All 3 tests PASS
    Expected Result: 3/3 PASS
    Evidence: .sisyphus/evidence/task-6-timeout-tests.txt

  Scenario: Existing tests still pass
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v ./internal/skills/...
      2. Assert: ALL tests pass (no regression)
    Expected Result: Zero failures
    Evidence: .sisyphus/evidence/task-6-all-tests.txt
  ```

  **Commit**: YES
  - Message: `fix(skills): add default timeout and fix timeout context detection (S12,S13)`
  - Files: `bridge/internal/skills/executor.go`, `bridge/internal/skills/executor_timeout_test.go`
  - Pre-commit: `cd bridge && go test -v ./internal/skills/...`

- [x] 7. Complete parseSkillFile Field Extraction (F3+)

  **What to do**:
  - Current `parseSkillFile` at `registry.go:86-143` only extracts: `name`, `description`, `homepage`
  - It silently drops: `timeout`, `version`, `enabled`, and `parameters`
  - Add extraction for:
    - `timeout` — parse as integer, convert to `time.Duration` (seconds), set on `skill.Timeout`
    - `version` — parse as string, set on `skill.Version`
    - `enabled` — parse as boolean, set on `skill.Enabled` (currently hardcoded `true` at line 94)
    - `parameters` — extract the parameter map from frontmatter and populate `skill.Parameters`
  - Keep the existing field extraction logic — just add the missing fields after line 139
  - Remove the comment "simplified for now" at line 140-141 (replace with actual extraction)
  - Write `bridge/internal/skills/registry_parse_test.go`:
    - Test `TestParseSkillFile_AllFields` — parse a real SKILL.md, verify all fields populated
    - Test `TestParseSkillFile_TimeoutExtraction` — verify timeout parsed as seconds
    - Test `TestParseSkillFile_EnabledFalse` — verify `enabled: false` respected
    - Test `TestParseSkillFile_MissingOptionalFields` — verify defaults when fields absent

  **Must NOT do**:
  - Do NOT change how programmatic skills are registered (they set their own fields)
  - Do NOT change the YAML parser (yaml.v3 is already correct)
  - Do NOT modify `ScanSkills` or any other function

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Requires understanding the Skill struct and SKILL.md format
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 1 confirmation tests for baseline)
  - **Parallel Group**: Wave 2 (with Tasks 8-9)
  - **Blocks**: Task 9
  - **Blocked By**: Task 1

  **References**:

  **Pattern References**:
  - `bridge/internal/skills/registry.go:86-143` — `parseSkillFile` function (the code to modify)
  - `bridge/internal/skills/registry.go:287-340` — `RegisterWebDAV` showing what a complete Skill struct looks like (name, description, timeout, version, parameters, risk, domain)

  **API/Type References**:
  - `bridge/internal/skills/registry.go:1-30` — `Skill` and `Param` struct definitions
  - `container/openclaw-src/skills/*/SKILL.md` — Real SKILL.md files with nested metadata to test against

  **WHY Each Reference Matters**:
  - registry.go:86-143: The function to modify
  - RegisterWebDAV: Shows the target state — what a fully-populated Skill looks like
  - SKILL.md files: Real test inputs to verify parsing works

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: parseSkillFile extracts all fields
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v -run "TestParseSkillFile" ./internal/skills/...
      2. Assert: All tests PASS
    Expected Result: 4/4 tests PASS, field extraction verified
    Evidence: .sisyphus/evidence/task-7-parse-test.txt

  Scenario: Confirmation tests still pass (no regression)
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v -run "TestYAMLv3Parser_S3" ./internal/skills/...
      2. Assert: PASS
    Expected Result: The yaml.v3 confirmation test still works after field extraction changes
    Evidence: .sisyphus/evidence/task-7-no-regression.txt
  ```

  **Commit**: YES
  - Message: `fix(skills): complete parseSkillFile field extraction for timeout/version/enabled (F3)`
  - Files: `bridge/internal/skills/registry.go`, `bridge/internal/skills/registry_parse_test.go`
  - Pre-commit: `cd bridge && go test -v ./internal/skills/...`

- [x] 8. Add ff00::/8 Multicast to SSRF (S6 Remaining)

  **What to do**:
  - In `ssrf.go:22-36`, the CIDR list includes most IPv6 private ranges but is missing `ff00::/8` (multicast)
  - Add `"ff00::/8"` to the `cidrs` slice in the `init()` function
  - Write test in `bridge/internal/skills/ssrf_test.go`:
    - Test `TestSSRF_IPv6Multicast` — verify `ff00::1` is blocked
    - Test `TestSSRF_ExistingIPv6Ranges` — verify existing ranges still work (regression check)

  **Must NOT do**:
  - Do NOT modify any other SSRF logic
  - Do NOT add DNS rebinding protection (S11 — deferred)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 1-line addition + small test
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7, 9)
  - **Blocks**: F1-F4
  - **Blocked By**: None

  **References**:
  - `bridge/internal/skills/ssrf.go:22-36` — The CIDR list to extend

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Multicast blocked
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v -run "TestSSRF_IPv6Multicast" ./internal/skills/...
      2. Assert: PASS
    Expected Result: ff00::1 blocked by SSRF validator
    Evidence: .sisyphus/evidence/task-8-multicast-blocked.txt
  ```

  **Commit**: YES
  - Message: `fix(ssrf): add ff00::/8 multicast CIDR (S6 remaining)`
  - Files: `bridge/internal/skills/ssrf.go`, `bridge/internal/skills/ssrf_test.go`
  - Pre-commit: `cd bridge && go test -v ./internal/skills/...`

- [x] 9. Expand Policy + Registry Test Coverage

  **What to do**:
  - Create `bridge/internal/skills/policy_test.go`:
    - Test `TestPolicyEnforcer_AllowExplicit` — explicitly allowed skill passes
    - Test `TestPolicyEnforcer_BlockExplicit` — explicitly blocked skill denied
    - Test `TestPolicyEnforcer_DenyUnknown` — unknown skill denied (S2 confirmation)
    - Test `TestPolicyEnforcer_AllowOverridesBlock` — allow takes precedence over block
    - Test `TestPolicyEnforcer_AllSkillPoliciesHaveRisk` — verify every entry in Policy map has a non-empty Risk level
  - Create `bridge/internal/skills/registry_test.go`:
    - Test `TestRegistry_ScanSkills` — verify ScanSkills populates the registry
    - Test `TestRegistry_LookupByName` — verify registered skills are findable
    - Test `TestRegistry_DomainExtraction` — verify domain extracted correctly from skill names
    - Test `TestRegistry_RiskAssignment` — verify risk levels match domains

  **Must NOT do**:
  - Do NOT add tests for individual skill handlers (web_search, email, etc.) — defer to separate phase
  - Do NOT modify production code

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Test writing with understanding of policy/registry architecture
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (needs Tasks 1 and 7 complete for baseline)
  - **Parallel Group**: Wave 2 (with Tasks 7-8)
  - **Blocks**: F1-F4
  - **Blocked By**: Tasks 1, 7

  **References**:

  **Pattern References**:
  - `bridge/internal/skills/executor_authorizer_test.go` — Existing test showing conventions
  - `bridge/internal/skills/policy.go` — Policy definitions to test against
  - `bridge/internal/skills/registry.go:38-83` — Registry functions to test

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All new tests pass
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v -run "TestPolicyEnforcer|TestRegistry" ./internal/skills/...
      2. Assert: All tests PASS
    Expected Result: ~9 tests PASS covering policy and registry
    Evidence: .sisyphus/evidence/task-9-policy-registry-tests.txt
  ```

  **Commit**: YES
  - Message: `test(skills): expand policy and registry test coverage`
  - Files: `bridge/internal/skills/policy_test.go`, `bridge/internal/skills/registry_test.go`
  - Pre-commit: `cd bridge && go test -v ./internal/skills/...`

---

## Final Verification Wave

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in `.sisyphus/evidence/`. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./internal/skills/...` + `go build -o /dev/null ./cmd/bridge`. Review all changed files for: `as any`/`@ts-ignore` equivalents, empty catches, commented-out code, unused imports. Check for AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Run every QA scenario from every task. Save evidence to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff. Verify 1:1 — everything in spec was built, nothing beyond spec. Check "Must NOT do" compliance. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | VERDICT`

---

## Commit Strategy

- **1**: `test(skills): add confirmation tests for already-fixed findings S2,S3,S6,S7,S8,F1`
- **2**: `fix(skills): replace blanket dangerousChars with context-aware validation (S1)`
- **3**: `fix(deploy): pin curl|bash to release tag with integrity check (S4)`
- **4**: `fix(status): replace return 0 with exit 0 (S5)`
- **5**: `fix(provision): quote sudo commands and change to confirm automation (S16)`
- **6**: `fix(skills): add default timeout and fix timeout context detection (S12,S13)`
- **7**: `fix(skills): complete parseSkillFile field extraction for timeout/version/enabled (F3)`
- **8**: `fix(ssrf): add ff00::/8 multicast CIDR (S6 remaining)`
- **9**: `test(skills): expand policy and registry test coverage`

---

## Success Criteria

### Verification Commands
```bash
cd bridge && go test -v ./internal/skills/...     # Expected: ALL PASS, ~30 functions
cd bridge && go build -o /dev/null ./cmd/bridge    # Expected: clean build
bash tests/test-deployment-skills.sh               # Expected: ALL PASS
grep 'return 0' .skills/status.yaml                # Expected: no matches
grep 'raw.githubusercontent.*main' .skills/deploy.yaml  # Expected: no matches
grep 'automation: "confirm"' .skills/provision.yaml      # Expected: matches on sudo steps
```

### Final Checklist
- [x] All "Must Have" present
- [x] All "Must NOT Have" absent
- [x] All tests pass
- [x] No regression on already-fixed findings
