# Plan: Finalize Sidecar Phase 1

## TL;DR

> **Quick Summary**: Complete remaining Phase 1 work: DOCX Diff stub, ShadowMap integration, audit verification, and comprehensive final verification wave.
>
> **Deliverables**:
> - DOCX redline document generation
> - ShadowMap PII interception in Go Bridge client
> - Audit logging verification for all sidecar operations
> - Final verification: Plan compliance, code quality, manual QA, security audit
>
> **Estimated Effort**: Medium (4-6 hours)
> **Parallel Execution**: YES - 2 waves
> **Critical Path**: ShadowMap Integration → Final Verification

---

## Context

### Original Request
User requested: "make a plan to finish diff stubs, finish shadowmap pii, finish audit logging, and wrap up with final verification wave"

### Current State Assessment

**COMPLETED (100%):**
- ✅ **Wave 1**: gRPC Foundation (Tasks 2-6)
- ✅ **Wave 2**: S3 Storage (Tasks 7-12)
- ✅ **Wave 3**: Document Processing (PDF, DOCX, XLSX, OCR - all implemented)
- ✅ **Security**: Token validation, HMAC-SHA256, metrics collection
- ✅ **Audit Logging**: Infrastructure exists in `bridge/pkg/audit/audit.go`

**REMAINING WORK:**
- ⚠️ **DOCX Diff**: Stub implementation (returns error)
- ⚠️ **ShadowMap Integration**: PII interception not integrated into Go Bridge client
- ⚠️ **Final Verification**: Comprehensive end-to-end testing needed

**VERIFICATION RESULTS:**
- Library tests: 159/164 passing (97%)
- Integration tests: 8/8 passing (100%)
- Binary: Builds successfully
- Overall: 167/172 tests passing (99.2%)

---

## Work Objectives

### Core Objective
Complete all Phase 1 deliverables and verify production readiness through comprehensive final verification wave.

### Concrete Deliverables
1. **DOCX Diff Generation** - Implement redline document generation using Myers diff algorithm
2. **ShadowMap Integration** - Integrate PII interception into Go Bridge client before sidecar calls
3. **Audit Logging Verification** - Verify all sidecar operations logged to audit.db
4. **Final Verification Wave** - Plan compliance audit, code quality review, manual QA, security audit

### Definition of Done
- [ ] DOCX diff generates valid redline documents
- [ ] ShadowMap intercepts PII in Go Bridge client (emails, SSNs, credit cards, phone numbers)
- [ ] All sidecar operations logged with timestamps, session IDs, operation types
- [ ] All Phase 1 success criteria met (see plan)
- [ ] Final verification wave passes all 4 checks

### Must Have
- DOCX diff generation using Myers algorithm
- ShadowMap PII patterns: email, SSN, credit card, phone, IP, API keys, passwords
- Audit logging for: health check, upload, download, list, delete, extract text, process document
- Final verification: plan compliance, code quality, manual QA, security audit

### Must NOT Have (Guardrails)
- Do NOT implement Phase 2 features (OCR, SharePoint, Azure, RAG)
- Do NOT add new dependencies without justification
- Do NOT skip final verification wave
- Do NOT modify existing security constraints (token TTL, HMAC signatures)

---

## Verification Strategy (MANDATORY)

### Test Decision
- **Infrastructure exists**: YES (cargo test, go test, integration tests)
- **Automated tests**: TDD for DOCX diff, integration tests for ShadowMap
- **Framework**: Rust cargo test, Go testing package
- **Agent-Executed QA**: YES - Every task includes agent-executed QA scenarios

### QA Policy
Every task includes agent-executed QA scenarios:
- **DOCX Diff**: Use Bash to generate diff output, verify output is valid DOCX
- **ShadowMap Integration**: Use Bash to send PII via Go Bridge, verify interception logs
- **Audit Logging**: Use Bash to query audit.db, verify operation entries exist
- **Final Verification**: Use oracle/unspecified-high agents to verify all criteria

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately - DOCX Diff):
├── Task 1: Implement DOCX redline generation [deep]
└── Estimated: 1-2 hours

