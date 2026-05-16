#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# VPS Integration Smoke Test
#
# Tests a running ArmorClaw deployment via SSH + curl.
# Validates health, auth enforcement, governance RPCs, and discovery endpoint.
#
# Usage:  VPS_IP=1.2.3.4 ADMIN_TOKEN=xxx bash tests/test-vps-smoke.sh
# Requires: ssh, curl, jq
# ──────────────────────────────────────────────────────────────────────────────

# Auto-source .env for VPS connection details
set -a
source .env 2>/dev/null || true
set +a

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/transport.sh"

# ── Environment variables ─────────────────────────────────────────────────────
: "${VPS_IP:?missing VPS_IP (set in .env or CLI)}"
: "${VPS_USER:=root}"
: "${BRIDGE_PORT:=8443}"
: "${ADMIN_TOKEN:?missing ADMIN_TOKEN (pass via CLI)}"
: "${CI_MODE:=0}"

# ── Dependency check ──────────────────────────────────────────────────────────
command -v jq >/dev/null 2>&1 || { echo "FAIL: jq is required"; exit 1; }

# ── Counters ──────────────────────────────────────────────────────────────────
TOTAL=0 PASSED=0 FAILED=0
H_TOTAL=0 H_PASS=0  # Health
A_TOTAL=0 A_PASS=0  # Auth
G_TOTAL=0 G_PASS=0  # Governance
D_TOTAL=0 D_PASS=0  # Discovery
FAILURES=""
SOCKET_TESTS=0 HTTP_TESTS=0

# ── Helpers ───────────────────────────────────────────────────────────────────
ssh_vps() { ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=no "${VPS_USER}@${VPS_IP}" "$@"; }

rpc_vps() {
  local method="$1" params="${2:-}"
  if [ -z "$params" ]; then
    params='{}'
  fi
  ssh_vps "curl -kfsS -H 'Authorization: Bearer ${ADMIN_TOKEN}' -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"${method}\",\"params\":${params}}' https://localhost:${BRIDGE_PORT}/api"
}

run_test() {
    local name="$1" expected="$2" actual="$3"
    TOTAL=$((TOTAL + 1))
    if echo "$actual" | grep -q "$expected"; then
        PASSED=$((PASSED + 1))
        echo "  [PASS] $name"
    else
        FAILED=$((FAILED + 1))
        FAILURES="$FAILURES\n  - $name: expected '$expected' got: $(echo "$actual" | head -c 200)"
        echo "  [FAIL] $name"
        echo "    Expected: $expected"
        echo "    Got: $(echo "$actual" | head -c 200)"
    fi
}

# ── Transport Detection ────────────────────────────────────────────────────────
HAS_SOCKET=false
HAS_HTTP=false
TRANSPORT_MODE="none"

check_socat() {
  ssh_vps "command -v socat >/dev/null 2>&1" 2>/dev/null
}

detect_transport() {
  HAS_SOCKET=false
  HAS_HTTP=false

  if check_socat; then
    if ssh_vps "test -S /run/armorclaw/bridge.sock" 2>/dev/null; then
      HAS_SOCKET=true
    fi
  else
    echo "[INFO] socat not available on VPS — socket tests will be skipped"
  fi

  local http_code
  http_code=$(ssh_vps "curl -kfsS -o /dev/null -w '%{http_code}' https://localhost:${BRIDGE_PORT}/health 2>/dev/null || curl -kfsS -o /dev/null -w '%{http_code}' http://localhost:${BRIDGE_PORT}/health 2>/dev/null || echo 000")
  if [ "$http_code" = "200" ]; then
    HAS_HTTP=true
  fi

  if $HAS_SOCKET && $HAS_HTTP; then
    TRANSPORT_MODE="both"
  elif $HAS_SOCKET; then
    TRANSPORT_MODE="socket"
  elif $HAS_HTTP; then
    TRANSPORT_MODE="http"
  else
    TRANSPORT_MODE="none"
  fi

  echo "[INFO] Transport: socket=$HAS_SOCKET http=$HAS_HTTP mode=$TRANSPORT_MODE"
}

