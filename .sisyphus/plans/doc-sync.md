# Full Codebase Doc Sync — armorclaw.md + license-system.md

## TL;DR

> **Quick Summary**: Sync /doc/ markdown files with current codebase state. Primary focus: armorclaw.md (18 stale references to deleted blindfill code, outdated counts). Secondary: license-system.md (missing team features).
> 
> **Deliverables**:
> - Updated `doc/armorclaw.md` — all blindfill code-path references removed, counts corrected, missing features added
> - Updated `doc/license-system.md` — team-aware enforcement documented (code-verified only)
> - Verification that ArmorChat.md, client-applications.md, sidecar-pipeline.md are accurate (source-backed)
> - Git push to remote after all commits
> 
> **Estimated Effort**: Medium (10 tasks, mostly text edits)
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: T1 → T2 → T3 → T8 → T10 → push

---

## Context

### Original Request
"git add; git commit; git push; update /doc/ folder's markdown files" → User chose "Full codebase sync" when asked about scope.

### Interview Summary
**Key Discussions**:
- User wants ALL doc files verified against current codebase state
- Last full doc refresh was 14 commits ago (695bdf0)
- Some files already updated during arch-cleanup work (voice-stack.md, agent-runtime.md, communication-infra.md) — those are CLEAN

**Research Findings**:
- `rust-vault/src/blindfill/` fully deleted (commit 1563260) — armorclaw.md still references it 10+ times
- EventPublisher removed from matrix adapter (commit 8c3ce69)
- Rust vault tests: 33 (doc says 58)
- Bridge pkg count: 67 (doc says 60)
- Bridge internal count: 17 (doc says 19)
- main.go: 3527 lines (doc says 3503)
- RPC methods: 89 confirmed (doc says 89 — CORRECT)
- Several missing features: self-hosted mode, EventBus panic recovery, RegisterBridgeHandler

### Metis Review
**Identified Gaps** (addressed in-plan):
- Session at 50-descendant limit — all verification must be in-process
- armorclaw.md has the most references (18 discrepancies) — needs careful sequential editing
- 3 files recently updated are CLEAN — must NOT be modified

---

## Work Objectives

### Core Objective
Fix all stale data in /doc/ markdown files to accurately reflect current codebase state after arch-cleanup.

### Concrete Deliverables
- `doc/armorclaw.md`: Remove blindfill code references, update counts, add missing features
- `doc/license-system.md`: Add team-aware enforcement section
- Verification: ArmorChat.md, client-applications.md, sidecar-pipeline.md accuracy confirmed

### Definition of Done
- [x] Zero references to `rust-vault/src/blindfill/` in any doc file
- [x] All test/package/line counts match current codebase
- [x] `cargo test --lib` count (33) matches doc
- [x] Bridge package counts (67 pkg, 17 internal) match doc
- [x] Self-hosted mode and EventBus panic recovery documented in armorclaw.md

### Must Have
- All blindfill code-path references removed from armorclaw.md
- Accurate test counts, package counts, line counts
- Self-hosted deployment mode documented
- EventBus dual-bus architecture referenced in armorclaw.md

### Must NOT Have (Guardrails)
- Do NOT touch doc/voice-stack.md (just updated, verified clean)
- Do NOT touch doc/agent-runtime.md (just updated, verified clean)
- Do NOT touch doc/communication-infra.md (just updated, verified clean)
- Do NOT add new doc files — only update existing ones
- Do NOT reorganize section structure — fix in-place
- Do NOT remove Go BlindFill documentation (`bridge/pkg/pii/`) — that's still active
- Do NOT change RPC method count — 89 is confirmed accurate

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: N/A (doc-only, no code tests)
- **Automated tests**: None (markdown validation)
- **Framework**: grep-based verification

