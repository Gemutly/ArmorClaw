#!/usr/bin/env bash
# vps-lifecycle.sh — Top-level lifecycle orchestrator for ArmorClaw VPS
#
# Phases:
#   detect          — Detect VPS topology (read-only)
#   deploy          — Deploy/update ArmorClaw based on topology
#   admin-bootstrap — Bootstrap admin identity on Conduit
#   test-bootstrap  — Bootstrap test user + session + crypto verification
#   validate        — Run validation (smoke or full)
#   report          — Aggregate and emit evidence report
#   all             — Run all phases in sequence (default)
#
# Exit codes:
#   0 = pass
#   1 = fail
#   2 = partial (some phases passed, some failed)
#
# Usage:
#   bash scripts/vps-lifecycle.sh --vps-ip 1.2.3.4 --ssh-key ~/.ssh/id_ed25519 --phase all
#   bash scripts/vps-lifecycle.sh --vps-ip 1.2.3.4 --ssh-key ~/.ssh/id_ed25519 --phase validate --mode smoke
#   bash scripts/vps-lifecycle.sh --vps-ip 1.2.3.4 --ssh-key ~/.ssh/id_ed25519 --phase deploy --force --deploy-mode fresh-install

set -uo pipefail

# ── Paths ─────────────────────────────────────────────────────────────────────
_SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
_REPO_ROOT="$(cd "${_SCRIPT_DIR}/.." && pwd)"

# ── Source .env if present ────────────────────────────────────────────────────
set -a
source "${_REPO_ROOT}/.env" 2>/dev/null || true
set +a

# ── CLI defaults ──────────────────────────────────────────────────────────────
VPS_IP=""
SSH_KEY=""
SSH_USER="root"
PHASE="all"
MODE="smoke"
DEPLOY_MODE=""
FORCE=false
FEATURE_GROUPS=""
REPORT_FORMAT="json+text"
OUTPUT_DIR=""
SKIP_DEPLOY=false
SKIP_BOOTSTRAP=false

# ── Result accumulators ──────────────────────────────────────────────────────
_PHASE_PASS=0
_PHASE_FAIL=0
_PHASE_SKIP=0
_PHASE_RESULTS=()   # "phase_name:status" pairs
_START_TIME=$(date +%s)

# ── Usage ─────────────────────────────────────────────────────────────────────
usage() {
  cat <<'USAGE'
Usage: vps-lifecycle.sh [OPTIONS]

Required:
  --vps-ip IP          VPS IP address
  --ssh-key PATH       SSH key path

Options:
  --ssh-user USER      SSH user (default: root)
  --phase PHASE        detect | deploy | admin-bootstrap | test-bootstrap |
                       validate | report | all (default: all)
  --mode MODE          smoke | full (default: smoke)
  --deploy-mode MODE   replace-existing | reuse-existing-matrix |
                       side-by-side | fresh-install
  --force              Skip safety confirmations (REQUIRED for non-interactive)
  --feature-groups     Comma-separated groups (default: a-d for smoke, a-i for full)
  --report-format FMT  json | text | json+text (default: json+text)
  --output-dir DIR     Evidence output directory
  --skip-deploy        Skip deploy/update phase
  --skip-bootstrap     Skip bootstrap, use persisted state
USAGE
  exit 1
}

# ── Argument parsing ──────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --vps-ip)         VPS_IP="${2:?--vps-ip requires a value}"; shift 2 ;;
    --ssh-key)        SSH_KEY="${2:?--ssh-key requires a value}"; shift 2 ;;
    --ssh-user)       SSH_USER="${2:?--ssh-user requires a value}"; shift 2 ;;
    --phase)          PHASE="${2:?--phase requires a value}"; shift 2 ;;
    --mode)           MODE="${2:?--mode requires a value}"; shift 2 ;;
    --deploy-mode)    DEPLOY_MODE="${2:?--deploy-mode requires a value}"; shift 2 ;;
    --force)          FORCE=true; shift ;;
    --feature-groups) FEATURE_GROUPS="${2:?--feature-groups requires a value}"; shift 2 ;;
    --report-format)  REPORT_FORMAT="${2:?--report-format requires a value}"; shift 2 ;;
    --output-dir)     OUTPUT_DIR="${2:?--output-dir requires a value}"; shift 2 ;;
    --skip-deploy)    SKIP_DEPLOY=true; shift ;;
    --skip-bootstrap) SKIP_BOOTSTRAP=true; shift ;;
    -h|--help)        usage ;;
    *)                echo "ERROR: Unknown option: $1" >&2; usage ;;
  esac
done

# ── Validate required args ────────────────────────────────────────────────────
if [[ -z "$VPS_IP" ]]; then
  echo "ERROR: --vps-ip is required" >&2
  usage
fi
if [[ -z "$SSH_KEY" ]]; then
  echo "ERROR: --ssh-key is required" >&2
  usage
fi

# Expand ~ in SSH key path
SSH_KEY="${SSH_KEY/#\~/$HOME}"

if [[ ! -f "$SSH_KEY" ]]; then
  echo "ERROR: SSH key not found: $SSH_KEY" >&2
  exit 1
fi

# Validate phase
case "$PHASE" in
  detect|deploy|admin-bootstrap|test-bootstrap|validate|report|all) ;;
  *) echo "ERROR: Invalid phase: $PHASE" >&2; usage ;;
esac

# Validate mode
case "$MODE" in
  smoke|full) ;;
  *) echo "ERROR: Invalid mode: $MODE" >&2; usage ;;
esac

# Validate deploy-mode if provided
if [[ -n "$DEPLOY_MODE" ]]; then
  case "$DEPLOY_MODE" in
    replace-existing|reuse-existing-matrix|side-by-side|fresh-install) ;;
    *) echo "ERROR: Invalid deploy-mode: $DEPLOY_MODE" >&2; usage ;;
  esac
fi

# ── Evidence directory ────────────────────────────────────────────────────────
EVIDENCE_DIR="${OUTPUT_DIR:-${_REPO_ROOT}/.sisyphus/evidence/vps-lifecycle}"
mkdir -p "$EVIDENCE_DIR"

