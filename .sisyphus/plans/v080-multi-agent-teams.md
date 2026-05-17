# ArmorClaw v0.8.0 — Governed Multi-Agent Team Coordination

## TL;DR

> **Quick Summary**: Build a governed multi-agent team coordination layer on ArmorClaw's production-deployed platform. A CapabilityBroker (new `pkg/capability/`) sits above all executors as middleware, enforcing role-based scoping and risk classification before every action. Phase 4 (Handoff Protocol) deferred pending infrastructure reassessment after Phase 3.
> 
> **Deliverables**:
> - CapabilityBroker middleware in `pkg/capability/` — composes (not wraps) existing Governor
> - Team data model + role registry in new `pkg/team/`
> - Bridge-local executor activation for browser, doc query, email
> - Dynamic secret request flow (BlindFillCard on Android)
> - LLM-driven team composition from natural language goals
> - Full governance audit trail with intent tracing and artifact lineage
> 
> **Estimated Effort**: Large (10+ weeks)
> **Parallel Execution**: YES — up to 3 waves after Phase -1
> **Critical Path**: Phase -1 → Phase 0 → Phase 1 typed artifacts → Phase 1 broker → Phase 2 → Phase 3 → REASSESS → Phase 5+6 → Phase 7

---

## Context

### Original Request
10-phase, ~80-task roadmap for adding multi-agent team coordination to ArmorClaw. Key capability: natural-language goal → governed decomposition → typed handoffs → secure multi-agent execution.

### Interview Summary
**Key Discussions**:
- Phase 0 assessment: 2 of 5 tasks likely already done (Qdrant v1.7, PDF panic)
- Phase 1 (Governance Foundation): confirmed as critical path — everything depends on it
- Phase 4 (Handoff): user decided to DEFER — reassess after Phase 3
- Timeline: don't adjust durations, focus on results quality
- Team size: 3+ developers → aggressive parallelism possible

**Research Findings**:
- Industry validates hub-and-spoke broker pattern (LangGraph, Google ADK, AutoGen converged here)
- NetworkMode:none exceeds industry norms — keep it
- OWASP Agentic Top 10 (2026) is canonical risk reference
- "Broker must never be executor" — critical design principle
- Three separate approval systems exist but should NOT be consolidated in v0.8.0 (only define interface)
- `bridge/internal/ai/` can't be imported by `pkg/` — extract interface, don't restructure
- `main.go` is 3684 lines — guaranteed merge conflicts with 3+ devs

### Metis Review
**Identified Gaps** (addressed):
- `main.go` extraction into `setup_*.go` files — ADDED as Phase -1 (prerequisite)
- Broker should COMPOSE Governor (HAS-A), not WRAP (IS-A) — CORRECTED
- Broker must be middleware in BOTH execution pipelines — GUARDED
- Broker goes in `pkg/capability/`, NOT `pkg/secretary/` — DIRECTED
- Approval consolidation is scope creep — LOCKED OUT of v0.8.0
- SecurityConfig/SealedKeystore/containerd are scope creep — LOCKED OUT
- DEFER timeout policy must be defined before implementation — GUARDED
- Edge cases: circular deps, concurrent approvals, broker crash, role escalation — ADDRESSED in acceptance criteria

---

## Work Objectives

### Core Objective
Add a governed multi-agent team coordination layer where every capability invocation passes through a CapabilityBroker with typed artifacts, risk classification (ALLOW/DENY/DEFER), and role-based scoping.

### Concrete Deliverables
- `pkg/capability/` — CapabilityBroker, risk taxonomy, action evaluation
- `pkg/team/` — Team, Role, Member types, team store, CRUD service
- `pkg/interfaces/capability.go` — Broker, RiskClassifier interfaces
- `pkg/interfaces/consent.go` — ConsentProvider interface
- `cmd/bridge/setup_*.go` — Extracted wiring from main.go
- Typed artifact contracts (ActionRequest, ActionResponse, SecretRef, BrowserIntent, etc.)
- Bridge-local executor activation (browser, doc query)
- Dynamic secret request flow with BlindFillCard Android composable
- LLM team composition from natural language goals
- Comprehensive governance audit trail

### Definition of Done
- [ ] `go test ./pkg/capability/... ./pkg/team/... ./pkg/interfaces/...` → PASS
- [ ] `go test -bench=BenchmarkBrokerAuthorize -benchtime=5s` → p99 < 10ms
- [ ] Broker fail-closed test: inject crash → all actions deny with clear error
- [ ] Role escalation test: agent requests capability beyond role → DENY
- [ ] Agent with zero capabilities → all actions DENY
- [ ] DEFER → 300s timeout → auto-deny with notification
- [ ] Team dissolution → in-flight workflows fail with "team dissolved"
- [ ] `cargo test` in rust-vault/ → PASS
- [ ] ArmorChat builds and BlindFillCard renders correctly
- [ ] No changes to `pkg/governor/` (brownfield preserved)

### Must Have
- CapabilityBroker as middleware in BOTH execution pipelines (SkillExecutor + StepExecutor)
- Typed artifact contracts defined as Go structs with JSON schemas
- Risk taxonomy: ALLOW / DENY / DEFER with 6 risk classes (payment, identity_pii, credential_use, external_communication, file_exfiltration, irreversible_action)
- Fail-closed broker (DENY on crash/timeout/error)
- Team data model with role registry and team CRUD
- Bridge-local executor for browser and doc query (using existing BridgeLocalRegistry)
- Dynamic secret request with BlindFillCard Android composable
- Intent tracing and artifact lineage in audit log
- DEFER timeout: 300s with auto-deny and Matrix notification

### Must NOT Have (Guardrails)
- ❌ No modification to `pkg/governor/` — zero lines changed
- ❌ No approval system consolidation — only define ConsentProvider interface
- ❌ No SecurityConfig tier activation — v0.9.0+
- ❌ No SealedKeystore wiring — v0.9.0+
- ❌ No containerd/Firecracker changes — zero changes to `pkg/runtime/`
- ❌ No third execution pipeline — broker is middleware, not dispatch
- ❌ No package restructuring of `internal/ai/` — extract interface instead
- ❌ No relaxation of NetworkMode:none
- ❌ No import of `internal/` packages from new `pkg/capability/` code
- ❌ No package-level globals in broker — struct-based, constructor-injected
- ❌ No two developers with simultaneous unmerged changes to `main.go`
- ❌ No Phase 4 (Handoff Protocol) — deferred pending Phase 3 reassessment

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (100+ Go test files, `go test`, `testify`, comprehensive mocks)
- **Automated tests**: YES (TDD for new packages, tests-after for integration)
- **Framework**: `go test` with `testify/assert` and `testify/require`
- **Rust**: `cargo test` in rust-vault/
- **Android**: JUnit + Compose testing for BlindFillCard
- **Benchmarks**: `go test -bench` for broker latency

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go packages**: Use Bash (`go test`, `go vet`, `go build`) — compile, test, assert output
- **Rust packages**: Use Bash (`cargo test`, `cargo check`) — compile, test, assert output
- **Android**: Use Bash (`./gradlew test`, `./gradlew assembleDebug`) — build, test
- **Integration**: Use Bash (`go test -tags=integration`) — wire components, assert behavior
- **Benchmarks**: Use Bash (`go test -bench`) — assert p99 < threshold

---

## Execution Strategy

### Parallel Execution Waves

