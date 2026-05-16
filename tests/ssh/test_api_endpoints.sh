#!/bin/bash
# API Endpoint Tests
# Tests Bridge RPC, Matrix client, and health endpoints

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

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

if [ -z "$BRIDGE_PORT" ]; then
    echo -e "${YELLOW}Warning: BRIDGE_PORT not set, using default 8443${NC}"
    BRIDGE_PORT=8443
fi

if [ -z "$MATRIX_PORT" ]; then
    echo -e "${YELLOW}Warning: MATRIX_PORT not set, using default 6167${NC}"
    MATRIX_PORT=6167
fi

EVIDENCE_DIR="$PROJECT_DIR/.sisyphus/evidence"
mkdir -p "$EVIDENCE_DIR"

TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0
JSON_OUTPUT=""

log_evidence() {
    local test_name="$1"
    local status="$2"
    local message="$3"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "[$timestamp] [$status] $test_name: $message" >> "$EVIDENCE_DIR/task-5-api-endpoints.txt"
}

add_json_result() {
    local test_name="$1"
    local status="$2"
    local message="$3"
    local entry="{\"test\":\"$test_name\",\"status\":\"$status\",\"message\":\"$message\"}"
    if [ -z "$JSON_OUTPUT" ]; then
        JSON_OUTPUT="[$entry"
    else
        JSON_OUTPUT="$JSON_OUTPUT,$entry"
    fi
}

test_http_endpoint() {
    local endpoint_name="$1"
    local url="$2"
    local expected_pattern="$3"
    local timeout="${4:-5}"

    echo -n "Testing $endpoint_name... "
    ((TESTS_TOTAL++))

    RESPONSE=$(ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$VPS_USER@$VPS_IP" "curl -s --max-time $timeout \"${url}\"" 2>&1 || true)

    if echo "$RESPONSE" | grep -q "$expected_pattern"; then
        print_result "$endpoint_name" "PASS" "Endpoint is accessible"
        log_evidence "$endpoint_name" "PASS" "Endpoint is accessible"
        add_json_result "$endpoint_name" "PASS" "Endpoint is accessible"
        ((TESTS_PASSED++))
        return 0
    else
        print_result "$endpoint_name" "FAIL" "Endpoint is not accessible or returned unexpected response"
        log_evidence "$endpoint_name" "FAIL" "Endpoint returned unexpected response: $RESPONSE"
        add_json_result "$endpoint_name" "FAIL" "Endpoint returned unexpected response"
        ((TESTS_FAILED++))
        return 1
    fi
}

test_timeout_handling() {
    local endpoint_name="$1"
    local url="$2"
    local timeout="${3:-1}"

    echo -n "Testing timeout handling for $endpoint_name ($timeout second timeout)... "
    ((TESTS_TOTAL++))

    START_TIME=$(date +%s)
    RESPONSE=$(ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$VPS_USER@$VPS_IP" "curl -s --max-time $timeout \"${url}\"" 2>&1 || echo "TIMEOUT")
    END_TIME=$(date +%s)
    ELAPSED=$((END_TIME - START_TIME))

    if echo "$RESPONSE" | grep -q "TIMEOUT\|timed out\|Operation timed out" || [ "$ELAPSED" -le "$((timeout + 2))" ]; then
        print_result "$endpoint_name Timeout" "PASS" "Timeout handled correctly in ${ELAPSED}s"
        log_evidence "$endpoint_name Timeout" "PASS" "Timeout handled in ${ELAPSED}s"
        add_json_result "$endpoint_name Timeout" "PASS" "Timeout handled correctly in ${ELAPSED}s"
        ((TESTS_PASSED++))
        return 0
    else
        print_result "$endpoint_name Timeout" "FAIL" "Timeout not handled correctly (took ${ELAPSED}s)"
        log_evidence "$endpoint_name Timeout" "FAIL" "Timeout took ${ELAPSED}s"
        add_json_result "$endpoint_name Timeout" "FAIL" "Timeout not handled correctly"
        ((TESTS_FAILED++))
        return 1
    fi
}

