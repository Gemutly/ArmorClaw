# ArmorClaw v0.5.0 → v0.6.0: Revised Technical Timeline

## TL;DR

> **Quick Summary**: Implement a 4-phase upgrade covering operational transparency (state inference + blocker fix), governance hardening (Rust Vault deployment + PPTX extractor + v6 audit mode), advanced automation (bridge-side compaction + parallel execution + step failover), and mobile client polish (timeline accuracy + PII masking + email approval).
>
> **Deliverables**:
> - Bridge-side agent state inference from CDP/workflow signals
> - Fixed blocker metadata pipeline (7 bugs, container→Bridge→Matrix)
> - Rust Vault binary entrypoint + Docker deployment
> - PPTX extractor in Rust sidecar + format routing update
> - v6 microkernel audit mode with structured logging
> - Bridge-side session transcript compaction
> - StepParallel execution engine with per-step failover
> - WorkflowTimeline duration accuracy fix
> - Dynamic PII masking + event parsing in BlockerResponseDialog
> - Email approval card (Android) + bridge RPC handlers
> - VERSION file update to 0.6.0
>
> **Estimated Effort**: XL
> **Parallel Execution**: YES — 5 waves
> **Critical Path**: All implementation tasks → Wave 4 documentation → Final Review

---

## Context

### Original Request
User presented a detailed revised technical timeline for ArmorClaw v0.5.0 → v0.6.0 with 4 phases, 14 tasks, and 4 ❓ clarification items. All ❓ items were resolved through code analysis.

### Interview Summary
**Key Discussions**:
- State inference: No CDP→state mapping exists anywhere; this is a full design task
- Rust Vault: Library crate only, needs binary entrypoint + deployment (not "74 errors")
- Compaction: TypeScript has full system (~154 files), Go Bridge has zero; bridge-side pre-compaction is the approach
- Email approval: Bridge-side implemented, Android-side is zero; referenced doc does not exist
- Blocker metadata: 9 initial findings (not 2); 2 determined to be non-bugs (false positives), 7 genuine bugs including dead-code event routing and container-side message loss
- PPTX: Routing already works through Python sidecar; Phase 2.2 needs re-scoping

**Research Findings**:
- Three state systems exist (not two): AgentStatus (11), BrowserState (6), Bridge BrowserStatus (5)
- StepParallel types defined but zero implementation
- ToolSidecar execution returns mock results
- Android BlockerResponseDialog.kt is dead code — no code constructs BlockerInfo from events
- Email approval missing bridge RPC handlers (not just Android)
- VERSION file is 0.4.1 (not 0.5.0)

### Metis Review
**Identified Gaps** (addressed):
- 7 blocker bugs (not 2) — expanded Phase 1.2 scope (2 of original 9 findings were false positives)
- Three state systems (not two) — Phase 1.1 targets AgentStatus only, BrowserState unification deferred
- PPTX routing already works — Phase 2.2 re-scoped to Rust extractor only (move off Python)
- Android blocker dialog is dead code — added Android event parsing to Phase 1.2
- Email approval needs bridge RPC — added to Phase 4.3
- Compaction is session transcript, not WAL — clarified in Phase 3.1

---

## Work Objectives

### Core Objective
Implement 4 phases of architectural improvements to ArmorClaw, delivering operational transparency, governance hardening, advanced automation capabilities, and mobile client polish for the v0.6.0 release.

### Concrete Deliverables
- Bridge-side state inference engine producing AgentStatus transitions from CDP/workflow signals
- Fully functional blocker pipeline from container → Bridge → Android → user response → workflow resume
- Deployable Rust Vault service with Docker containerization
- PPTX text extraction in Rust sidecar (move off Python dependency)
- v6 microkernel audit mode for safe graduated activation with structured logging
- Bridge-side LLM context compaction before container dispatch
- Parallel step execution engine with multi-agent failover
- Android email approval card with bridge-side RPC handlers
- Real-time agent status broadcasting via Matrix events
- Accurate workflow timeline duration rendering in Android

### Definition of Done
- [ ] `go test ./...` passes with zero failures
- [ ] `cargo build --release` in rust-vault/ passes with zero errors
- [ ] `docker compose up rust-vault` reports healthy
- [ ] All QA scenarios pass with evidence in `.sisyphus/evidence/`
- [ ] VERSION file updated to 0.6.0
- [ ] CHANGELOG.md updated with v0.6.0 entry

### Must Have
- All 7 blocker metadata bugs fixed end-to-end
- Rust Vault binary compiles and runs
- PPTX extraction works through Rust sidecar
- v6 audit mode logs without intercepting
- StepParallel executes at least 2 branches concurrently
- Email approval round-trip works (Android approve → Bridge → email sends)
- Step execution failover works for both sequential and parallel steps

### Must NOT Have (Guardrails)
- No unification of the three state systems (AgentStatus only; BrowserState/Bridge BrowserStatus deferred)
- No rewrite of TypeScript compaction in Go (bridge-side pre-dispatch pruning only)
- No per-agent/per-tenant feature flag infrastructure for v6 (use global VaultConfig)
- No distributed parallel execution across containers (goroutine-based within Bridge only)
- No email composition UI (approval card only)
- No modification to Jetski PII scrubbing patterns
- No change to 3-layer document routing architecture (`RouteExtractText`)
- No removal or bypass of SQLCipher
- No change to Matrix as control plane
- No weakening of approval flow for payments or critical PII
- No auto-execution of learned skills
- No ToolSidecar communication protocol implementation (separate future milestone)
- No v6 enforcement activation (stays in audit mode; ToolSidecar protocol is a hard prerequisite)
- No `WorkflowStep` struct schema changes (parallel uses dependency edges, not new fields)

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.
> Acceptance criteria requiring "user manually tests/confirms" are FORBIDDEN.

### Test Decision
- **Infrastructure exists**: YES (Go: `go test`, Rust: `cargo test`, Python: `pytest`, Android: instrumented tests)
- **Automated tests**: Tests-after (implementation first, verification tests per task)
- **Framework**: Go standard `testing`, Rust built-in, Python `pytest`, Android Compose test

### QA Policy
Every task MUST include agent-executed QA scenarios (see TODO template below).
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go packages**: `go test -v -run TestName ./pkg/...`
- **Rust**: `cargo test --manifest-path rust-vault/Cargo.toml`
- **Python**: `cd container && python -m pytest`
- **Docker**: `docker compose up -d && docker compose ps && curl health endpoint`
- **Android**: Build verification via `./gradlew assembleDebug`

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1a (Start Immediately — no file overlap):
├── Task 1.2: Fix blocker metadata pipeline (7 Bridge-side bugs) [deep]
├── Task 2.1: Rust Vault binary + deployment [unspecified-high]
├── Task 2.2: PPTX extractor in Rust sidecar [deep]
└── Task 4.1: Timeline duration accuracy fix [quick]

Wave 1b (After 1a — Bridge-side, coordinated):
├── Task 3.1: Bridge-side session compaction [deep]
└── Task 3.2: StepParallel execution engine [ultrabrain]

