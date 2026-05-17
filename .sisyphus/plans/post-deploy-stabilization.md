# Post-Deploy Stabilization and Feature Validation

## TL;DR

> **Quick Summary**: Stabilize the ArmorClaw Docker deployment by fixing the unattended startup path, resolving the YARA rule compilation error that silently disables malware scanning, investigating studio RPC surface and registering only validated methods, validating Matrix/agent/browser feature flows, and cleaning up broken test scripts.
> 
> **Deliverables**:
> - Working unattended Docker startup with bridge responding to health.check
> - YARA rules compiling and scanning successfully
> - Studio RPC methods investigated and correctly registered (investigation gate before registration)
> - Matrix e2e message flow validated (both Bridge RPC adapter AND real Matrix client path)
> - Clean test suite with no phantom method calls
> - Production logging cleanup in Go code (pkg/, internal/)
> 
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 5 waves + final verification
> **Critical Path**: T1 (startup guard) → T3 (bridge exec) → T6 (YARA fix) → T9 (Matrix RPC) → T13 (studio investigation) → T14 (studio registration) → T18 (test cleanup) → T22 (logging) → F1-F4

---

## Context

### Original Request
Post-deploy stabilization and feature validation after completing the initial 11/11 post-deploy-fixes plan. VPS deployment at 5.183.11.149 shows 9/10 tests passing but several stability gaps: YARA silently disabled, CI mode blocks before Matrix, config overwritten on restart, phantom test methods.

### Interview Summary
**Key Discussions**:
- YARA bug: `exploit_kit_landing` rule has unreferenced `$pdf_exploit` string — hard compilation error disables all malware scanning
- CI mode: `setup-quick.sh` L1693 `exec tail -f /dev/null` blocks before Matrix setup
- Config overwrite: `generate_config()` unconditionally overwrites config.toml with no guard
- Studio routing: 24 studio methods defined but only `studio.deploy` and `studio.stats` registered in handler map
- `config.list` confirmed NOT in RPC handler map — test expectation is invalid

**Research Findings**:
- Three startup paths exist: Docker quickstart (entrypoint-wrapper → setup-quick.sh), legacy Docker (quickstart-entrypoint.sh → container-setup.sh), bare-metal (setup-quick.sh)
- `quickstart-entrypoint.sh` is dead code in Docker path — copied but never invoked
- Bridge binary NOT started in container path — only systemd (bare-metal) or setup-quick.sh `start_bridge()` start it
- YARA `InitYARA()` failure is non-fatal by design (bridge runs with scanning disabled)
- 109 RPC methods registered; several test scripts call phantom unregistered methods
- `test-attach-config.sh` entirely broken (all 6 tests call unregistered `attach_config`)
- `test-element-x-flow.sh` also has 12 phantom `attach_config` calls (out of scope for this plan)

### Metis Review
**Identified Gaps** (addressed):
- Bridge startup in Docker container must be verified (health.check RPC)
- YARA path resolution is relative — needs absolute path check
- Structured logging scope must be bounded to non-test production code only
- `studio.deploy` registered but not in canonical StudioMethods list — needs investigation
- CI mode must preserve container-alive behavior
- Config guard should follow `quickstart-entrypoint.sh:59` pattern

---

## Work Objectives

### Core Objective
Stabilize the ArmorClaw Docker deployment so that: (1) the bridge starts reliably in unattended mode, (2) YARA malware scanning is functional, (3) key feature flows (Matrix, studio, trust, browser) are validated, and (4) the test suite reflects reality.

### Concrete Deliverables
- `deploy/setup-quick.sh` — config preservation guard + CI mode fix + bridge exec as PID 1
- `bridge/configs/yara_rules.yar` — fixed `exploit_kit_landing` condition
- `bridge/pkg/rpc/server.go` — validated studio methods registered in handler map based on T13 investigation
- `bridge/internal/adapter/matrix.go` — fmt.Printf replaced with structured logging
- Test scripts cleaned: phantom methods removed from `test-matrix-integration.sh`, `test-attach-config.sh` removed, `test-secret-passing.sh` fixed
- New test: `tests/test-yara-smoke.sh` (YARA compilation + runtime scan)

### Definition of Done
- [ ] `docker run armorclaw:latest` starts bridge and responds to `health.check` RPC
- [ ] `yara -p 1 /opt/armorclaw/configs/yara_rules.yar /dev/null` exits 0 inside container
- [ ] All studio methods classified as valid-for-Bridge by T13 return NOT -32601; stale methods documented
- [ ] `matrix.login → matrix.send → matrix.receive` flow works through bridge RPC
- [ ] VPS test suite: 9/9 PASS (config.list test removed)
- [ ] Zero `fmt.Printf`/`fmt.Println` in `pkg/` and `internal/` non-test Go files

### Must Have
- Bridge starts and responds to health.check inside Docker container
- YARA rules compile without errors and scan attachments
- Studio methods investigated, validated methods registered, stale methods documented
- Matrix e2e flow works (both Bridge RPC adapter and real client messaging)
- Test suite reflects actual RPC method registry

### Must NOT Have (Guardrails)
- Do NOT modify `container-setup.sh` or `deploy/container-setup.sh`
- Do NOT touch `bridge/pkg/yara/scanner.go` internal logic
- Do NOT change `InitYARA()` to be fatal on error
- Do NOT add new RPC methods (only register existing studio handlers)
- Do NOT add catch-all/wildcard RPC dispatch for `studio.*`
- Do NOT sanitize `att.Filename` in YARA temp path (file as follow-up)
- Do NOT add config versioning/migration logic
- Do NOT touch `*_test.go` files in the logging task
- Do NOT touch test files beyond specifically named scripts
- Do NOT touch healthcheck in Dockerfile.quickstart
- Do NOT modify `test-element-x-flow.sh` (out of scope — 12 phantom calls noted)
- Do NOT log or print password values anywhere

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** - ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (VPS test suite, local test scripts)
- **Automated tests**: Tests-after (fix then verify)
- **Framework**: bash test scripts + curl/socat for RPC + go test for unit

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/post-deploy/`.

- **RPC verification**: Use Bash (curl/socat) — Send JSON-RPC to bridge socket, assert response fields
- **YARA verification**: Use Bash — Run yara CLI inside Docker container, check exit code
- **Test suite**: Use Bash (ssh) — Run VPS test suite on 5.183.11.149, parse PASS/FAIL
- **Logging audit**: Use ast_grep_search — Find remaining fmt.Printf in production code

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately - startup stabilization):
├── T1: Config preservation guard in generate_config() [quick]
├── T2: Separate CI vs production startup paths [quick]
├── T3: Bridge startup as PID 1 in container [unspecified-high]
├── T4: Restart resilience (.bootstrapped flag + bridge restart) [quick]
└── T5: Remove dead quickstart-entrypoint.sh from Dockerfile [quick]

Wave 2 (After Wave 1 - YARA fix + runtime verification):
├── T6: Fix YARA exploit_kit_landing unreferenced string [quick]
├── T7: YARA runtime smoke test script [unspecified-high]
└── T8: Verify YARA path resolution in Docker container [quick]

Wave 3 (After Wave 2 - Matrix e2e validation):
├── T9: Matrix Bridge RPC adapter flow test [deep]
├── T10: Matrix real client message flow test [unspecified-high]
├── T11: Token-expiry recovery test (re-login after 401) [unspecified-high]
└── T12: Provisioning.start + provisioning.claim smoke test [unspecified-high]

Wave 4 (After Wave 3 - feature validation):
├── T13: Studio method investigation (determine which methods are real vs stale) [deep]
├── T14: Studio method registration based on investigation [unspecified-high]
├── T15: Studio agent lifecycle validation (create→list→spawn→stop) [deep]
├── T16: Trust/PII flow validation (request→approve→fulfill) [unspecified-high]
└── T17: Browser/Jetski smoke (navigate→status→complete) [unspecified-high]

Wave 5 (After Wave 4 - test suite cleanup + logging):
├── T18: Remove config.list test from VPS smoke suite [quick]
├── T19: Remove phantom methods from test-matrix-integration.sh [quick]
├── T20: Remove test-attach-config.sh (entirely broken) [quick]
├── T21: Fix test-secret-passing.sh phantom methods [quick]
└── T22: Remove fmt.Printf from production code (not structured logging) [unspecified-high]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── F1: Startup audit — bridge starts in Docker, CI mode, restart resilience (oracle)
├── F2: YARA + feature audit — YARA scanning works, studio methods respond (unspecified-high)
├── F3: Test suite integrity — VPS 9/9, no phantom methods, zero broken tests (unspecified-high)
└── F4: Scope fidelity check — no guardrail violations, no scope creep (deep)
→ Present results → Get explicit user okay

Critical Path: T1 → T3 → T6 → T9 → T13 → T14 → T18 → T22 → F1-F4
Parallel Speedup: ~65% faster than sequential
Max Concurrent: 5 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1   | - | T3, T4 | 1 |
| T2   | - | T3 | 1 |
| T3   | T1, T2 | T6 | 1 |
| T4   | T1 | T6 | 1 |
| T5   | - | - | 1 |
| T6   | T3, T4 | T7, T8 | 2 |
| T7   | T6 | T9 | 2 |
| T8   | T6 | T9 | 2 |
| T9   | T7, T8 | T10, T11, T12 | 3 |
| T10  | T9 | T13 | 3 |
| T11  | T9 | T13 | 3 |
| T12  | T9 | T13 | 3 |
| T13  | T10, T11, T12 | T14 | 4 |
| T14  | T13 | T15, T16, T17 | 4 |
| T15  | T14 | T18 | 4 |
| T16  | - | T18 | 4 |
| T17  | - | T18 | 4 |
| T18  | T15, T16, T17 | T22 | 5 |
| T19  | - | - | 5 |
| T20  | - | - | 5 |
| T21  | - | - | 5 |
| T22  | T18 | F1-F4 | 5 |
| F1-F4 | T22 | user okay | FINAL |

### Agent Dispatch Summary

- **Wave 1**: **5** — T1→`quick`, T2→`quick`, T3→`unspecified-high`, T4→`quick`, T5→`quick`
- **Wave 2**: **3** — T6→`quick`, T7→`unspecified-high`, T8→`quick`
- **Wave 3**: **4** — T9→`deep`, T10→`unspecified-high`, T11→`unspecified-high`, T12→`unspecified-high`
- **Wave 4**: **5** — T13→`deep`, T14→`unspecified-high`, T15→`deep`, T16→`unspecified-high`, T17→`unspecified-high`
- **Wave 5**: **5** — T18-T21→`quick`, T22→`unspecified-high`
- **FINAL**: **4** — F1→`oracle`, F2→`unspecified-high`, F3→`unspecified-high`, F4→`deep`

---

## TODOs

- [x] T1. Config Preservation Guard in generate_config()

  **What to do**:
  - In `deploy/setup-quick.sh`, modify the `generate_config()` function (starting at line 1009) to check if `config.toml` already exists before overwriting
  - Follow the pattern from `quickstart-entrypoint.sh:59`: `if [ ! -f "$config_file" ]` guard
  - Add a `FORCE_REGEN` flag (default false) that can override the guard if needed
  - The guard should be: if config exists AND FORCE_REGEN is not set, log "Config exists, preserving" and return
  - Do NOT add config versioning, backup, or migration logic

  **Must NOT do**:
  - Do NOT add config versioning or migration
  - Do NOT modify container-setup.sh
  - Do NOT change the config.toml format or schema

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T2, T3, T4, T5)
  - **Blocks**: T3, T4
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `deploy/quickstart-entrypoint.sh:59` - Config guard pattern: `if [ ! -f "$CONFIG_DIR/config.toml" ]` before copying template. Follow this exact pattern.

  **API/Type References**:
  - `deploy/setup-quick.sh:1009-1105` - The `generate_config()` function to modify. Line 1029 has the unconditional `cat > "$config_file"` that needs the guard.

  **WHY Each Reference Matters**:
  - quickstart-entrypoint.sh:59 shows the canonical guard pattern already used in the codebase
  - setup-quick.sh:1029 is the exact line where the overwrite happens

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Config preserved on second run
    Tool: Bash (ssh)
    Preconditions: VPS at 5.183.11.149 has existing config.toml with custom values
    Steps:
      1. SSH to VPS, read current config: `ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'cat /etc/armorclaw/config.toml'`
      2. Save a unique value: `grep 'server_name' /etc/armorclaw/config.toml`
      3. Re-run generate_config portion: `ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'bash /opt/armorclaw/quickstart.sh --non-interactive'`
      4. Check config unchanged: `ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'cat /etc/armorclaw/config.toml'`
      5. Assert: original server_name value still present
    Expected Result: Config file content identical after re-run
    Failure Indicators: server_name value changed or config.toml has different content
    Evidence: .sisyphus/evidence/post-deploy/task-1-config-preserved.txt

  Scenario: Force regen works when flag set
    Tool: Bash (ssh)
    Preconditions: VPS has existing config.toml
    Steps:
      1. SSH to VPS, note current config hash: `md5sum /etc/armorclaw/config.toml`
      2. Run with FORCE_REGEN: `FORCE_REGEN=true bash /opt/armorclaw/quickstart.sh --non-interactive`
      3. Check config regenerated: `md5sum /etc/armorclaw/config.toml`
    Expected Result: Config file content differs (regenerated)
    Evidence: .sisyphus/evidence/post-deploy/task-1-force-regen.txt
  ```

  **Commit**: YES (groups with Wave 1)
  - Message: `fix(startup): preserve config, separate CI/production, bridge as PID 1, restart resilience`
  - Files: `deploy/setup-quick.sh`
  - Pre-commit: `grep -c 'config_file' deploy/setup-quick.sh` (verify changes present)

