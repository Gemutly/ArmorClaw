# VPS Testing Plan via SSH Tunnel

## Connection Details

**SSH Tunnel Command**:
```bash
ssh -L 4096:127.0.0.1:4096 -o IdentityAgent=none -i ~/.ssh/openclaw_win root@5.183.11.149
```

**Local Endpoints** (via tunnel):
- Bridge: `http://127.0.0.1:4096`
- Bridge Socket: Forwarded via tunnel

**Remote Server**: `5.183.11.149` (root access)

---

## Execution Strategy

### Wave 1: Connectivity & Health (Independent)
├── Task 1: Establish SSH tunnel and verify [quick]
├── Task 2: Check bridge health on VPS [quick]
├── Task 3: Check Matrix/Conduit status [quick]
└── Task 4: Check Docker containers [quick]

### Wave 2: Feature Testing via Matrix (After Wave 1)
├── Task 5: Test Rolodex commands [unspecified-high]
├── Task 6: Test WebDAV commands [unspecified-high]
├── Task 7: Test Calendar commands [unspecified-high]
└── Task 8: Test Three-Way Consent [unspecified-high]

### Wave 3: Verification (After Wave 2)
├── Task 9: Check audit logs [quick]
├── Task 10: Verify security gates [unspecified-high]
└── Task 11: Document results [writing]

---

## TODOs

- [ ] 1. Establish SSH Tunnel and Verify Connectivity

  **What to do**:
  - Open terminal and run the SSH tunnel command
  - Verify tunnel is active
  - Test bridge health endpoint via tunnel

  **Commands**:
  ```bash
  # Terminal 1: Establish tunnel (keep running)
  ssh -L 4096:127.0.0.1:4096 -o IdentityAgent=none -i ~/.ssh/openclaw_win root@5.183.11.149
  
  # Terminal 2: Test connectivity (while tunnel runs)
  curl -v http://127.0.0.1:4096/health
  ```

  **Acceptance Criteria**:
  - [ ] SSH tunnel establishes without error
  - [ ] curl returns 200 OK or bridge response
  - [ ] Tunnel stays active for testing duration

  **Parallelization**: Can start immediately, no dependencies

---

- [ ] 2. Check Bridge Health on VPS

  **What to do**:
  - SSH into VPS directly
  - Check bridge service status
  - Verify bridge socket exists
  - Check bridge logs for errors

  **Commands**:
  ```bash
  # SSH into VPS
  ssh -i ~/.ssh/openclaw_win root@5.183.11.149
  
  # On VPS:
  systemctl status armorclaw-bridge
  ls -la /run/armorclaw/bridge.sock
  journalctl -u armorclaw-bridge -n 50 --no-pager
  ```

  **Acceptance Criteria**:
  - [ ] Bridge service is active (running)
  - [ ] Socket file exists at /run/armorclaw/bridge.sock
  - [ ] No critical errors in logs

  **Parallelization**: Can run in parallel with Task 1 (separate terminal)

---

- [ ] 3. Check Matrix/Conduit Status

  **What to do**:
  - Verify Matrix Conduit is running
  - Test Matrix API endpoint
  - Check if bridge user is registered

  **Commands**:
  ```bash
  # On VPS:
  docker ps | grep conduit
  curl http://localhost:6167/_matrix/client/versions
  
  # Check bridge user (if Matrix is running)
  curl -X GET "http://localhost:6167/_matrix/client/v3/profile/@bridge:localhost"
  ```

  **Acceptance Criteria**:
  - [ ] Conduit container running
  - [ ] Matrix API responds with version info
  - [ ] Bridge user exists (or can be created)

  **Parallelization**: Can run in parallel with Tasks 1-2

---

- [ ] 4. Check Docker Containers

  **What to do**:
  - List all running containers
  - Check container health status
  - Verify no containers are restarting

  **Commands**:
  ```bash
  # On VPS:
  docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
  docker ps -a | grep -v "Up"  # Check for stopped containers
  ```

  **Acceptance Criteria**:
  - [ ] All expected containers running (bridge, conduit, etc.)
  - [ ] No containers in restart loop
  - [ ] Ports exposed correctly

  **Parallelization**: Can run in parallel with Tasks 1-3

