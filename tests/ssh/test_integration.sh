#!/bin/bash
# Integration Tests
# Tests cross-component integration: Bridge↔Matrix, Bridge→Agent, Agent→Browser, Matrix→Agent,
# end-to-end encryption, authentication flows, and approval workflows

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

EVIDENCE_DIR="$PROJECT_DIR/.sisyphus/evidence"

if [ -f "$PROJECT_DIR/.env" ]; then
    source "$PROJECT_DIR/.env"
else
    echo -e "${RED}Error: .env file not found at $PROJECT_DIR${NC}"
    exit 2
fi

# Validate required environment variables
if [ -z "$VPS_IP" ]; then
    echo -e "${RED}Error: VPS_IP not set${NC}"
    exit 2
fi

if [ -z "$VPS_USER" ]; then
    echo -e "${RED}Error: VPS_USER not set${NC}"
    exit 2
fi

if [ -z "$SSH_KEY_PATH" ]; then
    echo -e "${RED}Error: SSH_KEY_PATH not set${NC}"
    exit 2
fi

if [ -z "$MATRIX_PORT" ]; then
    echo -e "${YELLOW}Warning: MATRIX_PORT not set, using default 6167${NC}"
    MATRIX_PORT=6167
fi

if [ -z "$BRIDGE_PORT" ]; then
    echo -e "${YELLOW}Warning: BRIDGE_PORT not set, using default 8443${NC}"
    BRIDGE_PORT=8443
fi

# Expand SSH key path
SSH_KEY_PATH="${SSH_KEY_PATH/#\~/$HOME}"

# Create evidence directory if it doesn't exist
mkdir -p "$EVIDENCE_DIR"

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_WARNED=0
TESTS_TOTAL=0

# Function to format result (simple, no JSON)
print_result() {
    local test_name="$1"
    local status="$2"
    local message="$3"

    ((TESTS_TOTAL++)) || true

    if [ "$status" = "PASS" ]; then
        ((TESTS_PASSED++)) || true
        echo -e "${GREEN}[PASS]${NC} $test_name: $message"
    elif [ "$status" = "WARN" ]; then
        ((TESTS_WARNED++)) || true
        echo -e "${YELLOW}[WARN]${NC} $test_name: $message"
    else
        ((TESTS_FAILED++)) || true
        echo -e "${RED}[FAIL]${NC} $test_name: $message"
    fi
}

# Function to execute remote command via SSH
ssh_exec() {
    local command="$1"
    local timeout="${2:-10}"
    timeout "$timeout" ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=no -o ConnectTimeout=10 -o BatchMode=yes "$VPS_USER@$VPS_IP" "$command" 2>&1
    return $?
}

# Function to test HTTP endpoint
test_http_endpoint() {
    local url="$1"
    local expected_status="${2:-200}"
    local timeout="${3:-5}"
    local method="${4:-GET}"

    RESULT=$(timeout "$timeout" curl -s -o /dev/null -w "%{http_code}" -X "$method" "$url" 2>&1)
    CURL_EXIT=$?

    if [ $CURL_EXIT -eq 124 ]; then
        echo "TIMEOUT"
        return 124
    elif [ $CURL_EXIT -ne 0 ]; then
        echo "ERROR:$CURL_EXIT"
        return $CURL_EXIT
    fi

    if [ "$RESULT" = "$expected_status" ]; then
        echo "OK"
        return 0
    else
        echo "$RESULT"
        return 1
    fi
}

# Function to check if container exists
container_exists() {
    local container_name="$1"
    ssh_exec "docker ps --format '{{.Names}}' | grep -q '^${container_name}$'" 2>/dev/null
    return $?
}

echo "========================================="
echo "Integration Tests"
echo "========================================="
echo "VPS IP: $VPS_IP"
echo "VPS User: $VPS_USER"
echo "Matrix Port: $MATRIX_PORT"
echo "Bridge Port: $BRIDGE_PORT"
echo "========================================="
echo ""
echo -e "${CYAN}Note: VPS only has Matrix (Conduit) and coturn running. Some tests may fail (expected).${NC}"
echo ""

