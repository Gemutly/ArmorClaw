# Plan: Layer 0 — Context Window Persistence

## TL;DR

> **Quick Summary**: Implement proactive LLM context compaction triggered at task boundaries via the `agent_end` plugin hook. The `shouldCompact()` utility checks token usage against the context window threshold and triggers `compactEmbeddedPiSessionDirect()` when exceeded. Persistence is automatic via the existing JSONL session file (Track B confirmed). The `before_prompt_build` hook serves as a safety net for long single-task sessions.
>
> **Deliverables**:
> - `shouldCompact()` threshold utility in `compaction.ts` with tests
> - Extended `PluginHookAgentEndEvent` and `PluginHookBeforePromptBuildEvent` types with `sessionFile`
> - `agent_end` handler registration for proactive compaction
> - `before_prompt_build` safety net handler
> - `vitest.unit.config.ts` prerequisite (test scripts are broken without it)
>
> **Estimated Effort**: Medium
> **Parallel Execution**: YES — 4 waves
> **Critical Path**: Task 1 (vitest config) → Task 2 (shouldCompact) → Tasks 3+4 (type extensions, parallel) → Tasks 5+6 (handlers, parallel) → FV

---

## Context

### Original Request
Implement Layer 0: Context Window Persistence — proactive compaction that compresses LLM conversation history before context overflow, using OpenClaw's existing plugin hook system.

### Investigation Findings
- **Chat history**: `activeSession.messages` (AgentMessage[]) — created at attempt.ts:575, persisted to JSONL
- **Token limits**: Three disconnected layers — Go Bridge (12K, unused), OpenClaw (per-model via `resolveContextWindowInfo()`), default (200K)
- **Compaction engine**: `compactEmbeddedPiSessionDirect()` at compact.ts:244 — opens own SessionManager, writes to JSONL
- **Session restore**: JSONL is append-only tree, `buildSessionContext()` resolves compacted view, `agent.replaceMessages()` restores on next run — **persistence is automatic**
- **Plugin hooks**: 20 hooks in `plugins/types.ts:298-318`, `agent_end` fires after every LLM run, `before_prompt_build` fires before every LLM call

### Metis Review
**Identified Gaps** (all addressed):
- Hook events lack `sessionFile` → RESOLVED: User chose Option A (extend hook types)
- Race condition with `agent_end` + `session.dispose()` → RESOLVED: Safe because `compactEmbeddedPiSessionDirect()` opens own SessionManager + write lock
- `before_prompt_build` IS awaited (unlike `agent_end`) → NOT fire-and-forget, safe to await
- Vitest configs missing → Task 1 prerequisite
- Anti-thrash guard needed → Follow `shouldRunMemoryFlush()` pattern
- Minimum message count guard needed → `MIN_MESSAGES_FOR_COMPACTION`

---

## Work Objectives

### Core Objective
Implement a proactive compaction trigger that compresses conversation history before context overflow, registered on `agent_end` (primary) and `before_prompt_build` (safety net).

### Concrete Deliverables
- `vitest.unit.config.ts` — restore broken test scripts
- `shouldCompact()` — pure threshold utility with anti-thrash guard
- Extended `PluginHookAgentEndEvent` type with `sessionFile?: string`
- Extended `PluginHookBeforePromptBuildEvent` type with `sessionFile?: string`
- `proactive-compaction.ts` — handler module registering on both hooks
- Co-located test files for each new module

### Definition of Done
- [x] `pnpm test:fast` passes in container/openclaw-src
- [x] `shouldCompact()` returns correct boolean for token thresholds
- [x] `agent_end` handler gates on `success === true`, excludes context-overflow errors
- [x] `before_prompt_build` handler has cooldown to avoid per-call overhead
- [x] No existing tests broken by type extensions
- [x] Compaction output persists to JSONL (Track B confirmed automatic)

### Must Have
- `shouldCompact()` pure utility with: threshold check, minimum message guard, anti-thrash tracking
- `agent_end` handler: `success === true` gate, context-overflow exclusion, calls `compactEmbeddedPiSessionDirect()`
- `before_prompt_build` safety net: same threshold logic, cooldown mechanism
- Unit tests for `shouldCompact()` with specific token counts
- Hook wiring tests following `wired-hooks-compaction.test.ts` pattern

