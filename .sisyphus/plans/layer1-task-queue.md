# Layer 1: Task Queue — Persistent Scheduling & Agent State

## TL;DR

> **Quick Summary**: Build a persistent task scheduler that dispatches scheduled tasks to agents via warm-start (Matrix room injection — agent already polls Matrix via Bridge socket) or cold-start (spawn new container). Prerequisite: bind-mount agent state directory so sessions survive container restarts.
>
> **Deliverables**:
> - Agent state directory bind-mounted at `/var/lib/armorclaw/agent-state/{definitionID}/`
> - RoomID persisted on AgentInstance
> - Dead in-memory scheduler deleted
> - `robfig/cron/v3` dependency added
> - `task_scheduler.go` — stateless dispatcher (15s ticker loop, uses `cron.ParseStandard().Next()` as pure function)
> - `definition_id` column on `scheduled_tasks`
> - `task.*` RPC domain (create, list, cancel, get)
> - `app.armorclaw.task_dispatch` Matrix event type
> - Doc updates (context routing, RPC count, state machine notes)
>
> **Estimated Effort**: Medium
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: Task 1 (state mount) → Task 2.6 (RoomID) → Task 3 (store methods) → Task 4 (scheduler loop) → Task 6 (RPC) → Task 7 (wiring) → F1-F4

---

## Context

### Warm-Start Dispatch Mechanism (User Decision)

**Decision**: Matrix room injection. The Bridge sends `app.armorclaw.task_dispatch` as a Matrix event to the agent's room via `MatrixAdapter.SendEvent()`. The agent already polls Matrix messages through the Bridge Unix socket (`ARMORCLAW_BRIDGE_SOCKET=/run/armorclaw/bridge.sock`). No new container-level mechanism needed.

**Why not cold-start-only**: Cold-start spawns a new container for every dispatch, which is expensive and doesn't reuse running agents. Matrix room injection uses the existing message pipeline.

**Why not Unix socket**: Would require a new agent-side listener, bind-mount configuration, and protocol design. Matrix room injection reuses existing infrastructure.

### Two-Database Architecture

`scheduled_tasks` lives in `/var/lib/armorclaw/rolodex.db` (secretary store). `agent_instances` and `agent_definitions` live in the studio store (separate SQLite file). The scheduler bridges this via the `Factory` interface — same pattern as `WorkflowOrchestratorImpl`. No cross-database FK needed; `definition_id` is validated at task-creation time.

### Metis Review

**Identified Gaps (addressed)**:
- Warm-start mechanism undefined → **Resolved**: Matrix room injection
- Two-database cross-reference → **Resolved**: Factory interface pattern (same as WorkflowOrchestratorImpl)
- Concurrent dispatch edge case → **Added guardrail**: If previous execution still running, skip this tick
- Bridge restart during execution → **Added guardrail**: On startup, re-calculate next_run for missed tasks
- No migration framework → **Confirmed**: Follow existing raw SQL ALTER TABLE pattern
- `cron.ParseStandard().Next()` returns zero time → **Added guardrail**: Log error, deactivate task, don't crash loop

### Original Request
User wants persistent task scheduling for ArmorClaw agents — the ability to schedule tasks that survive Bridge restarts and container restarts, dispatching to running agents or cold-starting new containers as needed.

### Investigation Findings

**Layer 1 Investigation (complete)**:

| Assumption | Reality |
|---|---|
| "No task persistence exists" | `bridge/pkg/secretary/store.go` has `scheduled_tasks` table with full CRUD |
| "No scheduler exists" | `bridge/pkg/secretary/orchestrator_scheduler.go` exists but is 100% dead code — zero callers, two disconnected parallel systems |
| "Need to build task storage" | Task storage already exists in `/var/lib/armorclaw/rolodex.db` (plain SQLite, NOT keystore) |
| "No cron parser exists" | Correct — no cron library in go.mod, no cron parsing code anywhere. Need `robfig/cron/v3` |
| "RoomID is tracked" | `SpawnRequest.RoomID` exists but `Spawn()` never reads it. `AgentInstance` has no `room_id` column |
| "Agent state persists" | `ReadonlyRootfs: true` + zero bind mounts = no writable path. Sessions go to ephemeral `/tmp` or fail silently |

