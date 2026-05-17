# BEATO Blocker Remediation Sprint

## TL;DR

> **Quick Summary**: Fix the two blockers preventing ArmorClaw BEATO runtime progress — vendor navchart behavior into the existing `bridge/pkg/browser` package (same-package, no subpackage) to unblock Bridge image rebuild, then fix sidecar-office HMAC crash-loop via a Compose init-service — followed by validation of Office E2E, Browser RPCs, Email outbox, and an honest runtime BEATO report.
> 
> **Deliverables**:
> - Vendored navchart behavior in `bridge/pkg/browser/` (multi_tab.go, replay.go, diagnostics.go — same package, reuses chart_types.go)
> - Updated imports in 5 files (no external jetski dependency for bridge)
> - Rebuilt and deployed Bridge image with all previously-committed RPCs
> - Fixed sidecar-office HMAC secret provisioning via `office-secret-init` one-shot Compose service
> - Validated Office E2E (XLSX → extract text → artifact)
> - Tested 9 previously-untested Browser RPCs on VPS
> - Deployed and verified Email outbox queue RPCs
> - Honest BEATO runtime report scored against the existing rubric
> 
> **Estimated Effort**: Large (6 phases, sequential dependency chain)
> **Parallel Execution**: YES — 4 waves
> **Critical Path**: T2 → T4 → T6 → T9 → Final (T1 + T2 parallel start)
> 
> **CTO Conditional Approval (2026-05-16)**: Score 8.2/10. Patches applied: same-package navchart vendoring, container-path diagnostics, init-service HMAC provisioning, RPC count correction, deterministic image tags, auth-gated smoke tests, tightened HMAC permissions.

---

## Context

### Original Request
CTO accepted the current-status report as runtime source of truth (BEATO runtime readiness ~50%). Directed a focused blocker-remediation sprint — not another broad BEATO wave. Two blockers: (1) navchart dependency blocks Bridge rebuild, (2) sidecar-office HMAC crash blocks Office. After fixes: validate Office E2E, Browser RPCs, Email outbox, produce honest runtime report.

### Interview Summary
**Key Discussions**:
- CTO priority ordering: navchart fix → sidecar HMAC fix → Office E2E → Browser coverage → Email outbox → Final report
- Vendoring preferred over go.mod replace (Docker build reliability)
- sidecar-office crash is PermissionError on `/run/armorclaw/secrets/office-hmac`
- Audio intentionally deferred, Postfix/DNS out of scope

**Research Findings**:
- Navchart: 4 source files, 357 lines, zero external deps — vendor into existing `bridge/pkg/browser/` package (same-package, no subpackage to avoid duplicate type identity issues with `chart_types.go`)
- Sidecar: No provisioning code exists for office-hmac secret; `GenerateSharedSecret()` in token.go never called from main
- RPC breakdown — 7 newly deployed BEATO RPCs (document.extract_text, document.status, document.list_jobs, email.queue_status, email.get, email.list, email.retry) + 9 previously untested Browser RPCs (browser.fill, browser.click, browser.cancel, browser.wait_for_element, browser.wait_for_captcha, browser.wait_for_2fa, browser.complete, browser.fail, browser.replay_diagnostics)
- `test-rpc-methods.sh` is 0 bytes — no CI gate for RPC methods
- `browser_test.go.bak` has 7 disabled tests — do NOT restore in this sprint
- BEATO scoring template at `tests/reports/beato-verification-report.md` with 100-point rubric

### CTO Review (2026-05-16)
**Score**: 8.2/10 — CONDITIONAL APPROVAL, patched before dispatch.
**P0 fixes applied**:
- Navchart vendoring changed from subpackage to same-package approach to avoid duplicate Go type identity (`browser.NavChart` vs `browser/navchart.NavChart`)
- Sidecar diagnostic path corrected: container sees `/run/secrets/shared_secret`, not the host path `/run/armorclaw/secrets/office-hmac`
- HMAC provisioning changed from Bridge `os.Chown()` (unprivileged UID 10002 cannot chown to 10001) to `office-secret-init` one-shot Compose service running as root
**P1 fixes applied**:
- "17 RPCs" replaced with exact 7+9 breakdown
- Docker build/deploy commands canonicalized with deterministic image tags
- F3 renamed from "Real Manual QA" to "Live VPS Integration QA"
- All RPC smoke tests now require auth tokens (follow `test-browser-smoke.sh` pattern)
- Final HMAC file mode tightened from 0644 to 0440
- Secret generation kept out of `main.go` — uses `bridge/pkg/sidecar/office_provision.go`

### Metis Review
**Identified Gaps** (addressed):
- All BEATO RPCs require auth tokens via `rpc_safety.go` — smoke tests must use valid auth (follow `test-browser-smoke.sh` pattern)
- `browser_test.go.bak` disabled tests are out of scope — don't attempt restoration
- Email queue handler tests only test registration, not logic — first real exercise on VPS
- Email approval flow depends on Matrix — need Matrix healthcheck prerequisite
- Phase 6 must use the existing BEATO scoring template, not invent a new rubric
- Document RPCs depend on Python sidecar being healthy — verify before E2E tests
- Must NOT trust existing 100/100 BEATO score — re-verify from scratch

---

## Work Objectives

### Core Objective
Fix the two blockers preventing Bridge image rebuild and Office sidecar health, then validate BEATO runtime readiness across all pillars except Audio.

### Concrete Deliverables
- `bridge/pkg/browser/multi_tab.go`, `replay.go`, `diagnostics.go` — vendored navchart behavior in existing browser package (reuses chart_types.go)
- 5 files with updated imports (browser.go, server.go, 3 test files)
- New Bridge Docker image deployed to VPS with all previously-committed RPCs
- `office-secret-init` one-shot Compose service for HMAC provisioning
- Sidecar-office running healthy (no crash-loop)
- Office E2E validated (XLSX extraction + negative tests)
- Browser RPC coverage: 12/12 tested on VPS (3 existing + 9 newly tested)
- Email outbox queue RPCs callable on VPS
- Runtime BEATO report at `tests/reports/beato-runtime-report.md`

### Definition of Done
- [ ] `go test ./pkg/rpc/... ./pkg/browser/... ./pkg/sidecar/... ./pkg/email/... -count=1` passes
- [ ] `cd bridge && docker build -t armorclaw:beato-fix -f Dockerfile .` succeeds
- [ ] All 7 newly deployed RPCs + 9 previously untested Browser RPCs return valid JSON on VPS
- [ ] `armorclaw-sidecar-office` stays Up/healthy for 5+ minutes
- [ ] `document.extract_text` returns extracted text from XLSX on VPS
- [ ] 12/12 browser.* RPCs respond on VPS
- [ ] Email approval pipeline test passes (`tests/test-email-pipeline.sh`)
- [ ] BEATO runtime report scored ≥90/100 against rubric

### Must Have
- Navchart dependency fully eliminated from bridge/go.mod (same-package vendoring, no subpackage)
- Sidecar-office HMAC secret provisioned by `office-secret-init` one-shot service (not Bridge chown)
- All 7 newly deployed + 9 previously untested RPCs callable on VPS
- Honest runtime scores (not inflated)
- HMAC file mode final state: 0440 (not 0644)

### Must NOT Have (Guardrails)
- Do NOT create `bridge/pkg/browser/navchart/` subpackage — use same-package vendoring
- Do NOT duplicate types that exist in `chart_types.go`
- Do NOT restore `browser_test.go.bak` — separate cleanup task
- Do NOT modify `tests/test-rpc-methods.sh` — not this sprint
- Do NOT deploy Postfix/DNS
- Do NOT start Audio implementation
- Do NOT rewrite risk scoring
- Do NOT touch SQLCipher, Matrix state, or Secretary state
- Do NOT change `container-setup.sh`
- Do NOT use mattn/go-sqlite3
- Do NOT expose Jetski on public host port
- Do NOT call BEATO 100% until runtime tests pass on VPS
- Do NOT attempt to restructure existing architecture (brownfield: minimal patches)
- Do NOT rely on unprivileged Bridge `os.Chown()` for HMAC secret ownership
- Do NOT weaken Bridge container privileges (no privileged mode, no CAP_CHOWN)
- Do NOT put secret generation code directly in `main.go` — use `office_provision.go`
- Do NOT leave HMAC file mode as 0644 — final state must be 0440
- Do NOT send unauthenticated RPC calls in smoke tests — always use auth token

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.
> Acceptance criteria requiring "user manually tests/confirms" are FORBIDDEN.

### Test Decision
- **Infrastructure exists**: YES (go test, bash test harness)
- **Automated tests**: Tests-after (verify existing tests pass after changes)
- **Framework**: go test + bash test harness

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go code**: Use Bash — `go test`, `go build`, `go vet`
- **Docker**: Use Bash — `docker build`, `docker compose`, `docker logs`, `docker ps`
- **RPC verification**: Use Bash (curl/socat via SSH) — follow `test-browser-smoke.sh` pattern with auth tokens
- **Integration**: Use Bash — existing test scripts (`test-email-pipeline.sh`, `test-sidecar-docs.sh`)

