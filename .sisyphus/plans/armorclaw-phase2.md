# ArmorClaw Phase 2: Transport, Ingestion, Vault & Provenance

## TL;DR

> **Quick Summary**: Implement Phase 2 Master Spec v2.1 — 5 waves: fix prerequisites (broken imports, gRPC wiring, extractors), add TrustedProxyGuard at net.Conn level, implement YARA disk-based scanning and tiered ONNX OCR in Rust sidecar, build XChaCha20Poly1305 split-storage vault with Qdrant, and add PDF generation with HMAC provenance signing.
>
> **Deliverables**: TrustedProxyGuard (3 entry points), YARA scanner (10-15 rules), ONNX OCR tier (tract-onnx), XChaCha20Poly1305 split-storage with Qdrant RBAC, HMAC-SHA256 provenance, PDF generation, Qdrant in Docker, updated Caddyfile, integration tests
>
> **Estimated Effort**: XL (3-4 weeks)
> **Parallel Execution**: YES — 5 waves + final verification
> **Critical Path**: Wave 0 → Wave 2 (YARA/OCR) → Wave 3 (vault) → Wave 4 (output) → Final

---

## Context

### Original Request
Implement ArmorClaw Phase 2 Master Specification v2.1 — 4 waves of new features (Transport, Ingestion, Vault, Provenance) on the v6.0.0 microkernel governance architecture.

### Interview Summary
- TrustedProxyGuard: net.Conn level (NOT http.Handler) — bridge uses raw JSON-RPC
- ToolSidecar: Extend existing Rust sidecar with tract-onnx (NOT new TypeScript service)
- gRPC Wiring: Wire 7 placeholder handlers as prerequisite
- Extractors: Route PDF/DOCX through sidecar gRPC (fix Rust extractors)
- Test Strategy: TDD with existing Go/Rust frameworks
- Qdrant: Add container to docker-compose.yml (subnet 172.29.0.0/24)
- ONNX: tract-onnx in Rust for OCR only, paddle_ocr_v2.onnx via download script
- YARA: Starter ruleset 10-15 rules covering document threats

### Research Findings
- 3 handleConnection() instances need guarding (rpc/server.go, agent/injection.go, socket/server.go)
- MCP router has 2 broken imports (toolsidecar + translator)
- Sidecar library compiles (0 errors), binary has 80 errors (non-blocking)
- Qdrant code exists but commented out with broken import
- Caddy already in sentinel profile with bridge routes in template
- Rust-vault missing chacha20poly1305/hmac deps

### Metis Review (addressed)
- Guard all 3 handleConnection(), Unix socket bypass, IPv6 support
- Deterministic nonce derivation (HMAC-based, not random) for XChaCha20
- Key versioning alongside encrypted blobs
- chacha20poly1305 in sidecar, vault keeps SQLCipher
- ONNX is OCR only, NOT embeddings

---

## Work Objectives

### Core Objective
Secure document processing pipeline: TrustedProxyGuard → YARA CDR scanning → tiered OCR → XChaCha20 encrypted split-storage with Qdrant RBAC → PDF output with HMAC provenance.

### Definition of Done
- [ ] `cd bridge && go build ./...` passes (including pkg/mcp/)
- [ ] `cd sidecar && cargo build --lib` passes with 0 errors
- [ ] `cd bridge && go test ./pkg/yara/... ./pkg/trust/... -v` passes
- [ ] `cd sidecar && cargo test --lib` passes with all new modules
- [ ] `bash tests/test_yara_heap_profile.sh` passes (heap < 50MB)
- [ ] `bash tests/test_xchacha_nonce_length.sh` passes (nonce = 24 bytes)
- [ ] Docker compose healthy: Qdrant + Caddy + Bridge + Vault

### Must Have
- All 3 handleConnection() guarded, YARA ScanFile (not ScanMem), XChaCha20 24-byte nonce, Qdrant RBAC, HMAC-SHA256 provenance, TDD for all modules

### Must NOT Have (Guardrails)
- NO new TypeScript services, NO http.Handler middleware, NO SQLCipher changes, NO proto changes, NO new RPCs, NO existing middleware modification, NO ONNX for embeddings, NO Matrix bypass, NO approval flow weakening, NO Caddy WAF, NO full binary fix

---

## Verification Strategy
- **TDD**: RED (failing test) → GREEN (minimal impl) → REFACTOR
- **Go**: `go test ./path/... -v`, **Rust**: `cargo test --lib module -- -v`
- **Performance**: `go tool pprof` for YARA heap
- **Evidence**: `.sisyphus/evidence/task-{N}-{slug}.{ext}`

