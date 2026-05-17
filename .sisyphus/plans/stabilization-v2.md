User has answered your questions: "Plan is ready at .sisyphus/plans/stabilization-v2.md (21 tasks + 4 final verification). How would you like to proceed?"="# ArmorClaw Stabilization v4.1 — Complete Execution Plan

## How to Execute This Plan

1. **Branch**: Create `stabilization-v4` branch off `main`. All work lands here.
2. **Wave order**: Complete each wave before starting the next. Waves 3 and 4 have parallel tasks within them — those can run concurrently.
3. **Investigation gates**: T3a, T6a, T7a, and T0a produce design documents. **No implementation PR for T3/T6/T7/T0 may be merged until the corresponding design doc is reviewed and approved.** These are blocking gates.
4. **Evidence**: All QA scenario output goes to `.sisyphus/evidence/task-{N}-{slug}.txt`. No exceptions.
5. **Harness**: Run `tests/matrix-e2e/harness.sh start` before any Phase E test. Run `harness.sh stop` after.
6. **Milestone gates**: After Waves 1–3, pause for a milestone review before starting Wave 4. After Waves 4–5, pause again before Wave 6. See Milestone Gates section.
7. **Two-release option**: If timeline pressure exists, split at the Wave 3/4 boundary:
   - **Release 1** (Waves 1–3): Nil stubs, command wiring, Java sidecar, MatrixManager, AgentFileTailer, Guard mode-aware. ~8 days.
   - **Release 2** (Waves 4–6): Conduit harness, Matrix E2E tests, event validation, cleanup. ~12 days.
8. **Ticket backlog**: See the table at the bottom. Open T1, T2, T3a, and T0a as the starter set.

---

## TL;DR

> **Quick Summary**: Close 5 nil-stubbed `rpc.Config` fields (with interface confirmation first), wire only the missing Matrix command families (after audit), deploy Java sidecar for DOC/PPT, fully integrate agent file tailing into Secretary, construct live MatrixManager for voice, and build Matrix E2E test infrastructure starting with documented commands.
>
> **Deliverables**:
> - 5 nil stubs wired: AuditLog, Guard, NavChartStore, SkillGate, GovernanceRoomID
> - Only missing command families wired into `processEvents()` (after audit confirms gap)
> - Java sidecar deployed as DOC/PPT default; Python DOC/PPT fallback removed
> - AgentFileTailer wired into Secretary; agent-mode file writer injected; tailer race fixed
> - MatrixManager constructed with live Matrix client
> - Build fix: CGO_ENABLED + .gitignore updates
> - Conduit test fixtures and lifecycle harness
> - Matrix E2E tests: documented commands first (`/status`, `!agent skills`, `/approve`), then expanded
> - Separate validation for `m.notice` command responses vs. custom runtime events
>
> **Estimated Effort**: Large (~20 days, or ~8 + ~12 if split into two releases)
> **Parallel Execution**: YES - 6 waves
> **Critical Path**: T1 → T3a → T3 → T0a → T0 → T8 → T11 → T12 → T15 → T21a → F1-F5

---

## Context

### What the first stabilization (v1) completed
- Phase 0: Plan cleanup and document alignment
- Phase A (verification): Socket canonicalization, MkdirAll, default Bridge socket path, parent directory provisioning, sidecar socket startup validation
- Phase B1 (partial): Voice error code normalization to `-32007`, structured prereq diagnostics, voice contract document, RequireE2EE conditional behavior
- Phase B2: E2EE default-on for fresh installs, runtime atomic, fresh-install detection, encryption/key-exchange wiring into MatrixAdapter
- Phase C (partial): ExtractionMode observability, CI gate for Java sidecar, conservative doc updates
- Phase D (foundations only): Agent file protocol document, agent-side file writer, Bridge-side tailer code
- 5 nil stubs accepted as known gaps with graceful degradation

### What remains genuinely open
- 5 nil stubs need **implementation** (not just wiring): AuditLog, Guard, NavChartStore, SkillGate, GovernanceRoomID
- **Unknown command routing gap**: The architecture docs suggest some Matrix commands (`/claim_admin`, `/status`, `/approve`, `/reject`, `!agent skills`, `!agent forget-skill`) may already be routed through `processEvents()`, while others may be missing. The exact gap is unconfirmed. T0a will determine this.
- Java sidecar is **not deployed** as default — DOC/PPT extraction still falls through to Python's broken XlsConverter path
- AgentFileTailer is **not integrated** — file protocol and tailer code exist but are dead code, not wired into Secretary
- Agent-mode containers have **no file writer injection** — the backward channel only works in step mode
- Tailer race condition (`TestAgentFileTailer_StopsAt10MBCap`) is **unfixed**
- MatrixManager is **not constructed** — `SetMatrixManager()` exists but injects `nil` at runtime
- Matrix E2E tests **don't exist** — command path is only mock-tested, never validated through live Conduit
- Build script uses `CGO_ENABLED=0` — incompatible with SQLCipher and YARA
- `.gitignore` missing `build-output/` and `go.work.sum`
- Conduit test server missing `nginx.conf` and `wellknown/` fixtures

### v3 → v4 Changes (from prior critique)

| Change | Why |
|--------|-----|
| Added T0a — Command-path audit before T0 | T0 assumed all Matrix command routing is dead; docs suggest some already exists |
| Phase E reordered — documented commands first | Start with `/status`, `!agent skills`, `!agent forget-skill`, `/approve`, `/reject` which are confirmed in docs |
| Added T3a — Audit contract alignment | T3 proposed JSON-lines but architecture says `audit.db`; must confirm interface first |
| T4 Guard has mode-aware fail behavior | Nil Guard is a security gap in Sentinel mode; same degradation pattern is too permissive |
| Added T6a, T7a — Interface confirmations | Plan invented concrete semantics without confirming actual interfaces |
| TJ1/TJ2 deployment target clarified | Explicit rollout: dedicated Java compose path first, merge after validation |
| TA2, TA3 moved into scope | Agent-mode injection and tailer race fix are needed for Phase D closure |
| T20 split into T21a (m.notice) + T21b (custom events) | Command responses are `m.notice` Matrix messages; runtime events are `workflow.*`/`agent.*` custom types |
| TV1 dependency fixed | Depends on Matrix client availability, not GovernanceRoomID |

### v4 → v4.1 Changes (from latest critique)

| Change | Why |
|--------|-----|
| Added "How to Execute This Plan" section | Plan is very long; needs quick-start guidance |
| Added Milestone Gates between wave groups | ~20 days is ambitious; need explicit checkpoints |
| Added Risk Register with mitigations | Critical path has several high-risk tasks in sequence |
| T0 scope explicitly conditional on T0a | T0 only wires what T0a confirms is missing — no more, no less |
| TJ1 specifies exact error message | Prevents cryptic failures when Java sidecar is down |
| Added F5 Zero-Trust Compliance review | Verify Guard hard-fail, no CharArray leakage, E2EE, SQLCipher intact |
| T3a/T6a/T7a are now blocking gates | No implementation PR merged until design doc reviewed |
| T0a inventory must be machine-readable | CSV/JSON format for T0 automation |
| TJ2 adds explicit Python-not-called test | Verify Python is not invoked for DOC/PPT when Java is down |
| TA2 cross-repo coordination note | Bridge + container/openclaw changes need coordination |

---

## Scope Boundaries

### In scope
- All 5 `rpc.Config` nil stub implementation and wiring (with interface confirmation first)
- Command-path audit to identify exactly which command families need wiring
- Wiring only the missing command families into `processEvents()` — scope determined entirely by T0a
- Java sidecar deployment as DOC/PPT default routing target
- Python DOC/PPT fallback removal
- AgentFileTailer integration into Secretary
- Agent-mode file writer injection
- Tailer race condition fix
- MatrixManager construction and injection
- Conduit test infrastructure and Matrix E2E tests (documented commands first)
- Build tooling fixes
- Dev environment prerequisite verification
- Separate validation for m.notice command responses and custom runtime events

### Explicitly out of scope (tracked for future work)

| Item | Tracking ID | Reason |
|------|-----------|--------|
| `e2ee.restore_backup` | AC-4 | Intentional security decision, v1.1 |
| Local STT/TTS providers (ONNX) | AC-6 | Deferred |
| Challenge-response keystore unseal | AC-7 | v1.1 |
| Qdrant builder migration | AC-13 | Low priority |
| PPTX→PDF conversion | AC-9 | Low priority |
| Per-group parallel collection policy override | AC-15 | Low priority |
| Crash-only WebSocket redesign | AC-14 | Requires CTO approval |
| ArmorChat memory zeroization | AT-1 | Separate codebase |
| E2EE fresh/legacy integration test | AC-B2.5 | Existing Go test patterns sufficient |
| Dead BudgetTracker removal | AC-D5 | Low risk, low priority |
| Per-user rate limiting (Guard) | — | Scope creep, v1.1 |
| Per-operation gating (Guard) | — | Scope creep, v1.1 |
| Dynamic skill loading | — | Scope creep |
| Network-based skill validation | — | Scope creep |

