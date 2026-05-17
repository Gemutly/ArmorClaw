# Post-Deploy VPS Fixes: Matrix Auth, YARA Pipeline, Test Suite

## TL;DR

> **Quick Summary**: Fix the 5 remaining issues from VPS functional testing: Matrix bridge re-authentication failure (userID never initialized, Username never passed), YARA rules missing from Docker image AND temp file never written (scanning empty files), YARA error handling conflates "uninitialized" with "malware found", test suite using wrong RPC methods and response assertions.
>
> **Deliverables**:
> - Matrix bridge re-authenticates from password when token expires
> - YARA rules shipped in Docker image + attachment content actually written before scanning
> - YARA graceful degradation: allow email on init failure, block on malware detection
> - VPS test suite aligned to 110 registered RPC methods and real API responses
> - Docker image rebuilt, pushed, deployed, and re-tested on VPS
>
> **Estimated Effort**: Medium
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: T1/T2 → T5 (go test gate) → T6 (Docker rebuild) → T7 (VPS test)

---

## Context

### Original Request
Fix remaining issues from VPS functional test report: Matrix auth, YARA rules, test suite alignment, and expanded coverage.

### Interview Summary
**Key Discussions**:
- Matrix auth has TWO bugs: `ensureValidToken()` reads `m.userID` which is empty (never set in `New()`), AND `main.go:2324` doesn't pass `Username` in the `adapter.Config{}` construction
- `New()` stores `cfg.Password` but NOT `cfg.Username` and NOT `userID`
- YARA has TWO bugs: rules file not in Docker image, AND `ingest_server.go` creates temp file PATH but never writes `att.Content` to it — YARA always scans empty files
- YARA error handling doesn't distinguish "uninitialized" from "malware found" — both return `(false, error)` or `(false, nil)`, both result in email rejection
- Test suite: `"status"` RPC method doesn't exist; correct name is `"bridge.status"`; discovery API response has no "discovery" string

**Research Findings**:
- `bridge/internal/adapter/matrix.go:2036` — `username := m.userID` but userID only set after `Login()` at line 360
- `bridge/internal/adapter/matrix.go:223-242` — `New()` stores `cfg.Password` but NOT `cfg.Username` and NOT `userID`
- `bridge/cmd/bridge/main.go:2324-2328` — adapter.Config{} construction passes Password but omits Username entirely
- `bridge/pkg/config/config.go:1324-1333` — `ToMatrixConfig()` missing `Password` field (VERIFY IF USED before fixing)
- `bridge/pkg/rpc/server.go:1219-1333` — Full 110-method RPC registry: `"bridge.status"` not `"status"`
- `bridge/pkg/yara/scanner.go:50-51` — `ScanFileForMalware` returns `(false, error)` when uninitialized → email rejection
- `bridge/pkg/email/ingest_server.go:153-155` — temp file path created but `att.Content` NEVER written before YARA scan
- `bridge/cmd/bridge/main.go:2255` — YARA path: `filepath.Join("configs", "yara_rules.yar")` (relative)
- `Dockerfile.quickstart:202` — copies top-level `configs/` but yara_rules.yar in `bridge/configs/`

**Review Findings** (9.0/10 review, incorporated):
- Add `go test` pre-flight gate before Docker rebuild (Suggestion 1)
- Add explicit log message when YARA scan is skipped (Suggestion 2)
- Ensure password is never logged or printed in debug output (Suggestion 3)
- Natural checkpoint at Docker rebuild — no wave split needed (Suggestion 4)

### Metis Review
**Critical Gaps Identified**:
1. **YARA temp file never written** — `ingest_server.go:153-155` creates path but doesn't write content. YARA scans empty files even when initialized. This makes the entire YARA pipeline ineffective.
2. **Username not passed in runtime construction** — `main.go:2324` constructs `adapter.Config{}` without `Username`. Even if `New()` stored it, the runtime path doesn't pass it.
3. **ToMatrixConfig() may be unused** — Must verify with `grep -rn "ToMatrixConfig" bridge/` before investing effort. If unused, skip.
4. **Password leak risk** — `Login()` at line 352 calls `WithInputs(map[string]any{"user": username})` which logs the username. Must audit all error paths in `ensureValidToken` to ensure password value never appears in error messages.
5. **YARA error vs malware** — Need a sentinel error (`ErrNotInitialized`) to distinguish "YARA not loaded" from "scan failed on real file" from "malware found". Current tri-state is broken.

---

## Work Objectives

### Core Objective
Fix all remaining VPS deployment issues so the bridge runs fully functional: Matrix authenticated, YARA scanning real attachment content, and test suite aligned to live method names.

### Concrete Deliverables
- `bridge/internal/adapter/matrix.go` — `username` field added, stored from `New()`, used by `ensureValidToken()`
- `bridge/cmd/bridge/main.go` — `adapter.Config{}` construction includes `Username`
- `Dockerfile.quickstart` — copies `bridge/configs/yara_rules.yar` into image
- `bridge/pkg/email/ingest_server.go` — writes `att.Content` to temp file before YARA scan; graceful degradation on YARA init error; explicit log messages
- `deploy/health-check.sh` — uses correct RPC method name
- Dockerfile HEALTHCHECK — verified against registry
- VPS test suite — aligned to `bridge.status` and real discovery API fields
- Docker image rebuilt, pushed, deployed, tested on VPS with report

