# Rust Office Sidecar Implementation Plan (v6.0 Enterprise Sovereign)

> **Status**: DRAFT
> **Created**: 2025-01-03
> **Estimated Duration**: 6-8 weeks (2 phases)
> **Architecture Review**: Oracle consultation completed ✅

---

## TL;DR

Implement a Rust Office Sidecar that handles heavy I/O operations (storage, document processing, OCR) while the Go Bridge maintains security sovereignty (keystore, PII interception, audit).

**Phase 1 (4 weeks)**: Core infrastructure + S3 storage + PDF/DOCX processing
**Phase 2 (2-4 weeks)**: Advanced features (OCR, XLSX, embeddings, SharePoint, Azure)

---

## Executive Summary

**Problem**: Go Bridge is optimized for security and orchestration, not heavy I/O operations like cloud storage uploads, document OCR, and embedding generation.

**Solution**: Introduce a stateless Rust Sidecar process that handles data-plane operations under the strict security supervision of the Go Bridge.

**Key Insight from Oracle**: This separation of concerns (Control Plane vs Data Plane) is the **right architectural approach** for a security-focused system, but the 4-6 week timeline is **aggressive**. Better to phase implementation.

---

## Context

### Original Request
User provided comprehensive plan for Rust Office Sidecar with gRPC-over-UDS, cloud storage backends, document processing, OCR, legal diffing, and RAG integration.

### Oracle Consultation
**Oracle reviewed the architecture and identified:**
- ✅ Control Plane vs Data Plane separation is correct
- ⚠️ 4-6 week timeline is aggressive—recommend phasing
- ⚠️ Token TTL of 5 minutes is too short—use 30 minute sessions
- ⚠️ Tesseract FFI complexity—consider subprocess approach for v1
- ⚠️ SharePoint API is notoriously complex—defer to Phase 2
- ❌ Missing circuit breaker, graceful degradation, version negotiation, rate limiting

### Architecture Review Findings

**Security Model (Oracle Approved):**
```
┌─────────────────────────────────────────────────────┐
│         GO BRIDGE (CONTROL PLANE)                   │
│                                                     │
│  • SQLCipher Keystore (Master)                      │
│  • ShadowMap PII Interception                       │
│  • Matrix Protocol Handler                          │
│  • Audit Logging (Immutable)                        │
│  • NATS Message Broker                              │
│  • Agent Lifecycle Management                       │
│  • Credential Injection (BlindFill)                 │
│                                                     │
│  STATE:                                              │
│  • vault.db (User secrets, backed up daily)         │
│  • matrix_state.db (Ephemeral ratchets)            │
│  • audit.db (Immutable transaction log)             │
└──────────────────┬──────────────────────────────────┘
                   │
                   │ Unix Domain Socket
                   │ /run/armorclaw/sidecar.sock
                   │ (0600 permissions)
                   │
                   ▼
┌─────────────────────────────────────────────────────┐
│         RUST SIDECAR (DATA PLANE)                   │
│                                                     │
│  • Cloud Storage Operations (S3, SharePoint, etc)   │
│  • Document Processing (PDF, DOCX, XLSX)           │
│  • OCR Processing (Tesseract FFI)                   │
│  • Legal Diffing (Myers Algorithm)                  │
│  • Large File Streaming (Zero-copy)                 │
│  • Embedding Generation (for RAG)                   │
│                                                     │
│  STATELESS:                                         │
│  • NO persistent storage                            │
│  • NO credential caching                            │
│  • NO secret persistence                            │
│  • ALL state in memory during request lifecycle     │
└─────────────────────────────────────────────────────┘
```

**Communication Protocol:**
- **Transport**: Unix Domain Socket (UDS) with 0600 permissions
- **Protocol**: gRPC over UDS (HTTP/2 framing)
- **Serialization**: Protobuf
- **Authentication**: Per-request ephemeral tokens (30 min TTL)
- **Authorization**: Go Bridge validates all requests before forwarding

**Why This Approach (Oracle):**
1. ✅ Preserves existing security model—Go Bridge remains security boundary
2. ✅ Leverages Rust strengths—memory-safe parsing of untrusted documents
3. ✅ Stateless sidecar—simplifies crash recovery, matches security requirements
4. ✅ UDS + tokens—appropriate for single-host, extensible to mTLS later

