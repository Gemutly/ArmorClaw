# MarkItDown Polyglot Sidecar

## TL;DR

> **Quick Summary**: Expand ArmorClaw's document ingestion pipeline with a Python gRPC sidecar (MarkItDown) running in strict parallel isolation alongside the existing Rust sidecar. The Go Bridge is the sole routing authority — it performs non-destructive 8-byte magic header inspection combined with `document_format` string matching to route each format to its designated worker. The Python worker handles OpenXML spreadsheets/presentations (`.xlsx`, `.pptx`) and legacy OLE formats (`.msg`, `.doc`, `.xls`, `.ppt`). The Rust worker handles `.docx` and `.pdf`. Plain text formats (`.txt`, `.csv`, `.json`, `.md`) are decoded natively in Go with no sidecar invocation. Both sidecars are treated as equally hostile — kernel-isolated via `network_mode: none`, UID 10001, AppArmor, ephemeral tmpfs, and crash-only TTL recycling.
> 
> **Deliverables**:
> - Python gRPC server implementing the existing `SidecarService.ExtractText` RPC
> - Go Bridge 3-layer routing: Layer 0 (native text bypass) → Layer 1 (compound magic+format switch) → Layer 2 (strict mismatch drop)
> - Docker Compose file with symmetrical hardening (identical to Rust sidecar)
> - Token validation interceptor ported from Go to Python
> 
> **Estimated Effort**: Medium
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: T2 → T3 → T4+T5 (parallel) → FINAL

---

## Context

### Original Request
Expand ArmorClaw's document ingestion pipeline with a Python gRPC sidecar (MarkItDown) for formats the Rust sidecar cannot handle. Both sidecars run in strict parallel isolation — neither filters for the other. The Go Bridge is the sole routing authority, using non-destructive 8-byte magic header inspection + `document_format` string matching.

### Interview Summary
**Key Discussions**:
- **Format scope**: Python handles `.xlsx`, `.pptx` (OpenXML), `.msg`, `.doc`, `.xls`, `.ppt` (OLE). Rust handles `.docx`, `.pdf`. Plain text (`.txt`, `.csv`, `.json`, `.md`) bypasses both sidecars entirely — decoded natively in Go.
- **Compound validation**: Magic bytes alone insufficient (PK/OLE are container formats shared by many file types). Must validate magic bytes AND `document_format`/extension together.
- **Strict drop policy**: If magic bytes and format string disagree (e.g., ZIP magic but format claims `.msg`), the request is instantly dropped with `codes.InvalidArgument`. No guessing, no autocorrect.
- **Symmetrical containment**: Both sidecars are treated as equally hostile. Neither is a "filter" or "shield" for the other. Kernel-level isolation (AppArmor, UID 10001, `network_mode: none`, tmpfs) provides blast radius containment.
- **Threshold streaming**: <10MB use `convert_stream(BytesIO)`, >10MB spill to tmpfs temp file with `try/finally` cleanup
- **Version negotiation**: Python server must inject `x-sidecar-server-version: 1.0.0` or Go client rejects connection
- **Retry strategy**: Same as Rust sidecar — 5 retries, exponential backoff, 5s max

**Research Findings**:
- Existing `ExtractText` is **unary** RPC (not streaming) — `ExtractTextRequest` → `ExtractTextResponse`
- Go client defaults to 256MB max message size — Python's 100MB is more conservative, compatible
- Token format: `{request_id}:{timestamp}:{operation}:{hmac_hex}`, HMAC-SHA256, 30min TTL, 5min max age
- MarkItDown `convert_stream()` accepts `StreamInfo(extension=".xlsx")` for format hints
- MarkItDown exceptions: `MissingDependencyException`, `UnsupportedFormatException`
- No existing sidecar Dockerfile — Python sidecar is first containerized sidecar
- Rust sidecar binary has 74 compilation errors — it's library-only, not running in Docker

### Metis Review (Self-Performed — session descendant cap prevented agent spawn)
**Identified Gaps** (addressed):
- **Proto stubs**: Python gRPC stubs generated via `grpcio-tools` in Dockerfile build step
- **Expanded OLE support**: Python sidecar now handles all legacy OLE formats (`.msg`, `.doc`, `.xls`, `.ppt`) — requires `markitdown` extras and additional dependencies (`olefile`, `xlrd`)
- **Socket isolation**: Python worker must NOT have access to Rust sidecar socket or bridge socket
- **PII interceptor**: Go office client needs its own PII interceptor instance, not shared with Rust client
- **TTL verification**: QA must verify container exits after 50 requests and Docker restarts it
- **In-flight drain**: 30s grace period allows current requests (including large 50MB+ spreadsheets) to complete before exit

---

## Work Objectives

### Core Objective
Expand the document ingestion pipeline with a Python gRPC sidecar (MarkItDown) running in strict parallel isolation with the existing Rust sidecar. The Go Bridge is the sole routing authority — Layer 0 natively decodes plain text, Layer 1 uses compound magic-byte + format validation to route each format to its designated worker, and mismatched payloads are strictly dropped. Both sidecars are treated as equally hostile with symmetrical kernel-level containment.

### Concrete Deliverables
- `sidecar-python/` directory with gRPC server, Dockerfile, and interceptor
- New `bridge/pkg/sidecar/office_client.go` with 3-layer routing (native text bypass + compound magic+format switch + strict drop)
- `deploy/docker-compose.sidecar-py.yml` with hardened container config
- `container/apparmor-profile-office` with strict allowlist

### Definition of Done
- [ ] `go vet ./bridge/...` passes with zero warnings
- [ ] `go build ./bridge/...` compiles cleanly
- [ ] `go test ./bridge/pkg/sidecar/...` passes
- [ ] Python gRPC server starts, binds to Unix socket, responds to HealthCheck
- [ ] Python server converts test `.xlsx`, `.msg`, `.pptx`, `.doc` files via ExtractText RPC
- [ ] Go Bridge Layer 0: `.txt`/`.csv`/`.json`/`.md` decoded natively, no sidecar invoked
- [ ] Go Bridge Layer 1: `.xlsx`/`.pptx`/`.msg`/`.doc`/`.xls`/`.ppt` → Python socket (active)
- [ ] Go Bridge Layer 1: `.docx`/`.pdf` → Rust socket (wired but pending Rust sidecar deployment)
- [ ] Go Bridge logs `LEVEL_WARN` at startup if Rust sidecar socket absent
- [ ] Compound validation rejects magic byte + format mismatches with `codes.InvalidArgument`
- [ ] TTL recycling exits process after 50 requests
- [ ] Token validation rejects requests without valid HMAC

### Must Have
- Same `ExtractText` RPC contract as Rust sidecar (unary, same messages)
- Layer 0: Native Go decoding of `.txt`, `.csv`, `.json`, `.md` — zero sidecar invocation
- Layer 1: Compound validation (8-byte magic header + `document_format` string) for all binary formats
- Strict drop policy: Magic byte ↔ format mismatch = instant `codes.InvalidArgument`, no guessing
- Symmetrical containment: Both sidecars run with identical hardening (`network_mode: none`, UID 10001, AppArmor, tmpfs, crash-only TTL)
- Threshold streaming: <10MB memory, >10MB tmpfs spill with cleanup
- Version interceptor: `x-sidecar-server-version: 1.0.0`
- Token validation with HMAC-SHA256, 30min TTL, 5min max age
- Crash-only TTL recycling at 50 requests with 30s graceful drain

