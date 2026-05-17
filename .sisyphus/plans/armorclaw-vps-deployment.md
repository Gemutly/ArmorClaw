# ArmorClaw VPS Redeployment Plan

## TL;DR

> **Quick Summary**: Connect to VPS, backup critical data, clear existing Docker containers (preserve volumes), reinstall ArmorClaw using docker-compose-full.yml, verify health check passes HEALTHY, run minimal use case tests (Tests 1, 8, 9). If health check fails, apply fixes based on DEPLOYMENT_LESSONS.md.
> 
> **Deliverables**:
> - VPS cleared of old containers
> - ArmorClaw full stack running (Matrix, Bridge, Caddy, Sygnal)
> - Health check passing (HEALTHY status)
> - Core use cases validated (Secretary Commands, AI Provider, Agent Creation)
> 
> **Estimated Effort**: Medium (45-60 min)
> **Parallel Execution**: Limited (sequential deployment required)
> **Critical Path**: Backup → Cleanup → Deploy → Health Check → Tests

---

## Context

### Original Request
User wants to connect to VPS using stored environment variables (CONNECT_VPS, VPS_IP, AI_API_KEY, AI_PROVIDER), clear the VPS of blocking Docker containers/processes, reinstall ArmorClaw, run all use cases, perform health check, and fix any issues based on DEPLOYMENT_LESSONS.md.

### Interview Summary
**Key Discussions**:
- User has environment variables pre-configured for VPS access
- "Clear VPS" means docker-compose down (non-destructive, preserves volumes)
- "Run all use cases" means the 11 feature test categories from vps-testing-guide.md
- Health check must return HEALTHY status (not DEGRADED)
- Sentinel Ops involvement is consultative (reference DEPLOYMENT_LESSONS.md)

**Research Findings**:
- 4 deployment lessons: Socket Trap, Identity Firewall, SSH Tunneling, Hardware Unsealing
- Critical volumes: bridge_keystore, conduit_data, caddy_data, letsencrypt
- Network topology: matrix-net (172.20.0.0/24), bridge-net (172.21.0.0/24)
- Health check covers: Docker, Matrix, Bridge, Network, Security, HTTPS

### Metis Review
**Identified Gaps** (addressed):
- Missing rollback strategy → Added backup task before destructive actions
- Unclear "clear VPS" definition → Defined as docker-compose down (preserve volumes)
- Missing test scope → Defined as minimal (Tests 1, 8, 9) with optional expansion
- Missing deployment lesson verification → Added explicit verification tasks
- Edge cases not addressed → Added port conflict, disk space, and SSH timeout handling

---

## Work Objectives

### Core Objective
Redeploy ArmorClaw on VPS from clean state, verify all services healthy, validate core functionality.

### Concrete Deliverables
- SSH connection to VPS verified
- Existing Docker containers stopped (volumes preserved)
- Backup of critical data completed
- ArmorClaw full stack deployed via docker-compose-full.yml
- Health check returning HEALTHY status
- Core use cases passing (Tests 1, 8, 9)

### Definition of Done
- [ ] `docker ps` shows 4+ armorclaw containers running
- [ ] `./deploy/health-check.sh` exits with code 0, status HEALTHY
- [ ] `curl http://localhost:6167/_matrix/client/versions` returns valid JSON
- [ ] `curl http://localhost:8443/health` returns OK/healthy
- [ ] Test 1 (Secretary Commands) passes: `!secretary help` returns command list
- [ ] Test 8 (AI Provider) passes: `/ai providers` returns provider list
- [ ] Test 9 (Agent Creation) passes: Agent responds to request

### Must Have
- All 4 deployment lessons applied in docker-compose.yml
- Volume backup before any destructive action
- Health check must pass (not just run)
- SSH connectivity validated before proceeding

### Must NOT Have (Guardrails)
- MUST NOT destroy volumes without user confirmation
- MUST NOT proceed if health check returns UNHEALTHY
- MUST NOT skip deployment lesson verification
- MUST NOT use default/placeholder values in production configs
- MUST NOT expose ports without firewall rules

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (health-check.sh, test-e2e.sh)
- **Automated tests**: Tests-after (run after deployment)
- **Framework**: bash scripts + curl + Matrix commands
- **Agent-Executed QA**: ALWAYS (mandatory for all tasks)

