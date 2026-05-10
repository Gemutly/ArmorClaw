#!/usr/bin/env bash
# assertions.sh — Matrix-aware assertion helpers for E2E tests
#
# Extends tests/lib/assert_json.sh with Matrix-specific assertions.
# Requires: GREEN, RED, NC, jq

# ── assert_notice <events_json_array> <expected_msg_substring> ─────────────────
# Checks that at least one m.notice event in the array contains the substring.
assert_notice() {
  local events="$1"
  local expected="$2"

  local count
  count=$(echo "$events" | jq -r '
    [.[] | select(.type == "m.notice")]
    | length
  ' 2>/dev/null)

  if [[ "${count:-0}" -eq 0 ]]; then
    echo -e "${RED}[FAIL]${NC} No m.notice events found"
    return 1
  fi

  local match
  match=$(echo "$events" | jq -r --arg sub "$expected" '
    [.[] | select(.type == "m.notice" and (.content.body // "" | test($sub; "i")))]
    | length
  ' 2>/dev/null)

  if [[ "${match:-0}" -gt 0 ]]; then
    echo -e "${GREEN}[PASS]${NC} m.notice contains '$expected' ($match match(es))"
    return 0
  else
    echo -e "${RED}[FAIL]${NC} No m.notice contains '$expected'"
    return 1
  fi
}

# ── assert_event_type <events_json_array> <expected_type> [min_count] ──────────
# Checks that at least min_count events of the given type exist.
assert_event_type() {
  local events="$1"
  local expected_type="$2"
  local min_count="${3:-1}"

  local count
  count=$(echo "$events" | jq -r --arg t "$expected_type" '
    [.[] | select(.type == $t)] | length
  ' 2>/dev/null)

  if [[ "${count:-0}" -ge "$min_count" ]]; then
    echo -e "${GREEN}[PASS]${NC} Found $count event(s) of type '$expected_type' (min: $min_count)"
    return 0
  else
    echo -e "${RED}[FAIL]${NC} Found ${count:-0} event(s) of type '$expected_type', expected >= $min_count"
    return 1
  fi
}

# ── assert_room_state <state_events_json> <event_type> <state_key> ─────────────
# Checks that a specific state event exists in room state.
assert_room_state() {
  local state="$1"
  local event_type="$2"
  local state_key="${3:-""}"

  local found
  found=$(echo "$state" | jq -r --arg t "$event_type" --arg k "$state_key" '
    [.[] | select(.type == $t and (.state_key // "") == $k)] | length
  ' 2>/dev/null)

  if [[ "${found:-0}" -gt 0 ]]; then
    echo -e "${GREEN}[PASS]${NC} Room has state event '$event_type' with key '$state_key'"
    return 0
  else
    echo -e "${RED}[FAIL]${NC} Room missing state event '$event_type' with key '$state_key'"
    return 1
  fi
}

# ── assert_http_status <response> <expected_code> ──────────────────────────────
# Checks curl response code. Usage: code=$(curl -s -o /tmp/r.json -w "%{http_code}" ...)
assert_http_status() {
  local actual="$1"
  local expected="$2"

  if [[ "$actual" == "$expected" ]]; then
    echo -e "${GREEN}[PASS]${NC} HTTP $actual"
    return 0
  else
    echo -e "${RED}[FAIL]${NC} HTTP $actual (expected $expected)"
    return 1
  fi
}

# ── assert_json_field <json> <field_path> <expected> ───────────────────────────
# Checks a nested field via jq path (e.g. ".result.tab_id").
assert_json_field() {
  local json="$1"
  local path="$2"
  local expected="$3"

  local actual
  actual=$(echo "$json" | jq -r "$path" 2>/dev/null)

  if [[ "$actual" == "$expected" ]]; then
    echo -e "${GREEN}[PASS]${NC} $path == '$expected'"
    return 0
  else
    echo -e "${RED}[FAIL]${NC} $path expected '$expected', got '$actual'"
    return 1
  fi
}