### Must NOT Have (Guardrails)
- Do NOT remove SQLCipher
- Do NOT bypass Matrix as control plane
- Do NOT weaken approval flow for payments or critical PII
- Do NOT add E2EE encryption/decryption tests
- Do NOT require SSH/VPS access for Phase E tests — local Docker Conduit only
- Do NOT require real OpenAI/Docker/TURN credentials for tests — use mocks or skip flags
- Do NOT silently fall back to Python for DOC/PPT — return clear error instead
- Do NOT wire AdminCommandHandler unconditionally — only wire missing families after T0a audit
- Do NOT duplicate existing command dispatch — verify before adding
- Do NOT introduce direct production secret access
- Do NOT add dependency injection frameworks
- Do NOT redesign the budget system
- Do NOT add `restore_backup` without updating CI security gate tests first
- Do NOT create Phase E criteria requiring "deploy to VPS" or "open ArmorChat app"
- Do NOT create Phase E criteria requiring "manually check Matrix room" — use curl/jq
- Do NOT assume `test-matrix-plane.sh` can be reused without refactoring (SSH dependency)
- Do NOT hardcode production secrets in test fixtures
- Do NOT use port 443 for Conduit test fixtures (use 8008/8448)
- Do NOT add CGO dependencies to non-bridge targets
- Do NOT modify docker-compose.yml Conduit service definition — only add missing referenced files
- Do NOT use Python for Matrix test library — bash/curl/jq only
- Do NOT use `websocat` for sync — use polling with `curl`
- Do NOT remove `doc/` or `.sisyphus/` from `.gitignore`
- Do NOT apply the same nil-check degradation to all fields — Guard must fail hard in sentinel mode
- Do NOT process the Bridge bot's own messages as commands
- Do NOT modify existing command handler implementations — only wire missing dispatch
- Do NOT replace `audit.db` without updating architecture docs to match
- Do NOT invent NavChartStore or SkillGate interfaces without confirming what `rpc.Config` actually expects
- Do NOT conflate `m.notice` command responses with `workflow.*`/`agent.*` custom events
- Do NOT refactor unrelated code during dead-code cleanup
- Do NOT add libyara-dev to production Docker images (build-time only)
- Do NOT merge T3/T6/T7 implementation PRs until T3a/T6a/T7a design docs are reviewed and approved (blocking gates)
- Do NOT exceed T0's scope beyond what T0a confirms is missing

---

## Milestone Gates

### Gate 1: After Wave 3 (before starting Wave 4)

**Checkpoint**: Nil stubs closed, command wiring complete, Java sidecar deployed, MatrixManager constructed, AgentFileTailer integrated.

**Verification commands**:
```bash
# All nil stubs resolved
grep "= nil" bridge/cmd/bridge/main.go | grep -v "TODO" | grep -v "tracking"
# Expected: 0 hits

# Build succeeds
CGO_ENABLED=1 go build -tags "yara" ./bridge/cmd/bridge/

# Guard fails in sentinel mode
cd bridge && go test -v -run TestGuard_SentinelModeFailure ./pkg/governor/...

# DOC/PPT returns error (not XlsConverter) when Java down
cd bridge && go test -v -run TestRouteExtractText_DOC_JavaDown_ReturnsError ./pkg/sidecar/...

# All Go tests pass
cd bridge && go test ./...

# MatrixManager non-nil when Matrix available
grep "voiceMgr.SetMatrixManager" bridge/cmd/bridge/main.go
# Expected: non-nil construction
```

**Go/No-Go criteria**: All verification commands pass → proceed to Wave 4. Any failure → fix before proceeding.

**Two-release split point**: If timeline pressure exists, ship Release 1 here and defer Waves 4–6 to Release 2.

### Gate 2: After Wave 5 (before starting Wave 6)

**Checkpoint**: Conduit harness stable, documented-command E2E tests passing, conditional commands tested or skipped.

**Verification commands**:
```bash
# Harness starts cleanly
cd tests/matrix-e2e && ./harness.sh start && ./harness.sh status && ./harness.sh stop

# All documented-command tests pass
cd tests/matrix-e2e
for t in cases/test-status.sh cases/test-agent-skills.sh cases/test-agent-forget-skill.sh cases/test-approve-reject.sh; do
    ./run-test.sh "$t"
done
```

**Go/No-Go criteria**: All documented-command tests pass → proceed to Wave 6.

---

## Risk Register

| Risk | Probability | Impact | Mitigation | Owner |
|------|------------|--------|-----------|-------|
| T3a reveals AuditLog interface incompatible with plan | Medium | High — T3 blocked | T3a is a blocking gate; if incompatible, update T3 design before implementing | T3a assignee |
| T0a reveals most commands already wired | Low | Low — T0 scope shrinks | T0 explicitly conditional on T0a; smaller scope = faster delivery | T0a assignee |
| T0a reveals no commands wired | Medium | Medium — T0 scope grows | T0 budget includes 1 day for full wiring; escalate if >1.5 days needed | T0 assignee |
| MatrixManager constructor has stale API (unwired since implementation) | Medium | Medium — TV1 blocked | TV1 includes API verification step; budget 0.5 day for reconciliation | TV1 assignee |
| Conduit test harness unreliable in CI | Medium | Medium — Phase E blocked | Start with local-only testing; add CI incrementally; allow Phase E to be partially complete | T11 assignee |
| Agent-mode file writer requires container image changes | Medium | Low — TA2 scope clarification | TA2 explicitly notes cross-repo coordination (Bridge + container/openclaw); coordinate before starting | TA2 assignee |
| Guard implementation scope creep (per-user, per-operation) | Low | Medium — T4 overruns | Explicitly out of scope; T4 limited to per-method RPS only | T4 assignee |
| Java sidecar fails to start in Docker Compose | Low | Medium — TJ1 blocked | TJ1 uses dedicated compose path; validate before merging into main deployment | TJ1 assignee |
| TestAgentFileTailer race is deeper than expected | Low | Low — TA3 overruns | TA3 budget 0.5 day; escalate if race requires architecture changes | TA3 assignee |
| ~20 day timeline exceeds team capacity | Medium | High — incomplete delivery | Two-release split at Gate 1 (after Wave 3) | Project lead |

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go code**: `go test -v -run <TestName>`
- **Build verification**: `go build` with correct tags
- **Matrix E2E**: bash/curl against local Docker Conduit
- **Conduit lifecycle**: `docker-compose` up/down with health checks
- **Interface confirmation**: `grep` + code reading to verify actual interfaces before implementation
- **Command-path audit**: `grep` for existing command routing in adapter code
- **Nil stub wiring**: `grep` on `main.go` to verify non-nil assignment
- **Blocking gates**: T3a/T6a/T7a design docs must be reviewed before T3/T6/T7 implementation PRs are merged

---

## Commit and Branching Strategy

- Work on `stabilization-v4` branch off `main`
- Each task is a single commit
- Commits require: build passes, new tests pass, existing tests not regressed
- **Blocking gate enforcement**: T3/T6/T7 implementation PRs include a checklist item confirming the corresponding design doc (T3a/T6a/T7a) was reviewed
- Nil-stub safety: every newly wired `rpc.Config` field is guarded, but **not all fields use the same degradation pattern** — Guard fails hard in sentinel mode, others degrade to warnings

**Guard-specific degradation pattern:**
```go
if rpcCfg.Guard != nil {
    rpcCfg.Guard.Check(method)
} else {
    if cfg.ServerMode == "sentinel" || cfg.ServerMode == "cloudflare" {
        log.Fatalf("[FATAL] Guard required in %s mode but failed to initialize", cfg.ServerMode)
    }
    log.Printf("[WARN] Guard not available, RPC rate limiting disabled")
}
```

**Standard degradation pattern (AuditLog, NavChartStore, SkillGate):**
```go
if rpcCfg.AuditLog != nil {
    rpcCfg.AuditLog.Log(...)
} else {
    log.Printf("[WARN] AuditLog not configured, skipping audit entry")
}
```

---

## Execution Waves