### QA Policy
Every task includes agent-executed QA scenarios with evidence capture.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **VPS Operations**: Use Bash (ssh, docker commands)
- **API/Health Checks**: Use Bash (curl, jq)
- **Matrix Tests**: Use Bash (curl for API, Matrix commands for use cases)

---

## Execution Strategy

### Sequential Execution (Deployment Dependency Chain)

```
Phase 1 (Pre-deployment — validation & backup):
├── Task 1: Validate environment variables and SSH connection [quick]
├── Task 2: Check VPS prerequisites (disk, ports, Docker) [quick]
└── Task 3: Backup critical volumes [quick]

Phase 2 (Cleanup — stop existing containers):
├── Task 4: Stop all ArmorClaw containers [quick]
└── Task 5: Verify cleanup complete [quick]

Phase 3 (Deployment — install ArmorClaw):
├── Task 6: Clone/update ArmorClaw repository on VPS [quick]
├── Task 7: Configure environment variables [quick]
├── Task 8: Deploy full stack with docker-compose [unspecified-high]
└── Task 9: Wait for services healthy [quick]

Phase 4 (Verification — health check & lessons):
├── Task 10: Run comprehensive health check [quick]
├── Task 11: Verify deployment lessons applied [quick]
└── Task 12: If failures, analyze and apply fixes [deep]

Phase 5 (Use Case Testing — validate functionality):
├── Task 13: Test 1 - Secretary Commands [quick]
├── Task 14: Test 8 - AI Provider Commands [quick]
└── Task 15: Test 9 - Agent Creation [unspecified-high]

Phase FINAL (Review & Documentation):
├── Task F1: Plan compliance audit [oracle]
├── Task F2: Capture final evidence [unspecified-high]
└── Task F3: Present results to user [quick]
```

### Critical Path
Task 1 → Task 2 → Task 3 → Task 4 → Task 5 → Task 6 → Task 7 → Task 8 → Task 9 → Task 10 → Task 11 → (Task 12 if needed) → Task 13 → Task 14 → Task 15 → F1 → F2 → F3

---

## TODOs

- [ ] 1. Validate Environment Variables and SSH Connection

  **What to do**:
  - Verify all required environment variables are set (AI_API_KEY, CONNECT_VPS, VPS_IP, AI_PROVIDER)
  - Test SSH connection to VPS using CONNECT_VPS command
  - Verify Docker is accessible on VPS
  - Check user has sufficient permissions (docker group or sudo)

  **Must NOT do**:
  - Do not proceed if any environment variable is missing
  - Do not skip connection test before destructive operations

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple validation task with clear pass/fail criteria
  - **Skills**: []
    - No special skills needed for SSH/Docker validation

  **Parallelization**:
  - **Can Run In Parallel**: NO (foundation for all other tasks)
  - **Parallel Group**: Phase 1
  - **Blocks**: All subsequent tasks
  - **Blocked By**: None (can start immediately)

  **References**:
  - `DEPLOYMENT_LESSONS.md:11-13` - SSH Tunneling logic (WSL vs Linux)
  - `.env.example` - Required environment variables format

  **Acceptance Criteria**:
  - [ ] All environment variables set and non-empty
  - [ ] SSH connection succeeds with 10-second timeout
  - [ ] Docker daemon running on VPS
  - [ ] User can execute docker commands

  **QA Scenarios**:

  ```
  Scenario: Environment variables are set correctly
    Tool: Bash
    Preconditions: Shell environment loaded
    Steps:
      1. test -n "$AI_API_KEY" && echo "AI_API_KEY: SET" || echo "AI_API_KEY: MISSING"
      2. test -n "$CONNECT_VPS" && echo "CONNECT_VPS: SET" || echo "CONNECT_VPS: MISSING"
      3. test -n "$VPS_IP" && echo "VPS_IP: SET" || echo "VPS_IP: MISSING"
      4. test -n "$AI_PROVIDER" && echo "AI_PROVIDER: SET" || echo "AI_PROVIDER: MISSING"
    Expected Result: All 4 variables show "SET"
    Failure Indicators: Any variable shows "MISSING"
    Evidence: .sisyphus/evidence/task-01-env-validation.txt

  Scenario: SSH connection succeeds
    Tool: Bash
    Preconditions: CONNECT_VPS variable contains valid SSH command
    Steps:
      1. timeout 10 $CONNECT_VPS "echo 'SSH_CONNECTION_OK'" 2>&1
    Expected Result: Output contains "SSH_CONNECTION_OK"
    Failure Indicators: Timeout, permission denied, connection refused
    Evidence: .sisyphus/evidence/task-01-ssh-connection.txt
  ```

  **Commit**: NO

