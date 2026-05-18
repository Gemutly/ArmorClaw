# rpc-methods.sh — Dynamic RPC method list extraction
#
# Sourced library (no shebang, no main block).
# Extracts the authoritative list of RPC method names from
# bridge/pkg/rpc/server.go registerHandlers() at source time.
#
# Provides:
#   _rpc_load_methods <repo_root>   — populates RPC_METHODS array
#   ${RPC_METHODS[@]}               — all registered method names
#   ${#RPC_METHODS[@]}              — total method count
#
# Usage:
#   source "${_SCRIPT_DIR}/lib/rpc-methods.sh"
#   _rpc_load_methods "$_REPO_ROOT"
#   echo "Total: ${#RPC_METHODS[@]}"

_rpc_load_methods() {
  local repo_root="${1:?Usage: _rpc_load_methods <repo_root>}"
  local server_go="${repo_root}/bridge/pkg/rpc/server.go"

  if [[ ! -f "$server_go" ]]; then
    echo "ERROR: $server_go not found" >&2
    RPC_METHODS=()
    return 1
  fi

  RPC_METHODS=()

  # Extract all method name keys from the registerHandlers() handler map.
  # Pattern matches lines like:  "method.name": s.handler,
  # Handles both s.handler and non-s. handler references.
  while IFS= read -r method; do
    RPC_METHODS+=("$method")
  done < <(
    awk '
      /h := map\[string\]HandlerFunc\{/,/^\t\}/ {
        if (match($0, /^\t\t"/)) {
          s = $0
          sub(/^[^"]*"/, "", s)
          sub(/".*/, "", s)
          print s
        }
      }
    ' "$server_go" | sort -u
  )
}