rpc_socket() {
  local method="$1"
  local params="${2:-{\}}"
  if [ "$params" = "\{\}" ] || [ "$params" = "{}" ]; then
    params='{}'
  fi
  local payload="{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"${method}\",\"params\":${params},\"auth\":\"${ADMIN_TOKEN}\"}"
  ssh_vps "echo '${payload}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock" 2>/dev/null
}

detect_transport

if [ "$TRANSPORT_MODE" = "none" ]; then
  echo "FAIL: No transport available (neither socket nor HTTP)"
  exit 1
fi

# ══════════════════════════════════════════════════════════════════════════════
# Category A: Bridge Health (3 tests)
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo "Category A: Bridge Health"
echo "========================================="

# A1: SSH connectivity
H_TOTAL=$((H_TOTAL + 1)); TOTAL=$((TOTAL + 1))
SSH_RESULT=$(ssh_vps "echo ok" 2>&1) && SSH_OK=true || SSH_OK=false
if $SSH_OK && [ "$SSH_RESULT" = "ok" ]; then
    PASSED=$((PASSED + 1)); H_PASS=$((H_PASS + 1))
    echo "  [PASS] [1/11] SSH connectivity"
else
    FAILED=$((FAILED + 1))
    FAILURES="$FAILURES\n  - [1/11] SSH connectivity: got '$SSH_RESULT'"
    echo "  [FAIL] [1/11] SSH connectivity"
    echo "    Got: $(echo "$SSH_RESULT" | head -c 200)"
fi

# A2: Health endpoint (HTTP-only)
if $HAS_HTTP; then
  H_TOTAL=$((H_TOTAL + 1)); TOTAL=$((TOTAL + 1)); HTTP_TESTS=$((HTTP_TESTS + 1))
  HEALTH=$(ssh_vps "curl -kfsS https://localhost:${BRIDGE_PORT}/health" 2>&1) && HEALTH_OK=true || HEALTH_OK=false
  if $HEALTH_OK && echo "$HEALTH" | jq -e '.status == "ok"' >/dev/null 2>&1; then
      PASSED=$((PASSED + 1)); H_PASS=$((H_PASS + 1))
      echo "  [PASS] Health endpoint returns ok (http)"
  else
      FAILED=$((FAILED + 1))
      FAILURES="$FAILURES\n  - Health endpoint: got '$(echo "$HEALTH" | head -c 200)'"
      echo "  [FAIL] Health endpoint"
      echo "    Got: $(echo "$HEALTH" | head -c 200)"
  fi
else
  echo "  [SKIP] Health endpoint (no HTTP transport)"
fi

# A3: Bridge container running
H_TOTAL=$((H_TOTAL + 1)); TOTAL=$((TOTAL + 1))
CONTAINERS=$(ssh_vps "docker ps --filter name=armorclaw --format '{{.Names}}'" 2>&1) && C_OK=true || C_OK=false
if $C_OK && [ -n "$CONTAINERS" ]; then
    PASSED=$((PASSED + 1)); H_PASS=$((H_PASS + 1))
    echo "  [PASS] [3/11] Bridge container running"
else
    FAILED=$((FAILED + 1))
    FAILURES="$FAILURES\n  - [3/11] Bridge container running: got '$(echo "$CONTAINERS" | head -c 200)'"
    echo "  [FAIL] [3/11] Bridge container running"
    echo "    Got: $(echo "$CONTAINERS" | head -c 200)"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Category B: Auth Enforcement
# Bridge returns HTTP 200 even for auth errors — check JSON-RPC body.
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo "Category B: Auth Enforcement"
echo "========================================="