# ── Export env vars for lib modules ───────────────────────────────────────────
export VPS_IP
export VPS_USER="$SSH_USER"
export SSH_KEY_PATH="$SSH_KEY"
export BRIDGE_PORT="${BRIDGE_PORT:-8080}"
export MATRIX_PORT="${MATRIX_PORT:-6167}"

# ── Build SSH host string ────────────────────────────────────────────────────
_SSH_HOST="${SSH_USER}@${VPS_IP}"

# ── ssh_vps helper (mirrors contract.sh interface) ────────────────────────────
ssh_vps() {
  ssh -i "$SSH_KEY" -o StrictHostKeyChecking=no -o ConnectTimeout=10 \
    -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR \
    "$_SSH_HOST" "$@"
}

# ── Logging helpers ──────────────────────────────────────────────────────────
log_info()  { echo "[lifecycle] INFO: $*"; }
log_pass()  { echo "[lifecycle] PASS: $*"; }
log_fail()  { echo "[lifecycle] FAIL: $*"; }
log_warn()  { echo "[lifecycle] WARN: $*"; }

# ── Source lib modules ────────────────────────────────────────────────────────
source "${_SCRIPT_DIR}/lib/topology.sh"
source "${_SCRIPT_DIR}/lib/probe.sh"
source "${_SCRIPT_DIR}/lib/admin-bootstrap.sh"
source "${_SCRIPT_DIR}/lib/test-session-bootstrap.sh"
source "${_SCRIPT_DIR}/lib/matrix-state.sh"
source "${_SCRIPT_DIR}/lib/report.sh"
source "${_SCRIPT_DIR}/lib/rpc-probe.sh"

# ── Phase result tracking ────────────────────────────────────────────────────
record_phase() {
  local name="$1"
  local status="$2"  # pass, fail, skip
  _PHASE_RESULTS+=("${name}:${status}")
  case "$status" in
    pass) ((_PHASE_PASS++)) || true ;;
    fail) ((_PHASE_FAIL++)) || true ;;
    skip) ((_PHASE_SKIP++)) || true ;;
  esac
}

# ── Phase: detect ─────────────────────────────────────────────────────────────
# Detect VPS topology via _topology_detect() from topology.sh.
# Stores topology JSON to evidence directory.
phase_detect() {
  log_info "=== Phase: detect ==="
  log_info "Detecting VPS topology at ${VPS_IP}..."

  # Verify SSH connectivity
  if ! ssh_vps "true" 2>/dev/null; then
    log_fail "SSH connectivity to ${_SSH_HOST} failed"
    record_phase "detect" "fail"
    return 1
  fi
  log_pass "SSH connectivity verified"

  # Run topology detection
  if ! _topology_detect; then
    log_fail "Topology detection failed"
    record_phase "detect" "fail"
    return 1
  fi

  # Classify topology
  local classification
  classification=$(_topology_classify)
  log_info "Topology classified: ${classification}"

  # Get recommendation
  local recommendation
  recommendation=$(_topology_recommend_mode)
  log_info "Recommended deploy mode: ${recommendation}"

  # Save topology JSON to evidence
  local topo_json
  topo_json=$(_topology_to_json)
  echo "$topo_json" > "${EVIDENCE_DIR}/topology.json"
  log_pass "Topology evidence saved to ${EVIDENCE_DIR}/topology.json"

  # Check for ambiguous topology
  if [[ "$classification" == "mixed" && "$FORCE" != "true" ]]; then
    log_fail "Mixed topology detected. Use --force with explicit --deploy-mode to proceed."
    log_fail "Evidence: ${EVIDENCE_DIR}/topology.json"
    record_phase "detect" "fail"
    return 1
  fi

  # If no deploy-mode specified, use recommendation
  if [[ -z "$DEPLOY_MODE" ]]; then
    DEPLOY_MODE="$recommendation"
    log_info "No --deploy-mode specified, using recommendation: ${DEPLOY_MODE}"
  fi

  record_phase "detect" "pass"
  return 0
}

