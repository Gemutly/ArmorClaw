# BEATO v1.1 Runtime Fix — Work Plan

## TL;DR

> **Objective:** Raise BEATO runtime readiness from **61/100 → 85–90/100** by wiring existing auth middleware, registering missing browser RPCs, verifying already-complete tasks, and honestly re-scoring.
>
> **Deliverables:**
> - SafetyMiddleware wired to all BEATO-sensitive RPC handlers
> - Per-handler auth regression tests
> - `browser.screenshot` and `browser.close` handlers implemented and registered
> - Browser smoke coverage for all 14 methods
> - Rust sidecar formally deferred with documented score ceiling
> - Email outbox revalidated after auth enforcement
> - AppArmor risk-accepted with documented compensating controls
> - Updated BEATO runtime report with evidence
>
> **Estimated Effort:** Medium (8-12 tasks across 4 waves)
> **Parallel Execution:** YES — 4 waves
> **Critical Path:** T1 (auth wire) → T2 (auth tests) → T8 (email revalidation) → T11 (re-score)

---

## Context

### Original Request
BEATO v1.1 plan: raise runtime readiness from 61/100 toward 85-90/100 by fixing the highest-impact runtime failures.

### Interview Summary
**Key Discussions:**
- Three parallel explore agents analyzed current codebase state exhaustively
- Key finding: `rpc_safety.go` (299 lines) is production-quality but **never wired** into `registerHandlers()` — all 150+ handlers accept unauthenticated requests
- T3 (Python sidecar), T7 (Matrix client), T9 (Healthchecks) confirmed ALREADY DONE from previous sprint
- Rust sidecar has no Dockerfile — formal defer is the right call
- `browser.screenshot` and `browser.close` have **no handler functions at all** — need implementation, not just registration
- AppArmor profile doesn't exist — risk-accept with compensating controls documented

**Research Findings:**
- `rpc_safety.go`: 4 RPC groups defined (Browser 20/min, Jetski 30/min, Document 10/min, Email 20/min)
- `server.go` lines 1228-1377: flat map of ~150 handlers, all raw unwrapped
- `browser.go`: 12 handlers exist, 2 missing (screenshot, close)
- Existing test suite: 214 Go test functions, 14 shell test scripts, but NO per-handler auth integration tests

### Metis Review
**Identified Gaps (addressed):**
- **Handler exclusion list**: `health.*`, `hardening.*`, `provisioning.*`, `mobile.*`, `keystore.*`, `device.*`, `invite.*`, `e2ee.*` must NOT be wrapped — would break healthcheck and bootstrap flows
- **Healthcheck deadlock risk**: If `health.check` is wrapped, Docker healthcheck probe fails → infinite restart loop
- **Token provisioning**: Admin token must be available via env var for VPS deployment
- **Existing integration tests**: Shell test scripts don't send tokens — will break after T1; must update test fixtures
- **browser.screenshot PII risk**: Screenshot data could contain PII; risk-accept for this sprint
- **No Text/Audio RPC groups**: Only Browser, Jetski, Document, Email groups exist; Text/Audio middleware is out of scope
- **Auth token rotation**: SafetyMiddleware captures token at creation; rotation requires restart (acceptable)

---

## Work Objectives

### Core Objective
Convert BEATO from "code exists" to "runtime verified" by fixing production behavior. Three tasks from v1.1 are already complete — verify and move on.

### Concrete Deliverables
- `server.go`: SafetyMiddleware wired around BEATO handlers (browser.*, document.*, email.*, approve_email, deny_email, email_approval_status, email.list_pending)
- `browser.go`: `handleBrowserScreenshot` and `handleBrowserClose` implemented
- `rpc_safety_test.go` or new test files: Per-handler auth regression tests
- `tests/test-browser-smoke.sh`: Extended to cover all 14 browser methods
- `.sisyphus/evidence/t4-rust-deferral.md`: Formal deferral document
- `tests/reports/beato-runtime-report.md`: Updated score with fresh evidence
- `.sisyphus/evidence/beato-v1.1-index.md`: Complete evidence index

### Definition of Done
- [ ] `bridge_rpc "health.check" '{}'` → succeeds (no auth required)
- [ ] `bridge_rpc "browser.navigate" '{}'` → auth error (-32011)
- [ ] `bridge_rpc "document.extract_text" '{}'` → auth error (-32011)
- [ ] `bridge_rpc "email.queue_status" '{}'` → auth error (-32011)
- [ ] `bridge_rpc "browser.screenshot" '{"token":"VALID",...}'` → non-method-not-found
- [ ] `bridge_rpc "browser.close" '{"token":"VALID",...}'` → non-method-not-found
- [ ] `cd bridge && go test ./pkg/rpc/... -run "Auth"` → all pass
- [ ] BEATO runtime report shows ≥ 85/100

### Must Have
- Auth enforcement on ALL sensitive BEATO RPCs (browser.*, document.*, email.*)
- Handler exclusion list preventing healthcheck/bootstrap deadlock
- Per-handler auth regression tests proving enforcement cannot silently regress
- browser.screenshot and browser.close handlers that return valid JSON
- Evidence-based BEATO re-score — no inflated claims

### Must NOT Have (Guardrails)
- Do NOT modify `rpc_safety.go` — zero lines changed (wiring only)
- Do NOT wrap `health.*`, `hardening.*`, `provisioning.*`, `mobile.*`, `keystore.*` handlers
- Do NOT disable auth to make tests pass
- Do NOT bypass HMAC validation
- Do NOT change the scoring rubric — only re-score with fresh evidence
- Do NOT attempt Matrix reconnection (operational, not code)
- Do NOT start Audio implementation
- Do NOT deploy Rust sidecar (formal defer this sprint)
- Do NOT change proto files
- Do NOT touch existing 12 browser handler registrations
- Do NOT claim BEATO 100% unless runtime tests prove it

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES — Go test framework + shell harness
- **Automated tests**: TDD for T1/T2 (auth middleware wiring + regression tests), tests-after for T5/T6
- **Framework**: `go test` for Go unit/integration tests, `bash` for shell smoke tests

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go tests**: Use `bash` (`go test -v -run ...`) — run tests, parse output
- **RPC verification**: Use `bash` (`echo '...' | socat - UNIX-CONNECT:...`) — send JSON-RPC, assert response
- **Shell smoke tests**: Use `bash` (`tests/test-*.sh`) — run harness, check PASS/FAIL
- **Documentation**: Use `bash` (file existence + content checks)

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — Security foundation):
├── T1: Wire SafetyMiddleware into registerHandlers() [unspecified-high]
├── T2: Add per-handler auth regression tests [quick]
└── T3-V: Verify Python sidecar is operational [quick]

