# ArmorClaw Secretary Features Implementation Plan

## TL;DR

> **Quick Summary**: Add Secretary features to ArmorClaw that enable task templates, multi-agent orchestration, enhanced PII management, and proactive notifications - all integrated with existing Playwright browser-service, SQLCipher keystore, and Matrix control plane.

**Deliverables**:
- Task template engine with scheduling
- Workflow orchestrator for multi-agent coordination
- Extended PII registry with approval policies
- Notification system with push/Matrix delivery

**Estimated Effort**: Large (8-10 weeks)
**Parallel Execution**: YES - 4 phases with parallel waves
**Critical Path**: Phase 1 (Templates) → Phase 2 (Orchestration) → Phase 4 (Integration)

---

## Context

### Original Request
Plan implementation of ArmorClaw Secretary features inside the existing architecture with specific constraints:
- Browser automation remains Playwright-based (no Rod)
- Runtime orchestration in Bridge + Matrix (not OMO)
- Reuse Agent Studio, PII registry, MCP approval, Matrix RPC patterns
- API keys stay environment-based (never in keystore)
- Add as bounded packages with migrations
- All custodial PII requires policy enforcement, audit logging, approval flow

### Interview Summary
**Key Discussions**:
- Constraints confirmed: Playwright-only, SQLCipher keystore, Matrix control plane
- Reuse patterns: BlindFill, HITL approval, Browser skills, Agent definitions
- Scope: Templates, orchestration, PII extensions, notifications
- Security: All PII through approval flow, audit logging mandatory

**Research Findings**:
- Browser Service: `/browser-service/src/` - TypeScript/Playwright with stealth mode
- Go Browser Client: `/bridge/pkg/browser/client.go` - HTTP client to browser-service
- PII System: `/bridge/pkg/pii/resolver.go` - BlindFill engine with approval
- Keystore: `/bridge/pkg/keystore/keystore.go` - SQLCipher, hardware-bound
- Agent Studio: `/bridge/pkg/studio/` - Definitions, skills, provisioning
- Matrix: `/bridge/internal/adapter/matrix.go` - Event handling, commands

### Metis Review
**Identified Gaps** (addressed in plan):
- Missing: Task template storage schema
- Missing: Workflow definition format
- Missing: Approval policy engine for auto-approve rules
- Missing: Notification delivery abstraction

---

## Work Objectives

### Core Objective
Implement Secretary features as bounded packages that integrate seamlessly with existing architecture, following all security patterns for custodial PII.

### Concrete Deliverables
- `bridge/pkg/secretary/` - Core secretary package
- `bridge/pkg/templates/` - Task template engine
- `bridge/pkg/workflows/` - Multi-agent orchestration
- `bridge/pkg/notifications/` - Notification delivery
- Database migrations for templates, workflows, policies
- Matrix command handlers for `!secretary` commands

### Definition of Done
- [ ] All phases implemented and tested
- [ ] All PII flows through approval system
- [ ] All actions logged to audit trail
- [ ] Matrix commands functional (`!secretary *`)
- [ ] No new security vulnerabilities introduced

### Must Have
- Task templates with scheduling
- Multi-agent workflow execution
- PII approval for all custodial data
- Audit logging for all operations
- Matrix command interface

### Must NOT Have (Guardrails)
- No Rod browser library (Playwright only)
- No API key persistence to keystore
- No direct secret access
- No bypass of approval flows
- No PII values in logs
- No network access without egress proxy (future)

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (Go test framework in bridge/)
- **Automated tests**: YES (TDD recommended)
- **Framework**: Go testing + testify
- **If TDD**: Each task follows RED-GREEN-REFACTOR

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Backend**: Use Bash (curl) - Send requests, assert status + response fields
- **Database**: Use Bash (sqlite3) - Query, verify schema/data
- **Go Code**: Use Bash (go test) - Run unit tests, verify pass

---

## Execution Strategy

### Parallel Execution Waves