---

## Work Objectives

### Core Objective
Implement a Rust Office Sidecar that handles heavy I/O operations while maintaining zero-trust security sovereignty in the Go Bridge.

### Concrete Deliverables

**Phase 1 (Weeks 1-4):**
- [ ] Rust sidecar binary with gRPC-over-UDS
- [ ] Go Bridge sidecar client with ephemeral token generation
- [ ] S3 storage connector (upload, download, list, delete)
- [ ] PDF processing (extract text, split, merge)
- [ ] DOCX processing (extract text, basic editing)
- [ ] Integration tests for core operations
- [ ] Circuit breaker and rate limiting
- [ ] Audit logging integration

**Phase 2 (Weeks 5-8):**
- [ ] XLSX processing (extract data, formulas)
- [ ] OCR integration (Tesseract subprocess approach)
- [ ] SharePoint connector
- [ ] Azure Blob connector
- [ ] Legal diffing (Myers algorithm with redline DOCX output)
- [ ] Embedding generation for RAG
- [ ] Qdrant integration for ephemeral collections
- [ ] Performance optimization and load testing

### Definition of Done

- [ ] All Phase 1 deliverables implemented and tested
- [ ] Security audit passed (no credential leakage, proper token validation)
- [ ] Integration tests covering happy path and error scenarios
- [ ] Documentation updated (README.md, doc/armorclaw.md)
- [ ] Performance benchmarks meet targets (100 req/s, <100ms latency for small files)
- [ ] Circuit breaker and graceful degradation tested
- [ ] Audit logging verified in Go Bridge

### Must Have

- **Security Sovereignty**: All credentials remain in Go Bridge SQLCipher keystore
- **Stateless Sidecar**: Rust process maintains NO persistent state
- **Audit Trail**: Every sidecar operation logged in Go Bridge audit.db
- **PII Interception**: ShadowMap validates all requests before forwarding
- **Ephemeral Tokens**: 30-minute TTL, HMAC-signed, validated on every request
- **Circuit Breaker**: Sidecar fails fast when cloud storage unavailable
- **Rate Limiting**: Cloud API rate limits enforced in sidecar
- **Graceful Degradation**: Go Bridge queues operations when sidecar unavailable

### Must NOT Have (Guardrails from Oracle)

- **NO persistent credential storage** in Rust sidecar
- **NO credential caching** in memory beyond request lifecycle
- **NO direct cloud API calls** without Go Bridge interception
- **NO audit logging in sidecar** (all logging in Go Bridge)
- **NO SharePoint in Phase 1** (too complex, defer to Phase 2)
- **NO Tesseract FFI in Phase 1** (use subprocess approach first)
- **NO 5-minute token TTL** (use 30 minutes to reduce regeneration overhead)

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: Yes (Go Bridge has extensive test coverage)
- **Automated tests**: TDD (test-driven development)
- **Framework**: Go: `testing` package + `testify`; Rust: `cargo test` + `tokio-test`
- **Approach**: RED (failing test) → GREEN (minimal impl) → REFACTOR

### QA Policy
Every task includes agent-executed QA scenarios with evidence capture.

**Test Categories:**
1. **Unit Tests** (Rust): Each connector, document processor, security module
2. **Integration Tests** (Go + Rust): End-to-end sidecar operations
3. **Security Tests**: Token validation, credential isolation, PII interception
4. **Performance Tests**: Large file handling (5GB+), concurrent requests
5. **Failure Tests**: Sidecar crash recovery, cloud storage unavailability

**Evidence Location**: `.sisyphus/evidence/sidecar-{test-name}.{ext}`

---

## Execution Strategy

### Phase 1: Core Infrastructure (Weeks 1-4)

**Wave 1 (Days 1-5): Project Foundation**
```
├── Task 1: Create Rust sidecar project structure [quick]
├── Task 2: Define gRPC protocol (sidecar.proto) [quick]
├── Task 3: Implement gRPC server with UDS listener [deep]
├── Task 4: Create Go Bridge sidecar client [deep]
├── Task 5: Implement ephemeral token generation (30 min TTL) [deep]
└── Task 6: Add security middleware (token validation, HMAC) [deep]
```

