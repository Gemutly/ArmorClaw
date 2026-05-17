# Plan: Documentation Gap Triage

## TL;DR

> **Quick Summary**: Fix `doc/armorclaw.md` structural issues and fill critical coverage gaps using a core + per-component approach. Main doc stays focused on the value chain; peripheral systems get dedicated docs that the main doc links to.
> 
> **Deliverables**:
> - Fixed `doc/armorclaw.md` (Package Index, TOC, component tags, summary paragraphs with links, orphan cleanup)
> - `doc/voice-stack.md` (audio, voice, webrtc, turn)
> - `doc/sidecar-pipeline.md` (Rust gRPC pipeline, YARA scanner, Go client)
> - `doc/secretary-workflow.md` (workflow engine, task scheduler, approvals, PII blocking)
> - `doc/agent-runtime.md` (memory, cache, executor, speculative execution)
> - `doc/license-system.md` (license-server, enforcement, governor)
> - `doc/client-applications.md` (admin-panel, ArmorTerminal, setup-wizard, OpenClaw UI)
> - `doc/communication-infra.md` (push notifications, SSO, WebSocket, event bus internals)
> 
> **Estimated Effort**: Medium (8 tasks, mostly writing)
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: T1 (main doc fixes) → T2-T8 (per-component docs, parallel) → F1-F4 (verification)

---

## Context

### Original Request
Gap analysis revealed `doc/armorclaw.md` covers ~40% of the codebase (20 of 80 packages in the Package Index, 12+ missing subsystem sections). The doc is 3042 lines — too long to double, but too incomplete to trust as authoritative. User chose triage approach: keep main doc focused, create per-component docs for peripheral systems.

### Interview Summary
**Key Discussions**:
- User chose "Triage: core + per-component" over monolith or quick-fix approaches
- Main doc should not grow significantly — add summary paragraphs with links, not full sections
- Existing scattered docs (sidecar/README.md, 52 SKILL.md files) must be scavenged, not rewritten

**Research Findings**:
- 53 of 80 packages undocumented (43 `pkg/` + 10 `internal/`)
- 12 malformed `<component>` HTML tags at lines 100-134
- `doc/ArmorChat.md` already exists (5340 lines) — must not duplicate
- `sidecar/README.md` is production-ready — scavenge, don't rewrite
- 37 OpenClaw platform extensions and 31+ skills are out of scope for this plan
- Version number conflict: doc says 4.10.0, README says 4.8.0

### Metis Review
**Identified Gaps** (addressed):
- Corrected package gap count from "~35 of ~65" to "53 of 80"
- Split `operational-governance.md` grouping (zero cross-dependencies between packages)
- Added `pkg/secretary/` as most architecturally significant Package Index omission
- Added malformed `<component>` tag cleanup
- Added `license-system.md` as new doc (was unassigned)
- Expanded `agent-memory.md` scope to include cache, executor, speculative execution
- Mandated scavenge-first approach for existing scattered docs

---

## Work Objectives

### Core Objective
Make `doc/armorclaw.md` a trustworthy LLM-readable index of the entire codebase by fixing structural issues, filling the Package Index, and linking to dedicated per-component docs for systems that need more than a summary paragraph.

### Concrete Deliverables
- 1 modified file: `doc/armorclaw.md`
- 7 new files: `doc/voice-stack.md`, `doc/sidecar-pipeline.md`, `doc/secretary-workflow.md`, `doc/agent-runtime.md`, `doc/license-system.md`, `doc/client-applications.md`, `doc/communication-infra.md`

### Definition of Done
- [ ] `doc/armorclaw.md` Package Index covers 40+ packages (up from 20)
- [ ] TOC matches every `##` heading exactly (no orphans, no missing entries)
- [ ] Every `##` section has either detailed content or a summary paragraph linking to a per-component doc
- [ ] All 12 malformed `<component>` tags fixed
- [ ] All 7 per-component docs follow consistent template: Overview → Architecture → Key Packages → Configuration → Integration Points
- [ ] No content duplication between new docs and existing scattered docs (sidecar/README.md, SKILL.md files)