```
Phase 1 - Foundation (Week 1-2, 8 parallel tasks):
├── T1: Database schema migrations
├── T2: Task template types & interfaces
├── T3: Template storage layer
├── T4: Cron scheduling engine
├── T5: PII policy types
├── T6: Notification types
├── T7: Workflow types & interfaces
└── T8: Matrix command router for secretary

Phase 2 - Core Features (Week 3-5, 10 parallel tasks):
├── T9: Template CRUD operations
├── T10: Template instantiation
├── T11: Scheduler execution
├── T12: PII policy engine
├── T13: Auto-approve rules
├── T14: Notification dispatcher
├── T15: Matrix notification delivery
├── T16: Workflow definition parser
├── T17: Workflow executor
└── T18: Agent-to-agent messaging

Phase 3 - Integration (Week 6-7, 6 parallel tasks):
├── T19: Browser skill integration
├── T20: BlindFill integration
├── T21: Agent Studio integration
├── T22: Audit logging integration
├── T23: Matrix room creation
└── T24: SDTW notification delivery

Phase 4 - Polish (Week 8, 4 parallel tasks):
├── T25: Error handling & recovery
├── T26: Documentation
├── T27: Integration tests
└── T28: Security review

Phase FINAL (After ALL tasks):
├── F1: Plan compliance audit
├── F2: Code quality review
├── F3: Manual QA
└── F4: Scope fidelity check
```

### Dependency Matrix

- **T1**: — — T3, T5, T6, T7
- **T2**: T1 — T9, T10
- **T3**: T1, T2 — T9, T10, T11
- **T4**: T1 — T11
- **T5**: T1 — T12, T13
- **T6**: T1 — T14, T15
- **T7**: T1 — T16, T17
- **T8**: — — T15, T23
- **T9-18**: Phase 1 complete — Phase 3
- **T19-24**: Phase 2 complete — Phase 4

---

### Phase 1: Foundation (Templates & Storage)

- [x] 1. **Secretary Types & Interfaces**
  
  **What to do**:
  - Create `bridge/pkg/secretary/types.go` with core types
  - Define TaskTemplate, Workflow, ApprovalPolicy types
  - Define NotificationChannel types
  - Create interfaces for template and workflow execution
  
  **Must NOT do**:
  - Modify existing types in studio or pii packages
  - Introduce new keystore encryption methods
  
  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: [] (standard Go patterns)
  
  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3, 4)
  - **Blocks**: Tasks 5-12
  - **Blocked By**: None
  
  **References**:
  - `bridge/pkg/studio/types.go:14-50` - Type definition patterns (AgentDefinition, Skill, PIIField)
  - `bridge/pkg/pii/skill_manifest.go:24-78` - VariableRequest pattern for PII
  - `bridge/pkg/audit/audit.go:14-32` - Audit event types
  
  **Acceptance Criteria**:
  - [ ] File created: `bridge/pkg/secretary/types.go`
  - [ ] All types compile without errors
  - [ ] Go vet passes on new package
  - [ ] Types compatible with existing studio types
  
  **QA Scenarios**:
  ```
  Scenario: Type compilation
    Tool: Bash (go build)
    Preconditions: Go toolchain installed
    Steps:
      1. cd /home/mink/src/armorclaw-omo/bridge
      2. go build ./pkg/secretary/...
    Expected Result: Build succeeds with no errors
    Failure Indicators: "undefined", "cannot find"
    Evidence: .sisyphus/evidence/task-1-types-compile.txt
  
  Scenario: Go vet pass
    Tool: Bash (go vet)
    Preconditions: Build successful
    Steps:
      1. cd /home/mink/src/armorclaw-omo/bridge
      2. go vet ./pkg/secretary/...
    Expected Result: All vet checks pass
    Failure Indicators: "shadowed", "unused"
    Evidence: .sisyphus/evidence/task-1-vet-pass.txt
  ```

### Phase 1: Foundation (Templates & Storage)

