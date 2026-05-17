# Finish All Verified Features — Make Them Run

## TL;DR

> **Quick Summary**: Fix 5 subsystems identified during verification to be production-running: warm dispatch cleanup, Rust sidecar full fix, v6 wiring gap, ArmorChat control-plane (with backend RPC wiring), and Jetski/browser path hardening. All work follows TDD.
> 
> **Deliverables**:
> - Rust sidecar binary compiles and all 8 RPCs work end-to-end
> - v6 microkernel wiring gap closed (vaultClient → MCPRouter), tool execution real (not mock), stays gated
> - ArmorChat has 4 new ViewModels, Home dashboard, all placeholder screens replaced, orphaned screens wired
> - Jetski CDP proxy gates approvals, PII scrub active in default config, full-path E2E test
> - Warm dispatch dead code completely removed
> - All doc discrepancies fixed
> 
> **Estimated Effort**: XL (6 areas, ~25 tasks)
> **Parallel Execution**: YES - 5 waves
> **Critical Path**: Sidecar proto sync → Sidecar compile fix → Sidecar impl → E2E test

---

## Context

### Original Request
User verified 6 subsystems against source code and found significant gaps. Requested plan to make all features "actually running, not just documented."

### Interview Summary
**Key Discussions**:
- Browser path: Both systems running, browser-service (Playwright) canonical, Jetski as standalone CDP proxy
- v6: Fix wiring + stubs, keep V6Microkernel=false default — ready but not forced
- Sidecar: Full fix — sync protos, fix 74 compilation errors, implement real RPCs
- ArmorChat backend: Partially ready — secretary RPCs exist but not in main server dispatch, email.list_pending missing, account.delete missing
- Test strategy: TDD (RED → GREEN → REFACTOR) for all new implementation

**Research Findings**:
- Sidecar: Rust proto has 8 RPCs vs Go's 7. Generated code stale. ONNX fallback never invoked. HealthCheck returns zeros.
- v6: setupMCPRouter() doesn't accept vaultClient. executeTool() returns mock map.
- ArmorChat: BridgeApi.kt has zero secretary/workflow/account RPC methods. All 4 ViewModels need building from scratch. 13 UI components exist (561-line WorkflowTimeline, etc.) but no host screens.
- Jetski: ApprovalClient exists but proxy.go doesn't call it. PII scrub default is OFF (Free-Ride mode).
- Warm dispatch: 277 lines dead code across 4 locations.

### Metis Review
**Identified Gaps** (addressed):
- ArmorChat client (BridgeApi.kt) has no secretary/workflow RPC methods — confirmed as build-from-scratch, not just wiring
- Secretary RPC handler exists in separate file not in main server dispatch — must register methods in server.go
- Team and Voice have NO backend at all — explicitly out of scope

---

## Work Objectives

### Core Objective
Make all 5 verified subsystem gaps production-ready with real functionality, real tests, and accurate documentation.

### Concrete Deliverables
- `sidecar/` compiles cleanly (`cargo build` succeeds)
- `bridge/pkg/mcp/router.go` executeTool() performs real tool execution
- ArmorChat Home screen shows agent list, pending approvals, active workflows
- Jetski CDP proxy requires approval for sensitive operations
- Zero warm dispatch references in codebase (except historical CHANGELOG entries)
- All doc files reflect single consistent truth

### Definition of Done
- [ ] `cargo build --manifest-path sidecar/Cargo.toml` succeeds with 0 errors
- [ ] `go build ./...` in bridge/ succeeds
- [ ] `go test ./...` in bridge/ passes (including new v6 wiring tests)
- [ ] ArmorChat builds and all 11 routes have real screens (0 placeholders)
- [ ] Jetski E2E test exercises approval gating
- [ ] `grep -r "PrewarmPool\|GetRunningInstance" bridge/` returns 0 matches
- [ ] No doc file claims XLSX routes to Python

### Must Have
- Rust sidecar binary compiles with all 8 RPCs functional
- v6 vaultClient wiring gap closed
- ArmorChat 4 ViewModels + Home dashboard + orphaned screens wired
- Jetski approval gating active in CDP proxy
- Warm dispatch dead code deleted
- TDD for all new code (tests written first)

### Must NOT Have (Guardrails)
- Do NOT enable V6Microkernel by default — keep it opt-in
- Do NOT change the canonical browser path away from browser-service
- Do NOT remove SQLCipher encryption
- Do NOT bypass Matrix as control plane
- Do NOT weaken existing approval flows
- Do NOT add Team or Voice backends (out of scope — no backend exists)
- Do NOT add AI slop: excessive comments, over-abstraction, generic names
- Do NOT touch existing working code without a test covering its current behavior first

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** - ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (Go testing, Rust cargo test, Kotlin JUnit)
- **Automated tests**: TDD — RED (failing test) → GREEN (minimal impl) → REFACTOR
- **Framework**: Go `testing`, Rust `#[test]`, Kotlin JUnit + Compose UI testing

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Backend (Go/Rust)**: Bash (go test, cargo test) — run tests, assert pass count
- **Android**: Bash (./gradlew test, ./gradlew assembleDebug) — build + unit tests
- **Browser path**: Bash (go test) — E2E tests with mock servers
- **Doc verification**: Grep for specific claims, assert count

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — cleanup + foundation + RPC wiring):
├── Task 1:  Delete warm dispatch dead code [quick]
├── Task 2:  Sync sidecar proto files [quick]
├── Task 3:  Wire secretary RPC into main server [unspecified-high]
├── Task 4:  Add missing RPC endpoints (email.list_pending, account.delete) [unspecified-high]
├── Task 5:  Fix v6 vaultClient wiring gap [quick]
└── Task 6:  Add BridgeApi methods for secretary/workflow/account [unspecified-high]

Wave 2 (After Wave 1 — sidecar compile + v6 execution + Jetski):
├── Task 7:  Fix Rust sidecar compilation (74 errors) [deep]
├── Task 8:  Implement real tool execution in MCPRouter [deep]
├── Task 9:  Wire Jetski approval into CDP proxy [unspecified-high]
├── Task 10: Enable PII scrub in Jetski default config [quick]
├── Task 11: ArmorChat AgentManagementViewModel + AgentScreen [deep]
├── Task 12: ArmorChat HitlViewModel + ApprovalScreen [deep]
└── Task 13: ArmorChat WorkflowViewModel + WorkflowScreen [deep]

Wave 3 (After Wave 2 — sidecar RPCs + ArmorChat screens):
├── Task 14: Implement sidecar real HealthCheck telemetry [unspecified-high]
├── Task 15: Implement sidecar ProcessDocument/convert [deep]
├── Task 16: Wire sidecar ONNX OCR fallback [unspecified-high]
├── Task 17: ArmorChat Home dashboard + AccountDeletion screen [deep]
├── Task 18: Wire ArmorChat orphaned screens (KeyRecovery, Secrets, Migration) [quick]
└── Task 19: ArmorChat EmailApproval real screen (replace placeholder) [unspecified-high]

Wave 4 (After Wave 3 — E2E + integration + docs):
├── Task 20: Sidecar E2E integration test (Go client → Rust server) [unspecified-high]
├── Task 21: Browser path full E2E test (Bridge → browser-service → result) [unspecified-high]
├── Task 22: Fix all doc discrepancies (XLSX routing, v6 status, browser paths, sidecar state, ArmorChat capabilities) [writing]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay

Critical Path: Task 2 → Task 7 → Task 14/15/16 → Task 20 → F1-F4
Parallel Speedup: ~65% faster than sequential
Max Concurrent: 7 (Wave 2)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | — | — | 1 |
| 2 | — | 7 | 1 |
| 3 | — | 6, 11, 12, 13 | 1 |
| 4 | — | 6, 17 | 1 |
| 5 | — | 8 | 1 |
| 6 | 3, 4 | 11, 12, 13, 17, 19 | 1 |
| 7 | 2 | 14, 15, 16, 20 | 2 |
| 8 | 5 | 20 | 2 |
| 9 | — | 21 | 2 |
| 10 | — | 21 | 2 |
| 11 | 6 | 17 | 2 |
| 12 | 6 | 17, 19 | 2 |
| 13 | 6 | 17 | 2 |
| 14 | 7 | 20 | 3 |
| 15 | 7 | 20 | 3 |
| 16 | 7 | 20 | 3 |
| 17 | 11, 12, 13 | — | 3 |
| 18 | — | — | 3 |
| 19 | 12 | — | 3 |
| 20 | 7, 14, 15, 16, 8, 9, 10 | F1-F4 | 4 |
| 21 | 8, 9, 10 | F1-F4 | 4 |
| 22 | 1-21 | F1-F4 | 4 |

