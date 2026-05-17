# Test Coverage & Operational Hardening Plan

## TL;DR

> **Quick Summary**: Close all missing test coverage gaps and implement operational hardening for production readiness.
> 
> **Deliverables**:
> - E2E tests for all 11 user stories
> - Docker sidecars for voice E2E tests
> - Health monitoring and automated backups
> - Install-to-first-agent automation verification
> - Post-setup security hygiene automation
> 
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 4 waves
> **Critical Path**: Test Infrastructure → Integration Tests → E2E Tests → Ops Hardening

---

## Context

### Original Request
The review.md flags missing test coverage for:
- Installation (US-1)
- API Key Setup (US-2)
- Admin User Creation (US-3)
- Calendar (US-7)
- WebDAV (US-8)
- Contacts/Rolodex (US-9)
- Mobile App Connection (US-10)
- Three-Way Consent (US-11)
- Voice E2E (needs Docker sidecars)
- PII Web-Form Flow (US-6)

Additional operational gaps:
- Health endpoint / health monitoring
- Automated backups
- Log rotation
- Monitoring / metrics / alerting
- Zero-trust CLIENT_SIDE encryption path
- Post-setup security hygiene (admin password cleanup)
- Install-to-first-agent flow verification

### Current State

| Category | Status | Notes |
|----------|--------|-------|
| Voice Unit Tests | ✅ EXISTS | Mock clients in bridge/pkg/voice/*_test.go |
| Voice E2E Tests | ⚠️ EXISTS BUT SKIPS | Needs ARMORCLAW_E2E=1 + Docker sidecars |
| PII/Browser Tests | ✅ EXISTS | blindfill_e2e_test.go, pii_shadow_e2e_test.go |
| Integration Tests | ✅ PARTIAL | test-rpc-methods.sh, test-e2e.sh |
| Docker Sidecars | ✅ PARTIAL | tests/matrix-test-server/docker-compose.yml |

### Research Findings

**Existing Infrastructure:**
- GitHub Actions CI with multiple test jobs
- Docker Compose for Matrix test server (Conduit, Coturn, Nginx)
- Voice E2E expects services at localhost:8001/8002/8003
- Makefile with test targets
- Test scripts in tests/ directory

**Missing Components:**
- tests/docker-compose.voice.yml for voice sidecars
- E2E test scripts for US-1, US-2, US-3, US-7, US-8, US-9, US-10, US-11
- Health monitoring daemon
- Backup automation
- Log rotation config
- Metrics export

---

## Work Objectives

### Core Objective
Achieve production-ready test coverage and operational hardening for ArmorClaw.

### Concrete Deliverables
1. **Test Infrastructure**: Docker sidecars for voice E2E
2. **E2E Tests**: Automated tests for all 11 user stories
3. **Health Monitoring**: /health endpoint + Prometheus metrics
4. **Backup System**: Automated daily backups with retention
5. **Log Rotation**: Structured logging with rotation
6. **Security Hygiene**: Auto-cleanup of sensitive files post-setup
7. **Verification Flow**: Install → Matrix login → !status → first agent → AI response

### Definition of Done
- [ ] All 11 user stories have E2E tests
- [ ] Voice E2E tests run in CI with Docker sidecars
- [ ] Health monitoring returns structured status
- [ ] Backups run daily with 7-day retention
- [ ] Logs rotate at 100MB with 10-file retention
- [ ] Admin password file auto-removed after first login
- [ ] Install-to-first-agent flow verified by CI

### Must Have
- Voice E2E tests with real sidecar services
- Health monitoring endpoint
- Automated backups
- Log rotation
- Post-setup security cleanup

### Must NOT Have (Guardrails)
- No external service dependencies in unit tests
- No production secrets in test configs
- No breaking changes to existing tests
- No heavy LLM services in CI sidecars
- No automatic rollback on backup failure

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES
- **Automated tests**: TDD for new test infrastructure, tests-after for E2E
- **Framework**: Go testing + bash scripts + Docker Compose

### QA Policy
Every task includes agent-executed QA scenarios:
- **Voice Sidecars**: Docker commands to start/stop/verify health
- **E2E Tests**: Script execution with assertion checks
- **Health Monitoring**: curl /health endpoint, verify Prometheus format
- **Backups**: Create backup, verify file exists, test restore
- **Log Rotation**: Generate logs, trigger rotation, verify file count

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — infrastructure + scaffolding):
├── Task 1: Create Docker sidecars for voice E2E [quick]
├── Task 2: Create test script scaffolding [quick]
├── Task 3: Health monitoring endpoint [unspecified-high]
├── Task 4: Prometheus metrics export [unspecified-high]
├── Task 5: Log rotation configuration [quick]
└── Task 6: Backup script foundation [unspecified-high]

Wave 2 (After Wave 1 — E2E tests for user stories):
├── Task 7: US-1 Installation E2E test [unspecified-high]
├── Task 8: US-2 API Key Setup E2E test [unspecified-high]
├── Task 9: US-3 Admin/Bridge User E2E test [unspecified-high]
├── Task 10: US-7 Calendar E2E test [unspecified-high]
├── Task 11: US-8 WebDAV E2E test [unspecified-high]
└── Task 12: US-9 Contacts E2E test [unspecified-high]

Wave 3 (After Wave 2 — advanced tests + ops):
├── Task 13: US-10 Mobile App Connection E2E test [unspecified-high]
├── Task 14: US-11 Three-Way Consent E2E test [unspecified-high]
├── Task 15: Voice E2E CI integration [unspecified-high]
├── Task 16: Post-setup security hygiene automation [unspecified-high]
├── Task 17: Install-to-first-agent verification script [deep]
└── Task 18: CI workflow update for all E2E tests [unspecified-high]

Wave FINAL (After ALL tasks — verification):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Test coverage verification (unspecified-high)
├── Task F3: Ops hardening verification (unspecified-high)
└── Task F4: Security hygiene check (deep)
-> Present results -> Get explicit user okay

Critical Path: Task 1 → Task 15 → Task 18 → F1-F4 → user okay
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 6 (Wave 1)
```

---

## TODOs

- [ ] 1. Create Docker Sidecars for Voice E2E

  **What to do**:
  - Create `tests/docker-compose.voice.yml` with VAD, STT, TTS services
  - Use lightweight images (whisper.cpp, vosk, piper)
  - Add health check endpoints to each service
  - Create `tests/config/voice-test.toml` for bridge config

  **Must NOT do**:
  - Do NOT use heavy LLM-based services
  - Do NOT expose ports to host except for bridge access
  - Do NOT include in production compose files

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Standard Docker Compose configuration following existing patterns
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2-6)
  - **Blocks**: Task 15 (Voice E2E CI integration)
  - **Blocked By**: None (can start immediately)

  **References**:
  - Pattern: `tests/matrix-test-server/docker-compose.yml` - Sidecar pattern with health checks
  - API: `bridge/pkg/voice/e2e_test.go` - Expected service URLs (localhost:8001/8002/8003)
  - Config: `tests/config/load-test.toml` - Test configuration pattern

  **Acceptance Criteria**:
  - [ ] tests/docker-compose.voice.yml exists with VAD, STT, TTS services
  - [ ] Each service has health check endpoint
  - [ ] `docker-compose -f tests/docker-compose.voice.yml up -d` succeeds
  - [ ] All services report healthy within 30s

  **QA Scenarios**:
  ```
  Scenario: Voice sidecars start and become healthy
    Tool: Bash (docker-compose)
    Preconditions: Docker daemon running
    Steps:
      1. docker-compose -f tests/docker-compose.voice.yml up -d
      2. sleep 30
      3. curl -f http://localhost:8001/health
      4. curl -f http://localhost:8002/health
      5. curl -f http://localhost:8003/health
    Expected Result: All health checks return 200 OK
    Failure Indicators: Any curl fails with non-200 or connection refused
    Evidence: .sisyphus/evidence/task-1-voice-sidecars.txt

  Scenario: Voice sidecars cleanup properly
    Tool: Bash (docker-compose)
    Preconditions: Sidecars running
    Steps:
      1. docker-compose -f tests/docker-compose.voice.yml down
      2. docker ps --filter "name=armorclaw" --format "{{.Names}}"
    Expected Result: No armorclaw-voice containers running
    Failure Indicators: Containers still listed
    Evidence: .sisyphus/evidence/task-1-cleanup.txt
  ```

  **Commit**: YES
  - Message: `feat(test): add Docker sidecars for voice E2E tests`
  - Files: tests/docker-compose.voice.yml, tests/config/voice-test.toml

---

- [ ] 2. Create Test Script Scaffolding

  **What to do**:
  - Create `tests/e2e/` directory for E2E test scripts
  - Create template script `tests/e2e/template.sh` with common patterns
  - Create `tests/e2e/common.sh` with shared functions (start_bridge, stop_bridge, wait_for_matrix)
  - Add test configuration files

  **Must NOT do**:
  - Do NOT duplicate existing test patterns
  - Do NOT create tests for already-covered user stories (US-4, US-5, US-6)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Standard shell script scaffolding
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3-6)
  - **Blocks**: Tasks 7-14 (all E2E test tasks)
  - **Blocked By**: None

  **References**:
  - Pattern: `tests/test-e2e.sh` - Existing E2E test structure
  - Pattern: `tests/test-rpc-methods.sh` - RPC test patterns
  - Common: `tests/matrix-test-server/` - Test server setup

  **Acceptance Criteria**:
  - [ ] tests/e2e/ directory exists
  - [ ] tests/e2e/common.sh with start_bridge, stop_bridge, wait_for_matrix functions
  - [ ] tests/e2e/template.sh with placeholder test cases
  - [ ] All scripts have proper error handling and cleanup

  **QA Scenarios**:
  ```
  Scenario: Common functions load successfully
    Tool: Bash
    Preconditions: tests/e2e/common.sh exists
    Steps:
      1. source tests/e2e/common.sh
      2. type start_bridge
      3. type stop_bridge
      4. type wait_for_matrix
    Expected Result: All functions defined
    Failure Indicators: "type: start_bridge: not found"
    Evidence: .sisyphus/evidence/task-2-common-functions.txt
  ```

  **Commit**: YES
  - Message: `feat(test): add E2E test scaffolding and common functions`
  - Files: tests/e2e/common.sh, tests/e2e/template.sh

---

- [ ] 3. Health Monitoring Endpoint

  **What to do**:
  - Add `/health` HTTP endpoint to bridge RPC server
  - Return JSON with status of: bridge, matrix, keystore, browser-service
  - Add `health.check` RPC method for programmatic access
  - Create health check script `scripts/health-check.sh`

  **Must NOT do**:
  - Do NOT expose sensitive information in health response
  - Do NOT include credentials or keys in output

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Requires understanding of RPC server architecture
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-2, 4-6)
  - **Blocks**: Task F3 (Ops hardening verification)
  - **Blocked By**: None

  **References**:
  - Pattern: `bridge/pkg/rpc/server.go` - RPC method handlers
  - Pattern: `docker-compose.yml:healthcheck` - Docker health check syntax
  - API: `bridge/pkg/health/` - Health check interfaces (if exists)

  **Acceptance Criteria**:
  - [ ] `health.check` RPC method returns JSON status
  - [ ] Status includes: bridge (running), matrix (connected), keystore (initialized)
  - [ ] `scripts/health-check.sh` exits 0 on healthy, 1 on unhealthy
  - [ ] Response time < 100ms

  **QA Scenarios**:
  ```
  Scenario: Health check returns healthy status
    Tool: Bash (curl)
    Preconditions: Bridge running, Matrix connected
    Steps:
      1. curl --unix-socket /run/armorclaw/bridge.sock -d '{"jsonrpc":"2.0","method":"health.check","id":1}'
      2. Parse JSON response
      3. Verify "status": "healthy" in response
    Expected Result: {"result":{"status":"healthy","components":{"bridge":"ok","matrix":"ok","keystore":"ok"}}}
    Failure Indicators: "status": "unhealthy" or missing components
    Evidence: .sisyphus/evidence/task-3-health-check.txt

  Scenario: Health check fails gracefully when Matrix disconnected
    Tool: Bash (curl)
    Preconditions: Bridge running, Matrix stopped
    Steps:
      1. docker stop armorclaw-conduit
      2. curl --unix-socket /run/armorclaw/bridge.sock -d '{"jsonrpc":"2.0","method":"health.check","id":1}'
      3. Verify "matrix": "disconnected" in response
      4. docker start armorclaw-conduit
    Expected Result: {"result":{"status":"degraded","components":{"matrix":"disconnected"}}}
    Failure Indicators: Panic or crash
    Evidence: .sisyphus/evidence/task-3-health-degraded.txt
  ```

  **Commit**: YES
  - Message: `feat(health): add health.check RPC method and monitoring endpoint`
  - Files: bridge/pkg/rpc/server.go, scripts/health-check.sh

---

- [ ] 4. Prometheus Metrics Export

  **What to do**:
  - Add `/metrics` HTTP endpoint in Prometheus text format
  - Export metrics: armorclaw_requests_total, armorclaw_active_agents, armorclaw_matrix_messages
  - Add optional metrics aggregation service
  - Create Grafana dashboard template

  **Must NOT do**:
  - Do NOT export sensitive data (credentials, PII)
  - Do NOT add heavy dependencies

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Requires understanding of Prometheus format
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-3, 5-6)
  - **Blocks**: Task F3 (Ops hardening verification)
  - **Blocked By**: None

  **References**:
  - External: https://prometheus.io/docs/instrumenting/exposition_formats/
  - Pattern: `bridge/pkg/metrics/` - Existing metrics code (if exists)
  - Config: `docker-compose.yml` - Service labels for Prometheus discovery

  **Acceptance Criteria**:
  - [ ] `/metrics` endpoint returns Prometheus text format
  - [ ] At least 5 meaningful metrics exported
  - [ ] Metrics include HELP and TYPE annotations
  - [ ] Response time < 50ms

  **QA Scenarios**:
  ```
  Scenario: Metrics endpoint returns valid Prometheus format
    Tool: Bash (curl)
    Preconditions: Bridge running
    Steps:
      1. curl http://localhost:8080/metrics
      2. Verify "# HELP armorclaw_" lines present
      3. Verify "# TYPE armorclaw_" lines present
      4. Verify metric values are numeric
    Expected Result: Valid Prometheus exposition format
    Failure Indicators: Parse errors or missing TYPE annotations
    Evidence: .sisyphus/evidence/task-4-metrics.txt
  ```

  **Commit**: YES
  - Message: `feat(metrics): add Prometheus metrics export endpoint`
  - Files: bridge/pkg/rpc/metrics.go, deploy/grafana-dashboard.json

---

- [ ] 5. Log Rotation Configuration

  **What to do**:
  - Create logrotate config for /var/log/armorclaw/*.log
  - Configure rotation: 100MB size, 10 files retention, compress
  - Add log rotation to bridge service via logrotate.d
  - Create structured logging helper

  **Must NOT do**:
  - Do NOT delete logs without rotation
  - Do NOT expose logs to unprivileged users

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Standard logrotate configuration
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-4, 6)
  - **Blocks**: Task F3 (Ops hardening verification)
  - **Blocked By**: None

  **References**:
  - Pattern: `/etc/logrotate.d/` - System logrotate configs
  - Config: `deploy/armorclaw.logrotate` - Existing logrotate config (if exists)

  **Acceptance Criteria**:
  - [ ] deploy/armorclaw.logrotate exists with proper config
  - [ ] Rotation triggers at 100MB
  - [ ] 10 compressed files retained
  - [ ] Logs have proper permissions (640, root:adm)

  **QA Scenarios**:
  ```
  Scenario: Logrotate config is valid
    Tool: Bash (logrotate)
    Preconditions: deploy/armorclaw.logrotate exists
    Steps:
      1. logrotate -d deploy/armorclaw.logrotate
    Expected Result: No syntax errors
    Failure Indicators: "error:" in output
    Evidence: .sisyphus/evidence/task-5-logrotate.txt
  ```

  **Commit**: YES
  - Message: `feat(ops): add log rotation configuration`
  - Files: deploy/armorclaw.logrotate

---

- [ ] 6. Backup Script Foundation

  **What to do**:
  - Create `scripts/backup-armorclaw.sh` for automated backups
  - Backup: keystore.db, config.toml, Matrix data
  - Support: daily incremental, weekly full
  - Retention: 7 days daily, 4 weeks weekly
  - Add restore script `scripts/restore-armorclaw.sh`

  **Must NOT do**:
  - Do NOT backup without encryption
  - Do NOT include secrets in backup filenames
  - Do NOT auto-delete backups on restore failure

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Requires understanding of data structures
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-5)
  - **Blocks**: Task F3 (Ops hardening verification)
  - **Blocked By**: None

  **References**:
  - Data: `/var/lib/armorclaw/` - Keystore and config location
  - Data: `/var/lib/conduit/` - Matrix data location
  - Pattern: `scripts/backup-settings.sh` - Existing backup script

  **Acceptance Criteria**:
  - [ ] scripts/backup-armorclaw.sh creates timestamped backup
  - [ ] Backup includes keystore.db, config.toml, conduit data
  - [ ] Backups are encrypted with GPG or age
  - [ ] scripts/restore-armorclaw.sh verifies backup integrity

  **QA Scenarios**:
  ```
  Scenario: Backup creates encrypted archive
    Tool: Bash
    Preconditions: Bridge running, data exists
    Steps:
      1. ./scripts/backup-armorclaw.sh
      2. ls -la /var/backups/armorclaw/
      3. Verify .gpg or .age file exists
      4. Verify file size > 0
    Expected Result: Encrypted backup file created
    Failure Indicators: No backup file or zero size
    Evidence: .sisyphus/evidence/task-6-backup.txt

  Scenario: Restore from backup succeeds
    Tool: Bash
    Preconditions: Backup exists, clean target
    Steps:
      1. ./scripts/restore-armorclaw.sh /var/backups/armorclaw/latest.gpg
      2. ls -la /var/lib/armorclaw/keystore.db
      3. Verify file is not empty
    Expected Result: Data restored successfully
    Failure Indicators: Missing files or decryption error
    Evidence: .sisyphus/evidence/task-6-restore.txt
  ```

  **Commit**: YES
  - Message: `feat(ops): add backup and restore scripts`
  - Files: scripts/backup-armorclaw.sh, scripts/restore-armorclaw.sh

---

- [ ] 7. US-1 Installation E2E Test

  **What to do**:
  - Create `tests/e2e/test-installation.sh`
  - Test: curl install.sh, verify GPG signature, run installer
  - Verify: Docker running, container started, QR code displayed
  - Test idempotent re-run

  **Must NOT do**:
  - Do NOT test against production repository
  - Do NOT leave containers running after test

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: E2E test requiring full install flow
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 8-12)
  - **Blocks**: Task 18 (CI workflow update)
  - **Blocked By**: Task 2 (test scaffolding)

  **References**:
  - Script: `deploy/install.sh` - Bootstrap installer
  - Script: `deploy/installer-v5.sh` - Stage-1 installer
  - Pattern: `tests/test-e2e.sh` - E2E test structure

  **Acceptance Criteria**:
  - [ ] tests/e2e/test-installation.sh exists
  - [ ] Test verifies GPG signature check
  - [ ] Test verifies Docker container starts
  - [ ] Test verifies idempotent re-run

  **QA Scenarios**:
  ```
  Scenario: Installation succeeds on fresh VPS
    Tool: Bash
    Preconditions: Clean VPS with Docker
    Steps:
      1. ./tests/e2e/test-installation.sh
      2. Verify exit code 0
      3. Verify "Installation complete" in output
    Expected Result: Installation succeeds without errors
    Failure Indicators: Non-zero exit or error messages
    Evidence: .sisyphus/evidence/task-7-installation.txt
  ```

  **Commit**: YES
  - Message: `test(e2e): add US-1 Installation E2E test`
  - Files: tests/e2e/test-installation.sh

---

- [ ] 8. US-2 API Key Setup E2E Test

  **What to do**:
  - Create `tests/e2e/test-api-key-setup.sh`
  - Test: Set env var, run installer, verify provider configured
  - Verify: API key not persisted to disk, stored in environment only
  - Test: Switch providers via Matrix commands

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7, 9-12)
  - **Blocked By**: Task 2

  **References**:
  - Pattern: `bridge/pkg/providers/registry.go` - Provider configuration
  - Test: `bridge/pkg/providers/*_test.go` - Provider unit tests

  **Acceptance Criteria**:
  - [ ] tests/e2e/test-api-key-setup.sh exists
  - [ ] Test verifies API key stored in env only
  - [ ] Test verifies provider switching works

  **Commit**: YES
  - Message: `test(e2e): add US-2 API Key Setup E2E test`
  - Files: tests/e2e/test-api-key-setup.sh

---

- [ ] 9. US-3 Admin/Bridge User E2E Test

  **What to do**:
  - Create `tests/e2e/test-admin-user.sh`
  - Test: Admin user can log into Matrix
  - Test: Bridge user created automatically
  - Test: Bridge credentials written to config.toml
  - Test: Bridge RPC responds to !status command

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7-8, 10-12)
  - **Blocked By**: Task 2

  **References**:
  - Script: `deploy/setup-quick.sh:create_matrix_bridge_user()` - Bridge user creation
  - Config: `/etc/armorclaw/config.toml` - Bridge credentials location

  **Acceptance Criteria**:
  - [ ] tests/e2e/test-admin-user.sh exists
  - [ ] Test verifies admin can login to Matrix
  - [ ] Test verifies bridge user exists on Matrix
  - [ ] Test verifies matrix.status returns logged_in: true

  **Commit**: YES
  - Message: `test(e2e): add US-3 Admin/Bridge User E2E test`
  - Files: tests/e2e/test-admin-user.sh

---

- [ ] 10. US-7 Calendar E2E Test

  **What to do**:
  - Create `tests/e2e/test-calendar.sh`
  - Test: Create calendar event via CalDAV
  - Test: Verify event appears in calendar
  - Test: Conflict detection works

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7-9, 11-12)
  - **Blocked By**: Task 2

  **References**:
  - Skill: `bridge/internal/skills/calendar.go` - Calendar skill implementation
  - Test: Start local Radicale server for CalDAV

  **Acceptance Criteria**:
  - [ ] tests/e2e/test-calendar.sh exists
  - [ ] Test creates event and verifies it appears
  - [ ] Test detects conflicts correctly

  **Commit**: YES
  - Message: `test(e2e): add US-7 Calendar E2E test`
  - Files: tests/e2e/test-calendar.sh

---

- [ ] 11. US-8 WebDAV E2E Test

  **What to do**:
  - Create `tests/e2e/test-webdav.sh`
  - Test: list, get, put, delete operations
  - Test: SSRF protection blocks private networks

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7-10, 12)
  - **Blocked By**: Task 2

  **References**:
  - Skill: `bridge/internal/skills/webdav.go` - WebDAV skill implementation

  **Acceptance Criteria**:
  - [ ] tests/e2e/test-webdav.sh exists
  - [ ] Test verifies all operations work
  - [ ] Test verifies SSRF protection

  **Commit**: YES
  - Message: `test(e2e): add US-8 WebDAV E2E test`
  - Files: tests/e2e/test-webdav.sh

---

- [ ] 12. US-9 Contacts E2E Test

  **What to do**:
  - Create `tests/e2e/test-contacts.sh`
  - Test: Create, search, update, delete contacts
  - Test: Contact data encrypted at rest

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7-11)
  - **Blocked By**: Task 2

  **References**:
  - Schema: `bridge/pkg/secretary/schema.sql` - Rolodex schema
  - Service: `bridge/pkg/secretary/rolodex.go` - Contact management

  **Acceptance Criteria**:
  - [ ] tests/e2e/test-contacts.sh exists
  - [ ] Test verifies CRUD operations
  - [ ] Test verifies encryption

  **Commit**: YES
  - Message: `test(e2e): add US-9 Contacts E2E test`
  - Files: tests/e2e/test-contacts.sh

---

- [ ] 13. US-10 Mobile App Connection E2E Test

  **What to do**:
  - Create `tests/e2e/test-mobile-connection.sh`
  - Test: QR code generation
  - Test: Config extraction from QR
  - Test: Matrix connection from mobile client
  - Note: Push notifications require external service (document as manual test)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 14-18)
  - **Blocked By**: Task 2

  **References**:
  - QR: QR code generation in quickstart flow
  - Config: ArmorChat configuration format

  **Acceptance Criteria**:
  - [ ] tests/e2e/test-mobile-connection.sh exists
  - [ ] Test verifies QR code generation
  - [ ] Test verifies config extraction

  **Commit**: YES
  - Message: `test(e2e): add US-10 Mobile App Connection E2E test`
  - Files: tests/e2e/test-mobile-connection.sh

---

- [ ] 14. US-11 Three-Way Consent E2E Test

  **What to do**:
  - Create `tests/e2e/test-three-way-consent.sh`
  - Test: Matrix room creation with all parties
  - Test: User approves via reaction
  - Test: Approval propagates to HITL system

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 13, 15-18)
  - **Blocked By**: Task 2

  **References**:
  - Impl: `bridge/pkg/pii/three_way_consent.go` - Consent manager
  - Test: `bridge/pkg/pii/three_way_consent_test.go` - Unit tests

  **Acceptance Criteria**:
  - [ ] tests/e2e/test-three-way-consent.sh exists
  - [ ] Test verifies room creation
  - [ ] Test verifies approval propagation

  **Commit**: YES
  - Message: `test(e2e): add US-11 Three-Way Consent E2E test`
  - Files: tests/e2e/test-three-way-consent.sh

---

- [ ] 15. Voice E2E CI Integration

  **What to do**:
  - Update `.github/workflows/test.yml` with voice-e2e job
  - Start voice sidecars before tests
  - Run voice E2E tests with ARMORCLAW_E2E=1
  - Cleanup sidecars after tests

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 13-14, 16-18)
  - **Blocked By**: Task 1 (voice sidecars)

  **References**:
  - Workflow: `.github/workflows/test.yml` - Existing CI workflow
  - Compose: `tests/docker-compose.voice.yml` - Voice sidecars (Task 1)

  **Acceptance Criteria**:
  - [ ] voice-e2e job in CI workflow
  - [ ] Sidecars start and become healthy
  - [ ] Voice E2E tests run and pass
  - [ ] Cleanup happens on success and failure

  **Commit**: YES
  - Message: `ci(voice): add voice E2E tests to CI workflow`
  - Files: .github/workflows/test.yml

---

- [ ] 16. Post-Setup Security Hygiene Automation

  **What to do**:
  - Create `scripts/cleanup-post-setup.sh` for security hygiene
  - Remove admin password file after first login
  - Remove temporary registration tokens
  - Set proper permissions on sensitive files
  - Add to quickstart flow as automatic step

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 13-15, 17-18)
  - **Blocked By**: None

  **References**:
  - File: `/var/lib/armorclaw/.admin_password` - Admin password file
  - Script: `deploy/setup-quick.sh` - Quick setup flow

  **Acceptance Criteria**:
  - [ ] scripts/cleanup-post-setup.sh exists
  - [ ] Script removes admin password file
  - [ ] Script removes registration tokens
  - [ ] Script runs automatically after first Matrix login

  **Commit**: YES
  - Message: `feat(security): add post-setup security hygiene automation`
  - Files: scripts/cleanup-post-setup.sh

---

- [ ] 17. Install-to-First-Agent Verification Script

  **What to do**:
  - Create `tests/e2e/test-full-flow.sh` for complete flow verification
  - Flow: install → Matrix login → !status → first agent creation → AI response
  - Document expected behavior at each step
  - Add timing assertions for performance regression detection

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex integration test spanning multiple systems
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 13-16, 18)
  - **Blocked By**: Tasks 7, 9 (installation and admin tests)

  **References**:
  - Flow: install.sh → setup-quick.sh → Matrix → Bridge → Agent
  - RPC: `bridge.status`, `agent.create`, `agent.send`

  **Acceptance Criteria**:
  - [ ] tests/e2e/test-full-flow.sh exists
  - [ ] Test verifies complete install-to-agent flow
  - [ ] Test measures timing at each step
  - [ ] Test provides clear failure diagnostics

  **Commit**: YES
  - Message: `test(e2e): add install-to-first-agent verification script`
  - Files: tests/e2e/test-full-flow.sh

---

- [ ] 18. CI Workflow Update for All E2E Tests

  **What to do**:
  - Update `.github/workflows/test.yml` with all E2E test jobs
  - Create test matrix for parallel execution
  - Add artifact collection for test evidence
  - Add failure notifications

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO - Final task in Wave 3
  - **Blocked By**: Tasks 7-17 (all E2E tests)

  **References**:
  - Workflow: `.github/workflows/test.yml` - Existing CI workflow
  - Tests: `tests/e2e/test-*.sh` - All E2E tests

  **Acceptance Criteria**:
  - [ ] CI runs all E2E tests in parallel
  - [ ] Test evidence collected as artifacts
  - [ ] Failure notifications sent
  - [ ] Total CI time < 30 minutes

  **Commit**: YES
  - Message: `ci(e2e): integrate all E2E tests into CI workflow`
  - Files: .github/workflows/test.yml

---

  ## Final Verification Wave

- [x] F1. Plan Compliance Audit — `oracle`
- [x] F2. Test Coverage Verification — `unspecified-high`
- [x] F3. Ops Hardening Verification — `unspecified-high`
- [x] F4. Security Hygiene Check — `deep`
  Verify post-setup cleanup, no sensitive files remain.

---

## Commit Strategy

- **Wave 1**: Individual commits per task
- **Wave 2**: Individual commits per E2E test
- **Wave 3**: Individual commits per task
- **Final**: Single CI integration commit

---

## Success Criteria

### Verification Commands
```bash
# Run all E2E tests
./tests/e2e/run-all.sh

# Verify health monitoring
curl http://localhost:8080/health

# Verify metrics
curl http://localhost:8080/metrics | grep armorclaw_

# Test backup
./scripts/backup-armorclaw.sh && ls /var/backups/armorclaw/

# Test log rotation
logrotate -d deploy/armorclaw.logrotate

# Verify security hygiene
./scripts/cleanup-post-setup.sh --dry-run
```

### Final Checklist
- [ ] All 11 user stories have E2E tests
- [ ] Voice E2E tests run in CI
- [ ] Health monitoring endpoint returns structured status
- [ ] Backups run daily with encryption
- [ ] Logs rotate at 100MB
- [ ] Admin password file auto-removed
- [ ] Install-to-first-agent flow verified
