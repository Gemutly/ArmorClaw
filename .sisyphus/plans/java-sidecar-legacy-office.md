# Java Sidecar Legacy Office — Continuation Plan (T11-T14 + Final Wave)

## TL;DR

> **Quick Summary**: Complete remaining test coverage for the Java gRPC sidecar (DOC/PPT extraction via Apache POI). T1-T10 are committed; this plan covers T11-T14 (Go routing tests, Go E2E, Java JUnit 5, bash harness update) + F1-F4 final verification.
>
> **Deliverables**:
> - Go routing tests verifying Java sidecar path with fallback to Python
> - Go E2E test with Java sidecar subprocess
> - Java JUnit 5 extraction unit tests (ExtractorServiceTest.java)
> - Updated bash harness with Java sidecar scenarios
> - Pre-built test fixtures (sample.doc, sample.ppt)
>
> **Estimated Effort**: Medium
> **Parallel Execution**: YES — 2 waves
> **Critical Path**: T13 (fixtures) → T11/T12/T14 (parallel) → F1-F4

---

## Context

### Original Request
Create a Java 21 gRPC sidecar for legacy Office format text extraction (.doc, .ppt) using Apache POI, integrated with the existing Go Bridge 3-layer routing pipeline.

### Previous Progress (T1-T10 — ALL COMPLETE)
All implementation tasks committed:
- Maven skeleton + proto generation (`764d6d8`)
- gRPC server + HealthCheck + Unix socket (`c1349fe`)
- HMAC-SHA256 token auth + version interceptor (`fd81792`)
- Apache POI .doc extraction via HWPFDocument/WordExtractor (`37e5a64`)
- Apache POI .ppt extraction via QuickButCruddyTextExtractor (`c27e6ef`)
- Go client factory `java_client.go` (`1d6339d`)
- Bridge routing: DOC/PPT → Java (with Python fallback) (`8207230`)
- AppArmor profile (`18cc086`)
- TTL recycling + stale socket cleanup (`e5f0fe7`)
- Docker image + docker-compose (`3cee050`)

### Metis Review
**Identified Gaps** (all addressed):
- Mockito missing from pom.xml → T13 prerequisite
- No `createCorruptPpt()` fixture → T13 adds it
- Existing DOC/PPT tests don't assert positive routing → T11 fixes
- JVM cold start slower than Python → T12 uses 20s timeout
- No OLE2 generation in bash → T14 uses pre-built fixtures

---

## Work Objectives

### Core Objective
Complete test coverage for the Java sidecar: Go routing tests, Go E2E tests, Java unit tests, and bash harness integration.

### Concrete Deliverables
- `bridge/pkg/sidecar/office_client_test.go` — extended with Java routing tests
- `bridge/pkg/sidecar/java_sidecar_e2e_test.go` — new Go E2E test file
- `sidecar-java/src/test/java/com/armorclaw/sidecar/ExtractorServiceTest.java` — new JUnit 5 test
- `sidecar-java/src/test/java/com/armorclaw/sidecar/TestFixtures.java` — extended with corruptPpt
- `sidecar-java/pom.xml` — add Mockito dependencies
- `tests/fixtures/sample.doc` — pre-built DOC fixture
- `tests/fixtures/sample.ppt` — pre-built PPT fixture
- `tests/test-sidecar-docs.sh` — updated with Java sidecar scenarios

### Definition of Done
- [ ] `cd bridge && go test -v -run "TestRouteExtractText_Java" ./pkg/sidecar/...` → PASS
- [ ] `cd bridge && go test -v -run "TestE2E_Java" ./pkg/sidecar/...` → PASS or skip (no Java)
- [ ] `cd sidecar-java && mvn test` → BUILD SUCCESS
- [ ] `bash -n tests/test-sidecar-docs.sh` → exit 0

