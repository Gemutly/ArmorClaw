# Agent Studio Improvement Plan v3 — Observable Containers + Blocker Protocol + Learned Skills

## TL;DR

> **Quick Summary**: Transform Agent Studio containers from black boxes into observable artifact producers with structured event streaming, human-in-the-loop blocker resolution, and learned skill persistence — while preserving all ArmorClaw security constraints (zero network, PII sovereignty, Matrix-only control plane).
> 
> **Deliverables**:
> - Bridge EventReader with 10MB cap and incremental tailing
> - Container EventEmitter with PIPE_BUF enforcement
> - Progress streaming from container → Bridge → ArmorChat
> - Blocker protocol with PII-safe resolution via env vars
> - Learned skills extraction/persistence in SQLite
> - ArmorChat status timeline + blocker dialog + Matrix commands
> 
> **Estimated Effort**: XL (6 phases, ~25 days)
> **Parallel Execution**: YES — 5-6 waves per phase
> **Critical Path**: Phase 1 (foundation) → Phase 2 (container emission) → Phase 3 (streaming) → Phase 4 (blockers) → Phase 5 (skills) → Phase 6 (client)

---

## Context

### Original Request
User provided a comprehensive 6-phase improvement plan ("Agent Studio Improvement Plan v3") with CTO corrections applied, covering purge ordering, `docker kill` for resource protection, PIPE_BUF enforcement, Bridge-first phase ordering, PII-safe blocker flow, and SQLCipher schema as foundational prerequisite.

### Interview Summary
**Key Discussions**:
- All 8 referenced Bridge Go files exist with rich implementations (orchestrator, step executor, event emitter, notifications, factory, RPC)
- Container Python code is minimal (83-line step_runner, 54-line result_writer, 56-line step_config) — no existing event streaming
- ArmorChat's `BridgeApi.kt` is the actual RPC client (plan incorrectly names it `BridgeRpcClient.kt`)
- `Route.Home` is a placeholder screen — chat/dashboard needs building

**Research Findings**:
- **Dual event bus**: `MatrixEventBus` (internal ring buffer) + `EventBus` (external WebSocket+filtering) — currently disconnected
- **Workflow state machine has 5 states** (pending/running/completed/failed/cancelled) — needs BLOCKED added as 6th
- **Factory has Stop()/Remove() but NO Kill()** — need new method using `docker.ContainerKill()`
- **Secretary store is plain SQLite** (not SQLCipher) — learned_skills table goes in unencrypted DB
- **Matrix processEvents() drops non-message events** — custom event types need handling added
- **ArmorChat has no WebSocket client** — only synchronous OkHttp JSON-RPC

### Metis Review
**Identified Gaps** (addressed):
- State machine is 5 states not 11 → corrected, adding BLOCKED as 6th state
- EventEmitter interface already exists → extending, not creating parallel system
- Handler signature `(cfg) -> str` must be preserved → inject EventEmitter via StepConfig, not parameter
- ArmorChat streaming needs transport decision → Matrix /sync reuse (zero new infrastructure) preferred over WebSocket
- Bridge restart loses pending blocker state → acknowledged, document as known limitation
- Container state dir collision if same definition runs twice → add instance ID to state dir path

---

## Work Objectives

### Core Objective
Transform containers into observable artifact producers with progress streaming, blocker handling, and skill learning while respecting every ArmorClaw security constraint.

### Concrete Deliverables
- `bridge/pkg/skills/learned_store.go` — SQLite-backed learned skills persistence
- `bridge/pkg/skills/extractor.go` — Skill extraction from completed task events
- `bridge/pkg/secretary/event_reader.go` — Incremental event file tailing with 10MB cap
- `bridge/pkg/secretary/cleanup.go` — State directory purge (before notification)
- `container/openclaw/events.py` — EventEmitter with PIPE_BUF (4096 byte) enforcement
- Extended `result.json` with underscore-prefixed backward-compatible fields
- `StatusBlocked` workflow state with valid transitions
- `factory.Kill()` for SIGKILL on resource exhaustion
- `resolve_blocker` RPC handler with PII-safe response delivery
- ArmorChat `WorkflowTimeline`, `BlockerResponseDialog`, status banner
- Matrix commands `!agent skills`, `!agent forget-skill`

### Definition of Done
- [ ] `go test ./pkg/secretary/... ./pkg/skills/...` passes all new tests
- [ ] `python -m pytest container/tests/` passes all new tests
- [ ] Bridge tails `_events.jsonl` during execution, streams progress to Matrix
- [ ] `_events.jsonl` and state directory verified absent after task completion
- [ ] Container exceeding 10MB event log is SIGKILLed, state dir purged
- [ ] Blocker → blocked → resolve → re-spawn → complete E2E flow works
- [ ] Blocker response content absent from Bridge logs and VPS disk
- [ ] Learned skills persisted, injected as suggestions, confidence adjusts with outcomes

### Must Have
- PIPE_BUF enforcement (every event line ≤ 4096 bytes)
- 10MB cap with `docker kill` (SIGKILL, not SIGTERM)
- Purge ordering: parse → RemoveAll → notify
- PII in blocker responses: env var only, never files, never logged
- Backward compatibility: old containers work with new Bridge, vice versa
- Learned skills: confidence ≥ 0.4 threshold, suggestions only (never auto-executed)

### Must NOT Have (Guardrails)
- No network access to containers (NetworkMode "none" preserved)
- No Matrix connectivity in containers
- No change to spawn-exit lifecycle
- No real-time streaming (500ms polling is the ceiling)
- No auto-execution of learned skills
- No change to base `result.json` fields (status, output, data, error, duration_ms)
- Handler function signature stays `(cfg) -> str` — EventEmitter, comments, blockers injected via config dict keys (`_emitter_ref`, `_comments`, `_blockers`), NOT as function parameters
- No parallel event system — extend existing `EventEmitter` interface
- No WebSocket client added to ArmorChat — use Matrix /sync for streaming
- No PII in any file on VPS disk (including result.json)

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES
- **Automated tests**: YES (Tests-after — comprehensive test tables per phase from user's plan)
- **Framework**: Go `go test` for Bridge, Python `unittest` for Container, Kotlin JUnit for Android

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Bridge (Go)**: Use Bash (`go test -v ./pkg/secretary/...`), Bash (`curl` for RPC endpoints)
- **Container (Python)**: Use Bash (`python -m pytest container/tests/`)
- **Android (Kotlin)**: Use Bash (`./gradlew test`) for unit tests
- **Integration**: Use Bash (`docker compose` + `curl` RPC) for E2E flows

---

## Execution Strategy

### Parallel Execution Waves

```
Phase 1 (Bridge Foundation — 6 tasks, 4 days):
├── Task 1:  SQLite learned_skills table + LearnedStore [quick]
├── Task 2:  EventReader with 10MB cap + incremental tailing [unspecified-high]
├── Task 3:  State directory cleanup utility [quick]
├── Task 4:  ExtendedStepResult types + ParseExtendedStepResult [unspecified-high]
├── Task 5:  Factory.Kill() method (SIGKILL via Docker) [quick]
└── Task 6:  StatusBlocked state + transition map update [quick]
  Gate: Bridge can create LearnedStore, EventReader passes all tests,
        cleanupStateDir verified, Kill() issues SIGKILL

Phase 2 (Container Emission — 5 tasks, 3 days):
├── Task 7:  EventEmitter with PIPE_BUF enforcement [unspecified-high]
├── Task 8:  Enriched result_writer (write_enriched_result) [quick]
├── Task 9:  StepRunner integration (inject EventEmitter via StepConfig) [unspecified-high]
├── Task 10: _blocker_response and _retry parsing in step_config [quick]
└── Task 11: Container tests (events, result_writer, step_runner) [unspecified-high]
  Gate: Container writes _events.jsonl + extended result.json,
        Phase 1 ParseExtendedStepResult reads output correctly

Phase 3 (Progress Streaming — 5 tasks, 4 days):
├── Task 12: Polling loop in waitForCompletion (tail _events.jsonl) [deep]
├── Task 13: Progress event emission to MatrixEventBus [unspecified-high]
├── Task 14: Timeline formatter (FormatTimelineMessage) [quick]
├── Task 15: Purge ordering (parse → RemoveAll → notify) + 10MB kill flow [deep]
└── Task 16: Matrix event type handling in processEvents [unspecified-high]
  Gate: Bridge emits `workflow.progress` events to MatrixEventBus during execution,
        posts formatted timeline to Matrix room after completion, state dir verified purged
        via `stateDirExists()` check

Phase 4 (Blocker Protocol — 5 tasks, 5 days):
├── Task 17: Blocker loop with PII safety (executeStepWithBlockerHandling) [deep]
├── Task 18: waitForBlockerResponse (sync.Map channel pattern) [unspecified-high]
├── Task 19: resolve_blocker RPC handler (studio namespace) [unspecified-high]
├── Task 20: Blocker notification formatting + Matrix delivery [quick]
└── Task 21: Blocker integration tests (E2E flow, PII verification) [deep]
  Gate: Container reports blocker → Bridge blocks → RPC response → re-spawn → complete

Phase 5 (Learned Skills — 4 tasks, 5 days):
├── Task 22: Skill extraction pipeline (ExtractFromResult) [unspecified-high]
├── Task 23: Skill injection at dispatch (injectLearnedSkills) [quick]
├── Task 24: Post-completion extraction + RecordOutcome [unspecified-high]
└── Task 25: Learned skills integration tests [deep]
  Gate: Successful task extracts skills, future matching tasks receive suggestions

Phase 6 (Client Surface — 5 tasks, 4 days):
├── IMPORTANT: All ArmorChat file references use BridgeApi.kt (NOT BridgeRpcClient.kt)
├── IMPORTANT: All streaming uses Matrix /sync via ControlPlaneStore (NOT WebSocket)
├── Task 26: ArmorChat WorkflowTimeline composable [visual-engineering]
├── Task 27: ArmorChat BlockerResponseDialog [visual-engineering]
├── Task 28: ArmorChat agent status banner (GovernanceBanner) [quick]
├── Task 29: BridgeApi resolve_blocker RPC method [quick]
└── Task 30: Matrix commands (!agent skills, !agent forget-skill) [unspecified-high]
  Gate: ArmorChat shows timeline during execution, blocker dialog on block,
        skills manageable via Matrix commands

Wave FINAL (After ALL phases — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay

Critical Path: T1-T4 → T7-T11 → T12-T16 → T17-T21 → T22-T25 → T26-T30 → F1-F4
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 6 (Phase 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1-6 | None | 7-11, 12-16, 22-25 | 1 |
| 7 | None | 9, 11 | 2 |
| 8 | None | 9, 11 | 2 |
| 9 | 7, 8 | 11 | 2 |
| 10 | None | 9 | 2 |
| 11 | 7, 8, 9 | Phase 3 | 2 |
| 12 | 2, 5, 6 | 15 | 3 |
| 13 | 12 | 16 | 3 |
| 14 | 4 | 16 | 3 |
| 15 | 12, 14 | Phase 4 | 3 |
| 16 | 13 | Phase 4 | 3 |
| 17 | 6, 12, 15 | 21 | 4 |
| 18 | 17 | 19 | 4 |
| 19 | 18 | 21 | 4 |
| 20 | 14 | Phase 6 | 4 |
| 21 | 17-20 | Phase 5 | 4 |
| 22 | 1, 4 | 25 | 5 |
| 23 | 1, 22 | 25 | 5 |
| 24 | 22, 23 | 25 | 5 |
| 25 | 22-24 | Phase 6 | 5 |
| 26 | 14 | F3 | 6 |
| 27 | 19 | F3 | 6 |
| 28 | 6 | F3 | 6 |
| 29 | 19 | 27 | 6 |
| 30 | 1, 22 | F1 | 6 |

### Agent Dispatch Summary

- **Phase 1**: 6 tasks — T1,T3,T5,T6 → `quick`, T2,T4 → `unspecified-high`
- **Phase 2**: 5 tasks — T7,T9,T11 → `unspecified-high`, T8,T10 → `quick`
- **Phase 3**: 5 tasks — T12,T15 → `deep`, T13,T16 → `unspecified-high`, T14 → `quick`
- **Phase 4**: 5 tasks — T17,T21 → `deep`, T18,T19 → `unspecified-high`, T20 → `quick`
- **Phase 5**: 4 tasks — T22,T24 → `unspecified-high`, T23 → `quick`, T25 → `deep`
- **Phase 6**: 5 tasks — T26,T27 → `visual-engineering`, T28,T29 → `quick`, T30 → `unspecified-high`
- **FINAL**: 4 tasks — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. SQLite Learned Skills Table + LearnedStore

  **What to do**:
  - Create `bridge/pkg/skills/learned_store.go` with `LearnedStore` struct backed by `*sql.DB`
  - Add `learned_skills` table DDL to the secretary store's `initSchema()` in `bridge/pkg/secretary/store.go` (NOT the SQLCipher keystore — secretary store uses plain SQLite). Follow the existing v1→v2 migration pattern at `store.go:240-280`
  - Table schema: `id TEXT PK, name TEXT UNIQUE, description TEXT, source_task_id TEXT, source_template_id TEXT, pattern_type TEXT NOT NULL, pattern_data TEXT NOT NULL, trigger_keywords TEXT NOT NULL, success_count INT DEFAULT 0, failure_count INT DEFAULT 0, last_used_at INT, created_at INT NOT NULL, confidence REAL DEFAULT 0.5`
  - Create index `idx_learned_confidence ON learned_skills(confidence)`
  - Implement `NewLearnedStore(db *sql.DB)`, `Save(LearnedSkill)`, `FindForTask(taskDesc, limit)`, `RecordOutcome(skillID, bool)`, `Delete(skillID)`, `ListForAgent(limit)`
  - `FindForTask` must: query WHERE confidence >= 0.4, then rank by keyword overlap with task description
  - Create `bridge/pkg/skills/learned_store_test.go` with all 12 tests from the user's plan

  **Must NOT do**:
  - Do NOT put the table in the SQLCipher keystore (`pkg/keystore/`) — secretary store is separate and unencrypted
  - Do NOT add a migration framework — use `CREATE TABLE IF NOT EXISTS`
  - Do NOT auto-execute skills — they are suggestions only

  **Security Trade-Off (Documented)**:
  - Learned skills are stored in the secretary store (plain SQLite), NOT in the SQLCipher
    keystore. This is a deliberate trade-off: the keystore holds credentials and PII that
    must be encrypted at rest. Learned skills are execution patterns (command sequences,
    file operations) — they contain no secrets. Storing them in SQLCipher would add
    complexity (separate connection, key management) with no security benefit. If this
    trade-off is unacceptable in a future compliance regime, the table can be migrated
    to SQLCipher by changing the `sql.Open()` call and adding the cipher pragmas.

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3, 4, 5, 6)
  - **Blocks**: Tasks 22, 23, 30
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/store.go:114` — Secretary SQLite store initialization pattern (`sql.Open("sqlite3", path)`)
  - `bridge/pkg/secretary/store.go:240-280` — v1→v2 migration pattern (CREATE TABLE IF NOT EXISTS)
  - `bridge/pkg/secretary/store.go:567-634` (keystore example) — Multi-statement DDL in `initSchema()`

  **API/Type References**:
  - `bridge/pkg/secretary/types.go` — Domain types pattern (WorkflowStep, TaskTemplate etc.)

  **WHY Each Reference Matters**:
  - `store.go:114` shows the secretary DB is plain SQLite, not SQLCipher — the learned_skills table goes here
  - `store.go:240-280` shows the migration pattern to follow for additive schema changes

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/skills/... -run TestLearnedStore` → PASS (12 tests)
  - [ ] `LearnedStore` creates table on existing secretary DB without error
  - [ ] `Save()` + `FindForTask()` + `RecordOutcome()` round-trip correctly
  - [ ] Skills with confidence < 0.4 excluded from `FindForTask()` results
  - [ ] `RecordOutcome(success=false)` decreases confidence, `RecordOutcome(success=true)` increases it

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Save and retrieve a learned skill
    Tool: Bash (go test)
    Preconditions: Clean in-memory SQLite DB
    Steps:
      1. Create LearnedStore with sql.Open("sqlite3", ":memory:")
      2. Save a LearnedSkill{Name: "db-migrate", PatternType: "command_sequence", TriggerKeywords: ["migrate","database"], Confidence: 0.7}
      3. Call FindForTask("migrate the production database", 5)
    Expected Result: Returns exactly 1 skill with name "db-migrate"
    Failure Indicators: Empty slice returned, wrong skill name, confidence mismatch
    Evidence: .sisyphus/evidence/task-1-save-retrieve.txt

  Scenario: Confidence threshold filtering
    Tool: Bash (go test)
    Preconditions: DB with 3 skills: confidence 0.3, 0.6, 0.8
    Steps:
      1. Save 3 skills with different confidence values
      2. Call FindForTask("any task", 10)
    Expected Result: Returns only skills with confidence >= 0.4 (2 of 3)
    Failure Indicators: Returns the 0.3-confidence skill, returns all 3
    Evidence: .sisyphus/evidence/task-1-confidence-threshold.txt
  ```

  **Commit**: YES
  - Message: `feat(skills): add learned_skills table and LearnedStore`
  - Files: `bridge/pkg/skills/learned_store.go, bridge/pkg/skills/learned_store_test.go`
  - Pre-commit: `go test ./pkg/skills/...`

- [x] 2. EventReader with 10MB Cap + Incremental Tailing

  **What to do**:
  - Create `bridge/pkg/secretary/event_reader.go` with `EventReader` struct
  - `NewEventReader(stateDir string) *EventReader`
  - `ReadNew() ([]StepEvent, int64, error)` — returns new events since last call, file size, error
  - Track `byteOffset int64` and `lastSeq int` for incremental reads
  - Size check BEFORE reading: if `os.Stat()` shows > 10MB, return `ErrEventLogExceeded`
  - File at exactly 10MB (10*1024*1024) does NOT trigger error — only > 10MB
  - Skip malformed JSON lines, skip lines with seq <= lastSeq
  - Skip comment lines (starting with `#`) and empty lines
  - Create `bridge/pkg/secretary/event_reader_test.go` with all 9 tests from user's plan

  **Must NOT do**:
  - Do NOT read the entire file each call — use Seek to byteOffset
  - Do NOT trigger error at exactly 10MB — only strictly greater than

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3, 4, 5, 6)
  - **Blocks**: Task 12
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:321-349` — `waitForCompletion()` polling loop pattern (500ms ticker, Docker status check)

  **API/Type References**:
  - `bridge/pkg/secretary/result.go:39` — `ContainerStepResult` struct pattern (will be extended in Task 4)

  **WHY Each Reference Matters**:
  - `orchestrator_integration.go:321-349` shows the existing 500ms polling loop where EventReader will be integrated in Phase 3

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/secretary/... -run TestEventReader` → PASS (9 tests)
  - [ ] Reads incrementally across 5+ poll cycles with zero duplication
  - [ ] File at exactly 10MB does NOT trigger `ErrEventLogExceeded`
  - [ ] File at 10MB+1 byte DOES trigger `ErrEventLogExceeded`
  - [ ] Malformed JSON lines skipped, good lines after still returned
  - [ ] Missing file returns nil, nil (not error)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Incremental reads with offset tracking
    Tool: Bash (go test)
    Preconditions: Temp dir with _events.jsonl containing 5 valid JSON lines
    Steps:
      1. Create EventReader for temp dir
      2. Call ReadNew() — expect 5 events, verify offset advanced
      3. Append 3 more lines to the file
      4. Call ReadNew() again — expect exactly 3 new events
      5. Call ReadNew() again — expect 0 events (no new data)
    Expected Result: Second call returns exactly 3 events with seq > lastSeq from first call
    Failure Indicators: Returns 8 events on second call, returns 0, offset not advanced
    Evidence: .sisyphus/evidence/task-2-incremental-reads.txt

  Scenario: 10MB cap enforcement
    Tool: Bash (go test)
    Preconditions: _events.jsonl file of exactly 10*1024*1024 bytes
    Steps:
      1. Create EventReader for this dir
      2. Call ReadNew() — should succeed (exactly at cap)
      3. Append 1 byte to the file (total > 10MB)
      4. Call ReadNew() — should return ErrEventLogExceeded
    Expected Result: First call succeeds, second call returns ErrEventLogExceeded
    Failure Indicators: First call fails, second call doesn't return ErrEventLogExceeded
    Evidence: .sisyphus/evidence/task-2-10mb-cap.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add EventReader with 10MB cap`
  - Files: `bridge/pkg/secretary/event_reader.go, bridge/pkg/secretary/event_reader_test.go`
  - Pre-commit: `go test ./pkg/secretary/... -run TestEventReader`

- [x] 3. State Directory Cleanup Utility

  **What to do**:
  - Create `bridge/pkg/secretary/cleanup.go`
  - `cleanupStateDir(stateDir string) error` — calls `os.RemoveAll(stateDir)`
  - Empty string = no-op (return nil)
  - Nonexistent path = no error (RemoveAll handles this)
  - `stateDirExists(stateDir string) bool` — test helper, checks for `_events.jsonl` in dir
  - Create `bridge/pkg/secretary/cleanup_test.go` with 4 tests

  **Must NOT do**:
  - Do NOT use partial cleanup — RemoveAll the entire state directory
  - Do NOT fail the workflow if cleanup fails — log the error but proceed

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 4, 5, 6)
  - **Blocks**: Task 15
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:321-349` — `waitForCompletion()` where cleanup will be integrated

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/secretary/... -run TestCleanup` → PASS (4 tests)
  - [ ] `cleanupStateDir()` removes directory and all contents
  - [ ] `stateDirExists()` returns false after cleanup
  - [ ] Empty string input returns nil (no error)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Cleanup removes state directory
    Tool: Bash (go test)
    Preconditions: Temp dir with _events.jsonl and result.json files
    Steps:
      1. Create temp dir with both files
      2. Call cleanupStateDir(tempDir)
      3. Call os.Stat(tempDir)
    Expected Result: os.Stat returns error with os.IsNotExist = true
    Failure Indicators: Directory still exists after cleanup
    Evidence: .sisyphus/evidence/task-3-cleanup.txt

  Scenario: Nonexistent path is no-op
    Tool: Bash (go test)
    Steps:
      1. Call cleanupStateDir("/nonexistent/path/12345")
    Expected Result: Returns nil (no error)
    Failure Indicators: Returns non-nil error
    Evidence: .sisyphus/evidence/task-3-noop.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add state dir cleanup utility`
  - Files: `bridge/pkg/secretary/cleanup.go, bridge/pkg/secretary/cleanup_test.go`
  - Pre-commit: `go test ./pkg/secretary/... -run TestCleanup`