### Definition of Done
- [ ] Bridge Matrix sync shows `logged_in: true` in `matrix.status` response
- [ ] YARA initialization succeeds (no warning in logs)
- [ ] YARA temp file contains actual attachment content (verified by unit test)
- [ ] VPS test suite: 10/10 PASS with correct assertions
- [ ] Report saved to `.sisyphus/reports/`
- [ ] No password values appear in any log statement or error message

### Must Have
- Matrix bridge re-authenticates from password when token expires
- `Username` passed from main.go AND stored in MatrixAdapter during `New()`
- YARA rules present in Docker image at correct path
- Email ingest writes actual attachment bytes before YARA scan
- YARA gracefully skips scan (allows email) when not initialized, with explicit log message
- YARA blocks email when malware actually detected
- Test suite uses registered RPC method names
- `go test` passes as gate before Docker rebuild

### Must NOT Have (Guardrails)
- Do NOT modify `container-setup.sh` or `deploy/container-setup.sh`
- Do NOT initialize `userID` to bare localpart — must be full Matrix ID `@localpart:server` or empty
- Do NOT make YARA scan error silently allow actual malware — only skip on initialization error (sentinel `ErrNotInitialized`)
- Do NOT change `Login()` method behavior or response handling
- Do NOT change HEALTHCHECK in Dockerfile.quickstart without verifying it matches the method registry
- Do NOT add new provisioning RPC or speculative features
- Do NOT touch Go bridge architecture — minimal patches only
- Do NOT log or print password values anywhere — audit all error paths
- Do NOT fix `ToMatrixConfig()` without first verifying it's actually called at runtime
- Do NOT sanitize `att.Filename` in YARA temp path — file as follow-up
- Do NOT change `bridge/pkg/yara/scanner.go` internal logic

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (bridge has `bridge/pkg/rpc/*_test.go`, `bridge/internal/adapter/*_test.go` files)
- **Automated tests**: YES (tests-after — add tests for Matrix auth fix and YARA temp file fix)
- **Framework**: go test

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go bridge**: Use Bash (go test) — Run unit tests, check pass/fail
- **VPS deployment**: Use Bash (SSH) — Deploy, run test suite, collect results

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately - independent fixes):
├── Task 1: Fix Matrix auth — add username field + store in New() [deep]
├── Task 2: Fix Matrix auth — pass Username in main.go adapter construction [quick]
├── Task 3: Fix YARA — copy rules to Docker image [quick]
├── Task 4: Fix YARA — write attachment content to temp file + graceful degradation [unspecified-high]
└── Task 5: Fix test suite — RPC names + discovery assertions + health-check.sh [quick]

Wave 2 (After Wave 1 — verify + rebuild + deploy):
├── Task 6: Go test gate — run all unit tests as pre-flight check [quick]
├── Task 7: Rebuild Docker image + push + deploy to VPS [quick]
└── Task 8: Run full VPS test suite + save report [unspecified-high]

Wave FINAL (After ALL tasks — verification):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
└── Task F3: Scope fidelity check (deep)

Critical Path: T1 → T6 → T7 → T8 → F1-F3
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 5 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| T1 | - | T6 |
| T2 | T1 (username field must exist first) | T6 |
| T3 | - | T7 |
| T4 | - | T6 |
| T5 | - | T7 |
| T6 | T1, T2, T4 | T7 |
| T7 | T3, T5, T6 | T8 |
| T8 | T7 | F1-F3 |
| F1 | T8 | user okay |
| F2 | T8 | user okay |
| F3 | T8 | user okay |

### Agent Dispatch Summary

- **Wave 1**: **5** — T1 → `deep`, T2 → `quick`, T3 → `quick`, T4 → `unspecified-high`, T5 → `quick`
- **Wave 2**: **3** — T6 → `quick`, T7 → `quick`, T8 → `unspecified-high`
- **FINAL**: **3** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `deep`

---

## TODOs