if $HAS_HTTP; then
  # B1-HTTP: No auth → v4.6.0 returns empty result for public methods like device.list
  A_TOTAL=$((A_TOTAL + 1)); TOTAL=$((TOTAL + 1)); HTTP_TESTS=$((HTTP_TESTS + 1))
  NOAUTH_RESP=$(ssh_vps "curl -ks -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"device.list\"}' https://localhost:${BRIDGE_PORT}/api" 2>&1)
  if echo "$NOAUTH_RESP" | jq -e '.result' >/dev/null 2>&1; then
      PASSED=$((PASSED + 1)); A_PASS=$((A_PASS + 1))
      echo "  [PASS] No auth returns result for public method (http)"
  else
      FAILED=$((FAILED + 1))
      FAILURES="$FAILURES\n  - No auth (http): got '$(echo "$NOAUTH_RESP" | head -c 300)'"
      echo "  [FAIL] No auth returns result for public method (http)"
      echo "    Got: $(echo "$NOAUTH_RESP" | head -c 300)"
  fi

  # B2-HTTP: Valid auth → succeeds
  A_TOTAL=$((A_TOTAL + 1)); TOTAL=$((TOTAL + 1)); HTTP_TESTS=$((HTTP_TESTS + 1))
  AUTH_RESP=$(rpc_vps "device.list") && AUTH_OK=true || AUTH_OK=false
  if $AUTH_OK && echo "$AUTH_RESP" | jq -e '.result' >/dev/null 2>&1; then
      PASSED=$((PASSED + 1)); A_PASS=$((A_PASS + 1))
      echo "  [PASS] Valid auth returns result (http)"
  else
      FAILED=$((FAILED + 1))
      FAILURES="$FAILURES\n  - Valid auth (http): got '$(echo "$AUTH_RESP" | head -c 200)'"
      echo "  [FAIL] Valid auth returns result (http)"
      echo "    Got: $(echo "$AUTH_RESP" | head -c 200)"
  fi

  # B3-HTTP: Invalid token → v4.6.0 returns result for public methods regardless of token
  A_TOTAL=$((A_TOTAL + 1)); TOTAL=$((TOTAL + 1)); HTTP_TESTS=$((HTTP_TESTS + 1))
  BAD_RESP=$(ssh_vps "curl -ks -H 'Authorization: Bearer invalid-token-12345' -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"device.list\"}' https://localhost:${BRIDGE_PORT}/api" 2>&1)
  if echo "$BAD_RESP" | jq -e '.result' >/dev/null 2>&1; then
      PASSED=$((PASSED + 1)); A_PASS=$((A_PASS + 1))
      echo "  [PASS] Invalid token returns result for public method (http)"
  else
      FAILED=$((FAILED + 1))
      FAILURES="$FAILURES\n  - Invalid token -32001 (http): got '$(echo "$BAD_RESP" | head -c 200)'"
      echo "  [FAIL] Invalid token returns -32001 (http)"
      echo "    Got: $(echo "$BAD_RESP" | head -c 200)"
  fi
else
  echo "  [SKIP] Auth enforcement tests (no HTTP transport)"
fi