- [x] 4. ExtendedStepResult Types + ParseExtendedStepResult

  **What to do**:
  - Modify `bridge/pkg/secretary/result.go` to add new types:
    - `StepEvent` struct: `Seq int`, `Type string`, `Name string`, `TsMs int`, `Detail map[string]interface{}`, `DurationMs *int`
    - `Blocker` struct: `BlockerType string`, `Message string`, `Suggestion string`, `Field string`
    - `SkillCandidate` struct: `Name string`, `Description string`, `PatternType string`, `PatternData string`, `Confidence float64`
    - `EventsSummary` struct: `Total int`, `Types map[string]int`
    - `ExtendedStepResult` struct embedding `*ContainerStepResult` plus: `Comments []string`, `Blockers []Blocker`, `SkillCandidates []SkillCandidate`, `EventsSummary *EventsSummary`, `Events []StepEvent` (held in memory only, `json:"-"`)
  - Implement `ParseExtendedStepResult(stateDir string) (*ExtendedStepResult, error)`:
    1. Call existing `ParseContainerStepResult(stateDir)` for base fields
    2. Re-read `result.json` and parse underscore-prefixed fields (`_comments`, `_blockers`, `_skill_candidates`, `_events_summary`)
    3. Call `ReadEventsFile(stateDir)` to load `_events.jsonl` into memory
  - Implement `ReadEventsFile(stateDir string) ([]StepEvent, error)` — reads and parses `_events.jsonl`
  - Create `bridge/pkg/secretary/result_test.go` (or extend existing) with 4 tests

  **Must NOT do**:
  - Do NOT change existing `ContainerStepResult` struct or `ParseContainerStepResult` function signature
  - Do NOT remove the existing `ParseContainerStepResult` — new function calls it internally

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3, 5, 6)
  - **Blocks**: Task 14, Task 22
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/result.go:39` — Existing `ContainerStepResult` struct — embed this, do not modify it

  **API/Type References**:
  - `container/openclaw/result_writer.py:20-31` — Python `build_result()` schema (status, output, data, error, duration_ms)

  **WHY Each Reference Matters**:
  - `result.go:39` is the existing contract — new types must extend, not replace
  - `result_writer.py` shows the Python-side format that Go must parse

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/secretary/... -run TestParseExtended` → PASS (4 tests)
  - [ ] New `result.json` with all underscore fields → all fields parsed correctly
  - [ ] Old `result.json` (no underscore fields) → base result returned, no error
  - [ ] Missing `_events.jsonl` → Events field is nil, no error

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Parse extended result with all new fields
    Tool: Bash (go test)
    Preconditions: Temp dir with result.json containing _comments, _blockers, _skill_candidates, _events_summary AND _events.jsonl with 3 events
    Steps:
      1. Call ParseExtendedStepResult(tempDir)
      2. Verify result.Comments has expected values
      3. Verify result.Blockers[0].Message matches
      4. Verify result.Events has 3 entries
    Expected Result: All underscore fields populated, Events has 3 parsed StepEvent entries
    Failure Indicators: Any field is nil/empty, Events count mismatch
    Evidence: .sisyphus/evidence/task-4-extended-parse.txt

  Scenario: Backward compatible with old format
    Tool: Bash (go test)
    Preconditions: Temp dir with result.json containing ONLY base fields (status, output, duration_ms), no _events.jsonl
    Steps:
      1. Call ParseExtendedStepResult(tempDir)
      2. Verify result.Status == "success"
      3. Verify result.Comments is nil
      4. Verify result.Events is nil
    Expected Result: Base fields populated, all new fields nil/empty, no error
    Failure Indicators: Error returned, panic on nil access
    Evidence: .sisyphus/evidence/task-4-backward-compat.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add ExtendedStepResult types and parser`
  - Files: `bridge/pkg/secretary/result.go, bridge/pkg/secretary/result_test.go`
  - Pre-commit: `go test ./pkg/secretary/... -run TestParseExtended`

- [x] 5. Factory.Kill() Method (SIGKILL via Docker)

  **What to do**:
  - Modify `bridge/pkg/studio/factory.go` to add `Kill(instanceID string) error` method on `AgentFactory`
  - Implementation: Use `f.dockerClient.ContainerKill(ctx, instanceID, "SIGKILL")` with 5-second context timeout
  - This sends SIGKILL immediately — no graceful shutdown period (unlike `Stop()` which sends SIGTERM + 30s wait)
  - Create/extend test to verify Kill issues SIGKILL, not SIGTERM
  - Ensure `DockerClient` interface in `factory.go` includes the `ContainerKill` method (check if it's already there — the interface wraps Docker SDK)

  **Must NOT do**:
  - Do NOT modify existing `Stop()` or `Remove()` methods
  - Do NOT add graceful shutdown to Kill — it must be immediate SIGKILL

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3, 4, 6)
  - **Blocks**: Task 12, Task 15
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/studio/factory.go:200-250` — Existing `Stop()` method pattern (SIGTERM + timeout)
  - `bridge/pkg/studio/factory.go:50-80` — `DockerClient` interface definition

  **WHY Each Reference Matters**:
  - `Stop()` shows the graceful shutdown pattern — Kill must bypass all of this
  - `DockerClient` interface must be checked for `ContainerKill` support

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/studio/... -run TestFactoryKill` → PASS
  - [ ] `Kill()` calls Docker's `ContainerKill` with "SIGKILL" signal
  - [ ] `Kill()` uses 5-second context timeout

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Kill issues SIGKILL
    Tool: Bash (go test)
    Preconditions: Mock DockerClient that records calls
    Steps:
      1. Call factory.Kill("container-123")
      2. Verify mock received ContainerKill("container-123", "SIGKILL")
    Expected Result: ContainerKill called with SIGKILL, NOT ContainerStop
    Failure Indicators: ContainerStop called instead, wrong signal
    Evidence: .sisyphus/evidence/task-5-factory-kill.txt
  ```

  **Commit**: YES
  - Message: `feat(studio): add factory.Kill() for SIGKILL`
  - Files: `bridge/pkg/studio/factory.go, bridge/pkg/studio/factory_test.go`
  - Pre-commit: `go test ./pkg/studio/... -run TestFactoryKill`

