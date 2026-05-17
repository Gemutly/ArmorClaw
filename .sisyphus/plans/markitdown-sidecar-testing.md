# MarkItDown Sidecar — Testing Plan

## TL;DR

> **Quick Summary**: Comprehensive testing plan covering all 5 implementation tasks of the MarkItDown Polyglot Sidecar. Split into 2 phases: Phase 1 (local unit + integration tests runnable without Docker) and Phase 2 (Docker container + security tests requiring a VPS/Docker environment). Real test fixture files generated programmatically for all 6 supported formats.
> 
> **Deliverables**:
> - `sidecar-python/test_worker.py` — Python server unit tests (socket binding, format mapping, threshold streaming, TTL)
> - `sidecar-python/conftest.py` — Shared pytest fixtures (real .xlsx/.msg/.pptx/.doc/.xls/.ppt files)
> - `bridge/pkg/sidecar/office_client_e2e_test.go` — Go → Python end-to-end integration tests
> - `sidecar-python/test_docker_integration.py` — Docker container lifecycle + security validation
> 
> **Estimated Effort**: Medium
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: T1 (fixtures) → T2 (Python tests) + T3 (Go E2E) → T4 (Docker tests)

---

## Context

### What We're Testing
The MarkItDown Polyglot Sidecar feature (5 commits, 15 files, 1692 lines):
- **T1** (stubs): Python gRPC stubs from existing proto — import verification
- **T2** (server): Python gRPC server with MarkItDown + threshold streaming + Dockerfile
- **T3** (routing): Go Bridge 3-layer routing (native text bypass + compound magic+format switch + strict drop)
- **T4** (deploy): Docker Compose with symmetrical containment + AppArmor profile
- **T5** (interceptor): HMAC-SHA256 token validation interceptor

### Existing Test Coverage
- **Go**: 18 routing tests in `office_client_test.go` (all pass)
- **Go**: 25+ client/PII/retry tests in `client_test.go` (2 pre-existing failures unrelated to this feature)
- **Python**: 12 interceptor tests in `test_interceptor.py` (all pass)

### What's NOT Tested Yet
1. Python server actually starting, binding Unix socket, accepting connections
2. ExtractText RPC converting real .xlsx/.msg/.pptx/.doc/.xls/.ppt files via MarkItDown
3. Threshold streaming boundary (<10MB BytesIO vs >10MB temp file)
4. TTL recycling (50 requests → graceful shutdown)
5. Go → Python end-to-end: Go client calls Python server with real files
6. Docker container: starts, socket appears, no network, tmpfs, AppArmor
7. gRPC version metadata in responses

### Metis Gap Analysis (Self-Performed)
**Identified Gaps** (addressed):
- **New test files only**: Do NOT modify existing test files — create new ones
- **Programmatic fixtures**: Generate .xlsx via openpyxl, .msg via olefile — no binary commits
- **No sudo for unit tests**: Docker tests separated into Phase 2
- **Edge cases**: Empty file, corrupt content, exact 10MB boundary, concurrent TTL drain

---

## Work Objectives

### Core Objective
Validate that all MarkItDown sidecar components work correctly in isolation (unit), together (integration), and in production configuration (Docker).

### Concrete Deliverables
- `sidecar-python/conftest.py` — pytest fixtures generating real test files for 6 formats
- `sidecar-python/test_worker.py` — Python server unit + integration tests
- `bridge/pkg/sidecar/office_client_e2e_test.go` — Go → Python E2E integration tests
- `sidecar-python/test_docker_integration.py` — Docker container + security validation

### Definition of Done
- [x] `pytest sidecar-python/test_worker.py` — all tests pass
- [x] `pytest sidecar-python/test_interceptor.py` — all 12 tests pass (regression)
- [x] `go test ./bridge/pkg/sidecar/...` — all routing + E2E tests pass
- [x] Real .xlsx/.msg/.pptx/.doc/.xls/.ppt files convert successfully via ExtractText
- [x] Threshold streaming tested at boundary (<10MB and >10MB)
- [x] TTL recycling verified (server exits after MAX_REQUESTS)
- [x] Docker container starts, socket appears, no network access confirmed
- [ ] AppArmor profile loaded and enforced
- [x] All "Must NOT" guardrails from implementation plan verified in tests

### Must Have
- Real test fixture files generated programmatically (openpyxl for .xlsx, olefile for .msg, etc.)
- Tests for all 6 formats: .xlsx, .pptx, .msg, .doc, .xls, .ppt
- Tests for all 3 routing layers (native text, compound validation, strict drop)
- Threshold streaming boundary test (exactly at and around 10MB)
- TTL recycling test (server exits after N requests)
- Docker container lifecycle + security tests
- Go → Python end-to-end with real gRPC calls

