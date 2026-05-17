# Beta Hardening Plan

**Branch**: `beta-hardening` (from `main`)
**Goal**: Eliminate security footguns, unblock CI, enforce strict operational correctness for Beta release.

## Constraints
- Do NOT change NetworkMode
- Do NOT fix image mismatch (agent-base:latest)
- Do NOT touch v6 microkernel code
- Do NOT touch sidecar/Qdrant/pdf.rs
- /doc/ is in .gitignore — files must be force-added with `git add -f`
- Do not use `//nolint:govet` comments
- Do not abstract the SQL dialect (license server is Postgres-only)
- Do not panic or call log.Fatalf inside the turn package
- Do not write fallback mkdir logic inside studio tests (pure DI only)

---

## TODOs

### Phase 1: Security & CI Blockers

- [x] T1: Enforce TURN Secret Requirement
- [x] T2: Fix Studio CI via Strict DI

### Phase 2: Core HITL Reliability

- [x] T3: Bound and Configure PII Approval Timeout
- [x] T4: Immediate PII Notification Routing

### Phase 3: Configuration Correctness

- [x] T5: Strict Synchronization of License Grace Periods
- [x] T6: Graceful Degradation on WebSocket Misconfiguration

### Phase 4: Operational Sustainability

- [x] T7: Postgres Validation Table Retention
- [x] T8: Resolve Rolodex go vet Bug

---

## Final Verification Wave

- [x] F1: `go vet ./...` passes with zero warnings
- [x] F2: `go build ./...` passes
- [x] F3: `go test ./bridge/... ./license-server/...` all green
- [x] F4: All doc changes accurate and force-added