```
Wave 1 (Start Immediately — build fixes + audit alignment):
├── T1:  Fix build-bridge-binaries.sh CGO_ENABLED [0.25d]
├── T2:  Update .gitignore for build artifacts [0.25d]
└── T3a: Audit contract alignment [0.25d]

Wave 2 (After Wave 1 — AuditLog + interface confirmations + command audit):
├── T3:  Implement and wire AuditLog [1d, depends on T3a, BLOCKING GATE: T3a must be reviewed first]
├── T6a: Confirm NavChartStore interface [0.25d]
├── T7a: Confirm SkillGate interface [0.25d]
└── T0a: Command-path audit and route inventory [0.5d]

Wave 3 (After Wave 2 — nil stub implementations + command wiring):
├── T4:  Implement and wire Guard (mode-aware) [1d, depends on T3]
├── T5:  Implement and wire GovernanceRoomID [0.5d]
├── T6:  Implement and wire NavChartStore [1d, depends on T3, T6a, BLOCKING GATE: T6a must be reviewed first]
├── T7:  Implement and wire SkillGate [0.5d, depends on T3, T7a, BLOCKING GATE: T7a must be reviewed first]
└── T0:  Wire missing command families [1d, depends on T0a, scope = exactly what T0a confirms is missing]

═══ GATE 1 CHECKPOINT ═══
═══ Two-release split point (ship Release 1 here if needed) ═══

Wave 4 (After Gate 1 — infrastructure + deployments + integrations):
├── T8:  Create Conduit test fixtures [0.5d]
├── T9:  Verify/fix libyara-dev [0.25d, depends on T1]
├── T10: Verify/fix Python gRPC [0.25d]
├── T11: Build Conduit lifecycle harness [1.5d, depends on T8]
├── T12: Build Matrix client test library [1d, depends on T11]
├── TJ1: Deploy Java sidecar as DOC/PPT default [1d]
├── TJ2: Remove Python DOC/PPT fallback [0.5d, depends on TJ1]
├── TV1: Construct and inject MatrixManager [1.5d]
├── TA1: Wire AgentFileTailer into Secretary [1.5d]
├── TA2: Inject file-writer for agent mode [0.5d, depends on TA1, CROSS-REPO: Bridge + container/openclaw]
└── TA3: Fix AgentFileTailer race condition [0.5d]

Wave 5 (After T0 + T11 + T12 — Matrix E2E tests):
├── T15: /status E2E test [0.5d]
├── T16: !agent skills E2E test [0.5d]
├── T17: !agent forget-skill E2E test [0.5d]
├── T18: /approve and /reject E2E test [0.5d]
├── T19: !agent create/list/spawn/stop E2E test [0.5d, conditional on T0a]
└── T20: !secretary commands E2E test [0.5d, conditional on T0a]

═══ GATE 2 CHECKPOINT ═══

Wave 6 (After Gate 2 — validation + cleanup):
├── T21a: Command response validation (m.notice) [0.5d]
├── T21b: Custom event validation (workflow.* / agent.* / blocker.*) [0.5d]
└── T22: Dead-code cleanup [0.5d]

Wave FINAL (After ALL tasks — 5 parallel reviews):
├── F1: Plan compliance audit
├── F2: Code quality review
├── F3: Automated integration QA
├── F4: Scope fidelity check
└── F5: Zero-trust & security posture check
→ Present results → Get explicit user okay
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave | Blocking Gate |
|------|-----------|--------|------|---------------|
| T1 | — | T3, T9 | 1 | — |
| T2 | — | — | 1 | — |
| T3a | — | T3 | 1 | T3 blocked until T3a reviewed |
| T3 | T1, T3a | T4, T6, T7 | 2 | — |
| T6a | — | T6 | 2 | T6 blocked until T6a reviewed |
| T7a | — | T7 | 2 | T7 blocked until T7a reviewed |
| T0a | — | T0 | 2 | T0 blocked until T0a reviewed |
| T4 | T3 | — | 3 | — |
| T5 | — | — | 3 | — |
| T6 | T3, T6a | — | 3 | — |
| T7 | T3, T7a | — | 3 | — |
| T0 | T0a | T15-T20 | 3 | — |
| T8 | — | T11 | 4 | — |
| T9 | T1 | — | 4 | — |
| T10 | — | — | 4 | — |
| T11 | T8 | T12, T15-T20 | 4 | — |
| T12 | T11 | T15-T20 | 4 | — |
| TJ1 | — | TJ2 | 4 | — |
| TJ2 | TJ1 | — | 4 | — |
| TV1 | — | — | 4 | — |
| TA1 | — | TA2 | 4 | — |
| TA2 | TA1 | — | 4 | — |
| TA3 | — | — | 4 | — |
| T15 | T0, T11, T12 | T21a | 5 | — |
| T16 | T0, T11, T12 | T21a | 5 | — |
| T17 | T0, T11, T12 | T21a | 5 | — |
| T18 | T0, T11, T12 | T21a | 5 | — |
| T19 | T0, T11, T12 | T21a | 5 | — |
| T20 | T0, T11, T12 | T21a | 5 | — |
| T21a | T15-T20 | F1-F5 | 6 | — |
| T21b | T15-T20 | F1-F5 | 6 | — |
| T22 | T3-T7, T0 | F1-F5 | 6 | — |

### Critical Path

```
T1 → T3a → T3 → T0a → T0 → T8 → T11 → T12 → T15 → T21a → F1-F5
```

### Estimated Effort by Wave

| Wave | Tasks | Cumulative |
|------|-------|-----------|
| 1 | T1, T2, T3a | 0.75 |
| 2 | T3, T6a, T7a, T0a | 2.25 |
| 3 | T4, T5, T6, T7, T0 | 5.75 |
| — | **Gate 1 checkpoint** | — |
| 4 | T8-T12, TJ1, TJ2, TV1, TA1, TA2, TA3 | 14.5 |
| 5 | T15-T20 | 17.5 |
| — | **Gate 2 checkpoint** | — |
| 6 | T21a, T21b, T22 | 19 |
| FINAL | F1-F5 | 20 |

**Total: ~20 days** (or ~8 + ~12 if split into two releases at Gate 1)

---

## Definition of Done

- [x] `grep "= nil" bridge/cmd/bridge/main.go` for `rpc.Config` fields returns 0 (or only explicitly deferred items with tracking IDs)
- [x] `CGO_ENABLED=1 go build -tags "yara" ./bridge/cmd/bridge/` succeeds
- [x] `grep "build-output/" .gitignore` returns a match
- [x] `grep "go.work.sum" .gitignore` returns a match
- [x] Command-path audit completed (T0a) with machine-readable inventory
- [x] Only missing command families wired (T0), no duplication of existing dispatch
- [x] `/status` sent via Matrix room produces `m.notice` response
- [x] DOC/PPT extraction returns text, not XlsConverter errors
- [x] DOC/PPT extraction returns exact error message when Java is down: `"DOC/PPT extraction requires the Java sidecar but it is currently unavailable. Deploy sidecar-java for legacy Office format support or contact your administrator."`
- [x] `voice.start_session` returns session ID when all prerequisites met
- [x] Agent file events flow through `WorkflowEventEmitter` in live workflow
- [x] Agent-mode containers produce `agent_status.json` visible to Bridge tailer
- [x] `TestAgentFileTailer_StopsAt10MBCap` passes 50 consecutive runs
- [x] Guard fails startup in sentinel/cloudflare mode if initialization fails
- [x] Guard gracefully degrades in native/dev mode if initialization fails
- [x] `tests/matrix-e2e/harness.sh start` brings up Conduit + Bridge in under 30 seconds
- [x] All documented-command E2E tests pass
- [x] `m.notice` command responses validated separately from custom runtime events
- [x] All Go tests pass
- [x] T3a, T6a, T7a design docs reviewed before implementation PRs merged
- [x] Python is never invoked for DOC/PPT extraction (verified by test)

---

## Tasks

---

### T1 — Fix build-bridge-binaries.sh CGO_ENABLED

**What to do**:
- Read `scripts/build-bridge-binaries.sh` — find the `CGO_ENABLED=0` line
- Change to `CGO_ENABLED=1` — SQLCipher and YARA both require CGO
- Verify: `CGO_ENABLED=1 go build -tags "yara" ./bridge/cmd/bridge/`
- If additional flags needed, add with comments

**Must NOT do**:
- Do NOT change the Docker build (already correct)
- Do NOT add CGO dependencies to non-bridge targets

**References**: `scripts/build-bridge-binaries.sh:1-50`, `bridge/Dockerfile:25-40`

**Acceptance Criteria**:
- [x] `scripts/build-bridge-binaries.sh` does not contain `CGO_ENABLED=0`
- [x] `CGO_ENABLED=1 go build -tags "yara" ./bridge/cmd/bridge/` succeeds (if libyara-dev installed)

**QA Scenarios**:
```
Scenario: Build script uses CGO
  Tool: Bash
  Steps:
    1. grep "CGO_ENABLED" scripts/build-bridge-binaries.sh
    2. Assert: output does NOT contain "CGO_ENABLED=0"
  Expected Result: Script does not disable CGO
  Evidence: .sisyphus/evidence/task-1-build-cgo.txt