### QA Policy
Every task includes agent-executed QA via grep/read commands.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately - armorclaw.md fixes):
├── Task 1: Remove blindfill directory/code references from armorclaw.md [quick]
├── Task 2: Update test/package/line counts in armorclaw.md [quick]
├── Task 3: Update Rust Vault section — remove vestigial language [quick]
└── Task 4: Add missing features to armorclaw.md (self-hosted, EventBus, RegisterBridgeHandler) [quick]

Wave 2 (After Wave 1 - other doc files):
├── Task 5: Update license-system.md with team-aware enforcement [quick]
├── Task 6: Verify ArmorChat.md accuracy [quick]
└── Task 7: Verify client-applications.md + sidecar-pipeline.md accuracy [quick]

Wave 3 (After Wave 2 - final verification):
├── Task 8: Cross-file grep verification — zero blindfill code references remain [quick]
├── Task 9: Count verification — all numbers match codebase [quick]
└── Task 10: Final commit with all doc changes [quick]

Wave FINAL (After ALL tasks — in-process only):
├── F1: grep for "blindfill/placeholder" and "blindfill/cdp_interceptor" across all doc files
├── F2: Verify all counts match codebase
└── F3: Verify no unintended changes to voice-stack.md, agent-runtime.md, communication-infra.md
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| T1 | - | T8 |
| T2 | - | T9 |
| T3 | T1 | T8 |
| T4 | - | T8 |
| T5 | - | - |
| T6 | - | - |
| T7 | - | - |
| T8 | T1, T3, T4 | T10 |
| T9 | T2 | T10 |
| T10 | T8, T9 | F1-F3 |

---

## TODOs

- [x] 1. Remove blindfill directory/code references from armorclaw.md

  **What to do**:
  - Line 25: Change "Modify PII injection / BlindFill" row — remove `rust-vault/src/blindfill/placeholder.rs` reference. Change to: `bridge/pkg/pii/` and Jetski sidecar reference only
  - Line 284: Remove `│   │   ├── blindfill/` entry from directory tree. That directory no longer exists.
  - Line 445: In `pkg/secretary/` description, remove "blindfill" from the list. The secretary package doesn't contain blindfill — that's in `pkg/pii/`.
  - Lines 939, 952-954: In "Relationship to Jetski Browser Sidecar" section, remove all references to CdpInterceptor and the blindfill module being "vestigial" or "retained for test coverage". Replace with accurate statement: blindfill module was deleted in v0.9.0 (commit 1563260), superseded by Jetski CDP proxy.
  - Lines 1205, 1211: In "BlindFill Placeholder System" section, remove references to `rust-vault/src/blindfill/placeholder.rs` and `rust-vault/src/blindfill/cdp_interceptor.rs`. These files no longer exist.
  - Line 1079: Remove `pub cdp_enabled: bool` from VaultConfig struct — blindfill CDP config is gone.

  **Must NOT do**:
  - Do NOT remove Go BlindFill documentation (bridge/pkg/pii/ is still active)
  - Do NOT remove the BlindFill concept — only the Rust vault code-path references

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T2, T4)
  - **Parallel Group**: Wave 1
  - **Blocks**: T8
  - **Blocked By**: None

  **References**:
  - `doc/armorclaw.md` lines 25, 284, 445, 939, 952-954, 1079, 1205, 1211 — all contain stale blindfill references
  - Commit `1563260` — "refactor(vault): remove vestigial blindfill module"
  - `rust-vault/src/lib.rs` — verify no blindfill module declaration

  **Acceptance Criteria**:
  - [ ] `grep -c "blindfill/" doc/armorclaw.md` returns 0 (no file-path references)
  - [ ] `grep -c "CdpInterceptor" doc/armorclaw.md` returns 0
  - [ ] Context Routing table (line 25) references only `bridge/pkg/pii/` and Jetski
  - [ ] Directory tree no longer shows `blindfill/` under rust-vault

  **QA Scenarios:**
  ```
  Scenario: No blindfill file-path references remain
    Tool: Bash (grep)
    Steps:
      1. grep -rn "blindfill/" doc/armorclaw.md
      2. Assert: zero matches
    Expected Result: Empty grep output
    Evidence: .sisyphus/evidence/task-1-no-blindfill-paths.txt

  Scenario: No CdpInterceptor references remain
    Tool: Bash (grep)
    Steps:
      1. grep -rn "CdpInterceptor" doc/armorclaw.md
      2. Assert: zero matches
    Expected Result: Empty grep output
    Evidence: .sisyphus/evidence/task-1-no-cdpinterceptor.txt
  ```

  **Commit**: YES (groups with T2-T4)
  - Message: `docs(armorclaw): sync with codebase — remove blindfill references, update counts`
  - Files: `doc/armorclaw.md`