Wave 2 (After Wave 1b — depends on 2.1, 2.2):
├── Task 1.1: Bridge-side state inference engine (depends: research) [ultrabrain]
├── Task 2.3: Route XLSX/PPTX to Rust sidecar (depends: 2.2) [quick]
├── Task 2.4: v6 microkernel audit mode (depends: 2.1) [deep]
├── Task 3.3: Step failover per-step wrapper (depends: none — decoupled from 3.2) [deep]
└── Task 4.3: Email approval card + bridge RPC (no v6 dependency) [deep]

Wave 3 (After Wave 2 — depends on 1.1, 1.2):
├── Task 1.3: Implement BroadcastStatus() (depends: 1.1) [unspecified-high]
└── Task 4.2: Dynamic PII masking + Android event parsing (depends: 1.2) [visual-engineering]

Wave 4 (After Wave 3 — documentation):
└── Task 4.4: VERSION + CHANGELOG + Documentation update [writing]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: End-to-end integration QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)

Critical Path: All implementation tasks → Wave 4 documentation → F1-F4
Parallel Speedup: ~55% faster than sequential
Max Concurrent: 5 (Wave 2)
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| 2.1 | None (Wave 1a) | 2.4 |
| 2.2 | None (Wave 1a) | 2.3 |
| 4.1 | None (Wave 1a) | F1-F4 |
| 1.2 | None (Wave 1a) | 4.2 |
| 3.1 | None (Wave 1b) | F1-F4 |
| 3.2 | None (Wave 1b) | F1-F4 |
| 1.1 | None (Wave 2) | 1.3 |
| 2.3 | 2.2 (Wave 2) | F1-F4 |
| 2.4 | 2.1 (Wave 2) | F1-F4 |
| 3.3 | None (Wave 2 — decoupled from 3.2) | F1-F4 |
| 4.3 | None (Wave 2 — no v6 dependency) | F1-F4 |
| 1.3 | 1.1 (Wave 3) | F1-F4 |
| 4.2 | 1.2 (Wave 3) | F1-F4 |
| 4.4 | All above (Wave 4) | F1-F4 |

### Agent Dispatch Summary

- **Wave 1a**: 4 tasks — T1.2→`deep`, T2.1→`unspecified-high`, T2.2→`deep`, T4.1→`quick`
- **Wave 1b**: 2 tasks — T3.1→`deep`, T3.2→`ultrabrain`
- **Wave 2**: 5 tasks — T1.1→`ultrabrain`, T2.3→`quick`, T2.4→`deep`, T3.3→`deep`, T4.3→`deep`
- **Wave 3**: 2 tasks — T1.3→`unspecified-high`, T4.2→`visual-engineering`
- **Wave 4**: 1 task — T4.4→`writing`
- **FINAL**: 4 tasks — F1→`oracle`, F2→`unspecified-high`, F3→`unspecified-high`, F4→`deep`

---

## TODOs

