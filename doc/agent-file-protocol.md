# Agent File Protocol

> Backward communication channel from container to Bridge.
>
> Containers write status and events to files in a bind-mounted state
> directory. The Bridge reads these files to observe agent progress
> without requiring RPC polling or WebSocket connections from inside the
> container.

## Design Goals

- **No outbound network from container** — containers run with `NetworkMode: none`; files are the only backward channel.
- **Atomic reads** — the Bridge never sees a half-written status file.
- **Append-only events** — `agent_events.jsonl` grows monotonically; the Bridge tails incrementally.
- **Bounded size** — both files have a 10 MB soft cap; containers truncate or stop writing gracefully.

---

## File 1: `agent_status.json`

A single JSON object representing the **current** agent state. Overwritten
atomically on every state change.

### Write Requirement: Atomic Rename

```
1. Write full JSON to  agent_status.json.tmp
2. os.Rename(tmp, "agent_status.json")
```

`os.Rename` is atomic on the same filesystem (POSIX), so the Bridge
never observes a partial file.

### Schema

| Field       | Type               | Required | Description |
|-------------|--------------------|----------|-------------|
| `agent_id`  | `string`           | yes      | Unique agent identifier (e.g. `"agent-7f2a"`). |
| `state`     | `string` (enum)    | yes      | One of the 11 agent states (see below). |
| `timestamp` | `integer`          | yes      | Unix epoch **milliseconds** when the state was written. |
| `message`   | `string`           | no       | Human-readable status message (e.g. `"Filling credit card form"`). |
| `metadata`  | `object`           | no       | Additional context (see metadata fields below). |

### State Enum (11 values)

Defined in `bridge/pkg/agent/state.go`:

| Value                  | Category         | Description |
|------------------------|------------------|-------------|
| `IDLE`                 | inactive         | Not performing any task. |
| `INITIALIZING`         | active           | Starting up, loading skills. |
| `BROWSING`             | active           | Navigating to a URL or waiting for page load. |
| `FORM_FILLING`         | active           | Filling form fields. |
| `AWAITING_CAPTCHA`     | terminal         | Needs human CAPTCHA solving. |
| `AWAITING_2FA`         | terminal         | Needs a 2FA code from the user. |
| `AWAITING_APPROVAL`    | terminal         | Waiting for BlindFill approval. |
| `PROCESSING_PAYMENT`   | active           | Submitting a payment. |
| `ERROR`                | inactive         | Recoverable error encountered. |
| `COMPLETE`             | inactive         | Task finished successfully. |
| `OFFLINE`              | terminal         | Agent container not reachable. |

**Terminal states** require external user action to leave: `AWAITING_CAPTCHA`, `AWAITING_2FA`, `AWAITING_APPROVAL`, `OFFLINE`.

**Active states** indicate work in progress: `BROWSING`, `FORM_FILLING`, `INITIALIZING`, `PROCESSING_PAYMENT`.

### Metadata Object

All fields optional. Defined in `bridge/pkg/agent/state.go` `StatusMetadata`:

| Field             | Type       | Description |
|-------------------|------------|-------------|
| `url`             | `string`   | Current page URL (while browsing). |
| `step`            | `string`   | Current step indicator (e.g. `"2/5"`). |
| `progress`        | `integer`  | Percentage 0–100. |
| `error`           | `string`   | Error details (when state is `ERROR`). |
| `task_id`         | `string`   | Current task identifier. |
| `task_type`       | `string`   | Task type (e.g. `"flight_booking"`). |
| `fields_requested`| `string[]` | PII fields being requested (for `AWAITING_APPROVAL`). |
| `workflow_id`     | `string`   | Associated workflow instance ID. |
| `inferred_from`   | `string`   | Source of state inference: `"cdp"`, `"workflow"`, or `"command"`. |

### Example

```json
{
  "agent_id": "agent-7f2a",
  "state": "FORM_FILLING",
  "timestamp": 1717261234567,
  "message": "Filling payment form",
  "metadata": {
    "url": "https://booking.example.com/checkout",
    "step": "3/5",
    "progress": 60,
    "task_id": "task-abc123",
    "fields_requested": ["credit_card_number", "cvv", "expiry"]
  }
}
```

---

## File 2: `agent_events.jsonl`

A **JSONL** (JSON Lines) file — one JSON object per line. Each line is an
execution event appended by the container. The Bridge tails this file
incrementally (500 ms polling) to stream progress to ArmorChat.

### Write Requirement: PIPE_BUF Line Limit

Each line (including the trailing `\n`) **must not exceed 4096 bytes**
(`PIPE_BUF` on Linux). This guarantees safe reads from pipes and FIFOs
without partial-line corruption.