- [x] T2. Separate CI vs Production Startup Paths

  **What to do**:
  - In `deploy/setup-quick.sh`, modify the CI_MODE block (lines 1691-1694) so it does NOT block before Matrix setup
  - Current behavior: `exec tail -f /dev/null` at L1693 runs AFTER bridge start but BEFORE Matrix setup (L1696-1714)
  - New behavior: CI mode should complete Matrix setup (ensure_matrix at L1704) and THEN block with `exec tail -f /dev/null`
  - Move the CI tail block to AFTER `ensure_matrix()` completes (after line 1714)
  - Preserve the container-alive behavior — do NOT remove `exec tail -f /dev/null`
  - In CI mode, `ensure_matrix()` must use auto-generated credentials (no interactive prompts)

  **Must NOT do**:
  - Do NOT remove `exec tail -f /dev/null`
  - Do NOT change ensure_matrix() logic
  - Do NOT add interactive prompts in CI mode

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T3, T4, T5)
  - **Blocks**: T3
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `deploy/setup-quick.sh:1666-1757` - The `main()` function showing the complete flow
  - `deploy/setup-quick.sh:1691-1694` - Current CI_MODE block that blocks before Matrix
  - `deploy/setup-quick.sh:1696-1714` - Matrix setup (ensure_matrix, prompt_api_key, generate_qr) that CI mode currently skips
  - `deploy/setup-quick.sh:441-561` - The `ensure_matrix()` function — auto-creates Conduit, registers users

  **WHY Each Reference Matters**:
  - Lines 1691-1694 show exactly where the CI block is and what it prevents from running
  - Lines 1696-1714 show what CI mode is missing (Matrix setup)
  - ensure_matrix() shows it already handles non-interactive mode (auto-generates passwords)

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: CI mode completes Matrix setup before blocking
    Tool: Bash (ssh)
    Preconditions: VPS at 5.183.11.149, Docker available
    Steps:
      1. SSH to VPS
      2. Run quickstart in CI mode: `CI=true bash /opt/armorclaw/quickstart.sh --non-interactive`
      3. Wait 30 seconds, then check Conduit: `curl -sf http://localhost:6167/_matrix/client/versions`
      4. Check bridge health: `echo '{"jsonrpc":"2.0","id":1,"method":"health.check"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
    Expected Result: Conduit responds with versions JSON, bridge responds with health ok
    Failure Indicators: Conduit not running or bridge not responding
    Evidence: .sisyphus/evidence/post-deploy/task-2-ci-matrix.txt

  Scenario: CI mode keeps container alive
    Tool: Bash (ssh)
    Preconditions: CI mode running
    Steps:
      1. Check if tail process is running: `ps aux | grep 'tail -f /dev/null'`
    Expected Result: tail process found (container stays alive)
    Failure Indicators: No tail process, container exited
    Evidence: .sisyphus/evidence/post-deploy/task-2-ci-alive.txt
  ```

  **Commit**: YES (groups with Wave 1)
  - Message: `fix(startup): preserve config, separate CI/production, bridge as PID 1, restart resilience`
  - Files: `deploy/setup-quick.sh`

- [x] T3. Bridge Startup as PID 1 in Container

  **What to do**:
  - In `deploy/setup-quick.sh`, modify `start_bridge()` (lines 1195-1237) so that when running inside Docker:
    1. Detect Docker environment (check `/.dockerenv` exists OR `$container` env var)
    2. Instead of running bridge in background (`&`), `exec` the bridge binary as PID 1
    3. This ensures the bridge process receives signals and the container stays alive
  - Current problem: `start_bridge()` backgrounds the process (`armorclaw-bridge -config $CONFIG_DIR/config.toml &`), then setup-quick.sh finishes, and the container exits killing the backgrounded bridge
  - Fix: In Docker mode, after setup completes, exec the bridge as the final command (PID 1)
  - The CI mode `exec tail -f /dev/null` will handle keepalive in CI; in production Docker, exec the bridge itself
  - **Process model clarification**: Production Docker path → Bridge is PID 1. CI validation path → setup completes (including Matrix), then keepalive (`tail -f /dev/null`) may be PID 1. These are two separate runtime contracts.

  **Must NOT do**:
  - Do NOT change the bare-metal/systemd path
  - Do NOT change bridge binary arguments or flags
  - Do NOT touch Dockerfile.quickstart HEALTHCHECK

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T4, T5)
  - **Blocks**: T6, T7, T8
  - **Blocked By**: T1, T2

  **References**:
  **Pattern References**:
  - `deploy/setup-quick.sh:1195-1237` - The `start_bridge()` function. Lines 1202-1208 show the non-systemd background mode that breaks in Docker.
  - `deploy/setup-quick.sh:1666-1757` - The `main()` function showing where `start_bridge()` is called (line 1688) and where the script ends.

  **API/Type References**:
  - `Dockerfile.quickstart:241` - WORKDIR /opt/armorclaw sets the container working directory
  - `Dockerfile.quickstart:222` - HEALTHCHECK uses `socat - UNIX-CONNECT:/run/armorclaw/bridge.sock` — confirms bridge must create this socket

  **WHY Each Reference Matters**:
  - start_bridge() L1202-1208 shows the background mode that loses the bridge when the shell exits
  - Dockerfile WORKDIR and HEALTHCHECK define the runtime environment the bridge must operate in

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Bridge is PID 1 in Docker container
    Tool: Bash (ssh)
    Preconditions: Container rebuilt and deployed to VPS
    Steps:
      1. SSH to VPS
      2. Find container: `docker ps --filter ancestor=mikegemut/armorclaw:latest --format '{{.ID}}'`
      3. Check PID 1: `docker exec <container_id> cat /proc/1/cmdline | tr '\0' ' '`
    Expected Result: Output contains `armorclaw-bridge` (bridge is PID 1)
    Failure Indicators: PID 1 is `tail`, `bash`, or anything else
    Evidence: .sisyphus/evidence/post-deploy/task-3-pid1.txt

  Scenario: Bridge responds to health.check after startup
    Tool: Bash (ssh)
    Preconditions: Container running on VPS
    Steps:
      1. SSH to VPS
      2. Call health RPC: `echo '{"jsonrpc":"2.0","id":1,"method":"health.check"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
    Expected Result: `{"jsonrpc":"2.0","id":1,"result":{"status":"ok"}}`
    Failure Indicators: Connection refused, timeout, or error response
    Evidence: .sisyphus/evidence/post-deploy/task-3-health.txt

  Scenario: Bridge receives SIGTERM and shuts down cleanly
    Tool: Bash (ssh)
    Preconditions: Container running on VPS
    Steps:
      1. `docker stop --time 10 <container_id>`
      2. Check exit code: `docker inspect <container_id> --format '{{.State.ExitCode}}'`
    Expected Result: Exit code 0 (clean shutdown)
    Failure Indicators: Exit code 137 (SIGKILL) or other non-zero
    Evidence: .sisyphus/evidence/post-deploy/task-3-signal.txt
  ```

  **Commit**: YES (groups with Wave 1)
  - Message: `fix(startup): preserve config, separate CI/production, bridge as PID 1, restart resilience`
  - Files: `deploy/setup-quick.sh`