- [x] 1. Fix Matrix auth — add `username` field to MatrixAdapter + store in `New()`

  **What to do**:
  - In `bridge/internal/adapter/matrix.go`, add a new field `username string` to the `MatrixAdapter` struct (distinct from `userID string`)
    - `userID` = full Matrix ID format `@localpart:server` — only set after successful login from server response
    - `username` = the localpart/username from config — available immediately in `New()`
  - In `New()` (lines 223-242), store `cfg.Username` into the new `m.username` field:
    ```go
    m.username = cfg.Username
    ```
  - In `ensureValidToken()` (around line 2036), change the password fallback to use `m.username` instead of `m.userID`:
    ```go
    // BEFORE: username := m.userID  (empty, never set before login)
    // AFTER:  username := m.username  (set from config in New())
    ```
  - This is the CRITICAL fix: when `ensureValidToken` falls back to password login, it needs a non-empty username. `m.userID` is empty until after the first successful login. `m.username` is available from construction time.
  - Add a unit test: construct `MatrixAdapter` via `New()` with `Username: "bridge"`, verify `m.username == "bridge"` and `m.userID == ""`
  - Add a unit test: mock `ensureValidToken` with expired token, verify it uses `m.username` for password re-login

  **Must NOT do**:
  - Do NOT initialize `userID` from config — it MUST come from the Matrix server login response
  - Do NOT change `Login()` method behavior
  - Do NOT change token refresh logic — only change the data source for username in fallback path
  - Do NOT log the password value anywhere

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Requires tracing Matrix auth flow through multiple methods, understanding the distinction between username and userID, and writing careful tests for the re-auth path
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T3, T4, T5)
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 2 (needs username field to exist), Task 6 (go test gate)
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/internal/adapter/matrix.go:195-210` — MatrixAdapter struct definition — add `username string` field here, near the existing `userID string` field
  - `bridge/internal/adapter/matrix.go:223-242` — `New()` constructor showing where `m.username = cfg.Username` should be added
  - `bridge/internal/adapter/matrix.go:2030-2068` — Full `ensureValidToken()` method — line 2036 is where `m.userID` is read (currently empty at re-auth time)

  **API/Type References**:
  - `bridge/internal/adapter/matrix.go:360` — `m.userID = result.UserID` — the ONLY place userID should be set (from login response)
  - `bridge/internal/adapter/matrix.go:198` — `adapter.Config` struct showing `Username string` field exists in config but is never stored

  **WHY Each Reference Matters**:
  - The struct definition is where the new field goes — must be distinct from `userID`
  - `New()` is where initialization happens — the fix location
  - `ensureValidToken()` line 2036 is where the bug manifests — `m.userID` is empty, needs `m.username` instead

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Username stored in MatrixAdapter during construction
    Tool: Bash (go test)
    Preconditions: adapter.Config has Username: "bridge-test"
    Steps:
      1. cd bridge && go test -v -run TestMatrixAdapter_UsernameStored ./internal/adapter/...
      2. Assert m.username == "bridge-test" AND m.userID == ""
    Expected Result: Test PASS — username set, userID still empty (awaits login)
    Failure Indicators: m.username is empty, or m.userID was pre-populated
    Evidence: .sisyphus/evidence/task-1-username-stored.txt

  Scenario: ensureValidToken uses m.username for password fallback
    Tool: Bash (go test)
    Preconditions: Token expired, m.userID is empty, m.username is "bridge", m.password is set
    Steps:
      1. cd bridge && go test -v -run TestEnsureValidToken_UsernameFallback ./internal/adapter/...
      2. Assert re-login call uses "bridge" as username (not empty string)
    Expected Result: Test PASS — password login succeeds with correct username
    Failure Indicators: "no password available" error, or login called with empty username
    Evidence: .sisyphus/evidence/task-1-password-fallback.txt
  ```

  **Commit**: YES (groups with T2)
  - Message: `fix(bridge): store username in MatrixAdapter for re-authentication fallback`
  - Files: `bridge/internal/adapter/matrix.go`, `bridge/internal/adapter/matrix_test.go`
  - Pre-commit: `cd bridge && go test ./internal/adapter/... -v`

