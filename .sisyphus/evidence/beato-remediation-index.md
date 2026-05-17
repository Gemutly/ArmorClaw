# BEATO Remediation Evidence Index

Date: 2026-05-16
Sprint: Blocker Remediation
VPS: 5.183.11.149

| Task | Evidence Files | Pass/Fail | Notes |
|------|---------------|-----------|-------|
| T1 | task-1-navchart-vendor.txt, task-1-no-jetski-refs.txt | PASS | 3 files vendored (multi_tab.go, replay.go, diagnostics.go), 0 jetski/navchart refs in Go source, 122 browser tests pass |
| T2 | task-2-hmac-debug.md | PASS | H2 confirmed: parent dir /run/armorclaw is mode 770/10001:10001, blocks traversal for uid 0 without CAP_DAC_OVERRIDE |
| T3 | task-3-bridge-build.txt, task-3-vps-bridge-status.txt | PASS | Docker build succeeded (82.4s), image pushed as mikegemut/armorclaw:beato-fix, bridge RPC responds on VPS |
| T4 | task-4-no-bridge-chown.txt | PASS | Bridge never calls os.Chown for HMAC. init-service approach used instead. HMAC provisioned with correct ownership |
| T5 | task-5-rpc-verification.txt, task-5-auth-gate.txt | PASS (with caveat) | 16/16 RPCs registered. Auth middleware NOT enforcing tokens (security finding, not a registration failure) |
| T6 | task-6-office-e2e.txt, task-6-mime-mismatch.txt, task-6-corrupt-file.txt | PARTIAL | XLSX extraction FAILS (no Rust sidecar). MIME mismatch test PASS. Corrupt file test PASS. 2/3 negative tests pass |
| T7 | task-7-browser-coverage.txt | PARTIAL | 10/12 registered. browser.screenshot and browser.close return "method not found" |
| T8 | task-8-email-pipeline.txt, task-8-queue-persistence.txt | PASS | Outbox store wired via main.go init, SQLite at /var/lib/armorclaw/email-outbox.db, all 6 email RPCs respond correctly |
| T9 | task-9-report-exists.txt | PASS | This report |

## Honest BEATO Score: 61/100

| Pillar | Score | Max |
|--------|-------|-----|
| Browser | 16 | 25 |
| Email | 18 | 20 |
| Text | 15 | 20 |
| Office | 10 | 25 |
| Audio | 2 | 10 |
| **Total** | **61** | **100** |

## Previous Report Comparison

Previous report: 100/100. This was inaccurate.
Current honest score: 61/100.
Gap: 39 points from inflated claims (Office 25 vs 10, Browser 25 vs 16, Audio 10 vs 2, Text 20 vs 15).