# ── Phase: deploy ─────────────────────────────────────────────────────────────
# Deploy/update ArmorClaw based on topology.
# Requires --force + explicit --deploy-mode for safety.
phase_deploy() {
  log_info "=== Phase: deploy ==="

  if [[ "$SKIP_DEPLOY" == "true" ]]; then
    log_info "Skipping deploy (--skip-deploy)"
    record_phase "deploy" "skip"
    return 0
  fi

  # Safety: require --force for destructive deploy
  if [[ "$FORCE" != "true" ]]; then
    log_fail "Deploy phase requires --force to proceed"
    log_fail "Use: --force --deploy-mode <mode>"
    record_phase "deploy" "fail"
    return 1
  fi

  # Safety: require explicit --deploy-mode
  if [[ -z "$DEPLOY_MODE" ]]; then
    log_fail "Deploy phase requires --deploy-mode"
    log_fail "Options: replace-existing | reuse-existing-matrix | side-by-side | fresh-install"
    record_phase "deploy" "fail"
    return 1
  fi

  log_info "Deploy mode: ${DEPLOY_MODE}"

  # Load topology if not already detected
  if [[ ! -f "${EVIDENCE_DIR}/topology.json" ]]; then
    log_warn "No topology evidence found — running detect first"
    if ! phase_detect; then
      record_phase "deploy" "fail"
      return 1
    fi
  fi

  # Deploy based on mode
  case "$DEPLOY_MODE" in
    fresh-install)
      log_info "Running fresh install on VPS..."
      if ssh_vps "command -v docker >/dev/null 2>&1 && docker ps >/dev/null 2>&1" 2>/dev/null; then
        log_pass "Docker available on VPS"
        # Invoke installer via SSH — curl the install script
        log_info "Triggering ArmorClaw install.sh on VPS..."
        local install_exit=0
        ssh_vps "curl -fsSL https://raw.githubusercontent.com/Gemutly/ArmorClaw/main/deploy/install.sh | bash" 2>&1 || install_exit=$?
        if [[ $install_exit -ne 0 ]]; then
          log_fail "Fresh install failed (exit: ${install_exit})"
          record_phase "deploy" "fail"
          return 1
        fi
        log_pass "Fresh install completed"
      else
        log_fail "Docker not available on VPS — cannot deploy"
        record_phase "deploy" "fail"
        return 1
      fi
      ;;
    replace-existing)
      log_info "Replacing existing installation..."

      # ── Preserve ARMORCLAW_KEYSTORE_SECRET before stopping containers ───────
      # The hardware-bound keystore requires this secret. Without it, existing
      # encrypted data becomes inaccessible after the update.
      local _keystore_secret=""
      # Try running container env first (most reliable for live deployments)
      local _container_id
      _container_id=$(ssh_vps "docker ps -q --filter name=armorclaw 2>/dev/null | head -1" 2>/dev/null || true)
      if [[ -n "$_container_id" ]]; then
        _keystore_secret=$(ssh_vps "docker inspect --format='{{range .Config.Env}}{{println .}}{{end}}' '${_container_id}' 2>/dev/null | grep '^ARMORCLAW_KEYSTORE_SECRET=' | head -1 | cut -d= -f2-" 2>/dev/null || true)
      fi
      # Fallback: check /etc/armorclaw/.env on disk
      if [[ -z "$_keystore_secret" ]]; then
        _keystore_secret=$(ssh_vps "grep '^ARMORCLAW_KEYSTORE_SECRET=' /etc/armorclaw/.env 2>/dev/null | head -1 | cut -d= -f2-" 2>/dev/null || true)
      fi
      # Fallback: check docker-compose .env in /opt/armorclaw
      if [[ -z "$_keystore_secret" ]]; then
        _keystore_secret=$(ssh_vps "grep '^ARMORCLAW_KEYSTORE_SECRET=' /opt/armorclaw/.env 2>/dev/null | head -1 | cut -d= -f2-" 2>/dev/null || true)
      fi

      if [[ -n "$_keystore_secret" ]]; then
        log_pass "ARMORCLAW_KEYSTORE_SECRET preserved from existing installation"
      else
        log_warn "ARMORCLAW_KEYSTORE_SECRET not found on existing installation — hardware-bound keystore may fail after update"
      fi

      # Stop existing containers and reinstall
      ssh_vps "docker stop \$(docker ps -q --filter name=conduit) 2>/dev/null; docker stop \$(docker ps -q --filter name=armorclaw) 2>/dev/null; docker stop \$(docker ps -q --filter name=quickstart) 2>/dev/null" 2>/dev/null || true
      log_info "Existing containers stopped — running fresh install..."

      # Deploy new installation, passing preserved keystore secret if found
      local install_exit=0
      if [[ -n "$_keystore_secret" ]]; then
        ssh_vps "ARMORCLAW_KEYSTORE_SECRET='${_keystore_secret}' curl -fsSL https://raw.githubusercontent.com/Gemutly/ArmorClaw/main/deploy/install.sh | bash" 2>&1 || install_exit=$?
      else
        ssh_vps "curl -fsSL https://raw.githubusercontent.com/Gemutly/ArmorClaw/main/deploy/install.sh | bash" 2>&1 || install_exit=$?
      fi
      if [[ $install_exit -ne 0 ]]; then
        log_fail "Replace deploy failed (exit: ${install_exit})"
        record_phase "deploy" "fail"
        return 1
      fi
      log_pass "Replace deploy completed"
      ;;
    reuse-existing-matrix)
      log_info "Reusing existing Matrix (Conduit) — deploying Bridge only"
      # Only deploy/update the Bridge, keep Conduit running
      log_warn "Reuse-existing-matrix: Bridge-only deploy not yet automated"
      log_info "Verifying existing Conduit is healthy..."
      local conduit_status
      conduit_status=$(ssh_vps "curl -sf -o /dev/null -w '%{http_code}' -m 5 http://localhost:6167/_matrix/client/versions" 2>/dev/null || echo "000")
      if [[ "$conduit_status" != "200" ]]; then
        log_fail "Existing Conduit not healthy (status: ${conduit_status})"
        record_phase "deploy" "fail"
        return 1
      fi
      log_pass "Existing Conduit verified healthy"
      ;;
    side-by-side)
      log_info "Side-by-side deployment — not recommended, may cause port conflicts"
      log_warn "Side-by-side mode: deploying with alternate ports"
      # For now, just verify the existing installation is healthy
      log_warn "Side-by-side automated deploy not yet implemented — skipping"
      record_phase "deploy" "skip"
      return 0
      ;;
    *)
      log_fail "Unknown deploy mode: $DEPLOY_MODE"
      record_phase "deploy" "fail"
      return 1
      ;;
  esac

  # Wait for services to be ready
  log_info "Waiting for Bridge to become ready..."
  local elapsed=0
  local bridge_ready=false
  while (( elapsed < 120 )); do
    if ssh_vps "curl -sf -o /dev/null -m 5 http://localhost:${BRIDGE_PORT}/health" 2>/dev/null; then
      bridge_ready=true
      break
    fi
    if ssh_vps "curl -sf -k -o /dev/null -m 5 https://localhost:${BRIDGE_PORT}/health" 2>/dev/null; then
      bridge_ready=true
      break
    fi
    sleep 5
    (( elapsed += 5 )) || true
    log_info "Waiting for Bridge... (${elapsed}s/120s)"
  done

  if [[ "$bridge_ready" != "true" ]]; then
    log_fail "Bridge not ready after 120s"
    record_phase "deploy" "fail"
    return 1
  fi
  log_pass "Bridge is ready"

  # Wait for Conduit
  log_info "Waiting for Matrix Conduit to become ready..."
  elapsed=0
  local conduit_ready=false
  while (( elapsed < 120 )); do
    local status
    status=$(ssh_vps "curl -sf -o /dev/null -w '%{http_code}' -m 5 http://localhost:${MATRIX_PORT}/_matrix/client/versions" 2>/dev/null || echo "000")
    if [[ "$status" == "200" ]]; then
      conduit_ready=true
      break
    fi
    sleep 5
    (( elapsed += 5 )) || true
    log_info "Waiting for Conduit... (${elapsed}s/120s)"
  done

  if [[ "$conduit_ready" != "true" ]]; then
    log_fail "Conduit not ready after 120s"
    record_phase "deploy" "fail"
    return 1
  fi
  log_pass "Conduit is ready"

  record_phase "deploy" "pass"
  return 0
}

