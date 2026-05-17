# Critical Test Coverage Sprint

## TL;DR

> **Quick Summary**: Add comprehensive test coverage for three security-critical gaps: Installation flow, API key management, and Three-Way Consent (PII protection).
>
> **Deliverables**:
> - `tests/integration/test-installation.sh` - Installation flow tests
> - `bridge/pkg/keystore/api_key_test.go` - API key validation and rotation tests
> - `bridge/pkg/pii/three_way_consent_test.go` - Consent flow tests (zero → full coverage)
>
> **Estimated Effort**: Medium
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: Task 1 → Task 4 → Task 7

---

## Context

### Original Request
Add critical test coverage for gaps identified in architecture review:
1. Installation tests - first user interaction, currently only partial coverage
2. API Key tests - AI cannot function without valid keys
3. Three-Way Consent tests - PII protection gate with ZERO test coverage

### Interview Summary
**Key Discussions**:
- Priority: Three-Way Consent is P0 (security-critical, zero coverage)
- Framework: Go `testing` package + testify (existing pattern)
- Shell tests: Bash with `test_*()` functions (existing pattern)
- Scope: Go bridge only (Android QR tests out of scope)

**Research Findings**:
- 100+ Go test files exist in bridge/
- 19 shell test scripts exist in tests/
- `three_way_consent.go` has 799 lines but NO test file
- Existing mock patterns: `MockMatrixAdapter`, `MockHITLManager`
- CI: `.github/workflows/test.yml` runs `go test -v -race`

### Metis Review
**Identified Gaps** (addressed):
- Consent tests must verify audit logging (compliance requirement)
- Installation tests must handle Docker daemon not running
- API key tests must cover provider validation

---

## Work Objectives

### Core Objective
Achieve test coverage for three security-critical paths that currently have partial or no tests.

### Concrete Deliverables
- `tests/integration/test-installation.sh` - 6 installation tests
- `bridge/pkg/keystore/api_key_test.go` - 5 API key tests
- `bridge/pkg/pii/three_way_consent_test.go` - 8 consent flow tests

### Definition of Done
- [ ] All new tests pass: `go test -v ./pkg/pii/... ./pkg/keystore/...`
- [ ] Shell tests pass: `./tests/integration/test-installation.sh`
- [ ] No regressions: Full `go test ./...` passes
- [ ] Coverage visible: `go test -cover ./pkg/pii/... ./pkg/keystore/...`

### Must Have
- Test for consent room creation
- Test for correct users invited
- Test for token reuse prevention
- Test for API key rotation
- Test for idempotent installation

### Must NOT Have (Guardrails)
- No Android/Kotlin tests (out of scope)
- No browser UI tests (out of scope)
- No external service dependencies (mock everything)
- No changes to production code (tests only)

---

## Verification Strategy (MANDATORY)

### Test Decision
- **Infrastructure exists**: YES (go test, testify, bash)
- **Automated tests**: YES (TDD pattern - write tests first)
- **Framework**: go test + testify
- **If TDD**: Each task follows RED (failing test) → GREEN (minimal impl already exists) → REFACTOR

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go tests**: Use Bash (go test) — Run tests, assert pass/fail, capture output
- **Shell tests**: Use Bash — Run script, check exit code, validate output
- **Coverage**: Use Bash (go test -cover) — Run coverage, assert threshold

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — consent tests, highest risk):
├── Task 1: Three-Way Consent basic tests [unspecified-high]
├── Task 2: Three-Way Consent room creation tests [unspecified-high]
└── Task 3: Three-Way Consent token tests [unspecified-high]

Wave 2 (After Wave 1 — API key tests):
├── Task 4: API key validation tests [quick]
├── Task 5: API key rotation tests [quick]
└── Task 6: API key provider tests [quick]

Wave 3 (After Wave 2 — installation tests):
├── Task 7: Installation GPG and idempotency tests [unspecified-high]
├── Task 8: Installation Docker readiness tests [unspecified-high]
└── Task 9: Installation network resilience tests [unspecified-high]

