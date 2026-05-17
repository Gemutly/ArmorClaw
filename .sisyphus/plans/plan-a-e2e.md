# Plan A: ArmorClaw E2E — Contract Discovery, Deployment, Event Validation, Harness Execution

## TL;DR

> **Quick Summary**: Deploy ArmorClaw to VPS (or detect existing), discover its live RPC/event/endpoint contract, provision admin access, validate control-plane event emission, and run existing harness suites. All discovery is runtime-probed — no hardcoded assumptions.
> 
> **Deliverables**:
> - Contract manifest (`contract_manifest.json`) with discovered RPC methods, event types, endpoints
> - VPS deployment healthy and provisioned
> - Event validation evidence (workflow/agent events verified via Matrix /sync)
> - Tier A harness results on VPS
> - Provisioning outputs for ArmorChat plan
> 
> **Estimated Effort**: Large (12 tasks, multi-phase)
> **Parallel Execution**: YES - 5 waves (Wave 0–4)
> **Critical Path**: T1→T2→T4→T10→T12, with T7→T8 completing in parallel before T12

---

## Context

### Original Request
User provided a complete ArmorClaw Agent Test Plan specification with Phase A0 (contract discovery), A1 (deployment), A2 (provisioning), A3 (event validation), and A4 (harness execution). User chose to extend existing infrastructure and execute Plan A first.

### Interview Summary
**Key Discussions**:
- Voice decision: Defer until after Plan A and Streams 1-5 complete
- Infrastructure approach: Extend existing `scripts/` and `tests/lib/` rather than create standalone
- Priority: Plan A first, then strategic streams
- VPS already configured: 5.183.11.149, bridge:8080, matrix:6167

**Research Findings**:
- Existing `tests/lib/load_env.sh` sources .env, provides ssh_vps(), rpc_call
- Existing `tests/lib/assert_json.sh` provides JSON assertion helpers
- Existing `tests/lib/restart_bridge.sh` provides bridge restart with flock serialization
- Existing `scripts/deploy-infrastructure.sh` handles deployment
- Existing `scripts/provision-matrix.sh` handles Matrix setup
- No `armorclaw/.env` — config is at repo root `.env`
- Voice: pkg/voice/ commented out, STT/TTS/VAD interface-only (deferred)
- Secretary workflow: Full engine with bridge_local_registry, step execution
- ArmorChat: EmailApprovalScreen with expiry, ApprovalScreen, DeepLinkHandler

### Gap Analysis (Metis-equivalent)
**Identified Gaps** (addressed in-plan):
- `armorclaw/.env` doesn't exist — scripts must use root `.env` like existing `load_env.sh`
- New `_vps_ssh()` duplicates existing `ssh_vps()` from `tests/lib/load_env.sh` — must reuse
- New `_vps_bridge_rpc()` duplicates existing `rpc_call` — must reuse or extend
- Deploy scripts already exist — A1 should detect and skip if already deployed
- Test harness scripts expect `load_env.sh` pattern — new scripts should follow same convention
- Voice deferred — removed from Tier B scope for this plan

---

## Work Objectives

### Core Objective
Get ArmorClaw deployed on VPS with a verified contract, provisioned admin access, validated event emission, and passing Tier A harness tests.

### Concrete Deliverables
- `scripts/lib/contract.sh` — shared library extending tests/lib/ with contract discovery helpers
- `scripts/a0_discover.sh` — runtime RPC/event/endpoint discovery
- `scripts/a1_deploy.sh` — topology-aware deployment (or skip if already running)
- `scripts/a2_provision.sh` — admin bootstrap with discovered RPC methods
- `scripts/a3_events.sh` — control-plane event validation via Matrix /sync
- `scripts/a4_harness.sh` — existing harness execution on VPS
- `scripts/a_run_all.sh` — master runner
- `.skills/e2e-deploy.yaml` — agent skill definition
- `.sisyphus/evidence/armorclaw/contract_manifest.json` — THE key output
- `.sisyphus/evidence/armorclaw/a2_provisioning_outputs.json` — for ArmorChat plan

### Definition of Done
- [ ] `contract_manifest.json` exists with discovered RPC methods, event types, and endpoints
- [ ] VPS deployment healthy (`/health` returns 200)
- [ ] `/.well-known/matrix/client` returns valid `m.homeserver.base_url`
- [ ] At least one admin RPC responds successfully
- [ ] At least one workflow or agent event path verified via Matrix /sync
- [ ] Tier A harness passes for health suite
- [ ] `a2_provisioning_outputs.json` contains connection details for ArmorChat plan