### Must NOT Have (Guardrails)
- No summary message tagging/structure — that's Layer 1
- No SOPGate integration — that's Phase 3
- No Bridge state machine changes — no cross-process plumbing
- No configuration system for threshold — use a named constant
- No debouncing/rate-limiting/scheduling — just trigger on hook
- No modification of `CONTEXT_INPUT_HEADROOM_RATIO` — separate constant
- No modification of existing `before_compaction`/`after_compaction` hook behavior
- No use of `compactEmbeddedPiSession()` (with lane queueing) — deadlock risk from inside a run

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (Vitest 4.0.18, 1343 existing tests)
- **Automated tests**: YES (TDD — write tests before implementation)
- **Framework**: Vitest 4.0.18
- **TDD flow**: Each task follows RED (failing test) → GREEN (minimal impl) → REFACTOR

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Module tests**: `cd container/openclaw-src && pnpm test:fast --run`
- **Type checking**: `cd container/openclaw-src && pnpm tsc --noEmit`
- **Hook wiring**: Verify via grep that handler is registered and called

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — prerequisite + pure utility):
├── Task 1: Create vitest.unit.config.ts [quick]
└── Task 2: Implement shouldCompact() utility with TDD [deep]

Wave 2 (After Wave 1 — type extensions, parallel):
├── Task 3: Extend PluginHookAgentEndEvent with sessionFile [quick]
└── Task 4: Extend PluginHookBeforePromptBuildEvent with sessionFile [quick]

Wave 3 (After Wave 2 — handlers, parallel):
├── Task 5: Register agent_end proactive compaction handler [unspecified-high]
└── Task 6: Register before_prompt_build safety net handler [unspecified-high]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| 1 | - | 2, 5, 6 |
| 2 | 1 | 5, 6 |
| 3 | - | 5 |
| 4 | - | 6 |
| 5 | 2, 3 | F1-F4 |
| 6 | 2, 4 | F1-F4 |

### Agent Dispatch Summary

- **Wave 1**: 2 — T1 `quick`, T2 `deep`
- **Wave 2**: 2 — T3 `quick`, T4 `quick`
- **Wave 3**: 2 — T5 `unspecified-high`, T6 `unspecified-high`
- **FINAL**: 4 — F1 `oracle`, F2 `unspecified-high`, F3 `unspecified-high`, F4 `deep`

---

## TODOs