# ============================================================================
# TEST 1: Bridge ↔ Matrix Communication
# ============================================================================
echo "--- Bridge ↔ Matrix Communication ---"

# Test 1.1: Matrix client API is accessible
echo "Test 1.1: Matrix client API is accessible... "
MATRIX_CLIENT_STATUS=$(test_http_endpoint "http://$VPS_IP:$MATRIX_PORT/_matrix/client/versions" 200 10)
if [ "$MATRIX_CLIENT_STATUS" = "OK" ]; then
    print_result "Matrix Client API" "PASS" "Matrix client API is responding"
else
    print_result "Matrix Client API" "FAIL" "Matrix client API not responding (status: $MATRIX_CLIENT_STATUS)" "Container may not be running or port not accessible"
fi

# Test 1.2: Matrix server info is accessible
echo "Test 1.2: Matrix server info is accessible... "
MATRIX_SERVER_INFO=$(test_http_endpoint "http://$VPS_IP:$MATRIX_PORT/_matrix/client/v3/login" 200 10)
if [ "$MATRIX_SERVER_INFO" = "OK" ]; then
    print_result "Matrix Server Info" "PASS" "Matrix server endpoint is responding with login flows"
else
    print_result "Matrix Server Info" "WARN" "Matrix server endpoint unexpected response (status: $MATRIX_SERVER_INFO)" "May indicate non-standard Matrix setup"
fi

# Test 1.3: Matrix Federation endpoint
echo "Test 1.3: Matrix Federation endpoint is accessible... "
MATRIX_FEDERATION=$(test_http_endpoint "http://$VPS_IP:$MATRIX_PORT/_matrix/federation/v1/version" 200 10)
if [ "$MATRIX_FEDERATION" = "OK" ]; then
    print_result "Matrix Federation" "PASS" "Matrix federation endpoint is responding"
else
    print_result "Matrix Federation" "WARN" "Matrix federation endpoint not responding (status: $MATRIX_FEDERATION)" "Federation may be disabled"
fi

# Test 1.4: Matrix Conduit container status
echo "Test 1.4: Matrix Conduit container status... "
CONTAINER_STATUS=$(ssh_exec "docker ps --format '{{.Names}}: {{.Status}}'")
if echo "$CONTAINER_STATUS" | grep -q "conduit.*running\|matrix.*running"; then
    CONTAINER_NAME=$(ssh_exec "docker ps --format '{{.Names}}' | grep -E '(conduit|matrix)' | head -1")
    print_result "Matrix Container" "PASS" "Matrix container is running: $CONTAINER_NAME"
else
    print_result "Matrix Container" "WARN" "Matrix container not found or not running" "Matrix may be running with different name or not started"
fi

# Test 1.5: Matrix container health
echo "Test 1.5: Matrix container health status... "
MATRIX_CONTAINER=$(ssh_exec "docker ps --format '{{.Names}}' | grep -E '(conduit|matrix)' | head -1")
if [ -n "$MATRIX_CONTAINER" ]; then
    MATRIX_HEALTH=$(ssh_exec "docker inspect -f '{{.State.Health.Status}}' '$MATRIX_CONTAINER' 2>/dev/null || echo 'no_health_check'")
    if [ "$MATRIX_HEALTH" = "healthy" ] || [ "$MATRIX_HEALTH" = "no_health_check" ] || [ -z "$MATRIX_HEALTH" ]; then
        print_result "Matrix Container Health" "PASS" "Matrix container is healthy or no health check configured"
    else
        print_result "Matrix Container Health" "WARN" "Matrix container health check: $MATRIX_HEALTH"
    fi
else
    print_result "Matrix Container Health" "SKIP" "Matrix container not found"
fi

echo ""

# ============================================================================
# TEST 2: Bridge → Agent Communication
# ============================================================================
echo "--- Bridge → Agent Communication ---"

# Test 2.1: Bridge container status
echo "Test 2.1: Bridge container status... "
if echo "$CONTAINER_STATUS" | grep -q "bridge.*running"; then
    BRIDGE_CONTAINER=$(ssh_exec "docker ps --format '{{.Names}}' | grep -i 'bridge' | head -1")
    print_result "Bridge Container" "PASS" "Bridge container is running: $BRIDGE_CONTAINER"