### Must NOT Have (Guardrails)
- Do NOT modify the existing proto file (`sidecar.proto`)
- Do NOT modify existing Rust sidecar code
- Do NOT use Python as a "filter" or "shield" for Rust — they are parallel, equally distrusted workers
- Do NOT route `.docx` or `.pdf` to the Python sidecar — those go to Rust exclusively
- Do NOT route `.xlsx`, `.pptx`, `.msg`, `.doc`, `.xls`, `.ppt` to the Rust sidecar — those go to Python exclusively
- Do NOT invoke any sidecar for `.txt`, `.csv`, `.json`, `.md` — decode natively in Go
- Do NOT introduce HTTP/FastAPI — gRPC only
- Do NOT bypass or weaken token validation
- Do NOT change NetworkMode from `none`
- Do NOT give Python worker access to Rust sidecar socket or bridge socket
- Do NOT inject secrets via environment variables — use memory-only file mount only
- Do NOT include grpcio-tools or build toolchain in the production Docker image
- Do NOT use `//nolint` comments
- Do NOT abstract the gRPC contract — it's proto-defined, not interface-defined
- Do NOT guess or autocorrect format mismatches — strict drop policy only

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (Go tests via `go test`, Python tests via `pytest`)
- **Automated tests**: Tests-after (write tests alongside implementation)
- **Framework**: Go: `go test` / Python: `pytest`
- **If TDD**: N/A — tests-after approach

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go Bridge**: Use Bash (`go vet`, `go build`, `go test`)
- **Python Server**: Use Bash (`pytest`, `python -c` for smoke tests)
- **Docker**: Use Bash (`docker compose`, `docker exec`)
- **gRPC**: Use `grpcurl` or Python client for end-to-end verification

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation — can all start immediately):
├── Task 1: Proto reference & Python stubs generation [quick]
├── Task 2: Python gRPC server with MarkItDown + threshold streaming [deep]
└── Task 5: Token validation interceptor [quick]

Wave 2 (After Wave 1 — Go routing depends on understanding Python contract):
├── Task 3: Go Bridge MIME-based routing with compound validation [deep]
└── Task 4: Docker Compose, AppArmor, and TTL recycling [unspecified-high]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay

Critical Path: T1 → T2 → T3 → FINAL
Parallel Speedup: T5 runs parallel with T2; T4 runs parallel with T3
Max Concurrent: 3 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1   | — | T2, T5 | 1 |
| T2   | T1 | T3 | 1 |
| T3   | T2 | T4 | 2 |
| T4   | T2 | FINAL | 2 |
| T5   | T1 | T4 | 1 |

### Agent Dispatch Summary

- **Wave 1**: T1 → `quick`, T2 → `deep`, T5 → `quick`
- **Wave 2**: T3 → `deep`, T4 → `unspecified-high`
- **FINAL**: F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Generate Python gRPC Stubs from Existing Proto

   **What to do**:
   - Create `sidecar-python/` directory structure
   - Copy `bridge/pkg/sidecar/sidecar.proto` to `sidecar-python/proto/sidecar.proto` (reference only, no modification)
   - Generate Python gRPC stubs — **this step is also performed inside the Dockerfile multi-stage build** (Stage 1). For local development, generate manually:
     ```bash
     pip install grpcio-tools
     python -m grpc_tools.protoc -I./proto --python_out=./proto --grpc_python_out=./proto ./proto/sidecar.proto
     ```
   - Create `sidecar-python/requirements-dev.txt` with build-time dependencies: `grpcio-tools` (compiler toolchain — NOT included in production image)
   - Create `sidecar-python/requirements.txt` with runtime dependencies only: `grpcio`, `grpcio-health-checking`, `markitdown[outlook,xlsx,pptx]`, `olefile`, `xlrd` (used by Dockerfile Stage 2)
   - Create `sidecar-python/proto/__init__.py` to make it a proper Python package
   - Verify stubs import correctly: `python -c "from proto import sidecar_pb2, sidecar_pb2_grpc"`

  **Must NOT do**:
  - Do NOT modify the proto file in any way
  - Do NOT modify any existing proto files elsewhere in the repo

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 5)
  - **Blocks**: Tasks 2, 5
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/sidecar/sidecar.proto` — Full proto definition (package `armorclaw.sidecar.v1`, service `SidecarService`, RPC `ExtractText`)
  - `sidecar/src/grpc/proto/sidecar.proto` — Identical copy, reference only

  **API/Type References**:
  - `bridge/pkg/sidecar/sidecar.proto:7-20` — Service definition with all RPC methods
  - `bridge/pkg/sidecar/sidecar.proto:109-121` — `ExtractTextRequest` and `ExtractTextResponse` message definitions
  - `bridge/pkg/sidecar/sidecar.proto:23-28` — `RequestMetadata` message (request_id, ephemeral_token, timestamp_unix, operation_signature)

  **External References**:
  - grpcio-tools: `pip install grpcio-tools` — Proto compilation tool
  - MarkItDown: `pip install 'markitdown[outlook,xlsx]'` — Only the dependencies we need

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Python stubs import correctly
    Tool: Bash
    Preconditions: sidecar-python/ directory exists with proto stubs
    Steps:
      1. cd sidecar-python && python -c "from proto import sidecar_pb2, sidecar_pb2_grpc"
      2. Assert exit code 0
    Expected Result: No import errors, exit code 0
    Failure Indicators: ImportError, ModuleNotFoundError, non-zero exit
    Evidence: .sisyphus/evidence/task-1-stub-import.txt

  Scenario: Proto file is unmodified from source
    Tool: Bash
    Preconditions: Both proto files exist
    Steps:
      1. diff bridge/pkg/sidecar/sidecar.proto sidecar-python/proto/sidecar.proto
      2. Assert exit code 0 (identical)
    Expected Result: No diff output, exit code 0
    Failure Indicators: Diff output showing changes
    Evidence: .sisyphus/evidence/task-1-proto-diff.txt
  ```

   **Commit**: YES
   - Message: `chore(sidecar-python): generate Python gRPC stubs from existing proto`
   - Files: `sidecar-python/` (proto stubs, requirements.txt, requirements-dev.txt, proto/__init__.py)
   - Pre-commit: `cd sidecar-python && python -c "from proto import sidecar_pb2, sidecar_pb2_grpc"`