### Must NOT Have (Guardrails)
- Do NOT modify existing test files (`office_client_test.go`, `test_interceptor.py`)
- Do NOT commit binary test fixture files — generate in test setup
- Do NOT require `sudo` for Phase 1 (unit + integration tests)
- Do NOT test MarkItDown library internals — test OUR integration only
- Do NOT test Rust sidecar — out of scope
- Do NOT weaken security constraints to make tests pass
- Do NOT skip tests that require real file conversion — they're the whole point

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES
- **Automated tests**: Tests-after (this IS the testing plan)
- **Framework**: Go: `go test` / Python: `pytest`

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation — fixture generation):
└── Task 1: Test fixtures + conftest.py [quick]

Wave 2 (Core tests — MAX PARALLEL):
├── Task 2: Python server unit tests (depends: T1) [deep]
├── Task 3: Go → Python E2E integration tests (depends: T1) [deep]
└── Task 4: Python server edge case tests (depends: T1) [unspecified-high]

Wave 3 (Docker + security — requires Docker runtime):
└── Task 5: Docker container + AppArmor tests (depends: T2, T3) [unspecified-high]

Wave FINAL (After ALL tasks — regression + guardrails):
├── Task F1: Full regression suite (all tests) [quick]
└── Task F2: Must NOT guardrails audit [deep]

Critical Path: T1 → T2 → T5 → FINAL
Parallel Speedup: T2 + T3 + T4 run in parallel
Max Concurrent: 3 (Wave 2)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1 | - | T2, T3, T4 | 1 |
| T2 | T1 | T5, FINAL | 2 |
| T3 | T1 | T5, FINAL | 2 |
| T4 | T1 | FINAL | 2 |
| T5 | T2, T3 | FINAL | 3 |
| F1 | T1-T5 | - | FINAL |
| F2 | T1-T5 | - | FINAL |

### Agent Dispatch Summary

- **Wave 1**: 1 task — T1 → `quick`
- **Wave 2**: 3 tasks — T2 → `deep`, T3 → `deep`, T4 → `unspecified-high`
- **Wave 3**: 1 task — T5 → `unspecified-high`
- **FINAL**: 2 tasks — F1 → `quick`, F2 → `deep`

---

## TODOs

