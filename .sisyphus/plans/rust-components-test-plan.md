# Plan: Expand VPS Tests for Rust Components

**Created:** 2026-04-05  
**Author:** Prometheus  
**Status:** Draft  
**Priority:** High

---

## Executive Summary

**Current State:** 
- ✅ 10 VPS test files (3,410 lines)
- ❌ 0 tests for Rust Vault sidecar
- ❌ 0 tests for Rust Office sidecar

**Goal:** Add comprehensive test coverage for both Rust components, ensuring they integrate correctly with the Go Bridge and meet production reliability standards.

**Estimated Effort:** 15-20 hours (2-3 sprints)

---

## Component Analysis

### Rust Vault Sidecar

**Purpose:** Security-hardened cryptographic enclave

**Key Features:**
- gRPC server on Unix socket (`/run/armorclaw/rust-vault.sock`)
- State bifurcation (vault.db + matrix_state.db)
- Network-layer BlindFill via CDP
- mTLS authentication
- Rate limiting (100 req/s)
- Circuit breaking
- Zeroization of secrets

**Test Categories Needed:**
1. Socket availability and permissions
2. gRPC API functionality
3. Database encryption (SQLCipher)
4. BlindFill injection (CDP)
5. Rate limiting and circuit breaking
6. mTLS certificate validation
7. Secret zeroization
8. Integration with Go Bridge

### Rust Office Sidecar

**Purpose:** Data plane for heavy I/O operations

**Key Features:**
- S3 connector (working)
- SharePoint connector (working)
- Document processing (PDF/DOCX working, XLSX/OCR stubs)
- Token validation (HMAC-SHA256)
- Circuit breakers and rate limiting
- Metrics (Prometheus)

**Test Categories Needed:**
1. Library availability (can be imported)
2. S3 operations (upload, download, list, delete)
3. SharePoint operations
4. Document processing (PDF/DOCX)
5. Token validation
6. Circuit breaker behavior
7. Rate limiting
8. Metrics collection
9. Integration with Go Bridge

---

## Test Plan

### Phase 1: Rust Vault Sidecar Tests (8-10 hours)

#### Test File: `tests/ssh/test_rust_vault.sh` (NEW)

**Estimated Lines:** 600-700

**Test Cases:**

| Test ID | Test Name | Description | Priority |
|---------|-----------|-------------|----------|
| RV-01 | Socket Availability | Verify Unix socket exists with 0600 permissions | P0 |
| RV-02 | gRPC Health Check | Call health check endpoint | P0 |
| RV-03 | Secret Store/Retrieve | Store and retrieve secret via gRPC | P0 |
| RV-04 | BlindFill Injection | Test CDP placeholder resolution | P0 |
| RV-05 | Rate Limiting | Verify 100 req/s limit enforced | P1 |
| RV-06 | Circuit Breaker | Test circuit opens after failures | P1 |
| RV-07 | mTLS Validation | Verify certificate required | P1 |
| RV-08 | Database Encryption | Verify SQLCipher encryption active | P1 |
| RV-09 | Zeroization | Verify secrets zeroized after use | P2 |
| RV-10 | State Bifurcation | Verify vault.db and matrix_state.db separate | P1 |
| RV-11 | Concurrency Limit | Verify 10 concurrent request limit | P1 |
| RV-12 | Matrix State Storage | Test Matrix crypto state storage | P1 |

#### Test File: `tests/ssh/test_rust_vault_integration.sh` (NEW)

**Estimated Lines:** 400-500

**Test Cases:**

| Test ID | Test Name | Description | Priority |
|---------|-----------|-------------|----------|
| RV-INT-01 | Bridge Secret Request | Bridge requests secret from Vault | P0 |
| RV-INT-02 | BlindFill Browser Flow | Secret injected into browser via CDP | P0 |
| RV-INT-03 | PII Approval Workflow | End-to-end PII request with approval | P0 |
| RV-INT-04 | Secret Rotation | Test secret rotation without restart | P1 |
| RV-INT-05 | Failure Recovery | Vault recovers from temporary failure | P1 |

---

### Phase 2: Rust Office Sidecar Tests (6-8 hours)

#### Test File: `tests/ssh/test_rust_sidecar.sh` (NEW)

**Estimated Lines:** 700-800

**Test Cases:**

| Test ID | Test Name | Description | Priority |
|---------|-----------|-------------|----------|
| RS-01 | Library Import | Verify library can be imported | P0 |
| RS-02 | S3 Upload | Upload file to S3 bucket | P0 |
| RS-03 | S3 Download | Download file from S3 bucket | P0 |
| RS-04 | S3 List | List objects in S3 bucket | P0 |
| RS-05 | S3 Delete | Delete object from S3 bucket | P0 |
| RS-06 | S3 Range Request | Download partial file (range) | P1 |
| RS-07 | S3 Streaming | Stream large file upload/download | P1 |
| RS-08 | SharePoint Auth | Authenticate with SharePoint | P1 |
| RS-09 | SharePoint Upload | Upload file to SharePoint | P1 |
| RS-10 | SharePoint Download | Download file from SharePoint | P1 |
| RS-11 | PDF Extraction | Extract text from PDF | P0 |
| RS-12 | DOCX Extraction | Extract text from DOCX | P0 |
| RS-13 | XLSX Stub | Verify helpful error returned | P2 |
| RS-14 | OCR Stub | Verify helpful error returned | P2 |
| RS-15 | Token Validation | Validate HMAC-SHA256 token | P0 |
| RS-16 | Token Expiry | Reject expired token | P0 |
| RS-17 | Circuit Breaker Open | Circuit opens after failures | P1 |
| RS-18 | Circuit Breaker Close | Circuit closes after recovery | P1 |
| RS-19 | Rate Limiting | Verify rate limit enforced | P1 |
| RS-20 | Metrics Export | Prometheus metrics available | P1 |