- [x] T4. Restart Resilience (.bootstrapped Flag + Bridge Restart)

  **What to do**:
  - In `deploy/setup-quick.sh`, add restart-detection logic at the beginning of `main()`:
    1. Check for `/opt/armorclaw/.bootstrapped` flag file
    2. If flag exists, skip setup steps (prerequisites, install, config generation, keystore init)
    3. Go directly to starting the bridge (exec as PID 1 in Docker)
    4. This ensures container restart doesn't re-run the entire setup
  - Create the `.bootstrapped` flag file at the end of the setup flow (after verify_health succeeds)
  - Note: `quickstart-entrypoint.sh:89-92` already checks `.bootstrapped` — follow this pattern

  **Must NOT do**:
  - Do NOT delete .bootstrapped on failure (let re-runs recover)
  - Do NOT add health checking beyond what already exists
  - Do NOT modify the verify_health function

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T3, T5)
  - **Blocks**: T6
  - **Blocked By**: T1

  **References**:
  **Pattern References**:
  - `deploy/quickstart-entrypoint.sh:89-92` - Bootstrap flag check pattern: `if [ -f "/opt/armorclaw/.bootstrapped" ]; then exec container-setup.sh "$@"; fi`
  - `deploy/quickstart-entrypoint.sh:371` - Flag creation: `touch /opt/armorclaw/.bootstrapped`
  - `deploy/setup-quick.sh:1666-1757` - The `main()` function to modify

  **WHY Each Reference Matters**:
  - quickstart-entrypoint.sh shows the canonical bootstrapped flag pattern
  - setup-quick.sh main() shows where to add the check and where to create the flag

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Container restart skips setup, starts bridge immediately
    Tool: Bash (ssh)
    Preconditions: Container deployed and bootstrapped on VPS
    Steps:
      1. SSH to VPS, check .bootstrapped exists: `docker exec <container> ls -la /opt/armorclaw/.bootstrapped`
      2. Restart container: `docker restart <container_id>`
      3. Wait 10 seconds
      4. Check bridge health: `echo '{"jsonrpc":"2.0","id":1,"method":"health.check"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
    Expected Result: Bridge responds with health ok after restart, no re-setup
    Failure Indicators: Setup re-runs (long startup time), bridge not responding
    Evidence: .sisyphus/evidence/post-deploy/task-4-restart.txt

  Scenario: Config preserved across restart
    Tool: Bash (ssh)
    Preconditions: Container with custom config.toml
    Steps:
      1. Before restart: `docker exec <container> md5sum /etc/armorclaw/config.toml`
      2. Restart container: `docker restart <container_id>`
      3. After restart: `docker exec <container> md5sum /etc/armorclaw/config.toml`
    Expected Result: MD5 hashes match (config unchanged)
    Failure Indicators: Hashes differ (config overwritten)
    Evidence: .sisyphus/evidence/post-deploy/task-4-config-stable.txt
  ```

  **Commit**: YES (groups with Wave 1)
  - Message: `fix(startup): preserve config, separate CI/production, bridge as PID 1, restart resilience`
  - Files: `deploy/setup-quick.sh`

- [x] T5. Remove Dead quickstart-entrypoint.sh from Dockerfile

  **What to do**:
  - In `Dockerfile.quickstart`, remove line 154: `COPY deploy/quickstart-entrypoint.sh /opt/armorclaw/quickstart-entrypoint.sh`
  - This script is never invoked by `entrypoint-wrapper.sh` — it calls `quickstart.sh` (setup-quick.sh) instead
  - The `quickstart-entrypoint.sh` is dead code in the Docker path, adding confusion
  - Do NOT delete the file itself (it may be used by other deployment paths) — only remove the Dockerfile COPY

  **Must NOT do**:
  - Do NOT delete `deploy/quickstart-entrypoint.sh` from the repo
  - Do NOT change any other Dockerfile lines
  - Do NOT modify entrypoint-wrapper.sh

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T3, T4)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `Dockerfile.quickstart:154` - The COPY line to remove
  - `Dockerfile.quickstart:225-237` - The entrypoint-wrapper.sh heredoc showing it does NOT call quickstart-entrypoint.sh

  **WHY Each Reference Matters**:
  - Line 154 is the exact line to remove
  - Lines 225-237 prove the entrypoint wrapper never calls this script

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: quickstart-entrypoint.sh not in Docker image
    Tool: Bash (ssh)
    Preconditions: Docker image rebuilt
    Steps:
      1. Build image: `docker buildx build --load -t armorclaw:test -f Dockerfile.quickstart .`
      2. Check for file: `docker run --rm armorclaw:test ls /opt/armorclaw/quickstart-entrypoint.sh`
    Expected Result: `ls: cannot access` error (file not in image)
    Failure Indicators: File found in image
    Evidence: .sisyphus/evidence/post-deploy/task-5-dead-code-removed.txt

  Scenario: Docker image still builds and runs correctly
    Tool: Bash (ssh)
    Preconditions: Image built
    Steps:
      1. `docker run --rm armorclaw:test echo "build ok"`
    Expected Result: Container runs without error
    Evidence: .sisyphus/evidence/post-deploy/task-5-build-ok.txt
  ```

  **Commit**: YES (separate)
  - Message: `chore: remove dead quickstart-entrypoint.sh from Docker image`
  - Files: `Dockerfile.quickstart`

- [x] T6. Fix YARA exploit_kit_landing Unreferenced String

  **What to do**:
  - In `bridge/configs/yara_rules.yar`, fix rule `exploit_kit_landing` (lines 101-112)
  - The string `$pdf_exploit` is defined at line 109 but NOT referenced in the condition at line 111
  - Current condition (line 111): `($iframe_inject and $object_cls) or $java_webkit`
  - Fix: Add `$pdf_exploit` to the condition. Use: `($iframe_inject and $object_cls) or $java_webkit or $pdf_exploit`
  - This is the ONLY broken rule — the other 6 rules in the file are correct
  - This fix is CRITICAL: without it, YARA compilation fails entirely, silently disabling ALL malware scanning

  **Must NOT do**:
  - Do NOT touch scanner.go or InitYARA() error handling
  - Do NOT change InitYARA() to be fatal
  - Do NOT modify any other YARA rules
  - Do NOT add new YARA rules

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T7, T8)
  - **Blocks**: T7, T8
  - **Blocked By**: T3, T4

  **References**:
  **Pattern References**:
  - `bridge/configs/yara_rules.yar:101-112` - The broken rule. Line 109 defines `$pdf_exploit`, line 111 is the condition that doesn't reference it.

  **API/Type References**:
  - `bridge/pkg/yara/scanner.go` - InitYARA() compiles rules via `compiler.AddFile()` + `GetRules()`. Read-only context.
  - `bridge/cmd/bridge/main.go:2256-2258` - Shows InitYARA failure only logs warning (non-fatal by design)

  **External References**:
  - YARA docs: `ERROR_UNREFERENCED_STRING` is a hard compilation error. Strings must be referenced in condition or prefixed with `_` (YARA ≥ 4.5.0).

  **WHY Each Reference Matters**:
  - yara_rules.yar:109/111 is the exact fix location
  - scanner.go shows compilation fails hard on this error
  - main.go shows the security impact: YARA failure disables ALL malware scanning silently

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: YARA rules compile without errors
    Tool: Bash
    Preconditions: Docker image rebuilt with fix, deployed to VPS
    Steps:
      1. SSH to VPS: `ssh -i ~/.ssh/openclaw_win root@5.183.11.149`
      2. Find container: `docker ps --filter ancestor=mikegemut/armorclaw:latest --format '{{.ID}}'`
      3. Compile test: `docker exec <cid> yara -p 1 /opt/armorclaw/configs/yara_rules.yar /dev/null`
    Expected Result: Exit code 0 (compilation succeeded)
    Failure Indicators: Non-zero exit code with ERROR_UNREFERENCED_STRING message
    Evidence: .sisyphus/evidence/post-deploy/task-6-yara-compile.txt

  Scenario: YARA scanning is enabled in bridge
    Tool: Bash (ssh)
    Preconditions: Bridge running with fixed YARA rules
    Steps:
      1. Check bridge logs for YARA init: `docker logs <cid> 2>&1 | grep -i yara`
    Expected Result: No "YARA initialization failed" warning in logs
    Failure Indicators: "Warning: YARA initialization failed" still appears
    Evidence: .sisyphus/evidence/post-deploy/task-6-yara-enabled.txt
  ```

  **Commit**: YES (separate)
  - Message: `fix(yara): reference $pdf_exploit in exploit_kit_landing condition`
  - Files: `bridge/configs/yara_rules.yar`

