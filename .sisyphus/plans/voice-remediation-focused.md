# Voice Backend Remediation: Focused Completion

> **Quick Summary**: Address critical gaps in voice backend AI pipeline - service layer files, LLM integration, WebRTC production wiring, and E2E test refactoring.
> 
> **Deliverables**:
> - Service wrapper files (stt_service.go, tts_service.go, vad_service.go)
> - LLM integration replacing TODO stub in pipeline.go
> - WebRTC production wiring verification
> - E2E test file refactoring to resolve deadlock
> 
> **Estimated Effort**: Medium (2-3 hours)
> **Parallel Execution**: YES - 2 waves
> **Critical Path**: Service files → LLM integration → E2E tests → Verification

---

## Context

### Background
Final Verification Wave (F1-F4) identified critical gaps:
1. **F1 (Plan Compliance)**: Missing Tasks 8-10 (service layer files)
2. **F4 (Scope Fidelity)**: LLM stub at pipeline.go:263, WebRTC only in tests
3. **F3 (Manual QA)**: Integration test deadlock in TestIntegrationFullPipelineWithHITL

### User Directive (Option C)
> "Decoupling the Deadlock - Move integration scenarios to e2e_test.go with time.After gates"
> "Resolving the Missing Three - Generate lean service wrappers around existing clients"
> "LLM & WebRTC Production Wiring - Remove TODO, wire to ai.chat RPC, promote engine.go to production"

### Current State
| Component | Status | Action Needed |
|-----------|--------|---------------|
| stt_client.go | ✅ Complete | Wrap in service |
| tts_client.go | ✅ Complete | Wrap in service |
| vad_client.go | ✅ Complete | Wrap in service |
| pipeline.go:263 | ⚠️ Stubbed | Replace with LLM call |
| engine.go:239-279 | ✅ Production | Verify wiring |
| integration_test.go | ⚠️ Deadlock | Refactor to e2e_test.go |

---

## Work Objectives

### Core Objective
Complete the missing deliverables to pass Final Verification Wave.

### Concrete Deliverables
1. `bridge/pkg/voice/stt_service.go` - Thin wrapper around STTClient
2. `bridge/pkg/voice/tts_service.go` - Thin wrapper around TTSClient
3. `bridge/pkg/voice/vad_service.go` - Thin wrapper around VADClient
4. Updated `bridge/pkg/voice/pipeline.go` - LLM integration at line 263
5. `bridge/pkg/voice/e2e_test.go` - Refactored integration tests with proper timeouts
6. Evidence files in `.sisyphus/evidence/`

### Definition of Done
- [ ] All service files compile without errors
- [ ] All tests pass: `go test ./bridge/pkg/voice/...`
- [ ] LLM integration returns actual AI responses
- [ ] E2E tests complete without deadlock
- [ ] Final Verification Wave passes (F1-F4 APPROVE)

### Must Have
- Service wrappers with retry logic and metrics
- LLM integration using existing ai.chat RPC
- E2E tests with time.After gates to prevent hanging

### Must NOT Have (Guardrails)
- Over-engineering service layer (keep thin)
- Breaking existing client implementations
- Adding new external dependencies
- Modifying test patterns that work

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (Go testing framework)
- **Automated tests**: YES (Tests after)
- **Framework**: `go test`

### QA Policy
Each task includes agent-executed QA scenarios with evidence capture.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — service wrappers):
├── Task R1: STT Service Wrapper [quick]
├── Task R2: TTS Service Wrapper [quick]
├── Task R3: VAD Service Wrapper [quick]
└── Task R4: Service Unit Tests [quick]

Wave 2 (After Wave 1 — integration):
├── Task R5: LLM Integration in pipeline.go [deep]
├── Task R6: WebRTC Production Verification [quick]
└── Task R7: E2E Test Refactoring [unspecified-high]

