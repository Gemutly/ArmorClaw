# VPS ArmorClaw Agent-Driven Full Lifecycle Plan

## TL;DR

> **Quick Summary**: Fix the Docker deploy pipeline, make the A-pipeline scripts topology-aware with HTTPS probing, fix A0 discovery classification, add authenticated Matrix smoke testing, and build a top-level lifecycle orchestrator — all in Release 1. Release 2 expands to full feature matrix (groups A-I) and adds an agent-callable skill wrapper.
> 
> **Deliverables**:
> - Working Docker image that doesn't crash on VPS
> - Topology-aware deploy script (HTTPS detection, port conflict checks, env normalization shim)
> - Fixed A0 discovery (responds_with_error classification, retry budget 1→2)
> - Authenticated Matrix smoke test (non-optional auth)
> - Top-level lifecycle orchestrator (discover → deploy → update → validate → report)
> - Feature matrix expansion for groups A-D then E-I (Release 2)
> - `.skills/test-vps.yaml` agent-callable wrapper (Release 2)
> 
> **Estimated Effort**: Large (2 releases, 7 phases)
> **Parallel Execution**: YES - 3 waves in Release 1 Phase 1-2
> **Critical Path**: Task 1 (VPS state probe) → Task 2 (Dockerfile fix) → Task 4 (HTTPS probe) → Task 5 (topology deploy) → Task 6 (A0 fix) → Task 8 (auth Matrix smoke) → Task 9 (orchestrator) → Task 10 (report)

---

## Context

### Original Request
User wants a comprehensive VPS ArmorClaw agent-driven full lifecycle system that: detects topology, deploys/updates over SSH, bootstraps Matrix, validates features, and produces machine+human-readable reports. Split into 2 releases: Release 1 fixes deploy reliability + authenticated smoke; Release 2 expands to full feature matrix.

### Interview Summary
**Key Discussions**:
- Split into 7 phases across 2 releases
- Release 1: Fix Docker image crash, topology-aware deploy, HTTPS probing, A0 discovery fix, authenticated Matrix smoke, top-level orchestrator
- Release 2: Feature groups A-D then E-I, skill wrapper
- Brownfield-only — minimal patches, no rewrites
- Fail-fast + evidence preservation, no complex rollback in Release 1
- `skip-disabled` semantics for flag-gated features
- Report step at the end of lifecycle flow
- Do NOT treat legacy healthy services as proof new Docker artifact works
- Do NOT keep Matrix auth optional for smoke mode

**Research Findings** (5 parallel explore agents):
- **Dockerfile**: `libolm3` NOT needed — build uses `goolm` (pure Go Olm). Crash likely from ldconfig/ld-linux path or image arch mismatch, not missing package. `libolm-dev` is a build-time leftover.
- **A0 RPC**: 0/103 methods classified as "responding" — semantic bug: `.error` with empty `{}` classified as "error" not "responding". `max_retries=1` too tight. curl uses `-sf -k -m 5` against HTTPS — fails on HTTP-only Bridge.
- **HTTPS gaps**: `_contract_wait_http()` line 98 in contract.sh is HTTP-only (P0 blocker). ~16 locations total (not 40). Matrix/Conduit HTTP calls are CORRECT — do NOT touch them.
- **Test infrastructure**: Very rich — A0-A4 chaining, 40+ test scripts, 11+ assertion helpers, full Matrix client library.
- **Env vars**: 3 naming conventions coexist. Need translation shim, not codebase-wide rename.

### Metis Review
**Identified Gaps** (all addressed):
- **VPS state unknown**: Added Task 1 (VPS state discovery) as P0 gate before all other work
- **Port ambiguity (8080 vs 8443)**: VPS state probe resolves this
- **`.env` has real secrets**: Guardrail: never commit `.env`, all work uses `.env.example`
- **Report generation already exists**: Extend `vps-validate.sh`'s `generate_report()`, don't build new
- **A-pipeline scripts ARE mutable**: Can modify `a*.sh` and `scripts/lib/*.sh` — only architecture constraints (SQLCipher, Matrix, approval flow) are immutable
- **Concurrent execution risk**: Add `flock` to orchestrator
- **Data migration on update**: Save current image tag before update, no schema migration in Release 1

---

## Work Objectives

### Core Objective
Create a fully agent-driven VPS lifecycle that can: discover what's running, deploy/update the ArmorClaw Docker image, bootstrap Matrix with authenticated smoke, validate feature groups, and produce machine-readable reports — all without human intervention.

### Concrete Deliverables
- `Dockerfile.quickstart` fix that produces a runnable image on VPS
- `_contract_wait_http()` enhanced with HTTPS support in `scripts/lib/contract.sh`
- `_normalize_env()` translation shim in `scripts/lib/contract.sh`
- `_check_port_conflict()` function for deploy safety
- `a0_discover.sh` classification fix + retry budget increase
- Authenticated Matrix smoke in `vps-matrix-cli-test.sh` (auth required, not optional)
- `scripts/lifecycle.sh` top-level orchestrator (discover → deploy → update → validate → report)
- Release 2: feature matrix groups A-D, then E-I
- `.skills/test-vps.yaml` agent-callable wrapper

### Definition of Done
- [ ] `docker run --rm mikegemut/armorclaw:latest /opt/armorclaw/armorclaw-bridge --version` returns version on VPS
- [ ] `./scripts/lifecycle.sh discover` exits 0 with topology JSON
- [ ] `./scripts/lifecycle.sh deploy` exits 0 with healthy containers
- [ ] `./scripts/lifecycle.sh validate` produces report.json with `overall_score ≥ 80`
- [ ] `./scripts/lifecycle.sh report` produces timestamped evidence in `.sisyphus/evidence/`
- [ ] A0 discovery classifies methods with "responds_with_error" status
- [ ] Matrix smoke test requires authentication (no optional path)

### Must Have
- Docker image runs without crash on VPS (5.183.11.149)
- HTTPS probing for Bridge endpoints (not Matrix/Conduit internals)
- Topology detection before any deploy action
- Authenticated Matrix smoke — non-optional
- Fail-fast with evidence preservation
- Agent-callable lifecycle (exit codes, JSON output, no human input)

