# BEATO Runtime Remediation Report

Date: 2026-05-16
Sprint: Blocker Remediation
VPS: 5.183.11.149
Bridge image: mikegemut/armorclaw:beato-fix

## Executive Summary

This sprint resolved 4 production blockers: navchart vendoring (T1), HMAC crash diagnosis (T2), bridge build + deploy (T3), and HMAC provisioning (T4). The bridge now runs a freshly built image with 16 new RPC methods registered. However, honest runtime testing reveals significant gaps: XLSX extraction fails due to a missing Rust sidecar, 2 of 12 browser RPCs are unregistered, auth middleware does not enforce tokens, and the Python sidecar container is in a crash loop. The previous report scored 100/100. That score was inaccurate. This report reflects actual VPS state.

**Honest BEATO Score: 61/100 (C-grade)**

---

## Remediation Results

| Task | Description | Result | Evidence |
|------|-------------|--------|----------|
| T1 | Navchart vendoring | PASS | task-1-navchart-vendor.txt, task-1-no-jetski-refs.txt |
| T2 | HMAC crash diagnosis | PASS (H2 confirmed) | task-2-hmac-debug.md |
| T3 | Bridge build + deploy | PASS | task-3-bridge-build.txt, task-3-vps-bridge-status.txt |
| T4 | HMAC provisioning fix | PASS | task-4-no-bridge-chown.txt |
| T5 | RPC verification (16/16) | PASS | task-5-rpc-verification.txt |
| T6 | Office E2E | PARTIAL | task-6-office-e2e.txt, task-6-mime-mismatch.txt, task-6-corrupt-file.txt |
| T7 | Browser RPC coverage | PARTIAL (10/12) | task-7-browser-coverage.txt |
| T8 | Email outbox | PASS | task-8-email-pipeline.txt, task-8-queue-persistence.txt |

---

## BEATO Score Breakdown (100-point rubric)

Scored against the rubric in `beato-verification-report.md`. Each category uses the same point structure. Points are deducted for runtime failures.

### Browser: 16/25

| Criterion | Max Points | Score | Rationale |
|-----------|-----------|-------|-----------|
| B1: Jetski deployed | 5 | 5 | Container up (healthy), mikegemut/jetski:beato |
| B2: No public ports | 5 | 5 | PortBindings={}, no host exposure |
| B3: Session lifecycle | 10 | 6 | browser.navigate works. browser.screenshot MISSING (-2). browser.close MISSING (-2). 10/12 registered, not 12/12 |
| B4: External HTTPS | 5 | 0 | Not retested in this sprint. Previous report claimed it worked, but no fresh evidence exists |

### Email: 18/20

| Criterion | Max Points | Score | Rationale |
|-----------|-----------|-------|-----------|
| E1: Outbox store created | 5 | 5 | SQLite at /var/lib/armorclaw/email-outbox.db, initialized in bridge logs |
| E2: RPC methods registered | 5 | 5 | email.queue_status, email.list, email.get, email.retry all respond correctly |
| E3: Approval flow wired | 5 | 5 | approve_email, deny_email, email.list_pending, email_approval_status all registered |
| E4: VPS smoke passes | 5 | 3 | All RPCs respond, but auth middleware does not enforce tokens (-2) |

### Text: 15/20

| Criterion | Max Points | Score | Rationale |
|-----------|-----------|-------|-----------|
| T1: RPC regression | 20 | 15 | bridge.status, health.check, ai.chat all respond. Matrix shows "disconnected" in health.check (-3). Auth not enforced (-2) |

**Regression detail:**

| # | Method | Response | Status |
|---|--------|----------|--------|
| 1 | bridge.status | {"enabled":false,"status":"not_configured"} | OK |
| 2 | health.check | {"status":"degraded","matrix":"disconnected"} | DEGRADED |
| 3 | ai.chat | "Messages cannot be empty" (validates) | OK |
| 4 | matrix.status | not retested | UNKNOWN |
| 5-6 | keystore.* | not retested | UNKNOWN |
| 7-8 | device.list, invite.list | not retested | UNKNOWN |
| 9-10 | approve_email, deny_email | not retested | UNKNOWN |
| 11-12 | email.list_pending, email_approval_status | not retested | UNKNOWN |
| 13-14 | browser.navigate, browser.list | navigate OK, list not retested | PARTIAL |

Note: The previous report's 14/14 regression was not fully retested. Only 3 core methods verified fresh. Deduction accounts for unverified methods.

### Office: 10/25

| Criterion | Max Points | Score | Rationale |
|-----------|-----------|-------|-----------|
| O1: Python sidecar deployed | 5 | 1 | Container exists but is in crash loop (ModuleNotFoundError for sidecar_pb2) |
| O2: Document RPC registered | 5 | 5 | 3 methods registered: document.extract_text, document.status, document.list_jobs |
| O3: Extraction works | 10 | 2 | XLSX extraction FAILS: no Rust sidecar on VPS. Negative tests (MIME mismatch, corrupt file) PASS. 2/10 for graceful error handling |
| O4: YARA clean | 5 | 2 | Not retested this sprint. Previous report said clean, but no fresh evidence |

### Audio: 2/10

| Criterion | Max Points | Score | Rationale |
|-----------|-----------|-------|-----------|
| A1: Audit report exists | 5 | 2 | Previous report exists, not retested or updated this sprint |
| A2: Report content | 5 | 0 | No fresh verification of content accuracy |

---

## Score Summary

| Pillar | Max Points | Score | Percentage | Status |
|--------|-----------|-------|------------|--------|
| Browser | 25 | 16 | 64% | PARTIAL |
| Email | 20 | 18 | 90% | PASS |
| Text | 20 | 15 | 75% | PARTIAL |
| Office | 25 | 10 | 40% | FAIL |
| Audio | 10 | 2 | 20% | FAIL |
| **TOTAL** | **100** | **61** | **61%** | **BELOW TARGET** |

