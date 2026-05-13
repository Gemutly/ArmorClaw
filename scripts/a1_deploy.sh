#!/usr/bin/env bash
# a1_deploy.sh — Phase A1: Topology-aware deployment for ArmorClaw E2E
#
# Deploys ArmorClaw to VPS if not already running. Inspects Docker image
# to determine topology (single vs multi-service). Creates compose file
# based on inspection, starts containers, waits for readiness.

set -uo pipefail

_SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${_SCRIPT_DIR}/lib/contract.sh"
source "${_SCRIPT_DIR}/lib/tls.sh"
source "${_SCRIPT_DIR}/lib/probe.sh"

log_info "========================================="
log_info " Phase A1: Topology-Aware Deployment"
log_info "========================================="

FORCE_REDEPLOY="${FORCE_REDEPLOY:-0}"
DEPLOY_IMAGE="${DEPLOY_IMAGE:-mikegemut/armorclaw:latest}"

# ── A1.1: Verify VPS SSH ────────────────────────────────────────────────────
log_info "A1.1: Verifying SSH connectivity..."
if ! _contract_ssh_test; then
  log_fail "A1.1: SSH connectivity failed"
  exit 1
fi

# ── A1.2: Ensure Docker on VPS ──────────────────────────────────────────────
log_info "A1.2: Checking Docker on VPS..."
DOCKER_AVAILABLE=$(ssh_vps "command -v docker && docker info >/dev/null 2>&1 && echo DOCKER_OK || echo DOCKER_MISSING" 2>/dev/null)

if echo "$DOCKER_AVAILABLE" | grep -q "DOCKER_MISSING"; then
  log_fail "A1.2: Docker not available on VPS"
  _contract_save "a1_docker_check.txt" "Docker not available on VPS"
  exit 1
fi
log_pass "A1.2: Docker is available"

# ── A1.3: Resource preflight ────────────────────────────────────────────────
log_info "A1.3: Checking VPS resources..."
VPS_MEM=$(ssh_vps "free -m | awk '/Mem:/{print \$2}'" 2>/dev/null || echo "0")
VPS_DISK=$(ssh_vps "df -BG / | awk 'NR==2{print \$4}'" 2>/dev/null | tr -d 'G' || echo "0")
VPS_CPU=$(ssh_vps "nproc" 2>/dev/null || echo "0")

_contract_save "a1_vps_resources.json" "$(jq -nc \
  --arg mem "$VPS_MEM" --arg disk "$VPS_DISK" --arg cpu "$VPS_CPU" \
  '{memory_mb: ($mem | tonumber), disk_gb: ($disk | tonumber), cpu_count: ($cpu | tonumber)}')"

if [[ ${VPS_MEM:-0} -lt 1500 ]]; then
  log_fail "A1.3: Insufficient memory: ${VPS_MEM}MB (need ≥1500MB)"
  exit 1
fi
if [[ ${VPS_DISK:-0} -lt 5 ]]; then
  log_fail "A1.3: Insufficient disk: ${VPS_DISK}GB (need ≥5GB)"
  exit 1
fi
log_pass "A1.3: Resources OK — RAM: ${VPS_MEM}MB, Disk: ${VPS_DISK}GB, CPU: ${VPS_CPU}"

# ── Check if already deployed and healthy ────────────────────────────────────
if [[ "$FORCE_REDEPLOY" != "1" ]]; then
  HEALTH_RESULT=$(ssh_vps "$(declare -f _probe_bridge_health); _probe_bridge_health ${BRIDGE_PORT}" 2>/dev/null)
  if [[ $? -eq 0 && -n "$HEALTH_RESULT" ]]; then
    log_pass "A1: Bridge already healthy at port ${BRIDGE_PORT} (skip deploy). Set FORCE_REDEPLOY=1 to override."
    _contract_save "a1_deploy_status.json" "$(jq -nc '{
      phase: "A1", status: "already_healthy", force_redeploy: false
    }')"
    harness_summary
    exit 0
  fi