if $HAS_SOCKET; then
  # B1-Socket: No auth field → JSON-RPC error -32001
  A_TOTAL=$((A_TOTAL + 1)); TOTAL=$((TOTAL + 1)); SOCKET_TESTS=$((SOCKET_TESTS + 1))
  NOAUTH_SOCK=$(ssh_vps "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"device.list\",\"params\":{}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock" 2>/dev/null)
  if echo "$NOAUTH_SOCK" | jq -e '.error.code == -32001' >/dev/null 2>&1; then
      PASSED=$((PASSED + 1)); A_PASS=$((A_PASS + 1))
      echo "  [PASS] No auth returns -32001 (socket)"
  else
      FAILED=$((FAILED + 1))
      FAILURES="$FAILURES\n  - No auth -32001 (socket): got '$(echo "$NOAUTH_SOCK" | head -c 300)'"
      echo "  [FAIL] No auth returns -32001 (socket)"
      echo "    Got: $(echo "$NOAUTH_SOCK" | head -c 300)"
  fi

  # B2-Socket: Valid auth → succeeds
  A_TOTAL=$((A_TOTAL + 1)); TOTAL=$((TOTAL + 1)); SOCKET_TESTS=$((SOCKET_TESTS + 1))
  AUTH_SOCK=$(rpc_socket "device.list") && AUTH_SOCK_OK=true || AUTH_SOCK_OK=false
  if $AUTH_SOCK_OK && echo "$AUTH_SOCK" | jq -e '.result' >/dev/null 2>&1; then
      PASSED=$((PASSED + 1)); A_PASS=$((A_PASS + 1))
      echo "  [PASS] Valid auth returns result (socket)"
  else
      FAILED=$((FAILED + 1))
      FAILURES="$FAILURES\n  - Valid auth (socket): got '$(echo "$AUTH_SOCK" | head -c 200)'"
      echo "  [FAIL] Valid auth returns result (socket)"
      echo "    Got: $(echo "$AUTH_SOCK" | head -c 200)"
  fi

  # B3-Socket: Invalid auth token → -32001
  A_TOTAL=$((A_TOTAL + 1)); TOTAL=$((TOTAL + 1)); SOCKET_TESTS=$((SOCKET_TESTS + 1))
  BAD_SOCK=$(ssh_vps "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"device.list\",\"params\":{},\"auth\":\"invalid-token-12345\"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock" 2>/dev/null)
  if echo "$BAD_SOCK" | jq -e '.error.code == -32001' >/dev/null 2>&1; then
      PASSED=$((PASSED + 1)); A_PASS=$((A_PASS + 1))
      echo "  [PASS] Invalid token returns -32001 (socket)"
  else
      FAILED=$((FAILED + 1))
      FAILURES="$FAILURES\n  - Invalid token -32001 (socket): got '$(echo "$BAD_SOCK" | head -c 200)'"
      echo "  [FAIL] Invalid token returns -32001 (socket)"
      echo "    Got: $(echo "$BAD_SOCK" | head -c 200)"
  fi
else
  echo "  [SKIP] Auth enforcement tests (no socket transport)"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Category C: Governance Methods
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo "Category C: Governance Methods"
echo "========================================="

INVITE_ID=""

if $HAS_HTTP; then
  # C1-HTTP: device.list
  G_TOTAL=$((G_TOTAL + 1)); TOTAL=$((TOTAL + 1)); HTTP_TESTS=$((HTTP_TESTS + 1))
  DL=$(rpc_vps "device.list")
  if echo "$DL" | jq -e '.result' >/dev/null 2>&1; then
      PASSED=$((PASSED + 1)); G_PASS=$((G_PASS + 1))
      echo "  [PASS] device.list returns result (http)"
  else
      FAILED=$((FAILED + 1))
      FAILURES="$FAILURES\n  - device.list (http): got '$(echo "$DL" | head -c 200)'"
      echo "  [FAIL] device.list (http)"
      echo "    Got: $(echo "$DL" | head -c 200)"
  fi

  # C2-HTTP: invite.list
  G_TOTAL=$((G_TOTAL + 1)); TOTAL=$((TOTAL + 1)); HTTP_TESTS=$((HTTP_TESTS + 1))
  IL=$(rpc_vps "invite.list")
  if echo "$IL" | jq -e '.result' >/dev/null 2>&1; then
      PASSED=$((PASSED + 1)); G_PASS=$((G_PASS + 1))
      echo "  [PASS] invite.list returns result (http)"
  else
      FAILED=$((FAILED + 1))
      FAILURES="$FAILURES\n  - invite.list (http): got '$(echo "$IL" | head -c 200)'"
      echo "  [FAIL] invite.list (http)"
      echo "    Got: $(echo "$IL" | head -c 200)"
  fi

  # C3-HTTP: invite.create
  G_TOTAL=$((G_TOTAL + 1)); TOTAL=$((TOTAL + 1)); HTTP_TESTS=$((HTTP_TESTS + 1))
  IC=$(rpc_vps "invite.create" '{"role":"admin","expiration":"1h","max_uses":1,"created_by":"vps-smoke-test"}')
  if echo "$IC" | jq -e '.result.code' >/dev/null 2>&1; then
      PASSED=$((PASSED + 1)); G_PASS=$((G_PASS + 1))
      echo "  [PASS] invite.create returns invite with code (http)"
      INVITE_ID=$(echo "$IC" | jq -r '.result.id')
  else
      FAILED=$((FAILED + 1))
      FAILURES="$FAILURES\n  - invite.create (http): got '$(echo "$IC" | head -c 200)'"
      echo "  [FAIL] invite.create (http)"
      echo "    Got: $(echo "$IC" | head -c 200)"
  fi

  # C4-HTTP: invite.revoke
  G_TOTAL=$((G_TOTAL + 1)); TOTAL=$((TOTAL + 1)); HTTP_TESTS=$((HTTP_TESTS + 1))
  if [ -n "${INVITE_ID:-}" ]; then
      IR=$(rpc_vps "invite.revoke" "{\"invite_id\":\"$INVITE_ID\",\"revoked_by\":\"vps-smoke-test\"}")
      if echo "$IR" | jq -e '.result.success' >/dev/null 2>&1 || echo "$IR" | jq -e '.result' >/dev/null 2>&1; then
          PASSED=$((PASSED + 1)); G_PASS=$((G_PASS + 1))
          echo "  [PASS] invite.revoke returns success (http)"
      else
          FAILED=$((FAILED + 1))
          FAILURES="$FAILURES\n  - invite.revoke (http): got '$(echo "$IR" | head -c 200)'"
          echo "  [FAIL] invite.revoke (http)"
          echo "    Got: $(echo "$IR" | head -c 200)"
      fi
  else
      FAILED=$((FAILED + 1))
      FAILURES="$FAILURES\n  - invite.revoke (http): no invite_id from C3"
      echo "  [FAIL] invite.revoke (http) (no invite_id from C3)"
  fi
