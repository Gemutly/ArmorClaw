# Customer-Facing ArmorClaw Ops Skill

## TL;DR

> **Quick Summary**: Build a consolidated `.skills/ops.yaml` skill that lets users connect their LLM (Claude Code, OpenCode, etc.) to deploy, monitor, and remediate ArmorClaw on a VPS — with progressive disclosure for both technical and non-technical operators.
> 
> **Deliverables**:
> - `.skills/ops.yaml` — Consolidated ops skill with 6 action paths (deploy, health, redeploy, logs, backup, restore)
> - `.skills/ops/SKILL.md` — Extended human documentation with progressive disclosure
> - `.skills/README.md` — Updated index with ops skill entry
> 
> **Estimated Effort**: Medium
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: T1 (ops.yaml) → T2 (SKILL.md) → T3 (README) → T4 (tests) → FINAL

---

## Context

### Original Request
Make a customer-facing skill that lets users hook up their LLMs to a set of skills that help deploy and manage the startup of ArmorClaw. This includes checking up on the sidecars and ensuring that the bridge/matrix/conduit are up and running. Also give the LLM some skills to redeploy unhealthy portions.

### Interview Summary
**Key Discussions**:
- **Target audience**: Both developers and non-technical operators — progressive disclosure (simple default, advanced options)
- **Organization**: One consolidated ops.yaml with action parameter routing
- **Autonomy**: Confirm before action — LLM proposes, asks before restart/redeploy
- **Deployment modes**: Native (Unix sockets) + Sentinel (TCP + Caddy + Let's Encrypt)
- **Existing skills**: Keep deploy.yaml, status.yaml, cloudflare.yaml, provision.yaml — add ops.yaml alongside
- **Health depth**: Endpoint + container status checks (pass/fail with remediation guidance)
- **Additional ops**: Logs viewing + backup/restore

**Research Findings**:
- ArmorClaw has 14+ services across 5 dependency layers
- 7 different compose files define overlapping topologies — NOT one monolithic stack
- Health endpoints: Matrix (6167), Bridge (8443/socket), Vault (socket), Sygnal (5000), Qdrant (6333), Caddy (config validate), Jetski (9222), Browser-Service (3000), Catwalk (8080), Coturn (UDP 3478)
- Socket-based services (vault, sidecar-office) have no HTTP healthcheck — need fallback checks
- deploy/health-check.sh already exists as comprehensive health oracle on the VPS
- Existing .skills/ schema: 5 top-level keys, 3 automation levels, SSH-based execution

### Metis Review
**Identified Gaps** (addressed):
- **Topology detection**: 7 compose files with overlapping services — skill must auto-detect which topology is deployed → Added topology detection step in health/redeploy actions
- **Deploy action scope**: Should ops.yaml deploy re-implement or delegate? → Delegates to existing deploy/install.sh installer (keep it simple, don't duplicate)
- **No-healthcheck services**: vault, sidecar-office, coturn use socket/UDP → Added socket-based fallback health checks

---

## Work Objectives

### Core Objective
Create a single consolidated `.skills/ops.yaml` skill that gives LLMs full lifecycle control over ArmorClaw deployments — deploy from scratch, monitor health, diagnose issues, and remediate by restarting unhealthy components in the correct dependency order.

### Concrete Deliverables
- `.skills/ops.yaml` — Complete skill file with all 6 action paths
- `.skills/ops/SKILL.md` — Extended documentation for human readers
- `.skills/README.md` — Updated index with ops skill entry

### Definition of Done
- [x] `ops.yaml` parses as valid YAML with correct schema
- [x] All 6 action paths (deploy, health, redeploy, logs, backup, restore) have functional steps
- [x] Each step correctly gates on the `action` parameter
- [x] Health action checks all core services (bridge, matrix, vault) + sidecars
- [x] Redeploy action restarts in dependency order (vault → conduit → bridge → support → sidecars)
- [x] Platform detection step matches existing .skills/ pattern exactly

### Must Have
- Action parameter routing (deploy|health|redeploy|logs|backup|restore)
- Progressive disclosure (simple output default, verbose for details)
- Dependency-aware restart ordering
- Socket-based fallback for vault/sidecar health checks
- Topology auto-detection (which compose stack is running)
- Confirm automation for destructive operations (restart, redeploy, restore)
- Auto automation for read-only operations (health, logs, status)
- Guide automation for manual intervention steps

### Must NOT Have (Guardrails)
- Do NOT modify existing .skills/ files (deploy.yaml, status.yaml, cloudflare.yaml, provision.yaml)
- Do NOT add Bridge-internal Go handlers — this is an external AI CLI tool skill only
- Do NOT implement SSH key management — assume user has SSH access configured
- Do NOT add retry loops for Bridge WebSocket failures (crash-only design per review.md)
- Do NOT duplicate deploy/install.sh logic — delegate to existing installer
- Do NOT add monitoring/alerting infrastructure — this is a point-in-time check tool
- Do NOT expose secrets in output — mask tokens, keys, passwords
- Do NOT bypass the confirm automation level for any destructive operation

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** - ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: NO (no .yaml skill testing exists)
- **Automated tests**: YES (tests after implementation)
- **Framework**: Shell-based validation (yq for YAML parsing, bash for step execution verification)
- **If TDD**: N/A — tests after

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **YAML validation**: Use `yq` or `python3 -c "import yaml"` to validate schema
- **Step logic**: Use Bash to simulate action parameter and verify step gates correctly
- **Documentation**: Verify SKILL.md exists with correct frontmatter
- **Integration**: Verify ops.yaml appears in README.md index

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation - parallel):
├── T1: Create ops.yaml with full schema + all action paths [unspecified-high]
└── T2: Create ops/SKILL.md extended documentation [writing]

Wave 2 (After Wave 1):
├── T3: Update .skills/README.md with ops entry [quick]
└── T4: Write validation tests for ops.yaml [unspecified-high]

Wave FINAL (After ALL tasks — 4 parallel reviews, then user okay):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Real manual QA (unspecified-high)
└── F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: T1 → T4 → F1-F4 → user okay
Parallel Speedup: ~40% faster than sequential
Max Concurrent: 2 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1 | - | T3, T4, F1-F4 | 1 |
| T2 | - | T3 | 1 |
| T3 | T2 | F1-F4 | 2 |
| T4 | T1 | F1-F4 | 2 |

### Agent Dispatch Summary

- **Wave 1**: 2 agents — T1 → `unspecified-high`, T2 → `writing`
- **Wave 2**: 2 agents — T3 → `quick`, T4 → `unspecified-high`
- **FINAL**: 4 agents — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Create `.skills/ops.yaml` — Consolidated Ops Skill with All Action Paths

  **What to do**:
  - Create `.skills/ops.yaml` following the exact schema from `.skills/TEMPLATE.yaml`
  - Define schema with these top-level keys: `name: "armorclaw_ops"`, `version: "1.0.0"`, `description`
  - Define parameters (see list below)
  - Write ALL steps for ALL 6 action paths, each gated by `action` parameter check
  - Follow the SSH command pattern from `.skills/PLATFORM.md` exactly
  - Use the platform detection block verbatim from existing skills
  - Use Unicode box-drawing separators (━━━) for section output formatting
  - Use status indicators: ✓ ✗ ⚠ ℹ

  **Parameters to define**:
  ```yaml
  parameters:
    - name: "action"
      type: "string"
      required: true
      description: "Action to perform: deploy|health|redeploy|logs|backup|restore"
      default: "health"
    - name: "vps_ip"
      type: "string"
      required: true
      description: "IP address or hostname of target VPS"
      default: ""
    - name: "ssh_user"
      type: "string"
      required: false
      description: "SSH username for VPS access"
      default: "root"
    - name: "ssh_key"
      type: "string"
      required: false
      description: "Path to SSH private key"
      default: "~/.ssh/id_ed25519"
    - name: "service"
      type: "string"
      required: false
      description: "Target service for action (bridge|matrix|vault|sygnal|qdrant|caddy|jetski|browser|sidecar-office|coturn|catwalk|all)"
      default: "all"
    - name: "mode"
      type: "string"
      required: false
      description: "Deployment mode: native|sentinel"
      default: "native"
    - name: "verbose"
      type: "boolean"
      required: false
      description: "Show detailed diagnostic output"
      default: "false"
    - name: "domain"
      type: "string"
      required: false
      description: "Public domain for sentinel mode (required for deploy with mode=sentinel)"
      default: ""
    - name: "tail"
      type: "string"
      required: false
      description: "Number of log lines to show (for logs action)"
      default: "100"
    - name: "backup_path"
      type: "string"
      required: false
      description: "Path for backup file (for backup/restore actions)"
      default: "/tmp/armorclaw-backup-$(date +%Y%m%d-%H%M%S).tar.gz"
  ```

  **Steps to write** (each gated by action check):

  **Step 1: `detect_platform`** (automation: auto, runs for ALL actions)
  - Verbatim platform detection block from existing skills
  - Gate: always runs

  **Step 2: `validate_ssh`** (automation: auto, runs for ALL actions)
  - Validate SSH key exists and VPS is reachable
  - Gate: always runs

  **Step 3: `detect_topology`** (automation: auto, runs for health|redeploy|logs|backup|restore)
  - SSH to VPS and detect which compose stack is running:
    - Check for running containers: `docker ps --format '{{.Names}}'`
    - Identify topology based on container names present:
      - Quickstart: `armorclaw` single container + `armorclaw-conduit`
      - Sentinel: `armorclaw-sentinel`, `armorclaw-conduit`, `armorclaw-caddy`, `armorclaw-vault`
      - Full: adds `armorclaw-sygnal`, `armorclaw-qdrant`
      - Topology-separation: separate compose stacks
    - Store detected topology in `TOPOLOGY` variable for subsequent steps
    - Print detected topology to user

  **Step 4: `deploy`** (automation: confirm, runs for action=deploy ONLY)
  - Gate: `if [ "$action" != "deploy" ]; then exit 0; fi`
  - Delegate to existing deploy/install.sh — do NOT reimplement
  - SSH to VPS and run: `bash -c "$(curl -fsSL https://raw.githubusercontent.com/mikmin/armorclaw/v0.7.0/deploy/install.sh)"`
  - Pass through mode and domain parameters as environment variables
  - After installer completes, run health verification
  - Print connection info (admin token, URLs)

  **Step 5: `check_health`** (automation: auto, runs for action=health ONLY)
  - Gate: `if [ "$action" != "health" ]; then exit 0; fi`
  - SSH to VPS and check each service in dependency order:
    - Layer 0: Docker daemon (`docker info`), Vault socket (`test -S /run/armorclaw/keystore.sock`)
    - Layer 1: Matrix Conduit (`curl -sf http://localhost:6167/_matrix/client/versions`)
    - Layer 2: Bridge socket (`test -S /run/armorclaw/bridge.sock`) + HTTP (`curl -sf http://localhost:8443/health` or `curl -sf http://localhost:8081/health` in sentinel)
    - Layer 3: Sygnal (`curl -sf http://localhost:5000/health`), Qdrant (`curl -sf http://localhost:6333/healthz`)
    - Layer 4: Sidecars — Jetski (`curl -sf http://localhost:9222/health`), Browser-Service (`curl -sf http://localhost:3000/health`), Sidecar-Office socket (`test -S /run/armorclaw/office-sidecar/sidecar-office.sock`)
    - Layer 5: Caddy (only if sentinel mode: `docker exec armorclaw-caddy caddy validate --config /etc/caddy/Caddyfile 2>/dev/null`)
  - For each service: print ✓ SERVICENAME (healthy) or ✗ SERVICENAME (unhealthy - REASON)
  - At the end, print summary: "X/Y services healthy" with overall status
  - If verbose=true, also show container uptime, memory usage, restart counts

  **Step 6: `redeploy`** (automation: confirm, runs for action=redeploy ONLY)
  - Gate: `if [ "$action" != "redeploy" ]; then exit 0; fi`
  - SSH to VPS and restart services in dependency order based on `service` parameter:
    - If service=all or service not specified:
      1. Stop in reverse order: sidecars → support → bridge → matrix → vault
      2. Start in dependency order: vault → matrix (wait healthy) → bridge (wait healthy) → support → sidecars
      3. Each step waits for health before proceeding (max 60s per service)
    - If service=specific:
      1. Stop only that service: `docker restart <container-name>`
      2. Wait for health (max 60s)
      3. If the service depends on others, check dependencies first
  - Print each step with ✓/✗ indicator
  - After all restarts, run a full health check to verify recovery
  - If any service fails to recover, print diagnostic guidance

  **Step 7: `view_logs`** (automation: auto, runs for action=logs ONLY)
  - Gate: `if [ "$action" != "logs" ]; then exit 0; fi`
  - SSH to VPS and run `docker logs --tail ${tail} <container>`
  - Map service parameter to container name:
    - bridge → armorclaw-sentinel (or armorclaw for native)
    - matrix → armorclaw-conduit
    - vault → armorclaw-vault
    - sygnal → armorclaw-sygnal
    - etc.
  - If service=all, show last 20 lines from each core service (bridge, matrix, vault)
  - Print with service header separator

  **Step 8: `create_backup`** (automation: confirm, runs for action=backup ONLY)
  - Gate: `if [ "$action" != "backup" ]; then exit 0; fi`
  - SSH to VPS and create tar.gz backup of:
    - `/opt/armorclaw/.env` (secrets and config)
    - `/opt/armorclaw/configs/` (Caddyfile, conduit.toml, sygnal.yaml)
    - `/opt/armorclaw/data/` (Matrix data, if exists)
    - Docker volumes: `armorclaw-qdrant-data`, `caddy_data`, `caddy_config` (via `docker run --rm -v ... alpine tar`)
  - Write to backup_path on VPS
  - Print backup size and location
  - Mask any secrets in output (show first 4 chars + ***)

  **Step 9: `restore_backup`** (automation: confirm, runs for action=restore ONLY)
  - Gate: `if [ "$action" != "restore" ]; then exit 0; fi`
  - SSH to VPS and verify backup file exists at backup_path
  - Validate backup contents (check for .env, configs/)
  - Stop all services (in reverse dependency order)
  - Restore files from tar.gz
  - Restart services in dependency order (vault → matrix → bridge → support → sidecars)
  - Run full health check to verify restoration

  **Step 10: `print_summary`** (automation: auto, runs for ALL actions)
  - Print action summary with next steps guidance
  - For health: "If issues found, run /ops action=redeploy service=<name>"
  - For redeploy: "Run /ops action=health to verify"
  - For deploy: "Run /ops action=health to verify, then /ops action=backup to save config"
  - For logs: "For persistent issues, run /ops action=redeploy"
  - For backup: "Restore with /ops action=restore backup_path=<path>"
  - For restore: "Run /ops action=health to verify restoration"

  **Must NOT do**:
  - Do NOT modify any existing .skills/ files
  - Do NOT duplicate deploy/install.sh logic — delegate to it
  - Do NOT add Go Bridge handlers
  - Do NOT expose secrets in output
  - Do NOT use `return 0` (use `exit 0` for early-exit)
  - Do NOT hardcode IPs or ports — use variables from parameters

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multi-file creation with complex bash scripts, needs careful attention to YAML schema compliance and SSH command patterns
  - **Skills**: `[]`
    - No special skills needed — this is YAML + bash writing

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T2)
  - **Parallel Group**: Wave 1 (with T2)
  - **Blocks**: T3, T4, F1-F4
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References** (existing code to follow):
  - `.skills/TEMPLATE.yaml` — Exact YAML schema structure to follow (5 top-level keys, parameter schema, step schema)
  - `.skills/deploy.yaml` — SSH command patterns, platform detection block, deploy step structure, output formatting
  - `.skills/status.yaml` — Health check patterns (check_docker, check_matrix, check_bridge, check_services steps)
  - `.skills/PLATFORM.md` — Cross-platform SSH patterns, path handling, error handling conventions
  - `.skills/provision.yaml` — Confirm automation pattern for QR generation (how to ask user before action)

  **API/Type References** (health endpoints to check):
  - `docker-compose.yml` — Container names, health check commands, network configuration for Native mode
  - `docker-compose-full.yml` — Container names, health check commands, dependency order for Sentinel/Full mode
  - `deploy/health-check.sh` — Comprehensive health check script with all service endpoints and expected responses

  **Architecture References** (dependency order and restart commands):
  - Bridge: `docker restart armorclaw-sentinel` (sentinel) or `sudo systemctl restart armorclaw-bridge` (native)
  - Matrix: `docker restart armorclaw-conduit`
  - Vault: `docker restart armorclaw-vault`
  - Sygnal: `docker restart armorclaw-sygnal`
  - Qdrant: `docker restart armorclaw-qdrant`
  - Caddy: `docker restart armorclaw-caddy` (sentinel only)
  - Jetski: `docker restart jetski`
  - Browser: `docker restart armorclaw-browser`
  - Sidecar-Office: `docker restart armorclaw-sidecar-office`
  - Dependency order: vault → conduit → bridge → sygnal/qdrant → caddy → sidecars

  **External References**:
  - YAML spec: Use block scalars (`|`) for multi-line commands
  - Docker health check: `curl -sf http://localhost:PORT/path` returns 0 on healthy

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: ops.yaml parses as valid YAML
    Tool: Bash
    Preconditions: ops.yaml exists at .skills/ops.yaml
    Steps:
      1. Run: python3 -c "import yaml; d=yaml.safe_load(open('.skills/ops.yaml')); print(d['name'])"
      2. Assert output contains "armorclaw_ops"
    Expected Result: YAML parses without error, name field is "armorclaw_ops"
    Failure Indicators: Python traceback, KeyError, parse error
    Evidence: .sisyphus/evidence/task-1-yaml-parse.txt

  Scenario: All 6 actions have gated steps
    Tool: Bash
    Preconditions: ops.yaml exists
    Steps:
      1. Run: grep -c 'action.*!=.*"deploy\|action.*!=.*"health\|action.*!=.*"redeploy\|action.*!=.*"logs\|action.*!=.*"backup\|action.*!=.*"restore"' .skills/ops.yaml
      2. Assert count >= 6 (at least one gate per action)
    Expected Result: At least 6 action gates found in the file
    Failure Indicators: Count < 6, meaning some actions are ungateed
    Evidence: .sisyphus/evidence/task-1-action-gates.txt

  Scenario: Platform detection block matches existing skills
    Tool: Bash
    Preconditions: ops.yaml exists, deploy.yaml exists
    Steps:
      1. Extract platform detection from deploy.yaml
      2. Extract platform detection from ops.yaml
      3. Compare: they should be identical (or functionally equivalent)
    Expected Result: Platform detection blocks match
    Failure Indicators: Different platform names, different detection logic
    Evidence: .sisyphus/evidence/task-1-platform-detection.txt

  Scenario: No shell injection in parameters
    Tool: Bash
    Preconditions: ops.yaml exists
    Steps:
      1. Check all SSH commands properly quote variables: grep for unquoted variable expansion
      2. Verify no `eval` usage
      3. Verify no unsanitized `$()` in user-provided parameter contexts
    Expected Result: All variables properly quoted, no eval, no injection vectors
    Failure Indicators: Unquoted $VARIABLE in SSH command strings, eval usage
    Evidence: .sisyphus/evidence/task-1-shell-safety.txt
  ```

  **Commit**: YES (groups with T2)
  - Message: `feat(skills): add consolidated ops skill with full lifecycle management`
  - Files: `.skills/ops.yaml`
  - Pre-commit: `python3 -c "import yaml; yaml.safe_load(open('.skills/ops.yaml'))"`

- [x] 2. Create `.skills/ops/SKILL.md` — Extended Documentation

  **What to do**:
  - Create `.skills/ops/SKILL.md` with YAML frontmatter and extended documentation
  - Frontmatter: `name: armorclaw_ops`, `version: 1.0.0`, `description: Consolidated ArmorClaw lifecycle management`
  - Write documentation sections:
    - **Overview**: What the skill does, who it's for
    - **Quick Start**: Simple examples for each action (progressive disclosure — simple first)
    - **Actions Reference**: Detailed docs for each action (deploy, health, redeploy, logs, backup, restore)
    - **Parameters Reference**: Table of all parameters with types, defaults, and descriptions
    - **Service Names**: Table mapping friendly names to container names
    - **Dependency Order**: Visual diagram showing service dependency layers
    - **Troubleshooting**: Common issues and remediation commands
    - **Advanced Usage**: Verbose mode, specific service targeting, custom backup paths
  - Include concrete examples for every action
  - Use progressive disclosure: simple examples at top, advanced scenarios at bottom

  **Must NOT do**:
  - Do NOT duplicate the entire ops.yaml step content in prose
  - Do NOT modify existing SKILL.md files in other skill directories

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Documentation-focused task requiring clear technical writing and progressive disclosure
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T1)
  - **Parallel Group**: Wave 1 (with T1)
  - **Blocks**: T3
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `.skills/deploy/SKILL.md` — Existing SKILL.md format and structure to match
  - `.skills/status/SKILL.md` — Another existing SKILL.md for reference
  - `.skills/provision/SKILL.md` — Third example of SKILL.md format

  **API/Type References**:
  - `.skills/ops.yaml` — The skill being documented (T1 creates this, but the writer can reference the schema)
  - `.skills/README.md` — Existing index showing how skills are summarized

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: SKILL.md has correct YAML frontmatter
    Tool: Bash
    Preconditions: .skills/ops/SKILL.md exists
    Steps:
      1. Run: head -6 .skills/ops/SKILL.md
      2. Assert first line is "---"
      3. Assert "name:" line exists in frontmatter
      4. Assert "version:" line exists in frontmatter
      5. Assert "description:" line exists in frontmatter
      6. Assert closing "---" exists
    Expected Result: Valid YAML frontmatter with name, version, description
    Failure Indicators: Missing frontmatter, missing fields, malformed YAML
    Evidence: .sisyphus/evidence/task-2-frontmatter.txt

  Scenario: Documentation covers all 6 actions
    Tool: Bash
    Preconditions: .skills/ops/SKILL.md exists
    Steps:
      1. For each action in deploy health redeploy logs backup restore:
         grep -qi "action" .skills/ops/SKILL.md
      2. Assert all 6 actions are mentioned
    Expected Result: All 6 actions documented
    Failure Indicators: Any action not mentioned
    Evidence: .sisyphus/evidence/task-2-action-coverage.txt

  Scenario: Documentation includes examples
    Tool: Bash
    Preconditions: .skills/ops/SKILL.md exists
    Steps:
      1. grep -c "/ops" .skills/ops/SKILL.md
      2. Assert at least 6 example invocations found
    Expected Result: Multiple usage examples provided
    Failure Indicators: Fewer than 6 example invocations
    Evidence: .sisyphus/evidence/task-2-examples.txt
  ```

  **Commit**: YES (groups with T1)
  - Message: `feat(skills): add consolidated ops skill with full lifecycle management`
  - Files: `.skills/ops/SKILL.md`
  - Pre-commit: none

