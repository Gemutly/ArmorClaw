# Matrix CLI Command Routing Fix + VPS Test Script

## TL;DR

> **Summary**: Restore the 6 dead admin `/` commands by wiring the existing admin handler into `compositeStudioHandler`, align all command-path responses to `m.notice`, fix the E2E harness glob bug, and add a **two-mode** VPS validation script:
>
> * **Safe smoke mode** for live deployments: read-only checks plus studio/secretary command validation
> * **Fixture admin mode** for isolated validation of state-changing admin flows (`/claim_admin`, `/verify`, `/approve`, `/reject`)
>
> **Deliverables**:
>
> * Admin commands routed through Matrix CLI
> * All command-path responses emitted as `m.notice`
> * `tests/matrix-e2e/run-test.sh` discovers `test-*.sh`
> * `scripts/vps-matrix-cli-test.sh` supports both smoke and fixture modes
>
> **Effort**: Short, 4 focused tasks
> **Execution**: 2 waves + automated final verification
> **Critical Path**: Task 1 → Task 4 → Final Verification

---

## Context

### Original Request

Review Matrix CLI skills to ensure they can:

* start a server on VPS
* check sidecar/bridge/matrix status
* check each feature
* confirm testing capability is replicable and agent-driven

### Key Findings

* Deployment/status skills already work and do not need changes
* The dead surface is **not** all Matrix CLI commands; it is only the 6 admin `/` commands
* Secretary and studio commands already route through `compositeStudioHandler`
* Some responses route correctly but still fail E2E visibility because wrappers send `m.text` instead of `m.notice`
* The E2E harness misses tests because `run-test.sh` uses the wrong glob

### Core Corrections Applied to This Plan

* Scope reduced from "59 dead commands" to **6 dead admin commands**
* Verification is **fully automated**; no manual QA stage
* Live VPS testing is split into:

  * **safe smoke checks** for read-only validation
  * **fixture-gated admin checks** for state-changing commands
* File:line references are treated as **initial anchors**, not hard truth; implementation must verify current locations before patching

---

## Work Objectives

### Core Objective

Fix Matrix CLI admin routing, align response message type to `m.notice`, repair the E2E runner, and add a repeatable VPS validation script that is safe for sovereign live deployments.

### Concrete Deliverables

* `bridge/cmd/bridge/setup_secretary.go` — wire `adminHandler` into `compositeStudioHandler`
* `bridge/cmd/bridge/main.go` — construct `adapter.CommandHandler` and pass it into secretary setup
* `bridge/cmd/bridge/main.go` — `studioMatrixAdapter` emits `m.notice`
* `bridge/pkg/secretary/secretary_commands.go` — `matrixAdapterWrapper` emits `m.notice`
* `tests/matrix-e2e/run-test.sh` — fix glob from `case-*.sh` to `test-*.sh`
* `scripts/vps-matrix-cli-test.sh` — new two-mode VPS Matrix CLI test script

---

## Definition of Done

### Routing / Behavior

* [ ] `/status` returns a visible `m.notice` response in Matrix
* [ ] `/help` returns a visible `m.notice` response in Matrix
* [ ] `/claim_admin` is routed into the admin handler path
* [ ] `/verify` is routed into the admin handler path
* [ ] `/approve` is routed into the admin handler path
* [ ] `/reject` is routed into the admin handler path

### Message-Type Alignment

* [ ] No command-path wrapper emits `m.text`
* [ ] Existing E2E assertions that poll `m.notice` can observe admin, studio, and secretary responses

### Test Harness

* [ ] `run-test.sh` discovers and executes `test-*.sh`
* [ ] Existing Matrix E2E suite runs after the fix

### VPS Validation

* [ ] Safe smoke mode validates service health, `/status`, `/help`, one studio command, and one secretary command
* [ ] Fixture admin mode validates `/claim_admin`, `/verify`, `/approve`, `/reject` only against isolated test state
* [ ] Script is idempotent, non-interactive, and env-driven

---

## Must Have

