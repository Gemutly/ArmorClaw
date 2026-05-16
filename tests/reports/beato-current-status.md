# ArmorClaw — BEATO Current Status Report

**Date**: 2026-05-16
**Author**: Atlas (Master Orchestrator)
**HEAD**: `fccfc8f` on `main`
**VPS**: production server (scrubbed)
**Scope**: Honest runtime assessment of all 5 BEATO pillars

---

## Executive Summary

This report is the **single source of truth** for the current state of the BEATO capability pillars (Browser, Email, Audio, Text, Office) as they exist in the ArmorClaw codebase and on the production VPS.

**Key distinction**: The BEATO verification report (`beato-verification-report.md`) scored 100/100 — that score measures **plan completion** (26/26 tasks, 44/44 checkboxes). This report measures **actual runtime state** on the VPS.

### Honest Bottom Line

```
BEATO COVERAGE — Actual Runtime State (2026-05-16)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
B — Browser   ████████░░  ~80%   Deployed, working, some RPCs untested
E — Email     █████░░░░░  ~50%   Code complete, partially deployed, outbox untested on VPS
A — Audio     █░░░░░░░░░  ~10%   Audit only, runtime disabled, deferred to v1.4
T — Text      ██████████  ~95%   Production-ready, 14/14 RPC regression pass
O — Office    ██░░░░░░░░  ~20%   Deployed but crash-looping, document RPC not in bridge image
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Runtime:      █████░░░░░  ~50%   (2/5 pillars fully working, 1 partial, 2 broken/deferred)
Plan Score:   ██████████  100%   (26/26 tasks completed per plan)
```

### Why Plan Score ≠ Runtime Score

| Factor | Impact |
|--------|--------|
| Sidecar-office crash-loop | Office pillar non-functional on VPS |
| Bridge image not rebuilt | 17 new RPC methods committed but not in deployed binary |
| Navchart import blocker | Cannot rebuild bridge image without resolving dependency |
| Audio intentionally deferred | Only audit-complete, not a failure |
| Email outbox untested on VPS | Code works locally, not verified in production |

---

## VPS Container Inventory

| Container | Image | Status | Reality |
|-----------|-------|--------|---------|
| armorclaw (bridge) | REDACTED-REGISTRY/armorclaw:latest | Up (healthy) | Running pre-BEATO image. New RPC methods (document.*, email.queue_*, email.list, email.get, email.retry) are NOT in the deployed binary. |
| armorclaw-jetski | REDACTED-REGISTRY/jetski:beato | Up (healthy) | Working. CDP proxy on port 9222, RPC on 9223. Internal-only network. |
| armorclaw-sidecar-office | REDACTED-REGISTRY/sidecar-office:beato | Crash-looping | `PermissionError: [Errno 13]` reading HMAC secret file. Container restarts every few seconds. |
| armorclaw-conduit | matrixconduit/matrix-conduit:latest | Up | Matrix homeserver. Working normally. |

### Network Topology

```
┌──────────────────────────────────────────────────────────┐
│                    Docker Networks                        │
│                                                          │
│  armorclaw-internal (bridge, internal)                   │
│    ├── armorclaw (bridge)                                 │
│    └── armorclaw-jetski                                   │
│         Ports: 9222 (CDP), 9223 (RPC) — internal only    │
│                                                          │
│  none (network_mode)                                     │
│    └── armorclaw-sidecar-office                           │
│         Socket: /run/armorclaw/sidecar-office.sock        │
│         Status: CRASH-LOOPING (secret file permission)    │
│                                                          │
│  default bridge                                           │
│    ├── armorclaw                                          │
│    └── armorclaw-conduit (port 6167)                      │
└──────────────────────────────────────────────────────────┘
```

### Resource Usage

| Container | CPU | Memory | Limit | % Used |
|-----------|-----|--------|-------|--------|
| armorclaw (bridge) | 0.00% | 21 MiB | 7.7 GiB | 0.27% |
| armorclaw-jetski | 0.05% | 5 MiB | 512 MiB | 0.97% |
| armorclaw-sidecar-office | 0.08% | 117 MiB | 512 MiB | 22.8% |
| armorclaw-conduit | 0.01% | 45 MiB | 7.7 GiB | 0.57% |
| **Total** | **0.14%** | **188 MiB** | — | **2.3% of host** |

RAM available: ~7,070 MB. Disk free: ~73 GB. No OOM kills.

---

