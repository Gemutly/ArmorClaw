# Expose matrix.join_room RPC

## TL;DR

> **Quick Summary**: Wire the existing `MatrixAdapter.JoinRoom()` method through the RPC server as `matrix.join_room`, closing the final matrix-deployment verification gap.
> 
> **Deliverables**:
> - `matrix.join_room` RPC handler in `bridge/pkg/rpc/server.go`
> - Handler registered alongside existing `matrix.*` methods
> - Unit test for the new handler
> - Deploy updated bridge binary to VPS
> - Prove `!status` end-to-end via bridge room
> 
> **Estimated Effort**: Quick (1-2 hours)
> **Parallel Execution**: NO — single sequential task
> **Critical Path**: Wire handler → Build → Deploy → Verify !status

---

## Context

### Original Request
During matrix-deployment verification, the final proof step (!status in bridge room) was blocked because the bridge user has `rooms_count=0` and there is no RPC method to make it join a room.

### Root Cause
- `MatrixAdapter.JoinRoom()` exists at `bridge/internal/adapter/matrix.go:1625` with signature:
  ```go
  func (m *MatrixAdapter) JoinRoom(ctx context.Context, roomIDOrAlias string, viaServers []string, reason string) (string, error)
  ```
- The RPC server at `bridge/pkg/rpc/server.go:732-735` exposes `matrix.status`, `matrix.login`, `matrix.send`, `matrix.receive` — but NOT `matrix.join_room`.
- Conduit does not support admin API, so there's no server-side way to force-join the bridge user.
- Bridge user's access token is in encrypted SQLCipher keystore (can't extract for manual curl).

### Solution
Add a single RPC handler that calls the existing `JoinRoom()` method. This is the smallest possible change.

---

## Work Objectives

### Core Objective
Expose `matrix.join_room` RPC so the bridge can join rooms on command, unblocking the !status end-to-end verification.

### Concrete Deliverables
- `matrix.join_room` RPC handler
- Unit test
- Deployed to VPS
- !status proof captured as evidence

### Definition of Done
- [ ] `matrix.join_room` handler registered in server.go
- [ ] `cd bridge && go test ./pkg/rpc/...` passes
- [ ] `cd bridge && go build ./cmd/bridge` succeeds
- [ ] Bridge deployed to VPS and restarted
- [ ] Bridge user joins room `!IGY2TnBy2gp9GpW__JI0JG0SP61PW0CeGvWCqFUMZCI`
- [ ] `!status` sent and response captured as evidence

### Must Have
- RPC handler follows existing `HandlerFunc` signature pattern
- Error handling for invalid room ID and Matrix API failures
- Handler registered in `registerHandlers()` alongside other `matrix.*` methods

### Must NOT Have (Guardrails)
- Do NOT modify `MatrixAdapter.JoinRoom()` — it already works
- Do NOT add auto-join logic — this is an explicit RPC call only
- Do NOT modify any existing RPC handlers
- Do NOT add authentication to this handler (bridge socket is already local-only)

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (Go: testify)
- **Automated tests**: YES (tests-after — single handler, low risk)
- **Framework**: Go `go test` + testify

### QA Policy
- **Unit test**: `go test` for handler registration and parameter parsing
- **Integration test**: SSH to VPS, call RPC, verify bridge joins room
- **End-to-end**: Send `!status`, capture response

---

## Execution Strategy

```
Step 1: Add handler (code change)
├── Add handleMatrixJoinRoom to server.go
├── Register "matrix.join_room" in registerHandlers()
├── Write test in server_test.go or new handler test file
└── go test + go build

Step 2: Deploy to VPS
├── Cross-compile or build on VPS
├── Replace binary at /opt/armorclaw/armorclaw-bridge
├── Restart armorclaw-bridge.service
└── Verify matrix.status still works

Step 3: Prove !status end-to-end
├── Call matrix.join_room with test room ID
├── Verify bridge user is now in room (rooms_count > 0)
├── Call matrix.send with "!status" message
└── Capture response as evidence
```

---

## TODOs