---

## Execution Strategy

```
Wave 0 (8 tasks): Fix prerequisites — stubs, Qdrant fix, extractors, gRPC wiring
Wave 1 (5 tasks): TrustedProxyGuard + Caddyfile
Wave 2 (6 tasks): YARA scanner + ONNX OCR
Wave 3 (5 tasks): XChaCha20 encryption + split-storage + Qdrant
Wave 4 (3 tasks): PDF generation + HMAC provenance
Wave FINAL (4 tasks): Parallel verification
```

---

## TODOs

### Wave 0: Prerequisites (8 tasks, ALL PARALLEL within constraints)

- [x] 1. Create `bridge/pkg/toolsidecar` Go Stub
  **What**: Minimal types/stubs fixing broken import in `bridge/pkg/mcp/router.go:24`. Follow `bridge/pkg/sidecar/client.go` pattern.
  **Must NOT**: Implement actual logic, add deps, modify router.go
  **Category**: `quick` | **Blocks**: 5-8 | **Blocked By**: None
  **Acceptance**: `cd bridge && go build ./pkg/mcp/...` → SUCCESS
  **Commit**: Groups with Task 2 → `feat(bridge): add pkg/toolsidecar and pkg/translator stubs`

- [x] 2. Create `bridge/pkg/translator` Go Stub
  **What**: Minimal types/stubs fixing broken import in `bridge/pkg/mcp/router.go:25`. Follow `bridge/pkg/sidecar/client.go` pattern.
  **Must NOT**: Implement actual logic, add deps
  **Category**: `quick` | **Blocks**: 5-8 | **Blocked By**: None
  **Acceptance**: `cd bridge && go build ./pkg/mcp/...` → SUCCESS
  **Commit**: Groups with Task 1

- [x] 3. Fix Sidecar Qdrant Module Import + Re-enable
  **What**: Fix broken `Qdrant` type import in `sidecar/src/document/qdrant.rs`. Uncomment `pub mod qdrant;` in `sidecar/src/document/mod.rs`. Fix API compat with qdrant-client v1.7.
  **Must NOT**: Add RPCs, change proto, add encryption
  **Category**: `quick` | **Blocks**: 4, 22, 23 | **Blocked By**: None
  **Refs**: `sidecar/src/document/mod.rs:26`, `sidecar/src/document/qdrant.rs`
  **Acceptance**: `cd sidecar && cargo build --lib` → 0 errors
  **Commit**: `fix(sidecar): re-enable Qdrant module with fixed import`

- [x] 4. Add Qdrant Container to docker-compose.yml
  **What**: Add `qdrant` service (qdrant/qdrant:latest) to `docker-compose.yml`. New network `armorclaw-vault` subnet `172.29.0.0/24`. Ports 6333/6334 internal. Persistent volume `qdrant_data:/qdrant/storage`. Health check.
  **Must NOT**: Conflicting subnets, host port exposure, modify existing networks
  **Category**: `quick` | **Blocks**: 22, 23 | **Blocked By**: 3 (ideally)
  **Acceptance**: `docker compose config --services | grep qdrant` succeeds
  **Commit**: `feat(docker): add Qdrant vector DB service`

- [x] 5. Fix PDF Extractor (Sidecar gRPC Delegation)
  **What**: Fix broken lopdf API calls in `sidecar/src/document/pdf.rs`. Add test with known PDF. Update Go bridge `bridge/internal/skills/file_read.go` to delegate to sidecar.
  **Must NOT**: Change proto, replace lopdf without reason
  **Category**: `deep` | **Blocked By**: 1, 2
  **Refs**: `sidecar/src/document/pdf.rs`, `bridge/internal/skills/file_read.go`, `bridge/pkg/sidecar/client.go`
  **Acceptance**: `cd sidecar && cargo test --lib document::pdf` → PASS with real text
  **Commit**: Groups with Task 6 → `fix(sidecar): fix PDF and DOCX extraction`

- [x] 6. Fix DOCX Extractor (Sidecar gRPC Delegation)
  **What**: Fix docx-rs API issues in `sidecar/src/document/docx.rs`. Add test. Add DOCX delegation in Go bridge.
  **Must NOT**: Change proto, break XLSX
  **Category**: `deep` | **Blocked By**: 1, 2
  **Refs**: `sidecar/src/document/docx.rs`, `sidecar/src/document/xlsx.rs` (working pattern)
  **Acceptance**: `cd sidecar && cargo test --lib document::docx` → PASS
  **Commit**: Groups with Task 5