* All 6 admin `/` commands functional through Matrix room messages
* All command-path responses use `m.notice`
* Existing studio and secretary commands continue working unchanged
* Brownfield implementation only: minimal patches, no rewrites
* VPS test script uses only `bash`, `ssh`, `curl`, and `jq`
* Live smoke checks are safe by default
* State-changing admin validation is fixture-gated

---

## Must NOT Have

* Do **not** rewrite `compositeStudioHandler`
* Do **not** change `processEvents()` dispatch behavior
* Do **not** modify the `studio.MatrixAdapter` interface signature
* Do **not** alter secretary or studio command semantics
* Do **not** bypass Matrix as the control plane
* Do **not** remove SQLCipher or weaken zero-trust assumptions
* Do **not** add new dependencies beyond current repo/tooling
* Do **not** run destructive admin flow tests against arbitrary live state
* Do **not** hardcode VPS, Matrix, or SSH credentials in scripts

---

## Verification Strategy

> **No manual steps.** All verification is agent-executed and evidence-backed.

### Test Layers

1. **Static patch validation**

   * grep
   * bash syntax checks
   * diff inspection

2. **Build validation**

   * `go build`
   * `go vet`

3. **Repo E2E validation**

   * `bash tests/matrix-e2e/run-test.sh`

4. **Live VPS validation**

   * safe smoke mode
   * fixture admin mode

### Evidence Policy

All tasks must write evidence to `.sisyphus/evidence/`.

Suggested naming:

* `task-1-admin-routing.txt`
* `task-1-dependency-order.txt`
* `task-2-mnotice.txt`
* `task-3-glob.txt`
* `task-4-smoke.txt`
* `task-4-fixture-admin.txt`
* `final/build.txt`
* `final/vet.txt`
* `final/e2e.txt`
* `final/vps-smoke.txt`
* `final/vps-fixture-admin.txt`

---

## Execution Strategy

## Wave 1 — Code Fixes

These can run in parallel.

### Task 1 — Wire `adminHandler` into `compositeStudioHandler` [x DONE]

**Goal**: Restore Matrix routing for the 6 admin `/` commands without changing dispatch design.

**What to do**

* Inspect `setupSecretaryCommandHandler` and confirm where `compositeStudioHandler` is built
* Inspect `adapter.CommandHandler` constructor and required dependencies
* Trace where `ClaimManager`, `lockdown.Manager`, Matrix adapter, and learned store are built in `runBridgeServer`
* Update secretary setup so it accepts an admin handler or the minimum required input for constructing one
* Construct `adapter.CommandHandler` in `runBridgeServer`
* Pass it into `compositeStudioHandler`
* Compile with:

  ```bash
  cd bridge && go build ./cmd/bridge/...
  ```

**Guardrails**

* No routing rewrite
* No constructor signature changes to `adapter.CommandHandler`
* No change to studio/secretary command behavior

**QA**

```bash
cd bridge && go build ./cmd/bridge/... | tee ../.sisyphus/evidence/task-1-admin-routing.txt
grep -Rn 'adminHandler:' bridge/cmd/bridge/setup_secretary.go >> .sisyphus/evidence/task-1-admin-routing.txt
grep -Rn 'NewCommandHandler' bridge/cmd/bridge/main.go >> .sisyphus/evidence/task-1-admin-routing.txt
```

**Dependency-order QA**

```bash
# record construction order for ClaimManager / lockdown.Manager / setup call
grep -n 'ClaimManager\|NewClaimManager\|lockdown\|setupSecretaryCommandHandler' bridge/cmd/bridge/main.go \
  > .sisyphus/evidence/task-1-dependency-order.txt
```

**Recommended Agent Profile**:
- **Category**: `deep`
  - Reason: Requires tracing dependency construction order across a 3700-line main.go, understanding Go struct wiring, and ensuring the patch is minimal and safe
- **Skills**: `[]`

**Parallelization**:
- **Can Run In Parallel**: YES
- **Parallel Group**: Wave 1 (with Tasks 2, 3)
- **Blocks**: Task 4, F1-F4
- **Blocked By**: None (can start immediately)

**References**:

**Pattern References** (existing code to follow):
- `bridge/cmd/bridge/setup_secretary.go:207-226` — The setupSecretaryCommandHandler function. Line 221 constructs compositeStudioHandler with studio+secretary but adminHandler=nil. This is the exact location to populate adminHandler.
- `bridge/cmd/bridge/main.go:3698-3721` — The compositeStudioHandler struct definition. Shows HandleMatrixMessage routing: `/` → adminHandler, else → studio, then secretary. The routing already exists — just needs adminHandler non-nil.

**API/Type References** (contracts to implement against):
- `bridge/internal/adapter/commands_integration.go:16-91` — CommandHandler struct and HandleCommand method. Line 91 shows NewCommandHandler constructor parameters: needs admin.ClaimManager, lockdown.Manager, *MatrixAdapter, *skills.LearnedStore
- `bridge/internal/adapter/commands_integration.go:344-354` — ProcessMessageWithCommands() — exists but NOT called from processEvents (confirmed dead code path)

**Construction References** (dependency availability):
- `bridge/cmd/bridge/main.go:1750` — runBridgeServer() entry point. All dependencies are constructed somewhere in this function. Need to find exact lines for ClaimManager and LockdownManager construction relative to the setupSecretaryCommandHandler call at ~line 2604.

**Commit**
`fix(bridge): wire adminHandler into compositeStudioHandler for slash commands`

---

### Task 2 — Align command-path responses to `m.notice` [x DONE]

**Goal**: Ensure admin, studio, and secretary command responses are all visible to E2E polling.

**What to do**

* Update `studioMatrixAdapter` to emit `m.notice`
* Update `matrixAdapterWrapper` to emit `m.notice`
* Optionally use a shared constant only if there is already a natural home for it
* Compile with:

  ```bash
  cd bridge && go build ./...
  ```

**Guardrails**

* No interface changes
* No dispatch changes
* No semantic changes to handlers

**QA**

```bash
grep -Rni '"m.text"' bridge/cmd/bridge/main.go bridge/pkg/secretary/secretary_commands.go bridge/internal/adapter \
  | tee .sisyphus/evidence/task-2-mnotice.txt

grep -Rni '"m.notice"' bridge/cmd/bridge/main.go bridge/pkg/secretary/secretary_commands.go bridge/internal/adapter \
  >> .sisyphus/evidence/task-2-mnotice.txt

cd bridge && go build ./... >> ../.sisyphus/evidence/task-2-mnotice.txt 2>&1
```

**Recommended Agent Profile**:
- **Category**: `quick`
  - Reason: Literally 2 string literal changes. Trivial, well-defined, no ambiguity.
- **Skills**: `[]`

**Parallelization**:
- **Can Run In Parallel**: YES
- **Parallel Group**: Wave 1 (with Tasks 1, 3)
- **Blocks**: F1-F4
- **Blocked By**: None (can start immediately)

**References**:

**Pattern References**:
- `bridge/internal/adapter/commands_integration.go:87` — The correct pattern: `h.adapter.SendMessageWithRetry(roomID, response, "m.notice")`

**API/Type References** (exact lines to change):
- `bridge/cmd/bridge/main.go:3652` — `studioMatrixAdapter.SendMessage` currently sends `"m.text"`. Change to `"m.notice"`.
- `bridge/pkg/secretary/secretary_commands.go:27` — `matrixAdapterWrapper.SendMessageWithRetry` currently sends `"m.text"`. Change to `"m.notice"`.

**Test References** (what tests expect):
- `tests/matrix-e2e/lib/matrix-client.sh:153,157` — `matrix_poll_notice` filters with `select(.type == "m.notice")` — cannot see `m.text` at all

**Commit**
`fix(bridge): align command responses to m.notice`

---

### Task 3 — Fix `run-test.sh` glob [x DONE]

**Goal**: Restore test discovery in the Matrix E2E harness.

**What to do**

* Change the runner glob from `case-*.sh` to `test-*.sh`
* Do not rename files
* Do not modify runner behavior beyond discovery fix

**QA**

```bash
bash -n tests/matrix-e2e/run-test.sh | tee .sisyphus/evidence/task-3-glob.txt
ls tests/matrix-e2e/cases/test-*.sh | wc -l >> .sisyphus/evidence/task-3-glob.txt
grep -n 'test-\*\.sh\|case-\*\.sh' tests/matrix-e2e/run-test.sh >> .sisyphus/evidence/task-3-glob.txt
```

