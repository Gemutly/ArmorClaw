#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# Matrix Plane Integration Tests
#
# Tests Matrix messaging via direct Conduit API (curl over SSH) for messaging,
# and matrix-commander for file upload / assistant round-trip.
# Covers login, sync, send/verify, round-trip, file upload, and optional
# assistant round-trip.
#
# Usage:
#   MATRIX_USER=@user:server MATRIX_PASSWORD=xxx ROOM_ID=!room:server \
#     bash tests/test-matrix-plane.sh
#
# Optional: ASSISTANT_ROOM_ID=!assistant:server  (enables Category D)
# ──────────────────────────────────────────────────────────────────────────────

# Auto-source .env for VPS connection details
set -a
source .env 2>/dev/null || true
set +a

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/transport.sh"
detect_transport

# ── Environment variables ─────────────────────────────────────────────────────
: "${MATRIX_PORT:=6167}"
: "${HOMESERVER:=https://${VPS_IP:-localhost}:${MATRIX_PORT}}"
: "${MATRIX_USER:?missing MATRIX_USER (pass via CLI or .env)}"
: "${MATRIX_PASSWORD:?missing MATRIX_PASSWORD (pass via CLI or .env)}"
: "${ROOM_ID:?missing ROOM_ID (pass via CLI or .env)}"
: "${MATRIX_STORE:=$HOME/.matrix-commander}"
: "${MC_MODE:=docker}"
: "${CI_MODE:=0}"
: "${VPS_IP:?missing VPS_IP (pass via CLI or .env)}"
: "${VPS_USER:=root}"
: "${SSH_KEY_PATH:=$HOME/.ssh/openclaw_win}"

# ── Prerequisite check ────────────────────────────────────────────────────────
if [[ "$MC_MODE" == "docker" ]]; then
    command -v docker >/dev/null 2>&1 || { echo "FAIL: docker required (MC_MODE=docker)"; exit 1; }
else
    command -v matrix-commander >/dev/null 2>&1 || { echo "FAIL: matrix-commander required (MC_MODE=local)"; exit 1; }
fi
command -v jq >/dev/null 2>&1 || { echo "FAIL: jq required for API tests"; exit 1; }

# ── matrix-commander wrapper ──────────────────────────────────────────────────
mc() {
    if [[ "$MC_MODE" == "docker" ]]; then
        docker run --rm \
            -v "$MATRIX_STORE:/data" \
            matrixcommander/matrix-commander "$@"
    else
        matrix-commander "$@"
    fi
}

# ── SSH / Matrix API helpers ─────────────────────────────────────────────────
ssh_run() {
    ssh -i "${SSH_KEY_PATH}" -o ConnectTimeout=10 -o StrictHostKeyChecking=no "${VPS_USER}@${VPS_IP}" "$*"
}

MATRIX_TOKEN=""
MATRIX_USER_ID=""

matrix_login() {
    local resp
    resp=$(ssh_run "curl -s -X POST 'http://localhost:${MATRIX_PORT}/_matrix/client/v3/login' -H 'Content-Type: application/json' -d '{\"type\":\"m.login.password\",\"user\":\"${MATRIX_USER}\",\"password\":\"${MATRIX_PASSWORD}\"}'")
    MATRIX_TOKEN=$(echo "$resp" | jq -r '.access_token // empty')
    MATRIX_USER_ID=$(echo "$resp" | jq -r '.user_id // empty')
}

matrix_send() {
    local room_id="$1" message="$2"
    local txn_id
    txn_id=$(openssl rand -hex 8)
    ssh_run "curl -s -X PUT 'http://localhost:${MATRIX_PORT}/_matrix/client/v3/rooms/${room_id}/send/m.room.message/${txn_id}' -H 'Content-Type: application/json' -H 'Authorization: Bearer ${MATRIX_TOKEN}' -d '{\"msgtype\":\"m.text\",\"body\":\"${message}\"}'" | jq -r '.event_id // empty'
}

matrix_receive() {
    local room_id="$1"
    ssh_run "curl -s 'http://localhost:${MATRIX_PORT}/_matrix/client/v3/rooms/${room_id}/messages?dir=b&limit=20' -H 'Authorization: Bearer ${MATRIX_TOKEN}'"
}

