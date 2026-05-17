# Codebase Review Fixes — Comprehensive Bug, Security & Quality Plan

## TL;DR

> **Quick Summary**: Fix all critical bugs, security vulnerabilities, and code quality issues discovered during the comprehensive codebase review. Includes production panic removal, weak randomness hardening, self-assignment bug fix, E2EE crypto implementation, god object refactoring, test coverage for untested packages, and provider type consolidation.
> 
> **Deliverables**:
> - Fixed production panics (8 instances → proper error returns)
> - Hardened token generation (crypto/rand replaces time.Now())
> - Fixed self-assignment bug in rolodex.go
> - Consolidated 5 duplicate provider type definitions into single source of truth
> - Removed 4 dead functions in executor/engine.go
> - Fixed insecure Docker chmod 666 recommendations in deploy scripts
> - Refactored 3 god objects (main.go, matrix.go, keystore.go) into smaller modules
> - Implemented core E2EE Olm/Megolm methods in ArmorChat Android
> - Added tests for 4 critical untested packages (recovery, provisioning, http, skills)
> - Removed 1 TypeScript `as any` instance in browser-service
> 
> **Estimated Effort**: Large (30-40 tasks across 6 waves)
> **Parallel Execution**: YES — 6 waves
> **Critical Path**: P0 Security Fixes → Provider Consolidation → God Object Refactoring (matrix.go) → E2EE Implementation → Tests

---

## Context

### Original Request
Comprehensive codebase review of the ArmorClaw project (Go bridge + Matrix Conduit + SQLCipher keystore + Android ArmorChat + browser-service TypeScript + Python bridge_client) followed by a work plan to fix all discovered issues.

### Interview Summary
**Key Discussions**:
- User wants ALL review findings addressed in one comprehensive plan (P0 through P2)
- God object refactoring: included (main.go 3314 lines, matrix.go 1899 lines, keystore.go 1440 lines)
- E2EE crypto implementation: included (11 Olm/Megolm TODO stubs in ArmorChat)
- Test coverage for untested packages: included (recovery, provisioning, docker, http, skills)
- TypeScript cleanup: included (1 `as any` instance found — original review overstated at 228)

**Research Findings** (verified against codebase):
- **MD5 usage**: NOT FOUND — original review was incorrect, removed from plan
- **Dead code**: CONFIRMED — 4 unused functions at engine.go:291-337
- **Python errors**: bridge_client.py compiles clean — removed from plan
- **TypeScript `as any`**: Only 1 instance (browser.ts) — not 228 as originally reported
- **Provider type duplication**: CONFIRMED — 5 separate type definitions across keystore.go, providers/registry.go, secrets/injection.go, sso/sso.go, internal/ai/types.go
- **Production panics**: CONFIRMED — 8 across enforcement.go (2), securerandom.go (4), notifier.go (1), compliance.go (1)
- **Weak randomness**: CONFIRMED — rpc/server.go:682 uses time.Now().Nanosecond()

### Metis Review
**Identified Gaps** (addressed):
- Verified all findings against actual codebase — removed 3 false positives (MD5, Python errors, TypeScript scale)
- Containerd/firecracker adapters: EXCLUDED (explicitly marked as v5.0 future work in codebase)
- E2EE split into core methods (5 blocking) and enhancement methods (10 remaining)
- God object refactoring: incremental extraction with tests after each
- Rollback strategy: each god object extraction is a separate task with verification
- Test coverage targets: minimum 60% for each package (unit tests)

---

## Work Objectives

### Core Objective
Eliminate all critical bugs and security vulnerabilities, consolidate architectural debt (duplicate types, god objects), implement missing E2EE crypto, and establish test coverage for critical untested packages.

### Concrete Deliverables
- 8 production panic sites replaced with proper error handling
- 1 weak randomness source replaced with crypto/rand
- 1 self-assignment bug fixed in rolodex.go
- 5 provider type definitions consolidated to 1 canonical source
- 4 dead functions removed from engine.go
- 2 insecure chmod 666 recommendations fixed in deploy scripts
- 3 god objects split into smaller, focused modules (<500 lines each)
- 11 E2EE Olm/Megolm methods implemented in ArmorChat Android
- 4 critical packages given test coverage (recovery, provisioning, http, skills)
- 1 TypeScript `as any` removed from browser-service

### Definition of Done
- [ ] `go build ./...` compiles with zero errors
- [ ] `go test ./...` passes with no failures
- [ ] `go vet ./...` reports no issues
- [ ] Zero `panic()` calls in production request-handling paths
- [ ] Zero `time.Now()` usage for security-sensitive token generation
- [ ] All god objects split to <500 lines per file
- [ ] E2EE methods compile and pass basic unit tests in Android project

### Must Have
- All 8 production panics replaced with proper error returns or logging
- crypto/rand used for all token/session generation
- Provider types consolidated to single source of truth (providers/registry.go)
- Existing `go test ./...` must continue to pass after all changes
- No SQLCipher removal or Matrix bypass (AGENTS.md constraints)
- Prefer minimal patches over rewrites

### Must NOT Have (Guardrails)
- No removal of SQLCipher encryption
- No bypassing Matrix as control plane
- No weakening of approval flow for payments or critical PII
- No direct production secret access introduced
- No containerd/firecracker adapter implementation (deferred to v5.0)
- No `panic()` in any new code for request-handling paths
- No `time.Now().Nanosecond()` for security-sensitive random values
- No new `as any` type assertions in TypeScript
- No god objects >500 lines created as a result of refactoring

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (Go `go test`, 10 test files with ~31K lines)
- **Automated tests**: Tests-after (add tests alongside fixes, not TDD)
- **Framework**: Go `go test` for bridge; `./gradlew test` for Android; Jest for browser-service
- **If TDD**: Not applicable — brownfield code, tests-after approach

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go bridge**: Use Bash (`go build`, `go test`, `go vet`) — compile, test, lint
- **Android ArmorChat**: Use Bash (`./gradlew test`, `./gradlew build`) — compile and test
- **TypeScript browser-service**: Use Bash (`npm test`, `npx tsc --noEmit`) — type check and test
- **Deploy scripts**: Use Bash (`bash -n`, `shellcheck`) — syntax validation

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — P0 security fixes, all independent):
├── Task 1: Fix self-assignment bug in rolodex.go [quick]
├── Task 2: Replace production panics with error returns [quick]
├── Task 3: Replace weak randomness with crypto/rand [quick]
├── Task 4: Remove dead code from engine.go [quick]
├── Task 5: Fix Docker chmod 666 recommendations [quick]
└── Task 6: Remove TypeScript `as any` in browser-service [quick]

Wave 2 (After Wave 1 — provider consolidation + keystore prep):
├── Task 7: Consolidate provider types to canonical registry [unspecified-high]
├── Task 8: Refactor keystore.go god object (1440 lines) [deep]
└── Task 9: Add tests for provisioning package [unspecified-high]