---

- [ ] 2. Check VPS Prerequisites

  **What to do**:
  - Check available disk space (require 5GB+ free)
  - Verify required ports are available (80, 443, 6167, 8443, 8448)
  - Check Docker version (24.0+)
  - Verify no conflicting services running

  **Must NOT do**:
  - Do not proceed if disk space < 2GB
  - Do not proceed if critical ports are blocked

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple system checks with standard commands
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 3 after Task 1 passes)
  - **Parallel Group**: Phase 1
  - **Blocks**: Phase 2 tasks
  - **Blocked By**: Task 1

  **References**:
  - `docs/guides/production-deployment.md` - System requirements
  - `docker-compose-full.yml:19-20` - Port mappings

  **Acceptance Criteria**:
  - [ ] Disk space >= 5GB free
  - [ ] Ports 80, 443, 6167, 8443, 8448 available
  - [ ] Docker version >= 24.0
  - [ ] No conflicting services on required ports

  **QA Scenarios**:

  ```
  Scenario: VPS has sufficient disk space
    Tool: Bash
    Preconditions: SSH connection established
    Steps:
      1. $CONNECT_VPS "df -h | grep -E '^/dev/' | head -1 | awk '{print \$4}'"
    Expected Result: Output shows >= 5G available
    Failure Indicators: Less than 5G available
    Evidence: .sisyphus/evidence/task-02-disk-space.txt

  Scenario: Required ports are available
    Tool: Bash
    Preconditions: SSH connection established
    Steps:
      1. $CONNECT_VPS "netstat -tlnp 2>/dev/null | grep -E ':(80|443|6167|8443|8448)' || echo 'ALL_PORTS_AVAILABLE'"
    Expected Result: Output shows "ALL_PORTS_AVAILABLE" or only armorclaw services
    Failure Indicators: Other services listening on required ports
    Evidence: .sisyphus/evidence/task-02-ports.txt
  ```

  **Commit**: NO

---

- [ ] 3. Backup Critical Volumes

  **What to do**:
  - List all existing armorclaw volumes
  - Create backup directory with timestamp
  - Export critical volumes (bridge_keystore, conduit_data, caddy_data, letsencrypt)
  - Export keystore database if accessible
  - Verify backup integrity

  **Must NOT do**:
  - Do not proceed to cleanup until backup verified
  - Do not skip any critical volume

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Standard Docker volume backup operations
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 2 after Task 1 passes)
  - **Parallel Group**: Phase 1
  - **Blocks**: Phase 2 tasks
  - **Blocked By**: Task 1

  **References**:
  - `docker-compose-full.yml:161-174` - Volume definitions
  - `DEPLOYMENT_LESSONS.md:15-17` - Hardware Unsealing (keystore backup)

  **Acceptance Criteria**:
  - [ ] All armorclaw volumes listed
  - [ ] Backup directory created with timestamp
  - [ ] Critical volumes exported to backup
  - [ ] Backup files exist and size > 0

  **QA Scenarios**:

  ```
  Scenario: Critical volumes backed up successfully
    Tool: Bash
    Preconditions: SSH connection established, Docker volumes exist
    Steps:
      1. $CONNECT_VPS "docker volume ls | grep armorclaw"
      2. $CONNECT_VPS "mkdir -p ~/armorclaw-backup-\$(date +%Y%m%d-%H%M%S)"
      3. $CONNECT_VPS "cd ~/armorclaw-backup-* && docker run --rm -v armorclaw_conduit_data:/data -v \$(pwd):/backup alpine tar czf /backup/conduit_data.tar.gz /data"
      4. $CONNECT_VPS "ls -lh ~/armorclaw-backup-*/"
    Expected Result: Backup files exist with size > 0
    Failure Indicators: Backup files missing or 0 bytes
    Evidence: .sisyphus/evidence/task-03-backup.txt
  ```

  **Commit**: NO

