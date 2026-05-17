#!/usr/bin/env bash
# check-beato-runtime.sh — Lightweight BEATO runtime health summary
#
# Reports PASS/WARN/FAIL for each BEATO pillar by SSHing to the VPS
# and checking service status. Designed as an operational tool (not CI).
#
# Usage:
#   bash tests/ops/check-beato-runtime.sh
#
# Exit codes:
#   0 — all checks PASS
#   1 — at least one FAIL
#   2 — no FAILs but at least one WARN

set -euo pipefail

# ── Source shared helpers ─────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../lib/load_env.sh"

# ── Colors ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# ── Counters ──────────────────────────────────────────────────────────────────
BEATO_PASS=0
BEATO_WARN=0
BEATO_FAIL=0

# ── Output helpers ────────────────────────────────────────────────────────────
check_pass() { echo -e "  ${GREEN}[PASS]${NC} $1"; ((BEATO_PASS++)) || true; }
check_warn() { echo -e "  ${YELLOW}[WARN]${NC} $1"; ((BEATO_WARN++)) || true; }
check_fail() { echo -e "  ${RED}[FAIL]${NC} $1"; ((BEATO_FAIL++)) || true; }

# ── Secret scrubbing ──────────────────────────────────────────────────────────
# Strips any value that looks like a token/key/password from output
scrub_secrets() {
  sed -E \
    -e 's/(token["[:space:]]*[:=]["[:space:]]*)[^"[:space:]]+/\1***REDACTED***/gi' \
    -e 's/(key["[:space:]]*[:=]["[:space:]]*)[^"[:space:]]+/\1***REDACTED***/gi' \
    -e 's/(password["[:space:]]*[:=]["[:space:]]*)[^"[:space:]]+/\1***REDACTED***/gi' \
    -e 's/(secret["[:space:]]*[:=]["[:space:]]*)[^"[:space:]]+/\1***REDACTED***/gi' \
    -e 's/sk-[a-zA-Z0-9]{10,}/***REDACTED***/g' \
    -e 's/[a-f0-9]{32,}/***REDACTED***/g'
}

# ── Header ────────────────────────────────────────────────────────────────────
echo "========================================"
echo " ArmorClaw BEATO Runtime Health"
echo " $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
echo " VPS: ${VPS_IP}"
echo "========================================"
echo ""

# ── Pre-flight: SSH connectivity ─────────────────────────────────────────────
echo -e "${CYAN}[CONNECT]${NC} Testing SSH connectivity..."
if ssh_vps "echo ok" >/dev/null 2>&1; then
  check_pass "SSH connection to ${VPS_IP}"
else
  check_fail "Cannot SSH to ${VPS_IP} — aborting"
  echo ""
  echo "========================================"
  echo " Summary: PASS=${BEATO_PASS} WARN=${BEATO_WARN} FAIL=${BEATO_FAIL}"
  echo -e " ${RED}Status: UNREACHABLE${NC}"
  echo "========================================"
  exit 1
fi
echo ""

# ═════════════════════════════════════════════════════════════════════════════
# B — Bridge Health
# ═════════════════════════════════════════════════════════════════════════════
echo -e "${CYAN}[BRIDGE]${NC} Checking Bridge health..."

# Check via Unix socket
BRIDGE_RESP=$(ssh_vps "curl -sf --unix-socket /run/armorclaw/bridge.sock http://localhost/health 2>/dev/null" 2>/dev/null || echo "")
if [[ "$BRIDGE_RESP" == *"ok"* || "$BRIDGE_RESP" == *"healthy"* || "$BRIDGE_RESP" == *"running"* ]]; then
  check_pass "Bridge socket health: responding"
elif [[ -n "$BRIDGE_RESP" ]]; then
  # Got a response but not "ok" — check HTTP too
  check_warn "Bridge socket responded but status unclear: $(echo "$BRIDGE_RESP" | scrub_secrets | head -c 120)"
else
  # Fallback: check HTTP endpoint
  HTTP_RESP=$(ssh_vps "curl -sf --max-time 3 http://localhost:${BRIDGE_PORT}/health 2>/dev/null || curl -ksf --max-time 3 https://localhost:${BRIDGE_PORT}/health 2>/dev/null" 2>/dev/null || echo "")
  if [[ "$HTTP_RESP" == *"ok"* || "$HTTP_RESP" == *"healthy"* ]]; then
    check_pass "Bridge HTTP health: responding"
  else
    check_fail "Bridge not responding (socket and HTTP both failed)"
  fi