- [x] 3. Update `.skills/README.md` with Ops Skill Entry

  **What to do**:
  - Read current `.skills/README.md` to understand the existing index format
  - Add a new row for the ops skill in the quick-reference table
  - Include: name, description, parameters summary, example commands
  - Match the existing table format exactly
  - Ensure the ops skill entry is listed alongside deploy, status, cloudflare, provision

  **Must NOT do**:
  - Do NOT modify existing skill entries in the README
  - Do NOT change the README structure or format

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single file edit, adding one row to an existing table
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T4)
  - **Parallel Group**: Wave 2 (with T4)
  - **Blocks**: F1-F4
  - **Blocked By**: T2 (need to know documentation exists)

  **References**:

  **Pattern References**:
  - `.skills/README.md` — Existing index format (read first, match exactly)

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: README references ops skill
    Tool: Bash
    Preconditions: .skills/README.md updated
    Steps:
      1. grep -i "ops" .skills/README.md
      2. Assert output contains "armorclaw_ops" or "ops" reference
    Expected Result: Ops skill appears in README index
    Failure Indicators: No mention of ops skill
    Evidence: .sisyphus/evidence/task-3-readme-ops.txt

  Scenario: Existing entries unchanged
    Tool: Bash
    Preconditions: .skills/README.md updated
    Steps:
      1. git diff .skills/README.md | grep "^-" | grep -v "^---" | wc -l
      2. Assert no existing lines were removed (only additions)
    Expected Result: Only new lines added, no existing content removed
    Failure Indicators: Lines deleted from existing entries
    Evidence: .sisyphus/evidence/task-3-readme-diff.txt
  ```

  **Commit**: YES (groups with T4)
  - Message: `test(skills): add ops skill validation and README update`
  - Files: `.skills/README.md`
  - Pre-commit: none

- [x] 4. Write Validation Tests for ops.yaml

  **What to do**:
  - Create `.skills/ops/tests/` directory
  - Write a `validate.sh` shell script that:
    - Validates ops.yaml parses as correct YAML
    - Checks all required top-level keys exist (name, version, description, parameters, steps)
    - Checks all required parameters are defined (action, vps_ip, ssh_user, ssh_key, service, mode, verbose, domain, tail, backup_path)
    - Checks all 6 actions have at least one step that gates on them
    - Checks platform detection step exists
    - Checks all automation levels are valid (auto, confirm, guide only)
    - Checks no `return 0` (should be `exit 0`) in commands
    - Checks SSH commands properly quote variables
    - Checks no hardcoded IPs or ports (should use variables)
  - Write a `test-action-gates.sh` that simulates each action parameter and verifies the correct steps would execute
  - Make both scripts executable

  **Must NOT do**:
  - Do NOT test against a live VPS (these are offline validation tests)
  - Do NOT modify ops.yaml

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Test creation requiring bash scripting, YAML parsing, and careful validation logic
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T3)
  - **Parallel Group**: Wave 2 (with T3)
  - **Blocks**: F1-F4
  - **Blocked By**: T1 (need ops.yaml to exist to test against)

  **References**:

  **Pattern References**:
  - `.skills/ops.yaml` — The file being tested (created by T1)
  - `bridge/internal/skills/executor_dangerous_test.go` — Example test file patterns in this project

  **API/Type References**:
  - `.skills/TEMPLATE.yaml` — Schema to validate against

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: Validation script runs and passes
    Tool: Bash
    Preconditions: .skills/ops/tests/validate.sh exists and is executable
    Steps:
      1. Run: bash .skills/ops/tests/validate.sh
      2. Assert exit code is 0
      3. Assert output contains "PASS" for each check
    Expected Result: All validation checks pass
    Failure Indicators: Non-zero exit code, "FAIL" in output
    Evidence: .sisyphus/evidence/task-4-validate-output.txt

  Scenario: Action gate test runs and passes
    Tool: Bash
    Preconditions: .skills/ops/tests/test-action-gates.sh exists and is executable
    Steps:
      1. Run: bash .skills/ops/tests/test-action-gates.sh
      2. Assert exit code is 0
      3. Assert all 6 actions are tested
    Expected Result: All 6 actions have correctly gated steps
    Failure Indicators: Non-zero exit code, any action missing gates
    Evidence: .sisyphus/evidence/task-4-action-test-output.txt

  Scenario: Validation catches real errors
    Tool: Bash
    Preconditions: validate.sh exists
    Steps:
      1. Create a broken copy of ops.yaml (remove a parameter, add invalid automation)
      2. Run validate.sh against the broken copy
      3. Assert exit code is non-zero (validation fails)
      4. Clean up broken copy
    Expected Result: Validation correctly rejects broken YAML
    Failure Indicators: Validation passes on broken input
    Evidence: .sisyphus/evidence/task-4-negative-test.txt
  ```

  **Commit**: YES (groups with T3)
  - Message: `test(skills): add ops skill validation tests and README update`
  - Files: `.skills/ops/tests/validate.sh`, `.skills/ops/tests/test-action-gates.sh`
  - Pre-commit: `bash .skills/ops/tests/validate.sh`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, check YAML structure). For each "Must NOT Have": search ops.yaml for forbidden patterns — reject with line number if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Validate ops.yaml is well-formed YAML. Check: consistent indentation, no shell injection vulnerabilities, all parameters referenced in steps, no hardcoded IPs/ports (use variables). Verify bash scripts handle missing commands gracefully (curl not found, docker not installed). Check SKILL.md frontmatter matches ops.yaml metadata.
  Output: `YAML [PASS/FAIL] | Shell Safety [N issues] | Parameter Coverage [N/N] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Parse ops.yaml with `python3 -c "import yaml; yaml.safe_load(open('.skills/ops.yaml'))"` — verify no parse errors. Extract each step's command and verify the action-gating logic works (each step should have an action check at the top). Verify the README.md references the ops skill. Verify SKILL.md exists with proper frontmatter.
  Output: `Parse [PASS/FAIL] | Action Gates [N/N] | Docs [PASS/FAIL] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual file content. Verify 1:1 — everything in spec was built, nothing beyond spec was built. Check "Must NOT do" compliance. Verify existing .skills/ files are untouched (git diff should show no changes to deploy.yaml, status.yaml, cloudflare.yaml, provision.yaml). Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Existing Files [UNTOUCHED/MODIFIED] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **T1+T2**: `feat(skills): add consolidated ops skill with full lifecycle management` - .skills/ops.yaml, .skills/ops/SKILL.md