---

- [ ] 4. Stop All ArmorClaw Containers

  **What to do**:
  - List all running armorclaw containers
  - Stop containers using docker-compose down (preserves volumes)
  - Verify all containers stopped
  - Remove orphaned containers if any

  **Must NOT do**:
  - Do NOT use `docker-compose down -v` (destroys volumes)
  - Do not proceed until all containers verified stopped

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Standard Docker operations
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (must complete before Task 5)
  - **Parallel Group**: Phase 2
  - **Blocks**: Task 5
  - **Blocked By**: Tasks 1, 2, 3

  **References**:
  - `docker-compose-full.yml` - Service definitions
  - `deploy/deploy-all.sh` - Cleanup patterns

  **Acceptance Criteria**:
  - [ ] All armorclaw containers listed
  - [ ] docker-compose down executed successfully
  - [ ] No armorclaw containers running

  **QA Scenarios**:

  ```
  Scenario: All containers stopped successfully
    Tool: Bash
    Preconditions: Backup completed, SSH connection established
    Steps:
      1. $CONNECT_VPS "docker ps --filter 'name=armorclaw-' --format '{{.Names}}'"
      2. $CONNECT_VPS "cd /opt/armorclaw && docker compose down"
      3. $CONNECT_VPS "docker ps --filter 'name=armorclaw-' --format '{{.Names}}' | wc -l"
    Expected Result: Final count shows 0
    Failure Indicators: Count > 0 after down command
    Evidence: .sisyphus/evidence/task-04-cleanup.txt
  ```

  **Commit**: NO

---

- [ ] 5. Verify Cleanup Complete

  **What to do**:
  - Confirm no armorclaw containers running
  - Check for orphaned networks
  - Verify docker system is clean
  - Document current state before deployment

  **Must NOT do**:
  - Do not proceed if any armorclaw containers still running

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple verification
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (must complete before Phase 3)
  - **Parallel Group**: Phase 2
  - **Blocks**: Phase 3 tasks
  - **Blocked By**: Task 4

  **References**:
  - `docker-compose-full.yml:59-66` - Network definitions

  **Acceptance Criteria**:
  - [ ] Zero armorclaw containers running
  - [ ] Network state documented
  - [ ] Ready for fresh deployment

  **QA Scenarios**:

  ```
  Scenario: Cleanup verification passed
    Tool: Bash
    Preconditions: Task 4 completed
    Steps:
      1. $CONNECT_VPS "docker ps -a --filter 'name=armorclaw-' --format '{{.Names}}: {{.Status}}'"
      2. $CONNECT_VPS "docker network ls | grep armorclaw || echo 'NO_NETWORKS'"
    Expected Result: No running containers, networks may exist (will be reused)
    Failure Indicators: Running containers found
    Evidence: .sisyphus/evidence/task-05-verify-cleanup.txt
  ```

  **Commit**: NO

---

- [ ] 6. Clone/Update ArmorClaw Repository on VPS

  **What to do**:
  - Check if /opt/armorclaw exists
  - If exists, pull latest changes
  - If not exists, clone from GitHub
  - Verify repository integrity

  **Must NOT do**:
  - Do not proceed if clone/pull fails

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Standard git operations
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (must complete before Task 7)
  - **Parallel Group**: Phase 3
  - **Blocks**: Task 7
  - **Blocked By**: Task 5

  **References**:
  - `README.md:25-40` - Installation instructions
  - `deploy/install.sh` - Bootstrap installer

  **Acceptance Criteria**:
  - [ ] Repository exists at /opt/armorclaw
  - [ ] Latest code pulled/cloned
  - [ ] docker-compose-full.yml present

  **QA Scenarios**:

  ```
  Scenario: Repository ready for deployment
    Tool: Bash
    Preconditions: Cleanup complete
    Steps:
      1. $CONNECT_VPS "test -d /opt/armorclaw && cd /opt/armorclaw && git pull || git clone https://github.com/Gemutly/ArmorClaw.git /opt/armorclaw"
      2. $CONNECT_VPS "test -f /opt/armorclaw/docker-compose-full.yml && echo 'COMPOSE_FILE_OK'"
    Expected Result: Repository exists, compose file present
    Failure Indicators: Clone fails, compose file missing
    Evidence: .sisyphus/evidence/task-06-repo.txt
  ```

  **Commit**: NO