### Must NOT Have (Guardrails)
- NO commits of `.env` file (contains real secrets)
- NO destructive VPS operations without explicit topology+state check
- NO network access introduced to agent containers (NetworkMode: none is absolute)
- NO changes to Go bridge source code in Release 1
- NO modifications to Matrix/Conduit HTTP endpoint URLs (they are correct as http://localhost:${MATRIX_PORT})
- NO new report generation system (extend existing `generate_report()`)
- NO rewrite of A0 classification system (add new status + increase retries)
- NO TLS-everywhere refactor (fix Bridge health checks only, ~16 locations)
- NO env var rename across codebase (translation shim only)
- NO concurrent deploy (flock guard required)
- NO destructive operations without explicit `--force` flag + confirmation in non-smoke mode

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.
> Acceptance criteria requiring "user manually tests/confirms" are FORBIDDEN.

### Test Decision
- **Infrastructure exists**: YES (40+ test scripts, A0-A4 harness, assertion helpers)
- **Automated tests**: Tests-after (extend existing test suites after implementation)
- **Framework**: Bash test harness (existing `tests/test-*.sh` + `tests/lib/`)

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **VPS/Deploy**: Use Bash (SSH + curl) — Deploy, check health, validate responses
- **Script changes**: Use Bash — Run script with test args, check exit code + output
- **Docker**: Use Bash — Build, run, inspect, logs
- **Matrix**: Use Bash (curl + jq) — Register, login, send, sync, validate

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 0 (P0 Gate — VPS state discovery, blocks everything):
└── Task 1: VPS state probe [quick]

Wave 1 (Start after Wave 0 — foundation fixes, MAX PARALLEL):
├── Task 2: Dockerfile fix (depends: 1) [quick]
├── Task 3: Env var normalization shim (depends: 1) [quick]
├── Task 4: HTTPS probe function in contract.sh (depends: 1) [quick]
├── Task 5: Port conflict detection (depends: 1) [quick]
└── Task 6: A0 discovery classification fix (depends: 1) [unspecified-high]

Wave 2 (After Wave 1 — deploy + integration):
├── Task 7: Topology-aware deploy (depends: 2, 4, 5) [deep]
├── Task 8: Authenticated Matrix smoke (depends: 3, 4) [unspecified-high]
└── Task 9: Lifecycle orchestrator (depends: 7, 8) [deep]

Wave 3 (After Wave 2 — report + integration):
└── Task 10: Report step integration (depends: 9) [unspecified-high]

Wave FINAL (After ALL Release 1 tasks — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay

RELEASE 2 (after Release 1 verified):
├── Task 11: Feature group A — Matrix control plane [deep]
├── Task 12: Feature group B — Agent/Studio [unspecified-high]
├── Task 13: Feature group C — Secretary workflows [deep]
├── Task 14: Feature group D — Trust/PII [deep]
├── Task 15: Feature groups E-I expansion [unspecified-high]
└── Task 16: .skills/test-vps.yaml wrapper [quick]

Critical Path: Task 1 → Task 2 → Task 7 → Task 9 → Task 10 → F1-F4 → user okay
Parallel Speedup: ~60% faster than sequential (Wave 1 has 5 parallel tasks)
Max Concurrent: 5 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | — | 2,3,4,5,6 | 0 |
| 2 | 1 | 7 | 1 |
| 3 | 1 | 8 | 1 |
| 4 | 1 | 7, 8 | 1 |
| 5 | 1 | 7 | 1 |
| 6 | 1 | 9 | 1 |
| 7 | 2,4,5 | 9 | 2 |
| 8 | 3,4 | 9 | 2 |
| 9 | 6,7,8 | 10 | 2 |
| 10 | 9 | F1-F4 | 3 |
| 11-16 | F1-F4 | — | R2 |

### Agent Dispatch Summary

- **Wave 0**: 1 — T1 → `quick`
- **Wave 1**: 5 — T2 → `quick`, T3 → `quick`, T4 → `quick`, T5 → `quick`, T6 → `unspecified-high`
- **Wave 2**: 3 — T7 → `deep`, T8 → `unspecified-high`, T9 → `deep`
- **Wave 3**: 1 — T10 → `unspecified-high`
- **FINAL**: 4 — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`
- **Release 2**: 6 — T11 → `deep`, T12 → `unspecified-high`, T13 → `deep`, T14 → `deep`, T15 → `unspecified-high`, T16 → `quick`

---

## TODOs

- [ ] 1. VPS State Discovery Probe (P0 Gate)

  **What to do**:
  - Create `scripts/lib/vps-state.sh` with a `probe_vps_state()` function that SSHes into the VPS and collects:
    1. Running services: `docker ps --format '{{.Names}} {{.Status}}'` and `systemctl list-units --type=service --state=running | grep -E 'armorclaw|conduit|caddy|cloudflared'`
    2. Listening ports: `ss -tlnp | grep -E '8080|8443|6167|80|443'`
    3. Available tools: `which jq socat curl docker ssh websocat`
    4. Disk space: `df -h / | tail -1 | awk '{print $4}'`
    5. Docker version: `docker version --format '{{.Server.Version}}'`
    6. Current deployment mode: check for `ARMORCLAW_SERVER_MODE` in running container env, or detect from ports (8443+senty = sentinel, 8080 only = native)
    7. Current image tag: `docker inspect --format='{{.Config.Image}}' $(docker ps -q --filter name=armor) 2>/dev/null || echo "none"`
  - Output a JSON object to stdout: `{"mode":"sentinel","bridge_port":8080,"bridge_running":true,"bridge_image":"mikegemut/armorclaw:v4.6.0","matrix_port":6167,"matrix_running":true,"tools":{"jq":true,"socat":true,"curl":true,"docker":true,"websocat":false},"disk_free_gb":"42","docker_version":"29.2.1"}`
  - Exit 0 if SSH connection succeeds, exit 1 if unreachable
  - Save output to `.sisyphus/evidence/task-1-vps-state.json`

  **Must NOT do**:
  - Do NOT modify any existing scripts
  - Do NOT run any state-changing commands on the VPS
  - Do NOT commit the `.env` file

  **Post-Task 1 Investigation (blocks Task 2)**:
  After collecting VPS state, also capture the Docker crash diagnostics:
  1. `ssh root@5.183.11.149 "docker pull mikegemut/armorclaw:latest && docker run --rm mikegemut/armorclaw:latest /opt/armorclaw/armorclaw-bridge --version 2>&1"` — capture exact error message
  2. `ssh root@5.183.11.149 "docker run --rm --entrypoint ldd mikegemut/armorclaw:latest /opt/armorclaw/armorclaw-bridge 2>&1"` — check dynamic library dependencies
  3. `ssh root@5.183.11.149 "docker run --rm --entrypoint sh mikegemut/armorclaw:latest -c 'uname -m && cat /etc/os-release | head -2'"` — check image arch + OS
  Save output to `.sisyphus/evidence/task-1-docker-crash-diagnostics.txt` — this determines whether Task 2 is `ldconfig` fix or arch-mismatch fix

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single new file, well-defined bash function, clear output format
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (P0 gate — all other tasks depend on its results)
  - **Parallel Group**: Wave 0 (sequential)
  - **Blocks**: Tasks 2, 3, 4, 5, 6
  - **Blocked By**: None

  **References**:

  **Pattern References** (existing code to follow):
  - `scripts/lib/contract.sh:1-30` — Bash library pattern (sourcing, function naming, error handling)
  - `scripts/lib/load_env.sh` — Environment loader pattern, sources `.env`, provides helper functions

  **API/Type References**:
  - `.env` — Has VPS_IP, BRIDGE_PORT, ADMIN_TOKEN (DO NOT commit this file)

  **External References**:
  - VPS at 5.183.11.149, SSH key at `~/.ssh/openclaw_win`

  **WHY Each Reference Matters**:
  - `contract.sh` shows the bash library convention — new file must match (sourcing pattern, function naming `_prefix_*`)
  - `load_env.sh` shows how to source `.env` and export variables — follow this pattern
  - `.env` has the SSH connection params — use them, never modify or commit

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: VPS probe succeeds on reachable VPS
    Tool: Bash
    Preconditions: VPS at 5.183.11.149 is reachable via SSH
    Steps:
      1. Run: `source scripts/lib/vps-state.sh && probe_vps_state > /tmp/vps-state.json`
      2. Run: `jq -r '.bridge_running' /tmp/vps-state.json`
      3. Run: `jq -r '.tools.jq' /tmp/vps-state.json`
    Expected Result: JSON output with bridge_running=true/false, tools.jq=true/false, valid JSON structure
    Failure Indicators: SSH timeout, empty output, invalid JSON, exit code != 0
    Evidence: .sisyphus/evidence/task-1-vps-state-probe.json

  Scenario: VPS probe fails gracefully on unreachable host
    Tool: Bash
    Preconditions: SSH to a non-existent host (e.g., VPS_IP=127.0.0.1 with SSH key that doesn't work)
    Steps:
      1. Run: `VPS_IP=127.0.0.1 source scripts/lib/vps-state.sh && probe_vps_state`
      2. Check exit code
    Expected Result: Exit code 1, error message "SSH connection failed" to stderr
    Failure Indicators: Exit code 0, no error message, hang
    Evidence: .sisyphus/evidence/task-1-vps-state-unreachable.txt
  ```

  **Commit**: YES
  - Message: `fix(scripts): add VPS state discovery probe`
  - Files: `scripts/lib/vps-state.sh`
  - Pre-commit: `bash -n scripts/lib/vps-state.sh`

- [ ] 2. Dockerfile ldconfig Fix

  **What to do**:
  - Add `RUN ldconfig` after the `apt-get install` block in the runtime stage of `Dockerfile.quickstart` (around line 102-110)
  - This ensures the dynamic linker cache includes all installed libraries (particularly `libyara10` from backports which apt postinst may not properly register)
  - Remove `libolm-dev` from the build stage (line ~15) since `goolm` (pure Go) is used — it's a build-time leftover that inflates the image
  - Build and push the image to Docker Hub: `docker build -t mikegemut/armorclaw:latest -f Dockerfile.quickstart . && docker push mikegemut/armorclaw:latest`
  - Verify on VPS: `ssh root@5.183.11.149 "docker run --rm mikegemut/armorclaw:latest /opt/armorclaw/armorclaw-bridge --version"`

  **Must NOT do**:
  - Do NOT modify Go bridge source code
  - Do NOT remove SQLCipher or any actual runtime dependencies
  - Do NOT change the HEALTHCHECK directive

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 2-line Dockerfile change (ldconfig + remove libolm-dev), build + push + verify
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 3, 4, 5, 6)
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 7
  - **Blocked By**: Task 1

  **References**:

  **Pattern References**:
  - `Dockerfile.quickstart:90-120` — Runtime stage apt-get install block, this is where ldconfig goes after
  - `Dockerfile.quickstart:12-18` — Build stage with `libolm-dev` to remove

  **API/Type References**:
  - `bridge/pkg/olm/olm_backend_goolm.go` — Has `!libolm` build tag confirming goolm is the default (pure Go, no C lib needed)

  **External References**:
  - Docker Hub: `mikegemut/armorclaw:latest`
  - VPS: `ssh root@5.183.11.149`

  **WHY Each Reference Matters**:
  - Runtime stage lines 90-120 show exactly where `ldconfig` should be added (after apt install, before COPY)
  - `olm_backend_goolm.go` confirms `libolm-dev` is unused — safe to remove
  - Docker Hub is the deployment target — image must push successfully

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Docker image builds and runs on VPS
    Tool: Bash
    Preconditions: Docker build environment available, Docker Hub credentials configured
    Steps:
      1. Run: `docker build -t mikegemut/armorclaw:latest -f Dockerfile.quickstart . 2>&1 | tail -5`
      2. Run: `docker push mikegemut/armorclaw:latest`
      3. Run: `ssh root@5.183.11.149 "docker pull mikegemut/armorclaw:latest && docker run --rm mikegemut/armorclaw:latest /opt/armorclaw/armorclaw-bridge --version"`
    Expected Result: Build succeeds, push succeeds, VPS run returns version string (e.g., "v4.8.0")
    Failure Indicators: Build failure, push auth error, VPS container crash, "libolm.so.3: cannot open shared object file"
    Evidence: .sisyphus/evidence/task-2-docker-build.txt

  Scenario: Image size doesn't increase significantly
    Tool: Bash
    Preconditions: Previous image tag available
    Steps:
      1. Run: `docker images mikegemut/armorclaw:latest --format '{{.Size}}'`
      2. Compare with previous build (should be same or smaller since libolm-dev removed)
    Expected Result: Image size ≤ previous build size
    Failure Indicators: Image size > previous build (indicates unnecessary additions)
    Evidence: .sisyphus/evidence/task-2-image-size.txt
  ```

  **Commit**: YES
  - Message: `fix(docker): add ldconfig to runtime stage, remove unused libolm-dev`
  - Files: `Dockerfile.quickstart`
  - Pre-commit: `docker build --check -f Dockerfile.quickstart .`

- [ ] 3. Env Var Normalization Shim

  **What to do**:
  - Add a `_normalize_env()` bash function to `scripts/lib/contract.sh` that maps the 3 naming conventions:
    - Convention A (canonical): `ARMORCLAW_*` (e.g., `ARMORCLAW_ADMIN_TOKEN`)
    - Convention B (flat): `API_KEY`, `ADMIN_TOKEN`
    - Convention C (provider-native): `OPEN_AI_KEY`, `ZAI_API_KEY`, `OPENROUTER_API_KEY`
  - The shim reads whatever is set and exports the canonical forms:
    - `ADMIN_TOKEN` → `ARMORCLAW_ADMIN_TOKEN` (if ARMORCLAW_ADMIN_TOKEN not already set)
    - `API_KEY` → `OPENROUTER_API_KEY` (if OPENROUTER_API_KEY not already set)
    - `OPEN_AI_KEY` → kept as-is (provider-native, already correct)
    - `ZAI_API_KEY` → kept as-is (provider-native, already correct)
  - Add the shim call at the top of `scripts/a1_deploy.sh`, `scripts/a2_provision.sh`, `scripts/a3_events.sh`, and the new `scripts/lifecycle.sh`
  - Do NOT modify Go bridge config loading or Docker Compose files
  - Do NOT rename variables across the codebase

  **Must NOT do**:
  - Do NOT modify Go bridge source code
  - Do NOT modify Docker Compose files
  - Do NOT rename variables in existing scripts beyond adding the shim call

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single bash function addition + sourcing in 4 files
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2, 4, 5, 6)
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 8
  - **Blocked By**: Task 1

  **References**:

  **Pattern References**:
  - `scripts/lib/contract.sh:1-30` — Bash library pattern to follow for new function
  - `scripts/lib/load_env.sh` — Shows how env vars are sourced and exported

  **API/Type References**:
  - `.env` — Contains `API_KEY=...`, `ADMIN_TOKEN=...` (Convention B names)
  - `scripts/a1_deploy.sh` — Expects `OPENROUTER_API_KEY` (Convention C)

  **External References**:
  - `README.md:Environment Variables section` — Documents the 3 naming conventions

  **WHY Each Reference Matters**:
  - `contract.sh` is where all shared bash library functions live — new function goes here
  - `.env` shows the actual var names in use — shim must handle these exact names
  - `a1_deploy.sh` shows what the scripts expect — shim must produce these

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Shim maps flat env vars to canonical forms
    Tool: Bash
    Preconditions: .env loaded with ADMIN_TOKEN and API_KEY set
    Steps:
      1. Run: `source scripts/lib/contract.sh && _normalize_env && echo "ARMORCLAW_ADMIN_TOKEN=$ARMORCLAW_ADMIN_TOKEN" && echo "OPENROUTER_API_KEY=$OPENROUTER_API_KEY"`
    Expected Result: ARMORCLAW_ADMIN_TOKEN has value from ADMIN_TOKEN, OPENROUTER_API_KEY has value from API_KEY
    Failure Indicators: Empty values, wrong mapping, error messages
    Evidence: .sisyphus/evidence/task-3-env-shim-test.txt

  Scenario: Shim doesn't override already-set canonical vars
    Tool: Bash
    Preconditions: ARMORCLAW_ADMIN_TOKEN already set to a specific value
    Steps:
      1. Run: `export ARMORCLAW_ADMIN_TOKEN="test-value" && source scripts/lib/contract.sh && _normalize_env && echo "$ARMORCLAW_ADMIN_TOKEN"`
    Expected Result: "test-value" (not overridden by ADMIN_TOKEN)
    Failure Indicators: Value changed to something else
    Evidence: .sisyphus/evidence/task-3-env-shim-nooverride.txt
  ```

  **Commit**: YES
  - Message: `feat(scripts): add env var normalization shim`
  - Files: `scripts/lib/contract.sh`, `scripts/a1_deploy.sh`, `scripts/a2_provision.sh`, `scripts/a3_events.sh`
  - Pre-commit: `bash -n scripts/lib/contract.sh && bash -n scripts/a1_deploy.sh`

- [ ] 4. HTTPS Probe Function in contract.sh

  **What to do**:
  - Extend `_contract_wait_http()` in `scripts/lib/contract.sh` (currently at line ~98) to support HTTPS:
    - Add optional `scheme` parameter (default `http`)
    - Add `-k` flag for HTTPS (skip cert verification for self-signed/Cloudflare)
    - Update curl command from `curl -sf -m 5 http://...` to `curl -sf -k -m 5 ${scheme}://...`
  - Update the 3 call sites in `scripts/a1_deploy.sh` that call `_contract_wait_http()` for Bridge endpoints to use `scheme=https` when the Bridge is in sentinel mode (detected from env or topology)
  - Do NOT change Matrix/Conduit HTTP calls — they are correct as `http://localhost:${MATRIX_PORT}`
  - Do NOT change any other HTTP-only locations in this task (those are lower priority)

  **Must NOT do**:
  - Do NOT change Matrix/Conduit endpoint URLs
  - Do NOT refactor all 16 HTTP-only locations (only the P0 blocker + 3 Bridge call sites)
  - Do NOT remove HTTP support (must still work for native mode)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single function modification + 3 call site updates
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2, 3, 5, 6)
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 7, 8
  - **Blocked By**: Task 1

  **References**:

  **Pattern References**:
  - `scripts/lib/contract.sh:90-110` — Current `_contract_wait_http()` implementation (HTTP-only, line 98 is the curl call)
  - `scripts/a1_deploy.sh` — Contains 3 call sites to `_contract_wait_http()` for Bridge health checks

  **API/Type References**:
  - `scripts/a1_deploy.sh` — Call sites that need scheme parameter

  **WHY Each Reference Matters**:
  - Line 98 is the exact P0 blocker — the curl call that only does HTTP
  - `a1_deploy.sh` has the 3 Bridge-specific call sites that need HTTPS support

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: HTTPS probe succeeds on Bridge with self-signed cert
    Tool: Bash
    Preconditions: Bridge running on VPS at port 8443 with HTTPS
    Steps:
      1. Run: `source scripts/lib/contract.sh && _contract_wait_http "5.183.11.149" "8443" "/health" "https" 10`
      2. Check exit code
    Expected Result: Exit code 0, function returns successfully
    Failure Indicators: Exit code 1, timeout, curl SSL error without -k flag
    Evidence: .sisyphus/evidence/task-4-https-probe-bridge.txt

  Scenario: HTTP probe still works (backward compat)
    Tool: Bash
    Preconditions: Service running on localhost:8080 with HTTP
    Steps:
      1. Run: `source scripts/lib/contract.sh && _contract_wait_http "localhost" "8080" "/health" "http" 5`
      2. Check exit code
    Expected Result: Exit code 0 (HTTP mode still works as before)
    Failure Indicators: Exit code 1, curl error
    Evidence: .sisyphus/evidence/task-4-http-probe-compat.txt

  Scenario: HTTPS probe fails gracefully on unreachable host
    Tool: Bash
    Preconditions: No service on target port
    Steps:
      1. Run: `source scripts/lib/contract.sh && _contract_wait_http "localhost" "19999" "/health" "https" 2`
    Expected Result: Exit code 1, error message about timeout
    Failure Indicators: Exit code 0 (false positive), hang
    Evidence: .sisyphus/evidence/task-4-https-probe-fail.txt
  ```

  **Commit**: YES
  - Message: `fix(scripts): add HTTPS support to _contract_wait_http`
  - Files: `scripts/lib/contract.sh`, `scripts/a1_deploy.sh`
  - Pre-commit: `bash -n scripts/lib/contract.sh && bash -n scripts/a1_deploy.sh`

- [ ] 5. Port Conflict Detection

  **What to do**:
  - Add `_check_port_conflict()` function to `scripts/lib/contract.sh` that:
    1. Takes a port number as argument
    2. SSHes to VPS and checks `ss -tlnp | grep :${PORT}`
    3. If port is occupied, checks if it's occupied by the expected service (Bridge, Matrix, etc.)
    4. Returns 0 if port is free or occupied by expected service, 1 if conflict with unexpected process
    5. Outputs conflict info to stderr: "PORT_CONFLICT: port 8080 occupied by PID 12345 (nginx)"
  - Integrate into `scripts/a1_deploy.sh` deploy flow — run port checks BEFORE `docker compose up`
  - Exit with clear error message if conflict detected, preserving evidence

  **Must NOT do**:
  - Do NOT kill existing processes to resolve conflicts
  - Do NOT modify any running services on the VPS

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single function addition + integration in deploy script
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2, 3, 4, 6)
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 7
  - **Blocked By**: Task 1

  **References**:

  **Pattern References**:
  - `scripts/lib/contract.sh` — Where the function goes (follow naming convention `_check_*`)
  - `scripts/a1_deploy.sh:30-50` — Deploy flow where port check should be inserted (before docker compose up)

  **WHY Each Reference Matters**:
  - `contract.sh` is the shared library — all deploy helper functions go here
  - `a1_deploy.sh` lines 30-50 show the pre-deploy phase where the check should happen

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Port conflict detected on occupied port
    Tool: Bash
    Preconditions: Port 8080 occupied on VPS by nginx (or any non-Bridge process)
    Steps:
      1. Run: `source scripts/lib/contract.sh && _check_port_conflict "5.183.11.149" "8080" "bridge"`
    Expected Result: Exit code 1, stderr contains "PORT_CONFLICT: port 8080 occupied by"
    Failure Indicators: Exit code 0 when port is occupied by unexpected process
    Evidence: .sisyphus/evidence/task-5-port-conflict.txt

  Scenario: No conflict on free port
    Tool: Bash
    Preconditions: Port 19999 is free on VPS
    Steps:
      1. Run: `source scripts/lib/contract.sh && _check_port_conflict "5.183.11.149" "19999" "bridge"`
    Expected Result: Exit code 0, no output to stderr
    Failure Indicators: Exit code 1 when port is actually free
    Evidence: .sisyphus/evidence/task-5-port-free.txt
  ```

  **Commit**: YES
  - Message: `feat(scripts): add port conflict detection`
  - Files: `scripts/lib/contract.sh`, `scripts/a1_deploy.sh`
  - Pre-commit: `bash -n scripts/lib/contract.sh`

