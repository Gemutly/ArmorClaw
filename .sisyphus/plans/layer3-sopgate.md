# Layer 3 — SOPGate: Wire Workflow Execution Engine

## TL;DR

> **Quick Summary**: Connect the existing but dormant `OrchestratorIntegration` execution engine into ArmorClaw's production code so workflows actually execute steps, and route scheduled tasks with `template_id` through the workflow engine instead of raw description dispatch.
>
> **Deliverables**:
> - Fix RoomID bug in `executeStep()` (uses user ID instead of room ID)
> - Add `room_id` column to `workflows` table with migration
> - Fix `CreateTemplate` INSERT bug (10 cols / 9 placeholders — runtime panic)
> - Construct full `OrchestratorIntegration` dependency chain in `main.go`
> - Wire execution engine into `SecretaryCommandHandler`
> - Route scheduler tasks with `TemplateID` through workflow engine
> - Update `doc/armorclaw.md` with workflow lifecycle documentation
> - ~15-20 tests for new code paths
>
> **Estimated Effort**: Medium (6 implementation tasks + 1 doc task + 1 test task + 4 verification)
> **Parallel Execution**: YES — 3 waves
> **Critical Path**: Task 1 (RoomID schema) → Task 2 (bug fixes) → Task 3 (construction chain) → Task 4-5 (wiring) → Task 6 (scheduler routing) → Task 7 (doc) → Task 8 (tests)

---

## Context

### Original Request
Wire the existing template/workflow execution engine (`OrchestratorIntegration`) into production code. Currently `secretary.start_workflow` creates a workflow record but never executes steps. Scheduled tasks with `template_id` should route through the workflow engine instead of raw warm/cold dispatch.

### Interview Summary
**Key Discussions**:
- RoomID design: Add `room_id TEXT DEFAULT ''` to workflows table, persist from command context, empty = fire-and-forget
- Test strategy: Tests-after, ~15-20 tests
- Two execution paths: Path A (ticker stub, active) + Path B (real execution, zero callers). Merge by removing Path A's goroutine, keeping its state methods for Path B callbacks.
- No LLM classifier — template matching is explicit via `template_id`
- Do NOT modify StepExecutor, DependencyValidator, ApprovalEngine, NotificationService internals

**Research Findings**:
- `OrchestratorIntegration` has zero production callers — it's dead code
- `SecretaryCommandHandler` in `main.go:2596-2607` has `Store: nil, Orchestrator: nil` — crashes on any store/orchestrator access
- `CreateTemplate` INSERT has 10 columns but 9 `?` placeholders — runtime panic on every call
- `StepExecutor` takes `*studio.AgentFactory` directly (not an interface) — available via `studioService.GetFactory()`
- `WorkflowOrchestratorImpl` requires `Store` + `EventEmitter` — both available in `main.go`
- `eventBus` already exists in `main.go:2665` — passed to RPC server config

### Metis Review
**Identified Gaps** (addressed):
- Full dependency chain unwired: `EventEmitter → WorkflowOrchestratorImpl → DependencyValidator → StepExecutor → OrchestratorIntegration` — none constructed in `main.go`
- `CreateTemplate` bug blocks `StartWorkflowExecution()` (it calls `GetTemplate()`)
- `SpawnRequestRef` has no `RoomID` field — cold dispatch can't propagate room
- `SecretaryCommandHandler` nil Store/Orchestrator must be fixed as part of wiring
- RPC has no `room_id` — RPC-initiated workflows will be fire-and-forget (room_id = "")
- `ListWorkflows` no-op filter — explicitly out of scope (separate PR)
- Warm dispatch sends `CronExpression` as Description — explicitly out of scope

---

## Work Objectives

### Core Objective
Make `OrchestratorIntegration.StartWorkflowExecution()` callable from production code so that workflows actually execute their steps, and route scheduled tasks with `template_id` through the workflow engine.

### Concrete Deliverables
- `workflows` table gains `room_id TEXT DEFAULT ''` column
- `Workflow` struct gains `RoomID string` field
- `CreateTemplate` INSERT fixed (10 cols / 10 placeholders)
- `executeStep()` uses `workflow.RoomID` instead of `workflow.CreatedBy`
- `SpawnRequestRef` gains `RoomID string` field
- `OrchestratorIntegration` constructed in `main.go` with full dependency chain
- `SecretaryCommandHandler` wired with real Store and Orchestrator
- `TaskScheduler.dispatchTask()` routes through workflow engine when `TemplateID != ""`
- `TaskDispatchPayload` gains `WorkflowID string` for traceability
- `doc/armorclaw.md` updated with workflow lifecycle documentation
- ~15-20 tests covering new code paths

### Definition of Done
- [ ] `cd bridge && go build ./...` succeeds
- [ ] `cd bridge && go test ./pkg/secretary/...` — all tests pass (existing + new)
- [ ] `executeStep()` sets `RoomID: workflow.RoomID` (not `CreatedBy`)
- [ ] `NewOrchestratorIntegration()` has at least 1 production caller
- [ ] `SecretaryCommandHandler` has non-nil Store and non-nil Orchestrator
- [ ] Scheduler routes template-based tasks through workflow engine
- [ ] Free-text dispatch (no TemplateID) still works identically

### Must Have
- RoomID propagation from triggering context through to agent spawn
- Full `OrchestratorIntegration` dependency chain constructed in `main.go`
- Template-based routing in scheduler `dispatchTask()`
- DefinitionID guard updated: `if DefinitionID == "" && TemplateID == "" { deactivate }`
- `CreateTemplate` INSERT bug fixed
- All existing tests still pass

### Must NOT Have (Guardrails)
- No LLM classifier in dispatch path — agent responsibility, not Bridge
- No modifications to `StepExecutor`, `DependencyValidator`, `ApprovalEngine`, `NotificationService` internals
- No new Matrix commands for workflow execution
- No `secretary.RPCHandler` registration on main RPC server (deferred — no room_id in RPC context)
- No fix for `ListWorkflows` no-op filter (separate PR)
- No fix for warm dispatch Description=CronExpression (separate PR)
- No seed templates or template versioning
- No workflow editing UI
- No agent-side workflow event handling

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (`go test` with in-package mocks)
- **Automated tests**: Tests-after
- **Framework**: `go test` + `testify` assertions
- **Pattern**: In-package mocks, `setupTest*` helpers, table-driven tests

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Build verification**: `go build ./...` after every task
- **Test verification**: `go test ./pkg/secretary/...` after every task
- **Regression check**: All 38 existing Layer 1 tests must still pass

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation — schema, types, bug fixes):
├── Task 1: Add room_id to workflows schema + Workflow struct [quick]
├── Task 2: Fix CreateTemplate INSERT bug + add RoomID to SpawnRequestRef [quick]
└── Task 3: Fix executeStep RoomID bug (uses workflow.RoomID) [quick]

