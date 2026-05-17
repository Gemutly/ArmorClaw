# ArmorClaw Status Review Remediation

## TL;DR

> **Quick Summary**: External status review contained fabricated code-level claims and inaccurate metrics. This plan executes the verified fixes: blocker-response PII defense-in-depth, blindfill dead-code cleanup, admin RPC parity matrix, test-count corrections, and OpenClaw UI description fix. ArmorChat/Vodozemac work excluded — separate codebase.
>
> **Deliverables**:
> - Blocker-response PII token-reference implementation (replaces raw PII with vault tokens)
> - Rust Vault blindfill module cleaned up (deleted or documented as vestigial)
> - Admin panel RPC parity matrix documented
> - Test counts corrected across all docs
> - OpenClaw UI description corrected (not Lit)
>
> **Estimated Effort**: Medium
> **Parallel Execution**: YES — 2 waves (code fixes first, then docs)
> **Critical Path**: T1 (blocker PII) → T5 (test docs, depends on T1 test changes)

---

## Context

### Original Request
Validate external project status review, correct false claims, execute verified fixes.

### Validation Results
Four parallel explore agents verified all claims against source code:

**Fabricated (rejected)** — `requestVerification()`, `EncryptionStatus.VERIFIED`, `EncryptionMode.SERVER_SIDE` do not exist in this codebase. Origin likely Element Android confusion.

**Misleading (corrected)** — Only ONE active CDP layer (Jetski), not two. Rust Vault `CdpInterceptor` has zero production callers. Circuit breakers exist where they belong (Jetski, Bridge). OpenClaw UI is canvas-host HTTP server, not Lit web components.

**Numeric corrections** — 89 RPC methods (not 81), 161 Rust Vault tests (not 96), 106 document pipeline tests (not 105), 65 Python tests (not 72).

### Scope Boundary
- **INCLUDE**: `bridge/`, `rust-vault/`, `jetski/`, `sidecar-*`, `container/`, `applications/admin-panel/`, `applications/setup-wizard/`, `doc/`
- **EXCLUDE**: ArmorChat (`applications/ArmorChat/`), Vodozemac/E2EE audit — separate codebase

---

## Work Objectives

### Core Objective
Fix the 5 verified gaps in ArmorClaw and correct all inaccurate documentation.

### Concrete Deliverables
- `bridge/pkg/secretary/orchestrator_integration.go` — blocker PII protected
- `rust-vault/src/blindfill/` — dead code cleaned up
- Admin RPC parity matrix — documented
- `doc/sidecar-pipeline.md`, `doc/armorclaw.md` — corrected test counts
- Any OpenClaw "Lit" references — corrected

### Definition of Done
- [x] All 5 tasks completed and verified
- [x] No scope leaks into ArmorChat codebase
- [x] All existing tests still pass

### Must Have
- BlockerResponse.Input no longer holds raw PII in memory after consumption
- Blindfill module status clear (deleted or explicitly documented as vestigial)
- Test counts in docs match running test suites

### Must NOT Have (Guardrails)
- Do NOT touch `applications/ArmorChat/` — separate codebase
- Do NOT modify E2EE/crypto code — that's ArmorChat scope
- Do NOT add new RPC methods — only document parity gaps
- Do NOT change any sidecar routing logic — it works

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: YES (Go `go test`, Rust `cargo test`)
- **Automated tests**: Tests-after (fixes to existing code, add tests for new behavior)
- **Framework**: Go testing + Rust cargo test

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — code fixes, MAX PARALLEL):
├── Task 1: Blocker-response PII defense-in-depth [deep]
├── Task 2: Rust Vault blindfill dead-code cleanup [unspecified-high]
└── Task 3: Admin panel RPC parity matrix [unspecified-high]

Wave 2 (After Wave 1 — doc corrections, depends on T1 test changes):
├── Task 4: Test-count documentation corrections [quick]
└── Task 5: OpenClaw UI description correction [quick]

Wave FINAL (After ALL tasks — 3 parallel reviews):
├── F1: Scope fidelity check — verify no ArmorChat leakage [deep]
├── F2: Numeric verification — run actual test suites [unspecified-high]
└── F3: Code quality review — build + lint + test [unspecified-high]
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| T1 | — | T4 |
| T2 | — | — |
| T3 | — | — |
| T4 | T1 (test counts) | — |
| T5 | — | — |