```

**Commit**: `fix(build): set CGO_ENABLED=1 in build-bridge-binaries.sh`

---

### T2 — Update .gitignore for build artifacts

**What to do**:
- Add `build-output/`, `go.work.sum`, `bridge/armorclaw-bridge` to `.gitignore` if missing

**Must NOT do**: Do NOT remove `doc/` or `.sisyphus/` from `.gitignore`

**Acceptance Criteria**:
- [x] `grep "build-output/" .gitignore` returns a match
- [x] `grep "go.work.sum" .gitignore` returns a match

**Commit**: `chore: add build artifacts to .gitignore`

---

### T3a — Audit contract alignment [BLOCKING GATE]

**What to do**:
- Read `bridge/pkg/rpc/server.go` — find the `rpc.Config.AuditLog` field type and interface definition (every method signature, parameter type, return type)
- Read `bridge/pkg/audit/` — determine what already exists (types, interfaces, implementations)
- Read the architecture docs — confirm whether the documented audit model uses `audit.db` (SQLCipher) or file-based logging
- Determine the correct backing implementation:
  - **Option A**: `rpc.Config.AuditLog` expects an interface that the existing `audit.db` model can satisfy → implement an adapter wrapping the existing model
  - **Option B**: `rpc.Config.AuditLog` expects a simple logging interface → implement a JSON-lines adapter, documented as a **runtime adapter** (not a replacement for `audit.db`)
  - **Option C**: Both → implement a composite adapter that writes to both
- Document the decision in `bridge/pkg/audit/DESIGN.md` with:
  - Interface definition (method signatures with parameter types and return types)
  - Chosen backing implementation and rationale
  - How this relates to the existing `audit.db` model
  - Config additions needed

**Must NOT do**:
- Do NOT replace `audit.db` without updating architecture docs to match
- Do NOT write any implementation code — this is an investigation and design task only

**Blocking gate**: T3 implementation PR cannot be merged until this design doc is reviewed and approved.

**References**:
- `bridge/pkg/rpc/server.go` — `rpc.Config` struct, AuditLog field type
- `bridge/pkg/audit/` — Existing audit package
- Architecture docs on audit logging

**Acceptance Criteria**:
- [x] `rpc.Config.AuditLog` interface fully documented (method signatures, parameter types, return types)
- [x] Decision documented: which backing implementation to use and why
- [x] Relationship to existing `audit.db` model documented
- [x] No implementation code written

**QA Scenarios**:
```
Scenario: AuditLog interface documented
  Tool: Bash
  Steps:
    1. cat bridge/pkg/audit/DESIGN.md
  Expected Result: File exists with interface definition and backing decision
  Evidence: .sisyphus/evidence/task-3a-audit-design.txt

Scenario: Interface matches rpc.Config
  Tool: Bash
  Steps:
    1. grep "AuditLog" bridge/pkg/rpc/server.go | head -5
    2. Compare against documented interface in DESIGN.md
  Expected Result: Documented interface matches what rpc.Config expects
  Evidence: .sisyphus/evidence/task-3a-audit-interface.txt
```

**Commit**: `docs(audit): document AuditLog interface and backing decision`

---

### T3 — Implement and wire AuditLog

**What to do**:
- Based on T3a's decision, implement the concrete `AuditLog` in `bridge/pkg/audit/audit.go`
- If T3a determined JSON-lines is correct:
  - Structured JSON lines to configurable directory
  - Fields: timestamp, operation, user_id, agent_id, success, duration_ms, detail
  - Rotation at configurable size
  - Fallback: `NewStderrAuditLog()` when log directory unwritable
  - Document this as a **runtime adapter** for the `AuditLog` interface, not a replacement for `audit.db`
- If T3a determined `audit.db` is correct:
  - Implement adapter wrapping the existing audit database model
  - Satisfy the `rpc.Config.AuditLog` interface
- Wire in `main.go`:
  ```go
  auditLogger, err := audit.NewAuditLog(audit.Config{...})
  if err != nil {
      log.Printf("[WARN] audit logger init failed: %v, degrading to stderr", err)
      auditLogger = audit.NewStderrAuditLog()
  }
  rpcCfg.AuditLog = auditLogger
  ```
- Add nil-check guards in at least 3 RPC handlers
- Add config section if needed
- Create unit tests

**Must NOT do**:
- Do NOT use SQLCipher for JSON-lines adapter (no secrets in audit log files)
- Do NOT block Bridge startup if audit log dir is unwritable
- Do NOT replace `audit.db` without updating architecture docs

**Blocking gate prerequisite**: T3a design doc must be reviewed and approved before this PR is merged.

**References**: T3a design document, `bridge/cmd/bridge/main.go:2630-2634`, `bridge/pkg/rpc/server.go`

**Acceptance Criteria**:
- [x] Concrete type satisfies `rpc.Config.AuditLog` interface (confirmed by T3a)
- [x] `main.go` assigns non-nil AuditLog to `rpcCfg.AuditLog`
- [x] `go test ./bridge/pkg/audit/...` passes
- [x] Bridge starts even if audit log dir is unwritable
- [x] At least 3 RPC handlers consume AuditLog with nil-check guards

**QA Scenarios**:
```
Scenario: AuditLog writes entry
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestAuditLog_WritesEntry ./pkg/audit/...
  Expected Result: PASS
  Evidence: .sisyphus/evidence/task-3-audit-write.txt

Scenario: AuditLog degrades gracefully
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestAuditLog_DegradedMode ./pkg/audit/...
  Expected Result: PASS, no fatal exit
  Evidence: .sisyphus/evidence/task-3-audit-degrade.txt

Scenario: AuditLog wired in main.go
  Tool: Bash
  Steps:
    1. grep "rpcCfg.AuditLog" bridge/cmd/bridge/main.go
    2. Assert: output does NOT contain "= nil" without tracking ID
  Expected Result: Non-nil assignment
  Evidence: .sisyphus/evidence/task-3-audit-wired.txt
```

**Commit**: `feat(audit): implement AuditLog per T3a design`

---

### T6a — Confirm NavChartStore interface and caller expectations [BLOCKING GATE]

**What to do**:
- Read `bridge/pkg/rpc/server.go` — find `rpc.Config.NavChartStore` field type and interface definition
- Read `bridge/pkg/browser/` — find existing NavChart types and any existing store interface
- Read JetskiBroker code — find where NavChart data is produced and what it calls
- Document in `bridge/pkg/browser/navchart_design.md`:
  - Method signatures on the `NavChartStore` interface
  - All callers (JetskiBroker, `browser.replay_diagnostics`) and expected method calls
  - NavChart data type structure

**Must NOT do**: Do NOT implement anything. Do NOT invent interfaces not confirmed in code.

**Blocking gate**: T6 implementation PR cannot be merged until this design doc is reviewed and approved.

**Acceptance Criteria**:
- [x] `NavChartStore` interface fully documented with method signatures
- [x] All callers identified with expected method calls
- [x] NavChart data type structure documented

**Commit**: `docs(browser): document NavChartStore interface and callers`

---

### T7a — Confirm SkillGate interface and execution points [BLOCKING GATE]

**What to do**:
- Read `bridge/pkg/rpc/server.go` — find `rpc.Config.SkillGate` field type and interface definition
- Read `bridge/pkg/skills/` — find existing skill registry, store, and execution flow
- Read `bridge/pkg/secretary/orchestrator_integration.go` — find where `injectLearnedSkills()` is called
- Document in `bridge/pkg/skills/skill_gate_design.md`:
  - Method signatures on the `SkillGate` interface
  - Where skill execution is gated (RPC handler? StepExecutor? Both?)
  - How allow/block lists interact with existing `skills.allow`/`skills.block` RPC methods

**Must NOT do**: Do NOT implement anything. Do NOT invent interfaces not confirmed in code.

**Blocking gate**: T7 implementation PR cannot be merged until this design doc is reviewed and approved.

**Acceptance Criteria**:
- [x] `SkillGate` interface fully documented with method signatures
- [x] All execution/gating points identified
- [x] Interaction with existing `skills.allow`/`skills.block` RPC documented

**Commit**: `docs(skills): document SkillGate interface and gating points`

---

### T0a — Command-path audit and route inventory

**What to do**:
- Read `bridge/internal/adapter/adapter.go` — find `processEvents()` and trace the full Matrix event routing
- Read `bridge/internal/adapter/commands_integration.go` — understand `CommandHandler`/`AdminCommandHandler` and all registered commands
- For each command prefix, determine:
  - Is it routed in `processEvents()`? (Yes/No)
  - What handler does it reach? (CommandHandler method name)
  - Does it produce an `m.notice` response? (Yes/No)
- Expected command families to inventory:
  - `/claim_admin`, `/status`, `/verify`, `/approve`, `/reject`, `/help`
  - `!agent skills`, `!agent forget-skill`
  - `!agent create`, `!agent list`, `!agent spawn`, `!agent stop`
  - `!secretary list`, `!secretary run`, `!secretary status`, `!secretary cancel`
  - `!ai ...`
- Write the inventory to `bridge/internal/adapter/COMMAND_INVENTORY.md`
- **Also produce a machine-readable version** at `bridge/internal/adapter/COMMAND_INVENTORY.json`:
  ```json
  [
    {"command": "/status", "routed": true, "handler": "CommandHandler.handleStatus", "mnotice": true, "status": "live"},
    {"command": "!agent create", "routed": false, "handler": null, "mnotice": false, "status": "missing"},
    ...
  ]
  ```
- This inventory determines the **exact scope** of T0: only wire the commands with `"status": "missing"` or `"status": "broken"`

**Must NOT do**:
- Do NOT wire anything — this is an audit task only
- Do NOT modify any code
- Do NOT assume all commands are dead or all are live

**References**: `bridge/internal/adapter/adapter.go`, `bridge/internal/adapter/commands_integration.go`, `bridge/pkg/matrixcmd/`

**Acceptance Criteria**:
- [x] Complete inventory in `COMMAND_INVENTORY.md` with columns: Command | Routed | Handler | m.notice | Status
- [x] Machine-readable `COMMAND_INVENTORY.json` with same data
- [x] Each command categorized as Live / Missing / Broken
- [x] Clear list of which command families T0 must wire (the Missing/Broken ones only)

**QA Scenarios**:
```
Scenario: Command inventory complete
  Tool: Bash
  Steps:
    1. cat bridge/internal/adapter/COMMAND_INVENTORY.md
    2. cat bridge/internal/adapter/COMMAND_INVENTORY.json | jq length
  Expected Result: Both files exist, JSON has ≥10 entries
  Evidence: .sisyphus/evidence/task-0a-command-inventory.txt