Wave 2 (Construction + Wiring — depends on Wave 1):
├── Task 4: Construct OrchestratorIntegration chain in main.go [deep]
├── Task 5: Wire OrchestratorIntegration into SecretaryCommandHandler [unspecified-high]
└── Task 6: Route scheduler tasks through workflow engine [unspecified-high]

Wave 3 (Polish):
├── Task 7: Update doc/armorclaw.md with workflow lifecycle [writing]
└── Task 8: Write ~15-20 tests for new code paths [deep]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay

Critical Path: Task 1 → Task 3 → Task 4 → Task 5 → Task 6 → Task 8 → F1-F4
Parallel Speedup: ~40% (Wave 1 fully parallel)
Max Concurrent: 3 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | — | 3, 4 | 1 |
| 2 | — | 4, 6 | 1 |
| 3 | 1 | 4 | 1 |
| 4 | 1, 2, 3 | 5, 6 | 2 |
| 5 | 4 | 8 | 2 |
| 6 | 4 | 7, 8 | 2 |
| 7 | 6 | — | 3 |
| 8 | 5, 6 | F1-F4 | 3 |

### Agent Dispatch Summary

- **Wave 1**: 3 tasks — T1→`quick`, T2→`quick`, T3→`quick`
- **Wave 2**: 3 tasks — T4→`deep`, T5→`unspecified-high`, T6→`unspecified-high`
- **Wave 3**: 2 tasks — T7→`writing`, T8→`deep`
- **FINAL**: 4 tasks — F1→`oracle`, F2→`unspecified-high`, F3→`unspecified-high`, F4→`deep`

---

## TODOs

> Implementation + Test = ONE Task. Never separate.
> EVERY task MUST have: Recommended Agent Profile + Parallelization info + QA Scenarios.

- [x] 1. Add `room_id` to workflows schema + Workflow struct

  **What to do**:
  - In `bridge/pkg/secretary/store.go`: Add `room_id TEXT DEFAULT ''` to the `workflows` CREATE TABLE DDL (after `created_by` at line ~168)
  - In `bridge/pkg/secretary/store.go`: Add migration for existing databases in `initSchema()`: `ALTER TABLE workflows ADD COLUMN room_id TEXT DEFAULT ''` wrapped with duplicate-column handling (follow existing `ALTER TABLE` pattern in store.go for `scheduled_tasks`)
  - In `bridge/pkg/secretary/store.go`: Update `CreateWorkflow()` INSERT to include `room_id` column and parameter
  - In `bridge/pkg/secretary/store.go`: Update `GetWorkflow()` and `ListWorkflows()` row.Scan() to include `room_id`
  - In `bridge/pkg/secretary/store.go`: Update `UpdateWorkflow()` to persist `room_id`
  - In `bridge/pkg/secretary/types.go`: Add `RoomID string` field to `Workflow` struct (after `CreatedBy`)

  **Must NOT do**:
  - Do not modify `task_templates`, `scheduled_tasks`, or any other table
  - Do not add a FK constraint on room_id (it's not a reference, just a string)
  - Do not change the `Factory` interface or `FactoryInterface`

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single struct field addition + DDL change + scan/insert updates. Well-defined pattern from Layer 1.
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3)
  - **Blocks**: Task 3 (executeStep needs Workflow.RoomID), Task 4 (construction chain)
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/store.go:156-170` — workflows CREATE TABLE DDL (add `room_id TEXT DEFAULT ''` after `created_by`)
  - `bridge/pkg/secretary/store.go:237-278` — Migration pattern: `ALTER TABLE` with duplicate-column handling (see how `scheduled_tasks` columns were added)
  - `bridge/pkg/secretary/store.go` — `CreateWorkflow()`, `GetWorkflow()`, `UpdateWorkflow()` — add `room_id` to INSERT, Scan, UPDATE

  **API/Type References**:
  - `bridge/pkg/secretary/types.go` — `Workflow` struct — add `RoomID string` after `CreatedBy`

  **Test References**:
  - `bridge/pkg/secretary/store_scheduled_tasks_test.go` — Pattern for testing schema migrations (ListDueTasks, MarkDispatched tests)

  **WHY Each Reference Matters**:
  - The DDL at lines 156-170 defines the exact columns to extend
  - The migration pattern at 237-278 shows how to handle `ALTER TABLE` for existing DBs safely
  - CreateWorkflow/GetWorkflow/UpdateWorkflow all need the new column — follow existing column pattern

  **Acceptance Criteria**:

  - [ ] `cd bridge && go build ./...` succeeds
  - [ ] `Workflow` struct has `RoomID string` field
  - [ ] `CREATE TABLE workflows` DDL includes `room_id TEXT DEFAULT ''`
  - [ ] `CreateWorkflow()` INSERT includes `room_id`
  - [ ] `GetWorkflow()` Scan includes `room_id`
  - [ ] Existing migration handles adding `room_id` to existing DBs

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Workflow CRUD with room_id
    Tool: Bash (go test)
    Preconditions: Clean build, no existing workflows
    Steps:
      1. cd bridge && go test ./pkg/secretary/... -run TestWorkflowRoomID -v -count=1
      2. Assert test passes with output containing "PASS"
    Expected Result: Workflow with RoomID="!room123:example.com" can be created, retrieved, and updated
    Failure Indicators: Build fails, test fails, RoomID not persisted
    Evidence: .sisyphus/evidence/task-1-workflow-roomid-crud.txt

  Scenario: Migration adds room_id to existing database
    Tool: Bash (go test)
    Preconditions: Database with workflows table but no room_id column
    Steps:
      1. cd bridge && go test ./pkg/secretary/... -run TestWorkflowMigration -v -count=1
      2. Assert migration runs without error
      3. Assert existing workflows have room_id="" after migration
    Expected Result: ALTER TABLE runs cleanly, existing workflows get empty room_id
    Failure Indicators: "duplicate column name" error, migration panics
    Evidence: .sisyphus/evidence/task-1-migration-existing-db.txt
  ```

  **Commit**: YES (groups with schema change)
  - Message: `fix(secretary): add room_id to workflows table and Workflow struct`
  - Files: `bridge/pkg/secretary/store.go`, `bridge/pkg/secretary/types.go`
  - Pre-commit: `cd bridge && go build ./...`