```
Phase -1 (Start Immediately — enables all parallelism):
└── Task 1: Extract main.go into setup_*.go files [quick]

Wave 1 (Phase 0 + Phase 1 foundation — after Phase -1):
├── Task 2: Verify/close Phase 0.1 Qdrant + 0.2 PDF [quick]
├── Task 3: Tighten Docker vault volume mount [quick]
├── Task 4: Vault governance auto-detect activation [quick]
├── Task 5: Wire email attachments → RouteExtractText [unspecified-high]
├── Task 6: Define typed artifact contracts [deep]
├── Task 7: Define CapabilityBroker + RiskClassifier interfaces [deep]
└── Task 8: Define ConsentProvider interface [quick]

Wave 2 (Phase 1 core — after Wave 1):
├── Task 9: Build CapabilityBroker implementation [deep]
├── Task 10: Build risk taxonomy engine [unspecified-high]
├── Task 11: Extend audit for governance events + lineage [unspecified-high]
├── Task 12: Wire broker into StepExecutor [deep]
├── Task 13: Wire broker into SkillExecutor (internal path) [unspecified-high]
├── Task 14: Assess V6Microkernel wiring gaps [quick]
└── Task 15: Encapsulate PendingApproval globals into struct [quick]

Wave 3 (Phase 1 Rust + Phase 2 start — parallel):
├── Task 16: Extend vault proto with capability_scope [unspecified-high]
├── Task 17: Implement vault scope validation in ephemeral.rs [unspecified-high]
├── Task 18: Define Team/Member/Role/Template types [deep]
├── Task 19: Build TeamRole registry [deep]
├── Task 20: Build team store (SQLCipher schema + CRUD) [deep]
└── Task 21: Build team RPC service [unspecified-high]

Wave 4 (Phase 2 wiring + Phase 3 start):
├── Task 22: Wire TeamRole → CapabilityBroker [deep]
├── Task 23: Extend WorkflowStep with team_id [unspecified-high]
├── Task 24: Activate BridgeLocalRegistry in StepExecutor [quick]
├── Task 25: Register browser_execute as bridge-local handler [unspecified-high]
├── Task 26: Register doc_query as bridge-local handler [unspecified-high]
├── Task 27: Build BrowserContextManager [unspecified-high]
└── Task 28: Wire typed artifacts into bridge-local executors [deep]

Wave 5 (Phase 3 completion + Phase 5 start):
├── Task 29: Build QueryDocuments gRPC in Rust sidecar [unspecified-high]
├── Task 30: Per-team Qdrant collection creation [unspecified-high]
├── Task 31: CAPTCHA delegation blocker [quick]
├── Task 32: Extend EmailDispatcher with team routing [unspecified-high]
├── Task 33: Build IMAP inbox management [unspecified-high]
├── Task 34: Build email thread context tracker [unspecified-high]
└── Task 35: Build email draft workflow + templates [unspecified-high]

Wave 6 (Phase 6 — parallel with Wave 5):
├── Task 36: Define SecretRequestEvent Matrix event [deep]
├── Task 37: Build SecretRequestManager [deep]
├── Task 38: Add request_secret bridge-local step type [unspecified-high]
├── Task 39: Build BlindFillCard Android composable [visual-engineering]
├── Task 40: Wire secret request through approval policy [deep]
└── Task 41: On approval → store secret + return SecretRef [deep]

Wave 7 (Phase 7 — LLM Team Composition):
├── Task 42: Extract AIClient interface to pkg/interfaces [quick]
├── Task 43: Build structured output parser for LLM responses [deep]
├── Task 44: Build TeamComposer LLM decomposition [deep]
├── Task 45: Build TeamPlanExecutor [deep]
├── Task 46: Enhance AgentFactory for role specialization [unspecified-high]
├── Task 47: Wire TeamComposer into TaskScheduler [unspecified-high]
└── Task 48: Fallback/retry for team execution [unspecified-high]

Wave 8 (Phase 8 — Observability):
├── Task 49: Extend audit schema for team events [unspecified-high]
├── Task 50: Per-team metrics collection [unspecified-high]
├── Task 51: Team governance controls [quick]
├── Task 52: Wire team governance to approval policy [deep]
├── Task 53: Team timeline UI in admin panel [visual-engineering]
└── Task 54: License system team-aware enforcement [unspecified-high]

Wave FINAL (Verification — after ALL implementation):
├── Task F1: Plan compliance audit [oracle]
├── Task F2: Code quality review [unspecified-high]
├── Task F3: Integration QA [unspecified-high]
└── Task F4: Scope fidelity check [deep]
→ Present results → Get explicit user okay

REASSESS POINT (after Wave 4 / Phase 3):
└── Verify bridge-local executors work correctly → decide on Phase 4 (Handoff) vs Phase 5

Critical Path: Task 1 → Task 6 → Task 7 → Task 9 → Task 12 → Task 22 → Task 24 → REASSESS
Max Concurrent: 7 tasks (Waves 1-3)
Parallel Speedup: ~60% faster than sequential
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | — | 2-15 | -1 |
| 2-5 | 1 | — | 1 |
| 6 | 1 | 9, 10, 25-28, 36-41 | 1 |
| 7 | 1 | 9, 10, 12, 13 | 1 |
| 8 | 1 | 9, 40 | 1 |
| 9 | 6, 7, 8 | 12, 13, 22, 40 | 2 |
| 10 | 6, 7 | 9, 40 | 2 |
| 11 | 7 | 49 | 2 |
| 12 | 9 | 22, 24 | 2 |
| 13 | 9 | — | 2 |
| 14 | 1 | — | 2 |
| 15 | 1 | 9 | 2 |
| 16 | 7 | 17 | 3 |
| 17 | 16 | — | 3 |
| 18 | 7 | 19, 20, 21 | 3 |
| 19 | 18 | 22 | 3 |
| 20 | 18 | 21 | 3 |
| 21 | 20 | — | 3 |
| 22 | 9, 19 | 44 | 4 |
| 23 | 18 | — | 4 |
| 24 | 12 | 25, 26, 28 | 4 |
| 25 | 6, 24 | — | 4 |
| 26 | 6, 24 | — | 4 |
| 27 | 25 | — | 4 |
| 28 | 6, 24, 25, 26 | — | 4 |
| 29-31 | 24 | — | 5 |
| 32-35 | 22 | — | 5 |
| 36-41 | 9, 12 | — | 6 |
| 42 | — | 44 | 7 |
| 43 | 42 | 44 | 7 |
| 44 | 22, 42, 43 | 45 | 7 |
| 45 | 44 | 47 | 7 |
| 46 | 19 | 45 | 7 |
| 47 | 45 | — | 7 |
| 48 | 45 | — | 7 |
| 49 | 11 | — | 8 |
| 50-54 | 18, 22 | — | 8 |

### Agent Dispatch Summary

- **Phase -1**: 1 — T1 → `quick`
- **Wave 1**: 7 — T2-T5 → `quick`/`unspecified-high`, T6-T7 → `deep`, T8 → `quick`
- **Wave 2**: 7 — T9/T12 → `deep`, T10/T11/T13 → `unspecified-high`, T14/T15 → `quick`
- **Wave 3**: 6 — T16-T17 → `unspecified-high`, T18-T19/T20 → `deep`, T21 → `unspecified-high`
- **Wave 4**: 7 — T22/T28 → `deep`, T23-T27 → `unspecified-high`/`quick`
- **Wave 5**: 7 — T29-T35 → `unspecified-high`/`quick`
- **Wave 6**: 6 — T36-T37/T40-T41 → `deep`, T38 → `unspecified-high`, T39 → `visual-engineering`
- **Wave 7**: 7 — T42 → `quick`, T43-T45 → `deep`, T46-T48 → `unspecified-high`
- **Wave 8**: 6 — T49-T52/T54 → `unspecified-high`/`quick`, T53 → `visual-engineering`
- **FINAL**: 4 — F1 → `oracle`, F2-F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

### Phase -1: Extract main.go (Prerequisite for parallelism)

- [x] 1. Extract `main.go` wiring into `setup_*.go` files

  **What to do**:
  - Read `bridge/cmd/bridge/main.go` (3684 lines) — identify `runBridgeServer()` wiring zones (lines 1710-2961)
  - Create `cmd/bridge/setup_vault.go` — extract vault client + event bridge init (lines 2267-2286)
  - Create `cmd/bridge/setup_mcp.go` — extract MCPRouter creation (lines 2493-2529)
  - Create `cmd/bridge/setup_secretary.go` — extract secretary/orchestrator/step executor wiring (lines 2531-2653)
  - Create `cmd/bridge/setup_broker.go` — empty placeholder for Phase 1 broker wiring
  - Create `cmd/bridge/setup_teams.go` — empty placeholder for Phase 2 team wiring
  - Replace extracted code in `main.go` with function calls: `setupVault()`, `setupMCP()`, `setupSecretary()`
  - Verify `go build ./cmd/bridge/` passes
  - Run full test suite: `go test ./...`

  **Must NOT do**:
  - Do NOT change any logic — pure extraction
  - Do NOT modify any package outside `cmd/bridge/`
  - Do NOT add new dependencies

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: [`/git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: NO — blocks all other tasks
  - **Parallel Group**: Phase -1 (sequential)
  - **Blocks**: Tasks 2-54
  - **Blocked By**: None

  **References**:
  - `bridge/cmd/bridge/main.go:1710-2961` — The `runBridgeServer()` function that needs extraction. Contains all subsystem initialization: vault (2267-2286), MCP router (2493-2529), secretary services (2531-2653), command handler (2655-2699)
  - `bridge/cmd/bridge/main.go:2267-2286` — Vault client init with V6Microkernel check
  - `bridge/cmd/bridge/main.go:2493-2529` — MCPRouter creation with skills, PII interceptor
  - `bridge/cmd/bridge/main.go:2600-2653` — StepExecutor creation with ApprovalEngine, factory

  **Acceptance Criteria**:
  - [ ] `go build ./cmd/bridge/` → PASS
  - [ ] `go test ./cmd/bridge/...` → PASS
  - [ ] `git diff --stat HEAD -- bridge/cmd/bridge/main.go` → shows reduction of ~800+ lines
  - [ ] Files created: setup_vault.go, setup_mcp.go, setup_secretary.go, setup_broker.go, setup_teams.go

  **QA Scenarios**:

  ```
  Scenario: Bridge compiles and runs after extraction
    Tool: Bash
    Preconditions: Go toolchain available
    Steps:
      1. `cd bridge && go build ./cmd/bridge/`
      2. Verify binary produced: `ls -la bridge`
      3. `go test ./cmd/bridge/... -v`
    Expected Result: Build succeeds, all tests pass, zero compilation errors
    Failure Indicators: "undefined: setupVault" or any compilation error
    Evidence: .sisyphus/evidence/task-1-build.txt

  Scenario: No behavioral change — existing tests still pass
    Tool: Bash
    Preconditions: All existing tests were passing before change
    Steps:
      1. `cd bridge && go test ./... -count=1`
      2. Assert: same number of tests pass as before
    Expected Result: All tests pass with zero failures
    Failure Indicators: Any test failure, especially in secretary/ or rpc/
    Evidence: .sisyphus/evidence/task-1-tests.txt
  ```

  **Commit**: YES
  - Message: `refactor(bridge): extract main.go wiring into setup_*.go files`
  - Files: `cmd/bridge/setup_*.go`, `cmd/bridge/main.go`
  - Pre-commit: `go build ./cmd/bridge/ && go test ./cmd/bridge/...`

### Wave 1: Phase 0 Unblock + Phase 1 Foundation

- [x] 2. Verify and close Phase 0.1 (Qdrant) + Phase 0.2 (PDF panic)

  **What to do**:
  - Read `sidecar/src/document/qdrant.rs` — confirm it uses v1.7 builder pattern (`CreateCollectionBuilder`, `UpsertPointsBuilder`, etc.)
  - Read `sidecar/src/document/pdf.rs` — confirm zero production panics (no `.unwrap()`/`.expect()` outside tests)
  - Check git history for any recent PDF panic fixes: `git log --oneline --all --grep="pdf.*panic\|PDF.*panic"`
  - If both are confirmed done, close with documentation. If not, fix.
  - Run `cargo check --lib` in sidecar/ to verify compilation

  **Must NOT do**:
  - Do NOT modify code unless a genuine issue is found

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 3-8)
  - **Parallel Group**: Wave 1
  - **Blocks**: None (verification only)
  - **Blocked By**: Task 1

  **References**:
  - `sidecar/Cargo.toml:37` — qdrant-client version declaration
  - `sidecar/src/document/qdrant.rs:5-13` — Builder pattern imports to verify
  - `sidecar/src/document/pdf.rs` — Full file, check for unwrap/expect in non-test code
  - `sidecar/src/output/pdf.rs` — Contains test-only panics, verify `#[cfg(test)]` gating

  **Acceptance Criteria**:
  - [ ] `cargo check --lib` in sidecar/ → PASS
  - [ ] Qdrant: builder pattern confirmed in qdrant.rs OR fix applied
  - [ ] PDF: zero production panics confirmed OR specific panic documented and fixed

  **QA Scenarios**:

  ```
  Scenario: Sidecar compiles with current Qdrant client
    Tool: Bash
    Preconditions: Rust toolchain available
    Steps:
      1. `cd sidecar && cargo check --lib 2>&1`
      2. Assert: no errors
    Expected Result: Compilation succeeds, builder pattern is valid against v1.7
    Failure Indicators: "no function or associated item named `CreateCollectionBuilder`"
    Evidence: .sisyphus/evidence/task-2-qdrant-check.txt

  Scenario: PDF extraction handles malformed input without panic
    Tool: Bash
    Preconditions: Rust toolchain, test data available
    Steps:
      1. `cd sidecar && cargo test --lib document::pdf 2>&1`
      2. Assert: all tests pass, no panic in output
    Expected Result: All PDF tests pass, graceful error handling
    Failure Indicators: "thread 'xxx' panicked at"
    Evidence: .sisyphus/evidence/task-2-pdf-tests.txt
  ```

  **Commit**: YES (only if changes needed)
  - Message: `fix(sidecar): verify qdrant v1.7 builder and pdf panic status`
  - Files: `sidecar/src/document/`

