# Production Readiness Work Plan

## User Requirements (Confirmed)
- **Environment**: VPS (Virtual Private Server)
- **Priority**: All features (WebDAV, Calendar, Rolodex, Consent)
- **Test Env**: VPS is provisioned and ready

## Executive Summary

**Goal**: Go to production. Test all features and ensure OpenClaw agents have access through appropriate channels.

**Critical Gaps Identified**:
| Gap | Impact | Priority |
|-----|--------|----------|
| WebDAV Matrix handlers missing | Users can't test WebDAV via chat | P0 |
| Calendar Matrix handlers missing | Users can't test Calendar via chat | P0 |
| Permission Prediction Layer | OpenClaw can't predict consent needs | P1 |

**Scope Boundaries**:
- INCLUDE: Add missing Matrix command handlers (WebDAV, Calendar)
- INCLUDE: Implement Permission Prediction Layer (MCP tools)
- INCLUDE: Test all features end-to-end on VPS
- EXCLUDE: New feature development beyond identified gaps
- EXCLUDE: Documentation updates (focus on testing)

## Work Phases

### Phase A: Add Missing Matrix Handlers (P0 - Production Blocker)

**Dependencies**: None - Independent feature

**Tasks**:
- A1: Add WebDAV handlers to `secretary_commands.go`
  - `handleWebDAVList(ctx, roomID, args[])`
  - `handleWebDAVGet(ctx, roomID, args[])`
  - `handleWebDAVPut(ctx, roomID, args[])`
  - `handleWebDAVDelete(ctx, roomID, args[])`
  - Add routing case: `webdav`
  - Update help text

- A2: Add Calendar handlers to `secretary_commands.go`
  - `handleCalendarList(ctx, roomID, args[])`
  - `handleCalendarCreate(ctx, roomID, args[])`
  - `handleCalendarGetEvents(ctx, roomID, args[])`
  - `handleCalendarGetEvent(ctx, roomID, args[])`
  - `handleCalendarUpdate(ctx, roomID, args[])`
  - `handleCalendarDelete(ctx, roomID, args[])`
  - Add routing case: `calendar`
  - Update help text

**Acceptance Criteria**:
- [ ] All handlers added to `secretary_commands.go`
- [ ] Command routing works (`webdav`, `calendar`)
- [ ] Help text updated with examples
- [ ] Error messages formatted consistently
- [ ] `go build ./bridge/...` passes
- [ ] Unit tests pass (if any)

**Verification**:
```bash
# Test WebDAV commands
!secretary webdav list http://localhost:8080/
!secretary webdav get http://localhost:8080/test.txt
!secretary webdav put http://localhost:8080/test.txt "Hello World"
!secretary webdav delete http://localhost:8080/test.txt

# Test Calendar commands
!secretary calendar list
!secretary calendar create "Test Event" start="2026-03-20T10:00:00Z" end="2026-03-20T11:00:00Z"
!secretary calendar get <event_id>
!secretary calendar delete <event_id>
```

**Estimated Effort**: Medium (2-3 hours)

---

### Phase B: Implement Permission Prediction Layer (P1 - OpenClaw Access)

**Dependencies**: Phase A complete (Matrix handlers added)

**Tasks**:
- B1: Create `bridge/pkg/permissions/predictor.go`
  - Define `PermissionPredictor` struct
  - `Predict(formID, subjectID)` method
  - `getSensitivity(field)` helper
  - Integrate with forms DB, rolodex DB, consent store

- B2: Add MCP tools for permission flows
  - `predict_permissions` tool
  - `request_consent` tool
  - `check_consent` tool

- B3: Wire into RPC methods
  - Add `permissions_predict` RPC method
  - Add `consent_request` RPC method
  - Add `consent_check` RPC method

- B4: Update skill execution to use permissions
  - Modify `executor.go` to check predictions before execution
  - Add approval flow for sensitive tasks

**Acceptance Criteria**:
- [ ] `predictor.go` created with unit tests
- [ ] MCP tools registered in executor
- [ ] RPC methods added to methods_permissions.go
- [ ] Skills check predictions before execution
- [ ] `go build ./bridge/...` passes
- [ ] Documentation updated

**Verification**:
```bash
# Test permission prediction
curl -X POST http://localhost:8443/api/permissions/predict \
  -H "Content-Type: application/json" \
  -d '{"task_type": "form_fill", "form_id": "hospital_intake", "subject_id": "john_doe"}'

# Test consent request
curl -X POST http://localhost:8443/api/consent/request \
  -H "Content-Type: application/json" \
  -d '{"form_id": "hospital_intake", "subject_id": "john_doe", "fields": ["ssn"]}'

# Test consent check
curl -X POST http://localhost:8443/api/consent/check \
  -H "Content-Type: application/json" \
  -d '{"form_id": "hospital_intake", "subject_id": "john_doe", "field": "ssn"}'
```

**Estimated Effort**: Medium-High (4-6 hours)

---

### Phase C: VPS Testing (All Phases)

**Dependencies**: Phase A + Phase B complete