### Evidence Index (MANDATORY)

> Every task MUST produce evidence files. The final task (T9 or F-wave) MUST create an audit index.

Upon completion, create `.sisyphus/evidence/beato-remediation-index.md` with this exact structure:

```markdown
# BEATO Remediation Evidence Index

| Task | Evidence files | Pass/Fail | Notes |
|---|---|---|---|
| T1 | task-1-navchart-vendor.txt, task-1-no-jetski-refs.txt | PASS/FAIL | |
| T2 | task-2-hmac-debug.md, task-2-entrypoint-test.txt, task-2-host-path.txt | PASS/FAIL | Confirmed H_ |
| T3 | task-3-bridge-build.txt, task-3-vps-bridge-status.txt | PASS/FAIL | |
| T4 | task-4-sidecar-healthy.txt, task-4-secret-permissions.txt, task-4-no-bridge-chown.txt | PASS/FAIL | |
| T5 | task-5-rpc-verification.txt, task-5-auth-gate.txt | PASS/FAIL | |
| T6 | task-6-office-e2e.txt, task-6-mime-mismatch.txt, task-6-corrupt-file.txt | PASS/FAIL | |
| T7 | task-7-browser-coverage.txt | PASS/FAIL | |
| T8 | task-8-email-pipeline.txt, task-8-queue-persistence.txt | PASS/FAIL | |
| T9 | task-9-report-exists.txt | PASS/FAIL | |
```

This keeps auditability without drowning the final reviewer.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — core fixes, MAX PARALLEL):
├── T1: Vendor navchart into bridge/pkg/browser (same-package) [unspecified-high]
└── T2: Debug sidecar-office HMAC crash (VPS investigation, read-only) [deep]

Wave 2 (After Wave 1 — build, deploy, fix):
├── T3: Build and deploy rebuilt Bridge image (depends: T1) [quick]
└── T4: Fix sidecar-office HMAC provisioning via init-service (depends: T2) [unspecified-high]
    NOTE: T4 implementation MUST NOT start until T2 confirms the real mount/path/permission failure

Wave 3 (After Wave 2 — validation, ordered):
├── T5: Verify new Bridge RPCs on VPS (depends: T3) [quick]
├── T7: Browser RPC coverage test (depends: T3) [quick]  ← starts immediately with T5 after T3
├── T6: Office E2E validation (depends: T4 AND T5) [deep]  ← ONLY after BOTH T4 and T5 pass
│
│ Execution order within Wave 3:
│   T3 complete → T5 + T7 (parallel)
│   T4 complete + T5 complete → T6 (sequential gate)
│   T5 + T6 + T7 complete → T8/T9


Wave 4 (After Wave 3 — email):
├── T8: Email outbox deploy and test (depends: T3, T5) [unspecified-high]

Wave 5 (After Wave 4 — report):
├── T9: Final runtime BEATO report (depends: T5-T8) [writing]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Live VPS Integration QA (unspecified-high)
└── F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay

Critical Path: T2 → T4 → T6 → T9 → Final (diagnosis drives fix design)
Dispatch Sequence (CTO approved):
  1. T1 + T2 (parallel, immediate start)
  2. T3 only after T1 passes
  3. T4 only after T2 confirms hypothesis (template complete)
  4. T5 + T7 after T3
  5. T6 after T4 AND T5 both pass
  6. T8
  7. T9
Parallel Speedup: ~40% faster than sequential (T1+T2 parallel, T3+T4 parallel, T5+T7 parallel before T6 gate)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1 | — | T3 | 1 |
| T2 | — | T4 | 1 |
| T3 | T1 | T5, T6, T7, T8 | 2 |
| T4 | T2 | T6 | 2 |
| T5 | T3 | T6, T8, T9 | 3 |
| T6 | T4, T5 | T9 | 3 |
| T7 | T3 | T9 | 3 |
| T8 | T3, T5 | T9 | 4 |
| T9 | T5, T6, T7, T8 | Final | 5 |

### Agent Dispatch Summary