**Session file path**: `/home/claw/.openclaw/agents/main/sessions/<sessionId>.jsonl`
- `resolveStateDir()` → `~/.openclaw` (no `OPENCLAW_STATE_DIR` env override in container)
- `resolveAgentSessionsDir()` → `<stateDir>/agents/main/sessions`

**Secretary store DB**: `/var/lib/armorclaw/rolodex.db` — plain SQLite3, no SQLCipher, no unseal needed

**`scheduled_tasks` current schema**:
```sql
CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id TEXT PRIMARY KEY,
    template_id TEXT NOT NULL,          -- FK → task_templates.id
    cron_expression TEXT NOT NULL,
    timezone TEXT DEFAULT 'UTC',
    next_run INTEGER,
    last_run INTEGER,
    is_active INTEGER DEFAULT 1,
    created_by TEXT NOT NULL,
    FOREIGN KEY (template_id) REFERENCES task_templates(id) ON DELETE CASCADE
);
```

**`task_templates` has no `definition_id` field. `scheduled_tasks` has no `definition_id`. Both need it.**

**Existing factory methods for scheduler**:
- `store.ListInstances(definitionID, StatusRunning)` — already exists, returns `[]*AgentInstance`
- `factory.ListInstances(definitionID)` — wrapper, returns all statuses
- Need to add: `factory.GetRunningInstance(definitionID)` — convenience wrapper returning single instance

**Studio linkage chain for scheduler**:
```
ScheduledTask.definition_id  ← NEW COLUMN
  → studio.GetAgent(definition_id)                              // AgentDefinition
  → store.ListInstances(definition_id, StatusRunning)           // running?
  → if running: instance.RoomID                                 // warm dispatch
  → if not: factory.Spawn(definition_id, taskDescription)       // cold start
```

### Key Design Decision

**robfig/cron/v3 usage pattern**: One `cron.New()` per task (not one global scheduler). When a task is cancelled, remove its cron entry. When `next_run` is updated, recreate the cron entry. The scheduler loop is a stateless dispatcher — it reads due tasks from the DB, dispatches them, and updates `next_run`. It does not hold task state in memory.

---

## Work Objectives

### Core Objective
Enable persistent task scheduling that dispatches to agents via warm-start (Matrix event) or cold-start (container spawn), with state surviving Bridge and container restarts.

### Concrete Deliverables
- `/var/lib/armorclaw/agent-state/{definitionID}/` bind-mounted to `/home/claw/.openclaw` in containers
- `room_id TEXT` column on `agent_instances` table, populated at spawn time
- Dead `orchestrator_scheduler.go` and test file deleted
- `robfig/cron/v3` in `go.mod`
- `bridge/pkg/secretary/task_scheduler.go` — 15-second tick loop, dispatches due tasks
- `bridge/pkg/secretary/task_dispatch.go` — Matrix event payload builder
- `definition_id TEXT` column on `scheduled_tasks` + `ScheduledTask` struct
- 4 new RPC methods: `task.create`, `task.list`, `task.cancel`, `task.get`
- Context routing rule + RPC count + state machine notes updated in doc

### Definition of Done
- [ ] `go build ./...` passes in bridge/
- [ ] Agent spawns, writes JSONL session, container removed, re-spawned — session persists
- [ ] Scheduled task with `next_run` in the past gets dispatched within 15 seconds
- [ ] Bridge restart — task still dispatched (rolodex.db persistence)
- [ ] `task.create` RPC returns task_id; `task.list` shows it; `task.cancel` sets `is_active=0`
- [ ] Running agent receives `app.armorclaw.task_dispatch` Matrix event

### Must Have
- Agent state directory bind-mount working end-to-end
- Scheduler loop dispatching due tasks (warm and cold paths)
- `task.*` RPC domain functional
- `definition_id` on `scheduled_tasks` and `room_id` on `agent_instances`
- Dead code deleted