- [x] 6. StatusBlocked Workflow State + Transition Map

  **What to do**:
  - Add `StatusBlocked WorkflowStatus = "blocked"` constant to `bridge/pkg/secretary/types.go` (alongside existing StatusPending/Running/Completed/Failed/Cancelled)
  - Modify `validateTransition()` in `bridge/pkg/secretary/orchestrator.go:440-446` to add:
    - `StatusRunning → StatusBlocked` (agent hits a blocker during execution)
    - `StatusBlocked → StatusRunning` (blocker resolved, re-spawn)
    - `StatusBlocked → StatusFailed` (blocker timeout or max retries)
    - `StatusBlocked → StatusCancelled` (user cancels while blocked)
  - Add `BlockWorkflow(workflowID, reason, message)` method to `WorkflowOrchestratorImpl` — transitions to blocked and emits event
  - Create test verifying all new transitions accepted and invalid ones rejected (e.g., Blocked → Completed is invalid)

  **Must NOT do**:
  - Do NOT change existing transitions for other states
  - Do NOT add a "paused" state — that's a different feature

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3, 4, 5)
  - **Blocks**: Task 17, Task 28
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/orchestrator.go:440-446` — Existing `validateTransition()` with 5-state map
  - `bridge/pkg/secretary/types.go:16-22` — Existing `WorkflowStatus` constants

  **WHY Each Reference Matters**:
  - `orchestrator.go:440-446` is the EXACT line to modify — add Blocked transitions here

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/secretary/... -run TestBlocked` → PASS
  - [ ] Running → Blocked transition accepted
  - [ ] Blocked → Running transition accepted
  - [ ] Blocked → Failed transition accepted
  - [ ] Blocked → Completed transition REJECTED
  - [ ] `BlockWorkflow()` sets status and emits event

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Valid blocked state transitions
    Tool: Bash (go test)
    Steps:
      1. Transition StatusRunning → StatusBlocked — expect success
      2. Transition StatusBlocked → StatusRunning — expect success
      3. Transition StatusBlocked → StatusFailed — expect success
    Expected Result: All three transitions succeed without error
    Failure Indicators: Any transition returns error
    Evidence: .sisyphus/evidence/task-6-blocked-transitions.txt

  Scenario: Invalid blocked transition rejected
    Tool: Bash (go test)
    Steps:
      1. Transition StatusBlocked → StatusCompleted — expect error
    Expected Result: Error returned indicating invalid transition
    Failure Indicators: Transition succeeds (Completed from Blocked is forbidden)
    Evidence: .sisyphus/evidence/task-6-invalid-transition.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add StatusBlocked workflow state`
  - Files: `bridge/pkg/secretary/types.go, bridge/pkg/secretary/orchestrator.go, bridge/pkg/secretary/orchestrator_test.go`
  - Pre-commit: `go test ./pkg/secretary/... -run TestBlocked`

- [x] 7. EventEmitter with PIPE_BUF Enforcement

  **What to do**:
  - Create `container/openclaw/events.py` with `EventEmitter` class
  - Define `EventType` constants: STEP, FILE_READ, FILE_WRITE, FILE_DELETE, COMMAND_RUN, OBSERVATION, BLOCKER, ERROR, ARTIFACT, PROGRESS, CHECKPOINT
  - Define `StepEvent` dataclass: `seq int`, `type str`, `name str`, `ts_ms int`, `detail dict`, `duration_ms Optional[int]`
  - `EventEmitter.__init__(state_dir)` — opens `_events.jsonl` in write mode, writes header comment `# Agent Studio execution events`, initializes `_seq=0` and `_start_ms` from `time.monotonic()`
  - `emit(event_type, name, detail=None, duration_ms=None)` — builds StepEvent, serializes to JSON+newline via `json.dumps(asdict(event))`, enforces PIPE_BUF: if encoded line > 4096 bytes, replace detail with `{"_truncated": True, "_original_size": N}`. If STILL > 4096 after truncation, truncate name to 64 chars. Append to file, increment seq
  - Convenience methods: `step()`, `file_read(path, lines, size_bytes)`, `file_write(path, changes, size_bytes)`, `file_delete(path)`, `command_run(command, exit_code, duration_ms, truncated)`, `observation(message, detail)`, `blocker(blocker_type, message, suggestion, field)`, `error(message, detail)`, `artifact(name, path, mime_type, size_bytes)`, `progress(percent, message)`, `checkpoint(name, detail)`
  - `close()` — emits `_summary` event with `total_events` and `total_ms`
  - Use monotonic clock for timestamps (`time.monotonic()`)
  - Create `container/openclaw/tests/test_events.py` with 7 tests per Phase 2 test plan (section 4.4)

  **Must NOT do**:
  - Do NOT use `print()` for event output — must write to `_events.jsonl` via file append
  - Do NOT allow any line to exceed 4096 bytes even after truncation fallback
  - Do NOT reset the seq counter between calls

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 8, 10)
  - **Blocks**: Tasks 9, 11
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `container/openclaw/result_writer.py:20-31` — Existing `build_result()` pattern for atomic JSON write to state dir
  - `container/openclaw/step_config.py` — StepConfig pattern showing how config flows into the container

  **API/Type References**:
  - `bridge/pkg/secretary/result.go` — `StepEvent` Go struct (from Task 4) — the Python `StepEvent` must produce JSON with matching field names (`seq`, `type`, `name`, `ts_ms`, `detail`, `duration_ms`)

  **WHY Each Reference Matters**:
  - `result_writer.py:20-31` shows the existing atomic write pattern and state_dir conventions that EventEmitter must follow
  - Go's `StepEvent` struct defines the wire contract — Python must emit JSON with exactly matching field names

  **Acceptance Criteria**:

  - [ ] `python -m pytest container/tests/test_events.py -v` → PASS (7 tests)
  - [ ] Every line in `_events.jsonl` parses as valid JSON (no partial writes)
  - [ ] Event with detail > 4096 bytes produces truncated line containing `{"_truncated": true, "_original_size": N}` that still parses as valid JSON
  - [ ] `close()` emits final `_summary` event with correct total count

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: PIPE_BUF enforcement on large detail
    Tool: Bash (pytest)
    Preconditions: Clean temp directory
    Steps:
      1. Create EventEmitter for temp dir
      2. Emit event with detail containing a 5000-byte string value
      3. Read _events.jsonl, parse each line as JSON
    Expected Result: All lines parse. The large-detail event has detail._truncated == true and detail._original_size > 4096
    Failure Indicators: JSON parse error, line > 4096 bytes, missing _truncated marker
    Evidence: .sisyphus/evidence/task-7-pipebuf-enforcement.txt

  Scenario: All event types produce valid JSON
    Tool: Bash (pytest)
    Preconditions: Clean temp directory
    Steps:
      1. Create EventEmitter
      2. Call each convenience method: step(), file_read(), file_write(), file_delete(), command_run(), observation(), blocker(), error(), artifact(), progress(), checkpoint()
      3. Close emitter
      4. Read _events.jsonl, skip header line, parse every data line
    Expected Result: 11 event lines + 1 _summary line, all parse as valid JSON with correct type field
    Failure Indicators: Any line fails JSON parse, wrong type value
    Evidence: .sisyphus/evidence/task-7-all-types.txt
  ```

  **Commit**: YES
  - Message: `feat(container): add EventEmitter with PIPE_BUF enforcement`
  - Files: `container/openclaw/events.py, container/openclaw/tests/test_events.py`
  - Pre-commit: `python -m pytest container/tests/test_events.py -v`

- [x] 8. Enriched Result Writer (write_enriched_result)

  **What to do**:
  - Modify `container/openclaw/result_writer.py` to add `write_enriched_result()` function
  - Function signature: `write_enriched_result(state_dir, status, output, data=None, error=None, duration_ms=0, comments=None, blockers=None, skill_candidates=None, events_summary=None)`
  - Build base result dict with the 5 existing fields: `status`, `output`, `data`, `error`, `duration_ms`
  - Append underscore-prefixed fields only when non-empty: `_comments`, `_blockers`, `_skill_candidates`, `_events_summary`
  - Use existing `_atomic_write_json()` helper to write `result.json`
  - Existing `write_result()` function must remain unchanged (backward compat)
  - Create/extend `container/openclaw/tests/test_result_writer.py` with 3 tests per Phase 2 test plan (section 4.4)

  **Must NOT do**:
  - Do NOT modify the existing `write_result()` function signature or behavior
  - Do NOT include empty underscore fields — omit them entirely from JSON output
  - Do NOT change the base 5 field names or format

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7, 10)
  - **Blocks**: Tasks 9, 11
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `container/openclaw/result_writer.py:20-31` — Existing `build_result()` and `_atomic_write_json()` patterns to reuse

  **API/Type References**:
  - `bridge/pkg/secretary/result.go` — Go `ParseExtendedStepResult()` (from Task 4) must be able to read Python's output

  **WHY Each Reference Matters**:
  - `_atomic_write_json()` is the existing safe write helper — reuse it, do not create a parallel write function
  - Go's parser expects underscore-prefixed keys exactly matching `_comments`, `_blockers`, `_skill_candidates`, `_events_summary`

  **Acceptance Criteria**:

  - [ ] `python -m pytest container/tests/test_result_writer.py -v` → PASS (3+ tests)
  - [ ] All 5 base fields always present in output JSON
  - [ ] Null/empty underscore fields completely absent from output JSON
  - [ ] Non-empty underscore fields present with correct `_` prefix
  - [ ] Existing `write_result()` still works unchanged

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All underscore fields present when provided
    Tool: Bash (pytest)
    Preconditions: Clean temp directory
    Steps:
      1. Call write_enriched_result with comments=["Note 1"], blockers=[{"blocker_type":"auth"}], skill_candidates=[{"name":"skill1"}], events_summary={"total":5,"types":{"step":3}}
      2. Read and parse result.json
    Expected Result: JSON contains _comments, _blockers, _skill_candidates, _events_summary with correct values
    Failure Indicators: Any underscore field missing, values mismatched
    Evidence: .sisyphus/evidence/task-8-enriched-result.txt

  Scenario: Empty fields omitted, base fields always present
    Tool: Bash (pytest)
    Preconditions: Clean temp directory
    Steps:
      1. Call write_enriched_result with only base fields (no comments, no blockers, etc.)
      2. Read and parse result.json
    Expected Result: JSON contains exactly: status, output, data, error, duration_ms. No underscore keys present.
    Failure Indicators: Underscore keys present with null/empty values, base field missing
    Evidence: .sisyphus/evidence/task-8-omit-empty.txt
  ```

  **Commit**: YES
  - Message: `feat(container): add enriched result writer`
  - Files: `container/openclaw/result_writer.py, container/openclaw/tests/test_result_writer.py`
  - Pre-commit: `python -m pytest container/tests/test_result_writer.py -v`

- [x] 9. StepRunner Integration (Inject EventEmitter via Config Dict)

  **What to do**:
  - Modify `container/openclaw/step_runner.py` to integrate EventEmitter into the step execution flow
  - Create EventEmitter at the start of `run_step()`, close it in a `finally` block
  - Inject emitter, comments list, and blockers list into the config dict BEFORE calling the handler:
    ```python
    config["_emitter_ref"] = emitter
    config["_comments"] = []
    config["_blockers"] = []
    ```
  - Handler signature stays `(cfg) -> str` — do NOT change it
  - After handler returns, extract comments and blockers from the config dict:
    ```python
    comments = config.get("_comments", [])
    blockers = config.get("_blockers", [])
    ```
  - Log `_retry` context as observation event if present in config
  - Log `_blocker_response` as observation event if present in config
  - Log `relevant_skills` as observation event if present in config
  - After handler returns, also call `_extract_blockers_from_events(state_dir)` and merge with any blockers from config dict
  - Call `_summarize_events(state_dir)` to build events_summary
  - Call `write_enriched_result()` (from Task 8) instead of `write_result()`
  - On exception: emit error event, write enriched result with `status="failed"`
  - Add `_extract_blockers_from_events()` and `_summarize_events()` helper functions
  - Ensure state dir path includes instance ID to prevent collision when same definition runs twice (check if factory.Spawn() already passes an instance ID in config — if so, use it)
  - Create/extend `container/openclaw/tests/test_step_runner.py` with 5 tests

  **Must NOT do**:
  - Do NOT change the handler function signature — it stays `(cfg) -> str`
  - Do NOT add emitter, comments, or blockers as function parameters to handlers
  - Do NOT require handlers to import events.py — handlers that don't check `cfg.get("_emitter_ref")` work unchanged
  - Do NOT fail the step if EventEmitter operations fail — event emission is best-effort
  - Do NOT leak the EventEmitter — always close in `finally` block

  **Handler usage pattern** (for reference — NOT a change to make):
  ```python
  # Existing handler — continues to work unchanged:
  def my_handler(cfg):
      return "done"

  # Updated handler — opt-in to event emission:
  def my_handler(cfg):
      emitter = cfg.get("_emitter_ref")
      if emitter:
          emitter.step("starting work")
      # ... do work ...
      if need_password:
          blockers = cfg.get("_blockers", [])
          blockers.append({"blocker_type": "auth", "message": "need password"})
      return "done"
  ```

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (sequential within wave)
  - **Blocks**: Task 11
  - **Blocked By**: Tasks 7, 8

  **References**:

  **Pattern References**:
  - `container/openclaw/step_runner.py` — Existing 83-line `run_step()` showing handler call pattern: `result = handler(config)`
  - `container/openclaw/step_config.py` — StepConfig showing how config dict flows to handler

  **API/Type References**:
  - `container/openclaw/events.py` (Task 7) — EventEmitter API

  **WHY Each Reference Matters**:
  - `step_runner.py` shows the existing `handler(config)` call — the call site does NOT change, only the config dict is enriched before the call
  - Handlers access the emitter through the config dict, keeping the function signature stable

  **Acceptance Criteria**:

  - [ ] `python -m pytest container/tests/test_step_runner.py -v` → PASS (5 tests)
  - [ ] Handler function signature unchanged: still `(cfg) -> str`
  - [ ] `_events.jsonl` written during step execution with at least checkpoint + observation events
  - [ ] `result.json` includes `_events_summary` after step completes
  - [ ] Handler that accesses `cfg["_emitter_ref"]` can emit events
  - [ ] Handler that does NOT access `cfg["_emitter_ref"]` still works (no error)
  - [ ] Handler that appends to `cfg["_blockers"]` causes blockers to appear in result.json
  - [ ] On exception: error event emitted, enriched result written with status=failed
  - [ ] State dir path includes instance ID (no collision when same definition runs twice)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Handler with event emission via config dict
    Tool: Bash (pytest)
    Preconditions: Clean temp dir, mock handler that does `cfg["_emitter_ref"].step("test")` then returns `{"status":"success","output":"done"}`
    Steps:
      1. Run step with config={"type":"test","task":"test task"}
      2. Verify _events.jsonl exists with step event named "test"
      3. Verify result.json has _events_summary with total >= 1
    Expected Result: Event emitted through config dict accessor, result.json has summary
    Failure Indicators: No events in _events.jsonl, handler crashed
    Evidence: .sisyphus/evidence/task-9-config-injection.txt

  Scenario: Legacy handler without emitter access still works
    Tool: Bash (pytest)
    Preconditions: Clean temp dir, mock handler that just returns `{"status":"success","output":"done"}` without touching _emitter_ref
    Steps:
      1. Run step with config={"type":"test","task":"test task"}
      2. Verify no error raised
      3. Verify result.json has status="success"
    Expected Result: Handler runs successfully, result written
    Failure Indicators: Handler crashes, KeyError on _emitter_ref
    Evidence: .sisyphus/evidence/task-9-legacy-handler.txt

  Scenario: Handler reports blocker via config dict
    Tool: Bash (pytest)
    Preconditions: Mock handler that does `cfg["_blockers"].append({"blocker_type":"auth","message":"need password"})` then returns `{"status":"success","output":"blocked"}`
    Steps:
      1. Run step
      2. Verify result.json has _blockers with message "need password"
    Expected Result: Blocker from config dict appears in result.json
    Failure Indicators: _blockers is empty or missing
    Evidence: .sisyphus/evidence/task-9-blocker-via-config.txt
  ```

  **Commit**: YES
  - Message: `feat(container): integrate EventEmitter via config dict injection`
  - Files: `container/openclaw/step_runner.py, container/openclaw/tests/test_step_runner.py`
  - Pre-commit: `python -m pytest container/tests/test_step_runner.py -v`

- [x] 10. _blocker_response and _retry Parsing in StepConfig

  **What to do**:
  - Modify `container/openclaw/step_config.py` to add parsing for new config keys
  - Add `_retry` property to StepConfig: returns dict with `attempt` and `previous_result` or None
  - Add `_blocker_response` property to StepConfig: returns dict with `input`, `provided_at`, `resolved_by`, `note` or None
  - Add `relevant_skills` property to StepConfig: returns list of skill dicts or None
  - These are read-only accessors on the existing config dict — no structural changes to StepConfig
  - Validate that `_retry["attempt"]` is an integer >= 1 when present
  - Validate that `_blocker_response["input"]` is non-empty string when present
  - Add tests for all new properties

  **Must NOT do**:
  - Do NOT change existing StepConfig properties or methods
  - Do NOT add write methods — these are read-only views into the config dict
  - Do NOT raise exceptions on missing keys — return None

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7, 8)
  - **Blocks**: Task 9
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `container/openclaw/step_config.py` — Existing 56-line StepConfig class showing property accessor pattern

  **API/Type References**:
  - `container/openclaw/step_runner.py` — Consumer of StepConfig properties (Task 9 will call these)

  **WHY Each Reference Matters**:
  - `step_config.py` at 56 lines shows the minimal property accessor pattern to follow
  - StepRunner needs these properties to log retry/blocker context as observation events

  **Acceptance Criteria**:

  - [ ] `python -m pytest container/tests/ -v -k "step_config"` → PASS
  - [ ] `_retry` returns None when key absent, returns dict when present
  - [ ] `_blocker_response` returns None when key absent, returns dict when present
  - [ ] `relevant_skills` returns None when key absent, returns list when present

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Parse _blocker_response from config
    Tool: Bash (pytest)
    Preconditions: StepConfig with config dict containing _blocker_response key
    Steps:
      1. Create StepConfig({"_blocker_response": {"input": "my-password", "provided_at": 123456, "resolved_by": "user1"}})
      2. Access _blocker_response property
    Expected Result: Returns dict with input="my-password", provided_at=123456, resolved_by="user1"
    Failure Indicators: Returns None, raises KeyError, returns wrong structure
    Evidence: .sisyphus/evidence/task-10-blocker-response.txt

  Scenario: Missing keys return None
    Tool: Bash (pytest)
    Steps:
      1. Create StepConfig({"type": "test"})
      2. Access _retry, _blocker_response, relevant_skills properties
    Expected Result: All three return None
    Failure Indicators: Any raises exception or returns non-None
    Evidence: .sisyphus/evidence/task-10-missing-keys.txt
  ```

  **Commit**: YES
  - Message: `feat(container): add _blocker_response and _retry parsing to StepConfig`
  - Files: `container/openclaw/step_config.py`
  - Pre-commit: `python -m pytest container/tests/ -k "step_config" -v`

