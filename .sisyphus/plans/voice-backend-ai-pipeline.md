# Voice Backend AI Pipeline: Sovereign Voice Agent

> **Quick Summary**: Add local STT (Whisper), TTS (Piper), VAD (Silero), and HITL interlock for voice-initiated actions during Matrix calls.
> **Deliverables**:
> - STT/TTS/VAD Docker sidecar integration
> - Voice pipeline: WebRTC → VAD → STT → LLM → TTS → WebRTC)
> - HITL interlock with Matrix approval flow
> - Skill gate to disable MCP/BlindFill during calls
> - Matrix adapter m.call.* event routing
> - Voice manager wiring in main.go

> 
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 6 waves
> **Critical Path**: Matrix routing → Voice manager → Pipeline → HITL interlock

---

## Context

### Original Request
Implement the Voice Backend AI Pipeline with:
1. Local STT (Whisper sidecar)
2. Local TTS (Piper sidecar)
3. Voice Activity Detection (Silero VAD)
4. HITL interlock for sensitive actions
5. Skill gate for MCP/BlindFill during calls
### Interview Summary
**Key Discussions**:
- **Deployment**: Docker sidecars (Whisper :9001, Piper :9002, Silero VAD :9003)
- **VAD**: Silero for state-of-the-art turn detection
- **HITL**: Pause pipeline + Matrix notification with ✅/❌ reactions, 30s timeout, auto-reject
- **Storage**: No persistence - audio buffers zeroed after transcription
**Research Findings**:
- Existing voice infrastructure: WebRTC, Matrix signaling, security, budget tracking - all complete
- Integration gaps: Matrix adapter doesn't route m.call.* events, voice manager stubbed
- HITL patterns: three-way consent with ✅/❌ reactions
- AI service patterns: HTTP client with retry logic, provider registry
### Metis Review
**Identified Gaps** (addressed):
- **Turn-taking strategy**: Silence detection via VAD (simplest, natural flow)
- **Fallback strategy**: Retry 3x → fallback to text-only or error audio
- **Skill gate scope**: Entire call duration (simpler, more secure)
- **Testing strategy**: Unit tests with mocks, integration tests with mock sidecars
- **Edge cases**: Network interruption, service failure, user interruption, state desync - all defined
**Scope Boundaries** (LOCKED DOWN):
- **IN**: Voice calls, local STT/TTS, HITL approvals, skill gate, Matrix routing
- **OUT**: Multi-language (V1: English only), voice training, call recording, multi-party calls
**Performance Targets** (applied):
- End-to-end latency: < 500ms
- VAD chunk size: 2400ms
- STT/TTS timeout: 5000ms
- HITL timeout: 30s (configurable)
**Guardrails Applied** (from Metis review):
- **No storage**: Audio buffers zeroed immediately after transcription
- **Skill gate**: MCP/BlindFill disabled for entire call duration
- **E2EE termination**: All decryption in Bridge, not containers
- **Container isolation**: No audio devices in containers
- **Matrix as control plane**: All signaling via m.call.* events
- **Minimal patches**: Extend existing code, don't rewrite
---
## Work Objectives
### Core Objective
Add local AI voice pipeline (Whisper STT + Piper TTS + Silero VAD) to ArmorClaw Bridge with Human-in-the-Loop interlock for sensitive actions during voice calls.
### Concrete Deliverables
- `bridge/pkg/voice/stt_client.go` - Whisper HTTP client
- `bridge/pkg/voice/tts_client.go` - Piper HTTP client
- `bridge/pkg/voice/vad_client.go` - Silero VAD HTTP client
- `bridge/pkg/voice/pipeline.go` - Voice pipeline orchestrator
- `bridge/pkg/voice/skill_gate.go` - Skill gate for MCP/BlindFill
- `bridge/pkg/voice/hitl_interlock.go` - HITL interlock using existing consent patterns
- `bridge/internal/adapter/matrix.go` - Add m.call.* event routing
- `bridge/cmd/bridge/main.go` - Wire voice manager
- `deploy/ai/docker-compose.ai.yml` - Add Whisper/Piper/VAD sidecars
### Definition of Done
- [ ] User can initiate voice call to Matrix room
- [ ] User speaks → VAD detects speech → STT transcribes → LLM responds → TTS speaks → User hears response
- [ ] LLM requests sensitive action → HITL interlock pauses → Matrix approval → User approves → action executes
### Must Have
- Local STT/TTS/VAD services
- Voice pipeline integration with existing WebRTC infrastructure
- HITL interlock for sensitive actions
- Skill gate for MCP/BlindFill during calls
- m.call.* event routing in Matrix adapter
- Voice manager wired in main.go
### Must NOT Have (Guardrails)
- Multi-language support (V1: English only)
- Voice training/customization
- Call recording or storage
- Multi-party calls
- Background call continuity
- Audio devices in containers
- Bypassing Matrix for signaling
- Rewriting existing voice/WebRTC engine
---
## Verification Strategy (MANDATORY)
> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.
### Test Decision
- **Infrastructure exists**: YES (Go testing framework)
- **Automated tests**: YES (Tests after) - Unit tests with mocks, integration tests with mock sidecars, E2E tests with real containers
- **Framework**: `go test`
- **If TDD**: Each task follows RED (failing test) → GREEN (minimal impl) → REFACTOR
### QA Policy
Every task MUST include agent-executed QA scenarios (see TODO template below).
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.
- **Frontend/UI**: Use Playwright (playwright skill) — Navigate, interact, assert DOM, screenshot
- **TUI/CLI**: Use interactive_bash (tmux) — Run command, send keystrokes, validate output
- **API/Backend**: Use Bash (curl) — Send requests, assert status + response fields
- **Library/Module**: Use Bash (bun/node REPL) — Import, call functions, compare output
---
## Execution Strategy
### Parallel Execution Waves
```
Wave 1 (Start Immediately — foundation + config):
├── Task 1: Docker Compose sidecar config [quick]
├── Task 2: STT client (Whisper) [quick]
├── Task 3: TTS client (Piper) [quick]
├── Task 4: VAD client (Silero) [quick]
├── Task 5: Type definitions for voice pipeline [quick]
├── Task 6: Skill gate types [quick]
└── Task 7: HITL interlock types [quick]

Wave 2 (After Wave 1 — core services):
├── Task 8: STT service implementation [unspecified-high]
├── Task 9: TTS service implementation [unspecified-high]
├── Task 10: VAD service implementation [unspecified-high]
└── Task 11: Pipeline orchestrator [deep]

Wave 3 (After Wave 2 — integration):
├── Task 12: Matrix adapter m.call.* routing [quick]
├── Task 13: Voice manager wiring in main.go [quick]
├── Task 14: WebRTC OnTrack integration [deep]
├── Task 15: HITL interlock implementation [deep]
├── Task 16: Skill gate integration [deep]
Wave 4 (After Wave 3 — testing + documentation):
├── Task 17: Unit tests for all components [quick]
├── Task 18: Integration tests with mock sidecars [unspecified-high]
├── Task 19: E2E test with real containers [unspecified-high]
└── Task 20: Documentation update [quick]
Wave FINAL (After ALL tasks — 4 parallel reviews, then user okay):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: Task 1 → Task 8 → Task 11 → Task 15 → Wave 4 → user okay
Parallel Speedup: ~70% faster than sequential
Max Concurrent: 7 (Waves 1 & 2)
### Dependency Matrix (abbreviated — show ALL tasks in your generated plan)
- **1-7**: — 8-14
- **8**: 3, 5, 7 — 11, 14
- **11**: 8 — 14, 15
- **14**: 8, 11 — 15, 16
- **15**: 6, 11, 14 — 16
- **16**: 6, 15 — 16
- **17-19**: 11, 15, 16 — 20
- **20**: 17-19 — F1-F4

> This is abbreviated for reference. YOUR generated plan must include the FULL matrix for ALL tasks.
### Agent Dispatch Summary
- **1**: **7** — T1-T4 → `quick`, T5-T7 → `quick`
- **2**: **7** — T8-T10 → `unspecified-high`, T11 → `unspecified-high`, T14 → `deep`, T17-T19 → `quick`, T20 → `quick`
- **3**: **4** — T12-T13 → `quick`, T15-T16 → `deep`
- **4**: **4** — T17-T19 → `unspecified-high`, F1-F4 → `oracle`/`unspecified-high`/`deep`/`unspecified-high`/`git`
---
## TODOs
> Implementation + Test = ONE Task. Never separate.
> EVERY task MUST have: Recommended Agent Profile + Parallelization info + QA Scenarios.
> **A task WITHOUT QA Scenarios is INCOMPLETE. No exceptions.
- [x] 1. Docker Compose Sidecar Configuration
- [x] 2. STT Client (Whisper HTTP Client)
- [x] 3. TTS Client (Piper HTTP Client)
- [x] 4. VAD Client (Silero HTTP Client)
- [x] 5. Type Definitions for Voice Pipeline
- [x] 6. Skill Gate Types
- [x] 7. HITL Interlock Types
- [x] 8. STT Service Implementation
- [x] 9. TTS Service Implementation
- [x] 10. VAD Service Implementation
- [x] 11. Pipeline Orchestrator
- [x] 12. Matrix Adapter m.call.* Routing
- [x] 13. Voice Manager Wiring in main.go
- [x] 14. WebRTC OnTrack Integration
- [x] 15. HITL Interlock Implementation
- [x] 16. Skill Gate Integration
  **What to do**:
  - Modify `bridge/pkg/voice/skill_gate.go` to integrate with voice manager
  - Add EnableForCall(callID) method to enable skill gate for active call
  - Add DisableForCall(callID) method to disable skill gate after call
  - Add IsSkillAllowedDuringCall(skillID, callID) method
  - Integrate with existing skill execution in RPC handlers
  - Add logging for blocked skill attempts
  - Return clear error message: "Skill [skill] is disabled during voice calls"
  **Must NOT do**:
  - Block skills outside of voice call context
  - Block all skills (only MCP/BlindFill/PII)
  - Add network-level blocking
  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
    - **Category**: `deep`
    - Reason: Integration with existing skill execution and RPC handlers
    - **Skills**: []
    - No special skills needed for skill gate integration
  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential
  - **Blocks**: Tasks 17-20 (depend on skill gate)
  - **Blocked By**: Tasks 6, 13 (needs skill gate types and voice manager)
  **References** (CRITICAL - Be Exhaustive):
  **Pattern References** (existing code to follow):
  - `bridge/pkg/voice/security.go:89-145` - SecurityEnforcer integration pattern
    - `bridge/pkg/rpc/server.go:registerHandlers()` - Where to add skill checks
  **API/Type References** (contracts to implement against):
  - `bridge/pkg/voice/skill_gate.go:SkillGate` - Types from Task 6
    - `bridge/pkg/voice/manager.go:Manager` - Manager from Task 13
  **External References** (libraries and frameworks):
  - Existing skill IDs: MCP, BlindFill, PII injection, browser_fill_with_pii
  **WHY Each Reference Matters**:
    - `security.go` - Shows how to integrate security checks with voice manager
    - `registerHandlers()` - Shows where skill execution handlers are registered
    - Skill IDs - Need to know exact identifiers to block
  **Acceptance Criteria**:
    > **AGENT-EXECUTABLE VERIFICATION ONLY** — No human action permitted.
    **If TDD (tests enabled):**
    - [ ] Test file created: `bridge/pkg/voice/skill_gate_integration_test.go`
    - [ ] bun test `bridge/pkg/voice/skill_gate_integration_test.go` → PASS (4 tests, 0 failures)
    **QA Scenarios (MANDATORY — task is INCOMPLETE without these):**
  ```
  Scenario: [Happy path — MCP blocked during active call]
    Tool: Bash (go test)
    Preconditions: Voice manager with active call, skill gate enabled
    Steps:
      1. `cd /home/mink/src/armorclaw-omo/bridge`
      2. `go test -run TestSkillGateMCPBlocked ./pkg/voice/...`
      3. Enable skill gate for call
      4. Try to execute MCP skill
      5. Verify error returned: "Skill mcp is disabled during voice calls"
    Expected Result: Test passes, MCP blocked
    Failure Indicators: Test fails, MCP allowed
    Evidence: .sisyphus/evidence/task-16-skill-gate-mcp-blocked.log
  Scenario: [Allowed — non-sensitive skill allowed during call]
    Tool: Bash (go test)
    Preconditions: Voice manager with active call, skill gate enabled
    Steps:
      1. `cd /home/mink/src/armorclaw-omo/bridge`
      2. `go test -run TestSkillGateNonSensitiveAllowed ./pkg/voice/...`
      3. Enable skill gate for call
      4. Try to execute non-sensitive skill (e.g., web_browsing)
      5. Verify skill is allowed
    Expected Result: Test passes, skill allowed
    Failure Indicators: Test fails, skill blocked
    Evidence: .sisyphus/evidence/task-16-skill-gate-non-sensitive.log
  Scenario: [Lifecycle — skill gate disabled after call ends]
    Tool: Bash (go test)
    Preconditions: Voice manager with ended call
    Steps:
      1. `cd /home/mink/src/armorclaw-omo/bridge`
      2. `go test -run TestSkillGateDisabledAfterCall ./pkg/voice/...`
      3. Enable skill gate for call
      4. End call
      5. Try to execute MCP skill
      6. Verify skill is allowed
    Expected Result: Test passes, skills allowed after call
    Failure Indicators: Test fails, skills still blocked
    Evidence: .sisyphus/evidence/task-16-skill-gate-after-call.log
  Scenario: [Error handling — clear error message for blocked skill]
    Tool: Bash (go test)
    Preconditions: Voice manager with active call
    Steps:
      1. `cd /home/mink/src/armorclaw-omo/bridge`
      2. `go test -run TestSkillGateErrorMessage ./pkg/voice/...`
      3. Enable skill gate for call
      4. Try to execute BlindFill skill
      5. Verify error message contains skill name and "voice calls"
    Expected Result: Test passes, clear error message
    Failure Indicators: Test fails, generic error message
    Evidence: .sisyphus/evidence/task-16-skill-gate-error-message.log
  **Evidence to Capture:**
  - [ ] Each evidence file named: task-16-skill-gate-mcp-blocked.log
  - [ ] Terminal output showing test results
  **Commit**: NO (groups with N)
  - Message: `feat(voice): integrate skill gate with voice manager`
  - Files: `bridge/pkg/voice/skill_gate.go`, `bridge/pkg/voice/skill_gate_integration_test.go`
    - Pre-commit: `go test ./bridge/pkg/voice/...`