- [x] 1. Test Fixtures and conftest.py

  **What to do**:
  - Create `sidecar-python/conftest.py` with pytest fixtures that generate real test files:
    - `xlsx_file(tmp_path)` — use `openpyxl` to create a minimal .xlsx with 1 sheet, 3 rows, "Hello ArmorClaw" in A1
    - `pptx_file(tmp_path)` — use `python-pptx` to create a minimal .pptx with 1 slide, 1 text box containing "Test Presentation"
    - `msg_file(tmp_path)` — use `extract_msg` or raw OLE structure to create a minimal .msg with Subject="Test Email", Body="Hello from ArmorClaw"
    - `doc_file(tmp_path)` — create a minimal OLE .doc file using `olefile` with a valid Word document stream
    - `xls_file(tmp_path)` — create a minimal .xls file using `xlwt` with 1 sheet, 1 cell containing "Test"
    - `ppt_file(tmp_path)` — create a minimal OLE .ppt file using `olefile` with a valid PowerPoint stream
  - Each fixture returns a `pathlib.Path` to the generated file
  - Each fixture generates the file ONCE per session (scope="session") for performance
  - Add `python-pptx`, `xlwt` to `requirements-dev.txt` if not already present
  - Also add a `large_xlsx_file(tmp_path)` fixture that generates a .xlsx > 10MB for threshold streaming test
  - Add a `grpc_channel(tmp_path)` fixture that starts the Python server on a temp Unix socket, yields the channel, and stops the server in cleanup
  - Add a `secret_file(tmp_path)` fixture that writes a test HMAC secret to a temp file (for interceptor integration)

  **Must NOT do**:
  - Do NOT commit binary fixture files to git
  - Do NOT use real user data in fixtures — synthetic data only
  - Do NOT require network access for fixture generation

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single file creation, well-defined fixture pattern
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (foundation)
  - **Parallel Group**: Wave 1 (solo)
  - **Blocks**: T2, T3, T4
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `sidecar-python/test_interceptor.py` — existing pytest patterns (fixtures, parametrize, tmp_path usage)
  - `bridge/pkg/sidecar/client_test.go:setupTestServer()` — Go test server pattern (grpc.Server on temp socket)

  **API/Type References**:
  - `openpyxl.Workbook()` — creates .xlsx files
  - `pptx.Presentation()` — creates .pptx files
  - `olefile.OleFileIO` — creates/reads OLE files
  - `grpc.server()` — Python gRPC server creation
  - `worker.OfficeSidecarServicer` — the server class under test

  **External References**:
  - pytest fixtures: `@pytest.fixture(scope="session")`
  - openpyxl: `pip install openpyxl`
  - python-pptx: `pip install python-pptx`

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All 6 fixture files are valid and have correct magic bytes
    Tool: Bash (pytest)
    Preconditions: conftest.py created, dependencies installed
    Steps:
      1. pytest -v -k "test_fixture" sidecar-python/conftest.py (or inline test)
      2. For each fixture file: read first 8 bytes
      3. xlsx/pptx: assert bytes[0:4] == b"PK\x03\x04" (ZIP magic)
      4. msg/doc/xls/ppt: assert bytes[0:8] == b"\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1" (OLE magic)
    Expected Result: All 6 fixtures generate files with correct magic bytes
    Failure Indicators: Wrong magic bytes, file not created, import error
    Evidence: .sisyphus/evidence/task-1-fixtures.txt

  Scenario: gRPC channel fixture starts server and creates valid socket
    Tool: Bash (pytest)
    Preconditions: worker.py, proto stubs exist
    Steps:
      1. Use grpc_channel fixture in a test
      2. Call HealthCheck RPC via the channel
      3. Assert response is received
    Expected Result: Server starts, channel connects, HealthCheck succeeds
    Failure Indicators: Connection refused, socket not created, import error
    Evidence: .sisyphus/evidence/task-1-grpc-channel.txt
  ```

  **Commit**: YES
  - Message: `test(sidecar-python): add test fixtures and conftest for all 6 document formats`
  - Files: `sidecar-python/conftest.py`, `sidecar-python/requirements-dev.txt`
  - Pre-commit: `cd sidecar-python && python -c "import conftest"`

- [x] 2. Python Server Unit Tests

  **What to do**:
  - Create `sidecar-python/test_worker.py` with the following test classes:
  
  **`TestFormatMapping`**:
  - Test that all 6 MIME types map to correct extensions
  - Test that unrecognized MIME types return None
  - Test case insensitivity of MIME matching
  
  **`TestServerStartup`**:
  - Test server starts and binds to Unix socket (using grpc_channel fixture from conftest)
  - Test socket has correct permissions (0o600)
  - Test HealthCheck RPC returns healthy response
  - Test version metadata (`x-sidecar-server-version: 1.0.0`) present in response
  
  **`TestExtractTextFormats`** (parametrized for all 6 formats):
  - Test .xlsx conversion: send real xlsx bytes → assert "Hello ArmorClaw" in response.text
  - Test .pptx conversion: send real pptx bytes → assert "Test Presentation" in response.text
  - Test .msg conversion: send real msg bytes → assert email content in response.text
  - Test .doc conversion: send real doc bytes → assert text extracted
  - Test .xls conversion: send real xls bytes → assert "Test" in response.text
  - Test .ppt conversion: send real ppt bytes → assert text extracted
  - For each: assert `page_count >= 1`
  - For each: assert `metadata` map is populated
  
  **`TestExtractTextErrors`**:
  - Test empty document_content (0 bytes) → UNIMPLEMENTED or error
  - Test unsupported format string → server returns error (not crash)
  - Test corrupt magic bytes (ZIP header then garbage) → MarkItDown returns error, not crash
  
  **`TestThresholdStreaming`**:
  - Test small file (< 10MB) uses BytesIO path (verify via logging or mock)
  - Test large file (> 10MB) uses temp file path (verify via logging or mock)
  - Test exact 10MB boundary behavior
  
  **`TestTTLRecycling`**:
  - Test server tracks request count
  - Test server initiates shutdown after MAX_REQUESTS
  - Test concurrent request completes during drain period
  
  **`TestUnimplementedRPCs`**:
  - Test UploadBlob returns UNIMPLEMENTED
  - Test DownloadBlob returns UNIMPLEMENTED
  - Test ListBlobs returns UNIMPLEMENTED
  - Test DeleteBlob returns UNIMPLEMENTED
  - Test ProcessDocument returns UNIMPLEMENTED

  **Must NOT do**:
  - Do NOT modify `worker.py` to add test hooks — test the public API only
  - Do NOT import private functions — test through gRPC RPCs
  - Do NOT use `unittest.mock.patch` on MarkItDown for the format conversion tests — use real conversion

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex test suite with real file conversion, server lifecycle, and threshold logic
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T3, T4)
  - **Blocks**: T5, FINAL
  - **Blocked By**: T1 (needs conftest fixtures)

  **References**:

  **Pattern References**:
  - `sidecar-python/test_interceptor.py` — existing pytest patterns in this project
  - `bridge/pkg/sidecar/office_client_test.go:makeRoutingReq()` — pattern for constructing test requests

  **API/Type References**:
  - `sidecar-python/worker.py:OfficeSidecarServicer` — the servicer class under test
  - `sidecar-python/worker.py:FORMAT_MAP` — format MIME → extension mapping
  - `sidecar-python/worker.py:THRESHOLD_BYTES` — 10MB threshold constant
  - `sidecar-python/proto/sidecar_pb2.py` — ExtractTextRequest, ExtractTextResponse protobuf types
  - `sidecar-python/proto/sidecar_pb2_grpc.py` — SidecarServiceServicer, SidecarServiceStub

  **Test References**:
  - `bridge/pkg/sidecar/office_client_test.go:TestRouteExtractText_XLSX_RoutesToPython` — pattern for format-specific test
  - `sidecar-python/test_interceptor.py:TestValidateToken` — pattern for parametrized validation tests

  **External References**:
  - grpcio testing: `grpc_testing.server()` for in-process testing
  - pytest parametrize: `@pytest.mark.parametrize("format,mime", [...])`

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All 6 formats convert successfully
    Tool: Bash (pytest)
    Preconditions: conftest.py fixtures working, MarkItDown installed
    Steps:
      1. pytest -v -k "TestExtractTextFormats" sidecar-python/test_worker.py
      2. Assert all 6 parametrized tests pass
    Expected Result: 6/6 format conversions return non-empty text
    Failure Indicators: MarkItDown exception, empty text, gRPC error
    Evidence: .sisyphus/evidence/task-2-format-conversion.txt

  Scenario: Server starts and responds to HealthCheck
    Tool: Bash (pytest)
    Preconditions: worker.py exists, proto stubs generated
    Steps:
      1. pytest -v -k "TestServerStartup" sidecar-python/test_worker.py
      2. Assert HealthCheck returns healthy response
      3. Assert version metadata present
    Expected Result: HealthCheck succeeds, version 1.0.0 in metadata
    Failure Indicators: Connection refused, timeout, missing metadata
    Evidence: .sisyphus/evidence/task-2-healthcheck.txt

  Scenario: Threshold streaming boundary
    Tool: Bash (pytest)
    Preconditions: Server running
    Steps:
      1. Send <10MB file → assert BytesIO path used
      2. Send >10MB file → assert temp file path used
    Expected Result: Different code paths triggered at 10MB boundary
    Failure Indicators: Same path for both, temp file not cleaned up
    Evidence: .sisyphus/evidence/task-2-threshold.txt

  Scenario: TTL recycling triggers shutdown
    Tool: Bash (pytest)
    Preconditions: Server running with MAX_REQUESTS=5 (test override)
    Steps:
      1. Send 5 ExtractText requests
      2. Assert server signals shutdown intent
      3. Assert 6th request fails or server has stopped
    Expected Result: Server exits after MAX_REQUESTS
    Failure Indicators: Server continues past limit, no shutdown signal
    Evidence: .sisyphus/evidence/task-2-ttl.txt

  Scenario: Unimplemented RPCs return UNIMPLEMENTED
    Tool: Bash (pytest)
    Preconditions: Server running
    Steps:
      1. Call UploadBlob → assert UNIMPLEMENTED
      2. Call DownloadBlob → assert UNIMPLEMENTED
      3. Call ListBlobs → assert UNIMPLEMENTED
      4. Call DeleteBlob → assert UNIMPLEMENTED
      5. Call ProcessDocument → assert UNIMPLEMENTED
    Expected Result: All 5 RPCs return UNIMPLEMENTED status
    Failure Indicators: RPC succeeds, returns wrong error code
    Evidence: .sisyphus/evidence/task-2-unimplemented.txt
  ```

  **Commit**: YES
  - Message: `test(sidecar-python): add worker unit tests for server, format mapping, and streaming`
  - Files: `sidecar-python/test_worker.py`
  - Pre-commit: `cd sidecar-python && python -m pytest test_worker.py -v`

