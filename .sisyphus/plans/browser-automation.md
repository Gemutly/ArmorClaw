# Browser Automation: Bridge-Brokered, Jetski-Executed

## TL;DR

> **Quick Summary**: Implement the Browser Automation Strategy v2.3 — standardize ArmorClaw on a Bridge-brokered, Jetski-executed browser model. The Bridge owns semantic browser methods, policy, and audit; Jetski owns CDP execution, security, and telemetry. Five phases from stabilization through robustness.
>
> **Deliverables**:
> - `BrowserBroker` interface + `JetskiBroker` implementation in Bridge
> - Jetski approval client wired to Bridge PII methods
> - Jetski deployment blockers fixed (CGO, env vars, network)
> - `learned_charts` table + normalization pipeline + chart injection
> - Chart validation, audit trail, replay-through-approval enforcement
> - Selector fallback, confidence scoring, chart versioning
> - 3 new harness test scripts extending existing test infrastructure
>
> **Estimated Effort**: Large (12–16 weeks per strategy document)
> **Parallel Execution**: YES — 6 waves
> **Critical Path**: T6 (BrowserBroker interface) → T8 (JetskiBroker.Navigate) → T11-T14 (semantic methods) → T18 (normalization) → T22 (validator) → T25-T29 (robustness) → F1-F4

---

## Context

### Original Request
Implement Browser Automation Strategy v2.3 for ArmorClaw: standardize on Bridge-brokered, Jetski-executed browser model with 5 phases. Legacy browser-service rollback is a **temporary escape hatch**, not permanent dual architecture.

### Interview Summary
**Key Discussions**:
- Investigated Jetski as sidecar and found 6 blocking gaps (hardcoded URL, REST vs CDP mismatch, approval dead code, protocol mismatch, SQLCipher/CGO, Docker network)
- Strategy v2.3 adopts findings with transport locked: CDP WebSocket (:9222) for execution, JSON-RPC HTTP (:9223) for management
- `injectLearnedSkills` is dead code (never called in production) — must fix in Phase 0

**Research Findings**:
- Bridge has 3 disconnected call paths: RPC stub (no HTTP), secretary handler (HTTP), queue processor (HTTP)
- Jetski approval client is nil at runtime — ~15 lines to wire
- Lighthouse already has a `charts` table (blessed/signed) — learned_charts must be separate
- Chartmaker has `StateCompiler` (TypeScript) that compiles RecordedAction[] into NavChart
- Sonar buffer passes empty sessionID — per-session tracking broken

### Metis Review
**Identified Gaps** (addressed):
- **injectLearnedSkills dead code**: Promoted to Phase 0 prerequisite (Task 7)
- **Three call paths**: Plan accounts for all 3 — BrowserBroker replaces Client in paths B+C, then wires path A
- **ServiceResponse/ServiceError types**: Guardrail added — broker must produce same response shapes
- **Lighthouse vs learned_charts naming**: Separate tables, separate databases, separate concerns
- **FillRequest.Sensitive**: Must be compatible with existing ServiceFillField.ValueRef
- **e2e_fullpath_test.go mock**: Must continue to pass through Phase 0-1 (guardrail)

---

## Work Objectives

### Core Objective
Make Jetski the functional browser sidecar for ArmorClaw, with Bridge owning the semantic contract and Jetski owning CDP execution, in 5 sequential phases.

### Concrete Deliverables
- `bridge/pkg/browser/broker.go` — BrowserBroker interface
- `bridge/pkg/browser/jetski_broker.go` — JetskiBroker implementation
- Jetski approval client wired to Bridge PII methods
- Jetski Dockerfile and docker-compose fixes
- `bridge/pkg/browser/chart_types.go` — Go NavChart types
- `bridge/pkg/browser/chart_store.go` — learned_charts persistence
- `bridge/pkg/browser/chart_validator.go` — validation rules
- `bridge/pkg/browser/normalizer.go` — CDP-to-semantic normalization
- 3 new test scripts in tests/
- Evidence of all Phase 0 exit criteria passing

### Definition of Done
- [ ] `navigate("https://example.com")` works end-to-end through Jetski
- [ ] `fill("#email", "...")` works with sensitive flag triggering approval
- [ ] `click("#submit")` works
- [ ] `extract(...)` returns structured data
- [ ] `screenshot()` returns image bytes
- [ ] Sensitive fill paths trigger approval; raw PII is never logged
- [ ] Health/status/session checks pass
- [ ] Bridge↔Jetski connectivity survives restart
- [ ] Happy path no longer depends on legacy browser-service

### Must Have
- BrowserBroker exposes 14 interface methods: 11 documented semantic browser operations plus 3 internal lifecycle methods (Health, Close, Reconnect)
- JetskiBroker implementing against CDP WebSocket (:9222) and JSON-RPC HTTP (:9223)
- FillRequest with Sensitive flag compatible with existing PII approval flow
- learned_charts table in secretary SQLite (separate from Lighthouse charts)
- Chart validation rejecting raw PII before storage
- Replay always routes through normal approval/policy path
- All existing tests continue to pass

### Architecture Lock (NON-NEGOTIABLE — bind all phases)

These rules are locked before implementation begins. No task may violate these.

1. **Bridge public contract = semantic browser broker.** Agents, workflows, and ArmorChat speak `browser.navigate`, `browser.fill`, `browser.click` — never raw CDP.
2. **Jetski execution transport = CDP WebSocket `:9222`.** Used exclusively by the Bridge's JetskiBroker. Not exposed to agents or external callers.
3. **Jetski management transport = JSON-RPC HTTP `:9223`.** Used for health, session lifecycle, and administrative operations.
4. **No semantic API added to Jetski.** Jetski remains a CDP proxy + security layer. The Bridge is the only semantic boundary.
5. **No raw CDP exposed beyond Bridge internals.** The `jetski_broker.go` file is the ONLY place CDP commands are constructed. All other code uses BrowserBroker methods.
6. **Legacy browser-service fallback sunsets after Phase 4.** If browser job success rate ≥ 92% over 500+ consecutive jobs with zero fallback invocations in 30 days, the fallback code path and `ARMORCLAW_BROWSER_FALLBACK` flag are removed. The sunset milestone is tracked in the Phase 4 exit criteria.

### Must NOT Have (Guardrails)
- Do NOT remove `browser-service/` directory until Phase 4 completes
- Do NOT modify `ServiceResponse` / `ServiceError` types — broker must produce same shapes
- Do NOT change Jetski port assignments (9222/9223 locked)
- Do NOT merge `learned_charts` into Lighthouse's `charts` table — separate databases
- Do NOT modify `e2e_fullpath_test.go` mock structure through Phase 0-1
- Do NOT bypass `BridgeLocalRegistry` pattern for broker registration
- Do NOT add semantic REST endpoints to Jetski — Bridge translates internally
- Do NOT merge Chartmaker's TypeScript StateCompiler — normalization is pure Go
- Do NOT add retry/circuit-breaker in Phase 0 — that's Phase 4 work
- Do NOT bundle `injectLearnedSkills` wiring with chart work — it's a Phase 0 prerequisite

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (Go httptest + shell harness with tests/lib/)
- **Automated tests**: YES (tests-after — extend existing harness)
- **Framework**: Go testing + shell harness (tests/lib/)

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/browser-automation/`.

- **Bridge Go code**: Use `go test ./...` + `go build ./...`
- **Jetski Go code**: Use `go build ./...` + `go vet ./...`
- **Integration**: Shell harness with `tests/lib/` helpers (load_env.sh, common_output.sh, assert_json.sh)
- **Docker**: `docker compose` commands for build/health verification
- **API/CDP**: curl for JSON-RPC endpoints, wscat/websocat for WebSocket

> **Note on verification scope**: All implementation tasks and routine verification are agent-executed (zero human intervention required for task completion). The Final Verification Wave (F1-F4) includes an agent-executed end-to-end QA sweep (F3) that exercises every QA scenario across all tasks. Human review of the consolidated F1-F4 results is a release gate, not a task completion requirement.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Phase 0 foundation — 7 tasks, ALL independent):
├── T1: Unify Bridge browser URL config [quick]
├── T2: Add Jetski approval env vars [quick]
├── T3: Wire Jetski approval client [quick]
├── T4: Fix Jetski Dockerfile CGO/SQLCipher [quick]
├── T5: Fix docker-compose env vars + network [quick]
├── T6: Create BrowserBroker interface + types [quick]
└── T7: Wire injectLearnedSkills into production [quick]

Wave 2 (Phase 0 integration — 4 tasks, depends on Wave 1):
├── T8: Implement JetskiBroker.Navigate (depends: T1, T6) [deep]
├── T9: Register JetskiBroker with feature flag (depends: T1, T6, T8) [quick]
├── T16: Wire RPC Path A handlers through Broker (depends: T9) [deep]
└── T10a: Phase 0 exit criteria harness test (depends: T4, T5, T8, T9, T16) [unspecified-high]

Wave 3 (Phase 1 — parallel semantic methods, 5 tasks):
├── T11: JetskiBroker.Fill + Sensitive flag (depends: T8) [deep]
├── T12: JetskiBroker.Click + WaitForElement (depends: T8) [unspecified-high]
├── T13: JetskiBroker.Extract + Screenshot (depends: T8) [unspecified-high]
├── T14: JetskiBroker job lifecycle (depends: T8) [unspecified-high]
└── T15: Legacy fallback mechanism (depends: T9) [unspecified-high]

Wave 3b (Phase 1 gate — 1 task):
└── T10b: Phase 1 exit criteria harness test (depends: T11, T12, T13, T14) [unspecified-high]

Wave 4 (Phase 2 — recording + charts, 5 tasks):
├── T17: Fix Sonar session tracking (depends: T3) [quick]
├── T18: Go NavChart type definitions (no deps) [quick]
├── T19: Normalization pipeline (depends: T17, T18) [deep]
├── T20: learned_charts table + persistence (depends: T18) [unspecified-high]
└── T21: Chart injection into skills path (depends: T7, T20) [unspecified-high]

Wave 5 (Phase 3 — security, 4 tasks):
├── T22: Chart validator (depends: T18, T20) [deep]
├── T23: Replay-through-approval enforcement (depends: T14, T22) [deep]
├── T24: Audit trail for chart lifecycle (depends: T22) [unspecified-high]
└── T25: PII test scanner (depends: T22) [quick]

Wave 6 (Phase 4 — robustness, 5 tasks):
├── T26: Selector fallback + confidence (depends: T19, T22) [deep]
├── T27: Multi-tab/popup handling [STRETCH] (depends: T14) [unspecified-high]
├── T28: Chart versioning + rollback (depends: T20, T22) [unspecified-high]
├── T29: Performance benchmarking (depends: T15) [unspecified-high]
└── T30: Replay diagnostics [STRETCH] (depends: T23) [unspecified-high]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Agent-executed end-to-end QA sweep (unspecified-high)
└── F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay

Critical Path: T6 → T8 → T11 → T19 → T22 → T26 → F1-F4 → user okay
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 7 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1 | — | T8, T9 | 1 |
| T2 | — | T3 | 1 |
| T3 | T2 | T17 | 1 |
| T4 | — | T10 | 1 |
| T5 | — | T10 | 1 |
| T6 | — | T8, T9 | 1 |
| T7 | — | T21 | 1 |
| T8 | T1, T6 | T9, T11-T14 | 2 |
| T9 | T1, T6, T8 | T15, T16 | 2 |
| T10a | T4, T5, T8, T9, T16 | Wave 3 | 2 |
| T10b | T11, T12, T13, T14 | Wave 4 | 3b |
| T11 | T8 | T23 | 3 |
| T12 | T8 | — | 3 |
| T13 | T8 | — | 3 |
| T14 | T8 | T23, T27 | 3 |
| T15 | T9 | T29 | 3 |
| T16 | T9 | T10a | 2 |
| T17 | T3 | T19 | 4 |
| T18 | — | T19, T20, T22 | 4 |
| T19 | T17, T18 | T26 | 4 |
| T20 | T18 | T21, T22, T28 | 4 |
| T21 | T7, T20 | T26, T28 | 4 |
| T22 | T18, T20 | T23, T24, T25, T26, T28 | 5 |
| T23 | T14, T22 | T30 | 5 |
| T24 | T22 | — | 5 |
| T25 | T22 | — | 5 |
| T26 | T19, T22 | — | 6 |
| T27 | T14 | — | 6 |
| T28 | T20, T22 | — | 6 |
| T29 | T15 | — | 6 |
| T30 | T23 | — | 6 |

### Agent Dispatch Summary

- **Wave 1**: **7** — T1-T5 → `quick`, T6 → `quick`, T7 → `quick`
- **Wave 2**: **4** — T8 → `deep`, T9 → `quick`, T16 → `deep`, T10a → `unspecified-high`
- **Wave 3**: **5** — T11 → `deep`, T12-T14 → `unspecified-high`, T15 → `unspecified-high`
- **Wave 3b**: **1** — T10b → `unspecified-high`
- **Wave 4**: **5** — T17-T18 → `quick`, T19 → `deep`, T20-T21 → `unspecified-high`
- **Wave 5**: **4** — T22-T23 → `deep`, T24 → `unspecified-high`, T25 → `quick`
- **Wave 6**: **5** — T26 → `deep`, T27-T30 → `unspecified-high` (T27, T30 are stretch goals — skip if time-boxed)
- **FINAL**: **4** — F1 → `oracle`, F2-F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

> **CDP Isolation Rule**: Only `bridge/pkg/browser/jetski_broker.go` may construct or send CDP commands. No other Bridge package may speak CDP directly. All browser interaction flows through the BrowserBroker interface.

- [x] 1. Unify Bridge Browser URL Configuration

  **What to do**:
  - In `bridge/cmd/bridge/setup_secretary.go:122`, replace hardcoded `"http://localhost:3002"` with values read from config
  - Split the single `BrowserConfig.ServiceURL` into two explicit endpoint fields:
    ```go
    type BrowserConfig struct {
        Enabled     bool
        CDPUrl      string  // CDP WebSocket URL for browser execution (e.g. "ws://jetski:9222")
        RPCUrl      string  // JSON-RPC HTTP URL for management (e.g. "http://jetski:9223")
        Backend     string  // "jetski" | "legacy"
        Fallback    bool    // temporary escape hatch
        Timeout     int
        MaxRetries  int
        RetryDelay  int
        Stealth     BrowserStealthConfig
        Queue       BrowserQueueConfig
    }
    ```
  - Env vars: `ARMORCLAW_BROWSER_CDP_URL` (default: `ws://localhost:9222`), `ARMORCLAW_BROWSER_RPC_URL` (default: `http://localhost:9223`)
  - For legacy backend: `CDPUrl` and `RPCUrl` are unused; a `LegacyURL` field (env: `ARMORCLAW_BROWSER_LEGACY_URL`, default: `http://localhost:3002`) replaces the old `ServiceURL`
  - Add `ARMORCLAW_BROWSER_BACKEND` env var (values: `jetski` | `legacy`, default: `jetski`)
  - Add `ARMORCLAW_BROWSER_FALLBACK` env var (bool, default: `true`)
  - In `setup_secretary.go`, when `backend=jetski`: construct JetskiBroker using `CDPUrl` + `RPCUrl`; when `backend=legacy`: construct Client using `LegacyURL`
  - In `processor.go`, align with same config fields

  **Must NOT do**:
  - Do NOT remove the existing browser-service REST Client — it's the legacy fallback path
  - Do NOT change any ServiceResponse/ServiceError types
  - Do NOT modify e2e_fullpath_test.go

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T2-T7)
  - **Blocks**: T8, T9
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/cmd/bridge/setup_secretary.go:100-140` — Current wiring of browser client with hardcoded URL at line 122
  - `bridge/pkg/config/config.go:407-428` — BrowserConfig struct — replace single ServiceURL with CDPUrl + RPCUrl + LegacyURL
  - `bridge/pkg/config/config.go:920-940` — Default config values including BrowserConfig defaults

  **API/Type References**:
  - `bridge/pkg/browser/client.go:224-240` — NewClient(url) constructor that takes ServiceURL string
  - `bridge/pkg/browser/processor.go:36-55` — ServiceProcessorConfig with ServiceURL, DefaultTimeout, MaxRetries

  **WHY Each Reference Matters**:
  - `setup_secretary.go:122` is the ONE place where the URL is hardcoded — this is the fix target
  - `config.go:407-428` shows all existing browser config fields and env var mappings — add new fields following same pattern
  - `processor.go:48` shows the SECOND default URL (localhost:3000) — align to LegacyURL default (localhost:3002)

  **Acceptance Criteria**:

  - [ ] `bridge/cmd/bridge/setup_secretary.go` reads browser URLs from config, not hardcoded
  - [ ] `bridge/pkg/config/config.go` has separate `CDPUrl`, `RPCUrl`, `LegacyUrl`, `Backend`, `Fallback` fields
  - [ ] Env vars: `ARMORCLAW_BROWSER_CDP_URL`, `ARMORCLAW_BROWSER_RPC_URL`, `ARMORCLAW_BROWSER_LEGACY_URL`, `ARMORCLAW_BROWSER_BACKEND`, `ARMORCLAW_BROWSER_FALLBACK`
  - [ ] `go build ./cmd/bridge/...` succeeds
  - [ ] `go test ./pkg/browser/...` passes (existing tests unaffected)

  **QA Scenarios:**

  ```
  Scenario: Bridge starts with Jetski backend config
    Tool: Bash
    Preconditions: Bridge binary built
    Steps:
      1. Set ARMORCLAW_BROWSER_BACKEND=jetski ARMORCLAW_BROWSER_CDP_URL=ws://jetski:9222 ARMORCLAW_BROWSER_RPC_URL=http://jetski:9223
       2. Run bridge with --dry-run or check config loading logs
       3. Verify logs show browser backend=jetski cdp=ws://jetski:9222 rpc=http://jetski:9223
     Expected Result: Config loads with Jetski URLs, no errors
     Failure Indicators: Config loads localhost:3000 or localhost:3002, panic on startup
    Evidence: .sisyphus/evidence/browser-automation/task-1-config-load.txt

  Scenario: Legacy fallback config still works
    Tool: Bash
    Preconditions: Bridge binary built
    Steps:
      1. Set ARMORCLAW_BROWSER_BACKEND=legacy ARMORCLAW_BROWSER_LEGACY_URL=http://localhost:3002
       2. Run bridge or check config
       3. Verify logs show browser backend=legacy legacy_url=http://localhost:3002
    Expected Result: Legacy config loads without error
    Failure Indicators: Config validation rejects legacy backend
    Evidence: .sisyphus/evidence/browser-automation/task-1-legacy-config.txt
  ```

  **Commit**: YES
  - Message: `fix(bridge): split BrowserConfig into CDP/RPC/Legacy endpoints, add backend selection`
  - Files: `bridge/cmd/bridge/setup_secretary.go`, `bridge/pkg/config/config.go`, `bridge/pkg/browser/processor.go`
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/browser/...`

