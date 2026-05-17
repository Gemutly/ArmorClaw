# Doc Audit Fixes — Round 3 (5 Issues from Second Audit)

## TL;DR

> **Quick Summary**: Apply 5 factual accuracy corrections to documentation files discovered during the second full doc audit.
> 
> **Deliverables**:
> - 5 edited markdown files with corrected facts
> - All changes committed and pushed
> 
> **Estimated Effort**: Quick (5 small edits)
> **Parallel Execution**: YES - all 5 edits are independent
> **Critical Path**: All edits → verify → commit → push

---

## Context

### Original Request
User asked to "review /doc/ markdownfiles... ensure files represent the current status of the codebase" for the third time after two previous audit rounds.

### Previous Work
- First audit (`45dc2eb`): Fixed 8 issues in `doc/armorclaw.md`
- Second audit: Verified all first-round edits landed correctly, but found 5 NEW issues (4 across other doc files, plus RPC count still wrong at 88 vs actual 93)

### Audit Findings (5 issues)

| # | File | Issue | Severity |
|---|------|-------|----------|
| 1 | `doc/armorclaw.md` | RPC count says 88, actual is 93 (3 locations) | 🔴 High |
| 2 | `doc/client-applications.md` | ArmorTerminal says "Android (Java)" with Matrix SDK + Firebase — actual is Kotlin with neither | 🔴 High |
| 3 | `doc/sidecar-pipeline.md` | Python sidecar socket path missing subdirectory | 🟡 Medium |
| 4 | `doc/communication-infra.md` | UnifiedPush listed as supported but has no provider implementation | 🟡 Medium |
| 5 | `doc/email-android-integration.md` | Response path implies Matrix events, but actual transport is RPC | 🟡 Medium |

---

## Work Objectives

### Core Objective
Correct 5 factual inaccuracies across 4 documentation files to match the current codebase.

### Concrete Deliverables
- `doc/armorclaw.md` — 3 occurrences of "88 registered methods" → "93 registered methods"
- `doc/client-applications.md` — ArmorTerminal tech stack corrected
- `doc/sidecar-pipeline.md` — Socket path corrected
- `doc/communication-infra.md` — UnifiedPush note added
- `doc/email-android-integration.md` — Transport note added

### Must NOT Have (Guardrails)
- Do NOT modify any files outside `doc/`
- Do NOT change version numbers, dates, or section headers
- Do NOT touch code files (.go, .py, .kt, etc.)
- Do NOT add content beyond the specified corrections
- Remember: `doc/` is in `.gitignore` — must use `git add -f`

---

## Verification Strategy

- **Automated tests**: None (documentation only)
- **Agent QA**: grep verification after each edit

---

## Execution Strategy

### Parallel Execution

```
Wave 1 (All 5 edits — independent):
├── Task 1: Fix RPC count in armorclaw.md (replace all 3 occurrences)
├── Task 2: Fix ArmorTerminal tech stack in client-applications.md
├── Task 3: Fix socket path in sidecar-pipeline.md
├── Task 4: Add UnifiedPush note in communication-infra.md
└── Task 5: Add transport note in email-android-integration.md

Wave FINAL: Verify all + commit + push
```

---

## TODOs

- [ ] 1. Fix RPC method count in `doc/armorclaw.md`

  **What to do**:
  - Replace ALL 3 occurrences of `88 registered methods` with `93 registered methods` (use replaceAll=true)
  - Locations: lines ~247, ~396, ~433

  **Must NOT do**:
  - Do not change anything else in the file

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2-5)
  - **Blocks**: Final verification
  - **Blocked By**: None

  **References**:
  - `doc/armorclaw.md:247` — Tree diagram line: `│   │   ├── rpc/              # JSON-RPC 2.0 server (88 registered methods)`
  - `doc/armorclaw.md:396` — Feature list line: `- JSON-RPC 2.0 API (88 registered methods across 18 domains)`
  - `doc/armorclaw.md:433` — Struct comment: `handlers          map[string]HandlerFunc  // 88 registered methods`
  - `bridge/pkg/rpc/server.go` — Contains `registerHandlers()` with 93 actual registered methods

  **Acceptance Criteria**:
  - [ ] `grep -c "88 registered methods" doc/armorclaw.md` returns 0
  - [ ] `grep -c "93 registered methods" doc/armorclaw.md` returns 3

  **QA Scenarios**:
  ```
  Scenario: RPC count updated everywhere
    Tool: Bash (grep)
    Preconditions: Edit applied
    Steps:
      1. grep -c "88 registered methods" doc/armorclaw.md → expect 0
      2. grep -c "93 registered methods" doc/armorclaw.md → expect 3
    Expected Result: Zero occurrences of old count, three of new count
    Evidence: .sisyphus/evidence/task-1-rpc-count.txt
  ```

  **Commit**: Groups with final commit
  - Message: `docs: fix RPC count 88→93 in armorclaw.md`
  - Files: `doc/armorclaw.md`