- [x] 2. **Secretary Database Schema**
  
  **What to do**:
  - Create `bridge/pkg/secretary/schema.sql`
  - Define tables: task_templates, workflows, approval_policies, notification_channels
  - Add indexes for efficient queries
  
  **Must NOT do**:
  - Modify existing studio schema
  - Change keystore table structure
  
  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []
  
  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3, 4)
  - **Blocks**: Task 6 (schema migration)
  - **Blocked By**: None
  
  **References**:
  - `bridge/pkg/studio/schema.sql` - Existing schema patterns
  - `bridge/pkg/keystore/keystore.go:357-416` - Schema initialization pattern
  - `bridge/pkg/audit/audit.go:122-155` - Audit table schema
  
  **Acceptance Criteria**:
  - [ ] File created: `bridge/pkg/secretary/schema.sql`
  - [ ] SQL syntax valid (sqlite3 check)
  - [ ] Tables: task_templates, workflows, workflow_steps, approval_policies, notification_channels
  
  **QA Scenarios**:
  ```
  Scenario: SQL validation
    Tool: Bash (sqlite3)
    Preconditions: schema.sql created
    Steps:
      1. sqlite3 :memory: < schema.sql
      2. .tables
    Expected Result: Tables created without errors
    Failure Indicators: "syntax error", "table already exists"
    Evidence: .sisyphus/evidence/task-2-schema-valid.txt
  ```

- [x] 3. **Secretary Store (SQLite)**
  
  **What to do**:
  - Create `bridge/pkg/secretary/store.go`
  - Implement CRUD operations for templates, workflows, policies
  - Add query methods for listing and filtering
  - Integrate with existing audit logging
  
  **Must NOT do**:
  - Duplicate store logic from studio package
  - Bypass audit logging
  
  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []
  
  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 4)
  - **Blocks**: Tasks 7-12
  - **Blocked By**: Task 2 (schema)
  
  **References**:
  - `bridge/pkg/studio/store.go:45-150` - Store implementation pattern
  - `bridge/pkg/keystore/keystore.go:596-640` - Database operations
  - `bridge/pkg/audit/audit.go:195-241` - Audit logging integration
  
  **Acceptance Criteria**:
  - [ ] File created: `bridge/pkg/secretary/store.go`
  - [ ] All CRUD methods implemented
  - [ ] Audit logging integrated
  - [ ] Unit tests pass
  
  **QA Scenarios**:
  ```
  Scenario: Store CRUD operations
    Tool: Bash (go test)
    Preconditions: store.go created
    Steps:
      1. cd /home/mink/src/armorclaw-omo/bridge
      2. go test ./pkg/secretary/store_test.go
    Expected Result: All tests pass (create, read, update, delete)
    Failure Indicators: "FAIL", "panic"
    Evidence: .sisyphus/evidence/task-3-store-crud.txt
  ```

- [~] 4. **Template Engine Core** ⚠️ PARTIAL - conditional branching is a STUB
  
  **What to do**:
  - Create `bridge/pkg/secretary/template_engine.go`
  - Implement template instantiation logic ✅
  - Add variable substitution engine ✅
  - Support conditional branching in templates ❌ STUB (evaluateConditions returns unchanged)
  - Add validation for required variables ✅
  
  **Must NOT do**:
  - Execute templates without validation
  - Allow arbitrary code execution
  
  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []
  
  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3)
  - **Blocks**: Tasks 8, 10
  - **Blocked By**: Tasks 1, 3 (types and store)
  
  **References**:
  - `bridge/pkg/pii/resolver.go:85-181` - Variable resolution pattern
  - `bridge/pkg/studio/browser_skill.go:365-439` - Form fill with variables
  - `bridge/pkg/browser/browser.go:146-202` - Fill command handling
  
  **Acceptance Criteria**:
  - [ ] File created: `bridge/pkg/secretary/template_engine.go`
  - [ ] Variable substitution works
  - [ ] Conditional branching works
  - [ ] Validation catches missing variables
  
  **QA Scenarios**:
  ```
  Scenario: Template instantiation
    Tool: Bash (go test)
    Preconditions: template_engine.go created
    Steps:
      1. go test -run TestInstantiateTemplate
    Expected Result: Template variables substituted correctly
    Failure Indicators: "variable not found", "validation failed"
    Evidence: .sisyphus/evidence/task-4-template-instantiate.txt
  
  Scenario: Conditional branching
    Tool: Bash (go test)
    Preconditions: template_engine.go created
    Steps:
      1. go test -run TestConditionalBranching
    Expected Result: Branches execute based on conditions
    Failure Indicators: "wrong branch", "condition not evaluated"
    Evidence: .sisyphus/evidence/task-4-conditional-branch.txt
  ```

---

### Phase 2: Core Features

