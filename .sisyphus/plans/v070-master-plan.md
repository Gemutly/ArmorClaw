# v0.7.0 Master Plan — ArmorClaw Codebase Refinement

## TL;DR

> **Quick Summary**: Close validated integration gaps across Bridge runtime and ArmorChat Android. Deprecate architecturally illegal warm dispatch, wire EventBus to existing WebSocket infrastructure, implement deep link handling that makes HITL notifications functional, fix SecurityConfig persistence bug, and add inter-step workflow data passing.
>
> **Deliverables**:
> - Warm dispatch dead code removed, orchestrator enforces cold-only dispatch
> - WebSocket EventBus wired through existing HTTP `/ws` server
> - DeepLinkHandler.kt + cold-start intent handling in ArmorChat
> - SecurityConfigViewModel wired to SecurityConfigScreen (permissions actually saved)
> - Inter-step data propagation in Secretary workflow
> - Admin panel connected to real Bridge API (mock data removed)
> - ServerConfig persisted via DataStore in ArmorChat
>
> **Estimated Effort**: Large (7-8 weeks)
> **Parallel Execution**: YES — 3 waves per phase, 5-7 tasks per wave
> **Critical Path**: Task 1 (warm dispatch deprecation) → Task 3 (WebSocket wiring) → Task 8 (inter-step data) → Task 15 (E2E tests)

---

## Context

### Original Request
Corrected v0.7.0 plan that removes already-completed tasks from a prior draft, injects codebase-validated real gaps, and applies CTO directives: crash-only WebSocket tolerance, formal warm dispatch deprecation, elevated DeepLinkHandler priority.

### Interview Summary
**Key Discussions**:
- 6 of 12 claimed "gaps" in the original draft were already implemented (config persistence, email HITL, voice UI, admin panel scaffold, ArmorTerminal, OpenClaw UI)
- Warm dispatch is architecturally illegal under NetworkMode: none — deprecation, not implementation
- WebSocket `log.Fatalf` in `eventbus.go:146` is intentional crash-only design per CTO mandate
- DeepLinkHandler missing neutered entire HITL notification flow — elevated to top priority
- SecurityConfigViewModel is defined but never wired to SecurityConfigScreen — functional bug

**Research Findings**:
- `bridge/pkg/websocket/websocket.go` (74 lines) is a stub; `bridge/pkg/http/server.go:673-819` has working gorilla/websocket
- `ContainerStepResult.Data` (result.go:50) exists but `orchestrator_integration.go` discards it
- All 12 ArmorChat integration tests are `assertTrue(true)` placeholders
- `WorkflowStep` (types.go:64) has no `Input` field — adding one is a breaking schema change
- `BridgeRepository` credentials (homeserverUrl, accessToken, deviceId) are in-memory only
- `review.md` mandated by AGENTS.md does not exist

### Metis Review
**Identified Gaps** (addressed):
- DeepLinkHandler is 5 sub-tasks, not 1: promote to sub-phase with explicit decomposition
- Cold-start notification taps need `LaunchedEffect` checking `getIntent()`, not just `onNewIntent()`
- Inter-step data passing is a breaking schema change to WorkflowStep — migration path needed
- SecurityConfigViewModel wiring is a functional bug — promoted from Phase 2 to Phase 1
- EventBus `sendToSubscriber()` goroutine discards data (`_ = data` at eventbus.go:437) — cleanup needed with WebSocket wiring
- `open_invites` notification extra also broken — explicitly excluded from v0.7.0 scope

---

## Work Objectives

### Core Objective
Close all validated integration gaps between Bridge runtime and ArmorChat Android, achieving production readiness across deployment, workflow, and client layers without compromising zero-trust isolation.

### Concrete Deliverables
- Warm dispatch code removed from `bridge/pkg/secretary/` cascade
- EventBus publishes through HTTP server's existing `/ws` Broadcast methods
- `DeepLinkHandler.kt` created; `MainActivity.kt` handles cold-start and warm-resume intents
- `SecurityConfigScreen.kt` wired to `SecurityConfigViewModel` (permissions saved via RPC)
- `WorkflowStep.Input` field with migration guide for template JSON
- Admin panel `bridgeApi.ts` returns real data (mock fallbacks removed)
- ArmorChat `ConfigManager.kt` persists `ServerConfig` to EncryptedSharedPreferences

### Definition of Done
- [ ] `grep -r "warmDispatch" bridge/pkg/secretary/ --include="*.go"` returns empty
- [ ] `grep -n "not yet implemented" bridge/pkg/websocket/websocket.go` returns empty
- [ ] `test -f applications/ArmorChat/.../navigation/DeepLinkHandler.kt` succeeds
- [ ] `grep "viewModel\|SecurityConfigViewModel" .../SecurityConfigScreen.kt` returns match
- [ ] `grep "Input\|StepData" bridge/pkg/secretary/types.go` returns match
- [ ] All Phase 1 `go test` commands pass

### Must Have
- Cold-only dispatch enforcement (warm dispatch completely removed)
- Crash-only WebSocket tolerance preserved (no graceful fallbacks)
- Deep links functional for both cold-start and warm-resume notification taps
- SecurityConfig permissions actually persisted to Bridge
- Inter-step data passing works for sequential workflows
- All existing tests continue to pass after schema changes

### Must NOT Have (Guardrails)
- Do NOT add a settings screen to ArmorChat
- Do NOT persist auth tokens (EncryptedSharedPreferences needs separate security review — v0.8.0)
- Do NOT build general-purpose event replay or pub/sub system
- Do NOT build data transformation pipelines or conditional branching for workflows
- Do NOT unify all deep link types into one handler (SignedConfigParser handles config links)
- Do NOT touch the HTTP server's working `/ws` endpoint — wire INTO it
- Do NOT remove the `log.Fatalf` crash-only pattern without CTO approval
- Do NOT compromise NetworkMode: none for any dispatch optimization
- Do NOT add `WorkflowStep.Input` without updating template JSON schema and documenting migration
- Do NOT handle `open_invites` deep links in v0.7.0 (explicitly excluded)

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES
- **Automated tests**: Tests-after (existing Go tests, new Android tests alongside implementation)
- **Framework**: Go `testing` package (Bridge), JUnit (Android), Vitest (Admin Panel)

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Bridge Go**: `go test ./bridge/pkg/... -v` — unit + integration tests
- **Android**: `./gradlew test` — JUnit unit tests for handlers/viewmodels
- **Admin Panel**: `npm test` — Vitest for API wiring verification

---

## Execution Strategy

### Parallel Execution Waves