fi

# ── A1.4: Pull and inspect ArmorClaw image ──────────────────────────────────
log_info "A1.4: Pulling ${DEPLOY_IMAGE}..."
ssh_vps "docker pull ${DEPLOY_IMAGE}" 2>/dev/null || {
  log_fail "A1.4: Failed to pull ${DEPLOY_IMAGE}"
  exit 1
}
log_pass "A1.4: Image pulled"

log_info "A1.4: Inspecting image..."
IMAGE_INSPECT=$(ssh_vps "docker inspect ${DEPLOY_IMAGE} --format '{{.Config.ExposedPorts}} {{.Config.Entrypoint}} {{.Config.Cmd}}'" 2>/dev/null || echo "")
IMAGE_PORTS=$(ssh_vps "docker inspect ${DEPLOY_IMAGE} --format '{{range \$p, \$conf := .Config.ExposedPorts}}{{println \$p}}{{end}}'" 2>/dev/null || echo "")

_contract_save "a1_image_inspect.txt" "Image: ${DEPLOY_IMAGE}\nInspect: ${IMAGE_INSPECT}\nPorts: ${IMAGE_PORTS}"

# ── A1.5: Generate secrets ──────────────────────────────────────────────────
log_info "A1.5: Generating secrets..."
TURN_SECRET="${TURN_SECRET:-$(openssl rand -hex 16)}"
KEYSTORE_SECRET="${KEYSTORE_SECRET:-$(openssl rand -hex 16)}"

# ── A1.6: Determine API key ─────────────────────────────────────────────────
API_KEY_VAR="$(_get_api_key_var)" || {
  log_fail "A1.6: No AI API key found. Set OPENROUTER_API_KEY, OPEN_AI_KEY, or ZAI_API_KEY in .env"
  exit 1
}
API_KEY_VAL="$(_get_api_key)"
log_pass "A1.6: Using API key from ${API_KEY_VAR}"

# ── A1.7: Create docker-compose.yml ─────────────────────────────────────────
log_info "A1.7: Creating docker-compose.yml on VPS..."

# Determine topology from image inspection
if echo "$IMAGE_PORTS" | grep -q "6167"; then
  TOPOLOGY="single"
  log_info "A1.7: Single-image topology detected (Bridge+Matrix in one image)"
else
  TOPOLOGY="multi"
  log_info "A1.7: Multi-service topology (separate Bridge + Matrix containers)"
fi

ssh_vps "mkdir -p /opt/armorclaw"

if [[ "$TOPOLOGY" == "single" ]]; then
  ssh_vps "cat > /opt/armorclaw/docker-compose.plan-a.yml << 'COMPOSEEOF'
version: '3.8'
services:
  armorclaw:
    image: ${DEPLOY_IMAGE}
    container_name: armorclaw
    restart: unless-stopped
    ports:
      - \"${BRIDGE_PORT:-8080}:8080\"
      - \"${MATRIX_PORT:-6167}:6167\"
    environment:
      - ARMORCLAW_SERVER_MODE=native
      - ARMORCLAW_RPC_TRANSPORT=tcp
      - ARMORCLAW_LISTEN_ADDR=0.0.0.0:8080
      - ${API_KEY_VAR}=${API_KEY_VAL}
      - TURN_SECRET=${TURN_SECRET}
      - KEYSTORE_SECRET=${KEYSTORE_SECRET}
    volumes:
      - armorclaw-data:/etc/armorclaw
      - armorclaw-keystore:/var/lib/armorclaw
      - /var/run/docker.sock:/var/run/docker.sock
volumes:
  armorclaw-data:
  armorclaw-keystore:
COMPOSEEOF"
else
  ssh_vps "cat > /opt/armorclaw/docker-compose.plan-a.yml << 'COMPOSEEOF'