- [x] 5. **Workflow Orchestrator** ✅ VERIFIED
  
  **What to do**:
  - Create `bridge/pkg/secretary/orchestrator.go` ✅
  - Implement multi-agent coordination ✅ (activeWorkflows map, goroutine per workflow)
  - Add state synchronization between agents ✅
  - Implement dependency tracking ✅ (Step.Order, currentIndex, validateTransition)
  - Add workflow status tracking ✅ (Status enum, state machine)
  
  **Must NOT do**:
  - Block on non-dependent tasks
  - Allow circular dependencies
  
  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: [`systematic-debugging`]
  
  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Phase 1)
  - **Parallel Group**: Wave 2 (with Tasks 6, 7)
  - **Blocks**: Tasks 9, 10
  - **Blocked By**: Tasks 1, 3, 4 (types, store, template engine)
  
  **References**:
  - `bridge/pkg/studio/factory.go:200-280` - Agent spawning pattern
  - `bridge/pkg/agent/state_machine.go` - State machine for tracking
  - `bridge/internal/executor/` - Execution patterns
  
  **Acceptance Criteria**:
  - [ ] File created: `bridge/pkg/secretary/orchestrator.go`
  - [ ] Multi-agent coordination works
  - [ ] Dependency tracking prevents circular deps
  - [ ] Status tracking emits events
  
  **QA Scenarios**:
  ```
  Scenario: Multi-agent workflow
    Tool: Bash (go test)
    Preconditions: orchestrator.go created
    Steps:
      1. go test -run TestMultiAgentWorkflow
    Expected Result: Agents execute in correct order
    Failure Indicators: "deadlock", "wrong order"
    Evidence: .sisyphus/evidence/task-5-multi-agent.txt
  ```

- [~] 6. **Scheduling Engine** ⚠️ PARTIAL - no timezone handling
  
  **What to do**:
  - Create `bridge/pkg/secretary/scheduler.go` ✅ (orchestrator_scheduler.go)
  - Implement cron-like scheduling ⚠️ (time.Ticker, not cron expressions)
  - Add recurring task support ✅ (ScheduleRecurring type)
  - Implement timezone handling ❌ (uses time.Now() only)
  - Add task execution tracking ✅ (ScheduledJob struct, events)
  
  **Must NOT do**:
  - Execute scheduled tasks if system is in lockdown mode
  - Miss schedules due to timezone issues
  
  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []
  
  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 7)
  - **Blocks**: Tasks 11, 12
  - **Blocked By**: Task 3 (store)
  
  **References**:
  - `bridge/pkg/audit/audit.go` - Audit logging for task execution
  - `bridge/pkg/studio/types.go:150-180` - Instance status tracking
  - `bridge/pkg/provisioning/manager.go` - Token expiration patterns
  
  **Acceptance Criteria**:
  - [ ] File created: `bridge/pkg/secretary/scheduler.go`
  - [ ] Cron expressions parsed correctly
  - [ ] Tasks execute at scheduled times
  - [ ] Timezone handling works
  
  **QA Scenarios**:
  ```
  Scenario: Scheduled task execution
    Tool: Bash (go test)
    Preconditions: scheduler.go created
    Steps:
      1. go test -run TestScheduledTask
      2. Wait for scheduled time
    Expected Result: Task executes at scheduled time
    Failure Indicators: "task not executed", "wrong time"
    Evidence: .sisyphus/evidence/task-6-scheduled.txt
  ```