- [x] 7. Wire Sidecar gRPC Handlers — Blob Operations
  **What**: Wire `UploadBlob`, `DownloadBlob`, `ListBlobs`, `DeleteBlob` in `sidecar/src/grpc/server.rs` to S3 connector. Add tests.
  **Must NOT**: Change proto, add RPCs, modify connectors
  **Category**: `deep` | **Blocks**: 5, 6 | **Blocked By**: 1, 2
  **Refs**: `sidecar/src/grpc/server.rs`, `sidecar/src/connectors/s3.rs`
  **Acceptance**: `cd sidecar && cargo test --lib grpc::server` → PASS for 4 blob handlers
  **Commit**: Groups with Task 8 → `feat(sidecar): wire gRPC handlers to library implementations`

- [x] 8. Wire Sidecar gRPC Handlers — Document Processing
  **What**: Wire `ExtractText` (route to PDF/DOCX/XLSX/OCR), `ProcessDocument` (full pipeline), `HealthCheck` in `sidecar/src/grpc/server.rs`.
  **Must NOT**: Change proto, add document types
  **Category**: `deep` | **Blocks**: 5, 6 | **Blocked By**: 1, 2
  **Refs**: `sidecar/src/grpc/server.rs`, `sidecar/src/document/mod.rs`, `sidecar/src/document/ocr.rs`
  **Acceptance**: ExtractText with PDF returns text, with image triggers OCR
  **Commit**: Groups with Task 7

### Wave 1: Transport (5 tasks)

- [x] 9. Write TrustedProxyGuard Tests (TDD)
  **What**: Create `bridge/pkg/trust/proxy_guard_test.go` with 8 tests: allows trusted TCP, blocks untrusted TCP, skips Unix socket, IPv6 support, TTL refresh, extracts client IP, wipes headers untrusted, dual-check locking race test. Use `net.Pipe()`.
  **Category**: `deep` | **Blocks**: 10 | **Blocked By**: Wave 0
  **Refs**: `bridge/internal/skills/allowlist.go` (sync.RWMutex + net.IP pattern)
  **Acceptance**: 8 tests compile, all FAIL (TDD RED)
  **Commit**: Groups with Task 10 → `feat(bridge): implement TrustedProxyGuard at net.Conn level`

- [x] 10. Implement TrustedProxyGuard
  **What**: Create `bridge/pkg/trust/proxy_guard.go`. TTL-cached proxy IP (60s), sync.RWMutex, double-check locking, DNS resolution of `armorclaw-proxy` with `ARMORCLAW_PROXY_IP` env override. `Check(net.Addr) error`: UnixAddr→nil, TCPAddr→IP check. IPv4+IPv6.
  **Must NOT**: Use http.Handler, parse HTTP headers, modify handleConnection
  **Category**: `deep` | **Blocks**: 11 | **Blocked By**: 9
  **Acceptance**: All 8 tests pass, no races with `-race` flag
  **Commit**: Groups with Task 9

- [x] 11. Guard All 3 handleConnection() Entry Points
  **What**: Add guard to Server struct (server.go:129), Config (server.go:154). Init in New() when tcp. Insert `guard.Check(conn.RemoteAddr())` at top of: (1) `bridge/pkg/rpc/server.go:1021`, (2) `bridge/pkg/agent/injection.go:104`, (3) `bridge/pkg/socket/server.go:236`. Wire in main.go:2561.
  **Must NOT**: Modify existing middleware, guard when unix transport
  **Category**: `unspecified-high` | **Blocks**: 13 | **Blocked By**: 10
  **Acceptance**: `grep -rn "guard.Check" bridge/pkg/rpc/server.go bridge/pkg/agent/injection.go bridge/pkg/socket/server.go` → 3 matches
  **Commit**: `feat(bridge): guard all handleConnection entry points`

- [x] 12. Update Caddyfile with Bridge API Routes
  **What**: Add `/api*`, `/health`, `/discover` → bridge to root `Caddyfile`. Keep Matrix routes.
  **Must NOT**: Add Caddy auth/WAF/rate-limiting, remove Matrix routes
  **Category**: `quick` | **Blocks**: 13 | **Blocked By**: None
  **Refs**: `Caddyfile`, `configs/Caddyfile.template` (reference with bridge routes)
  **Acceptance**: Caddyfile has bridge routes
  **Commit**: `feat(caddy): update Caddyfile with bridge API routes`

