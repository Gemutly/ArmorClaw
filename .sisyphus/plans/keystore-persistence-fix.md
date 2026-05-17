# Work Plan: Keystore Persistence Fix

> **Priority:** Critical | **Status:** Draft  
> **Estimated Effort:** Medium (3-5 days implementation)

## TL;DR

> **Goal:** Fix keystore persistence issue where API keys are lost on bridge restart in containerized deployments.
> **Root Cause:** Hardware-derived SQL instability in Docker containers.

---

## Context

### Problem Statement
API keys stored via RPC are lost after bridge restart. The keystore appears to be "wiped" but data is actually rendered inaccessible due to key mismatch.

### Root Cause Analysis
The `collectEntropy()` function in `bridge/pkg/keystore/keystore.go` (lines 185-226) derives the SQLCipher master key from hardware identifiers that are **unstable in Docker containers**:

| Entropy Source | Native Host | Docker Container |
|----------------|-------------|------------------|
| `/etc/machine-id` | Stable (OS install) | **Changes** (container regenerates) |
| DMI product UUID | Stable (SMBIOS) | **Not available** |
| MAC address | Stable | **Changes** (container recreate) |
| Hostname | Stable | **Container-specific** |

### Failure Mode
```
First Run:
  entropy (container A) → PBKDF2 → key A → SQLCipher encrypt → keystore.db

Container Restart/Recreate:
  entropy (container B) → PBKDF2 → key B ≠ key A
  SQLCipher with key B opens keystore.db → "file is not a database" or empty DB
  CREATE TABLE IF NOT EXISTS → succeeds, appears as fresh DB
  Old data is INVISIBLE (encrypted with key A, not deleted)
```

### Dangerous Code Path (main.go:1758-1794)
The "corruption recovery" logic triggers on SQLCipher errors:
- Interprets "file is not a database" as "wrong key" → deletes DB
- This destroys potentially recoverable data

### Related Bug
`keystore.go:769` uses undefined `ProviderZhipu` constant - needs to be added.

---

## Work Objectives

### Core Objective
Ensure keystore persists data across container restarts by replacing hardware-derived entropy with explicit secret sources for containerized deployments.

### Concrete Deliverables
1. `bridge/pkg/keystore/keystore.go` - Add explicit secret source hierarchy, container detection
2. `bridge/cmd/bridge/main.go` - Fix "corruption recovery" logic to not auto-wipe
3. `docker-compose-full.yml` - Add keystore secret environment variable
4. `docs/ACTIVE/review.md` - Document Phase 15

### Definition of Done
- [ ] Keys persist across container restarts
- [ ] Keys persist across container recreate
- [ ] Native host mode still works
- [ ] Key mismatch produces clear error, not auto-wipe

### Must Have
- Explicit secret source for containerized deployments (env var or file)
- Container detection (`.dockerenv`, cgroup)
- Auto-generation of key on first run in containers
- Clear error on key mismatch (no silent data loss)

### Must NOT Have (Guardrails)
- Auto-wipe of database on key mismatch
- Silent data loss
- Hardware-derived keys in containers

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (bun test, vitest)
- **Automated tests**: YES - Tests after implementation
- **Framework**: bun test

### QA Policy
Every task includes agent-executed QA scenarios (Playwright for UI, tmux for CLI, curl for API).

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — single file, foundational):
├── Task 1: Add ProviderZhipu constant [quick]

Wave 2 (After Wave 1 — core implementation):
├── Task 2: Add explicit secret source hierarchy to keystore.go [deep]
├── Task 3: Add container detection to keystore.go [quick]
├── Task 4: Fix corruption recovery in main.go [deep]

Wave 3 (After Wave 2 — integration):
├── Task 4: Add keystore secret to docker-compose-full.yml [quick]
├── Task 5: Add keystore key generation to Dockerfile.quickstart [quick]
└── Task 6: Update review.md with Phase 15 [writing]