# ── Counters ──────────────────────────────────────────────────────────────────
TOTAL=0 PASSED=0 FAILED=0
LS_TOTAL=0 LS_PASS=0  # Login & Sync
M_TOTAL=0 M_PASS=0    # Messaging
F_TOTAL=0 F_PASS=0    # File Upload
AS_TOTAL=0 AS_PASS=0  # Assistant
FAILURES=""

# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo "Category A: Login & Sync"
echo "========================================="

# A1: Login via curl (matrix-commander requires interactive input)
LS_TOTAL=$((LS_TOTAL + 1)); TOTAL=$((TOTAL + 1))
echo "  [1/N] Logging in via Matrix API..."
matrix_login
if [[ -n "$MATRIX_TOKEN" ]]; then
    PASSED=$((PASSED + 1)); LS_PASS=$((LS_PASS + 1))
    echo "  [PASS] Login succeeds (user: $MATRIX_USER_ID)"
else
    FAILED=$((FAILED + 1))
    echo "  [FAIL] Login fails"
fi

# A2: Sync via curl
LS_TOTAL=$((LS_TOTAL + 1)); TOTAL=$((TOTAL + 1))
echo "  [2/N] Syncing..."
SYNC_RESP=$(ssh_run "curl -s 'http://localhost:${MATRIX_PORT}/_matrix/client/v3/sync?timeout=0' -H 'Authorization: Bearer ${MATRIX_TOKEN}'" 2>&1)
if echo "$SYNC_RESP" | jq -e '.rooms' >/dev/null 2>&1 || echo "$SYNC_RESP" | jq -e '.next_batch' >/dev/null 2>&1; then
    PASSED=$((PASSED + 1)); LS_PASS=$((LS_PASS + 1))
    echo "  [PASS] Sync succeeds"
else
    FAILED=$((FAILED + 1))
    echo "  [FAIL] Sync fails"
    echo "    Got: $(echo "$SYNC_RESP" | head -c 200)"
fi

# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo "Category B: Messaging (Direct API)"
echo "========================================="

# B1: Login via curl
M_TOTAL=$((M_TOTAL + 1)); TOTAL=$((TOTAL + 1))
echo "  [3/N] Logging in via Matrix API..."
matrix_login
if [[ -n "$MATRIX_TOKEN" ]]; then
    PASSED=$((PASSED + 1)); M_PASS=$((M_PASS + 1))
    echo "  [PASS] Login via curl (user: $MATRIX_USER_ID)"
else
    FAILED=$((FAILED + 1))
    echo "  [FAIL] Login via curl failed"
fi

# B2: Send message with unique token
UNIQUE_TOKEN="ARMORCLAW-$(openssl rand -hex 4)"
M_TOTAL=$((M_TOTAL + 1)); TOTAL=$((TOTAL + 1))
echo "  [4/N] Sending message with token: $UNIQUE_TOKEN"
EVENT_ID=$(matrix_send "$ROOM_ID" "$UNIQUE_TOKEN test message from armorclaw smoke test")
if [[ -n "$EVENT_ID" ]]; then
    PASSED=$((PASSED + 1)); M_PASS=$((M_PASS + 1))
    echo "  [PASS] Send message (event_id: $EVENT_ID)"
else
    FAILED=$((FAILED + 1))
    echo "  [FAIL] Send message failed"
fi

# B3: Poll for message via matrix_receive
M_TOTAL=$((M_TOTAL + 1)); TOTAL=$((TOTAL + 1))
echo "  [5/N] Verifying message landed..."
FOUND=false
for attempt in 1 2 3 4 5 6; do
    sleep 5
    RECENT=$(matrix_receive "$ROOM_ID" || true)
    if echo "$RECENT" | grep -q "$UNIQUE_TOKEN"; then
        FOUND=true
        break
    fi
done
if $FOUND; then
    PASSED=$((PASSED + 1)); M_PASS=$((M_PASS + 1))
    echo "  [PASS] Message verified in room"
else
    FAILED=$((FAILED + 1))
    echo "  [FAIL] Message not found after 30s"
fi

