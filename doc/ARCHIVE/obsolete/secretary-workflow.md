# Secretary Workflow System

> Part of the [ArmorClaw System Documentation](armorclaw.md)

Deep dive into ArmorClaw's workflow engine: scheduled tasks, multi-step workflows, and PII approval gates.

> **Source root:** `bridge/pkg/secretary/`

---

## Overview

The secretary package is ArmorClaw's automation core. It turns task templates into runnable workflows, dispatches them on cron schedules or on demand, and enforces human approval gates whenever a step touches PII.

Three things happen inside the secretary:

1. **Scheduling.** A `TaskScheduler` polls the database every 15 seconds, picks up tasks whose `next_run` is due, and dispatches them.
2. **Workflow execution.** For tasks with a template, the scheduler creates a `Workflow` record, hands it to the `WorkflowOrchestratorImpl`, and the orchestrator walks through each `WorkflowStep` sequentially, spawning isolated containers for each one.
3. **PII approval.** Before any step that references PII fields, the `ApprovalEngineImpl` evaluates policies. If a policy requires manual approval, the step blocks until the user responds from ArmorChat (or a 120 second timeout expires).

```
ScheduledTask (cron)
       │
       ▼
  TaskScheduler.tick()
       │
       ├── template_id set? ──► templateDispatch()
       │        creates Workflow ──► Orchestrator.StartWorkflow()
       │        OrchestratorIntegration.StartWorkflowExecution()
       │               │
       │               ▼
       │        StepExecutor.ExecuteSteps()
       │          for each step:
       │            ApprovalEngine.EvaluateStep()  ──► PII gate
       │            factory.Spawn()                 ──► container
       │            waitForCompletion()             ──► 500ms poll
       │            AdvanceWorkflow()
       │
        └── no template ──► coldDispatch() only
                 container spawn (no warm dispatch)
```

---

> ⚠️ **Data Flow (Updated)**
>
> Agent containers in the secretary workflow execute with `NetworkMode: "none"`, meaning they have **zero network access**. Communication flow:
>
> - **Inbound to container**: Environment variables (`STEP_CONFIG`, `PII_*` fallback)
> - **Outbound from container**: Exit code + `result.json` (step mode) or exit code only (agent mode)
> - **Real-time events**: Containers emit `StepEvent` entries to `_events.jsonl` during execution, which the Bridge tails for live progress
>
> In step mode (STEP_CONFIG present), the container writes structured results to `result.json` in the bind-mounted state dir before exit. The Bridge reads this via `ParseContainerStepResult()` (or `ParseExtendedStepResult()` for enriched results with blockers and skill candidates). During execution, the Bridge also tails `_events.jsonl` via `EventReader` for real-time step progress. See `doc/agent-runtime.md` for the step mode flow.
>
> Remaining limitations:
> - Agent state transitions (BROWSING, FORM_FILLING, etc.) are **invisible** to the Bridge
> - Browser automation is **impossible** in this mode (no network to reach browser service)
> - Agent mode (no STEP_CONFIG) still has no backward channel

---

## Architecture

### Component map

| Component | Source file | Role |
|-----------|------------|------|
| `WorkflowOrchestratorImpl` | `orchestrator.go` | State machine: pending → running → completed/failed/cancelled. Holds active workflows in memory, emits events on every transition. |
| `DependencyValidator` | `orchestrator.go` (embedded) | Validates step ordering before execution. |
| `OrchestratorIntegration` | `orchestrator_integration.go` | Glues orchestrator + executor + approval engine + notifications together. Owns the goroutine that runs a workflow end to end. |
| `StepExecutor` | `orchestrator_integration.go` | Spawns containers, polls for completion, retries on recoverable errors. |
| `WorkflowEventEmitter` | `orchestrator_events.go` | Publishes `workflow.*` events to the `MatrixEventBus`. |
| `ApprovalEngineImpl` | `approvals.go` | Evaluates policies against PII fields. Returns allow/deny/require_approval per field. |
| `PendingApproval` / `HandlePIIResponse` | `pending_approval.go` | Blocking PII gate: publishes `app.armorclaw.pii_request` to Matrix, waits for `app.armorclaw.pii_response`. |
| `NotificationService` | `notifications.go` | Fan out workflow and approval notifications to subscribers (Matrix adapter, etc.). |
| `TaskScheduler` | `task_scheduler.go` | 15 second tick loop. Stateless dispatcher that reads due tasks from DB. |
| `EventReader` | `event_reader.go` | Incremental `_events.jsonl` tailer. Tracks byte offset and sequence number for deduplication. Enforces 10 MB cap. |
| `EventFileCleaner` | `cleanup.go` | Removes the state directory (including `_events.jsonl`) after step completion. Ensures parse→purge→notify ordering. |
| `BlockerHandler` | `orchestrator_integration.go` | Runs the spawn→wait→blocker loop: blocks workflow, waits for user response, re-spawns with updated config. Max 3 retries, 10-minute timeout. |
| `SkillInjector` | `orchestrator_integration.go` | Injects `relevant_skills` into step config before dispatch via `injectLearnedSkills()`. |
| `SkillExtractor` | `bridge/pkg/skills/extractor.go` | Analyzes `ExtendedStepResult` with 5 strategies to produce `LearnedSkill` suggestions. |
| `MatrixEventBus` | `bridge/internal/events/matrix_event_bus.go` | Ring buffer (default 1024 slots). Delivers events to the Matrix conduit and to in process subscribers. |

### Key types (types.go)

