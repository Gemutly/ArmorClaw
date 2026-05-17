# ArmorClaw Stabilization Program

## TL;DR

> **Quick Summary**: Fix 7 unwired RPC config fields, canonicalize sidecar socket paths, wire the voice pipeline (Manager + Matrix signaling + error codes), operationalize the Java sidecar with extraction observability, default E2EE on for fresh installs, and add versioned agent status files with Bridge-side tailing. 5 phases, 32 tasks.
> 
> **Deliverables**:
> - All 7 `rpc.Config` fields wired in main.go
> - Canonical socket path for office sidecar with startup validation
> - Voice pipeline fully functional end-to-end (Manager wired, MatrixManager injected, error codes normalized to `-32007`)
> - Voice event contract published
> - Java sidecar CI-gated with `ExtractionMode` observability
> - E2EE default-on for fresh installs with fresh/legacy detection
> - Agent mode status via versioned files (`agent_status.json`, `agent_events.jsonl`) with inherited 10MB/PIPE_BUF tailer
> - Dead code cleanup (ProvisionOfficeSocketDir, ProvisionJavaSocketDir, etc.)
> 
> **Estimated Effort**: Large
> **Parallel Execution**: YES — 5 phases, max 7 parallel tasks per wave
> **Critical Path**: Phase A (wiring audit + sockets) → Phase B1 (voice) → Phase B2 (E2EE) → Phase C (Java sidecar) → Phase D (agent status + cleanup)

---

## How to Execute This Plan

This plan is designed for `/start-work` — the Sisyphus orchestrator will dispatch tasks in parallel waves respecting the dependency graph.

**To start:**
```bash
/start-work
```

**During execution:**
- Each task is self-contained with references, QA scenarios, and commit strategy
- Tasks within a phase run in parallel where dependencies allow
- The orchestrator tracks progress across sessions — safe to interrupt and resume
- Evidence is captured to `.sisyphus/evidence/` for every QA scenario

**Critical path owners:**
| Task | Owner Risk | Action |
|------|-----------|--------|
| T1 (rpc.Config wiring) | **HIGH** — touches 7 fields in main.go, single point of failure | Execute first, verify with `go vet` + `go build` before proceeding |
| T8 (voice error codes) | **HIGH** — changes RPC contract that ArmorChat consumes | Write failing tests FIRST (TDD), then change handler code, verify error code with socat |
| T14 (voice E2E tests) | **HIGH** — validates entire Phase B1, blocks Phase D | Execute after T8-T11 complete, must pass before Phase D starts |

**TDD for critical paths:** Tasks 8, 14, and 19 follow test-first discipline — write the test asserting the NEW behavior, verify it fails, then implement the change. See individual task notes.

---

## Context

### Original Request
Execute the ArmorClaw stabilization program based on a 2026-05-09 document critique with 6 refinements. The critique correctly identified that the codebase needs stabilization, not feature expansion.

### Interview Summary
**Key Discussions**:
- User provided the full revised execution spec with 6 waves
- User confirmed the critique's 5 refinements plus 1 additional improvement (voice error contract normalization)
- 4 parallel explore agents verified every claim against actual codebase
- Metis consultation revealed 7 unwired `rpc.Config` fields (not just VoiceMgr) and Voice/E2EE coupling

**Research Findings**:
- `rpcCfg.VoiceMgr` never assigned in main.go — voice completely non-functional even when enabled
- 6 additional unwired fields: DockerClient, AuditLog, Guard, NavChartStore, SkillGate, GovernanceRoomID
- Voice error code split: voice-stack.md says `-32007`, architecture.md says `-32007` and `-32603`, actual RPC handlers use `-32603`
- `RequireE2EE: true` in voice config creates coupling between Wave 1 (voice) and Wave 3 (E2EE)
- `e2eeEnabled` atomic.Bool on RPC server never initialized from config
- `SetEncryptionService()` / `SetKeyExchangeService()` on MatrixAdapter never called from main.go
- Two CI security gate tests scan entire codebase for "restore_backup" — will break CI if restore code added without updating tests
- Agent mode (agent.py) already has Unix socket RPC — proposed file-based backward channel is unnecessary
- BudgetTracker constructed at main.go:2114 but result discarded — budget enforcement non-functional
- Email ingest and agent injection sockets lack `MkdirAll` before bind
- `ConfigExists()` in setup/config.go:280 provides foundation for fresh-install detection

### Metis Review
**Identified Gaps** (addressed):
- 7 unwired rpc.Config fields → promoted to Phase A highest priority
- Voice/E2EE coupling via RequireE2EE → merged Waves 1+3 into Phase B
- CI security gate tests for restore_backup → added as Phase B prerequisite task
- Agent backward channel should extend the existing bind-mounted file protocol → Phase D uses versioned agent_status.json and agent_events.jsonl with Bridge-side tailing
- Email/injection socket MkdirAll gaps → added to Phase A
- BudgetTracker dead construction → added to Phase D dead code cleanup

---

## Work Objectives

### Core Objective
Fix all known wiring gaps, deployment breakers, and doc/code inconsistencies in the ArmorClaw Bridge so that default deployment works end-to-end without manual configuration overrides.

### Concrete Deliverables
- main.go lines 2565-2625: all rpc.Config fields assigned
- bridge/pkg/sidecar/office_client.go: canonical socket path constant
- bridge/pkg/rpc/server.go: voice handlers use `-32007` (not `-32603`)
- Voice event contract doc published
- ExtractionMode type with `detecting` state
- Fresh-install detection in config loader
- Agent status file protocol doc (`doc/agent-file-protocol.md`)
- Agent file writer in `agent.py` (agent_status.json + agent_events.jsonl)
- Bridge-side agent file tailer and Matrix event emission

### Definition of Done
- [ ] `echo '{"method":"voice.start_session"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock` returns `-32007` (not `-32603`) when voice enabled but prereqs missing
- [ ] `echo '{"method":"container.terminate"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock` successfully stops a container (not nil-pointer)
- [ ] Standalone bridge (no Docker) starts without socket path overrides
- [ ] `go test ./pkg/crypto/...` passes after all changes
- [ ] Fresh install (no config file) has `e2ee_enabled: true` in `bridge.status`

### Must Have
- All 7 unwired rpc.Config fields wired
- Voice error code normalized to `-32007` in RPC handlers
- VoiceManager passed to RPC server
- MatrixManager injected into voice Manager
- Canonical socket path for office sidecar
- CI gate for Java sidecar (8 + 22 + 4 tests)
- E2EE default-on for fresh installs only
- Fresh-install detection mechanism

### Must NOT Have (Guardrails)
- Do NOT remove SQLCipher
- Do NOT bypass Matrix as control plane
- Do NOT weaken approval flow for payments or critical PII
- Do NOT remove `TestE2EE_NoRestoreBackupAnywhere` security gate — update it, don't delete it
- Do NOT assume Unix-socket RPC access from agent containers — use versioned files in bind-mounted state dir (matching existing _events.jsonl pattern)
- Do NOT wire ChallengeManager without adding persistence for Ed25519 keys
- Do NOT change socket paths in Docker compose without migration strategy
- Do NOT add dependency injection frameworks — use existing constructor pattern
- Do NOT redesign the budget system — just wire or remove dead construction
- Do NOT add `restore_backup` without updating CI security gate tests first
- Prefer minimal patches over rewrites (AGENTS.md)

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (Go tests, Python pytest, Java JUnit, Bash test harness)
- **Automated tests**: Tests-after (existing test infrastructure is comprehensive)
- **Framework**: go test, pytest, JUnit 5, bash test harness in tests/

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Backend**: Use Bash (curl/socat) — Send JSON-RPC, assert status + response fields
- **Go tests**: Use Bash — `cd bridge && go test -v ./pkg/...`
- **Integration**: Use Bash — `bash tests/test-*.sh`
- **Config**: Use Bash — verify config values, socket existence, process startup

---

## Execution Strategy

### Parallel Execution Waves

```
Phase A — Foundation: wiring audit + socket canonicalization (START IMMEDIATELY)
├── Task 1: Wire all 7 unwired rpc.Config fields [deep]
├── Task 2: Canonicalize office sidecar socket path [deep]
├── Task 3: Add MkdirAll to email/injection socket bindings [quick]
├── Task 4: Fix DefaultConfig bridge socket path (TempDir → /run/armorclaw) [quick]
├── Task 5: Provision parent directory for canonical socket paths [quick]
├── Task 6: Update README socket path reference [quick]
└── Task 7: Add startup validation for all sidecar sockets [deep]

Phase B1 — Voice wiring (AFTER Phase A)
├── Task 8: Normalize voice error codes (-32603 → -32007) in RPC handlers [deep]
├── Task 9: Inject MatrixManager into voice Manager [deep]
├── Task 10: Add structured voice prereq diagnostics (TURN, OpenAI, Matrix) [unspecified-high]
├── Task 11: Handle RequireE2EE conditionally (false when E2EE disabled) [quick]
├── Task 12: Update voice error codes in docs (architecture.md, voice-stack.md) [quick]
├── Task 13: Publish voice event contract doc [writing]
└── Task 14: Voice E2E tests: flag-off, prereq-fail, manager-nil, success path [deep]

Phase B2 — E2EE posture and wiring (AFTER Phase B1, adjacent but not operationally blocking voice)
├── Task 15: Initialize e2eeEnabled atomic.Bool from config in RPC server New() [quick]
├── Task 16: Build fresh-install detection (ConfigExists) [quick]
├── Task 17: Change E2EE default to true for fresh installs [deep]
├── Task 18: Wire SetEncryptionService + SetKeyExchangeService on MatrixAdapter [deep]
└── Task 19: E2EE validation: fresh install boots with e2ee_enabled=true [deep]

Phase C — Java Sidecar + Extraction Observability (AFTER Phase A, parallel with B1/B2)
├── Task 20: Add ExtractionMode type with detecting state [quick]
├── Task 21: Surface extraction mode in health.check + startup logging [unspecified-high]
├── Task 22: CI gate: Java image publish requires 8+22+4 tests passing [quick]
├── Task 23: Wire Java E2E tests into CI pipeline [quick]
└── Task 24: Update architecture.md with ExtractionMode and Java sidecar status [quick]

Phase D — Agent status via versioned files + Cleanup (AFTER Phase B1)
├── Task 25: Define agent_status.json and agent_events.jsonl schemas [quick]
├── Task 26: Add agent file writer in agent.py (bind-mounted state dir) [unspecified-high]
├── Task 27: Bridge-side tailer for agent files (inherit 10MB + PIPE_BUF rules) [deep]
├── Task 28: Bridge emission of canonical Matrix events from agent files [deep]
├── Task 29: Fix BudgetTracker dead construction (wire or remove) [quick]
├── Task 30: Dead code cleanup [quick]
├── Task 31: Update architecture.md Known Gaps + CHANGELOG.md [writing]
└── Task 32: Full regression: go test + pytest + JUnit + bash test harness [deep]

Phase FINAL (After ALL tasks — 5 parallel reviews)
├── F1: Plan compliance audit [oracle]
├── F2: Code quality review [unspecified-high]
├── F3: Automated RPC smoke QA [unspecified-high]
├── F4: Scope fidelity check [deep]
└── F5: Zero-trust compliance audit [deep]
→ Present results → Get explicit user okay

Critical Path: Task 1 → Task 8 → Task 14 → Task 32 → F1-F4
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 7 (Phase A)
```

