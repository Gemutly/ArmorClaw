# Layer 3.1: Step Config Passthrough + PII Approval Blocking

## TL;DR

> **Quick Summary**: Add structured Config passthrough to spawned agent containers (`STEP_CONFIG` env var) and wire PII approval blocking into workflow execution so steps requiring approval emit Matrix events and wait for phone-based approval instead of immediately erroring.
> 
> **Deliverables**:
> - `STEP_CONFIG` env var in agent containers when step has structured config
> - `PendingApproval()` function that emits PII request events and blocks for response
> - Multi-agent execution behavior documented as unimplemented/deferred
> 
> **Estimated Effort**: Quick
> **Parallel Execution**: YES - 2 waves
> **Critical Path**: T1 → (independent) → T4 (tests) → FINAL

---

## Context

### Original Request
User specified three targeted fixes for the workflow execution engine: Config passthrough, PII approval blocking, and multi-agent documentation. Minimal scope — "three tasks, one commit."

### Interview Summary
**Key Discussions**:
- Layer 3 (SOPGate) is COMPLETE (commit 6849e55, 186 tests pass, build clean)
- Investigation identified three architectural gaps; user decided to fix Config passthrough and PII approval, defer skill invocation and parallel steps
- Tests-after strategy (same as Layer 3)
- Explicit exclusions: no schema changes, no new RPC methods, no ListWorkflows fix, no warm dispatch fix

### Metis Review
**Critical Findings** (all addressed in plan):
- `buildEnvironment()` takes `(*AgentDefinition, string)`, NOT `SpawnRequest` — must append `STEP_CONFIG` in `Spawn()` instead
- `SpawnRequestRef` mirror struct in `task_scheduler.go` — decided NOT to mirror (Config only for workflow paths)
- `StepExecutor` has no `EventEmitter`/`MatrixEventBus` — must inject via `StepExecutorConfig`
- No `PiiRef`/`PiiField` types exist — use `[]string` consistently
- No "Phase 3" section in doc/armorclaw.md — insert as subsection within Workflow Execution Lifecycle

---

## Work Objectives

### Core Objective
Wire two missing data paths into the workflow execution engine: (1) structured step config → agent container, (2) PII approval request → Matrix event → blocking wait → resume/deny.

### Concrete Deliverables
- `bridge/pkg/studio/factory.go` — `Config json.RawMessage` field on `SpawnRequest` + `STEP_CONFIG` env var injection in `Spawn()`
- `bridge/pkg/secretary/pending_approval.go` — New file: `PendingApproval()` function
- `bridge/pkg/secretary/orchestrator_integration.go` — Replace NeedsApproval error return with `PendingApproval()` call
- `bridge/pkg/secretary/orchestrator_integration.go` — Add `EventBus` field to `StepExecutorConfig`
- `bridge/cmd/bridge/main.go` — Wire `EventBus` into `StepExecutorConfig` construction
- `doc/armorclaw.md` — Multi-agent execution behavior documented

### Definition of Done
- [ ] `go build ./...` in bridge/ compiles clean
- [ ] `go test ./pkg/studio/... ./pkg/secretary/...` — all existing 186 tests + new tests pass
- [ ] Agent container with `Config` in SpawnRequest receives `STEP_CONFIG` env var
- [ ] PII-requiring step emits `app.armorclaw.pii_request` and blocks until response or timeout

### Must Have
- `STEP_CONFIG` env var present in container when SpawnRequest.Config is non-empty
- `STEP_CONFIG` env var ABSENT when SpawnRequest.Config is nil/empty
- `PendingApproval()` emits `app.armorclaw.pii_request` Matrix event with stepID and required fields
- `PendingApproval()` blocks until matching `app.armorclaw.pii_response` arrives, ctx cancels, or timeout
- `PendingApproval()` returns approved field IDs on approval, error on denial/timeout
- Multi-agent documentation in doc/armorclaw.md