- [x] T7. YARA Runtime Smoke Test Script

  **What to do**:
  - Create `tests/test-yara-smoke.sh` that validates YARA scanning works end-to-end:
    1. Check YARA rules compile (`yara -p 1 /opt/armorclaw/configs/yara_rules.yar /dev/null`)
    2. Create a test file with a known malicious pattern (one of the patterns from yara_rules.yar)
    3. Run YARA scan against the test file, verify it detects the pattern
    4. Run YARA scan against a clean file, verify no detection
    5. Test bridge RPC: create an email attachment scenario, trigger YARA scan via the ingest path
  - Follow existing test script patterns from `tests/test-vps-smoke.sh` (Tier A: SSH + curl)
  - Source `tests/lib/load_env.sh` for VPS connectivity
  - Use `tests/lib/assert_json.sh` for JSON assertions

  **Must NOT do**:
  - Do NOT modify scanner.go
  - Do NOT add new YARA rules
  - Do NOT change YARA scan API

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T6, T8)
  - **Blocks**: T9
  - **Blocked By**: T6

  **References**:
  **Pattern References**:
  - `tests/test-vps-smoke.sh` - Tier A test pattern: SSH to VPS, run commands, parse output. Follow this structure.
  - `tests/lib/load_env.sh` - Environment loader for VPS_IP, ADMIN_TOKEN
  - `tests/lib/assert_json.sh` - JSON assertion helpers

  **API/Type References**:
  - `bridge/configs/yara_rules.yar` - Contains all YARA rules with string patterns to use as test data
  - `bridge/pkg/email/ingest_server.go:152-165` - The email attachment YARA scan path (write temp file → scan → cleanup)

  **WHY Each Reference Matters**:
  - test-vps-smoke.sh shows the canonical test structure for this project
  - yara_rules.yar has the actual patterns to use as positive test cases
  - ingest_server.go:152-165 shows how YARA scanning works at runtime

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: YARA smoke test passes all checks
    Tool: Bash
    Preconditions: VPS deployed with fixed YARA rules
    Steps:
      1. Run: `bash tests/test-yara-smoke.sh`
      2. Check exit code: `echo $?`
    Expected Result: Exit code 0, output shows "YARA Smoke: N/N PASS"
    Failure Indicators: Non-zero exit, any test shows FAIL
    Evidence: .sisyphus/evidence/post-deploy/task-7-yara-smoke.txt

  Scenario: YARA detects known malicious pattern
    Tool: Bash (ssh)
    Preconditions: VPS deployed
    Steps:
      1. SSH to VPS
      2. Create test file with iframe injection pattern: `echo '<iframe src="javascript:alert(1)">' > /tmp/test-malware.html`
      3. Scan: `docker exec <cid> yara /opt/armorclaw/configs/yara_rules.yar /tmp/test-malware.html`
    Expected Result: Output contains rule name match (e.g., `exploit_kit_landing`)
    Failure Indicators: No match output (YARA not detecting the pattern)
    Evidence: .sisyphus/evidence/post-deploy/task-7-yara-detect.txt
  ```

  **Commit**: YES (groups with T8)
  - Message: `test(yara): add smoke test and verify path resolution`
  - Files: `tests/test-yara-smoke.sh`

- [x] T8. Verify YARA Path Resolution in Docker Container

  **What to do**:
  - Verify that the YARA rules path resolves correctly inside the Docker container
  - `main.go:2255` uses `filepath.Join("configs", "yara_rules.yar")` — a relative path
  - Docker WORKDIR is `/opt/armorclaw` (Dockerfile.quickstart:241)
  - Rules are copied to `/opt/armorclaw/configs/yara_rules.yar` (Dockerfile.quickstart:204)
  - Verify: inside a running container, the relative path resolves correctly
  - If it doesn't resolve (e.g., WORKDIR changed), add an absolute path fallback in `main.go`:
    - Check `/opt/armorclaw/configs/yara_rules.yar` if relative path fails
    - Do NOT change the relative path default (bare-metal relies on it)
  - Log which YARA path is being used for debugging

  **Must NOT do**:
  - Do NOT hardcode only the absolute path
  - Do NOT change scanner.go
  - Do NOT change WORKDIR in Dockerfile

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T6, T7)
  - **Blocks**: T9
  - **Blocked By**: T6

  **References**:
  **Pattern References**:
  - `bridge/cmd/bridge/main.go:2255-2258` - YARA init with relative path: `yaraRulesPath := filepath.Join("configs", "yara_rules.yar")`
  - `Dockerfile.quickstart:204` - YARA rules copy: `COPY bridge/configs/yara_rules.yar /opt/armorclaw/configs/yara_rules.yar`
  - `Dockerfile.quickstart:241` - WORKDIR: `/opt/armorclaw`

  **WHY Each Reference Matters**:
  - main.go:2255 shows the relative path that may fail if CWD changes
  - Dockerfile lines confirm the expected absolute path in the container

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: YARA rules found at relative path in Docker
    Tool: Bash (ssh)
    Preconditions: Container running on VPS
    Steps:
      1. SSH to VPS
      2. Check file exists: `docker exec <cid> ls -la /opt/armorclaw/configs/yara_rules.yar`
      3. Check bridge CWD: `docker exec <cid> ls -la configs/yara_rules.yar` (relative path)
    Expected Result: Both paths show the same file (872 bytes, yara_rules.yar)
    Failure Indicators: Relative path not found
    Evidence: .sisyphus/evidence/post-deploy/task-8-yara-path.txt

  Scenario: Absolute path fallback works
    Tool: Bash (ssh)
    Preconditions: If fallback was added
    Steps:
      1. Check bridge logs: `docker logs <cid> 2>&1 | grep -i 'yara.*path'`
    Expected Result: Log shows which YARA path was resolved
    Failure Indicators: No path log or "YARA initialization failed"
    Evidence: .sisyphus/evidence/post-deploy/task-8-yara-path-log.txt
  ```

  **Commit**: YES (groups with T7)
  - Message: `test(yara): add smoke test and verify path resolution`
  - Files: `bridge/cmd/bridge/main.go` (if path fallback added), `tests/test-yara-smoke.sh`

- [x] T9. Matrix E2E Validation: Bridge RPC Adapter Flow

  **What to do**:
  - Create `tests/test-matrix-e2e-rpc.sh` that validates the Matrix message flow through the **Bridge RPC adapter**:
    1. `matrix.login` — authenticate with bridge, get token
    2. `matrix.join_room` — join a test room (or create one)
    3. `matrix.send` — send a message via bridge RPC
    4. `matrix.receive` — retrieve messages, verify the sent message appears
    5. Clean up test room/messages
  - Follow Tier A pattern (SSH + curl to VPS)
  - **IMPORTANT**: This test validates the BRIDGE RPC ADAPTER ONLY — it proves the bridge's Matrix adapter works for admin/container operations
  - A separate test (T10) validates the REAL Matrix client messaging path (via `/sync`)

  **Must NOT do**:
  - Do NOT test direct Conduit API (use bridge RPC methods only)
  - Do NOT create new Matrix rooms permanently
  - Do NOT modify matrix.go handler code

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T10, T11)
  - **Blocks**: T10, T11
  - **Blocked By**: T7, T8

  **References**:
  **Pattern References**:
  - `tests/test-matrix-plane.sh` - Matrix testing pattern (uses direct Conduit API — T10 follows this pattern instead)
  - `tests/test-vps-smoke.sh` - Tier A SSH+curl pattern to follow
  - `tests/lib/load_env.sh` - VPS_IP, ADMIN_TOKEN loader

  **API/Type References**:
  - `bridge/pkg/rpc/server.go:1219-1333` - Handler map showing registered Matrix methods: `matrix.status`, `matrix.login`, `matrix.send`, `matrix.receive`, `matrix.join_room`
  - `bridge/internal/adapter/matrix.go` - Matrix adapter with login, send, receive handlers

  **Test References**:
  - `tests/test-matrix-integration.sh` - Existing matrix test (has phantom methods — use only the working test patterns from lines 1-200)

  **WHY Each Reference Matters**:
  - server.go handler map confirms which methods are available
  - test-matrix-plane.sh shows how to test Matrix directly (T10 uses this pattern)
  - test-matrix-integration.sh has working patterns for matrix.login and matrix.send that can be adapted

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Bridge RPC Matrix adapter round-trip
    Tool: Bash
    Preconditions: VPS deployed, Conduit running on port 6167, bridge connected
    Steps:
      1. Run: `bash tests/test-matrix-e2e-rpc.sh`
      2. Check exit code: `echo $?`
    Expected Result: Exit code 0, all 5 steps (login, join, send, receive, cleanup) PASS
    Failure Indicators: Any step returns FAIL, message not received
    Evidence: .sisyphus/evidence/post-deploy/task-9-matrix-rpc.txt

  Scenario: matrix.login returns valid session
    Tool: Bash (ssh + socat)
    Preconditions: Bridge running on VPS
    Steps:
      1. Call matrix.login: `echo '{"jsonrpc":"2.0","id":1,"method":"matrix.login","params":{"username":"bridge","password":"bridgepassword123"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
    Expected Result: Response contains `{"jsonrpc":"2.0","id":1,"result":{...}}` without error
    Failure Indicators: -32601 method not found, or error in result
    Evidence: .sisyphus/evidence/post-deploy/task-9-login.txt
  ```

  **Commit**: YES
  - Message: `test(matrix): e2e flow, token recovery, provisioning smoke`
  - Files: `tests/test-matrix-e2e-rpc.sh`