- [x] 13. End-to-End Transport Integration Test
  **What**: Create `tests/test_transport_guard.sh`: start stack, verify Caddy routes, verify guard blocks untrusted, verify health.
  **Category**: `unspecified-high` | **Blocked By**: 11, 12
  **Acceptance**: `bash tests/test_transport_guard.sh` → exit 0
  **Commit**: `test(transport): end-to-end TrustedProxyGuard integration test`

### Wave 2: Secure Ingestion (6 tasks)

- [x] 14. Write YARA Scanner Tests (TDD)
  **What**: Create `bridge/pkg/yara/scanner_test.go` with 9 tests: clean PDF, EICAR, macro DOCX, not found, empty, valid rules, invalid rules, missing file, concurrent safety. Testdata: EICAR string, clean text, test rules.
  **Category**: `deep` | **Blocks**: 15 | **Blocked By**: Wave 0
  **Acceptance**: 9 tests compile, all FAIL
  **Commit**: Groups with Task 15 → `feat(bridge): implement YARA disk-based scanner`

- [x] 15. Implement YARA Disk-Based Scanner
  **What**: Add `github.com/hillu/go-yara/v4` to go.mod. Create `bridge/pkg/yara/scanner.go`: `InitYARA(ruleFile)`, `ScanFileForMalware(filePath) → (clean, error)` using `ScanFile()` (disk-based). Update Dockerfile for libyara-dev if needed.
  **Must NOT**: Use ScanMem/ScanBytes, allocate []byte
  **Category**: `deep` | **Blocks**: 16, 19 | **Blocked By**: 14
  **Acceptance**: All 9 tests pass with CGO_ENABLED=1
  **Commit**: Groups with Task 14

- [x] 16. Create Starter YARA Ruleset
  **What**: Create `bridge/configs/yara_rules.yar` with 10-15 rules: EICAR, macro/VBA, JS in PDF, PE headers, CVE patterns, obfuscated scripts. Each with meta/strings/condition sections.
  **Must NOT**: Exceed 15 rules, include non-document formats, add threat intel feeds
  **Category**: `quick` | **Blocked By**: 15
  **Acceptance**: Rules compile and load, count ≥ 10
  **Commit**: `feat(bridge): add starter YARA ruleset`

- [x] 17. Write ONNX OCR Backend Tests (TDD)
  **What**: Add 7 tests to OCR test module: model loading, text extraction, fallback to Tesseract, missing model, backend selection (tesseract/onnx/auto). Test fixture PNG with known text.
  **Category**: `deep` | **Blocks**: 18 | **Blocked By**: Wave 0
  **Refs**: `sidecar/src/document/ocr.rs` (OcrExtractor + OcrConfig)
  **Acceptance**: 7 tests compile, all FAIL
  **Commit**: Groups with Task 18 → `feat(sidecar): add ONNX OCR tier via tract-onnx`

- [x] 18. Implement ONNX OCR Tier via tract-onnx
  **What**: Add `tract-onnx` to Cargo.toml. Create `scripts/download_onnx_model.sh` (paddle_ocr_v2.onnx, SHA256 verify). Extend OcrConfig with backend enum (Tesseract/Onnx/Auto). Implement OnnxBackend. Auto mode: ONNX first, Tesseract fallback if < 50 chars.
  **Must NOT**: Replace Tesseract, use ONNX for embeddings, embed model, use onnxruntime-rs
  **Category**: `deep` | **Blocked By**: 17
  **Acceptance**: All 7 tests pass, download script works, 3 backend modes functional
  **Commit**: Groups with Task 17

- [x] 19. Create test_yara_heap_profile.sh
  **What**: Create `tests/test_yara_heap_profile.sh`: 1000 test files + 100MB file, run scanner with pprof, assert heap < 5MB for large file, < 50MB for batch.
  **Category**: `quick` | **Blocked By**: 15
  **Acceptance**: Script exits 0, heap bounds verified
  **Commit**: `test(perf): add YARA heap profiling test`

### Wave 3: Encrypted Vector Vault (5 tasks)