Scenario: JSON is valid and parseable
  Tool: Bash
  Steps:
    1. cat bridge/internal/adapter/COMMAND_INVENTORY.json | jq '.[] | select(.status=="missing") | .command'
  Expected Result: Lists missing commands (may be empty if all are live)
  Evidence: .sisyphus/evidence/task-0a-missing-commands.txt
```

**Commit**: `docs(matrix): document command-path audit and route inventory`

---

### T4 — Implement and wire Guard (mode-aware)

**What to do**:
- Read `bridge/pkg/rpc/server.go` — confirm Guard interface
- Read `bridge/pkg/governor/` — check for existing types
- Implement `bridge/pkg/governor/guard.go`:
  - Per-RPC-method rate limiting (configurable RPS per method group)
  - Method groups: `default`, `container`, `ai`, `health` (exempt)
  - Token bucket with configurable burst
  - All decisions logged to AuditLog
  - **Mode-aware fail behavior**:
    - **Sentinel/Cloudflare mode**: `log.Fatalf` if Guard initialization fails — Bridge must not run without rate limiting in exposed deployments
    - **Native/Dev mode**: `log.Printf` warning and continue — rate limiting is optional in local deployments
- Rate-limited calls return `-32006` with method name and limit
- Wire in `main.go` with mode-aware degradation
- Add config section
- Create unit tests including mode-specific tests

**Must NOT do**:
- Do NOT use the same degradation pattern for Guard as for AuditLog/NavChartStore/SkillGate
- Do NOT silently degrade Guard in sentinel mode
- Do NOT add per-user or per-operation gating (out of scope)

**References**: `bridge/pkg/rpc/server.go`, `bridge/pkg/governor/`, `bridge/pkg/config/config.go` (ServerMode)

**Acceptance Criteria**:
- [x] `bridge/pkg/governor/guard.go` exists
- [x] `rpcCfg.Guard` is non-nil at Bridge startup
- [x] Rate-limited calls return `-32006`
- [x] `health.check` is NOT rate-limited
- [x] In sentinel/cloudflare mode: Bridge **fails to start** if Guard init fails
- [x] In native/dev mode: Bridge starts with `[WARN]` if Guard init fails
- [x] `go test ./bridge/pkg/governor/...` passes

**QA Scenarios**:
```
Scenario: Guard allows under limit
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestGuard_AllowsUnderLimit ./pkg/governor/...
  Expected Result: PASS
  Evidence: .sisyphus/evidence/task-4-guard-allow.txt

Scenario: Guard blocks over limit
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestGuard_BlocksOverLimit ./pkg/governor/...
  Expected Result: PASS, -32006 error code
  Evidence: .sisyphus/evidence/task-4-guard-block.txt

Scenario: Guard fails startup in sentinel mode
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestGuard_SentinelModeFailure ./pkg/governor/...
  Expected Result: PASS, fatal exit on Guard init failure in sentinel mode
  Evidence: .sisyphus/evidence/task-4-guard-sentinel.txt

Scenario: Guard degrades in native mode
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestGuard_NativeModeDegradation ./pkg/governor/...
  Expected Result: PASS, warning logged but Bridge continues
  Evidence: .sisyphus/evidence/task-4-guard-native.txt

Scenario: Health check exempt from rate limiting
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestGuard_HealthCheckExempt ./pkg/governor/...
  Expected Result: PASS, health.check never rate-limited
  Evidence: .sisyphus/evidence/task-4-guard-health.txt
```

**Commit**: `feat(governor): implement Guard with mode-aware fail behavior`

---

### T5 — Implement and wire GovernanceRoomID

**What to do**:
- Config-first with auto-create fallback
- Auto-created room ID persisted to state file (`/var/lib/armorclaw/.governance-room`), not config file
- Graceful degradation if Matrix client unavailable

**Acceptance Criteria**:
- [x] `rpcCfg.GovernanceRoomID` is a valid room ID or empty with logged warning
- [x] Config field `governance.room_id` exists
- [x] Room ID persists to state file across restarts
- [x] Bridge starts even if Matrix client unavailable

**QA Scenarios**:
```
Scenario: GovernanceRoomID from config
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestGovernanceRoomID_ConfigProvided ./pkg/...
  Expected Result: PASS, uses config value
  Evidence: .sisyphus/evidence/task-5-governance-config.txt

Scenario: GovernanceRoomID auto-create when config empty
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestGovernanceRoomID_AutoCreate ./pkg/...
  Expected Result: PASS, creates room and persists to state file
  Evidence: .sisyphus/evidence/task-5-governance-autocreate.txt

Scenario: GovernanceRoomID graceful when Matrix unavailable
  Tool: Bash
  Steps:
    1. grep "governance" bridge/cmd/bridge/main.go | grep -i "warn"
  Expected Result: Warning logged when Matrix client nil or room creation fails
  Evidence: .sisyphus/evidence/task-5-governance-graceful.txt

Scenario: State file persists across restarts
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestGovernanceRoomID_StateFilePersist ./pkg/...
  Expected Result: PASS, second start reads state file
  Evidence: .sisyphus/evidence/task-5-governance-persist.txt
```

**Commit**: `feat(matrix): resolve GovernanceRoomID with config + auto-create + state file`

---

### T6 — Implement and wire NavChartStore

**What to do**:
- Based on T6a's confirmed interface, implement `bridge/pkg/browser/navchart_store.go`
- SQLite persistence (plain, not SQLCipher — no secrets in nav charts)
- PII scan before storage (basic regex)
- Max chart count with LRU eviction
- All operations logged to AuditLog
- Graceful degradation if DB init fails

**Must NOT do**: Do NOT implement interfaces not confirmed by T6a. Do NOT use SQLCipher.

**Blocking gate prerequisite**: T6a design doc must be reviewed and approved before this PR is merged.

**Acceptance Criteria**:
- [x] Implementation matches confirmed interface from T6a
- [x] `rpcCfg.NavChartStore` is non-nil (or nil with logged warning)
- [x] `go test ./bridge/pkg/browser/...` passes
- [x] Bridge starts even if navchart DB is unwritable

**QA Scenarios**:
```
Scenario: NavChartStore round-trip
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestNavChartStore_SaveAndRetrieve ./pkg/browser/...
  Expected Result: PASS
  Evidence: .sisyphus/evidence/task-6-navchart-roundtrip.txt

Scenario: NavChartStore rejects PII
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestNavChartStore_RejectsPII ./pkg/browser/...
  Expected Result: PASS, chart with SSN pattern rejected
  Evidence: .sisyphus/evidence/task-6-navchart-pii.txt

Scenario: Graceful degradation on DB failure
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestNavChartStore_GracefulDegradation ./pkg/browser/...
  Expected Result: PASS, no fatal exit on unwritable path
  Evidence: .sisyphus/evidence/task-6-navchart-degrade.txt
```

**Commit**: `feat(browser): implement NavChartStore per T6a design`

---

### T7 — Implement and wire SkillGate

**What to do**:
- Based on T7a's confirmed interface, implement `bridge/pkg/skills/skill_gate.go`
- `Check(skillName string) (allowed bool, reason string)`
- Gate logic: BlockList → AllowList → DefaultPolicy
- All deny decisions logged to AuditLog
- Wire to StepExecutor and `skills.execute` RPC handler per T7a's confirmed execution points

**Must NOT do**: Do NOT implement interfaces not confirmed by T7a. Do NOT add dynamic skill loading or network validation.

**Blocking gate prerequisite**: T7a design doc must be reviewed and approved before this PR is merged.

**Acceptance Criteria**:
- [x] Implementation matches confirmed interface from T7a
- [x] `rpcCfg.SkillGate` is non-nil
- [x] Blocked skill returns deny with reason
- [x] `go test ./bridge/pkg/skills/...` passes

**QA Scenarios**:
```
Scenario: SkillGate blocks listed skill
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestSkillGate_BlockedSkill ./pkg/skills/...
  Expected Result: PASS
  Evidence: .sisyphus/evidence/task-7-skillgate-block.txt

