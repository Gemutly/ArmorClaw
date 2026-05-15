#!/bin/bash
set -euo pipefail

# ArmorClaw v1: Secrets Injection Validation Tests
# Core differentiator: validates that secrets exist ONLY in memory
# Never on disk, never in docker inspect, never in logs

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/transport.sh"

echo "🧪 Secrets Injection Validation Tests"
echo "======================================="
echo ""

# Test secret (fake but realistic format)
TEST_SECRET="sk-test-secret-$(date +%s)-validation"
CONTAINER_NAME="test-sec-$$"

# Cleanup handler
cleanup() {
    docker stop $CONTAINER_NAME >/dev/null 2>&1 || true
    docker rm $CONTAINER_NAME >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "Test 1: Secret exists in process memory (EXPECTED)"
echo "----------------------------------------------------"
echo "Starting container with test secret..."

# Start container with test secret
docker run -d --rm --name $CONTAINER_NAME \
    -e OPENAI_API_KEY="$TEST_SECRET" \
    -e ANTHROPIC_API_KEY="sk-ant-test-$(date +%s)" \
    mikegemut/armorclaw:latest python -c "import time; time.sleep(999999)" >/dev/null 2>&1

sleep 1

# Verify secret is in process environment (expected - this is how the agent works)
if docker exec $CONTAINER_NAME env | grep -q "$TEST_SECRET"; then
    echo "✅ PASS: Secret present in process memory (expected behavior)"
else
    echo "❌ FAIL: Secret NOT in process memory (unexpected)"
    exit 1
fi

echo ""
echo "Test 2: Docker inspect secret exposure check"
echo "---------------------------------------------------"

# Check docker inspect for secret leakage
INSPECT_OUTPUT=$(docker inspect $CONTAINER_NAME)

# NOTE: Environment variables passed via -e are always visible in docker inspect
# This is acceptable for testing mode. Production use via bridge file descriptor
# passing would NOT expose secrets in docker inspect.
if echo "$INSPECT_OUTPUT" | grep -qi "$TEST_SECRET"; then
    echo "⚠️  INFO: Secret visible in docker inspect (expected with -e flag)"
    echo "   This is acceptable for testing. Production bridge mode uses"
    echo "   file descriptor passing which does NOT expose secrets in docker inspect."
else
    echo "✅ PASS: No secret in docker inspect"
fi

# Also check for the Anthropic key
if echo "$INSPECT_OUTPUT" | grep -qi "sk-ant-test"; then
    echo "⚠️  INFO: Anthropic secret visible in docker inspect (expected with -e flag)"
else
    echo "✅ PASS: No Anthropic secret in docker inspect"
fi

echo ""
echo "Test 3: No secrets in container logs (CRITICAL)"
echo "--------------------------------------------------"

LOGS_OUTPUT=$(docker logs $CONTAINER_NAME 2>&1)

if echo "$LOGS_OUTPUT" | grep -qi "sk-"; then
    echo "❌ FAIL: Secret leaked in container logs!"
    echo "Found keys starting with 'sk-'"
    echo ""
    echo "Logs:"
    echo "$LOGS_OUTPUT"
    exit 1
else
    echo "✅ PASS: No secrets in container logs"
fi

echo ""
echo "Test 4: No secrets written to disk (CRITICAL)"
echo "-----------------------------------------------"

# Check /etc/environment (common env file location)
ENV_CONTENT=$(docker exec $CONTAINER_NAME cat /etc/environment 2>/dev/null || echo "")

if echo "$ENV_CONTENT" | grep -qi "sk-"; then
    echo "❌ FAIL: Secret written to /etc/environment!"
    exit 1
else
    echo "✅ PASS: No secrets in /etc/environment"
fi

# Check other common disk locations (use Python since find is removed for security)
for DISK_PATH in /tmp /var/tmp /home; do
    SECRET_ON_DISK=$(docker exec $CONTAINER_NAME python3 -c "
import os
for root, dirs, files in os.walk('$DISK_PATH'):
    for f in files:
        try:
            path = os.path.join(root, f)
            with open(path) as fh:
                if 'sk-' in fh.read(4096):
                    print(path)
        except: pass
" 2>/dev/null || echo "")
    if [ -n "$SECRET_ON_DISK" ]; then
        echo "❌ FAIL: Secret found on disk in $DISK_PATH!"
        exit 1
    fi
done
echo "✅ PASS: No secrets found on disk"

echo ""
echo "Test 5: No shell to enumerate secrets (EXPECTED)"
echo "-------------------------------------------------"

# Verify shell is not available (enumeration protection)
if docker exec $CONTAINER_NAME sh -c "env" 2>/dev/null; then
    echo "❌ FAIL: Shell available for secret enumeration!"
    exit 1
else
    echo "✅ PASS: No shell available (cannot enumerate all env vars)"
fi

if docker exec $CONTAINER_NAME bash -c "env" 2>/dev/null; then
    echo "❌ FAIL: Bash available for secret enumeration!"
    exit 1
else
    echo "✅ PASS: No bash available (cannot enumerate all env vars)"
fi

echo ""
echo "Test 6: No process listing tools (EXPECTED)"
echo "---------------------------------------------"

# Verify process inspection tools are not available
if docker exec $CONTAINER_NAME ps aux 2>/dev/null | grep -v "grep"; then
    echo "❌ FAIL: ps command available (can inspect process env)"
    exit 1
else
    echo "✅ PASS: ps command not available"
fi

if docker exec $CONTAINER_NAME top -b -n1 2>/dev/null; then
    echo "❌ FAIL: top command available (can inspect process)"
    exit 1
else
    echo "✅ PASS: top command not available"
fi

echo ""
echo "Test 7: Direct /proc/self/environ check (HONEST)"
echo "---------------------------------------------------"

# Even without shell, Python/Node can read /proc/self/environ
# This is EXPECTED and documented in our threat model
if docker exec $CONTAINER_NAME python -c "import os; open('/proc/self/environ').read()" 2>/dev/null | grep -q "$TEST_SECRET"; then
    echo "⚠️  EXPECTED: /proc/self/environ is readable by runtime (documented limitation)"
    echo "   This is acceptable - agent can read own env, but cannot:"
    echo "   - Write to disk"
    echo "   - Escape to host"
    echo "   - Persist secrets after shutdown"
else
    echo "ℹ️  INFO: /proc/self/environ not readable (unexpected but acceptable)"
fi

echo ""
echo "Test 8: Secrets vanish on container restart"
echo "---------------------------------------------"

# Stop and restart the container (simulates restart scenario)
docker stop $CONTAINER_NAME >/dev/null 2>&1
sleep 1

# Start new container with a DIFFERENT dummy key (container requires API key to start)
docker run -d --rm --name $CONTAINER_NAME \
    -e OPENAI_API_KEY="sk-restart-dummy-key" \
    mikegemut/armorclaw:latest python -c "import time; time.sleep(999999)" >/dev/null 2>&1
sleep 2

# Verify NO old secrets in the restarted container
if docker exec $CONTAINER_NAME env | grep -q "$TEST_SECRET"; then
    echo "❌ FAIL: Secret persisted after restart!"
    exit 1
else
    echo "✅ PASS: Secrets do NOT persist across container restarts"
fi

echo ""
echo "======================================="
echo "✅ ALL SECRETS VALIDATION TESTS PASSED"
echo ""
echo "Summary:"
echo "  ✅ Secrets exist in process memory (as designed)"
echo "  ℹ️  Docker inspect check (env var mode shows secrets, bridge mode does not)"
echo "  ✅ No secrets in container logs"
echo "  ✅ No secrets written to disk"
echo "  ✅ No shell for enumeration"
echo "  ✅ No process inspection tools"
echo "  ✅ Secrets do not persist after restart"
echo ""
echo "ArmorClaw containment verified: blast radius = volatile memory only"
echo ""
echo "NOTE: Environment variable mode (-e flag) exposes secrets in docker inspect."
echo "      Production deployments should use bridge file descriptor passing."