- [x] 1.2. Fix Blocker Metadata Pipeline (7 Bugs, Bridge-Side)

  **What to do**:
  - **Bug 1 (container)**: Fix `_extract_blockers_from_events()` in `container/openclaw/step_runner.py:66` — include `event.get("name")` as `"message"` in extracted blocker dict (root cause of empty messages)
  - **Bug 2**: Add `suggestion` and `field` to `EmitBlockerWarning()` metadata in `bridge/pkg/secretary/orchestrator_events.go:358-372`
  - **Bug 3**: Add `workflow_id` and `step_id` to `EmitBlockerWarning()` metadata
  - **Bug 4 (CRITICAL)**: Add `case "blocker":` to event routing switch in `orchestrator_integration.go:~414` — currently drops blocker events, making `EmitBlockerWarning` dead code
  - **Bug 5**: Add blocker forwarding to `MatrixEventForwarder` in `orchestrator_events.go`
  - **Bug 6**: Fix `BlockWorkflow` → `EmitBlocked` to preserve blocker metadata instead of discarding
  - **Bug 7**: Fix test data in `orchestrator_events_test.go` to match real container output (event name as message, not detail["message"])
  - Update tests: `go test ./pkg/secretary/... -run Blocker` must verify full metadata propagation
  - **Scope stops at the Bridge boundary** — blocker event reaches Matrix room with all 5 fields. Android event parsing is Task 4.2.

  **Must NOT do**:
  - Refactor the blocker handling architecture (two-path system)
  - Add new blocker types
  - Add Android event parsing (that's Task 4.2)

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1a (with Tasks 2.1, 2.2, 4.1)
  - **Blocks**: Task 4.2 (PII masking needs complete blocker data)
  - **Blocked By**: None

  **Pattern References**:
  - `bridge/pkg/secretary/orchestrator_events.go:358-372` — `EmitBlockerWarning()` current implementation (missing field/suggestion)
  - `container/openclaw/step_runner.py:66` — `_extract_blockers_from_events()` loses message from event name
  - `bridge/pkg/secretary/orchestrator_integration.go:~414` — event routing switch (missing blocker case)

  **API/Type References**:
  - `container/openclaw/events.py:139-153` — Container-side blocker event schema (has blocker_type, suggestion, field)
  - `bridge/pkg/secretary/orchestrator_events.go:MatrixEventForwarder` — Needs blocker forwarding

  **Test References**:
  - `bridge/pkg/secretary/orchestrator_events_test.go:72-99` — Test only asserts blocker_type and message

  **Acceptance Criteria**:

  - [ ] `go test ./pkg/secretary/... -run Blocker` passes
  - [ ] `cd container && python -m pytest test_step_runner.py -v` passes
  - [ ] Blocker event from container reaches Matrix room with all 5 fields: blocker_type, message, suggestion, field, workflow_id

  ```
  Scenario: Blocker event propagates full metadata to Matrix
    Tool: Bash (curl)
    Preconditions: Bridge running, test Matrix room configured
    Steps:
      1. Run `go test ./pkg/secretary/... -run TestEmitBlockerWarning -v`
      2. Verify output contains metadata keys: blocker_type, message, suggestion, field, workflow_id
      3. Verify message value matches event name (not nil)
    Expected Result: All 5 metadata fields present and non-nil
    Failure Indicators: Any metadata field missing or nil
    Evidence: .sisyphus/evidence/task-1.2-blocker-metadata-propagation.txt

  Scenario: Container blocker message preserved end-to-end
    Tool: Bash
    Preconditions: Container test environment
    Steps:
      1. Run `cd container && python -m pytest test_step_runner.py::test_extract_blockers_preserves_message -v`
      2. Verify blocker dict has "message" key populated from event name
    Expected Result: Message field populated, not empty
    Failure Indicators: Message is empty string or absent
    Evidence: .sisyphus/evidence/task-1.2-container-blocker-message.txt
  ```

  **Commit**: YES
  - Message: `fix(secretary): fix blocker metadata pipeline — 7 bugs container→Bridge→Matrix`
  - Files: `container/openclaw/step_runner.py`, `bridge/pkg/secretary/orchestrator_events.go`, `bridge/pkg/secretary/orchestrator_integration.go`
  - Pre-commit: `go test ./pkg/secretary/... -run Blocker`

- [x] 2.1. Rust Vault Binary Entrypoint + Docker Deployment

  **What to do**:
  - **Prerequisite**: Run `cargo build 2>&1` inside `rust-vault/` to categorize actual errors (protoc-related vs. logic errors vs. type mismatches). If non-protoc errors exist, add a sub-task to fix them and adjust effort estimate. Do not assume the problem is trivial.
  - Confirm `bridge/pkg/vault/client.go` exists and is wired (it provides `IssueBlindFillToken`, `ConsumeTokenForSidecar`, `ZeroizeToolSecrets`). If missing, add "create Go gRPC client" to scope.
  - Create `rust-vault/src/main.rs` (~10-15 lines) calling `grpc::server::run_server()` with config from environment
  - Create `rust-vault/Dockerfile` — multi-stage build (Rust builder → minimal runtime). Follow `sidecar-python/Dockerfile` hardening pattern: network none, cap-drop ALL, read-only, non-root user
  - Add `HEALTHCHECK` instruction to Dockerfile using `grpc_health_probe` or socket existence check
  - Add `armorclaw-vault` service to `docker-compose.yml` (the `armorclaw-vault` network already exists at subnet 172.29.0.0/24). Follow `deploy/docker-compose.sidecar-py.yml` pattern
  - Add shared volume mount for `/run/armorclaw/` between `armorclaw-vault` and bridge containers (required for Unix socket communication)
  - Docker health check: use `grpc_health_probe` against the gRPC service, or check that the Unix socket exists. **No code change needed in the vault** — no HTTP health endpoint.
  - Verify `cargo build --release` passes with zero errors after protoc install + any error fixes

  **Must NOT do**:
  - Implement the ToolSidecar communication protocol (separate future milestone)
  - Modify the Go proto stubs (`bridge/pkg/vault/proto/`) — they're pre-generated and correct
  - Add `net`/`sync` features to `tokio-stream` in Cargo.toml (known constraint from learnings)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1a (with Tasks 1.2, 2.2, 4.1)
  - **Blocks**: Task 2.4
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `rust-vault/src/grpc/server.rs` — `run_server()` function already exists, binds Unix socket with 0600
  - `sidecar-python/Dockerfile` — Container hardening pattern (multi-stage, non-root, network none)
  - `deploy/docker-compose.sidecar-py.yml` — Docker Compose sidecar deployment pattern
  - `.sisyphus/notepads/v6-microkernel/learnings.md` — Rust Vault implementation learnings (tokio-stream constraints, gRPC bugs)

  **API/Type References**:
  - `rust-vault/Cargo.toml` — Library crate, dependencies listed
  - `rust-vault/src/lib.rs` — Module structure (config, error, db, blindfill, grpc, governance)
  - `rust-vault/proto/governance.proto` — 4 RPCs: IssueEphemeralToken, ConsumeEphemeralToken, ZeroizeToolSecrets, SubscribeEvents
  - `bridge/pkg/vault/client.go` — Go gRPC client that will connect to the service
  - `bridge/pkg/config/config.go:769-772` — VaultConfig with SocketPath default `/run/armorclaw/keystore.sock`

  **Acceptance Criteria**:

  - [ ] Error categorization complete: actual error count documented, fix plan known
  - [ ] `cd rust-vault && cargo build --release` — zero errors
  - [ ] `cd rust-vault && cargo test` — all pass
  - [ ] `docker compose up rust-vault` — Unix socket created at `/run/armorclaw/keystore.sock`
  - [ ] `bridge/pkg/vault/client.go` confirmed to exist and compile

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Rust Vault binary compiles and runs
    Tool: Bash
    Preconditions: protoc installed
    Steps:
      1. `cd rust-vault && cargo build --release 2>&1`
      2. Verify exit code 0 and output contains "Finished"
      3. Verify binary exists at target/release/rust-vault
    Expected Result: Binary compiled successfully
    Failure Indicators: Compilation errors, missing protoc
    Evidence: .sisyphus/evidence/task-2.1-rust-vault-build.txt

  Scenario: Docker container starts healthy
    Tool: Bash
    Preconditions: Docker available
    Steps:
      1. `docker compose up rust-vault -d`
      2. Wait 10 seconds
      3. `docker compose ps rust-vault` — verify status "healthy"
      4. `docker compose exec rust-vault ls /run/armorclaw/keystore.sock` — verify socket created
    Expected Result: Container running, socket file exists
    Failure Indicators: Container exited, socket missing, unhealthy
    Evidence: .sisyphus/evidence/task-2.1-rust-vault-docker.txt

  Scenario: Go Bridge connects to Rust Vault
    Tool: Bash
    Preconditions: Rust Vault container running
    Steps:
      1. `cd bridge && go test ./pkg/vault/... -v -timeout 30s`
      2. Verify all tests pass
    Expected Result: All vault client tests pass
    Failure Indicators: Connection refused, timeout, gRPC errors
    Evidence: .sisyphus/evidence/task-2.1-vault-client-test.txt
  ```

  **Commit**: YES
  - Message: `feat(vault): add binary entrypoint, Dockerfile, and docker-compose deployment`
  - Files: `rust-vault/src/main.rs`, `rust-vault/Dockerfile`, `docker-compose.yml`
  - Pre-commit: `cd rust-vault && cargo build --release`

- [x] 2.2. PPTX Extractor in Rust Sidecar

  **What to do**:
  - Create `sidecar/src/document/pptx.rs` using the `zip` crate (PPTX is ZIP-based)
  - Parse `ppt/slides/slide*.xml`, extract text from `<a:t>` elements within shape trees
  - Handle: slides enumeration, text frames across shapes/groups/tables, speaker notes from `ppt/notesSlides/`
  - Return structured `ExtractTextResult` matching existing pattern (page count, metadata map, text content)
  - Register in `sidecar/src/document/mod.rs`
  - Add tests: basic slide extraction, multi-slide deck, embedded tables, speaker notes, empty slides

  **Must NOT do**:
  - Modify the existing 3-layer routing in `office_client.go` (Task 2.3 handles routing)
  - Add PPTX support to the Python sidecar (reducing Python dependency is the goal)
  - Change any existing extractor interfaces

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1a (with Tasks 1.2, 2.1, 4.1)
  - **Blocks**: Task 2.3
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `sidecar/src/document/docx.rs` — Reference extractor pattern (ZIP-based, XML parsing, ExtractTextResult)
  - `sidecar/src/document/xlsx.rs` — Another ZIP-based extractor for comparison
  - `sidecar/src/document/mod.rs` — Registration pattern for document extractors

  **API/Type References**:
  - `sidecar/src/document/mod.rs:ExtractTextResult` — Return type with page_count, metadata, text

  **Acceptance Criteria**:

  - [ ] `cargo test --manifest-path sidecar/Cargo.toml -p sidecar -- pptx` — all pass
  - [ ] PPTX extraction returns slide text with correct slide ordering
  - [ ] Speaker notes extracted separately in metadata

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: PPTX text extraction works
    Tool: Bash
    Preconditions: Rust sidecar compiles
    Steps:
      1. Create test PPTX with 3 slides containing known text
      2. Run `cargo test --manifest-path sidecar/Cargo.toml -p sidecar -- test_pptx_extract -v`
      3. Verify output contains text from all 3 slides in order
    Expected Result: All slide text extracted in order
    Failure Indicators: Missing slides, wrong order, empty text
    Evidence: .sisyphus/evidence/task-2.2-pptx-extraction.txt

  Scenario: Malformed PPTX handled gracefully
    Tool: Bash
    Steps:
      1. Create invalid ZIP file renamed to .pptx
      2. Run extraction test
      3. Verify error returned (not panic)
    Expected Result: Descriptive error, no panic
    Failure Indicators: Panic, crash, empty error
    Evidence: .sisyphus/evidence/task-2.2-pptx-error.txt
  ```

  **Commit**: YES
  - Message: `feat(sidecar): add PPTX text extraction in Rust`
  - Files: `sidecar/src/document/pptx.rs`, `sidecar/src/document/mod.rs`

- [x] 3.1. Bridge-Side Session Transcript Compaction

  **What to do**:
  - Create `bridge/internal/ai/compaction.go` implementing **session transcript compaction** (NOT WAL/file compaction)
  - Implement `EstimateMessageTokens(messages []Message) int` — token counting for message history
  - Add `CompactionThresholdTokens int` config field to `RuntimeConfig` with sensible default (e.g., 100000 for large-context models). **Do NOT use `MaxTokens` (4096) as the threshold source** — that's a per-task budget, not a context window.
  - Implement `CompactHistory(ctx context.Context, messages []Message, threshold int) ([]Message, error)` — calls LLM to summarize messages when token count exceeds threshold
  - Wire into step dispatch: before `factory.Spawn()`, check `EstimateMessageTokens` against `CompactionThresholdTokens`; if exceeded, call `CompactHistory` and pass compacted history via `STEP_CONFIG`
  - Use TypeScript `container/openclaw-src/src/agents/compaction.ts` as reference architecture (not copy — Go implementation)
  - Summarization prompt: compress conversation to key facts, decisions, and current state
  - **Cost acknowledgment**: Compaction calls the LLM before every step that exceeds threshold. For a 10-step workflow with long histories, this could mean 5-10 extra LLM calls per workflow. This cost is documented and accepted.

  **Must NOT do**:
  - Rewrite TypeScript compaction system in Go (bridge-side pre-dispatch pruning only)
  - Modify the TypeScript compaction system
  - Implement WAL/file compaction (that's `budget/persistence.go`, completely different)
  - Create a container→Bridge communication channel

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1b (with Task 3.2)
  - **Blocks**: None directly
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `container/openclaw-src/src/agents/compaction.ts:17` — `estimateMessagesTokens()` reference (strips tool-result details, sums per-message estimates)
  - `container/openclaw-src/src/agents/compaction/should-compact.ts` — Proactive compaction trigger logic (75% threshold)
  - `container/openclaw-src/src/agents/pi-embedded-runner/compact.ts:244` — Full compaction pipeline reference

  **API/Type References**:
  - `bridge/internal/ai/client.go` — `AIClient` interface (Chat/ChatStream — use Chat for summarization)
  - `bridge/internal/agent/runtime.go` — Agent runtime with MaxTokens config
  - `bridge/internal/memory/store.go` — Message store (source of messages to compact)

  **Acceptance Criteria**:

  - [ ] `go test ./internal/ai/... -run Compact` — all pass
  - [ ] Compaction triggers when token estimate exceeds `CompactionThresholdTokens` config value
  - [ ] Compacted messages significantly shorter than original (≥50% token reduction)
  - [ ] No compaction when below threshold

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Compaction triggers at threshold
    Tool: Bash
    Steps:
      1. Create test with 100 messages exceeding `CompactionThresholdTokens` (set to 50000 for test)
      2. Run `go test ./internal/ai/... -run TestCompactHistory -v`
      3. Verify compaction was called and output is shorter
    Expected Result: Compacted messages have <50% of original tokens
    Failure Indicators: No compaction, same token count, empty output
    Evidence: .sisyphus/evidence/task-3.1-compaction-trigger.txt

  Scenario: No compaction below threshold
    Tool: Bash
    Steps:
      1. Create test with messages totaling well below `CompactionThresholdTokens` (e.g., 1000 tokens vs threshold of 50000)
      2. Verify no LLM call made
    Expected Result: Original messages returned unchanged
    Failure Indicators: Unnecessary LLM call, messages modified
    Evidence: .sisyphus/evidence/task-3.1-no-compaction.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): bridge-side session transcript compaction before dispatch`
  - Files: `bridge/internal/ai/compaction.go`, `bridge/internal/ai/compaction_test.go`, `bridge/internal/agent/runtime.go`

- [x] 3.2. StepParallel Execution Engine

  **What to do**:
  - Implement parallel step execution in `bridge/pkg/secretary/orchestrator_integration.go`
  - Detect `StepType == StepParallel` or `StepParallelSplit` in `ExecuteSteps()` loop
  - **Split→Merge linking**: Use dependency edges (Option C). Template authors must declare explicit dependencies from Merge to all Split children. The existing `DependencyValidator` topological sort already handles this. No new `WorkflowStep` fields needed. Steps between Split and Merge that share the same Merge dependency are treated as parallel branches.
  - For `StepParallelSplit`: identify all steps whose dependency edges point to the same `StepParallelMerge` step, spawn their containers concurrently using `errgroup` goroutine pool
  - For `StepParallel` (standalone): execute step's sub-steps concurrently
  - Collect results with error aggregation: configurable policy (any-failure-fails-all vs best-effort)
  - Implement `StepParallelMerge` as synchronization barrier — wait for all parallel branches, then continue sequential execution
  - Maintain workflow state consistency under concurrent step completion (mutex on workflow state updates)
  - **Resource governance**: Concurrency limit is configurable via `MaxParallelContainers` in secretary config. Default: 2 (appropriate for minimum VPS spec). Verify against `bridge/pkg/docker/` resource governance patterns. Do NOT default to 4 — that could OOM a small VPS.

  **Must NOT do**:
  - Implement distributed parallel execution across containers (goroutine-based within Bridge only)
  - Change the `StepType` enum or `WorkflowStep` struct
  - Modify the existing sequential execution path (must remain functional for non-parallel steps)

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1b (with Task 3.1)
  - **Blocks**: F1-F4
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:151-234` — Current `ExecuteSteps()` sequential for loop (refactor target)
  - `bridge/pkg/secretary/types.go:99-101` — StepParallel, StepParallelSplit, StepParallelMerge enum values

  **API/Type References**:
  - `bridge/pkg/secretary/types.go:72` — `WorkflowStep.Type StepType` field
  - `bridge/pkg/secretary/orchestrator_dependencies.go` — `DependencyValidator` (topological sort, needs parallel-aware scheduling)

  **Acceptance Criteria**:

  - [ ] `go test ./pkg/secretary/... -run Parallel` — all pass
  - [ ] StepParallelSplit spawns N containers concurrently (verified by timing: N sequential steps take ~N×T, parallel take ~T)
  - [ ] StepParallelMerge blocks until all branches complete
  - [ ] Sequential steps still work unchanged (backward compatibility)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Parallel execution is faster than sequential
    Tool: Bash
    Steps:
      1. Create workflow with 3 parallel steps each taking 2s
      2. Run `go test ./pkg/secretary/... -run TestStepParallelSplit -v`
      3. Verify total execution time < 4s (3 sequential would be 6s)
    Expected Result: Wall clock time ~2s, not ~6s
    Failure Indicators: Time > 4s (sequential execution)
    Evidence: .sisyphus/evidence/task-3.2-parallel-timing.txt

  Scenario: Parallel branch failure handling
    Tool: Bash
    Steps:
      1. Create workflow with 3 parallel steps where 1 fails
      2. Verify configurable policy: any-failure-fails-all vs best-effort
      3. Verify other branches complete even when one fails
    Expected Result: Failure detected, other branches complete, error aggregated
    Failure Indicators: All branches killed on single failure, or failure silently ignored
    Evidence: .sisyphus/evidence/task-3.2-parallel-failure.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): implement StepParallel execution engine`
  - Files: `bridge/pkg/secretary/orchestrator_integration.go`, `bridge/pkg/secretary/orchestrator_parallel.go` (new), tests

- [x] 4.1. Timeline Duration Accuracy Fix

  **What to do**:
  - Update `WorkflowTimeline.kt` to strictly use `duration_ms` from `ExtendedStepResult.Events` for progress bar rendering
  - Data path: container `StepEvent.duration_ms` → `_events.jsonl` → `ExtendedStepResult.Events` → Matrix timeline event → Android `WorkflowEvent.durationMs` → UI
  - Ensure events with `null` duration don't break rendering (default to 0 or indeterminate)
  - Verify the full data path preserves duration values

  **Must NOT do**:
  - Change the `StepEvent` schema or `ExtendedStepResult` parsing
  - Modify the Matrix timeline event format

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1a (with Tasks 1.2, 2.1, 2.2)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/components/WorkflowTimeline.kt` — Current timeline composable

  **API/Type References**:
  - `bridge/pkg/secretary/notifications.go` — `GetTimelineEvents()` with `TimelineEvent.DurationMs`
  - `bridge/pkg/secretary/result.go` — `StepEvent.DurationMs` field

  **Acceptance Criteria**:

  - [ ] `./gradlew assembleDebug` passes in ArmorChat
  - [ ] Timeline renders duration from `duration_ms` field, not estimated

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Duration renders from data
    Tool: Bash
    Steps:
      1. Build ArmorChat: `cd applications/ArmorChat && ./gradlew assembleDebug`
      2. Verify build succeeds
    Expected Result: APK built successfully
    Failure Indicators: Build failure
    Evidence: .sisyphus/evidence/task-4.1-build-success.txt
  ```

  **Commit**: YES
  - Message: `fix(ArmorChat): respect duration_ms in WorkflowTimeline rendering`
  - Files: `applications/ArmorChat/...`

- [x] 1.1. Bridge-Side State Inference Engine

  **What to do**:
  - Define formal CDP→AgentStatus inference table with explicit handling for all 11 states:
    - **Reliable CDP mappings** (observable from browser events):
      - Jetski CDP `Page.frameNavigated` → `BROWSING`
      - Jetski CDP `DOM.focus` on input elements → `FORM_FILLING`
      - Tool dispatch with no browser activity → `INITIALIZING`
    - **Command-driven** (already works via explicit RPC):
      - `RequestPIIAccess()` call → `AWAITING_APPROVAL`
    - **Exit-driven** (determined by container exit, use `ForceTransition`):
      - Container exit 0 → `COMPLETE`
      - Container exit non-zero → `ERROR`
    - **Start/default** (no inference needed):
      - `IDLE` — initial state before any CDP events arrive; inference engine never needs to produce this
    - **Invisible states** (use workflow side-channel or connection events, NOT CDP): `AWAITING_CAPTCHA`, `AWAITING_2FA`, `PROCESSING_PAYMENT`, `OFFLINE` — these cannot be reliably inferred from CDP alone. CAPTCHA/2FA detection via iframe/input patterns produce unacceptable false-positive rates. PROCESSING_PAYMENT is workflow-specific. OFFLINE detected by connection loss to Jetski/container. Infer from workflow state instead.
    - Unknown/unclassifiable CDP events → maintain current state (do NOT transition)
  - **Interaction with existing StateMachine**: Inference produces `AgentStatus` values fed into `StateMachine.ForceTransition()` (NOT `Transition()` — inferred transitions may not follow the normal valid-transitions graph, e.g., CDP idle during AWAITING_APPROVAL should not regress to IDLE).
  - Create `bridge/pkg/agent/state_inference.go` implementing `InferAgentState(cdpEvents []CDPEvent, workflowState WorkflowStatus) AgentStatus`
  - Handle Jetski restart: when CDP connection drops and reconnects, maintain current inferred state (don't reset to IDLE)
  - Handle race condition: CDP events during AWAITING_APPROVAL (browser idle) — must NOT transition to IDLE
  - Write tests covering all 11 states + unknown + race conditions
  - This targets the existing 11-state AgentStatus system ONLY. BrowserState unification is deferred.

  **Must NOT do**:
  - Unify all three state systems into one (AgentStatus only)
  - Modify the existing command-driven transition path (it continues to work for explicit RPC calls)
  - Build per-agent state tracking (inference is per-workflow-instance)

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (conceptually, but starts Wave 2)
  - **Parallel Group**: Wave 2 (with Tasks 2.3, 2.4, 3.3, 4.3)
  - **Blocks**: Task 1.3
  - **Blocked By**: None (can start immediately but logically in Wave 2)

  **References**:

  **Pattern References**:
  - `bridge/pkg/agent/state.go` — All 11 AgentStatus constants, ValidTransitions map
  - `bridge/pkg/agent/state_machine.go` — StateMachine.Transition(), ForceTransition(), convenience methods
  - `bridge/pkg/agent/status_integration.go` — StatusEmitter interfaces (BlindFillStatusEmitter, BrowserStatusEmitter)
  - `bridge/pkg/browser/browser.go` — How browser events trigger state transitions via CallbackAdapter
  - `bridge/pkg/queue/browser_queue.go:663-703` — Existing command-driven transition triggers

  **API/Type References**:
  - `bridge/pkg/agent/state.go:StatusEvent` — Event struct for state changes
  - `jetski/internal/cdp/proxy.go` — CDP proxy that records frames (currently to sonar buffer only)
  - `jetski/internal/sonar/recorder.go` — RecordFrame() writes method+params to circular buffer

  **External References**:
  - CDP spec: https://chromedevtools.github.io/devtools-protocol/ — Page, DOM, Runtime domains for event types

  **Acceptance Criteria**:

  - [ ] `go test ./pkg/agent/... -run Infer` — all pass
  - [ ] Inference table covers all 11 AgentStatus states with explicit mapping or "invisible" classification
  - [ ] Unknown CDP events do not cause state transition
  - [ ] Jetski restart does not reset inferred state

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: CDP navigation event infers BROWSING
    Tool: Bash
    Steps:
      1. Run `go test ./pkg/agent/... -run TestInferAgentState_Browsing -v`
      2. Verify Page.frameNavigated event maps to StatusBrowsing
    Expected Result: Inferred state == BROWSING
    Evidence: .sisyphus/evidence/task-1.1-infer-browsing.txt

  Scenario: Jetski restart preserves state
    Tool: Bash
    Steps:
      1. Set inferred state to FORM_FILLING
      2. Simulate CDP disconnect (no events for 5s)
      3. Verify state remains FORM_FILLING
    Expected Result: State unchanged during disconnect
    Evidence: .sisyphus/evidence/task-1.1-jetski-restart.txt

  Scenario: Invisible state handled via workflow
    Tool: Bash
    Steps:
      1. Workflow status = AWAITING_APPROVAL
      2. CDP events = idle (no browser activity)
      3. Verify inference produces AWAITING_APPROVAL from workflow state, not IDLE from CDP
    Expected Result: State = AWAITING_APPROVAL
    Failure Indicators: State transitions to IDLE during approval wait
    Evidence: .sisyphus/evidence/task-1.1-invisible-state.txt
  ```

  **Commit**: YES
  - Message: `feat(agent): implement bridge-side state inference from CDP/workflow signals`
  - Files: `bridge/pkg/agent/state_inference.go`, `bridge/pkg/agent/state_inference_test.go`

