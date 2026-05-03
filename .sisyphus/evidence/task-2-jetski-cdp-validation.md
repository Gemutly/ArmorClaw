# Task 2: Jetski CDP Event Access Validation

**Date:** 2026-05-03
**Status:** COMPLETE
**Gate Result:** `cdp_event_stream_exists: false`

---

## 1. Jetski RPC API — All Registered Endpoints

File: `jetski/internal/rpc/rpc.go`

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/rpc/status` | GET | Returns `active_sessions` count and `engine_health` |
| `/rpc/session/create` | POST | Creates a session, returns `{"id": "session-N"}` |
| `/rpc/session/close` | POST | Closes a session by ID |
| `/rpc/health` | GET | Returns `status` and `uptime_seconds` |
| `/rpc/approval/*` | POST | Conditional — registered only if `ApprovalClient` is non-nil |

**Key finding:** There is NO `/rpc/events`, `/rpc/cdp/stream`, `/rpc/subscribe`, or any WebSocket/SSE streaming endpoint in the RPC server. All endpoints are simple HTTP request/response.

---

## 2. CDP Proxy — Event Interception Points

File: `jetski/internal/cdp/proxy.go`

The Proxy is a **bidirectional WebSocket pipe** between agent (clientConn) and browser engine (engineConn). It intercepts CDP messages in both directions:

### Inbound (Client → Engine): `forwardToEngine()`
- Reads from `clientConn`, forwards to `engineConn`
- Applies PII scrubbing, PII scanning, method routing, and approval checks
- **Records events** via `MessageRecorder` callback (line 289-291):
  ```go
  if p.recorder != nil && msg.Method != "" {
      p.recorder(msg.Method, msg.Params)
  }
  ```

### Outbound (Engine → Client): `forwardToClient()`
- Reads from `engineConn`, forwards to `clientConn`
- **Also records events** via the same `MessageRecorder` callback (line 345-349):
  ```go
  if p.recorder != nil && messageType == websocket.TextMessage {
      var msg CDPMessage
      if json.Unmarshal(data, &msg) == nil && msg.Method != "" {
          p.recorder(msg.Method, msg.Params)
      }
  }
  ```

### MessageRecorder Type
```go
type MessageRecorder func(method string, params json.RawMessage)
```
Set via `proxy.SetRecorder(fn)`. This is an **in-process callback only** — there is no external interface to subscribe to recorded events.

---

## 3. How the Recorder is Currently Used

File: `jetski/cmd/observer/main.go` (line 86-88)

```go
cdpProxy.SetRecorder(func(method string, params json.RawMessage) {
    sonar.RecordFrame(sonarBuf, method, params, cdpProxy.GetSessionID())
})
```

The recorder feeds into the **Sonar telemetry buffer** (`CircularBuffer`) for post-mortem `WreckageReport` generation. This is a **flight data recorder pattern** — data is written to an in-memory ring buffer for crash analysis, NOT streamed to external consumers.

---

## 4. Bridge Expected CDPEvent Types

File: `bridge/pkg/agent/state_inference.go`

```go
type CDPEvent struct {
    Method string
    Params map[string]interface{}
}
```

`InferAgentState()` processes these CDP methods:

| CDP Method | Maps To AgentStatus | Notes |
|------------|---------------------|-------|
| `Page.frameNavigated` | `StatusBrowsing` | Page navigation detected |
| `DOM.focus` (on input elements) | `StatusFormFilling` | Checks nodeName: INPUT/TEXTAREA/SELECT |
| `Runtime.executionContextCreated` | `StatusInitializing` | Only from Idle/Offline states |
| `Inspector.detached` | No transition | Jetski restart — maintain state |
| `Inspector.targetCrashed` | No transition | Browser crash — maintain state |

The function signature expects a **batched slice**: `InferAgentState(cdpEvents []CDPEvent, ...)`

---

## 5. External Access Mechanism Assessment

### Question: Can the Bridge subscribe to CDP events from Jetski on port 9223?

**Answer: NO.**

The Jetski RPC server on port 9223 (`/rpc/*`) provides only:
1. Session management (create/close)
2. Health/status polling
3. Approval flow (conditional)

There is **no streaming mechanism** — no WebSocket event subscription, no SSE endpoint, no gRPC streaming, no pub/sub. CDP events flow through the proxy only within a single WebSocket connection (agent ↔ Jetski ↔ browser). The `MessageRecorder` is an in-process Go callback with no network exposure.

### What exists vs what's needed:

| Component | Exists | Exposed Externally |
|-----------|--------|-------------------|
| CDP event interception (recorder) | YES — `MessageRecorder` callback | NO — in-process only |
| Sonar telemetry buffer | YES — `CircularBuffer` for crash reports | NO — in-memory ring buffer |
| RPC status/health endpoints | YES — HTTP on :9223 | YES — but no event data |
| WebSocket CDP proxy | YES — on :9222 (for agents) | YES — but this is the raw CDP pipe, not a curated event stream |

### Why raw CDP WebSocket (port 9222) doesn't solve this:
- Port 9222 is a **single-connection** WebSocket proxy — the Bridge can't open a second connection
- Even if it could, the Bridge would need to parse ALL CDP traffic and filter for the 4-5 event types it cares about
- This duplicates the PII scrubbing and introduces a second consumer on a single-connection pipe

---

## 6. Changes Needed to Enable CDP Event Streaming

To make CDP events available to the Bridge, Jetski needs:

### Option A: RPC Event Streaming Endpoint (Recommended)
Add a new endpoint to `jetski/internal/rpc/rpc.go`:
- `GET /rpc/events/stream` — SSE (Server-Sent Events) endpoint
- Register the `MessageRecorder` in `main.go` to fan-out events to SSE subscribers
- Files to modify:
  - `jetski/internal/rpc/rpc.go` — add SSE handler and subscriber registry
  - `jetski/cmd/observer/main.go` — wire recorder to RPC server subscriber fan-out
  - New: `jetski/internal/rpc/eventbus.go` — subscriber registry and fan-out logic

### Option B: Shared Event Buffer via RPC Polling
Add a ring buffer accessible via RPC:
- `GET /rpc/events?since=<seq>` — returns buffered CDP events since sequence number
- Simpler to implement but higher latency (polling-based)
- Files to modify:
  - `jetski/internal/rpc/rpc.go` — add events endpoint
  - `jetski/cmd/observer/main.go` — wire recorder to shared buffer

### Data Format (Bridge-Ready):
```json
{
  "seq": 42,
  "session_id": "session-1",
  "method": "Page.frameNavigated",
  "params": {"frame": {"id": "main"}, "url": "https://example.com"},
  "timestamp": "2026-05-03T12:00:00Z"
}
```

---

## 7. Available CDP Event Types (from state_inference.go)

Events the Bridge cares about for state inference:

1. **Page.frameNavigated** — navigation events
2. **DOM.focus** — form element focus (with nodeType/nodeName/type params)
3. **Runtime.executionContextCreated** — JS context creation
4. **Inspector.detached** — CDP disconnect
5. **Inspector.targetCrashed** — browser crash

All other CDP events are ignored by the inference engine (maintain current state).

---

## Conclusion

**`cdp_event_stream_exists: false`**

Jetski has internal CDP event interception (via `MessageRecorder`) but no mechanism to expose these events to external consumers. The Bridge's `InferAgentState()` function is fully implemented and tested but **unwired** — it cannot receive CDP events because Jetski has no streaming endpoint.

**Task 2.5 (CDP streaming endpoint) IS needed.** The infrastructure for event capture exists inside Jetski; only the external delivery mechanism is missing.
