# Task 2: Jetski CDP Event Access Validation

**Date**: 2026-05-03
**Status**: COMPLETE
**Verdict**: `cdp_event_stream_exists: false`

## 1. RPC API Inventory (`jetski/internal/rpc/rpc.go`)

The RPC server runs on port **9223** and registers exactly **4 core endpoints + conditional approval endpoints**:

| Endpoint | Method | Parameters | Return Type | Description |
|----------|--------|------------|-------------|-------------|
| `/rpc/status` | GET | none | `{"active_sessions": int, "engine_health": string}` | Session count + health string |
| `/rpc/session/create` | POST | none | `{"id": string}` | Creates `session-N` ID, increments counter |
| `/rpc/session/close` | POST | `{"id": string}` | `{"status": "closed"}` | Removes session from map |
| `/rpc/health` | GET | none | `{"status": "healthy", "uptime": float64}` | Uptime in seconds |

**Conditional (if approval client != nil):**
| Endpoint | Registered via | Purpose |
|----------|---------------|---------|
| Approval handlers | `approval.RegisterApprovalHandlers(mux, ac)` | HITL approval flow |

### Key Finding: No Event Subscription Endpoint

**None of these endpoints return CDP events or provide a streaming/event subscription mechanism.** The RPC server is a simple HTTP JSON API with request-response semantics only.

## 2. CDP Proxy Architecture (`jetski/internal/cdp/proxy.go`)

### How CDP Events Flow

The proxy is a **bidirectional WebSocket pipe** between client (agent) and engine (Lightpanda):

```
Client (Agent) <--WebSocket--> Jetski Proxy <--WebSocket--> Engine (Lightpanda)
     Port 9222                           Port 9333
```

### `forwardToEngine()` (client → engine, lines 249-320)

1. Reads message from `clientConn`
2. Unmarshals to `CDPMessage` struct
3. **PII scrubbing**: `p.ScrubPII(data)` — regex replacement of SSN, CC, email, password
4. **PII scanning**: `p.piiScanner.ScanJSONMessage()` — logs findings
5. **Message recording**: `p.recorder(msg.Method, msg.Params)` — writes to Sonar buffer
6. **Method routing**: `p.router.Route(msg.Method)` — translates Input.* commands via 3-tier fallback
7. **Approval gate**: `p.checkApproval(&msg)` — blocks Input.insertText (PII) and Page.navigate
8. Writes to `engineConn`

### `forwardToClient()` (engine → client, lines 322-365)

1. Reads message from `engineConn`
2. **Message recording**: `p.recorder(msg.Method, msg.Params)` — writes to Sonar buffer
3. **Session ID extraction**: `p.extractSessionID(data)` — extracts `sessionId` from responses
4. Writes to `clientConn`

### CDP Message Format (`CDPMessage` struct, line 36-42)

```go
type CDPMessage struct {
    ID     int             `json:"id,omitempty"`      // Request ID (commands only)
    Method string          `json:"method,omitempty"`  // e.g. "Page.frameNavigated"
    Params json.RawMessage `json:"params,omitempty"`  // Event parameters
    Result json.RawMessage `json:"result,omitempty"`  // Command response
    Error  *CDPError       `json:"error,omitempty"`   // Error response
}
```

**Events** (engine → client) have `Method` + `Params`, no `ID`.
**Commands** (client → engine) have `ID` + `Method` + `Params`.
**Responses** (engine → client) have `ID` + `Result` or `ID` + `Error`.

## 3. Sonar Telemetry (`jetski/internal/sonar/`)

CDP events ARE captured — but only into an **in-memory circular buffer**:

```go
// CDPFrame (buffer.go:10-15)
type CDPFrame struct {
    Timestamp time.Time       `json:"timestamp"`
    Method    string          `json:"method"`
    Params    json.RawMessage `json:"params"`
    SessionID string          `json:"session_id"`
}
```

- **Buffer capacity**: 1000 frames (set in `main.go:74`)
- **Buffer type**: Circular — oldest frames evicted when full
- **Access methods**: `GetLastN(n)`, `GetAll()`, `Count()`
- **Consumer**: Only `WreckageReport` (black box flight recorder for failures)
- **No RPC exposure**: The Sonar buffer is NOT exposed via any RPC endpoint