Wave 2 (After Wave 1 — Browser + Office decision):
├── T5: Implement browser.screenshot + browser.close handlers [unspecified-high]
├── T6: Extend browser smoke coverage to all 14 methods [quick]
└── T4: Formal defer of Rust sidecar with score ceiling doc [quick]

Wave 3 (After Wave 2 — Integration + Stability):
├── T7-V: Verify Matrix client is operational [quick]
├── T8: Revalidate email outbox lifecycle after auth enforcement [unspecified-high]
├── T9-V: Verify healthchecks are comprehensive [quick]
└── T10: AppArmor risk acceptance with compensating controls [quick]

Wave 4 (After Wave 3 — Final validation):
├── T11: Full BEATO runtime re-score [writing]
└── T12: Evidence index [quick]

Wave FINAL (After ALL — 4 parallel reviews, then user okay):
├── F1: Security audit (oracle)
├── F2: Runtime QA (unspecified-high)
├── F3: Scope compliance (oracle)
└── F4: Release recommendation (writing)

Critical Path: T1 → T2 → T8 → T11 → F1-F4 → user okay
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 3 (Wave 2)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1 | — | T2, T8 | 1 |
| T2 | T1 | T8 | 1 |
| T3-V | — | — | 1 |
| T4 | — | T11 | 2 |
| T5 | — | T6 | 2 |
| T6 | T5 | T11 | 2 |
| T7-V | — | — | 3 |
| T8 | T1, T2 | T11 | 3 |
| T9-V | — | — | 3 |
| T10 | — | T11 | 3 |
| T11 | T1-T10 | T12 | 4 |
| T12 | T11 | F1-F4 | 4 |

### Agent Dispatch Summary

- **Wave 1**: 3 — T1 `unspecified-high`, T2 `quick`, T3-V `quick`
- **Wave 2**: 3 — T5 `unspecified-high`, T6 `quick`, T4 `quick`
- **Wave 3**: 4 — T7-V `quick`, T8 `unspecified-high`, T9-V `quick`, T10 `quick`
- **Wave 4**: 2 — T11 `writing`, T12 `quick`
- **FINAL**: 4 — F1 `oracle`, F2 `unspecified-high`, F3 `oracle`, F4 `writing`

---

## TODOs

- [x] 1. Wire SafetyMiddleware into BEATO Handler Registration

  **What to do**:
  - In `server.go`, add a `safety *SafetyMiddleware` field to the `Server` struct
  - In `New()` or `registerHandlers()`, instantiate `NewBEATOSafetyMiddleware(adminToken)` using the admin token from config/env
  - After building the `h` map in `registerHandlers()` (line 1228-1377), selectively wrap BEATO-sensitive handlers using `safety.WrapForGroup()`
  - **WRAP these handlers** (auth required):
    - `browser.*` → `BrowserRPCGroup`: navigate, fill, click, status, wait_for_element, wait_for_captcha, wait_for_2fa, complete, fail, list, cancel, replay_diagnostics, screenshot (T5), close (T5)
    - `document.*` → `DocumentRPCGroup`: extract_text, status, list_jobs
    - `email.*` + email approval → `EmailRPCGroup`: queue_status, get, retry, list, approve_email, deny_email, email_approval_status, email.list_pending
  - **DO NOT wrap these handlers** (exclusion list — would break healthcheck/bootstrap):
    - `health.*`: health.check
    - `hardening.*`: hardening.status, hardening.ack, hardening.rotate_password
    - `provisioning.*`: provisioning.start, provisioning.claim
    - `mobile.*`: mobile.heartbeat
    - `keystore.*`: keystore.unseal, keystore.sealed, keystore.seal, keystore.extend_session, keystore.session_status, keystore.list_keys, keystore.delete_key
    - `device.*`: device.list, device.get, device.approve, device.reject
    - `invite.*`: invite.list, invite.create, invite.revoke, invite.validate
    - `e2ee.*`: e2ee.create_backup, e2ee.delete_backup, e2ee.backup_exists
    - `bridge.e2ee_*`: bridge.e2ee_enable, bridge.e2ee_disable
    - `store_key`: store_key
  - Zero lines changed in `rpc_safety.go`

  **Must NOT do**:
  - Do NOT modify `rpc_safety.go` — wiring only
  - Do NOT wrap `health.check` — would cause Docker healthcheck deadlock (infinite restart loop)
  - Do NOT wrap `hardening.*` — needed before admin token exists
  - Do NOT wrap `provisioning.*` — needed for initial setup
  - Do NOT log raw tokens
  - Do NOT change the middleware chain order

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Precise code surgery across a large handler map with critical exclusion logic
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T3-V)
  - **Parallel Group**: Wave 1 (with T2 sequential after, T3-V parallel)
  - **Blocks**: T2, T6 (partially), T8
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/rpc_safety.go:270-299` — `NewBEATOSafetyMiddleware()` factory and `WrapForGroup()` method. This is the wiring API to call.
  - `bridge/pkg/rpc/rpc_safety.go:289` — `WrapForGroup(group RPCGroup, handler HandlerFunc) HandlerFunc` — wraps a single handler. Call this for each BEATO handler.
  - `bridge/pkg/rpc/rpc_safety.go:30-45` — RPC group constants: `BrowserRPCGroup`, `JetskiRPCGroup`, `DocumentRPCGroup`, `EmailRPCGroup` with rate limit profiles.
  - `bridge/pkg/rpc/server.go:1228-1377` — `registerHandlers()` function with the full flat handler map. This is where wrapping happens.

  **API/Type References**:
  - `bridge/pkg/rpc/rpc_safety.go:21-25` — `SafetyMiddleware` struct with `AdminToken` field and `Wrap()` method.
  - `bridge/pkg/rpc/server.go:1-50` — `Server` struct definition. Need to add `safety *SafetyMiddleware` field here.

  **Test References**:
  - `bridge/pkg/rpc/rpc_safety_test.go:40-95` — Existing auth tests showing expected error codes: missing token → `-32011` (RPCAuthRequired), invalid token → `-32012` (RPCAuthForbidden).

  **WHY Each Reference Matters**:
  - `rpc_safety.go:270-299`: The `WrapForGroup()` function is the only API needed. It takes a group and a handler, returns a wrapped handler.
  - `server.go:1228-1377`: The exclusion list is critical. Wrapping `health.check` will break Docker healthcheck → infinite restart loop.
  - `rpc_safety_test.go`: Error codes `-32011` and `-32012` are the auth error responses that all wrapped handlers must return.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Healthcheck works without auth (exclusion verification)
    Tool: Bash (go test)
    Preconditions: Bridge running with SafetyMiddleware wired
    Steps:
      1. cd bridge && go test -v -run TestHealthCheck ./pkg/rpc/...
      2. Verify test passes without providing any token
    Expected Result: health.check handler responds successfully without auth
    Failure Indicators: Test fails with auth error -32011 or -32012
    Evidence: .sisyphus/evidence/task-1-healthcheck-no-auth.txt

  Scenario: Browser RPC rejects unauthenticated request
    Tool: Bash (go test)
    Preconditions: SafetyMiddleware wired
    Steps:
      1. cd bridge && go test -v -run TestRPCSafetyAuthMissingToken ./pkg/rpc/...
      2. Assert response contains error code -32011
    Expected Result: browser.navigate without token returns RPCAuthRequired (-32011)
    Failure Indicators: Request succeeds or returns different error code
    Evidence: .sisyphus/evidence/task-1-browser-no-auth-rejection.txt

  Scenario: Document RPC rejects unauthenticated request
    Tool: Bash (echo + socat or go test)
    Preconditions: SafetyMiddleware wired
    Steps:
      1. Send {"jsonrpc":"2.0","id":1,"method":"document.extract_text","params":{}} without token
      2. Assert response error code is -32011
    Expected Result: document.extract_text returns RPCAuthRequired
    Failure Indicators: Request succeeds or returns -32601 (method not found)
    Evidence: .sisyphus/evidence/task-1-document-no-auth-rejection.txt

  Scenario: Email RPC rejects unauthenticated request
    Tool: Bash (go test)
    Preconditions: SafetyMiddleware wired
    Steps:
      1. Send {"jsonrpc":"2.0","id":1,"method":"email.queue_status","params":{}} without token
      2. Assert response error code is -32011
    Expected Result: email.queue_status returns RPCAuthRequired
    Failure Indicators: Request succeeds without auth
    Evidence: .sisyphus/evidence/task-1-email-no-auth-rejection.txt

  Scenario: Hardening RPC works without auth (exclusion verification)
    Tool: Bash (go test)
    Preconditions: SafetyMiddleware wired
    Steps:
      1. cd bridge && go test -v -run TestHardening ./pkg/rpc/...
      2. Verify hardening.status responds without auth error
    Expected Result: hardening.* handlers work without token (excluded from wrapping)
    Failure Indicators: Returns -32011 auth error
    Evidence: .sisyphus/evidence/task-1-hardening-exclusion.txt
  ```

  **Commit**: YES
  - Message: `fix(rpc): wire SafetyMiddleware into BEATO handler registration`
  - Files: `bridge/pkg/rpc/server.go`
  - Pre-commit: `cd bridge && go test ./pkg/rpc/... -count=1`

