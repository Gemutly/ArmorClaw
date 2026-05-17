# VPS ArmorClaw Agent-Driven Full Lifecycle

## TL;DR

> **Quick Summary**: Build a single non-interactive agent-driven flow that detects VPS topology, deploys/updates ArmorClaw over SSH, bootstraps Matrix test access via existing `cmd/bootstrap-admin` + dedicated test-user provisioning, validates core feature groups through Matrix-driven and RPC-backed checks, and produces a final machine-readable + human-readable report.
>
> **Deliverables**:
> - Fixed Docker runtime image with proper deps + CI runtime gate
> - Topology-aware deploy/update with HTTPS-first Bridge probing (not Matrix/Conduit)
> - Authenticated Matrix smoke that never silently skips (admin bootstrap + test-user + test-room + session persistence)
> - Fixed A0 RPC method discovery (103/0 mismatch resolved)
> - Feature group test suites A-D (Matrix control plane, Agent/Studio, Secretary, Trust/PII)
> - Expanded feature groups E-I with skip-disabled semantics
> - Top-level lifecycle orchestrator with `--phase` CLI from day one
> - `.skills/test-vps.yaml` agent-callable skill wrapper (shipped in Wave 2, not late)
> - Final report generation system (JSON + human-readable)
>
> **Estimated Effort**: Large (2 releases)
> **Parallel Execution**: YES - 5 waves
> **Critical Path**: T1 → T3 → T11 → T12 → T13-T16 → T21 → F1-F4

---

## Context

### Original Request
Build one non-interactive agent-callable flow that can, with minimal user input: detect VPS topology, deploy/update ArmorClaw over SSH, bootstrap Matrix access for the test agent, validate ArmorClaw features through Matrix-driven and RPC-backed checks, and produce a final report.

### Interview Summary
**Key Discussions**:
- Two-release strategy: Release 1 (fix broken deploy + authenticated smoke), Release 2 (scoped feature matrix A-D then E-I)
- Report step at end: machine-readable + human-readable with topology, deploy result, per-feature-group status, evidence paths, overall verdict
- Non-interactive from day one: safety comes from explicit mode flags (`--deploy-mode replace-existing --force`), not confirmation prompts
- HTTPS-first probing for Bridge only — internal Matrix/Conduit localhost checks stay HTTP (the field report showed Matrix healthy on 6167; the false-negative was Bridge HTTPS)
- Skip-disabled semantics for flag-gated features
- Minimal user input: SSH info + optional overrides only

**Research Findings**:
- `Dockerfile.quickstart`: 3-stage build, libolm-dev is build-time only, runtime missing libolm3 (but goolm is default — not blocking)
- Health check protocol INCONSISTENT: docker-compose=HTTP, a0/a2/contract=HTTPS(-k), a1=HTTP — fix for Bridge, leave Matrix as-is
- A0 uses single 5s attempt per method — likely cause of 103/0 mismatch
- `cmd/bootstrap-admin/main.go` is a first-boot tool: creates admin user via HMAC shared-secret, writes guard file at `/var/lib/armorclaw/.bootstrapped` — useful for admin bootstrap but NOT the same as test-user/session bootstrap
- 109 registered RPC methods across bridge, health, browser, skills, studio, secretary, device, invite, keystore, e2ee, voice, email
- Feature flags: ZeroTrustKeystore, VoicePipeline, MultiTabReplay, E2EEBackup
- `matrix-e2e/` harness is most complete test infra with Conduit lifecycle management
- `MautrixStoreAdapter` uses MemoryStore with Flush-only persistence — crypto sessions can be lost on crash

### Metis Review
**Identified Gaps** (addressed):
- libolm3 missing is MEDIUM not HIGH: goolm (pure Go) is default backend, doesn't need system lib → defensive fix only
- Crypto session state loss on crash: MemoryStore → Flush gap → bootstrap must verify crypto state
- Hardware-bound keystore fails on VPS migration: ARMORCLAW_KEYSTORE_SECRET must be set before update
- Bootstrap admin tool already exists: REUSE `cmd/bootstrap-admin` for admin identity, but create separate test-user bootstrap
- Secret risk classification is name-based: test both DENY and ALLOW paths, don't change behavior

### User Review Corrections (5 issues fixed)
1. **HTTPS-first for Bridge only**: Internal Matrix/Conduit localhost checks remain HTTP — only Bridge health probes use HTTPS-first
2. **Zero human intervention**: Replaced F3 "Real Manual QA" with "Automated Integration QA". Skill wrapper is non-interactive (uses `--mode ... --force`, not `confirm`)
3. **Skill/orchestrator CLI alignment**: Orchestrator has `--phase` as first-class CLI from T11. Skill wrapper calls matching `--phase` subcommands.
4. **Admin vs test-user bootstrap split**: T8 = admin bootstrap (using cmd/bootstrap-admin). T9 = test-user + test-room + session bootstrap. Separated because admin identity ≠ test session.
5. **Skill wrapper ships in Wave 2**: `.skills/test-vps.yaml` created alongside orchestrator, even if thin initially (smoke + a-d only)

---

## Work Objectives

### Core Objective
Create a single non-interactive agent-driven VPS lifecycle flow that deploys/updates ArmorClaw over SSH, bootstraps Matrix test access, validates core feature groups, and produces a comprehensive report — with minimal user input and zero human-in-the-loop.

### Concrete Deliverables
- `Dockerfile.quickstart` runtime stage fix (defensive libolm3 addition)
- CI runtime gate in `.github/workflows/dockerhub.yml`
- `scripts/lib/topology.sh` — topology detection module
- `scripts/lib/probe.sh` — HTTPS-first health probing module (Bridge only)
- Fixed `scripts/a0_discover.sh` with proper time budgets and classification
- Fixed `scripts/a1_deploy.sh` with HTTPS-first Bridge probing
- `scripts/lib/admin-bootstrap.sh` — admin identity bootstrap (using cmd/bootstrap-admin semantics)
- `scripts/lib/test-session-bootstrap.sh` — test-user + test-room + session bootstrap
- `scripts/lib/matrix-state.sh` — Matrix test state persistence
- `scripts/vps-lifecycle.sh` — top-level lifecycle orchestrator with `--phase` CLI
- `.skills/test-vps.yaml` — agent-callable skill wrapper (non-interactive, ships in Wave 2)
- `scripts/lib/report.sh` — report generation infrastructure
- `scripts/feature-groups/group-a-matrix.sh` through `group-i-flags.sh`

### Definition of Done

#### Release 1 done when
- [ ] Fresh Docker deploy starts Bridge successfully
- [ ] CI image-level runtime gate catches broken images before push
- [ ] Mixed VPS topology detected and classified correctly
- [ ] HTTPS-first probing works for Bridge (HTTP untouched for Matrix/Conduit localhost)
- [ ] API key env var contract normalized with backward-compatible fallback
- [ ] A0 shows responding methods on a healthy bridge (was 0/109)
- [ ] Admin bootstrap creates Conduit admin (using cmd/bootstrap-admin when available)
- [ ] Test-user/session bootstrap creates dedicated test user + tagged test room
- [ ] Matrix state persists across reruns (no repeated MATRIX_PASSWORD gaps)
- [ ] Lifecycle orchestrator runs detect → deploy → bootstrap → validate → report non-interactively (Release 1 validate = `--mode smoke` scope: topology checks + Bridge health + authenticated Matrix smoke + A0 sanity; NOT the full A–I feature group matrix)
- [ ] `.skills/test-vps.yaml` invokes smoke lifecycle flow non-interactively
- [ ] Zero confirmation prompts in the entire flow
- [ ] CLI contract consistent: `--mode` = smoke|full, `--deploy-mode` = replace-existing|..., `--force` for safety bypass

#### Release 2 done when
- [ ] Feature groups A-D pass end-to-end
- [ ] Feature groups E-I run with skip-disabled semantics
- [ ] Final report produced in JSON and human-readable formats with per-group breakdown
- [ ] Trust/PII tests (Group D) pin exact approval/event triggers and success conditions
- [ ] `.skills/test-vps.yaml` supports full mode with all feature groups