- [x] 2. Add Jetski Approval Environment Variable Overrides

  **What to do**:
  - In `jetski/pkg/config/config.go`, add env var mappings in `applyEnvOverrides()` (lines 153-190) for the approval config fields:
    - `JETSKI_APPROVAL_ENABLED` → `cfg.Approval.Enabled`
    - `JETSKI_BRIDGE_URL` → `cfg.Approval.BridgeURL`
    - `JETSKI_ROOM_ID` → `cfg.Approval.RoomID`
    - `JETSKI_APPROVAL_TIMEOUT` → `cfg.Approval.Timeout`
  - Follow the exact same pattern as existing env var mappings (os.Getenv + strconv.ParseBool/Atoi + conditional assignment)
  - Add validation in `Validate()` (lines 192-234) for approval config: if `Approval.Enabled=true`, require non-empty `BridgeURL` and `RoomID`

  **Must NOT do**:
  - Do NOT change the ApprovalConfig struct fields
  - Do NOT change default values in defaultConfig
  - Do NOT add new config file keys (env-only for now)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T3-T7)
  - **Blocks**: T3
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `jetski/pkg/config/config.go:153-190` — `applyEnvOverrides()` function showing exact pattern for env var mapping with type conversion
  - `jetski/pkg/config/config.go:72-78` — `ApprovalConfig` struct with Enabled, BridgeURL, RoomID, Timeout, SensitiveOperations fields

  **API/Type References**:
  - `jetski/pkg/config/config.go:192-234` — `Validate()` function where approval validation should be added

  **WHY Each Reference Matters**:
  - `applyEnvOverrides()` lines 153-190 is the ONLY place env vars are consumed — add the 4 missing approval mappings here using the exact same os.Getenv pattern
  - `Validate()` is where config correctness is checked — approval requires BridgeURL when enabled

  **Acceptance Criteria**:

  - [ ] `applyEnvOverrides()` maps all 4 approval env vars
  - [ ] `Validate()` rejects approval.Enabled=true with empty BridgeURL
  - [ ] `go build ./...` in jetski/ succeeds
  - [ ] Existing config tests pass

  **QA Scenarios:**

  ```
  Scenario: Approval env vars override config
    Tool: Bash
    Preconditions: Jetski binary built
    Steps:
      1. Set JETSKI_APPROVAL_ENABLED=true JETSKI_BRIDGE_URL=http://bridge:8080 JETSKI_ROOM_ID=!room:test
      2. Run jetski with --log-level debug
      3. Check startup logs for approval config values
    Expected Result: Logs show approval enabled=true bridgeURL=http://bridge:8080 roomID=!room:test
    Failure Indicators: Approval shows as disabled or empty bridgeURL
    Evidence: .sisyphus/evidence/browser-automation/task-2-approval-env.txt

  Scenario: Validation rejects approval without BridgeURL
    Tool: Bash
    Preconditions: Jetski binary built
    Steps:
      1. Set JETSKI_APPROVAL_ENABLED=true with no JETSKI_BRIDGE_URL
      2. Run jetski
      3. Check for validation error in logs/output
    Expected Result: Startup fails with clear validation error about missing BridgeURL
    Failure Indicators: Starts successfully with approval enabled but no URL
    Evidence: .sisyphus/evidence/browser-automation/task-2-validation-error.txt
  ```

  **Commit**: YES
  - Message: `feat(jetski): add approval env var overrides to config`
  - Files: `jetski/pkg/config/config.go`
  - Pre-commit: `cd jetski && go build ./...`

- [x] 3. Wire Jetski Approval Client to Bridge PII Methods

  **What to do**:
  - In `jetski/cmd/observer/main.go:136`, replace `rpc.NewServer(nil)` with:
    1. Conditionally create `approval.NewApprovalClient(cfg.Approval.BridgeURL, cfg.Approval.RoomID, cfg.Approval.Timeout)` when `cfg.Approval.Enabled`
    2. Pass the approval client to `rpc.NewServer(approvalClient)`
    3. Also call `cdpProxy.SetApprovalClient(approvalClient)` and `cdpProxy.SetApprovalTimeout(cfg.Approval.Timeout)` after proxy creation (line 85-87)
  - In `jetski/internal/approval/matrix_client.go`, change `RequestApproval()` to send requests to Bridge's JSON-RPC 2.0 endpoint using `method: "pii.request"` format instead of REST POST to `/rpc/approval/request`
  - The request payload must include: `agent_id`, `skill_id`, `profile_id` fields that Bridge's `pii.request` handler expects
  - Map Jetski's 4 `OperationType` values (`session_create`, `navigation`, `file_download`, `pii_input`) to Bridge's PII operation types

  **Must NOT do**:
  - Do NOT change the ApprovalClient interface
  - Do NOT remove the fire-and-forget approval response handling
  - Do NOT add new operation types

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (after T2 merges)
  - **Parallel Group**: Wave 1 (starts after T2)
  - **Blocks**: T17
  - **Blocked By**: T2

  **References**:

  **Pattern References**:
  - `jetski/cmd/observer/main.go:84-136` — Current initialization flow showing where approval client creation should be inserted
  - `jetski/internal/approval/matrix_client.go:40-93` — Existing approval client constructor and RequestApproval method

  **API/Type References**:
  - `jetski/internal/approval/matrix_client.go:13-29` — OperationType enum and ApprovalRequest struct
  - `jetski/internal/cdp/proxy.go:55-57` — ApprovalChecker interface
  - `jetski/internal/cdp/proxy.go:92-98` — SetApprovalClient/SetApprovalTimeout methods (currently never called)
  - `jetski/internal/rpc/rpc.go:14-28` — Server struct with ac *ApprovalClient field and conditional handler registration

  **WHY Each Reference Matters**:
  - `main.go:136` is where `rpc.NewServer(nil)` makes approval dead — this is THE fix point
  - `matrix_client.go:56-93` shows the HTTP POST protocol that must change to JSON-RPC 2.0
  - `proxy.go:92-98` shows SetApprovalClient already exists but is never called — just need to call it

  **Acceptance Criteria**:

  - [ ] `main.go` creates approval client when `cfg.Approval.Enabled=true`
  - [ ] `cdpProxy.SetApprovalClient()` is called when approval is enabled
  - [ ] `rpc.NewServer()` receives non-nil approval client when enabled
  - [ ] Approval requests use JSON-RPC 2.0 format with `pii.request` method
  - [ ] `go build ./...` succeeds in jetski/

  **QA Scenarios:**

  ```
  Scenario: Approval client wires when enabled
    Tool: Bash
    Preconditions: Jetski binary built, Bridge running with PII handler
    Steps:
      1. Set JETSKI_APPROVAL_ENABLED=true JETSKI_BRIDGE_URL=http://bridge:8080 JETSKI_ROOM_ID=!room:test
      2. Start jetski
      3. Check logs for "approval client configured"
      4. Verify /rpc/approval/pending endpoint returns empty list (not 404)
    Expected Result: Jetski starts with approval client active, RPC endpoints registered
    Failure Indicators: Logs show "approval disabled" or /rpc/approval/pending returns 404
    Evidence: .sisyphus/evidence/browser-automation/task-3-approval-wired.txt

  Scenario: Free-Ride mode still works without approval
    Tool: Bash
    Preconditions: Jetski binary built
    Steps:
      1. Start jetski with no approval env vars
      2. Verify startup succeeds
      3. Verify /rpc/approval/pending returns 404 (not registered)
    Expected Result: Jetski starts in Free-Ride mode, approval endpoints absent
    Failure Indicators: Startup fails without approval config
    Evidence: .sisyphus/evidence/browser-automation/task-3-free-ride.txt
  ```

  **Commit**: YES
  - Message: `fix(jetski): wire approval client to Bridge PII methods`
  - Files: `jetski/cmd/observer/main.go`, `jetski/internal/approval/matrix_client.go`
  - Pre-commit: `cd jetski && go build ./...`

- [x] 4. Fix Jetski Dockerfile CGO/SQLCipher Conflict

  **What to do**:
  - The current `jetski/Dockerfile` line 9 uses `CGO_ENABLED=0` but `go.mod` requires `github.com/mutecomm/go-sqlcipher/v4` which needs CGO
  - **Option A (recommended)**: Use Go build tags to make SQLCipher optional. Add `//go:build cgo` to `sqlcipher_session.go` and create a stub `//go:build !cgo` version that falls back to plain SQLite with a log warning
  - Update the Dockerfile to build with `CGO_ENABLED=0` and `-tags=nosqlcipher` (or just rely on the build tag default)
  - For production deployments that need encryption, add a separate Dockerfile target with CGO enabled (add `gcc` and `libsqlcipher-dev` to build stage)
  - Update `security/session.go` to check at runtime which implementation is available

  **Must NOT do**:
  - Do NOT remove the SQLCipher code
  - Do NOT make encryption mandatory (Free-Ride mode must work without CGO)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1-T3, T5-T7)
  - **Blocks**: T10
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `jetski/Dockerfile:1-37` — Full 3-stage build: go build (CGO_ENABLED=0) + lightpanda fetch + alpine runtime
  - `jetski/internal/security/sqlcipher_session.go:46-87` — `NewSQLCipherSessionStore()` which will fail without CGO

  **API/Type References**:
  - `jetski/internal/security/session.go:8-50` — SaveSession/LoadSession facade that calls into SQLCipher store
  - `jetski/go.mod` — Lists `github.com/mutecomm/go-sqlcipher/v4` dependency

  **WHY Each Reference Matters**:
  - `Dockerfile:9` has `CGO_ENABLED=0` — this is the build-time blocker for SQLCipher
  - `sqlcipher_session.go:46-87` is the code that will panic at runtime without CGO — needs build tag isolation
  - `session.go` is the facade that can check which implementation is available

  **Acceptance Criteria**:

  - [ ] `docker build -f jetski/Dockerfile jetski/` succeeds without CGO
  - [ ] Container starts and `/rpc/health` returns OK
  - [ ] Log warning appears when SQLCipher unavailable (not a panic)
  - [ ] `go build -tags=cgo ./...` with CGO still compiles SQLCipher version

  **QA Scenarios:**

  ```
  Scenario: Docker build without CGO succeeds
    Tool: Bash
    Preconditions: Docker available
    Steps:
      1. Run docker build -f jetski/Dockerfile -t jetski:test jetski/
      2. Check build exit code
      3. Run docker run --rm jetski:test observer --help
    Expected Result: Build succeeds, binary runs and shows help
    Failure Indicators: Build fails with CGO/SQLCipher error
    Evidence: .sisyphus/evidence/browser-automation/task-4-docker-build.txt

  Scenario: Container starts with degraded encryption warning
    Tool: Bash
    Preconditions: jetski:test image built
    Steps:
      1. docker run --rm -p 9223:9223 jetski:test observer --port=9222 2>&1 | head -20
      2. Check logs for "encryption unavailable" or similar warning
      3. curl http://localhost:9223/rpc/health
    Expected Result: Container starts, health check passes, warning about encryption appears
    Failure Indicators: Panic on startup, health check fails
    Evidence: .sisyphus/evidence/browser-automation/task-4-degraded-mode.txt
  ```

  **Commit**: YES
  - Message: `fix(jetski): resolve CGO/SQLCipher Dockerfile conflict with build tags`
  - Files: `jetski/Dockerfile`, `jetski/internal/security/sqlcipher_session.go`, `jetski/internal/security/session.go`
  - Pre-commit: `cd jetski && go build ./...`

