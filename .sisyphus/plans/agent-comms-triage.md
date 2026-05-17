# Plan: Agent Communication Channel — Critical Finding Triage

## TL;DR

> **Quick Summary**: Fix the agent communication architecture in three gated phases: (1) Update docs to ground truth — no code changes, (2) Ship backward channel + PII socket wiring as one atomic PR, (3) Clean up dead code that silently fails.
> 
> **Deliverables**:
> - Updated `doc/armorclaw.md`, `doc/secretary-workflow.md`, `doc/agent-runtime.md` reflecting actual architecture
> - New `bridge/pkg/secretary/result.go` — StepResult type + result.json parsing
> - Modified `bridge/pkg/secretary/orchestrator_integration.go` — read result.json after container exit
> - Modified `bridge/pkg/studio/factory.go` — wire PII socket bind-mount, remove env var fallback
> - Modified `bridge/pkg/secretary/task_scheduler.go` — warmDispatch returns error
> - Modified `bridge/pkg/agent/integration.go` — BroadcastStatus returns documented error
> - Integration test: spawn container → write result.json → Bridge reads → StepResult parsed
> 
> **Estimated Effort**: Large (3 phases, ~12 tasks)
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: Phase 1 (docs) → Phase 2 (backward channel + PII) → Phase 3 (dead code cleanup)

---

## Context

### Original Request
Investigation confirmed that agent containers run with `NetworkMode: "none"` and exit-code-only feedback. Five critical issues (C1-C5) were verified in code: no backward channel, browser automation impossible in workflows, warm dispatch sends events into void, architecture diagrams describe non-existent capabilities, PII falls back to insecure env vars. User requested three-phase fix: docs first, then backward channel + PII socket as atomic pair, then dead code cleanup.

### Interview Summary
**Key Discussions**:
- User provided comprehensive impact analysis classifying all affected doc sections and code paths
- Two execution modes exist: Mode A (Agent Studio, no network) and Mode B (OpenClaw Gateway, HTTP_PROXY through Squid)
- Mode B integration is incomplete (entrypoint.ts TODO at line 185)
- PII socket helpers exist in `bridge/pkg/docker/pii_mounts.go` but are dead code — never called
- User explicitly chose "docs first to establish ground truth" before any code changes

**Research Findings**:
- `factory.go:121` — NetworkMode: "none" enforced in 4 locations
- `factory.go:123` — Only bind-mount: state dir (rw)
- `factory.go:196-206` — PII injected via env vars, not sockets
- `pii_mounts.go` — PreparePIISocketMount() exists but never called
- `task_scheduler.go` — warmDispatch() is fire-and-forget Matrix event
- `integration.go:330-338` — BroadcastStatus() stub returns nil
- `armorclaw/entrypoint.ts:185` — TODO: "Integrate with actual OpenClaw agent here"
- Image mismatch: factory.go hardcodes `armorclaw/agent-base:latest`, no Dockerfile produces it

### Metis Review
**Identified Gaps** (addressed):
- **UID 10001 permissions**: Container writes as UID 10001, Bridge reads as root. State dir is rw bind-mount. Need to verify result.json is readable by Bridge after container writes it. Added as investigation step in Phase 2 task.
- **PII socket lifecycle**: When are sockets created relative to container spawn? Current architecture spawns per-step (no pool). PII socket creation goes into `factory.go Spawn()` before container start. Added as implementation detail.
- **Dead code interface mismatches**: `pii_mounts.go` helpers may have subtle interface differences from what `factory.go` expects. Added verification step in Phase 3.

---

## Work Objectives

### Core Objective
Make the workflow execution path minimally functional and honest: docs reflect reality, backward channel enables structured results, PII delivery uses secure socket path, dead code no longer silently fails.

### Concrete Deliverables
- 3 updated doc files (armorclaw.md, secretary-workflow.md, agent-runtime.md)
- 1 new Go file (bridge/pkg/secretary/result.go)
- 4 modified Go files (orchestrator_integration.go, factory.go, task_scheduler.go, integration.go)
- 1 new integration test file

### Definition of Done
- [ ] All docs accurately describe unidirectional, exit-code-only architecture (Phase 1)
- [ ] Integration test passes: spawn → write result.json → read → parse StepResult (Phase 2)
- [ ] PII socket bind-mounted in factory.go Spawn(), env var fallback removed (Phase 2)
- [ ] warmDispatch() returns error, not silent success (Phase 3)
- [ ] BroadcastStatus() returns documented error, not nil (Phase 3)
- [ ] grep -r "TODO\|FIXME\|STUB\|returns nil" in affected packages → zero results (Phase 3)