# ── Phase: admin-bootstrap ───────────────────────────────────────────────────
# Bootstrap admin identity via _admin_bootstrap() from admin-bootstrap.sh.
phase_admin_bootstrap() {
  log_info "=== Phase: admin-bootstrap ==="

  if [[ "$SKIP_BOOTSTRAP" == "true" ]]; then
    log_info "Skipping admin bootstrap (--skip-bootstrap)"
    record_phase "admin-bootstrap" "skip"
    return 0
  fi

  log_info "Bootstrapping admin identity on Conduit..."

  if ! _admin_bootstrap "$_SSH_HOST"; then
    log_fail "Admin bootstrap failed"
    record_phase "admin-bootstrap" "fail"
    return 1
  fi

  # Save admin credentials to evidence (chmod 600 for security)
  local admin_state
  admin_state=$(jq -n \
    --arg user_id "${_ADMIN_USER_ID}" \
    --arg access_token "${_ADMIN_ACCESS_TOKEN}" \
    '{
      admin_user_id: $user_id,
      admin_access_token: $access_token,
      timestamp: (now | todate)
    }')
  echo "$admin_state" > "${EVIDENCE_DIR}/admin-state.json"
  chmod 600 "${EVIDENCE_DIR}/admin-state.json"

  log_pass "Admin bootstrap complete: ${_ADMIN_USER_ID}"
  record_phase "admin-bootstrap" "pass"
  return 0
}

# ── Phase: test-bootstrap ────────────────────────────────────────────────────
# Bootstrap test user + session via _test_session_bootstrap() + save state via
# _matrix_save_state().
phase_test_bootstrap() {
  log_info "=== Phase: test-bootstrap ==="

  if [[ "$SKIP_BOOTSTRAP" == "true" ]]; then
    log_info "Skipping test bootstrap (--skip-bootstrap) — loading persisted state"

    if _matrix_load_state; then
      if _matrix_state_is_valid "$_SSH_HOST"; then
        log_pass "Persisted state valid: ${_TEST_USER_ID}"
        record_phase "test-bootstrap" "pass"
        return 0
      else
        log_warn "Persisted state invalid — attempting token refresh"
        if _matrix_refresh_token "$_SSH_HOST"; then
          log_pass "Token refreshed successfully"
          record_phase "test-bootstrap" "pass"
          return 0
        fi
        log_fail "Persisted state unusable and refresh failed — re-bootstrap required"
        record_phase "test-bootstrap" "fail"
        return 1
      fi
    else
      log_fail "No persisted state found and --skip-bootstrap specified"
      record_phase "test-bootstrap" "fail"
      return 1
    fi
  fi

  # Require admin access token from previous phase
  if [[ -z "${_ADMIN_ACCESS_TOKEN:-}" ]]; then
    # Try loading from evidence
    if [[ -f "${EVIDENCE_DIR}/admin-state.json" ]]; then
      _ADMIN_ACCESS_TOKEN=$(jq -r '.admin_access_token // empty' "${EVIDENCE_DIR}/admin-state.json" 2>/dev/null)
    fi
    if [[ -z "${_ADMIN_ACCESS_TOKEN:-}" ]]; then
      log_fail "No admin access token — run admin-bootstrap phase first"
      record_phase "test-bootstrap" "fail"
      return 1
    fi
  fi

  log_info "Bootstrapping test session..."

  if ! _test_session_bootstrap "$_SSH_HOST" --bootstrap-admin-token "$_ADMIN_ACCESS_TOKEN"; then
    log_fail "Test session bootstrap failed"
    record_phase "test-bootstrap" "fail"
    return 1
  fi

  # Persist state for future runs
  _matrix_save_state

  log_pass "Test bootstrap complete: ${_TEST_USER_ID} room=${_TEST_ROOM_ID:-?} crypto=${_TEST_CRYPTO_VERIFIED}"
  record_phase "test-bootstrap" "pass"
  return 0
}