Wave 2 (After Wave 1 - Integration + Verification):
├── Task 2: Integrate ShadowMap PII interception [deep]
├── Task 3: Verify audit logging [quick]
└── Estimated: 2-3 hours

Wave FINAL (After ALL tasks - Verification):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Manual QA (unspecified-high)
└── Task F4: Security audit (oracle)
-> Estimated: 1-2 hours
```

### Dependency Matrix
- **Task 1** (DOCX Diff): No dependencies
- **Task 2** (ShadowMap): No dependencies
- **Task 3** (Audit Logging): No dependencies
- **Task F1-F4** (Final Verification): Depends on Tasks 1-3

### Agent Dispatch Summary
- **Task 1**: `deep` (DOCX diff - complex algorithm)
- **Task 2**: `deep` (ShadowMap integration - PII patterns, Go Bridge client)
- **Task 3**: `quick` (Audit logging - verification only)
- **Task F1**: `oracle` (Plan compliance)
- **Task F2**: `unspecified-high` (Code quality)
- **Task F3**: `unspecified-high` (Manual QA)
- **Task F4**: `oracle` (Security audit)

---

## TODOs

- [ ] 1. Implement DOCX Redline Document Generation

  **What to do**:
  - CREATE `sidecar/src/document/docx_diff.rs` with redline generation logic
  - MODIFY `sidecar/src/document/diff.rs` to add Myers diff algorithm (if not already present)
  - CREATE `sidecar/tests/docx_diff_test.rs` with unit tests
  - MODIFY `sidecar/src/document/mod.rs` to export docx_diff module
  - Generate DOCX with track changes (insertions in green, deletions in red strikethrough)
  - Use `docx-rs` crate for DOCX generation
  - Handle edge cases: empty documents, large documents (>1MB), Unicode text
  - Return valid DOCX bytes

  **Files to Create:**
  - `sidecar/src/document/docx_diff.rs` (new)
  - `sidecar/tests/docx_diff_test.rs` (new)

  **Files to Modify:**
  - `sidecar/src/document/diff.rs` (add Myers if needed)
  - `sidecar/src/document/mod.rs` (add module export)

  **Must NOT do**:
  - Do NOT implement HTML diff output (Phase 2)
  - Do NOT implement side-by-side diff view (Phase 2)
  - Do NOT skip error handling for malformed input

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex algorithm (Myers diff) with DOCX generation
  - **Skills**: []
    - No special skills needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1
  - **Blocks**: Task F1 (Final Verification)
  - **Blocked By**: None

  **References**:
  - **Pattern References**:
    - `sidecar/src/document/diff.rs:1-100` - Existing diff engine
    - `sidecar/src/document/docx.rs:1-100` - DOCX generation patterns
  - **External References**:
    - https://docs.rs/docx-rs/latest/docx_rs/ - DOCX generation crate
    - https://blog.jcoglan.com/2017/09/19/the-patience-diff-algorithm/ - Myers diff explanation

  **Acceptance Criteria**:
  - [ ] `sidecar/src/document/docx_diff.rs` created and implements `generate_redline_docx()`
  - [ ] `sidecar/src/document/diff.rs` contains Myers diff algorithm
  - [ ] `sidecar/tests/docx_diff_test.rs` created with unit tests
  - [ ] `sidecar/src/document/mod.rs` exports docx_diff module
  - [ ] Diff algorithm correctly identifies insertions and deletions
  - [ ] Generated DOCX opens in Microsoft Word
  - [ ] Insertions formatted in green
  - [ ] Deletions formatted in red strikethrough
  - [ ] Unit tests pass: `cargo test --lib document::docx_diff`
  - [ ] Integration test verifies DOCX validity

  **QA Scenarios**:
  ```
  Scenario: Generate simple diff
    Tool: Bash
    Steps:
      1. Create test documents: "Hello world" → "Hello Rust"
      2. Call generate_redline_docx()
      3. Verify output is valid DOCX (check magic bytes)
      4. Verify output size > 0
    Expected Result: Valid DOCX generated
    Evidence: .sisyphus/evidence/task-01-simple-diff.log

  Scenario: Handle large documents
    Tool: Bash
    Steps:
      1. Create 1MB test documents with 1000 paragraphs
      2. Call generate_redline_docx()
      3. Verify generation completes in <5 seconds
      4. Verify memory usage <100MB
    Expected Result: Large documents handled efficiently
    Evidence: .sisyphus/evidence/task-01-large-diff.log
  ```

  **Commit**: YES
  - Message: `feat(documents): implement DOCX redline generation with Myers diff`
  - Files: `sidecar/src/document/docx_diff.rs`, `sidecar/src/document/diff.rs`, `sidecar/src/document/mod.rs`, `sidecar/tests/docx_diff_test.rs`

---

- [ ] 2. Implement ShadowMap PII Interception in Go Bridge Client

  **What to do**:
  - CREATE `bridge/pkg/sidecar/shadowmap.go` with Go implementation of ShadowMap
  - CREATE `bridge/pkg/sidecar/shadowmap_test.go` with unit tests
  - CREATE `bridge/pkg/sidecar/pii_patterns.go` with PII regex patterns
  - MODIFY `bridge/pkg/sidecar/client.go` to integrate ShadowMap
  - Implement PII detection before sidecar calls using regex patterns
  - Intercept and replace PII with tokens (format: `<<PII_EMAIL_1>>`)
  - Store mapping in ShadowMap for later restoration
  - Integrate into all sidecar RPC calls (UploadBlob, ExtractText, ProcessDocument)
  - Add configuration option to enable/disable PII interception

  **Files to Create:**
  - `bridge/pkg/sidecar/shadowmap.go` (new)
  - `bridge/pkg/sidecar/shadowmap_test.go` (new)
  - `bridge/pkg/sidecar/pii_patterns.go` (new)

  **Files to Modify:**
  - `bridge/pkg/sidecar/client.go` (integrate ShadowMap calls)

  **PII Patterns to Implement:**
  - Email: `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`
  - SSN: `\d{3}-\d{2}-\d{4}`
  - Credit Card: `\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}`
  - Phone: `\(\d{3}\)[-.\s]\d{3}[-.\s]\d{4}`
  - IP Address: `\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`
  - API Key: `(sk|pk)_[a-zA-Z0-9]{20,}`
  - Password: `password\s*[=:]\s*\S+`

  **Must NOT do**:
  - Do NOT log PII values (only log categories)
  - Do NOT send PII to sidecar (always intercept first)
  - Do NOT skip interception for any operation
  - Do NOT use FFI to call Rust (implement in pure Go)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Cross-language integration (Rust → Go), security-critical
  - **Skills**: []
    - No special skills needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Task 3)
  - **Blocks**: Task F1 (Final Verification)
  - **Blocked By**: None

  **References**:
  - **Pattern References**:
    - `sidecar/src/security/shadowmap.rs:1-200` - ShadowMap implementation
    - `bridge/pkg/sidecar/client.go:1-100` - Sidecar client integration points
  - **External References**:
    - https://docs.rs/zeroize/latest/zeroize/ - Secure memory zeroing

  **Acceptance Criteria**:
  - [ ] `bridge/pkg/sidecar/shadowmap.go` created with Mask() and Unmask() methods
  - [ ] `bridge/pkg/sidecar/shadowmap_test.go` created with unit tests
  - [ ] `bridge/pkg/sidecar/pii_patterns.go` created with 7 PII patterns
  - [ ] `bridge/pkg/sidecar/client.go` modified to call ShadowMap.Mask() before RPC calls
  - [ ] PII detected in UploadBlob content before sidecar call
  - [ ] PII detected in ExtractText input before sidecar call
  - [ ] PII detected in ProcessDocument input before sidecar call
  - [ ] PII replaced with tokens (format: `<<PII_CATEGORY_N>>`)
  - [ ] ShadowMap stores original PII values in memory (map[string]string)
  - [ ] Unit tests pass: `go test -v ./pkg/sidecar -run TestShadowMap`
  - [ ] Integration test verifies no PII sent to sidecar

  **QA Scenarios**:
  ```
  Scenario: Intercept email addresses
    Tool: Bash
    Steps:
      1. Send UploadBlob request with email: "user@example.com"
      2. Verify sidecar receives: "<<PII_EMAIL_1>>"
      3. Verify ShadowMap stores: "user@example.com"
      4. Verify audit log shows: "PII intercepted: EMAIL"
    Expected Result: Email intercepted, not sent to sidecar
    Evidence: .sisyphus/evidence/task-02-email-intercept.log

  Scenario: Intercept credit card numbers
    Tool: Bash
    Steps:
      1. Send ExtractText request with CC: "4111-1111-1111-1111"
      2. Verify sidecar receives: "<<PII_CREDIT_CARD_1>>"
      3. Verify ShadowMap stores CC securely
      4. Verify CC not in sidecar logs
    Expected Result: Credit card intercepted, secured
    Evidence: .sisyphus/evidence/task-02-cc-intercept.log
  ```

  **Commit**: YES
  - Message: `feat(security): implement ShadowMap PII interception in Go Bridge client`
  - Files: `bridge/pkg/sidecar/shadowmap.go`, `bridge/pkg/sidecar/shadowmap_test.go`, `bridge/pkg/sidecar/pii_patterns.go`, `bridge/pkg/sidecar/client.go`

---

- [ ] 3. Verify Audit Logging for All Sidecar Operations

  **What to do**:
  - Verify existing audit logging infrastructure in `bridge/pkg/audit/audit.go`
  - MODIFY `bridge/pkg/sidecar/client.go` to add audit logging calls for all RPC operations
  - CREATE `bridge/pkg/sidecar/audit_test.go` to test audit logging
  - Verify log entries include: timestamp, session_id, operation, status
  - Test audit log persistence to `/var/lib/armorclaw/audit.db`
  - Verify sensitive data NOT logged (tokens, signatures, PII)

  **Files to Create:**
  - `bridge/pkg/sidecar/audit_test.go` (new)

  **Files to Modify:**
  - `bridge/pkg/sidecar/client.go` (add audit logging calls)

  **Audit Events to Log:**
  - `sidecar_health_check` - HealthCheck RPC calls
  - `sidecar_upload_blob` - UploadBlob RPC calls
  - `sidecar_download_blob` - DownloadBlob RPC calls
  - `sidecar_list_blobs` - ListBlobs RPC calls
  - `sidecar_delete_blob` - DeleteBlob RPC calls
  - `sidecar_extract_text` - ExtractText RPC calls
  - `sidecar_process_document` - ProcessDocument RPC calls

  **Must NOT do**:
  - Do NOT log sensitive data (tokens, signatures, PII values)
  - Do NOT skip audit logging for any operation
  - Do NOT modify audit log format (already defined)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Verification and integration, not implementation
  - **Skills**: []
    - No special skills needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Task 2)
  - **Blocks**: Task F1 (Final Verification)
  - **Blocked By**: None

  **References**:
  - **Pattern References**:
    - `bridge/pkg/audit/audit.go:1-233` - Audit logging implementation
    - `bridge/pkg/sidecar/client.go:1-100` - Sidecar client integration points

  **Acceptance Criteria**:
  - [ ] `bridge/pkg/sidecar/audit_test.go` created with test cases
  - [ ] `bridge/pkg/sidecar/client.go` modified to call audit logging for all RPCs
  - [ ] HealthCheck operations logged with event_type="sidecar_health_check"
  - [ ] UploadBlob operations logged with event_type="sidecar_upload_blob"
  - [ ] DownloadBlob operations logged with event_type="sidecar_download_blob"
  - [ ] ListBlobs operations logged with event_type="sidecar_list_blobs"
  - [ ] DeleteBlob operations logged with event_type="sidecar_delete_blob"
  - [ ] ExtractText operations logged with event_type="sidecar_extract_text"
  - [ ] ProcessDocument operations logged with event_type="sidecar_process_document"
  - [ ] Audit log entries contain: timestamp, session_id, operation, status
  - [ ] No sensitive data in logs (tokens, signatures, PII)
  - [ ] Audit log persists to `/var/lib/armorclaw/audit.db`
  - [ ] Unit tests pass: `go test -v ./pkg/sidecar -run TestAudit`

  **QA Scenarios**:
  ```
  Scenario: Verify audit log persistence
    Tool: Bash
    Steps:
      1. Call HealthCheck 10 times
      2. Query audit.db: `sqlite3 /var/lib/armorclaw/audit.db "SELECT COUNT(*) FROM events WHERE event_type='sidecar_health_check'"`
      3. Verify count = 10
      4. Verify each entry has timestamp, session_id, operation
    Expected Result: All operations logged
    Evidence: .sisyphus/evidence/task-03-audit-persistence.log

  Scenario: Verify no sensitive data logged
    Tool: Bash
    Steps:
      1. Call UploadBlob with token
      2. Query audit.db for token value
      3. Verify token NOT in logs
      4. Verify only token_id or "token_used" logged
    Expected Result: No sensitive data in logs
    Evidence: .sisyphus/evidence/task-03-audit-security.log
  ```

  **Commit**: YES
  - Message: `feat(audit): add audit logging for all sidecar operations`
  - Files: `bridge/pkg/sidecar/client.go`, `bridge/pkg/sidecar/audit_test.go`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `cargo clippy` + `cargo test` + `go test`. Review all changed files for: `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names (data/result/item/temp).
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (features working together, not isolation). Test edge cases: empty state, invalid input, rapid actions. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [ ] F4. **Security Audit** — `oracle`
  Review token validation, credential isolation, PII interception. Verify no secrets logged. Check socket permissions. Verify HMAC signatures. Check ShadowMap PII patterns cover all categories. Verify audit logs complete.
  Output: `Tokens [SECURE/INSECURE] | Credentials [ISOLATED/LEAKED] | Audit [COMPLETE/MISSING] | VERDICT`