---
## Wave 4 (After Wave 3 — testing + documentation)
- [x] 17. Unit Tests for All Components
  **What to do**:
  - Ensure all unit tests pass for voice pipeline components
  - Run full test suite: `go test ./bridge/pkg/voice/... -v`
  - Add missing test coverage for edge cases
  - Verify mock implementations for sidecar services
  - Add benchmark tests for audio processing performance
  **Must NOT do**:
  - Add integration tests (separate task)
  - Test with real sidecar services (separate task)
  - Skip failing tests (fix them)
  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
    - **Category**: `quick`
    - Reason: Running and verifying existing tests
    - **Skills**: []
    - No special skills needed for test verification
  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 18, 19, 20)
  - **Blocks**: Tasks F1-F4 (depend on passing tests)
  - **Blocked By**: Tasks 1-16 (need implementation)
  **References** (CRITICAL - Be Exhaustive):
  **Pattern References** (existing code to follow):
  - `bridge/pkg/voice/*_test.go` - All test files created in previous tasks
    - `bridge/pkg/rpc/server_test.go` - Test pattern reference
  **WHY Each Reference Matters**:
    - Test files - Need to verify all tests pass
        - `server_test.go` - Shows existing test patterns in the codebase
  **Acceptance Criteria**:
    > **AGENT-EXECUTABLE VERIFICATION ONLY** — No human action permitted.
    **If TDD (tests enabled):**
    - [ ] All test files verified: `bridge/pkg/voice/*_test.go`
    - [ ] bun test `./bridge/pkg/voice/...` → PASS (all tests, 0 failures)
    **QA Scenarios (MANDATORY — task is INCOMPLETE without these):**
  ```
  Scenario: [Happy path — all unit tests pass]
    Tool: Bash (go test)
    Preconditions: All voice components implemented
    Steps:
      1. `cd /home/mink/src/armorclaw-omo/bridge`
      2. `go test ./pkg/voice/... -v -count=1`
      3. Verify all tests pass
      4. Verify coverage > 70%
    Expected Result: All tests pass, coverage acceptable
    Failure Indicators: Any test fails, coverage < 70%
    Evidence: .sisyphus/evidence/task-17-unit-tests-pass.log
  Scenario: [Benchmark — audio processing performance acceptable]
    Tool: Bash (go test -bench)
    Preconditions: Benchmark tests added
    Steps:
      1. `cd /home/mink/src/armorclaw-omo/bridge`
      2. `go test -bench=. ./pkg/voice/... -benchmem`
      3. Verify STT latency < 500ms (mock)
      4. Verify TTS latency < 500ms (mock)
      5. Verify memory usage reasonable
    Expected Result: Benchmarks pass, performance acceptable
    Failure Indicators: Any benchmark fails, latency too high
    Evidence: .sisyphus/evidence/task-17-benchmarks.log
  Scenario: [Coverage — all components have test coverage]
    Tool: Bash (go test -cover)
    Preconditions: All tests written
    Steps:
      1. `cd /home/mink/src/armorclaw-omo/bridge`
      2. `go test ./pkg/voice/... -coverprofile=coverage.out`
      3. `go tool cover -func=coverage.out`
      4. Verify all functions have coverage
      5. Verify no missing edge case tests
    Expected Result: Full coverage, all edge cases tested
    Failure Indicators: Coverage gaps, missing edge cases
    Evidence: .sisyphus/evidence/task-17-coverage.log
  **Evidence to Capture:**
  - [ ] Each evidence file named: task-17-unit-tests-pass.log
  - [ ] Terminal output showing test results
  **Commit**: NO (groups with N)
  - Message: `test(voice): verify all unit tests pass`
  - Files: Test files only
  - Pre-commit: N/A (test task)
---
- [x] 18. Integration Tests with Mock Sidecars
  **What to do**:
  - Create integration tests with mocked sidecar services
  - Mock Whisper API responses in tests
  - Mock Piper API responses in tests
  - Mock Silero VAD responses in tests
  - Test full pipeline flow with mocks
  - Test HITL interlock with mocks
  - Test error handling with mock failures
  **Must NOT do**:
  - Use real sidecar services (separate task)
  - Skip error scenario tests
  - Add performance tests (separate task)
  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
    - **Category**: `unspecified-high`
    - Reason: Complex integration tests with mocking
    - **Skills**: []
    - No special skills needed for integration tests
  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 17, 19, 20)
  - **Blocks**: Tasks F1-F4 (depend on passing tests)
  - **Blocked By**: Tasks 1-16 (need implementation)
  **References** (CRITICAL - Be Exhaustive):
  **Pattern References** (existing code to follow):
  - `bridge/pkg/voice/stt_service_test.go` - Mock pattern for STT
    - `bridge/pkg/voice/tts_service_test.go` - Mock pattern for TTS
    - `bridge/pkg/voice/vad_service_test.go` - Mock pattern for VAD
  **WHY Each Reference Matters**:
    - Test files - Show how to mock sidecar services
  **Acceptance Criteria**:
    > **AGENT-EXECUTABLE VERIFICATION ONLY** — No human action permitted.
    **If TDD (tests enabled):**
    - [ ] Test file created: `bridge/pkg/voice/integration_test.go`
    - [ ] bun test `bridge/pkg/voice/integration_test.go` → PASS (6 tests, 0 failures)
    **QA Scenarios (MANDATORY — task is INCOMPLETE without these):**
  ```
  Scenario: [Happy path — full pipeline with mocks]
    Tool: Bash (go test)
    Preconditions: All mock services implemented
    Steps:
      1. `cd /home/mink/src/armorclaw-omo/bridge`
      2. `go test -run TestIntegrationFullPipeline ./pkg/voice/...`
      3. Mock Whisper returns "Hello"
      4. Mock LLM returns "Hi there"
      5. Mock Piper returns audio
      6. Verify full pipeline completes
    Expected Result: Test passes, full pipeline works
    Failure Indicators: Test fails, pipeline stuck
    Evidence: .sisyphus/evidence/task-18-integration-pipeline.log
  Scenario: [Error handling — STT failure with mocks]
    Tool: Bash (go test)
    Preconditions: Mock services with failure simulation
    Steps:
      1. `cd /home/mink/src/armorclaw-omo/bridge`
      2. `go test -run TestIntegrationSTTFailure ./pkg/voice/...`
      3. Mock Whisper returns error
      4. Verify pipeline handles error gracefully
      5. Verify retry logic is triggered
    Expected Result: Test passes, error handled
    Failure Indicators: Test fails, pipeline crashes
    Evidence: .sisyphus/evidence/task-18-integration-stt-failure.log
  Scenario: [HITL — approval flow with mocks]
    Tool: Bash (go test)
    Preconditions: Mock consent manager
    Steps:
      1. `cd /home/mink/src/armorclaw-omo/bridge`
      2. `go test -run TestIntegrationHITLApproval ./pkg/voice/...`
      3. Mock sensitive action
      4. Mock approval reaction
      5. Verify pipeline resumes
      6. Verify action executes
    Expected Result: Test passes, HITL works
    Failure Indicators: Test fails, HITL stuck
    Evidence: .sisyphus/evidence/task-18-integration-hitl.log
  Scenario: [Timeout — HITL timeout with mocks]
    Tool: Bash (go test)
    Preconditions: Mock consent manager with timeout
    Steps:
      1. `cd /home/mink/src/armorclaw-omo/bridge`
      2. `go test -run TestIntegrationHITLTimeout ./pkg/voice/...`
      3. Mock sensitive action
      4. Wait for timeout (5s in test)
      5. Verify auto-reject
      6. Verify pipeline resumes
    Expected Result: Test passes, timeout works
    Failure Indicators: Test fails, no auto-reject
    Evidence: .sisyphus/evidence/task-18-integration-timeout.log
  Scenario: [Skill gate — MCP blocked with mocks]
    Tool: Bash (go test)
    Preconditions: Mock skill gate
    Steps:
      1. `cd /home/mink/src/armorclaw-omo/bridge`
      2. `go test -run TestIntegrationSkillGate ./pkg/voice/...`
      3. Enable skill gate
      4. Try MCP skill
      5. Verify skill blocked
      6. Verify error message
    Expected Result: Test passes, skill gate works
    Failure Indicators: Test fails, skill allowed
    Evidence: .sisyphus/evidence/task-18-integration-skill-gate.log
  Scenario: [Recovery — service recovery after failure]
    Tool: Bash (go test)
    Preconditions: Mock services with transient failures
    Steps:
      1. `cd /home/mink/src/armorclaw-omo/bridge`
      2. `go test -run TestIntegrationServiceRecovery ./pkg/voice/...`
      3. Mock Whisper fails once then succeeds
      4. Verify retry logic works
      5. Verify pipeline completes
    Expected Result: Test passes, recovery works
    Failure Indicators: Test fails, no retry
    Evidence: .sisyphus/evidence/task-18-integration-recovery.log
  **Evidence to Capture:**
  - [ ] Each evidence file named: task-18-integration-pipeline.log
  - [ ] Terminal output showing test results
  **Commit**: NO (groups with N)
  - Message: `test(voice): add integration tests with mock sidecars`
  - Files: `bridge/pkg/voice/integration_test.go`
  - Pre-commit: `go test ./bridge/pkg/voice/...`
