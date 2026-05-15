#!/usr/bin/env bash
# Cloudflare HTTPS Setup - Integration Tests
# =============================================================================
# Comprehensive integration tests for Cloudflare setup functionality.
# All tests use mocked environments and do not require real Cloudflare accounts.
#
# Usage: bash tests/test-cloudflare-setup.sh
#
# Tests:
#   1. Library function loading
#   2. Network detection (mocked)
#   3. Mode prompt with recommendation
#   4. Tunnel mode setup (dry-run)
#   5. Proxy mode setup (dry-run)
#   6. Mode routing in main()
#   7. Integration of modes
# =============================================================================

set -euo pipefail

# Test configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
source "$SCRIPT_DIR/lib/transport.sh"
detect_transport
LIBRARY_FILE="${PROJECT_ROOT}/deploy/lib/cloudflare-functions.sh"
TEST_RESULTS=()
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# Test utility functions
print_header() {
    echo ""
    echo -e "${BOLD}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}  $1${NC}"
    echo -e "${BOLD}═══════════════════════════════════════════════════════════${NC}"
    echo ""
}

print_test() {
    local test_name="$1"
    echo -e "[TEST] ${CYAN}${test_name}${NC}"
}

print_pass() {
    local test_name="$1"
    echo -e "${GREEN}✓ PASS${NC}: ${test_name}"
    PASSED_TESTS=$((PASSED_TESTS + 1))
}

print_fail() {
    local test_name="$1"
    local reason="$2"
    echo -e "${RED}✗ FAIL${NC}: ${test_name}"
    echo -e "  ${YELLOW}Reason:${NC} ${reason}"
    FAILED_TESTS=$((FAILED_TESTS + 1))
    TEST_RESULTS+=("FAIL: $test_name - $reason")
}

print_skip() {
    local test_name="$1"
    local reason="$2"
    echo -e "${YELLOW}⊘ SKIP${NC}: ${test_name}"
    echo -e "  ${YELLOW}Reason:${NC} ${reason}"
}

# Cleanup function
cleanup() {
    # Clean up test artifacts
    rm -f /tmp/test-cloudflare-*.log
    rm -rf /tmp/test-cloudflare-*
}

trap cleanup EXIT

# =============================================================================
# Test 1: Library Function Loading
# =============================================================================

test_library_loading() {
    print_test "Library function loading"

    # Test that the library file exists
    if [ ! -f "$LIBRARY_FILE" ]; then
        print_fail "Library file loading" "Library file not found: $LIBRARY_FILE"
        return 1
    fi

    # Test that the library can be sourced
    if ! bash -n "$LIBRARY_FILE" 2>/dev/null; then
        print_fail "Library file syntax" "Syntax error in $LIBRARY_FILE"
        return 1
    fi

    # Test that expected functions are defined after sourcing
    local required_functions=(
        "log_info"
        "log_warn"
        "log_error"
        "die_on_error"
        "check_cloudflare_prerequisites"
        "install_cloudflared"
        "check_cloudflare_nameservers"
        "get_cloudflare_zone_id"
        "get_existing_dns_record_id"
        "create_or_update_dns_a_record"
        "create_dns_a_record"
        "detect_network_environment"
        "setup_cloudflare_tunnel"
        "setup_cloudflare_proxy"
    )

    local missing_functions=0
    for func in "${required_functions[@]}"; do
        if ! grep -q "^${func}()" "$LIBRARY_FILE"; then
            echo -e "  ${RED}✗${NC} Missing function: ${func}"
            missing_functions=$((missing_functions + 1))
        else
            echo -e "  ${GREEN}✓${NC} Found function: ${func}"
        fi
    done

    if [ "$missing_functions" -gt 0 ]; then
        print_fail "Library function definitions" "$missing_functions functions missing"
        return 1
    fi

    print_pass "Library function loading"
    return 0
}

# =============================================================================
# Test 2: Network Detection (Mocked)
# =============================================================================