- [x] 2. Add Per-Handler Auth Regression Tests

  **What to do**:
  - Create test functions proving each BEATO RPC group enforces auth at the handler level
  - Add these tests to `bridge/pkg/rpc/rpc_safety_test.go` or a new file `bridge/pkg/rpc/auth_integration_test.go`
  - Required test functions:
    - `TestBrowserHandlersRequireAuth` — call each browser.* handler without token → expect -32011
    - `TestDocumentHandlersRequireAuth` — call document.extract_text, document.status, document.list_jobs without token → expect -32011
    - `TestEmailHandlersRequireAuth` — call email.queue_status, email.get, email.retry, email.list without token → expect -32011
    - `TestEmailApprovalHandlersRequireAuth` — call approve_email, deny_email, email_approval_status, email.list_pending without token → expect -32011
    - `TestExcludedHandlersDoNotRequireAuth` — call health.check, hardening.status, provisioning.start without token → expect success (not -32011)
    - `TestValidTokenAllowsBEATOHandlers` — call browser.navigate with valid token → expect passthrough (handler-specific error, not auth error)
    - `TestInvalidTokenRejected` — call browser.navigate with wrong token → expect -32012 (RPCAuthForbidden)

  **Must NOT do**:
  - Do NOT modify existing tests
  - Do NOT test middleware in isolation (already covered by existing tests)
  - Do NOT create tests that require VPS access
  - Do NOT send raw tokens to evidence files

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Follows established test patterns in `rpc_safety_test.go`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1 (sequential after T1)
  - **Blocks**: T8
  - **Blocked By**: T1

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/rpc_safety_test.go:40-95` — Existing auth test pattern: create SafetyMiddleware, create mock handler, call Wrap(), send request, assert error code.
  - `bridge/pkg/rpc/rpc_safety_test.go:261-310` — `TestRPCSafetyGroupProfiles` and `TestRPCSafetyWrapForGroup` show how to test per-group wrapping.

  **API/Type References**:
  - `bridge/pkg/rpc/rpc_safety.go:50-60` — Error codes: `RPCAuthRequired = -32011`, `RPCAuthForbidden = -32012`.

  **Test References**:
  - `bridge/pkg/rpc/rpc_safety_test.go:79-95` — `TestRPCSafetyAuthValidToken` shows valid token test pattern.

  **WHY Each Reference Matters**:
  - `rpc_safety_test.go:40-95`: The exact pattern to follow for per-handler tests. Same setup, same assertions, just testing individual handlers instead of the middleware class.
  - `rpc_safety.go:50-60`: Error codes are the concrete assertions — tests must check for exactly these codes.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All auth regression tests pass
    Tool: Bash (go test)
    Preconditions: T1 wiring complete
    Steps:
      1. cd bridge && go test -v -run "Auth" ./pkg/rpc/... -count=1
      2. Assert 0 failures
    Expected Result: All TestBrowserHandlersRequireAuth, TestDocumentHandlersRequireAuth, TestEmailHandlersRequireAuth, TestExcludedHandlersDoNotRequireAuth, TestValidTokenAllowsBEATOHandlers, TestInvalidTokenRejected pass
    Failure Indicators: Any test failure, especially auth-not-enforced failures
    Evidence: .sisyphus/evidence/task-2-auth-regression-tests.txt

  Scenario: Exclusion list verified — health.check not auth-gated
    Tool: Bash (go test)
    Steps:
      1. cd bridge && go test -v -run TestExcludedHandlersDoNotRequireAuth ./pkg/rpc/...
      2. Assert health.check, hardening.status, provisioning.start all pass without token
    Expected Result: Excluded handlers respond successfully without auth
    Failure Indicators: Any excluded handler returns -32011
    Evidence: .sisyphus/evidence/task-2-exclusion-verification.txt
  ```

  **Commit**: YES
  - Message: `test(rpc): add per-handler auth regression tests for BEATO handlers`
  - Files: `bridge/pkg/rpc/auth_integration_test.go` (new file)
  - Pre-commit: `cd bridge && go test ./pkg/rpc/... -run "Auth" -count=1`