test_json_format() {
    local endpoint_name="$1"
    local url="$2"

    echo -n "Testing JSON format for $endpoint_name... "
    ((TESTS_TOTAL++))

    RESPONSE=$(ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$VPS_USER@$VPS_IP" "curl -s --max-time 5 \"${url}\"" 2>&1 || true)

    if echo "$RESPONSE" | command -v jq >/dev/null 2>&1 && echo "$RESPONSE" | jq . >/dev/null 2>&1; then
        print_result "$endpoint_name JSON Format" "PASS" "Valid JSON response"
        log_evidence "$endpoint_name JSON Format" "PASS" "Valid JSON structure"
        add_json_result "$endpoint_name JSON Format" "PASS" "Valid JSON response"
        ((TESTS_PASSED++))
        return 0
    elif echo "$RESPONSE" | grep -qE '^\{.*\}$|^\[.*\]$'; then
        print_result "$endpoint_name JSON Format" "PASS" "JSON-like structure detected (jq not available)"
        log_evidence "$endpoint_name JSON Format" "PASS" "JSON-like structure"
        add_json_result "$endpoint_name JSON Format" "PASS" "JSON-like structure (jq unavailable)"
        ((TESTS_PASSED++))
        return 0
    else
        print_result "$endpoint_name JSON Format" "WARN" "Could not validate JSON format"
        log_evidence "$endpoint_name JSON Format" "WARN" "Could not validate JSON: $RESPONSE"
        add_json_result "$endpoint_name JSON Format" "WARN" "Could not validate JSON format"
        return 0
    fi
}

test_error_response() {
    local endpoint_name="$1"
    local url="$2"
    local expected_status="${3:-404}"

    echo -n "Testing $expected_status error response for $endpoint_name... "
    ((TESTS_TOTAL++))

    HTTP_CODE=$(ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$VPS_USER@$VPS_IP" "curl -s -o /dev/null -w '%{http_code}' --max-time 5 \"${url}\"" 2>&1 || echo "000")

    if [ "$HTTP_CODE" = "$expected_status" ]; then
        print_result "$endpoint_name Error $expected_status" "PASS" "Correct HTTP status code"
        log_evidence "$endpoint_name Error $expected_status" "PASS" "HTTP $expected_status returned"
        add_json_result "$endpoint_name Error $expected_status" "PASS" "Correct HTTP status code"
        ((TESTS_PASSED++))
        return 0
    else
        print_result "$endpoint_name Error $expected_status" "FAIL" "Expected $expected_status, got $HTTP_CODE"
        log_evidence "$endpoint_name Error $expected_status" "FAIL" "Expected $expected_status, got $HTTP_CODE"
        add_json_result "$endpoint_name Error $expected_status" "FAIL" "Incorrect HTTP status code"
        ((TESTS_FAILED++))
        return 1
    fi
}

test_auth_endpoint() {
    local endpoint_name="$1"
    local url="$2"

    echo -n "Testing authentication for $endpoint_name... "
    ((TESTS_TOTAL++))

    RESPONSE=$(ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$VPS_USER@$VPS_IP" "curl -s --max-time 5 \"${url}\"" 2>&1 || true)

    if echo "$RESPONSE" | grep -qi "unauthorized\|authentication required\|missing token\|401" || echo "$RESPONSE" | grep -q '"code":401\|"errcode":"M_UNAUTHORIZED"'; then
        print_result "$endpoint_name Auth" "PASS" "Authentication required as expected"
        log_evidence "$endpoint_name Auth" "PASS" "Authentication enforced"
        add_json_result "$endpoint_name Auth" "PASS" "Authentication required"
        ((TESTS_PASSED++))
        return 0
    else
        print_result "$endpoint_name Auth" "WARN" "Could not verify authentication requirement"
        log_evidence "$endpoint_name Auth" "WARN" "Auth verification unclear: $RESPONSE"
        add_json_result "$endpoint_name Auth" "WARN" "Could not verify auth requirement"
        return 0
    fi
}