- [x] 7. **Approval Policy Engine** ✅ VERIFIED
  
  **What to do**:
  - Create `bridge/pkg/secretary/approval_policy.go` ✅ (approvals.go)
  - Implement policy evaluation logic ✅ (Evaluate, evaluatePolicies, evaluateSinglePolicy)
  - Add auto-approve rules support ✅ (policy.AutoApprove, conditions)
  - Add delegation rules ✅ (policy.DelegateTo field)
  - Integrate with existing PII approval flow ✅ (PIIFields in EvaluationContext)
  
  **Must NOT do**:
  - Auto-approve critical PII without user override
  - Bypass audit logging for approvals
  
  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []
  
  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 6)
  - **Blocks**: Tasks 11, 12
  - **Blocked By**: Task 3 (store)
  
  **References**:
  - `bridge/pkg/pii/hitl_consent.go:34-99` - HITL approval flow
  - `bridge/pkg/studio/mcp_approval.go:88-175` - MCP approval patterns
  - `bridge/pkg/keystore/sealed_keystore.go:28-39` - Unseal policies
  
  **Acceptance Criteria**:
  - [ ] File created: `bridge/pkg/secretary/approval_policy.go`
  - [ ] Auto-approve rules work for low-sensitivity PII
  - [ ] Delegation rules work
  - [ ] Audit logging for all policy decisions
  
  **QA Scenarios**:
  ```
  Scenario: Auto-approve low-sensitivity
    Tool: Bash (go test)
    Preconditions: approval_policy.go created
    Steps:
      1. Create request with low-sensitivity PII
      2. Evaluate policy
    Expected Result: Auto-approved without user interaction
    Failure Indicators: "requires approval", "policy denied"
    Evidence: .sisyphus/evidence/task-7-auto-approve.txt
  ```

---

### Phase 3: Integration

- [x] 8. **Matrix Command Integration** ✅ VERIFIED
  
  **What to do**:
  - Create `bridge/pkg/secretary/commands.go` ✅ (secretary_commands.go)
  - Implement `!secretary` command handler ✅ (HandleMessage with full routing)
  - Add template management commands ✅ (list templates)
  - Add workflow commands ✅ (create, start, list, status, cancel)
  - Integrate with MatrixAdapter ✅
  
  **Must NOT do**:
  - Duplicate command patterns from studio package
  - Allow unauthenticated command execution
  
  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []
  
  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Phase 2)
  - **Parallel Group**: Wave 3 (with Tasks 9, 10)
  - **Blocks**: Tasks 11, 12
  - **Blocked By**: Tasks 4, 5, 6 (template engine, orchestrator, scheduler)
  
  **References**:
  - `bridge/pkg/studio/commands.go:100-300` - Command handler pattern
  - `bridge/pkg/matrixcmd/handler.go` - Matrix command routing
  - `bridge/internal/adapter/matrix.go` - MatrixAdapter interface
  
  **Acceptance Criteria**:
  - [ ] File created: `bridge/pkg/secretary/commands.go`
  - [ ] `!secretary` commands work
  - [ ] Template commands work
  - [ ] Workflow commands work
  
  **QA Scenarios**:
  ```
  Scenario: Secretary help command
    Tool: Bash (go test)
    Preconditions: commands.go created
    Steps:
      1. Send "!secretary help"
    Expected Result: Help text displayed with all commands
    Failure Indicators: "unknown command", "no response"
    Evidence: .sisyphus/evidence/task-8-help-command.txt
  ```

- [~] 9. **RPC Methods for Secretary** ⚠️ PARTIAL - missing template methods
  
  **What to do**:
  - Create `bridge/pkg/secretary/rpc.go` ✅
  - Implement JSON-RPC methods:
    - `secretary.create_template` ❌ (not implemented)
    - `secretary.list_templates` ❌ (not implemented)
    - `secretary.spawn_workflow` ⚠️ (start_workflow exists)
    - `secretary.list_workflows` ✅
  - Integrate with existing RPC server ✅
  
  **Must NOT do**:
  - Duplicate RPC patterns from studio package
  - Allow unauthorized RPC calls
  
  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []
  
  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 8, 10)
  - **Blocks**: Tasks 11, 12
  - **Blocked By**: Tasks 3, 5 (store, orchestrator)
  
  **References**:
  - `bridge/pkg/studio/rpc.go` - RPC handler pattern
  - `bridge/pkg/rpc/browser.go` - RPC method signatures
  - `bridge/pkg/rpc/pii.go` - PII request RPC
  
  **Acceptance Criteria**:
  - [ ] File created: `bridge/pkg/secretary/rpc.go`
  - [ ] All RPC methods implemented
  - [ ] RPC methods registered with server
  - [ ] Unit tests pass
  
  **QA Scenarios**:
  ```
  Scenario: RPC template creation
    Tool: Bash (curl)
    Preconditions: RPC server running
    Steps:
      1. curl -X POST -d '{"method":"secretary.create_template","params":{...}}' http://localhost:8443/rpc
    Expected Result: Template created with ID returned
    Failure Indicators: "method not found", "invalid params"
    Evidence: .sisyphus/evidence/task-9-rpc-template.txt
  ```