### Must Have
- Contract discovery that probes the LIVE Bridge (not hardcoded method lists)
- Topology-aware deployment (detect image contents before composing)
- Event validation via Matrix /sync (not just log scraping)
- Evidence files for every phase
- Reuse of existing tests/lib/ infrastructure

### Must NOT Have (Guardrails)
- Do NOT duplicate `ssh_vps()` — reuse from `tests/lib/load_env.sh`
- Do NOT duplicate `rpc_call` — extend or reuse existing pattern
- Do NOT replace existing deploy scripts — A1 should call them when appropriate
- Do NOT assume RPC method names — discover at runtime
- Do NOT assume Docker image contents — inspect before composing
- Do NOT assume `armorclaw/.env` exists — use root `.env` pattern
- Do NOT include voice tests in Tier B for this plan (deferred)
- Do NOT hardcode event type prefixes — verify against live /sync
- Do NOT touch existing test scripts in tests/
- Do NOT commit raw `.sisyphus/evidence/armorclaw/` session artifacts containing tokens or Matrix access credentials
- Do NOT commit `a2_matrix_session.json` unredacted
- Do NOT hardcode VPS IP addresses — use `$VPS_IP` and `$BRIDGE_PORT` from `.env`

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: YES (bash test harness in tests/)
- **Automated tests**: Harness execution (Tier A/B)
- **Framework**: bash + curl + jq

### QA Policy
Every task includes agent-executed QA via bash commands.
Evidence saved to `.sisyphus/evidence/armorclaw/`.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 0 (Foundation):
├── Task 1: Create scripts/lib/contract.sh — contract discovery helper library [quick]
└── Task 6: Create .skills/e2e-deploy.yaml — agent skill definition [quick]

Wave 1 (Discovery + Deployment):
├── Task 2: Create scripts/a0_discover.sh — Phase A0 contract discovery [deep]
└── Task 3: Create scripts/a1_deploy.sh — Phase A1 topology-aware deployment [deep]

Wave 2 (Provisioning + Events):
├── Task 4: Create scripts/a2_provision.sh — Phase A2 admin bootstrap [deep]
└── Task 5: Create scripts/a3_events.sh — Phase A3 event validation [deep]

Wave 3 (Harness + Runner):
├── Task 7: Create scripts/a4_prepare.sh — copy harness to VPS [quick]
├── Task 8: Create scripts/a4_harness.sh — run test suites on VPS [deep]
└── Task 9: Create scripts/a_run_all.sh — master runner [quick]

Wave 4 (Execution + Verification):
├── Task 10: Run Phase A0-A2 on VPS — discover contract + deploy + provision [deep]
├── Task 11: Run Phase A3 — validate events on live deployment [deep]
└── Task 12: Run Phase A4 — execute Tier A harness on VPS [deep]

Wave FINAL (After ALL tasks):
├── F1: Verify contract_manifest.json exists with live_discovered.rpc_methods
├── F2: Verify deployment is healthy (/health = 200)
├── F3: Verify provisioning outputs exist + at least one admin RPC responds
├── F4: Verify at least one event type confirmed via /sync
└── F5: Verify Tier A health suite passed
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| T1 | - | T2, T3, T4, T5 |
| T2 | T1 | T4, T5, T10 |
| T3 | T1 | T10 |
| T4 | T2 | T10 |
| T5 | T2 | T11 |
| T6 | - | - |
| T7 | - | T8 |
| T8 | T7 | T12 |
| T9 | - | - |
| T10 | T2, T3, T4 | T11, T12 |
| T11 | T5, T10 | - |
| T12 | T8, T10 | - |

> **Runtime dependency note**: T5 script creation depends only on T2, but Phase A3 execution (T11) may consume `a2_matrix_session.json` produced by Phase A2 during T10 — if it exists. If Matrix registration was blocked (session SKIPPED), T11 runs A3 in degraded mode (A3.1-A3.3 SKIP, A3.4-A3.5 best-effort). This is a runtime artifact dependency, not a script-creation dependency.

### Agent Dispatch Summary

- **Wave 0**: T1 → `quick`, T6 → `quick`
- **Wave 1**: T2 → `deep`, T3 → `deep`
- **Wave 2**: T4 → `deep`, T5 → `deep`
- **Wave 3**: T7 → `quick`, T8 → `deep`, T9 → `quick`
- **Wave 4**: T10 → `deep`, T11 → `deep`, T12 → `deep`

---

## TODOs

