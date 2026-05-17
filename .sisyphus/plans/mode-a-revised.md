# Agent Studio Improvement Plan v3.1 — Observable Containers + Learned Skills

## TL;DR

> **Quick Summary**: Make Agent Studio containers produce structured execution events, stream progress to ArmorChat via Matrix, and persist learned skills — scoped to what Mode A containers can actually do (no network, deterministic computation). Blocker protocol, SIGKILL resource protection, and 7 network-dependent event types deferred to Mode B.
>
> **What changed from v3**: Phase 0 bug fixes added. Blocker protocol (5 tasks) cut. Factory.Kill() cut. StatusBlocked state cut. Event types slashed from 11 to 4. 10MB SIGKILL replaced with soft cap (stop tailing, log warning). Manual QA removed. Task 9.5 added (Matrix room event forwarding). Skill extraction threshold raised to 3+ distinct events. GetTimelineEvents() added for structured UI data. ~40% scope reduction.
>
> **Deliverables**:
> - Bridge EventReader with incremental tailing and soft 10MB cap
> - Container EventEmitter with 4 event types and PIPE_BUF enforcement
> - Progress streaming from container → Bridge → Matrix → ArmorChat
> - Learned skills extraction/persistence in SQLite
> - ArmorChat WorkflowTimeline + status indicator + Matrix commands
>
> **Estimated Effort**: L (5 phases + review, ~23 days realistic with Android uncertainty)
> **Parallel Execution**: YES — 3-4 waves per phase
> **Critical Path**: Phase 0 → Phase 1 (foundation) → Phase 2 (container emission) → Phase 3 (streaming) → Phase 4 (skills) → Phase 5 (client) → Review

## What Was Cut and Why

| Feature | v3 Task(s) | Reason for Deferral |
|---------|-----------|-------------------|
| Blocker protocol | T17-T21 | Containers have no network — nothing in Mode A needs external input (passwords, API keys). The blocker flow's PII safety guarantees were also unachievable (Docker overlay stores env vars). Revisit when Mode B containers make LLM calls that may require auth. |
| Factory.Kill() | T5 | Only needed for 10MB SIGKILL path. With soft cap, not needed. |
| StatusBlocked state | T6 | Only needed for blocker protocol. No consumers without blockers. |
| 7 event types | T7 (partial) | FILE_READ/WRITE/DELETE, COMMAND_RUN, OBSERVATION, BLOCKER — these describe network-dependent operations that Mode A containers don't perform. Kept: STEP, PROGRESS, CHECKPOINT, ERROR. |
| BlockerResponseDialog | T27 | No blocker protocol to respond to. |
| resolve_blocker RPC | T19, T29 | No blocker protocol. |
| _blocker_response/_retry parsing | T10 | No blocker protocol. |
| 10MB SIGKILL | T15 (partial) | With 4 event types, exceeding 10MB requires ~250,000 events — implausible. Changed to soft cap. |
| Manual QA (F3) | F3 | Contradicted ZERO HUMAN INTERVENTION policy. |

### Preserved for Mode B Reintegration

When Mode B containers have network access, the following can be added incrementally:
- Full 11-type event vocabulary (add FILE_*, COMMAND_RUN, OBSERVATION, BLOCKER to existing EventEmitter)
- Blocker protocol with PII-safe resolution (add executeStepWithBlockerHandling around existing step execution)
- StatusBlocked state (add to existing 5-state machine — transitions already documented in v3 T6)
- Factory.Kill() (add to existing AgentFactory — implementation already documented in v3 T5)
- 10MB SIGKILL path (add to existing EventReader cap check — change soft cap to hard kill)
- BlockerResponseDialog (add to ArmorChat — consumes workflow.blocked events from Matrix)
- resolve_blocker RPC (add to BridgeApi.kt and server.go — spec already documented in v3 T19)

All v3 task specifications remain valid reference material for Mode B reintegration. Nothing in this plan's changes makes v3's designs incorrect — they're premature, not wrong.

## Context

### Why v3 Was Revised

The v3 plan review identified that Mode A containers (`NetworkMode: "none"`) can only execute three handlers: `echo`, `transform`, and `default`. No LLM calls, no browser automation, no API integrations, no file system operations beyond the bind-mounted state dir. Building an 11-type event system, blocker protocol, and SIGKILL resource protection around containers that produce at most 3 lines of output was over-engineered.

Additionally, two production bugs were identified but not addressed in v3: warm dispatch silently fails (sends Matrix events to containers with no Matrix connection), and the Python sidecar's token interceptor crashes at runtime (async/sync mismatch).

This plan fixes both bugs, then scopes container observability to what Mode A can actually produce useful events about.

### Mode A Container Capabilities (Current)

| Handler | What It Does | Events It Would Produce |
|---------|-------------|----------------------|
| `echo` | Returns input as output | STEP (start/end) |
| `transform` | JSON-to-JSON conversion | STEP (start/end), PROGRESS (percent), ERROR (on bad input) |
| `default` | Logs task received, exits | STEP (start/end), ERROR (not implemented) |

Custom handlers (user-defined) can additionally produce CHECKPOINT events at arbitrary points and ERROR events on failures. They cannot produce FILE_* or COMMAND_RUN events because there is no network, no shell access, and no file system beyond the state dir.

---

## Work Objectives

### Core Objective
Make containers observable during execution, stream progress to ArmorChat, and learn from successful task patterns — within Mode A's actual capabilities.

### Concrete Deliverables
- `bridge/pkg/skills/learned_store.go` — SQLite-backed learned skills persistence
- `bridge/pkg/skills/extractor.go` — Skill extraction from completed task events
- `bridge/pkg/secretary/event_reader.go` — Incremental event file tailing with soft 10MB cap
- `bridge/pkg/secretary/cleanup.go` — State directory purge
- `container/openclaw/events.py` — EventEmitter with 4 event types and PIPE_BUF (4096 byte) enforcement
- Extended `result.json` with underscore-prefixed backward-compatible fields
- ArmorChat `WorkflowTimeline` composable
- Matrix commands `!agent skills`, `!agent forget-skill`

### Definition of Done
- [ ] `go test ./pkg/secretary/... ./pkg/skills/... ./internal/adapter/...` passes all new tests
- [ ] `python -m pytest container/tests/` passes all new tests
- [ ] Bridge tails `_events.jsonl` during execution, emits progress to MatrixEventBus
- [ ] `_events.jsonl` and state directory verified absent after task completion
- [ ] Learned skills persisted, injected as suggestions, confidence adjusts with outcomes
- [ ] Warm dispatch no longer silently fails
- [ ] Python sidecar token interceptor works in production