### Must Have
- Minimal user input (SSH info + optional overrides)
- Non-interactive: safety from explicit mode flags, not confirmation prompts
- SSH-based deploy or update
- Matrix-authenticated validation path (never optional for smoke)
- Admin bootstrap separate from test-user/session bootstrap
- Explicit topology detection before any changes
- HTTPS-aware probing for Bridge only (Matrix/Conduit localhost stays HTTP)
- Fail-fast on hard blockers with evidence preservation
- Per-feature-group reporting (pass/fail/skip-disabled/not-run)
- Agent-callable skill wrapper shipping alongside orchestrator
- skip-disabled semantics for flag-gated features
- Crypto session state verification after test-session bootstrap
- ARMORCLAW_KEYSTORE_SECRET persisted before any update flow
- `--phase` as first-class orchestrator CLI from day one

### Must NOT Have (Guardrails)
- Do NOT treat legacy healthy services as proof that new Docker artifact works
- Do NOT keep Matrix auth optional for smoke mode
- Do NOT apply HTTPS-first probing to Matrix/Conduit localhost checks
- Do NOT use confirmation prompts — use explicit mode flags (`--deploy-mode replace-existing --force`)
- Do NOT conflate admin bootstrap with test-user/session bootstrap
- Do NOT run destructive admin/approval tests against arbitrary live state
- Do NOT claim "all features" unless report shows coverage by feature group
- Do NOT introduce a giant new framework before deploy path is stable
- Do NOT build a new admin bootstrap tool — reuse `cmd/bootstrap-admin` semantics
- Do NOT change the name-based secret risk classification behavior
- Do NOT implement full rollback in Release 1 (fail-fast + evidence only)
- Do NOT touch SQLCipher, Matrix control plane, or approval flow

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** - ALL verification is agent-executed. No exceptions.
> No confirmation prompts. No manual steps. Safety = explicit mode flags.

### Test Decision
- **Infrastructure exists**: YES (bash test harnesses in tests/, matrix-e2e/)
- **Automated tests**: YES (Tests-after — each feature group gets its own test suite)
- **Framework**: bash + curl + jq (matching existing test harness patterns)
- **Evidence**: Stored in `.sisyphus/evidence/vps-lifecycle/`

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **VPS/SSH**: Use Bash/SSH automation — Run commands on VPS, validate output, capture logs
- **API/RPC**: Use Bash (curl) — Send JSON-RPC requests, assert status + response fields
- **Matrix**: Use Bash (curl to Matrix API) — Login, send, poll, assert message delivery
- **Docker**: Use Bash — Build, start, health-check, inspect containers
- **Report**: Use Bash — Run orchestrator, parse JSON output, assert required fields

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — unblock reality):
├── Task 1:  Fix Dockerfile.quickstart runtime deps [quick]
├── Task 2:  Add CI image-level runtime gate [quick]
├── Task 3:  Build topology detection module [unspecified-high]
├── Task 4:  Build HTTPS-first probe module (Bridge only) [quick]
└── Task 5:  Normalize API key env var contract [quick]

Wave 2 (After Wave 1 — restore trustworthy smoke + orchestrator + skill):
├── Task 6:  Investigate A0 103/0 mismatch [deep]
├── Task 7:  Fix A0 based on investigation (depends: 6) [unspecified-high]
├── Task 8:  Admin bootstrap module (depends: 4) [unspecified-high]
├── Task 9:  Test-user/session bootstrap module (depends: 8) [unspecified-high]
├── Task 10: Matrix test state persistence (depends: 9) [quick]
├── Task 11: Lifecycle orchestrator with --phase CLI (depends: 3,4,5,7,8,9,10) [deep]
└── Task 12: .skills/test-vps.yaml skill wrapper (depends: 11) [quick]

Wave 3 (After Wave 2 — core feature validation + report infra):
├── Task 13: Report generation infrastructure (depends: 11) [unspecified-high]
├── Task 14: Feature Group A — Matrix control plane (depends: 11) [unspecified-high]
├── Task 15: Feature Group B — Agent lifecycle / Studio (depends: 11) [unspecified-high]
├── Task 16: Feature Group C — Secretary workflows (depends: 11) [unspecified-high]
└── Task 17: Feature Group D — Trust / PII / approvals (depends: 11) [unspecified-high]

Wave 4 (After Wave 3 — expanded coverage):
├── Task 18: Feature Groups E+F — Email + Sidecar (depends: 13) [unspecified-high]
├── Task 19: Feature Groups G+H — Browser/Jetski + Event bus (depends: 13) [unspecified-high]
└── Task 20: Feature Group I — Flag-gated features w/ skip-disabled (depends: 13) [unspecified-high]

Wave 5 (After Wave 4 — final report step):
└── Task 21: Final report aggregation + evidence packaging (depends: 13-20) [unspecified-high]

Wave FINAL (After ALL tasks — 4 automated reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Automated integration QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: T1 → T3 → T11 → T13 → T21 → F1-F4
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 5 (Waves 1 and 3)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | - | 2 | 1 |
| 2 | 1 | 11 | 1 |
| 3 | - | 11 | 1 |
| 4 | - | 8, 11 | 1 |
| 5 | - | 11 | 1 |
| 6 | - | 7 | 2 |
| 7 | 6 | 11 | 2 |
| 8 | - | 9, 11 | 2 |
| 9 | 8 | 10, 11 | 2 |
| 10 | 9 | 11 | 2 |
| 11 | 3,4,5,7,8,9,10 | 12-17 | 2 |
| 12 | 11 | F1-F4 | 2 |
| 13 | 11 | 18-20, 21 | 3 |
| 14 | 11 | 21 | 3 |
| 15 | 11 | 21 | 3 |
| 16 | 11 | 21 | 3 |
| 17 | 11 | 21 | 3 |
| 18 | 13 | 21 | 4 |
| 19 | 13 | 21 | 4 |
| 20 | 13 | 21 | 4 |
| 21 | 13-20 | F1-F4 | 5 |

### Agent Dispatch Summary

- **Wave 1**: 5 tasks — T1-T2 → `quick`, T3 → `unspecified-high`, T4-T5 → `quick`
- **Wave 2**: 7 tasks — T6 → `deep`, T7-T9 → `unspecified-high`, T10 → `quick`, T11 → `deep`, T12 → `quick`
- **Wave 3**: 5 tasks — T13-T17 → `unspecified-high`
- **Wave 4**: 3 tasks — T18-T20 → `unspecified-high`
- **Wave 5**: 1 task — T21 → `unspecified-high`
- **FINAL**: 4 tasks — F1 → `oracle`, F2+F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Fix Dockerfile.quickstart runtime dependencies

  **What to do**:
  - Add `libolm3` to the runtime stage of `Dockerfile.quickstart` (after line ~200 where `libsqlcipher0` and `libyara10` are installed)
  - This is a defensive fix — goolm (pure Go) is the default backend and doesn't need the system library. Do NOT assume libolm3 is the root cause of any issue. Adding it ensures future compatibility if `-tags libolm` is ever used.
  - Also verify `ca-certificates` is present (needed for HTTPS probing to external endpoints)

  **Must NOT do**:
  - Do NOT change the build stage (libolm-dev is already there)
  - Do NOT modify the Go build tags or switch from goolm to libolm
  - Do NOT assume libolm3 is the root cause of deployment failures

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3, 4, 5)
  - **Blocks**: Task 2
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `Dockerfile.quickstart:193-210` — Runtime stage `RUN apt-get install` block. Add `libolm3` alongside `libsqlcipher0` and `libyara10`.

  **API/Type References**:
  - `bridge/pkg/crypto/olm_backend_goolm.go` — Pure Go backend (default, no system lib needed)
  - `bridge/pkg/crypto/olm_backend_libolm.go` — libolm backend (requires `//go:build libolm` tag, NOT currently set)

  **WHY Each Reference Matters**:
  - `Dockerfile.quickstart:193-210`: Exact location to add the package
  - `olm_backend_goolm.go`: Confirms goolm is the default — fix is defensive, not blocking

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Docker image builds with libolm3 in runtime
    Tool: Bash
    Steps:
      1. Run: `docker build -f Dockerfile.quickstart -t armorclaw-libolm-test .`
      2. Run: `docker run --rm armorclaw-libolm-test dpkg -l libolm3`
      3. Assert: output contains `ii  libolm3`
    Expected Result: libolm3 present in runtime image
    Failure Indicators: `dpkg-query: package 'libolm3' is not installed`
    Evidence: .sisyphus/evidence/task-1-dockerfile-fix.txt

  Scenario: Bridge binary still links correctly after change
    Tool: Bash
    Steps:
      1. Run: `docker run --rm armorclaw-libolm-test ldd /opt/armorclaw/armorclaw-bridge 2>&1 | head -20`
      2. Assert: no "not found" lines for any library
    Expected Result: All shared library dependencies resolved
    Failure Indicators: Any `=> not found` lines
    Evidence: .sisyphus/evidence/task-1-ldd-check.txt
  ```

  **Commit**: YES (groups with T2)
  - Message: `fix(docker): add libolm3 runtime dep + image-level runtime gate`
  - Files: `Dockerfile.quickstart`, `.github/workflows/dockerhub.yml`

- [x] 2. Add CI image-level runtime gate

  **What to do**:
  - Add a new job step in `.github/workflows/dockerhub.yml` after image build, before push:
    1. Run `ldd` on the bridge binary inside the container
    2. Start container with `ARMORCLAW_SKIP_DOCKER_CHECK=true`
    3. Verify Bridge process starts (stays running 10 seconds)
    4. Verify `/health` answers internally
  - Gate push on this new runtime gate passing

  **Must NOT do**:
  - Do NOT remove existing smoke test — enhance it
  - Do NOT add heavy integration tests to CI

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T3,T4,T5 — but depends on T1 for testing)
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 11 (indirectly)
  - **Blocked By**: Task 1

  **References**:

  **Pattern References**:
  - `.github/workflows/dockerhub.yml:85-115` — Existing `test-image` job. Enhance this.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: CI workflow has runtime gate steps
    Tool: Bash
    Steps:
      1. Assert: `grep -c "ldd" .github/workflows/dockerhub.yml` >= 1
      2. Assert: `grep "needs:" .github/workflows/dockerhub.yml | grep -c "test-image"` >= 1
    Expected Result: ldd + health gate before push
    Failure Indicators: grep returns 0
    Evidence: .sisyphus/evidence/task-2-ci-gate.yaml
  ```

  **Commit**: YES (groups with T1)
  - Message: `fix(docker): add libolm3 runtime dep + image-level runtime gate`
  - Files: `Dockerfile.quickstart`, `.github/workflows/dockerhub.yml`