- [x] T10. Matrix E2E Validation: Real Client Message Flow

  **What to do**:
  - Create `tests/test-matrix-client-flow.sh` that validates the REAL user-facing Matrix messaging path:
    1. Login directly to Conduit via `/_matrix/client/v3/login`
    2. Create a room via `/_matrix/client/v3/createRoom`
    3. Send a message via `/_matrix/client/v3/rooms/{roomId}/send/m.room.message/{txnId}`
    4. Sync messages via `/_matrix/client/v3/sync` and verify message appears
    5. This validates the path ArmorChat actually uses (Matrix SDK `/sync`)
  - Follow the pattern from `tests/test-matrix-plane.sh` (direct Conduit API)
  - This test validates the USER-FACING messaging path, not the bridge admin adapter

  **Must NOT do**:
  - Do NOT use bridge RPC methods (this tests the Matrix protocol directly)
  - Do NOT modify any handler code

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T9, T11)
  - **Blocks**: T12
  - **Blocked By**: T9

  **References**:
  **Pattern References**:
  - `tests/test-matrix-plane.sh` - Direct Conduit API test pattern — follow this exactly

  **API/Type References**:
  - Conduit REST API: `/_matrix/client/v3/login`, `/_matrix/client/v3/sync`, `/_matrix/client/v3/rooms/.../send/...`

  **WHY Each Reference Matters**:
  - test-matrix-plane.sh already has the Conduit API testing pattern — replicate for messaging flow

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Matrix client message round-trip via /sync
    Tool: Bash (ssh + curl)
    Preconditions: Conduit running on port 6167
    Steps:
      1. Run: `bash tests/test-matrix-client-flow.sh`
      2. Check exit code
    Expected Result: Exit code 0, message sent appears in /sync response
    Failure Indicators: Message not in sync response
    Evidence: .sisyphus/evidence/post-deploy/task-10-client-flow.txt
  ```

  **Commit**: YES (groups with T9)
  - Message: `test(matrix): e2e flow, token recovery, provisioning smoke`
  - Files: `tests/test-matrix-client-flow.sh`

- [x] T11. Token-Expiry Recovery Test (Re-login After 401)

  **What to do**:
  - Create `tests/test-token-recovery.sh` that validates the bridge handles Matrix token expiry:
    1. Login via `matrix.login`, get access token
    2. Attempt an operation (e.g., `matrix.send`) with the valid token
    3. Simulate token expiry by calling `matrix.send` with an invalid/expired token
    4. Verify the bridge returns an appropriate error (not a crash)
    5. Re-login and verify the new token works
  - Follow Tier A pattern (SSH + curl to VPS)
  - Do NOT actually expire tokens (too slow) — instead test with a deliberately invalid token string
  - Verify bridge doesn't crash or enter a bad state after token error

  **Must NOT do**:
  - Do NOT modify token handling code in matrix.go
  - Do NOT wait for real token expiry (use invalid token instead)
  - Do NOT add `matrix.refresh_token` method (it doesn't exist)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T9, T11)
  - **Blocks**: T12
  - **Blocked By**: T9

  **References**:
  **Pattern References**:
  - `tests/test-vps-smoke.sh` - Tier A pattern
  - `bridge/internal/adapter/matrix.go:ensureValidToken()` - Token refresh logic already in the bridge

  **API/Type References**:
  - `bridge/pkg/rpc/server.go` - `matrix.login`, `matrix.send` methods for testing

  **WHY Each Reference Matters**:
  - ensureValidToken() shows how the bridge handles token refresh internally
  - Testing with invalid token validates error handling without waiting for real expiry

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Bridge handles invalid token gracefully
    Tool: Bash (ssh + socat)
    Preconditions: Bridge running on VPS
    Steps:
      1. Login: `echo '{"jsonrpc":"2.0","id":1,"method":"matrix.login","params":{"username":"bridge","password":"bridgepassword123"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
      2. Send with invalid token: `echo '{"jsonrpc":"2.0","id":2,"method":"matrix.send","params":{"access_token":"invalid_token_12345","room_id":"!test:localhost","message":"test"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
    Expected Result: Error response about invalid token, NOT a crash or hang
    Failure Indicators: Connection refused, timeout, or bridge crash
    Evidence: .sisyphus/evidence/post-deploy/task-11-token-error.txt

  Scenario: Re-login after token error succeeds
    Tool: Bash (ssh + socat)
    Preconditions: Invalid token error just occurred
    Steps:
      1. Re-login: `echo '{"jsonrpc":"2.0","id":3,"method":"matrix.login","params":{"username":"bridge","password":"bridgepassword123"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
    Expected Result: New valid session established
    Failure Indicators: Login fails after previous token error
    Evidence: .sisyphus/evidence/post-deploy/task-11-relogin.txt
  ```

  **Commit**: YES (groups with T9)
  - Message: `test(matrix): e2e flow, token recovery, provisioning smoke`
  - Files: `tests/test-token-recovery.sh`

- [x] T12. Provisioning.start + Provisioning.claim Smoke Test

  **What to do**:
  - Create `tests/test-provisioning.sh` that validates the provisioning lifecycle:
    1. Call `provisioning.start` — should return a provisioning session/token
    2. Call `provisioning.claim` with the session token — should complete provisioning
    3. Verify the provisioned device appears in `device.list`
  - Follow Tier A pattern (SSH + curl to VPS)
  - This is the first test for these registered methods (no test-provisioning.sh exists yet)

  **Must NOT do**:
  - Do NOT add new provisioning methods
  - Do NOT modify provisioning handler code
  - Do NOT test provisioning QR code generation (visual, out of scope)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T9, T10)
  - **Blocks**: T12
  - **Blocked By**: T9

  **References**:
  **Pattern References**:
  - `tests/test-vps-smoke.sh` - Tier A test pattern
  - `tests/lib/load_env.sh` - Environment loader

  **API/Type References**:
  - `bridge/pkg/rpc/server.go` - `provisioning.start` and `provisioning.claim` registered handlers
  - `bridge/pkg/rpc/server.go` - `device.list` handler for post-provisioning verification

  **WHY Each Reference Matters**:
  - server.go confirms provisioning methods are registered and available
  - device.list is needed to verify the provisioned device was created

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Provisioning lifecycle completes
    Tool: Bash
    Preconditions: Bridge running on VPS, admin token available
    Steps:
      1. Run: `bash tests/test-provisioning.sh`
      2. Check exit code: `echo $?`
    Expected Result: Exit code 0, all 3 steps (start, claim, verify) PASS
    Failure Indicators: Any step returns FAIL
    Evidence: .sisyphus/evidence/post-deploy/task-12-provisioning.txt

  Scenario: provisioning.start returns session token
    Tool: Bash (ssh + socat)
    Preconditions: Bridge running on VPS
    Steps:
      1. Call: `echo '{"jsonrpc":"2.0","id":1,"method":"provisioning.start","params":{}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
    Expected Result: Response with session/token in result, no error
    Failure Indicators: -32601 or error in result
    Evidence: .sisyphus/evidence/post-deploy/task-12-start.txt
  ```

  **Commit**: YES (groups with T9)
  - Message: `test(matrix): e2e flow, token recovery, provisioning smoke`
  - Files: `tests/test-provisioning.sh`

- [x] T13. Studio Method Investigation Gate (REQUIRED before T14)

  **What to do**:
  - **INVESTIGATION TASK — do NOT register any methods yet**
  - Investigate the mismatch between:
    - `StudioMethods` list in `bridge/pkg/studio/integration.go:259-286` (22 methods)
    - Handler map in `bridge/pkg/rpc/server.go:1272-1273` (2 registered: `studio.deploy`, `studio.stats`)
    - `studio.deploy` is registered but NOT in `StudioMethods` list
  - Determine for each of the 22 methods whether they are:
    1. **Real and intentionally not exposed** (should be registered — API surface gap)
    2. **Stale/internal only** (should be removed from StudioMethods — dead code)
    3. **Android-side assumptions** (ArmorChat expects them but bridge doesn't implement — spec mismatch)
    4. **Future API not meant for current Bridge RPC** (deliberately excluded)
  - Check `bridge/pkg/rpc/studio.go` — `handleStudio()` dispatches to `studio.HandleRPCMethod()` which uses a method switch. Verify each method in the switch actually does something.
  - Check if `studio.deploy` is a legacy entry point or a separate method
  - Check `bridge/pkg/rpc/delegation_gate.go` — which 5 methods require delegation
  - **Output**: Write findings to `.sisyphus/evidence/post-deploy/task-13-studio-investigation.md` with a classification table
  - T13 (registration) will be based on these findings

  **Must NOT do**:
  - Do NOT register any methods in this task
  - Do NOT modify any code in this task
  - Do NOT add new RPC methods

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with T13 after this completes)
  - **Blocks**: T13
  - **Blocked By**: T10, T11

  **References**:
  **Pattern References**:
  - `bridge/pkg/studio/integration.go:259-286` - `StudioMethods` list (22 methods)
  - `bridge/pkg/rpc/server.go:1272-1273` - Handler map (2 registered)
  - `bridge/pkg/rpc/studio.go` - `handleStudio()` dispatch and `IsStudioMethod()`
  - `bridge/pkg/rpc/delegation_gate.go` - Delegation gate requirements

  **WHY Each Reference Matters**:
  - integration.go has the canonical method list that may be stale
  - server.go has the actual registered methods
  - studio.go shows how handleStudio routes internally
  - delegation_gate.go shows which methods need special handling

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Investigation report produced
    Tool: Bash
    Preconditions: Codebase analyzed
    Steps:
      1. Check file exists: `cat .sisyphus/evidence/post-deploy/task-13-studio-investigation.md`
    Expected Result: File exists with classification table for all 22 methods + studio.deploy analysis
    Failure Indicators: File missing or incomplete
    Evidence: .sisyphus/evidence/post-deploy/task-13-studio-investigation.md

  Scenario: Each method classified with evidence
    Tool: Bash
    Preconditions: Report written
    Steps:
      1. Check each method has a classification: `grep -c 'Classification:' .sisyphus/evidence/post-deploy/task-13-studio-investigation.md`
    Expected Result: At least 23 entries (22 StudioMethods + studio.deploy)
    Failure Indicators: Missing classifications
    Evidence: .sisyphus/evidence/post-deploy/task-13-studio-investigation.md
  ```

  **Commit**: YES
  - Message: `docs(studio): studio method investigation findings`
  - Files: `.sisyphus/evidence/post-deploy/task-13-studio-investigation.md`

