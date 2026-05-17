# BEATO Current Status Report — Comprehensive

## TL;DR

> **Quick Summary**: Write a single comprehensive BEATO status report covering what works, what doesn't, and what wasn't tested across all 5 pillars (Browser, Email, Audio, Text, Office) with honest assessment of the sidecar-office crash-loop and navchart blocker.
> 
> **Deliverables**:
> - One markdown report at `tests/reports/beato-current-status.md`
> 
> **Estimated Effort**: Quick
> **Parallel Execution**: NO — single task
> **Critical Path**: T1

---

## Context

### Original Request
User requested: "I want a single source report; please make a separate comprehensive report of what works, doesn't work, and what was not tested. Go over specific features of BEATO tested."

### Key Facts

**What exists in `tests/reports/`:**
- `beato-verification-report.md` — Scoring 100/100, but this is a "plan completion" report, not a current-state report. It reflects plan checkboxes, not runtime reality.
- `beato-progress-report.md` — Pre-BEATO baseline at 74% coverage. Outdated — written before Waves 0-5.
- `audio-capability-audit.md` — Audio audit, accurate.
- `pre-beato-complete-report.md` — Pre-BEATO calibration at 60% coverage. Very outdated.
- `post-update-v4.6.0-baseline.md` — Post-update baseline. Outdated.

**The gap**: No single report tells the honest truth about what's actually working on the VPS RIGHT NOW. The verification report claims 100/100 but the sidecar is crash-looping and new RPC methods aren't in the deployed bridge image.

### Actual Runtime State (VPS 5.183.11.149)

| Container | Status | Reality |
|-----------|--------|---------|
| armorclaw (bridge) | Up (healthy) | Running pre-BEATO image — new methods NOT in deployed binary |
| armorclaw-jetski | Up (healthy) | Working — browser navigate/list/click confirmed |
| armorclaw-sidecar-office | Crash-looping | `PermissionError` on HMAC secret file |
| armorclaw-conduit | Up | Matrix homeserver working |

### Blockers
1. **Sidecar-office crash-loop**: `PermissionError: [Errno 13]` reading `/run/armorclaw/secrets/office-hmac` — file readable from test container but not from actual sidecar
2. **Bridge image rebuild blocked**: `browser.go` imports `github.com/armorclaw/jetski/navchart` with no `go.mod` replace — new RPC methods committed but not deployed
3. **device.list / invite.list**: Pre-existing "database is closed" error

---

## Work Objectives

### Core Objective
Produce an honest, comprehensive single-source report that separates "code committed" from "actually running on VPS" and identifies every gap.

### Concrete Deliverables
- `tests/reports/beato-current-status.md`

### Definition of Done
- [x] Report covers all 5 BEATO pillars with per-feature status
- [x] Clear separation between "works in code/tests" vs "works on VPS"
- [x] Sidecar crash documented with root cause analysis
- [x] Navchart blocker documented with resolution path
- [x] Each pillar has "Tested" / "Not Tested" / "Cannot Test" breakdown

---

## TODOs