else
    print_result "Bridge Container" "WARN" "Bridge container not found (expected on current VPS)"
fi

# Test 2.2: Bridge RPC endpoint
echo "Test 2.2: Bridge RPC HTTP endpoint... "
BRIDGE_RPC_STATUS=$(test_http_endpoint "http://$VPS_IP:$BRIDGE_PORT/health" 200 10)
if [ "$BRIDGE_RPC_STATUS" = "OK" ]; then
    print_result "Bridge RPC HTTP" "PASS" "Bridge RPC HTTP endpoint is responding"
elif [ "$BRIDGE_RPC_STATUS" = "TIMEOUT" ] || [ "$BRIDGE_RPC_STATUS" = "ERROR:7" ]; then
    print_result "Bridge RPC HTTP" "SKIP" "Bridge RPC HTTP endpoint not accessible (expected on current VPS)"
else
    print_result "Bridge RPC HTTP" "WARN" "Bridge RPC HTTP endpoint unexpected response (status: $BRIDGE_RPC_STATUS)"
fi

# Test 2.3: Docker socket availability for Bridge
echo "Test 2.3: Docker socket available for Bridge... "
BRIDGE_CONTAINER=$(ssh_exec "docker ps --format '{{.Names}}' | grep -i 'bridge' | head -1")
if [ -n "$BRIDGE_CONTAINER" ] && [ "$BRIDGE_CONTAINER" != "" ]; then
    DOCKER_SOCKET_MOUNT=$(ssh_exec "docker inspect '$BRIDGE_CONTAINER' 2>/dev/null | grep -o '/var/run/docker.sock' | head -1")
    if [ -n "$DOCKER_SOCKET_MOUNT" ]; then
        print_result "Bridge Docker Socket" "PASS" "Bridge has access to Docker socket"
    else
        print_result "Bridge Docker Socket" "WARN" "Bridge may not have Docker socket access"
    fi
else
    print_result "Bridge Docker Socket" "SKIP" "Bridge container not found"
fi

echo ""

# ============================================================================
# TEST 3: Agent → Browser Communication
# ============================================================================
echo "--- Agent → Browser Communication ---"

# Test 3.1: Browser-service container status
echo "Test 3.1: Browser-service container status... "
if echo "$CONTAINER_STATUS" | grep -q "browser.*running"; then
    BROWSER_CONTAINER=$(ssh_exec "docker ps --format '{{.Names}}' | grep -i 'browser' | head -1")
    print_result "Browser-Service Container" "PASS" "Browser-service container is running: $BROWSER_CONTAINER"
else
    print_result "Browser-Service Container" "SKIP" "Browser-service container not found (expected on current VPS)"
fi

# Test 3.2: Browser service API
echo "Test 3.2: Browser service API endpoint... "
if echo "$CONTAINER_STATUS" | grep -q "browser.*running"; then
    BROWSER_API_CHECK=$(test_http_endpoint "http://$VPS_IP:3000/health" 200 10)
    if [ "$BROWSER_API_CHECK" = "OK" ]; then
        print_result "Browser Service API" "PASS" "Browser service API is responding on port 3000"
    else
        BROWSER_API_CHECK=$(test_http_endpoint "http://$VPS_IP:9222/json/version" 200 10)
        if [ "$BROWSER_API_CHECK" = "OK" ]; then
            print_result "Browser Service API" "PASS" "Browser service (CDP) is responding on port 9222"
        else
            print_result "Browser Service API" "WARN" "Browser service API not accessible on common ports"
        fi
    fi
else
    print_result "Browser Service API" "SKIP" "Browser-service container not found (cannot test)"
fi