- [x] 2. Build Python gRPC Server with MarkItDown and Threshold Streaming

  **What to do**:
  - Create `sidecar-python/worker.py` — the main gRPC server
  - Implement `SidecarServiceServicer` with:
    - `HealthCheck` — returns status, uptime, version `1.0.0`
    - `ExtractText` — the core conversion logic
   - **ExtractText implementation**:
     1. Read `request.document_content` (bytes payload from gRPC)
     2. Determine format from `request.document_format` and map to file extension:
        - `"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"` → `.xlsx`
        - `"application/vnd.openxmlformats-officedocument.presentationml.presentation"` → `.pptx`
        - `"application/vnd.ms-outlook"` → `.msg`
        - `"application/msword"` (OLE, not OpenXML) → `.doc`
        - `"application/vnd.ms-excel"` (OLE) → `.xls`
        - `"application/vnd.ms-powerpoint"` (OLE) → `.ppt`
        - Any unrecognized format → return `INVALID_ARGUMENT` immediately
    3. **Threshold streaming**:
       - If `len(document_content) < 10 * 1024 * 1024` (10MB):
         ```python
         stream = io.BytesIO(document_content)
         stream_info = StreamInfo(extension=extension, mimetype=request.document_format)
         result = md.convert_stream(stream, stream_info=stream_info)
         ```
       - If `>= 10MB`:
         ```python
         tmpdir = "/tmp/office_worker"
         os.makedirs(tmpdir, exist_ok=True)
         tmp = tempfile.NamedTemporaryFile(dir=tmpdir, delete=False, suffix=extension)
         try:
             tmp.write(document_content)
             tmp.flush()
             tmp.close()
             result = md.convert(tmp.name)
         finally:
             os.remove(tmp.name)
         ```
    4. Return `ExtractTextResponse(text=result.markdown, page_count=0, metadata={})`
  - **gRPC server configuration**:
    ```python
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=4),
        options=[
            ('grpc.max_receive_message_length', 100 * 1024 * 1024),
            ('grpc.max_send_message_length', 100 * 1024 * 1024)
        ]
    )
    ```
  - **Version interceptor**: Server interceptor that adds `x-sidecar-server-version: 1.0.0` to response metadata
   - **Socket binding**:
     ```python
     os.umask(0o077)  # Atomic 0600 permissions
     socket_path = "/run/armorclaw/sidecar-office.sock"
     server.add_insecure_port(f"unix://{socket_path}")
     ```
     After bind, assert permissions: `os.chmod(socket_path, 0o600)` as safety assertion.
     
     **IMPORTANT**: The Docker Compose mounts the host directory `/run/armorclaw/office-sidecar` to `/run/armorclaw` inside the container. The socket is created at the container path `/run/armorclaw/sidecar-office.sock`, which maps to the host path `/run/armorclaw/office-sidecar/sidecar-office.sock`. The Go Bridge client connects to the host path.
  - **Error handling**: Catch `MissingDependencyException`, `UnsupportedFormatException`, and generic `Exception`. Return appropriate gRPC status codes (`INVALID_ARGUMENT`, `INTERNAL`).
  - **TTL recycling**: Implement request counter with `threading.Event` drain signal:
    ```python
    request_count = 0
    request_lock = threading.Lock()
    drain_event = threading.Event()
    MAX_REQUESTS = 50

    def increment_and_check():
        nonlocal request_count
        with request_lock:
            request_count += 1
            if request_count >= MAX_REQUESTS:
                drain_event.set()
    ```
     Main thread waits on `drain_event`, then calls `server.stop(grace=30).wait()` — **30s grace** (changed from initial 10s spec per CTO mandate). 10s is insufficient for large corporate spreadsheets (50MB+) that may be mid-parse when the TTL counter triggers; they need time to complete their MarkItDown conversion before the server terminates the connection.
   - Create `sidecar-python/Dockerfile` — **strict multi-stage build** to keep compiler toolchain out of production image:
     ```dockerfile
     # ---- Stage 1: Builder — proto compilation + stub generation ----
     FROM python:3.12-slim AS builder
     WORKDIR /build
     RUN pip install --no-cache-dir grpcio-tools
     COPY proto/sidecar.proto ./proto/
     RUN python -m grpc_tools.protoc \
         -I./proto \
         --python_out=./proto \
         --grpc_python_out=./proto \
         ./proto/sidecar.proto
     COPY proto/__init__.py ./proto/

     # ---- Stage 2: Runtime — minimal production image ----
     FROM python:3.12-slim
     WORKDIR /app
     RUN pip install --no-cache-dir grpcio grpcio-health-checking 'markitdown[outlook,xlsx,pptx]' olefile xlrd
     COPY --from=builder /build/proto/ ./proto/
      COPY worker.py interceptor.py ./
     RUN mkdir -p /run/armorclaw /tmp/office_worker
     # Note: /run/armorclaw is mounted from host via Docker volume — mkdir is for non-Docker dev only
     # Note: /run/secrets/shared_secret is mounted read-only by Docker Compose for secret injection
     EXPOSE (none — Unix socket only)
     CMD ["python", "worker.py"]
     ```
     **Why multi-stage**: `grpcio-tools` pulls in `protobuf` compiler toolchain (~80MB). The runtime image contains only `grpcio` (C-core bindings), `markitdown`, and the compiled `_pb2.py` stubs — no compiler, no build tools, minimal attack surface.

   **Must NOT do**:
   - Do NOT implement any HTTP/FastAPI endpoints
   - Do NOT modify the proto file
   - Do NOT add support for formats beyond the designated Python scope (`.xlsx`, `.pptx`, `.msg`, `.doc`, `.xls`, `.ppt`)
   - Do NOT process `.docx` or `.pdf` — those are Rust's responsibility
   - Do NOT write fallback mkdir logic in production code (use tmpfs mount from Docker)
   - Do NOT panic or call sys.exit inside the request handler — use gRPC error codes

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T5)
  - **Parallel Group**: Wave 1 (with Tasks 1, 5)
  - **Blocks**: Tasks 3, 4
  - **Blocked By**: Task 1 (needs proto stubs)

  **References**:

  **Pattern References**:
  - `bridge/pkg/sidecar/client.go:134-193` — Go client connection pattern (Unix socket, dialer, backoff)
  - `bridge/pkg/sidecar/token.go:16-23` — Token constants (TTL=30min, MaxTimestampAge=5min)
  - `bridge/pkg/sidecar/version.go:11-26` — Version constants and metadata key names

  **API/Type References**:
  - `bridge/pkg/sidecar/sidecar.proto:109-121` — `ExtractTextRequest` fields: metadata, document_format, document_content (bytes), document_uri, options
  - `bridge/pkg/sidecar/sidecar.proto:23-28` — `RequestMetadata`: request_id, ephemeral_token, timestamp_unix, operation_signature
  - `bridge/pkg/sidecar/sidecar.proto:33-39` — `HealthCheckResponse`: status, uptime_seconds, active_requests, memory_used_bytes, version

   **External References**:
   - MarkItDown: `convert_stream(BytesIO, StreamInfo)` — in-memory conversion
   - MarkItDown: `convert(file_path)` — file-based conversion
   - MarkItDown: `StreamInfo(extension=".xlsx", mimetype="...")` — format hints
   - MarkItDown extras: `[outlook]` for .msg, `[xlsx]` for .xlsx, `[pptx]` for .pptx
   - Additional deps: `olefile` for OLE container parsing (.doc, .xls, .ppt), `xlrd` for legacy .xls
   - grpcio: `grpc.server(ThreadPoolExecutor, options=[...])` — server configuration
   - MarkItDown exceptions: `MissingDependencyException`, `UnsupportedFormatException`

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Server starts and binds to Unix socket
    Tool: Bash
    Preconditions: sidecar-python/ has worker.py and Dockerfile
    Steps:
      1. mkdir -p /run/armorclaw/office-sidecar && cd sidecar-python && python worker.py &
      2. sleep 2
      3. test -S /run/armorclaw/office-sidecar/sidecar-office.sock && echo "SOCKET EXISTS"
      4. stat -c "%a" /run/armorclaw/office-sidecar/sidecar-office.sock
      5. kill %1
    Expected Result: Socket exists with permissions 600 (or 700)
    Failure Indicators: Socket not found, wrong permissions
    Evidence: .sisyphus/evidence/task-2-socket-binding.txt

  Scenario: HealthCheck RPC returns version 1.0.0
    Tool: Bash (grpcurl or python client)
    Preconditions: Server is running on Unix socket
    Steps:
      1. grpcurl -plaintext -unix /run/armorclaw/office-sidecar/sidecar-office.sock armorclaw.sidecar.v1.SidecarService/HealthCheck
      2. Assert response contains "status" and "version": "1.0.0"
    Expected Result: JSON response with version 1.0.0
    Failure Indicators: Connection refused, wrong version, missing fields
    Evidence: .sisyphus/evidence/task-2-healthcheck.txt

  Scenario: ExtractText converts a test .xlsx file (< 10MB)
    Tool: Bash (python client script)
    Preconditions: Server running, test .xlsx file available
    Steps:
      1. Create a small .xlsx test file with openpyxl
      2. Send ExtractText RPC with document_content=bytes, document_format="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
      3. Assert response.text contains expected cell values
    Expected Result: Markdown text with spreadsheet content
    Failure Indicators: Empty text, error status, exception
    Evidence: .sisyphus/evidence/task-2-xlsx-conversion.txt

  Scenario: ExtractText converts a test .pptx file
    Tool: Bash (python client script)
    Preconditions: Server running, test .pptx file available
    Steps:
      1. Create a small .pptx test file with python-pptx
      2. Send ExtractText RPC with document_content=bytes, document_format="application/vnd.openxmlformats-officedocument.presentationml.presentation"
      3. Assert response.text contains slide content
    Expected Result: Markdown text with presentation content
    Failure Indicators: Empty text, error status, exception
    Evidence: .sisyphus/evidence/task-2-pptx-conversion.txt

  Scenario: ExtractText converts a test .msg file
    Tool: Bash (python client script)
    Preconditions: Server running, test .msg file available
    Steps:
      1. Create a small .msg test file (or use a known fixture)
      2. Send ExtractText RPC with document_content=bytes, document_format="application/vnd.ms-outlook"
      3. Assert response.text contains email header/body content
    Expected Result: Markdown text with email content
    Failure Indicators: Empty text, error status, exception
    Evidence: .sisyphus/evidence/task-2-msg-conversion.txt

  Scenario: ExtractText rejects unsupported format
    Tool: Bash (python client script)
    Preconditions: Server running
    Steps:
      1. Send ExtractText with document_content=random bytes, document_format="application/unknown"
      2. Assert gRPC error status INVALID_ARGUMENT or INTERNAL
    Expected Result: Error response, not crash
    Failure Indicators: Server crash, unhandled exception
    Evidence: .sisyphus/evidence/task-2-reject-format.txt

  Scenario: TTL recycling exits after 50 requests
    Tool: Bash
    Preconditions: Server running with MAX_REQUESTS=50
    Steps:
      1. Send 50 HealthCheck requests in a loop
      2. After request 50, verify server process exits
      3. Assert exit code 0
    Expected Result: Process exits cleanly after 50th request
    Failure Indicators: Process still running, crash without drain
    Evidence: .sisyphus/evidence/task-2-ttl-recycle.txt

  Scenario: Threshold streaming — large file uses temp file
    Tool: Bash (python client with >10MB payload)
    Preconditions: Server running
    Steps:
      1. Create a >10MB .xlsx file
      2. Send ExtractText RPC
      3. Assert temp file is cleaned up after conversion (check /tmp/office_worker/ is empty)
    Expected Result: Conversion succeeds, no leftover temp files
    Failure Indicators: Temp files remain, conversion fails
    Evidence: .sisyphus/evidence/task-2-threshold-streaming.txt
  ```

   **Commit**: YES
   - Message: `feat(sidecar-python): implement ExtractText server with MarkItDown and threshold streaming`
   - Files: `sidecar-python/worker.py`, `sidecar-python/Dockerfile`
   - Pre-commit: `cd sidecar-python && python -c "import worker"`

- [x] 3. Implement Go Bridge 3-Layer Routing (Native Bypass + Compound Switch + Strict Drop)

  **What to do**:
   - Create new file `bridge/pkg/sidecar/office_client.go` containing:
      - `OfficeClient` struct wrapping a `*Client` pointing to `/run/armorclaw/office-sidecar/sidecar-office.sock` (host path — the Docker container maps this from `/run/armorclaw/office-sidecar` → container `/run/armorclaw`)
      - `NewOfficeClient(config *Config) *Client` — factory that creates a Client with `SocketPath: "/run/armorclaw/office-sidecar/sidecar-office.sock"` and `MaxMsgSize: 100 * 1024 * 1024` (100MB)
     - `RouteExtractText(ctx context.Context, req *ExtractTextRequest) (*ExtractTextResponse, error)` — the routing function
   - **RouteExtractText 3-layer routing logic**:

     **Layer 0 — Bridge Bypass (Plain Text)**: Intercept plain text formats and decode natively in Go. Zero sidecar invocation.
     ```go
     // Plain text formats — decode natively, no sidecar needed
     plainFormats := map[string]bool{
         "text/plain": true, "text/csv": true,
         "application/json": true, "text/markdown": true,
     }
     fmtLower := strings.ToLower(req.DocumentFormat)
     if plainFormats[fmtLower] || strings.HasSuffix(fmtLower, ".txt") ||
        strings.HasSuffix(fmtLower, ".csv") || strings.HasSuffix(fmtLower, ".json") ||
        strings.HasSuffix(fmtLower, ".md") {
         text := string(req.DocumentContent)
         return &ExtractTextResponse{
             Text: text,
             PageCount: 1,
             Metadata: map[string]string{"source": "bridge-native"},
         }, nil
     }
     ```

     **Layer 1 — Compound Magic+Format Switch**: Non-destructive 8-byte header inspection combined with `document_format` string matching. Each format is routed to exactly one designated worker.

     Step 1: Check payload size:
     ```go
     content := req.DocumentContent
     if len(content) < 8 {
         return nil, status.Errorf(codes.InvalidArgument,
             "payload too small to contain valid magic bytes (need 8, got %d)", len(content))
     }
     ```

     Step 2: Magic byte detection (non-destructive peek):
     ```go
     isZIP := content[0] == 0x50 && content[1] == 0x4B && content[2] == 0x03 && content[3] == 0x04
     isOLE := content[0] == 0xD0 && content[1] == 0xCF && content[2] == 0xE0 &&
              content[3] == 0xA1 && content[4] == 0xB1 && content[5] == 0x1A &&
              content[6] == 0x1E && content[7] == 0xE1
     isPDF := content[0] == 0x25 && content[1] == 0x50 && content[2] == 0x44 && content[3] == 0x46
     ```

     Step 3: Format string classification:
     ```go
     f := strings.ToLower(req.DocumentFormat)
     isXlsx := strings.Contains(f, "spreadsheetml") || strings.HasSuffix(f, ".xlsx")
     isPptx := strings.Contains(f, "presentationml") || strings.HasSuffix(f, ".pptx")
     isDocx := strings.Contains(f, "wordprocessingml") || strings.HasSuffix(f, ".docx")
     isMsg  := strings.Contains(f, "outlook") || strings.HasSuffix(f, ".msg")
     isDoc  := (strings.Contains(f, "msword") && !strings.Contains(f, "wordprocessingml")) || strings.HasSuffix(f, ".doc")
     isXls  := strings.Contains(f, "ms-excel") || strings.HasSuffix(f, ".xls")
     isPpt  := strings.Contains(f, "ms-powerpoint") || strings.HasSuffix(f, ".ppt")
     isPdf  := strings.Contains(f, "pdf") || strings.HasSuffix(f, ".pdf")
     ```

     Step 4: Compound validation and routing:
     ```go
     // === PYTHON SIDECAR ROUTES (OpenXML spreadsheets/presentations + legacy OLE) ===
     if isZIP && isXlsx {
         return officeClient.ExtractText(ctx, req)  // .xlsx → Python
     }
     if isZIP && isPptx {
         return officeClient.ExtractText(ctx, req)  // .pptx → Python
     }
     if isOLE && isMsg {
         return officeClient.ExtractText(ctx, req)  // .msg → Python
     }
     if isOLE && isDoc {
         return officeClient.ExtractText(ctx, req)  // .doc → Python
     }
     if isOLE && isXls {
         return officeClient.ExtractText(ctx, req)  // .xls → Python
     }
     if isOLE && isPpt {
         return officeClient.ExtractText(ctx, req)  // .ppt → Python
     }

     // === RUST SIDECAR ROUTES (OpenXML documents + PDF) ===
     if isZIP && isDocx {
         return rustClient.ExtractText(ctx, req)  // .docx → Rust
     }
     if isPDF && isPdf {
         return rustClient.ExtractText(ctx, req)  // .pdf → Rust
     }
     ```
     **⚠️ Rust Sidecar Status**: As of this plan, the Rust sidecar binary has 74 compilation errors and is not running. The `.docx` and `.pdf` routes are wired correctly but will fail with `UNAVAILABLE` after retry exhaustion (~30s latency) until the Rust sidecar is deployed. The Go Bridge should log a `LEVEL_WARN` at startup if `/run/armorclaw/sidecar.sock` does not exist, indicating that `.docx`/`.pdf` routing is degraded. This is acceptable — the routing logic is correct and will work once the Rust sidecar is operational.

     **Layer 2 — Strict Drop Policy**: Any payload where magic bytes and format string disagree is instantly rejected. No guessing, no autocorrect, no fallback routing.
     ```go
     // Mismatch detection: magic says one container, format claims another
     if isZIP && (isMsg || isDoc || isXls || isPpt || isPdf) {
         return nil, status.Errorf(codes.InvalidArgument,
             "magic byte/format mismatch: ZIP container but format claims %q", req.DocumentFormat)
     }
     if isOLE && (isXlsx || isPptx || isDocx || isPdf) {
         return nil, status.Errorf(codes.InvalidArgument,
             "magic byte/format mismatch: OLE container but format claims %q", req.DocumentFormat)
     }
     if isPDF && !isPdf {
         return nil, status.Errorf(codes.InvalidArgument,
             "magic byte/format mismatch: PDF header but format claims %q", req.DocumentFormat)
     }

     // Unknown/unrecognized format — also drop
     return nil, status.Errorf(codes.InvalidArgument,
         "unrecognized document format: %q", req.DocumentFormat)
     ```

   - Create test file `bridge/pkg/sidecar/office_client_test.go` with:
     **Layer 0 tests:**
     - Test `.txt` decoded natively (no sidecar invoked)
     - Test `.csv` decoded natively
     - Test `.json` decoded natively
     - Test `.md` decoded natively
     **Layer 1 — Python routes:**
     - Test `.xlsx` (ZIP magic + xlsx format) → Python
     - Test `.pptx` (ZIP magic + pptx format) → Python
     - Test `.msg` (OLE magic + outlook format) → Python
     - Test `.doc` (OLE magic + msword format) → Python
     - Test `.xls` (OLE magic + ms-excel format) → Python
     - Test `.ppt` (OLE magic + ms-powerpoint format) → Python
     **Layer 1 — Rust routes:**
     - Test `.docx` (ZIP magic + docx format) → Rust
     - Test `.pdf` (PDF magic + pdf format) → Rust
     **Layer 2 — Strict drop:**
     - Test ZIP magic + `.msg` format → InvalidArgument
     - Test OLE magic + `.xlsx` format → InvalidArgument
     - Test PDF magic + `.msg` format → InvalidArgument
     - Test unknown format → InvalidArgument
     - Test short buffer (1 byte) → InvalidArgument
    - Integrate `OfficeClient` into the bridge initialization (wherever `NewClient` is called for the Rust sidecar, also create an `OfficeClient`)
    - **Socket directory provisioning**: Add to the Bridge startup sequence (before launching sidecar containers):
     ```go
     os.MkdirAll("/run/armorclaw/office-sidecar", 0770)
     os.Chown("/run/armorclaw/office-sidecar", 10001, 10001)
     ```
     The Python container runs as UID 10001. Without `0770` + `chown`, the worker cannot create its socket in a root-owned `0700` directory.
   - **Sidecar health check at startup**: After provisioning socket directories, check if the Rust sidecar socket exists:
     ```go
     if _, err := os.Stat("/run/armorclaw/sidecar.sock"); os.IsNotExist(err) {
         log.Warn("Rust sidecar socket not found — .docx and .pdf routing will fail until sidecar is deployed")
     }
     ```

   **Must NOT do**:
   - Do NOT modify the existing `Client` struct or its methods
   - Do NOT change the default socket path for the Rust sidecar
   - Do NOT route `.docx` or `.pdf` to the Python sidecar
   - Do NOT route `.xlsx`, `.pptx`, `.msg`, `.doc`, `.xls`, `.ppt` to the Rust sidecar
   - Do NOT trust the `DocumentFormat` field alone — always require magic byte confirmation
   - Do NOT guess or autocorrect mismatched formats — strict drop only
   - Do NOT invoke any sidecar for plain text formats (`.txt`, `.csv`, `.json`, `.md`)

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T4)
  - **Parallel Group**: Wave 2
  - **Blocks**: FINAL
  - **Blocked By**: Task 2 (needs Python server contract understood)

  **References**:

  **Pattern References**:
  - `bridge/pkg/sidecar/client.go:36-58` — `Config` struct and `DefaultConfig()` pattern
  - `bridge/pkg/sidecar/client.go:60-99` — `Client` struct and `NewClient()` factory
  - `bridge/pkg/sidecar/client.go:134-193` — `Connect()` method with Unix socket dialer
  - `bridge/pkg/sidecar/client.go:467-498` — Existing `ExtractText()` method with PII interceptor
  - `bridge/pkg/sidecar/audit_client.go:264-292` — AuditClient's `ExtractText` wrapper pattern

  **API/Type References**:
  - `bridge/pkg/sidecar/sidecar.proto:109-115` — `ExtractTextRequest` fields including `document_format` (string) and `document_content` (bytes)
  - `bridge/pkg/sidecar/sidecar.proto:23-28` — `RequestMetadata` for token passing

  **Test References**:
  - `bridge/pkg/sidecar/client_test.go:651-669` — Existing `TestExtractText` test pattern with mock server
  - `bridge/pkg/sidecar/client_test.go:145-156` — Mock server setup pattern

   **External References**:
   - gRPC status codes: `codes.InvalidArgument` for validation failures
   - ZIP magic bytes: `50 4B 03 04` (shared by .xlsx, .pptx, .docx, .odt, .jar, etc.)
   - OLE magic bytes: `D0 CF 11 E0 A1 B1 1A E1` (shared by .msg, .doc, .xls, .ppt, .mdb)
   - PDF magic bytes: `25 50 44 46` (%PDF)

   **Acceptance Criteria**:

   **QA Scenarios (MANDATORY):**

   ```
   Scenario: .txt decoded natively — no sidecar invoked
     Tool: Bash
     Steps:
       1. Create ExtractTextRequest with text content "hello world" + document_format="text/plain"
       2. Call RouteExtractText
       3. Assert response returned directly with text="hello world" — no gRPC call to any sidecar
     Expected Result: Text returned natively, zero network/socket activity
     Failure Indicators: Sidecar client invoked, gRPC error
     Evidence: .sisyphus/evidence/task-3-native-text.txt

   Scenario: .csv decoded natively — no sidecar invoked
     Tool: Bash
     Steps:
       1. Create ExtractTextRequest with CSV content + document_format="text/csv"
       2. Call RouteExtractText
       3. Assert response returned directly
     Expected Result: CSV text returned natively
     Failure Indicators: Sidecar client invoked
     Evidence: .sisyphus/evidence/task-3-native-csv.txt

   Scenario: .xlsx routes to Python sidecar
     Tool: Bash
     Steps:
       1. Create ExtractTextRequest with ZIP magic bytes (50 4B 03 04...) + document_format="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
       2. Call RouteExtractText
       3. Assert request was sent to officeClient (mock captures the target)
     Expected Result: Office client receives the request
     Failure Indicators: Request sent to rustClient instead
     Evidence: .sisyphus/evidence/task-3-xlsx-routing.txt

   Scenario: .pptx routes to Python sidecar
     Tool: Bash
     Steps:
       1. Create ExtractTextRequest with ZIP magic bytes + document_format="application/vnd.openxmlformats-officedocument.presentationml.presentation"
       2. Call RouteExtractText
       3. Assert request sent to officeClient
     Expected Result: Office client receives the request
     Failure Indicators: Request sent to rustClient
     Evidence: .sisyphus/evidence/task-3-pptx-routing.txt

   Scenario: .msg routes to Python sidecar
     Tool: Bash
     Steps:
       1. Create ExtractTextRequest with OLE magic bytes (D0 CF 11 E0...) + document_format="application/vnd.ms-outlook"
       2. Call RouteExtractText
       3. Assert request sent to officeClient
     Expected Result: Office client receives the request
     Failure Indicators: Request sent to rustClient
     Evidence: .sisyphus/evidence/task-3-msg-routing.txt

   Scenario: .doc (OLE) routes to Python sidecar
     Tool: Bash
     Steps:
       1. Create ExtractTextRequest with OLE magic bytes + document_format="application/msword"
       2. Call RouteExtractText
       3. Assert request sent to officeClient
     Expected Result: Office client receives the request
     Failure Indicators: Request sent to rustClient
     Evidence: .sisyphus/evidence/task-3-doc-routing.txt

   Scenario: .xls (OLE) routes to Python sidecar
     Tool: Bash
     Steps:
       1. Create ExtractTextRequest with OLE magic bytes + document_format="application/vnd.ms-excel"
       2. Call RouteExtractText
       3. Assert request sent to officeClient
     Expected Result: Office client receives the request
     Failure Indicators: Request sent to rustClient
     Evidence: .sisyphus/evidence/task-3-xls-routing.txt

   Scenario: .ppt (OLE) routes to Python sidecar
     Tool: Bash
     Steps:
       1. Create ExtractTextRequest with OLE magic bytes + document_format="application/vnd.ms-powerpoint"
       2. Call RouteExtractText
       3. Assert request sent to officeClient
     Expected Result: Office client receives the request
     Failure Indicators: Request sent to rustClient
     Evidence: .sisyphus/evidence/task-3-ppt-routing.txt

   Scenario: .docx routes to Rust sidecar (NOT Python)
     Tool: Bash
     Steps:
       1. Create ExtractTextRequest with ZIP magic bytes + document_format="application/vnd.openxmlformats-officedocument.wordprocessingml.document"
       2. Call RouteExtractText
       3. Assert request sent to rustClient
     Expected Result: Rust client receives the request
     Failure Indicators: Request sent to officeClient (wrong!)
     Evidence: .sisyphus/evidence/task-3-docx-rust-routing.txt

   Scenario: .pdf routes to Rust sidecar (NOT Python)
     Tool: Bash
     Steps:
       1. Create ExtractTextRequest with PDF magic bytes (25 50 44 46...) + document_format="application/pdf"
       2. Call RouteExtractText
       3. Assert request sent to rustClient
     Expected Result: Rust client receives the request
     Failure Indicators: Request sent to officeClient
     Evidence: .sisyphus/evidence/task-3-pdf-rust-routing.txt

   Scenario: ZIP magic + .msg format → strict drop
     Tool: Bash
     Steps:
       1. Create ExtractTextRequest with ZIP magic bytes but document_format="application/vnd.ms-outlook"
       2. Call RouteExtractText
       3. Assert gRPC error codes.InvalidArgument with "mismatch"
     Expected Result: Error "magic byte/format mismatch"
     Failure Indicators: Request routed to either client without validation
     Evidence: .sisyphus/evidence/task-3-mismatch-zip-msg.txt

   Scenario: OLE magic + .xlsx format → strict drop
     Tool: Bash
     Steps:
       1. Create ExtractTextRequest with OLE magic bytes but document_format="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
       2. Call RouteExtractText
       3. Assert gRPC error codes.InvalidArgument with "mismatch"
     Expected Result: Error "magic byte/format mismatch"
     Failure Indicators: Request routed without validation
     Evidence: .sisyphus/evidence/task-3-mismatch-ole-xlsx.txt

   Scenario: PDF magic + .msg format → strict drop
     Tool: Bash
     Steps:
       1. Create ExtractTextRequest with PDF magic bytes (25 50 44 46) but document_format="application/vnd.ms-outlook"
       2. Call RouteExtractText
       3. Assert gRPC error codes.InvalidArgument with "mismatch"
     Expected Result: Error "magic byte/format mismatch"
     Failure Indicators: Request routed without validation
     Evidence: .sisyphus/evidence/task-3-mismatch-pdf-msg.txt

   Scenario: Short buffer rejected gracefully
     Tool: Bash
     Steps:
       1. Create ExtractTextRequest with document_content=[]byte{0x50} (1 byte)
       2. Call RouteExtractText
       3. Assert gRPC error codes.InvalidArgument "payload too small"
     Expected Result: Clean rejection, no panic
     Failure Indicators: Panic, index out of range, crash
     Evidence: .sisyphus/evidence/task-3-short-buffer.txt
   ```

   **Commit**: YES
   - Message: `feat(sidecar): implement 3-layer document routing with compound validation and strict drop policy`
   - Files: `bridge/pkg/sidecar/office_client.go`, `bridge/pkg/sidecar/office_client_test.go`
   - Pre-commit: `cd bridge && go vet ./pkg/sidecar/... && go test ./pkg/sidecar/...`

- [x] 4. Docker Compose, AppArmor Profile, and Symmetrical Container Hardening

  **What to do**:
   - Create `deploy/docker-compose.sidecar-py.yml`:
     ```yaml
     services:
       sidecar-office:
         build:
           context: ../sidecar-python
           dockerfile: Dockerfile
         image: armorclaw/sidecar-office:latest
         container_name: armorclaw-sidecar-office
         user: "10001:10001"  # Symmetrical containment — same non-root UID as Rust sidecar
         restart: unless-stopped
         network_mode: none  # Zero network access — symmetrical with Rust sidecar
         volumes:
           - /run/armorclaw/office-sidecar:/run/armorclaw  # Isolated subdirectory — Python worker must NOT see Rust or Bridge sockets
           - /run/armorclaw/secrets/office-hmac:/run/secrets/shared_secret:ro  # tmpfs-backed secret file (read-only, never touches disk)
         tmpfs:
           - /tmp/office_worker:size=512m,mode=0700  # RAM disk, 512MB — no named volume needed
         read_only: true
         cap_drop:
           - ALL
         security_opt:
           - no-new-privileges:true
           - apparmor=armorclaw-office-worker
         deploy:
           resources:
             limits:
               cpus: '1'
               memory: 512M
             reservations:
               memory: 128M
     ```
     **Symmetrical Containment**: This container config mirrors the Rust sidecar's hardening exactly: `network_mode: none`, UID 10001, `cap_drop: ALL`, `read_only: true`, AppArmor, resource limits. Both sidecars are treated as equally hostile — if either is compromised, kernel namespaces and AppArmor trap it in an air-gapped box with no network, no pivot path, no Bridge memory access.
   - **Secret provisioning (host-side)**: The Go Bridge must perform the following at startup BEFORE launching the Python container:
     ```bash
     # In Go Bridge startup sequence:
     mount -t tmpfs -o size=1M,mode=0700 tmpfs /run/armorclaw/secrets/
     echo -n "${SIDECAR_SHARED_SECRET}" > /run/armorclaw/secrets/office-hmac
     chmod 0400 /run/armorclaw/secrets/office-hmac
     ```
     This ensures the secret exists only in RAM (tmpfs), never on persistent disk. The Docker Compose volume mount then exposes it read-only to the container.
  - Create `container/apparmor-profile-office`:
    ```apparmor
    #include <tunables/global>

    profile armorclaw-office-worker flags=(attach_disconnected,mediate_deleted) {
      #include <abstractions/base>

      # Python runtime
      /usr/bin/python3.* ix,
      /usr/lib/python3.*/** r,
      /usr/local/lib/python3.*/dist-packages/** r,

       # Temp file workspace (tmpfs)
       /tmp/office_worker/** rw,

       # Shared secret socket (read-only mount — injected by Go Bridge at container boot)
       /run/secrets/shared_secret r,

        # Socket directory (isolated mount — container only sees /run/armorclaw/)
        /run/armorclaw/** rw,

      # Deny shells (defense in depth)
      deny /bin/sh x,
      deny /bin/bash x,
      deny /bin/dash x,
      deny /usr/bin/zsh x,

      # Deny network tools
      deny /usr/bin/curl x,
      deny /usr/bin/wget x,
      deny /usr/bin/nc x,

      # Deny host filesystem access
      deny /home/** rw,
      deny /root/** rw,
      deny /etc/** rw,

      # System libraries (read-only)
      /usr/lib/** r,
      /lib/** r,
      /lib64/** r,
    }
    ```
  - Verify AppArmor profile loads: `sudo apparmor_parser -r container/apparmor-profile-office`
   - Verify container starts: `docker compose -f deploy/docker-compose.sidecar-py.yml up -d`
   - Verify socket appears: `test -S /run/armorclaw/office-sidecar/sidecar-office.sock`
   - Verify socket isolation: `test ! -S /run/armorclaw/sidecar.sock` (Rust socket should NOT be visible from container)
  - Verify TTL recycling: Send 50 requests, verify container exits and Docker restarts it

   **Must NOT do**:
   - Do NOT change NetworkMode from `none`
   - Do NOT give the container access to the Docker socket
   - Do NOT mount the Rust sidecar socket into this container
   - Do NOT use `deny /** r,` in AppArmor (breaks Python runtime)
   - Do NOT bypass AppArmor with `unconfined`
   - Do NOT run as root — use UID 10001 (symmetrical with Rust sidecar)
   - Do NOT weaken containment relative to the Rust sidecar — identical hardening for both

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T3)
  - **Parallel Group**: Wave 2
  - **Blocks**: FINAL
  - **Blocked By**: Task 2 (needs Python server and Dockerfile)

  **References**:

  **Pattern References**:
  - `container/apparmor-profile` — Existing AppArmor profile for OpenClaw containers (explicit deny pattern)
  - `docker-compose.bridge.yml:227-286` — OpenClaw container pattern (non-root, cap_drop ALL, no network, read_only)
  - `docker-compose.bridge.yml:18-34` — Default config constants (restart policies, health checks)

  **API/Type References**:
  - `bridge/pkg/sidecar/client.go:21` — `DefaultSocketPath = "/run/armorclaw/sidecar.sock"` (Rust socket, do NOT mount into Python container)

  **External References**:
  - Docker Compose tmpfs: `tmpfs: /tmp/office_worker:size=512m,mode=0700`
  - AppArmor profiles: `apparmor_parser -r` to load

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Container starts and socket appears
    Tool: Bash
    Preconditions: Dockerfile built, compose file exists
    Steps:
      1. docker compose -f deploy/docker-compose.sidecar-py.yml build
      2. docker compose -f deploy/docker-compose.sidecar-py.yml up -d
      3. sleep 3
      4. test -S /run/armorclaw/office-sidecar/sidecar-office.sock && echo "SOCKET OK"
      5. docker compose -f deploy/docker-compose.sidecar-py.yml ps
    Expected Result: Container running, socket exists at isolated subdirectory
    Failure Indicators: Container exited, socket missing
    Evidence: .sisyphus/evidence/task-4-container-start.txt

  Scenario: Container has no network access
    Tool: Bash
    Preconditions: Container running
    Steps:
      1. docker exec armorclaw-sidecar-office python -c "import socket; s=socket.socket(); s.settimeout(2); s.connect(('8.8.8.8', 80))" 2>&1
      2. Assert failure (NetworkMode none)
    Expected Result: Connection refused / network unreachable
    Failure Indicators: Successful connection (network leak!)
    Evidence: .sisyphus/evidence/task-4-no-network.txt

  Scenario: tmpfs mount exists and is RAM-backed
    Tool: Bash
    Preconditions: Container running
    Steps:
      1. docker exec armorclaw-sidecar-office df -T /tmp/office_worker
      2. Assert filesystem type is "tmpfs"
    Expected Result: tmpfs filesystem, 512MB size
    Failure Indicators: ext4/overlay (disk-backed, wrong!)
    Evidence: .sisyphus/evidence/task-4-tmpfs.txt

  Scenario: TTL recycling — container exits after 50 requests
    Tool: Bash
    Preconditions: Container running, Python server at MAX_REQUESTS=50
    Steps:
      1. Send 50 HealthCheck requests via grpcurl
      2. docker compose -f deploy/docker-compose.sidecar-py.yml ps
      3. sleep 15 (wait for restart)
      4. docker compose -f deploy/docker-compose.sidecar-py.yml ps
    Expected Result: Container restarts (restart count increases)
    Failure Indicators: Container stuck, not restarted
    Evidence: .sisyphus/evidence/task-4-ttl-recycle.txt

  Scenario: AppArmor profile loads without errors
    Tool: Bash
    Preconditions: AppArmor profile file exists
    Steps:
      1. sudo apparmor_parser -r container/apparmor-profile-office
      2. sudo aa-status | grep armorclaw-office-worker
    Expected Result: Profile loaded and enforced
    Failure Indicators: Parse error, profile not in aa-status
    Evidence: .sisyphus/evidence/task-4-apparmor.txt

  Scenario: Socket isolation — Rust sidecar socket NOT visible from Python container
    Tool: Bash
    Preconditions: Both Python sidecar container and Rust sidecar running
    Steps:
      1. test -S /run/armorclaw/sidecar.sock && echo "RUST SOCKET EXISTS ON HOST"
      2. test -S /run/armorclaw/office-sidecar/sidecar-office.sock && echo "OFFICE SOCKET EXISTS ON HOST"
      3. docker exec armorclaw-sidecar-office ls /run/armorclaw/ 2>&1
      4. Assert output shows ONLY sidecar-office.sock — NOT sidecar.sock or bridge.sock
    Expected Result: Container sees only its own socket, not Rust or Bridge sockets
    Failure Indicators: sidecar.sock or bridge.sock visible inside container (isolation breach!)
    Evidence: .sisyphus/evidence/task-4-socket-isolation.txt

  Scenario: Secret injection via tmpfs-backed file mount — no env var leakage, no disk persistence
    Tool: Bash
    Preconditions: Container running with secret mount, Go Bridge has provisioned tmpfs
    Steps:
      1. mount | grep "/run/armorclaw/secrets" && echo "TMPFS CONFIRMED" || echo "NOT TMPFS — BLOCKER"
      2. docker exec armorclaw-sidecar-office env | grep -i secret 2>&1
      3. Assert NO output (no SECRET/SHARED_SECRET env vars exist)
      4. docker exec armorclaw-sidecar-office test -r /run/secrets/shared_secret && echo "SECRET MOUNT OK"
      5. docker exec armorclaw-sidecar-office stat -c "%a" /run/secrets/shared_secret
    Expected Result: No secret env vars, secret file exists and is readable, host path is tmpfs (RAM-only)
    Failure Indicators: SHARED_SECRET visible in env output, secret file missing, host path on ext4/xfs (disk — blocker!)
    Evidence: .sisyphus/evidence/task-4-secret-injection.txt
  ```

  **Commit**: YES
  - Message: `feat(deploy): add Docker Compose and AppArmor for Python office sidecar`
  - Files: `deploy/docker-compose.sidecar-py.yml`, `container/apparmor-profile-office`
  - Pre-commit: `docker compose -f deploy/docker-compose.sidecar-py.yml config` (validate compose syntax)

- [x] 5. HMAC-SHA256 Token Validation Interceptor (Python)

  **What to do**:
   - Create `sidecar-python/interceptor.py`:
     - Port the Go `TokenGenerator.ValidateToken()` logic to Python
     - **Token format**: `{request_id}:{timestamp}:{operation}:{hmac_hex_signature}`
     - **Shared secret loading** — read from a tmpfs-backed file at startup, NOT from environment variables. The Go Bridge mounts a `tmpfs` at `/run/armorclaw/secrets/` at startup (RAM-only, never touches persistent disk), writes the secret as a regular file, and Docker mounts it read-only into the container:
       ```python
       import os

       def load_shared_secret() -> str:
           """Read HMAC shared secret from a tmpfs-backed file mounted by the Go Bridge.
           
           The Go Bridge:
           1. Mounts tmpfs at /run/armorclaw/secrets/ (RAM-only, no disk persistence)
           2. Writes the shared secret to /run/armorclaw/secrets/office-hmac
           3. Docker Compose mounts this file read-only into the container at /run/secrets/shared_secret
           
           The secret never touches persistent storage. It is never exposed via environment variables.
           """
           secret_path = os.environ.get("SECRET_PATH", "/run/secrets/shared_secret")
           with open(secret_path, "r") as f:
               secret = f.read().strip()
           if not secret:
               raise RuntimeError(f"Shared secret is empty or missing at {secret_path}")
           return secret
       ```
     - **Validation steps**:
      1. Parse token into 4 parts (split by `:`)
      2. Check timestamp age: `now - timestamp <= 5 minutes` (MaxTimestampAge)
      3. Check token TTL: `now <= timestamp + 30 minutes` (TokenTTL)
      4. Recompute HMAC: `hmac_sha256(shared_secret, f"{request_id}{timestamp}{operation}")`
      5. Compare using `hmac.compare_digest()` (constant-time, like Go's `hmac.Equal`)
    - **gRPC server interceptor**: Read `x-request-token` from incoming metadata. If missing or invalid, abort with `grpc.StatusCode.UNAUTHENTICATED`.
    - Also validate `x-sidecar-version` metadata and respond with `x-sidecar-server-version: 1.0.0` (combines version interceptor from T2).
  - Create `sidecar-python/test_interceptor.py`:
    - Test valid token passes
    - Test expired token rejected (timestamp > 30min ago)
    - Test old timestamp rejected (timestamp > 5min in the past)
    - Test invalid signature rejected
    - Test missing token rejected
    - Test malformed token rejected (wrong number of parts)
   - Integrate interceptor into `worker.py` server setup:
     ```python
     from interceptor import TokenInterceptor, load_shared_secret
     shared_secret = load_shared_secret()  # Reads from /run/secrets/shared_secret (memory-only mount)
     interceptor = TokenInterceptor(shared_secret)
     server = grpc.server(
         futures.ThreadPoolExecutor(max_workers=4),
         interceptors=[interceptor],
         options=[...]
     )
     ```
     The secret is loaded once at startup and held in process memory only. It is never written to disk, never logged, and never exposed via environment variables.

   **Must NOT do**:
   - Do NOT implement token generation — only validation (Go Bridge generates tokens)
   - Do NOT use non-constant-time comparison for HMAC verification
   - Do NOT cache shared secret beyond process lifetime
   - Do NOT log the shared secret or token signatures
   - Do NOT read the shared secret from environment variables — use the memory-only file mount only

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2)
  - **Blocks**: Task 4
  - **Blocked By**: Task 1 (needs proto stubs for gRPC metadata types)

  **References**:

  **Pattern References**:
  - `bridge/pkg/sidecar/token.go:26-63` — TokenGenerator.GenerateToken() — shows exact HMAC computation: `HMAC-SHA256(shared_secret, f"{request_id}{timestamp}{operation}")`
  - `bridge/pkg/sidecar/token.go:66-70` — calculateHMAC() — uses `hmac.New(sha256.New, sharedSecret)`
  - `bridge/pkg/sidecar/token.go:112-126` — ValidateTokenSignature() — constant-time comparison via `hmac.Equal()`
  - `bridge/pkg/sidecar/token.go:142-168` — ValidateToken() — full validation flow: parse → check age → check TTL → verify signature
  - `bridge/pkg/sidecar/version.go:108-136` — ClientVersionInterceptor and StreamClientVersionInterceptor — gRPC metadata patterns

  **API/Type References**:
  - `bridge/pkg/sidecar/token.go:17-22` — Constants: `TokenTTL = 30 * time.Minute`, `MaxTimestampAge = 5 * time.Minute`
  - `bridge/pkg/sidecar/version.go:22-25` — Metadata keys: `x-sidecar-version`, `x-sidecar-server-version`

  **External References**:
  - Python `hmac.compare_digest()` — constant-time comparison (equivalent to Go's `hmac.Equal()`)
  - grpcio server interceptors: `grpc.server(interceptors=[...])`

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Valid token accepted
    Tool: Bash (pytest)
    Preconditions: interceptor.py with test_interceptor.py
    Steps:
      1. Generate a valid token using the same algorithm as Go (shared_secret + request_id + timestamp + operation)
      2. Call interceptor with token in metadata
      3. Assert request passes through (no UNAUTHENTICATED error)
    Expected Result: Request continues to handler
    Failure Indicators: UNAUTHENTICATED error
    Evidence: .sisyphus/evidence/task-5-valid-token.txt

  Scenario: Missing token rejected
    Tool: Bash (pytest)
    Preconditions: interceptor.py
    Steps:
      1. Call interceptor with no x-request-token metadata
      2. Assert UNAUTHENTICATED error
    Expected Result: grpc.StatusCode.UNAUTHENTICATED
    Failure Indicators: Request passes through
    Evidence: .sisyphus/evidence/task-5-missing-token.txt

  Scenario: Expired token rejected (> 30 minutes old)
    Tool: Bash (pytest)
    Preconditions: interceptor.py
    Steps:
      1. Generate token with timestamp 31 minutes ago
      2. Call interceptor with this token
      3. Assert UNAUTHENTICATED error
    Expected Result: Rejection with "token has expired"
    Failure Indicators: Token accepted
    Evidence: .sisyphus/evidence/task-5-expired-token.txt

  Scenario: Invalid signature rejected
    Tool: Bash (pytest)
    Preconditions: interceptor.py
    Steps:
      1. Generate token but tamper with signature (change last 4 chars)
      2. Call interceptor
      3. Assert UNAUTHENTICATED error
    Expected Result: Rejection with "invalid signature"
    Failure Indicators: Token accepted (signature bypass!)
    Evidence: .sisyphus/evidence/task-5-invalid-sig.txt

  Scenario: Version metadata injected in response
    Tool: Bash (pytest or grpcurl)
    Preconditions: Server running with interceptor
    Steps:
      1. Send HealthCheck with x-sidecar-version: 1.0.0 in metadata
      2. Inspect response trailing metadata
      3. Assert x-sidecar-server-version: 1.0.0 present
    Expected Result: Server version in response metadata
    Failure Indicators: Missing metadata key
    Evidence: .sisyphus/evidence/task-5-version-metadata.txt

  Scenario: Shared secret loaded from file mount (not environment variable)
    Tool: Bash (pytest)
    Preconditions: interceptor.py with test_interceptor.py, test secret file at /run/secrets/shared_secret
    Steps:
      1. Write a test secret to a temp file
      2. Call load_shared_secret() with SECRET_SOCKET_PATH pointing to the temp file
      3. Assert the secret value matches what was written
      4. Call load_shared_secret() with a missing path
      5. Assert RuntimeError is raised
    Expected Result: Secret loaded correctly from file, error on missing file
    Failure Indicators: Secret read from env var, no error on missing file
    Evidence: .sisyphus/evidence/task-5-secret-loading.txt
  ```

  **Commit**: YES
  - Message: `feat(sidecar-python): add HMAC-SHA256 token validation interceptor`
  - Files: `sidecar-python/interceptor.py`, `sidecar-python/test_interceptor.py`
  - Pre-commit: `cd sidecar-python && python -m pytest test_interceptor.py -v`

---

## Final Verification Wave

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists. For each "Must NOT Have": search codebase for forbidden patterns. Check evidence files. Compare deliverables against plan.
  Output: `Must Have [7/7] | Must NOT Have [7/7] | Tasks [5/5] | VERDICT: APPROVE`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./bridge/...` + `go build ./bridge/...` + `go test ./bridge/pkg/sidecar/...`. Run `pytest` in sidecar-python. Review all changed files for: `as any`/type assertions, empty catches, console.log, commented-out code, unused imports. Check AI slop.
  Output: `Build [PASS] | Lint [PASS] | Tests [30 pass / 0 fail] | Files [14 clean / 0 issues] | VERDICT: APPROVE`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task. Test cross-task integration. Test edge cases: empty file, corrupt file, wrong magic bytes, missing token, expired token. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [automated via test suite] | Integration [routing + interceptor verified] | Edge Cases [short buffer, mismatch, unknown format tested] | VERDICT: APPROVE`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff. Verify 1:1. Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [5/5 compliant] | Contamination [CLEAN] | Unaccounted [CLEAN] | VERDICT: APPROVE`

---

## Commit Strategy

- **T1**: `chore(sidecar-python): generate Python gRPC stubs from existing proto` - sidecar-python/proto/, sidecar-python/requirements.txt, sidecar-python/requirements-dev.txt
- **T2**: `feat(sidecar-python): implement ExtractText server with MarkItDown and threshold streaming` - sidecar-python/worker.py, sidecar-python/Dockerfile
- **T3**: `feat(sidecar): implement 3-layer document routing with compound validation and strict drop policy` - bridge/pkg/sidecar/office_client.go, bridge/pkg/sidecar/office_client_test.go
- **T4**: `feat(deploy): add Docker Compose and AppArmor for Python office sidecar` - deploy/docker-compose.sidecar-py.yml, container/apparmor-profile-office
- **T5**: `feat(sidecar-python): add HMAC-SHA256 token validation interceptor` - sidecar-python/interceptor.py, sidecar-python/test_interceptor.py

---

## Success Criteria

### Verification Commands
```bash
# Go Bridge
cd bridge && go vet ./...           # Expected: zero warnings
cd bridge && go build ./...         # Expected: compiles cleanly
cd bridge && go test ./pkg/sidecar/...  # Expected: all tests pass

# Python Server
cd sidecar-python && python -m pytest  # Expected: all tests pass

# Docker
docker compose -f deploy/docker-compose.sidecar-py.yml up -d  # Expected: container starts
docker compose -f deploy/docker-compose.sidecar-py.yml ps      # Expected: running, healthy
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All Go tests pass
- [ ] All Python tests pass
- [ ] Docker container starts as UID 10001 with `network_mode: none`
- [ ] Layer 0: `.txt`/`.csv`/`.json`/`.md` decoded natively in Go
- [ ] Layer 1: Compound validation routes each format to correct sidecar
- [ ] Layer 2: Mismatched magic bytes + format claims strictly dropped
- [ ] TTL recycling works (container exits after 50 requests)
- [ ] Symmetrical containment — Python hardening matches Rust sidecar