- [x] 2. Update test/package/line counts in armorclaw.md

  **What to do**:
  - Line 245: Change "3,503 lines" to "3,527 lines" for bridge/cmd/bridge/main.go
  - Line 244: Change "(60 packages)" to "(67 packages)" for bridge/pkg/
  - Line 276: Change "(19 packages)" to "(17 packages)" for bridge/internal/
  - Line 289: Change "58 tests (config, vault, placeholder, CDP, mTLS)" to "33 tests (config, vault, mTLS — blindfill tests removed v0.9.0)"
  - Line 1126: Change "58 tests (cargo test --lib)" to "33 tests (cargo test --lib)"
  - Lines 1130-1133: Update test commands — remove `cargo test --all` reference (full test suite has pre-existing failures in governance_service_test.rs)

  **Must NOT do**:
  - Do NOT change the RPC method count (89 is correct)
  - Do NOT change test commands that work

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T1, T4)
  - **Parallel Group**: Wave 1
  - **Blocks**: T9
  - **Blocked By**: None

  **References**:
  - `wc -l bridge/cmd/bridge/main.go` → 3527
  - `ls -d bridge/pkg/*/ | wc -l` → 67
  - `ls -d bridge/internal/*/ | wc -l` → 17
  - `cargo test --lib --manifest-path rust-vault/Cargo.toml` → "33 passed; 0 failed"
  - `grep -c '":' in registerHandlers block` → 89

  **Acceptance Criteria**:
  - [ ] `grep "3,527" doc/armorclaw.md` returns matches
  - [ ] `grep "67 packages" doc/armorclaw.md` returns matches
  - [ ] `grep "17 packages" doc/armorclaw.md` returns matches (for internal)
  - [ ] `grep "33 tests" doc/armorclaw.md` returns matches

  **QA Scenarios:**
  ```
  Scenario: All counts match codebase
    Tool: Bash (grep)
    Steps:
      1. grep "3,527" doc/armorclaw.md | wc -l → >= 1
      2. grep "67 packages" doc/armorclaw.md | wc -l → >= 1
      3. grep "17 packages" doc/armorclaw.md | wc -l → >= 1
      4. grep "33 tests" doc/armorclaw.md | wc -l → >= 1
    Expected Result: All counts >= 1
    Evidence: .sisyphus/evidence/task-2-counts-verified.txt
  ```

  **Commit**: YES (groups with T1, T3, T4)
  - Message: same as T1
  - Files: `doc/armorclaw.md`