- [x] 3. Go → Python E2E Integration Tests

  **What to do**:
  - Create `bridge/pkg/sidecar/office_client_e2e_test.go` (NEW file — do NOT modify `office_client_test.go`)
  - This file tests the full path: Go `RouteExtractText` → gRPC → Python `OfficeSidecarServicer`
  - Uses `exec.Command("python3", "-m", "worker")` to start the Python server in test setup
  - Test setup:
    1. Create temp directory for socket
    2. Start Python server as subprocess with `SIDECAR_SOCKET=temp.sock` and `SIDECAR_SECRET_FILE=temp_secret`
    3. Wait for socket file to appear (poll with 100ms interval, 10s timeout)
    4. Create Go `Client` pointing to the temp socket
    5. Run tests
    6. Kill Python subprocess in cleanup
  
  **Test Cases**:
  - `TestE2E_XLSX_GoToPython` — Go sends real .xlsx bytes → Python converts → response.text contains expected content
  - `TestE2E_MSG_GoToPython` — Go sends real .msg bytes with OLE magic → Python converts → response non-empty
  - `TestE2E_NativeText_NoSidecar` — Go sends "text/plain" with "hello" → response.text == "hello", no sidecar call
  - `TestE2E_StrictDrop_ZIPMsgMismatch` — Go sends ZIP magic with "application/vnd.ms-outlook" → InvalidArgument
  - `TestE2E_StrictDrop_OLEXLSXMismatch` — Go sends OLE magic with "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" → InvalidArgument
  - `TestE2E_Docx_RoutesAway` — Go sends ZIP magic with "application/vnd.openxmlformats-officedocument.wordprocessingml.document" → confirm it does NOT call Python (expect connection error since no Rust sidecar, but verify routing target is correct via error message)
  - `TestE2E_VersionMetadata` — Go sends request → verify response metadata contains server version
  
  **Skip condition**: If `python3` is not available or `worker.py` fails to import, skip with `t.Skip("Python sidecar not available")`

  **Must NOT do**:
  - Do NOT modify `office_client.go` or `office_client_test.go`
  - Do NOT require Docker — pure subprocess test
  - Do NOT hardcode socket paths — use temp directories

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Cross-language E2E testing, subprocess management, gRPC channel lifecycle
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T2, T4)
  - **Blocks**: T5, FINAL
  - **Blocked By**: T1 (needs fixture generation pattern reference)

  **References**:

  **Pattern References**:
  - `bridge/pkg/sidecar/office_client_test.go:setupRoutingClients()` — Go gRPC test server pattern
  - `bridge/pkg/sidecar/client_test.go:setupTestServer()` — grpc.Server lifecycle management

  **API/Type References**:
  - `bridge/pkg/sidecar/office_client.go:RouteExtractText()` — the function under test
  - `bridge/pkg/sidecar/office_client.go:NewOfficeClient()` — client factory
  - `sidecar-python/worker.py` — Python server that will be started as subprocess
  - `bridge/pkg/sidecar/sidecar.pb.go:ExtractTextRequest` — request type construction

  **Test References**:
  - `bridge/pkg/sidecar/office_client_test.go:TestRouteExtractText_XLSX_RoutesToPython` — similar routing test but with mock server (this task uses REAL Python server)

  **External References**:
  - Go `os/exec` — subprocess management
  - Go `testing.T.Skip()` — conditional test skip

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Go routes .xlsx to Python and gets converted text
    Tool: Bash (go test)
    Preconditions: Python3 available, worker.py imports successfully
    Steps:
      1. go test -v -run TestE2E_XLSX_GoToPython ./bridge/pkg/sidecar/
      2. Assert response.text contains non-empty content
      3. Assert response.page_count > 0
    Expected Result: Real text extracted from .xlsx via Python MarkItDown
    Failure Indicators: Python server won't start, gRPC error, empty text
    Evidence: .sisyphus/evidence/task-3-e2e-xlsx.txt

  Scenario: Go routes .msg (OLE) to Python and gets converted text
    Tool: Bash (go test)
    Preconditions: Python3 available, olefile installed
    Steps:
      1. go test -v -run TestE2E_MSG_GoToPython ./bridge/pkg/sidecar/
      2. Assert response.text is non-empty
    Expected Result: OLE .msg converted via Python
    Failure Indicators: "unrecognized document format" (OLE magic bug), Python conversion error
    Evidence: .sisyphus/evidence/task-3-e2e-msg.txt

  Scenario: Native text bypass returns without sidecar call
    Tool: Bash (go test)
    Preconditions: None (no Python needed)
    Steps:
      1. go test -v -run TestE2E_NativeText_NoSidecar ./bridge/pkg/sidecar/
      2. Assert response.text == "hello world"
      3. Assert response.metadata["source"] == "bridge-native"
    Expected Result: Text returned without any gRPC call
    Failure Indicators: Sidecar connection attempted
    Evidence: .sisyphus/evidence/task-3-e2e-native.txt
  ```

  **Commit**: YES
  - Message: `test(bridge): add Go-Python E2E integration tests for ExtractText RPC`
  - Files: `bridge/pkg/sidecar/office_client_e2e_test.go`
  - Pre-commit: `cd bridge && go vet ./pkg/sidecar/ && go build ./pkg/sidecar/`

- [x] 4. Edge Case and Error Path Tests

  **What to do**:
  - Create `sidecar-python/test_edge_cases.py` with edge case and error path tests:
  
  **`TestEmptyPayload`**:
  - Send ExtractText with 0 bytes → assert server returns error (not crash)
  - Send ExtractText with 1 byte → assert server returns error
  - Send ExtractText with 7 bytes (below 8-byte magic threshold) → assert error
  
  **`TestCorruptFiles`**:
  - ZIP magic (PK\x03\x04) followed by 1MB of random bytes → assert MarkItDown error handled gracefully
  - OLE magic (D0CF11E0...) followed by garbage → assert MarkItDown error handled gracefully
  - Valid ZIP container but not a recognized office format (e.g., a .jar file) → assert appropriate error
  
  **`TestThresholdBoundary`**:
  - File exactly 10MB (10 * 1024 * 1024 bytes) → verify which path is taken
  - File 10MB - 1 byte → verify BytesIO path
  - File 10MB + 1 byte → verify temp file path
  - After temp file path: verify cleanup (no files left in /tmp/office_worker)
  
  **`TestConcurrentRequests`**:
  - Send 10 concurrent ExtractText requests → all succeed
  - Send requests during TTL drain → some succeed, some fail gracefully
  
  **`TestFormatStringVariants`**:
  - "application/vnd.ms-excel" vs "application/vnd.ms-excel.sheet.macroenabled.12" → both route to .xls handler
  - "application/vnd.ms-outlook" vs "application/vnd.ms-outlook-pst" → only first matches .msg
  - "TEXT/PLAIN" (uppercase) → still recognized as plain text (Go side, verify via test)
  
  **`TestTokenIntegration`**:
  - Valid token + valid request → success
  - Expired token + valid request → UNAUTHENTICATED
  - Missing token + valid request → UNAUTHENTICATED
  - Valid token + valid request → verify version metadata in response

  **Must NOT do**:
  - Do NOT test MarkItDown internal error handling — test OUR error wrapping
  - Do NOT modify worker.py or interceptor.py

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Edge case testing requires understanding of all boundary conditions
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T2, T3)
  - **Blocks**: FINAL
  - **Blocked By**: T1 (needs conftest fixtures)

  **References**:

  **Pattern References**:
  - `sidecar-python/test_interceptor.py:TestValidateToken` — token edge case patterns
  - `bridge/pkg/sidecar/office_client_test.go:TestRouteExtractText_ShortBuffer` — short payload pattern

  **API/Type References**:
  - `sidecar-python/worker.py:THRESHOLD_BYTES = 10 * 1024 * 1024` — threshold constant
  - `sidecar-python/interceptor.py:validate_token()` — token validation function
  - `bridge/pkg/sidecar/office_client.go:RouteExtractText()` — routing logic with edge cases

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Corrupt files handled gracefully without server crash
    Tool: Bash (pytest)
    Preconditions: Server running
    Steps:
      1. Send ZIP magic + 1MB garbage → assert gRPC error returned (not UNAVAILABLE)
      2. Send OLE magic + garbage → assert gRPC error returned
      3. Send second valid request → assert server is still alive
    Expected Result: Errors returned, server continues accepting requests
    Failure Indicators: Server crash, UNAVAILABLE on subsequent requests
    Evidence: .sisyphus/evidence/task-4-corrupt-files.txt

  Scenario: Threshold boundary at exactly 10MB
    Tool: Bash (pytest)
    Preconditions: Server running
    Steps:
      1. Create file of exactly 10MB
      2. Create file of 10MB-1
      3. Create file of 10MB+1
      4. Send each → assert all handled without crash
    Expected Result: All three sizes handled, correct code path for each
    Failure Indicators: Crash at boundary, memory spike, temp file leak
    Evidence: .sisyphus/evidence/task-4-threshold-boundary.txt

  Scenario: Token + request integration
    Tool: Bash (pytest)
    Preconditions: Server running with interceptor
    Steps:
      1. Valid token + valid .xlsx → assert success + version metadata
      2. Expired token + valid .xlsx → assert UNAUTHENTICATED
      3. Missing token + valid .xlsx → assert UNAUTHENTICATED
    Expected Result: Token validation and format conversion work together
    Failure Indicators: Valid token rejected, invalid token accepted
    Evidence: .sisyphus/evidence/task-4-token-integration.txt
  ```

  **Commit**: YES
  - Message: `test(sidecar-python): add edge case tests for threshold streaming, TTL, and corrupt files`
  - Files: `sidecar-python/test_edge_cases.py`
  - Pre-commit: `cd sidecar-python && python -m pytest test_edge_cases.py -v`

