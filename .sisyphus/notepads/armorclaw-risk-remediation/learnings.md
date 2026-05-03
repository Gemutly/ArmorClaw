# Learnings — ArmorClaw Risk Remediation

## 2026-05-03: Task 2 — Jetski CDP Event Access Validation

- Jetski RPC (port 9223) has only 4 endpoints: status, session/create, session/close, health. NO event subscription endpoint exists.
- CDP events flow bidirectionally through proxy.go but are only recorded into an in-memory Sonar circular buffer (1000 frames).
- Sonar buffer is consumed only by WreckageReport (post-mortem flight recorder), never exposed via RPC.
- The `MessageRecorder` callback pattern (`func(method string, params json.RawMessage)`) already fires on every CDP message in both directions — good hook point for a future subscriber.
- Bridge's `InferAgentState()` expects `[]CDPEvent{Method string, Params map[string]interface{}}` but Jetski stores `json.RawMessage` — type conversion needed at the boundary.
- Task 2.5 IS needed: must add `/rpc/events/stream` WebSocket endpoint or SSE to Jetski RPC server, wired to a broadcast channel from the existing recorder callback.
- Method router (router.go) only handles outbound commands, NOT events — no event interception there.
- Approval handlers are conditionally registered via `approval.RegisterApprovalHandlers(mux, ac)` — separate from the 4 core RPC endpoints.

## Task: TLS signConfig HMAC Security Bug Fix (2026-05-03)

### Finding: signConfig v2 HMAC omitted TLSTrustHint and CertExpiresAt
- Bug: v2 HMAC included TLSMode + TLSFingerprintSHA256 but missed TLSTrustHint and CertExpiresAt
- Impact: Attacker could modify TLS trust hint without invalidating QR config signature
- Fix: Added both fields to the Sprintf format string in the v2 branch
- ValidateConfig auto-fixed since it delegates to signConfig

### Pattern: HMAC field coverage must match all signed config fields
- When adding fields to ConfigPayload, always check signConfig includes them
- v1/v2 branch separation means each branch needs independent audit
- Test strategy: sign → tamper single field → verify rejection catches omissions
