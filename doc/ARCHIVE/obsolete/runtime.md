# ArmorClaw Agent Runtime & Workflows
<!-- Source: agent-runtime.md, secretary-workflow.md, migration/workflow-step-input.md -->

## Table of Contents

- [Agent Runtime Internals](#agent-runtime-internals)
- [Secretary Workflow System](#secretary-workflow-system)
- [Migration: WorkflowStep Input Field](#migration-workflowstep-input-field)

---

## Agent Runtime Internals

> Part of the ArmorClaw System Documentation

> **Bridge-Side Architecture**
>
> The agent runtime and state machine described in this document are **Bridge-side only**. They run inside the Go Bridge process, not inside agent containers.
>
> **Container-to-Bridge reporting.** Containers in step mode now emit structured events to `_events.jsonl` during execution (via `EventEmitter` in `events.py`). The Bridge tails this file via `EventReader` for real-time progress. The 11-state state machine (`IDLE` through `OFFLINE`) is a Bridge-internal library used for lifecycle tracking. Containers cannot report which high-level state they are in. The Bridge observes: `running` (container exists) -> `completed` (exit 0) -> `failed` (exit non-zero), plus structured `StepEvent` entries from the event stream.
>
> **Backward channel:** Containers in step mode (STEP_CONFIG present) write structured results to `result.json` in the bind-mounted state dir before exit, and emit `StepEvent` entries to `_events.jsonl` throughout execution. The Bridge reads results via `ParseContainerStepResult()` (or `ParseExtendedStepResult()` for enriched output) and tails events via `EventReader`. See [Step Mode](#step-mode-step_config) below.
>
> Agent containers execute with `NetworkMode: "none"` and have zero network access. Communication: environment variables in, exit code + `result.json` + `_events.jsonl` out (step mode) or exit code only out (agent mode).

## Overview

The agent runtime is the in-process Go engine inside the Bridge that manages task execution, conversation memory, tool dispatch, and result caching. It operates below the container lifecycle (which is handled by `pkg/studio` via Docker) and above the AI provider layer. When the Bridge receives a task through the Matrix control plane, the runtime takes over: it routes the request, executes tool calls through the executor, caches results, and tracks progress step by step.

The runtime is *not* the same thing as the agent state machine documented in [Agent State Machine (Go Bridge)](architecture.md#agent-state-machine-go-bridge). That state machine tracks high-level agent phases like `BROWSING`, `FORM_FILLING`, and `AWAITING_APPROVAL`. This runtime handles the lower-level task loop: reasoning steps, tool invocations, speculative predictions, and memory persistence.

### Execution Modes

- **Mode A (Agent Studio)**: Containers spawned by `factory.Spawn()` with `NetworkMode: "none"`. Task delivered via `STEP_CONFIG` env var. Results via exit code + `result.json` in bind-mounted state dir (`/home/claw/.openclaw/`). When `STEP_CONFIG` is present, container runs in **step mode**: parses config, executes task, writes `result.json`, exits. When absent, runs in **agent mode** (Matrix polling loop). No network access. This is the default for secretary workflow steps.
- **Network-Isolated Execution**: Agent containers always run with `NetworkMode: "none"` and zero network access. Browser automation runs via the Jetski sidecar (separate container with network access, CDP proxy with PII scrubbing). LLM API calls are made by the Bridge process, not by agent containers. No proxy-out path exists for containers — all outbound network operations are handled by the Bridge or Jetski. See `jetski/` for the sidecar architecture.

## Architecture

```
                         ┌─────────────────────────────────────┐
                         │         Runtime                     │
                         │  (internal/agent/)                  │
                         │  Bridge-side only, not in container │
                         │                                     │
    Task config ────────▶│  Run(ctx, task)                     │
    (STEP_CONFIG env)    │    │                                │
                         │    ├─▶ Router.Route()               │
                         │    ├─▶ SpeculativeExecutor          │
                         │    │     (predictions)              │
                         │    ├─▶ ToolExecutor.Execute         │
                         │    │     (per tool call)            │
                         │    └─▶ build Result                 │
                         │                                     │
                         │  Wired at startup:                  │
                         │    Store  (memory)                  │
                         │    ToolCache (LRU + TTL)            │
                         └─────────────────────────────────────┘

    Bidirectional container communication (step mode):

    Task config (STEP_CONFIG env var) ──────▶ Container (NetworkMode: "none")
    Bridge polls Docker ContainerInspect ◀────── Container (exit code + result.json)
    Bridge tails _events.jsonl via EventReader ◀────── Container (StepEvent entries during execution)
```

## Step Mode (STEP_CONFIG)

When the Bridge sets `STEP_CONFIG` via `factory.go:Spawn()`, the container enters step mode instead of the default agent mode (Matrix polling loop). Step mode is the backward channel for Mode A containers.

**Flow:**
1. Bridge calls `factory.Spawn()` with `step.Config` → sets `STEP_CONFIG` env var
2. `entrypoint.py` detects `STEP_CONFIG` → imports `step_runner`
3. `step_runner.py` creates an `EventEmitter`, parses config via `step_config.py`, dispatches to a handler, emits events to `_events.jsonl`, writes `result.json`
4. Container exits (0 for success, 1 for failure)
5. Bridge's `waitForCompletion()` polls Docker (500ms interval) and tails `_events.jsonl` via `EventReader.ReadNew()`, then calls `ParseExtendedStepResult(stateDir)`

**Container-side modules** (in `container/openclaw/`):
- `step_config.py` — Parses `STEP_CONFIG` env var into `StepConfig` object. Provides `_blocker_response` and `relevant_skills` properties.
- `step_runner.py` — Executes step via handler (echo, transform, default), writes enriched result. Creates `EventEmitter` per step, merges blockers from config and events.
- `result_writer.py` — Atomic write of `result.json` (temp file + `os.rename()`)
- `events.py` — `EventEmitter` class. Writes `StepEvent` entries to `_events.jsonl`. Enforces `PIPE_BUF` (4096 bytes) atomic writes.

**result.json schema** (matches `ContainerStepResult` in `bridge/pkg/secretary/result.go`, enriched via `ExtendedStepResult`):
```json
{
  "status": "success",
  "output": "human-readable output",
  "data": {"key": "value"},
  "error": "error message if failed",
  "duration_ms": 1500,
  "_comments": ["optional annotations"],
  "_blockers": [{"blocker_type": "missing_input", "message": "...", "suggestion": "..."}],
  "_skill_candidates": [{"name": "...", "pattern_type": "...", "confidence": 0.7}],
  "_events_summary": {"total": 12, "types": {"step": 3, "command_run": 5}}
}
```

Base fields: `status` (string, required), `output` (string, required), `data` (map, omitempty), `error` (string, omitempty), `duration_ms` (int, required). Enriched underscore-prefixed fields: `_comments`, `_blockers`, `_skill_candidates`, `_events_summary`. Parsed by `ParseExtendedStepResult()` which also reads `_events.jsonl` for the full event list.

**Handlers:** Computation-only (no network). `echo` (for testing), `transform` (JSON-to-JSON), default (logs task received). Additional handlers require a separate plan with AI proxy socket.

### Observable Containers / Event Emission

Containers in step mode emit structured events to `_events.jsonl` throughout execution. This is implemented by the `EventEmitter` class in `events.py`.

**EventEmitter** (`container/openclaw/events.py`):

- Constructor takes a `state_dir` path and opens `_events.jsonl` for append.
- Each `emit()` call serializes a `StepEvent` dataclass as a single JSON line.
- `PIPE_BUF` (4096 bytes) enforcement: lines exceeding this limit are progressively truncated (detail replaced, then name shortened, then detail dropped). This guarantees atomic writes on Linux when the file is read concurrently by the Bridge.
- Convenience methods: `step()`, `file_read()`, `file_write()`, `file_delete()`, `command_run()`, `observation()`, `blocker()`, `error()`, `artifact()`, `progress()`, `checkpoint()`.
- `close()` writes a `_summary` event and closes the file handle.

**`_events.jsonl` format:** One JSON object per line. Schema matches Go `StepEvent` struct:

```json
{"seq": 1, "type": "step", "name": "processing data", "ts_ms": 500, "detail": {}, "duration_ms": null}
{"seq": 2, "type": "command_run", "name": "python transform.py", "ts_ms": 1200, "detail": {"exit_code": 0}, "duration_ms": 700}
```

**10 MB soft cap:** The Bridge's `EventReader` enforces a 10 MB limit. If `_events.jsonl` exceeds this, `ReadNew()` returns `ErrEventLogExceeded`. The calling code in `waitForCompletion()` logs a warning and stops tailing events, but does **not** kill the container. The container continues executing and finishes naturally via the normal Docker polling loop. After completion, `cleanupStateDir()` purges the oversized log. This soft cap preserves the container's output while preventing unbounded memory/disk growth from event tailing.

**Integration with StepRunner:** `StepRunner.run()` creates the `EventEmitter` as the first action and injects it into `step_config.config["_emitter_ref"]` so handlers can emit events without importing `events.py`. On completion, the runner reads `_events.jsonl` to extract blockers and event summaries for the enriched result.

### Blocker Protocol (Container Side)

Containers can signal that they need human input to proceed, distinct from the PII approval flow. Blockers handle missing input, ambiguous situations, or required decisions. PII approval gates access to sensitive data fields.

**Container signaling:**

1. The handler calls `emitter.blocker(blocker_type, message, suggestion, field)` to write a blocker event to `_events.jsonl`.
2. Alternatively, the handler appends to `step_config.config["_blockers"]` list.
3. On completion, `StepRunner` merges blockers from both sources into the enriched result.

**Bridge detection:**

1. `executeStepWithBlockerHandling()` in `orchestrator_integration.go` checks `ExtendedStepResult.Blockers` after the container exits.
2. If blockers are found, the workflow transitions to `StatusBlocked`.
3. The Bridge waits for a response via the `resolve_blocker` RPC or Matrix event.

**Container receives resolution:**

1. The Bridge calls `appendBlockerResponse()` to add `_blocker_response` to the step config.
2. `UnblockWorkflow()` transitions back to `StatusRunning`.
3. The container is re-spawned with the updated config.
4. The handler reads `step_config._blocker_response` property to get the response data (input, note, user_id, provided_at).

**PII safety distinction:** PII approval operates via `PendingApproval()` + `HandlePIIResponse()` with `app.armorclaw.pii_request`/`pii_response` Matrix events and a 120-second timeout. Blocker resolution operates via `waitForBlockerResponse()` + `DeliverBlockerResponse()` with a 10-minute timeout and up to 3 retries. Blocker input is never logged or written to disk, passed via environment variable only.

The `Runtime` struct in `internal/agent/runtime.go` holds all subsystems together:

| Field | Package | Role |
|-------|---------|------|
| `executor` | `internal/executor/` | Runs individual tool calls (shell, skills) |
| `cache` | `internal/cache/` | LRU cache for tool results with TTL eviction |
| `router` | `internal/router/` | Resolves which tools are available for a room/user |
| `memory` | `internal/memory/` | SQLite-backed message and context store |
| `speculative` | `internal/speculative/` | Pre-executes predicted tool calls |

The runtime connects to the container lifecycle like this:

1. **factory.Spawn** (in `pkg/studio`) creates a Docker container with the OpenClaw agent inside.
2. The Bridge sends a task to the agent via the Matrix room.
3. Inside the Bridge process, `Runtime.Run(ctx, task)` executes the task loop: route, predict, execute tools, collect results.
4. Steps accumulate on the `Task` object. Each step records its type (`reason`, `tool_call`, `tool_result`, `final`), duration, and output.
5. When the loop finishes (or hits `MaxSteps`), the runtime produces a `Result` with token usage, step count, and total duration.
6. **waitForCompletion** in the studio layer waits for the container to report back, then surfaces the `StepResult` to the Matrix room.

## Key Packages

### Runtime (internal/agent/)

**Files**: `runtime.go`, `types.go`

The runtime is the top-level coordinator. `NewRuntime(cfg)` wires together the executor, cache, router, memory store, and (optionally) the speculative executor. Defaults are applied for any zero-valued config fields.

**RuntimeConfig fields:**

| Field | Default | Purpose |
|-------|---------|---------|
| `MaxSteps` | 10 | Upper bound on reasoning steps per task |
| `MaxTokens` | 4096 | Token budget (checked against task usage) |
| `Timeout` | 30s | Per-task wall clock timeout |
| `EnableSpeculation` | true | Whether to pre-execute predicted tool calls |
| `MaxParallelTools` | 3 | Concurrency limit for tool execution |

**Task execution flow** (`Run` method):

1. Set task status to `running`, record start time, emit metrics.
2. Call `router.Route(roomID, userID)` to get available tools. If none, return immediately.
3. Loop up to `MaxSteps`:
   - Check for context cancellation.
   - Create a `reason` step and attach it to the task.
   - Generate predictions and feed them to the speculative executor.
   - Extract tool calls from the step.
   - Execute each tool call via `executor.Execute(ctx, call)`.
   - Record tool results as new steps on the task.
4. Build and return the final `Result`.

**Task model** (`types.go`):

A `Task` carries an ID, room ID, user ID, conversation history, step list, status, and metadata. Steps are typed: `reason`, `tool_call`, `tool_result`, `final`. Each step tracks its tool name, input, output, error, duration, and whether it was speculative.

Task statuses follow a simple lifecycle: `pending` -> `running` -> one of `completed`, `failed`, `cancelled`.

The `Result` struct captures the task ID, response text, tool call count, token usage breakdown (prompt / completion / total), wall clock duration, step count, and completion timestamp.

### Memory (internal/memory/)

**Files**: `store.go`, `checkpoint.go`, `batch.go`

The memory subsystem provides durable, per-room storage for conversation history and arbitrary key-value context. It backs onto SQLite (via the `modernc.org/sqlite` driver, no CGO required).

**Store** (`store.go`):

The `Store` wraps a `sql.DB` connection with a read-write mutex. It manages two tables:

- **`messages`**: Stores conversation turns keyed by room ID. Each message has an ID, role, content, timestamp, and serialized metadata. Queries are indexed on `room_id`. `GetMessages(roomID, limit)` returns messages in chronological order (the query fetches `DESC`, then the results are reversed in Go).
- **`contexts`**: A key-value store scoped to room ID. Uses `ON CONFLICT` upsert for `SetContext`. Supports get-by-key, get-all-for-room, delete, and full room clearing.

Both tables record metrics through `metrics.RecordMemoryOperation()` for every mutation and query.

`PruneMessages(olderThan)` deletes messages older than the given duration. `ClearRoom(roomID)` wipes all messages and context for a room.

**Checkpointer** (`checkpoint.go`):

Runs a background goroutine that issues `PRAGMA wal_checkpoint(TRUNCATE)` on a configurable interval (default: 5 minutes). On close, it performs one final checkpoint before exiting. This keeps the WAL file from growing unbounded under heavy write loads.

**BatchWriter** (`batch.go`):

Buffers messages in memory and flushes them to the store in batches. This reduces write amplification when the runtime is processing many messages in quick succession.

- **`maxBatch`**: Default 100. When the pending buffer reaches this size, a flush triggers immediately.
- **`interval`**: Default 1 second. A ticker also triggers periodic flushes.
- **`flushChan`**: A non-blocking signal channel. `Add(msg)` pushes to the buffer and signals the flush goroutine if the batch is full.

On close (`Close()`), the stop channel is closed and the goroutine performs a final flush before exiting.

### Cache (internal/cache/)

**Files**: `lru.go`, `ratelimit.go`

**LRU cache** (`lru.go`):

A generic least-recently-used cache with TTL-based expiration. Uses Go's `container/list` for eviction ordering and a map for O(1) lookups.

- **`maxSize`**: Default 1000 entries. When full, the oldest entry is evicted.
- **`defaultTTL`**: Default 5 minutes. Each entry tracks its `expiresAt` timestamp.
- **`onEvict`**: Optional callback fired when an entry is removed (by eviction, deletion, or clear).

`Get(key)` promotes the entry to the front of the eviction list. If the entry is expired, it is removed and returns `nil, false`.

`GetOrCompute(key, compute)` is a convenience that returns the cached value if present, otherwise calls the `compute` function and caches the result. This is the primary access pattern for tool results.

`PurgeExpired()` scans all entries and removes those past their TTL. Returns the count of purged entries.

**Rate limiter** (`ratelimit.go`):

A per-key rate limiter built on `golang.org/x/time/rate`. Each key gets its own token-bucket limiter, lazily created on first access.

- **`Rate`**: Default 10 tokens/second.
- **`Burst`**: Default 20 tokens.

`Allow(key)` is non-blocking: returns true if the request can proceed. `Wait(key)` blocks until a token is available. `WaitTimeout(key, timeout)` adds a deadline.

`SetRate` and `SetBurst` update the configuration for all existing limiters. `Remove(key)` cleans up a limiter that is no longer needed.

### Tool Executor (internal/executor/)

**File**: `engine.go`

The tool executor dispatches tool calls to their implementations. It wraps execution in a worker pool for concurrency control and routes calls through the security gateway.

**ToolExecutor** struct:

| Field | Purpose |
|-------|---------|
| `pool` | Worker pool that limits concurrent executions |
| `petg` | Security gateway that validates and filters tool calls |
| `skills` | Registry that resolves skill names to executable definitions |
| `timeout` | Per-call timeout (default 30s) |

**Execution flow:**

1. Look up the tool name in the skill registry. If unknown, reject immediately.
2. Pass the call through the security gateway (`petg.ValidateToolCall`) for policy checks.
3. Submit the call to the worker pool with a timeout context.
4. Record metrics (success or error) with timing.

**Worker pool** (`ToolPool`):

The pool starts with `MaxWorkers/2` goroutines (minimum 5 by default). Each worker reads from a buffered task channel. `ExecuteBatch` runs multiple calls concurrently and collects all results. The pool is closed by closing the task channel and waiting for workers to drain.

**Shell tool**: The only built-in tool is `shell`, which runs commands via `exec.CommandContext`. Output goes through `petg.FilterOutput` to strip any PII that might have leaked into tool output.

### Speculative Execution (internal/speculative/)

**File**: `executor.go`

The speculative executor pre-runs tool calls that the runtime predicts the task will need. This hides latency: when the actual tool call arrives, the result is already cached.

> **Note on Container Isolation**: Speculative execution pre-computes Go-side tool call results. However, the actual agent work happens inside containers with `NetworkMode: "none"` and no network access. The speculative cache is useful for Go-side operations (keystore lookups, approval checks) but cannot pre-compute results for container-internal LLM calls or browser operations, since those execute in isolation.

**SpeculativeExecutor** struct:

| Field | Purpose |
|-------|---------|
| `executor` | The underlying `ToolExecutor` for actual execution |
| `cache` | LRU cache for predicted results (default 1000 entries, 5 min TTL) |
| `predictions` | Map of call ID to prediction timestamp |
| `results` | Map of call ID to pre-computed `ToolResult` |
| `pendingCalls` | Queue of calls queued for prediction |

**Predict(ctx, call)**:

1. Check if a result is already cached for this call ID. If so, return it immediately (cache hit, metrics recorded).
2. If no cached result, execute the call through the underlying executor.
3. Store the result and record the prediction timestamp.
4. Return the result.

`AddPredictions(calls)` queues calls for prediction. The runtime calls this after generating predictions from the current step's route result.

`ExecuteBatch(ctx, calls)` runs predictions for multiple calls concurrently. Each call goes through `Predict`, and results are collected. If any prediction fails, `ErrPredictionFailed` is returned.

`ClearPredictions()` wipes all cached predictions, results, and pending calls. Called on close.

## Configuration

All configuration flows through `RuntimeConfig` in `internal/agent/runtime.go`. The runtime is created in the Bridge startup sequence and lives for the lifetime of the process.

| Parameter | Config Field | Default | Notes |
|-----------|-------------|---------|-------|
| Max steps per task | `RuntimeConfig.MaxSteps` | 10 | Prevents runaway reasoning loops |
| Max tokens per task | `RuntimeConfig.MaxTokens` | 4096 | Token budget for the full task |
| Task timeout | `RuntimeConfig.Timeout` | 30s | Wall clock limit per task |
| Speculative execution | `RuntimeConfig.EnableSpeculation` | true | Set false to disable prediction |
| Parallel tool limit | `RuntimeConfig.MaxParallelTools` | 3 | Worker pool size for tool calls |
| Tool cache size | Hardcoded in `NewRuntime` | 500 entries | ToolCache max size |
| Tool cache TTL | Hardcoded in `NewRuntime` | 10 min | ToolCache entry TTL |
| Memory DB path | `StoreConfig.Path` | `:memory:` | Set to file path for persistence |
| Checkpoint interval | `CheckpointerConfig.Interval` | 5 min | WAL checkpoint frequency |
| Batch write size | `BatchWriterConfig.MaxBatch` | 100 | Messages buffered before flush |
| Batch write interval | `BatchWriterConfig.Interval` | 1s | Periodic flush cadence |
| LRU cache size | `LRUConfig.MaxSize` | 1000 | Max cached entries |
| LRU cache TTL | `LRUConfig.DefaultTTL` | 5 min | Default entry lifetime |
| Rate limit (tokens/s) | `RateLimitConfig.Rate` | 10 | Per-key token bucket rate |
| Rate limit burst | `RateLimitConfig.Burst` | 20 | Per-key burst capacity |

## Integration Points

### Container Lifecycle

The runtime sits between the container factory and the Matrix control plane. The sequence:

1. `pkg/studio` calls `factory.Spawn()` to create an isolated Docker container running OpenClaw.
2. The Bridge creates a `Task` with the room ID, user ID, and conversation.
3. `Runtime.Run(ctx, task)` executes the task loop internally (routing, tool calls, speculation).
4. Steps accumulate on the task. Results flow back through the studio layer.
5. `waitForCompletion` blocks until the container reports its final `StepResult`.
6. The result is surfaced to the Matrix room as a message.

### Bridge Observable States vs State Machine States

The Bridge can observe four container states via Docker `ContainerInspect` and `_events.jsonl`:

| Bridge-Observable State | How Detected |
|------------------------|-------------|
| **Running** | Container exists and `State.Running == true` |
| **Completed** | Container exited with code 0 |
| **Failed** | Container exited with non-zero code, or container gone |
| **Events** | `StepEvent` entries in `_events.jsonl` (step, file ops, commands, observations, blockers, errors) |

The 11-state agent state machine (`IDLE`, `INITIALIZING`, `BROWSING`, `FORM_FILLING`, `AWAITING_CAPTCHA`, `AWAITING_2FA`, `AWAITING_APPROVAL`, `PROCESSING_PAYMENT`, `ERROR`, `COMPLETE`, `OFFLINE`) is defined in `bridge/pkg/agent/state.go`. Transitions are **programmatic only**: triggered by Bridge-side state inference (see below), not by agent-reported events. As of v0.6.0, `BroadcastStatus()` relays state transitions to clients via Matrix events (see [BroadcastStatus](#broadcaststatus-v060) below).

For the agent state machine definition (states like `BROWSING`, `AWAITING_APPROVAL`, `PROCESSING_PAYMENT`), see [Agent State Machine (Go Bridge)](architecture.md#agent-state-machine-go-bridge).

### State Inference (Bridge-Side) (v0.6.0)

Bridge-side state inference maps Chrome DevTools Protocol (CDP) events and workflow engine status to `AgentStatus` values. This runs entirely in the Bridge. It does **not** require container changes.

**Source**: `bridge/pkg/agent/state_inference.go`

**Inference priority order**:

| Priority | Source | Behavior |
|----------|--------|----------|
| 1 (highest) | Workflow side-channel overrides | Captcha, 2FA, payment, offline. These are intentional blocking states invisible to CDP. |
| 2 | Exit-driven states | Workflow completion: `exit_0` → `COMPLETE`, `exit_nonzero` → `ERROR` |
| 3 | Approval pinning | If currently `AWAITING_APPROVAL`, CDP events do **not** transition away. Approval is managed by `RequestPIIAccess` RPC. |
| 4 | CDP event-driven | `Page.frameNavigated` → `BROWSING`, `DOM.focus` on input elements → `FORM_FILLING`, `Runtime.executionContextCreated` → `INITIALIZING` (if `IDLE`/`OFFLINE`) |
| 5 (lowest) | Unknown events | Maintain current state. No transition. |

Uses `ForceTransition` on the `StateMachine` because inferred transitions may not follow the normal valid-transitions graph.

### BroadcastStatus (v0.6.0)

`BroadcastStatus()` is implemented (was previously a stub). It publishes `com.armorclaw.agent.status` Matrix events via the `MatrixEventBus` when state transitions occur.

**Source**: `bridge/pkg/agent/integration.go:359-384`

Event payload:

| Field | Description |
|-------|-------------|
| `status` | New `AgentStatus` value |
| `agent_id` | Agent identifier |
| `previous` | Prior state |
| `metadata` | `workflow_id`, `step`, `inferred_from` |
| `timestamp` | Event time |

Event routing registered in `bridge/internal/adapter/matrix.go` under `com.armorclaw.` prefix.

### Security Gateway (PETG)

Tool calls pass through `petg.ValidateToolCall(ctx, toolName, args)` before execution. This checks policies, rate limits, and PII interception rules. Tool output is filtered through `petg.FilterOutput()` to strip any secrets that leaked into results.

### Metrics

All subsystems emit metrics via `internal/metrics`:
- `RecordTaskStart` / `RecordTaskComplete` for task lifecycle
- `RecordToolCall(name, status, duration)` for tool execution
- `RecordMemoryOperation(op)` for store operations (insert, select, upsert, delete)
- `RecordSpeculativeCall(status)` for prediction hits and misses
- `RecordCacheHit` / `RecordCacheMiss` for cache effectiveness

### Memory Store

The memory store is shared across the runtime. Conversations and per-room context persist between tasks. The `BatchWriter` handles high-throughput ingestion, while the `Checkpointer` keeps the WAL file compact. For long-running agents, `PruneMessages` can be called periodically to cap storage growth.

### Cache Layer

The `ToolCache` (an LRU instance) sits in front of the executor. Tool results that are expensive to compute but deterministic can be served from cache. The speculative executor maintains its own separate LRU for predicted results, preventing cache pollution from speculative calls that never materialize.

### Proactive Compaction Hooks

The OpenClaw runtime provides two plugin hooks for triggering session compaction before context window overflow:

1. **`agent_end` hook** (primary): Fires after every successful LLM run with `{messages, success, error, durationMs}`. Gates on `success === true`, checks `estimateMessagesTokens(messages)` against the context window (~75% threshold), and calls compaction. Runs at natural task boundaries where summaries are most coherent.

2. **`before_prompt_build` hook** (safety net): Fires at each LLM call before the prompt is assembled. Checks token estimate against threshold. Catches long single-task sessions that never cross a task boundary. More disruptive than `agent_end` since it fires mid-task.

These hooks are OpenClaw-side (TypeScript) and do not require Bridge changes. The `agent_end` hook is the recommended primary trigger. See Context Management Architecture in armorclaw.md for the full three-tier approach.

## Agent State Visibility

### The 11 AgentStatus States

`bridge/pkg/agent/state.go` defines 11 states for the agent lifecycle state machine. These represent the full set of phases an agent can occupy during a task, from idle startup through browsing, form interaction, human-in-the-loop waits, payment, and termination.

| State | Constant | Description |
|-------|----------|-------------|
| `IDLE` | `StatusIdle` | No active task. Entry point and reset state. |
| `INITIALIZING` | `StatusInitializing` | Agent starting up. New JS context created. |
| `BROWSING` | `StatusBrowsing` | Navigating to or loading a URL. |
| `FORM_FILLING` | `StatusFormFilling` | Filling form fields (input, textarea, select). |
| `AWAITING_CAPTCHA` | `StatusAwaitingCaptcha` | Blocked on CAPTCHA. Needs human intervention. |
| `AWAITING_2FA` | `StatusAwaiting2FA` | Blocked on 2FA code. Needs human input. |
| `AWAITING_APPROVAL` | `StatusAwaitingApproval` | Blocked on BlindFill PII approval via `RequestPIIAccess` RPC. |
| `PROCESSING_PAYMENT` | `StatusProcessingPayment` | Submitting a payment. |
| `ERROR` | `StatusError` | Recoverable error. Can transition back to `IDLE` or `INITIALIZING`. |
| `COMPLETE` | `StatusComplete` | Task finished successfully. Returns to `IDLE`. |
| `OFFLINE` | `StatusOffline` | Agent not reachable. Terminal state, can only go to `INITIALIZING`. |

Valid transitions are defined in the `ValidTransitions` map. Terminal states (`AWAITING_CAPTCHA`, `AWAITING_2FA`, `AWAITING_APPROVAL`, `OFFLINE`) require external action to leave. Active states (`BROWSING`, `FORM_FILLING`, `INITIALIZING`, `PROCESSING_PAYMENT`) indicate the agent is working. User-action states (`AWAITING_CAPTCHA`, `AWAITING_2FA`, `AWAITING_APPROVAL`) need human input from ArmorChat.

### State Inference Engine (Not Wired in Production)

The Bridge contains a state inference engine in `bridge/pkg/agent/state_inference.go` that maps CDP events and workflow engine status to `AgentStatus` values. The function `InferAgentState()` applies a 4-priority resolution:

| Priority | Source | Maps to | Notes |
|----------|--------|---------|-------|
| 1 (highest) | Workflow side-channel | `AWAITING_CAPTCHA`, `AWAITING_2FA`, `PROCESSING_PAYMENT`, `OFFLINE` | Intentional blocking states invisible to CDP. |
| 2 | Exit-driven states | `COMPLETE` (exit 0), `ERROR` (exit non-zero) | Workflow engine reports `exit_0` or `exit_nonzero`. |
| 3 | Approval pinning | `AWAITING_APPROVAL` stays sticky | CDP events do not transition away from approval. Managed by `RequestPIIAccess` RPC. |
| 4 (lowest) | CDP events | `BROWSING`, `FORM_FILLING`, `INITIALIZING` | `Page.frameNavigated`, `DOM.focus` on inputs, `Runtime.executionContextCreated`. Unknown events maintain current state. |

The inference engine uses `ForceTransition` on the `StateMachine` because inferred transitions may not follow the normal valid-transitions graph. `ApplyInferredState()` is the convenience wrapper that combines inference with the actual state machine update.

**Current status**: The inference engine is implemented and tested but not wired into the production path. Jetski's CDP proxy records frames but does not feed them into `InferAgentState()`. The `BroadcastStatus()` method relays transitions to clients via Matrix events, but since the inference engine is disconnected, production state transitions are limited to programmatic calls from `Integration` methods.

### The Visibility Gap: Containers Cannot Report State

Agent containers run with `NetworkMode: "none"` and have zero network access. They cannot call back to the Bridge to report which `AgentStatus` they are in. The Bridge observes only three coarse states via Docker: `running` (container exists), `completed` (exit 0), and `failed` (exit non-zero), plus structured `StepEvent` entries from `_events.jsonl`.

From `bridge/pkg/agent/integration.go`:

> "Container agents cannot report their state. BroadcastStatus() is not yet implemented."

This means the 11-state state machine is a Bridge-side library. States advance based on Bridge-observed container lifecycle events (spawn, poll exit code) and programmatic API calls, not agent-reported phase transitions. The inference engine is the intended mechanism to bridge this gap by inferring fine-grained states from CDP events and workflow signals, but until it is wired in, only about 8 of the 11 states are reachable via the current programmatic path. The three unreachable states via inference alone are `AWAITING_CAPTCHA`, `AWAITING_2FA`, and `PROCESSING_PAYMENT` (these require the workflow side-channel, which is not connected). The `AWAITING_APPROVAL` state is reachable via the `RequestPIIAccess` RPC path.

### 8 Parallel State Enums

The codebase defines 8 distinct state enums across different packages. Each tracks a different lifecycle concern, and none are consolidated.

| # | Enum Type | Package | Values | Domain |
|---|-----------|---------|--------|--------|
| 1 | `AgentStatus` | `bridge/pkg/agent/` | 11 states (`IDLE` through `OFFLINE`) | Agent operational phase |
| 2 | `TaskStatus` | `bridge/internal/agent/` | 5 states: `pending`, `running`, `completed`, `failed`, `cancelled` | Runtime task lifecycle |
| 3 | `BrowserStatus` | `bridge/pkg/browser/` | 5 states: `idle`, `navigating`, `loading`, `ready`, `error` | Browser session state |
| 4 | `ServiceState` | `bridge/pkg/browser/` | 6 states: `IDLE`, `LOADING`, `FILLING`, `WAITING`, `PROCESSING`, `ERROR` | Browser-service HTTP client state |
| 5 | `BrowserState` | `bridge/pkg/studio/` | 6 states: `LOADING`, `FILLING`, `WAITING`, `PROCESSING`, `IDLE`, `ERROR` | Browser skill event protocol state |
| 6 | `JobStatus` (broker) | `bridge/pkg/browser/` | 7 states: `pending`, `running`, `paused`, `completed`, `failed`, `cancelled`, `awaiting_pii` | Jetski browser job queue |
| 7 | `JobStatus` (queue) | `bridge/pkg/queue/` | 7 states: `pending`, `running`, `paused`, `completed`, `failed`, `cancelled`, `awaiting_pii` | Browser job queue (separate package) |
| 8 | `InstanceStatus` | `bridge/pkg/studio/` | 6 states: `pending`, `running`, `paused`, `completed`, `failed`, `cancelled` | Agent instance (Docker container) lifecycle |
| 9 | `WorkflowStatus` | `bridge/pkg/secretary/` | 6 states: `pending`, `running`, `blocked`, `completed`, `failed`, `cancelled` | Secretary workflow execution |

**Note**: The task spec identified 8 enums, but source analysis reveals 9. `ServiceState` and `BrowserState` are separate enums in separate packages (`browser/` vs `studio/`) despite similar values. The two `JobStatus` types are also separate, defined independently in `browser/broker_types.go` and `queue/browser_queue.go`.

### Reachability Summary

Of the 11 `AgentStatus` states, not all are reachable through current production code paths:

| State | Reachable via | Path |
|-------|---------------|------|
| `IDLE` | Programmatic | Default start state, reset after completion/error |
| `INITIALIZING` | Programmatic + Inference (P4) | `Runtime.executionContextCreated` CDP event |
| `BROWSING` | Programmatic + Inference (P4) | `Integration.StartBrowsing()`, `Page.frameNavigated` CDP |
| `FORM_FILLING` | Programmatic + Inference (P4) | `Integration.UpdateProgress()`, `DOM.focus` on inputs |
| `AWAITING_CAPTCHA` | Programmatic + Inference (P1) | `Integration.WaitForCaptcha()`, workflow "captcha" side-channel (not wired) |
| `AWAITING_2FA` | Programmatic + Inference (P1) | `Integration.WaitFor2FA()`, workflow "twofa" side-channel (not wired) |
| `AWAITING_APPROVAL` | Programmatic | `Integration.RequestPIIAccess()`, inference P3 keeps it sticky |
| `PROCESSING_PAYMENT` | Programmatic + Inference (P1) | `Integration.StartPayment()`, workflow "payment" side-channel (not wired) |
| `ERROR` | Programmatic + Inference (P2) | `Integration.FailTask()`, exit non-zero |
| `COMPLETE` | Programmatic + Inference (P2) | `Integration.CompleteTask()`, exit zero |
| `OFFLINE` | Programmatic + Inference (P1) | Default when no state machine events, workflow "offline" side-channel |

The inference engine exists in `state_inference.go` but the Jetski CDP proxy does not feed frames into it. Until the inference path is connected, states like `AWAITING_CAPTCHA`, `AWAITING_2FA`, and `PROCESSING_PAYMENT` rely solely on explicit programmatic calls from `Integration` methods.

---

## Secretary Workflow System

> Part of the ArmorClaw System Documentation

Deep dive into ArmorClaw's workflow engine: scheduled tasks, multi-step workflows, and PII approval gates.

> **Source root:** `bridge/pkg/secretary/`

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

## Matrix Commands

The secretary workflow exposes learned skill management through Matrix commands, handled by `CommandHandler` in `bridge/internal/adapter/commands_integration.go`.

### Available commands

| Command | Usage | Description |
|---------|-------|-------------|
| `!agent skills <agent_id>` | `!agent skills researcher-1` | Lists learned skills for the agent. Shows name, confidence (0.0 to 1.0), and success count. Limited to 20 results. |
| `!agent forget-skill <agent_id> <skill_id>` | `!agent forget-skill researcher-1 ls_xxx_123` | Deletes a learned skill by ID. The agent_id parameter is accepted for future per-agent scoping but currently lists globally. |

Both commands require the `learnedStore` to be configured (non-nil) on the `CommandHandler`. If not available, they return an error message.

Source: `bridge/internal/adapter/commands_integration.go`

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

---

## Migration: WorkflowStep Input Field

> **Version:** 0.7.0  
> **Date:** 2026-04-18  
> **Status:** Backward compatible — no action required for existing templates

## What Changed

The `WorkflowStep` struct in `bridge/pkg/secretary/types.go` now includes an optional `Input` field:

```go
type WorkflowStep struct {
    // ... existing fields ...
    Input map[string]any `json:"input,omitempty"`
}
```

This field allows a step to declare data dependencies on outputs from previous steps, enabling inter-step data flow in workflow templates.

## Impact

### Existing Templates — No Changes Needed

Templates without an `input` field continue to work unchanged. The `omitempty` JSON tag ensures:

- **Serialization:** Steps with `nil` Input produce the same JSON as before (no `"input"` key).
- **Deserialization:** JSON without an `"input"` key produces a `nil` Input value.
- **Database:** Existing stored templates in SQLite round-trip correctly — no migration required.

### New Templates — Using the Input Field

To pass data from one step to another, add an `input` object to the consuming step:

```json
{
  "steps": [
    {
      "step_id": "step_1",
      "order": 0,
      "type": "action",
      "name": "Place Order"
    },
    {
      "step_id": "step_2",
      "order": 1,
      "type": "action",
      "name": "Send Confirmation",
      "input": {
        "order_id": "{{steps.step_1.data.order_id}}",
        "customer_email": "{{steps.step_1.data.email}}"
      }
    }
  ]
}
```

## Template Variable Syntax

The `Input` field supports template variable references using the `{{steps.<step_id>.data.<key>}}` syntax:

| Pattern | Description |
|---------|-------------|
| `{{steps.step_1.data.order_id}}` | Reference a value from a previous step's result data |
| `{{steps.fetch_user.data.profile.name}}` | Nested value from a previous step's result data |

The orchestrator (Task 13) will resolve these references at runtime by looking up `ContainerStepResult.Data` from the referenced step.

### Resolution Rules

1. References are resolved before step execution.
2. If a referenced step has not completed, the resolution fails and the step is blocked.
3. If a referenced key does not exist in the source step's `data`, the value resolves to `nil`.
4. Unresolved literal values (no `{{...}}` pattern) are passed through as-is.

## Examples

### Simple Data Passing

```json
{
  "step_id": "use_previous",
  "order": 1,
  "type": "action",
  "name": "Process Result",
  "input": {
    "result_url": "{{steps.scrape_page.data.url}}"
  }
}
```

### Empty Input (Explicit)

```json
{
  "step_id": "standalone",
  "order": 0,
  "type": "action",
  "name": "Independent Step",
  "input": {}
}
```

### Mixed Literal and Template Values

```json
{
  "step_id": "notify",
  "order": 2,
  "type": "action",
  "name": "Send Notification",
  "input": {
    "template": "order_confirmation",
    "recipient": "{{steps.step_1.data.customer_email}}",
    "order_ref": "{{steps.step_1.data.order_id}}"
  }
}
```

## API Changes

| Method | Change |
|--------|--------|
| `template.create` | Accepts optional `input` per step |
| `template.update` | Accepts optional `input` per step |
| `template.get` | Returns `input` when present |

## Testing

Backward compatibility is verified by:

- `TestWorkflowStep_InputField_BackwardCompat_NoInput` — JSON without `input` unmarshals with nil Input
- `TestWorkflowStep_InputField_WithInput` — JSON with `input` unmarshals correctly
- `TestWorkflowStep_InputField_EmptyInput` — JSON with `"input": {}` unmarshals as empty map
- `TestWorkflowStep_InputField_Omitempty` — nil Input is excluded from serialized JSON