- [ ] 6. A0 Discovery Classification Fix

  **What to do**:
  - In `scripts/a0_discover.sh`, add a new classification status `responds_with_error` for methods that:
    - Return a valid JSON-RPC response
    - Have `.error` field set
    - Have empty params `{}`
  - This is distinct from "error" (network/protocol failure) — it means the method exists and responds, but needs parameters
  - Increase `max_retries` from 1 to 2 (lines 112, 164, 205, 256) to handle transient failures
  - Update the classification output to include the new status in the summary counts
  - Do NOT rewrite the classification system — add alongside existing statuses

  **Must NOT do**:
  - Do NOT rewrite the classification logic (add new status, don't restructure)
  - Do NOT change the JSON-RPC call format
  - Do NOT increase retries beyond 2 (keep it reasonable)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Requires understanding A0 classification logic, careful modification without breaking existing flow
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2, 3, 4, 5)
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 9
  - **Blocked By**: Task 1

  **References**:

  **Pattern References**:
  - `scripts/a0_discover.sh:100-130` — Classification logic where method responses are categorized
  - `scripts/a0_discover.sh:160-185` — Method probing loop with retry logic

  **API/Type References**:
  - `scripts/a0_discover.sh:112,164,205,256` — Lines where `max_retries=1` is set

  **WHY Each Reference Matters**:
  - Lines 100-130 show how responses are classified — new status must fit into this flow
  - Lines 112,164,205,256 are the 4 retry budget settings — change from 1 to 2

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: A0 discovers methods with responds_with_error status
    Tool: Bash
    Preconditions: Bridge running on VPS, RPC endpoint accessible
    Steps:
      1. Run: `./scripts/a0_discover.sh 2>&1 | tee /tmp/a0-output.txt`
      2. Run: `grep -c "responds_with_error" /tmp/a0-output.txt`
      3. Run: `grep "max_retries" scripts/a0_discover.sh | head -4`
    Expected Result: At least some methods classified as "responds_with_error" (not 0), max_retries set to 2 in all 4 locations
    Failure Indicators: 0 methods with new status, max_retries still 1 in any location
    Evidence: .sisyphus/evidence/task-6-a0-classification.txt

  Scenario: A0 still correctly identifies truly non-responding methods
    Tool: Bash
    Preconditions: Bridge running with known working and non-working methods
    Steps:
      1. Run: `./scripts/a0_discover.sh 2>&1 | grep -E "found|responding|error"`
      2. Verify "found" count equals sum of all status categories
    Expected Result: Total methods found = sum of (responding + responds_with_error + not_responding)
    Failure Indicators: Count mismatch, double-counting, missing methods
    Evidence: .sisyphus/evidence/task-6-a0-counts.txt
  ```

  **Commit**: YES
  - Message: `fix(a0): add responds_with_error classification + increase retry budget to 2`
  - Files: `scripts/a0_discover.sh`
  - Pre-commit: `bash -n scripts/a0_discover.sh`

- [ ] 7. Topology-Aware Deploy

  **What to do**:
  - Update `scripts/a1_deploy.sh` to be topology-aware:
    1. Call `probe_vps_state()` from `scripts/lib/vps-state.sh` to get current state
    2. Call `_check_port_conflict()` for target ports before deploying
    3. Detect topology: single-image (quickstart) vs multi-image (full stack)
    4. Generate appropriate docker-compose file based on topology:
       - Single-image: 1 service (bridge + matrix in one container)
       - Multi-image: 2 services (bridge + matrix as separate containers)
    5. Use `_contract_wait_http()` with correct scheme (http/https) based on detected mode
    6. Use `_normalize_env()` to ensure env vars are correctly mapped
  - After deploy, verify with health check (NOT just "container running")
  - Save current image tag before any pull operation (for rollback reference)
  - Save deploy evidence to `.sisyphus/evidence/task-7-deploy/`

  **Must NOT do**:
  - Do NOT treat "container running" as "deploy successful" — must verify health endpoint
  - Do NOT remove existing containers without checking if they're from a previous deploy
  - Do NOT hardcode VPS IP — use `$VPS_IP` from `.env`
  - Do NOT skip port conflict checks

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Requires understanding existing deploy flow, integrating 4 new functions, topology detection logic
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (integrates work from Tasks 2, 4, 5)
  - **Parallel Group**: Wave 2 (with Tasks 8)
  - **Blocks**: Task 9
  - **Blocked By**: Tasks 2, 4, 5

  **References**:

  **Pattern References**:
  - `scripts/a1_deploy.sh` — Current deploy script to enhance
  - `scripts/lib/contract.sh:_contract_wait_http()` — HTTPS-capable health check (from Task 4)
  - `scripts/lib/contract.sh:_check_port_conflict()` — Port conflict detection (from Task 5)
  - `scripts/lib/vps-state.sh:probe_vps_state()` — VPS topology detection (from Task 1)

  **API/Type References**:
  - `docker-compose.yml` — Current compose file structure
  - `docker-compose-full.yml` — Multi-service compose for reference

  **WHY Each Reference Matters**:
  - `a1_deploy.sh` is the target file — all enhancements go here
  - The 3 library functions are the new tools available for the deploy logic
  - Compose files show the two topology patterns to generate

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Fresh deploy succeeds with topology detection
    Tool: Bash (SSH)
    Preconditions: VPS has no running ArmorClaw containers, Docker installed
    Steps:
      1. Run: `./scripts/a1_deploy.sh 2>&1 | tee /tmp/deploy-output.txt`
      2. Run: `ssh root@5.183.11.149 "docker ps --format '{{.Names}} {{.Status}}'"`
      3. Run: `ssh root@5.183.11.149 "curl -sf http://localhost:8080/health"`
    Expected Result: Deploy completes, container(s) running with "healthy" status, health endpoint returns 200
    Failure Indicators: Deploy exits non-zero, container crashes, health endpoint unreachable
    Evidence: .sisyphus/evidence/task-7-deploy-fresh.txt

  Scenario: Port conflict blocks deploy
    Tool: Bash (SSH)
    Preconditions: Port 8080 occupied by a non-Bridge process on VPS
    Steps:
      1. Run: `./scripts/a1_deploy.sh 2>&1 | tee /tmp/deploy-conflict.txt`
      2. Check for PORT_CONFLICT in output
    Expected Result: Deploy aborts with "PORT_CONFLICT: port 8080 occupied" message, no containers created
    Failure Indicators: Deploy proceeds despite conflict, containers created on wrong port
    Evidence: .sisyphus/evidence/task-7-deploy-conflict.txt

  Scenario: Update from existing preserves data
    Tool: Bash (SSH)
    Preconditions: ArmorClaw already deployed with data volumes
    Steps:
      1. Run: `./scripts/a1_deploy.sh 2>&1 | tee /tmp/deploy-update.txt`
      2. Run: `ssh root@5.183.11.149 "docker volume ls | grep armorclaw"`
    Expected Result: Existing volumes preserved, new image running, old image tag saved for reference
    Failure Indicators: Volumes deleted, data lost, no rollback tag saved
    Evidence: .sisyphus/evidence/task-7-deploy-update.txt
  ```

  **Commit**: YES
  - Message: `feat(deploy): topology-aware deploy with HTTPS + port checks`
  - Files: `scripts/a1_deploy.sh`
  - Pre-commit: `bash -n scripts/a1_deploy.sh`

