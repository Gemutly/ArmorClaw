# Secretary Gap Fix Plan - Complete to 100%

## TL;DR

> **Quick Summary**: Fix 3 partial implementations to complete Secretary features at 100% - conditional branching, timezone support, and template RPC methods. All changes are Go-only, no Python dependency.
> 
> **Deliverables**:
> - Conditional branching in template engine
> - Timezone-aware job scheduling
> - Template CRUD RPC methods
>
> **Estimated Effort**: Small (2-4 hours total)
> **Parallel Execution**: YES - all 3 gaps are independent
> **Critical Path**: None - all can run in parallel

---

## Context

### Original Request
Complete the Secretary implementation to 100% by fixing 3 partial implementations identified during plan verification.

### Gap Analysis Summary

| Gap | Issue | Impact | Effort |
|-----|-------|--------|--------|
| **G1: Conditional Branching** | `evaluateConditions()` is a stub | Workflows execute all steps regardless of conditions | ~1 hour |
| **G2: Timezone Handling** | Scheduler uses `time.Now()` only | Jobs run at server time, not user timezone | ~1 hour |
| **G3: Template RPC Methods** | Missing `create_template`, `list_templates` | Templates only manageable via Matrix commands | ~1 hour |

### Architecture Principle
**Go-only implementation** - The existing Go Secretary is fully capable. No Python/OMO layer needed.

---

## Work Objectives

### Core Objective
Complete the 3 partial implementations to achieve 100% Secretary feature parity.

### Concrete Deliverables
- Working conditional step evaluation in template engine
- Timezone-aware job scheduling
- Full template CRUD via RPC API

### Definition of Done
- [ ] All 3 gaps resolved
- [ ] Go tests pass for each gap
- [ ] No regressions in existing functionality
- [ ] Secretary features work without Python dependency

### Must Have
- Conditional branching with condition evaluation
- Timezone support in scheduler
- Template RPC methods (create, list, get, update, delete)

### Must NOT Have (Guardrails)
- No Python/OMO layer introduction
- No breaking changes to existing APIs
- No new external dependencies (use stdlib)

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (Go test framework)
- **Automated tests**: YES (TDD recommended)
- **Framework**: Go testing + testify

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/gap-{N}-{scenario-slug}.{ext}`.

---

## Execution Strategy

### Parallel Execution

```
All 3 gaps are INDEPENDENT - run in parallel:

Gap 1 (Conditional Branching):
├── G1.1: Define Condition types in types.go [quick]
├── G1.2: Implement evaluateConditions() [unspecified-high]
└── G1.3: Add unit tests [quick]

Gap 2 (Timezone Handling):
├── G2.1: Add Timezone field to SchedulerConfig [quick]
├── G2.2: Load timezone in Start() [quick]
├── G2.3: Use timezone in processScheduledJobs() [quick]
└── G2.4: Add unit tests [quick]

Gap 3 (Template RPC):
├── G3.1: Add handleCreateTemplate() [quick]
├── G3.2: Add handleGetTemplate() [quick]
├── G3.3: Add handleListTemplates() [quick]
├── G3.4: Add handleUpdateTemplate() [quick]
├── G3.5: Add handleDeleteTemplate() [quick]
└── G3.6: Register methods in Handle() [quick]
```

---

## Task Dependency Graph (MANDATORY)

| Task ID | Task Name | Blocked By | Blocks | Parallel Group |
|---------|-----------|------------|--------|----------------|
| G1.1 | Define Condition Types | — | G1.2 | Wave 1 |
| G1.2 | Implement evaluateConditions() | G1.1 | G1.3 | Wave 2 |
| G1.3 | Add unit tests (G1) | G1.2 | — | Wave 3 |
| G2.1 | Add Timezone Config Fields | — | G2.2 | Wave 1 |
| G2.2 | Load Timezone in Start() | G2.1 | G2.3 | Wave 2 |
| G2.3 | Use Timezone in processScheduledJobs() | G2.2 | G2.4 | Wave 3 |
| G2.4 | Add unit tests (G2) | G2.3 | — | Wave 4 |
| G3.1 | handleCreateTemplate() | — | G3.6 | Wave 1 |
| G3.2 | handleGetTemplate() | — | G3.6 | Wave 1 |
| G3.3 | handleListTemplates() | — | G3.6 | Wave 1 |
| G3.4 | handleUpdateTemplate() | — | G3.6 | Wave 1 |
| G3.5 | handleDeleteTemplate() | — | G3.6 | Wave 1 |
| G3.6 | Register Methods in Handle() | G3.1-G3.5 | — | Wave 2 |
| F1 | Gap Compliance Check | G1.3, G2.4, G3.6 | — | Final |
| F2 | Build & Test | F1 | — | Final |
| F3 | Regression Check | F2 | — | Final |
| F4 | Python-Free Verification | F3 | — | Final |

---

## Parallel Execution Graph (MANDATORY)

```
Wave 1 (Start Immediately — 6 parallel tasks):
├── G1.1: Define Condition Types [quick]
├── G2.1: Add Timezone Config Fields [quick]
├── G3.1: handleCreateTemplate() [quick]
├── G3.2: handleGetTemplate() [quick]
├── G3.3: handleListTemplates() [quick]
└── G3.4: handleUpdateTemplate() [quick]

