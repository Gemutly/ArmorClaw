# Voice Stack

> Part of the [ArmorClaw System Documentation](armorclaw.md)

## Current State

The voice stack has a complete implementation including WebRTC, budget enforcement, security policies, TURN traversal, and **OpenAI cloud speech providers** (STT via Whisper, TTS via tts-1) with an energy-threshold VAD and PCM routing pipeline. All components are gated behind the `VoicePipeline` feature flag. When enabled (`feature_voice_pipeline = "cloud"` in config or `ARMORCLAW_FEATURE_VOICE_PIPELINE=cloud` env var), the voice manager initializes and 3 RPC methods (`voice.start_session`, `voice.stop_session`, `voice.status`) become available. Audio terminates at the Bridge — agent containers receive text only.

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
| Voice Manager | Implemented, gated by feature flag | `bridge/pkg/voice/manager.go` |
| Voice Manager Wiring | Wired in `main.go:1988-2103` | `bridge/cmd/bridge/main.go` |
| Voice RPC Handlers | 3 methods, flag-gated | `bridge/pkg/rpc/server.go` |
| **STT Provider (OpenAI Whisper)** | **Implemented** | `bridge/pkg/voice/stt_openai.go` |
| **TTS Provider (OpenAI tts-1)** | **Implemented** | `bridge/pkg/voice/tts_openai.go` |
| **VAD (Energy Threshold)** | **Implemented** | `bridge/pkg/voice/vad.go` |
| **PCM Routing Pipeline** | **Implemented** | `bridge/pkg/voice/pcm.go` |
| Voice Error Codes | Implemented | `bridge/pkg/voice/errors.go` |
| E2E Provider Tests | Implemented (mocked HTTP) | `bridge/pkg/voice/e2e_providers_test.go` |
| VAD + PCM Tests | Implemented | `bridge/pkg/voice/vad_pcm_test.go` |

### What Is Not Yet Implemented

| Component | Status | Gap |
|-----------|--------|-----|
| Local STT/TTS | Not implemented | v1.0 uses cloud only (OpenAI). Local ONNX providers deferred. |
| Matrix Call Signaling Wiring | Implemented, unwired | `MatrixManager` exists but is not wired into the top-level `Manager`. |

### Runtime Reality

The voice manager initialization in `main.go` is gated by the `VoicePipeline` feature flag. When `feature_voice_pipeline = "cloud"` is set in config (or `ARMORCLAW_FEATURE_VOICE_PIPELINE=cloud` env var), the full initialization runs.

The `Manager.CreatePCMRouter()` method wires the PCM routing pipeline for each call session:

```go
router := voiceMgr.CreatePCMRouter(callID, sttProvider, ttsProvider, agentBridge)
```

The pipeline routes audio entirely within the Bridge process:
```
Input PCM → VAD (energy threshold) → STT (OpenAI Whisper) → text → agent → text → TTS (OpenAI tts-1) → output PCM
```

Agent containers run with `NetworkMode: none` — they receive transcribed text and return text responses. No audio enters or leaves the agent container.

When the flag is off (default `VoicePipeline = "off"`), voice RPC methods return `-32601`. When enabled but the voice manager fails to start, methods return `-32007` (`voice_not_configured`).

### E2E Test Coverage

`bridge/pkg/voice/e2e_providers_test.go` (1102 lines) covers the full voice pipeline with mocked HTTP servers:
- STT lifecycle (PCM→WAV→transcription, empty audio, API errors)
- TTS lifecycle (text→speech, empty text, API errors)
- Rate-limit handling (HTTP 429/402 → error code `-32008`)
- VAD event detection (speech start/end/silence)
- PCM routing end-to-end (VAD→STT→agent→TTS pipeline)
- Feature flag off (methods return `-32601`)

The older `e2e_test.go` tests HTTP sidecar health checks under `ARMORCLAW_E2E=1` and is unrelated to the OpenAI provider implementation.

## Overview

The voice stack is designed to let ArmorClaw agents make and receive real-time phone calls through the mobile app. It handles everything from audio encoding to NAT traversal entirely inside the Bridge. Audio never touches the agent container directly; the Bridge encodes, decodes, and forwards it between the WebRTC peer and the agent's stdin/stdout or gRPC stream.