Wave 3 (After Wave 2 — matrix refactoring + more tests):
├── Task 10: Refactor matrix.go god object (1899 lines) [deep]
├── Task 11: Refactor main.go god object (3314 lines) [deep]
├── Task 12: Add tests for recovery package [unspecified-high]
└── Task 13: Add tests for http package [unspecified-high]

Wave 4 (After Wave 3 — E2EE implementation):
├── Task 14: Implement core E2EE methods (5 blocking Olm/Megolm) [deep]
├── Task 15: Implement remaining E2EE methods (6 enhancement) [deep]
├── Task 16: Add tests for skills package [unspecified-high]
└── Task 17: Add tests for docker/client.go [unspecified-high]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Full test suite run + real QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: T1-T6 (parallel) → T7 → T8 → T10 → T11 → T14 → T15 → F1-F4
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 6 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | — | — | 1 |
| 2 | — | — | 1 |
| 3 | — | — | 1 |
| 4 | — | — | 1 |
| 5 | — | — | 1 |
| 6 | — | — | 1 |
| 7 | — | 8 | 2 |
| 8 | 7 | — | 2 |
| 9 | — | — | 2 |
| 10 | 7, 8 | 11 | 3 |
| 11 | 10 | — | 3 |
| 12 | — | — | 3 |
| 13 | — | — | 3 |
| 14 | 10 | 15 | 4 |
| 15 | 14 | — | 4 |
| 16 | — | — | 4 |
| 17 | — | — | 4 |

### Agent Dispatch Summary

- **Wave 1**: **6 tasks** — T1-T6 all `quick` (independent P0 fixes)
- **Wave 2**: **3 tasks** — T7 `unspecified-high`, T8 `deep`, T9 `unspecified-high`
- **Wave 3**: **4 tasks** — T10 `deep`, T11 `deep`, T12-T13 `unspecified-high`
- **Wave 4**: **4 tasks** — T14 `deep`, T15 `deep`, T16-T17 `unspecified-high`
- **FINAL**: **4 tasks** — F1 `oracle`, F2-F3 `unspecified-high`, F4 `deep`

---

## TODOs

- [ ] 1. Fix self-assignment bug in rolodex.go

  **What to do**:
  - Open `bridge/pkg/secretary/rolodex.go` line 216
  - The line `existing.Name = existing.Name` is a no-op self-assignment
  - Determine the correct behavior by reading surrounding context (likely should be `existing.Name = contact.Name` to copy the new contact's name)
  - Fix the assignment to use the correct source variable
  - Verify no other self-assignment patterns exist in the same file

  **Must NOT do**:
  - Do not change any function signatures
  - Do not add new imports or dependencies

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single-line bug fix in one file
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `systematic-debugging`: Not needed — bug location and fix are clear

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3, 4, 5, 6)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/rolodex.go:200-230` — The UpdateContact function context showing how `existing` and `contact` structs are used

  **API/Type References**:
  - `bridge/pkg/secretary/rolodex.go` — The Contact struct definition (search for `type Contact struct`)

  **WHY Each Reference Matters**:
  - The surrounding function context shows that `contact` is the input parameter with new values and `existing` is the record being updated. The fix should assign from `contact.Name` to `existing.Name`.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Self-assignment bug is fixed
    Tool: Bash (grep)
    Preconditions: Fix applied to rolodex.go
    Steps:
      1. Run: grep -n "existing.Name = existing.Name" bridge/pkg/secretary/rolodex.go
      2. Assert: no output (pattern no longer exists)
      3. Run: grep -n "existing.Name = contact.Name" bridge/pkg/secretary/rolodex.go
      4. Assert: output shows the corrected line
    Expected Result: Self-assignment eliminated, correct assignment present
    Failure Indicators: grep still finds self-assignment, or no replacement found
    Evidence: .sisyphus/evidence/task-1-self-assignment-fix.txt

  Scenario: Bridge compiles after fix
    Tool: Bash
    Preconditions: Fix applied
    Steps:
      1. Run: go build ./bridge/...
      2. Assert: exit code 0
    Expected Result: Clean compilation
    Evidence: .sisyphus/evidence/task-1-compile-check.txt
  ```

  **Commit**: YES
  - Message: `fix(rolodex): correct self-assignment bug in UpdateContact`
  - Files: `bridge/pkg/secretary/rolodex.go`
  - Pre-commit: `go build ./bridge/...`