- [x] 2. Fix Matrix auth — pass `Username` in main.go adapter construction

  **What to do**:
  - In `bridge/cmd/bridge/main.go`, locate the `adapter.Config{}` construction (around line 2324-2328)
  - Add the `Username` field to the config literal:
    ```go
    adapter.Config{
        HomeserverURL: ...,
        Username:      cfg.Matrix.Username,  // ADD THIS LINE
        Password:      cfg.Matrix.Password,
        ...
    }
    ```
  - This ensures the runtime path passes the username so `New()` can store it (T1 added the storage logic)
  - Verify `cfg.Matrix.Username` is populated from config — trace the config loading path
  - **IMPORTANT**: Before touching `ToMatrixConfig()`, run `grep -rn "ToMatrixConfig" bridge/` to verify it's actually called somewhere. If unused, DO NOT fix it — skip it entirely.

  **Must NOT do**:
  - Do NOT fix `ToMatrixConfig()` without verifying it's called at runtime
  - Do NOT log or print the password value — audit any `log.Debug` / `log.Info` calls near this code
  - Do NOT change the `Password` field (already correctly passed)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single field addition in one construction literal, plus a grep to verify ToMatrixConfig usage
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T1 — username field must exist in struct)
  - **Parallel Group**: Wave 1 (runs after T1 completes, concurrent with T3/T4/T5)
  - **Blocks**: Task 6 (go test gate)
  - **Blocked By**: Task 1 (username field added to struct)

  **References**:

  **Pattern References**:
  - `bridge/cmd/bridge/main.go:2324-2328` — The `adapter.Config{}` construction showing `Password` is passed but `Username` is missing — add it here
  - `bridge/internal/adapter/matrix.go:198` — `adapter.Config` struct showing `Username string` field exists

  **API/Type References**:
  - `bridge/pkg/config/config.go` — Config struct showing `Matrix.Username` field (verify path: `cfg.Matrix.Username`)

  **WHY Each Reference Matters**:
  - The main.go construction is the runtime path — this is what actually runs in production
  - The Config struct confirms `Username` field exists and is available

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Username passed in adapter.Config construction
    Tool: Bash (go test + grep)
    Preconditions: T1 completed (username field exists)
    Steps:
      1. grep -n "Username:" bridge/cmd/bridge/main.go | grep "adapter.Config\|adapter\.Config"
      2. Assert a line exists with Username: cfg.Matrix.Username (or equivalent)
      3. cd bridge && go build ./cmd/bridge/... — assert builds without error
    Expected Result: grep finds the Username field, build succeeds
    Failure Indicators: No Username in adapter.Config, or build fails
    Evidence: .sisyphus/evidence/task-2-username-passed.txt

  Scenario: ToMatrixConfig verified as used or skipped
    Tool: Bash (grep)
    Preconditions: None
    Steps:
      1. grep -rn "ToMatrixConfig" bridge/ --include="*.go"
      2. Record whether it's called anywhere besides its definition
    Expected Result: Clear answer — either "called at X,Y" or "unused, safely skipped"
    Failure Indicators: Ambiguous result
    Evidence: .sisyphus/evidence/task-2-tomatrixconfig-check.txt
  ```

  **Commit**: YES (groups with T1)
  - Message: `fix(bridge): pass Username in adapter config for Matrix re-auth`
  - Files: `bridge/cmd/bridge/main.go`
  - Pre-commit: `cd bridge && go build ./cmd/bridge/... && go test ./internal/adapter/... -v`

- [x] 3. Fix YARA — copy rules to Docker image

  **What to do**:
  - In `Dockerfile.quickstart`, add a COPY directive for YARA rules AFTER the existing `COPY configs/ /opt/armorclaw/configs/` line (around line 202):
    ```dockerfile
    COPY bridge/configs/yara_rules.yar /opt/armorclaw/configs/yara_rules.yar
    ```
  - This ensures the bridge can find the rules at the relative path `configs/yara_rules.yar` (as referenced in `main.go:2255` which uses `filepath.Join("configs", "yara_rules.yar")`)
  - Verify the file exists at `bridge/configs/yara_rules.yar` before adding the COPY line
  - Verify the Dockerfile working directory is `/opt/armorclaw` (so relative paths resolve correctly)

  **Must NOT do**:
  - Do NOT modify the YARA scanner initialization in `main.go`
  - Do NOT modify `bridge/pkg/yara/scanner.go`
  - Do NOT change the YARA rules file itself
  - Do NOT move the rules file to a different location

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single COPY line addition in Dockerfile, verify file exists
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T4, T5)
  - **Blocks**: Task 7 (Docker rebuild)
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `Dockerfile.quickstart:202` — Existing `COPY configs/ /opt/armorclaw/configs/` — add the YARA copy RIGHT AFTER this line, same pattern
  - `bridge/configs/yara_rules.yar` — The actual YARA rules file (153 lines, 12 rules) — verify it exists before adding COPY

  **API/Type References**:
  - `bridge/cmd/bridge/main.go:2254-2258` — YARA initialization showing `filepath.Join("configs", "yara_rules.yar")` — confirms the relative path that must match the Dockerfile COPY destination

  **WHY Each Reference Matters**:
  - Dockerfile COPY must land at the same relative path that `main.go` expects
  - Rules file must exist locally before Dockerfile COPY can work

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: YARA rules file exists in Docker image
    Tool: Bash (docker)
    Preconditions: Docker image built with new COPY directive
    Steps:
      1. docker run --rm mikegemut/armorclaw:latest ls -la /opt/armorclaw/configs/yara_rules.yar
      2. Assert file exists and size > 0
    Expected Result: File listed, size ~5KB+
    Failure Indicators: "No such file or directory"
    Evidence: .sisyphus/evidence/task-3-yara-rules-in-image.txt

  Scenario: YARA COPY line placed correctly in Dockerfile
    Tool: Bash (grep)
    Preconditions: Edit made
    Steps:
      1. grep -n "yara_rules.yar" Dockerfile.quickstart
      2. Assert exactly 1 match found
    Expected Result: Line found with COPY directive for yara_rules.yar
    Failure Indicators: No match, or multiple matches
    Evidence: .sisyphus/evidence/task-3-copy-verified.txt
  ```

  **Commit**: YES
  - Message: `fix(docker): ship YARA rules in Docker image`
  - Files: `Dockerfile.quickstart`
  - Pre-commit: `grep -n "yara_rules.yar" Dockerfile.quickstart`