- [x] 5. Fix docker-compose Jetski Environment Variables and Network

  **What to do**:
  - In `docker-compose.jetski.yml`, fix the env var names to match `config.go`'s `applyEnvOverrides()`:
    - Remove `JETSKI_CDP_PORT=9222` (not consumed by config.go — the port comes from `JETSKI_PORT` or `--port` flag)
    - Change `JETSKI_ENGINE_PORT=9223` to `JETSKI_ENGINE_PORT=9333` (Lightpanda runs on 9333, not 9223)
    - Add `JETSKI_APPROVAL_ENABLED=false`, `JETSKI_BRIDGE_URL=http://bridge:8080`, `JETSKI_ROOM_ID=`
  - Fix network alignment: `docker-compose.jetski.yml` uses `bridge-net: external: armorclaw-bridge` but the main compose uses `bridge-net` with `internal: 172.26.0.0/24`. Ensure both reference the same external network or use the same subnet.
  - Add a named volume for session persistence: `jetski-sessions:/app/sessions` (read_only container needs volume mount)
  - Fix healthcheck to use correct port: `wget localhost:9223/rpc/health` (RPC is on 9223, not 9222 — the health endpoint is on the RPC mux, not the CDP WebSocket)

  **Must NOT do**:
  - Do NOT change port assignments (9222 CDP, 9223 RPC, 9333 Lightpanda)
  - Do NOT remove read_only security setting
  - Do NOT change the main docker-compose.yml

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1-T4, T6-T7)
  - **Blocks**: T10
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `docker-compose.jetski.yml:1-84` — Current Jetski overlay compose file with incorrect env vars and network config
  - `jetski/pkg/config/config.go:153-190` — `applyEnvOverrides()` showing which env var names are actually consumed

  **API/Type References**:
  - `jetski/cmd/observer/main.go:114-117` — Health endpoint registration on the HTTP mux (port 9223 RPC)
  - `jetski/cmd/observer/main.go:149` — Startup log confirming CDP port and RPC port 9223

  **WHY Each Reference Matters**:
  - `docker-compose.jetski.yml` has env vars that config.go doesn't recognize — they're silently ignored
  - `applyEnvOverrides()` shows the ACTUAL env var names that work — compose must use these exact names
  - Health endpoint is on the RPC mux (9223), not the CDP WebSocket (9222) — healthcheck is wrong

  **Acceptance Criteria**:

  - [ ] All env vars in docker-compose.jetski.yml match config.go `applyEnvOverrides()` names
  - [ ] `JETSKI_ENGINE_PORT` matches Lightpanda default (9333)
  - [ ] Network name matches main compose or is properly external
  - [ ] Session volume declared and mounted
  - [ ] `docker compose -f docker-compose.jetski.yml config` validates without errors

  **QA Scenarios:**

  ```
  Scenario: Docker compose config validates
    Tool: Bash
    Preconditions: docker-compose.jetski.yml present
    Steps:
      1. Run docker compose -f docker-compose.jetski.yml config
      2. Check exit code
      3. Verify environment section shows correct env var names
    Expected Result: Config validates, env vars match JETSKI_PORT/JETSKI_ENGINE_PORT/JETSKI_APPROVAL_ENABLED
    Failure Indicators: Validation error, or env vars show JETSKI_CDP_PORT (removed)
    Evidence: .sisyphus/evidence/browser-automation/task-5-compose-config.txt

  Scenario: Jetski container starts with correct config
    Tool: Bash
    Preconditions: Docker available, jetski image built
    Steps:
      1. docker compose -f docker-compose.jetski.yml up -d
      2. Wait 5 seconds
      3. docker logs jetski 2>&1 | head -20
      4. Verify startup shows correct ports (CDP 9222, RPC 9223)
    Expected Result: Container healthy, logs show correct ports, no config warnings
    Failure Indicators: Wrong ports, missing env var warnings, crash
    Evidence: .sisyphus/evidence/browser-automation/task-5-container-start.txt
  ```

  **Commit**: YES
  - Message: `fix(jetski): align docker-compose env vars with config struct, fix network`
  - Files: `docker-compose.jetski.yml`
  - Pre-commit: `docker compose -f docker-compose.jetski.yml config`

- [x] 6. Create BrowserBroker Interface and Types

  **What to do**:
  - Create `bridge/pkg/browser/broker.go` defining the `BrowserBroker` interface:
    ```go
    type BrowserBroker interface {
        StartJob(ctx context.Context, req StartJobRequest) (JobID, error)
        Navigate(ctx context.Context, jobID JobID, url string) error
        Fill(ctx context.Context, jobID JobID, req FillRequest) error
        Click(ctx context.Context, jobID JobID, selector string) error
        WaitForElement(ctx context.Context, jobID JobID, selector string, timeout time.Duration) error
        WaitForCaptcha(ctx context.Context, jobID JobID, timeout time.Duration) error
        WaitFor2FA(ctx context.Context, jobID JobID, timeout time.Duration) error
        Extract(ctx context.Context, jobID JobID, spec ExtractSpec) (ExtractResult, error)
        Screenshot(ctx context.Context, jobID JobID) ([]byte, error)
        Status(ctx context.Context, jobID JobID) (JobStatus, error)
        Complete(ctx context.Context, jobID JobID) error
        Fail(ctx context.Context, jobID JobID, err error) error
        List(ctx context.Context) ([]JobSummary, error)
        Cancel(ctx context.Context, jobID JobID) error
    }
    ```
  - Create `bridge/pkg/browser/broker_types.go` with all types: `JobID` (string), `StartJobRequest`, `FillRequest` (with `Sensitive bool`), `ExtractSpec`, `ExtractResult`, `JobStatus`, `JobSummary`
  - Ensure `FillRequest` is compatible with existing `ServiceFillField.ValueRef` — `Sensitive=true` maps to the PII approval path, `Sensitive=false` maps to direct fill
  - Ensure all broker methods return the same response shapes that `handler.go` produces (via `serviceResponseToBrowserResult()`)

  **Must NOT do**:
  - Do NOT implement the broker yet (just interface + types)
  - Do NOT modify existing client.go or handler.go
  - Do NOT change ServiceResponse/ServiceError types

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1-T5, T7)
  - **Blocks**: T8, T9
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/browser/client.go:242-338` — Existing REST Client showing the 11 methods and their request/response types that the broker must mirror
  - `bridge/pkg/browser/handler.go:166` — `serviceResponseToBrowserResult()` showing the conversion that must be preserved

  **API/Type References**:
  - `bridge/pkg/browser/client.go:88-134` — ServiceFillCommand and ServiceFillField showing ValueRef pattern for PII
  - `bridge/pkg/capability/types.go` — BrowserIntent and BrowserResult types that handler.go produces
  - `bridge/pkg/queue/browser_queue.go:21-41` — Existing BrowserCommand and BrowserJob types

  **WHY Each Reference Matters**:
  - `client.go:242-338` shows ALL 11 REST methods and their exact signatures — broker must expose equivalent operations
  - `client.go:88-134` shows how fill handles PII via ValueRef — FillRequest.Sensitive must map to this
  - `capability/types.go` shows the final output shape — broker results must be convertible to BrowserResult

  **Acceptance Criteria**:

  - [ ] `bridge/pkg/browser/broker.go` defines BrowserBroker interface with 14 methods
  - [ ] `bridge/pkg/browser/broker_types.go` defines all types
  - [ ] `go build ./pkg/browser/...` succeeds
  - [ ] Types are compatible with existing BrowserIntent/BrowserResult

  **QA Scenarios:**

  ```
  Scenario: Interface compiles and satisfies handler expectations
    Tool: Bash
    Preconditions: bridge/ compiles
    Steps:
      1. cd bridge && go build ./pkg/browser/...
      2. Verify broker.go and broker_types.go compile
      3. Check that FillRequest.Sensitive field exists
    Expected Result: Clean compilation, no errors
    Failure Indicators: Compilation error, missing fields
    Evidence: .sisyphus/evidence/browser-automation/task-6-interface-compiles.txt

  Scenario: Types convert to existing BrowserResult
    Tool: Bash (go test)
    Preconditions: Types defined
    Steps:
      1. Write a compile-time assertion that ExtractResult can produce BrowserResult-compatible data
      2. Run go test ./pkg/browser/...
    Expected Result: All tests pass, type compatibility confirmed
    Failure Indicators: Type mismatch compilation error
    Evidence: .sisyphus/evidence/browser-automation/task-6-type-compat.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): add BrowserBroker interface and types`
  - Files: `bridge/pkg/browser/broker.go`, `bridge/pkg/browser/broker_types.go`
  - Pre-commit: `cd bridge && go build ./pkg/browser/...`

- [x] 7. Wire injectLearnedSkills into Production Step Execution

  **What to do**:
  - `injectLearnedSkills()` is defined at `bridge/pkg/secretary/orchestrator_integration.go:1195-1231` but is NEVER called in the production step execution path
  - The production path is `executeStepWithAgent()` (around line 809). Currently it calls `recordSkillOutcomes` (L875) and `onSkillExtraction` (L877-879) but never injects skills before execution
  - Add a call to `injectLearnedSkills(config, taskDescription)` at the point where step config is prepared but before the agent container is spawned — approximately before the agent execution call
  - Ensure `StepExecutorConfig.SkillFinder` is non-nil (it comes from the orchestrator setup — verify it's wired)
  - Add a feature flag `ARMORCLAW_LEARNED_SKILLS_INJECTION` (default: true) so injection can be disabled if needed

  **Must NOT do**:
  - Do NOT modify `recordSkillOutcomes` or `onSkillExtraction` callbacks — those work correctly
  - Do NOT change the skill extraction strategies in `skills/extractor.go`
  - Do NOT add chart injection yet (that's Phase 2, Task T21)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1-T6)
  - **Blocks**: T21
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:1195-1231` — `injectLearnedSkills()` method showing exactly what it does (finds relevant skills, injects into config JSON)
  - `bridge/pkg/secretary/orchestrator_integration.go:809-882` — `executeStepWithAgent()` showing the production execution path where injection should be added

  **API/Type References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:87-99` — `LearnedSkillInfo` struct and `SkillFinder` interface
  - `bridge/pkg/secretary/orchestrator_integration.go:141-159` — `StepExecutorConfig` with SkillFinder field
  - `bridge/pkg/skills/learned_store.go` — `LearnedStore` with `FindForTask()` method
  - `bridge/pkg/skills/extractor.go:23-124` — `ExtractFromResult()` with 5 extraction strategies

  **Test References**:
  - `bridge/pkg/secretary/orchestrator_integration_test.go:1282-1555` — Comprehensive tests for inject/record/outcome cycle including mockSkillFinder

  **WHY Each Reference Matters**:
  - `orchestrator_integration.go:1195-1231` is the method that needs to be called — it's already written and tested
  - `orchestrator_integration.go:809-882` is WHERE it needs to be called — before agent spawn but after config prep
  - `orchestrator_integration_test.go:1298` shows TestInjectLearnedSkills_WithMatch proving the method works correctly

  **Acceptance Criteria**:

  - [ ] `injectLearnedSkills()` is called in the production step execution path
  - [ ] Feature flag `ARMORCLAW_LEARNED_SKILLS_INJECTION` controls injection
  - [ ] Existing test suite passes: `go test ./pkg/secretary/...`
  - [ ] `go build ./...` succeeds

  **QA Scenarios:**

  ```
  Scenario: Skills injected before agent execution
    Tool: Bash (go test)
    Preconditions: LearnedStore with a stored skill
    Steps:
      1. Run TestInjectLearnedSkills_WithMatch from orchestrator_integration_test.go
      2. Verify test passes (skill found and injected into config)
    Expected Result: Test passes, config contains relevant_skills array
    Failure Indicators: Test fails, config missing relevant_skills
    Evidence: .sisyphus/evidence/browser-automation/task-7-skill-injection.txt

  Scenario: Injection can be disabled via feature flag
    Tool: Bash (go test)
    Preconditions: ARMORCLAW_LEARNED_SKILLS_INJECTION=false
    Steps:
      1. Set env var and run tests
      2. Verify skills are NOT injected
    Expected Result: Config unchanged, no relevant_skills added
    Failure Indicators: Skills still injected despite flag being false
    Evidence: .sisyphus/evidence/browser-automation/task-7-flag-disable.txt
  ```

  **Commit**: YES
  - Message: `fix(bridge): wire injectLearnedSkills into production step execution`
  - Files: `bridge/pkg/secretary/orchestrator_integration.go`
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/secretary/...`