### Must NOT Have (Guardrails)
- Do NOT modify the keystore schema or interact with SQLCipher
- Do NOT remove `ReadonlyRootfs: true` — add explicit bind mounts instead
- Do NOT change `ResolveStateDir()` logic in the TypeScript code
- Do NOT create a global cron scheduler — use `cron.ParseStandard().Next()` as a pure function for next_run calculation
- Do NOT hold task state in memory — the loop is stateless, DB is truth
- Do NOT add `definition_id` to `task_templates` — it goes on `scheduled_tasks` only
- Do NOT wire the scheduler to the old dead `Scheduler` struct — build fresh
- Do NOT change the secretary store DB path (`/var/lib/armorclaw/rolodex.db`)
- Do NOT modify Dockerfile.openclaw image build — state dir is host-side only
- Do NOT skip the ALTER TABLE migration pattern (must handle "duplicate column" gracefully)
- Do NOT use `ScheduleRecurring` or `ScheduleOnce` types from the deleted scheduler
- Do NOT add network access to containers (`NetworkMode` stays `"none"`)
- Do NOT introduce a migration framework or schema versioning — follow raw SQL pattern
- Do NOT modify `bridge/pkg/runtime/docker/adapter.go` or `bridge/pkg/docker/client.go` — changes go in `studio/factory.go` directly
- Do NOT touch the sidecar gRPC proto for scheduler communication
- Do NOT crash the scheduler loop on cron parse errors — log, deactivate task, continue
- Do NOT dispatch a task if its previous execution is still running (check container status first)
- Do NOT add a `Factory` method that the scheduler doesn't need — use existing `ListInstances(definitionID, StatusRunning)` via convenience wrapper
  - Do NOT panic on errors — log and continue

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Tasks 2.6, 2.5, 3)
  - **Parallel Group**: Wave 2 (with Tasks 3, 5 — but this file depends on 3's store methods being complete)
  - **Blocks**: Tasks 6, 7 (RPC and wiring need scheduler to exist)
  - **Blocked By**: Tasks 2.6 (RoomID), 2.5 (cron lib), 3 (store methods)

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/store.go:42-48` — `Store` interface. `ListDueTasks` and `MarkDispatched` will be added by Task 3.
  - `bridge/pkg/secretary/store.go:921-941` — `ListPendingScheduledTasks` — the existing query pattern for pending tasks. `ListDueTasks` follows this pattern.
  - `bridge/pkg/studio/factory.go:286-289` — `ListInstances(definitionID)` — scheduler uses `GetRunningInstance()` wrapper (added in Task 2.6).
  - `bridge/pkg/secretary/orchestrator_integration.go` — Shows how secretary interacts with studio. Follow similar interface patterns.

  **API/Type References**:
  - `robfig/cron/v3` — `cron.ParseStandard(expr)` returns a `Schedule` interface. `sched.Next(time.Now())` returns the next `time.Time`. This is a pure function — no scheduler instance needed.
  - `bridge/pkg/secretary/types.go:165-189` — `ScheduledTask` struct with `DefinitionID`, `CronExpression`, `Timezone` fields.

  **External References**:
  - robfig/cron/v3 docs: https://pkg.go.dev/github.com/robfig/cron/v3 — `ParseStandard()` expects standard 5-field cron format (min hour dom month dow)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Scheduler dispatches one-shot task due now
    Tool: Bash
    Preconditions: Bridge running, task with next_run in the past inserted in DB
    Steps:
      1. Insert task: `sqlite3 /var/lib/armorclaw/rolodex.db "INSERT INTO scheduled_tasks (id, template_id, cron_expression, timezone, next_run, is_active, created_by, definition_id) VALUES ('test-dispatch-1', '', '', 'UTC', $(date +%s), 1, 'admin', 'test-agent')"`
      2. Wait 20 seconds (2 tick cycles)
      3. Check logs for "dispatched task" or "cold-started agent"
      4. Query DB: `sqlite3 /var/lib/armorclaw/rolodex.db "SELECT is_active, last_run FROM scheduled_tasks WHERE id='test-dispatch-1'"`
    Expected Result: is_active=0 (deactivated one-shot), last_run set, log shows dispatch
    Failure Indicators: Task still active, no log entry, last_run NULL
    Evidence: .sisyphus/evidence/task-4-one-shot-dispatch.txt

  Scenario: Scheduler calculates next_run for cron task
    Tool: Bash
    Steps:
      1. Insert cron task: next_run in the past, cron_expression="0 * * * *" (every hour)
      2. Wait for dispatch
      3. Query: `SELECT next_run, is_active FROM scheduled_tasks WHERE id=...`
    Expected Result: is_active=1 (still active), next_run updated to next hour
    Failure Indicators: Task deactivated, next_run not updated
    Evidence: .sisyphus/evidence/task-4-cron-next-run.txt

  Scenario: Scheduler skips task with empty definition_id
    Tool: Bash
    Steps:
      1. Insert task with definition_id=''
      2. Wait for tick
      3. Check logs for "no definition_id, skipping"
    Expected Result: Warning logged, task marked dispatched (not retried)
    Evidence: .sisyphus/evidence/task-4-skip-no-defid.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add task scheduler loop with cold/warm start dispatch`
  - Files: `bridge/pkg/secretary/task_scheduler.go`
  - Pre-commit: `cd bridge && go build ./...`

- [x] 5. Create task dispatch payload + Matrix event type

  **What to do**:
  - Create new file `bridge/pkg/secretary/task_dispatch.go`
  - Define `TaskDispatchPayload` struct:
    ```go
    type TaskDispatchPayload struct {
        TaskID       string `json:"task_id"`
        Description  string `json:"description"`
        DispatchedAt int64  `json:"dispatched_at"`
        Source       string `json:"source"`
    }
    ```
  - Define `BuildTaskDispatchPayload(task ScheduledTask, description string) TaskDispatchPayload`
  - Define event type constant: `const EventTypeTaskDispatch = "app.armorclaw.task_dispatch"`
  - This is used by `task_scheduler.go` in the warm-start path

  **Must NOT do**:
  - Do NOT define this in the matrix package — it's a secretary concern
  - Do NOT add agent-side handling — the agent already processes Matrix messages; this is a structured event it will receive

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 3, 4)
  - **Blocks**: Task 4 (scheduler uses the payload builder) — but can be in same wave since it's small
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/orchestrator_events.go` — Shows how secretary emits events. Follow the same pattern for event type constants and payload construction.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Task dispatch payload builds correctly
    Tool: Bash
    Steps:
      1. `cd bridge && go test -run TestBuildTaskDispatchPayload ./pkg/secretary/...`
      2. If no test exists, verify: build a payload with known values, check JSON serialization
    Expected Result: Payload has task_id, description, dispatched_at, source="scheduler"
    Evidence: .sisyphus/evidence/task-5-dispatch-payload.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add task dispatch payload builder and Matrix event type`
  - Files: `bridge/pkg/secretary/task_dispatch.go`
  - Pre-commit: `cd bridge && go build ./...`

