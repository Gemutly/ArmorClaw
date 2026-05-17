# Infrastructure Recommendations Implementation

## TL;DR

> **Quick Summary**: Implement 5 infrastructure improvements: bridge/coturn healthchecks, certbot systemd check, IPv4 preference in IP detection, Docker build error visibility, and container rollback mechanism.
>
> **Deliverables**:
> - Bridge healthcheck in docker-compose-full.yml
> - Coturn healthcheck in docker-compose.matrix.yml
> - Certbot systemd timer check script
> - IPv4-preferred IP detection with fallback
> - Docker build error visibility improvements
> - Container rollback mechanism (containers only)
>
> **Estimated Effort**: Medium
> **Parallel Execution**: YES - 2 waves
> **Critical Path**: Healthchecks → Rollback mechanism

---

## Context

### Original Request
Implement remaining non-blocking infrastructure recommendations:
1. Add bridge healthcheck to docker-compose-full.yml (LOW)
2. Show build errors on failure (LOW)
3. Prefer IPv4 in IP detection (LOW)
4. Add healthchecks for certbot/coturn (LOW)
5. Consider automatic rollback on failure (MEDIUM)

### Interview Summary
**Key Discussions**:
- Rollback scope: Containers only (not configs or volumes)
- Certbot healthcheck: Systemd timer/service check
- Build error visibility: Docker builds only (not all build scripts)

**Research Findings**:
- Bridge service has NO healthcheck (uses RPC over Unix socket at `/run/armorclaw/bridge.sock`)
- Coturn service has NO healthcheck (uses host network mode)
- IP detection uses `curl -s https://api.ipify.org` which can return IPv6
- Existing healthcheck pattern: wget spider mode with 30s interval, 10s timeout, 3 retries
- Certbot runs standalone (not in Docker), invoked via `certbot certonly --standalone`

### Metis Review
**Identified Gaps** (addressed):
- Bridge has no HTTP endpoint → use socket-based healthcheck
- Coturn needs simple port check (not full TURN allocation test)
- IPv4 fallback needed when only IPv6 available → try IPv4, warn on IPv6
- Rollback tracking → Docker tags with `:prev` suffix

---

## Work Objectives

### Core Objective
Add production-ready healthchecks, improve error visibility, and implement safe rollback for container deployments.

### Concrete Deliverables
- `docker-compose-full.yml` - bridge healthcheck added
- `docker-compose.matrix.yml` - coturn healthcheck added
- `scripts/certbot-healthcheck.sh` - systemd timer check script
- `docker-compose.matrix.yml` - IPv4-preferred IP detection
- `scripts/docker-build-with-errors.sh` - wrapper for better build error visibility
- `scripts/container-rollback.sh` - container rollback mechanism

### Definition of Done
- [ ] All healthchecks report healthy in `docker ps`
- [ ] IP detection returns IPv4 on dual-stack systems
- [ ] Build failures show clear error messages with context
- [ ] Rollback can restore previous container version

### Must Have
- Bridge healthcheck using socket (not HTTP, since bridge has no HTTP endpoint)
- Coturn UDP port check (simple, no TURN credentials needed)
- IPv4 preference with graceful IPv6 fallback
- Rollback for containers only

### Must NOT Have (Guardrails)
- NO bridge HTTP endpoint implementation
- NO TURN test credentials or turnutils installation
- NO certificate expiry validation (just systemd timer check)
- NO build notification/alerting system
- NO config or volume rollback
- NO changes to existing Matrix/Sygnal/Nginx healthchecks

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: NO unit tests
- **Automated tests**: NONE (infrastructure changes)
- **Verification**: Agent-executed QA scenarios via Bash commands

### QA Policy
Every task includes agent-executed QA scenarios using Bash commands to verify Docker healthcheck status, IP format, build output, and rollback functionality.

Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately - independent infrastructure):
├── Task 1: Bridge healthcheck in docker-compose-full.yml [quick]
├── Task 2: Coturn healthcheck in docker-compose.matrix.yml [quick]
├── Task 3: Certbot systemd check script [quick]
├── Task 4: IPv4 preference in IP detection [quick]
└── Task 5: Docker build error visibility [quick]