- [x] 2. Fix `CreateTemplate` INSERT bug + add `RoomID` to `SpawnRequestRef`

  **What to do**:
  - In `bridge/pkg/secretary/store.go:312`: Add 10th `?` placeholder to `CreateTemplate` INSERT VALUES. Currently: `VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)` (9). Should be: `VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)` (10). The 10 args are: `template.ID, template.Name, template.Description, string(stepsJSON), template.Variables, string(piiRefsJSON), template.CreatedBy, time.Now().UnixMilli(), time.Now().UnixMilli(), isActive`.
  - In `bridge/pkg/secretary/task_scheduler.go:35-39`: Add `RoomID string` field to `SpawnRequestRef` struct (after `UserID`).
  - In `bridge/cmd/bridge/main.go:3566-3571`: Update `studioFactoryAdapter.Spawn()` to pass `RoomID` from `SpawnRequestRef` to `studio.SpawnRequest`.

  **Must NOT do**:
  - Do not change the `FactoryInterface` or `Factory` interface definitions
  - Do not change `studio.SpawnRequest` (it already has `RoomID` from Layer 1)
  - Do not change column order in the INSERT statement

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single character fix (add `?`), single field addition, single adapter update. Under 10 lines total.
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3)
  - **Blocks**: Task 4 (construction chain uses StepExecutor which depends on templates), Task 6 (routing needs SpawnRequestRef.RoomID)
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/store.go:310-313` — The buggy INSERT: 10 column names, 9 `?` placeholders, 10 args. Add one `?` to make it 10/10/10.
  - `bridge/pkg/secretary/task_scheduler.go:35-39` — `SpawnRequestRef` struct: add `RoomID string` after `UserID`
  - `bridge/cmd/bridge/main.go:3566-3571` — `studioFactoryAdapter.Spawn()`: add `RoomID: req.RoomID` to the `studio.SpawnRequest` literal

  **API/Type References**:
  - `bridge/pkg/studio/factory.go` — `SpawnRequest` already has `RoomID string` (added in Layer 1)

  **WHY Each Reference Matters**:
  - The INSERT bug at 310-313 is a runtime panic — every CreateTemplate call fails
  - SpawnRequestRef.RoomID is needed so cold dispatch can pass room context to the factory
  - The adapter at 3566-3571 is the bridge between SpawnRequestRef and studio.SpawnRequest

  **Acceptance Criteria**:

  - [ ] `cd bridge && go build ./...` succeeds
  - [ ] `CreateTemplate` INSERT has 10 `?` placeholders matching 10 columns
  - [ ] `SpawnRequestRef` has `RoomID string` field
  - [ ] `studioFactoryAdapter.Spawn()` passes `RoomID` through

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: CreateTemplate INSERT works without panic
    Tool: Bash (go test)
    Preconditions: Clean build
    Steps:
      1. cd bridge && go test ./pkg/secretary/... -run TestCreateTemplate -v -count=1
      2. Assert test passes — template is created and retrievable
    Expected Result: Template created with all fields persisted, no SQL error
    Failure Indicators: "5 values for 10 columns" or similar SQL error
    Evidence: .sisyphus/evidence/task-2-create-template-fix.txt

  Scenario: SpawnRequestRef RoomID propagates to studio.SpawnRequest
    Tool: Bash (go test)
    Preconditions: SpawnRequestRef has RoomID field
    Steps:
      1. cd bridge && go test ./pkg/secretary/... -run TestSpawnRequestRefRoomID -v -count=1
      2. Assert adapter passes RoomID through
    Expected Result: studio.SpawnRequest receives RoomID from SpawnRequestRef
    Failure Indicators: RoomID lost in adapter, empty string when it should be set
    Evidence: .sisyphus/evidence/task-2-spawn-request-roomid.txt
  ```

  **Commit**: YES
  - Message: `fix(secretary): fix CreateTemplate INSERT placeholder count and add RoomID to SpawnRequestRef`
  - Files: `bridge/pkg/secretary/store.go`, `bridge/pkg/secretary/task_scheduler.go`, `bridge/cmd/bridge/main.go`
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/secretary/... -count=1`

- [x] 3. Fix `executeStep` RoomID bug — use `workflow.RoomID`

  **What to do**:
  - In `bridge/pkg/secretary/orchestrator_integration.go:245`: Change `RoomID: workflow.CreatedBy` to `RoomID: workflow.RoomID`
  - This depends on Task 1 being complete (Workflow struct needs RoomID field)

  **Must NOT do**:
  - Do not change `executeStep()` logic beyond the RoomID fix
  - Do not modify `StepExecutor`, `DependencyValidator`, or other internals
  - Do not add room_id parameter to `executeStep()` — it already receives `workflow *Workflow` which will have `RoomID`

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single line change. Replace `CreatedBy` with `RoomID` on one line.
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (technically, but depends on Task 1's Workflow struct change)
  - **Parallel Group**: Wave 1 (with Tasks 1, 2) — can start as soon as Task 1 is complete
  - **Blocks**: Task 4 (construction chain needs correct RoomID behavior)
  - **Blocked By**: Task 1 (needs Workflow.RoomID field)

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:228-259` — `executeStep()` method. Line 245: `RoomID: workflow.CreatedBy` — this is the bug. Change to `RoomID: workflow.RoomID`.

  **API/Type References**:
  - `bridge/pkg/secretary/types.go` — `Workflow` struct (after Task 1: has `RoomID string`)

  **WHY Each Reference Matters**:
  - Line 245 is the single source of Bug A. `CreatedBy` is a Matrix user ID like `@user:example.com`, not a room ID. Matrix `SendEvent()` will fail silently with this value.

  **Acceptance Criteria**:

  - [ ] `cd bridge && go build ./...` succeeds
  - [ ] `orchestrator_integration.go:245` reads `RoomID: workflow.RoomID` (not `CreatedBy`)
  - [ ] `grep -n "workflow.CreatedBy" bridge/pkg/secretary/orchestrator_integration.go` returns no matches on the RoomID line

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: executeStep uses workflow.RoomID for spawn
    Tool: Bash (go test + grep)
    Preconditions: Task 1 complete (Workflow has RoomID)
    Steps:
      1. grep -n "RoomID.*workflow\." bridge/pkg/secretary/orchestrator_integration.go
      2. Assert line 245 shows RoomID: workflow.RoomID
      3. cd bridge && go test ./pkg/secretary/... -run TestExecuteStepRoomID -v -count=1
    Expected Result: Agent spawned with workflow.RoomID, not workflow.CreatedBy
    Failure Indicators: Still references CreatedBy, test fails
    Evidence: .sisyphus/evidence/task-3-execute-step-roomid.txt

  Scenario: Empty RoomID still works (fire-and-forget)
    Tool: Bash (go test)
    Preconditions: Workflow with RoomID=""
    Steps:
      1. cd bridge && go test ./pkg/secretary/... -run TestExecuteStepEmptyRoomID -v -count=1
      2. Assert spawn succeeds with empty RoomID (no crash)
    Expected Result: Agent spawned successfully, RoomID is empty string
    Failure Indicators: Panic, error on empty RoomID
    Evidence: .sisyphus/evidence/task-3-empty-roomid.txt
  ```

  **Commit**: YES
  - Message: `fix(secretary): use workflow.RoomID instead of CreatedBy in executeStep`
  - Files: `bridge/pkg/secretary/orchestrator_integration.go`
  - Pre-commit: `cd bridge && go build ./...`

- [x] 4. Construct `OrchestratorIntegration` dependency chain in `main.go`

  **What to do**:
  - In `bridge/cmd/bridge/main.go`: After the task scheduler construction (line ~2638), construct the full `OrchestratorIntegration` dependency chain:

  ```
  Construction order (leaves → root):
  1. WorkflowEventEmitter — needs: events.MatrixEventBus (already exists as `eventBus` at line 2665)
  2. WorkflowOrchestratorImpl — needs: Store (rolodexStore), Factory (studioFactoryAdapter or direct), EventEmitter
  3. DependencyValidator — needs: nothing (no-arg constructor)
  4. StepExecutor — needs: *studio.AgentFactory (studioService.GetFactory()), *DependencyValidator, ApprovalChecker (nil for v1 or construct ApprovalEngineImpl), timeouts
  5. NotificationService — needs: Store (rolodexStore), Logger
  6. OrchestratorIntegration — needs: all of the above via IntegrationConfig
  ```

  - Store the constructed `OrchestratorIntegration` in a local variable accessible to both `SecretaryCommandHandler` and `TaskScheduler` wiring.
  - Also construct `ApprovalEngineImpl` since `StepExecutor` needs an `ApprovalChecker` — or pass nil and let `StepExecutor` skip approval checks (verify if StepExecutor handles nil ApprovalChecker gracefully). Check the code first.

  **Must NOT do**:
  - Do not modify StepExecutor, DependencyValidator, ApprovalEngine, or NotificationService internals
  - Do not register `secretary.RPCHandler` on the main RPC server (deferred)
  - Do not remove the existing task scheduler construction — add alongside it
  - Do not change the `studioFactoryAdapter` — it serves the scheduler, not the orchestrator

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Requires understanding a 6-component dependency graph, cross-package type compatibility (`*studio.AgentFactory` vs `secretary.Factory`), and inserting into a 3600-line main.go. Multiple compilation pitfalls.
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (first task of wave, others depend on it)
  - **Blocks**: Task 5, Task 6
  - **Blocked By**: Task 1 (Workflow.RoomID), Task 2 (CreateTemplate fix), Task 3 (executeStep RoomID fix)

  **References**:

  **Pattern References**:
  - `bridge/cmd/bridge/main.go:2596-2637` — Current wiring: SecretaryCommandHandler (nil Store/Orchestrator) + TaskScheduler construction. Insert construction after line 2638.
  - `bridge/cmd/bridge/main.go:3546-3573` — `studioFactoryAdapter` bridges `studio.AgentFactory` → `secretary.FactoryInterface`
  - `bridge/cmd/bridge/main.go:2665` — `EventBus: eventBus` — already exists, passed to RPC config

  **API/Type References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:371-399` — `OrchestratorIntegration` struct + `IntegrationConfig` + constructor
  - `bridge/pkg/secretary/orchestrator_integration.go:52-59` — `StepExecutorConfig`: needs `*studio.AgentFactory`, `*DependencyValidator`, `ApprovalChecker`
  - `bridge/pkg/secretary/orchestrator.go:34-68` — `OrchestratorConfig`: needs `Store`, `EventEmitter`, optional `Factory`
  - `bridge/pkg/secretary/orchestrator_events.go:59-64` — `NewWorkflowEventEmitter(bus *events.MatrixEventBus)`
  - `bridge/pkg/secretary/orchestrator_dependencies.go:51` — `NewDependencyValidator()` (no args)
  - `bridge/pkg/secretary/approvals.go:97-104` — `ApprovalEngineConfig`: needs `Store`, `Logger`
  - `bridge/pkg/secretary/notifications.go:79-96` — `NotificationServiceConfig`: needs `Store`, `Logger`

  **External References**:
  - `bridge/pkg/studio/factory.go:35` — `AgentFactory` struct — `studioService.GetFactory()` returns `*studio.AgentFactory`
  - `bridge/pkg/events/` — `MatrixEventBus` type used by `WorkflowEventEmitter`

  **WHY Each Reference Matters**:
  - The construction chain must follow dependency order — each reference shows what inputs each constructor needs
  - `StepExecutorConfig.Factory` takes `*studio.AgentFactory` directly (not an interface), so we can use `studioService.GetFactory()` directly
  - `OrchestratorConfig.Factory` takes `secretary.Factory` interface — may need the existing `studioFactoryAdapter` or a new adapter
  - `ApprovalChecker` is an interface — we can construct `ApprovalEngineImpl` or pass nil (check if StepExecutor handles nil)

  **Acceptance Criteria**:

  - [ ] `cd bridge && go build ./...` succeeds
  - [ ] `OrchestratorIntegration` constructed with non-nil Store, non-nil Orchestrator, non-nil Executor
  - [ ] Variable accessible for Task 5 (handler wiring) and Task 6 (scheduler routing)
  - [ ] `SecretaryCommandHandler` nil Store and nil Orchestrator are replaced with constructed instances
  - [ ] `go test ./pkg/secretary/... -count=1` — all existing tests pass

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Full dependency chain compiles and constructs
    Tool: Bash
    Preconditions: Tasks 1-3 complete
    Steps:
      1. cd bridge && go build ./...
      2. Assert exit code 0, no compilation errors
      3. cd bridge && go vet ./cmd/bridge/...
      4. Assert no vet issues
    Expected Result: Build succeeds, all types satisfy their interfaces
    Failure Indicators: "does not implement" errors, nil pointer warnings, type mismatch
    Evidence: .sisyphus/evidence/task-4-construction-chain.txt

  Scenario: Existing tests still pass after construction
    Tool: Bash (go test)
    Preconditions: Construction chain added to main.go
    Steps:
      1. cd bridge && go test ./pkg/secretary/... -count=1
      2. Assert all 38 existing tests pass
    Expected Result: PASS — no regressions from construction changes
    Failure Indicators: Any test failure, compilation error
    Evidence: .sisyphus/evidence/task-4-regression-test.txt

  Scenario: SecretaryCommandHandler has non-nil Store and Orchestrator
    Tool: Bash (grep)
    Preconditions: main.go updated
    Steps:
      1. grep -A 15 "NewSecretaryCommandHandler" bridge/cmd/bridge/main.go
      2. Assert Store: field is non-nil (references rolodexStore or constructed store)
      3. Assert Orchestrator: field is non-nil (references constructed orchestrator)
    Expected Result: Both fields are wired to constructed instances, not nil
    Failure Indicators: Store: nil or Orchestrator: nil still present
    Evidence: .sisyphus/evidence/task-4-handler-wiring.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): construct OrchestratorIntegration dependency chain in main.go`
  - Files: `bridge/cmd/bridge/main.go`
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/secretary/... -count=1`