- [x] 3. Tighten Docker vault volume mount for volume isolation

  **What to do**:
  - Read all docker-compose files and identify Rust Vault's mount of `/run/armorclaw/` (too broad)
  - Change vault mount from `/run/armorclaw:/run/armorclaw` to `/run/armorclaw/vault:/run/armorclaw` (or specific socket path)
  - Verify Python Office Sidecar's mount `/run/armorclaw/office-sidecar/` remains isolated
  - Verify Jetski has no shared volume paths
  - Document the volume isolation scheme in a comment in each docker-compose file

  **Must NOT do**:
  - Do NOT change container networking
  - Do NOT add new services

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2, 4-8)
  - **Parallel Group**: Wave 1
  - **Blocks**: None
  - **Blocked By**: Task 1

  **References**:
  - `docker-compose.yml:102` — Rust Vault container with broad `/run/armorclaw` mount
  - `docker-compose-full.yml` — Bridge `bridge_run` named volume mapping
  - `docker-compose.jetski.yml` — Jetski compose (verify no volume overlap)
  - `deploy/docker-compose.sidecar-py.yml` — Python sidecar with isolated `/run/armorclaw/office-sidecar/` mount

  **Acceptance Criteria**:
  - [ ] Rust Vault mounts specific socket path, NOT entire `/run/armorclaw/`
  - [ ] No shared mount paths between Jetski and Doc Sidecar
  - [ ] Volume isolation documented in comments

  **QA Scenarios**:

  ```
  Scenario: Vault cannot see office sidecar socket
    Tool: Bash
    Preconditions: Docker available
    Steps:
      1. `grep -A5 "rust-vault\|vault:" docker-compose.yml`
      2. Assert: mount path is specific (e.g., `/run/armorclaw/vault.sock`)
      3. Assert: no mount of bare `/run/armorclaw/`
    Expected Result: Vault container has narrow mount, no access to office-sidecar/ directory
    Failure Indicators: Mount of `/run/armorclaw:/run/armorclaw` still present
    Evidence: .sisyphus/evidence/task-3-vault-mount.txt

  Scenario: Jetski and Doc Sidecar have no shared volumes
    Tool: Bash
    Steps:
      1. Extract volume mounts from Jetski compose
      2. Extract volume mounts from Doc Sidecar compose
      3. Assert: intersection is empty
    Expected Result: Zero shared mount paths
    Failure Indicators: Any path appearing in both services
    Evidence: .sisyphus/evidence/task-3-isolation-check.txt
  ```

  **Commit**: YES
  - Message: `fix(deploy): tighten vault volume mount for cross-sidecar isolation`
  - Files: `docker-compose.yml`, `docker-compose-full.yml`

- [x] 4. Activate vault governance with auto-detect (not hard default flip)

  **What to do**:
  - Read `bridge/pkg/config/config.go:1014` — current `V6Microkernel: false` default
  - Read `bridge/cmd/bridge/setup_vault.go` — vault init logic (extracted in Task 1)
  - Change activation strategy: instead of flipping default to `true`, add auto-detection — if vault socket exists at startup (`/run/armorclaw/vault.sock`), enable automatically
  - Preserve graceful degradation: if auto-detected but connection fails, log warning and continue
  - Keep `V6Microkernel` config flag as override (explicit `true` or `false` beats auto-detect)
  - Update `deploy/` docker-compose files to ensure vault container starts before bridge
  - Test with vault present and absent

  **Must NOT do**:
  - Do NOT break standalone bridge deployments (no vault)
  - Do NOT change vault container configuration

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2-3, 5-8)
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 14 (V6Microkernel assessment)
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/config/config.go:1014` — `V6Microkernel: false` default
  - `bridge/cmd/bridge/main.go:2267-2286` (now in `setup_vault.go`) — Vault init logic with graceful degradation
  - `rust-vault/src/governance/ephemeral.rs` — EphemeralTokenStore (430 lines, production-ready)
  - `rust-vault/proto/governance.proto` — 4 RPCs: Issue/Consume/Zeroize/Subscribe

  **Acceptance Criteria**:
  - [ ] Bridge starts successfully with vault present → auto-enables governance
  - [ ] Bridge starts successfully with vault absent → logs warning, continues
  - [ ] `V6Microkernel=true` config overrides auto-detect ON
  - [ ] `V6Microkernel=false` config overrides auto-detect OFF
  - [ ] `go test ./pkg/vault/...` → PASS

  **QA Scenarios**:

  ```
  Scenario: Auto-detect enables governance when vault socket present
    Tool: Bash
    Preconditions: Vault socket file exists at expected path
    Steps:
      1. `touch /run/armorclaw/vault.sock`
      2. Start bridge with no V6Microkernel config
      3. Check logs for "vault governance auto-detected"
    Expected Result: Vault client connects, event bridge starts
    Failure Indicators: "vault governance disabled" or connection error without graceful degradation
    Evidence: .sisyphus/evidence/task-4-auto-detect.txt

  Scenario: Graceful degradation when vault absent
    Tool: Bash
    Preconditions: No vault socket file
    Steps:
      1. `rm -f /run/armorclaw/vault.sock`
      2. Start bridge with no V6Microkernel config
      3. Check logs for "vault not available, continuing without governance"
    Expected Result: Bridge starts normally, no crash, clear log message
    Failure Indicators: `log.Fatalf` or bridge crash
    Evidence: .sisyphus/evidence/task-4-graceful.txt
  ```

  **Commit**: YES
  - Message: `feat(vault): auto-detect governance activation instead of hard default`
  - Files: `bridge/cmd/bridge/setup_vault.go`, `bridge/pkg/config/config.go`

- [x] 5. Wire email attachments to document sidecar via RouteExtractText()

  **What to do**:
  - Read `bridge/pkg/email/ingest_server.go` — find where attachments are stored and YARA scanned
  - Read `bridge/pkg/sidecar/office_client.go` — understand `RouteExtractText()` signature and routing
  - Add sidecar clients (`officeClient`, `rustClient`) to `IngestServerConfig` struct
  - After YARA scan passes, for each attachment with extractable format (PDF, DOCX, XLSX, PPTX):
    - Map MIME ContentType to DocumentFormat (`application/pdf` → `"pdf"`, etc.)
    - Call `RouteExtractText()` with attachment content
    - Store extracted text chunks, associate with emailID
    - Include extracted text in `EmailReceivedEvent` or publish separate event
  - Design decision: ASYNC approach — extract in goroutine, don't block email ingest. If sidecar is down, skip extraction and continue.
  - Add size limit check (skip extraction for attachments > 10MB)
  - Write tests for MIME→format mapping and error handling

  **Must NOT do**:
  - Do NOT modify `RouteExtractText()` or sidecar clients
  - Do NOT block email ingest on sidecar availability
  - Do NOT route non-extractable formats (images, video, audio)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2-4, 6-8)
  - **Parallel Group**: Wave 1
  - **Blocks**: None
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/email/ingest_server.go:128-149` — Attachment storage and YARA scanning. This is where the gap is — after YARA scan, no RouteExtractText call
  - `bridge/pkg/email/mime_parser.go:131-151` — `ParsedAttachment{Filename, Content, ContentType, ContentID, Size}`. Content field has the raw bytes
  - `bridge/pkg/sidecar/office_client.go` — `RouteExtractText(ctx, ExtractTextRequest) (*ExtractTextResponse, error)`. 3-layer routing: text bypass → magic byte routing → strict drop
  - `bridge/pkg/sidecar/client.go` — gRPC sidecar client (531 lines). Methods: ExtractText, StoreDocument, etc.

  **Acceptance Criteria**:
  - [ ] Email with PDF attachment → extracted text stored alongside email
  - [ ] Email with DOCX attachment → extracted text stored alongside email
  - [ ] Email with image attachment → skipped (not routed to sidecar)
  - [ ] Email with 15MB attachment → skipped (over size limit)
  - [ ] Sidecar down → email processed normally, extraction skipped with warning log
  - [ ] `go test ./pkg/email/... -run TestAttachmentExtraction` → PASS

  **QA Scenarios**:

  ```
  Scenario: PDF attachment text extracted from email
    Tool: Bash
    Steps:
      1. `cd bridge && go test -v -run TestAttachmentExtraction ./pkg/email/...`
      2. Check for "extracted text" in test output
    Expected Result: Text extracted from PDF attachment, stored with email record
    Failure Indicators: "sidecar unavailable" or "unsupported format" for valid PDF
    Evidence: .sisyphus/evidence/task-5-pdf-extract.txt

  Scenario: Sidecar down — email processing continues
    Tool: Bash
    Steps:
      1. Mock sidecar client to return connection error
      2. Send email with PDF attachment
      3. Assert: email stored, extraction skipped, warning logged
    Expected Result: Email processed, no crash, log contains "skipping extraction"
    Failure Indicators: Email processing blocked or failed
    Evidence: .sisyphus/evidence/task-5-sidecar-down.txt
  ```

  **Commit**: YES
  - Message: `feat(email): wire attachments to document sidecar via RouteExtractText`
  - Files: `bridge/pkg/email/ingest_server.go`, `bridge/pkg/email/ingest_server_test.go`
  - Pre-commit: `go test ./pkg/email/...`