- [x] 2.3. Route XLSX and PPTX to Rust Sidecar

  **What to do**:
  - Update 3-layer routing in `bridge/pkg/sidecar/office_client.go` Layer 1:
    - Route `.xlsx` to Rust sidecar (currently routed to Python; `xlsx.rs` exists)
    - Route `.pptx` to Rust sidecar (new; `pptx.rs` from Task 2.2)
    - Keep `.msg`, `.doc`, `.xls`, `.ppt` on Python (no Rust extractors)
  - Update existing routing tests to reflect new destinations
  - Python sidecar remains for 4 legacy formats — cannot be decommissioned

  **Must NOT do**:
  - Decommission the Python sidecar
  - Change the 3-layer routing architecture (Layer 0/1/2 pattern stays)
  - Modify Layer 2 strict-drop validation

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 1.1, 2.4, 3.3, 4.3)
  - **Blocks**: None
  - **Blocked By**: Task 2.2 (PPTX extractor must exist)

  **References**:

  **Pattern References**:
  - `bridge/pkg/sidecar/office_client.go:73-85` — Layer 1 compound validation (ZIP magic + MIME type → routing)

  **Acceptance Criteria**:

  - [ ] `go test ./pkg/sidecar/... -run RouteExtractText -v` — all pass with updated routing
  - [ ] XLSX requests route to Rust sidecar
  - [ ] PPTX requests route to Rust sidecar
  - [ ] MSG/XLS/DOC/PPT requests still route to Python sidecar

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: XLSX routes to Rust
    Tool: Bash
    Steps:
      1. Run `go test ./pkg/sidecar/... -run TestRouteExtractText_XLSX -v`
      2. Verify routing decision targets Rust sidecar
    Expected Result: Rust sidecar selected for XLSX
    Evidence: .sisyphus/evidence/task-2.3-xlsx-routing.txt

  Scenario: PPTX routes to Rust
    Tool: Bash
    Steps:
      1. Run `go test ./pkg/sidecar/... -run TestRouteExtractText_PPTX -v`
      2. Verify routing decision targets Rust sidecar
    Expected Result: Rust sidecar selected for PPTX
    Evidence: .sisyphus/evidence/task-2.3-pptx-routing.txt
  ```

  **Commit**: YES
  - Message: `feat(sidecar): route XLSX and PPTX to Rust sidecar`
  - Files: `bridge/pkg/sidecar/office_client.go`, `bridge/pkg/sidecar/office_client_test.go`

- [x] 2.4. v6 Microkernel Audit Mode

  **What to do**:
  - Add `V6AuditMode` to `VaultConfig` in `bridge/pkg/config/config.go` — when `true`, MCP Router logs every tool call that *would* be intercepted without actually intercepting
  - Wire `ComplianceAuditLog` into MCP Router execution path (currently uses `AuditLog`)
  - Audit mode behavior:
    - Log every tool call that SkillGate *would* intercept (PII detected, blocked, redacted)
    - Log every governance check that *would* be made (vault token issued, secret zeroized)
    - Log every ToolSidecar that *would* be spawned (image, skill, arguments)
    - Do NOT actually intercept, spawn containers, or issue tokens
    - Produce structured audit report to logs
  - Add `v6_audit_mode` TOML config and `ARMORCLAW_V6_AUDIT_MODE` env var
  - Defer mTLS enforcement until Rust Vault is deployed and accessible

  **Must NOT do**:
  - Build per-agent/per-tenant feature flag infrastructure (use global VaultConfig)
  - Implement ToolSidecar communication protocol
  - Change default of `v6_microkernel` from `false` (enforcement deferred to future milestone)

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 1.1, 2.3, 3.3, 4.3)
  - **Blocks**: F1-F4
  - **Blocked By**: Task 2.1 (vault must be deployable for gRPC connectivity)

  **References**:

  **Pattern References**:
  - `bridge/pkg/mcp/router.go:73-111` — MCPRouter struct and initialization
  - `bridge/pkg/mcp/router.go:324-346` — v6 microkernel conditional paths
  - `bridge/pkg/config/config.go:769-772` — VaultConfig with V6Microkernel flag

  **API/Type References**:
  - `bridge/pkg/interfaces/skillgate.go` — SkillGate interface (4 methods to log-but-skip)
  - `bridge/pkg/toolsidecar/toolsidecar.go` — ToolSidecar provisioner (log-but-skip spawning)
  - `bridge/pkg/governor/skillgate.go` — Governor (PII detection to log)

  **Acceptance Criteria**:

  - [ ] `go test ./pkg/mcp/... -run Audit -v` — all pass
  - [ ] With `v6_audit_mode=true`, tool calls pass through unmodified but logged
  - [ ] With `v6_audit_mode=false` and `v6_microkernel=false`, legacy path unchanged

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Audit mode logs without intercepting
    Tool: Bash
    Steps:
      1. Set v6_audit_mode=true in test config
      2. Execute skill that would trigger PII detection
      3. Verify log contains "would intercept" entry
      4. Verify tool call passes through unmodified
    Expected Result: Tool call succeeds normally, audit log entries present
    Failure Indicators: Tool call intercepted/blocked, or no log entries
    Evidence: .sisyphus/evidence/task-2.4-audit-mode.txt

  Scenario: Legacy path unaffected
    Tool: Bash
    Steps:
      1. Set v6_microkernel=false, v6_audit_mode=false
      2. Run existing skill execution tests
      3. Verify all pass unchanged
    Expected Result: All existing tests pass
    Evidence: .sisyphus/evidence/task-2.4-legacy-path.txt
  ```

  **Commit**: YES
  - Message: `feat(mcp): add v6 microkernel audit mode`
  - Files: `bridge/pkg/config/config.go`, `bridge/pkg/mcp/router.go`, `bridge/pkg/mcp/router_test.go`

