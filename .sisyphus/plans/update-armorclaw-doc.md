# Plan: Update doc/armorclaw.md with Context Management & State Machine Architecture

## TL;DR

> **Quick Summary**: Add two new sections to doc/armorclaw.md documenting (1) OpenClaw context management architecture and (2) Go Bridge agent state machine, plus update existing sections with corrected data.
> 
> **Deliverables**:
> - New "Context Management Architecture" subsection under OpenClaw Agent Runtime
> - New "Agent State Machine (Go Bridge)" standalone section
> - Updated numbers/counts throughout
> 
> **Estimated Effort**: Quick
> **Parallel Execution**: YES - 1 wave
> **Critical Path**: Single task

---

## TODOs

- [x] 1. Add "Context Management Architecture" subsection to OpenClaw Agent Runtime section

  **What to do**:
  - Insert a new subsection after the "Skills" subsection (after line ~2507) and before the `---` separator (line ~2508)
  - Title: `### Context Management Architecture`
  - Content to add:
    - Explain OpenClaw uses a **reactive overflow → compaction** pipeline
    - Document the Context Window Resolution priority chain (4 levels from config to default)
    - List key files: `context-window-guard.ts`, `context.ts`, `defaults.ts` with their constants
    - Document in-memory chat history: `activeSession.messages` (AgentMessage[]), where created (`attempt.ts:575`), where replaced (`attempt.ts:691`), where persisted (JSONL session file), where snapshotted (`compact.ts:582`)
    - Document the Compaction Pipeline (post-overflow, reactive): overflow detection (`run.ts:585-601`), auto-compaction trigger (`run.ts:603-681`, up to 3 retries), tool-result truncation fallback (`run.ts:685-731`), final fallback error message (`run.ts:744-765`)
    - Document the Compaction Engine table from `compaction.ts`: `estimateMessagesTokens()`, `summarizeInStages()`, `pruneHistoryForContextShare()`, `chunkMessagesByMaxTokens()`, `computeAdaptiveChunkRatio()`, `summarizeWithFallback()`
    - Add "What's Missing" callout: no proactive pre-overflow compression; injection point is `attempt.ts:838`

  **Must NOT do**:
  - Do not modify any TypeScript source files
  - Do not add implementation code — documentation only

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single file edit, well-defined content to insert
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Task 2)
  - **Blocks**: Task 3
  - **Blocked By**: None

  **References**:
  - `doc/armorclaw.md:2474-2507` — Current OpenClaw Agent Runtime section (insert after Skills subsection)
  - `container/openclaw-src/src/agents/context-window-guard.ts` — Context window resolution logic
  - `container/openclaw-src/src/agents/context.ts` — MODEL_CACHE, lookupContextTokens
  - `container/openclaw-src/src/agents/defaults.ts` — DEFAULT_CONTEXT_TOKENS = 200_000
  - `container/openclaw-src/src/agents/compaction.ts` — Full compaction engine
  - `container/openclaw-src/src/agents/pi-embedded-runner/run.ts` — Lines 585-681 (overflow detection + compaction trigger)
  - `container/openclaw-src/src/agents/pi-embedded-runner/run/attempt.ts` — Lines 575, 691, 838 (session creation, message replacement, pre-prompt injection point)
  - `container/openclaw-src/src/agents/pi-embedded-runner/compact.ts` — Lines 244-720 (compactEmbeddedPiSessionDirect)

  **Acceptance Criteria**:
  - [ ] New `### Context Management Architecture` subsection exists in doc/armorclaw.md
  - [ ] Documents context window resolution chain
  - [ ] Documents in-memory chat history (`activeSession.messages`)
  - [ ] Documents compaction pipeline (4 stages)
  - [ ] Documents compaction engine function table
  - [ ] Notes "What's Missing: Proactive Pre-Overflow Compression" with injection point

  **QA Scenarios**:
  ```
  Scenario: Verify new section exists and is complete
    Tool: Bash (grep)
    Preconditions: doc/armorclaw.md exists
    Steps:
      1. grep -c "Context Management Architecture" doc/armorclaw.md → expect 1
      2. grep -c "activeSession.messages" doc/armorclaw.md → expect ≥ 3
      3. grep -c "compaction" doc/armorclaw.md → expect ≥ 10
      4. grep "Proactive Pre-Overflow" doc/armorclaw.md → expect match
    Expected Result: All greps return expected counts
    Evidence: .sisyphus/evidence/task-1-section-exists.txt

  Scenario: Verify section is under OpenClaw Agent Runtime
    Tool: Bash (grep)
    Steps:
      1. Verify "Context Management Architecture" appears AFTER "OpenClaw Agent Runtime" heading
      2. Verify it appears BEFORE "RPC API Reference" heading
    Expected Result: Line numbers are in correct order
    Evidence: .sisyphus/evidence/task-1-section-placement.txt
  ```

  **Commit**: YES (groups with 2)
  - Message: `docs(armorclaw): add context management and state machine architecture sections`
  - Files: `doc/armorclaw.md`