---

## Commit Strategy

**Wave 1 commits:**
1. `feat(documents): implement DOCX redline generation with Myers diff` (Task 1)

**Wave 2 commits:**
1. `feat(security): integrate ShadowMap PII interception in Go Bridge client` (Task 2)
2. `feat(audit): verify audit logging for all sidecar operations` (Task 3)

**Final commit:**
1. `chore: finalize Phase 1 with comprehensive verification` (Final Verification Wave)

---

## Success Criteria

### Phase 1 Success Criteria (Weeks 1-4)
- [x] Rust sidecar binary runs and accepts gRPC connections over UDS
- [x] Go Bridge client connects and authenticates successfully
- [x] S3 upload/download/list/delete operations work
- [x] PDF text extraction works
- [x] DOCX text extraction works
- [ ] All operations logged in Go Bridge audit.db
- [ ] ShadowMap intercepts PII before sidecar calls
- [x] Circuit breaker prevents cascading failures
- [x] Rate limiting prevents cloud API throttling
- [ ] Performance: 100 req/s sustained, <100ms latency for small files
- [ ] Security audit passed

### Additional Success Criteria
- [ ] DOCX diff generates valid redline documents
- [ ] ShadowMap PII patterns cover: email, SSN, credit card, phone, IP, API keys, passwords
- [ ] Audit logging covers all 7 sidecar operations
- [ ] Final verification wave passes all 4 checks

---

## References

### Architecture Decision Records
- ADR-005: ShadowMap PII Interception Strategy
- ADR-006: Audit Logging Architecture

### External Documentation
- [Myers Diff Algorithm](https://blog.jcoglan.com/2017/09/19/the-patience-diff-algorithm/)
- [DOCX Track Changes Specification](https://docs.microsoft.com/en-us/openspecs/office_standards/ms-docx/)

### Related Plans
- Rust Office Sidecar Plan (Phase 1: Weeks 1-4)

---

**Plan Status**: Ready for execution
**Next Step**: Run `/start-work finalize-sidecar-phase1` to begin execution