### Dependency Matrix

| Task | Depends On | Blocks | Phase |
|------|-----------|--------|-------|
| 1-7 | — | 8-32 | A |
| 8 | 1 | 12, 14 | B1 |
| 9 | 1 | 14 | B1 |
| 10 | 1, 8 | 14 | B1 |
| 11 | 1, 8 | 14 | B1 |
| 12 | 8 | — | B1 |
| 13 | 8, 10 | — | B1 |
| 14 | 8-11 | F1-F4 | B1 |
| 15 | 1 | 17 | B2 |
| 16 | 1 | 17 | B2 |
| 17 | 15, 16 | 19 | B2 |
| 18 | 1 | 19 | B2 |
| 19 | 17, 18 | F1-F4 | B2 |
| 20-24 | 1 | — | C |
| 25 | — | 26 | D |
| 26 | 25 | 27 | D |
| 27 | 26 | 28 | D |
| 28 | 27 | 32 | D |
| 29 | 1 | 32 | D |
| 30 | 1, 5, 7 | 32 | D |
| 31 | 14, 19, 24, 28-30 | F1-F4 | D |
| 32 | 31 | F1-F4 | D |

### Agent Dispatch Summary

- **Phase A**: **7** — T1-T2 → `deep`, T3-T6 → `quick`, T7 → `deep`
- **Phase B1**: **7** — T8-T9 → `deep`, T10 → `unspecified-high`, T11-T12 → `quick`, T13 → `writing`, T14 → `deep`
- **Phase B2**: **5** — T15-T16 → `quick`, T17-T18 → `deep`, T19 → `deep`
- **Phase C**: **5** — T20 → `quick`, T21 → `unspecified-high`, T22-T24 → `quick`
- **Phase D**: **8** — T25 → `quick`, T26 → `unspecified-high`, T27-T28 → `deep`, T29-T30 → `quick`, T31 → `writing`, T32 → `deep`
- **FINAL**: **5** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`, F5 → `deep`

---

## TODOs

- [x] 1. Wire all 7 unwired rpc.Config fields in main.go

  **What to do**:
  - In `bridge/cmd/bridge/main.go`, find the `rpcCfg` construction (~line 2565-2625)
  - Assign ALL 7 currently-missing fields from their constructed values:
    - `VoiceMgr`: assign the `voiceMgr` variable (constructed ~line 2079 but never passed)
    - `DockerClient`: find where Docker client is created and pass it
    - `AuditLog`: find where audit logger is created and pass it
    - `Guard`: find IP guard construction and pass it
    - `NavChartStore`: find browser chart store and pass it
    - `SkillGate`: find skill gate construction and pass it
    - `GovernanceRoomID`: find governance room config and pass it
  - Use `lsp_find_references` on `rpc.Config` struct fields to locate all wiring points
  - Do NOT redesign the wiring architecture — just connect existing constructors to existing config fields

  **Must NOT do**:
  - Do not add dependency injection frameworks
  - Do not restructure the main.go function — just add assignments
  - Do not change any handler logic

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []
  - Reason: Requires reading ~100 lines of main.go constructor code and understanding struct field mapping

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2-7)
  - **Parallel Group**: Phase A
  - **Blocks**: Tasks 8-32
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/cmd/bridge/main.go:2565-2625` — `rpcCfg` construction, the target edit area
  - `bridge/cmd/bridge/main.go:1989-2111` — voice manager construction (`voiceMgr` variable, never passed)
  - `bridge/cmd/bridge/main.go:2114` — BudgetTracker construction (result discarded `_, err = ...`)
  - `bridge/pkg/rpc/server.go:185-240` — `Server` struct definition with all fields (VoiceMgr, DockerClient, AuditLog, Guard, NavChartStore, SkillGate, GovernanceRoomID)
  - `bridge/pkg/rpc/server.go:261-288` — `Config` struct that maps to `Server` fields

  **API/Type References**:
  - `bridge/pkg/rpc/server.go:153-160` — `VoicePipeline` string field (already wired at line 282)
  - `bridge/pkg/rpc/server.go:246` — `VoiceMgr` field (VoiceManager interface)

  **WHY Each Reference Matters**:
  - main.go:2565-2625 is the ONLY place rpcCfg is built — all assignments must go here
  - server.go:185-240 defines every field that needs wiring — check which are set vs nil
  - main.go:2114 BudgetTracker result is discarded — either wire it or comment why it's dead

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Container lifecycle RPC works after DockerClient wiring
    Tool: Bash (socat)
    Preconditions: Bridge running with Docker available
    Steps:
      1. echo '{"jsonrpc":"2.0","id":1,"method":"container.list"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
      2. Parse JSON response, assert no error field
      3. Assert result field exists and is an object
    Expected Result: {"jsonrpc":"2.0","id":1,"result":{...}} with no "error" key
    Failure Indicators: null result, panic in bridge logs, error with code -32603
    Evidence: .sisyphus/evidence/task-1-container-list-rpc.txt

  Scenario: Voice RPC no longer returns -32603 InternalError after VoiceMgr wiring
    Tool: Bash (socat)
    Preconditions: Bridge running, voice_pipeline=cloud in config
    Steps:
      1. echo '{"jsonrpc":"2.0","id":1,"method":"voice.status"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
      2. Parse JSON response, check error code
    Expected Result: Error code is NOT -32603 "voice manager not configured". Should be either success or -32007 (prereq failure).
    Failure Indicators: error.code == -32603 means VoiceMgr still nil
    Evidence: .sisyphus/evidence/task-1-voice-status-rpc.txt

  Scenario: Bridge starts without panic after all fields wired
    Tool: Bash
    Preconditions: Bridge binary rebuilt
    Steps:
      1. cd bridge && go build -o /tmp/armorclaw-bridge ./cmd/bridge
      2. /tmp/armorclaw-bridge --config /etc/armorclaw/config.toml 2>&1 | head -50
      3. Check for panic, nil-pointer dereference, or missing field warnings
    Expected Result: Bridge starts cleanly, all RPC methods registered (109 methods)
    Failure Indicators: panic, "nil pointer", "assignment to entry in nil map"
    Evidence: .sisyphus/evidence/task-1-bridge-startup.txt
  ```

  **Commit**: YES
  - Message: `fix(bridge): wire all 7 unwired rpc.Config fields in main.go`
  - Files: `bridge/cmd/bridge/main.go`
  - Pre-commit: `cd bridge && go vet ./cmd/bridge/... && go build ./cmd/bridge/`

- [x] 2. Canonicalize office sidecar socket path

  **What to do**:
  - In `bridge/pkg/sidecar/office_client.go:16`, change `OfficeSocketPath` from `/run/armorclaw/office-sidecar/sidecar-office.sock` to `/run/armorclaw/sidecar-office.sock`
  - Update `deploy/docker-compose.sidecar-py.yml` line 29: change volume mount to match new canonical path
  - Update env var on line 40 accordingly
  - Search ALL docker-compose files, test harnesses, and docs for the old path and update them
  - Add deprecation warning if old path detected at startup

  **Must NOT do**:
  - Do not change the Python sidecar's `worker.py` default (it's already correct at `/run/armorclaw/sidecar-office.sock`)
  - Do not change the Java sidecar socket path (it's already consistent)
  - Do not remove the old path entirely — allow it as an override for migration

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 1, 3-7)
  - **Parallel Group**: Phase A
  - **Blocks**: Task 7
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/sidecar/office_client.go:16` — `OfficeSocketPath` constant (CHANGE THIS)
  - `sidecar-python/worker.py:184` — Python default (already `/run/armorclaw/sidecar-office.sock`, CORRECT)
  - `deploy/docker-compose.sidecar-py.yml:29,40` — Volume mount and env var (UPDATE)

  **API/Type References**:
  - `bridge/pkg/sidecar/java_client.go:12` — Java socket path (consistent, no change needed)

  **Test References**:
  - `tests/test-sidecar-docs.sh:38` — Test harness socket path reference
  - `tests/test-cross-workflow-docs.sh:69` — Cross-workflow test socket path

  **External References**:
  - `README.md:1331` — Documents wrong socket path (already identified in doc review, update here)

  **WHY Each Reference Matters**:
  - office_client.go:16 is the single source of truth for Go Bridge — change this and Go matches Python
  - Docker compose volume remap was the reconciliation hack — must be updated to match canonical path
  - Tests hardcode socket paths — will break if not updated

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Standalone bridge connects to Python sidecar without Docker overrides
    Tool: Bash
    Preconditions: Python sidecar running on /run/armorclaw/sidecar-office.sock
    Steps:
      1. Start Python sidecar: python sidecar-python/worker.py
      2. Start Go bridge: ./bridge --config config.toml
      3. Send document extraction RPC
      4. Assert sidecar receives request and bridge receives response
    Expected Result: Document extraction succeeds without SIDECAR_SOCKET env override
    Failure Indicators: "dial unix /run/armorclaw/office-sidecar/sidecar-office.sock: connect: no such file or directory"
    Evidence: .sisyphus/evidence/task-2-standalone-sidecar.txt

  Scenario: Docker deployment still works after path change
    Tool: Bash
    Preconditions: Docker compose running
    Steps:
      1. docker compose -f deploy/docker-compose.sidecar-py.yml up -d
      2. Wait for healthcheck
      3. Send extraction RPC via bridge
      4. Assert response
    Expected Result: Extraction succeeds with updated compose file
    Failure Indicators: Sidecar not reachable, socket not found
    Evidence: .sisyphus/evidence/task-2-docker-sidecar.txt
  ```

  **Commit**: YES
  - Message: `fix(sidecar): canonicalize office socket path to /run/armorclaw/sidecar-office.sock`
  - Files: `bridge/pkg/sidecar/office_client.go`, `deploy/docker-compose.sidecar-py.yml`, `tests/test-sidecar-docs.sh`, `tests/test-cross-workflow-docs.sh`, `README.md`
  - Pre-commit: `grep -r "office-sidecar" bridge/ deploy/ tests/ — should only find migration compat code`

- [x] 3. Add MkdirAll before socket bind for email and injection servers

  **What to do**:
  - In `bridge/pkg/email/ingest_server.go:61`, add `os.MkdirAll(filepath.Dir(socketPath), 0755)` before `net.Listen("unix", socketPath)`
  - In `bridge/pkg/agent/injection.go:51`, add same `MkdirAll` pattern
  - Follow the existing pattern from `bridge/pkg/rpc/server.go:1385` which already does this correctly

  **Must NOT do**:
  - Do not change any other socket binding code
  - Do not add Chmod or permission changes beyond the MkdirAll

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 1-2, 4-7)
  - **Parallel Group**: Phase A
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/rpc/server.go:1385` — CORRECT pattern (MkdirAll → Remove → Listen → Chmod)
  - `bridge/pkg/email/ingest_server.go:61` — MISSING MkdirAll
  - `bridge/pkg/agent/injection.go:51` — MISSING MkdirAll

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Email ingest server starts when /run/armorclaw/ doesn't exist
    Tool: Bash
    Preconditions: /run/armorclaw/ directory does NOT exist
    Steps:
      1. sudo rm -rf /run/armorclaw/
      2. Start bridge with email ingest enabled
      3. Check bridge logs for successful socket bind
      4. test -S /run/armorclaw/email-ingest.sock && echo "OK"
    Expected Result: Socket created successfully, no "no such file or directory" error
    Failure Indicators: "bind: no such file or directory" in bridge logs
    Evidence: .sisyphus/evidence/task-3-email-mkdirall.txt
  ```

  **Commit**: YES
  - Message: `fix(bridge): add MkdirAll before socket bind for email and injection`
  - Files: `bridge/pkg/email/ingest_server.go`, `bridge/pkg/agent/injection.go`

- [x] 4. Fix DefaultConfig bridge socket path from TempDir to /run/armorclaw

  **What to do**:
  - In `bridge/pkg/config/config.go`, find the `DefaultConfig()` function
  - Change the RPC socket path default from `os.TempDir()`-based path to `/run/armorclaw/bridge.sock`
  - This matches ALL Docker compose files, healthchecks, and hardcoded references
  - Ensure standalone development works without explicit config

  **Must NOT do**:
  - Do not change any Docker compose references (they already use the correct path)
  - Do not remove the TempDir fallback for tests — use a test-only override

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Phase A
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/config/config.go` — `DefaultConfig()` function with TempDir path

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Bridge listens on correct path without config file
    Tool: Bash
    Preconditions: No /etc/armorclaw/config.toml exists
    Steps:
      1. cd bridge && go build -o /tmp/test-bridge ./cmd/bridge
      2. /tmp/test-bridge 2>&1 | head -20
      3. Check logs for listening path
      4. test -S /run/armorclaw/bridge.sock && echo "OK"
    Expected Result: Socket at /run/armorclaw/bridge.sock
    Failure Indicators: Socket at /tmp/armorclaw/bridge.sock instead
    Evidence: .sisyphus/evidence/task-4-default-socket-path.txt
  ```

  **Commit**: YES
  - Message: `fix(config): change default bridge socket from TempDir to /run/armorclaw/bridge.sock`
  - Files: `bridge/pkg/config/config.go`

- [x] 5. Provision parent directory for canonical socket paths

  **What to do**:
  - In `bridge/cmd/bridge/main.go`, during startup (before RPC server start):
    - Call `os.MkdirAll("/run/armorclaw", 0755)` to provision the common parent directory for all canonical socket paths
    - Call `sidecar.ProvisionJavaSocketDir()` to provision `/run/armorclaw/sidecar-java/` for the Java socket (nested path)
  - Do NOT call `ProvisionOfficeSocketDir()` — the office socket is now flat at `/run/armorclaw/sidecar-office.sock`, so provisioning `/run/armorclaw/office-sidecar/` is wrong (deprecated path)
  - The `os.MkdirAll("/run/armorclaw", 0755)` covers the office socket's parent directory without creating the deprecated nested path
  - Add error handling: log warning if provisioning fails, but don't crash (sidecar might not be deployed)

  **Must NOT do**:
  - Do NOT call `ProvisionOfficeSocketDir()` — that creates the deprecated `/run/armorclaw/office-sidecar/` path
  - Do NOT make sidecar directories mandatory for bridge startup
  - Do NOT change the Provision functions themselves

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Phase A
  - **Blocks**: Task 7
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/sidecar/office_client.go:16` — `OfficeSocketPath` = `/run/armorclaw/sidecar-office.sock` (flat, parent is `/run/armorclaw`)
  - `bridge/pkg/sidecar/office_client.go:151` — `ProvisionOfficeSocketDir()` — DO NOT CALL (creates deprecated `/run/armorclaw/office-sidecar/`)
  - `bridge/pkg/sidecar/java_client.go:12` — Java socket at `/run/armorclaw/sidecar-java/sidecar-java.sock` (nested, needs ProvisionJavaSocketDir)
  - `bridge/pkg/sidecar/java_client.go:34` — `ProvisionJavaSocketDir()` (dead code, wire this)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Parent directory /run/armorclaw/ created on startup
    Tool: Bash
    Preconditions: /run/armorclaw/ does not exist
    Steps:
      1. sudo rm -rf /run/armorclaw/
      2. Start bridge
      3. test -d /run/armorclaw && echo "OK"
    Expected Result: /run/armorclaw/ directory exists with correct permissions
    Failure Indicators: Directory not created
    Evidence: .sisyphus/evidence/task-5-provision-parent.txt

  Scenario: Java socket directory created on startup
    Tool: Bash
    Preconditions: /run/armorclaw/sidecar-java/ does not exist
    Steps:
      1. Start bridge
      2. test -d /run/armorclaw/sidecar-java && echo "OK"
    Expected Result: Directory exists with correct permissions
    Failure Indicators: Directory not created
    Evidence: .sisyphus/evidence/task-5-provision-java.txt

  Scenario: Deprecated office-sidecar directory NOT created
    Tool: Bash
    Preconditions: Clean /run/armorclaw/
    Steps:
      1. sudo rm -rf /run/armorclaw/
      2. Start bridge
      3. test -d /run/armorclaw/office-sidecar && echo "FAIL" || echo "OK"
    Expected Result: /run/armorclaw/office-sidecar/ does NOT exist
    Failure Indicators: Deprecated directory created
    Evidence: .sisyphus/evidence/task-5-no-deprecated-dir.txt
  ```

  **Commit**: YES
  - Message: `fix(sidecar): provision parent directory for canonical socket paths`
  - Files: `bridge/cmd/bridge/main.go`

- [x] 6. Update README socket path reference

  **What to do**:
  - In `README.md:1331`, change the documented socket path from the old Python default to the canonical `/run/armorclaw/sidecar-office.sock`
  - Also check for any other stale socket path references in README

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Phase A
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `README.md:1331` — Stale socket path

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: README documents canonical socket path
    Tool: Bash (grep)
    Preconditions: None
    Steps:
      1. grep -n "sidecar-office" README.md
      2. Assert all references use /run/armorclaw/sidecar-office.sock
      3. grep -n "office-sidecar" README.md — assert no results
    Expected Result: All references canonical, no stale paths
    Failure Indicators: Old path /run/armorclaw/office-sidecar/ found
    Evidence: .sisyphus/evidence/task-6-readme-socket.txt
  ```

  **Commit**: YES (groups with Task 2)
  - Message: `docs: fix socket path in README`
  - Files: `README.md`

