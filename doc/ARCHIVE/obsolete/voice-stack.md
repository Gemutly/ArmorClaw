# Voice Stack

> Part of the [ArmorClaw System Documentation](armorclaw.md)

## Current State

The voice stack has a complete infrastructure layer (WebRTC, budget enforcement, security policies, TURN traversal) but **zero concrete speech providers**. STT, TTS, and VAD services define interfaces only. No AI provider backends exist. The voice manager initialization is commented out in `bridge/cmd/bridge/main.go`.

### What Exists

| Component | Status | File |
|-----------|--------|------|
| WebRTC Engine | Implemented | `bridge/pkg/webrtc/engine.go` |
| WebRTC Sessions | Implemented | `bridge/pkg/webrtc/session.go` |
| TURN Manager | Implemented | `bridge/pkg/turn/turn.go` |
| Budget Tracker | Implemented | `bridge/pkg/voice/budget.go` |
| Security Enforcer | Implemented | `bridge/pkg/voice/security.go` |
| TTL Manager | Implemented | `bridge/pkg/voice/security.go` |
| Security Audit | Implemented | `bridge/pkg/voice/security.go` |
| Matrix Call Signaling | Implemented (unwired) | `bridge/pkg/voice/matrix.go` |
| Voice Manager | Implemented (commented out) | `bridge/pkg/voice/manager.go` |

### What Is Missing

| Component | Status | Gap |
|-----------|--------|-----|
| STT Provider | Interface only | `voice.Transcriber` has no implementation |
| TTS Provider | Interface only | `voice.Synthesizer` has no implementation |
| VAD Provider | Interface only | `voice.SpeechDetector` has no implementation |
| Audio Pipeline | Not implemented | No PCM routing between WebRTC and agent |
| Voice Manager Wiring | Commented out | `main.go` lines 1988-2103 |

### Runtime Reality

The voice import and all initialization code in `main.go` is wrapped in a block comment:

```go
// TODO: Voice package needs refactoring - uncomment when fixed
// "github.com/armorclaw/bridge/pkg/voice"

/*
    voiceConfig := voice.DefaultConfig()
    ...
    voiceMgr := voice.NewManager(...)
    if err := voiceMgr.Start(); err != nil { ... }
*/
```

Even if uncommented, the voice manager sets its internal `voiceMgr` field to `nil`, so Matrix call signaling would not function.

### Interface Discrepancy

Two packages define overlapping voice interfaces with different method signatures:

**`bridge/pkg/interfaces/voice.go`** (canonical result types):
- `VoiceManager.HandleMatrixCallEvent(roomID, eventID, senderID string, event interface{}) error`
- `Transcriber.Transcribe(ctx, audioData []byte) (*TranscriptionResult, error)`
- `Synthesizer.Synthesize(ctx, text string) (*SynthesisResult, error)`
- `SpeechDetector.DetectSpeech(ctx, audioData []byte) (*VADResult, error)`

**`bridge/pkg/voice/` package** (service wrappers):
- `Manager.HandleMatrixCallEvent(roomID, eventID, senderID string, event *CallEvent) error`
- `Transcriber.Transcribe(ctx, audioData []byte) (*interfaces.TranscriptionResult, error)`
- `Synthesizer.Synthesize(ctx, text string) (*interfaces.SynthesisResult, error)`
- `SpeechDetector.DetectSpeech(ctx, audioData []byte) (*interfaces.VADResult, error)`

The `VoiceManager` signatures differ: `interface{}` vs `*CallEvent`. The `Manager` struct does not satisfy the `interfaces.VoiceManager` interface. The `Transcriber`, `Synthesizer`, and `SpeechDetector` interfaces are duplicated between packages, though they share the same method signatures and return types from `interfaces`.

### E2E Test Expectations

`bridge/pkg/voice/e2e_test.go` expects HTTP sidecar services that do not exist:
- VAD at `http://localhost:8001/health`
- STT at `http://localhost:8002/health`
- TTS at `http://localhost:8003/health`

