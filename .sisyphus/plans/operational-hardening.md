# Operational Hardening Plan

## TL;DR

> **Quick Summary**: Expose existing Prometheus metrics, deploy monitoring stack, automate backups, and wire alert handlers for production readiness.
> 
> **Deliverables**:
> - `/metrics` endpoint on bridge HTTP server
> - Prometheus + Grafana in docker-compose-full.yml
> - Automated backup script with systemd timer
> - Matrix + webhook alert handlers
> - Security hardening (auth, encryption, rate limits)
>
> **Estimated Effort**: Medium (15-20 hours)
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: Metrics endpoint → Prometheus config → Alert handlers

---

## Context

### Original Request
Complete the 5 operational hardening items identified in post-production roadmap for production ArmorClaw deployment.

### Interview Summary
**User Decisions**:
- **Deployment Target**: Codebase only (features in code/Docker images, manual VPS deployment)
- **Monitoring Stack**: New Prometheus/Grafana deployment needed
- **Backup Storage**: Local only (`/var/lib/armorclaw/backups/`)
- **Alerting**: Matrix (primary) + Webhook (external integration)

**Research Findings**:
- `/health` endpoint already exists at `bridge/pkg/http/server.go:146`
- 20+ Prometheus metrics already instrumented in `bridge/internal/metrics/agent.go`
- Logrotate config exists in `deploy/harden-final.sh:268-282`
- Backup utility exists at `deploy/backup-settings.sh` (manual only)

### Metis Review
**Identified Gaps** (addressed):
- **Health endpoint exists**: No new `/healthz` needed, use existing `/health`
- **Time underestimated**: Adjusted from 6-8h to 15-20h realistic estimate
- **Security guardrails**: Added authentication, rate limiting, encryption requirements
- **Scope creep risks**: Locked down to minimal viable implementation

---

## Work Objectives

### Core Objective
Implement production-ready operational hardening: metrics exposure, monitoring stack, automated backups, and alerting.

### Concrete Deliverables
- `/metrics` HTTP endpoint on bridge port 8443
- Prometheus + Grafana services in docker-compose-full.yml
- `deploy/backup-automated.sh` script + systemd timer unit
- Matrix notification handler for alerts
- Webhook support for external integrations
- Security configuration (auth, rate limits, encryption)

### Definition of Done
- [ ] `curl http://localhost:8443/metrics` returns Prometheus-formatted metrics
- [ ] Prometheus scrapes metrics successfully
- [ ] Grafana dashboard shows bridge metrics
- [ ] Backup runs automatically at 2 AM daily
- [ ] Alert fires to Matrix room on health failure
- [ ] Webhook receives alert payload on threshold breach
- [ ] No public exposure of /metrics or Grafana

### Must Have
- Prometheus metrics endpoint exposed (authenticated or internal network only)
- Daily automated backups with 7-day retention
- Matrix alerting for critical failures
- Resource limits on Prometheus/Grafana containers

### Must NOT Have (Guardrails)
- **DO NOT** expose /metrics endpoint publicly (security risk)
- **DO NOT** store backup encryption keys alongside backups
- **DO NOT** log API keys or secrets in metrics/alerts
- **DO NOT** bypass Matrix as primary alert channel
- **DO NOT** add custom dashboards beyond 5 core panels
- **DO NOT** implement advanced alerting (ML, anomaly detection)
- **DO NOT** add notification channels beyond Matrix + webhook
- **DO NOT** modify existing `/health` endpoint behavior

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: N/A (infrastructure changes)
- **Automated tests**: NO (infrastructure focused)
- **Agent-Executed QA**: YES - verify each component works

### QA Policy
Every task includes agent-executed QA scenarios using:
- **API/Backend**: Bash (curl) - Send requests, assert status + response
- **Infrastructure**: Bash - Verify services running, configs valid
- **Integration**: Bash - Test end-to-end flows

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation - can start immediately):
├── Task 1: Add /metrics endpoint to bridge [quick]
├── Task 2: Create backup script + systemd unit [quick]
└── Task 3: Add security middleware (rate limits, auth) [unspecified-high]

Wave 2 (Infrastructure - depends on Wave 1):
├── Task 4: Add Prometheus + Grafana to docker-compose [visual-engineering]
├── Task 5: Create Grafana dashboards [visual-engineering]
└── Task 6: Wire alert handlers (Matrix + webhook) [unspecified-high]

Wave 3 (Integration - depends on Wave 2):
├── Task 7: Configure alert rules [quick]
└── Task 8: Documentation + deployment guide [writing]

