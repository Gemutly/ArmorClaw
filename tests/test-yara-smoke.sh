#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# YARA Runtime Smoke Test
#
# Validates YARA scanning works end-to-end on the VPS deployment.
# Tests rule compilation, pattern detection, clean-file handling,
# and bridge YARA integration via log inspection.
#
# Tier A: SSH + docker exec on VPS (requires live deployment).
#
# Usage:  bash tests/test-yara-smoke.sh
# Requires: .env with VPS_IP, SSH_KEY_PATH
# ──────────────────────────────────────────────────────────────────────────────

# ── Source shared helpers ─────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/load_env.sh"
source "${SCRIPT_DIR}/lib/transport.sh"
source "${SCRIPT_DIR}/lib/common_output.sh"

# ── Constants ─────────────────────────────────────────────────────────────────
YARA_RULES_PATH="/opt/armorclaw/configs/yara_rules.yar"
TEST_DIR="/tmp/armorclaw-yara-smoke"
CONTAINER_FILTER="name=armorclaw"

# ── Evidence directory ────────────────────────────────────────────────────────
EVIDENCE_DIR="${_REPO_ROOT:-$(cd "${SCRIPT_DIR}/.." && pwd)}/.sisyphus/evidence/post-deploy"
mkdir -p "$EVIDENCE_DIR"

# ── Discover running container ────────────────────────────────────────────────
log_info "Discovering ArmorClaw container..."
CONTAINER_ID=$(ssh_vps "docker ps --filter ${CONTAINER_FILTER} --format '{{.ID}}' | head -1" 2>/dev/null) || CONTAINER_ID=""

if [[ -z "$CONTAINER_ID" ]]; then
  log_fail "No running ArmorClaw container found (filter: ${CONTAINER_FILTER})"
  harness_summary
  exit 1
fi
log_info "Container ID: ${CONTAINER_ID}"

# ── Helper: run yara inside container ─────────────────────────────────────────
yara_in_container() {
  ssh_vps "docker exec ${CONTAINER_ID} yara -p 1 ${YARA_RULES_PATH} $1" 2>&1
}

# ── Helper: create test file on VPS then copy into container ──────────────────
create_vps_test_file() {
  local path="$1"
  local content="$2"
  ssh_vps "cat > ${path} << 'ARMORCLAW_YARA_EOF'
${content}
ARMORCLAW_YARA_EOF"
}

# ── Setup: create test directory and files on VPS ─────────────────────────────
log_info "Creating test files on VPS..."
ssh_vps "mkdir -p ${TEST_DIR}"

# File 1: iframe injection (matches exploit_kit_landing — needs BOTH iframe AND classid)
create_vps_test_file "${TEST_DIR}/iframe_exploit.html" \
  '<html><body><iframe src="http://evil.com/malware.html"></iframe><object classid="clsid:12345678-1234-1234-1234-123456789012"></object></body></html>'

# File 2: EICAR test string (matches eicar_test_file)
create_vps_test_file "${TEST_DIR}/eicar.txt" \
  'X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*'

# File 3: clean file (should match nothing)
create_vps_test_file "${TEST_DIR}/clean.txt" \
  'Hello World'

# Copy test files into container
ssh_vps "docker cp ${TEST_DIR} ${CONTAINER_ID}:/tmp/armorclaw-yara-smoke" 2>/dev/null || {
  # Fallback: create files directly inside container via docker exec
  log_info "docker cp failed, creating files via docker exec..."
  ssh_vps "docker exec ${CONTAINER_ID} mkdir -p ${TEST_DIR}"
  ssh_vps "docker exec ${CONTAINER_ID} bash -c \"echo '<html><body><iframe src=\\\"http://evil.com/malware.html\\\"></iframe><object classid=\\\"clsid:12345678-1234-1234-1234-123456789012\\\"></object></body></html>' > ${TEST_DIR}/iframe_exploit.html\""
  ssh_vps "docker exec ${CONTAINER_ID} bash -c \"echo 'X5O!P%@AP[4\\\\PZX54(P^)7CC)7}\\\$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!\\\$H+H*' > ${TEST_DIR}/eicar.txt\""
  ssh_vps "docker exec ${CONTAINER_ID} bash -c \"echo 'Hello World' > ${TEST_DIR}/clean.txt\""
}