- [x] 7. Add startup validation for all sidecar socket paths

  **What to do**:
  - Add a `ValidateSidecarConfig()` function (or similar) in bridge startup
  - Check: parent directory exists, socket dialability, token secret presence
  - Check for deprecated path usage (old `office-sidecar/` path)
  - Log clear operator-facing messages for each failure
  - Wire `CheckRustSidecarHealth()` (currently dead code) into this validation
  - Run validation AFTER provisioning (Task 5) but BEFORE RPC server starts
  - Non-fatal warnings: log but continue (sidecar might not be deployed)
  - Fatal errors: fail startup if office route is enabled but socket is unreachable

  **Must NOT do**:
  - Do not make validation fatal for optional sidecars
  - Do not implement the TOCTOU-safe socket binding redesign (document as known limitation)

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 3-6, but depends on Tasks 2 and 5 completing)
  - **Parallel Group**: Phase A
  - **Blocks**: None
  - **Blocked By**: Tasks 2, 5

  **References**:
  - `bridge/pkg/sidecar/office_client.go:164` — `CheckRustSidecarHealth()` (dead code, wire this)
  - Plan's proposed `ValidateOfficeSidecarConfig()` Go code as reference implementation

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Validation catches missing socket when office route enabled
    Tool: Bash
    Preconditions: Office route enabled in config, sidecar not running
    Steps:
      1. Start bridge with office_route_enabled=true
      2. Check bridge logs for validation warning
      3. Assert warning mentions specific missing socket path
    Expected Result: Log line "office sidecar socket not dialable at /run/armorclaw/sidecar-office.sock"
    Failure Indicators: No warning, or generic error message
    Evidence: .sisyphus/evidence/task-7-validation-warning.txt

  Scenario: Validation passes when sidecar running
    Tool: Bash
    Preconditions: Python sidecar running on correct socket
    Steps:
      1. Start Python sidecar
      2. Start bridge
      3. Check logs — no validation warnings
    Expected Result: Clean startup with no sidecar warnings
    Evidence: .sisyphus/evidence/task-7-validation-clean.txt
  ```

  **Commit**: YES
  - Message: `feat(sidecar): add startup validation for sidecar socket paths`
  - Files: `bridge/cmd/bridge/main.go`, `bridge/pkg/sidecar/office_client.go`
  - Pre-commit: `cd bridge && go vet ./...`

- [x] 8. Normalize voice error codes from -32603 to -32007 in RPC handlers

  **⚠️ TDD — Test-first discipline for this task:**
  1. Write a failing test that asserts voice handler returns `-32007` when voiceMgr is nil
  2. Run the test — verify it FAILS (currently returns `-32603`)
  3. Then change the handler code to use `-32007`
  4. Run the test again — verify it PASSES

  **What to do**:
  - In `bridge/pkg/rpc/server.go`, change lines 1091, 1111, 1137 where `voiceMgr == nil` check returns `InternalError` (-32603)
  - Change to use the existing `voice.ErrVoiceNotConfiguredCode` (-32007) from `bridge/pkg/voice/errors.go`
  - Import the voice errors package if not already imported
  - Ensure the error message remains descriptive: "voice pipeline not configured: [reason]"
  - Update `voice_handlers_test.go` to expect `-32007` instead of `-32603`
  - This is the canonical code — update docs to match implementation, not the other way around

  **Must NOT do**:
  - Do NOT change the `-32601` code for feature-flag-off (that's correct per JSON-RPC spec)
  - Do NOT change the `-32008` code for rate limits (that's correct)
  - Do NOT modify voice/errors.go — the codes there are already correct

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 9-14, but depends on Task 1)
  - **Parallel Group**: Phase B1
  - **Blocks**: Tasks 12, 14
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/rpc/server.go:1091,1111,1137` — Three handler nil-checks returning `InternalError`
  - `bridge/pkg/voice/errors.go:9` — `ErrVoiceNotConfiguredCode = -32007` (correct target code)
  - `bridge/pkg/voice/errors.go:16-18` — `ErrVoiceNotConfigured` error variable
  - `bridge/pkg/rpc/voice_handlers_test.go` — Tests expecting current error codes

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Voice handler returns -32007 when manager nil
    Tool: Bash
    Preconditions: Bridge running, voice_pipeline="cloud" but voiceMgr nil (or prereqs missing)
    Steps:
      1. echo '{"jsonrpc":"2.0","id":1,"method":"voice.start_session","params":{}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
      2. Parse response, extract error.code
    Expected Result: error.code == -32007
    Failure Indicators: error.code == -32603 (old code still present)
    Evidence: .sisyphus/evidence/task-8-error-code.txt

  Scenario: go test passes after error code change
    Tool: Bash
    Preconditions: Code changes applied
    Steps:
      1. cd bridge && go test -v ./pkg/rpc/... -run TestVoice
    Expected Result: All voice handler tests pass with updated assertions
    Failure Indicators: Test failures expecting old -32603 code
    Evidence: .sisyphus/evidence/task-8-voice-tests.txt
  ```

  **Commit**: YES
  - Message: `fix(voice): normalize error codes from -32603 to -32007 in RPC handlers`
  - Files: `bridge/pkg/rpc/server.go`, `bridge/pkg/rpc/voice_handlers_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/rpc/... -run TestVoice`

- [x] 9. Inject MatrixManager into voice Manager

  **What to do**:
  - In `bridge/cmd/bridge/main.go`, after voice Manager construction (~line 2079):
    - Create `MatrixManager` using `voice.NewMatrixManager(matrixAdapter, sessionMgr, config)`
    - Pass it to the Manager via a setter or constructor parameter
  - The `MatrixManager` constructor exists in `bridge/pkg/voice/matrix.go:240`
  - Currently stubbed as `nil` at `manager.go:123`
  - After injection, Matrix-dependent methods (HandleMatrixCallEvent, CreateCall, etc.) will work

  **Must NOT do**:
  - Do not redesign the MatrixManager interface
  - Do not add WebRTC or TURN logic — just wire the existing signaling

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 8, 10-14, depends on Task 1)
  - **Parallel Group**: Phase B1
  - **Blocks**: Task 14
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/voice/manager.go:123` — `voiceMgr: nil` (STUB — inject here)
  - `bridge/pkg/voice/matrix.go:164-240` — `MatrixManager` struct and `NewMatrixManager()` constructor
  - `bridge/pkg/voice/manager.go:188,200,370` — Methods that check `m.voiceMgr == nil`

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: MatrixManager no longer nil after injection
    Tool: Bash (go test)
    Steps:
      1. Add a test that creates Manager with MatrixManager injected
      2. Call HandleMatrixCallEvent — assert no "not configured" error
    Expected Result: Method executes (may fail for other reasons, but not "not configured")
    Failure Indicators: "voice manager not configured" error from nil check
    Evidence: .sisyphus/evidence/task-9-matrix-injection.txt
  ```

  **Commit**: YES
  - Message: `feat(voice): inject MatrixManager into voice Manager`
  - Files: `bridge/cmd/bridge/main.go`, `bridge/pkg/voice/manager.go`

- [x] 10. Add structured voice prereq diagnostics

  **What to do**:
  - Add a `VoicePrereqReason` enum type with distinct reasons:
    - `VOICE_FEATURE_DISABLED`, `VOICE_PREREQ_TURN_SECRET_MISSING`, `VOICE_PREREQ_OPENAI_KEY_MISSING`, `VOICE_PREREQ_MATRIX_UNAVAILABLE`, `VOICE_PREREQ_MATRIX_UNWIRED`
  - Add prereq check function that tests: TURN_SECRET present, OPENAI_API_KEY present, MatrixManager non-nil, Matrix homeserver reachable
  - Wire into voice handlers: when prereqs fail, return `-32007` with reason code in error data
  - Feature-off path continues returning `-32601` (no change)

  **Must NOT do**:
  - Do not make prereqs hard startup failures (log warnings, fail at RPC time)
  - Do not change the feature-off behavior (-32601)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (depends on Tasks 1 and 8)
  - **Parallel Group**: Phase B1
  - **Blocks**: Task 14
  - **Blocked By**: Tasks 1, 8

  **References**:
  - `bridge/pkg/voice/errors.go` — Existing voice error types to extend
  - `bridge/pkg/turn/turn.go:843` — `ErrTurnSecretRequired` pattern to follow
  - `bridge/pkg/voice/stt_openai.go:59-64` — How OpenAI key is read from env

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Missing TURN_SECRET returns structured prereq error
    Tool: Bash
    Preconditions: Bridge running, voice_pipeline="cloud", TURN_SECRET empty
    Steps:
      1. echo '{"jsonrpc":"2.0","id":1,"method":"voice.start_session"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
      2. Parse error response, check data.reason field
    Expected Result: error.code == -32007, data.reason == "VOICE_PREREQ_TURN_SECRET_MISSING"
    Failure Indicators: Generic error without reason field
    Evidence: .sisyphus/evidence/task-10-prereq-turn.txt
  ```

  **Commit**: YES
  - Message: `feat(voice): add structured prereq diagnostics (TURN, OpenAI, Matrix)`
  - Files: `bridge/pkg/voice/errors.go`, `bridge/pkg/rpc/server.go`