### Agent Dispatch Summary

- **Wave 1**: T1 → `deep`, T2 → `unspecified-high`, T3 → `unspecified-high`
- **Wave 2**: T4 → `quick`, T5 → `quick`
- **FINAL**: F1 → `deep`, F2 → `unspecified-high`, F3 → `unspecified-high`

---

## TODOs

- [x] 1. Add defense-in-depth zeroization for blocker-response PII

  **What to do**:
  - `BlockerResponse.Input` in `orchestrator_integration.go:1042-1049` holds raw sensitive data in memory
  - Currently protected only by "never logged" convention — no explicit memory clearing
  - **Implementation: Token-reference (default)** — replace raw PII with a short-lived vault token that the next container step resolves. This matches BlindFill patterns, removes raw PII from the in-memory config map entirely, and avoids relying on Go string zeroization semantics (Go strings are immutable; `[]byte` zero-fill is not a reliable security primitive)
  - Do not store raw PII in `BlockerResponse.Input` longer than the current step requires
  - Only fall back to memory zeroization if token-reference is blocked by a real integration constraint (document the constraint if this occurs)
  - Update `appendBlockerResponse()` (line 1293-1312) to emit vault token instead of raw value
  - Update `TestBlockerLoop_PII_NotLogged` (line 1091-1175) to verify token-only storage

  **Must NOT do**:
  - Do not change the blocker protocol flow itself — only how PII is stored/passed
  - Do not add logging of blocker responses

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T2, T3)
  - **Blocks**: T4 (test count docs may need updating)
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:1042-1049` — BlockerResponse struct with `Input string` field
  - `bridge/pkg/secretary/orchestrator_integration.go:1293-1312` — `appendBlockerResponse()` appends raw Input into step config JSON
  - `bridge/pkg/secretary/orchestrator_integration_test.go:1091-1175` — existing PII-not-logged test
  - `bridge/pkg/secretary/result.go:98-104` — Blocker struct definition
  - `bridge/pkg/vault/` — vault client for token-reference approach

  **Acceptance Criteria**:
  - [ ] BlockerResponse.Input uses vault token references — raw PII never persists in the config map
  - [ ] Existing test `TestBlockerLoop_PII_NotLogged` updated and passes
  - [ ] New test verifies token-only storage (raw PII absent from config JSON)
  - [ ] `go test ./pkg/secretary/...` passes with 0 regressions

  **QA Scenarios**:

  ```
  Scenario: Blocker response uses token reference, not raw PII
    Tool: Bash (go test)
    Preconditions: Bridge secretary package compiles
    Steps:
      1. Run: cd bridge && go test -v -run "TestBlockerLoop_PII" ./pkg/secretary/...
      2. Assert: test passes
      3. Read orchestrator_integration.go — verify appendBlockerResponse writes vault token, not raw Input
    Expected Result: Test passes, config map contains token reference instead of raw PII
    Failure Indicators: Raw PII string visible in step config JSON, token resolution fails
    Evidence: .sisyphus/evidence/task-1-blocker-pii-test.txt

  Scenario: No regressions in secretary tests
    Tool: Bash (go test)
    Preconditions: All changes applied
    Steps:
      1. Run: cd bridge && go test ./pkg/secretary/... 2>&1
      2. Assert: 0 failures
    Expected Result: All existing tests pass, new token-reference test included
    Evidence: .sisyphus/evidence/task-1-secretary-regression.txt
  ```

  **Commit**: YES
  - Message: `fix(secretary): add defense-in-depth for blocker-response PII`
  - Files: `bridge/pkg/secretary/orchestrator_integration.go`, `bridge/pkg/secretary/orchestrator_integration_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/secretary/...`

- [x] 2. Clean up Rust Vault blindfill dead code

  **What to do**:
  - The `blindfill` module (`rust-vault/src/blindfill/`) has ZERO production callers
  - Only test files in `rust-vault/tests/` reference these modules
  - Three files: `cdp_interceptor.rs` (generates Fetch.enable params), `integration.rs` (BlindFillIntegrator), `placeholder.rs` (placeholder parsing)
  - Re-exported in `lib.rs` but never wired into any gRPC server handler
  - Choose one: (a) delete entirely, (b) add module-level doc comment marking as vestigial future-design artifact with Jetski relationship noted
  - If keeping: add `#[doc(hidden)]` on public items + clear comment block explaining: "This module is a BlindFill design artifact. The active CDP interception layer is Jetski (`jetski/internal/cdp/proxy.go`). This code has no production callers."
  - Update any docs that reference "two CDP layers" to clarify Jetski is sole active layer

  **Must NOT do**:
  - Do not delete test files — they verify the module's internal logic
  - Do not modify Jetski code
  - Do not remove the `blindfill` module from Cargo.toml if tests still need it

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T3)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `rust-vault/src/blindfill/mod.rs` — re-exports CdpInterceptor and BlindFillIntegrator
  - `rust-vault/src/blindfill/cdp_interceptor.rs` — generates Fetch.enable CDP params, resolves `{{VAULT:field:hash}}` placeholders
  - `rust-vault/src/blindfill/integration.rs` — BlindFillIntegrator resolves placeholders against VaultDb
  - `rust-vault/src/blindfill/placeholder.rs` — placeholder parsing (16 inline tests)
  - `rust-vault/tests/cdp_interceptor_test.rs` — 6 tests, only non-module caller
  - `rust-vault/tests/blindfill_integration_test.rs` — 4 tests, only non-module caller
  - `rust-vault/src/lib.rs` — check how blindfill module is declared

  **Acceptance Criteria**:
  - [ ] Module status clear: either deleted OR documented as vestigial with Jetski relationship
  - [ ] If kept: `#[doc(hidden)]` on public items + module-level doc comment
  - [ ] `cargo test --all` in rust-vault passes with 0 regressions
  - [ ] No doc references "two CDP layers" without clarifying Jetski is sole active layer

  **QA Scenarios**:

  ```
  Scenario: Rust Vault tests pass after cleanup
    Tool: Bash (cargo test)
    Preconditions: Rust toolchain available
    Steps:
      1. Run: cd rust-vault && cargo test --all 2>&1
      2. Assert: "test result: ok" with 0 failures
    Expected Result: All 161 tests pass
    Evidence: .sisyphus/evidence/task-2-vault-tests.txt

  Scenario: No production callers of blindfill remain
    Tool: Bash (grep)
    Preconditions: Cleanup complete
    Steps:
      1. Run: grep -rn "CdpInterceptor\|BlindFillIntegrator" rust-vault/src/ --include="*.rs" | grep -v "mod.rs\|blindfill/"
      2. Assert: zero matches (only module-internal references remain)
    Expected Result: No production code references blindfill types
    Evidence: .sisyphus/evidence/task-2-blindfill-callers.txt
  ```

  **Commit**: YES
  - Message: `chore(vault): clean up blindfill module — mark as vestigial design artifact`
  - Files: `rust-vault/src/blindfill/mod.rs`, `rust-vault/src/lib.rs`, `doc/armorclaw.md`
  - Pre-commit: `cd rust-vault && cargo test --all`

