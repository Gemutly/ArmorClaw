# Architectural Cleanup — Dead Code Removal + Gap Documentation

## TL;DR

> **Quick Summary**: Remove vestigial Rust blindfill module, clean dead EventBus code paths, and document three architectural gaps (voice providers, agent state visibility, dual EventBus).
> 
> **Deliverables**:
> - Rust vault `blindfill/` module deleted (4 source + 3 test files, ~1,576 lines)
> - Dead EventBus code paths removed (EventPublisher, ReceiveEvents)
> - `doc/voice-stack.md` rewritten to document interface-only gap
> - `review.md` voice status corrected
> - `doc/agent-runtime.md` updated with state inference gap
> - `doc/communication-infra.md` updated with dual-bus architecture clarity
> - CHANGELOG.md corrected (wrong state names)
> - Architecture comments added to EventBus packages
> 
> **Estimated Effort**: Medium
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: T1 (blindfill deletion) → T8 (Rust build verify) → F1-F4

---

## Context

### Original Request
Address 4 architectural concerns: (1) Rust Vault CdpInterceptor is vestigial — blindfill has zero production callers, Jetski superseded it. (2) Multiple event bus implementations cause naming confusion. (3) Voice services are interface-only — no concrete STT/TTS/VAD providers. (4) Agent state visibility gap — containers can't report BROWSING/FORM_FILLING states.

### Interview Summary
**Key Discussions**:
- Resolution level: "Remove dead code + document rest" — no full implementation for any concern
- C1 confirmed safe: only one compilation coupling (error.rs → PlaceholderParseError)
- Prometheus metric names with "blindfill" left as-is (cosmetic legacy, avoids breaking dashboards)
- Voice code NOT deleted — ~3,300 lines retained as functional skeleton, but documented as interface-only
- `doc/voice-stack.md` to be rewritten (not just warning header) to document the gap accurately

**Research Findings**:
- C1: Rust blindfill has exactly ONE compilation coupling (error.rs). Go BlindFill (14+ files) is completely separate and unaffected.
- C2: `RegisterBridgeHandler` has ONE live registration (email.received at setup_email.go:60) — must be preserved. EventPublisher in matrix adapter is dead (never set). 3 files import both bus packages.
- C3: Voice infrastructure (WebRTC, signaling, budget, security) is fully implemented. Only STT/TTS/VAD providers are missing. review.md incorrectly says "Production Ready".
- C4: 11 agent states (not 12), 8 parallel state enums across codebase. State inference engine exists but is completely unwired in production. Jetski CDP not fed to inference.

### Metis Review
**Identified Gaps** (addressed):
- RegisterBridgeHandler is NOT dead (one email registration) — MUST NOT remove it
- Agent states = 11 not 12, parallel enums = 8 not 4 — corrected in plan
- Voice has ~3,300 lines of dead code but user chose markers-only approach
- review.md voice status is actively misleading — must be fixed
- C1 ↔ C4 cross-coupling via BlindFillStatusEmitter in agent package — must not break

---

## Work Objectives

### Core Objective
Remove dead Rust blindfill code, clean dead EventBus paths, and honestly document architectural gaps in voice services, agent state visibility, and dual EventBus architecture.

### Concrete Deliverables
- `rust-vault/src/blindfill/` deleted (4 source files)
- `rust-vault/tests/blindfill_*.rs` deleted (3 test files)
- `rust-vault/src/error.rs` cleaned (removed PlaceholderParseError import + From impl)
- `rust-vault/src/lib.rs` cleaned (removed blindfill module)
- `bridge/internal/adapter/matrix.go` cleaned (removed dead EventPublisher code)
- `doc/voice-stack.md` rewritten
- `doc/agent-runtime.md` updated
- `doc/communication-infra.md` updated
- `review.md` voice status corrected
- `CHANGELOG.md` wrong state names corrected
- Architecture comments in `pkg/eventbus/eventbus.go` and `internal/events/matrix_event_bus.go`

### Definition of Done
 - [ ] `cd rust-vault && cargo build && cargo test` passes with zero blindfill references
- [ ] `cd bridge && go build ./cmd/bridge` passes
- [ ] `review.md` does NOT say "Production Ready" for voice
- [ ] `doc/voice-stack.md` contains "interface-only" or "no concrete provider"
- [ ] `doc/agent-runtime.md` documents 11 states and the inference-wiring gap
- [ ] No code references to `blindfill` module in rust-vault/src/ (except cosmetic metric names)

### Must Have
- Blindfill source AND test files fully deleted
- error.rs compilation coupling resolved (no dangling imports)
- RegisterBridgeHandler and its email registration PRESERVED
- review.md voice status corrected
- voice-stack.md accurately documents interface-only state