- [x] 3-V. Verify Python Sidecar is Operational (ALREADY DONE — Confirm Only)

  **What to do**:
  - Confirm the Python sidecar code is complete and functional:
    1. Verify `sidecar-python/proto/sidecar_pb2.py` exists and is valid Python
    2. Verify `sidecar-python/proto/sidecar_pb2_grpc.py` exists and is valid Python
    3. Verify `sidecar-python/proto/__init__.py` exists (makes it a package)
    4. Verify `sidecar-python/worker.py` line 20 uses `from proto import sidecar_pb2, sidecar_pb2_grpc` (correct import path)
    5. Verify `sidecar-python/Dockerfile` builds (multi-stage, python:3.12-slim)
    6. Verify `deploy/docker-compose.sidecar-py.yml` has hardened config: `network_mode: none`, `cap_drop: ALL`, `read_only: true`
  - Record evidence that the Python sidecar is structurally complete

  **Must NOT do**:
  - Do NOT modify any Python sidecar files
  - Do NOT attempt to build or deploy the container
  - Do NOT fix any VPS runtime issues (out of scope)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple file existence and content verification
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `sidecar-python/Dockerfile` — Multi-stage build; verify builder stage compiles proto, runtime stage copies.
  - `sidecar-python/worker.py:20` — Import line: `from proto import sidecar_pb2, sidecar_pb2_grpc`

  **API/Type References**:
  - `sidecar-python/proto/sidecar_pb2.py` — Generated protobuf stub
  - `sidecar-python/proto/sidecar_pb2_grpc.py` — Generated gRPC stub (gRPC 1.80.0)

  **WHY Each Reference Matters**:
  - Previous sprint reported crash loop with `ModuleNotFoundError: No module named 'sidecar_pb2'`. The fix was already applied (proto files moved to `proto/` subpackage). This task confirms the fix is in place.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Python sidecar protobuf files exist and are importable
    Tool: Bash (file checks)
    Steps:
      1. test -f sidecar-python/proto/sidecar_pb2.py && echo "PASS"
      2. test -f sidecar-python/proto/sidecar_pb2_grpc.py && echo "PASS"
      3. test -f sidecar-python/proto/__init__.py && echo "PASS"
      4. grep -q "from proto import sidecar_pb2" sidecar-python/worker.py && echo "PASS"
    Expected Result: All 4 checks PASS
    Failure Indicators: Any file missing or import path wrong
    Evidence: .sisyphus/evidence/task-3v-python-sidecar-verify.txt

  Scenario: Dockerfile exists and compose has hardened config
    Tool: Bash (file checks)
    Steps:
      1. test -f sidecar-python/Dockerfile && echo "PASS"
      2. grep -q "network_mode: none" deploy/docker-compose.sidecar-py.yml && echo "PASS"
      3. grep -q "cap_drop:" deploy/docker-compose.sidecar-py.yml && echo "PASS"
    Expected Result: All checks PASS
    Failure Indicators: Missing file or missing security config
    Evidence: .sisyphus/evidence/task-3v-python-sidecar-security.txt
  ```

  **Commit**: NO — Verify only, no code changes

- [x] 4. Formal Defer of Rust Sidecar with Score Ceiling

  **What to do**:
  - Create a deferral document at `.sisyphus/evidence/task-4-rust-deferral.md` stating:
    1. What was deferred: Rust sidecar Dockerfile + deployment
    2. Why: No Dockerfile exists (`sidecar/Dockerfile` absent), no compose entry, deployment is a separate effort
    3. What works: Library compiles, 252 tests pass, all document processing modules implemented
    4. What's missing: Containerized runtime, gRPC-over-Unix-socket deployment, hardened Docker config
    5. Office score ceiling without Rust: PDF text extraction works via Python sidecar fallback; XLSX/PPTX/DOCX via Python; Rust adds S3 streaming and advanced PDF ops
    6. Maximum Office score without Rust: 15/25 (vs 20-25/25 with Rust)
    7. Follow-up task: Create `sidecar/Dockerfile` + `deploy/docker-compose.sidecar-rust.yml` in next sprint
    8. Impact on total BEATO: ceiling is ~90/100 without Rust (vs 95+ with Rust)

  **Must NOT do**:
  - Do NOT create a Rust sidecar Dockerfile (that's the deferred work)
  - Do NOT claim Rust is production-ready
  - Do NOT inflate the Office score

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Documentation-only task
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T5, T6)
  - **Blocks**: T11
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `sidecar/Cargo.toml` — Shows the Rust sidecar crate with all dependencies
  - `sidecar/README.md` — Documents 252 tests passing, no Dockerfile

  **API/Type References**:
  - `deploy/docker-compose.sidecar-py.yml` — Pattern for what a Rust compose entry would look like

  **WHY Each Reference Matters**:
  - `Cargo.toml`: Proves the library is mature enough to defer (not abandon)
  - `docker-compose.sidecar-py.yml`: Shows the hardening pattern that a Rust deployment would follow

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Deferral document exists with all required sections
    Tool: Bash (file checks)
    Steps:
      1. test -f .sisyphus/evidence/task-4-rust-deferral.md && echo "PASS"
      2. grep -q "Office score ceiling" .sisyphus/evidence/task-4-rust-deferral.md && echo "PASS"
      3. grep -q "follow-up" .sisyphus/evidence/task-4-rust-deferral.md && echo "PASS"
      4. grep -q "maximum.*BEATO" .sisyphus/evidence/task-4-rust-deferral.md && echo "PASS"
    Expected Result: Document exists with ceiling, follow-up plan, and impact statement
    Failure Indicators: File missing or required sections absent
    Evidence: .sisyphus/evidence/task-4-rust-deferral.md

  Scenario: No Rust Dockerfile was created (deferral confirmed)
    Tool: Bash
    Steps:
      1. test ! -f sidecar/Dockerfile && echo "PASS: No Dockerfile (correctly deferred)"
    Expected Result: sidecar/Dockerfile does NOT exist
    Failure Indicators: sidecar/Dockerfile exists (task did the deferred work instead of deferring)
    Evidence: .sisyphus/evidence/task-4-no-rust-dockerfile.txt
  ```

  **Commit**: YES
  - Message: `docs(beato): document Rust sidecar deferral and Office score ceiling`
  - Files: `.sisyphus/evidence/task-4-rust-deferral.md`

