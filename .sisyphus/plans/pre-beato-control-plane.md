# Pre-BEATO Control Plane Validation — Remaining Work Plan

## TL;DR

> **Quick Summary**: Prove the live Matrix control plane is trustworthy through fresh baseline, transport migration, assertion alignment, command coverage, lifecycle proofs, resilience gates, and CI guardrails.
>
> **Current State**: Infrastructure is stable. Transport, HTTPS, Matrix login/sync, WebSocket, and bridge restart all work. Remaining gaps are: 32 scripts on legacy transport, 6 assertion mismatches, no Matrix command coverage proof, no Studio/Secretary lifecycle proof, no CI guardrails.
>
> **Deliverables**:
> - Fresh baseline from pushed repo state (17 PASS / 8 FAIL / 22 PARTIAL / 2 ENV_MISSING is current)
> - All 51 test scripts on shared transport
> - 6 assertion-fix patches aligned to actual v4.6.0 behavior
> - Matrix command coverage matrix
> - Matrix-driven control-flow tests (happy + negative paths)
> - Studio lifecycle proof test
> - Secretary lifecycle proof test (aligned to probed method availability)
> - Matrix command → execution → event correlation test
> - Restart recovery, WSS reconnect, concurrency gate tests
> - CI guardrails (bash -n, go vet, go test) + nightly resilience jobs
>
> **Estimated Effort**: Large
> **Parallel Execution**: YES — 6 waves
> **Critical Path**: T1 → T4 → T6 → T9 → T12 → T14 → T15 → T19

---

## Part 1: Completed Stabilization (Reference Only)

This section documents what was already achieved. It is NOT work to be done.

### Phase 1 + Phase 2 Stabilization — DONE

| What | Status | Evidence |
|------|--------|----------|
| Shared transport detector (`tests/lib/transport.sh`) | ✅ Done | Commit `f422d99` |
| 16 scripts migrated to shared transport | ✅ Done | Commit `9b62f1e` |
| 129+ Go unit tests across 5 packages | ✅ Done | Commit `2bbc884` |
| VPS bridge detection (HTTPS-first) | ✅ Done | Commit `0c19677` |
| HTTPS transport fix (3 shared libs) | ✅ Done | Commit `06ff150` |
| Matrix bridge user re-registered | ✅ Done | `logged_in: true, connected: true` |
| SSH self-reference fixed | ✅ Done | ed25519 key on VPS |
| Tools installed (jq, socat, websocat) | ✅ Done | All available on VPS |
| YARA container filter fix | ✅ Partial | `name=armorclaw` — still FAIL in suite |
| WebSocket 30s stability + reconnect | ✅ Observed | Not formalized as gate test |
| Bridge restart recovery (2-5s) | ✅ Observed | Not formalized as gate test |
| Studio: `studio.list_agents` works | ✅ Verified | Returns `{"agents":null,"count":0}` |
| Studio: full lifecycle (create/spawn/stop/delete) | ❌ Not proven | Only list_agents tested |
| Secretary: any method working | ❌ Not proven | `secretary.list_workflows` → "method not found" |
| Full suite: 17 PASS / 8 FAIL / 22 PARTIAL / 2 ENV_MISSING | ✅ Current | 49 scripts |

### What the 8 FAIL Are

| Script | Root Cause | Classification |
|--------|-----------|----------------|
| test-browser-smoke.sh | Jetski not deployed | Feature-gated (BEATO-entry) |
| test-webrtc-voice.sh | Voice stack not provisioned | Feature-gated (BEATO-entry) |
| test-yara-smoke.sh | Container matching incomplete | Pre-existing assertion |
| test-deployment-skills.sh | `.skills/` structure mismatch | Pre-existing assertion |
| test-quickstart-entrypoint.sh | Output format expectation | Pre-existing assertion |
| test-secrets.sh | Response shape mismatch | Pre-existing assertion |
| test-vps-smoke.sh | Expects `mode=http`, gets `mode=both` | Pre-existing assertion |
| test-p0crit3-socket-injection.sh | Socket environment assumptions | Pre-existing assertion |

---

## Part 2: Pre-BEATO Definition

### What "Pre-BEATO Complete" Means

Pre-BEATO is done when ALL of these are true:

1. **Transport stable**: All scripts use shared transport, no legacy paths
2. **Matrix connected**: `matrix.status` returns `logged_in: true, connected: true`
3. **WSS stable**: WebSocket connection proven stable ≥30s with event receipt
4. **Restart proven**: Bridge restart recovery is a repeatable gate test (not anecdotal)
5. **No infra-caused FAILs**: Zero FAIL caused by transport, SSH, missing tools, or harness bugs
6. **One real command → execution → event flow proven**: Matrix command triggers action, event confirms completion
7. **Studio lifecycle proven**: create → get → spawn → list → stop → delete with cleanup verification
8. **Secretary aligned**: Method availability probed, available methods tested, documented
9. **CI catches regressions**: bash -n, go vet, go test run on every push
10. **Baseline reproducible**: Full suite runnable from pushed repo state on VPS

### What is NOT Pre-BEATO (BEATO-Entry)

These are explicitly out of scope. Do NOT include in pre-BEATO waves:

- Deploy Jetski browser sidecar → resolves browser-smoke FAIL
- Provision voice stack → resolves webrtc-voice FAIL
- Deploy Office/document sidecars
- Broad Browser/Email/Audio/Office E2E validation
- Registering new RPC methods
- Any ArmorChat work

### Pre-BEATO Gate Metric

The primary gate is NOT "≥80% PASS across all 49 scripts."

The correct gate is:

> **Zero infrastructure-caused FAILs, with feature-gated scripts excluded from the denominator.**

Feature-gated scripts (browser-smoke, webrtc-voice) should report SKIP or ENV_MISSING when their subsystem is absent. The PASS rate is calculated against the remaining scripts.

---

## Part 3: Remaining Pre-BEATO Work (The Actual Plan)

### Scope Boundaries

- **INCLUDE**: Transport migration, assertion alignment, Matrix command proofs, Studio/Secretary lifecycle, resilience gates, CI guardrails, evidence discipline
- **EXCLUDE**: Jetski, voice, Office sidecars, new RPC methods, ArmorChat, production code changes

### Guardrails

- Do NOT change production handler behavior — fix test assertions only
- Do NOT treat feature-gated failures as product bugs
- Do NOT add new RPC methods
- Do NOT modify container-setup.sh or deploy/container-setup.sh
- Do NOT touch bridge/pkg/yara/scanner.go internal logic
- Do NOT touch cmd/ directory
- Do NOT add structured logging library
- Do NOT log or print credential values in any test or plan file
- Do NOT use destructive operations in concurrency tests
- Do NOT hardcode https:// in transport — use auto-detect with HTTP fallback
- Do NOT assume Secretary methods work — probe availability first
- Accept `mode=both` as a valid transport state (bridge runs `--network host`)

---

## Verification Strategy

> **All verification is agent-executed.** No human manual testing required.
> Acceptance criteria requiring "user manually tests/confirms" are forbidden.

### Test Approach
- **Existing harness**: bash scripts on VPS via SSH
- **New test scripts**: Follow existing pattern (source transport.sh, use helpers)
- **Go tests**: `go test` for bridge package regression
- **CI**: GitHub Actions for automated checks

### Evidence Format
Every task stores evidence to `.sisyphus/evidence/pbcp-{NN}/`:
- Command transcript (full output)
- Raw JSON payloads where relevant
- Commit SHA and bridge image tag
- PASS/FAIL result per scenario

### Final Verification

After ALL implementation tasks, run a consolidated verification:
1. Full suite rerun from pushed state
2. Check all 10 pre-BEATO exit criteria against evidence
3. Present results to user for explicit approval

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — baseline + convergence + CI):
├── T1:  Full 49-script baseline rerun + evidence bundle [deep]
├── T2:  VPS repo convergence verification [quick]
└── T3:  CI guardrails — bash -n + go vet + go test workflow [quick]

Wave 2 (After Wave 1 — transport migration + assertion alignment + Matrix contract):
├── T4:  Migrate 32 legacy scripts batch 1 (16 scripts) [unspecified-high]
├── T5:  Migrate 32 legacy scripts batch 2 (16 scripts) [unspecified-high]
├── T6:  Fix 6 assertion mismatches aligned to v4.6.0 [unspecified-high]
└── T7:  Lock matrix.status contract + document valid states [quick]

Wave 3 (After Wave 2 — transport proof + Matrix command coverage):
├── T8:  WebSocket/EventBus E2E proof test [deep]
├── T9:  Matrix command coverage matrix document [writing]
├── T10: Matrix-driven control-flow tests (happy paths) [deep]
└── T11: Matrix negative/error-path tests [unspecified-high]

Wave 4 (After Wave 3 — stateful lifecycle proofs):
├── T12: Studio lifecycle proof test [deep]
├── T13: Secretary lifecycle proof test (probe-first) [deep]
└── T14: Matrix command → execution → event correlation test [deep]

