# Learnings - ArmorClaw Risk Remediation

## 2026-05-03 Session Start
- Brownfield Go Bridge project with Matrix Conduit homeserver
- SQLCipher keystore for all encrypted storage
- mautrix-go v0.26.4 in go.mod but NO crypto imports yet
- Custom sync loop (must NOT be replaced)
- AgentStatus string constants are Matrix protocol contract - MUST NOT change
- E2EE is completely unimplemented (documented in HANDOFF_E2EE.md)
- Agent state inference infrastructure exists but is unwired

## TLS signConfig HMAC Security Bug — Verified Fixed
- `signConfig()` v2 (public.go:471-483) already includes TLSTrustHint and CertExpiresAt in HMAC input
- v1 format (line 460-469) correctly excludes TLS fields (no version branch issue)
- ValidateConfig() (line 410-423) delegates to signConfig() — automatically covers new fields
- All 5 security tests in public_test.go PASS: TestSignConfigV2IncludesTLSFields, TestValidateConfigRejectsTamperedTLSTrustHint, TestValidateConfigRejectsTamperedCertExpiresAt, TestValidateConfigAcceptsValidV2, TestSignConfigV1Unchanged
- No code changes needed — fix and tests were already in place

## Jetski CDP Event Access Validation
- Jetski RPC (port 9223) has ONLY: /rpc/status, /rpc/session/create, /rpc/session/close, /rpc/health, /rpc/approval/*
- NO event streaming endpoint exists — no SSE, no WebSocket subscription, no gRPC streaming
- Jetski Proxy has MessageRecorder callback that captures ALL CDP events bidirectionally — but it's in-process only
- Currently recorder feeds Sonar telemetry buffer (CircularBuffer) for post-mortem WreckageReport — not external consumers
- Bridge InferAgentState() is fully implemented and tested but UNWIRED — no way to receive CDP events
- Port 9222 is single-connection CDP WebSocket pipe — Bridge cannot attach as second consumer
- To enable: add SSE endpoint to rpc.go + subscriber fan-out (Option A) or ring buffer polling (Option B)
- Gate result: cdp_event_stream_exists = FALSE → Task 2.5 IS needed

## 2026-05-03: Task 1 — Conduit E2EE Validation

### CRITICAL: Inherited Wisdom Was Wrong (3 of 4 E2EE claims)
- "Sync filter excludes m.room.encrypted" → WRONG: matrix.go:85 INCLUDES it
- "SyncResponse lacks ToDevice field" → WRONG: both matrix.go:145 and client.go:298 have ToDevice
- "mautrix-go v0.26.4 is in go.mod" → WRONG: not in go.mod at all
- "no crypto imports exist" → WRONG: key_ingestion.go imports local pkg/crypto which has SQLCipher store

### Lesson: Always verify inherited wisdom against source before planning
The notepad from the session start had stale/incorrect information about E2EE state. Source verification took 5 minutes and corrected 3 of 4 claims. This could have caused wasted effort on already-completed work.

### E2EE Infrastructure Found (more than expected)
- ToDevice processing: processToDeviceEvents() wired in sync loop (matrix.go:805)
- KeyIngestionManager: full implementation with verification done + forwarded key handling
- crypto.Store interface + SQLCipher-backed KeystoreBackedStore (inbound group sessions table)
- SyncResponse has ToDevice, DeviceLists, DeviceOneTimeKeysCount in BOTH matrix.go and client.go
- conduit_e2ee_test.go already exists (514 lines) with live + mock + compilation tests

### What's Actually Missing
1. mautrix-go dependency (not in go.mod)
2. ingestKeyIntoStore() is a stub (returns nil)
3. No Olm/Megolm encrypt/decrypt
4. No device key upload or OTK management
5. No cross-signing implementation
6. Conduit live endpoint validation (tests skip without token)

### UIAA Strategy for Task 17: Strategy B (Medium)
Bridge infrastructure is partially built. Missing crypto library integration + actual encrypt/decrypt. Conduit likely supports standard endpoints.

## TLS Hardcoded Mode Fix - updateQRTLSInfo()
- **Bug**: `updateQRTLSInfo()` hardcoded `"private"` TLS mode regardless of actual deployment mode
- **Fix**: Added `s.deriveTLSMode()` call at top of function, early return for `"none"` mode with zero-value TLS fields
- **Native mode**: deriveTLSMode returns "none" → SetTLSInfo("none", "", "", 0)
- **Sentinel+self-signed**: deriveTLSMode returns "private" → full TLS info populated
- **Sentinel+CA**: deriveTLSMode returns "public" → full TLS info with public_ca trust hint
- **Test coverage**: 4 test cases in server_tls_test.go (Native, SelfSigned, CA, EnvOverride)
- **Build note**: `go test ./pkg/http/...` requires libyara at link time; env has libyara10 but linker looks at /tmp/yara-dev — pre-existing env issue, not code issue
- **No struct changes**: ConfigPayload layout untouched, v2 fields gated by ARMORCLAW_QR_VERSION env

## Task 7.4 — ToDevice in SyncResponse + m.room.encrypted Filter

### All code changes were already in place
- SyncResponse in matrix.go (line 142-155): ToDevice, DeviceLists, DeviceOneTimeKeysCount
- SyncResponse in client.go (line 296-302): same fields
- bridgeSyncFilter: m.room.encrypted in timeline types (line 85), to_device section (line 109-111)
- processToDeviceEvents() wired in sync loop (matrix.go:805) routing to keyIngestion.HandleKeyEvent

### Only change needed: fix 2 outdated gap-documentation tests
- TestConduitE2EESyncFilterDocumentsGap: was documenting that m.room.encrypted was missing → now FAILS because it's present. Renamed to TestConduitE2EESyncFilterIncludesEncrypted, inverted assertion
- TestConduitE2EESyncResponseDocumentsGap: was documenting missing fields → now has them. Renamed to TestConduitE2EESyncResponseHasE2EEFields, added field verification assertions
- Lesson: gap-documentation tests become failing tests once gaps are filled. Must update them to positive assertions.

## 2026-05-03 Task 23: State Enum Audit and Consolidation

### Enum Landscape
- 106 `type X string` enum-like definitions across bridge/pkg/
- 13 iota-based int enums
- Most enums are domain-specific and correctly scoped to their package

### Duplicates Found
- **Safe merge**: `email.ApprovalStatus` → alias to `pii.AccessRequestStatus` (identical lowercase values: pending/approved/rejected/expired)
- **Blocked by import cycle**: `studio.BrowserState` ↔ `browser.ServiceState` and `studio.WaitUntil` ↔ `browser.ServiceWaitUntil` — cycle is browser→queue→studio
- **Not merged (semantic difference)**: `pii.ConsentState` vs `pii.AccessRequestStatus` — same values but different lifecycle domains
- **Not merged (case mismatch)**: `studio.ApprovalStatus` (UPPERCASE) vs `pii.AccessRequestStatus` (lowercase)
- **Not merged (subset relationship)**: `queue.JobStatus` (7 values), `studio.InstanceStatus` (6), `secretary.BlindFillStatus` (6) — each has unique values

### Import Cycle Constraint
- `browser` → `queue` → `studio` creates a cycle
- Cannot alias studio enums to browser enums without breaking this cycle
- Solution: extract shared browser types to `bridge/pkg/types/browser.go` (deferred to avoid scope creep)

### Pattern: Type Alias for Backward Compatibility
- `type OldStatus = pkg.NewStatus` — Go type alias preserves all method sets and const assignments
- Constants forwarded: `const OldPending = pkg.NewPending`
- Deprecated doc comment on the alias type guides future consumers

### AgentStatus Contract Test
- Added `state_contract_test.go` that asserts all 11 string constants are exactly as specified
- Catches accidental string changes (Matrix protocol contract with Android)
- Also asserts count=11 to catch additions/removals

## 2026-05-03 Azure Blob rustls Migration

### Key Finding: aws-lc-rs vs ring for rustls crypto backend
- AWS SDK for Rust v1.x defaults to `aws-lc-rs` as rustls crypto backend via `aws-config/default-https-client` → `aws-smithy-runtime/default-https-client` → `aws-smithy-http-client?/rustls-aws-lc`
- `aws-lc-rs` requires `clang` at build time (aws-lc-sys crate) — a system dependency
- Solution: disable `default-features` on `aws-config` and `aws-sdk-s3`, use `aws-sdk-s3/rustls` feature instead which routes through `aws-smithy-runtime/tls-rustls` → `legacy-rustls-ring` (uses ring, only needs gcc)
- `cargo tree -i aws-lc-sys` confirmed removal after feature change

### Azure SDK v0.21 API is completely different from earlier versions
- Old code used `BlobClient::from_connection_string()` — doesn't exist in v0.21
- v0.21 pattern: `BlobServiceClient::new(account, StorageCredentials::access_key(...))` → `.container_client(name)` → `.blob_client(name)` → `.put_block_blob(data)` / `.get_content()` / `.delete()`
- `ContainerClient::list_blobs()` returns paginated stream via `.into_stream()` — must iterate with `futures::StreamExt`
- `ListBlobsResponse.blobs` has `items: Vec<BlobItem>` (enum: Blob | BlobPrefix) — use `.blobs()` method to filter only actual blobs
- `Prefix` type requires `'static` lifetime — pass `prefix.to_string()` not `&str`

### Two CloudConnector traits exist in codebase
- `connector.rs` has 7-method version (upload, download, list, delete, health_check, get_config)
- `sharepoint.rs` has 4-method version (upload, download, list, delete) — THIS is the canonical one used by mod.rs
- Azure blob connector implements the 4-method trait from sharepoint

### Pre-existing test issues fixed
- `grpc/middleware/rate_limit.rs` tests: missing `use std::time::Duration; use tonic::Code;`
- Same file: duplicate `test_rate_limit_interceptor_with_registry` function (renamed second to `_rate_limiting`)
- Same file: extra `}` closing brace that split tests module
- `connectors/aws_s3.rs` tests: missing `use tokio::io::BufReader;`

### Build requirement: CC=gcc
- Environment has `CC=clang` but clang not installed (no sudo access)
- Workaround: `CC=gcc cargo build` / `CC=gcc cargo test --lib`
- This should be set in `.cargo/config.toml` for the project