- [x] 5. Implement browser.screenshot and browser.close Handlers

  **What to do**:
  - Implement two new handler functions in `bridge/pkg/rpc/browser.go`:
    1. `handleBrowserScreenshot(ctx context.Context, req *Request) (interface{}, *ErrorObj)`:
       - Parse `session_id` and optional `format` (default "png"), `full_page` (default false) from params
       - Look up active browser session (follow `handleBrowserStatus` pattern at browser.go:403)
       - If no active session, return controlled error: `{"error": {"code": -32000, "message": "no active browser session"}}`
       - If session exists, call the Jetski/browser-service screenshot endpoint
       - Return `{"image": "<base64-encoded>", "format": "png", "session_id": "..."}` on success
       - For now, if the browser-service screenshot API is not available, return a "not yet available" error with the correct response shape
    2. `handleBrowserClose(ctx context.Context, req *Request) (interface{}, *ErrorObj)`:
       - Parse optional `session_id` from params
       - Close the browser session/job (follow `handleBrowserComplete` pattern at browser.go:700)
       - If no active session, return idempotent success: `{"ok": true, "message": "no active session"}`
       - If session exists, close it and return `{"ok": true, "session_id": "..."}`
  - Register both handlers in `server.go` `registerHandlers()` map:
    - `"browser.screenshot": s.handleBrowserScreenshot`
    - `"browser.close": s.handleBrowserClose`
  - Both handlers MUST be wrapped with `BrowserRPCGroup` SafetyMiddleware (T1 wiring)

  **Must NOT do**:
  - Do NOT remove or modify `browser.complete`
  - Do NOT expose Jetski publicly
  - Do NOT allow browser actions without auth (T1 handles this)
  - Do NOT implement PII scrubbing for screenshots (risk-accept this sprint)
  - Do NOT change proto files
  - Do NOT touch existing 12 browser handler registrations

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: New handler implementation following existing patterns, requires understanding session lifecycle
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T4)
  - **Parallel Group**: Wave 2 (with T4, T6 after this completes)
  - **Blocks**: T6
  - **Blocked By**: None (can start immediately, auth wrapping from T1 applied later)

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/browser.go:121-230` — `handleBrowserNavigate`: Request parsing, session lookup, RPC call, response shaping. Follow this pattern exactly.
  - `bridge/pkg/rpc/browser.go:403-490` — `handleBrowserStatus`: Session status lookup pattern.
  - `bridge/pkg/rpc/browser.go:700-763` — `handleBrowserComplete`: Session completion/close pattern. `handleBrowserClose` is similar but idempotent.
  - `bridge/pkg/rpc/server.go:1231-1242` — Browser handler registrations in the `h` map. Add screenshot and close entries here.

  **API/Type References**:
  - `bridge/pkg/rpc/browser.go:1-40` — Browser session manager interface and request/response types.
  - `bridge/pkg/browser/` — Browser session management package.

  **Test References**:
  - `bridge/pkg/rpc/browser.go:121` — `handleBrowserNavigate` function signature: `func (s *Server) handleBrowserNavigate(ctx context.Context, req *Request) (interface{}, *ErrorObj)`

  **WHY Each Reference Matters**:
  - `handleBrowserNavigate` (browser.go:121): The canonical pattern for a browser RPC handler. Parse params → lookup session → call service → shape response.
  - `handleBrowserComplete` (browser.go:700): Close is similar to complete but idempotent — no error if session already gone.
  - `handleBrowserStatus` (browser.go:403): Screenshot needs session lookup, same as status check.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: browser.screenshot returns valid response (not method-not-found)
    Tool: Bash (go test)
    Preconditions: Handlers registered
    Steps:
      1. cd bridge && go test -v -run TestBrowserScreenshot ./pkg/rpc/...
      2. Verify response is not -32601 (method not found)
    Expected Result: Handler returns either success response or a controlled business error (e.g., "no active session"), NOT method-not-found
    Failure Indicators: Response code -32601 (handler not registered)
    Evidence: .sisyphus/evidence/task-5-browser-screenshot.txt

  Scenario: browser.close returns valid response (not method-not-found)
    Tool: Bash (go test)
    Preconditions: Handlers registered
    Steps:
      1. cd bridge && go test -v -run TestBrowserClose ./pkg/rpc/...
      2. Verify response is not -32601
    Expected Result: Handler returns ok:true or idempotent success
    Failure Indicators: Response code -32601
    Evidence: .sisyphus/evidence/task-5-browser-close.txt

  Scenario: browser.close is idempotent (no session → success, not error)
    Tool: Bash (go test)
    Steps:
      1. Call browser.close with no active session
      2. Assert response is {"ok": true, "message": "no active session"} or similar
    Expected Result: No error, idempotent success
    Failure Indicators: Returns error when no session exists
    Evidence: .sisyphus/evidence/task-5-browser-close-idempotent.txt

  Scenario: Existing browser handlers still work
    Tool: Bash (go test)
    Steps:
      1. cd bridge && go test -v -run TestBrowser ./pkg/rpc/... -count=1
      2. Assert 0 failures
    Expected Result: All existing browser tests still pass
    Failure Indicators: Any existing test breaks
    Evidence: .sisyphus/evidence/task-5-existing-browser-tests.txt
  ```

  **Commit**: YES
  - Message: `feat(rpc): implement browser.screenshot and browser.close handlers`
  - Files: `bridge/pkg/rpc/browser.go`, `bridge/pkg/rpc/server.go`
  - Pre-commit: `cd bridge && go test ./pkg/rpc/... -count=1`

