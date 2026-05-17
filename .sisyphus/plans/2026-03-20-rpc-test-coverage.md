# RPC Test Coverage Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add comprehensive RPC test coverage with CI integration to catch issues before Docker Hub builds

**Architecture:** Hybrid testing approach with unit tests (Go) + integration tests (bash/socat) running in GitHub Actions before Docker builds. Tests validate method registration, Matrix v3 login format, and socket communication.

**Tech Stack:** Go 1.24, testing package, socat for socket communication, GitHub Actions

---

## File Structure

```
bridge/pkg/rpc/
├── server.go                  # Existing - handlers map
├── server_test.go             # NEW - method registration coverage
├── matrix_handler_test.go     # NEW - Matrix v3 login tests

tests/
├── test-rpc-methods.sh        # NEW - integration tests via socket

.github/workflows/
└── dockerhub.yml              # MODIFY - add test jobs before build
```

---

## Chunk 1: Method Registration Unit Tests

### Task 1: Create Method Registration Test

**Files:**
- Create: `bridge/pkg/rpc/server_test.go`

- [ ] **Step 1: Write the failing test**

```go
package rpc

import (
	"testing"
)

// TestMethodRegistration verifies all expected RPC methods are registered
func TestMethodRegistration(t *testing.T) {
	tests := []struct {
		method string
	}{
		{"ping"},
		{"status"},
		{"matrix.status"},
		{"matrix.login"},
		{"matrix.send"},
		{"ai.chat"},
		{"browser.navigate"},
		{"browser.fill"},
		{"browser.click"},
		{"browser.status"},
	}

	server := NewServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if _, exists := server.handlers[tt.method]; !exists {
				t.Errorf("method %q not registered in handlers map", tt.method)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd bridge && go test -v -run TestMethodRegistration ./pkg/rpc/`