## B — Browser (Jetski Sidecar)

### What Works (Deployed + Verified)

| Feature | Status | Evidence |
|---------|--------|----------|
| Jetski container deployed | ✅ Working | `docker ps` shows Up (healthy) |
| CDP proxy operational | ✅ Working | Port 9222 internal, CDP WebSocket proxy active |
| RPC port 9223 accessible | ✅ Working | Jetski status RPC responds |
| `browser.navigate` | ✅ Working | Returns `{status: "running", job_id: "..."}` |
| `browser.list` | ✅ Working | Returns `{count: 7, jobs: [...]}` |
| `browser.status` | ✅ Working | Validates params, returns session status |
| External HTTPS navigation | ✅ Working | `browser.navigate` to `https://example.com` completes |
| No public host ports | ✅ Verified | `PortBindings={}`, no `ports:` in compose |
| Security hardening | ✅ Verified | `read_only`, `cap_drop ALL`, `no-new-privileges`, `mem_limit 512m`, `pids_limit 256` |
| Session lifecycle | ✅ Working | navigate → running → completed → list shows job |
| Network isolation | ✅ Verified | Only bridge and jetski on `armorclaw-internal` network |

### What Doesn't Work

| Issue | Severity | Details |
|-------|----------|---------|
| `browser.click` | ⚠️ Not tested on VPS | Registered in code, never called during smoke test |
| `browser.fill` | ⚠️ Not tested on VPS | Registered in code, never called during smoke test |
| `browser.cancel` | ⚠️ Not tested on VPS | Registered in code, never called during smoke test |
| `browser.wait_for_element` | ⚠️ Not tested on VPS | Registered in code, never called during smoke test |
| `browser.wait_for_captcha` | ⚠️ Not tested on VPS | Registered in code, never called during smoke test |
| `browser.wait_for_2fa` | ⚠️ Not tested on VPS | Registered in code, never called during smoke test |
| `browser.complete` | ⚠️ Not tested on VPS | Registered in code, never called during smoke test |
| `browser.fail` | ⚠️ Not tested on VPS | Registered in code, never called during smoke test |
| `browser.replay_diagnostics` | ⚠️ Not tested on VPS | Registered in code, never called during smoke test |

### What Wasn't Tested

- **BlindFill™ injection** through browser — the PII scrubbing pipeline was verified in unit tests but not in a live browser session
- **NavChart pipeline** — CDP frame → NavChart normalization (6-stage) exists in code but no E2E replay was tested on VPS
- **Concurrent browser sessions** — only single-session smoke tested
- **Agent → Jetski → Browser → Approval** full workflow — never tested end-to-end on VPS

### What's In Code But Not Deployed

- All browser RPC methods listed above ARE in the deployed bridge image (they were part of the pre-BEATO codebase). The navchart blocker only affects future bridge rebuilds, not the current deployed set.

### Browser RPC Methods (12 registered)

| Method | In Deployed Image | Tested on VPS |
|--------|-------------------|---------------|
| `browser.navigate` | ✅ Yes | ✅ Yes |
| `browser.fill` | ✅ Yes | ❌ No |
| `browser.click` | ✅ Yes | ❌ No |
| `browser.status` | ✅ Yes | ✅ Yes |
| `browser.wait_for_element` | ✅ Yes | ❌ No |
| `browser.wait_for_captcha` | ✅ Yes | ❌ No |
| `browser.wait_for_2fa` | ✅ Yes | ❌ No |
| `browser.complete` | ✅ Yes | ❌ No |
| `browser.fail` | ✅ Yes | ❌ No |
| `browser.list` | ✅ Yes | ✅ Yes |
| `browser.cancel` | ✅ Yes | ❌ No |
| `browser.replay_diagnostics` | ✅ Yes | ❌ No |

**Browser Coverage: 3/12 RPC methods verified on VPS (25%). All 12 registered and accessible.**

---

## E — Email

### What Works (Deployed + Verified)

| Feature | Status | Evidence |
|---------|--------|----------|
| Email approval RPC methods | ✅ Working | `email_approval_status` → `{pending_count:0, timeout_s:300}` |
| `email.list_pending` | ✅ Working | Returns `{approvals:[], count:0}` |
| `approve_email` | ✅ Working | Validates `approval_id` parameter |
| `deny_email` | ✅ Working | Validates `approval_id` parameter |
| HITL approval flow | ✅ Wired | Approval/deny handlers in `email_approval.go` |
| Email team routing | ✅ Code complete | `RoutingRuleStore` with SQLite backing, pattern matching |
| EmailDispatcher | ✅ Code complete | Template-based dispatch engine |
| IMAP ingest | ✅ Code complete | Email ingestion from IMAP servers |
| SMTP client | ✅ Code complete | Outbound email via SMTP |
| Gmail/Outlook adapters | ✅ Code complete | Platform-specific adapters |

