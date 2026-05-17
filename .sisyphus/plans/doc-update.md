# Update armorclaw.md to Reflect Skills Review Codebase Changes

## TL;DR

> **Quick Summary**: Update `doc/armorclaw.md` (3,454 lines) with surgical additions documenting 12 verified codebase changes from the skills review project. Two claimed changes (F2 calendar overlap, S4 deploy.yaml version param) are dropped because the code does not reflect them.
> 
> **Deliverables**:
> - Updated `doc/armorclaw.md` with 6 targeted section additions
> - No other files modified
> 
> **Estimated Effort**: Quick (5 surgical edits, all in one file)
> **Parallel Execution**: YES — 5 tasks across 2 waves
> **Critical Path**: Task 1 → Tasks 2-5 → Final Verification

---

## Context

### Original Request
Update `/doc/` markdown files to reflect codebase changes made during the skills review project.

### Interview Summary
**Key Discussions**:
- Analyzed all 13 files in `/doc/` — only `armorclaw.md` is affected
- Verified each of 14 claimed code changes against actual source code
- Found 2 changes NOT present in code (F2 hasOverlap, S4 deploy.yaml version param)
- Found Skills RPC section already documents `email_send` and `allowlist_remove`

**Research Findings**:
- `armorclaw.md` has **zero mentions** of: containsDangerousChars, SSRF, WebDAV internals, deny-by-default policy, yaml.v3 parser, or Governor-as-default-SkillGate
- This means the plan is **adding new content**, not updating existing descriptions
- The doc version is 0.7.0 (different scheme from README's 4.8.0)
- Skills RPC section at lines 2966-2984 already has both `email_send` (line 2980) and `allowlist_remove` (line 2976)

### Metis Review
**Identified Gaps** (addressed):
- F2 (hasOverlap midnight normalization) NOT fixed in code → **DROPPED from scope**
- S4/T10 (deploy.yaml version parameter) NOT in code → **DROPPED from scope**
- Skills RPC entries already exist → **Changed from "add" to "verify accuracy"**
- Depth of new content → **Architectural significance only, no function-level detail**
- Docker YARA fix ambiguous → **DROPPED unless clarified**

---

## Work Objectives

### Core Objective
Add architecturally-relevant descriptions of 12 verified security and behavior changes to `doc/armorclaw.md`, keeping content at the system documentation level (not implementation-level detail).

### Concrete Deliverables
- `doc/armorclaw.md` with 6 surgical section updates reflecting verified code changes

### Definition of Done
- [ ] `grep -c "deny-by-default\|deny by default" doc/armorclaw.md` → ≥ 1
- [ ] `grep -c "IPv6" doc/armorclaw.md` → ≥ 1
- [ ] `grep -c "HTTPS.*enforc\|requires HTTPS" doc/armorclaw.md` → ≥ 1 (in WebDAV context)
- [ ] `wc -l doc/armorclaw.md` → > 3454
- [ ] `git diff --name-only` → only `doc/armorclaw.md`

### Must Have
- Accurate reflection of verified code changes only
- Surgical insertions that don't disrupt existing content
- Architectural-level descriptions (security model changes, behavioral contracts)

### Must NOT Have (Guardrails)
- ❌ Documentation of F2 (hasOverlap) — code still has midnight normalization
- ❌ Documentation of S4/T10 (deploy.yaml version parameter) — no such parameter exists
- ❌ Implementation-level details (function signatures, line numbers, code snippets of internals)
- ❌ Review findings that weren't resolved (46 findings, only 14 addressed)
- ❌ Modifications to any file other than `doc/armorclaw.md`
- ❌ AI slop: excessive comments, over-explanation, generic filler text
- ❌ Content bloat — keep additions concise and focused

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: N/A (documentation, not code)
- **Automated tests**: N/A
- **Framework**: N/A

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Documentation**: Use Bash (grep/wc) — verify terms appear, verify no content lost, verify file structure

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — foundation):
└── Task 1: Version header + Context Routing Rules update [quick]