- [x] 1. Create `scripts/lib/contract.sh` — contract discovery helper library

  **What to do**:
  - Create `scripts/lib/contract.sh` as a bash library sourced by all Phase A scripts
  - MUST source `tests/lib/load_env.sh` first (provides ssh_vps, rpc_call, log_result)
  - Provide these functions (extending, not duplicating, existing infrastructure):
    - `_contract_bridge_rpc()` — call Bridge JSON-RPC via ssh_vps (wraps existing rpc_call pattern)
    - `_contract_wait_http()` — wait for HTTP endpoint on VPS (port + path + timeout)
    - `_contract_save()` — save JSON evidence to .sisyphus/evidence/armorclaw/
    - `_contract_load_manifest()` — load contract_manifest.json
    - `_contract_update_manifest()` — update contract_manifest.json with new section
  - `_contract_bridge_rpc()` MUST support bounded retry with backoff for transient SSH/curl failures
  - `_contract_wait_http()` MUST print elapsed time and last observed status code
  - Helper defaults should prefer resilience over immediate failure for network probes
  - Follow bash conventions from existing tests/lib/ (set -euo pipefail, proper error handling)
  - Create `.sisyphus/evidence/armorclaw/` directory structure

  **Manifest schema (minimum):**
  ```json
  {
    "live_discovered": {
      "rpc_methods": [
        {
          "name": "string",
          "status": "responds|error|timeout|unknown",
          "empty_params_result": "string",
          "notes": "string"
        }
      ],
      "event_types": [
        {
          "type": "string",
          "source": "sync|logs",
          "verified": true
        }
      ],
      "endpoints": [
        {
          "path": "string",
          "status_code": 200,
          "response_keys": ["string"]
        }
      ]
    },
    "documented_reference": {
      "env_vars": ["string"],
      "deep_links": ["string"]
    },
    "runtime_flags": {
      "deployment_required": false
    },
    "provisioning": {}
  }
  ```

  **Must NOT do**:
  - Do NOT duplicate ssh_vps() — source tests/lib/load_env.sh
  - Do NOT duplicate rpc_call — extend or wrap
  - Do NOT hardcode VPS_IP/BRIDGE_PORT — get from env vars

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T6)
  - **Parallel Group**: Wave 0
  - **Blocks**: T2, T3, T4, T5
  - **Blocked By**: None

  **References**:
  - `tests/lib/load_env.sh` — existing env loading pattern (MUST follow)
  - `tests/lib/assert_json.sh` — JSON assertion pattern
  - `tests/lib/common_output.sh` — PASS/FAIL/SKIP counter pattern
  - `.env` (root) — VPS_IP, VPS_USER, SSH_KEY_PATH, BRIDGE_PORT, MATRIX_PORT

  **Acceptance Criteria**:
  - [ ] `scripts/lib/contract.sh` exists and is syntactically valid (`bash -n scripts/lib/contract.sh`)
  - [ ] Sources tests/lib/load_env.sh successfully
  - [ ] Provides _contract_bridge_rpc(), _contract_wait_http(), _contract_save()

  **QA Scenarios:**
  ```
  Scenario: Library loads without error
    Tool: Bash
    Steps:
      1. bash -n scripts/lib/contract.sh → exit 0
      2. source scripts/lib/contract.sh; type _contract_bridge_rpc → "function"
    Expected Result: No syntax errors, functions defined
    Evidence: .sisyphus/evidence/task-1-lib-check.txt
  ```

  **Commit**: YES
  - Message: `feat(tests): add contract discovery helper library for E2E plan`
  - Files: `scripts/lib/contract.sh`