### Must NOT Have (Guardrails)
- Do NOT modify `buildEnvironment()` function signature — too many callers
- Do NOT mirror Config through `SpawnRequestRef` / `task_scheduler.go` / `main.go` adapter — out of scope (Config only for workflow paths)
- Do NOT create new `PiiRef` or `PiiField` types — use `[]string` consistently with existing code
- Do NOT add new methods to the `EventEmitter` interface — 5 mock implementations would need updating
- Do NOT modify `ApprovalChecker` interface signature — used by tests
- Do NOT register any new RPC methods or secretary RPCHandler
- Do NOT fix ArmorChat `pii.approve_access`/`pii.reject_access` naming mismatch — out of scope
- Do NOT add schema changes, new store methods, or new RPC methods
- Do NOT fix ListWorkflows no-op filter or warm dispatch Description=CronExpression bug — separate PRs
- Do NOT implement skill invocation as a step type or parallel step execution — deferred

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (go test, existing 186 tests)
- **Automated tests**: Tests-after (same as Layer 3)
- **Framework**: go test

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go code**: Use `go test -v -run TestName ./pkg/...` — compile, run, assert pass/fail
- **Documentation**: Use `grep` to verify content present in doc/armorclaw.md

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — all independent):
├── Task 1: Config passthrough (factory.go) [quick]
├── Task 2: PII approval blocking (pending_approval.go + orchestrator_integration.go) [deep]
└── Task 3: Document multi-agent behavior (doc/armorclaw.md) [quick]

Wave 2 (After Wave 1 — tests + main.go wiring):
├── Task 4: Tests for Config passthrough + PII approval [unspecified-high]
└── Task 5: Wire EventBus into StepExecutorConfig in main.go [quick]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Real manual QA (unspecified-high)
└── F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: T2 → T4 → T5 → FINAL
Parallel Speedup: T1, T2, T3 run in parallel
Max Concurrent: 3 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1    | —         | 4      | 1    |
| 2    | —         | 4, 5   | 1    |
| 3    | —         | FINAL  | 1    |
| 4    | 1, 2      | FINAL  | 2    |
| 5    | 2         | FINAL  | 2    |

### Agent Dispatch Summary