- [ ] 2. Replace production panics with proper error handling

  **What to do**:
  - Find and replace all 8 `panic()` calls in production request-handling paths:
    - `bridge/pkg/enforcement/enforcement.go` — 2 panics (license checking)
    - `bridge/pkg/securerandom/random.go` — 4 panics (random generation failures)
    - `bridge/pkg/errors/notifier.go:27` — 1 panic (notification error)
    - `bridge/pkg/audit/compliance.go:56` — 1 panic (hash key generation)
  - For each panic, determine the appropriate replacement:
    - **Unrecoverable crypto init** (securerandom.go): Replace with `log.Fatalf()` or return error — these are genuinely unrecoverable but shouldn't crash the process mid-request
    - **License enforcement** (enforcement.go): Return structured error via the bridge's error system (errsys)
    - **Notification** (notifier.go): Log error and continue (non-critical path)
    - **Hash key** (compliance.go): Return error to caller
  - Read the existing error handling patterns in each file before making changes
  - Use the bridge's `errsys` package for structured errors where applicable

  **Must NOT do**:
  - Do NOT remove error checking — only change HOW errors are reported
  - Do NOT introduce `log.Fatal` in request-handling goroutines (use return error instead)
  - Do NOT change the behavior of crypto initialization — still fail hard, just cleaner

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Multiple small replacements across 4 files, pattern is clear
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `systematic-debugging`: Not debugging, just replacing patterns

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3, 4, 5, 6)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `bridge/pkg/errors/` — The errsys package for structured error handling
  - `bridge/pkg/securerandom/random.go` — Current panic patterns for crypto failures
  - `bridge/pkg/enforcement/enforcement.go` — Current panic patterns for license checks

  **WHY Each Reference Matters**:
  - errsys shows the canonical error handling pattern used throughout the bridge
  - securerandom panics are in crypto init — must still fail hard but cleanly
  - enforcement panics need to return proper errors to callers

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: No panics in production paths
    Tool: Bash (grep)
    Preconditions: All replacements applied
    Steps:
      1. Run: grep -rn "panic(" bridge/pkg/enforcement/enforcement.go bridge/pkg/securerandom/random.go bridge/pkg/errors/notifier.go bridge/pkg/audit/compliance.go
      2. Assert: no output (zero panic calls remaining)
    Expected Result: Zero panic() calls in these 4 files
    Evidence: .sisyphus/evidence/task-2-no-panics.txt

  Scenario: Bridge compiles and tests pass
    Tool: Bash
    Preconditions: All replacements applied
    Steps:
      1. Run: go build ./bridge/...
      2. Assert: exit code 0
      3. Run: go vet ./bridge/pkg/enforcement/ ./bridge/pkg/securerandom/ ./bridge/pkg/errors/ ./bridge/pkg/audit/
      4. Assert: exit code 0
    Expected Result: Clean build and vet
    Evidence: .sisyphus/evidence/task-2-build-vet.txt
  ```

  **Commit**: YES
  - Message: `fix(security): replace production panics with proper error handling`
  - Files: `bridge/pkg/enforcement/enforcement.go`, `bridge/pkg/securerandom/random.go`, `bridge/pkg/errors/notifier.go`, `bridge/pkg/audit/compliance.go`
  - Pre-commit: `go build ./bridge/... && go vet ./bridge/...`

- [ ] 3. Replace weak randomness with crypto/rand

  **What to do**:
  - Open `bridge/pkg/rpc/server.go` line 682
  - Find the token generation using `time.Now().Nanosecond()` — this is predictable and insecure
  - Replace with `crypto/rand` based generation. Use `hex.EncodeToString()` to create a random token string
  - Pattern: create a byte slice, fill with `crypto/rand.Read()`, encode to hex
  - Example: `buf := make([]byte, 16); _, _ = rand.Read(buf); token := hex.EncodeToString(buf)`
  - Check if `crypto/rand` is already imported; add if not
  - Search the entire bridge codebase for any other `time.Now()` usage in token/session/auth generation contexts

  **Must NOT do**:
  - Do not change non-security uses of `time.Now()` (logging, timestamps, etc.)
  - Do not change function signatures or return types
  - Do not add external dependencies

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Small crypto replacement in one file
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `systematic-debugging`: Not debugging, pattern replacement

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 4, 5, 6)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/server.go:680-690` — The current token generation code
  - `bridge/pkg/securerandom/random.go` — The project's own secure random utility (may already have a function to reuse)

  **WHY Each Reference Matters**:
  - server.go:680-690 shows the exact line to fix
  - securerandom/random.go may already have a secure random function — prefer reusing it over inline crypto/rand

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Weak randomness eliminated
    Tool: Bash (grep)
    Preconditions: Fix applied
    Steps:
      1. Run: grep -n "time.Now().Nanosecond()" bridge/pkg/rpc/server.go
      2. Assert: no output (weak pattern removed)
      3. Run: grep -n "crypto/rand\|securerandom" bridge/pkg/rpc/server.go
      4. Assert: output shows crypto/rand import or securerandom usage
    Expected Result: time.Now().Nanosecond() gone, crypto/rand present
    Evidence: .sisyphus/evidence/task-3-weak-random-fix.txt

  Scenario: No other weak randomness in security contexts
    Tool: Bash (grep)
    Preconditions: Fix applied
    Steps:
      1. Run: grep -rn "time.Now().Nanosecond()\|time.Now().UnixNano()" bridge/ --include="*.go" | grep -i "token\|session\|auth\|secret\|key\|nonce"
      2. Assert: no output (no security-sensitive time-based randomness)
    Expected Result: No weak randomness in any security context
    Evidence: .sisyphus/evidence/task-3-no-other-weak-random.txt

  Scenario: Bridge compiles after change
    Tool: Bash
    Preconditions: Fix applied
    Steps:
      1. Run: go build ./bridge/pkg/rpc/...
      2. Assert: exit code 0
    Expected Result: Clean compilation
    Evidence: .sisyphus/evidence/task-3-compile.txt
  ```

  **Commit**: YES
  - Message: `fix(security): replace weak randomness with crypto/rand for token generation`
  - Files: `bridge/pkg/rpc/server.go`
  - Pre-commit: `go build ./bridge/pkg/rpc/...`

- [ ] 4. Remove dead code from engine.go

  **What to do**:
  - Open `bridge/internal/executor/engine.go` lines 291-337
  - Remove 4 unused functions:
    - `isValidJSON(s string) bool` (line 291)
    - `truncateOutput(output string, maxLen int) string` (line 296)
    - `parseCommand(cmdStr string) []string` (line 303)
    - `readOutput(r io.Reader, maxBytes int64) (string, error)` (line 337)
  - Before removing, verify these functions are truly unused by running `grep -rn "isValidJSON\|truncateOutput\|parseCommand\|readOutput" bridge/` — confirm no call sites exist outside their own definitions
  - Remove the functions and any associated unused imports (e.g., `strings`, `encoding/json`, `io` if no longer needed)
  - Run `go build` and `go test` after removal to confirm nothing breaks

  **Must NOT do**:
  - Do not remove any other functions
  - Do not refactor the remaining engine.go code
  - Do not change any function signatures

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Remove 4 clearly unused functions
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3, 5, 6)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `bridge/internal/executor/engine.go:291-337` — The 4 unused functions to remove

  **WHY Each Reference Matters**:
  - These are the exact lines containing the dead code

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Dead functions removed
    Tool: Bash (grep)
    Preconditions: Functions removed
    Steps:
      1. Run: grep -n "func isValidJSON\|func truncateOutput\|func parseCommand\|func readOutput" bridge/internal/executor/engine.go
      2. Assert: no output (all 4 functions removed)
    Expected Result: Zero dead functions
    Evidence: .sisyphus/evidence/task-4-dead-code-removed.txt

  Scenario: Build and tests pass
    Tool: Bash
    Preconditions: Functions removed
    Steps:
      1. Run: go build ./bridge/internal/executor/...
      2. Assert: exit code 0
      3. Run: go test ./bridge/internal/executor/... 2>&1
      4. Assert: exit code 0 (or no test files found is acceptable)
    Expected Result: Clean compilation
    Evidence: .sisyphus/evidence/task-4-build-test.txt
  ```

  **Commit**: YES
  - Message: `refactor(executor): remove unused helper functions`
  - Files: `bridge/internal/executor/engine.go`
  - Pre-commit: `go build ./bridge/internal/executor/...`