- [x] T14. Studio Method Registration (Based on T13 Findings)

  **What to do**:
  - Read `.sisyphus/evidence/post-deploy/task-13-studio-investigation.md`
  - Based on the investigation findings, register in `bridge/pkg/rpc/server.go` ONLY the methods classified as "real and intentionally not exposed"
  - Each entry maps to `handleStudio` (the existing generic handler)
  - For methods requiring delegation gate, use `s.withDelegationGate(s.handleStudio)` pattern
  - Remove any methods classified as "stale" from `StudioMethods` list
  - Resolve `studio.deploy` status (keep, remove, or clarify)
  - Do NOT add catch-all/wildcard dispatch

  **Must NOT do**:
  - Do NOT add catch-all RPC dispatch
  - Do NOT create new handler functions
  - Do NOT register methods classified as "stale" or "future"

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4 sequential after T12
  - **Blocks**: T14, T15, T16
  - **Blocked By**: T12

  **References**:
  **Pattern References**:
  - `.sisyphus/evidence/post-deploy/task-13-studio-investigation.md` - Investigation findings from T12
  - `bridge/pkg/rpc/server.go:1272-1273` - Current registration pattern to follow

  **API/Type References**:
  - `bridge/pkg/rpc/delegation_gate.go` - Methods requiring delegation gate

  **WHY Each Reference Matters**:
  - T13 investigation is the decision authority for what gets registered
  - server.go shows the exact pattern for registration

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Classified methods registered and NOT returning -32601
    Tool: Bash (ssh + socat)
    Preconditions: Bridge rebuilt and deployed with registrations from T13 findings
    Steps:
      1. For each "register" classified method from T12, call: `echo '{"jsonrpc":"2.0","id":1,"method":"<method>","params":{}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
    Expected Result: All registered methods return NOT -32601
    Failure Indicators: Any method returns `{"error":{"code":-32601}}`
    Evidence: .sisyphus/evidence/post-deploy/task-13-studio-registered.txt
  ```

  **Commit**: YES
  - Message: `fix(studio): register validated methods based on investigation`
  - Files: `bridge/pkg/rpc/server.go`, possibly `bridge/pkg/studio/integration.go`

- [x] T15. Studio Agent Lifecycle Validation (Create→List→Spawn→Stop)

  **What to do**:
  - Create `tests/test-studio-lifecycle.sh` that validates the studio agent lifecycle through bridge RPC:
    1. `studio.create_agent` — create a test agent
    2. `studio.list_agents` — verify the agent appears
    3. `studio.get_agent` — fetch agent details
    4. `studio.spawn_agent` — start an agent instance
    5. `studio.list_instances` — verify the instance appears
    6. `studio.stop_instance` — stop the instance
    7. `studio.delete_agent` — clean up
  - Follow Tier A pattern (SSH + curl to VPS)
  - This test validates that the studio methods actually work after registration (T12)

  **Must NOT do**:
  - Do NOT create new studio handlers
  - Do NOT modify studio integration code
  - Do NOT leave test agents/instances running after test

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with T12, T14, T15)
  - **Blocks**: T16
  - **Blocked By**: T12

  **References**:
  **Pattern References**:
  - `tests/test-agent-runtime.sh` - Existing studio test (but calls unregistered methods — this is the replacement)
  - `tests/test-vps-smoke.sh` - Tier A test pattern

  **API/Type References**:
  - `bridge/pkg/studio/integration.go:259-286` - StudioMethods list with all method names and expected params
  - `bridge/pkg/rpc/studio.go` - handleStudio() dispatch logic

  **WHY Each Reference Matters**:
  - test-agent-runtime.sh has patterns for studio testing (though currently broken)
  - StudioMethods list defines the method signatures to use

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Full agent lifecycle completes
    Tool: Bash
    Preconditions: Bridge running with all studio methods registered (T12 done)
    Steps:
      1. Run: `bash tests/test-studio-lifecycle.sh`
      2. Check exit code: `echo $?`
    Expected Result: Exit code 0, all 7 lifecycle steps PASS
    Failure Indicators: Any step returns FAIL
    Evidence: .sisyphus/evidence/post-deploy/task-15-studio-lifecycle.txt

  Scenario: Agent cleanup leaves no residual instances
    Tool: Bash (ssh + socat)
    Preconditions: Lifecycle test completed
    Steps:
      1. Call studio.list_instances: `echo '{"jsonrpc":"2.0","id":1,"method":"studio.list_instances","params":{}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
    Expected Result: Empty instances list (all cleaned up)
    Failure Indicators: Orphaned instances remaining
    Evidence: .sisyphus/evidence/post-deploy/task-15-cleanup.txt
  ```

  **Commit**: YES (groups with T12)
  - Message: `test(studio): validate agent lifecycle against registered studio rpc`
  - Files: `tests/test-studio-lifecycle.sh`

- [x] T16. Trust/PII Flow Validation (Request→Approve→Fulfill)

  **What to do**:
  - Create `tests/test-pii-flow.sh` that validates the PII trust flow through bridge RPC:
    1. `pii.request` — request PII access
    2. `pii.list_pending` — verify request appears
    3. `pii.status` — check request status
    4. `pii.approve` — approve the request
    5. `pii.fulfill` — fulfill the request
    6. `pii.stats` — verify stats reflect the flow
  - Follow Tier A pattern (SSH + curl to VPS)
  - This validates that PII methods work correctly (existing test-trust-layer.sh already tests this but uses a different pattern)

  **Must NOT do**:
  - Do NOT modify PII handler code
  - Do NOT add new PII methods

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with T12, T13, T15)
  - **Blocks**: T16
  - **Blocked By**: T12

  **References**:
  **Pattern References**:
  - `tests/test-trust-layer.sh` - Existing PII test with 8 scenarios. Use as reference for request format and assertion patterns.
  - `tests/lib/load_env.sh` - Environment loader

  **API/Type References**:
  - `bridge/pkg/rpc/pii.go` - PII handler functions
  - `bridge/pkg/rpc/server.go` - Registered PII methods: `pii.request`, `pii.approve`, `pii.deny`, `pii.status`, `pii.list_pending`, `pii.stats`, `pii.cancel`, `pii.fulfill`

  **WHY Each Reference Matters**:
  - test-trust-layer.sh has working PII test patterns to reference
  - server.go confirms all PII methods are registered

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Full PII request-approve-fulfill flow
    Tool: Bash
    Preconditions: Bridge running on VPS, admin token available
    Steps:
      1. Run: `bash tests/test-pii-flow.sh`
      2. Check exit code: `echo $?`
    Expected Result: Exit code 0, all 6 steps PASS
    Failure Indicators: Any step returns FAIL
    Evidence: .sisyphus/evidence/post-deploy/task-16-pii-flow.txt

  Scenario: PII denial path works
    Tool: Bash (ssh + socat)
    Preconditions: Bridge running
    Steps:
      1. Request PII, then deny it: `pii.request` → `pii.deny`
      2. Check status shows denied
    Expected Result: Status shows "denied"
    Failure Indicators: Status not updated or error
    Evidence: .sisyphus/evidence/post-deploy/task-16-pii-deny.txt
  ```

  **Commit**: YES (groups with T12)
  - Message: `test(pii): validate trust layer request-approve-fulfill flow`
  - Files: `tests/test-pii-flow.sh`

- [x] T17. Browser/Jetski Smoke Test (Navigate→Status→Complete)

  **What to do**:
  - Create `tests/test-browser-smoke.sh` that validates browser automation through bridge RPC:
    1. `browser.navigate` — navigate to a test URL (e.g., `https://example.com`)
    2. `browser.status` — check session status
    3. `browser.complete` — mark session complete
  - Follow Tier A pattern (SSH + curl to VPS)
  - Gracefully skip if Jetski/browser service not deployed (follow `test-jetski-sidecar.sh` pattern)
  - **Skip semantics are part of the acceptance bar**: PASS if browser methods work, SKIP if Jetski not deployed (exit 0 with SKIP message), FAIL only if deployed and broken
  - This is the first test for browser RPC methods (currently no bridge RPC browser test exists)

  **Must NOT do**:
  - Do NOT modify browser handler code
  - - Do NOT require Jetski to be deployed (graceful skip)
  - Do NOT test complex browser interactions (just navigate→status→complete)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with T12, T13, T14)
  - **Blocks**: T16
  - **Blocked By**: T12

  **References**:
  **Pattern References**:
  - `tests/test-jetski-sidecar.sh` - Jetski test with graceful skip pattern (lines 1-30)
  - `tests/test-vps-smoke.sh` - Tier A test pattern

  **API/Type References**:
  - `bridge/pkg/rpc/browser.go` - Browser handlers: `handleBrowserNavigate`, `handleBrowserStatus`, `handleBrowserComplete`
  - `bridge/pkg/rpc/server.go` - Registered browser methods (12 total)

  **WHY Each Reference Matters**:
  - test-jetski-sidecar.sh shows the graceful skip pattern for optional services
  - browser.go defines the handler signatures and expected params

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Browser RPC methods respond (not -32601)
    Tool: Bash (ssh + socat)
    Preconditions: Bridge running on VPS
    Steps:
      1. Call browser.status: `echo '{"jsonrpc":"2.0","id":1,"method":"browser.status","params":{}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
    Expected Result: Response that is NOT -32601 (may be "no active session" error, which is valid)
    Failure Indicators: -32601 method not found
    Evidence: .sisyphus/evidence/post-deploy/task-17-browser-status.txt

  Scenario: Test gracefully skips when Jetski not deployed
    Tool: Bash
    Preconditions: Jetski not available on VPS
    Steps:
      1. Run: `bash tests/test-browser-smoke.sh`
      2. Check output for "SKIP" message
    Expected Result: Exit code 0 with SKIP message (not FAIL)
    Failure Indicators: Exit code 1 (hard failure instead of skip)
    Evidence: .sisyphus/evidence/post-deploy/task-17-browser-skip.txt
  ```

  **Commit**: YES (groups with T12)
  - Message: `test(browser): smoke test navigate-status-complete with skip semantics`
  - Files: `tests/test-browser-smoke.sh`

