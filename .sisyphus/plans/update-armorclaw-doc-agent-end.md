# Plan: Update doc/armorclaw.md — agent_end Hook Discovery

## TL;DR

> **Quick Summary**: Replace the "What's Missing" callout in the Context Management Architecture section with a three-tier trigger model (agent_end primary, before_prompt_build safety net, future Bridge signal). Include comparison table, overflow exclusion caveat, and revised capability gap framing. Update the Agent State Machine section's Layer 0 Implication to match.
>
> **Deliverables**:
> - Replace "What's Missing" blockquote with "Proactive Compression: Available Hooks" subsection including three-tier trigger model and comparison table
> - Update Layer 0 Implication in State Machine section with three-trigger approach and overflow exclusion caveat
>
> **Estimated Effort**: Quick
> **Parallel Execution**: NO — single file, two dependent edits
> **Critical Path**: Single task

---

## TODOs

- [x] 1. Replace "What's Missing" callout with agent_end hook discovery

  **What to do**:
  - Find the blockquote at approximately line 2562 that starts with `> **What's Missing**: There is no proactive (pre-overflow) compression.`
  - Replace it with a new `#### Proactive Compression: Available Hooks` subsection containing:
    - Discovery: OpenClaw has internal task-completion signals independent of Bridge
    - `agent_end` plugin hook at `attempt.ts:1151` — fires after every LLM run, receives `{messages, success, error, durationMs}`, registered via `api.on("agent_end", handler)`
    - `session.state` diagnostic event at `runs.ts:143` — `{state: "idle", reason: "run_completed"}`
    - Full plugin lifecycle list from `plugins/types.ts:298-318`
    - Recommended approach: Register `agent_end` plugin for proactive compaction + pre-prompt token check as safety net

  **Exact replacement text**:
  ```markdown
  #### Proactive Compression: Available Hooks

  > **Update**: The OpenClaw runtime **does** have internal task-completion signals, independent of the Bridge state machine. The capability gap is smaller than initially assessed — it is not "no proactive compression" but "no plugin leveraging the existing `agent_end` hook for proactive compression."

  **Three-Trigger Architecture for Layer 0:**

  | Tier | Hook | When It Fires | Purpose |
  |------|------|---------------|---------|
  | **Primary** | `agent_end` (success === true) | After task completes | Compaction at natural task boundaries |
  | **Safety net** | `before_prompt_build` | Before every LLM call | Catches long single-task sessions |
  | **Future** | External Bridge signal | On state machine → IDLE/COMPLETE | Reserved; requires Bridge→Container plumbing |

  **`agent_end` plugin hook** — Fires at `attempt.ts:1151` after every LLM run completes. Receives `{messages, success, error, durationMs}`. Plugins register via `api.on("agent_end", handler)`.

  **Why `agent_end` is the correct primary trigger:**

  | Aspect | `before_prompt_build` | `agent_end` |
  |--------|-----------------------|-------------|
  | When it fires | Before every LLM call | After task completes |
  | Risk of mid-task compaction | **Yes** — fires during multi-step tasks | **No** — `success === true` means task is done |
  | Token cost of compaction itself | Charged against current task's context | Charged in a clean window after task is done |
  | Message snapshot freshness | Messages about to be sent anyway | Completed task's final state — ideal for summarization |

  **`success === true` gate with context-overflow exclusion:**
  `agent_end` fires on *every* run completion, including failures and aborts. The handler must gate on `success === true` to avoid compacting corrupted or incomplete history. **Critical exclusion**: if the run failed *because* of context overflow (the existing `isLikelyContextOverflowError` path at `run.ts:585`), the handler must skip — the reactive compaction retry loop at `run.ts:603-681` already handles this case.

  **`session.state` diagnostic event** — Fires via `clearActiveEmbeddedRun()` at `runs.ts:143` with `{state: "idle", reason: "run_completed"}`. Observable through `emitDiagnosticEvent()`.

  **Full plugin lifecycle** — The system supports 18 hooks: `before_prompt_build`, `llm_input`, `llm_output`, `agent_end`, `before_compaction`, `after_compaction`, `session_start`, `session_end`, and others. All defined in `plugins/types.ts:298-318`.

  **Recommended approach for Layer 0**: Register an `agent_end` plugin that gates on `success === true`, checks `estimateMessagesTokens(messages)` against the context window (~75% threshold), and calls `compactEmbeddedPiSessionDirect()`. Add a `before_prompt_build` safety net at `attempt.ts:838` for long single-task sessions. No Bridge changes needed.
  ```

  **Must NOT do**:
  - Do not modify any TypeScript source files
  - Do not change any other sections of the doc

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential
  - **Blocks**: Task 2
  - **Blocked By**: None

  **References**:
  - `doc/armorclaw.md:2562` — Current "What's Missing" blockquote (exact old string to replace)
  - `container/openclaw-src/src/agents/pi-embedded-runner/run/attempt.ts:1148-1171` — agent_end hook emission
  - `container/openclaw-src/src/agents/pi-embedded-runner/runs.ts:136-151` — clearActiveEmbeddedRun with run_completed
  - `container/openclaw-src/src/plugins/types.ts:298-318` — PluginHookName type union
  - `container/openclaw-src/src/plugins/hooks.ts:268-276` — runAgentEnd implementation
  - `container/openclaw-src/src/logging/diagnostic.ts:169-202` — logSessionStateChange + emitDiagnosticEvent

  **Acceptance Criteria**:
  - [ ] Old "What's Missing" blockquote is replaced
  - [ ] New "#### Proactive Compression: Available Hooks" subsection exists
  - [ ] Three-tier trigger model table present
  - [ ] Comparison table (agent_end vs before_prompt_build) present
  - [ ] `success === true` gate with context-overflow exclusion documented
  - [ ] Capability gap reframed from "no proactive compression" to "no plugin leveraging agent_end"
  - [ ] Mentions `session.state` diagnostic event
  - [ ] Contains recommended Layer 0 approach with three triggers

  **QA Scenarios**:
  ```
  Scenario: Verify old content removed and new content present
    Tool: Bash (grep)
    Steps:
      1. grep -c "What's Missing" doc/armorclaw.md → expect 0
      2. grep -c "Proactive Compression" doc/armorclaw.md → expect 1
      3. grep -c "agent_end" doc/armorclaw.md → expect ≥ 3
      4. grep -c "session.state" doc/armorclaw.md → expect ≥ 1
      5. grep -c "before_prompt_build" doc/armorclaw.md → expect ≥ 1
      6. grep "success === true" doc/armorclaw.md → expect match
      7. grep -c "isLikelyContextOverflowError" doc/armorclaw.md → expect ≥ 1
    Expected Result: All counts match
    Evidence: .sisyphus/evidence/task-4-hooks-update.txt
  ```

  **Commit**: YES (groups with 2)
  - Message: `docs(armorclaw): add agent_end hook discovery for proactive compression`
  - Files: `doc/armorclaw.md`