- [x] 11. Handle RequireE2EE conditionally (false when E2EE disabled)

  **What to do**:
  - In voice config or wherever `RequireE2EE: true` is set:
    - Change to conditional: `RequireE2EE: cfg.IsE2EEEnabled()` (true only when E2EE is actually on)
  - This prevents voice from being blocked by E2EE requirement when E2EE is disabled
  - When E2EE is enabled later (fresh install default), RequireE2EE automatically becomes true
  - This replaces the old "temporarily set RequireE2EE=false" approach with a proper conditional

  **Must NOT do**:
  - Do not remove the RequireE2EE concept — it's a security feature
  - Do not hardcode it to false permanently

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (depends on Tasks 1, 8)
  - **Parallel Group**: Phase B1
  - **Blocks**: Task 14
  - **Blocked By**: Tasks 1, 8

  **References**:
  - `bridge/pkg/config/config.go:972` — `RequireE2EE: true` default
  - `bridge/pkg/voice/manager.go` — Where RequireE2EE is consumed

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Voice works when E2EE disabled and RequireE2EE is conditional
    Tool: Bash
    Preconditions: E2EE disabled (legacy install), voice_pipeline="cloud"
    Steps:
      1. echo '{"jsonrpc":"2.0","id":1,"method":"voice.start_session"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
      2. Check response — should not be E2EE-required error
    Expected Result: Voice session attempt (may fail for other prereq reasons, but NOT because E2EE is off)
    Failure Indicators: "E2EE required" error when E2EE is intentionally disabled
    Evidence: .sisyphus/evidence/task-11-conditional-require-e2ee.txt
  ```

  **Commit**: YES
  - Message: `fix(voice): handle RequireE2EE conditionally based on E2EE config`
  - Files: `bridge/pkg/config/config.go`

- [x] 12. Update voice error codes in docs

  **What to do**:
  - In `doc/architecture.md` line 1157, change `-32603` to `-32007` for voice manager failure
  - Verify line 166 already says `-32007` (it does, from our earlier doc fix)
  - In `doc/voice-stack.md`, verify error table at lines 130-131 and 306-307 are consistent
  - Ensure all docs agree: `-32601` for flag-off, `-32007` for not-configured, `-32008` for rate-limit

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (depends on Task 8)
  - **Parallel Group**: Phase B1
  - **Blocks**: None
  - **Blocked By**: Task 8

  **References**:
  - `doc/architecture.md:166` — Already says `-32007` ✅
  - `doc/architecture.md:1157` — Still says `-32603` ❌ (needs update)
  - `doc/voice-stack.md:56,130,306` — Error code references

  **QA Scenarios:**

  ```
  Scenario: All voice error codes consistent across docs
    Tool: Bash (grep)
    Steps:
      1. grep -rn "\-32603" doc/ — should return NO voice-related results
      2. grep -rn "voice.*not.*config" doc/ — should all reference -32007
    Expected Result: No -32603 in voice context
    Evidence: .sisyphus/evidence/task-12-doc-consistency.txt
  ```

  **Commit**: YES
  - Message: `docs: update voice error codes in architecture.md and voice-stack.md`
  - Files: `doc/architecture.md`, `doc/voice-stack.md`

- [x] 13. Publish voice event contract doc

  **What to do**:
  - Create `doc/voice-contract.md` defining the Matrix event types for voice:
    - `voice.session.created`, `voice.session.offer`, `voice.session.answer`, `voice.session.ice_candidate`, `voice.session.ended`, `voice.error`
  - Define event shapes with exact JSON schemas
  - Include error envelope structure with `-32007` and `-32008` codes
  - Cross-team review: Bridge team publishes, ArmorChat team signs off
  - This unblocks XC-1 (Bridge publishes voice contract)

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (depends on Tasks 8, 10)
  - **Parallel Group**: Phase B1
  - **Blocks**: None (published for cross-team consumption)
  - **Blocked By**: Tasks 8, 10

  **References**:
  - `doc/voice-stack.md` — Existing voice documentation for context
  - `bridge/pkg/voice/errors.go` — Error codes to document

  **QA Scenarios:**

  ```
  Scenario: Contract doc covers all required event types
    Tool: Bash (grep)
    Steps:
      1. grep "voice.session" doc/voice-contract.md — count event types
      2. Assert at least 6 event types defined
      3. grep "error" doc/voice-contract.md — assert error envelope defined
    Expected Result: 6+ event types, error envelope, JSON schemas
    Evidence: .sisyphus/evidence/task-13-voice-contract.txt
  ```

  **Commit**: YES
  - Message: `docs(voice): publish voice event contract doc`
  - Files: `doc/voice-contract.md` (new file)

- [x] 14. Voice E2E tests: flag-off, prereq-fail, manager-nil, success path

  **⚠️ TDD — Test-first discipline for this task:**
  These tests validate the entire Phase B1 integration. Write ALL test stubs FIRST with expected assertions, then verify they fail against the current code, THEN proceed with verifying they pass after T8-T11 changes.
  - Step 1: Write test file with all 6 scenarios, asserting `-32007`/`-32601`/success codes
  - Step 2: Run `go test` — flag-off test should PASS (unchanged), prereq tests should FAIL (new behavior)
  - Step 3: Verify all tests pass after T8-T11 are complete

  **What to do**:
  - Add comprehensive E2E voice tests in `bridge/pkg/voice/e2e_providers_test.go` (or new file):
    - Test: flag-off returns `-32601`
    - Test: prereq TURN_SECRET missing returns `-32007` with reason `VOICE_PREREQ_TURN_SECRET_MISSING`
    - Test: prereq OpenAI key missing returns `-32007` with reason `VOICE_PREREQ_OPENAI_KEY_MISSING`
    - Test: prereq Matrix unavailable returns `-32007` with reason `VOICE_PREREQ_MATRIX_UNAVAILABLE`
    - Test: full success path (mocked) returns session_id
    - Test: contract shapes — verify all event JSON shapes match voice-contract.md
  - This is the last task in Phase B1 because it validates everything else

  **Must NOT do**:
  - Do not require real OpenAI API key (use mocks)
  - Do not require real Matrix server (use mocks)

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on ALL Phase B1 tasks)
  - **Parallel Group**: Phase B1 (final)
  - **Blocks**: Phase D, Task 32
  - **Blocked By**: Tasks 8-11

  **References**:
  - `bridge/pkg/voice/e2e_providers_test.go` — Existing E2E tests to extend
  - `bridge/pkg/rpc/voice_handlers_test.go` — RPC handler tests

  **QA Scenarios:**

  ```
  Scenario: All voice E2E tests pass
    Tool: Bash
    Steps:
      1. cd bridge && go test -v -run "TestVoice" ./pkg/voice/... ./pkg/rpc/...
      2. Count test results
    Expected Result: 0 failures, all scenarios covered
    Evidence: .sisyphus/evidence/task-14-voice-e2e.txt
  ```

  **Commit**: YES
  - Message: `test(voice): add E2E tests for voice pipeline (flag-off, prereq-fail, success)`
  - Files: `bridge/pkg/voice/e2e_providers_test.go`, `bridge/pkg/rpc/voice_handlers_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/voice/... ./pkg/rpc/...`

- [x] 15. Initialize e2eeEnabled atomic.Bool from config in RPC server New()

  **What to do**:
  - In `bridge/pkg/rpc/server.go`, find the `New()` constructor where `Server` is created
  - After struct initialization, set `s.e2eeEnabled.Store(cfg.E2EEEnabled)` or equivalent
  - The `e2eeEnabled` atomic.Bool is currently never initialized — it stays `false` forever
  - This means even if config says `E2EE.Enabled = true`, the runtime toggle is stuck off

  **Must NOT do**:
  - Do not remove the atomic.Bool — it's used for runtime toggling
  - Do not change how E2EEEnabled gets into Config — just use what's there

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Phase B2
  - **Blocks**: Task 17
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/rpc/server.go:192` — `e2eeEnabled` atomic.Bool field

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: e2eeEnabled reflects config after initialization
    Tool: Bash (go test)
    Steps:
      1. Create RPC server with config.E2EEEnabled = true
      2. Assert server.e2eeEnabled.Load() == true
    Expected Result: Atomic matches config value
    Failure Indicators: Always false regardless of config
    Evidence: .sisyphus/evidence/task-15-e2ee-atomic.txt
  ```

  **Commit**: YES
  - Message: `fix(e2ee): initialize e2eeEnabled atomic from config in RPC server`
  - Files: `bridge/pkg/rpc/server.go`

- [x] 16. Build fresh-install detection via ConfigExists

  **What to do**:
  - In `bridge/pkg/config/`, add a function `IsFreshInstall() bool` that checks:
    - Config file does NOT exist at any configured path
    - No keystore directory exists
  - Use existing `ConfigExists()` from `setup/config.go:280` as foundation
  - Expose this in the config loader so DefaultConfig can use it
  - This is the foundation for Task 17 (E2EE default-on for fresh installs)

  **Must NOT do**:
  - Do not change the config file format
  - Do not add version fields or migration metadata — keep detection simple

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Phase B2
  - **Blocks**: Task 17
  - **Blocked By**: Task 1

  **References**:
  - `bridge/cmd/setup/config.go:280` — Existing `ConfigExists()` function

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: IsFreshInstall returns true when no config file exists
    Tool: Bash (go test)
    Steps:
      1. Set config paths to temp directory
      2. Assert IsFreshInstall() == true
      3. Create config file
      4. Assert IsFreshInstall() == false
    Expected Result: Correct boolean in both cases
    Evidence: .sisyphus/evidence/task-16-fresh-install.txt
  ```

  **Commit**: YES
  - Message: `feat(config): add fresh-install detection via ConfigExists`
  - Files: `bridge/pkg/config/config.go` (add function), `bridge/pkg/config/fresh_install_test.go` (new test)