```
Phase 1: Critical Fixes (Weeks 1-2)

Wave 1 (Start Immediately — independent deprecation + Android + Bridge):
├── Task 1: Deprecate warm dispatch [quick]
├── Task 2: Implement DeepLinkHandler.kt [deep]
├── Task 3: Wire SecurityConfigViewModel to Screen [unspecified-high]
└── Task 4: Create review.md [quick]

Wave 2 (After Wave 1 — WebSocket + cold-start):
├── Task 5: Wire EventBus through HTTP /ws server [deep]
├── Task 6: Add cold-start intent handling to MainActivity [unspecified-high]
├── Task 7: Update AndroidManifest intent-filters [quick]
└── Task 8: Clean up EventBus sendToSubscriber dead goroutine [unspecified-high]

Wave 3 (After Wave 2 — integration verification):
├── Task 9: E2E test: WebSocket event delivery [unspecified-high]
├── Task 10: E2E test: Deep link cold-start + warm-resume [unspecified-high]
└── Task 11: Replace ArmorChat placeholder integration tests [unspecified-high]

Phase 2: Integration Polish (Weeks 3-5)

Wave 4 (After Phase 1 — independent):
├── Task 12: Add WorkflowStep.Input field + migration guide [deep]
├── Task 13: Propagate ContainerStepResult.Data in orchestrator [unspecified-high]
├── Task 14: Persist ServerConfig via EncryptedSharedPreferences [unspecified-high]
└── Task 15: Wire admin panel to real Bridge API [unspecified-high]

Wave 5 (After Wave 4 — downstream):
├── Task 16: Handle parallel step data merge [unspecified-high]
├── Task 17: ServerConfig expiration check + fallback UI [quick]
└── Task 18: Template JSON migration tool [unspecified-high]

Phase 3: Release Hardening (Weeks 6-8)

Wave 6 (After Phase 2 — validation):
├── Task 19: Full-stack E2E test suite [deep]
├── Task 20: Security audit (YARA, PII, isolation) [unspecified-high]
├── Task 21: Performance benchmarks [unspecified-high]
└── Task 22: Documentation sync [writing]

Wave FINAL:
├── Task F1: Plan compliance audit [oracle]
├── Task F2: Code quality review [unspecified-high]
├── Task F3: Real manual QA [unspecified-high]
└── Task F4: Scope fidelity check [deep]

Critical Path: Task 1 → Task 5 → Task 9 → Task 12 → Task 13 → Task 19 → F1-F4
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 4 (Waves 1, 2, 4)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | — | 5 | 1 |
| 2 | — | 6, 7, 10 | 1 |
| 3 | — | 11 | 1 |
| 4 | — | — | 1 |
| 5 | 1 | 9 | 2 |
| 6 | 2 | 10 | 2 |
| 7 | 2 | 10 | 2 |
| 8 | 5 | 9 | 2 |
| 9 | 5, 8 | 19 | 3 |
| 10 | 2, 6, 7 | 19 | 3 |
| 11 | 3 | 19 | 3 |
| 12 | — | 13, 16, 18 | 4 |
| 13 | 12 | 16, 19 | 4 |
| 14 | — | 17 | 4 |
| 15 | — | — | 4 |
| 16 | 13 | 19 | 5 |
| 17 | 14 | — | 5 |
| 18 | 12 | — | 5 |
| 19 | 9, 10, 11, 13 | F1-F4 | 6 |
| 20 | — | F1-F4 | 6 |
| 21 | — | F1-F4 | 6 |
| 22 | 19 | F1-F4 | 6 |

### Agent Dispatch Summary

- **Wave 1**: 4 tasks — T1 → `quick`, T2 → `deep`, T3 → `unspecified-high`, T4 → `quick`
- **Wave 2**: 4 tasks — T5 → `deep`, T6 → `unspecified-high`, T7 → `quick`, T8 → `unspecified-high`
- **Wave 3**: 3 tasks — T9, T10, T11 → `unspecified-high`
- **Wave 4**: 4 tasks — T12 → `deep`, T13, T14, T15 → `unspecified-high`
- **Wave 5**: 3 tasks — T16, T18 → `unspecified-high`, T17 → `quick`
- **Wave 6**: 4 tasks — T19 → `deep`, T20, T21 → `unspecified-high`, T22 → `writing`
- **FINAL**: 4 tasks — F1 → `oracle`, F2, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

### Phase 1: Critical Fixes (Weeks 1-2)

- [x] 1. Deprecate Warm Dispatch — Enforce Cold-Only Container Dispatch

  **What to do**:
  - Remove `warmDispatch()` method from `TaskScheduler` (task_scheduler.go:231-233)
  - Remove `GetRunningInstance()` call at task_scheduler.go:159 and the method from `FactoryInterface` (task_scheduler.go:23)
  - Remove `EventTypeTaskDispatch` constant and `BuildTaskDispatchPayload()` from task_dispatch.go
  - Remove or rename task_dispatch_test.go (orphaned test file)
  - Update `studioFactoryAdapter` in main.go:3613 to remove `GetRunningInstance` implementation
  - Remove warm dispatch WARN log at task_scheduler.go:166-171 (the "skipped" message)
  - Update `dispatchTask()` to go directly to `coldDispatch()` without warm check
  - Update CHANGELOG.md to document deprecation with rationale (NetworkMode: none is non-negotiable)
  - Update doc/secretary-workflow.md to remove any warm dispatch references

  **Must NOT do**:
  - Do NOT change NetworkMode for any container
  - Do NOT add IPC channels or bind-mounted event files
  - Do NOT touch cold dispatch path

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3, 4)
  - **Blocks**: Task 5 (EventBus cleanup depends on dispatch path being clear)
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - `bridge/pkg/secretary/task_scheduler.go:140-171` — dispatchTask() with warm/cold branching
  - `bridge/pkg/secretary/task_scheduler.go:231-233` — warmDispatch() one-liner to remove
  - `bridge/pkg/secretary/task_scheduler.go:23` — FactoryInterface with GetRunningInstance

  **API/Type References**:
  - `bridge/cmd/main.go:3613` — studioFactoryAdapter implementing the interface

  **Acceptance Criteria**:
  - [ ] `grep -r "warmDispatch\|warm_dispatch" bridge/pkg/secretary/ --include="*.go"` returns empty
  - [ ] `grep -r "EventTypeTaskDispatch\|BuildTaskDispatchPayload" bridge/pkg/secretary/ --include="*.go"` returns empty
  - [ ] `go test ./bridge/pkg/secretary/... -count=1` → PASS

  **QA Scenarios**:
  ```
  Scenario: Warm dispatch code fully removed
    Tool: Bash
    Preconditions: Branch clean, all changes committed
    Steps:
      1. Run: grep -r "warmDispatch\|EventTypeTaskDispatch" bridge/pkg/secretary/ --include="*.go" | grep -v "_test.go"
      2. Assert: empty output (zero matches)
    Expected Result: No warm dispatch references in production code
    Evidence: .sisyphus/evidence/task-1-warm-removal.txt

  Scenario: All secretary tests pass after deprecation
    Tool: Bash
    Preconditions: Code changes applied
    Steps:
      1. Run: go test ./bridge/pkg/secretary/... -count=1 -v 2>&1 | tail -20
      2. Assert: "FAIL" not in output
    Expected Result: All tests pass, zero failures
    Evidence: .sisyphus/evidence/task-1-secretary-tests.txt
  ```

  **Commit**: YES
  - Message: `refactor(secretary): deprecate warm dispatch — enforce cold-only container dispatch`
  - Files: `bridge/pkg/secretary/task_scheduler.go`, `bridge/pkg/secretary/task_dispatch.go`, `bridge/cmd/main.go`, `CHANGELOG.md`

- [x] 2. Implement DeepLinkHandler.kt for ArmorChat

  **What to do**:
  - CREATE `navigation/DeepLinkHandler.kt` — parse `armorclaw://room/{roomId}` and `armorclaw://email/approve/{approvalId}` URIs to NavHost routes
  - MODIFY `navigation/Route.kt` — add sealed class `Room(val roomId: String)` and `EmailApproval(val approvalId: String)` routes
  - MODIFY `navigation/ArmorClawNavHost.kt` — add composable destinations for Room (navigate to conversation) and EmailApproval (navigate to approval card in conversation)
  - Handle deep link URI parsing with proper error handling (malformed URIs → fallback to Home)
  - Export a `DeepLinkHandler.handle(uri: Uri): Route?` function that returns null for unrecognized schemes

  **Must NOT do**:
  - Do NOT handle `armorclaw://config` deep links (SignedConfigParser already handles those)
  - Do NOT handle `open_invites` extra (explicitly excluded from v0.7.0)
  - Do NOT create a unified handler for all deep link types
  - Do NOT add a settings screen

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3, 4)
  - **Blocks**: Tasks 6, 7, 10
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/Route.kt` — existing sealed class route pattern
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt` — existing NavHost composable pattern
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/config/SignedConfigParser.kt:106-113` — URI parsing pattern to follow

  **API/Type References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/data/model/EmailApprovalEvent.kt` — EmailApprovalEvent model with approval_id field
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/push/ArmorClawMessagingService.kt:307-315` — FCM creates `armorclaw://email/approve/$approvalId` deep links

  **Acceptance Criteria**:
  - [ ] `test -f applications/ArmorChat/.../navigation/DeepLinkHandler.kt` succeeds
  - [ ] `grep "class Room\|class EmailApproval" applications/ArmorChat/.../Route.kt` returns matches
  - [ ] DeepLinkHandler parses `armorclaw://room/!abc:server` → `Route.Room("!abc:server")`
  - [ ] DeepLinkHandler parses `armorclaw://email/approve/approval_123` → `Route.EmailApproval("approval_123")`
  - [ ] DeepLinkHandler returns null for `armorclaw://config?d=...` (handled by SignedConfigParser)

  **QA Scenarios**:
  ```
  Scenario: Deep link URI parsing correctness
    Tool: Bash (./gradlew test)
    Preconditions: DeepLinkHandler.kt exists
    Steps:
      1. Write unit test: DeepLinkHandlerTest.kt
      2. Test: handle(Uri.parse("armorclaw://room/!room123:server")) returns Route.Room
      3. Test: handle(Uri.parse("armorclaw://email/approve/appr_456")) returns Route.EmailApproval
      4. Test: handle(Uri.parse("armorclaw://config?d=abc")) returns null
      5. Test: handle(Uri.parse("https://example.com")) returns null
      6. Run: ./gradlew test --tests "*.DeepLinkHandlerTest"
    Expected Result: All 5 tests pass
    Evidence: .sisyphus/evidence/task-2-deeplink-parsing.txt

  Scenario: Malformed URI handling
    Tool: Bash
    Steps:
      1. Test: handle(Uri.parse("armorclaw://room/")) returns null (empty roomId)
      2. Test: handle(Uri.parse("armorclaw://email/approve/")) returns null (empty approvalId)
    Expected Result: Graceful null return, no crash
    Evidence: .sisyphus/evidence/task-2-malformed-uri.txt
  ```

  **Commit**: YES
  - Message: `feat(armorchat): implement DeepLinkHandler with notification routing`
  - Files: `navigation/DeepLinkHandler.kt`, `navigation/Route.kt`, `navigation/ArmorClawNavHost.kt`