- [x] 3. Document admin panel RPC endpoint parity matrix

  **What to do**:
  - The admin panel (`applications/admin-panel/src/services/bridgeApi.ts`, 553 lines) calls RPC methods
  - The main Bridge RPC server has 89 registered handlers (`bridge/pkg/rpc/server.go:857-951`)
  - Cross-reference every method called in `bridgeApi.ts` against the handler map
  - Some methods may be handled by separate subsystems: lockdown package, enforcement package (`enforcement/rpc_handlers.go`), public handlers (`public_handlers.go`)
  - Produce a parity matrix: method name → handler status (registered/missing/subsystem)
  - Document gaps with recommended next steps

  **Must NOT do**:
  - Do not implement missing RPC methods — only document gaps
  - Do not modify admin panel code

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `applications/admin-panel/src/services/bridgeApi.ts` — admin panel RPC client (553 lines)
  - `bridge/pkg/rpc/server.go:857-951` — main handler map (89 methods)
  - `bridge/pkg/rpc/public_handlers.go` — 5 additional public methods (system.health, system.config, system.info, system.time, device.validate)
  - `bridge/pkg/enforcement/rpc_handlers.go` — 6 enforcement handlers (license.status, license.features, license.check_feature, compliance.status, platform.limits, platform.check)
  - `bridge/internal/lockdown/` — lockdown subsystem handlers (lockdown.status, lockdown.get_challenge, lockdown.claim_ownership, lockdown.transition, security.*)

  **Acceptance Criteria**:
  - [ ] Parity matrix produced: every admin-panel method → bridge handler → status
  - [ ] Gaps clearly marked with recommended action
  - [ ] Saved to `.sisyphus/drafts/admin-rpc-parity.md`

  **QA Scenarios**:

  ```
  Scenario: Parity matrix covers all admin panel methods
    Tool: Bash (grep)
    Preconditions: bridgeApi.ts read
    Steps:
      1. Extract all RPC method strings from bridgeApi.ts
      2. Cross-reference against server.go handler map + public_handlers + enforcement + lockdown
      3. Verify every method has a status entry
    Expected Result: 100% coverage of admin panel methods in parity matrix
    Evidence: .sisyphus/evidence/task-3-parity-matrix.txt
  ```

  **Commit**: YES
  - Message: `docs(admin): document RPC endpoint parity matrix`
  - Files: `.sisyphus/drafts/admin-rpc-parity.md`
  - Pre-commit: none (documentation only)