- [x] 8. Implement JetskiBroker with Navigate Method

  **What to do**:
  - Create `bridge/pkg/browser/jetski_broker.go` implementing `BrowserBroker` interface from T6
  - Constructor: `NewJetskiBroker(cdpURL string, rpcURL string, logger *slog.Logger) *JetskiBroker`
  - `StartJob()`: Call Jetski RPC `POST /rpc/session/create` to create a session, store mapping of JobID→sessionID
  - `Navigate(ctx, jobID, url)`: Send CDP command `Page.navigate` via WebSocket to Jetski's `:9222` proxy
    - Use `gorilla/websocket` (already in Bridge's go.mod via Jetski)
    - Construct CDP message: `{\"id\": N, \"method\": \"Page.navigate\", \"params\": {\"url\": \"...\"}}`
    - Read response, check for CDP error, return success/failure
  - Store active WebSocket connections in a sync.Map keyed by JobID
  - Implement basic CDP message ID counter (atomic increment)
  - `Status()`: Call Jetski RPC `GET /rpc/status` to check session status

  **Must NOT do**:
  - Do NOT add retry logic (Phase 4)
  - Do NOT implement other methods yet (T11-T14)
  - Do NOT change the existing Client or handler.go

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — foundation for T11-T14
  - **Parallel Group**: Wave 2 (with T9, T10)
  - **Blocks**: T9, T11, T12, T13, T14
  - **Blocked By**: T1, T6

  **References**:

  **Pattern References**:
  - `bridge/pkg/browser/client.go:242-278` — Existing Navigate implementation showing request/response types (ServiceNavigateCommand → ServiceResponse)
  - `jetski/internal/cdp/proxy.go:35-47` — CDPMessage and CDPError wire types showing exact CDP JSON format
  - `jetski/internal/cdp/proxy.go:100-125` — WebSocket Start() showing how CDP proxy handles connections

  **API/Type References**:
  - `jetski/internal/rpc/rpc.go:30-35` — RPC endpoints: `/rpc/session/create` (POST), `/rpc/session/close` (POST), `/rpc/status` (GET), `/rpc/health` (GET)
  - `bridge/pkg/browser/broker.go` — BrowserBroker interface (created in T6)
  - `bridge/pkg/browser/broker_types.go` — All broker types (created in T6)

  **WHY Each Reference Matters**:
  - `client.go:242-278` shows the existing Navigate contract — JetskiBroker.Navigate must produce equivalent results
  - `proxy.go:35-47` shows the exact CDP wire format — JetskiBroker must send this format to Jetski's WebSocket
  - `rpc.go:30-35` shows the management API for session lifecycle — JetskiBroker uses these for StartJob/Status

  **Acceptance Criteria**:

  - [ ] `jetski_broker.go` implements BrowserBroker interface (compiles)
  - [ ] `StartJob()` creates session via Jetski RPC and returns JobID
  - [ ] `Navigate()` sends CDP Page.navigate via WebSocket and returns success
  - [ ] `Status()` queries Jetski RPC for session status
  - [ ] `go build ./pkg/browser/...` succeeds

  **QA Scenarios:**

  ```
  Scenario: Navigate through Jetski CDP proxy
    Tool: Bash
    Preconditions: Jetski running on localhost:9222/9223, Lightpanda engine up
    Steps:
      1. Build bridge test binary or use go test
      2. Create JetskiBroker("ws://localhost:9222", "http://localhost:9223")
      3. Call StartJob() → get JobID
      4. Call Navigate(jobID, "https://example.com")
      5. Verify CDP response has no error
    Expected Result: Navigate succeeds, no CDP error in response
    Failure Indicators: WebSocket connection refused, CDP error response, timeout
    Evidence: .sisyphus/evidence/browser-automation/task-8-navigate.txt

  Scenario: Navigate fails gracefully when Jetski unreachable
    Tool: Bash
    Preconditions: Jetski NOT running
    Steps:
      1. Create JetskiBroker with Jetski URLs
      2. Call StartJob()
      3. Verify error returned (not panic)
    Expected Result: Clear error "Jetski unreachable" or connection refused
    Failure Indicators: Panic, goroutine leak, hanging
    Evidence: .sisyphus/evidence/browser-automation/task-8-navigate-error.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): implement JetskiBroker with Navigate method`
  - Files: `bridge/pkg/browser/jetski_broker.go`
  - Pre-commit: `cd bridge && go build ./pkg/browser/...`

- [x] 9. Register JetskiBroker with Backend Feature Flag

  **What to do**:
  - In `bridge/cmd/bridge/setup_secretary.go`, modify the browser handler registration to use the feature flag from T1:
    ```go
    var handler func(json.RawMessage) (json.RawMessage, error)
    if cfg.Browser.Backend == "jetski" {
        broker := browser.NewJetskiBroker(cfg.Browser.CDPUrl, cfg.Browser.RPCUrl, logger)
        handler = browser.NewBrokerHandler(broker)
    } else {
        handler = browser.Handler(browser.NewClient(cfg.Browser.LegacyURL))
    }
    bridgeLocalRegistry.Register(browser.HandlerName, handler)
    ```
  - Create `bridge/pkg/browser/broker_handler.go` — a `BrokerHandler(broker BrowserBroker)` adapter that wraps the broker in the same `func(ctx, config json.RawMessage) (json.RawMessage, error)` signature as the existing `Handler(client)`
  - The BrokerHandler dispatches actions the same way as `handler.go:72-87` but calls broker methods instead of client methods
  - Add logging for every broker call (method, jobID, duration, error)
  - When `ARMORCLAW_BROWSER_FALLBACK=true` and Jetski fails, fall back to legacy Client with a warning log

  **Must NOT do**:
  - Do NOT remove the existing `Handler(client)` registration path
  - Do NOT modify e2e_fullpath_test.go
  - Do NOT add circuit breaker (Phase 4)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — depends on T8
  - **Parallel Group**: Wave 2 (with T8, T10)
  - **Blocks**: T15, T16
  - **Blocked By**: T1, T6, T8

  **References**:

  **Pattern References**:
  - `bridge/cmd/bridge/setup_secretary.go:100-140` — Current registry wiring showing how bridgeLocal handlers are registered
  - `bridge/pkg/browser/handler.go:35-87` — Existing Handler(client) pattern that BrokerHandler must mirror

  **API/Type References**:
  - `bridge/pkg/browser/broker.go` — BrowserBroker interface (T6)
  - `bridge/pkg/browser/jetski_broker.go` — JetskiBroker implementation (T8)

  **WHY Each Reference Matters**:
  - `setup_secretary.go:122` is where the choice between Jetski and legacy happens
  - `handler.go:35-87` shows the dispatch pattern — BrokerHandler must produce identical behavior

  **Acceptance Criteria**:

  - [ ] `ARMORCLAW_BROWSER_BACKEND=jetski` activates JetskiBroker
  - [ ] `ARMORCLAW_BROWSER_BACKEND=legacy` uses existing Client
  - [ ] Fallback logs warning when falling back to legacy
  - [ ] `go build ./cmd/bridge/...` succeeds
  - [ ] Existing e2e_fullpath_test.go passes with `ARMORCLAW_BROWSER_BACKEND=legacy`

  **QA Scenarios:**

  ```
  Scenario: Jetski backend selected via config
    Tool: Bash
    Preconditions: Bridge binary built
    Steps:
      1. Set ARMORCLAW_BROWSER_BACKEND=jetski
      2. Start bridge
      3. Check logs for "browser backend=jetski"
    Expected Result: Logs show Jetski broker initialized
    Failure Indicators: Logs show "browser backend=legacy" or panic
    Evidence: .sisyphus/evidence/browser-automation/task-9-jetski-backend.txt

  Scenario: Legacy fallback works when Jetski unreachable
    Tool: Bash
    Preconditions: Bridge built, Jetski NOT running, legacy browser-service available
    Steps:
      1. Set ARMORCLAW_BROWSER_BACKEND=jetski ARMORCLAW_BROWSER_FALLBACK=true
      2. Start bridge
      3. Attempt browser operation
      4. Check logs for "falling back to legacy"
    Expected Result: Operation falls back to legacy with logged warning
    Failure Indicators: Operation fails without fallback, no warning log
    Evidence: .sisyphus/evidence/browser-automation/task-9-fallback.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): register JetskiBroker with backend feature flag and fallback`
  - Files: `bridge/cmd/bridge/setup_secretary.go`, `bridge/pkg/browser/broker_handler.go`
  - Pre-commit: `cd bridge && go build ./cmd/bridge/...`

- [x] 10a. Phase 0 Exit Criteria Harness Test

  **What to do**:
  - Create `tests/test-browser-broker.sh` following the existing harness pattern:
    - Source `tests/lib/load_env.sh`, `tests/lib/common_output.sh`, `tests/lib/assert_json.sh`
    - Tier B (graceful skip when Jetski unavailable)
  - Phase 0 test scenarios ONLY (capabilities that exist after Wave 2):
    - **BB0**: Prerequisites (check Jetski reachable on 9223, Bridge reachable)
    - **BB1**: Health check (`GET /rpc/health` returns ok)
    - **BB2**: Session lifecycle (create → status → close)
    - **BB3**: Navigate through Bridge RPC (`browser.navigate` → verify success via `browser.status`)
    - **BB4**: Backend selection (`ARMORCLAW_BROWSER_BACKEND=jetski` activates broker, `legacy` activates client)
    - **BB5**: Fallback path (Jetski unreachable → legacy fallback when flag enabled, logged WARNING)
    - **BB6**: Latency gate — average `browser.navigate` latency < 3 seconds over 20 consecutive calls (measure round-trip from RPC call to success response)
    - **BB7**: Restart resilience — Bridge ↔ Jetski connection survives 5 consecutive Bridge restarts without manual intervention (each restart followed by a successful `browser.navigate`)
  - Evidence dir: `.sisyphus/evidence/browser-automation/`
  - Follow existing patterns from `test-jetski-sidecar.sh` (J0-J5 scenarios)

  **Must NOT do**:
  - Do NOT test fill/click/extract/screenshot — those are Phase 1 (T10b)
  - Do NOT modify existing test scripts

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — Phase 0 validation gate
  - **Parallel Group**: Wave 2 (with T8, T9, T16)
  - **Blocks**: All Wave 3+ tasks
  - **Blocked By**: T4, T5, T8, T9, T16

  **References**:

  **Pattern References**:
  - `tests/test-jetski-sidecar.sh` — Exact pattern: J0-J5 scenarios, Tier B skip, load_env + common_output + assert_json
  - `tests/lib/assert_json.sh:69-105` — `assert_rpc_success()` and `assert_rpc_error()` for JSON-RPC validation

  **API/Type References**:
  - `tests/lib/load_env.sh` — VPS_IP, VPS_USER, BRIDGE_PORT env vars
  - `tests/lib/common_output.sh` — log_pass/fail/skip/harness_summary pattern

  **WHY Each Reference Matters**:
  - `test-jetski-sidecar.sh` is the closest existing test — copy its structure exactly
  - `assert_rpc_success()` validates JSON-RPC responses — use for all Bridge RPC calls

  **Acceptance Criteria**:

  - [ ] `tests/test-browser-broker.sh` exists and follows harness conventions
  - [ ] Emits PASS/FAIL/SKIP per scenario
  - [ ] Evidence saved to `.sisyphus/evidence/browser-automation/`
  - [ ] Gracefully skips when Jetski or Bridge unreachable
  - [ ] `bash tests/test-browser-broker.sh` runs without error (PASS or SKIP)
  - [ ] BB6: Average navigate latency < 3s over 20 calls (evidence includes timing data)
  - [ ] BB7: Bridge restarts 5× with successful navigate after each (evidence includes restart count)

  **QA Scenarios:**

  ```
  Scenario: Phase 0 harness runs with Jetski available
    Tool: Bash
    Preconditions: Jetski + Bridge running
    Steps:
      1. bash tests/test-browser-broker.sh
       2. Check BB0-BB7 PASS
       3. Review evidence files including latency timing and restart resilience log
    Expected Result: BB0-BB7 PASS, evidence files present (including latency report and restart log)
    Failure Indicators: Any FAIL, missing evidence
    Evidence: .sisyphus/evidence/browser-automation/task-10a-phase0-harness.txt

  Scenario: Phase 0 harness gracefully skips when Jetski unavailable
    Tool: Bash
    Preconditions: Jetski NOT running
    Steps:
      1. bash tests/test-browser-broker.sh
      2. Check for SKIP output
    Expected Result: BB0 SKIP, all subsequent SKIP, exit code 0
    Failure Indicators: FAIL (not SKIP), non-zero exit code
    Evidence: .sisyphus/evidence/browser-automation/task-10a-skip.txt
  ```

  **Commit**: YES
  - Message: `test(harness): add Phase 0 exit criteria with latency and restart gates`
  - Files: `tests/test-browser-broker.sh`
  - Pre-commit: `bash -n tests/test-browser-broker.sh` (syntax check)

- [x] 10b. Phase 1 Exit Criteria Harness Test

  **What to do**:
  - Add Phase 1 scenarios to `tests/test-browser-broker.sh` (extend the file created in T10a):
    - **BB8**: Fill through Bridge RPC (`browser.fill` with non-sensitive value → verify success)
    - **BB9**: Click through Bridge RPC (`browser.click` → verify success)
    - **BB10**: Extract returns structured data
    - **BB11**: Screenshot returns image bytes (verify PNG header)
    - **BB12**: Sensitive fill triggers approval check (PII path active when approval enabled → verify `AWAITING_PII` status)
    - **BB13**: Full workflow (navigate → fill → click → extract → screenshot end-to-end)
  - These scenarios validate that all semantic browser methods work through the broker
  - Evidence dir: `.sisyphus/evidence/browser-automation/`

  **Must NOT do**:
  - Do NOT add chart/recording tests (Phase 2)
  - Do NOT modify existing test scripts

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — Phase 1 validation gate
  - **Parallel Group**: After Wave 3 completes
  - **Blocks**: All Wave 4+ tasks
  - **Blocked By**: T11, T12, T13, T14 (all semantic methods must be implemented)

  **References**:

  **Pattern References**:
  - `tests/test-browser-broker.sh` — File created in T10a to extend
  - `tests/test-secretary-workflow-core.sh` — Shows multi-step workflow test pattern (W3)

  **API/Type References**:
  - `tests/lib/assert_json.sh` — assert_rpc_success for validating responses

  **WHY Each Reference Matters**:
  - T10a file is the base — extend with Phase 1 scenarios
  - `test-secretary-workflow-core.sh:W3` shows multi-step test pattern for BB13 end-to-end

  **Acceptance Criteria**:

  - [ ] BB8-BB13 scenarios added to test-browser-broker.sh
  - [ ] Emits PASS/FAIL/SKIP per scenario
  - [ ] Evidence saved to `.sisyphus/evidence/browser-automation/`
  - [ ] `bash tests/test-browser-broker.sh` runs BB0-BB13 (PASS or SKIP)

  **QA Scenarios:**

  ```
  Scenario: Full Phase 1 harness runs with all methods working
    Tool: Bash
    Preconditions: Jetski + Bridge running, broker configured
    Steps:
      1. bash tests/test-browser-broker.sh
      2. Verify BB0-BB13 all PASS
    Expected Result: All scenarios PASS
    Failure Indicators: BB8+ FAIL (semantic methods not working)
    Evidence: .sisyphus/evidence/browser-automation/task-10b-phase1-harness.txt

  Scenario: Sensitive fill triggers AWAITING_PII status
    Tool: Bash
    Preconditions: Jetski running with approval enabled
    Steps:
      1. Send browser.fill with Sensitive=true
      2. Verify response contains AWAITING_PII or approval_required status
    Expected Result: Status indicates approval needed
    Failure Indicators: Fill completes without approval
    Evidence: .sisyphus/evidence/browser-automation/task-10b-sensitive-fill.txt
  ```

  **Commit**: YES
  - Message: `test(harness): add Phase 1 exit criteria (fill, click, extract, screenshot, sensitive fill)`
  - Files: `tests/test-browser-broker.sh`
  - Pre-commit: `bash -n tests/test-browser-broker.sh`

- [x] 11. JetskiBroker.Fill with Sensitive Flag and PII Handling

  **What to do**:
  - In `jetski_broker.go`, implement `Fill(ctx, jobID, FillRequest)`:
    - When `FillRequest.Sensitive=false`: Send CDP `Input.insertText` directly with the literal value
    - When `FillRequest.Sensitive=true`: The value is a placeholder/PII reference — resolve through Bridge's PII approval flow before filling
    - For sensitive fills: Send approval request via Bridge RPC `pii.request`, wait for approval, receive actual value, then CDP `Input.insertText` with approved value
    - Use the Jetski approval client (wired in T3) for the approval check
  - Ensure fill produces the same `BrowserResult` shape as `handler.go:fillAction()` (which calls `client.Fill()`)
  - Add human-like typing delay when `BrowserStealthConfig.HumanLikeTyping=true` (send characters one at a time with random delays)

  **Must NOT do**:
  - Do NOT log the actual fill value when Sensitive=true
  - Do NOT bypass the approval flow for sensitive values

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T12-T16, all depend only on T8)
  - **Parallel Group**: Wave 3
  - **Blocks**: T23
  - **Blocked By**: T8

  **References**:

  **Pattern References**:
  - `bridge/pkg/browser/handler.go:97-107` — Existing fillAction() showing how fill dispatches through client
  - `bridge/pkg/browser/client.go:285-295` — Client.Fill() showing ServiceFillCommand request type

  **API/Type References**:
  - `bridge/pkg/browser/client.go:88-134` — ServiceFillField with ValueRef for PII references
  - `bridge/pkg/browser/broker_types.go` — FillRequest with Selector, Value, Sensitive fields (T6)
  - `jetski/internal/cdp/proxy.go:150-181` — `needsApproval()` checking for `Input.insertText` as PII trigger

  **WHY Each Reference Matters**:
  - `handler.go:97-107` shows the existing fill flow — broker fill must produce identical results
  - `client.go:88-134` shows ValueRef pattern — FillRequest.Sensitive=true must map to this PII path
  - `proxy.go:150-181` shows Jetski already detects PII in Input.insertText — the broker must respect this

  **Acceptance Criteria**:

  - [ ] `Fill()` with Sensitive=false fills literal value via CDP
  - [ ] `Fill()` with Sensitive=true routes through approval
  - [ ] Sensitive values never appear in logs
  - [ ] `go test ./pkg/browser/...` passes

  **QA Scenarios:**

  ```
  Scenario: Non-sensitive fill works directly
    Tool: Bash
    Preconditions: Jetski + Lightpanda running, session created
    Steps:
      1. Navigate to form page
      2. Call Fill(jobID, FillRequest{Selector:"#name", Value:"John", Sensitive:false})
      3. Extract the field value to verify
    Expected Result: Field contains "John", no approval triggered
    Failure Indicators: Approval requested for non-sensitive fill, field empty
    Evidence: .sisyphus/evidence/browser-automation/task-11-fill-nonsensitive.txt

  Scenario: Sensitive fill triggers approval
    Tool: Bash
    Preconditions: Jetski running with approval enabled, Bridge PII handler active
    Steps:
      1. Navigate to form page
      2. Call Fill(jobID, FillRequest{Selector:"#ssn", Value:"{{ssn}}", Sensitive:true})
      3. Verify approval request sent to Bridge
      4. Verify value NOT filled until approval received
    Expected Result: Approval request logged, fill blocks until approved, no raw PII in logs
    Failure Indicators: Value filled without approval, raw PII in logs
    Evidence: .sisyphus/evidence/browser-automation/task-11-fill-sensitive.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): implement JetskiBroker.Fill with sensitive flag and PII handling`
  - Files: `bridge/pkg/browser/jetski_broker.go`
  - Pre-commit: `cd bridge && go build ./pkg/browser/...`