# ── Phase: validate ──────────────────────────────────────────────────────────
# Run validation based on mode:
#   smoke: topology checks + Bridge health + Matrix smoke + A0 sanity
#   full:  feature groups A-I (stubbed for now — T14-T20 will create them)
phase_validate() {
  log_info "=== Phase: validate ==="
  log_info "Mode: ${MODE}"

  local validate_pass=0
  local validate_fail=0
  local validate_skip=0
  local validate_results=()

  # ── Check 1: Topology verification ──────────────────────────────────────────
  log_info "[validate] Running topology verification..."
  if [[ -f "${EVIDENCE_DIR}/topology.json" ]]; then
    local classification
    classification=$(jq -r '.classification // "unknown"' "${EVIDENCE_DIR}/topology.json" 2>/dev/null)
    log_pass "[validate] Topology: ${classification}"
    validate_results+=("topology:pass")
    ((validate_pass++)) || true
  else
    # Run a quick detect if topology evidence not available
    if _topology_detect 2>/dev/null; then
      local classification
      classification=$(_topology_classify)
      log_pass "[validate] Topology (fresh detect): ${classification}"
      _topology_to_json > "${EVIDENCE_DIR}/topology.json"
      validate_results+=("topology:pass")
      ((validate_pass++)) || true
    else
      log_fail "[validate] Topology detection failed"
      validate_results+=("topology:fail")
      ((validate_fail++)) || true
    fi
  fi

  # ── Check 2: Bridge health (via probe.sh) ──────────────────────────────────
  log_info "[validate] Probing Bridge health on port ${BRIDGE_PORT}..."
  local bridge_health
  bridge_health=$(ssh_vps "curl -sf -k -m 5 'https://localhost:${BRIDGE_PORT}/health' 2>/dev/null || curl -sf -m 5 'http://localhost:${BRIDGE_PORT}/health' 2>/dev/null")

  if [[ $? -eq 0 && -n "$bridge_health" ]]; then
    log_pass "[validate] Bridge health: ${bridge_health}"
    validate_results+=("bridge-health:pass")
    ((validate_pass++)) || true
  else
    log_fail "[validate] Bridge health check failed (port ${BRIDGE_PORT})"
    validate_results+=("bridge-health:fail")
    ((validate_fail++)) || true
  fi

  # ── Check 3: Matrix smoke test ─────────────────────────────────────────────
  # Login → /status → /help → send/receive
  log_info "[validate] Running Matrix smoke test..."

  local matrix_url="http://localhost:${MATRIX_PORT}"

  # Use test user if available, otherwise try admin
  local test_token="${_TEST_ACCESS_TOKEN:-}"
  local test_room="${_TEST_ROOM_ID:-}"

  if [[ -z "$test_token" ]]; then
    # Try loading from persisted state
    if [[ -f "${EVIDENCE_DIR}/admin-state.json" ]]; then
      test_token=$(jq -r '.admin_access_token // empty' "${EVIDENCE_DIR}/admin-state.json" 2>/dev/null)
    fi
  fi

  if [[ -n "$test_token" ]]; then
    # Matrix login verification
    local whoami_resp
    whoami_resp=$(ssh_vps "curl -s '${matrix_url}/_matrix/client/r0/account/whoami?access_token=${test_token}'" 2>/dev/null)
    local whoami_user
    whoami_user=$(echo "$whoami_resp" | jq -r '.user_id // empty' 2>/dev/null)

    if [[ -n "$whoami_user" && "$whoami_user" != "null" ]]; then
      log_pass "[validate] Matrix whoami: ${whoami_user}"
      validate_results+=("matrix-login:pass")
      ((validate_pass++)) || true
    else
      log_fail "[validate] Matrix whoami failed"
      validate_results+=("matrix-login:fail")
      ((validate_fail++)) || true
    fi

    # Matrix sync check
    local sync_resp
    sync_resp=$(ssh_vps "curl -s '${matrix_url}/_matrix/client/v3/sync?access_token=${test_token}&timeout=0'" 2>/dev/null)
    local next_batch
    next_batch=$(echo "$sync_resp" | jq -r '.next_batch // empty' 2>/dev/null)

    if [[ -n "$next_batch" ]]; then
      log_pass "[validate] Matrix sync: next_batch=${next_batch}"
      validate_results+=("matrix-sync:pass")
      ((validate_pass++)) || true
    else
      log_fail "[validate] Matrix sync failed"
      validate_results+=("matrix-sync:fail")
      ((validate_fail++)) || true
    fi

    # Send/receive message test (if we have a room)
    if [[ -n "$test_room" && "$test_room" != "null" ]]; then
      local txn_id="txn-validate-$$-$(date +%s%N)"
      local test_body="validate-smoke-$(date +%s)"

      local send_resp
      send_resp=$(ssh_vps "curl -s -X PUT '${matrix_url}/_matrix/client/v3/rooms/${test_room}/send/m.room.message/${txn_id}?access_token=${test_token}' -H 'Content-Type: application/json' -d '{\"msgtype\":\"m.text\",\"body\":\"${test_body}\"}'" 2>/dev/null)

      local sent_event_id
      sent_event_id=$(echo "$send_resp" | jq -r '.event_id // empty' 2>/dev/null)

      if [[ -n "$sent_event_id" ]]; then
        # Poll for message
        local found_msg=false
        local poll_elapsed=0
        while (( poll_elapsed < 10 )); do
          local poll_resp
          poll_resp=$(ssh_vps "curl -s '${matrix_url}/_matrix/client/v3/sync?access_token=${test_token}&timeout=1000&since=${next_batch}'" 2>/dev/null)
          local msg_body
          msg_body=$(echo "$poll_resp" | jq -r --arg room "$test_room" --arg body "$test_body" \
            '.rooms.join[$room].timeline.events // [] | .[] | select(.type == "m.room.message") | .content.body // empty' 2>/dev/null | head -1)
          if [[ "$msg_body" == "$test_body" ]]; then
            found_msg=true
            break
          fi
          next_batch=$(echo "$poll_resp" | jq -r '.next_batch // empty' 2>/dev/null)
          sleep 2
          (( poll_elapsed += 2 )) || true
        done

        if [[ "$found_msg" == "true" ]]; then
          log_pass "[validate] Matrix send/receive: message round-trip confirmed"
          validate_results+=("matrix-send-recv:pass")
          ((validate_pass++)) || true
        else
          log_fail "[validate] Matrix send/receive: sent but not received"
          validate_results+=("matrix-send-recv:fail")
          ((validate_fail++)) || true
        fi
      else
        log_fail "[validate] Matrix send failed"
        validate_results+=("matrix-send-recv:fail")
        ((validate_fail++)) || true
      fi
    else
      log_warn "[validate] No test room available — skipping send/receive test"
      validate_results+=("matrix-send-recv:skip")
      ((validate_skip++)) || true
    fi

    # Matrix /help command (status command via Matrix)
    # This checks if Bridge responds to Matrix commands
    log_info "[validate] Checking Bridge /status command..."
    local status_resp
    status_resp=$(ssh_vps "curl -s -X PUT '${matrix_url}/_matrix/client/v3/rooms/${test_room:-!noop}/send/m.room.message/txn-status-$$?access_token=${test_token}' -H 'Content-Type: application/json' -d '{\"msgtype\":\"m.text\",\"body\":\"/status\"}'" 2>/dev/null)
    # We don't validate the response here — just that the send succeeded
    # Bridge command response comes async via Matrix
    log_info "[validate] /status command sent (async response expected)"
  else
    log_fail "[validate] No access token available for Matrix smoke test"
    validate_results+=("matrix-login:fail")
    validate_results+=("matrix-sync:skip")
    validate_results+=("matrix-send-recv:skip")
    ((validate_fail++)) || true
    ((validate_skip++)) || true
    ((validate_skip++)) || true
  fi

  # ── Check 4: RPC endpoint probe ──────────────────────────────────────────────
  log_info "[validate] Probing bridge RPC endpoints..."
  local rpc_probe_result
  rpc_probe_result=$(_rpc_probe_bridge "${SSH_USER}@${VPS_IP}" "${BRIDGE_PORT}" 2>/dev/null) || true

  if [[ "$rpc_probe_result" == "compatible" ]]; then
    log_pass "[validate] RPC probe: ${rpc_probe_result}"
    validate_results+=("rpc-probe:pass")
    ((validate_pass++)) || true
  else
    log_warn "[validate] RPC probe: ${rpc_probe_result} (see evidence for details)"
    validate_results+=("rpc-probe:${rpc_probe_result}")
    ((validate_fail++)) || true
  fi

  # ── Check 5: A0 sanity — verify >0 responding RPC methods ──────────────────
  log_info "[validate] Running A0 sanity (RPC method discovery)..."
  local discover_resp
  discover_resp=$(ssh_vps "curl -sf -k -m 10 'https://localhost:${BRIDGE_PORT}/api' -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"rpc.discover\",\"params\":{}}' 2>/dev/null || curl -sf -m 10 'http://localhost:${BRIDGE_PORT}/api' -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"rpc.discover\",\"params\":{}}' 2>/dev/null" 2>/dev/null)

  local method_count=0
  if [[ -n "$discover_resp" ]]; then
    method_count=$(echo "$discover_resp" | jq -r '.result.methods // [] | length' 2>/dev/null || echo "0")
  fi

  if [[ "$method_count" -gt 0 ]]; then
    log_pass "[validate] A0 sanity: ${method_count} RPC methods discovered"
    validate_results+=("a0-sanity:pass")
    ((validate_pass++)) || true
  else
    log_fail "[validate] A0 sanity: no RPC methods discovered (Bridge may not be running)"
    validate_results+=("a0-sanity:fail")
    ((validate_fail++)) || true
  fi

  # ── Mode: full — feature groups ────────────────────────────────────────────
  if [[ "$MODE" == "full" ]]; then
    log_info "[validate] Full mode — running feature groups..."

    # Default feature groups for full mode
    local groups="${FEATURE_GROUPS:-a,b,c,d,e,f,g,h,i}"
    local IFS=','
    local group_array=($groups)
    unset IFS

    local _bcd_blocker_added=false
    for group in "${group_array[@]}"; do
      # ── Gate B/C/D on RPC compatibility probe ──────────────────────────────
      if [[ "$group" == @(b|c|d) && "$rpc_probe_result" != "compatible" ]]; then
        log_warn "[validate] Feature group ${group}: SKIP-DISABLED (RPC probe: ${rpc_probe_result})"
        validate_results+=("group-${group}:skip-disabled")
        ((validate_skip++)) || true
        if [[ "$_bcd_blocker_added" != "true" ]]; then
          _report_add_blocker "validate" "Groups B/C/D skipped: RPC probe classified as '${rpc_probe_result}' — bridge API incompatible or unreachable" "high"
          _bcd_blocker_added=true
        fi
        # Write skip-disabled evidence file so report phase picks up correct status
        local _gated_name
        case "$group" in
          b) _gated_name="B-Studio" ;;
          c) _gated_name="C-Secretary" ;;
          d) _gated_name="D-Trust" ;;
        esac
        jq -n --arg group "$_gated_name" --arg classification "$rpc_probe_result" \
          '{group: $group, overall: "skip-disabled", results: [], details: ("Skipped: RPC probe classified as " + $classification), duration_ms: 0}' \
          > "${EVIDENCE_DIR}/group-${group}-summary.json"
        continue
      fi

      local group_script
      group_script=$(compgen -G "${_SCRIPT_DIR}/feature-groups/group-${group}-*.sh" 2>/dev/null | head -1)
      group_script="${group_script:-${_SCRIPT_DIR}/feature-groups/group-${group}.sh}"
      if [[ -f "$group_script" ]]; then
        log_info "[validate] Running feature group ${group}..."
        # Source the library and call its entry function directly
        # Group scripts are sourced libraries (no shebang, no main block)
        source "$group_script"
        local group_result
        group_result=$(_group_${group}_run \
          --vps-ip "${VPS_IP}" \
          --ssh-key "${SSH_KEY_PATH}" \
          --ssh-user "${SSH_USER:-root}" \
          --output-dir "${EVIDENCE_DIR}" 2>&1) || true
        # Parse the result JSON — each group outputs a JSON array of test results
        local group_status
        group_status=$(echo "$group_result" | tail -1 | jq -r 'if type == "object" then .overall // "fail" elif type == "array" then (.[0].status // "fail") else "fail" end' 2>/dev/null || echo "fail")
        if [[ "$group_status" == "pass" ]]; then
          log_pass "[validate] Feature group ${group}: PASS"
          validate_results+=("group-${group}:pass")
          ((validate_pass++)) || true
        elif [[ "$group_status" == "not-run" ]]; then
          log_warn "[validate] Feature group ${group}: NOT-RUN (service unavailable)"
          validate_results+=("group-${group}:not-run")
          ((validate_skip++)) || true
        else
          log_fail "[validate] Feature group ${group}: ${group_status^^}"
          validate_results+=("group-${group}:${group_status}")
          ((validate_fail++)) || true
        fi
      else
        log_warn "[validate] Feature group ${group} script not found — skipping (T14-T20)"
        validate_results+=("group-${group}:skip")
        ((validate_skip++)) || true
      fi
    done
  fi

  # Save validate evidence
  local validate_evidence
  validate_evidence=$(jq -nc \
    --arg mode "$MODE" \
    --argjson pass "$validate_pass" \
    --argjson fail "$validate_fail" \
    --argjson skip "$validate_skip" \
    --arg results "$(printf '%s\n' "${validate_results[@]}" | jq -R . | jq -s .)" \
    '{
      mode: $mode,
      pass: $pass,
      fail: $fail,
      skip: $skip,
      results: ($results | map(split(":") | {(.[0]): .[1]}) | add // {}),
      timestamp: (now | todate)
    }')
  echo "$validate_evidence" > "${EVIDENCE_DIR}/validate-results.json"

  log_info "[validate] Results: ${validate_pass} PASS | ${validate_fail} FAIL | ${validate_skip} SKIP"

  if [[ $validate_fail -gt 0 ]]; then
    record_phase "validate" "fail"
    return 1
  fi

  record_phase "validate" "pass"
  return 0
}

