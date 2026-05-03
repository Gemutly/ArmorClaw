# Conduit E2EE Validation — Evidence Report

**Date:** 2026-05-03
**Task:** Task 1 — Conduit E2EE Support Gate Validation
**Status:** UNBLOCKED (bridge infrastructure partially ready; Conduit requires live probe)

---

## Capability Matrix

| # | Capability | Bridge Code Status | Conduit HS Status | Evidence File | Evidence Lines |
|---|-----------|-------------------|-------------------|---------------|----------------|
| 1 | ToDevice (sendToDevice) | **READY** | **UNKNOWN** (needs live test) | `bridge/internal/adapter/matrix.go` | 109-111 (filter), 131-134 (type), 798-799 (processing), 805-825 (handler) |
| 2 | Device Keys Upload (`/keys/upload`) | **STUB** (test only) | **UNKNOWN** (needs live test) | `bridge/internal/adapter/conduit_e2ee_test.go` | 161-186 |
| 3 | Device Keys Query (`/keys/query`) | **STUB** (test only) | **UNKNOWN** (needs live test) | `bridge/internal/adapter/conduit_e2ee_test.go` | 189-208 |
| 4 | Device Keys Claim (`/keys/claim`) | **STUB** (test only) | **UNKNOWN** (needs live test) | `bridge/internal/adapter/conduit_e2ee_test.go` | 211-232 |
| 5 | Cross-Signing Upload | **STUB** (test only) | **UNKNOWN** (needs live test) | `bridge/internal/adapter/conduit_e2ee_test.go` | 235-257, 259-298 |
| 6 | SyncFilter includes `m.room.encrypted` | **YES** | N/A | `bridge/internal/adapter/matrix.go` | 85 |
| 7 | SyncResponse.ToDevice field | **YES** (both) | N/A | `bridge/internal/adapter/matrix.go` | 145; `bridge/pkg/matrix/client.go` | 298 |
| 8 | SyncResponse.DeviceLists field | **YES** (both) | N/A | `bridge/internal/adapter/matrix.go` | 146; `bridge/pkg/matrix/client.go` | 299 |
| 9 | Crypto Store (SQLCipher) | **YES** | N/A | `bridge/pkg/crypto/keystore_store.go` | 1-253 |
| 10 | Olm/Megolm encryption/decryption | **NO** | N/A | `bridge/go.mod` (no mautrix-go) | — |

---

## Boolean Capability Flags

```yaml
todevice_supported: true          # Bridge processes ToDevice events via processToDeviceEvents()
device_keys_supported: false       # No actual /keys/* implementation; test stubs only
cross_signing_supported: false     # No cross-signing logic; UIAA probe test exists
mroom_encrypted_in_filter: true    # bridgeSyncFilter includes "m.room.encrypted" (matrix.go:85)
sync_response_has_todevice: true   # Both matrix.go and client.go SyncResponse have ToDevice field
crypto_store_exists: true          # SQLCipher-backed KeystoreBackedStore in pkg/crypto/
mautrix_crypto_imported: false     # mautrix-go NOT in go.mod; no Olm/Megolm integration
```

---

## UIAA Strategy for Task 17

**Recommended: Strategy B (Medium)**

