# Work Plan: E2EE Architecture for ArmorClaw Bridge

## Overview

Remove Bridge from trust boundary by implementing encrypted passthrough, cross-signing, and device authorization. This allows clients (ArmorChat, Android) to own their own encryption keys, making Bridge a "trustless relay" for Matrix message transport.

## Success Conditions

- Encrypted blobs pass through unchanged
- Cross-signing metadata handled
- Device authorization via QR claim
- Cross-signing during provisioning
- Agents use shadow mapping for PII instead of direct access

## TL;DR

Bridge becomes a "trustless relay" for Matrix transport - encrypted blobs and cross-signing metadata pass through with client-side decryption. Cross-signing during provisioning removes Bridge from trust boundary.

## Context

Phase 17 (Voice Backend AI Pipeline) complete. This plan implements E2EE for Matrix communication.

**Key Decisions:**

| Decision | Choice | Rationale |
|-----------|--------|-----------|
| **Trust Model** | CLIENT_SIDE decryption | Client (ArmorChat) owns session keys, not Bridge |
| **Message Flow** | Encrypted passthrough | Bridge routes blobs unchanged |
| **Device Auth** | Cross-signing QR claim | Hardware-backed chain of trust |
| **PII Handling** | Shadow Mapping 2.0 | Agents receive placeholders, not raw data |

## Scope

### IN Scope

- **m.room.encrypted** passthrough in Matrix adapter
- **Cross-signing** during provisioning
- **Encrypted key backups** (recovery blobs)
- **Device authorization** RPC methods
- **Shadow Mapping 2.0** in agents
- Testing with mock encrypted events

### OUT Scope (V1)

- Bridge-side decryption (existing behavior)
- No access token storage in keystore
- No changes to existing message handling (`m.room.message`)
- Existing tests continue to work
- All existing functionality preserved

## Non-Negotiable Constraints

- **Security**: No server-side decryption (Bridge cannot read encrypted content)
- **Client Ownership**: Session keys managed on-device
- **Performance**: No bridge-side decryption overhead
- **Backwards Compatibility**: Existing tests continue to pass
- **Minimal Patches**: No rewrites, only targeted changes

## Dependencies

### Internal

- Matrix SDK (go-Olm/Megolm)
- Encryption libraries (Rust SDK, libolm, libsodium)
- Element Web (Android)

### External

- Testing utilities
- Existing test infrastructure

## Risks

| Risk | Mitigation |
|-------|------------|
| **Breaking existing clients** | ArmorChat and Element Web expect encrypted messages |
| **Server-side decryption bypass** | Bridge currently decrypts Matrix events before sending to clients |
| **Breaking device sessions** | If device sessions are managed per-device, clients lose ability to verify signatures |
| **Client-side decryption complexity** | Adds latency and complexity to decryption logic |
| **Key backup migration** | Complex change, requires careful planning to avoid breaking existing clients |
| **Testing coverage gaps** | New code means existing tests may not cover all scenarios |
| **Security review** | Need thorough security audit for any implementation changes |

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                  E2EE PASSTHROUGH ARCHITECTURE                        │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────────┐   │
│  │                    ARMORCHAT CLIENT (Android)            │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ Matrix SDK  │  │ Rust SDK    │  │ Key Vault   │     │   │
│  │  │ (Go-Olm)   │  │ (vodozemac) │  │ (Secure)    │     │   │
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘     │   │
│  │         │                  │                  │               │   │
│  │         └──────────────────┴──────────────────┘               │   │
│  │                          │                               │   │
│  │                    Decrypt (client-side)              │   │
│  │                          │                               │   │
│  └──────────────────────────┬───┴───────────────────────────────┘   │
│                         │                                         │
│                ┌─────────▼──────────────────────────────────────────┐   │
│                │         DISPLAY (Plaintext)                  │   │
│                └─────────┬──────────────────────────────────────────┘   │
│                          │                                         │
│                ┌─────────▼──────────────────────────────────────────┐   │
│                │         USER INPUT (Text, Voice)             │   │
│                └─────────┬──────────────────────────────────────────┘   │
│                          │                                         │
└──────────────────────────┼─────────────────────────────────────────────────┘
                         │