- [x] 11. Container Tests (Events, Result Writer, Step Runner)

  **What to do**:
  - Create comprehensive integration tests spanning all Phase 2 components
  - `container/openclaw/tests/test_events.py` — 7 tests: TestEventEmitterBasic, TestEventEmitterAllTypes, TestEventEmitterClose, TestEventEmitterPipeBufEnforcement, TestEventEmitterPipeBufEdgeCase, TestEventEmitterPipeBufExtreme, TestEventEmitterNoPartialWrites
  - Extend `container/openclaw/tests/test_result_writer.py` — 3 tests: TestWriteEnrichedResultAllFields, TestWriteEnrichedResultOmitsEmpty, TestWriteEnrichedResultBackwardCompat
  - Extend `container/openclaw/tests/test_step_runner.py` — 5 tests: TestStepRunnerWithEmitter, TestStepRunnerRetryContext, TestStepRunnerBlockerResponse, TestStepRunnerLearnedSkills, TestStepRunnerException
  - Cross-component test: emit events, write enriched result, verify Phase 1 Go parser (`ParseExtendedStepResult` from Task 4) can read the output
  - All tests use temp directories, clean up after themselves

  **Must NOT do**:
  - Do NOT require a running Bridge or Docker for unit tests
  - Do NOT modify production code to make tests pass — fix the code, do not weaken the tests
  - Do NOT use `mock.patch` on EventEmitter internals — test real file writes

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (final task in phase)
  - **Blocks**: Phase 3 (all tasks)
  - **Blocked By**: Tasks 7, 8, 9

  **References**:

  **Pattern References**:
  - `container/openclaw/result_writer.py` — Existing test patterns for result writer tests
  - `bridge/pkg/secretary/result.go` — Go side of the cross-component contract

  **API/Type References**:
  - `container/openclaw/events.py` (Task 7) — EventEmitter full API
  - `container/openclaw/result_writer.py` (Task 8) — `write_enriched_result()` signature
  - `container/openclaw/step_runner.py` (Task 9) — `run_step()` with emitter integration

  **WHY Each Reference Matters**:
  - Cross-component validation ensures the Python container output matches what Go's `ParseExtendedStepResult` expects
  - The Phase 2 exit gate requires Phase 1 Go code to read Phase 2 Python output correctly

  **Acceptance Criteria**:

  - [ ] `python -m pytest container/tests/ -v` → PASS (15 tests across 3 files)
  - [ ] TestEventEmitterPipeBufEnforcement: detail > 4096 bytes truncated, line still parses
  - [ ] TestEventEmitterNoPartialWrites: every line in file parses as valid JSON
  - [ ] TestWriteEnrichedResultBackwardCompat: base 5 fields always present
  - [ ] TestStepRunnerException: error event in _events.jsonl, status="failed" in result.json
  - [ ] Cross-component: result.json parseable by Phase 1 `ExtendedStepResult` types

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Full container emission cycle
    Tool: Bash (pytest)
    Preconditions: All Phase 2 files present (events.py, result_writer.py, step_runner.py)
    Steps:
      1. Create StepRunner with mock handler that emits step + file_read + artifact events
      2. Run step, verify _events.jsonl has 3+ valid JSON lines
      3. Verify result.json has _events_summary.total matching event count
      4. Read result.json with Go-style parsing (json.loads, check underscore keys)
    Expected Result: _events.jsonl has correct events, result.json has all underscore fields, cross-parses cleanly
    Failure Indicators: Missing events, wrong counts, parse errors on either side
    Evidence: .sisyphus/evidence/task-11-full-cycle.txt

  Scenario: Old container output still valid
    Tool: Bash (pytest)
    Preconditions: result.json with only base 5 fields (no underscore keys, no _events.jsonl)
    Steps:
      1. Write a plain result.json with status/output/data/error/duration_ms
      2. Verify write_result() (old function) still works
      3. Verify enriched parser handles missing underscore keys gracefully
    Expected Result: Old format parses without error, all base fields accessible
    Failure Indicators: Parse error, missing base fields, exception on absent underscore keys
    Evidence: .sisyphus/evidence/task-11-backward-compat.txt
  ```

  **Commit**: YES
  - Message: `test(container): comprehensive Phase 2 test suite`
  - Files: `container/openclaw/tests/test_events.py, container/openclaw/tests/test_result_writer.py, container/openclaw/tests/test_step_runner.py`
  - Pre-commit: `python -m pytest container/tests/ -v`

---

- [x] 12. Polling Loop in waitForCompletion (Tail _events.jsonl)

  **What to do**:
  - Modify `bridge/pkg/secretary/orchestrator_integration.go` function `waitForCompletion()` to integrate EventReader
  - Create `EventReader` at start of function, use 500ms ticker (existing pattern at line 321-349)
  - On each tick: read Docker status, check completion, then tail new events via `reader.ReadNew()`
  - For each new event: route by type to appropriate emission method (step/file/progress → EmitProgress, error → EmitStepError, blocker → EmitBlockerWarning)
  - On container completion (exit 0 or non-zero): call `ParseExtendedStepResult()` → `cleanupStateDir()` → send timeline/comment notifications from parsed data in memory
  - On `ErrEventLogExceeded`: call `factory.Kill(instanceID)` (SIGKILL), then `cleanupStateDir()`, then `FailWorkflow()`
  - On context cancellation: `cleanupStateDir()`, return error
  - This replaces the existing simple Docker-status-only polling with event-aware polling

  **Must NOT do**:
  - Do NOT change the 500ms polling interval
  - Do NOT block on event reading — ReadNew() must be non-blocking
  - Do NOT hold events in memory after emission — the reader tracks offset internally

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (sequential start)
  - **Blocks**: Task 15
  - **Blocked By**: Tasks 2, 5, 6

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:321-349` — Existing `waitForCompletion()` polling loop (500ms ticker, Docker status check) — this is the EXACT function to modify
  - `bridge/pkg/secretary/event_reader.go` (Task 2) — `ReadNew()` API: returns `([]StepEvent, int64, error)`

  **API/Type References**:
  - `bridge/pkg/secretary/result.go` (Task 4) — `ParseExtendedStepResult()`, `ExtendedStepResult`
  - `bridge/pkg/secretary/cleanup.go` (Task 3) — `cleanupStateDir()`
  - `bridge/pkg/studio/factory.go` (Task 5) — `Kill()` method

  **WHY Each Reference Matters**:
  - `orchestrator_integration.go:321-349` is the EXACT function being rewritten — existing pattern must be preserved
  - Task 2's `ReadNew()` is the incremental tail API being consumed here
  - Task 5's `Kill()` is the SIGKILL path for 10MB overflow

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/secretary/... -run TestTail` → PASS
  - [ ] Bridge tails `_events.jsonl` during live execution, reads new events each 500ms tick
  - [ ] Zero duplicate events across 5+ poll cycles (offset tracking)
  - [ ] On container exit: parse → purge → notify ordering verified

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Incremental tail reads during execution
    Tool: Bash (go test)
    Preconditions: Mock factory returning "running" status, temp _events.jsonl growing by 2 events per tick
    Steps:
      1. Start waitForCompletion with mock factory
      2. Append 2 events to _events.jsonl
      3. Wait 1 tick (500ms), append 2 more events
      4. Wait another tick
      5. Set mock status to "completed"
    Expected Result: EventReader reads exactly 2 new events per tick, never duplicates, total 4 events received
    Failure Indicators: More than 2 events per tick, zero events on later ticks, duplicate seq numbers
    Evidence: .sisyphus/evidence/task-12-tail-reads.txt

  Scenario: 10MB cap triggers Kill not Stop
    Tool: Bash (go test)
    Preconditions: _events.jsonl file at 10MB + 1 byte, mock factory
    Steps:
      1. Start waitForCompletion
      2. ReadNew() returns ErrEventLogExceeded
      3. Verify factory.Kill() was called (not factory.Stop())
      4. Verify cleanupStateDir() was called after Kill
    Expected Result: Kill() called with SIGKILL, Stop() never called, state dir purged
    Failure Indicators: Stop() called instead of Kill(), state dir still exists, no error returned
    Evidence: .sisyphus/evidence/task-12-10mb-kill.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add event streaming in waitForCompletion`
  - Files: `bridge/pkg/secretary/orchestrator_integration.go, bridge/pkg/secretary/orchestrator_integration_test.go`
  - Pre-commit: `go test ./pkg/secretary/... -run TestTail`

- [x] 13. Progress Event Emission to MatrixEventBus

  **What to do**:
  - Modify `bridge/pkg/secretary/orchestrator_events.go` to add event emission methods on `WorkflowEventEmitter`
  - Add `EmitProgress(workflowID string, event StepEvent)` — builds `WorkflowEvent` with type `workflow.progress`, extracts percent from detail if type is "progress", publishes to bus
  - Add `EmitStepError(workflowID string, event StepEvent)` — builds `WorkflowEvent` with type `workflow.step_error`, publishes to bus
  - Add `EmitBlockerWarning(workflowID string, event StepEvent)` — builds `WorkflowEvent` with type `workflow.blocker_warning`, publishes to bus
  - Define `ProgressDetail` struct: `EventSeq int`, `EventType string`, `StepName string`, `ElapsedMs int`, `Detail map[string]interface{}`
  - All methods publish via `e.bus.Publish()` using the existing MatrixEventBus
  - Create tests verifying events are published with correct type and detail

  **Must NOT do**:
  - Do NOT create a parallel event system — extend the existing `WorkflowEventEmitter` and `MatrixEventBus`
  - Do NOT block on publish — bus.Publish must be synchronous but fast
  - Do NOT filter events here — filtering is the bus consumer's job

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (sequential after Task 12)
  - **Blocks**: Task 16
  - **Blocked By**: Task 12

  **References**:

  **Pattern References**:
  - `bridge/internal/events/matrix_event_bus.go` — `MatrixEventBus.Publish()` method — THIS is the correct bus to use (not `pkg/eventbus/EventBus`). MatrixEventBus is the ring buffer that feeds into the existing `processEvents()` loop, which delivers events to ArmorChat via Matrix /sync. The other EventBus (`pkg/eventbus/`) is for external WebSocket clients and is currently disconnected from Matrix delivery.

  **API/Type References**:
  - `bridge/pkg/secretary/orchestrator_events.go` — Existing `WorkflowEventEmitter` struct and its `bus` field (typed as `*MatrixEventBus`)
  - `bridge/internal/events/matrix_event_bus.go:Publish()` — Method signature: `Publish(event *MatrixEvent)`

  **WHY Each Reference Matters**:
  - `orchestrator_events.go` already holds a `*MatrixEventBus` reference — the new progress methods emit to this bus
  - Using MatrixEventBus (not EventBus) ensures progress events flow through the existing Matrix /sync path that ArmorChat already consumes without any new transport infrastructure

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/secretary/... -run TestEmitProgress` → PASS
  - [ ] `EmitProgress()` publishes WorkflowEvent with correct type and detail
  - [ ] `EmitStepError()` publishes WorkflowEvent with step error detail
  - [ ] `EmitBlockerWarning()` publishes WorkflowEvent with blocker detail
  - [ ] Progress percent extracted correctly from detail map when present

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Progress event published with percent
    Tool: Bash (go test)
    Preconditions: Mock EventBus that records published events
    Steps:
      1. Call EmitProgress("wf-123", StepEvent{Seq:5, Type:"progress", Name:"50% complete", Detail:{"percent": 50}})
      2. Verify mock received one event with type "workflow.progress"
      3. Verify Progress.Percent == 50
    Expected Result: One event published with percent=50, message="50% complete"
    Failure Indicators: Zero events published, wrong type, percent not 50
    Evidence: .sisyphus/evidence/task-13-progress-emit.txt

  Scenario: Step error event published
    Tool: Bash (go test)
    Steps:
      1. Call EmitStepError("wf-123", StepEvent{Type:"error", Name:"timeout", Detail:{"message":"connection lost"}})
      2. Verify published event type is "workflow.step_error"
    Expected Result: Event with correct type and error detail published
    Failure Indicators: Wrong event type, missing detail fields
    Evidence: .sisyphus/evidence/task-13-step-error.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add progress event emission to Matrix`
  - Files: `bridge/pkg/secretary/orchestrator_events.go, bridge/pkg/secretary/orchestrator_events_test.go`
  - Pre-commit: `go test ./pkg/secretary/... -run TestEmitProgress`