---

- [ ] 7. Configure Environment Variables

  **What to do**:
  - Create/update .env file with required variables
  - Set MATRIX_DOMAIN to VPS_IP or domain
  - Set AI_API_KEY and AI_PROVIDER from local environment
  - Generate ARMORCLAW_KEYSTORE_SECRET (32-byte base64)
  - Configure ARMORCLAW_CONTAINER_MODE=false (Lesson 2)
  - Configure ARMORCLAW_RPC_TYPE=tcp (Lesson 1)

  **Must NOT do**:
  - Do not use placeholder/default values
  - Do not skip ARMORCLAW_CONTAINER_MODE and ARMORCLAW_RPC_TYPE

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Configuration file creation
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (must complete before Task 8)
  - **Parallel Group**: Phase 3
  - **Blocks**: Task 8
  - **Blocked By**: Task 6

  **References**:
  - `DEPLOYMENT_LESSONS.md:4-6` - Socket Trap (ARMORCLAW_RPC_TYPE=tcp)
  - `DEPLOYMENT_LESSONS.md:8-9` - Identity Firewall (ARMORCLAW_CONTAINER_MODE=false)
  - `DEPLOYMENT_LESSONS.md:15-17` - Hardware Unsealing (ARMORCLAW_KEYSTORE_SECRET)
  - `.env.example` - Environment variable format

  **Acceptance Criteria**:
  - [ ] .env file created with all required variables
  - [ ] ARMORCLAW_RPC_TYPE=tcp set
  - [ ] ARMORCLAW_CONTAINER_MODE=false set
  - [ ] ARMORCLAW_KEYSTORE_SECRET generated (32-byte base64)
  - [ ] AI_API_KEY and AI_PROVIDER configured

  **QA Scenarios**:

  ```
  Scenario: Environment configured correctly
    Tool: Bash
    Preconditions: Repository cloned
    Steps:
      1. $CONNECT_VPS "cd /opt/armorclaw && cat > .env << EOF
MATRIX_DOMAIN=$VPS_IP
ARMORCLAW_API_KEY=$AI_API_KEY
ARMORCLAW_CONTAINER_MODE=false
ARMORCLAW_RPC_TYPE=tcp
ARMORCLAW_KEYSTORE_SECRET=\$(openssl rand -base64 32)
AI_PROVIDER=$AI_PROVIDER
EOF"
      2. $CONNECT_VPS "cd /opt/armorclaw && grep -E 'ARMORCLAW_RPC_TYPE=tcp|ARMORCLAW_CONTAINER_MODE=false' .env"
    Expected Result: Both configuration lines present
    Failure Indicators: Missing configuration lines
    Evidence: .sisyphus/evidence/task-07-config.txt
  ```

  **Commit**: NO

---

- [ ] 8. Deploy Full Stack with Docker Compose

  **What to do**:
  - Navigate to /opt/armorclaw
  - Run docker compose -f docker-compose-full.yml up -d
  - Monitor startup logs for errors
  - Wait for all services to start

  **Must NOT do**:
  - Do not proceed if docker compose fails
  - Do not skip log monitoring

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Critical deployment step requiring careful monitoring
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (must complete before Task 9)
  - **Parallel Group**: Phase 3
  - **Blocks**: Task 9
  - **Blocked By**: Task 7

  **References**:
  - `docker-compose-full.yml` - Full stack configuration
  - `DEPLOYMENT_LESSONS.md` - Deployment lessons to apply

  **Acceptance Criteria**:
  - [ ] docker compose up -d succeeds
  - [ ] All 4 services starting (matrix, sygnal, caddy, bridge)
  - [ ] No critical errors in logs

  **QA Scenarios**:

  ```
  Scenario: Full stack deployment succeeds
    Tool: Bash
    Preconditions: Environment configured
    Steps:
      1. $CONNECT_VPS "cd /opt/armorclaw && docker compose -f docker-compose-full.yml up -d"
      2. $CONNECT_VPS "docker ps --filter 'name=armorclaw-' --format '{{.Names}}: {{.Status}}'"
      3. $CONNECT_VPS "docker compose -f /opt/armorclaw/docker-compose-full.yml logs --tail=20"
    Expected Result: 4 containers running or starting
    Failure Indicators: Containers exited, error in logs
    Evidence: .sisyphus/evidence/task-08-deploy.txt
  ```

  **Commit**: NO