### Must Have
- Package Index covers all bridge/pkg/ packages with at least one-line descriptions
- `pkg/secretary/` documented (most significant omission)
- TOC is accurate and complete
- Malformed component tags fixed
- Main doc line count does not grow by more than 200 lines (summary paragraphs + Package Index rows, not full sections)

### Must NOT Have (Guardrails)
- Do NOT duplicate content from `sidecar/README.md`, SKILL.md files, or `docs/guides/` — link or synthesize
- Do NOT cover all 37 OpenClaw extensions or 31+ undocumented skills — deferred to future work
- Do NOT cover infrastructure services (Qdrant, Coturn, Squid, Nginx, Synapse, PostgreSQL) — deferred
- Do NOT cover the container build pipeline — deferred
- Do NOT expand main doc beyond ~3200 lines — summary paragraphs only
- Do NOT group unrelated packages under a single doc title without framing them as independent subsystems
- Do NOT remove `<component>` tags entirely — some tooling may depend on them; only fix the malformed ones

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** - ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: N/A (documentation only)
- **Automated tests**: N/A
- **Framework**: none

### QA Policy
Every task includes agent-executed QA scenarios. Evidence saved to `.sisyphus/evidence/`.

- **Documentation QA**: Use `grep` to verify links, `wc -l` for size checks, heading pattern matching for structure
- **Content QA**: Verify Package Index row count, TOC completeness, no duplicate content

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately - main doc structural fixes):
└── Task 1: Fix doc/armorclaw.md structural issues + Package Index + TOC [writing]

Wave 2 (After Wave 1 - all parallel, independent docs):
├── Task 2: doc/voice-stack.md [writing]
├── Task 3: doc/sidecar-pipeline.md [writing]
├── Task 4: doc/secretary-workflow.md [writing]
├── Task 5: doc/agent-runtime.md [writing]
├── Task 6: doc/license-system.md [writing]
├── Task 7: doc/client-applications.md [writing]
└── Task 8: doc/communication-infra.md [writing]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Doc quality review (unspecified-high)
├── Task F3: Link verification (unspecified-high)
└── Task F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| T1   | None      | T2-T8 (needs summary paragraph locations) |
| T2-T8| T1 (for link targets) | F1-F4 |
| F1-F4| T1-T8 | User okay |

### Agent Dispatch Summary

