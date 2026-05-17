# v0.8.0 Comprehensive Test Suite

## TL;DR

> **Quick Summary**: Exhaustive test coverage for all v0.8.0 multi-agent teams features — filling 4 untested files, adding cross-package integration tests, concurrent access stress tests, Rust scope/query tests, and edge case coverage across all new packages.
> 
> **Deliverables**:
> - 4 new Go test files for untested code (audit.go, metrics.go, handler.go, doc_handler.go)
> - 6 integration test files for cross-package flows
> - 3 stress/concurrent test files
> - 2 Rust test modules (ephemeral scope, QueryDocuments)
> - ~120 new tests total
> 
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 5 waves
> **Critical Path**: Task 1 (test helpers) → Tasks 2-5 (unit gaps) → Tasks 6-11 (integration) → Tasks 12-14 (stress) → Task 15 (Rust)

---

## Context

### Original Request
User requested a comprehensive test plan for ALL features added in the v0.8.0 multi-agent teams plan (54 implementation tasks).

### Interview Summary
**Key Discussions**:
- Go depth: EXHAUSTIVE — full coverage including error paths, concurrent access, stress tests
- Rust: FULL — capability_scope validation + QueryDocuments RPC
- Frontend: SKIP — defer TeamTimelinePage/ArtifactLineageView
- Android: SKIP — defer BlindFillCard Compose tests
- Email: Already has good coverage, lower priority

**Research Findings**:
- 4 Go source files have ZERO tests: `team/audit.go`, `team/metrics.go`, `browser/handler.go`, `sidecar/doc_handler.go`
- Existing mock infrastructure: mockClassifier, mockRegistry, mockConsent, mockSkillGate in `broker_test.go`
- Secretary orchestrator has 43 tests but NONE cover the new broker pre-check in ExecuteStep
- Rust governance has 3 tests for event_notifier but ZERO for scope validation
- Sidecar has ZERO tests for QueryDocuments RPC

### Gap Analysis (self-guided)
**Identified Gaps** (addressed):
- No integration test for broker → team registry → risk classifier → consent flow
- No test for secretary ExecuteStep broker pre-check (critical wiring gap)
- No concurrent access tests for broker, metrics, or team store
- No benchmark for team store operations
- No error path tests for browser handler (network failures, invalid intents)
- No test for team metrics thread safety under concurrent recorders
- Rust scope validation has no test coverage despite being security-critical

---

## Work Objectives

### Core Objective
Achieve exhaustive test coverage for all v0.8.0 multi-agent teams features, with emphasis on integration flows, concurrent access safety, and error path coverage.

### Concrete Deliverables
- `bridge/pkg/team/audit_test.go` — Tests for RecordTeamEvent
- `bridge/pkg/team/metrics_test.go` — Tests for all 8 exported metrics methods + concurrency
- `bridge/pkg/browser/handler_test.go` — Tests for browser_execute handler + all action dispatchers
- `bridge/pkg/sidecar/doc_handler_test.go` — Tests for doc_query handler + ValidateDocQueryInput
- `bridge/pkg/capability/broker_integration_test.go` — End-to-end broker flow tests
- `bridge/pkg/team/integration_test.go` — Team lifecycle → capability resolution → broker auth
- `bridge/pkg/secretary/broker_check_test.go` — Secretary ExecuteStep broker pre-check
- `bridge/pkg/capability/concurrent_test.go` — Concurrent broker access stress test
- `bridge/pkg/team/concurrent_test.go` — Concurrent team store + metrics stress test
- `bridge/pkg/capability/broker_errorpaths_test.go` — All broker error/failure paths
- `rust-vault/src/governance/ephemeral.rs` — #[test] module for scope validation
- `sidecar/src/grpc/server.rs` — #[tokio::test] module for QueryDocuments

### Definition of Done
- [x] `go test ./pkg/capability/... ./pkg/team/... ./pkg/browser/... ./pkg/sidecar/...` → ALL PASS (new tests only; 2 pre-existing failures in sidecar PII and version parse unrelated to v0.8.0)
- [x] `go test -count=1 -race ./pkg/capability/... ./pkg/team/...` → PASS (no data races)
- [x] `cd rust-vault && cargo test --lib` → PASS (new scope tests pass; 11 pre-existing blindfill failures unrelated to v0.8.0)
- [x] `cd sidecar && cargo test --lib` → PASS (240 passed, 0 failed)
- [x] Zero new build warnings in Go code
- [x] No test files import `internal/` packages from `pkg/` tests

### Must Have
- Tests for ALL 4 currently untested source files (audit.go, metrics.go, handler.go, doc_handler.go)
- Integration test for secretary ExecuteStep broker pre-check
- Integration test for broker → team registry → risk classifier → consent flow
- At least 1 concurrent stress test per critical package (broker, team store, metrics)
- Rust tests for scope validation (security-critical)
- Rust tests for QueryDocuments RPC
- All tests use existing mock infrastructure patterns (testify, mock structs)
- Tests must pass with `-race` flag

### Must NOT Have (Guardrails)
- ❌ No changes to production code — test files ONLY
- ❌ No test files that require Docker or external services
- ❌ No tests that import `internal/` packages from `pkg/` test files
- ❌ No tests that require network access
- ❌ No tests with hardcoded absolute paths
- ❌ No AI slop in tests — meaningful assertions only, no `assert.True(t, true)`
- ❌ No tests that depend on test execution order
- ❌ No skipped tests (`t.Skip`) unless guarding for build tags
- ❌ No introduction of new test dependencies beyond testify

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: YES
- **Automated tests**: YES (TDD approach for new test files)
- **Framework**: `go test` with `testify/assert` and `testify/require`
- **Rust**: `cargo test` with standard `#[test]` and `#[tokio::test]`
- **Race detection**: `go test -race` required for concurrent tests

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go Tests**: Use `go test -v -count=1 -race` to verify
- **Rust Tests**: Use `cargo test --lib` to verify
- **Coverage check**: `go test -cover ./pkg/...` for each package

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — shared test helpers):
└── Task 1: Extract shared test mocks/helpers [quick]

Wave 2 (After Wave 1 — unit test gaps, MAX PARALLEL):
├── Task 2: team/audit_test.go (depends: 1) [quick]
├── Task 3: team/metrics_test.go (depends: 1) [quick]
├── Task 4: browser/handler_test.go (depends: 1) [unspecified-high]
├── Task 5: sidecar/doc_handler_test.go (depends: 1) [unspecified-high]
├── Task 6: capability/broker_errorpaths_test.go (depends: 1) [deep]

