# Conduit E2EE Capability Validation

**Task:** 1 — Validate Conduit E2EE Support (Prerequisite Spike)
**Date:** 2026-05-03
**Status:** Spike Complete — Documentation Only

## Purpose

Document which Matrix E2EE endpoints Conduit supports and identify
prerequisite gaps in the ArmorClaw bridge before E2EE implementation.

## Capability Matrix

| Endpoint | Matrix Path | Conduit Support | Bridge Gap |
|----------|-------------|----------------|------------|
| ToDevice | `PUT /_matrix/client/v3/sendToDevice/{type}/{txId}` | Needs live validation | `SyncResponse` lacks `ToDevice` field (matrix.go:127-136) |
| Key Upload | `POST /_matrix/client/v3/keys/upload` | Needs live validation | No crypto module exists; `bridge/pkg/crypto/e2ee.go` referenced in HANDOFF but file missing |
| Key Query | `POST /_matrix/client/v3/keys/query` | Needs live validation | No crypto module exists |
| Key Claim | `POST /_matrix/client/v3/keys/claim` | Needs live validation | No crypto module exists |
| Cross-signing | `POST /_matrix/client/v3/keys/device_signing/upload` | Needs live validation | No cross-signing support; UIAA integration missing |
| UIAA for cross-signing | Same as cross-signing | Needs live validation | No UIAA handler in MatrixAdapter |

## Conduit Server Status

**cdp_event_stream_exists:** false (N/A for this task — no CDP dependency)

## Source Code Analysis

### go.mod Dependency Status

`mautrix-go` is **NOT** in `bridge/go.mod`. The HANDOFF_E2EE.md document states
`maunium.net/go/mautrix v0.26.4` was added, but the current go.mod does not contain it.
This means either:
1. The dependency was removed after the HANDOFF was written, or
2. The HANDOFF was aspirational and the dependency was never added.

**Action required:** Add `maunium.net/go/mautrix` to go.mod before any E2EE work.

### SyncResponse Gaps (matrix.go:127-136)

Current struct:
```go
type SyncResponse struct {
    NextBatch string `json:"next_batch"`
    Rooms     struct {
        Join map[string]struct {
            Timeline struct {
                Events []json.RawMessage `json:"events"`
            } `json:"timeline"`
        } `json:"join"`
    } `json:"rooms"`
}
```

Missing fields required for E2EE:
- `ToDevice` — receives to-device messages (key exchanges, SAS verification)
- `DeviceLists` — tracks changed device lists
- `DeviceOneTimeKeysCount` — signals when to upload more one-time keys

### Sync Filter Gaps (matrix.go:76-107)

The `bridgeSyncFilter` explicitly lists only these timeline types:
- `m.room.message`
- `m.room.member`
- `m.room.bridge`
- `app.armorclaw.alert`
- `com.armorclaw.agent.status`

**Missing:** `m.room.encrypted` — encrypted messages are silently dropped.

### Key Ingestion Skeleton (key_ingestion.go:167-178)

A stub `ingestKeyIntoStore` exists but returns nil without doing anything.
The comments reference libolm directly, but mautrix-go's crypto abstraction
should be used instead.

### Crypto Module Status

HANDOFF_E2EE.md references `bridge/pkg/crypto/e2ee.go` as "Created (incomplete)"
but **this file does not exist**. The entire crypto module needs to be created.

## Required mautrix-go Crypto Imports

```go
import (
    "maunium.net/go/mautrix"
    "maunium.net/go/mautrix/crypto/cryptohelper"
    "maunium.net/go/mautrix/crypto/store"
    "maunium.net/go/mautrix/id"
)
```

### Key Types

| Type | Package | Purpose |
|------|---------|---------|
| `CryptoHelper` | `crypto/cryptohelper` | Auto encrypt/decrypt via `client.Crypto` |
| `OlmAccount` | `crypto/account` | Device identity and one-time key generation |
| `InboundGroupSession` | `crypto/olm` | Megolm session for decrypting received messages |
| `OutboundGroupSession` | `crypto/olm` | Megolm session for encrypting sent messages |
| `Store` | `crypto/store` | Persistent storage interface for sessions and keys |
| `CrossSigningKey` | `crypto` | Cross-signing key management |
| `DeviceID` | `id` | Typed device identifier |
| `RoomID` | `id` | Typed room identifier |

## Test File Created

`bridge/internal/adapter/conduit_e2ee_test.go`

### Test Functions

| Test | Requires Conduit | Purpose |
|------|-----------------|---------|
| `TestConduitE2EE` | Yes (env vars) | Probes all E2EE endpoints on live Conduit |
| `TestConduitE2EECompilation` | No | Validates types compile, documents gaps |
| `TestConduitE2EEMockServer` | No | Mock server validates test infrastructure |
| `TestConduitE2EESyncFilterDocumentsGap` | No | Documents sync filter missing m.room.encrypted |
| `TestConduitE2EESyncResponseDocumentsGap` | No | Documents SyncResponse missing ToDevice |

### Running Live Tests

```bash
export CONDUIT_TEST_URL=http://localhost:6167
export CONDUIT_TEST_TOKEN=<access_token>
export CONDUIT_TEST_USER=@youruser:localhost
export CONDUIT_TEST_DEVICE=YOUR_DEVICE

go test -v -run TestConduitE2EE ./internal/adapter/...
```

### Running Compilation Tests (always pass)

```bash
go test -v -run "TestConduitE2EEMockServer|TestConduitE2EECompilation|TestConduitE2EESync" ./internal/adapter/...
```

## Dependencies on This Task

This task's output blocks:
- Task 7: E2EE implementation (needs to know which endpoints are supported)
- Task 9: Cross-signing (needs UIAA flow documentation)
- Task 13: Key backup (needs crypto module imports)

## Decisions

1. **Use mautrix-go crypto/cryptohelper** — provides automatic encrypt/decrypt,
   handles device registration, OTK management, and session management.
2. **Use goolm (pure Go) backend** — no CGO dependency, easier cross-compilation.
3. **Store keys in SQLCipher** — already in use for keystore, consistent security model.