# B4: Round-trip test (send different token, verify received)
RT_TOKEN="ARMORCLAW-RT-$(openssl rand -hex 4)"
M_TOTAL=$((M_TOTAL + 1)); TOTAL=$((TOTAL + 1))
echo "  [6/N] Round-trip test with token: $RT_TOKEN"
RT_EVENT=$(matrix_send "$ROOM_ID" "$RT_TOKEN round-trip test")
RT_FOUND=false
for attempt in 1 2 3 4 5 6; do
    sleep 5
    RT_RECENT=$(matrix_receive "$ROOM_ID" || true)
    if echo "$RT_RECENT" | grep -q "$RT_TOKEN"; then
        RT_FOUND=true
        break
    fi
done
if $RT_FOUND; then
    PASSED=$((PASSED + 1)); M_PASS=$((M_PASS + 1))
    echo "  [PASS] Round-trip verified"
else
    FAILED=$((FAILED + 1))
    echo "  [FAIL] Round-trip not verified after 30s"
fi

# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo "Category C: File Upload (Direct API)"
echo "========================================="

F_TOTAL=$((F_TOTAL + 1)); TOTAL=$((TOTAL + 1))
TMPFILE=$(mktemp /tmp/armorclaw-smoke-XXXXXX.txt)
UPLOAD_CONTENT="ArmorClaw smoke test file upload $(date)"
echo "$UPLOAD_CONTENT" > "$TMPFILE"
echo "  [7/N] Uploading file via Matrix media API..."
UPLOAD_RESP=$(ssh_run "curl -s -X POST 'http://localhost:${MATRIX_PORT}/_matrix/media/v3/upload' -H 'Authorization: Bearer ${MATRIX_TOKEN}' -H 'Content-Type: text/plain' -d '${UPLOAD_CONTENT}'" 2>&1)
MXC_URI=$(echo "$UPLOAD_RESP" | jq -r '.content_uri // empty')
if [[ -n "$MXC_URI" ]]; then
    PASSED=$((PASSED + 1)); F_PASS=$((F_PASS + 1))
    echo "  [PASS] File upload succeeds (mxc: $MXC_URI)"
else
    FAILED=$((FAILED + 1))
    echo "  [FAIL] File upload fails"
    echo "    Got: $(echo "$UPLOAD_RESP" | head -c 200)"
fi
rm -f "$TMPFILE"

# ══════════════════════════════════════════════════════════════════════════════
echo ""
if [ -n "${ASSISTANT_ROOM_ID:-}" ]; then
    echo "========================================="
    echo "Category D: Assistant Round-Trip"
    echo "========================================="

    AS_TOTAL=$((AS_TOTAL + 1)); TOTAL=$((TOTAL + 1))
    echo "  [8/N] Sending assistant prompt..."
    mc --room "$ASSISTANT_ROOM_ID" --message "Reply with exactly: SMOKE_TEST_OK" 2>&1 || true
    ASSISTANT_FOUND=false
    for attempt in $(seq 1 12); do
        sleep 5
        AS_RECENT=$(mc --room "$ASSISTANT_ROOM_ID" --sync off --timeout 5000 2>&1 || true)
        if echo "$AS_RECENT" | grep -q "SMOKE_TEST_OK"; then
            ASSISTANT_FOUND=true
            break
        fi
    done
    if $ASSISTANT_FOUND; then
        PASSED=$((PASSED + 1)); AS_PASS=$((AS_PASS + 1))
        echo "  [PASS] Assistant responded with SMOKE_TEST_OK"
    else
        FAILED=$((FAILED + 1))
        echo "  [FAIL] Assistant did not respond within 60s"
    fi
else
    echo "  [SKIP] Assistant round-trip (ASSISTANT_ROOM_ID not set)"
fi

# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "========================================="
echo "MATRIX PLANE TEST SUMMARY"
echo "========================================="
echo "Total: $TOTAL | Passed: $PASSED | Failed: $FAILED"
echo "Groups: LoginSync($LS_PASS/$LS_TOTAL) Messaging($M_PASS/$M_TOTAL) FileUpload($F_PASS/$F_TOTAL) Assistant($AS_PASS/$AS_TOTAL)"
echo "========================================="
if [ -n "$FAILURES" ]; then
    echo "FAILURES:"
    echo -e "$FAILURES"
    echo "========================================="
fi

if [ "$FAILED" -gt 0 ]; then
    exit 1
fi