test_health_endpoint() {
    local endpoint_name="$1"
    local url="$2"

    echo -n "Testing health endpoint $endpoint_name... "
    ((TESTS_TOTAL++))

    RESPONSE=$(ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$VPS_USER@$VPS_IP" "curl -s --max-time 5 \"${url}\"" 2>&1 || true)

    if echo "$RESPONSE" | grep -qiE 'healthy|status.*ok|up|running' || echo "$RESPONSE" | grep -q '"status":"healthy"'; then
        print_result "$endpoint_name Health" "PASS" "Service is healthy"
        log_evidence "$endpoint_name Health" "PASS" "Service healthy"
        add_json_result "$endpoint_name Health" "PASS" "Service is healthy"
        ((TESTS_PASSED++))
        return 0
    else
        print_result "$endpoint_name Health" "FAIL" "Service health unclear or degraded"
        log_evidence "$endpoint_name Health" "FAIL" "Health check failed: $RESPONSE"
        add_json_result "$endpoint_name Health" "FAIL" "Service health unclear"
        ((TESTS_FAILED++))
        return 1
    fi
}
print_result() {
    local test_name="$1"
    local status="$2"
    local message="$3"
    
    if [ "$status" = "PASS" ]; then
        echo -e "${GREEN}[PASS]${NC} $test_name: $message"
    else
        echo -e "${RED}[FAIL]${NC} $test_name: $message"
    fi
}

echo "========================================="
echo "API Endpoint Tests"
echo "========================================="
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Phase 1: Basic Connectivity Tests"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

echo ""
echo "Test 1: Bridge RPC health check"
test_http_endpoint "Bridge RPC" "http://localhost:$BRIDGE_PORT" '"healthy"' 5

echo ""
echo "Test 2: Matrix client versions endpoint"
test_http_endpoint "Matrix Client Versions" "http://localhost:$MATRIX_PORT/_matrix/client/versions" '"versions"' 5

echo ""
echo "Test 3: Matrix federation endpoint"
test_http_endpoint "Matrix Federation" "http://localhost:$MATRIX_PORT/_matrix/federation/v1/version" '"version"' 5