- [x] 12. JetskiBroker.Click + WaitForElement

  **What to do**:
  - Implement `Click(ctx, jobID, selector)`:
    - Send CDP `Runtime.evaluate` with JS that finds element by selector and calls `.click()`
    - This mirrors what Jetski's translator.go already does (mouse event → Runtime.evaluate)
    - Or use the higher-level approach: send mouse coordinates via CDP `Input.dispatchMouseEvent` after getting element position via `DOM.getBoxModel`
  - Implement `WaitForElement(ctx, jobID, selector, timeout)`:
    - Poll using CDP `DOM.querySelector` at intervals until element found or timeout
    - Use configurable polling interval (default 500ms)
  - Implement `WaitForCaptcha(ctx, jobID, timeout)` and `WaitFor2FA(ctx, jobID, timeout)`:
    - Wait for specific DOM conditions indicating captcha resolution or 2FA field appearance
    - These are specialized waits that check for common captcha/2FA selectors

  **Must NOT do**:
  - Do NOT implement captcha solving — just waiting for user resolution
  - Do NOT add self-healing selector logic (Phase 4)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T11, T13-T16)
  - **Blocks**: None directly
  - **Blocked By**: T8

  **References**:

  **Pattern References**:
  - `jetski/internal/cdp/translator.go:22-50` — Existing translateMouseEvent showing how clicks are converted to Runtime.evaluate
  - `jetski/internal/cdp/translator.go:52-90` — translateMouseClick showing selector generation and JS execution

  **API/Type References**:
  - `bridge/pkg/browser/client.go:296-306` — Client.Click() and ServiceClickCommand
  - `bridge/pkg/browser/client.go:307-317` — Client.Wait() and ServiceWaitCommand
  - `bridge/pkg/browser/broker_types.go` — WaitForElement parameters (T6)

  **WHY Each Reference Matters**:
  - `translator.go:22-50` shows Jetski ALREADY translates mouse events to JS clicks — broker can use same approach or bypass translator
  - `client.go:296-317` shows existing Click/Wait request shapes — broker must produce compatible results

  **Acceptance Criteria**:

  - [ ] Click sends CDP command that clicks the element
  - [ ] WaitForElement polls until found or timeout
  - [ ] WaitForCaptcha and WaitFor2FA wait for specific DOM states
  - [ ] `go build ./pkg/browser/...` succeeds

  **QA Scenarios:**

  ```
  Scenario: Click submits a button
    Tool: Bash
    Preconditions: Jetski running, page with button loaded
    Steps:
      1. Navigate to page with submit button
      2. Call Click(jobID, "#submit")
      3. Verify navigation or DOM change occurred
    Expected Result: Button clicked, page response observed
    Failure Indicators: No DOM change, CDP error
    Evidence: .sisyphus/evidence/browser-automation/task-12-click.txt

  Scenario: WaitForElement times out correctly
    Tool: Bash
    Preconditions: Jetski running, page without target element
    Steps:
      1. Navigate to page
      2. Call WaitForElement(jobID, "#nonexistent", 3*time.Second)
      3. Verify timeout error returned after ~3s
    Expected Result: Error returned with timeout message after ~3 seconds
    Failure Indicators: Returns immediately, or hangs forever
    Evidence: .sisyphus/evidence/browser-automation/task-12-wait-timeout.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): implement JetskiBroker.Click and WaitFor methods`
  - Files: `bridge/pkg/browser/jetski_broker.go`
  - Pre-commit: `cd bridge && go build ./pkg/browser/...`

- [x] 13. JetskiBroker.Extract + Screenshot

  **What to do**:
  - Implement `Extract(ctx, jobID, spec)`:
    - Send CDP `Runtime.evaluate` with JS that queries selectors and returns values
    - Build JS from ExtractSpec: for each field, `document.querySelector(selector)?.textContent` etc.
    - Parse JS return value into ExtractResult map
  - Implement `Screenshot(ctx, jobID)`:
    - Send CDP `Page.captureScreenshot` with format `"png"`
    - Return base64-decoded bytes (CDP returns base64 string)
  - Ensure both produce same response shapes as existing `client.Extract()` and `client.Screenshot()`

  **Must NOT do**:
  - Do NOT implement full-page screenshots — viewport only for now
  - Do NOT add image processing or comparison

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T11-T12, T14-T16)
  - **Blocks**: None directly
  - **Blocked By**: T8

  **References**:

  **Pattern References**:
  - `bridge/pkg/browser/client.go:318-327` — Client.Extract() with ServiceExtractCommand
  - `bridge/pkg/browser/client.go:328-337` — Client.Screenshot() with ServiceScreenshotCommand

  **API/Type References**:
  - `bridge/pkg/browser/broker_types.go` — ExtractSpec and ExtractResult types (T6)
  - `jetski/internal/cdp/proxy.go:35-47` — CDPMessage format for sending CDP commands

  **WHY Each Reference Matters**:
  - `client.go:318-337` shows existing extract/screenshot contracts — broker must match
  - `proxy.go:35-47` shows CDP wire format for sending Page.captureScreenshot

  **Acceptance Criteria**:

  - [ ] Extract returns map of selector→value
  - [ ] Screenshot returns PNG bytes
  - [ ] `go build ./pkg/browser/...` succeeds

  **QA Scenarios:**

  ```
  Scenario: Extract returns page data
    Tool: Bash
    Preconditions: Jetski running, example.com loaded
    Steps:
      1. Navigate to example.com
      2. Call Extract(jobID, ExtractSpec{Fields: [{"selector":"h1","name":"title"}]})
      3. Verify result contains {"title":"Example Domain"}
    Expected Result: ExtractResult with title field matching page heading
    Failure Indicators: Empty result, wrong value, CDP error
    Evidence: .sisyphus/evidence/browser-automation/task-13-extract.txt

  Scenario: Screenshot returns valid PNG
    Tool: Bash
    Preconditions: Jetski running, page loaded
    Steps:
      1. Navigate to page
      2. Call Screenshot(jobID)
      3. Check first 8 bytes match PNG magic number (89 50 4E 47)
    Expected Result: Non-empty byte slice starting with PNG header
    Failure Indicators: Empty bytes, wrong magic number, CDP error
    Evidence: .sisyphus/evidence/browser-automation/task-13-screenshot.png
  ```

  **Commit**: YES
  - Message: `feat(bridge): implement JetskiBroker.Extract and Screenshot`
  - Files: `bridge/pkg/browser/jetski_broker.go`
  - Pre-commit: `cd bridge && go build ./pkg/browser/...`

- [x] 14. JetskiBroker Job Lifecycle (Complete, Fail, List, Cancel)

  **What to do**:
  - Implement `Complete(ctx, jobID)`:
    - Close the CDP WebSocket connection
    - Call Jetski RPC `POST /rpc/session/close` with session ID
    - Remove job from active jobs map
    - Emit completion event
  - Implement `Fail(ctx, jobID, err)`:
    - Same cleanup as Complete but with error status
    - Log failure reason
  - Implement `List(ctx)`:
    - Return summary of all active jobs from internal map
  - Implement `Cancel(ctx, jobID)`:
    - Cancel the context associated with the job
    - Close WebSocket
    - Close Jetski session
    - Remove from active map
  - Add internal `activeJobs sync.Map` to JetskiBroker struct

  **Must NOT do**:
  - Do NOT persist job state (that's the queue's job)
  - Do NOT add job retry logic (Phase 4)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T11-T13, T15-T16)
  - **Blocks**: T23, T27
  - **Blocked By**: T8

  **References**:

  **Pattern References**:
  - `bridge/pkg/queue/browser_queue.go:21-110` — Existing BrowserJob, BrowserQueue, JobProcessor interface showing the lifecycle model
  - `bridge/pkg/queue/browser_queue.go:542-610` — processJob() showing how jobs transition through states

  **API/Type References**:
  - `jetski/internal/rpc/rpc.go:33-34` — `/rpc/session/create` and `/rpc/session/close` endpoints
  - `bridge/pkg/browser/broker_types.go` — JobStatus, JobSummary types (T6)

  **WHY Each Reference Matters**:
  - `browser_queue.go:21-110` shows the existing lifecycle model (pending→running→completed/failed/cancelled) — broker must align
  - `rpc.go:33-34` shows Jetski session endpoints — broker maps jobs to these sessions

  **Acceptance Criteria**:

  - [ ] Complete closes session and WebSocket
  - [ ] Fail closes with error status
  - [ ] List returns active jobs
  - [ ] Cancel terminates running job
  - [ ] No goroutine leaks after job completion

  **QA Scenarios:**

  ```
  Scenario: Full job lifecycle: create → navigate → complete
    Tool: Bash
    Preconditions: Jetski running
    Steps:
      1. StartJob → get jobID
      2. Navigate to example.com
      3. List → verify 1 active job
      4. Complete → verify session closed
      5. List → verify 0 active jobs
    Expected Result: Clean lifecycle, no leaked connections
    Failure Indicators: Session not closed, job still in active map, goroutine leak
    Evidence: .sisyphus/evidence/browser-automation/task-14-lifecycle.txt

  Scenario: Cancel terminates running navigation
    Tool: Bash
    Preconditions: Jetski running
    Steps:
      1. StartJob
      2. Start Navigate to slow-loading URL
      3. Immediately Cancel
      4. Verify no hanging goroutines
    Expected Result: Cancel returns immediately, no leaked resources
    Failure Indicators: Cancel blocks, goroutine still running
    Evidence: .sisyphus/evidence/browser-automation/task-14-cancel.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): implement JetskiBroker job lifecycle methods`
  - Files: `bridge/pkg/browser/jetski_broker.go`
  - Pre-commit: `cd bridge && go build ./pkg/browser/...`

- [x] 15. Legacy Fallback Mechanism (Temporary Escape Hatch)

  **What to do**:
  - In `broker_handler.go`, add fallback logic:
    - When a JetskiBroker call fails with connection error, check `ARMORCLAW_BROWSER_FALLBACK`
    - If fallback enabled, create a one-time legacy Client pointing to `cfg.Browser.LegacyURL`
    - Retry the failed operation using the legacy Client
    - Log the fallback with WARNING level including: original error, operation, jobID
    - Emit a metric/counter for fallback usage
  - Add a `FallbackLog` struct that records every fallback event (timestamp, operation, error, whether it succeeded)
  - This is explicitly TEMPORARY — add a code comment marking it for removal after Phase 4 stabilization
  - Add `ARMORCLAW_BROWSER_FALLBACK_MAX_RETRIES` (default: 3) to prevent infinite fallback loops

  **Must NOT do**:
  - Do NOT make fallback the default path — Jetski is always primary
  - Do NOT cache the legacy Client between operations
  - Do NOT add circuit breaker (Phase 4)
  - Do NOT remove this escape hatch until Phase 4 confidence scoring proves Jetski is reliable

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T11-T14, T16)
  - **Blocks**: T29
  - **Blocked By**: T9

  **References**:

  **Pattern References**:
  - `bridge/pkg/browser/handler.go:35-87` — Existing Handler(client) showing the legacy dispatch pattern that fallback must use
  - `bridge/pkg/browser/client.go:242-338` — Legacy Client methods that fallback calls

  **API/Type References**:
  - `bridge/pkg/config/config.go:407-428` — BrowserConfig with ServiceURL for legacy backend

  **WHY Each Reference Matters**:
  - `handler.go` shows the legacy path that fallback recreates on Jetski failure
  - `client.go` shows all 11 REST methods available for fallback

  **Acceptance Criteria**:

  - [ ] Fallback triggers on Jetski connection failure when flag enabled
  - [ ] Every fallback logged with WARNING
  - [ ] Fallback counter available for monitoring
  - [ ] Code comment marks feature as TEMPORARY
  - [ ] `go build ./...` succeeds

  **QA Scenarios:**

  ```
  Scenario: Fallback activates when Jetski down
    Tool: Bash
    Preconditions: Jetski NOT running, legacy browser-service running
    Steps:
      1. Set ARMORCLAW_BROWSER_BACKEND=jetski ARMORCLAW_BROWSER_FALLBACK=true
      2. Attempt Navigate
      3. Check logs for "falling back to legacy" WARNING
      4. Verify navigation succeeded via legacy
    Expected Result: Navigation succeeds, WARNING logged
    Failure Indicators: Navigation fails, no fallback log
    Evidence: .sisyphus/evidence/browser-automation/task-15-fallback.txt

  Scenario: No fallback when flag disabled
    Tool: Bash
    Preconditions: Jetski NOT running
    Steps:
      1. Set ARMORCLAW_BROWSER_BACKEND=jetski ARMORCLAW_BROWSER_FALLBACK=false
      2. Attempt Navigate
      3. Verify error returned (no fallback)
    Expected Result: Error returned, no WARNING log
    Failure Indicators: Fallback happens despite flag=false
    Evidence: .sisyphus/evidence/browser-automation/task-15-no-fallback.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): add temporary legacy fallback escape hatch`
  - Files: `bridge/pkg/browser/broker_handler.go`
  - Pre-commit: `cd bridge && go build ./...`