- [x] 2. Create `scripts/a0_discover.sh` — Phase A0 contract discovery

  **What to do**:
  - Create `scripts/a0_discover.sh` implementing the user's Phase A0 specification
  - Source `scripts/lib/contract.sh` (which sources tests/lib/load_env.sh)
  - Implement these discovery steps:
    - A0.1: Verify VPS SSH connectivity
    - A0.2: Check if Bridge already running (systemd/Docker/port probe).
      - If running: record deployment state and continue discovery.
      - If not running: record `deployment_required=true` in `contract_manifest.json`, save evidence, and exit discovery cleanly.
      - Do NOT invoke `a1_deploy.sh` from inside A0. Deployment is performed by A1 or by the master runner.
    - A0.3: Discover HTTP endpoints (/health, /api, /.well-known/matrix/client, /qr/config, /metrics, /version)
    - A0.4: Discover RPC methods — try rpc.discover/system.listMethods first, then probe known names
    - A0.5: Discover RPC parameter schemas — call each method with empty params, capture error messages
    - A0.6: Discover Matrix event types — register test user, create room, sync, extract event types
    - A0.7: Document env var names from armorclaw.md
    - A0.8: Document deep link formats
    - A0.9: Generate contract_manifest.json combining all discoveries
  - RPC method probe list from user's spec (bridge.*, provisioning.*, workflow.*, agent.*, hitl.*, etc.)
  - Use `_contract_save()` for all evidence files
  - Generate machine-readable `contract_manifest.json`

  **Must NOT do**:
  - Do NOT assume RPC method names work — probe and record errors
  - Do NOT hardcode deployment — detect and skip if already running
  - Do NOT fail on individual discovery failures — record and continue

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T1)
  - **Parallel Group**: Wave 1 (after T1)
  - **Blocks**: T4, T10
  - **Blocked By**: T1

  **References**:
  - User's complete A0 specification (provided in conversation)
  - `scripts/lib/contract.sh` (T1 output)
  - `tests/lib/load_env.sh` — env loading pattern
  - `bridge/pkg/rpc/server.go` — registerHandlers() for known method list
  - `doc/armorclaw.md` — documented RPC methods, env vars, deep links

  **Acceptance Criteria**:
  - [ ] `bash -n scripts/a0_discover.sh` passes
  - [ ] Script sources scripts/lib/contract.sh
  - [ ] Generates .sisyphus/evidence/armorclaw/contract_manifest.json

  **QA Scenarios:**
  ```
  Scenario: Script is valid bash
    Tool: Bash
    Steps:
      1. bash -n scripts/a0_discover.sh → exit 0
    Expected Result: No syntax errors
    Evidence: .sisyphus/evidence/task-2-syntax-check.txt

  Scenario: Dry-run structure check
    Tool: Bash
    Steps:
      1. grep -c "A0\." scripts/a0_discover.sh → >= 8 (8 substeps)
      2. grep "contract_manifest" scripts/a0_discover.sh → present
    Expected Result: All substeps present
    Evidence: .sisyphus/evidence/task-2-structure-check.txt
  ```

  **Commit**: YES
  - Message: `feat(tests): add Phase A0 contract discovery script`
  - Files: `scripts/a0_discover.sh`

- [x] 3. Create `scripts/a1_deploy.sh` — Phase A1 topology-aware deployment

  **What to do**:
  - Create `scripts/a1_deploy.sh` implementing the user's Phase A1 specification
  - Source `scripts/lib/contract.sh`
  - Implement deployment steps:
    - A1.1: Verify VPS SSH
    - A1.2: Ensure Docker on VPS
    - A1.3: Pull ArmorClaw image, inspect contents (entrypoint, exposed ports, env vars)
    - A1.4: Generate secrets (TURN_SECRET, KEYSTORE_SECRET)
    - A1.5: Resolve API key env var name from provider
    - A1.6: Determine topology (single image vs multi-service based on image inspection)
    - A1.7: Create docker-compose.yml based on topology
    - A1.8: Start containers
    - A1.9: Wait for Bridge /health (3 min timeout)
    - A1.10: Wait for Bridge /api
    - A1.11: Wait for Matrix homeserver
    - A1.12: Verify /.well-known/matrix/client
    - A1.13: Collect deployment evidence
  - Key: Use API_KEY env var names from armorclaw.md (OPEN_AI_KEY, OPENROUTER_API_KEY, ZAI_API_KEY)
  - Check if existing deployment scripts in scripts/ should be called instead of reimplemented
  - A1 MUST capture VPS resource evidence before deployment:
    - available memory
    - available disk
    - CPU count
  - If resource checks indicate likely deployment failure, save evidence and fail with a clear diagnostic before container startup

  **Must NOT do**:
  - Do NOT assume image contents — inspect at runtime
  - Do NOT hardcode compose topology — determine from image
  - Do NOT ignore existing deploy scripts — check scripts/deploy-infrastructure.sh first

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Topology-aware deployment logic requires careful conditional branching
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T1)
  - **Parallel Group**: Wave 1 (after T1)
  - **Blocks**: T10
  - **Blocked By**: T1

  **References**:
  - User's complete A1 specification
  - `scripts/lib/contract.sh` (T1)
  - `scripts/deploy-infrastructure.sh` — existing deploy logic
  - `deploy/` directory — Dockerfile, compose files
  - `.env` — VPS_IP, BRIDGE_PORT, MATRIX_PORT, API keys

  **Acceptance Criteria**:
  - [ ] `bash -n scripts/a1_deploy.sh` passes
  - [ ] Inspects Docker image before creating compose
  - [ ] Waits for both /health and Matrix before proceeding
  - [ ] Saves evidence to .sisyphus/evidence/armorclaw/

  **Commit**: YES
  - Message: `feat(tests): add Phase A1 topology-aware deployment script`
  - Files: `scripts/a1_deploy.sh`

