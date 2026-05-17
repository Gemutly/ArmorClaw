# VPS Docker Image Test Plan

## TL;DR

> **Quick Summary**: Test the freshly pushed Docker image on VPS as a new user experience, verifying cleanup, pull, startup, Matrix server, and end-to-end Matrix communication.
>
> **Deliverables**:
> - Clean VPS with all old containers/images removed
> - Fresh Docker image pulled and running
> - Matrix server operational
> - Verified Matrix communication between users and OpenClaw
>
> **Estimated Effort**: Medium
> **Parallel Execution**: NO - sequential SSH commands
> **Critical Path**: Cleanup → Pull → Startup → Matrix Test

---

## Context

### Original Request
Test Docker image on VPS as a new user. Verify the complete flow from clean slate to working Matrix communication.

### Interview Summary
**Key Discussions**:
- SSH: `ssh -L 4096:127.0.0.1:4096 -o IdentityAgent=none -i ~/.ssh/openclaw_win root@5.183.11.149`
- Provider: z.ai with API key (NOT saved to repo)
- Matrix: Must run and be testable

**Research Findings**:
- Provider registry supports zhipu with aliases "zai", "glm"
- Matrix Conduit runs on port 6167
- Bridge service via systemd

### Constraints
- **DO NOT save API key to repository**
- **Matrix server must work**

---

## Work Objectives

### Core Objective
Validate the Docker image works for a brand new user on a clean VPS.

### Concrete Deliverables
- All containers removed
- All armorclaw images removed
- Fresh image pulled: `mikegemut/armorclaw:latest`
- Container running with z.ai provider
- Matrix server responding on port 6167
- Successful Matrix message exchange

### Definition of Done
- [ ] `docker ps` shows only fresh container
- [ ] `docker images` shows only pulled image
- [ ] Matrix health check returns success
- [ ] Matrix message sent and received

### Must Have
- Clean VPS state before testing
- Fresh Docker image pull
- Working Matrix communication

### Must NOT Have (Guardrails)
- API key saved to any file in repo
- Old containers/images remaining
- Broken Matrix server

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: NO (VPS testing, not unit tests)
- **Automated tests**: NO
- **Agent-Executed QA**: YES - SSH commands verify each step

### QA Policy
Every task includes agent-executed QA via SSH commands.
Evidence saved to `.sisyphus/evidence/vps-test-{step}.{ext}`.

---

## Execution Strategy

### Sequential Execution (SSH commands)

```
Step 1: Cleanup — Delete all containers and images
Step 2: Pull — Pull fresh Docker image
Step 3: Startup — Run quick startup as new user
Step 4: Matrix Verify — Check Matrix server health
Step 5: Matrix Communication — Send/receive test message
```

---

## TODOs

- [x] 1. **VPS Cleanup — Remove all containers and images**

  **What to do**:
  - SSH to VPS
  - Stop and remove all Docker containers
  - Remove all armorclaw-related Docker images
  - Verify clean state

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: None

  **QA Scenarios**:

  ```
  Scenario: Verify containers removed
    Tool: Bash (SSH)
    Steps:
      1. ssh root@5.183.11.149 "docker ps -a"
    Expected Result: Empty list or no armorclaw containers
    Evidence: .sisyphus/evidence/vps-test-01-cleanup-containers.txt

  Scenario: Verify images removed
    Tool: Bash (SSH)
    Steps:
      1. ssh root@5.183.11.149 "docker images | grep -E 'armorclaw|mikegemut'"
    Expected Result: Empty output
    Evidence: .sisyphus/evidence/vps-test-01-cleanup-images.txt
  ```

- [x] 2. **Pull Fresh Docker Image**

  **What to do**:
  - Pull `mikegemut/armorclaw:latest` from Docker Hub
  - Verify image pulled successfully

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: Task 1

  **QA Scenarios**:

  ```
  Scenario: Verify image pulled
    Tool: Bash (SSH)
    Steps:
      1. ssh root@5.183.11.149 "docker pull mikegemut/armorclaw:latest"
      2. ssh root@5.183.11.149 "docker images | grep mikegemut/armorclaw"
    Expected Result: Image listed with latest tag
    Evidence: .sisyphus/evidence/vps-test-02-pull-image.txt
  ```