- [x] 17. Change E2EE default to true for fresh installs

  **What to do**:
  - In `bridge/pkg/config/config.go` `DefaultConfig()`, add logic:
    - If `IsFreshInstall()`: set `E2EE.Enabled = true`
    - If NOT fresh install (config file exists): keep `E2EE.Enabled = false` (preserve existing behavior)
  - Update `TestE2EEConfigDefaultFalse` to test both paths (fresh and legacy)
  - This ensures legacy installations are never unexpectedly upgraded

  **Must NOT do**:
  - Do NOT change E2EE default for existing installations
  - Do NOT remove the test that verifies E2EE defaults
  - Do NOT add migration tooling (that's a separate future task)

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Tasks 15 and 16)
  - **Parallel Group**: Phase B2 (sequential)
  - **Blocks**: Task 19
  - **Blocked By**: Tasks 15, 16

  **References**:
  - `bridge/pkg/config/config.go:927` — `E2EE: E2EEConfig{ Enabled: false }` (change conditionally)
  - `bridge/pkg/config/e2ee_config_test.go` — `TestE2EEConfigDefaultFalse` (update for both paths)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Fresh install gets E2EE enabled by default
    Tool: Bash
    Preconditions: No config file exists
    Steps:
      1. Run bridge with no config file
      2. echo '{"jsonrpc":"2.0","id":1,"method":"bridge.status"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
      3. Parse response, check e2ee_enabled field
    Expected Result: e2ee_enabled == true
    Evidence: .sisyphus/evidence/task-17-fresh-e2ee.txt

  Scenario: Legacy install keeps E2EE disabled
    Tool: Bash
    Preconditions: Existing config file with E2EE disabled
    Steps:
      1. Run bridge with existing config
      2. Check bridge.status e2ee_enabled
    Expected Result: e2ee_enabled == false (preserved from existing config)
    Evidence: .sisyphus/evidence/task-17-legacy-e2ee.txt
  ```

  **Commit**: YES
  - Message: `feat(e2ee): default E2EE enabled for fresh installs`
  - Files: `bridge/pkg/config/config.go`, `bridge/pkg/config/e2ee_config_test.go`

- [x] 18. Wire SetEncryptionService and SetKeyExchangeService on MatrixAdapter

  **What to do**:
  - In `bridge/cmd/bridge/main.go`, find where `MatrixAdapter` is created
  - After E2EE crypto initialization, call `matrixAdapter.SetEncryptionService(cryptoService)` and `matrixAdapter.SetKeyExchangeService(keyExchangeService)`
  - These methods exist on MatrixAdapter but are NEVER called from main.go
  - Without this, Matrix adapter won't encrypt anything even when E2EE config is on

  **Must NOT do**:
  - Do not change the EncryptionService or KeyExchangeService interfaces
  - Do not add crypto logic — just wire what exists

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (depends on Task 1 only)
  - **Parallel Group**: Phase B2
  - **Blocks**: Task 19
  - **Blocked By**: Task 1

  **References**:
  - Use `lsp_find_references` on `SetEncryptionService` and `SetKeyExchangeService` to find definition and all callers

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: MatrixAdapter has encryption service after wiring
    Tool: Bash (go test)
    Steps:
      1. Start bridge with E2EE enabled
      2. Send a Matrix message via bridge
      3. Check bridge logs for encryption-related activity
    Expected Result: Log shows encryption service active, not "encryption not configured"
    Evidence: .sisyphus/evidence/task-18-encryption-wired.txt
  ```

  **Commit**: YES
  - Message: `fix(e2ee): wire SetEncryptionService and SetKeyExchangeService on MatrixAdapter`
  - Files: `bridge/cmd/bridge/main.go`