- **Wave 1**: 3 tasks — T1 → `quick`, T2 → `deep`, T3 → `quick`
- **Wave 2**: 2 tasks — T4 → `unspecified-high`, T5 → `quick`
- **FINAL**: 4 tasks — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Config passthrough — SpawnRequest.Config → STEP_CONFIG env var

  **What to do**:
  - In `bridge/pkg/studio/factory.go`:
    1. Add `Config json.RawMessage \`json:"config,omitempty"\`` field to `SpawnRequest` struct (after existing 4 fields at ~line 63)
    2. In `Spawn()` method (~line 89-95), AFTER the `buildEnvironment()` call and BEFORE the Docker container creation, add:
       ```go
       if len(req.Config) > 0 {
           env = append(env, "STEP_CONFIG="+string(req.Config))
       }
       ```
       This appends `STEP_CONFIG` to the env slice returned by `buildEnvironment()`.
    3. Do NOT modify `buildEnvironment()` function signature — it takes `(*AgentDefinition, string)` and that's fine. The Config injection happens in `Spawn()` which has access to the full `SpawnRequest`.
  - In `bridge/pkg/secretary/orchestrator_integration.go`:
    1. Where `SpawnRequest` is constructed (~line 241-246 in `executeStep()`), add `Config: step.Config` to the struct literal. `WorkflowStep.Config` is already `json.RawMessage` so this is a direct field copy.
  - Verify with `go build ./...` that no compilation errors occur. The new field has `omitempty` so all existing `SpawnRequest{}` literals compile fine without the Config field.

  **Must NOT do**:
  - Do NOT modify `buildEnvironment()` function signature
  - Do NOT mirror Config through `SpawnRequestRef` in task_scheduler.go
  - Do NOT update the adapter in main.go that converts SpawnRequestRef → SpawnRequest
  - Do NOT create new types — use `json.RawMessage` which is already the type of `WorkflowStep.Config`

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single struct field addition + 3-line env var injection, ~15 lines of production code
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - None needed — pure Go struct/env manipulation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3)
  - **Blocks**: Task 4 (tests)
  - **Blocked By**: None

  **References**:

  **Pattern References** (existing code to follow):
  - `bridge/pkg/studio/factory.go:59-64` — `SpawnRequest` struct with current 4 fields (DefinitionID, TaskDescription, UserID, RoomID). Add Config as 5th field.
  - `bridge/pkg/studio/factory.go:89-95` — `Spawn()` method that calls `buildEnvironment()` at ~line 93. Insert STEP_CONFIG injection AFTER this call, BEFORE container creation.
  - `bridge/pkg/studio/factory.go:177-203` — `buildEnvironment()` function — DO NOT MODIFY this. Reference only to understand what env vars are already set (TASK_DESCRIPTION, AGENT_ID, etc.)

  **API/Type References** (contracts to implement against):
  - `bridge/pkg/secretary/types.go:63-90` — `WorkflowStep` struct — has `Config json.RawMessage` field. This is the source of Config data that flows into SpawnRequest.

  **Test References** (testing patterns to follow):
  - `bridge/pkg/studio/factory_test.go` — Existing test patterns for Spawn/factory. Follow same mocking approach.

  **WHY Each Reference Matters**:
  - `factory.go:59-64` — Exact struct layout to add Config field without breaking JSON serialization
  - `factory.go:89-95` — Exact insertion point in Spawn() method flow
  - `types.go:63-90` — Confirms WorkflowStep.Config is already json.RawMessage — direct field copy, no conversion needed

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: STEP_CONFIG present when Config is non-empty
    Tool: Bash (go test)
    Preconditions: SpawnRequest with Config: json.RawMessage(`{"model":"gpt-4","temperature":0.7}`)
    Steps:
      1. Run: cd bridge && go test -v -run TestSpawnConfigPassthrough ./pkg/studio/... 
      2. Assert test passes: Spawn() was called, env slice contains "STEP_CONFIG={"model":"gpt-4","temperature":0.7}"
    Expected Result: env slice contains exact STEP_CONFIG string
    Failure Indicators: STEP_CONFIG not found in env, or value truncated/malformed
    Evidence: .sisyphus/evidence/task-1-config-passthrough.txt

  Scenario: STEP_CONFIG absent when Config is nil/empty
    Tool: Bash (go test)
    Preconditions: SpawnRequest with Config: nil
    Steps:
      1. Run: cd bridge && go test -v -run TestSpawnConfigPassthrough ./pkg/studio/... 
      2. Assert: env slice does NOT contain any "STEP_CONFIG=" entry
    Expected Result: No STEP_CONFIG entry in env slice
    Failure Indicators: STEP_CONFIG= present with empty value
    Evidence: .sisyphus/evidence/task-1-config-nil.txt

  Scenario: Existing SpawnRequest construction compiles without Config field
    Tool: Bash (go build)
    Preconditions: All existing SpawnRequest{} literals in codebase
    Steps:
      1. Run: cd bridge && go build ./...
      2. Assert: zero compilation errors
    Expected Result: Clean build — omitempty means existing code doesn't need Config field
    Failure Indicators: Compilation error about missing Config field
    Evidence: .sisyphus/evidence/task-1-build-clean.txt
  ```

  **Commit**: YES (groups with Tasks 2, 3, 4, 5)
  - Message: `feat(secretary): add step config passthrough and PII approval blocking for workflow execution`
  - Files: `bridge/pkg/studio/factory.go`, `bridge/pkg/secretary/orchestrator_integration.go`
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/studio/...`