- [x] 5. Docker Container and Security Validation Tests

  **What to do**:
  - Create `sidecar-python/test_docker_integration.py` with container lifecycle and security tests
  - These tests require Docker runtime and are marked with `@pytest.mark.skipif` when Docker is unavailable
  - Skip condition: `shutil.which("docker") is None`
  
  **Test Setup**:
  - Build Docker image from `sidecar-python/Dockerfile` if not already built
  - Start container via `docker compose -f deploy/docker-compose.sidecar-py.yml up -d`
  - Wait for socket to appear at `/run/armorclaw/office-sidecar/sidecar-office.sock`
  - Run tests
  - Teardown: `docker compose -f deploy/docker-compose.sidecar-py.yml down -v`
  
  **`TestContainerLifecycle`**:
  - `test_container_starts` — `docker ps` shows container running
  - `test_socket_appears` — socket file exists and is a Unix socket
  - `test_socket_permissions` — socket has mode 0600
  - `test_container_uid` — process runs as UID 10001
  - `test_healthcheck` — HealthCheck RPC succeeds via the socket
  
  **`TestNetworkIsolation`**:
  - `test_no_inet` — `docker exec ... python -c "import socket; ... connect to 8.8.8.8"` → fails
  - `test_no_dns` — `docker exec ... python -c "socket.getaddrinfo('google.com', 80)"` → fails
  - `test_network_mode_none` — `docker inspect` shows NetworkMode == "none"
  
  **`TestFilesystemSecurity`**:
  - `test_read_only_root` — `docker exec ... touch /test` → fails (read-only root)
  - `test_tmpfs_mount` — `docker exec ... df -T /tmp/office_worker` → shows tmpfs
  - `test_no_docker_socket` — `docker exec ... ls /var/run/docker.sock` → fails
  - `test_no_rust_socket` — `docker exec ... ls /run/armorclaw/sidecar.sock` → fails (socket isolation)
  - `test_no_bridge_socket` — `docker exec ... ls /run/armorclaw/bridge.sock` → fails
  
  **`TestAppArmorProfile`**:
  - `test_profile_loads` — `sudo apparmor_parser -r container/apparmor-profile-office` succeeds
  - `test_profile_enforced` — `sudo aa-status | grep armorclaw-office-worker` shows profile
  - `test_shell_denied` — `docker exec ... /bin/sh -c "echo test"` → fails
  - `test_network_tools_denied` — `docker exec ... /usr/bin/curl ...` → fails
  
  **`TestSecretInjection`**:
  - `test_no_secret_env` — `docker exec ... env | grep SECRET` → no output
  - `test_secret_file_readable` — `docker exec ... test -r /run/secrets/shared_secret` → success
  - `test_secret_file_permissions` — file is read-only (0400)
  
  **`TestTTLRecycling`** (requires `MAX_REQUESTS=5` override):
  - Send 5 requests → container exits
  - `docker compose ps` → shows restart count increased
  - After restart → socket reappears and server works again
  
  **`TestResourceLimits`**:
  - `test_memory_limit` — `docker inspect` shows memory limit 512MB
  - `test_cpu_limit` — `docker inspect` shows CPU limit 1
  - `test_no_new_privileges` — `docker inspect` shows no-new-privileges:true
  
  **`TestCapabilities`**:
  - `test_all_caps_dropped` — `docker inspect` shows cap_drop: ALL
  
  **Must NOT do**:
  - Do NOT test on a production VPS — use dev/test environment only
  - Do NOT leave containers running after tests — always clean up
  - Do NOT modify the Dockerfile or compose file to make tests pass

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Docker lifecycle management, security validation, subprocess coordination
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (exclusive Docker resources)
  - **Parallel Group**: Wave 3 (solo)
  - **Blocks**: FINAL
  - **Blocked By**: T2 (Python server tests pass), T3 (E2E tests pass)

  **References**:

  **Pattern References**:
  - `docker-compose.bridge.yml:227-286` — existing container hardening config to verify against
  - `container/apparmor-profile` — existing AppArmor profile for comparison

  **API/Type References**:
  - `deploy/docker-compose.sidecar-py.yml` — compose file under test
  - `container/apparmor-profile-office` — AppArmor profile under test
  - `sidecar-python/Dockerfile` — Dockerfile under test

  **External References**:
  - Docker SDK for Python: `pip install docker` (optional, can use subprocess)
  - `docker inspect --format` — extract container config
  - `apparmor_parser -r` — load AppArmor profile

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Container starts with full security hardening
    Tool: Bash (pytest)
    Preconditions: Docker available, image built
    Steps:
      1. docker compose -f deploy/docker-compose.sidecar-py.yml up -d
      2. docker inspect armorclaw-sidecar-office — verify all security fields
      3. pytest -v -k "TestContainerLifecycle" test_docker_integration.py
    Expected Result: Container runs with UID 10001, network none, read-only, cap_drop ALL
    Failure Indicators: Container exits, wrong UID, network present, caps not dropped
    Evidence: .sisyphus/evidence/task-5-container-start.txt

  Scenario: Network isolation verified
    Tool: Bash (pytest)
    Preconditions: Container running
    Steps:
      1. docker exec armorclaw-sidecar-office python -c "import socket; s=socket.socket(); s.settimeout(2); s.connect(('8.8.8.8', 80))"
      2. Assert failure (NetworkMode none)
      3. docker inspect — verify NetworkMode == "none"
    Expected Result: All outbound connections fail
    Failure Indicators: Successful connection (CRITICAL SECURITY BUG)
    Evidence: .sisyphus/evidence/task-5-network-isolation.txt

  Scenario: Socket isolation — no cross-sidecar access
    Tool: Bash (pytest)
    Preconditions: Container running
    Steps:
      1. docker exec armorclaw-sidecar-office ls /run/armorclaw/ 2>&1
      2. Assert output contains sidecar-office.sock but NOT sidecar.sock or bridge.sock
    Expected Result: Container sees only its own socket
    Failure Indicators: Rust or Bridge socket visible (CRITICAL ISOLATION BUG)
    Evidence: .sisyphus/evidence/task-5-socket-isolation.txt

  Scenario: TTL recycling — container restarts after MAX_REQUESTS
    Tool: Bash (pytest)
    Preconditions: Container running with MAX_REQUESTS=5
    Steps:
      1. Record restart count: docker inspect --format '{{.RestartCount}}'
      2. Send 5 HealthCheck requests via grpcurl
      3. Sleep 15s
      4. docker compose ps — verify container restarted
      5. Send 6th request → assert success (server recovered)
    Expected Result: Container restarts, socket reappears, server works again
    Failure Indicators: Container stuck, no restart, socket missing after restart
    Evidence: .sisyphus/evidence/task-5-ttl-recycling.txt
  ```

  **Commit**: YES
  - Message: `test(sidecar-python): add Docker container lifecycle and security validation tests`
  - Files: `sidecar-python/test_docker_integration.py`
  - Pre-commit: `cd sidecar-python && python -c "import test_docker_integration"`

---

## Final Verification Wave

- [x] F1. **Full Regression Suite** — `quick`
  Run ALL tests: `go test ./bridge/pkg/sidecar/...`, `pytest sidecar-python/`, verify zero regressions in existing tests.
  Output: `Go [N/N pass] | Python [N/N pass] | Regressions [0] | VERDICT`

- [x] F2. **Must NOT Guardrails Audit** — `deep`
  For each "Must NOT Have" from the implementation plan: write or verify a test that confirms the guardrail holds. Search test files for coverage of each guardrail.
  Output: `Guardrails [N/N verified] | VERDICT`

---

## Commit Strategy

- **T1**: `test(sidecar-python): add test fixtures and conftest for all 6 document formats`
- **T2**: `test(sidecar-python): add worker unit tests for server, format mapping, and streaming`
- **T3**: `test(bridge): add Go-Python E2E integration tests for ExtractText RPC`
- **T4**: `test(sidecar-python): add edge case tests for threshold streaming, TTL, and corrupt files`
- **T5**: `test(sidecar-python): add Docker container lifecycle and security validation tests`
- **F1+F2**: `test(sidecar-python): add regression suite and guardrail audit`

---

## Success Criteria

### Verification Commands
```bash
# Phase 1 (local — no Docker needed)
cd sidecar-python && python -m pytest test_worker.py test_interceptor.py -v
cd bridge && go test -v -run "TestRouteExtractText|TestE2E" ./pkg/sidecar/

# Phase 2 (requires Docker)
docker compose -f deploy/docker-compose.sidecar-py.yml up -d
cd sidecar-python && python -m pytest test_docker_integration.py -v

# Full regression
cd bridge && go test ./pkg/sidecar/...
cd sidecar-python && python -m pytest -v
```

### Final Checklist
- [ ] All 6 formats (.xlsx, .pptx, .msg, .doc, .xls, .ppt) convert successfully
- [ ] Layer 0 (native text) returns text without sidecar invocation
- [ ] Layer 1 (compound validation) routes each format to correct sidecar
- [ ] Layer 2 (strict drop) rejects all magic+format mismatches
- [ ] Threshold streaming boundary tested at <10MB and >10MB
- [ ] TTL recycling verified (server exits after MAX_REQUESTS)
- [ ] Token validation rejects expired/tampered/missing tokens
- [ ] Docker container starts with UID 10001, network_mode none
- [ ] AppArmor profile loaded and enforced
- [ ] Zero regressions in existing test suites