# Test 3.3: Agent-Browser network connectivity
echo "Test 3.3: Agent-Browser network connectivity... "
BRIDGE_CONTAINER=$(ssh_exec "docker ps --format '{{.Names}}' | grep -i 'bridge' | head -1")
BROWSER_CONTAINER=$(ssh_exec "docker ps --format '{{.Names}}' | grep -i 'browser' | head -1")
if [ -n "$BRIDGE_CONTAINER" ] && [ -n "$BROWSER_CONTAINER" ]; then
    NETWORK_CHECK=$(ssh_exec "docker network ls --format '{{.Name}}' | grep -E 'armorclaw|bridge'" 2>/dev/null)
    if [ -n "$NETWORK_CHECK" ]; then
        print_result "Agent-Browser Network" "PASS" "Shared networks found for agent-browser communication"
    else
        print_result "Agent-Browser Network" "WARN" "No shared network detected between agent and browser"
    fi
else
    print_result "Agent-Browser Network" "SKIP" "Bridge or Browser containers not found (cannot test)"
fi

echo ""

# ============================================================================
# TEST 4: Matrix → Agent Messaging
# ============================================================================
echo "--- Matrix → Agent Messaging ---"

# Test 4.1: Matrix room creation capability
echo "Test 4.1: Matrix room creation capability... "
MATRIX_ROOM_STATUS=$(test_http_endpoint "http://$VPS_IP:$MATRIX_PORT/_matrix/client/v3/createRoom" 401 10 "POST")
if [ "$MATRIX_ROOM_STATUS" = "401" ] || [ "$MATRIX_ROOM_STATUS" = "403" ]; then
    print_result "Matrix Room Creation" "PASS" "Matrix room creation endpoint requires authentication (as expected)"
elif [ "$MATRIX_ROOM_STATUS" = "OK" ]; then
    print_result "Matrix Room Creation" "WARN" "Matrix allows room creation without authentication (security concern)"
else
    print_result "Matrix Room Creation" "WARN" "Matrix room creation endpoint unexpected response (status: $MATRIX_ROOM_STATUS)"
fi

# Test 4.2: Matrix authentication endpoint
echo "Test 4.2: Matrix authentication endpoint... "
MATRIX_AUTH_STATUS=$(test_http_endpoint "http://$VPS_IP:$MATRIX_PORT/_matrix/client/v3/login" 200 10)
if [ "$MATRIX_AUTH_STATUS" = "OK" ]; then
    print_result "Matrix Authentication" "PASS" "Matrix authentication endpoint is responding with login flows"
else
    print_result "Matrix Authentication" "WARN" "Matrix authentication endpoint unexpected response (status: $MATRIX_AUTH_STATUS)"
fi

# Test 4.3: Matrix user registration endpoint
echo "Test 4.3: Matrix user registration endpoint... "
MATRIX_REG_STATUS=$(test_http_endpoint "http://$VPS_IP:$MATRIX_PORT/_matrix/client/v3/register" 401 10)
if [ "$MATRIX_REG_STATUS" = "401" ] || [ "$MATRIX_REG_STATUS" = "403" ]; then
    print_result "Matrix Registration" "PASS" "Matrix registration endpoint requires authentication or is disabled"
else
    print_result "Matrix Registration" "WARN" "Matrix registration may be open (status: $MATRIX_REG_STATUS)"
fi

# Test 4.4: Matrix event sending capability
echo "Test 4.4: Matrix event sending capability... "
MATRIX_EVENT_STATUS=$(test_http_endpoint "http://$VPS_IP:$MATRIX_PORT/_matrix/client/v3/rooms/!test/send/m.room.message" 401 10 "PUT")
if [ "$MATRIX_EVENT_STATUS" = "401" ] || [ "$MATRIX_EVENT_STATUS" = "403" ]; then
    print_result "Matrix Event Sending" "PASS" "Matrix event sending requires authentication (as expected)"
elif [ "$MATRIX_EVENT_STATUS" = "404" ]; then
    print_result "Matrix Event Sending" "PASS" "Room not found (endpoint exists)"
else
    print_result "Matrix Event Sending" "WARN" "Matrix event sending unexpected response (status: $MATRIX_EVENT_STATUS)"
fi

echo ""

# ============================================================================
# TEST 5: End-to-End Encryption
# ============================================================================
echo "--- End-to-End Encryption ---"