Expected: FAIL (file doesn't exist yet)

- [ ] **Step 3: Verify handlers map is exported**

Check if `handlers` field is accessible. If not, add a getter method to `server.go`:

```go
// GetRegisteredMethods returns all registered method names
func (s *Server) GetRegisteredMethods() []string {
	methods := make([]string, 0, len(s.handlers))
	for method := range s.handlers {
		methods = append(methods, method)
	}
	return methods
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd bridge && go test -v -run TestMethodRegistration ./pkg/rpc/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add bridge/pkg/rpc/server_test.go
git commit -m "test(rpc): add method registration coverage test"
```

---

## Chunk 2: Matrix Handler Unit Tests

### Task 2: Create Matrix Handler Tests

**Files:**
- Create: `bridge/pkg/rpc/matrix_handler_test.go`

- [ ] **Step 1: Write the failing test for matrix.status**

```go
package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/armorclaw/bridge/internal/adapter"
)

// mockMatrixAdapter implements MatrixAdapter for testing
type mockMatrixAdapter struct {
	enabled   bool
	connected bool
	loggedIn  bool
}

func (m *mockMatrixAdapter) IsEnabled() bool         { return m.enabled }
func (m *mockMatrixAdapter) IsConnected() bool       { return m.connected }
func (m *mockMatrixAdapter) IsLoggedIn() bool        { return m.loggedIn }
func (m *mockMatrixAdapter) Login(user, pass string) error { return nil }
func (m *mockMatrixAdapter) StartSync()              {}
func (m *mockMatrixAdapter) StopSync()               {}

func TestHandleMatrixStatus(t *testing.T) {
	tests := []struct {
		name     string
		adapter  *mockMatrixAdapter
		expected MatrixStatusResult
	}{
		{
			name: "logged in and connected",
			adapter: &mockMatrixAdapter{
				enabled:   true,
				connected: true,
				loggedIn:  true,
			},
			expected: MatrixStatusResult{
				Enabled:   true,
				Connected: true,
				LoggedIn:  true,
			},
		},
		{
			name: "connected but not logged in",
			adapter: &mockMatrixAdapter{
				enabled:   true,
				connected: true,
				loggedIn:  false,
			},
			expected: MatrixStatusResult{
				Enabled:   true,
				Connected: true,
				LoggedIn:  false,
			},
		},
		{
			name: "disabled",
			adapter: &mockMatrixAdapter{
				enabled: false,
			},
			expected: MatrixStatusResult{
				Enabled: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &Server{matrix: tt.adapter}
			
			req := &Request{
				Version: "2.0",
				Method:  "matrix.status",
				Params:  json.RawMessage("{}"),
			}

			result, err := server.handleMatrixStatus(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			status, ok := result.(MatrixStatusResult)
			if !ok {
				t.Fatalf("expected MatrixStatusResult, got %T", result)
			}

			if status.Enabled != tt.expected.Enabled {
				t.Errorf("Enabled: got %v, want %v", status.Enabled, tt.expected.Enabled)
			}
			if status.Connected != tt.expected.Connected {
				t.Errorf("Connected: got %v, want %v", status.Connected, tt.expected.Connected)
			}
			if status.LoggedIn != tt.expected.LoggedIn {
				t.Errorf("LoggedIn: got %v, want %v", status.LoggedIn, tt.expected.LoggedIn)
			}
		})
	}
}

// MatrixStatusResult matches the response structure
type MatrixStatusResult struct {
	Enabled   bool   `json:"enabled"`
	Connected bool   `json:"connected"`
	LoggedIn  bool   `json:"logged_in"`
	Homeserver string `json:"homeserver,omitempty"`
	Error     string `json:"error,omitempty"`
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd bridge && go test -v -run TestHandleMatrixStatus ./pkg/rpc/`
Expected: FAIL (file doesn't exist yet)

- [ ] **Step 3: Verify handleMatrixStatus exists and is exported**

If the handler is not exported, you may need to test through the public interface or export it.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd bridge && go test -v -run TestHandleMatrixStatus ./pkg/rpc/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add bridge/pkg/rpc/matrix_handler_test.go
git commit -m "test(rpc): add matrix.status handler unit tests"
```

---

## Chunk 3: Integration Test Script

### Task 3: Create Socket-Based Integration Tests

**Files:**
- Create: `tests/test-rpc-methods.sh`

- [ ] **Step 1: Create the integration test script**

```bash
#!/bin/bash
# RPC Integration Tests - Tests actual socket communication
# Run: ./tests/test-rpc-methods.sh [bridge-binary-path]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BRIDGE_BIN="${1:-${SCRIPT_DIR}/../bridge/build/armorclaw-bridge}"
SOCKET_PATH="/tmp/bridge-test-$$.sock"
BRIDGE_PID=""
TEST_NAME="rpc-integration"

# Cleanup function
cleanup() {
    if [[ -n "$BRIDGE_PID" ]]; then
        kill "$BRIDGE_PID" 2>/dev/null || true
        wait "$BRIDGE_PID" 2>/dev/null || true
    fi
    rm -f "$SOCKET_PATH" 2>/dev/null || true
}
trap cleanup EXIT

# Check dependencies
if ! command -v socat &>/dev/null; then
    echo "❌ socat not installed. Install with: apt-get install socat"
    exit 1
fi

# Build bridge binary if needed
if [[ ! -x "$BRIDGE_BIN" ]]; then
    echo "Building bridge binary..."
    cd "${SCRIPT_DIR}/../bridge"
    go build -o build/armorclaw-bridge ./cmd/bridge
    BRIDGE_BIN="build/armorclaw-bridge"
fi

# Start bridge in background
echo "Starting bridge..."
ARMORCLAW_SOCKET_PATH="$SOCKET_PATH" \
    "$BRIDGE_BIN" &
BRIDGE_PID=$!

# Wait for socket with timeout
echo "Waiting for socket..."
for i in {1..30}; do
    if [[ -S "$SOCKET_PATH" ]]; then
        break
    fi
    sleep 0.5
done

if [[ ! -S "$SOCKET_PATH" ]]; then
    echo "❌ FAILED: Socket not created after 15 seconds"
    exit 1
fi
echo "✅ Socket created at $SOCKET_PATH"

# RPC call function
rpc_call() {
    local method="$1"
    local params="${2:-{}}"
    local timeout="${3:-5}"
    
    timeout "$timeout" bash -c \
        "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$method\",\"params\":$params}' | \
         socat - UNIX-CONNECT:$SOCKET_PATH" 2>/dev/null
}

# Test function
test_method() {
    local method="$1"
    local expected="$2"
    local params="${3:-{}}"
    
    result=$(rpc_call "$method" "$params")
    
    if echo "$result" | grep -q "$expected"; then
        echo "✅ PASSED: $method"
        return 0
    else
        echo "❌ FAILED: $method - expected '$expected' in response"
        echo "   Got: $result"
        return 1
    fi
}

# Critical RPC methods to test
# Format: "method" "expected_in_response" "params"
CRITICAL_METHODS=(
    "ping"
    "status"
    "matrix.status"
)

echo ""
echo "Testing critical RPC methods..."
echo ""

FAILED=0
for method in "${CRITICAL_METHODS[@]}"; do
    if ! test_method "$method" '"result"'; then
        ((FAILED++)) || true
    fi
done

# Test error handling
echo ""
echo "Testing error handling..."

# Invalid method should return error code -32601
if ! test_method "invalid.method" '"code":-32601'; then
    ((FAILED++)) || true
fi

# Summary
echo ""
echo "========================================"
if [[ $FAILED -eq 0 ]]; then
    echo "🎉 All integration tests passed ($(( ${#CRITICAL_METHODS[@]} + 1 )) tests)"
    exit 0
else
    echo "❌ $FAILED test(s) failed"
    exit 1
fi
```

- [ ] **Step 2: Make script executable**

Run: `chmod +x tests/test-rpc-methods.sh`

- [ ] **Step 3: Run the test locally**

Run: `./tests/test-rpc-methods.sh`
Expected: PASS (all critical methods respond)

- [ ] **Step 4: Commit**

```bash
git add tests/test-rpc-methods.sh
git commit -m "test(rpc): add socket-based integration tests"
```

---

## Chunk 4: CI Workflow Updates

### Task 4: Add Test Jobs to GitHub Actions

**Files:**
- Modify: `.github/workflows/dockerhub.yml`

- [ ] **Step 1: Add precheck job**

Insert after the `env` section, before `build-push-docker`:

```yaml
  precheck:
    name: Pre-check (vet + compile)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: actions/setup-go@v5
        with:
          go-version-file: 'bridge/go.mod'
          cache: true
      
      - name: Go vet
        run: cd bridge && go vet ./pkg/rpc/... ./internal/adapter/...
      
      - name: Compile bridge
        run: cd bridge && go build -o /dev/null ./cmd/bridge

  rpc-unit-tests:
    name: RPC Unit Tests
    needs: precheck
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: actions/setup-go@v5
        with:
          go-version-file: 'bridge/go.mod'
          cache: true
      
      - name: Run RPC unit tests
        run: |
          cd bridge
          go test -v -race -coverprofile=coverage.out ./pkg/rpc/...
          go tool cover -func=coverage.out | grep total
      
      - name: Upload coverage
        uses: actions/upload-artifact@v4
        with:
          name: rpc-coverage
          path: bridge/coverage.out

  rpc-integ-tests:
    name: RPC Integration Tests
    needs: rpc-unit-tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: actions/setup-go@v5
        with:
          go-version-file: 'bridge/go.mod'
          cache: true
      
      - name: Install socat
        run: sudo apt-get install -y socat
      
      - name: Run integration tests
        run: |
          chmod +x tests/test-rpc-methods.sh
          ./tests/test-rpc-methods.sh
      
      - name: Upload logs on failure
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: rpc-integ-logs
          path: /tmp/bridge-test-*.log
          if-no-files-found: ignore
```

- [ ] **Step 2: Add needs dependencies to build-push-docker**

Modify the `build-push-docker` job to depend on tests:

```yaml
  build-push-docker:
    name: Build & Push Docker Image
    needs: [rpc-unit-tests, rpc-integ-tests]
    runs-on: ubuntu-latest
    # ... rest of job unchanged
```

- [ ] **Step 3: Separate build from push**

Rename the existing `build-push-docker` to `docker-build` and add a separate `docker-push` job:

```yaml
  docker-build:
    name: Build Docker Image
    needs: [rpc-unit-tests, rpc-integ-tests]
    runs-on: ubuntu-latest
    timeout-minutes: 30
    # ... existing build steps but WITHOUT push
    steps:
      # ... existing checkout, setup, login, metadata steps ...
      
      - name: Build Docker image (no push)
        uses: docker/build-push-action@v5
        with:
          context: .
          file: ./Dockerfile.quickstart
          push: false
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          platforms: ${{ env.PLATFORMS }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
          build-args: |
            BUILD_DATE=${{ github.event.head_commit.timestamp }}
            VCS_REF=${{ github.sha }}

  docker-smoke:
    name: Docker Smoke Test
    needs: docker-build
    runs-on: ubuntu-latest
    if: github.event_name != 'pull_request'
    steps:
      - name: Pull and test
        run: |
          docker pull ${{ env.DOCKER_IMAGE }}:latest
          docker run --rm ${{ env.DOCKER_IMAGE }}:latest --help
          docker run --rm ${{ env.DOCKER_IMAGE }}:latest /opt/armorclaw/armorclaw-bridge --version

  docker-push:
    name: Push to Docker Hub
    needs: docker-smoke
    runs-on: ubuntu-latest
    if: github.event_name != 'pull_request'
    steps:
      - uses: actions/checkout@v4
      
      - name: Log in to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKER_USERNAME }}
          password: ${{ secrets.DOCKER_PASSWORD }}
      
      - name: Push image
        uses: docker/build-push-action@v5
        with:
          context: .
          file: ./Dockerfile.quickstart
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          platforms: ${{ env.PLATFORMS }}
```

- [ ] **Step 4: Verify workflow syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/dockerhub.yml'))"`
Expected: No output (valid YAML)

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/dockerhub.yml
git commit -m "ci: add RPC test jobs before Docker build"
```

---

## Chunk 5: Documentation

### Task 5: Update README with Test Information

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add testing section to README**

Add after the "Testing" section in README.md:

```markdown
### RPC Test Coverage

The bridge includes comprehensive RPC tests:

**Unit Tests** (`bridge/pkg/rpc/*_test.go`):
- Method registration coverage (catches missing handlers)
- Matrix v3 login format validation
- Handler response structure tests

**Integration Tests** (`tests/test-rpc-methods.sh`):
- Socket communication validation
- Critical method availability
- Error handling verification

Run locally:
\`\`\`bash
# Unit tests
cd bridge && go test -v ./pkg/rpc/...

# Integration tests
./tests/test-rpc-methods.sh
\`\`\`

CI Pipeline:
\`\`\`
precheck → rpc-unit-tests → rpc-integ-tests → docker-build → docker-smoke → docker-push
\`\`\`
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add RPC testing documentation"
```

---

## Final Verification

- [ ] **Step 1: Run all tests locally**

```bash
cd bridge && go test -v ./pkg/rpc/...
./tests/test-rpc-methods.sh
```

- [ ] **Step 2: Verify CI workflow**

Push to a test branch and verify all jobs run in order:
1. precheck passes
2. rpc-unit-tests passes
3. rpc-integ-tests passes
4. docker-build runs
5. docker-smoke runs (if not PR)
6. docker-push runs (if not PR)

---

## Success Criteria

- [ ] All RPC handlers have corresponding tests
- [ ] Method registration test catches missing `handlers[method]` entries
- [ ] Integration tests validate socket communication
- [ ] CI blocks Docker builds on test failures
- [ ] Test failures include logs/artifacts for debugging