- **Wave 1**: 1 task — T1 → `writing`
- **Wave 2**: 7 tasks — T2-T8 → `writing` (all parallel)
- **FINAL**: 4 tasks — F1 → `oracle`, F2-F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] T1. Fix doc/armorclaw.md structural issues, Package Index, and TOC

  **What to do**:
  - Fix 12 malformed `<component>` HTML tags at lines 100-134 (8 missing closing `"` quotes, 1 duplicate ID)
  - Update version number to match source of truth (verify README 4.8.0 vs doc 4.10.0)
  - Expand Package Index from 20 to 40+ packages by adding missing packages in existing categories:
    - **Missing from Control Plane**: `pkg/secretary/` (workflow engine, task scheduler, approvals — most significant omission), `pkg/health/`, `pkg/runtime/`
    - **Missing from Security & Trust**: `pkg/yara/`, `pkg/securerandom/`, `pkg/crypto/`
    - **Missing from Communication**: `pkg/push/`, `pkg/sso/`, `pkg/websocket/`, `pkg/translator/`, `pkg/matrixcmd/`, `pkg/notification/`
    - **Missing from Container & Runtime**: `pkg/sidecar/`, `pkg/ghost/`, `pkg/setup/`, `pkg/docker/`
    - **Missing from Observability**: `pkg/eventlog/`, `pkg/metrics/` (internal), `internal/memory/`, `internal/cache/`, `internal/executor/`
    - **New category "Real-Time Communication"**: `pkg/audio/`, `pkg/voice/`, `pkg/webrtc/`, `pkg/turn/`
    - **New category "Identity & Access"**: `pkg/license/`, `pkg/permissions/`, `pkg/invite/`, `pkg/admin/`
  - Update TOC to match all `##` headings (add missing entries: Agent State Machine, etc.)
  - Move orphan Agent State Machine section (line 2956) to proper location in the TOC order
  - Remove or fill empty Review Documentation stub (line 3033)
  - For each subsystem that now has a per-component doc, replace any existing stub with a 3-5 line summary paragraph ending with `> See [doc/xxx.md](xxx.md) for full documentation.`
  - Add 12 missing TOC entries to match the new/actual section structure

  **Must NOT do**:
  - Do NOT add full sections for peripheral systems — summary paragraphs with links only
  - Do NOT remove `<component>` tags entirely — only fix the malformed ones
  - Do NOT grow the doc beyond ~3200 lines (currently 3042)
  - Do NOT rewrite existing content that is accurate

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (foundation for T2-T8)
  - **Parallel Group**: Wave 1 (solo)
  - **Blocks**: T2, T3, T4, T5, T6, T7, T8
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `doc/armorclaw.md:361-422` — Existing Package Index structure (6 categories with tables)
  - `doc/armorclaw.md:35-62` — Existing TOC (25 entries)
  - `doc/armorclaw.md:100-134` — Malformed `<component>` tags
  - `doc/armorclaw.md:2956-3042` — Orphan Agent State Machine section + empty Review Documentation stub

  **Source of Truth References**:
  - `README.md:4` — Version badge (says 4.8.0)
  - `doc/armorclaw.md:5` — Version (says 4.10.0) — reconcile these
  - `bridge/pkg/` — Full directory listing (65 packages) for Package Index completeness check

  **Existing Per-Component Docs to Link To**:
  - `doc/ArmorChat.md` — 5340 lines, already exists. Do NOT create ArmorChat content in new docs.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Package Index completeness
    Tool: Bash (grep)
    Steps:
      1. grep -c "| \`pkg/" doc/armorclaw.md — count pkg/ entries in Package Index
      2. Verify count >= 40 (up from 20)
    Expected Result: Count is 40 or more
    Evidence: .sisyphus/evidence/task-t1-package-index-count.txt

  Scenario: TOC matches headings
    Tool: Bash (grep)
    Steps:
      1. grep "^## " doc/armorclaw.md | sed 's/## //' > /tmp/headings.txt
      2. grep "^\d\+\. \[" doc/armorclaw.md | sed 's/.*\] //' > /tmp/toc.txt
      3. diff /tmp/headings.txt /tmp/toc.txt
    Expected Result: No missing entries (diff shows only formatting differences)
    Evidence: .sisyphus/evidence/task-t1-toc-match.txt

  Scenario: Component tags well-formed
    Tool: Bash (grep)
    Steps:
      1. grep -n '<component' doc/armorclaw.md
      2. Verify all have matching closing tags and properly quoted IDs
    Expected Result: All component tags have id="..." format with closing tags
    Evidence: .sisyphus/evidence/task-t1-component-tags.txt

  Scenario: Line count within bounds
    Tool: Bash (wc)
    Steps:
      1. wc -l doc/armorclaw.md
    Expected Result: <= 3200 lines
    Evidence: .sisyphus/evidence/task-t1-line-count.txt
  ```

  **Commit**: YES
  - Message: `docs(armorclaw): fix Package Index, TOC, and structural issues`
  - Files: `doc/armorclaw.md`

---

- [x] T2. Create doc/voice-stack.md

  **What to do**:
  - Scavenge source files to understand each package's purpose:
    - `bridge/pkg/audio/` (opus.go, pcm.go — audio encoding)
    - `bridge/pkg/voice/` (budget.go — voice call budget tracking)
    - `bridge/pkg/webrtc/` (engine.go, session.go — WebRTC session management)
    - `bridge/pkg/turn/` (turn.go — TURN/STUN relay configuration)
  - Write doc following template: Overview → Architecture → Key Packages → Configuration → Integration Points
  - Include ASCII diagram showing audio flow: phone → TURN → WebRTC → Bridge → agent
  - Document how voice budget integrates with the main budget system

  **Must NOT do**:
  - Do NOT document infrastructure services (Coturn, etc.) — deferred
  - Do NOT write code, only documentation

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T3-T8)
  - **Blocks**: F1-F4
  - **Blocked By**: T1 (for link target location in main doc)

  **References**:

  **Pattern References**:
  - `doc/armorclaw.md:1664-1707` — Browser Service section (example of a well-structured per-component section)

  **Source References**:
  - `bridge/pkg/audio/opus.go` — Opus audio encoding
  - `bridge/pkg/audio/pcm.go` — PCM audio processing
  - `bridge/pkg/voice/budget.go` — Voice call budget tracking
  - `bridge/pkg/webrtc/engine.go` — WebRTC engine
  - `bridge/pkg/webrtc/session.go` — WebRTC session management
  - `bridge/pkg/turn/turn.go` — TURN relay configuration

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: Doc follows template
    Tool: Bash (grep)
    Steps:
      1. Verify file has sections: Overview, Architecture, Key Packages, Configuration
      2. grep "^## \|^### " doc/voice-stack.md
    Expected Result: At least 4 sections present
    Evidence: .sisyphus/evidence/task-t2-structure.txt

  Scenario: All 4 packages documented
    Tool: Bash (grep)
    Steps:
      1. grep "pkg/audio\|pkg/voice\|pkg/webrtc\|pkg/turn" doc/voice-stack.md
    Expected Result: All 4 packages referenced
    Evidence: .sisyphus/evidence/task-t2-package-coverage.txt
  ```

  **Commit**: YES (groups with T3-T8)
  - Message: `docs: add per-component documentation for voice, sidecar, secretary, agent-runtime, license, clients, comms`
  - Files: `doc/voice-stack.md`