- [x] 16. Wire RPC Path A Handlers Through Broker

  **What to do**:
  - The JSON-RPC handlers in `bridge/pkg/rpc/browser.go` currently use `BrowserSkill` (a status-emitting stub that never calls browser-service). Wire these to call through the BrowserBroker instead:
  - In `bridge/pkg/rpc/browser.go`, modify each handler (handleBrowserNavigate, handleBrowserFill, etc.) to:
    1. Look up the active BrowserBroker from a broker registry
    2. Call the broker method (Navigate, Fill, etc.)
    3. Return the broker result as the JSON-RPC response
  - Add a `SetBroker(broker BrowserBroker)` method to the RPC browser handler struct so the broker is injectable
  - Keep `BrowserSkill` as a fallback for status-only mode (when no broker is configured)
  - The 11 RPC methods registered in `server.go:848-861` should all route through the broker when available

  **Must NOT do**:
  - Do NOT remove BrowserSkill entirely — it's used for status tracking
  - Do NOT break existing RPC method signatures
  - Do NOT change the JSON-RPC response format

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — depends on T9 (sequential within Wave 2)
  - **Parallel Group**: Wave 2 (moved from Wave 3 so T10a can test RPC handlers)
  - **Blocks**: T10a
  - **Blocked By**: T9

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/browser.go:60-122` — Current RPC handlers using BrowserSkill stub (the Path A code)
  - `bridge/pkg/browser/browser.go:61` — BrowserSkill.Navigate() showing it only emits StatusEvents, no HTTP calls

  **API/Type References**:
  - `bridge/pkg/rpc/server.go:848-861` — Method registry mapping browser.* method names to handlers
  - `bridge/pkg/browser/broker.go` — BrowserBroker interface (T6)

  **WHY Each Reference Matters**:
  - `browser.go:60-122` shows all 11 RPC handlers that are currently stubs — this is the wiring target
  - `browser/browser.go:61` proves BrowserSkill doesn't call any backend — the broker must replace this behavior

  **Acceptance Criteria**:

  - [ ] RPC `browser.navigate` calls broker.Navigate() when broker configured
  - [ ] RPC `browser.fill` calls broker.Fill() when broker configured
  - [ ] RPC falls back to BrowserSkill when no broker configured
  - [ ] All 11 browser RPC methods route through broker
  - [ ] `go test ./pkg/rpc/...` passes

  **QA Scenarios:**

  ```
  Scenario: RPC browser.navigate calls through broker
    Tool: Bash
    Preconditions: Bridge running with JetskiBroker configured
    Steps:
      1. Send JSON-RPC browser.navigate with url="https://example.com"
      2. Verify broker.Navigate is called (check logs)
      3. Verify JSON-RPC response contains success
    Expected Result: Navigation succeeds, broker called, JSON-RPC response matches
    Failure Indicators: BrowserSkill stub used instead, RPC returns stub response
    Evidence: .sisyphus/evidence/browser-automation/task-16-rpc-wiring.txt

  Scenario: RPC methods work without broker (backward compat)
    Tool: Bash
    Preconditions: Bridge running without broker configured
    Steps:
      1. Send JSON-RPC browser.navigate
      2. Verify BrowserSkill handles it (status update only)
    Expected Result: Status event emitted, no error
    Failure Indicators: Panic or error when broker is nil
    Evidence: .sisyphus/evidence/browser-automation/task-16-rpc-fallback.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): wire RPC browser handlers through BrowserBroker`
  - Files: `bridge/pkg/rpc/browser.go`
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/rpc/...`

- [x] 17. Fix Sonar Session Tracking

  **What to do**:
  - In `jetski/cmd/observer/main.go:86`, the Sonar recorder is initialized with an empty sessionID string `""`:
    ```go
    cdpProxy.SetRecorder(func(method string, params json.RawMessage) {
        sonar.RecordFrame("", method, params) // empty sessionID
    })
    ```
  - This means all sessions' CDP frames are mixed under the same empty key in the Reporter's per-session metrics map
  - Fix: Pass the actual session ID from the RPC session manager. When `handleSessionCreate` (rpc.go:33) creates a new session, propagate the session ID to the CDP proxy's recorder
  - Options: (A) Use a goroutine-safe session context that the recorder closure reads, or (B) store session ID on the CDP proxy and have the proxy pass it to the recorder
  - Recommended: Add a `SetSessionID(id string)` method to the Proxy that updates an atomic string, and have the recorder closure read from it

  **Must NOT do**:
  - Do NOT change the CircularBuffer API
  - Do NOT change the CDPFrame struct

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with T18)
  - **Blocks**: T19
  - **Blocked By**: T3

  **References**:

  **Pattern References**:
  - `jetski/cmd/observer/main.go:84-87` — Current recorder setup with empty sessionID
  - `jetski/internal/sonar/recorder.go:8-18` — RecordFrame function signature accepting sessionID

  **API/Type References**:
  - `jetski/internal/sonar/reporter.go:17-24` — Reporter struct with per-session SelectorMetrics
  - `jetski/internal/rpc/rpc.go:14-28` — Server struct with sessions map for session ID generation

  **WHY Each Reference Matters**:
  - `main.go:86` is where the empty string is passed — this is the fix target
  - `reporter.go:17-24` shows per-session tracking that's broken without proper session IDs

  **Acceptance Criteria**:

  - [ ] Sonar frames recorded with actual session IDs
  - [ ] Reporter's per-session metrics map has distinct entries per session
  - [ ] Wreckage reports include correct session IDs
  - [ ] `go build ./...` succeeds in jetski/

  **QA Scenarios:**

  ```
  Scenario: Two sessions produce separate frame histories
    Tool: Bash
    Preconditions: Jetski running
    Steps:
      1. POST /rpc/session/create → session-1
      2. Navigate via CDP
      3. POST /rpc/session/create → session-2
      4. Navigate via CDP
      5. Check wreckage reports have correct session IDs
    Expected Result: session-1 frames tagged with session-1, session-2 with session-2
    Failure Indicators: All frames tagged with "" (empty)
    Evidence: .sisyphus/evidence/browser-automation/task-17-session-tracking.txt
  ```

  **Commit**: YES
  - Message: `fix(jetski): wire session ID into Sonar recorder`
  - Files: `jetski/cmd/observer/main.go`, `jetski/internal/cdp/proxy.go`
  - Pre-commit: `cd jetski && go build ./...`

- [x] 18. Go NavChart Type Definitions

  **What to do**:
  - Create `bridge/pkg/browser/chart_types.go` defining Go types that match the Chartmaker NavChart JSON schema:
    ```go
    type NavChart struct {
        Version      int                    `json:"version"`
        TargetDomain string                 `json:"target_domain"`
        Metadata     ChartMetadata          `json:"metadata"`
        ActionMap    map[string]ChartAction  `json:"action_map"`
    }
    type ChartMetadata struct {
        GeneratedBy  string  `json:"generated_by"`
        Timestamp    int64   `json:"timestamp"`
        SessionID    string  `json:"session_id,omitempty"`
        TotalActions int     `json:"total_actions,omitempty"`
    }
    type ChartAction struct {
        ActionType    string          `json:"action_type"` // click, input, navigate, wait, assert
        Selector      *ChartSelector  `json:"selector,omitempty"`
        Value         string          `json:"value,omitempty"`
        URL           string          `json:"url,omitempty"`
        FrameRouting  *FrameRouting   `json:"frame_routing,omitempty"`
        PostActionWait *WaitCondition `json:"post_action_wait,omitempty"`
    }
    type ChartSelector struct {
        PrimaryCSS     string `json:"primary_css"`
        SecondaryXPath string `json:"secondary_xpath,omitempty"`
        FallbackJS     string `json:"fallback_js,omitempty"`
    }
    type FrameRouting struct {
        Selector string `json:"selector,omitempty"`
        Name     string `json:"name,omitempty"`
        Origin   string `json:"origin,omitempty"`
    }
    type WaitCondition struct {
        Type     string `json:"type"` // waitForVisible, waitForHidden, waitForTimeout, waitForSelector
        Selector string `json:"selector,omitempty"`
        Timeout  int    `json:"timeout,omitempty"` // milliseconds
    }
    ```
  - Create `bridge/pkg/browser/chart_types_test.go` with JSON marshal/unmarshal tests verifying compatibility with the chartmaker schema
  - Include an `ActionType` enum with constants: `ActionClick`, `ActionInput`, `ActionNavigate`, `ActionWait`, `ActionAssert`

  **Must NOT do**:
  - Do NOT import or depend on Chartmaker TypeScript code
  - Do NOT add Lighthouse-specific fields (blessed, signature, downloads)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with T17)
  - **Blocks**: T19, T20, T22
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `jetski/jetski-chartmaker/src/core/recorder/state-compiler.ts:3-57` — TypeScript NavChart, CompiledAction, ChartSelector types that Go types must match
  - `jetski/jetski-chartmaker/schemas/nav-chart.json:1-137` — JSON Schema defining exact field names, types, and enums

  **API/Type References**:
  - `jetski/lighthouse/charts/stripe.com.acsb.json` — Example NavChart file showing real action_map structure with frame_routing

  **Test References**:
  - `jetski/jetski-chartmaker/src/core/validator/schema.ts` — Ajv-based validation showing what valid charts look like

  **WHY Each Reference Matters**:
  - `state-compiler.ts:3-57` shows the canonical NavChart shape — Go types must serialize to identical JSON
  - `nav-chart.json` is the authoritative schema — Go types must satisfy this schema exactly
  - `stripe.com.acsb.json` is a real chart file — test unmarshaling against this

  **Acceptance Criteria**:

  - [ ] Go types serialize to JSON matching chartmaker schema
  - [ ] `stripe.com.acsb.json` can be unmarshaled into Go NavChart struct
  - [ ] `go test ./pkg/browser/...` with chart type tests passes
  - [ ] No dependency on TypeScript or Node.js

  **QA Scenarios:**

  ```
  Scenario: NavChart JSON roundtrip matches schema
    Tool: Bash (go test)
    Preconditions: chart_types.go and chart_types_test.go written
    Steps:
      1. Unmarshal stripe.com.acsb.json into NavChart
      2. Re-marshal to JSON
      3. Compare key fields (version, target_domain, action_map keys)
    Expected Result: Roundtrip produces equivalent JSON
    Failure Indicators: Missing fields, wrong types, empty action_map
    Evidence: .sisyphus/evidence/browser-automation/task-18-chart-types.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): add Go NavChart type definitions matching chartmaker schema`
  - Files: `bridge/pkg/browser/chart_types.go`, `bridge/pkg/browser/chart_types_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/browser/...`