- [x] T18. Remove config.list Test from VPS Smoke Suite

  **What to do**:
  - In `tests/test-vps-smoke.sh`, find and remove the test case that calls `config.list`
  - `config.list` does NOT exist in the RPC handler map — the test expects a method that isn't implemented
  - The VPS test suite currently shows 9/10 PASS with only this test failing
  - After removal, the suite should show 9/9 PASS
  - Do NOT implement `config.list` — just remove the invalid test expectation

  **Must NOT do**:
  - Do NOT implement config.list RPC method
  - Do NOT modify other passing tests
  - Do NOT change the test framework or assertion helpers

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with T17, T18, T19, T20)
  - **Blocks**: T20
  - **Blocked By**: T13, T14, T15

  **References**:
  **Pattern References**:
  - `tests/test-vps-smoke.sh` - The VPS smoke test to modify. Find the `config.list` test case and remove it.

  **API/Type References**:
  - `bridge/pkg/rpc/server.go:1219-1333` - Confirms NO `config.*` methods registered

  **WHY Each Reference Matters**:
  - test-vps-smoke.sh is the file to modify
  - server.go confirms the method doesn't exist

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: VPS test suite shows 9/9 PASS after config.list removal
    Tool: Bash (ssh)
    Preconditions: Changes deployed to VPS
    Steps:
      1. SSH to VPS: `ssh -i ~/.ssh/openclaw_win root@5.183.11.149`
      2. Run suite: `bash /opt/armorclaw/tests/test-vps-smoke.sh`
      3. Parse output for PASS/FAIL count
    Expected Result: "9/9 PASS" (or equivalent)
    Failure Indicators: Any FAIL, or count not 9
    Evidence: .sisyphus/evidence/post-deploy/task-18-vps-9pass.txt

  Scenario: No config.list reference remains in test file
    Tool: Bash
    Preconditions: File edited
    Steps:
      1. `grep -c 'config.list' tests/test-vps-smoke.sh`
    Expected Result: 0 (zero matches)
    Failure Indicators: Any matches found
    Evidence: .sisyphus/evidence/post-deploy/task-18-no-config-list.txt
  ```

  **Commit**: YES (groups with T17-T19)
  - Message: `chore(tests): remove phantom methods, clean broken tests`
  - Files: `tests/test-vps-smoke.sh`

- [x] T19. Remove Phantom Methods from test-matrix-integration.sh

  **What to do**:
  - In `tests/test-matrix-integration.sh`, remove or comment out tests that call phantom methods:
    1. `attach_config` — NOT registered (appears in tests 6, 7)
    2. `list_configs` — NOT registered (appears in test 7)
    3. `matrix.refresh_token` — NOT registered (appears in test 9)
    4. `webrtc.list` — NOT registered (appears in test 12)
  - Keep all working tests (matrix.status, matrix.login, matrix.send, matrix.receive, bridge.status)
  - Add a comment noting which methods were removed and why (phantom/unregistered)
  - Do NOT fix by registering the phantom methods — they don't exist in the handler map

  **Must NOT do**:
  - Do NOT register phantom methods
  - Do NOT modify working test cases
  - Do NOT touch test-element-x-flow.sh (out of scope — 12 phantom calls, separate issue)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with T16, T18, T19, T20)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `tests/test-matrix-integration.sh` - File to modify. Contains 12 tests, 4 of which call phantom methods.

  **API/Type References**:
  - `bridge/pkg/rpc/server.go:1219-1333` - Confirms which methods are registered (matrix.*, bridge.*, but NOT attach_config, list_configs, matrix.refresh_token, webrtc.list)

  **WHY Each Reference Matters**:
  - test-matrix-integration.sh is the file to clean
  - server.go confirms which methods exist vs which are phantom

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: No phantom method calls remain in test file
    Tool: Bash
    Preconditions: File edited
    Steps:
      1. `grep -E 'attach_config|list_configs|matrix.refresh_token|webrtc.list' tests/test-matrix-integration.sh`
    Expected Result: Zero matches (all phantom calls removed)
    Failure Indicators: Any matches found
    Evidence: .sisyphus/evidence/post-deploy/task-19-no-phantoms.txt

  Scenario: Working tests still present
    Tool: Bash
    Preconditions: File edited
    Steps:
      1. `grep -c 'matrix.login\|matrix.send\|matrix.receive\|matrix.status\|bridge.status' tests/test-matrix-integration.sh`
    Expected Result: At least 5 matches (working tests preserved)
    Failure Indicators: Fewer than 5 matches (working tests accidentally removed)
    Evidence: .sisyphus/evidence/post-deploy/task-19-working-tests.txt
  ```

  **Commit**: YES (groups with T16)
  - Message: `chore(tests): remove phantom methods, clean broken tests`
  - Files: `tests/test-matrix-integration.sh`

- [x] T20. Remove test-attach-config.sh (Entirely Broken)

  **What to do**:
  - Delete `tests/test-attach-config.sh` — ALL 6 tests call `attach_config` which is NOT registered
  - The entire test suite is broken — every single test returns -32601
  - Add a note in the commit explaining the file was removed because all tests called unregistered methods
  - Do NOT create a replacement — the methods don't exist

  **Must NOT do**:
  - Do NOT register `attach_config` method
  - Do NOT create a replacement test for nonexistent methods

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with T16, T17, T19, T20)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `tests/test-attach-config.sh` - File to delete. All 6 tests call `attach_config` which is not registered.

  **API/Type References**:
  - `bridge/pkg/rpc/server.go:1219-1333` - Confirms NO `attach_config` method registered

  **WHY Each Reference Matters**:
  - test-attach-config.sh is the file to delete
  - server.go confirms the methods it tests don't exist

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: test-attach-config.sh no longer exists
    Tool: Bash
    Preconditions: File deleted
    Steps:
      1. `ls tests/test-attach-config.sh`
    Expected Result: "No such file or directory"
    Failure Indicators: File still exists
    Evidence: .sisyphus/evidence/post-deploy/task-20-removed.txt
  ```

  **Commit**: YES (groups with T16)
  - Message: `chore(tests): remove phantom methods, clean broken tests`
  - Files: `tests/test-attach-config.sh` (deleted)

- [x] T21. Fix test-secret-passing.sh Phantom Methods

  **What to do**:
  - In `tests/test-secret-passing.sh`, fix the phantom method calls:
    1. `store_key` — IS registered (keep this)
    2. `get_key` — NOT registered (remove or replace tests calling this)
    3. `start` — NOT registered (remove or replace tests calling this)
  - Keep tests that only use `store_key`
  - Remove or comment out tests that call `get_key` or `start`
  - Add a comment noting which methods were removed and why

  **Must NOT do**:
  - Do NOT register `get_key` or `start` methods
  - Do NOT delete the entire file (store_key tests are valid)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with T16, T17, T18, T20)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `tests/test-secret-passing.sh` - File to modify. Tests 2-7 call phantom methods.

  **API/Type References**:
  - `bridge/pkg/rpc/server.go` - `store_key` IS registered. `get_key` and `start` are NOT.

  **WHY Each Reference Matters**:
  - test-secret-passing.sh is the file to clean
  - server.go confirms store_key exists but get_key/start don't

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: No phantom method calls in test-secret-passing.sh
    Tool: Bash
    Preconditions: File edited
    Steps:
      1. `grep -E '"method":"get_key"|"method":"start"' tests/test-secret-passing.sh`
    Expected Result: Zero matches
    Failure Indicators: Any matches found
    Evidence: .sisyphus/evidence/post-deploy/task-21-no-phantoms.txt

  Scenario: store_key tests preserved
    Tool: Bash
    Preconditions: File edited
    Steps:
      1. `grep -c 'store_key' tests/test-secret-passing.sh`
    Expected Result: At least 1 match (valid tests preserved)
    Failure Indicators: Zero matches (all tests accidentally removed)
    Evidence: .sisyphus/evidence/post-deploy/task-21-store-key-preserved.txt
  ```

  **Commit**: YES (groups with T16)
  - Message: `chore(tests): remove phantom methods, clean broken tests`
  - Files: `tests/test-secret-passing.sh`