These tests run only when `ARMORCLAW_E2E=1` is set. They will fail until concrete providers are deployed.

## Overview

The voice stack is designed to let ArmorClaw agents make and receive real-time phone calls through the mobile app. It handles everything from audio encoding to NAT traversal entirely inside the Bridge. Audio never touches the agent container directly; the Bridge encodes, decodes, and forwards it between the WebRTC peer and the agent's stdin/stdout or gRPC stream.

The stack is built on four packages: `audio` for PCM and Opus processing, `voice` for call budget enforcement and speech services, `webrtc` for peer connection management and session lifecycle, and `turn` for NAT traversal with ephemeral credentials.

## Architecture

Audio flows through a fixed path from the caller's phone to the agent and back. The Bridge sits in the middle, handling codec work, budget checks, and signaling. TURN relays handle NAT punching when direct connections are not possible.

```
                          ArmorClaw Voice Call Flow

  ┌──────────┐       ┌───────────┐       ┌─────────────────────────────────────┐
  │  Phone   │       │   TURN    │       │            Bridge (VPS)             │
  │ ArmorChat│       │  Relay    │       │                                     │
  │          │       │           │       │  ┌─────────┐  ┌───────┐  ┌───────┐ │
  │ Mic ─────┼──SDP──┼───────────┼──RTP──┼─▶│ webrtc  │─▶│ audio │─▶│ voice │ │
  │          │       │  (NAT     │       │  │ engine  │  │ pcm   │  │ budget│ │
  │ Speaker ◀┼──SDP──┼──traversal│◀─RTP──┼──│ session │◀─│ opus  │◀─│ check │ │
  │          │       │  only)    │       │  └────┬────┘  └───┬───┘  └───┬───┘ │
  └──────────┘       └───────────┘       │       │            │           │     │
                                          │       │            │           │     │
                                          │       ▼            ▼           │     │
                                          │  ┌──────────────────────┐     │     │
                                          │  │    Agent Container   │◀────┘     │
                                          │  │    (AI runtime)      │           │
                                          │  └──────────────────────┘           │
                                          └─────────────────────────────────────┘

  Signaling path (SDP offer/answer, ICE candidates):
    Phone ◀── Matrix E2EE room ──▶ Bridge

  Media path (audio RTP):
    Phone ◀── TURN relay (or direct) ──▶ Bridge

  Budget path:
    Bridge tracks tokens + duration per session, enforces hard stop
```

The signaling layer uses Matrix rooms for SDP exchange and ICE candidate trickling. The media layer runs over RTP through TURN or direct UDP. Budget enforcement runs as a background goroutine that checks every 30 seconds.

## Key Packages

### `bridge/pkg/audio/`

PCM processing and Opus codec support. All audio I/O lives here, not in agent containers.

| File | Purpose |
|------|---------|
| `pcm.go` | `AudioConfig` defaults (48 kHz, mono, 16-bit, 20 ms frames), `AudioStream` bidirectional frame channels, `AudioPipeline` per-session stream pairs, `PCMMixer` for combining multiple streams, `PCMEncoder` with sample rate conversion, `AudioBuffer` circular ring buffer, `WebRTCTrackReader`/`Writer` for Pion track I/O |
| `opus.go` | `OpusEncoder`/`OpusDecoder` for PCM-to-Opus conversion, `OpusConfig` with bitrate/complexity/FEC/DTX tuning, `RTPOpusPacketizer`/`RTPDepacketizer` for RTP framing, `AudioStats` frame/packet/jitter tracking, `AudioLevelMeter` dBFS measurement, `OpusPayloader`/`Depayloader` for Pion integration |

Default audio config: 48 kHz sample rate, mono, 16-bit depth, 960 samples per frame (20 ms), 10-frame buffer (200 ms).

### `bridge/pkg/voice/`

Call budget tracking, security enforcement, and speech service wrappers. Prevents runaway token costs, enforces time limits, and defines abstraction over AI provider speech APIs.

