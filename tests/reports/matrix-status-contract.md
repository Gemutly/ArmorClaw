# matrix.status RPC Contract

## Method
`matrix.status` — Returns Matrix Conduit connection status.

**Auth required**: No (public method, returns same result with or without `Authorization` header)

## Request

Standard JSON-RPC 2.0:
```json
{"jsonrpc":"2.0","id":1,"method":"matrix.status"}
```

## Response Fields

| Field | Type | Always Present | Description |
|-------|------|----------------|-------------|
| `enabled` | bool | Yes | Whether the Matrix adapter is configured in the bridge |
| `connected` | bool | Yes | Whether the Matrix adapter is initialized (not necessarily sync-connected) |
| `logged_in` | bool | Yes | Whether the bridge Matrix user is authenticated with the homeserver |
| `homeserver` | string | Yes | The Matrix homeserver URL (e.g., `http://localhost:6167`) |
| `user_id` | string | No (omitempty) | The Matrix user ID of the bridge (e.g., `@bridge:5.183.11.149`) |
| `last_sync` | string | No (omitempty) | Timestamp of last successful sync (if available) |
| `error` | string | No (omitempty) | Error message if something is wrong |

## Valid States

### State 1: Operating Normally
```json
{
  "enabled": true,
  "connected": true,
  "logged_in": true,
  "homeserver": "http://localhost:6167",
  "user_id": "@bridge:5.183.11.149"
}
```
**Meaning**: Matrix adapter is configured, initialized, authenticated, and connected to the homeserver.

### State 2: Configured but Not Logged In
```json
{
  "enabled": true,
  "connected": true,
  "logged_in": false,
  "homeserver": "http://127.0.0.1:6167",
  "error": "not logged in"
}
```
**Meaning**: Matrix adapter exists and is initialized, but the bridge user has not authenticated. The `error` field provides the reason.

### State 3: Not Configured (Adapter Missing)
```json
{
  "enabled": false,
  "connected": false,
  "error": "matrix adapter not configured"
}
```
**Meaning**: The Matrix adapter was not initialized at bridge startup. No homeserver URL or user ID available.

### Important Notes

- `connected: true` does **not** mean WebSocket sync is active — it means the adapter object is initialized in memory. Check `logged_in` for authentication state.
- The `connected` field is hardcoded to `true` when the adapter exists (see source reference below). It reflects adapter initialization, not live connection state.
- `user_id` is only populated when `logged_in: true` (omitempty).
- `last_sync` is defined in the struct but may not be populated by the current handler implementation.

## Source References

| Reference | File | Lines |
|-----------|------|-------|
| Handler implementation | `bridge/pkg/rpc/server.go` | 480-502 |
| Response struct | `bridge/pkg/rpc/server.go` | 139-147 |
| Handler registration | `bridge/pkg/rpc/server.go` | 1265 |
| Unit tests | `bridge/pkg/rpc/matrix_handler_test.go` | 47-147 |

## Handler Logic (Pseudocode)

```
handleMatrixStatus():
  if matrix adapter is nil:
    return {enabled: false, connected: false, error: "matrix adapter not configured"}
  
  result = {
    enabled: true,
    connected: true,  // always true when adapter exists
    logged_in: adapter.IsLoggedIn(),
    homeserver: adapter.GetHomeserver(),
    user_id: adapter.GetUserID()
  }
  
  if not adapter.IsLoggedIn():
    result.error = "not logged in"
  
  return result
```

## Live Evidence

- **VPS IP**: 5.183.11.149
- **Bridge version**: 4.6.0
- **Captured**: 2026-05-15T23:01:53Z
- **Response**: `{"enabled":true,"connected":true,"logged_in":true,"homeserver":"http://localhost:6167","user_id":"@bridge:5.183.11.149"}`
- **Evidence file**: `.sisyphus/evidence/pbcp-07/matrix-status-response.json`