- [x] 3.3. Step Failover (Per-Step, Not Parallel-Specific)

  **What to do**:
  - Implement failover in `executeStep()` itself (or a wrapper), independent of the parallel engine
  - Both sequential and parallel paths call the same step execution function, so both get failover
  - If a step's primary `AgentIDs[0]` returns `StatusFailed`, attempt execution with `AgentIDs[1:]`
  - Try each fallback agent in order until one succeeds or all exhausted
  - Log each failover attempt with agent ID and failure reason
  - Configurable failover policy: retry-on-failure vs immediate-fail
  - Aggregate results: report which agents were attempted and which succeeded

  **Must NOT do**:
  - Embed failover inside the parallel engine (it's a per-step concern)
  - Change the `WorkflowStep.AgentIDs` schema

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 1.1, 2.3, 2.4, 4.3)
  - **Blocks**: None
  - **Blocked By**: None (decoupled from Task 3.2)

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/types.go:WorkflowStep.AgentIDs` — Agent ID list for failover

  **Acceptance Criteria**:

  - [ ] `go test ./pkg/secretary/... -run Failover -v` — all pass
  - [ ] Primary agent failure triggers fallback to secondary
  - [ ] All agents exhausted → step fails with aggregated error

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Failover on primary failure
    Tool: Bash
    Steps:
      1. Create step with AgentIDs: ["fail-agent", "success-agent"]
      2. "fail-agent" returns StatusFailed
      3. Verify "success-agent" is attempted
      4. Verify step succeeds with success-agent result
    Expected Result: Step succeeds via failover
    Failure Indicators: Step fails after primary failure without trying secondary
    Evidence: .sisyphus/evidence/task-3.3-failover-success.txt

  Scenario: All agents exhausted
    Tool: Bash
    Steps:
      1. Create step with AgentIDs: ["fail1", "fail2", "fail3"]
      2. All return StatusFailed
      3. Verify step fails with error listing all attempted agents
    Expected Result: Step fails, error mentions all 3 agents
    Evidence: .sisyphus/evidence/task-3.3-failover-exhausted.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add step failover with multi-agent fallback`
  - Files: `bridge/pkg/secretary/orchestrator_integration.go`, tests