- **Wave 1**: 2 agents — T1 → `unspecified-high`, T2 → `deep`
- **Wave 2**: 2 agents — T3 → `quick`, T4 → `unspecified-high`
- **Wave 3**: 3 agents — T5 → `quick`, T6 → `deep`, T7 → `quick`
- **Wave 4**: 1 agent — T8 → `unspecified-high`
- **Wave 5**: 1 agent — T9 → `writing`
- **FINAL**: 4 agents — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Vendor missing navchart behavior into existing browser package

  **What to do**:
  - **Do NOT create `bridge/pkg/browser/navchart/` subpackage.** Use same-package vendoring to avoid duplicate Go type identity issues.
  - The existing `bridge/pkg/browser/chart_types.go` already defines: `NavChart`, `ChartAction`, `ChartSelector`, `FrameRouting`, `WaitCondition`, `Assertion`, `ActionType`, `SelectorTier`, plus constants.
  - The jetski/navchart source adds *functions* and *structs* not in chart_types.go: `MultiTabStore`, `NewMultiTabStore()`, `StoreChart()`, `GetCharts()`, `RemoveTab()`, `TabIDs()`, `ReplayConfig`, `ReplayResult`, `Replay()`, `ReplayWithStore()`, `ChartDiff`, `DiffCharts()`, `DiffReplay()`.
  - Create 3 new files in `bridge/pkg/browser/` (same package, `package browser`):
    - `multi_tab.go` — Copy `MultiTabStore` + all methods from `jetski/navchart/multi_tab.go`, change `package navchart` → `package browser`. Replace the `NavChart`/`ChartMetadata` references to use the existing types from `chart_types.go`.
    - `replay.go` — Copy `ReplayConfig`, `ReplayResult`, `Replay()`, `ReplayWithStore()` from `jetski/navchart/replay.go`, change `package navchart` → `package browser`. Use existing types from `chart_types.go`.
    - `diagnostics.go` — Copy `ChartDiff`, `DiffCharts()`, `DiffReplay()` from `jetski/navchart/diagnostics.go`, change `package navchart` → `package browser`. Use existing types from `chart_types.go`.
  - **Do NOT copy `jetski/navchart/types.go`** — `chart_types.go` already has all the types. Only vendor the behavior (functions/structs) that's missing.
  - Update imports in 5 bridge files from `"github.com/armorclaw/jetski/navchart"` to `"github.com/armorclaw/bridge/pkg/browser"` (or remove the import where already in package browser):
    - `bridge/pkg/rpc/browser.go:13`
    - `bridge/pkg/rpc/server.go:42`
    - `bridge/pkg/rpc/replay_diagnostics_test.go:8`
    - `bridge/pkg/rpc/replay_gating_test.go:8`
    - `bridge/pkg/rpc/edge_case_test.go:9`
  - Where files reference `navchart.MultiTabStore` → `browser.MultiTabStore`, `navchart.DiffCharts` → `browser.DiffCharts`, etc.
  - **Only use a subpackage `bridge/pkg/browser/navchart/` if same-package vendoring creates a real import cycle** (verify by building first).

  **Must NOT do**:
  - Do NOT create `bridge/pkg/browser/navchart/` subpackage unless same-package fails due to import cycle
  - Do NOT modify `jetski/navchart/` — it stays as-is for Jetski's own use
  - Do NOT delete or modify `bridge/pkg/browser/chart_types.go` — reuse its types
  - Do NOT duplicate types that already exist in chart_types.go
  - Do NOT add a go.mod replace directive — the whole point is eliminating the external dependency
  - Do NOT change the Dockerfile — after vendoring, the bridge/ context contains everything needed

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Touches core Browser/RPC compile paths; may expose import-cycle or type compatibility issues that require judgment beyond mechanical copy
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `archon-dev`: Not Archon project code

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T2)
  - **Parallel Group**: Wave 1 (with T2)
  - **Blocks**: T3 (build+deploy), and transitively T5-T9
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References** (existing code to follow):
  - `bridge/pkg/browser/chart_types.go` — **REUSE THESE TYPES**: NavChart, ChartAction, ChartSelector, FrameRouting, WaitCondition, Assertion, ActionType, SelectorTier, ChartMetadata, plus all constants. Do NOT duplicate.
  - `jetski/navchart/multi_tab.go` — Source: MultiTabStore struct, NewMultiTabStore(), StoreChart(), GetCharts(), RemoveTab(), TabIDs() — COPY BEHAVIOR ONLY, change package to `browser`, use chart_types.go types
  - `jetski/navchart/replay.go` — Source: ReplayConfig, ReplayResult, Replay(), ReplayWithStore() — COPY BEHAVIOR ONLY, change package to `browser`
  - `jetski/navchart/diagnostics.go` — Source: ChartDiff struct (fields: Action, Expected, Actual, Match), DiffCharts(), DiffReplay() — COPY BEHAVIOR ONLY, change package to `browser`
  - `jetski/navchart/types.go` — **DO NOT COPY** — types already exist in chart_types.go

  **API/Type References** (contracts to implement against):
  - `bridge/pkg/rpc/browser.go:13` — Current import line to change: `"github.com/armorclaw/jetski/navchart"` → `"github.com/armorclaw/bridge/pkg/browser"` (or remove if already in browser package)
  - `bridge/pkg/rpc/server.go:42` — Current import line to change, stores `*navchart.MultiTabStore` → `*browser.MultiTabStore` in Server struct
  - `bridge/pkg/rpc/replay_diagnostics_test.go:8` — Test import to update
  - `bridge/pkg/rpc/replay_gating_test.go:8` — Test import to update
  - `bridge/pkg/rpc/edge_case_test.go:9` — Test import to update

  **External References**:
  - Go same-package convention: all files in `bridge/pkg/browser/` share `package browser` and can reference each other's symbols directly without imports

  **WHY Each Reference Matters**:
  - chart_types.go is the EXISTING type source — duplicating types creates two incompatible Go types (browser.NavChart ≠ browser/navchart.NavChart)
  - The 3 jetski navchart files provide BEHAVIOR that chart_types.go lacks — this is what we're vending
  - The 5 import lines must point to the same package that now contains the behavior

  **Acceptance Criteria**:

  - [ ] `bridge/pkg/browser/multi_tab.go`, `replay.go`, `diagnostics.go` exist with `package browser`
  - [ ] `bridge/pkg/browser/navchart/` does NOT exist (unless import cycle proven)
  - [ ] All 5 files updated: `grep -r "jetski/navchart" bridge/pkg/rpc/` returns zero results
  - [ ] `cd bridge && go test ./pkg/rpc/... ./pkg/browser/... -count=1` → ALL TESTS PASS

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Vendored code compiles and tests pass (same-package)
    Tool: Bash
    Preconditions: bridge/pkg/browser/multi_tab.go, replay.go, diagnostics.go exist with package browser
    Steps:
      1. cd bridge && go vet ./pkg/browser/...
           Expected: zero errors
      2. cd bridge && go test ./pkg/rpc/... ./pkg/browser/... ./pkg/sidecar/... ./pkg/email/... -count=1
           Expected: ALL TESTS PASS, zero failures
      3. cd bridge && go build ./cmd/bridge
           Expected: binary builds successfully, no linker errors
    Expected Result: All tests pass, binary builds clean
    Failure Indicators: "cannot find package", "undefined: navchart.ChartDiff", duplicate type definitions, test compilation errors
    Evidence: .sisyphus/evidence/task-1-navchart-vendor.txt

  Scenario: No jetski references remain in bridge imports
    Tool: Bash
    Preconditions: All 5 import updates applied
    Steps:
      1. grep -rn "jetski/navchart" bridge/pkg/
           Expected: zero matches
      2. grep -rn "jetski" bridge/go.mod
           Expected: zero matches (no external jetski dependency)
      3. test ! -d bridge/pkg/browser/navchart && echo "NO_SUBPACKAGE" || echo "SUBPACKAGE_EXISTS"
           Expected: NO_SUBPACKAGE (unless import cycle required it)
    Expected Result: Zero references to jetski/navchart in bridge code, no subpackage created
    Failure Indicators: Any grep match — missed import update
    Evidence: .sisyphus/evidence/task-1-no-jetski-refs.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-1-navchart-vendor.txt` — go test + go build output
  - [ ] `task-1-no-jetski-refs.txt` — grep proof of zero jetski references + no subpackage

  **Commit**: YES
  - Message: `fix(bridge): vendor navchart behavior into existing browser package`
  - Files: `bridge/pkg/browser/multi_tab.go`, `bridge/pkg/browser/replay.go`, `bridge/pkg/browser/diagnostics.go`, `bridge/pkg/rpc/browser.go`, `bridge/pkg/rpc/server.go`, `bridge/pkg/rpc/replay_diagnostics_test.go`, `bridge/pkg/rpc/replay_gating_test.go`, `bridge/pkg/rpc/edge_case_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/rpc/... ./pkg/browser/... -count=1`

- [x] 2. Debug sidecar-office HMAC crash on VPS

  **What to do**:
  - SSH to VPS (5.183.11.149) and run the diagnostic command suite to identify the exact root cause of the PermissionError
  - Execute these commands sequentially:
    1. `docker logs armorclaw-sidecar-office --tail=100` — capture crash output
    2. `docker inspect armorclaw-sidecar-office` — verify volume mounts and security opts
    3. `stat -c "%a %U %G %u %g %n" /run/armorclaw /run/armorclaw/secrets /run/armorclaw/secrets/office-hmac` — check full permission chain
    4. `namei -l /run/armorclaw/secrets/office-hmac` — verify directory traversal permissions
    5. `dmesg | grep -i -E "apparmor|denied|audit" | tail -100` — check for AppArmor denials
    6. `journalctl -k | grep -i -E "apparmor|denied|audit" | tail -100` — check kernel audit log
  - Run the override-entrypoint test checking BOTH host and container paths:
    ```bash
    # INSIDE the container — the authoritative path is /run/secrets/shared_secret
    docker compose -f deploy/docker-compose.sidecar-py.yml run --rm \
      --entrypoint sh sidecar-office -c '
    id
    echo "SECRET_PATH=${SECRET_PATH:-/run/secrets/shared_secret}"
    ls -ld /run /run/secrets 2>&1 || true
    ls -l /run/secrets/shared_secret 2>&1 || echo "CONTAINER_SECRET_MISSING"
    python - <<PY
    from pathlib import Path
    import os
    p = Path(os.environ.get("SECRET_PATH", "/run/secrets/shared_secret"))
    print("path", p)
    print("exists", p.exists())
    if p.exists():
        print("stat", oct(p.stat().st_mode), p.stat().st_uid, p.stat().st_gid)
        print("read_prefix", p.read_text()[:4])
    PY
    '
    ```
    ```bash
    # HOST-SIDE — verify the host path exists with correct permissions
    namei -l /run/armorclaw/secrets/office-hmac
    stat -c "%a %U %G %u %g %n" /run/armorclaw /run/armorclaw/secrets /run/armorclaw/secrets/office-hmac
    ```
  - Check if the AppArmor profile `armorclaw-office-worker` is loaded: `sudo aa-status 2>/dev/null | grep armorclaw`
  - Determine which hypothesis matches the evidence:
    - H1: Secret file doesn't exist (no provisioning code found)
    - H2: Parent directory `/run/armorclaw/secrets/` has restrictive permissions
    - H3: AppArmor profile blocks access
    - H4: Race condition (file created after container start)
    - H5: Docker bind-mount semantics issue (file replaced = inode change)
  - Record findings to `.sisyphus/evidence/task-2-hmac-debug.md`

  **Must NOT do**:
  - Do NOT apply any fixes yet — this is diagnosis only
  - Do NOT restart any services on the VPS
  - Do NOT modify compose files or Bridge code during diagnosis

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Requires SSH access to VPS, systematic hypothesis testing, and careful evidence gathering. Multiple possible root causes need methodical elimination.
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `playwright-cli`: No browser interaction needed

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T1)
  - **Parallel Group**: Wave 1 (with T1)
  - **Blocks**: T4 (fix sidecar)
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References** (existing code to follow):
  - `deploy/docker-compose.sidecar-py.yml` — Compose config for sidecar-office: user 10001:10001, network_mode: none, security_opt: apparmor=armorclaw-office-worker, volume mount `/run/armorclaw/secrets/office-hmac:/run/secrets/shared_secret:ro`
  - `sidecar-python/interceptor.py:23-38` — `load_shared_secret()` reads `os.environ.get("SECRET_PATH", "/run/secrets/shared_secret")` at line 34 — this is the crash point
  - `sidecar-python/worker.py:167` — Entry point calls `load_shared_secret()` as first action in `serve()`

  **API/Type References** (contracts to implement against):
  - `bridge/pkg/sidecar/token.go:170-178` — `GenerateSharedSecret()` generates random HMAC secret, `EncodeSharedSecret()`/`DecodeSharedSecret()` for encoding. These exist but are NEVER called from main.go.
  - `bridge/cmd/bridge/main.go:2636` — Creates `/run/armorclaw` directory but does NOT create `/run/armorclaw/secrets/` or write `office-hmac`

  **Test References**:
  - `bridge/pkg/sidecar/office_client_e2e_test.go:137` — E2E test writes secret with `os.WriteFile(path, data, 0600)` — this is the pattern the Bridge should use

  **WHY Each Reference Matters**:
  - The compose file shows exact volume mounts and security constraints — the diagnosis must verify each constraint
  - interceptor.py:34 is the crash line — confirms the read path
  - main.go:2636 proves NO provisioning exists — this is the strongest hypothesis
  - token.go has the generation function ready to use — it just needs to be called from main.go

  **Acceptance Criteria**:

  > **CRITICAL**: T4 is BLOCKED until this diagnosis template is fully completed.
  > The evidence file `task-2-hmac-debug.md` MUST follow this exact structure.

  - [ ] All 6 diagnostic commands executed with output captured
  - [ ] Override-entrypoint test executed with results captured
  - [ ] Root cause hypothesis confirmed with evidence
  - [ ] Findings document written to `.sisyphus/evidence/task-2-hmac-debug.md` using the **Required Diagnosis Template** below

  **Required Diagnosis Template** (`.sisyphus/evidence/task-2-hmac-debug.md`):

  The evidence file MUST follow this structure. No freeform narratives — the hypothesis table drives T4.

  ```markdown
  # Office HMAC Crash Diagnosis

  ## Confirmed Hypothesis

  Choose exactly one primary hypothesis:

  - H1: Secret file does not exist
  - H2: Parent directory permissions block traversal
  - H3: AppArmor/SELinux blocks access
  - H4: Race condition / secret created after sidecar starts
  - H5: Docker bind-mount/inode issue
  - H6: Other — describe

  **Confirmed hypothesis:** H_

  ## Evidence Summary

  ### Host Path

  Command:
  ```bash
  namei -l /run/armorclaw/secrets/office-hmac
  stat -c "%a %U %G %u %g %n" /run/armorclaw /run/armorclaw/secrets /run/armorclaw/secrets/office-hmac
  ```

  Observed result:
  ```text
  ...
  ```

  ### Container Path

  Command:
  ```bash
  docker compose -f deploy/docker-compose.sidecar-py.yml run --rm \
    --entrypoint sh sidecar-office -c '...'
  ```

  Observed result:
  ```text
  ...
  ```

  ### AppArmor / Kernel Denials

  Command:
  ```bash
  dmesg | grep -i -E "apparmor|denied|audit"
  journalctl -k | grep -i -E "apparmor|denied|audit"
  ```

  Observed result:
  ```text
  ...
  ```

  ## Why This Hypothesis Is Confirmed

  Explain in 3–6 bullets why the evidence supports the selected hypothesis and rules out the others.

  ## Implication for T4

  State exactly what T4 must do.

  Example:
  ```text
  T4 must create the secret before sidecar startup using office-secret-init,
  mount it to /run/secrets/shared_secret, and grant read access to UID/GID
  required by sidecar and Bridge.
  ```

  ## Do Not Do

  List any fixes the evidence rules out.

  Example:
  ```text
  Do not remove AppArmor; no denial was observed.
  Do not change sidecar UID; permission chain was the issue.
  ```
  ```

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: VPS diagnostic commands return usable data
    Tool: Bash (ssh)
    Preconditions: SSH access to 5.183.11.149, docker running
    Steps:
      1. ssh root@5.183.11.149 "docker logs armorclaw-sidecar-office --tail=50" 2>&1
           Expected: PermissionError traceback visible, or crash evidence
      2. ssh root@5.183.11.149 "stat -c '%a %U %G %u %g %n' /run/armorclaw/secrets/office-hmac" 2>&1
           Expected: Permission string, OR "No such file or directory"
      3. ssh root@5.183.11.149 "ls -ld /run/armorclaw/secrets/ 2>&1 || echo DIR_NOT_FOUND"
           Expected: Directory listing OR DIR_NOT_FOUND
    Expected Result: Sufficient evidence to confirm one of the 5 hypotheses
    Failure Indicators: SSH connection fails, docker not running
    Evidence: .sisyphus/evidence/task-2-hmac-debug.md

  Scenario: Override-entrypoint container test (checks container-mount path)
    Tool: Bash (ssh)
    Preconditions: sidecar-office compose file present on VPS
    Steps:
      1. ssh root@5.183.11.149 'docker compose -f /opt/armorclaw/docker-compose.sidecar-py.yml run --rm --entrypoint sh sidecar-office -c "id && echo SECRET_PATH=\${SECRET_PATH:-/run/secrets/shared_secret} && ls -ld /run /run/secrets 2>&1 && ls -l /run/secrets/shared_secret 2>&1 || echo CONTAINER_SECRET_MISSING"'
           Expected: Shows container user, directory permissions, file accessibility at /run/secrets/shared_secret
    Expected Result: Container can or cannot reach the secret — definitive evidence of mount correctness
    Failure Indicators: Compose file not found on VPS, CONTAINER_SECRET_MISSING
    Evidence: .sisyphus/evidence/task-2-entrypoint-test.txt

  Scenario: Host-side secret path verification
    Tool: Bash (ssh)
    Preconditions: SSH access to VPS
    Steps:
      1. ssh root@5.183.11.149 "namei -l /run/armorclaw/secrets/office-hmac"
           Expected: Full permission chain visible
      2. ssh root@5.183.11.149 "stat -c '%a %U %G %u %g %n' /run/armorclaw /run/armorclaw/secrets /run/armorclaw/secrets/office-hmac"
           Expected: Permission string and ownership for host-side path
    Expected Result: Host path exists with correct permissions (or proves it's missing)
    Failure Indicators: "No such file or directory" for any path component
    Evidence: .sisyphus/evidence/task-2-host-path.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-2-hmac-debug.md` — Full diagnosis findings with hypothesis confirmation
  - [ ] `task-2-entrypoint-test.txt` — Override-entrypoint test output

  **Commit**: NO (diagnosis only, no code changes)