- [x] 1. Create `vitest.unit.config.ts` to restore test:fast script

  **What to do**:
  - Create `container/openclaw-src/vitest.unit.config.ts` with a minimal configuration that:
    - Includes `src/**/*.test.ts`
    - Excludes `.e2e.test.ts`, `.live.test.ts`, `.browser.test.ts`
    - Uses `vitest/config` defineConfig
    - Sets test environment to `node`
  - This is a prerequisite — `pnpm test:fast` and `pnpm test:coverage` reference this file and currently fail because it doesn't exist
  - Check `scripts/test-parallel.mjs` for any additional config expectations (vitest.extensions.config.ts, vitest.gateway.config.ts) — but only create the unit config for now

  **Must NOT do**:
  - Do NOT modify package.json scripts
  - Do NOT create e2e/live/extension/gateway configs (out of scope)
  - Do NOT add coverage thresholds yet (separate concern)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Task 2)
  - **Blocks**: Tasks 2, 5, 6 (all testing depends on this)
  - **Blocked By**: None

  **References**:
  - `container/openclaw-src/package.json` — scripts section shows `test:fast` references `vitest.unit.config.ts`
  - `container/openclaw-src/ui/vitest.config.ts` — existing vitest config for reference pattern
  - `container/openclaw-src/ui/vitest.node.config.ts` — another existing config for reference
  - `container/openclaw-src/scripts/test-parallel.mjs` — shows what the parallel test runner expects

  **Acceptance Criteria**:
  - [x] `container/openclaw-src/vitest.unit.config.ts` exists
  - [x] `cd container/openclaw-src && pnpm test:fast --run` exits 0 (or at least doesn't fail with "config not found")
  - [x] Config includes `src/**/*.test.ts` and excludes e2e/live/browser patterns

  **QA Scenarios**:
  ```
  Scenario: Verify test:fast runs with new config
    Tool: Bash
    Steps:
      1. cd container/openclaw-src && pnpm test:fast --run 2>&1 | head -20
    Expected Result: Command completes without "Cannot find config" error. May have test failures but config loads successfully.
    Evidence: .sisyphus/evidence/task-1-vitest-config.txt
  ```

  **Commit**: YES
  - Message: `chore: add vitest.unit.config.ts to restore test:fast script`
  - Files: `container/openclaw-src/vitest.unit.config.ts`

- [x] 2. Implement `shouldCompact()` threshold utility with TDD

  **What to do**:
  - **TDD: Write tests first** in `container/openclaw-src/src/agents/compaction/should-compact.test.ts`
  - Then implement `shouldCompact()` in `container/openclaw-src/src/agents/compaction/should-compact.ts`
  - The function signature:
    ```typescript
    export function shouldCompact(params: {
      messages: AgentMessage[];
      contextWindowTokens: number;
      compactionCount: number;
      lastProactiveCompactionCount: number;
      threshold?: number; // default PROACTIVE_COMPACTION_RATIO = 0.75
    }): { shouldCompact: boolean; reason: string }
    ```
  - Export a named constant: `export const PROACTIVE_COMPACTION_RATIO = 0.75;`
  - Export a named constant: `export const MIN_MESSAGES_FOR_COMPACTION = 20;`
  - Logic:
    1. If `messages.length < MIN_MESSAGES_FOR_COMPACTION` → `false` (too few messages to waste an LLM call on)
    2. If `lastProactiveCompactionCount === compactionCount` → `false` (anti-thrash: already compacted at this compaction count)
    3. Compute `tokenEstimate = estimateMessagesTokens(messages)`
    4. If `tokenEstimate >= contextWindowTokens * threshold` → `true`
    5. Else → `false`
  - Follow the pattern of `shouldRunMemoryFlush()` in `memory-flush.ts:113-143` (threshold + double-fire prevention)

  **Test cases**:
  - Token usage >= 75% threshold → returns true
  - Token usage < 75% threshold → returns false
  - Messages < MIN_MESSAGES_FOR_COMPACTION → returns false
  - lastProactiveCompactionCount === compactionCount → returns false (anti-thrash)
  - Custom threshold override → respects custom value
  - Returns reason string for logging/debugging

  **Must NOT do**:
  - Do NOT reinvent token counting — use existing `estimateMessagesTokens()` from `compaction.ts:17`
  - Do NOT add configuration system — just named constants
  - Do NOT modify existing `estimateMessagesTokens()` or `CONTEXT_INPUT_HEADROOM_RATIO`
  - Do NOT use `as any` or `@ts-ignore`

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: [`test-driven-development`]
    - `test-driven-development`: TDD workflow (RED → GREEN → REFACTOR) for this pure utility

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Task 1)
  - **Blocks**: Tasks 5, 6
  - **Blocked By**: Task 1 (vitest config must exist to run tests)

  **References**:
  - `container/openclaw-src/src/agents/compaction.ts:17` — `estimateMessagesTokens()` function to use
  - `container/openclaw-src/src/agents/memory-flush.ts:113-143` — `shouldRunMemoryFlush()` pattern to follow (threshold + anti-thrash)
  - `container/openclaw-src/src/agents/compaction.ts:387` — `resolveContextWindowTokens()` for context window resolution
  - `container/openclaw-src/src/agents/context-window-guard.ts` — `resolveContextWindowInfo()` for full multi-source resolution
  - `container/openclaw-src/src/agents/tool-result-context-guard.ts:5` — `CONTEXT_INPUT_HEADROOM_RATIO = 0.75` (reference constant, do NOT modify)

  **Acceptance Criteria**:
  - [x] `container/openclaw-src/src/agents/compaction/should-compact.ts` exists with `shouldCompact()` function
  - [x] `container/openclaw-src/src/agents/compaction/should-compact.test.ts` exists with ≥ 6 test cases
  - [x] `cd container/openclaw-src && pnpm vitest run src/agents/compaction/should-compact.test.ts` passes
  - [x] Exports `PROACTIVE_COMPACTION_RATIO` and `MIN_MESSAGES_FOR_COMPACTION` constants
  - [x] No `as any` or `@ts-ignore`

  **QA Scenarios**:
  ```
  Scenario: Verify shouldCompact tests pass
    Tool: Bash
    Steps:
      1. cd container/openclaw-src && pnpm vitest run src/agents/compaction/should-compact.test.ts
    Expected Result: All tests pass, 0 failures
    Evidence: .sisyphus/evidence/task-2-should-compact-tests.txt

  Scenario: Verify shouldCompact type checks
    Tool: Bash
    Steps:
      1. cd container/openclaw-src && pnpm tsc --noEmit 2>&1 | grep -i "should-compact" || echo "No type errors in should-compact"
    Expected Result: No type errors
    Evidence: .sisyphus/evidence/task-2-typecheck.txt
  ```

  **Commit**: YES
  - Message: `feat(compaction): add shouldCompact threshold utility with anti-thrash guard`
  - Files: `container/openclaw-src/src/agents/compaction/should-compact.ts`, `container/openclaw-src/src/agents/compaction/should-compact.test.ts`