### Agent Dispatch Summary

- **Wave 1**: 6 tasks — 3 `quick`, 3 `unspecified-high`
- **Wave 2**: 7 tasks — 3 `deep`, 3 `unspecified-high`, 1 `quick`
- **Wave 3**: 6 tasks — 2 `deep`, 2 `unspecified-high`, 2 `quick`
- **Wave 4**: 3 tasks — 2 `unspecified-high`, 1 `writing`
- **FINAL**: 4 tasks — 1 `oracle`, 2 `unspecified-high`, 1 `deep`

---

## TODOs

- [x] 1. Delete warm dispatch dead code

  **What to do**:
  - Delete `bridge/pkg/secretary/task_dispatch.go` (1-line empty stub)
  - Delete `PrewarmPool` system from `bridge/pkg/docker/client.go` lines 912-1050 (~138 lines: PrewarmPoolConfig, PrewarmPool, NewPrewarmPool, Acquire, Start, Stop, Size, refillLoop)
  - Delete `GetRunningInstance` method from `bridge/pkg/studio/factory.go` lines 409-419 (~11 lines)
  - Delete 3 dead tests from `bridge/pkg/studio/factory_test.go` lines 529-655 (~127 lines: TestGetRunningInstance_*)
  - Fix stale comment in `bridge/pkg/secretary/task_scheduler.go` line 27: remove "warm dispatch" reference
  - Fix stale comment in `bridge/pkg/secretary/task_scheduler.go` line 162: remove "warm dispatch" reference
  - Fix stale comment in `bridge/pkg/secretary/task_scheduler_test.go` line 308: simplify assertion message

  **Must NOT do**:
  - Do NOT touch CHANGELOG.md or review.md (historical records are expected)
  - Do NOT remove `coldDispatch` references (live code path)
  - Do NOT remove `prewarmSessionFile` in container/ (unrelated to warm dispatch)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2-6)
  - **Blocks**: Nothing
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `bridge/pkg/docker/client.go:912-1050` — PrewarmPool dead code block to delete
  - `bridge/pkg/studio/factory.go:409-419` — GetRunningInstance dead method
  - `bridge/pkg/studio/factory_test.go:529-655` — Dead tests for GetRunningInstance
  - `bridge/pkg/secretary/task_dispatch.go` — Empty stub file (delete entirely)
  - `bridge/pkg/secretary/task_scheduler.go:27,162` — Stale comments to fix

  **Acceptance Criteria**:
  - [ ] `go build ./...` succeeds from bridge/ directory
  - [ ] `go test ./...` succeeds from bridge/ directory
  - [ ] `grep -r "PrewarmPool" bridge/pkg/` returns 0 matches
  - [ ] `grep -r "GetRunningInstance" bridge/pkg/` returns 0 matches (except in deleted files)
  - [ ] `grep "warm.dispatch\|warmDispatch" bridge/pkg/secretary/task_scheduler.go` returns 0 matches

  **QA Scenarios**:

  ```
  Scenario: Go build succeeds after dead code removal
    Tool: Bash
    Steps:
      1. cd bridge && go build ./...
    Expected Result: Exit code 0, no errors
    Evidence: .sisyphus/evidence/task-1-go-build.txt

  Scenario: Go tests pass after dead code removal
    Tool: Bash
    Steps:
      1. cd bridge && go test ./...
    Expected Result: All tests pass, 0 failures
    Evidence: .sisyphus/evidence/task-1-go-test.txt

  Scenario: No warm dispatch remnants in non-historical code
    Tool: Bash
    Steps:
      1. grep -r "PrewarmPool\|GetRunningInstance" bridge/pkg/
    Expected Result: grep returns exit code 1 (no matches)
    Evidence: .sisyphus/evidence/task-1-grep-clean.txt
  ```

  **Commit**: YES
  - Message: `chore(cleanup): remove warm dispatch dead code`
  - Files: bridge/pkg/secretary/task_dispatch.go, bridge/pkg/docker/client.go, bridge/pkg/studio/factory.go, bridge/pkg/studio/factory_test.go, bridge/pkg/secretary/task_scheduler.go, bridge/pkg/secretary/task_scheduler_test.go
  - Pre-commit: `cd bridge && go build ./... && go test ./...`

- [x] 2. Sync sidecar proto files between Rust and Go

  **What to do**:
  - Read `sidecar/src/grpc/proto/sidecar.proto` (Rust, 8 RPCs including QueryDocuments)
  - Read `bridge/pkg/sidecar/sidecar.proto` (Go, 7 RPCs, no QueryDocuments)
  - Decide: either add QueryDocuments to Go proto OR remove it from Rust proto
  - If adding to Go: add `QueryDocuments`, `QueryDocumentsRequest`, `QueryDocumentsResponse`, `DocumentChunk` message types
  - If removing from Rust: remove the RPC and messages from Rust proto
  - Regenerate Rust proto code: `cd sidecar && cargo build` (requires protoc + tonic-build)
  - If protoc not available: manually update `sidecar/src/grpc/proto/armorclaw.sidecar.v1.rs` to match the proto source
  - Verify generated code matches proto source
  - Run `cargo check` to verify proto consistency

  **Must NOT do**:
  - Do NOT remove QueryDocuments from Rust server.rs implementation (keep the code, just ensure proto matches)
  - Do NOT break existing Go gRPC client compatibility

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3-6)
  - **Blocks**: Task 7 (sidecar compilation fix)
  - **Blocked By**: None

  **References**:
  - `sidecar/src/grpc/proto/sidecar.proto` — Rust proto source (8 RPCs)
  - `bridge/pkg/sidecar/sidecar.proto` — Go proto source (7 RPCs)
  - `sidecar/src/grpc/proto/armorclaw.sidecar.v1.rs` — Generated Rust code (stale, missing QueryDocuments)
  - `sidecar/build.rs` — Proto build script (tonic-build)

  **Acceptance Criteria**:
  - [ ] Both proto files have identical RPC definitions
  - [ ] Generated Rust code includes all RPCs from proto source
  - [ ] `cargo check --manifest-path sidecar/Cargo.toml` shows proto-related errors resolved

  **QA Scenarios**:

  ```
  Scenario: Proto files have identical RPC method names
    Tool: Bash
    Steps:
      1. Extract RPC names from sidecar/src/grpc/proto/sidecar.proto
      2. Extract RPC names from bridge/pkg/sidecar/sidecar.proto
      3. diff the two lists
    Expected Result: Identical RPC method names
    Evidence: .sisyphus/evidence/task-2-proto-diff.txt

  Scenario: Cargo check reports no proto-related errors
    Tool: Bash
    Steps:
      1. cargo check --manifest-path sidecar/Cargo.toml 2>&1 | grep -i proto
    Expected Result: No proto-related errors (or fewer than before)
    Evidence: .sisyphus/evidence/task-2-cargo-check.txt
  ```

  **Commit**: YES
  - Message: `fix(sidecar): sync proto files between Rust and Go`
  - Files: sidecar/src/grpc/proto/sidecar.proto, bridge/pkg/sidecar/sidecar.proto, sidecar/src/grpc/proto/armorclaw.sidecar.v1.rs (if manually updated)