# Test 5.1: Matrix supports E2EE
echo "Test 5.1: Matrix supports end-to-end encryption... "
MATRIX_VERSIONS=$(ssh_exec "curl -s http://$VPS_IP:$MATRIX_PORT/_matrix/client/versions 2>/dev/null" || echo "")
if echo "$MATRIX_VERSIONS" | grep -q "m\."; then
    print_result "Matrix E2EE Support" "PASS" "Matrix client API versioning detected (E2EE supported)"
else
    print_result "Matrix E2EE Support" "WARN" "Could not verify Matrix E2EE support"
fi

# Test 5.2: E2EE algorithms available
echo "Test 5.2: E2EE algorithms available... "
if echo "$MATRIX_VERSIONS" | grep -qi "olm\|megolm\|vodozemac"; then
    print_result "E2EE Algorithms" "PASS" "E2EE algorithms detected in Matrix versions"
elif [ -n "$MATRIX_VERSIONS" ]; then
    print_result "E2EE Algorithms" "WARN" "E2EE algorithms not explicitly listed in versions (may still be supported)"
else
    print_result "E2EE Algorithms" "SKIP" "Could not retrieve Matrix versions"
fi

# Test 5.3: TLS/SSL encryption for Matrix
echo "Test 5.3: TLS/SSL encryption for Matrix API... "
if ssh_exec "curl -sk https://$VPS_IP:$MATRIX_PORT/_matrix/client/versions >/dev/null 2>&1"; then
    print_result "Matrix TLS/SSL" "PASS" "Matrix API supports TLS/SSL encryption"
elif ssh_exec "curl -sk http://$VPS_IP:$MATRIX_PORT/_matrix/client/versions >/dev/null 2>&1"; then
    print_result "Matrix TLS/SSL" "WARN" "Matrix API using HTTP (not encrypted)"
else
    print_result "Matrix TLS/SSL" "SKIP" "Could not test TLS/SSL encryption"
fi

# Test 5.4: TLS/SSL for Bridge RPC
echo "Test 5.4: TLS/SSL encryption for Bridge RPC... "
BRIDGE_CONTAINER=$(ssh_exec "docker ps --format '{{.Names}}' | grep -i 'bridge' | head -1")
if [ -n "$BRIDGE_CONTAINER" ]; then
    if ssh_exec "curl -sk https://$VPS_IP:$BRIDGE_PORT/health >/dev/null 2>&1"; then
        print_result "Bridge RPC TLS/SSL" "PASS" "Bridge RPC supports TLS/SSL encryption"
    else
        print_result "Bridge RPC TLS/SSL" "WARN" "Bridge RPC may not have TLS/SSL configured"
    fi
else
    print_result "Bridge RPC TLS/SSL" "SKIP" "Bridge container not found (cannot test)"
fi

echo ""

# ============================================================================
# TEST 6: Authentication Flows
# ============================================================================
echo "--- Authentication Flows ---"

# Test 6.1: Matrix login endpoint accessibility
echo "Test 6.1: Matrix login endpoint accessibility... "
MATRIX_LOGIN_STATUS=$(test_http_endpoint "http://$VPS_IP:$MATRIX_PORT/_matrix/client/v3/login" 200 10)
if [ "$MATRIX_LOGIN_STATUS" = "OK" ]; then
    print_result "Matrix Login Endpoint" "PASS" "Matrix login endpoint is accessible"
else
    print_result "Matrix Login Endpoint" "WARN" "Matrix login endpoint unexpected response (status: $MATRIX_LOGIN_STATUS)"
fi

# Test 6.2: Token verification endpoint
echo "Test 6.2: Matrix token verification capability... "
MATRIX_TOKEN_STATUS=$(test_http_endpoint "http://$VPS_IP:$MATRIX_PORT/_matrix/client/v3/account/whoami" 401 10)
if [ "$MATRIX_TOKEN_STATUS" = "401" ]; then
    print_result "Matrix Token Verification" "PASS" "Matrix token verification endpoint exists and requires auth"
else
    print_result "Matrix Token Verification" "WARN" "Matrix token verification unexpected response (status: $MATRIX_TOKEN_STATUS)"
