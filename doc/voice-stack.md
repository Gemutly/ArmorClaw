# Voice Stack

> Part of the [ArmorClaw System Documentation](armorclaw.md) | See also: [Voice Event Contract](#voice-event-contract) (below)

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

---

## Voice Event Contract

This document defines the Matrix event types used by the ArmorClaw voice pipeline for WebRTC call signaling and session lifecycle. Every event is a Matrix custom event sent over the existing E2EE room channel.

### Event Types

The voice manager emits six event types during a call session. They follow the Matrix `m.call.*` convention already used by the signaling layer in `bridge/pkg/voice/matrix.go`.

| Event | Type | Direction | Purpose |
|-------|------|-----------|---------|
| Session Created | `voice.session.created` | Bridge → Room | A new voice session has been allocated |
| SDP Offer | `voice.session.offer` | Caller → Room | WebRTC SDP offer for peer connection |
| SDP Answer | `voice.session.answer` | Callee → Room | WebRTC SDP answer accepting the call |
| ICE Candidate | `voice.session.ice_candidate` | Either → Room | Trickle ICE candidate for NAT traversal |
| Session Ended | `voice.session.ended` | Either → Room | Call terminated (normal, rejected, expired, or error) |
| Voice Error | `voice.error` | Bridge → Room | Structured error for pipeline failures |

---

### Event Schemas

#### `voice.session.created`

Emitted when the Bridge allocates a new voice session. The session is in `pending` state until the WebRTC peer connection is established.

```json
{
  "type": "voice.session.created",
  "content": {
    "session_id": "sess_a1b2c3d4e5f6a7b8",
    "call_id": "call_1715234567890123456",
    "room_id": "!abc123:armorclaw.com",
    "caller_id": "@user:armorclaw.com",
    "callee_id": "@bridge:armorclaw.com",
    "state": "pending",
    "created_at": 1715234567,
    "expires_at": 1715235167,
    "lifetime_ms": 600000,
    "turn_credentials": {
      "urls": ["turn:matrix.armorclaw.com:3478?transport=udp"],
      "username": "1715235167:sess_a1b2c3d4e5f6a7b8",
      "credential": "base64-hmac-sha1-credential"
    }
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `session_id` | string | yes | Unique session ID (`sess_` prefix, 16 hex chars from `crypto/rand`) |
| `call_id` | string | yes | Matrix call ID (`call_` prefix with nanosecond timestamp) |
| `room_id` | string | yes | Matrix room where signaling occurs |
| `caller_id` | string | yes | Matrix user ID of the caller |
| `callee_id` | string | yes | Matrix user ID of the bridge |
| `state` | string | yes | Session state: `pending`, `active`, `ended`, `failed`, `expired` |
| `created_at` | integer | yes | Unix timestamp of session creation |
| `expires_at` | integer | yes | Unix timestamp when session TTL expires |
| `lifetime_ms` | integer | yes | Session lifetime in milliseconds |
| `turn_credentials` | object | yes | Ephemeral TURN credentials scoped to this session |

Session states transition as follows:

```
pending → active → ended
pending → failed
pending → expired  (TTL)
active  → failed
active  → expired  (TTL)
```

Default TTL is 10 minutes, maximum 1 hour. See [Voice Stack](#bridgewebbrtc) for session lifecycle details.

---

#### `voice.session.offer`

Carries the SDP offer from the caller to establish a WebRTC peer connection. This is the Matrix VoIP signaling equivalent of `m.call.invite`.

```json
{
  "type": "voice.session.offer",
  "content": {
    "session_id": "sess_a1b2c3d4e5f6a7b8",
    "call_id": "call_1715234567890123456",
    "party_id": "@user:armorclaw.com",
    "version": "0",
    "offer": {
      "type": "offer",
      "sdp": "v=0\r\no=- 1234567890 1 IN IP4 0.0.0.0\r\n..."
    },
    "lifetime": 60000,
    "created_at": 1715234567
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `session_id` | string | yes | Session this offer belongs to |
| `call_id` | string | yes | Matrix call ID |
| `party_id` | string | yes | Matrix user ID of the party sending the offer |
| `version` | string | yes | Matrix VoIP version (`"0"`) |
| `offer.type` | string | yes | SDP type, always `"offer"` |
| `offer.sdp` | string | yes | Full SDP payload |
| `lifetime` | integer | yes | Offer validity in milliseconds |
| `created_at` | integer | yes | Unix timestamp of offer creation |

The SDP contains Opus codec parameters (48 kHz, mono, 64 kbps bitrate). The Bridge validates that `party_id` is a room member before processing, enforced by `RequireMembership` (on by default).

---

#### `voice.session.answer`

Carries the SDP answer back from the callee. Accepts the call and completes the WebRTC peer connection setup.

```json
{
  "type": "voice.session.answer",
  "content": {
    "session_id": "sess_a1b2c3d4e5f6a7b8",
    "call_id": "call_1715234567890123456",
    "party_id": "@bridge:armorclaw.com",
    "version": "0",
    "answer": {
      "type": "answer",
      "sdp": "v=0\r\no=- 9876543210 1 IN IP4 0.0.0.0\r\n..."
    }
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `session_id` | string | yes | Session this answer belongs to |
| `call_id` | string | yes | Matrix call ID |
| `party_id` | string | yes | Matrix user ID of the answering party |
| `version` | string | yes | Matrix VoIP version (`"0"`) |
| `answer.type` | string | yes | SDP type, always `"answer"` |
| `answer.sdp` | string | yes | Full SDP answer payload |

The Bridge must verify that the answer comes from the expected party. Once accepted, the session transitions to `active` state and media begins flowing through the PCM pipeline: `Input PCM → VAD → STT → text → agent → text → TTS → output PCM`.

---

#### `voice.session.ice_candidate`

Trickles ICE candidates for NAT traversal. Both parties send candidates as they are discovered. TURN relay candidates use ephemeral credentials generated by `turn.Manager`.

```json
{
  "type": "voice.session.ice_candidate",
  "content": {
    "session_id": "sess_a1b2c3d4e5f6a7b8",
    "call_id": "call_1715234567890123456",
    "party_id": "@user:armorclaw.com",
    "version": "0",
    "candidates": [
      {
        "candidate": "candidate:1 1 udp 2130706431 192.168.1.50 49152 typ host",
        "sdpMLineIndex": 0,
        "sdpMid": "0"
      },
      {
        "candidate": "candidate:2 1 udp 16777215 203.0.113.1 3478 typ relay raddr 0.0.0.0 rport 0",
        "sdpMLineIndex": 0,
        "sdpMid": "0"
      }
    ]
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `session_id` | string | yes | Session these candidates belong to |
| `call_id` | string | yes | Matrix call ID |
| `party_id` | string | yes | Matrix user ID of the party sending candidates |
| `version` | string | yes | Matrix VoIP version (`"0"`) |
| `candidates` | array | yes | List of ICE candidates |
| `candidates[].candidate` | string | yes | SDP-formatted candidate string |
| `candidates[].sdpMLineIndex` | integer | yes | Media line index (0 for audio) |
| `candidates[].sdpMid` | string | yes | Media section ID |

Candidates can include host (direct), srflx (server reflexive), and relay (TURN) types. The Bridge's `turn.Manager` generates ephemeral credentials with format `<unix_expiry>:<session_id>` for the username and `base64(HMAC-SHA1(secret, username))` for the password. These credentials are scoped to a single session and auto-expire.

---

#### `voice.session.ended`

Signals that the voice session has terminated. Covers normal hangup, rejection, expiry, and error cases.

```json
{
  "type": "voice.session.ended",
  "content": {
    "session_id": "sess_a1b2c3d4e5f6a7b8",
    "call_id": "call_1715234567890123456",
    "party_id": "@user:armorclaw.com",
    "version": "0",
    "reason": "hangup",
    "ended_at": 1715234867,
    "duration_ms": 300000,
    "token_usage": {
      "input_tokens": 42500,
      "output_tokens": 18300
    }
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `session_id` | string | yes | Session that ended |
| `call_id` | string | yes | Matrix call ID |
| `party_id` | string | yes | Matrix user ID of the party that ended the call |
| `version` | string | yes | Matrix VoIP version (`"0"`) |
| `reason` | string | yes | One of: `hangup`, `rejected`, `expired`, `failed`, `manager_shutdown`, `ice_timeout` |
| `ended_at` | integer | yes | Unix timestamp of session end |
| `duration_ms` | integer | no | Actual call duration in milliseconds |
| `token_usage` | object | no | Token consumption for the session |

**Reason values:**

| Reason | Description |
|--------|-------------|
| `hangup` | Normal hangup by either party |
| `rejected` | Callee rejected the incoming call |
| `expired` | Session TTL exceeded (default 10 min, max 1 hour) |
| `failed` | Connection error or WebRTC failure |
| `manager_shutdown` | Bridge is shutting down, ending all active calls |
| `ice_timeout` | ICE connection could not be established |

Token usage reflects the STT (input) and TTS (output) consumption tracked by the `BudgetTracker`. The tracker checks limits every 30 seconds and enforces hard stops when configured.

---

#### `voice.error`

Structured error envelope for voice pipeline failures. Used when the Bridge cannot process a voice request or when the pipeline encounters an unrecoverable error.

```json
{
  "type": "voice.error",
  "content": {
    "code": -32007,
    "message": "voice pipeline not configured: feature flag is off or API key is missing",
    "session_id": "sess_a1b2c3d4e5f6a7b8",
    "call_id": "call_1715234567890123456",
    "cause": "TURN_SECRET environment variable is not set",
    "prereq_failures": [
      {
        "reason": "VOICE_PREREQ_TURN_SECRET_MISSING",
        "message": "TURN_SECRET environment variable is not set"
      },
      {
        "reason": "VOICE_PREREQ_OPENAI_KEY_MISSING",
        "message": "OPENAI_API_KEY environment variable is not set"
      }
    ]
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `code` | integer | yes | Error code (see table below) |
| `message` | string | yes | Human-readable error description |
| `session_id` | string | no | Associated session ID if available |
| `call_id` | string | no | Associated call ID if available |
| `cause` | string | no | Underlying cause of the error |
| `prereq_failures` | array | no | List of prerequisite check failures (only for `-32007`) |

**Error codes:**

| Code | Name | Condition |
|------|------|-----------|
| `-32007` | `voice_not_configured` | Voice pipeline is not configured. The `VoicePipeline` feature flag is off, or a required environment variable (`TURN_SECRET`, `OPENAI_API_KEY`) is missing, or the Matrix adapter is not wired. |
| `-32008` | `voice_rate_limited` | OpenAI API returned HTTP 429 (rate limit), 402 (payment required), or 403 with quota/billing keywords. The provider has exhausted its allowance. |

Additionally, when the voice feature flag is completely off (`VoicePipeline = "off"`, the default), voice RPC methods return `-32601` (method not found) rather than `-32007`. Code `-32007` fires only when the flag is enabled but prerequisites are unmet.

**Prerequisite failure reasons** (attached to `-32007` errors):

| Reason | Description |
|--------|-------------|
| `VOICE_PREREQ_TURN_SECRET_MISSING` | `TURN_SECRET` environment variable is not set |
| `VOICE_PREREQ_OPENAI_KEY_MISSING` | `OPENAI_API_KEY` environment variable is not set |
| `VOICE_PREREQ_MATRIX_UNAVAILABLE` | Matrix Conduit homeserver is not reachable |
| `VOICE_PREREQ_MATRIX_UNWIRED` | Matrix adapter is not logged in or not connected |

The `prereq_failures` array provides structured diagnostics so clients can distinguish between configuration gaps (missing API keys) and infrastructure problems (Matrix unreachable). This is populated by `CheckVoicePrereqs()` in `errors.go`.

---

### Event Flow

A typical voice call follows this sequence:

```
Caller                    Matrix Room                    Bridge
  │                          │                             │
  │  voice.session.offer     │                             │
  │─────────────────────────▶│────────────────────────────▶│
  │                          │                             │ allocate session
  │                          │  voice.session.created      │
  │                          │◀────────────────────────────│
  │                          │                             │
  │  voice.session.          │                             │
  │  ice_candidate (×N)      │                             │
  │─────────────────────────▶│────────────────────────────▶│
  │                          │                             │ gather candidates
  │                          │  voice.session.             │
  │                          │  ice_candidate (×N)         │
  │                          │◀────────────────────────────│
  │                          │                             │
  │                          │  voice.session.answer       │
  │                          │◀────────────────────────────│
  │◀─────────────────────────│                             │
  │                          │                             │
  │  ══════════ RTP media flows via TURN/direct ═══════════│
  │  PCM → VAD → STT → agent → TTS → PCM                 │
  │                          │                             │
  │  voice.session.ended     │                             │
  │─────────────────────────▶│────────────────────────────▶│
  │                          │  voice.session.ended        │
  │                          │◀────────────────────────────│
  │◀─────────────────────────│                             │
```

Error path (pipeline not configured):

```
Caller                    Matrix Room                    Bridge
  │                          │                             │
  │  voice.session.offer     │                             │
  │─────────────────────────▶│────────────────────────────▶│
  │                          │                             │ prereq check fails
  │                          │  voice.error                │
  │                          │◀────────────────────────────│
  │◀─────────────────────────│                             │
  │  (code: -32007)          │                             │
```

---

### Cross-References

- **Audio pipeline details**: [Voice Stack](#) covers the PCM routing, VAD, STT/TTS provider configuration, and codec parameters
- **WebRTC session lifecycle**: `bridge/pkg/webrtc/session.go` manages session state transitions and TTL enforcement
- **TURN credential generation**: `bridge/pkg/turn/turn.go` generates ephemeral HMAC-SHA1 credentials
- **Budget enforcement**: `bridge/pkg/voice/budget.go` tracks token usage and duration per session
- **Error types and codes**: `bridge/pkg/voice/errors.go` defines `VoiceError`, `VoicePrereqReason`, and `CheckVoicePrereqs()`
- **Matrix call signaling**: `bridge/pkg/voice/matrix.go` implements the `MatrixManager` that processes these events