fi
echo ""

# ═════════════════════════════════════════════════════════════════════════════
# E — Email Queue Status
# ═════════════════════════════════════════════════════════════════════════════
echo -e "${CYAN}[EMAIL]${NC} Checking Email queue..."

# Try RPC email.list_pending via socket
EMAIL_RESP=$(ssh_vps "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"email.list_pending\"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock 2>/dev/null | head -c 512" 2>/dev/null || echo "")
if [[ -n "$EMAIL_RESP" ]]; then
  # Check for error vs success
  if echo "$EMAIL_RESP" | grep -q '"error"'; then
    check_warn "Email RPC returned error (may not be configured)"
  else
    check_pass "Email queue: RPC responding"
  fi
else
  # Fallback: check if postfix is running
  POSTFIX=$(ssh_vps "systemctl is-active postfix 2>/dev/null || echo inactive" 2>/dev/null || echo "unknown")
  if [[ "$POSTFIX" == "active" ]]; then
    check_pass "Email: Postfix service active"
  else
    check_warn "Email: RPC unreachable, Postfix not active"
  fi
fi
echo ""

# ═════════════════════════════════════════════════════════════════════════════
# A — (Matrix) Conduit Status
# ═════════════════════════════════════════════════════════════════════════════
echo -e "${CYAN}[MATRIX]${NC} Checking Matrix/Conduit status..."

# Try RPC matrix.status via socket
MATRIX_RPC=$(ssh_vps "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"matrix.status\"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock 2>/dev/null | head -c 512" 2>/dev/null || echo "")
if [[ -n "$MATRIX_RPC" ]] && echo "$MATRIX_RPC" | grep -qi '"connected"'; then
  if echo "$MATRIX_RPC" | grep -qi '"connected":\s*true\|"connected"\s*:\s*true'; then
    check_pass "Matrix RPC: connected=true"
  else
    check_warn "Matrix RPC: connected=false"
  fi
else
  # Fallback: HTTP check on Conduit
  CONDUIT=$(ssh_vps "curl -sf --max-time 3 http://localhost:${MATRIX_PORT}/_matrix/client/versions 2>/dev/null" 2>/dev/null || echo "")
  if [[ -n "$CONDUIT" ]]; then
    check_pass "Matrix Conduit: HTTP responding on port ${MATRIX_PORT}"
  else
    check_fail "Matrix Conduit not responding (RPC and HTTP both failed)"
  fi
fi
echo ""

# ═════════════════════════════════════════════════════════════════════════════
# T — (Sidecar) Python Sidecar Status
# ═════════════════════════════════════════════════════════════════════════════
echo -e "${CYAN}[SIDECAR]${NC} Checking Python sidecar..."

SIDECAR_STATUS=$(ssh_vps "docker ps --filter name=sidecar-office --format '{{.Status}}' 2>/dev/null" 2>/dev/null || echo "")
if [[ "$SIDECAR_STATUS" == *"Up"* ]]; then
  # Check uptime — warn if less than 2 minutes (might be crash-looping)
  UPTIME_SEC=$(echo "$SIDECAR_STATUS" | grep -oP 'Up \K([0-9]+)(?= second)' 2>/dev/null || echo "999")
  UPTIME_MIN=$(echo "$SIDECAR_STATUS" | grep -oP 'Up \K([0-9]+)(?= minute)' 2>/dev/null || echo "999")
  if [[ "$UPTIME_SEC" != "999" && "$UPTIME_SEC" -lt 120 ]]; then
    check_warn "Python sidecar: Up but only ${UPTIME_SEC}s (possible crash loop)"
  else
    check_pass "Python sidecar: Running (${SIDECAR_STATUS})"
  fi
elif [[ "$SIDECAR_STATUS" == *"Restarting"* ]]; then
  check_fail "Python sidecar: Restarting (crash loop detected)"
elif [[ -z "$SIDECAR_STATUS" ]]; then
  check_warn "Python sidecar: Container not found (may not be deployed)"
else
  check_warn "Python sidecar: ${SIDECAR_STATUS}"
fi
echo ""

# ═════════════════════════════════════════════════════════════════════════════
# O — (Jetski) Browser Sidecar + Docker Restart Counts + Disk + RAM
# ═════════════════════════════════════════════════════════════════════════════

# ── Jetski ────────────────────────────────────────────────────────────────────
echo -e "${CYAN}[JETSKI]${NC} Checking Jetski browser sidecar..."