Wave 2 (After Wave 1 — all parallel, MAX PARALLEL):
├── Task 2: Security Architecture — skill policy + SSRF + WebDAV additions [unspecified-high]
├── Task 3: Governor-Shield — default SkillGate wiring note [quick]
├── Task 4: Skills RPC section — verify accuracy of existing entries [quick]
└── Task 5: Deployment Skills section — minor note about deploy.yaml [quick]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Doc quality review (unspecified-high)
├── Task F3: QA verification (unspecified-high)
└── Task F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| 1 | - | 2, 3, 4, 5 |
| 2 | 1 | F1-F4 |
| 3 | 1 | F1-F4 |
| 4 | 1 | F1-F4 |
| 5 | 1 | F1-F4 |
| F1-F4 | 2-5 | user okay |

### Agent Dispatch Summary

- **Wave 1**: 1 task — T1 → `quick`
- **Wave 2**: 4 tasks — T2 → `unspecified-high`, T3-T5 → `quick`
- **FINAL**: 4 tasks — F1 → `oracle`, F2-F4 → `unspecified-high`

---

## TODOs

- [ ] 1. Version Header + Context Routing Rules

  **What to do**:
  - Update the version header (line 5) from `0.7.0` to `0.7.1`
  - Add a changes note (line 12) for v0.7.1: `Skills security hardening: deny-by-default policy, SSRF IPv6 protections, WebDAV HTTPS enforcement, Governor wired as default SkillGate, yaml.v3 parser adoption`
  - Add new rows to the Context Routing Rules table (lines 17-44) for:
    - `Modify skill security policy` → `bridge/internal/skills/executor.go` and `bridge/internal/skills/policy.go`
    - `Modify SSRF protection` → `bridge/internal/skills/ssrf.go`
    - `Modify WebDAV client` → `bridge/internal/skills/webdav.go`
    - `Modify calendar conflict detection` → `bridge/internal/skills/calendar.go`
    - `Modify skill YAML loading` → `bridge/internal/skills/registry.go`
    - `Modify allowlist management` → `bridge/internal/skills/allowlist.go`

  **Must NOT do**:
  - Do not change the README version (4.8.0) — different versioning scheme
  - Do not remove existing routing rule rows
  - Do not add routing rules for files not changed

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO (foundation for other tasks)
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 2, 3, 4, 5
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `doc/armorclaw.md:5-6` — Current version and changes format
  - `doc/armorclaw.md:11-13` — v0.7.0 changes note (follow this pattern)
  - `doc/armorclaw.md:17-44` — Context Routing Rules table (add rows following existing format)

  **API/Type References**:
  - `bridge/internal/skills/executor.go` — containsDangerousChars (S1), IsAllowed deny-by-default (S2)
  - `bridge/internal/skills/policy.go` — Policy map with email.send entry (S8)
  - `bridge/internal/skills/ssrf.go` — IPv6 CIDR blocks, url.Parse usage (S6+S7)
  - `bridge/internal/skills/webdav.go` — Timeout constant, HTTPS enforcement (S9+S10)
  - `bridge/internal/skills/registry.go` — yaml.v3 import, yaml.Unmarshal usage (S3)
  - `bridge/internal/skills/allowlist.go` — AllowlistManager
  - `bridge/internal/skills/calendar.go` — hasOverlap function (NOT fixed, for routing only)

  **Acceptance Criteria**:
  - [ ] Version header shows `0.7.1`
  - [ ] v0.7.1 changes note added with skill security hardening summary
  - [ ] 6 new routing rule rows added (one per modified skill file)
  - [ ] No existing routing rules removed or modified
  - [ ] `wc -l doc/armorclaw.md` shows line count increased

  **QA Scenarios**:

  ```
  Scenario: Version header updated
    Tool: Bash (grep)
    Steps:
      1. grep "Version.*0.7.1" doc/armorclaw.md
    Expected Result: 1 match
    Failure Indicators: 0 matches or > 1 match
    Evidence: .sisyphus/evidence/task-1-version-header.txt

  Scenario: Routing rules added for skill files
    Tool: Bash (grep)
    Steps:
      1. grep "skills/executor.go" doc/armorclaw.md
      2. grep "skills/ssrf.go" doc/armorclaw.md
      3. grep "skills/webdav.go" doc/armorclaw.md
    Expected Result: Each grep returns ≥ 1 match (in routing rules table)
    Failure Indicators: Any grep returns 0 matches
    Evidence: .sisyphus/evidence/task-1-routing-rules.txt

  Scenario: No existing content lost
    Tool: Bash (grep)
    Steps:
      1. grep "bridge/pkg/pii/" doc/armorclaw.md (existing row)
      2. grep "bridge/pkg/rpc/server.go" doc/armorclaw.md (existing row)
    Expected Result: Both return matches (existing content preserved)
    Failure Indicators: Any existing row not found
    Evidence: .sisyphus/evidence/task-1-content-preserved.txt
  ```

  **Commit**: YES (groups with tasks 2-5)
  - Message: `docs(armorclaw): add skills security hardening to system documentation`
  - Files: `doc/armorclaw.md`