- [ ] 6. Extend Browser Smoke Coverage to All 14 Methods

  **What to do**:
  - Extend `tests/test-browser-smoke.sh` to cover all 14 browser RPC methods:
    - Existing: browser.status (B1), browser.navigate (B2), browser.complete (B3)
    - Add: browser.fill, browser.click, browser.cancel, browser.wait_for_element, browser.wait_for_captcha, browser.wait_for_2fa, browser.fail, browser.list, browser.replay_diagnostics, browser.screenshot, browser.close
  - For each method, test:
    1. Valid JSON response (not method-not-found -32601)
    2. Auth enforced (call without token → expect -32011)
  - Follow the existing test structure (B0 prerequisites, B1-B3 pattern, PASS/FAIL counters)
  - External HTTPS navigation: test `browser.navigate` to `https://example.com` or explicitly mark as blocked with reason

  **Must NOT do**:
  - Do NOT remove existing B1-B3 test scenarios
  - Do NOT add tests that require VPS deployment
  - Do NOT test PII scrubbing (out of scope)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Follows existing shell test pattern, extends coverage
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (sequential after T5)
  - **Blocks**: T11
  - **Blocked By**: T5

  **References**:

  **Pattern References**:
  - `tests/test-browser-smoke.sh:1-100` — Existing test structure: `rpc_call_auth` helper, B0/B1/B2/B3 scenarios, dual-transport (Unix socket + HTTPS), evidence output.

  **API/Type References**:
  - `bridge/pkg/rpc/server.go:1231-1242` — All 12+2 browser handler registrations for method name reference.

  **Test References**:
  - `tests/test-browser-smoke.sh` — Existing 265-line test with B0-B3 scenarios.

  **WHY Each Reference Matters**:
  - `test-browser-smoke.sh`: The exact pattern to extend. Same helpers, same transport, same evidence format.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All 14 browser methods return valid JSON
    Tool: Bash (tests/test-browser-smoke.sh)
    Preconditions: T5 handlers registered
    Steps:
      1. bash tests/test-browser-smoke.sh
      2. Assert all 14 methods tested, 0 FAIL
    Expected Result: 14/14 browser methods return valid JSON (not method-not-found)
    Failure Indicators: Any method returns -32601
    Evidence: .sisyphus/evidence/task-6-browser-rpc-smoke.txt

  Scenario: Auth enforcement on all browser methods
    Tool: Bash (tests/test-browser-smoke.sh)
    Steps:
      1. For each browser method, call without token
      2. Assert each returns -32011 (RPCAuthRequired)
    Expected Result: All 14 methods reject unauthenticated requests
    Failure Indicators: Any method succeeds without token
    Evidence: .sisyphus/evidence/task-6-browser-auth-check.txt
  ```

  **Commit**: YES
  - Message: `test(browser): extend smoke coverage to all 14 browser RPCs`
  - Files: `tests/test-browser-smoke.sh`
  - Pre-commit: `bash -n tests/test-browser-smoke.sh`

- [ ] 7-V. Verify Matrix Client is Operational (ALREADY DONE — Confirm Only)

  **What to do**:
  - Confirm the Matrix client code is complete and functional:
    1. Verify `bridge/pkg/matrix/client.go` exists with Login, SendMessage, GetMessages, Sync, JoinRoom methods
    2. Verify `bridge/pkg/matrix/client_test.go` exists with test coverage
    3. Verify `bridge/internal/adapter/` has MatrixAdapter implementation
    4. Verify `docker-compose-full.yml` has Matrix Conduit service with healthcheck
    5. Verify Matrix RPC handlers are registered: `matrix.status`, `matrix.login`, `matrix.send`, `matrix.receive`, `matrix.join_room`
  - Note: VPS runtime disconnection is an operational issue (stale credentials), NOT a code gap

  **Must NOT do**:
  - Do NOT attempt to fix Matrix disconnection on VPS
  - Do NOT modify any Matrix client files
  - Do NOT rotate Matrix credentials

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: File existence and content verification only
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T8, T9-V, T10)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/matrix/client.go` — 382 lines, complete Matrix client
  - `docker-compose-full.yml` — Matrix Conduit service definition

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Matrix client code is complete
    Tool: Bash (file checks)
    Steps:
      1. test -f bridge/pkg/matrix/client.go && echo "PASS"
      2. test -f bridge/pkg/matrix/client_test.go && echo "PASS"
      3. grep -q "func.*Login" bridge/pkg/matrix/client.go && echo "PASS"
      4. grep -q "func.*Sync" bridge/pkg/matrix/client.go && echo "PASS"
    Expected Result: All checks PASS — Matrix client code is structurally complete
    Failure Indicators: Missing files or methods
    Evidence: .sisyphus/evidence/task-7v-matrix-verify.txt

  Scenario: Matrix RPC handlers registered
    Tool: Bash (grep)
    Steps:
      1. grep -q '"matrix.status"' bridge/pkg/rpc/server.go && echo "PASS"
      2. grep -q '"matrix.login"' bridge/pkg/rpc/server.go && echo "PASS"
      3. grep -q '"matrix.send"' bridge/pkg/rpc/server.go && echo "PASS"
    Expected Result: Matrix RPC handlers registered in server
    Failure Indicators: Missing handler registrations
    Evidence: .sisyphus/evidence/task-7v-matrix-handlers.txt
  ```

  **Commit**: NO — Verify only

- [ ] 8. Revalidate Email Outbox Lifecycle After Auth Enforcement

  **What to do**:
  - After T1 (auth wiring) is complete, revalidate the email queue/outbox lifecycle:
    1. Run `tests/test-email-pipeline.sh` with auth tokens in requests
    2. Verify `email.queue_status` requires auth (rejects without token)
    3. Verify outbox entry creation still works with valid token
    4. Verify `approve_email` and `deny_email` still work with valid token
    5. Verify queue persists after bridge restart
  - If tests fail, diagnose whether it's auth wiring or email logic and fix
  - Scope: No Postfix, no DNS, no external email send — local lifecycle only

  **Must NOT do**:
  - Do NOT send real email externally
  - Do NOT deploy Postfix/DNS
  - Do NOT store raw email body in queue table
  - Do NOT disable auth to make tests pass

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Integration testing that may require code fixes
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (sequential after T1+T2)
  - **Blocks**: T11
  - **Blocked By**: T1, T2

  **References**:

  **Pattern References**:
  - `tests/test-email-pipeline.sh` — 308 lines, M0-M6 scenarios. Uses `rpc_call_auth` helper.
  - `bridge/pkg/rpc/email_queue.go` — Email queue handler
  - `bridge/pkg/rpc/email_approval_test.go` — 13 existing tests

  **API/Type References**:
  - `bridge/pkg/rpc/server.go:1318-1321` — Email approval handlers: approve_email, deny_email, email_approval_status, email.list_pending
  - `bridge/pkg/rpc/server.go:1373-1376` — Email queue handlers: email.queue_status, email.get, email.retry, email.list

  **WHY Each Reference Matters**:
  - `test-email-pipeline.sh`: The integration test that must pass after auth wiring. If the test harness doesn't send tokens, it will break.
  - `email_approval_test.go`: Existing unit tests that should still pass unchanged.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Email queue_status requires auth
    Tool: Bash (go test)
    Preconditions: T1 auth wiring complete
    Steps:
      1. Call email.queue_status without token
      2. Assert response error code -32011
    Expected Result: Auth error returned
    Failure Indicators: Request succeeds without token
    Evidence: .sisyphus/evidence/task-8-email-auth-rejection.txt

  Scenario: Email pipeline lifecycle works with auth
    Tool: Bash (tests/test-email-pipeline.sh)
    Preconditions: T1+T2 complete, auth tokens in test harness
    Steps:
      1. bash tests/test-email-pipeline.sh
      2. Assert all M1-M6 scenarios PASS
    Expected Result: All email pipeline tests pass with auth enforcement
    Failure Indicators: Any test fails due to auth rejection when token is provided
    Evidence: .sisyphus/evidence/task-8-email-pipeline.txt

  Scenario: Approve/deny email still works
    Tool: Bash (go test)
    Steps:
      1. cd bridge && go test -v -run TestEmailApproval ./pkg/rpc/... -count=1
      2. Assert all tests pass
    Expected Result: Approval flow works correctly with auth
    Failure Indicators: Tests fail after auth wiring
    Evidence: .sisyphus/evidence/task-8-email-approval.txt
  ```

  **Commit**: MAYBE — only if code changes needed
  - Message: `test(email): revalidate outbox lifecycle after auth enforcement` or `fix(email): update email handlers for auth compatibility`
  - Files: `tests/test-email-pipeline.sh` (if harness needs token updates)