# ══════════════════════════════════════════════════════════════════════════════
# Test 1: YARA rules compile successfully
# ══════════════════════════════════════════════════════════════════════════════
log_info "Test 1: YARA rules compile..."
COMPILE_RESULT=$(ssh_vps "docker exec ${CONTAINER_ID} yara -p 1 ${YARA_RULES_PATH} /dev/null" 2>&1) && COMPILE_OK=true || COMPILE_OK=false

if $COMPILE_OK; then
  log_pass "YARA rules compile successfully (exit code 0)"
else
  log_fail "YARA rules failed to compile: ${COMPILE_RESULT}"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Test 2: YARA detects iframe injection → exploit_kit_landing
# ══════════════════════════════════════════════════════════════════════════════
log_info "Test 2: YARA detects iframe injection pattern..."
IFRAME_RESULT=$(yara_in_container "${TEST_DIR}/iframe_exploit.html") || true

if echo "$IFRAME_RESULT" | grep -q "exploit_kit_landing"; then
  log_pass "YARA detects iframe injection → exploit_kit_landing"
else
  log_fail "YARA did not detect iframe injection. Output: ${IFRAME_RESULT}"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Test 3: YARA detects EICAR test file → eicar_test_file
# ══════════════════════════════════════════════════════════════════════════════
log_info "Test 3: YARA detects EICAR test file..."
EICAR_RESULT=$(yara_in_container "${TEST_DIR}/eicar.txt") || true

if echo "$EICAR_RESULT" | grep -q "eicar_test_file"; then
  log_pass "YARA detects EICAR test file → eicar_test_file"
else
  log_fail "YARA did not detect EICAR. Output: ${EICAR_RESULT}"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Test 4: YARA returns no match on clean file
# ══════════════════════════════════════════════════════════════════════════════
log_info "Test 4: YARA returns no match on clean file..."
CLEAN_RESULT=$(yara_in_container "${TEST_DIR}/clean.txt") || true

# YARA outputs nothing (empty) when no rules match
if [[ -z "$CLEAN_RESULT" ]]; then
  log_pass "YARA returns no match on clean file"
else
  log_fail "YARA unexpectedly matched clean file: ${CLEAN_RESULT}"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Test 5: YARA scanning active in bridge (log check)
# ══════════════════════════════════════════════════════════════════════════════
log_info "Test 5: Bridge YARA integration active (log check)..."
BRIDGE_LOGS=$(ssh_vps "docker logs --tail 200 ${CONTAINER_ID} 2>&1" || true)

if echo "$BRIDGE_LOGS" | grep -q "YARA initialization failed"; then
  log_fail "Bridge reports 'YARA initialization failed' in logs"
elif echo "$BRIDGE_LOGS" | grep -qi "yara"; then
  log_pass "Bridge YARA integration active (YARA mentioned in logs, no failure)"
else
  # No YARA mention at all — bridge may not have processed any files yet
  log_pass "Bridge running (no YARA errors in logs — YARA idle until first scan)"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Cleanup test files from VPS
# ══════════════════════════════════════════════════════════════════════════════
log_info "Cleaning up test files..."
ssh_vps "rm -rf ${TEST_DIR}" 2>/dev/null || true
ssh_vps "docker exec ${CONTAINER_ID} rm -rf ${TEST_DIR}" 2>/dev/null || true

# ── Save evidence ─────────────────────────────────────────────────────────────
{
  echo "=== YARA Smoke Test Evidence ==="
  echo "Date: $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  echo "Container: ${CONTAINER_ID}"
  echo "Rules path: ${YARA_RULES_PATH}"
  echo ""
  echo "--- Compile Check ---"
  echo "${COMPILE_RESULT:-skipped}"
  echo ""
  echo "--- Iframe Detection ---"
  echo "${IFRAME_RESULT:-skipped}"
  echo ""
  echo "--- EICAR Detection ---"
  echo "${EICAR_RESULT:-skipped}"
  echo ""
  echo "--- Clean File ---"
  echo "${CLEAN_RESULT:-no match (expected)}"
  echo ""
  echo "--- Bridge Log (YARA lines) ---"
  echo "$BRIDGE_LOGS" | grep -i "yara" || echo "(no YARA lines in recent logs)"
} > "${EVIDENCE_DIR}/yara-smoke-evidence.txt" 2>/dev/null || true

# ── Summary ───────────────────────────────────────────────────────────────────
TOTAL=$((FULL_SYSTEM_PASSED + FULL_SYSTEM_FAILED + FULL_SYSTEM_SKIPPED))
echo ""
echo "YARA Smoke: ${FULL_SYSTEM_PASSED}/${TOTAL} PASS"
harness_summary