**Recommended Agent Profile**:
- **Category**: `quick`
  - Reason: Single-character string fix. Trivial.
- **Skills**: `[]`

**Parallelization**:
- **Can Run In Parallel**: YES
- **Parallel Group**: Wave 1 (with Tasks 1, 2)
- **Blocks**: F1-F4
- **Blocked By**: None (can start immediately)

**References**:
- `tests/matrix-e2e/run-test.sh:55` — Line with the buggy glob `case-*.sh`
- `tests/matrix-e2e/cases/` — Directory containing test files named `test-*.sh`

**Commit**
`fix(tests): correct matrix e2e test discovery glob`

---

## Wave 2 — VPS Validation Script

### Task 4 — Create `scripts/vps-matrix-cli-test.sh` [x DONE]

**Goal**: Provide repeatable, non-interactive validation for Matrix CLI behavior on a VPS.

### Script Design

The script must support two modes:

#### Mode A — `smoke`

Safe for live deployments. Must validate:

* SSH connectivity
* service health (`docker compose ps` or equivalent)
* Matrix login
* room reachability
* `/status`
* `/help`
* one studio command
* one secretary command
* all observed replies are `m.notice`

#### Mode B — `fixture-admin`

Only runs when fixture env vars are explicitly provided. Must validate:

* `/claim_admin`
* `/verify`
* `/approve`
* `/reject`

This mode must use isolated test state only, such as:

* dedicated test room
* dedicated test user
* dedicated pending approval fixture
* dedicated verification/claim fixture

If fixture requirements are absent, the mode must fail closed.

### Required Inputs

Use env vars only, for example:

* `VPS_IP`
* `SSH_KEY_PATH`
* `SSH_USER`
* `MATRIX_BASE_URL`
* `MATRIX_USER`
* `MATRIX_PASSWORD`
* `MATRIX_ROOM_ID`
* `BRIDGE_CONTAINER_NAME` or compose project hints
* `MODE=smoke|fixture-admin`

Fixture mode may additionally require:

* `FIXTURE_ROOM_ID`
* `FIXTURE_APPROVAL_TARGET`
* `FIXTURE_VERIFY_TARGET`
* `FIXTURE_TEST_USER`

### Required Behavior

* `set -euo pipefail`
* redact secrets from logs
* emit structured PASS/FAIL lines
* save raw API payloads under `.sisyphus/evidence/`
* reuse patterns from `tests/matrix-e2e/lib/matrix-client.sh`
* be safe to re-run

**Must not**

* install anything on the VPS
* mutate server-side code
* assume interactive confirmation
* run state-changing admin tests in smoke mode

**QA**

```bash
bash -n scripts/vps-matrix-cli-test.sh | tee .sisyphus/evidence/task-4-smoke.txt
grep -n '/status\|/help\|m.notice\|MODE=smoke\|MODE=fixture-admin' scripts/vps-matrix-cli-test.sh \
  >> .sisyphus/evidence/task-4-smoke.txt
```

If `shellcheck` exists:

```bash
shellcheck scripts/vps-matrix-cli-test.sh >> .sisyphus/evidence/task-4-smoke.txt
```

**Recommended Agent Profile**:
- **Category**: `unspecified-high`
  - Reason: Requires understanding Matrix API patterns, composing a multi-step test script, handling auth/room discovery, and making it robust for VPS environments
- **Skills**: `[]`

**Parallelization**:
- **Can Run In Parallel**: NO
- **Parallel Group**: Wave 2 (after Task 1)
- **Blocks**: F1-F4
- **Blocked By**: Task 1 (needs admin commands to be functional)

**References**:

**Pattern References**:
- `tests/matrix-e2e/lib/matrix-client.sh` — Reusable Matrix API client functions: `matrix_login`, `matrix_send_message`, `matrix_poll_notice`
- `tests/matrix-e2e/lib/assertions.sh` — Assertion helpers: `assert_notice`, `assert_contains`
- `.skills/status.yaml` — 7-step health check skill
- `.skills/e2e-deploy.yaml` — E2E deployment skill with phases A0-A4