Wave 2 (After Wave 1 - rollback depends on healthchecks):
└── Task 6: Container rollback mechanism [unspecified-high]

Wave FINAL (After ALL tasks - verification):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Infrastructure health review (unspecified-high)
└── Task F3: Scope fidelity check (deep)

Critical Path: Task 1-5 (parallel) → Task 6 → Final
Parallel Speedup: ~70% faster than sequential
Max Concurrent: 5 (Wave 1)
```

### Dependency Matrix
- **1-5**: — — 6 (rollback needs healthchecks in place)
- **6**: 1, 2 — F1-F3
- **F1-F3**: 1-6 — —

### Agent Dispatch Summary
- **Wave 1**: **5** agents → all `quick`
- **Wave 2**: **1** agent → `unspecified-high`
- **FINAL**: **3** agents → `oracle`, `unspecified-high`, `deep`

---

## TODOs

- [x] 1. Bridge Healthcheck in docker-compose-full.yml

- [x] 2. Coturn Healthcheck in docker-compose.matrix.yml

  **What to do**:
  - Add healthcheck block to bridge service in docker-compose-full.yml
  - Use socket-based check since bridge has no HTTP endpoint
  - Follow existing pattern: 30s interval, 10s timeout, 3 retries
  - Add 60s start_period for slow Matrix connection

  **Must NOT do**:
  - Do NOT add HTTP endpoint to bridge binary
  - Do NOT change bridge service configuration beyond healthcheck

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single file edit, well-defined pattern to follow
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `git-master`: Simple edit, atomic commit afterward

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2-5)
  - **Blocks**: Task 6 (rollback)
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - `docker-compose-full.yml:23-28` - Matrix healthcheck pattern (wget spider, intervals)
  - `docker-compose-full.yml:54-59` - Sygnal healthcheck pattern (wget spider)
  - `docker-compose.matrix.yml:92-96` - Nginx healthcheck pattern

  **API/Type References**:
  - `docker-compose-full.yml:100-136` - Bridge service definition (add healthcheck here)
  - Bridge socket path: `/run/armorclaw/bridge.sock`

  **WHY Each Reference Matters**:
  - Matrix/Sygnal healthchecks show the pattern: wget spider, 30s interval, 10s timeout, 3 retries
  - Bridge service definition is where healthcheck block goes
  - Bridge socket is the endpoint for healthcheck (not HTTP)

  **Acceptance Criteria**:
  - [ ] healthcheck block added to bridge service
  - [ ] `docker compose -f docker-compose-full.yml config` validates
  - [ ] Healthcheck uses socket check, not HTTP

  **QA Scenarios**:

  ```
  Scenario: Bridge healthcheck reports healthy after startup
    Tool: Bash
    Preconditions: docker-compose-full.yml services running
    Steps:
      1. docker compose -f docker-compose-full.yml up -d
      2. sleep 70  # Wait for start_period + first check
      3. docker inspect --format='{{.State.Health.Status}}' armorclaw-bridge
    Expected Result: Output is "healthy"
    Failure Indicators: Output is "unhealthy" or "starting"
    Evidence: .sisyphus/evidence/task-01-bridge-healthy.txt

  Scenario: Bridge healthcheck fails when socket missing
    Tool: Bash
    Preconditions: Bridge container stopped
    Steps:
      1. docker stop armorclaw-bridge
      2. docker inspect --format='{{.State.Health.Status}}' armorclaw-bridge 2>/dev/null || echo "stopped"
    Expected Result: Output is "stopped" or health status is not "healthy"
    Failure Indicators: Health status is "healthy" (shouldn't be)
    Evidence: .sisyphus/evidence/task-01-bridge-unhealthy.txt
  ```

  **Evidence to Capture**:
  - [ ] docker inspect output showing healthy status
  - [ ] docker compose config validation output

  **Commit**: YES
  - Message: `feat(healthcheck): add bridge service healthcheck`
  - Files: `docker-compose-full.yml`
  - Pre-commit: `docker compose -f docker-compose-full.yml config`

---

  - [x] 2. Coturn Healthcheck in docker-compose.matrix.yml

  **What to do**:
  - Add healthcheck block to coturn service in docker-compose.matrix.yml
  - Use UDP port check (nc -z -u) for port 3478
  - Account for host network mode - healthcheck runs in container
  - Follow existing pattern: 30s interval, 10s timeout, 3 retries

- [x] 3. Certbot Systemd Check Script

  **Must NOT do**:
  - Do NOT add TURN test credentials
  - Do NOT install turnutils or other TURN testing tools
  - Do NOT change coturn configuration beyond healthcheck

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single file edit, simple port check pattern
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3-5)
  - **Blocks**: Task 6 (rollback)
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - `docker-compose.matrix.yml:92-96` - Nginx healthcheck pattern
  - `docker-compose-full.yml:23-28` - Matrix healthcheck pattern

  **API/Type References**:
  - `docker-compose.matrix.yml:15-54` - Coturn service definition (add healthcheck here)
  - Coturn ports: 3478 (STUN/TURN UDP), 5349 (TLS)

  **WHY Each Reference Matters**:
  - Nginx healthcheck in same file shows pattern
  - Coturn service needs healthcheck block, uses UDP port 3478

  **Acceptance Criteria**:
  - [ ] healthcheck block added to coturn service
  - [ ] `docker compose -f docker-compose.matrix.yml config` validates
  - [ ] Healthcheck uses UDP port check for 3478

  **QA Scenarios**:

  ```
  Scenario: Coturn healthcheck reports healthy with TURN server running
    Tool: Bash
    Preconditions: docker-compose.matrix.yml services running
    Steps:
      1. docker compose -f docker-compose.matrix.yml up -d coturn
      2. sleep 35  # Wait for interval + retries
      3. docker inspect --format='{{.State.Health.Status}}' armorclaw-coturn
    Expected Result: Output is "healthy"
    Failure Indicators: Output is "unhealthy"
    Evidence: .sisyphus/evidence/task-02-coturn-healthy.txt

  Scenario: Coturn healthcheck fails when port not listening
    Tool: Bash
    Preconditions: Coturn stopped
    Steps:
      1. docker stop armorclaw-coturn
      2. docker inspect --format='{{.State.Health.Status}}' armorclaw-coturn 2>/dev/null || echo "stopped"
    Expected Result: Output is "stopped" or status is not "healthy"
    Failure Indicators: None (verification of failure handling)
    Evidence: .sisyphus/evidence/task-02-coturn-stopped.txt
  ```

  **Evidence to Capture**:
  - [ ] docker inspect output showing healthy status
  - [ ] docker compose config validation output

  **Commit**: YES
  - Message: `feat(healthcheck): add coturn service healthcheck`
  - Files: `docker-compose.matrix.yml`
  - Pre-commit: `docker compose -f docker-compose.matrix.yml config`

---

- [x] 3. Certbot Systemd Check Script

  **What to do**:
  - Create `scripts/certbot-healthcheck.sh` to check systemd timer status
  - Check if certbot.timer (or certbot-renew.timer) is active
  - Optional: Check last successful run in journalctl (within 90 days)
  - Exit 0 if healthy, exit1 if unhealthy
  - Make script executable

- [x] 4. IPv4 Preference in IP Detection

  **What to do**:
  - Modify IP detection in coturn entrypoint to prefer IPv4
  - Change `curl -s https://api.ipify.org` to `curl -s -4 https://api.ipify.org`
  - Add fallback: try IPv4 first, then IPv6 if IPv4 fails
  - Also update `scripts/deploy-infrastructure.sh:82` with same pattern
  - Log warning when falling back to IPv6

  **Must NOT do**:
  - Do NOT check certificate expiry dates (just timer status)
  - Do NOT run certbot renew from this script
  - Do NOT modify certbot systemd configuration

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: New script file, well-defined purpose
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-2, 4-5)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - `deploy/health-check.sh:1-50` - Health check script pattern (colors, pass/fail functions)
  - `scripts/deploy-infrastructure.sh:125-162` - Certbot usage pattern

  **API/Type References**:
  - Systemd commands: `systemctl is-active`, `systemctl is-enabled`
  - Journalctl: `journalctl -u certbot.service --since`

  **WHY Each Reference Matters**:
  - health-check.sh shows the pattern for infrastructure checks
  - deploy-infrastructure.sh shows how certbot is invoked

  **Acceptance Criteria**:
  - [ ] `scripts/certbot-healthcheck.sh` exists and is executable
  - [ ] Script checks systemd timer status
  - [ ] Script exits 0 when timer active, 1 when inactive

  **QA Scenarios**:

  ```
  Scenario: Certbot healthcheck passes when timer is active
    Tool: Bash
    Preconditions: certbot.timer or similar exists and is active
    Steps:
      1. ./scripts/certbot-healthcheck.sh
      2. echo "Exit code: $?"
    Expected Result: Exit code is 0
    Failure Indicators: Exit code is 1
    Evidence: .sisyphus/evidence/task-03-certbot-pass.txt

  Scenario: Certbot healthcheck fails when timer inactive
    Tool: Bash
    Preconditions: certbot.timer stopped or non-existent
    Steps:
      1. sudo systemctl stop certbot.timer 2>/dev/null || true
      2. ./scripts/certbot-healthcheck.sh 2>&1 || true
      3. echo "Exit code: $?"
    Expected Result: Exit code is 1 (or 0 if no timer expected in dev)
    Failure Indicators: None (depends on environment)
    Evidence: .sisyphus/evidence/task-03-certbot-fail.txt
  ```

  **Evidence to Capture**:
  - [ ] Script execution output
  - [ ] Exit code verification

  **Commit**: YES
  - Message: `feat(healthcheck): add certbot systemd check script`
  - Files: `scripts/certbot-healthcheck.sh`
  - Pre-commit: `bash -n scripts/certbot-healthcheck.sh`

