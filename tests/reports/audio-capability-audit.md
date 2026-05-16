# Audio Capability Audit Report

**Date**: 2026-05-16
**Scope**: BEATO v1.2.1 → v1.3 activation audit
**Status**: AUDIT ONLY — no code changes

## Summary

The ArmorClaw voice stack is **architecturally complete but runtime-disabled**. The `voice.Manager` is initialized in code but commented out in `cmd/bridge/main.go`, leaving `voiceMgr` as nil at runtime. This is by design — voice is planned for full activation in v1.4.

## Existing Components

### Bridge Voice Package (`bridge/pkg/voice/`)

| File | Purpose | Status |
|------|---------|--------|
| `manager.go` | Core voice call manager — WebRTC, Matrix, budget, security | Complete, disabled |
| `budget.go` | Cost tracking for STT/TTS API calls | Complete, tested |
| `security.go` | Security enforcer + audit + TTL manager | Complete, tested |
| `pcm.go` | PCM audio routing (raw audio between WebRTC and processors) | Complete |
| `matrix.go` | Matrix call signaling integration | Complete, disabled tests |
| `errors.go` | Voice-specific error types | Complete |
| `e2e_test.go` | End-to-end voice tests (requires live STT/TTS) | Disabled |
| `e2e_providers_test.go` | Provider-specific integration tests | Disabled |

### WebRTC Package (`bridge/pkg/webrtc/`)

- `SessionManager`, `TokenManager`, `Engine` — WebRTC session lifecycle
- ICE candidate handling, DTLS fingerprint validation
- Used by voice manager for real-time audio transport

### TURN Package (`bridge/pkg/turn/`)

- TURN relay manager for NAT traversal
- Credential provisioning for voice calls

### Test Infrastructure

| File | Purpose |
|------|---------|
| `tests/test-voice-stack.sh` | Voice stack budget and STT/TTS/VAD validation |
| `tests/docker-compose.voice.yml` | Docker Compose for voice services |

## STT/TTS/VAD Capability Assessment

### Speech-to-Text (STT)

- **Architecture**: Provider-abstracted interface in voice manager
- **Providers**: Stub implementations for Whisper API, Google Speech API
- **Gap**: No live provider credentials configured; calls return placeholder text
- **Activation effort**: LOW — configure API key in bridge config.toml

### Text-to-Speech (TTS)

- **Architecture**: Provider-abstracted interface in voice manager
- **Providers**: Stub implementations for Google TTS, ElevenLabs
- **Gap**: Same as STT — no live provider configured
- **Activation effort**: LOW — configure API key in bridge config.toml

### Voice Activity Detection (VAD)

- **Architecture**: PCM router with silence detection in `pcm.go`
- **Implementation**: Energy-based VAD (threshold configurable)
- **Gap**: Not connected to STT pipeline (would need webhook/callback)
- **Activation effort**: MEDIUM — requires PCM → VAD → STT pipeline wiring

## RPC Methods (Registered but No-Op)

Three voice RPC methods are registered and respond with "voice not enabled":

- `voice.start_session` — returns voice-not-enabled
- `voice.stop_session` — returns voice-not-enabled
- `voice.status` — returns `{active: false, sessions: []}`

## Recommendations for v1.4 Activation

### Priority 1: Enable Voice Manager

1. Uncomment `voiceMgr` initialization in `cmd/bridge/main.go`
2. Add `[voice]` section to `config.toml` with provider credentials
3. Enable `voice.start_session` RPC to actually create WebRTC sessions

### Priority 2: Wire STT Pipeline

1. Configure Whisper API (or local Whisper.cpp) as STT provider
2. Connect PCM router output to STT input
3. Stream transcription results to Matrix room as `m.notice` events

### Priority 3: Wire TTS Pipeline

1. Configure TTS provider (Google TTS recommended for latency)
2. Accept text input via Matrix messages
3. Convert to PCM and inject into WebRTC audio stream

### Priority 4: VAD Optimization

1. Replace energy-based VAD with Silero VAD model
2. Use VAD to gate STT calls (only transcribe when speech detected)
3. Reduces API costs by ~60% during silence periods

## BEATO Audio Pillar Score: 5/25

- Architecture exists: ✅ (5 pts)
- Runtime enabled: ❌ (0 pts)
- STT functional: ❌ (0 pts)
- TTS functional: ❌ (0 pts)
- VAD functional: ❌ (0 pts)

**Note**: Per BEATO plan, Audio is audit-only in v1.3. Full activation deferred to v1.4.