---

- [ ] 9. Wait for Services Healthy

  **What to do**:
  - Wait for Matrix healthcheck to pass (up to 60s)
  - Wait for Bridge to be ready (up to 60s)
  - Check all container health statuses
  - Monitor for startup errors

  **Must NOT do**:
  - Do not proceed if services unhealthy after timeout

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple wait and check
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (must complete before Phase 4)
  - **Parallel Group**: Phase 3
  - **Blocks**: Phase 4 tasks
  - **Blocked By**: Task 8

  **References**:
  - `docker-compose-full.yml:23-28` - Matrix healthcheck
  - `docker-compose-full.yml:128-133` - Bridge healthcheck

  **Acceptance Criteria**:
  - [ ] Matrix container healthy
  - [ ] Sygnal container running
  - [ ] Caddy container running
  - [ ] Bridge container healthy

  **QA Scenarios**:

  ```
  Scenario: All services healthy
    Tool: Bash
    Preconditions: Deployment started
    Steps:
      1. $CONNECT_VPS "for i in {1..12}; do docker inspect armorclaw-matrix --format='{{.State.Health.Status}}' 2>/dev/null | grep -q healthy && echo 'MATRIX_HEALTHY' && break; sleep 5; done"
      2. $CONNECT_VPS "docker ps --filter 'name=armorclaw-' --format '{{.Names}}: {{.Status}}'"
    Expected Result: Matrix healthy, all containers running
    Failure Indicators: Container unhealthy or exited
    Evidence: .sisyphus/evidence/task-09-healthy.txt
  ```

  **Commit**: NO

---

- [ ] 10. Run Comprehensive Health Check

  **What to do**:
  - Execute deploy/health-check.sh --verbose
  - Capture all check results
  - Parse PASS/FAIL/WARN counts
  - Determine overall status (HEALTHY/DEGRADED/UNHEALTHY)

  **Must NOT do**:
  - Do not proceed to use case tests if UNHEALTHY

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Execute existing health check script
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (must complete before Task 11)
  - **Parallel Group**: Phase 4
  - **Blocks**: Task 11
  - **Blocked By**: Task 9

  **References**:
  - `deploy/health-check.sh` - Comprehensive health check script
  - `DEPLOYMENT_LESSONS.md` - Lessons to verify against

  **Acceptance Criteria**:
  - [ ] Health check script executes
  - [ ] Status is HEALTHY (exit 0, no FAILs)
  - [ ] All critical checks pass

  **QA Scenarios**:

  ```
  Scenario: Health check passes
    Tool: Bash
    Preconditions: All services running
    Steps:
      1. $CONNECT_VPS "cd /opt/armorclaw && ./deploy/health-check.sh --verbose"
      2. Capture exit code and status
    Expected Result: Exit code 0, status HEALTHY
    Failure Indicators: Exit code 1 (UNHEALTHY) or DEGRADED with critical warnings
    Evidence: .sisyphus/evidence/task-10-health-check.txt
  ```

  **Commit**: NO

---

