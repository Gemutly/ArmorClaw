# Fix: Smoke Test Docker Prerequisite Check

## TL;DR

> **Quick Summary**: Fix CI smoke test failing because quickstart checks for Docker daemon inside container.
> 
> **Deliverables**: 
> - Add `ARMORCLAW_SKIP_DOCKER_CHECK` environment variable support
> - Update smoke test to skip Docker check
> 
> **Estimated Effort**: Quick (2 files, ~10 lines)
> **Parallel Execution**: NO - sequential changes
> **Critical Path**: Direct fix

---

## Context

### Original Request
CI smoke test fails with:
```
✗ ERROR: Docker is installed but not running
ℹ Start with: systemctl start docker
```

### Root Cause Analysis
The quickstart entrypoint runs `setup-quick.sh` which calls `check_prerequisites()`. This function checks if Docker daemon is running using `docker info`. In Docker-in-Docker (DinD) environments like CI smoke tests, Docker CLI is installed but daemon isn't accessible.

### Current Flow
```
Dockerfile.quickstart ENTRYPOINT
  → /opt/armorclaw/entrypoint-wrapper.sh
    → /opt/armorclaw/quickstart.sh (setup-quick.sh)
      → check_prerequisites()
        → docker info check FAILS
          → exit 1
```

### Fix Approach
Add environment variable `ARMORCLAW_SKIP_DOCKER_CHECK=true` that:
1. Skips Docker daemon check in `check_prerequisites()`
2. Is set in the CI smoke test step

---

## Work Objectives

### Core Objective
Allow smoke test to run without requiring Docker daemon inside container.

### Concrete Deliverables
- Modified `deploy/setup-quick.sh` - support `ARMORCLAW_SKIP_DOCKER_CHECK` env var
- Modified `.github/workflows/dockerhub.yml` - set env var in smoke test

### Definition of Done
- [x] `check_prerequisites()` respects `ARMORCLAW_SKIP_DOCKER_CHECK` environment variable
- [x] Smoke test sets `ARMORCLAW_SKIP_DOCKER_CHECK=true`
- [ ] CI smoke test passes (pending push)

### Must Have
- Environment variable to skip Docker check
- Non-breaking change (default behavior unchanged)

### Must NOT Have (Guardrails)
- No changes to Dockerfile.quickstart
- No changes to other prerequisite checks
- No removal of Docker check (only skip when explicitly requested)

---

## TODOs

- [x] 1. Add ARMORCLAW_SKIP_DOCKER_CHECK support to setup-quick.sh

  **Status**: ✅ Complete
  - Commit: `6f812f4`
  - Added env var check before Docker daemon check
  - Default behavior unchanged

- [x] 2. Update smoke test to set ARMORCLAW_SKIP_DOCKER_CHECK

  **Status**: ✅ Complete
  - Commit: `6f812f4`
  - Added `-e ARMORCLAW_SKIP_DOCKER_CHECK=true` to docker run

---

## Final Verification Wave

- [x] F1. **Plan Compliance Audit**
  Verify changes match plan - env var support and usage.
  **Result**: ✅ APPROVE - Changes match plan exactly

- [ ] F2. **CI Verification**
  CI smoke test passes.
  **Status**: ⏳ Pending push - User must run `git push` to trigger CI

---

## Commit Strategy

- [x] **1**: `fix(ci): add ARMORCLAW_SKIP_DOCKER_CHECK for smoke tests` — deploy/setup-quick.sh, .github/workflows/dockerhub.yml
  **Commit**: `6f812f4`
  **Status**: Committed locally, awaiting push

---

## Success Criteria

### Verification Commands
```bash
# Verify setup-quick.sh has the env var check
grep -A5 "ARMORCLAW_SKIP_DOCKER_CHECK" deploy/setup-quick.sh

# Verify workflow has the env var
grep "ARMORCLAW_SKIP_DOCKER_CHECK" .github/workflows/dockerhub.yml

# Push to trigger CI
git push

# After push, check CI
gh run watch
```

### Final Checklist
- [x] setup-quick.sh updated
- [x] dockerhub.yml updated
- [x] Committed locally
- [ ] Pushed to remote (user action required)
- [ ] CI smoke test passes (pending push)