JETSKI_STATUS=$(ssh_vps "docker ps --filter name=jetski --format '{{.Status}}' 2>/dev/null" 2>/dev/null || echo "")
if [[ "$JETSKI_STATUS" == *"Up"* ]]; then
  check_pass "Jetski: Running (${JETSKI_STATUS})"
elif [[ "$JETSKI_STATUS" == *"Restarting"* ]]; then
  check_fail "Jetski: Restarting (crash loop)"
elif [[ -z "$JETSKI_STATUS" ]]; then
  check_warn "Jetski: Container not found (may not be deployed)"
else
  check_warn "Jetski: ${JETSKI_STATUS}"
fi
echo ""

# ── Docker Restart Counts ────────────────────────────────────────────────────
echo -e "${CYAN}[DOCKER]${NC} Checking container restart counts..."

CONTAINERS=("armorclaw" "armorclaw-jetski" "armorclaw-conduit" "armorclaw-sidecar-office")
for ctr in "${CONTAINERS[@]}"; do
  RESTARTS=$(ssh_vps "docker inspect --format '{{.RestartCount}}' ${ctr} 2>/dev/null || echo -1" 2>/dev/null || echo "-1")
  if [[ "$RESTARTS" == "-1" ]]; then
    check_warn "${ctr}: container not found"
  elif [[ "$RESTARTS" -eq 0 ]]; then
    check_pass "${ctr}: 0 restarts"
  elif [[ "$RESTARTS" -le 3 ]]; then
    check_warn "${ctr}: ${RESTARTS} restarts"
  else
    check_fail "${ctr}: ${RESTARTS} restarts (excessive)"
  fi
done
echo ""

# ── Disk Usage ────────────────────────────────────────────────────────────────
echo -e "${CYAN}[DISK]${NC} Checking disk usage..."

DISK_PCT=$(ssh_vps "df / --output=pcent 2>/dev/null | tail -1 | tr -d ' %'" 2>/dev/null || echo "0")
if [[ "$DISK_PCT" -ge 95 ]]; then
  check_fail "Disk: ${DISK_PCT}% used (critical)"
elif [[ "$DISK_PCT" -ge 80 ]]; then
  check_warn "Disk: ${DISK_PCT}% used"
else
  check_pass "Disk: ${DISK_PCT}% used"
fi
echo ""

# ── RAM Usage ─────────────────────────────────────────────────────────────────
echo -e "${CYAN}[RAM]${NC} Checking RAM usage..."

RAM_INFO=$(ssh_vps "free -m 2>/dev/null | awk '/Mem:/ {print \$2, \$3}'" 2>/dev/null || echo "0 0")
RAM_TOTAL=$(echo "$RAM_INFO" | awk '{print $1}')
RAM_USED=$(echo "$RAM_INFO" | awk '{print $2}')

if [[ "$RAM_TOTAL" -gt 0 ]]; then
  RAM_PCT=$(( RAM_USED * 100 / RAM_TOTAL ))
  if [[ "$RAM_PCT" -ge 95 ]]; then
    check_fail "RAM: ${RAM_PCT}% used (${RAM_USED}M / ${RAM_TOTAL}M)"
  elif [[ "$RAM_PCT" -ge 90 ]]; then
    check_warn "RAM: ${RAM_PCT}% used (${RAM_USED}M / ${RAM_TOTAL}M)"
  else
    check_pass "RAM: ${RAM_PCT}% used (${RAM_USED}M / ${RAM_TOTAL}M)"
  fi
else
  check_warn "RAM: Could not determine usage"
fi
echo ""

# ═════════════════════════════════════════════════════════════════════════════
# Summary
# ═════════════════════════════════════════════════════════════════════════════
echo "========================================"
echo " BEATO Runtime Summary"
echo "========================================"
echo -e " ${GREEN}PASS:${NC}  ${BEATO_PASS}"
echo -e " ${YELLOW}WARN:${NC}  ${BEATO_WARN}"
echo -e " ${RED}FAIL:${NC}  ${BEATO_FAIL}"
echo ""

if [[ $BEATO_FAIL -gt 0 ]]; then
  echo -e " ${RED}Status: UNHEALTHY${NC}"
  echo "========================================"
  exit 1
elif [[ $BEATO_WARN -gt 0 ]]; then
  echo -e " ${YELLOW}Status: DEGRADED${NC}"
  echo "========================================"
  exit 2
else
  echo -e " ${GREEN}Status: HEALTHY${NC}"
  echo "========================================"
  exit 0
fi
