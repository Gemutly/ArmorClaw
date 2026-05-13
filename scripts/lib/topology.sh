# topology.sh — VPS topology detection library for ArmorClaw
#
# Sourced library — no shebang, no main block. Functions prefixed with _.
# Detects existing ArmorClaw components on a VPS and classifies the topology
# to recommend the appropriate deployment mode. All detection is strictly
# read-only (no changes on the VPS).
#
# Usage:
#   source "$(dirname "$0")/lib/topology.sh"
#   _topology_detect
#   _topology_classify
#   _topology_recommend_mode
#   _topology_to_json

set -uo pipefail

# ── Locate repo root ─────────────────────────────────────────────────────────
_TOPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_TOPO_REPO_ROOT="$(cd "${_TOPO_DIR}/../.." && pwd)"

# ── Source existing infrastructure (provides ssh_vps, log_info, log_pass, etc.) ─
if [[ -f "${_TOPO_REPO_ROOT}/scripts/lib/contract.sh" ]]; then
  source "${_TOPO_REPO_ROOT}/scripts/lib/contract.sh"
fi

# ── Port list to probe ──────────────────────────────────────────────────────
_TOPO_PORTS=(6167 8443 8080 8448 5000)

# ── Detection result storage (populated by _topology_detect) ────────────────
_TOPO_SYSTEMD_BRIDGE=false
_TOPO_DOCKER_CONDUIT=false
_TOPO_DOCKER_QUICKSTART=false
_TOPO_PORTS_OCCUPIED="{}"
_TOPO_HAS_ENV=false
_TOPO_DOCKER_COMPOSE=false
_TOPO_CONDUIT_IMAGE=""
_TOPO_QUICKSTART_IMAGE=""
_TOPO_CLASSIFICATION="unknown"

# ── _topology_detect ────────────────────────────────────────────────────────
# SSH to VPS and detect existing ArmorClaw components.
# Populates _TOPO_* global variables. Returns 0 on success, 1 on SSH failure.
#
# Detection is strictly read-only: no files are written, no services changed.
_topology_detect() {
  # Verify ssh_vps is available
  if ! declare -f ssh_vps >/dev/null 2>&1; then
    echo "ERROR: ssh_vps() not available. Source tests/lib/load_env.sh or scripts/lib/contract.sh first." >&2
    return 1
  fi

  # Verify SSH connectivity
  if ! ssh_vps "true" 2>/dev/null; then
    echo "ERROR: SSH connectivity to VPS failed" >&2
    return 1
  fi

  # ── 1. Detect systemd bridge service ────────────────────────────────────
  local systemd_status
  systemd_status=$(ssh_vps "systemctl is-active armorclaw-bridge.service 2>/dev/null" 2>/dev/null || echo "")
  if [[ "$systemd_status" == "active" ]]; then
    _TOPO_SYSTEMD_BRIDGE=true
  fi

  # ── 2. Detect Docker containers ─────────────────────────────────────────
  local docker_names
  docker_names=$(ssh_vps "docker ps --format '{{.Names}}' 2>/dev/null" 2>/dev/null || echo "")

  # Conduit container (standalone)
  if echo "$docker_names" | grep -qi "conduit"; then
    _TOPO_DOCKER_CONDUIT=true
    _TOPO_CONDUIT_IMAGE=$(ssh_vps "docker ps --filter 'name=conduit' --format '{{.Image}}' 2>/dev/null | head -1" 2>/dev/null || echo "")
  fi

  # Quickstart container (all-in-one)
  if echo "$docker_names" | grep -qi "quickstart\|armorclaw"; then
    _TOPO_DOCKER_QUICKSTART=true
    _TOPO_QUICKSTART_IMAGE=$(ssh_vps "docker ps --filter 'name=quickstart' --format '{{.Image}}' 2>/dev/null | head -1" 2>/dev/null || echo "")
  fi

  # ── 3. Detect occupied ports ────────────────────────────────────────────
  local port json_parts=()
  for port in "${_TOPO_PORTS[@]}"; do
    local listener
    listener=$(ssh_vps "ss -tlnp 2>/dev/null | grep -c ':${port} ' || echo '0'" 2>/dev/null || echo "0")
    local occupied=false
    if [[ "$listener" -gt 0 ]]; then
      occupied=true
    fi
    json_parts+=("\"${port}\": ${occupied}")
  done
  _TOPO_PORTS_OCCUPIED="{${json_parts[*]// /}}"
  # Fix spacing: join with commas
  _TOPO_PORTS_OCCUPIED="{$(IFS=,; echo "${json_parts[*]}")}"

  # ── 4. Detect .env with API keys ────────────────────────────────────────
  local env_paths=(
    "/etc/armorclaw/.env"
    "/opt/armorclaw/.env"
    "${HOME}/.armorclaw/.env"
  )
  local env_path
  for env_path in "${env_paths[@]}"; do
    if ssh_vps "test -f '${env_path}' && grep -qE '(OPENROUTER_API_KEY|OPEN_AI_KEY|ZAI_API_KEY)' '${env_path}'" 2>/dev/null; then
      _TOPO_HAS_ENV=true
      break
    fi
  done

  # ── 5. Detect docker-compose installations ──────────────────────────────
  local compose_files=(
    "/opt/armorclaw/docker-compose.yml"
    "/opt/armorclaw/docker-compose.yaml"
    "/etc/armorclaw/docker-compose.yml"
    "/etc/armorclaw/docker-compose.yaml"
  )
  local compose_file
  for compose_file in "${compose_files[@]}"; do
    if ssh_vps "test -f '${compose_file}'" 2>/dev/null; then
      _TOPO_DOCKER_COMPOSE=true
      break
    fi
  done

  return 0
}