┌────────────────────────▼─────────────────────────────────────────────────┐
│                    BRIDGE (Go - Trustless Relay)              │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌───────────────────────────────────────────────────────────────────┐   │
│  │              Matrix Adapter (Encrypted Passthrough)                │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ Sync Events │  │ RPC Stream  │  │ Cross-Sign  │     │   │
│  │  │ (encrypted) │  │ (blobs)     │  │ (metadata)  │     │   │
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘     │   │
│  │         │                  │                  │               │   │
│  └─────────┴──────────────────┴──────────────────┴───────────────┘   │
│                                                                      │
│  ┌───────────────────────────────────────────────────────────────────────┐   │
│  │                    MatrixSyncManager                        │   │
│  │         Routes encrypted events to RPC without decryption           │   │
│  └───────────────────────────────────────────────────────────────────────┘   │
│                                                                      │
│  ┌───────────────────────────────────────────────────────────────────────┐   │
│  │                    Keystore (SQLCipher)                        │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ API Keys    │  │ Cold Vault   │  │ Key Backups │     │   │
│  │  │ (non-e2ee) │  │ (no tokens) │  │ (encrypted) │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └───────────────────────────────────────────────────────────────────────┘   │
│                                                                      │
│  ┌───────────────────────────────────────────────────────────────────────┐   │
│  │                    Provisioning (Cross-Signing)                │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ QR Claim    │  │ Cross-Sign  │  │ Device Auth │     │   │
│  │  │ (signature) │  │ (metadata)  │  │ (RPC API)   │     │   │
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘     │   │
│  └─────────┴──────────────────┴──────────────────┴───────────────┘   │
│                                                                      │
│  ┌───────────────────────────────────────────────────────────────────────┐   │
│  │                    Agent Studio (Shadow Mapping)               │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ PII Shadows  │  │ Redaction   │  │ Skill Gate  │     │   │
│  │  │ (placeholders)│  │ (metadata)   │  │ (hitl)      │     │   │
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘     │   │
│  └─────────┴──────────────────┴──────────────────┴───────────────┘   │
│                                                                      │
│  ┌───────────────────────────────────────────────────────────────────────┐   │
│  │                    Agent Containers (Docker)                │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ Agent Runtime│  │ Skills      │  │ MCP         │     │   │
│  │  │ (blind)     │  │ (shadowed) │  │ (no pii)    │     │   │
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘     │   │
│  └─────────┴──────────────────┴──────────────────┴───────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
                         │
┌────────────────────────▼─────────────────────────────────────────────────┐
│                  MATRIX SERVER (Conduit)                     │
├─────────────────────────────────────────────────────────────────────────────┤
│  Stores encrypted blobs as-is                                          │
│  No decryption capability                                          │
│  Routes events to Bridge and Clients equally                         │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Execution Strategy

### Wave 1: Matrix Adapter Refactor (2 tasks)

**Task 1.1: Encrypted Passthrough Implementation**
- Modify `bridge/internal/adapter/matrix.go` to handle `m.room.encrypted` events
- Add `EncryptedEvent` type for opaque blob handling
- Update `processEvents()` to route encrypted events without decryption
- Add security check: fail if plaintext message in encrypted room

**Task 1.2: RPC Stream Integration**
- Ensure `MatrixSyncManager` correctly routes encrypted timeline events to RPC stream
- Add encryption metadata to RPC payloads (algorithm, sender_key)
- Add tests for encrypted event routing

### Wave 2: Keystore "Cold Vault" Migration (2 tasks)

**Task 2.1: Remove Access Token Storage**
- Modify `bridge/pkg/keystore/keystore.go` to stop storing Matrix access tokens
- Remove `GetMatrixToken()`, `StoreMatrixToken()` methods
- Document migration path for existing tokens

**Task 2.2: Encrypted Key Backups**
- Add `store_encrypted_key_backup()` method for client key recovery
- Storage format: encrypted blob (Bridge cannot read)
- Add retrieval method for client recovery flows
- Add tests for key backup storage