- [x] 3. Wire SecurityConfigViewModel to SecurityConfigScreen

  **What to do**:
  - Read `SecurityConfigScreen.kt` and `SecurityConfigViewModel.kt`
  - Replace hardcoded `remember { mutableStateOf(listOf(...)) }` categories with ViewModel-backed state
  - Wire "Save & Continue" button to call `viewModel.savePermissions()` instead of just `onComplete()`
  - Connect ViewModel's `security.get_categories` and `security.set_category` RPC calls to the screen
  - Ensure loading/error states are handled in the UI
  - Verify that permissions are actually persisted to Bridge via RPC

  **Must NOT do**:
  - Do NOT add new security categories beyond what exists
  - Do NOT redesign the security config screen layout
  - Do NOT add a settings screen

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 4)
  - **Blocks**: Task 11 (placeholder test replacement)
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/viewmodel/SecurityConfigViewModel.kt` — ViewModel with real RPC calls already defined
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/viewmodel/BondingViewModel.kt` — reference for how ViewModel is wired to Screen

  **API/Type References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/network/BridgeApi.kt` — RPC methods for security config

  **Acceptance Criteria**:
  - [ ] `grep "viewModel\|SecurityConfigViewModel" .../SecurityConfigScreen.kt` returns match
  - [ ] `grep "remember.*mutableStateOf.*listOf.*banking" .../SecurityConfigScreen.kt` returns empty (hardcoded categories removed)
  - [ ] Save button calls `viewModel.savePermissions()` before `onComplete()`

  **QA Scenarios**:
  ```
  Scenario: SecurityConfigScreen uses ViewModel
    Tool: Bash (grep)
    Steps:
      1. grep "SecurityConfigViewModel" SecurityConfigScreen.kt
      2. Assert: match found
      3. grep "remember.*mutableStateOf.*listOf.*banking" SecurityConfigScreen.kt
      4. Assert: empty (hardcoded data removed)
    Expected Result: Screen delegates to ViewModel for state and persistence
    Evidence: .sisyphus/evidence/task-3-securityconfig-wired.txt

  Scenario: Save button persists permissions
    Tool: Bash
    Steps:
      1. Verify onClick handler calls viewModel.savePermissions()
      2. Verify viewModel.savePermissions() calls BridgeApi security.set_category RPC
    Expected Result: Permissions are persisted to Bridge, not just in-memory
    Evidence: .sisyphus/evidence/task-3-save-persists.txt
  ```

  **Commit**: YES
  - Message: `fix(armorchat): wire SecurityConfigViewModel to SecurityConfigScreen`
  - Files: `SecurityConfigScreen.kt`, potentially `SecurityConfigViewModel.kt` if adjustments needed

- [x] 4. Create review.md Project Status Document

  **What to do**:
  - Create `review.md` at repo root (mandated by AGENTS.md: "Read review.md before planning or modifying code")
  - Document current project status: v0.6.0 released, known gaps, architectural decisions
  - Include: subsystem status table, security decisions (NetworkMode: none, crash-only WebSocket), deprecation notices
  - Reference the v0.7.0 plan for active work items

  **Must NOT do**:
  - Do NOT include implementation details that belong in doc/ files
  - Do NOT duplicate armorclaw.md content

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:
  - `AGENTS.md` — mandates review.md existence
  - `.sisyphus/plans/v070-master-plan.md` — active plan to reference

  **Acceptance Criteria**:
  - [ ] `test -f review.md` succeeds
  - [ ] File contains subsystem status table
  - [ ] File documents NetworkMode: none as non-negotiable constraint
  - [ ] File documents crash-only WebSocket design decision

  **QA Scenarios**:
  ```
  Scenario: review.md exists and is complete
    Tool: Bash
    Steps:
      1. test -f review.md && echo "PASS" || echo "FAIL"
      2. grep "NetworkMode" review.md
      3. grep "WebSocket\|crash-only" review.md
    Expected Result: File exists with key architectural decisions documented
    Evidence: .sisyphus/evidence/task-4-review-md.txt
  ```

  **Commit**: YES
  - Message: `docs: create review.md project status document`
  - Files: `review.md`

- [x] 5. Wire EventBus Through HTTP /ws Server

  **What to do**:
  - Replace `bridge/pkg/websocket/websocket.go` stub (74 lines) with a thin adapter that delegates to `bridge/pkg/http/server.go`'s existing WebSocket infrastructure
  - Route `EventBus.PublishBridgeEvent()` to the HTTP server's `Broadcast*` methods
  - Option: Add a generic `BroadcastEvent(eventType string, payload []byte)` method to HTTP server, or map EventBus event types to existing 9 broadcast methods
  - Fix the `sendToSubscriber()` goroutine in eventbus.go:435-437 that discards data (`_ = data`)
  - Preserve the crash-only `log.Fatalf` at eventbus.go:146 — do NOT add graceful fallback
  - Verify `WebSocketEnabled=true` works end-to-end without crash (because stub is now real)

  **Must NOT do**:
  - Do NOT touch the HTTP server's working `/ws` endpoint (server.go:673-819)
  - Do NOT remove the `log.Fatalf` crash-only pattern — it's intentional CTO design
  - Do NOT build event replay, cursor-based subscription, or connection management UI
  - Do NOT build a general-purpose pub/sub system

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 1 completing first)
  - **Parallel Group**: Wave 2 (with Tasks 6, 7, 8)
  - **Blocks**: Tasks 8, 9
  - **Blocked By**: Task 1

  **References**:
  **Pattern References**:
  - `bridge/pkg/http/server.go:673-819` — working gorilla/websocket implementation to wire INTO
  - `bridge/pkg/http/server.go:872-1095` — existing Broadcast methods (BroadcastAgentStatus, BroadcastWorkflowProgress, etc.)
  - `bridge/pkg/eventbus/eventbus.go:119-146` — EventBus wiring to WebSocket stub (including the crash-only log.Fatalf)

  **API/Type References**:
  - `bridge/pkg/websocket/websocket.go` — 74-line stub to replace
  - `bridge/pkg/eventbus/eventbus.go:435-437` — sendToSubscriber discarding data
  - `bridge/pkg/config/config.go:468-475` — WebSocket config fields

  **Acceptance Criteria**:
  - [ ] `grep -n "not yet implemented" bridge/pkg/websocket/websocket.go` returns empty
  - [ ] `grep "EventBus\|PublishBridgeEvent" bridge/pkg/http/server.go` returns match
  - [ ] `go test ./bridge/pkg/eventbus/... -count=1` → PASS
  - [ ] Setting `WebSocketEnabled=true` in config does NOT crash the bridge (stub replaced with real adapter)

  **QA Scenarios**:
  ```
  Scenario: WebSocket event delivery end-to-end
    Tool: Bash
    Preconditions: Bridge running with WebSocketEnabled=true
    Steps:
      1. Start bridge with WebSocketEnabled=true in config
      2. Connect to ws://localhost:8443/ws via websocat or test client
      3. Trigger an agent status change event
      4. Verify event arrives on WebSocket connection
    Expected Result: Events flow from EventBus through HTTP /ws to connected clients
    Evidence: .sisyphus/evidence/task-5-ws-e2e.txt

  Scenario: Crash-only pattern preserved
    Tool: Bash
    Steps:
      1. grep "log.Fatalf" bridge/pkg/eventbus/eventbus.go
      2. Assert: match found (crash-only pattern still in place)
      3. Verify that with WebSocketEnabled=true, the bridge starts successfully (stub replaced)
    Expected Result: Crash-only pattern preserved but unreachable because stub is replaced
    Evidence: .sisyphus/evidence/task-5-crash-only.txt

  Scenario: sendToSubscriber no longer discards data
    Tool: Bash
    Steps:
      1. grep "_ = data" bridge/pkg/eventbus/eventbus.go
      2. Assert: no match (dead discard removed)
    Expected Result: EventBus data flows to WebSocket adapter instead of being discarded
    Evidence: .sisyphus/evidence/task-5-no-discard.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): wire EventBus through HTTP WebSocket server`
  - Files: `bridge/pkg/websocket/websocket.go`, `bridge/pkg/eventbus/eventbus.go`, possibly `bridge/pkg/http/server.go`

- [x] 6. Add Cold-Start Intent Handling to MainActivity

  **What to do**:
  - Add `onNewIntent()` override to `MainActivity.kt` to handle warm-resume notification taps
  - Add `LaunchedEffect(Unit)` in the root composable to check `getIntent()` for deep link URIs on cold start
  - Both paths should call `DeepLinkHandler.handle(uri)` and navigate to the appropriate route
  - Handle edge case: if user is mid-setup (on bonding/security screen), queue the deep link instead of navigating away

  **Must NOT do**:
  - Do NOT redesign the navigation structure
  - Do NOT handle deep links during bonding/setup unless setup is complete

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 7, 8)
  - **Blocks**: Task 10
  - **Blocked By**: Task 2 (needs DeepLinkHandler)

  **References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/MainActivity.kt` — bare 35-line Activity to modify
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/DeepLinkHandler.kt` — created in Task 2

  **Acceptance Criteria**:
  - [ ] `grep "override fun onNewIntent" applications/ArmorChat/.../MainActivity.kt` returns match
  - [ ] `grep "LaunchedEffect\|getIntent" applications/ArmorChat/.../MainActivity.kt` returns match
  - [ ] Deep link intent from cold start is handled within 500ms of Activity creation

  **QA Scenarios**:
  ```
  Scenario: Cold-start notification tap
    Tool: Bash (adb / manual test description)
    Steps:
      1. Kill ArmorChat app completely
      2. Send FCM push notification with armorclaw://room/!room123:server
      3. Tap notification
      4. Verify app opens directly to conversation room, not home screen
    Expected Result: Cold-start deep link navigates to correct destination
    Evidence: .sisyphus/evidence/task-6-cold-start.txt

  Scenario: Warm-resume notification tap
    Tool: Bash
    Steps:
      1. Open ArmorChat app (on home screen)
      2. Send FCM push notification with armorclaw://email/approve/approval_123
      3. Tap notification
      4. Verify onNewIntent handles the URI and navigates to approval card
    Expected Result: Warm-resume deep link navigates to approval card
    Evidence: .sisyphus/evidence/task-6-warm-resume.txt

  Scenario: Mid-setup notification tap
    Tool: Bash
    Steps:
      1. Open ArmorChat app mid-bonding flow
      2. Send FCM push notification
      3. Tap notification
      4. Verify app does NOT navigate away from bonding screen
    Expected Result: Deep link queued or suppressed during setup
    Evidence: .sisyphus/evidence/task-6-mid-setup.txt
  ```

  **Commit**: YES
  - Message: `fix(armorchat): add cold-start intent handling for notification deep links`
  - Files: `MainActivity.kt`

- [x] 7. Update AndroidManifest Intent Filters

  **What to do**:
  - Add `<intent-filter>` for `armorclaw://room/` scheme in AndroidManifest.xml
  - Add `<intent-filter>` for `armorclaw://email/approve/` scheme in AndroidManifest.xml
  - Ensure singleTop launchMode on MainActivity to prevent stacking on notification taps
  - Verify existing `armorclaw://config` and `https://armorclaw.app/config` filters are untouched

  **Must NOT do**:
  - Do NOT remove or modify existing intent filters
  - Do NOT add `matrix:` scheme handling
  - Do NOT add `open_invites` handling

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 6, 8)
  - **Blocks**: Task 10
  - **Blocked By**: Task 2 (needs routes defined)

  **References**:
  - `applications/ArmorChat/app/src/main/AndroidManifest.xml` — existing manifest

  **Acceptance Criteria**:
  - [ ] `grep "armorclaw://room" AndroidManifest.xml` returns match
  - [ ] `grep "armorclaw://email" AndroidManifest.xml` returns match
  - [ ] `grep "singleTop" AndroidManifest.xml` returns match for MainActivity

  **QA Scenarios**:
  ```
  Scenario: Intent filters registered
    Tool: Bash (aapt dump / grep)
    Steps:
      1. grep "armorclaw" AndroidManifest.xml | grep -E "room|email"
      2. Assert: both room and email scheme filters present
    Expected Result: Android can route armorclaw:// URIs to ArmorChat
    Evidence: .sisyphus/evidence/task-7-manifest-filters.txt
  ```

  **Commit**: YES (groups with Task 6)
  - Message: `fix(armorchat): add intent-filters for room and email deep links`
  - Files: `AndroidManifest.xml`