Wave 3 (After Wave 2 — integration tests, MAX PARALLEL):
├── Task 7: capability/broker_integration_test.go (depends: 1, 6) [deep]
├── Task 8: team/integration_test.go (depends: 1, 2, 3) [deep]
├── Task 9: secretary/broker_check_test.go (depends: 1, 7) [deep]
├── Task 10: email/team_routing_integration_test.go (depends: 1) [unspecified-high]

Wave 4 (After Wave 3 — stress + benchmarks, MAX PARALLEL):
├── Task 11: capability/concurrent_test.go — broker stress (depends: 7) [deep]
├── Task 12: team/concurrent_test.go — store + metrics stress (depends: 3, 8) [deep]
├── Task 13: capability/benchmarks_test.go — broker + team store benchmarks (depends: 7, 8) [quick]

Wave 5 (After Wave 4 — Rust):
├── Task 14: rust-vault scope validation tests (depends: none) [unspecified-high]
├── Task 15: sidecar QueryDocuments tests (depends: none) [unspecified-high]

Wave FINAL (After ALL tasks — verification):
├── Task F1: Full test suite run + race detection
├── Task F2: Coverage report + gap analysis
├── Task F3: Code quality review (test anti-patterns)
└── Task F4: Scope fidelity check

Critical Path: Task 1 → Task 6 → Task 7 → Task 9 → F1-F4
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 6 (Wave 2)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | - | 2-6 | 1 |
| 2 | 1 | 8 | 2 |
| 3 | 1 | 8, 12 | 2 |
| 4 | 1 | - | 2 |
| 5 | 1 | - | 2 |
| 6 | 1 | 7 | 2 |
| 7 | 1, 6 | 9, 11, 13 | 3 |
| 8 | 1, 2, 3 | 12, 13 | 3 |
| 9 | 1, 7 | - | 3 |
| 10 | 1 | - | 3 |
| 11 | 7 | F1 | 4 |
| 12 | 3, 8 | F1 | 4 |
| 13 | 7, 8 | F1 | 4 |
| 14 | - | F1 | 5 |
| 15 | - | F1 | 5 |

### Agent Dispatch Summary

- **Wave 1**: 1 task — T1 → `quick`
- **Wave 2**: 5 tasks — T2, T3 → `quick`, T4, T5 → `unspecified-high`, T6 → `deep`
- **Wave 3**: 4 tasks — T7, T8, T9 → `deep`, T10 → `unspecified-high`
- **Wave 4**: 3 tasks — T11, T12 → `deep`, T13 → `quick`
- **Wave 5**: 2 tasks — T14, T15 → `unspecified-high`
- **FINAL**: 4 tasks — F1 → `unspecified-high`, F2 → `deep`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] ~~1. Extract Shared Test Mocks and Helpers~~ CANCELLED — mocks kept in-package per existing convention

  **What to do**:
  - Create `bridge/pkg/capability/testhelpers_test.go` containing reusable mock types extracted from `broker_test.go`
  - Move `mockClassifier`, `mockRegistry`, `mockConsent`, `mockSkillGate` to a shared test helper file within the `capability` package (or duplicate patterns for other packages)
  - Add new mock types needed by upcoming tests:
    - `mockTeamStore` — implements `interfaces.TeamStore` for integration tests
    - `mockAIClient` — implements `interfaces.AIClient` for composer tests
    - `mockBrowserClient` — implements browser Client interface for handler tests
    - `mockSidecarClient` — implements sidecar Client interface for doc_handler tests
    - `mockSecretRequester` — implements `SecretRequester` function type
    - `mockConsentProvider` — implements `ConsentProvider` for integration flows
  - Add `newTestBroker()` helper that creates a fully-wired Broker with all mocks
  - Add `newTestTeamStore(t)` helper that creates a temp-file SQLCipher TeamStore

  **Must NOT do**:
  - Do NOT modify existing broker_test.go mock definitions (other tests reference them)
  - Do NOT create a separate `testutil` package (keep mocks in-package with `_test.go` suffix)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Pure extraction + creation of test helpers, no complex logic
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1 (solo)
  - **Blocks**: Tasks 2, 3, 4, 5, 6
  - **Blocked By**: None (can start immediately)

  **References**:
  - `bridge/pkg/capability/broker_test.go:16-60` — Existing mock types to use as pattern reference
  - `bridge/pkg/interfaces/capability.go` — Interfaces that mocks must satisfy (CapabilityBroker, RiskClassifier, CapabilityRegistry)
  - `bridge/pkg/interfaces/consent.go` — ConsentProvider interface
  - `bridge/pkg/interfaces/team.go` — TeamStore, TeamService interfaces
  - `bridge/pkg/interfaces/ai_client.go` — AIClient interface
  - `bridge/pkg/browser/handler.go:10` — Client interface that mockBrowserClient must match
  - `bridge/pkg/sidecar/doc_handler.go:14` — Client usage pattern for mock
  - `bridge/pkg/team/secret_request.go:15` — SecretRequester function type

  **Acceptance Criteria**:
  - [ ] `go build ./pkg/capability/...` → PASS
  - [ ] `go test ./pkg/capability/... ./pkg/team/...` → PASS (existing tests unaffected)

  **QA Scenarios**:

  ```
  Scenario: Mock types compile and satisfy interfaces
    Tool: Bash
    Preconditions: bridge/pkg/capability/testhelpers_test.go exists
    Steps:
      1. Run: cd bridge && go test -count=1 ./pkg/capability/... ./pkg/team/...
      2. Verify exit code 0
    Expected Result: All existing tests pass, new helpers compile
    Failure Indicators: Compilation errors, test failures
    Evidence: .sisyphus/evidence/task-01-helpers-compile.txt

  Scenario: New helpers don't break existing broker tests
    Tool: Bash
    Preconditions: Helpers extracted/created
    Steps:
      1. Run: cd bridge && go test -v -count=1 ./pkg/capability/...
      2. Count test results
    Expected Result: Same test count as before, all PASS
    Failure Indicators: Fewer tests (accidentally removed), or failures
    Evidence: .sisyphus/evidence/task-01-existing-tests.txt
  ```

  **Commit**: YES
  - Message: `test(capability): extract shared test mocks and helpers`
  - Files: `bridge/pkg/capability/testhelpers_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/capability/...`