- [x] 14. Timeline Formatter (FormatTimelineMessage)

  **What to do**:
  - Modify `bridge/pkg/secretary/notifications.go` to add `FormatTimelineMessage(result *ExtendedStepResult) string`
  - Build timeline string: title line with result.Output, separator, then one line per event with icon + name + duration
  - Implement `stepIcon(eventType string) string` — returns emoji for each event type: 🔹step, 📄file_read, ✏️file_write, 🗑️file_delete, ⌨️command_run, 💭observation, 🚧blocker, ❌error, 📦artifact, 🏁checkpoint
  - Skip `_summary` and `progress` events from timeline display
  - Show sub-details: file_read shows line count, command_run shows exit code with ✓/✗, blocker shows message, truncated shows warning
  - Footer: total duration in seconds + step count from EventsSummary
  - Fallback to plain result.Output when no events
  - Create tests in `notifications_test.go`

  **Must NOT do**:
  - Do NOT access the filesystem — work from `ExtendedStepResult` in memory only
  - Do NOT include progress events in the timeline (they are intermediate, not interesting for final view)
  - Do NOT panic on nil EventsSummary or empty Events slice

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (independent of Tasks 12-13)
  - **Blocks**: Tasks 15, 20, 26
  - **Blocked By**: Task 4

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/notifications.go` — Existing notification formatting functions (message construction pattern)
  - `bridge/pkg/secretary/result.go` (Task 4) — `ExtendedStepResult` struct with Events, EventsSummary, Comments

  **API/Type References**:
  - `bridge/pkg/secretary/result.go` — `StepEvent` struct with Type, Name, DurationMs, Detail fields

  **WHY Each Reference Matters**:
  - `notifications.go` shows the existing message formatting style — timeline must match
  - `ExtendedStepResult` in memory is the data source — the formatter must handle nil/empty gracefully since purge happens before notification

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/secretary/... -run TestTimeline` → PASS (3 tests)
  - [ ] Timeline renders correctly with icons, names, durations from parsed events
  - [ ] Empty Events falls back to plain output text
  - [ ] Truncated detail shown gracefully (no crash)
  - [ ] Footer shows correct total duration and step count

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Timeline from parsed events
    Tool: Bash (go test)
    Preconditions: ExtendedStepResult with 4 events: step, file_read(100 lines), command_run(exit 0), artifact
    Steps:
      1. Call FormatTimelineMessage(result)
      2. Verify output contains title, separator, 4 event lines with icons, footer with duration and count
    Expected Result: Output has "🔹 step_name", "📄 Read path" with "100 lines", "⌨️ Ran: cmd" with "✓ exit 0", "📦 Produced: artifact", footer with duration and step count
    Failure Indicators: Missing icons, wrong line count, missing footer, crash on detail access
    Evidence: .sisyphus/evidence/task-14-timeline.txt

  Scenario: Empty events fallback
    Tool: Bash (go test)
    Preconditions: ExtendedStepResult with empty Events slice, Output="Task complete"
    Steps:
      1. Call FormatTimelineMessage(result)
    Expected Result: Returns "Task complete" (plain output, no timeline formatting)
    Failure Indicators: Empty string, crash, "📋" prefix on plain text
    Evidence: .sisyphus/evidence/task-14-empty-timeline.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add timeline formatter`
  - Files: `bridge/pkg/secretary/notifications.go, bridge/pkg/secretary/notifications_test.go`
  - Pre-commit: `go test ./pkg/secretary/... -run TestTimeline`

- [x] 15. Purge Ordering (Parse → RemoveAll → Notify) + 10MB Kill Flow

  **What to do**:
  - Modify `bridge/pkg/secretary/orchestrator_integration.go` to enforce strict purge ordering in `waitForCompletion()`
  - On completion path: (1) call `ParseExtendedStepResult(stateDir)` → (2) call `cleanupStateDir(stateDir)` → (3) send timeline/comment notifications from parsed data in memory
  - On 10MB overflow path: (1) call `factory.Kill(instanceID)` → (2) call `cleanupStateDir(stateDir)` → (3) call `orchestrator.FailWorkflow()`
  - On cancellation path: call `cleanupStateDir(stateDir)`, return error
  - On failure path (non-zero exit): same parse → purge → notify ordering
  - Add tests verifying call order using mock recorder: cleanup called before notification, parse called before cleanup
  - Verify state directory does not exist after any exit path

  **Must NOT do**:
  - Do NOT send notifications before cleanup — purge must happen first (ADR-003)
  - Do NOT skip cleanup on any path — even error paths must purge
  - Do NOT hold file handles open after purge — all data must be in Go memory

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (sequential after Tasks 12, 14)
  - **Blocks**: Phase 4 (Task 17)
  - **Blocked By**: Tasks 12, 14

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:321-349` — Existing `waitForCompletion()` — this is where ordering must be enforced
  - `bridge/pkg/secretary/cleanup.go` (Task 3) — `cleanupStateDir()` API

  **API/Type References**:
  - `bridge/pkg/secretary/result.go` (Task 4) — `ParseExtendedStepResult()` — must capture all data before purge
  - `bridge/pkg/studio/factory.go` (Task 5) — `Kill()` for SIGKILL path

  **WHY Each Reference Matters**:
  - Purge ordering is ADR-003: state dir destroyed before Matrix notification. This minimizes the window where `_events.jsonl` exists on VPS disk
  - The parse step must extract everything into Go memory because the files are destroyed immediately after

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/secretary/... -run TestPurgeOrder` → PASS
  - [ ] Test verifies: parse called → cleanup called → notify called (strict order)
  - [ ] State directory verified absent after normal completion
  - [ ] State directory verified absent after failure
  - [ ] State directory verified absent after cancellation
  - [ ] State directory verified absent after 10MB kill
  - [ ] Comments posted to Matrix after state dir deleted (from memory)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Parse → Purge → Notify ordering
    Tool: Bash (go test)
    Preconditions: Mock notification service and cleanup recorder, temp state dir with events + result.json
    Steps:
      1. Run waitForCompletion to completion
      2. Check recorded call order: ParseExtendedStepResult → cleanupStateDir → Notify
    Expected Result: Calls happened in exact order: parse, cleanup, notify. No reordering.
    Failure Indicators: Notify before cleanup, cleanup before parse, any call missing
    Evidence: .sisyphus/evidence/task-15-purge-order.txt

  Scenario: 10MB kill purges state dir
    Tool: Bash (go test)
    Preconditions: _events.jsonl at 10MB+1, mock factory
    Steps:
      1. Run waitForCompletion
      2. Verify Kill() called
      3. Verify cleanupStateDir() called
      4. Verify FailWorkflow() called with message about "exceeded 10MB"
      5. Verify state dir does not exist
    Expected Result: Kill → cleanup → FailWorkflow in order, state dir gone
    Failure Indicators: State dir still exists, Kill not called, FailWorkflow not called
    Evidence: .sisyphus/evidence/task-15-kill-purge.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): enforce purge ordering and 10MB kill`
  - Files: `bridge/pkg/secretary/orchestrator_integration.go, bridge/pkg/secretary/orchestrator_integration_test.go`
  - Pre-commit: `go test ./pkg/secretary/... -run TestPurge`

- [x] 16. Matrix Event Type Handling in processEvents

  **What to do**:
  - Locate the `processEvents()` method in `bridge/internal/adapter/` (the Matrix adapter's sync loop that handles incoming Matrix events)
  - The current implementation filters for `m.room.message` events and drops everything else (including custom event types like `app.armorclaw.*` and workflow events)
  - Add handling for the following event types that the Bridge itself publishes to Matrix rooms:
    - `workflow.progress` — Forward to `ControlPlaneStore` for UI rendering
    - `workflow.step_error` — Forward to `ControlPlaneStore`
    - `workflow.blocker_warning` — Forward to `ControlPlaneStore`
    - `workflow.blocked` — Forward to `ControlPlaneStore`
    - `workflow.timeline` — Forward to `ControlPlaneStore`
    - `agent.comment` — Forward to `ControlPlaneStore`
    - `blocker.required` — Forward to `ControlPlaneStore`
  - The handling pattern: match on event type prefix (`workflow.`, `agent.`, `blocker.`), parse the content, call the appropriate method on `ControlPlaneStore` (or equivalent event processor)
  - Do NOT change how `m.room.message` events are handled — that path stays unchanged
  - Do NOT change how `m.call.*` events are handled (voice stack)
  - Ensure unrecognized custom event types are logged at DEBUG level but not dropped silently

  **Must NOT do**:
  - Do NOT use `pkg/eventbus/EventBus` for this — the events come through Matrix /sync via `MatrixEventBus`
  - Do NOT add WebSocket handling — ArmorChat has no WebSocket client
  - Do NOT change the existing message processing path

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (after Task 13)
  - **Blocks**: Phase 4
  - **Blocked By**: Task 13

  **References**:

  **Pattern References**:
  - `bridge/internal/adapter/matrix.go` — `processEvents()` method where Matrix sync events are dispatched
  - `bridge/internal/adapter/matrix.go` — Existing `m.room.message` handling pattern (the model for how to handle a new event type)

  **API/Type References**:
  - `shared/src/commonMain/kotlin/data/store/ControlPlaneStore.kt` — ArmorChat's event processor that subscribes to room events

  **WHY Each Reference Matters**:
  - `processEvents()` is the exact method that drops non-message events — this is where the fix goes
  - `ControlPlaneStore` in ArmorChat already has the subscription infrastructure — it just needs the Bridge to actually send these events through Matrix

  **Acceptance Criteria**:

  - [ ] `go test -v ./internal/adapter/... -run TestProcessEvents` → PASS
  - [ ] `workflow.progress` events from MatrixEventBus are published to the Matrix room
  - [ ] `workflow.blocked` events are published to the Matrix room
  - [ ] Existing `m.room.message` handling unchanged
  - [ ] Unrecognized custom events logged at DEBUG, not dropped silently

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: workflow.progress event delivered to Matrix room
    Tool: Bash (go test)
    Preconditions: Mock Matrix client that captures sent events
    Steps:
      1. Publish a workflow.progress event to MatrixEventBus
      2. Trigger processEvents() cycle
      3. Verify Matrix client received a room event with type containing "workflow.progress"
    Expected Result: Event sent to Matrix room via the adapter
    Failure Indicators: Event not sent, event dropped silently
    Evidence: .sisyphus/evidence/task-16-progress-delivery.txt
  ```

  **Commit**: YES
  - Message: `feat(adapter): handle custom workflow events in processEvents`
  - Files: `bridge/internal/adapter/matrix.go, bridge/internal/adapter/matrix_test.go`
  - Pre-commit: `go test ./internal/adapter/... -run TestProcessEvents`

---

- [x] 17. Blocker Loop with PII Safety (executeStepWithBlockerHandling)

  **What to do**:
  - Modify `bridge/pkg/secretary/orchestrator_integration.go` to add `executeStepWithBlockerHandling()` method on `StepExecutor`
  - Loop up to `MaxBlockerRetries` (3) attempts: spawn container → waitForCompletion → check for blockers
  - If result has `Blockers`: call `orchestrator.BlockWorkflow()` → send blocker notification → call `waitForBlockerResponse()` → append response to config → re-spawn
  - If no blockers: call `orchestrator.CompleteWorkflow()`, return success
  - On `ErrEventLogExceeded`: return immediately (already handled by waitForCompletion)
  - `appendBlockerResponse(config, response)` — adds `_blocker_response` dict to STEP_CONFIG JSON with `provided_at`, `input`, `resolved_by`, `note`
  - PII SAFETY: response content is NEVER logged, NEVER written to file. It flows from RPC handler memory → Go process memory → container env var (destroyed on exit)
  - Add `BlockerResponse` struct: `Input string`, `Note string`, `UserID string`
  - Define constants: `MaxBlockerRetries = 3`, `BlockerTimeout = 10 * time.Minute`

  **Must NOT do**:
  - Do NOT log the blocker response input content — PII safety is paramount
  - Do NOT write blocker response to any file on VPS disk
  - Do NOT exceed 3 retry attempts — fail the workflow after max retries
  - Do NOT allow blocker response to persist after container exit

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4 (sequential start)
  - **Blocks**: Task 21
  - **Blocked By**: Tasks 6, 12, 15

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/orchestrator_integration.go` — Existing step execution flow where blocker handling wraps
  - `bridge/pkg/secretary/orchestrator.go:440-446` — `validateTransition()` with Blocked state (from Task 6)

  **API/Type References**:
  - `bridge/pkg/secretary/result.go` (Task 4) — `ExtendedStepResult.Blockers []Blocker`
  - `bridge/pkg/secretary/cleanup.go` (Task 3) — `cleanupStateDir()` called by waitForCompletion
  - `bridge/pkg/secretary/types.go` (Task 6) — `BlockWorkflow()` method

  **WHY Each Reference Matters**:
  - The blocker loop is the most security-sensitive code in the plan — PII in blocker responses must never touch disk
  - `validateTransition()` with Blocked state (Task 6) must accept Running → Blocked transition for this to work

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/secretary/... -run TestBlockerLoop` → PASS
  - [ ] Blocker → blocked → response → re-spawn → complete flow works
  - [ ] 10-minute timeout fails workflow with clear error
  - [ ] 3 consecutive blockers fail workflow
  - [ ] `appendBlockerResponse()` adds `_blocker_response` to config JSON
  - [ ] Blocker response content absent from all log output

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Blocker resolved on first attempt
    Tool: Bash (go test)
    Preconditions: Mock factory, mock blocker response channel with immediate response
    Steps:
      1. Run executeStepWithBlockerHandling
      2. First spawn returns blockers with message "Requires password"
      3. Response delivered with input="secret123"
      4. Second spawn returns success (no blockers)
    Expected Result: Workflow completes, _blocker_response in second spawn config, response not logged
    Failure Indicators: Workflow stays blocked, response logged, second spawn has no _blocker_response
    Evidence: .sisyphus/evidence/task-17-blocker-resolve.txt

  Scenario: Max retries exceeded
    Tool: Bash (go test)
    Preconditions: Mock that always returns blockers
    Steps:
      1. Run executeStepWithBlockerHandling with a mock that returns blockers every time
      2. Verify it attempts exactly 3 spawns
      3. Verify workflow fails with "max blocker retries exceeded"
    Expected Result: 3 spawn attempts, final error about max retries
    Failure Indicators: More than 3 attempts, infinite loop, wrong error message
    Evidence: .sisyphus/evidence/task-17-max-retries.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add blocker protocol with PII safety`
  - Files: `bridge/pkg/secretary/orchestrator_integration.go, bridge/pkg/secretary/orchestrator_integration_test.go`
  - Pre-commit: `go test ./pkg/secretary/... -run TestBlocker`

- [x] 18. waitForBlockerResponse (sync.Map Channel Pattern)

  **What to do**:
  - Add `waitForBlockerResponse()` method to `StepExecutor` in `bridge/pkg/secretary/orchestrator_integration.go`
  - Use package-level `sync.Map` (`pendingBlockers`) as the channel registry
  - Key format: `"blocker:{workflowID}:{stepID}"`
  - On call: create buffered channel (cap 1), store in sync.Map, publish blocker request event to Matrix
  - Block on channel with 3-way select: receive response, timeout (10min), context cancelled
  - On completion: delete from sync.Map (defer)
  - On timeout: return error with timeout message, delete from sync.Map
  - The `resolve_blocker` RPC handler (Task 19) writes to this channel

  **Must NOT do**:
  - Do NOT leak channels — always delete from sync.Map on exit (use defer)
  - Do NOT use unbuffered channels — cap must be 1 to prevent RPC handler from blocking
  - Do NOT block the StepExecutor goroutine indefinitely — always have timeout

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4 (sequential after Task 17)
  - **Blocks**: Task 19
  - **Blocked By**: Task 17

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/orchestrator_integration.go` — Where `executeStepWithBlockerHandling` calls this function

  **API/Type References**:
  - `bridge/pkg/secretary/event_bus.go` — `Publish()` for sending blocker request event
  - `bridge/pkg/rpc/server.go` — Where `handleResolveBlocker` delivers to the channel (Task 19)

  **WHY Each Reference Matters**:
  - The sync.Map pattern is the bridge between the RPC handler (write side) and the step executor (read side)
  - Channel leak would prevent garbage collection and eventually exhaust memory on long-running bridges

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/secretary/... -run TestWaitForBlocker` → PASS
  - [ ] Returns `BlockerResponse` when channel receives value
  - [ ] Returns error on 10-minute timeout
  - [ ] Returns error on context cancellation
  - [ ] sync.Map entry deleted on all exit paths (success, timeout, cancel)
  - [ ] Channel is buffered (cap 1)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Response delivered via channel
    Tool: Bash (go test)
    Steps:
      1. Call waitForBlockerResponse in a goroutine
      2. Load channel from sync.Map using key "blocker:wf-123:step-1"
      3. Write BlockerResponse{Input:"password123"} to channel
      4. Verify goroutine returns with correct response
    Expected Result: Response received with Input="password123", sync.Map entry deleted
    Failure Indicators: Timeout, wrong response, sync.Map entry still present
    Evidence: .sisyphus/evidence/task-18-blocker-channel.txt

  Scenario: Timeout cleanup
    Tool: Bash (go test)
    Steps:
      1. Call waitForBlockerResponse with short timeout (1 second)
      2. Do NOT write to channel
      3. Wait for timeout
    Expected Result: Returns timeout error, sync.Map entry deleted
    Failure Indicators: Goroutine hangs, sync.Map entry leaked
    Evidence: .sisyphus/evidence/task-18-timeout.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add waitForBlockerResponse with sync.Map`
  - Files: `bridge/pkg/secretary/orchestrator_integration.go, bridge/pkg/secretary/orchestrator_integration_test.go`
  - Pre-commit: `go test ./pkg/secretary/... -run TestWaitForBlocker`

- [x] 19. resolve_blocker RPC Handler (Studio Namespace)

  **What to do**:
  - Modify `bridge/pkg/rpc/server.go` to add `resolve_blocker` RPC handler
  - Register handler: `handlers["resolve_blocker"] = s.handleResolveBlocker`
  - Request params: `workflow_id string`, `step_id string`, `input string`, `note string` (all required except note)
  - Validate: workflow_id, step_id, and input must be non-empty
  - Load channel from `pendingBlockers` sync.Map using key `"blocker:{workflowID}:{stepID}"`
  - If no channel found: return error "no pending blocker"
  - Build `BlockerResponse` and send to channel
  - PII SAFETY: Do NOT log `req.Input` or `resp.Input`. The security logger must not capture blocker response content. Add explicit comment in code marking this omission as intentional
  - Return `{"status": "delivered"}`

  **Must NOT do**:
  - Do NOT log the input content — this is PII (passwords, API keys, personal data)
  - Do NOT block if channel is full — it is buffered to 1, write should succeed immediately
  - Do NOT store the response on disk or in any persistent cache

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4 (sequential after Task 18)
  - **Blocks**: Tasks 21, 27, 29
  - **Blocked By**: Task 18

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/server.go` — Existing RPC handler registration pattern (`handlers["method"] = handler`)
  - `bridge/pkg/studio/rpc.go` — Studio-specific RPC handlers showing the pattern

  **API/Type References**:
  - `bridge/pkg/secretary/orchestrator_integration.go` (Task 18) — `pendingBlockers` sync.Map and `BlockerResponse` type

  **WHY Each Reference Matters**:
  - RPC handler is the ArmorChat-facing entry point for blocker resolution
  - The `pendingBlockers` sync.Map is the shared state between handler (writer) and executor (reader)

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/rpc/... -run TestResolveBlocker` → PASS (3 tests)
  - [ ] Delivers response to waiting channel
  - [ ] Returns error when no pending blocker for given IDs
  - [ ] Returns error on missing required fields
  - [ ] Security logger does NOT capture input content (verified by mock)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: RPC delivers blocker response
    Tool: Bash (go test)
    Preconditions: Channel registered in pendingBlockers for "blocker:wf-1:step-1"
    Steps:
      1. Call handleResolveBlocker with {workflow_id:"wf-1", step_id:"step-1", input:"my-secret"}
      2. Verify channel receives BlockerResponse with Input="my-secret"
      3. Verify return value is {"status":"delivered"}
    Expected Result: Channel receives response, RPC returns success
    Failure Indicators: Channel empty, RPC returns error, wrong response fields
    Evidence: .sisyphus/evidence/task-19-rpc-deliver.txt

  Scenario: PII not logged
    Tool: Bash (go test)
    Preconditions: Mock security logger that records all log calls
    Steps:
      1. Call handleResolveBlocker with input containing "super-secret-password"
      2. Check all security logger calls for "super-secret-password"
    Expected Result: Zero log entries contain the input string
    Failure Indicators: Any log entry contains "super-secret-password"
    Evidence: .sisyphus/evidence/task-19-pii-not-logged.txt
  ```

  **Commit**: YES
  - Message: `feat(rpc): add resolve_blocker handler`
  - Files: `bridge/pkg/rpc/server.go, bridge/pkg/rpc/server_test.go`
  - Pre-commit: `go test ./pkg/rpc/... -run TestResolveBlocker`

- [x] 20. Blocker Notification Formatting + Matrix Delivery

  **What to do**:
  - Modify `bridge/pkg/secretary/notifications.go` to add `formatBlockerMessage(blockers []Blocker) string`
  - Format: header "🚧 **Agent blocked — action required**", then for each blocker: numbered message, suggestion (if present), field name (if present)
  - Footer: "⏱ Expires in 10m0s" showing the timeout
  - Wire `formatBlockerMessage` into the blocker notification path in `executeStepWithBlockerHandling` (Task 17)
  - Use existing `notificationService.Notify()` with type `"blocker.required"`, workflowID, message, and detail set to blockers
  - Create tests for the formatter

  **Must NOT do**:
  - Do NOT include blocker response in any notification — only the blocker request
  - Do NOT hard-code the timeout string — reference the `BlockerTimeout` constant
  - Do NOT send notifications for empty blocker lists

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 17-19)
  - **Blocks**: Phase 6
  - **Blocked By**: Task 14

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/notifications.go` — Existing notification formatting and delivery pattern
  - `bridge/pkg/secretary/orchestrator_integration.go` — Where blocker notification is sent (Task 17)

  **API/Type References**:
  - `bridge/pkg/secretary/result.go` (Task 4) — `Blocker` struct with BlockerType, Message, Suggestion, Field

  **WHY Each Reference Matters**:
  - Notification formatting must be clear for ArmorChat users to understand what the agent needs
  - Existing notification pattern must be followed for consistent Matrix delivery

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/secretary/... -run TestBlockerNotif` → PASS
  - [ ] Formatted message contains blocker message, suggestion, and timeout
  - [ ] Notification sent via existing notificationService.Notify()
  - [ ] Multiple blockers are numbered correctly

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Blocker notification formatting
    Tool: Bash (go test)
    Steps:
      1. Call formatBlockerMessage with [Blocker{Message:"Requires API key", Suggestion:"Check .env file", Field:"api_key"}]
      2. Verify output contains "🚧", "Requires API key", "💡", "api_key", "Expires in"
    Expected Result: Formatted string with all blocker details and timeout
    Failure Indicators: Missing blocker message, missing suggestion, no timeout info
    Evidence: .sisyphus/evidence/task-20-blocker-format.txt

  Scenario: Multiple blockers numbered
    Tool: Bash (go test)
    Steps:
      1. Call formatBlockerMessage with 2 blockers
      2. Verify "Blocker 1" and "Blocker 2" present in output
    Expected Result: Both blockers numbered and present
    Failure Indicators: Only one blocker shown, numbering wrong
    Evidence: .sisyphus/evidence/task-20-multi-blocker.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add blocker notification formatting`
  - Files: `bridge/pkg/secretary/notifications.go, bridge/pkg/secretary/notifications_test.go`
  - Pre-commit: `go test ./pkg/secretary/... -run TestBlockerNotif`

- [x] 21. Blocker Integration Tests (E2E Flow, PII Verification)

  **What to do**:
  - Create comprehensive E2E tests in `bridge/pkg/secretary/orchestrator_integration_test.go`
  - `TestBlockerLoopResolve`: blocker → blocked → RPC response → re-spawn → complete. Verify full state machine transition sequence: running → blocked → running → completed
  - `TestBlockerTimeout`: blocker with no response → 10-minute timeout → workflow failed
  - `TestBlockerMaxRetries`: 3 consecutive blockers → workflow failed with retry exceeded message
  - `TestAppendBlockerResponse`: verify `_blocker_response` dict in second spawn config with correct fields
  - `TestResolveBlockerRPC`: verify RPC handler delivers to channel
  - `TestResolveBlockerNoPending`: error when no pending blocker
  - `TestResolveBlockerMissingFields`: error on empty required fields
  - `TestBlockerPIINotLogged`: mock security logger, verify blocker input content absent from all log entries
  - `TestBlockerNotificationFormat`: verify notification content
  - All tests use mocks for Docker, no real containers needed

  **Must NOT do**:
  - Do NOT require a real Docker daemon — mock everything
  - Do NOT use real timers for timeout tests — inject clock or use short timeouts
  - Do NOT skip PII verification — this is the most security-critical test in the plan

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4 (final task in phase)
  - **Blocks**: Phase 5 (all tasks)
  - **Blocked By**: Tasks 17, 18, 19, 20

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/orchestrator_integration_test.go` — Existing integration test patterns (mock factory, mock notification service)

  **API/Type References**:
  - `bridge/pkg/secretary/orchestrator_integration.go` (Tasks 17, 18) — `executeStepWithBlockerHandling()`, `waitForBlockerResponse()`
  - `bridge/pkg/rpc/server.go` (Task 19) — `handleResolveBlocker()`
  - `bridge/pkg/secretary/notifications.go` (Task 20) — `formatBlockerMessage()`

  **WHY Each Reference Matters**:
  - E2E test validates the entire blocker protocol works as a cohesive system, not just individual parts
  - PII verification test is the enforcement mechanism for ADR-004

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/secretary/... -run TestBlocker` → PASS (8 tests)
  - [ ] E2E: blocker → blocked → resolve → re-spawn → complete flow works
  - [ ] Timeout test: workflow fails after BlockerTimeout
  - [ ] Max retries test: workflow fails after 3 consecutive blockers
  - [ ] PII test: blocker input content verified absent from all log entries

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: End-to-end blocker resolution
    Tool: Bash (go test)
    Preconditions: Mock factory returning "running" then "completed", mock blocker response
    Steps:
      1. Run executeStepWithBlockerHandling
      2. First spawn: factory returns result with Blockers[{Message:"Need password"}]
      3. Verify workflow transitions to "blocked"
      4. Verify blocker notification sent
      5. Deliver blocker response via channel with input="secret123"
      6. Second spawn: factory returns result with no blockers
      7. Verify workflow transitions to "completed"
    Expected Result: State transitions: running → blocked → running → completed. No PII logged.
    Failure Indicators: Wrong state at any point, PII in logs, notification not sent
    Evidence: .sisyphus/evidence/task-21-e2e-blocker.txt

  Scenario: PII verification
    Tool: Bash (go test)
    Preconditions: Mock security logger recording all calls, blocker with PII response
    Steps:
      1. Execute blocker flow with response input="my-credit-card-4242"
      2. Search all logger calls for "my-credit-card-4242"
      3. Search all written files for "my-credit-card-4242"
    Expected Result: Zero matches in logs, zero matches in files
    Failure Indicators: Any match found — PII leaked
    Evidence: .sisyphus/evidence/task-21-pii-verify.txt
  ```

  **Commit**: YES
  - Message: `test(secretary): comprehensive blocker protocol integration tests`
  - Files: `bridge/pkg/secretary/orchestrator_integration_test.go, bridge/pkg/rpc/server_test.go`
  - Pre-commit: `go test ./pkg/secretary/... -run TestBlocker`