- [x] 8. Clean Up EventBus sendToSubscriber Dead Goroutine

  **What to do**:
  - Fix `sendToSubscriber()` in eventbus.go:403-449 — the goroutine spawns for each subscriber but discards data (`_ = data` at line 437)
  - When WebSocketEnabled=false, this goroutine should not be spawned at all (it's a no-op leak)
  - When WebSocketEnabled=true, data should flow through the adapter (wired in Task 5)
  - Verify no goroutine leak by checking subscriber count vs running goroutines

  **Must NOT do**:
  - Do NOT remove the subscriber model entirely
  - Do NOT touch the ring buffer or durable log

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 5)
  - **Parallel Group**: Wave 2 (after Task 5)
  - **Blocks**: Task 9
  - **Blocked By**: Task 5

  **References**:
  - `bridge/pkg/eventbus/eventbus.go:403-449` — sendToSubscriber goroutine
  - `bridge/pkg/eventbus/eventbus.go:435-437` — `_ = data` discard line

  **Acceptance Criteria**:
  - [ ] `grep "_ = data" bridge/pkg/eventbus/eventbus.go` returns empty
  - [ ] `go test ./bridge/pkg/eventbus/... -count=1` → PASS

  **QA Scenarios**:
  ```
  Scenario: No goroutine leak with WebSocketEnabled=false
    Tool: Bash
    Steps:
      1. Start bridge with WebSocketEnabled=false
      2. Publish 100 events
      3. Check goroutine count: runtime.NumGoroutine()
      4. Assert: no growth proportional to event count
    Expected Result: Stable goroutine count, no leak from discarded subscriber goroutines
    Evidence: .sisyphus/evidence/task-8-goroutine-leak.txt
  ```

  **Commit**: YES
  - Message: `refactor(eventbus): remove sendToSubscriber dead goroutine`
  - Files: `bridge/pkg/eventbus/eventbus.go`