- [ ] 9-V. Verify Healthchecks are Comprehensive (ALREADY DONE — Confirm Only)

  **What to do**:
  - Confirm healthchecks are comprehensive across all compose files:
    1. Check `docker-compose-full.yml` for matrix, bridge healthchecks
    2. Check `docker-compose.yml` for qdrant, vault, caddy healthchecks
    3. Check `deploy/docker-compose.beato.yml` for jetski healthcheck
    4. Check `deploy/docker-compose.sidecar-py.yml` for sidecar health status
    5. Verify `bridge/pkg/rpc/server.go` has `health.check` handler registered
  - Note: After T1 wiring, verify `health.check` is in the EXCLUSION list (not auth-gated)

  **Must NOT do**:
  - Do NOT modify any healthcheck configurations
  - Do NOT add new healthchecks

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: File existence and content verification
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T7-V, T8, T10)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `docker-compose-full.yml` — Full stack compose with healthchecks
  - `docker-compose.yml` — Base compose with healthchecks
  - `deploy/docker-compose.beato.yml` — Jetski healthcheck

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Healthchecks defined across compose files
    Tool: Bash (grep)
    Steps:
      1. grep -c "healthcheck:" docker-compose-full.yml docker-compose.yml deploy/docker-compose.beato.yml
      2. Assert each file has at least 1 healthcheck
    Expected Result: All compose files have healthcheck definitions
    Failure Indicators: Any file missing healthcheck
    Evidence: .sisyphus/evidence/task-9v-healthcheck-verify.txt

  Scenario: health.check handler registered and excluded from auth
    Tool: Bash (grep)
    Steps:
      1. grep -q '"health.check"' bridge/pkg/rpc/server.go && echo "PASS: registered"
      2. Verify health.check is NOT in the BEATO-wrapped handler set (exclusion list)
    Expected Result: health.check is registered and excluded from SafetyMiddleware
    Failure Indicators: health.check returns auth error after T1 wiring
    Evidence: .sisyphus/evidence/task-9v-health-exclusion.txt
  ```

  **Commit**: NO — Verify only

- [ ] 10. AppArmor Risk Acceptance with Compensating Controls

  **What to do**:
  - Create a risk acceptance document at `.sisyphus/evidence/task-10-apparmor-risk.md` stating:
    1. AppArmor profile `armorclaw-office-worker` does not exist
    2. AppArmor is commented out in `deploy/docker-compose.sidecar-py.yml` line 60-64
    3. Compensating controls currently in place:
       - `network_mode: none` — no network access
       - `cap_drop: ALL` — all Linux capabilities dropped
       - `read_only: true` — read-only root filesystem
       - `security_opt: no-new-privileges:true` — prevents privilege escalation
       - HMAC-SHA256 token validation — sidecar can only process authenticated requests
       - Unix domain socket only — no TCP exposure
    4. What would an AppArmor profile add: restrict file paths, prevent execve of non-whitelisted binaries, limit mount/syscall access
    5. Follow-up task: Create `deploy/apparmor/armorclaw-office-worker` profile and integrate into compose
    6. Risk level: MEDIUM — current Docker hardening provides significant containment

  **Must NOT do**:
  - Do NOT silently leave AppArmor disabled without documentation
  - Do NOT loosen `network_mode`
  - Do NOT add privileged mode
  - Do NOT weaken HMAC
  - Do NOT remove `no-new-privileges`

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Documentation-only task
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T7-V, T8, T9-V)
  - **Blocks**: T11
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `deploy/docker-compose.sidecar-py.yml:60-64` — AppArmor commented out with explanation

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Risk acceptance document exists with compensating controls
    Tool: Bash (file checks)
    Steps:
      1. test -f .sisyphus/evidence/task-10-apparmor-risk.md && echo "PASS"
      2. grep -q "network_mode.*none" .sisyphus/evidence/task-10-apparmor-risk.md && echo "PASS"
      3. grep -q "cap_drop" .sisyphus/evidence/task-10-apparmor-risk.md && echo "PASS"
      4. grep -q "follow-up" .sisyphus/evidence/task-10-apparmor-risk.md && echo "PASS"
    Expected Result: Document exists listing all compensating controls and follow-up plan
    Failure Indicators: File missing or required sections absent
    Evidence: .sisyphus/evidence/task-10-apparmor-risk.md

  Scenario: Docker hardening still in place (compensating controls verified)
    Tool: Bash (grep)
    Steps:
      1. grep -q "network_mode: none" deploy/docker-compose.sidecar-py.yml && echo "PASS"
      2. grep -q "cap_drop:" deploy/docker-compose.sidecar-py.yml && echo "PASS"
      3. grep -q "no-new-privileges" deploy/docker-compose.sidecar-py.yml && echo "PASS"
    Expected Result: All compensating controls present in compose file
    Failure Indicators: Any control removed
    Evidence: .sisyphus/evidence/task-10-compensating-controls.txt
  ```

  **Commit**: YES
  - Message: `docs(security): document AppArmor risk acceptance with compensating controls`
  - Files: `.sisyphus/evidence/task-10-apparmor-risk.md`