---

- [x] 22. Skill Extraction Pipeline (ExtractFromResult)

  **What to do**:
  - Create `bridge/pkg/skills/extractor.go` with extraction functions
  - Implement `ExtractFromResult(result *secretary.ExtendedStepResult, taskDesc, taskID, templateID string) []LearnedSkill`
  - Strategy 1: Agent self-reported — iterate `result.SkillCandidates`, build LearnedSkill from each
  - Strategy 2: Command sequence — extract 2+ `command_run` events from Events, build skill with PatternCommandSequence, confidence 0.6
  - Strategy 3: File operations — extract file reads/writes/deletes, build skill with PatternFileTransform if 1+ writes or 2+ reads, confidence 0.5
  - Define `PatternType` constants: `PatternCommandSequence`, `PatternFileTransform`, `PatternConfigTemplate`
  - `generateSkillName(taskDesc, suffix)` — deterministic name from task description hash + suffix
  - `deduplicateSkills(skills)` — remove duplicates by name
  - `extractCommandSequence(events)` — pull command_run events with command, exit_code
  - `extractFileOperations(events)` — categorize file_read/write/delete events by path
  - Create `bridge/pkg/skills/extractor_test.go` with 7 tests per Phase 5 test plan (section 7.4)

  **Must NOT do**:
  - Do NOT auto-execute extracted skills — they are suggestions only
  - Do NOT extract skills from failed tasks — only successful completions
  - Do NOT assign confidence above 0.7 for auto-extracted skills (self-reported can be higher)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 23, 24)
  - **Blocks**: Tasks 23, 25
  - **Blocked By**: Tasks 1, 4

  **References**:

  **Pattern References**:
  - `bridge/pkg/skills/learned_store.go` (Task 1) — `LearnedSkill` struct definition and storage pattern
  - `bridge/pkg/secretary/result.go` (Task 4) — `ExtendedStepResult`, `StepEvent`, `SkillCandidate`, `EventsSummary`

  **API/Type References**:
  - `bridge/pkg/secretary/result.go` — `StepEvent.Type` values: `"command_run"`, `"file_read"`, `"file_write"`, `"file_delete"`

  **WHY Each Reference Matters**:
  - `LearnedSkill` struct (Task 1) defines what the extractor must produce
  - `ExtendedStepResult` (Task 4) is the input — the extractor reads events and candidates from it

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/skills/... -run TestExtract` → PASS (7 tests)
  - [ ] Command sequence with 2+ commands produces a skill with PatternCommandSequence
  - [ ] Single command does NOT produce a command skill (threshold is 2)
  - [ ] File operations with 1+ write or 2+ reads produces a skill with PatternFileTransform
  - [ ] Agent self-reported candidates included directly
  - [ ] Empty events produces empty candidates
  - [ ] Duplicate skills deduplicated by name

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Command sequence extraction
    Tool: Bash (go test)
    Preconditions: ExtendedStepResult with Events containing 3 command_run events
    Steps:
      1. Call ExtractFromResult with taskDesc="deploy the database"
      2. Verify returned skills include one with PatternType=PatternCommandSequence
      3. Verify PatternData contains 3 commands
      4. Verify Confidence == 0.6
    Expected Result: One command sequence skill with 3 commands, confidence 0.6
    Failure Indicators: No skill extracted, wrong pattern type, wrong command count
    Evidence: .sisyphus/evidence/task-22-command-extract.txt

  Scenario: Self-reported skills included
    Tool: Bash (go test)
    Preconditions: ExtendedStepResult with SkillCandidates [{Name:"db-migrate", Confidence:0.8}]
    Steps:
      1. Call ExtractFromResult
      2. Verify returned skills include "db-migrate" with confidence 0.8
    Expected Result: Self-reported skill preserved with original confidence
    Failure Indicators: Self-reported skill missing, confidence changed
    Evidence: .sisyphus/evidence/task-22-self-reported.txt
  ```

  **Commit**: YES
  - Message: `feat(skills): add skill extraction pipeline`
  - Files: `bridge/pkg/skills/extractor.go, bridge/pkg/skills/extractor_test.go`
  - Pre-commit: `go test ./pkg/skills/... -run TestExtract`