---

- [x] T3. Create doc/sidecar-pipeline.md

  **What to do**:
  - Scavenge existing `sidecar/README.md` FIRST (production-ready content)
  - Document the Rust gRPC document pipeline: connectors, document processing, encryption, provenance chain
  - Include Go client: `bridge/pkg/sidecar/` (audit_client.go, client.go)
  - Include YARA scanner: `bridge/pkg/yara/` (scanner.go — content disarm and reconstruction)
  - Write doc following template
  - Note: This is NOT the same as `rust-vault/` — sidecar handles documents, vault handles secrets

  **Must NOT do**:
  - Do NOT rewrite content from `sidecar/README.md` — incorporate/synthesize
  - Do NOT confuse with `rust-vault/` (different component)

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2
  - **Blocks**: F1-F4
  - **Blocked By**: T1

  **References**:

  **Critical — Scavenge First**:
  - `sidecar/README.md` — Existing comprehensive documentation (READ THIS FIRST)

  **Source References**:
  - `sidecar/src/connectors/` — Document connector implementations
  - `sidecar/src/document/` — Document processing
  - `sidecar/src/encryption/` — Encryption layer
  - `sidecar/src/provenance/` — Provenance chain
  - `sidecar/src/grpc/` — gRPC service definitions
  - `bridge/pkg/sidecar/client.go` — Go gRPC client
  - `bridge/pkg/sidecar/audit_client.go` — Go audit client
  - `bridge/pkg/yara/scanner.go` — YARA content scanner

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: Doc does not duplicate sidecar/README.md
    Tool: Bash (grep)
    Steps:
      1. wc -l doc/sidecar-pipeline.md
      2. wc -l sidecar/README.md
      3. Verify new doc is shorter or synthesizes (not copy-paste)
    Expected Result: New doc provides architecture overview, not raw copy
    Evidence: .sisyphus/evidence/task-t3-no-duplication.txt

  Scenario: YARA and Go client included
    Tool: Bash (grep)
    Steps:
      1. grep "pkg/yara\|pkg/sidecar" doc/sidecar-pipeline.md
    Expected Result: Both Go packages referenced
    Evidence: .sisyphus/evidence/task-t3-go-packages.txt
  ```

  **Commit**: YES (groups with T2, T4-T8)
  - Files: `doc/sidecar-pipeline.md`

---

- [x] T4. Create doc/secretary-workflow.md

  **What to do**:
  - This is the most architecturally significant missing documentation.
  - Document the full secretary/workflow system:
    - `bridge/pkg/secretary/` — workflow engine, task scheduler, approval engine, notifications, pending approval (PII blocking), orchestrator integration
    - `bridge/internal/events/matrix_event_bus.go` — Matrix event bus used for PII approval
  - Document the two dispatch paths (workflow engine vs warm/cold dispatch)
  - Document PII approval flow (PendingApproval → Matrix event → HandlePIIResponse → approved/denied/timeout)
  - Document config passthrough (STEP_CONFIG env var)
  - Document data flow limitation (no structured results from containers, exit-code only)

  **Must NOT do**:
  - Do NOT duplicate content from the Workflow Execution Lifecycle section in main doc — this doc provides the DEEP dive, main doc has the summary

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2
  - **Blocks**: F1-F4
  - **Blocked By**: T1

  **References**:

  **Source References**:
  - `bridge/pkg/secretary/orchestrator.go` — WorkflowOrchestratorImpl, DependencyValidator
  - `bridge/pkg/secretary/orchestrator_integration.go` — StepExecutor, executeStep, waitForCompletion, PendingApproval
  - `bridge/pkg/secretary/orchestrator_events.go` — EventEmitter interface, WorkflowEventEmitter
  - `bridge/pkg/secretary/approvals.go` — ApprovalEngineImpl, EvaluateStep
  - `bridge/pkg/secretary/pending_approval.go` — PendingApproval, HandlePIIResponse
  - `bridge/pkg/secretary/notifications.go` — NotificationService
  - `bridge/pkg/secretary/task_scheduler.go` — TaskScheduler dispatch logic
  - `bridge/pkg/secretary/types.go` — WorkflowStep, StepType, ApprovalResult, Workflow, TaskTemplate
  - `bridge/internal/events/matrix_event_bus.go` — MatrixEventBus (used for PII events)

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: All secretary packages documented
    Tool: Bash (grep)
    Steps:
      1. grep "orchestrator\|approvals\|pending_approval\|notifications\|task_scheduler\|types" doc/secretary-workflow.md
    Expected Result: All 6 key files referenced
    Evidence: .sisyphus/evidence/task-t4-package-coverage.txt
  ```

  **Commit**: YES (groups with T2, T3, T5-T8)
  - Files: `doc/secretary-workflow.md`