---

- [ ] 2. Security Architecture — Skill Policy + SSRF + WebDAV Additions

  **What to do**:
  Add a new subsection under **Security Architecture** (after the Container Terminate RPC section, before Governor-Shield, around line 1686) titled `### Skill Security Hardening` covering the 6 verified security changes at architectural depth:

  1. **Deny-by-default skill policy**: `IsAllowed()` now denies skill execution by default. Skills must be explicitly listed in the policy map to auto-execute. New entries require explicit approval (e.g., `email.send` with `Risk: "high"`, `AutoExecute: false`).
  
  2. **Command injection protection**: `containsDangerousChars()` narrowed from broad URL-blocking to specific shell metacharacters (`|`, `;`, `` ` ``, `$(`, `${`). Legitimate URL characters (ampersands, equals signs) are no longer blocked.
  
  3. **SSRF IPv6 protection**: SSRF validator now blocks IPv6 link-local (`fe80::/10`) and loopback (`::1/128`) ranges in addition to existing IPv4 protections. Uses `url.Parse()` for robust URL parsing instead of string matching.
  
  4. **WebDAV security**: WebDAV client enforces HTTPS-only URLs (rejects HTTP). Configurable 30-second timeout prevents hanging connections.
  
  5. **YAML parser upgrade**: Skill registry uses `yaml.v3` for parsing `.skills/*.yaml` files, replacing the hand-rolled parser that had compatibility issues.
  
  6. **Calendar conflict detection**: `hasOverlap()` performs time-range comparison for calendar event conflict detection. (Note: midnight normalization removal is pending — current implementation normalizes to start-of-day.)

  Format: A concise table with columns `| Security Change | Component | Behavior |`, followed by brief description paragraph.

  **Must NOT do**:
  - Do not include function-level code or line numbers
  - Do not document F2 as "fixed" — note it as "pending improvement" or omit the midnight normalization note
  - Do not add more than ~30-40 lines to this section
  - Do not duplicate information already in the Governor-Shield section

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 3, 4, 5)
  - **Blocks**: F1-F4
  - **Blocked By**: Task 1

  **References**:

  **Pattern References**:
  - `doc/armorclaw.md:1395-1686` — Security Architecture section (add at the end, before Governor-Shield)
  - `doc/armorclaw.md:1639-1686` — Container Terminate RPC subsection (follow this depth/format)

  **API/Type References**:
  - `bridge/internal/skills/executor.go:259` — containsDangerousChars: checks `|`, `;`, `` ` ``, `$(`, `${`
  - `bridge/internal/skills/executor.go:348-360` — IsAllowed deny-by-default: returns false when not in policy map
  - `bridge/internal/skills/policy.go:85-91` — `"email.send"` entry: `Risk: "high"`, `AutoExecute: false`
  - `bridge/internal/skills/ssrf.go:30-36` — IPv6 CIDRs: `fe80::/10`, `::1/128`
  - `bridge/internal/skills/ssrf.go:120` — `url.Parse()` usage
  - `bridge/internal/skills/webdav.go:17` — `webdavTimeout = 30 * time.Second`
  - `bridge/internal/skills/webdav.go:194,281,354,422` — HTTPS enforcement checks
  - `bridge/internal/skills/registry.go:12` — `yaml.v3` import
  - `bridge/internal/skills/registry.go:282-284` — `yaml.Unmarshal` usage

  **Acceptance Criteria**:
  - [ ] New `### Skill Security Hardening` subsection exists
  - [ ] Contains deny-by-default policy description
  - [ ] Contains SSRF IPv6 description
  - [ ] Contains WebDAV HTTPS enforcement description
  - [ ] Contains YAML parser upgrade description
  - [ ] Does NOT claim hasOverlap midnight normalization is fixed
  - [ ] Section is ≤ 40 lines

  **QA Scenarios**:

  ```
  Scenario: Security hardening section exists with key terms
    Tool: Bash (grep)
    Steps:
      1. grep -c "deny-by-default\|deny by default" doc/armorclaw.md
      2. grep -c "IPv6" doc/armorclaw.md
      3. grep -c "HTTPS.*WebDAV\|WebDAV.*HTTPS" doc/armorclaw.md
      4. grep -c "yaml.v3\|yaml\.v3" doc/armorclaw.md
    Expected Result: Each count ≥ 1
    Failure Indicators: Any count = 0
    Evidence: .sisyphus/evidence/task-2-security-section.txt

  Scenario: F2 midnight normalization NOT documented as fixed
    Tool: Bash (grep)
    Steps:
      1. grep -i "midnight.*remov\|midnight.*fix\|normalization.*remov" doc/armorclaw.md
    Expected Result: 0 matches (should not claim it's fixed)
    Failure Indicators: ≥ 1 match claiming the fix exists
    Evidence: .sisyphus/evidence/task-2-no-false-claims.txt

  Scenario: Section is architecturally focused, not implementation-level
    Tool: Bash (grep)
    Steps:
      1. grep -c "executor.go:" doc/armorclaw.md (should NOT contain line-number references in the new section)
      2. grep -c "func.*contains\|func.*IsAllowed" doc/armorclaw.md (should NOT contain Go function signatures)
    Expected Result: Go function signatures and line-number refs absent from new section
    Failure Indicators: Implementation details in the section
    Evidence: .sisyphus/evidence/task-2-architectural-depth.txt
  ```

  **Commit**: YES (groups with tasks 1, 3-5)
  - Message: `docs(armorclaw): add skills security hardening to system documentation`
  - Files: `doc/armorclaw.md`

---

- [ ] 3. Governor-Shield — Default SkillGate Wiring Note

  **What to do**:
  Update the **Governor-Shield PII Interception** section (~lines 1687-1788) to note that the Governor is now wired as the default `SkillGate` implementation.

  Add a brief note after the "Integration Points" table (~line 1750) indicating:
  - The Governor is wired as the default `SkillGate` for the MCP router and tool executor
  - This means all tool calls automatically pass through PII interception without explicit configuration
  - Source: `bridge/pkg/governor/skillgate.go`, `bridge/cmd/bridge/main.go`

  This should be 3-5 lines — a single paragraph note.

  **Must NOT do**:
  - Do not rewrite existing Governor-Shield content
  - Do not add more than 5 lines
  - Do not include implementation details (wiring code, function calls)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 2, 4, 5)
  - **Blocks**: F1-F4
  - **Blocked By**: Task 1

  **References**:

  **Pattern References**:
  - `doc/armorclaw.md:1746-1751` — Integration Points table (add note after this)

  **API/Type References**:
  - `bridge/pkg/governor/skillgate.go` — Governor implementing SkillGate interface
  - `bridge/cmd/bridge/main.go:2515` — Wiring: `skillGate: governor`
  - `bridge/pkg/mcp/router.go:81,105` — MCPRouter using SkillGate

  **Acceptance Criteria**:
  - [ ] Note about default SkillGate wiring added to Governor-Shield section
  - [ ] Note is ≤ 5 lines
  - [ ] Existing Integration Points table unchanged

  **QA Scenarios**:

  ```
  Scenario: Default SkillGate wiring documented
    Tool: Bash (grep)
    Steps:
      1. grep -A2 -B2 "default.*SkillGate\|SkillGate.*default\|wired.*Governor\|Governor.*wired" doc/armorclaw.md
    Expected Result: ≥ 1 match showing the wiring note in the Governor-Shield section
    Failure Indicators: 0 matches
    Evidence: .sisyphus/evidence/task-3-skillgate-note.txt
  ```

  **Commit**: YES (groups with tasks 1-2, 4-5)
  - Message: `docs(armorclaw): add skills security hardening to system documentation`
  - Files: `doc/armorclaw.md`

---

- [ ] 4. Skills RPC Section — Verify Accuracy of Existing Entries

  **What to do**:
  Verify the existing Skills RPC entries at lines 2966-2984 are accurate given the code changes:

  1. `skills.email_send` (line 2980) — Confirm the entry description is accurate. The `email.send` policy entry now exists (S8) with `Risk: "high"` and `AutoExecute: false`. If the current description doesn't mention the approval gate, add a brief note.

  2. `skills.allowlist_remove` (line 2976) — Confirm the entry description is accurate. The `allowlist_remove` RPC is now properly wired to `AllowlistManager` (T11) for persistent removal. If the current description is vague, clarify that removal is persistent.

  3. `skills.allow` and `skills.block` — Note that the policy is now deny-by-default, so `skills.allow` is the mechanism to enable new skills.

  **Must NOT do**:
  - Do not add new RPC entries — they already exist
  - Do not rewrite the entire Skills RPC table
  - Do not add more than 2-3 words of clarification per entry

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 2, 3, 5)
  - **Blocks**: F1-F4
  - **Blocked By**: Task 1

  **References**:

  **Pattern References**:
  - `doc/armorclaw.md:2966-2984` — Skills RPC methods table

  **API/Type References**:
  - `bridge/pkg/rpc/server.go:867` — `allowlist_remove` registration
  - `bridge/pkg/rpc/methods_skills.go:320` — `allowlist_remove` handler
  - `bridge/internal/skills/policy.go:85-91` — `email.send` policy entry
  - `bridge/internal/skills/executor.go:348-360` — deny-by-default `IsAllowed()`

  **Acceptance Criteria**:
  - [ ] `skills.email_send` description mentions approval requirement (if not already)
  - [ ] `skills.allowlist_remove` description notes persistent removal (if not already)
  - [ ] `skills.allow` description mentions deny-by-default context (if not already)
  - [ ] No new rows added to the table

  **QA Scenarios**:

  ```
  Scenario: Skills RPC entries are accurate
    Tool: Bash (grep)
    Steps:
      1. grep "skills.email_send\|skills\.email_send" doc/armorclaw.md
      2. grep "skills.allowlist_remove\|skills\.allowlist_remove" doc/armorclaw.md
    Expected Result: Both entries still present, descriptions unchanged or minimally clarified
    Failure Indicators: Entries removed or descriptions drastically changed
    Evidence: .sisyphus/evidence/task-4-rpc-entries.txt
  ```

  **Commit**: YES (groups with tasks 1-3, 5)
  - Message: `docs(armorclaw): add skills security hardening to system documentation`
  - Files: `doc/armorclaw.md`

---

- [ ] 5. Deployment Skills Section — Minor Deploy.yaml Note

  **What to do**:
  Review the **Deployment Skills** section (lines 143-183) and determine if any update is needed for the `deploy.yaml` changes.

  The deploy.yaml has `version: "2.0.0"` as file metadata (not a parameter). The existing documentation may or may not reference this. Check if the Skills Directory tree (lines 170-183) or Available Skills table (lines 153-158) needs updating.

  If no change is needed (deploy.yaml file metadata version is not user-facing), this task may be a no-op. Document the decision either way.

  **Must NOT do**:
  - Do not add a version parameter that doesn't exist in deploy.yaml
  - Do not change the Skills Directory tree unless the file structure changed

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 2, 3, 4)
  - **Blocks**: F1-F4
  - **Blocked By**: Task 1

  **References**:

  **Pattern References**:
  - `doc/armorclaw.md:143-183` — Deployment Skills section
  - `doc/armorclaw.md:170-183` — Skills Directory tree
  - `.skills/deploy.yaml` — Current deploy.yaml (version: "2.0.0" as metadata only)

  **Acceptance Criteria**:
  - [ ] Decision documented: either update made or no-op justified
  - [ ] If update: specific line changed and why
  - [ ] If no-op: grep evidence that deploy.yaml user-facing interface unchanged

  **QA Scenarios**:

  ```
  Scenario: Deploy.yaml section is accurate
    Tool: Bash (grep)
    Steps:
      1. grep "deploy.yaml" doc/armorclaw.md
      2. Compare with actual .skills/deploy.yaml content
    Expected Result: Doc accurately reflects deploy.yaml current state
    Failure Indicators: Doc claims features not in deploy.yaml
    Evidence: .sisyphus/evidence/task-5-deploy-section.txt
  ```

  **Commit**: YES (groups with tasks 1-4)
  - Message: `docs(armorclaw): add skills security hardening to system documentation`
  - Files: `doc/armorclaw.md`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify content exists in `doc/armorclaw.md`. For each "Must NOT Have": search doc for forbidden content (F2 fix claims, S4 version parameter claims, implementation details). Check evidence files exist in `.sisyphus/evidence/`.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Doc Quality Review** — `unspecified-high`
  Review the full diff of `doc/armorclaw.md`. Check for: broken Markdown tables, unmatched headers, content duplication, grammar errors, inconsistent terminology. Verify the document structure is still coherent after insertions. Check no section was accidentally truncated.
  Output: `Markdown [PASS/FAIL] | Tables [PASS/FAIL] | Coherence [PASS/FAIL] | VERDICT`

- [ ] F3. **QA Verification** — `unspecified-high`
  Execute ALL QA scenarios from ALL tasks. Grep for each required term. Verify line count increased. Verify only `doc/armorclaw.md` was modified. Save evidence to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Grep Terms [N/N found] | File Check [PASS] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff. Verify 1:1 — everything in spec was done, nothing beyond spec was done. Check "Must NOT Have" compliance. Verify F2/S4 are NOT documented. Detect cross-task contamination.
  Output: `Tasks [N/N compliant] | Forbidden Content [CLEAN/N issues] | VERDICT`

---

## Commit Strategy

- **Single commit** after all tasks complete:
  - Message: `docs(armorclaw): add skills security hardening to system documentation`
  - Files: `doc/armorclaw.md`
  - Pre-commit: `grep -c "Version.*0.7.1" doc/armorclaw.md` (must be ≥ 1)

---

## Success Criteria

### Verification Commands
```bash
# Version bumped
grep "Version.*0.7.1" doc/armorclaw.md          # Expected: 1 match

# Key terms present
grep -c "deny-by-default" doc/armorclaw.md       # Expected: ≥ 1
grep -c "IPv6" doc/armorclaw.md                   # Expected: ≥ 1
grep -c "WebDAV.*HTTPS\|HTTPS.*WebDAV" doc/armorclaw.md  # Expected: ≥ 1
grep -c "yaml.v3" doc/armorclaw.md                # Expected: ≥ 1

# No false claims
grep -i "midnight.*remov\|midnight.*fix" doc/armorclaw.md  # Expected: 0 matches
grep "version.*parameter.*deploy" doc/armorclaw.md          # Expected: 0 matches

# File structure intact
wc -l doc/armorclaw.md                            # Expected: > 3454

# Only target file modified
git diff --name-only                               # Expected: doc/armorclaw.md
```

### Final Checklist
- [ ] All "Must Have" present (deny-by-default, IPv6, WebDAV HTTPS, yaml.v3, SkillGate wiring)
- [ ] All "Must NOT Have" absent (F2 fix claim, S4 version param, implementation details)
- [ ] Document is valid Markdown
- [ ] Only `doc/armorclaw.md` modified