---

  - [x] 4. IPv4 Preference in IP Detection

  **What to do**:
  - Modify IP detection in coturn entrypoint to prefer IPv4
  - Change `curl -s https://api.ipify.org` to `curl -s -4 https://api.ipify.org`
  - Add fallback: try IPv4 first, then IPv6 if IPv4 fails
  - Also update `scripts/deploy-infrastructure.sh:82` with same pattern
  - Log warning when falling back to IPv6

  **Must NOT do**:
  - Do NOT remove IPv6 support entirely
  - Do NOT change IP detection logic structure
  - Do NOT add external dependencies

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single-line changes in 2 files
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-3, 5)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - `docker-compose.matrix.yml:38` - Current IP detection (needs IPv4 flag)

  **API/Type References**:
  - `curl -4` - Force IPv4
  - `curl -6` - Force IPv6 (for fallback)

  **WHY Each Reference Matters**:
  - Coturn entrypoint is where IP detection happens for TURN
  - deploy-infrastructure.sh also uses IP detection for DNS validation

  **Acceptance Criteria**:
  - [ ] IP detection uses `curl -4` for IPv4 preference
  - [ ] Fallback to IPv6 with warning when IPv4 unavailable
  - [ ] Both files updated consistently

  **QA Scenarios**:

  ```
  Scenario: IP detection returns IPv4 on dual-stack system
    Tool: Bash
    Preconditions: System has both IPv4 and IPv6
    Steps:
      1. curl -s -4 https://api.ipify.org
      2. echo ""  # newline
    Expected Result: Output is IPv4 format (N.N.N.N)
    Failure Indicators: Output contains colons (IPv6)
    Evidence: .sisyphus/evidence/task-04-ipv4-detection.txt

  Scenario: IP detection shows warning on IPv6 fallback
    Tool: Bash
    Preconditions: IPv4 not available (simulated)
    Steps:
      1. curl -s -4 --connect-timeout 2 https://api.ipify.org 2>&1 || echo "IPv4 failed, trying IPv6..."
      2. curl -s https://api.ipify.org
    Expected Result: Shows fallback message, then returns IP
    Failure Indicators: No fallback handling (hard failure)
    Evidence: .sisyphus/evidence/task-04-ipv6-fallback.txt
  ```

  **Evidence to Capture**:
  - [ ] curl output showing IPv4 address
  - [ ] Fallback behavior output

  **Commit**: YES
  - Message: `fix(network): prefer IPv4 in IP detection with IPv6 fallback`
  - Files: `docker-compose.matrix.yml`, `scripts/deploy-infrastructure.sh`
  - Pre-commit: `bash -n scripts/deploy-infrastructure.sh`