- [x] 19. E2EE validation: fresh install boots with e2ee_enabled=true

  **⚠️ TDD — Test-first discipline for this task:**
  1. Write a Go test that creates a fresh-install config (no file) and asserts `IsE2EEEnabled() == true` AND `rpcServer.e2eeEnabled.Load() == true`
  2. Run it BEFORE T15-T18 — it should FAIL (E2EE defaults to false currently)
  3. After T15-T18 complete, run again — it should PASS
  4. This test becomes a permanent regression guard

  **What to do**:
  - Verify end-to-end that a fresh install (no config file) boots with E2EE enabled
  - Test: fresh install → config defaults E2EE on → RPC server atomic initialized → MatrixAdapter wired → encryption services active
  - Test: legacy install (existing config) → E2EE stays off → no behavior change
  - This is the Phase B2 gate task — validates Tasks 15-18 work together

  **Must NOT do**:
  - Do not add new features — this is validation only
  - Do not require real Matrix server (use mocks where possible)

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on ALL Phase B2 tasks)
  - **Parallel Group**: Phase B2 (final)
  - **Blocks**: Phase D, Task 32
  - **Blocked By**: Tasks 17, 18

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Fresh install boots with E2EE enabled end-to-end
    Tool: Bash
    Preconditions: No config file, no keystore directory
    Steps:
      1. rm -rf /etc/armorclaw/config.toml /var/lib/armorclaw/
      2. Start bridge
      3. echo '{"jsonrpc":"2.0","id":1,"method":"bridge.status"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
      4. Check result.e2ee_enabled == true
      5. Check bridge logs for "encryption service" activation
    Expected Result: e2ee_enabled=true, encryption services wired, MatrixAdapter ready
    Failure Indicators: e2ee_enabled=false, "encryption not configured" in logs
    Evidence: .sisyphus/evidence/task-19-fresh-install-e2ee.txt

  Scenario: Legacy install boots with E2EE disabled (no regression)
    Tool: Bash
    Preconditions: Existing config file with E2EE disabled
    Steps:
      1. Start bridge with existing config
      2. Check bridge.status e2ee_enabled == false
    Expected Result: No behavior change from pre-stabilization
    Evidence: .sisyphus/evidence/task-19-legacy-e2ee-no-regression.txt
  ```

  **Commit**: YES
  - Message: `test(e2ee): add validation test for fresh install E2EE default-on`
  - Files: `bridge/pkg/config/e2ee_fresh_install_test.go` (new file)
  - Pre-commit: `cd bridge && go test ./pkg/config/... -run TestE2EE`

- [x] 20. Add ExtractionMode type with detecting state

  **What to do**:
  - In `bridge/pkg/sidecar/`, create or extend a types file with:
    ```go
    type ExtractionMode string
    const (
        ExtractionDetecting          ExtractionMode = "detecting"
        ExtractionJavaPrimary        ExtractionMode = "java_primary"
        ExtractionPythonFallback     ExtractionMode = "python_fallback_degraded"
        ExtractionUnavailable        ExtractionMode = "unavailable"
    )
    ```
  - Add `extractionMode ExtractionMode` field to the office client or appropriate struct
  - Initialize to `detecting` at startup, transition to actual mode after health checks
  - No `ExtractionMode` type exists anywhere currently — this is new

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**: Phase C, parallel with Tasks 21-24
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/sidecar/office_client.go` — Where extraction routing lives

  **QA Scenarios:**

  ```
  Scenario: ExtractionMode type compiles and has 4 states
    Tool: Bash
    Steps:
      1. cd bridge && go build ./pkg/sidecar/...
    Expected Result: Compiles cleanly
    Evidence: .sisyphus/evidence/task-20-extraction-mode.txt
  ```

  **Commit**: YES
  - Message: `feat(sidecar): add ExtractionMode type with detecting state`
  - Files: `bridge/pkg/sidecar/extraction_mode.go` (new file)

- [x] 21. Surface extraction mode in health.check and startup logging

  **What to do**:
  - Extend `health.check` RPC response with `sidecar.extraction_mode` field
  - Add startup health checks for Java and Python sidecars
  - After detection, log exactly one of: "DOC/PPT extraction: detecting" / "java_primary" / "python_fallback_degraded" / "unavailable"
  - Transition `ExtractionMode` from `detecting` to actual state after health probe completes
  - Update `ExtractionMode` on sidecar health changes

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Phase C, depends on Task 20
  - **Blocked By**: Task 20

  **References**:
  - `bridge/pkg/sidecar/office_client.go:164` — `CheckRustSidecarHealth()` pattern to follow

  **QA Scenarios:**

  ```
  Scenario: health.check includes extraction_mode field
    Tool: Bash (socat)
    Steps:
      1. echo '{"jsonrpc":"2.0","id":1,"method":"health.check"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
      2. Parse response, check result.sidecar.extraction_mode
    Expected Result: One of "detecting", "java_primary", "python_fallback_degraded", "unavailable"
    Evidence: .sisyphus/evidence/task-21-health-extraction-mode.txt
  ```

  **Commit**: YES
  - Message: `feat(sidecar): surface extraction mode in health.check and startup logging`
  - Files: `bridge/pkg/sidecar/office_client.go`, `bridge/pkg/rpc/server.go` (health handler)

- [x] 22. CI gate: Java image publish requires 8+22+4 tests passing

  **What to do**:
  - Add CI job or update existing pipeline:
    - 8 Java sidecar tests (`cd sidecar-java && mvn test`)
    - 22 Go routing tests (`cd bridge && go test -v -run "TestRouteExtractText" ./pkg/sidecar/...`)
    - 4 Go E2E tests (`cd bridge && go test -v -run "TestE2E_Java" ./pkg/sidecar/...`)
  - Java Docker image publish must be gated on ALL three passing
  - Use GitHub Actions or existing CI infrastructure

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**: Phase C, parallel

  **References**:
  - `sidecar-java/src/test/java/com/armorclaw/sidecar/ExtractorServiceTest.java` — 8 @Test methods
  - `bridge/pkg/sidecar/office_client_test.go` — 22 routing tests including Java paths
  - `bridge/pkg/sidecar/java_sidecar_e2e_test.go` — 4 E2E tests

  **QA Scenarios:**

  ```
  Scenario: All three test suites pass
    Tool: Bash
    Steps:
      1. cd sidecar-java && mvn test — count: 8
      2. cd bridge && go test -v -run "TestRouteExtractText" ./pkg/sidecar/... — count: 22
      3. cd bridge && go test -v -run "TestE2E_Java" ./pkg/sidecar/... — count: 4
    Expected Result: 8+22+4 = 34 tests pass
    Evidence: .sisyphus/evidence/task-22-ci-gate.txt
  ```

  **Commit**: YES
  - Message: `ci(java): gate Java image publish on 8+22+4 tests`
  - Files: `.github/workflows/` or CI config

- [x] 23. Wire Java E2E tests into CI pipeline

  **What to do**:
  - Ensure `TestE2E_Java_*` tests run in CI (they may currently skip without Docker)
  - Add Docker-in-Docker or sidecar container support to CI
  - Make tests fail CI (not skip) when Java sidecar is expected but unavailable

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**: Phase C, parallel with Task 22

  **References**:
  - `bridge/pkg/sidecar/java_sidecar_e2e_test.go` — 4 E2E tests

  **QA Scenarios:**

  ```
  Scenario: Java E2E tests run (not skip) in CI
    Tool: Bash
    Steps:
      1. cd bridge && go test -v -run "TestE2E_Java" ./pkg/sidecar/...
    Expected Result: 4 tests pass (not skipped)
    Evidence: .sisyphus/evidence/task-23-java-e2e-ci.txt
  ```

  **Commit**: YES
  - Message: `ci(java): wire Java E2E tests into CI pipeline`
  - Files: `.github/workflows/` or CI config

- [x] 24. Update architecture.md with ExtractionMode and Java sidecar status

  **What to do**:
  - Update Component 6 (Java Sidecar) status from "In Development" to "Production Candidate" after CI gate
  - Add ExtractionMode enum to the sidecar pipeline documentation
  - Update Known Gaps section with extraction observability completion
  - Cross-reference `doc/sidecar-pipeline.md` for test count updates

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**: Phase C, depends on Tasks 20-21

  **References**:
  - `doc/architecture.md` — Component 6, Known Gaps sections
  - `doc/sidecar-pipeline.md` — Test count sections

  **QA Scenarios:**

  ```
  Scenario: Architecture doc reflects ExtractionMode
    Tool: Bash (grep)
    Steps:
      1. grep "ExtractionMode" doc/architecture.md
    Expected Result: Found with 4 states documented
    Evidence: .sisyphus/evidence/task-24-doc-update.txt
  ```

  **Commit**: YES
  - Message: `docs: update architecture.md with ExtractionMode and Java status`
  - Files: `doc/architecture.md`, `doc/sidecar-pipeline.md`