Wave 2 (After Wave 1 — 3 parallel tasks):
├── G1.2: Implement evaluateConditions() [unspecified-high]
├── G2.2: Load Timezone in Start() [quick]
├── G3.5: handleDeleteTemplate() [quick] (started in Wave 1, continues)

Wave 3 (After Wave 2 — 4 parallel tasks):
├── G1.3: Add unit tests (G1) [quick]
├── G2.3: Use Timezone in processScheduledJobs() [quick]
├── G3.6: Register Methods in Handle() [quick]
└── G2.4: Add unit tests (G2) [quick] (can overlap with G2.3)

Wave FINAL (After ALL tasks — 4 parallel verification tasks):
├── F1: Gap Compliance Check [quick]
├── F2: Build & Test [quick]
├── F3: Regression Check [unspecified-high]
└── F4: Python-Free Verification [quick]

Critical Path: G1.1 → G1.2 → G1.3 → F1-F4 (shortest: 3 steps)
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 6 (Wave 1)
```

---

## Category + Skills Recommendations (MANDATORY)

| Task ID | Category | Skills | Rationale |
|---------|----------|--------|-----------|
| G1.1 | quick | [] | Simple type definitions, follow existing patterns |
| G1.2 | unspecified-high | [] | Logic implementation, reuse approval engine patterns |
| G1.3 | quick | [verification-before-completion] | Test writing, ensure coverage |
| G2.1 | quick | [] | Simple struct field additions |
| G2.2 | quick | [] | Follow existing time.LoadLocation pattern |
| G2.3 | quick | [] | Simple time conversion using .In() |
| G2.4 | quick | [verification-before-completion] | Test writing |
| G3.1 | quick | [] | Follow existing RPC handler patterns |
| G3.2 | quick | [] | Simple getter, standard pattern |
| G3.3 | quick | [] | Simple list, standard pattern |
| G3.4 | quick | [] | Standard update pattern |
| G3.5 | quick | [] | Standard delete pattern |
| G3.6 | quick | [] | Simple switch case additions |
| F1 | quick | [] | File existence checks |
| F2 | quick | [verification-before-completion] | Build and test execution |
| F3 | unspecified-high | [] | Requires understanding existing features |
| F4 | quick | [] | Simple file search |

**Skills Evaluated but Omitted:**
- `systematic-debugging`: Not needed - straightforward implementation tasks
- `brainstorming`: Not needed - requirements are clear
- `writing-plans`: Not needed - this IS the plan
- `git-master`: Not needed - simple commits at end

---

## TODOs

---

### Gap 1: Conditional Branching in Template Engine

- [ ] G1.1. **Define Condition Types**

  **What to do**:
  - Add `Condition` type to `types.go` (or reuse from approvals.go)
  - Add `Conditions` field to `WorkflowStep` if not present
  - Define operators: eq, neq, in, nin, contains, gt, lt, gte, lte

  **Must NOT do**:
  - Duplicate Condition type if already exists in approvals.go
  - Break existing WorkflowStep JSON marshalling

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Gap 1 Foundation
  - **Blocks**: G1.2
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/secretary/approvals.go:283-287` - Existing Condition struct
  - `bridge/pkg/secretary/approvals.go:289-361` - evaluateCondition pattern
  - `bridge/pkg/secretary/types.go:63-87` - WorkflowStep definition

  **Acceptance Criteria**:
  - [ ] Condition type defined with Field, Operator, Value
  - [ ] WorkflowStep has Conditions field (json.RawMessage)
  - [ ] Types compile without errors

  **QA Scenarios**:
  ```
  Scenario: Type compilation
    Tool: Bash (go build)
    Steps:
      1. cd /home/mink/src/armorclaw-omo/bridge
      2. go build ./pkg/secretary/...
    Expected Result: Build succeeds
    Evidence: .sisyphus/evidence/gap1-types-compile.txt
  ```