- [ ] 11. Verify Deployment Lessons Applied

  **What to do**:
  - Verify Lesson 1 (Socket Trap): Check logs for "RPC transport: tcp"
  - Verify Lesson 2 (Identity Firewall): Check ARMORCLAW_CONTAINER_MODE=false
  - Verify Lesson 4 (Hardware Unsealing): Check keystore secret length
  - Document any lesson violations

  **Must NOT do**:
  - Do not skip lesson verification

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Verify configuration against known lessons
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (must complete before Task 12)
  - **Parallel Group**: Phase 4
  - **Blocks**: Task 12
  - **Blocked By**: Task 10

  **References**:
  - `DEPLOYMENT_LESSONS.md:4-17` - All 4 deployment lessons
  - `docker-compose.yml:34-38` - Expected configuration

  **Acceptance Criteria**:
  - [ ] Lesson 1 verified: TCP socket in use
  - [ ] Lesson 2 verified: Container mode disabled
  - [ ] Lesson 4 verified: Keystore secret valid

  **QA Scenarios**:

  ```
  Scenario: All deployment lessons applied
    Tool: Bash
    Preconditions: Services running
    Steps:
      1. $CONNECT_VPS "docker logs \$(docker ps -q --filter 'name=armorclaw-bridge') 2>&1 | grep -i 'RPC transport' || echo 'LESSON_1_NOT_VERIFIED'"
      2. $CONNECT_VPS "docker exec \$(docker ps -q --filter 'name=armorclaw-bridge') env | grep 'ARMORCLAW_CONTAINER_MODE=false' || echo 'LESSON_2_NOT_VERIFIED'"
      3. $CONNECT_VPS "docker exec \$(docker ps -q --filter 'name=armorclaw-bridge') env | grep ARMORCLAW_KEYSTORE_SECRET | wc -c"
    Expected Result: All lessons verified
    Failure Indicators: Any lesson shows NOT_VERIFIED
    Evidence: .sisyphus/evidence/task-11-lessons.txt
  ```

  **Commit**: NO

---

- [ ] 12. Analyze and Apply Fixes (If Health Check Failed)

  **What to do**:
  - If health check failed, analyze error output
  - Cross-reference failures with DEPLOYMENT_LESSONS.md
  - Apply appropriate fixes based on lesson violations
  - Re-run health check after fix
  - Iterate until HEALTHY or user aborts

  **Must NOT do**:
  - Do not apply fixes blindly without understanding root cause
  - Do not skip user confirmation for destructive fixes

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Requires deep understanding of deployment issues
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (conditional task)
  - **Parallel Group**: Phase 4
  - **Blocks**: Phase 5 tasks
  - **Blocked By**: Tasks 10, 11

  **References**:
  - `DEPLOYMENT_LESSONS.md` - All lessons and solutions
  - `docker-compose-full.yml` - Configuration to fix
  - `docs/guides/troubleshooting.md` - Troubleshooting guide

  **Acceptance Criteria**:
  - [ ] Root cause identified
  - [ ] Fix applied based on DEPLOYMENT_LESSONS.md
  - [ ] Health check re-run shows improvement
  - [ ] Final status is HEALTHY or user accepts DEGRADED

  **QA Scenarios**:

  ```
  Scenario: Fix applied and verified
    Tool: Bash
    Preconditions: Health check failed
    Steps:
      1. Analyze failure output from Task 10
      2. Identify which lesson was violated
      3. Apply fix (e.g., add ARMORCLAW_RPC_TYPE=tcp to .env)
      4. Restart affected service
      5. Re-run health check
    Expected Result: Health check now passes or shows improvement
    Failure Indicators: Fix does not resolve issue
    Evidence: .sisyphus/evidence/task-12-fix.txt
  ```

  **Commit**: NO

---

- [ ] 13. Test 1 - Secretary Commands

  **What to do**:
  - Connect to Matrix room (via Element X or API)
  - Send `!secretary help` command
  - Verify response contains command list
  - Test basic secretary functionality

  **Must NOT do**:
  - Do not proceed if Matrix connection fails

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple command test
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (requires healthy deployment)
  - **Parallel Group**: Phase 5
  - **Blocks**: Task 14
  - **Blocked By**: Tasks 10-12 (health verified)

  **References**:
  - `docs/ACTIVE/vps-testing-guide.md` - Test 1 specification
  - `README.md:47-53` - First task example

  **Acceptance Criteria**:
  - [ ] Matrix room accessible
  - [ ] `!secretary help` returns command list
  - [ ] Secretary bot responds correctly

  **QA Scenarios**:

  ```
  Scenario: Secretary commands work
    Tool: Bash (curl for Matrix API)
    Preconditions: Deployment healthy, Matrix accessible
    Steps:
      1. Get Matrix access token (from admin login)
      2. Send `!secretary help` to agents room via Matrix API
      3. Wait for response
      4. Verify response contains expected commands
    Expected Result: Response lists available secretary commands
    Failure Indicators: No response or error message
    Evidence: .sisyphus/evidence/task-13-secretary.txt
  ```

  **Commit**: NO