- [x] 6. Add task.* RPC handlers

  **What to do**:
  - In `bridge/pkg/secretary/rpc.go`, register 4 new RPC methods following the existing pattern (lines ~98-124):
    - `task.create` → `handleTaskCreate`
    - `task.list` → `handleTaskList`
    - `task.cancel` → `handleTaskCancel`
    - `task.get` → `handleTaskGet`
  - `handleTaskCreate`:
    - Parse params: `definition_id`, `description`, `cron_expression` (optional), `run_at` (optional ISO 8601), `timezone` (default "UTC")
    - If `run_at` provided: parse and set `next_run`
    - Else if `cron_expression` provided: use `cron.ParseStandard(expr).Next(now)` to calculate `next_run`
    - Else: `next_run = now` (immediate)
    - Generate task ID
    - Call `store.CreateScheduledTask()`
    - Return `{task_id, status: "pending", next_run}`
  - `handleTaskList`:
    - Parse optional `definition_id` filter
    - Call `store.ListScheduledTasks()`
    - Return `{tasks: [...]}`
  - `handleTaskCancel`:
    - Parse `task_id`
    - Call `store.UpdateScheduledTask()` setting `is_active = false`
    - Or add `CancelScheduledTask` method: `UPDATE scheduled_tasks SET is_active = 0 WHERE id = ?`
    - Return `{success: true}`
  - `handleTaskGet`:
    - Parse `task_id`
    - Call `store.GetScheduledTask()`
    - Return the task object
  - Add `cron.ParseStandard` import from `robfig/cron/v3` in rpc.go

  **Must NOT do**:
  - Do NOT create tasks without `definition_id` — validate it's present in `task.create`
  - Do NOT auto-create templates — `template_id` can be empty for direct task creation (no workflow template needed)
  - Do NOT change existing secretary RPC methods

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (with Tasks 7, 8)
  - **Blocks**: Task 7 (wiring needs RPC to exist)
  - **Blocked By**: Tasks 3 (store methods), 4 (scheduler exists)

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/rpc.go:98-124` — RPC dispatch registration. Follow exact pattern for adding new methods to the switch/map.
  - `bridge/pkg/secretary/rpc.go` — Any existing handler method (e.g., `handleStartWorkflow`). Follow the same params parsing and error handling pattern.

  **API/Type References**:
  - `bridge/pkg/secretary/types.go:165-189` — `ScheduledTask` struct. All fields available for RPC responses.
  - `robfig/cron/v3` — `cron.ParseStandard(expr).Next(now)` for calculating `next_run`

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: task.create RPC creates a scheduled task
    Tool: Bash (socat)
    Steps:
      1. `echo '{"jsonrpc":"2.0","id":1,"method":"task.create","params":{"definition_id":"test-agent","description":"Test scheduled task","run_at":"2026-04-14T12:00:00Z"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
      2. Verify response contains task_id, status="pending", next_run timestamp
      3. Query DB: `sqlite3 /var/lib/armorclaw/rolodex.db "SELECT * FROM scheduled_tasks WHERE definition_id='test-agent'"`
    Expected Result: Task created with correct next_run, definition_id populated
    Failure Indicators: RPC error, task not in DB, definition_id empty
    Evidence: .sisyphus/evidence/task-6-task-create.txt

  Scenario: task.list returns active tasks
    Tool: Bash (socat)
    Steps:
      1. Create 2 tasks
      2. `echo '{"jsonrpc":"2.0","id":2,"method":"task.list","params":{}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
      3. Verify response contains both tasks
    Expected Result: Array with 2 task objects
    Evidence: .sisyphus/evidence/task-6-task-list.txt

  Scenario: task.cancel deactivates task
    Tool: Bash (socat)
    Steps:
      1. Create a task, note task_id
      2. `echo '{"jsonrpc":"2.0","id":3,"method":"task.cancel","params":{"task_id":"<id>"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
      3. Verify response: `{success: true}`
      4. Query DB: `SELECT is_active FROM scheduled_tasks WHERE id='<id>'` → should be 0
    Expected Result: Task deactivated, not deleted
    Evidence: .sisyphus/evidence/task-6-task-cancel.txt

  Scenario: task.create with invalid cron expression returns error
    Tool: Bash (socat)
    Steps:
      1. `echo '{"jsonrpc":"2.0","id":4,"method":"task.create","params":{"definition_id":"test","cron_expression":"invalid"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
    Expected Result: Error response with "invalid cron expression"
    Evidence: .sisyphus/evidence/task-6-invalid-cron.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add task.* RPC domain (create, list, cancel, get)`
  - Files: `bridge/pkg/secretary/rpc.go`
  - Pre-commit: `cd bridge && go build ./...`