### Wave 3: Provisioning & Device Authorization (1 task)

**Task 3.1: Cross-Signing QR Claim**
- Update `bridge/pkg/provisioning/provisioning.go` to support cross-signing
- Add device signature verification during QR claim
- First device signs subsequent devices
- Add RPC methods to `BridgeAdminClient`:
  - `admin.list_devices()`
  - `admin.revoke_device(deviceID)`
  - `admin.verify_device(deviceID, signature)`
- Add tests for provisioning updates

### Wave 4: Shadow Mapping 2.0 (1 task)

**Task 4.1: BlindFill & Redaction Update**
- Update `bridge/internal/agent/agent_interceptor.go` for Shadow Mapping 2.0
- Agent operates on "Shadow References" instead of raw content
- Implement "BlindFill" logic:
  - Agent requests PII placeholder: `{{VAULT:email:0x...}}`
  - Bridge stores placeholder (cannot resolve)
  - ArmorChat detects placeholder and fills locally before encrypting
- Add sensitivity check for encrypted content
- Add tests for shadow mapping with encrypted messages

### Wave 5: Integration & Verification (1 task)

**Task 5.1: E2EE Smoke Tests**
- Create encrypted event fixtures
- Test passthrough through Matrix adapter
- Test RPC stream delivers encrypted blobs unchanged
- Test cross-signing provisioning flow
- Test agent shadow mapping with placeholders
- Verify build passes
- Run full test suite

## Files to Create

| File | Purpose |
|------|---------|
| `.sisyphus/plans/e2ee-architecture.md` | This plan |
| `bridge/internal/adapter/matrix_encrypted.go` | Encrypted event types and routing |
| `bridge/pkg/keystore/keystore_encrypted.go` | Key backup storage methods |
| `bridge/pkg/provisioning/provisioning_e2ee.go` | Cross-signing implementation |
| `bridge/internal/agent/agent_interceptor_e2ee.go` | Shadow mapping 2.0 |
| `bridge/internal/adapter/matrix_encrypted_test.go` | Encrypted passthrough tests |
| `bridge/pkg/keystore/keystore_encrypted_test.go` | Key backup tests |
| `bridge/pkg/provisioning/provisioning_e2ee_test.go` | Cross-signing tests |
| `bridge/internal/agent/agent_interceptor_e2ee_test.go` | Shadow mapping tests |

## Files to Modify

| File | Changes |
|------|----------|
| `bridge/internal/adapter/matrix.go` | Add m.room.encrypted handling |
| `bridge/pkg/keystore/keystore.go` | Remove token storage, add backup methods |
| `bridge/pkg/provisioning/provisioning.go` | Add cross-signing, device auth RPC |
| `bridge/internal/agent/agent_interceptor.go` | Update for Shadow Mapping 2.0 |
| `bridge/cmd/bridge/main.go` | Wire E2EE components |

## API Signatures

### Matrix Adapter (Encrypted Passthrough)

```go
// New event type
type EncryptedEvent struct {
    EventID     string
    RoomID      string
    Sender      string
    Timestamp   time.Time
    Algorithm   string  // m.megolm.v1.aes-sha2
    SenderKey  string  // base64 encoded
    CipherText  []byte  // encrypted blob
}

// New RPC payload
type EncryptedMessage struct {
    RoomID      string
    EventID     string
    Sender      string
    Algorithm   string
    SenderKey  string
    CipherText  []byte
    IsEncrypted bool
}

// Updated RPC method
func (a *MatrixAdapter) ProcessEncryptedEvent(event *EncryptedEvent) error
func (a *MatrixAdapter) GetEncryptedTimeline(roomID string) ([]EncryptedEvent, error)
```

### Keystore (Cold Vault)

```go
// New methods
func (k *Keystore) StoreEncryptedKeyBackup(userID string, encryptedBackup []byte) error
func (k *Keystore) GetEncryptedKeyBackup(userID string) ([]byte, error)
func (k *Keystore) DeleteEncryptedKeyBackup(userID string) error

// Removed methods
// func (k *Keystore) GetMatrixToken() (string, error)
// func (k *Keystore) StoreMatrixToken(token string) error
```

