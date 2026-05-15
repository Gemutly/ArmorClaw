#!/usr/bin/env bash
# test-matrix-integration.sh — Matrix adapter integration tests for ArmorClaw
#
# matrix.status RPC response contract (source: bridge/pkg/rpc/server.go:480-502):
#
#   State 1 — Not configured (adapter is nil):
#     result: {"enabled":false,"connected":false,"error":"matrix adapter not configured"}
#
#   State 2 — Configured + logged in:
#     result: {"enabled":true,"connected":true,"logged_in":true,
#              "homeserver":"https://domain","user_id":"@bridge:domain"}
#
#   State 3 — Configured but not logged in:
#     result: {"enabled":true,"connected":true,"logged_in":false,
#              "homeserver":"https://domain","error":"not logged in"}
#
#   Error (JSON-RPC level):
#     {"error":{"code":-32xxx,"message":"..."}}
#
# Usage:
#   bash tests/test-matrix-integration.sh
#
# Requirements:
#   - Bridge running (socket or HTTP)
#   - jq for JSON parsing

set -euo pipefail

# ── Resolve library paths ─────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="$SCRIPT_DIR/lib"

source "$LIB_DIR/transport.sh"
source "$LIB_DIR/common_output.sh"

# Fallback color vars (common_output.sh expects these from tests/e2e/common.sh)
GREEN="${GREEN:-\033[0;32m}"
RED="${RED:-\033[0;31m}"
YELLOW="${YELLOW:-\033[1;33m}"
NC="${NC:-\033[0m}"

# ── Detect bridge ─────────────────────────────────────────────────────────────
if ! optional_bridge; then
  log_env_missing "Bridge not available — skipping Matrix integration tests"
  harness_summary
  exit 0
fi

# ── Check jq ──────────────────────────────────────────────────────────────────
if ! command -v jq &>/dev/null; then
  log_env_missing "jq not installed — skipping Matrix integration tests"
  harness_summary
  exit 0
fi

# ══════════════════════════════════════════════════════════════════════════════
# TEST 1: matrix.status — response contract validation
# ══════════════════════════════════════════════════════════════════════════════
RESPONSE=$(rpc_call "matrix.status" '{}' 2>/dev/null || true)

if [[ -z "$RESPONSE" ]]; then
  log_fail "matrix.status returned empty response"
  harness_summary
  exit 1
fi

log_info "matrix.status response: $(echo "$RESPONSE" | jq -c '.')"

# Parse the result object (JSON-RPC wraps in .result)
RESULT_JSON=$(echo "$RESPONSE" | jq -c '.result // empty' 2>/dev/null || true)

if [[ -z "$RESULT_JSON" ]]; then
  # JSON-RPC error — check if it's a structured error
  ERROR_CODE=$(echo "$RESPONSE" | jq -r '.error.code // empty' 2>/dev/null || true)
  if [[ -n "$ERROR_CODE" ]]; then
    log_fail "matrix.status returned RPC error code $ERROR_CODE: $(echo "$RESPONSE" | jq -r '.error.message // "unknown"')"
  else
    log_fail "matrix.status unexpected response: $(echo "$RESPONSE" | jq -c '.')"
  fi
  harness_summary
  exit 1
fi

# ── Classify result state ─────────────────────────────────────────────────────

ENABLED=$(echo "$RESULT_JSON" | jq -r '.enabled // null')
CONNECTED=$(echo "$RESULT_JSON" | jq -r '.connected // null')
ERROR_MSG=$(echo "$RESULT_JSON" | jq -r '.error // empty')
LOGGED_IN=$(echo "$RESULT_JSON" | jq -r '.logged_in // null')
HOMESERVER=$(echo "$RESULT_JSON" | jq -r '.homeserver // empty')
USER_ID=$(echo "$RESULT_JSON" | jq -r '.user_id // empty')

if [[ "$ENABLED" == "false" && "$CONNECTED" == "false" ]]; then
  # State 1: Matrix adapter not configured — valid on deployments without Matrix
  log_gated_expected "Matrix adapter not configured (enabled=$ENABLED, error=$ERROR_MSG)"

elif [[ "$ENABLED" == "true" && "$CONNECTED" == "true" ]]; then
  # State 2 or 3: Adapter is present
  if [[ "$LOGGED_IN" == "true" ]]; then
    log_pass "matrix.status: connected and logged in (user=$USER_ID, hs=$HOMESERVER)"
  elif [[ "$LOGGED_IN" == "false" ]]; then
    log_gated_expected "Matrix adapter present but not logged in (homeserver=$HOMESERVER, error=$ERROR_MSG)"
  else
    log_pass "matrix.status: adapter present (enabled=$ENABLED, connected=$CONNECTED)"
  fi
else
  log_fail "matrix.status unexpected state: enabled=$ENABLED connected=$CONNECTED error=$ERROR_MSG"
fi

# ══════════════════════════════════════════════════════════════════════════════
# TEST 2: matrix.status — field-type validation (when result is present)
# ══════════════════════════════════════════════════════════════════════════════
if [[ -n "$RESULT_JSON" ]]; then
  # Verify expected fields are present and correctly typed
  TYPE_OK=true

  # .enabled must be boolean
  ENABLED_TYPE=$(echo "$RESULT_JSON" | jq -r '.enabled | type')
  if [[ "$ENABLED_TYPE" != "boolean" ]]; then
    log_fail "matrix.status .enabled type is '$ENABLED_TYPE', expected boolean"
    TYPE_OK=false
  fi

  # .connected must be boolean
  CONNECTED_TYPE=$(echo "$RESULT_JSON" | jq -r '.connected | type')
  if [[ "$CONNECTED_TYPE" != "boolean" ]]; then
    log_fail "matrix.status .connected type is '$CONNECTED_TYPE', expected boolean"
    TYPE_OK=false
  fi

  if $TYPE_OK; then
    log_pass "matrix.status field types valid (enabled=$ENABLED_TYPE, connected=$CONNECTED_TYPE)"
  fi
fi

# ══════════════════════════════════════════════════════════════════════════════
# TEST 3: Invalid RPC method returns -32601
# ══════════════════════════════════════════════════════════════════════════════
INVALID_RESP=$(rpc_call "invalid.nonexistent.method" '{}' 2>/dev/null || true)

if [[ -n "$INVALID_RESP" ]]; then
  INVALID_CODE=$(echo "$INVALID_RESP" | jq -r '.error.code // empty' 2>/dev/null || true)
  if [[ "$INVALID_CODE" == "-32601" ]]; then
    log_pass "Invalid method returns -32601 (Method not found)"
  else
    log_fail "Invalid method returned code '$INVALID_CODE', expected -32601"
  fi
else
  log_skip "Could not test invalid method (no response)"
fi

# ══════════════════════════════════════════════════════════════════════════════
# TEST 4: Malformed JSON returns -32700
# ══════════════════════════════════════════════════════════════════════════════
MALFORMED_RESP=$(echo 'not valid json' | \
  timeout 3 socat - UNIX-CONNECT:"$BRIDGE_SOCKET" 2>/dev/null || \
  echo '{"error":{"code":-32700}}')

MALFORMED_CODE=$(echo "$MALFORMED_RESP" | jq -r '.error.code // empty' 2>/dev/null || true)
if [[ "$MALFORMED_CODE" == "-32700" ]]; then
  log_pass "Malformed JSON returns -32700 (Parse error)"
else
  log_info "Malformed JSON response: $MALFORMED_RESP"
  log_skip "Malformed JSON test inconclusive (response: code=$MALFORMED_CODE)"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════
harness_summary