- [x] 3. Update Rust Vault section — remove vestigial language

  **What to do**:
  - Lines around 920-955: The "Runtime Model" section says "The `blindfill` module is retained solely for test coverage" — WRONG. Module is deleted. Rewrite to say the module was removed in v0.9.0.
  - Line 954: "The `blindfill` module is retained solely for test coverage" → Change to note it was deleted and why
  - Lines around 1102-1122: "CDP Interception" subsection shows Fetch.enable patterns from the deleted CDP interceptor. Remove or mark as historical context only.
  - Lines 1159-1216: "BlindFill Placeholder System" section — this is about the placeholder format (still used by Go BlindFill in `bridge/pkg/pii/`). Remove the "Implementation Details" subsection that references deleted Rust files. Keep the placeholder format docs (still valid for Go-side).
  - Lines 1246-1254: "Performance Characteristics" — remove "Placeholder Resolution: <1ms per placeholder lookup" since the Rust implementation is gone.

  **Must NOT do**:
  - Do NOT remove the placeholder format documentation — Go BlindFill still uses `{{VAULT:field:hash}}`
  - Do NOT remove security guarantees — they still apply to the Go implementation

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T1 being partially done to avoid conflicts)
  - **Parallel Group**: Wave 1 (after T1)
  - **Blocks**: T8
  - **Blocked By**: T1

  **References**:
  - `rust-vault/src/blindfill/` — DELETED, verify with `ls`
  - `rust-vault/src/lib.rs` — no `pub mod blindfill` declaration
  - `bridge/pkg/pii/` — Go BlindFill still active (do NOT remove references)

  **Acceptance Criteria**:
  - [ ] No reference to blindfill module being "retained" or "vestigial"
  - [ ] Placeholder format docs preserved (`{{VAULT:field:hash}}` still documented)
  - [ ] No references to `rust-vault/src/blindfill/` file paths

  **QA Scenarios:**
  ```
  Scenario: Vestigial language removed
    Tool: Bash (grep)
    Steps:
      1. grep -i "vestigial\|retained solely" doc/armorclaw.md
      2. Assert: zero matches
    Expected Result: Empty grep output
    Evidence: .sisyphus/evidence/task-3-no-vestigial.txt

  Scenario: Placeholder format preserved
    Tool: Bash (grep)
    Steps:
      1. grep "VAULT:field:hash" doc/armorclaw.md
      2. Assert: at least 1 match
    Expected Result: >= 1 match
    Evidence: .sisyphus/evidence/task-3-placeholder-preserved.txt
  ```

  **Commit**: YES (groups with T1, T2, T4)

- [x] 4. Add missing features to armorclaw.md

  **What to do**:
  - In "Deployment Modes" section: Verify self-hosted mode is documented with mDNS discovery, self-signed TLS, single-command setup. If not present, add it (content exists in README.md to reference).
  - In "Event Bus Patterns" section: Add note about RegisterBridgeHandler mechanism — the new handler registration system for bridge-side event handlers (documented in communication-infra.md).
  - In "Event Bus Patterns" section: Add note about EventBus panic recovery (commit ec19ae1) — handler snapshot for crash resilience.
  - In "v0.8.0 Changes" at top: Verify the change note is accurate. Consider adding a brief v0.9.0 change note for the blindfill removal and EventPublisher cleanup.

  **Must NOT do**:
  - Do NOT copy-paste entire sections from communication-infra.md — just add cross-references
  - Do NOT restructure the document

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T1, T2)
  - **Parallel Group**: Wave 1
  - **Blocks**: T8
  - **Blocked By**: None

  **References**:
  - `doc/communication-infra.md` — has the RegisterBridgeHandler and dual-bus documentation to cross-reference
  - `bridge/pkg/eventbus/eventbus.go` — Push Bus architecture comment
  - `bridge/internal/events/matrix_event_bus.go` — Stream Bus architecture comment
  - Commit `ec19ae1` — "fix(eventbus): add panic recovery and handler snapshot for bridge handlers"
  - `README.md` — self-hosted mode section for reference

  **Acceptance Criteria**:
  - [ ] Self-hosted mode mentioned in Deployment Modes section
  - [ ] RegisterBridgeHandler cross-reference exists in Event Bus section
  - [ ] EventBus panic recovery mentioned

  **QA Scenarios:**
  ```
  Scenario: Missing features added
    Tool: Bash (grep)
    Steps:
      1. grep -i "self-hosted\|selfhosted" doc/armorclaw.md → >= 1
      2. grep "RegisterBridgeHandler" doc/armorclaw.md → >= 1
      3. grep "panic recovery" doc/armorclaw.md → >= 1
    Expected Result: All >= 1
    Evidence: .sisyphus/evidence/task-4-features-added.txt
  ```

  **Commit**: YES (groups with T1-T3)