- [x] 6. Define typed artifact contracts (Phase 1 foundation)

  **What to do**:
  - Create `bridge/pkg/capability/types.go` with all typed artifact Go structs:
    - `ActionRequest{AgentID, TeamID, Action string; Params map[string]any}`
    - `ActionResponse{Allowed bool; Classification RiskLevel; Reason string; SessionID string}`
    - `RiskLevel` type — `ALLOW`, `DENY`, `DEFER`
    - `RiskClass` type — `payment`, `identity_pii`, `credential_use`, `external_communication`, `file_exfiltration`, `irreversible_action`
    - `CapabilitySet map[string]bool` — e.g., `{"browser.browse": true, "browser.fill": false}`
    - `SecretRef{Field string; Hash string; Version int}`
    - `BrowserIntent{URL string; Action string; FormFields []string}`
    - `BrowserResult{URL string; Title string; ExtractedData []string; Screenshots []string}`
    - `DocumentRef{CollectionID string; ChunkIDs []string}`
    - `ExtractedChunkSet{Chunks []string; Summary string}`
    - `EmailDraft{To, Subject, BodyMasked string; Attachments []string}`
    - `ApprovalDecision{Approved bool; Fields []string; DeniedFields []string}`
    - `WorkflowBlocker{Type, Message, Suggestion string}`
  - Each struct gets JSON tags and a `Validate() error` method
  - Write round-trip JSON marshal/unmarshal tests for every type
  - Write backward-compatibility test: adding a field to a struct doesn't break old JSON

  **Must NOT do**:
  - Do NOT put types in `pkg/secretary/types.go` — new package
  - Do NOT import `internal/` packages

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2-5, 7-8)
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 9, 10, 25-28, 36-41 (everything that uses typed artifacts)
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/secretary/types.go` — Existing workflow types (525 lines). These remain untouched — new types go in separate package
  - `bridge/pkg/secretary/bridge_local_registry.go:54-61` — StepConfig with generic `json.RawMessage`. Typed artifacts replace this generic handling
  - `bridge/pkg/email/hitl_approval.go` — Existing approval pattern. ApprovalDecision type should match this field structure

  **Acceptance Criteria**:
  - [ ] `go build ./pkg/capability/...` → PASS
  - [ ] Round-trip JSON test for every struct → PASS
  - [ ] Backward-compatibility test → PASS
  - [ ] `go vet ./pkg/capability/...` → PASS

  **QA Scenarios**:

  ```
  Scenario: All typed artifacts round-trip through JSON
    Tool: Bash
    Steps:
      1. `cd bridge && go test -v -run TestArtifactRoundTrip ./pkg/capability/...`
      2. Assert: all structs marshal → unmarshal → equal
    Expected Result: Every type passes round-trip test
    Failure Indicators: Any struct fails marshal or unmarshal
    Evidence: .sisyphus/evidence/task-6-roundtrip.txt

  Scenario: Backward compatibility — new field doesn't break old JSON
    Tool: Bash
    Steps:
      1. `cd bridge && go test -v -run TestArtifactBackwardCompat ./pkg/capability/...`
      2. Assert: old JSON (without new field) unmarshals successfully
    Expected Result: All types handle missing fields gracefully (zero values)
    Failure Indicators: Unmarshal error for valid old-format JSON
    Evidence: .sisyphus/evidence/task-6-backward.txt
  ```

  **Commit**: YES
  - Message: `feat(capability): define typed artifact contracts and risk taxonomy`
  - Files: `bridge/pkg/capability/types.go`, `bridge/pkg/capability/types_test.go`
  - Pre-commit: `go test ./pkg/capability/...`

- [x] 7. Define CapabilityBroker, RiskClassifier, and ConsentProvider interfaces

  **What to do**:
  - Create `bridge/pkg/interfaces/capability.go`:
    - `CapabilityBroker` interface: `Authorize(ctx, ActionRequest) (ActionResponse, error)`
    - `RiskClassifier` interface: `Classify(ctx, action string, params map[string]any) (RiskClass, RiskLevel)`
    - `CapabilityRegistry` interface: `GetCapabilities(role string) (CapabilitySet, error)`, `RegisterRole(role string, capabilities CapabilitySet) error`
  - Create `bridge/pkg/interfaces/consent.go`:
    - `ConsentProvider` interface: `RequestConsent(ctx, requestID, reason string, fields []string) (<-chan ConsentResult, error)`
    - `ConsentResult` struct: `{Approved bool; ApprovedFields []string; DeniedFields []string; Error error}`
  - Create `bridge/pkg/interfaces/team.go`:
    - `TeamStore` interface: `CreateTeam`, `GetTeam`, `ListTeams`, `AddMember`, `RemoveMember`, `DissolveTeam`
    - `TeamService` interface: `AssignRole`, `GetCapabilitiesForMember`, `ValidateTeamMembership`
  - Write compile-time interface satisfaction checks
  - Zero implementation — interfaces only (HITL implementations come in Phase 1 Wave 2)

  **Must NOT do**:
  - Do NOT append to `pkg/interfaces/skillgate.go` — new files
  - Do NOT import `internal/` packages
  - Do NOT define implementation — interfaces only

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2-6, 8)
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 9, 10, 12, 13 (all Phase 1 implementation)
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/interfaces/skillgate.go:13-29` — Existing SkillGate interface pattern. Follow this style for new interfaces
  - `bridge/pkg/governor/skillgate.go` — Governor implements SkillGate. CapabilityBroker will compose this, not wrap it
  - `bridge/pkg/secretary/approvals.go:89` — ApprovalEngineImpl is the richest approval system. ConsentProvider interface should be compatible with its EvaluateStep() pattern

  **Acceptance Criteria**:
  - [ ] `go build ./pkg/interfaces/...` → PASS
  - [ ] Compile-time interface checks exist for all interfaces
  - [ ] `go vet ./pkg/interfaces/...` → PASS

  **QA Scenarios**:

  ```
  Scenario: All interfaces compile and satisfy type checker
    Tool: Bash
    Steps:
      1. `cd bridge && go build ./pkg/interfaces/...`
      2. `go vet ./pkg/interfaces/...`
    Expected Result: Clean build, zero warnings
    Failure Indicators: Any compilation error
    Evidence: .sisyphus/evidence/task-7-interfaces.txt
  ```

  **Commit**: YES (groups with Task 6)
  - Message: `feat(capability): define broker, classifier, consent, and team interfaces`
  - Files: `bridge/pkg/interfaces/capability.go`, `bridge/pkg/interfaces/consent.go`, `bridge/pkg/interfaces/team.go`
  - Pre-commit: `go build ./pkg/interfaces/...`