### What Doesn't Work

| Issue | Severity | Details |
|-------|----------|---------|
| `email.queue_status` | ❌ Not in deployed image | New RPC method committed but blocked by navchart |
| `email.get` | ❌ Not in deployed image | New RPC method committed but blocked by navchart |
| `email.list` | ❌ Not in deployed image | New RPC method committed but blocked by navchart |
| `email.retry` | ❌ Not in deployed image | New RPC method committed but blocked by navchart |
| Outbox persistence | ❌ Not in deployed image | `OutboxStore` with go-sqlcipher committed but not in bridge binary |
| Postfix MTA | ❌ Not deployed | No local mail transfer agent on VPS |
| DNS (MX/SPF/DKIM) | ❌ Not configured | Domain DNS records not set up |

### What Wasn't Tested

- **Email end-to-end flow** — no test of: email arrives → queue → approval → dispatch
- **Outbox state machine** — 8 statuses (queued → awaiting_approval → approved → sending → sent, or retry_wait → dead_letter) tested locally with 11 unit tests but never verified on VPS
- **Concurrent email operations** — outbox concurrent access tested locally but not under VPS load
- **Email template rendering** — template engine exists but never tested with real data
- **SMTP delivery** — outbound email code exists but never sent a real email

### What's In Code But Not Deployed

| Component | File | Status |
|-----------|------|--------|
| `OutboxStore` | `bridge/pkg/email/outbox.go` | Committed, 11 tests pass locally |
| `email.queue_status` handler | `bridge/pkg/rpc/email_queue.go` | Committed, blocked by navchart |
| `email.get` handler | `bridge/pkg/rpc/email_queue.go` | Committed, blocked by navchart |
| `email.list` handler | `bridge/pkg/rpc/email_queue.go` | Committed, blocked by navchart |
| `email.retry` handler | `bridge/pkg/rpc/email_queue.go` | Committed, blocked by navchart |
| Outbox wired into approve/deny | `bridge/pkg/rpc/email_approval.go` | Committed, blocked by navchart |

**Email Coverage: 4/8 RPC methods accessible on VPS (50%). Outbox and queue completely untested in production.**

---

## A — Audio (Voice)

### What Works

| Feature | Status | Evidence |
|---------|--------|----------|
| Audio audit report | ✅ Complete | `tests/reports/audio-capability-audit.md` — 109 lines |
| Voice RPC methods registered | ✅ Working | `voice.start_session`, `voice.stop_session`, `voice.status` respond with "voice not enabled" |
| Voice manager architecture | ✅ Code complete | `bridge/pkg/voice/` — manager, budget, security, PCM routing |
| WebRTC package | ✅ Code complete | `bridge/pkg/webrtc/` — SessionManager, TokenManager, Engine |
| TURN relay package | ✅ Code complete | `bridge/pkg/turn/` — credential provisioning |
| Budget tracking | ✅ Tested | Cost tracking for STT/TTS API calls |
| Security enforcer | ✅ Tested | Audit + TTL manager for voice sessions |

### What Doesn't Work

| Issue | Severity | Details |
|-------|----------|---------|
| Voice manager runtime | ❌ Disabled | `voiceMgr` nil in `cmd/bridge/main.go` — commented out by design |
| STT (Speech-to-Text) | ❌ Not functional | Stub implementations only, no live provider credentials |
| TTS (Text-to-Speech) | ❌ Not functional | Stub implementations only, no live provider credentials |
| VAD (Voice Activity Detection) | ❌ Not functional | Energy-based VAD exists but not connected to STT pipeline |
| WebRTC voice integration | ❌ Fails | `test-webrtc-voice-integration.sh` — bridge config error |

### What Wasn't Tested

- Nothing beyond the audit was tested — Audio is intentionally deferred to v1.4
- All voice tests use `GATED_EXPECTED` (skip unless voice explicitly enabled)

### Intentional Deferral

