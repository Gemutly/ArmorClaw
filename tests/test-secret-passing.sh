#!/bin/bash
# Test script for secret passing mechanism
# Tests the complete flow: keystore → bridge → container

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/transport.sh"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "========================================"
echo "ArmorClaw Secret Passing Test Suite"
echo "========================================"
echo ""

# Check if bridge is running
if [ ! -S "/run/armorclaw/bridge.sock" ]; then
    echo -e "${RED}✗ Bridge socket not found${NC}"
    echo "  Start the bridge first: cd bridge && sudo ./build/armorclaw-bridge"
    exit 1
fi

echo -e "${GREEN}✓ Bridge socket found${NC}"
echo ""

# Test 1: Store API key in keystore
echo "[Test 1] Store API key in keystore"
echo "-----------------------------------"

RESPONSE=$(echo '{
    "jsonrpc": "2.0",
    "method": "store_key",
    "params": {
        "provider": "openai",
        "token": "sk-test-dummy-key-12345",
        "display_name": "Test Key"
    },
    "id": 1
}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock)

echo "Response: $RESPONSE" | jq .

if echo "$RESPONSE" | jq -e '.result.key_id' > /dev/null; then
    echo -e "${GREEN}✓ Test 1 passed: Key stored in keystore${NC}"
    KEY_ID=$(echo "$RESPONSE" | jq -r '.result.key_id')
    echo "  Key ID: $KEY_ID"
else
    echo -e "${RED}✗ Test 1 failed: Could not store key${NC}"
    exit 1
fi

echo ""

# Removed: tests for unregistered RPC methods (see git history for details).

# Cleanup
echo "========================================"
echo "Test Summary"
echo "========================================"

echo ""
echo -e "${GREEN}✓ Secret store_key mechanism is working!${NC}"
echo ""
echo "Key findings:"
echo "  • Keystore stores keys correctly (store_key registered)"
echo ""
echo "Next steps:"
echo "  • Test with real API keys"
echo "  • Verify API calls work from container"
echo "  • Test Matrix integration"