- [x] 8. Define ConsentProvider interface + assess approval consolidation strategy

  **What to do**:
  - Create `bridge/pkg/interfaces/consent.go` with `ConsentProvider` interface (if not created in Task 7)
  - Document how existing approval systems will implement this interface:
    - `HITLConsentManager` → implements ConsentProvider (MCP tool calls, 60s timeout)
    - `ApprovalEngineImpl` → implements ConsentProvider (workflow steps, policy-based)
    - `PendingApproval` → wraps ApprovalEngineImpl, implements ConsentProvider (blocking goroutine)
  - Write a design doc in `bridge/pkg/capability/CONSENT_DESIGN.md`:
    - Interface contract
    - Timeout policy: 300s for DEFER → auto-deny
    - Queue depth limit: 50 concurrent DEFERRED actions
    - Notification delivery: Matrix event + push within 2s
  - Do NOT implement — design document only

  **Must NOT do**:
  - Do NOT consolidate existing approval systems in v0.8.0
  - Do NOT modify `pkg/email/hitl_approval.go`, `pkg/secretary/approvals.go`, or `pkg/secretary/pending_approval.go`

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: [`writing`]

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2-7)
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 9, 40
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/email/hitl_approval.go:118` — EmailApprovalManager with 300s timeout, in-memory map
  - `bridge/pkg/secretary/approvals.go:89-611` — ApprovalEngineImpl with policy evaluation, store-backed
  - `bridge/pkg/secretary/pending_approval.go:22-25` — 120s default, 900s max timeout, package-level globals

  **Acceptance Criteria**:
  - [ ] `CONSENT_DESIGN.md` created with interface contract, timeout policy, queue limits
  - [ ] Interface defined in `pkg/interfaces/consent.go`
  - [ ] `go build ./pkg/interfaces/...` → PASS

  **QA Scenarios**:

  ```
  Scenario: Design document is complete and internally consistent
    Tool: Bash
    Steps:
      1. Verify CONSENT_DESIGN.md exists and contains: interface contract, timeout policy, queue limits
      2. Verify consent.go interface matches design document
    Expected Result: Document and code are consistent
    Failure Indicators: Missing sections or interface/design mismatch
    Evidence: .sisyphus/evidence/task-8-design.txt
  ```

  **Commit**: YES (groups with Tasks 6-7)
  - Message: `docs(capability): consent provider design document and interface`
  - Files: `bridge/pkg/capability/CONSENT_DESIGN.md`, `bridge/pkg/interfaces/consent.go`

### Wave 2: Phase 1 Core — Broker Implementation + Wiring

- [x] 9. Build CapabilityBroker implementation

  **What to do**:
  - Create `bridge/pkg/capability/broker.go`:
    - `Broker` struct: holds references to `SkillGate` (composition), `RiskClassifier`, `CapabilityRegistry`, `ConsentProvider`
    - `Authorize(ctx, ActionRequest) -> (ActionResponse, error)`:
      1. Look up agent's capabilities from registry (via team/role)
      2. If action not in capability set → DENY with "capability not in role"
      3. Classify risk via RiskClassifier → get RiskClass + RiskLevel
      4. If DENY → return denial with risk classification
      5. If DEFER → call ConsentProvider.RequestConsent(), block on channel with 300s timeout
      6. If ALLOW → call SkillGate.InterceptToolCall() for PII scrubbing
      7. Return ActionResponse with classification
    - Fail-closed: any error in steps 1-6 → DENY with error message
    - Zero package-level globals — struct-based, constructor-injected
  - Write comprehensive tests:
    - Unknown agent → DENY
    - Known agent, capability in role → ALLOW
    - Known agent, capability NOT in role → DENY
    - High-risk action → DEFER → timeout → auto-DENY
    - High-risk action → DEFER → consent granted → ALLOW
    - Broker crash/nil dependency → DENY (fail-closed)
    - Circular dependency: agent A calls B calls A → cycle detected at depth 5
  - Benchmark: `BenchmarkBrokerAuthorize` → p99 < 10ms for in-memory evaluation

  **Must NOT do**:
  - Do NOT modify `pkg/governor/` — broker COMPOSES SkillGate, doesn't change it
  - Do NOT create a new dispatch pipeline — broker is a pre-check interceptor
  - Do NOT import `internal/` packages

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — depends on Tasks 6, 7, 8
  - **Parallel Group**: Wave 2 (with Tasks 10-15)
  - **Blocks**: Tasks 12, 13, 22, 40
  - **Blocked By**: Tasks 6, 7, 8

  **References**:
  - `bridge/pkg/governor/skillgate.go:17` — Governor struct implements SkillGate. Broker holds a `SkillGate` interface reference, calls `InterceptToolCall()` in step 6
  - `bridge/pkg/secretary/approvals.go:263` — `evaluateCondition()` pattern for policy evaluation. Broker's risk check follows similar logic
  - `bridge/pkg/capability/CONSENT_DESIGN.md` — ConsentProvider contract from Task 8
  - `bridge/pkg/capability/types.go` — ActionRequest, ActionResponse, RiskLevel types from Task 6

  **Acceptance Criteria**:
  - [ ] `go test ./pkg/capability/... -v` → PASS (all broker tests)
  - [ ] `go test -bench=BenchmarkBrokerAuthorize -benchtime=5s ./pkg/capability/` → p99 < 10ms
  - [ ] Fail-closed test: inject nil classifier → DENY returned, no panic
  - [ ] Circular dependency test: cycle detected and denied at depth 5
  - [ ] `go vet ./pkg/capability/...` → PASS

  **QA Scenarios**:

  ```
  Scenario: Agent with valid role and low-risk action gets ALLOW
    Tool: Bash
    Steps:
      1. `cd bridge && go test -v -run TestBrokerAuthorize_Allow ./pkg/capability/...`
    Expected Result: ALLOW response, PII scrubbing applied, latency < 10ms
    Failure Indicators: DENY for valid capability, or latency > 10ms
    Evidence: .sisyphus/evidence/task-9-allow.txt

  Scenario: Agent requests capability outside role → DENY
    Tool: Bash
    Steps:
      1. `cd bridge && go test -v -run TestBrokerAuthorize_DenyCapability ./pkg/capability/...`
    Expected Result: DENY with "capability not in role" reason
    Failure Indicators: ALLOW for unauthorized capability
    Evidence: .sisyphus/evidence/task-9-deny.txt

  Scenario: High-risk action DEFER → 300s timeout → auto-DENY
    Tool: Bash
    Steps:
      1. `cd bridge && go test -v -run TestBrokerAuthorize_DeferTimeout ./pkg/capability/...`
    Expected Result: DEFER initially, then auto-DENY after 300s (use short timeout in test)
    Failure Indicators: No timeout, blocks forever
    Evidence: .sisyphus/evidence/task-9-defer.txt

  Scenario: Broker crash → fail-closed DENY
    Tool: Bash
    Steps:
      1. `cd bridge && go test -v -run TestBrokerAuthorize_FailClosed ./pkg/capability/...`
    Expected Result: DENY returned, no panic, clear error message
    Failure Indicators: Panic or ALLOW on error
    Evidence: .sisyphus/evidence/task-9-failclosed.txt

  Scenario: Benchmark p99 < 10ms
    Tool: Bash
    Steps:
      1. `cd bridge && go test -bench=BenchmarkBrokerAuthorize -benchtime=5s ./pkg/capability/`
    Expected Result: p99 < 10ms per authorization check
    Failure Indicators: p99 >= 10ms
    Evidence: .sisyphus/evidence/task-9-bench.txt
  ```

  **Commit**: YES
  - Message: `feat(capability): implement CapabilityBroker with fail-closed authorization`
  - Files: `bridge/pkg/capability/broker.go`, `bridge/pkg/capability/broker_test.go`
  - Pre-commit: `go test ./pkg/capability/...`

- [x] 10. Build risk taxonomy engine

  **What to do**:
  - Create `bridge/pkg/capability/risk_classifier.go`:
    - `RiskClassifierImpl` struct: maps action+params to RiskClass + RiskLevel
    - Taxonomy table (hardcoded, configurable later):
      | Action Pattern | Risk Class | Default Level |
      |---------------|------------|---------------|
      | `browser.*` | external_communication | ALLOW |
      | `browser.fill_forms` | external_communication | DEFER |
      | `email.send` | external_communication | DEFER |
      | `email.draft` | external_communication | ALLOW |
      | `secret.access` | credential_use | DEFER |
      | `secret.request` | credential_use | DEFER |
      | `payment.*` | payment | DEFER |
      | `pii.*` | identity_pii | DEFER |
      | `doc.*` | file_exfiltration | ALLOW |
      | `doc.delete` | file_exfiltration | DEFER |
      | `*` (default) | irreversible_action | DENY |
    - `Classify(ctx, action, params) -> (RiskClass, RiskLevel)` method
    - Configurable: read overrides from Bridge config (TOML)
  - Write tests for every entry in taxonomy + unknown action → DENY

  **Must NOT do**:
  - Do NOT activate SecurityConfig tiers (v0.9.0 scope)
  - Do NOT import `pkg/security/`

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 9, 11-15, after Tasks 6-7)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 9 (used by broker)
  - **Blocked By**: Tasks 6, 7

  **References**:
  - `bridge/pkg/capability/types.go` — RiskClass and RiskLevel types from Task 6
  - OWASP Agentic Top 10 ASI02 (Tool Misuse), ASI03 (Identity Abuse) — canonical risk framework
  - LangGraph Stage0 Gate pattern: ALLOW/DENY/DEFER routing based on tool side-effects

  **Acceptance Criteria**:
  - [ ] `go test ./pkg/capability/... -run TestRiskClassify` → PASS
  - [ ] Unknown action → `irreversible_action` + DENY
  - [ ] `browser.browse` → `external_communication` + ALLOW
  - [ ] `payment.process` → `payment` + DEFER

  **QA Scenarios**:

  ```
  Scenario: All risk classifications match taxonomy
    Tool: Bash
    Steps:
      1. `cd bridge && go test -v -run TestRiskClassify ./pkg/capability/...`
      2. Verify each action pattern produces correct RiskClass + RiskLevel
    Expected Result: All 10+ patterns match, unknown defaults to DENY
    Failure Indicators: Misclassified action
    Evidence: .sisyphus/evidence/task-10-taxonomy.txt

  Scenario: Unknown action defaults to DENY
    Tool: Bash
    Steps:
      1. `cd bridge && go test -v -run TestRiskClassify_Unknown ./pkg/capability/...`
    Expected Result: irreversible_action + DENY
    Failure Indicators: ALLOW for unknown action
    Evidence: .sisyphus/evidence/task-10-unknown.txt
  ```

  **Commit**: YES (groups with Task 9)
  - Message: `feat(capability): implement risk taxonomy engine with OWASP classification`
  - Files: `bridge/pkg/capability/risk_classifier.go`, `bridge/pkg/capability/risk_classifier_test.go`

- [x] 11. Extend audit schema for governance events + artifact lineage

  **What to do**:
  - Read `bridge/pkg/audit/audit.go:14-43` — existing EventType constants
  - Add governance event types: `capability_requested`, `capability_granted`, `capability_denied`, `capability_deferred`, `broker_intercept`, `artifact_created`, `artifact_transformed`, `artifact_lineage_query`
  - Read `bridge/pkg/audit/compliance.go:42-64` — ComplianceEntry struct
  - Add lineage fields (versioned — don't break hash chain for existing entries):
    - Create `ComplianceEntryV2` struct extending V1 with: `AgentID`, `TeamID`, `WorkflowID`, `StepID`, `ParentEntryID`, `IntentDescription string`
    - Hash chain uses V2 fields for V2 entries, V1 fields for V1 entries
  - Create `bridge/pkg/audit/lineage.go`:
    - `LogArtifactLineage(ctx, sourceType, sourceID, transformation, outputType, outputID, agentID)` method
    - `GetArtifactLineage(ctx, artifactID) -> ([]LineageEntry, error)` query method
    - Lineage entries stored in separate `artifact_lineage` SQLite table
  - Write tests for lineage recording and querying

  **Must NOT do**:
  - Do NOT modify existing ComplianceEntry struct — use versioned V2
  - Do NOT break existing hash chain integrity

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 9-10, 12-15)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 49 (Phase 8 audit extension)
  - **Blocked By**: Task 7

  **References**:
  - `bridge/pkg/audit/audit.go:14-43` — Existing 20 EventType constants. Add governance types here
  - `bridge/pkg/audit/compliance.go:42-64` — ComplianceEntry struct with HMAC-SHA256 hash chain. V2 must extend without breaking V1
  - `bridge/pkg/audit/tamper_evident.go` — Tamper-evident log with hash chain. V2 entries must chain correctly with V1

  **Acceptance Criteria**:
  - [ ] New event types added to `audit.go`
  - [ ] `ComplianceEntryV2` struct defined with lineage fields
  - [ ] `artifact_lineage` table created in audit database
  - [ ] `GetArtifactLineage()` returns ordered chain
  - [ ] Existing tests still pass (V1 hash chain intact)
  - [ ] `go test ./pkg/audit/...` → PASS

  **QA Scenarios**:

  ```
  Scenario: Artifact lineage recorded and queryable
    Tool: Bash
    Steps:
      1. `cd bridge && go test -v -run TestArtifactLineage ./pkg/audit/...`
      2. Record lineage: source=X, transform="extract", output=Y
      3. Query lineage for Y → returns chain [X → Y]
    Expected Result: Lineage chain returned in correct order
    Failure Indicators: Empty lineage or incorrect order
    Evidence: .sisyphus/evidence/task-11-lineage.txt

  Scenario: V1 hash chain integrity preserved
    Tool: Bash
    Steps:
      1. `cd bridge && go test -v -run TestTamperEvident ./pkg/audit/...`
    Expected Result: All existing hash chain tests pass
    Failure Indicators: Hash mismatch or integrity error
    Evidence: .sisyphus/evidence/task-11-hash.txt
  ```

  **Commit**: YES
  - Message: `feat(audit): add governance events, artifact lineage, and ComplianceEntryV2`
  - Files: `bridge/pkg/audit/audit.go`, `bridge/pkg/audit/lineage.go`, `bridge/pkg/audit/lineage_test.go`
  - Pre-commit: `go test ./pkg/audit/...`

- [x] 12. Wire CapabilityBroker into StepExecutor (workflow pipeline)

  **What to do**:
  - Read `bridge/pkg/secretary/orchestrator_integration.go:704-759` — `executeStep()` flow
  - Add `CapabilityBroker` field to `StepExecutorConfig` struct (line ~139-164)
  - In `executeStep()`, BEFORE `executeStepWithAgent()`:
    - Build `ActionRequest` from step config (agentID from step.AgentIDs[0], action from step type/params)
    - Call `broker.Authorize(ctx, req)`
    - If DENY → return error with denial reason
    - If DEFER → block until consent received or timeout
    - If ALLOW → proceed to existing execution
  - Log broker decision via audit (use Task 11's governance events)
  - Update `cmd/bridge/setup_broker.go` (created in Task 1) to create Broker and inject into StepExecutor
  - Write integration tests: broker blocks unauthorized step, allows authorized step

  **Must NOT do**:
  - Do NOT create a new execution path — insert check into existing `executeStep()`
  - Do NOT modify `pkg/governor/`

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — depends on Task 9
  - **Parallel Group**: Wave 2
  - **Blocks**: Tasks 22, 24
  - **Blocked By**: Tasks 9, 11

  **References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:704-759` — `executeStep()` entry point. Insert broker check BEFORE line 704's agent selection
  - `bridge/pkg/secretary/orchestrator_integration.go:139-164` — `StepExecutorConfig` struct. Add `CapabilityBroker` field here
  - `bridge/pkg/secretary/orchestrator_integration.go:461-493` — `checkApproval()` pattern. Follow this same pattern for broker injection
  - `bridge/cmd/bridge/setup_broker.go` — Broker wiring file (created in Task 1, populated here)

  **Acceptance Criteria**:
  - [ ] `StepExecutorConfig` has `CapabilityBroker` field
  - [ ] `executeStep()` calls broker before agent spawn
  - [ ] DENY → step fails with broker denial reason
  - [ ] ALLOW → existing execution proceeds unchanged
  - [ ] Broker nil → all steps execute (backward compatible, no broker = no restriction)
  - [ ] `go test ./pkg/secretary/...` → PASS

  **QA Scenarios**:

  ```
  Scenario: Broker denies unauthorized step
    Tool: Bash
    Steps:
      1. `cd bridge && go test -v -run TestBrokerDeniesStep ./pkg/secretary/...`
      2. Configure broker to DENY "email.send" for test agent
      3. Execute workflow with email.send step
    Expected Result: Step fails with "capability denied: email.send"
    Failure Indicators: Step executes despite denial
    Evidence: .sisyphus/evidence/task-12-deny.txt

  Scenario: Broker allows authorized step — existing flow unchanged
    Tool: Bash
    Steps:
      1. `cd bridge && go test -v -run TestBrokerAllowsStep ./pkg/secretary/...`
      2. Configure broker to ALLOW all for test agent
      3. Execute existing workflow test
    Expected Result: Same behavior as without broker (backward compatible)
    Failure Indicators: Different test results vs. pre-broker baseline
    Evidence: .sisyphus/evidence/task-12-allow.txt

  Scenario: Nil broker — all steps execute (backward compatible)
    Tool: Bash
    Steps:
      1. `cd bridge && go test -v -run TestNilBrokerAllowsAll ./pkg/secretary/...`
    Expected Result: All existing tests pass with nil broker
    Failure Indicators: Any test fails when broker is nil
    Evidence: .sisyphus/evidence/task-12-nil.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): wire CapabilityBroker into StepExecutor as pre-check middleware`
  - Files: `bridge/pkg/secretary/orchestrator_integration.go`, `bridge/cmd/bridge/setup_broker.go`
  - Pre-commit: `go test ./pkg/secretary/...`