---

- [x] T5. Create doc/agent-runtime.md

  **What to do**:
  - Document the agent runtime internals:
    - `bridge/internal/agent/` — Runtime, Task types
    - `bridge/internal/memory/` — Store, checkpoint, batch operations
    - `bridge/internal/cache/` — LRU cache, rate limiting
    - `bridge/internal/executor/` — ToolExecutor, skill registry, tool pool
    - `bridge/internal/speculative/` — SpeculativeExecutor (speculative tool execution)
  - Document how the agent runtime connects to the container lifecycle (factory.Spawn → waitForCompletion → StepResult)

  **Must NOT do**:
  - Do NOT duplicate content from doc/armorclaw.md Agent State Machine section — link to it instead

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2
  - **Blocks**: F1-F4
  - **Blocked By**: T1

  **References**:

  **Source References**:
  - `bridge/internal/agent/runtime.go` — Runtime struct, NewRuntime, RunTask
  - `bridge/internal/agent/types.go` — Task type
  - `bridge/internal/memory/store.go` — Memory store
  - `bridge/internal/memory/checkpoint.go` — Checkpoint logic
  - `bridge/internal/memory/batch.go` — Batch operations
  - `bridge/internal/cache/lru.go` — LRU cache
  - `bridge/internal/cache/ratelimit.go` — Rate limiting cache
  - `bridge/internal/executor/engine.go` — ToolExecutor
  - `bridge/internal/speculative/executor.go` — SpeculativeExecutor

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: All 5 internal packages documented
    Tool: Bash (grep)
    Steps:
      1. grep "internal/agent\|internal/memory\|internal/cache\|internal/executor\|internal/speculative" doc/agent-runtime.md
    Expected Result: All 5 packages referenced
    Evidence: .sisyphus/evidence/task-t5-package-coverage.txt
  ```

  **Commit**: YES (groups with T2-T4, T6-T8)
  - Files: `doc/agent-runtime.md`

---

- [x] T6. Create doc/license-system.md

  **What to do**:
  - Document the license validation system:
    - `license-server/` — Standalone Go microservice (main.go, main_test.go, Dockerfile, docker-compose.yml)
    - `bridge/pkg/license/` — Client, state_manager
    - `bridge/pkg/enforcement/` — Enforcement, bridge_integration
  - Document how the Bridge checks license state on startup and during operation
  - Document enforcement behavior (what happens when license expires)

  **Must NOT do**:
  - Do NOT expose license validation algorithm details — high-level behavior only

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2
  - **Blocks**: F1-F4
  - **Blocked By**: T1

  **References**:

  **Source References**:
  - `license-server/main.go` — License server entry point
  - `license-server/main_test.go` — Server tests
  - `bridge/pkg/license/client.go` — License client
  - `bridge/pkg/license/state_manager.go` — License state management
  - `bridge/pkg/enforcement/enforcement.go` — Enforcement logic
  - `bridge/pkg/enforcement/bridge_integration.go` — Bridge integration hooks

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: All 3 components documented
    Tool: Bash (grep)
    Steps:
      1. grep "license-server\|pkg/license\|pkg/enforcement" doc/license-system.md
    Expected Result: All 3 components referenced
    Evidence: .sisyphus/evidence/task-t6-package-coverage.txt
  ```

  **Commit**: YES (groups with T2-T5, T7-T8)
  - Files: `doc/license-system.md`