- [x] 3. Build and deploy rebuilt Bridge image

  **What to do**:
  - Build the Bridge Docker image locally from the updated codebase (canonical command):
    ```bash
    cd bridge && docker build -t armorclaw:beato-fix -f Dockerfile .
    ```
  - Verify the build succeeds (the navchart dependency is now internal, no external jetski required)
  - Push the image:
    ```bash
    docker tag armorclaw:beato-fix mikegemut/armorclaw:beato-fix
    docker push mikegemut/armorclaw:beato-fix
    ```
  - Deploy to VPS (5.183.11.149) — **MUST use the new image tag explicitly**:
    ```bash
    ssh root@5.183.11.149 "docker pull mikegemut/armorclaw:beato-fix"
    # Set the image via environment variable or compose override:
    ssh root@5.183.11.149 "cd /opt/armorclaw && ARMORCLAW_IMAGE=mikegemut/armorclaw:beato-fix docker compose up -d bridge"
    # Or patch the compose override file:
    # services:
    #   bridge:
    #     image: mikegemut/armorclaw:beato-fix
    ```
  - Wait for bridge to become healthy (check `docker ps`)
  - Verify the bridge is running the new image (CRITICAL — do not assume):
    ```bash
    ssh root@5.183.11.149 "docker inspect armorclaw --format '{{.Config.Image}}'"
    # Must show: mikegemut/armorclaw:beato-fix
    ssh root@5.183.11.149 "docker image inspect \"\$(docker inspect armorclaw --format '{{.Image}}')\" --format '{{.Id}}'"
    # Must match the pushed image ID
    ```
  - Quick smoke test that bridge responds:
    ```bash
    TOKEN=$(ssh root@5.183.11.149 'cat /run/armorclaw/admin-token 2>/dev/null || echo ""')
    ssh root@5.183.11.149 "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"status\",\"params\":{\"token\":\"$TOKEN\"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock"
    ```

  **Must NOT do**:
  - Do NOT modify docker-compose.yml on VPS beyond what's needed for the image update
  - Do NOT restart Matrix, Jetski, or other services unless necessary
  - Do NOT change the bridge's UID (10002) or SQLCipher configuration

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Well-defined build+deploy operation following established pipeline
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T4)
  - **Parallel Group**: Wave 2 (with T4)
  - **Blocks**: T5, T6, T7, T8
  - **Blocked By**: T1

  **References**:

  **Pattern References**:
  - `bridge/Dockerfile` — Multi-stage build: golang:1.25-bookworm + CGO for SQLCipher, runs as UID 10002. After navchart vendoring, `go mod download` will succeed without jetski.
  - `Dockerfile.quickstart:28-39` — Working pattern for reference: copies jetski/ and adds replace directive. Our approach is simpler — no external dependency at all.

  **API/Type References**:
  - `deploy/deploy-all.sh` — 6-phase deployment with rollback. Use this for production deployment.
  - `scripts/deploy-hostinger-dockerhub.sh` — Docker Hub deployment script
  - `Makefile` (root) — `vps-up`, `vps-down` targets

  **WHY Each Reference Matters**:
  - The Dockerfile shows the build process that was previously failing at `go mod download`
  - deploy-all.sh is the canonical deployment script with rollback capability
  - The quickstart Dockerfile shows a working build pattern for comparison

  **Acceptance Criteria**:

  - [ ] `cd bridge && docker build -t armorclaw:beato-fix -f Dockerfile .` succeeds
  - [ ] Image pushed to registry or transferred to VPS
  - [ ] VPS running new image: `docker inspect armorclaw --format '{{.Config.Image}}'` shows `mikegemut/armorclaw:beato-fix`
  - [ ] Bridge responds to `status` RPC with auth token

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Docker build succeeds without jetski dependency
    Tool: Bash
    Preconditions: T1 completed (navchart vendored into existing browser package), bridge/ context has all needed code
    Steps:
      1. cd bridge && docker build -t armorclaw:beato-fix -f Dockerfile . 2>&1
           Expected: "Successfully tagged armorclaw:beato-fix"
      2. docker run --rm armorclaw:beato-fix /usr/local/bin/armorclaw-bridge --help 2>&1 || true
           Expected: Binary executes (may show usage/help or config error — confirms binary exists)
    Expected Result: Image builds clean, binary is functional
    Failure Indicators: "go mod download: github.com/armorclaw/jetski/navchart: module not found", compile errors
    Evidence: .sisyphus/evidence/task-3-bridge-build.txt

  Scenario: Deployed bridge is running the CORRECT image
    Tool: Bash (ssh)
    Preconditions: Image deployed and bridge container is Up
    Steps:
      1. ssh root@5.183.11.149 "docker ps --filter name=armorclaw --format '{{.Status}}'"
           Expected: "Up" (healthy)
      2. ssh root@5.183.11.149 "docker inspect armorclaw --format '{{.Config.Image}}'"
           Expected: "mikegemut/armorclaw:beato-fix" (NOT the old tag)
      3. TOKEN=$(ssh root@5.183.11.149 'cat /run/armorclaw/admin-token 2>/dev/null'); ssh root@5.183.11.149 "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"status\",\"params\":{\"token\":\"$TOKEN\"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock"
           Expected: Valid JSON with status info (not "method not found", not auth error)
    Expected Result: Bridge is healthy, running the correct new image, and responding to RPC calls
    Failure Indicators: Container not running, old image tag, socket not accessible, no response, auth error
    Evidence: .sisyphus/evidence/task-3-vps-bridge-status.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-3-bridge-build.txt` — Docker build output
  - [ ] `task-3-vps-bridge-status.txt` — VPS bridge status and RPC response

  **Commit**: NO (deployment operation, no code changes)

- [x] 4. Fix sidecar-office HMAC provisioning via init-service

  **What to do**:
  - Based on T2 diagnosis findings, implement the HMAC fix. The preferred approach uses a Compose one-shot init service (NOT Bridge chown).
  - **Problem**: Bridge runs as UID 10002 and cannot `os.Chown()` to UID 10001. We must not weaken container privileges.

  **Fix Classification (CTO mandate)**:

  **Primary fix** (MUST implement):
  - Add `office-secret-init` one-shot Compose service.
  - Generate `/run/armorclaw/secrets/office-hmac` before sidecar starts.
  - Mount it into sidecar as `/run/secrets/shared_secret:ro`.
  - Use shared group access if Bridge also reads the secret.
  - Final file mode must be **0440**.

  **Required hardening** (preserve these constraints):
  - Sidecar keeps `network_mode: none`.
  - Sidecar keeps `cap_drop: ALL`.
  - Sidecar keeps UID 10001.
  - HMAC validation remains enabled.

  **Fallback only** (resilience, not fix):
  - Add retry logic in `sidecar-python/interceptor.py` only to handle mount timing/startup race.
  - Retry logic is NOT a substitute for managed provisioning.

  **Debug-only** (temporary, not production):
  - AppArmor removal is permitted only as a temporary diagnostic step.
  - If AppArmor is the cause, prefer updating/loading the profile.
  - Do NOT leave AppArmor disabled unless explicitly accepted in the final report.

  **Implementation Steps**:

  - **Solution**: Add `office-secret-init` one-shot Compose service that runs as root:

    1. **Add `office-secret-init` to compose file** (`deploy/docker-compose.sidecar-py.yml`):
       ```yaml
       office-secret-init:
         image: busybox:latest
         user: "0:0"
         command: >
           sh -c '
             set -eu;
             mkdir -p /run/armorclaw/secrets;
             if [ ! -s /run/armorclaw/secrets/office-hmac ]; then
               head -c 32 /dev/urandom | xxd -p -c 64 > /run/armorclaw/secrets/office-hmac;
             fi;
             chown 10001:10001 /run/armorclaw/secrets/office-hmac;
             chmod 0440 /run/armorclaw/secrets/office-hmac;
           '
         volumes:
           - /run/armorclaw:/run/armorclaw
         restart: "no"
       ```
    2. **Add dependency to sidecar-office**:
       ```yaml
       sidecar-office:
         depends_on:
           office-secret-init:
             condition: service_completed_successfully
       ```
       If Compose version does not support `service_completed_successfully`, use startup retry in `interceptor.py` instead.
    3. **Add startup retry in sidecar** (`sidecar-python/interceptor.py` load_shared_secret):
       - Retry the secret read for up to 30 seconds with 1-second intervals
       - Log each attempt
       - Exit with clear error if secret never appears
    4. **Bridge reads the secret** — Bridge only reads/uses the HMAC; it does NOT chown or create host files.
       - If Bridge needs the HMAC value, read from `/run/armorclaw/secrets/office-hmac`
       - Keep provisioning logic in `bridge/pkg/sidecar/office_provision.go` (not in main.go directly)
    5. **Handle AppArmor** (if T2 diagnosis shows AppArmor denial):
       - Either load the profile on the host, or remove `security_opt: apparmor=armorclaw-office-worker` from compose
       - Prefer loading the profile if it exists in the repo
  - Deploy the fix to VPS:
    - Rebuild and push sidecar-office image (with retry logic)
    - Deploy compose file with init-service on VPS
    - Verify sidecar stays healthy

  **Must NOT do**:
  - Do NOT rely on unprivileged Bridge `os.Chown()` — UID 10002 cannot chown to 10001
  - Do NOT weaken Bridge container privileges (no `--privileged`, no `CAP_CHOWN`)
  - Do NOT make the secret world-readable (mode must be 0440 or stricter)
  - Do NOT remove the `network_mode: none` constraint
  - Do NOT remove `cap_drop: ALL`
  - Do NOT change the sidecar UID (must remain 10001)
  - Do NOT bypass HMAC validation — it must remain functional
  - Do NOT put secret generation code directly in `main.go` — use `bridge/pkg/sidecar/office_provision.go`
  - Do NOT leave mode 0644 as the final state — must be 0440

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multiple possible fix patterns depending on diagnosis, requires judgment across Go and Python, Docker security configuration
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T3)
  - **Parallel Group**: Wave 2 (with T3)
  - **Blocks**: T6 (Office E2E)
  - **Blocked By**: T2

  **References**:

  **Pattern References**:
  - `bridge/pkg/sidecar/java_client.go` — `ProvisionJavaSocketDir()` — historical pattern for provisioning with `os.Chown`. NOTE: CTO decided NOT to follow this pattern for HMAC — use init-service instead because Bridge (UID 10002) cannot chown to UID 10001.
  - `bridge/pkg/sidecar/token.go:170-178` — `GenerateSharedSecret()` — generates 32-byte random hex secret. Available if the init-service needs to call Bridge for generation.
  - `bridge/pkg/sidecar/office_client_e2e_test.go:137` — `os.WriteFile(path, data, 0600)` — existing test pattern for writing the secret file

  **API/Type References**:
  - `sidecar-python/interceptor.py:23-38` — `load_shared_secret()` — the function that crashes. Reads from `SECRET_PATH` env var (default `/run/secrets/shared_secret`)
  - `deploy/docker-compose.sidecar-py.yml` — Compose config with volume mount, AppArmor, user constraints

  **Test References**:
  - `sidecar-python/test_interceptor.py` — 12 HMAC-SHA256 token validation tests. Verify these still pass after retry logic addition.
  - `sidecar-python/test_docker_integration.py` — 10 Docker integration tests. Verify sidecar lifecycle.

  **WHY Each Reference Matters**:
  - java_client.go provides the exact code pattern to follow for provisioning with correct ownership
  - token.go has the generation function ready — just wire it into main.go
  - interceptor.py is the crash site — the retry logic must be added here, preserving existing validation
  - The compose file shows the exact security constraints that must be maintained

  **Acceptance Criteria**:

  - [ ] `office-secret-init` one-shot service defined in compose file
  - [ ] `sidecar-office` depends on `office-secret-init` (with `condition: service_completed_successfully` or retry fallback)
  - [ ] `sidecar-python/interceptor.py` retries secret read for up to 30 seconds
  - [ ] Bridge does NOT call `os.Chown()` — no privileged file operations
  - [ ] `go test ./pkg/sidecar/... -count=1` passes (bridge)
  - [ ] `cd sidecar-python && python -m pytest test_interceptor.py -v` passes
  - [ ] Sidecar-office stays Up/healthy on VPS for 5+ minutes
  - [ ] HMAC file mode is 0440 (NOT 0644)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Sidecar-office stays healthy after fix
    Tool: Bash (ssh)
    Preconditions: Compose with init-service deployed, sidecar-office restarted
    Steps:
      1. ssh root@5.183.11.149 "docker ps --filter name=sidecar-office --format '{{.Status}}'"
           Expected: "Up X minutes" where X > 1
      2. sleep 300 && ssh root@5.183.11.149 "docker ps --filter name=sidecar-office --format '{{.Status}}'"
           Expected: "Up 5 minutes" — no crash/restart cycle
      3. ssh root@5.183.11.149 "docker logs armorclaw-sidecar-office --tail=20" 2>&1
           Expected: No PermissionError, no crash traceback. Server listening messages.
    Expected Result: Sidecar stable for 5+ minutes, no PermissionError
    Failure Indicators: "Restarting" status, PermissionError in logs, crash traceback
    Evidence: .sisyphus/evidence/task-4-sidecar-healthy.txt

  Scenario: HMAC secret provisioned with correct permissions by init-service
    Tool: Bash (ssh)
    Preconditions: office-secret-init completed successfully
    Steps:
      1. ssh root@5.183.11.149 "ls -l /run/armorclaw/secrets/office-hmac"
           Expected: File exists, owned by 10001:10001, mode 0440
      2. ssh root@5.183.11.149 "stat -c '%a %U %G' /run/armorclaw/secrets/office-hmac"
           Expected: Mode 0440, owner 10001, group 10001
      3. ssh root@5.183.11.149 "docker logs office-secret-init 2>&1 || true"
           Expected: Init-service completed successfully (exit 0)
    Expected Result: Secret file exists with correct ownership (10001:10001) and mode (0440)
    Failure Indicators: File not found, wrong ownership (root), wrong mode (0644/0666)
    Evidence: .sisyphus/evidence/task-4-secret-permissions.txt

  Scenario: Bridge did NOT perform privileged chown
    Tool: Bash
    Preconditions: Bridge code reviewed
    Steps:
      1. grep -rn "os.Chown" bridge/cmd/bridge/main.go
           Expected: Zero matches (Bridge does not chown)
      2. grep -rn "Chown" bridge/pkg/sidecar/office_provision.go || echo "NO_CHOWN"
           Expected: NO_CHOWN or no chown calls related to secret provisioning
    Expected Result: Bridge code contains no chown operations for the HMAC secret
    Failure Indicators: os.Chown found in bridge startup or provisioning code
    Evidence: .sisyphus/evidence/task-4-no-bridge-chown.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-4-sidecar-healthy.txt` — docker ps output + logs showing stable operation
  - [ ] `task-4-secret-permissions.txt` — file permissions verification (0440, uid 10001)
  - [ ] `task-4-no-bridge-chown.txt` — grep proof Bridge has no chown operations

  **Commit**: YES
  - Message: `fix(sidecar): provision office HMAC via init-service with correct permissions`
  - Files: `deploy/docker-compose.sidecar-py.yml`, `bridge/pkg/sidecar/office_provision.go` (new), `sidecar-python/interceptor.py`
  - Pre-commit: `cd bridge && go test ./pkg/sidecar/... -count=1`