| File | Purpose |
|------|---------|
| `budget.go` | `BudgetTracker` manages per-session limits, `VoiceSessionTracker` tracks token usage (input + output) and duration, `TokenUsage` counters, `Config` with default/duration limits and warning thresholds, background `EnforceLimits` loop (30 s interval), security logging for budget events |
| `stt_service.go` | `STTService` wraps `Transcriber` interface for speech-to-text. **Interface only, no provider.** |
| `tts_service.go` | `TTSService` wraps `Synthesizer` interface for text-to-speech synthesis. **Interface only, no provider.** |
| `vad_service.go` | `VADService` wraps `SpeechDetector` interface for voice activity detection. **Interface only, no provider.** |
| `manager.go` | `Manager` orchestrates sessions, budget, security, and WebRTC. Implemented but commented out in `main.go`. `MatrixManager` field is `nil`. |
| `matrix.go` | `MatrixManager` for Matrix call signaling (invite, answer, hangup, reject, ICE candidates). Implemented but never wired into the top-level `Manager`. |
| `security.go` | `SecurityEnforcer` (concurrent call limits, blocklists, rate limiting), `SecurityAudit` (call auditing, violation tracking, reports), `TTLManager` (session expiry enforcement). All fully implemented. |
| `e2e_test.go` | Health check tests for STT/TTS/VAD HTTP sidecars. Expects services at ports 8001/8002/8003 that do not exist. Skipped unless `ARMORCLAW_E2E=1`. |

Key defaults:
- Token limit: 100,000 per call
- Duration limit: 30 minutes per call
- Warning threshold: 80% of limit
- Hard stop: enabled by default

The tracker emits `voice_budget_warning` security events when usage crosses the warning threshold and `voice_budget_enforced` when hard-stopping a call.

### `bridge/pkg/webrtc/`

WebRTC peer connection management and session lifecycle. This is where Matrix rooms, agent containers, TURN allocations, and budget sessions are bound together.

| File | Purpose |
|------|---------|
| `engine.go` | `Engine` creates and manages `PeerConnectionWrapper` instances, registers Opus codec, handles SDP offer/answer exchange, writes audio to local tracks, reads RTP from remote tracks, integrates with `turn.Manager` for ephemeral credentials |
| `session.go` | `SessionManager` handles the full lifecycle of `Session` objects (pending, active, ended, failed, expired), TTL enforcement with 1-minute cleanup interval, binds session to container ID, Matrix room ID, TURN credentials, and budget session |
| `signaling.go` | WebRTC signaling |
| `token.go` | Token management |

Session states: `pending` (created, not connected) to `active` (media flowing) to `ended` (normal close), `failed` (error), or `expired` (TTL hit).

Default TTL: 10 minutes. Max TTL: 1 hour. Session IDs use `sess_` prefix with 16 hex chars from `crypto/rand`.

### `bridge/pkg/turn/`

NAT traversal with ephemeral per-session TURN credentials. No static passwords.

| File | Purpose |
|------|---------|
| `turn.go` | `Manager` generates time-limited TURN credentials using HMAC-SHA1, `TURNCredentials` with `<expiry>:<session_id>` username format, `ICEGatherer` for host candidate gathering (reflexive/relay gathering return empty, delegated to WebRTC stack), `ICECandidate` parsing and serialization, `STUNMessage` builder/parser for STUN binding requests, `CreateICEServers` helper for Pion integration |

Credential format: username is `<unix_expiry>:<session_id>`, password is `base64(HMAC-SHA1(secret, username))`. Credentials are scoped to a single session and auto-expire. A cleanup goroutine runs every minute to purge stale entries.

### Speech Services (`bridge/pkg/voice/`)

Three service wrappers define interfaces for AI provider speech APIs. **No concrete providers exist.** Each source file carries an `INTERFACE-ONLY` comment.

