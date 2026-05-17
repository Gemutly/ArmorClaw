# Plan: Fix ArmorClaw VPS Deployment

## TL;DR

> **Quick Summary**: Fix nil interface checks causing bridge.status RPC panic, deploy to VPS, and verify health.
> 
> **Deliverables**:
> - Fixed bridge_handlers.go with isInterfaceNil() for all interface checks
> - Deployed and verified armorclaw-bridge binary on VPS
> - All health checks passing
> 
> **Estimated Effort**: Quick (5-10 min)
> **Parallel Execution**: NO - sequential deployment steps
> **Critical Path**: Fix code → Build → Deploy → Test

---

## Context

### Original Request
User requested VPS deployment with health check. Previous attempts revealed:
1. Port 8080 not listening (FIXED - added HTTP discovery server)
2. bridge.status RPC panic (NEEDS FIX - nil interface check)
3. Keystore key mismatch (FIXED - using static secret)

### Interview Summary
**Key Discussions**:
- Port 8080: Added HTTP discovery server that binds to port 8080
- Network conflicts: Fixed docker-compose.yml to use external matrix-net
- Nil pointer panic: bridge_handlers.go uses `== nil` for interface types

**Research Findings**:
- `isInterfaceNil()` helper exists in server.go (lines 266-275)
- 8 occurrences of incorrect nil checks found in bridge_handlers.go
- Build agent confirmed the fix pattern

### Metis Review
**Identified Gaps** (addressed):
- All interface nil checks must use isInterfaceNil() not == nil
- Both bridgeMgr and appService are interface types

---

## Work Objectives

### Core Objective
Fix all nil interface checks in bridge_handlers.go and deploy working ArmorClaw bridge to VPS.

### Concrete Deliverables
- `/home/mink/src/armorclaw-omo/bridge/pkg/rpc/bridge_handlers.go` - Fixed nil checks
- `/home/mink/src/armorclaw-omo/bridge/armorclaw-bridge` - Built binary
- VPS at 5.183.11.149 running with all health checks passing

### Definition of Done
- [ ] All 8 nil interface checks changed to isInterfaceNil()
- [ ] Binary builds without errors
- [ ] Binary deployed to VPS
- [ ] bridge.status RPC returns valid response
- [ ] HTTP discovery on port 8080 returns valid response
- [ ] Matrix connectivity verified

### Must Have
- Fix all nil interface checks (not just bridge.status)
- Deploy and restart service
- Verify with actual RPC calls

### Must NOT Have (Guardrails)
- Do not modify isInterfaceNil() function
- Do not skip building and deploying
- Do not proceed without verifying RPC works

---

## Verification Strategy (MANDATORY)

### Test Decision
- **Infrastructure exists**: NO (VPS deployment)
- **Automated tests**: NO
- **Agent-Executed QA**: YES - SSH commands to VPS

### QA Policy
Every task includes agent-executed QA scenarios via SSH to VPS.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Sequential - code fix):
├── Task 1: Fix all nil interface checks in bridge_handlers.go [quick]

Wave 2 (Sequential - build and deploy):
├── Task 2: Build bridge binary [quick]
├── Task 3: Deploy binary to VPS [quick]
├── Task 4: Restart bridge service [quick]

Wave 3 (Sequential - verification):
├── Task 5: Test bridge.status RPC [quick]
├── Task 6: Test HTTP discovery [quick]
├── Task 7: Test Matrix connectivity [quick]
└── Task 8: Run full health check [quick]