- [x] 4. Fix YARA — write attachment content to temp file + graceful degradation

  **What to do**:
  - **Part A — Write attachment content** (CRITICAL): In `bridge/pkg/email/ingest_server.go`, around lines 153-157 where YARA scanning happens:
    1. After creating `tmpPath`, write the actual attachment content: `os.WriteFile(tmpPath, att.Content, 0600)`
    2. Add `defer os.Remove(tmpPath)` to clean up the temp file after scanning
    3. This is the most critical fix — without it, YARA scans empty files even when initialized
  - **Part B — Graceful degradation**: In the same YARA scanning section:
    1. When `ScanFileForMalware` returns an error, distinguish between "not initialized" and "scan error":
       - Check if `scanner == nil` or if the error indicates uninitialized state
       - If YARA is not initialized: log `log.Warn("YARA scan skipped (not initialized) — allowing email through")` and ALLOW the email (treat as clean)
       - If YARA IS initialized but scan errored: log the error and ALLOW the email through with a warning (prefer availability over false positive rejection)
    2. Only BLOCK the email when `isClean == false` AND `err == nil` (YARA successfully scanned and found malware)
    3. The decision matrix should be:
       | `isClean` | `err` | Action |
       |-----------|-------|--------|
       | true | nil | Allow (clean) |
       | false | nil | Block (malware detected) |
       | any | error (not initialized) | Allow + warn log |
       | any | error (scan failed) | Allow + error log |
  - Add a unit test: mock YARA returning error (not initialized), verify email passes through
  - Add a unit test: mock YARA returning `(false, nil)` — malware detected, verify email is rejected
  - Add a unit test: mock YARA returning `(true, nil)` — clean, verify email passes
  - Add a unit test: verify temp file is cleaned up after scan (check no orphan files)

  **Must NOT do**:
  - Do NOT make YARA scan failure silently allow actual malware — only skip on initialization/error
  - Do NOT modify `bridge/pkg/yara/scanner.go`
  - Do NOT change the YARA rules file
  - Do NOT sanitize `att.Filename` — file as follow-up issue
  - Do NOT add a `ErrNotInitialized` sentinel type to scanner.go — use a simple nil check or string match

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Touches security-adjacent logic (malware scanning pipeline), requires careful decision matrix, needs 4 unit tests with different scenarios
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T3, T5)
  - **Blocks**: Task 6 (go test gate)
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/email/ingest_server.go:143` — `s.storage.StoreAttachment()` showing that attachment data exists in `att.Content` at this point
  - `bridge/pkg/email/ingest_server.go:153-157` — The YARA scanning section — where temp file write and graceful degradation need to be added

  **API/Type References**:
  - `bridge/pkg/yara/scanner.go:50-51` — `ScanFileForMalware` signature returning `(bool, error)` — `bool` is `isClean`, `error` is non-nil when YARA not initialized
  - `bridge/pkg/yara/scanner.go:30-40` — YARA scanner struct showing nil-check pattern for uninitialized state

  **WHY Each Reference Matters**:
  - `ingest_server.go:153` is where the bug is — temp file path created but content never written
  - `att.Content` contains the actual `[]byte` attachment data — must be written to temp file
  - The decision matrix determines whether legitimate emails get blocked (current bug) or malware gets through

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Email passes through when YARA not initialized (graceful degradation)
    Tool: Bash (go test)
    Preconditions: Mock YARA scanner returning (false, errors.New("scanner not initialized"))
    Steps:
      1. cd bridge && go test -v -run TestIngest_YARANotInitialized ./pkg/email/...
      2. Assert email is NOT rejected — graceful pass-through
      3. Assert log contains "YARA scan skipped"
    Expected Result: Test PASS, email allowed through with warning log
    Failure Indicators: Email rejected
    Evidence: .sisyphus/evidence/task-4-yara-graceful.txt

  Scenario: Email blocked when YARA detects actual malware
    Tool: Bash (go test)
    Preconditions: Mock YARA scanner returning (false, nil) — malware detected
    Steps:
      1. cd bridge && go test -v -run TestIngest_YARABlock ./pkg/email/...
      2. Assert email IS rejected with malware detection message
    Expected Result: Test PASS, email rejected
    Failure Indicators: Email allowed through
    Evidence: .sisyphus/evidence/task-4-yara-block.txt

  Scenario: Clean email passes YARA scan
    Tool: Bash (go test)
    Preconditions: Mock YARA scanner returning (true, nil) — clean
    Steps:
      1. cd bridge && go test -v -run TestIngest_YARAClean ./pkg/email/...
      2. Assert email passes through normally
    Expected Result: Test PASS, email accepted
    Failure Indicators: Email rejected despite being clean
    Evidence: .sisyphus/evidence/task-4-yara-clean.txt

  Scenario: Temp file cleaned up after scan
    Tool: Bash (go test)
    Preconditions: YARA scan completes (success or error)
    Steps:
      1. cd bridge && go test -v -run TestIngest_TempFileCleanup ./pkg/email/...
      2. Assert temp file does NOT exist after scan completes
    Expected Result: Test PASS, no orphan temp files
    Failure Indicators: Temp file still exists after scan
    Evidence: .sisyphus/evidence/task-4-temp-cleanup.txt
  ```

  **Commit**: YES
  - Message: `fix(bridge): write attachment content before YARA scan, add graceful degradation`
  - Files: `bridge/pkg/email/ingest_server.go`, `bridge/pkg/email/ingest_server_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/email/... -v`