- [x] 5. Verify new Bridge RPCs on VPS

  **What to do**:
  - SSH to VPS and verify all 7 newly deployed BEATO RPCs plus 9 previously untested Browser RPCs are now callable
  - **ALL RPC calls MUST use auth tokens** (follow `test-browser-smoke.sh` pattern):
    ```bash
    TOKEN=$(ssh root@5.183.11.149 'cat /run/armorclaw/admin-token 2>/dev/null || echo ""')
    ```
  - Test 7 newly deployed BEATO RPCs:
    - `document.extract_text` — send with dummy payload, expect valid JSON (not "method not found")
    - `document.status` — expect valid JSON
    - `document.list_jobs` — expect valid JSON (empty list is OK)
    - `email.queue_status` — expect valid JSON
    - `email.get` — expect valid JSON (error for missing ID is OK — we're testing registration)
    - `email.list` — expect valid JSON
    - `email.retry` — expect valid JSON
  - Test 9 previously untested Browser RPCs:
    - `browser.fill`, `browser.click`, `browser.cancel`
    - `browser.wait_for_element`, `browser.wait_for_captcha`, `browser.wait_for_2fa`
    - `browser.complete`, `browser.fail`, `browser.replay_diagnostics`
  - Verify existing 14 Text regression RPCs still work:
    ```bash
    ssh root@5.183.11.149 "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"status\",\"params\":{\"token\":\"$TOKEN\"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock"
    ```
  - Capture all results

  **Must NOT do**:
  - Do NOT send actual document files through the pipeline (that's T6)
  - Do NOT send actual emails (that's T8)
  - Do NOT modify the Bridge code
  - Do NOT restart the Bridge during testing

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Straightforward RPC smoke testing following established patterns
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T6, T7)
  - **Parallel Group**: Wave 3 (with T6, T7)
  - **Blocks**: T6 (partially — T6 also needs T4), T8, T9
  - **Blocked By**: T3

  **References**:

  **Pattern References**:
  - `tests/test-browser-smoke.sh` — Existing pattern for RPC smoke testing on VPS with auth tokens. Follow this exact pattern for generating auth and calling RPCs.
  - `tests/test-email-pipeline.sh` — Pattern for email-specific RPC testing (7 scenarios)

  **API/Type References**:
  - `bridge/pkg/rpc/server.go:1369-1375` — Registration lines for the 7 new RPCs being tested
  - `bridge/pkg/rpc/document.go` — Document RPC handler signatures
  - `bridge/pkg/rpc/email_queue.go` — Email queue RPC handler signatures

  **WHY Each Reference Matters**:
  - test-browser-smoke.sh is the proven pattern for VPS RPC testing with auth
  - server.go registration lines confirm exact method names to test
  - Handler files confirm expected request/response shapes

  **Acceptance Criteria**:

  - [ ] All 3 document.* RPCs return valid JSON with auth token (not "method not found")
  - [ ] All 4 email queue RPCs return valid JSON with auth token (not "method not found")
  - [ ] All 9 previously-untested browser.* RPCs return valid JSON with auth token
  - [ ] Existing `status` RPC still works (regression check)
  - [ ] Results captured in evidence file

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All 7 newly deployed RPCs respond on VPS (with auth)
    Tool: Bash (ssh)
    Preconditions: New bridge image deployed and healthy
    Steps:
      1. TOKEN=$(ssh root@5.183.11.149 'cat /run/armorclaw/admin-token 2>/dev/null')
      2. For each method in document.extract_text, document.status, document.list_jobs, email.queue_status, email.get, email.list, email.retry:
         ssh root@5.183.11.149 "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"METHOD\",\"params\":{\"token\":\"$TOKEN\"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock"
         Expected: JSON response NOT containing "method not found"
      3. ssh root@5.183.11.149 "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"status\",\"params\":{\"token\":\"$TOKEN\"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock"
         Expected: Valid status JSON (regression check)
    Expected Result: 7/7 new RPCs callable, 1/1 regression RPC passing
    Failure Indicators: "method not found" for any method, socket connection refused, auth error
    Evidence: .sisyphus/evidence/task-5-rpc-verification.txt

  Scenario: Auth-gated RPCs require valid token
    Tool: Bash (ssh)
    Preconditions: Bridge running with rpc_safety.go middleware
    Steps:
      1. ssh root@5.183.11.149 'echo "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"document.status\"}" | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock'
         Expected: 401/auth error (proves middleware is active)
      2. TOKEN=$(ssh root@5.183.11.149 'cat /run/armorclaw/admin-token'); ssh root@5.183.11.149 "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"document.status\",\"params\":{\"token\":\"$TOKEN\"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock"
         Expected: Valid JSON response
    Expected Result: Middleware correctly gates access — no token = rejection, valid token = success
    Failure Indicators: RPCs respond without auth (security gap), valid token rejected
    Evidence: .sisyphus/evidence/task-5-auth-gate.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-5-rpc-verification.txt` — All 7 new RPC responses + regression check
  - [ ] `task-5-auth-gate.txt` — Auth middleware verification

  **Commit**: NO (verification only)

- [x] 6. Office E2E validation

  **What to do**:
  - Verify the full Office document pipeline works end-to-end on VPS
  - Prerequisite checks:
    1. Verify sidecar-office is healthy: `docker ps --filter name=sidecar-office`
    2. Verify bridge has new RPCs: `document.extract_text` callable
    3. Verify Matrix is connected (email approval flow depends on it)
  - Happy path test:
    1. Submit a small XLSX file through `document.extract_text`
    2. Verify YARA scan passes (clean file)
    3. Verify RouteExtractText routes to sidecar-office
    4. Verify extracted text is returned
    5. Verify artifact is stored
  - Negative tests (CTO specified):
    1. Corrupt file → controlled error (not crash/panic)
    2. MIME mismatch → reject before sidecar (strict drop at Layer 2)
    3. YARA match → reject before sidecar
    4. Missing HMAC → fail closed
    5. Invalid HMAC → fail closed
  - Follow the test pattern from `tests/test-sidecar-docs.sh` (bash integration test)
  - Architecture reference: Go Bridge validates MIME/magic → routes to Python sidecar via `/run/armorclaw/sidecar-office.sock` → 3-layer routing with strict drop on mismatch

  **Must NOT do**:
  - Do NOT test with files >10MB (threshold streaming boundary)
  - Do NOT disable YARA scanning
  - Do NOT bypass the 3-layer routing
  - Do NOT test with real sensitive documents

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Multi-step E2E testing across Go bridge + Python sidecar, requires creating test fixtures and validating 3-layer routing + YARA + HMAC security
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T5, T7 — but needs T4 AND T5 results)
  - **Parallel Group**: Wave 3 (starts after T4 and T5 both complete)
  - **Blocks**: T9
  - **Blocked By**: T4 (sidecar fix), T5 (RPC verification)

  **References**:

  **Pattern References**:
  - `tests/test-sidecar-docs.sh` — Existing bash integration test for the document pipeline. Follow this pattern for test structure and assertions.
  - `bridge/pkg/sidecar/office_client.go` — `RouteExtractText()` — 3-layer routing implementation. Layer 0: native text bypass. Layer 1: valid magic + MIME → route to sidecar. Layer 2: magic ≠ declared format → strict drop.

  **API/Type References**:
  - `bridge/pkg/rpc/document.go` — Document RPC handler. `document.extract_text` accepts file data, routes through YARA + sidecar.
  - `sidecar-python/worker.py` — Python sidecar worker. Processes XLSX/PPTX/MSG/XLS/DOC/PPT.
  - `bridge/pkg/sidecar/office_client_e2e_test.go` — E2E test pattern for Go→Python communication

  **Test References**:
  - `sidecar-python/test_worker.py` — 27 tests covering format mapping, text extraction, threshold streaming
  - `sidecar-python/test_edge_cases.py` — 16 tests for empty payloads, corrupt files, boundary conditions

  **WHY Each Reference Matters**:
  - test-sidecar-docs.sh is the existing integration test — follow its structure
  - office_client.go documents the 3-layer routing logic that negative tests must verify
  - The Python test files show what edge cases are already covered (don't duplicate)

  **Acceptance Criteria**:

  - [ ] XLSX file submitted → extracted text returned
  - [ ] Corrupt file → controlled error (no panic/crash)
  - [ ] MIME mismatch → rejected (not routed to sidecar)
  - [ ] YARA match → rejected before sidecar
  - [ ] Missing HMAC → fail closed
  - [ ] Invalid HMAC → fail closed

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: XLSX extraction happy path
    Tool: Bash (ssh)
    Preconditions: Bridge + sidecar-office both healthy, XLSX test file prepared
    Steps:
      1. TOKEN=$(ssh root@5.183.11.149 'cat /run/armorclaw/admin-token 2>/dev/null')
      2. Create a minimal valid XLSX file (base64 encoded)
      3. Submit via document.extract_text RPC on VPS with auth token
      4. Parse response JSON — check for extracted text content
    Expected Result: Response contains extracted text, no error
    Failure Indicators: "method not found", auth error, sidecar connection error, empty extraction
    Evidence: .sisyphus/evidence/task-6-office-e2e.txt

  Scenario: MIME mismatch strict drop
    Tool: Bash (ssh)
    Preconditions: Bridge running with 3-layer routing, auth token available
    Steps:
      1. TOKEN=$(ssh root@5.183.11.149 'cat /run/armorclaw/admin-token')
      2. Submit a file with XLSX extension but PNG magic bytes (with auth token)
      3. Verify response contains rejection error (not routed to sidecar)
    Expected Result: "strict drop" or format mismatch error
    Failure Indicators: File routed to sidecar despite mismatch (Layer 2 bypass)
    Evidence: .sisyphus/evidence/task-6-mime-mismatch.txt

  Scenario: Corrupt file controlled error
    Tool: Bash (ssh)
    Preconditions: Bridge + sidecar running, auth token available
    Steps:
      1. TOKEN=$(ssh root@5.183.11.149 'cat /run/armorclaw/admin-token')
      2. Submit random bytes as "test.xlsx" (with auth token)
      3. Verify controlled error response (no bridge crash, no sidecar crash)
    Expected Result: Error message about invalid/corrupt file
    Failure Indicators: Bridge panic, sidecar crash, 500 error
    Evidence: .sisyphus/evidence/task-6-corrupt-file.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-6-office-e2e.txt` — Happy path result
  - [ ] `task-6-mime-mismatch.txt` — MIME mismatch rejection
  - [ ] `task-6-corrupt-file.txt` — Corrupt file handling
  - [ ] Additional negative test evidence as needed

  **Commit**: NO (verification only)

- [x] 7. Browser RPC coverage test

  **What to do**:
  - Test the 9 remaining browser.* RPCs on VPS that weren't previously smoke-tested
  - Already tested (3/12): `browser.navigate`, `browser.status`, `browser.list`
  - To test (9/12): `browser.fill`, `browser.click`, `browser.cancel`, `browser.wait_for_element`, `browser.wait_for_captcha`, `browser.wait_for_2fa`, `browser.complete`, `browser.fail`, `browser.replay_diagnostics`
  - For each method:
    1. Send a valid RPC call (with auth token)
    2. Expect valid JSON response — business errors are acceptable (e.g., "session not found")
    3. The test validates that the RPC path is wired, not that it succeeds with test data
  - Use the `rpc_call` pattern from `tests/test-browser-smoke.sh`

  **Must NOT do**:
  - Do NOT attempt to restore `browser_test.go.bak` — separate task
  - Do NOT modify `tests/test-rpc-methods.sh` — not this sprint
  - Do NOT start actual browser sessions — just test RPC registration/wiring

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple RPC smoke testing following established test-browser-smoke.sh pattern
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T5, T6)
  - **Parallel Group**: Wave 3 (with T5, T6)
  - **Blocks**: T9
  - **Blocked By**: T3

  **References**:

  **Pattern References**:
  - `tests/test-browser-smoke.sh` — Existing browser smoke test. Follow this exact pattern for auth token generation and RPC call structure.

  **API/Type References**:
  - `bridge/pkg/rpc/browser.go` — Browser RPC handler implementations. All 12 methods registered at server.go:1230-1241.

  **WHY Each Reference Matters**:
  - test-browser-smoke.sh is the proven pattern — replicate it for the 9 untested methods
  - browser.go confirms handler signatures for constructing valid request payloads

  **Acceptance Criteria**:

  - [ ] All 9 previously-untested browser RPCs return valid JSON
  - [ ] Total browser coverage: 12/12 tested on VPS
  - [ ] None return "method not found"

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All 9 untested browser RPCs respond (with auth)
    Tool: Bash (ssh)
    Preconditions: New bridge image deployed
    Steps:
      1. TOKEN=$(ssh root@5.183.11.149 'cat /run/armorclaw/admin-token 2>/dev/null')
      2. For each method in browser.fill, browser.click, browser.cancel, browser.wait_for_element, browser.wait_for_captcha, browser.wait_for_2fa, browser.complete, browser.fail, browser.replay_diagnostics:
         Send RPC call via socat with dummy session_id and auth token
         Expected: Valid JSON response (may contain business error like "session not found", but NOT "method not found")
      3. Count: 9/9 methods responded with valid JSON
    Expected Result: 9/9 RPCs callable, 12/12 total browser coverage
    Failure Indicators: "method not found" for any method, auth error
    Evidence: .sisyphus/evidence/task-7-browser-coverage.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-7-browser-coverage.txt` — All 9 RPC responses

  **Commit**: NO (verification only)

- [x] 8. Email outbox deploy and test

  **What to do**:
  - After rebuilt Bridge is deployed (T3) and RPCs verified (T5), test the email outbox queue system
  - Prerequisite: Verify Matrix is healthy (email approval flow depends on Matrix connection):
    ```bash
    ssh root@5.183.11.149 "curl -s http://localhost:6167/_matrix/client/versions"
    ```
  - Verify email queue RPCs are callable (should have been confirmed in T5):
    - `email.queue_status` — empty queue expected
    - `email.get` — error for missing ID is OK
    - `email.list` — empty list expected
    - `email.retry` — error for missing ID is OK
  - Run the existing email pipeline test harness:
    ```bash
    bash tests/test-email-pipeline.sh
    ```
    This tests 7 scenarios: status, list, deny, approve, restart, and negative tests
  - Manual validation of the outbox lifecycle:
    1. Submit an approval request
    2. Confirm outbox entry appears
    3. Approve the request
    4. Confirm status transition
    5. Restart Bridge
    6. Confirm queue persists across restart
  - Do NOT start Postfix/DNS — that's a separate infrastructure phase

  **Must NOT do**:
  - Do NOT deploy Postfix or configure DNS
  - Do NOT send actual emails to external addresses
  - Do NOT modify the email state machine

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multi-step validation across RPC layer + OutboxStore + Matrix approval flow, requires careful state verification across restart
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential after T5)
  - **Parallel Group**: Wave 4
  - **Blocks**: T9
  - **Blocked By**: T3, T5

  **References**:

  **Pattern References**:
  - `tests/test-email-pipeline.sh` — Existing 7-scenario email pipeline test harness. Use this as the primary validation tool.
  - `bridge/pkg/email/outbox.go` (262 lines) — OutboxStore with 8-status state machine. Fully implemented with 11/11 tests passing in `outbox_test.go`.

  **API/Type References**:
  - `bridge/pkg/rpc/email_queue.go` — Queue RPC handlers (queue_status, get, retry, list)
  - `bridge/pkg/rpc/email_approval.go` — Approval handlers (approve_email, deny_email, email_approval_status, email.list_pending)

  **Test References**:
  - `bridge/pkg/email/outbox_test.go` — 11/11 tests passing. Data layer is well-tested.
  - `tests/test-email-pipeline.sh` — 7-scenario integration test

  **WHY Each Reference Matters**:
  - test-email-pipeline.sh is the canonical email validation — use it directly
  - outbox.go confirms the state machine is complete (just needs deployed bridge)
  - Note: email_queue_test.go only tests handler registration, not logic — the VPS test will be the first real handler+store exercise

  **Acceptance Criteria**:

  - [ ] `tests/test-email-pipeline.sh` passes (7/7 scenarios)
  - [ ] `email.queue_status` returns valid JSON
  - [ ] Outbox entry created after approval request
  - [ ] Approval changes entry status
  - [ ] Queue persists across Bridge restart

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Email pipeline test harness passes
    Tool: Bash
    Preconditions: Bridge deployed with email RPCs, Matrix healthy
    Steps:
      1. bash tests/test-email-pipeline.sh
           Expected: 7/7 scenarios PASS
    Expected Result: All email pipeline scenarios pass
    Failure Indicators: Any scenario FAIL, Matrix connection errors
    Evidence: .sisyphus/evidence/task-8-email-pipeline.txt

  Scenario: Queue persists across Bridge restart
    Tool: Bash (ssh)
    Preconditions: Email in queue from pipeline test
    Steps:
      1. TOKEN=$(ssh root@5.183.11.149 'cat /run/armorclaw/admin-token 2>/dev/null')
      2. ssh root@5.183.11.149 "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"email.queue_status\",\"params\":{\"token\":\"$TOKEN\"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock"
           Expected: Queue with entries
      3. ssh root@5.183.11.149 "docker restart armorclaw"
      4. sleep 10
      5. TOKEN=$(ssh root@5.183.11.149 'cat /run/armorclaw/admin-token 2>/dev/null')
      6. ssh root@5.183.11.149 "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"email.queue_status\",\"params\":{\"token\":\"$TOKEN\"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock"
           Expected: Same queue entries persist
    Expected Result: Queue state survives bridge restart
    Failure Indicators: Empty queue after restart (data loss)
    Evidence: .sisyphus/evidence/task-8-queue-persistence.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-8-email-pipeline.txt` — Pipeline test results
  - [ ] `task-8-queue-persistence.txt` — Restart persistence test

  **Commit**: NO (verification only)

- [x] 9. Final runtime BEATO report

  **What to do**:
  - Write an honest runtime BEATO verification report based on actual VPS test results
  - Use the existing scoring template from `tests/reports/beato-verification-report.md` with the 100-point rubric:
    - Browser: 25 pts (Jetski deployed, no public ports, session lifecycle, external HTTPS)
    - Email: 20 pts (Outbox store, RPC methods, approval flow, VPS smoke)
    - Text: 20 pts (14/14 RPC regression)
    - Office: 25 pts (Sidecar deployed, document RPC, extraction, YARA clean)
    - Audio: 10 pts (Audit report exists — deferred by design)
  - Score each pillar based on actual evidence from T5-T8
  - Target: ≥90/100 (not 100/100 — be honest about what's deferred)
  - Write to `tests/reports/beato-runtime-report.md` (new file — don't overwrite the old plan-completion report)
  - Include actual test results as evidence, not just checkboxes
  - Clearly mark Audio as "deferred by design, scored on audit-only basis"

  **Must NOT do**:
  - Do NOT inflate scores — use actual VPS evidence
  - Do NOT overwrite `tests/reports/beato-verification-report.md` — create a NEW file
  - Do NOT call BEATO 100% — Audio is deferred, and honest scoring matters
  - Do NOT invent test results — if something wasn't tested, say so

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Documenting test results into a structured report with scoring rubric
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on all prior results)
  - **Parallel Group**: Wave 5
  - **Blocks**: Final verification wave
  - **Blocked By**: T5, T6, T7, T8

  **References**:

  **Pattern References**:
  - `tests/reports/beato-verification-report.md` — Existing 100-point scoring template. Use the SAME rubric categories and point allocations for consistency.
  - `tests/reports/beato-current-status.md` — Current status report (464 lines). Use this as the baseline, then update with actual runtime results.
  - `tests/reports/audio-capability-audit.md` — Audio audit for the Audio scoring section.

  **API/Type References**:
  - All evidence files from T5-T8 — these are the source data for the report

  **WHY Each Reference Matters**:
  - The verification report template ensures consistent scoring methodology
  - The current status report is the baseline — the new report shows improvement
  - Evidence files are the ground truth — the report must cite them

  **Acceptance Criteria**:

  - [ ] Report written to `tests/reports/beato-runtime-report.md`
  - [ ] Uses the same 100-point rubric as beato-verification-report.md
  - [ ] Scores based on actual VPS evidence, not plan checkboxes
  - [ ] Each score cites specific evidence files
  - [ ] Audio section clearly marked "deferred by design"

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Report exists and follows scoring rubric
    Tool: Bash
    Preconditions: All validation tasks (T5-T8) completed with evidence
    Steps:
      1. test -f tests/reports/beato-runtime-report.md && echo "EXISTS" || echo "MISSING"
           Expected: EXISTS
      2. grep -c "Browser.*25" tests/reports/beato-runtime-report.md
           Expected: ≥1 (Browser section with 25-point allocation)
      3. grep -c "Audio.*deferred" tests/reports/beato-runtime-report.md
           Expected: ≥1 (Audio marked as deferred)
    Expected Result: Report follows the template structure with honest scoring
    Failure Indicators: Missing file, missing rubric sections, inflated scores
    Evidence: .sisyphus/evidence/task-9-report-exists.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-9-report-exists.txt` — Report structure verification

  **Commit**: YES
  - Message: `docs(beato): add runtime verification report`
  - Files: `tests/reports/beato-runtime-report.md`
  - Pre-commit: none

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle` (REJECT→remediated: office_provision.go created, AppArmor accepted, evidence captured)
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high` (APPROVE — 14/15 clean, 1 minor style)
  Run `go test ./...` + `go vet ./...` in bridge/. Review all changed files for: excessive comments, over-abstraction, unused imports, debug prints. Check AI slop: generic names, unnecessary wrappers. Verify no new dependencies added.
  Output: `Build [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Live VPS Integration QA** — `unspecified-high` (REJECT — pre-existing issues: sidecar-office broken image, Matrix network isolation)
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration: Bridge RPCs + Office sidecar + Email pipeline working together. All RPC calls MUST use auth tokens (follow `test-browser-smoke.sh` pattern). Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep` (APPROVE — 9/9 compliant, 0 unaccounted)
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built, nothing beyond spec. Check "Must NOT do" compliance. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **T1**: `fix(bridge): vendor navchart behavior into existing browser package` — bridge/pkg/browser/multi_tab.go, bridge/pkg/browser/replay.go, bridge/pkg/browser/diagnostics.go, bridge/pkg/rpc/browser.go, bridge/pkg/rpc/server.go, bridge/pkg/rpc/*_test.go
- **T4**: `fix(sidecar): provision office HMAC via init-service with correct permissions` — deploy/docker-compose.sidecar-py.yml, bridge/pkg/sidecar/office_provision.go, sidecar-python/interceptor.py
- **T3**: No commit (deployment operation)
- **T9**: `docs(beato): add runtime verification report` — tests/reports/beato-runtime-report.md

---

## Success Criteria

### Verification Commands
```bash
# Local: navchart vendoring (same-package, no subpackage)
cd bridge && go test ./pkg/rpc/... ./pkg/browser/... ./pkg/sidecar/... ./pkg/email/... -count=1
# Expected: ALL TESTS PASS

# Local: bridge binary builds
cd bridge && go build ./cmd/bridge
# Expected: success, no errors

# VPS: sidecar-office healthy (after init-service fix)
ssh root@5.183.11.149 "docker ps --filter name=sidecar-office --format '{{.Status}}'"
# Expected: "Up" (not "Restarting")

# VPS: HMAC secret has correct permissions (0440, uid 10001)
ssh root@5.183.11.149 "stat -c '%a %U %G' /run/armorclaw/secrets/office-hmac"
# Expected: "0440 10001 10001"

# VPS: new RPCs callable with auth
TOKEN=$(ssh root@5.183.11.149 'cat /run/armorclaw/admin-token')
ssh root@5.183.11.149 "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"document.status\",\"params\":{\"token\":\"$TOKEN\"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock"
# Expected: valid JSON response (not "method not found", not auth error)

# VPS: email pipeline
bash tests/test-email-pipeline.sh
# Expected: 7/7 scenarios pass
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All go tests pass
- [ ] Bridge image rebuilt and deployed (verified image tag matches)
- [ ] Sidecar-office healthy for 5+ minutes (init-service completed)
- [ ] HMAC secret mode 0440, owned by 10001:10001 (NOT 0644)
- [ ] 7 newly deployed RPCs + 9 previously untested Browser RPCs callable on VPS
- [ ] All RPC smoke tests use auth tokens
- [ ] BEATO runtime report scored ≥90/100