- [x] 3. Build topology detection module

  **What to do**:
  - Create `scripts/lib/topology.sh` as a sourced library:
    - `_topology_detect()` — SSH to VPS and detect: systemd bridge, Conduit container, quickstart container, occupied ports (6167, 8443, 8080, 8448, 5000), existing `.env` with API keys, docker-compose installations
    - `_topology_classify()` — classify as: `fresh`, `native-systemd`, `docker-conduit`, `docker-quickstart`, `mixed`, `unknown`
    - `_topology_recommend_mode()` — recommend: `replace-existing`, `reuse-existing-matrix`, `side-by-side`, `fresh-install`
  - Output structured JSON via `_topology_to_json()`
  - Read-only — no changes on VPS during detection

  **Must NOT do**:
  - Do NOT make changes on the VPS during detection
  - Do NOT install new tools on the VPS

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 11
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `scripts/lib/contract.sh:30-68` — SSH helper pattern `_ssh_vps()`
  - `scripts/a0_discover.sh:50-90` — Topology probing: port detection, Docker container listing
  - `deploy/health-check.sh:1-100` — Service detection patterns

  **WHY Each Reference Matters**:
  - `contract.sh:30-68`: SSH command wrapper to use for all VPS commands
  - `a0_discover.sh:50-90`: Existing probing code to extract and formalize

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Topology detection classifies a mixed VPS correctly
    Tool: Bash
    Steps:
      1. Source: `source scripts/lib/topology.sh`
      2. Run: `_topology_detect --vps-ip <IP> --ssh-key <KEY>`
      3. Assert: JSON contains classification, occupied ports, detected services
    Expected Result: Mixed topology correctly identified
    Failure Indicators: Classification "fresh" on deployed VPS
    Evidence: .sisyphus/evidence/task-3-topology-detect.json

  Scenario: Fresh VPS detected as fresh
    Tool: Bash
    Steps:
      1. Run against fresh VPS
      2. Assert: classification is "fresh", ports empty, recommend_mode is "fresh-install"
    Expected Result: No false positives on empty VPS
    Failure Indicators: Classifies empty VPS as having services
    Evidence: .sisyphus/evidence/task-3-topology-fresh.json
  ```

  **Commit**: YES (groups with T4, T5)
  - Message: `feat(scripts): topology detection, Bridge HTTPS probing, env normalization`
  - Files: `scripts/lib/topology.sh`

- [x] 4. Build HTTPS-first probe module (Bridge only)

  **What to do**:
  - Create `scripts/lib/probe.sh` as a sourced library:
    - `_probe_bridge_health()` — tries HTTPS first (`curl -sf -k https://localhost:${PORT}/health`), falls back to HTTP
    - `_probe_bridge_rpc()` — tries HTTPS first for Bridge JSON-RPC, falls back to HTTP
    - `_probe_scheme_detect()` — returns "https" or "http" for a given host:port
  - Returns response + detected scheme
  - Update `scripts/a1_deploy.sh` to use `_probe_bridge_health()` for Bridge health checks
  - Do NOT touch Matrix/Conduit localhost checks — those stay HTTP
  - Do NOT update `deploy/quickstart-entrypoint.sh` Matrix checks — leave as HTTP

  **Must NOT do**:
  - Do NOT apply HTTPS-first to Matrix/Conduit localhost checks
  - Do NOT change a0_discover.sh (already uses HTTPS — correct)
  - Do NOT change a2_provision.sh bridge probes (already HTTPS — correct)
  - Do NOT change `deploy/quickstart-entrypoint.sh` Matrix checks

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 8, Task 11
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `scripts/a0_discover.sh:44` — Correct HTTPS probe pattern to generalize: `curl -sf -k "https://localhost:${BRIDGE_PORT}/health"`
  - `scripts/a1_deploy.sh:61` — BUG to fix: `curl http://localhost:${BRIDGE_PORT}/health` (HTTP-only Bridge probe)
  - `scripts/lib/contract.sh:78-108` — HTTP-only `_contract_wait_http()` — enhance Bridge variant only

  **WHY Each Reference Matters**:
  - `a0_discover.sh:44`: Correct pattern to extract for Bridge probing
  - `a1_deploy.sh:61`: The bug — probes HTTP against HTTPS-only Bridge
  - `contract.sh:78-108`: Existing HTTP wait helper — enhance for Bridge use only

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Bridge probe detects HTTPS-only bridge
    Tool: Bash
    Steps:
      1. Source: `source scripts/lib/probe.sh`
      2. Run: `_probe_bridge_health "localhost" "8443"`
      3. Assert: return code 0, detected scheme "https"
    Expected Result: HTTPS-only Bridge probed via HTTPS
    Failure Indicators: Non-zero return or scheme "http" on HTTPS-only bridge
    Evidence: .sisyphus/evidence/task-4-https-bridge.txt

  Scenario: Bridge probe falls back to HTTP when HTTPS fails
    Tool: Bash
    Steps:
      1. Run: `_probe_bridge_health "localhost" "8080"` (Bridge on HTTP)
      2. Assert: return code 0, detected scheme "http"
    Expected Result: HTTP Bridge probed via fallback
    Failure Indicators: Non-zero return when Bridge running on HTTP
    Evidence: .sisyphus/evidence/task-4-http-fallback.txt

  Scenario: Matrix localhost checks remain HTTP (not affected)
    Tool: Bash
    Steps:
      1. Run: `grep "probe_matrix\|_probe_health.*6167" scripts/lib/probe.sh`
      2. Assert: no matches — module does not touch Matrix probing
    Expected Result: Module scoped to Bridge only
    Failure Indicators: Matrix/Conduit probe functions found in probe.sh
    Evidence: .sisyphus/evidence/task-4-no-matrix-touch.txt
  ```

  **Commit**: YES (groups with T3, T5)
  - Message: `feat(scripts): topology detection, Bridge HTTPS probing, env normalization`
  - Files: `scripts/lib/probe.sh`, `scripts/a1_deploy.sh`

- [x] 5. Normalize API key env var contract

  **What to do**:
  - Add helper to `scripts/lib/contract.sh`:
    ```bash
    _get_api_key() {
      local key="${ZAI_API_KEY:-${API_KEY:-}}"
      if [[ -z "$key" ]]; then
        echo "ERROR: ZAI_API_KEY or API_KEY must be set" >&2
        return 1
      fi
      echo "$key"
    }
    ```
  - Update scripts that read API key to use the helper: a0, a1, a2, vps-validate, vps-matrix-cli-test, deploy/health-check

  **Must NOT do**:
  - Do NOT remove `API_KEY` backward compat
  - Do NOT rename env vars in existing `.env` files

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 11
  - **Blocked By**: None

  **References**:
  - `scripts/lib/contract.sh` — Shared helper library. Add alongside `_ssh_vps`, `_contract_bridge_rpc`.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Helper reads ZAI_API_KEY, falls back to API_KEY, errors on neither
    Tool: Bash
    Steps:
      1. ZAI_API_KEY="test" → `_get_api_key` returns "test"
      2. unset ZAI_API_KEY; API_KEY="fallback" → returns "fallback"
      3. Both unset → stderr contains "must be set", exit 1
    Expected Result: Three cases handled correctly
    Evidence: .sisyphus/evidence/task-5-api-key.txt
  ```

  **Commit**: YES (groups with T3, T4)
  - Message: `feat(scripts): topology detection, Bridge HTTPS probing, env normalization`
  - Files: `scripts/lib/contract.sh`, updated scripts