### Must Have
- All existing tests continue to pass (zero regressions)
- Java routing tests verify BOTH Java path AND Python fallback
- E2E test skips gracefully when Java not available
- JUnit 5 tests cover happy path + error/edge cases for DOC and PPT
- Fixtures are small (<10KB) binary files

### Must NOT Have (Guardrails)
- Do NOT modify `RouteExtractText` routing logic in `office_client.go`
- Do NOT add `.xls` routing to Java — it always goes to Python by design
- Do NOT attempt inline OLE2 binary generation in bash
- Do NOT add test infrastructure to production Java code
- Do NOT change the proto definition
- Do NOT modify the Dockerfile or docker-compose
- Do NOT add Mockito to Go tests — Go uses mockServer struct embedding

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: YES (Go tests + JUnit 5 + bash harness)
- **Automated tests**: Tests-after (these ARE the tests)
- **Framework**: Go testing + JUnit 5 + bash

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — fixture + dependency foundation):
└── Task 13: Java test infrastructure + JUnit 5 extraction tests [deep]
    (produces fixtures that T12 and T14 reference)

Wave 2 (After Wave 1 — core tests, MAX PARALLEL):
├── Task 11: Go routing tests for Java sidecar path (depends: 13) [unspecified-high]
├── Task 12: Go E2E test with Java subprocess (depends: 13) [deep]
└── Task 14: Bash harness update (depends: 13) [unspecified-high]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Real manual QA (unspecified-high)
└── F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay

Critical Path: T13 → T11/T12/T14 (parallel) → F1-F4 → user okay
Parallel Speedup: ~50% faster than sequential
Max Concurrent: 3 (Wave 2)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 13   | —         | 11, 12, 14 | 1 |
| 11   | 13        | F1-F4 | 2 |
| 12   | 13        | F1-F4 | 2 |
| 14   | 13        | F1-F4 | 2 |

### Agent Dispatch Summary