- [ ] 11. Full BEATO Runtime Re-Score

  **What to do**:
  - Regenerate the runtime BEATO report from evidence gathered in T1-T10
  - Update `tests/reports/beato-runtime-report.md` with:
    1. Executive Summary — before/after comparison
    2. Score Table — per-pillar scores with evidence links
    3. Browser: score out of 25 — assess 14/14 methods registered, auth enforced, smoke passing
    4. Email: score out of 20 — assess auth enforced, outbox lifecycle validated, queue persistence
    5. Text: score out of 20 — assess Matrix client code complete (operational disconnect is ops)
    6. Office: score out of 25 — assess Python sidecar operational, Rust deferred (document ceiling), document pipeline working
    7. Audio: score out of 10 — audit-only points (deferred per plan)
    8. Before/After Comparison — 61/100 → target ≥85/100
    9. Evidence Links — per-task evidence files
    10. Remaining Runtime Gaps — what's still open
    11. Security Assessment — auth enforcement status, AppArmor posture
    12. Next Sprint Recommendation — Rust sidecar deployment, AppArmor profile, Matrix reconnection

  **Scoring Rules** (from existing rubric — DO NOT CHANGE):
  - Code existence does not score unless deployed and verified
  - Unauthenticated sensitive RPCs lose points
  - Crash-looping services score zero for runtime
  - Deferred Audio scores audit-only points
  - Unknown status must be marked unknown, not assumed green
  - Optional slipped tasks must be documented

  **Must NOT do**:
  - Do NOT change the scoring rubric
  - Do NOT inflate scores without evidence
  - Do NOT claim 100% unless runtime tests prove it
  - Do NOT hide failures by changing methodology

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Evidence-based report writing
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4 (with T12 after this)
  - **Blocks**: T12
  - **Blocked By**: T1-T10

  **References**:

  **Pattern References**:
  - `tests/reports/beato-runtime-report.md` — Current report (61/100). Follow this format.
  - `.sisyphus/evidence/beato-remediation-index.md` — Previous evidence index.

  **WHY Each Reference Matters**:
  - Current report shows the exact format and rubric. New report must follow the same structure with updated scores and fresh evidence.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: BEATO report exists with updated scores
    Tool: Bash (file checks)
    Steps:
      1. test -f tests/reports/beato-runtime-report.md && echo "PASS"
      2. grep -q "Executive Summary" tests/reports/beato-runtime-report.md && echo "PASS"
      3. grep -q "Before/After" tests/reports/beato-runtime-report.md && echo "PASS"
      4. Extract total score, assert >= 85
    Expected Result: Report exists with all required sections and total ≥ 85
    Failure Indicators: Score < 85 or missing sections
    Evidence: .sisyphus/evidence/task-11-report-check.txt

  Scenario: Score is evidence-based (every claim links to evidence)
    Tool: Bash (grep)
    Steps:
      1. grep -c "evidence/" tests/reports/beato-runtime-report.md
      2. Assert at least 10 evidence references
    Expected Result: Every score claim references an evidence file
    Failure Indicators: Claims without evidence backing
    Evidence: .sisyphus/evidence/task-11-evidence-links.txt
  ```

  **Commit**: YES
  - Message: `docs(beato): update runtime score after v1.1 remediation`
  - Files: `tests/reports/beato-runtime-report.md`

- [ ] 12. Evidence Index

  **What to do**:
  - Create `.sisyphus/evidence/beato-v1.1-index.md` — a single index of all evidence files
  - Format:

  ```markdown
  # BEATO v1.1 Runtime Fix — Evidence Index

  | Task | Evidence Files | Pass/Fail | Notes |
  |------|---------------|-----------|-------|
  | T1 | task-1-healthcheck-no-auth.txt, task-1-browser-no-auth-rejection.txt, ... | PASS/FAIL | Auth wiring |
  | T2 | task-2-auth-regression-tests.txt, task-2-exclusion-verification.txt | PASS/FAIL | Auth tests |
  | T3-V | task-3v-python-sidecar-verify.txt, task-3v-python-sidecar-security.txt | PASS | Verify only |
  | ... | ... | ... | ... |
  | T11 | beato-runtime-report.md | PASS | Re-score |
  ```

  - Every evidence file from T1-T11 must be listed
  - Pass/Fail status must match actual results

  **Must NOT do**:
  - Do NOT create evidence files that don't exist
  - Do NOT mark failed tasks as PASS

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple index creation from existing files
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4 (sequential after T11)
  - **Blocks**: F1-F4
  - **Blocked By**: T11

  **References**:
  - `.sisyphus/evidence/` — Directory containing all evidence files from T1-T11

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Evidence index exists and is complete
    Tool: Bash (file checks)
    Steps:
      1. test -f .sisyphus/evidence/beato-v1.1-index.md && echo "PASS"
      2. Count evidence files listed, count actual files in .sisyphus/evidence/
      3. Assert all task-*.txt files are referenced in index
    Expected Result: Index exists and references every evidence file
    Failure Indicators: Missing files from index
    Evidence: .sisyphus/evidence/task-12-index-completeness.txt
  ```

  **Commit**: YES
  - Message: `docs(beato): add evidence index for v1.1 runtime remediation`
  - Files: `.sisyphus/evidence/beato-v1.1-index.md`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [ ] F1. **Security Audit** — `oracle`
  Read the plan end-to-end. Verify: auth enforced on all BEATO-sensitive handlers, `health.*` excluded from auth, tokens not logged, HMAC validation unchanged, sidecars not public, SQLCipher unchanged, AppArmor posture documented. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Auth [N/N] | Exclusions [N/N] | No Token Logs [YES/NO] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Runtime QA** — `unspecified-high`
  Run `cd bridge && go test -v ./pkg/rpc/...` + all shell smoke tests. Review all changed files for: `as any`/type assertions, empty catches, console.log, commented-out code. Check AI slop patterns. Run browser smoke, email pipeline, sidecar docs tests.
  Output: `Build [PASS/FAIL] | Tests [N pass/N fail] | Smoke [N/N pass] | VERDICT`

- [ ] F3. **Scope Compliance** — `oracle`
  Verify: no Postfix/DNS, no Audio implementation, no rubric changes, no proto changes, no Rust deployment, no Matrix reconnection code, no `rpc_safety.go` modifications. Detect unaccounted changes. Check guardrails.
  Output: `Guardrails [N/N clean] | Unaccounted [CLEAN/N files] | VERDICT`

- [ ] F4. **Release Recommendation** — `writing`
  Produce: APPROVE / CONDITIONAL APPROVE / REJECT. Final BEATO runtime score. Remaining gaps. Recommended next sprint.
  Output: `.sisyphus/evidence/final-release-recommendation.md`

---

## Commit Strategy

| Task | Commit | Message |
|------|--------|---------|
| T1 | YES | `fix(rpc): wire SafetyMiddleware into BEATO handler registration` |
| T2 | YES | `test(rpc): add per-handler auth regression tests for BEATO handlers` |
| T3-V | NO | Verify only — no code change |
| T4 | YES | `docs(beato): document Rust sidecar deferral and Office score ceiling` |
| T5 | YES | `feat(rpc): implement browser.screenshot and browser.close handlers` |
| T6 | YES | `test(browser): extend smoke coverage to all 14 browser RPCs` |
| T7-V | NO | Verify only — no code change |
| T8 | MAYBE | `test(email): revalidate outbox lifecycle after auth enforcement` or code fix |
| T9-V | NO | Verify only — no code change |
| T10 | YES | `docs(security): document AppArmor risk acceptance with compensating controls` |
| T11 | YES | `docs(beato): update runtime score after v1.1 remediation` |
| T12 | YES | `docs(beato): add evidence index for v1.1 runtime remediation` |

---

## Success Criteria

### Verification Commands
```bash
cd bridge && go test ./pkg/rpc/... -run "Auth" -count=1  # Expected: all PASS
cd bridge && go test ./pkg/rpc/... -count=1               # Expected: all PASS, 0 failures
bash tests/test-browser-smoke.sh                           # Expected: 14/14 methods
bash tests/test-email-pipeline.sh                          # Expected: all PASS
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass
- [ ] BEATO score ≥ 85/100 (or honestly reported if lower)
- [ ] Evidence index complete