---

- [x] T7. Create doc/client-applications.md

  **What to do**:
  - Document the client applications:
    - `applications/admin-panel/` — Web dashboard
    - `applications/ArmorTerminal/` — TUI client
    - `applications/setup-wizard/` — Vite+TypeScript setup wizard
    - `container/openclaw-src/ui/` — OpenClaw agent runtime UI
  - NOTE: ArmorChat already has `doc/ArmorChat.md` (5340 lines). Do NOT duplicate. Reference it.

  **Must NOT do**:
  - Do NOT document ArmorChat — it already has doc/ArmorChat.md
  - Do NOT duplicate content from existing application READMEs

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2
  - **Blocks**: F1-F4
  - **Blocked By**: T1

  **References**:

  **Source References**:
  - `applications/admin-panel/` — Web dashboard (explore directory structure)
  - `applications/ArmorTerminal/` — TUI client (explore directory structure)
  - `applications/setup-wizard/` — Setup wizard (Vite+TypeScript)
  - `container/openclaw-src/ui/` — OpenClaw UI (Vite+TypeScript)

  **Existing Docs to Reference**:
  - `doc/ArmorChat.md` — Already comprehensive, do NOT duplicate

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: ArmorChat not duplicated
    Tool: Bash (grep)
    Steps:
      1. grep -c "ArmorChat" doc/client-applications.md
      2. Verify ArmorChat is only referenced as "See doc/ArmorChat.md", not documented in full
    Expected Result: ArmorChat mentioned only as a reference/link
    Evidence: .sisyphus/evidence/task-t7-no-armorchat-dup.txt

  Scenario: All 4 applications documented
    Tool: Bash (grep)
    Steps:
      1. grep "admin-panel\|ArmorTerminal\|setup-wizard\|openclaw-src/ui" doc/client-applications.md
    Expected Result: All 4 applications referenced
    Evidence: .sisyphus/evidence/task-t7-app-coverage.txt
  ```

  **Commit**: YES (groups with T2-T6, T8)
  - Files: `doc/client-applications.md`

---

- [x] T8. Create doc/communication-infra.md

  **What to do**:
  - Document independent communication subsystems (framed as independent, not a cohesive system):
    - `bridge/pkg/push/` — Mobile push notifications via Matrix Sygnal (gateway.go, providers.go)
    - `bridge/pkg/sso/` — Single sign-on (sso.go)
    - `bridge/pkg/websocket/` — WebSocket server (websocket.go — may be stub)
    - `bridge/pkg/eventbus/` — Event bus internals (eventbus.go, errors.go)
    - `bridge/internal/adapter/` — Matrix, Discord, Telegram adapters (sdtw/)
  - Explicitly frame as "independent subsystems" — do NOT imply false coherence

  **Must NOT do**:
  - Do NOT group these as if they share architecture — they don't
  - Do NOT duplicate content from main doc Event Bus Patterns section

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2
  - **Blocks**: F1-F4
  - **Blocked By**: T1

  **References**:

  **Source References**:
  - `bridge/pkg/push/gateway.go` — Push notification gateway
  - `bridge/pkg/push/providers.go` — Push providers
  - `bridge/pkg/sso/sso.go` — SSO implementation
  - `bridge/pkg/websocket/websocket.go` — WebSocket server
  - `bridge/pkg/eventbus/eventbus.go` — Event bus core
  - `bridge/internal/adapter/` — Platform adapters (Matrix, Slack, etc.)
  - `bridge/internal/sdtw/` — Discord, Telegram, WhatsApp adapters

  **Existing Doc Sections to Link**:
  - `doc/armorclaw.md` Event Bus Patterns section — link, don't duplicate

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: All 6 packages documented
    Tool: Bash (grep)
    Steps:
      1. grep "pkg/push\|pkg/sso\|pkg/websocket\|pkg/eventbus\|internal/adapter\|internal/sdtw" doc/communication-infra.md
    Expected Result: All 6 packages referenced
    Evidence: .sisyphus/evidence/task-t8-package-coverage.txt
  ```

  **Commit**: YES (groups with T2-T7)
  - Files: `doc/communication-infra.md`