- [x] 3. Wire secretary RPC methods into main server

  **What to do**:
  - Read `bridge/pkg/rpc/server.go` — find the handler registration table (lines 830-901)
  - Read `bridge/pkg/secretary/rpc.go` — understand the SecretaryRPCHandler and its Handle() method
  - Read `bridge/pkg/secretary/secretary_commands.go` — understand what commands exist
  - Add secretary.* and task.* methods to the main server's handler switch
  - Create thin wrapper methods in `bridge/pkg/rpc/` that delegate to the SecretaryRPCHandler
  - Add the secretary handler as a field on the Server struct
  - Register these methods: `secretary.start_workflow`, `secretary.get_workflow`, `secretary.cancel_workflow`, `secretary.advance_workflow`, `secretary.list_templates`, `secretary.create_template`, `secretary.get_template`, `secretary.delete_template`, `secretary.update_template`, `task.create`, `task.list`, `task.cancel`, `task.get`
  - Write TDD tests first: test that each method dispatches correctly and returns expected response shape

  **Must NOT do**:
  - Do NOT duplicate the secretary RPC handler logic — delegate to existing handler
  - Do NOT change the Matrix `!secretary` command handler
  - Do NOT remove or modify existing RPC methods

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-2, 4-6)
  - **Blocks**: Task 6 (BridgeApi), Tasks 11-13 (ViewModels)
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/rpc/server.go:830-901` — Handler registration table pattern to follow
  - `bridge/pkg/secretary/rpc.go:101-137` — Existing secretary RPC method dispatch (switch on method name)
  - `bridge/pkg/rpc/browser.go` — Example of how browser.* methods are wired (similar pattern)
  - `bridge/pkg/rpc/studio.go:25-36` — Example of delegated RPC handler (studio.* pattern)

  **Acceptance Criteria**:
  - [ ] `secretary.start_workflow` reachable via main RPC server
  - [ ] `secretary.get_workflow` reachable via main RPC server
  - [ ] `task.list` reachable via main RPC server
  - [ ] `go test ./pkg/rpc/...` passes with new tests
  - [ ] `go build ./...` succeeds

  **QA Scenarios**:

  ```
  Scenario: Secretary RPC methods are registered in server
    Tool: Bash
    Steps:
      1. grep -c "secretary\." bridge/pkg/rpc/server.go
    Expected Result: At least 6 secretary.* method registrations found
    Evidence: .sisyphus/evidence/task-3-secretary-rpc.txt

  Scenario: Secretary RPC tests pass
    Tool: Bash
    Steps:
      1. cd bridge && go test ./pkg/rpc/ -run Secretary -v
    Expected Result: All secretary tests pass
    Evidence: .sisyphus/evidence/task-3-secretary-test.txt
  ```

  **Commit**: YES
  - Message: `feat(rpc): wire secretary RPC methods into main server`
  - Files: bridge/pkg/rpc/server.go, bridge/pkg/rpc/secretary_handlers.go (new), bridge/pkg/rpc/secretary_handlers_test.go (new)

- [x] 4. Add missing RPC endpoints (email.list_pending, account.delete)

  **What to do**:
  - TDD: Write failing tests first for both endpoints
  - Add `email.list_pending` to `bridge/pkg/rpc/email_approval.go`: query HITL approval store for pending email approvals, return full detail list (not just count). Current `email_approval_status` only returns count.
  - Add `account.delete` to new file `bridge/pkg/rpc/account.go`: deactivate Matrix account, revoke sessions, schedule data cleanup
  - Register both methods in `bridge/pkg/rpc/server.go` handler table
  - Ensure proper auth gating (hardening must be complete for account.delete)

  **Must NOT do**:
  - Do NOT implement team or voice endpoints (no backend exists — out of scope)
  - Do NOT remove existing email_approval_status endpoint

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-3, 5-6)
  - **Blocks**: Task 6 (BridgeApi), Task 17 (AccountDeletion)
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/rpc/email_approval.go:134` — Current email_approval_status (count only)
  - `bridge/pkg/rpc/email_approval.go:40-84` — approve_email and deny_email patterns to follow
  - `bridge/pkg/rpc/hardening_handlers.go` — Example of auth-gated endpoint pattern
  - `bridge/pkg/pii/hitl_consent.go:296` — list_pending pattern for PII approvals

  **Acceptance Criteria**:
  - [ ] `email.list_pending` returns array of pending email approvals with detail
  - [ ] `account.delete` deactivates account and returns success
  - [ ] `go test ./pkg/rpc/ -run Email -v` passes
  - [ ] `go test ./pkg/rpc/ -run Account -v` passes

  **QA Scenarios**:

  ```
  Scenario: email.list_pending returns structured list
    Tool: Bash
    Steps:
      1. cd bridge && go test ./pkg/rpc/ -run TestEmailListPending -v
    Expected Result: Test passes, verifies response contains array with approval details
    Evidence: .sisyphus/evidence/task-4-email-list-pending.txt

  Scenario: account.delete requires hardening complete
    Tool: Bash
    Steps:
      1. cd bridge && go test ./pkg/rpc/ -run TestAccountDelete -v
    Expected Result: Test passes, verifies rejection before hardening, success after
    Evidence: .sisyphus/evidence/task-4-account-delete.txt
  ```

  **Commit**: YES
  - Message: `feat(rpc): add email.list_pending and account.delete endpoints`
  - Files: bridge/pkg/rpc/email_approval.go, bridge/pkg/rpc/account.go (new), bridge/pkg/rpc/email_approval_test.go (update), bridge/pkg/rpc/account_test.go (new)

- [x] 5. Fix v6 vaultClient wiring gap

  **What to do**:
  - TDD: Write failing test first — test that MCPRouter receives vaultClient and can issue blind-fill tokens
  - Modify `bridge/cmd/bridge/setup_mcp.go`: add `vaultClient` parameter to `setupMCPRouter()` signature
  - Set `VaultClient` in the `mcp.Config{}` construction (currently commented/missing)
  - Update call site in `bridge/cmd/bridge/main.go:2469` to pass `vaultClient`
  - Verify `router.go` can now call `r.vaultClient.IssueBlindFillToken()` and `r.vaultClient.ZeroizeToolSecrets()`
  - Keep V6Microkernel default as false (no change to default behavior)

  **Must NOT do**:
  - Do NOT change V6Microkernel default to true
  - Do NOT modify the vault client itself
  - Do NOT touch any code outside the wiring path

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-4, 6)
  - **Blocks**: Task 8 (real tool execution)
  - **Blocked By**: None

  **References**:
  - `bridge/cmd/bridge/setup_mcp.go:21` — Current setupMCPRouter signature (missing vaultClient)
  - `bridge/cmd/bridge/setup_mcp.go:44-50` — mcp.Config construction (missing VaultClient field)
  - `bridge/cmd/bridge/main.go:2261` — vaultClient creation
  - `bridge/cmd/bridge/main.go:2469` — setupMCPRouter call site
  - `bridge/pkg/mcp/router.go:414,436` — nil vaultClient guards that should now receive real client
  - `bridge/doc/V6_WIRING_GAPS.md` — Documents the gap being fixed

  **Acceptance Criteria**:
  - [ ] `setupMCPRouter()` accepts vaultClient parameter
  - [ ] `mcp.Config.VaultClient` is set when v6 is enabled
  - [ ] `go test ./pkg/mcp/ -run VaultClient -v` passes
  - [ ] V6Microkernel still defaults to false in config.go

  **QA Scenarios**:

  ```
  Scenario: VaultClient wiring test passes
    Tool: Bash
    Steps:
      1. cd bridge && go test ./pkg/mcp/ -run VaultClient -v
    Expected Result: Tests pass, vaultClient methods callable from MCPRouter
    Evidence: .sisyphus/evidence/task-5-vault-wiring.txt

  Scenario: V6Microkernel default unchanged
    Tool: Bash
    Steps:
      1. grep "V6Microkernel.*false" bridge/pkg/config/config.go
    Expected Result: Default is still false
    Evidence: .sisyphus/evidence/task-5-v6-default.txt
  ```

  **Commit**: YES
  - Message: `fix(v6): pass vaultClient to MCPRouter setup`
  - Files: bridge/cmd/bridge/setup_mcp.go, bridge/cmd/bridge/main.go, bridge/pkg/mcp/router_test.go