- [x] 5. Update license-system.md with team-aware enforcement

  **What to do**:
  - BEFORE WRITING: Verify team enforcement features exist in code:
    - `grep -rn "team" bridge/pkg/license/ bridge/pkg/enforcement/` — confirm team-aware code exists
    - Read `bridge/pkg/enforcement/manager.go` for CheckFeature team logic
    - Read `bridge/pkg/license/` for team-related state
  - Add a new subsection under "Enforcement" → "Team-Aware Enforcement" documenting:
    - Team audit events (governance mutations emit governance events)
    - Team metrics tracking
    - Per-team policy overrides
    - TeamRole registry with license-tier gating
  - CRITICAL: Only document what is confirmed in code. Do NOT invent concepts.
  - Reference commits: 32d5a84, 6eae414 for implementation details

  **Must NOT do**:
  - Do NOT change existing license tier descriptions — those are accurate
  - Do NOT add speculative features
  - Do NOT document team features that don't exist in bridge/pkg/license/ or bridge/pkg/enforcement/

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T6, T7)
  - **Parallel Group**: Wave 2
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/license/` — current license client
  - `bridge/pkg/enforcement/` — enforcement manager
  - Commits `32d5a84`, `6eae414` — team audit, metrics, governance, policy overrides

  **Acceptance Criteria**:
  - [ ] license-system.md has a "Team-Aware Enforcement" subsection
  - [ ] Mentions team audit events, metrics, policy overrides

  **QA Scenarios:**
  ```
  Scenario: Team enforcement documented
    Tool: Bash (grep)
    Steps:
      1. grep -i "team.*audit\|team.*enforcement\|team-aware" doc/license-system.md → >= 1
    Expected Result: >= 1 match
    Evidence: .sisyphus/evidence/task-5-team-enforcement.txt
  ```

  **Commit**: YES
  - Message: `docs(license): add team-aware enforcement section`
  - Files: `doc/license-system.md`

- [x] 6. Verify ArmorChat.md accuracy

  **What to do**:
  - Read `doc/ArmorChat.md` completely
  - Verify ALL claims against current codebase (not just spot checks):
    - Screen/component counts: `grep -rn "Screen\|ViewModel" applications/ArmorChat/app/src/main/java/ | wc -l` → match doc
    - ViewModel names: `ls applications/ArmorChat/app/src/main/java/**/viewmodel/` or grep for specific names
    - Deep link routes: `grep -rn "Route\." applications/ArmorChat/app/src/main/java/` → match doc
    - Matrix event handling: `grep -rn "app.armorclaw" applications/ArmorChat/` → match doc
    - Biometric keystore: verify classes mentioned exist
  - Use 2026-04-28 versions of files (not older April 18 snapshots)
  - If accurate, note "VERIFIED CLEAN" with evidence file recording each claim verified and the exact source path/command that confirmed it
  - If discrepancies found, fix them

  **Must NOT do**:
  - Do NOT rewrite sections that are accurate
  - Do NOT add new sections
  - Do NOT accept "looks fine" — verify with source commands

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T5, T7)
  - **Parallel Group**: Wave 2
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `doc/ArmorChat.md` — to verify
  - `applications/ArmorChat/app/` — Android source

  **Acceptance Criteria**:
  - [ ] All screen/component references verified against source
  - [ ] Any discrepancies fixed or confirmed clean

  **QA Scenarios:**
  ```
  Scenario: ArmorChat.md verified
    Tool: Bash (grep + read)
    Steps:
      1. Read doc/ArmorChat.md
      2. Verify 3 ViewModel names exist in applications/ArmorChat/
      3. Verify deep link routes match
    Expected Result: All claims match code, or fixes applied
    Evidence: .sisyphus/evidence/task-6-armorchat-verified.txt
  ```

  **Commit**: Only if changes needed

- [x] 7. Verify client-applications.md + sidecar-pipeline.md accuracy

  **What to do**:
  - Read both files completely
  - client-applications.md: Verify admin panel pages (grep for component names in applications/admin-panel/), ArmorTerminal components (grep in applications/ArmorTerminal/), OpenClaw UI capabilities (grep in container/openclaw-src/ui/)
  - sidecar-pipeline.md: Verify format routing table matches `bridge/pkg/sidecar/office_client.go`, test counts (run actual tests to confirm 106), Java sidecar routing (grep for javaClient in office_client.go)
  - Use 2026-04-28 versions of files (not older snapshots)
  - If accurate, note "VERIFIED CLEAN" with evidence file recording each claim verified and the exact source path/command that confirmed it
  - If discrepancies found, fix them

  **Must NOT do**:
  - Do NOT touch these files if they're accurate
  - Do NOT accept "looks fine" — verify with source commands

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T5, T6)
  - **Parallel Group**: Wave 2
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `doc/client-applications.md` — to verify
  - `doc/sidecar-pipeline.md` — to verify
  - Corresponding source directories

  **Acceptance Criteria**:
  - [ ] All claims verified or fixed
  - [ ] Test counts accurate

  **QA Scenarios:**
  ```
  Scenario: Both files verified
    Tool: Bash (read)
    Steps:
      1. Read both files
      2. Verify test counts, component names, routing logic
    Expected Result: All accurate or fixes applied
    Evidence: .sisyphus/evidence/task-7-client-sidecar-verified.txt
  ```

  **Commit**: Only if changes needed

- [x] 8. Cross-file grep verification — zero blindfill code references remain

  **What to do**:
  - Run `grep -rn "blindfill/placeholder\|blindfill/cdp_interceptor\|blindfill/cdp_\|blindfill/mod\|src/blindfill" doc/` — must return ZERO results
  - Run `grep -rn "vestigial\|retained solely" doc/` — must return ZERO results (in context of blindfill)
  - Run `grep -rn "EventPublisher" doc/` — must return ZERO results (removed from codebase in commit 8c3ce69)
  - If any results, fix the specific file

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3
  - **Blocks**: T10
  - **Blocked By**: T1, T3, T4

  **References**:
  - All doc/ files

  **Acceptance Criteria**:
  - [ ] `grep -rn "blindfill/" doc/` returns 0 results
  - [ ] `grep -rn "CdpInterceptor" doc/` returns 0 results
  - [ ] `grep -rn "vestigial" doc/` returns 0 results
  - [ ] `grep -rn "EventPublisher" doc/` returns 0 results

  **QA Scenarios:**
  ```
  Scenario: Complete blindfill + EventPublisher cleanup
    Tool: Bash (grep)
    Steps:
      1. grep -rn "blindfill/" doc/ → 0 results
      2. grep -rn "CdpInterceptor" doc/ → 0 results
      3. grep -rn "vestigial" doc/ → 0 results
      4. grep -rn "EventPublisher" doc/ → 0 results
    Expected Result: All empty
    Evidence: .sisyphus/evidence/task-8-final-blindfill-check.txt
  ```

  **Commit**: Only if fixes needed

- [x] 9. Count verification — all numbers match codebase

  **What to do**:
  - Verify each count in doc/armorclaw.md against live codebase:
    - Rust vault tests: `cargo test --lib --manifest-path rust-vault/Cargo.toml 2>&1 | grep "test result"`
    - Bridge pkg count: `ls -d bridge/pkg/*/ | wc -l`
    - Bridge internal count: `ls -d bridge/internal/*/ | wc -l`
    - main.go lines: `wc -l bridge/cmd/bridge/main.go`
    - RPC methods: count entries in registerHandlers map
  - All must match what's in the doc

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3
  - **Blocks**: T10
  - **Blocked By**: T2

  **References**:
  - Live codebase counts (commands above)

  **Acceptance Criteria**:
  - [ ] All 5 counts match between doc and codebase

  **QA Scenarios:**
  ```
  Scenario: All counts accurate
    Tool: Bash (count commands)
    Steps:
      1. Run each count command
      2. grep for the number in doc/armorclaw.md
      3. Verify match
    Expected Result: All 5 counts match
    Evidence: .sisyphus/evidence/task-9-count-verification.txt
  ```

  **Commit**: Only if fixes needed

- [x] 10. Final commit with all doc changes

  **What to do**:
  - Review all changes made across tasks
  - Ensure clean commit(s) with accurate messages
  - Verify no unintended changes to clean files (voice-stack.md, agent-runtime.md, communication-infra.md)
  - Push current branch to upstream: `git push` (verify branch name first with `git branch --show-current`)

  **Must NOT do**:
  - Do NOT commit changes to voice-stack.md, agent-runtime.md, communication-infra.md

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (final)
  - **Blocks**: F1-F3
  - **Blocked By**: T8, T9

  **References**:
  - Git status

  **Acceptance Criteria**:
  - [ ] All changes committed
  - [ ] No changes to clean files
  - [ ] Commit messages are descriptive

  **QA Scenarios:**
  ```
  Scenario: Clean commit state
    Tool: Bash (git)
    Steps:
      1. git status → clean working tree
      2. git diff HEAD~N..HEAD -- doc/voice-stack.md doc/agent-runtime.md doc/communication-infra.md → empty
    Expected Result: Clean tree, no collateral changes
    Evidence: .sisyphus/evidence/task-10-final-commit.txt
  ```

  **Commit**: YES
  - Message: `docs: full codebase sync — fix stale references and add missing features`
  - Pre-commit: `grep -rn "blindfill/" doc/` must return 0

---

## Final Verification Wave

- [x] F1. **Blindfill + EventPublisher Reference Cleanup** — `grep -rn "blindfill/placeholder\|blindfill/cdp_interceptor\|blindfill/cdp_\|EventPublisher" doc/` returns ZERO results
- [x] F2. **Count Accuracy** — All numbers verified:
  - Rust vault tests: 33 (cargo test --lib)
  - Bridge pkg: 67 (ls -d bridge/pkg/*/ | wc -l)
  - Bridge internal: 17 (ls -d bridge/internal/*/ | wc -l)
  - main.go: 3527 lines (wc -l)
  - RPC methods: 89 (grep -c '":' in registerHandlers)
- [x] F3. **No Collateral Damage** — `git diff doc/voice-stack.md doc/agent-runtime.md doc/communication-infra.md` returns EMPTY (these files untouched)

## Commit Strategy

- **T1-T4**: `docs(armorclaw): sync with current codebase — remove blindfill references, update counts` — doc/armorclaw.md
- **T5**: `docs(license): add team-aware enforcement section` — doc/license-system.md
- **T6-T7**: Verification only (no changes unless issues found)
- **T8-T9**: Verification only
- **T10**: Final commit if any stragglers

## Success Criteria

### Verification Commands
```bash
grep -rn "blindfill/placeholder\|blindfill/cdp_interceptor\|EventPublisher" doc/  # Expected: empty
grep "58 tests" doc/armorclaw.md  # Expected: empty (should say 33)
grep "60 packages" doc/armorclaw.md  # Expected: empty (should say 67)
grep "19 packages" doc/armorclaw.md  # Expected: empty (should say 17)
grep "3,503 lines" doc/armorclaw.md  # Expected: empty (should say 3,527)
git diff doc/voice-stack.md doc/agent-runtime.md doc/communication-infra.md  # Expected: empty
git log --oneline origin/main..HEAD  # Expected: shows new commits (after push: empty)
```

### Final Checklist
- [x] All "Must Have" present
- [x] All "Must NOT Have" absent
- [x] No unintended changes to clean files