- [x] 4. Correct test-count documentation across repo

  **What to do**:
  - `doc/sidecar-pipeline.md` test coverage table: update total from 90 to 106, fix breakdown:
    - Go routing: 22 (not 18 or 21)
    - Go E2E (Python sidecar): 7
    - Go E2E (Java sidecar): 4
    - Java JUnit: 8
    - Python: 65 (27+16+12+10)
  - `doc/armorclaw.md` test count row (~line 3641): current docs say Python 72 — **actual is 65** (27+16+12+10). Java sidecar is 12 (8 JUnit + 4 Go E2E). Update both to match validated counts.
  - Any Rust Vault test references: update from 96 to 161
  - Verify counts by running actual test suites before updating docs — do not trust any previously-documented number

  **Must NOT do**:
  - Do not change any test code — only documentation

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (but ideally after T1 in case T1 adds/changes tests)
  - **Parallel Group**: Wave 2 (with T5)
  - **Blocks**: None
  - **Blocked By**: T1 (test count depends on T1 test changes)

  **References**:
  - `doc/sidecar-pipeline.md:432-440` — test coverage table (currently shows 90 total)
  - `doc/armorclaw.md:3641` — test count row
  - Rust Vault: 161 `#[test]`/`#[tokio::test]` across 20 files (verified by explore agent)

  **Acceptance Criteria**:
  - [ ] All test count references in docs match actual codebase
  - [ ] Run `cd bridge && go test -v ./pkg/sidecar/...` to confirm Go count
  - [ ] Run `cd sidecar-python && python -m pytest --co -q` to confirm Python count

  **QA Scenarios**:

  ```
  Scenario: Documentation matches running test suites
    Tool: Bash (go test + pytest)
    Preconditions: Docs updated
    Steps:
      1. Run: cd bridge && go test -v -count=1 ./pkg/sidecar/... 2>&1 | grep -c "^--- PASS\|^--- FAIL"
      2. Run: cd sidecar-python && python -m pytest --co -q 2>&1 | tail -1
      3. Compare counts against doc/sidecar-pipeline.md table
    Expected Result: Documented counts match actual within ±2 (parametrized test expansion variance)
    Evidence: .sisyphus/evidence/task-4-test-counts.txt
  ```

  **Commit**: YES
  - Message: `docs(tests): correct test counts across documentation`
  - Files: `doc/sidecar-pipeline.md`, `doc/armorclaw.md`
  - Pre-commit: none (documentation)