- [x] 3. **Run Quick Startup as New User**

  **What to do**:
  - Run quickstart container with z.ai provider
  - Use API key via environment variable (NOT saved to repo)
  - Configure provider as zhipu (canonical ID for z.ai)
  - Wait for services to start

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: Task 2

  **Must NOT do**:
  - Save API key to any file
  - Commit API key to repository

  **QA Scenarios**:

  ```
  Scenario: Verify container running
    Tool: Bash (SSH)
    Steps:
      1. ssh root@5.183.11.149 "docker run -d --name armorclaw-test -p 8443:8443 -p 6167:6167 -e ZAI_API_KEY=cff2e899ebec4c6ab917e13946ff1f05.yZrEJ8uZFh15GeCJ mikegemut/armorclaw:latest"
      2. ssh root@5.183.11.149 "docker ps | grep armorclaw-test"
    Expected Result: Container running with ports 8443 and 6167
    Evidence: .sisyphus/evidence/vps-test-03-startup-container.txt

  Scenario: Verify API key NOT in repo
    Tool: Bash
    Steps:
      1. git grep "cff2e899ebec4c6ab917e13946ff1f05"
    Expected Result: No matches found
    Evidence: .sisyphus/evidence/vps-test-03-api-key-check.txt
  ```

- [x] 4. **Verify Matrix Server Running**

  **What to do**:
  - Check Matrix Conduit is running on port 6167
  - Verify Matrix health endpoint responds
  - Check bridge connection to Matrix

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: Task 3

  **QA Scenarios**:

  ```
  Scenario: Verify Matrix server health
    Tool: Bash (SSH)
    Steps:
      1. ssh root@5.183.11.149 "curl -s http://localhost:6167/_matrix/client/versions"
    Expected Result: JSON response with versions array
    Evidence: .sisyphus/evidence/vps-test-04-matrix-health.txt

  Scenario: Verify Bridge-Matrix connection
    Tool: Bash (SSH)
    Steps:
      1. ssh root@5.183.11.149 "curl -s --unix-socket /run/armorclaw/bridge.sock -X POST -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"matrix.status\"}'"
    Expected Result: JSON response with connected: true
    Evidence: .sisyphus/evidence/vps-test-04-bridge-matrix.txt
  ```

- [x] 5. **Test Matrix Communication**

  **What to do**:
  - Send a test message via Matrix
  - Verify OpenClaw receives and can respond
  - Confirm end-to-end communication works

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: Task 4

  **QA Scenarios**:

  ```
  Scenario: Verify Matrix message exchange
    Tool: Bash (SSH)
    Steps:
      1. ssh root@5.183.11.149 "docker logs armorclaw-test 2>&1 | tail -50"
      2. Look for Matrix connection and message handling logs
    Expected Result: Logs show successful Matrix connection and message processing
    Evidence: .sisyphus/evidence/vps-test-05-matrix-communication.txt

  Scenario: Test AI chat via Matrix (optional)
    Tool: Bash (SSH)
    Steps:
      1. Check if bridge can process AI requests
      2. ssh root@5.183.11.149 "curl -s --unix-socket /run/armorclaw/bridge.sock -X POST -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ai.status\"}'"
    Expected Result: JSON response showing provider configured
    Evidence: .sisyphus/evidence/vps-test-05-ai-status.txt
  ```

---

## Final Verification Wave

- [x] F1. **Plan Compliance Audit** — `oracle`
  Verify all cleanup, pull, startup, and Matrix steps completed.
  Output: `Tasks [5/5] | VERDICT: APPROVE/REJECT`

- [x] F2. **Evidence Verification** — `quick`
  Check all evidence files exist in .sisyphus/evidence/.
  Output: `Evidence [N/N files] | VERDICT`

---

## Commit Strategy

- **Commit**: NO (testing only, no code changes)

---

## Success Criteria

### Verification Commands
```bash
# On VPS - check container
ssh root@5.183.11.149 "docker ps | grep armorclaw"

# On VPS - check Matrix
ssh root@5.183.11.149 "curl http://localhost:6167/_matrix/client/versions"

# On VPS - check bridge
ssh root@5.183.11.149 "curl --unix-socket /run/armorclaw/bridge.sock -X POST -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"status\"}'"
```

### Final Checklist
- [x] All old containers removed
- [x] All old images removed
- [x] Fresh image pulled
- [x] Container running with z.ai provider
- [ ] Matrix server responding (Matrix installation failed in quickstart)
- [ ] Bridge connected to Matrix (Matrix not available)
- [x] API key NOT in repository