- [x] 19. Normalization Pipeline (CDP Frames → Semantic Steps)

  **What to do**:
  - Create `bridge/pkg/browser/normalizer.go` implementing the normalization pipeline:
    1. **Filter**: Remove low-value CDP noise (Network.*, Console.*, CSS.*, Log.*, DOM.childNode* unless related to navigation)
    2. **Group**: Group consecutive CDP frames into semantic steps (navigate → fill → click sequences)
    3. **Detect PII**: Scan `Input.insertText` params for PII patterns (SSN, CC, email, password) using the same regex patterns as `jetski/internal/security/pii_scanner.go`
    4. **Replace**: Replace detected PII values with placeholders (e.g., `{{ssn}}`, `{{credit_card}}`, `{{email}}`)
    5. **Extract selectors**: For each interaction, extract primary CSS selector + XPath fallback + JS fallback
    6. **Attach metadata**: domain, session_id, confidence (initial 0.5), requires_approval (true if any PII detected), outcome (success/failure)
  - Create `Normalize(frames []sonar.CDPFrame, sessionID string) (*NavChart, error)` function
  - Create `bridge/pkg/browser/normalizer_test.go` with test cases:
    - Empty frames → empty chart
    - Navigate-only → single navigate action
    - Navigate+Fill+Click → three actions with PII placeholder
    - PII detection and replacement validation

  **Must NOT do**:
  - Do NOT import Chartmaker TypeScript code
  - Do NOT add YARA scanning (that's future defense-in-depth, not Phase 2)
  - Do NOT persist charts yet (T20)

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — depends on T17 (session tracking) and T18 (types)
  - **Parallel Group**: Wave 4
  - **Blocks**: T26
  - **Blocked By**: T17, T18

  **References**:

  **Pattern References**:
  - `jetski/internal/security/pii_scanner.go:41-51` — PII detection patterns (SSN, CREDIT_CARD, EMAIL, PASSWORD)
  - `jetski/internal/security/pii_scanner.go:165-176` — MaskPII function showing replacement patterns
  - `jetski/jetski-chartmaker/src/core/recorder/state-compiler.ts:65-146` — TypeScript compileEvents/compileSingleAction showing how CDP events map to chart actions

  **API/Type References**:
  - `jetski/internal/sonar/buffer.go:10-15` — CDPFrame struct (Timestamp, Method, Params, SessionID)
  - `bridge/pkg/browser/chart_types.go` — NavChart, ChartAction, ChartSelector types (T18)

  **WHY Each Reference Matters**:
  - `pii_scanner.go:41-51` shows the 4 PII regex patterns — normalizer must use same detection
  - `state-compiler.ts:65-146` shows the TypeScript reference implementation — Go normalizer should produce equivalent output
  - `buffer.go:10-15` shows CDPFrame input format — normalizer consumes these frames

  **Acceptance Criteria**:

  - [ ] Normalize() produces NavChart from CDP frames
  - [ ] PII values replaced with placeholders
  - [ ] Non-PII values preserved as-is
  - [ ] Selectors extracted for click/fill actions
  - [ ] `go test ./pkg/browser/...` passes with normalizer tests

  **QA Scenarios:**

  ```
  Scenario: Login flow normalized with PII replacement
    Tool: Bash (go test)
    Preconditions: Test CDP frames simulating login (navigate + fill email + fill password + click)
    Steps:
      1. Create test frames: Page.navigate → Input.insertText(email) → Input.insertText(password) → Input.dispatchMouseEvent
      2. Call Normalize(frames, "session-1")
      3. Verify chart has 4 actions: navigate, input(email→{{email}}), input(password→{{password}}), click
      4. Verify requires_approval=true
    Expected Result: NavChart with 4 actions, PII replaced, approval flagged
    Failure Indicators: Raw PII in chart, missing actions, wrong order
    Evidence: .sisyphus/evidence/browser-automation/task-19-normalization.txt

  Scenario: Non-PII form fill preserves values
    Tool: Bash (go test)
    Preconditions: Test frames with non-sensitive fill
    Steps:
      1. Create frames: navigate + fill("John") + click
      2. Call Normalize()
      3. Verify value "John" preserved (not replaced)
    Expected Result: Chart contains literal value "John"
    Failure Indicators: "John" replaced with placeholder
    Evidence: .sisyphus/evidence/browser-automation/task-19-no-pii.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): add CDP-to-NavChart normalization pipeline`
  - Files: `bridge/pkg/browser/normalizer.go`, `bridge/pkg/browser/normalizer_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/browser/...`

- [x] 20. Learned Charts Table + Persistence Layer

  **What to do**:
  - Add `learned_charts` table to `bridge/pkg/secretary/store.go` `initSchema()` (after existing `learned_skills` table):
    ```sql
    CREATE TABLE IF NOT EXISTS learned_charts (
        chart_id TEXT PRIMARY KEY,
        domain TEXT NOT NULL,
        title TEXT NOT NULL,
        version INTEGER DEFAULT 1,
        steps TEXT NOT NULL,          -- JSON: action_map serialized
        selectors TEXT NOT NULL,      -- JSON: selector inventory
        placeholders TEXT NOT NULL,   -- JSON: placeholder registry
        requires_approval INTEGER DEFAULT 0,
        confidence REAL DEFAULT 0.5,
        created_from_session TEXT,
        created_at INTEGER NOT NULL,
        last_used_at INTEGER,
        success_count INTEGER DEFAULT 0,
        failure_count INTEGER DEFAULT 0,
        parent_chart_id TEXT,
        CHECK (confidence >= 0.0 AND confidence <= 1.0)
    );
    CREATE INDEX IF NOT EXISTS idx_charts_domain ON learned_charts(domain);
    CREATE INDEX IF NOT EXISTS idx_charts_confidence ON learned_charts(confidence DESC);
    ```
  - Create `bridge/pkg/browser/chart_store.go` — `ChartStore` interface and SQLite implementation:
    ```go
    type ChartStore interface {
        SaveChart(ctx context.Context, chart NavChart, meta ChartMeta) (string, error)
        FindForDomain(ctx context.Context, domain string, limit int) ([]ChartRecord, error)
        RecordOutcome(ctx context.Context, chartID string, success bool) error
        GetChart(ctx context.Context, chartID string) (*ChartRecord, error)
        DeleteChart(ctx context.Context, chartID string) error
    }
    ```
  - `ChartRecord` wraps NavChart + metadata (chartID, domain, confidence, success/failure counts, timestamps)
  - Use the same secretary SQLite database (NOT Lighthouse's separate database)

  **Must NOT do**:
  - Do NOT use Lighthouse's SQLite database or `charts` table
  - Do NOT store raw PII — steps should only contain placeholders for sensitive values
  - Do NOT add chart signing/blessing (that's Lighthouse's concern)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T17, T19)
  - **Parallel Group**: Wave 4
  - **Blocks**: T21, T22, T28
  - **Blocked By**: T18

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/store.go:236-252` — Existing `initSchema()` with `learned_skills` table — add `learned_charts` here
  - `bridge/pkg/skills/learned_store.go` — Existing LearnedStore implementation showing the CRUD pattern to follow

  **API/Type References**:
  - `bridge/pkg/skills/learned_store.go:35-50` — LearnedStore interface methods (Save, FindForTask, RecordOutcome, Delete, ListForAgent) — ChartStore should follow similar pattern
  - `bridge/pkg/browser/chart_types.go` — NavChart struct (T18) that will be stored

  **WHY Each Reference Matters**:
  - `store.go:236-252` is where the schema lives — add new table to same initSchema
  - `learned_store.go` is the reference CRUD implementation — ChartStore follows same patterns

  **Acceptance Criteria**:

  - [ ] `learned_charts` table created in secretary SQLite
  - [ ] ChartStore implements Save, FindForDomain, RecordOutcome, Get, Delete
  - [ ] `go test ./pkg/browser/...` passes with chart store tests
  - [ ] Existing `learned_skills` table unaffected

  **QA Scenarios:**

  ```
  Scenario: Save and retrieve chart roundtrip
    Tool: Bash (go test)
    Preconditions: In-memory SQLite with schema
    Steps:
      1. Create NavChart with 3 actions for example.com
      2. SaveChart → get chartID
      3. FindForDomain("example.com", 10) → verify chart returned
      4. GetChart(chartID) → verify full chart data
    Expected Result: Roundtrip preserves all chart fields
    Failure Indicators: Missing actions, wrong domain, empty selectors
    Evidence: .sisyphus/evidence/browser-automation/task-20-chart-store.txt

  Scenario: RecordOutcome updates confidence
    Tool: Bash (go test)
    Preconditions: Chart saved with confidence 0.5
    Steps:
      1. RecordOutcome(chartID, true) 5 times
      2. GetChart → verify success_count=5, confidence > 0.5
    Expected Result: Confidence increases with successful outcomes
    Failure Indicators: Confidence unchanged, counts wrong
    Evidence: .sisyphus/evidence/browser-automation/task-20-outcomes.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): add learned_charts table and ChartStore persistence`
  - Files: `bridge/pkg/secretary/store.go`, `bridge/pkg/browser/chart_store.go`, `bridge/pkg/browser/chart_store_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/browser/... ./pkg/secretary/...`

- [x] 21. Chart Injection into Learned Skills Path

  **What to do**:
  - Extend `injectLearnedSkills()` (now wired in production from T7) to also inject relevant browser charts alongside learned skills
  - When the step is a browser-execute step (detected by checking if config contains browser intent):
    1. Extract the target domain from the browser URL in the step config
    2. Call `ChartStore.FindForDomain(domain, 3)` to find top charts by confidence
    3. Add charts to the config under a `relevant_charts` key (alongside existing `relevant_skills`)
  - Extend `StepExecutorConfig` to include a `ChartFinder` interface (similar to existing `SkillFinder`)
  - After step execution, if charts were used, call `ChartStore.RecordOutcome(chartID, success)` based on the step result
  - This connects the Phase 2 recording pipeline to the Phase 0 skills injection fix

  **Must NOT do**:
  - Do NOT replace learned skills with charts — they serve different purposes
  - Do NOT inject charts for non-browser steps
  - Do NOT modify the skill extraction strategies

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4
  - **Blocks**: None directly
  - **Blocked By**: T7, T20

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:1195-1231` — `injectLearnedSkills()` showing the exact pattern: find relevant → inject into config JSON
  - `bridge/pkg/secretary/orchestrator_integration.go:1254-1280` — `recordSkillOutcomes()` showing the outcome recording pattern

  **API/Type References**:
  - `bridge/pkg/secretary/orchestrator_integration.go:87-99` — SkillFinder interface that ChartFinder should mirror
  - `bridge/pkg/browser/chart_store.go` — ChartStore interface with FindForDomain (T20)

  **WHY Each Reference Matters**:
  - `orchestrator_integration.go:1195-1231` shows exactly where to add chart injection — extend this method
  - `SkillFinder` interface is the pattern to follow for `ChartFinder`

  **Acceptance Criteria**:

  - [ ] Browser steps receive relevant charts in config
  - [ ] Non-browser steps unaffected
  - [ ] Chart outcomes recorded after execution
  - [ ] `go test ./pkg/secretary/...` passes

  **QA Scenarios:**

  ```
  Scenario: Browser step receives charts for matching domain
    Tool: Bash (go test)
    Preconditions: ChartStore with a chart for example.com
    Steps:
      1. Create step config with browser intent for example.com
      2. Run injectLearnedSkills (now also injects charts)
      3. Verify config contains relevant_charts array
    Expected Result: Config has relevant_charts with example.com chart
    Failure Indicators: No relevant_charts key, or charts for wrong domain
    Evidence: .sisyphus/evidence/browser-automation/task-21-chart-injection.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): inject relevant browser charts into step config`
  - Files: `bridge/pkg/secretary/orchestrator_integration.go`
  - Pre-commit: `cd bridge && go test ./pkg/secretary/...`

- [x] 22. Chart Validator (PII, Placeholder, Domain Policy)

  **What to do**:
  - Create `bridge/pkg/browser/chart_validator.go` with a `ChartValidator` that validates charts before storage and before replay:
  - **Before storage** (normalization output validation):
    - Reject if any step value matches PII patterns (SSN, CC, email, password in raw form)
    - Require valid placeholder format (`{{field_name}}`) for sensitive fields
    - Require at least `primary_css` selector for click/input actions
    - Reject if `target_domain` is empty or malformed
    - Reject if `action_map` is empty
    - Reject if schema version is unsupported
  - **Before replay**:
    - All storage validations PLUS:
    - Verify all placeholders can be resolved from available PII references
    - Verify domain is in allowed list (if policy configured)
    - Flag actions that require approval (`requires_approval=true`)
  - Create `ValidateForStorage(chart NavChart) error` and `ValidateForReplay(chart NavChart, availablePII map[string]string) error`
  - Create `bridge/pkg/browser/chart_validator_test.go` with test cases for each validation rule

  **Must NOT do**:
  - Do NOT add YARA scanning (future defense-in-depth)
  - Do NOT implement the approval flow itself (T23 handles that)
  - Do NOT block valid charts with non-PII literal values

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — foundation for T23-T25
  - **Parallel Group**: Wave 5
  - **Blocks**: T23, T24, T25, T26, T28
  - **Blocked By**: T18, T20

  **References**:

  **Pattern References**:
  - `jetski/internal/security/pii_scanner.go:41-51` — PII detection patterns to use for raw PII rejection
  - `bridge/pkg/skills/extractor.go:23-124` — Existing skill extraction validation showing how to validate learned artifacts

  **API/Type References**:
  - `bridge/pkg/browser/chart_types.go` — NavChart types to validate (T18)
  - `bridge/pkg/browser/chart_store.go` — ChartStore.Save should call ValidateForStorage (T20)

  **WHY Each Reference Matters**:
  - `pii_scanner.go:41-51` has the 4 PII regex patterns — validator must reject charts containing matches
  - `extractor.go` shows the validation pattern for learned artifacts — chart validation follows same approach

  **Acceptance Criteria**:

  - [ ] Validator rejects charts with raw PII in step values
  - [ ] Validator accepts charts with valid placeholders
  - [ ] Validator rejects empty/malformed charts
  - [ ] Replay validation checks placeholder resolvability
  - [ ] `go test ./pkg/browser/...` passes

  **QA Scenarios:**

  ```
  Scenario: Chart with raw SSN rejected
    Tool: Bash (go test)
    Preconditions: Chart with value "123-45-6789" in a step
    Steps:
      1. Create NavChart with SSN in action value
      2. Call ValidateForStorage(chart)
      3. Verify error contains "raw PII detected"
    Expected Result: Validation error with PII detection message
    Failure Indicators: Chart accepted with raw SSN
    Evidence: .sisyphus/evidence/browser-automation/task-22-pii-reject.txt

  Scenario: Chart with valid placeholders accepted
    Tool: Bash (go test)
    Preconditions: Chart with {{ssn}} placeholder
    Steps:
      1. Create NavChart with placeholder value
      2. Call ValidateForStorage(chart)
      3. Verify nil error
    Expected Result: Chart accepted
    Failure Indicators: Placeholder rejected
    Evidence: .sisyphus/evidence/browser-automation/task-22-placeholder-accept.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): add chart validator with PII rejection and policy checks`
  - Files: `bridge/pkg/browser/chart_validator.go`, `bridge/pkg/browser/chart_validator_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/browser/...`

- [x] 23. Replay-Through-Approval Enforcement

  **What to do**:
  - Ensure that when a chart is replayed, ALL approval-gated actions route through the normal Bridge PII approval path — NO shortcuts
  - In `jetski_broker.go`, when replaying chart actions:
    - For each action with `requires_approval=true`, call the approval flow (same as Fill with Sensitive=true)
    - Do NOT pre-fill approved values — each replay requires fresh approval
    - If any approval is denied, abort the chart replay and report failure
  - Add a `ReplayChart(ctx context.Context, jobID JobID, chart NavChart, piiValues map[string]string) error` method to JetskiBroker
  - This method iterates chart actions, resolves placeholders from `piiValues`, and executes each step with proper approval checks
  - Log each replay action with: action index, type, selector, whether approval was needed/granted/denied

  **Must NOT do**:
  - Do NOT cache approvals between chart replays
  - Do NOT skip approval for "known safe" charts — every replay goes through full governance
  - Do NOT auto-approve based on chart confidence score

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T24, T25)
  - **Parallel Group**: Wave 5
  - **Blocks**: T30
  - **Blocked By**: T14, T22

  **References**:

  **Pattern References**:
  - `bridge/pkg/browser/jetski_broker.go` — JetskiBroker.Fill (T11) showing how sensitive fill triggers approval — ReplayChart must use same path

  **API/Type References**:
  - `bridge/pkg/browser/chart_types.go` — ChartAction.RequiresApproval flag (add if not present)
  - `bridge/pkg/browser/chart_validator.go` — ValidateForReplay (T22) showing what validation happens before replay

  **WHY Each Reference Matters**:
  - T11's Fill with Sensitive=true is the approval gate — ReplayChart must call through this same gate
  - ChartValidator (T22) pre-validates before replay — ReplayChart runs after validation passes

  **Acceptance Criteria**:

  - [ ] ReplayChart iterates all chart actions
  - [ ] Approval-gated actions trigger PII approval flow
  - [ ] Denied approval aborts replay
  - [ ] Every replay action logged
  - [ ] `go test ./pkg/browser/...` passes

  **QA Scenarios:**

  ```
  Scenario: Chart replay triggers approval for sensitive fields
    Tool: Bash (go test)
    Preconditions: Chart with 3 actions, one requires approval
    Steps:
      1. Call ReplayChart with chart + PII values
      2. Verify approval request sent for action 2 (sensitive)
      3. Approve → verify replay continues
      4. Verify all 3 actions executed
    Expected Result: Replay completes after approval granted for action 2
    Failure Indicators: Sensitive action executed without approval
    Evidence: .sisyphus/evidence/browser-automation/task-23-replay-approval.txt

  Scenario: Denied approval aborts replay
    Tool: Bash (go test)
    Preconditions: Chart with sensitive action
    Steps:
      1. Call ReplayChart
      2. Deny approval for sensitive action
      3. Verify replay aborted, remaining actions not executed
    Expected Result: Replay fails with "approval denied", partial execution logged
    Failure Indicators: Remaining actions execute after denial
    Evidence: .sisyphus/evidence/browser-automation/task-23-replay-denied.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): enforce replay-through-approval for chart actions`
  - Files: `bridge/pkg/browser/jetski_broker.go`
  - Pre-commit: `cd bridge && go test ./pkg/browser/...`