Rationale:
- Bridge infrastructure is **partially built**: ToDevice processing, crypto store, key ingestion manager all exist
- Missing: mautrix-go dependency, actual Olm/Megolm crypto, encryption/decryption hooks in send/receive paths
- Conduit likely supports standard MSC endpoints (sendToDevice, /keys/*, device_signing/upload) but requires live validation
- UIAA for cross-signing upload is expected (standard Matrix behavior) — test infrastructure exists to probe this

---

## Corrections to Inherited Wisdom

The session start notepad contained 4 statements about E2EE state. Source-code verification found **3 were incorrect**:

| Statement | Inherited Wisdom | Actual (Source Verified) | File:Line |
|-----------|-----------------|------------------------|-----------|
| Sync filter excludes m.room.encrypted | "explicitly excludes" | **INCLUDES** it | `matrix.go:85` |
| SyncResponse lacks ToDevice field | "lacks ToDevice field in both" | **HAS** ToDevice, DeviceLists, DeviceOneTimeKeysCount | `matrix.go:145-147`, `client.go:298-300` |
| mautrix-go v0.26.4 in go.mod | "v0.26.4 is in go.mod" | **NOT in go.mod** | `bridge/go.mod` (full scan) |
| No crypto imports exist | "no crypto imports exist" | **Local crypto package exists** with SQLCipher store | `key_ingestion.go:12`, `pkg/crypto/*.go` |

---

## Existing E2EE Infrastructure Inventory

### Types & Structs
- `ToDevice` — `matrix.go:132-134`, `client.go:287-289`
- `DeviceLists` — `matrix.go:137-139`, `client.go:291-294`
- `SyncResponse` — `matrix.go:143-155`, `client.go:296-302` (includes ToDevice, DeviceLists, DeviceOneTimeKeysCount)
- `VerifiedDevice` — `key_ingestion.go:26-31`
- `ForwardedKey` — `key_ingestion.go:34-41`
- `KeyVerificationEvent` — `key_ingestion.go:44-51`
- `RoomKeyForwardEvent` — `key_ingestion.go:54-57`
- `RoomKeyContent` — `key_ingestion.go:60-68`

### Processing Logic
- `processToDeviceEvents()` — `matrix.go:805-825` — dispatches to KeyIngestionManager
- `KeyIngestionManager.HandleKeyEvent()` — `key_ingestion.go:236-256` — routes m.key.verification.done and m.forwarded_room_key
- `KeyIngestionManager.HandleVerificationDone()` — `key_ingestion.go:86-113`
- `KeyIngestionManager.HandleForwardedKey()` — `key_ingestion.go:117-165`
- `KeyIngestionManager.ingestKeyIntoStore()` — `key_ingestion.go:168-178` (stub — returns nil, comment says "In production")

### Crypto Store
- `crypto.Store` interface — `pkg/crypto/store.go:11-23`
- `crypto.MemoryStore` — `pkg/crypto/store.go:26-69`
- `crypto.KeystoreBackedStore` — `pkg/crypto/keystore_store.go:16-253` (SQLCipher-backed, with schema, CRUD, stats)
- Schema: `inbound_group_sessions` table with room_id, sender_key, session_id, session_key (BLOB), indexes

### Test Infrastructure
- `conduit_e2ee_test.go` — 514 lines, 8 test functions
  - `TestConduitE2EE` — live Conduit probe (skips without CONDUIT_TEST_TOKEN)
  - `TestConduitE2EECompilation` — struct/filter validation (always runs)
  - `TestConduitE2EEMockServer` — mock Conduit with all E2EE endpoints (always runs)
  - `TestConduitE2EESyncFilterDocumentsGap` — sync filter gap check (always runs)
  - `TestConduitE2EESyncResponseDocumentsGap` — SyncResponse gap documentation (always runs)

---

## Gaps Requiring Work

1. **mautrix-go dependency** — must be added to go.mod for Olm/Megolm crypto
2. **`ingestKeyIntoStore()` is a stub** — `key_ingestion.go:168-178` returns nil, comment says "In production"
3. **No message encryption/decryption** — `SendMessage` does not encrypt; `processEvents` does not decrypt m.room.encrypted
4. **No device key upload** — bridge never uploads its own device keys
5. **No OTK management** — bridge never uploads one-time keys
6. **No cross-signing** — no MSK, SSK, or USK management
7. **Conduit live validation** — all Conduit endpoint tests skip without live server + token

---

## Test File

**Path:** `bridge/internal/adapter/conduit_e2ee_test.go`
**Status:** EXISTS (514 lines)
**Tests requiring Conduit:** `TestConduitE2EE` (skips with `CONDUIT_TEST_TOKEN` env)
**Tests that always run:** `TestConduitE2EECompilation`, `TestConduitE2EEMockServer`, `TestConduitE2EESyncFilterDocumentsGap`, `TestConduitE2EESyncResponseDocumentsGap`

To run live validation:
```bash
CONDUIT_TEST_TOKEN=<token> CONDUIT_TEST_URL=http://localhost:6167 \
  go test -v ./internal/adapter/ -run TestConduitE2EE
```

To run all E2EE tests (no Conduit required):
```bash
go test -v ./internal/adapter/ -run TestConduitE2EE
```