- [x] 3. Extend `PluginHookAgentEndEvent` with optional `sessionFile`

  **What to do**:
  - In `container/openclaw-src/src/plugins/types.ts`, find `PluginHookAgentEndEvent` type definition
  - Add `sessionFile?: string` as an optional field (backward-compatible)
  - In `container/openclaw-src/src/agents/pi-embedded-runner/run/attempt.ts`, at the `agent_end` hook emission point (~line 1151-1170), pass `params.sessionFile` in the event object
  - Run `lsp_find_references` on `PluginHookAgentEndEvent` to find all consumers — verify none break
  - Run `ast_grep_search` for `agent_end` to verify all call sites

  **Must NOT do**:
  - Do NOT make `sessionFile` required — it's optional for backward compatibility
  - Do NOT modify any other hook event types in this task (Task 4 handles before_prompt_build)
  - Do NOT change the existing `agent_end` hook emission logic (just add the field)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Task 4)
  - **Blocks**: Task 5
  - **Blocked By**: None

  **References**:
  - `container/openclaw-src/src/plugins/types.ts` — `PluginHookAgentEndEvent` type definition
  - `container/openclaw-src/src/agents/pi-embedded-runner/run/attempt.ts:1148-1171` — agent_end hook emission (the call site to add sessionFile param)
  - `container/openclaw-src/src/plugins/hooks.ts:268-276` — `runAgentEnd()` implementation (verify it passes the event through)
  - `before_compaction`/`after_compaction` event types in types.ts — these already include `sessionFile`, follow their pattern

  **Acceptance Criteria**:
  - [x] `PluginHookAgentEndEvent` type has `sessionFile?: string` field
  - [x] `attempt.ts` passes `params.sessionFile` in the `agent_end` event emission
  - [x] `cd container/openclaw-src && pnpm tsc --noEmit` passes (no type errors in consumers)
  - [x] All existing consumers of `agent_end` still compile without changes

  **QA Scenarios**:
  ```
  Scenario: Verify type extension compiles
    Tool: Bash
    Steps:
      1. cd container/openclaw-src && pnpm tsc --noEmit
    Expected Result: Exit 0, no type errors
    Evidence: .sisyphus/evidence/task-3-typecheck.txt

  Scenario: Verify sessionFile is passed in emission
    Tool: Bash (grep)
    Steps:
      1. grep -n "sessionFile" container/openclaw-src/src/agents/pi-embedded-runner/run/attempt.ts | grep -i "agent_end\|runAgentEnd"
    Expected Result: At least 1 match showing sessionFile passed in the agent_end event
    Evidence: .sisyphus/evidence/task-3-sessionfile-passed.txt
  ```

  **Commit**: YES
  - Message: `feat(hooks): extend PluginHookAgentEndEvent with sessionFile`
  - Files: `container/openclaw-src/src/plugins/types.ts`, `container/openclaw-src/src/agents/pi-embedded-runner/run/attempt.ts`