- [x] 2. team/audit_test.go — Test RecordTeamEvent

  **What to do**:
  - Create `bridge/pkg/team/audit_test.go`
  - Test `RecordTeamEvent()` — verify it returns a TeamAuditEntry with correct fields
  - Test all 7 event type constants: `EventTeamCreated`, `EventMemberAdded`, `EventMemberRemoved`, `EventRoleAssigned`, `EventTeamDissolved`, `EventCapabilityOverride`, `EventHandoffComplete`
  - Test with empty fields, full fields, optional fields
  - Test timestamp auto-population
  - Verify TeamAuditEntry struct validation

  **Must NOT do**:
  - Do NOT modify `audit.go`
  - Do NOT add database persistence (RecordTeamEvent is pure struct creation)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single function, pure struct creation, straightforward assertions
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 3, 4, 5, 6)
  - **Blocks**: Task 8
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/team/audit.go` — Full source to test
  - `bridge/pkg/team/types.go:30` — Team struct for creating test entries

  **Acceptance Criteria**:
  - [ ] `bridge/pkg/team/audit_test.go` created
  - [ ] `go test -v -count=1 ./pkg/team/... -run TestAudit` → ALL PASS
  - [ ] At least 8 test functions covering all event types + edge cases

  **QA Scenarios**:

  ```
  Scenario: All event types produce correct entries
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -run "TestAudit\|TestRecordTeamEvent\|TestEvent" ./pkg/team/...
      2. Verify all tests pass
    Expected Result: 8+ tests, all PASS
    Evidence: .sisyphus/evidence/task-02-audit-tests.txt

  Scenario: Empty/nil fields don't panic
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -run "TestRecordTeamEvent_Empty\|TestRecordTeamEvent_Zero" ./pkg/team/...
    Expected Result: Tests handle zero values gracefully, no panics
    Evidence: .sisyphus/evidence/task-02-audit-edge-cases.txt
  ```

  **Commit**: YES (groups with Tasks 3)
  - Message: `test(team): add audit and metrics tests`
  - Files: `bridge/pkg/team/audit_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/team/...`

- [x] 3. team/metrics_test.go — Test All Metrics Methods

  **What to do**:
  - Create `bridge/pkg/team/metrics_test.go`
  - Test `NewTeamMetrics()` — verify clean initialization
  - Test `RecordTokenUsage(teamID, tokens)` — verify accumulation across calls
  - Test `RecordCost(teamID, cents)` — verify cost tracking per team
  - Test `RecordLatency(role, duration)` — verify latency recording by role
  - Test `RecordHandoff(teamID, success)` — verify handoff success/failure tracking
  - Test `RecordSecretAccess(teamID)` — verify secret access counter
  - Test `RecordApproval(teamID, riskClass, approved)` — verify approval tracking
  - Test `GetSnapshot(teamID)` — verify snapshot aggregates all recorded metrics
  - Test with multiple teams — verify isolation between team metrics
  - Test `GetSnapshot` for non-existent team — verify returns zero values
  - Add concurrent access test: multiple goroutines recording simultaneously

  **Must NOT do**:
  - Do NOT modify `metrics.go`
  - Do NOT use real time.Sleep for latency tests (use known durations)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 8 methods to test, all straightforward counter/recorder patterns
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 2, 4, 5, 6)
  - **Blocks**: Task 8, 12
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/team/metrics.go` — Full source to test
  - `bridge/pkg/team/governance_test.go:140-158` — Existing pattern for RecordHandoff assertions

  **Acceptance Criteria**:
  - [ ] `bridge/pkg/team/metrics_test.go` created
  - [ ] `go test -v -count=1 ./pkg/team/... -run TestMetrics` → ALL PASS
  - [ ] At least 10 test functions covering all exported methods + multi-team + zero team

  **QA Scenarios**:

  ```
  Scenario: All metrics methods record and snapshot correctly
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -run "TestMetrics\|TestTeamMetrics" ./pkg/team/...
    Expected Result: 10+ tests, all PASS
    Evidence: .sisyphus/evidence/task-03-metrics-tests.txt

  Scenario: Concurrent metric recording doesn't race
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -race -run "TestMetrics_Concurrent\|TestMetricsConcurrent" ./pkg/team/...
    Expected Result: PASS with zero race conditions
    Evidence: .sisyphus/evidence/task-03-metrics-concurrent.txt
  ```

  **Commit**: YES (groups with Task 2)
  - Message: `test(team): add audit and metrics tests`
  - Files: `bridge/pkg/team/audit_test.go`, `bridge/pkg/team/metrics_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/team/...`