---

- [x] 5. Docker Build Error Visibility

  **What to do**:
  - Create `scripts/docker-build-with-errors.sh` wrapper script
  - Use `set -eo pipefail` to catch errors early
  - Add `--progress=plain` to docker build for full output
  - On failure: show last 50 lines of build log, exit with clear message
  - Support both `docker compose build` and `docker build` commands

  **Must NOT do**:
  - Do NOT add notification/alerting systems
  - Do NOT modify existing build scripts
  - Do NOT add build dashboards

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: New wrapper script, well-defined purpose
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-4)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - `deploy/installer-v4.sh` - abort() function pattern for error handling
  - `scripts/deploy-infrastructure.sh:7-8` - set -e pattern

  **API/Type References**:
  - Docker build flags: `--progress=plain`, `--no-cache`
  - Docker compose: `docker compose build --progress=plain`

  **WHY Each Reference Matters**:
  - installer shows error handling patterns
  - Docker --progress=plain shows full build output for debugging

  **Acceptance Criteria**:
  - [ ] `scripts/docker-build-with-errors.sh` exists and is executable
  - [ ] Script uses `--progress=plain` for visibility
  - [ ] On failure, shows context and clear error message

  **QA Scenarios**:

  ```
  Scenario: Build wrapper shows full output on success
    Tool: Bash
    Preconditions: Valid docker-compose.yml exists
    Steps:
      1. ./scripts/docker-build-with-errors.sh docker-compose-full.yml
      2. echo "Exit code: $?"
    Expected Result: Build output visible, exit code 0
    Failure Indicators: Silent output or wrong exit code
    Evidence: .sisyphus/evidence/task-05-build-success.txt

  Scenario: Build wrapper shows error context on failure
    Tool: Bash
    Preconditions: Dockerfile with intentional error
    Steps:
      1. echo "RUN exit 1" >> /tmp/bad.Dockerfile
      2. docker build -f /tmp/bad.Dockerfile . 2>&1 | head -20 || true
      3. rm /tmp/bad.Dockerfile
    Expected Result: Error message shows context
    Failure Indicators: Generic "build failed" without details
    Evidence: .sisyphus/evidence/task-05-build-failure.txt
  ```

  **Evidence to Capture**:
  - [ ] Build output with --progress=plain
  - [ ] Error context on failure

  **Commit**: YES
  - Message: `feat(build): add docker build wrapper with error visibility`
  - Files: `scripts/docker-build-with-errors.sh`
  - Pre-commit: `bash -n scripts/docker-build-with-errors.sh`