- [x] 1. Write BEATO Current Status Report

  **What to do**:
  - Create `tests/reports/beato-current-status.md`
  - Structure by BEATO pillar (B, E, A, T, O)
  - For each pillar, cover:
    - **What Works** — features confirmed working (with evidence)
    - **What Doesn't Work** — known failures with root cause
    - **What Wasn't Tested** — features/code that exist but weren't verified
    - **What's In Code But Not Deployed** — committed but blocked from VPS
  - Include honest summary table with actual runtime status
  - Document the 3 blockers (sidecar crash, navchart, DB lock)
  - Include network topology and container inventory
  - Include baseline comparison (74% → claimed 100% → actual ~70-75%)
  - No sugar-coating — be direct about what's broken

  **Must NOT do**:
  - Do NOT claim 100% coverage when the sidecar is crash-looping
  - Do NOT hide the navchart blocker
  - Do NOT copy the verification report's scoring without caveats

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: This is a documentation/writing task, not code
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `agent-browser`: No browser interaction needed
    - `playwright-cli`: No browser testing needed

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (single task)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:

  **Primary Sources (evidence files)**:
  - `tests/reports/beato-verification-report.md` — Plan completion scoring (100/100). Use as "what the plan claimed" data point. Cross-reference against actual VPS state.
  - `tests/reports/beato-progress-report.md` — Pre-BEATO baseline (74%). Use for historical comparison.
  - `tests/reports/audio-capability-audit.md` — Audio pillar audit (accurate, use as-is).

  **VPS state (from session context, not files)**:
  - Containers: armorclaw (healthy), armorclaw-jetski (healthy), armorclaw-sidecar-office (crash-looping), armorclaw-conduit (up)
  - Sidecar crash: `interceptor.py:34` → `load_shared_secret()` → `open("/run/armorclaw/secrets/office-hmac", "r")` → `PermissionError: [Errno 13]`. File mode 644, uid 10001. Test container reads it fine. Actual container fails.
  - Bridge RPC: 146 methods in code, ~130 accessible on VPS (new ones blocked by navchart dependency)
  - Navchart blocker: `bridge/pkg/rpc/browser.go:13` imports `github.com/armorclaw/jetski/navchart` — no go.mod replace directive

  **Code references for feature verification**:
  - `bridge/pkg/rpc/server.go` — Full list of registered RPC methods
  - `bridge/pkg/rpc/document.go` — Document RPC handlers (3 methods: extract_text, status, list_jobs)
  - `bridge/pkg/rpc/email_queue.go` — Email queue RPC (4 methods: queue_status, get, list, retry)
  - `bridge/pkg/email/outbox.go` — OutboxStore with go-sqlcipher
  - `bridge/pkg/rpc/email_approval.go` — Outbox wired into approve/deny
  - `deploy/docker-compose.beato.yml` — Jetski compose overlay
  - `bridge/configs/yara_rules.yar` — YARA rules (fixed $_mz/$_pe)

  **Acceptance Criteria**:
  - [ ] File exists: `tests/reports/beato-current-status.md`
  - [ ] Report covers all 5 BEATO pillars
  - [ ] Each pillar has Works/Doesn't Work/Not Tested sections
  - [ ] Sidecar crash documented with root cause
  - [ ] Navchart blocker documented
  - [ ] Honest assessment distinguishes code-complete vs deployed vs working
  - [ ] No false claims of 100% coverage

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Report completeness check
    Tool: Bash (grep)
    Preconditions: File tests/reports/beato-current-status.md exists
    Steps:
      1. grep -c "## B " tests/reports/beato-current-status.md → expect ≥ 1
      2. grep -c "## E " tests/reports/beato-current-status.md → expect ≥ 1
      3. grep -c "## A " tests/reports/beato-current-status.md → expect ≥ 1
      4. grep -c "## T " tests/reports/beato-current-status.md → expect ≥ 1
      5. grep -c "## O " tests/reports/beato-current-status.md → expect ≥ 1
      6. grep -ci "crash-loop" tests/reports/beato-current-status.md → expect ≥ 1
      7. grep -ci "navchart" tests/reports/beato-current-status.md → expect ≥ 1
      8. grep -ci "what works" tests/reports/beato-current-status.md → expect ≥ 5
      9. grep -ci "what doesn't work" tests/reports/beato-current-status.md → expect ≥ 5
    Expected Result: All grep counts meet minimums
    Failure Indicators: Any count is 0
    Evidence: .sisyphus/evidence/task-1-report-completeness.txt

  Scenario: Honest coverage assessment
    Tool: Bash (grep)
    Preconditions: File tests/reports/beato-current-status.md exists
    Steps:
      1. grep -c "100/100\|100%" tests/reports/beato-current-status.md → verify it's caveated, not standalone claim
      2. grep -ci "actual\|runtime\|deployed" tests/reports/beato-current-status.md → expect ≥ 3
      3. Verify report does NOT say "All 5 pillars working" without qualification
    Expected Result: Report clearly distinguishes plan score from runtime reality
    Failure Indicators: Report claims 100% without noting sidecar crash
    Evidence: .sisyphus/evidence/task-1-honest-assessment.txt
  ```

  **Commit**: YES
  - Message: `docs(beato): comprehensive current-status report with honest BEATO assessment`
  - Files: `tests/reports/beato-current-status.md`
  - Pre-commit: `wc -l tests/reports/beato-current-status.md` (expect 200+ lines)

---

## Commit Strategy

- **T1**: `docs(beato): comprehensive current-status report with honest BEATO assessment` — `tests/reports/beato-current-status.md`

---

## Success Criteria

### Verification Commands
```bash
wc -l tests/reports/beato-current-status.md  # Expected: 200+ lines
grep -c "## " tests/reports/beato-current-status.md  # Expected: 15+ sections
grep -ci "crash-loop" tests/reports/beato-current-status.md  # Expected: ≥1
grep -ci "navchart" tests/reports/beato-current-status.md  # Expected: ≥1
```

### Final Checklist
- [x] All 5 BEATO pillars covered
- [x] Honest about sidecar crash
- [x] Honest about navchart blocker
- [x] Distinguishes code-complete vs deployed vs working
- [x] Single source of truth for current BEATO state