#### Test File: `tests/ssh/test_rust_sidecar_integration.sh` (NEW)

**Estimated Lines:** 400-500

**Test Cases:**

| Test ID | Test Name | Description | Priority |
|---------|-----------|-------------|----------|
| RS-INT-01 | S3 Delegation | Bridge delegates S3 operation to Sidecar | P0 |
| RS-INT-02 | PDF Processing | Bridge delegates PDF extraction | P0 |
| RS-INT-03 | DOCX Processing | Bridge delegates DOCX extraction | P0 |
| RS-INT-04 | Performance: S3 Upload | Benchmark upload speed | P1 |
| RS-INT-05 | Performance: PDF | Benchmark PDF extraction speed | P1 |
| RS-INT-06 | Error Propagation | Sidecar errors propagate to Bridge | P0 |

---

### Phase 3: Test Runner Updates (2-3 hours)

#### Update: `tests/ssh/run_all_tests.sh`

Add new test categories:

```bash
# New options:
-V, --vault          Run Rust Vault tests only
-S, --sidecar        Run Rust Sidecar tests only
-A, --all-rust       Run all Rust component tests

# New test categories:
vault               Rust Vault sidecar tests
sidecar             Rust Office sidecar tests
```

---

## Implementation Details

### Test Prerequisites

#### Rust Vault Tests

**Environment Variables:**
```bash
export RUST_VAULT_ENABLED=true
export RUST_VAULT_SOCKET_PATH=/run/armorclaw/rust-vault.sock
export SHARED_SECRET=test-secret-256-bit
```

**Required Tools:**
- `grpcurl` - For gRPC testing
- `openssl` - For certificate validation

#### Rust Sidecar Tests

**Environment Variables:**
```bash
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
export AWS_REGION=us-east-1
export S3_TEST_BUCKET=armorclaw-test-bucket
export SHARED_SECRET=test-secret-256-bit
```

**Required Tools:**
- `aws-cli` - For S3 verification
- `rustc` - For Sidecar library testing

---

## Success Criteria

### Phase 1 Completion (Rust Vault)

- [ ] All P0 tests pass (RV-01, RV-02, RV-03, RV-04)
- [ ] All P1 tests pass (RV-05 through RV-12)
- [ ] Integration tests pass (RV-INT-01 through RV-INT-05)
- [ ] Tests integrated into `run_all_tests.sh`

### Phase 2 Completion (Rust Sidecar)

- [ ] All P0 tests pass (RS-01, RS-02, RS-03, RS-04, RS-05, RS-11, RS-12, RS-15, RS-16)
- [ ] All P1 tests pass (RS-06 through RS-10, RS-17 through RS-20)
- [ ] Integration tests pass (RS-INT-01 through RS-INT-06)
- [ ] Tests integrated into `run_all_tests.sh`

### Phase 3 Completion (Test Runner)

- [ ] New test categories added to runner
- [ ] All tests run successfully via `--all-rust`
- [ ] JSON output supported for CI/CD

---

## Timeline

### Sprint 1 (Week 1-2): Rust Vault Tests

- Day 1-2: Create `test_rust_vault.sh` (P0 tests)
- Day 3-4: Create `test_rust_vault.sh` (P1/P2 tests)
- Day 5: Create `test_rust_vault_integration.sh`
- Day 6-7: Testing and fixes

### Sprint 2 (Week 3-4): Rust Sidecar Tests

- Day 1-2: Create `test_rust_sidecar.sh` (S3 tests)
- Day 3-4: Create `test_rust_sidecar.sh` (document/security tests)
- Day 5: Create `test_rust_sidecar_integration.sh`
- Day 6-7: Update test runner

### Sprint 3 (Week 5): Integration

- Day 1-2: Integration with existing tests
- Day 3-4: CI/CD integration
- Day 5: Final documentation

---

## Dependencies

### Required Tools

- `grpcurl` - For gRPC testing
- `aws-cli` - For S3 verification
- `openssl` - For certificate validation
- `rustc` - For Sidecar library testing

### Test Environment

- Running Rust Vault sidecar binary
- Compiled Rust Sidecar library
- Test S3 bucket (AWS credentials)
- Test documents (PDF, DOCX)

---

## Risks and Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Rust Vault binary not stable | High | Use mock gRPC server for initial tests |
| AWS credentials not available | Medium | Skip S3 tests if credentials missing |
| Test data cleanup fails | Low | Add aggressive cleanup on test failure |
| Integration tests flaky | Medium | Add retry logic with exponential backoff |

---

## Next Steps

1. **Review and Approve Plan**
   - Stakeholder review
   - Adjust timeline if needed

2. **Setup Test Environment**
   - Install required tools (grpcurl)
   - Configure AWS credentials
   - Create test S3 bucket

3. **Implement Phase 1**
   - Start with `test_rust_vault.sh`
   - Focus on P0 tests first

4. **Iterate and Expand**
   - Add P1/P2 tests
   - Move to Phase 2
   - Complete Phase 3

---

**Created by:** Prometheus  
**Date:** 2026-04-05  
**Version:** 1.0  
**Status:** Awaiting Approval