---

- [ ] G1.2. **Implement evaluateConditions()**

  **What to do**:
  - Replace stub in `template_engine.go:301-307`
  - Implement condition evaluation logic
  - Filter steps based on condition results
  - Support skip-if-false pattern

  **Current Stub**:
  ```go
  func (e *TemplateEngine) evaluateConditions(
      ctx context.Context,
      steps []WorkflowStep,
      variables map[string]string,
  ) ([]WorkflowStep, error) {
      return steps, nil  // ← Just returns all steps unchanged
  }
  ```

  **Expected Implementation**:
  ```go
  func (e *TemplateEngine) evaluateConditions(
      ctx context.Context,
      steps []WorkflowStep,
      variables map[string]string,
  ) ([]WorkflowStep, error) {
      var result []WorkflowStep
      
      for _, step := range steps {
          // Skip condition steps - they control flow, not execute
          if step.Type == StepCondition {
              // Evaluate and potentially skip next steps
              if !e.shouldExecute(step, variables) {
                  continue  // Skip this branch
              }
              continue
          }
          
          // Check step-level conditions
          if len(step.Conditions) > 0 {
              if !e.evaluateStepConditions(step.Conditions, variables) {
                  continue  // Skip this step
              }
          }
          
          result = append(result, step)
      }
      
      return result, nil
  }
  ```

  **Must NOT do**:
  - Remove steps from result that should execute
  - Break existing template instantiation

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on G1.1)
  - **Parallel Group**: Gap 1 Core
  - **Blocks**: G1.3
  - **Blocked By**: G1.1

  **References**:
  - `bridge/pkg/secretary/approvals.go:263-281` - evaluateConditions pattern
  - `bridge/pkg/secretary/approvals.go:289-361` - evaluateCondition with operators
  - `bridge/pkg/secretary/types.go:92-98` - StepType constants

  **Acceptance Criteria**:
  - [ ] evaluateConditions() evaluates step conditions
  - [ ] Steps with false conditions are skipped
  - [ ] StepCondition type steps control flow
  - [ ] All operators work (eq, neq, in, nin, contains)

  **QA Scenarios**:
  ```
  Scenario: Condition evaluation - skip step
    Tool: Bash (go test)
    Steps:
      1. cd /home/mink/src/armorclaw-omo/bridge
      2. go test -run TestEvaluateConditions ./pkg/secretary/...
    Expected Result: Steps with false conditions are filtered
    Evidence: .sisyphus/evidence/gap1-condition-skip.txt

  Scenario: Condition evaluation - include step
    Tool: Bash (go test)
    Steps:
      1. Create template with condition that evaluates to true
      2. Instantiate template
    Expected Result: Step included in result
    Evidence: .sisyphus/evidence/gap1-condition-include.txt
  ```

---

- [ ] G1.3. **Add Unit Tests for Conditional Branching**

  **What to do**:
  - Add tests to `template_engine_test.go`
  - Test all operators
  - Test skip/include scenarios
  - Test nested conditions

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: [`verification-before-completion`]

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on G1.2)
  - **Parallel Group**: Gap 1 Tests
  - **Blocks**: None
  - **Blocked By**: G1.2

  **Acceptance Criteria**:
  - [ ] Tests cover all operators
  - [ ] Tests pass
  - [ ] Coverage > 80% for evaluateConditions

  **QA Scenarios**:
  ```
  Scenario: All tests pass
    Tool: Bash (go test)
    Steps:
      1. cd /home/mink/src/armorclaw-omo/bridge
      2. go test -v ./pkg/secretary/... -run TestCondition
    Expected Result: All tests pass
    Evidence: .sisyphus/evidence/gap1-tests-pass.txt
  ```

---

### Gap 2: Timezone Handling in Scheduler