### Must Have
- Docs separate Mode A (Agent Studio) from Mode B (OpenClaw Gateway) capabilities
- CRITICAL WARNING about exit-code-only limitation prominent in secretary-workflow.md
- result.json convention documented for container-side authors
- PII socket wired through existing pii_mounts.go helpers
- Integration test as Phase 2 gate

### Must NOT Have (Guardrails)
- Do NOT change NetworkMode — that's a security decision requiring its own threat model
- Do NOT attempt Mode A/B convergence — both modes lack backward channel, converging two broken modes produces one broken mode
- Do NOT fix image mismatch (agent-base:latest) — blocker for deployment, not for architecture
- Do NOT touch v6 microkernel code — feature-flagged off, not causing harm
- Do NOT touch sidecar/Qdrant/pdf.rs — separate subsystem, separate triage plan
- Do NOT implement WebSocket server — dependent on having events worth streaming
- Do NOT modify Rust code in Phase 2 — only Go bridge code
- Do NOT ship backward channel without PII socket wiring — must be atomic

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** - ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: YES — Go test framework in bridge/
- **Automated tests**: TDD for Phase 2 (integration test first)
- **Framework**: `go test`

### QA Policy
- **Phase 1 (docs)**: grep-based verification — heading patterns, keyword presence/absence, link validity
- **Phase 2 (code)**: `go test` + `go build` + integration test gate
- **Phase 3 (cleanup)**: `grep -r "TODO\|FIXME\|STUB\|returns nil"` verification

---

## Execution Strategy

### Parallel Execution Waves