- [x] 2. Update Layer 0 Implication in Agent State Machine section

  **What to do**:
  - Find the "### Implication for Layer 0 (Context Window Persistence)" subsection in the Agent State Machine section (around line 3094)
  - Replace the first sentence: `The compression trigger cannot be a task-completion event because **no event crosses the Bridge→Container boundary**.`
  - With: `While the Bridge state machine's `→ IDLE` / `→ COMPLETE` transitions do not reach the container, the OpenClaw runtime has its own `agent_end` plugin hook (see [Context Management Architecture](#context-management-architecture)) that fires after every LLM run. This is the natural compression trigger.`
  - Keep the rest of the two-path description but adjust path 1 to say "Register an `agent_end` plugin" instead of "Add a new RPC method"

  **Exact old string to find**:
  ```
  The compression trigger cannot be a task-completion event because **no event crosses the Bridge→Container boundary**. There are two paths forward:

  1. **Add a task-completion signal** (changes scope): Add a new RPC method or Matrix event that the OpenClaw runtime subscribes to. When the Bridge state machine transitions to `COMPLETE` or `IDLE`, it sends a signal to the container. The OpenClaw runtime receives this and triggers `compactEmbeddedPiSessionDirect()`. This makes compaction event-driven but requires new Bridge→Container plumbing.

  2. **Add a pre-prompt token check** (simpler, contained): Add proactive compression inside the existing OpenClaw runtime at `attempt.ts:838`. Before each LLM call, check `estimateMessagesTokens(activeSession.messages)` vs `ctxInfo.tokens * 0.75`. If exceeded, call `compactEmbeddedPiSessionDirect()`. No Bridge changes needed. This is the path of least resistance.

  Both approaches can coexist — the event-driven approach gives compaction at natural task boundaries (cheaper, more coherent summaries), while the token-threshold approach is a safety net for long single-task sessions.
  ```

  **Exact new string**:
  ```
  The Bridge state machine's `→ IDLE` / `→ COMPLETE` transitions do not reach the container. However, the OpenClaw runtime has its own `agent_end` plugin hook (see [Context Management Architecture](#context-management-architecture)) that fires after every LLM run with `{messages, success, error, durationMs}`. This is the natural compression trigger — no Bridge changes needed.

  **Three-trigger approach for Layer 0:**

  1. **Register an `agent_end` plugin** (primary trigger — recommended): The plugin gates on `success === true`, checks `estimateMessagesTokens(messages)` against the context window (~75% threshold), and calls `compactEmbeddedPiSessionDirect()`. Compaction runs at natural task boundaries where summaries are most coherent. **Critical exclusion**: skip when the error is a context overflow — the existing reactive compaction retry loop at `run.ts:585-681` already handles this. No cross-process plumbing needed — the hook already fires in-process with the message snapshot.

  2. **Add a `before_prompt_build` token check** (safety net): At `attempt.ts:838`, check `estimateMessagesTokens(activeSession.messages)` vs `ctxInfo.tokens * 0.75` before each LLM call. This catches long single-task sessions that never cross a task boundary. Note: this fires mid-task, so compaction here is more disruptive — use only as fallback.

  3. **External Bridge signal** (future): A Bridge→Container RPC or Matrix event triggered on state machine `→ IDLE` / `→ COMPLETE`. Reserved; requires new cross-process plumbing.

  Tiers 1 and 2 coexist today — `agent_end` gives compaction at task boundaries (cheaper, more coherent), while `before_prompt_build` is a safety net. Tier 3 is a future extension point if cross-boundary events become necessary.
  ```

  **Must NOT do**:
  - Do not modify the "Critical Gap" subsection (that finding is still correct — the Bridge signal doesn't reach the container)
  - Do not change any other sections

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (after Task 1)
  - **Blocks**: None
  - **Blocked By**: Task 1

  **References**:
  - `doc/armorclaw.md` — "### Implication for Layer 0" subsection (around line 3094-3102)
  - The exact old string to match is provided above

  **Acceptance Criteria**:
  - [ ] Old "cannot be a task-completion event" sentence replaced
  - [ ] New text references `agent_end` plugin hook
  - [ ] Cross-reference link to Context Management Architecture section works
  - [ ] Two-path description updated with agent_end as recommended approach

  **QA Scenarios**:
  ```
  Scenario: Verify updated Layer 0 section
    Tool: Bash (grep)
    Steps:
      1. grep "cannot be a task-completion event" doc/armorclaw.md → expect NO match
      2. grep -c "agent_end" doc/armorclaw.md → expect ≥ 3 matches
      3. grep "register an.*agent_end" doc/armorclaw.md → expect match (case insensitive)
      4. grep "before_prompt_build" doc/armorclaw.md → expect ≥ 1 match
      5. grep "three-trigger\|three-trigger approach\|Three-trigger" doc/armorclaw.md → expect match
    Expected Result: All checks pass
    Evidence: .sisyphus/evidence/task-5-layer0-update.txt
  ```

  **Commit**: YES (groups with 1)
  - Message: `docs(armorclaw): add agent_end hook discovery for proactive compression`
  - Files: `doc/armorclaw.md`

---

## Commit Strategy

- **Tasks 1+2**: `docs(armorclaw): add agent_end hook discovery for proactive compression`
  - Files: `doc/armorclaw.md`
  - Pre-commit: `grep -c "agent_end" doc/armorclaw.md`

## Success Criteria

- "What's Missing" callout replaced with "Proactive Compression: Available Hooks" subsection
- `agent_end` plugin hook documented with exact file/line references
- Three-tier trigger model table present (primary / safety net / future)
- Comparison table (agent_end vs before_prompt_build) present
- `success === true` gate with context-overflow exclusion documented
- Capability gap reframed as "no plugin leveraging agent_end" not "no proactive compression"
- Layer 0 Implication updated with three-trigger approach
- Bridge state machine "Critical Gap" subsection preserved (still accurate)