---

- [ ] 14. Test 8 - AI Provider Commands

  **What to do**:
  - Send `/ai providers` command
  - Verify response lists available AI providers
  - Send `/ai status` command
  - Verify current provider configuration

  **Must NOT do**:
  - Do not proceed if AI_API_KEY is invalid

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple command test
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential with Test 1)
  - **Parallel Group**: Phase 5
  - **Blocks**: Task 15
  - **Blocked By**: Task 13

  **References**:
  - `docs/ACTIVE/vps-testing-guide.md` - Test 8 specification
  - `README.md:122-140` - AI provider commands

  **Acceptance Criteria**:
  - [ ] `/ai providers` returns provider list
  - [ ] Configured AI_PROVIDER appears in list
  - [ ] `/ai status` shows current configuration

  **QA Scenarios**:

  ```
  Scenario: AI provider commands work
    Tool: Bash (Matrix API)
    Preconditions: Secretary commands work, AI_API_KEY set
    Steps:
      1. Send `/ai providers` to Matrix room
      2. Verify response lists 10+ providers
      3. Send `/ai status` to Matrix room
      4. Verify response shows configured provider
    Expected Result: Provider list returned, status shows current config
    Failure Indicators: Empty list or error
    Evidence: .sisyphus/evidence/task-14-ai-provider.txt
  ```

  **Commit**: NO

---

- [ ] 15. Test 9 - Agent Creation

  **What to do**:
  - Send `!agent create name="TestAgent" skills="web_browsing"` command
  - Verify agent room is created
  - Send test request to agent
  - Verify agent responds

  **Must NOT do**:
  - Do not proceed if AI provider is not configured

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Core functionality test requiring AI integration
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (final test in sequence)
  - **Parallel Group**: Phase 5
  - **Blocks**: Phase FINAL
  - **Blocked By**: Task 14

  **References**:
  - `docs/ACTIVE/vps-testing-guide.md` - Test 9 specification
  - `README.md:47-53` - Agent creation example

  **Acceptance Criteria**:
  - [ ] Agent created successfully
  - [ ] Agent room created and accessible
  - [ ] Agent responds to request

  **QA Scenarios**:

  ```
  Scenario: Agent creation and interaction works
    Tool: Bash (Matrix API)
    Preconditions: AI provider working
    Steps:
      1. Send `!agent create name="TestAgent" skills="web_browsing"` to agents room
      2. Wait for agent room creation notification
      3. Join agent room
      4. Send test message: "What is the capital of France?"
      5. Wait for agent response
    Expected Result: Agent room created, agent responds with answer
    Failure Indicators: Room not created, no response, or error
    Evidence: .sisyphus/evidence/task-15-agent.txt
  ```

  **Commit**: NO

---

## Final Verification Wave

- [ ] F1. Plan Compliance Audit — `oracle`
  Verify all "Must Have" requirements met, all "Must NOT Have" avoided, evidence files exist.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Evidence [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. Capture Final Evidence — `unspecified-high`
  Collect all evidence files, create summary report, document any issues encountered.

- [ ] F3. Present Results to User — `quick`
  Summarize deployment status, health check results, test outcomes, and any issues.

---

## Commit Strategy

No commits - this is a deployment plan, not code changes.

---

## Success Criteria

### Verification Commands
```bash
# Container status
$CONNECT_VPS "docker ps --filter 'name=armorclaw-' --format '{{.Names}}: {{.Status}}'"
# Expected: 4+ containers running

# Health check
$CONNECT_VPS "cd /opt/armorclaw && ./deploy/health-check.sh"
# Expected: Exit code 0, Status: HEALTHY

# Matrix API
$CONNECT_VPS "curl -s http://localhost:6167/_matrix/client/versions"
# Expected: JSON with "versions" array

# Bridge health
$CONNECT_VPS "curl -s http://localhost:8443/health"
# Expected: OK or healthy response
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] Health check passes
- [ ] Core use cases validated
- [ ] Evidence captured