# Rewrite voice-stack.md to Reflect v1.0.0 Voice Pipeline

## TL;DR

> **Quick Summary**: Full rewrite of /doc/voice-stack.md to reflect implemented OpenAI Whisper STT, OpenAI TTS, energy-threshold VAD, and PCM routing pipeline. ~40% of the file needs rewriting (Current State, Package Table, Speech Services, Architecture Diagram, Config); ~60% is still accurate (WebRTC, TURN, Budget, Audio, Integration Points).
>
> **Deliverables**:
> - Updated /doc/voice-stack.md reflecting all implemented voice providers
>
> **Estimated Effort**: Quick
> **Parallel Execution**: NO — single file
> **Critical Path**: T1 → T2

---

## Context

### Original Request
User asked to "update /doc/ markdown files to reflect current codebase, especially components changed." Assessment revealed voice-stack.md is the only critically stale doc — architecture.md and bridge-reference.md were already updated in T14.

### Codebase-Verified Findings
- **STT implemented**: `bridge/pkg/voice/stt_openai.go` — OpenAI Whisper, PCM→WAV→transcription
- **TTS implemented**: `bridge/pkg/voice/tts_openai.go` — OpenAI tts-1 model, text→speech
- **VAD implemented**: `bridge/pkg/voice/vad.go` — energy-threshold (RMS per frame), configurable
- **PCM routing implemented**: `bridge/pkg/voice/pcm.go` — Input PCM → VAD → STT → text → agent → text → TTS → output PCM
- **VoiceVADConfig added**: `bridge/pkg/config/config.go` — energy_threshold, frame_duration_ms, silence_duration_ms, sample_rate
- **Error codes**: -32007 (not configured), -32008 (rate limited)
- **Interface mismatch resolved**: T4 was a no-op — interfaces already reconciled
- **E2E providers test**: `bridge/pkg/voice/e2e_providers_test.go` (1102 lines) with mocked HTTP

### Metis Review (Self-Applied)
**Guardrails Applied**:
- Don't touch architecture.md or bridge-reference.md — already accurate
- Preserve ~60% of voice-stack.md that's still correct (WebRTC, TURN, budget, audio config, integration points)
- Don't add new doc files
- Max 8 active markdown files in /doc/ (currently 9 excluding ArmorChat.md)
- No AI slop: no filler sections, no excessive comments, no generic descriptions

---

## Work Objectives

### Core Objective
Rewrite voice-stack.md to accurately reflect the implemented voice pipeline: OpenAI Whisper STT, OpenAI TTS, energy-threshold VAD, and PCM routing through the Bridge.

### Concrete Deliverables
- `/doc/voice-stack.md` — fully accurate documentation of the voice stack

### Definition of Done
- Zero occurrences of "interface only", "no implementation", "not implemented", "no provider" in voice-stack.md
- All 4 new files documented: stt_openai.go, tts_openai.go, vad.go, pcm.go
- PCM routing flow diagram shows the implemented pipeline
- VAD config section documents VoiceVADConfig fields
- Error codes (-32007, -32008) documented

### Must Have
- Updated "Current State" reflecting implemented providers
- Updated package table including all new files
- PCM routing architecture diagram
- VAD configuration documentation
- Error code documentation
- OpenAI provider details (Whisper STT, tts-1 TTS)

### Must NOT Have (Guardrails)
- Do NOT modify architecture.md or bridge-reference.md
- Do NOT create new documentation files
- Do NOT add AI slop (filler sections, excessive verbosity, generic descriptions)
- Do NOT remove accurate existing sections (WebRTC, TURN, budget, audio config)
- Do NOT document test file internals (e2e_providers_test.go structure is irrelevant to users)
- Do NOT include "Interface Discrepancy" section if resolved (T4 was a no-op)

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed.

### QA Policy
- Grep for stale claims in the rewritten file
- Verify referenced files exist in codebase
- Verify architecture.md and bridge-reference.md were NOT modified

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Single task):
└── T1: Rewrite voice-stack.md [writing]

Wave 2 (Verification):
└── T2: Verify accuracy — grep for stale claims, check file refs [quick]
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1 | — | T2 | 1 |
| T2 | T1 | — | 2 |

### Agent Dispatch Summary

- **Wave 1**: 1 task — T1 → `writing`
- **Wave 2**: 1 task — T2 → `quick`

---

## TODOs

