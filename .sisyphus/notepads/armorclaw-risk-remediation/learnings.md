
## Task 1: Conduit E2EE Validation Spike (2026-05-03)

### Finding: HANDOFF_E2EE.md is stale
- Claims mautrix-go v0.26.4 is in go.mod — it is NOT
- Claims bridge/pkg/crypto/e2ee.go exists — it does NOT
- Takeaway: Always verify handoff docs against actual source tree before planning

### Finding: SyncResponse is too minimal for E2EE
- Lacks ToDevice, DeviceLists, DeviceOneTimeKeysCount fields
- Sync filter explicitly excludes m.room.encrypted
- Both must be fixed before any E2EE implementation

### Finding: mautrix-go CryptoHelper is the right abstraction
- crypto/cryptohelper.NewCryptoHelper handles login, OTK upload, encrypt/decrypt
- Attach to client.Crypto for automatic E2EE
- goolm backend (pure Go) preferred — no CGO

### Finding: Conduit E2EE support needs live validation
- All 6 endpoints (ToDevice, keys/upload, keys/query, keys/claim, cross-signing, UIAA) need live Conduit to confirm
- Tests skip gracefully without CONDUIT_TEST_TOKEN env var
- Mock server tests validate infrastructure without Conduit