- [x] 5. Wire `OrchestratorIntegration` into `SecretaryCommandHandler`

  **What to do**:
  - In `bridge/cmd/bridge/main.go`: Update `SecretaryCommandHandler` construction (line ~2596) to include:
    - `Store: rolodexStore` (was `nil`)
    - `Orchestrator: orchestratorIntegration` (was `nil` — the `OrchestratorIntegration` constructed in Task 4)
  - In `bridge/pkg/secretary/secretary_commands.go:426` (or wherever `handleStartWorkflow` is): After `orchestrator.StartWorkflow(id)`, call `integration.StartWorkflowExecution(id)`. This requires `OrchestratorIntegration` to be accessible — either:
    - Add `WorkflowExecution IntegrationExecutor` field to `SecretaryCommandHandler` (new interface or use `*OrchestratorIntegration`), OR
    - Use the existing `Orchestrator` field if its interface includes `StartWorkflowExecution`
  - Check what type `SecretaryCommandHandler.Orchestrator` is — it may need to be widened to include execution methods, or a new field added.
  - In `bridge/pkg/secretary/secretary_commands.go`: For `handleStartWorkflow`, pass `room_id` from the Matrix room context to the workflow. The handler receives roomID as a parameter — persist it in the workflow's RoomID field before calling `StartWorkflow`.

  **Must NOT do**:
  - Do not modify `StepExecutor`, `DependencyValidator`, `ApprovalEngine`, `NotificationService`
  - Do not add new Matrix commands
  - Do not register RPC handlers

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Requires understanding `SecretaryCommandHandler` type definitions, interface compatibility, and the Matrix command dispatch flow. Moderate complexity in finding the right field to add and threading room_id.
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 4 construction)
  - **Parallel Group**: Wave 2 (after Task 4)
  - **Blocks**: Task 8 (tests)
  - **Blocked By**: Task 4 (needs constructed OrchestratorIntegration)

  **References**:

  **Pattern References**:
  - `bridge/cmd/bridge/main.go:2596-2607` — `SecretaryCommandHandler` construction with nil Store/Orchestrator
  - `bridge/pkg/secretary/secretary_commands.go:426` — `handleStartWorkflow` — where to add `StartWorkflowExecution()` call

  **API/Type References**:
  - `bridge/pkg/secretary/secretary_commands.go` — `SecretaryCommandHandler` struct definition — check Orchestrator field type
  - `bridge/pkg/secretary/secretary_commands.go` — `SecretaryCommandHandlerConfig` — config struct for construction
  - `bridge/pkg/secretary/orchestrator_integration.go:401` — `StartWorkflowExecution(workflowID string) error`

  **WHY Each Reference Matters**:
  - The handler config needs to be extended to accept the integration instance
  - `handleStartWorkflow` is where execution actually starts — the existing code only calls `StartWorkflow()` which sets status but doesn't execute steps
  - Room context comes from the Matrix message handler — need to trace where roomID is available in `handleStartWorkflow`

  **Acceptance Criteria**:

  - [ ] `cd bridge && go build ./...` succeeds
  - [ ] `SecretaryCommandHandler.Store` is non-nil (rolodexStore)
  - [ ] `SecretaryCommandHandler` has access to `OrchestratorIntegration` (via new field or widened interface)
  - [ ] `handleStartWorkflow` calls both `StartWorkflow()` AND `StartWorkflowExecution()`
  - [ ] Room ID from Matrix context is persisted to `workflow.RoomID` before execution starts
  - [ ] `go test ./pkg/secretary/... -count=1` passes

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: handleStartWorkflow calls StartWorkflowExecution
    Tool: Bash (grep + go test)
    Preconditions: Task 4 complete
    Steps:
      1. grep -n "StartWorkflowExecution" bridge/pkg/secretary/secretary_commands.go
      2. Assert at least one match in handleStartWorkflow
      3. cd bridge && go test ./pkg/secretary/... -count=1
    Expected Result: StartWorkflowExecution called after StartWorkflow in the command handler
    Failure Indicators: No call to StartWorkflowExecution, or call before StartWorkflow
    Evidence: .sisyphus/evidence/task-5-handler-execution-call.txt

  Scenario: Room ID persisted to workflow before execution
    Tool: Bash (go test)
    Preconditions: Task 1 complete (Workflow.RoomID exists)
    Steps:
      1. cd bridge && go test ./pkg/secretary/... -run TestStartWorkflowRoomID -v -count=1
      2. Assert workflow.RoomID is set from Matrix room context
    Expected Result: Workflow created with correct RoomID from triggering room
    Failure Indicators: RoomID is empty when it should be set
    Evidence: .sisyphus/evidence/task-5-room-id-persist.txt

  Scenario: SecretaryCommandHandler non-nil Store and Orchestrator
    Tool: Bash (grep)
    Preconditions: main.go updated
    Steps:
      1. grep -A 15 "NewSecretaryCommandHandler" bridge/cmd/bridge/main.go
      2. Assert no "nil" values for Store or Orchestrator fields
    Expected Result: All handler dependencies wired to real instances
    Failure Indicators: Store: nil, Orchestrator: nil, or Orchestrator: nil
    Evidence: .sisyphus/evidence/task-5-handler-deps.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): wire OrchestratorIntegration into SecretaryCommandHandler`
  - Files: `bridge/cmd/bridge/main.go`, `bridge/pkg/secretary/secretary_commands.go`
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/secretary/... -count=1`