- [ ] 1. Add matrix.join_room RPC Handler

  **What to do**:
  - Add `handleMatrixJoinRoom` method to `bridge/pkg/rpc/server.go`
  - Follow existing `HandlerFunc` signature: `func(ctx context.Context, req *Request) (interface{}, *ErrorObj)`
  - Unmarshal params: `{"room_id": "!xxx:server", "via_servers": ["server1"], "reason": "optional"}`
  - Call `s.GetMatrixAdapter().JoinRoom(ctx, roomID, viaServers, reason)`
  - Return the room ID string on success
  - Return error on failure (invalid params, Matrix API error)
  - Register in `registerHandlers()` at line ~735 (after `matrix.receive`):
    `"matrix.join_room": s.handleMatrixJoinRoom,`
  - Write test: verify method is registered, verify params unmarshaled correctly
  - Run `cd bridge && go test ./pkg/rpc/... && go build ./cmd/bridge`

  **Must NOT do**:
  - DO NOT modify `MatrixAdapter.JoinRoom()` — call it as-is
  - DO NOT add auto-join or any background logic
  - DO NOT modify existing handlers

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single handler following well-established pattern, ~30 lines of code
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocks**: Task 2
  - **Blocked By**: None

  **References**:

  **Pattern References** (existing code to follow):
  - `bridge/pkg/rpc/server.go:117` — `HandlerFunc` type signature: `func(ctx context.Context, req *Request) (interface{}, *ErrorObj)`
  - `bridge/pkg/rpc/server.go:732-735` — Where to add the new registration (alongside other `matrix.*` methods)
  - `bridge/pkg/rpc/server.go:748` — `GetMatrixAdapter()` method to access the adapter
  - `bridge/pkg/rpc/bridge_handlers.go:99-117` — Parameter unmarshaling pattern with `json.Unmarshal`
  - `bridge/internal/adapter/matrix.go:1625` — `JoinRoom` signature: `(ctx, roomIDOrAlias string, viaServers []string, reason string) (string, error)`

  **WHY Each Reference Matters**:
  - HandlerFunc signature: Every handler MUST match this exact signature
  - Lines 732-735: Insert the new method registration here to keep matrix methods grouped
  - GetMatrixAdapter(): This is how the handler accesses the adapter to call JoinRoom()
  - bridge_handlers.go: Shows the idiomatic json.Unmarshal pattern for request params
  - matrix.go:1625: The target method being called — params must match this signature

  **Acceptance Criteria**:
  - [ ] `"matrix.join_room"` appears in `registerHandlers()` map
  - [ ] `cd bridge && go test -v -run TestMatrixJoinRoom ./pkg/rpc/...` → PASS
  - [ ] `cd bridge && go test ./pkg/rpc/...` → ALL PASS (no regressions)
  - [ ] `cd bridge && go build ./cmd/bridge` → SUCCESS

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: matrix.join_room handler registered correctly
    Tool: Bash (go test)
    Steps:
      1. cd bridge && go test -v -run TestMatrixJoinRoom ./pkg/rpc/...
      2. Verify handler exists in server.handlers map
    Expected Result: PASS — "matrix.join_room" found in handlers
    Evidence: .sisyphus/evidence/matrix-join-room-handler-registered.txt

  Scenario: matrix.join_room rejects missing room_id
    Tool: Bash (go test)
    Steps:
      1. cd bridge && go test -v -run TestMatrixJoinRoomMissingParam ./pkg/rpc/...
      2. Verify error returned when room_id not provided in params
    Expected Result: PASS — error with "room_id required" message
    Evidence: .sisyphus/evidence/matrix-join-room-missing-param.txt
  ```

  **Commit**: YES
  - Message: `feat(rpc): expose matrix.join_room RPC handler`
  - Files: `bridge/pkg/rpc/server.go`, `bridge/pkg/rpc/server_test.go` (or new test file)
  - Pre-commit: `cd bridge && go test ./pkg/rpc/... && go build ./cmd/bridge`

- [ ] 2. Deploy and Verify !status End-to-End

  **What to do**:
  - SSH to VPS: `ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no -i ~/.ssh/openclaw_win root@5.183.11.149`
  - Build bridge binary on VPS (or cross-compile from local and scp)
  - Replace binary: `systemctl stop armorclaw-bridge && cp new-binary /opt/armorclaw/armorclaw-bridge && systemctl start armorclaw-bridge`
  - Verify bridge is running: `systemctl status armorclaw-bridge`
  - Verify matrix.status still works: `printf '{"jsonrpc":"2.0","id":1,"method":"matrix.status"}\n' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
  - Call matrix.join_room: `printf '{"jsonrpc":"2.0","id":2,"method":"matrix.join_room","params":{"room_id":"!IGY2TnBy2gp9GpW__JI0JG0SP61PW0CeGvWCqFUMZCI"}}\n' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
  - Verify rooms_count > 0 (bridge user now in room)
  - Send !status: `printf '{"jsonrpc":"2.0","id":3,"method":"matrix.send","params":{"room_id":"!IGY2TnBy2gp9GpW__JI0JG0SP61PW0CeGvWCqFUMZCI","message":"!status"}}\n' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`
  - Capture response as evidence
  - Clean up test artifacts: deactivate `test_probe_user`, delete dummy `store_key` entry `test-key-001`

  **Must NOT do**:
  - DO NOT skip the build verification step
  - DO NOT deploy without running tests first

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Deployment + verification, follows established SSH pattern
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocks**: None (final task)
  - **Blocked By**: Task 1

  **References**:

  **VPS Connection**:
  - SSH: `ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no -i ~/.ssh/openclaw_win root@5.183.11.149`
  - Bridge binary: `/opt/armorclaw/armorclaw-bridge`
  - Bridge service: `armorclaw-bridge.service`
  - RPC socket: `/run/armorclaw/bridge.sock`

  **Test Room**:
  - Room ID: `!IGY2TnBy2gp9GpW__JI0JG0SP61PW0CeGvWCqFUMZCI`
  - Created by: `armorclaw_admin`
  - Join rule: public

  **WHY Each Reference Matters**:
  - SSH command: The exact command that works for this VPS (key-specific)
  - Binary location: Where the compiled binary must be placed
  - Room ID: The pre-created test room the bridge user needs to join

  **Acceptance Criteria**:
  - [ ] Bridge binary replaced and service running
  - [ ] `matrix.status` returns `{connected:true, logged_in:true}`
  - [ ] `matrix.join_room` returns success (room ID)
  - [ ] `!status` sent to room and response captured
  - [ ] Test artifacts cleaned up

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: matrix.join_room successfully joins bridge to room
    Tool: Bash (SSH + socat)
    Preconditions: Bridge running, task 1 deployed
    Steps:
      1. SSH to VPS
      2. Call matrix.join_room with room_id "!IGY2TnBy2gp9GpW__JI0JG0SP61PW0CeGvWCqFUMZCI"
      3. Verify response contains room ID
      4. Call matrix.status and check rooms_count > 0
    Expected Result: Room ID returned, rooms_count > 0
    Evidence: .sisyphus/evidence/matrix-join-room-success.txt

  Scenario: !status returns expected bridge status response
    Tool: Bash (SSH + socat)
    Preconditions: Bridge user is in room (from scenario above)
    Steps:
      1. Call matrix.send with room_id and message "!status"
      2. Verify response indicates message sent successfully
    Expected Result: Message sent, bridge processes !status command
    Evidence: .sisyphus/evidence/matrix-status-e2e.txt

  Scenario: Test artifacts cleaned up
    Tool: Bash (SSH)
    Steps:
      1. Delete dummy store_key: call appropriate RPC or direct keystore cleanup
      2. Deactivate test_probe_user via admin API
    Expected Result: No test artifacts remaining
    Evidence: .sisyphus/evidence/matrix-cleanup.txt
  ```

  **Commit**: NO (deployment only, no code changes in this step)