**Wave 2 (Days 6-10): S3 Storage Connector**
```
├── Task 7: Implement S3 upload with streaming [deep]
├── Task 8: Implement S3 download with chunking [deep]
├── Task 9: Implement S3 list and delete [quick]
├── Task 10: Add circuit breaker for S3 operations [unspecified-high]
├── Task 11: Add rate limiting (100 req/s) [quick]
└── Task 12: Integration tests for S3 operations [unspecified-high]
```

**Wave 3 (Days 11-15): Document Processing**
```
├── Task 13: Implement PDF text extraction (lopdf) [deep]
├── Task 14: Implement PDF split and merge [deep]
├── Task 15: Implement DOCX text extraction (docx-rs) [quick]
├── Task 16: Implement DOCX basic editing [unspecified-high]
├── Task 17: Add file size validation (max 5GB) [quick]
└── Task 18: Integration tests for document processing [unspecified-high]
```

**Wave 4 (Days 16-20): Integration & Security**
```
├── Task 19: Integrate ShadowMap PII interception in Go client [deep]
├── Task 20: Add audit logging for all sidecar operations [quick]
├── Task 21: Implement graceful degradation (queue on sidecar down) [unspecified-high]
├── Task 22: Add version negotiation (client/server handshake) [quick]
├── Task 23: Security tests (token validation, credential isolation) [deep]
└── Task 24: Performance tests (concurrent requests, large files) [unspecified-high]
```

**Wave 5 (Days 21-28): Polish & Documentation**
```
├── Task 25: Add health check endpoint with metrics [quick]
├── Task 26: Add structured logging (JSON to stderr) [quick]
├── Task 27: Write integration test suite [unspecified-high]
├── Task 28: Update README.md with sidecar architecture [writing]
├── Task 29: Update doc/armorclaw.md with sidecar details [writing]
└── Task 30: Performance optimization and load testing [deep]
```

### Phase 2: Advanced Features (Weeks 5-8)

**Wave 6 (Days 29-35): Advanced Document Processing**
```
├── Task 31: Implement XLSX data extraction (calamine) [deep]
├── Task 32: Implement XLSX formula parsing [deep]
├── Task 33: Implement OCR with Tesseract subprocess [unspecified-high]
├── Task 34: Add OCR language detection and configuration [quick]
└── Task 35: Integration tests for XLSX and OCR [unspecified-high]
```

**Wave 7 (Days 36-42): Additional Cloud Connectors**
```
├── Task 36: Implement SharePoint connector (Graph API) [unspecified-high]
├── Task 37: Implement Azure Blob connector [unspecified-high]
├── Task 38: Add connector abstraction layer [deep]
├── Task 39: Integration tests for SharePoint and Azure [unspecified-high]
└── Task 40: Add cloud-specific rate limiting [quick]
```

**Wave 8 (Days 43-49): Legal Diffing & RAG**
```
├── Task 41: Implement Myers diff algorithm [deep]
├── Task 42: Generate HTML diff output [visual-engineering]
├── Task 43: Generate redline DOCX output [unspecified-high]
├── Task 44: Implement text chunking for RAG [deep]
├── Task 45: Integrate embedding generation [unspecified-high]
└── Task 46: Add Qdrant integration for ephemeral collections [unspecified-high]
```

**Wave 9 (Days 50-56): Final Integration**
```
├── Task 47: End-to-end testing (all connectors, all processors) [unspecified-high]
├── Task 48: Load testing (1000 concurrent requests, 5GB files) [unspecified-high]
├── Task 49: Security audit (credential isolation, token security) [oracle]
├── Task 50: Documentation final review [writing]
└── Task 51: Performance profiling and optimization [deep]
```

---

## TODOs

> Implementation + Test = ONE Task. Never separate.
> EVERY task MUST have: Recommended Agent Profile + Parallelization info + QA Scenarios.

### WAVE 1: Project Foundation (Days 1-5)