### Provisioning (Cross-Signing)

```go
// New RPC methods
func (p *Provisioning) GetQRClaimSignature(claimID string) (string, error)
func (p *Provisioning) VerifyDeviceSignature(deviceID string, signature string) (bool, error)
func (p *Provisioning) ListDevices() ([]Device, error)
func (p *Provisioning) RevokeDevice(deviceID string) error

type Device struct {
    DeviceID    string
    DisplayName string
    Trusted     bool
    LastSeen    time.Time
    SignedBy    string  // device ID that signed this one
}
```

### Agent Interceptor (Shadow Mapping 2.0)

```go
// Shadow placeholder format
const ShadowPlaceholderPattern = `{{VAULT:(email|phone|ssn|...):0x[0-9a-f]+}}`

// Updated interceptor
type ShadowMappingInterceptor struct {
    vaultEnabled bool
    shadowCache map[string]string
}

func (i *ShadowMappingInterceptor) InterceptRequest(req *AgentRequest) (*AgentRequest, error) {
    // Detect shadow placeholders
    // Map to encrypted metadata (Bridge cannot read)
    // Return placeholder to agent
}

func (i *ShadowMappingInterceptor) ValidateResponse(resp *AgentResponse) error {
    // Ensure no raw PII in response
    // Check for shadow placeholders only
}
```

## Testing Strategy

### Unit Tests

| Component | Tests |
|-----------|--------|
| Matrix Adapter | Encrypted passthrough, RPC routing, security checks |
| Keystore | Key backup storage, token removal |
| Provisioning | Cross-signing, device auth |
| Agent Interceptor | Shadow mapping, placeholder detection |

### Integration Tests

| Scenario | Test |
|-----------|-------|
| Encrypted message flow | Matrix → Adapter → RPC → Client |
| Cross-signing | QR claim → Signature verify → Device add |
| Key backup | Store → Retrieve → Decrypt (client-side) |
| Shadow mapping | Agent request → Placeholder → Client fill |

### E2E Tests

| Flow | Test |
|------|-------|
| Full E2EE chat | Client encrypts → Bridge routes → Client decrypts |
| Device provisioning | New device → Cross-sign → Verify |
| Recovery | Key backup → New device → Restore |

## Success Metrics

- All tests pass
- Build succeeds
- No bridge-side decryption in production
- Encrypted messages route unchanged through Bridge
- Cross-signing works during provisioning
- Shadow placeholders prevent PII leakage to Bridge
- Existing functionality preserved
- Documentation updated

## Rollback Plan

If E2EE implementation causes critical issues:

1. Revert Matrix adapter to existing decryption logic
2. Restore access token storage in keystore
3. Disable cross-signing (revert to legacy QR)
4. Disable shadow mapping (agents see raw content)
5. Hotfix deployment
6. Investigate and fix issues in next iteration

## Documentation Updates

- `docs/ACTIVE/review.md` - Add E2EE architecture section
- `docs/ACTIVE/e2ee-guide.md` - New guide for E2EE setup
- `docs/ACTIVE/shadow-mapping.md` - Shadow mapping 2.0 reference
- `README.md` - Update with E2EE overview

## References

- Matrix Spec: https://spec.matrix.org/v1.3/client-server-api/#m-room-encrypted
- go-Olm: https://github.com/matrix-org/gomatrix
- vodozemac (Rust SDK): https://github.com/vodozemac/vodozemac
- BlindFill: https://blindfill.org/

## Questions for User

1. Should I proceed with creating detailed task specifications for Wave 1?
2. Do you want me to start with a specific component (Matrix Adapter, Keystore, Provisioning)?
3. Any preference on which client library to use for Matrix E2EE (go-olm vs custom implementation)?
4. Should I include detailed API signatures in the plan, or keep them high-level?
5. Is there a specific timeline or deadline for E2EE implementation?
6. Should we create a staging environment for E2EE testing before production deployment?