### Must NOT Have (Guardrails)
- Do NOT remove `RegisterBridgeHandler()` or the email.received registration in setup_email.go
- Do NOT remove Go `BlindFillExecutor`, `BlindFillEngine`, or Android `BlindFillCard.kt` — separate from Rust blindfill
- Do NOT delete `bridge/pkg/voice/` package — it's a functional skeleton for future implementation
- Do NOT delete `bridge/pkg/webrtc/` — independent subsystem
- Do NOT touch `bridge/pkg/agent/status_integration.go` BlindFillStatusEmitter — it's in the agent package, not blindfill
- Do NOT rename Prometheus metrics with "blindfill" in their names — cosmetic legacy only
- Do NOT change any protobuf definitions
- Do NOT weaken SQLCipher, approval flows, or security constraints

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (Rust cargo test, Go build)
- **Automated tests**: Tests-after (verify builds pass after deletions)
- **Framework**: cargo test (Rust — lib + integration), go build (Go)

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Rust**: Use Bash (cargo build/test) — compile, run tests, verify no blindfill warnings
- **Go**: Use Bash (go build/vet) — compile, verify no import errors
- **Docs**: Use Bash (grep) — verify specific strings present/absent

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — 5 parallel tasks):
├── T1: Delete blindfill source + test files + cleanup error.rs + lib.rs [deep]
├── T2: Rewrite doc/voice-stack.md to document gap [writing]
├── T3: Fix review.md voice status + add voice interface-only markers [quick]
├── T4: Update doc/agent-runtime.md with state inference gap [writing]
└── T5: Add EventBus architecture comments in both bus packages [quick]

Wave 2 (After Wave 1 — 3 tasks, max parallel):
├── T6: Remove dead EventPublisher code path from matrix adapter [unspecified-high]
├── T7: Update doc/communication-infra.md with dual-bus clarity [writing]
└── T8: Fix CHANGELOG.md wrong state names [quick]

Wave 3 (After Wave 2 — 2 verification tasks):
├── T9: Rust vault build + test verification [quick]
└── T10: Go bridge build verification [quick]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── F1: Plan compliance audit [oracle]
├── F2: Code quality review [unspecified-high]
├── F3: Real manual QA [unspecified-high]
└── F4: Scope fidelity check [deep]

Critical Path: T1 → T9 → F1-F4
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 5 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| T1 | - | T9 |
| T2 | - | - |
| T3 | - | - |
| T4 | - | - |
| T5 | - | T6 |
| T6 | T5 | T10 |
| T7 | T5 | - |
| T8 | - | - |
| T9 | T1 | F1-F4 |
| T10 | T6 | F1-F4 |

### Agent Dispatch Summary