- [ ] 1. Create Rust Sidecar Project Structure

  **What to do**:
  - Create `sidecar/` directory with Cargo.toml
  - Set up project structure with modules (grpc, connectors, document, security, utils)
  - Add dependencies (tokio, tonic, prost, aws-sdk-s3, etc.)
  - Create `src/main.rs` with basic CLI argument parsing
  - Create `src/config.rs` with configuration management
  - Create `src/error.rs` with custom error types

  **Must NOT do**:
  - Do NOT add Tesseract FFI dependencies (defer to Phase 2)
  - Do NOT add SharePoint/Azure dependencies (Phase 2)
  - Do NOT add Qdrant client (Phase 2)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Straightforward project scaffolding
  - **Skills**: []
    - No special skills needed for file creation

  **Parallelization**:
  - **Can Run In Parallel**: NO (foundational)
  - **Parallel Group**: Sequential
  - **Blocks**: Tasks 2-6
  - **Blocked By**: None (can start immediately)

  **References**:
  - **Pattern References**:
    - `bridge/cmd/bridge/main.go:1-100` - Go Bridge entry point pattern
    - `bridge/pkg/config/config.go` - Configuration loading pattern
  
  **Acceptance Criteria**:
  - [ ] `sidecar/Cargo.toml` created with all Phase 1 dependencies
  - [ ] `sidecar/src/main.rs` compiles successfully
  - [ ] `cargo build` succeeds in sidecar directory
  - [ ] Directory structure matches specification

  **QA Scenarios**:
  ```
  Scenario: Project compiles successfully
    Tool: Bash
    Preconditions: Rust toolchain installed
    Steps:
      1. cd sidecar
      2. cargo check
      3. Verify exit code 0
    Expected Result: No compilation errors
    Failure Indicators: Compilation errors, missing dependencies
    Evidence: .sisyphus/evidence/task-01-compile.log
  
  Scenario: Dependencies resolve correctly
    Tool: Bash
    Steps:
      1. cd sidecar
      2. cargo metadata --format-version=1 | jq '.packages | length'
      3. Verify > 10 packages resolved
    Expected Result: All dependencies resolve
    Evidence: .sisyphus/evidence/task-01-deps.log
  ```

  **Commit**: NO (groups with Task 2)

---

- [x] 2. Define gRPC Protocol

  **What to do**:
  - Create `sidecar/src/grpc/proto/sidecar.proto`
  - Define SidecarService with all RPC methods
  - Define message types (UploadBlobRequest, DownloadBlobRequest, etc.)
  - Add RequestMetadata with ephemeral token fields
  - Add health check messages
  - Run `cargo build` to generate protobuf code

  **Must NOT do**:
  - Do NOT include SharePoint/Azure-specific messages (Phase 2)
  - Do NOT include OCR messages (Phase 2)
  - Do NOT include RAG messages (Phase 2)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Protocol definition is straightforward
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 1)
  - **Parallel Group**: Sequential with Task 1
  - **Blocks**: Tasks 3-6
  - **Blocked By**: Task 1

  **References**:
  - **External References**:
    - https://protobuf.dev/programming-guides/proto3/ - Protobuf syntax
    - https://grpc.io/docs/languages/rust/ - gRPC Rust documentation

  **Acceptance Criteria**:
  - [ ] `sidecar/src/grpc/proto/sidecar.proto` created
  - [ ] All Phase 1 RPC methods defined (UploadBlob, DownloadBlob, ListBlobs, DeleteBlob, ProcessDocument, ExtractText, HealthCheck)
  - [ ] Protobuf code generated in `sidecar/src/grpc/proto/sidecar.pb.rs`
  - [ ] `cargo build` succeeds with generated code

  **QA Scenarios**:
  ```
  Scenario: Protobuf compilation succeeds
    Tool: Bash
    Steps:
      1. cd sidecar
      2. cargo build
      3. Verify no errors
    Expected Result: Protobuf code generated successfully
    Evidence: .sisyphus/evidence/task-02-proto.log
  ```

  **Commit**: YES
  - Message: `feat(sidecar): add gRPC protocol definition`
  - Files: `sidecar/src/grpc/proto/sidecar.proto`, `sidecar/build.rs`

---