Wave FINAL (After ALL tasks — verification):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Test execution verification (unspecified-high)
├── Task F3: Coverage threshold check (unspecified-high)
└── Task F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: Task 1 → Task 4 → Task 7 → F1-F4 → user okay
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 3 (Waves 1 & 2)
```

### Dependency Matrix

- **1-3**: — — 4-9
- **4-6**: 1-3 — 7-9
- **7-9**: 4-6 — F1-F4
- **F1-F4**: 7-9 — user okay

### Agent Dispatch Summary

- **1**: **3** — T1-T3 → `unspecified-high`
- **2**: **3** — T4-T6 → `quick`
- **3**: **3** — T7-T9 → `unspecified-high`
- **FINAL**: **4** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [ ] 1. Three-Way Consent Basic Tests

  **What to do**:
  - Create `bridge/pkg/pii/three_way_consent_test.go`
  - Add `TestThreeWayConsentManager_RequestConsent` - verify room created
  - Add `TestThreeWayConsentManager_CorrectUsersInvited` - verify subject, requester, bridge invited
  - Add `TestThreeWayConsentManager_ConsentStateTransitions` - pending → approved/rejected

  **Must NOT do**:
  - Do not modify production code in `three_way_consent.go`
  - Do not add external dependencies

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Standard Go test file following existing patterns
  - **Skills**: []
    - testify already in codebase

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3)
  - **Blocks**: Tasks 4-6
  - **Blocked By**: None (can start immediately)

  **References**:
  - `bridge/pkg/pii/three_way_consent.go:67-120` - ConsentRoom struct and RequestConsent method
  - `bridge/pkg/pii/hitl_consent_test.go:12-50` - Existing HITL test pattern to follow
  - `bridge/pkg/pii/resolver_test.go` - Mock pattern examples

  **Acceptance Criteria**:
  - [ ] File created: `bridge/pkg/pii/three_way_consent_test.go`
  - [ ] `go test -v -run TestThreeWayConsentManager ./pkg/pii/...` → PASS (3 tests)

  **QA Scenarios**:
  ```
  Scenario: Request consent creates room with correct participants
    Tool: Bash (go test)
    Preconditions: ThreeWayConsentManager initialized with MockMatrixAdapter
    Steps:
      1. Call manager.RequestConsent(ctx, request) with subject "@patient:server.com" and requester "@doctor:server.com"
      2. Assert roomID returned is not empty
      3. Assert mock.GetInvites(roomID) contains all three participants
    Expected Result: Room created with 3 invites
    Failure Indicators: Empty roomID, missing invites
    Evidence: .sisyphus/evidence/task-1-consent-basic.out
  ```

  **Commit**: YES
  - Message: `test(pii): add Three-Way Consent basic tests`
  - Files: `bridge/pkg/pii/three_way_consent_test.go`

- [ ] 2. Three-Way Consent Room Creation Tests

  **What to do**:
  - Add `TestThreeWayConsentManager_RoomNotRecreated` - same request returns same room
  - Add `TestThreeWayConsentManager_RoomAliases` - verify optional alias set
  - Add `TestThreeWayConsentManager_InvalidRequest` - error on missing fields

  **Must NOT do**:
  - Do not create real Matrix rooms (use mocks)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Continuation of test file from Task 1
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3)
  - **Blocks**: Tasks 4-6
  - **Blocked By**: None (can start immediately)

  **References**:
  - `bridge/pkg/pii/three_way_consent.go:121-200` - Room creation logic
  - `bridge/pkg/pii/three_way_consent.go:20-34` - Error types to test

  **Acceptance Criteria**:
  - [ ] 3 additional tests in `three_way_consent_test.go`
  - [ ] `go test -v -run "TestThreeWayConsentManager.*Room" ./pkg/pii/...` → PASS

  **QA Scenarios**:
  ```
  Scenario: Duplicate request returns existing room
    Tool: Bash (go test)
    Preconditions: Room already created for request
    Steps:
      1. Call RequestConsent with same request ID
      2. Assert same roomID returned (no new room)
      3. Assert ErrConsentRoomExists returned on second call
    Expected Result: Idempotent room creation
    Evidence: .sisyphus/evidence/task-2-room-creation.out
  ```

  **Commit**: NO (groups with Task 3)

- [ ] 3. Three-Way Consent Token and Audit Tests

  **What to do**:
  - Add `TestThreeWayConsentManager_TokenCannotBeReused` - single-use tokens
  - Add `TestThreeWayConsentManager_ApprovalPropagatesToHITL` - verify HITL update
  - Add `TestThreeWayConsentManager_ConsentLogged` - audit log entry created
  - Add `TestThreeWayConsentManager_TokenExpiration` - expired tokens rejected

  **Must NOT do**:
  - Do not log actual PII values in tests

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Security-critical test completion
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2)
  - **Blocks**: Tasks 4-6
  - **Blocked By**: None (can start immediately)

  **References**:
  - `bridge/pkg/pii/three_way_consent.go:400-500` - Token handling
  - `bridge/pkg/pii/three_way_consent.go:600-700` - Audit logging
  - `bridge/pkg/audit/compliance_test.go` - Audit test patterns

  **Acceptance Criteria**:
  - [ ] 4 additional tests for token/audit
  - [ ] `go test -v ./pkg/pii/...` → PASS (all 10 tests)

  **QA Scenarios**:
  ```
  Scenario: Token cannot be reused after approval
    Tool: Bash (go test)
    Preconditions: Token used once for approval
    Steps:
      1. Create consent request, get token
      2. Use token (first use succeeds)
      3. Attempt to use same token again
      4. Assert error contains "already used"
    Expected Result: Second use rejected
    Failure Indicators: Token accepted twice
    Evidence: .sisyphus/evidence/task-3-token-reuse.out
  ```

  **Commit**: YES
  - Message: `test(pii): add Three-Way Consent token and audit tests`
  - Files: `bridge/pkg/pii/three_way_consent_test.go`

- [ ] 4. API Key Validation Tests

  **What to do**:
  - Create `bridge/pkg/keystore/api_key_test.go`
  - Add `TestAPIKeyValidation_ValidFormat` - correct key format accepted
  - Add `TestAPIKeyValidation_InvalidFormat` - malformed keys rejected
  - Add `TestAPIKeyValidation_UnknownProvider` - unknown provider rejected

  **Must NOT do**:
  - Do not store real API keys in test files
  - Do not make network calls to validate keys

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Standard validation tests
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 6)
  - **Blocks**: Tasks 7-9
  - **Blocked By**: Tasks 1-3

  **References**:
  - `bridge/pkg/keystore/keystore.go:711-760` - Store method
  - `bridge/pkg/providers/registry.go` - Provider validation
  - `bridge/pkg/keystore/keystore_test.go` - Existing keystore tests

  **Acceptance Criteria**:
  - [ ] File created: `bridge/pkg/keystore/api_key_test.go`
  - [ ] `go test -v -run TestAPIKeyValidation ./pkg/keystore/...` → PASS

  **QA Scenarios**:
  ```
  Scenario: Valid OpenAI key format accepted
    Tool: Bash (go test)
    Preconditions: Keystore initialized
    Steps:
      1. Call Store with provider="openai", token="sk-test123..."
      2. Assert no error returned
      3. Retrieve key and verify format preserved
    Expected Result: Key stored successfully
    Evidence: .sisyphus/evidence/task-4-key-validation.out
  ```

  **Commit**: NO (groups with Task 6)

- [ ] 5. API Key Rotation Tests

  **What to do**:
  - Add `TestAPIKeyRotation_Success` - old key replaced with new
  - Add `TestAPIKeyRotation_CacheInvalidated` - cached key cleared

  **Must NOT do**:
  - Do not test against real API endpoints

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Key lifecycle tests
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 6)
  - **Blocks**: Tasks 7-9
  - **Blocked By**: Tasks 1-3

  **References**:
  - `bridge/pkg/keystore/keystore.go` - Rotation logic (if exists) or add test-first
  - `bridge/pkg/keystore/keystore_test.go:TestStoreAndRetrieve` - Pattern

  **Acceptance Criteria**:
  - [ ] 2 rotation tests added
  - [ ] `go test -v -run TestAPIKeyRotation ./pkg/keystore/...` → PASS

  **QA Scenarios**:
  ```
  Scenario: Key rotation replaces old key
    Tool: Bash (go test)
    Preconditions: Key "test-key" stored with token "sk-old..."
    Steps:
      1. Call Rotate("test-key", "sk-new...")
      2. Retrieve key
      3. Assert token is "sk-new..."
      4. Assert "sk-old..." no longer accessible
    Expected Result: Old key completely replaced
    Evidence: .sisyphus/evidence/task-5-key-rotation.out
  ```

  **Commit**: NO (groups with Task 6)

- [ ] 6. API Key Roundtrip Tests

  **What to do**:
  - Add `TestAPIKeyRoundTrip_StoreAndGet` - stored key retrievable
  - Add `TestAPIKeyRoundTrip_EncryptionPreserved` - key encrypted at rest

  **Must NOT do**:
  - Do not log plaintext keys

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Integration-style key tests
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 5)
  - **Blocks**: Tasks 7-9
  - **Blocked By**: Tasks 1-3

  **References**:
  - `bridge/pkg/keystore/keystore.go:711-800` - Store/Get methods
  - `bridge/pkg/keystore/keystore_test.go` - Existing patterns

  **Acceptance Criteria**:
  - [ ] 2 roundtrip tests added
  - [ ] `go test -v ./pkg/keystore/...` → PASS (all 7 tests)

  **QA Scenarios**:
  ```
  Scenario: Key encrypted at rest
    Tool: Bash (go test)
    Preconditions: Key stored in keystore
    Steps:
      1. Store key with known value
      2. Read database file directly
      3. Assert plaintext value NOT present in file
      4. Assert encrypted blob IS present
    Expected Result: Key not readable from disk
    Evidence: .sisyphus/evidence/task-6-key-roundtrip.out
  ```

  **Commit**: YES
  - Message: `test(keystore): add API key validation, rotation, and roundtrip tests`
  - Files: `bridge/pkg/keystore/api_key_test.go`

- [ ] 7. Installation GPG and Idempotency Tests

  **What to do**:
  - Create `tests/integration/test-installation.sh`
  - Add `test_gpg_verification` - signature validates
  - Add `test_idempotency` - run twice, no duplicates
  - Add `test_syntax` - all shell scripts valid

  **Must NOT do**:
  - Do not make network calls (mock curl)
  - Do not require real GPG keys (use test fixtures)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Shell tests following existing pattern
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 8, 9)
  - **Blocks**: F1-F4
  - **Blocked By**: Tasks 4-6

  **References**:
  - `tests/integration/test-installer-hardening.sh` - Existing pattern to follow
  - `deploy/install.sh` - Main installer to test
  - `deploy/setup-matrix.sh` - Secondary script

  **Acceptance Criteria**:
  - [ ] File created: `tests/integration/test-installation.sh`
  - [ ] `./tests/integration/test-installation.sh` → PASS (3 tests)

  **QA Scenarios**:
  ```
  Scenario: Idempotent installation
    Tool: Bash (shell test)
    Preconditions: Installer script available
    Steps:
      1. Run installer in mock mode
      2. Run installer again
      3. Count "armorclaw" containers
      4. Assert count == 1 (not 2)
    Expected Result: Second run does not duplicate
    Evidence: .sisyphus/evidence/task-7-install-idempotent.out
  ```

  **Commit**: NO (groups with Task 9)

- [ ] 8. Installation Docker Readiness Tests

  **What to do**:
  - Add `test_docker_not_running` - graceful handling when Docker down
  - Add `test_docker_wait_timeout` - timeout after reasonable wait
  - Add `test_docker_ready` - proceeds when Docker available

  **Must NOT do**:
  - Do not actually stop Docker (mock systemctl)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Shell tests for Docker interaction
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 7, 9)
  - **Blocks**: F1-F4
  - **Blocked By**: Tasks 4-6

  **References**:
  - `tests/integration/test-installer-hardening.sh:57-63` - Docker wait pattern
  - `deploy/install.sh` - Docker readiness logic

  **Acceptance Criteria**:
  - [ ] 3 Docker tests added
  - [ ] `./tests/integration/test-installation.sh` → PASS (6 tests)

  **QA Scenarios**:
  ```
  Scenario: Docker not running handled gracefully
    Tool: Bash (shell test)
    Preconditions: Mock systemctl that returns "inactive"
    Steps:
      1. Run installer with mocked systemctl
      2. Assert installer does not hang
      3. Assert clear error message displayed
    Expected Result: Graceful failure, not hang
    Evidence: .sisyphus/evidence/task-8-docker-readiness.out
  ```

  **Commit**: NO (groups with Task 9)

- [ ] 9. Installation Network Resilience Tests

  **What to do**:
  - Add `test_network_timeout` - handles slow downloads
  - Add `test_checksum_mismatch` - detects corrupted files
  - Add `test_port_conflict` - handles port already in use

  **Must NOT do**:
  - Do not require actual network (use mocks)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Shell tests for network edge cases
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 7, 8)
  - **Blocks**: F1-F4
  - **Blocked By**: Tasks 4-6

  **References**:
  - `deploy/install.sh` - Download and checksum logic
  - `tests/test-e2e.sh` - E2E test pattern

  **Acceptance Criteria**:
  - [ ] 3 network tests added
  - [ ] `./tests/integration/test-installation.sh` → PASS (9 tests total)

  **QA Scenarios**:
  ```
  Scenario: Checksum mismatch detected
    Tool: Bash (shell test)
    Preconditions: Mock curl returns wrong checksum
    Steps:
      1. Run installer with mocked download
      2. Assert installer detects mismatch
      3. Assert installer does not proceed
    Expected Result: Installation aborted safely
    Evidence: .sisyphus/evidence/task-9-network-resilience.out
  ```

  **Commit**: YES
  - Message: `test(install): add installation flow tests for GPG, idempotency, Docker, and network`
  - Files: `tests/integration/test-installation.sh`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run test). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Test Execution Verification** — `unspecified-high`
  Run `go test -v ./pkg/pii/... ./pkg/keystore/...` and `./tests/integration/test-installation.sh`. Verify all new tests pass. Check for flaky tests (run 3x). Save output to `.sisyphus/evidence/final-qa/`.
  Output: `Tests [N/N pass] | Flaky [0/N] | VERDICT`

- [ ] F3. **Coverage Threshold Check** — `unspecified-high`
  Run `go test -cover ./pkg/pii/... ./pkg/keystore/...`. Verify coverage >= 60% for new code. Generate coverage report. Save to `.sisyphus/evidence/final-qa/coverage.out`.
  Output: `Coverage [N%] | Threshold [60%] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", verify only test files were created (no production code changes). Check "Must NOT do" compliance. Detect scope creep: tests touching non-test files.
  Output: `Tasks [N/N compliant] | Scope Creep [CLEAN/N issues] | VERDICT`

---

## Commit Strategy

- **All Tasks**: `test(scope): add critical test coverage for [area]`
- Pre-commit: `go test ./...`

---

## Success Criteria

### Verification Commands
```bash
go test -v ./pkg/pii/three_way_consent_test.go ./pkg/pii/three_way_consent.go
go test -v ./pkg/keystore/api_key_test.go
./tests/integration/test-installation.sh
```

### Final Checklist
- [ ] All "Must Have" present (consent room, token reuse, key rotation, idempotency)
- [ ] All "Must NOT Have" absent (no Android, no browser, no external deps)
- [ ] All tests pass
- [ ] Coverage >= 60% for new test files
