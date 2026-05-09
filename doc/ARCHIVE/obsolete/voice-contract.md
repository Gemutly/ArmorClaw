# Voice Event Contract

> Part of the [ArmorClaw System Documentation](armorclaw.md) | See also: [Voice Stack](voice-stack.md)

This document defines the Matrix event types used by the ArmorClaw voice pipeline for WebRTC call signaling and session lifecycle. Every event is a Matrix custom event sent over the existing E2EE room channel.

## Event Types

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

## Event Schemas

### `voice.session.created`

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

Default TTL is 10 minutes, maximum 1 hour. See [Voice Stack](voice-stack.md#bridge/pkg/webrtc/) for session lifecycle details.

---

### `voice.session.offer`

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

### `voice.session.answer`

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

### `voice.session.ice_candidate`

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

### `voice.session.ended`

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

### `voice.error`

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

## Event Flow

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

## Cross-References

- **Audio pipeline details**: [Voice Stack](voice-stack.md) covers the PCM routing, VAD, STT/TTS provider configuration, and codec parameters
- **WebRTC session lifecycle**: `bridge/pkg/webrtc/session.go` manages session state transitions and TTL enforcement
- **TURN credential generation**: `bridge/pkg/turn/turn.go` generates ephemeral HMAC-SHA1 credentials
- **Budget enforcement**: `bridge/pkg/voice/budget.go` tracks token usage and duration per session
- **Error types and codes**: `bridge/pkg/voice/errors.go` defines `VoiceError`, `VoicePrereqReason`, and `CheckVoicePrereqs()`
- **Matrix call signaling**: `bridge/pkg/voice/matrix.go` implements the `MatrixManager` that processes these events