- [x] 3. Implement gRPC Server with UDS Listener

  **What to do**:
  - Create `sidecar/src/grpc/server.rs`
  - Implement `SidecarServiceImpl` struct
  - Implement Unix Domain Socket listener
  - Set socket permissions to 0600
  - Add middleware layers (timeout, rate limit, concurrency limit)
  - Implement health check endpoint
  - Add graceful shutdown handling

  **Must NOT do**:
  - Do NOT implement business logic yet (placeholder responses only)
  - Do NOT add mTLS (UDS with filesystem permissions is sufficient)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex async Rust code with tokio and tonic
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 2)
  - **Parallel Group**: Sequential
  - **Blocks**: Tasks 4-6
  - **Blocked By**: Task 2

  **References**:
  - **External References**:
    - https://docs.rs/tonic/latest/tonic/ - Tonic gRPC documentation
    - https://docs.rs/tokio/latest/tokio/net/struct.UnixListener.html - UDS listener

  **Acceptance Criteria**:
  - [ ] Server starts and listens on `/run/armorclaw/sidecar.sock`
  - [ ] Socket has 0600 permissions
  - [ ] Health check endpoint returns valid response
  - [ ] Server handles graceful shutdown (SIGTERM, SIGINT)
  - [ ] Rate limiting middleware active (100 req/s)
  - [ ] Concurrency limit enforced (50 concurrent requests)

  **QA Scenarios**:
  ```
  Scenario: Server starts and creates socket
    Tool: Bash
    Preconditions: Rust sidecar binary built
    Steps:
      1. cargo run --bin armorclaw-sidecar &
      2. sleep 2
      3. ls -la /run/armorclaw/sidecar.sock
      4. Verify permissions are 0600
      5. pkill armorclaw-sidecar
    Expected Result: Socket created with correct permissions
    Evidence: .sisyphus/evidence/task-03-socket.log

  Scenario: Health check returns valid response
    Tool: Bash
    Steps:
      1. Start sidecar server
      2. Use grpcurl to call HealthCheck
      3. Verify response contains status="healthy"
    Expected Result: Health check works
    Evidence: .sisyphus/evidence/task-03-health.log
  ```

  **Commit**: YES
  - Message: `feat(sidecar): implement gRPC server with UDS listener`
  - Files: `sidecar/src/grpc/server.rs`, `sidecar/src/main.rs`

---

- [x] 4. Create Go Bridge Sidecar Client

  **What to do**:
  - Create `bridge/pkg/sidecar/client.go`
  - Implement `Client` struct with connection management
  - Implement Unix Domain Socket dialer for gRPC
  - Add connection pooling and retry logic
  - Implement context timeout handling
  - Add connection health checking
  - Create `bridge/pkg/sidecar/client_test.go` with unit tests

  **Must NOT do**:
  - Do NOT call sidecar from agent containers (only from Go Bridge)
  - Do NOT cache connections indefinitely (reconnect on failure)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex Go gRPC client with UDS transport
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 3)
  - **Parallel Group**: Sequential
  - **Blocks**: Task 5
  - **Blocked By**: Task 3

  **References**:
  - **Pattern References**:
    - `bridge/pkg/rpc/server.go` - Existing gRPC patterns in Go Bridge

  **Acceptance Criteria**:
  - [x] Client connects to sidecar over UDS
  - [x] Connection retry logic works (max 5 retries)
  - [x] Context timeout enforced (30s default)
  - [x] Health check method works
  - [x] Unit tests pass (>80% coverage)
  - [x] Connection closed properly on shutdown

  **QA Scenarios**:
  ```
  Scenario: Client connects to sidecar
    Tool: Bash
    Steps:
      1. Start Rust sidecar
      2. Run Go test: go test -v ./pkg/sidecar -run TestClientConnect
      3. Verify connection succeeds
    Expected Result: Client connects successfully
    Evidence: .sisyphus/evidence/task-04-client.log

  Scenario: Client handles sidecar restart
    Tool: Bash
    Steps:
      1. Start sidecar
      2. Connect client
      3. Kill sidecar
      4. Restart sidecar
      5. Verify client reconnects
    Expected Result: Graceful reconnection
    Evidence: .sisyphus/evidence/task-04-reconnect.log
  ```

  **Commit**: YES
  - Message: `feat(bridge): add sidecar gRPC client`
  - Files: `bridge/pkg/sidecar/client.go`, `bridge/pkg/sidecar/client_test.go`

---