- [x] 13. Wire CapabilityBroker into SkillExecutor (skill/MCP pipeline)

  **What to do**:
  - Read `bridge/internal/skills/executor.go:71` — `SkillExecutor.ExecuteSkill()` entry point
  - The broker lives in `pkg/capability/` and SkillExecutor is in `internal/skills/`. Cross the boundary via interface injection:
    - Define `Authorizer` interface in `pkg/interfaces/capability.go` (may already exist): `Authorize(ctx, ActionRequest) -> (ActionResponse, error)`
    - Inject `Authorizer` into SkillExecutor via constructor/config
    - In `ExecuteSkill()`, before calling the skill, call `authorizer.Authorize()` with agent context
  - Read `bridge/pkg/mcp/router.go:170` — MCP tool router dispatch. Add broker check before tool dispatch
  - Update `cmd/bridge/setup_mcp.go` (from Task 1) to inject broker

  **Must NOT do**:
  - Do NOT import `pkg/capability/` from `internal/` — use interface in `pkg/interfaces/`
  - Do NOT create a third dispatch mechanism

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 9-12, 14-15)
  - **Parallel Group**: Wave 2
  - **Blocks**: None
  - **Blocked By**: Task 9

  **References**:
  - `bridge/internal/skills/executor.go:71` — SkillExecutor.ExecuteSkill(). Add authorizer check before skill execution
  - `bridge/pkg/mcp/router.go:170` — MCP router dispatch. Add broker check before tool dispatch
  - `bridge/pkg/interfaces/capability.go` — Authorizer interface (from Task 7). Internal packages can import this

  **Acceptance Criteria**:
  - [ ] SkillExecutor checks broker before skill execution
  - [ ] MCPRouter checks broker before tool dispatch
  - [ ] DENY → skill/tool call fails with denial reason
  - [ ] No direct import of `pkg/capability/` from `internal/`
  - [ ] `go test ./internal/skills/...` → PASS

  **QA Scenarios**:

  ```
  Scenario: Broker denies unauthorized tool call via MCP
    Tool: Bash
    Steps:
      1. `cd bridge && go test -v -run TestBrokerDeniesTool ./pkg/mcp/...`
    Expected Result: Tool call rejected with capability denial
    Failure Indicators: Tool executes despite denial
    Evidence: .sisyphus/evidence/task-13-mcp-deny.txt

  Scenario: No import boundary violation
    Tool: Bash
    Steps:
      1. `cd bridge && go vet ./internal/skills/...`
      2. Verify no import of `pkg/capability/`
    Expected Result: Clean vet, no boundary violation
    Failure Indicators: Import cycle or internal importing pkg/capability
    Evidence: .sisyphus/evidence/task-13-boundary.txt
  ```

  **Commit**: YES (groups with Task 12)
  - Message: `feat(skills): wire CapabilityBroker into SkillExecutor and MCP router`
  - Files: `bridge/internal/skills/executor.go`, `bridge/pkg/mcp/router.go`, `bridge/cmd/bridge/setup_mcp.go`

- [x] 14. Assess V6Microkernel wiring gaps

  **What to do**:
  - Audit the following wiring gaps identified by Metis:
    - `ApprovalEngine` created (main.go:2573) but `StepExecutor` gets `ApprovalEngine: nil` (main.go:2634)
    - `SealedKeystore` (681 lines) never instantiated in production
    - `SecurityConfig` (675 lines, 5 tiers) never loaded in production
    - `TrustedWorkflowEngine` created but not wired into step execution
    - Vault client never passed to MCPRouter (main.go:2514-2520)
  - For each gap: assess whether it blocks broker work. If yes, document as Phase 1 prerequisite. If no, document as known tech debt for v0.9.0.
  - Write findings to `bridge/doc/V6_WIRING_GAPS.md`

  **Must NOT do**:
  - Do NOT fix the gaps — just assess and document
  - Do NOT activate SealedKeystore or SecurityConfig (v0.9.0 scope)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: [`writing`]

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 9-13, 15)
  - **Parallel Group**: Wave 2
  - **Blocks**: None (assessment only)
  - **Blocked By**: Task 1

  **References**:
  - `bridge/cmd/bridge/main.go:2573` — ApprovalEngine creation
  - `bridge/cmd/bridge/main.go:2634` — StepExecutor with ApprovalEngine: nil
  - `bridge/pkg/keystore/sealed_keystore.go` — 681-line SealedKeystore (unwired)
  - `bridge/pkg/security/categories.go` — 675-line SecurityConfig (unwired)

  **Acceptance Criteria**:
  - [ ] `bridge/doc/V6_WIRING_GAPS.md` created with assessment per gap
  - [ ] Each gap has verdict: BLOCKS_BROKER / KNOWN_TECH_DEBT / NOT_RELEVANT

  **QA Scenarios**:

  ```
  Scenario: Wiring gaps document is complete
    Tool: Bash
    Steps:
      1. Verify V6_WIRING_GAPS.md exists
      2. Verify it covers: ApprovalEngine, SealedKeystore, SecurityConfig, TrustedWorkflowEngine, Vault-to-MCPRouter
    Expected Result: Document covers all 5 gaps with verdicts
    Failure Indicators: Missing gap assessment
    Evidence: .sisyphus/evidence/task-14-assessment.txt
  ```

  **Commit**: YES
  - Message: `docs(bridge): assess V6Microkernel wiring gaps for broker prerequisites`
  - Files: `bridge/doc/V6_WIRING_GAPS.md`