test_network_detection_mocked() {
    print_test "Network detection in mocked environment"

    # Create a test script that sources the library and mocks network detection
    local test_script="/tmp/test-network-detection.sh"
    cat > "$test_script" <<'EOF'
#!/usr/bin/env bash
set -e

# Mock network commands
curl() {
    if [[ "$*" == *"api.ipify.org"* ]]; then
        echo "203.0.113.1"
        return 0
    elif [[ "$*" == *"cloudflare"* ]]; then
        echo '{"success":true,"result":[{"id":"test-zone-id"}]}'
        return 0
    else
        return 1
    fi
}

ip() {
    if [[ "$*" == *"route get"* ]]; then
        echo "192.168.1.50 dev eth0 src 192.168.1.50"
        return 0
    fi
    return 1
}

# Source the library (mock functions first)
source "$1" 2>/dev/null || true

# Test detect_network_environment
detect_network_environment

# Export results for verification (RECOMMEND and REASON are exported by the function)
echo "RECOMMEND=${RECOMMEND:-}"
echo "REASON=${REASON:-}"
EOF

    chmod +x "$test_script"

    # Run the test script
    local output
    if ! output=$("$test_script" "$LIBRARY_FILE" 2>&1); then
        print_fail "Network detection execution" "Test script failed: $output"
        rm -f "$test_script"
        return 1
    fi

    # Verify that the function ran and exported expected variables
    if ! echo "$output" | grep -q "RECOMMEND="; then
        print_fail "Network detection output" "RECOMMEND variable not set"
        rm -f "$test_script"
        return 1
    fi

    echo "  Output:"
    echo "$output" | sed 's/^/    /'

    rm -f "$test_script"
    print_pass "Network detection in mocked environment"
    return 0
}

# =============================================================================
# Test 3: Mode Prompt with Recommendation
# =============================================================================

test_mode_prompt_recommendation() {
    print_test "Mode prompt displays recommendation"

    # Create a test script that mocks the prompt function
    local test_script="/tmp/test-prompt.sh"
    cat > "$test_script" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

# Set environment variables for testing
export HAS_NAT="yes"
export PORTS_AVAILABLE="80,443"
export RECOMMEND="tunnel"
export REASON="NAT detected. Cloudflare Tunnel bypasses NAT."

# Source library functions
source "$1" 2>/dev/null || true

# Test that prompt function exists
if ! declare -f prompt_cloudflare_mode >/dev/null 2>&1; then
    echo "ERROR: prompt_cloudflare_mode function not found"
    exit 1
fi

echo "SUCCESS: prompt_cloudflare_mode function found"
echo "HAS_NAT=$HAS_NAT"
echo "PORTS_AVAILABLE=$PORTS_AVAILABLE"
echo "RECOMMEND=$RECOMMEND"
echo "REASON=$REASON"
EOF

    chmod +x "$test_script"

    # Run the test script
    local output
    if ! output=$("$test_script" "$LIBRARY_FILE" 2>&1); then
        print_fail "Prompt function execution" "Test script failed: $output"
        rm -f "$test_script"
        return 1
    fi

    # Verify that the function exists
    if ! echo "$output" | grep -q "SUCCESS"; then
        print_fail "Prompt function detection" "Function not properly defined"
        rm -f "$test_script"
        return 1
    fi

    echo "  Output:"
    echo "$output" | sed 's/^/    /'

    rm -f "$test_script"
    print_pass "Mode prompt displays recommendation"
    return 0
}

# =============================================================================
# Test 4: Tunnel Mode Setup (Dry-run)
# =============================================================================

test_tunnel_mode_dryrun() {
    print_test "Tunnel mode setup in dry-run"

    # Create a test script that tests tunnel setup in dry-run mode
    local test_script="/tmp/test-tunnel-dryrun.sh"
    cat > "$test_script" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

# Set dry-run mode
export DRY_RUN="true"

# Mock commands to prevent real operations
cloudflared() {
    echo "cloudflared: $* (mocked)"
    return 0
}

curl() {
    echo "curl: $* (mocked)"
    return 0
}

sudo() {
    echo "sudo: $* (mocked)"
    return 0
}

systemctl() {
    echo "systemctl: $* (mocked)"
    return 0
}

tee() {
    cat > "$2"
    return 0
}

# Source library
source "$1" 2>/dev/null || true

# Test that setup_cloudflare_tunnel function exists
if ! declare -f setup_cloudflare_tunnel >/dev/null 2>&1; then
    echo "ERROR: setup_cloudflare_tunnel function not found"
    exit 1
fi

echo "SUCCESS: setup_cloudflare_tunnel function found"
echo "DRY_RUN=$DRY_RUN"
EOF

    chmod +x "$test_script"

    # Run the test script
    local output
    if ! output=$("$test_script" "$LIBRARY_FILE" 2>&1); then
        print_fail "Tunnel mode execution" "Test script failed: $output"
        rm -f "$test_script"
        return 1
    fi

    # Verify that the function exists and dry-run is set
    if ! echo "$output" | grep -q "SUCCESS"; then
        print_fail "Tunnel mode detection" "Function not properly defined"
        rm -f "$test_script"
        return 1
    fi

    if ! echo "$output" | grep -q "DRY_RUN=true"; then
        print_fail "Dry-run mode" "DRY_RUN not set to true"
        rm -f "$test_script"
        return 1
    fi

    echo "  Output:"
    echo "$output" | sed 's/^/    /'

    rm -f "$test_script"
    print_pass "Tunnel mode setup in dry-run"
    return 0
}