- [x] 7. Wire scheduler loop into main.go

  **What to do**:
  - In `bridge/cmd/bridge/main.go`, find where the secretary store is initialized (~line 2527):
    ```go
    rolodexStore, err := secretary.NewStore(secretary.StoreConfig{
        Path:   "/var/lib/armorclaw/rolodex.db",
        ...
    })
    ```
  - After the secretary handler wiring (~line 2610), add:
    ```go
    // Initialize task scheduler
    taskScheduler := secretary.NewTaskScheduler(rolodexStore, studioFactory, matrixAdapter, nil)
    taskScheduler.Start()
    defer taskScheduler.Stop()
    log.Println("Task scheduler started (15s tick interval)")
    ```
  - Place the `defer taskScheduler.Stop()` in the graceful shutdown block alongside existing defers
  - Verify the factory and matrix adapter are available at this point in main.go (they should be — secretary handler wiring uses them)
  - The factory needs to implement the `FactoryInterface` defined in `task_scheduler.go`. Either:
    - Make `AgentFactory` satisfy the interface directly (if method signatures match)
    - Or create a thin adapter struct

  **Must NOT do**:
  - Do NOT start the scheduler if `rolodexStore` is nil (check the warning pattern at line 2532)
  - Do NOT change the existing secretary handler wiring
  - Do NOT move the scheduler start before the studio factory initialization

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (with Tasks 6, 8)
  - **Blocks**: F1-F4 (needs full system running for QA)
  - **Blocked By**: Tasks 4 (scheduler), 6 (RPC)

  **References**:

  **Pattern References**:
  - `bridge/cmd/bridge/main.go:2525-2613` — Secretary initialization block. Add scheduler wiring after line 2610.
  - `bridge/cmd/bridge/main.go:2527-2534` — Pattern for nil-check: `if rolodexStore != nil` guard.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Scheduler starts with bridge
    Tool: Bash
    Steps:
      1. Restart bridge
      2. Check logs for "Task scheduler started"
      3. Insert a due task via RPC
      4. Wait 20 seconds
      5. Verify dispatch in logs
    Expected Result: "Task scheduler started" log, task dispatched within 15s
    Failure Indicators: No scheduler log, task not dispatched
    Evidence: .sisyphus/evidence/task-7-scheduler-wired.txt

  Scenario: Scheduler stops on bridge shutdown
    Tool: Bash
    Steps:
      1. Send SIGTERM to bridge
      2. Check logs for clean shutdown (no goroutine leak warnings)
    Expected Result: Clean shutdown, no leaked goroutines
    Evidence: .sisyphus/evidence/task-7-scheduler-shutdown.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): wire task scheduler into main.go lifecycle`
  - Files: `bridge/cmd/bridge/main.go`
  - Pre-commit: `cd bridge && go build ./...`

- [x] 8. Doc updates

  **What to do**:
  - In `doc/armorclaw.md`, add context routing rule (after the "Run E2E tests" row in the Context Routing Rules table, ~line 32):
    ```
    | Add or manage scheduled tasks | `bridge/pkg/secretary/store.go` and `bridge/pkg/secretary/task_scheduler.go` |
    ```
  - In `doc/armorclaw.md`, update RPC method count in §4 from "60" to "64" (4 new task.* methods)
  - In `doc/armorclaw.md`, find the Custom ArmorClaw Events table in §8 and add:
    ```
    | `app.armorclaw.task_dispatch` | Scheduler-to-agent task directive (internal control plane) |
    ```
  - In `doc/armorclaw.md`, find §25 Agent State Machine and add note:
    - Agent state directory is bind-mounted at `/var/lib/armorclaw/agent-state/{definitionID}/`
    - State survives container restart (bind mount overrides `ReadonlyRootfs`)
    - JSONL sessions, logs, and config persist across container lifecycle
  - In `doc/armorclaw.md`, find the Secretary section and add note about task scheduler:
    - 15-second tick interval
    - Stateless dispatcher: reads due tasks from `rolodex.db`, dispatches, updates `next_run`
    - Warm path: Matrix event to running agent
    - Cold path: spawn new container from definition
    - Uses `robfig/cron/v3` for cron expression parsing

  **Must NOT do**:
  - Do NOT rewrite any existing sections
  - Do NOT change the document structure or ordering
  - Do NOT delete any existing content
  - Targeted additions only

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 6, 7)
  - **Blocks**: F1-F4
  - **Blocked By**: None (doc updates are independent of code)

  **References**:

  **Pattern References**:
  - `doc/armorclaw.md:11-33` — Context Routing Rules table. Add row after line 32.
  - `doc/armorclaw.md` — §4 Go Bridge Component (search for "60 methods" or "RPC methods" count)
  - `doc/armorclaw.md` — §8 Matrix section (search for Custom ArmorClaw Events table)
  - `doc/armorclaw.md` — §25 Agent State Machine

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Context routing rule present
    Tool: Bash
    Steps:
      1. `grep "scheduled tasks" doc/armorclaw.md`
    Expected Result: Finds the new row with store.go and task_scheduler.go paths
    Evidence: .sisyphus/evidence/task-8-routing-rule.txt

  Scenario: RPC count updated
    Tool: Bash
    Steps:
      1. `grep -n "64.*method\|method.*64" doc/armorclaw.md`
    Expected Result: Finds updated count
    Evidence: .sisyphus/evidence/task-8-rpc-count.txt
  ```

  **Commit**: YES
  - Message: `docs(armorclaw): add Layer 1 task queue architecture and routing rules`
  - Files: `doc/armorclaw.md`
  - Pre-commit: None

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go build ./...` + `go vet ./...` + `go test ./pkg/secretary/... ./pkg/studio/...`. Review all changed files for: `as any`, empty catches, `fmt.Println` in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (scheduler dispatches to running agent, RPC creates task that gets dispatched). Test edge cases: task with no definition_id, task with invalid cron, task cancelled mid-dispatch. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Task 1**: `feat(studio): bind-mount agent state directory for session persistence` — `bridge/pkg/studio/factory.go`
- **Task 2**: `refactor(secretary): remove dead in-memory scheduler` — `bridge/pkg/secretary/orchestrator_scheduler.go`, `bridge/pkg/secretary/orchestrator_scheduler_test.go`
- **Task 2.5**: `deps(secretary): add robfig/cron/v3 for scheduled task parsing` — `bridge/go.mod`, `bridge/go.sum`
- **Task 2.6**: `feat(studio): persist RoomID on AgentInstance for task dispatch` — `bridge/pkg/studio/types.go`, `bridge/pkg/studio/store.go`, `bridge/pkg/studio/factory.go`
- **Task 3**: `feat(secretary): add definition_id to scheduled_tasks and new store methods` — `bridge/pkg/secretary/store.go`, `bridge/pkg/secretary/types.go`
- **Task 4**: `feat(secretary): add task scheduler loop with cold/warm start dispatch` — `bridge/pkg/secretary/task_scheduler.go`
- **Task 5**: `feat(secretary): add task dispatch payload builder and Matrix event type` — `bridge/pkg/secretary/task_dispatch.go`
- **Task 6**: `feat(secretary): add task.* RPC domain (create, list, cancel, get)` — `bridge/pkg/secretary/rpc.go`
- **Task 7**: `feat(bridge): wire task scheduler into main.go lifecycle` — `bridge/cmd/bridge/main.go`
- **Task 8**: `docs(armorclaw): add Layer 1 task queue architecture and routing rules` — `doc/armorclaw.md`

---

## Success Criteria

### Verification Commands
```bash
cd bridge && go build ./...                          # Expected: clean build
cd bridge && go vet ./...                            # Expected: no warnings
cd bridge && go test ./pkg/secretary/... ./pkg/studio/...  # Expected: all pass
sqlite3 /var/lib/armorclaw/rolodex.db ".schema scheduled_tasks"  # Expected: has definition_id column
sqlite3 /var/lib/armorclaw/rolodex.db ".schema agent_instances"  # Expected: has room_id column
ls /var/lib/armorclaw/agent-state/                   # Expected: directories exist for spawned agents
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass
- [ ] Agent state persists across container restart
- [ ] Scheduled tasks dispatch within 15 seconds
- [ ] task.* RPC methods respond correctly
- [ ] Dead scheduler files deleted (zero references)
- [ ] Context routing rule added to doc