```
TaskTemplate           Definition of a reusable workflow (steps, variables, PII refs)
Workflow               Runtime instance of a template
WorkflowStep           One step in a template (action, condition, parallel variants)
WorkflowStatus         pending | running | blocked | completed | failed | cancelled
ApprovalPolicy         Rules for auto approve vs. manual gate, per PII field
ApprovalResult         Outcome of evaluating policies: approved/denied/needs_approval
ScheduledTask          Cron entry that triggers a template dispatch or a direct agent spawn
StepEvent              Structured event emitted by containers to _events.jsonl (seq, type, name, ts_ms, detail, duration_ms)
BlockerResponse        User response to a blocker prompt (input, note, user_id, provided_at)
ExtendedStepResult     Enriched result with _comments, _blockers, _skill_candidates, _events_summary
Blocker                Obstacle that prevented step completion (blocker_type, message, suggestion, field)
SkillCandidate         Detected automation opportunity from agent output (name, description, pattern_type, confidence)
LearnedSkill           Persisted execution pattern extracted from successful tasks (confidence, trigger_keywords, success/failure counts)
```

Step types (`StepType` enum):

| StepType | Constant | Purpose |
|----------|----------|---------|
| `StepAction` | `"action"` | Execute an agent action |
| `StepCondition` | `"condition"` | Evaluate a condition |
| `StepParallel` | `"parallel"` | Execute steps in parallel |
| `StepParallelSplit` | `"parallel_split"` | Fork into parallel branches |
| `StepParallelMerge` | `"parallel_merge"` | Rejoin parallel branches |

---

## Two Dispatch Paths

The `TaskScheduler` has two completely separate dispatch paths.

### Path 1: Workflow engine (template dispatch)

Triggered when `ScheduledTask.TemplateID` is set.

1. `templateDispatch()` fetches the `TaskTemplate` from the store.
2. Creates a `Workflow` record in `pending` status.
3. Calls `Orchestrator.StartWorkflow(workflowID)` which transitions to `running` and launches a background goroutine.
4. Calls `OrchestratorIntegration.StartWorkflowExecution(workflowID)` which runs `StepExecutor.ExecuteSteps()` in a goroutine.
5. On completion, calls `Orchestrator.CompleteWorkflow()` or `FailWorkflow()`.
6. After dispatch, calculates the next run time from the cron expression and updates the scheduled task.

### Path 2: Cold dispatch only (no template)

Triggered when `ScheduledTask.DefinitionID` is set but `TemplateID` is empty.

The scheduler always spawns a fresh container via `factory.Spawn()`. There is no warm dispatch path.

> **Deprecated**: Warm dispatch (`warmDispatch()`) was removed in v0.7.0. It was architecturally illegal under `NetworkMode: none` because containers cannot receive inbound Matrix events. The dead code has been purged.

- **Cold dispatch (FUNCTIONAL, limited).** The scheduler calls `factory.Spawn()` to create a fresh container for the task. Functional but limited to exit-code-only results.

After dispatch, the task's `next_run` is updated (cron) or the task is deactivated (one shot).

---

## Workflow Lifecycle

### State machine

```
              ┌─────────────┐
              │   pending   │
              └──────┬──────┘
                     │ StartWorkflow()
                     ▼
              ┌─────────────┐
        ┌────▶│   running   │◀───┐
        │     └──┬───┬──┬───┘    │
        │        │   │  │        │
        │        │   │  │        │
   CancelWorkflow│  │  │ AdvanceWorkflow
        │        │   │  │ (last step)
        │        │   │  │        │ UnblockWorkflow()
        │        │   │  │        │
        │        ▼   │  ▼        │
  ┌──────────┐  │ ┌──────────┐  │
  │cancelled │  │ │completed │──┘
  └──────────┘  │ └──────────┘
           FailWorkflow()
           BlockWorkflow()
                │        │
                ▼        ▼
         ┌──────────┐ ┌──────────┐
         │  failed  │ │ blocked  │
         └──────────┘ └──────────┘
```

Valid transitions (defined in `validateTransition`):

| From | To |
|------|----|
| pending | running, cancelled |
| running | completed, failed, cancelled, blocked |
| blocked | running, failed, cancelled |
| completed | (terminal) |
| failed | (terminal) |
| cancelled | (terminal) |

### Lifecycle walk through

1. **Template loaded.** `StartWorkflow()` loads the `TaskTemplate` from the store, sets `CurrentStep` to the first step's ID, stores the workflow in `activeWorkflows` map, emits `workflow.started`, and kicks off `executeWorkflow()` in a goroutine.

2. **Execution loop.** `OrchestratorIntegration.runWorkflow()` calls `StepExecutor.ExecuteSteps()`, which iterates through the validated step order.

3. **Step completion.** After each step, `AdvanceWorkflow()` updates `CurrentStep` and `currentIndex`. When the last step finishes, it calls `completeWorkflowLocked()` which sets status to `completed`, removes the workflow from `activeWorkflows`, and emits `workflow.completed`.

4. **Failure.** `FailWorkflow()` sets status to `failed`, stores the error message, removes from active map, emits `workflow.failed`.

5. **Cancellation.** `CancelWorkflow()` cancels the context, sets status to `cancelled`, removes from active map, emits `workflow.cancelled`. Also cancels any running step containers via `executor.CancelAllForWorkflow()`.

---

## Step Execution

Each step goes through `StepExecutor.executeStep()`:

```
StepExecutor                      Container (NetworkMode: "none")
    │                                      │
    ├─ No agentIDs? ──► ErrNoAgentForStep  │
    │                                      │
    ├─ Build SpawnRequest:                 │
    │     DefinitionID, TaskDescription,   │
    │     UserID, RoomID, Config           │
    │                                      │
    ├─ Inject learned skills into config   │
    │     (injectLearnedSkills)            │
    │                                      │
    ├─ STEP_CONFIG env ──────────────────▶ │ (inbound: env vars only)
    │                                      │
    ├─ Register in runningSteps map        │
    │                                      │
    └─ waitForCompletion(ctx, instanceID, stateDir)  │
         │                                 │
         └─ 500ms polling loop:            │
              GetStatus(instanceID)        │
              ReadNew() from _events.jsonl ◀──│ (real-time events)
              Route events: step_progress, │
                step_error, blocker_warning│
                Complete  ──▶ ParseExtended ◀──│ (outbound: exit code +
                             StepResult()      │  result.json + _events.jsonl)
                Failed     ──▶ ParseExtended ◀──│
                             StepResult()      │
                Running    ──▶ continue        │
                ctx.Done() ──▶ Stop, error     │
```