Wave FINAL (Verification):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Security review (unspecified-high)
├── Task F3: Integration test (unspecified-high)
└── Task F4: Documentation completeness (deep)
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|------------|--------|
| 1 | - | 4, 5, 6 |
| 2 | - | 8 |
| 3 | - | 4, 6 |
| 4 | 1, 3 | 5, 7 |
| 5 | 4 | F3 |
| 6 | 1, 3 | 7, F3 |
| 7 | 4, 6 | F3 |
| 8 | 2, 5, 7 | F4 |

### Agent Dispatch Summary

- **Wave 1**: 3 agents — T1→quick, T2→quick, T3→unspecified-high
- **Wave 2**: 3 agents — T4→visual-engineering, T5→visual-engineering, T6→unspecified-high
- **Wave 3**: 2 agents — T7→quick, T8→writing
- **FINAL**: 4 agents — F1→oracle, F2→unspecified-high, F3→unspecified-high, F4→deep

---

## TODOs

- [ ] 1. Add /metrics Endpoint to Bridge HTTP Server

  **What to do**:
  - Add `/metrics` route to `bridge/pkg/http/server.go` using `promhttp.Handler()`
  - Import `github.com/prometheus/client_golang/prometheus/promhttp`
  - Mount at `/metrics` path on existing HTTP server (port 8443)
  - Expose all 20+ existing metrics from `bridge/internal/metrics/agent.go`

  **Must NOT do**:
  - DO NOT add new metrics (only expose existing ones)
  - DO NOT expose endpoint publicly without auth/network restriction
  - DO NOT modify existing `/health` endpoint

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single file change, well-defined pattern
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3)
  - **Blocks**: Task 4, Task 5, Task 6
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/http/server.go:146` - Existing `/health` endpoint pattern to follow
  - `bridge/internal/metrics/agent.go` - All 20+ metrics already defined
  - `go.mod` - Check if prometheus client already imported

  **Acceptance Criteria**:
  - [ ] `curl http://localhost:8443/metrics` returns HTTP 200
  - [ ] Response includes `armorclaw_agent_tasks_total`
  - [ ] Response includes `armorclaw_llm_requests_total`
  - [ ] Response Content-Type is `text/plain; version=0.0.4`
  - [ ] Build passes with `go build ./cmd/bridge`

  **QA Scenarios**:
  ```
  Scenario: Metrics endpoint returns valid Prometheus format
    Tool: Bash (curl)
    Steps:
      1. Start bridge: `./bridge &`
      2. Wait for startup: `sleep 5`
      3. Request metrics: `curl -s http://localhost:8443/metrics`
      4. Check format: `grep "armorclaw_agent_tasks_total"`
    Expected Result: HTTP 200, contains armorclaw metrics
    Evidence: .sisyphus/evidence/task-1-metrics-endpoint.txt
  ```

  **Commit**: YES
  - Message: `feat(ops): add /metrics endpoint for Prometheus scraping`

- [ ] 2. Create Automated Backup Script + Systemd Timer

  **What to do**:
  - Create `deploy/backup-automated.sh` that calls existing `backup-settings.sh`
  - Create `deploy/armorclaw-backup.service` systemd unit
  - Create `deploy/armorclaw-backup.timer` (runs daily at 2 AM)
  - Add 7-day retention with automatic cleanup
  - Backup targets: rolodex.db, keystore.db, conduit data
  - DO NOT backup API keys (hardware-bound)

  **QA Scenarios**:
  ```
  Scenario: Backup runs and creates archive
    Tool: Bash
    Steps:
      1. Run backup manually: `./deploy/backup-automated.sh`
      2. Check backup exists: `ls /var/lib/armorclaw/backups/`
      3. Verify archive integrity: `unzip -t backup-*.zip`
    Expected Result: Backup file created, valid zip archive
    Evidence: .sisyphus/evidence/task-2-backup-script.txt

  Scenario: Systemd timer is scheduled
    Tool: Bash
    Steps:
      1. Copy units to /etc/systemd/system/
      2. Enable timer: `systemctl enable armorclaw-backup.timer`
      3. Check schedule: `systemctl list-timers armorclaw-backup.timer`
    Expected Result: Timer shows next run at 2 AM
    Evidence: .sisyphus/evidence/task-2-systemd-timer.txt
  ```

  **Commit**: YES
  - Message: `feat(backup): add automated backup with systemd timer`

- [ ] 3. Add Security Middleware (Rate Limits, Auth)

  **What to do**:
  - Add rate limiting middleware to `bridge/pkg/http/middleware/`
  - Rate limit: 100 req/min per IP for /metrics endpoint
  - Add optional basic auth for /metrics (config via `config.toml`)
  - Add IP allowlist for internal network access
  - DO NOT expose /metrics publicly (default: localhost only)

  **QA Scenarios**:
  ```
  Scenario: Rate limiting works
    Tool: Bash
    Steps:
      1. Send 110 requests rapidly: `for i in {1..110}; do curl -s http://localhost:8443/metrics > /dev/null; done`
      2. Check last response: `curl -s -w "%{http_code}" http://localhost:8443/metrics`
    Expected Result: HTTP 429 after 100 requests
    Evidence: .sisyphus/evidence/task-3-rate-limit.txt

  Scenario: /metrics not accessible from external IP
    Tool: Bash
    Steps:
      1. Request from external IP: `curl http://<EXTERNAL_IP>:8443/metrics`
    Expected Result: HTTP 403 Forbidden
    Evidence: .sisyphus/evidence/task-3-auth.txt
  ```

  **Commit**: YES
  - Message: `feat(security): add rate limiting and auth for metrics endpoint`

- [ ] 4. Add Prometheus + Grafana to Docker Compose

  **What to do**:
  - Add Prometheus service to `docker-compose-full.yml`
  - Add Grafana service to `docker-compose-full.yml`
  - Configure Prometheus to scrape bridge `/metrics` endpoint
  - Set resource limits: Prometheus 512MB, Grafana 256MB
  - Configure retention: Prometheus 7 days
  - Add persistent volumes for metrics and dashboards
  - Network isolation: internal monitoring network
  - DO NOT expose ports publicly (internal only)

  **QA Scenarios**:
  ```
  Scenario: Prometheus scrapes metrics successfully
    Tool: Bash
    Steps:
      1. Start stack: `docker compose -f docker-compose-full.yml up -d`
      2. Wait for startup: `sleep 30`
      3. Check targets: `curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[0].health'`
    Expected Result: "up"
    Evidence: .sisyphus/evidence/task-4-prometheus.txt

  Scenario: Grafana is healthy
    Tool: Bash
    Steps:
      1. Check health: `curl -s http://localhost:3000/api/health`
    Expected Result: {"commit":"...","database":"ok","version":"..."}
    Evidence: .sisyphus/evidence/task-4-grafana.txt
  ```

  **Commit**: YES
  - Message: `feat(monitoring): add Prometheus and Grafana to docker-compose`

- [ ] 5. Create Grafana Dashboards

  **What to do**:
  - Create 5 core dashboards (NO plugins):
    1. System Overview (CPU, memory, disk, network)
    2. Bridge Metrics (tasks, steps, tokens)
    3. Matrix Health (messages, connections)
    4. Storage (backup status, disk usage)
    5. Alerts (alert count, severity breakdown)
  - Configure Prometheus datasource in Grafana
  - Export dashboards as JSON for version control
  - DO NOT add custom plugins or advanced visualizations

  **QA Scenarios**:
  ```
  Scenario: Dashboards are accessible
    Tool: Bash (curl)
    Steps:
      1. List dashboards: `curl -s http://admin:admin@localhost:3000/api/search | jq '.[].title'`
    Expected Result: Contains "System", "Bridge", "Matrix", "Storage", "Alerts"
    Evidence: .sisyphus/evidence/task-5-dashboards.txt
  ```

  **Commit**: YES
  - Message: `feat(monitoring): add Grafana dashboards for ArmorClaw metrics`

- [ ] 6. Wire Alert Handlers (Matrix + Webhook)

  **What to do**:
  - Create `bridge/pkg/alerts/handler.go` for alert management
  - Implement Matrix notification sender (uses existing Matrix client)
  - Implement webhook sender (HTTP POST with retry)
  - Wire to health monitor failures in `bridge/pkg/health/monitor.go`
  - Wire to budget alerts in `bridge/pkg/budget/tracker.go`
  - Add configuration in `config.toml`:
    ```toml
    [alerts]
    matrix_room_id = "!xxx:server"
    webhook_url = "https://hooks.example.com/alert"
    webhook_enabled = true
    ```
  - Rate limit alerts: max 10/hour per alert type

  **QA Scenarios**:
  ```
  Scenario: Matrix alert sent on health failure
    Tool: Bash
    Steps:
      1. Trigger test alert: `curl -X POST http://localhost:8443/admin/alerts/test`
      2. Check Matrix room for message
    Expected Result: Alert message appears in Matrix room
    Evidence: .sisyphus/evidence/task-6-matrix-alert.txt

  Scenario: Webhook receives alert payload
    Tool: Bash (mock server)
    Steps:
      1. Start mock webhook: `nc -l 8080 &`
      2. Trigger test alert with webhook URL pointing to mock
      3. Check received payload
    Expected Result: JSON payload with alert_type, message, timestamp
    Evidence: .sisyphus/evidence/task-6-webhook.txt
  ```

  **Commit**: YES
  - Message: `feat(alerts): add Matrix and webhook alert handlers`

- [ ] 7. Configure Alert Rules

  **What to do**:
  - Add Prometheus alerting rules in `deploy/monitoring/alerts.yml`:
    - BridgeDown (critical): No metrics scrape for 2 minutes
    - HighErrorRate (warning): >5% error rate for 5 minutes
    - BudgetThreshold (warning): >80% budget usage
    - DiskSpaceLow (critical): <10% disk space
    - BackupFailed (warning): Backup job failed
    - ContainerUnhealthy (warning): Health check failing
  - Configure alert routing in Prometheus config
  - DO NOT add complex multi-condition rules (keep simple threshold-based)

  **QA Scenarios**:
  ```
  Scenario: Alert fires when bridge down
    Tool: Bash
    Steps:
      1. Stop bridge: `pkill armorclaw-bridge`
      2. Wait 3 minutes: `sleep 180`
      3. Check Prometheus alerts: `curl -s http://localhost:9090/api/v1/alerts | jq '.data.alerts[].state'`
    Expected Result: Alert state "firing"
    Evidence: .sisyphus/evidence/task-7-alert-rules.txt
  ```

  **Commit**: YES
  - Message: `feat(monitoring): add Prometheus alerting rules`

- [ ] 8. Documentation + Deployment Guide

  **What to do**:
  - Create `docs/guides/operational-hardening.md` with:
    - How to deploy monitoring stack
    - How to configure backup retention
    - How to set up webhook endpoints
    - How to access Grafana (default credentials)
    - Backup restore procedure
    - Alert troubleshooting
  - Update `docs/ACTIVE/review.md` with Phase 18
  - Add environment variables to `config.example.toml`
  - Create deployment checklist in `deploy/checklist-ops.md`

  **QA Scenarios**:
  ```
  Scenario: Documentation is complete
    Tool: Bash
    Steps:
      1. Check file exists: `test -f docs/guides/operational-hardening.md`
      2. Check required sections: `grep -E "## (Deploy|Backup|Alert|Grafana)" docs/guides/operational-hardening.md`
    Expected Result: All sections present
    Evidence: .sisyphus/evidence/task-8-docs.txt
  ```

  **Commit**: YES
  - Message: `docs(ops): add operational hardening deployment guide`

---

## Final Verification Wave

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read plan end-to-end. Verify each "Must Have" is implemented. Check evidence files exist.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Security Review** — `unspecified-high`
  Verify /metrics not publicly exposed. Check backup encryption. Validate no secrets in logs.
  Output: `Public Exposure [NONE] | Encryption [CONFIGURED] | Secrets [NONE] | VERDICT`

- [ ] F3. **Integration Test** — `unspecified-high`
  Start stack, trigger health failure, verify Matrix alert. Trigger threshold, verify webhook.
  Output: `Alerts [N/N] | Webhooks [N/N] | Backups [N/N] | VERDICT`

- [ ] F4. **Documentation Completeness** — `deep`
  Verify deployment guide exists. Check Grafana access docs. Verify backup restore procedure.
  Output: `Docs [N/N] | Examples [N/N] | Procedures [N/N] | VERDICT`

---

## Commit Strategy

- **Task 1-3**: `feat(ops): add metrics endpoint, backups, security middleware`
- **Task 4-5**: `feat(monitoring): add Prometheus and Grafana stack`
- **Task 6-7**: `feat(alerts): add Matrix and webhook alert handlers`
- **Task 8**: `docs(ops): add operational hardening documentation`

---

## Success Criteria

### Verification Commands
```bash
# Metrics endpoint
curl -s http://localhost:8443/metrics | head -20

# Prometheus scraping
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[].health'

# Grafana health
curl -s http://localhost:3000/api/health

# Backup status
systemctl list-timers armorclaw-backup.timer

# Alert test (trigger manually)
curl -X POST http://localhost:8443/admin/alerts/test
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] No public exposure of sensitive endpoints
- [ ] Backups running automatically
- [ ] Alerts firing to Matrix
- [ ] Documentation complete