- [x] 4. Create `scripts/a2_provision.sh` — Phase A2 admin bootstrap

  **What to do**:
  - Create `scripts/a2_provision.sh` implementing Phase A2
  - Source `scripts/lib/contract.sh`
  - Implement provisioning steps:
    - A2.1: Determine provisioning RPC method from contract_manifest.json
    - A2.2: Claim provisioning with discovered params (try empty first, then documented fields)
    - A2.3: Retrieve effective config via discovered method
    - A2.4: Verify bridge health/status
    - A2.5: Retrieve /qr/config for mobile provisioning
    - A2.6: Verify /.well-known/matrix/client discovery
    - A2.7: Create test Matrix user
    - A2.8: Create test room
    - A2.9: Write provisioning outputs (a2_provisioning_outputs.json always; a2_matrix_session.json if registration succeeded; if blocked, write `matrix_session: "SKIPPED"` with reason into provisioning outputs)
  - Key: ALL RPC calls use methods from contract_manifest.json (discovered in A0)
  - Save Matrix session for A3 event validation
  - Update contract_manifest.json with provisioning section

  **Must NOT do**:
  - Do NOT assume provisioning method name — read from manifest
  - Do NOT assume Matrix registration is open — handle both open and token-based
  - Do NOT fail if /qr/config unavailable — warn and continue

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T5, T6)
  - **Parallel Group**: Wave 2
  - **Blocks**: T10
  - **Blocked By**: T2

  **References**:
  - User's complete A2 specification
  - `scripts/lib/contract.sh` (T1)
  - `.sisyphus/evidence/armorclaw/contract_manifest.json` (T2 output)
  - `doc/armorclaw.md` — provisioning docs
  - `scripts/provision-matrix.sh` — existing provisioning logic

  **Acceptance Criteria**:
  - [ ] `bash -n scripts/a2_provision.sh` passes
  - [ ] Reads RPC methods from contract_manifest.json
  - [ ] Generates a2_provisioning_outputs.json (always)
  - [ ] Either generates a2_matrix_session.json (registration succeeded) OR writes `matrix_session: "SKIPPED"` into a2_provisioning_outputs.json with documented reason

  **Commit**: YES
  - Message: `feat(tests): add Phase A2 provisioning and admin bootstrap`
  - Files: `scripts/a2_provision.sh`

- [x] 5. Create `scripts/a3_events.sh` — Phase A3 event validation

  **What to do**:
  - Create `scripts/a3_events.sh` implementing Phase A3
  - Source `scripts/lib/contract.sh`
  - Implement event validation steps:
    - A3.0: Check for Matrix session — if a2_matrix_session.json is missing or a2_provisioning_outputs.json says `matrix_session: "SKIPPED"`, skip A3.1-A3.3 entirely, log all as SKIP with documented reason, and proceed directly to A3.4 (log scan) and A3.5 (event type scan via anonymous /sync if available)
    - A3.1: Send m.room.message and verify in /sync (requires session)
    - A3.2: Start workflow via discovered RPC, observe workflow events (requires session)
    - A3.3: Check for agent status events (requires session)
    - A3.4: Scan bridge logs for event publication evidence (does not require session)
    - A3.5: Full event type scan — multiple syncs, collect all types, cross-reference with expected (best-effort; anonymous if no session)
  - Use PASS/FAIL/SKIP counters from tests/lib/common_output.sh
  - Load expected events from contract_manifest.json
  - Save per-test evidence files
  - MUST save `a3.5_discovered_event_types.txt` with all discovered event types (one per line) for T11/F3 to check

  **Must NOT do**:
  - Do NOT assume event type names — verify against live /sync
  - Do NOT fail if no workflow events (may need manual trigger) — SKIP instead
  - Do NOT block on missing events — log and continue
  - If no auto-triggerable workflow or agent event path is discovered from the manifest, mark the corresponding check as SKIP with a documented reason
  - Do NOT rely on manual triggering for event validation

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T4, T6)
  - **Parallel Group**: Wave 2
  - **Blocks**: T11
  - **Blocked By**: T2

  **References**:
  - User's complete A3 specification
  - `scripts/lib/contract.sh` (T1)
  - `tests/lib/common_output.sh` — PASS/FAIL/SKIP counter pattern
  - `doc/communication-infra.md` — documented event types

  **Acceptance Criteria**:
  - [ ] `bash -n scripts/a3_events.sh` passes
  - [ ] Handles missing Matrix session gracefully (SKIP path for A3.1-A3.3)
  - [ ] Generates a3_summary.json with pass/fail/skip counts
  - [ ] a3.5_discovered_event_types.txt saved regardless of session availability

  **Commit**: YES
  - Message: `feat(tests): add Phase A3 control-plane event validation`
  - Files: `scripts/a3_events.sh`