### Retry behavior

`executeStepWithRetry()` wraps `executeStep()` with configurable retry:

- Default retry count: 1 (one retry after initial failure)
- Default retry delay: 1 second
- Default timeout: 5 minutes
- Only recoverable errors are retried. Spawn failures are recoverable. Steps with no agent assigned are not.

### Config passthrough

Each `WorkflowStep` carries a `Config` field (`json.RawMessage`). This is passed directly to `SpawnRequest.Config` when the container is spawned. The container receives it as the `STEP_CONFIG` environment variable.

This is how template authors pass step specific configuration (API endpoints, parameters, flags) into the agent container without modifying the agent definition.

### Inter-step data passing (v0.7.0)

WorkflowSteps now carry an optional `Input` field (`map[string]any`, JSON tag: `"input,omitempty"`) that enables data to flow from one step's output into the next step's input. This is the primary mechanism for sequential step data propagation.

#### WorkflowStep.Input field

```go
type WorkflowStep struct {
    // ... existing fields ...
    Input map[string]any `json:"input,omitempty"` // v0.7.0: template variables for inter-step data
}
```

The `Input` field supports **template variable references** using the `{{steps.<step_id>.data.<key>}}` syntax. When the orchestrator resolves a step's input before execution, it replaces these references with the corresponding values from previously completed steps' `result.json` `data` fields.

#### Resolution rules

1. **Variable syntax**: `{{steps.step_1.data.order_id}}` references the `order_id` key from step `step_1`'s result data.
2. **Resolution timing**: Variables are resolved just before the container is spawned, after PII approval checks.
3. **Missing references**: If a referenced step hasn't completed yet or the data key doesn't exist, the variable is replaced with an empty string and a warning is logged.
4. **Backward compatibility**: Existing templates without `input` fields round-trip unchanged. The field is optional (`omitempty`) and defaults to nil.

#### Example

```json
{
  "steps": [
    {
      "id": "fetch_order",
      "type": "action",
      "agent_ids": ["researcher"],
      "config": {"task": "Fetch order details for customer ACME-123"}
    },
    {
      "id": "process_order",
      "type": "action",
      "agent_ids": ["processor"],
      "input": {
        "order_id": "{{steps.fetch_order.data.order_id}}",
        "customer_email": "{{steps.fetch_order.data.email}}"
      },
      "config": {"task": "Process the order"}
    }
  ]
}
```

In this example, `process_order` receives the `order_id` and `email` from the `fetch_order` step's result data.

#### Migration

Existing templates are unaffected. To add inter-step data passing:
1. Add an `"input": {}` field to the steps that need upstream data.
2. Use `{{steps.<step_id>.data.<key>}}` syntax to reference previous step outputs.
3. A migration tool (`bridge/cmd/migrate-templates/`) is available to add empty `input` fields to existing template JSON files. See `doc/migration/workflow-step-input.md` for the full migration guide.

### Data flow

Containers spawned by the step executor run with `NetworkMode: "none"`. In step mode, the executor observes exit code, `result.json`, and `_events.jsonl`:

- Exit 0 (status `Completed`): step succeeded. Bridge reads `result.json` via `ParseExtendedStepResult()` which also reads `_events.jsonl` for timeline events.
- Non zero exit (status `Failed`): step failed. Bridge reads `result.json` for error details and `_events.jsonl` for any events emitted before failure.
- Container still running: keep polling. `EventReader.ReadNew()` tails `_events.jsonl` for real-time progress.

The container writes structured results (status, output, data, error, duration_ms) to `result.json` before exit. The `EventEmitter` in the container writes `StepEvent` entries to `_events.jsonl` throughout execution. After parsing, the state directory is purged via `cleanupStateDir()`. See `doc/agent-runtime.md` Step Mode section for the full flow.

---

## Parallel Step Execution (v0.6.0)

`StepParallelSplit` and `StepParallelMerge` step types are now **implemented** (previously defined in `StepType` but unused).

### How it works

1. **Group identification.** `IdentifyParallelGroups()` scans the step list for `StepParallelSplit`/`StepParallelMerge` pairs by `Order` field. All steps between a Split and its matching Merge form one parallel group.

2. **Goroutine pool.** Each group runs inside an `errgroup` pool with a configurable concurrency limit (`MaxParallelContainers`, default: 2). Each step in the group gets its own goroutine.

3. **Dependency edges.** The Split and Merge steps create implicit dependency edges: Split → first step in group, last step in group → Merge. No changes to the `WorkflowStep` struct itself.

4. **Collection policies.**

| Policy | Behavior |
|--------|----------|
| `FailFast` | Stop on first error. Cancel remaining goroutines. |
| `CollectAll` | Wait for all steps to finish. Collect every error. |

5. **Sequential backward compatibility.** Templates without Split/Merge steps work unchanged. The executor falls through to the normal sequential loop.

### Configuration

| Field | Default | Location |
|-------|---------|----------|
| `MaxParallelContainers` | 2 | `StepExecutorConfig` |
| Collection policy | `FailFast` | Hardcoded, per-group override planned |

Source: `bridge/pkg/secretary/orchestrator_parallel.go`

---

## Session Transcript Compaction (v0.6.0)

Bridge-side pre-dispatch pruning of session history before sending to the AI model. Prevents token overflow on long-running workflows.

### How it works

1. **Token estimation.** `EstimateMessageTokens()` provides a rough per-message estimate: `len(text) / 4` for text content, character count for tool results. Not exact, but consistent enough for threshold checks.

2. **Threshold check.** `ShouldCompact()` compares the estimated total against `CompactionThresholdTokens` (default: 100,000). Returns true if exceeded.

3. **Compaction.** `CompactHistory()` has a two-tier strategy:
   - **Primary:** Ask the AI to summarize the conversation history into a condensed form. Preserves key decisions, tool results, and context.
   - **Fallback:** If the AI call fails, apply windowed truncation. Keep the system prompt + first N messages + last N messages, dropping the middle.