Wave FINAL (After ALL tasks — verification):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Integration tests (deep)
└── Task F4: Scope fidelity check (deep)
```

---

## TODOs

- [ ] 1. Add ProviderZhipu constant

  **What to do**:
  Add `ProviderZhipu Provider = "zhipu"` to the constants block in `bridge/pkg/keystore/keystore.go` (line 62-68)
  
  **Files**: `bridge/pkg/keystore/keystore.go`

  **Recommended Agent**: `quick`

  **Parallelization**: Can start immediately (Wave 1)

  **Acceptance Criteria**:
  - [ ] Constant `ProviderZhipu` added to the Provider constants
  - [ ] `go build` succeeds without errors
  - [ ] `isValidProvider()` returns true for "zhipu"

  **QA Scenarios**:
  ```
  Scenario: Provider validation
    Tool: Bash (go test)
    Steps:
      1. cd bridge && go test -run TestIsValidProvider ./pkg/keystore/
    Expected Result: Test passes
    Evidence: .sisyphus/evidence/task-1-provider-validation.log
  ```

- [ ] 2. Add explicit secret source hierarchy to keystore

  **What to do**:
  Refactor `New()` in `bridge/pkg/keystore/keystore.go` to support explicit secret sources:
  1. Check `ARMORCLAW_KEYSTORE_SECRET` env var (base64-encoded 32 bytes)
  2. Check `keystore.key` file (persisted alongside db)
  3. Fall back to hardware-derived key ONLY for native host (not containers)
  4. Generate and persist new key if none found (first run in container)
  
  **Files**: `bridge/pkg/keystore/keystore.go`

  **Recommended Agent**: `deep`

  **Parallelization**: Wave 2 (after Task 1)

  **Must NOT do**:
  - Auto-wipe database on key mismatch
  - Silent data loss
  - Use hardware-derived keys in containers

  **References**:
  - `bridge/pkg/keystore/keystore.go:185-226` - `collectEntropy()` (understand what to replace)
  - `bridge/pkg/keystore/keystore.go:110-183` - `New()` and `deriveHardwareKey()` (modify)
  - `bridge/pkg/keystore/keystore.go:312-355` - `Open()` (ensure no auto-wipe)

  **Acceptance Criteria**:
  - [ ] Env var `ARMORCLAW_KEYSTORE_SECRET` is checked first
  - [ ] File `keystore.key` is checked second
  - [ ] Hardware-derived key only used when NOT in container
  - [ ] New key is generated and persisted if no other source available
  - [ ] `go test` passes

  **QA Scenarios**:
  ```
  Scenario: Env var key is used
    Tool: Bash (go test)
    Steps:
      1. Set ARMORCLAW_KEYSTORE_SECRET to a2. Run test with env var set
    Expected Result: Key is read from env var
    Evidence: .sisyphus/evidence/task-2-env-var-key.log

  Scenario: Key file is used
    Tool: Bash (go test)
    Steps:
      1. Create keystore.key file 2. Run test
    Expected Result: Key is read from file
    Evidence: .sisyphus/evidence/task-2-key-file.log

  Scenario: Key is generated and persisted
    Tool: Bash (go test)
    Steps:
      1. Run test with no env var or key file 2. Verify key file is created
    Expected Result: Key is generated and persisted to file
    Evidence: .sisyphus/evidence/task-2-key-generated.log
  ```

- [ ] 3. Add container detection to keystore

  **What to do**:
  Add `isRunningInContainer()` function to `bridge/pkg/keystore/keystore.go`:
  1. Check for `/.dockerenv` file existence
  2. Check `/proc/1/cgroup` for "docker" or "kubepods"
  
  **Files**: `bridge/pkg/keystore/keystore.go`

  **Recommended Agent**: `quick`

  **Parallelization**: Wave 2 (with Task 2)

  **Acceptance Criteria**:
  - [ ] Function `isRunningInContainer()` exists
  - [ ] Returns true when `/.dockerenv` exists
  - [ ] Returns true when "docker" or "kubepods" in cgroup
  - [ ] Returns false on native host

  **QA Scenarios**:
  ```
  Scenario: Detects Docker container
    Tool: Bash (go test)
    Steps:
      1. Create /.dockerenv 2. Run test
    Expected Result: isRunningInContainer() returns true
    Evidence: .sisyphus/evidence/task-3-docker-detection.log

  Scenario: Detects Kubernetes pod
    Tool: Bash (go test)
    Steps:
      1. Create /proc/1/cgroup with "kubepods" content 2. Run test
    Expected Result: isRunningInContainer() returns true
    Evidence: .sisyphus/evidence/task-3-k8s-detection.log

  Scenario: Native host not detected as container
    Tool: Bash (go test)
    Steps:
      1. Ensure no /.dockerenv or cgroup indicators 2. Run test
    Expected Result: isRunningInContainer() returns false
    Evidence: .sisyphus/evidence/task-3-native-host.log
  ```

- [ ] 4. Fix corruption recovery in main.go

  **What to do**:
  Modify `bridge/cmd/bridge/main.go` (lines 1758-1794) to:
  1. Detect key mismatch vs actual corruption
  2. On key mismatch: log clear error, do NOT auto-wipe
  3. Require manual intervention for key mismatch
  4. Only auto-recover for genuine file corruption
  
  **Files**: `bridge/cmd/bridge/main.go`

  **Recommended Agent**: `deep`

  **Parallelization**: Wave 2 (after Tasks 2,3)

  **Must NOT do**:
  - Auto-wipe database on key mismatch
  - Silent data loss

  **References**:
  - `bridge/cmd/bridge/main.go:1758-1794` - Current corruption recovery logic
  - `bridge/pkg/keystore/keystore.go` - New key derivation logic

  **Acceptance Criteria**:
  - [ ] Key mismatch produces clear error message
  - [ ] Key mismatch does NOT delete database
  - [ ] Bridge exits with non-zero code on key mismatch
  - [ ] Genuine corruption still triggers recovery

  **QA Scenarios**:
  ```
  Scenario: Key mismatch detected
    Tool: Bash (go test)
    Steps:
      1. Simulate key mismatch 2. Verify error is logged 3. Verify DB is not deleted
    Expected Result: Error logged, DB preserved
    Evidence: .sisyphus/evidence/task-4-key-mismatch.log

  Scenario: Genuine corruption recovery
    Tool: Bash (go test)
    Steps:
      1. Corrupt DB file 2. Run recovery
    Expected Result: DB is backed up and recreated
    Evidence: .sisyphus/evidence/task-4-corruption-recovery.log
  ```

- [ ] 5. Add keystore secret to docker-compose-full.yml

  **What to do**:
  Update `docker-compose-full.yml` to:
  1. Add `ARMORCLAW_KEYSTORE_SECRET` environment variable support
  2. Document how to generate and set the secret
  
  **Files**: `docker-compose-full.yml`

  **Recommended Agent**: `quick`

  **Parallelization**: Wave 3 (after Task 4)

  **Acceptance Criteria**:
  - [ ] Environment variable `ARMORCLAW_KEYSTORE_SECRET` is documented
  - [ ] Instructions for generating the secret are included
  - [ ] Secret can be passed via Docker secrets or env file

  **QA Scenarios**:
  ```
  Scenario: Secret can be passed to container
    Tool: Bash (docker compose)
    Steps:
      1. Set ARMORCLAW_KEYSTORE_SECRET 2. docker compose config 3. Verify env var is set
    Expected Result: Environment variable is visible in container config
    Evidence: .sisyphus/evidence/task-5-docker-secret-config.log
  ```

- [ ] 6. Update review.md with Phase 15

  **What to do**:
  Add Phase 15: Keystore Persistence Fix to `docs/ACTIVE/review.md`:
  1. Document root cause
  2. Document solution
  3. Document configuration options
  4. Add troubleshooting guide
  
  **Files**: `docs/ACTIVE/review.md`

  **Recommended Agent**: `writing`

  **Parallelization**: Wave 3 (with Tasks 4,5)

  **Acceptance Criteria**:
  - [ ] Phase 15 section added to review.md
  - [ ] Root cause documented
  - [ ] Solution documented
  - [ ] Configuration options documented
  - [ ] Troubleshooting guide included

---

## Final Verification Wave

- [ ] F1. Plan Compliance Audit (oracle)

  **What to do**: Verify all "Must Have" items are addressed in the plan. Check all file references exist. Verify no "Must NOT Have" items are violated.

- [ ] F2. Code Quality Review (unspecified-high)

  **What to do**: Run `go vet`, `go build`, and all tests. Check for any security issues.

- [ ] F3. Integration Tests (deep)

  **What to do**: Run full test suite including container restart tests, Verify keys persist across restarts.

- [ ] F4. Scope Fidelity Check (deep)

  **What to do**: Verify implementation matches plan. Check for scope creep or missing features.

---

## Commit Strategy

- **Task 1**: `fix(keystore): add ProviderZhipu constant`
- **Tasks 2-4**: `feat(keystore): add explicit secret sources and container detection`
- **Tasks 5-6**: `feat(deploy): add keystore secret support and docs`

---

## Success Criteria

### Verification Commands
```bash
# Verify ProviderZhipu constant
cd bridge && go build ./...

# Verify key persistence
docker compose restart armorclaw-bridge
# Check if keys still accessible

# Verify tests pass
cd bridge && go test ./pkg/keystore/... -v
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass
- [ ] Keys persist across container restart