- [ ] 5. Implement Ephemeral Token Generation (30 min TTL)

  **What to do**:
  - Create `bridge/pkg/sidecar/token.go`
  - Implement token generation with HMAC-SHA256
  - Set token TTL to 30 minutes (not 5 minutes per Oracle)
  - Add token validation in Rust sidecar
  - Create `sidecar/src/security/token.rs`
  - Implement request signature verification
  - Add timestamp validation (reject requests older than 5 minutes)

  **Must NOT do**:
  - Do NOT use 5-minute TTL (too short, causes regeneration overhead)
  - Do NOT store tokens persistently (in-memory only)
  - Do NOT reuse tokens across requests

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Security-critical code with cryptographic operations
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 4)
  - **Parallel Group**: Sequential
  - **Blocks**: Task 6
  - **Blocked By**: Task 4

  **References**:
  - **External References**:
    - https://docs.rs/hmac/latest/hmac/ - HMAC Rust crate
    - https://pkg.go.dev/crypto/hmac - HMAC Go package

  **Acceptance Criteria**:
  - [ ] Go Bridge generates tokens with 30-minute expiration
  - [ ] Tokens include HMAC signature of (request_id + timestamp + operation)
  - [ ] Rust sidecar validates token signature
  - [ ] Rust sidecar rejects expired tokens
  - [ ] Rust sidecar rejects requests with timestamp > 5 minutes old
  - [ ] Unit tests for token generation/validation

  **QA Scenarios**:
  ```
  Scenario: Valid token accepted
    Tool: Bash
    Steps:
      1. Generate token in Go
      2. Send request to Rust sidecar
      3. Verify request accepted
    Expected Result: Request succeeds
    Evidence: .sisyphus/evidence/task-05-valid-token.log

  Scenario: Expired token rejected
    Tool: Bash
    Steps:
      1. Generate token with old timestamp (6 minutes ago)
      2. Send request to Rust sidecar
      3. Verify request rejected with UNAUTHENTICATED
    Expected Result: Request rejected
    Evidence: .sisyphus/evidence/task-05-expired-token.log
  ```

  **Commit**: YES
  - Message: `feat(security): implement ephemeral token auth (30min TTL)`
  - Files: `bridge/pkg/sidecar/token.go`, `sidecar/src/security/token.rs`

---

- [ ] 6. Add Security Middleware (Token Validation, HMAC)

  **What to do**:
  - Create gRPC interceptor in Rust sidecar
  - Validate RequestMetadata on every RPC call
  - Verify HMAC signature matches expected value
  - Add request logging (without sensitive data)
  - Create `sidecar/src/grpc/interceptor.rs`
  - Add metrics collection (request count, latency, errors)
  - Write integration tests for security scenarios

  **Must NOT do**:
  - Do NOT log token values or signatures
  - Do NOT skip validation for any request type
  - Do NOT allow requests without metadata

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Security-critical middleware with complex validation logic
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 5)
  - **Parallel Group**: Sequential
  - **Blocks**: Wave 2 tasks
  - **Blocked By**: Task 5

  **References**:
  - **External References**:
    - https://docs.rs/tonic/latest/tonic/service/interceptor/index.html - Tonic interceptors

  **Acceptance Criteria**:
  - [ ] All RPC calls validated through interceptor
  - [ ] Invalid tokens rejected with UNAUTHENTICATED status
  - [ ] Valid tokens pass through to handler
  - [ ] Request metrics collected (Prometheus format)
  - [ ] Integration tests cover all security scenarios

  **QA Scenarios**:
  ```
  Scenario: Interceptor validates all requests
    Tool: Bash
    Steps:
      1. Send request without metadata
      2. Verify rejection
      3. Send request with invalid signature
      4. Verify rejection
      5. Send request with valid token
      6. Verify acceptance
    Expected Result: Only valid requests pass
    Evidence: .sisyphus/evidence/task-06-interceptor.log
  ```

  **Commit**: YES
  - Message: `feat(security): add gRPC security middleware`
  - Files: `sidecar/src/grpc/interceptor.rs`, `sidecar/src/grpc/server.rs`

---

### WAVE 2: S3 Storage Connector (Days 6-10)