- [x] 6. Create `.skills/e2e-deploy.yaml` — agent skill definition

  **What to do**:
  - Create `.skills/e2e-deploy.yaml` per user's specification
  - Include: name, description, command, automation_level, parameters, agent_instructions
  - Reference the execution order (A0→A1→A2→A3→A4)
  - Document critical boundaries (no hardcoded assumptions, discover at runtime)
  - List tools_required

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T1)
  - **Parallel Group**: Wave 0
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - User's complete .skills/e2e-deploy.yaml specification
  - Existing `.skills/*.yaml` files for pattern reference

  **Acceptance Criteria**:
  - [ ] `.skills/e2e-deploy.yaml` exists and is valid YAML
  - [ ] Contains agent_instructions with A0-A4 execution order

  **Commit**: YES (groups with Wave 0)
  - Message: `feat(skills): add e2e-deploy skill definition`
  - Files: `.skills/e2e-deploy.yaml`

- [x] 7. Create `scripts/a4_prepare.sh` — copy harness to VPS

  **What to do**:
  - Create `scripts/a4_prepare.sh` per user's spec
  - Source `scripts/lib/contract.sh`
  - Check if tests/ exists locally
  - Copy test files to VPS at /opt/armorclaw/tests/
  - Copy tests/lib/ (load_env.sh, assert_json.sh, etc.)
  - Make scripts executable on VPS
  - Handle case where tests/ not available (clone repo)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T8, T9)
  - **Parallel Group**: Wave 3
  - **Blocks**: T8
  - **Blocked By**: None

  **References**:
  - User's A4-prep specification
  - `tests/` directory — existing harness scripts
  - `tests/lib/` — shared libraries

  **Acceptance Criteria**:
  - [ ] `bash -n scripts/a4_prepare.sh` passes
  - [ ] Copies tests/ to VPS via scp

  **Commit**: YES (groups with T8, T9)

- [x] 8. Create `scripts/a4_harness.sh` — run test suites on VPS

  **What to do**:
  - Create `scripts/a4_harness.sh` per user's spec
  - Source `scripts/lib/contract.sh`
  - Map suite names to test files (health→test-system-health-baseline.sh, etc.)
  - Support suite parameter (comma-separated)
  - Run each test on VPS via ssh_vps
  - Parse output for pass/fail indicators
  - Generate a4_summary.json

  **Must NOT do**:
  - Do NOT modify existing test scripts
  - Do NOT assume test output format — handle multiple patterns

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Harness execution logic requires mapping suites to files and parsing multi-format output
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T7)
  - **Parallel Group**: Wave 3 (after T7)
  - **Blocks**: T12
  - **Blocked By**: T7

  **References**:
  - User's A4 specification
  - `tests/test-*.sh` — existing harness scripts
  - `tests/lib/load_env.sh` — env vars the tests expect

  **Acceptance Criteria**:
  - [ ] `bash -n scripts/a4_harness.sh` passes
  - [ ] Maps at least 10 suite names to test files
  - [ ] Generates a4_summary.json

  **Commit**: YES (groups with T7, T9)

- [x] 9. Create `scripts/a_run_all.sh` — master runner

  **What to do**:
  - Create `scripts/a_run_all.sh` per user's spec
  - Support positional phase argument: `a_run_all.sh all`, `a_run_all.sh A0`, `a_run_all.sh A1`, `a_run_all.sh A2`, `a_run_all.sh A3`, `a_run_all.sh A4`
  - Run phases in order (A0→A1→A2→A3→A4)
  - Stop on A0/A1 failure (fatal)
  - Generate final_summary.json
  - Print provisioning outputs for ArmorChat plan

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T7, T8)
  - **Parallel Group**: Wave 3
  - **Blocks**: T10, T11, T12
  - **Blocked By**: None

  **References**:
  - User's a_run_all.sh specification
  - All Phase A scripts (T2-T5, T7-T8)

  **Acceptance Criteria**:
  - [ ] `bash -n scripts/a_run_all.sh` passes
  - [ ] Supports positional phase argument (`a_run_all.sh A0`, `a_run_all.sh all`)
  - [ ] Generates final_summary.json

  **Commit**: YES (groups with T7, T8)