- [x] 6. Add BridgeApi methods for secretary/workflow/account RPCs

  **What to do**:
  - Read `applications/ArmorChat/app/src/main/java/app/armorclaw/api/BridgeApi.kt` — understand existing RPC call pattern
  - Add methods for secretary RPCs: `startWorkflow`, `getWorkflow`, `cancelWorkflow`, `listTemplates`
  - Add methods for task RPCs: `listTasks`, `getTask`, `cancelTask`
  - Add methods for new endpoints: `listPendingEmails`, `deleteAccount`
  - Add methods for existing endpoints that BridgeApi doesn't wrap yet: `listAgents`, `getAgent`, `listInstances`
  - Each method should follow existing BridgeApi pattern: create JSON-RPC request, send via BridgeApi.sendRpc(), parse response
  - Add corresponding data classes for request/response types
  - Write unit tests (JUnit) for each new method

  **Must NOT do**:
  - Do NOT change existing BridgeApi methods
  - Do NOT add UI code (ViewModels come in Wave 2)
  - Do NOT add Team or Voice methods (no backend)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (but needs Task 3 for secretary method names)
  - **Parallel Group**: Wave 1 (with Tasks 1-5)
  - **Blocks**: Tasks 11-13, 17, 19 (ViewModels)
  - **Blocked By**: Task 3 (secretary RPC method names)

  **References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/api/BridgeApi.kt` — Existing RPC client pattern
  - `bridge/pkg/rpc/server.go:830-901` — Full list of available RPC methods to wrap
  - `bridge/pkg/studio/rpc.go` — Studio.* method signatures (for agent CRUD)
  - `bridge/pkg/secretary/rpc.go` — Secretary.* method signatures (for workflows)

  **Acceptance Criteria**:
  - [ ] BridgeApi.kt has methods for all secretary, task, email, account, and studio agent RPCs
  - [ ] `./gradlew test` passes
  - [ ] Each method has a corresponding unit test

  **QA Scenarios**:

  ```
  Scenario: BridgeApi compiles with new methods
    Tool: Bash
    Steps:
      1. cd applications/ArmorChat && ./gradlew compileDebugKotlin
    Expected Result: BUILD SUCCESSFUL
    Evidence: .sisyphus/evidence/task-6-bridgeapi-compile.txt

  Scenario: BridgeApi unit tests pass
    Tool: Bash
    Steps:
      1. cd applications/ArmorChat && ./gradlew test
    Expected Result: All tests pass
    Evidence: .sisyphus/evidence/task-6-bridgeapi-test.txt
  ```

  **Commit**: YES
  - Message: `feat(android): add BridgeApi methods for secretary/workflow/account RPCs`
  - Files: applications/ArmorChat/app/src/main/java/app/armorclaw/api/BridgeApi.kt, applications/ArmorChat/app/src/test/java/app/armorclaw/api/BridgeApiTest.kt (new)

- [x] 7. Fix Rust sidecar compilation (74 errors)

  **What to do**:
  - Run `cargo build --manifest-path sidecar/Cargo.toml 2>&1` to get current error list
  - Categorize errors: proto mismatches, missing imports, type mismatches, dead code references
  - Fix each category systematically:
    - Proto mismatches should be resolved by Task 2 (proto sync)
    - Fix remaining import errors, add missing `use` statements
    - Fix type mismatches from stale generated code
    - Ensure all 8 RPC handler methods match the generated trait signature
  - Target: 0 compilation errors from `cargo build`

  **Must NOT do**:
  - Do NOT comment out or skip broken code — fix it properly
  - Do NOT add `allow(dead_code)` or similar suppressions
  - Do NOT remove RPC implementations — fix them

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (after Task 2)
  - **Parallel Group**: Wave 2 (with Tasks 8-13)
  - **Blocks**: Tasks 14, 15, 16, 20
  - **Blocked By**: Task 2 (proto sync)

  **References**:
  - `sidecar/src/grpc/server.rs` — Main server implementation with 8 RPC handlers
  - `sidecar/src/grpc/proto/armorclaw.sidecar.v1.rs` — Generated proto code
  - `sidecar/src/document/` — Document processing modules (may have broken imports)
  - `sidecar/README.md` — Documents "74 errors remaining"

  **Acceptance Criteria**:
  - [ ] `cargo build --manifest-path sidecar/Cargo.toml` succeeds with 0 errors
  - [ ] `cargo check --manifest-path sidecar/Cargo.toml` succeeds

  **QA Scenarios**:

  ```
  Scenario: Rust sidecar compiles cleanly
    Tool: Bash
    Steps:
      1. cargo build --manifest-path sidecar/Cargo.toml 2>&1
    Expected Result: Exit code 0, "Finished dev profile" message
    Evidence: .sisyphus/evidence/task-7-cargo-build.txt

  Scenario: No compilation errors remain
    Tool: Bash
    Steps:
      1. cargo build --manifest-path sidecar/Cargo.toml 2>&1 | grep -c "^error"
    Expected Result: 0 errors
    Evidence: .sisyphus/evidence/task-7-error-count.txt
  ```

  **Commit**: YES
  - Message: `fix(sidecar): resolve all compilation errors`
  - Files: sidecar/src/ (multiple files as needed)

- [x] 8. Implement real tool execution in MCPRouter

  **What to do**:
  - TDD: Write failing test — test that executeTool() actually executes a tool via ToolSidecar and returns real result
  - Read `bridge/pkg/mcp/router.go:486-490` — current mock implementation
  - Read `bridge/pkg/toolsidecar/toolsidecar.go` — ToolSidecar provisioner (Spawn/Stop lifecycle)
  - Implement real tool execution flow:
    1. Spawn ToolSidecar container via provisioner
    2. Send tool command to container via stdin/volume mount
    3. Wait for result (with timeout)
    4. Parse result from container output
    5. Stop and cleanup container
    6. Return parsed result
  - Handle errors: container spawn failure, timeout, parse error
  - Ensure governance checks (SkillGate, consent) still apply before execution

  **Must NOT do**:
  - Do NOT remove the SkillGate/consent checks that happen before executeTool()
  - Do NOT change the ToolSidecar container security settings (NetworkMode none, cap-drop ALL, etc.)
  - Do NOT enable v6 by default

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7, 9-13)
  - **Blocks**: Tasks 20, 21
  - **Blocked By**: Task 5 (vaultClient wiring)

  **References**:
  - `bridge/pkg/mcp/router.go:486-490` — Current stub (returns mock map)
  - `bridge/pkg/mcp/router.go:440-520` — Full executeTool context (provisioner, container mgmt)
  - `bridge/pkg/toolsidecar/toolsidecar.go` — SpawnToolSidecar/StopToolSidecar lifecycle
  - `bridge/pkg/mcp/router_test.go` — Existing tests for MCPRouter

  **Acceptance Criteria**:
  - [ ] `executeTool()` spawns a real container, runs tool, returns parsed result
  - [ ] `go test ./pkg/mcp/ -run ExecuteTool -v` passes
  - [ ] Mock result removed from router.go

  **QA Scenarios**:

  ```
  Scenario: Tool execution test passes with real Docker mock
    Tool: Bash
    Steps:
      1. cd bridge && go test ./pkg/mcp/ -run TestExecuteTool -v
    Expected Result: Test passes, tool execution produces real result (not mock)
    Evidence: .sisyphus/evidence/task-8-tool-exec.txt

  Scenario: No "mock result" in router.go
    Tool: Bash
    Steps:
      1. grep "mock result\|For now" bridge/pkg/mcp/router.go
    Expected Result: Exit code 1 (no matches)
    Evidence: .sisyphus/evidence/task-8-no-mock.txt
  ```

  **Commit**: YES
  - Message: `feat(v6): implement real tool execution in MCPRouter`
  - Files: bridge/pkg/mcp/router.go, bridge/pkg/mcp/router_test.go

- [x] 9. Wire Jetski approval gating into CDP proxy

  **What to do**:
  - TDD: Write failing test — test that CDP proxy calls ApprovalClient.RequestApproval() before forwarding sensitive operations
  - Read `jetski/internal/cdp/proxy.go` — understand forwardToEngine() flow
  - Read `jetski/internal/approval/matrix_client.go` — understand ApprovalClient interface
  - In `proxy.go` forwardToEngine(), add approval check before forwarding:
    - For `Input.insertText`: check if text matches PII patterns → request approval
    - For `Page.navigate`: request approval for new domain navigation
    - For other methods: pass through without approval
  - Wire ApprovalClient into Proxy struct (add field, set in constructor)
  - If approval denied: return error response to CDP client, do NOT forward
  - Make approval timeout configurable (default 60s)

  **Must NOT do**:
  - Do NOT gate ALL CDP methods — only sensitive ones
  - Do NOT remove existing PII scrubbing (it's a separate layer)
  - Do NOT change the ApprovalClient HTTP API

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7-8, 10-13)
  - **Blocks**: Task 21
  - **Blocked By**: None

  **References**:
  - `jetski/internal/cdp/proxy.go:160-190` — forwardToEngine() where approval check should be added
  - `jetski/internal/approval/matrix_client.go` — ApprovalClient.RequestApproval() interface
  - `jetski/internal/security/pii_scanner.go` — PII pattern matching for determining which ops need approval
  - `jetski/configs/config.yaml` — Approval config section (currently enabled: false)

  **Acceptance Criteria**:
  - [ ] CDP proxy calls ApprovalClient before forwarding Input.insertText with PII
  - [ ] CDP proxy calls ApprovalClient before forwarding Page.navigate to new domains
  - [ ] Denied approval returns error response, does NOT forward to engine
  - [ ] `go test ./internal/cdp/ -run Approval -v` passes

  **QA Scenarios**:

  ```
  Scenario: Approval gating test passes
    Tool: Bash
    Steps:
      1. cd jetski && go test ./internal/cdp/ -run Approval -v
    Expected Result: Tests pass — approval requested for sensitive ops, denied ops blocked
    Evidence: .sisyphus/evidence/task-9-approval-test.txt

  Scenario: Non-sensitive ops pass through without approval
    Tool: Bash
    Steps:
      1. cd jetski && go test ./internal/cdp/ -run Passthrough -v
    Expected Result: Tests pass — non-sensitive CDP methods forwarded immediately
    Evidence: .sisyphus/evidence/task-9-passthrough.txt
  ```

  **Commit**: YES
  - Message: `feat(jetski): wire approval gating into CDP proxy`
  - Files: jetski/internal/cdp/proxy.go, jetski/internal/cdp/proxy_test.go

- [x] 10. Enable PII scrub in Jetski default config

  **What to do**:
  - Read `jetski/configs/config.yaml` — current defaults
  - Change `security.encrypt_session` from `false` to `true` (this enables Tethered Mode which activates PII scrub)
  - Change `approval.enabled` from `false` to `true`
  - Verify PII scrub patterns in `jetski/internal/cdp/proxy.go:346-351` are comprehensive
  - Update any related config documentation

  **Must NOT do**:
  - Do NOT change PII scrub patterns without testing
  - Do NOT remove the Free-Ride mode option (it should still be configurable)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7-9, 11-13)
  - **Blocks**: Task 21
  - **Blocked By**: None

  **References**:
  - `jetski/configs/config.yaml` — Current defaults
  - `jetski/internal/cdp/proxy.go:346-351` — PII scrub patterns (SSN, CC, email, password)
  - `jetski/pkg/config/config.go` — Config struct definitions

  **Acceptance Criteria**:
  - [ ] Default config has `encrypt_session: true`
  - [ ] Default config has `approval.enabled: true`
  - [ ] `go test ./...` in jetski passes with new defaults

  **QA Scenarios**:

  ```
  Scenario: Default config enables PII scrub
    Tool: Bash
    Steps:
      1. grep "encrypt_session" jetski/configs/config.yaml
    Expected Result: Value is true
    Evidence: .sisyphus/evidence/task-10-config.txt
  ```

  **Commit**: YES
  - Message: `fix(jetski): enable PII scrub in default config`
  - Files: jetski/configs/config.yaml

- [x] 11. ArmorChat AgentManagementViewModel + AgentScreen

  **What to do**:
  - TDD: Write failing tests for ViewModel behavior first
  - Create `AgentManagementViewModel.kt` with:
    - State: agent list, selected agent, instances, loading/error states
    - Methods: loadAgents(), createAgent(), deleteAgent(), refreshInstances()
    - Uses BridgeApi.listAgents(), getAgent(), listInstances(), createAgent(), deleteAgent()
  - Create `AgentScreen.kt` with:
    - Agent list with status indicators (running/stopped)
    - Create agent dialog (name, skills selection)
    - Agent detail view (skills, instances, actions)
    - Delete confirmation dialog
  - Wire into navigation: Route.AgentManagement in NavHost (replacing placeholder if exists)
  - Add Route.AgentManagement to Route.kt
  - Follow existing patterns from BondingViewModel/BondingScreen

  **Must NOT do**:
  - Do NOT create the ViewModel without tests
  - Do NOT use any API not already in BridgeApi.kt (added by Task 6)
  - Do NOT hardcode API URLs

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7-10, 12-13)
  - **Blocks**: Task 17 (Home dashboard)
  - **Blocked By**: Task 6 (BridgeApi methods)

  **References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/viewmodel/BondingViewModel.kt` — Pattern to follow
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/security/BondingScreen.kt` — Screen pattern
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/api/BridgeApi.kt` — API methods (listAgents, createAgent, etc. from Task 6)
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/Route.kt` — Route definitions
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt` — NavHost wiring

  **Acceptance Criteria**:
  - [ ] AgentManagementViewModel.kt exists with state, methods, and tests
  - [ ] AgentScreen.kt exists with list, detail, create, delete UI
  - [ ] Route.AgentManagement defined and wired in NavHost
  - [ ] `./gradlew test` passes

  **QA Scenarios**:

  ```
  Scenario: ViewModel unit tests pass
    Tool: Bash
    Steps:
      1. cd applications/ArmorChat && ./gradlew test --tests "*AgentManagement*"
    Expected Result: All tests pass
    Evidence: .sisyphus/evidence/task-11-agent-vm-test.txt

  Scenario: App compiles with new screen
    Tool: Bash
    Steps:
      1. cd applications/ArmorChat && ./gradlew assembleDebug
    Expected Result: BUILD SUCCESSFUL
    Evidence: .sisyphus/evidence/task-11-assemble.txt
  ```

  **Commit**: YES
  - Message: `feat(android): AgentManagementViewModel + AgentScreen`
  - Files: applications/ArmorChat/app/src/main/java/app/armorclaw/viewmodel/AgentManagementViewModel.kt (new), applications/ArmorChat/app/src/main/java/app/armorclaw/ui/agent/AgentScreen.kt (new), applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/Route.kt, applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt

- [x] 12. ArmorChat HitlViewModel + ApprovalScreen

  **What to do**:
  - TDD: Write failing tests first
  - Create `HitlViewModel.kt` with:
    - State: pending approvals (PII + MCP + Email combined), loading/error
    - Methods: loadPendingApprovals(), approvePii(), denyPii(), approveMcp(), rejectMcp(), approveEmail(), denyEmail()
    - Uses BridgeApi: pii.list_pending, studio.list_pending_approvals, email.list_pending
  - Create `ApprovalScreen.kt` with:
    - Tab layout: PII | MCP | Email approvals
    - Reuse existing UI components: PiiApprovalCard, EmailApprovalCard, BlindFillCard, GovernanceBanner
    - Batch approve/deny with per-item toggles
    - Timeout countdown on pending items
  - Wire Route.EmailApproval to this screen (replacing placeholder)
  - Also add Route.PiiApproval if separate route needed

  **Must NOT do**:
  - Do NOT duplicate PiiApprovalCard or EmailApprovalCard — import from existing components
  - Do NOT create separate ViewModels for each approval type — combine into one

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7-11, 13)
  - **Blocks**: Task 17 (Home dashboard), Task 19 (EmailApproval screen)
  - **Blocked By**: Task 6 (BridgeApi methods)

  **References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/components/PiiApprovalCard.kt` — 370 lines, batched HITL with per-field toggles
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/components/EmailApprovalCard.kt` — 194 lines, email approval with timeout
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/components/BlindFillCard.kt` — 389 lines, credential injection
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/components/GovernanceBanner.kt` — 317 lines

  **Acceptance Criteria**:
  - [ ] HitlViewModel loads and displays pending PII, MCP, and email approvals
  - [ ] ApprovalScreen reuses existing card components
  - [ ] EmailApproval route shows real screen (not placeholder)
  - [ ] `./gradlew test` passes

  **QA Scenarios**:

  ```
  Scenario: HitlViewModel tests pass
    Tool: Bash
    Steps:
      1. cd applications/ArmorChat && ./gradlew test --tests "*Hitl*"
    Expected Result: All tests pass
    Evidence: .sisyphus/evidence/task-12-hitl-vm-test.txt

  Scenario: No placeholder for EmailApproval route
    Tool: Bash
    Steps:
      1. grep "EmailApproval" applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt
    Expected Result: Shows ApprovalScreen composable, not PlaceholderScreen
    Evidence: .sisyphus/evidence/task-12-email-route.txt
  ```

  **Commit**: YES
  - Message: `feat(android): HitlViewModel + ApprovalScreen`
  - Files: applications/ArmorChat/app/src/main/java/app/armorclaw/viewmodel/HitlViewModel.kt (new), applications/ArmorChat/app/src/main/java/app/armorclaw/ui/approval/ApprovalScreen.kt (new), applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/Route.kt, applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt

- [x] 13. ArmorChat WorkflowViewModel + WorkflowScreen

  **What to do**:
  - TDD: Write failing tests first
  - Create `WorkflowViewModel.kt` with:
    - State: workflow list, selected workflow, timeline events, loading/error
    - Methods: loadWorkflows(), startWorkflow(), cancelWorkflow(), resolveBlocker()
    - Uses BridgeApi: secretary.start_workflow, get_workflow, cancel_workflow, events.replay
  - Create `WorkflowScreen.kt` with:
    - Workflow list with status badges (running/completed/failed/blocked)
    - Workflow detail with timeline view (reuse WorkflowTimeline.kt — 561 lines)
    - Blocker resolution dialog (reuse BlockerResponseDialog.kt — 518 lines)
  - Add Route.Workflow to Route.kt and wire in NavHost

  **Must NOT do**:
  - Do NOT duplicate WorkflowTimeline or BlockerResponseDialog — import from existing components

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7-12)
  - **Blocks**: Task 17 (Home dashboard)
  - **Blocked By**: Task 6 (BridgeApi methods)

  **References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/components/WorkflowTimeline.kt` — 561 lines, fully implemented
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/components/BlockerResponseDialog.kt` — 518 lines

  **Acceptance Criteria**:
  - [ ] WorkflowViewModel loads workflows and displays timeline
  - [ ] WorkflowScreen reuses WorkflowTimeline component
  - [ ] Route.Workflow defined and wired
  - [ ] `./gradlew test` passes

  **QA Scenarios**:

  ```
  Scenario: WorkflowViewModel tests pass
    Tool: Bash
    Steps:
      1. cd applications/ArmorChat && ./gradlew test --tests "*Workflow*"
    Expected Result: All tests pass
    Evidence: .sisyphus/evidence/task-13-workflow-vm-test.txt
  ```

  **Commit**: YES
  - Message: `feat(android): WorkflowViewModel + WorkflowScreen`
  - Files: applications/ArmorChat/app/src/main/java/app/armorclaw/viewmodel/WorkflowViewModel.kt (new), applications/ArmorChat/app/src/main/java/app/armorclaw/ui/workflow/WorkflowScreen.kt (new), applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/Route.kt, applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt

- [x] 14. Implement sidecar real HealthCheck telemetry

  **What to do**:
  - TDD: Write failing test — test that HealthCheck returns actual uptime, active_requests, memory_used_bytes (not hardcoded zeros)
  - Read `sidecar/src/grpc/server.rs` — find HealthCheck handler
  - Track server start time (use `std::time::Instant` at server creation)
  - Track active request count via `AtomicU64` counter (increment on request start, decrement on end)
  - Read memory usage from `/proc/self/status` (VmRSS) on Linux, or use a cross-platform approach
  - Update HealthCheck response with real values
  - Return actual server version from Cargo.toml version string

  **Must NOT do**:
  - Do NOT use external crates for memory tracking — keep it simple
  - Do NOT block the HealthCheck RPC on expensive operations

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 15-19)
  - **Blocks**: Task 20
  - **Blocked By**: Task 7 (sidecar compiles)

  **References**:
  - `sidecar/src/grpc/server.rs` — HealthCheck handler (currently hardcoded zeros)
  - `sidecar/Cargo.toml` — Version string

  **Acceptance Criteria**:
  - [ ] HealthCheck returns real uptime > 0
  - [ ] HealthCheck returns active_requests matching concurrent requests
  - [ ] HealthCheck returns memory_used_bytes > 0
  - [ ] `cargo test --manifest-path sidecar/Cargo.toml` passes

  **QA Scenarios**:

  ```
  Scenario: HealthCheck test passes with real telemetry
    Tool: Bash
    Steps:
      1. cargo test --manifest-path sidecar/Cargo.toml -- health_check
    Expected Result: Test passes, uptime and memory > 0
    Evidence: .sisyphus/evidence/task-14-healthcheck.txt
  ```

  **Commit**: YES
  - Message: `feat(sidecar): implement real HealthCheck telemetry`
  - Files: sidecar/src/grpc/server.rs

- [x] 15. Implement sidecar ProcessDocument convert operation

  **What to do**:
  - TDD: Write failing test — test convert DOCX→PDF produces valid output
  - Read `sidecar/src/grpc/server.rs` — find ProcessDocument handler's convert branch (currently passthrough stub)
  - Implement document conversion:
    - DOCX → PDF: use a Rust PDF generation approach (printpdf crate or similar)
    - XLSX → CSV: use calamine (already a dependency) to dump sheets as CSV
    - PPTX → PDF: placeholder with clear "not yet supported" error
  - Write converted output to temp file, return as response bytes
  - Clean up temp files after response

  **Must NOT do**:
  - Do NOT call external CLI tools (like libreoffice) — keep it pure Rust
  - Do NOT implement formats beyond DOCX→PDF and XLSX→CSV for now

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 14, 16-19)
  - **Blocks**: Task 20
  - **Blocked By**: Task 7 (sidecar compiles)

  **References**:
  - `sidecar/src/grpc/server.rs` — ProcessDocument convert stub
  - `sidecar/src/document/xlsx.rs` — Calamine-based XLSX reader (pattern reference)
  - `sidecar/Cargo.toml` — Dependencies (check for PDF crate or add one)

  **Acceptance Criteria**:
  - [ ] ProcessDocument convert for DOCX returns non-trivial output
  - [ ] ProcessDocument convert for XLSX returns CSV output
  - [ ] `cargo test --manifest-path sidecar/Cargo.toml` passes

  **QA Scenarios**:

  ```
  Scenario: Convert test passes
    Tool: Bash
    Steps:
      1. cargo test --manifest-path sidecar/Cargo.toml -- convert
    Expected Result: Tests pass for DOCX→PDF and XLSX→CSV
    Evidence: .sisyphus/evidence/task-15-convert.txt
  ```

  **Commit**: YES
  - Message: `feat(sidecar): implement ProcessDocument convert operation`
  - Files: sidecar/src/grpc/server.rs, sidecar/src/document/convert.rs (new)

- [x] 16. Wire sidecar ONNX OCR fallback

  **What to do**:
  - TDD: Write failing test — test that OCR falls back to ONNX when Tesseract fails
  - Read `sidecar/src/document/ocr.rs` — understand OcrExtractor and OnnxBackend
  - Modify `OcrExtractor::extract()` to:
    1. Try Tesseract first (existing behavior)
    2. If Tesseract fails (non-zero exit, no text), try OnnxBackend
    3. If both fail, return error
  - The ONNX backend struct already exists with `extract()` method — just wire it into the fallback path
  - Add config option for enabling/disabling ONNX fallback (default: enabled)

  **Must NOT do**:
  - Do NOT remove Tesseract as primary path
  - Do NOT add new ONNX model files — use existing model path configuration

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 14-15, 17-19)
  - **Blocks**: Task 20
  - **Blocked By**: Task 7 (sidecar compiles)

  **References**:
  - `sidecar/src/document/ocr.rs:575` — Full OCR implementation with both backends
  - `sidecar/src/document/ocr.rs` OnnxBackend — Already implemented but never called from extract()

  **Acceptance Criteria**:
  - [ ] OcrExtractor falls back to ONNX when Tesseract fails
  - [ ] `cargo test --manifest-path sidecar/Cargo.toml -- ocr` passes

  **QA Scenarios**:

  ```
  Scenario: ONNX fallback test passes
    Tool: Bash
    Steps:
      1. cargo test --manifest-path sidecar/Cargo.toml -- ocr::fallback
    Expected Result: Test passes — ONNX backend called when Tesseract returns empty
    Evidence: .sisyphus/evidence/task-16-onnx-fallback.txt
  ```

  **Commit**: YES
  - Message: `feat(sidecar): wire ONNX OCR fallback into extract path`
  - Files: sidecar/src/document/ocr.rs