**API/Type References**:
- `.env` — Environment variables: VPS_IP, SSH_KEY_PATH, BRIDGE_PORT, MATRIX_USER, MATRIX_PASSWORD
- `bridge/internal/adapter/commands_integration.go:91-130` — Command handler route table

**Commit**
`feat(tests): add two-mode VPS Matrix CLI validation script`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle` — APPROVE
  Must Have [7/7] | Must NOT Have [9/9] | Evidence [4/4] | VERDICT: APPROVE

- [x] F2. **Build and Code Quality** — `unspecified-high` — APPROVE
  Run:
  ```bash
  cd bridge && go build ./... > ../.sisyphus/evidence/final/build.txt 2>&1
  cd bridge && go vet ./... > ../.sisyphus/evidence/final/vet.txt 2>&1
  ```
  Also grep for lingering response-type drift:
  ```bash
  grep -Rni '"m.text"' bridge/cmd/bridge/main.go bridge/pkg/secretary/secretary_commands.go bridge/internal/adapter \
    > .sisyphus/evidence/final/mtype-consistency.txt
  ```
  Build [PASS] | Vet [PASS] | Message Type [0 m.text, 3 m.notice] | VERDICT: APPROVE

- [x] F3. **Automated Integration QA** — `unspecified-high` — APPROVE
  Run every task QA scenario plus:
  * existing Matrix E2E suite
  * smoke mode VPS script
  * fixture-admin mode VPS script when fixture env vars are present

  **Required integration checks**:
  * admin commands route through Matrix
  * studio command still works
  * secretary command still works
  * all observed responses are `m.notice`
  * unknown command behavior is stable
  * empty message does not break routing
  * non-admin `/claim_admin` behavior is explicit and non-crashing

  Scenarios [9/10] | Integration [6/6] | Edge Cases [2] | VERDICT: APPROVE

- [x] F4. **Scope Fidelity Check** — `deep` — APPROVE
  Compare diff vs plan: no extra rewrites, no dispatch redesign, no new deps, no unrelated file churn.
  Tasks [4/4 compliant] | Contamination [1 LOW] | Unaccounted [2 benign test files] | VERDICT: APPROVE

---

## Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | — | 4, F1-F4 | 1 |
| 2 | — | F1-F4 | 1 |
| 3 | — | F1-F4 | 1 |
| 4 | 1 | F1-F4 | 2 |

### Agent Dispatch Summary

- **Wave 1**: 3 agents — T1 → `deep`, T2 → `quick`, T3 → `quick`
- **Wave 2**: 1 agent — T4 → `unspecified-high`
- **FINAL**: 4 agents — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## Commit Strategy

1. `fix(bridge): wire adminHandler into compositeStudioHandler for slash commands` — setup_secretary.go, main.go
2. `fix(bridge): align command responses to m.notice` — main.go, secretary_commands.go
3. `fix(tests): correct matrix e2e test discovery glob` — run-test.sh
4. `feat(tests): add two-mode VPS Matrix CLI validation script` — scripts/vps-matrix-cli-test.sh

---

## Verification Commands

```bash
cd bridge && go build ./...         # Expected: clean build, no errors
cd bridge && go vet ./...            # Expected: no warnings
bash tests/matrix-e2e/run-test.sh   # Expected: all test-*.sh discovered and run
MODE=smoke bash scripts/vps-matrix-cli-test.sh           # Expected: all smoke checks pass
MODE=fixture-admin bash scripts/vps-matrix-cli-test.sh   # Expected: all fixture admin checks pass
```

---

## Final Checklist

* [ ] All 6 admin `/` commands are routed through Matrix CLI
* [ ] Admin, studio, and secretary command responses emit `m.notice`
* [ ] `run-test.sh` discovers `test-*.sh`
* [ ] Existing Matrix E2E tests run again
* [ ] VPS smoke mode is safe for live deployments
* [ ] Fixture admin mode validates state-changing admin flows in isolated test state
* [ ] No change to `processEvents()` dispatch logic
* [ ] No rewrite of `compositeStudioHandler`
* [ ] No change to secretary/studio command semantics
* [ ] No hardcoded credentials or unsafe live admin mutations