Scenario: SkillGate allows unknown with default allow
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestSkillGate_DefaultAllow ./pkg/skills/...
  Expected Result: PASS
  Evidence: .sisyphus/evidence/task-7-skillgate-allow.txt

Scenario: SkillGate denies with default deny
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestSkillGate_DefaultDeny ./pkg/skills/...
  Expected Result: PASS
  Evidence: .sisyphus/evidence/task-7-skillgate-deny.txt
```

**Commit**: `feat(skills): implement SkillGate per T7a design`

---

### T0 — Wire missing command families into processEvents()

**What to do**:
- Read `bridge/internal/adapter/COMMAND_INVENTORY.json` from T0a
- For each command with `"status": "missing"` or `"status": "broken"`:
  - Add prefix check in `processEvents()`
  - Route to the appropriate handler method
  - Ensure handler sends `m.notice` response
- Do NOT touch commands with `"status": "live"` — they already work
- Add guard: don't process bridge bot's own messages
- Add guard: only process commands from room members

**Scope is exactly what T0a confirms is missing — no more, no less.**

**Must NOT do**:
- Do NOT wire AdminCommandHandler unconditionally
- Do NOT duplicate existing command dispatch
- Do NOT modify existing handler implementations
- Do NOT process bridge bot's own messages
- Do NOT exceed the scope defined by T0a's inventory

**References**: `COMMAND_INVENTORY.json` from T0a, `adapter.go`, `commands_integration.go`

**Acceptance Criteria**:
- [x] All commands identified as "Missing" in T0a now produce `m.notice` responses
- [x] Commands identified as "Live" in T0a still work (no regression)
- [x] Bridge bot's own `m.notice` messages are not re-processed
- [x] Non-command messages are unaffected

**QA Scenarios**:
```
Scenario: Each missing command now produces m.notice
  Tool: Go test
  Steps:
    1. For each command in COMMAND_INVENTORY.json with status "missing":
       - Send command as Matrix room message
       - Verify m.notice response received
  Expected Result: All previously-missing commands now respond
  Evidence: .sisyphus/evidence/task-0-missing-commands-live.txt

Scenario: Existing commands still work
  Tool: Go test
  Steps:
    1. For each command in COMMAND_INVENTORY.json with status "live":
       - Send command as Matrix room message
       - Verify m.notice response still received
  Expected Result: No regression
  Evidence: .sisyphus/evidence/task-0-existing-regression.txt

Scenario: No infinite loop
  Tool: Go test
  Steps:
    1. Send command, wait for m.notice response
    2. Verify bridge bot doesn't re-process its own m.notice
  Expected Result: Exactly one command invocation
  Evidence: .sisyphus/evidence/task-0-no-loop.txt
```

**Commit**: `feat(matrix): wire missing command families per T0a inventory`

---

### T8 — Create Conduit test fixtures

**What to do**:
- Create `tests/matrix-test-server/nginx.conf` — reverse proxy for Conduit
- Create `tests/matrix-test-server/wellknown/server` — `{"m.server": "localhost:8448"}`
- Create `tests/matrix-test-server/wellknown/client` — `{"m.homeserver": {"base_url": "http://localhost:8008"}}`
- Verify docker-compose up/down works

**Acceptance Criteria**:
- [x] All three fixtures exist
- [x] `docker-compose up -d` succeeds
- [x] `curl -sf http://localhost:8008/_matrix/client/versions` returns valid JSON
- [x] `docker-compose down -v` cleans up, no orphaned containers

**Commit**: `test(matrix): add missing Conduit test fixtures`

---

### T9 — Verify/fix libyara-dev in Docker builds

**What to do**: Verify `libyara-dev` in Dockerfiles. Add to docs if missing.

**Acceptance Criteria**:
- [x] `libyara-dev` installation present in Dockerfile(s)
- [x] Build prerequisite documented

**Commit**: (only if changes needed)

---

### T10 — Verify/fix Python gRPC setup

**What to do**: Verify `grpcio`/`grpcio-tools` in requirements.txt and Dockerfile. Add if missing.

**Acceptance Criteria**:
- [x] gRPC packages present in requirements.txt
- [x] Dockerfile installs them

**Commit**: (only if changes needed)

---

### T11 — Build Conduit lifecycle harness

**What to do**:
- Create `tests/matrix-e2e/` directory structure
- Implement `harness.sh` (start/stop/status)
- Implement `lib/conduit.sh` (start/stop/health/register/create room/invite bot)
- Implement `lib/bridge.sh` (start/stop/health)
- Implement `lib/assertions.sh` (assert_notice, assert_json, etc.)
- Implement `run-test.sh` (test runner with cleanup trap)
- Create `fixtures/test-config.toml` and `fixtures/test-users.json`

**Acceptance Criteria**:
- [x] `harness.sh start` brings up Conduit + Bridge in under 30s
- [x] `harness.sh stop` tears down cleanly, no orphaned containers
- [x] User registration works
- [x] All scripts are executable

**Commit**: `test(matrix): build Conduit lifecycle harness`

---

### T12 — Build Matrix client test library

**What to do**:
- Create `tests/matrix-e2e/lib/matrix-client.sh` with 8 functions:
  - `matrix_login`, `matrix_send`, `matrix_sync`, `matrix_poll_notice`
  - `matrix_create_room`, `matrix_invite`, `matrix_join`, `matrix_get_messages`
- Each uses `curl -sf` + `jq`, returns data via `echo`

**Acceptance Criteria**:
- [x] All 8 functions exported
- [x] `matrix_login` returns access token
- [x] `matrix_send` returns event_id
- [x] `matrix_poll_notice` finds m.notice within timeout

**Commit**: `test(matrix): build Matrix client test library`

---

### TJ1 — Deploy Java sidecar as DOC/PPT default

**What to do**:
- Add Java sidecar container to Docker Compose with same hardening as Python sidecar
- Update `RouteExtractText()`: DOC/PPT → `javaClient.ExtractText()` as primary
- If `javaClient` is nil: return **exact error message**:
  ```
  "DOC/PPT extraction requires the Java sidecar but it is currently unavailable. Deploy sidecar-java for legacy Office format support or contact your administrator."
  ```
- **Rollout strategy**: Use the existing dedicated Java compose path first. Merge into main deployment only after validation confirms DOC/PPT extraction succeeds.
- Verify `ExtractionMode` reports `java_primary`

**Critical design decision**: Do NOT silently fall back to Python for DOC/PPT. A clear, specific error is better than a silent broken fallback.

**Acceptance Criteria**:
- [x] DOC/PPT extraction returns text in default Docker Compose deployment
- [x] DOC/PPT extraction returns exact error message when Java sidecar is down
- [x] MSG/XLS still route to Python

**QA Scenarios**:
```
Scenario: DOC extraction with Java sidecar
  Tool: Bash
  Steps:
    1. Send DOC file to ExtractText endpoint
    2. Check response contains text
  Expected Result: Text extraction succeeds
  Evidence: .sisyphus/evidence/task-j1-doc-extract.txt

Scenario: DOC extraction without Java sidecar returns exact error
  Tool: Bash
  Steps:
    1. Send DOC file to ExtractText endpoint with Java sidecar down
    2. Check response contains exact error message
  Expected Result: "DOC/PPT extraction requires the Java sidecar but it is currently unavailable. Deploy sidecar-java for legacy Office format support or contact your administrator."
  Evidence: .sisyphus/evidence/task-j1-doc-no-java.txt

Scenario: MSG still routes to Python
  Tool: Bash
  Steps:
    1. Send MSG file to ExtractText endpoint
    2. Check response contains text
  Expected Result: MSG extraction works via Python
  Evidence: .sisyphus/evidence/task-j1-msg-python.txt
```

**Commit**: `feat(sidecar): deploy Java sidecar as DOC/PPT default routing target`

---

### TJ2 — Remove Python DOC/PPT fallback

**What to do**:
- Remove Python from DOC/PPT routing in `RouteExtractText()`
- Update Python `FORMAT_MAP` if it claims DOC/PPT
- Python handles only: MSG, XLS
- **Add explicit test** verifying Python is NOT called for DOC/PPT when Java is down:
  ```go
  func TestRouteExtractText_DOC_PythonNotCalledWhenJavaDown(t *testing.T) {
      // Verify that DOC extraction with Java down does NOT invoke Python client
      // The error must come from the Go routing layer, not from a Python XlsConverter error
  }
  ```

**Final routing table**:

| Format | Route |
|--------|-------|
| DOCX, XLSX, PPTX, PDF | Rust |
| DOC, PPT | Java (exact error message if down) |
| MSG, XLS | Python |
| Plain text | Native Go |

**Acceptance Criteria**:
- [x] DOC/PPT never degrades to Python XlsConverter
- [x] Test explicitly verifies Python is not called for DOC/PPT
- [x] MSG/XLS still works via Python