- [x] 17. ArmorChat Home dashboard + AccountDeletion screen

  **What to do**:
  - TDD: Write failing tests first
  - Create `HomeScreen.kt`:
    - Dashboard layout with cards for: active agents count, pending approvals count, running workflows count
    - Navigation to AgentScreen, ApprovalScreen, WorkflowScreen
    - Quick actions: approve all, cancel workflow
    - Replace Route.Home placeholder in NavHost
  - Create `AccountDeletionScreen.kt`:
    - Confirmation dialog with "type DELETE to confirm" safety
    - Calls BridgeApi.deleteAccount()
    - Shows hardening status prerequisite
    - On success: navigate to BondingScreen (restart setup)
  - Add Route.AccountDeletion to Route.kt

  **Must NOT do**:
  - Do NOT make Home screen a simple placeholder — it's the main post-onboarding screen
  - Do NOT skip the confirmation safety on account deletion

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (after ViewModels from Wave 2)
  - **Parallel Group**: Wave 3 (with Tasks 14-16, 18-19)
  - **Blocks**: Nothing (terminal ArmorChat task)
  - **Blocked By**: Tasks 11, 12, 13 (ViewModels provide data for dashboard cards)

  **References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt:152` — Current Home placeholder
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/viewmodel/BondingViewModel.kt` — Pattern for ViewModel
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/api/BridgeApi.kt` — deleteAccount(), listAgents(), etc.

  **Acceptance Criteria**:
  - [ ] Route.Home shows HomeScreen with agent/approval/workflow cards
  - [ ] AccountDeletionScreen requires confirmation and calls API
  - [ ] `./gradlew test` passes
  - [ ] `./gradlew assembleDebug` succeeds

  **QA Scenarios**:

  ```
  Scenario: Home screen is not placeholder
    Tool: Bash
    Steps:
      1. grep "Route.Home" applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt
    Expected Result: Shows HomeScreen composable, not PlaceholderScreen
    Evidence: .sisyphus/evidence/task-17-home-route.txt

  Scenario: App builds with all new screens
    Tool: Bash
    Steps:
      1. cd applications/ArmorChat && ./gradlew assembleDebug
    Expected Result: BUILD SUCCESSFUL
    Evidence: .sisyphus/evidence/task-17-assemble.txt
  ```

  **Commit**: YES
  - Message: `feat(android): Home dashboard + AccountDeletion screen`
  - Files: applications/ArmorChat/app/src/main/java/app/armorclaw/ui/home/HomeScreen.kt (new), applications/ArmorChat/app/src/main/java/app/armorclaw/ui/account/AccountDeletionScreen.kt (new), applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/Route.kt, applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt

- [x] 18. Wire ArmorChat orphaned screens into navigation

  **What to do**:
  - Wire KeyRecoveryScreen.kt (213 lines) into Route.KeyRecovery — replace PlaceholderScreen at NavHost line 160
  - Wire SecretsScreen.kt (420 lines) — add Route.Secrets to Route.kt, add NavHost entry
  - Wire MigrationScreen.kt (411 lines) — add Route.Migration to Route.kt, add NavHost entry
  - Ensure each screen gets necessary parameters (roomId, etc.) from nav arguments
  - Add navigation triggers: KeyBackup → KeyRecovery, SecurityConfig → Secrets, hardening complete → Migration

  **Must NOT do**:
  - Do NOT modify the screen composables themselves — they're already implemented
  - Do NOT add new screens — just wire existing ones

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 14-17, 19)
  - **Blocks**: Nothing
  - **Blocked By**: None

  **References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/security/KeyRecoveryScreen.kt` — 213 lines, real screen
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/security/SecretsScreen.kt` — 420 lines, real screen
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/migration/MigrationScreen.kt` — 411 lines, real screen
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt:160` — KeyRecovery placeholder

  **Acceptance Criteria**:
  - [ ] Route.KeyRecovery shows KeyRecoveryScreen (not placeholder)
  - [ ] Route.Secrets exists with SecretsScreen
  - [ ] Route.Migration exists with MigrationScreen
  - [ ] `./gradlew assembleDebug` succeeds

  **QA Scenarios**:

  ```
  Scenario: No placeholder routes remain for KeyRecovery
    Tool: Bash
    Steps:
      1. grep "KeyRecovery" applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt
    Expected Result: Shows KeyRecoveryScreen, not PlaceholderScreen
    Evidence: .sisyphus/evidence/task-18-keyrecovery-route.txt
  ```

  **Commit**: YES
  - Message: `feat(android): wire orphaned screens into navigation`
  - Files: applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/Route.kt, applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt

- [x] 19. ArmorChat EmailApproval real screen

  **What to do**:
  - Create dedicated `EmailApprovalScreen.kt` for the email/approve/{approvalId} deep link route
  - Display email details (sender, subject, body preview) from approval data
  - Show approve/deny buttons with timeout countdown
  - Reuse EmailApprovalCard component
  - Wire HitlViewModel for this screen or create lightweight EmailApprovalViewModel
  - Replace PlaceholderScreen at NavHost line 182

  **Must NOT do**:
  - Do NOT duplicate EmailApprovalCard — import from components

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 14-18)
  - **Blocks**: Nothing
  - **Blocked By**: Task 12 (HitlViewModel)

  **References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/components/EmailApprovalCard.kt` — 194 lines
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt:182` — Current placeholder

  **Acceptance Criteria**:
  - [ ] email/approve/{approvalId} shows real screen (not placeholder)
  - [ ] `./gradlew assembleDebug` succeeds

  **QA Scenarios**:

  ```
  Scenario: Email approval route is real screen
    Tool: Bash
    Steps:
      1. grep "email/approve" applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt
    Expected Result: Shows EmailApprovalScreen, not PlaceholderScreen
    Evidence: .sisyphus/evidence/task-19-email-route.txt
  ```

  **Commit**: YES
  - Message: `feat(android): real EmailApproval screen`
  - Files: applications/ArmorChat/app/src/main/java/app/armorclaw/ui/email/EmailApprovalScreen.kt (new), applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt

- [x] 20. Sidecar E2E integration test

  **What to do**:
  - Create `sidecar/tests/e2e_integration_test.rs` (or add to existing test structure)
  - Test each RPC end-to-end:
    - HealthCheck: verify real telemetry values
    - UploadBlob + DownloadBlob: round-trip data integrity
    - ListBlobs: verify uploaded blob appears
    - ExtractText: PDF, DOCX, XLSX, image OCR
    - ProcessDocument: extract_text and convert operations
    - QueryDocuments (if proto was synced with it)
  - Use a test gRPC client connecting to a sidecar test server
  - Test data: small sample files checked into `sidecar/tests/testdata/`

  **Must NOT do**:
  - Do NOT test against external services (S3, Qdrant) — mock them
  - Do NOT skip RPCs — test all 8

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 21-22)
  - **Blocks**: F1-F4
  - **Blocked By**: Tasks 7, 14, 15, 16

  **References**:
  - `sidecar/src/grpc/server.rs` — All RPC handlers
  - `jetski/tests/e2e_tethered_test.go` — Pattern for E2E test structure

  **Acceptance Criteria**:
  - [ ] `cargo test --manifest-path sidecar/Cargo.toml -- e2e` passes
  - [ ] All 8 RPCs tested

  **QA Scenarios**:

  ```
  Scenario: Sidecar E2E tests pass
    Tool: Bash
    Steps:
      1. cargo test --manifest-path sidecar/Cargo.toml -- e2e 2>&1
    Expected Result: All E2E tests pass
    Evidence: .sisyphus/evidence/task-20-sidecar-e2e.txt
  ```

  **Commit**: YES
  - Message: `test(sidecar): E2E integration test`
  - Files: sidecar/tests/e2e_integration_test.rs (new), sidecar/tests/testdata/ (new directory)

- [x] 21. Browser path full E2E test

  **What to do**:
  - Create `bridge/pkg/browser/e2e_fullpath_test.go`
  - Test the complete path: Bridge handler → HTTP client → mock browser-service → result parsing
  - Test scenarios:
    - Navigate: handler dispatches correct HTTP request, parses response
    - Fill with PII: handler requests HITL approval, waits for approval, then fills
    - Fill without PII: handler fills directly
    - Extract: handler parses extracted data
    - Screenshot: handler returns image data
  - Mock browser-service using httptest.Server
  - Mock HITL approval using in-memory channel

  **Must NOT do**:
  - Do NOT test against real browser-service (use mock)
  - Do NOT test Jetski CDP proxy path (separate concern)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 20, 22)
  - **Blocks**: F1-F4
  - **Blocked By**: Tasks 8, 9, 10

  **References**:
  - `bridge/pkg/browser/handler.go` — Browser action handler
  - `bridge/pkg/browser/client.go` — HTTP client to browser-service
  - `bridge/pkg/secretary/browser_integration_test.go` — Existing integration test pattern

  **Acceptance Criteria**:
  - [ ] `go test ./pkg/browser/ -run E2E -v` passes
  - [ ] Tests cover navigate, fill (with/without PII), extract, screenshot

  **QA Scenarios**:

  ```
  Scenario: Browser E2E tests pass
    Tool: Bash
    Steps:
      1. cd bridge && go test ./pkg/browser/ -run E2E -v
    Expected Result: All E2E tests pass
    Evidence: .sisyphus/evidence/task-21-browser-e2e.txt
  ```

  **Commit**: YES
  - Message: `test(browser): full-path E2E test`
  - Files: bridge/pkg/browser/e2e_fullpath_test.go (new)

- [x] 22. Fix all doc discrepancies from verification

  **What to do**:
  - Fix `doc/sidecar-pipeline.md`: Change 3 XLSX routing claims (lines 324, 358, 452) — XLSX routes to Rust, not Python
  - Fix `doc/sidecar-pipeline.md`: Update proto claim from "mirrors" to "synced" or document drift status
  - Fix `doc/sidecar-pipeline.md`: Note ONNX fallback is now wired (after Task 16)
  - Fix `doc/sidecar-pipeline.md`: Note HealthCheck has real telemetry (after Task 14)
  - Fix `doc/sidecar-pipeline.md`: Note ProcessDocument convert is implemented (after Task 15)
  - Update `bridge/pkg/config/config.go` godoc for V6Microkernel: note wiring gap is fixed, tool execution is real, still gated
  - Update `bridge/doc/V6_WIRING_GAPS.md`: mark Gap 5 as RESOLVED (vaultClient now passed)
  - Update `bridge/config.example.toml`: update v6 comment to reflect fixed wiring
  - Fix browser path docs: document both paths (browser-service HTTP primary, Jetski CDP secondary)
  - Update ArmorChat capabilities docs: reflect 4 new ViewModels, Home dashboard, 0 placeholders
  - Fix `doc/sidecar-pipeline.md`: Note QueryDocuments status (whether added to Go or removed from Rust)

  **Must NOT do**:
  - Do NOT add claims about features that aren't implemented
  - Do NOT remove historical CHANGELOG entries
  - Do NOT create new documentation files — update existing ones

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 20-21)
  - **Blocks**: F1-F4
  - **Blocked By**: Tasks 1-21 (docs should reflect final state)

  **References**:
  - `doc/sidecar-pipeline.md` — 3 XLSX routing errors, proto drift claim, OCR/HealthCheck/convert stubs
  - `bridge/pkg/config/config.go:769-771` — V6Microkernel godoc
  - `bridge/doc/V6_WIRING_GAPS.md` — Gap 5 (vaultClient wiring)
  - `bridge/config.example.toml:355-359` — v6 config comments

  **Acceptance Criteria**:
  - [ ] `grep "routes to Python\|-> Python" doc/sidecar-pipeline.md` returns 0 matches
  - [ ] `grep "mirrors" doc/sidecar-pipeline.md` returns 0 matches for proto claim
  - [ ] All doc sections reflect implemented state

  **QA Scenarios**:

  ```
  Scenario: No XLSX-to-Python routing claims
    Tool: Bash
    Steps:
      1. grep -c "Python.*xlsx\|xlsx.*Python" doc/sidecar-pipeline.md
    Expected Result: 0 matches
    Evidence: .sisyphus/evidence/task-22-doc-fix.txt

  Scenario: v6 wiring gap marked resolved
    Tool: Bash
    Steps:
      1. grep "RESOLVED" bridge/doc/V6_WIRING_GAPS.md
    Expected Result: At least 1 match for Gap 5
    Evidence: .sisyphus/evidence/task-22-v6-doc.txt
  ```

  **Commit**: YES
  - Message: `docs: fix all verification discrepancies`
  - Files: doc/sidecar-pipeline.md, bridge/pkg/config/config.go, bridge/doc/V6_WIRING_GAPS.md, bridge/config.example.toml, doc/armorclaw.md (ArmorChat section updates)

---

## Final Verification Wave

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go build ./...` + `go test ./...` in bridge/. Run `cargo build` + `cargo test` in sidecar/. Run `./gradlew test` in ArmorChat. Review all changed files for: `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop.
  Output: `Build [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration. Test edge cases: empty state, invalid input, rapid actions. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff. Verify 1:1. Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **1**: `chore(cleanup): remove warm dispatch dead code` - bridge/pkg/secretary/task_dispatch.go, bridge/pkg/docker/client.go, bridge/pkg/studio/factory.go, bridge/pkg/studio/factory_test.go, bridge/pkg/secretary/task_scheduler.go, bridge/pkg/secretary/task_scheduler_test.go
- **2**: `fix(sidecar): sync proto files between Rust and Go` - sidecar/src/grpc/proto/sidecar.proto, bridge/pkg/sidecar/sidecar.proto
- **3**: `feat(rpc): wire secretary RPC methods into main server` - bridge/pkg/rpc/server.go
- **4**: `feat(rpc): add email.list_pending and account.delete endpoints` - bridge/pkg/rpc/email_approval.go, bridge/pkg/rpc/account.go (new)
- **5**: `fix(v6): pass vaultClient to MCPRouter setup` - bridge/cmd/bridge/setup_mcp.go, bridge/cmd/bridge/main.go
- **6**: `feat(android): add BridgeApi methods for secretary/workflow/account RPCs` - applications/ArmorChat/app/src/main/java/app/armorclaw/api/BridgeApi.kt
- **7**: `fix(sidecar): resolve all compilation errors` - sidecar/src/ (multiple files)
- **8**: `feat(v6): implement real tool execution in MCPRouter` - bridge/pkg/mcp/router.go
- **9**: `feat(jetski): wire approval gating into CDP proxy` - jetski/internal/cdp/proxy.go
- **10**: `fix(jetski): enable PII scrub in default config` - jetski/configs/config.yaml
- **11**: `feat(android): AgentManagementViewModel + AgentScreen` - applications/ArmorChat/app/src/main/java/app/armorclaw/viewmodel/AgentManagementViewModel.kt (new), applications/ArmorChat/app/src/main/java/app/armorclaw/ui/agent/AgentScreen.kt (new)
- **12**: `feat(android): HitlViewModel + ApprovalScreen` - applications/ArmorChat/app/src/main/java/app/armorclaw/viewmodel/HitlViewModel.kt (new), applications/ArmorChat/app/src/main/java/app/armorclaw/ui/approval/ApprovalScreen.kt (new)
- **13**: `feat(android): WorkflowViewModel + WorkflowScreen` - applications/ArmorChat/app/src/main/java/app/armorclaw/viewmodel/WorkflowViewModel.kt (new), applications/ArmorChat/app/src/main/java/app/armorclaw/ui/workflow/WorkflowScreen.kt (new)
- **14**: `feat(sidecar): implement real HealthCheck telemetry` - sidecar/src/grpc/server.rs
- **15**: `feat(sidecar): implement ProcessDocument convert operation` - sidecar/src/grpc/server.rs, sidecar/src/document/convert.rs (new)
- **16**: `feat(sidecar): wire ONNX OCR fallback into extract path` - sidecar/src/document/ocr.rs
- **17**: `feat(android): Home dashboard + AccountDeletion screen` - applications/ArmorChat/app/src/main/java/app/armorclaw/ui/home/HomeScreen.kt (new), applications/ArmorChat/app/src/main/java/app/armorclaw/ui/account/AccountDeletionScreen.kt (new)
- **18**: `feat(android): wire orphaned screens into navigation` - applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt, applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/Route.kt
- **19**: `feat(android): real EmailApproval screen` - applications/ArmorChat/app/src/main/java/app/armorclaw/ui/email/EmailApprovalScreen.kt (new)
- **20**: `test(sidecar): E2E integration test` - sidecar/tests/ (new)
- **21**: `test(browser): full-path E2E test` - bridge/pkg/browser/e2e_test.go (new)
- **22**: `docs: fix all verification discrepancies` - doc/sidecar-pipeline.md, doc/armorclaw.md, bridge/pkg/config/config.go, bridge/doc/V6_WIRING_GAPS.md

---

## Success Criteria

### Verification Commands
```bash
cargo build --manifest-path sidecar/Cargo.toml           # Expected: 0 errors
cargo test --manifest-path sidecar/Cargo.toml              # Expected: all pass
cd bridge && go build ./...                                # Expected: succeeds
cd bridge && go test ./...                                 # Expected: all pass
cd applications/ArmorChat && ./gradlew assembleDebug       # Expected: BUILD SUCCESSFUL
cd applications/ArmorChat && ./gradlew test                # Expected: all pass
grep -r "PrewarmPool\|GetRunningInstance" bridge/          # Expected: 0 matches
grep -r "placeholder" applications/ArmorChat/              # Expected: 0 matches in NavHost
grep "routes to Python" doc/sidecar-pipeline.md            # Expected: 0 matches
```

### Final Checklist
- [ ] Rust sidecar binary compiles with 0 errors
- [ ] All 8 sidecar RPCs return real results (not stubs)
- [ ] v6 vaultClient wiring gap closed
- [ ] v6 executeTool() performs real tool execution
- [ ] V6Microkernel still defaults to false
- [ ] ArmorChat has 0 placeholder routes
- [ ] ArmorChat has 4 new ViewModels (AgentManagement, Hitl, Workflow, Hitl)
- [ ] ArmorChat has Home dashboard with agent/approval/workflow cards
- [ ] Jetski CDP proxy gates approvals
- [ ] Jetski default config has PII scrub enabled
- [ ] Zero warm dispatch dead code
- [ ] All doc files internally consistent