---

- [x] 6. Container Rollback Mechanism

  **What to do**:
  - Create `scripts/container-rollback.sh` script
  - Implement tag-based rollback: track `:current` and `:prev` tags
  - Before deploy: `docker tag service:current service:prev`
  - On rollback: `docker tag service:prev service:current && docker-compose up -d`
  - Support rollback for specific service or all services
  - Add `--dry-run` mode to preview changes
  - Log rollback actions to `/var/log/armorclaw/rollback.log`

  **Must NOT do**:
  - Do NOT rollback configs or volumes
  - Do NOT add rollback UI
  - Do NOT coordinate rollback across services (independent per container)
  - Do NOT rollback on healthcheck failure (only manual trigger)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multiple considerations (tag management, logging, safety checks)
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (depends on healthchecks from Wave 1)
  - **Blocks**: Final verification
  - **Blocked By**: Tasks 1-2 (healthchecks must be in place)

  **References**:
  **Pattern References**:
  - `deploy/health-check.sh:1-50` - Health check pattern for verifying rollback
  - `docker-compose-full.yml` - Service definitions for rollback targets

  **API/Type References**:
  - Docker commands: `docker tag`, `docker inspect`, `docker-compose up -d`
  - Service names: armorclaw-bridge, armorclaw-matrix, armorclaw-coturn

  **WHY Each Reference Matters**:
  - health-check.sh can verify services after rollback
  - docker-compose files define service names for rollback

  **Acceptance Criteria**:
  - [ ] `scripts/container-rollback.sh` exists and is executable
  - [ ] Script tracks :current and :prev image tags
  - [ ] `--dry-run` mode shows what would happen without executing
  - [ ] Rollback logs to /var/log/armorclaw/rollback.log

  **QA Scenarios**:

  ```
  Scenario: Rollback restores previous container version
    Tool: Bash
    Preconditions: Two image versions tagged (current, prev)
    Steps:
      1. docker tag armorclaw/bridge:0.3.0 armorclaw/bridge:prev
      2. docker tag armorclaw/bridge:0.3.1 armorclaw/bridge:current
      3. ./scripts/container-rollback.sh bridge --dry-run
      4. echo "Dry run exit code: $?"
    Expected Result: Shows "Would rollback bridge to armorclaw/bridge:prev"
    Failure Indicators: Shows wrong version or no output
    Evidence: .sisyphus/evidence/task-06-rollback-dryrun.txt

  Scenario: Rollback dry-run does not change running containers
    Tool: Bash
    Preconditions: Services running
    Steps:
      1. CURRENT=$(docker inspect --format='{{.Config.Image}}' armorclaw-bridge)
      2. ./scripts/container-rollback.sh bridge --dry-run
      3. AFTER=$(docker inspect --format='{{.Config.Image}}' armorclaw-bridge)
      4. [ "$CURRENT" = "$AFTER" ] && echo "No change (correct)" || echo "Changed (wrong)"
    Expected Result: "No change (correct)"
    Failure Indicators: Image changed during dry-run
    Evidence: .sisyphus/evidence/task-06-rollback-nochange.txt
  ```

  **Evidence to Capture**:
  - [ ] Dry-run output
  - [ ] Log file entry

  **Commit**: YES
  - Message: `feat(rollback): add container rollback mechanism`
  - Files: `scripts/container-rollback.sh`
  - Pre-commit: `bash -n scripts/container-rollback.sh`