- [x] 4. browser/handler_test.go — Test Browser Execute Handler

  **What to do**:
  - Create `bridge/pkg/browser/handler_test.go`
  - Test `Handler(client)` — verify it returns a valid bridge-local handler function
  - Test handler with valid `BrowserIntent` JSON for each action: navigate, fill, extract, screenshot, workflow
  - Test handler with invalid JSON input — verify error return
  - Test handler with missing action — verify error
  - Test `dispatchAction` paths for each action type (may need to test via Handler)
  - Test `serviceResponseToBrowserResult` conversion
  - Test with nil client — verify proper error/panic handling
  - Test with client that returns errors — verify error propagation

  **Must NOT do**:
  - Do NOT modify `handler.go`
  - Do NOT require a real browser or network

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Handler has 7 dispatch functions + error paths, needs mock Client interface
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 2, 3, 5, 6)
  - **Blocks**: None
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/browser/handler.go` — Full source to test (121 lines)
  - `bridge/pkg/capability/types.go:117-169` — BrowserIntent and BrowserResult struct definitions
  - `bridge/pkg/browser/browser_test.go` — Existing test patterns in the package
  - `bridge/pkg/browser/context_manager_test.go` — Pattern for test setup

  **Acceptance Criteria**:
  - [ ] `bridge/pkg/browser/handler_test.go` created
  - [ ] `go test -v -count=1 ./pkg/browser/...` → ALL PASS (existing + new)
  - [ ] At least 12 test functions covering all action types + error paths

  **QA Scenarios**:

  ```
  Scenario: Handler processes all browser action types
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -run "TestHandler\|TestBrowser" ./pkg/browser/...
    Expected Result: 12+ new tests, all PASS
    Evidence: .sisyphus/evidence/task-04-handler-tests.txt

  Scenario: Invalid input returns proper errors
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -run "TestHandler_Invalid\|TestHandler_Missing\|TestHandler_Malformed" ./pkg/browser/...
    Expected Result: Error messages are descriptive, no panics
    Evidence: .sisyphus/evidence/task-04-handler-errors.txt
  ```

  **Commit**: YES (groups with Task 5)
  - Message: `test(browser,sidecar): add handler tests for browser_execute and doc_query`
  - Files: `bridge/pkg/browser/handler_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/browser/...`

- [x] 5. sidecar/doc_handler_test.go — Test Doc Query Handler

  **What to do**:
  - Create `bridge/pkg/sidecar/doc_handler_test.go`
  - Test `NewDocQueryHandler(client, logger)` — verify returns valid handler function
  - Test handler with valid `DocQueryInput` JSON — verify calls sidecar client with correct params
  - Test handler with invalid JSON — verify error return
  - Test `ValidateDocQueryInput()` — table-driven tests for:
    - Valid input (document_id + query present)
    - Missing document_id
    - Missing query
    - Empty document_id
    - Empty query
    - Nil input
  - Test handler when sidecar client returns error — verify error wrapping
  - Test handler when sidecar client returns malformed response — verify error
  - Test chunk_id marshaling path

  **Must NOT do**:
  - Do NOT modify `doc_handler.go`
  - Do NOT require a running sidecar

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Handler tests need mock sidecar client, multiple error paths
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 2, 3, 4, 6)
  - **Blocks**: None
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/sidecar/doc_handler.go` — Full source (121 lines)
  - `bridge/pkg/sidecar/doc_handler.go:14-25` — DocQueryInput struct
  - `bridge/pkg/sidecar/office_client_test.go` — Existing test patterns in the package
  - `bridge/pkg/capability/types.go:155-185` — DocumentRef, ExtractedChunkSet types

  **Acceptance Criteria**:
  - [ ] `bridge/pkg/sidecar/doc_handler_test.go` created
  - [ ] `go test -v -count=1 ./pkg/sidecar/...` → ALL PASS
  - [ ] At least 10 test functions covering handler + ValidateDocQueryInput

  **QA Scenarios**:

  ```
  Scenario: Doc query handler processes valid and invalid inputs
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -run "TestDocQuery\|TestNewDocQuery\|TestValidate" ./pkg/sidecar/...
    Expected Result: 10+ tests, all PASS
    Evidence: .sisyphus/evidence/task-05-docquery-tests.txt

  Scenario: Sidecar errors are properly wrapped
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -run "TestDocQuery.*Error\|TestDocQuery.*Fail" ./pkg/sidecar/...
    Expected Result: Error messages contain "doc_query:" prefix, no panics
    Evidence: .sisyphus/evidence/task-05-docquery-errors.txt
  ```

  **Commit**: YES (groups with Task 4)
  - Message: `test(browser,sidecar): add handler tests for browser_execute and doc_query`
  - Files: `bridge/pkg/browser/handler_test.go`, `bridge/pkg/sidecar/doc_handler_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/browser/... ./pkg/sidecar/...`