- [x] 9. E2E Test: WebSocket Event Delivery

  **What to do**:
  - Write E2E test that starts bridge with WebSocketEnabled=true
  - Connect WebSocket client to /ws endpoint
  - Trigger agent status change event via test RPC
  - Verify event arrives on WebSocket client within 2 seconds
  - Test disconnection/reconnection behavior
  - Test that events are not queued/dropped during disconnection (documented behavior)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Tasks 5, 8)
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 19
  - **Blocked By**: Tasks 5, 8

  **References**:
  - `bridge/pkg/http/server_test.go` — existing HTTP server test patterns

  **Acceptance Criteria**:
  - [ ] `go test ./bridge/pkg/http/... -run TestWebSocketE2E -v` → PASS
  - [ ] Event delivery latency < 2 seconds

  **QA Scenarios**:
  ```
  Scenario: Event delivery within latency target
    Tool: Bash (go test)
    Steps:
      1. go test ./bridge/pkg/http/... -run TestWebSocketE2E -v
      2. Assert: PASS
    Expected Result: Events flow from EventBus to WebSocket client in <2s
    Evidence: .sisyphus/evidence/task-9-ws-e2e.txt
  ```

  **Commit**: YES
  - Message: `test(bridge): E2E WebSocket event delivery`
  - Files: `bridge/pkg/http/server_test.go` or new test file