---

## Final Verification Wave (MANDATORY)

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists. For each "Must NOT Have": search codebase for forbidden patterns. Check evidence files exist. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Infrastructure Health Review** — `unspecified-high`
  Run `deploy/health-check.sh` and verify all healthchecks report healthy. Run each QA scenario and capture evidence. Verify docker-compose config validates for all files.
  Output: `Healthchecks [N/N healthy] | Configs [valid/invalid] | QA [N/N pass] | VERDICT`

- [ ] F3. **Scope Fidelity Check** — `deep`
  For each task: verify only specified files were modified. Check no HTTP endpoint added to bridge binary. Check no TURN credentials created. Check rollback is containers-only.
  Output: `Files [N modified] | Scope Creep [CLEAN/N violations] | VERDICT`

---

## Commit Strategy

- **Task 1**: `feat(healthcheck): add bridge service healthcheck` — docker-compose-full.yml
- **Task 2**: `feat(healthcheck): add coturn service healthcheck` — docker-compose.matrix.yml
- **Task 3**: `feat(healthcheck): add certbot systemd check script` — scripts/certbot-healthcheck.sh
- **Task 4**: `fix(network): prefer IPv4 in IP detection` — docker-compose.matrix.yml, scripts/deploy-infrastructure.sh
- **Task 5**: `feat(build): add docker build wrapper with error visibility` — scripts/docker-build-with-errors.sh
- **Task 6**: `feat(rollback): add container rollback mechanism` — scripts/container-rollback.sh

---

## Success Criteria

### Verification Commands
```bash
# All healthchecks healthy
docker ps --format 'table {{.Names}}\t{{.Status}}' | grep -E '(healthy|running)'

# IPv4 detection
curl -s -4 https://api.ipify.org

# Build wrapper exists
ls -la scripts/docker-build-with-errors.sh

# Rollback script exists
ls -la scripts/container-rollback.sh
```

### Final Checklist
- [ ] All "Must Have" present (healthchecks, IPv4, rollback)
- [ ] All "Must NOT Have" absent (no HTTP endpoint, no TURN creds, no config rollback)
- [ ] All QA scenarios pass
- [ ] Evidence captured in .sisyphus/evidence/