```
Phase 1 — Ground Truth (docs only, no code):
├── T1: Update doc/armorclaw.md architecture diagrams and feature claims [writing]
├── T2: Update doc/secretary-workflow.md with CRITICAL WARNING and Mode separation [writing]
└── T3: Update doc/agent-runtime.md to reflect Bridge-side-only state machine [writing]
→ GATE: PR merged. No code PRs until docs land.

Phase 2 — The Atomic Pair (backward channel + PII socket, one PR):
├── T4: Define StepResult type and result.json convention [deep]
├── T5: Wire PII socket bind-mount into factory.go Spawn() [deep]
├── T6: Implement result.json reading in orchestrator_integration.go (depends: T4) [deep]
├── T7: Integration test: spawn → write result.json → read → parse (depends: T4, T5, T6) [deep]
→ GATE: Integration test passes. All Phase 2 tasks in one PR.

Phase 3 — Dead Code Cleanup:
├── T8: warmDispatch() returns error instead of silent success [quick]
├── T9: BroadcastStatus() returns documented error instead of nil [quick]
└── T10: Comment agent state machine transitions as Bridge-side only [quick]
→ GATE: grep returns zero TODO/FIXME/STUB/returns nil.

Wave FINAL (After ALL phases — 4 parallel reviews):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Integration test verification (unspecified-high)
└── F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| T1-T3 | None | Phase 2 gate |
| T4 | Phase 1 gate | T6, T7 |
| T5 | Phase 1 gate | T7 |
| T6 | T4 | T7 |
| T7 | T4, T5, T6 | Phase 3 gate |
| T8-T10 | Phase 2 gate | F1-F4 |
| F1-F4 | T1-T10 | User okay |

### Agent Dispatch Summary

- **Phase 1**: 3 tasks — T1-T3 → `writing`
- **Phase 2**: 4 tasks — T4-T7 → `deep`
- **Phase 3**: 3 tasks — T8-T10 → `quick`
- **FINAL**: 4 tasks — F1 → `oracle`, F2-F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] T1. Update doc/armorclaw.md — architecture diagrams and feature claims

  **What to do**:
  - Replace bidirectional arrows in architecture diagrams with accurate unidirectional flow (env vars in, exit code out)
  - Add explicit "Mode A (Agent Studio)" and "Mode B (OpenClaw Gateway)" labels to architecture sections
  - Mark "21 Browser Skills" as "Mode B only" in features table
  - Mark "Kill-on-Violation" as "post-hoc (exit code != 0)" not "reactive"
  - Fix "Communication Patterns" table: CDP WebSocket row needs "Mode B only, requires HTTP_PROXY" annotation
  - Add a new `## Agent Communication Model` section that clearly documents:
    - Mode A: `NetworkMode: "none"`, env vars in, exit code out, no backward channel (YET)
    - Mode B: `armorclaw-isolated` network, HTTP_PROXY through Squid, Bridge socket RPC
  - Add CRITICAL WARNING box near the Package Index or Executive Summary: "Agent containers in Mode A have no network access and can only report success/failure via exit code. Structured results require Phase 2 backward channel (planned)."
  - Remove or annotate aspirational language like "step-by-step progress tracking"

  **Must NOT do**:
  - Do NOT remove features from the feature list — annotate them with mode requirements
  - Do NOT change any code
  - Do NOT describe Phase 2 implementation details — just note "backward channel planned"

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T2, T3)
  - **Parallel Group**: Phase 1
  - **Blocks**: Phase 2 gate
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `doc/armorclaw.md:163-200` — System Architecture section with bidirectional diagrams
  - `doc/armorclaw.md:1488-1640` — Component Integration Patterns (Bridge ↔ Agent section)
  - `doc/armorclaw.md:1724-1768` — Browser Service section (claims CDP connectivity)

  **Source of Truth References**:
  - `bridge/pkg/studio/factory.go:121` — NetworkMode: "none"
  - `bridge/pkg/studio/factory.go:123` — Only bind-mount: state dir
  - `bridge/pkg/secretary/orchestrator_integration.go:302-328` — waitForCompletion() polls exit code
  - `container/Dockerfile.openclaw-standalone` — Mode B image definition
  - `docker-compose.bridge.yml` — Mode B network config (armorclaw-isolated + HTTP_PROXY)

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: Architecture diagrams are unidirectional
    Tool: Bash (grep)
    Steps:
      1. grep -c "◀────▶\|<-->\|bidirectional" doc/armorclaw.md
      2. Verify count is 0 or all instances are annotated as "planned/aspirational"
    Expected Result: No unqualified bidirectional arrows between Bridge and Agent
    Evidence: .sisyphus/evidence/task-t1-diagrams.txt

  Scenario: Mode A/B separation exists
    Tool: Bash (grep)
    Steps:
      1. grep -c "Mode A\|Mode B\|Agent Studio\|OpenClaw Gateway" doc/armorclaw.md
      2. Verify count >= 4
    Expected Result: Both modes mentioned at least twice each
    Evidence: .sisyphus/evidence/task-t1-mode-separation.txt

  Scenario: CRITICAL WARNING present
    Tool: Bash (grep)
    Steps:
      1. grep -c "CRITICAL.*exit-code\|exit-code-only\|no backward channel" doc/armorclaw.md
      2. Verify count >= 1
    Expected Result: At least one prominent warning about exit-code-only limitation
    Evidence: .sisyphus/evidence/task-t1-warning.txt
  ```

  **Commit**: YES
  - Message: `docs(armorclaw): update architecture to reflect actual unidirectional communication model`
  - Files: `doc/armorclaw.md`

---

- [x] T2. Update doc/secretary-workflow.md — CRITICAL WARNING and Mode separation

  **What to do**:
  - Move the "Data flow limitation" note from its current buried position to a CRITICAL WARNING box at the TOP of the document (after Overview, before Architecture)
  - Rewrite the "Two Dispatch Paths" section:
    - Warm dispatch: Mark as **NON-FUNCTIONAL**. Explain: "Sends Matrix event to agent's room. Agent has no Matrix connection (NetworkMode: none). Event is received only by ArmorChat and Bridge sync. This path silently fails."
    - Cold dispatch: Mark as functional but limited (spawn → poll → exit code only)
  - Update architecture diagram to show unidirectional flow (env vars → container → exit code)
  - Add explicit note: "`workflow.progress` events are Bridge-inferred (container still running), NOT agent-reported progress."
  - Add Mode A vs Mode B section explaining which capabilities work where
  - Add "Prerequisites for Full Functionality" section noting Phase 2 backward channel plan

  **Must NOT do**:
  - Do NOT remove the two dispatch paths — they exist in code, document them accurately
  - Do NOT describe Phase 2 implementation — just note it's planned
  - Do NOT change any code

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T1, T3)
  - **Parallel Group**: Phase 1
  - **Blocks**: Phase 2 gate
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `doc/secretary-workflow.md` — Full document (currently 415 lines)
  - `doc/armorclaw.md` — Main doc (for cross-reference consistency)

  **Source of Truth References**:
  - `bridge/pkg/secretary/task_scheduler.go` — warmDispatch() and coldDispatch() implementation
  - `bridge/pkg/secretary/orchestrator_integration.go:302-328` — waitForCompletion()
  - `bridge/pkg/studio/factory.go:121` — NetworkMode: "none"
  - `bridge/pkg/agent/integration.go:330-338` — BroadcastStatus() stub

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: CRITICAL WARNING is prominent
    Tool: Bash (grep)
    Steps:
      1. grep -n "CRITICAL" doc/secretary-workflow.md | head -5
      2. Verify first CRITICAL appears before Architecture section
    Expected Result: Warning in first 50 lines of document
    Evidence: .sisyphus/evidence/task-t2-warning-position.txt

  Scenario: Warm dispatch marked as non-functional
    Tool: Bash (grep)
    Steps:
      1. grep -A5 "warm.*dispatch\|Warm.*dispatch" doc/secretary-workflow.md | grep -c "NON-FUNCTIONAL\|non-functional\|silently fails"
      2. Verify count >= 1
    Expected Result: Warm dispatch explicitly marked as broken
    Evidence: .sisyphus/evidence/task-t2-warm-dispatch.txt

  Scenario: workflow.progress accuracy noted
    Tool: Bash (grep)
    Steps:
      1. grep -c "Bridge-inferred\|not agent-reported" doc/secretary-workflow.md
      2. Verify count >= 1
    Expected Result: Progress events explicitly noted as Bridge-inferred
    Evidence: .sisyphus/evidence/task-t2-progress-accuracy.txt
  ```

  **Commit**: YES (groups with T1, T3)
  - Files: `doc/secretary-workflow.md`