**QA Scenarios**:
```
Scenario: DOC never routes to Python
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestRouteExtractText_DOC_RoutesToJava ./pkg/sidecar/...
  Expected Result: PASS
  Evidence: .sisyphus/evidence/task-j2-doc-java.txt

Scenario: Python not called for DOC when Java is down
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestRouteExtractText_DOC_PythonNotCalledWhenJavaDown ./pkg/sidecar/...
  Expected Result: PASS, error comes from Go routing layer, not Python
  Evidence: .sisyphus/evidence/task-j2-python-not-called.txt
```

**Commit**: `feat(sidecar): remove Python DOC/PPT fallback, Java-only for legacy Office`

---

### TV1 — Construct and inject MatrixManager

**What to do**:
- Read `bridge/pkg/voice/matrix.go` — verify `MatrixManager` constructor signatures match what `Manager` expects (check for stale API from being "implemented but unwired")
- In `main.go`, after Matrix client initialization:
  ```go
  var voiceMatrixMgr *voice.MatrixManager
  if matrixClient != nil {
      voiceMatrixMgr = voice.NewMatrixManager(voice.MatrixManagerConfig{
          MatrixClient: matrixClient,
      })
  } else {
      log.Printf("[WARN] voice: Matrix client unavailable, call signaling disabled")
  }
  voiceMgr.SetMatrixManager(voiceMatrixMgr)
  ```

**Dependency note**: TV1 depends on **Matrix client availability** (initialized earlier in `main.go`), NOT on GovernanceRoomID (T5). The v3 plan's dependency was incorrect and has been fixed.

**Acceptance Criteria**:
- [x] `voiceMgr` has non-nil `matrixManager` when Matrix client is available
- [x] `voice.start_session` succeeds when all prerequisites met (flag + TURN + OpenAI + Matrix)
- [x] `voice.start_session` returns `-32007` with `matrix_unwired` when Matrix client is nil
- [x] No nil-pointer panic in any voice path

**QA Scenarios**:
```
Scenario: MatrixManager constructed with live client
  Tool: Bash
  Steps:
    1. grep "voiceMgr.SetMatrixManager" bridge/cmd/bridge/main.go
    2. Assert: argument is non-nil construction (not literal nil)
  Expected Result: SetMatrixManager called with constructed MatrixManager
  Evidence: .sisyphus/evidence/task-v1-matrixmanager-wired.txt

Scenario: Voice session returns -32007 when Matrix unavailable
  Tool: Go test
  Steps:
    1. cd bridge && go test -v -run TestVoice_MatrixUnwired ./pkg/voice/...
  Expected Result: PASS, returns -32007 with reason matrix_unwired
  Evidence: .sisyphus/evidence/task-v1-voice-matrix-unwired.txt
```

**Commit**: `feat(voice): construct and inject MatrixManager with live Matrix client`

---

### TA1 — Wire AgentFileTailer into Secretary StepExecutor

**What to do**:
- Modify `waitForCompletion()` to integrate tailer alongside `EventReader`
- `AgentFileTailer` and `EventReader` are parallel event sources feeding into `WorkflowEventEmitter`
- New event type: `workflow.agent_status`

**Acceptance Criteria**:
- [x] `AgentFileTailer` constructed inside `waitForCompletion()`
- [x] Agent file events flow through `WorkflowEventEmitter`
- [x] Step-mode workflows still work (no regression)
- [x] `go test ./bridge/pkg/secretary/...` passes

**Commit**: `feat(secretary): wire AgentFileTailer into StepExecutor`

---

### TA2 — Inject file-writer behavior for agent mode (no STEP_CONFIG) [CROSS-REPO]

**What to do**:
- In `StepExecutor.executeStep()`, detect agent mode (no `STEP_CONFIG`)
- Inject environment variables to activate the file writer:
  - `AGENT_FILE_WRITER_ENABLED=1`
  - `AGENT_STATUS_DIR=<stateDir>`
- In agent entrypoint (`container/openclaw/entrypoint.ts`), check for `AGENT_FILE_WRITER_ENABLED` and initialize `agent_file_writer.py`
- Agent file writer writes `agent_status.json` on: initialization (running), error (error + detail), completion (completed + summary)
- Bridge tailer (already wired in TA1) consumes these files regardless of mode

**Cross-repo coordination note**: This task requires changes in two codebases:
1. `bridge/pkg/secretary/orchestrator_integration.go` — inject env vars into container config
2. `container/openclaw/entrypoint.ts` — read env vars and activate file writer

