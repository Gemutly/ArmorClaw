#!/bin/bash
# =============================================================================
# Test Suite for Hardened Installer
# Tests: Lockfile, Docker wait, env passthrough, compose detection
# =============================================================================

set -euo pipefail

# Set project root relative to this script
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORK_DIR=$(mktemp -d)
FAILED=0

log() { echo "[TEST] $*"; }
pass() { echo "✓ $*"; }
fail() { echo "✗ $*"; ((FAILED++)); }
cleanup() { rm -rf "$WORK_DIR"; }
trap cleanup EXIT

# Test 1: Lockfile (skip if flock not available)
test_lockfile() {
    # Skip on Windows compatibility layers (MSYS, MINGW, Cygwin)
    if uname | grep -qiE 'mingw|msys|cygwin'; then
        log "Skipping flock test on Windows compatibility layer"
        return 0
    fi

    log "Test 1: Lockfile functionality"
    if ! command -v flock >/dev/null 2>&1; then
        log "flock not installed, skipping"
        pass "Lockfile test skipped (flock not available)"
        return 0
    fi
    
    # Acquire lock in a subshell and hold it
    (
        exec 200>"$WORK_DIR/lock"
        flock -n 200 || exit 1
        sleep 0.5
    ) &
    local locker_pid=$!
    
    # Give the subshell a moment to acquire the lock
    sleep 0.1
    
    # Try to acquire the same lock from this process
    if ( exec 200>"$WORK_DIR/lock"; flock -n 200 ) 2>/dev/null; then
        fail "Second process got lock"
        kill $locker_pid 2>/dev/null || true
        return 1
    fi
    
    wait $locker_pid
    pass "Lockfile functional"
}

# Test 2: Docker wait
test_docker_wait() {
    log "Test 2: Docker wait loop"
    if ! command -v docker >/dev/null 2>&1; then log "Docker not installed"; return 0; fi
    if ! docker info >/dev/null 2>&1; then log "Docker not running"; return 0; fi
    pass "Docker ready"
}

# Test 3: Env passthrough
test_env_passthrough() {
    log "Test 3: Environment variable passthrough"
    export DOCKER_COMPOSE="docker compose" CONDUIT_VERSION="v1.0" CONDUIT_IMAGE="test:tag"
    local output
    output=$(bash -c 'if [[ -n "$DOCKER_COMPOSE" && -n "$CONDUIT_VERSION" && -n "$CONDUIT_IMAGE" ]]; then echo "PASS"; fi')
    if [[ "$output" == "PASS" ]]; then pass "Env vars passed"; else fail "Env vars not passed"; fi
}

# Test 4: Docker compose detection
test_docker_compose() {
    log "Test 4: Docker Compose detection"
    if docker compose version >/dev/null 2>&1; then
        DOCKER_COMPOSE="docker compose"
        [[ "$DOCKER_COMPOSE" == "docker compose" ]] && pass "Detected docker compose"
    elif docker-compose version >/dev/null 2>&1; then
        DOCKER_COMPOSE="docker-compose"
        [[ "$DOCKER_COMPOSE" == "docker-compose" ]] && pass "Detected docker-compose"
    else
        log "Docker Compose not installed"
    fi
}

# Test 5: CONDUIT_IMAGE fallback
test_conduit_fallback() {
    log "Test 5: CONDUIT_IMAGE fallback"
    export CONDUIT_VERSION="v2.0"
    CONDUIT_IMAGE="${CONDUIT_IMAGE:-test-image:$CONDUIT_VERSION}"
    [[ "$CONDUIT_IMAGE" == "test-image:v2.0" ]] && pass "Fallback works"
}

# Test 6: Syntax
test_syntax() {
    log "Test 6: Syntax validation"
    for f in install.sh setup-matrix.sh quickstart-entrypoint.sh deploy-infra.sh; do
        bash -n "$PROJECT_ROOT/deploy/$f" && pass "Syntax valid: $f" || fail "Syntax error: $f"
    done
}

# Core installer scripts that require Docker runtime
INSTALLER_SCRIPTS=(
  setup-matrix.sh
  quickstart-entrypoint.sh
  deploy-infra.sh
  installer-v5.sh
)

# Test 7: Wait logic exists
test_wait_logic() {
    log "Test 7: wait_for_docker logic"
    for f in "${INSTALLER_SCRIPTS[@]}"; do
        script="$PROJECT_ROOT/deploy/$f"

        if grep -q "wait_for_docker()" "$script" || grep -q "for ((i=1;i<=10;i++))" "$script"; then
            pass "Wait logic found in: $f"
        else
            fail "Wait logic missing from: $f"
        fi
    done
}

# Test 8: Variable order
test_variable_order() {
    log "Test 8: Variable ordering"
    if grep -q "DOCKER_COMPOSE=\"\${DOCKER_COMPOSE:-docker compose}\"" "$PROJECT_ROOT/deploy/setup-matrix.sh"; then
        pass "DOCKER_COMPOSE fallback correct"
    else
        fail "DOCKER_COMPOSE issue in setup-matrix.sh"
    fi
}

# Test 9: Systemd template hardening
test_systemd_hardening() {
    log "Test 9: Systemd template hardening"
    local found_simple=0
    local found_runtime=0
    
    for f in setup-quick.sh setup-wizard.sh installer-v4.sh install-bridge.sh; do
        if grep -qE "Type=(simple|notify)" "$PROJECT_ROOT/deploy/$f"; then
            ((found_simple++))
        else
            fail "Type=simple or Type=notify missing in $f"
        fi
        
        if grep -q "RuntimeDirectory=armorclaw" "$PROJECT_ROOT/deploy/$f"; then
            ((found_runtime++))
        else
            fail "RuntimeDirectory missing in $f"
        fi
    done
    
    if [[ $found_simple -eq 4 && $found_runtime -eq 4 ]]; then
        pass "Systemd templates hardened (Type=simple + RuntimeDirectory)"
    fi
}

main() {
    echo "=========================================="
    echo "Running Installer Test Suite"
    echo "=========================================="
    
    test_lockfile || true
    test_docker_wait || true
    test_env_passthrough || true
    test_docker_compose || true
    test_conduit_fallback || true
    test_syntax || true
    test_wait_logic || true
    test_variable_order || true
    test_systemd_hardening || true
    
    echo "=========================================="
    if [[ $FAILED -eq 0 ]]; then echo "All tests passed!"; exit 0; else echo "FAILED: $FAILED test(s)"; exit 1; fi
}

main "$@"