- [x] 6. Investigate A0 103/0 mismatch

  **What to do**:
  - Create `scripts/a0_investigate.sh`:
    1. For each of 109 registered RPC methods: capture raw curl response with verbose output, measure timing, save to `.sisyphus/evidence/a0-investigate/{method}.json`
    2. Test with multiple timeout budgets: 5s (current), 10s, 30s
    3. Test both HTTP and HTTPS transports
    4. Classify ROOT CAUSE: `timeout`, `parsing`, `transport`, `method-missing`, `param-required`
  - Output structured investigation report (JSON) with `dominant_root_cause`

  **Must NOT do**:
  - Do NOT fix A0 yet — just investigate

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T8)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 7
  - **Blocked By**: None

  **References**:
  - `scripts/a0_discover.sh:118-192` — KNOWN_METHODS array + classification loop
  - `scripts/lib/contract.sh:38-68` — 5s timeout (`-m 5`) — primary suspect
  - `bridge/pkg/rpc/server.go:1222-1329` — Full RPC dispatch table (ground truth)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Investigation produces per-method findings with timing
    Tool: Bash
    Steps:
      1. Run investigation against healthy VPS bridge
      2. Assert: summary.json has `total_methods`, `dominant_root_cause`, per-method timing
    Expected Result: Structured root cause data
    Evidence: .sisyphus/evidence/task-6-investigation-summary.json

  Scenario: Root cause identified (not "unknown")
    Tool: Bash
    Steps:
      1. `jq '.dominant_root_cause' summary.json`
      2. Assert: non-empty, not "unknown"
    Expected Result: Clear root cause
    Evidence: .sisyphus/evidence/task-6-root-cause.txt
  ```

  **Commit**: YES
  - Message: `investigate(scripts): A0 RPC 103/0 mismatch root cause analysis`
  - Files: `scripts/a0_investigate.sh`

- [x] 7. Fix A0 based on investigation findings

  **What to do**:
  - Based on T6 results, fix `scripts/a0_discover.sh`:
    - If timeout: increase from 5s to appropriate value
    - If parsing: fix classification logic lines 165-182
    - If transport: integrate `scripts/lib/probe.sh` for HTTPS-first
    - If param-required: ensure error responses classified as "found" not "timeout"
  - Update `scripts/lib/contract.sh` timeout/retry values
  - Preserve raw sample evidence

  **Must NOT do**:
  - Do NOT change KNOWN_METHODS list
  - Do NOT remove manifest output format

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (after T6)
  - **Blocks**: Task 11
  - **Blocked By**: Task 6

  **References**:
  - `scripts/a0_discover.sh:158-192` — Classification logic to fix
  - `scripts/lib/contract.sh:38-68` — Timeout/retry logic (primary suspect)
  - `.sisyphus/evidence/a0-investigate/summary.json` — T6 results

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: A0 discovers responding methods on healthy bridge
    Tool: Bash
    Steps:
      1. Run fixed A0 against healthy VPS
      2. Assert: `jq '.methods | map(select(.status == "responds")) | length'` > 0
    Expected Result: >0 responding methods (was 0/109)
    Failure Indicators: Still 0
    Evidence: .sisyphus/evidence/task-7-a0-fixed.json
  ```

  **Commit**: YES
  - Message: `fix(scripts): A0 RPC method classification — correct timeout + parsing`
  - Files: `scripts/a0_discover.sh`, `scripts/lib/contract.sh`