- [ ] 8. Authenticated Matrix Smoke Test

  **What to do**:
  - Modify `scripts/vps-matrix-cli-test.sh` to make authentication REQUIRED (not optional) in smoke mode:
    1. Remove the "skip if no credentials" path in smoke mode
    2. Require `MATRIX_USER` and `MATRIX_PASSWORD` env vars (fail with clear error if missing)
    3. Add a `MATRIX_TEST_USER` / `MATRIX_TEST_PASSWORD` option for a dedicated test user
    4. Smoke mode must: login, create test room, send message, verify bridge response via `/sync`
    5. Add cleanup: delete test room after smoke completes
  - Add a credential validation gate at the start: test `ADMIN_TOKEN` against Bridge `status` RPC
  - Extend `vps-validate.sh` smoke mode to use the new authenticated path
  - Update `.env.example` (NOT `.env`) with `MATRIX_TEST_USER` and `MATRIX_TEST_PASSWORD` entries

  **Must NOT do**:
  - Do NOT use admin credentials for smoke tests (use dedicated test user)
  - Do NOT keep Matrix auth optional
  - Do NOT leave test rooms behind (cleanup required)
  - Do NOT modify `vps-matrix-cli-test.sh` smoke behavior for non-smoke modes

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Requires understanding Matrix auth flow, careful modification of existing test script, cleanup logic
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 7)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 9
  - **Blocked By**: Tasks 3, 4

  **References**:

  **Pattern References**:
  - `scripts/vps-matrix-cli-test.sh:7-14` — Current Matrix login with MATRIX_USER + MATRIX_PASSWORD
  - `scripts/vps-matrix-cli-test.sh:560-563` — Current smoke mode (may send messages considered state-changing)
  - `scripts/vps-validate.sh` — Calls vps-matrix-cli-test.sh as subprocess

  **API/Type References**:
  - `tests/lib/load_env.sh` — Environment loader, sources `.env`

  **WHY Each Reference Matters**:
  - Lines 7-14 show the existing auth flow — extend this, don't replace it
  - Lines 560-563 show current smoke mode behavior — make auth required here
  - `vps-validate.sh` is the caller — must ensure it passes the right env vars

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Authenticated smoke test succeeds
    Tool: Bash
    Preconditions: Bridge running, Matrix running, MATRIX_USER and MATRIX_PASSWORD set in .env
    Steps:
      1. Run: `./scripts/vps-matrix-cli-test.sh --smoke 2>&1 | tee /tmp/matrix-smoke.txt`
      2. Run: `grep -c "PASS" /tmp/matrix-smoke.txt`
    Expected Result: All smoke checks pass, login succeeds, test room created and deleted, no "SKIP" for auth
    Failure Indicators: "SKIP: no Matrix credentials", auth failure, test room not cleaned up
    Evidence: .sisyphus/evidence/task-8-matrix-smoke.txt

  Scenario: Smoke test fails without credentials
    Tool: Bash
    Preconditions: MATRIX_USER and MATRIX_PASSWORD NOT set
    Steps:
      1. Run: `unset MATRIX_USER MATRIX_PASSWORD && ./scripts/vps-matrix-cli-test.sh --smoke 2>&1`
    Expected Result: Exit code 1, error message "MATRIX_USER and MATRIX_PASSWORD required for smoke mode"
    Failure Indicators: Exit code 0, test proceeds without auth, silent skip
    Evidence: .sisyphus/evidence/task-8-matrix-no-creds.txt
  ```

  **Commit**: YES
  - Message: `fix(smoke): require Matrix authentication in smoke mode`
  - Files: `scripts/vps-matrix-cli-test.sh`, `scripts/vps-validate.sh`, `.env.example`
  - Pre-commit: `bash -n scripts/vps-matrix-cli-test.sh && bash -n scripts/vps-validate.sh`

- [ ] 9. Lifecycle Orchestrator

  **What to do**:
  - Create `scripts/lifecycle.sh` as the top-level orchestrator with these subcommands (Release 1 scope):
    1. `discover` — Call `probe_vps_state()`, output topology JSON, exit 0 if reachable
    2. `deploy` — Call `a1_deploy.sh` with topology awareness, verify health, exit 0/1
    3. `validate` — Call `vps-validate.sh` (which calls A0-A4), capture report.json, exit 0 if score ≥ 80
  - **Deferred to Release 2** (Task 10): `update` and `report` subcommands — these add complexity without unblocking the core deploy+validate loop. Release 1 validates that the full cycle works end-to-end before adding more subcommands.
  - Add `flock` guard to prevent concurrent execution
  - Each subcommand produces structured output (JSON) and exit codes
  - Support `--mode=smoke` flag for smoke-only validation (zero state-changing ops in smoke)
  - Support `--vps-ip=...` flag to override VPS_IP from env
  - Support `--force` flag required for any destructive operations (in non-smoke mode)
  - Integrate `_normalize_env()` at the top of the script

  **Must NOT do**:
  - Do NOT duplicate logic from existing scripts — call them as subprocesses
  - Do NOT replace `vps-validate.sh` or `a_run_all.sh` — orchestrate them
  - Do NOT run destructive operations in smoke mode

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: New script integrating multiple existing tools, lifecycle state management, flock, structured output
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Tasks 6, 7, 8)
  - **Parallel Group**: Wave 2 (sequential after 7 and 8)
  - **Blocks**: Task 10
  - **Blocked By**: Tasks 6, 7, 8

  **References**:

  **Pattern References**:
  - `scripts/a1_deploy.sh` — Deploy subprocess to call
  - `scripts/a0_discover.sh` — Discovery subprocess to call
  - `scripts/vps-validate.sh:155-296` — `generate_report()` function to reuse for report step
  - `deploy/install-bridge.sh:392` — `flock` pattern for concurrent execution prevention

  **API/Type References**:
  - `scripts/lib/vps-state.sh:probe_vps_state()` — VPS state probe to call
  - `scripts/lib/contract.sh:_normalize_env()` — Env normalization to call

  **External References**:
  - `.skills/TEMPLATE.yaml` — Schema for Phase 7 skill wrapper (future compatibility)

  **WHY Each Reference Matters**:
  - `vps-validate.sh:155-296` has the existing report generator — reuse it
  - `deploy/install-bridge.sh:392` shows the flock pattern already in use
  - `.skills/TEMPLATE.yaml` shows the future skill interface — design CLI to be compatible

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Full lifecycle runs discover → deploy → validate → report
    Tool: Bash
    Preconditions: VPS reachable, Docker installed, credentials in .env
    Steps:
      1. Run: `./scripts/lifecycle.sh discover`
      2. Run: `./scripts/lifecycle.sh deploy`
      3. Run: `./scripts/lifecycle.sh validate`
      4. Run: `./scripts/lifecycle.sh report`
      5. Check: `ls .sisyphus/evidence/lifecycle-report-*.json`
    Expected Result: All subcommands exit 0, report JSON exists, overall_score in report ≥ 80
    Failure Indicators: Any subcommand exits non-zero, report file missing, score < 80
    Evidence: .sisyphus/evidence/task-9-lifecycle-full.txt

  Scenario: Discover fails gracefully on unreachable VPS
    Tool: Bash
    Preconditions: VPS unreachable (wrong IP or SSH key)
    Steps:
      1. Run: `./scripts/lifecycle.sh --vps-ip=127.0.0.1 discover 2>&1`
    Expected Result: Exit code 1, error message about SSH connection
    Failure Indicators: Exit code 0, hang, no error message
    Evidence: .sisyphus/evidence/task-9-lifecycle-unreachable.txt

  Scenario: Concurrent execution blocked by flock
    Tool: Bash
    Preconditions: One lifecycle.sh already running
    Steps:
      1. Run: `./scripts/lifecycle.sh deploy &` (background)
      2. Run: `./scripts/lifecycle.sh deploy 2>&1` (second attempt)
    Expected Result: Second attempt exits with message "lifecycle.sh already running" or similar
    Failure Indicators: Both run simultaneously, data corruption
    Evidence: .sisyphus/evidence/task-9-lifecycle-flock.txt
  ```

  **Commit**: YES
  - Message: `feat(lifecycle): add top-level orchestrator script`
  - Files: `scripts/lifecycle.sh`
  - Pre-commit: `bash -n scripts/lifecycle.sh`