- [x] 10. **Notification System** ✅ VERIFIED
  
  **What to do**:
  - Create `bridge/pkg/secretary/notifications.go` ✅ (577 lines)
  - Implement notification delivery ✅ (Dispatch method, subscriber pattern)
  - Add Matrix notification support ✅ (MatrixNotificationAdapter)
  - Add push notification integration (via existing push package) ⚠️ (adapter pattern ready)
  - Implement notification scheduling ✅ (workflow/approval helpers)
  
  **Must NOT do**:
  - Send notifications during lockdown mode
  - Bypass notification preferences
  
  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []
  
  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 8, 9)
  - **Blocks**: Task 12
  - **Blocked By**: Task 3 (store)
  
  **References**:
  - `bridge/pkg/push/sygnal.go` - Push notification patterns
  - `bridge/pkg/matrix/client.go` - Matrix messaging
  - `bridge/pkg/eventbus/events.go` - Event publishing
  
  **Acceptance Criteria**:
  - [ ] File created: `bridge/pkg/secretary/notifications.go`
  - [ ] Matrix notifications work
  - [ ] Push notifications work
  - [ ] Scheduling works
  
  **QA Scenarios**:
  ```
  Scenario: Matrix notification
    Tool: Bash (go test)
    Preconditions: notifications.go created
    Steps:
      1. Create notification request
      2. Send to Matrix room
    Expected Result: Message appears in Matrix room
    Failure Indicators: "send failed", "room not found"
    Evidence: .sisyphus/evidence/task-10-matrix-notification.txt
  ```

---

### Phase 4: Polish & Documentation

- [x] 11. **Error Handling & Recovery** ✅ VERIFIED
  
  **What to do**:
  - Add comprehensive error handling to all secretary packages ✅
  - Implement recovery mechanisms for failed workflows ✅
  - Add retry logic for transient failures ✅
  - Implement graceful degradation ✅
  
  **Must NOT do**:
  - Panic on errors
  - Lose workflow state on failure
  
  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: [`systematic-debugging`]
  
  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Task 12)
  - **Blocks**: Final verification
  - **Blocked By**: Tasks 5-10 (all core features)
  
  **References**:
  - `bridge/pkg/studio/factory.go:350-400` - Error handling patterns
  - `bridge/pkg/keystore/keystore.go:790-819` - Retry logic
  - `bridge/internal/ai/retry.go` - Retry patterns
  
  **Acceptance Criteria**:
  - [ ] All error cases handled
  - [ ] Recovery mechanisms work
  - [ ] Retry logic handles transient failures
  - [ ] Graceful degradation works
  
  **QA Scenarios**:
  ```
  Scenario: Workflow failure recovery
    Tool: Bash (go test)
    Preconditions: Error handling implemented
    Steps:
      1. Start workflow
      2. Simulate failure
      3. Verify recovery
    Expected Result: Workflow recovers or fails gracefully
    Failure Indicators: "panic", "state lost"
    Evidence: .sisyphus/evidence/task-11-recovery.txt
  ```

- [x] 12. **Integration Tests** ✅ VERIFIED
  
  **What to do**:
  - Create `bridge/pkg/secretary/integration_test.go` ✅
  - Add end-to-end tests for:
    - Template creation → instantiation → execution ✅ (template_engine_test.go)
    - Workflow creation → agent spawning → completion ✅ (orchestrator_test.go, orchestrator_integration.go)
    - Approval request → user approval → PII injection ✅ (audit_test.go, blindfill_integration_test.go)
  - Add Matrix command integration tests ✅ (secretary_commands_test.go)
  
  **Must NOT do**:
  - Skip integration tests for critical paths
  
  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: [`verification-before-completion`]
  
  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Task 11)
  - **Blocks**: Final verification
  - **Blocked By**: Tasks 5-10 (all core features)
  
  **References**:
  - `bridge/pkg/studio/studio_test.go` - Integration test patterns
  - `bridge/pkg/pii/blindfill_e2e_test.go` - E2E test patterns
  - `bridge/pkg/agent/e2e_test.go` - Agent E2E tests
  
  **Acceptance Criteria**:
  - [ ] File created: `bridge/pkg/secretary/integration_test.go`
  - [ ] All critical paths tested
  - [ ] Tests pass
  - [ ] Coverage > 80%
  
  **QA Scenarios**:
  ```
  Scenario: Full workflow integration
    Tool: Bash (go test)
    Preconditions: All features implemented
    Steps:
      1. go test -v ./pkg/secretary/... -run Integration
    Expected Result: All integration tests pass
    Failure Indicators: "FAIL", "timeout"
    Evidence: .sisyphus/evidence/task-12-integration.txt
  ```

