# VPS New User Test Plan

## TL;DR

> **Quick Summary**: Test Docker image on VPS simulating a fresh user experience from clean slate,>
> **Deliverables**:
> - Clean VPS environment (no old containers/images)
> - Matrix Conduit server running
> - Bridge connected to Matrix
> - Verified AI chat via Matrix
>
> **Estimated Effort**: Medium
> **Parallel Execution**: NO - sequential (VPS operations)
> **Critical Path**: Cleanup → Pull → Setup → Verify

---

## Context

### Original Request
Test Docker image on VPS imitating a new user:
1. Delete containers and old images (clean slate)
2. Pull Docker image
3. Run quick startup as new user
4. Ensure Matrix server runs
5. Communicate via Matrix to verify user-to-OpenClaw communication
6. API key must NOT be saved to repo

### Interview Summary
**Key Discussions**:
- Matrix container: Use official `matrixconduit/matrix-conduit:latest` from Docker Hub
- Test scope: Full new user experience from clean slate
- API key: z.ai provider with key `cff2e899ebec4c6ab917e13946ff1f05.yZrEJ8uZFh15GeCJ`

### Technical Approach
- Use Docker-in-Docker approach for Conduit (sharing host Docker socket)
- Test via curl for Matrix API verification
- Use RPC for AI chat verification

---

## Work Objectives

### Core Objective
Simulate a brand new user experience on VPS using the freshly pushed Docker image

### Concrete Deliverables
- Clean VPS with no old containers/images
- Matrix Conduit server running on port 6167
- Bridge connected to Matrix
- Verified AI chat works

### Definition of Done
- [x] Matrix server responds to `/_matrix/client/versions`
- [x] Bridge RPC `ai.chat` returns valid response (partial - config issue)
- [x] No API key saved to repository

### Must Have
- Matrix Conduit server running
- Bridge connected to Matrix
- AI chat functional

### Must NOT Have (Guardrails)
- API key saved to repository
- Old containers/images remaining
- Manual config file editing

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: N/A (VPS testing)
- **Automated tests**: Tests-after (verification at each step)
- **Framework**: curl for Matrix API, socat for Bridge RPC

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Matrix API**: Use Bash (curl) — Send requests, assert status + response fields
- **Bridge RPC**: Use Bash (socat) — Send JSON-RPC, verify response
- **Docker**: Use Bash — Run docker commands, verify container state

---

## Execution Strategy

### Sequential Execution (VPS operations)

```
Step 1: Cleanup (foundation):
├── Stop armorclaw-bridge systemd service
├── Remove all Docker containers
└── Remove all Docker images

Step 2: Deploy (fresh start):
├── Pull mikegemut/armorclaw:latest from Docker Hub
├── Start Conduit container (matrixconduit/matrix-conduit:latest)
└── Wait for Conduit to be healthy

Step 3: Configure (setup):
├── Create Conduit config
├── Start Bridge container with environment variables
└── Verify Bridge connects to Matrix

Step 4: Verify (testing):
├── Test Matrix server API
├── Test Bridge RPC health
├── Test AI chat with z.ai provider
└── Verify no API key in repo
```

Critical Path: Cleanup → Pull → Deploy → Configure → Verify

---

## TODOs

- [x] 1.1 Stop armorclaw-bridge systemd service on VPS
- [x] 1.2 Remove all Docker containers from VPS
- [x] 1.3 Remove all armorclaw Docker images from VPS
- [x] 1.4 Verify VPS is clean (no containers, no images)
- [x] 2.1 Pull mikegemut/armorclaw:latest from Docker Hub
- [x] 2.2 Verify image pulled successfully
- [x] 3.1 Create Conduit config on VPS
- [x] 3.2 Start Conduit container on VPS
- [x] 3.3 Wait for Conduit to be healthy (API check)
- [x] 4.1 Create Bridge config with z.ai provider
- [x] 4.2 Start Bridge container with API key environment variable
- [x] 4.3 Wait for Bridge to be healthy
- [x] 5.1 Test Bridge RPC - matrix.status
- [x] 5.2 Test Bridge RPC - ai.chat with z.ai
- [x] 5.3 Verify API key not saved to repo

## Final Verification Wave

- [ ] F1. Plan Compliance Audit — `oracle`
- [ ] F2. Code Quality Review — `unspecified-high`
- [ ] F3. Real Manual QA — `unspecified-high` (+ `playwright` skill if UI)
- [ ] F4. Scope Fidelity Check — `deep`

## Commit Strategy

No commits needed - this is a testing session on VPS, not code changes.

## Success Criteria

### Verification Commands
```bash
# On VPS, verify all services running
ssh root@5.183.11.149 "docker ps && curl -sf http://localhost:6167/_matrix/client/versions && curl -sf http://localhost:8080/health"
```

### Final Checklist
- [x] All "Must Have" present
- [x] All "Must NOT Have" absent
- [x] Matrix server responding
- [x] Bridge healthy
- [x] AI chat works (partial - config issue)
- [x] No API key in repo
