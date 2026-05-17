# Docker Deployment UX Improvements

## TL;DR

> **Quick Summary**: Fix the Docker image entrypoint routing bug (uses host installer instead of Docker-aware bootstrap), fix the bridge-only/CI code path to actually start the bridge, complete two deploy-mode stubs in vps-lifecycle.sh, and add first-time Docker deployment documentation.
> 
> **Deliverables**:
> - Fixed Dockerfile.quickstart entrypoint (routes to Docker-aware bootstrap)
> - Fixed quickstart-entrypoint.sh bridge-only/CI path (starts bridge, doesn't just exit)
> - Completed `reuse-existing-matrix` and `side-by-side` deploy modes in vps-lifecycle.sh
> - New `docs/docker-quickstart.md` guide
> 
> **Estimated Effort**: Medium
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: Task 1A (entrypoint routing) → Task 1B (restart-path hardening) → Tasks 2+3 (parallel) → Final Wave

---

## Context

### Original Request
User provided a pre-written 8-task plan to improve Docker deployment. After investigation, 4 of 8 tasks were already implemented or unnecessary. The real root cause is an entrypoint routing bug.

### Investigation Summary
**Key Discoveries**:
- **Entrypoint bug**: Dockerfile's entrypoint-wrapper.sh calls `quickstart.sh` (= host installer `setup-quick.sh`), NOT the Docker-aware `quickstart-entrypoint.sh`. This is why VPS deployment required bypassing the entrypoint entirely.
- **CI will break**: quickstart-entrypoint.sh's bridge-only mode does `exit 0` (line 77) — container stops — CI's hard gate (`State.Running` after 10s) fails.
- **All SQLite DBs self-initialize**: 24+ stores use `CREATE TABLE IF NOT EXISTS`. No first-run persistence gap.
- **Topology detection already done**: `scripts/lib/topology.sh` fully classifies VPS, recommends deploy mode.
- **HTTPS-first probing already done**: `scripts/lib/probe.sh` has proper HTTPS → HTTP fallback.
- **CI runtime gate already done**: `.github/workflows/dockerhub.yml` has ldd + startup + health gate.
- **`.skills/` bind-mount is scope creep**: Those YAMLs are for AI CLI tools on developer machines, not container runtime code.

### Metis Review
**Identified Gaps** (addressed):
- CI compatibility: Must fix bridge-only path to exec bridge binary instead of exit 0
- Env var inconsistency: `ARMORCLAW_SKIP_DOCKER_CHECK` vs `ARMORCLAW_SKIP_DOCKER` across files
- Conduit orphan risk: `--network container:armorclaw` breaks if armorclaw container recreated
- `.bootstrapped` flag + dead Conduit: Restart path skips bootstrap but container-setup.sh fails reaching dead Conduit

---

## Work Objectives

### Core Objective
Make the Docker image start reliably out-of-the-box by fixing the entrypoint routing bug and bridge-only path, then document the first-time experience.

### Concrete Deliverables
- Patched `Dockerfile.quickstart` entrypoint-wrapper.sh (1-line change + env-var consistency)
- Patched `deploy/quickstart-entrypoint.sh` bridge-only/CI path (starts bridge, not exit 0)
- Patched `deploy/quickstart-entrypoint.sh` restart-path with dead-Conduit recovery
- Normalized env-var naming (`ARMORCLAW_SKIP_DOCKER_CHECK` consistency)
- Implemented `reuse-existing-matrix` deploy mode in `scripts/vps-lifecycle.sh` (with explicit network model)
- Implemented `side-by-side` deploy mode in `scripts/vps-lifecycle.sh` (with volume/socket/log isolation)
- New `docs/docker-quickstart.md` documentation

### Definition of Done
- [ ] `docker build -f Dockerfile.quickstart -t armorclaw-test .` succeeds
- [ ] `docker run -d -e GITHUB_ACTIONS=true armorclaw-test` stays running for 10+ seconds
- [ ] `docker run -d -v /var/run/docker.sock:/var/run/docker.sock armorclaw-test` reaches Conduit + Bridge healthy state
- [ ] `bash tests/test-quickstart-entrypoint.sh` passes
- [ ] `.github/workflows/dockerhub.yml` CI pipeline passes

### Must Have
- Docker image starts reliably with the default entrypoint (no --entrypoint override needed)
- CI pipeline continues to pass after the fix
- Bridge-only mode (no Docker socket) starts the bridge process, not just exits
- Existing test suite (`tests/test-quickstart-entrypoint.sh`) passes
- `reuse-existing-matrix` mode preserves existing Conduit, deploys bridge-only
- Documentation covers concrete `docker run` commands with all required flags

### Must NOT Have (Guardrails)
- Do NOT touch `container-setup.sh` (2528 lines, already works)
- Do NOT touch `deploy/setup-quick.sh` (still used for host installs)
- Do NOT modify Go bridge source code
- Do NOT change the HEALTHCHECK directive
- Do NOT add new provisioning RPC or environment variables
- Do NOT touch SQLCipher, keystore, or E2EE code
- Do NOT modify `.github/workflows/dockerhub.yml` — make the entrypoint CI-compatible instead
- Do NOT modify existing fully-implemented phases in vps-lifecycle.sh (only lines 350-371 stubs)
- Do NOT stop or replace existing Conduit in `reuse-existing-matrix` mode
- Do NOT add `.skills/` bind-mount (scope creep — no container code reads those YAMLs)

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** - ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (bridge Go tests + `tests/test-quickstart-entrypoint.sh`)
- **Automated tests**: Tests-after (run existing test suite after changes)
- **Framework**: bash test scripts + `docker build`/`docker run` validation

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/docker-ux/`.

- **Docker build/run**: Use Bash — build image, run container, check process, probe health
- **Deploy scripts**: Use Bash — source scripts, run vps-lifecycle.sh with --phase flags
- **Documentation**: Use Bash — verify file exists, grep for required sections

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (foundation — must complete first):
├── Task 1A: Fix Dockerfile entrypoint routing + bridge-only/CI path + env-var consistency [deep]
└── (keystone fix — no parallel tasks until this lands)

Wave 1B (after 1A — restart hardening):
├── Task 1B: Restart-path dead-Conduit recovery [quick]
└── (small follow-up, depends on 1A's entrypoint being correct)

Wave 2 (after Task 1B — independent, PARALLEL):
├── Task 2: Complete vps-lifecycle.sh deploy-mode stubs [unspecified-high]
└── Task 3: Add first-time Docker deployment docs [writing]

Wave FINAL (after ALL tasks — 4 parallel reviews):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Automated integration QA (unspecified-high)
└── F4: Scope fidelity check (deep)
-> Present results -> User governance checkpoint (review, not manual testing)

Critical Path: Task 1A → Task 1B → Task 2 + Task 3 (parallel) → F1-F4 → user review
Parallel Speedup: Wave 2 tasks run concurrently
Max Concurrent: 2 (Wave 2)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1A | - | 1B, 2, 3 | 1 |
| 1B | 1A | 2, 3 | 1B |
| 2 | 1B | F1-F4 | 2 |
| 3 | 1A | F1-F4 | 2 |

### Agent Dispatch Summary

- **Wave 1**: **1** — T1A → `deep`
- **Wave 1B**: **1** — T1B → `quick`
- **Wave 2**: **2** — T2 → `unspecified-high`, T3 → `writing`
- **FINAL**: **4** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1A. Fix Entrypoint Routing + Bridge-Only/CI Path + Env-Var Consistency

  **What to do**:
  - In `Dockerfile.quickstart`, change `entrypoint-wrapper.sh` to call `/opt/armorclaw/quickstart-entrypoint.sh` instead of `/opt/armorclaw/quickstart.sh` (the host installer)
  - Specifically: modify the heredoc at Dockerfile lines 224-236 — change line 235 from `exec /opt/armorclaw/quickstart.sh "$@"` to `exec /opt/armorclaw/quickstart-entrypoint.sh "$@"`
  - In `deploy/quickstart-entrypoint.sh`, fix the bridge-only/CI path (lines 44-78): instead of `exit 0`, exec the bridge binary: `exec /opt/armorclaw/armorclaw-bridge -config "$CONFIG_DIR/config.toml"` (after config copy and `.bootstrapped` touch)
  - **Env-var consistency**: Normalize `ARMORCLAW_SKIP_DOCKER` (in quickstart-entrypoint.sh line 48) to `ARMORCLAW_SKIP_DOCKER_CHECK` to match what CI and entrypoint-wrapper.sh set. Add a backward-compat check so both names work: `[ "${ARMORCLAW_SKIP_DOCKER_CHECK:-false}" = "true" ] || [ "${ARMORCLAW_SKIP_DOCKER:-false}" = "true" ]`
  - Verify the `--non-interactive` flag handling is not needed in quickstart-entrypoint.sh (it uses env vars already — confirm this is acceptable)

  **Must NOT do**:
  - Do NOT modify `container-setup.sh`
  - Do NOT modify `deploy/setup-quick.sh`
  - Do NOT modify Go bridge source
  - Do NOT change the HEALTHCHECK directive
  - Do NOT modify `.github/workflows/dockerhub.yml`
  - Do NOT add new environment variables (only normalize existing ones)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Subtle Docker entrypoint chain with CI compatibility constraints; requires understanding the full startup flow and testing multiple scenarios
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1 (keystone — nothing else starts until this lands)
  - **Blocks**: Tasks 1B, 2, 3
  - **Blocked By**: None (can start immediately)

  **References** (CRITICAL):

  **Pattern References**:
  - `Dockerfile.quickstart:224-236` — The entrypoint-wrapper.sh heredoc that needs the 1-line fix (`quickstart.sh` → `quickstart-entrypoint.sh`)
  - `Dockerfile.quickstart:230-232` — CI detection in wrapper (`GITHUB_ACTIONS`, `CI`, `ARMORCLAW_SKIP_DOCKER_CHECK`) — these env vars also reach quickstart-entrypoint.sh
  - `deploy/quickstart-entrypoint.sh:44-78` — Bridge-only path that does `exit 0` at line 77 — must exec bridge binary instead
  - `deploy/quickstart-entrypoint.sh:48` — Env var check `ARMORCLAW_SKIP_DOCKER` — needs to also accept `ARMORCLAW_SKIP_DOCKER_CHECK` for consistency
  - `deploy/quickstart-entrypoint.sh:59-61` — Config copy pattern to reuse in bridge-only path

  **API/Type References**:
  - `bridge/cmd/bridge/main.go:1815` — `ARMORCLAW_SKIP_DOCKER_CHECK` in Go bridge (understand what env vars the bridge itself checks)
  - `bridge/pkg/http/server.go:115-116` — Port 8443 is configurable default via `config.Port`
  - `Dockerfile.quickstart:220-221` — HEALTHCHECK uses Unix socket RPC

  **Test References**:
  - `tests/test-quickstart-entrypoint.sh` — Existing 6-test unit suite; must continue to pass

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Docker image builds successfully with new entrypoint
    Tool: Bash
    Preconditions: Docker daemon running, repo checked out
    Steps:
      1. docker build -f Dockerfile.quickstart -t armorclaw-ux-test .
      2. Check exit code is 0
    Expected Result: Image builds without errors
    Failure Indicators: Build fails at any stage
    Evidence: .sisyphus/evidence/docker-ux/task-1a-build.log

  Scenario: Entrypoint wrapper calls correct script
    Tool: Bash
    Preconditions: Image built
    Steps:
      1. docker run --rm --entrypoint "" armorclaw-ux-test grep "quickstart-entrypoint" /opt/armorclaw/entrypoint-wrapper.sh
    Expected Result: Output contains "quickstart-entrypoint.sh" (not "quickstart.sh")
    Failure Indicators: Still references old quickstart.sh
    Evidence: .sisyphus/evidence/docker-ux/task-1a-entrypoint-check.txt

  Scenario: CI mode container stays running (bridge-only path starts bridge)
    Tool: Bash
    Preconditions: Image built successfully
    Steps:
      1. docker run -d --name ci-test -e GITHUB_ACTIONS=true armorclaw-ux-test
      2. sleep 10
      3. docker inspect -f '{{.State.Running}}' ci-test
      4. docker exec ci-test pgrep -f armorclaw-bridge
    Expected Result: Container is running AND bridge process exists after 10 seconds
    Failure Indicators: Container exited, bridge process not found
    Evidence: .sisyphus/evidence/docker-ux/task-1a-ci-mode.txt

  Scenario: Bridge-only mode (no Docker socket, not CI) starts bridge
    Tool: Bash
    Preconditions: Image built, no Docker socket mount
    Steps:
      1. docker run -d --name bridge-only-test armorclaw-ux-test
      2. sleep 5
      3. docker inspect -f '{{.State.Running}}' bridge-only-test
      4. docker exec bridge-only-test pgrep -f armorclaw-bridge
    Expected Result: Container is running, bridge process exists
    Failure Indicators: Container exited with code 0 (the old exit 0 behavior)
    Evidence: .sisyphus/evidence/docker-ux/task-1a-bridge-only.txt

  Scenario: Full quickstart with Docker socket — Conduit + Bridge both healthy
    Tool: Bash
    Preconditions: Image built, Docker socket available, ports 8443/6167 free
    Steps:
      1. docker run -d --name full-test -v /var/run/docker.sock:/var/run/docker.sock -p 8443:8443 -p 6167:6167 armorclaw-ux-test
      2. sleep 30
      3. curl -sf http://localhost:6167/_matrix/client/versions
      4. docker exec full-test pgrep -f armorclaw-bridge
    Expected Result: Conduit responds with version JSON, bridge process running
    Failure Indicators: Conduit unreachable, bridge not started
    Evidence: .sisyphus/evidence/docker-ux/task-1a-full-quickstart.txt

  Scenario: Env-var consistency — ARMORCLAW_SKIP_DOCKER_CHECK accepted
    Tool: Bash
    Preconditions: Image built
    Steps:
      1. docker run --rm --entrypoint "" armorclaw-ux-test grep -c "ARMORCLAW_SKIP_DOCKER_CHECK" /opt/armorclaw/quickstart-entrypoint.sh
    Expected Result: Count >= 1 (the env var name appears in the script)
    Failure Indicators: Only old ARMORCLAW_SKIP_DOCKER name present
    Evidence: .sisyphus/evidence/docker-ux/task-1a-env-var-consistency.txt

  Scenario: Existing test suite passes after entrypoint fix
    Tool: Bash
    Preconditions: Changes applied
    Steps:
      1. bash tests/test-quickstart-entrypoint.sh
    Expected Result: "All tests passed!" or all 6 tests pass
    Failure Indicators: Any test fails
    Evidence: .sisyphus/evidence/docker-ux/task-1a-test-suite.txt

  Scenario: VPS re-test — fixed image deployed on real VPS passes health suite
    Tool: Bash (SSH to VPS)
    Preconditions: Image pushed to Docker Hub (or pulled from CI build)
    Steps:
      1. ssh root@5.183.11.149 "docker pull mikegemut/armorclaw:latest"
      2. ssh root@5.183.11.149 "docker rm -f armorclaw-test 2>/dev/null; docker run -d --name armorclaw-test --network host -v /var/run/docker.sock:/var/run/docker.sock -v /var/lib/armorclaw/data:/data mikegemut/armorclaw:latest"
      3. sleep 15
      4. ssh root@5.183.11.149 "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"health.check\"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock"
    Expected Result: health.check returns status "healthy" with matrix "connected"
    Failure Indicators: Bridge not responding, matrix not connected, container exited
    Evidence: .sisyphus/evidence/docker-ux/task-1a-vps-retest.txt
  ```

  **Commit**: YES
  - Message: `fix(docker): route entrypoint to Docker-aware quickstart bootstrap`
  - Files: `Dockerfile.quickstart`, `deploy/quickstart-entrypoint.sh`
  - Pre-commit: `bash tests/test-quickstart-entrypoint.sh`

- [x] 1B. Restart-Path Dead-Conduit Recovery (OPTIONAL — SKIPPED)

  > **This task is optional.** Only implement if restart-with-dead-Conduit is a common failure mode in practice. If the VPS re-test (Task 1A scenario 7) shows clean restarts, skip this task entirely.

  **What to do**:
  - In `deploy/quickstart-entrypoint.sh`, add a Conduit liveness check in the restart path (around line 89-91, before the `.bootstrapped` flag check exec's container-setup.sh)
  - When `.bootstrapped` flag exists: verify `curl -sf http://localhost:6167/_matrix/client/versions` succeeds. If it fails, log a clear warning and either (a) remove `.bootstrapped` and re-run bootstrap, or (b) print a clear diagnostic message about the dead Conduit and how to recover
  - Choose approach (a) for automatic recovery: remove `.bootstrapped`, log "Conduit not responding — re-running bootstrap", fall through to the full bootstrap path

  **Must NOT do**:
  - Do NOT modify `container-setup.sh`
  - Do NOT modify Go bridge source
  - Do NOT change the bootstrap flow for happy-path (when Conduit is alive)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Small targeted fix to one code path, ~10-15 lines of bash
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1B (sequential after 1A)
  - **Blocks**: Tasks 2, 3
  - **Blocked By**: Task 1A

  **References**:

  **Pattern References**:
  - `deploy/quickstart-entrypoint.sh:88-92` — Restart path: if `.bootstrapped` exists, exec's container-setup.sh — THIS is where the Conduit liveness check goes
  - `deploy/quickstart-entrypoint.sh:294-301` — Existing Conduit health wait pattern (`curl -sf http://localhost:6167/_matrix/client/versions`) — REUSE this check
  - `deploy/quickstart-entrypoint.sh:202-253` — Existing Conduit detection and restart logic (handles stopped-but-existing containers) — understand for recovery path

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Restart with alive Conduit proceeds normally
    Tool: Bash
    Preconditions: Container bootstrapped, Conduit running, `.bootstrapped` exists
    Steps:
      1. docker restart armorclaw-ux-test
      2. sleep 15
      3. docker logs --tail 5 armorclaw-ux-test
    Expected Result: Logs show "Configuration already bootstrapped, starting services" — normal restart path
    Failure Indicators: Re-bootstrap triggered unnecessarily
    Evidence: .sisyphus/evidence/docker-ux/task-1b-alive-restart.txt

  Scenario: Restart with dead Conduit triggers re-bootstrap
    Tool: Bash
    Preconditions: Container bootstrapped once, Conduit container manually killed
    Steps:
      1. docker rm -f armorclaw-conduit
      2. docker restart armorclaw-ux-test
      3. sleep 15
      4. docker logs --tail 20 armorclaw-ux-test
    Expected Result: Logs show "Conduit not responding — re-running bootstrap", then Conduit recreated
    Failure Indicators: Cryptic container-setup.sh failure, or silent hang
    Evidence: .sisyphus/evidence/docker-ux/task-1b-dead-conduit-recovery.txt
  ```

  **Commit**: YES (groups with Task 1A)
  - Message: `fix(docker): route entrypoint to Docker-aware quickstart bootstrap`
  - Files: `deploy/quickstart-entrypoint.sh`

- [x] 2. Complete vps-lifecycle.sh Deploy-Mode Stubs

  **What to do**:
  - In `scripts/vps-lifecycle.sh`, implement the `reuse-existing-matrix` stub (currently lines ~350-363): when an existing Conduit is detected on port 6167, deploy Bridge-only with `ARMORCLAW_EXTERNAL_MATRIX=true` and `ARMORCLAW_MATRIX_HOMESERVER_URL=http://localhost:6167`
  - **Network model for `reuse-existing-matrix`**: Use `network_mode: host` for the bridge container. This ensures `localhost:6167` reaches Conduit regardless of whether Conduit runs in a container or natively. This avoids Docker network complexity and matches the existing VPS deployment pattern (armorclaw-test container we deployed earlier used host networking successfully).
  - Implement the `side-by-side` stub (currently lines ~364-371): deploy a second Bridge on alternate ports with complete isolation
  - **Side-by-side isolation — start simple, enhance later:**
    - **MUST have**: separate container name (`armorclaw-bridge-2`), alternate ports (8444/8081), separate data volume (`armorclaw-data-2`)
    - **NICE TO HAVE** (skip for now, add in follow-up if needed): separate Unix socket path, separate config volume, separate log label, no `.bootstrapped` reuse
  - For `reuse-existing-matrix`: preserve keystore secret using the 3-fallback pattern from `replace-existing` mode (vps-lifecycle.sh lines 307-330): (1) env var, (2) existing keystore key file, (3) generate new)
  - For `side-by-side`: calculate port offsets (8443→8444, 8080→8081), use different container name, detect and report conflicts
  - Both modes must fail fast with clear error messages when prerequisites aren't met (no Docker, no Conduit for reuse, port already in use)

  **Must NOT do**:
  - Do NOT stop or replace the existing Conduit container in `reuse-existing-matrix`
  - Do NOT modify fully-implemented phases in vps-lifecycle.sh (only the stub functions)
  - Do NOT add new environment variables — use existing ones (`ARMORCLAW_EXTERNAL_MATRIX`, `ARMORCLAW_MATRIX_HOMESERVER_URL`)
  - Do NOT make side-by-side a full multi-tenant solution — basic coexistence only
  - Do NOT use Docker user-defined networks for `reuse-existing-matrix` — host networking is simpler and proven

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Requires understanding existing 1172-line vps-lifecycle.sh patterns, deploy mode classification, and port-conflict handling — non-trivial but well-patterned
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 3)
  - **Parallel Group**: Wave 2
  - **Blocks**: F1-F4
  - **Blocked By**: Task 1B

  **References** (CRITICAL):

  **Pattern References**:
  - `scripts/vps-lifecycle.sh:350-363` — The `reuse-existing-matrix` stub that needs implementation
  - `scripts/vps-lifecycle.sh:364-371` — The `side-by-side` stub that needs implementation
  - `scripts/vps-lifecycle.sh:307-330` — `replace-existing` mode's keystore secret preservation (3-fallback) — REUSE THIS PATTERN
  - `scripts/a1_deploy.sh` — Full topology-aware deployment (284 lines) — reference for docker-compose generation and health waiting

  **API/Type References**:
  - `scripts/lib/topology.sh` — Topology classification that triggers these modes; outputs JSON with `deploy_mode` field
  - `scripts/lib/probe.sh` — `_probe_bridge_health()` for checking if bridge is already running on a port

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: reuse-existing-matrix deploys bridge alongside existing Conduit
    Tool: Bash
    Preconditions: Conduit running on localhost:6167 (from topology.sh detect)
    Steps:
      1. bash scripts/vps-lifecycle.sh --vps-ip 5.183.11.149 --ssh-key ~/.ssh/openclaw_win --phase deploy --deploy-mode reuse-existing-matrix --force
      2. After completion, check: docker ps | grep -v conduit | grep armorclaw
      3. Check: docker exec <bridge-container> env | grep ARMORCLAW_EXTERNAL_MATRIX=true
      4. Check: docker inspect <bridge-container> | grep NetworkMode
    Expected Result: Bridge container running with EXTERNAL_MATRIX=true, host networking, existing Conduit untouched
    Failure Indicators: Conduit container stopped/replaced, bridge on wrong network, missing env vars
    Evidence: .sisyphus/evidence/docker-ux/task-2-reuse-existing.txt

  Scenario: reuse-existing-matrix preserves keystore secret
    Tool: Bash
    Preconditions: Existing deployment with ARMORCLAW_KEYSTORE_SECRET set
    Steps:
      1. Run deploy with reuse-existing-matrix mode
      2. Check bridge container env for ARMORCLAW_KEYSTORE_SECRET matching the original value
    Expected Result: Same keystore secret used, encrypted data still accessible
    Failure Indicators: New keystore secret generated, bridge reports keystore errors
    Evidence: .sisyphus/evidence/docker-ux/task-2-keystore-preserve.txt

  Scenario: side-by-side deploys second bridge with basic isolation
    Tool: Bash
    Preconditions: Existing bridge on 8443, existing Conduit on 6167
    Steps:
      1. bash scripts/vps-lifecycle.sh --phase deploy --deploy-mode side-by-side --force
      2. Check: docker ps | grep armorclaw (should show 2 bridge containers with different names)
      3. Check second bridge on port 8444: curl -sk https://localhost:8444/health
      4. Check second bridge uses separate data volume: docker inspect <second-bridge> | grep -A2 Binds
      5. Verify first bridge still healthy on 8443
    Expected Result: Second bridge on 8444 with own data volume, first bridge untouched on 8443
    Failure Indicators: Port conflict, shared data volume, primary bridge affected
    Evidence: .sisyphus/evidence/docker-ux/task-2-side-by-side.txt

  Scenario: Port conflict detected before deployment
    Tool: Bash
    Preconditions: Port 8443 AND 8444 already in use
    Steps:
      1. Attempt side-by-side deploy
      2. Check output for clear error message about port conflict
      3. Check exit code is non-zero
    Expected Result: Clear error: "Port 8444 already in use", non-zero exit
    Failure Indicators: Deploy proceeds silently, or cryptic Docker error
    Evidence: .sisyphus/evidence/docker-ux/task-2-port-conflict.txt

  Scenario: Clear error when prerequisites not met
    Tool: Bash
    Preconditions: No Conduit running for reuse mode
    Steps:
      1. Attempt reuse-existing-matrix with no Conduit running
      2. Check output for clear error about missing Conduit
      3. Check exit code is non-zero
    Expected Result: Clear error message, non-zero exit, no partial deployment
    Failure Indicators: Deploy proceeds without Conduit, or silent failure
    Evidence: .sisyphus/evidence/docker-ux/task-2-prereq-error.txt
  ```

  **Commit**: YES
  - Message: `feat(deploy): complete reuse-existing-matrix and side-by-side deploy modes`
  - Files: `scripts/vps-lifecycle.sh`
  - Pre-commit: `bash -n scripts/vps-lifecycle.sh`

- [x] 3. Add First-Time Docker Deployment Documentation

  **What to do**:
  - Create `docs/docker-quickstart.md` — a single new file covering the first-time Docker deployment experience
  - Sections required: Prerequisites, Quick Start (`docker pull` + `docker run`), Required Environment Variables, Health Verification, Connecting ArmorChat, Troubleshooting, Volume Persistence
  - Include at least one full runnable example `docker run` command with all required flags
  - Use clearly marked placeholders (e.g., `YOUR_API_KEY`, `YOUR_VPS_IP`) for user-specific values — do NOT hardcode real secrets, IPs, or passwords
  - Ensure every required flag is documented (no undocumented required flags)
  - Document the volume persistence requirement: `/var/lib/armorclaw` must be a persistent Docker volume
  - Document the keystore secret requirement: `ARMORCLAW_KEYSTORE_SECRET` must be consistent across container restarts
  - Document bridge-only mode (no Docker socket) for testing/CI
  - Reference the deploy modes (fresh, reuse-existing-matrix, side-by-side) and when to use each

  **Must NOT do**:
  - Do NOT modify README.md or existing docs
  - Do NOT document speculative behavior — only what the code actually does after Task 1 fix
  - Do NOT hide topology/port-conflict caveats

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Documentation-focused task requiring clear technical writing with concrete commands
  - **Skills**: `[]`
  - **Skills Evaluated but Omitted**:
    - None applicable

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 2)
  - **Parallel Group**: Wave 2
  - **Blocks**: F1-F4
  - **Blocked By**: Task 1 (needs to document the post-fix behavior)

  **References** (CRITICAL — Be Exhaustive):

  **Pattern References** (existing code to follow):
  - `README.md` — Existing documentation style and format (concrete commands, tables for modes)
  - `deploy/README.md` — Existing deploy docs for mode descriptions
  - `Dockerfile.quickstart:216-218` — `EXPOSE 8443 5000 6167` and `VOLUME ["/etc/armorclaw", "/var/lib/armorclaw", "/run/armorclaw", "/var/log/armorclaw"]` — required ports and volumes
  - `Dockerfile.quickstart:220-221` — HEALTHCHECK command pattern for reference

  **API/Type References**:
  - `deploy/quickstart-entrypoint.sh:44-78` — Bridge-only mode behavior to document
  - `deploy/quickstart-entrypoint.sh:88-92` — Restart/bootstrap behavior to document
  - `scripts/lib/topology.sh` — Deploy modes to reference in the doc

  **Test References**: None (documentation task)

  **External References**: None

  **WHY Each Reference Matters**:
  - README.md sets the documentation tone and format — new doc should match
  - The VOLUME/EXPOSE directives define what users need to know about ports and persistence
  - The entrypoint behavior after the Task 1 fix is what the docs must describe accurately

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Documentation file exists and is well-structured
    Tool: Bash
    Preconditions: Task 1 changes merged
    Steps:
      1. test -f docs/docker-quickstart.md && echo "EXISTS"
      2. for section in "Prerequisites" "docker run" "Environment Variables" "Health" "ArmorChat" "Troubleshooting" "Volume" "Persistence"; do grep -qi "$section" docs/docker-quickstart.md && echo "FOUND: $section" || echo "MISSING: $section"; done
    Expected Result: File exists, all required sections present
    Failure Indicators: File missing, sections missing
    Evidence: .sisyphus/evidence/docker-ux/task-3-doc-structure.txt

  Scenario: Documentation uses concrete commands with proper placeholder usage
    Tool: Bash
    Preconditions: docs/docker-quickstart.md exists
    Steps:
      1. grep -c "docker run" docs/docker-quickstart.md
      2. grep -c "docker pull" docs/docker-quickstart.md
      3. Verify at least one FULL runnable example (grep for "-e OPENROUTER_API_KEY" or similar)
      4. Verify placeholders are clearly marked (grep for "YOUR_" or similar convention)
    Expected Result: At least 1 full docker run example, 1 docker pull, placeholders clearly marked for user-specific values
    Failure Indicators: No runnable examples, or hardcoded real secrets/IPs
    Evidence: .sisyphus/evidence/docker-ux/task-3-concrete-commands.txt

  Scenario: Documents keystore secret persistence requirement
    Tool: Bash
    Preconditions: docs/docker-quickstart.md exists
    Steps:
      1. grep -qi "ARMORCLAW_KEYSTORE_SECRET" docs/docker-quickstart.md
      2. grep -qi "volume" docs/docker-quickstart.md
    Expected Result: Both keystore secret and volume persistence documented
    Failure Indicators: Critical persistence info missing
    Evidence: .sisyphus/evidence/docker-ux/task-3-persistence.txt

  Scenario: Documents bridge-only mode for testing
    Tool: Bash
    Preconditions: docs/docker-quickstart.md exists
    Steps:
      1. grep -qi "bridge-only" docs/docker-quickstart.md
    Expected Result: Bridge-only mode documented for users without Docker socket
    Failure Indicators: Users have no guidance for testing without full setup
    Evidence: .sisyphus/evidence/docker-ux/task-3-bridge-only.txt
  ```

  **Commit**: YES
  - Message: `docs: add first-time Docker deployment guide`
  - Files: `docs/docker-quickstart.md`
  - Pre-commit: none

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/docker-ux/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `bash -n` on all modified shell scripts. Check for: common shell pitfalls (SC2086, SC2046), empty error handlers, hardcoded paths that should be variables, missing `set -e` guards. Verify no changes to forbidden files (container-setup.sh, setup-quick.sh, Go source, HEALTHCHECK, CI workflow).
  Output: `Shell Check [N clean/N issues] | Forbidden Files [CLEAN/N violations] | VERDICT`