### Must Have
- PIPE_BUF enforcement (every event line ≤ 4096 bytes)
- Soft 10MB cap (stop tailing, log warning, container finishes normally)
- Purge ordering: parse → RemoveAll → notify
- Backward compatibility: old containers work with new Bridge, vice versa
- Learned skills: confidence ≥ 0.4 threshold, suggestions only (never auto-executed)
- Warm dispatch fix: skip when NetworkMode "none", fall through to cold dispatch
- Python interceptor fix: sync wrapper for sync gRPC server

### Must NOT Have
- No network access to containers (NetworkMode "none" preserved)
- No Matrix connectivity in containers
- No change to spawn-exit lifecycle
- No real-time streaming (500ms polling is the ceiling)
- No auto-execution of learned skills
- No change to base `result.json` fields (status, output, data, error, duration_ms)
- Handler function signature stays `(cfg) -> str` — EventEmitter injected via config dict key `_emitter_ref`, NOT as function parameter
- No WebSocket client added to ArmorChat — use Matrix /sync for event delivery
- No blocker protocol, no StatusBlocked state, no SIGKILL
- No PII in blocker responses (there are no blocker responses)

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES
- **Automated tests**: YES (Tests-after — comprehensive per task)
- **Framework**: Go `go test`, Python `pytest`, Kotlin JUnit

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

---

## Execution Strategy