- [x] 5. Correct OpenClaw UI description in docs

  **What to do**:
  - Search all docs for references to "Lit web components" in context of OpenClaw UI
  - Correct to "canvas-host HTTP server with pre-built bundle (`a2ui.bundle.js`)"
  - The actual architecture: `container/openclaw-src/src/canvas-host/server.ts` (478 lines) serves a pre-built JS bundle via HTTP
  - No Lit dependencies exist in the source tree (verified: zero matches for `LitElement`, `@lit`, `lit-element`, `customElements.define`)

  **Must NOT do**:
  - Do not change OpenClaw source code — only documentation
  - Do not investigate what `a2ui.bundle.js` contains beyond confirming it's not Lit

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T4)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `container/openclaw-src/src/canvas-host/server.ts` — HTTP server (478 lines)
  - `container/openclaw-src/src/canvas-host/a2ui.ts` — asset resolution (209 lines)
  - `doc/armorclaw.md` — component status matrix (may reference OpenClaw UI)

  **Acceptance Criteria**:
  - [ ] No docs reference "Lit web components" for OpenClaw UI
  - [ ] Accurate canvas-host description documented where OpenClaw UI is described
  - [ ] `grep -r "Lit" container/openclaw-src/` returns nothing (verification)

  **QA Scenarios**:

  ```
  Scenario: No Lit references in docs or code
    Tool: Bash (grep)
    Preconditions: Docs updated
    Steps:
      1. Run: grep -rn "Lit web\|LitElement\|lit-element" doc/ container/openclaw-src/
      2. Assert: zero matches
    Expected Result: No Lit references found
    Evidence: .sisyphus/evidence/task-5-no-lit-refs.txt
  ```

  **Commit**: YES
  - Message: `docs(openclaw): correct UI framework description`
  - Files: relevant doc files
  - Pre-commit: `grep -rn "Lit web\|LitElement" doc/ container/openclaw-src/` returns 0

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

- [x] F1. **Scope Fidelity Check** — `deep`
  Verify NO ArmorChat-specific changes leaked in. Every file modified must be within ArmorClaw scope. Check for any references to `applications/ArmorChat/` in diffs. If found, REJECT and flag.

- [x] F2. **Numeric Verification** — `unspecified-high`
  Run actual test suites and compare output against documentation:
  ```bash
  cd rust-vault && cargo test --all 2>&1 | grep "test result"
  cd bridge && go test ./pkg/sidecar/... 2>&1 | grep "^---"
  cd sidecar-python && python -m pytest --co -q 2>&1 | tail -1
  cd sidecar-java && JAVA_HOME="$(asdf where java temurin-21.0.11+10.0.LTS)" mvn test
  ```

- [x] F3. **Code Quality Review** — `unspecified-high`
  `go test ./...` in bridge, `cargo test --all` in rust-vault. Check for regressions. Verify no `as any`/`unwrap()` in changed code.

---

## Commit Strategy

| Task | Message | Files | Pre-commit |
|------|---------|-------|------------|
| T1 | `fix(secretary): add defense-in-depth for blocker-response PII` | `bridge/pkg/secretary/orchestrator_integration.go`, `..._test.go` | `cd bridge && go test ./pkg/secretary/...` |
| T2 | `chore(vault): mark blindfill as vestigial design artifact` | `rust-vault/src/blindfill/mod.rs`, `doc/armorclaw.md` | `cd rust-vault && cargo test --all` |
| T3 | `docs(admin): document RPC endpoint parity matrix` | `.sisyphus/drafts/admin-rpc-parity.md` | none |
| T4 | `docs(tests): correct test counts across documentation` | `doc/sidecar-pipeline.md`, `doc/armorclaw.md` | none |
| T5 | `docs(openclaw): correct UI framework description` | relevant docs | `grep -rn "Lit web" doc/` |

---

## Success Criteria

### Verification Commands
```bash
# Secretary tests pass (including new PII test)
cd bridge && go test -v ./pkg/secretary/... 2>&1 | grep "PASS\|FAIL"

# Rust Vault tests pass (all 161)
cd rust-vault && cargo test --all 2>&1 | grep "test result"

# Document pipeline tests unchanged
cd bridge && go test -v -count=1 ./pkg/sidecar/... 2>&1 | grep -c "PASS"

# No ArmorChat files in diff
git diff --name-only | grep "ArmorChat"  # Should return nothing
```

### Final Checklist
- [x] All 5 tasks completed
- [x] Zero regressions in any test suite
- [x] No ArmorChat files touched
- [x] All fabricated claims documented and rejected in Context section
- [x] Test counts in docs match running suites