- [x] 4. Extend `PluginHookBeforePromptBuildEvent` with optional `sessionFile`

  **What to do**:
  - In `container/openclaw-src/src/plugins/types.ts`, find `PluginHookBeforePromptBuildEvent` type definition
  - Add `sessionFile?: string` as an optional field (backward-compatible)
  - In `container/openclaw-src/src/agents/pi-embedded-runner/run/attempt.ts`, at the `before_prompt_build` hook emission point (~line 888-901), pass `params.sessionFile` in the event object
  - Run `lsp_find_references` on `PluginHookBeforePromptBuildEvent` to find all consumers — verify none break

  **Must NOT do**:
  - Do NOT make `sessionFile` required — optional for backward compatibility
  - Do NOT modify `PluginHookAgentEndEvent` (Task 3 handles that)
  - Do NOT change the existing `before_prompt_build` hook emission logic (just add the field)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Task 3)
  - **Blocks**: Task 6
  - **Blocked By**: None

  **References**:
  - `container/openclaw-src/src/plugins/types.ts` — `PluginHookBeforePromptBuildEvent` type definition
  - `container/openclaw-src/src/agents/pi-embedded-runner/run/attempt.ts:888-901` — before_prompt_build hook emission (the call site to add sessionFile param)
  - `container/openclaw-src/src/plugins/hooks.ts:233-241` — `runBeforePromptBuild()` implementation
  - Task 3's changes — same pattern, different event type

  **Acceptance Criteria**:
  - [x] `PluginHookBeforePromptBuildEvent` type has `sessionFile?: string` field
  - [x] `attempt.ts` passes `params.sessionFile` in the `before_prompt_build` event emission
  - [x] `cd container/openclaw-src && pnpm tsc --noEmit` passes
  - [x] All existing consumers of `before_prompt_build` still compile without changes

  **QA Scenarios**:
  ```
  Scenario: Verify type extension compiles
    Tool: Bash
    Steps:
      1. cd container/openclaw-src && pnpm tsc --noEmit
    Expected Result: Exit 0, no type errors
    Evidence: .sisyphus/evidence/task-4-typecheck.txt

  Scenario: Verify sessionFile is passed in emission
    Tool: Bash (grep)
    Steps:
      1. grep -n "sessionFile" container/openclaw-src/src/agents/pi-embedded-runner/run/attempt.ts | grep -i "before_prompt\|runBeforePrompt"
    Expected Result: At least 1 match showing sessionFile passed in the before_prompt_build event
    Evidence: .sisyphus/evidence/task-4-sessionfile-passed.txt
  ```

  **Commit**: YES
  - Message: `feat(hooks): extend PluginHookBeforePromptBuildEvent with sessionFile`
  - Files: `container/openclaw-src/src/plugins/types.ts`, `container/openclaw-src/src/agents/pi-embedded-runner/run/attempt.ts`