- [x] 15. Encapsulate PendingApproval globals into struct

  **What to do**:
  - Read `bridge/pkg/secretary/pending_approval.go` — identify package-level globals:
    - `var pendingApps = make(map[string]chan piiResponse)` (line 68)
    - `var ApprovalTimeout = DefaultPIIApprovalTimeout` (line 30)
    - `var piiAlertDispatcher func(...)` (line 34)
  - Create `PendingApprovalManager` struct with these as fields
  - Refactor `PendingApproval()` function into method: `pm.RequestApproval(ctx, ...)`
  - Replace global state with struct instance injected at wiring time
  - Update `cmd/bridge/setup_secretary.go` (from Task 1) to create and inject the manager
  - Update all callers of `PendingApproval()` to use the struct method

  **Must NOT do**:
  - Do NOT change the approval semantics — pure encapsulation
  - Do NOT consolidate with other approval systems

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: [`/git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 9-14)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 9 (broker uses approval flow)
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/secretary/pending_approval.go:30` — `ApprovalTimeout` global variable
  - `bridge/pkg/secretary/pending_approval.go:34` — `piiAlertDispatcher` global function variable
  - `bridge/pkg/secretary/pending_approval.go:68` — `pendingApps` global map

  **Acceptance Criteria**:
  - [ ] Zero package-level mutable globals in `pending_approval.go`
  - [ ] `PendingApprovalManager` struct with constructor
  - [ ] All callers updated to use struct method
  - [ ] `go test ./pkg/secretary/...` → PASS

  **QA Scenarios**:

  ```
  Scenario: No package-level mutable globals remain
    Tool: Bash
    Steps:
      1. `grep -n "^var " bridge/pkg/secretary/pending_approval.go`
      2. Assert: only const or immutable declarations remain
    Expected Result: No mutable `var` at package level
    Failure Indicators: `var pendingApps` or `var ApprovalTimeout` still present
    Evidence: .sisyphus/evidence/task-15-globals.txt

  Scenario: All secretary tests pass after refactoring
    Tool: Bash
    Steps:
      1. `cd bridge && go test ./pkg/secretary/... -count=1`
    Expected Result: All existing tests pass
    Failure Indicators: Any test failure
    Evidence: .sisyphus/evidence/task-15-tests.txt
  ```

  **Commit**: YES
  - Message: `refactor(secretary): encapsulate PendingApproval globals into struct`
  - Files: `bridge/pkg/secretary/pending_approval.go`, `bridge/cmd/bridge/setup_secretary.go`
  - Pre-commit: `go test ./pkg/secretary/...`

### Wave 3: Phase 1 Rust + Phase 2 Team Model (Parallel)

- [x] 16. Extend vault proto with capability_scope field
  - **What to do**: Add `capability_scope` field to `IssueTokenRequest` and `ConsumeTokenRequest` in `rust-vault/proto/governance.proto`. Add `CapabilityScope string` to `TokenEntry` in `ephemeral.rs`. Update `consume_token()` to validate scope.
  - **Must NOT do**: Do NOT break existing proto consumers (field is additive)
  - **References**: `rust-vault/proto/governance.proto:73-91`, `rust-vault/src/governance/ephemeral.rs:114`
  - **QA**: `cargo test` in rust-vault/ → PASS. Existing Issue/Consume RPCs still work without scope field.
  - **Commit**: `feat(vault): add capability_scope to governance proto`

- [x] 17. Implement vault scope validation in ephemeral.rs
  - **What to do**: Validate `capability_scope` on `consume_token()` — if stored scope doesn't match request scope, deny. Add `capability_scope_violation` event to EventNotifier. Write tests for scope mismatch, scope match, and empty scope (backward compatible).
  - **References**: `rust-vault/src/governance/ephemeral.rs:206`, `rust-vault/src/governance/event_notifier.rs:73`
  - **QA**: Scope mismatch → token denied. Scope match → token consumed. Empty scope → backward compatible.
  - **Commit**: `feat(vault): implement capability scope validation on token consume`

- [x] 18. Define Team/Member/Role/Template types in pkg/team/
  - **What to do**: Create `bridge/pkg/team/types.go` with: `Team{ID, Name, TemplateID, SharedContext, LifecycleState, Budgets}`, `TeamMember{TeamID, AgentID, RoleName, AllowedTools, AllowedSecretPrefixes, BrowserContextID, Priority}`, `TeamRole{Name, Capabilities CapabilitySet, Description}`, `TeamTemplate{ID, Name, Roles []TeamRole, DefaultContext}`. Lifecycle states: Active, Suspended, Dissolved. Each type gets JSON tags + Validate() method.
  - **Must NOT do**: Do NOT add to `pkg/secretary/types.go` — new package
  - **References**: `bridge/pkg/secretary/types.go` (525 lines of workflow types — separate concern), `bridge/pkg/capability/types.go` (CapabilitySet type from Task 6)
  - **QA**: Round-trip JSON tests, Validate() rejects empty TeamID, unknown LifecycleState.
  - **Commit**: `feat(team): define team data model types`

- [x] 19. Build TeamRole registry
  - **What to do**: Create `bridge/pkg/team/roles.go` with built-in roles: `team_lead` (all capabilities), `browser_specialist` (browse+extract), `form_filler` (BlindFill submit), `doc_analyst` (ingest/summarize/reference), `email_clerk` (draft/send through approval), `supervisor` (synthesize, request HITL). Each role maps to a CapabilitySet. `GetRole(name) → (TeamRole, error)`, `ListRoles() → []TeamRole`, `ValidateRoleAssignment(role, existingRoles) error`.
  - **References**: Roadmap Phase 2 Task 2.3 — CTO's role taxonomy adopted
  - **QA**: Each role has correct capabilities. `team_lead` has superset. `browser_specialist` cannot send email.
  - **Commit**: `feat(team): build TeamRole registry with built-in roles`

- [x] 20. Build team store (SQLCipher schema + CRUD)
  - **What to do**: Create `bridge/pkg/team/store.go` with SQLCipher schema: `teams`, `team_members`, `team_roles` tables. Methods: `CreateTeam`, `GetTeam`, `ListTeams`, `UpdateTeam`, `AddMember`, `RemoveMember`, `DissolveTeam`. Use `BEGIN IMMEDIATE` transactions for concurrent modification safety. Optimistic locking via `version` column. Handle edge cases: team with zero members, last member removal auto-dissolves, agent in multiple teams.
  - **Must NOT do**: Do NOT use package-level globals — struct-based with SQLCipher connection injected
  - **References**: `bridge/pkg/secretary/schema.sql` — existing SQLite schema pattern. Follow this style. Use `BEGIN IMMEDIATE` for write transactions.
  - **QA**: CRUD cycle: create → get → update → dissolve. Concurrent modification: two writes, second fails with version mismatch. Zero-member team: all actions DENY.
  - **Commit**: `feat(team): build team store with SQLCipher schema and CRUD`

- [x] 21. Build team RPC service
  - **What to do**: Create `bridge/pkg/team/service.go` implementing `TeamService` interface from `pkg/interfaces/team.go`. RPC handlers: `create_team`, `get_team`, `list_teams`, `add_member`, `remove_member`, `dissolve_team`, `assign_role`, `get_capabilities_for_member`. Register in `cmd/bridge/setup_teams.go` (from Task 1). Wire into existing RPC server.
  - **References**: `bridge/pkg/rpc/server.go` — existing RPC registration pattern
  - **QA**: `curl` each RPC method against running bridge. Team creation returns team_id. Member addition returns member_id.
  - **Commit**: `feat(team): build team RPC service`

### Wave 4: Phase 2 Wiring + Phase 3 Bridge-Local Executors

- [x] 22. Wire TeamRole → CapabilityBroker
  - **What to do**: When broker resolves capabilities for an agent, it now checks: does agent have a team membership? If yes, look up role from team store → get CapabilitySet from role registry → intersect with action request. If no team, use existing non-team authorization. Broker's `CapabilityRegistry` implementation delegates to `TeamRole` lookup.
  - **Must NOT do**: Do NOT require team membership for all agents — non-team agents use existing authorization
  - **References**: `bridge/pkg/capability/broker.go` (Task 9), `bridge/pkg/team/roles.go` (Task 19)
  - **QA**: Agent with `browser_specialist` role → can browse, cannot send email. Agent without team → existing behavior.
  - **Commit**: `feat(team): wire TeamRole to CapabilityBroker for role-based scoping`

- [x] 23. Extend WorkflowStep with team_id + assigned_member_id
  - **What to do**: Add `TeamID string` and `AssignedMemberID string` fields to `WorkflowStep` in `pkg/secretary/types.go`. Update schema.sql with migration. Update `DependencyValidator` to validate team exists and member is in team when these fields are set. Make fields optional — existing workflows without teams continue to work.
  - **Must NOT do**: Do NOT require team_id on all steps — optional for backward compatibility
  - **References**: `bridge/pkg/secretary/types.go:64-96`, `bridge/pkg/secretary/schema.sql`
  - **QA**: Workflow with team_id validates team exists. Workflow without team_id passes validation (backward compat).
  - **Commit**: `feat(secretary): extend WorkflowStep with team assignment fields`

- [x] 24. Activate BridgeLocalRegistry in StepExecutor
  - **What to do**: In `executeStep()`, BEFORE agent spawn: check `stepConfig.ExecutionMode`. If `bridge_local`, call `ExecuteBridgeLocal()` instead of `factory.Spawn()`. Create `BridgeLocalRegistry` instance in `cmd/bridge/setup_secretary.go`. Register placeholder handler for testing.
  - **Must NOT do**: Do NOT skip broker check for bridge-local steps — they go through the same governance
  - **References**: `bridge/pkg/secretary/bridge_local_registry.go:77` — `ExecuteBridgeLocal()` function (exists, not wired). `bridge/pkg/secretary/orchestrator_integration.go:761` — current spawn path (add mode branch here)
  - **QA**: Step with `execution_mode: bridge_local` → handler called, NO container spawned. Step without mode → existing container spawn.
  - **Commit**: `feat(secretary): activate BridgeLocalRegistry in StepExecutor`

- [x] 25. Register browser_execute as bridge-local handler
  - **What to do**: Create `bridge/pkg/browser/handler.go` implementing `BridgeLocalHandler`. Accepts `BrowserIntent` (typed artifact), calls Jetski via existing browser client, returns `BrowserResult`. Register as `"browser_execute"` in BridgeLocalRegistry. Pass through broker for capability check.
  - **References**: `bridge/pkg/browser/client.go` — existing browser client, `bridge/pkg/capability/types.go` — BrowserIntent/BrowserResult
  - **QA**: `browser_execute` handler with valid BrowserIntent → BrowserResult returned. Unauthorized → DENY before browser call.
  - **Commit**: `feat(browser): register browser_execute as bridge-local handler`

- [x] 26. Register doc_query as bridge-local handler
  - **What to do**: Create `bridge/pkg/sidecar/doc_handler.go` implementing `BridgeLocalHandler`. Accepts `DocumentRef` + query text, calls sidecar gRPC QueryDocuments, returns `ExtractedChunkSet`. Register as `"doc_query"` in BridgeLocalRegistry.
  - **References**: `bridge/pkg/sidecar/client.go` — gRPC sidecar client
  - **QA**: `doc_query` with valid DocumentRef → chunks returned. Empty collection → graceful empty result.
  - **Commit**: `feat(sidecar): register doc_query as bridge-local handler`

- [x] 27. Build BrowserContextManager
  - **What to do**: Create `bridge/pkg/browser/context_manager.go`. Manages browser contexts per agent: `AllocateContext(agentID) → contextID`, `ReleaseContext(agentID)`, `GetContext(agentID) → contextID`. One context per agent, enforced. Integration with Jetski multi-context RPC (if available).
  - **References**: `jetski/internal/cdp/proxy.go` — Jetski CDP proxy
  - **QA**: Allocate → success. Allocate again for same agent → returns existing context. Release → context freed.
  - **Commit**: `feat(browser): build BrowserContextManager for per-agent isolation`

- [x] 28. Wire typed artifacts into bridge-local executors
  - **What to do**: Replace generic `json.RawMessage` I/O in bridge-local handlers with typed artifact structs. `browser_execute` handler accepts `BrowserIntent` struct, returns `BrowserResult` struct. `doc_query` handler accepts `DocumentRef` + query, returns `ExtractedChunkSet`. Add validation at handler boundary.
  - **References**: `bridge/pkg/secretary/bridge_local_registry.go` — generic handler signature. `bridge/pkg/capability/types.go` — typed artifacts from Task 6
  - **QA**: Invalid BrowserIntent → validation error. Valid BrowserIntent → typed BrowserResult (not raw JSON).
  - **Commit**: `feat(secretary): wire typed artifacts into bridge-local executor handlers`

### Wave 5: Phase 3 Completion + Phase 5 Email

- [x] 29. Build QueryDocuments gRPC in Rust sidecar
  - **What to do**: Add `QueryDocuments` RPC to `sidecar.proto`. Accepts: collection_id, query_text, clearance_level, max_results. Returns: encrypted chunks. Implement in `sidecar/src/grpc/server.rs`.
  - **References**: `sidecar/src/grpc/server.rs`, `sidecar/src/document/qdrant.rs` — Qdrant search
  - **QA**: `cargo test --lib` → PASS. Query returns ranked chunks.
  - **Commit**: `feat(sidecar): implement QueryDocuments gRPC RPC`

- [x] 30. Per-team Qdrant collection creation
  - **What to do**: On team creation (Task 20), create `team_{id}` collection in Qdrant via sidecar gRPC. Store collection reference in team's SharedContext.
  - **References**: `sidecar/src/document/qdrant.rs` — `create_collection()` method
  - **QA**: Create team → Qdrant collection exists with team name.
  - **Commit**: `feat(team): create per-team Qdrant collection on team creation`

- [x] 31. CAPTCHA delegation blocker
  - **What to do**: Add `captcha` blocker type in orchestrator. Jetski detects CAPTCHA → emits blocker with screenshot → routed to team lead or human via Matrix.
  - **References**: `jetski/internal/cdp/proxy.go`, `bridge/pkg/secretary/orchestrator_integration.go`
  - **QA**: CAPTCHA detected → blocker emitted with screenshot URL.
  - **Commit**: `feat(jetski): add CAPTCHA delegation blocker`

- [x] 32. Extend EmailDispatcher with team routing
  - **What to do**: Match inbound email to team by address/rules. Route to email specialist role within the team.
  - **References**: `bridge/pkg/email/dispatcher.go`
  - **QA**: Email to team address → routed to team's email_clerk.
  - **Commit**: `feat(email): add team-based email routing`

- [x] 33. Build IMAP inbox management
  - **What to do**: Create `bridge/pkg/email/imap.go` with: ListFolders, ListMessages, ReadMessage, Archive, MarkRead. Scoped to email specialist role.
  - **QA**: List folders → returns server folders. Read message → returns parsed email.
  - **Commit**: `feat(email): implement IMAP inbox management`

- [x] 34. Build email thread context tracker
  - **What to do**: Create `bridge/pkg/email/thread_tracker.go`. Track In-Reply-To/References headers. `get_thread_context(threadID)` returns full thread.
  - **QA**: Two related emails → thread context returns both in order.
  - **Commit**: `feat(email): build email thread context tracker`

- [x] 35. Build email draft workflow + templates
  - **What to do**: Create `bridge/pkg/email/drafts.go` (save/update/send draft) and `bridge/pkg/email/templates.go` (per-team templates with variable substitution).
  - **QA**: Save draft → retrieve → update → send. Template with {{name}} → substituted.
  - **Commit**: `feat(email): implement draft workflow and team templates`

### Wave 6: Phase 6 — Dynamic Secret Request Flow (Parallel with Wave 5)

- [x] 36. Define SecretRequestEvent Matrix event
  - **What to do**: Define `app.armorclaw.secret_request` Matrix event type: `{request_id, agent_id, team_id, credential_name, target_domain, reason, risk_class}`.
  - **References**: `bridge/pkg/email/events.go` — existing Matrix event pattern
  - **QA**: Event serializes/deserializes correctly.
  - **Commit**: `feat(secrets): define SecretRequestEvent Matrix event`

- [x] 37. Build SecretRequestManager
  - **What to do**: Create `bridge/pkg/team/secret_request.go`. Agent emits secret request → Bridge publishes Matrix event → blocks on response channel (300s timeout, same pattern as EmailApprovalManager). Follow struct-based pattern from Task 15 (no globals).
  - **References**: `bridge/pkg/email/hitl_approval.go` — same pattern but for secret requests
  - **QA**: Request → published → response received → SecretRef returned. Timeout → auto-deny.
  - **Commit**: `feat(secrets): build SecretRequestManager with HITL flow`

- [x] 38. Add request_secret bridge-local step type
  - **What to do**: Register `request_secret` handler in BridgeLocalRegistry. Container declares need → Bridge executes HITL → returns SecretRef placeholder (not raw value).
  - **QA**: Step with `request_secret` → SecretRef returned, no raw value.
  - **Commit**: `feat(secrets): add request_secret bridge-local step type`

- [x] 39. Build BlindFillCard Android composable
  - **What to do**: Create `BlindFillCard.kt` following existing PiiApprovalCard pattern (370 lines). Renders: target domain, risk badge, approve/deny buttons. Deep link: `armorclaw://secret/approve/{request_id}`. Reuse Material 3 components, color theming, callback patterns from existing cards.
  - **References**: `applications/ArmorChat/.../PiiApprovalCard.kt` — 370-line pattern to follow. `applications/ArmorChat/.../EmailApprovalCard.kt` — 194-line simpler pattern
  - **QA**: Card renders with test data. Approve button fires callback. Deny button fires callback. Countdown timer works.
  - **Commit**: `feat(android): build BlindFillCard composable for secret requests`

- [x] 40. Wire secret request through approval policy
  - **What to do**: Secret requests use broker's risk classification. `credential_use` risk class → approval mode. High-risk (payment, identity) → biometric required. Low-risk (generic API key) → auto-approve per policy.
  - **References**: `bridge/pkg/capability/risk_classifier.go` (Task 10) — `secret.request` → credential_use → DEFER
  - **QA**: High-risk → DEFER requires approval. Low-risk → auto-ALLOW.
  - **Commit**: `feat(secrets): wire secret requests through unified approval policy`

- [x] 41. On approval → store secret in keystore + return SecretRef
  - **What to do**: User provides value → encrypted storage in SQLCipher keystore → return `{{VAULT:field:hash}}` placeholder to agent. Agent never sees raw value.
  - **References**: `bridge/pkg/keystore/keystore.go` — SQLCipher-backed storage
  - **QA**: Approve with test value → keystore has encrypted entry → agent receives SecretRef hash.
  - **Commit**: `feat(secrets): store approved secrets and return SecretRef placeholder`

### Wave 7: Phase 7 — LLM-Driven Team Composition

- [x] 42. Extract AIClient interface to pkg/interfaces/
  - **What to do**: Extract `AIClient` interface from `bridge/internal/ai/client.go` to `bridge/pkg/interfaces/ai_client.go`. Original internal package implements the interface (no restructuring needed).
  - **Must NOT do**: Do NOT move `internal/ai/` to `pkg/ai/` — just extract interface
  - **QA**: `go build ./...` → PASS. Secretary package can now reference AIClient via interface.
  - **Commit**: `refactor(ai): extract AIClient interface to pkg/interfaces`

- [x] 43. Build structured output parser for LLM responses
  - **What to do**: Create `bridge/pkg/capability/structured_output.go`. Parse LLM JSON responses into Go structs. Schema validation, retry on malformed output (max 3 retries), fallback to default team composition.
  - **QA**: Valid JSON → parsed struct. Invalid JSON → retry → fallback. Max retries → error.
  - **Commit**: `feat(capability): build structured output parser for LLM team composition`

- [x] 44. Build TeamComposer LLM decomposition
  - **What to do**: Create `bridge/pkg/team/composer.go`. Accepts goal string → calls LLM with prompt including available roles, typed artifact definitions → returns TeamPlan with subtasks, role assignments, artifact contracts. Prompt engineering: include TeamRole registry + typed artifact schemas.
  - **References**: `bridge/pkg/team/roles.go` (Task 19), `bridge/pkg/capability/types.go` (Task 6)
  - **QA**: "Research and book a flight" → team plan with browser_specialist + form_filler roles.
  - **Commit**: `feat(team): build TeamComposer with LLM-driven decomposition`

- [x] 45. Build TeamPlanExecutor
  - **What to do**: Create `bridge/pkg/team/executor.go`. Creates Team, instantiates members via AgentFactory with role specialization, launches orchestrated workflow using typed handoffs. Validates every delegation through CapabilityBroker (Task 22).
  - **QA**: TeamPlan → team created → members spawned → steps executed → team dissolved.
  - **Commit**: `feat(team): build TeamPlanExecutor for governed team execution`

- [x] 46. Enhance AgentFactory for role specialization
  - **What to do**: Extend `factory.Spawn()` with `SpecializationConfig{Role, Skills, SystemPrompt, ScopedSecrets}`. Role determines system prompt template and available skills.
  - **References**: `bridge/pkg/studio/factory.go` — existing agent factory
  - **QA**: Spawn with browser_specialist role → system prompt includes browse instructions, skills limited to browser tools.
  - **Commit**: `feat(studio): enhance AgentFactory with role specialization`

- [x] 47. Wire TeamComposer into TaskScheduler
  - **What to do**: Add `auto_team: true` flag support. When set, TaskScheduler delegates to TeamComposer instead of linear workflow.
  - **QA**: Task with `auto_team: true` → TeamComposer called → team created → executed.
  - **Commit**: `feat(scheduler): wire TeamComposer into TaskScheduler with auto_team flag`

- [x] 48. Fallback/retry for team execution
  - **What to do**: Same agent retry → failover to different agent → escalate to lead → escalate to human.
  - **QA**: Agent failure → retry → different agent → success. All agents fail → escalation to human.
  - **Commit**: `feat(team): implement fallback and retry for team execution`

### Wave 8: Phase 8 — Observability, Governance & License

- [x] 49. Extend audit schema for team events
  - **What to do**: Add team-specific events: `team_created`, `team_dissolved`, `member_added`, `member_removed`, `role_assigned`, `delegation_sent`, `handoff_complete`. Extend ComplianceEntryV2 with team fields.
  - **QA**: Team creation → audit event logged with team_id.
  - **Commit**: `feat(audit): add team lifecycle events to audit schema`

- [x] 50. Per-team metrics collection
  - **What to do**: Create `bridge/pkg/team/metrics.go`. Track: token usage, cost, latency per role, handoff success rate, secret access count, approval rate by risk class. Expose via existing Prometheus endpoint.
  - **QA**: Team execution → metrics incremented. Prometheus scrape → team metrics visible.
  - **Commit**: `feat(team): add per-team metrics collection`

- [x] 51. Team governance controls
  - **What to do**: Add config: `max_members_per_team`, `max_teams_per_instance`, `allowed_roles[]`. Enforce in team store CRUD.
  - **QA**: Create team with 20 members → rejected if max is 10.
  - **Commit**: `feat(team): add governance controls for team size and role limits`

- [x] 52. Wire team governance to approval policy
  - **What to do**: Team-level policy overrides: "this team auto-approves external_communication but requires HITL for payment". Broker checks team-specific overrides before global policy.
  - **QA**: Team with payment auto-approve → payment DEFER bypassed. Team without override → global policy applies.
  - **Commit**: `feat(team): wire team-level policy overrides to approval engine`

- [x] 53. Team timeline UI in admin panel
  - **What to do**: Extend admin panel with team-level events, artifact lineage visualization, intent trace per step. React/Vite components.
  - **References**: `applications/admin-panel/src/pages/AuditLogPage.tsx` — existing audit page pattern
  - **QA**: Team timeline loads → shows events in chronological order → artifact lineage clickable.
  - **Commit**: `feat(admin): add team timeline UI with lineage visualization`

- [x] 54. License system team-aware enforcement
  - **What to do**: Update license-server for team-aware instance counting: `max_teams`, `max_members_per_team` per tier.
  - **References**: `license-server/main.go`
  - **QA**: License with max_teams=3 → 4th team creation rejected.
  - **Commit**: `feat(license): add team-aware enforcement to license system`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./...` + `go build ./...` + `go test ./...` + `cargo test`. Review all changed files for: `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names. Verify no changes to `pkg/governor/`.
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Integration QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (broker + teams + executors + audit). Test edge cases: zero capabilities, team dissolution, broker crash, role escalation, circular deps. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination: Task N touching Task M's files. Flag unaccounted changes. Verify no changes to `pkg/governor/`, `pkg/runtime/`, no approval consolidation.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Phase -1**: `refactor(bridge): extract main.go wiring into setup_*.go files` — cmd/bridge/setup_*.go
- **Wave 1**: `feat(capability): define typed artifact contracts and interfaces` — pkg/capability/types.go, pkg/interfaces/capability.go, pkg/interfaces/consent.go
- **Wave 1**: `fix(sidecar): verify qdrant v1.7 builder, tighten docker volumes` — sidecar/, deploy/
- **Wave 1**: `feat(email): wire attachments to document sidecar` — bridge/pkg/email/, bridge/pkg/sidecar/
- **Wave 2**: `feat(capability): implement CapabilityBroker with risk taxonomy` — pkg/capability/
- **Wave 2**: `feat(audit): add governance events and artifact lineage` — pkg/audit/
- **Wave 2**: `feat(secretary): wire broker into step and skill execution` — pkg/secretary/, internal/skills/
- **Wave 3**: `feat(vault): add capability_scope to governance proto` — rust-vault/
- **Wave 3**: `feat(team): define team data model and role registry` — pkg/team/
- **Wave 3**: `feat(team): build team store and RPC service` — pkg/team/, pkg/rpc/
- **Wave 4**: `feat(team): wire TeamRole to CapabilityBroker` — pkg/team/, pkg/capability/
- **Wave 4**: `feat(secretary): activate BridgeLocalRegistry for browser and doc` — pkg/secretary/, pkg/browser/, pkg/sidecar/

---

## Success Criteria

### Verification Commands
```bash
# Core packages compile and test
go test ./pkg/capability/... ./pkg/team/... ./pkg/interfaces/... -v
# Broker latency
go test -bench=BenchmarkBrokerAuthorize -benchtime=5s ./pkg/capability/
# Rust vault
cd rust-vault && cargo test
# No changes to governor
git diff --stat HEAD -- bridge/pkg/governor/  # Expected: empty
# No changes to runtime
git diff --stat HEAD -- bridge/pkg/runtime/   # Expected: empty
# Security constraint preserved
grep -r "NetworkMode.*none" bridge/pkg/docker/  # Expected: matches
```

### Final Checklist
- [x] All "Must Have" present
- [x] All "Must NOT Have" absent (especially: zero changes to pkg/governor/)
- [x] All tests pass (Go + Rust + Android)
- [x] Broker fail-closed verified
- [x] DEFER timeout (300s) verified
- [x] No approval system consolidation (only ConsentProvider interface)
- [x] main.go properly refactored into setup_*.go