# ── Phase: report ─────────────────────────────────────────────────────────────
# Aggregate evidence from all phases and feature groups, compute verdict,
# emit JSON + text reports via scripts/lib/report.sh.
#
# Exit codes from this phase:
#   0 = pass
#   1 = fail or blocked
#   2 = partial
phase_report() {
  log_info "=== Phase: report ==="

  # ── 1. Initialize report module ──────────────────────────────────────────────
  _report_init --output-dir "$EVIDENCE_DIR"

  # ── 2. Set topology/deploy context ──────────────────────────────────────────
  local topo_classification="unknown"
  local topo_deploy_mode="${DEPLOY_MODE:-unknown}"
  if [[ -f "${EVIDENCE_DIR}/topology.json" ]]; then
    topo_classification=$(jq -r '.classification // "unknown"' "${EVIDENCE_DIR}/topology.json" 2>/dev/null)
    topo_deploy_mode=$(jq -r '.recommendation // "unknown"' "${EVIDENCE_DIR}/topology.json" 2>/dev/null)
  fi
  _report_set_topology "$topo_classification" "$topo_deploy_mode"

  # ── 3. Add lifecycle phase results ──────────────────────────────────────────
  local _pr_name _pr_status
  for entry in "${_PHASE_RESULTS[@]+"${_PHASE_RESULTS[@]}"}"; do
    _pr_name="${entry%%:*}"
    _pr_status="${entry##*:}"
    _report_add_phase "$_pr_name" "$_pr_status" ""
  done

  # ── 4. Set deploy results from phase tracking ───────────────────────────────
  local _dr_fresh="not-run"
  local _dr_existing="not-run"
  local _dr_matrix="not-run"
  for entry in "${_PHASE_RESULTS[@]+"${_PHASE_RESULTS[@]}"}"; do
    _pr_name="${entry%%:*}"
    _pr_status="${entry##*:}"
    case "$_pr_name" in
      deploy)
        if [[ "$topo_deploy_mode" == "fresh-install" ]]; then
          _dr_fresh="$_pr_status"
        else
          _dr_existing="$_pr_status"
        fi
        ;;
      admin-bootstrap) _dr_matrix="$_pr_status" ;;
      test-bootstrap)
        if [[ "$_dr_matrix" == "not-run" ]]; then
          _dr_matrix="$_pr_status"
        fi
        ;;
    esac
  done
  _report_set_deploy_results "$_dr_fresh" "$_dr_existing" "$_dr_matrix"

  if [[ "$_dr_fresh" == "fail" || "$_dr_existing" == "fail" ]]; then
    _report_add_blocker "deploy" "Deploy phase failed — cannot proceed with validation" "high"
  fi

  # ── 4b. Blocker: deploy not-run ────────────────────────────────────────────
  if [[ "$_dr_fresh" == "not-run" && "$_dr_existing" == "not-run" ]]; then
    _report_add_blocker "deploy" "Deploy was not executed — no deployment attempted" "high"
  fi

  # ── 4c. Blocker: RPC compatibility mismatch ───────────────────────────────
  if [[ -f "${EVIDENCE_DIR}/validate-results.json" ]]; then
    local _a0_sanity
    _a0_sanity=$(jq -r '.results["a0-sanity"] // "unknown"' "${EVIDENCE_DIR}/validate-results.json" 2>/dev/null)
    if [[ "$_a0_sanity" == "fail" ]]; then
      _report_add_blocker "validate" "RPC compatibility mismatch — no RPC methods discovered (Bridge may not expose expected API)" "high"
    fi
  fi

  # ── 4d. Blocker: Matrix token invalidation ────────────────────────────────
  if [[ -f "${EVIDENCE_DIR}/validate-results.json" ]]; then
    local _matrix_login
    _matrix_login=$(jq -r '.results["matrix-login"] // "unknown"' "${EVIDENCE_DIR}/validate-results.json" 2>/dev/null)
    if [[ "$_matrix_login" == "fail" ]]; then
      _report_add_blocker "validate" "Matrix token invalidation — login failed, Bridge cannot authenticate with homeserver" "high"
    fi
  fi

  # ── 5. Feature group definitions (A-I) ──────────────────────────────────────
  #   group_letter: display name
  local -A _FG_NAMES
  _FG_NAMES[a]="A-Matrix"
  _FG_NAMES[b]="B-Studio"
  _FG_NAMES[c]="C-Secretary"
  _FG_NAMES[d]="D-Trust"
  _FG_NAMES[e]="E-Email"
  _FG_NAMES[f]="F-Sidecar"
  _FG_NAMES[g]="G-Browser"
  _FG_NAMES[h]="H-Events"
  _FG_NAMES[i]="I-Flags"

  local -a _SMOKE_GROUPS=(a b c d)
  local -a _FULL_GROUPS=(a b c d e f g h i)

  local -a _scope_groups
  if [[ "$MODE" == "smoke" ]]; then
    _scope_groups=("${_SMOKE_GROUPS[@]}")
  else
    _scope_groups=("${_FULL_GROUPS[@]}")
  fi

  # ── 6. Collect feature group results from evidence ──────────────────────────
  local _fg_letter _fg_name _fg_status _fg_details _fg_evidence
  for _fg_letter in "${_FULL_GROUPS[@]}"; do
    _fg_name="${_FG_NAMES[${_fg_letter}]}"

    # Check if this group is in scope for the current mode
    local _in_scope=false
    local _sg
    for _sg in "${_scope_groups[@]}"; do
      if [[ "$_sg" == "$_fg_letter" ]]; then
        _in_scope=true
        break
      fi
    done

    local _fg_pattern="${EVIDENCE_DIR}/group-${_fg_letter}-*.json"
    local _fg_files=()
    local _gf
    for _gf in $_fg_pattern; do
      if [[ -f "$_gf" ]]; then
        _fg_files+=("$_gf")
      fi
    done

    if [[ ${#_fg_files[@]} -eq 0 ]]; then
      if [[ "$_in_scope" == "true" ]]; then
        _fg_status="not-run"
        _fg_details="No evidence found for ${_fg_name}"
      else
        _fg_status="not-run"
        _fg_details="Not in scope for ${MODE} mode"
      fi
      _fg_evidence=""
    else
      local _fg_pass=0 _fg_fail=0 _fg_skip=0 _fg_total=${#_fg_files[@]}
      local _ev_file _ev_overall
      for _ev_file in "${_fg_files[@]}"; do
        _ev_overall=$(jq -r '.overall // "unknown"' "$_ev_file" 2>/dev/null)
        case "$_ev_overall" in
          pass)         ((_fg_pass++)) || true ;;
          fail)         ((_fg_fail++)) || true ;;
          skip|skip-disabled) ((_fg_skip++)) || true ;;
          *)            ((_fg_fail++)) || true ;;
        esac
        _report_add_evidence "$_ev_file"
      done

      if [[ $_fg_fail -gt 0 ]]; then
        _fg_status="fail"
        _fg_details="${_fg_fail}/${_fg_total} test(s) failed in ${_fg_name}"
      elif [[ $_fg_pass -eq $_fg_total ]]; then
        _fg_status="pass"
        _fg_details="${_fg_total}/${_fg_total} test(s) passed in ${_fg_name}"
      elif [[ $_fg_pass -gt 0 ]]; then
        _fg_status="fail"
        _fg_details="${_fg_pass} passed, ${_fg_skip} skipped, ${_fg_fail} failed in ${_fg_name}"
      else
        _fg_status="fail"
        _fg_details="No passing tests in ${_fg_name}"
      fi
      _fg_evidence="${_fg_files[0]}"
    fi

    _report_add_feature_group "$_fg_name" "$_fg_status" "$_fg_details" "$_fg_evidence"
    log_info "[report] ${_fg_name}: ${_fg_status} ${_fg_details}"
  done

  # ── 7. Collect additional evidence files (non-group) ────────────────────────
  local _ev_f
  for _ev_f in "${EVIDENCE_DIR}"/*.json; do
    if [[ -f "$_ev_f" ]]; then
      local _ev_basename
      _ev_basename="$(basename "$_ev_f")"
      if [[ "$_ev_basename" == group-* ]]; then
        continue
      fi
      if [[ "$_ev_basename" == report.json || "$_ev_basename" == report.txt ]]; then
        continue
      fi
      _report_add_evidence "$_ev_f"
    fi
  done

  # ── 8. Compute overall verdict ──────────────────────────────────────────────
  _report_set_verdict
  local verdict
  verdict=$(_report_get_verdict)
  log_info "[report] Overall verdict: ${verdict}"

  # ── 9. Emit reports ─────────────────────────────────────────────────────────
  case "$REPORT_FORMAT" in
    json)
      _report_emit_json
      ;;
    text)
      _report_emit_text
      ;;
    json+text)
      _report_emit_json
      _report_emit_text
      ;;
    *)
      log_warn "[report] Unknown --report-format '${REPORT_FORMAT}', defaulting to json+text"
      _report_emit_json
      _report_emit_text
      ;;
  esac

  # ── 10. Print summary to stderr ─────────────────────────────────────────────
  local end_time
  end_time=$(date +%s)
  local duration=$(( end_time - _START_TIME ))

  local _report_output_dir
  _report_output_dir=$(_report_get_output_dir)

  echo "" >&2
  echo "=========================================" >&2
  echo " VPS Lifecycle Report" >&2
  echo " Mode: ${MODE} | VPS: ${VPS_IP}" >&2
  echo " Deploy: ${DEPLOY_MODE:-auto} | Duration: ${duration}s" >&2
  echo "=========================================" >&2

  local _entry _eg_group _eg_status _eg_bar
  for _entry in "${_REPORT_FEATURE_GROUPS[@]}"; do
    _eg_group=$(echo "$_entry" | jq -r '.group')
    _eg_status=$(echo "$_entry" | jq -r '.status')
    case "$_eg_status" in
      pass)         _eg_bar="████████" ;;
      fail)         _eg_bar="████░░░░" ;;
      skip-disabled) _eg_bar="░░░░░░░░" ;;
      not-run)      _eg_bar="────────" ;;
      *)            _eg_bar="????????" ;;
    esac
    echo "  ${_eg_bar} ${_eg_group}  [${_eg_status^^}]" >&2
  done

  if [[ ${#_REPORT_BLOCKERS[@]} -gt 0 ]]; then
    echo "─── Blockers ─────────────────────" >&2
    local _blk _blk_phase _blk_msg _blk_sev
    for _blk in "${_REPORT_BLOCKERS[@]}"; do
      _blk_phase=$(echo "$_blk" | jq -r '.phase')
      _blk_msg=$(echo "$_blk" | jq -r '.message')
      _blk_sev=$(echo "$_blk" | jq -r '.severity')
      echo "  [${_blk_sev^^}] ${_blk_phase}: ${_blk_msg}" >&2
    done
  fi

  echo "=========================================" >&2
  echo " Verdict: ${verdict}" >&2
  echo " Report:  ${_report_output_dir}/report.json" >&2
  echo " Text:    ${_report_output_dir}/report.txt" >&2
  echo "=========================================" >&2

  record_phase "report" "$verdict"

  # ── 11. Exit with appropriate code ──────────────────────────────────────────
  case "$verdict" in
    pass)    return 0 ;;
    partial) return 2 ;;
    fail)    return 1 ;;
    blocked) return 1 ;;
    *)       return 1 ;;
  esac
}

# ── Main: phase dispatch ─────────────────────────────────────────────────────
log_info "========================================="
log_info " ArmorClaw VPS Lifecycle Orchestrator"
log_info " Phase: ${PHASE} | Mode: ${MODE} | VPS: ${VPS_IP}"
log_info "========================================="

case "$PHASE" in
  detect)
    phase_detect
    ;;
  deploy)
    phase_deploy
    ;;
  admin-bootstrap)
    phase_admin_bootstrap
    ;;
  test-bootstrap)
    phase_test_bootstrap
    ;;
  validate)
    phase_validate
    ;;
  report)
    phase_report
    exit $?
    ;;
  all)
    # Run all phases in sequence
    phase_detect || true

    if [[ "$SKIP_DEPLOY" != "true" ]]; then
      phase_deploy || true
    fi

    if [[ "$SKIP_BOOTSTRAP" != "true" ]]; then
      phase_admin_bootstrap || true
      phase_test_bootstrap || true
    fi

    phase_validate || true
    phase_report || true
    ;;
esac

# ── Final exit code ──────────────────────────────────────────────────────────
_FINAL_EXIT=0
if [[ $_PHASE_FAIL -gt 0 && $_PHASE_PASS -gt 0 ]]; then
  _FINAL_EXIT=2  # partial
elif [[ $_PHASE_FAIL -gt 0 ]]; then
  _FINAL_EXIT=1  # fail
fi

log_info "========================================="
log_info " Lifecycle Complete: exit=${_FINAL_EXIT}"
log_info " ${_PHASE_PASS} PASS | ${_PHASE_FAIL} FAIL | ${_PHASE_SKIP} SKIP"
log_info "========================================="

exit $_FINAL_EXIT