- [ ] 5. Fix Docker chmod 666 recommendations in deploy scripts

  **What to do**:
  - Open `deploy/container-setup.sh` lines 261 and 1100
  - Find the `chmod 666` recommendations for Docker socket permissions
  - Replace with the secure alternative: `sudo usermod -aG docker $USER` (adds user to docker group)
  - Update the error message/suggestion text to recommend the group-based approach
  - Also check `deploy/setup-quick.sh` line 1339 for any similar issues
  - Verify the scripts still pass syntax check with `bash -n`

  **Must NOT do**:
  - Do not change the actual Docker socket permissions logic
  - Do not remove error handling for permission denied scenarios
  - Do not change any other deploy script behavior

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Text replacement in deploy scripts
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3, 4, 6)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `deploy/container-setup.sh:255-265` — First chmod 666 recommendation context
  - `deploy/container-setup.sh:1095-1105` — Second chmod 666 recommendation context

  **WHY Each Reference Matters**:
  - These are the exact locations with insecure permission recommendations

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: No chmod 666 recommendations
    Tool: Bash (grep)
    Preconditions: Recommendations updated
    Steps:
      1. Run: grep -n "chmod 666" deploy/container-setup.sh deploy/setup-quick.sh
      2. Assert: no output (no chmod 666 in any deploy script)
      3. Run: grep -n "usermod -aG docker" deploy/container-setup.sh
      4. Assert: output shows the secure alternative
    Expected Result: Insecure recommendation gone, secure alternative present
    Evidence: .sisyphus/evidence/task-5-chmod-fix.txt

  Scenario: Scripts pass syntax check
    Tool: Bash
    Preconditions: Changes applied
    Steps:
      1. Run: bash -n deploy/container-setup.sh && echo "PASS" || echo "FAIL"
      2. Assert: output contains "PASS"
    Expected Result: No syntax errors
    Evidence: .sisyphus/evidence/task-5-syntax-check.txt
  ```

  **Commit**: YES
  - Message: `fix(deploy): replace chmod 666 recommendation with secure docker group alternative`
  - Files: `deploy/container-setup.sh`, `deploy/setup-quick.sh`
  - Pre-commit: `bash -n deploy/container-setup.sh && bash -n deploy/setup-quick.sh`

- [ ] 6. Remove TypeScript `as any` and fix Python bridge_client errors

  **What to do**:

  **TypeScript (browser-service):**
  - Open `browser-service/src/browser.ts` — find the 1 `as any` instance
  - Replace with proper type annotation or type guard
  - Run `npx tsc --noEmit` to verify type safety

  **Python (container/openclaw/bridge_client.py):**
  - Fix 19 Pyright errors identified by LSP:
    - 3x `"response" is possibly unbound` (lines 103, 104, 109) — initialize response before try block or add else clause
    - 3x `"response" is possibly unbound` (lines 460, 461, 466) — same pattern
    - 2x `Dict[str, str]` assigned to `str` parameter (lines 315, 566) — fix type mismatch
    - 1x `Dict[str, Any]` assigned to `str` (line 647) — fix type mismatch
    - 2x `List[Dict]` assigned to `str` (lines 926, 928) — fix type mismatch
    - 3x `int` assigned to `str` (lines 962, 964, 966) — convert to str()
    - 1x `Dict[str, Any]` assigned to `str` (line 1016) — fix type mismatch
    - 2x `CoroutineType` returned instead of `await` result (lines 1018, 1022) — add `await`
  - Run `python3 -m py_compile bridge_client.py` to verify syntax

  **Must NOT do**:
  - Do not change any runtime behavior — only type fixes
  - Do not refactor the Python code structure
  - Do not add new dependencies

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Type fixes in 2 files
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3, 4, 5)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `browser-service/src/browser.ts` — The TypeScript file with `as any`
  - `container/openclaw/bridge_client.py` — The Python file with 19 Pyright errors

  **WHY Each Reference Matters**:
  - These are the exact files needing type fixes

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: TypeScript has no `as any`
    Tool: Bash (grep)
    Preconditions: Fix applied
    Steps:
      1. Run: grep -rn "as any" browser-service/src/
      2. Assert: no output
    Expected Result: Zero `as any` instances
    Evidence: .sisyphus/evidence/task-6-ts-no-any.txt

  Scenario: Python compiles clean
    Tool: Bash
    Preconditions: Fixes applied
    Steps:
      1. Run: cd container/openclaw && python3 -m py_compile bridge_client.py && echo "PASS" || echo "FAIL"
      2. Assert: output contains "PASS"
    Expected Result: Clean Python compilation
    Evidence: .sisyphus/evidence/task-6-python-compile.txt
  ```

  **Commit**: YES
  - Message: `fix(types): remove `as any` in browser.ts and fix Pyright errors in bridge_client.py`
  - Files: `browser-service/src/browser.ts`, `container/openclaw/bridge_client.py`
  - Pre-commit: `grep -rn "as any" browser-service/src/ && cd container/openclaw && python3 -m py_compile bridge_client.py`

