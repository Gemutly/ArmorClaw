# ArmorClaw Security

> <!-- Source: armorclaw.md, license-system.md -->

## Table of Contents

1. [Security Architecture Overview](#security-architecture-overview)
2. [Rust Vault](#rust-vault)
3. [BlindFill Security](#blindfill-security)
4. [PII Masking](#pii-masking)
5. [Governor-Shield](#governor-shield)
6. [Keystore Security](#keystore-security)
7. [Container Hardening](#container-hardening)
8. [Audit Logging](#audit-logging)
9. [Zero-Trust Architecture](#zero-trust-architecture)
10. [Secure Document Pipeline](#secure-document-pipeline)
11. [v6 Microkernel Governance](#v6-microkernel-governance)
12. [License System](#license-system)

---

## Security Architecture Overview

ArmorClaw's security model is built on multiple overlapping layers: **BlindFill™** secret injection ensures agents never see raw PII values; **Governor-Shield** intercepts PII in AI tool calls before they reach external models; **SQLCipher keystore** provides hardware-bound encrypted credential storage; **container isolation** runs each agent in a hardened Docker container with `NetworkMode: "none"`; and a **zero-trust device verification** system gates all access. All communication uses Matrix E2EE. The Bridge implements full Matrix E2EE via mautrix-go v0.27.0 with Olm/Megolm encrypt/decrypt, SAS emoji verification, and cross-signing bootstrap. E2EE is gated by `matrix.e2ee.enabled` (default false) with runtime toggle RPCs and audit logging.

### Key Security Features

| Feature | Description |
|---------|-------------|
| **BlindFill™** | Memory-only secret injection, agents never see raw values |
| **Placeholder Masking** | Strict `{{VAULT:field:hash}}` format prevents secret exposure |
| **Prompt Injection Detection** | 3 pattern detectors (unicode tricks, random chars, repetition) |
| **Kill-on-Violation** | Terminate compromised containers via RPC *(post-hoc: detected via exit code, not reactive)* |
| **USB Security Validation** | 2 security tests for ShadowMap gatekeeper and vault hold-to-reveal |
| **E2EE Messaging** | All communication via Matrix protocol with Megolm encryption |
| **Container Isolation** | Each agent runs in hardened Docker container |
| **Human-in-the-Loop** | Mobile approval for sensitive operations (payments, PII) |
| **SQLCipher Keystore** | Hardware-bound encrypted credential storage |
| **Split-Storage RAG** | Document chunks stored separately from vector embeddings |
| **YARA Content Disarm** | Malicious content detected and neutralized before processing |
| **TTL Proxy Guard** | Ephemeral tokens (30 min TTL) for sidecar communication |
| **Jetski CDP Proxy** | Tethered Mode browser proxy with PII scrubbing and encrypted sessions |

### Security & Trust Packages

| Package | Purpose |
|---------|---------|
| `pkg/pii/` | BlindFill engine for secure PII injection |
| `pkg/keystore/` | SQLCipher encrypted credential storage |
| `pkg/trust/` | Zero-trust device verification |
| `pkg/security/` | Website guard and security policies |
| `pkg/enforcement/` | License validation and enforcement |
| `pkg/lockdown/` | Admin reset mode |
| `pkg/yara/` | YARA-based content disarm and reconstruction scanner |
| `pkg/securerandom/` | Cryptographically secure random number generation |
| `pkg/crypto/` | Olm/Megolm E2EE engine (CryptoEngine), SAS verification, cross-signing, SQLCipher-backed key store |

---

## Rust Vault

### Purpose

The Rust Vault is a **security-hardened cryptographic enclave** that provides heavy I/O operations for ArmorClaw with enhanced security features. It implements:

- **State Bifurcation** - Separate persistent secrets (vault.db) from ephemeral crypto state (matrix_state.db)
- **Network-Layer BlindFill** - Inject secrets at network layer via Chrome DevTools Protocol
- **gRPC Governance** - Ephemeral token lifecycle management with zeroization
- **Zeroization** - All secrets zeroized in memory after use
- **mTLS Authentication** - gRPC over Unix domain sockets with certificate validation

### Runtime Model: Deployed Service (v0.6.0)

> **Updated in v0.6.0**: The Rust Vault is now a **deployed Docker service** with its own binary entrypoint, hardened container, and docker-compose configuration. It communicates with the Go Bridge via Unix domain socket IPC over a shared volume.

**Binary entrypoint**: `rust-vault/src/main.rs` (28 lines) registers the gRPC governance service and starts the Tokio runtime.

**Cargo.toml** `[[bin]]` section: `name = "armorclaw-vault"`

**Docker build** (multi-stage hardened):
- `network_mode: none` at build time for dependency fetch
- Runtime user UID 10001 (non-root)
- `cap_drop: ALL`, `read_only: true`, `no-new-privileges: true`

**docker-compose** service: `armorclaw-vault` shares `/run/armorclaw/` volume with the bridge for Unix socket IPC (`rust-vault.sock`).

This means:
- The Rust Vault **runs as a standalone process** alongside the Bridge in production
- There is **no runtime port conflict** with Jetski (see below) — communication is Unix socket only
- The `blindfill` module was **removed in v0.9.0** (commit 1563260) — superseded by Jetski CDP proxy
- The governance gRPC service (`rust-vault/src/governance/`) is activated when the v6 microkernel flag is enabled

### Relationship to Jetski Browser Sidecar

The Rust Vault's blindfill module (Phase 1 CDP interception) was removed in v0.9.0. Jetski is now the sole CDP security layer.

| Aspect | Rust Vault (historical) | Jetski CDP Proxy |
|--------|--------------------------|-----------------|
| **Type** | Library (removed v0.9.0) | Standalone Go binary (Docker container) |
| **What it did** | Generated `Fetch.enable` params, resolved `{{VAULT:field:hash}}` placeholders | Full WebSocket proxy between agent and Lightpanda engine |
| **Port usage** | None (library, no listener) | Listens on 9222 (CDP), 9223 (RPC) |
| **PII handling** | Placeholder format validation | Active CDP message-level PII scrubbing |
| **Runtime state** | Removed in commit 1563260 | Deployed via `docker-compose.jetski.yml` |

> **Note**: The Go-side BlindFill engine (`bridge/pkg/pii/`) remains active and handles placeholder resolution at the Bridge level. The `{{VAULT:field:hash}}` format is still used for PII injection workflows.

### Architecture

```
┌───────────────────────────────────────────────────────────────────────┐
│                         THE VPS (Office)                              │
│                                                                       │
│  ┌─────────────┐      ┌─────────────┐      ┌─────────────┐           │
│  │ ArmorClaw   │◀────▶│  Rust Vault   │      │  Jetski     │           │
│  │ Bridge      │ gRPC │  (Sidecar)    │      │  CDP Proxy  │           │
│  │ (Orchestr.) │ Unix │  (Keystore +  │      │  + Browser  │           │
│  │             │      │   Governance) │      │             │           │
│  └──────┬──────┘      └──────────────┘      └──────┬──────┘           │
│         │                                          │                   │
│         │            Bridge-level BlindFill         │                   │
│         │            (Memory-Only, Go)              │                   │
│         │                                          │                   │
└─────────┼──────────────────────────────────────────┼───────────────────┘
          │                                          │
          │ Secure Matrix Tunnel (E2EE)              │
          │                                          │
┌─────────▼──────────────────────────────────────────▼───────────────────┐
│                         USER (Mobile)                                 │
│   ArmorChat App                                                      │
│   "Book a flight to NYC"  [Approve Credit Card] 🔐                   │
└───────────────────────────────────────────────────────────────────────┘
```

### Integration with ArmorClaw

**Go Bridge → Rust Vault:**
- gRPC over Unix Domain Socket (`/run/armorclaw/rust-vault.sock`)
- mTLS authentication with certificate validation
- Keystore proxy API for secret retrieval
- Rate limiting (100 requests/second) with atomic operations
- Concurrency limiting (10 concurrent requests)

> **Deprecated (v0.9.0):** This section describes the old architecture where Rust Vault handled CDP interception directly. Jetski is now the sole CDP security layer.
>
> **Rust Vault → Playwright/Browser (historical):**
> - Chrome DevTools Protocol (CDP) interception
> - Filters XHR and Fetch requests only (not wildcard)
> - Placeholder resolution: `{{VAULT:payment.card_number:a1b2c3d4e5f6}}`
> - Network-layer injection (secrets never reach agent)

**Security Features:**

1. **State Bifurcation**
   - `vault.db` - Persistent secrets (SQLCipher encrypted)
   - `matrix_state.db` - Ephemeral crypto state (SQLCipher encrypted)
   - Separate databases prevent cross-contamination

2. **Network-Layer BlindFill**
   - CDP interceptor filters by resourceType (XHR, Fetch only)
   - Placeholder format: `{{VAULT:name:a1b2c3d4e5f6}}` (flat lookups only)
   - Secrets injected at network layer, never accessible to agent
   - Zeroized immediately after request completes

3. **gRPC Security**
   - Unix domain socket with 0600 permissions
   - mTLS authentication (certificate validation)
   - Rate limiting: 100 req/s with atomic operations (no mutex)
   - Concurrency limiting: 10 concurrent requests with semaphore

4. **Memory Safety**
   - All secrets use `Zeroizing<String>` from zeroize crate
   - Secrets zeroized on drop
   - No secret caching beyond request lifecycle
   - No secret values in logs

5. **Key Derivation**
   - PBKDF2-HMAC-SHA512 with 256,000 iterations
   - 32-byte salt for each database
   - Compatible with Go Bridge implementation

6. **SQLCipher Configuration**
   - `cipher_plaintext_header_size=32` for performance
   - `synchronous=NORMAL` for durability
   - Separate encryption keys for vault.db and matrix_state.db

7. **Logging**
   - Basic logging only (no comprehensive observability)
   - No secret values in logs
   - No circuit breakers or advanced retry logic

### Configuration

**Environment Variables:**

```bash
# Rust Vault Configuration
RUST_VAULT_ENABLED=true
RUST_VAULT_SOCKET_PATH=/run/armorclaw/rust-vault.sock
RUST_VAULT_TLS_ENABLED=true
RUST_VAULT_TLS_CERT_PATH=/etc/armorclaw/rust-vault.crt
RUST_VAULT_TLS_KEY_PATH=/etc/armorclaw/rust-vault.key
RUST_VAULT_TLS_CA_PATH=/etc/armorclaw/ca.crt

# Rate Limiting
RUST_VAULT_RATE_LIMIT=100              # Requests per second
RUST_VAULT_BURST_SIZE=20               # Burst capacity

# Concurrency
RUST_VAULT_MAX_CONCURRENT=10           # Max concurrent requests

# BlindFill
RUST_VAULT_CDP_ENABLED=true            # Enable CDP interception
```

**Default Configuration:**

```rust
pub struct VaultConfig {
    // Socket Configuration
    pub keystore_socket_path: PathBuf,
    pub use_tls: bool,
    pub tls: Option<TlsConfig>,
    
    // Rate Limiting
    pub rate_limit: u32,           // Default: 100
    pub burst_size: u32,           // Default: 20
    
    // Concurrency
    pub max_concurrent: usize,     // Default: 10
}
```

### API Reference

**gRPC Methods (via Unix Socket):**

```protobuf
service Keystore {
    // Secret Management
    rpc StoreSecret(StoreSecretRequest) returns (StoreSecretResponse);
    rpc RetrieveSecret(RetrieveSecretRequest) returns (RetrieveSecretResponse);
    rpc DeleteSecret(DeleteSecretRequest) returns (DeleteSecretResponse);
    rpc ListSecrets(ListSecretsRequest) returns (ListSecretsResponse);
    
    // Matrix State
    rpc StoreMatrixState(StoreMatrixStateRequest) returns (StoreMatrixStateResponse);
    rpc RetrieveMatrixState(RetrieveMatrixStateRequest) returns (RetrieveMatrixStateResponse);
}
```

**CDP Interception:**

```json
{
  "method": "Fetch.enable",
  "params": {
    "patterns": [
      {
        "urlPattern": "*",
        "resourceType": "XHR",
        "requestStage": "Request"
      },
      {
        "urlPattern": "*",
        "resourceType": "Fetch",
        "requestStage": "Request"
      }
    ]
  }
}
```

### Testing

**Test Coverage: 33 tests (cargo test --lib)**

**Run Tests:**

```bash
cd rust-vault
cargo test --lib
cargo clippy -- -D warnings
```

### Security Considerations

**Guardrails Respected:**

- ✅ No wildcard URL patterns (resourceType filtering instead)
- ✅ No WebSocket interception
- ✅ No document.write() or innerHTML interception
- ✅ No comprehensive observability (basic logging only)
- ✅ No circuit breakers or advanced retry logic
- ✅ No secret caching beyond request lifecycle
- ✅ No secret values in logs
- ✅ No advanced placeholder features (conditionals, loops, nesting)

**Production Checklist:**

- [ ] Generate TLS certificates for mTLS
- [ ] Set Unix socket permissions to 0600
- [ ] Configure SQLCipher encryption keys
- [ ] Enable rate limiting and concurrency limits
- [ ] Test CDP interception with real browser
- [ ] Verify zeroization in memory dumps
- [ ] Audit logs for secret exposure

### Performance Characteristics

- **Memory**: ~2MB bounded for download streams
- **Rate Limiting**: 100 req/s with atomic operations
- **Concurrency**: 10 concurrent requests with semaphore
- **Key Derivation**: 256,000 iterations (compatible with Go Bridge)
- **Zeroization**: Immediate on drop, no caching
- **Socket**: Unix domain socket (0600 permissions)

### Troubleshooting

**Common Issues:**

1. **Socket Permission Denied**
   ```bash
   ls -la /run/armorclaw/rust-vault.sock
   # Should show: srw------- 1 root root 0 ... rust-vault.sock
   chmod 0600 /run/armorclaw/rust-vault.sock
   ```

2. **mTLS Authentication Failed**
   ```bash
   # Verify certificates exist
   ls -la /etc/armorclaw/rust-vault.{crt,key} /etc/armorclaw/ca.crt
   
   # Check certificate expiry
   openssl x509 -in /etc/armorclaw/rust-vault.crt -text -noout | grep "Not After"
   ```

3. **SQLCipher Key Derivation Mismatch**
   ```bash
   # Ensure PBKDF2-HMAC-SHA512 with 256,000 iterations
   # Check Go Bridge compatibility
   grep -r "PBKDF2" bridge/pkg/keystore/
   ```

4. **CDP Interception Not Working**
   ```bash
   # Verify CDP is enabled
   curl http://localhost:9222/json/list
   
    # Check resourceType filtering
    # Should only intercept XHR and Fetch requests
    ```

---

## BlindFill Security

### BlindFill™ Secret Injection

**Core Principle**: Agents request PII by reference name, never see actual values. Secrets are injected directly into browser/containers via memory-only methods.

**Flow Architecture:**
```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   Agent     │────▶│ Approval     │────▶│   PII       │
│ Requests    │     │ Engine       │     │ Injector    │
│ "card_num"  │     │ Evaluate     │     │ (Socket)    │
└─────────────┘     │ Policy       │     └─────────────┘
                    │ Returns:     │            │
                    │ ["card_num"] │            │
                    └──────────────┘            │
                                                  ▼
                                           ┌─────────────┐
                                           │   Browser/  │
                                           │ Container   │
                                           │ Receives    │
                                           │ 4242...     │
                                           │ (not agent) │
                                           └─────────────┘
```

**Injection Methods:**

1. **Unix Domain Socket** (Primary, memory-only):
   - Path: `/run/armorclaw/pii/{container}.pii.sock`
   - Permissions: 0600 (owner only)
   - TTL: 5 seconds
   - Socket deleted after delivery

2. **Environment Variables** (Fallback):
   - Prefix: `PII_`
   - Format: `PII_{field_name}={value}`
   - Warning: May be visible in process listings

### BlindFill Placeholder System

The Rust Vault enforces **strict placeholder masking** to prevent agents from ever seeing real secret values. This is a critical security feature for defending against prompt injection attacks.

#### Placeholder Format

**Strict Format**: `{{VAULT:field:hash}}`

- **VAULT:** - Required prefix (case-sensitive)
- **field** - Secret identifier (e.g., `payment.card_number`, `user.email`)
- **hash** - Lowercase hexadecimal hash (e.g., `a1b2c3d4e5f6`)

**Examples**:
```
{{VAULT:payment.card_number:a1b2c3d4e5f6}}
{{VAULT:user.email:f7e8d9c0b1a2}}
{{VAULT:api.stripe_key:3d4e5f6a7b8c}}
```

#### Security Guarantees

1. **Real Values Never Exposed**:
   - Agents only see placeholders, never actual secrets
   - Real values injected at network layer via CDP
   - Secrets zeroized immediately after injection

2. **Strict Validation**:
   - Case-sensitive `VAULT:` prefix required
   - Lowercase hexadecimal hash only
   - No whitespace, nested placeholders, or conditionals
   - Old formats (e.g., `{{secret:...}}`) explicitly rejected

3. **Prompt Injection Defense**:
   - Placeholder format prevents adversarial prompts from accessing secrets
   - No support for conditionals (`if/else/endif`)
   - No support for loops (`for/endfor`)
   - No support for nested placeholders

4. **Placeholder Resolution Flow**:
   ```
   Agent Request → Placeholder → CDP Interceptor → Real Value → Browser Form
                  (agent sees)    (network layer)  (injected)    (filled)
   ```

#### Implementation Details

**Go BlindFill Engine** (`bridge/pkg/pii/`):
- Validates strict `{{VAULT:field:hash}}` format
- Rejects malformed placeholders with clear error messages
- Prevents injection attacks via field/hash manipulation
- Resolves placeholders to real values from SQLCipher keystore
- Values injected at browser form level via Jetski CDP proxy

#### Use Cases

1. **Payment Processing**:
   - Agent requests: `{{VAULT:payment.card_number:abc123}}`
   - Browser receives: `4242 4242 4242 4242`
   - Agent never sees: Real card number

2. **Form Filling**:
   - Agent requests: `{{VAULT:user.email:def456}}`
   - Browser receives: `user@example.com`
   - Agent never sees: Real email address

3. **API Authentication**:
   - Agent requests: `{{VAULT:api.stripe_key:ghi789}}`
   - Browser receives: `sk_live_...`
   - Agent never sees: Real API key

#### Error Handling

**Invalid Placeholder Examples**:
```
{{secret:payment.card}}          ❌ Wrong prefix (must be VAULT:)
{{VAULT:payment.card:ABC123}}    ❌ Uppercase hash (must be lowercase)
{{VAULT:payment.card:abc}}       ❌ Invalid hash length
{{ VAULT:payment.card:abc123 }}  ❌ Whitespace not allowed
{{VAULT:{{nested}}:abc123}}      ❌ Nested placeholders not allowed
{{VAULT:payment.card:abc123}}    ✅ Valid
```

---

## PII Masking

### PII Approval Workflow

**States:**
- `pending` — Awaiting user approval (default: 5 min TTL)
- `approved` — User approved specific fields
- `denied` — User denied request
- `expired` — Request timed out
- `cancelled` — Agent cancelled request
- `fulfilled` — Approved data delivered

**PII Request Structure:**
```go
type PIIRequest struct {
    ID              string
    AgentID         string
    SkillID         string
    ProfileID       string
    RequestedFields []PIIFieldRequest
    Context         string              // Reason shown to user
    RoomID          string              // Matrix room for events
    Status          PIIRequestStatus
    CreatedAt       time.Time
    ExpiresAt       time.Time           // Default: +5 min
    ApprovedFields  []string
    ApprovedBy      string
    DeniedReason    string
}

type PIIFieldRequest struct {
    Key         string
    DisplayName string
    Required    bool
    Sensitive   bool
}
```

**Approval Engine Decision Types:**
- `DecisionAllow` — Auto-approve
- `DecisionDeny` — Block
- `DecisionRequireApproval` — Ask user

### Prompt Injection Detection

ArmorClaw includes **real-time prompt injection detection** to defend against adversarial attacks like those pioneered by "Pliny the Prompter". The system detects non-linguistic noise patterns and flags suspicious sessions for human intervention.

#### Detection Patterns

| Pattern | Detection Method | Examples |
|---------|-----------------|----------|
| **Unicode Tricks** | Zero-width chars, combining diacritics, homoglyphs | `H̵̭̓ ELLO`, `\u200B`, Cyrillic lookalikes |
| **Random Characters** | Shannon entropy >3.4 bits + >50% non-alphanumeric | `asdf1234!@#$`, `xk29!@#mz84` |
| **Repetition** | 8+ consecutive chars, repeated sequences | `aaaaaaaa`, `testtesttesttest` |

#### Implementation

**Location**: `container/openclaw-src/src/gateway/injection-detection.ts`

```typescript
interface DetectionResult {
  isSuspicious: boolean;
  reasons: DetectionReason[];  // "unicode_tricks" | "random_chars" | "repetition"
}

function detectPromptInjection(text: string): DetectionResult;
```

#### Integration Points

- **Rate Limiting**: Integrated with `control-plane-rate-limit.ts`
- **Security Logging**: Flagged sessions logged with reason codes
- **Sentinel Mode**: Hook point available for human intervention alerts

#### Performance

- **Latency**: <1ms per detection
- **Complexity**: O(n) where n = message length
- **False Positives**: Tested against 5 legitimate message patterns

### Hardening State Management

**Mandatory Steps** (all must be true for `delegation_ready`):
```go
type UserHardeningState struct {
    UserID           string
    PasswordRotated  bool   // Changed initial password
    BootstrapWiped   bool   // Cleaned temp files
    DeviceVerified   bool   // Device is trusted
    RecoveryBackedUp bool   // Recovery keys backed up
    BiometricsEnabled bool   // Optional
    DelegationReady  bool   // Computed: all mandatory steps complete
}
```

### USB Security Validation Suite

ArmorClaw includes a **security validation suite** for testing critical security controls via TAP-formatted output for CI/CD integration.

**Location**: `tools/skills/armorchat_usb_validate.sh`

#### Test Cases

| Test | Purpose | Validates |
|------|---------|-----------|
| `shadowmap_gatekeeper_blocks_api_key` | API keys blocked by gatekeeper | ShadowMap regex patterns |
| `vault_hold_to_reveal_requires_2s_and_biometric` | Timing and biometric enforcement | Vault security requirements |

#### Usage

```bash
# Run security validation suite
bash tools/skills/armorchat_usb_validate.sh --suite security

# Expected output (TAP format)
TAP version 13
1..2
ok 1 - shadowmap_gatekeeper_blocks_api_key - API keys are blocked by gatekeeper
ok 2 - vault_hold_to_reveal_requires_2s_and_biometric - Timing and biometric requirements enforced
```

#### CI Integration

- Exit code 0 = all tests pass
- TAP format compatible with most CI systems
- Can be extended with additional security tests

### Container Terminate RPC (Kill-on-Violation)

ArmorClaw provides a **kill-on-violation capability** via the `TerminateContainer` RPC method, allowing immediate termination of compromised or misbehaving agent containers.

**Location**: `bridge/pkg/rpc/container_handlers.go`

#### Method Signature

```go
// TerminateContainer immediately stops a running container
func (h *Handlers) handleTerminateContainer(req jsonrpc.Request) jsonrpc.Response {
    // Parameters:
    // - container_id: string (required) - Docker container ID
    // - user_id: string (required) - Requesting user for authorization
    //
    // Returns:
    // - success: bool - Whether termination succeeded
    // - error: string - Error message if failed
}
```

#### Security Checks

1. **Authentication**: Requires valid `user_id` parameter
2. **Container Ownership**: Verifies container has ArmorClaw labels
3. **Docker API**: Calls `ContainerKill()` with SIGKILL for immediate termination

#### Usage

```bash
# Via JSON-RPC
curl -X POST http://localhost:8443/rpc -d '{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "terminateContainer",
  "params": {
    "container_id": "abc123...",
    "user_id": "user@matrix.org"
  }
}'
```

#### Integration with Sentinel Mode

- Can be triggered automatically on security violation detection
- Integrates with prompt injection detection for automated response
- Audit logged via `EventSecurityViolation` event type

---

## Governor-Shield

**Package**: `bridge/pkg/governor/`

**Purpose**: DefaultSkillGate layer for PII interception in AI tool calls and prompts, preventing sensitive data from reaching external AI models.

**Core Components**:

| Component | Description |
|-----------|-------------|
| **Governor** | Main PII interceptor with config, scrubber, and mapping management |
| **Config** | Configuration for logging, scrubbing behavior, Shadow Mapping, and performance |
| **PIIMapping** | Placeholder to original value mapping for restoration |

#### Governor Configuration

| Field | Type | Default | Description |
|-------|------|----------|-------------|
| `LogViolations` | bool | true | Log PII violations to audit trail |
| `LogMaskedPII` | bool | true | Include masked snippets in logs (safe for audit) |
| `StrictMode` | bool | false | Block all tool calls with any PII detected |
| `UseShadowMapping` | bool | true | Use Shadow Mapping (SHA256 hash-based placeholders) |
| `PlaceholderPrefix` | string | "[REDACTED:" | Prefix for placeholders |
| `MaxConcurrentCalls` | int | 100 | Maximum concurrent tool calls |
| `CacheMappings` | bool | true | Cache PII mappings for performance |

#### Core Methods

| Method | Purpose |
|--------|---------|
| `InterceptToolCall()` | Scrubs PII from tool call arguments using Shadow Mapping |
| `InterceptPrompt()` | Scans and redacts PII from user prompts before AI model |
| `RestoreOutput()` | Restores redacted PII placeholders in AI output using PIIMapping |
| `ValidateArgs()` | Validates tool call arguments for PII violations without modifying |

#### Shadow Mapping Implementation

**Process**:
1. Detect PII patterns using `bridge/pkg/pii` scrubber
2. Compute SHA256 hash of detected PII value
3. Replace with placeholder: `[REDACTED:{8-char-hash}]`
4. Store mapping: placeholder → original value
5. Restore in output before returning to user

**Benefits**:
- AI never sees raw PII values
- Reversible for legitimate use cases
- Audit trail with masked snippets
- Pattern-aware severity classification

#### Severity Classification

| Severity | Patterns | Examples |
|----------|-----------|-----------|
| **critical** | credit_card, aws_secret, aws_key_id, api_key (sk/pk/ai) | Payment cards, AWS credentials, API keys |
| **high** | ssn, github_token | Social Security Numbers, GitHub tokens |
| **medium** | email, phone, ip_address, bearer_token, token, secret, password | Contact info, auth tokens |
| **low** | All other patterns | Default classification |

#### Integration Points

| Component | Integration Point |
|-----------|------------------|
| **MCP Router** | `bridge/pkg/mcp/router.go` - Tool call PII gate |
| **PII Scrubber** | `bridge/pkg/pii/` - Detection patterns and redaction logic |

#### Usage Example

```go
// Initialize Governor with config
governor := NewGovernor(&Config{
    LogViolations:      true,
    UseShadowMapping:   true,
    PlaceholderPrefix:  "[REDACTED:",
    MaxConcurrentCalls: 100,
    CacheMappings:      true,
}, logger)

// Intercept tool call
scrubbedCall, err := governor.InterceptToolCall(ctx, &ToolCall{
    ToolName: "search_web",
    Arguments: map[string]interface{}{
        "query": "Call 555-123-4567 for John Doe",
    },
})

// scrubbedCall.Arguments["query"] = "Call [REDACTED:a1b2c3d4] for [REDACTED:e5f6g7h8]"
```

#### Audit Logging

Governor logs all PII violations with:
- Tool name and argument key
- Masked snippet (first 2 + *** + last 2 chars)
- Pattern types detected
- Severity classification

**Example Log Entry**:
```
WARN PII violation detected in tool_call tool=search_web key=query violations=2 masked_snippet=55********67 pattern_types=[phone, name]
```

---

## Keystore Security

### Purpose

The keystore provides **zero-knowledge encrypted credential storage** using SQLCipher with hardware-bound master keys. It enables:
- Secure API key storage (never persisted to disk as plaintext)
- BlindFill™ secret injection (agents never see raw values)
- Hardware binding (database useless if stolen)
- Zero-touch reboot (no password required)

### Database Schema

**Database Path**: `/var/lib/armorclaw/keystore.db` (encrypted)
**Encryption**: SQLCipher with XChaCha20-Poly1305 AEAD

```sql
-- API Credentials
CREATE TABLE credentials (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,                    -- openai, anthropic, cloudflare, etc.
    token_encrypted BLOB NOT NULL,             -- XChaCha20-Poly1305 encrypted
    nonce BLOB NOT NULL,                       -- AEAD nonce
    base_url TEXT,                             -- Custom endpoint
    display_name TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER,                        -- Token expiry (optional)
    tags TEXT                                  -- JSON array
);

CREATE INDEX idx_provider ON credentials(provider);
CREATE INDEX idx_expires_at ON credentials(expires_at);

-- User Profiles (BlindFill PII)
CREATE TABLE user_profiles (
    id TEXT PRIMARY KEY,
    profile_name TEXT NOT NULL,
    profile_type TEXT NOT NULL DEFAULT 'personal',
    data_encrypted BLOB NOT NULL,              -- JSON-serialized PII (encrypted)
    data_nonce BLOB NOT NULL,
    field_schema TEXT NOT NULL,                -- JSON schema of fields
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    last_accessed INTEGER,
    is_default INTEGER DEFAULT 0
);

CREATE INDEX idx_profile_type ON user_profiles(profile_type);
CREATE INDEX idx_profile_default ON user_profiles(is_default);

-- Matrix Refresh Tokens
CREATE TABLE matrix_refresh_tokens (
    id TEXT PRIMARY KEY,
    token_encrypted BLOB NOT NULL,
    nonce BLOB NOT NULL,
    homeserver_url TEXT NOT NULL,
    user_id TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

-- Hardening State
CREATE TABLE hardening_state (
    user_id TEXT PRIMARY KEY,
    password_rotated INTEGER DEFAULT 0,
    bootstrap_wiped INTEGER DEFAULT 0,
    device_verified INTEGER DEFAULT 0,
    recovery_backed_up INTEGER DEFAULT 0,
    biometrics_enabled INTEGER DEFAULT 0,
    delegation_ready INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Hardware Binding
CREATE TABLE hardware_binding (
    signature_hash TEXT PRIMARY KEY,
    bound_at INTEGER NOT NULL,
    entropy_sources TEXT NOT NULL             -- JSON of sources used
);

-- Metadata
CREATE TABLE metadata (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

### Key Derivation Hierarchy

**Priority order for master key source:**
1. `ARMORCLAW_KEYSTORE_SECRET` environment variable (base64-encoded 32 bytes)
2. `keystore.db.key` file (base64-encoded)
3. Container-persisted random key
4. Hardware-derived key (fallback)

**Hardware Entropy Sources:**
```go
// CollectEntropy() gathers from:
1. /etc/machine-id, /var/lib/dbus/machine-id
2. /sys/class/dmi/id/product_uuid (SMBIOS)
3. Primary MAC address (first non-loopback)
4. Hostname
5. OS/Architecture (runtime.GOOS, runtime.GOARCH)
6. /proc/cpuinfo (model name, vendor_id)
```

### Encryption Configuration

```go
const (
    saltLength       = 32
    pbkdf2Iterations = 256000  // SQLCipher default
    keyLength        = 32
    cipherPageSize   = 4096
    cipherKdfIter    = 256000
    cipherHmacAlg    = "HMAC_SHA512"
    cipherKdfAlgorithm = "PBKDF2_HMAC_SHA512"
)
```

**Connection String:**
```
file:keystore.db?_pragma_key=x'hex_master_key'&_pragma_cipher_page_size=4096&_pragma_kdf_iter=256000&_pragma_cipher_hmac_algorithm=HMAC_SHA512&_pragma_cipher_kdf_algorithm=PBKDF2_HMAC_SHA512&_foreign_keys=ON
```

### Supported Providers

```go
const (
    ProviderOpenAI     Provider = "openai"
    ProviderAnthropic  Provider = "anthropic"
    ProviderCloudflare Provider = "cloudflare"
    ProviderDeepSeek   Provider = "deepseek"
    ProviderGoogle     Provider = "google"
    ProviderGroq       Provider = "groq"
    ProviderMoonshot   Provider = "moonshot"
    ProviderNvidia     Provider = "nvidia"
    ProviderOllama     Provider = "ollama"
    ProviderOpenRouter Provider = "openrouter"
    ProviderXAI        Provider = "xai"
    ProviderZhipu      Provider = "zhipu"
)
```

### Environment Fallback

`Retrieve()` checks environment variables first:
- `OPENROUTER_API_KEY`
- `ZAI_API_KEY`
- `OPEN_AI_KEY`

---

## Container Hardening

### OpenClaw Agent Container Security

Each agent runs inside an isolated Docker container with maximum security restrictions:

```yaml
security_opt:
  - no-new-privileges:true
  - seccomp:seccomp-profile.json
  - apparmor:armorclaw-agent
cap_drop:
  - ALL
read_only: true
pids_limit: 100
memory: 512M
```

**Docker build** (multi-stage hardened):
- `network_mode: none` at build time for dependency fetch
- Runtime user UID 10001 (non-root)
- `cap_drop: ALL`, `read_only: true`, `no-new-privileges: true`

### Agent Communication Isolation

Agent containers always run with `NetworkMode: "none"` (no network access). Structured results are passed via `result.json` in the bind-mounted state dir (backward channel). Browser automation runs through the Jetski sidecar, a separate container with its own network stack; agent containers never perform browser operations directly.

> ⚠️ **CRITICAL**: Agent containers cannot browse the web directly. All browser automation goes through the Jetski sidecar. The Bridge brokers communication between the isolated agent container and the networked Jetski sidecar.

### ToolSidecar Isolation (v6-gated)

`Provisioner.SpawnToolSidecar()` creates hardened containers (NetworkMode: none, readonly, cap-drop ALL, 512MB memory). Tool execution uses Docker exec API for real command execution inside the container. `StopToolSidecar()` tears them down. Gated behind v6 microkernel flag (default: `V6Microkernel=false`).

```go
type ToolSidecar struct {
    ID        string
    SkillName string
    SessionID string
    CreatedAt time.Time
    Status    string
}
```

---

## Audit Logging

### Three-Tier Audit System

#### Tier 1: Basic Audit
```go
type Entry struct {
    Timestamp   time.Time
    EventType   EventType
    SessionID   string
    RoomID      string
    UserID      string
    Details     interface{}
}
```

#### Tier 2: Compliance Audit
```go
type ComplianceEntry struct {
    ID           string
    Timestamp    time.Time
    EventType    EventType
    UserID       string
    Source       string          // Component
    IPAddress    string
    UserAgent    string
    Action       string          // create, read, update, delete
    Resource     string
    Status       string          // success, failure, denied
    PreviousHash string          // Hash chain
    EntryHash    string
}
```

**Compliance Levels:**
- `standard` — 30-day retention
- `extended` — 90-day retention
- `full` — 1-year retention
- `hipaa` — 6-year retention

#### Tier 3: Tamper-Evident Audit
```go
type TamperEvidentEntry struct {
    Sequence     int64
    Timestamp    time.Time
    EventType    string
    Actor        Actor
    Action       string
    Resource     Resource
    Hash         string
    PreviousHash string
    Signature    string          // Optional: high-security mode
    Compliance   ComplianceFlags
}
```

### NavChart Audit Trail

**File**: `bridge/pkg/browser/chart_audit.go`

The `chart_audit` table tracks 5 chart lifecycle events:

| Event | When |
|-------|------|
| `created` | New chart saved |
| `updated` | Chart version updated |
| `replayed` | Chart replayed (success or failure) |
| `rejected` | Validation rejected a chart |
| `deleted` | Chart removed |

Audit details never contain PII values. Only placeholder references, domain, and outcome are logged.

---

## Zero-Trust Architecture

### Zero-Trust Device Verification

**Trust Score Calculation:**
- Base score from verification count, device status, IP history
- Anomalies add: +30 (new device), +20 (unverified), +15 (unknown IP), +25 (>3 failures)

**Device States:**
```go
const (
    StateUnverified        = "unverified"
    StatePendingApproval   = "pending_approval"
    StateAwaitingSecondFactor = "awaiting_second_factor"
    StateVerified          = "verified"
    StateRejected          = "rejected"
    StateExpired           = "expired"
)
```

**Verification Methods:**
- `admin_approval` — Admin must manually approve
- `second_factor` — Existing device confirms
- `wait_period` — Auto-approve after delay
- `automatic` — Not recommended

### E2EE Messaging

All communication between ArmorChat and the Bridge uses the Matrix protocol with Megolm end-to-end encryption.

### Matrix E2EE Implementation (v0.9.0)

The Bridge implements full Matrix end-to-end encryption using mautrix-go's `crypto` package:

| Component | File | Description |
|-----------|------|-------------|
| CryptoEngine | `pkg/crypto/engine.go` | OlmMachine wrapper with nil-safe access |
| EncryptionService | `pkg/crypto/encryption.go` | Dual-mode encrypt/decrypt with RoomEncryptionCache |
| SASVerificationService | `pkg/crypto/verification.go` | SAS emoji verification (64 emoji, HMAC-SHA256 commitment) |
| CrossSigningService | `pkg/crypto/crosssign.go` | Cross-signing bootstrap with Ed25519 key gen and UIAA |
| KeyExchangeService | `pkg/crypto/key_exchange.go` | Device key upload, query, and claim |
| Crypto Store | `pkg/crypto/keystore_store.go` | SQLCipher v2 schema with 5 tables |
| Kill Switch | `pkg/config/config.go` | `matrix.e2ee.enabled` (default false), runtime toggle, audit log |

**Build modes:** `-tags goolm` (pure Go, default) or `-tags libolm` (CGO fallback)
**Key storage:** SQLCipher keystore with v2 schema migration (additive, preserves existing data)
**Rollback:** Kill switch allows instant E2EE disable without restart via `bridge.e2ee_disable` RPC

### TLS Metadata Fixes (v0.9.0)

Three TLS bugs were fixed in the QR provisioning payload:

1. **Mode derivation** — `updateQRTLSInfo()` now calls `deriveTLSMode()` instead of hardcoding `"private"`. Native mode returns `"none"`, sentinel+self-signed returns `"private"`, sentinel+CA returns `"public"`.
2. **HMAC v2 signature** — `signConfig()` v2 now includes `TLSTrustHint` and `CertExpiresAt` in the HMAC input. Tampered TLS fields are detected by `ValidateConfig()`.
3. **Well-known enrichment** — `/.well-known/matrix/client` returns 4 TLS fields: `tls_mode`, `tls_fingerprint_sha256`, `tls_trust_hint`, `cert_expires_at`.

### mTLS Authentication (Rust Vault)

The Rust Vault uses gRPC over Unix domain sockets with mTLS authentication and certificate validation for all inter-service communication.

### HMAC Token Validation (Sidecars)

Both the Python MarkItDown sidecar and Java Apache POI sidecar use HMAC-SHA256 token validation for authentication. Tokens have a 30-minute TTL with 5-minute max age for replay prevention.

---

## Secure Document Pipeline

### Purpose

Phase 2 added a **secure document processing pipeline** to ArmorClaw, providing enterprise-grade document handling with security controls at every stage. It implements:

- **Split-Storage RAG** - Documents are split into chunks; embeddings stored separately from content in Qdrant
- **YARA Content Disarm & Reconstruct (CDR)** - Malicious content detected and neutralized before processing
- **TTL Proxy Guard** - Ephemeral authentication tokens (30-minute TTL) for sidecar communication

### Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  Document   │────▶│  YARA CDR   │────▶│   Split     │
│  Ingestion  │     │  Scanner     │     │   Storage   │
└─────────────┘     └──────────────┘     └──────┬──────┘
                                                │
                                     ┌──────────┴──────────┐
                                     ▼                     ▼
                              ┌─────────────┐      ┌─────────────┐
                              │  Qdrant     │      │  Content    │
                              │  Embeddings │      │  Store      │
                              │  (vectors)  │      │  (chunks)   │
                              └─────────────┘      └─────────────┘
```

### Components

#### Split-Storage RAG

| Feature | Description |
|---------|-------------|
| **Chunking** | Documents split into semantically meaningful chunks |
| **Embedding Separation** | Vector embeddings stored in Qdrant, content stored separately |
| **Retrieval** | Embedding similarity search retrieves relevant chunks |
| **Provider** | Uses OpenAI embeddings (no ONNX migration) |

#### YARA Content Disarm & Reconstruct

| Feature | Description |
|---------|-------------|
| **Rule Matching** | YARA rules scan incoming documents for malicious patterns |
| **Disarm** | Detected threats neutralized before processing |
| **Reconstruct** | Safe content reconstructed for downstream use |
| **Integration** | Requires CGO + libyara-dev |

#### TTL Proxy Guard

| Feature | Description |
|---------|-------------|
| **Token TTL** | 30-minute ephemeral tokens |
| **Validation** | HMAC-SHA256 signatures |
| **Replay Prevention** | Timestamp validation (5-minute max age) |
| **Scope** | Sidecar communication only |

### Security Guarantees

- ✅ No persistent credentials in sidecar memory
- ✅ No credential caching beyond request lifecycle
- ✅ All document processing logged in Go Bridge audit trail
- ✅ PII interception before sidecar calls
- ✅ YARA rules validated before deployment
- ✅ TTL tokens cannot be reused after expiry

---

## v6 Microkernel Governance

### Purpose

The v6 microkernel is a **governance layer** that adds ephemeral token lifecycle management, vault governance, and tool isolation to ArmorClaw. It is fully implemented in code but **disabled by default**. In v0.6.0, it operates in audit-only mode when enabled.

### Activation

```bash
# Enable v6 microkernel (requires Rust Vault governance service running)
export ARMORCLAW_V6_MICROKERNEL=true

# Or via TOML configuration
[vault]
v6_microkernel = true
socket_path = "/run/armorclaw/rust-vault.sock"
```

**Default**: `v6_microkernel = false` (see `bridge/pkg/config/config.go:990`)

### Architecture (v6 mode)

```
┌───────────────────────────────────────────────────────────────────────┐
│                       v6 Microkernel Architecture                     │
│                                                                       │
│  ┌─────────────┐     ┌──────────────┐     ┌──────────────────────┐   │
│  │ MCP Router  │────▶│ SkillGate    │────▶│ ToolSidecar          │   │
│  │ (router.go) │     │ (PII check)  │     │ (isolated container) │   │
│  └──────┬──────┘     └──────────────┘     └──────────────────────┘   │
│         │                                  ▲ v6Microkernel=true     │
│         │                                  │                         │
│         ▼                                  │                         │
│  ┌──────────────┐   gRPC/Unix   ┌─────────┴──────────┐              │
│  │ Vault Client │──────────────▶│ Rust Vault          │              │
│  │ (proto/)     │              │ Governance Service   │              │
│  └──────────────┘              │ - IssueEphemeralTok │              │
│                                │ - ConsumeEphemeral  │              │
│                                │ - ZeroizeToolSecrets│              │
│                                │ - SubscribeEvents   │              │
│                                └────────────────────┘              │
└───────────────────────────────────────────────────────────────────────┘
```

### Audit Mode (v0.6.0)

When v6 is enabled, it operates in **audit-only mode** by default. This logs what *would* happen without actually intercepting tool calls:

- Requires **both** `V6Microkernel=true` **and** `V6AuditMode=true` in VaultConfig
- Logs PII violations detected by SkillGate
- Logs governance checks that would block or redirect tool calls
- Logs would-be ToolSidecar spawns (no containers are actually created)
- **ToolSidecar communication protocol** is a hard prerequisite for enforcement mode. Until that protocol ships, audit mode is the safe default.
- Source: `bridge/pkg/mcp/router.go:handleAuditMode()`, `bridge/pkg/config/config.go`

#### MCP Router (`bridge/pkg/mcp/router.go`)

Routes all MCP `tools/call` requests through a security pipeline:

1. **SkillGate validation** — PII interception and redaction
2. **HITL consent workflow** — Human approval for PII operations
3. **ToolSidecar provisioning** — Isolated execution via Docker exec API (when v6 enabled)
4. **Vault governance** — Ephemeral token issuance + zeroization (when v6 enabled)
5. **Audit logging** — Compliance trail

> **vaultClient wiring**: `vaultClient` is passed directly to `setupMCPRouter()` in `bridge/cmd/bridge/setup_mcp.go`, which forwards it to `mcp.New()` config. The previous gap where vaultClient was nil is now closed.

```go
type MCPRouter struct {
    skillGate     interfaces.SkillGate
    consentMgr    *pii.HITLConsentManager
    auditor       *audit.AuditLog
    translator    *translator.RPCToMCPTranslator
    vaultClient   VaultClient    // wired via setupMCPRouter, nil when v6_microkernel=false
    v6Microkernel bool           // false by default
}
```

#### Vault Governance Client (`bridge/pkg/vault/proto/`)

Generated gRPC client stubs from `governance.proto`. Provides four methods:

| Method | Purpose |
|--------|---------|
| `IssueEphemeralToken` | Create short-lived token granting scoped secret access |
| `ConsumeEphemeralToken` | One-time use — token invalidated after consumption |
| `ZeroizeToolSecrets` | Securely erase all in-memory secrets for a tool/session |
| `SubscribeEvents` | gRPC server stream for governance events |

**Token lifecycle**: Issue → Consume (one-time) → Expire (TTL) → Zeroize

### Behavior: v6 On vs Off

| Aspect | v6 Microkernel OFF (default) | v6 Microkernel ON |
|--------|----------------------------|-------------------|
| **Vault governance** | Skipped entirely | Active — ephemeral tokens, zeroization |
| **Tool isolation** | Skills execute in-process | ToolSidecar containers (SpawnToolSidecar) |
| **Secret access** | Direct keystore retrieval | Vault-issued ephemeral tokens |
| **Event streaming** | No governance events | gRPC stream from Rust Vault |
| **Backward compat** | Full v4.x behavior | Enhanced security model |

### Test Coverage

4 dedicated tests in `bridge/pkg/mcp/router_test.go`:
- `TestExecuteTool_V6MicrokernelIssuesAndZeroizes` — verifies token issuance + zeroization
- `TestExecuteTool_V6MicrokernelOffSkipsVault` — verifies vault bypass when disabled
- Edge case tests for token lifecycle and consent integration

### Relationship to v4.x Documentation

This section documents code that exists in the repository but is **not active** in the current v4.8.0 release. The rest of this document describes the active v4.x architecture. When `v6_microkernel` is enabled, the MCP Router adds the governance layer described here on top of the existing v4.x components.

---

## License System

### Overview

The ArmorClaw license system controls access to premium features like platform bridging, compliance modes, and advanced security. It has three parts: a standalone microservice that manages license records, a client library inside the Bridge that caches validation results and handles offline scenarios, and an enforcement layer that gates features by tier.

The system is designed to be resilient. If the license server becomes unreachable, the Bridge keeps operating using cached results for a configurable grace period. This means temporary network blips don't take down production deployments.

### Architecture

```
┌──────────────────────┐
│   license-server     │  Standalone Go microservice (Docker)
│   (PostgreSQL)       │  Owns the license database, handles
│                      │  validation, activation, admin ops
└──────────┬───────────┘
           │ HTTP (JSON API)
           ▼
┌──────────────────────┐
│  bridge/pkg/license  │  Client library inside the Bridge
│  Client              │  Caches validations, offline-first
│  StateManager        │  Runs periodic polls, tracks state
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ bridge/pkg/enforcement│ Feature gating by tier
│ Manager              │  Platform limits, compliance modes
│ BridgeEnforcer       │  Hooks into bridge lifecycle
│ BridgeHook           │
└──────────────────────┘
```

The license server runs as its own Docker container. The Bridge communicates with it over HTTP. On startup, the Bridge validates its license, caches the result, and then rechecks periodically in the background.

### License Tiers

| Tier | Key | Default Instances | Bridging | Compliance |
|------|-----|--------------------|----------|------------|
| Free | `free` | 1 | Slack only (3 channels, 10 users) | Basic |
| Pro | `pro` | 3 | Slack, Discord, Teams (50 channels, 200 users) | Standard with PHI scrubbing |
| Enterprise | `ent` | 10 | All platforms, unlimited channels/users | Strict with HIPAA, tamper evidence |

### Key Packages

#### License Server (`license-server/`)

The license server is a standalone Go HTTP service backed by PostgreSQL. It auto-creates its schema on startup.

**Endpoints:**

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/v1/licenses/validate` | License key | Validate a license and optional feature |
| POST | `/v1/licenses/activate` | None | Register a bridge instance |
| GET | `/v1/licenses/status` | Admin token | Full license details with usage stats |
| POST | `/admin/v1/licenses` | Admin token | Create a new license |
| DELETE | `/admin/v1/licenses/{key}` | Admin token | Revoke a license |
| GET | `/health` | None | Health check (pings database) |

**Validation flow:**

1. The request includes a license key, instance ID, optional feature name, and version.
2. The server checks key format, rate limits the request, then queries the database.
3. It verifies the license exists, is active, and hasn't expired.
4. If a specific feature was requested, it checks whether that feature is included in the license.
5. It registers or updates the instance heartbeat.
6. Every validation is logged to the `validations` table for audit purposes.

**Activation flow:**

Activation uses a database transaction with `SELECT FOR UPDATE` row locking to prevent race conditions when multiple instances try to activate simultaneously. If the license has a `max_instances` limit and that limit is reached, the request is rejected with `INSTANCE_LIMIT_EXCEEDED`.

**Rate limits** are per-license-key, hourly, and vary by tier: Free gets 100 requests/hour, Pro gets 1,000, and Enterprise gets 10,000.

#### License Client (`bridge/pkg/license/`)

The client library runs inside the Bridge process. It has two main types: `Client` handles validation requests and caching, while `StateManager` handles runtime state tracking and polling.

##### Client

`Client` wraps HTTP calls to the license server with an offline-first caching strategy:

- On first call for a feature, it contacts the server, validates, and caches the result.
- Subsequent calls for the same feature return the cached result without a network round trip.
- If the server is unreachable, it falls back to the cache.
- Cache entries include a grace period (default 3 days) calculated from the server's reported expiration. During grace, the Bridge keeps running even though the license has technically expired.
- `OfflineMode` can be set to never contact the server, relying entirely on cached data.

The cache is keyed by feature name, so checking different features produces separate cache entries. A special `"license-info"` feature key fetches general license metadata without checking a specific feature.

##### StateManager

`StateManager` tracks the overall license state across the Bridge's lifetime. It defines these states:

| State | Behavior | Meaning |
|-------|----------|---------|
| `Valid` | Normal | License active, all features available |
| `GracePeriod` | Degraded or ReadOnly | License expired but within grace window |
| `Expired` | Blocked or ReadOnly | Grace period over, service paused |
| `Invalid` | Blocked | License revoked or malformed |
| `Unknown` | Degraded | Could not reach server on startup |

**Startup:** `Initialize()` calls the validator once. If the server is unreachable, it sets state to `Unknown` with `BehaviorDegraded`, so the Bridge can still run in a limited capacity.

**Polling:** `StartPolling()` launches a background goroutine that revalidates at a configurable interval (default 24 hours). When a state transition occurs (Valid to GracePeriod, GracePeriod to Expired), it fires an alert through the `AlertSender` interface.

**Alert thresholds** are configurable. The defaults fire warnings at 30, 14, 7, and 1 day before expiry.

**Operation gating:** `CanPerformOperation()` checks whether a given operation type (read, write, container create, admin access, etc.) is allowed under the current behavior. In `BehaviorDegraded`, admin and config change operations are blocked. In `BehaviorBlocked`, everything is blocked.

#### Enforcement (`bridge/pkg/enforcement/`)

The enforcement layer sits between the Bridge's business logic and the license client. It answers questions like "can I bridge to Discord?" and "should I scrub PHI on this message?"

##### Manager

`Manager` holds a registry of all known features, each tagged with a minimum tier, category, and compliance flag. On creation, it registers these features:

- **Bridging:** Slack (Free), Discord (Pro), Teams (Pro), WhatsApp (Enterprise)
- **Compliance:** PHI scrubbing (Pro), HIPAA mode (Enterprise), audit export (Pro), tamper evidence (Enterprise)
- **Security:** SSO (Pro), SAML (Enterprise), MFA enforcement (Pro), hardware keys (Enterprise)
- **Voice:** Calls (Free), recording (Enterprise), transcription (Enterprise)
- **Management:** Dashboard (Pro), REST API (Pro), webhooks (Pro), priority support (Enterprise)
- **Limits:** Unlimited bridges (Pro), unlimited users (Enterprise)

`CheckFeature()` returns whether a feature is allowed. If no license is loaded, it falls back to Free tier defaults. If the license is expired but grace mode is enabled (not strict), Free tier features stay available.

`GetComplianceMode()` derives the compliance level from the license tier and its feature flags:

| Compliance Mode | Trigger |
|-----------------|---------|
| `none` | No license |
| `basic` | Free tier or expired Pro |
| `standard` | Pro tier with PHI scrubbing feature |
| `full` | Enterprise tier (without HIPAA feature) |
| `strict` | Enterprise tier with HIPAA feature |

`GetPlatformLimit()` returns per-platform limits (channels, users, messages/day) adjusted for the current tier. Enterprise gets unlimited everything; Pro gets increased quotas; Free gets the base limits.

##### BridgeEnforcer and BridgeHook

`BridgeEnforcer` wraps the Manager with bridge-specific checks. `BridgeHook` provides lifecycle hooks that the Bridge's AppService integration calls at key moments:

- `BeforeBridgeStart()` checks that at least the Slack bridge feature is available (Free tier minimum).
- `BeforeAdapterStart(platform)` checks that bridging to a specific platform is allowed.
- `BeforeChannelBridge(platform, currentCount)` checks platform access and enforces channel count limits.
- `ShouldScrubPHI()` returns whether PHI scrubbing is active and at what compliance level.
- `ShouldAuditLog()` returns whether audit logging is required.
- `GetComplianceConfig()` bundles all compliance settings into a single config struct.

##### LicenseStatusHandler

`LicenseStatusHandler` exposes license state over RPC. It provides `GetStatus()` (full license info plus platform availability), `RefreshLicense()`, `CheckFeatureAccess()`, and `GetComplianceMode()`. This is how the RPC layer and dashboard query the current license state.

### Configuration

#### License Server

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `PORT` | `8080` | No | HTTP listen port |
| `DATABASE_URL` | | Yes | PostgreSQL connection string |
| `ADMIN_TOKEN` | | Yes | Bearer token for admin endpoints |
| `GRACE_PERIOD_DAYS` | `3` | No | Days after expiry before hard block |

#### Bridge License Client

| Setting | Default | Description |
|---------|---------|-------------|
| `ServerURL` | `https://api.armorclaw.com/v1` | License server base URL |
| `LicenseKey` | | License key for this instance |
| `InstanceID` | Auto-generated | Unique instance identifier |
| `GracePeriodDays` | `3` | Local grace period if server unreachable |
| `OfflineMode` | `false` | Never contact server |
| `Timeout` | `10s` | HTTP request timeout |

#### State Manager

| Setting | Default | Description |
|---------|---------|-------------|
| `GracePeriodDuration` | `7 days` | Time after expiry before hard block |
| `PollInterval` | `24 hours` | How often to revalidate |
| `AlertThresholds` | `[30, 14, 7, 1]` | Days before expiry to send alerts |
| `BlockOnExpired` | `true` | Block all ops when fully expired |
| `ReadOnlyOnGrace` | `false` | Restrict to reads during grace |

#### Enforcement

| Setting | Default | Description |
|---------|---------|-------------|
| `DefaultComplianceMode` | `basic` | Compliance level when license missing |
| `EnableGracePeriod` | `true` | Allow degraded mode on expired license |
| `GracePeriodDays` | `3` | Grace window |
| `StrictMode` | `false` | Block all features on invalid license |

### Team-Aware Enforcement

The license system integrates with the team subsystem (`bridge/pkg/team/`) to enforce per-team governance and audit compliance:

**Team Governance** (`bridge/pkg/team/governance.go`):
- `GovernanceConfig` defines team size limits (`MaxMembersPerTeam`, `MaxTeamsPerInstance`) and allowed roles
- `GovernanceEnforcer` validates team creation, member additions, and role assignments against governance limits
- Per-team policy overrides allow individual teams to deviate from default risk-class handling (`overrides map[teamID][riskClass] → ALLOW or DEFER`)

**Team Audit Events** (`bridge/pkg/team/audit.go`):
- 7 event types: `team_created`, `team_dissolved`, `member_added`, `member_removed`, `role_assigned`, `delegation_sent`, `handoff_complete`
- Each governance mutation emits a `TeamAuditEntry` with event ID, team ID, agent ID, role, and timestamp

**Team Metrics** (`bridge/pkg/team/metrics.go`):
- Per-team tracking: token usage, cost, latency, handoff success rate, secret access count, approval rates
- `TeamMetricsSnapshot` provides read-only metric views per team

**Team Roles** (`bridge/pkg/team/roles.go`):
- Built-in role registry with capability sets (browser, form filling, document processing, etc.)
- Roles gated by license tier — Pro and Enterprise tiers unlock additional capabilities

### Integration Points

#### Bridge Startup Sequence

1. Bridge creates a `license.Client` with the configured license key.
2. Bridge creates a `StateManager` with the client as its validator.
3. `StateManager.Initialize()` contacts the license server. If the server is reachable and the key is valid, state becomes `Valid`. If unreachable, state becomes `Unknown` with `BehaviorDegraded`.
4. Bridge creates an `enforcement.Manager` with the license client.
5. Bridge calls `Manager.RefreshLicense()` to cache the license.
6. Bridge creates a `BridgeEnforcer` and `BridgeHook` from the Manager.
7. `BridgeHook.BeforeBridgeStart()` confirms the minimum bridge feature is available.
8. `StateManager.StartPolling()` begins background revalidation.
9. `Manager.StartPeriodicRefresh()` begins background license refresh.

#### During Operation

- The StateManager polls the server at the configured interval (default 24 hours).
- The enforcement Manager runs its own periodic refresh goroutine.
- Each platform adapter checks `BridgeHook.BeforeAdapterStart()` before connecting.
- Each new channel bridge checks `BridgeHook.BeforeChannelBridge()` to enforce limits.
- Messages passing through may be PHI-scrubbed or audit-logged depending on `GetComplianceConfig()`.
- RPC handlers use `LicenseStatusHandler.GetStatus()` to report license state to callers.

#### When a License Expires

1. The server returns `LICENSE_EXPIRED` on the next validation.
2. The StateManager transitions to `StateGracePeriod` (if within grace) or `StateExpired`.
3. During grace: `BehaviorDegraded` allows most operations but blocks admin/config changes. The dashboard shows a warning.
4. After grace: `BehaviorBlocked` pauses all operations. The dashboard shows an error page.
5. Alerts fire on state transitions through the `AlertSender` interface.

#### Docker Deployment

The license server runs as a separate container in the Docker Compose stack. It needs a PostgreSQL database (configured via `DATABASE_URL`). The Bridge container connects to it over the Docker network using the configured `ServerURL`.

Typical `docker-compose.yml` addition:

```yaml
license-server:
  build:
    context: ./license-server
  environment:
    DATABASE_URL: postgres://user:pass@license-db:5432/licenses
    ADMIN_TOKEN: ${LICENSE_ADMIN_TOKEN}
    GRACE_PERIOD_DAYS: "3"
  depends_on:
    - license-db
  ports:
    - "8080:8080"

license-db:
  image: postgres:16-alpine
  environment:
    POSTGRES_DB: licenses
    POSTGRES_USER: user
    POSTGRES_PASSWORD: ${LICENSE_DB_PASSWORD}
  volumes:
    - license-db-data:/var/lib/postgresql/data
```