---

- [ ] 5. Test Rolodex Commands via Matrix

  **What to do**:
  - Connect to Matrix room via Element X or Matrix client
  - Test all contact management commands
  - Verify CRUD operations work
  - Test encryption of sensitive fields

  **Prerequisite**: Matrix client connected to VPS Matrix server

  **Test Commands** (in Matrix room):
  ```
  !secretary contact create "John Doe" company="Acme" email="john@acme.com" phone="555-1234"
  !secretary contact list
  !secretary contact search "John"
  !secretary contact get <id_from_previous>
  !secretary contact update <id> notes="Test update"
  !secretary contact delete <id>
  ```

  **Acceptance Criteria**:
  - [ ] Contact created successfully
  - [ ] List shows created contact
  - [ ] Search finds contact by name
  - [ ] Get returns full contact details
  - [ ] Update modifies contact
  - [ ] Delete removes contact

  **Parallelization**: Must wait for Wave 1 (connectivity verified)

---

- [ ] 6. Test WebDAV Commands via Matrix

  **What to do**:
  - Test WebDAV operations (if WebDAV server configured)
  - List directory contents
  - Upload test file
  - Download file
  - Delete test file

  **Test Commands** (in Matrix room):
  ```
  !secretary webdav list http://localhost:8080/
  !secretary webdav put http://localhost:8080/test.txt "Hello ArmorClaw"
  !secretary webdav get http://localhost:8080/test.txt
  !secretary webdav delete http://localhost:8080/test.txt
  ```

  **Acceptance Criteria**:
  - [ ] List returns directory contents (or meaningful error if no WebDAV)
  - [ ] Put uploads file successfully
  - [ ] Get retrieves file content
  - [ ] Delete removes file

  **Note**: If WebDAV server not configured, test should return clear error message

  **Parallelization**: Can run in parallel with Tasks 5, 7, 8

---

- [ ] 7. Test Calendar Commands via Matrix

  **What to do**:
  - Test Calendar operations (if CalDAV server configured)
  - List calendars
  - Create test event
  - Get events
  - Delete event

  **Test Commands** (in Matrix room):
  ```
  !secretary calendar list
  !secretary calendar create "Test Meeting" start="2026-03-20T10:00:00Z" end="2026-03-20T11:00:00Z"
  !secretary calendar get_events <calendar_id>
  !secretary calendar delete <event_id>
  ```

  **Acceptance Criteria**:
  - [ ] List returns calendars (or error if no CalDAV configured)
  - [ ] Create adds event to calendar
  - [ ] Get events returns list including new event
  - [ ] Delete removes event

  **Note**: If CalDAV server not configured, test should return clear error message

  **Parallelization**: Can run in parallel with Tasks 5, 6, 8

---

- [ ] 8. Test Three-Way Consent Flow

  **What to do**:
  - Trigger PII access request
  - Verify consent room creation
  - Test approval flow
  - Test rejection flow
  - Verify audit logging

  **Test Commands**:
  ```
  # Trigger via BlindFill with PII reference
  !secretary run blindfill <template_with_pii>
  
  # In consent room:
  !approve <request_id>
  # or
  !reject <request_id> "Test rejection"
  ```

  **Acceptance Criteria**:
  - [ ] Consent room created with correct participants
  - [ ] Approval updates consent state
  - [ ] Rejection blocks PII access
  - [ ] Audit log contains consent event

  **Parallelization**: Can run in parallel with Tasks 5-7

---

- [ ] 9. Check Audit Logs

  **What to do**:
  - SSH into VPS
  - Review audit logs for all test operations
  - Verify no security violations
  - Check PII access is logged

  **Commands**:
  ```bash
  # On VPS:
  journalctl -u armorclaw-bridge --since "1 hour ago" | grep -i audit
  cat /var/log/armorclaw/audit.log | tail -100
  grep -i "pii\|consent\|approval" /var/log/armorclaw/bridge.log | tail -50
  ```

  **Acceptance Criteria**:
  - [ ] Audit logs exist and are readable
  - [ ] Test operations logged with timestamps
  - [ ] PII access events logged
  - [ ] No unauthorized access attempts

  **Parallelization**: Must wait for Wave 2 (feature tests complete)