---

- [x] T3. Update doc/agent-runtime.md — reflect Bridge-side-only state machine

  **What to do**:
  - Add prominent note at top: "The agent runtime described here is Bridge-side only. No container-to-Bridge state reporting exists. The 11-state state machine is a Bridge-internal library — containers cannot report which state they are in."
  - Update "Container Lifecycle" section to distinguish:
    - What the Bridge observes: spawned → running → completed/failed (via Docker ContainerInspect)
    - What the state machine tracks: 11 granular states (Bridge-inferred, not agent-reported)
  - Update "Speculative Execution" section: Note that speculative Go-side tool calls pre-compute results, but the actual agent work happens inside a container with no network — the cache may be useful for Go-side operations but not for container-internal LLM calls
  - Update architecture diagram to show unidirectional flow
  - Add Mode A vs Mode B note in Overview

  **Must NOT do**:
  - Do NOT remove the state machine documentation — it exists in code, document it accurately
  - Do NOT describe Phase 2 implementation
  - Do NOT change any code

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T1, T2)
  - **Parallel Group**: Phase 1
  - **Blocks**: Phase 2 gate
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `doc/agent-runtime.md` — Full document (currently 264 lines)

  **Source of Truth References**:
  - `bridge/pkg/agent/state.go:13-36` — 11 AgentStatus states
  - `bridge/pkg/agent/state_machine.go` — StateMachine with subscriber channels
  - `bridge/pkg/agent/integration.go:330-338` — BroadcastStatus() stub
  - `bridge/pkg/studio/factory.go:276-296` — GetStatus() via ContainerInspect

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: Bridge-side-only note present
    Tool: Bash (grep)
    Steps:
      1. grep -c "Bridge-side only\|Bridge-internal\|no container-to-Bridge" doc/agent-runtime.md
      2. Verify count >= 1
    Expected Result: Prominent note that state machine is Bridge-side
    Evidence: .sisyphus/evidence/task-t3-bridge-side.txt

  Scenario: Unidirectional flow noted
    Tool: Bash (grep)
    Steps:
      1. grep -c "unidirectional\|exit-code-only\|no backward" doc/agent-runtime.md
      2. Verify count >= 1
    Expected Result: Communication model accurately described
    Evidence: .sisyphus/evidence/task-t3-unidirectional.txt
  ```

  **Commit**: YES (groups with T1, T2)
  - Message: `docs(armorclaw): update architecture docs to reflect actual communication model`
  - Files: `doc/agent-runtime.md`

---

- [x] T4. Define StepResult type and result.json convention

  **What to do**:
  - Create `bridge/pkg/secretary/result.go` with:
    - `StepResult` struct: Status (success/failure/partial), Output (string), Data (map[string]any), Error (string), Duration (time.Duration)
    - `ParseStepResult(stateDir string) (*StepResult, error)` — reads `result.json` from the bind-mounted state directory
    - Handle edge cases: file doesn't exist (not an error — container may not produce results), malformed JSON, permission denied
    - Define the convention: Container writes `/home/claw/.openclaw/result.json` before exit. Bridge reads from `/var/lib/armorclaw/agent-state/{definitionID}/result.json` after container exits.
  - Create `bridge/pkg/secretary/result_test.go` with tests for:
    - Happy path: valid JSON → parsed StepResult
    - File missing → nil result, no error (convention: no result is valid)
    - Malformed JSON → error
    - Extra fields → ignored (forward-compatible)
  - Document the result.json convention in a comment block

  **Must NOT do**:
  - Do NOT modify any existing files yet
  - Do NOT add result reading to orchestrator_integration.go (that's T6)
  - Do NOT change the state dir bind-mount

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T5)
  - **Parallel Group**: Phase 2 first wave
  - **Blocks**: T6, T7
  - **Blocked By**: Phase 1 gate

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/types.go` — Existing type definitions (WorkflowStep, StepType, etc.)
  - `bridge/pkg/secretary/orchestrator_integration.go` — Where result reading will be added (T6)

  **Source of Truth References**:
  - `bridge/pkg/studio/factory.go:123` — Bind-mount: state dir `/var/lib/armorclaw/agent-state/{id}` → `/home/claw/.openclaw`
  - `bridge/pkg/secretary/orchestrator_integration.go:302-328` — waitForCompletion() current implementation

  **Existing Code to Follow**:
  - `bridge/pkg/secretary/types.go` — Type definition patterns (exported struct, JSON tags)

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: StepResult type compiles and tests pass
    Tool: Bash (go test)
    Steps:
      1. cd bridge && go test -v ./pkg/secretary/... -run TestStepResult
      2. Verify all tests pass
    Expected Result: PASS with >= 4 test cases
    Evidence: .sisyphus/evidence/task-t4-tests.txt

  Scenario: Edge cases covered
    Tool: Bash (grep)
    Steps:
      1. grep -c "file not exist\|malformed\|permission" bridge/pkg/secretary/result_test.go
      2. Verify count >= 2
    Expected Result: Missing file and malformed JSON tests present
    Evidence: .sisyphus/evidence/task-t4-edge-cases.txt
  ```

  **Commit**: YES (groups with T5-T7)
  - Files: `bridge/pkg/secretary/result.go`, `bridge/pkg/secretary/result_test.go`

---

- [x] T5. Wire PII socket bind-mount into factory.go Spawn()

  **What to do**:
  - Read `bridge/pkg/docker/pii_mounts.go` thoroughly — understand PreparePIISocketMount(), PreparePIITmpfsMount(), PreparePIIMounts() signatures and return types
  - Read `bridge/pkg/secrets/pii_injection.go` — understand PIIInjector socket method
  - In `factory.go Spawn()`, AFTER the state dir bind-mount (line 123), add PII socket mount:
    - Call `PreparePIISocketMount()` or equivalent to get the mount config
    - Add the mount to `hostConfig.Mounts` or `hostConfig.Binds`
    - Ensure the socket path exists on host before container starts
  - In `factory.go Spawn()`, change PII injection from env vars to socket-based:
    - The current env var injection (lines 196-206) should be REMOVED for workflow steps
    - PII values should instead be served through the Unix socket by the PIIInjector
    - Keep env var as FALLBACK only if socket method fails (with explicit warning log)
  - Verify the UID 10001 permission model: Bridge (root) creates socket, container (10001) connects. Socket permissions must allow UID 10001 to connect.
  - Add appropriate cleanup: socket file removed after container exits

  **Must NOT do**:
  - Do NOT change NetworkMode
  - Do NOT remove the state dir bind-mount
  - Do NOT modify pii_mounts.go itself — just wire it in (fix bugs in Phase 3)
  - Do NOT change the container image or entrypoint

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T4)
  - **Parallel Group**: Phase 2 first wave
  - **Blocks**: T7
  - **Blocked By**: Phase 1 gate

  **References**:

  **Critical — Read First**:
  - `bridge/pkg/docker/pii_mounts.go` — Socket mount helpers (dead code, needs wiring)
  - `bridge/pkg/secrets/pii_injection.go` — PIIInjector with socket method

  **Modification Target**:
  - `bridge/pkg/studio/factory.go:76-210` — Spawn() method (add mount, change PII injection)

  **Source of Truth References**:
  - `bridge/pkg/studio/factory.go:121` — Current NetworkMode: "none" (DO NOT CHANGE)
  - `bridge/pkg/studio/factory.go:123` — Current bind-mount (state dir only)
  - `bridge/pkg/studio/factory.go:196-206` — Current env var PII injection (TO BE REPLACED)
  - `bridge/pkg/studio/factory.go:98-110` — Container config (UID 10001:10001)

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: PII socket mount added to Spawn()
    Tool: Bash (grep)
    Steps:
      1. grep -n "PreparePII\|piiMount\|PIISocket" bridge/pkg/studio/factory.go
      2. Verify at least one call to PII mount helper
    Expected Result: PII socket mount wired into factory.go
    Evidence: .sisyphus/evidence/task-t5-pii-mount.txt

  Scenario: Env var PII injection replaced
    Tool: Bash (grep)
    Steps:
      1. grep -n "PII_\|PII_" bridge/pkg/studio/factory.go
      2. Verify env var injection is removed or guarded by "socket unavailable" fallback
    Expected Result: No unconditional PII_ env var injection
    Evidence: .sisyphus/evidence/task-t5-env-var-removal.txt

  Scenario: Code compiles
    Tool: Bash (go build)
    Steps:
      1. cd bridge && go build ./pkg/studio/...
      2. Verify exit 0
    Expected Result: Build succeeds
    Evidence: .sisyphus/evidence/task-t5-build.txt
  ```

  **Commit**: YES (groups with T4, T6, T7)
  - Files: `bridge/pkg/studio/factory.go`