# ── _topology_classify ──────────────────────────────────────────────────────
# Classify the VPS topology based on detection results.
# Returns one of: fresh, native-systemd, docker-conduit, docker-quickstart,
# mixed, unknown.
# Echoes the classification string to stdout.
_topology_classify() {
  local has_systemd="$_TOPO_SYSTEMD_BRIDGE"
  local has_conduit="$_TOPO_DOCKER_CONDUIT"
  local has_quickstart="$_TOPO_DOCKER_QUICKSTART"
  local has_compose="$_TOPO_DOCKER_COMPOSE"

  # Count active components
  local active_count=0
  [[ "$has_systemd" == "true" ]] && ((active_count++)) || true
  [[ "$has_conduit" == "true" ]] && ((active_count++)) || true
  [[ "$has_quickstart" == "true" ]] && ((active_count++)) || true

  # Nothing found → fresh
  if [[ $active_count -eq 0 && "$has_compose" == "false" ]]; then
    _TOPO_CLASSIFICATION="fresh"
    echo "fresh"
    return 0
  fi

  # Multiple overlapping components → mixed
  if [[ $active_count -gt 1 ]]; then
    _TOPO_CLASSIFICATION="mixed"
    echo "mixed"
    return 0
  fi

  # Single component classification
  if [[ "$has_systemd" == "true" ]]; then
    _TOPO_CLASSIFICATION="native-systemd"
    echo "native-systemd"
    return 0
  fi

  if [[ "$has_conduit" == "true" ]]; then
    _TOPO_CLASSIFICATION="docker-conduit"
    echo "docker-conduit"
    return 0
  fi

  if [[ "$has_quickstart" == "true" ]]; then
    _TOPO_CLASSIFICATION="docker-quickstart"
    echo "docker-quickstart"
    return 0
  fi

  # Only docker-compose files, no running containers
  if [[ "$has_compose" == "true" ]]; then
    _TOPO_CLASSIFICATION="docker-quickstart"
    echo "docker-quickstart"
    return 0
  fi

  _TOPO_CLASSIFICATION="unknown"
  echo "unknown"
  return 0
}

# ── _topology_recommend_mode ────────────────────────────────────────────────
# Recommend a deployment mode based on the classified topology.
# Returns one of: replace-existing, reuse-existing-matrix, side-by-side,
# fresh-install.
# Echoes the recommendation string to stdout.
_topology_recommend_mode() {
  case "$_TOPO_CLASSIFICATION" in
    fresh)
      echo "fresh-install"
      ;;
    native-systemd)
      echo "replace-existing"
      ;;
    docker-conduit)
      # If only Conduit is running, we can reuse it
      if [[ "$_TOPO_SYSTEMD_BRIDGE" == "false" ]]; then
        echo "reuse-existing-matrix"
      else
        echo "replace-existing"
      fi
      ;;
    docker-quickstart)
      echo "replace-existing"
      ;;
    mixed)
      # Mixed topologies need side-by-side to avoid conflicts
      echo "side-by-side"
      ;;
    *)
      # Unknown — safest default is fresh install
      echo "fresh-install"
      ;;
  esac
}

# ── _topology_to_json ──────────────────────────────────────────────────────
# Output the full topology detection result as structured JSON to stdout.
# Requires jq on the local machine.
_topology_to_json() {
  local classification="${_TOPO_CLASSIFICATION}"
  local recommendation
  recommendation=$(_topology_recommend_mode)

  # Parse occupied ports into proper JSON object
  local ports_json
  ports_json="{}"
  local port
  for port in "${_TOPO_PORTS[@]}"; do
    local occupied
    occupied=$(echo "$_TOPO_PORTS_OCCUPIED" | jq -r ".\"${port}\" // false" 2>/dev/null || echo "false")
    ports_json=$(echo "$ports_json" | jq --arg port "$port" --argjson occ "$occupied" '. + {($port): $occ}' 2>/dev/null || echo "{}")
  done

  # If jq parsing failed, rebuild from raw data
  if [[ "$ports_json" == "{}" ]]; then
    local port_parts=()
    for port in "${_TOPO_PORTS[@]}"; do
      local occ=false
      echo "$_TOPO_PORTS_OCCUPIED" | grep -q "\"${port}\": *true" && occ=true || true
      port_parts+=("\"${port}\": ${occ}")
    done
    ports_json="{$(IFS=,; echo "${port_parts[*]}")}"
  fi

  jq -n \
    --arg classification "$classification" \
    --arg recommendation "$recommendation" \
    --argjson systemd_bridge "$_TOPO_SYSTEMD_BRIDGE" \
    --argjson docker_conduit "$_TOPO_DOCKER_CONDUIT" \
    --argjson docker_quickstart "$_TOPO_DOCKER_QUICKSTART" \
    --argjson has_env "$_TOPO_HAS_ENV" \
    --argjson docker_compose "$_TOPO_DOCKER_COMPOSE" \
    --arg conduit_image "$_TOPO_CONDUIT_IMAGE" \
    --arg quickstart_image "$_TOPO_QUICKSTART_IMAGE" \
    --argjson ports_occupied "$ports_json" \
    '{
      classification: $classification,
      recommendation: $recommendation,
      detected: {
        systemd_bridge: $systemd_bridge,
        docker_conduit: $docker_conduit,
        docker_quickstart: $docker_quickstart,
        has_env: $has_env,
        docker_compose: $docker_compose,
        conduit_image: $conduit_image,
        quickstart_image: $quickstart_image,
        ports_occupied: $ports_occupied
      },
      timestamp: (now | todate)
    }'
}