Wave 5 (After Wave 4 — resilience gates):
├── T15: Restart recovery gate test [unspecified-high]
├── T16: WSS disconnect/reconnect gate test [quick]
└── T17: Concurrency smoke test [deep]

Wave 6 (After Wave 5 — CI nightly + final baseline):
├── T18: Nightly resilience CI jobs [quick]
└── T19: Final baseline rerun + exit criteria verification [deep]

Wave FINAL (Consolidated verification — present to user):
├── Full suite rerun from pushed state
├── Check all 10 exit criteria against evidence
└── Present results → get user explicit approval
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1 | — | T4, T5, T6, T19 | 1 |
| T2 | — | T4, T5, T6 | 1 |
| T3 | — | T18 | 1 |
| T4 | T1 | T8, T10 | 2 |
| T5 | T1 | T8, T10 | 2 |
| T6 | T1 | T10 | 2 |
| T7 | T1 | T9 | 2 |
| T8 | T4, T5 | T12, T13, T15, T16, T17 | 3 |
| T9 | T7 | T10, T11 | 3 |
| T10 | T6, T9 | T14 | 3 |
| T11 | T9, T10 | T14 | 3 |
| T12 | T8 | T14 | 4 |
| T13 | T8 | T14 | 4 |
| T14 | T10, T11, T12, T13 | T19 | 4 |
| T15 | T8 | T18, T19 | 5 |
| T16 | T8 | T18, T19 | 5 |
| T17 | T8 | T18, T19 | 5 |
| T18 | T3, T15 | — | 6 |
| T19 | T1, T14, T15, T16, T17 | FINAL | 6 |

### Agent Dispatch Summary

| Wave | Count | Assignments |
|------|-------|-------------|
| 1 | 3 | T1→deep, T2→quick, T3→quick |
| 2 | 4 | T4→unspecified-high, T5→unspecified-high, T6→unspecified-high, T7→quick |
| 3 | 4 | T8→deep, T9→writing, T10→deep, T11→unspecified-high |
| 4 | 3 | T12→deep, T13→deep, T14→deep |
| 5 | 3 | T15→unspecified-high, T16→quick, T17→deep |
| 6 | 2 | T18→quick, T19→deep |

---

## TODOs

- [x] T1. **Full 49-Script Baseline Rerun + Evidence Bundle** (Phase A)

  **What to do**:
  - SSH to VPS, verify bridge running: `curl -ksS https://localhost:8080/health`
  - Record metadata: commit SHA, container image tag, bridge version, VPS IP
  - Run ALL 49 test scripts, capture per-script output
  - Tally: PASS / FAIL / PARTIAL / ENV_MISSING
  - Store in `.sisyphus/evidence/pbcp-01/`
  - Write `.sisyphus/reports/pre-beato-fresh-baseline.md`

  **Must NOT do**: Fix failures, skip scripts, modify any files.

  **Recommended Agent**: `deep`
  - **Parallel Group**: Wave 1 | **Blocks**: T4, T5, T6, T19 | **Blocked By**: None

  **References**:
  - `tests/lib/transport.sh` — Transport functions scripts use
  - `tests/reports/test-harness-stabilization-report.md` — Previous baseline format

  **QA Scenarios**:
  ```
  Scenario: Baseline captures all 49 scripts with metadata
    Tool: Bash (SSH)
    Steps:
      1. Record commit SHA: `ssh_vps "cd /opt/armorclaw && git rev-parse HEAD"`
      2. Record bridge version: `ssh_vps "curl -ksS https://localhost:8080/health"`
      3. Run all 49 scripts, capture output per script
      4. Tally results, write report
    Expected: All 49 scripts tallied, commit SHA and bridge version recorded
    Evidence: .sisyphus/evidence/pbcp-01/baseline-full.log
  ```

  **Commit**: NO (read-only)

- [x] T2. **VPS Repo Convergence Verification** (Phase A)

  **What to do**:
  - Compare VPS git SHA with local origin/main SHA
  - If behind, `git pull origin main` on VPS
  - Check for uncommitted changes: `git status --short`
  - Record exact deployed SHA

  **Must NOT do**: Force push, reset, modify VPS files beyond `git pull`.

  **Recommended Agent**: `quick`
  - **Parallel Group**: Wave 1 | **Blocks**: T4, T5, T6 | **Blocked By**: None

  **References**: `tests/lib/load_env.sh` — `ssh_vps()` helper

  **QA Scenarios**:
  ```
  Scenario: VPS SHA matches origin/main
    Steps:
      1. Local: `git rev-parse origin/main`
      2. VPS: `ssh_vps "cd /opt/armorclaw && git rev-parse HEAD"`
      3. Must match
    Evidence: .sisyphus/evidence/pbcp-02/convergence.txt
  ```

  **Commit**: NO