```
Phase 0 (Bug Fixes — 2 tasks, 1 day):
├── Task 0.1: Fix warm dispatch silent failure [quick]
└── Task 0.2: Fix Python sidecar token interceptor [quick]
  Gate: Warm dispatch logs warning and falls to cold; Python interceptor doesn't crash

Phase 1 (Bridge Foundation — 4 tasks, 3 days):
├── Task 1:  SQLite learned_skills table + LearnedStore [quick]
├── Task 2:  EventReader with soft 10MB cap + incremental tailing [unspecified-high]
├── Task 3:  State directory cleanup utility [quick]
└── Task 4:  ExtendedStepResult types + ParseExtendedStepResult [unspecified-high]
  Gate: LearnedStore CRUD works, EventReader passes incremental tests,
        cleanupStateDir verified, extended parser reads new format

Phase 2 (Container Emission — 3 tasks, 2 days):
├── Task 5:  EventEmitter with 4 types + PIPE_BUF enforcement [unspecified-high]
├── Task 6:  Enriched result writer (write_enriched_result) [quick]
└── Task 7:  StepRunner integration + container tests [unspecified-high]
  Gate: Container writes _events.jsonl + extended result.json,
        Phase 1 ParseExtendedStepResult reads output correctly

Phase 3 (Progress Streaming — 4 tasks, 3 days):
├── Task 8:  Polling loop in waitForCompletion (tail _events.jsonl) [deep]
├── Task 9:  Progress event emission to MatrixEventBus [unspecified-high]
├── Task 9.5: Matrix room event forwarding [high]
└── Task 10: Timeline formatter + purge ordering [unspecified-high]
  Gate: Bridge emits workflow.progress to MatrixEventBus during execution,
        Task 9.5 forwards events to Matrix rooms, timeline posted to room after completion,
        state dir verified purged via stateDirExists()

Phase 4 (Learned Skills — 3 tasks, 4 days):
├── Task 11: Skill extraction pipeline (ExtractFromResult) [unspecified-high]
├── Task 12: Skill injection at dispatch (injectLearnedSkills) [quick]
└── Task 13: Post-completion extraction + RecordOutcome + integration tests [deep]
  Gate: Successful task extracts skills, future matching tasks receive suggestions,
        confidence adjusts with outcomes

### Task 11 Specification Update
- Changed "2+ STEP events" to "3+ STEP events with distinct names" to reduce noise
- Strategy 2 now requires 3+ distinct STEP events for pattern-based skill extraction

Phase 5 (Client Surface — 3 tasks, 4 days):
├── Task 14: ArmorChat WorkflowTimeline composable [visual-engineering]
├── Task 15: ArmorChat agent status indicator [quick]
└── Task 16: Matrix commands (!agent skills, !agent forget-skill) [unspecified-high]
  Gate: ArmorChat shows timeline during execution, skills manageable via Matrix

Wave REVIEW (After ALL phases — 3 parallel):
├── R1: Plan compliance audit (oracle)
├── R2: Code quality review (unspecified-high)
└── R3: Scope fidelity check (deep)

Critical Path: T0.1-T0.2 → T1-T4 → T5-T7 → T8-T9 → T9.5 → T10 → T11-T13 → T14-T16 → R1-R3
Max Concurrent: 4 (Phase 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 0.1 | None | 0.02 | 0 |
| 0.2 | 0.1 | Phase 1 | 0 |
| 1 | 0.2 | 11, 12, 16 | 1 |
| 2 | 0.2 | 8 | 1 |
| 3 | 0.2 | 10 | 1 |
| 4 | 0.2 | 10, 11 | 1 |
| 5 | None | 7 | 2 |
| 6 | None | 7 | 2 |
| 7 | 5, 6 | Phase 3 | 2 |
| 8 | 2, 7 | 9, 10 | 3 |
| 9 | 8 | Phase 4 | 3 |
| 9.5 | 9 | 10, 14 | 3 |
| 10 | 4, 8 | Phase 4 | 3 |
| 11 | 1, 4 | 12, 13, 16 | 4 |
| 12 | 1, 11 | 13 | 4 |
| 13 | 11, 12 | Phase 5 | 4 |
| 14 | 10 | R2 | 5 |
| 15 | None | R2 | 5 |
| 16 | 1, 11 | R1 | 5 |

---

## TODOs

- [x] 0.1. Fix Warm Dispatch Silent Failure

  **What to do**:
  - Modify `bridge/pkg/secretary/task_scheduler.go` function `warmDispatch()`
  - **Use the first approach**: check factory's default NetworkMode before attempting Matrix event dispatch. If the factory's default network mode is "none" (or if no running agent is found that has network), skip warm dispatch.
  - At the top of `warmDispatch()`, check whether the target container has network access. Since all Mode A containers use `NetworkMode: "none"`, and warm dispatch sends Matrix events that containers cannot receive, add an explicit check.
  - Implementation: Add a log message at WARN level: `"warm dispatch skipped: container has no network access, falling through to cold dispatch"`. Then call `coldDispatch()` and return.
  - Check `bridge/pkg/studio/factory.go` for how network mode is configured in SpawnRequest.
  - Create/extend test verifying that warm dispatch logs warning and falls to cold when NetworkMode is "none"

  **Must NOT do**:
  - Do NOT try to fix container Matrix connectivity — that requires network access
  - Do NOT remove warm dispatch entirely — it may work under Mode B
  - Do NOT change the cold dispatch path

  **Recommended Agent Profile**: `quick`
  **Parallelization**: NO (first task, unblocks Phase 1)
  **Blocked By**: None
  **Blocks**: Task 0.2

  **References**:
  - `bridge/pkg/secretary/task_scheduler.go` — `warmDispatch()` and `coldDispatch()` functions
  - `bridge/pkg/studio/factory.go` — SpawnRequest.NetworkMode field

  **Acceptance Criteria**:
  - [ ] `go test -v ./pkg/secretary/... -run TestWarmDispatch` → PASS
  - [ ] Warm dispatch logs WARN when container has no network access
  - [ ] Cold dispatch is called as fallback
  - [ ] Existing warm dispatch behavior unchanged when network is available (if tested)

  **QA Scenarios**:
  ```
  Scenario: Warm dispatch falls to cold on no-network container
    Tool: Bash (go test)
    Steps:
      1. Mock factory with NetworkMode "none", running instance exists
      2. Call warmDispatch for a scheduled task
      3. Verify WARN log message about skipped warm dispatch
      4. Verify coldDispatch was called
    Expected Result: Warning logged, cold dispatch executed
    Evidence: .sisyphus/evidence/task-0-1-warm-fallback.txt
  ```

  **Commit**: YES
  - Message: `fix(secretary): skip warm dispatch for no-network containers`
  - Files: `bridge/pkg/secretary/task_scheduler.go, bridge/pkg/secretary/task_scheduler_test.go`

- [x] 0.2. Fix Python Sidecar Token Interceptor

  **What to do**:
  - Modify `sidecar-python/interceptor.py` — the production `TokenInterceptor` uses `grpc_aio.ServerInterceptor` (async) but `worker.py` uses sync `grpc.server()`. This causes `AttributeError` at runtime.
  - Fix: Rewrite `TokenInterceptor` to use `grpc.ServerInterceptor` (sync) instead of `grpc_aio.ServerInterceptor` (async). The sync interceptor signature is `def intercept_service(continuation, handler_call_details)`.
  - The test file already has a `_SyncTokenInterceptor` that works — use it as the reference implementation for the fix.
  - Keep all validation logic (HMAC-SHA256, TTL, request ID) unchanged.
  - Verify existing 12 tests in `test_interceptor.py` still pass after the change.

  **Must NOT do**:
  - Do NOT switch worker.py to async `grpc.aio.server()` — that's a larger change
  - Do NOT remove the token validation logic
  - Do NOT change the token format or validation rules

  **Recommended Agent Profile**: `quick`
  **Parallelization**: NO (follows Task 0.1)
  **Blocked By**: Task 0.1
  **Blocks**: Phase 1

  **References**:
  - `sidecar-python/interceptor.py` — Production interceptor (broken)
  - `sidecar-python/test_interceptor.py` — `_SyncTokenInterceptor` (working reference)

  **Acceptance Criteria**:
  - [ ] `python -m pytest sidecar-python/test_interceptor.py -v` → PASS (12 tests)
  - [ ] `TokenInterceptor` uses `grpc.ServerInterceptor` not `grpc_aio.ServerInterceptor`
  - [ ] Token validation logic unchanged

  **QA Scenarios**:
  ```
  Scenario: Sync interceptor doesn't crash on startup
    Tool: Bash (pytest)
    Steps:
      1. Import worker.py module (triggers interceptor registration)
      2. Verify no AttributeError raised
    Expected Result: Clean import, no crash
    Evidence: .sisyphus/evidence/task-0-2-interceptor-fix.txt
  ```

  **Commit**: YES
  - Message: `fix(sidecar-python): use sync gRPC interceptor for sync server`
  - Files: `sidecar-python/interceptor.py`
  - Pre-commit: `python -m pytest sidecar-python/test_interceptor.py -v`

- [x] 1. SQLite Learned Skills Table + LearnedStore

  **What to do**:
  - Create `bridge/pkg/skills/learned_store.go` with `LearnedStore` struct backed by `*sql.DB`
  - Add `learned_skills` table DDL to secretary store's `initSchema()` in `bridge/pkg/secretary/store.go`
  - Table schema: `id TEXT PK, name TEXT UNIQUE, description TEXT, source_task_id TEXT, source_template_id TEXT, pattern_type TEXT NOT NULL, pattern_data TEXT NOT NULL, trigger_keywords TEXT NOT NULL, success_count INT DEFAULT 0, failure_count INT DEFAULT 0, last_used_at INT, created_at INT NOT NULL, confidence REAL DEFAULT 0.5`
  - Index `idx_learned_confidence ON learned_skills(confidence)`
  - Implement `NewLearnedStore(db *sql.DB)`, `Save(LearnedSkill)`, `FindForTask(taskDesc, limit)`, `RecordOutcome(skillID, bool)`, `Delete(skillID)`, `ListForAgent(limit)`
  - `FindForTask`: WHERE confidence >= 0.4, rank by keyword overlap
  - 12 tests in `learned_store_test.go`

  **Must NOT do**:
  - Do NOT put table in SQLCipher keystore — secretary store is plain SQLite
  - Do NOT auto-execute skills

  **Security Trade-Off (Documented)**:
  Learned skills stored in plain SQLite (not SQLCipher). Skills contain execution patterns, no secrets. Migratable to SQLCipher if future compliance requires it.

  **Recommended Agent Profile**: `quick`
  **Parallelization**: YES (Wave 1, with Tasks 2-4)
  **Blocked By**: Task 0.2
  **Blocks**: Tasks 11, 12, 16

  **References**:
  - `bridge/pkg/secretary/store.go:114` — Secretary SQLite init pattern
  - `bridge/pkg/secretary/store.go:240-280` — v1→v2 migration pattern
  - `bridge/pkg/secretary/types.go` — Domain types pattern

  **Acceptance Criteria**:
  - [ ] `go test -v ./pkg/skills/... -run TestLearnedStore` → PASS (12 tests)
  - [ ] Save/FindForTask/RecordOutcome round-trip correctly
  - [ ] Skills with confidence < 0.4 excluded from FindForTask

  **QA Scenarios**:
  ```
  Scenario: Save and retrieve a learned skill
    Tool: Bash (go test)
    Steps:
      1. Save LearnedSkill{Name: "db-migrate", PatternType: "command_sequence", TriggerKeywords: ["migrate","database"], Confidence: 0.7}
      2. Call FindForTask("migrate the production database", 5)
    Expected Result: Returns exactly 1 skill with name "db-migrate"
    Evidence: .sisyphus/evidence/task-1-save-retrieve.txt

  Scenario: Confidence threshold filtering
    Tool: Bash (go test)
    Steps:
      1. Save 3 skills: confidence 0.3, 0.6, 0.8
      2. Call FindForTask("any task", 10)
    Expected Result: Returns only 2 skills (0.6 and 0.8)
    Evidence: .sisyphus/evidence/task-1-confidence-threshold.txt
  ```

  **Commit**: YES
  - Message: `feat(skills): add learned_skills table and LearnedStore`
  - Files: `bridge/pkg/skills/learned_store.go, bridge/pkg/skills/learned_store_test.go, bridge/pkg/secretary/store.go`

- [x] 2. EventReader with Soft 10MB Cap + Incremental Tailing

  **What to do**:
  - Create `bridge/pkg/secretary/event_reader.go` with `EventReader` struct
  - `NewEventReader(stateDir string) *EventReader`
  - `ReadNew() ([]StepEvent, int64, error)` — returns new events since last call, file size, error
  - Track `byteOffset int64` and `lastSeq int` for incremental reads
  - **Soft cap**: if `os.Stat()` shows > 10MB, set internal `capExceeded bool` flag, log WARN, return empty slice with no error. Subsequent calls also return empty while flag is set.
  - File at exactly 10MB does NOT trigger cap — only strictly greater than
  - Skip malformed JSON lines, skip lines with seq <= lastSeq, skip comment lines and empty lines
  - 9 tests in `event_reader_test.go`

  **Must NOT do**:
  - Do NOT read entire file each call — use Seek to byteOffset
  - Do NOT return error on cap exceeded — return empty, log warning
  - Do NOT trigger at exactly 10MB — only strictly greater than

  **Recommended Agent Profile**: `unspecified-high`
  **Parallelization**: YES (Wave 1, with Tasks 1, 3, 4)
  **Blocked By**: Task 0.2
  **Blocks**: Task 8

  **References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:321-349` — waitForCompletion() polling loop
  - `bridge/pkg/secretary/result.go:39` — ContainerStepResult pattern

  **Acceptance Criteria**:
  - [ ] `go test -v ./pkg/secretary/... -run TestEventReader` → PASS (9 tests)
  - [ ] Reads incrementally across 5+ poll cycles with zero duplication
  - [ ] File at exactly 10MB does NOT trigger cap
  - [ ] File at 10MB+1 byte DOES trigger cap (CapExceeded returns true, ReadNew returns empty)
  - [ ] Missing file returns nil, nil (not error)

  **QA Scenarios**:
  ```
  Scenario: Incremental reads with offset tracking
    Tool: Bash (go test)
    Steps:
      1. Create EventReader, _events.jsonl with 5 lines
      2. ReadNew() → 5 events
      3. Append 3 lines
      4. ReadNew() → exactly 3 new events
      5. ReadNew() → 0 events
    Expected Result: Zero duplication across cycles
    Evidence: .sisyphus/evidence/task-2-incremental.txt

  Scenario: Soft cap stops tailing without error
    Tool: Bash (go test)
    Steps:
      1. Create EventReader, file at 10MB+1
      2. ReadNew() → empty slice, nil error
      3. CapExceeded() → true
    Expected Result: No error, cap flag set, empty return
    Evidence: .sisyphus/evidence/task-2-soft-cap.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add EventReader with soft 10MB cap`
  - Files: `bridge/pkg/secretary/event_reader.go, bridge/pkg/secretary/event_reader_test.go`