- [ ] 7. Consolidate provider types to canonical registry

  **What to do**:
  - The project has 5 separate provider type definitions:
    - `bridge/pkg/providers/registry.go:10` — `type Provider struct` (source of truth, most complete)
    - `bridge/pkg/keystore/keystore.go:60` — `type Provider string` (12 constants)
    - `bridge/pkg/secrets/injection.go:30` — `type SecretProvider string` (6 constants, outdated)
    - `bridge/pkg/sso/sso.go:28` — `type ProviderType string`
    - `bridge/internal/ai/types.go:5` — `type ProviderType string` (12 constants)
  - Make `providers/registry.go` the single source of truth for provider definitions
  - Update all other files to import from `providers/registry.go` instead of defining their own types
  - Remove duplicate type definitions and constants from keystore.go, secrets/injection.go, sso/sso.go, ai/types.go
  - Ensure all 12 providers are represented: openai, anthropic, zhipu, deepseek, moonshot, google, xai, openrouter, groq, nvidia, cloudflare, ollama
  - Run `go build ./bridge/...` and `go test ./bridge/...` to verify nothing breaks

  **Must NOT do**:
  - Do not change the Provider struct fields in registry.go
  - Do not remove any provider from the registry
  - Do not change the public API (RPC methods, exported function signatures)
  - Do not rename exported types that external code might depend on

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multi-file refactoring touching type system, needs careful dependency management
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (within Wave 2)
  - **Parallel Group**: Wave 2 (with Tasks 8, 9)
  - **Blocks**: Task 8 (keystore refactoring needs clean provider types first)
  - **Blocked By**: None (can start immediately, but T8 depends on it)

  **References**:

  **Pattern References**:
  - `bridge/pkg/providers/registry.go` — Canonical provider registry (source of truth)
  - `bridge/pkg/keystore/keystore.go:60-75` — Provider type and 12 constants to consolidate
  - `bridge/pkg/secrets/injection.go:30-39` — SecretProvider type (outdated, only 6 providers)
  - `bridge/pkg/sso/sso.go:28` — ProviderType for SSO
  - `bridge/internal/ai/types.go:5-20` — ProviderType for AI module

  **WHY Each Reference Matters**:
  - registry.go is the canonical source — all others should reference it
  - keystore.go was recently fixed to have 12 providers but still has its own type
  - secrets/injection.go is outdated with only 6 providers — critical to update
  - sso.go and ai/types.go each have their own ProviderType — redundant

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Single canonical provider type
    Tool: Bash (grep)
    Preconditions: Consolidation complete
    Steps:
      1. Run: grep -rn "type Provider struct\|type Provider string\|type SecretProvider string\|type ProviderType string" bridge/ --include="*.go"
      2. Assert: only `providers/registry.go` defines the canonical type
      3. Assert: other files use imports from providers package
    Expected Result: Only one provider type definition in codebase
    Evidence: .sisyphus/evidence/task-7-provider-consolidation.txt

  Scenario: All 12 providers present in registry
    Tool: Bash (grep)
    Preconditions: Consolidation complete
    Steps:
      1. Run: grep -c "Provider\|openai\|anthropic\|zhipu\|deepseek\|moonshot\|google\|xai\|openrouter\|groq\|nvidia\|cloudflare\|ollama" bridge/pkg/providers/registry.go
      2. Assert: all 12 provider names present
    Expected Result: Complete provider coverage
    Evidence: .sisyphus/evidence/task-7-all-providers.txt

  Scenario: Build and tests pass
    Tool: Bash
    Preconditions: Consolidation complete
    Steps:
      1. Run: go build ./bridge/...
      2. Assert: exit code 0
      3. Run: go test ./bridge/... 2>&1 | tail -5
      4. Assert: no test failures
    Expected Result: Full build and test pass
    Evidence: .sisyphus/evidence/task-7-build-test.txt
  ```

  **Commit**: YES
  - Message: `refactor(providers): consolidate provider types to canonical registry`
  - Files: `bridge/pkg/providers/registry.go`, `bridge/pkg/keystore/keystore.go`, `bridge/pkg/secrets/injection.go`, `bridge/pkg/sso/sso.go`, `bridge/internal/ai/types.go`
  - Pre-commit: `go build ./bridge/... && go test ./bridge/...`

- [ ] 8. Refactor keystore.go god object (1440 lines)

  **What to do**:
  - Split `bridge/pkg/keystore/keystore.go` (1440 lines) into a package of focused modules
  - Analyze the file structure and identify logical groupings (likely: storage operations, key management, provider validation, crypto operations)
  - Create sub-files within `bridge/pkg/keystore/`:
    - `keystore.go` — Core types, interface, and NewKeystore constructor (keep as entry point)
    - `storage.go` — SQLCipher database operations (CRUD)
    - `providers.go` — Provider validation and lookup (after Task 7 consolidation)
    - `crypto.go` — Encryption/decryption operations
  - Move functions to appropriate files while keeping the `Keystore` struct and its methods together
  - Ensure the package API remains identical (same exported types, functions, methods)
  - Each new file should be <400 lines
  - Run `go build ./bridge/pkg/keystore/...` and `go test ./bridge/pkg/keystore/...` to verify

  **Must NOT do**:
  - Do not change the public API of the keystore package
  - Do not move types or functions to different packages
  - Do not change SQLCipher usage or database schema
  - Do not remove any existing functionality

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Large file refactoring (1440 lines) with dependency management
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 10 (matrix.go refactoring may need clean keystore patterns)
  - **Blocked By**: Task 7 (provider types must be consolidated first)

  **References**:

  **Pattern References**:
  - `bridge/pkg/keystore/keystore.go` — The god object to split (1440 lines)
  - `bridge/pkg/providers/registry.go` — Canonical provider types (after Task 7)

  **WHY Each Reference Matters**:
  - keystore.go is the file being refactored — need to understand its full structure before splitting
  - registry.go will be used after Task 7 for provider validation

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: No file exceeds 400 lines
    Tool: Bash (wc)
    Preconditions: Refactoring complete
    Steps:
      1. Run: wc -l bridge/pkg/keystore/*.go
      2. Assert: no file exceeds 400 lines
    Expected Result: All files under 400 lines
    Evidence: .sisyphus/evidence/task-8-file-sizes.txt

  Scenario: Package API unchanged
    Tool: Bash
    Preconditions: Refactoring complete
    Steps:
      1. Run: go build ./bridge/...
      2. Assert: exit code 0 (no broken imports)
      3. Run: go test ./bridge/pkg/keystore/... 2>&1
      4. Assert: all existing tests pass
    Expected Result: Full backward compatibility
    Evidence: .sisyphus/evidence/task-8-api-compat.txt
  ```

  **Commit**: YES
  - Message: `refactor(keystore): split god object into focused modules`
  - Files: `bridge/pkg/keystore/keystore.go`, `bridge/pkg/keystore/storage.go`, `bridge/pkg/keystore/providers.go`, `bridge/pkg/keystore/crypto.go`
  - Pre-commit: `go build ./bridge/... && go test ./bridge/pkg/keystore/...`