- [x] T3. **CI Guardrails — bash -n + go vet + go test** (Phase F)

  **What to do**:
  - Add new job `script-and-go-checks` to `.github/workflows/test.yml`:
    - `bash -n` on all `tests/test-*.sh` and `tests/lib/*.sh`
    - `go vet` on 6 critical packages (rpc, matrix, security, socket, websocket, executor)
    - `go test -short` on same packages
  - Add as dependency for existing test jobs

  **Must NOT do**: Modify existing jobs, add nightly schedule (T18), run VPS suite in CI.

  **Recommended Agent**: `quick`
  - **Parallel Group**: Wave 1 | **Blocks**: T18 | **Blocked By**: None

  **References**:
  - `.github/workflows/test.yml:1-200` — Existing structure to follow
  - `.github/workflows/dockerhub.yml` — Go setup patterns

  **QA Scenarios**:
  ```
  Scenario: CI workflow validates scripts and Go packages
    Steps:
      1. Local: `bash -n` all test scripts → 0 errors
      2. Local: `cd bridge && go vet ./pkg/rpc/... ./pkg/matrix/... ./pkg/security/... ./pkg/socket/... ./pkg/websocket/... ./internal/executor/...` → 0 issues
      3. Local: `cd bridge && go test -short` same packages → all PASS
      4. YAML valid: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/test.yml'))"`
    Evidence: .sisyphus/evidence/pbcp-03/ci-guardrails-check.txt
  ```

  **Commit**: YES — `ci(tests): add bash -n, go vet, go test guardrails` — `.github/workflows/test.yml`

- [x] T4. **Migrate 32 Legacy Scripts to Shared Transport — Batch 1** (Phase A)

  **What to do**:
  Migrate first 16 scripts. For each: add `source transport.sh`, replace direct curl/socat with transport helpers.
  - test-agent-runtime.sh, test-browser-broker.sh, test-browser-smoke.sh
  - test-cloudflare-setup.sh (consistency only, may have 0 bridge calls)
  - test-container-setup.sh (consistency only)
  - test-deployment-skills.sh, test-deployment-usb.sh (consistency only)
  - test-discovery.sh, test-e2e.sh, test-element-x-flow.sh
  - test-eventbus-filtering.sh (socat=8 — high priority)
  - test-exploits.sh, test-governance-rpc.sh (socat=2)
  - test-matrix-client-flow.sh, test-matrix-e2e-rpc.sh (curl+socat)
  - test-matrix-plane.sh (curl=5)

  **Must NOT do**: Change test logic or assertions. Modify transport.sh.

  **Recommended Agent**: `unspecified-high`
  - **Parallel Group**: Wave 2 | **Blocks**: T8, T10 | **Blocked By**: T1

  **References**:
  - `tests/lib/transport.sh` — Helper functions
  - `tests/test-eventbus-streaming.sh` — Already migrated, shows pattern
  - `tests/test-trust-layer.sh` — Already migrated, curl→bridge_curl pattern

  **QA Scenarios**:
  ```
  Scenario: All 16 batch-1 scripts source transport.sh
    Steps: grep -c 'transport.sh' each file → ≥1; bash -n each → 0 errors
    Evidence: .sisyphus/evidence/pbcp-04/migration-batch1.txt
  Scenario: No direct curl to localhost:8080 remains
    Steps: grep 'curl.*localhost:8080' all 16 files → 0 matches (outside comments)
    Evidence: .sisyphus/evidence/pbcp-04/direct-calls-removed.txt
  ```

  **Commit**: YES — `refactor(tests): migrate batch 1 (16 scripts) to shared transport` — 16 files

- [x] T5. **Migrate 32 Legacy Scripts to Shared Transport — Batch 2** (Phase A)

  **What to do**:
  Migrate remaining 16 scripts:
  - test-persistence.sh, test-pii-flow.sh, test-provisioning.sh
  - test-quickstart-entrypoint.sh, test-rpc-methods.sh
  - test-secretary-workflow-deep.sh, test-secret-passing.sh
  - test-secrets.sh, test-studio-lifecycle.sh
  - test-tls-mode-integration.sh, test-tls-restart-safety.sh
  - test-token-recovery.sh, test-vps-smoke.sh
  - test-webrtc-voice-integration.sh, test-yara-smoke.sh
  - test-p0crit3-socket-injection.sh

  **Must NOT do**: Same as T4.

  **Recommended Agent**: `unspecified-high`
  - **Parallel Group**: Wave 2 | **Blocks**: T8, T10 | **Blocked By**: T1

  **References**: Same as T4.

  **QA Scenarios**:
  ```
  Scenario: All batch-2 scripts source transport.sh; zero scripts unmigrated
    Steps: grep -L 'transport\.sh' tests/test-*.sh → empty (or only scripts with 0 bridge calls)
    Evidence: .sisyphus/evidence/pbcp-05/migration-batch2.txt
  ```

  **Commit**: YES — `refactor(tests): migrate batch 2 (16 scripts) to shared transport` — 16 files

- [x] T6. **Align 6 Assertion Mismatches to v4.6.0** (Phase B)

  **What to do**:
  For each of 6 scripts, capture the ACTUAL v4.6.0 response first, then fix the assertion:
  1. **test-vps-smoke.sh**: Accept `mode=both` (bridge on `--network host` has both socket+HTTPS)
  2. **test-quickstart-entrypoint.sh**: Capture actual output format, update expectations
  3. **test-secrets.sh**: Capture actual response JSON, update field expectations
  4. **test-yara-smoke.sh**: Complete container name matching fix
  5. **test-p0crit3-socket-injection.sh**: Fix socket path environment assumptions
  6. **test-deployment-skills.sh**: Capture actual `.skills/` structure, update expectations

  **Must NOT do**: Change production handler behavior. Remove test coverage. Treat feature-gated FAILs as assertion issues.

  **Recommended Agent**: `unspecified-high`
  - **Parallel Group**: Wave 2 | **Blocks**: T10 | **Blocked By**: T1

  **References**:
  - `bridge/pkg/rpc/server.go:1219-1357` — Method registry for understanding response shapes

  **QA Scenarios**:
  ```
  Scenario: All 6 fixed scripts PASS on VPS
    Steps: Run each script on VPS individually → all PASS
    Evidence: .sisyphus/evidence/pbcp-06/assertion-fixes.txt
  Scenario: Actual v4.6.0 responses captured for each fix
    Steps: curl each relevant endpoint, save JSON response alongside assertion change
    Evidence: .sisyphus/evidence/pbcp-06/actual-responses.json
  ```

  **Commit**: YES — `fix(tests): align 6 assertion mismatches to v4.6.0 behavior` — 6 files

- [x] T7. **Lock matrix.status Contract** (Phase B)

  **What to do**:
  - Call `matrix.status` on VPS, record full JSON response
  - Document valid states in `tests/reports/matrix-status-contract.md`:
    - `logged_in: true, connected: true` — operating
    - `logged_in: true, connected: false` — configured, disconnected
    - `logged_in: false` — not configured
    - Error: Matrix unavailable
  - Cross-reference Go source in `bridge/pkg/matrix/` for all possible states

  **Must NOT do**: Change handler behavior, add new fields.

  **Recommended Agent**: `quick`
  - **Parallel Group**: Wave 2 | **Blocks**: T9 | **Blocked By**: T1

  **References**: `bridge/pkg/matrix/client.go`, `bridge/pkg/rpc/server.go:1265`

  **QA Scenarios**:
  ```
  Scenario: matrix.status response matches documented contract
    Steps: Call matrix.status on VPS, verify response has logged_in (bool) + connected (bool)
    Evidence: .sisyphus/evidence/pbcp-07/matrix-status-response.json
  ```

  **Commit**: YES — `docs(tests): document matrix.status contract` — `tests/reports/matrix-status-contract.md`

- [x] T8. **WebSocket/EventBus E2E Proof Test** (Phase B)

  **What to do**: Create `tests/test-ws-eventbus-proof.sh`:
  1. Connect to `/ws` via WSS, hold ≥30s
  2. Trigger action → verify event received
  3. Disconnect → reconnect → verify events still deliver
  Source transport.sh. Use ADMIN_TOKEN from env.

  **Recommended Agent**: `deep` | **Wave 3** | **Blocks**: T12, T13, T15, T16, T17 | **Blocked By**: T4, T5

  **References**: `tests/test-eventbus-streaming.sh`, `tests/lib/event_subscriber_helper.sh`

  **QA Scenarios**: WSS stable ≥30s + event received + reconnect works → `.sisyphus/evidence/pbcp-08/`

  **Commit**: YES — `test(ws): add WebSocket/EventBus E2E proof test`

- [x] T9. **Matrix Command Coverage Matrix** (Phase C)

  **What to do**: Create `tests/reports/matrix-command-coverage.md`:
  - Table: command | subsystem | expected effect | expected event | supported | tested
  - Rows: health, room join, send message, approval, studio.create_agent, studio.list_agents, secretary.list_templates, secretary.start_workflow, task.create
  - Based on actual RPC registry (`server.go:1219-1357`) and live probing

  **Recommended Agent**: `writing` | **Wave 3** | **Blocks**: T10, T11 | **Blocked By**: T7

  **References**: `bridge/pkg/rpc/server.go:1219-1357` (115+ methods), `tests/reports/matrix-status-contract.md`

  **QA Scenarios**: ≥9 rows, all columns filled → `.sisyphus/evidence/pbcp-09/`

  **Commit**: YES — `docs(matrix): create Matrix command coverage matrix`

- [x] T10. **Matrix Control-Flow Tests — Happy Paths** (Phase C)

  **What to do**: Create `tests/test-matrix-control-flow.sh`:
  1. Send message to Matrix room → verify bridge receives
  2. PII request + approve → verify status reflects approval
  3. Studio: create_agent → list_agents (count+1) → delete_agent → list_agents (count-1)
  4. Secretary: list_templates → get_template (if available)
  5. Event confirmation after each action via events.replay or WebSocket

  **Must NOT do**: Use ArmorChat. Add new RPC methods. Fall back to socket RPC.

  **Recommended Agent**: `deep` | **Wave 3** | **Blocks**: T14 | **Blocked By**: T6, T9

  **References**: `server.go:1265-1269` (Matrix), `server.go:1242-1250` (PII), `server.go:1273-1297` (Studio)

  **QA Scenarios**: ≥3/5 happy paths PASS → `.sisyphus/evidence/pbcp-10/`

  **Commit**: YES — `test(matrix): add control-flow happy-path tests`

- [x] T11. **Matrix Error-Path Tests** (Phase C)

  **What to do**: Create `tests/test-matrix-error-paths.sh`:
  1. Invalid token → verify 401/403
  2. Malformed JSON → verify parse error
  3. Non-existent method → verify "method not found"
  4. Missing agent ID → verify error
  5. Short timeout → verify timeout response
  6. Invalid room ID → verify error

  **Must NOT do**: Test security exploits. Send crash-inducing payloads.

  **Recommended Agent**: `unspecified-high` | **Wave 3** | **Blocks**: T14 | **Blocked By**: T9, T10

  **References**: `server.go:316` — `Handle()` error response format

  **QA Scenarios**: 6/6 error paths return correct errors → `.sisyphus/evidence/pbcp-11/`

  **Commit**: YES — `test(matrix): add error-path tests`

- [x] T12. **Studio Lifecycle Proof** (Phase D)

  **What to do**: Create `tests/test-studio-lifecycle-proof.sh`:
  1. create_agent → 2. get_agent (verify) → 3. list_agents (count≥1) → 4. spawn_agent → 5. list_instances → 6. stop_instance → 7. delete_agent → 8. list_agents (cleanup verified)
  Negative: get_agent with invalid ID → error. delete_agent with non-existent ID → error.
  Always clean up in teardown (even on failure).

  **Must NOT do**: Leave orphaned agents. Modify Studio handler.

  **Recommended Agent**: `deep` | **Wave 4** | **Blocks**: T14 | **Blocked By**: T8

  **References**: `server.go:1273-1297` — Studio methods all route through `handleStudio` with delegation gate

  **QA Scenarios**: 8/8 lifecycle PASS, 2/2 negative PASS → `.sisyphus/evidence/pbcp-12/`

  **Commit**: YES — `test(studio): add lifecycle proof test`

- [x] T13. **Secretary Lifecycle Proof (Probe-First)** (Phase D)

  **What to do**: Create `tests/test-secretary-lifecycle-proof.sh`:
  **Step 1 — Probe**: Call all 13 Secretary + 4 Task methods on VPS. Record which respond vs "method not found".
  **Step 2 — Test**: For responding methods, test lifecycle:
  - If templates work: create → get → delete → cleanup
  - If workflows work: start → get status → cancel → cleanup
  - If tasks work: create → list → cancel → verify
  **Step 3 — Document**: Write method availability matrix to evidence.

  **Must NOT do**: Assume methods work because registered. Leave orphaned workflows. Modify Secretary handler.

  **Recommended Agent**: `deep` | **Wave 4** | **Blocks**: T14 | **Blocked By**: T8

  **References**: `server.go:1314-1330` (Secretary), `server.go:1327-1330` (Task), `bridge/pkg/rpc/secretary_handlers.go`

  **QA Scenarios**: Method availability documented; lifecycle tested for available methods → `.sisyphus/evidence/pbcp-13/`

  **Commit**: YES — `test(secretary): add lifecycle proof test (probe-first)`

- [x] T14. **Matrix Command → Execution → Event Correlation** (Phase D)

  **What to do**: Create `tests/test-matrix-event-correlation.sh`:
  1. Start WebSocket event listener in background
  2. Call studio.create_agent → record agent_id
  3. Verify event listener received creation event with matching agent_id
  4. Call studio.delete_agent → cleanup
  5. Verify event listener received deletion event

  **Recommended Agent**: `deep` | **Wave 4** | **Blocks**: T19 | **Blocked By**: T10, T11, T12, T13

  **References**: `tests/lib/event_subscriber_helper.sh`, `server.go:1270-1271` (events.replay/stream)

  **QA Scenarios**: Creation+deletion events received with matching agent_ids → `.sisyphus/evidence/pbcp-14/`

  **Commit**: YES — `test(matrix): add command → event correlation test`

- [x] T15. **Restart Recovery Gate Test** (Phase E)

  **What to do**: Create `tests/test-restart-recovery-gate.sh`:
  1. Verify bridge healthy
  2. Start WS subscription
  3. `docker restart armorclaw`
  4. Poll /health every 1s until 200 OK (timeout 30s, target ≤10s)
  5. Verify matrix.status → logged_in + connected
  6. Verify RPC still works
  7. Verify WS reconnects or can be re-established

  **Must NOT do**: Restart Matrix Conduit. Use docker-compose.

  **Recommended Agent**: `unspecified-high` | **Wave 5** | **Blocks**: T18, T19 | **Blocked By**: T8

  **References**: `tests/lib/restart_bridge.sh`

  **QA Scenarios**: Recovery ≤10s, Matrix reconnects, RPC works → `.sisyphus/evidence/pbcp-15/`

  **Commit**: YES — `test(resilience): add restart recovery gate test`

- [x] T16. **WSS Reconnect Gate Test** (Phase E)

  **What to do**: Create `tests/test-wss-reconnect-gate.sh`:
  Connect → verify alive → force disconnect → reconnect → trigger action → verify event received

  **Must NOT do**: Restart bridge — only disconnect WebSocket client.

  **Recommended Agent**: `quick` | **Wave 5** | **Blocks**: T18, T19 | **Blocked By**: T8

  **QA Scenarios**: Event received after reconnect → `.sisyphus/evidence/pbcp-16/`

  **Commit**: YES — `test(ws): add WSS reconnect gate test`

- [x] T17. **Concurrency Smoke Test** (Phase E)

  **What to do**: Create `tests/test-concurrency-smoke.sh`:
  1. 10 concurrent `bridge.status` calls → all 200 OK
  2. 3 concurrent WSS subscribers → all receive event
  3. 20 rapid Matrix messages → bridge stays healthy

  Read-only/idempotent operations only. No create/delete.

  **Recommended Agent**: `deep` | **Wave 5** | **Blocks**: T18, T19 | **Blocked By**: T8

  **QA Scenarios**: 10/10 RPC succeed, 3/3 WS receive event, no crash → `.sisyphus/evidence/pbcp-17/`

  **Commit**: YES — `test(load): add concurrency smoke test`

- [x] T18. **Nightly Resilience CI Jobs** (Phase F)

  **What to do**: Add to `.github/workflows/test.yml`:
  - `schedule: cron: '0 3 * * *'` + `workflow_dispatch`
  - Job runs T15, T16, T17 on VPS (SSH via GitHub Secrets)
  - Upload results as artifacts

  **Must NOT do**: Store SSH keys in workflow. Run full 49-suite nightly. Modify existing jobs.

  **Recommended Agent**: `quick` | **Wave 6** | **Blocked By**: T3, T15

  **QA Scenarios**: Valid YAML, has schedule+dispatch, uses secrets → `.sisyphus/evidence/pbcp-18/`

  **Commit**: YES — `ci(tests): add nightly resilience jobs`

- [x] T19. **Final Baseline Rerun + Exit Criteria Verification** (Phase F)

  **What to do**:
  1. Re-verify VPS convergence (same as T2)
  2. Run full 49-script suite
  3. Compare against T1 baseline — show delta
  4. Check all 10 exit criteria with evidence citations
  5. Write `tests/reports/pre-beato-final-baseline.md`

  **Recommended Agent**: `deep` | **Wave 6** | **Blocks**: FINAL | **Blocked By**: T1, T14, T15, T16, T17

  **QA Scenarios**: PASS count ≥ T1 baseline; all 10 criteria have evidence → `.sisyphus/evidence/pbcp-19/`

  **Commit**: YES — `docs(reports): final Pre-BEATO baseline + exit criteria report`

---

## Consolidated Verification (after ALL implementation tasks)

After all 19 tasks are complete:

1. **Full suite rerun** from pushed repo state on VPS
2. **Check all 10 exit criteria** — for each criterion, cite the evidence file that proves it
3. **Present results** to user with pass/fail per criterion
4. **Get explicit user approval** before marking Pre-BEATO complete

Output: `tests/reports/pre-beato-final-baseline.md`

---

## Commit Strategy

| Wave | Commit Message | Files |
|------|---------------|-------|
| 1 | `ci(tests): add bash -n, go vet, go test guardrails` | `.github/workflows/test.yml` |
| 2a | `refactor(tests): migrate batch 1 (16 scripts) to shared transport` | 16 test scripts |
| 2b | `refactor(tests): migrate batch 2 (16 scripts) to shared transport` | 16 test scripts |
| 2c | `fix(tests): align 6 assertion mismatches to v4.6.0 behavior` | 6 test scripts |
| 2d | `docs(tests): document matrix.status contract and valid states` | `tests/reports/matrix-status-contract.md` |
| 3a | `test(ws): add WebSocket/EventBus E2E proof test` | `tests/test-ws-eventbus-proof.sh` |
| 3b | `docs(matrix): create Matrix command coverage matrix` | `tests/reports/matrix-command-coverage.md` |
| 3c | `test(matrix): add control-flow happy-path tests` | `tests/test-matrix-control-flow.sh` |
| 3d | `test(matrix): add error-path tests` | `tests/test-matrix-error-paths.sh` |
| 4a | `test(studio): add lifecycle proof test` | `tests/test-studio-lifecycle-proof.sh` |
| 4b | `test(secretary): add lifecycle proof test (probe-first)` | `tests/test-secretary-lifecycle-proof.sh` |
| 4c | `test(matrix): add command → event correlation test` | `tests/test-matrix-event-correlation.sh` |
| 5a | `test(resilience): add restart recovery gate test` | `tests/test-restart-recovery-gate.sh` |
| 5b | `test(ws): add WSS reconnect gate test` | `tests/test-wss-reconnect-gate.sh` |
| 5c | `test(load): add concurrency smoke test` | `tests/test-concurrency-smoke.sh` |
| 6a | `ci(tests): add nightly resilience jobs` | `.github/workflows/test.yml` |
| 6b | `docs(reports): final Pre-BEATO baseline + exit criteria report` | `tests/reports/pre-beato-final-baseline.md` |

---

## Success Criteria

### Pre-BEATO Exit Criteria (ALL must be TRUE)

1. [x] Full-suite baseline rerun published from pushed repo state
2. [x] Zero infrastructure-caused FAILs (feature-gated scripts excluded from denominator)
3. [x] Live WSS/EventBus proof passes (≥30s stable, event received, reconnect works)
4. [x] `matrix.status` contract documented and stable
5. [x] At least one Matrix-driven control slice passes end to end (command → action → event)
6. [x] Studio lifecycle statefully proven (create → get → spawn → list → stop → delete → cleanup)
7. [x] Secretary validation aligned to probed method availability; at least one slice proven if build supports it
8. [x] Restart recovery test passes (repeatable, scripted)
9. [x] WSS reconnect test passes (repeatable, scripted)
10. [x] CI guardrails exist: bash -n, go vet, go test on every push

### Verification Commands
```bash
# Syntax check all scripts
for f in tests/test-*.sh tests/lib/*.sh; do bash -n "$f"; done
# Expected: 0 errors

# Go critical packages
cd bridge && go vet ./pkg/rpc/... ./pkg/matrix/... ./pkg/security/... ./pkg/socket/... ./pkg/websocket/... ./internal/executor/...
cd bridge && go test -short ./pkg/rpc/... ./pkg/matrix/... ./pkg/security/... ./pkg/socket/... ./pkg/websocket/... ./internal/executor/...
# Expected: 0 issues, all PASS

# Matrix status
ssh_vps "curl -ksS https://localhost:8080/rpc -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"matrix.status\"}'"
# Expected: logged_in: true, connected: true

# Transport state (both modes valid)
ssh_vps "source /opt/armorclaw/tests/lib/transport.sh && detect_transport"
# Expected: mode=http OR mode=both (both are valid for --network host)

# Studio lifecycle
ssh_vps "bash /opt/armorclaw/tests/test-studio-lifecycle-proof.sh"
# Expected: PASS

# Restart recovery
ssh_vps "bash /opt/armorclaw/tests/test-restart-recovery-gate.sh"
# Expected: PASS
```
