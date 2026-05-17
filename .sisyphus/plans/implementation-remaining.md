# Implementation Plan: Remaining Production Features

## Completed Work

- ✅ Nil pointer crash fixed (sendMessage helper added)
- ✅ HandleMatrixMessage interface method added
- ✅ Rolodex service wired in main.go
- ✅ z.ai API provider configured
- ✅ WebDAV service wrapper created and wired
- ✅ Calendar service wrapper created and wired
- ✅ ApprovalEngine initialized and wired

## Remaining Implementation Work

### Wave 2: Database & Storage Integration

#### Task 1: ✅ COMPLETE - Rolodex Database Initialized
**Status**: Database service created, SQLCipher encryption enabled, wired to secretary handler

#### Task 2: ✅ COMPLETE - Contact CRUD Operations
**Status**: Handlers exist in secretary_commands.go, wired through RolodexService

#### Task 3: ✅ COMPLETE - WebDAV Client Integration
**Status**: WebDAVService wrapper created, ExecuteWebDAV wired, handlers implemented

#### Task 4: ✅ COMPLETE - CalDAV Calendar Integration
**Status**: CalendarService wrapper created, ExecuteCalendar wired, handlers implemented

### Wave 3: Consent & AI Integration

#### Task 5: ✅ COMPLETE - Consent Approval Engine Wired
**Status**: ApprovalEngine initialized with rolodexStore, passed to config

#### Task 6: Test Consent Request Handlers
**What**: Verify consent commands work via Matrix
**Commands to test**:
- `!secretary consent list`
- `!secretary consent approve <request_id>`
- `!secretary consent deny <request_id>`

#### Task 7: Deploy and Test on VPS
**What**: Deploy updated binary to VPS and run integration tests
**Steps**:
1. Build binary locally
2. Deploy to VPS
3. Restart service
4. Test all secretary commands via Matrix

### Wave 4: Integration Testing

#### Task 8: Matrix Command E2E Tests
**What**: Test complete command flows via Matrix API
**Test scenarios**:
- Contact CRUD cycle (create → list → get → update → delete)
- WebDAV operations (if server available)
- Calendar operations (if server available)
- AI completion with z.ai provider

#### Task 9: Final Verification
**What**: Comprehensive system check
**Checklist**:
- [ ] All `!secretary` commands respond without panic
- [ ] Rolodex database working with encryption
- [ ] WebDAV client functional (or gracefully degrades if no server)
- [ ] Calendar client functional (or gracefully degrades if no server)
- [ ] Consent approval flow working
- [ ] AI provider responding

## Execution Strategy

1. **Verify current state**: Build, deploy, test what's working
2. **Fix any issues**: Ensure foundation is solid
3. **Implement remaining features**: Wave 2, then Wave 3
4. **Deploy and test**: Verify on VPS
5. **Document**: Update review.md with final state

## Parallel Execution Opportunities

- Wave 2 Tasks 3-4 (WebDAV/CalDAV) can be parallelized
- Wave 3 Tasks 5-6 can be done together
- Wave 4 testing can start while features are being implemented