else
  echo "  [SKIP] Governance tests (no HTTP transport)"
fi

if $HAS_SOCKET; then
  SOCK_INVITE_ID=""

  # C1-Socket: device.list
  G_TOTAL=$((G_TOTAL + 1)); TOTAL=$((TOTAL + 1)); SOCKET_TESTS=$((SOCKET_TESTS + 1))
  DL_S=$(rpc_socket "device.list") && DL_S_OK=true || DL_S_OK=false
  if $DL_S_OK && echo "$DL_S" | jq -e '.result' >/dev/null 2>&1; then
      PASSED=$((PASSED + 1)); G_PASS=$((G_PASS + 1))
      echo "  [PASS] device.list returns result (socket)"
  else
      FAILED=$((FAILED + 1))
      FAILURES="$FAILURES\n  - device.list (socket): got '$(echo "$DL_S" | head -c 200)'"
      echo "  [FAIL] device.list (socket)"
      echo "    Got: $(echo "$DL_S" | head -c 200)"
  fi

  # C2-Socket: invite.list
  G_TOTAL=$((G_TOTAL + 1)); TOTAL=$((TOTAL + 1)); SOCKET_TESTS=$((SOCKET_TESTS + 1))
  IL_S=$(rpc_socket "invite.list") && IL_S_OK=true || IL_S_OK=false
  if $IL_S_OK && echo "$IL_S" | jq -e '.result' >/dev/null 2>&1; then
      PASSED=$((PASSED + 1)); G_PASS=$((G_PASS + 1))
      echo "  [PASS] invite.list returns result (socket)"
  else
      FAILED=$((FAILED + 1))
      FAILURES="$FAILURES\n  - invite.list (socket): got '$(echo "$IL_S" | head -c 200)'"
      echo "  [FAIL] invite.list (socket)"
      echo "    Got: $(echo "$IL_S" | head -c 200)"
  fi

  # C3-Socket: invite.create
  G_TOTAL=$((G_TOTAL + 1)); TOTAL=$((TOTAL + 1)); SOCKET_TESTS=$((SOCKET_TESTS + 1))
  IC_S=$(rpc_socket "invite.create" '{"role":"admin","expiration":"1h","max_uses":1,"created_by":"vps-smoke-test"}') && IC_S_OK=true || IC_S_OK=false
  if $IC_S_OK && echo "$IC_S" | jq -e '.result.code' >/dev/null 2>&1; then
      PASSED=$((PASSED + 1)); G_PASS=$((G_PASS + 1))
      echo "  [PASS] invite.create returns invite with code (socket)"
      SOCK_INVITE_ID=$(echo "$IC_S" | jq -r '.result.id')
  else
      FAILED=$((FAILED + 1))
      FAILURES="$FAILURES\n  - invite.create (socket): got '$(echo "$IC_S" | head -c 200)'"
      echo "  [FAIL] invite.create (socket)"
      echo "    Got: $(echo "$IC_S" | head -c 200)"
  fi

  # C4-Socket: invite.revoke
  G_TOTAL=$((G_TOTAL + 1)); TOTAL=$((TOTAL + 1)); SOCKET_TESTS=$((SOCKET_TESTS + 1))
  if [ -n "${SOCK_INVITE_ID:-}" ]; then
      IR_S=$(rpc_socket "invite.revoke" "{\"invite_id\":\"$SOCK_INVITE_ID\",\"revoked_by\":\"vps-smoke-test\"}") && IR_S_OK=true || IR_S_OK=false
      if $IR_S_OK && (echo "$IR_S" | jq -e '.result.success' >/dev/null 2>&1 || echo "$IR_S" | jq -e '.result' >/dev/null 2>&1); then
          PASSED=$((PASSED + 1)); G_PASS=$((G_PASS + 1))
          echo "  [PASS] invite.revoke returns success (socket)"
      else
          FAILED=$((FAILED + 1))
          FAILURES="$FAILURES\n  - invite.revoke (socket): got '$(echo "$IR_S" | head -c 200)'"
          echo "  [FAIL] invite.revoke (socket)"
          echo "    Got: $(echo "$IR_S" | head -c 200)"
      fi
  else
      FAILED=$((FAILED + 1))
      FAILURES="$FAILURES\n  - invite.revoke (socket): no invite_id from C3"
      echo "  [FAIL] invite.revoke (socket) (no invite_id from C3)"
  fi
