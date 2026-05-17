# Unblock Integration Tests

## TL;DR

> **Quick Summary**: Fix router_test.go mock initialization to use correct pii.HITLConsentManager API, then execute integration tests with sudo Docker commands.
>
> **Deliverables**:
> - Fixed router_test.go using pii.NewHITLConsentManager with correct HITLConfig
> - go test ./pkg/mcp/... passes
> - Task 10: E2E integration test with sudo Docker
> - Task 11: Crash recovery test with sudo Docker
>
> **Estimated Effort**: Quick (15-30 minutes)

---

## Issues to Fix

### Issue 1: Wrong Consent Manager API
**Current (BROKEN):**
```go
mockConsentMgr := governor.NewHITLConsentManager(logger.NewHITLConsentManager(nil), &governor.Config{
    Logger: mockLogger,
})
```

**Fix:**
```go
mockConsentMgr := pii.NewHITLConsentManager(pii.HITLConfig{
    Timeout: 60 * time.Second,
})
```

### Issue 2: CreatedAt Cannot Be nil
**Current (BROKEN):**
```go
CreatedAt: nil,
```

**Fix:**
```go
CreatedAt: time.Now(),
```

---

## Execution Steps

### Step 1: Fix router_test.go

- [ ] **1. Fix mockConsentMgr initialization (multiple locations)**
  - Replace `governor.NewHITLConsentManager(logger.NewHITLConsentManager(nil), &governor.Config{Logger: mockLogger})`
  - With `pii.NewHITLConsentManager(pii.HITLConfig{Timeout: 60 * time.Second})`
  - Occurrences: Lines 58-60, 116-118, 160-162, 199-201, 234-236, 307-309, 346-348

- [ ] **2. Fix CreatedAt field**
  - Replace `CreatedAt: nil,` with `CreatedAt: time.Now(),`
  - Occurrence: Line 45

### Step 2: Verify Tests Pass

- [ ] **3. Run go test**
  ```bash
  cd /home/mink/src/armorclaw-omo/bridge
  go test ./pkg/mcp/... -v
  ```

### Step 3: Integration Tests (with sudo)

- [ ] **4. Create armorclaw-isolated network**
  ```bash
  sudo docker network create armorclaw-isolated --driver bridge --internal
  ```

- [ ] **5. Run E2E Integration Test**
  ```bash
  # Start Rust Vault
  cd /home/mink/src/armorclaw-omo/sidecar
  sudo ./target/release/armorclaw-sidecar &
  
  # Start Go Bridge
  cd /home/mink/src/armorclaw-omo/bridge
  sudo ./armorclaw-bridge &
  
  # Wait for services
  sleep 5
  
  # Test RPC call
  START=$(date +%s.%N)
  echo '{"jsonrpc":"2.0","id":"test-001","method":"browser.navigate","params":{"url":"https://example.com"}}' | sudo nc -U /run/armorclaw/bridge.sock
  END=$(date +%s.%N)
  echo "Round-trip latency: $(echo "$END - $START" | bc) seconds"
  ```

- [ ] **6. Run Crash Recovery Test**
  ```bash
  TOOLSIDECAR_ID=$(sudo docker ps --filter name=toolsidecar -q)
  OPENCLAW_ID=$(sudo docker ps --filter name=openclaw -q)
  sudo docker kill $OPENCLAW_ID
  
  START=$(date +%s)
  for i in {1..60}; do
    if ! sudo docker ps --filter name=toolsidecar | grep -q $TOOLSIDECAR_ID; then
      END=$(date +%s)
      echo "ToolSidecar cleaned up in $((END - START)) seconds"
      exit 0
    fi
    sleep 1
  done
  echo "FAILED: ToolSidecar not cleaned up within 60 seconds"
  ```

---

## Success Criteria

- [ ] `go test ./pkg/mcp/...` passes all tests
- [ ] Integration tests complete with evidence logged