- [x] 10. E2E Test: Deep Link Cold-Start and Warm-Resume

  **What to do**:
  - Write instrumented test that verifies deep link handling
  - Test cold-start: app killed, notification tap opens to correct screen
  - Test warm-resume: app in background, notification tap navigates correctly
  - Test mid-setup: app on bonding screen, notification tap does not navigate away
  - Test multiple pending: tap second notification, first notification context preserved
  - Replace `assertTrue(true)` placeholders in IntegrationTest.kt for these scenarios

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Tasks 2, 6, 7)
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 19
  - **Blocked By**: Tasks 2, 6, 7

  **References**:
  - `applications/ArmorChat/app/src/androidTest/java/app/armorclaw/IntegrationTest.kt` — existing placeholder tests

  **Acceptance Criteria**:
  - [ ] `grep "assertTrue.*true" IntegrationTest.kt | wc -l` returns 0 for deep link tests
  - [ ] Cold-start and warm-resume tests both pass

  **QA Scenarios**:
  ```
  Scenario: All deep link E2E tests pass
    Tool: Bash (./gradlew connectedAndroidTest)
    Steps:
      1. ./gradlew connectedAndroidTest --tests "*.DeepLinkTest"
      2. Assert: PASS
    Expected Result: Cold-start, warm-resume, mid-setup, and multi-pending tests pass
    Evidence: .sisyphus/evidence/task-10-deeplink-e2e.txt
  ```

  **Commit**: YES
  - Message: `test(armorchat): E2E deep link cold-start and warm-resume`
  - Files: `IntegrationTest.kt` or new `DeepLinkTest.kt`

- [x] 11. Replace ArmorChat Placeholder Integration Tests

  **What to do**:
  - Audit all 12 `assertTrue("placeholder", true)` tests in IntegrationTest.kt
  - Replace placeholders for SecurityConfig flow with real assertions
  - Test: SecurityConfigViewModel loads categories from Bridge RPC
  - Test: SecurityConfigViewModel saves permissions via RPC
  - Leave placeholders for features not yet implemented (voice, etc.) with explicit skip annotations

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 3)
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 19
  - **Blocked By**: Task 3

  **References**:
  - `applications/ArmorChat/app/src/androidTest/java/app/armorclaw/IntegrationTest.kt`

  **Acceptance Criteria**:
  - [ ] `grep "assertTrue.*true" IntegrationTest.kt | grep -v "skip\|voice\|not.yet" | wc -l` returns 0 for SecurityConfig tests
  - [ ] SecurityConfig load/save tests have real assertions

  **QA Scenarios**:
  ```
  Scenario: No placeholder assertions for implemented features
    Tool: Bash
    Steps:
      1. grep "assertTrue.*true" IntegrationTest.kt | grep -v "skip"
      2. Assert: no matches for SecurityConfig-related tests
    Expected Result: All implemented features have real test assertions
    Evidence: .sisyphus/evidence/task-11-placeholder-replacement.txt
  ```

  **Commit**: YES
  - Message: `test(armorchat): replace placeholder integration tests`
  - Files: `IntegrationTest.kt`

### Phase 2: Integration Polish (Weeks 3-5)

- [x] 12. Add WorkflowStep.Input Field for Inter-Step Data

  **What to do**:
  - Add `Input map[string]any` field to `WorkflowStep` struct in types.go
  - Support template variable references like `{{steps.step_1.data.order_id}}`
  - Write migration guide: how to update existing template JSON files
  - Add JSON tag `input` to the field with `omitempty`
  - Write test that verifies backward compatibility: templates without `input` still work
  - This is a breaking schema change — document in CHANGELOG.md

  **Must NOT do**:
  - Do NOT add conditional branching based on data
  - Do NOT add data transformation pipelines
  - Do NOT add schema validation for step outputs

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 14, 15)
  - **Blocks**: Tasks 13, 16, 18
  - **Blocked By**: None (can start Phase 2 immediately after Phase 1)

  **References**:
  **Pattern References**:
  - `bridge/pkg/secretary/types.go:64-91` — WorkflowStep struct definition
  - `bridge/pkg/secretary/result.go:50` — ContainerStepResult.Data field that produces the data

  **API/Type References**:
  - Template JSON files — existing format to extend

  **Acceptance Criteria**:
  - [ ] `grep "Input\|StepData" bridge/pkg/secretary/types.go` returns match for new field
  - [ ] Templates without `input` field continue to work (backward compatible)
  - [ ] Migration guide document exists

  **QA Scenarios**:
  ```
  Scenario: Backward compatibility
    Tool: Bash
    Steps:
      1. Load existing template JSON without "input" field
      2. Unmarshal into WorkflowStep
      3. Assert: no error, Input field is nil/empty
    Expected Result: Existing templates unaffected
    Evidence: .sisyphus/evidence/task-12-backward-compat.txt

  Scenario: New template with input references
    Tool: Bash
    Steps:
      1. Load template JSON with "input": {"order_id": "{{steps.step_1.data.order_id}}"}
      2. Unmarshal into WorkflowStep
      3. Assert: Input field populated with template reference
    Expected Result: New input field works
    Evidence: .sisyphus/evidence/task-12-input-field.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add WorkflowStep.Input field for inter-step data`
  - Files: `bridge/pkg/secretary/types.go`, migration guide