---
- [x] 19. E2E Test with Real Containers
  **What to do**:
  - Create E2E test with real Docker sidecar containers
  - Start Whisper, Piper, and Silero VAD containers in test
  - Test real transcription with actual audio
  - Test real synthesis with actual text
  - Test real VAD with actual audio
  - Verify latency meets targets (< 500ms)
  - Clean up containers after test
  **Must NOT do**:
  - Run in CI without container support
  - Skip cleanup (containers must be removed)
  - Use production containers (use test-specific)
  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
    - **Category**: `unspecified-high`
    - Reason: E2E testing with real services
    - **Skills**: []
    - No special skills needed for E2E tests
  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 17, 18, 20)
  - **Blocks**: Tasks F1-F4 (depend on passing tests)
  - **Blocked By**: Tasks 1, 18 (need implementation and mocks)
  **References** (CRITICAL - Be Exhaustive):
  **Pattern References** (existing code to follow):
  - `bridge/pkg/voice/integration_test.go` - Integration test pattern
    - `deploy/ai/docker-compose.voice.yml` - Docker Compose for sidecars
  **External References** (libraries and frameworks):
  - Docker SDK: `github.com/docker/docker/client` - Container management
    - Test containers: `github.com/testcontainers/testcontainers-go` - E2E testing
  **WHY Each Reference Matters**:
    - `integration_test.go` - Shows how to structure integration tests
    - `docker-compose.voice.yml` - Shows sidecar configuration for tests
        - Docker SDK - Need to start/stop containers programmatically
  **Acceptance Criteria**:
    > **AGENT-EXECUTABLE VERIFICATION ONLY** — No human action permitted.
    **If TDD (tests enabled):**
    - [ ] Test file created: `bridge/pkg/voice/e2e_test.go`
    - [ ] bun test `bridge/pkg/voice/e2e_test.go` → PASS (4 tests, 0 failures)
    **QA Scenarios (MANDATORY — task is INCOMPLETE without these):**
  ```
  Scenario: [Happy path — real transcription with Whisper]
    Tool: Bash (go test)
    Preconditions: Whisper container running
    Steps:
      1. `cd /home/mink/src/armorclaw-omo/bridge`
      2. `go test -run TestE2EWhisperTranscription ./pkg/voice/... -tags=e2e`
      3. Load test audio file (hello.pcm)
      4. Send to real Whisper container
      5. Verify transcription contains "hello"
      6. Verify latency < 500ms
    Expected Result: Test passes, real transcription works
    Failure Indicators: Test fails, no transcription or high latency
    Evidence: .sisyphus/evidence/task-19-e2e-whisper.log
  Scenario: [Happy path — real synthesis with Piper]
    Tool: Bash (go test)
    Preconditions: Piper container running
    Steps:
      1. `cd /home/mink/src/armorclaw-omo/bridge`
      2. `go test -run TestE2EPiperSynthesis ./pkg/voice/... -tags=e2e`
      3. Send "Hello, world" to real Piper container
      4. Verify audio bytes returned
      5. Verify audio duration matches text length
      6. Verify latency < 500ms
    Expected Result: Test passes, real synthesis works
    Failure Indicators: Test fails, no audio or high latency
    Evidence: .sisyphus/evidence/task-19-e2e-piper.log
  Scenario: [Happy path — real VAD with Silero]
    Tool: Bash (go test)
    Preconditions: Silero VAD container running
    Steps:
      1. `cd /home/mink/src/armorclaw-omo/bridge`
      2. `go test -run TestE2ESileroVAD ./pkg/voice/... -tags=e2e`
      3. Load test audio file (speech.pcm)
      4. Send to real Silero VAD container
      5. Verify speech_detected is true
      6. Verify latency < 100ms
    Expected Result: Test passes, real VAD works
    Failure Indicators: Test fails, no detection or high latency
    Evidence: .sisyphus/evidence/task-19-e2e-silero.log
  Scenario: [Cleanup — containers removed after test]
    Tool: Bash (docker ps)
    Preconditions: E2E tests completed
    Steps:
      1. `docker ps -a --filter name=voice-test`
      2. Verify no test containers running
      3. Verify no test containers exist
    Expected Result: No test containers remaining
    Failure Indicators: Test containers still exist
    Evidence: .sisyphus/evidence/task-19-e2e-cleanup.log
  **Evidence to Capture:**
  - [ ] Each evidence file named: task-19-e2e-whisper.log
  - [ ] Terminal output showing test results
  **Commit**: NO (groups with N)
  - Message: `test(voice): add E2E tests with real containers`
  - Files: `bridge/pkg/voice/e2e_test.go`
  - Pre-commit: `go test ./bridge/pkg/voice/... -tags=e2e`
---
- [x] 20. Documentation Update
  **What to do**:
  - Update `docs/reference/voice-pipeline.md` with new architecture
  - Document STT/TTS/VAD sidecar configuration
  - Document HITL interlock behavior
  - Document skill gate restrictions
  - Add configuration examples
  - Add troubleshooting guide
  **Must NOT do**:
  - Create new documentation structure
  - Document internal implementation details
  - Add API documentation (separate task)
  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
    - **Category**: `quick`
    - Reason: Documentation updates
    - **Skills**: []
    - No special skills needed for documentation
  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 17, 18, 19)
  - **Blocks**: Tasks F1-F4 (depend on documentation)
  - **Blocked By**: Tasks 1-16 (need implementation details)
  **References** (CRITICAL - Be Exhaustive):
  **Pattern References** (existing code to follow):
  - `docs/reference/rpc-api.md` - Documentation pattern
    - `README.md` - README structure
  **WHY Each Reference Matters**:
    - `rpc-api.md` - Shows how to document API and configuration
    - `README.md` - Shows how to document features
  **Acceptance Criteria**:
    > **AGENT-EXECUTABLE VERIFICATION ONLY** — No human action permitted.
    **If TDD (tests enabled):**
    - [ ] Documentation file created: `docs/reference/voice-pipeline.md`
    - [ ] File contains all sections: Architecture, Configuration, HITL, Skill Gate
    **QA Scenarios (MANDATORY — task is INCOMPLETE without these):**
  ```
  Scenario: [Happy path — documentation complete]
    Tool: Bash (ls -la)
    Preconditions: Documentation written
    Steps:
      1. `ls -la /home/mink/src/armorclaw-omo/docs/reference/voice-pipeline.md`
      2. Verify file exists
      3. Verify file size > 2000 bytes
    Expected Result: Documentation file exists
    Failure Indicators: File missing or too small
    Evidence: .sisyphus/evidence/task-20-docs-exist.log
  Scenario: [Content — all sections present]
    Tool: Bash (grep)
    Preconditions: Documentation file exists
    Steps:
      1. `grep -c "## Architecture" /home/mink/src/armorclaw-omo/docs/reference/voice-pipeline.md`
      2. `grep -c "## Configuration" /home/mink/src/armorclaw-omo/docs/reference/voice-pipeline.md`
      3. `grep -c "## HITL Interlock" /home/mink/src/armorclaw-omo/docs/reference/voice-pipeline.md`
      4. `grep -c "## Skill Gate" /home/mink/src/armorclaw-omo/docs/reference/voice-pipeline.md`
      5. Verify all sections present
    Expected Result: All sections present
    Failure Indicators: Any section missing
    Evidence: .sisyphus/evidence/task-20-docs-sections.log
  Scenario: [Examples — configuration examples present]
    Tool: Bash (grep)
    Preconditions: Documentation file exists
    Steps:
      1. `grep -c '```yaml' /home/mink/src/armorclaw-omo/docs/reference/voice-pipeline.md`
      2. `grep -c 'STT_URL' /home/mink/src/armorclaw-omo/docs/reference/voice-pipeline.md`
      3. Verify configuration examples present
    Expected Result: Examples present
    Failure Indicators: No examples
    Evidence: .sisyphus/evidence/task-20-docs-examples.log
  Scenario: [Troubleshooting — troubleshooting section present]
    Tool: Bash (grep)
    Preconditions: Documentation file exists
    Steps:
      1. `grep -c "## Troubleshooting" /home/mink/src/armorclaw-omo/docs/reference/voice-pipeline.md`
      2. `grep -c "STT.*failed" /home/mink/src/armorclaw-omo/docs/reference/voice-pipeline.md`
      3. Verify troubleshooting guide present
    Expected Result: Troubleshooting section present
    Failure Indicators: No troubleshooting
    Evidence: .sisyphus/evidence/task-20-docs-troubleshooting.log
  **Evidence to Capture:**
  - [ ] Each evidence file named: task-20-docs-exist.log
  - [ ] Terminal output showing documentation structure
  **Commit**: NO (groups with N)
  - Message: `docs(voice): add voice pipeline documentation`
  - Files: `docs/reference/voice-pipeline.md`
  - Pre-commit: N/A (documentation task)