---

- [x] T6. Implement result.json reading in orchestrator_integration.go

  **What to do**:
  - Modify `waitForCompletion()` in `orchestrator_integration.go` (lines 302-328):
    - AFTER container exits (exit code 0), call `ParseStepResult(stateDir)` from T4
    - If result.json exists and parses, include StepResult in the StepExecutionResult
    - If result.json missing, StepResult is nil (container chose not to report)
    - If result.json malformed, log warning and include error in StepExecutionResult
  - Update `StepExecutionResult` type (in types.go) to include `*StepResult` field
  - Update `executeStep()` to pass state dir path to `waitForCompletion()`
  - Ensure backward compatibility: existing code that doesn't check StepResult still works

  **Must NOT do**:
  - Do NOT change the polling interval (500ms)
  - Do NOT change the container spawn logic
  - Do NOT add any network-dependent code

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Phase 2 second wave
  - **Blocks**: T7
  - **Blocked By**: T4 (needs StepResult type)

  **References**:

  **Modification Target**:
  - `bridge/pkg/secretary/orchestrator_integration.go:302-328` — waitForCompletion()
  - `bridge/pkg/secretary/types.go` — StepExecutionResult type

  **Dependency**:
  - `bridge/pkg/secretary/result.go` — StepResult type and ParseStepResult() from T4

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: Code compiles with new StepResult integration
    Tool: Bash (go build)
    Steps:
      1. cd bridge && go build ./pkg/secretary/...
      2. Verify exit 0
    Expected Result: Build succeeds
    Evidence: .sisyphus/evidence/task-t6-build.txt

  Scenario: waitForCompletion reads result.json
    Tool: Bash (grep)
    Steps:
      1. grep -n "ParseStepResult\|result.json\|StepResult" bridge/pkg/secretary/orchestrator_integration.go
      2. Verify result reading code exists
    Expected Result: ParseStepResult called after container exit
    Evidence: .sisyphus/evidence/task-t6-result-reading.txt
  ```

  **Commit**: YES (groups with T4, T5, T7)
  - Files: `bridge/pkg/secretary/orchestrator_integration.go`, `bridge/pkg/secretary/types.go`

---

- [x] T7. Integration test: spawn → write result.json → read → parse

  **What to do**:
  - Create `bridge/pkg/secretary/backward_channel_test.go` with integration test:
    - Test `TestStepResultRead`: Uses actual Docker to spawn a lightweight container that writes a result.json to the bind-mounted state dir, then verify the Bridge can read and parse it
    - If Docker is unavailable in CI, create a unit test that simulates the filesystem interaction: create a temp dir with result.json, call ParseStepResult, verify output
    - Test error paths: missing file (nil, no error), malformed JSON (error), extra fields (forward-compat)
  - Verify PII socket mount works: spawn a container with the PII mount config from T5, verify the socket path is accessible inside
  - Test UID 10001 permission: verify that a file written by UID 10001 in the state dir is readable by the Bridge process

  **Must NOT do**:
  - Do NOT require a full ArmorClaw deployment for the test
  - Do NOT test actual LLM execution — just the file convention

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Phase 2 final task
  - **Blocks**: Phase 3 gate
  - **Blocked By**: T4 (StepResult type), T5 (PII mount), T6 (result reading)

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/result_test.go` — Unit tests from T4 (extend with integration)
  - `bridge/pkg/studio/factory_test.go` — Existing factory test patterns

  **Dependency**:
  - `bridge/pkg/secretary/result.go` — ParseStepResult from T4
  - `bridge/pkg/studio/factory.go` — Spawn with PII mount from T5
  - `bridge/pkg/secretary/orchestrator_integration.go` — Result reading from T6

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: Integration test passes
    Tool: Bash (go test)
    Steps:
      1. cd bridge && go test -v ./pkg/secretary/... -run TestStepResultRead
      2. Verify PASS
    Expected Result: Test passes with meaningful assertions
    Evidence: .sisyphus/evidence/task-t7-integration-test.txt

  Scenario: Error paths tested
    Tool: Bash (grep)
    Steps:
      1. grep -c "malformed\|missing\|permission" bridge/pkg/secretary/backward_channel_test.go
      2. Verify count >= 2
    Expected Result: Error paths explicitly tested
    Evidence: .sisyphus/evidence/task-t7-error-paths.txt
  ```

  **Commit**: YES (groups with T4-T6)
  - Message: `feat(secretary): implement backward channel via state dir + wire PII socket mount`
  - Files: `bridge/pkg/secretary/backward_channel_test.go`

---

- [x] T8. warmDispatch() returns error instead of silent success

  **What to do**:
  - In `bridge/pkg/secretary/task_scheduler.go`, modify `warmDispatch()`:
    - Keep the function signature
    - Remove the Matrix event send
    - Return an error: `fmt.Errorf("warm dispatch requires backward channel — not implemented: agent containers have no Matrix connectivity (NetworkMode: none)")`
  - Update `dispatchTask()` to handle the error from warmDispatch() — log it and potentially fall back to coldDispatch()

  **Must NOT do**:
  - Do NOT remove the function entirely — callers exist
  - Do NOT change the cold dispatch path

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T9, T10)
  - **Parallel Group**: Phase 3
  - **Blocks**: F1-F4
  - **Blocked By**: Phase 2 gate

  **References**:

  **Modification Target**:
  - `bridge/pkg/secretary/task_scheduler.go` — warmDispatch() function

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: warmDispatch returns error
    Tool: Bash (grep)
    Steps:
      1. grep -A3 "func.*warmDispatch" bridge/pkg/secretary/task_scheduler.go | grep -c "error\|Errorf"
      2. Verify error return present
    Expected Result: Function returns explicit error
    Evidence: .sisyphus/evidence/task-t8-warm-dispatch.txt

  Scenario: Code compiles
    Tool: Bash (go build)
    Steps:
      1. cd bridge && go build ./pkg/secretary/...
    Expected Result: Build succeeds
    Evidence: .sisyphus/evidence/task-t8-build.txt
  ```

  **Commit**: YES (groups with T9, T10)
  - Files: `bridge/pkg/secretary/task_scheduler.go`