- [x] 1.3. Implement BroadcastStatus() from State Inference

  **What to do**:
  - Replace the stub `BroadcastStatus()` in `bridge/pkg/agent/integration.go:339-341` with a real implementation
  - Consume Bridge-inferred state transitions from the inference engine (Task 1.1)
  - Emit `com.armorclaw.agent.status` Matrix events to the agent's room
  - Use existing `MatrixEventBus.Publish()` pattern from `WorkflowEventEmitter`
  - Include state metadata: workflow_id, step_name, inferred_from (CDP/workflow/command), timestamp
  - Register event type in `processEvents()` routing in `internal/adapter/matrix.go`

  **Must NOT do**:
  - Emit status events from containers (Bridge-side only)
  - Create a new event bus (use existing MatrixEventBus)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (with Task 4.2)
  - **Blocks**: None
  - **Blocked By**: Task 1.1 (state inference engine)

  **References**:

  **Pattern References**:
  - `bridge/pkg/agent/integration.go:339-341` — Current stub (always returns error)
  - `bridge/pkg/secretary/orchestrator_events.go` — `WorkflowEventEmitter` pattern for Matrix event publishing
  - `bridge/internal/adapter/matrix.go` — `processEvents()` routing for custom event types

  **Acceptance Criteria**:

  - [ ] `go test ./pkg/agent/... -run BroadcastStatus -v` — all pass
  - [ ] State change produces Matrix event in agent room
  - [ ] Event includes: state, workflow_id, inferred_from, timestamp

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: State change emits Matrix event
    Tool: Bash
    Steps:
      1. Run `go test ./pkg/agent/... -run TestBroadcastStatus -v`
      2. Verify event published to MatrixEventBus
      3. Verify event type is com.armorclaw.agent.status
    Expected Result: Matrix event emitted with correct payload
    Evidence: .sisyphus/evidence/task-1.3-broadcast-status.txt
  ```

  **Commit**: YES
  - Message: `feat(agent): implement BroadcastStatus with state inference`
  - Files: `bridge/pkg/agent/integration.go`, `bridge/internal/adapter/matrix.go`

- [x] 4.2. Dynamic PII Masking + Android Event Parsing in BlockerResponseDialog

  **What to do**:
  - **Android event parsing**: Add code in ArmorChat to construct `BlockerInfo` from `workflow.blocker_warning` Matrix events. Currently `BlockerResponseDialog.kt` is dead code — nothing constructs `BlockerInfo` from events. Follow `PiiApprovalCard.kt` pattern for Matrix event → UI model parsing.
  - Update `BlockerResponseDialog.kt` to dynamically mask input fields based on the `field` name from blocker metadata (now available on the wire after Task 1.2 fix)
  - Sensitive field names: password, card, key, token, secret, cvv, pin, ssn
  - Masking: use `TextField` with `visualTransformation = PasswordVisualTransformation()` for sensitive fields

  **Must NOT do**:
  - Change the BlockerResponseDialog composable structure
  - Add new blocker types
  - Modify Bridge-side blocker handling (that's Task 1.2)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Task 1.3)
  - **Blocks**: None
  - **Blocked By**: Task 1.2 (blocker metadata must include field)

  **References**:

  **Pattern References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/components/PiiApprovalCard.kt` — Pattern for Matrix event → UI model parsing + PII masking
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/components/BlockerResponseDialog.kt` — Current dialog (field masking + event parsing needed)

  **Acceptance Criteria**:

  - [ ] `./gradlew assembleDebug` passes
  - [ ] Field named "password" renders with password masking
  - [ ] Field named "email" renders without masking
  - [ ] BlockerInfo constructed from Matrix `workflow.blocker_warning` events

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Build succeeds with masking changes
    Tool: Bash
    Steps:
      1. `cd applications/ArmorChat && ./gradlew assembleDebug`
      2. Verify build succeeds
    Expected Result: APK built
    Evidence: .sisyphus/evidence/task-4.2-build.txt
  ```

  **Commit**: YES
  - Message: `feat(ArmorChat): dynamic PII masking in BlockerResponseDialog`
  - Files: `applications/ArmorChat/...`