- [x] 10. Run Phase A0-A2 on VPS — discover contract + deploy + provision

  **What to do**:
  - Execute `bash scripts/a0_discover.sh` — initial discovery pass
  - Check `deployment_required` flag in the generated manifest:
    - If `deployment_required == true`: execute `bash scripts/a1_deploy.sh`, then **re-run** `bash scripts/a0_discover.sh` to populate the manifest with live RPC/event data from the newly deployed Bridge
    - If `deployment_required == false` (Bridge already running): skip deployment and rediscovery
  - Execute `bash scripts/a2_provision.sh` — consumes the now-populated manifest
  - Use `scripts/a_run_all.sh` as an operator convenience wrapper, not as a required blocker for Phase A execution
  - Verify contract_manifest.json was generated with live-discovered RPC methods (not just deployment_required flag)
  - Verify a2_provisioning_outputs.json was generated
  - If Matrix session was created (registration not blocked): verify a2_matrix_session.json exists
  - If Matrix registration was blocked: verify a2_provisioning_outputs.json contains `matrix_session: "SKIPPED"` with documented reason
  - If any phase fails, collect bridge logs and diagnose

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (execution task)
  - **Parallel Group**: Wave 4
  - **Blocks**: T11, T12
  - **Blocked By**: T2, T3, T4

  **References**:
  - All Phase A scripts (T2-T5)
  - `.env` — VPS credentials

  **Acceptance Criteria**:
  - [ ] contract_manifest.json exists with `live_discovered.rpc_methods` containing ≥1 method with `.status == "responds"` (minimum); ≥10 is stretch goal
  - [ ] a2_provisioning_outputs.json exists with non-empty `homeserver_url`
  - [ ] Either a2_matrix_session.json exists (registration succeeded) OR a2_provisioning_outputs.json contains `matrix_session: "SKIPPED"` with documented reason

  **QA Scenarios:**
  ```
  Scenario: Contract manifest has live-discovered methods
    Tool: Bash
    Steps:
      1. test -f .sisyphus/evidence/armorclaw/contract_manifest.json
      2. jq '.live_discovered.rpc_methods | length' .sisyphus/evidence/armorclaw/contract_manifest.json → >= 1
      3. jq '[.live_discovered.rpc_methods[] | select(.status=="responds")] | length' .sisyphus/evidence/armorclaw/contract_manifest.json → >= 1
    Expected Result: Manifest with at least one responding RPC method
    Evidence: .sisyphus/evidence/task-10-contract-check.txt

  Scenario: Provisioning outputs exist with valid homeserver
    Tool: Bash
    Steps:
      1. test -f .sisyphus/evidence/armorclaw/a2_provisioning_outputs.json
      2. jq '.homeserver_url' .sisyphus/evidence/armorclaw/a2_provisioning_outputs.json → non-empty
    Expected Result: Valid provisioning data
    Evidence: .sisyphus/evidence/task-10-provisioning-check.txt

  Scenario: Matrix session handled (present or controlled skip)
    Tool: Bash
    Steps:
      1. Check: test -f .sisyphus/evidence/armorclaw/a2_matrix_session.json → pass
         OR jq '.matrix_session' .sisyphus/evidence/armorclaw/a2_provisioning_outputs.json → "SKIPPED"
    Expected Result: Either session file exists, or skip is documented
    Evidence: .sisyphus/evidence/task-10-matrix-session-check.txt
  ```

  **Commit**: Only if fixes needed

- [x] 11. Run Phase A3 — validate events on live deployment

  **What to do**:
  - Execute `bash scripts/a3_events.sh` directly (or `bash scripts/a_run_all.sh A3`)
  - Verify event validation completed
  - Check a3_summary.json for pass/fail/skip counts
  - If Matrix session was SKIPPED (from T10): expect A3.1-A3.3 to be SKIP, A3.4-A3.5 may still produce results
  - If session exists: expect at least one pass from A3.1-A3.3
  - If all SKIP, check bridge logs for event emission evidence
  - Verify a3.5_discovered_event_types.txt was created (may be empty if no events found — acceptable with documented reason)

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T12)
  - **Parallel Group**: Wave 4 (after T10)
  - **Blocks**: None
  - **Blocked By**: T5, T10

  **Acceptance Criteria**:
  - [ ] a3_summary.json exists with numeric pass/fail/skip
  - [ ] a3.5_discovered_event_types.txt exists (may be empty with documented reason)
  - [ ] If Matrix session was available: at least one of A3.1-A3.3 shows pass or documented skip

  **Commit**: Only if fixes needed

- [x] 12. Run Phase A4 — execute Tier A harness on VPS

  **What to do**:
  - Execute `bash scripts/a_run_all.sh A4` (defaults to health suite)
  - Verify harness results
  - Check a4_summary.json
  - If health suite passes, optionally run additional suites (trust, workflow-core, email)
  - Document any failures with VPS bridge logs

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T11)
  - **Parallel Group**: Wave 4 (after T10)
  - **Blocks**: None
  - **Blocked By**: T8, T10

  **Acceptance Criteria**:
  - [ ] a4_summary.json exists
  - [ ] Health suite result is PASS or documented failure with diagnosis

  **Commit**: YES (final)
  - Message: `test(e2e): Plan A execution — contract discovery + Tier A results`
  - Files: Evidence files + any fixes

---

## Final Verification Wave