**Environment**:
- VPS provisioned and accessible
- Docker installed
- ArmorClaw bridge running
- Matrix/Conduit running
- Element X client installed on phone/desktop

**Tasks**:
- C1: Pre-flight checks
  - Verify Docker is running: `docker ps`
  - Verify bridge is running: `systemctl status armorclaw-bridge`
  - Verify Matrix is accessible: `curl http://localhost:6167/_matrix/client/versions`
  - Verify socket exists: `test -S /run/armorclaw/bridge.sock`

- C2: Test Rolodex via Matrix
  - Create contact: `!secretary contact create "Test User" email="test@example.com"`
  - List contacts: `!secretary contact list`
  - Search contacts: `!secretary contact search "Test"`
  - Get contact: `!secretary contact get <id>`
  - Update contact: `!secretary contact update <id> notes="Updated"`
  - Delete contact: `!secretary contact delete <id>`
  - Verify encryption in database

- C3: Test WebDAV via Matrix
  - List files: `!secretary webdav list http://localhost:8080/`
  - Get file: `!secretary webdav get http://localhost:8080/test.txt`
  - Put file: `!secretary webdav put http://localhost:8080/test.txt "Hello World"`
  - Delete file: `!secretary webdav delete http://localhost:8080/test.txt`
  - Verify SSRF protection

- C4: Test Calendar via Matrix
  - List calendars: `!secretary calendar list`
  - Create event: `!secretary calendar create "Meeting" start="2026-03-20T14:00:00Z" end="2026-03-20T15:00:00Z"`
  - Get events: `!secretary calendar get <calendar_id>`
  - Get event: `!secretary calendar get <event_id>`
  - Update event: `!secretary calendar update <event_id> start="2026-03-20T16:00:00Z"`
  - Delete event: `!secretary calendar delete <event_id>`
  - Verify conflict detection

- C5: Test Three-Way Consent
  - Trigger PII access (form with SSN field)
  - Verify consent room creation
  - Verify approval/rejection flow
  - Verify token validation
  - Verify audit logging

- C6: Test Permission Prediction Layer
  - Call `predict_permissions` MCP tool
  - Verify required fields detected
  - Verify consent initiated when needed
  - Verify permission cache

- C7: Test Agent Access
  - Create agent: `!agent create name="TestAgent" skills="web_browsing"`
  - Send task to agent room
  - Verify agent execution
  - Verify skill access via RPC

- C8: End-to-End Verification
  - Verify audit logs for all operations
  - Verify PII not exposed in logs
  - Verify security gates working
  - Verify budget limits enforced

**Acceptance Criteria**:
- [ ] All pre-flight checks pass
- [ ] All Matrix commands work
- [ ] WebDAV operations succeed
- [ ] Calendar operations succeed
- [ ] Three-way consent works
- [ ] Permission prediction works
- [ ] Agent can execute skills
- [ ] No security violations
- [ ] Audit logs complete
- [ ] Documentation verified

**Verification Commands**:
```bash
# Full health check
./deploy/verify-bridge.sh

# Bridge logs
journalctl -u armorclaw-bridge -n 100

# Matrix logs
docker logs armorclaw-conduit -n 100

# Audit logs
grep -r "audit" /var/log/armorclaw/bridge.log

# Test results
cat /tmp/test-results.log
```

**Estimated Effort**: High (8-12 hours including testing time)

---

## File Changes Summary

| Phase | File | Lines Added | Purpose |
|-------|------|--------------|---------|
| A | `bridge/pkg/secretary/secretary_commands.go` | +200 | Matrix handlers |
| B | `bridge/pkg/permissions/predictor.go` | +200 | Permission logic |
| B | `bridge/pkg/rpc/methods_permissions.go` | +100 | RPC methods |
| B | `bridge/internal/executor/engine.go` | +50 | Integration |

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking existing Matrix commands | Medium | Test thoroughly before commit |
| Permission prediction misses edge cases | Medium | Add comprehensive unit tests |
| Consent flow complexity | High | Incremental rollout, monitor logs |
| VPS testing time overrun | Low | Use existing VPS if available |

## Success Criteria

**Definition of Done**:
- All Phase A tasks complete and verified
- All Phase B tasks complete and verified
- All Phase C tasks complete and verified
- Production ready checklist complete
- No P0 blockers remaining

**Go/No-Go Decision Points**:
- If any P0 blocker found after Phase A → Stop, reassess
- If permission prediction proves too complex → Simplify to basic whitelist
- If VPS testing reveals major issues → Fix before proceeding

## Timeline

| Phase | Estimated | Dependencies |
|-------|-----------|--------------|
| A: Matrix Handlers | 2-3 hours | None |
| B: Permission Layer | 4-6 hours | A complete |
| C: VPS Testing | 8-12 hours | A + B complete |
| **Total** | **14-21 hours** | |

## Notes

- Use existing Rolodex and Consent patterns from Phase 12
- Follow error handling patterns in `secretary_commands.go`
- Test each phase before proceeding to next
- Document any workarounds discovered during testing
- Keep audit logs for compliance verification

---

*Generated: 2026-03-15*