- **Wave 1**: 5 — T1 → `deep`, T2 → `writing`, T3 → `quick`, T4 → `writing`, T5 → `quick`
- **Wave 2**: 3 — T6 → `unspecified-high`, T7 → `writing`, T8 → `quick`
- **Wave 3**: 2 — T9 → `quick`, T10 → `quick`
- **FINAL**: 4 — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Delete Rust Vault Blindfill Module + Cleanup Couplings

  **What to do**:
  - Delete `rust-vault/src/blindfill/` directory entirely (4 files: mod.rs, placeholder.rs, cdp_interceptor.rs, integration.rs)
  - Delete `rust-vault/tests/blindfill_integration_test.rs`
  - Delete `rust-vault/tests/cdp_interceptor_test.rs`
  - Delete `rust-vault/tests/placeholder_test.rs`
  - Edit `rust-vault/src/lib.rs`: Remove `pub mod blindfill;` and its vestigial comment
  - Edit `rust-vault/src/error.rs`:
    - Remove `use crate::blindfill::placeholder::PlaceholderParseError;` (line 1)
    - Remove the `impl From<PlaceholderParseError> for VaultError` block (lines ~49-67)
    - **BEFORE removing** `VaultError::InvalidPlaceholderFormat` and `VaultError::SecretNotFound` variants: run a deterministic check:
      ```bash
      grep -r "InvalidPlaceholderFormat\|SecretNotFound" rust-vault/src/ --include="*.rs" | grep -v "error.rs"
      ```
      If zero matches → safe to remove the variants. If matches exist → keep variants, remove only the `From` impl.
  - Do NOT touch `governance/ephemeral.rs` metric names (cosmetic legacy, user decision)
  - Do NOT touch Go BlindFill code or BlindFillStatusEmitter in agent package

  **Must NOT do**:
  - Do NOT modify Go bridge BlindFill code
  - Do NOT modify BlindFillStatusEmitter in agent package
  - Do NOT rename Prometheus metrics
  - Do NOT touch protobuf definitions

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3, 4, 5)
  - **Blocks**: Task 9 (Rust build verification)
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `rust-vault/src/blindfill/mod.rs` — Module root declaring submodules and `#[doc(hidden)]` re-exports. Contains "VESTIGIAL" comment confirming zero callers.
  - `rust-vault/src/error.rs:1` — The import line to remove: `use crate::blindfill::placeholder::PlaceholderParseError;`
  - `rust-vault/src/error.rs:49-67` — The `From<PlaceholderParseError> for VaultError` impl block to remove
  - `rust-vault/src/lib.rs:5` — The `pub mod blindfill;` declaration to remove

  **API/Type References**:
  - `rust-vault/src/governance/ephemeral.rs:11-90` — 6 metric functions with "blindfill" names. DO NOT TOUCH — cosmetic legacy only.
  - `bridge/pkg/agent/status_integration.go` — `BlindFillStatusEmitter` lives here. NOT related to Rust blindfill. Do NOT touch.

  **WHY Each Reference Matters**:
  - error.rs is the ONLY compilation coupling — must be untangled before lib.rs can remove the module
  - lib.rs declares the module — must be last file edited (after error.rs cleanup)
  - ephemeral.rs metrics share "blindfill" naming but have ZERO code imports from the module — safe to leave

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Rust vault compiles without blindfill
    Tool: Bash
    Preconditions: blindfill/ directory deleted, error.rs cleaned, lib.rs cleaned
    Steps:
      1. cd rust-vault && cargo build 2>&1
      2. Verify exit code 0
      3. Verify no "blindfill" in compiler output (excluding metric names)
    Expected Result: Clean compilation with zero errors
    Failure Indicators: Compilation error about PlaceholderParseError or missing module
    Evidence: .sisyphus/evidence/task-1-rust-build.txt

  Scenario: No blindfill code-path references remain
    Tool: Bash
    Preconditions: All deletions complete
    Steps:
      1. grep -r "blindfill" rust-vault/src/ --include="*.rs" -l
      2. For each matched file: verify it contains only cosmetic metric name strings (function/variable names in governance/ephemeral.rs), NOT module imports, type references, or function calls
      3. grep -r "use.*blindfill\|mod blindfill\|blindfill::" rust-vault/src/ --include="*.rs"
      4. Verify exit code 1 (no module/code-path references)
    Expected Result: Zero module or code-path references. Cosmetic metric name strings in ephemeral.rs are acceptable.
    Failure Indicators: Any .rs file still importing from or calling into the blindfill module
    Evidence: .sisyphus/evidence/task-1-no-blindfill-refs.txt
  ```

  **Commit**: YES
  - Message: `refactor(vault): remove vestigial blindfill module — superseded by Jetski CDP proxy`
  - Files: `rust-vault/src/blindfill/*`, `rust-vault/tests/blindfill_*`, `rust-vault/src/error.rs`, `rust-vault/src/lib.rs`
  - Pre-commit: `cd rust-vault && cargo build && cargo test`

- [x] 2. Rewrite doc/voice-stack.md to Document Interface-Only Gap

  **What to do**:
  - Read current `doc/voice-stack.md` (197 lines) — it describes a fully functional voice stack with 0 providers
  - Rewrite to accurately reflect the ACTUAL state:
    - WebRTC infrastructure (engine, signaling, sessions) — FULLY implemented
    - Budget/security enforcement — FULLY implemented
    - Matrix call signaling — IMPLEMENTED but unwired (voice manager commented out in main.go)
    - STT/TTS/VAD services — INTERFACE ONLY, no concrete providers
    - Audio pipeline (readRTP → decode → VAD → STT → AI → TTS → encode → write) — NOT implemented
    - Voice manager initialization — COMMENTED OUT in main.go
  - Add prominent section: "Provider Gap" listing exactly what's missing
  - Keep the architecture diagrams and interface definitions (they're accurate)
  - Note the E2E test expectations (HTTP sidecar services at ports 8001/8002/8003)
  - Note duplicate interfaces between `interfaces/voice.go` and `voice/` package (different method signatures)
  - Do NOT delete the file — rewrite it to be accurate

  **Must NOT do**:
  - Do NOT delete voice-stack.md
  - Do NOT add AI slop (unnecessary verbosity, marketing language)
  - Do NOT claim providers exist when they don't

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3, 4, 5)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `doc/voice-stack.md` — Current file (197 lines) to rewrite. Contains inaccurate claims about functional services.
  - `doc/sidecar-pipeline.md` — Example of how to document a pipeline with both implemented and missing parts accurately.

  **API/Type References**:
  - `bridge/pkg/voice/stt_service.go` — `Transcriber` interface + `STTService` wrapper (interface-only, no provider)
  - `bridge/pkg/voice/tts_service.go` — `Synthesizer` interface + `TTSService` wrapper (interface-only, no provider)
  - `bridge/pkg/voice/vad_service.go` — `SpeechDetector` interface + `VADService` wrapper (interface-only, no provider)
  - `bridge/pkg/voice/manager.go` — Voice Manager orchestrator (implemented but voiceMgr = nil)
  - `bridge/pkg/voice/budget.go` — BudgetTracker (FULLY implemented)
  - `bridge/pkg/voice/security.go` — SecurityEnforcer (FULLY implemented)
  - `bridge/pkg/voice/matrix.go` — MatrixManager call signaling (implemented but unwired)
  - `bridge/pkg/webrtc/engine.go` — WebRTC Engine (FULLY implemented, pion/webrtc v3)
  - `bridge/pkg/interfaces/voice.go` — Canonical interface definitions (different method signatures from voice/ package)
  - `bridge/cmd/bridge/main.go:56-57,1988-2103,2776` — Voice import + init + shutdown — ALL COMMENTED OUT

  **WHY Each Reference Matters**:
  - The rewrite must distinguish between what's real (WebRTC, budget, security) and what's missing (STT/TTS/VAD providers, audio pipeline, wiring)
  - interfaces/voice.go has DIFFERENT method signatures from voice/ package — this is a real discrepancy worth noting
  - main.go comments explain WHY voice is disabled — this context belongs in the doc

  **Acceptance Criteria**:

  - [ ] File contains a "Current State" section with a two-column table: **Implemented** (WebRTC, budget, security, Matrix signaling) vs **Missing** (STT/TTS/VAD providers, audio pipeline, voice manager wiring)
  - [ ] Old misleading claims ("all three service wrappers are implemented and tested") are removed

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: voice-stack.md has Implemented vs Missing table
    Tool: Bash
    Preconditions: File rewritten
    Steps:
      1. grep -c "Implemented\|Missing\|Current State" doc/voice-stack.md
      2. Verify count >= 4 (section header + at least 1 Implemented + 1 Missing + table structure)
    Expected Result: Clear two-column status table present
    Failure Indicators: No status table or only one column
    Evidence: .sisyphus/evidence/task-2-voice-gap-documented.txt

  Scenario: voice-stack.md does not claim functional providers
    Tool: Bash
    Preconditions: File rewritten
    Steps:
      1. grep -i "all three service wrappers are implemented and tested" doc/voice-stack.md
      2. Verify exit code 1 (no match)
    Expected Result: No match for previously misleading claim
    Failure Indicators: Old misleading text still present
    Evidence: .sisyphus/evidence/task-2-no-misleading-claims.txt
  ```

  **Commit**: YES (groups with Task 3)
  - Message: `docs(voice): rewrite voice-stack.md and correct review.md status`
  - Files: `doc/voice-stack.md`

- [x] 3. Fix review.md Voice Status + Add Interface-Only Markers

  **What to do**:
  - Find and fix the voice status line in `review.md` that says "Voice (WebRTC) | **Production Ready**" — change to accurately reflect interface-only state
  - Add interface-only markers in the voice package source files:
    - `bridge/pkg/voice/stt_service.go`: Add comment above `Transcriber` interface: `// INTERFACE-ONLY: No concrete provider implementations exist. See doc/voice-stack.md.`
    - `bridge/pkg/voice/tts_service.go`: Same marker above `Synthesizer` interface
    - `bridge/pkg/voice/vad_service.go`: Same marker above `SpeechDetector` interface
    - `bridge/pkg/voice/manager.go`: Add comment noting voiceMgr = nil and voice disabled in main.go
  - Keep markers concise — one line each, no AI slop

  **Must NOT do**:
  - Do NOT delete any voice code
  - Do NOT add verbose deprecation notices
  - Do NOT modify WebRTC code

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 4, 5)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `review.md:30` area — The line claiming "Voice (WebRTC) | **Production Ready**" that must be corrected
  - `bridge/pkg/voice/stt_service.go:10` — Transcriber interface to mark
  - `bridge/pkg/voice/tts_service.go:11` — Synthesizer interface to mark
  - `bridge/pkg/voice/vad_service.go:11` — SpeechDetector interface to mark

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: review.md voice status corrected
    Tool: Bash
    Steps:
      1. grep "Production Ready" review.md | grep -i voice
      2. Verify exit code 1 (no match)
    Expected Result: Voice line no longer says "Production Ready"
    Evidence: .sisyphus/evidence/task-3-review-voice-fixed.txt

  Scenario: Voice interfaces have markers
    Tool: Bash
    Steps:
      1. grep -c "INTERFACE-ONLY" bridge/pkg/voice/stt_service.go bridge/pkg/voice/tts_service.go bridge/pkg/voice/vad_service.go
      2. Verify count >= 3 (one per file)
    Expected Result: All three service files have interface-only markers
    Evidence: .sisyphus/evidence/task-3-voice-markers.txt
  ```

  **Commit**: YES (groups with Task 2)
  - Message: `docs(voice): rewrite voice-stack.md and correct review.md status`
  - Files: `review.md`, `bridge/pkg/voice/stt_service.go`, `bridge/pkg/voice/tts_service.go`, `bridge/pkg/voice/vad_service.go`, `bridge/pkg/voice/manager.go`

- [x] 4. Update doc/agent-runtime.md with State Inference Gap

  **What to do**:
  - Add a new section "Agent State Visibility" to `doc/agent-runtime.md` documenting:
    - The 11 AgentStatus states (IDLE, INITIALIZING, BROWSING, FORM_FILLING, AWAITING_CAPTCHA, AWAITING_2FA, AWAITING_APPROVAL, PROCESSING_PAYMENT, ERROR, COMPLETE, OFFLINE)
    - How states are inferred (4-priority inference: workflow side-channel → AWAITING_APPROVAL sticky → CDP events → default)
    - The visibility gap: containers run with NetworkMode:none and CANNOT report their high-level state
    - The state inference engine is implemented but NOT wired in production (no live CDP feed from Jetski to Bridge)
    - The 8 parallel state enums across the codebase: AgentStatus, TaskStatus, BrowserStatus, JobStatus/broker, JobStatus/queue, BrowserState, InstanceStatus, WorkflowStatus
    - Known limitation: only ~8 of 11 states are reachable via current inference; container is a black box
  - Keep factual and concise — no AI slop

  **Must NOT do**:
  - Do NOT implement any state reporting mechanism
  - Do NOT consolidate state enums
  - Do NOT add verbose prose

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3, 5)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `doc/agent-runtime.md` — Existing doc (431 lines) to extend. Currently does not cover the state visibility gap.
  - `bridge/pkg/agent/state.go` — All 11 AgentStatus constants, ValidTransitions map
  - `bridge/pkg/agent/state_machine.go` — StateMachine struct with convenience methods
  - `bridge/pkg/agent/state_inference.go` — `InferAgentState()` function with 4-priority inference logic

  **API/Type References**:
  - `bridge/pkg/agent/integration.go:32` — Comment: "Container agents cannot report their state. BroadcastStatus() is not yet implemented."
  - `bridge/pkg/browser/client.go` — `ServiceState` enum (IDLE, LOADING, FILLING, WAITING, PROCESSING, ERROR)
  - `bridge/pkg/studio/browser_skill.go` — `BrowserState` enum (LOADING, FILLING, WAITING, PROCESSING, IDLE, ERROR)
  - `bridge/pkg/browser/browser.go` — `BrowserStatus` constants (idle, navigating, loading, ready, error)

  **WHY Each Reference Matters**:
  - state_inference.go has the exact inference logic — the doc should accurately describe how it works
  - integration.go has the explicit admission that containers can't report state — quote this
  - The 8 parallel enums are a maintenance hazard — documenting them makes future consolidation easier

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: agent-runtime.md documents state gap
    Tool: Bash
    Steps:
      1. grep -c "11 states\|eleven states\|inference gap\|visibility gap\|cannot report" doc/agent-runtime.md
      2. Verify count >= 3
    Expected Result: Doc mentions 11 states and the visibility/inference gap
    Evidence: .sisyphus/evidence/task-4-agent-state-gap.txt
  ```

  **Commit**: YES (groups with Tasks 5, 7, 8)
  - Message: `docs: document EventBus architecture, agent state gap, and CHANGELOG corrections`
  - Files: `doc/agent-runtime.md`

- [x] 5. Add EventBus Architecture Comments in Both Bus Packages

  **What to do**:
  - Add a top-level architecture comment to `bridge/pkg/eventbus/eventbus.go` explaining:
    - This is the "Push Bus" — pushes typed BridgeEvent structs to WebSocket clients and in-process handlers
    - Used by: Vault events, Email events (via RegisterBridgeHandler)
    - NOT used for: Matrix sync events, workflow events, agent status (those go through internal/events)
    - Key difference from internal/events: fire-and-forget delivery to WebSocket, no cursor/sequence semantics
  - Add a top-level architecture comment to `bridge/internal/events/matrix_event_bus.go` explaining:
    - This is the "Stream Bus" — high-throughput ring buffer with cursor-based polling
    - Used by: Matrix sync events, workflow events, agent status, RPC long-poll (ArmorChat streaming)
    - NOT used for: vault/email events (those go through pkg/eventbus)
    - Key difference from pkg/eventbus: durable with sequence numbers, cursor-based WaitForEvents
  - Keep comments to 5-10 lines each — concise architectural note, not a tutorial

  **Must NOT do**:
  - Do NOT rename packages, types, or methods
  - Do NOT remove RegisterBridgeHandler or its email registration
  - Do NOT remove any functional code
  - Do NOT add verbose documentation

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3, 4)
  - **Blocks**: Task 6, Task 7
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/eventbus/eventbus.go` — Add comment near top of file (after package declaration, before imports)
  - `bridge/internal/events/matrix_event_bus.go` — Add comment near top of file

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Both bus files have architecture comments
    Tool: Bash
    Steps:
      1. grep -c "Push Bus\|Stream Bus\|architecture\|NOT used for" bridge/pkg/eventbus/eventbus.go bridge/internal/events/matrix_event_bus.go
      2. Verify count >= 4 (at least 2 per file)
    Expected Result: Both files have distinguishing architecture comments
    Evidence: .sisyphus/evidence/task-5-eventbus-comments.txt
  ```

  **Commit**: YES (groups with Tasks 4, 7, 8)
  - Message: `docs: document EventBus architecture, agent state gap, and CHANGELOG corrections`
  - Files: `bridge/pkg/eventbus/eventbus.go`, `bridge/internal/events/matrix_event_bus.go`

- [x] 6. Remove Dead EventPublisher Code Path from Matrix Adapter

  **What to do**:
  - Read `bridge/internal/adapter/matrix.go` to locate the dead `EventPublisher` code
  - The `eventPublisher` field on `MatrixAdapter` struct is declared but NEVER SET — it remains nil
  - Remove:
    - The `eventPublisher` field from the `MatrixAdapter` struct
    - The `EventPublisher` interface type (if defined in this file and unused elsewhere)
    - Any methods that use `eventPublisher` (e.g., `publishEvent`, `ReceiveEvents`)
    - Dead imports introduced only for the publisher
  - Do NOT remove:
    - `RegisterBridgeHandler()` — LIVE, used by setup_email.go:60
    - Any non-publisher fields on MatrixAdapter
    - The `MatrixEventBus` integration (that's the Stream Bus, fully functional)
  - After cleanup, verify `go build ./cmd/bridge` passes

  **Must NOT do**:
  - Do NOT remove `RegisterBridgeHandler` or its email.received registration
  - Do NOT modify `MatrixEventBus` (internal/events/) — that's the live Stream Bus
  - Do NOT remove functional event handling code
  - Do NOT touch setup_email.go

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T7, T8 after T5 completes)
  - **Parallel Group**: Wave 2 (with Tasks 7, 8)
  - **Blocks**: Task 10 (Go build verification)
  - **Blocked By**: Task 5 (architecture comments should be added first)

  **References**:

  **Pattern References**:
  - `bridge/internal/adapter/matrix.go` — Contains dead `eventPublisher` field + `ReceiveEvents` method. The field is declared on MatrixAdapter but never initialized (nil forever).
  - `bridge/cmd/bridge/setup_email.go:60` — LIVE registration: `eventBus.RegisterBridgeHandler("email.received", ...)` — MUST PRESERVE

  **API/Type References**:
  - `bridge/pkg/eventbus/eventbus.go` — The `RegisterBridgeHandler` function lives here — DO NOT TOUCH
  - `bridge/internal/events/matrix_event_bus.go` — The Stream Bus (fully functional) — DO NOT TOUCH

  **WHY Each Reference Matters**:
  - The EventPublisher is the Push Bus's dead twin in the matrix adapter — it was never wired up
  - RegisterBridgeHandler is the ONLY live handler registration and must survive
  - Distinguishing dead publisher code from live event handling code is critical

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Dead EventPublisher code removed, live code preserved
    Tool: Bash
    Preconditions: Code cleanup complete
    Steps:
      1. grep -r "eventPublisher\|EventPublisher\|ReceiveEvents" bridge/internal/adapter/matrix.go
      2. Verify exit code 1 (no matches — dead code removed)
      3. grep -r "RegisterBridgeHandler" bridge/pkg/eventbus/eventbus.go
      4. Verify exit code 0 (live code preserved)
    Expected Result: Dead publisher code gone, RegisterBridgeHandler intact
    Evidence: .sisyphus/evidence/task-6-dead-publisher-removed.txt

  Scenario: Go bridge builds clean after EventPublisher removal
    Tool: Bash
    Preconditions: Cleanup complete
    Steps:
      1. cd bridge && go build ./cmd/bridge 2>&1
      2. Verify exit code 0
    Expected Result: Clean compilation
    Failure Indicators: Undefined types, missing imports
    Evidence: .sisyphus/evidence/task-6-go-build.txt
  ```

  **Commit**: YES
  - Message: `refactor(eventbus): remove dead EventPublisher code path from matrix adapter`
  - Files: `bridge/internal/adapter/matrix.go`
  - Pre-commit: `cd bridge && go build ./cmd/bridge`

- [x] 7. Update doc/communication-infra.md with Dual-Bus Architecture Clarity

  **What to do**:
  - Read `doc/communication-infra.md` and find the EventBus section
  - Update to clearly explain the dual-bus architecture:
    - **Push Bus** (`pkg/eventbus`): Fire-and-forget WebSocket delivery. Used for vault events, email events. Key method: `RegisterBridgeHandler`. No cursors, no sequencing.
    - **Stream Bus** (`internal/events`): High-throughput ring buffer with cursor-based polling. Used for Matrix sync, workflow events, agent status, RPC long-poll. Key method: `WaitForEvents`. Sequence numbers for ordering.
    - **Why two buses**: Different delivery semantics (push vs stream), different consumers (WebSocket clients vs long-poll RPC), different guarantees (at-most-once vs cursor-replayable)
  - Cross-reference the architecture comments added in Task 5
  - Keep factual, no marketing language

  **Must NOT do**:
  - Do NOT recommend consolidation (user chose documentation-only)
  - Do NOT add verbose tutorials

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 6, 8)
  - **Blocks**: None
  - **Blocked By**: Task 5 (should reference the architecture comments)

  **References**:

  **Pattern References**:
  - `doc/communication-infra.md` — Existing doc with EventBus section to update
  - `bridge/pkg/eventbus/eventbus.go` — Architecture comment added by Task 5 (reference it)
  - `bridge/internal/events/matrix_event_bus.go` — Architecture comment added by Task 5 (reference it)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: communication-infra.md explains dual bus roles
    Tool: Bash
    Steps:
      1. grep -c "Push Bus\|Stream Bus\|dual.*bus\|pkg/eventbus.*internal/events" doc/communication-infra.md
      2. Verify count >= 3
    Expected Result: Doc clearly distinguishes the two bus implementations
    Evidence: .sisyphus/evidence/task-7-dual-bus-doc.txt
  ```

  **Commit**: YES (groups with Tasks 4, 5, 8)
  - Message: `docs: document EventBus architecture, agent state gap, and CHANGELOG corrections`
  - Files: `doc/communication-infra.md`

- [x] 8. Fix CHANGELOG.md Wrong State Names

  **What to do**:
  - Read `CHANGELOG.md` and find the agent state entries with wrong names
  - The codebase defines 11 states: IDLE, INITIALIZING, BROWSING, FORM_FILLING, AWAITING_CAPTCHA, AWAITING_2FA, AWAITING_APPROVAL, PROCESSING_PAYMENT, ERROR, COMPLETE, OFFLINE
  - CHANGELOG likely lists non-existent states (e.g., "NAVIGATING", "INPUTTING", "AWAITING_INPUT") — correct these to match the actual `state.go` definitions
  - If CHANGELOG mentions "12 states", correct to "11 states"
  - Add a one-line note under the appropriate version section: "Removed vestigial Rust `blindfill/` module (superseded by Jetski CDP proxy, zero production callers)."
  - Do NOT change anything else in CHANGELOG

  **Must NOT do**:
  - Do NOT add new changelog entries (this is a correction only)
  - Do NOT modify state.go or any source code

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 6, 7)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `CHANGELOG.md` — Find entries with wrong state names
  - `bridge/pkg/agent/state.go` — Source of truth for the 11 state names

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: CHANGELOG state names match source code
    Tool: Bash
    Steps:
      1. Extract state names from state.go: grep "Status" bridge/pkg/agent/state.go | grep "=" | head -15
      2. For each state name in CHANGELOG agent-state section, verify it appears in state.go
      3. Verify CHANGELOG does not say "12 states"
    Expected Result: All state names in CHANGELOG match state.go exactly
    Evidence: .sisyphus/evidence/task-8-changelog-states.txt
  ```

  **Commit**: YES (groups with Tasks 4, 5, 7)
  - Message: `docs: document EventBus architecture, agent state gap, and CHANGELOG corrections`
  - Files: `CHANGELOG.md`

- [x] 9. Rust Vault Build + Test Verification

  **What to do**:
  - Run `cd rust-vault && cargo build` — verify clean compilation with zero errors
  - Run `cd rust-vault && cargo test` — verify ALL tests pass (lib + integration, not just --lib)
  - Grep for any remaining "blindfill" code-path references in `rust-vault/src/` (module imports, type references, function calls — NOT cosmetic metric name strings)
  - Verify `rust-vault/src/blindfill/` directory no longer exists
  - Verify error.rs has no dangling `PlaceholderParseError` import
  - Capture all output as evidence

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 10)
  - **Parallel Group**: Wave 3 (with Task 10)
  - **Blocks**: F1-F4 (Final Verification)
  - **Blocked By**: Task 1 (blindfill deletion must complete first)

  **References**:
  - `rust-vault/src/error.rs` — Verify no blindfill imports remain
  - `rust-vault/src/lib.rs` — Verify no blindfill module declaration

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Rust vault builds clean post-blindfill removal
    Tool: Bash
    Steps:
      1. cd rust-vault && cargo build 2>&1 | tee /tmp/rust-build.log
      2. Verify exit code 0
      3. grep -i "error\|blindfill" /tmp/rust-build.log | grep -v "0 errors"
      4. Verify exit code 1 (no errors or blindfill mentions)
    Expected Result: Clean build, zero errors, zero blindfill references
    Evidence: .sisyphus/evidence/task-9-rust-build.txt

  Scenario: Rust vault tests pass post-blindfill removal
    Tool: Bash
    Steps:
      1. cd rust-vault && cargo test 2>&1 | tee /tmp/rust-test.log
      2. Verify exit code 0
      3. Verify "0 failed" in output
    Expected Result: All tests pass (lib + integration), zero failures
    Evidence: .sisyphus/evidence/task-9-rust-tests.txt

  Scenario: No blindfill references in Rust source
    Tool: Bash
    Steps:
      1. grep -r "blindfill" rust-vault/src/ --include="*.rs" | grep -v "ephemeral.rs"
      2. Verify exit code 1 (no matches)
    Expected Result: Zero blindfill references in source (ephemeral.rs metrics excluded)
    Evidence: .sisyphus/evidence/task-9-no-blindfill-refs.txt
  ```

  **Commit**: NO (verification only, no code changes)

- [x] 10. Go Bridge Build Verification

  **What to do**:
  - Run `cd bridge && go build ./cmd/bridge` — verify clean compilation
  - Run `cd bridge && go vet ./pkg/eventbus/... ./internal/adapter/... ./internal/events/...` — verify no vet issues in touched packages only
  - Run `cd bridge && go test ./pkg/eventbus/... ./internal/adapter/... ./internal/events/...` — verify tests pass in touched packages
  - Verify `RegisterBridgeHandler` still exists: `grep -r "RegisterBridgeHandler" bridge/`
  - Verify `setup_email.go` still has the email.received registration
  - Verify `BlindFillStatusEmitter` is untouched: `grep -r "BlindFillStatusEmitter" bridge/`
  - Capture all output as evidence

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 9)
  - **Parallel Group**: Wave 3 (with Task 9)
  - **Blocks**: F1-F4 (Final Verification)
  - **Blocked By**: Task 6 (EventPublisher removal must complete first)

  **References**:
  - `bridge/cmd/bridge/setup_email.go:60` — Verify email handler registration intact

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Go bridge builds clean post-cleanup
    Tool: Bash
    Steps:
      1. cd bridge && go build ./cmd/bridge 2>&1 | tee /tmp/go-build.log
      2. Verify exit code 0
    Expected Result: Clean build
    Evidence: .sisyphus/evidence/task-10-go-build.txt

  Scenario: Go tests pass in touched packages
    Tool: Bash
    Steps:
      1. cd bridge && go test ./pkg/eventbus/... ./internal/adapter/... ./internal/events/... 2>&1 | tee /tmp/go-test.log
      2. Verify exit code 0
      3. Verify "FAIL" does not appear in output
    Expected Result: All tests pass in touched packages
    Failure Indicators: Test failures in eventbus, adapter, or events packages
    Evidence: .sisyphus/evidence/task-10-go-tests.txt

  Scenario: RegisterBridgeHandler preserved
    Tool: Bash
    Steps:
      1. grep -r "RegisterBridgeHandler" bridge/ --include="*.go"
      2. Verify at least 2 matches (definition in eventbus.go + usage in setup_email.go)
    Expected Result: Handler function exists and is used
    Evidence: .sisyphus/evidence/task-10-handler-preserved.txt

  Scenario: BlindFillStatusEmitter untouched
    Tool: Bash
    Steps:
      1. grep -r "BlindFillStatusEmitter" bridge/ --include="*.go"
      2. Verify at least 1 match
    Expected Result: Agent package emitter untouched
    Evidence: .sisyphus/evidence/task-10-emitter-intact.txt

  Scenario: Email handler wiring intact
    Tool: Bash
    Steps:
      1. grep -A5 "email.received" bridge/cmd/bridge/setup_email.go
      2. Verify match found and shows RegisterBridgeHandler call
      3. grep "RegisterBridgeHandler" bridge/pkg/eventbus/eventbus.go
      4. Verify function definition exists
    Expected Result: Both the handler registration and the function definition exist
    Failure Indicators: Missing registration or missing function definition
    Evidence: .sisyphus/evidence/task-10-email-wiring.txt
  ```

  **Commit**: NO (verification only, no code changes)

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE.
> After F1-F4 complete: present consolidated results to user. Work is then COMPLETE.
> If any agent REJECTs: fix issues → re-run that agent → present again → auto-complete on all-APPROVE.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `cd rust-vault && cargo build && cargo test` and `cd bridge && go build ./cmd/bridge && go vet ./pkg/eventbus/... ./internal/adapter/... ./internal/events/...`. Verify zero blindfill references in rust-vault/src/ (grep for "blindfill" excluding metrics). Verify RegisterBridgeHandler still exists. Verify no broken imports.
  Output: `Rust Build [PASS/FAIL] | Go Build [PASS/FAIL] | Blindfill refs [N] | RegisterBridgeHandler [PRESENT/MISSING] | VERDICT`

- [x] F3. **Full Automated QA Execution** — `unspecified-high`
  Execute every QA scenario from every task. Verify doc/voice-stack.md contains "interface-only" and does NOT claim providers exist. Verify review.md does NOT say "Production Ready" for voice. Verify doc/agent-runtime.md mentions "11 states" and "inference gap". Verify doc/communication-infra.md explains dual bus roles.
  Output: `Scenarios [N/N pass] | Doc Accuracy [N/N] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff. Verify 1:1 — everything in spec was done (no missing), nothing beyond spec was done (no creep). Check "Must NOT do" compliance. Flag unaccounted changes. Verify no BlindFillStatusEmitter breakage.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **T1**: `refactor(vault): remove vestigial blindfill module — superseded by Jetski CDP proxy` — rust-vault/src/blindfill/*, rust-vault/tests/blindfill_*, rust-vault/src/error.rs, rust-vault/src/lib.rs
- **T6**: `refactor(eventbus): remove dead EventPublisher code path from matrix adapter` — bridge/internal/adapter/matrix.go
- **T2+T3**: `docs(voice): rewrite voice-stack.md and correct review.md status` — doc/voice-stack.md, review.md
- **T4+T5+T7+T8**: `docs: document EventBus architecture, agent state gap, and CHANGELOG corrections` — doc/agent-runtime.md, doc/communication-infra.md, bridge/pkg/eventbus/eventbus.go, bridge/internal/events/matrix_event_bus.go, CHANGELOG.md

---

## Success Criteria

### Verification Commands
```bash
cd rust-vault && cargo build                              # Expected: clean build, no blindfill warnings
cd rust-vault && cargo test                                # Expected: all tests pass (lib + integration)
cd bridge && go build ./cmd/bridge                        # Expected: clean build
cd bridge && go vet ./pkg/eventbus/... ./internal/adapter/... ./internal/events/...  # Expected: no issues (touched packages only)
grep -r "blindfill" rust-vault/src/                       # Expected: only metric names in governance/ephemeral.rs
grep -r "RegisterBridgeHandler" bridge/                   # Expected: eventbus.go + setup_email.go (preserved)
grep "Production Ready" review.md                         # Expected: no match for voice line
grep "interface-only\|no concrete provider" doc/voice-stack.md  # Expected: at least 1 match
grep "11 states\|inference gap" doc/agent-runtime.md      # Expected: at least 1 match each
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] Rust vault builds clean
- [ ] Go bridge builds clean
- [ ] RegisterBridgeHandler preserved (email pipeline functional)
- [ ] No BlindFillStatusEmitter breakage