Audio is **not a failure** — it was scoped as "audit only" in the BEATO plan. The voice stack is ~30% complete architecturally with clear v1.4 activation recommendations documented in the audit report.

**Audio Coverage: Audit 100%, Runtime 0% (by design). Audio pillar score: 5/25 capability.**

---

## T — Text (Matrix + Secretary)

### What Works (Deployed + Verified)

| Feature | Status | Evidence |
|---------|--------|----------|
| Matrix login/sync/rooms | ✅ Production | `@bridge:VPS` connected, logged_in=true |
| `bridge.status` | ✅ Working | Returns `{enabled:false, status:"not_configured"}` |
| `health.check` | ✅ Working | Returns `{status:"degraded", bridge:"ok"}` |
| `ai.chat` | ✅ Working | Validates "Messages cannot be empty" |
| `matrix.status` | ✅ Working | Returns `{enabled:true, connected:true, logged_in:false}` |
| `keystore.sealed` | ✅ Working | Returns "Feature disabled: zero_trust_keystore" (expected) |
| `keystore.session_status` | ✅ Working | Returns "Feature disabled" (expected) |
| Studio lifecycle | ✅ 8 PASS | create→get→list→spawn→stop→delete verified |
| Secretary methods | ✅ 17+ methods | 146 RPC methods registered in code |
| Artifact protocol | ✅ Deployed | 4 `secretary.artifact_*` methods working |
| Approval delegation | ✅ Deployed | 3 `secretary.*_delegation` methods working |
| Workflow templates | ✅ Verified | `list_templates` returns data |
| Restart recovery | ✅ 4s | Well under 10s target |
| WSS reconnect | ✅ 5 PASS | 3 rapid WebSocket reconnection cycles |
| EventBus streaming | ✅ PASS | WebSocket events proven |
| Trust layer (PII detection) | ✅ PASS | PII detection + approval flow verified |
| RPC regression (14 methods) | ✅ 14/14 | All respond correctly |

### What Doesn't Work

| Issue | Severity | Details |
|-------|----------|---------|
| `device.list` | ⚠️ Pre-existing | Returns "database is closed" — known bug, not BEATO-related |
| `invite.list` | ⚠️ Pre-existing | Returns "database is closed" — known bug, not BEATO-related |
| Matrix credentials | ⚠️ Intermittent | Token expires after Docker redeploy, requires re-login |

### What Wasn't Tested

- **All 146 RPC methods individually** — only 14 were regression-tested
- **Full secretary workflow** (create→run→complete→artifact) with real AI provider
- **Agent container spawning** from secretary workflow
- **Learned skills extraction** from container event logs
- **NavChart injection** into browser steps
- **Concurrent secretary workflows** (10 parallel RPC + 3 WS tested, not full workflows)

**Text Coverage: ~95%. The strongest pillar. Production-ready with minor pre-existing issues.**

---

## O — Office (Document Pipeline)

### What Works (In Code)

| Feature | Status | Evidence |
|---------|--------|----------|
| Python MarkItDown sidecar | ✅ Code complete | XLSX, PPTX, MSG, XLS, DOC, PPT — 61 tests pass locally |
| 3-layer routing (`RouteExtractText`) | ✅ Code complete | Native bypass → sidecar → strict drop |
| YARA CDR | ✅ Code complete | `$_mz` and `$_pe` warnings fixed in `yara_rules.yar` |
| Document RPC handlers | ✅ Code complete | `document.extract_text`, `document.status`, `document.list_jobs` in `document.go` |
| HMAC-SHA256 auth | ✅ Verified | Token format `{request_id}:{timestamp}:{operation}:{hmac_hex}` validated in T2.3 |
| XLSX extraction | ✅ Verified locally | gRPC test extracted "Hello ArmorClaw" from test XLSX |
| Split-Storage RAG | ✅ Code complete | Document chunking with separate embeddings |
| Dynamic sidecar spawning | ✅ Code complete | SkillRouter — skill→image mapping + lifecycle |

### What Doesn't Work

| Issue | Severity | Root Cause | Details |
|-------|----------|------------|---------|
| **Sidecar-office crash-loop** | 🔴 Critical | Permission error | `PermissionError: [Errno 13] Permission denied: '/run/armorclaw/secrets/office-hmac'` in `interceptor.py:34` during `load_shared_secret()`. File mode 644, uid 10001. Test container reads it fine. Actual sidecar container fails. |
| `document.extract_text` on VPS | ❌ Not in deployed image | Navchart blocker | Committed in code but bridge image not rebuilt |
| `document.status` on VPS | ❌ Not in deployed image | Navchart blocker | Same as above |
| `document.list_jobs` on VPS | ❌ Not in deployed image | Navchart blocker | Same as above |