---

## Final Verification Wave

> 4 review agents run in PARALLEL. ALL must APPROVE.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read plan end-to-end. Verify each "Must Have" implemented, each "Must NOT Have" absent. Check evidence files exist. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `go build` + `go vet` + `go test`. Review for: `as any`/`//nolint`, empty catches, console.log in prod, commented-out code, unused imports.
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Security & PII Review** — `unspecified-high`
  Verify PII handling: BlindFill integration, approval flows, audit logging. Check no PII values in logs. Verify environment-based API keys only.
  Output: `PII Handling [PASS/FAIL] | Approval Flows [PASS/FAIL] | Audit Logging [PASS/FAIL] | VERDICT`

- [ ] F4. **Architecture Fidelity Check** — `deep`
  Verify Secretary features follow existing patterns: Playwright-only browser automation, Matrix control plane, SQLCipher keystore, Agent Studio integration. Check no new encryption methods, no Rod library, no network bypass.
  Output: `Patterns [N/N compliant] | Constraints [CLEAN/N violations] | VERDICT`

---

## Commit Strategy

Commits by phase:
- **Phase 1**: `feat(secretary): add foundation - schema, types, store, template engine`
- **Phase 2**: `feat(secretary): add core features - orchestrator, scheduler, approval policies`
- **Phase 3**: `feat(secretary): add integration - Matrix commands, RPC, notifications`
- **Phase 4**: `feat(secretary): polish - error handling, integration tests, docs`

---

## Success Criteria

### Verification Commands
```bash
go build ./...                              # Expected: build succeeds
go test ./pkg/secretary/...                 # Expected: all tests pass
go vet ./pkg/secretary/...                  # Expected: no issues
sqlite3 /var/lib/armorclaw/bridge.db ".tables"  # Expected: includes secretary tables
```

### Final Checklist
- [ ] All "Must Have" present (Playwright, SQLCipher, Matrix, BlindFill, approval flows, audit logging)
- [ ] All "Must NOT Have" absent (Rod library, persisted API keys, network bypass, disk PII)
- [ ] All tests pass
- [ ] PII approval flows work
- [ ] Matrix commands functional
- [ ] Evidence files in .sisyphus/evidence/


> 4 review agents run in PARALLEL. ALL must APPROVE.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Verify all "Must Have" implemented, all "Must NOT Have" absent. Check evidence files.

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `go test ./...` + `go vet`. Review for `as any`, empty catches, console.log.

- [ ] F3. **Manual QA** — `unspecified-high`
  Execute all QA scenarios end-to-end. Test cross-feature integration.

- [ ] F4. **Scope Fidelity Check** — `deep`
  Verify 1:1 mapping between spec and implementation. No scope creep.

---

## Commit Strategy

Commits by phase:
- Phase 1: `feat(secretary): add foundation - schema, types, interfaces`
- Phase 2: `feat(secretary): add core features - templates, workflows, notifications`
- Phase 3: `feat(secretary): integrate with existing systems`
- Phase 4: `feat(secretary): polish - error handling, docs, tests`

---

## Success Criteria

### Verification Commands
```bash
# All tests pass
go test ./bridge/pkg/secretary/... -v
go test ./bridge/pkg/templates/... -v
go test ./bridge/pkg/workflows/... -v
go test ./bridge/pkg/notifications/... -v

# Database migrations applied
sqlite3 /var/lib/armorclaw/bridge.db ".tables" | grep -E "templates|workflows|policies"

# Matrix commands work
# In Matrix room: !secretary help
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass
- [ ] PII approval flows work
- [ ] Audit logging complete
- [ ] Matrix commands functional