- [x] 8. Admin bootstrap module (using cmd/bootstrap-admin)

  **What to do**:
  - Create `scripts/lib/admin-bootstrap.sh`:
    - `_admin_bootstrap()` — ensures an admin user exists on the Conduit:
      1. Check if Conduit is running (HTTP localhost:6167 — NOT HTTPS-first)
      2. Check if admin already bootstrapped (guard file `/var/lib/armorclaw/.bootstrapped`)
      3. **If `cmd/bootstrap-admin` binary exists on the VPS**: invoke it directly over SSH. It handles HMAC-SHA1 registration, guard file, and idempotency natively. This is the preferred path — reduces drift risk.
      4. **If `cmd/bootstrap-admin` is NOT available**: emulate its behavior via shell (register admin via HMAC-SHA1 shared-secret API calls, write guard file). This is the fallback path.
      5. Return admin identity (user_id, password from cmd/bootstrap-admin or registration). **Do NOT assume access_token comes from the tool.** Perform an explicit Matrix login step (`POST /_matrix/client/v3/login` with `m.login.password`) to obtain the access_token used by later phases.
    - `_admin_is_bootstrapped()` — check guard file
  - This is for ADMIN IDENTITY only — creating the Conduit admin user. NOT for test-user/session.
  - Works over SSH to VPS

  **Must NOT do**:
  - Do NOT create test users here (that's T9)
  - Do NOT create test rooms here (that's T9)
  - Do NOT persist sessions here (that's T10)
  - Do NOT use HTTPS for Conduit localhost checks

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T6, T7)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 9
  - **Blocked By**: None (admin bootstrap talks to Conduit on localhost:6167 via HTTP — no probe module needed)

  **References**:

  **Pattern References**:
  - `bridge/cmd/bootstrap-admin/main.go` — Existing first-boot tool: HMAC-SHA1 registration, guard file, Conduit on localhost:6167. Pattern to REUSE.
  - `tests/matrix-e2e/lib/conduit.sh:80-140` — `conduit_register()` HMAC shared-secret registration. Same API calls.

  **WHY Each Reference Matters**:
  - `cmd/bootstrap-admin/main.go`: The existing tool — replicate its API calls, not the tool itself (since we run over SSH)
  - `conduit.sh:80-140`: Battle-tested Conduit registration pattern

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Admin bootstrap creates admin on fresh Conduit
    Tool: Bash
    Steps:
      1. Run admin bootstrap against VPS with Conduit but no admin
      2. Assert: returns admin user_id + password (from cmd/bootstrap-admin or registration)
      3. Assert: explicit Matrix login step performed, access_token obtained
      4. Verify: `curl http://localhost:6167/_matrix/client/v3/account/whoami -H 'Authorization: Bearer <token>'` returns 200 with matching user_id
    Expected Result: Admin registered, access_token obtained via explicit login (not from tool)
    Evidence: .sisyphus/evidence/task-8-admin-bootstrap.json

  Scenario: Admin bootstrap is idempotent (guard file)
    Tool: Bash
    Steps:
      1. Run admin bootstrap twice against same VPS
      2. Assert: second run returns cached/same credentials without re-registering
      3. Assert: guard file exists at /var/lib/armorclaw/.bootstrapped
    Expected Result: Idempotent, no duplicate users
    Evidence: .sisyphus/evidence/task-8-admin-idempotent.json
  ```

  **Commit**: YES (groups with T9, T10)
  - Message: `feat(scripts): admin bootstrap + test-session bootstrap + state persistence`
  - Files: `scripts/lib/admin-bootstrap.sh`

- [x] 9. Test-user/session bootstrap module

  **What to do**:
  - Create `scripts/lib/test-session-bootstrap.sh`:
    - `_test_session_bootstrap()` — creates a dedicated non-admin test user and session:
      1. Register test user using **HMAC-SHA1 shared-secret registration** — the same mechanism used by `conduit.sh:conduit_register()` and `cmd/bootstrap-admin`. This is the battle-tested path that works with Conduit's registration API. Do NOT assume a generic "admin token can register users" path — Conduit requires the shared-secret HMAC mechanism when `allow_registration = false`.
         - **Source of CONDUIT_REGISTRATION_SECRET**: Read from the deployed Conduit config file on the VPS (`/etc/armorclaw/conduit.toml` → `[global]` → `registration_shared_secret`), or from the Conduit container environment (`docker exec <conduit-container> env | grep REGISTRATION`). If neither source yields the secret, fail bootstrap immediately with a clear blocker message in the report: `"CONDUIT_REGISTRATION_SECRET not found — check conduit.toml or container env"`. Do not proceed with registration without it.
         - Compute HMAC-SHA1 of the desired username using the retrieved secret
         - `POST /_matrix/client/r0/register` with `{"username": "armorclaw-vps-test", "password": "<generated>", "auth": {"type": "m.login.dummy", "mac": "<hmac_hex>"}}`
      2. Login as test user via `POST /_matrix/client/v3/login` with `m.login.password` → capture access_token, device_id, refresh_token
       3. Create or reuse a tagged test room (e.g., room name `#armorclaw-vps-test-*` with creation timestamp). The `--bootstrap-admin-token` (obtained from T8's explicit admin login) is used **exclusively for bootstrap-only room/bot management** — room creation, bot invitation, and other one-time setup actions. It is NOT used for user registration, test validation traffic, or any operation after bootstrap completes.
      5. Verify crypto session state via **concrete observable signals** (NOT "CryptoEngine non-nil" — bash over SSH cannot inspect process memory):
         - **Signal 1**: Successful `/_matrix/client/v3/login` as test user → captures access_token (proves auth works)
         - **Signal 2**: Successful `/_matrix/client/v3/sync` with `since=""` → returns initial sync data (proves session is live)
         - **Signal 3**: Successful `/_matrix/client/v3/account/whoami` with test token → returns user_id (proves token valid)
         - **Signal 4**: Send a test message to test room → poll for delivery via sync → message appears in timeline (proves send/receive round-trip works)
         - If Bridge logs are accessible: check for crypto store initialization log line (e.g., "OlmMachine initialized" or "crypto store loaded")
         - If ANY signal fails: log the failure, mark `crypto_state_verified: false`, continue (do not block bootstrap)
     - Admin credentials from T8 used for **bootstrap-only room/bot management** (room creation, bot invitation), NOT for user registration or test validation traffic
    - Creates REUSABLE tagged test room (not create/delete each run — gives evidence continuity)
  - This is separate from admin bootstrap because:
    - Admin identity ≠ test session
    - Test user needs specific room, crypto state, and session persistence
    - Guardrails say smoke must not use arbitrary live admin flows

  **Must NOT do**:
  - Do NOT create admin users here (that's T8)
  - Do NOT use bootstrap admin token for test operations (use dedicated test user — admin token is for bootstrap-only room/bot management)
  - Do NOT delete test rooms on each run (reuse tagged rooms)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (after T8)
  - **Blocks**: Task 10
  - **Blocked By**: Task 8

  **References**:

  **Pattern References**:
  - `tests/matrix-e2e/lib/conduit.sh:140-200` — `conduit_login()`, `conduit_create_room()`, `conduit_invite_bot()`
  - `scripts/vps-matrix-cli-test.sh:100-200` — `matrix_login()`, `matrix_send()`, `matrix_poll_notice()`
  - `bridge/pkg/crypto/store_adapter.go` — MemoryStore + Flush pattern. Must verify crypto state after setup.

  **WHY Each Reference Matters**:
  - `conduit.sh:140-200`: Room creation and bot invitation patterns
  - `vps-matrix-cli-test.sh:100-200`: VPS-specific Matrix API calls
  - `store_adapter.go`: Crypto gap — must verify session state

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Test session bootstrap creates user + room + session
    Tool: Bash
    Steps:
      1. Run: `_test_session_bootstrap --vps-ip <IP> --bootstrap-admin-token <TOKEN>`
      2. Assert: returns test_user_id, test_room_id, access_token
      3. Assert: test room has bridge bot as member
      4. Assert: crypto_state_verified is true (based on 4 concrete signals: login, sync, whoami, send/receive)
      5. Signal 1 verify: `curl -sf http://localhost:6167/_matrix/client/v3/account/whoami -H 'Authorization: Bearer <test_token>'` returns 200 with matching user_id
      6. Signal 4 verify: send "hello" to test room, poll sync, assert message in timeline
      7. Verify: --bootstrap-admin-token was used ONLY for room creation / bot invitation — no admin token appears in any validation traffic
    Expected Result: Full test session with concrete crypto verification
    Failure Indicators: Missing fields, any signal fails, bot not in room
    Evidence: .sisyphus/evidence/task-9-test-session.json

  Scenario: Test session reuses existing tagged room on rerun
    Tool: Bash
    Steps:
      1. Run bootstrap twice
      2. Assert: same room_id on both runs
      3. Assert: no duplicate rooms created
    Expected Result: Reuses tagged room for evidence continuity
    Failure Indicators: New room created each run
    Evidence: .sisyphus/evidence/task-9-room-reuse.json
  ```

  **Commit**: YES (groups with T8, T10)
  - Message: `feat(scripts): admin bootstrap + test-session bootstrap + state persistence`
  - Files: `scripts/lib/test-session-bootstrap.sh`

- [x] 10. Matrix test state persistence

  **What to do**:
  - Create `scripts/lib/matrix-state.sh`:
    - `_matrix_save_state()` — save to `.sisyphus/evidence/vps-lifecycle/matrix-state.json`: access_token, refresh_token, device_id, user_id, test_room_id, conduit_base_url, bootstrap_timestamp, crypto_state_verified
    - `_matrix_load_state()` — load from persisted state
    - `_matrix_state_is_valid()` — verify token still valid via `whoami`
    - `_matrix_refresh_token()` — refresh if expired
  - File permissions: chmod 600 (tokens, not passwords)
  - On reruns: load → validate → only re-bootstrap if invalid

  **Must NOT do**:
  - Do NOT store passwords (tokens only)
  - Do NOT make file world-readable

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (after T9)
  - **Blocks**: Task 11
  - **Blocked By**: Task 9

  **References**:
  - `scripts/a_run_all.sh` — `_contract_save()` pattern
  - `bridge/internal/adapter/matrix.go:276` — Token fields: access_token, user_id, device_id, refresh_token

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: State round-trips with secure permissions
    Tool: Bash
    Steps:
      1. Save state with test values
      2. Load state → assert values match
      3. Assert: file permissions are 600
    Expected Result: Secure round-trip
    Evidence: .sisyphus/evidence/task-10-state.txt
  ```

  **Commit**: YES (groups with T8, T9)
  - Message: `feat(scripts): admin bootstrap + test-session bootstrap + state persistence`
  - Files: `scripts/lib/matrix-state.sh`

- [x] 11. Lifecycle orchestrator with --phase CLI

  **What to do**:
  - Create `scripts/vps-lifecycle.sh` with `--phase` as first-class CLI:
    ```
    Usage: vps-lifecycle.sh [OPTIONS]
      --vps-ip IP          VPS IP address (required)
      --ssh-key PATH       SSH key path (required)
      --ssh-user USER      SSH user (default: root)
      --phase PHASE        detect | deploy | admin-bootstrap | test-bootstrap | validate | report | all (default: all)
      --mode MODE          smoke | full (default: smoke)
      --deploy-mode MODE   replace-existing | reuse-existing-matrix | side-by-side | fresh-install
      --force              Skip safety confirmations (REQUIRED for non-interactive)
      --feature-groups     Comma-separated groups (default: a-d for full)
      --report-format      json | text | json+text (default: json+text)
      --output-dir DIR     Evidence output directory
      --skip-deploy        Skip deploy/update
      --skip-bootstrap     Skip bootstrap, use persisted state
    ```
  - Phase flow when `--phase all`:
    1. **detect** — topology detection (T3)
    2. **deploy** — deploy/update based on topology (fail-fast without `--force` + explicit `--deploy-mode`)
    3. **admin-bootstrap** — admin user setup (T8)
    4. **test-bootstrap** — test user/room/session (T9/T10)
    5. **validate** — feature group validation. **Release 1 scope**: topology checks + Bridge health (via `_probe_bridge_health`) + authenticated Matrix smoke (login → `/status` → `/help` → send/receive round-trip) + A0 sanity (verify >0 responding methods). **Release 2 scope**: expands to feature groups A–I via `scripts/feature-groups/group-*.sh`. The `--mode smoke` flag selects Release 1 scope; `--mode full` selects Release 2 scope.
    6. **report** — aggregate and emit report (T13/T21)
  - `--force` replaces all confirmation prompts: without it, ambiguous topology fails fast with evidence
  - Exits: 0=pass, 1=fail, 2=partial
  - Writes evidence to `.sisyphus/evidence/vps-lifecycle/`

  **Must NOT do**:
  - Do NOT add confirmation prompts — use `--force` + explicit `--deploy-mode`
  - Do NOT support `full` mode until feature groups A-D exist (T14-T17)
  - Do NOT make destructive changes without `--force`

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (depends on T3,T4,T5,T7,T8,T9,T10)
  - **Blocks**: Tasks 12-17
  - **Blocked By**: Tasks 3, 4, 5, 7, 8, 9, 10

  **References**:

  **Pattern References**:
  - `scripts/a_run_all.sh` — Master E2E runner (A0-A4). Phase orchestration pattern.
  - `scripts/vps-validate.sh` — Existing smoke/full modes + JSON report pattern.

  **API/Type References**:
  - `scripts/lib/topology.sh` (T3) — `_topology_detect()`, `_topology_classify()`
  - `scripts/lib/probe.sh` (T4) — `_probe_bridge_health()`
  - `scripts/lib/admin-bootstrap.sh` (T8) — `_admin_bootstrap()`
  - `scripts/lib/test-session-bootstrap.sh` (T9) — `_test_session_bootstrap()`
  - `scripts/lib/matrix-state.sh` (T10) — `_matrix_load_state()`, `_matrix_state_is_valid()`

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: --phase all runs full lifecycle non-interactively
    Tool: Bash
    Steps:
      1. Run: `bash scripts/vps-lifecycle.sh --vps-ip <IP> --ssh-key <KEY> --phase all --mode smoke --force`
      2. Assert: exit 0 or 2
      3. Assert: report.json exists with all phase sections
    Expected Result: Full lifecycle, zero prompts
    Evidence: .sisyphus/evidence/task-11-full-e2e.json

  Scenario: Fails fast on ambiguous topology without --force
    Tool: Bash
    Steps:
      1. Run without --force on mixed topology
      2. Assert: exit 1, message says "explicit --deploy-mode required" or "use --force"
    Expected Result: No destructive changes without explicit consent via flags
    Evidence: .sisyphus/evidence/task-11-fail-fast.txt

  Scenario: Individual phases work independently
    Tool: Bash
    Steps:
      1. `--phase detect` → outputs topology JSON only
      2. `--phase report` → aggregates existing evidence into report
    Expected Result: Each phase runnable standalone
    Evidence: .sisyphus/evidence/task-11-phases.txt
  ```

  **Commit**: YES (groups with T12)
  - Message: `feat(scripts): lifecycle orchestrator + skill wrapper`
  - Files: `scripts/vps-lifecycle.sh`

- [x] 12. .skills/test-vps.yaml skill wrapper

  **What to do**:
  - Create `.skills/test-vps.yaml` following existing schema:
    - All `automation: "auto"` — NO `confirm` steps (non-interactive)
    - Calls `scripts/vps-lifecycle.sh` with matching `--phase` subcommands
    - Safety via `--force` flag (always set) and explicit `--deploy-mode`
    - Parameters: `vps_ip`, `ssh_key_path`, `mode` (smoke/full), `deploy_mode`, `feature_groups`
    - Steps match orchestrator phases: detect → deploy → admin-bootstrap → test-bootstrap → validate → report
  - Ships in Wave 2 even if thin (smoke + detect only initially). Grows as features land.

  **Must NOT do**:
  - Do NOT use `automation: "confirm"` anywhere — non-interactive
  - Do NOT duplicate orchestrator logic — just call `vps-lifecycle.sh`

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (after T11)
  - **Blocks**: F1-F4
  - **Blocked By**: Task 11

  **References**:
  - `.skills/TEMPLATE.yaml` — Schema
  - `.skills/e2e-deploy.yaml` — Closest existing skill (multi-phase VPS)
  - `scripts/vps-lifecycle.sh` (T11) — The orchestrator this wraps

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Skill YAML valid and non-interactive
    Tool: Bash
    Steps:
      1. Parse YAML: valid
      2. `grep -c 'automation: "confirm"' .skills/test-vps.yaml` → 0
      3. `grep -c 'automation: "auto"' .skills/test-vps.yaml` → >= 5
      4. All steps reference `scripts/vps-lifecycle.sh --phase <name>`
    Expected Result: Valid, non-interactive skill
    Evidence: .sisyphus/evidence/task-12-skill.txt
  ```

  **Commit**: YES (groups with T11)
  - Message: `feat(scripts): lifecycle orchestrator + skill wrapper`
  - Files: `.skills/test-vps.yaml`

- [x] 13. Report generation infrastructure

  **What to do**:
  - Create `scripts/lib/report.sh` as a sourced library:
    - `_report_init()`, `_report_add_phase()`, `_report_add_feature_group()`, `_report_set_verdict()`
    - `_report_emit_json()` — JSON report with: topology, deploy_mode, fresh_deploy_result, existing_install_result, matrix_bootstrap_result, feature_groups (per-group matrix with pass/fail/skip-disabled/not-run), blockers, evidence_paths, overall_verdict
    - `_report_emit_text()` — Human-readable with visual bar chart per group (████░░░░), blockers section, evidence index
    - Report must NOT collapse into one score until feature coverage is trustworthy

  **Must NOT do**:
  - Do NOT collapse all results into a single score
  - Do NOT produce report without per-feature-group breakdown

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T14-T17)
  - **Blocks**: Tasks 18-20, Task 21
  - **Blocked By**: Task 11

  **References**:
  - `scripts/vps-validate.sh:250-305` — Existing report pattern
  - `jetski/internal/sonar/reporter.go` — Atomic write: temp file + rename

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Report produces valid JSON + readable text with groups A-I
    Tool: Bash
    Steps:
      1. Init, add phase, add groups, set verdict, emit both formats
      2. `jq .` parses JSON
      3. Text contains VERDICT, all 9 groups, bar charts
    Expected Result: Both formats with complete structure
    Evidence: .sisyphus/evidence/task-13-report.json

  Scenario: Empty groups show NOT RUN, verdict not "pass"
    Tool: Bash
    Steps:
      1. Emit with no groups added
      2. Assert: groups show NOT RUN, verdict is "blocked" or "partial"
    Expected Result: Incomplete coverage reflected in verdict
    Evidence: .sisyphus/evidence/task-13-report-empty.txt
  ```

  **Commit**: YES
  - Message: `feat(scripts): report generation infrastructure`
  - Files: `scripts/lib/report.sh`

- [x] 14. Feature Group A — Matrix control plane tests

  **What to do**:
  - Create `scripts/feature-groups/group-a-matrix.sh`:
    1. Matrix login — `matrix.login` RPC or Matrix API
    2. Send/receive in test room — verify delivery via sync
    3. `/status` command — verify m.notice response
    4. `/help` command — verify response
    5. One studio command — `!agent list`
    6. One secretary command — `!secretary status`
  - Uses authenticated session from T9/T10 (never skips auth)

  **Must NOT do**:
  - Do NOT run fixture-admin flows (that's Group D)
  - Do NOT skip auth

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T13, T15, T16, T17)
  - **Blocks**: Task 21
  - **Blocked By**: Task 11

  **References**:
  - `scripts/vps-matrix-cli-test.sh:200-400` — Smoke mode test patterns
  - `tests/matrix-e2e/cases/test-status.sh` — E2E /status test

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All Group A tests pass on healthy deployment
    Tool: Bash
    Steps:
      1. Run group A against healthy VPS
      2. Assert: 6/6 pass with evidence
    Expected Result: 6/6 pass
    Evidence: .sisyphus/evidence/task-14-group-a.json

  Scenario: Handles missing bridge gracefully
    Tool: Bash
    Steps:
      1. Run with bridge stopped
      2. Assert: tests fail (not skip), no hangs
    Expected Result: Clear failure messages
    Evidence: .sisyphus/evidence/task-14-group-a-no-bridge.json
  ```

  **Commit**: YES (groups with T15-T17)
  - Message: `feat(tests): feature group test suites A-D`
  - Files: `scripts/feature-groups/group-a-matrix.sh`

- [x] 15. Feature Group B — Agent lifecycle / Studio tests

  **What to do**:
  - Create `scripts/feature-groups/group-b-studio.sh`:
    1. Deploy/create agent via `studio.deploy`
    2. List agents — verify test agent appears
    3. Start agent — verify running
    4. Observe agent — check status
    5. Stop/delete agent — cleanup
  - All via RPC over SSH. Cleanup guaranteed.

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 21
  - **Blocked By**: Task 11

  **References**:
  - `tests/test-vps-smoke.sh:159-198` — RPC via SSH pattern
  - `bridge/pkg/rpc/server.go` — studio.deploy, studio.stats

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Agent lifecycle completes, no artifacts left
    Tool: Bash
    Steps:
      1. Run group B
      2. Assert: 5/5 pass
      3. After: verify no test agents remain
    Expected Result: Full lifecycle, clean cleanup
    Evidence: .sisyphus/evidence/task-15-group-b.json
  ```

  **Commit**: YES (groups with T14, T16, T17)
  - Message: `feat(tests): feature group test suites A-D`
  - Files: `scripts/feature-groups/group-b-studio.sh`

- [x] 16. Feature Group C — Secretary workflows tests

  **What to do**:
  - Create `scripts/feature-groups/group-c-secretary.sh`:
    1. Start workflow
    2. Blocked workflow path — verify blocks
    3. User response path — approve via Matrix, verify advances
    4. Workflow completion — verify completes
  - Clean up test workflows

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 21
  - **Blocked By**: Task 11

  **References**:
  - `tests/matrix-e2e/cases/test-secretary-commands.sh`
  - `tests/matrix-e2e/cases/test-approve-reject.sh`
  - `bridge/pkg/secretary/` — Workflow engine

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Secretary workflow blocked→approved→done
    Tool: Bash
    Steps:
      1. Run group C
      2. Assert: 4/4 pass, workflow_id captured, approval advances
    Expected Result: Full secretary lifecycle
    Evidence: .sisyphus/evidence/task-16-group-c.json
  ```

  **Commit**: YES (groups with T14, T15, T17)
  - Message: `feat(tests): feature group test suites A-D`
  - Files: `scripts/feature-groups/group-c-secretary.sh`

- [x] 17. Feature Group D — Trust / PII / approvals tests

  **What to do**:
  - Create `scripts/feature-groups/group-d-trust.sh` — pin tests to the **actual approval/event contract** from the secretary and trust subsystems:
    1. **Approval publication** — trigger an action that requires approval via `secretary.start_workflow` with a PII-gated template. Verify: workflow enters `blocked` state, an approval request event is emitted to Matrix room, event contains `approval_id` and `approval_type`.
    2. **Approve handling** — send approval via `device.approve` RPC or Matrix approval response. Verify: workflow advances past blocker, completion event emitted, approval state recorded in audit trail.
    3. **Reject handling** — trigger new approval, reject via `device.reject` RPC. Verify: workflow enters `rejected` state, action blocked, rejection event emitted with rejection reason.
    4. **Fail-closed behavior** — trigger approval, do NOT respond. Verify: after timeout, workflow enters `failed` state (not `approved`), fail-closed event emitted. Check that no action proceeded without explicit approval.
    5. **Secret classification** — test the actual `secret_approval.go` contract:
       - Trigger secret access with name containing `payment_card_number` → assert: `risk_level: "high"`, `decision: "deny"`, approval required
       - Trigger secret access with name `user_preference` → assert: `risk_level: "low"`, `decision: "allow"`, auto-approved
       - Document in test output: classification is name-based via `strings.Contains(lower, "payment")`
  - Each test pins the exact trigger (RPC method + params) and exact success condition (state transition + event emission + audit trail)

  **Must NOT do**:
  - Do NOT change name-based secret classification
  - Do NOT test against production PII

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 21
  - **Blocked By**: Task 11

  **References**:
  - `bridge/pkg/capability/secret_approval.go` — Name-based classification
  - `bridge/pkg/rpc/server.go` — device.approve/reject, invite.create/revoke

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Approval flow + fail-closed + secret classification
    Tool: Bash
    Steps:
      1. Run group D
      2. Assert: 5/5 pass (approve, reject, fail-closed, deny, allow)
    Expected Result: Full approval lifecycle including fail-closed
    Evidence: .sisyphus/evidence/task-17-group-d.json
  ```

  **Commit**: YES (groups with T14-T16)
  - Message: `feat(tests): feature group test suites A-D`
  - Files: `scripts/feature-groups/group-d-trust.sh`

- [x] 18. Feature Groups E+F — Email + Sidecar

  **What to do**:
  - `scripts/feature-groups/group-e-email.sh`: email pipeline health, outbound HITL, timeout/audit
  - `scripts/feature-groups/group-f-sidecar.sh`: sidecar health, happy-path extraction, fallback/error
  - Both use skip-disabled when service not present

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with T19, T20)
  - **Blocks**: Task 21
  - **Blocked By**: Task 13

  **References**:
  - `bridge/pkg/rpc/server.go` — email_approval_status, email.list_pending
  - `deploy/docker-compose.sidecar-py.yml`

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Email pipeline tested, sidecar skip-disabled when absent
    Tool: Bash
    Steps:
      1. Run groups E+F
      2. Assert: email tests report pass/fail (not not-run) when configured
      3. Assert: sidecar tests report skip-disabled when not deployed
    Expected Result: Correct reporting per service availability
    Evidence: .sisyphus/evidence/task-18-groups-ef.json
  ```

  **Commit**: YES (groups with T19, T20)
  - Message: `feat(tests): feature group test suites E-I`
  - Files: `scripts/feature-groups/group-e-email.sh`, `scripts/feature-groups/group-f-sidecar.sh`

- [x] 19. Feature Groups G+H — Browser/Jetski + Event bus

  **What to do**:
  - `scripts/feature-groups/group-g-browser.sh`: Jetski availability, one browser action, diagnostics
  - `scripts/feature-groups/group-h-events.sh`: event emission, event capture

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4
  - **Blocks**: Task 21
  - **Blocked By**: Task 13

  **References**:
  - `bridge/pkg/rpc/server.go` — browser.status, browser.navigate, browser.list
  - `jetski/internal/rpc/rpc.go` — Jetski RPC

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Browser tested when available, events round-trip
    Tool: Bash
    Steps:
      1. Run groups G+H
      2. Assert: availability test works, event emission→capture verified
    Expected Result: Browser + event verification
    Evidence: .sisyphus/evidence/task-19-groups-gh.json
  ```

  **Commit**: YES (groups with T18, T20)
  - Message: `feat(tests): feature group test suites E-I`
  - Files: `scripts/feature-groups/group-g-browser.sh`, `scripts/feature-groups/group-h-events.sh`

- [x] 20. Feature Group I — Flag-gated features with skip-disabled

  **What to do**:
  - Create `scripts/feature-groups/group-i-flags.sh`:
    1. ZeroTrustKeystore (if enabled): keystore.unseal, keystore.sealed, keystore.session_status
    2. VoicePipeline (if not "off"): voice.status
    3. E2EEBackup (if enabled): e2ee.backup_exists
    4. MultiTabReplay (if enabled): browser.replay_diagnostics
  - Each reads feature flag, then: test OR skip-disabled (NOT fail)

  **Must NOT do**:
  - Do NOT enable disabled features
  - Do NOT fail for intentionally disabled features

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4
  - **Blocks**: Task 21
  - **Blocked By**: Task 13

  **References**:
  - `bridge/pkg/config/config.go:59-73` — FeatureFlags: ZeroTrustKeystore, VoicePipeline, MultiTabReplay, E2EEBackup

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All disabled features report skip-disabled (not fail)
    Tool: Bash
    Steps:
      1. Run group I with all flags off
      2. Assert: 4/4 skip-disabled, details explain "flag is false/off"
    Expected Result: No failures for disabled features
    Evidence: .sisyphus/evidence/task-20-group-i-skip.json

  Scenario: Mixed enabled/disabled reported correctly
    Tool: Bash
    Steps:
      1. Run with some flags on
      2. Assert: enabled tested (pass/fail), disabled skip-disabled
    Expected Result: Correct per-flag reporting
    Evidence: .sisyphus/evidence/task-20-group-i-mixed.json
  ```

  **Commit**: YES (groups with T18, T19)
  - Message: `feat(tests): feature group test suites E-I`
  - Files: `scripts/feature-groups/group-i-flags.sh`

- [x] 21. Final report aggregation + evidence packaging

  **What to do**:
  - Enhance `scripts/vps-lifecycle.sh --phase report`:
    1. Collect all phase results from `.sisyphus/evidence/vps-lifecycle/`
    2. Aggregate per-group results (A-I): pass/fail/skip-disabled/not-run counts
    3. Compute verdict: `pass` (all tested groups pass), `partial` (some fails, non-blocking), `fail` (A-D failures), `blocked` (deploy failed)
    4. Collect evidence index
    5. Generate JSON + text reports via `_report_emit_json/text()` (T13)
    6. Optional: tar.gz evidence directory
    7. Exit: 0=pass, 1=fail, 2=partial
  - This is the explicit "report step at the end"
  - Report shows what actually happened. Untested groups = "not-run" (not "pass")

  **Must NOT do**:
  - Do NOT collapse into single score without group breakdown
  - Do NOT delete evidence after packaging

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 5 (after ALL other tasks)
  - **Blocks**: F1-F4
  - **Blocked By**: Tasks 13-20

  **References**:
  - `scripts/lib/report.sh` (T13) — Report infrastructure
  - Evidence files from T3, T8-T11, T14-T20

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Complete report with all 9 groups, JSON + text
    Tool: Bash
    Steps:
      1. Run: `--phase report` after full lifecycle
      2. Assert: report.json has all 9 groups + verdict
      3. Assert: report.txt has visual bars + VERDICT line
      4. Assert: exit code reflects actual result (0/1/2)
    Expected Result: Complete, honest report
    Evidence: .sisyphus/evidence/task-21-final-report.json

  Scenario: Partial run shows not-run for untested groups
    Tool: Bash
    Steps:
      1. Run smoke mode (A-D only)
      2. Report: E-I show "not-run", verdict is "partial" (not "pass")
    Expected Result: Incomplete coverage NOT masked as success
    Evidence: .sisyphus/evidence/task-21-partial-report.json
  ```

  **Commit**: YES
  - Message: `feat(scripts): final report aggregation + evidence packaging`
  - Files: `scripts/vps-lifecycle.sh`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 automated review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
>
> **ZERO confirmation prompts during execution. Safety = explicit mode flags.**

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Review all changed shell scripts with `shellcheck`. Review Dockerfile with `hadolint`. Check for: unset variables, missing error handling, hardcoded paths, unquoted expansions. Verify all curl calls have `-f`/`--fail` or explicit status checking. Check AI slop: excessive comments, over-abstraction, generic variable names.
  Output: `Shellcheck [PASS/FAIL] | Hadolint [PASS/FAIL] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Automated Integration QA** — `unspecified-high`
  Start from clean state. Execute the FULL lifecycle non-interactively via `vps-lifecycle.sh --vps-ip <IP> --ssh-key <KEY> --mode full --force`. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration: detect → deploy → admin-bootstrap → test-bootstrap → validate A–I → report in ONE run. (With `--mode full`, the validate phase must exercise ALL feature groups A through I — not just the smoke subset.) Test edge cases: no SSH access, wrong port, HTTPS-only bridge, missing env vars, ambiguous topology (--force should override). Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes. Verify no confirmation prompts exist anywhere in the flow.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

| Wave | Commit | Message | Files |
|------|--------|---------|-------|
| 1 | T1+T2 | `fix(docker): add runtime deps + image gate` | `Dockerfile.quickstart`, `.github/workflows/dockerhub.yml` |
| 1 | T3+T4+T5 | `feat(scripts): topology detection, Bridge HTTPS probing, env normalization` | `scripts/lib/topology.sh`, `scripts/lib/probe.sh`, relevant deploy scripts |
| 2 | T6+T7 | `fix(scripts): investigate and fix A0 RPC method classification` | `scripts/a0_discover.sh`, `scripts/lib/contract.sh` |
| 2 | T8+T9+T10 | `feat(scripts): admin bootstrap + test-session bootstrap + state persistence` | `scripts/lib/admin-bootstrap.sh`, `scripts/lib/test-session-bootstrap.sh`, `scripts/lib/matrix-state.sh` |
| 2 | T11+T12 | `feat(scripts): lifecycle orchestrator + skill wrapper` | `scripts/vps-lifecycle.sh`, `.skills/test-vps.yaml` |
| 3 | T13 | `feat(scripts): report generation infrastructure` | `scripts/lib/report.sh` |
| 3 | T14-T17 | `feat(tests): feature group test suites A-D` | `scripts/feature-groups/group-*.sh` |
| 4 | T18-T20 | `feat(tests): feature group test suites E-I` | `scripts/feature-groups/group-*.sh` |
| 5 | T21 | `feat(scripts): final report aggregation + evidence packaging` | `scripts/vps-lifecycle.sh` (report phase) |

---

## Success Criteria

### Verification Commands
```bash
# Docker image builds and starts
docker build -f Dockerfile.quickstart -t armorclaw-test . && \
  docker run --rm -e ARMORCLAW_SKIP_DOCKER_CHECK=true armorclaw-test health-check

# Topology detection works
bash scripts/lib/topology.sh --dry-run

# HTTPS-first Bridge probing (NOT Matrix)
source scripts/lib/probe.sh && _probe_health "localhost" "8443"  # Bridge: HTTPS-first
# Matrix localhost stays HTTP: curl -sf http://localhost:6167/_matrix/client/versions

# A0 discovers responding methods on healthy bridge
bash scripts/a0_discover.sh --vps-ip <IP> --ssh-key <KEY> | jq '.methods | map(select(.status == "responds")) | length'

# Full lifecycle non-interactive
bash scripts/vps-lifecycle.sh --vps-ip <IP> --ssh-key <KEY> --mode full --force

# Phase-specific execution
bash scripts/vps-lifecycle.sh --vps-ip <IP> --ssh-key <KEY> --phase detect
bash scripts/vps-lifecycle.sh --vps-ip <IP> --ssh-key <KEY> --phase deploy --deploy-mode replace-existing --force
bash scripts/vps-lifecycle.sh --vps-ip <IP> --ssh-key <KEY> --phase admin-bootstrap
bash scripts/vps-lifecycle.sh --vps-ip <IP> --ssh-key <KEY> --phase test-bootstrap
bash scripts/vps-lifecycle.sh --vps-ip <IP> --ssh-key <KEY> --phase validate --feature-groups a-d
bash scripts/vps-lifecycle.sh --vps-ip <IP> --ssh-key <KEY> --phase report --report-format json+text

# Skill wrapper invokes the flow
# (via .skills/test-vps.yaml — non-interactive)

# Feature groups report
jq '.feature_groups' .sisyphus/evidence/vps-lifecycle/report.json
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] Docker image starts Bridge without crashes
- [ ] Mixed VPS topologies classified correctly
- [ ] A0 shows responding methods on healthy bridge (109 registered methods)
- [ ] Matrix smoke never silently skips
- [ ] HTTPS-first probing for Bridge only (Matrix localhost untouched)
- [ ] Admin bootstrap and test-session bootstrap are separate modules
- [ ] Feature groups A-D pass
- [ ] Feature groups E-I report skip-disabled where applicable
- [ ] Final report includes topology + deploy result + per-group status + evidence paths + verdict
- [ ] .skills/test-vps.yaml invokes the full lifecycle flow non-interactively
- [ ] Reruns require only SSH info + minimal optional overrides
- [ ] Zero confirmation prompts in the entire flow