---

- [x] T9. BroadcastStatus() returns documented error instead of nil

  **What to do**:
  - In `bridge/pkg/agent/integration.go`, modify `BroadcastStatus()`:
    - Remove the stub that returns nil
    - Return an error: `fmt.Errorf("BroadcastStatus: agent status broadcasting not implemented — no container-to-Bridge state reporting channel exists")`
    - Keep the TODO comment but convert it to a documented limitation
  - Check all callers of BroadcastStatus() — ensure they handle the error appropriately (don't crash)

  **Must NOT do**:
  - Do NOT remove the function entirely
  - Do NOT implement actual broadcasting (that requires Phase 2 backward channel + additional work)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T8, T10)
  - **Parallel Group**: Phase 3
  - **Blocks**: F1-F4
  - **Blocked By**: Phase 2 gate

  **References**:

  **Modification Target**:
  - `bridge/pkg/agent/integration.go:330-338` — BroadcastStatus() stub

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: BroadcastStatus returns error
    Tool: Bash (grep)
    Steps:
      1. grep -A5 "func.*BroadcastStatus" bridge/pkg/agent/integration.go | grep -c "error\|Errorf"
      2. Verify error return present
    Expected Result: Function returns explicit error, not nil
    Evidence: .sisyphus/evidence/task-t9-broadcast-status.txt

  Scenario: Callers handle error
    Tool: Bash (grep)
    Steps:
      1. grep -rn "BroadcastStatus" bridge/pkg/
      2. Verify all callers either log the error or handle it gracefully
    Expected Result: No callers will crash on non-nil error
    Evidence: .sisyphus/evidence/task-t9-callers.txt
  ```

  **Commit**: YES (groups with T8, T10)
  - Files: `bridge/pkg/agent/integration.go`

---

- [x] T10. Comment agent state machine transitions as Bridge-side only

  **What to do**:
  - In `bridge/pkg/agent/state.go`, add a package-level comment or block comment at the top:
    ```
    // NOTE: The state machine defined here is a Bridge-side library.
    // Container-to-Bridge state reporting does not exist.
    // States advance based on container lifecycle events (spawn, poll exit code),
    // not agent-reported phase transitions.
    // The 11 states (IDLE through OFFLINE) are defined for future use when
    // a backward communication channel is implemented.
    ```
  - In `bridge/pkg/agent/integration.go`, add a comment at the top:
    ```
    // NOTE: Integration struct wires StateMachine + HITLConsentManager.
    // The StateMachine runs in the Bridge process. Container agents cannot
    // report their state. BroadcastStatus() is not yet implemented.
    ```

  **Must NOT do**:
  - Do NOT change any function signatures
  - Do NOT change any state transition logic
  - Do NOT remove any states

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T8, T9)
  - **Parallel Group**: Phase 3
  - **Blocks**: F1-F4
  - **Blocked By**: Phase 2 gate

  **References**:

  **Modification Target**:
  - `bridge/pkg/agent/state.go:1-15` — Package/type comments
  - `bridge/pkg/agent/integration.go:1-30` — Package/type comments

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: Bridge-side-only comment present
    Tool: Bash (grep)
    Steps:
      1. grep -c "Bridge-side\|no container-to-Bridge\|backward channel" bridge/pkg/agent/state.go bridge/pkg/agent/integration.go
      2. Verify count >= 1 in each file
    Expected Result: Clear documentation that state machine is Bridge-side only
    Evidence: .sisyphus/evidence/task-t10-comments.txt
  ```

  **Commit**: YES (groups with T8, T9)
  - Message: `fix(secretary): convert silent failures to explicit errors in warmDispatch and BroadcastStatus`
  - Files: `bridge/pkg/agent/state.go`, `bridge/pkg/agent/integration.go`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go build ./...` + `go vet ./...` + `go test ./...`. Review all changed files for: correctness, error handling, edge cases. Verify PII socket lifecycle is correct. Verify result.json parsing handles malformed/missing files. Check for race conditions in concurrent container management.
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | VERDICT`