- [x] 2. Add "Agent State Machine (Go Bridge)" standalone section

  **What to do**:
  - Insert a new section before the "Review Documentation" section (before line ~2979)
  - Title: `## Agent State Machine (Go Bridge)`
  - Content to add:
    - Document 11 states with table (State, Description, Category)
    - Document all 12 transitions leading to IDLE or COMPLETE with table (From, To, Trigger, Code Path)
    - Document the event flow: `StateMachine.Transition()` → `eventChan` → `Integration.processEvents()` → `onStatusChange` callback
    - **Critical Gap callout**: `onStatusChange` is never wired in production code (only in tests). `BroadcastStatus()` is a stub. Eventbus has `EventTypeAgentStatusChanged` but nothing publishes to it. OpenClaw TypeScript has zero references to agent status events. The `→ IDLE` and `→ COMPLETE` transitions are **invisible to the container runtime**.
    - **Implication for Layer 0**: Compression trigger cannot be task-completion event (no event crosses boundary). Two paths: (1) Add task-completion signal (new RPC method, changes scope) or (2) Add pre-prompt token check in attempt.ts:838 (simpler, contained). Both can coexist.

  **Must NOT do**:
  - Do not modify Go source files
  - Do not add implementation code

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single file edit, well-defined content
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Task 1)
  - **Blocks**: Task 3
  - **Blocked By**: None

  **References**:
  - `doc/armorclaw.md:2977-2988` — Insertion point (before Review Documentation section)
  - `bridge/pkg/agent/state.go` — 11 states, ValidTransitions map, StatusEvent struct
  - `bridge/pkg/agent/state_machine.go` — StateMachine with eventChan, subscribers, Transition(), processEvents()
  - `bridge/pkg/agent/integration.go` — Integration.processEvents() (line 232-258), OnStatusChange() (line 66-69), BroadcastStatus() stub (line 330-338)
  - `bridge/pkg/agent/status_integration.go` — IntegrationEmitter, BrowserStatusEmitter, BlindFillStatusEmitter
  - `bridge/pkg/browser/processor.go:164-166` — ForceTransition to StatusComplete
  - `bridge/pkg/eventbus/events.go:22` — EventTypeAgentStatusChanged (defined but unused)

  **Acceptance Criteria**:
  - [ ] New `## Agent State Machine (Go Bridge)` section exists
  - [ ] Contains 11-state table
  - [ ] Contains 12-transition table for IDLE/COMPLETE paths
  - [ ] Documents event flow from StateMachine to Integration
  - [ ] Contains "Critical Gap" callout about no signal reaching OpenClaw
  - [ ] Contains Layer 0 implication with both paths documented

  **QA Scenarios**:
  ```
  Scenario: Verify state machine section exists
    Tool: Bash (grep)
    Steps:
      1. grep -c "Agent State Machine" doc/armorclaw.md → expect 1
      2. grep -c "AWAITING_CAPTCHA" doc/armorclaw.md → expect ≥ 3
      3. grep "Critical Gap" doc/armorclaw.md → expect match
      4. grep "Layer 0" doc/armorclaw.md → expect match
    Expected Result: All greps return expected counts
    Evidence: .sisyphus/evidence/task-2-state-machine-section.txt

  Scenario: Verify section placement
    Tool: Bash (grep -n)
    Steps:
      1. grep -n "Agent State Machine" doc/armorclaw.md → get line number
      2. grep -n "Review Documentation" doc/armorclaw.md → get line number
      3. Verify state machine section line < review documentation line
    Expected Result: Correct ordering
    Evidence: .sisyphus/evidence/task-2-placement.txt
  ```

  **Commit**: YES (groups with 1)
  - Message: `docs(armorclaw): add context management and state machine architecture sections`
  - Files: `doc/armorclaw.md`

- [x] 3. Verify document integrity

  **What to do**:
  - Read the updated doc/armorclaw.md end-to-end
  - Verify no broken markdown (unmatched headers, tables)
  - Verify section ordering is correct
  - Verify cross-references still work
  - Verify no duplicate sections

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (after Tasks 1 and 2)
  - **Blocks**: None
  - **Blocked By**: Tasks 1, 2

  **Acceptance Criteria**:
  - [ ] `grep -c "## " doc/armorclaw.md` returns expected count (should be higher than before by 1)
  - [ ] No duplicate `### Context Management Architecture` sections
  - [ ] No duplicate `## Agent State Machine` sections

  **QA Scenarios**:
  ```
  Scenario: Verify no duplicate sections
    Tool: Bash
    Steps:
      1. grep -c "Context Management Architecture" doc/armorclaw.md → expect exactly 1
      2. grep -c "Agent State Machine" doc/armorclaw.md → expect exactly 1
    Expected Result: Both return 1
    Evidence: .sisyphus/evidence/task-3-no-duplicates.txt

  Scenario: Verify markdown structure is intact
    Tool: Bash
    Steps:
      1. Count H2 headings: grep -c "^## " doc/armorclaw.md
      2. Verify "End of Documentation" is still the last line
    Expected Result: Structure intact
    Evidence: .sisyphus/evidence/task-3-structure.txt
  ```

  **Commit**: NO

---

## Commit Strategy

- **Tasks 1+2**: `docs(armorclaw): add context management and state machine architecture sections`
  - Files: `doc/armorclaw.md`
  - Pre-commit: `grep -c "Context Management Architecture" doc/armorclaw.md && grep -c "Agent State Machine" doc/armorclaw.md`

## Success Criteria

- `doc/armorclaw.md` contains new "Context Management Architecture" subsection under OpenClaw
- `doc/armorclaw.md` contains new "Agent State Machine (Go Bridge)" standalone section
- Both sections contain the investigation findings: context window resolution, in-memory history, compaction pipeline, state transitions, event flow gap, and Layer 0 implications
- Document structure is intact with no duplicates