---

## Final Verification Wave

- [ ] F1. **Plan Compliance Check** — `quick`
  Verify: matrix.join_room registered, build passes, deployed, !status captured, artifacts cleaned.
  Output: `Handler [YES/NO] | Build [PASS/FAIL] | Deployed [YES/NO] | !status [CAPTURED/PENDING] | VERDICT`

---

## Commit Strategy

- **Task 1**: `feat(rpc): expose matrix.join_room RPC handler` — server.go, test file
- **Task 2**: No code commit (deployment + verification only)

---

## Success Criteria

### Verification Commands
```bash
# On VPS — handler exists
cd bridge && go test -v -run TestMatrixJoinRoom ./pkg/rpc/...

# On VPS — bridge health after deploy
printf '{"jsonrpc":"2.0","id":1,"method":"matrix.status"}\n' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock

# On VPS — join room
printf '{"jsonrpc":"2.0","id":2,"method":"matrix.join_room","params":{"room_id":"!IGY2TnBy2gp9GpW__JI0JG0SP61PW0CeGvWCqFUMZCI"}}\n' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock

# On VPS — !status
printf '{"jsonrpc":"2.0","id":3,"method":"matrix.send","params":{"room_id":"!IGY2TnBy2gp9GpW__JI0JG0SP61PW0CeGvWCqFUMZCI","message":"!status"}}\n' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
```

### Final Checklist
- [ ] `matrix.join_room` handler registered and tested
- [ ] Bridge builds and deploys successfully
- [ ] Bridge user joins test room
- [ ] `!status` response captured as evidence
- [ ] Test artifacts cleaned up
- [ ] matrix-deployment plan fully closed (all checkboxes complete)