- [ ] 7. Implement S3 Upload with Streaming

  **What to do**:
  - Create `sidecar/src/connectors/aws_s3.rs`
  - Implement upload function using AWS SDK for Rust
  - Support both in-memory content and file path streaming
  - Use ByteStream for zero-copy uploads
  - Handle large files (>1GB) without memory exhaustion
  - Compute SHA256 hash during upload
  - Return UploadBlobResponse with etag and hash

  **Must NOT do**:
  - Do NOT load entire file into memory (use streaming)
  - Do NOT cache AWS credentials (use ephemeral tokens from request)
  - Do NOT skip SHA256 computation

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex async I/O with AWS SDK and streaming
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Wave 1)
  - **Parallel Group**: Wave 2 (with Tasks 8-12)
  - **Blocks**: Task 12
  - **Blocked By**: Task 6

  **References**:
  - **External References**:
    - https://docs.rs/aws-sdk-s3/latest/aws_sdk_s3/ - AWS S3 SDK for Rust

  **Acceptance Criteria**:
  - [ ] Upload succeeds for files up to 5GB
  - [ ] Memory usage stays constant regardless of file size
  - [ ] SHA256 hash computed correctly
  - [ ] Proper error handling for S3 failures
  - [ ] Unit tests with mocked S3 client

  **QA Scenarios**:
  ```
  Scenario: Upload small file (< 100MB)
    Tool: Bash
    Steps:
      1. Create 50MB test file
      2. Upload via sidecar
      3. Verify response contains blob_id and hash
      4. Download from S3 and verify hash matches
    Expected Result: Upload succeeds, hash matches
    Evidence: .sisyphus/evidence/task-07-small-upload.log

  Scenario: Upload large file (5GB)
    Tool: Bash
    Steps:
      1. Create 5GB sparse file
      2. Monitor memory usage during upload
      3. Verify memory stays < 500MB
      4. Verify upload completes successfully
    Expected Result: Memory bounded, upload succeeds
    Evidence: .sisyphus/evidence/task-07-large-upload.log
  ```

  **Commit**: YES
  - Message: `feat(storage): implement S3 upload with streaming`
  - Files: `sidecar/src/connectors/aws_s3.rs`

---

- [ ] 8. Implement S3 Download with Chunking

  **What to do**:
  - Implement download_stream function
  - Use async_stream for streaming response
  - Chunk downloads at 1MB per chunk
  - Support range requests (offset_bytes, max_bytes)
  - Handle S3 errors gracefully
  - Return BlobChunk stream

  **Must NOT do**:
  - Do NOT buffer entire file in memory
  - Do NOT skip chunking (always stream)
  - Do NOT ignore range parameters

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex async streaming with error handling
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7, 9-12)
  - **Blocks**: Task 12
  - **Blocked By**: Task 6

  **Acceptance Criteria**:
  - [ ] Downloads stream in 1MB chunks
  - [ ] Memory usage bounded to ~2MB
  - [ ] Range requests work correctly
  - [ ] Last chunk has is_last=true
  - [ ] Errors properly propagated in stream

  **QA Scenarios**:
  ```
  Scenario: Stream download in chunks
    Tool: Bash
    Steps:
      1. Upload 10MB file to S3
      2. Download via sidecar
      3. Count chunks received
      4. Verify ~10 chunks (1MB each)
      5. Verify file integrity
    Expected Result: Proper chunking, correct file
    Evidence: .sisyphus/evidence/task-08-stream.log
  ```

  **Commit**: YES
  - Message: `feat(storage): implement S3 streaming download`
  - Files: `sidecar/src/connectors/aws_s3.rs`

---