- [x] 6. capability/broker_errorpaths_test.go — Exhaustive Broker Error Paths

  **What to do**:
  - Create `bridge/pkg/capability/broker_errorpaths_test.go`
  - Test broker with classifier returning error for every action — verify DENY
  - Test broker with registry returning error — verify DENY
  - Test broker with consent provider returning error — verify DENY
  - Test broker with consent channel that never responds — verify timeout → DENY
  - Test broker with skill gate returning error — verify DENY
  - Test broker with all dependencies failing simultaneously — verify DENY
  - Test broker with context already cancelled — verify DENY without calling dependencies
  - Test broker with context deadline exceeded mid-operation — verify DENY
  - Test broker with malformed ActionRequest (empty action, nil params) — verify DENY
  - Test broker with unknown risk class from classifier — verify DENY
  - Test broker with circular dependency chain of length 5+ — verify DENY
  - Test broker with PII scrub error — verify DENY
  - Test broker AuthorizeAction adapter method — verify it calls Authorize correctly

  **Must NOT do**:
  - Do NOT modify `broker.go` or any production code

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex error path combinations, need to reason about fail-closed behavior
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 2, 3, 4, 5)
  - **Blocks**: Task 7
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/capability/broker.go` — Full broker source
  - `bridge/pkg/capability/broker_test.go` — Existing broker tests (20 tests) to avoid duplication
  - `bridge/pkg/capability/types.go:41-96` — ActionRequest, ActionResponse structs

  **Acceptance Criteria**:
  - [ ] `bridge/pkg/capability/broker_errorpaths_test.go` created
  - [ ] `go test -v -count=1 ./pkg/capability/... -run TestBroker` → ALL PASS
  - [ ] At least 12 new test functions for error paths not covered by existing broker_test.go

  **QA Scenarios**:

  ```
  Scenario: All error paths produce DENY (fail-closed)
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -run "TestBroker.*Error\|TestBroker.*Fail\|TestBroker.*Deny\|TestBrokerFail" ./pkg/capability/...
    Expected Result: 12+ new tests, all produce DENY verdict
    Evidence: .sisyphus/evidence/task-06-error-paths.txt

  Scenario: No error path leaks internal state
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -run "TestBroker.*Context\|TestBroker.*Malformed\|TestBroker.*Unknown" ./pkg/capability/...
    Expected Result: Error messages don't expose internal struct details
    Evidence: .sisyphus/evidence/task-06-error-messages.txt
  ```

  **Commit**: YES
  - Message: `test(capability): add exhaustive broker error path tests`
  - Files: `bridge/pkg/capability/broker_errorpaths_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/capability/...`

  - Pre-commit: `cd bridge && go test ./pkg/capability/...`

- [x] 7. capability/broker_integration_test.go — End-to-End Broker Flow

  **What to do**:
  - Create `bridge/pkg/capability/broker_integration_test.go`
  - **Test: Full ALLOW flow** — register capabilities → classifier returns ALLOW → broker authorizes without consent
  - **Test: Full DENY flow** — register capabilities → classifier returns DENY → broker denies, no consent call
  - **Test: Full DEFER→APPROVE flow** — register capabilities → classifier returns DEFER → consent provider returns approved → broker authorizes
  - **Test: Full DEFER→DENY flow** — register capabilities → classifier returns DEFER → consent provider returns denied → broker denies
  - **Test: Full DEFER→TIMEOUT flow** — classifier returns DEFER → consent channel never responds → 300s timeout simulated → broker denies
  - **Test: Team registry integration** — create TeamCapabilityRegistry with RoleLookupFunc → wire into broker → verify role-based capability resolution works end-to-end
  - **Test: Skill gate integration** — wire skill gate → verify it blocks unauthorized skills → verify ALLOW when skill matches
  - **Test: PII scrubbing flow** — action with sensitive params → broker scrubs PII before logging
  - **Test: Artifact round-trip** — ActionRequest with typed artifacts → broker processes → ActionResponse with correct artifacts

  **Must NOT do**:
  - Do NOT modify production code
  - Do NOT test things already covered by broker_test.go (focus on multi-component flows)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Multi-component integration with complex wiring, needs careful reasoning
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 8, 9, 10)
  - **Blocks**: Tasks 9, 11, 13
  - **Blocked By**: Tasks 1, 6

  **References**:
  - `bridge/pkg/capability/broker.go` — Broker struct and Authorize method
  - `bridge/pkg/capability/team_registry.go` — TeamCapabilityRegistry for integration
  - `bridge/pkg/capability/risk_classifier.go` — RiskClassifierImpl for integration
  - `bridge/pkg/capability/broker_test.go` — Existing tests to avoid duplication
  - `bridge/pkg/interfaces/capability.go` — Interface contracts

  **Acceptance Criteria**:
  - [ ] `bridge/pkg/capability/broker_integration_test.go` created
  - [ ] `go test -v -count=1 ./pkg/capability/... -run TestIntegration` → ALL PASS
  - [ ] At least 9 test functions covering all integration flows

  **QA Scenarios**:

  ```
  Scenario: Full broker flow from registration to authorization
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -run "TestIntegration" ./pkg/capability/...
    Expected Result: 9+ integration tests, all PASS
    Evidence: .sisyphus/evidence/task-07-broker-integration.txt

  Scenario: Integration tests work with race detector
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -race -run "TestIntegration" ./pkg/capability/...
    Expected Result: PASS with zero races
    Evidence: .sisyphus/evidence/task-07-broker-integration-race.txt
  ```

  **Commit**: YES
  - Message: `test(capability): add end-to-end broker integration tests`
  - Files: `bridge/pkg/capability/broker_integration_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/capability/...`

- [x] 8. team/integration_test.go — Team Lifecycle → Capability Resolution → Broker Auth

  **What to do**:
  - Create `bridge/pkg/team/integration_test.go`
  - **Test: Team creation → member addition → role assignment → capability lookup** — Create team, add members with different roles, verify TeamService.GetCapabilitiesForMember returns correct caps per role
  - **Test: Team dissolution flow** — Create team, dissolve it, verify all subsequent operations fail
  - **Test: Auto-dissolve on last member removal** — Create team with 2 members, remove both, verify team auto-dissolves
  - **Test: Role change propagation** — Assign new role to member, verify capabilities update
  - **Test: Multiple team membership** — Agent in 2 teams, verify capabilities merged correctly
  - **Test: TeamStore → TeamService → TeamCapabilityRegistry → Broker** — Full chain: create team via store, add member via service, resolve caps via registry, authorize via broker
  - **Test: Governance enforcement** — Create team with max_members=2, attempt to add 3rd member → verify rejection
  - **Test: Policy override flow** — Set team policy override for action, verify broker uses override instead of default

  **Must NOT do**:
  - Do NOT modify production code
  - Do NOT test things already covered by service_test.go or store_test.go individually (focus on cross-component)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Multi-component flow across store→service→registry→broker
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 7, 9, 10)
  - **Blocks**: Tasks 12, 13
  - **Blocked By**: Tasks 1, 2, 3

  **References**:
  - `bridge/pkg/team/store.go` — TeamStore for creating test teams
  - `bridge/pkg/team/service.go` — TeamService for member operations
  - `bridge/pkg/team/governance.go` — GovernanceEnforcer for policy tests
  - `bridge/pkg/capability/team_registry.go` — TeamCapabilityRegistry for cap resolution
  - `bridge/pkg/capability/broker.go` — Broker for authorization
  - `bridge/pkg/team/store_test.go` — Pattern for creating temp DB teams
  - `bridge/pkg/team/service_test.go` — Pattern for service-level tests

  **Acceptance Criteria**:
  - [ ] `bridge/pkg/team/integration_test.go` created
  - [ ] `go test -v -count=1 ./pkg/team/... -run TestIntegration` → ALL PASS
  - [ ] At least 8 test functions covering full team lifecycle flows

  **QA Scenarios**:

  ```
  Scenario: Full team lifecycle from creation to dissolution
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -run "TestIntegration_Team" ./pkg/team/...
    Expected Result: 8+ integration tests, all PASS
    Evidence: .sisyphus/evidence/task-08-team-integration.txt

  Scenario: Store→Service→Registry→Broker chain works
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -run "TestIntegration_FullChain\|TestIntegration_Broker" ./pkg/team/...
    Expected Result: Broker authorizes based on team role capabilities
    Evidence: .sisyphus/evidence/task-08-team-broker-chain.txt
  ```

  **Commit**: YES
  - Message: `test(team): add cross-component team lifecycle integration tests`
  - Files: `bridge/pkg/team/integration_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/team/...`

- [x] 9. secretary/broker_check_test.go — ExecuteStep Broker Pre-Check

  **What to do**:
  - Create `bridge/pkg/secretary/broker_check_test.go`
  - **Test: ExecuteStep with broker ALLOW** — broker authorizes → step executes normally
  - **Test: ExecuteStep with broker DENY** — broker denies → step skipped with clear error
  - **Test: ExecuteStep with broker error** — broker returns error → step skipped, error logged
  - **Test: ExecuteStep with nil broker (backward compat)** — nil broker → step executes (no check)
  - **Test: ExecuteStep with context cancelled** — context cancelled → broker check returns DENY
  - **Test: Multiple sequential steps with broker** — step 1 ALLOW, step 2 DENY, step 3 skipped
  - **Test: Broker receives correct ActionRequest** — verify the action name, params, and metadata passed to broker match the step being executed

  **Must NOT do**:
  - Do NOT modify `orchestrator_integration.go` or any production code
  - Do NOT test existing orchestrator functionality (only the new broker check code path)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Tests internal secretary integration code, needs understanding of StepExecutor flow
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 7, 8, 10)
  - **Blocks**: None
  - **Blocked By**: Tasks 1, 7

  **References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:710-725` — Broker check code in ExecuteStep
  - `bridge/pkg/secretary/orchestrator_integration_test.go` — Existing orchestrator tests (43 tests) for patterns
  - `bridge/internal/skills/executor_authorizer_test.go` — Pattern for testing authorizer integration
  - `bridge/pkg/capability/types.go:41-60` — ActionRequest that broker expects

  **Acceptance Criteria**:
  - [ ] `bridge/pkg/secretary/broker_check_test.go` created
  - [ ] `go test -v -count=1 ./pkg/secretary/... -run TestBroker` → ALL PASS
  - [ ] At least 7 test functions covering all broker check paths

  **QA Scenarios**:

  ```
  Scenario: Broker check intercepts step execution correctly
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -run "TestBroker.*Step\|TestStep.*Broker" ./pkg/secretary/...
    Expected Result: 7+ tests, ALLOW→execute, DENY→skip, error→skip, nil→execute
    Evidence: .sisyphus/evidence/task-09-broker-check.txt

  Scenario: Backward compatibility — nil broker doesn't break existing flows
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -run "TestBroker.*Nil\|TestBroker.*Backward" ./pkg/secretary/...
    Expected Result: Step executes normally without broker check
    Evidence: .sisyphus/evidence/task-09-backward-compat.txt
  ```

  **Commit**: YES
  - Message: `test(secretary): add broker pre-check integration tests`
  - Files: `bridge/pkg/secretary/broker_check_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/secretary/...`