- [x] 4.3. Email Approval Card + Bridge RPC Handlers

  **What to do**:

  **Bridge-side**:
  - Create `bridge/pkg/rpc/email_approval.go` with `approve_email` and `deny_email` RPC handlers (flat method names matching existing convention: `resolve_blocker`, `claim_admin`, `add_key`)
  - Follow `bridge/pkg/rpc/pii.go` as the exact pattern for approval RPC handlers
  - Wire handlers in RPC method registration
  - Handlers call `EmailApprovalManager.HandleApprovalResponse()` (already implemented)

  **Android-side**:
  - Add `AlertType.EMAIL_APPROVAL_REQUEST` to `SystemAlert.kt`
  - Create `EmailApprovalCard.kt` composable following `PiiApprovalCard.kt` pattern
  - Add Matrix event listener for `app.armorclaw.email_approval_request` in Android sync processing
  - Add BridgeApi call to submit approval response (approve/deny)
  - Handle timeout: show expired state when 300s approval window expires
  - Verify Android Matrix sync filter includes `app.armorclaw.email_approval_request`

  **Must NOT do**:
  - Build email composition UI (approval card only)
  - Modify `EmailApprovalManager` (already works)
  - Change the approval timeout (300s is configured)

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 1.1, 2.3, 2.4, 3.3)
  - **Blocks**: None
  - **Blocked By**: None (no v6 dependency — EmailApprovalManager operates independently)

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/pii.go` — EXACT pattern for approval RPC handlers (must follow this)
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/components/PiiApprovalCard.kt` — EXACT pattern for approval card UI
  - `bridge/pkg/email/hitl_approval.go` — EmailApprovalManager (already implemented, needs RPC wiring)

  **API/Type References**:
  - `bridge/pkg/email/hitl_approval.go:77` — Sends `app.armorclaw.email_approval_request` event
  - `bridge/pkg/email/hitl_approval.go:HandleApprovalResponse()` — Accepts approval_id, approved bool
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/data/model/SystemAlert.kt` — AlertType enum (needs EMAIL_APPROVAL_REQUEST)

  **Acceptance Criteria**:

  - [ ] `go test ./pkg/rpc/... -run EmailApproval -v` — all pass
  - [ ] `./gradlew assembleDebug` passes
  - [ ] Bridge receives `approve_email` RPC and calls HandleApprovalResponse
  - [ ] Email with PII: approval request → Android card → approve → email sends

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Bridge RPC handler processes approval
    Tool: Bash
    Steps:
      1. Run `go test ./pkg/rpc/... -run TestEmailApprovalRPC -v`
      2. Verify HandleApprovalResponse called with correct approval_id and approved=true
    Expected Result: RPC handler wired correctly
    Evidence: .sisyphus/evidence/task-4.3-rpc-handler.txt

  Scenario: Android builds with email approval card
    Tool: Bash
    Steps:
      1. `cd applications/ArmorChat && ./gradlew assembleDebug`
      2. Verify build succeeds
    Expected Result: APK built with EmailApprovalCard
    Evidence: .sisyphus/evidence/task-4.3-android-build.txt
  ```

  **Commit**: YES
  - Message: `feat(email): email approval card + bridge RPC handlers`
  - Files: `bridge/pkg/rpc/email_approval.go`, `applications/ArmorChat/...`