- [x] 24. Audit Trail for Chart Lifecycle

  **What to do**:
  - Add audit logging for all chart lifecycle events:
    - `chart.created` — when a chart is saved (includes: chartID, domain, action count, source session)
    - `chart.updated` — when confidence/usage changes (includes: chartID, new confidence, outcome)
    - `chart.replayed` — when a chart is used for replay (includes: chartID, jobID, actions executed, approvals needed/granted/denied)
    - `chart.rejected` — when validation fails (includes: chartID, reason, validator errors)
    - `chart.deleted` — when a chart is removed (includes: chartID, domain)
  - Use Bridge's existing event emission pattern (agent.StatusEvent or a new `chart.ChartEvent`)
  - Store audit entries in the secretary SQLite alongside charts (add `chart_audit` table):
    ```sql
    CREATE TABLE IF NOT EXISTS chart_audit (
        event_id INTEGER PRIMARY KEY AUTOINCREMENT,
        event_type TEXT NOT NULL,
        chart_id TEXT NOT NULL,
        details TEXT NOT NULL,  -- JSON
        created_at INTEGER NOT NULL
    );
    CREATE INDEX IF NOT EXISTS idx_audit_chart ON chart_audit(chart_id);
    CREATE INDEX IF NOT EXISTS idx_audit_type ON chart_audit(event_type);
    ```
  - Emit events to Matrix via existing event broadcaster for ArmorChat visibility

  **Must NOT do**:
  - Do NOT store PII values in audit log (only placeholders)
  - Do NOT make audit logging optional — it's always on

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with T23, T25)
  - **Blocks**: None directly
  - **Blocked By**: T22

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/store.go:236-252` — `initSchema()` where `chart_audit` table should be added
  - `bridge/pkg/browser/browser.go:61` — BrowserSkill emitting StatusEvents — audit events should use similar pattern

  **API/Type References**:
  - `bridge/pkg/browser/chart_store.go` — ChartStore interface (T20) where audit events should be emitted on Save/RecordOutcome/Delete

  **WHY Each Reference Matters**:
  - `store.go:236-252` is where all schema additions go — add chart_audit table here
  - StatusEvent emission pattern is established — chart events follow same approach

  **Acceptance Criteria**:

  - [ ] All 5 chart lifecycle events produce audit entries
  - [ ] Audit entries stored in chart_audit table
  - [ ] No PII values in audit details
  - [ ] `go test ./pkg/browser/...` passes

  **QA Scenarios:**

  ```
  Scenario: Chart creation produces audit entry
    Tool: Bash (go test)
    Preconditions: ChartStore initialized
    Steps:
      1. SaveChart → get chartID
      2. Query chart_audit for event_type='chart.created' AND chart_id=chartID
      3. Verify details JSON contains domain and action count
    Expected Result: Audit entry exists with correct event type and details
    Failure Indicators: No audit entry, or missing fields
    Evidence: .sisyphus/evidence/browser-automation/task-24-audit-create.txt

  Scenario: Chart replay produces audit entry with approval status
    Tool: Bash (go test)
    Preconditions: Chart replayed successfully with 1 approval
    Steps:
      1. ReplayChart
      2. Query chart_audit for event_type='chart.replayed'
      3. Verify details include approvals_needed=1, approvals_granted=1
    Expected Result: Replay audit includes approval counts
    Failure Indicators: Audit missing approval details
    Evidence: .sisyphus/evidence/browser-automation/task-24-audit-replay.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): add audit trail for chart lifecycle events`
  - Files: `bridge/pkg/secretary/store.go`, `bridge/pkg/browser/chart_audit.go`
  - Pre-commit: `cd bridge && go test ./pkg/browser/...`

- [x] 25. PII Test Scanner

  **What to do**:
  - Create `bridge/pkg/browser/pii_scanner.go` — a standalone scanner that checks ALL stored charts for raw PII
  - `ScanChartsForPII(ctx context.Context) ([]PIIFinding, error)` — iterates all charts, runs PII regex against all step values
  - `PIIFinding` struct: ChartID, ActionIndex, Field, MatchedPattern, Severity
  - This is a diagnostic tool to verify that the validator (T22) is working correctly — run it periodically
  - Also create a harness test `tests/test-navchart-security.sh` that runs the scanner:
    - **NS0**: Prerequisites
    - **NS1**: No raw PII in stored charts (scanner returns 0 findings)
    - **NS2**: Policy rejection (malformed chart rejected by validator)
    - **NS3**: Approval still required on replay
    - **NS4**: Audit log entries present for chart operations
    - **NS5**: Malicious/malformed chart rejected

  **Must NOT do**:
  - Do NOT auto-fix PII leaks — just detect and report
  - Do NOT run as a background daemon — it's an on-demand scan

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with T23, T24)
  - **Blocks**: None directly
  - **Blocked By**: T22

  **References**:

  **Pattern References**:
  - `jetski/internal/security/pii_scanner.go:41-89` — Existing PII scanner showing the 4 patterns and ScanCDPMessage approach
  - `tests/test-trust-layer.sh:1-653` — Existing trust-layer test showing the harness pattern for security tests

  **API/Type References**:
  - `bridge/pkg/browser/chart_store.go` — ChartStore.GetChart for iterating charts
  - `bridge/pkg/browser/chart_types.go` — NavChart with action values to scan

  **WHY Each Reference Matters**:
  - `pii_scanner.go:41-89` has the exact PII detection patterns — Go scanner must use same regexes
  - `test-trust-layer.sh` is the security test template — new test follows same structure

  **Acceptance Criteria**:

  - [ ] Scanner detects raw PII in chart step values
  - [ ] Scanner returns 0 findings for clean charts
  - [ ] `tests/test-navchart-security.sh` runs with PASS/FAIL/SKIP
  - [ ] `go test ./pkg/browser/...` passes

  **QA Scenarios:**

  ```
  Scenario: Scanner detects injected PII
    Tool: Bash (go test)
    Preconditions: ChartStore with a chart containing raw SSN (bypassing validator — simulating a bug)
    Steps:
      1. Insert chart with raw "123-45-6789" directly into DB
      2. Run ScanChartsForPII
      3. Verify finding with pattern=SSN
    Expected Result: 1 finding with SSN pattern detected
    Failure Indicators: 0 findings despite raw PII present
    Evidence: .sisyphus/evidence/browser-automation/task-25-pii-scan.txt

  Scenario: Security harness passes on clean charts
    Tool: Bash
    Preconditions: Charts stored via normal pipeline (validator active)
    Steps:
      1. bash tests/test-navchart-security.sh
      2. Verify all NS0-NS5 PASS
    Expected Result: All scenarios PASS
    Failure Indicators: Any FAIL
    Evidence: .sisyphus/evidence/browser-automation/task-25-security-harness.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): add PII scanner for stored charts, add navchart security test`
  - Files: `bridge/pkg/browser/pii_scanner.go`, `tests/test-navchart-security.sh`
  - Pre-commit: `cd bridge && go test ./pkg/browser/...`

---

## Phase 4 — Robustness and Optimization

- [x] 26. Selector Fallback with Confidence Scoring

  **What to do**:
  - When executing a chart action, try selectors in order: `primary_css` → `secondary_xpath` → `fallback_js`
  - If primary selector fails, try fallback with a WARNING log
  - Track which selector tier succeeded in `ChartStore.RecordOutcome()` — increment tier-specific counters
  - Adjust chart confidence based on selector tier used:
    - Primary hit: confidence += 0.05
    - Secondary hit: confidence += 0.02
    - Fallback hit: confidence += 0.01
    - All fail: confidence -= 0.1
  - Cap confidence at [0.0, 1.0]
  - Add `selector_tier` field to `learned_charts` table: `primary_hits`, `secondary_hits`, `fallback_hits` INTEGER columns

  **Must NOT do**:
  - Do NOT implement AI-based selector generation (out of scope)
  - Do NOT modify charts in place — only update metadata

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 6 (with T27-T30)
  - **Blocks**: None
  - **Blocked By**: T19, T22

  **References**: `bridge/pkg/browser/chart_types.go` ChartSelector (primary_css, secondary_xpath, fallback_js)

  **Acceptance Criteria**:
  - [ ] Fallback to secondary selector works when primary fails
  - [ ] Confidence score updated based on tier used
  - [ ] `go test ./pkg/browser/...` passes

  **QA Scenarios:**
  ```
  Scenario: Primary selector fails, fallback succeeds
    Tool: Bash (go test)
    Steps:
      1. Replay chart with invalid primary CSS but valid fallback JS
      2. Verify action succeeds via fallback
      3. Verify confidence adjusted for fallback tier
    Expected Result: Action succeeds, confidence += 0.01, fallback_hits incremented
    Evidence: .sisyphus/evidence/browser-automation/task-26-fallback.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): add selector fallback with confidence scoring`
  - Files: `bridge/pkg/browser/jetski_broker.go`, `bridge/pkg/browser/chart_store.go`
  - Pre-commit: `cd bridge && go test ./pkg/browser/...`

- [ ] 27. Multi-Tab and Popup Handling **[STRETCH GOAL]**

  > **Skip condition**: Phase 4 succeeds without this task. Execute only if time permits and core Phase 4 tasks (T26, T28, T29) are complete. Multi-tab handling is not required for initial production deployment — most automation workflows are single-tab.

  **What to do**:
  - Handle CDP `Target.targetCreated` events during chart execution
  - When a new tab/popup opens during replay:
    - Track the new target ID
    - Attach to the new target's CDP session
    - Continue chart actions in the new context if needed
    - Close popup tabs after chart completion
  - Add `Page.getWindowInfo` call at start of each chart to track window context

  **Must NOT do**:
  - Do NOT implement cross-tab orchestration (out of scope)
  - Do NOT add popup blocking — just handle existing popups

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave 6 | **Blocked By**: T14

  **References**: `jetski/internal/cdp/router.go:41-54` (Target domain passthrough)

  **Acceptance Criteria**:
  - [ ] Popup detected and tracked during execution
  - [ ] Popup closed after chart completion
  - [ ] No resource leaks from unclosed tabs

  **QA Scenarios:**
  ```
  Scenario: Popup detected during navigate
    Tool: Bash (go test)
    Steps:
      1. Navigate to page that opens popup
      2. Verify Target.targetCreated event captured
      3. Complete chart, verify popup closed
    Expected Result: Popup tracked and closed cleanly
    Evidence: .sisyphus/evidence/browser-automation/task-27-popup.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): add multi-tab and popup handling`
  - Files: `bridge/pkg/browser/jetski_broker.go`

- [x] 28. Chart Versioning and Rollback

  **What to do**:
  - Add version tracking to charts: when a chart is updated, create a new version with `parent_chart_id` pointing to the previous version
  - Implement `ChartStore.ListVersions(ctx, chartID) ([]ChartRecord, error)` — returns all versions of a chart
  - Implement `ChartStore.RevertToVersion(ctx, chartID, version int) error` — sets a specific version as active by adjusting confidence scores
  - Add `version` column increment logic in `SaveChart`: when saving a chart with same domain+title as existing, increment version

  **Must NOT do**:
  - Do NOT delete old versions — keep full history
  - Do NOT implement branching (just linear versioning)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave 6 | **Blocked By**: T20, T22

  **References**: `bridge/pkg/browser/chart_store.go` ChartStore interface (T20)

  **Acceptance Criteria**:
  - [ ] Saving same-domain chart creates new version
  - [ ] ListVersions returns full history
  - [ ] RevertToVersion restores previous confidence
  - [ ] `go test ./pkg/browser/...` passes

  **Commit**: YES
  - Message: `feat(bridge): add chart versioning and rollback`
  - Files: `bridge/pkg/browser/chart_store.go`

- [x] 29. Performance Benchmarking

  **What to do**:
  - Create `tests/test-navchart-pipeline.sh` — harness test for the full chart pipeline:
    - **NP0**: Prerequisites
    - **NP1**: Record → normalize → store pipeline
    - **NP2**: Placeholder insertion verified
    - **NP3**: Confidence metadata correct
    - **NP4**: Chart reused in later workflow step
    - **NP5**: Pipeline performance benchmark (record+normalize+store < 5s for 100 frames)
  - Add timing instrumentation to normalizer.go and chart_store.go
  - Log timing for: normalization, validation, storage, retrieval, replay

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave 6 | **Blocked By**: T15

  **References**: `tests/test-browser-broker.sh` (T10) for harness pattern

  **Acceptance Criteria**:
  - [ ] Pipeline test runs with PASS/FAIL/SKIP
  - [ ] Normalization of 100 frames completes in < 5s
  - [ ] Evidence saved to `.sisyphus/evidence/browser-automation/`

  **Commit**: YES
  - Message: `test(harness): add navchart pipeline test with performance benchmarks`
  - Files: `tests/test-navchart-pipeline.sh`

- [ ] 30. Replay Diagnostics **[STRETCH GOAL]**

  > **Skip condition**: Phase 4 succeeds without this task. Execute only if time permits and core Phase 4 tasks (T26, T28, T29) are complete. Diagnostics are valuable for debugging but not required for initial production — existing logging provides sufficient coverage for v1.

  **What to do**:
  - Add detailed diagnostic logging for chart replay:
    - Per-action timing (start, end, duration)
    - Selector tier used (primary/secondary/fallback)
    - CDP command and response for each action (sanitized — no PII)
    - Approval request/response timing
    - Screenshot at each action (optional, controlled by `ARMORCLAW_REPLAY_SCREENSHOTS` flag)
  - Store diagnostics in chart_audit table with event_type `chart.replay_action`
  - Create `bridge/pkg/browser/replay_diagnostics.go` — `ReplayDiagnostics` struct that collects per-action data and writes to audit on completion

  **Must NOT do**:
  - Do NOT store CDP responses containing PII
  - Do NOT make screenshots mandatory (performance impact)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave 6 | **Blocked By**: T23

  **References**: `bridge/pkg/browser/chart_audit.go` (T24) for audit event emission

  **Acceptance Criteria**:
  - [ ] Each replay action logged with timing
  - [ ] No PII in diagnostic output
  - [ ] Screenshots optional via flag
  - [ ] `go test ./pkg/browser/...` passes

  **Commit**: YES
  - Message: `feat(bridge): add replay diagnostics with timing and screenshots`
  - Files: `bridge/pkg/browser/replay_diagnostics.go`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in `.sisyphus/evidence/browser-automation/`. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go build ./...` + `go vet ./...` on both `bridge/` and `jetski/`. Run `go test ./...` on bridge. Review all changed files for: `as any`, empty catches, `fmt.Println` in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Agent-Executed End-to-End QA Sweep** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (Navigate+Fill+Click+Extract working together, not isolation). Test edge cases: empty state, invalid input, Jetski down. Save to `.sisyphus/evidence/browser-automation/final-qa/`.
  **Mandatory cross-subsystem scenario X7**: Record a NavChart via chart recording (T19), store it via ChartStore (T20), then replay it through the Secretary workflow (T22 validates, T23 replays). Verify the replayed actions produce equivalent DOM state to the original recording. This is the end-to-end proof that Phases 2 and 3 integrate correctly.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Cross-subsystem [X7 PASS/FAIL] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination: Task N touching Task M's files. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

| Wave | Commit Message | Key Files | Pre-commit |
|------|---------------|-----------|------------|
| 1 | `fix(bridge): split BrowserConfig into CDP/RPC/Legacy endpoints, add backend selection` | `setup_secretary.go`, `config.go`, `processor.go` | `go build ./...` |
| 1 | `feat(jetski): add approval env var overrides to config` | `config.go` | `go build ./...` |
| 1 | `fix(jetski): wire approval client in main.go` | `main.go` | `go build ./...` |
| 1 | `fix(jetski): resolve CGO/SQLCipher Dockerfile conflict` | `Dockerfile` | `docker build` |
| 1 | `fix(jetski): align docker-compose env vars with config struct` | `docker-compose.jetski.yml` | `docker compose config` |
| 1 | `feat(bridge): add BrowserBroker interface and types` | `broker.go`, `broker_types.go` | `go build ./...` |
| 1 | `fix(bridge): wire injectLearnedSkills into step execution` | `orchestrator_integration.go` | `go test ./...` |
| 2 | `feat(bridge): implement JetskiBroker.Navigate` | `jetski_broker.go` | `go build ./...` |
| 2 | `feat(bridge): register JetskiBroker with feature flag` | `setup_secretary.go` | `go build ./...` |
| 2 | `test(harness): add Phase 0 exit criteria with latency and restart gates` | `tests/test-browser-broker.sh` | `bash tests/test-browser-broker.sh` |
| 3b | `test(harness): add Phase 1 exit criteria (fill, click, extract, screenshot)` | `tests/test-browser-broker.sh` | `bash tests/test-browser-broker.sh` |
| 3 | (one commit per semantic method implementation) | `jetski_broker.go` | `go test ./...` |
| 4 | (one commit per chart/recording component) | various | `go build ./...` |
| 5 | (one commit per security component) | various | `go test ./...` |
| 6 | (one commit per robustness feature) | various | `go test ./...` |

---

## Success Criteria

### Verification Commands
```bash
# Bridge builds
cd bridge && go build ./...                    # Expected: success
cd bridge && go test ./...                     # Expected: all pass

# Jetski builds
cd jetski && go build ./...                    # Expected: success
cd jetski && go vet ./...                      # Expected: clean

# Docker builds
docker compose -f docker-compose.jetski.yml build  # Expected: success
docker compose -f docker-compose.jetski.yml up -d   # Expected: healthy
curl http://localhost:9223/rpc/health                # Expected: {"status":"ok"}

# Phase 0 exit criteria
bash tests/test-browser-broker.sh              # Expected: BB0-BB7 PASS
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All existing tests still pass (zero regressions)
- [ ] Phase 0 exit criteria: functional + security + operational gates pass
- [ ] Browser job success rate ≥ 92% (measured over test runs)
- [ ] Raw PII persisted in charts: 0 incidents
- [ ] Replay preserves approval/blocker semantics in 100% of gated cases