### Configuration

| Field | Default | Location |
|-------|---------|----------|
| `CompactionThresholdTokens` | 100,000 | `bridge/internal/agent/runtime.go` |

Source: `bridge/internal/ai/compaction.go`

---

## Step Failover (v0.6.0)

Per-step failover with multi-agent fallback. If the primary agent for a step fails, the executor tries the next agent from the step's agent list.

### How it works

1. **Agent list.** Each `WorkflowStep` can specify multiple agent IDs in its `AgentIDs` field. Previously only the first was used.

2. **Failover loop.** On step failure, the executor advances to the next agent ID (up to `StepRetryCount` attempts total). Each attempt spawns a fresh container for the new agent.

3. **Policy control.**

| `FailoverPolicy` | Behavior |
|------------------|----------|
| `FailoverRetry` | Try next agent on failure. Default. |
| `FailoverImmediateFail` | Fail the step immediately on first error. |

4. **Error aggregation.** `FailoverAggregatedError` collects errors from every attempt (agent ID, error message, timestamp) for diagnostics and logging.

### Configuration

| Field | Default | Location |
|-------|---------|----------|
| `FailoverPolicy` | `FailoverRetry` | `StepExecutorConfig` |
| `StepRetryCount` | 1 (one retry) | `StepExecutorConfig` |

Source: `bridge/pkg/secretary/orchestrator_integration.go`

---

## Observable Containers

Containers in step mode emit structured events to `_events.jsonl` in the bind-mounted state directory during execution. This is implemented by the `EventEmitter` class in `container/openclaw/events.py`.

### How it works

1. `StepRunner.run()` creates an `EventEmitter` instance for the state directory.
2. The emitter opens `_events.jsonl` for append and writes a header comment.
3. Handlers emit events via convenience methods (`step()`, `file_read()`, `command_run()`, etc.).
4. Each event is serialized as a single JSON line, respecting `PIPE_BUF` (4096 bytes) for atomic writes on Linux. Lines exceeding this limit are truncated (detail replaced with `_truncated: true`, then name shortened, then detail dropped entirely).
5. On close, the emitter writes a `_summary` event with total event count and elapsed time.

### Event types

| Type | Method | Purpose |
|------|--------|---------|
| `step` | `step()` | Generic step start/complete |
| `file_read` | `file_read()` | File read operation (path, lines, size) |
| `file_write` | `file_write()` | File write operation (path, changes, size) |
| `file_delete` | `file_delete()` | File deletion (path) |
| `command_run` | `command_run()` | Shell command execution (command, exit_code, truncated) |
| `observation` | `observation()` | Agent observation or note |
| `blocker` | `blocker()` | Agent hit an obstacle needing human input |
| `error` | `error()` | Error during execution |
| `artifact` | `artifact()` | Output artifact produced (name, path, mime_type, size) |
| `checkpoint` | `checkpoint()` | Named execution checkpoint |
| `progress` | `progress()` | Progress percentage update |

Source: `container/openclaw/events.py`

---

## Event Streaming

The Bridge tails `_events.jsonl` during step execution for real-time progress visibility.

### EventReader (event_reader.go)

`EventReader` incrementally reads new events from `<stateDir>/_events.jsonl`. Each call to `ReadNew()` returns only lines appended since the previous call, tracked via byte offset and sequence number. If the file does not exist, it returns `(nil, 0, nil)` so callers can poll without special casing.

**10 MB soft cap**: If the file exceeds `maxEventLogSize` (10 MB), `ReadNew()` returns `ErrEventLogExceeded`. The calling code in `waitForCompletion()` handles this by logging a warning and setting a `capExceeded` flag. The container is **not** killed — it continues executing and finishes naturally via the normal Docker polling loop. After completion, `cleanupStateDir()` purges the oversized log. This is a soft cap, not a hard termination: the container's output is preserved, only real-time event tailing stops.

### Event routing

During the 500ms polling loop in `waitForCompletion()`, events are read and routed by type:

1. **step, progress** events are converted to `EmitStepProgress()` workflow events via `WorkflowEventEmitter`, extracting `percent` from `detail`.
2. **error** events are converted to `EmitStepError()` workflow events.
3. **blocker** events are converted to `EmitBlockerWarning()` workflow events with blocker_type and message from detail.
4. All other events are collected into `ExtendedStepResult.Events` for timeline formatting.

### State directory cleanup

After step completion (success or failure), `cleanupStateDir()` removes the entire state directory including `_events.jsonl`. The ordering is:

1. **Parse** result.json and _events.jsonl into `ExtendedStepResult`
2. **Purge** the state directory via `cleanupStateDir()`
3. **Notify** subscribers with the parsed result

This ensures events are never lost before they can be processed.

---

## PII Approval Flow

This is the human in the loop gate that blocks workflow steps involving sensitive data.

### How it triggers

Inside `StepExecutor.ExecuteSteps()`, before executing each step:

```go
if e.approvalEngine != nil && len(template.PIIRefs) > 0 {
    approvalResult, err := e.approvalEngine.EvaluateStep(...)
    // If approval required and not yet approved:
    approvedFields, err := PendingApproval(ctx, eventBus, roomID, stepID, deniedFields)
}
```

Only runs when the template declares `PIIRefs` (PII field references). If there are no PII references, the approval check is skipped entirely.

### Policy evaluation (approvals.go)

`ApprovalEngineImpl.Evaluate()`:

1. If `PIIFields` is empty, immediately returns approved (no gate).
2. Loads all active policies from the store.
3. Filters policies whose `PIIFields` overlap with the requested fields.
4. For each matching policy, calls `evaluateSinglePolicy()`:
   - If `AutoApprove` is true and conditions pass: allow.
   - If `AutoApprove` is true but conditions fail: require_approval.
   - If conditions pass: allow.
   - Otherwise: require_approval.
5. Merges results across policies: approved fields minus denied fields, denied fields win over approved.