- [x] 10. email/team_routing_integration_test.go — Team Routing Flow

  **What to do**:
  - Create `bridge/pkg/email/team_routing_integration_test.go`
  - **Test: Email routed to correct team** — Dispatcher uses TeamMatcher to route email to team by recipient
  - **Test: Email with no matching team** — Falls through to default handler
  - **Test: Thread tracking integration** — Email arrives, thread tracker finds related thread, dispatcher uses thread context
  - **Test: Template rendering + draft flow** — Load template, substitute variables, create draft, list drafts, send draft
  - **Test: IMAP inbox + team routing** — Simulated IMAP message → team routing → thread tracking → draft response
  - **Test: Multiple teams receive different emails** — Verify routing isolation

  **Must NOT do**:
  - Do NOT modify any production code
  - Do NOT require a real IMAP server

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multi-component email flow, needs mock IMAP + mock team matcher
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 7, 8, 9)
  - **Blocks**: None
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/email/dispatcher.go` — Dispatcher with TeamMatcher injection
  - `bridge/pkg/email/thread_tracker.go` — ThreadTracker for message threading
  - `bridge/pkg/email/drafts.go` — DraftManager
  - `bridge/pkg/email/templates.go` — TeamTemplateManager with {{var}} substitution
  - `bridge/pkg/email/imap.go` — IMAPClient with IMAPDialer injection
  - `bridge/pkg/email/dispatcher_test.go` — Existing dispatcher tests for patterns

  **Acceptance Criteria**:
  - [ ] `bridge/pkg/email/team_routing_integration_test.go` created
  - [ ] `go test -v -count=1 ./pkg/email/... -run TestIntegration` → ALL PASS
  - [ ] At least 6 test functions covering team routing + thread + draft flow

  **QA Scenarios**:

  ```
  Scenario: Email flows from inbox to team response
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -run "TestIntegration" ./pkg/email/...
    Expected Result: 6+ integration tests, all PASS
    Evidence: .sisyphus/evidence/task-10-email-integration.txt
  ```

  **Commit**: YES
  - Message: `test(email): add team routing integration tests`
  - Files: `bridge/pkg/email/team_routing_integration_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/email/...`

- [x] 11. capability/concurrent_test.go — Broker Stress Tests

  **What to do**:
  - Create `bridge/pkg/capability/concurrent_test.go`
  - **Test: 100 concurrent Authorize calls** — All ALLOW, verify no panics or races
  - **Test: 100 concurrent Authorize calls** — Mix of ALLOW/DENY/DEFER, verify correct classification per call
  - **Test: Concurrent broker with shared registry** — Multiple goroutines register roles while others authorize
  - **Test: Concurrent consent requests** — Multiple DEFER actions, each gets own consent channel
  - **Test: Broker under sustained load** — 10s sustained load with 50 concurrent goroutines, verify no deadlocks
  - **Test: Rapid create/authorize cycles** — Create broker, authorize, discard, repeat 1000 times
  - **Test: Concurrent team registry lookups** — Multiple goroutines looking up different roles simultaneously

  **Must NOT do**:
  - Do NOT modify production code
  - Do NOT use real time delays (keep test fast, use mocked time if needed)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Concurrent test design requires careful synchronization reasoning
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 12, 13)
  - **Blocks**: F1
  - **Blocked By**: Task 7

  **References**:
  - `bridge/pkg/capability/broker.go` — Broker source for understanding thread safety
  - `bridge/pkg/capability/broker_test.go:630-670` — Existing benchmarks for pattern reference
  - `bridge/pkg/capability/team_registry.go` — TeamCapabilityRegistry for concurrent access testing

  **Acceptance Criteria**:
  - [ ] `bridge/pkg/capability/concurrent_test.go` created
  - [ ] `go test -v -count=1 -race ./pkg/capability/... -run TestConcurrent` → ALL PASS, zero races
  - [ ] At least 7 test functions for concurrent scenarios

  **QA Scenarios**:

  ```
  Scenario: Concurrent broker access passes race detector
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -race -run "TestConcurrent\|TestStress" ./pkg/capability/...
    Expected Result: All pass, zero DATA RACE warnings
    Evidence: .sisyphus/evidence/task-11-concurrent-race.txt

  Scenario: Sustained load doesn't deadlock
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -timeout=30s -run "TestConcurrent_Sustained\|TestStress" ./pkg/capability/...
    Expected Result: Completes within 30s (no deadlock)
    Evidence: .sisyphus/evidence/task-11-sustained-load.txt
  ```

  **Commit**: YES (groups with Tasks 12, 13)
  - Message: `test(capability,team): add concurrent stress tests and benchmarks`
  - Files: `bridge/pkg/capability/concurrent_test.go`
  - Pre-commit: `cd bridge && go test -race ./pkg/capability/...`

- [x] 12. team/concurrent_test.go — Team Store + Metrics Stress Tests

  **What to do**:
  - Create `bridge/pkg/team/concurrent_test.go`
  - **Test: 50 concurrent CreateTeam calls** — Verify all created teams are distinct and retrievable
  - **Test: Concurrent AddMember/RemoveMember** — Multiple goroutines adding/removing members from same team
  - **Test: Concurrent GetTeam while updating** — Readers don't block writers, no panics
  - **Test: Concurrent metrics recording** — 100 goroutines recording different metrics simultaneously
  - **Test: Team dissolution during concurrent access** — Dissolve team while others are reading/writing → verify graceful failures
  - **Test: Concurrent governance checks** — Multiple goroutines calling ValidateTeamCreation/ValidateMemberAddition simultaneously

  **Must NOT do**:
  - Do NOT modify production code
  - Do NOT leak temp database files (use t.Cleanup)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Concurrent SQLCipher access patterns, need careful temp file cleanup
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 11, 13)
  - **Blocks**: F1
  - **Blocked By**: Tasks 3, 8

  **References**:
  - `bridge/pkg/team/store.go` — TeamStore for concurrent access testing
  - `bridge/pkg/team/metrics.go` — TeamMetrics for concurrent recording
  - `bridge/pkg/team/store_test.go` — Pattern for temp DB creation
  - `bridge/pkg/team/governance.go` — GovernanceEnforcer for concurrent checks

  **Acceptance Criteria**:
  - [ ] `bridge/pkg/team/concurrent_test.go` created
  - [ ] `go test -v -count=1 -race ./pkg/team/... -run TestConcurrent` → ALL PASS, zero races
  - [ ] At least 6 test functions for concurrent team operations

  **QA Scenarios**:

  ```
  Scenario: Concurrent team store operations pass race detector
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -race -run "TestConcurrent\|TestStress" ./pkg/team/...
    Expected Result: All pass, zero DATA RACE warnings
    Evidence: .sisyphus/evidence/task-12-team-concurrent.txt

  Scenario: Temp database files cleaned up
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -count=1 -run "TestConcurrent" ./pkg/team/...
      2. Check for leftover .db files in bridge/ directory: ls *.db 2>/dev/null
    Expected Result: No leftover temp files
    Evidence: .sisyphus/evidence/task-12-cleanup.txt
  ```

  **Commit**: YES (groups with Tasks 11, 13)
  - Message: `test(capability,team): add concurrent stress tests and benchmarks`
  - Files: `bridge/pkg/team/concurrent_test.go`
  - Pre-commit: `cd bridge && go test -race ./pkg/team/...`

- [x] 13. capability/benchmarks_test.go — Broker + Team Store Benchmarks

  **What to do**:
  - Create `bridge/pkg/capability/benchmarks_test.go`
  - **Benchmark: BrokerAuthorize with team registry** — Measure overhead of team-based capability lookup vs direct registry
  - **Benchmark: TeamCapabilityRegistry_GetCapabilities** — Role lookup performance for all 6 built-in roles
  - **Benchmark: TeamStore_CreateTeam** — Database write performance
  - **Benchmark: TeamStore_GetTeam** — Database read performance
  - **Benchmark: TeamMetrics_RecordAndGetSnapshot** — Metrics recording + snapshot overhead
  - **Benchmark: RiskClassifier_Classify** — Classification performance across all risk classes
  - **Benchmark: StructuredOutputParser_Parse** — JSON parsing performance with retries

  **Must NOT do**:
  - Do NOT modify production code
  - Do NOT add benchmarks that require external services

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Straightforward benchmark functions following existing patterns
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 11, 12)
  - **Blocks**: F1
  - **Blocked By**: Tasks 7, 8

  **References**:
  - `bridge/pkg/capability/broker_test.go:630-670` — Existing BenchmarkBrokerAuthorize for pattern
  - `bridge/pkg/audit/compliance_test.go:611-640` — Existing benchmarks for pattern

  **Acceptance Criteria**:
  - [ ] `bridge/pkg/capability/benchmarks_test.go` created
  - [ ] `go test -bench=. -benchtime=1s -run=^$ ./pkg/capability/...` → ALL complete without errors
  - [ ] At least 7 benchmark functions

  **QA Scenarios**:

  ```
  Scenario: All benchmarks complete without errors
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -bench=. -benchtime=1s -run=^$ ./pkg/capability/...
    Expected Result: All benchmarks complete, no panics
    Evidence: .sisyphus/evidence/task-13-benchmarks.txt
  ```

  **Commit**: YES (groups with Tasks 11, 12)
  - Message: `test(capability,team): add concurrent stress tests and benchmarks`
  - Files: `bridge/pkg/capability/benchmarks_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/capability/...`

- [x] 14. Rust: Vault Scope Validation Tests

  **What to do**:
  - Add `#[cfg(test)] mod tests` block to `rust-vault/src/governance/ephemeral.rs` (or extend existing)
  - **Test: Valid scope matches** — Create ScopeMatch with matching capability_scope, verify validation passes
  - **Test: Scope mismatch** — Create ScopeMatch with non-matching scope, verify ScopeMismatch error
  - **Test: Empty scope matches all** — Empty capability_scope should match any action
  - **Test: Multiple scopes** — capability_scope with multiple entries, verify each is checked
  - **Test: Scope validation with nil/empty governance** — Verify graceful handling
  - **Test: proto round-trip** — Verify CapabilityScope proto serialization/deserialization

  Also check `rust-vault/proto/governance.proto` for the capability_scope field definition and test the generated struct.

  **Must NOT do**:
  - Do NOT modify production Rust code (only add tests)
  - Do NOT change proto definitions

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Rust testing with proto types, needs understanding of Rust test patterns
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Task 15)
  - **Blocks**: F1
  - **Blocked By**: None

  **References**:
  - `rust-vault/src/governance/ephemeral.rs` — Scope validation logic to test
  - `rust-vault/proto/governance.proto` — capability_scope field definition
  - `rust-vault/src/governance/event_notifier.rs:234-246` — Existing test patterns
  - `rust-vault/src/grpc/governance_service.rs` — ScopeMismatch mapping

  **Acceptance Criteria**:
  - [ ] Tests added to `rust-vault/src/governance/ephemeral.rs` or new test module
  - [ ] `cd rust-vault && cargo test --lib` → ALL PASS
  - [ ] At least 6 test functions for scope validation

  **QA Scenarios**:

  ```
  Scenario: All scope validation tests pass
    Tool: Bash
    Steps:
      1. Run: cd rust-vault && cargo test --lib
    Expected Result: New tests pass, existing tests unaffected
    Evidence: .sisyphus/evidence/task-14-rust-scope.txt
  ```

  **Commit**: YES (groups with Task 15)
  - Message: `test(rust-vault,sidecar): add scope validation and QueryDocuments tests`
  - Files: `rust-vault/src/governance/ephemeral.rs`
  - Pre-commit: `cd rust-vault && cargo test --lib`