- [x] F3. **Integration Test Verification** — `unspecified-high`
  Run the Phase 2 integration test explicitly. Verify it actually spawns a container, writes a result, and reads it back. Check that the test exercises error paths (missing result.json, malformed JSON, permission denied). Verify PII socket bind-mount works in the test.
  Output: `Integration Test [PASS/FAIL] | Error Paths [N/N] | PII Socket [PASS/FAIL] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built, nothing beyond spec was built. Check "Must NOT do" compliance. Verify Phase 1 docs are accurate (no aspirational language). Verify Phase 2 changes are atomic (backward channel + PII together). Verify Phase 3 grep gate passed.
  Output: `Tasks [N/N compliant] | Phase Gates [N/N passed] | VERDICT`

---

## Commit Strategy

- **Phase 1**: `docs(armorclaw): update architecture docs to reflect actual communication model` — doc/armorclaw.md, doc/secretary-workflow.md, doc/agent-runtime.md
- **Phase 2**: `feat(secretary): implement backward channel via state dir + wire PII socket mount` — bridge/pkg/secretary/result.go, bridge/pkg/secretary/orchestrator_integration.go, bridge/pkg/studio/factory.go, bridge/pkg/secretary/result_test.go
- **Phase 3**: `fix(secretary): convert silent failures to explicit errors in warmDispatch and BroadcastStatus` — bridge/pkg/secretary/task_scheduler.go, bridge/pkg/agent/integration.go, bridge/pkg/agent/state.go

---

## Success Criteria

### Verification Commands
```bash
# Phase 1: Docs reflect reality
grep -c "Mode A\|Mode B\|Agent Studio\|OpenClaw Gateway" doc/armorclaw.md  # Expected: >= 4
grep -c "CRITICAL.*exit-code-only\|exit-code-only" doc/secretary-workflow.md  # Expected: >= 1
grep -c "Bridge-side only\|unidirectional" doc/agent-runtime.md  # Expected: >= 1

# Phase 2: Backward channel works
cd bridge && go test -v ./pkg/secretary/... -run TestStepResultRead  # Expected: PASS
cd bridge && go build ./...  # Expected: success

# Phase 3: Dead code cleaned
grep -rn "returns nil" bridge/pkg/secretary/task_scheduler.go bridge/pkg/agent/integration.go  # Expected: 0 matches

# All phases
cd bridge && go test ./...  # Expected: all pass
```

### Final Checklist
- [ ] All docs accurately describe actual architecture (no aspirational diagrams)
- [ ] Mode A and Mode B capabilities explicitly separated
- [ ] Integration test passes for backward channel
- [ ] PII socket bind-mounted, env var fallback removed
- [ ] warmDispatch returns error, not nil
- [ ] BroadcastStatus returns error, not nil
- [ ] State machine transitions documented as Bridge-side only