Both changes must be coordinated and tested together. The Bridge change is safe to merge independently (env vars are ignored by containers that don't read them), but the agent-mode backward channel won't work until both are deployed.

**Why this is in scope**: Without it, Phase D only solves the backward channel for step-mode containers. Agent-mode containers still have only exit codes. The plan claims to solve the agent backward channel; this is required for that claim to be true.

**Acceptance Criteria**:
- [x] Agent-mode containers produce `agent_status.json`
- [x] Bridge tailer reads agent-mode status
- [x] No behavior change for step-mode containers

**QA Scenarios**:
```
Scenario: Agent-mode container produces agent_status.json
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestAgentMode_ProducesStatusFile ./pkg/secretary/...
  Expected Result: PASS, agent_status.json appears in state dir
  Evidence: .sisyphus/evidence/task-a2-agent-mode-status.txt

Scenario: Step-mode container unaffected
  Tool: Bash
  Steps:
    1. cd bridge && go test -v -run TestStepMode_UnaffectedByAgentWriter ./pkg/secretary/...
  Expected Result: PASS, no behavior change
  Evidence: .sisyphus/evidence/task-a2-step-mode-regression.txt
```

**Commit**: `feat(agent): inject file-writer for agent-mode containers`

---

### TA3 — Fix `TestAgentFileTailer_StopsAt10MBCap` race condition

**What to do**:
- Read the test and identify the race (likely: tailer goroutine and test assertion competing on file state)
- Fix with proper synchronization (WaitGroup, channel signal, or deterministic test ordering)
- Verify by running 50 times:
  ```bash
  go test -count=50 -run TestAgentFileTailer_StopsAt10MBCap ./pkg/agent/...
  ```

**Why this is in scope**: The tailer is being wired into the Secretary (TA1). A race condition in the tailer's test means the tailer itself may have a race condition. Must be fixed before relying on the tailer in production workflows.

**Acceptance Criteria**:
- [x] Test passes 50 consecutive runs with no flake
- [x] 10 MB cap behavior still covered

**Commit**: `fix(agent): resolve AgentFileTailer test race condition`

---

### T15 — /status E2E test

**What to do**:
- Create `tests/matrix-e2e/cases/test-status.sh`
- Test flow: start harness → login → create room → invite bot → send `/status` → poll for m.notice → teardown

**Why first**: `/status` is a documented admin command that should already be routed. Simplest test to validate the full E2E path.

**Acceptance Criteria**:
- [x] `./run-test.sh cases/test-status.sh` returns PASS
- [x] m.notice response contains status information

**Commit**: `test(matrix): /status E2E test`

---

### T16 — !agent skills E2E test

**What to do**:
- Create `tests/matrix-e2e/cases/test-agent-skills.sh`
- Send `!agent skills test-agent-1` → poll for m.notice → teardown

**Why second**: Explicitly documented in architecture as a Matrix command handled by CommandHandler.

**Acceptance Criteria**:
- [x] m.notice response received (even if empty skill list)

**Commit**: `test(matrix): !agent skills E2E test`

---

### T17 — !agent forget-skill E2E test

**What to do**:
- Create `tests/matrix-e2e/cases/test-agent-forget-skill.sh`
- Send `!agent forget-skill test-agent-1 ls_xxx_123` → poll for m.notice → teardown

**Why third**: Also documented in architecture. Tests a second `!agent` subcommand.

**Acceptance Criteria**:
- [x] m.notice response received (even if skill not found)

**Commit**: `test(matrix): !agent forget-skill E2E test`

---

### T18 — /approve and /reject E2E test

**What to do**:
- Create `tests/matrix-e2e/cases/test-approve-reject.sh`
- Create pending PII request via RPC → send `/approve <id>` → poll for m.notice → create another → send `/reject <id>` → poll for m.notice → teardown

**Why fourth**: `/approve` and `/reject` are documented admin commands. Tests HITL approval flow through Matrix.

**Acceptance Criteria**:
- [x] `/approve` produces m.notice confirming approval
- [x] `/reject` produces m.notice confirming rejection

**Commit**: `test(matrix): /approve /reject E2E test`

---

### T19 — !agent create/list/spawn/stop E2E test (conditional on T0a)

**What to do**:
- Create `tests/matrix-e2e/cases/test-agent-commands.sh`
- **Only implement if T0a confirms these commands exist (or are wired by T0)**
- If T0a shows they don't exist and T0 didn't wire them, skip this test and mark as deferred in the test file with a clear reason
- Test flow (if commands exist):
  1. Send `!agent create test-agent-$(date +%s)` → poll for m.notice
  2. Send `!agent list` → poll for m.notice
  3. Send `!agent spawn <name>` → poll for m.notice
  4. Send `!agent stop <name>` → poll for m.notice

**Acceptance Criteria**:
- [x] Test file exists (even if skipped)
- [x] If commands exist: all produce m.notice responses
- [x] If commands don't exist: test is marked as skipped with clear reason

**Commit**: `test(matrix): !agent commands E2E test`

---

### T20 — !secretary commands E2E test (conditional on T0a)

**What to do**:
- Create `tests/matrix-e2e/cases/test-secretary-commands.sh`
- **Only implement if T0a confirms these commands exist (or are wired by T0)**
- Test flow (if commands exist):
  1. Send `!secretary list` → poll for m.notice
  2. Send `!secretary status` → poll for m.notice
  3. Send `!secretary run <template>` → poll for m.notice (handle "no templates" gracefully)

**Acceptance Criteria**:
- [x] Test file exists (even if skipped)
- [x] If commands exist: all produce m.notice responses
- [x] If commands don't exist: test is marked as skipped with clear reason

**Commit**: `test(matrix): !secretary commands E2E test`

---

### T21a — Command response validation (m.notice)

**What to do**:
- Create `tests/matrix-e2e/cases/test-mnotice-responses.sh`
- For each command tested in T15-T20, validate the `m.notice` response:
  - `msgtype == "m.notice"` in the Matrix event envelope
  - Response body is non-empty
  - Response body matches expected user-facing text contract
- This validates the **command reply contract**: Matrix room message → Bridge handler → `m.notice` response

**Why separate from T21b**: Command responses are `m.notice` Matrix messages (user-facing text). Custom runtime events are `workflow.*`/`agent.*` structured JSON events. Different contracts, different consumers, different validation.

**Acceptance Criteria**:
- [x] All m.notice responses have correct `msgtype`
- [x] All m.notice responses have non-empty body
- [x] Response bodies match expected text patterns

**Commit**: `test(matrix): m.notice command response validation`

---

### T21b — Custom event validation (workflow.* / agent.* / blocker.*)

**What to do**:
- Create `tests/matrix-e2e/cases/test-custom-events.sh`
- If workflows or agent actions produce custom events, validate their shapes:
  - `workflow.started` → `workflow_id`, `template_id`, `status` required
  - `workflow.step_progress` → `workflow_id`, `step_id`, `progress` required
  - `workflow.completed` → `workflow_id`, `status`, `duration_ms` required
  - `workflow.failed` → `workflow_id`, `error` required
  - `workflow.agent_status` → `agent_id`, `state`, `timestamp` required
  - `blocker.warning` → `workflow_id`, `step_id`, `blocker_type` required
- Use `jq` to validate each event's JSON structure
- Mismatches documented as cross-codebase coordination items

**Why separate from T21a**: These are structured custom events published through `MatrixEventBus`, not `m.notice` message replies. Different schemas, different consumers (ArmorChat's `ControlPlaneStore` vs. Matrix room display).

**Acceptance Criteria**:
- [x] Custom events validated against documented schemas
- [x] Mismatches documented as cross-codebase issues
- [x] Summary: N events validated, N matched, N mismatched

**Commit**: `test(matrix): custom event shape validation`

---

### T22 — Dead-code cleanup

**What to do**:
- Remove resolved `TODO: plug` comments
- Remove stale wiring-gap comments
- Verify all nil-check guards follow consistent patterns (with mode-aware Guard exception)

**Acceptance Criteria**:
- [x] No resolved `TODO: plug` comments remain
- [x] Nil-check guards consistent per field type

**Commit**: `chore: clean resolved TODOs and stale comments`

---

## Final Verification Wave

> 5 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

### F1 — Plan Compliance Audit (deep)

For each Definition of Done item: verify or reject with file:line evidence. For each Guardrail: search codebase for forbidden patterns. Check all evidence files. Compare deliverables against plan.

**Output**: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT`

### F2 — Code Quality Review (unspecified-high)

`go vet` + `go build` + test suite. Review all changed files for anti-patterns.

**Output**: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N/N] | Files [N/N] | VERDICT`

### F3 — Automated Integration QA (unspecified-high)

Start from clean state. Execute every QA scenario. Test cross-task integration. Save to `.sisyphus/evidence/final-qa/`.

**Output**: `Scenarios [N/N] | Integration [N/N] | VERDICT`

### F4 — Scope Fidelity Check (deep)

For each task: compare spec vs. diff. Verify 1:1. Check Guardrails compliance. Detect contamination.

**Output**: `Tasks [N/N compliant] | Contamination [CLEAN/N] | VERDICT`

### F5 — Zero-Trust & Security Posture Check (deep)

Verify security invariants were maintained throughout the stabilization:

- **Guard**: Confirm Bridge fails to start in sentinel mode when Guard is nil. Confirm Bridge starts with warning in native mode.
- **CharArray / memory**: No sensitive data (passwords, PII, tokens) left in string form where a `[]byte` or `CharArray` should be used. Search for patterns like `string(password)`, `string(token)`, `fmt.Sprintf("%s", secretVar)`.
- **E2EE wiring**: Confirm `SetEncryptionService()` and `SetKeyExchangeService()` are called when E2EE is enabled.
- **SQLCipher intact**: `go vet` confirms no imports of `database/sql` without SQLCipher wrapper. Keystore still uses `go-sqlcipher`.
- **No production secrets in test fixtures**: Search test fixture files for patterns matching API keys, passwords, tokens (beyond test-specific values like `"test-password-12345"`).
- **No silent Python fallback for DOC/PPT**: Test explicitly verifies Python is not called for DOC/PPT extraction.

**Output**: `Guard [PASS/FAIL] | CharArray [PASS/FAIL] | E2EE [PASS/FAIL] | SQLCipher [PASS/FAIL] | Secrets [PASS/FAIL] | Fallback [PASS/FAIL] | VERDICT`

---

## Ticket-Ready Backlog

| Ticket | Title | Wave | Estimate | Depends On | Gate |
|--------|-------|------|----------|-----------|------|
| STAB-101 | Fix build-bridge-binaries.sh CGO_ENABLED | 1 | 0.25d | — | — |
| STAB-102 | Update .gitignore for build artifacts | 1 | 0.25d | — | — |
| STAB-103a | Audit contract alignment | 1 | 0.25d | — | **T3 blocked until reviewed** |
| STAB-103 | Implement and wire AuditLog | 2 | 1d | 101, 103a | — |
| STAB-106a | Confirm NavChartStore interface | 2 | 0.25d | — | **T6 blocked until reviewed** |
| STAB-107a | Confirm SkillGate interface | 2 | 0.25d | — | **T7 blocked until reviewed** |
| STAB-100a | Command-path audit and route inventory | 2 | 0.5d | — | **T0 blocked until reviewed** |
| STAB-104 | Implement and wire Guard (mode-aware) | 3 | 1d | 103 | — |
| STAB-105 | Implement and wire GovernanceRoomID | 3 | 0.5d | — | — |
| STAB-106 | Implement and wire NavChartStore | 3 | 1d | 103, 106a | — |
| STAB-107 | Implement and wire SkillGate | 3 | 0.5d | 103, 107a | — |
| STAB-100 | Wire missing command families | 3 | 1d | 100a | — |
| STAB-109 | Create Conduit test fixtures | 4 | 0.5d | — | — |
| STAB-110 | Verify/fix libyara-dev | 4 | 0.25d | 101 | — |
| STAB-111 | Verify/fix Python gRPC | 4 | 0.25d | — | — |
| STAB-112 | Build Conduit lifecycle harness | 4 | 1.5d | 109 | — |
| STAB-113 | Build Matrix client test library | 4 | 1d | 112 | — |
| STAB-114 | Deploy Java sidecar as DOC/PPT default | 4 | 1d | — | — |
| STAB-115 | Remove Python DOC/PPT fallback | 4 | 0.5d | 114 | — |
| STAB-116 | Construct and inject MatrixManager | 4 | 1.5d | — | — |
| STAB-117 | Wire AgentFileTailer into Secretary | 4 | 1.5d | — | — |
| STAB-117a | Inject file-writer for agent mode | 4 | 0.5d | 117 | — |
| STAB-117b | Fix AgentFileTailer race condition | 4 | 0.5d | — | — |
| STAB-118 | /status E2E test | 5 | 0.5d | 100, 112, 113 | — |
| STAB-119 | !agent skills E2E test | 5 | 0.5d | 100, 112, 113 | — |
| STAB-120 | !agent forget-skill E2E test | 5 | 0.5d | 100, 112, 113 | — |
| STAB-121 | /approve /reject E2E test | 5 | 0.5d | 100, 112, 113 | — |
| STAB-122 | !agent commands E2E test (conditional) | 5 | 0.5d | 100, 112, 113 | — |
| STAB-123 | !secretary commands E2E test (conditional) | 5 | 0.5d | 100, 112, 113 | — |
| STAB-124a | m.notice command response validation | 6 | 0.5d | 118-123 | — |
| STAB-124b | Custom event shape validation | 6 | 0.5d | 118-123 | — |
| STAB-125 | Dead-code cleanup | 6 | 0.5d | 103-107, 100 | — |

**Total tickets**: 33 implementation + 5 verification = 38". You can now continue with the user's answers in mind.