### Sidecar Crash Root Cause Analysis

**Symptom**: `armorclaw-sidecar-office` container crashes immediately on startup with:
```
PermissionError: [Errno 13] Permission denied: '/run/armorclaw/secrets/office-hmac'
```

**Investigation performed**:
1. ✅ File exists at the expected path, mode `100644`, owned by uid `10001`
2. ✅ `chmod 644` applied — file is world-readable
3. ✅ Test container (`docker run --rm` with same image + volume) CAN read the file
4. ✅ Container recreated without `--read-only` flag — still fails
5. ✅ Container recreated with explicit `--user root` — still fails
6. ❌ Actual sidecar container still gets `PermissionError`

**Hypotheses remaining**:
- **Race condition**: Bridge may recreate/overwrite the secret file after sidecar starts, with restrictive permissions
- **SELinux/AppArmor**: Docker security labels on the volume may differ between `docker run` (test) and compose-managed container
- **Volume mount timing**: Bind mount from named volume may not be available at Python startup

**Resolution needed**: SSH into VPS, check `dmesg`/`audit.log` for AVC denials, check if bridge recreates the secret, add startup delay or health-check dependency.

### What Wasn't Tested

- **Document extraction on VPS** — sidecar is crash-looping, no documents processed
- **YARA CDR in production** — rules fixed in code, not in deployed image
- **Split-Storage RAG** — code exists, never tested end-to-end
- **PPTX/MSG/XLS/DOC/PPT formats** — only XLSX was locally verified
- **Threshold streaming** (10MB boundary) — tested locally, not on VPS
- **TTL recycling** (server exits after 50 requests) — not tested

**Office Coverage: Code 80%, Deployed 0%. The only fully broken pillar.**

---

## Blockers

### Blocker 1: Sidecar-Office Crash-Loop (CRITICAL)

- **Pillar**: O (Office)
- **Impact**: Document processing completely non-functional on VPS
- **Root cause**: `PermissionError` reading HMAC secret file
- **Resolution path**: Debug secret file lifecycle (who creates it, when, with what permissions). Check SELinux/AppArmor. Add startup delay.
- **Effort**: 2-4 hours

### Blocker 2: Bridge Image Rebuild Blocked (HIGH)

- **Pillar**: E (Email), O (Office), future B (Browser) enhancements
- **Impact**: 17 new RPC methods committed but not in deployed bridge binary
- **Root cause**: `bridge/pkg/rpc/browser.go:13` imports `github.com/armorclaw/jetski/navchart` but no `go.mod replace` directive exists. The bridge Dockerfile copies only `bridge/` context and cannot access `../jetski/`.
- **Resolution path**: Add `replace github.com/armorclaw/jetski/navchart => ../../jetski/navchart` to `bridge/go.mod` OR vendor the navchart types into the bridge package.
- **Effort**: 1-2 hours
- **New RPC methods blocked**:
  - `document.extract_text`, `document.status`, `document.list_jobs` (3 methods)
  - `email.queue_status`, `email.get`, `email.list`, `email.retry` (4 methods)
  - Future: any method requiring navchart types

### Blocker 3: Device/Invite DB Lock (LOW — Pre-existing)

- **Pillar**: T (Text)
- **Impact**: `device.list` and `invite.list` return "database is closed"
- **Root cause**: Unknown — pre-existing issue, not BEATO-related
- **Resolution path**: Investigate DB connection lifecycle in device/invite handlers
- **Effort**: Unknown

---

## Commit History (BEATO Work)