Wave FINAL (After ALL tasks — verification):
├── Task F1: Re-run Plan Compliance Audit
├── Task F2: Re-run Code Quality Review
├── Task F3: Re-run Manual QA
└── Task F4: Re-run Scope Fidelity Check
-> Present results -> Get explicit user okay
```

### Dependency Matrix
- **R1-R3**: No dependencies (can run in parallel)
- **R4**: Depends on R1-R3
- **R5**: Depends on R1-R3 (uses service interfaces)
- **R6**: No dependencies (verification only)
- **R7**: Depends on R5 (needs LLM for E2E)
- **FINAL**: Depends on R1-R7

---

## TODOs

### Wave 1: Service Wrappers

- [ ] R1. STT Service Wrapper

  **What to do**:
  - Create `bridge/pkg/voice/stt_service.go`
  - Wrap STTClient with service layer
  - Add retry logic with exponential backoff
  - Add metrics collection (latency, word count)
  - Zero audio buffer after transcription (security)
  - Handle context cancellation

  **Must NOT do**:
  - Re-implement client logic
  - Add new HTTP endpoints
  - Change client interface

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []
  
  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with R2, R3)
  - **Blocks**: R4, R5

  **References**:
  - `bridge/pkg/voice/stt_client.go` - Client to wrap
  - `bridge/internal/ai/retry.go` - Retry pattern
  
  **Acceptance Criteria**:
  - [ ] File created: `bridge/pkg/voice/stt_service.go`
  - [ ] `go build ./bridge/pkg/voice/...` → PASS
  - [ ] Service wraps client correctly

  **Commit**: YES
  - Message: `feat(voice): add STT service wrapper`
  - Files: `bridge/pkg/voice/stt_service.go`

---

- [ ] R2. TTS Service Wrapper

  **What to do**:
  - Create `bridge/pkg/voice/tts_service.go`
  - Wrap TTSClient with service layer
  - Add retry logic with exponential backoff
  - Add SSML normalization (optional)
  - Add metrics collection
  - Handle text length limits

  **Must NOT do**:
  - Re-implement client logic
  - Add SSML generation (just normalization)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with R1, R3)

  **References**:
  - `bridge/pkg/voice/tts_client.go` - Client to wrap

  **Acceptance Criteria**:
  - [ ] File created: `bridge/pkg/voice/tts_service.go`
  - [ ] `go build ./bridge/pkg/voice/...` → PASS

  **Commit**: YES
  - Message: `feat(voice): add TTS service wrapper`
  - Files: `bridge/pkg/voice/tts_service.go`

---

- [ ] R3. VAD Service Wrapper

  **What to do**:
  - Create `bridge/pkg/voice/vad_service.go`
  - Wrap VADClient with service layer
  - Add retry logic
  - Add threshold filtering
  - Add metrics collection

  **Must NOT do**:
  - Re-implement client logic
  - Change detection algorithm

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with R1, R2)

  **References**:
  - `bridge/pkg/voice/vad_client.go` - Client to wrap

  **Acceptance Criteria**:
  - [ ] File created: `bridge/pkg/voice/vad_service.go`
  - [ ] `go build ./bridge/pkg/voice/...` → PASS

  **Commit**: YES
  - Message: `feat(voice): add VAD service wrapper`
  - Files: `bridge/pkg/voice/vad_service.go`

---

- [ ] R4. Service Unit Tests

  **What to do**:
  - Create test files for service wrappers
  - Test retry logic
  - Test metrics collection
  - Test error handling

  **Must NOT do**:
  - Test client functionality (already tested)
  - Add integration tests (separate task)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: [`superpowers/test-driven-development`]

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: R1, R2, R3

  **Acceptance Criteria**:
  - [ ] `go test ./bridge/pkg/voice/...` → PASS

  **Commit**: YES
  - Message: `test(voice): add service wrapper unit tests`
  - Files: `bridge/pkg/voice/*_service_test.go`

---

### Wave 2: Integration

- [ ] R5. LLM Integration in pipeline.go

  **What to do**:
  - Replace TODO stub at line 263 in `pipeline.go`
  - Integrate with existing AI service
  - Call `ai.chat` RPC or equivalent
  - Handle LLM errors gracefully
  - Return actual AI response text

  **Code Location**: `bridge/pkg/voice/pipeline.go:263-264`
  ```go
  // Current (stub):
  // TODO: Integrate with LLM service
  responseText := transcription.Text
  
  // Target:
  response, err := p.llmClient.Chat(ctx, transcription.Text)
  if err != nil {
      slog.Warn("LLM chat failed", "error", err)
      responseText = transcription.Text // fallback
  } else {
      responseText = response.Text
  }
  ```

  **Must NOT do**:
  - Create new LLM client (use existing)
  - Add streaming responses (future task)
  - Block pipeline on LLM timeout

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: R1-R3

  **References**:
  - `bridge/internal/ai/client.go` - AI client pattern
  - `bridge/internal/ai/anthropic.go` - Anthropic integration
  - `bridge/internal/ai/openai.go` - OpenAI integration

  **Acceptance Criteria**:
  - [ ] TODO removed from pipeline.go
  - [ ] LLM integration returns actual responses
  - [ ] `go test ./bridge/pkg/voice/...` → PASS

  **Commit**: YES
  - Message: `feat(voice): integrate LLM service in voice pipeline`
  - Files: `bridge/pkg/voice/pipeline.go`

---

- [ ] R6. WebRTC Production Verification

  **What to do**:
  - Verify `engine.go:239-279` production wiring
  - Confirm OnTrack handler calls pipeline.ProcessAudio()
  - Verify audio flows from WebRTC → Pipeline
  - Add evidence file

  **Note**: Production code already exists at `engine.go:239-279`:
  ```go
  func (e *Engine) readRTP(track *webrtc.TrackRemote) {
      // ... reads RTP, creates AudioChunk, calls pipeline.ProcessAudio()
  }
  ```

  **Must NOT do**:
  - Modify working production code
  - Add test mocks to production

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with R5, R7)

  **Acceptance Criteria**:
  - [ ] Evidence file: `.sisyphus/evidence/task-14-webrtc-production.log`
  - [ ] Verification: `engine.go` has `readRTP` → `ProcessAudio` wiring

  **Commit**: NO (verification only)

---

- [ ] R7. E2E Test Refactoring

  **What to do**:
  - Move integration scenarios from `integration_test.go` to `e2e_test.go`
  - Add `time.After` gates to prevent deadlock
  - Use `select` blocks for timeout handling
  - Fix `TestIntegrationFullPipelineWithHITL` deadlock

  **Deadlock Root Cause**: HITL interlock waits forever for approval in test
  
  **Fix Pattern**:
  ```go
  select {
  case <-approvalChan:
      // Approval received
  case <-time.After(5 * time.Second):
      // Timeout - auto-approve in test
      hitl.HandleApproval(req.ID, "test-user", true, "test approval")
  }
  ```

  **Must NOT do**:
  - Remove test scenarios
  - Increase timeout beyond 10s
  - Skip HITL tests

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: R5 (needs LLM for E2E)

  **Acceptance Criteria**:
  - [ ] File created: `bridge/pkg/voice/e2e_test.go`
  - [ ] All E2E tests pass without deadlock
  - [ ] `go test -run TestE2E ./bridge/pkg/voice/...` → PASS

  **Commit**: YES
  - Message: `test(voice): refactor E2E tests with timeout gates`
  - Files: `bridge/pkg/voice/e2e_test.go`

---

### Wave FINAL: Verification

- [ ] F1. Re-run Plan Compliance Audit

  **What to do**:
  - Verify Tasks 8-10 now have service files
  - Verify Task 11 has LLM integration
  - Verify Task 14 has production wiring
  - Output: `Must Have [16/16] | VERDICT: APPROVE`

- [ ] F2. Re-run Code Quality Review

  **What to do**:
  - Run `go build ./bridge/pkg/voice/...`
  - Run `go test ./bridge/pkg/voice/...`
  - Check for anti-patterns
  - Output: `Build [PASS] | Tests [N pass/0 fail] | VERDICT: APPROVE`

- [ ] F3. Re-run Manual QA

  **What to do**:
  - Execute all E2E test scenarios
  - Capture evidence in `.sisyphus/evidence/final-qa/`
  - Output: `Scenarios [N/N pass] | VERDICT: APPROVE`

- [ ] F4. Re-run Scope Fidelity Check

  **What to do**:
  - Verify all remediation tasks completed
  - No scope creep
  - Output: `Tasks [20/20 compliant] | VERDICT: APPROVE`

---

## Commit Strategy

- **R1-R3**: `feat(voice): add service wrappers` — stt_service.go, tts_service.go, vad_service.go
- **R4**: `test(voice): add service unit tests` — *_service_test.go
- **R5**: `feat(voice): integrate LLM service` — pipeline.go
- **R6**: No commit (verification)
- **R7**: `test(voice): refactor E2E tests` — e2e_test.go

---

## Success Criteria

### Verification Commands
```bash
# Build check
cd bridge && go build ./pkg/voice/...

# Unit tests
cd bridge && go test ./pkg/voice/... -v

# E2E tests
cd bridge && go test -run TestE2E ./pkg/voice/... -v

# Coverage
cd bridge && go test ./pkg/voice/... -cover
```

### Final Checklist
- [ ] All service files created and compiling
- [ ] LLM integration working
- [ ] WebRTC production verified
- [ ] E2E tests pass without deadlock
- [ ] F1-F4 all APPROVE
- [ ] User explicit approval received