- **T3+T4**: `test(skills): add ops skill validation tests and README update` - .skills/README.md, .skills/ops/tests/

---

## Success Criteria

### Verification Commands
```bash
# YAML parses correctly
python3 -c "import yaml; d=yaml.safe_load(open('.skills/ops.yaml')); assert d['name'] == 'armorclaw_ops'; print('OK')"
# Expected: OK

# All 6 actions have at least one step
python3 -c "
import yaml
d=yaml.safe_load(open('.skills/ops.yaml'))
actions = set()
for step in d['steps']:
    cmd = step['command']
    for action in ['deploy','health','redeploy','logs','backup','restore']:
        if action in cmd:
            actions.add(action)
assert len(actions) >= 6, f'Missing actions: {set([\"deploy\",\"health\",\"redeploy\",\"logs\",\"backup\",\"restore\"]) - actions}'
print(f'All {len(actions)} actions covered')
"
# Expected: All 6 actions covered

# SKILL.md exists with correct frontmatter
head -5 .skills/ops/SKILL.md | grep -q "name:"
# Expected: exit code 0

# README references ops skill
grep -q "ops" .skills/README.md
# Expected: exit code 0

# Existing skills untouched
git diff --name-only | grep -v "ops" | grep ".skills/" || echo "No changes to existing skills"
# Expected: No changes to existing skills
```

### Final Checklist
- [x] All "Must Have" present
- [x] All "Must NOT Have" absent
- [x] ops.yaml validates as correct YAML
- [x] All 6 action paths have functional steps
- [x] SKILL.md exists with correct frontmatter
- [x] README.md updated with ops entry
- [x] Existing .skills/ files untouched