- [x] 13. Propagate ContainerStepResult.Data Between Sequential Steps

  **What to do**:
  - Add accumulator map in `executeSequential()` (orchestrator_integration.go) to collect step results
  - After step N completes, merge `ContainerStepResult.Data` into accumulator
  - Before step N+1 executes, resolve `{{steps.step_id.data.key}}` references in step Config
  - Inject resolved data into STEP_CONFIG env var alongside static config
  - Update `step_config.py` in container to expose `_prev_step_data` field
  - Handle edge case: step produces no Data (empty merge, no error)
  - Handle edge case: template reference points to non-existent step (error with clear message)

  **Must NOT do**:
  - Do NOT change parallel step execution (addressed in Task 16 separately)
  - Do NOT add conditional branching

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 12)
  - **Parallel Group**: Wave 4
  - **Blocks**: Tasks 16, 19
  - **Blocked By**: Task 12

  **References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:233-262` — executeSequential loop that discards results
  - `bridge/pkg/secretary/result.go:50` — ContainerStepResult.Data
  - `container/openclaw/step_config.py` — STEP_CONFIG parser to extend
  - `container/openclaw/result_writer.py` — writes result.Data

  **Acceptance Criteria**:
  - [ ] `grep "ContainerResult\.Data\|stepResult.*Data\|accumulatedData" bridge/pkg/secretary/orchestrator_integration.go` returns match
  - [ ] `grep "_prev_step_data\|prevStepData" container/openclaw/step_config.py` returns match
  - [ ] `go test ./bridge/pkg/secretary/... -run "TestSequential\|TestDataPass" -v` → PASS

  **QA Scenarios**:
  ```
  Scenario: Step N output feeds into step N+1
    Tool: Bash (go test)
    Steps:
      1. Create 2-step workflow: step_1 produces {"order_id": "abc123"}
      2. step_2 config has input: {"order_ref": "{{steps.step_1.data.order_id}}"}
      3. Execute workflow
      4. Assert: step_2 receives order_ref=abc123 in STEP_CONFIG
    Expected Result: Data flows between sequential steps
    Evidence: .sisyphus/evidence/task-13-inter-step-data.txt

  Scenario: Missing step reference produces clear error
    Tool: Bash
    Steps:
      1. Create workflow with reference to non-existent step: {{steps.step_99.data.x}}
      2. Execute workflow
      3. Assert: error message contains "step_99" and "not found"
    Expected Result: Graceful failure with actionable error
    Evidence: .sisyphus/evidence/task-13-missing-ref.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): propagate ContainerStepResult.Data between sequential steps`
  - Files: `bridge/pkg/secretary/orchestrator_integration.go`, `container/openclaw/step_config.py`

- [x] 14. Persist ServerConfig via EncryptedSharedPreferences

  **What to do**:
  - Create `ConfigManager.kt` in ArmorChat config package (follow `ArmorTerminal`'s ConfigManager.kt as reference)
  - Use Android EncryptedSharedPreferences to persist `ServerConfig` (homeserver URL, device ID)
  - Save config after successful QR scan / SignedConfigParser parse
  - Load config on app startup; if expired, fall back to re-provisioning screen
  - Handle expiration: ServerConfig has `expiresAt` field from SignedConfigParser

  **Must NOT do**:
  - Do NOT persist auth tokens (accessToken, refreshToken) — v0.8.0 with separate security review
  - Do NOT add a settings screen

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 12, 13, 15)
  - **Blocks**: Task 17
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `applications/ArmorTerminal/android-app/src/main/java/com/armorclaw/armorterminal/config/ConfigManager.kt` — reference implementation
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/config/SignedConfigParser.kt` — produces ServerConfig

  **API/Type References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/data/repository/BridgeRepository.kt` — holds in-memory credentials (homeserverUrl, accessToken, deviceId)

  **Acceptance Criteria**:
  - [ ] `find applications/ArmorChat -name "ConfigManager.kt"` returns path
  - [ ] `grep "EncryptedSharedPreferences\|SharedPreferences" ConfigManager.kt` returns match
  - [ ] ServerConfig survives app process death

  **QA Scenarios**:
  ```
  Scenario: Config persists across restart
    Tool: Bash (adb / test)
    Steps:
      1. Complete QR scan → ServerConfig saved
      2. Kill app process
      3. Restart app
      4. Assert: app loads ServerConfig, does not show bonding screen
    Expected Result: Config loaded from EncryptedSharedPreferences
    Evidence: .sisyphus/evidence/task-14-config-persist.txt

  Scenario: Expired config triggers re-provisioning
    Tool: Bash
    Steps:
      1. Save ServerConfig with expiresAt in the past
      2. Restart app
      3. Assert: bonding/setup screen shown
    Expected Result: Expired config triggers re-provisioning flow
    Evidence: .sisyphus/evidence/task-14-config-expiry.txt
  ```

  **Commit**: YES
  - Message: `feat(armorchat): persist ServerConfig via EncryptedSharedPreferences`
  - Files: new `ConfigManager.kt`, modifications to setup flow

- [x] 15. Wire Admin Panel to Real Bridge API

  **What to do**:
  - Replace mock data fallbacks in `applications/admin-panel/src/App.tsx` with real API calls
  - Ensure `bridgeApi.ts` service layer calls actual Bridge endpoints
  - Wire Dashboard page to real agent/system stats
  - Wire Security page to real `security.get_categories` RPC
  - Wire Devices page to real device listing
  - Keep loading states and error handling for API failures

  **Must NOT do**:
  - Do NOT add new pages beyond the existing 7
  - Do NOT redesign the UI layout
  - Do NOT add authentication/login screen (admin panel is local-network only)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 12, 13, 14)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `applications/admin-panel/src/App.tsx` — main app with mock data (MOCK_STATS at line 108)
  - `applications/admin-panel/src/services/bridgeApi.ts` — API service layer

  **Acceptance Criteria**:
  - [ ] `grep "MOCK_STATS\|mock" applications/admin-panel/src/App.tsx` returns empty
  - [ ] Dashboard shows real agent/system data from Bridge

  **QA Scenarios**:
  ```
  Scenario: Admin panel uses real API
    Tool: Bash
    Steps:
      1. grep "MOCK_STATS\|mockData\|MOCK" applications/admin-panel/src/App.tsx
      2. Assert: empty (no mock data references)
    Expected Result: All data comes from Bridge API
    Evidence: .sisyphus/evidence/task-15-admin-api.txt
  ```

  **Commit**: YES
  - Message: `feat(admin-panel): wire to real Bridge API, remove mock data`
  - Files: `applications/admin-panel/src/App.tsx`, `applications/admin-panel/src/services/bridgeApi.ts`

- [x] 16. Handle Parallel Step Data Merge

  **What to do**:
  - Extend `StepParallelSplit` / `StepParallelMerge` handling in orchestrator_integration.go
  - When parallel steps complete, collect all `ContainerStepResult.Data` outputs
  - Merge into a structured map keyed by step_id
  - Make available to subsequent steps via `{{steps.parallel_split_id.data.branch_1.key}}` syntax

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 13)
  - **Parallel Group**: Wave 5
  - **Blocks**: Task 19
  - **Blocked By**: Task 13

  **References**:
  - `bridge/pkg/secretary/types.go` — StepParallelSplit, StepParallelMerge
  - `bridge/pkg/secretary/orchestrator_integration.go` — parallel execution logic

  **Acceptance Criteria**:
  - [ ] Parallel step outputs are merged into workflow context
  - [ ] Subsequent steps can reference parallel branch outputs

  **QA Scenarios**:
  ```
  Scenario: Parallel outputs merged correctly
    Tool: Bash (go test)
    Steps:
      1. Create 3-branch parallel step
      2. Each branch produces different data
      3. Merge step receives all 3 outputs keyed by branch step_id
    Expected Result: All branch data accessible in merge step
    Evidence: .sisyphus/evidence/task-16-parallel-merge.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): merge parallel step results into workflow context`
  - Files: `bridge/pkg/secretary/orchestrator_integration.go`

- [x] 17. ServerConfig Expiration Check and Re-Provisioning Fallback

  **What to do**:
  - Add expiration check on app startup in ConfigManager
  - If ServerConfig is expired, clear it and navigate to bonding screen
  - Show brief toast/snackbar explaining "Configuration expired, please re-provision"
  - Config expiration TTL should match what SignedConfigParser generates

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 14)
  - **Parallel Group**: Wave 5
  - **Blocks**: None
  - **Blocked By**: Task 14

  **Acceptance Criteria**:
  - [ ] Expired config triggers re-provisioning screen
  - [ ] Non-expired config loads normally

  **QA Scenarios**:
  ```
  Scenario: Expired config handled gracefully
    Tool: Bash
    Steps:
      1. Save ServerConfig with expiresAt = now() - 1 hour
      2. Restart app
      3. Assert: bonding screen shown with expiration message
    Expected Result: User prompted to re-provision, not stuck
    Evidence: .sisyphus/evidence/task-17-expiry-fallback.txt
  ```

  **Commit**: YES
  - Message: `feat(armorchat): add ServerConfig expiration check and re-provisioning fallback`

- [x] 18. Template JSON Migration Tool for WorkflowStep.Input Schema

  **What to do**:
  - Create a CLI tool or script that migrates existing template JSON files
  - Add `"input": {}` field to WorkflowStep objects that lack it
  - Validate templates against new schema
  - Document migration command in CHANGELOG.md

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 12)
  - **Parallel Group**: Wave 5
  - **Blocks**: None
  - **Blocked By**: Task 12

  **Acceptance Criteria**:
  - [ ] Migration tool exists and runs without error on existing templates
  - [ ] Migrated templates pass schema validation

  **QA Scenarios**:
  ```
  Scenario: Migration preserves existing template data
    Tool: Bash
    Steps:
      1. Run migration tool on test template
      2. Diff output vs input
      3. Assert: only "input": {} added, no other changes
    Expected Result: Non-destructive migration
    Evidence: .sisyphus/evidence/task-18-migration.txt
  ```

  **Commit**: YES
  - Message: `tool(secretary): template JSON migration for WorkflowStep.Input schema`

### Phase 3: Release Hardening (Weeks 6-8)

- [x] 19. Full-Stack E2E Test Suite

  **What to do**:
  - Create comprehensive test suite covering: Bridge → Secretary → Sidecars → ArmorChat flows
  - Test email HITL end-to-end: inbound email → PII detection → Matrix event → approval → outbound
  - Test workflow execution: create → dispatch → step execution → completion → event emission
  - Test WebSocket: event publication → delivery to connected client
  - Test deep links: FCM notification → tap → correct screen navigation

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Blocked By**: Tasks 9, 10, 11, 13
  - **Blocks**: Tasks F1-F4

- [x] 20. Security Audit (YARA, PII, Isolation)

  **What to do**:
  - Run full security audit: YARA rule coverage, PII masking completeness, container isolation
  - Verify NetworkMode: none enforced on all containers
  - Verify BlindFill never exposes raw PII to agents
  - Verify Governor-Shield scrubs all tool call arguments
  - Document findings in review.md

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

- [x] 21. Performance Benchmarks

  **What to do**:
  - Benchmark cold dispatch latency (container spawn to ready)
  - Benchmark WebSocket event delivery latency
  - Benchmark inter-step data propagation overhead
  - Benchmark Secretary workflow throughput (sequential vs parallel)
  - Document baseline numbers for regression detection

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

- [x] 22. Documentation Sync

  **What to do**:
  - Sync all sub-docs (voice-stack.md, secretary-workflow.md, etc.) with armorclaw.md
  - Update email-android-integration.md with ArmorChat findings appendix
  - Update armorclaw.md with inter-step data passing, WebSocket wiring, deprecation notices
  - Update CHANGELOG.md with v0.7.0 release notes
  - Update review.md with final audit results

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./bridge/...` + `go test ./bridge/... -count=1` + `./gradlew test`. Review all changed files for: `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (features working together, not isolation). Test edge cases: cold-start notification, mid-setup notification tap, multiple pending approvals, network loss during WebSocket. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination: Task N touching Task M's files. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **1**: `refactor(secretary): deprecate warm dispatch — enforce cold-only container dispatch`
- **2**: `feat(armorchat): implement DeepLinkHandler with notification routing`
- **3**: `fix(armorchat): wire SecurityConfigViewModel to SecurityConfigScreen`
- **4**: `docs: create review.md project status document`
- **5**: `feat(bridge): wire EventBus through HTTP WebSocket server`
- **6**: `fix(armorchat): add cold-start intent handling for notification deep links`
- **7**: `fix(armorchat): add intent-filters for room and email deep links`
- **8**: `refactor(eventbus): remove sendToSubscriber dead goroutine`
- **9**: `test(bridge): E2E WebSocket event delivery`
- **10**: `test(armorchat): E2E deep link cold-start and warm-resume`
- **11**: `test(armorchat): replace placeholder integration tests`
- **12**: `feat(secretary): add WorkflowStep.Input field for inter-step data`
- **13**: `feat(secretary): propagate ContainerStepResult.Data between sequential steps`
- **14**: `feat(armorchat): persist ServerConfig via EncryptedSharedPreferences`
- **15**: `feat(admin-panel): wire to real Bridge API, remove mock data`
- **16**: `feat(secretary): merge parallel step results into workflow context`
- **17**: `feat(armorchat): add ServerConfig expiration check and re-provisioning fallback`
- **18**: `tool(secretary): template JSON migration for WorkflowStep.Input schema`
- **19**: `test: full-stack E2E test suite`
- **20**: `audit: security review (YARA, PII, isolation)`
- **21**: `bench: performance benchmarks for Bridge + Secretary`
- **22**: `docs: sync all sub-docs with armorclaw.md`

---

## Success Criteria

### Verification Commands
```bash
# Warm dispatch removed
grep -r "warmDispatch\|warm_dispatch\|EventTypeTaskDispatch\|BuildTaskDispatchPayload" bridge/pkg/secretary/ --include="*.go" | grep -v "_test.go"
# Expected: empty

# WebSocket wired
grep -n "not yet implemented" bridge/pkg/websocket/websocket.go
# Expected: empty

# DeepLinkHandler exists
test -f applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/DeepLinkHandler.kt
# Expected: 0 (success)

# SecurityConfig wired
grep "viewModel\|SecurityConfigViewModel" applications/ArmorChat/app/src/main/java/app/armorclaw/ui/security/SecurityConfigScreen.kt
# Expected: match found

# Inter-step data
grep "Input\|StepData\|PreviousStepData" bridge/pkg/secretary/types.go
# Expected: match found

# All Go tests pass
go test ./bridge/... -count=1
# Expected: PASS, 0 failures
```

### Final Checklist
- [ ] All "Must Have" items present and verified
- [ ] All "Must NOT Have" items absent (grep confirms)
- [ ] All Go tests pass (`go test ./bridge/... -count=1`)
- [ ] All Android tests pass (`./gradlew test`)
- [ ] Zero `assertTrue(true)` placeholder tests remain in touched files
- [ ] `review.md` exists and is current
- [ ] Documentation synchronized across all sub-docs