- [ ] 9. Add tests for provisioning package

  **What to do**:
  - Add unit tests for `bridge/pkg/provisioning/` (635 lines, currently 0 tests)
  - Read the provisioning package to understand its public API and responsibilities
  - Write tests covering:
    - Agent creation flow (happy path)
    - Agent deletion flow
    - Resource allocation and limits
    - Error cases (invalid input, missing dependencies, duplicate agents)
    - Configuration validation
  - Use table-driven tests following Go conventions
  - Mock external dependencies (Docker client, database) using interfaces
  - Target: minimum 60% line coverage
  - Run `go test -cover ./bridge/pkg/provisioning/...` to verify

  **Must NOT do**:
  - Do not change any production code in the provisioning package
  - Do not add new dependencies (use standard library mocking or existing test utilities)
  - Do not write integration tests that require Docker running

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Writing tests for existing untested package, needs understanding of the domain
  - **Skills**: [`test-driven-development`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7, 8)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `bridge/pkg/provisioning/` — The package to test (read all files)
  - `bridge/pkg/rpc/server_test.go` — Existing test patterns in the project (table-driven, mock setup)

  **Test References**:
  - Any existing `*_test.go` files in `bridge/pkg/` for assertion style and mock patterns

  **WHY Each Reference Matters**:
  - provisioning package needs full reading to understand what to test
  - rpc/server_test.go shows the project's testing conventions

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Tests exist and pass
    Tool: Bash
    Preconditions: Tests written
    Steps:
      1. Run: go test -v ./bridge/pkg/provisioning/... 2>&1
      2. Assert: at least 5 test functions (TestXxx) found
      3. Assert: all tests pass
    Expected Result: Tests pass with good coverage
    Evidence: .sisyphus/evidence/task-9-provisioning-tests.txt

  Scenario: Coverage target met
    Tool: Bash
    Preconditions: Tests written
    Steps:
      1. Run: go test -cover ./bridge/pkg/provisioning/...
      2. Assert: coverage >= 60%
    Expected Result: Minimum 60% line coverage
    Evidence: .sisyphus/evidence/task-9-coverage.txt
  ```

  **Commit**: YES
  - Message: `test(provisioning): add unit tests for provisioning package`
  - Files: `bridge/pkg/provisioning/*_test.go`
  - Pre-commit: `go test ./bridge/pkg/provisioning/...`

- [ ] 10. Refactor matrix.go god object (1899 lines)

  **What to do**:
  - Split `bridge/internal/adapter/matrix.go` (1899 lines) into focused modules
  - Analyze the file structure and identify logical groupings (likely: Matrix client/connection, login/auth, room management, event handling, message sending, sync)
  - Create sub-files within `bridge/internal/adapter/`:
    - `matrix.go` — Core MatrixAdapter struct, interface, NewMatrixAdapter constructor (keep as entry point)
    - `matrix_client.go` — HTTP client setup, connection management
    - `matrix_auth.go` — Login, logout, token refresh, registration
    - `matrix_rooms.go` — Room creation, joining, leaving, listing
    - `matrix_events.go` — Event handling, sync, incoming message processing
    - `matrix_messages.go` — Message sending, formatting, encryption
  - Each new file should be <400 lines
  - Ensure the package API remains identical (same exported types, functions, methods)
  - This file handles Matrix integration — critical path per AGENTS.md ("Do not bypass Matrix as control plane")
  - Run `go build ./bridge/...` and `go test ./bridge/internal/adapter/...` to verify

  **Must NOT do**:
  - Do not change the MatrixAdapter public API
  - Do not change how Matrix sync works
  - Do not bypass Matrix as control plane
  - Do not remove any existing functionality

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Large file refactoring (1899 lines), critical Matrix integration code
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 11 (main.go may reference matrix adapter patterns), Task 14 (E2EE needs clean matrix module)
  - **Blocked By**: Task 7 (provider consolidation), Task 8 (keystore patterns to follow)

  **References**:

  **Pattern References**:
  - `bridge/internal/adapter/matrix.go` — The god object to split (1899 lines)
  - `bridge/pkg/keystore/` — Refactored keystore from Task 8 (pattern to follow)

  **WHY Each Reference Matters**:
  - matrix.go is the file being refactored — need full understanding
  - keystore refactoring from Task 8 provides the pattern for splitting into sub-files

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: No file exceeds 400 lines
    Tool: Bash (wc)
    Preconditions: Refactoring complete
    Steps:
      1. Run: wc -l bridge/internal/adapter/matrix*.go
      2. Assert: no file exceeds 400 lines
    Expected Result: All files under 400 lines
    Evidence: .sisyphus/evidence/task-10-file-sizes.txt

  Scenario: Full build passes (critical — Matrix integration)
    Tool: Bash
    Preconditions: Refactoring complete
    Steps:
      1. Run: go build ./bridge/...
      2. Assert: exit code 0
      3. Run: go test ./bridge/internal/adapter/... 2>&1
      4. Assert: all tests pass
    Expected Result: Zero breakage across entire bridge
    Evidence: .sisyphus/evidence/task-10-build-test.txt

  Scenario: MatrixAdapter still exported correctly
    Tool: Bash
    Preconditions: Refactoring complete
    Steps:
      1. Run: grep -rn "type MatrixAdapter" bridge/internal/adapter/
      2. Assert: MatrixAdapter type found and exported
    Expected Result: Public API preserved
    Evidence: .sisyphus/evidence/task-10-api-preserved.txt
  ```

  **Commit**: YES
  - Message: `refactor(matrix): split adapter god object into focused modules`
  - Files: `bridge/internal/adapter/matrix*.go`
  - Pre-commit: `go build ./bridge/... && go test ./bridge/internal/adapter/...`

- [ ] 11. Refactor main.go god object (3314 lines)

  **What to do**:
  - Split `bridge/cmd/bridge/main.go` (3314 lines) into subcommand packages
  - Analyze the file structure and identify logical groupings (likely: config loading, server startup, CLI commands, signal handling, middleware setup)
  - Create sub-files within `bridge/cmd/bridge/`:
    - `main.go` — Entry point only (func main), config loading, and orchestration (<200 lines)
    - `config.go` — Configuration loading, validation, defaults
    - `server.go` — HTTP/RPC server setup and startup
    - `commands.go` — CLI command handlers (if any)
    - `middleware.go` — Middleware registration and setup
    - `signals.go` — Signal handling and graceful shutdown
  - Each new file should be <500 lines
  - Keep `main.go` as the slim entry point
  - Run `go build ./bridge/cmd/bridge/...` and `go test ./bridge/...` to verify

  **Must NOT do**:
  - Do not change the binary entry point behavior
  - Do not change configuration options or environment variable handling
  - Do not change server startup sequence

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Largest god object (3314 lines), bridge entry point
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3
  - **Blocks**: None (final god object refactoring)
  - **Blocked By**: Task 10 (follow matrix.go refactoring pattern)

  **References**:

  **Pattern References**:
  - `bridge/cmd/bridge/main.go` — The god object to split (3314 lines)
  - `bridge/internal/adapter/matrix*.go` — Refactored matrix from Task 10 (pattern to follow)

  **WHY Each Reference Matters**:
  - main.go is the file being refactored
  - matrix.go refactoring provides the established pattern

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: main.go is now a slim entry point
    Tool: Bash (wc)
    Preconditions: Refactoring complete
    Steps:
      1. Run: wc -l bridge/cmd/bridge/main.go
      2. Assert: < 200 lines
      3. Run: wc -l bridge/cmd/bridge/*.go
      4. Assert: no file exceeds 500 lines
    Expected Result: main.go under 200 lines, all files under 500
    Evidence: .sisyphus/evidence/task-11-file-sizes.txt

  Scenario: Binary builds and runs
    Tool: Bash
    Preconditions: Refactoring complete
    Steps:
      1. Run: go build -o /tmp/armorclaw-test ./bridge/cmd/bridge/
      2. Assert: exit code 0
      3. Run: go test ./bridge/... 2>&1 | tail -5
      4. Assert: no test failures
    Expected Result: Binary compiles, all tests pass
    Evidence: .sisyphus/evidence/task-11-build-test.txt
  ```

  **Commit**: YES
  - Message: `refactor(cmd): split main.go into focused subcommand modules`
  - Files: `bridge/cmd/bridge/main.go`, `bridge/cmd/bridge/config.go`, `bridge/cmd/bridge/server.go`, `bridge/cmd/bridge/middleware.go`, `bridge/cmd/bridge/signals.go`
  - Pre-commit: `go build ./bridge/... && go test ./bridge/...`

- [ ] 12. Add tests for recovery package

  **What to do**:
  - Add unit tests for `bridge/pkg/recovery/` (678 lines, currently 0 tests)
  - Read the recovery package to understand its public API (likely handles agent recovery, state restoration, crash recovery)
  - Write tests covering:
    - Recovery flow happy path
    - State restoration after failure
    - Recovery from partial/corrupted state
    - Error handling (missing state, invalid state format)
  - Use table-driven tests, mock external dependencies
  - Target: minimum 60% line coverage

  **Must NOT do**:
  - Do not change production code in recovery package
  - Do not write integration tests requiring Docker

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Writing tests for untested critical package
  - **Skills**: [`test-driven-development`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 10, 11, 13)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/recovery/` — The package to test (read all files)
  - `bridge/pkg/rpc/server_test.go` — Existing test patterns

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Tests exist and pass with coverage
    Tool: Bash
    Steps:
      1. Run: go test -v -cover ./bridge/pkg/recovery/... 2>&1
      2. Assert: at least 5 test functions, all pass, coverage >= 60%
    Expected Result: Tests pass with 60%+ coverage
    Evidence: .sisyphus/evidence/task-12-recovery-tests.txt
  ```

  **Commit**: YES
  - Message: `test(recovery): add unit tests for recovery package`
  - Files: `bridge/pkg/recovery/*_test.go`
  - Pre-commit: `go test ./bridge/pkg/recovery/...`

- [ ] 13. Add tests for http package

  **What to do**:
  - Add unit tests for `bridge/pkg/http/` (1049 lines, currently 0 tests)
  - Read the http package to understand its public API (HTTP server, handlers, middleware)
  - Write tests covering:
    - Request routing (correct handler per endpoint)
    - Request validation and error responses
    - Authentication/authorization middleware
    - Error handling (404, 500, malformed requests)
  - Use `net/http/httptest` for handler testing
  - Target: minimum 60% line coverage

  **Must NOT do**:
  - Do not change production code in http package
  - Do not start real HTTP servers in tests

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Writing tests for HTTP server package
  - **Skills**: [`test-driven-development`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 10, 11, 12)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/http/server.go` — Main server implementation (1049 lines)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Tests exist and pass with coverage
    Tool: Bash
    Steps:
      1. Run: go test -v -cover ./bridge/pkg/http/... 2>&1
      2. Assert: at least 5 test functions, all pass, coverage >= 60%
    Expected Result: Tests pass with 60%+ coverage
    Evidence: .sisyphus/evidence/task-13-http-tests.txt
  ```

  **Commit**: YES
  - Message: `test(http): add unit tests for HTTP server package`
  - Files: `bridge/pkg/http/*_test.go`
  - Pre-commit: `go test ./bridge/pkg/http/...`

- [ ] 14. Implement core E2EE methods (5 blocking Olm/Megolm)

  **What to do**:
  - Open `applications/ArmorChat/app/src/main/java/app/armorclaw/crypto/MatrixOlmService.kt`
  - Implement the 5 most critical E2EE methods that block basic Matrix encryption:
    1. `createOutboundSession()` — Create Olm session for new 1:1 conversation
    2. `createInboundSession()` — Create Olm session from received pre-key message
    3. `encryptMessage()` — Encrypt message using Olm session
    4. `decryptMessage()` — Decrypt received Olm-encrypted message
    5. `createMegolmSession()` — Create Megolm session for group conversations
  - Use the `org.matrix.olm` (OlmKit) library — check `build.gradle` for the dependency
  - Follow the Matrix E2EE specification for key exchange and session management
  - Store sessions securely in the Android KeyStore or SQLCipher database
  - Add unit tests with known test vectors from the Matrix spec
  - Run `./gradlew test` to verify

  **Must NOT do**:
  - Do not implement the remaining 6 enhancement methods (that's Task 15)
  - Do not change the MatrixOlmService public API/interface
  - Do not remove the TODO comments for unimplemented methods
  - Do not hardcode any encryption keys or secrets

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Crypto implementation requiring Matrix E2EE spec knowledge
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4
  - **Blocks**: Task 15 (remaining methods depend on core being implemented)
  - **Blocked By**: Task 10 (matrix.go refactoring should be done first to avoid conflicts)

  **References**:

  **Pattern References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/crypto/MatrixOlmService.kt` — The file with TODO stubs (15 TODOs)
  - `applications/ArmorChat/app/build.gradle` — Check for OlmKit dependency

  **External References**:
  - Matrix E2EE spec: https://spec.matrix.org/v1.2/client-server-api/#end-to-end-encryption
  - Olm library docs: https://matrix.org/docs/projects/other/olm

  **WHY Each Reference Matters**:
  - MatrixOlmService.kt shows the exact method signatures and TODO comments to replace
  - build.gradle confirms whether OlmKit is already a dependency

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Core E2EE methods compile
    Tool: Bash
    Preconditions: Implementation complete
    Steps:
      1. Run: cd applications/ArmorChat && ./gradlew compileDebugKotlin 2>&1 | tail -10
      2. Assert: BUILD SUCCESSFUL
    Expected Result: Kotlin compilation succeeds
    Evidence: .sisyphus/evidence/task-14-e2ee-compile.txt

  Scenario: TODO stubs replaced for core methods
    Tool: Bash (grep)
    Preconditions: Implementation complete
    Steps:
      1. Run: grep -n "TODO" applications/ArmorChat/app/src/main/java/app/armorclaw/crypto/MatrixOlmService.kt
      2. Assert: TODOs still present only for non-core methods (6 remaining), not for the 5 core methods
    Expected Result: Core method TODOs replaced, enhancement TODOs remain
    Evidence: .sisyphus/evidence/task-14-todo-check.txt

  Scenario: Unit tests pass
    Tool: Bash
    Preconditions: Tests written
    Steps:
      1. Run: cd applications/ArmorChat && ./gradlew test 2>&1 | tail -15
      2. Assert: all tests pass
    Expected Result: E2EE tests pass
    Evidence: .sisyphus/evidence/task-14-e2ee-tests.txt
  ```

  **Commit**: YES
  - Message: `feat(e2ee): implement core Olm/Megolm methods for Matrix encryption`
  - Files: `applications/ArmorChat/app/src/main/java/app/armorclaw/crypto/MatrixOlmService.kt`, `applications/ArmorChat/app/src/test/java/app/armorclaw/crypto/MatrixOlmServiceTest.kt`
  - Pre-commit: `cd applications/ArmorChat && ./gradlew test`

- [ ] 15. Implement remaining E2EE methods (6 enhancement)

  **What to do**:
  - Continue in `applications/ArmorChat/app/src/main/java/app/armorclaw/crypto/MatrixOlmService.kt`
  - Implement the remaining 6 E2EE methods:
    1. `encryptMegolmMessage()` — Encrypt message using Megolm session for groups
    2. `decryptMegolmMessage()` — Decrypt Megolm-encrypted group message
    3. `rotateMegolmSession()` — Rotate Megolm session key
    4. `shareMegolmKey()` — Share Megolm session key with new group member
    5. `handleKeyRequest()` — Process incoming key share requests
    6. `storeSession()` / `loadSession()` — Persistent session storage
  - Follow same patterns as Task 14 (OlmKit usage, secure storage)
  - Add unit tests for each method
  - Run `./gradlew test` to verify

  **Must NOT do**:
  - Do not change methods implemented in Task 14
  - Do not change the public API
  - Do not hardcode any encryption keys

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Crypto implementation building on Task 14
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4
  - **Blocks**: None
  - **Blocked By**: Task 14 (core methods must be implemented first)

  **References**:

  **Pattern References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/crypto/MatrixOlmService.kt` — The file (now with 5 methods implemented from Task 14)

  **WHY Each Reference Matters**:
  - Same file, building on Task 14 patterns

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All E2EE methods implemented (zero TODOs)
    Tool: Bash (grep)
    Preconditions: All methods implemented
    Steps:
      1. Run: grep -c "TODO" applications/ArmorChat/app/src/main/java/app/armorclaw/crypto/MatrixOlmService.kt
      2. Assert: count is 0 (all TODOs replaced)
    Expected Result: Zero remaining TODO stubs
    Evidence: .sisyphus/evidence/task-15-all-implemented.txt

  Scenario: All tests pass
    Tool: Bash
    Steps:
      1. Run: cd applications/ArmorChat && ./gradlew test 2>&1 | tail -15
      2. Assert: BUILD SUCCESSFUL, all tests pass
    Expected Result: Full E2EE test suite passes
    Evidence: .sisyphus/evidence/task-15-all-tests.txt
  ```

  **Commit**: YES
  - Message: `feat(e2ee): implement remaining Megolm and session management methods`
  - Files: `applications/ArmorChat/app/src/main/java/app/armorclaw/crypto/MatrixOlmService.kt`
  - Pre-commit: `cd applications/ArmorChat && ./gradlew test`

- [ ] 16. Add tests for skills package

  **What to do**:
  - Add unit tests for `bridge/internal/skills/` (15 files, currently 0 tests)
  - Read the skills package to understand its public API (skill registration, execution, matching)
  - Write tests covering:
    - Skill registration and lookup
    - Skill matching (input → correct skill)
    - Skill execution flow
    - Error handling (unknown skill, invalid input, skill failure)
  - Use table-driven tests
  - Target: minimum 60% line coverage for the package
  - Run `go test -cover ./bridge/internal/skills/...` to verify

  **Must NOT do**:
  - Do not change production code in skills package
  - Do not write integration tests requiring actual skill execution

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Writing tests for 15-file package
  - **Skills**: [`test-driven-development`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 14, 15, 17)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/internal/skills/` — The package to test (read all 15 files)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Tests exist and pass with coverage
    Tool: Bash
    Steps:
      1. Run: go test -v -cover ./bridge/internal/skills/... 2>&1
      2. Assert: at least 5 test functions, all pass, coverage >= 60%
    Expected Result: Tests pass with 60%+ coverage
    Evidence: .sisyphus/evidence/task-16-skills-tests.txt
  ```

  **Commit**: YES
  - Message: `test(skills): add unit tests for skills package`
  - Files: `bridge/internal/skills/*_test.go`
  - Pre-commit: `go test ./bridge/internal/skills/...`

- [ ] 17. Add tests for docker client

  **What to do**:
  - Add unit tests for `bridge/pkg/docker/client.go` (996 lines, currently 0 tests)
  - Read the docker client to understand its public API (container management, image operations)
  - Write tests covering:
    - Container creation and removal
    - Image pull and management
    - Container status and listing
    - Error handling (Docker API failures, timeout, missing container)
  - Mock the Docker API client using interfaces or httptest
  - Target: minimum 60% line coverage
  - Run `go test -cover ./bridge/pkg/docker/...` to verify

  **Must NOT do**:
  - Do not change production code in docker package
  - Do not write integration tests requiring Docker daemon

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Writing tests for Docker client with API mocking
  - **Skills**: [`test-driven-development`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 14, 15, 16)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/docker/client.go` — The file to test (996 lines)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Tests exist and pass with coverage
    Tool: Bash
    Steps:
      1. Run: go test -v -cover ./bridge/pkg/docker/... 2>&1
      2. Assert: at least 5 test functions, all pass, coverage >= 60%
    Expected Result: Tests pass with 60%+ coverage
    Evidence: .sisyphus/evidence/task-17-docker-tests.txt
  ```

  **Commit**: YES
  - Message: `test(docker): add unit tests for docker client`
  - Files: `bridge/pkg/docker/*_test.go`
  - Pre-commit: `go test ./bridge/pkg/docker/...`

---

## Final Verification Wave

> 4 review agents run in PARALLEL. ALL must APPROVE.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, grep for patterns). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `go build ./...` + `go vet ./...` + `go test ./...`. Review all changed files for: `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names (data/result/item/temp).
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (provider types work across modules, refactored god objects compile together). Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Wave 1 (P0 fixes)**: Individual commits per task
  - `fix(rolodex): correct self-assignment bug in UpdateContact` — rolodex.go
  - `fix(security): replace production panics with proper error handling` — enforcement.go, securerandom.go, notifier.go, compliance.go
  - `fix(security): replace weak randomness with crypto/rand for token generation` — rpc/server.go
  - `refactor(executor): remove unused helper functions` — engine.go
  - `fix(deploy): replace chmod 666 recommendation with secure alternative` — container-setup.sh
  - `fix(typescript): remove `as any` type assertion in browser.ts` — browser.ts

- **Wave 2 (consolidation)**: One commit per task
  - `refactor(providers): consolidate provider types to canonical registry` — multiple files
  - `refactor(keystore): split god object into focused modules` — keystore.go → keystore/*.go

- **Wave 3 (refactoring)**: One commit per extraction
  - `refactor(matrix): split adapter into focused modules` — matrix.go → matrix/*.go
  - `refactor(cmd): split main.go into subcommand packages` — main.go → cmd/bridge/*.go

- **Wave 4 (E2EE + tests)**: One commit per task
  - `feat(e2ee): implement core Olm/Megolm methods` — MatrixOlmService.kt
  - `feat(e2ee): implement remaining crypto methods` — MatrixOlmService.kt
  - `test(recovery): add unit tests for recovery package` — recovery/*_test.go
  - `test(http): add unit tests for http server` — http/*_test.go
  - `test(skills): add unit tests for skills package` — skills/*_test.go
  - `test(docker): add unit tests for docker client` — docker/*_test.go

---

## Success Criteria

### Verification Commands
```bash
# Go bridge — must all pass
go build ./bridge/...          # Expected: exit 0 (no errors)
go vet ./bridge/...            # Expected: exit 0 (no issues)
go test ./bridge/...           # Expected: all tests pass

# Verify no panics in production paths
grep -rn "panic(" bridge/pkg/enforcement/ bridge/pkg/securerandom/ bridge/pkg/errors/ bridge/pkg/audit/
# Expected: 0 results (all replaced with error returns)

# Verify no weak randomness
grep -rn "time.Now().Nanosecond()" bridge/
# Expected: 0 results

# Verify no dead code
grep -rn "func isValidJSON\|func truncateOutput\|func parseCommand\|func readOutput" bridge/internal/executor/
# Expected: 0 results

# Verify no chmod 666
grep -rn "chmod 666" deploy/
# Expected: 0 results

# Verify provider consolidation
grep -rn "type Provider string\|type SecretProvider string\|type ProviderType string" bridge/
# Expected: only in providers/registry.go (canonical source)

# Verify god object sizes
wc -l bridge/cmd/bridge/main.go        # Expected: <500 lines
wc -l bridge/internal/adapter/matrix.go # Expected: <500 lines
wc -l bridge/pkg/keystore/keystore.go   # Expected: <500 lines (or split into package)

# TypeScript
cd browser-service && npm run build     # Expected: exit 0
grep -rn "as any" browser-service/src/  # Expected: 0 results

# Android
cd applications/ArmorChat && ./gradlew test  # Expected: all tests pass
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass (`go test ./bridge/...`)
- [ ] Zero panics in production paths
- [ ] Zero weak randomness sources
- [ ] Zero dead code functions
- [ ] Provider types consolidated to single source
- [ ] God objects all <500 lines
- [ ] E2EE methods implemented and tested
- [ ] Test coverage added for 4 critical packages