- [ ] G2.1. **Add Timezone Configuration**

  **What to do**:
  - Add `Timezone string` to `SchedulerConfig` (line 86-92)
  - Add `location *time.Location` to `Scheduler` struct (line 72-84)
  - Default to UTC if not specified

  **Changes**:
  ```go
  type SchedulerConfig struct {
      Store        Store
      Orchestrator *WorkflowOrchestratorImpl
      EventEmitter EventEmitter
      TickInterval time.Duration
      Logger       *logger.Logger
      Timezone     string  // ← ADD: IANA timezone (e.g., "America/New_York")
  }
  
  type Scheduler struct {
      // ... existing fields ...
      location     *time.Location  // ← ADD: parsed timezone
      timezone     string          // ← ADD: configured timezone string
  }
  ```

  **Must NOT do**:
  - Require timezone (must be optional)
  - Break existing scheduler creation

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Gap 2 Foundation
  - **Blocks**: G2.2, G2.3
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/secretary/orchestrator_scheduler.go:72-92` - Scheduler and Config structs
  - `bridge/pkg/secretary/trusted_workflows.go:606-614` - Existing timezone pattern

  **Acceptance Criteria**:
  - [ ] Timezone field added to SchedulerConfig
  - [ ] location field added to Scheduler
  - [ ] Code compiles

  **QA Scenarios**:
  ```
  Scenario: Config compilation
    Tool: Bash (go build)
    Steps:
      1. cd /home/mink/src/armorclaw-omo/bridge
      2. go build ./pkg/secretary/...
    Expected Result: Build succeeds
    Evidence: .sisyphus/evidence/gap2-config-compile.txt
  ```

---

- [ ] G2.2. **Load Timezone in Start()**

  **What to do**:
  - Modify `Start()` method (lines 129-143)
  - Load timezone using `time.LoadLocation()`
  - Store parsed location on scheduler
  - Fallback to UTC on error

  **Implementation**:
  ```go
  func (s *Scheduler) Start() {
      s.mu.Lock()
      defer s.mu.Unlock()
      
      if s.running {
          return
      }
      
      // Load timezone
      var loc *time.Location
      if s.timezone != "" {
          var err error
          loc, err = time.LoadLocation(s.timezone)
          if err != nil {
              s.log.Warn("timezone_load_failed", "timezone", s.timezone, "error", err)
              loc = time.UTC
          }
      } else {
          loc = time.UTC
      }
      s.location = loc
      
      s.ticker = time.NewTicker(s.tickInterval)
      s.running = true
      
      go s.run()
      
      s.log.Info("scheduler_started", "tick_interval", s.tickInterval, "timezone", s.timezone)
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on G2.1)
  - **Parallel Group**: Gap 2 Core
  - **Blocks**: G2.3
  - **Blocked By**: G2.1

  **References**:
  - `bridge/pkg/secretary/orchestrator_scheduler.go:129-143` - Start() method
  - `bridge/pkg/secretary/trusted_workflows.go:609-614` - time.LoadLocation pattern

  **Acceptance Criteria**:
  - [ ] Timezone loaded on Start()
  - [ ] Falls back to UTC on invalid timezone
  - [ ] Log message includes timezone

  **QA Scenarios**:
  ```
  Scenario: Valid timezone loads
    Tool: Bash (go test)
    Steps:
      1. Create scheduler with timezone="America/New_York"
      2. Start scheduler
    Expected Result: location set to America/New_York
    Evidence: .sisyphus/evidence/gap2-valid-timezone.txt

  Scenario: Invalid timezone falls back to UTC
    Tool: Bash (go test)
    Steps:
      1. Create scheduler with timezone="Invalid/Timezone"
      2. Start scheduler
    Expected Result: location set to UTC, warning logged
    Evidence: .sisyphus/evidence/gap2-invalid-timezone.txt
  ```

---