- [ ] 10. Report Step + Update Subcommand (Release 1 deferred)

  **What to do**:
  - Add `update` subcommand to `scripts/lifecycle.sh`:
    1. Save current image tag before pulling: `docker inspect --format='{{.Image}}'`
    2. Pull new image: `docker pull mikegemut/armorclaw:latest`
    3. Restart containers with new image
    4. Verify health with `_contract_wait_http()`
    5. If health check fails, log warning but do NOT auto-rollback (preserve old image tag for manual rollback)
  - Add `report` subcommand to `scripts/lifecycle.sh`:
    1. Collect all evidence files from `.sisyphus/evidence/` generated during the lifecycle run
    2. Call `vps-validate.sh`'s `generate_report()` to produce the final report JSON
    3. Archive the report with a timestamp: `lifecycle-report-$(date +%Y%m%d-%H%M%S).json`
    4. Include a summary section: which phases passed/failed, scores, timestamps
    5. Output both machine-readable JSON and human-readable summary to stdout
  - Ensure report doesn't overwrite previous reports (timestamp preservation)
  - Add report path to evidence directory

  **Must NOT do**:
  - Do NOT build a new report generation system (extend existing)
  - Do NOT overwrite previous reports
  - Do NOT include sensitive data (API keys, tokens) in reports

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Two new subcommands for lifecycle orchestrator, extends existing script
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (sequential after Task 9)
  - **Blocks**: F1-F4
  - **Blocked By**: Task 9

  **References**:

  **Pattern References**:
  - `scripts/vps-validate.sh:155-296` — Existing `generate_report()` with JSON scoring
  - `scripts/vps-validate.sh:273` — Fixed path report write — this is what needs timestamping

  **WHY Each Reference Matters**:
  - Lines 155-296 are the existing report generator — extend this, don't replace
  - Line 273 shows where reports are written — needs timestamp suffix to prevent overwrites

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Report generated with timestamp and valid JSON
    Tool: Bash
    Preconditions: Lifecycle run completed with evidence files present
    Steps:
      1. Run: `./scripts/lifecycle.sh report 2>&1`
      2. Run: `ls -la .sisyphus/evidence/lifecycle-report-*.json`
      3. Run: `jq '.overall_score' .sisyphus/evidence/lifecycle-report-*.json`
    Expected Result: Report file exists with timestamp in name, valid JSON, overall_score present
    Failure Indicators: No report file, invalid JSON, missing score field
    Evidence: .sisyphus/evidence/task-10-report.txt

  Scenario: Multiple reports don't overwrite each other
    Tool: Bash
    Preconditions: One report already exists from previous run
    Steps:
      1. Count existing reports: `ls .sisyphus/evidence/lifecycle-report-*.json | wc -l`
      2. Run: `./scripts/lifecycle.sh report`
      3. Count again: `ls .sisyphus/evidence/lifecycle-report-*.json | wc -l`
    Expected Result: Count increased by 1, previous report still intact
    Failure Indicators: Same count (overwrite), previous report modified
    Evidence: .sisyphus/evidence/task-10-report-no-overwrite.txt
  ```

  **Commit**: YES
  - Message: `feat(lifecycle): integrate report step with evidence archiving`
  - Files: `scripts/lifecycle.sh`
  - Pre-commit: `bash -n scripts/lifecycle.sh`

---

## Release 2 Tasks (after Release 1 verified)

> Release 2 tasks are deferred until Release 1 is fully verified and user-approved.
> These are sketched here for scope completeness but will be detailed in a follow-up plan iteration.

- [ ] 11. Feature Group A — Matrix Control Plane
  - Enable Matrix adapter tests in A4 harness
  - Verify Matrix room creation, invitation, messaging, E2EE
  - Skip-disabled semantics for when Matrix is not deployed
  - **Depends on**: Release 1 complete

- [ ] 12. Feature Group B — Agent/Studio
  - Enable Agent runtime and Studio tests in A4 harness
  - Container lifecycle, event emission, skill extraction
  - **Depends on**: Release 1 complete

- [ ] 13. Feature Group C — Secretary Workflows
  - Enable Secretary workflow core and deep tests
  - State machine, blocker resolution, restart survival
  - **Depends on**: Release 1 complete

- [ ] 14. Feature Group D — Trust/PII
  - Enable Trust layer and PII detection tests
  - Approval flow, risk classification, BlindFill
  - **Depends on**: Release 1 complete

- [ ] 15. Feature Groups E-I Expansion
  - Email pipeline, voice stack, Jetski sidecar, license, platform adapters
  - Each group has its own test suite in the A4 harness
  - `skip-disabled` for undeployed subsystems
  - **Depends on**: Tasks 11-14

- [ ] 16. `.skills/test-vps.yaml` Wrapper
  - Create `.skills/test-vps.yaml` following `.skills/TEMPLATE.yaml` schema
  - CLI interface: `opencode skill test-vps --vps-ip=5.183.11.149 --mode=smoke`
  - Wraps `scripts/lifecycle.sh` with appropriate flags
  - Returns exit code 0/1 for agent consumption
  - **Depends on**: Tasks 11-15

---

## Final Verification Wave (MANDATORY — after ALL Release 1 implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
>
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in `.sisyphus/evidence/`. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `bash -n` on all modified scripts. Check for: empty catches, hardcoded IPs/credentials, unused variables, missing error handling. Check AI slop: excessive comments, over-abstraction, generic names. Verify `shellcheck` passes on new code.
  Output: `Syntax [PASS/FAIL] | Shellcheck [PASS/FAIL] | Security [PASS/FAIL] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Real Manual QA** — `unspecified-high`
  SSH into VPS. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (deploy → validate → report pipeline). Test edge cases: no Bridge running, wrong port, expired credentials. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination: Task N touching Task M's files. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