#### STTService (stt_service.go)
- Wraps `Transcriber` interface for speech-to-text
- `NewSTTService(client Transcriber)` creates service
- `Transcribe(ctx, audioData []byte)` returns `*TranscriptionResult, error`
- Uses `slog.Logger` for structured logging

#### TTSService (tts_service.go)
- Wraps `Synthesizer` interface for text-to-speech synthesis
- `NewTTSService(client Synthesizer)` creates service
- `Synthesize(ctx, text string)` returns `*SynthesisResult, error`

#### VADService (vad_service.go)
- Wraps `SpeechDetector` interface for voice activity detection
- `NewVADService(client SpeechDetector)` creates service
- `DetectSpeech(ctx, audioData []byte)` returns `*VADResult, error`

#### Design Pattern
All three services follow the same interface+wrapper pattern:
1. Define a provider interface (`Transcriber`, `Synthesizer`, `SpeechDetector`)
2. Service struct holds the provider client and a logger
3. Constructor takes the provider, returns the service
4. Methods delegate to provider with error passthrough

This allows swapping AI providers (OpenAI Whisper, Google STT, etc.) without changing callers. But no providers are plugged in yet.

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TURN_SECRET` | **Required.** Shared secret for HMAC-SHA1 credential generation. Bridge refuses to start if empty. | _(none, must be set)_ |
| `TURN_HOST` | TURN relay hostname or IP | `matrix.armorclaw.com` |
| `TURN_PORT` | TURN relay port | `3478` |
| `TURN_PROTOCOL` | Transport protocol: `udp`, `tcp`, or `tls` | `udp` |
| `TURN_REALM` | Authentication realm | `armorclaw` |
| `TURN_DEFAULT_TTL` | Credential lifetime | `10m` |
| `TURN_MAX_TTL` | Maximum credential lifetime | `1h` |

### Budget Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `DefaultTokenLimit` | 100,000 | Max tokens per call |
| `DefaultDurationLimit` | 30 min | Max call duration |
| `WarningThreshold` | 0.8 (80%) | Emit warning at this usage fraction |
| `HardStop` | true | Terminate call when limit is hit |
| `DefaultLifetime` | 10 min | Default session TTL |
| `MaxLifetime` | 1 hour | Maximum session TTL |

### Audio Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `SampleRate` | 48,000 Hz | Opus standard sample rate |
| `Channels` | 1 (mono) | Voice calls use mono |
| `BitDepth` | 16 bit | PCM16 format |
| `FrameSize` | 960 samples | 20 ms frames at 48 kHz |
| `BufferSize` | 10 frames | 200 ms jitter buffer |
| `Bitrate` | 64,000 bps | Opus target bitrate |
| `Complexity` | 5 (0-10) | Encoder complexity |
| `FEC` | enabled | Forward error correction |
| `DTX` | disabled | Discontinuous transmission |

## Integration Points

### Matrix Rooms

Signaling uses the existing Matrix E2EE infrastructure. The Bridge sends SDP offers and answers as Matrix events, and ICE candidates are trickled through the same encrypted channel. Each voice session is bound to a Matrix room ID, and the `RequireMembership` config flag (on by default) ensures only room members can initiate calls.

### Budget System

The `voice.BudgetTracker` integrates with the Bridge's security logger. Every session start, end, budget warning, and enforcement action is logged as a security event. Token usage from the AI model's input (speech-to-text) and output (text-to-speech) is tracked per call and checked against limits every 30 seconds.

### Agent Runtime

Audio frames flow between the Bridge and agent containers through the `AudioPipeline`. The pipeline creates a `StreamPair` (inbound + outbound) for each session. The agent container receives decoded PCM audio and produces PCM audio back, without needing to know about WebRTC, Opus, or TURN. The Bridge handles all codec and protocol work.

### TURN Infrastructure

The `turn.Manager` generates ephemeral credentials scoped to individual sessions. The WebRTC engine calls `SetTURNServersWithManager` before each peer connection is created, getting fresh TURN URLs and credentials that expire with the session. This avoids static credentials and limits the blast radius of any leak.