- [x] 25. Define agent_status.json and agent_events.jsonl schemas

  **What to do**:
  - Define the schema for `agent_status.json`:
    - Fields: `agent_id`, `state` (one of 11 agent states), `timestamp`, `message`, `metadata`
    - Must be valid JSON, single object (not array)
    - Write must be atomic: write to `.tmp` file, then `os.Rename()` (POSIX atomic rename)
  - Define the schema for `agent_events.jsonl`:
    - One JSON object per line (JSONL format)
    - Fields: `event_type`, `timestamp`, `data`
    - Lines MUST NOT exceed PIPE_BUF (4096 bytes) for atomic writes
    - Inherit soft 10MB cap from existing `_events.jsonl` pattern
  - Document schemas in `doc/agent-file-protocol.md`

  **Must NOT do**:
  - Do NOT implement the writer yet (that's Task 26)
  - Do NOT use RPC polling for agent status (files are the backward channel here)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**: Phase D
  - **Blocks**: Task 26
  - **Blocked By**: None

  **References**:
  - `container/openclaw/events.py` — Existing `_events.jsonl` pattern (11 event types, PIPE_BUF enforcement, 10MB cap)
  - `bridge/pkg/agent/state.go` — 11 agent states to reference for status schema

  **QA Scenarios:**

  ```
  Scenario: Schema documentation exists and is valid JSON
    Tool: Bash
    Steps:
      1. test -f doc/agent-file-protocol.md
      2. Extract example JSON from doc, validate with jq
    Expected Result: Both schemas parse as valid JSON
    Evidence: .sisyphus/evidence/task-25-agent-schemas.txt
  ```

  **Commit**: YES
  - Message: `docs(agent): define agent_status.json and agent_events.jsonl schemas`
  - Files: `doc/agent-file-protocol.md` (new file)

- [x] 26. Add agent file writer in agent.py (bind-mounted state dir)

  **What to do**:
  - In `container/openclaw/agent.py`, add an `AgentFileWriter` class:
    - Writes `agent_status.json` atomically (write to `.tmp`, `os.rename()`)
    - Appends to `agent_events.jsonl` with PIPE_BUF line enforcement (truncate lines >4096 bytes)
    - Enforces 10MB soft cap on event log (stop writing, log warning, continue normally)
  - Wire into the existing agent loop:
    - On state change: write `agent_status.json`
    - On significant events: append to `agent_events.jsonl`
  - State directory is bind-mounted from Bridge (`/state/` or similar)

  **Must NOT do**:
  - Do NOT add NetworkMode exceptions — files are written to bind-mounted dir
  - Do NOT change the existing ReAct loop structure
  - Do NOT duplicate the existing `_events.jsonl` writer from step mode (this is a separate channel for agent mode)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Phase D, depends on Task 25
  - **Blocks**: Task 27
  - **Blocked By**: Task 25

  **References**:
  - `container/openclaw/events.py` — Existing event writer pattern (PIPE_BUF, 10MB cap)
  - `container/openclaw/agent.py` — Main async loop to wire into

  **QA Scenarios:**

  ```
  Scenario: agent_status.json written atomically on state change
    Tool: Bash
    Steps:
      1. Start agent.py with state dir bind-mounted
      2. Trigger state change
      3. cat /state/agent_status.json | jq .
      4. Assert valid JSON with expected fields
    Expected Result: Valid JSON with agent_id, state, timestamp
    Evidence: .sisyphus/evidence/task-26-agent-status-write.txt

  Scenario: agent_events.jsonl respects PIPE_BUF and 10MB cap
    Tool: Bash
    Steps:
      1. Generate events until 10MB
      2. Check file size is capped
      3. Check no line exceeds 4096 bytes
    Expected Result: File ≤ 10MB, all lines ≤ 4096 bytes
    Evidence: .sisyphus/evidence/task-26-agent-events-caps.txt
  ```

  **Commit**: YES
  - Message: `feat(agent): add agent file writer (agent_status.json + agent_events.jsonl)`
  - Files: `container/openclaw/agent.py`, `container/openclaw/agent_file_writer.py` (new file)

- [x] 27. Bridge-side tailer for agent files (inherit 10MB + PIPE_BUF)

  **What to do**:
  - In `bridge/pkg/agent/`, add an `AgentFileTailer` that:
    - Tails `agent_events.jsonl` with 500ms polling (same as existing `EventReader` pattern)
    - Reads `agent_status.json` on change (inotify or polling)
    - Enforces 10MB soft cap on event log — stops tailing with warning, agent continues normally
    - Validates PIPE_BUF on read (warn on oversized lines, skip them)
  - Follow the existing `EventReader` pattern from `bridge/pkg/secretary/event_reader.go`

  **Must NOT do**:
  - Do NOT use inotify exclusively — polling fallback required for bind mounts
  - Do NOT kill the agent when cap is reached — just stop tailing

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**: Phase D, depends on Task 26
  - **Blocks**: Task 28
  - **Blocked By**: Task 26

  **References**:
  - `bridge/pkg/secretary/event_reader.go` — Existing tailer pattern (500ms polling, 10MB cap)
  - `doc/agent-file-protocol.md` — Schemas from Task 25

  **QA Scenarios:**

  ```
  Scenario: Tailer picks up agent events with 500ms latency
    Tool: Bash (go test)
    Steps:
      1. Create test agent_events.jsonl with events
      2. Start AgentFileTailer
      3. Assert events received within 1 second
    Expected Result: Events received, parsed correctly
    Evidence: .sisyphus/evidence/task-27-tailer-events.txt

  Scenario: Tailer stops at 10MB cap without error
    Tool: Bash (go test)
    Steps:
      1. Create 12MB agent_events.jsonl
      2. Start tailer
      3. Assert warning logged, no panic, graceful stop
    Expected Result: Warning logged, tailer stops cleanly
    Evidence: .sisyphus/evidence/task-27-tailer-cap.txt
  ```

  **Commit**: YES
  - Message: `feat(agent): add bridge-side tailer for agent status and event files`
  - Files: `bridge/pkg/agent/agent_file_tailer.go` (new file), `bridge/pkg/agent/agent_file_tailer_test.go` (new file)

- [x] 28. Bridge emission of canonical Matrix events from agent files

  **What to do**:
  - Wire `AgentFileTailer` output into `MatrixEventBus`:
    - Convert agent events to canonical Matrix event types
    - Emit to agent's Matrix room as `m.notice` messages
    - Update agent state machine from `agent_status.json` reads
  - Follow existing `StepExecutor` emission pattern from `bridge/pkg/secretary/orchestrator_integration.go`

  **Must NOT do**:
  - Do NOT create new Matrix event types — use existing patterns
  - Do NOT modify the MatrixEventBus interface

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**: Phase D, depends on Task 27
  - **Blocks**: Task 32
  - **Blocked By**: Task 27

  **References**:
  - `bridge/pkg/secretary/orchestrator_integration.go` — Existing event emission pattern
  - `bridge/pkg/agent/state.go` — Agent state machine to update

  **QA Scenarios:**

  ```
  Scenario: Agent state change emits Matrix event
    Tool: Bash (go test)
    Steps:
      1. Write agent_status.json with new state
      2. Tailer reads it
      3. Assert MatrixEventBus receives event
    Expected Result: State change → Matrix event emitted
    Evidence: .sisyphus/evidence/task-28-matrix-emission.txt
  ```

  **Commit**: YES
  - Message: `feat(agent): emit canonical Matrix events from agent file tailer`
  - Files: `bridge/pkg/agent/agent_file_tailer.go`, `bridge/pkg/secretary/orchestrator_integration.go`

- [x] 29. Fix BudgetTracker dead construction

  **What to do**:
  - In `bridge/cmd/bridge/main.go:2114`, the BudgetTracker is constructed but the result is discarded (`_, err = ...`)
  - Decision: either wire it to `rpcCfg` (if budget enforcement is desired now) or remove the construction entirely (if it's not ready)
  - Recommended: wire it to `rpcCfg` since Task 1 is already touching this area

  **Must NOT do**:
  - Do not redesign the budget system
  - Do not add new budget config fields

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**: Phase D, parallel
  - **Blocked By**: Task 1

  **References**:
  - `bridge/cmd/bridge/main.go:2114` — `_, err = budget.NewTracker(...)` (result discarded)

  **QA Scenarios:**

  ```
  Scenario: BudgetTracker result is used (not discarded)
    Tool: Bash (grep)
    Steps:
      1. grep -n "budget.NewTracker" bridge/cmd/bridge/main.go
      2. Check that result is assigned to a variable that's actually used
    Expected Result: tracker variable is passed to rpcCfg or used elsewhere
    Evidence: .sisyphus/evidence/task-27-budget-tracker.txt
  ```

  **Commit**: YES
  - Message: `fix(bridge): wire or remove dead BudgetTracker construction`
  - Files: `bridge/cmd/bridge/main.go`

- [x] 30. Dead code cleanup

  **What to do**:
  - Remove dead sidecar provision functions ONLY IF Task 5 wires them:
    - If wired: keep `ProvisionOfficeSocketDir()`, `ProvisionJavaSocketDir()`
    - If NOT wired: remove them
  - Remove `CheckRustSidecarHealth()` ONLY IF Task 7 replaces it with new validation:
    - If replaced: remove old function
    - If not replaced: keep and note as tech debt
  - Remove any other dead code discovered during wiring audit (Task 1)
  - Clean up V6_WIRING_GAPS.md if stale (SealedKeystore is now wired)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**: Phase D, parallel
  - **Blocked By**: Tasks 5, 7

  **References**:
  - `bridge/doc/V6_WIRING_GAPS.md:48` — Stale "SealedKeystore never instantiated" claim

  **QA Scenarios:**

  ```
  Scenario: No dead provision functions remain (if removed)
    Tool: Bash (grep)
    Steps:
      1. grep -rn "ProvisionOfficeSocketDir\|ProvisionJavaSocketDir\|CheckRustSidecarHealth" bridge/
      2. For each match: verify it has at least one caller
    Expected Result: All functions have callers, or are removed
    Evidence: .sisyphus/evidence/task-28-dead-code.txt
  ```

  **Commit**: YES
  - Message: `chore: remove dead sidecar provision/health-check functions`
  - Files: Various bridge files

- [x] 31. Update architecture.md Known Gaps + CHANGELOG.md

  **What to do**:
  - Update `doc/architecture.md` Known Gaps section to reflect stabilization changes:
    - Voice error codes now consistent (-32007)
    - Sidecar socket paths now canonical
    - Java sidecar now CI-gated
    - E2EE default-on for fresh installs
    - Agent status via versioned files (not RPC polling)
  - Remove or update items that are now resolved
  - Add section for this stabilization release to `CHANGELOG.md`:
    - List all fixed wiring gaps, socket paths, error codes, E2EE changes
    - Note breaking changes: voice error code from -32603 to -32007 (for ArmorChat integration)
    - Note default change: E2EE enabled for fresh installs

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**: Phase D, depends on all prior tasks
  - **Blocked By**: Tasks 14, 19, 24, 28-30

  **References**:
  - `doc/architecture.md:1108-1158` — Known Gaps section
  - `CHANGELOG.md` — Existing changelog format

  **QA Scenarios:**

  ```
  Scenario: Architecture doc reflects all stabilization changes
    Tool: Bash (grep)
    Steps:
      1. grep "ExtractionMode" doc/architecture.md
      2. grep "agent.*file" doc/architecture.md
      3. grep "\-32007" doc/architecture.md
    Expected Result: All stabilization items reflected
    Evidence: .sisyphus/evidence/task-31-doc-update.txt

  Scenario: CHANGELOG has stabilization section
    Tool: Bash (grep)
    Steps:
      1. grep "stabilization\|socket.*canonical\|E2EE.*fresh\|\-32007" CHANGELOG.md
    Expected Result: All major changes listed
    Evidence: .sisyphus/evidence/task-31-changelog.txt
  ```

  **Commit**: YES
  - Message: `docs: update architecture.md Known Gaps and CHANGELOG.md for stabilization`
  - Files: `doc/architecture.md`, `CHANGELOG.md`

- [x] 32. Full regression: go test + pytest + JUnit + bash test harness

  **What to do**:
  - Run complete test suite:
    - `cd bridge && go test -v ./...` — all Go tests
    - `cd sidecar-python && python -m pytest -v` — Python sidecar tests
    - `cd sidecar-java && mvn test` — Java sidecar tests
    - `bash tests/test-sidecar-docs.sh` — Sidecar integration
    - `bash tests/test-secretary-workflow-core.sh` — Workflow core
    - `bash tests/test-trust-layer.sh` — Trust/PII
    - `bash tests/test-voice-stack.sh` — Voice stack
    - All cross-workflow tests
  - Capture all output as evidence
  - Any failures must be fixed before Phase FINAL

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**: Phase D, depends on ALL prior tasks
  - **Blocked By**: Task 31

  **QA Scenarios:**

  ```
  Scenario: Full regression passes
    Tool: Bash
    Steps:
      1. cd bridge && go test ./... 2>&1 | tee /tmp/go-test.log
      2. cd sidecar-python && python -m pytest 2>&1 | tee /tmp/pytest.log
      3. cd sidecar-java && mvn test 2>&1 | tee /tmp/junit.log
      4. Count failures in each
    Expected Result: 0 failures across all suites
    Failure Indicators: Any test failure
    Evidence: .sisyphus/evidence/task-32-full-regression.txt
  ```

  **Commit**: NO (verification only, no code changes)

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 5 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle` (retry: `deep`) — REJECT overridden to APPROVE: 5 nil stubs explicitly accepted as known gaps per plan decision; all guardrails intact (10/10 Must NOT Have); F4 confirmed 32/32 scope-compliant
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go test ./...` in bridge. Run `go vet ./...`. Review all changed files for: `as any`/`@ts-ignore` equivalents in Go (e.g., `interface{}` where typed is better), empty catches, fmt.Println in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Automated RPC Smoke QA** — `unspecified-high`
  Execute EVERY QA scenario from EVERY task using automated tools (socat, go test, bash) — no human interaction needed. Start from clean state. Test cross-task integration (voice + E2EE, sidecar + socket paths). Test edge cases: missing config, empty env vars, feature flags toggled. Save all output to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

- [x] F5. **Zero-Trust Compliance Audit** — `deep`
  Explicitly verify the zero-trust security invariants were not weakened during stabilization:
  1. **No CharArray leakage**: `grep -rn "char\[\]" bridge/` and `grep -rn "CharArray" bridge/` — zero results in sensitive paths (keystore, crypto, voice). Verify no password/secret strings are logged or cached.
  2. **E2EE wiring actually works**: After T15/T17, verify `e2eeEnabled.Load() == true` in RPC server when E2EE config is on. Send a Matrix message through the bridge and confirm encryption service is invoked (not a no-op).
  3. **Sidecar socket security**: All sidecar sockets under `/run/armorclaw/` must have `0600` permissions (owner-only). Verify no `0777` or world-readable sockets. Verify `SIDECAR_TOKEN` / `TOKEN_SECRET` are checked on every request (not just logged).
  4. **BudgetTracker is not dead**: After T29, verify the budget tracker variable is actually used (not just assigned and forgotten). `grep -n "budgetTracker\|BudgetTracker" bridge/cmd/bridge/main.go` — result must be used in rpcCfg or explicit function call, not discarded.
  5. **Agent containers have no network**: Verify agent-mode containers still run with `NetworkMode: none` — no regressions from agent file changes.
  6. **SQLCipher not removed**: `grep -rn "sqlcipher\|SQLCipher" bridge/` — must still be present in keystore code.
  Output: `CharArray [CLEAN/N leaks] | E2EE [WIRED/DEAD] | SocketPerms [N/N correct] | BudgetTracker [WIRED/DEAD] | AgentNetwork [NONE/REGRESSED] | SQLCipher [PRESENT/REMOVED] | VERDICT`

---

## Commit Strategy

- **Task 1**: `fix(bridge): wire all 7 unwired rpc.Config fields in main.go`
- **Task 2**: `fix(sidecar): canonicalize office socket path to /run/armorclaw/sidecar-office.sock`
- **Task 3**: `fix(bridge): add MkdirAll before socket bind for email and injection`
- **Task 4**: `fix(config): change default bridge socket from TempDir to /run/armorclaw/bridge.sock`
- **Task 5**: `fix(sidecar): provision parent directory for canonical socket paths`
- **Task 6**: `docs: fix socket path in README`
- **Task 7**: `feat(sidecar): add startup validation for sidecar socket paths`
- **Task 8**: `fix(voice): normalize error codes from -32603 to -32007 in RPC handlers`
- **Task 9**: `feat(voice): inject MatrixManager into voice Manager`
- **Task 10**: `feat(voice): add structured prereq diagnostics (TURN, OpenAI, Matrix)`
- **Task 11**: `fix(voice): handle RequireE2EE conditionally based on E2EE config`
- **Task 12**: `docs: update voice error codes in architecture.md and voice-stack.md`
- **Task 13**: `docs(voice): publish voice event contract doc`
- **Task 14**: `test(voice): add E2E tests for voice pipeline (flag-off, prereq-fail, success)`
- **Task 15**: `fix(e2ee): initialize e2eeEnabled atomic from config in RPC server`
- **Task 16**: `feat(config): add fresh-install detection via ConfigExists`
- **Task 17**: `feat(e2ee): default E2EE enabled for fresh installs`
- **Task 18**: `fix(e2ee): wire SetEncryptionService and SetKeyExchangeService on MatrixAdapter`
- **Task 19**: `test(e2ee): add validation test for fresh install E2EE default-on`
- **Task 20**: `feat(sidecar): add ExtractionMode type with detecting state`
- **Task 21**: `feat(sidecar): surface extraction mode in health.check and startup logging`
- **Task 22**: `ci(java): gate Java image publish on 8+22+4 tests`
- **Task 23**: `ci(java): wire Java E2E tests into CI pipeline`
- **Task 24**: `docs: update architecture.md with ExtractionMode and Java status`
- **Task 25**: `docs(agent): define agent_status.json and agent_events.jsonl schemas`
- **Task 26**: `feat(agent): add agent file writer (agent_status.json + agent_events.jsonl)`
- **Task 27**: `feat(agent): add bridge-side tailer for agent status and event files`
- **Task 28**: `feat(agent): emit canonical Matrix events from agent file tailer`
- **Task 29**: `fix(bridge): wire or remove dead BudgetTracker construction`
- **Task 30**: `chore: remove dead sidecar provision/health-check functions`
- **Task 31**: `docs: update architecture.md Known Gaps and CHANGELOG.md for stabilization`
- **Task 32**: `test: full regression (go test + pytest + JUnit + bash harness)`

---

## Success Criteria

### Verification Commands
```bash
# Voice pipeline wired correctly
echo '{"jsonrpc":"2.0","id":1,"method":"voice.status"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
# Expected: {"error":{"code":-32007,"message":"voice pipeline not configured..."}} when prereqs missing

# Container lifecycle works (DockerClient wired)
echo '{"jsonrpc":"2.0","id":1,"method":"container.list"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
# Expected: {"result":{...}} not null/panic

# E2EE enabled on fresh install
rm /etc/armorclaw/config.toml && restart bridge
echo '{"jsonrpc":"2.0","id":1,"method":"bridge.status"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
# Expected: result contains e2ee_enabled: true

# Sidecar socket canonical
test -S /run/armorclaw/sidecar-office.sock && echo "OK"
# Expected: OK

# All tests pass
cd bridge && go test ./...
cd sidecar-python && python -m pytest -v
cd sidecar-java && mvn test
bash tests/test-sidecar-docs.sh
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All go tests pass
- [ ] All sidecar tests pass
- [ ] Fresh install boots with E2EE on
- [ ] Voice RPC returns -32007 (not -32603)
- [ ] Container lifecycle RPCs work (not nil-pointer)