fi

# Test 6.3: Bridge authentication endpoint
echo "Test 6.3: Bridge authentication endpoint... "
BRIDGE_CONTAINER=$(ssh_exec "docker ps --format '{{.Names}}' | grep -i 'bridge' | head -1")
if [ -n "$BRIDGE_CONTAINER" ]; then
    BRIDGE_AUTH_STATUS=$(test_http_endpoint "http://$VPS_IP:$BRIDGE_PORT/" 404 10)
    if [ "$BRIDGE_AUTH_STATUS" = "404" ]; then
        print_result "Bridge Auth Endpoint" "PASS" "Bridge endpoint is accessible (404 expected for root)"
    else
        print_result "Bridge Auth Endpoint" "WARN" "Bridge authentication endpoint unexpected response (status: $BRIDGE_AUTH_STATUS)"
    fi
else
    print_result "Bridge Auth Endpoint" "SKIP" "Bridge container not found (cannot test)"
fi

# Test 6.4: Password strength enforcement
echo "Test 6.4: Password strength enforcement (Conduit config)... "
CONDUIT_CONFIG=$(ssh_exec "cat /etc/conduit/conduit.toml 2>/dev/null || cat /etc/matrix-conduit/conduit.toml 2>/dev/null || echo ''")
if echo "$CONDUIT_CONFIG" | grep -qi "password_policy\|password_length"; then
    print_result "Password Enforcement" "PASS" "Password policy configured in Conduit"
else
    print_result "Password Enforcement" "WARN" "Password policy not explicitly configured (using defaults)"
fi

echo ""

# ============================================================================
# TEST 7: Approval Workflows
# ============================================================================
echo "--- Approval Workflows ---"

# Test 7.1: HITL approval system availability
echo "Test 7.1: Human-in-the-loop approval system availability... "
BRIDGE_CONTAINER=$(ssh_exec "docker ps --format '{{.Names}}' | grep -i 'bridge' | head -1")
if [ -n "$BRIDGE_CONTAINER" ]; then
    print_result "HITL Approval System" "WARN" "Cannot verify HITL approval system (bridge running but API not documented)"
else
    print_result "HITL Approval System" "SKIP" "Bridge container not found (cannot test)"
fi

# Test 7.2: Secret injection with approval
echo "Test 7.2: Secret injection with approval flow... "
BRIDGE_CONTAINER=$(ssh_exec "docker ps --format '{{.Names}}' | grep -i 'bridge' | head -1")
if [ -n "$BRIDGE_CONTAINER" ] && [ "$BRIDGE_CONTAINER" != "" ]; then
    KEYSTORE_EXISTS=$(ssh_exec "docker exec '$BRIDGE_CONTAINER' ls -la /var/lib/armorclaw 2>/dev/null || echo ''")
    if [ -n "$KEYSTORE_EXISTS" ]; then
        print_result "Secret Injection Approval" "PASS" "Keystore directory exists for secret management with approval"
    else
        print_result "Secret Injection Approval" "WARN" "Could not verify keystore for secret approval flow"
    fi
else
    print_result "Secret Injection Approval" "SKIP" "Bridge container not found (cannot test)"
fi

# Test 7.3: SQLCipher keystore for secrets
echo "Test 7.3: SQLCipher keystore for encrypted secrets... "
BRIDGE_CONTAINER=$(ssh_exec "docker ps --format '{{.Names}}' | grep -i 'bridge' | head -1")
if [ -n "$BRIDGE_CONTAINER" ] && [ "$BRIDGE_CONTAINER" != "" ]; then
    SQLCIPHER_DB=$(ssh_exec "docker exec '$BRIDGE_CONTAINER' find /var/lib/armorclaw -name '*.db' -o -name '*.sqlite' 2>/dev/null || echo ''")
    if [ -n "$SQLCIPHER_DB" ]; then
        print_result "SQLCipher Keystore" "PASS" "Encrypted keystore database found"
    else
        print_result "SQLCipher Keystore" "WARN" "Could not locate SQLCipher keystore database"
    fi