- [x] T22. Remove fmt.Printf from Production Go Code (Not Structured Logging)

  **What to do**:
  - Replace all `fmt.Printf`, `fmt.Println` with `log.Printf`, `log.Println` in production Go code
  - **This is NOT structured logging** — it is removing raw `fmt.Printf` calls and using stdlib `log` instead
  - The deliverable is "production logging cleanup," not "structured logging"
  - Scope: **production Go files in `bridge/pkg/` and `bridge/internal/` only**
  - Exclude: `*_test.go` files, `cmd/` directory, `deploy/` scripts
  - Use `ast_grep_search` to find all instances: pattern `fmt.Printf($$$)` and `fmt.Println($$$)`
  - Use `ast_grep_replace` to replace with `log.Printf($$$)` and `log.Println($$$)`
  - Special handling for `fmt.Sprintf` used inside log statements — these should be kept as-is
  - Verify no password values are logged (audit all changed lines for `password`, `secret`, `token` in format strings)
  - If `log` package not already imported, add the import

  **Must NOT do**:
  - Do NOT touch `*_test.go` files
  - Do NOT touch `cmd/` directory
  - Do NOT touch `deploy/` scripts
  - Do NOT log password/secret/token values
  - Do NOT change error handling (fmt.Errorf → keep as-is)
  - Do NOT add structured logging library (use stdlib `log` only)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with T16, T17, T18, T19)
  - **Blocks**: F1-F4
  - **Blocked By**: T16

  **References**:
  **Pattern References**:
  - `bridge/internal/adapter/matrix.go` - Contains ~15 `fmt.Printf` calls that need replacing

  **API/Type References**:
  - Go stdlib `log` package — `log.Printf`, `log.Println` for structured output to stderr

  **External References**:
  - ast_grep_search pattern: `fmt.Printf($$$)` to find all instances
  - ast_grep_replace pattern: `fmt.Printf($$$)` → `log.Printf($$$)` for replacement

  **WHY Each Reference Matters**:
  - matrix.go has the most fmt.Printf calls and was explicitly called out in previous plan
  - ast_grep tools provide efficient find-and-replace for this pattern

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Zero fmt.Printf/Println in production code
    Tool: Bash (ast_grep_search)
    Preconditions: Replacements done
    Steps:
      1. Search: `ast_grep_search --pattern 'fmt.Printf($$$)' --include '*.go' --exclude '*_test.go' bridge/pkg/ bridge/internal/`
      2. Search: `ast_grep_search --pattern 'fmt.Println($$$)' --include '*.go' --exclude '*_test.go' bridge/pkg/ bridge/internal/`
    Expected Result: Zero matches for both patterns
    Failure Indicators: Any matches remaining
    Evidence: .sisyphus/evidence/post-deploy/task-22-no-fmt-printf.txt

  Scenario: No password/secret values logged
    Tool: Bash (grep)
    Preconditions: Replacements done
    Steps:
      1. `grep -rn 'log.Printf.*password\|log.Printf.*secret\|log.Printf.*token' bridge/pkg/ bridge/internal/ --include='*.go' | grep -v '_test.go'`
    Expected Result: Zero matches (no secrets logged)
    Failure Indicators: Any match containing password/secret/token
    Evidence: .sisyphus/evidence/post-deploy/task-22-no-secrets.txt

  Scenario: Bridge compiles after logging changes
    Tool: Bash
    Preconditions: Changes made
    Steps:
      1. `cd bridge && go build ./...`
    Expected Result: Exit code 0 (compiles cleanly)
    Failure Indicators: Compilation errors (missing imports, type mismatches)
    Evidence: .sisyphus/evidence/post-deploy/task-22-compiles.txt
  ```

  **Commit**: YES (separate)
  - Message: `refactor(log): replace fmt.Printf with log.Printf in production code`
  - Files: `bridge/pkg/**/*.go`, `bridge/internal/**/*.go`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. 
> **Technical validation is fully automated** — agents run checks, produce evidence, and render verdicts.
> **User approval is a GOVERNANCE CHECKPOINT** (not part of the test contract): results are presented, user says "okay" to proceed. This is separate from the automated PASS/FAIL verification.

- [x] F1. **Startup Audit** — `oracle`
  Read the plan's "Must Have" for startup. Verify: (1) `docker run armorclaw:latest` starts bridge and health.check responds with `{"status":"ok"}`, (2) CI=true mode completes Matrix setup before blocking, (3) config.toml preserved across container restart, (4) .bootstrapped flag prevents re-setup. Run each check via SSH to VPS at 5.183.11.149. Check evidence files exist in .sisyphus/evidence/post-deploy/.
  Output: `Startup [N/N] | CI Mode [PASS/FAIL] | Config Preservation [PASS/FAIL] | Restart [PASS/FAIL] | VERDICT: APPROVE/REJECT`

- [x] F2. **YARA + Feature Audit** — `unspecified-high`
  Verify: (1) YARA rules compile: `docker exec <container> yara -p 1 /opt/armorclaw/configs/yara_rules.yar /dev/null` exits 0, (2) All studio methods classified as valid by T13 return NOT -32601 (call each via socat), (3) Matrix e2e flow: login→send→receive works, (4) PII flow: request→approve→fulfill works. Check evidence files.
  Output: `YARA [PASS/FAIL] | Studio [N/22] | Matrix [PASS/FAIL] | PII [PASS/FAIL] | VERDICT: APPROVE/REJECT`

- [x] F3. **Test Suite Integrity** — `unspecified-high`
  SSH to VPS at 5.183.11.149. Run full VPS test suite. Expected: 9/9 PASS (config.list removed). Verify: zero phantom method calls remain in any test script. Run `grep -r 'attach_config\|list_configs\|matrix.refresh_token\|webrtc.list\|get_key' tests/` — expect zero matches. Verify test-attach-config.sh is deleted. Verify test-secret-passing.sh only calls registered methods.
  Output: `VPS Suite [9/9] | Phantom Methods [0] | Removed Scripts [verified] | VERDICT: APPROVE/REJECT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built, nothing beyond spec was built. Check "Must NOT Have" compliance: container-setup.sh untouched, scanner.go untouched, no new RPC methods added, no wildcard dispatch, no config versioning, no *_test.go changes from logging task. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Guardrails [CLEAN/N violations] | Unaccounted [CLEAN/N files] | VERDICT: APPROVE/REJECT`

---

## Commit Strategy

- **Wave 1**: `fix(startup): preserve config, separate CI/production, bridge as PID 1, restart resilience` - deploy/setup-quick.sh, Dockerfile.quickstart
- **T5**: `chore: remove dead quickstart-entrypoint.sh from Docker image` - Dockerfile.quickstart
- **T6**: `fix(yara): reference $pdf_exploit in exploit_kit_landing condition` - bridge/configs/yara_rules.yar
- **T7-T8**: `test(yara): add smoke test and verify path resolution` - tests/test-yara-smoke.sh
- **T9-T11**: `test(matrix): e2e flow, token recovery, provisioning smoke` - tests/test-matrix-e2e-rpc.sh, tests/test-token-recovery.sh, tests/test-provisioning.sh
- **T13-T17**: `docs(studio): classify bridge-exposed vs stale studio methods`, `fix(studio): register validated bridge studio methods`, `test(studio): validate agent lifecycle`, `test(pii): validate trust layer flow`, `test(browser): smoke test with skip semantics`
- **T16-T19**: `chore(tests): remove phantom methods, clean broken tests` - tests/test-vps-smoke.sh, tests/test-matrix-integration.sh, tests/test-secret-passing.sh
- **T20**: `refactor(log): replace fmt.Printf with log.Printf in production code` - bridge/pkg/**/*.go, bridge/internal/**/*.go

---

## Success Criteria

### Verification Commands
```bash
# Bridge health in Docker
echo '{"jsonrpc":"2.0","id":1,"method":"health.check"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
# Expected: {"jsonrpc":"2.0","id":1,"result":{"status":"ok"}}

# YARA compilation
docker exec <container> yara -p 1 /opt/armorclaw/configs/yara_rules.yar /dev/null
# Expected: exit code 0

# Studio methods registered
echo '{"jsonrpc":"2.0","id":1,"method":"studio.list_agents","params":{}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
# Expected: NOT -32601

# VPS test suite
ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'cd /opt/armorclaw && bash tests/test-vps-smoke.sh'
# Expected: 9/9 PASS

# No phantom methods
grep -r 'attach_config\|list_configs\|matrix.refresh_token\|webrtc.list' tests/
# Expected: zero matches (excluding test-element-x-flow.sh which is out of scope)

# No fmt.Printf in production
ast_grep_search --pattern 'fmt.Printf($$$)' --include '*.go' --exclude '*_test.go' bridge/pkg/ bridge/internal/
# Expected: zero matches
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] Bridge responds to health.check in Docker container
- [ ] YARA rules compile and scan
- [ ] All validated studio methods registered; stale methods documented
- [ ] Matrix e2e flow works
- [ ] VPS test suite 9/9 PASS
- [ ] Zero phantom methods in cleaned test scripts
- [ ] Zero fmt.Printf in production Go code