| Wave | Commit Message | Files | Pre-commit |
|------|---------------|-------|------------|
| 0 | `fix(scripts): add VPS state discovery probe` | `scripts/lib/vps-state.sh` | `bash -n scripts/lib/vps-state.sh` |
| 1 | `fix(docker): add ldconfig to runtime stage` | `Dockerfile.quickstart` | `docker build --check -f Dockerfile.quickstart .` |
| 1 | `feat(scripts): add env var normalization shim` | `scripts/lib/contract.sh` | `bash -n scripts/lib/contract.sh` |
| 1 | `fix(scripts): add HTTPS support to _contract_wait_http` | `scripts/lib/contract.sh` | `bash -n scripts/lib/contract.sh` |
| 1 | `feat(scripts): add port conflict detection` | `scripts/lib/contract.sh` | `bash -n scripts/lib/contract.sh` |
| 1 | `fix(a0): add responds_with_error classification + increase retry budget` | `scripts/a0_discover.sh` | `bash -n scripts/a0_discover.sh` |
| 2 | `feat(deploy): topology-aware deploy with HTTPS + port checks` | `scripts/a1_deploy.sh` | `bash -n scripts/a1_deploy.sh` |
| 2 | `fix(smoke): require Matrix authentication in smoke mode` | `scripts/vps-matrix-cli-test.sh` | `bash -n scripts/vps-matrix-cli-test.sh` |
| 2 | `feat(lifecycle): add top-level orchestrator script` | `scripts/lifecycle.sh` | `bash -n scripts/lifecycle.sh` |
| 3 | `feat(lifecycle): add update and report subcommands` | `scripts/lifecycle.sh` | `bash -n scripts/lifecycle.sh` |