- [x] 20. Write XChaCha20 Encryption Tests (TDD)
  **What**: Create `sidecar/src/encryption/` with 9 tests: roundtrip, 24-byte nonce, ciphertext differs, tamper detection, different nonces, empty/large plaintext, key versioning, deterministic nonce (HMAC-based).
  **Must NOT**: Use 12-byte nonce (ChaCha20), must be 24-byte (XChaCha20)
  **Category**: `deep` | **Blocks**: 21 | **Blocked By**: Wave 0
  **Refs**: chacha20poly1305 crate docs, spec split_storage.rs
  **Acceptance**: 9 tests compile, all FAIL
  **Commit**: Groups with Task 21 → `feat(sidecar): implement XChaCha20Poly1305 AEAD encryption`

- [x] 21. Implement XChaCha20Poly1305 AEAD Module
  **What**: Add `chacha20poly1305`, `hmac`, `sha2` to sidecar/Cargo.toml. Create `sidecar/src/encryption/aead.rs`: AeadCipher, deterministic nonce via HMAC-SHA256(key_id||blob_id)[:24], key versioning, Zeroizing<Vec<u8>> for plaintext. Add `pub mod encryption;` to lib.rs.
  **Must NOT**: Random nonces, ChaCha20 (12-byte), add to rust-vault, change SQLCipher
  **Category**: `deep` | **Blocks**: 22, 23, 24 | **Blocked By**: 20
  **Refs**: `sidecar/src/security/shadowmap.rs` (Zeroizing pattern)
  **Acceptance**: All 9 tests pass, nonce = 24 bytes
  **Commit**: Groups with Task 20

- [x] 22. Write Split-Storage + Qdrant Integration Tests (TDD)
  **What**: Create `sidecar/src/split_storage/` with 8 tests: encrypted text, plain vectors, roundtrip, RBAC clearance (own/higher), irrecoverable halves, collection creation, key version.
  **Category**: `deep` | **Blocks**: 23 | **Blocked By**: 21, 4
  **Refs**: `sidecar/src/document/qdrant.rs`, spec split_storage.rs
  **Acceptance**: 8 tests compile, all FAIL
  **Commit**: Groups with Task 23 → `feat(sidecar): implement split-storage manager with Qdrant`

- [x] 23. Implement Split-Storage Manager + Qdrant Wiring
  **What**: Create `sidecar/src/split_storage/manager.rs`: SplitStorageManager with store_chunk (encrypt text, plain vector + encrypted text + clearance → Qdrant) and search_and_decrypt (RBAC filter clearance ≤ user_clearance, decrypt). Add `pub mod split_storage;` to lib.rs.
  **Must NOT**: Encrypt vectors, add RPCs, store keys in Qdrant
  **Category**: `unspecified-high` | **Blocked By**: 22
  **Acceptance**: All 8 tests pass, RBAC enforced
  **Commit**: Groups with Task 22

- [x] 24. Create test_xchacha_nonce_length.sh
  **What**: Create `tests/test_xchacha_nonce_length.sh`: encrypt test string, decode base64, assert first 24 bytes = nonce, verify total = 24 + plaintext + 16 (tag).
  **Category**: `quick` | **Blocked By**: 21
  **Acceptance**: Script exits 0, nonce = 24 bytes
  **Commit**: `test(crypto): add XChaCha nonce length test`

### Wave 4: Output & Provenance (3 tasks)

- [x] 25. Write PDF Generation Tests (TDD)
  **What**: Create `sidecar/src/output/` with 6 tests: generate PDF, magic bytes (%PDF-), non-empty, metadata, Unicode (CJK/Arabic), write to file.
  **Category**: `deep` | **Blocks**: 26 | **Blocked By**: Wave 3
  **Acceptance**: 6 tests compile, all FAIL
  **Commit**: Groups with Task 26 → `feat(sidecar): implement PDF generation module`

- [x] 26. Implement PDF Generation Module
  **What**: Add `genpdf` (or `printpdf`) to Cargo.toml. Create `sidecar/src/output/pdf.rs`: generate_pdf(text, metadata, path), PdfMetadata struct, text-searchable, Unicode font embedding, metadata in info dict. Add `pub mod output;` to lib.rs.
  **Must NOT**: DOCX/HTML output, image-based PDF, streaming
  **Category**: `unspecified-high` | **Blocked By**: 25
  **Acceptance**: All 6 tests pass, PDF has %PDF- magic bytes
  **Commit**: Groups with Task 25