- [ ] G2.3. **Use Timezone in processScheduledJobs()**

  **What to do**:
  - Modify `processScheduledJobs()` (lines 168-199)
  - Convert `now` to configured timezone
  - Use timezone-aware comparison

  **Implementation**:
  ```go
  func (s *Scheduler) processScheduledJobs() {
      s.mu.Lock()
      defer s.mu.Unlock()
      
      now := time.Now()
      if s.location != nil {
          now = now.In(s.location)  // ← Convert to configured timezone
      }
      
      for id, job := range s.scheduledJobs {
          // ... existing logic ...
      }
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on G2.2)
  - **Parallel Group**: Gap 2 Core
  - **Blocks**: G2.4
  - **Blocked By**: G2.2

  **References**:
  - `bridge/pkg/secretary/orchestrator_scheduler.go:168-199` - processScheduledJobs()
  - `bridge/pkg/secretary/trusted_workflows.go:613` - now.In(loc) pattern

  **Acceptance Criteria**:
  - [ ] now converted to configured timezone
  - [ ] Jobs execute at correct local time
  - [ ] UTC jobs still work (backwards compatible)

  **QA Scenarios**:
  ```
  Scenario: Timezone-aware job execution
    Tool: Bash (go test)
    Steps:
      1. Create scheduler with timezone="America/New_York"
      2. Schedule job for 14:00 NY time
      3. Advance time to 14:00 NY (19:00 UTC)
    Expected Result: Job executes at 14:00 NY time
    Evidence: .sisyphus/evidence/gap2-timezone-exec.txt
  ```

---

- [ ] G2.4. **Add Unit Tests for Timezone**

  **What to do**:
  - Add tests to scheduler test file
  - Test valid/invalid timezones
  - Test job execution timing

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: [`verification-before-completion`]

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on G2.3)
  - **Parallel Group**: Gap 2 Tests
  - **Blocks**: None
  - **Blocked By**: G2.3

  **Acceptance Criteria**:
  - [ ] Tests cover timezone scenarios
  - [ ] Tests pass

---

### Gap 3: Template RPC Methods

- [ ] G3.1. **Add handleCreateTemplate()**

  **What to do**:
  - Add `CreateTemplateParams` struct
  - Implement `handleCreateTemplate()` RPC handler
  - Call store.CreateTemplate()

  **Implementation**:
  ```go
  type CreateTemplateParams struct {
      Name        string                 `json:"name"`
      Description string                 `json:"description,omitempty"`
      Steps       []WorkflowStep         `json:"steps"`
      Variables   json.RawMessage        `json:"variables,omitempty"`
      PIIRefs     []string               `json:"pii_refs,omitempty"`
      CreatedBy   string                 `json:"created_by"`
  }
  
  func (h *RPCHandler) handleCreateTemplate(req *RPCRequest) *RPCResponse {
      var params CreateTemplateParams
      if err := json.Unmarshal(req.Params, &params); err != nil {
          return ErrorResponse(ErrInvalidParams, "Invalid params: "+err.Error())
      }
      
      template := &TaskTemplate{
          ID:          generateTemplateID(),
          Name:        params.Name,
          Description: params.Description,
          Steps:       params.Steps,
          Variables:   params.Variables,
          PIIRefs:     params.PIIRefs,
          CreatedBy:   params.CreatedBy,
          CreatedAt:   time.Now().UnixMilli(),
          IsActive:    true,
      }
      
      if err := h.store.CreateTemplate(context.Background(), template); err != nil {
          return ErrorResponse(ErrInternal, "Failed to create template: "+err.Error())
      }
      
      h.log.Info("template_created_via_rpc", "template_id", template.ID, "by", req.UserID)
      
      return SuccessResponse(template)
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Gap 3 RPC Methods
  - **Blocks**: G3.6
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/secretary/rpc.go:127-154` - handleStartTemplate pattern
  - `bridge/pkg/secretary/store.go:222-259` - CreateTemplate signature

  **Acceptance Criteria**:
  - [ ] Method handler created
  - [ ] Validates required fields
  - [ ] Returns created template with ID

---

- [ ] G3.2. **Add handleGetTemplate()**

  **What to do**:
  - Add `GetTemplateParams` struct
  - Implement handler calling store.GetTemplate()

  **Implementation**:
  ```go
  type GetTemplateParams struct {
      TemplateID string `json:"template_id"`
  }
  
  func (h *RPCHandler) handleGetTemplate(req *RPCRequest) *RPCResponse {
      var params GetTemplateParams
      if err := json.Unmarshal(req.Params, &params); err != nil {
          return ErrorResponse(ErrInvalidParams, "Invalid params: "+err.Error())
      }
      
      template, err := h.store.GetTemplate(context.Background(), params.TemplateID)
      if err != nil {
          return ErrorResponse(ErrNotFound, "Template not found: "+params.TemplateID)
      }
      
      return SuccessResponse(template)
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Gap 3 RPC Methods
  - **Blocks**: G3.6
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/secretary/rpc.go:160-180` - handleGetWorkflow pattern
  - `bridge/pkg/secretary/store.go:262-295` - GetTemplate signature

---

- [ ] G3.3. **Add handleListTemplates()**

  **What to do**:
  - Add `ListTemplatesParams` struct
  - Implement handler calling store.ListTemplates()

  **Implementation**:
  ```go
  type ListTemplatesParams struct {
      ActiveOnly bool `json:"active_only,omitempty"`
  }
  
  func (h *RPCHandler) handleListTemplates(req *RPCRequest) *RPCResponse {
      var params ListTemplatesParams
      if len(req.Params) > 0 {
          if err := json.Unmarshal(req.Params, &params); err != nil {
              return ErrorResponse(ErrInvalidParams, "Invalid params: "+err.Error())
          }
      }
      
      filter := TemplateFilter{}
      if params.ActiveOnly {
          filter.IsActive = true
      }
      
      templates, err := h.store.ListTemplates(context.Background(), filter)
      if err != nil {
          return ErrorResponse(ErrInternal, "Failed to list templates: "+err.Error())
      }
      
      return SuccessResponse(map[string]interface{}{
          "templates": templates,
          "count":     len(templates),
      })
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Gap 3 RPC Methods
  - **Blocks**: G3.6
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/secretary/rpc.go:187-208` - handleListWorkflows pattern
  - `bridge/pkg/secretary/store.go:350-400` - ListTemplates signature

---

- [ ] G3.4. **Add handleUpdateTemplate()**

  **What to do**:
  - Add `UpdateTemplateParams` struct
  - Implement handler calling store.UpdateTemplate()

  **Implementation**:
  ```go
  type UpdateTemplateParams struct {
      TemplateID  string          `json:"template_id"`
      Name        string          `json:"name,omitempty"`
      Description string          `json:"description,omitempty"`
      Steps       []WorkflowStep  `json:"steps,omitempty"`
      Variables   json.RawMessage `json:"variables,omitempty"`
      PIIRefs     []string        `json:"pii_refs,omitempty"`
      IsActive    *bool           `json:"is_active,omitempty"`
  }
  
  func (h *RPCHandler) handleUpdateTemplate(req *RPCRequest) *RPCResponse {
      var params UpdateTemplateParams
      if err := json.Unmarshal(req.Params, &params); err != nil {
          return ErrorResponse(ErrInvalidParams, "Invalid params: "+err.Error())
      }
      
      template, err := h.store.GetTemplate(context.Background(), params.TemplateID)
      if err != nil {
          return ErrorResponse(ErrNotFound, "Template not found: "+params.TemplateID)
      }
      
      // Apply updates
      if params.Name != "" {
          template.Name = params.Name
      }
      if params.Description != "" {
          template.Description = params.Description
      }
      if params.Steps != nil {
          template.Steps = params.Steps
      }
      if params.Variables != nil {
          template.Variables = params.Variables
      }
      if params.PIIRefs != nil {
          template.PIIRefs = params.PIIRefs
      }
      if params.IsActive != nil {
          template.IsActive = *params.IsActive
      }
      template.UpdatedAt = time.Now().UnixMilli()
      
      if err := h.store.UpdateTemplate(context.Background(), template); err != nil {
          return ErrorResponse(ErrInternal, "Failed to update template: "+err.Error())
      }
      
      h.log.Info("template_updated_via_rpc", "template_id", template.ID, "by", req.UserID)
      
      return SuccessResponse(template)
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Gap 3 RPC Methods
  - **Blocks**: G3.6
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/secretary/store.go:300-350` - UpdateTemplate signature

---

- [ ] G3.5. **Add handleDeleteTemplate()**

  **What to do**:
  - Add `DeleteTemplateParams` struct
  - Implement handler calling store.DeleteTemplate()

  **Implementation**:
  ```go
  type DeleteTemplateParams struct {
      TemplateID string `json:"template_id"`
  }
  
  func (h *RPCHandler) handleDeleteTemplate(req *RPCRequest) *RPCResponse {
      var params DeleteTemplateParams
      if err := json.Unmarshal(req.Params, &params); err != nil {
          return ErrorResponse(ErrInvalidParams, "Invalid params: "+err.Error())
      }
      
      if err := h.store.DeleteTemplate(context.Background(), params.TemplateID); err != nil {
          return ErrorResponse(ErrInternal, "Failed to delete template: "+err.Error())
      }
      
      h.log.Info("template_deleted_via_rpc", "template_id", params.TemplateID, "by", req.UserID)
      
      return SuccessResponse(map[string]interface{}{
          "template_id": params.TemplateID,
          "deleted":     true,
      })
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Gap 3 RPC Methods
  - **Blocks**: G3.6
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/secretary/store.go:400-430` - DeleteTemplate signature

---

- [ ] G3.6. **Register Methods in Handle()**

  **What to do**:
  - Add new methods to switch statement in Handle()
  - Order alphabetically with other methods

  **Implementation**:
  ```go
  func (h *RPCHandler) Handle(req *RPCRequest) *RPCResponse {
      switch req.Method {
      // ... existing methods ...
      case "secretary.create_template":
          return h.handleCreateTemplate(req)
      case "secretary.get_template":
          return h.handleGetTemplate(req)
      case "secretary.list_templates":
          return h.handleListTemplates(req)
      case "secretary.update_template":
          return h.handleUpdateTemplate(req)
      case "secretary.delete_template":
          return h.handleDeleteTemplate(req)
      default:
          return ErrorResponse(ErrNotFound, fmt.Sprintf("Unknown method: %s", req.Method))
      }
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on G3.1-G3.5)
  - **Parallel Group**: Gap 3 Integration
  - **Blocks**: None
  - **Blocked By**: G3.1, G3.2, G3.3, G3.4, G3.5

  **Acceptance Criteria**:
  - [ ] All 5 methods registered
  - [ ] Methods callable via RPC

  **QA Scenarios**:
  ```
  Scenario: RPC template CRUD
    Tool: Bash (curl)
    Steps:
      1. curl -X POST -d '{"method":"secretary.create_template","params":{...}}'
      2. curl -X POST -d '{"method":"secretary.list_templates"}'
      3. curl -X POST -d '{"method":"secretary.get_template","params":{"template_id":"..."}}'
      4. curl -X POST -d '{"method":"secretary.update_template","params":{...}}'
      5. curl -X POST -d '{"method":"secretary.delete_template","params":{...}}'
    Expected Result: All operations succeed
    Evidence: .sisyphus/evidence/gap3-rpc-crud.txt
  ```

---

## Final Verification Wave

> Run after ALL gap fixes complete. 4 review agents run in PARALLEL. ALL must APPROVE.

- [ ] F1. **Gap Compliance Check** — `quick`
  
  **What to do**:
  - Read the plan end-to-end
  - For each gap: verify implementation exists (read file, check function)
  - Check evidence files exist in .sisyphus/evidence/
  
  **QA Scenarios**:
  ```
  Scenario: Gap 1 implementation check
    Tool: Bash (grep + file read)
    Steps:
      1. grep -l "evaluateConditions" bridge/pkg/secretary/template_engine.go
      2. Verify function body is NOT just "return steps, nil"
      3. Check for condition evaluation logic (if/switch statements)
    Expected Result: evaluateConditions() has actual implementation
    Failure Indicators: Function body is stub, no condition logic
    Evidence: .sisyphus/evidence/f1-gap1-check.txt

  Scenario: Gap 2 implementation check
    Tool: Bash (grep)
    Steps:
      1. grep -l "Timezone" bridge/pkg/secretary/orchestrator_scheduler.go
      2. grep -l "time.LoadLocation" bridge/pkg/secretary/orchestrator_scheduler.go
      3. Verify location field exists on Scheduler struct
    Expected Result: Timezone config and LoadLocation pattern present
    Failure Indicators: No Timezone field, no location usage
    Evidence: .sisyphus/evidence/f1-gap2-check.txt

  Scenario: Gap 3 implementation check
    Tool: Bash (grep)
    Steps:
      1. grep "secretary.create_template" bridge/pkg/secretary/rpc.go
      2. grep "secretary.list_templates" bridge/pkg/secretary/rpc.go
      3. grep "handleCreateTemplate" bridge/pkg/secretary/rpc.go
      4. Verify 5 template methods in Handle() switch
    Expected Result: All 5 template RPC methods registered
    Failure Indicators: Missing method names, no handlers
    Evidence: .sisyphus/evidence/f1-gap3-check.txt
  ```
  
  Output: `G1 [DONE/PENDING] | G2 [DONE/PENDING] | G3 [DONE/PENDING] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Build & Test** — `quick`
  
  **What to do**:
  - Run `go build` on secretary package
  - Run `go test` on secretary package
  - Check for compilation errors and test failures
  
  **QA Scenarios**:
  ```
  Scenario: Go build succeeds
    Tool: Bash (go build)
    Steps:
      1. cd /home/mink/src/armorclaw-omo/bridge
      2. go build ./pkg/secretary/...
    Expected Result: Build succeeds with no errors
    Failure Indicators: "undefined", "cannot find", "syntax error"
    Evidence: .sisyphus/evidence/f2-build.txt

  Scenario: Go tests pass
    Tool: Bash (go test)
    Steps:
      1. cd /home/mink/src/armorclaw-omo/bridge
      2. go test -v ./pkg/secretary/... -count=1
    Expected Result: All tests pass (0 failures)
    Failure Indicators: "FAIL", "panic", " assertion failed"
    Evidence: .sisyphus/evidence/f2-tests.txt
  ```
  
  Output: `Build [PASS/FAIL] | Tests [N pass/N fail] | VERDICT: APPROVE/REJECT`

- [ ] F3. **Regression Check** — `unspecified-high`
  
  **What to do**:
  - Run full secretary test suite
  - Verify existing features still work
  - Check for breaking changes in API signatures
  
  **QA Scenarios**:
  ```
  Scenario: Existing orchestrator tests pass
    Tool: Bash (go test)
    Steps:
      1. cd /home/mink/src/armorclaw-omo/bridge
      2. go test -v ./pkg/secretary/... -run TestOrchestrator -count=1
    Expected Result: All orchestrator tests pass
    Failure Indicators: "FAIL", regression in workflow execution
    Evidence: .sisyphus/evidence/f3-orchestrator.txt

  Scenario: Existing approval tests pass
    Tool: Bash (go test)
    Steps:
      1. cd /home/mink/src/armorclaw-omo/bridge
      2. go test -v ./pkg/secretary/... -run TestApproval -count=1
    Expected Result: All approval tests pass
    Failure Indicators: "FAIL", regression in approval flow
    Evidence: .sisyphus/evidence/f3-approvals.txt

  Scenario: Store operations unchanged
    Tool: Bash (go test)
    Steps:
      1. cd /home/mink/src/armorclaw-omo/bridge
      2. go test -v ./pkg/secretary/... -run TestStore -count=1
    Expected Result: All store tests pass
    Failure Indicators: "FAIL", regression in CRUD operations
    Evidence: .sisyphus/evidence/f3-store.txt
  ```
  
  Output: `Regressions [0/N] | VERDICT: APPROVE/REJECT`

- [ ] F4. **Python-Free Verification** — `quick`
  
  **What to do**:
  - Confirm Secretary is 100% Go
  - No Python/OMO files in bridge/pkg/secretary/
  - No Python dependencies added
  
  **QA Scenarios**:
  ```
  Scenario: No Python files in secretary package
    Tool: Bash (find)
    Steps:
      1. find /home/mink/src/armorclaw-omo/bridge/pkg/secretary -name "*.py"
    Expected Result: No files found (empty output)
    Failure Indicators: Any .py file paths returned
    Evidence: .sisyphus/evidence/f4-no-python.txt

  Scenario: No Python dependencies in go.mod
    Tool: Bash (grep)
    Steps:
      1. grep -i "python" /home/mink/src/armorclaw-omo/bridge/go.mod
    Expected Result: No matches (empty output)
    Failure Indicators: Any python-related lines
    Evidence: .sisyphus/evidence/f4-no-py-deps.txt

  Scenario: All source files are Go
    Tool: Bash (find + wc)
    Steps:
      1. find /home/mink/src/armorclaw-omo/bridge/pkg/secretary -name "*.go" | wc -l
      2. Verify count > 0 (existing Go files)
    Expected Result: Positive count of .go files
    Failure Indicators: Zero .go files (package deleted)
    Evidence: .sisyphus/evidence/f4-go-files.txt
  ```
  
  Output: `Python-free [YES/NO] | VERDICT: APPROVE/REJECT`

---

## Commit Strategy

- **Gap 1**: `feat(secretary): implement conditional branching in template engine`
- **Gap 2**: `feat(secretary): add timezone support to scheduler`
- **Gap 3**: `feat(secretary): add template CRUD RPC methods`

---

## Success Criteria

### Verification Commands
```bash
# Build succeeds
go build ./bridge/...

# All tests pass
go test ./bridge/pkg/secretary/... -v

# No Python files
find bridge/pkg/secretary -name "*.py" | wc -l  # Expected: 0

# Template RPC works
curl -X POST -d '{"method":"secretary.list_templates"}' http://localhost:8443/rpc
```

### Final Checklist
- [ ] All 3 gaps resolved
- [ ] Go tests pass
- [ ] No Python dependency introduced
- [ ] Secretary features complete at 100%
- [ ] Evidence files in .sisyphus/evidence/