- [x] 4.4. VERSION + CHANGELOG + Documentation Update

  **What to do**:
  - Update `VERSION` file from 0.4.1 to 0.6.0
  - Add v0.6.0 entry to `CHANGELOG.md` with all features from this plan
  - Update `PRODUCTION_READINESS.md` with new capabilities
  - Update relevant `doc/*.md` files to reflect changes:
    - `doc/agent-runtime.md` — state inference section
    - `doc/secretary-workflow.md` — blocker pipeline fix, parallel execution, compaction
    - `doc/communication-infra.md` — BroadcastStatus, email approval events
    - `doc/sidecar-pipeline.md` — PPTX routing to Rust
    - `doc/armorclaw.md` — Rust Vault deployment, v6 microkernel status

  **Must NOT do**:
  - Change any code files
  - Add new documentation files outside `doc/`

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4 (sole task)
  - **Blocks**: None
  - **Blocked By**: All implementation tasks complete

  **References**:

  **Pattern References**:
  - `CHANGELOG.md` — Existing changelog format
  - `VERSION` — Current version 0.4.1
  - `PRODUCTION_READINESS.md` — Existing format

  **Acceptance Criteria**:

  - [ ] VERSION file contains `0.6.0`
  - [ ] CHANGELOG.md has v0.6.0 entry listing all deliverables
  - [ ] All modified doc files render correctly (no broken links)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Version bumped correctly
    Tool: Bash
    Steps:
      1. `cat VERSION`
      2. Verify output is "0.6.0"
    Expected Result: 0.6.0
    Evidence: .sisyphus/evidence/task-4.4-version.txt
  ```

  **Commit**: YES
  - Message: `chore: bump VERSION to 0.6.0, update CHANGELOG and docs`
  - Files: `VERSION`, `CHANGELOG.md`, `PRODUCTION_READINESS.md`, `doc/*.md`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists. For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go test ./...` + `cargo test` + linters. Review all changed files for: `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **End-to-End Integration QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration. Test edge cases: concurrent blockers, Jetski restart during inference, compaction during active tailing, email batch approval. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff. Verify 1:1 — everything in spec was built, nothing beyond spec was built. Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **1.2**: `fix(secretary): fix blocker metadata pipeline — 7 bugs container→Bridge→Matrix`
- **2.1**: `feat(vault): add binary entrypoint, Dockerfile, and docker-compose deployment`
- **2.2**: `feat(sidecar): add PPTX text extraction in Rust`
- **2.3**: `feat(sidecar): route XLSX and PPTX to Rust sidecar`
- **2.4**: `feat(mcp): add v6 microkernel audit mode`
- **1.1**: `feat(agent): implement bridge-side state inference from CDP/workflow signals`
- **1.3**: `feat(agent): implement BroadcastStatus with state inference`
- **3.1**: `feat(secretary): bridge-side session transcript compaction before dispatch`
- **3.2**: `feat(secretary): implement StepParallel execution engine`
- **3.3**: `feat(secretary): add step failover with multi-agent fallback`
- **4.1**: `fix(ArmorChat): respect duration_ms in WorkflowTimeline rendering`
- **4.2**: `feat(ArmorChat): dynamic PII masking + event parsing in BlockerResponseDialog`
- **4.3**: `feat(email): email approval card + bridge RPC handlers`
- **4.4**: `chore: bump VERSION to 0.6.0, update CHANGELOG and docs`

---

## Success Criteria

### Verification Commands
```bash
cd bridge && go test ./...                        # Expected: PASS, 0 failures
cd rust-vault && cargo build --release            # Expected: Finished, 0 errors
cd rust-vault && cargo test                       # Expected: PASS, 0 failures
docker compose up rust-vault && docker compose ps # Expected: healthy
cd sidecar && cargo test                          # Expected: PASS, 0 failures
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass (Go, Rust, Python)
- [ ] Rust Vault container healthy
- [ ] VERSION file says 0.6.0
- [ ] CHANGELOG.md has v0.6.0 entry