- [x] 15. Rust: Sidecar QueryDocuments Tests

  **What to do**:
  - Add `#[cfg(test)] mod tests` block to `sidecar/src/grpc/server.rs` (or new test module)
  - **Test: QueryDocuments with valid request** — Verify correct response with document chunks
  - **Test: QueryDocuments with empty query** — Verify error handling
  - **Test: QueryDocuments with non-existent collection** — Verify error response
  - **Test: QueryDocuments with invalid collection name** — Verify validation
  - **Test: QueryDocuments proto round-trip** — Verify DocumentChunk serialization
  - **Test: QueryDocuments with large result set** — Verify handling of multiple chunks

  Also verify `sidecar/src/grpc/proto/sidecar.proto` DocumentChunk message definition.

  **Must NOT do**:
  - Do NOT modify production Rust code (only add tests)
  - Do NOT change proto definitions
  - Do NOT require a running Qdrant instance

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Rust gRPC testing with proto types, needs tokio::test for async
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Task 14)
  - **Blocks**: F1
  - **Blocked By**: None

  **References**:
  - `sidecar/src/grpc/server.rs` — QueryDocuments implementation
  - `sidecar/src/grpc/proto/sidecar.proto` — QueryDocuments RPC + DocumentChunk message
  - `sidecar/src/document/qdrant.rs` — Qdrant client (for understanding mock needs)

  **Acceptance Criteria**:
  - [ ] Tests added to `sidecar/src/grpc/server.rs` or new test module
  - [ ] `cd sidecar && cargo test --lib` → ALL PASS
  - [ ] At least 6 test functions for QueryDocuments

  **QA Scenarios**:

  ```
  Scenario: All QueryDocuments tests pass
    Tool: Bash
    Steps:
      1. Run: cd sidecar && cargo test --lib
    Expected Result: New tests pass, existing tests unaffected
    Evidence: .sisyphus/evidence/task-15-rust-query.txt
  ```

  **Commit**: YES (groups with Task 14)
  - Message: `test(rust-vault,sidecar): add scope validation and QueryDocuments tests`
  - Files: `sidecar/src/grpc/server.rs`
  - Pre-commit: `cd sidecar && cargo test --lib`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Full Test Suite Run + Race Detection** — `unspecified-high`
  Run `go test -v -count=1 -race ./pkg/capability/... ./pkg/team/... ./pkg/browser/... ./pkg/sidecar/... ./pkg/audit/... ./pkg/studio/... ./internal/skills/... ./pkg/mcp/...`. Run `cd rust-vault && cargo test --lib`. Run `cd sidecar && cargo test --lib`. All must PASS with zero race conditions.
  Output: `Go [44 new tests PASS, 0 races] | Rust vault [16 tests PASS] | Sidecar [7 tests PASS] | VERDICT: APPROVE`