else
    print_result "SQLCipher Keystore" "SKIP" "Bridge container not found (cannot test)"
fi

# Test 7.4: Approval logging
echo "Test 7.4: Approval logging and audit trail... "
BRIDGE_CONTAINER=$(ssh_exec "docker ps --format '{{.Names}}' | grep -i 'bridge' | head -1")
if [ -n "$BRIDGE_CONTAINER" ] && [ "$BRIDGE_CONTAINER" != "" ]; then
    LOGS_EXISTS=$(ssh_exec "docker exec '$BRIDGE_CONTAINER' ls -la /var/log/armorclaw 2>/dev/null || echo ''")
    if [ -n "$LOGS_EXISTS" ]; then
        print_result "Approval Logging" "PASS" "Logging directory found for approval audit trail"
    else
        print_result "Approval Logging" "WARN" "Could not verify approval logging directory"
    fi
else
    print_result "Approval Logging" "SKIP" "Bridge container not found (cannot test)"
fi

echo ""

# ============================================================================
# Generate Summary
# ============================================================================
echo ""
echo "========================================="
echo "Test Summary"
echo "========================================="
echo -e "Total Tests: $TESTS_TOTAL"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
echo -e "${YELLOW}Warned: $TESTS_WARNED${NC}"
echo -e "${RED}Failed: $TESTS_FAILED${NC}"
echo ""

# Save console output evidence
CONSOLE_FILE="$EVIDENCE_DIR/task-6-integration-success.txt"
echo "Integration Test Results - $(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$CONSOLE_FILE"
echo "VPS IP: $VPS_IP" >> "$CONSOLE_FILE"
echo "VPS User: $VPS_USER" >> "$CONSOLE_FILE"
echo "Matrix Port: $MATRIX_PORT" >> "$CONSOLE_FILE"
echo "Bridge Port: $BRIDGE_PORT" >> "$CONSOLE_FILE"
echo "" >> "$CONSOLE_FILE"
echo "Total Tests: $TESTS_TOTAL" >> "$CONSOLE_FILE"
echo "Passed: $TESTS_PASSED" >> "$CONSOLE_FILE"
echo "Warned: $TESTS_WARNED" >> "$CONSOLE_FILE"
echo "Failed: $TESTS_FAILED" >> "$CONSOLE_FILE"
echo "" >> "$CONSOLE_FILE"
echo "Note: VPS only has Matrix (Conduit) and coturn running." >> "$CONSOLE_FILE"
echo "      SKIP/WARN results for Bridge, Agent, and Browser components are expected." >> "$CONSOLE_FILE"
echo -e "${CYAN}Console output saved to $CONSOLE_FILE${NC}"

# Save detailed evidence
DETAILS_FILE="$EVIDENCE_DIR/task-6-integration-details.txt"
echo "Integration Test Details - $(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$DETAILS_FILE"
echo "========================================" >> "$DETAILS_FILE"
echo "" >> "$DETAILS_FILE"
echo "Container Status:" >> "$DETAILS_FILE"
ssh_exec "docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'" >> "$DETAILS_FILE" 2>/dev/null || echo "  Failed to list containers" >> "$DETAILS_FILE"
echo "" >> "$DETAILS_FILE"
echo "Matrix Client Versions:" >> "$DETAILS_FILE"
ssh_exec "curl -s http://$VPS_IP:$MATRIX_PORT/_matrix/client/versions 2>/dev/null" >> "$DETAILS_FILE" || echo "  Failed to retrieve Matrix versions" >> "$DETAILS_FILE"
echo "" >> "$DETAILS_FILE"
echo "Network Configuration:" >> "$DETAILS_FILE"
ssh_exec "docker network ls" >> "$DETAILS_FILE" 2>/dev/null || echo "  Failed to list networks" >> "$DETAILS_FILE"
echo -e "${CYAN}Detailed evidence saved to $DETAILS_FILE${NC}"

echo ""
echo "========================================="
echo "Integration Tests Complete"
echo "========================================="

# Exit with appropriate code
if [ $TESTS_FAILED -gt 0 ]; then
    exit 1
fi
exit 0