- [x] 3. State Directory Cleanup Utility

  **What to do**:
  - Create `bridge/pkg/secretary/cleanup.go`
  - `cleanupStateDir(stateDir string) error` — `os.RemoveAll(stateDir)`, log error but don't fail workflow
  - Empty string = no-op, nonexistent path = no error
  - `stateDirExists(stateDir string) bool` — checks for `_events.jsonl` in dir
  - 4 tests

  **Recommended Agent Profile**: `quick`
  **Parallelization**: YES (Wave 1)
  **Blocked By**: Task 0.2
  **Blocks**: Task 10

  **Acceptance Criteria**:
  - [ ] `go test -v ./pkg/secretary/... -run TestCleanup` → PASS (4 tests)
  - [ ] cleanupStateDir removes directory, stateDirExists returns false after

  **Commit**: YES
  - Message: `feat(secretary): add state dir cleanup utility`
  - Files: `bridge/pkg/secretary/cleanup.go, bridge/pkg/secretary/cleanup_test.go`

- [x] 4. ExtendedStepResult Types + ParseExtendedStepResult

  **What to do**:
  - Modify `bridge/pkg/secretary/result.go` to add:
    - `StepEvent` struct: `Seq int`, `Type string`, `Name string`, `TsMs int`, `Detail map[string]interface{}`, `DurationMs *int`
    - `SkillCandidate` struct: `Name string`, `Description string`, `PatternType string`, `PatternData string`, `Confidence float64`
    - `EventsSummary` struct: `Total int`, `Types map[string]int`
    - `ExtendedStepResult` embedding `*ContainerStepResult` plus: `Comments []string`, `SkillCandidates []SkillCandidate`, `EventsSummary *EventsSummary`, `Events []StepEvent` (`json:"-"`)
  - **Removed from v3**: `Blocker` struct and `Blockers` field — blocker protocol deferred
  - `ParseExtendedStepResult(stateDir)`: call existing ParseContainerStepResult, parse underscore fields (`_comments`, `_skill_candidates`, `_events_summary`), load `_events.jsonl`
  - `ReadEventsFile(stateDir) ([]StepEvent, error)`
  - 4 tests

  **Must NOT do**:
  - Do NOT change existing ContainerStepResult or ParseContainerStepResult
  - Do NOT add Blocker types

  **Recommended Agent Profile**: `unspecified-high`
  **Parallelization**: YES (Wave 1)
  **Blocked By**: Task 0.2
  **Blocks**: Tasks 10, 11

  **References**:
  - `bridge/pkg/secretary/result.go:39` — ContainerStepResult (embed, don't modify)
  - `container/openclaw/result_writer.py:20-31` — Python result schema

  **Acceptance Criteria**:
  - [ ] `go test -v ./pkg/secretary/... -run TestParseExtended` → PASS (4 tests)
  - [ ] New format → all fields parsed
  - [ ] Old format → base result returned, no error
  - [ ] Missing _events.jsonl → Events nil, no error

  **Commit**: YES
  - Message: `feat(secretary): add ExtendedStepResult types and parser`
  - Files: `bridge/pkg/secretary/result.go, bridge/pkg/secretary/result_test.go`

- [x] 5. EventEmitter with 4 Types + PIPE_BUF Enforcement

  **What to do**:
  - Create `container/openclaw/events.py` with `EventEmitter` class
  - **4 event types only**: STEP, PROGRESS, CHECKPOINT, ERROR
  - `StepEvent` dataclass: `seq int`, `type str`, `name str`, `ts_ms int`, `detail dict`, `duration_ms Optional[int]`
  - `__init__(state_dir)` — opens `_events.jsonl`, writes header `# Agent Studio events`, inits `_seq=0`, `_start_ms` from `time.monotonic()`
  - `emit(event_type, name, detail=None, duration_ms=None)` — serialize to JSON+newline, enforce PIPE_BUF: if line > 4096 bytes, replace detail with `{"_truncated": True, "_original_size": N}`. If STILL > 4096, truncate name to 64 chars.
  - Convenience methods: `step(name, detail=None)`, `progress(percent, message=None)`, `checkpoint(name, detail=None)`, `error(message, detail=None)`
  - `close()` — emits `_summary` event with `total_events` and `total_ms`
  - **Deferred types** (documented as comment for Mode B): FILE_READ, FILE_WRITE, FILE_DELETE, COMMAND_RUN, OBSERVATION, BLOCKER, ARTIFACT
  - 7 tests in `container/openclaw/tests/test_events.py`

  **Must NOT do**:
  - Do NOT implement deferred event types
  - Do NOT use print() — write to _events.jsonl
  - Do NOT allow any line > 4096 bytes

  **Recommended Agent Profile**: `unspecified-high`
  **Parallelization**: YES (Wave 2, with Task 6)
  **Blocked By**: None
  **Blocks**: Task 7

  **References**:
  - `container/openclaw/result_writer.py:20-31` — Atomic write pattern
  - `bridge/pkg/secretary/result.go` — StepEvent Go struct (wire contract)

  **Acceptance Criteria**:
  - [ ] `python -m pytest container/tests/test_events.py -v` → PASS (7 tests)
  - [ ] Every line in _events.jsonl parses as valid JSON
  - [ ] PIPE_BUF: large detail produces truncated line with `_truncated` marker
  - [ ] close() emits _summary with correct count

  **QA Scenarios**:
  ```
  Scenario: PIPE_BUF enforcement
    Tool: Bash (pytest)
    Steps:
      1. Create EventEmitter
      2. Emit event with 5000-byte detail
      3. Read _events.jsonl, parse lines
    Expected Result: Large event has detail._truncated == true, all lines parse
    Evidence: .sisyphus/evidence/task-5-pipebuf.txt

  Scenario: 4 types produce valid JSON
    Tool: Bash (pytest)
    Steps:
      1. Call step(), progress(50, "halfway"), checkpoint("midpoint"), error("oops")
      2. Close emitter
      3. Parse all lines
    Expected Result: 4 event lines + 1 _summary, all valid JSON with correct type field
    Evidence: .sisyphus/evidence/task-5-four-types.txt
  ```

  **Commit**: YES
  - Message: `feat(container): add EventEmitter with 4 types and PIPE_BUF`
  - Files: `container/openclaw/events.py, container/openclaw/tests/test_events.py`

- [x] 6. Enriched Result Writer (write_enriched_result)

  **What to do**:
  - Modify `container/openclaw/result_writer.py` to add `write_enriched_result()`
  - Signature: `write_enriched_result(state_dir, status, output, data=None, error=None, duration_ms=0, comments=None, skill_candidates=None, events_summary=None)`
  - **Removed from v3**: `blockers` parameter and `_blockers` field
  - Append underscore-prefixed fields only when non-empty: `_comments`, `_skill_candidates`, `_events_summary`
  - Use existing `_atomic_write_json()`
  - Existing `write_result()` unchanged
  - 3 tests

  **Must NOT do**:
  - Do NOT include _blockers field
  - Do NOT modify existing write_result()

  **Recommended Agent Profile**: `quick`
  **Parallelization**: YES (Wave 2)
  **Blocked By**: None
  **Blocks**: Task 7

  **Acceptance Criteria**:
  - [ ] `python -m pytest container/tests/test_result_writer.py -v` → PASS (3+ tests)
  - [ ] Non-empty underscore fields present, empty ones absent
  - [ ] Base 5 fields always present

  **Commit**: YES
  - Message: `feat(container): add enriched result writer`
  - Files: `container/openclaw/result_writer.py, container/openclaw/tests/test_result_writer.py`

- [x] 7. StepRunner Integration + Container Tests

  **What to do**:
  - Modify `container/openclaw/step_runner.py` to integrate EventEmitter
  - Create EventEmitter at start of `run_step()`, close in `finally`
  - Inject into config dict: `config["_emitter_ref"] = emitter`, `config["_comments"] = []`
  - **Removed from v3**: `config["_blockers"] = []`, blocker extraction from events, `_retry`/`_blocker_response`/`relevant_skills` logging
  - After handler returns: extract comments from config, call `_summarize_events()`, call `write_enriched_result()`
  - On exception: emit error event, write enriched result with status="failed"
  - Handler signature stays `(cfg) -> str`
  - State dir path includes instance ID (check factory.Spawn config)
  - Integration tests: 5 tests in `test_step_runner.py`
  - Cross-component test: verify Go ParseExtendedStepResult can read Python output

  **Must NOT do**:
  - Do NOT change handler signature
  - Do NOT add blocker-related config keys
  - Do NOT fail step if EventEmitter ops fail (best-effort)

  **Recommended Agent Profile**: `unspecified-high`
  **Parallelization**: NO (Wave 2, after Tasks 5+6)
  **Blocked By**: Tasks 5, 6
  **Blocks**: Phase 3

  **References**:
  - `container/openclaw/step_runner.py` — existing step runner
  - `container/openclaw/tests/test_step_runner.py` — existing tests

  **Acceptance Criteria**:
  - [ ] `python -m pytest container/tests/test_step_runner.py -v` → PASS (5 tests)
  - [ ] Events written through config dict accessor
  - [ ] Cross-component parsing works (Go can read Python output)

  **Commit**: YES
  - Message: `feat(container): integrate EventEmitter via config dict`
  - Files: `container/openclaw/step_runner.py, container/openclaw/tests/test_step_runner.py`

- [x] 8. Polling Loop in waitForCompletion (Tail _events.jsonl)

  **What to do**:
  - Modify `bridge/pkg/secretary/orchestrator_integration.go` `waitForCompletion()`
  - Create EventReader at start, use 500ms ticker (existing pattern)
  - On each tick: read Docker status, check completion, then `reader.ReadNew()`
  - For each new event: route by type — STEP/CHECKPOINT → EmitProgress, ERROR → EmitStepError, PROGRESS → EmitProgress
  - On container completion: `ParseExtendedStepResult()` → `cleanupStateDir()` → send timeline/comment notifications from parsed data in memory
  - On `reader.CapExceeded()`: log WARN, continue polling Docker status (no tailing), still parse result on completion
  - On context cancellation: cleanupStateDir, return error

  **Must NOT do**:
  - Do NOT call factory.Kill() — no Factory.Kill() exists
  - Do NOT change 500ms interval
  - Do NOT block on event reading

  **Recommended Agent Profile**: `deep`
  **Parallelization**: NO (Wave 3, start)
  **Blocked By**: Tasks 2, 7
  **Blocks**: Tasks 9, 10

  **References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:321-349` — waitForCompletion() (EXACT function)
  - `bridge/pkg/secretary/event_reader.go` (Task 2) — ReadNew() and CapExceeded() APIs

  **Acceptance Criteria**:
  - [ ] `go test -v ./pkg/secretary/... -run TestTail` → PASS
  - [ ] Zero duplicate events across 5+ poll cycles
  - [ ] On completion: parse → purge → notify ordering
  - [ ] On cap exceeded: warning logged, Docker polling continues

  **QA Scenarios**:
  ```
  Scenario: Incremental tail during execution
    Tool: Bash (go test)
    Steps:
      1. Mock factory returning "running", temp _events.jsonl growing 2 events/tick
      2. Tick 1: 2 events → Tick 2: append 2, read 2 → Tick 3: set "completed"
    Expected Result: Exactly 2 new events per tick, total 4
    Evidence: .sisyphus/evidence/task-8-tail.txt

  Scenario: Soft cap continues Docker polling
    Tool: Bash (go test)
    Steps:
      1. _events.jsonl at 10MB+1
      2. ReadNew() returns empty, CapExceeded true
      3. Docker polling continues, container exits normally
      4. Result still parsed on completion
    Expected Result: No kill, warning logged, normal completion path
    Evidence: .sisyphus/evidence/task-8-soft-cap-poll.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add event streaming in waitForCompletion`
  - Files: `bridge/pkg/secretary/orchestrator_integration.go, bridge/pkg/secretary/orchestrator_integration_test.go`

- [x] 9. Progress Event Emission to MatrixEventBus

  **What to do**:
  - Modify `bridge/pkg/secretary/orchestrator_events.go` to add methods on WorkflowEventEmitter:
    - `EmitProgress(workflowID string, event StepEvent)` — type `workflow.progress`, extract percent from detail
    - `EmitStepError(workflowID string, event StepEvent)` — type `workflow.step_error`
  - **Removed from v3**: `EmitBlockerWarning()`
  - `ProgressDetail` struct: `EventSeq int`, `EventType string`, `StepName string`, `ElapsedMs int`, `Detail map[string]interface{}`
  - Publish via existing `e.bus.Publish()` on MatrixEventBus

  **Must NOT do**:
  - Do NOT create parallel event system — extend existing WorkflowEventEmitter
  - Do NOT use pkg/eventbus/EventBus — use MatrixEventBus only

  **Recommended Agent Profile**: `unspecified-high`
  **Parallelization**: NO (Wave 3, after Task 8)
  **Blocked By**: Task 8
  **Blocks**: Phase 4

  **References**:
  - `bridge/internal/events/matrix_event_bus.go` — MatrixEventBus.Publish()
  - `bridge/pkg/secretary/orchestrator_events.go` — Existing WorkflowEventEmitter with bus field

  **Acceptance Criteria**:
  - [ ] `go test -v ./pkg/secretary/... -run TestEmitProgress` → PASS
  - [ ] EmitProgress publishes with correct type and percent
  - [ ] EmitStepError publishes with error detail

  **Commit**: YES
  - Message: `feat(secretary): add progress event emission to Matrix`
  - Files: `bridge/pkg/secretary/orchestrator_events.go, bridge/pkg/secretary/orchestrator_events_test.go`

- [x] 9.5. Matrix Room Event Forwarding

  **What to do**:
  - This is the critical missing bridge between internal MatrixEventBus (ring buffer) and actual Matrix room messages that ArmorChat can receive via /sync.
  - Modify `bridge/internal/adapter/commands_integration.go` or the Bridge's event processing loop to subscribe to `workflow.*` events from MatrixEventBus and forward them as Matrix room messages.
  - Implementation approach:
    1. Subscribe to `workflow.progress` and `workflow.step_error` event types from MatrixEventBus
    2. For each event: publish as `m.room.message` with `msgtype: m.notice` to the workflow's associated Matrix room
    3. Use existing Matrix client (already used for command handling) to send messages
  - The workflow room ID is available from the orchestrator's workflow state (each workflow has an associated Matrix room)
  - After completion: post formatted timeline to room using `FormatTimelineMessage()` from Task 10
  - 4 tests: subscription setup, progress forwarding, error forwarding, timeline posting on completion

  **Must NOT do**:
  - Do NOT modify ArmorChat's processEvents() — that's a separate concern
  - Do NOT create a WebSocket — Matrix /sync is the transport
  - Do NOT forward blocker events (they don't exist in v3.1)

  **Recommended Agent Profile**: `unspecified-high`
  **Parallelization**: NO (Wave 3, after Task 9)
  **Blocked By**: Task 9
  **Blocks**: Tasks 10, 14

  **References**:
  - `bridge/internal/events/matrix_event_bus.go` — MatrixEventBus.Subscribe() pattern
  - `bridge/internal/adapter/commands_integration.go` — Matrix client for sending messages
  - `bridge/pkg/secretary/orchestrator_events.go` — Event types published by Task 9

  **Acceptance Criteria**:
  - [ ] `go test -v ./pkg/secretary/... -run TestMatrixForwarding` → PASS (4 tests)
  - [ ] workflow.progress events appear as m.notice messages in Matrix room
  - [ ] Timeline posted to room after workflow completion
  - [ ] No forwarding for non-workflow events

  **QA Scenarios**:
  ```
  Scenario: Progress events forwarded to Matrix room
    Tool: Bash (go test)
    Steps:
      1. Emit workflow.progress event to MatrixEventBus
      2. Verify m.room.message sent to workflow's Matrix room with msgtype m.notice
    Expected Result: Event delivered to room, ArmorChat /sync can see it
    Evidence: .sisyphus/evidence/task-9-5-forwarding.txt

  Scenario: Timeline posted on completion
    Tool: Bash (go test)
    Steps:
      1. Complete workflow with 4 events
      2. Verify formatted timeline message sent to room
    Expected Result: Formatted message with event icons and footer
    Evidence: .sisyphus/evidence/task-9-5-timeline-post.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): forward workflow events to Matrix rooms`
  - Files: `bridge/pkg/secretary/orchestrator_events.go, bridge/pkg/secretary/orchestrator_events_test.go`

- [x] 10. Timeline Formatter + Purge Ordering

  **What to do**:
  - Modify `bridge/pkg/secretary/notifications.go` to add two functions:
    - `FormatTimelineMessage(result *ExtendedStepResult) string` — formatted plaintext for Matrix room notifications
    - `GetTimelineEvents(result *ExtendedStepResult) []TimelineEvent` — structured event data for ArmorChat UI (progress bar, type-specific styling, live indicator)
  - `TimelineEvent` struct: `Seq int`, `Type string`, `Name string`, `TsMs int`, `Detail map[string]interface{}`, `DurationMs *int`, `Percent *int` (extracted from detail for progress events)
  - Icon mapping for 4 types: 🔹step, 📊progress, 🏁checkpoint, ❌error
  - Skip `_summary` events from timeline
  - Footer: total duration + step count from EventsSummary
  - Fallback to plain result.Output when no events
  - **Purge ordering is enforced in Task 8** — this task only provides the formatter that Task 8's notify step calls
  - Verify in tests: both functions work from in-memory ExtendedStepResult (no filesystem access)
  - 5 tests (3 for FormatTimelineMessage, 2 for GetTimelineEvents)

  **Must NOT do**:
  - Do NOT access filesystem — work from in-memory ExtendedStepResult only
  - Do NOT include icons for deferred event types

  **Recommended Agent Profile**: `unspecified-high`
  **Parallelization**: YES (Wave 3, with Task 9 start)
  **Blocked By**: Tasks 4, 8
  **Blocks**: Phase 4

  **Acceptance Criteria**:
  - [ ] `go test -v ./pkg/secretary/... -run TestTimeline` → PASS (3 tests)
  - [ ] Timeline renders with icons, names, durations
  - [ ] Empty events falls back to plain output

  **Commit**: YES
  - Message: `feat(secretary): add timeline formatter`
  - Files: `bridge/pkg/secretary/notifications.go, bridge/pkg/secretary/notifications_test.go`

- [x] 11. Skill Extraction Pipeline (ExtractFromResult)

  **What to do**:
  - Create `bridge/pkg/skills/extractor.go`
  - `ExtractFromResult(result *ExtendedStepResult, taskDesc, taskID, templateID string) []LearnedSkill`
  - Strategy 1: Agent self-reported — iterate `result.SkillCandidates`
  - Strategy 2: Step pattern — extract 3+ STEP events with distinct names, build skill with PatternStepSequence, confidence 0.5 (replaces v3's command_sequence which requires COMMAND_RUN events). Requiring 3+ distinct names prevents noise from every successful step execution.
  - Strategy 3: Checkpoint pattern — extract CHECKPOINT events, build skill with PatternCheckpointSequence, confidence 0.4
  - **Removed from v3**: command_sequence extraction (requires COMMAND_RUN events), file_operations extraction (requires FILE_* events)
  - `PatternType` constants: `PatternStepSequence`, `PatternCheckpointSequence`, `PatternConfigTemplate`
  - `generateSkillName`, `deduplicateSkills` helpers
  - 7 tests

  **Must NOT do**:
  - Do NOT extract from failed tasks
  - Do NOT assign confidence above 0.7 for auto-extracted skills

  **Recommended Agent Profile**: `unspecified-high`
  **Parallelization**: YES (Wave 4, with Task 12)
  **Blocked By**: Tasks 1, 4
  **Blocks**: Tasks 12, 13

  **References**:
  - `bridge/pkg/skills/extractor.go` (new file)
  - `bridge/pkg/secretary/result.go` — ExtendedStepResult (already implemented)

  **Acceptance Criteria**:
  - [ ] `go test -v ./pkg/skills/... -run TestExtract` → PASS (7 tests)
  - [ ] 3+ STEP events with distinct names produces skill with PatternStepSequence
  - [ ] 2 STEP events with same name does NOT produce skill (noise filter)
  - [ ] Self-reported candidates included
  - [ ] Empty events → empty candidates

  **Commit**: YES
  - Message: `feat(skills): add skill extraction pipeline`
  - Files: `bridge/pkg/skills/extractor.go, bridge/pkg/skills/extractor_test.go`

- [x] 12. Skill Injection at Dispatch (injectLearnedSkills)

  **What to do**:
  - Add `injectLearnedSkills(config json.RawMessage, taskDesc string) json.RawMessage` to StepExecutor
  - Call `learnedStore.FindForTask(taskDesc, 3)`, append `relevant_skills` to config JSON
  - Nil store or no matches → return config unchanged
  - 3 tests

  **Recommended Agent Profile**: `quick`
  **Parallelization**: YES (Wave 4)
  **Blocked By**: Tasks 1, 11
  **Blocks**: Task 13

  **References**:
  - `bridge/pkg/secretary/orchestrator_integration.go` — StepExecutor

  **Acceptance Criteria**:
  - [ ] `go test -v ./pkg/secretary/... -run TestInjectLearnedSkills` → PASS
  - [ ] Inject for matching task → config has relevant_skills
  - [ ] No matches → config unchanged

  **Commit**: YES
  - Message: `feat(secretary): inject learned skills at dispatch`
  - Files: `bridge/pkg/secretary/orchestrator_integration.go, bridge/pkg/secretary/orchestrator_integration_test.go`

- [x] 13. Post-Completion Extraction + RecordOutcome + Integration Tests

  **What to do**:
  - Add post-completion extraction: after CompleteWorkflow(), if status="success" and learnedStore != nil, call ExtractFromResult, Save each skill
  - Save failure → log but don't fail workflow
  - RecordOutcome for previously suggested skills based on result status
  - **Data flow**: The list of suggested skill IDs must be captured during dispatch (Task 12) in the workflow's in-memory state. Add `SuggestedSkillIDs []string` field to the orchestrator's workflow state, populated during `injectLearnedSkills()`. Post-completion code reads this field to call RecordOutcome on the correct skills.
  - Full lifecycle integration test: extract → save → inject → execute → record outcome → verify confidence adjustment
  - Confidence decay test: RecordOutcome(false) x3 on 0.5 confidence skill → drops below 0.4 → no longer suggested
  - 12 tests across extractor_test.go and orchestrator_integration_test.go

  **Must NOT do**:
  - Do NOT extract from failed tasks
  - Do NOT fail workflow on skill ops

  **Recommended Agent Profile**: `deep`
  **Parallelization**: NO (Wave 4, final)
  **Blocked By**: Tasks 11, 12
  **Blocks**: Phase 5

  **Acceptance Criteria**:
  - [ ] `go test -v ./pkg/secretary/... -run "TestPostCompletion|TestConfidenceDecay"` → PASS
  - [ ] Full skill lifecycle works: extract → save → inject → execute → record outcome → confidence adjusted
  - [ ] Failed task extraction does not save skills

  **QA Scenarios**:
  ```
  Scenario: Full skill lifecycle
    Tool: Bash (go test)
    Steps:
      1. Extract from result with 3 STEP events → save skill
      2. Inject for matching task → config has relevant_skills
      3. RecordOutcome(true) → confidence up
      4. RecordOutcome(false) x5 → confidence below 0.4
      5. Inject again → skill NOT suggested
    Expected Result: Full feedback loop works
    Evidence: .sisyphus/evidence/task-13-lifecycle.txt
  ```

  **Commit**: YES
  - Message: `feat(skills): post-completion extraction and lifecycle tests`
  - Files: `bridge/pkg/skills/extractor_test.go, bridge/pkg/secretary/orchestrator_integration.go, bridge/pkg/secretary/orchestrator_integration_test.go`

- [x] 14. ArmorChat WorkflowTimeline Composable

  **What to do**:
  - Create `applications/ArmorChat/.../ui/components/WorkflowTimeline.kt`
  - Scrollable vertical timeline for workflow.progress events
  - Icon mapping for 4 types: step→🔹, progress→📊, checkpoint→🏁, error→❌
  - Progress bar at top from latest percent
  - "Live" indicator when running, "Complete" badge when done
  - Empty state: "Waiting for agent activity..."
  - Material 3 theming, light + dark

  **Must NOT do**:
  - Do NOT add WebSocket — Matrix /sync only
  - Do NOT hard-code colors

  **Recommended Agent Profile**: `visual-engineering`
  **Parallelization**: YES (Wave 5, with Tasks 15, 16)
  **Blocked By**: Task 10
  **Blocks**: R2

  **References**:
  - `applications/ArmorChat/.../ui/components/MessageBubble.kt` — Existing component pattern
  - `shared/src/commonMain/kotlin/data/store/ControlPlaneStore.kt` — Matrix /sync event consumer
  - `androidApp/src/main/kotlin/com/armorclaw/app/data/bridge/BridgeApi.kt` — Correct RPC client name

  **Acceptance Criteria**:
  - [ ] Compose preview renders correctly with 4 event types
  - [ ] Empty state shows waiting message
  - [ ] Progress bar updates, live/complete indicators work

  **Commit**: YES
  - Message: `feat(armorchat): add WorkflowTimeline composable`
  - Files: `applications/ArmorChat/.../ui/components/WorkflowTimeline.kt`

- [x] 15. ArmorChat Agent Status Indicator

  **What to do**:
  - Modify existing status banner component to add visual state for `WorkflowStatus.RUNNING`: blue/teal pulsing indicator with step count from latest progress event
  - No new "blocked" state (StatusBlocked not implemented)
  - Existing states unchanged
  - Use Material 3 `colorScheme.primaryContainer` for running

  **Must NOT do**:
  - Do NOT add blocked state
  - Do NOT change existing idle/completed/failed states

  **Recommended Agent Profile**: `quick`
  **Parallelization**: YES (Wave 5)
  **Blocked By**: None
  **Blocks**: R2

  **Acceptance Criteria**:
  - [ ] Running state: pulsing indicator, step count
  - [ ] Existing states unchanged

  **Commit**: YES
  - Message: `feat(armorchat): add running state to status banner`
  - Files: `applications/ArmorChat/.../ui/components/GovernanceBanner.kt`

- [x] 16. Matrix Commands (!agent skills, !agent forget-skill)

  **What to do**:
  - Add to `bridge/internal/adapter/commands_integration.go`
  - `!agent skills <agent_id>` — ListForAgent(20), formatted numbered list
  - `!agent forget-skill <agent_id> <skill_id>` — Delete, confirm
  - Access LearnedStore via existing DI
  - 4 tests

  **Recommended Agent Profile**: `unspecified-high`
  **Parallelization**: YES (Wave 5)
  **Blocked By**: Tasks 1, 11
  **Blocks**: R1

  **References**:
  - `bridge/internal/adapter/commands_integration.go` — existing commands

  **Acceptance Criteria**:
  - [ ] `go test -v ./bridge/internal/adapter/... -run TestAgentCommands` → PASS (4 tests)
  - [ ] `!agent skills` lists skills with confidence
  - [ ] `!agent forget-skill` deletes and confirms

  **Commit**: YES
  - Message: `feat(commands): add !agent skills and forget-skill`
  - Files: `bridge/internal/adapter/commands_integration.go, bridge/internal/adapter/commands_integration_test.go`