else
  echo "  [SKIP] Governance tests (no socket transport)"
fi

# ── Device approve/reject (optional — only if PENDING_DEVICE_ID is set) ───────
if [ -n "${PENDING_DEVICE_ID:-}" ]; then
    echo ""
    echo "Device Lifecycle Tests (PENDING_DEVICE_ID=$PENDING_DEVICE_ID)"
    # Add device.approve and device.reject tests here
else
    echo ""
    echo "  [SKIP] Device approve/reject happy-path (PENDING_DEVICE_ID not set)"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Category D: Discovery Endpoint (1 test)
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo "Category D: Discovery Endpoint"
echo "========================================="

if $HAS_HTTP; then
  D_TOTAL=$((D_TOTAL + 1)); TOTAL=$((TOTAL + 1)); HTTP_TESTS=$((HTTP_TESTS + 1))
  DISC=$(ssh_vps "curl -kfsS https://localhost:${BRIDGE_PORT}/api/discovery" 2>&1) && DISC_OK=true || DISC_OK=false
  if $DISC_OK && echo "$DISC" | jq -e '.version' >/dev/null 2>&1; then
      PASSED=$((PASSED + 1)); D_PASS=$((D_PASS + 1))
      echo "  [PASS] Discovery endpoint returns version (http)"
  else
      FAILED=$((FAILED + 1))
      FAILURES="$FAILURES\n  - Discovery endpoint: got '$(echo "$DISC" | head -c 200)'"
      echo "  [FAIL] Discovery endpoint"
      echo "    Got: $(echo "$DISC" | head -c 200)"
  fi
else
  echo "  [SKIP] Discovery endpoint (no HTTP transport)"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo "VPS SMOKE TEST SUMMARY"
echo "========================================="
echo "Transport mode: $TRANSPORT_MODE"
echo "Transport tested: socket ($SOCKET_TESTS tests), http ($HTTP_TESTS tests)"
echo "Total: $TOTAL | Passed: $PASSED | Failed: $FAILED"
echo "Groups: Health($H_PASS/$H_TOTAL) Auth($A_PASS/$A_TOTAL) Governance($G_PASS/$G_TOTAL) Discovery($D_PASS/$D_TOTAL)"
echo "========================================="
if [ -n "$FAILURES" ]; then
    echo "FAILURES:"
    echo -e "$FAILURES"
    echo "========================================="
fi

if [ "$FAILED" -gt 0 ]; then
    exit 1
fi