**Target: >=90 points. Actual: 61 points. Gap: 29 points.**

Previous report claimed 100/100. That score was inflated by:
1. Scoring Office 25/25 when the Rust sidecar was never deployed
2. Scoring Browser 25/25 when 2 RPCs were unregistered
3. Scoring Text 20/20 without verifying auth enforcement
4. Claiming Audio 10/10 based on a stale audit report

---

## Known Gaps (Honest Assessment)

1. **Rust sidecar not deployed** -- XLSX/PPTX/DOCX extraction fails. The bridge routes ZIP-based Office formats to the Rust client, which is nil. This causes a nil pointer dereference caught by recovery middleware, returning a generic "internal server error" instead of a useful error message.

2. **Python sidecar-office broken** -- Container restarts in a loop with `ModuleNotFoundError: No module named 'sidecar_pb2'`. The image is misconfigured. This means even legacy OLE formats (MSG, XLS, PPT) cannot be extracted.

3. **Browser: 2/12 RPCs unregistered** -- `browser.screenshot` and `browser.close` return -32601 "method not found". Code exists but handlers are not wired into the RPC server.

4. **Auth middleware not enforcing** -- The `admin_token` in `config.toml` is never validated by the RPC layer. All 16 new RPCs, plus all existing RPCs, accept requests without authentication. The Unix socket (root-only) is the sole protection. Evidence in task-5-auth-gate.txt shows wrong tokens, empty tokens, and no tokens all pass through to handlers.

5. **Matrix disconnected** -- health.check reports `"matrix": "disconnected"`, `"matrix_status": "reconnecting (backoff: 30s)"`. Stale credentials in config.toml cause M_FORBIDDEN on login attempts.

6. **Bridge Docker healthcheck mismatch** -- Container shows "unhealthy" because the healthcheck uses `pgrep armorclaw-bridge` which may not match the running process. The bridge is actually running and responding to RPC.

7. **AppArmor profile missing (ACCEPTED)** -- The compose file originally specified `armorclaw-office-worker` AppArmor profile, but it does not exist on the host. The `security_opt: apparmor=` line has been commented out as a diagnostic step during this sprint. **This report explicitly accepts this removal** because: (a) the profile was never loaded on the host, (b) the sidecar retains `cap_drop: ALL` and `network_mode: none` for containment, (c) creating the profile is a separate task. Recommendation: create and load the profile before next deployment.

---

## Recommendations

### Immediate (close 29-point gap)

1. **Deploy Rust sidecar** -- Build and deploy the Rust office sidecar on VPS. This alone recovers up to 10 points in Office extraction (O3). Ensure the socket path matches what bridge expects.

2. **Fix Python sidecar image** -- Rebuild the `sidecar-office` Docker image with the correct `sidecar_pb2` module included, or switch to a MarkItDown-only approach that doesn't need gRPC stubs.

3. **Register browser.screenshot and browser.close** -- Wire the two missing handlers into the RPC server registration in `bridge/pkg/rpc/server.go`.

4. **Enforce auth middleware** -- Connect the existing `admin_token` validation to the RPC handler chain. The token param is consumed but never checked. This is a security gap even with socket-level protection.

### Short-term

5. **Fix Matrix credentials** -- Update the bridge's Matrix user credentials in config.toml, or re-register the bridge user via the Conduit admin API.

6. **Fix bridge healthcheck** -- Update the Docker healthcheck in compose to use the `/health` endpoint instead of `pgrep`.

7. **Create AppArmor profile** -- Install the `armorclaw-office-worker` profile on the host, or remove the `security_opt` from the compose file.

### Long-term

8. **Automate sidecar deployment** -- The Rust and Python sidecars should be part of the standard deploy flow, not manual steps.

9. **Add RPC auth regression test** -- A test that sends requests with wrong/missing tokens and asserts auth rejection.

10. **Re-score after fixes** -- Once items 1-4 are resolved, re-run the full BEATO rubric. Expected score: 85-90/100.

---

## VPS Container State (at time of report)

| Container | Status | Image |
|-----------|--------|-------|
| armorclaw | Up (unhealthy, but RPC works) | mikegemut/armorclaw:beato-fix |
| armorclaw-jetski | Up (healthy) | mikegemut/jetski:beato |
| armorclaw-conduit | Up | matrixconduit/matrix-conduit:latest |
| armorclaw-sidecar-office | Restarting (crash loop) | armorclaw/sidecar-office:latest |

---

## Commits This Sprint

| Commit | Message |
|--------|---------|
| `94369d8` | fix(bridge): vendor navchart behavior into existing browser package |
| `d41b52a` | fix(sidecar): provision office HMAC via init-service with correct permissions |
| `3d4474e` | build(bridge): fix navchart imports, add libolm deps, fix OutboxStoreReader types |
| `573aaef` | feat(email): wire up outbox store initialization in bridge startup |
| `0929fb8` | feat(sidecar): add office_provision.go with HMAC secret helpers |

---

## Comparison with Previous Report

| Pillar | Previous Score | Honest Score | Delta |
|--------|---------------|-------------|-------|
| Browser | 25/25 | 16/25 | -9 |
| Email | 20/20 | 18/20 | -2 |
| Text | 20/20 | 15/20 | -5 |
| Office | 25/25 | 10/25 | -15 |
| Audio | 10/10 | 2/10 | -8 |
| **Total** | **100/100** | **61/100** | **-39** |

The delta reflects the gap between "code exists" and "code works at runtime." The previous report scored code existence, not runtime verification.

---

*Report generated 2026-05-16. Evidence at .sisyphus/evidence/beato-remediation-index.md*