---

- [ ] 2. Fix ArmorTerminal tech stack in `doc/client-applications.md`

  **What to do**:
  - Replace the tech stack line for ArmorTerminal

  **Exact edit**:
  ```
  oldString: "| **Tech stack** | Android (Java), traditional Android SDK, Matrix SDK, OkHttp, Retrofit, Firebase Cloud Messaging |"
  newString: "| **Tech stack** | Android (Kotlin), traditional Android SDK, OkHttp, Retrofit |"
  ```

  **Must NOT do**:
  - Do not change any other rows in the table
  - Do not modify other sections

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3-5)
  - **Blocks**: Final verification
  - **Blocked By**: None

  **References**:
  - `doc/client-applications.md:87` — Current (wrong) tech stack row
  - `applications/ArmorTerminal/` — Actual Kotlin source files (.kt), no Firebase or Matrix SDK imports

  **Acceptance Criteria**:
  - [ ] `grep "Android (Java)" doc/client-applications.md` returns empty
  - [ ] `grep "Kotlin" doc/client-applications.md` returns at least 1 match
  - [ ] `grep "Firebase" doc/client-applications.md` returns empty (in tech stack row)

  **QA Scenarios**:
  ```
  Scenario: Tech stack corrected
    Tool: Bash (grep)
    Preconditions: Edit applied
    Steps:
      1. grep "Android (Java)" doc/client-applications.md → expect no output
      2. grep "Kotlin" doc/client-applications.md → expect 1 match
    Expected Result: Java replaced with Kotlin, Firebase and Matrix SDK removed
    Evidence: .sisyphus/evidence/task-2-tech-stack.txt
  ```

  **Commit**: Groups with final commit

---

- [ ] 3. Fix Python sidecar socket path in `doc/sidecar-pipeline.md`

  **What to do**:
  - Fix the socket path to include the subdirectory

  **Exact edit**:
  ```
  oldString: "/run/armorclaw/sidecar-office.sock"
  newString: "/run/armorclaw/office-sidecar/sidecar-office.sock"
  ```

  **Must NOT do**:
  - Do not change anything else in the file

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-2, 4-5)
  - **Blocks**: Final verification
  - **Blocked By**: None

  **References**:
  - `doc/sidecar-pipeline.md:390` — Current (wrong) path
  - Python sidecar code — Actual socket path includes `/office-sidecar/` subdirectory

  **Acceptance Criteria**:
  - [ ] `grep "office-sidecar/sidecar-office.sock" doc/sidecar-pipeline.md` returns 1 match
  - [ ] `grep "/run/armorclaw/sidecar-office.sock" doc/sidecar-pipeline.md` returns empty

  **QA Scenarios**:
  ```
  Scenario: Socket path corrected
    Tool: Bash (grep)
    Preconditions: Edit applied
    Steps:
      1. grep "office-sidecar/sidecar-office.sock" doc/sidecar-pipeline.md → expect 1 match
    Expected Result: Path includes subdirectory
    Evidence: .sisyphus/evidence/task-3-socket-path.txt
  ```

  **Commit**: Groups with final commit

---

- [ ] 4. Clarify UnifiedPush in `doc/communication-infra.md`

  **What to do**:
  - Add a note that UnifiedPush has no provider implementation

  **Exact edit**:
  ```
  oldString: "- **Unified Push** as an abstract platform identifier"
  newString: "- **Unified Push** as an abstract platform identifier (constant defined, no provider implementation)"
  ```

  **Must NOT do**:
  - Do not remove UnifiedPush from the list
  - Do not add new sections

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-3, 5)
  - **Blocks**: Final verification
  - **Blocked By**: None

  **References**:
  - `doc/communication-infra.md:31` — Current (misleading) UnifiedPush line
  - ArmorChat source — Only a constant declaration, no provider wiring

  **Acceptance Criteria**:
  - [ ] `grep "no provider implementation" doc/communication-infra.md` returns 1 match

  **QA Scenarios**:
  ```
  Scenario: UnifiedPush note added
    Tool: Bash (grep)
    Preconditions: Edit applied
    Steps:
      1. grep "no provider implementation" doc/communication-infra.md → expect 1 match
    Expected Result: Clarifying note present
    Evidence: .sisyphus/evidence/task-4-unifiedpush.txt
  ```

  **Commit**: Groups with final commit