- [x] 5. Fix test suite — RPC method names + discovery assertions + health-check.sh

  **What to do**:
  - **Part A — RPC method names**: Search ALL files for the incorrect RPC method name `"status"` (bare word) and replace with `"bridge.status"`:
    - `deploy/health-check.sh` — PRODUCTION health monitoring script
    - `Dockerfile.quickstart` — HEALTHCHECK directive
    - Any test scripts in `tests/` that use `"status"`
    - Any other files found via `grep -rn '"status"' deploy/ tests/ Dockerfile*`
    - DO NOT change `"matrix.status"` — that one is correct
  - **Part B — Discovery test assertion**: In the VPS test suite, change the assertion from `grep -q "discovery"` to validate actual response fields:
    ```bash
    # OLD: echo "..." | socat ... | grep -q "discovery"
    # NEW: validate JSON fields exist
    RESPONSE=$(echo '{"jsonrpc":"2.0","id":1,"method":"discovery.info"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock)
    echo "$RESPONSE" | python3 -c "import json,sys; d=json.load(sys.stdin); assert 'api_url' in d.get('result',{}), 'missing api_url'"
    ```
    Or simpler: `grep -q '"api_url"'` if python3 is available
  - **Part C — Verify Dockerfile HEALTHCHECK**: Confirm the HEALTHCHECK in Dockerfile.quickstart uses `"bridge.status"` not `"status"`. The HEALTHCHECK is critical — wrong method name means Docker reports container as unhealthy.

  **Must NOT do**:
  - Do NOT change any Go source code in this task
  - Do NOT modify `container-setup.sh`
  - Do NOT change `matrix.status` — that method name is correct
  - Do NOT add new tests — only fix existing broken assertions

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Search-and-replace across known files, straightforward assertion updates
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T3, T4)
  - **Blocks**: Task 7 (Docker rebuild)
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/server.go:1219-1333` — Full RPC handler registration map showing ALL 110 method names — use this as the source of truth for correct method names
  - `deploy/health-check.sh` — Production health check script that MUST use `bridge.status`
  - `Dockerfile.quickstart` — HEALTHCHECK line that Docker executes to determine container health

  **API/Type References**:
  - `bridge/pkg/discovery/http.go:98-165` — Discovery HTTP server showing actual response shape: `api_url`, `mode`, `port`, `service_name` — use these field names for assertions
  - `bridge/pkg/discovery/mdns.go:35-51` — BridgeInfo struct with field names

  **WHY Each Reference Matters**:
  - `server.go` is the single source of truth for method names — any method not in this map will return "method not found"
  - `health-check.sh` affects production monitoring — wrong method means false unhealthy alerts

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: No bare "status" RPC calls remain in deploy/test files
    Tool: Bash (grep)
    Preconditions: All replacements made
    Steps:
      1. grep -rn '"status"' deploy/ tests/ Dockerfile* --include="*.sh" --include="Dockerfile*"
      2. Assert ZERO matches (or only "matrix.status" which is correct)
    Expected Result: grep returns exit code 1 (no matches found)
    Failure Indicators: Any line containing bare "status" (not "bridge.status" or "matrix.status")
    Evidence: .sisyphus/evidence/task-5-no-bare-status.txt

  Scenario: Dockerfile HEALTHCHECK uses bridge.status
    Tool: Bash (grep)
    Preconditions: Dockerfile.quickstart updated
    Steps:
      1. grep -n "HEALTHCHECK" Dockerfile.quickstart
      2. grep -A2 "HEALTHCHECK" Dockerfile.quickstart | grep "bridge.status"
    Expected Result: HEALTHCHECK line contains "bridge.status"
    Failure Indicators: "bridge.status" not found in HEALTHCHECK output
    Evidence: .sisyphus/evidence/task-5-healthcheck-method.txt
  ```

  **Commit**: YES
  - Message: `fix(tests,deploy): align RPC method names and discovery assertions`
  - Files: `deploy/health-check.sh`, `Dockerfile.quickstart`, test scripts
  - Pre-commit: `grep -rn '"status"' deploy/ tests/ Dockerfile* — should return nothing`

