# group-a-matrix.sh — Feature Group A: Matrix control plane tests
#
# Sourced library (no shebang, no main block).
# Functions prefixed with _ to avoid namespace collisions.
#
# Tests the Matrix control plane: login, send/receive, bot commands.
# Uses authenticated session from matrix-state.sh — never creates new sessions.
# All Matrix API calls go through SSH to localhost:6167 on the VPS.
#
# Usage:
#   source "${_SCRIPT_DIR}/feature-groups/group-a-matrix.sh"
#   _group_a_run --vps-ip 1.2.3.4 --ssh-key ~/.ssh/id_rsa --ssh-user root --output-dir /tmp/evidence

# ── Internal helpers ────────────────────────────────────────────────────────────

# _ga_ssh(command) — run command on VPS via SSH
# Uses _GA_VPS_IP, _GA_SSH_KEY, _GA_SSH_USER set by _group_a_run.
_ga_ssh() {
  ssh -i "$_GA_SSH_KEY" \
    -o ConnectTimeout=10 \
    -o BatchMode=yes \
    -o StrictHostKeyChecking=accept-new \
    "${_GA_SSH_USER}@${_GA_VPS_IP}" \
    "$@"
}

# _ga_matrix_curl(method, endpoint, [body]) — call Matrix API via SSH curl
# Method is GET or PUT. Endpoint is the path after /_matrix/client/v3/.
# Uses _GA_TOKEN for authentication.
# Returns response body on stdout.
_ga_matrix_curl() {
  local method="${1:?Usage: _ga_matrix_curl method endpoint [body]}"
  local endpoint="${2:?Usage: _ga_matrix_curl method endpoint [body]}"
  local body="${3:-}"

  local curl_args="-s -m 15"
  curl_args+=" -H 'Authorization: Bearer ${_GA_TOKEN}'"
  curl_args+=" -H 'Content-Type: application/json'"

  if [[ "$method" == "GET" ]]; then
    _ga_ssh "curl ${curl_args} 'http://localhost:6167${endpoint}'" 2>/dev/null
  else
    local escaped_body
    escaped_body=$(echo "$body" | sed "s/'/'\\\\''/g")
    _ga_ssh "curl ${curl_args} -X ${method} 'http://localhost:6167${endpoint}' -d '${escaped_body}'" 2>/dev/null
  fi
}

# _ga_send_message(room_id, body) — send m.text to a room, return event_id
_ga_send_message() {
  local room_id="${1:?Usage: _ga_send_message room_id body}"
  local body="${2:?Usage: _ga_send_message room_id body}"
  local txn_id="ga-$$-$(date +%s%N)"

  local escaped_body
  escaped_body=$(echo "$body" | jq -Rs . 2>/dev/null || echo "\"$body\"")

  local resp
  resp=$(_ga_matrix_curl PUT \
    "/_matrix/client/v3/rooms/${room_id}/send/m.room.message/${txn_id}" \
    "{\"msgtype\": \"m.text\", \"body\": ${escaped_body}}") || return 1

  echo "$resp" | jq -r '.event_id // empty' 2>/dev/null
}