---
## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
>
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [x] F1. Plan Compliance Audit (oracle)
- [x] F2. Code Quality Review (unspecified-high)
- [x] F3. Real Manual QA (unspecified-high)
- [x] F4. Scope Fidelity Check (deep)
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination: Task N touching Task M's files. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`
---
## Commit Strategy
- **1**: `feat(voice): add Docker Compose sidecar config` — deploy/ai/docker-compose.voice.yml, docker compose config
- **2**: `feat(voice): add STT client with Whisper integration` — bridge/pkg/voice/stt_client.go, stt_client_test.go, go test
- **3**: `feat(voice): add TTS client with Piper integration` — bridge/pkg/voice/tts_client.go, tts_client_test.go, go test
- **4**: `feat(voice): add VAD client with Silero integration` — bridge/pkg/voice/vad_client.go, vad_client_test.go, go test
- **5**: `feat(voice): add type definitions for voice pipeline` — bridge/pkg/voice/types.go, types_test.go, go test
- **6**: `feat(voice): add skill gate for MCP/BlindFill during calls` — bridge/pkg/voice/skill_gate.go, skill_gate_test.go, go test
- **7**: `feat(voice): add HITL interlock types for voice pipeline` — bridge/pkg/voice/hitl_interlock.go, hitl_interlock_test.go, go test
- **8**: `feat(voice): implement STT service with Whisper integration` — bridge/pkg/voice/stt_service.go, stt_service_test.go, go test
- **9**: `feat(voice): implement TTS service with Piper integration` — bridge/pkg/voice/tts_service.go, tts_service_test.go, go test
- **10**: `feat(voice): implement VAD service with Silero integration` — bridge/pkg/voice/vad_service.go, vad_service_test.go, go test
- **11**: `feat(voice): implement voice pipeline orchestrator` — bridge/pkg/voice/pipeline.go, pipeline_test.go, go test
- **12**: `feat(voice): route m.call.* events to voice manager` — bridge/internal/adapter/matrix.go, matrix_call_test.go, go test
- **13**: `feat(voice): wire voice manager in main.go` — bridge/cmd/bridge/main.go, main_voice_test.go, go test
- **14**: `feat(voice): integrate WebRTC OnTrack with voice pipeline` — bridge/pkg/webrtc/engine.go, engine_voice_test.go, go test
- **15**: `feat(voice): implement HITL interlock for voice-initiated actions` — bridge/pkg/voice/hitl_interlock.go, hitl_interlock_impl_test.go, go test
- **16**: `feat(voice): integrate skill gate with voice manager` — bridge/pkg/voice/skill_gate.go, skill_gate_integration_test.go, go test
- **17**: `test(voice): verify all unit tests pass` — Test files, go test
- **18**: `test(voice): add integration tests with mock sidecars` — bridge/pkg/voice/integration_test.go, go test
- **19**: `test(voice): add E2E tests with real containers` — bridge/pkg/voice/e2e_test.go, go test -tags=e2e
- **20**: `docs(voice): add voice pipeline documentation` — docs/reference/voice-pipeline.md
---
## Success Criteria
### Verification Commands
```bash
# Start sidecars
docker compose -f deploy/ai/docker-compose.voice.yml up -d

# Run all voice tests
cd bridge && go test ./pkg/voice/... -v

# Run E2E tests (requires sidecars)
cd bridge && go test ./pkg/voice/... -tags=e2e

# Verify voice manager starts
curl http://localhost:8443/status | grep voice
```
### Final Checklist
- [ ] All "Must Have" present (STT/TTS/VAD clients, pipeline, HITL, skill gate)
- [ ] All "Must NOT Have" absent (audio storage, skills allowed during calls)
- [ ] All tests pass
- [ ] Sidecars healthy
- [ ] Voice manager starts without error