- [x] 23. Skill Injection at Dispatch (injectLearnedSkills)

  **What to do**:
  - Modify `bridge/pkg/secretary/orchestrator_integration.go` to add `injectLearnedSkills()` method on `StepExecutor`
  - Signature: `injectLearnedSkills(config json.RawMessage, taskDesc string) json.RawMessage`
  - If `learnedStore` is nil: return config unchanged
  - Call `learnedStore.FindForTask(taskDesc, 3)` to get up to 3 matching skills
  - If no skills found: return config unchanged
  - For each skill: build context map with `name`, `confidence`, `pattern`, `source` fields
  - Append `"relevant_skills"` key to config JSON with the skill context list
  - Return modified config JSON
  - Wire this into the dispatch path BEFORE spawning the container

  **Must NOT do**:
  - Do NOT auto-execute skills — they go in `relevant_skills` as suggestions only
  - Do NOT modify config if no skills found — preserve original exactly
  - Do NOT inject more than 3 skills — limit is hardcoded

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 22, 24)
  - **Blocks**: Task 25
  - **Blocked By**: Tasks 1, 22

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/orchestrator_integration.go` — Existing dispatch/spawn path where injection happens
  - `bridge/pkg/skills/learned_store.go` (Task 1) — `FindForTask()` API returning ranked skills

  **API/Type References**:
  - `bridge/pkg/skills/learned_store.go` — `LearnedSkill` struct with Name, Confidence, PatternData, SuccessCount

  **WHY Each Reference Matters**:
  - `FindForTask()` already handles confidence threshold (>= 0.4) and keyword ranking — injection just needs to serialize results
  - The dispatch path is where STEP_CONFIG is built — skills must be injected before container spawn

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/secretary/... -run TestInjectLearned` → PASS (3 tests)
  - [ ] Skills present in config JSON after injection
  - [ ] Nil store returns config unchanged
  - [ ] No matching keywords returns config unchanged
  - [ ] Maximum 3 skills injected

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Skills injected into config
    Tool: Bash (go test)
    Preconditions: LearnedStore with skill "db-migrate" matching "database" keyword
    Steps:
      1. Call injectLearnedSkills with taskDesc="migrate the production database"
      2. Parse returned JSON
      3. Verify "relevant_skills" key exists with 1 entry
      4. Verify entry has name="db-migrate" and confidence field
    Expected Result: Config JSON has relevant_skills with matching skill
    Failure Indicators: No relevant_skills key, empty array, wrong skill data
    Evidence: .sisyphus/evidence/task-23-inject-skills.txt

  Scenario: No match preserves config
    Tool: Bash (go test)
    Preconditions: LearnedStore with skill matching "database", taskDesc about "web browsing"
    Steps:
      1. Call injectLearnedSkills with taskDesc="browse the web for news"
      2. Compare returned JSON to input JSON
    Expected Result: Byte-for-byte identical (no modification)
    Failure Indicators: relevant_skills key added with empty array, JSON reformatted
    Evidence: .sisyphus/evidence/task-23-no-match.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): inject learned skills at dispatch`
  - Files: `bridge/pkg/secretary/orchestrator_integration.go, bridge/pkg/secretary/orchestrator_integration_test.go`
  - Pre-commit: `go test ./pkg/secretary/... -run TestInjectLearned`

- [x] 24. Post-Completion Extraction + RecordOutcome

  **What to do**:
  - Modify `bridge/pkg/secretary/orchestrator_integration.go` to add post-completion skill extraction in the step execution flow
  - After `CompleteWorkflow()`: if result status is "success" and `learnedStore != nil`, call `skills.ExtractFromResult(result, step.Description, workflowID, template.ID)`
  - For each extracted skill: generate ID (`"ls_{workflowID}_{timestamp}"`), set CreatedAt, call `learnedStore.Save(skill)`
  - If Save fails: log infrastructure error but do NOT fail the workflow
  - Wire `RecordOutcome` into the dispatch path: before injecting skills, record the outcome of previously suggested skills
  - After container completes: for each skill in `relevant_skills` from the config, call `learnedStore.RecordOutcome(skillID, success)` where success is based on result status
  - This adjusts confidence over time: successful uses increase confidence, failures decrease it

  **Must NOT do**:
  - Do NOT extract skills from failed tasks — only successful completions
  - Do NOT fail the workflow if skill extraction or saving fails
  - Do NOT block the workflow on skill operations — they are infrastructure, not critical path

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 22, 23)
  - **Blocks**: Task 25
  - **Blocked By**: Tasks 22, 23

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/orchestrator_integration.go` — Existing completion path where extraction hooks in
  - `bridge/pkg/skills/extractor.go` (Task 22) — `ExtractFromResult()` API
  - `bridge/pkg/skills/learned_store.go` (Task 1) — `Save()` and `RecordOutcome()` APIs

  **API/Type References**:
  - `bridge/pkg/skills/learned_store.go` — `RecordOutcome(skillID string, success bool)` adjusts confidence

  **WHY Each Reference Matters**:
  - `RecordOutcome` is the feedback loop — without it, confidence never adjusts and bad skills stay at 0.5
  - Extraction only on success prevents learning from failures (which would reinforce bad patterns)

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/secretary/... -run TestPostCompletion` → PASS (2 tests)
  - [ ] Skills extracted and saved on successful task completion
  - [ ] No extraction on failed task
  - [ ] Save failure does not fail the workflow
  - [ ] RecordOutcome called for each suggested skill after completion

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Successful task extracts and saves skills
    Tool: Bash (go test)
    Preconditions: LearnedStore, step with 2+ command_run events, result status="success"
    Steps:
      1. Run post-completion extraction
      2. Call learnedStore.FindForTask with matching keywords
      3. Verify extracted skill appears
    Expected Result: Skill saved to store, findable by keyword match
    Failure Indicators: No skill in store, Save() error propagated
    Evidence: .sisyphus/evidence/task-24-extract-save.txt

  Scenario: Failed task skips extraction
    Tool: Bash (go test)
    Preconditions: LearnedStore, step with events, result status="failed"
    Steps:
      1. Run post-completion check with status="failed"
      2. Verify ExtractFromResult is NOT called
      3. Verify no new skills in store
    Expected Result: No skills extracted, store unchanged
    Failure Indicators: Skills extracted from failed task, workflow fails on extraction
    Evidence: .sisyphus/evidence/task-24-no-extract-on-fail.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): post-completion skill extraction and outcome recording`
  - Files: `bridge/pkg/secretary/orchestrator_integration.go, bridge/pkg/secretary/orchestrator_integration_test.go`
  - Pre-commit: `go test ./pkg/secretary/... -run TestPostCompletion`

- [x] 25. Learned Skills Integration Tests

  **What to do**:
  - Create comprehensive integration tests for the full learned skills lifecycle
  - `TestExtractCommandSequence`: commands extracted in order
  - `TestExtractCommandSequenceSingle`: 1 command = no candidate (threshold: 2)
  - `TestExtractFileOperations`: reads/writes/deletes categorized
  - `TestExtractFileOperationsThreshold`: 1 read + 0 writes = no candidate
  - `TestExtractSelfReported`: agent candidates included
  - `TestExtractNoEvents`: empty events = empty candidates
  - `TestDeduplicateSkills`: same name deduped
  - `TestInjectLearnedSkills`: skills in config JSON
  - `TestInjectLearnedSkillsNilStore`: nil store = unchanged
  - `TestInjectLearnedSkillsNoMatch`: no matching keywords = unchanged
  - `TestPostCompletionExtraction`: skills saved on success
  - `TestPostCompletionExtractionSkippedOnFail`: no extraction on failure
  - Full lifecycle test: extract → save → inject → execute → record outcome → verify confidence adjustment
  - Files: `bridge/pkg/skills/extractor_test.go`, `bridge/pkg/secretary/orchestrator_integration_test.go`

  **Must NOT do**:
  - Do NOT mock the extractor — test real extraction logic
  - Do NOT mock LearnedStore — use in-memory SQLite for real persistence
  - Do NOT skip the full lifecycle test — it validates the entire feedback loop

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 5 (final task in phase)
  - **Blocks**: Phase 6 (all tasks)
  - **Blocked By**: Tasks 22, 23, 24

  **References**:

  **Pattern References**:
  - `bridge/pkg/skills/learned_store_test.go` (Task 1) — In-memory SQLite test pattern
  - `bridge/pkg/skills/extractor.go` (Task 22) — `ExtractFromResult()` function under test

  **API/Type References**:
  - `bridge/pkg/skills/learned_store.go` (Task 1) — `Save()`, `FindForTask()`, `RecordOutcome()`
  - `bridge/pkg/skills/extractor.go` (Task 22) — `ExtractFromResult()`

  **WHY Each Reference Matters**:
  - The full lifecycle test validates the feedback loop: extract → persist → inject → execute → adjust confidence
  - Without integration tests, the individual components (extraction, injection, outcome recording) might not work together

  **Acceptance Criteria**:

  - [ ] `go test -v ./pkg/skills/... -run TestExtract` → PASS (7 tests)
  - [ ] `go test -v ./pkg/secretary/... -run TestInjectLearned` → PASS (3 tests)
  - [ ] `go test -v ./pkg/secretary/... -run TestPostCompletion` → PASS (2 tests)
  - [ ] Full lifecycle: successful task extracts skills, future matching tasks receive suggestions
  - [ ] RecordOutcome(success=false) decreases confidence below 0.4, skill no longer suggested

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Full skill lifecycle
    Tool: Bash (go test)
    Preconditions: Clean in-memory SQLite, ExtendedStepResult with 3 command_run events
    Steps:
      1. Extract skills from result → get 1 command_sequence skill
      2. Save skill to LearnedStore
      3. Inject skills for task "deploy database" → config has relevant_skills
      4. Simulate success: RecordOutcome(skillID, true) → confidence increases
      5. Inject again → skill still suggested
      6. Simulate 5 failures: RecordOutcome(skillID, false) x5 → confidence drops below 0.4
      7. Inject again → skill NOT suggested (below threshold)
    Expected Result: Skill lifecycle works end to end: extract → save → inject → adjust → filter
    Failure Indicators: Skill not extracted, not saved, not injected, confidence doesn't adjust
    Evidence: .sisyphus/evidence/task-25-lifecycle.txt

  Scenario: Confidence threshold filtering in lifecycle
    Tool: Bash (go test)
    Preconditions: Skill saved with confidence 0.5
    Steps:
      1. RecordOutcome(skillID, false) x3
      2. Call FindForTask — verify skill excluded (confidence < 0.4)
    Expected Result: After 3 failures on a skill starting at 0.5 confidence, it drops below 0.4 and is filtered out
    Failure Indicators: Skill still returned, confidence above 0.4
    Evidence: .sisyphus/evidence/task-25-confidence-filter.txt
  ```

  **Commit**: YES
  - Message: `test(skills): comprehensive learned skills integration tests`
  - Files: `bridge/pkg/skills/extractor_test.go, bridge/pkg/secretary/orchestrator_integration_test.go`
  - Pre-commit: `go test ./pkg/skills/... ./pkg/secretary/... -run "TestExtract|TestInjectLearned|TestPostCompletion"`

---

- [x] 26. ArmorChat WorkflowTimeline Composable

  **What to do**:
  - Create `applications/ArmorChat/.../ui/components/WorkflowTimeline.kt` — a Jetpack Compose composable
  - Scrollable vertical timeline rendering `workflow.progress` events during execution
  - Each timeline row: event icon (mapped from event type), step name, duration (formatted as seconds), detail line (exit code, line count, file path)
  - Map Go event types to icons: step→🔹, file_read→📄, file_write→✏️, file_delete→🗑️, command_run→⌨️, observation→💭, blocker→🚧, error→❌, artifact→📦, checkpoint→🏁
  - Progress bar at top showing latest percent (from progress events)
  - "Live" indicator when workflow is running, "Complete" badge when done
  - Empty state: centered text "Waiting for agent activity..."
  - Design for Matrix /sync delivered events, not WebSocket
  - Support both light and dark themes using Material 3 theming

  **Must NOT do**:
  - Do NOT add WebSocket support — use Matrix /sync for event delivery
  - Do NOT block the UI thread on event parsing
  - Do NOT hard-code colors — use Material 3 theme colors

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 6 (with Tasks 27, 28, 29, 30)
  - **Blocks**: Task F3
  - **Blocked By**: Task 14

  **References**:

  **Pattern References**:
  - `androidApp/src/main/kotlin/com/armorclaw/app/screens/chat/components/MessageBubble.kt` — Existing message rendering component (pattern for timeline item layout)
  - `shared/src/commonMain/kotlin/data/store/ControlPlaneStore.kt` — Subscribes to Matrix room events including the new `workflow.progress` events from Task 16

  **API/Type References**:
  - `androidApp/src/main/kotlin/com/armorclaw/app/data/bridge/BridgeApi.kt` — The actual RPC client (NOT BridgeRpcClient.kt — that file does not exist)
  - `shared/src/commonMain/kotlin/domain/model/UnifiedMessage.kt` — Message model that may need a variant for workflow progress events
  - `bridge/pkg/secretary/orchestrator_events.go` (Task 13) — `ProgressDetail` struct: EventSeq, EventType, StepName, ElapsedMs, Detail
  - `bridge/pkg/secretary/notifications.go` (Task 14) — `stepIcon()` function showing the icon mapping

  **WHY Each Reference Matters**:
  - `BridgeApi.kt` is the correct filename for the RPC client — all Phase 6 tasks must reference this
  - `ControlPlaneStore` is where workflow.progress events arrive from Matrix /sync — the timeline component consumes from here
  - The icon mapping must match between Bridge (Go) and ArmorChat (Kotlin) for consistency

  **Acceptance Criteria**:

  - [ ] Compose preview renders timeline with correct icons and layout
  - [ ] Empty state shows "Waiting for agent activity..." message
  - [ ] Progress bar updates with latest percent from progress events
  - [ ] "Live" indicator shown during execution, "Complete" badge when done
  - [ ] Light and dark themes supported

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Timeline renders events correctly
    Tool: Bash (gradlew test + Compose preview)
    Steps:
      1. Create WorkflowTimeline with 5 sample events: step, file_read(100 lines), command_run(exit 0), artifact, checkpoint
      2. Render in Compose preview
      3. Verify each row shows correct icon, step name, and detail
    Expected Result: 5 rows with correct icons (🔹📄⌨️📦🏁), names, and sub-details (100 lines, ✓ exit 0)
    Failure Indicators: Wrong icons, missing details, layout crash
    Evidence: .sisyphus/evidence/task-26-timeline-render.txt

  Scenario: Empty state
    Tool: Bash (gradlew test)
    Steps:
      1. Create WorkflowTimeline with empty event list
      2. Verify "Waiting for agent activity..." text displayed
    Expected Result: Empty state message visible, no crash on empty list
    Failure Indicators: Blank screen, crash, "null" text
    Evidence: .sisyphus/evidence/task-26-empty-state.txt
  ```

  **Commit**: YES
  - Message: `feat(armorchat): add WorkflowTimeline composable`
  - Files: `applications/ArmorChat/.../ui/components/WorkflowTimeline.kt`
  - Pre-commit: `./gradlew test`