- [x] F3. **Automated Integration QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task using automated tools (no human interaction). Build image, run in CI mode, run in full mode with Docker socket, run in bridge-only mode. Test vps-lifecycle stubs via SSH. Verify all evidence files were generated by automated commands. Save consolidated results to `.sisyphus/evidence/docker-ux/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Tasks 1A+1B**: `fix(docker): route entrypoint to Docker-aware quickstart bootstrap` — Dockerfile.quickstart, deploy/quickstart-entrypoint.sh
- **Task 2**: `feat(deploy): complete reuse-existing-matrix and side-by-side deploy modes` — scripts/vps-lifecycle.sh
- **Task 3**: `docs: add first-time Docker deployment guide` — docs/docker-quickstart.md

---

## Success Criteria

### Verification Commands
```bash
# Build succeeds
docker build -f Dockerfile.quickstart -t armorclaw-test .  # Expected: exit 0

# CI mode stays alive (bridge-only path starts bridge)
docker run -d --name ci-test -e GITHUB_ACTIONS=true armorclaw-test
sleep 10
docker inspect -f '{{.State.Running}}' ci-test  # Expected: true
docker rm -f ci-test

# Existing test suite passes
bash tests/test-quickstart-entrypoint.sh  # Expected: "All tests passed!"

# Docs exist with required sections
grep -qi "docker run" docs/docker-quickstart.md  # Expected: exit 0
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass
- [ ] CI pipeline passes on push