# _ga_poll_for_response(room_id, since_token, expected_substring, timeout_secs)
# Poll /sync until an m.notice or m.text response appears in the room timeline.
# Returns: "next_batch_token<TAB>matching_event_json" on stdout, empty on timeout.
_ga_poll_for_response() {
  local room_id="${1:?Usage: _ga_poll_for_response room_id since expected timeout}"
  local since_token="${2:-}"
  local expected="${3:-}"
  local timeout_secs="${4:-30}"

  local elapsed=0
  local interval=2
  local current_since="$since_token"

  while [[ $elapsed -lt $timeout_secs ]]; do
    local endpoint="/_matrix/client/v3/sync?timeout=3000"
    if [[ -n "$current_since" ]]; then
      endpoint+="&since=${current_since}"
    fi

    local resp
    resp=$(_ga_matrix_curl GET "$endpoint") || true

    if [[ -n "$resp" ]]; then
      local next_batch
      next_batch=$(echo "$resp" | jq -r '.next_batch // empty' 2>/dev/null)
      [[ -n "$next_batch" ]] && current_since="$next_batch"

      # Extract timeline events for the target room
      local events
      events=$(echo "$resp" | jq -c --arg room "$room_id" \
        '.rooms.join[$room].timeline.events // []' 2>/dev/null)

      if [[ -n "$events" && "$events" != "[]" ]]; then
        local match
        if [[ -n "$expected" ]]; then
          match=$(echo "$events" | jq -c --arg sub "$expected" \
            '[.[] | select(
              (.type == "m.notice" or .type == "m.text")
              and ((.content.body // "") | test($sub; "i"))
            )][0]' 2>/dev/null)
        else
          match=$(echo "$events" | jq -c \
            '[.[] | select(.type == "m.notice" or .type == "m.text")][0]' 2>/dev/null)
        fi

        if [[ -n "$match" && "$match" != "null" ]]; then
          printf '%s\t%s' "$current_since" "$match"
          return 0
        fi
      fi
    fi

    sleep "$interval"
    elapsed=$((elapsed + interval))
  done

  return 1
}

# _ga_make_result(name, status, details, evidence_path, duration_ms)
# Produce a single test result JSON object.
_ga_make_result() {
  local name="${1:?}"
  local status="${2:?}"
  local details="${3:-}"
  local evidence_path="${4:-}"
  local duration_ms="${5:-0}"

  jq -nc \
    --arg name "$name" \
    --arg status "$status" \
    --arg details "$details" \
    --arg evidence "$evidence_path" \
    --argjson duration "$duration_ms" \
    '{name: $name, status: $status, details: $details, evidence_path: $evidence, duration_ms: $duration}'
}

# _ga_save_evidence(filename, content) — save evidence JSON to output dir
_ga_save_evidence() {
  local filename="${1:?Usage: _ga_save_evidence filename content}"
  local content="${2:-}"
  local filepath="${_GA_OUTPUT_DIR}/${filename}"
  mkdir -p "$_GA_OUTPUT_DIR"
  echo "$content" > "$filepath"
  echo "$filepath"
}

# ── Individual test functions ───────────────────────────────────────────────────

# _ga_test_login() — verify access_token works via whoami
_ga_test_login() {
  local start_ms
  start_ms=$(( $(date +%s%N) / 1000000 ))

  local resp
  resp=$(_ga_matrix_curl GET "/_matrix/client/v3/account/whoami") || {
    local end_ms=$(( $(date +%s%N) / 1000000 ))
    local evidence_path
    evidence_path=$(_ga_save_evidence "group-a-login.json" '{"error": "whoami request failed"}')
    _ga_make_result "matrix-login" "fail" "whoami request failed (curl error)" "$evidence_path" $(( end_ms - start_ms ))
    return 0
  }

  local evidence_path
  evidence_path=$(_ga_save_evidence "group-a-login.json" "$resp")

  local user_id
  user_id=$(echo "$resp" | jq -r '.user_id // empty' 2>/dev/null)

  local end_ms=$(( $(date +%s%N) / 1000000 ))

  if [[ -n "$user_id" && "$user_id" != "null" ]]; then
    _ga_make_result "matrix-login" "pass" "whoami confirmed: ${user_id}" "$evidence_path" $(( end_ms - start_ms ))
  else
    local errcode
    errcode=$(echo "$resp" | jq -r '.errcode // "unknown"' 2>/dev/null)
    _ga_make_result "matrix-login" "fail" "whoami returned error: ${errcode}" "$evidence_path" $(( end_ms - start_ms ))
  fi
}

# _ga_test_send_receive() — send message to test room, poll sync for it in timeline
_ga_test_send_receive() {
  local start_ms
  start_ms=$(( $(date +%s%N) / 1000000 ))

  # Do an initial sync to get a since token
  local init_sync
  init_sync=$(_ga_matrix_curl GET "/_matrix/client/v3/sync?timeout=0") || {
    local end_ms=$(( $(date +%s%N) / 1000000 ))
    local evidence_path
    evidence_path=$(_ga_save_evidence "group-a-send-receive.json" '{"error": "initial sync failed"}')
    _ga_make_result "send-receive" "fail" "initial sync failed" "$evidence_path" $(( end_ms - start_ms ))
    return 0
  }

  local since_token
  since_token=$(echo "$init_sync" | jq -r '.next_batch // empty' 2>/dev/null)

  # Send a test message
  local msg_body="[group-a-test] send-receive probe $(date +%s)"
  local event_id
  event_id=$(_ga_send_message "$_GA_ROOM_ID" "$msg_body") || {
    local end_ms=$(( $(date +%s%N) / 1000000 ))
    local evidence_path
    evidence_path=$(_ga_save_evidence "group-a-send-receive.json" '{"error": "send message failed"}')
    _ga_make_result "send-receive" "fail" "send message failed (curl error)" "$evidence_path" $(( end_ms - start_ms ))
    return 0
  }

  # Poll sync for the message to appear in the room timeline
  local poll_result
  poll_result=$(_ga_poll_for_response "$_GA_ROOM_ID" "$since_token" "group-a-test" 30) || {
    local end_ms=$(( $(date +%s%N) / 1000000 ))
    local evidence_path
    evidence_path=$(_ga_save_evidence "group-a-send-receive.json" \
      "$(jq -nc --arg eid "$event_id" --arg msg "$msg_body" \
        '{sent_event_id: $eid, sent_body: $msg, error: "message not found in timeline within 30s"}')")
    _ga_make_result "send-receive" "fail" "sent ${event_id} but message not found in timeline within 30s" "$evidence_path" $(( end_ms - start_ms ))
    return 0
  }

  local matched_event
  matched_event=$(echo "$poll_result" | cut -f2)

  local evidence_path
  evidence_path=$(_ga_save_evidence "group-a-send-receive.json" \
    "$(jq -nc --arg eid "$event_id" --arg msg "$msg_body" --arg match "$matched_event" \
      '{sent_event_id: $eid, sent_body: $msg, matched_event: ($match | fromjson)}')")

  local end_ms=$(( $(date +%s%N) / 1000000 ))
  _ga_make_result "send-receive" "pass" "sent ${event_id}, confirmed in timeline" "$evidence_path" $(( end_ms - start_ms ))
}

# _ga_test_status_command() — send /status, poll for m.notice response
_ga_test_status_command() {
  local start_ms
  start_ms=$(( $(date +%s%N) / 1000000 ))

  # Initial sync for since token
  local init_sync
  init_sync=$(_ga_matrix_curl GET "/_matrix/client/v3/sync?timeout=0") || {
    local end_ms=$(( $(date +%s%N) / 1000000 ))
    local evidence_path
    evidence_path=$(_ga_save_evidence "group-a-status.json" '{"error": "initial sync failed"}')
    _ga_make_result "status-command" "fail" "initial sync failed" "$evidence_path" $(( end_ms - start_ms ))
    return 0
  }
  local since_token
  since_token=$(echo "$init_sync" | jq -r '.next_batch // empty' 2>/dev/null)

  # Send /status
  local event_id
  event_id=$(_ga_send_message "$_GA_ROOM_ID" "/status") || {
    local end_ms=$(( $(date +%s%N) / 1000000 ))
    local evidence_path
    evidence_path=$(_ga_save_evidence "group-a-status.json" '{"error": "send /status failed"}')
    _ga_make_result "status-command" "fail" "send /status failed" "$evidence_path" $(( end_ms - start_ms ))
    return 0
  }

  # Poll for m.notice containing "status" or "bridge"
  local poll_result
  poll_result=$(_ga_poll_for_response "$_GA_ROOM_ID" "$since_token" "status|bridge" 30) || {
    local end_ms=$(( $(date +%s%N) / 1000000 ))
    local evidence_path
    evidence_path=$(_ga_save_evidence "group-a-status.json" \
      "$(jq -nc --arg eid "$event_id" '{sent_event_id: $eid, error: "no m.notice response within 30s"}')")
    _ga_make_result "status-command" "fail" "sent /status (${event_id}) but no m.notice response within 30s" "$evidence_path" $(( end_ms - start_ms ))
    return 0
  }

  local matched_event
  matched_event=$(echo "$poll_result" | cut -f2)
  local msgtype
  msgtype=$(echo "$matched_event" | jq -r '.content.msgtype // empty' 2>/dev/null)
  local body
  body=$(echo "$matched_event" | jq -r '.content.body // empty' 2>/dev/null)

  local evidence_path
  evidence_path=$(_ga_save_evidence "group-a-status.json" \
    "$(jq -nc --arg eid "$event_id" --arg mt "$msgtype" --arg body "$body" \
      '{sent_event_id: $eid, response_msgtype: $mt, response_body_preview: ($body | .[0:200])}')")

  local end_ms=$(( $(date +%s%N) / 1000000 ))

  if [[ "$msgtype" == "m.notice" ]]; then
    _ga_make_result "status-command" "pass" "m.notice received: ${body:0:80}" "$evidence_path" $(( end_ms - start_ms ))
  else
    _ga_make_result "status-command" "fail" "response msgtype='${msgtype}', expected m.notice" "$evidence_path" $(( end_ms - start_ms ))
  fi
}

# _ga_test_help_command() — send /help, poll for response
_ga_test_help_command() {
  local start_ms
  start_ms=$(( $(date +%s%N) / 1000000 ))

  local init_sync
  init_sync=$(_ga_matrix_curl GET "/_matrix/client/v3/sync?timeout=0") || {
    local end_ms=$(( $(date +%s%N) / 1000000 ))
    local evidence_path
    evidence_path=$(_ga_save_evidence "group-a-help.json" '{"error": "initial sync failed"}')
    _ga_make_result "help-command" "fail" "initial sync failed" "$evidence_path" $(( end_ms - start_ms ))
    return 0
  }
  local since_token
  since_token=$(echo "$init_sync" | jq -r '.next_batch // empty' 2>/dev/null)

  local event_id
  event_id=$(_ga_send_message "$_GA_ROOM_ID" "/help") || {
    local end_ms=$(( $(date +%s%N) / 1000000 ))
    local evidence_path
    evidence_path=$(_ga_save_evidence "group-a-help.json" '{"error": "send /help failed"}')
    _ga_make_result "help-command" "fail" "send /help failed" "$evidence_path" $(( end_ms - start_ms ))
    return 0
  }

  local poll_result
  poll_result=$(_ga_poll_for_response "$_GA_ROOM_ID" "$since_token" "help" 30) || {
    local end_ms=$(( $(date +%s%N) / 1000000 ))
    local evidence_path
    evidence_path=$(_ga_save_evidence "group-a-help.json" \
      "$(jq -nc --arg eid "$event_id" '{sent_event_id: $eid, error: "no response within 30s"}')")
    _ga_make_result "help-command" "fail" "sent /help (${event_id}) but no response within 30s" "$evidence_path" $(( end_ms - start_ms ))
    return 0
  }

  local matched_event
  matched_event=$(echo "$poll_result" | cut -f2)
  local msgtype
  msgtype=$(echo "$matched_event" | jq -r '.content.msgtype // empty' 2>/dev/null)
  local body
  body=$(echo "$matched_event" | jq -r '.content.body // empty' 2>/dev/null)

  local evidence_path
  evidence_path=$(_ga_save_evidence "group-a-help.json" \
    "$(jq -nc --arg eid "$event_id" --arg mt "$msgtype" --arg body "$body" \
      '{sent_event_id: $eid, response_msgtype: $mt, response_body_preview: ($body | .[0:200])}')")

  local end_ms=$(( $(date +%s%N) / 1000000 ))

  if [[ "$msgtype" == "m.notice" ]]; then
    _ga_make_result "help-command" "pass" "m.notice received: ${body:0:80}" "$evidence_path" $(( end_ms - start_ms ))
  else
    _ga_make_result "help-command" "fail" "response msgtype='${msgtype}', expected m.notice" "$evidence_path" $(( end_ms - start_ms ))
  fi
}

# _ga_test_agent_list() — send !agent list, poll for response containing agent info
_ga_test_agent_list() {
  local start_ms
  start_ms=$(( $(date +%s%N) / 1000000 ))

  local init_sync
  init_sync=$(_ga_matrix_curl GET "/_matrix/client/v3/sync?timeout=0") || {
    local end_ms=$(( $(date +%s%N) / 1000000 ))
    local evidence_path
    evidence_path=$(_ga_save_evidence "group-a-agent-list.json" '{"error": "initial sync failed"}')
    _ga_make_result "agent-list" "fail" "initial sync failed" "$evidence_path" $(( end_ms - start_ms ))
    return 0
  }
  local since_token
  since_token=$(echo "$init_sync" | jq -r '.next_batch // empty' 2>/dev/null)

  local event_id
  event_id=$(_ga_send_message "$_GA_ROOM_ID" "!agent list") || {
    local end_ms=$(( $(date +%s%N) / 1000000 ))
    local evidence_path
    evidence_path=$(_ga_save_evidence "group-a-agent-list.json" '{"error": "send !agent list failed"}')
    _ga_make_result "agent-list" "fail" "send !agent list failed" "$evidence_path" $(( end_ms - start_ms ))
    return 0
  }

  # Poll for any m.notice response (just verify routing works)
  local poll_result
  poll_result=$(_ga_poll_for_response "$_GA_ROOM_ID" "$since_token" "" 30) || {
    local end_ms=$(( $(date +%s%N) / 1000000 ))
    local evidence_path
    evidence_path=$(_ga_save_evidence "group-a-agent-list.json" \
      "$(jq -nc --arg eid "$event_id" '{sent_event_id: $eid, error: "no m.notice response within 30s"}')")
    _ga_make_result "agent-list" "fail" "sent !agent list (${event_id}) but no response within 30s" "$evidence_path" $(( end_ms - start_ms ))
    return 0
  }

  local matched_event
  matched_event=$(echo "$poll_result" | cut -f2)
  local msgtype
  msgtype=$(echo "$matched_event" | jq -r '.content.msgtype // empty' 2>/dev/null)
  local body
  body=$(echo "$matched_event" | jq -r '.content.body // empty' 2>/dev/null)

  local evidence_path
  evidence_path=$(_ga_save_evidence "group-a-agent-list.json" \
    "$(jq -nc --arg eid "$event_id" --arg mt "$msgtype" --arg body "$body" \
      '{sent_event_id: $eid, response_msgtype: $mt, response_body_preview: ($body | .[0:200])}')")

  local end_ms=$(( $(date +%s%N) / 1000000 ))

  if [[ "$msgtype" == "m.notice" ]]; then
    _ga_make_result "agent-list" "pass" "m.notice received: ${body:0:80}" "$evidence_path" $(( end_ms - start_ms ))
  else
    _ga_make_result "agent-list" "fail" "response msgtype='${msgtype}', expected m.notice" "$evidence_path" $(( end_ms - start_ms ))
  fi
}

# _ga_test_secretary_status() — send !secretary status, poll for response
_ga_test_secretary_status() {
  local start_ms
  start_ms=$(( $(date +%s%N) / 1000000 ))

  local init_sync
  init_sync=$(_ga_matrix_curl GET "/_matrix/client/v3/sync?timeout=0") || {
    local end_ms=$(( $(date +%s%N) / 1000000 ))
    local evidence_path
    evidence_path=$(_ga_save_evidence "group-a-secretary-status.json" '{"error": "initial sync failed"}')
    _ga_make_result "secretary-status" "fail" "initial sync failed" "$evidence_path" $(( end_ms - start_ms ))
    return 0
  }
  local since_token
  since_token=$(echo "$init_sync" | jq -r '.next_batch // empty' 2>/dev/null)

  local event_id
  event_id=$(_ga_send_message "$_GA_ROOM_ID" "!secretary status") || {
    local end_ms=$(( $(date +%s%N) / 1000000 ))
    local evidence_path
    evidence_path=$(_ga_save_evidence "group-a-secretary-status.json" '{"error": "send !secretary status failed"}')
    _ga_make_result "secretary-status" "fail" "send !secretary status failed" "$evidence_path" $(( end_ms - start_ms ))
    return 0
  }

  # Poll for any m.notice response
  local poll_result
  poll_result=$(_ga_poll_for_response "$_GA_ROOM_ID" "$since_token" "" 30) || {
    local end_ms=$(( $(date +%s%N) / 1000000 ))
    local evidence_path
    evidence_path=$(_ga_save_evidence "group-a-secretary-status.json" \
      "$(jq -nc --arg eid "$event_id" '{sent_event_id: $eid, error: "no m.notice response within 30s"}')")
    _ga_make_result "secretary-status" "fail" "sent !secretary status (${event_id}) but no response within 30s" "$evidence_path" $(( end_ms - start_ms ))
    return 0
  }

  local matched_event
  matched_event=$(echo "$poll_result" | cut -f2)
  local msgtype
  msgtype=$(echo "$matched_event" | jq -r '.content.msgtype // empty' 2>/dev/null)
  local body
  body=$(echo "$matched_event" | jq -r '.content.body // empty' 2>/dev/null)

  local evidence_path
  evidence_path=$(_ga_save_evidence "group-a-secretary-status.json" \
    "$(jq -nc --arg eid "$event_id" --arg mt "$msgtype" --arg body "$body" \
      '{sent_event_id: $eid, response_msgtype: $mt, response_body_preview: ($body | .[0:200])}')")

  local end_ms=$(( $(date +%s%N) / 1000000 ))

  if [[ "$msgtype" == "m.notice" ]]; then
    _ga_make_result "secretary-status" "pass" "m.notice received: ${body:0:80}" "$evidence_path" $(( end_ms - start_ms ))
  else
    _ga_make_result "secretary-status" "fail" "response msgtype='${msgtype}', expected m.notice" "$evidence_path" $(( end_ms - start_ms ))
  fi
}

# ── Main entry point ────────────────────────────────────────────────────────────

# _group_a_run([--vps-ip IP] [--ssh-key PATH] [--ssh-user USER] [--output-dir DIR])
# Executes all Matrix control plane tests and returns JSON results to stdout.
#
# Parameters (all optional if corresponding env vars are set):
#   --vps-ip      VPS IP address (default: $VPS_IP)
#   --ssh-key     SSH private key path (default: $SSH_KEY_PATH)
#   --ssh-user    SSH username (default: $SSH_USER or "root")
#   --output-dir  Directory for evidence files (default: .sisyphus/evidence/vps-lifecycle)
#
# Requires:
#   - scripts/lib/matrix-state.sh already sourced (provides _matrix_load_state)
#   - scripts/lib/test-session-bootstrap.sh already sourced if bootstrapping needed
#
# Returns: JSON array of test results on stdout. Each element:
#   {name, status: pass|fail|skip, details, evidence_path, duration_ms}
_group_a_run() {
  # Parse arguments
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --vps-ip)     _GA_VPS_IP="$2";     shift 2 ;;
      --ssh-key)    _GA_SSH_KEY="$2";    shift 2 ;;
      --ssh-user)   _GA_SSH_USER="$2";   shift 2 ;;
      --output-dir) _GA_OUTPUT_DIR="$2"; shift 2 ;;
      *) echo "[group-a] WARN: unknown argument: $1" >&2; shift ;;
    esac
  done

  # Apply defaults from env vars
  _GA_VPS_IP="${_GA_VPS_IP:-${VPS_IP:-}}"
  _GA_SSH_KEY="${_GA_SSH_KEY:-${SSH_KEY_PATH:-}}"
  _GA_SSH_USER="${_GA_SSH_USER:-${SSH_USER:-root}}"
  _GA_OUTPUT_DIR="${_GA_OUTPUT_DIR:-${_SCRIPT_DIR:-.}/../.sisyphus/evidence/vps-lifecycle}"

  # Validate required params
  if [[ -z "$_GA_VPS_IP" ]]; then
    echo '[group-a] ERROR: --vps-ip or VPS_IP required' >&2
    jq -nc '[{name:"group-a-matrix",status:"fail",details:"--vps-ip not provided",evidence_path:"",duration_ms:0}]'
    return 1
  fi
  if [[ -z "$_GA_SSH_KEY" ]]; then
    echo '[group-a] ERROR: --ssh-key or SSH_KEY_PATH required' >&2
    jq -nc '[{name:"group-a-matrix",status:"fail",details:"--ssh-key not provided",evidence_path:"",duration_ms:0}]'
    return 1
  fi

  mkdir -p "$_GA_OUTPUT_DIR"

  # Load Matrix session state
  if ! _matrix_load_state 2>/dev/null; then
    # No valid session — all tests fail with clear message
    echo "[group-a] Matrix session not bootstrapped — failing all tests" >&2
    local fail_result
    fail_result=$(_ga_make_result "group-a-matrix-all" "fail" "Matrix session not bootstrapped — run test-session-bootstrap first" "" 0)
    jq -nc --argjson r "$fail_result" '[$r]'
    return 1
  fi

  # Set session variables for helper functions
  _GA_TOKEN="${_TEST_ACCESS_TOKEN}"
  _GA_ROOM_ID="${_TEST_ROOM_ID}"

  # Validate session has the minimum fields
  if [[ -z "$_GA_TOKEN" || -z "$_GA_ROOM_ID" ]]; then
    echo "[group-a] Loaded state is incomplete (missing token or room_id)" >&2
    local fail_result
    fail_result=$(_ga_make_result "group-a-matrix-all" "fail" "Matrix session not bootstrapped — access_token or test_room_id missing" "" 0)
    jq -nc --argjson r "$fail_result" '[$r]'
    return 1
  fi

  echo "[group-a] Session loaded: user=${_TEST_USER_ID:-?} room=${_GA_ROOM_ID}" >&2

  # Run all 6 tests, collecting results
  local results=()

  results+=( "$(_ga_test_login)" )
  results+=( "$(_ga_test_send_receive)" )
  results+=( "$(_ga_test_status_command)" )
  results+=( "$(_ga_test_help_command)" )
  results+=( "$(_ga_test_agent_list)" )
  results+=( "$(_ga_test_secretary_status)" )

  # Build JSON array from results
  local json_array="["
  local first=true
  for r in "${results[@]}"; do
    if [[ "$first" == "true" ]]; then
      first=false
    else
      json_array+=","
    fi
    json_array+="$r"
  done
  json_array+="]"

  # Output results
  echo "$json_array" | jq '.'

  # Determine overall status
  local fail_count
  fail_count=$(echo "$json_array" | jq '[.[] | select(.status == "fail")] | length')
  if [[ "$fail_count" -gt 0 ]]; then
    echo "[group-a] ${#results[@]} tests, ${fail_count} failed" >&2
    return 1
  fi

  echo "[group-a] All ${#results[@]} tests passed" >&2
  return 0
}