- [x] 5. Register proactive compaction handler on `agent_end` hook

  **What to do**:
  - Create `container/openclaw-src/src/agents/pi-embedded-runner/proactive-compaction.ts`
  - Create co-located test file `proactive-compaction.test.ts`
  - Implement a handler function that:
    1. Gates on `success === true` — skip if run failed
    2. Checks if error is a context overflow via `isLikelyContextOverflowError()` — skip if so (reactive path handles it)
    3. Resolves context window tokens using `resolveContextWindowTokens()` or the model from the event
    4. Calls `shouldCompact()` with messages, context window tokens, and compaction tracking state
    5. If `shouldCompact()` returns true, calls `compactEmbeddedPiSessionDirect({ sessionFile, config, ... })` fire-and-forget (`.catch()` for error logging)
  - Register the handler on `agent_end` via the plugin hook system
  - The handler is fire-and-forget — it runs asynchronously after the hook fires
  - `compactEmbeddedPiSessionDirect()` opens its own SessionManager (line 525) and acquires its own write lock (line 508-513) — safe to run concurrently
  - Track `lastProactiveCompactionCount` to prevent thrashing (follow `shouldRunMemoryFlush` pattern)

  **Must NOT do**:
  - Do NOT use `compactEmbeddedPiSession()` (with lane queueing) — deadlock risk
  - Do NOT modify the existing `agent_end` hook emission in attempt.ts (Task 3 already extended it)
  - Do NOT await the compaction call — fire-and-forget with `.catch()`
  - Do NOT modify existing reactive compaction logic

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: [`test-driven-development`]
    - `test-driven-development`: TDD workflow for handler implementation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Task 6)
  - **Blocks**: F1-F4
  - **Blocked By**: Tasks 2, 3

  **References**:
  - `container/openclaw-src/src/agents/compaction/should-compact.ts` — Task 2's `shouldCompact()` utility
  - `container/openclaw-src/src/agents/pi-embedded-runner/compact.ts:244-720` — `compactEmbeddedPiSessionDirect()` implementation (opens own SessionManager at line 525, own write lock at 508-513)
  - `container/openclaw-src/src/agents/pi-embedded-runner/run/attempt.ts:1148-1171` — `agent_end` hook emission point (now with sessionFile from Task 3)
  - `container/openclaw-src/src/agents/pi-embedded-helpers/errors.ts:63` — `isLikelyContextOverflowError()` for the overflow exclusion check
  - `container/openclaw-src/src/agents/memory-flush.ts:113-143` — `shouldRunMemoryFlush()` pattern (anti-thrash tracking with `lastFlushAt === compactionCount`)
  - `container/openclaw-src/src/plugins/hooks.ts:268-276` — `runAgentEnd()` implementation showing event shape
  - `container/openclaw-src/src/plugins/hooks.test-helpers.ts` — `createMockPluginRegistry()` for test setup
  - Existing test pattern: `wired-hooks-compaction.test.ts` — hook wiring test structure (vi.hoisted + vi.mock + dynamic import)

  **Acceptance Criteria**:
  - [x] `proactive-compaction.ts` exists with agent_end handler
  - [x] `proactive-compaction.test.ts` exists with ≥ 4 test cases (success gate, overflow exclusion, threshold trigger, skip below threshold)
  - [x] Handler gates on `success === true`
  - [x] Handler excludes context-overflow errors
  - [x] Handler calls `shouldCompact()` before triggering compaction
  - [x] `cd container/openclaw-src && pnpm vitest run src/agents/pi-embedded-runner/proactive-compaction.test.ts` passes

  **QA Scenarios**:
  ```
  Scenario: Verify handler tests pass
    Tool: Bash
    Steps:
      1. cd container/openclaw-src && pnpm vitest run src/agents/pi-embedded-runner/proactive-compaction.test.ts
    Expected Result: All tests pass, 0 failures
    Evidence: .sisyphus/evidence/task-5-agent-end-handler-tests.txt

  Scenario: Verify handler skips on failure
    Tool: Bash (grep)
    Steps:
      1. grep "success" container/openclaw-src/src/agents/pi-embedded-runner/proactive-compaction.ts
    Expected Result: Handler checks success === true before proceeding
    Evidence: .sisyphus/evidence/task-5-success-gate.txt
  ```

  **Commit**: YES
  - Message: `feat(compaction): register proactive compaction on agent_end hook`
  - Files: `container/openclaw-src/src/agents/pi-embedded-runner/proactive-compaction.ts`, `container/openclaw-src/src/agents/pi-embedded-runner/proactive-compaction.test.ts`