Truncation strategy (from `container/openclaw/events.py`):

1. Truncate `detail` field, add `{"_truncated": true, "_original_size": N}`.
2. If still over 4096 bytes, truncate `name` to 64 characters.
3. Last resort: drop `detail` entirely (empty object `{}`).

### Soft Cap: 10 MB

The Bridge stops tailing when the file exceeds **10 MB** and returns
`ErrEventLogExceeded`. The container is **not killed** — it finishes
normally. After task completion, the state directory (including the
event log) is purged.

### Schema

Each line is a JSON object with these fields:

| Field          | Type              | Required | Description |
|----------------|-------------------|----------|-------------|
| `seq`          | `integer`         | yes      | Monotonically increasing sequence number (1-based). |
| `type`         | `string` (enum)   | yes      | Event type (see below). |
| `name`         | `string`          | yes      | Event name — usually the target path or a label. |
| `ts_ms`        | `integer`         | yes      | Elapsed milliseconds since task start (monotonic clock). |
| `detail`       | `object`          | no       | Key-value context for the event. |
| `duration_ms`  | `integer`         | no       | Duration of the operation in milliseconds. |

### Event Types (11 values)

Defined in `container/openclaw/events.py` `EventType`:

| Type              | Description |
|-------------------|-------------|
| `step`            | Generic task step completed. |
| `file_read`       | File was read. `detail`: `{"lines": N, "size_bytes": N}` |
| `file_write`      | File was written. `detail`: `{"changes": N, "size_bytes": N}` |
| `file_delete`     | File was deleted. |
| `command_run`     | Shell command executed. `detail`: `{"exit_code": N, "truncated": bool}` |
| `observation`     | Agent observation or insight. |
| `blocker`         | Blocking issue. `detail`: `{"blocker_type": str, "suggestion": str, "field": str}` |
| `error`           | Error encountered. |
| `artifact`        | Artifact produced. `detail`: `{"path": str, "mime_type": str, "size_bytes": N}` |
| `progress`        | Progress update. `detail`: `{"percent": N}` |
| `checkpoint`      | Named checkpoint reached. |

### Example File

```jsonl
# Agent execution events
{"seq":1,"type":"step","name":"Navigate to booking page","ts_ms":0,"detail":{}}
{"seq":2,"type":"file_read","name":"/data/preferences.json","ts_ms":120,"detail":{"lines":42,"size_bytes":1024}}
{"seq":3,"type":"step","name":"Fill passenger details","ts_ms":3400,"detail":{}}
{"seq":4,"type":"command_run","name":"curl -s https://api.example.com/availability","ts_ms":5100,"detail":{"exit_code":0,"truncated":false},"duration_ms":1700}
{"seq":5,"type":"observation","name":"Found 3 available flights","ts_ms":7200,"detail":{"count":3}}
{"seq":6,"type":"blocker","name":"Credit card approval required","ts_ms":8900,"detail":{"blocker_type":"pii","suggestion":"Use BlindFill","field":"credit_card_number"}}
{"seq":7,"type":"progress","name":"","ts_ms":12000,"detail":{"percent":50}}
{"seq":8,"type":"artifact","name":"booking_confirmation","ts_ms":15000,"detail":{"path":"/output/confirmation.pdf","mime_type":"application/pdf","size_bytes":245760}}
{"seq":9,"type":"step","name":"Complete","ts_ms":15200,"detail":{}}
```

---

## File Lifecycle

```
Container start
  |
  +-- Write initial agent_status.json  (state: INITIALIZING)
  +-- Create agent_events.jsonl        (write comment header)
  |
  +-- [task execution loop]
  |     +-- On state change: atomic write agent_status.json
  |     +-- On event:       append line to agent_events.jsonl
  |
  +-- Write final agent_status.json    (state: COMPLETE or ERROR)
  +-- Close agent_events.jsonl
  |
Bridge reads
  |
  +-- Poll agent_status.json           (read full file, parse JSON)
  +-- Tail agent_events.jsonl          (incremental read via byte offset)
  |
  +-- After task complete: purge state directory
```

## Cross-References

| Component | File | Role |
|-----------|------|------|
| Event writer | `container/openclaw/events.py` | Python `EventEmitter` — writes `_events.jsonl` with PIPE_BUF enforcement |
| Event reader | `bridge/pkg/secretary/event_reader.go` | Go `EventReader` — incremental tail with 10 MB soft cap |
| Agent states | `bridge/pkg/agent/state.go` | Go `AgentStatus` type — 11 states + valid transitions + `StatusMetadata` |
| Matrix events | `bridge/pkg/agent/state.go` | Go `StatusEvent` — Matrix event type `com.armorclaw.agent.status` |