- [x] 2. PII approval blocking — PendingApproval() + EventBus injection

  **What to do**:

  **Part A: Create `bridge/pkg/secretary/pending_approval.go`** (new file):
  1. Define `PendingApproval` function:
     ```go
     func PendingApproval(ctx context.Context, eventBus EventEmitter, roomID, stepID string, requiredFields []string) ([]string, error)
     ```
     - `EventEmitter` is the existing interface from `orchestrator_events.go` (has `Publish` method)
     - Uses `[]string` for field names — no new types needed
  2. Emit `app.armorclaw.pii_request` Matrix event via `eventBus.Publish()` containing stepID and requiredFields
  3. Subscribe to events — use a channel or polling pattern to watch for `app.armorclaw.pii_response` events matching this stepID
  4. Block with `select` on:
     - Response channel: if response says "approved", return approved field IDs `([]string, nil)`
     - Response channel: if response says "denied", return `([]string, fmt.Errorf("PII approval denied for step %s", stepID))`
     - Context.Done(): return `([]string, ctx.Err())`
     - `time.After(120 * time.Second)`: return `([]string, fmt.Errorf("PII approval timed out for step %s after 120s", stepID))`
  5. Follow the `WorkflowEventEmitter.publish()` pattern from `orchestrator_events.go:161` for event emission

  **Part B: Modify `bridge/pkg/secretary/orchestrator_integration.go`**:
  1. Add `EventBus EventEmitter` field to `StepExecutorConfig` struct (~line 52-59)
  2. Store `eventBus` in `StepExecutor` struct when constructed from config
  3. At the NeedsApproval branch (~line 148-149), replace the error return:
     ```go
     // BEFORE:
     // return fmt.Errorf("step %s requires PII approval", step.ID)
     
     // AFTER:
     approved, err := PendingApproval(ctx, e.eventBus, workflow.RoomID, step.ID, result.RequiredFields)
     if err != nil {
         return fmt.Errorf("PII approval failed for step %s: %w", step.ID, err)
     }
     // approved fields can be used by the step — for now, log and continue
     ```
  4. Guard with nil check: if `e.eventBus == nil`, fall back to error return (same as current behavior) — this prevents breaking non-workflow paths that don't have EventBus wired

  **Part C: Wire EventBus in main.go** (construction site):
  1. In `bridge/cmd/bridge/main.go`, at the `StepExecutorConfig` construction (~line 2636), add `EventBus: eventEmitter` (the `WorkflowEventEmitter` constructed earlier in the dependency chain)

  **Must NOT do**:
  - Do NOT add new methods to the `EventEmitter` interface — it already has `Publish`
  - Do NOT modify `ApprovalChecker` interface — used by tests
  - Do NOT create `PiiRef` or `PiiField` types — use `[]string`
  - Do NOT fix ArmorChat RPC method naming mismatch (`pii.approve_access` vs `pii.approve`) — out of scope
  - Do NOT register new RPC methods

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Multi-file change touching new file creation, struct modification, conditional logic with Matrix event emission/subsciption, and main.go wiring. Requires understanding event flow and blocking patterns.
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - None needed — pure Go code, no external tools

  **Parallelization**:
  - **Can Run In Parallel**: YES (Part A can be done independently; Parts B/C depend on Part A's function signature)
  - **Parallel Group**: Wave 1 (with Tasks 1, 3) — write pending_approval.go in parallel
  - **Blocks**: Tasks 4, 5
  - **Blocked By**: None

  **References**:

  **Pattern References** (existing code to follow):
  - `bridge/pkg/secretary/orchestrator_events.go:161` — `WorkflowEventEmitter.publish()` method — follow this exact pattern for emitting Matrix events (event type, room ID, payload)
  - `bridge/pkg/secretary/orchestrator_integration.go:52-59` — `StepExecutorConfig` struct — add `EventBus` field here following same injection pattern as existing fields
  - `bridge/pkg/secretary/orchestrator_integration.go:141-155` — Current NeedsApproval branch — the exact code to replace with PendingApproval() call
  - `bridge/pkg/secretary/orchestrator_integration.go:89-95` — `NewStepExecutor()` constructor — store eventBus from config

  **API/Type References** (contracts to implement against):
  - `bridge/pkg/secretary/orchestrator_events.go` — `EventEmitter` interface — has `Publish(ctx, roomID, eventType string, payload map[string]interface{}) error`. Use this to emit PII request events.
  - `bridge/pkg/secretary/approvals.go:124-160` — `ApprovalEngine.Evaluate()` returns `ApprovalResult` with `Required bool`, `NeedsApproval bool`, `RequiredFields []string` — these RequiredFields are the `[]string` to pass to PendingApproval

  **External References**:
  - Matrix event types: `"app.armorclaw.pii_request"` and `"app.armorclaw.pii_response"` — used as string literals, NOT constants
  - Existing RPC methods `pii.approve`/`pii.reject` (in secretary RPC handler) handle the phone-side response — these emit the `pii_response` events that PendingApproval watches for

  **WHY Each Reference Matters**:
  - `orchestrator_events.go:161` — Exact pattern for publishing Matrix events; copy the publish approach
  - `orchestrator_integration.go:52-59` — Injection point for EventBus into StepExecutor
  - `orchestrator_integration.go:141-155` — The code block being replaced; understand the current guard logic
  - `approvals.go:124-160` — Understand what EvaluateStep returns (RequiredFields []string) to thread into PendingApproval

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: PendingApproval emits pii_request event and returns on approval
    Tool: Bash (go test)
    Preconditions: Mock EventEmitter that records Publish calls; mock event subscription that sends approval response after 100ms
    Steps:
      1. Call PendingApproval(ctx, mockEventBus, "!room:server", "step-pay-1", []string{"payment.card_number"})
      2. Assert: mockEventBus.Publish was called with eventType="app.armorclaw.pii_request", roomID="!room:server"
      3. Send approval response to subscription channel: {stepID: "step-pay-1", approved: true, fields: ["payment.card_number"]}
      4. Assert: PendingApproval returns ([]string{"payment.card_number"}, nil)
    Expected Result: Event emitted, function returns approved fields
    Failure Indicators: Event not emitted, wrong event type, function doesn't return, returns error
    Evidence: .sisyphus/evidence/task-2-pending-approval-approve.txt

  Scenario: PendingApproval returns error on denial
    Tool: Bash (go test)
    Preconditions: Mock EventEmitter; send denial response
    Steps:
      1. Call PendingApproval(ctx, mockEventBus, "!room:server", "step-pay-2", []string{"payment.card_number"})
      2. Send denial response: {stepID: "step-pay-2", approved: false}
      3. Assert: returns ([]string, error) where error contains "denied"
    Expected Result: Error returned with "denied" in message
    Failure Indicators: Returns nil error, or returns fields despite denial
    Evidence: .sisyphus/evidence/task-2-pending-approval-deny.txt

  Scenario: PendingApproval returns error on timeout
    Tool: Bash (go test)
    Preconditions: Mock EventEmitter; no response sent; ctx with 1s timeout
    Steps:
      1. Call PendingApproval(ctx_with_1s_timeout, mockEventBus, "!room:server", "step-pay-3", []string{"ssn"})
      2. Wait for function to return
      3. Assert: returns error (context.DeadlineExceeded or custom timeout error)
    Expected Result: Error returned indicating timeout
    Failure Indicators: Function hangs indefinitely, returns nil error
    Evidence: .sisyphus/evidence/task-2-pending-approval-timeout.txt

  Scenario: NeedsApproval branch calls PendingApproval instead of returning error
    Tool: Bash (go build + go test)
    Preconditions: StepExecutor with EventBus wired; step that triggers NeedsApproval=true
    Steps:
      1. Run: cd bridge && go build ./...
      2. Run: cd bridge && go test -v -run TestExecuteSteps ./pkg/secretary/...
      3. Assert: test shows PendingApproval being called (not immediate error return)
    Expected Result: PendingApproval called, event emitted, blocking wait begins
    Failure Indicators: Old error return path still active
    Evidence: .sisyphus/evidence/task-2-integration-wiring.txt
  ```

  **Commit**: YES (groups with Tasks 1, 3, 4, 5)
  - Message: `feat(secretary): add step config passthrough and PII approval blocking for workflow execution`
  - Files: `bridge/pkg/secretary/pending_approval.go`, `bridge/pkg/secretary/orchestrator_integration.go`, `bridge/cmd/bridge/main.go`
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/secretary/...`

- [x] 3. Document multi-agent behavior as unimplemented

  **What to do**:
  - In `doc/armorclaw.md`, find the "Workflow Execution Lifecycle" section (added in Layer 3, T7)
  - Add a new `### Multi-Agent Execution` subsection at the end of that section with:
    - `Step.AgentIDs []string` is a selection pool, not a parallel execution directive
    - Only `AgentIDs[0]` is used by `executeStep()` in `orchestrator_integration.go`
    - `StepParallel` type exists in the `StepType` enum but is not implemented
    - Multi-agent step execution is a deferred feature, not a bug
    - When implemented, `AgentIDs[1:]` will be used for failover or parallel execution
  - Keep it concise (~10-15 lines of markdown)

  **Must NOT do**:
  - Do NOT create a new "Phase 3" section — insert as subsection within existing Workflow Execution Lifecycle
  - Do NOT document implementation details for parallel execution (it's deferred)
  - Do NOT modify any code files — documentation only

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single documentation file edit, ~15 lines of markdown
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - None needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2)
  - **Blocks**: FINAL
  - **Blocked By**: None

  **References**:

  **Pattern References** (existing code to follow):
  - `doc/armorclaw.md` — Look at the "Workflow Execution Lifecycle" section for heading levels and formatting style. Match the existing markdown conventions.
  - `bridge/pkg/secretary/types.go:63-90` — `WorkflowStep` struct — shows `AgentIDs []string` field (confirm exact field name)
  - `bridge/pkg/secretary/types.go:92-101` — `StepType` enum — shows `StepParallel` constant (confirm exact name)
  - `bridge/pkg/secretary/orchestrator_integration.go:237` — `executeStep()` picks `AgentIDs[0]` (confirm exact line)

  **WHY Each Reference Matters**:
  - `doc/armorclaw.md` — Formatting conventions for the documentation
  - `types.go:63-101` — Source of truth for field names and enum values to document accurately
  - `orchestrator_integration.go:237` — The actual code behavior being documented

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Multi-agent documentation present in doc/armorclaw.md
    Tool: Bash (grep)
    Preconditions: doc/armorclaw.md exists
    Steps:
      1. Run: grep -c "Multi-Agent Execution" doc/armorclaw.md
      2. Assert: count >= 1 (section heading exists)
      3. Run: grep -c "AgentIDs\[0\]" doc/armorclaw.md
      4. Assert: count >= 1 (behavior documented)
      5. Run: grep -c "StepParallel" doc/armorclaw.md
      6. Assert: count >= 1 (unimplemented type mentioned)
      7. Run: grep -c "deferred" doc/armorclaw.md
      8. Assert: count >= 1 (deferred status documented)
    Expected Result: All grep counts >= 1
    Failure Indicators: Any grep count is 0
    Evidence: .sisyphus/evidence/task-3-doc-present.txt

  Scenario: Documentation is properly formatted markdown
    Tool: Bash (grep)
    Preconditions: doc/armorclaw.md
    Steps:
      1. Run: grep "^### Multi-Agent Execution" doc/armorclaw.md
      2. Assert: heading uses ### (h3) level matching parent section style
    Expected Result: Heading found with correct level
    Failure Indicators: No ### heading, or wrong heading level
    Evidence: .sisyphus/evidence/task-3-doc-format.txt
  ```

  **Commit**: YES (groups with Tasks 1, 2, 4, 5)
  - Message: `feat(secretary): add step config passthrough and PII approval blocking for workflow execution`
  - Files: `doc/armorclaw.md`
  - Pre-commit: `cd bridge && go build ./...` (doc-only change but verify build still clean)

- [x] 4. Tests for Config passthrough and PII approval blocking

  **What to do**:
  - Create test file `bridge/pkg/studio/factory_config_test.go` (or add to existing `factory_test.go`):
    1. `TestSpawnConfigPassthrough` — SpawnRequest with Config, verify STEP_CONFIG in env
    2. `TestSpawnConfigNil` — SpawnRequest with nil Config, verify no STEP_CONFIG in env
    3. `TestSpawnConfigEmpty` — SpawnRequest with `Config: json.RawMessage{}`, verify no STEP_CONFIG
    4. `TestSpawnConfigRawJSON` — Config with malformed JSON, verify it passes through as-is
  - Create test file `bridge/pkg/secretary/pending_approval_test.go`:
    1. `TestPendingApprovalApproved` — mock EventBus, send approval response, verify returns fields
    2. `TestPendingApprovalDenied` — mock EventBus, send denial, verify error
    3. `TestPendingApprovalTimeout` — short ctx timeout, verify context error
    4. `TestPendingApprovalEmitsEvent` — verify `app.armorclaw.pii_request` event emitted with correct payload
    5. `TestPendingApprovalCtxCancelled` — pre-cancelled ctx, verify immediate return
  - Follow existing test patterns from `bridge/pkg/studio/factory_test.go` and `bridge/pkg/secretary/orchestrator_integration_test.go`
  - Use table-driven tests where appropriate

  **Must NOT do**:
  - Do NOT test ArmorChat RPC methods — out of scope
  - Do NOT test StepExecutor internals beyond what's changed
  - Do NOT create integration tests that require running Matrix server

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multiple test files with 9+ test cases, requires understanding mocking patterns for EventBus and Docker client
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - None needed

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (with Task 5 — these CAN run in parallel since T5 is main.go wiring, not test code)
  - **Blocks**: FINAL
  - **Blocked By**: Tasks 1, 2

  **References**:

  **Pattern References** (existing code to follow):
  - `bridge/pkg/studio/factory_test.go` — Existing factory test patterns (mock Docker client, assert env vars)
  - `bridge/pkg/secretary/orchestrator_integration_test.go` — Existing StepExecutor test patterns (mock dependencies, table-driven)
  - `bridge/pkg/secretary/approvals_test.go` — Existing ApprovalEngine test patterns

  **API/Type References**:
  - `bridge/pkg/secretary/orchestrator_events.go` — `EventEmitter` interface — mock this for PendingApproval tests

  **WHY Each Reference Matters**:
  - `factory_test.go` — Copy the mocking approach for Docker client and env var assertions
  - `orchestrator_integration_test.go` — Copy the StepExecutor construction pattern with mocked config
  - `approvals_test.go` — Understand how ApprovalResult is used in tests

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All new tests pass
    Tool: Bash (go test)
    Preconditions: Tasks 1 and 2 complete, code compiles
    Steps:
      1. Run: cd bridge && go test -v -run "TestSpawn|TestPendingApproval" ./pkg/studio/... ./pkg/secretary/...
      2. Assert: all tests PASS, zero failures
    Expected Result: All new tests pass
    Failure Indicators: Any test failure
    Evidence: .sisyphus/evidence/task-4-tests-pass.txt

  Scenario: All existing tests still pass
    Tool: Bash (go test)
    Preconditions: New test code added
    Steps:
      1. Run: cd bridge && go test ./pkg/studio/... ./pkg/secretary/...
      2. Assert: all 186+ tests pass, zero failures
    Expected Result: No regressions
    Failure Indicators: Any previously-passing test now fails
    Evidence: .sisyphus/evidence/task-4-no-regressions.txt
  ```

  **Commit**: YES (groups with Tasks 1, 2, 3, 5)
  - Message: `feat(secretary): add step config passthrough and PII approval blocking for workflow execution`
  - Files: `bridge/pkg/studio/factory_config_test.go`, `bridge/pkg/secretary/pending_approval_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/studio/... ./pkg/secretary/...`

- [x] 5. Wire EventBus into StepExecutorConfig in main.go

  **What to do**:
  - In `bridge/cmd/bridge/main.go`, find the `StepExecutorConfig` struct construction (from Layer 3 T4, ~line 2636)
  - Add `EventBus: eventEmitter` to the struct literal
  - `eventEmitter` is the `WorkflowEventEmitter` variable constructed earlier in the dependency chain (Layer 3 T4, ~line 2619)
  - This is a single-line addition to an existing struct literal

  **Must NOT do**:
  - Do NOT modify the dependency chain order
  - Do NOT add new construction code — just add one field to existing struct literal
  - Do NOT touch any other main.go code

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single field addition to existing struct literal, 1 line of code
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - None needed

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 4 — different files)
  - **Parallel Group**: Wave 2 (with Task 4)
  - **Blocks**: FINAL
  - **Blocked By**: Task 2 (needs StepExecutorConfig.EventBus field to exist)

  **References**:

  **Pattern References**:
  - `bridge/cmd/bridge/main.go:2619` — `WorkflowEventEmitter` construction — this is the `eventEmitter` variable to reference
  - `bridge/cmd/bridge/main.go:2636` — `StepExecutorConfig` struct literal — add `EventBus: eventEmitter` here
  - `bridge/pkg/secretary/orchestrator_integration.go:52-59` — `StepExecutorConfig` struct definition (modified in Task 2 to add EventBus field)

  **WHY Each Reference Matters**:
  - `main.go:2619` — The eventEmitter variable name to use in the wiring
  - `main.go:2636` — Exact struct literal where EventBus field goes
  - `orchestrator_integration.go:52-59` — Confirms the field name and type from Task 2

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: EventBus wired into StepExecutorConfig
    Tool: Bash (go build)
    Preconditions: Task 2 complete (EventBus field exists on StepExecutorConfig)
    Steps:
      1. Run: cd bridge && go build ./...
      2. Assert: zero compilation errors
    Expected Result: Clean build — EventBus field is recognized in main.go struct literal
    Failure Indicators: "unknown field EventBus in struct literal" or similar compile error
    Evidence: .sisyphus/evidence/task-5-build-clean.txt

  Scenario: Full test suite passes with wiring in place
    Tool: Bash (go test)
    Preconditions: All tasks complete
    Steps:
      1. Run: cd bridge && go test ./pkg/studio/... ./pkg/secretary/...
      2. Assert: all tests pass
    Expected Result: All tests pass, no regressions from wiring change
    Failure Indicators: Any test failure
    Evidence: .sisyphus/evidence/task-5-tests-pass.txt
  ```

  **Commit**: YES (groups with Tasks 1, 2, 3, 4)
  - Message: `feat(secretary): add step config passthrough and PII approval blocking for workflow execution`
  - Files: `bridge/cmd/bridge/main.go`
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/studio/... ./pkg/secretary/...`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go build ./...` + `go vet ./...` + `go test ./pkg/studio/... ./pkg/secretary/...`. Review all changed files for: unnecessary type assertions, empty catches, fmt.Println in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (Config + PII working together in a workflow). Test edge cases: nil Config, empty Config, ctx cancelled, timeout. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git diff HEAD). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **1**: `feat(secretary): add step config passthrough and PII approval blocking for workflow execution`
  - Files: `bridge/pkg/studio/factory.go`, `bridge/pkg/secretary/pending_approval.go`, `bridge/pkg/secretary/orchestrator_integration.go`, `bridge/cmd/bridge/main.go`, `doc/armorclaw.md`, plus any test files
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/studio/... ./pkg/secretary/...`

---

## Success Criteria

### Verification Commands
```bash
cd bridge && go build ./...                    # Expected: clean build, zero errors
cd bridge && go test ./pkg/studio/... ./pkg/secretary/...  # Expected: all tests pass
grep -c "STEP_CONFIG" bridge/pkg/studio/factory.go         # Expected: ≥ 2 (field + env var)
grep -c "PendingApproval" bridge/pkg/secretary/            # Expected: ≥ 3 (define + call + test)
grep -c "Multi-Agent" doc/armorclaw.md                     # Expected: ≥ 1
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass (186 existing + new)
- [ ] Single commit with correct message