version: '3.8'
services:
  matrix:
    image: matrixconduit/matrix-conduit:latest
    container_name: armorclaw-matrix
    restart: unless-stopped
    ports:
      - \"${MATRIX_PORT:-6167}:6167\"
    environment:
      - CONDUIT_SERVER_NAME=${VPS_IP}
      - CONDUIT_DATABASE_BACKEND=rocksdb
      - CONDUIT_ALLOW_REGISTRATION=true
      - CONDUIT_REGISTRATION_TOKEN=planatest
    volumes:
      - conduit-data:/var/lib/matrix-conduit
  bridge:
    image: ${DEPLOY_IMAGE}
    container_name: armorclaw-bridge
    restart: unless-stopped
    depends_on:
      - matrix
    ports:
      - \"${BRIDGE_PORT:-8080}:8080\"
    environment:
      - ARMORCLAW_SERVER_MODE=native
      - ARMORCLAW_RPC_TRANSPORT=tcp
      - ARMORCLAW_LISTEN_ADDR=0.0.0.0:8080
      - ARMORCLAW_EXTERNAL_MATRIX=true
      - ARMORCLAW_MATRIX_HOMESERVER_URL=http://matrix:6167
      - ${API_KEY_VAR}=${API_KEY_VAL}
      - TURN_SECRET=${TURN_SECRET}
      - KEYSTORE_SECRET=${KEYSTORE_SECRET}
    volumes:
      - armorclaw-data:/etc/armorclaw
      - armorclaw-keystore:/var/lib/armorclaw
      - /var/run/docker.sock:/var/run/docker.sock
volumes:
  conduit-data:
  armorclaw-data:
  armorclaw-keystore:
COMPOSEEOF"
fi
log_pass "A1.7: Compose file created (${TOPOLOGY} topology)"

# ── A1.8: Start containers ──────────────────────────────────────────────────
log_info "A1.8: Starting containers..."

if [[ "$FORCE_REDEPLOY" == "1" ]]; then
  ssh_vps "cd /opt/armorclaw && docker compose -f docker-compose.plan-a.yml down" 2>/dev/null || true
fi

ssh_vps "cd /opt/armorclaw && docker compose -f docker-compose.plan-a.yml up -d" 2>/dev/null || {
  log_fail "A1.8: Failed to start containers"
  ssh_vps "cd /opt/armorclaw && docker compose -f docker-compose.plan-a.yml logs" 2>/dev/null | tail -20 > "${EVIDENCE_DIR}/a1_startup_failure.log"
  exit 1
}
log_pass "A1.8: Containers started"

# ── A1.9: Wait for Bridge /health (3 min) ───────────────────────────────────
log_info "A1.9: Waiting for Bridge /health (3 min timeout)..."
if ! _contract_wait_http "$BRIDGE_PORT" "/health" 180; then
  log_fail "A1.9: Bridge /health did not respond within 3 minutes"
  ssh_vps "docker logs armorclaw 2>&1 | tail -30" > "${EVIDENCE_DIR}/a1_bridge_logs.txt" 2>/dev/null || true
  ssh_vps "docker logs armorclaw-bridge 2>&1 | tail -30" > "${EVIDENCE_DIR}/a1_bridge_logs.txt" 2>/dev/null || true
  exit 1
fi

# ── A1.10: Wait for Bridge /api ─────────────────────────────────────────────
log_info "A1.10: Waiting for Bridge /api..."
if ! _contract_wait_http "$BRIDGE_PORT" "/api" 60; then
  log_info "A1.10: /api not responding (may be normal for some deployments)"
fi

# ── A1.11: Wait for Matrix homeserver ────────────────────────────────────────
log_info "A1.11: Waiting for Matrix homeserver on port ${MATRIX_PORT}..."
if ! _contract_wait_http "$MATRIX_PORT" "/_matrix/client/versions" 180; then
  log_fail "A1.11: Matrix homeserver did not respond within 3 minutes"
  exit 1