| Commit | Message | Impact |
|--------|---------|--------|
| `b9ee6ab` | feat(beato): add RPC safety middleware + fix Jetski Dockerfile | Wave 0 — safety framework |
| `5c55a5e` | feat(beato): add Docker Compose overlay for Jetski deployment | Wave 1 — Jetski deployment |
| `2077b0f` | fix(jetski): resolve crash-loop from hardcoded path + broken logger | Wave 1 — Jetski fix |
| `f63e01b` | fix(yara): prefix unreferenced strings with `$_` | Wave 2 — YARA fix |
| `ab8b47e` | feat(beato): register document RPC methods with 3-layer routing | Wave 2 — Document RPC |
| `eb7468f` | fix(docker): update bridge Dockerfile to golang:1.25-bookworm | Wave 2 — Dockerfile |
| `32c85d9` | feat(beato): register email queue RPC methods | Wave 3 — Email queue |
| `9e26d9a` | feat(beato): add outbound email queue with SQLite backing | Wave 3 — Outbox store |
| `68b027d` | feat(beato): wire outbox into email approval flow | Wave 3 — Approval wiring |
| `3739906` | docs(beato): add audio capability audit report | Wave 4 — Audio audit |
| `07d6f30` | docs(beato): final BEATO verification report — 100/100 coverage | Wave 5 — Verification |
| `fccfc8f` | docs: update BEATO progress reports and baseline documentation | Wave 5 — Reports |

---

## Baseline Comparison

| Metric | Before BEATO (74%) | Plan Claim (100%) | Actual Runtime (~50%) |
|--------|-------------------|-------------------|----------------------|
| Browser | 0% (undeployed) | 100% | ~80% (deployed, partially tested) |
| Email | Partial (approval only) | 100% | ~50% (approval working, outbox not deployed) |
| Audio | No audit | 100% | ~10% (audit only, deferred) |
| Text | 100% | 100% | ~95% (production, minor issues) |
| Office | Partial (code only) | 100% | ~20% (code complete, VPS broken) |
| VPS containers | 2 (bridge, conduit) | 4 (+jetski, +sidecar) | 3.5 (jetski working, sidecar crash-looping) |
| RPC methods | 129 | 146 | ~130 in deployed binary (16 blocked) |
| RAM headroom | ~7 GB | ~7 GB | ~7 GB (no degradation) |

---

## Recommendations

### Priority 1: Fix Sidecar Crash-Loop (1-2 hours)
This unblocks the entire Office pillar. SSH to VPS, debug the secret file lifecycle.

### Priority 2: Fix Navchart Dependency (1-2 hours)
This unblocks bridge image rebuild, which deploys 17 new RPC methods. Add `go.mod replace` directive or vendor types.

### Priority 3: Test Remaining Browser RPCs (2-3 hours)
12 browser RPC methods registered, only 3 tested. Run `browser.click`, `browser.fill`, `browser.cancel`, etc. through VPS smoke tests.

### Priority 4: Deploy Postfix + DNS for Email (3-5 days)
Email needs a real MTA and DNS configuration. This is infrastructure work, not code.

### Priority 5: Audio v1.4 Activation (Weeks)
Voice stack activation requires STT/TTS provider credentials and pipeline wiring. Not urgent.

---

## Files Modified During BEATO Work

### New Files

| File | Purpose |
|------|---------|
| `bridge/pkg/rpc/document.go` | Document RPC handlers + 3-layer routing (297 lines) |
| `bridge/pkg/rpc/document_test.go` | Document RPC tests |
| `bridge/pkg/rpc/email_queue.go` | Email queue RPC handlers (199 lines) |
| `bridge/pkg/rpc/email_queue_test.go` | Email queue tests |
| `bridge/pkg/email/outbox.go` | Outbound email queue with go-sqlcipher |
| `bridge/pkg/email/outbox_test.go` | Outbox CRUD + transition tests (11 tests) |
| `bridge/pkg/rpc/METHOD_AUDIT.md` | RPC method registration audit |
| `deploy/docker-compose.beato.yml` | Jetski compose overlay |
| `deploy/env/beato.env.example` | Environment template |
| `tests/reports/audio-capability-audit.md` | Audio capability audit (109 lines) |
| `tests/test-browser-smoke-beato.sh` | Browser smoke test |

### Modified Files

| File | Change |
|------|--------|
| `bridge/pkg/rpc/server.go` | +17 RPC method registrations, sidecar/outbox fields |
| `bridge/pkg/rpc/email_approval.go` | Outbox persistence wired into approve/deny |
| `bridge/configs/yara_rules.yar` | `$_mz`/`$_pe` fix |
| `bridge/pkg/rpc/discovery_test.go` | Method count updated (129→146) |
| `bridge/pkg/rpc/replay_gating_test.go` | Method count updated |
| `Dockerfile` | golang:1.25-bookworm |

---

*Report generated by Atlas (Master Orchestrator) — BEATO Current Status Report*
*This report supersedes `beato-verification-report.md` for runtime accuracy.*
*For plan completion tracking, see the verification report.*