- [x] 1. Rewrite /doc/voice-stack.md

  **What to do**:
  - **PRESERVE these sections verbatim** (still accurate):
    - "Overview" (lines 87-91) — general description of voice stack purpose
    - "Architecture" diagram (lines 93-127) — UPDATE the diagram to show PCM routing flow (VAD → STT → agent → TTS)
    - "Key Packages" → `bridge/pkg/audio/` (lines 130-140) — still accurate
    - "Key Packages" → `bridge/pkg/webrtc/` (lines 165-176) — still accurate
    - "Key Packages" → `bridge/pkg/turn/` (lines 180-188) — still accurate
    - "Configuration" → Budget Configuration (lines 233-242) — still accurate
    - "Configuration" → Audio Configuration (lines 244-256) — still accurate
    - "Integration Points" (lines 258-274) — still accurate
  - **REWRITE these sections**:
    - **"Current State"** (lines 5-7): Replace "no AI provider backends exist yet" with accurate description of implemented OpenAI providers
    - **"What Exists"** table (lines 9-23): Add entries for stt_openai.go, tts_openai.go, vad.go, pcm.go, e2e_providers_test.go
    - **"What Is Missing"** table (lines 25-32): Remove STT/TTS/VAD/Audio Pipeline entries (now implemented). Replace with actual remaining gaps (if any — e.g., local STT/TTS, ONNX models)
    - **"Runtime Reality"** (lines 34-58): Update to reflect CreatePCMRouter wiring and PCM pipeline
    - **"Interface Discrepancy"** (lines 60-76): REMOVE — T4 confirmed interfaces are reconciled
    - **"E2E Test Expectations"** (lines 78-85): UPDATE — e2e_providers_test.go uses mocked HTTP, not sidecar services
    - **"Key Packages" → `bridge/pkg/voice/`** table (lines 142-156): Add new files (stt_openai.go, tts_openai.go, vad.go, pcm.go, vad_pcm_test.go, e2e_providers_test.go). Update stt_service.go/tts_service.go descriptions. Remove "Interface only, no provider" claims.
    - **"Speech Services"** section (lines 190-217): REWRITE — replace "No concrete providers exist" with actual OpenAI provider details. Document:
      - OpenAI Whisper STT: PCM→WAV conversion, /v1/audio/transcriptions endpoint, language support
      - OpenAI TTS: tts-1 model, /v1/audio/speech endpoint, voice options
      - Energy-threshold VAD: RMS calculation, configurable threshold, frame-based processing
      - PCM routing: PCMRouter struct, callback-based flow, AgentTextBridge interface
    - **Architecture diagram** (lines 97-125): UPDATE to show PCM routing flow: Input PCM → VAD → STT → text → agent → text → TTS → output PCM
  - **ADD new sections**:
    - **VAD Configuration**: Document VoiceVADConfig fields (energy_threshold=0.01, frame_duration_ms=20, silence_duration_ms=300, sample_rate=16000)
    - **Error Codes**: Document -32007 (voice_not_configured), -32008 (voice_rate_limited)
    - **PCM Pipeline**: Document the full audio flow through Bridge, emphasizing agents receive text only

  **Must NOT do**:
  - Do NOT touch architecture.md or bridge-reference.md
  - Do NOT remove accurate WebRTC/TURN/budget/audio sections
  - Do NOT add filler content or verbose descriptions

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Technical documentation rewrite requiring accuracy and clarity
  - **Skills**: []
    - No specialized skills needed — this is pure documentation work

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1 (single task)
  - **Blocks**: T2
  - **Blocked By**: None

  **References**:

  **Source Files to Document** (read these to write accurate descriptions):
  - `bridge/pkg/voice/stt_openai.go` — OpenAI Whisper STT implementation (198 lines)
  - `bridge/pkg/voice/tts_openai.go` — OpenAI TTS implementation (132 lines)
  - `bridge/pkg/voice/vad.go` — Energy-threshold VAD (266 lines)
  - `bridge/pkg/voice/pcm.go` — PCM routing pipeline (338 lines)
  - `bridge/pkg/voice/manager.go` — Voice manager, CreatePCMRouter (lines 513-540)
  - `bridge/pkg/voice/errors.go` — Voice error codes (13 lines)
  - `bridge/pkg/config/config.go` — VoiceVADConfig fields (search for "VoiceVAD" or "VAD")

  **Existing Doc to Rewrite**:
  - `doc/voice-stack.md` — Current 274-line file

  **Accurate Reference Docs** (DON'T modify, use as style guide):
  - `doc/architecture.md` lines 153-168 — Voice Pipeline section (already accurate)
  - `doc/bridge-reference.md` lines 356-371 — Voice methods + error codes (already accurate)

  **WHY Each Reference Matters**:
  - Source files: The executor MUST read the actual implementations to write accurate descriptions — no guessing
  - voice-stack.md: The file being rewritten — executor needs the full current content
  - architecture.md/bridge-reference.md: Style reference — match the level of detail and accuracy

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: No stale claims remain in voice-stack.md
    Tool: Bash
    Preconditions: voice-stack.md has been rewritten
    Steps:
      1. grep -i "interface only\|no implementation\|not implemented\|no provider\|no concrete" doc/voice-stack.md
      2. Verify ZERO matches
    Expected Result: Zero matches for any stale claim
    Failure Indicators: Any match found
    Evidence: .sisyphus/evidence/task-1-no-stale-claims.txt

  Scenario: All new source files are documented
    Tool: Bash
    Preconditions: voice-stack.md has been rewritten
    Steps:
      1. grep "stt_openai" doc/voice-stack.md
      2. grep "tts_openai" doc/voice-stack.md
      3. grep -E "vad\.go|energy.threshold" doc/voice-stack.md
      4. grep -E "pcm\.go|PCMRouter|PCM routing" doc/voice-stack.md
    Expected Result: All 4 source files referenced in the doc
    Failure Indicators: Any source file not mentioned
    Evidence: .sisyphus/evidence/task-1-file-refs.txt

  Scenario: Architecture and bridge-reference docs NOT modified
    Tool: Bash
    Preconditions: Task complete
    Steps:
      1. git diff --name-only HEAD~1 | grep -v voice-stack.md
      2. Verify zero matches (only voice-stack.md should be modified)
    Expected Result: Only voice-stack.md in the diff
    Failure Indicators: architecture.md or bridge-reference.md in the diff
    Evidence: .sisyphus/evidence/task-1-no-scope-creep.txt

  Scenario: Error codes documented
    Tool: Bash
    Steps:
      1. grep -E "\-32007|\-32008" doc/voice-stack.md
    Expected Result: Both error codes mentioned
    Failure Indicators: Missing error code
    Evidence: .sisyphus/evidence/task-1-error-codes.txt
  ```

  **Commit**: YES
  - Message: `docs(voice): rewrite voice-stack.md to reflect v1.0.0 implemented providers`
  - Files: `doc/voice-stack.md`
  - Pre-commit: `grep -ic "interface only\|no provider" doc/voice-stack.md` (expect: 0)

- [x] 2. Verify Documentation Accuracy

  **What to do**:
  - Run all QA scenarios from T1
  - Cross-reference voice-stack.md claims against actual source files
  - Verify architecture.md and bridge-reference.md were NOT modified
  - Verify file count in /doc/ is ≤ 8 (excluding ArmorChat.md)
  - Check that preserved sections (WebRTC, TURN, budget) are intact

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple verification — grep, diff, file count
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (after T1)
  - **Blocks**: None
  - **Blocked By**: T1

  **Acceptance Criteria**:

  ```
  Scenario: All QA scenarios pass
    Tool: Bash
    Steps:
      1. grep -ic "interface only\|no provider\|not implemented" doc/voice-stack.md → expect 0
      2. grep -c "stt_openai\|tts_openai\|vad\.go\|pcm\.go" doc/voice-stack.md → expect ≥ 4
      3. git diff --name-only HEAD~1 → expect only voice-stack.md
      4. grep -c "\-32007\|\-32008" doc/voice-stack.md → expect ≥ 2
      5. ls doc/*.md | grep -v ArmorChat.md | wc -l → expect ≤ 8
    Expected Result: All checks pass
    Failure Indicators: Any check fails
    Evidence: .sisyphus/evidence/task-2-verification.txt
  ```

  **Commit**: NO (verification only, no code changes)

---

## Commit Strategy

- **T1**: `docs(voice): rewrite voice-stack.md to reflect v1.0.0 implemented providers` — doc/voice-stack.md

---

## Success Criteria

### Verification Commands
```bash
grep -ic "interface only\|no provider\|not implemented" doc/voice-stack.md  # Expected: 0
grep -c "stt_openai\|tts_openai" doc/voice-stack.md                         # Expected: ≥ 2
grep -c "\-32007\|\-32008" doc/voice-stack.md                                # Expected: ≥ 2
git diff --name-only HEAD~1                                                   # Expected: doc/voice-stack.md only
ls doc/*.md | grep -v ArmorChat.md | wc -l                                   # Expected: ≤ 8
```

### Final Checklist
- [ ] Zero stale "interface only" / "no provider" claims
- [ ] All new source files (stt_openai.go, tts_openai.go, vad.go, pcm.go) documented
- [ ] Error codes (-32007, -32008) documented
- [ ] PCM routing flow diagram included
- [ ] VAD configuration documented
- [ ] architecture.md and bridge-reference.md NOT modified
- [ ] WebRTC/TURN/budget/audio sections preserved intact