---

## Final Verification Wave

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify doc exists (read file, check content). For each "Must NOT Have": search docs for forbidden patterns. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Doc Quality Review** — `unspecified-high`
  Check all 8 files for consistent template: Overview → Architecture → Key Packages → Configuration → Integration Points. Verify no duplicate content between new docs and existing scattered docs (sidecar/README.md, SKILL.md files, docs/guides/). Check markdown formatting (headers, links, code blocks).
  Output: `Template [N/N consistent] | Duplicates [CLEAN/N found] | Formatting [CLEAN/N issues] | VERDICT`

- [x] F3. **Link Verification** — `unspecified-high`
  Extract all markdown links from all 8 doc files. Verify each link resolves to an existing file or section. Check that the main doc's summary paragraphs correctly link to the per-component docs. Check that no broken cross-references exist.
  Output: `Links [N/N valid] | Broken [list] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual file (git diff or file content). Verify 1:1 — everything in spec was built, nothing beyond spec was built. Check "Must NOT do" compliance. Detect scope creep. Verify Package Index count increased. Verify main doc line count within bounds.
  Output: `Tasks [N/N compliant] | Line Count [within/over budget] | Package Index [N entries] | VERDICT`

---

## Deferred Work (Tier 2 — Future Planning)

These items were identified during gap analysis but are explicitly out of scope:

- **OpenClaw platform extensions** (37 adapters: Discord, Telegram, Slack, WhatsApp, etc.) — needs separate investigation
- **OpenClaw non-browser skills** (31+ skills) — needs separate investigation
- **Infrastructure services** (Qdrant, Coturn, Squid, Nginx, Synapse, PostgreSQL) — ops documentation
- **Container build pipeline** (Dockerfile.openclaw, Dockerfile.openclaw-standalone, seccomp, apparmor)
- **Deployment guides** (41 existing guides in docs/guides/) — already covered
- **CI/CD documentation** (GitHub Actions workflows)
- **Scripts documentation** (scripts/, tools/, deploy/)
- **Setup Wizard** deep-dive (architecture decisions, component interaction)

---

## Commit Strategy

- **Wave 1**: `docs(armorclaw): fix Package Index, TOC, and structural issues` — `doc/armorclaw.md`
- **Wave 2**: `docs: add per-component documentation for voice, sidecar, secretary, agent-runtime, license, clients, comms` — all 7 new doc files (single commit after all are complete)

---

## Success Criteria

### Verification Commands
```bash
# Package Index grew
grep -c "| \`pkg/" doc/armorclaw.md  # Expected: >= 40

# TOC is complete
grep "^## " doc/armorclaw.md | wc -l  # Count headings
grep "^\d\+\. \[" doc/armorclaw.md | wc -l  # Count TOC entries — should match

# Main doc didn't explode
wc -l doc/armorclaw.md  # Expected: <= 3200

# All 7 new docs exist
ls -la doc/voice-stack.md doc/sidecar-pipeline.md doc/secretary-workflow.md doc/agent-runtime.md doc/license-system.md doc/client-applications.md doc/communication-infra.md

# No content duplication with sidecar README
grep -c "sidecar/README" doc/sidecar-pipeline.md  # Should reference it
```

### Final Checklist
- [ ] Package Index covers 40+ packages
- [ ] TOC matches all ## headings
- [ ] Malformed component tags fixed
- [ ] All 7 per-component docs exist with consistent template
- [ ] No content duplication with existing scattered docs
- [ ] Main doc within line budget (~3200)