fi

# ── A1.12: Verify /.well-known/matrix/client ─────────────────────────────────
log_info "A1.12: Verifying /.well-known/matrix/client..."
WELL_KNOWN=$(ssh_vps "curl -sf -m 5 'http://localhost:${BRIDGE_PORT}/.well-known/matrix/client'" 2>/dev/null || echo "")
if [[ -n "$WELL_KNOWN" ]] && echo "$WELL_KNOWN" | jq -e '.["m.homeserver"].base_url' >/dev/null 2>&1; then
  log_pass "A1.12: /.well-known/matrix/client is valid"
else
  log_info "A1.12: /.well-known/matrix/client not on bridge port (may be on Matrix port or not configured)"
  WELL_KNOWN=$(ssh_vps "curl -sf -m 5 'http://localhost:${MATRIX_PORT}/.well-known/matrix/client'" 2>/dev/null || echo "")
  if [[ -n "$WELL_KNOWN" ]] && echo "$WELL_KNOWN" | jq -e '.["m.homeserver"].base_url' >/dev/null 2>&1; then
    log_pass "A1.12: /.well-known/matrix/client found on Matrix port"
  else
    log_info "A1.12: /.well-known not found (non-fatal for native mode)"
  fi
fi

# ── A1.12b: Generate TLS certs if sentinel mode ──────────────────────────────
log_info "A1.12b: Checking TLS cert requirements..."
CERTS_ON_VPS=$(ssh_vps "test -f /etc/armorclaw/certs/server.crt && echo EXISTS || echo MISSING" 2>/dev/null || echo "UNKNOWN")
if [[ "$CERTS_ON_VPS" == "MISSING" ]]; then
  log_info "A1.12b: Generating self-signed certs with public IP ${VPS_IP}..."
  if ssh_vps "bash deploy/scripts/generate-certs.sh --public-ip ${VPS_IP} --output /etc/armorclaw/certs" 2>/dev/null; then
    log_pass "A1.12b: Self-signed certs generated with public IP SAN"
  else
    log_info "A1.12b: Cert generation failed or script not available on VPS (non-fatal)"
  fi
else
  log_pass "A1.12b: Certs already exist on VPS"
fi

TLS_METADATA=$(ssh_vps "cat /etc/armorclaw/certs/server.crt 2>/dev/null" 2>/dev/null | \
  openssl x509 -noout -fingerprint -sha256 -enddate -subject -issuer 2>/dev/null || echo "")
_contract_save "a1_tls_metadata.json" "$(echo "$TLS_METADATA" | jq -R -s '{
  cert_details: .,
  public_ip: $IP,
  external_scheme: "https"
}' --arg IP "$VPS_IP")"

# ── A1.13: Collect deployment evidence ───────────────────────────────────────
log_info "A1.13: Collecting deployment evidence..."
CONTAINERS=$(ssh_vps "docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'" 2>/dev/null || echo "No containers found")
_contract_save "a1_containers.txt" "$CONTAINERS"

_contract_save "a1_deploy_status.json" "$(jq -nc \
  --arg topology "$TOPOLOGY" \
  --arg image "$DEPLOY_IMAGE" \
  --arg force "$FORCE_REDEPLOY" \
  --argjson mem "$VPS_MEM" --argjson disk "$VPS_DISK" --argjson cpu "$VPS_CPU" \
  '{
    phase: "A1",
    status: "deployed",
    topology: $topology,
    image: $image,
    force_redeploy: ($force == "1"),
    vps: {memory_mb: $mem, disk_gb: $disk, cpu_count: $cpu},
    timestamp: (now | todate)
  }')"

_contract_update_manifest '.runtime_flags.deployment_required' 'false'

log_info "========================================="
log_info " Phase A1: Deployment Complete"
log_info "========================================="
harness_summary