The stack is built on four packages: `audio` for PCM and Opus processing, `voice` for call budget enforcement and speech services, `webrtc` for peer connection management and session lifecycle, and `turn` for NAT traversal with ephemeral credentials.

## Architecture

Audio flows through a fixed path from the caller's phone to the agent and back. The Bridge sits in the middle, handling codec work, VAD gating, STT/TTS via OpenAI, budget checks, and signaling. TURN relays handle NAT punching when direct connections are not possible.

```
                          ArmorClaw Voice Call Flow (v1.0.0)

  ┌──────────┐       ┌───────────┐       ┌──────────────────────────────────────────────┐
  │  Phone   │       │   TURN    │       │                Bridge (VPS)                  │
  │ ArmorChat│       │  Relay    │       │                                              │
  │          │       │           │       │  ┌─────────┐   ┌───────┐   ┌───────┐        │
  │ Mic ─────┼──SDP──┼───────────┼──RTP──┼─▶│ webrtc  │──▶│ audio │──▶│ VAD   │        │
  │          │       │  (NAT     │       │  │ engine  │   │ pcm   │   │(energy│        │
  │          │       │ traversal)│       │  │ session │   │ 16kHz  │   │thresh)│        │
  │ Speaker ◀┼──SDP──┼───────────┼◀─RTP──┼──│         │◀──│       │◀──│       │        │
  └──────────┘       └───────────┘       │  └─────────┘   └───┬───┘   └───┬───┘        │
                                          │                    │            │             │
                                          │                    ▼            │             │
                                          │             ┌──────────┐      │             │
                                          │             │ STT      │      │             │
                                          │             │ OpenAI   │◀─────┘             │
                                          │             │ Whisper  │                    │
                                          │             └────┬─────┘                    │
                                          │                  │ text                      │
                                          │                  ▼                           │
                                          │           ┌──────────────┐                  │
                                          │           │    Agent     │                  │
                                          │           │  Container   │                  │
                                          │           │(text in/out) │                  │
                                          │           └──────┬───────┘                  │
                                          │                  │ text                      │
                                          │                  ▼                           │
                                          │             ┌──────────┐     ┌───────┐      │
                                          │             │ TTS      │     │ voice │      │
                                          │             │ OpenAI   │     │ budget│      │
                                          │             │ tts-1    │     │ check │      │
                                          │             └────┬─────┘     └───────┘      │
                                          │                  │ PCM                      │
                                          │                  ▼                          │
                                          │             ┌──────────┐                    │
                                          │             │ Output   │                    │
                                          │             │ PCM →    │──▶ RTP → Phone     │
                                          │             │ WebRTC   │                    │
                                          │             └──────────┘                    │
                                          └──────────────────────────────────────────────┘

  PCM Pipeline (Bridge-local, 16kHz):
    Input PCM → VAD → STT → text → agent → text → TTS → output PCM

  Agent boundary:
    Agent receives TEXT only (NetworkMode: none)
    Agent returns TEXT only

  Error codes:
    -32007: voice pipeline not configured
    -32008: voice rate limit / quota exceeded
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

Call budget tracking, security enforcement, speech services, VAD, and PCM routing. Prevents runaway token costs, enforces time limits, and provides the full audio pipeline from microphone input to speaker output.

| File | Purpose |
|------|---------|
| `stt_openai.go` | `OpenAISTTProvider` — OpenAI Whisper STT. Accepts PCM audio (16kHz), converts to WAV, calls `/v1/audio/transcriptions`, returns transcription with confidence, duration, word count, and latency. Handles 429/402 rate limits with error code `-32008`. |
| `tts_openai.go` | `OpenAITTSProvider` — OpenAI TTS. Accepts text, calls `/v1/audio/speech` with tts-1 model (default voice: alloy), returns MP3 audio data. Handles rate limits and quota errors with `-32008`. |
| `vad.go` | `EnergyThresholdVAD` — Energy-threshold voice activity detection. Computes RMS per frame, emits `speech_start`/`speech_end`/`silence` events. Frame-based processing with configurable threshold, frame duration, and silence duration. Also provides `ComputeRMS`, `BytesToInt16Samples`, `GenerateSilence`, and `GenerateTone` utilities. |
| `pcm.go` | `PCMRouter` — Routes audio through the full pipeline: VAD→STT→agent→TTS. State machine (idle→listening→processing). `AgentTextBridge` interface for agent text I/O. `StreamReader`/`StreamWriter` for io.Reader/Writer integration. Callbacks: `OnSpeechStart`, `OnSpeechEnd`, `OnAgentResponse`, `OnOutputPCM`, `OnError`. |
| `errors.go` | Voice error types and codes. `VoiceError` with code/message/cause. `-32007` (not configured), `-32008` (rate limited). `IsVoiceNotConfigured()`, `IsVoiceRateLimit()` type guards. |
| `manager.go` | `Manager` orchestrates sessions, budget, security, and WebRTC. `CreatePCMRouter()` wires the PCM pipeline per call. `MatrixManager` field is `nil` (Matrix signaling unwired). |
| `budget.go` | `BudgetTracker` manages per-session limits, `VoiceSessionTracker` tracks token usage (input + output) and duration, `TokenUsage` counters, `Config` with default/duration limits and warning thresholds, background `EnforceLimits` loop (30 s interval), security logging for budget events |
| `matrix.go` | `MatrixManager` for Matrix call signaling (invite, answer, hangup, reject, ICE candidates). Implemented but not wired into the top-level `Manager`. |
| `security.go` | `SecurityEnforcer` (concurrent call limits, blocklists, rate limiting), `SecurityAudit` (call auditing, violation tracking, reports), `TTLManager` (session expiry enforcement). All fully implemented. |
| `e2e_providers_test.go` | Full pipeline E2E tests with mocked OpenAI HTTP. Covers STT/TTS lifecycle, rate limits (429→-32008), VAD events, PCM routing, feature flag off. |
| `vad_pcm_test.go` | VAD and PCM unit tests. Energy threshold detection, frame processing, silence generation, tone generation, PCM routing state machine. |

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

## Speech Providers

### OpenAI Whisper STT (`stt_openai.go`)

The STT provider converts raw PCM audio to text via OpenAI's Whisper API.

- **Constructor**: `NewOpenAISTTProvider(cfg OpenAISTTConfig)` — requires API key, optional base URL and model
- **Environment**: Reads `OPEN_AI_KEY` or `OPENAI_API_KEY`
- **Model**: `whisper-1` (default)
- **Endpoint**: `POST /v1/audio/transcriptions`
- **Audio path**: PCM (16kHz, mono, 16-bit) → WAV (44-byte header) → multipart upload
- **Response**: `TranscriptionResult` with text, confidence, duration, word count, latency
- **Rate limits**: HTTP 429/402 → `VoiceError` with code `-32008`
- **Timeout**: 60 seconds

### OpenAI TTS (`tts_openai.go`)

The TTS provider converts agent text responses to audio via OpenAI's TTS API.

- **Constructor**: `NewOpenAITTSProvider(cfg OpenAITTSConfig)` — requires API key, optional base URL, model, and voice
- **Environment**: Reads `OPEN_AI_KEY` or `OPENAI_API_KEY`
- **Model**: `tts-1` (default)
- **Voice**: `alloy` (default), also supports `echo`, `fable`, `onyx`, `nova`, `shimmer`
- **Endpoint**: `POST /v1/audio/speech`
- **Response**: `SynthesisResult` with audio data (MP3), text length, latency
- **Rate limits**: HTTP 429/402/403 (with quota/billing keywords) → `VoiceError` with code `-32008`
- **Timeout**: 120 seconds

### Energy-Threshold VAD (`vad.go`)

Voice activity detection using RMS energy calculation per audio frame.

- **Constructor**: `NewEnergyThresholdVAD(config EnergyVADConfig)`
- **Algorithm**: Computes RMS (root mean square) of int16 samples per frame. Compares against configurable energy threshold.
- **Events**: `VADEventSpeechStart` (energy crosses threshold), `VADEventSpeechEnd` (energy stays below threshold for N consecutive frames), `VADEventSilence`
- **State machine**: Silence → Speech (on threshold cross) → Silence (on silence timeout)
- **Processing**: `ProcessPCM([]byte)` splits raw PCM into frames, returns VAD events. Accumulates partial frames across calls.
- **Utilities**: `ComputeRMS`, `BytesToInt16Samples`, `Int16SamplesToBytes`, `GenerateSilence`, `GenerateTone`

### PCM Router (`pcm.go`)

Routes audio through the full pipeline within the Bridge process.

- **Constructor**: `NewPCMRouter(config, stt, tts, agent)` — takes STT provider, TTS provider, and `AgentTextBridge`
- **`AgentTextBridge`** interface: `SendText(ctx, text) (string, error)` — the agent's text I/O
- **State machine**: `routerIdle` → `routerListening` (VAD speech start) → `routerProcessing` (VAD speech end, STT running) → `routerIdle` (TTS output complete)
- **Flow**: `ProcessInputPCM(pcmData)` → VAD events → buffer speech → STT → agent.SendText → TTS → `OnOutputPCM` callback
- **Callbacks**: `OnSpeechStart`, `OnSpeechEnd`, `OnAgentResponse`, `OnOutputPCM`, `OnError`
- **Stream I/O**: `StreamReader` (io.Reader for output PCM), `StreamWriter` (io.Writer for input PCM)
- **VAD bypass**: If `VADEnabled=false`, passes raw PCM directly to STT without VAD gating

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
| `OPEN_AI_KEY` / `OPENAI_API_KEY` | OpenAI API key for STT/TTS | _(none)_ |

### VAD Configuration

Configured under `[voice.vad]` in TOML or via environment variables:

| Setting | TOML Key | Env Variable | Default | Description |
|---------|----------|--------------|---------|-------------|
| Energy Threshold | `energy_threshold` | `ARMORCLAW_VOICE_VAD_ENERGY_THRESHOLD` | `0.01` | RMS energy level to trigger speech detection. Lower = more sensitive. |
| Frame Duration | `frame_duration_ms` | `ARMORCLAW_VOICE_VAD_FRAME_DURATION_MS` | `20` | Duration of each analysis frame in milliseconds |
| Silence Duration | `silence_duration_ms` | `ARMORCLAW_VOICE_VAD_SILENCE_DURATION_MS` | `300` | Consecutive silence frames before declaring speech end |
| Sample Rate | `sample_rate` | `ARMORCLAW_VOICE_VAD_SAMPLE_RATE` | `16000` | PCM sample rate (Hz). Must match STT input expectation. |

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

### Voice Error Codes

| Code | Name | Trigger |
|------|------|---------|
| `-32007` | `voice_not_configured` | Voice pipeline is not configured (flag off or API key missing) |
| `-32008` | `voice_rate_limited` | OpenAI API rate limit (429) or quota exceeded (402/403 with billing keywords) |

## Integration Points

### Matrix Rooms

Signaling uses the existing Matrix E2EE infrastructure. The Bridge sends SDP offers and answers as Matrix events, and ICE candidates are trickled through the same encrypted channel. Each voice session is bound to a Matrix room ID, and the `RequireMembership` config flag (on by default) ensures only room members can initiate calls.

### Budget System

The `voice.BudgetTracker` integrates with the Bridge's security logger. Every session start, end, budget warning, and enforcement action is logged as a security event. Token usage from the AI model's input (speech-to-text) and output (text-to-speech) is tracked per call and checked against limits every 30 seconds.

### Agent Runtime

The PCM router sends **text only** to agent containers via the `AgentTextBridge` interface. The agent receives transcribed text from STT and returns text responses for TTS. No audio enters or leaves the agent container — containers run with `NetworkMode: none` and have no network access.

### TURN Infrastructure

The `turn.Manager` generates ephemeral credentials scoped to individual sessions. The WebRTC engine calls `SetTURNServersWithManager` before each peer connection is created, getting fresh TURN URLs and credentials that expire with the session. This avoids static credentials and limits the blast radius of any leak.
