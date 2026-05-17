# BEATO v1.1 Runtime Fix — Evidence Index

> Generated: 2026-05-16
> Sprint: BEATO v1.1 Runtime Fix
> Plan: `.sisyphus/plans/beato-v1.1-runtime-fix.md`

## Implementation Tasks

| Task | Description | Evidence Files | Status | Commit |
|------|-------------|---------------|--------|--------|
| T1 | Wire SafetyMiddleware into BEATO handlers | Source: `bridge/pkg/rpc/server.go` (23 handlers wrapped, nil guard added) | PASS | `ed86935` |
| T2 | Auth regression tests | Source: `bridge/pkg/rpc/auth_integration_test.go` (7 test functions, 261 lines) | PASS | `37b7bbb` |
| T3-V | Verify Python sidecar operational | (7/7 checks passed: proto files, import path, Dockerfile, compose) | PASS | N/A (verify only) |
| T4 | Formal defer of Rust sidecar | `.sisyphus/evidence/task-4-rust-deferral.md` | PASS | `30caa63` |
| T5 | Implement browser.screenshot + browser.close | Source: `bridge/pkg/rpc/browser.go` (+146 lines), `bridge/pkg/rpc/server.go` (+4 lines) | PASS | `9f30fe4` |
| T6 | Extend browser smoke to 14 methods | Source: `tests/test-browser-smoke.sh` (+350 lines, B4-B14 + AUTH section) | PASS | `fdb8811` |
| T7-V | Verify Matrix client operational | `.sisyphus/evidence/task-7v-matrix-verify.txt` | PASS | N/A (verify only) |
| T8 | Revalidate email outbox after auth | `.sisyphus/evidence/task-8-email-revalidation.txt` (8/8 handlers wrapped) | PASS | N/A (verify only) |
| T9-V | Verify healthchecks comprehensive | `.sisyphus/evidence/task-9v-healthcheck-verify.txt` (7+ healthchecks across compose) | PASS | N/A (verify only) |
| T10 | AppArmor risk acceptance | `.sisyphus/evidence/task-10-apparmor-risk.md` | PASS | (in Wave 3 commit) |
| T11 | BEATO runtime re-score | `tests/reports/beato-runtime-report.md` (v1.1 section appended, 71/100) | PASS | (pending) |
| T12 | Evidence index | `.sisyphus/evidence/beato-v1.1-index.md` (this file) | PASS | (pending) |

## BEATO Score Progression

| Pillar | Max | v1.0 | v1.1 | Delta |
|--------|-----|------|------|-------|
| Browser | 25 | 16 | 21 | +5 |
| Email | 20 | 18 | 19 | +1 |
| Text | 20 | 15 | 16 | +1 |
| Office | 25 | 10 | 13 | +3 |
| Audio | 10 | 2 | 2 | 0 |
| **Total** | **100** | **61** | **71** | **+10** |

## Commits (v1.1 Sprint)

| Commit | Message | Tasks |
|--------|---------|-------|
| `ed86935` | feat(rpc): wire SafetyMiddleware into BEATO handler registration | T1 |
| `37b7bbb` | test(rpc): add per-handler auth regression tests | T2 |
| `9f30fe4` | feat(rpc): implement browser.screenshot and browser.close handlers | T5 |
| `fdb8811` | test(browser): extend smoke coverage to all 14 browser RPCs | T6 |
| `30caa63` | docs: add Rust sidecar formal deferral document (T4) | T4 |
| (pending) | docs: Wave 3 + Wave 4 evidence | T7-V, T8, T9-V, T10, T11, T12 |

## Key Decisions

1. **Auth enforcement via SafetyMiddleware wrapping** — zero lines changed in `rpc_safety.go`
2. **Nil guard** (`if s.safety != nil`) prevents panics in existing tests
3. **Handler exclusion list** — health.*, hardening.*, provisioning.*, mobile.*, keystore.*, device.*, invite.*, e2ee.* NOT wrapped
4. **Rust sidecar formally deferred** — no Dockerfile exists, Office ceiling 15/25 without it
5. **AppArmor risk accepted** — MEDIUM risk, 6 compensating controls active
6. **Honest scoring** — 71/100, not inflated to reach 85 target
7. **browser.close is idempotent** — returns success when no session exists
8. **browser.screenshot returns pending** — placeholder when session exists, error when no session

## Files Modified (v1.1 Sprint)

| File | Change | Tasks |
|------|--------|-------|
| `bridge/pkg/rpc/server.go` | SafetyMiddleware field, Config.AdminToken, 23 handler wrapping, 2 browser registrations | T1, T5 |
| `bridge/pkg/rpc/browser.go` | handleBrowserScreenshot + handleBrowserClose (+146 lines) | T5 |
| `bridge/pkg/rpc/auth_integration_test.go` | NEW — 7 auth regression test functions (261 lines) | T2 |
| `tests/test-browser-smoke.sh` | B4-B14 + AUTH enforcement section (+350 lines) | T6 |
| `tests/reports/beato-runtime-report.md` | v1.1 score section appended (+165 lines) | T11 |
| `.sisyphus/evidence/task-4-rust-deferral.md` | NEW — Rust sidecar deferral document | T4 |
| `.sisyphus/evidence/task-10-apparmor-risk.md` | NEW — AppArmor risk acceptance | T10 |