- [x] 6. Go test gate — run all unit tests as pre-flight check

  **What to do**:
  - After ALL Wave 1 tasks (T1-T5) complete, run the full Go test suite as a mandatory gate before Docker rebuild:
    ```bash
    cd bridge && go test -v ./internal/adapter/... ./pkg/config/... ./pkg/yara/... ./pkg/email/... ./pkg/rpc/...
    ```
  - If ANY test fails, STOP and fix before proceeding to Docker rebuild (Task 7)
  - This is the checkpoint recommended by the reviewer — ensures code quality before investing time in Docker build
  - Save test output as evidence
  - Also run `go vet ./bridge/...` to catch any issues

  **Must NOT do**:
  - Do NOT proceed to Docker rebuild if any test fails
  - Do NOT skip tests for specific packages
  - Do NOT modify any code — this is a verification-only task

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Run two commands, capture output, check pass/fail
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (sequential first)
  - **Blocks**: Task 7 (Docker rebuild)
  - **Blocked By**: Tasks 1, 2, 4 (code changes that need testing)

  **References**:
  - All test files created in T1, T2, T4
  - `bridge/pkg/rpc/*_test.go` — existing RPC tests
  - `bridge/pkg/email/*_test.go` — existing + new email tests

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All Go tests pass
    Tool: Bash (go test)
    Preconditions: All Wave 1 code changes committed
    Steps:
      1. cd bridge && go test -v ./internal/adapter/... ./pkg/config/... ./pkg/yara/... ./pkg/email/... ./pkg/rpc/... 2>&1 | tee .sisyphus/evidence/task-6-go-test-full.txt
      2. Assert exit code 0
    Expected Result: ALL tests PASS, 0 failures
    Failure Indicators: Any test FAIL, exit code != 0
    Evidence: .sisyphus/evidence/task-6-go-test-full.txt

  Scenario: Go vet finds no issues
    Tool: Bash (go vet)
    Preconditions: All code changes committed
    Steps:
      1. cd bridge && go vet ./...
      2. Assert exit code 0
    Expected Result: No issues reported
    Failure Indicators: Any vet warning or error
    Evidence: .sisyphus/evidence/task-6-go-vet.txt
  ```

  **Commit**: NO (verification-only task)

- [x] 7. Rebuild Docker image + push + deploy to VPS

  **What to do**:
  - Build new Docker image from `Dockerfile.quickstart` incorporating all fixes from T1-T5:
    ```bash
    docker build -t mikegemut/armorclaw:latest -f Dockerfile.quickstart .
    ```
  - Push to Docker Hub:
    ```bash
    docker push mikegemut/armorclaw:latest
    ```
  - SSH to VPS (`ssh -i ~/.ssh/openclaw_win root@5.183.11.149`) and:
    1. Stop existing container: `docker stop armorclaw armorclaw-conduit 2>/dev/null || true`
    2. Remove containers: `docker rm armorclaw armorclaw-conduit 2>/dev/null || true`
    3. Remove volumes for clean start: `docker volume rm armorclaw-data armorclaw-keystore 2>/dev/null || true`
    4. Pull new image: `docker pull mikegemut/armorclaw:latest`
    5. Start Conduit: `docker run -d --name armorclaw-conduit --restart unless-stopped -v armorclaw-conduit:/var/lib/matrix-conduit -e CONDUIT_SERVER_NAME=5.183.11.149 -p 6167:6167 matrixconduit/matrix-conduit:latest`
    6. Wait 5 seconds for Conduit to start
    7. Start ArmorClaw with all env vars (source from `.env` file in project):
       ```bash
       docker run -d --name armorclaw --restart unless-stopped \
         --user root \
         -v /var/run/docker.sock:/var/run/docker.sock \
         -v armorclaw-data:/etc/armorclaw \
         -v armorclaw-keystore:/var/lib/armorclaw \
         -v /run/armorclaw:/run/armorclaw \
         -p 8080:8080 -p 8443:8443 -p 6167:6167 \
         -e ARMORCLAW_API_KEY=$OPENROUTER_API_KEY \
         -e ARMORCLAW_EXTERNAL_MATRIX=true \
         -e ARMORCLAW_SERVER_NAME=5.183.11.149 \
         -e ARMORCLAW_KEYSTORE_SECRET=JWW8vV62rMnCA144ybfpTe/0MNaj7PKtNqb7d4ieLKU= \
         -e ARMORCLAW_HTTP_PORT=8080 \
         mikegemut/armorclaw:latest
       ```
    8. Wait for HEALTHCHECK to report healthy (up to 60s): `docker inspect --format='{{.State.Health.Status}}' armorclaw`
  - Record new Docker image SHA

  **Must NOT do**:
  - Do NOT modify `container-setup.sh`
  - Do NOT skip the clean volume removal — stale config can cause issues
  - Do NOT deploy without verifying the image builds successfully first
  - Do NOT deploy if Task 6 (go test gate) failed

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Straightforward Docker build/push/deploy sequence, well-defined steps
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (after T6 gate, sequential with T8)
  - **Blocks**: Task 8 (VPS test)
  - **Blocked By**: Tasks 3, 5 (Docker changes), Task 6 (test gate)

  **References**:

  **Pattern References**:
  - Previous successful deployment in `.sisyphus/reports/vps-functional-test-2026-05-14.md` — follow same deployment steps
  - `.env` file in project root — contains VPS IP, SSH key path, API keys

  **API/Type References**:
  - VPS: `5.183.11.149`, user `root`, SSH key `~/.ssh/openclaw_win`
  - Docker Hub: `mikegemut/armorclaw:latest`

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Docker image builds successfully with YARA rules
    Tool: Bash (docker)
    Preconditions: All T1-T5 changes committed, T6 gate passed
    Steps:
      1. docker build -t mikegemut/armorclaw:latest -f Dockerfile.quickstart . 2>&1 | tail -5
      2. Assert "Successfully built" or "Successfully tagged" in output
    Expected Result: Build succeeds with no errors
    Failure Indicators: Build fails, "COPY failed", or YARA rules not found
    Evidence: .sisyphus/evidence/task-7-docker-build.txt

  Scenario: VPS container healthy after deploy
    Tool: Bash (SSH)
    Preconditions: Image pushed to Docker Hub, deployed on VPS
    Steps:
      1. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker inspect --format="{{.State.Health.Status}}" armorclaw'
      2. Assert output is "healthy"
    Expected Result: "healthy"
    Failure Indicators: "unhealthy", "starting", or empty output
    Evidence: .sisyphus/evidence/task-7-vps-healthy.txt

  Scenario: No YARA initialization warning in logs
    Tool: Bash (SSH)
    Preconditions: Container running on VPS
    Steps:
      1. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker logs armorclaw 2>&1 | grep -i "yara.*fail\|yara.*error\|yara.*warning"'
      2. Assert exit code 1 (no matches found)
    Expected Result: No YARA-related warnings in logs
    Failure Indicators: Any line mentioning YARA failure/error
    Evidence: .sisyphus/evidence/task-7-yara-init.txt
  ```

  **Commit**: NO (deployment task, no code changes)