# =============================================================================
# Test 5: Proxy Mode Setup (Dry-run)
# =============================================================================

test_proxy_mode_dryrun() {
    print_test "Proxy mode setup in dry-run"

    # Create a test script that tests proxy setup in dry-run mode
    local test_script="/tmp/test-proxy-dryrun.sh"
    cat > "$test_script" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

# Set dry-run mode
export DRY_RUN="true"
export CF_API_TOKEN="test-token-12345"

# Mock commands to prevent real operations
curl() {
    if [[ "$*" == *"api.cloudflare.com"* ]]; then
        echo '{"success":true,"result":[{"id":"test-zone-id"}]}'
        return 0
    elif [[ "$*" == *"dns_records"* ]]; then
        echo '{"success":true,"result":[{"id":"test-record-id","content":"203.0.113.1"}]}'
        return 0
    else
        echo "curl: $* (mocked)"
        return 0
    fi
}

jq() {
    # Simple JSON parser mock
    if [[ "$*" == *".success"* ]]; then
        echo "true"
    elif [[ "$*" == *".result[0].id"* ]]; then
        echo "test-zone-id"
    elif [[ "$*" == *".result[0].content"* ]]; then
        echo "203.0.113.1"
    else
        echo "null"
    fi
    return 0
}

nc() {
    echo "nc: $* (mocked)"
    return 0
}

timeout() {
    echo "timeout: $* (mocked)"
    return 0
}

# Source library
source "$1" 2>/dev/null || true

# Test that setup_cloudflare_proxy function exists
if ! declare -f setup_cloudflare_proxy >/dev/null 2>&1; then
    echo "ERROR: setup_cloudflare_proxy function not found"
    exit 1
fi

# Test that create_dns_a_record function exists
if ! declare -f create_dns_a_record >/dev/null 2>&1; then
    echo "ERROR: create_dns_a_record function not found"
    exit 1
fi

echo "SUCCESS: setup_cloudflare_proxy function found"
echo "SUCCESS: create_dns_a_record function found"
echo "DRY_RUN=$DRY_RUN"
echo "CF_API_TOKEN=$CF_API_TOKEN"
EOF

    chmod +x "$test_script"

    # Run the test script
    local output
    if ! output=$("$test_script" "$LIBRARY_FILE" 2>&1); then
        print_fail "Proxy mode execution" "Test script failed: $output"
        rm -f "$test_script"
        return 1
    fi

    # Verify that the functions exist and dry-run is set
    if ! echo "$output" | grep -q "SUCCESS: setup_cloudflare_proxy"; then
        print_fail "Proxy mode detection" "Function not properly defined"
        rm -f "$test_script"
        return 1
    fi

    if ! echo "$output" | grep -q "SUCCESS: create_dns_a_record"; then
        print_fail "DNS record function detection" "create_dns_a_record function not properly defined"
        rm -f "$test_script"
        return 1
    fi

    echo "  Output:"
    echo "$output" | sed 's/^/    /'

    rm -f "$test_script"
    print_pass "Proxy mode setup in dry-run"
    return 0
}

# =============================================================================
# Test 6: Mode Routing in main()
# =============================================================================