Condition evaluation supports these operators: `eq`/`==`/`=`, `neq`/`!=`, `in`, `nin`/`not_in`, `contains`. Fields that can be checked include `workflow.status`, `workflow.created_by`, `template.id`, `template.name`, `step.type`, `step.id`, `initiator`, `subject`, plus any key in the workflow's `Variables` map.

### The blocking gate (pending_approval.go)

When `ApprovalResult.NeedsApproval` is true:

1. `PendingApproval()` registers a channel in a global map (`pendingApps`), keyed by step ID.
2. Publishes an `app.armorclaw.pii_request` event to the Matrix room containing the step ID and required fields.
3. Blocks on one of three outcomes:
   - **Approved.** `HandlePIIResponse()` delivers a response via the channel. Returns approved field list.
   - **Denied.** Same channel, but `Approved: false`. Returns error.
   - **Timeout.** 120 seconds. Returns error.
   - **Cancellation.** Context cancelled (workflow cancelled). Returns error.

4. `HandlePIIResponse()` is the entry point called by the RPC handler when an `app.armorclaw.pii_response` Matrix event arrives from the ArmorChat client. It looks up the step ID in the pending map and sends the response down the channel.

```
StepExecutor                    MatrixEventBus              ArmorChat
    │                               │                         │
    ├─ PendingApproval()            │                         │
    │   register channel            │                         │
    │   publish pii_request ────────▶│──► Matrix room ────────▶│
    │   block on channel            │                         │
    │                               │                         │
    │                               │    user taps Approve     │
    │                               │◀── pii_response ────────│
    │                               │                         │
    │   HandlePIIResponse()         │                         │
    │   channel <- response         │                         │
    │   unblock                     │                         │
    │   continue step execution     │                         │
```

### Approval outcomes

| Scenario | What happens |
|----------|-------------|
| No PII fields in template | No approval check. Step runs immediately. |
| PII fields but no matching policies | Auto approved. Step runs. |
| Policy with auto_approve + conditions pass | Auto approved. Step runs. |
| Policy requires approval | Blocks. PII request sent to Matrix. Waits for response. |
| User approves | Step unblocks. Execution continues. |
| User denies | Step fails with "PII approval denied". Workflow fails. |
| 120s timeout | Step fails with timeout error. Workflow fails. |
| Workflow cancelled while waiting | Step unblocks with context cancellation. Workflow cleaned up. |

---

## Blocker Protocol

The blocker protocol is a human-in-the-loop resolution mechanism for obstacles encountered during step execution. It is distinct from PII approval: blockers handle missing input or ambiguous situations, while PII approval gates access to sensitive data fields.

### How it works

1. **Container signals blocker.** The container writes a `blocker` event to `_events.jsonl` via `EventEmitter.blocker()`, or appends to the `_blockers` list in the config dict. On completion, these are merged into `ExtendedStepResult.Blockers`.

2. **Bridge detects blocker.** `executeStepWithBlockerHandling()` checks `ExtendedStepResult.Blockers` after step completion. If blockers are present, it calls `orchestrator.BlockWorkflow()` to transition the workflow to `StatusBlocked`.

3. **Notification.** `BlockWorkflow()` persists the status change and emits a `workflow.blocked` event via `EmitBlocked()`. The notification reaches the user's Matrix room as a formatted blocker message (via `FormatBlockerMessage()`).

4. **Wait for resolution.** `waitForBlockerResponse()` registers a channel in the `pendingBlockers` sync.Map, keyed by `"blocker:{workflowID}:{stepID}"`, and waits for one of:
   - **Response received.** An external caller (RPC or Matrix handler) calls `DeliverBlockerResponse()` which sends the response down the channel.
   - **Timeout.** `BlockerTimeout` (10 minutes). Returns error.
   - **Cancellation.** Context cancelled (workflow cancelled). Returns error.

5. **Re-spawn.** On resolution, `appendBlockerResponse()` adds `_blocker_response` to the step config, `UnblockWorkflow()` transitions back to `StatusRunning`, and the container is re-spawned with the updated config.

6. **Retry limit.** Max `MaxBlockerRetries` (3) attempts. After that, the step fails.

### PII safety

Blocker responses may contain sensitive input (passwords, API keys). The response payload is:
- Never logged (intentional omission from log statements)
- Passed to the container via the `_blocker_response` config key (environment variable only, never written to disk as a standalone file)
- The `BlockerResponse.Input` field carries the raw user input

### RPC handler

The `resolve_blocker` RPC method (`bridge/pkg/rpc/server.go`) accepts `workflow_id`, `step_id`, and `input` parameters, constructs a `BlockerResponse`, and calls `DeliverBlockerResponse()`.

```
Container                    Bridge                          User (ArmorChat)
    │                          │                                  │
    ├─ emit blocker event      │                                  │
    │   to _events.jsonl       │                                  │
    │                          │                                  │
    ├─ write result.json       │                                  │
    │   with _blockers         │                                  │
    │                          │                                  │
    │   ── exit ──────────────▶│                                  │
    │                          │                                  │
    │                          ├─ BlockWorkflow()                 │
    │                          │   status → blocked               │
    │                          │                                  │
    │                          ├─ EmitBlocked() ──▶ Matrix ──────▶│
    │                          │   FormatBlockerMessage()         │
    │                          │                                  │
    │                          │         user provides input      │
    │                          │◀── resolve_blocker RPC ──────────│
    │                          │   or Matrix /sync event          │
    │                          │                                  │
    │                          ├─ DeliverBlockerResponse()        │
    │                          │   channel ← response             │
    │                          │                                  │
    │                          ├─ UnblockWorkflow()               │
    │                          │   status → running               │
    │                          │                                  │
    │◀───── re-spawn ──────────│                                  │
    │   STEP_CONFIG with       │                                  │
    │   _blocker_response      │                                  │
```

Sources: `bridge/pkg/secretary/orchestrator_integration.go`, `bridge/pkg/rpc/server.go`, `container/openclaw/events.py`