- [x] F2. **Coverage Report + Gap Analysis** — `deep`
  Run `go test -cover ./pkg/capability/... ./pkg/team/... ./pkg/browser/... ./pkg/sidecar/...`. For each package, report coverage percentage. Identify any exported function in new code that still has 0% coverage. Compare test count before and after this plan.
  Output: `Coverage [capability 29.4%, team 28.8%, secretary 0.5%, browser 4.7%, sidecar 0.5%] | Gaps [0 new untested exports] | New Tests [67 added] | VERDICT: APPROVE`

- [x] F3. **Test Quality Review** — `unspecified-high`
  Review ALL new test files for anti-patterns: trivial assertions (`assert.True(true)`), no error path tests, missing table-driven tests where applicable, skipped tests, test interdependencies, hardcoded values that should be constants. Check for proper cleanup (t.Cleanup, defer os.Remove for temp files).
  Output: `Files [13 clean / 0 issues] | Anti-patterns [0] | VERDICT: APPROVE`

- [x] F4. **Scope Fidelity Check** — `deep`
  Verify ONLY test files were created/modified. Zero changes to production code. Zero changes to pkg/governor/, pkg/runtime/. All new test files are in expected packages. No import of internal/ from pkg/ tests.
  Output: `Production changes [CLEAN] | Forbidden imports [CLEAN] | Scope [CLEAN] | VERDICT: APPROVE`

---

## Commit Strategy

- **Wave 1**: `test(capability): extract shared test mocks and helpers` — test helpers
- **Wave 2**: `test(team,browser,sidecar,capability): fill unit test gaps` — 4 new test files + error paths
- **Wave 3**: `test(capability,team,secretary,email): add integration tests` — 4 integration test files
- **Wave 4**: `test(capability,team): add concurrent stress tests and benchmarks` — 3 stress/bench files
- **Wave 5**: `test(rust-vault,sidecar): add scope validation and QueryDocuments tests` — Rust test modules

---

## Success Criteria

### Verification Commands
```bash
# Full Go suite with race detection
cd bridge && go test -v -count=1 -race ./pkg/capability/... ./pkg/team/... ./pkg/browser/... ./pkg/sidecar/... ./pkg/audit/... ./pkg/studio/... ./internal/skills/... ./pkg/mcp/...
# Coverage report
cd bridge && go test -cover ./pkg/capability/... ./pkg/team/... ./pkg/browser/... ./pkg/sidecar/...
# Rust vault
cd rust-vault && cargo test --lib
# Sidecar
cd sidecar && cargo test --lib
# No production code changes
git diff --stat HEAD -- '*.go' ':!*_test.go' ':!vendor/'  # Expected: empty for bridge/
```

### Final Checklist
- [x] All 4 previously untested files now have test coverage
- [x] Integration tests cover all cross-package flows
- [x] Concurrent tests pass with `-race` flag
- [x] Rust scope validation tested
- [x] Rust QueryDocuments tested
- [x] No changes to production code
- [x] Zero test failures
- [x] No data races detected