(Continuing with Tasks 9-51 in similar format...)

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Verify all Phase 1 deliverables implemented. Check evidence files exist. Verify security constraints (no credential caching, stateless sidecar, audit logging).
  Output: `Deliverables [N/N] | Security Constraints [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `cargo clippy` + `cargo test` + `go test`. Review for unsafe code, unwrap() in production paths, missing error handling.
  Output: `Clippy [PASS/FAIL] | Tests [N pass/N fail] | Unsafe [N instances] | VERDICT`

- [ ] F3. **Real Manual QA** — `unspecified-high`
  Test end-to-end: Matrix client → Go Bridge → Sidecar → S3. Verify audit logs written. Test large file upload (5GB). Test sidecar crash recovery.
  Output: `E2E Flow [PASS/FAIL] | Large Files [PASS/FAIL] | Crash Recovery [PASS/FAIL] | VERDICT`

- [ ] F4. **Security Audit** — `oracle`
  Review token validation, credential isolation, PII interception. Verify no secrets logged. Check socket permissions. Verify HMAC signatures.
  Output: `Tokens [SECURE/INSECURE] | Credentials [ISOLATED/LEAKED] | Audit [COMPLETE/MISSING] | VERDICT`

---

## Commit Strategy

**Wave 1 commits:**
1. `feat(sidecar): add gRPC protocol definition` (Task 2)
2. `feat(sidecar): implement gRPC server with UDS listener` (Task 3)
3. `feat(bridge): add sidecar gRPC client` (Task 4)
4. `feat(security): implement ephemeral token auth (30min TTL)` (Task 5)
5. `feat(security): add gRPC security middleware` (Task 6)

**Wave 2 commits:**
1. `feat(storage): implement S3 upload with streaming` (Task 7)
2. `feat(storage): implement S3 streaming download` (Task 8)
3. `feat(storage): add S3 list and delete operations` (Task 9)
4. `feat(reliability): add circuit breaker for S3` (Task 10)
5. `feat(reliability): add rate limiting` (Task 11)
6. `test(storage): add S3 integration tests` (Task 12)

(Continue pattern for all waves...)

---

## Success Criteria

### Phase 1 Success Criteria (Weeks 1-4)
- [ ] Rust sidecar binary runs and accepts gRPC connections over UDS
- [ ] Go Bridge client connects and authenticates successfully
- [ ] S3 upload/download/list/delete operations work
- [ ] PDF text extraction works
- [ ] DOCX text extraction works
- [ ] All operations logged in Go Bridge audit.db
- [ ] ShadowMap intercepts PII before sidecar calls
- [ ] Circuit breaker prevents cascading failures
- [ ] Rate limiting prevents cloud API throttling
- [ ] Performance: 100 req/s sustained, <100ms latency for small files
- [ ] Security audit passed

### Phase 2 Success Criteria (Weeks 5-8)
- [ ] XLSX data extraction works
- [ ] OCR processing works (subprocess approach)
- [ ] SharePoint connector works
- [ ] Azure Blob connector works
- [ ] Legal diffing produces correct output
- [ ] Embedding generation works
- [ ] Qdrant integration works
- [ ] Load test: 1000 concurrent requests, 5GB files
- [ ] Documentation updated

---

## References

### Architecture Decision Records
- ADR-001: Control Plane vs Data Plane Separation
- ADR-002: Stateless Sidecar Design
- ADR-003: Ephemeral Token Authentication (30min TTL)
- ADR-004: gRPC-over-UDS Communication

### External Documentation
- [gRPC Rust Documentation](https://docs.rs/tonic/latest/tonic/)
- [AWS SDK for Rust](https://docs.rs/aws-sdk-s3/latest/aws_sdk_s3/)
- [Oracle Consultation Results](#oracle-consultation) (see above)

### Related Plans
- Cloudflare HTTPS Setup (completed)
- Bridge HTTP Server Fix (completed)

---

## Appendix: Oracle Recommendations Summary

1. ✅ **Token TTL**: Changed from 5 minutes to 30 minutes
2. ✅ **Phased Implementation**: Split into Phase 1 (4 weeks) and Phase 2 (2-4 weeks)
3. ✅ **Circuit Breaker**: Added to Wave 2
4. ✅ **Rate Limiting**: Added to Wave 2
5. ✅ **Graceful Degradation**: Added to Wave 4
6. ✅ **Version Negotiation**: Added to Wave 4
7. ✅ **Tesseract Approach**: Subprocess instead of FFI for Phase 2
8. ✅ **SharePoint Deferral**: Moved to Phase 2
9. ✅ **Bounded Memory**: File size validation added to Wave 3
10. ✅ **Monitoring**: Health check with metrics added to Wave 5

---

**Plan Status**: Ready for Phase 1 implementation
**Next Step**: Run `/start-work rust-office-sidecar` to begin execution