---

## Success Criteria

### Verification Commands
```bash
# Docker image runs on VPS
ssh root@5.183.11.149 "docker run --rm mikegemut/armorclaw:latest /opt/armorclaw/armorclaw-bridge --version"
# Expected: v4.8.0 (or current version string)

# Lifecycle discover
./scripts/lifecycle.sh discover
# Expected: exit 0, JSON with topology info

# Lifecycle deploy
./scripts/lifecycle.sh deploy
# Expected: exit 0, healthy containers

# Lifecycle validate
./scripts/lifecycle.sh validate
# Expected: exit 0, report.json with overall_score ≥ 80

# Lifecycle report
./scripts/lifecycle.sh report
# Expected: exit 0, timestamped evidence in .sisyphus/evidence/

# A0 discovery
./scripts/a0_discover.sh
# Expected: methods classified with "responds_with_error" status, retry budget 2

# Authenticated Matrix smoke
./scripts/vps-matrix-cli-test.sh --smoke
# Expected: requires MATRIX_USER + MATRIX_PASSWORD, exits 0 on success
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All existing test scripts still pass (`bash -n` syntax check)
- [ ] Docker image builds in CI and runs on VPS
- [ ] Lifecycle orchestrator produces valid JSON reports
- [ ] No `.env` file committed
- [ ] No Go bridge source code modified
- [ ] No Matrix/Conduit HTTP URLs changed