- [x] 6. Route scheduler tasks through workflow engine when `TemplateID != ""`

  **What to do**:
  - In `bridge/pkg/secretary/task_scheduler.go:134-160` (`dispatchTask`):
    - Update the DefinitionID guard at line 136: change `if strings.TrimSpace(task.DefinitionID) == ""` to `if strings.TrimSpace(task.DefinitionID) == "" && strings.TrimSpace(task.TemplateID) == ""` — template-based tasks don't need a DefinitionID
    - Before the existing warm/cold dispatch logic, add a template-based dispatch branch:
      ```
      if task.TemplateID != "" {
          ts.templateDispatch(ctx, task)
          return
      }
      ```
  - Add new method `templateDispatch(ctx context.Context, task ScheduledTask)`:
    1. Get template from store: `ts.store.GetTemplate(ctx, task.TemplateID)`
    2. Create workflow from template: Use `CreateWorkflowFromTemplate` pattern (check `bridge/pkg/secretary/studio_integration.go` or implement inline)
    3. Set workflow RoomID from task context (may be empty for scheduler-triggered — acceptable)
    4. Call `ts.orchestrator.StartWorkflow(workflow.ID)` — requires adding `orchestrator` field to `TaskScheduler`
    5. Call `ts.integration.StartWorkflowExecution(workflow.ID)` — requires adding `integration` field to `TaskScheduler`
    6. On success: `ts.updateAfterDispatch(ctx, task)`
    7. On error: log and return (don't deactivate — the task will be retried on next tick)
  - In `bridge/pkg/secretary/task_scheduler.go`: Update `TaskScheduler` struct to include `orchestrator WorkflowOrchestrator` and `integration IntegrationExecutor` fields (define minimal interface or use concrete types)
  - In `bridge/pkg/secretary/task_scheduler.go`: Update `NewTaskScheduler()` to accept the new dependencies
  - In `bridge/pkg/secretary/task_dispatch.go`: Add `WorkflowID string` to `TaskDispatchPayload` for traceability
  - In `bridge/cmd/bridge/main.go:2629`: Update `NewTaskScheduler` call to pass orchestrator and integration

  **Must NOT do**:
  - Do not remove or change the existing warm/cold dispatch fallback (when TemplateID == "")
  - Do not build an LLM template resolver
  - Do not add `template_id` as required parameter on `task.create` RPC (that's a validation change, not routing)
  - Do not modify the task store or scheduled_tasks schema

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Requires modifying the scheduler dispatch flow, adding a new dispatch method, creating a minimal interface, and updating the construction chain. Multiple files touched with interdependencies.
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 4)
  - **Parallel Group**: Wave 2 (after Task 4, parallel with Task 5)
  - **Blocks**: Task 7 (doc), Task 8 (tests)
  - **Blocked By**: Task 4 (needs OrchestratorIntegration instance)

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/task_scheduler.go:134-160` — `dispatchTask()` method — add template branch before warm/cold
  - `bridge/pkg/secretary/task_scheduler.go:162-189` — `warmDispatch()` and `coldDispatch()` — follow same pattern for `templateDispatch()`
  - `bridge/pkg/secretary/task_scheduler.go:52-64` — `TaskScheduler` struct and constructor — add new fields

  **API/Type References**:
  - `bridge/pkg/secretary/studio_integration.go` — `CreateWorkflowFromTemplate` pattern (if it exists) — check for reuse
  - `bridge/pkg/secretary/orchestrator_integration.go:401` — `StartWorkflowExecution(workflowID string) error`
  - `bridge/pkg/secretary/orchestrator.go:70` — `StartWorkflow(workflowID string) error`
  - `bridge/pkg/secretary/task_dispatch.go` — `TaskDispatchPayload` — add `WorkflowID string`
  - `bridge/cmd/bridge/main.go:2629` — `NewTaskScheduler` call site — add new params

  **Test References**:
  - `bridge/pkg/secretary/task_scheduler_test.go` — Existing dispatch tests — follow pattern for template dispatch tests

  **WHY Each Reference Matters**:
  - `dispatchTask()` at 134-160 is the core routing logic — template dispatch must be checked before warm/cold
  - `CreateWorkflowFromTemplate` in studio_integration.go may already handle template→workflow creation
  - The scheduler struct needs both orchestrator (for StartWorkflow) and integration (for StartWorkflowExecution)
  - TaskScheduler constructor call at main.go:2629 needs updating

  **Acceptance Criteria**:

  - [ ] `cd bridge && go build ./...` succeeds
  - [ ] `dispatchTask()` checks TemplateID first, routes to `templateDispatch()` when set
  - [ ] `templateDispatch()` creates workflow from template, starts execution
  - [ ] DefinitionID guard allows tasks with TemplateID but no DefinitionID
  - [ ] `TaskDispatchPayload` has `WorkflowID string` field
  - [ ] Free-text dispatch (TemplateID == "") still works identically
  - [ ] `go test ./pkg/secretary/... -count=1` passes

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Template-based task routes through workflow engine
    Tool: Bash (go test)
    Preconditions: Task 4-5 complete, template exists in test DB
    Steps:
      1. cd bridge && go test ./pkg/secretary/... -run TestTemplateDispatch -v -count=1
      2. Assert: task with TemplateID creates workflow, calls StartWorkflowExecution
      3. Assert: task is marked dispatched after workflow starts
    Expected Result: Workflow created from template, execution started, task dispatched
    Failure Indicators: Falls through to warm/cold dispatch, workflow not created, panic
    Evidence: .sisyphus/evidence/task-6-template-dispatch.txt

  Scenario: Free-text dispatch still works (no TemplateID)
    Tool: Bash (go test)
    Preconditions: Task has empty TemplateID
    Steps:
      1. cd bridge && go test ./pkg/secretary/... -run TestDispatchTask_NoTemplate -v -count=1
      2. Assert: task dispatches via warm/cold path (same as before)
    Expected Result: Existing dispatch behavior unchanged for non-template tasks
    Failure Indicators: Regression in warm/cold dispatch, test failure
    Evidence: .sisyphus/evidence/task-6-free-text-dispatch.txt

  Scenario: Template not found — graceful error
    Tool: Bash (go test)
    Preconditions: Task references non-existent TemplateID
    Steps:
      1. cd bridge && go test ./pkg/secretary/... -run TestTemplateDispatch_NotFound -v -count=1
      2. Assert: error logged, task NOT deactivated (will retry)
    Expected Result: Graceful error, task remains active for retry
    Failure Indicators: Task deactivated, panic, crash
    Evidence: .sisyphus/evidence/task-6-template-not-found.txt

  Scenario: DefinitionID empty but TemplateID set — task NOT deactivated
    Tool: Bash (go test)
    Preconditions: Task has DefinitionID="" but TemplateID="tmpl_123"
    Steps:
      1. cd bridge && go test ./pkg/secretary/... -run TestDispatchTask_TemplateOnlyNoDef -v -count=1
      2. Assert: task routes to templateDispatch, not deactivated
    Expected Result: Template dispatch path taken, task dispatched
    Failure Indicators: Task deactivated by DefinitionID guard
    Evidence: .sisyphus/evidence/task-6-no-def-with-template.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): route template-based scheduled tasks through workflow engine`
  - Files: `bridge/pkg/secretary/task_scheduler.go`, `bridge/pkg/secretary/task_dispatch.go`, `bridge/cmd/bridge/main.go`
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/secretary/... -count=1`

- [x] 7. Update `doc/armorclaw.md` with workflow execution lifecycle

  **What to do**:
  - In `doc/armorclaw.md` §8 (custom events table): Add workflow execution events:
    - `app.armorclaw.workflow_step_progress` — step progress during execution
    - `app.armorclaw.workflow_completed` — workflow finished successfully
    - `app.armorclaw.workflow_failed` — workflow failed with error
  - In `doc/armorclaw.md` Context Routing Rules: Add: "Execute or modify workflow steps → `bridge/pkg/secretary/orchestrator_integration.go` and `bridge/pkg/secretary/step_executor.go`"
  - In `doc/armorclaw.md` §4 (RPC methods): Update note — no new RPC methods, just wiring existing ones (no count change needed)
  - In `doc/armorclaw.md` new subsection (after scheduler section or §18): Document the workflow execution lifecycle:
    ```
    Template → CreateWorkflowFromTemplate → StartWorkflow (sets status=Running)
      → StartWorkflowExecution (spawns goroutine)
        → StepExecutor.ExecuteSteps()
          → For each step: DependencyValidator → ApprovalChecker → factory.Spawn(agent)
          → waitForCompletion (500ms polling)
          → On complete: AdvanceWorkflow
          → On fail: FailWorkflow (if recoverable, retry)
        → On all steps complete: CompleteWorkflow
    ```
  - Document the two dispatch paths for scheduled tasks:
    - `TemplateID != ""` → workflow engine (template-based)
    - `TemplateID == ""` → warm/cold dispatch (free-text)
  - Document room_id semantics: workflows store triggering room ID, empty = fire-and-forget

  **Must NOT do**:
  - Do not add architecture diagrams (text-only is fine)
  - Do not document planned features, only what's implemented
  - Do not change the existing doc structure beyond the additions listed above

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Documentation-only task. Requires understanding the execution flow to describe it accurately.
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 8)
  - **Parallel Group**: Wave 3 (with Task 8)
  - **Blocks**: None
  - **Blocked By**: Task 6 (needs actual routing logic to document)

  **References**:

  **Pattern References**:
  - `doc/armorclaw.md` — Existing documentation structure, section numbering, event table format

  **API/Type References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:401-430` — `StartWorkflowExecution()` flow
  - `bridge/pkg/secretary/orchestrator_integration.go:431-518` — `runWorkflow()`, `executeSteps()`, `waitForCompletion()`
  - `bridge/pkg/secretary/task_scheduler.go:134-160` — Dispatch routing logic

  **WHY Each Reference Matters**:
  - The doc must accurately reflect the actual execution flow in the code
  - The event types in orchestrator_events.go are the Matrix events emitted during execution

  **Acceptance Criteria**:

  - [ ] `doc/armorclaw.md` contains workflow execution lifecycle documentation
  - [ ] Custom events table includes workflow_step_progress, workflow_completed, workflow_failed
  - [ ] Context routing rules include orchestrator_integration.go
  - [ ] Two dispatch paths documented (template-based vs free-text)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Documentation completeness check
    Tool: Bash (grep)
    Preconditions: doc/armorclaw.md updated
    Steps:
      1. grep -c "workflow execution lifecycle" doc/armorclaw.md
      2. Assert count >= 1
      3. grep -c "workflow_step_progress" doc/armorclaw.md
      4. Assert count >= 1
      5. grep -c "orchestrator_integration.go" doc/armorclaw.md
      6. Assert count >= 1
    Expected Result: All documentation sections present with correct references
    Failure Indicators: Any grep returns 0
    Evidence: .sisyphus/evidence/task-7-doc-completeness.txt
  ```

  **Commit**: YES
  - Message: `docs(armorclaw): document workflow execution lifecycle`
  - Files: `doc/armorclaw.md`
  - Pre-commit: None (documentation only)

- [x] 8. Write ~15-20 tests for Layer 3 code paths

  **What to do**:
  - Create or extend test files for new code paths. Target test coverage:

  **In `bridge/pkg/secretary/orchestrator_integration_test.go` (new file, ~5-7 tests)**:
  - Test StartWorkflowExecution: workflow created from template, steps execute
  - Test StartWorkflowExecution_NotRunning: rejects non-running workflow
  - Test StartWorkflowExecution_AlreadyExecuting: rejects duplicate execution
  - Test executeStep Uses RoomID: verifies RoomID from workflow, not CreatedBy
  - Test CancelWorkflowExecution: cancels running workflow
  - Test GetExecutionStatus: returns correct status

  **In `bridge/pkg/secretary/task_scheduler_test.go` (extend, ~5-8 tests)**:
  - Test TemplateDispatch: task with TemplateID routes through workflow engine
  - Test TemplateDispatch_TemplateNotFound: graceful error, task not deactivated
  - Test TemplateDispatch_DefinitionIDEmptyButTemplateSet: not deactivated
  - Test FreeTextDispatch_Unchanged: tasks without TemplateID still use warm/cold
  - Test TemplateDispatch_WorkflowCreation: verifies workflow created from template
  - Test TemplateDispatch_RoomIDPropagation: verifies room_id set on workflow
  - Test TemplateDispatch_ErrorNotDeactivated: workflow creation error doesn't deactivate task

  **In `bridge/pkg/secretary/store_test.go` or `store_scheduled_tasks_test.go` (extend, ~3-4 tests)**:
  - Test WorkflowRoomID_CRUD: create, read, update workflow with RoomID
  - Test CreateTemplate_Fixed: template creation works (10 cols / 10 placeholders)
  - Test WorkflowMigration_AddRoomID: existing DB gets room_id column

  **In `bridge/pkg/secretary/rpc_test.go` (extend, ~2 tests)**:
  - Test task.create_with_TemplateID: template_id stored correctly
  - Test task.create_without_TemplateID: works as before (backward compat)

  Follow existing test patterns: in-package mocks, `testify` assertions, `setupTest*` helpers.

  **Must NOT do**:
  - Do not create a shared mock package
  - Do not modify test helpers from other packages
  - Do not skip tests that require root (use mocks instead)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Writing ~15-20 tests across 4+ test files with proper mocking of complex dependency chains (Store, Factory, EventEmitter, ApprovalChecker). Requires understanding the full execution flow to write meaningful assertions.
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 7)
  - **Parallel Group**: Wave 3 (with Task 7)
  - **Blocks**: F1-F4 (verification needs tests to pass)
  - **Blocked By**: Task 5, Task 6 (tests verify their code)

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/task_scheduler_test.go` — Existing scheduler tests: mock store, mock factory, mock matrix. Follow this pattern.
  - `bridge/pkg/secretary/rpc_test.go` — Existing RPC tests: mock dependencies, table-driven tests. Follow this pattern.
  - `bridge/pkg/secretary/store_scheduled_tasks_test.go` — Existing store tests: in-memory DB, CRUD assertions. Follow this pattern.
  - `bridge/pkg/secretary/orchestrator_test.go` — Existing orchestrator tests: `setupTestOrchestrator()` helper, mock store. Follow this pattern.

  **API/Type References**:
  - `bridge/pkg/secretary/orchestrator_integration.go` — Public methods to test: `StartWorkflowExecution`, `CancelWorkflowExecution`, `GetExecutionStatus`
  - `bridge/pkg/secretary/task_scheduler.go` — `dispatchTask()` and `templateDispatch()` to test

  **Test References**:
  - `bridge/pkg/secretary/task_scheduler_test.go:1-50` — Test setup pattern: mockStore, mockFactory, mockMatrix
  - `bridge/pkg/secretary/orchestrator_test.go:18` — `orchestratorTestStore` mock pattern
  - `bridge/pkg/secretary/orchestrator_test.go:305` — `setupTestOrchestrator()` helper pattern

  **WHY Each Reference Matters**:
  - Existing test patterns show exactly how to mock the secretary store and factory
  - The `setupTest*` helpers reduce boilerplate and keep tests consistent
  - Table-driven tests are the project convention for multi-scenario coverage

  **Acceptance Criteria**:

  - [ ] `cd bridge && go test ./pkg/secretary/... -count=1` — all tests pass (existing + new)
  - [ ] ~15-20 new tests added across 4+ test files
  - [ ] No tests skipped (all use mocks, no root required)
  - [ ] At least 1 negative test per new test file (error/edge case)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Full test suite passes with new tests
    Tool: Bash (go test)
    Preconditions: All implementation tasks complete
    Steps:
      1. cd bridge && go test ./pkg/secretary/... -v -count=1 2>&1 | tee /tmp/test-output.txt
      2. Assert: 0 failures
      3. Count new tests: grep -c "=== RUN.*Test.*Template\|=== RUN.*Test.*Workflow\|=== RUN.*Test.*RoomID\|=== RUN.*Test.*Execution" /tmp/test-output.txt
      4. Assert: count >= 15
    Expected Result: All tests pass, 15-20 new tests present
    Failure Indicators: Any test failure, fewer than 15 new tests
    Evidence: .sisyphus/evidence/task-8-test-suite.txt

  Scenario: Existing Layer 1 tests not broken
    Tool: Bash (go test)
    Preconditions: New tests added
    Steps:
      1. cd bridge && go test ./pkg/secretary/... -run "TestTaskScheduler|TestRPC|TestStore|TestDispatch" -count=1
      2. Assert: all existing tests pass
    Expected Result: 38 Layer 1 tests still pass
    Failure Indicators: Any regression in existing tests
    Evidence: .sisyphus/evidence/task-8-regression.txt
  ```

  **Commit**: YES
  - Message: `test(secretary): add Layer 3 workflow execution tests`
  - Files: `bridge/pkg/secretary/orchestrator_integration_test.go`, `bridge/pkg/secretary/task_scheduler_test.go`, `bridge/pkg/secretary/store_scheduled_tasks_test.go`, `bridge/pkg/secretary/rpc_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/secretary/... -count=1`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `cd bridge && go build ./...` + `go vet ./pkg/secretary/...` + `go test ./pkg/secretary/...`. Review all changed files for: `as any`/`@ts-ignore` equivalents (Go: `interface{}` abuse, empty catches, `fmt.Println` in prod, commented-out code, unused imports). Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (features working together, not isolation). Test edge cases: empty state, invalid input, rapid actions. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Wave 1**: 2-3 commits (schema+types, bug fixes, executeStep fix)
  - `fix(secretary): add room_id to workflows table and Workflow struct`
  - `fix(secretary): fix CreateTemplate INSERT placeholder count and add RoomID to SpawnRequestRef`
  - `fix(secretary): use workflow.RoomID instead of CreatedBy in executeStep`
- **Wave 2**: 2-3 commits (construction, wiring, routing)
  - `feat(bridge): construct OrchestratorIntegration dependency chain in main.go`
  - `feat(secretary): wire OrchestratorIntegration into SecretaryCommandHandler`
  - `feat(secretary): route template-based scheduled tasks through workflow engine`
- **Wave 3**: 2 commits (docs, tests)
  - `docs(armorclaw): document workflow execution lifecycle`
  - `test(secretary): add Layer 3 workflow execution tests`

---

## Success Criteria

### Verification Commands
```bash
cd bridge && go build ./...                                    # Expected: success (exit 0)
cd bridge && go test ./pkg/secretary/... -count=1              # Expected: PASS (all tests)
cd bridge && go vet ./pkg/secretary/...                        # Expected: no issues
grep -n "workflow.RoomID" bridge/pkg/secretary/orchestrator_integration.go  # Expected: RoomID used
grep -c "NewOrchestratorIntegration" bridge/cmd/bridge/main.go              # Expected: ≥1
grep -c "workflow engine" doc/armorclaw.md                                   # Expected: ≥1
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All existing 38 Layer 1 tests still pass
- [ ] ~15-20 new tests pass
- [ ] `go build ./...` succeeds
- [ ] `go vet ./pkg/secretary/...` clean