---

- [ ] 5. Add transport note to `doc/email-android-integration.md`

  **What to do**:
  - Add a transport note clarifying that approval responses use RPC, not Matrix events

  **Exact edit**:
  Use enough context to uniquely target the `email_approval_response` section (not the other two event types):
  ```
  oldString: "**Event Type**: `app.armorclaw.email_approval_response`\n**Classification**: Transient message event (NOT state event)"
  newString: "**Event Type**: `app.armorclaw.email_approval_response`\n**Classification**: Transient message event (NOT state event)\n\n> **Transport Note**: While this event type is defined for Matrix, the Bridge actually processes approval responses via JSON-RPC methods (`approve_email`, `deny_email`). The Matrix event type serves as the conceptual schema for the ArmorChat UI layer."
  ```

  **Must NOT do**:
  - Do not modify the other two event type sections (email_approval_request, email.received)
  - Do not remove existing content

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-4)
  - **Blocks**: Final verification
  - **Blocked By**: None

  **References**:
  - `doc/email-android-integration.md:44-45` — Target section (email_approval_response)
  - `bridge/pkg/rpc/server.go` — Contains `approve_email` and `deny_email` RPC handlers (not Matrix event handlers)

  **Acceptance Criteria**:
  - [ ] `grep "Transport Note" doc/email-android-integration.md` returns 1 match
  - [ ] `grep "approve_email.*deny_email" doc/email-android-integration.md` returns 1 match

  **QA Scenarios**:
  ```
  Scenario: Transport note added
    Tool: Bash (grep)
    Preconditions: Edit applied
    Steps:
      1. grep "Transport Note" doc/email-android-integration.md → expect 1 match
      2. grep "approve_email" doc/email-android-integration.md → expect 1 match
    Expected Result: Note clarifying RPC transport present
    Evidence: .sisyphus/evidence/task-5-transport-note.txt
  ```

  **Commit**: Groups with final commit

---

## Final Verification Wave

- [ ] F1. **Grep Verification** — `quick`
  Run all verification greps from tasks 1-5:
  ```bash
  echo "=== RPC count ===" && grep -c "88 registered methods" doc/armorclaw.md && grep -c "93 registered methods" doc/armorclaw.md
  echo "=== Tech stack ===" && grep "Android (Java)" doc/client-applications.md || echo "PASS: no Java reference"
  echo "=== Socket path ===" && grep "office-sidecar/sidecar-office.sock" doc/sidecar-pipeline.md
  echo "=== UnifiedPush ===" && grep "no provider implementation" doc/communication-infra.md
  echo "=== Transport note ===" && grep "Transport Note" doc/email-android-integration.md
  ```
  All must pass before committing.

- [ ] F2. **Commit and Push** — `quick`
  ```bash
  git add -f doc/armorclaw.md doc/client-applications.md doc/sidecar-pipeline.md doc/communication-infra.md doc/email-android-integration.md
  git commit -m "docs: fix 5 accuracy issues from second audit round

- armorclaw.md: RPC count 88→93 (3 locations)
- client-applications.md: ArmorTerminal Java→Kotlin, remove Firebase/Matrix SDK
- sidecar-pipeline.md: Python socket path add office-sidecar subdirectory
- communication-infra.md: clarify UnifiedPush has no provider implementation
- email-android-integration.md: note approval responses use RPC not Matrix"
  git push
  ```

---

## Commit Strategy

- **Single commit**: All 5 doc fixes together (logically related — same audit round)
  - Message: `docs: fix 5 accuracy issues from second audit round`
  - Files: 5 doc files
  - Must use `git add -f` because `doc/` is gitignored

---

## Success Criteria

### Verification Commands
```bash
grep -c "88 registered methods" doc/armorclaw.md     # Expected: 0
grep -c "93 registered methods" doc/armorclaw.md     # Expected: 3
grep "Android (Java)" doc/client-applications.md     # Expected: no output
grep "Kotlin" doc/client-applications.md             # Expected: 1 match
grep "office-sidecar/sidecar" doc/sidecar-pipeline.md # Expected: 1 match
grep "no provider" doc/communication-infra.md        # Expected: 1 match
grep "Transport Note" doc/email-android-integration.md # Expected: 1 match
```

### Final Checklist
- [ ] All 5 edits applied and verified
- [ ] No unintended changes in any doc file
- [ ] Changes committed and pushed