- [x] 27. ArmorChat BlockerResponseDialog

  **What to do**:
  - Create `applications/ArmorChat/.../ui/components/BlockerResponseDialog.kt` — a Jetpack Compose dialog
  - Triggered by `workflow.blocked` event from Matrix /sync
  - Shows blocker description, suggestion (if present), field name (if present)
  - Free-text input field for user response
  - Optional note field
  - "Send" button calls `BridgeApi.resolveBlocker()` RPC
  - Auto-dismisses when `workflow.running` event received (blocker resolved, agent resumed)
  - Loading spinner while RPC call in flight
  - Error state with retry if RPC fails
  - Timeout warning: show remaining time from the 10-minute blocker timeout

  **Must NOT do**:
  - Do NOT cache blocker response text after sending — clear the input field
  - Do NOT show the response text in any log or persistent UI after send
  - Do NOT block the dialog on network — show loading state during RPC

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 6 (with Tasks 26, 28, 29, 30)
  - **Blocks**: Task F3
  - **Blocked By**: Task 19

  **References**:

  **Pattern References**:
  - `applications/ArmorChat/` — Existing dialog component patterns in the project

  **API/Type References**:
  - `androidApp/src/main/kotlin/com/armorclaw/app/data/bridge/BridgeApi.kt` — Where Task 29 adds the `resolveBlocker()` method
  - `shared/src/commonMain/kotlin/data/store/ControlPlaneStore.kt` — Where `workflow.blocked` events arrive from Task 16
  - `bridge/pkg/rpc/server.go` (Task 19) — `resolve_blocker` RPC: params are workflow_id, step_id, input, note
  - `bridge/pkg/secretary/result.go` (Task 4) — `Blocker` struct: BlockerType, Message, Suggestion, Field

  **WHY Each Reference Matters**:
  - `resolve_blocker` RPC defines the exact params the dialog must send
  - `Blocker` struct defines what information the dialog displays
  - `BridgeApi.kt` is the correct filename (NOT BridgeRpcClient.kt)

  **Acceptance Criteria**:

  - [ ] Dialog shows blocker message and suggestion
  - [ ] `resolveBlocker` RPC called with correct workflow_id, step_id, input
  - [ ] Dialog auto-dismisses on `workflow.running` event
  - [ ] Loading spinner shown during RPC call
  - [ ] Error state with retry shown on RPC failure

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Blocker dialog sends RPC
    Tool: Bash (gradlew test)
    Preconditions: Mock BridgeApi that records resolveBlocker calls
    Steps:
      1. Show BlockerResponseDialog with blocker message "Need API key"
      2. Type "sk-xxx-yyy" in input field
      3. Click "Send"
      4. Verify mock.resolveBlocker called with input="sk-xxx-yyy"
    Expected Result: RPC called with correct params, dialog shows loading then dismisses
    Failure Indicators: RPC not called, wrong params, dialog doesn't dismiss
    Evidence: .sisyphus/evidence/task-27-blocker-dialog.txt

  Scenario: Auto-dismiss on workflow running
    Tool: Bash (gradlew test)
    Steps:
      1. Show BlockerResponseDialog
      2. Emit workflow.running event
      3. Verify dialog is no longer visible
    Expected Result: Dialog dismissed without user action
    Failure Indicators: Dialog stays visible after workflow.running event
    Evidence: .sisyphus/evidence/task-27-auto-dismiss.txt
  ```

  **Commit**: YES
  - Message: `feat(armorchat): add BlockerResponseDialog`
  - Files: `applications/ArmorChat/.../ui/components/BlockerResponseDialog.kt`
  - Pre-commit: `./gradlew test`

- [x] 28. ArmorChat Agent Status Banner (GovernanceBanner)

  **What to do**:
  - Modify `applications/ArmorChat/.../ui/components/GovernanceBanner.kt` (or equivalent status banner)
  - Add new visual state for `WorkflowStatus.BLOCKED`: warning yellow background, 🚧 icon, "Action Required" text
  - Tapping the blocked banner navigates to the chat room with the blocker dialog
  - Add visual state for `WorkflowStatus.RUNNING`: blue/teal pulsing indicator, step count
  - Existing states (idle, completed, failed) remain unchanged
  - Use Material 3 color scheme: blocked → `colorScheme.errorContainer` or tertiary yellow, running → `colorScheme.primaryContainer`

  **Must NOT do**:
  - Do NOT change existing banner states for idle/completed/failed
  - Do NOT add auto-refresh — rely on Matrix /sync push

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 6 (with Tasks 26, 27, 29, 30)
  - **Blocks**: Task F3
  - **Blocked By**: Task 6

  **References**:

  **Pattern References**:
  - `applications/ArmorChat/.../ui/components/GovernanceBanner.kt` — Existing status banner with visual states
  - `bridge/pkg/secretary/types.go` (Task 6) — `StatusBlocked = "blocked"` constant

  **API/Type References**:
  - `bridge/pkg/secretary/types.go` — `WorkflowStatus` enum: pending, running, blocked, completed, failed, cancelled

  **WHY Each Reference Matters**:
  - `StatusBlocked` (Task 6) is the new state the banner must render
  - Existing banner states must not break when the new blocked state is added

  **Acceptance Criteria**:

  - [ ] Blocked state: yellow background, 🚧 icon, "Action Required" text
  - [ ] Tapping blocked banner navigates to chat with blocker dialog
  - [ ] Running state: blue/teal indicator, step count
  - [ ] Existing states (idle, completed, failed) unchanged

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Blocked banner visual state
    Tool: Bash (gradlew test)
    Steps:
      1. Render GovernanceBanner with status=WorkflowStatus.BLOCKED
      2. Verify yellow warning background
      3. Verify 🚧 icon visible
      4. Verify "Action Required" text displayed
    Expected Result: Banner shows blocked state with correct visual treatment
    Failure Indicators: Default state shown, wrong color, missing icon
    Evidence: .sisyphus/evidence/task-28-blocked-banner.txt

  Scenario: Tap navigates to blocker dialog
    Tool: Bash (gradlew test)
    Steps:
      1. Render GovernanceBanner with status=BLOCKED
      2. Perform click on banner
      3. Verify navigation event to chat room with blocker dialog route
    Expected Result: Navigation triggered to blocker dialog screen
    Failure Indicators: No navigation, wrong destination
    Evidence: .sisyphus/evidence/task-28-tap-navigate.txt
  ```

  **Commit**: YES
  - Message: `feat(armorchat): add blocked status to GovernanceBanner`
  - Files: `applications/ArmorChat/.../ui/components/GovernanceBanner.kt`
  - Pre-commit: `./gradlew test`

- [x] 29. BridgeApi resolve_blocker RPC Method

  **What to do**:
  - Add `resolveBlocker(workflowId: String, stepId: String, input: String, note: String = "")` method to `androidApp/src/main/kotlin/com/armorclaw/app/data/bridge/BridgeApi.kt` (NOT BridgeRpcClient.kt — that file does not exist)
  - Build JSON-RPC request: method `"resolve_blocker"`, params with workflow_id, step_id, input, note
  - Use existing OkHttp JSON-RPC client pattern from `BridgeApi.kt`
  - Handle response: success returns `{"status": "delivered"}`, error returns RPC error
  - Add error handling for: no pending blocker, missing fields, network timeout
  - Add unit test verifying correct JSON-RPC payload construction

  **Must NOT do**:
  - Do NOT store the input parameter in any local cache or log
  - Do NOT modify existing RPC methods in BridgeApi
  - Do NOT use a different HTTP client — reuse existing OkHttp pattern

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 6 (with Tasks 26, 27, 28, 30)
  - **Blocks**: Task 27
  - **Blocked By**: Task 19

  **References**:

  **Pattern References**:
  - `androidApp/src/main/kotlin/com/armorclaw/app/data/bridge/BridgeApi.kt` — Existing RPC method implementations (follow the same OkHttp JSON-RPC pattern)

  **API/Type References**:
  - `bridge/pkg/rpc/server.go` (Task 19) — `resolve_blocker` RPC spec: required params are workflow_id, step_id, input

  **WHY Each Reference Matters**:
  - BridgeApi.kt is the single RPC client file — all Bridge calls go through here

  **Acceptance Criteria**:

  - [ ] Unit test verifying correct JSON-RPC payload: method, all 4 params
  - [ ] Returns `RpcResult.success` with `{"status": "delivered"}` on success
  - [ ] Returns `RpcResult.error` on "no pending blocker" response
  - [ ] Handles network timeout gracefully

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Correct RPC payload
    Tool: Bash (gradlew test)
    Steps:
      1. Call resolveBlocker("wf-1", "step-1", "my-password", "note text")
      2. Capture the HTTP request body
      3. Verify JSON-RPC: {"jsonrpc":"2.0", "method":"resolve_blocker", "params":{"workflow_id":"wf-1","step_id":"step-1","input":"my-password","note":"note text"}}
    Expected Result: Correct JSON-RPC payload with all fields
    Failure Indicators: Missing params, wrong method name, wrong JSON structure
    Evidence: .sisyphus/evidence/task-29-rpc-payload.txt

  Scenario: Error response handled
    Tool: Bash (gradlew test)
    Preconditions: Mock server returning {"error":{"code":-1,"message":"no pending blocker"}}
    Steps:
      1. Call resolveBlocker("wf-1", "step-1", "input")
      2. Verify RpcResult.error returned with message
    Expected Result: Error result, no crash
    Failure Indicators: Exception thrown, success returned on error
    Evidence: .sisyphus/evidence/task-29-error-handling.txt
  ```

  **Commit**: YES
  - Message: `feat(armorchat): add resolve_blocker to BridgeApi`
  - Files: `applications/ArmorChat/.../network/BridgeApi.kt, applications/ArmorChat/.../network/BridgeApiTest.kt`
  - Pre-commit: `./gradlew test`

- [x] 30. Matrix Commands (!agent skills, !agent forget-skill)

  **What to do**:
  - Modify `bridge/internal/adapter/commands_integration.go` to add two new Matrix commands
  - `!agent skills <agent_id>` — calls `learnedStore.ListForAgent(20)`, formats response as numbered list with name, confidence, success count, pattern type. Returns "No learned skills yet." if empty
  - `!agent forget-skill <agent_id> <skill_id>` — calls `learnedStore.Delete(skillID)`, returns confirmation "Forgot skill: {skill_id}". Returns error if skill not found
  - Register both commands in the existing command handler map
  - Access `LearnedStore` via the existing dependency injection (passed through orchestrator/store)
  - Format skill listing: "📚 Learned Skills for {agent_id}:\n1. {name} (confidence: 0.72, 5 successful)\n..."

  **Must NOT do**:
  - Do NOT auto-delete skills below threshold — only explicit `forget-skill` deletes
  - Do NOT expose skill pattern data in the listing — just name, confidence, count
  - Do NOT allow deleting skills from other agents (validate agent ownership)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 6 (with Tasks 26, 27, 28, 29)
  - **Blocks**: Task F1
  - **Blocked By**: Tasks 1, 22

  **References**:

  **Pattern References**:
  - `bridge/internal/adapter/commands_integration.go` — Existing `!agent` command handlers showing registration and response patterns

  **API/Type References**:
  - `bridge/pkg/skills/learned_store.go` (Task 1) — `ListForAgent(limit)`, `Delete(skillID)` APIs

  **WHY Each Reference Matters**:
  - `commands_integration.go` is where all `!agent` commands live — new commands follow the same pattern
  - `ListForAgent` and `Delete` from Task 1 provide the data access needed

  **Acceptance Criteria**:

  - [ ] `!agent skills <agent_id>` returns formatted skill list
  - [ ] `!agent skills <agent_id>` returns "No learned skills yet." when empty
  - [ ] `!agent forget-skill <agent_id> <skill_id>` deletes skill and confirms
  - [ ] `!agent forget-skill` returns error for non-existent skill_id
  - [ ] Commands registered in existing handler map

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: List learned skills
    Tool: Bash (go test + integration test)
    Preconditions: LearnedStore with 3 skills for agent "researcher"
    Steps:
      1. Process Matrix message "!agent skills researcher"
      2. Verify response contains 3 numbered entries with names, confidence values, success counts
    Expected Result: Formatted list: "📚 Learned Skills for researcher:\n1. db-migrate (confidence: 0.72, 5 successful)\n..."
    Failure Indicators: Empty response, wrong format, missing skills
    Evidence: .sisyphus/evidence/task-30-list-skills.txt

  Scenario: Forget a skill
    Tool: Bash (go test + integration test)
    Preconditions: LearnedStore with skill "db-migrate" for agent "researcher"
    Steps:
      1. Process Matrix message "!agent forget-skill researcher db-migrate"
      2. Verify response contains "Forgot skill: db-migrate"
      3. Call learnedStore.FindForTask — verify skill no longer returned
    Expected Result: Skill deleted, confirmation message sent
    Failure Indicators: Skill still in store, error response, wrong message
    Evidence: .sisyphus/evidence/task-30-forget-skill.txt
  ```

  **Commit**: YES
  - Message: `feat(commands): add !agent skills and forget-skill`
  - Files: `bridge/internal/adapter/commands_integration.go, bridge/internal/adapter/commands_integration_test.go`
  - Pre-commit: `go test ./internal/adapter/... -run TestAgentSkills`

---

## Final Verification Wave

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go test ./pkg/secretary/... ./pkg/skills/...` + `python -m pytest container/tests/` + `./gradlew test`. Review all changed files for: `as any`/`@ts-ignore` (Go: `interface{}` abuse), empty catches, fmt.Println in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-phase integration: container emits events → Bridge tails → Matrix receives → ArmorChat renders. Test edge cases: 10MB cap trigger, blocker timeout, concurrent blockers. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

| Phase | Commit Message | Files |
|-------|---------------|-------|
| 1 | `feat(skills): add learned_skills table and LearnedStore` | `bridge/pkg/skills/learned_store.go, _test.go` |
| 1 | `feat(secretary): add EventReader with 10MB cap` | `bridge/pkg/secretary/event_reader.go, _test.go` |
| 1 | `feat(secretary): add state dir cleanup utility` | `bridge/pkg/secretary/cleanup.go, _test.go` |
| 1 | `feat(secretary): add ExtendedStepResult types` | `bridge/pkg/secretary/result.go, _test.go` |
| 1 | `feat(studio): add factory.Kill() for SIGKILL` | `bridge/pkg/studio/factory.go, _test.go` |
| 1 | `feat(secretary): add StatusBlocked workflow state` | `bridge/pkg/secretary/types.go, orchestrator.go, _test.go` |
| 2 | `feat(container): add EventEmitter with PIPE_BUF` | `container/openclaw/events.py, tests/test_events.py` |
| 2 | `feat(container): add enriched result writer` | `container/openclaw/result_writer.py, tests/test_result_writer.py` |
| 2 | `feat(container): integrate EventEmitter in StepRunner` | `container/openclaw/step_runner.py, step_config.py, tests/` |
| 3 | `feat(secretary): add event streaming in waitForCompletion` | `bridge/pkg/secretary/orchestrator_integration.go, _test.go` |
| 3 | `feat(secretary): add progress event emission to Matrix` | `bridge/pkg/secretary/orchestrator_events.go, _test.go` |
| 3 | `feat(secretary): add timeline formatter` | `bridge/pkg/secretary/notifications.go, _test.go` |
| 3 | `feat(secretary): enforce purge ordering and 10MB kill` | `bridge/pkg/secretary/orchestrator_integration.go, _test.go` |
| 4 | `feat(secretary): add blocker protocol with PII safety` | `bridge/pkg/secretary/orchestrator_integration.go, _test.go` |
| 4 | `feat(rpc): add resolve_blocker handler` | `bridge/pkg/studio/rpc.go, bridge/pkg/rpc/server.go, _test.go` |
| 5 | `feat(skills): add skill extraction pipeline` | `bridge/pkg/skills/extractor.go, _test.go` |
| 5 | `feat(secretary): inject learned skills at dispatch` | `bridge/pkg/secretary/orchestrator_integration.go, _test.go` |
| 6 | `feat(armorchat): add WorkflowTimeline and BlockerDialog` | `applications/ArmorChat/.../ui/components/WorkflowTimeline.kt, BlockerResponseDialog.kt` |
| 6 | `feat(armorchat): add resolve_blocker to BridgeApi` | `applications/ArmorChat/.../network/BridgeApi.kt` |
| 6 | `feat(commands): add !agent skills and forget-skill` | `bridge/internal/adapter/commands_integration.go` |

---

## Success Criteria

### Verification Commands
```bash
# Phase 1: Bridge Foundation
go test -v ./pkg/skills/... ./pkg/secretary/...  # Expected: all PASS
go test -v -run TestEventReaderExceedsCap ./pkg/secretary/...  # Expected: PASS (10MB cap)
go test -v -run TestCleanupStateDir ./pkg/secretary/...  # Expected: PASS (dir removed)

# Phase 2: Container Emission
python -m pytest container/tests/test_events.py -v  # Expected: all PASS
python -m pytest container/tests/test_step_runner.py -v  # Expected: all PASS

# Phase 3: Progress Streaming
go test -v -run TestPurgeOrder ./pkg/secretary/...  # Expected: PASS (parse→purge→notify)
go test -v -run TestTailKillsOnExceededCap ./pkg/secretary/...  # Expected: PASS (Kill called)

# Phase 4: Blocker Protocol
go test -v -run TestBlockerLoopResolve ./pkg/secretary/...  # Expected: PASS (E2E)
go test -v -run TestBlockerPIINotLogged ./pkg/studio/...  # Expected: PASS

# Phase 5: Learned Skills
go test -v ./pkg/skills/...  # Expected: all PASS

# Phase 6: Client
./gradlew test  # Expected: all PASS
```

### Final Checklist
- [x] All "Must Have" present (PIPE_BUF, 10MB cap, purge ordering, PII safety, backward compat, confidence threshold)
- [x] All "Must NOT Have" absent (no network in containers, no auto-execute skills, no PII logging on blocker responses, handler signature stays `(cfg) -> str`)
- [ ] All tests pass (`go test`, `pytest`, `gradlew test`)
- [ ] `_events.jsonl` verified absent from VPS disk after task completion
- [ ] Blocker response content verified absent from Bridge logs