- [x] F1. **Contract manifest exists** — `test -f .sisyphus/evidence/armorclaw/contract_manifest.json && jq '.live_discovered.rpc_methods | length'` → ≥1
- [x] F2. **Deployment healthy** — `source .env && curl -sf http://$VPS_IP:$BRIDGE_PORT/health` → 200 (or evidence of healthy VPS)
- [x] F3. **Provisioning + admin RPC** — `jq '.homeserver_url' .sisyphus/evidence/armorclaw/a2_provisioning_outputs.json` → non-empty AND at least one responding RPC whose name matches an admin/provisioning/bridge-status pattern (e.g. `jq '[.live_discovered.rpc_methods[] | select(.status=="responds") | .name] | map(select(test("admin|provision|bridge|status|config|health")))' contract_manifest.json` → ≥1 match)
- [x] F4. **Event validation** — `jq '.pass' .sisyphus/evidence/armorclaw/a3_summary.json` → ≥1 OR documented reason for all-SKIP
- [x] F5. **Harness results** — `jq '.pass' .sisyphus/evidence/armorclaw/a4_summary.json` → ≥1 for health suite

## Operational Safety

### Cleanup / Rollback (Manual Procedure)
After Plan A execution completes, the operator may run cleanup manually:
- Stop and remove Plan A deployment containers created by A1 (`ssh_vps "docker stop ... && docker rm ..."`)
- Remove temporary compose artifacts written by the E2E plan (`ssh_vps "rm -f /opt/armorclaw/docker-compose.plan-a.yml"`)
- Capture final bridge logs before teardown: `ssh_vps "docker logs armorclaw-bridge > /tmp/bridge-final.log 2>&1"`
- Preserve `.sisyphus/evidence/armorclaw/` by default
- This is a documented manual procedure, not an automated task within Plan A scope.

### Idempotency Rules
- A1 MUST detect an already-healthy deployment and skip destructive redeploy unless `FORCE_REDEPLOY=1`
- A2 MUST detect an existing admin claim / Matrix user / test room and reuse them where possible
- A3 MUST use per-run room/message markers or fresh sync cursors to avoid replay ambiguity
- A4 MUST safely overwrite the VPS test workspace without modifying source-controlled local test files

### Provisioning Fallback
- T4 includes creating a test Matrix user and room. In some deployments that will be blocked by registration policy.
- Attempt automated registration via discovered path; if blocked, record the restriction and mark the user/room setup as a controlled SKIP rather than treating it as an unexpected failure.

## Evidence Security (Manual Procedure)
- Before committing any evidence, the operator should verify `.gitignore` covers `.sisyphus/evidence/armorclaw/*session*.json` and files containing raw tokens.
- Add to `.gitignore` if not present: `.sisyphus/evidence/armorclaw/*session*.json` and `.sisyphus/evidence/armorclaw/*token*`.
- Raw Matrix access tokens, cookies, QR payload secrets, and session JSON are runtime-only artifacts — never commit unredacted.
- This is a documented manual hygiene step, not an automated task within Plan A scope.

## Commit Strategy

- **T1**: `feat(tests): add contract discovery helper library`
- **T2**: `feat(tests): add Phase A0 contract discovery script`
- **T3**: `feat(tests): add Phase A1 topology-aware deployment script`
- **T4**: `feat(tests): add Phase A2 provisioning and admin bootstrap`
- **T5**: `feat(tests): add Phase A3 control-plane event validation`
- **T6**: `feat(skills): add e2e-deploy skill definition`
- **T7-T9**: `feat(tests): add Phase A4 harness runner and master runner`
- **T10-T12**: `test(e2e): Plan A execution results`

## Success Criteria

### Verification Commands
```bash
# Contract discovery
test -f .sisyphus/evidence/armorclaw/contract_manifest.json && echo "PASS"

# Deployment health
source .env && curl -sf http://$VPS_IP:$BRIDGE_PORT/health && echo "PASS"

# Event validation
jq '.pass + .skip > 0' .sisyphus/evidence/armorclaw/a3_summary.json 2>/dev/null && echo "PASS"

# Harness execution
jq '.pass > 0' .sisyphus/evidence/armorclaw/a4_summary.json 2>/dev/null && echo "PASS"

# Provisioning outputs
jq '.homeserver_url' .sisyphus/evidence/armorclaw/a2_provisioning_outputs.json 2>/dev/null && echo "PASS"
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] contract_manifest.json with ≥1 discovered RPC method (minimum; ≥10 is stretch goal)
- [ ] No hardcoded RPC method assumptions in any script
- [ ] Existing tests/lib/ reused (ssh_vps, load_env pattern)
- [ ] Voice excluded from Tier B