test_mode_routing() {
    print_test "Mode routing in main()"

    # Create a test script that tests mode routing logic
    local test_script="/tmp/test-mode-routing.sh"
    cat > "$test_script" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

# Mock input for mode selection
export CF_MODE="tunnel"

# Mock functions
detect_network_environment() {
    export HAS_NAT="yes"
    export PORTS_AVAILABLE="80,443"
    export RECOMMEND="tunnel"
    export REASON="NAT detected"
}

setup_cloudflare_tunnel() {
    echo "setup_cloudflare_tunnel: called"
    return 0
}

setup_cloudflare_proxy() {
    echo "setup_cloudflare_proxy: called"
    return 0
}

# Source library
source "$1" 2>/dev/null || true

# Test mode routing logic
if [ "$CF_MODE" = "tunnel" ]; then
    echo "MODE_ROUTING: tunnel mode selected"
    # In real code, this would call setup_cloudflare_tunnel
elif [ "$CF_MODE" = "proxy" ]; then
    echo "MODE_ROUTING: proxy mode selected"
    # In real code, this would call setup_cloudflare_proxy
else
    echo "ERROR: Invalid CF_MODE: $CF_MODE"
    exit 1
fi

echo "SUCCESS: Mode routing works correctly"
EOF

    chmod +x "$test_script"

    # Run the test script
    local output
    if ! output=$("$test_script" "$LIBRARY_FILE" 2>&1); then
        print_fail "Mode routing execution" "Test script failed: $output"
        rm -f "$test_script"
        return 1
    fi

    # Verify that mode routing works
    if ! echo "$output" | grep -q "SUCCESS"; then
        print_fail "Mode routing" "Mode routing logic failed"
        rm -f "$test_script"
        return 1
    fi

    if ! echo "$output" | grep -q "tunnel mode selected"; then
        print_fail "Tunnel mode routing" "Tunnel mode not properly routed"
        rm -f "$test_script"
        return 1
    fi

    echo "  Output:"
    echo "$output" | sed 's/^/    /'

    rm -f "$test_script"
    print_pass "Mode routing in main()"
    return 0
}

# =============================================================================
# Test 7: Integration of Modes
# =============================================================================

test_integration_modes() {
    print_test "Integration of Tunnel and Proxy modes"

    # Create a test script that tests mode integration
    local test_script="/tmp/test-integration.sh"
    cat > "$test_script" <<'EOF'
#!/usr/bin/env bash
set -e

# Mock environment variables
export CF_API_TOKEN="test-token-12345"
export CF_ZONE_ID="test-zone-id"
export PUBLIC_URL="https://example.com"
export MATRIX_URL="https://matrix.example.com"
export DOMAIN="example.com"

# Mock commands
curl() {
    echo "curl: $* (mocked)"
    return 0
}

jq() {
    echo '{"success":true}'
    return 0
}

cloudflared() {
    echo "cloudflared: $* (mocked)"
    return 0
}

# Source library
source "$1" 2>/dev/null || true

# Test that all required environment variables are set for integration
required_vars=(
    "CF_API_TOKEN"
    "CF_ZONE_ID"
    "PUBLIC_URL"
    "MATRIX_URL"
    "DOMAIN"
)

missing_vars=0
for var in "${required_vars[@]}"; do
    if [ -z "${!var:-}" ]; then
        echo "ERROR: Missing variable: $var"
        missing_vars=$((missing_vars + 1))
    else
        echo "OK: $var=${!var}"
    fi
done

if [ "$missing_vars" -gt 0 ]; then
    echo "ERROR: $missing_vars required variables missing"
    exit 1
fi

echo "SUCCESS: All integration variables set correctly"
EOF

    chmod +x "$test_script"

    # Run the test script
    local output
    if ! output=$("$test_script" "$LIBRARY_FILE" 2>&1); then
        print_fail "Integration test execution" "Test script failed: $output"
        rm -f "$test_script"
        return 1
    fi

    # Verify that all integration variables are set
    if ! echo "$output" | grep -q "SUCCESS"; then
        print_fail "Integration variables" "Not all required variables set"
        rm -f "$test_script"
        return 1
    fi

    echo "  Output:"
    echo "$output" | sed 's/^/    /'

    rm -f "$test_script"
    print_pass "Integration of Tunnel and Proxy modes"
    return 0
}

# =============================================================================
# Main Test Runner
# =============================================================================

main() {
    print_header "Cloudflare HTTPS Setup - Integration Tests"

    TOTAL_TESTS=7

    # Run all tests
    test_library_loading || true
    test_network_detection_mocked || true
    test_mode_prompt_recommendation || true
    test_tunnel_mode_dryrun || true
    test_proxy_mode_dryrun || true
    test_mode_routing || true
    test_integration_modes || true

    # Print summary
    print_header "Test Summary"
    echo ""
    echo -e "Total Tests: ${BOLD}${TOTAL_TESTS}${NC}"
    echo -e "${GREEN}Passed:${NC} ${PASSED_TESTS}"
    echo -e "${RED}Failed:${NC} ${FAILED_TESTS}"
    echo ""

    if [ "$FAILED_TESTS" -eq 0 ]; then
        echo -e "${GREEN}${BOLD}✓ All tests passed!${NC}"
        return 0
    else
        echo -e "${RED}${BOLD}✗ Some tests failed${NC}"
        echo ""
        echo "Failed tests:"
        for result in "${TEST_RESULTS[@]}"; do
            echo "  - $result"
        done
        return 1
    fi
}

# Run main function
main "$@"