- [x] 6. Register `before_prompt_build` safety net handler

  **What to do**:
  - Add a `before_prompt_build` handler in the same `proactive-compaction.ts` module (or a separate `proactive-compaction-safety-net.ts` if the agent prefers separation)
  - Create co-located test file if separate module
  - Implement a handler that:
    1. Checks if a cooldown period has passed since last proactive compaction check (to avoid per-call overhead during multi-step tool use)
    2. Resolves context window tokens
    3. Calls `shouldCompact()` with messages and context info
    4. If true, calls `compactEmbeddedPiSessionDirect({ sessionFile, ... })` — but this is inside the write lock context and the session is active, so it MUST use `compactEmbeddedPiSessionDirect()` not the lane-aware version
    5. Unlike `agent_end`, this hook IS awaited — so the handler can await compaction before the prompt is sent
  - The cooldown mechanism: track timestamp of last check, skip if < N seconds since last. Use a simple module-level variable or a Map keyed by sessionId.

  **Must NOT do**:
  - Do NOT use `compactEmbeddedPiSession()` (lane-aware) — deadlock risk
  - Do NOT run the check on every single LLM call without cooldown (fires during multi-step tool use loops)
  - Do NOT modify the existing `before_prompt_build` hook emission in attempt.ts (Task 4 already extended it)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: [`test-driven-development`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Task 5)
  - **Blocks**: F1-F4
  - **Blocked By**: Tasks 2, 4

  **References**:
  - `container/openclaw-src/src/agents/compaction/should-compact.ts` — Task 2's utility
  - `container/openclaw-src/src/agents/pi-embedded-runner/run/attempt.ts:888-901` — `before_prompt_build` hook emission (now with sessionFile from Task 4)
  - `container/openclaw-src/src/agents/pi-embedded-runner/compact.ts:244-720` — `compactEmbeddedPiSessionDirect()`
  - Task 5's `proactive-compaction.ts` — if using same file, coordinate with Task 5 agent

  **Acceptance Criteria**:
  - [x] `before_prompt_build` handler exists (either in same proactive-compaction.ts or separate file)
  - [x] Test file exists with ≥ 3 test cases (threshold trigger, cooldown skip, compact call)
  - [x] Handler has cooldown mechanism to avoid per-call overhead
  - [x] Handler calls `shouldCompact()` before triggering compaction
  - [x] `cd container/openclaw-src && pnpm vitest run` passes for the test file

  **QA Scenarios**:
  ```
  Scenario: Verify safety net handler tests pass
    Tool: Bash
    Steps:
      1. cd container/openclaw-src && pnpm vitest run src/agents/pi-embedded-runner/proactive-compaction.test.ts
    Expected Result: All tests pass (including safety net tests)
    Evidence: .sisyphus/evidence/task-6-safety-net-tests.txt

  Scenario: Verify cooldown exists
    Tool: Bash (grep)
    Steps:
      1. grep -i "cooldown\|lastCheck\|last.*check" container/openclaw-src/src/agents/pi-embedded-runner/proactive-compaction.ts
    Expected Result: At least 1 match showing cooldown tracking
    Evidence: .sisyphus/evidence/task-6-cooldown.txt
  ```

  **Commit**: YES
  - Message: `feat(compaction): register before_prompt_build safety net handler`
  - Files: `container/openclaw-src/src/agents/pi-embedded-runner/proactive-compaction.ts` (or new file), test file

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `tsc --noEmit` + `pnpm test:fast --run`. Review all changed files for: `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration. Test edge cases: empty session, single message, already-compacted session. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Task 1**: `chore: add vitest.unit.config.ts to restore test:fast script`
- **Task 2**: `feat(compaction): add shouldCompact threshold utility with anti-thrash guard`
- **Task 3**: `feat(hooks): extend PluginHookAgentEndEvent with sessionFile`
- **Task 4**: `feat(hooks): extend PluginHookBeforePromptBuildEvent with sessionFile`
- **Task 5**: `feat(compaction): register proactive compaction on agent_end hook`
- **Task 6**: `feat(compaction): register before_prompt_build safety net handler`

---

## Success Criteria

### Verification Commands
```bash
cd container/openclaw-src && pnpm test:fast --run   # All tests pass
cd container/openclaw-src && pnpm tsc --noEmit       # Type check passes
grep -c "shouldCompact" src/agents/compaction.ts      # ≥ 1
grep -c "sessionFile" src/plugins/types.ts            # ≥ 3 (2 new optional fields + existing)
grep -c "agent_end" src/agents/pi-embedded-runner/run/attempt.ts  # Hook still fires
grep -c "before_prompt_build" src/agents/pi-embedded-runner/run/attempt.ts  # Hook still fires
```

### Final Checklist
- [x] All "Must Have" present
- [x] All "Must NOT Have" absent
- [x] All tests pass
- [x] No existing tests broken