Critical Path: Task 1 → Task 2 → Task 3 → Task 4 → Tasks 5-8
```

---

## TODOs

- [ ] 1. Fix nil interface checks in bridge_handlers.go

  **What to do**:
  - Replace all `== nil` checks for `s.bridgeMgr` with `isInterfaceNil(s.bridgeMgr)`
  - Replace all `== nil` checks for `s.appService` with `isInterfaceNil(s.appService)`
  - Lines to fix: 16, 38, 62, 91, 143, 193, 210, 228

  **Must NOT do**:
  - Do not modify the isInterfaceNil function itself
  - Do not change any other logic

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (Wave 1)
  - **Blocks**: Tasks 2-8
  - **Blocked By**: None

  **References**:
  - `/home/mink/src/armorclaw-omo/bridge/pkg/rpc/bridge_handlers.go` - File to edit
  - `/home/mink/src/armorclaw-omo/bridge/pkg/rpc/server.go:266-275` - isInterfaceNil helper

  **Acceptance Criteria**:
  - [ ] All 8 occurrences changed from `== nil` to `isInterfaceNil()`
  - [ ] File compiles without errors

  **QA Scenarios**:
  ```
  Scenario: Build succeeds after fix
    Tool: Bash
    Steps:
      1. cd /home/mink/src/armorclaw-omo/bridge
      2. go build -o armorclaw-bridge ./cmd/bridge
    Expected Result: Binary created, no compile errors
    Evidence: armorclaw-bridge file exists
  ```

  **Commit**: YES
  - Message: `fix(rpc): use isInterfaceNil for interface nil checks`
  - Files: `bridge/pkg/rpc/bridge_handlers.go`

- [ ] 2. Build bridge binary

  **What to do**:
  - Build the Go binary with CGO enabled for SQLCipher
  - Verify binary is created and has correct size

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (Wave 2)
  - **Blocks**: Tasks 3-8
  - **Blocked By**: Task 1

  **References**:
  - `/home/mink/src/armorclaw-omo/bridge/` - Build directory

  **QA Scenarios**:
  ```
  Scenario: Binary builds successfully
    Tool: Bash
    Steps:
      1. cd /home/mink/src/armorclaw-omo/bridge
      2. CGO_ENABLED=1 go build -o armorclaw-bridge ./cmd/bridge
      3. ls -la armorclaw-bridge
    Expected Result: Binary ~35MB
    Evidence: armorclaw-bridge exists with size > 30MB
  ```

  **Commit**: NO

- [ ] 3. Deploy binary to VPS

  **What to do**:
  - Stop the armorclaw-bridge service
  - Copy new binary to /opt/armorclaw/armorclaw-bridge
  - Set executable permissions

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (Wave 2)
  - **Blocks**: Tasks 4-8
  - **Blocked By**: Task 2

  **References**:
  - VPS: 5.183.11.149
  - SSH: `ssh -o IdentityAgent=none -i ~/.ssh/openclaw_win root@5.183.11.149`

  **QA Scenarios**:
  ```
  Scenario: Binary deployed
    Tool: Bash
    Steps:
      1. Stop service: ssh root@5.183.11.149 "systemctl stop armorclaw-bridge"
      2. Remove old: ssh root@5.183.11.149 "rm -f /opt/armorclaw/armorclaw-bridge"
      3. Copy new: scp armorclaw-bridge root@5.183.11.149:/opt/armorclaw/
      4. Set perms: ssh root@5.183.11.149 "chmod +x /opt/armorclaw/armorclaw-bridge"
    Expected Result: Binary copied, service stopped
    Evidence: File exists on VPS
  ```

  **Commit**: NO

- [ ] 4. Restart bridge service

  **What to do**:
  - Start the armorclaw-bridge service
  - Wait for it to become active
  - Check service status

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (Wave 2)
  - **Blocks**: Tasks 5-8
  - **Blocked By**: Task 3

  **QA Scenarios**:
  ```
  Scenario: Service starts successfully
    Tool: Bash
    Steps:
      1. ssh root@5.183.11.149 "systemctl start armorclaw-bridge"
      2. sleep 3
      3. ssh root@5.183.11.149 "systemctl is-active armorclaw-bridge"
    Expected Result: "active"
    Evidence: Service status output
  ```

  **Commit**: NO

- [ ] 5. Test bridge.status RPC

  **What to do**:
  - Send JSON-RPC request to bridge socket
  - Verify response is valid JSON without errors

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (Wave 3)
  - **Blocks**: None
  - **Blocked By**: Task 4

  **QA Scenarios**:
  ```
  Scenario: bridge.status returns valid response
    Tool: Bash
    Steps:
      1. ssh root@5.183.11.149 "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"bridge.status\",\"params\":{}}' | nc -U /var/run/armorclaw/bridge.sock"
    Expected Result: {"enabled":false,"status":"not_configured"} or {"enabled":true,...}
    Evidence: Valid JSON response without error
  ```

  **Commit**: NO

- [ ] 6. Test HTTP discovery

  **What to do**:
  - Test port 8080 is listening
  - Test /api/discovery endpoint
  - Test /health endpoint

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (Wave 3)
  - **Blocks**: None
  - **Blocked By**: Task 4

  **QA Scenarios**:
  ```
  Scenario: HTTP discovery works
    Tool: Bash
    Steps:
      1. ssh root@5.183.11.149 "curl -s http://localhost:8080/api/discovery"
      2. ssh root@5.183.11.149 "curl -s http://localhost:8080/health"
    Expected Result: Valid JSON responses
    Evidence: Discovery info and health status
  ```

  **Commit**: NO

- [ ] 7. Test Matrix connectivity

  **What to do**:
  - Verify Matrix Conduit is running
  - Test Matrix client API endpoint

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (Wave 3)
  - **Blocks**: None
  - **Blocked By**: Task 4

  **QA Scenarios**:
  ```
  Scenario: Matrix is accessible
    Tool: Bash
    Steps:
      1. ssh root@5.183.11.149 "curl -s http://localhost:6167/_matrix/client/versions"
    Expected Result: {"versions":["r0.5.0",...,"v1.12"],...}
    Evidence: Matrix version response
  ```

  **Commit**: NO

- [ ] 8. Run full health check

  **What to do**:
  - Run comprehensive health check
  - Verify all services are healthy
  - Document final status

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (Wave 3)
  - **Blocks**: None
  - **Blocked By**: Tasks 5-7

  **QA Scenarios**:
  ```
  Scenario: Full health check passes
    Tool: Bash
    Steps:
      1. Check all ports: netstat -tlnp | grep -E '8080|6167|8448'
      2. Check all services: systemctl is-active armorclaw-bridge && docker ps
      3. Test all endpoints
    Expected Result: All green
    Evidence: Health check summary
  ```

  **Commit**: NO

---

## Final Verification Wave

- [ ] F1. Plan Compliance Audit
  Verify all nil interface checks fixed, binary deployed, services running.

- [ ] F2. Code Quality Review
  Run go build, verify no errors or warnings.

- [ ] F3. Real Manual QA
  Execute all QA scenarios on VPS.

- [ ] F4. Scope Fidelity Check
  Confirm only bridge_handlers.go was modified.

---

## Commit Strategy

- **Commit 1**: `fix(rpc): use isInterfaceNil for interface nil checks`
  - Files: `bridge/pkg/rpc/bridge_handlers.go`
  - Pre-commit: `go build ./...`

---

## Success Criteria

### Verification Commands
```bash
# Local build
cd /home/mink/src/armorclaw-omo/bridge && go build -o armorclaw-bridge ./cmd/bridge

# Deploy
scp -o IdentityAgent=none -i ~/.ssh/openclaw_win armorclaw-bridge root@5.183.11.149:/opt/armorclaw/

# Test
ssh -o IdentityAgent=none -i ~/.ssh/openclaw_win root@5.183.11.149 "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"bridge.status\"}' | nc -U /var/run/armorclaw/bridge.sock"
```

### Final Checklist
- [ ] All nil interface checks use isInterfaceNil()
- [ ] Binary builds successfully
- [ ] Binary deployed to VPS
- [ ] bridge.status RPC returns valid response
- [ ] HTTP discovery on port 8080 works
- [ ] Matrix connectivity verified
- [ ] All health checks passing