### Blocker Metadata Pipeline Fix (v0.6.0)

Fixed 7 bugs in the blocker metadata pipeline from container → Bridge → Matrix:

| Bug | Fix |
|-----|-----|
| Container `events.py:blocker()` put human-readable message in `event.name`, not in `event.detail["message"]` | Bridge now extracts from both locations |
| `EmitBlockerWarning()` was never called — no `case "blocker":` in the event routing switch | `case "blocker":` added to routing switch |
| Blocker metadata (blocker_type, suggestion, field, workflow_id) dropped during pipeline transit | Metadata now flows through the full pipeline to Matrix events |
| `BlockWorkflow` and `EmitBlocked`不接受 variadic metadata params | Now accept optional metadata kwargs without breaking existing callers |

Source: `bridge/pkg/secretary/orchestrator_integration.go`

---

## Learned Skills Pipeline

The learned skills pipeline extracts reusable execution patterns from successful task completions and suggests them for future similar tasks.

### Extraction (bridge/pkg/skills/extractor.go)

`ExtractFromResult()` analyzes an `ExtendedStepResult` using five strategies:

1. **Self-reported candidates.** The container may include `_skill_candidates` in `result.json`. Each `SkillCandidate` (name, description, pattern_type, confidence) is converted directly into a `LearnedSkill`. If confidence is unset, defaults to 0.5.

2. **Command sequence.** If the events contain 2+ `command_run` events, a `command_sequence` skill is extracted with the command list as pattern data. Confidence: 0.6.

3. **File operations.** If the events contain 1+ `file_write` or 2+ `file_read` events, a `file_transform` skill is extracted with file paths grouped by operation type. Confidence: 0.5.

4. **Step sequence.** If the events contain 3+ distinct step names (e.g., `step`, `command_run`, `file_read` in sequence), a `step_sequence` skill is extracted capturing the ordered step pattern. Confidence: 0.5.

5. **Checkpoint sequence.** If the events contain any `checkpoint` events, a `checkpoint_sequence` skill is extracted capturing the checkpoint names and order. Confidence: 0.4.

Skills are deduplicated by name before saving.

### Persistence (bridge/pkg/skills/learned_store.go)

`LearnedStore` persists skills in **plain SQLite** (not SQLCipher, since learned skills contain no secrets). Key operations:

- `Save()`: Persists a `LearnedSkill`. Generates UUID if no ID provided. Rejects duplicate names.
- `FindForTask()`: Searches for skills matching a task description. Filters by `confidence >= 0.4`, ranks by keyword overlap with the task description, returns top N results.
- `RecordOutcome()`: Updates confidence based on success/failure. Success adds +0.1 (capped at 1.0). Failure subtracts 0.2 (floored at 0.0). Skills below 0.4 are effectively filtered out by `FindForTask()`.
- `Delete()`: Removes a skill by ID.
- `ListForAgent()`: Returns skills ordered by confidence for browsing.

### Injection at dispatch

`injectLearnedSkills()` in `StepExecutor` is called before spawning the container. It:
1. Calls `skillFinder.FindForTask(taskDesc, 3)` to get up to 3 matching skills.
2. Adds a `relevant_skills` array to the step config with name, confidence, pattern, and source task ID.
3. The container reads this via `StepConfig.relevant_skills`.

### Outcome recording

After step completion (success or failure), `recordSkillOutcomes()` iterates over the `relevant_skills` from the original config and calls `onSkillOutcome()` for each. This adjusts confidence up or down based on whether the skill suggestion was helpful.

Sources: `bridge/pkg/skills/extractor.go`, `bridge/pkg/skills/learned_store.go`, `bridge/pkg/secretary/orchestrator_integration.go`

---

## Matrix Commands

The secretary workflow exposes learned skill management through Matrix commands, handled by `CommandHandler` in `bridge/internal/adapter/commands_integration.go`.

### Available commands

| Command | Usage | Description |
|---------|-------|-------------|
| `!agent skills <agent_id>` | `!agent skills researcher-1` | Lists learned skills for the agent. Shows name, confidence (0.0 to 1.0), and success count. Limited to 20 results. |
| `!agent forget-skill <agent_id> <skill_id>` | `!agent forget-skill researcher-1 ls_xxx_123` | Deletes a learned skill by ID. The agent_id parameter is accepted for future per-agent scoping but currently lists globally. |

Both commands require the `learnedStore` to be configured (non-nil) on the `CommandHandler`. If not available, they return an error message.

Source: `bridge/internal/adapter/commands_integration.go`

---

## Event System

### Workflow events (orchestrator_events.go)

`WorkflowEventEmitter` publishes event types to the `MatrixEventBus`:

| Event type | Constant | Triggered by |
|------------|----------|-------------|
| `workflow.started` | `WorkflowEventStarted` | `StartWorkflow()` |
| `workflow.progress` | `WorkflowEventProgress` | `AdvanceWorkflow()`, `UpdateProgress()`, `executeWorkflow()` ticker |
| `workflow.blocked` | `WorkflowEventBlocked` | `BlockWorkflow()` |
| `workflow.completed` | `WorkflowEventCompleted` | `completeWorkflowLocked()` |
| `workflow.failed` | `WorkflowEventFailed` | `FailWorkflow()` |
| `workflow.cancelled` | `WorkflowEventCancelled` | `CancelWorkflow()` |
| `workflow.step_progress` | `WorkflowEventStepProgress` | `EmitStepProgress()` from container `_events.jsonl` |
| `workflow.step_error` | `WorkflowEventStepError` | `EmitStepError()` from container `_events.jsonl` |
| `workflow.blocker_warning` | `WorkflowEventBlockerWarning` | `EmitBlockerWarning()` from container `_events.jsonl` |

Each event carries: workflow ID, template ID, status, optional step info, progress percentage, error message, duration in milliseconds, and arbitrary metadata.

### Container step events (_events.jsonl)

Containers emit structured `StepEvent` entries to `_events.jsonl` during execution. The Bridge tails this file via `EventReader` and routes events into workflow events:

| Container event type | Routed to |
|---------------------|-----------|
| `step` | `EmitStepProgress()` with progress percent from `detail["percent"]` |
| `error` | `EmitStepError()` |
| `blocker` | `EmitBlockerWarning()` with blocker_type, message from detail |

Other container event types (`file_read`, `file_write`, `file_delete`, `command_run`, `observation`, `artifact`, `checkpoint`) are parsed and included in `ExtendedStepResult.Events` for timeline formatting.

The event file is purged after step completion via `cleanupStateDir()`. Purge ordering: parse result → purge directory → notify subscribers (never lose events before notification).

### PII events (pending_approval.go)

| Event type | Direction | Purpose |
|------------|-----------|---------|
| `app.armorclaw.pii_request` | Orchestrator → client | Asks user to approve PII field access |
| `app.armorclaw.pii_response` | Client → orchestrator | User's approve/deny decision |

### MatrixEventBus (bridge/internal/events/matrix_event_bus.go)

The bus is a fixed size ring buffer (default 1024 slots, max batch 128 events). It supports:

- `Publish()` — adds an event, broadcasts to waiters, notifies live subscribers.
- `GetEventsAfter(cursor)` — returns events newer than the given sequence number.
- `WaitForEvents(cursor)` — blocks with 25ms polling until new events arrive.
- `Subscribe()` — returns a buffered channel (cap 100) that receives every published event.

Subscribers that are too slow are silently skipped (non blocking send). The ring buffer wraps around, dropping the oldest events when full.

> **Note**: `workflow.progress` events were originally Bridge-inferred only (polling Docker status). With the `_events.jsonl` event streaming pipeline, containers now report real-time progress. The `workflow.step_progress` events carry structured data from container `StepEvent` entries. The original `workflow.progress` events from Docker polling still exist but are supplemented by the richer step events.

---

## Notifications

`NotificationService` is a pub/sub layer separate from the raw event bus. It formats human readable messages and dispatches them to registered `NotificationSubscriber` implementations.

### Notification types

| Type | When |
|------|------|
| `workflow.started` | Workflow begins execution |
| `workflow.progress` | Step progress updates |
| `workflow.completed` | All steps finished |
| `workflow.failed` | A step failed |
| `workflow.cancelled` | Workflow was cancelled |
| `approval.required` | PII approval needed |
| `approval.approved` | User approved PII access |
| `approval.denied` | User denied PII access |

### Matrix adapter

`MatrixNotificationAdapter` implements `NotificationSubscriber` by calling a `sendFunc(ctx, roomID, message)`. This is how notifications reach the user's Matrix room as readable messages (not structured events).

---

## Execution Mode Capabilities

The secretary workflow engine operates in **Mode A (Agent Studio)**:

| Capability | Status | Notes |
|-----------|--------|-------|
| Scheduled task triggering | ✅ Works | Cron-based, 15s tick interval |
| Container lifecycle management | ✅ Works | Spawn, poll, stop |
| PII approval gating | ✅ Works | Matrix → user → approve/deny |
| Workflow state tracking | ✅ Works | Bridge-level: pending → running → completed/failed |
| Structured step results | ✅ Step mode | `result.json` in state dir (step mode only) |
| Agent-reported progress | ✅ Available | Via `_events.jsonl` event streaming (step, file ops, commands, observations) |
| Browser automation | ✅ Via Jetski | Agent delegates to Jetski sidecar (separate container with network) |
| Warm dispatch | ❌ Removed (v0.7.0) | Dead code purged. `warmDispatch()` removed from `TaskScheduler`. All dispatch is cold-only. |

Browser automation is handled by the Jetski sidecar, a separate container with network access that acts as a CDP proxy to the Lightpanda browser engine. Agent containers never perform browser operations directly.

---

## Integration Points

### How the pieces connect

```
TaskScheduler
     │
     ├── store (SQLCipher) ── templates, workflows, scheduled tasks, policies
     ├── factory (studio.AgentFactory) ── container spawn/stop/status
      ├── matrix (MatrixAdapter) ── cold dispatch only (warm dispatch removed v0.7.0)
     │
     ├── orchestrator (WorkflowOrchestratorImpl)
     │       ├── store ── workflow CRUD
     │       ├── factory ── container lifecycle
     │       └── eventEmitter (WorkflowEventEmitter)
     │               └── bus (MatrixEventBus) ── ring buffer + subscribers
     │
     └── integration (OrchestratorIntegration)
             ├── orchestrator
             ├── executor (StepExecutor)
             │       ├── factory ── spawn containers
             │       ├── validator (DependencyValidator) ── step order validation
             │       ├── approvalEngine (ApprovalEngineImpl) ── PII policy eval
             │       └── eventBus (MatrixEventBus) ── PII request/response
             ├── store
             ├── approvalEngine
             └── notificationService (NotificationService)
                     └── subscribers [MatrixNotificationAdapter, ...]
```

### Bridge-Local Registry

The `BridgeLocalRegistry` (`bridge/pkg/secretary/bridge_local_registry.go`) enables certain workflow steps to execute natively in the Bridge process without spawning agent containers. This is used for:

- **Email send steps**: `email_send` → `OutboundExecutor` — validates, resolves PII placeholders, sends via Gmail/Outlook/SMTP
- **Email approval steps**: `email_approval` → `EmailApprovalManager` — blocks until user approves via ArmorChat

When the `StepExecutor` encounters a step, it checks the registry first. If a native handler is found, the step runs in-process. Otherwise, the standard container spawn path is used. This provides:

- **Performance**: No container overhead for native operations
- **Security**: Sensitive operations (PII resolution, OAuth token access) remain in the Bridge
- **Simplicity**: Email pipeline steps run as native Go code

### Shutdown behavior

`Orchestrator.Shutdown()` iterates all active workflows, cancels their contexts, sets each to `cancelled` status with reason "orchestrator shutdown", persists to store, emits `workflow.cancelled` events, and clears the active map.