---

- [ ] 10. Verify Security Gates

  **What to do**:
  - Verify SSRF protection blocks internal URLs
  - Check PII fields require approval
  - Verify no unauthorized skill execution
  - Check container isolation

  **Commands**:
  ```bash
  # Test SSRF protection (should fail)
  !secretary webdav get file:///etc/passwd
  
  # Verify PII requires approval
  !secretary contact create "Test" ssn="123-45-6789"  # Should trigger consent
  
  # Check container security
  docker inspect armorclaw-bridge | grep -A 10 "SecurityOpt"
  ```

  **Acceptance Criteria**:
  - [ ] SSRF protection blocks file:// URLs
  - [ ] PII fields trigger consent flow
  - [ ] Container runs with security options (no-new-privileges, etc.)

  **Parallelization**: Must wait for Wave 2

---

- [ ] 11. Document Test Results

  **What to do**:
  - Compile all test results
  - Document any failures or issues
  - Create summary report
  - Save to `.sisyphus/evidence/vps-test-results.md`

  **Template**:
  ```markdown
  # VPS Test Results
  
  **Date**: 2026-03-15
  **Server**: 5.183.11.149
  **Bridge Version**: v4.11.0
  
  ## Wave 1: Connectivity
  - Task 1 (SSH Tunnel): ✅/❌
  - Task 2 (Bridge Health): ✅/❌
  - Task 3 (Matrix): ✅/❌
  - Task 4 (Docker): ✅/❌
  
  ## Wave 2: Features
  - Task 5 (Rolodex): ✅/❌ - Notes
  - Task 6 (WebDAV): ✅/❌ - Notes
  - Task 7 (Calendar): ✅/❌ - Notes
  - Task 8 (Consent): ✅/❌ - Notes
  
  ## Wave 3: Verification
  - Task 9 (Audit Logs): ✅/❌
  - Task 10 (Security): ✅/❌
  
  ## Issues Found
  1. [Issue description]
  2. [Issue description]
  
  ## Production Ready?
  - [ ] YES - All tests pass
  - [ ] NO - Issues require fixing
  ```

  **Acceptance Criteria**:
  - [ ] All test results documented
  - [ ] Issues clearly listed
  - [ ] Production readiness decision made

  **Parallelization**: Must wait for all Waves complete

---

## Final Verification Wave

- [ ] F1. **Connectivity Audit** — `quick`
  Verify SSH tunnel, bridge health, Matrix, and Docker all pass.
  Output: `Connectivity [4/4] | VERDICT: PASS/FAIL`

- [ ] F2. **Feature Coverage** — `unspecified-high`
  Verify all 4 feature areas tested (Rolodex, WebDAV, Calendar, Consent).
  Output: `Features [4/4 tested] | VERDICT: PASS/FAIL`

- [ ] F3. **Security Verification** — `unspecified-high`
  Verify audit logs, security gates, and PII protection working.
  Output: `Security [PASS/FAIL] | VERDICT: PASS/FAIL`

---

## Success Criteria

### Minimum for Production Ready
- [ ] Wave 1: All 4 tasks pass
- [ ] Wave 2: At least 3/4 features work (WebDAV/Calendar may need server setup)
- [ ] Wave 3: Audit logging works, no security violations
- [ ] Documentation: Test results recorded

### Go/No-Go Decision
- **GO**: All Wave 1 pass + Rolodex + Consent pass + Audit logs work
- **NO-GO**: Any Wave 1 failure, or critical security issue

---

## Quick Start Commands

```bash
# Terminal 1: Establish SSH tunnel
ssh -L 4096:127.0.0.1:4096 -o IdentityAgent=none -i ~/.ssh/openclaw_win root@5.183.11.149

# Terminal 2: Test bridge via tunnel
curl http://127.0.0.1:4096/health

# Terminal 3: SSH for direct VPS access
ssh -i ~/.ssh/openclaw_win root@5.183.11.149
# Then run: systemctl status armorclaw-bridge
```

---

*Generated: 2026-03-15*
*Plan: production-readiness → VPS Testing Phase*