- [x] 8. Run full VPS test suite + save report

  **What to do**:
  - SSH to VPS and run the full 10-point test suite:
    1. **Container Stability**: `docker ps --filter name=armorclaw --format '{{.Status}}'` — verify "Up" and "0 restarts"
    2. **Entrypoint Routing**: `docker exec armorclaw ps -p 1 -o comm=` — verify bridge binary is PID 1
    3. **Bridge Process**: `docker exec armorclaw kill -0 1` — verify PID 1 is alive
    4. **Conduit Health**: `curl -s http://localhost:6167/_matrix/client/versions` — verify JSON with versions
    5. **Bridge HTTP/Discovery**: `curl -s http://localhost:8080/api/discovery` — verify JSON with `api_url`, `mode`, `port` fields
    6. **Bridge RPC Socket**: `echo '{"jsonrpc":"2.0","id":1,"method":"bridge.status"}' | docker exec -i armorclaw socat - UNIX-CONNECT:/run/armorclaw/bridge.sock` — verify JSON response with `result`
    7. **HEALTHCHECK**: `docker inspect --format='{{.State.Health.Status}}' armorclaw` — verify "healthy"
    8. **Matrix Integration** (PRIMARY FIX VERIFICATION): `echo '{"jsonrpc":"2.0","id":1,"method":"matrix.status"}' | docker exec -i armorclaw socat - UNIX-CONNECT:/run/armorclaw/bridge.sock` — verify `logged_in: true`
    9. **Non-Interactive Setup**: `docker logs armorclaw 2>&1 | grep -c /dev/tty` — verify count is 0
    10. **Config Sections**: `echo '{"jsonrpc":"2.0","id":1,"method":"config.list"}' | docker exec -i armorclaw socat - UNIX-CONNECT:/run/armorclaw/bridge.sock` — verify 5+ config sections
  - **Extra verification** (from review suggestion): After test #8, run an explicit integration check:
    ```bash
    ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'echo "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"matrix.status\"}" | docker exec -i armorclaw socat - UNIX-CONNECT:/run/armorclaw/bridge.sock | python3 -c "import json,sys; d=json.load(sys.stdin); assert d[\"result\"][\"logged_in\"] == True"'
    ```
  - Save results to `.sisyphus/reports/vps-test-post-fixes-{date}.md`
  - Format: markdown table with Test #, Name, Status (PASS/FAIL), Details

  **Must NOT do**:
  - Do NOT modify any code — this is a test-only task
  - Do NOT skip any test — all 10 must run
  - Do NOT mark a test PASS if it fails

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Requires SSH to VPS, running commands, parsing JSON, producing structured report
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (sequential, after T7)
  - **Blocks**: F1-F3 (verification wave)
  - **Blocked By**: Task 7

  **References**:

  **Pattern References**:
  - `.sisyphus/reports/vps-functional-test-2026-05-14.md` — Previous test report with exact format

  **API/Type References**:
  - VPS: `5.183.11.149`, user `root`, SSH key `~/.ssh/openclaw_win`
  - RPC methods: `bridge.status`, `matrix.status`, `config.list`

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Full test suite runs and produces report
    Tool: Bash (SSH)
    Preconditions: VPS deployed with fixed image (Task 7 complete)
    Steps:
      1. Run all 10 tests via SSH
      2. Record each result (PASS/FAIL) with details
      3. Save markdown report to .sisyphus/reports/vps-test-post-fixes-2026-05-14.md
      4. grep -c "PASS" report — assert >= 10
    Expected Result: 10/10 PASS, report file exists with all results
    Failure Indicators: Any test FAIL, report not saved, or < 10 PASS entries
    Evidence: .sisyphus/reports/vps-test-post-fixes-2026-05-14.md

  Scenario: Matrix integration shows logged_in: true (primary fix verification)
    Tool: Bash (SSH + RPC)
    Preconditions: Bridge running on VPS with T1+T2 fixes
    Steps:
      1. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'echo "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"matrix.status\"}" | docker exec -i armorclaw socat - UNIX-CONNECT:/run/armorclaw/bridge.sock'
      2. Parse response, check result.logged_in field
    Expected Result: "logged_in": true
    Failure Indicators: "logged_in": false or error response
    Evidence: .sisyphus/evidence/task-8-matrix-loggedin.txt
  ```

  **Commit**: NO (test/report task)
  - Evidence: `.sisyphus/reports/vps-test-post-fixes-2026-05-14.md`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 3 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
>
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go test ./bridge/...` + `go vet ./bridge/...`. Review all changed files for: type assertions without checks, empty catches, fmt.Printf in prod, commented-out code, unused imports. **Check password leak**: `grep -r 'password' bridge/internal/adapter/matrix.go | grep -v 'func\|//\|type\|Password string'` — must find zero raw password values in log/error statements. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Tests [N pass/N fail] | Password Leak [CLEAN/N issues] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Flag unaccounted changes. Verify `ToMatrixConfig()` was only touched if verified as used at runtime.
  Output: `Tasks [N/N compliant] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **T1+T2**: `fix(bridge): store username in MatrixAdapter for re-authentication fallback` — matrix.go, main.go
- **T4**: `fix(bridge): write attachment content before YARA scan, add graceful degradation` — ingest_server.go
- **T3**: `fix(docker): ship YARA rules in Docker image` — Dockerfile.quickstart
- **T5**: `fix(tests): align VPS test suite to registered RPC methods` — health-check.sh, test scripts
- **T7+T8**: `test(vps): rebuild, deploy, and verify all fixes on VPS` — .sisyphus/reports/

---

## Success Criteria

### Verification Commands
```bash
# Matrix auth
cd bridge && go test -v -run TestMatrixAuth ./internal/adapter/...   # Expected: PASS

# YARA pipeline
cd bridge && go test -v -run TestIngest ./pkg/email/...               # Expected: PASS

# RPC methods
cd bridge && go test -v -run TestRegistered ./pkg/rpc/...             # Expected: PASS

# Password leak check
grep -r 'password' bridge/internal/adapter/matrix.go | grep -v 'func\|//\|type\|Password string'
# Expected: zero matches (no raw password values in log/error statements)
```

### VPS Test Suite (post-deploy)
```bash
ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'echo "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"matrix.status\"}" | docker exec -i armorclaw socat - UNIX-CONNECT:/run/armorclaw/bridge.sock'
# Expected: "logged_in": true

ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker logs armorclaw 2>&1 | grep -c "YARA initialization failed"'
# Expected: 0

ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'echo "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"bridge.status\"}" | docker exec -i armorclaw socat - UNIX-CONNECT:/run/armorclaw/bridge.sock'
# Expected: valid JSON with "result" field
```

### Final Checklist
- [ ] Matrix `logged_in: true` in matrix.status response
- [ ] No YARA initialization warning in logs
- [ ] YARA temp file contains actual attachment content (unit test proof)
- [ ] No password values in any log statement or error message
- [ ] VPS test suite 10/10 PASS
- [ ] Report saved to `.sisyphus/reports/`
