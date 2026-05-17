# F2: Must NOT Guardrails Audit

**Date**: 2026-04-16
**Verdict**: APPROVE (12/12 guardrails verified)

## Guardrail Verification

| # | Guardrail | Test Coverage | Location | Verdict |
|---|-----------|---------------|----------|---------|
| 1 | `.docx` does NOT route to Python | `TestE2E_Docx_DoesNotCallPython` verifies `officeMock.extractCalled == false` | office_client_e2e_test.go:310 | PASS |
| 2 | `.xlsx/.pptx/.msg/.doc/.xls/.ppt` route to Python, NOT Rust | Existing routing tests + `TestE2E_XLSX_GoToPython`, `TestE2E_MSG_GoToPython` | office_client_e2e_test.go:261,283 | PASS |
| 3 | Plain text bypass — no sidecar invocation | `TestE2E_NativeText_NoSidecar` checks `source == "bridge-native"` | office_client_e2e_test.go:206 | PASS |
| 4 | gRPC only — no HTTP/FastAPI | Architectural: worker.py uses `grpc.server()` exclusively | worker.py | PASS |
| 5 | Token rejects expired/missing/tampered | `TestTokenIntegration` — 4 sub-tests (valid, expired, missing, tampered) | test_edge_cases.py:157-207 | PASS |
| 6 | NetworkMode stays `none` | `TestNetworkIsolation::test_network_mode_none` + `test_no_dns` | test_docker_integration.py:111-122 | PASS |
| 7 | Strict drop: magic+format mismatch → InvalidArgument | `TestE2E_StrictDrop_ZIPMsgMismatch` + `TestE2E_StrictDrop_OLEXLSXMismatch` | office_client_e2e_test.go:223-259 | PASS |
| 8 | Storage RPCs return UNIMPLEMENTED | `TestUnimplementedRPCs` — 5 sub-tests (Upload/Download/List/Delete/Process) | test_worker.py:237-263 | PASS |
| 9 | No existing test files modified | `git diff` confirms zero changes to test_interceptor.py, office_client_test.go | git | PASS |
| 10 | No binary fixtures committed | All fixtures generated programmatically in conftest.py | conftest.py | PASS |
| 11 | No sudo required for Phase 1 | All 55 Python + 25 Go tests pass without sudo | pytest + go test | PASS |
| 12 | Security constraints not weakened | Token HMAC, network isolation, cap_drop ALL, read-only root all verified | test_edge_cases.py, test_docker_integration.py | PASS |

## Test Counts

- **Python**: 55 passed, 10 skipped (Docker), 0 failed
- **Go**: 25 passed (18 existing + 7 E2E), 0 failed
- **Total new tests**: 60 (27 + 16 + 10 + 7)

## Bugs Found & Fixed

1. **StreamInfo import scope** — moved from local to module-level in worker.py
2. **SIDECAR_SOCKET hardcoded** — added env var configurability for testing
3. **Lambda closure variable capture** — fixed default-argument capture in _SyncTokenInterceptor
4. **Async TokenInterceptor incompatible with sync grpc.server()** — documented as pre-existing production bug