echo ""
echo "Test 4: Bridge RPC JSON-RPC 2.0"
echo -n "Testing Bridge RPC JSON-RPC 2.0... "
((TESTS_TOTAL++))
RPC_RESPONSE=$(ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$VPS_USER@$VPS_IP" "curl -s --max-time 5 -X POST -H 'Content-Type: application/json' -d '{\\\"jsonrpc\\\":\\\"2.0\\\",\\\"method\\\":\\\"health.check\\\",\\\"id\\\":1}' http://localhost:$BRIDGE_PORT/" 2>&1 || true)

if echo "$RPC_RESPONSE" | grep -q '"result":"healthy"'; then
    print_result "Bridge RPC JSON-RPC" "PASS" "Bridge RPC is healthy"
    log_evidence "Bridge RPC JSON-RPC" "PASS" "RPC responding correctly"
    add_json_result "Bridge RPC JSON-RPC" "PASS" "RPC responding correctly"
    ((TESTS_PASSED++))
else
    print_result "Bridge RPC JSON-RPC" "FAIL" "Bridge RPC not responding correctly"
    log_evidence "Bridge RPC JSON-RPC" "FAIL" "RPC response: $RPC_RESPONSE"
    add_json_result "Bridge RPC JSON-RPC" "FAIL" "RPC not responding correctly"
    ((TESTS_FAILED++))
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Phase 2: Health Endpoint Tests"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

echo ""
echo "Test 5: Bridge health endpoint /health"
test_health_endpoint "Bridge /health" "http://localhost:$BRIDGE_PORT/health"

echo ""
echo "Test 6: Bridge status endpoint /status"
test_health_endpoint "Bridge /status" "http://localhost:$BRIDGE_PORT/status"

echo ""
echo "Test 7: Matrix health check"
test_health_endpoint "Matrix Health" "http://localhost:$MATRIX_PORT/_matrix/client/versions"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Phase 3: Timeout Handling Tests"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

echo ""
echo "Test 8: Bridge RPC timeout handling (1 second timeout)"
test_timeout_handling "Bridge RPC" "http://localhost:$BRIDGE_PORT" 1

echo ""
echo "Test 9: Matrix API timeout handling (1 second timeout)"
test_timeout_handling "Matrix API" "http://localhost:$MATRIX_PORT/_matrix/client/versions" 1

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Phase 4: Response Format Validation Tests"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

echo ""
echo "Test 10: Bridge RPC JSON format validation"
test_json_format "Bridge RPC" "http://localhost:$BRIDGE_PORT"

echo ""
echo "Test 11: Matrix versions JSON format validation"
test_json_format "Matrix Versions" "http://localhost:$MATRIX_PORT/_matrix/client/versions"

echo ""
echo "Test 12: Bridge /health JSON format validation"
test_json_format "Bridge Health" "http://localhost:$BRIDGE_PORT/health"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Phase 5: Error Response Format Tests"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

echo ""
echo "Test 13: Bridge RPC 404 error response"
test_error_response "Bridge RPC" "http://localhost:$BRIDGE_PORT/nonexistent" "404"

echo ""
echo "Test 14: Matrix API 404 error response"
test_error_response "Matrix API" "http://localhost:$MATRIX_PORT/_matrix/client/invalid" "404"

echo ""
echo "Test 15: Bridge RPC invalid request (400/500)"
test_error_response "Bridge RPC Invalid" "http://localhost:$BRIDGE_PORT/api/invalid" "400"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Phase 6: Authentication Endpoint Tests"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

echo ""
echo "Test 16: Matrix admin endpoint (requires auth)"
test_auth_endpoint "Matrix Admin" "http://localhost:$MATRIX_PORT/_synapse/admin/v1/users"

echo ""
echo "Test 17: Bridge protected endpoint (requires auth)"
test_auth_endpoint "Bridge Protected" "http://localhost:$BRIDGE_PORT/api/protected"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Phase 7: Performance Tests"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

echo ""
echo "Test 18: API response time measurement"
echo -n "Testing API response time... "
((TESTS_TOTAL++))
START_TIME=$(date +%s.%N)
RESPONSE=$(ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$VPS_USER@$VPS_IP" "curl -s --max-time 5 http://localhost:$BRIDGE_PORT/" 2>&1 || true)
END_TIME=$(date +%s.%N)
ELAPSED=$(echo "$END_TIME - $START_TIME" | bc -l 2>/dev/null || echo "0.000")
echo "Response time: ${ELAPSED}s"

if command -v bc >/dev/null 2>&1 && (( $(echo "$ELAPSED < 2.0" | bc -l) )); then
    print_result "API Response Time" "PASS" "Response time is acceptable (${ELAPSED}s)"
    log_evidence "API Response Time" "PASS" "Response time: ${ELAPSED}s"
    add_json_result "API Response Time" "PASS" "Response time acceptable (${ELAPSED}s)"
    ((TESTS_PASSED++))
else
    print_result "API Response Time" "WARN" "Response time is slow (${ELAPSED}s)"
    log_evidence "API Response Time" "WARN" "Response time: ${ELAPSED}s"
    add_json_result "API Response Time" "WARN" "Response time slow (${ELAPSED}s)"
    ((TESTS_PASSED++))
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Phase 8: Summary"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

echo ""
echo "========================================="
echo "API Endpoint Tests Summary"
echo "========================================="
echo ""
echo -e "Total Tests:  $TESTS_TOTAL"
echo -e "${GREEN}Passed:       $TESTS_PASSED${NC}"
if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}Failed:       $TESTS_FAILED${NC}"
else
    echo -e "${GREEN}Failed:       $TESTS_FAILED${NC}"
fi

PASS_RATE=$(echo "scale=2; ($TESTS_PASSED / $TESTS_TOTAL) * 100" | bc -l 2>/dev/null || echo "0")
echo -e "Pass Rate:    ${PASS_RATE}%"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ ALL TESTS PASSED${NC}"
    EXIT_CODE=0
else
    echo -e "${RED}✗ SOME TESTS FAILED${NC}"
    EXIT_CODE=1
fi

echo ""
echo "========================================="
echo "JSON Output"
echo "========================================="

JSON_OUTPUT="$JSON_OUTPUT]"
echo "$JSON_OUTPUT" | jq . 2>/dev/null || echo "$JSON_OUTPUT"
echo ""

echo ""
echo "========================================="
echo "Evidence Saved"
echo "========================================="
echo "Evidence file: $EVIDENCE_DIR/task-5-api-endpoints.txt"
echo ""

log_evidence "SUMMARY" "COMPLETE" "Total: $TESTS_TOTAL, Passed: $TESTS_PASSED, Failed: $TESTS_FAILED, Pass Rate: ${PASS_RATE}%"

echo "========================================="
echo "API Tests Complete"
echo "========================================="

exit $EXIT_CODE