- [x] 27. Implement HMAC-SHA256 Provenance Signing
  **What**: Create `sidecar/src/provenance/signer.rs`: ProvenanceSigner (HMAC-SHA256, Zeroizing key), generate_signature (8-byte truncated hex), format_provenance (`[Provenance: AC-v6-Sig:{sig} | Sess:{id}]`), verify_signature. 5 TDD tests: sign+verify, tampered fails, wrong key fails, format check, deterministic. Add `pub mod provenance;` to lib.rs.
  **Must NOT**: Blockchain/ledger, separate DB, new gRPC RPCs
  **Category**: `deep` | **Blocked By**: None (parallel with 25-26)
  **Refs**: Spec `rust-vault/src/provenance/signer.rs`, `sidecar/src/security/shadowmap.rs`
  **Acceptance**: All 5 tests pass, format matches spec
  **Commit**: `feat(sidecar): implement HMAC-SHA256 provenance signing`

---

## Final Verification Wave

> 4 review agents in PARALLEL. ALL must APPROVE. Get user "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`: Verify all Must Have present, Must NOT Have absent, evidence exists. Output: `VERDICT: APPROVE/REJECT`
- [x] F2. **Code Quality Review** — `unspecified-high`: Build + test + lint all code. No type assertions, empty catches, AI slop. Output: `VERDICT`
- [x] F3. **Real Manual QA** — `unspecified-high`: Execute all QA scenarios, test integration, edge cases. Output: `VERDICT`
- [x] F4. **Scope Fidelity Check** — `deep`: 1:1 spec-to-impl verification, no scope creep. Output: `VERDICT`

---

## Commit Strategy

| Tasks | Message | Gate |
|-------|---------|------|
| 1-2 | `feat(bridge): add pkg/toolsidecar and pkg/translator stubs` | `go build ./pkg/mcp/...` |
| 3 | `fix(sidecar): re-enable Qdrant module with fixed import` | `cargo build --lib` |
| 4 | `feat(docker): add Qdrant vector DB service` | `docker compose config \| grep qdrant` |
| 5-6 | `fix(sidecar): fix PDF and DOCX extraction` | `cargo test --lib document` |
| 7-8 | `feat(sidecar): wire gRPC handlers to library implementations` | `cargo test --lib grpc::server` |
| 9-10 | `feat(bridge): implement TrustedProxyGuard at net.Conn level` | `go test ./pkg/trust/... -v` |
| 11 | `feat(bridge): guard all handleConnection entry points` | `go build ./...` |
| 12 | `feat(caddy): update Caddyfile with bridge API routes` | `grep api Caddyfile` |
| 13 | `test(transport): end-to-end TrustedProxyGuard integration test` | `bash -n test_transport_guard.sh` |
| 14-15 | `feat(bridge): implement YARA disk-based scanner` | `CGO_ENABLED=1 go test ./pkg/yara/... -v` |
| 16 | `feat(bridge): add starter YARA ruleset` | `go test ./pkg/yara/... -run Rules` |
| 17-18 | `feat(sidecar): add ONNX OCR tier via tract-onnx` | `cargo test --lib document::ocr` |
| 19 | `test(perf): add YARA heap profiling test` | `bash -n test_yara_heap_profile.sh` |
| 20-21 | `feat(sidecar): implement XChaCha20Poly1305 AEAD encryption` | `cargo test --lib encryption` |
| 22-23 | `feat(sidecar): implement split-storage manager with Qdrant` | `cargo test --lib split_storage` |
| 24 | `test(crypto): add XChaCha nonce length test` | `bash -n test_xchacha_nonce_length.sh` |
| 25-26 | `feat(sidecar): implement PDF generation module` | `cargo test --lib output::pdf` |
| 27 | `feat(sidecar): implement HMAC-SHA256 provenance signing` | `cargo test --lib provenance` |

---

## Success Criteria

```bash
cd bridge && go build ./...                                                    # SUCCESS
cd sidecar && cargo build --lib                                                # 0 errors
cd bridge && go test ./pkg/trust/... ./pkg/yara/... -v                         # PASS
cd sidecar && cargo test --lib encryption split_storage provenance output -- -v  # PASS
bash tests/test_yara_heap_profile.sh                                           # heap < 50MB
bash tests/test_xchacha_nonce_length.sh                                        # nonce = 24 bytes
docker compose --profile sentinel up -d                                        # all healthy
```

### Final Checklist
- [ ] All Must Have present, all Must NOT Have absent
- [ ] Go + Rust tests pass, Docker stack healthy
- [ ] No TypeScript services, no http.Handler, SQLCipher unchanged, protos unchanged