- **Wave 1**: 1 — T13 → `deep`
- **Wave 2**: 3 — T11 → `unspecified-high`, T12 → `deep`, T14 → `unspecified-high`
- **FINAL**: 4 — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 13. Java Test Infrastructure + JUnit 5 Extraction Tests

  **What to do**:
  - Add `mockito-core` (5.x) and `mockito-junit-jupiter` to `sidecar-java/pom.xml` under test dependencies
  - Add `createCorruptPpt()` method to `TestFixtures.java` — return `byte[]` of random non-OLE2 bytes (matching `createCorruptDoc()` pattern)
  - Create `ExtractorServiceTest.java` with JUnit 5 test methods:
    - `extractDocText_minimalDoc_returnsExpectedText` — use `TestFixtures.createMinimalDoc()`, call `extractText` with format `"application/msword"`, assert response text contains "Test document content"
    - `extractDocText_emptyDoc_returnsEmptyString` — use `TestFixtures.createEmptyDoc()`, assert response text is empty or blank
    - `extractDocText_corruptDoc_returnsInvalidArgument` — use `TestFixtures.createCorruptDoc()`, assert `INVALID_ARGUMENT` error
    - `extractPptText_minimalPpt_returnsExpectedText` — use `TestFixtures.createMinimalPpt()`, call with format `"application/vnd.ms-powerpoint"`, assert response text contains "Test slide content"
    - `extractPptText_emptyPpt_returnsEmptyString` — use `TestFixtures.createEmptyPpt()`, assert response text is empty or blank
    - `extractPptText_corruptPpt_returnsInvalidArgument` — use `TestFixtures.createCorruptPpt()`, assert error
    - `extractText_unsupportedFormat_returnsInvalidArgument` — send format `"application/pdf"`, assert error
    - `healthCheck_returnsServingStatus` — call `healthCheck`, assert status=="SERVING" and version=="1.0.0"
  - Generate pre-built test fixtures:
    - Write a small helper script or Java main that uses TestFixtures to dump `sample.doc` and `sample.ppt` to `tests/fixtures/`
    - Commit the binary fixture files (<10KB each)

  **Must NOT do**:
  - Do NOT add test-mode endpoints to production ExtractorService
  - Do NOT modify ExtractorService.java, ServerMain.java, TokenInterceptor.java, VersionInterceptor.java
  - Do NOT add `.xls` test cases (Java sidecar doesn't handle XLS)
  - Do NOT use PowerMock or any framework other than Mockito

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Requires understanding the ExtractorService internal dispatch logic and POI error handling
  - **Skills**: `[]`
  - **Skills Evaluated but Omitted**:
    - None relevant — pure Java testing task

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1 (solo)
  - **Blocks**: Tasks 11, 12, 14
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References** (existing code to follow):
  - `sidecar-java/src/main/java/com/armorclaw/sidecar/ExtractorService.java:96-119` — `extractText()` dispatch logic: format contains "msword" → `extractDocText`, "ms-powerpoint" → `extractPptText`, else INVALID_ARGUMENT
  - `sidecar-java/src/main/java/com/armorclaw/sidecar/ExtractorService.java:121-158` — `extractDocText()` private method: HWPFDocument + WordExtractor, catches EncryptedDocumentException, ArrayIndexOutOfBoundsException, IOException
  - `sidecar-java/src/main/java/com/armorclaw/sidecar/ExtractorService.java:160-196` — `extractPptText()` private method: QuickButCruddyTextExtractor, same error pattern
  - `sidecar-java/src/test/java/com/armorclaw/sidecar/TestFixtures.java:17-47` — Existing fixture methods: createMinimalDoc (HWPFDocument insert), createEmptyDoc, createCorruptDoc
  - `sidecar-java/src/test/java/com/armorclaw/sidecar/TestFixtures.java:53-71` — PPT fixture methods: createMinimalPpt (HSLFSlideShow + HSLFTextBox), createEmptyPpt

  **API/Type References** (contracts to implement against):
  - `sidecar-java/src/main/proto/sidecar.proto:112-124` — ExtractTextRequest/Response message shapes
  - `sidecar-java/src/main/java/com/armorclaw/sidecar/ExtractorService.java:31-36` — Constructor signature: `(AtomicInteger, int, Runnable)`

  **External References**:
  - Mockito 5 + JUnit 5 integration: `@ExtendWith(MockitoExtension.class)` + `@Mock` StreamObserver

  **Acceptance Criteria**:

  - [ ] `grep -c "mockito" sidecar-java/pom.xml` → ≥ 2 (mockito-core + mockito-junit-jupiter)
  - [ ] `grep -c "createCorruptPpt" sidecar-java/src/test/java/com/armorclaw/sidecar/TestFixtures.java` → ≥ 1
  - [ ] `cd sidecar-java && mvn test` → BUILD SUCCESS with ≥ 8 test methods in ExtractorServiceTest
  - [ ] `ls tests/fixtures/sample.doc tests/fixtures/sample.ppt` → both exist, non-zero size

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: JUnit 5 extraction tests pass for DOC happy path
    Tool: Bash (mvn test)
    Preconditions: sidecar-java/pom.xml has Mockito, ExtractorServiceTest.java exists
    Steps:
      1. cd sidecar-java && mvn test -Dtest=ExtractorServiceTest#extractDocText_minimalDoc_returnsExpectedText
      2. Assert BUILD SUCCESS in output
      3. Assert test method ran and passed
    Expected Result: BUILD SUCCESS, test passes
    Failure Indicators: BUILD FAILURE, test failures, compilation errors
    Evidence: .sisyphus/evidence/task-13-doc-happy.txt

  Scenario: JUnit 5 handles corrupt DOC gracefully
    Tool: Bash (mvn test)
    Preconditions: ExtractorServiceTest with corruptDoc test
    Steps:
      1. cd sidecar-java && mvn test -Dtest=ExtractorServiceTest#extractDocText_corruptDoc_returnsInvalidArgument
      2. Assert test passes with error verification
    Expected Result: Test confirms INVALID_ARGUMENT error for corrupt input
    Failure Indicators: Test expects different behavior, assertion failures
    Evidence: .sisyphus/evidence/task-13-doc-corrupt.txt

  Scenario: Test fixtures are valid binary files
    Tool: Bash
    Preconditions: tests/fixtures/sample.doc and sample.ppt generated
    Steps:
      1. file tests/fixtures/sample.doc → assert contains "Microsoft" or "Composite Document File"
      2. file tests/fixtures/sample.ppt → assert contains "Microsoft" or "Composite Document File"
      3. wc -c tests/fixtures/sample.doc tests/fixtures/sample.ppt → assert each < 10240 bytes
    Expected Result: Both files are valid OLE2 compound documents, each <10KB
    Failure Indicators: Wrong file type, zero-byte files, excessive size
    Evidence: .sisyphus/evidence/task-13-fixtures-validation.txt
  ```

  **Commit**: YES
  - Message: `test(sidecar-java): add JUnit 5 extraction tests with Mockito`
  - Files: `sidecar-java/pom.xml`, `sidecar-java/src/test/java/com/armorclaw/sidecar/ExtractorServiceTest.java`, `sidecar-java/src/test/java/com/armorclaw/sidecar/TestFixtures.java`, `tests/fixtures/sample.doc`, `tests/fixtures/sample.ppt`
  - Pre-commit: `cd sidecar-java && mvn test`

- [x] 11. Go Routing Tests for Java Sidecar Path

  **What to do**:
  - Extend `setupRoutingClients` in `office_client_test.go` OR create new helper `setupRoutingClientsWithJava` that returns THREE mock pairs: (office, officeMock), (rust, rustMock), (java, javaMock)
  - Fix existing positive assertion gaps in `TestRouteExtractText_DOC_RoutesToPython` (line 148) and `TestRouteExtractText_PPT_RoutesToPython` (line 176): add `officeMock.extractCalled == true` assertion
  - Add new test cases:
    - `TestRouteExtractText_DOC_RoutesToJava`: OLE magic + "application/msword" + javaClient present → assert `javaMock.extractCalled == true`, `officeMock.extractCalled == false`
    - `TestRouteExtractText_PPT_RoutesToJava`: OLE magic + "application/vnd.ms-powerpoint" + javaClient present → assert `javaMock.extractCalled == true`, `officeMock.extractCalled == false`
    - `TestRouteExtractText_DOC_FallbackToPython`: OLE magic + "application/msword" + javaClient == nil → assert `officeMock.extractCalled == true` (existing behavior, now explicitly verified)
    - `TestRouteExtractText_PPT_FallbackToPython`: OLE magic + "application/vnd.ms-powerpoint" + javaClient == nil → assert `officeMock.extractCalled == true`

  **Must NOT do**:
  - Do NOT modify `RouteExtractText` in `office_client.go`
  - Do NOT change routing logic
  - Do NOT add `.xls` Java routing tests (XLS always goes to Python)
  - Do NOT add Mockito to Go tests — Go uses mockServer struct embedding `UnimplementedSidecarServiceServer`

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Requires understanding Go gRPC mock server pattern and routing table
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 12, 14)
  - **Blocks**: F1-F4
  - **Blocked By**: Task 13 (needs fixture files for context)

  **References**:

  **Pattern References** (existing code to follow):
  - `bridge/pkg/sidecar/office_client_test.go:80-104` — `setupRoutingClients` helper: creates two mock servers, returns clients + mock references. Mirror this pattern for three-server setup
  - `bridge/pkg/sidecar/office_client_test.go:148-160` — `TestRouteExtractText_DOC_RoutesToPython`: OLE magic + "application/msword" format, asserts `rustMock.extractCalled == false` (missing `officeMock.extractCalled == true` — MUST FIX)
  - `bridge/pkg/sidecar/office_client_test.go:176-188` — `TestRouteExtractText_PPT_RoutesToPython`: Same pattern for PPT, same missing assertion

  **API/Type References**:
  - `bridge/pkg/sidecar/office_client.go:97-111` — Java routing: lines 97-101 show DOC routing (`if javaClient != nil { return javaClient.ExtractText }` with `officeClient` fallback), lines 106-111 show PPT routing (same pattern)
  - `bridge/pkg/sidecar/office_client.go:38-44` — `RouteExtractText` function signature: `(ctx, req, officeClient, rustClient, javaClient)`

  **Acceptance Criteria**:

  - [ ] `cd bridge && go test -v -run "TestRouteExtractText_Java" ./pkg/sidecar/...` → PASS (4 new tests)
  - [ ] `cd bridge && go test -v -run "TestRouteExtractText" ./pkg/sidecar/...` → ALL PASS (existing 18 + 4 new)
  - [ ] `grep -c "officeMock.extractCalled" bridge/pkg/sidecar/office_client_test.go` → ≥ 4 (the 2 new positive assertions + 2 fixed existing)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Java DOC routing test verifies Java mock is called
    Tool: Bash (go test)
    Preconditions: New setupRoutingClientsWithJava helper, javaMock tracks extractCalled
    Steps:
      1. cd bridge && go test -v -run "TestRouteExtractText_DOC_RoutesToJava" ./pkg/sidecar/...
      2. Assert PASS in output
      3. Verify javaMock.extractCalled == true
    Expected Result: PASS — DOC routes to Java when javaClient != nil
    Failure Indicators: Test fails, javaMock not called, wrong sidecar called
    Evidence: .sisyphus/evidence/task-11-java-doc-routing.txt

  Scenario: Java PPT routing test verifies Java mock is called
    Tool: Bash (go test)
    Preconditions: Same helper as DOC test
    Steps:
      1. cd bridge && go test -v -run "TestRouteExtractText_PPT_RoutesToJava" ./pkg/sidecar/...
      2. Assert PASS
    Expected Result: PASS — PPT routes to Java when javaClient != nil
    Failure Indicators: Test fails, officeClient called instead of javaClient
    Evidence: .sisyphus/evidence/task-11-java-ppt-routing.txt

  Scenario: Existing tests unchanged — regression gate
    Tool: Bash (go test)
    Preconditions: All existing tests still pass
    Steps:
      1. cd bridge && go test -v -run "TestRouteExtractText" ./pkg/sidecar/... 2>&1
      2. Count PASS lines — must be ≥ 22 (18 existing + 4 new)
    Expected Result: All tests PASS, zero failures
    Failure Indicators: Any FAIL, regression in existing tests
    Evidence: .sisyphus/evidence/task-11-regression-gate.txt
  ```

  **Commit**: YES
  - Message: `test(bridge): add Go routing tests for Java sidecar path`
  - Files: `bridge/pkg/sidecar/office_client_test.go`
  - Pre-commit: `cd bridge && go test -v -run "TestRouteExtractText" ./pkg/sidecar/...`

- [x] 12. Go E2E Test with Java Sidecar Subprocess

  **What to do**:
  - Create new file `bridge/pkg/sidecar/java_sidecar_e2e_test.go`
  - Mirror the Python E2E pattern from `office_client_e2e_test.go`:
    - `findJava(t)`: check `java` binary on PATH (version 21+), skip if missing
    - `startJavaSidecar(t)`: start `sidecar.jar` as subprocess with env vars `SIDECAR_JAVA_SOCKET_PATH`, `SIDECAR_JAVA_MAX_REQUESTS=1000`, `SIDECAR_SHARED_SECRET_PATH` pointing to empty file (dev mode)
    - Socket poll with **20-second deadline** (JVM cold start), 200ms interval
    - `defer javaSrv.stop()` with Kill + Wait
  - Add E2E test cases:
    - `TestE2E_Java_HealthCheck`: call HealthCheck, assert status=="SERVING", version=="1.0.0"
    - `TestE2E_Java_DocExtraction`: read `tests/fixtures/sample.doc`, call ExtractText with format "application/msword", assert response text contains expected content
    - `TestE2E_Java_PptExtraction`: read `tests/fixtures/sample.ppt`, call ExtractText with format "application/vnd.ms-powerpoint", assert response text contains expected content
    - `TestE2E_Java_UnsupportedFormat`: call ExtractText with "application/pdf", assert INVALID_ARGUMENT error
  - All tests must `t.Skip()` when Java 21 or `sidecar.jar` not available
  - Look for `sidecar.jar` at `../sidecar-java/target/sidecar.jar` (relative to bridge/)

  **Must NOT do**:
  - Do NOT attempt `mvn package` — rely on pre-built JAR, skip if missing
  - Do NOT add network dependencies — Unix socket only
  - Do NOT test TTL drain (integration-level, too slow for E2E)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Requires coordinating Java subprocess lifecycle, socket polling, and gRPC client setup
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 11, 14)
  - **Blocks**: F1-F4
  - **Blocked By**: Task 13 (needs fixture files)

  **References**:

  **Pattern References**:
  - `bridge/pkg/sidecar/office_client_e2e_test.go:16-23` — `findPython3(t)`: skip pattern for missing binary
  - `bridge/pkg/sidecar/office_client_e2e_test.go:124-186` — `startPythonSidecar(t)`: subprocess start, env setup, socket poll with 10s deadline, defer cleanup. Mirror this for Java with 20s deadline
  - `bridge/pkg/sidecar/office_client_e2e_test.go:195-204` — `makeClient(t)`: NewClient with socket path, 15s timeout, 100MB max message size

  **API/Type References**:
  - `bridge/pkg/sidecar/java_client.go:11-16` — Java socket path and max message size constants
  - `sidecar-java/src/main/java/com/armorclaw/sidecar/ServerMain.java:31-34` — Env var `SIDECAR_JAVA_SOCKET_PATH`
  - `sidecar-java/src/main/java/com/armorclaw/sidecar/ServerMain.java:36-37` — Env var `SIDECAR_JAVA_MAX_REQUESTS`

  **Acceptance Criteria**:

  - [ ] File exists: `bridge/pkg/sidecar/java_sidecar_e2e_test.go`
  - [ ] `cd bridge && go test -v -run "TestE2E_Java" ./pkg/sidecar/...` → PASS or skip gracefully
  - [ ] When Java + JAR available: all 4 E2E tests pass
  - [ ] When Java missing: output contains "skipping" or "Skip"

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Java E2E skips when Java not available
    Tool: Bash (go test)
    Preconditions: Java 21 NOT on PATH or sidecar.jar not built
    Steps:
      1. cd bridge && go test -v -run "TestE2E_Java" ./pkg/sidecar/... 2>&1
      2. Assert output contains "Skip" or "skipping"
    Expected Result: Tests skip gracefully, no failures
    Failure Indicators: Test fails instead of skipping, panic, compilation error
    Evidence: .sisyphus/evidence/task-12-skip-no-java.txt

  Scenario: Java E2E DOC extraction passes when Java available
    Tool: Bash (go test)
    Preconditions: Java 21 on PATH, sidecar-java/target/sidecar.jar exists, tests/fixtures/sample.doc exists
    Steps:
      1. cd sidecar-java && mvn package -DskipTests
      2. cd bridge && go test -v -run "TestE2E_Java_DocExtraction" ./pkg/sidecar/... 2>&1
      3. Assert PASS, response contains expected text
    Expected Result: PASS — DOC text extracted from real Java sidecar subprocess
    Failure Indicators: Connection refused, extraction returned empty, timeout
    Evidence: .sisyphus/evidence/task-12-doc-e2e.txt

  Scenario: Java E2E health check reports SERVING
    Tool: Bash (go test)
    Preconditions: Java 21 + sidecar.jar available
    Steps:
      1. cd bridge && go test -v -run "TestE2E_Java_HealthCheck" ./pkg/sidecar/... 2>&1
      2. Assert PASS, status=="SERVING", version=="1.0.0"
    Expected Result: PASS with correct health response
    Failure Indicators: Wrong status, wrong version, connection failure
    Evidence: .sisyphus/evidence/task-12-health.txt
  ```

  **Commit**: YES
  - Message: `test(bridge): add Go E2E test for Java sidecar subprocess`
  - Files: `bridge/pkg/sidecar/java_sidecar_e2e_test.go`
  - Pre-commit: `cd bridge && go vet ./pkg/sidecar/... && go build ./pkg/sidecar/...`

- [x] 14. Bash Harness Update for Java Sidecar

  **What to do**:
  - Add Java sidecar socket path to `tests/test-sidecar-docs.sh`:
    - `JAVA_SOCK="/run/armorclaw/sidecar-java/sidecar-java.sock"`
    - Check socket existence via `ssh_vps "test -S '$JAVA_SOCK'"`
  - Add Java socket check in D0 (prerequisites) section:
    - `JAVA_SOCK_EXISTS=true/false` flag
    - Update the "skip entire script" logic: if ALL THREE (rust, python, java) are absent, skip
  - Add D2.5 scenario: Java sidecar health check
    - Use `grpcurl` on `JAVA_SOCK` to call HealthCheck
    - Assert status=="SERVING", log version
  - Add D5.5 scenario: DOC extraction via Java sidecar
    - Read `tests/fixtures/sample.doc` from host, base64-encode, send via `rpc_doc`
    - Assert extracted text is non-empty
  - Add D5.6 scenario: PPT extraction via Java sidecar
    - Read `tests/fixtures/sample.ppt`, same pattern
  - Add evidence files for new scenarios

  **Must NOT do**:
  - Do NOT attempt inline OLE2 binary generation — use pre-built fixtures from Task 13
  - Do NOT modify existing D1-D7 scenario logic (only ADD new scenarios)
  - Do NOT add Java socket path to `grpcurl_rust` or `grpcurl_python` helpers — create `grpcurl_java` if needed
  - Do NOT change the skip behavior for existing scenarios

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Requires understanding existing bash harness patterns and SSH-based gRPC testing
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 11, 12)
  - **Blocks**: F1-F4
  - **Blocked By**: Task 13 (needs fixture files)

  **References**:

  **Pattern References**:
  - `tests/test-sidecar-docs.sh:36-38` — Existing socket path definitions: `RUST_SOCK` and `PYTHON_SOCK`. Add `JAVA_SOCK` alongside
  - `tests/test-sidecar-docs.sh:42-60` — `grpcurl_rust()` and `grpcurl_python()` helper patterns. Mirror for Java
  - `tests/test-sidecar-docs.sh:87-128` — D0 prerequisites section: jq check, grpcurl check, socket existence checks. Add Java socket check here
  - `tests/test-sidecar-docs.sh:134-161` — D1 Rust health check pattern. Mirror for D2.5 Java health
  - `tests/test-sidecar-docs.sh:286-321` — D5 Python office extraction pattern. Mirror for D5.5/D5.6 Java DOC/PPT extraction

  **API/Type References**:
  - `tests/test-sidecar-docs.sh:62-77` — `rpc_doc()` helper: bridge RPC via HTTPS or Unix socket SSH fallback
  - `deploy/docker-compose.sidecar-java.yml:39` — Java socket path env: `SIDECAR_JAVA_SOCKET_PATH=/run/armorclaw/sidecar-java.sock`

  **Acceptance Criteria**:

  - [ ] `grep -c "sidecar-java" tests/test-sidecar-docs.sh` → ≥ 3 (socket def, health scenario, extraction scenario)
  - [ ] `bash -n tests/test-sidecar-docs.sh` → exit 0
  - [ ] `grep -c "D2.5\|D5.5\|D5.6" tests/test-sidecar-docs.sh` → ≥ 3

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Bash harness syntax is valid
    Tool: Bash
    Preconditions: File exists
    Steps:
      1. bash -n tests/test-sidecar-docs.sh
      2. Assert exit code 0
    Expected Result: No syntax errors
    Failure Indicators: Non-zero exit, parse error
    Evidence: .sisyphus/evidence/task-14-syntax.txt

  Scenario: Java socket path and scenarios present
    Tool: Bash (grep)
    Preconditions: File updated
    Steps:
      1. grep "JAVA_SOCK" tests/test-sidecar-docs.sh | wc -l → assert ≥ 2
      2. grep "grpcurl_java\|sidecar-java" tests/test-sidecar-docs.sh | wc -l → assert ≥ 3
    Expected Result: Java sidecar integrated into harness
    Failure Indicators: Missing socket path, no Java scenarios
    Evidence: .sisyphus/evidence/task-14-java-present.txt

  Scenario: Existing scenarios unchanged — regression
    Tool: Bash (grep)
    Preconditions: D0-D7 scenarios still present
    Steps:
      1. grep -c "^log_info.*D[0-7]:" tests/test-sidecar-docs.sh → assert ≥ 7
      2. grep "RUST_SOCK=" tests/test-sidecar-docs.sh → assert unchanged path
      3. grep "PYTHON_SOCK=" tests/test-sidecar-docs.sh → assert unchanged path
    Expected Result: All existing scenarios intact
    Failure Indicators: Missing scenarios, changed socket paths
    Evidence: .sisyphus/evidence/task-14-regression.txt
  ```

  **Commit**: YES
  - Message: `test(harness): add Java sidecar scenarios to bash test harness`
  - Files: `tests/test-sidecar-docs.sh`
  - Pre-commit: `bash -n tests/test-sidecar-docs.sh`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./pkg/sidecar/...` + `mvn compile` + `mvn test`. Review all changed files for: `as any`/`@ts-ignore` equivalents (raw types, suppressed warnings), empty catches, System.out in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Go Vet [PASS/FAIL] | Java Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (Go routing + Java extraction working together). Test edge cases: empty DOC, corrupt PPT, unsupported format. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination: Task N touching Task M's files. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **T13**: `test(sidecar-java): add JUnit 5 extraction tests with Mockito` — ExtractorServiceTest.java, TestFixtures.java, pom.xml, tests/fixtures/*
- **T11**: `test(bridge): add Go routing tests for Java sidecar path` — office_client_test.go
- **T12**: `test(bridge): add Go E2E test for Java sidecar subprocess` — java_sidecar_e2e_test.go
- **T14**: `test(harness): add Java sidecar scenarios to bash test harness` — tests/test-sidecar-docs.sh

---

## Success Criteria

### Verification Commands
```bash
# Go routing tests (all existing + new Java path)
cd bridge && go test -v -run "TestRouteExtractText" ./pkg/sidecar/...
# Expected: ALL PASS (18+ existing + new Java tests)

# Go E2E Java test (skip or pass depending on Java availability)
cd bridge && go test -v -run "TestE2E_Java" ./pkg/sidecar/...
# Expected: PASS or skip

# Java unit tests
cd sidecar-java && mvn test
# Expected: BUILD SUCCESS

# Bash harness syntax
bash -n tests/test-sidecar-docs.sh
# Expected: exit 0
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All Go tests pass (zero regressions)
- [ ] All Java tests pass
- [ ] Bash harness valid
- [ ] Test fixtures committed