`TaskScheduler.Stop()` closes the stop channel and waits for the goroutine to exit.

---

## Source File Reference

| File | Key types/functions |
|------|-------------------|
| `orchestrator.go` | `WorkflowOrchestratorImpl`, `NewWorkflowOrchestrator`, `StartWorkflow`, `AdvanceWorkflow`, `CancelWorkflow`, `CompleteWorkflow`, `FailWorkflow`, `BlockWorkflow`, `UnblockWorkflow`, `validateTransition` |
| `orchestrator_integration.go` | `StepExecutor`, `NewStepExecutor`, `ExecuteSteps`, `executeStep`, `executeStepWithRetry`, `waitForCompletion`, `OrchestratorIntegration`, `StartWorkflowExecution`, `runWorkflow`, `executeStepWithBlockerHandling`, `DeliverBlockerResponse`, `injectLearnedSkills`, `appendBlockerResponse`, `FailoverPolicy`, `FailoverAggregatedError` |
| `orchestrator_parallel.go` | `IdentifyParallelGroups`, `executeParallelGroup`, `MaxParallelContainers`, parallel Split/Merge handling |
| `orchestrator_events.go` | `EventEmitter` interface, `WorkflowEventEmitter`, `WorkflowEvent`, `WorkflowEventBuilder`, `EmitStepProgress`, `EmitStepError`, `EmitBlockerWarning` |
| `approvals.go` | `ApprovalEngineImpl`, `Evaluate`, `EvaluateStep`, `EvaluateWorkflow`, `evaluatePolicies`, `ApprovalPolicy`, `ApprovalRequest` |
| `pending_approval.go` | `PendingApproval`, `HandlePIIResponse`, PII event constants |
| `notifications.go` | `NotificationService`, `Notification`, `NotificationSubscriber` interface, `MatrixNotificationAdapter`, `FormatTimelineMessage`, `stepIcon`, `FormatBlockerMessage` |
| `event_reader.go` | `EventReader`, `NewEventReader`, `ReadNew`, `maxEventLogSize`, `ErrEventLogExceeded` |
| `cleanup.go` | `cleanupStateDir`, `stateDirExists` |
| `result.go` | `ContainerStepResult`, `ParseContainerStepResult`, `ParseExtendedStepResult`, `StepEvent`, `Blocker`, `SkillCandidate`, `ExtendedStepResult`, `EventsSummary`, `ReadEventsFile` |
| `task_scheduler.go` | `TaskScheduler`, `NewTaskScheduler`, `Start`, `Stop`, `tick`, `dispatchTask`, `templateDispatch`, `coldDispatch` |
| `types.go` | `TaskTemplate`, `Workflow`, `WorkflowStep`, `StepType`, `WorkflowStatus`, `ApprovalResult`, `ApprovalPolicy`, `ScheduledTask`, interface definitions (v0.7.0: `WorkflowStep.Input` field added) |
| `bridge/internal/events/matrix_event_bus.go` | `MatrixEventBus`, `MatrixEvent`, `Publish`, `GetEventsAfter`, `Subscribe` |
| `bridge/pkg/skills/extractor.go` | `ExtractFromResult`, `PatternCommandSequence`, `PatternFileTransform`, `PatternStepSequence`, `PatternCheckpointSequence`, `PatternConfigTemplate` |
| `bridge/pkg/skills/learned_store.go` | `LearnedStore`, `LearnedSkill`, `Save`, `FindForTask`, `RecordOutcome`, `Delete`, `ListForAgent` |
| `bridge/pkg/rpc/server.go` | `handleResolveBlocker` (resolve_blocker RPC handler) |
| `bridge/internal/adapter/commands_integration.go` | `CommandHandler`, `handleAgentSkills`, `handleAgentForgetSkill` |
| `bridge/internal/ai/compaction.go` | `EstimateMessageTokens`, `ShouldCompact`, `CompactHistory`, `CompactionThresholdTokens` |
| `container/openclaw/events.py` | `EventEmitter`, `StepEvent`, `EventType`, `PIPE_BUF` |
| `container/openclaw/step_runner.py` | `StepRunner`, `_extract_blockers_from_events`, `_summarize_events` |
| `container/openclaw/step_config.py` | `StepConfig`, `_blocker_response` property, `relevant_skills` property |

---

## Remaining Prerequisites

The backward communication channel (`result.json`), event streaming (`_events.jsonl`), blocker protocol, learned skills pipeline, parallel execution, compaction, and step failover are now implemented. Remaining gaps:

1. ~~**Shared state dir**: Container writes `result.json` to the bind-mounted state directory before exit~~ ✅ Done
2. ~~**Bridge reads result**: After container exit, Bridge reads and parses `result.json`~~ ✅ Done
3. ~~**Structured step results**: Multi-step workflows can pass data between steps via `result.json` `data` field — container handlers needed for each step type~~ ✅ Done (v0.7.0: `WorkflowStep.Input` field with template variable resolution)
4. ~~**PII socket wiring**: Secure PII delivery via Unix socket instead of environment variables~~ ✅ Done
5. ~~**Event streaming**: Containers emit StepEvents to `_events.jsonl`, Bridge tails for real-time progress~~ ✅ Done
6. ~~**Blocker protocol**: Human-in-the-loop blocker resolution with re-spawn~~ ✅ Done
7. ~~**Learned skills**: Extraction, persistence, injection, and outcome recording~~ ✅ Done
8. **Browser automation**: Handled by Jetski sidecar (separate container with network). Agent containers delegate browser operations to Jetski via the Bridge. No direct browser access from isolated containers.
9. ~~**Parallel step execution**: `StepParallelSplit`/`StepParallelMerge` with `errgroup` goroutine pool~~ ✅ Done (v0.6.0)
10. ~~**Step failover**: Multi-agent fallback with `FailoverRetry`/`FailoverImmediateFail` policies~~ ✅ Done (v0.6.0)
11. ~~**Session compaction**: Pre-dispatch token estimation and AI-powered history pruning~~ ✅ Done (v0.6.0)