### How Recording Is Wired (`main.go:86-88`)

```go
cdpProxy.SetRecorder(func(method string, params json.RawMessage) {
    sonar.RecordFrame(sonarBuf, method, params, cdpProxy.GetSessionID())
})
```

The `MessageRecorder` callback type is `func(method string, params json.RawMessage)` (proxy.go:50).

## 4. Bridge's Expected CDP Events (`bridge/pkg/agent/state_inference.go`)

The bridge's `InferAgentState()` function consumes `[]CDPEvent`:

```go
type CDPEvent struct {
    Method string
    Params map[string]interface{}
}
```

**Recognized event types:**

| CDP Method | Inferred Agent Status | Notes |
|-----------|----------------------|-------|
| `Page.frameNavigated` | `StatusBrowsing` | Page navigation detected |
| `DOM.focus` (input element) | `StatusFormFilling` | Checks nodeName/type for INPUT/TEXTAREA/SELECT |
| `Runtime.executionContextCreated` | `StatusInitializing` | Only if current state is Idle or Offline |
| `Inspector.detached` | No transition | Connection drop — maintain current state |
| `Inspector.targetCrashed` | No transition | Connection drop — maintain current state |

**Type mismatch**: Bridge expects `Params map[string]interface{}` but Jetski stores `json.RawMessage`. Conversion needed.

## 5. Method Router (`jetski/internal/cdp/router.go`)

The router handles **commands** (client → engine), not events:

| Domain | Default Action | Notes |
|--------|---------------|-------|
| Page | Passthrough | |
| Runtime | Translate | 3-tier fallback (CSS → XPath → JS) |
| Input | Passthrough | Individual handlers for mouse/key/text |
| Network | Passthrough | |
| DOM | Passthrough | |
| Target | Passthrough | |
| Browser | Passthrough | |
| Emulation | Passthrough | |
| Fetch | Passthrough | |
| Security | Passthrough | |
| Performance | Passthrough | |
| Schema | Passthrough | |

**The router does NOT intercept or emit events.** It only transforms outbound commands.

## 6. Conclusion

### `cdp_event_stream_exists: false`

Jetski does **NOT** expose CDP events via its RPC API (port 9223). Here's why:

1. **RPC server** has only 4 endpoints (status, session/create, session/close, health) — none return CDP events
2. **CDP proxy** is a pure WebSocket pipe — it transparently forwards events but does not expose them via HTTP
3. **Sonar buffer** captures events in-memory (1000-frame circular buffer) but is only used for `WreckageReport` post-mortem analysis, not exposed via RPC
4. **No WebSocket subscription endpoint** exists on the RPC server for external consumers

### What Changes Are Needed for Task 2.5

To enable CDP event streaming from Jetski to Bridge's `InferAgentState()`:

| Change | File | Function | Description |
|--------|------|----------|-------------|
| Add RPC endpoint | `jetski/internal/rpc/rpc.go` | New handler | `/rpc/events/stream` — WebSocket or SSE endpoint |
| Expose Sonar buffer | `jetski/internal/rpc/rpc.go` | Handler logic | Read from Sonar `CircularBuffer`, stream new frames |
| Or: add subscription callback | `jetski/internal/cdp/proxy.go` | `SetRecorder` pattern | Add a second recorder for RPC subscribers |
| Wire RPC to proxy | `jetski/cmd/observer/main.go` | `main()` | Pass Sonar buffer reference to RPC server |
| Add event subscriber RPC | `jetski/internal/rpc/rpc.go` | New struct field | `Server` needs access to Sonar buffer |
| Type conversion | Bridge-side | `InferAgentState()` call site | Convert `json.RawMessage` → `map[string]interface{}` |

**Estimated effort**: ~100-150 lines of Go across rpc.go and main.go. Medium complexity (needs concurrent-safe subscription pattern, likely channel-based).

**Recommended approach**: Add a `/rpc/events/stream` WebSocket endpoint to the RPC server that subscribes to a new broadcast channel in the Proxy. The Proxy's existing `recorder` callback already fires on every CDP message in both directions — add a second fan-out to a pub/sub channel.
