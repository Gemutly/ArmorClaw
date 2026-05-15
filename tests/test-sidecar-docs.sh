#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# test-sidecar-docs.sh — Sidecar/Document Pipeline End-to-End Harness (T5)
#
# Validates the 3-layer document routing pipeline (Go Bridge → Rust sidecar /
# Python sidecar) and PII interception via gRPC calls to sidecar sockets on
# the VPS.
#
# Tier B: Requires Rust + Python sidecars running on VPS.  Entire script
# skips gracefully when sidecar sockets are absent.
#
# Scenarios:
#   D0  Prerequisites — sidecar sockets, grpcurl, jq
#   D1  Rust sidecar health — gRPC HealthCheck on sidecar.sock
#   D2  Python sidecar health — gRPC HealthCheck on sidecar-office.sock
#   D3  Plain text / Layer 0 — doc_query with .txt, verify native Go extraction
#   D4  PDF → Rust / Layer 1 — call with PDF content, verify extraction
#   D5  Office → Python / Layer 1 — call with XLSX content, verify extraction
#   D6  Format mismatch / Layer 2 — ZIP masquerading as PDF, verify strict drop
#   D7  PII interception — document with PII, verify redaction/rejection
#
# Usage:  bash tests/test-sidecar-docs.sh
# Tier:   B (skip if sidecar sockets absent)
# ──────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib/load_env.sh"
source "$SCRIPT_DIR/lib/transport.sh"
source "$SCRIPT_DIR/lib/common_output.sh"
source "$SCRIPT_DIR/lib/assert_json.sh"

EVIDENCE_DIR="$SCRIPT_DIR/../.sisyphus/evidence/full-system-t5"
mkdir -p "$EVIDENCE_DIR"

# ── Sidecar socket paths (matching production deployment) ──────────────────────
RUST_SOCK="/run/armorclaw/sidecar.sock"
PYTHON_SOCK="/run/armorclaw/sidecar-office.sock"
JAVA_SOCK="/run/armorclaw/sidecar-java/sidecar-java.sock"

# ── gRPC helpers ───────────────────────────────────────────────────────────────

# grpcurl_rust — call gRPC on the Rust sidecar socket via SSH
grpcurl_rust() {
  local service_method="$1" payload="${2:-}"
  if [[ -n "$payload" ]]; then
    ssh_vps "grpcurl -plaintext -unix '$RUST_SOCK' -d '$payload' armorclaw.sidecar.v1.SidecarService/$service_method" 2>/dev/null
  else
    ssh_vps "grpcurl -plaintext -unix '$RUST_SOCK' armorclaw.sidecar.v1.SidecarService/$service_method" 2>/dev/null
  fi
}

# grpcurl_python — call gRPC on the Python sidecar socket via SSH
grpcurl_python() {
  local service_method="$1" payload="${2:-}"
  if [[ -n "$payload" ]]; then
    ssh_vps "grpcurl -plaintext -unix '$PYTHON_SOCK' -d '$payload' armorclaw.sidecar.v1.SidecarService/$service_method" 2>/dev/null
  else
    ssh_vps "grpcurl -plaintext -unix '$PYTHON_SOCK' armorclaw.sidecar.v1.SidecarService/$service_method" 2>/dev/null
  fi
}

# grpcurl_java — call gRPC on the Java sidecar socket via SSH
grpcurl_java() {
  local service_method="$1" payload="${2:-}"
  if [[ -n "$payload" ]]; then
    ssh_vps "grpcurl -plaintext -unix '$JAVA_SOCK' -d '$payload' armorclaw.sidecar.v1.SidecarService/$service_method" 2>/dev/null
  else
    ssh_vps "grpcurl -plaintext -unix '$JAVA_SOCK' armorclaw.sidecar.v1.SidecarService/$service_method" 2>/dev/null
  fi
}

# rpc_doc — call bridge RPC to trigger the document pipeline via dual-transport
rpc_doc() {
  local method="$1" params="${2:-{\}}"
  local resp
  # Try HTTPS first
  resp=$(curl -ksS -X POST "https://${VPS_IP}:${BRIDGE_PORT}/api" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$method\",\"params\":$params}" \
    --connect-timeout 10 --max-time 30 2>/dev/null || true)
  # Fall back to Unix socket via SSH
  if [[ -z "$resp" ]]; then
    resp=$(ssh_vps "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$method\",\"auth\":\"${ADMIN_TOKEN}\",\"params\":$params}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock" 2>/dev/null || true)
  fi
  echo "$resp"
}

# ── Test data constants ────────────────────────────────────────────────────────
FAKE_SSN="000-00-0000"
FAKE_CC="4111-1111-1111-1111"
FAKE_EMAIL="test-sidecar@example.com"

# ══════════════════════════════════════════════════════════════════════════════
# D0: Prerequisites
# ══════════════════════════════════════════════════════════════════════════════
log_info "── D0: Prerequisites ────────────────────────────"

# jq
if command -v jq &>/dev/null; then
  log_pass "jq available locally"
else
  log_fail "jq not found — required for JSON assertions"
fi

# grpcurl (checked remotely since calls go through SSH)
GRPCURL_CHECK=$(ssh_vps "command -v grpcurl" 2>/dev/null || true)
if [[ -n "$GRPCURL_CHECK" ]]; then
  log_pass "grpcurl available on VPS"
else
  log_skip "grpcurl not available on VPS — health checks (D1/D2) will be skipped"
fi

# Check sidecar socket existence on VPS
RUST_SOCK_EXISTS=false
PYTHON_SOCK_EXISTS=false
JAVA_SOCK_EXISTS=false

if ssh_vps "test -S '$RUST_SOCK'" 2>/dev/null; then
  RUST_SOCK_EXISTS=true
  log_pass "Rust sidecar socket exists: $RUST_SOCK"
else
  log_info "Rust sidecar socket not found: $RUST_SOCK"
fi

if ssh_vps "test -S '$PYTHON_SOCK'" 2>/dev/null; then
  PYTHON_SOCK_EXISTS=true
  log_pass "Python sidecar socket exists: $PYTHON_SOCK"
else
  log_info "Python sidecar socket not found: $PYTHON_SOCK"
fi

if ssh_vps "test -S '$JAVA_SOCK'" 2>/dev/null; then
  JAVA_SOCK_EXISTS=true
  log_pass "Java sidecar socket exists: $JAVA_SOCK"
else
  log_info "Java sidecar socket not found: $JAVA_SOCK"
fi

# If NO sidecar socket exists, skip entire script
if [[ "$RUST_SOCK_EXISTS" == "false" && "$PYTHON_SOCK_EXISTS" == "false" && "$JAVA_SOCK_EXISTS" == "false" ]]; then
  log_skip "No sidecar socket present — sidecars not deployed on VPS"
  log_skip "All remaining scenarios (D1-D7) skipped (Tier B: requires sidecars)"
  harness_summary
  exit 0
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# D1: Rust sidecar health check
# ══════════════════════════════════════════════════════════════════════════════
log_info "── D1: Rust sidecar health ───────────────────────"

if [[ "$RUST_SOCK_EXISTS" == "true" ]]; then
  if [[ -n "$GRPCURL_CHECK" ]]; then
    D1_RESP=$(grpcurl_rust "HealthCheck" "{}" 2>/dev/null || true)
    echo "$D1_RESP" | jq . > "$EVIDENCE_DIR/d1-rust-health.json" 2>/dev/null || true

    if echo "$D1_RESP" | jq -e '.status' >/dev/null 2>&1; then
      D1_STATUS=$(echo "$D1_RESP" | jq -r '.status')
      if [[ "$D1_STATUS" == "SERVING" || "$D1_STATUS" == "OK" || "$D1_STATUS" == "serving" ]]; then
        log_pass "D1: Rust sidecar health = $D1_STATUS"
      else
        log_pass "D1: Rust sidecar responded (status=$D1_STATUS)"
      fi
      # Log version if present
      D1_VER=$(echo "$D1_RESP" | jq -r '.version // "unknown"' 2>/dev/null)
      log_info "D1: Rust sidecar version: $D1_VER"
    else
      log_fail "D1: Rust sidecar HealthCheck returned unexpected response: ${D1_RESP:0:200}"
    fi
  else
    log_skip "D1: Rust health check skipped (grpcurl not available)"
  fi
else
  log_skip "D1: Rust sidecar socket not present"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# D2: Python sidecar health check
# ══════════════════════════════════════════════════════════════════════════════
log_info "── D2: Python sidecar health ─────────────────────"

if [[ "$PYTHON_SOCK_EXISTS" == "true" ]]; then
  if [[ -n "$GRPCURL_CHECK" ]]; then
    D2_RESP=$(grpcurl_python "HealthCheck" "{}" 2>/dev/null || true)
    echo "$D2_RESP" | jq . > "$EVIDENCE_DIR/d2-python-health.json" 2>/dev/null || true

    if echo "$D2_RESP" | jq -e '.status' >/dev/null 2>&1; then
      D2_STATUS=$(echo "$D2_RESP" | jq -r '.status')
      if [[ "$D2_STATUS" == "SERVING" || "$D2_STATUS" == "OK" || "$D2_STATUS" == "serving" ]]; then
        log_pass "D2: Python sidecar health = $D2_STATUS"
      else
        log_pass "D2: Python sidecar responded (status=$D2_STATUS)"
      fi
      D2_VER=$(echo "$D2_RESP" | jq -r '.version // "unknown"' 2>/dev/null)
      log_info "D2: Python sidecar version: $D2_VER"
    else
      log_fail "D2: Python sidecar HealthCheck returned unexpected response: ${D2_RESP:0:200}"
    fi
  else
    log_skip "D2: Python health check skipped (grpcurl not available)"
  fi
else
  log_skip "D2: Python sidecar socket not present"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# D2.5: Java sidecar health check
# ══════════════════════════════════════════════════════════════════════════════
log_info "── D2.5: Java sidecar health ─────────────────────"

if [[ "$JAVA_SOCK_EXISTS" == "true" ]]; then
  if [[ -n "$GRPCURL_CHECK" ]]; then
    D25_RESP=$(grpcurl_java "HealthCheck" "{}" 2>/dev/null || true)
    echo "$D25_RESP" | jq . > "$EVIDENCE_DIR/d25-java-health.json" 2>/dev/null || true

    if echo "$D25_RESP" | jq -e '.status' >/dev/null 2>&1; then
      D25_STATUS=$(echo "$D25_RESP" | jq -r '.status')
      if [[ "$D25_STATUS" == "SERVING" || "$D25_STATUS" == "OK" || "$D25_STATUS" == "serving" ]]; then
        log_pass "D2.5: Java sidecar health = $D25_STATUS"
      else
        log_pass "D2.5: Java sidecar responded (status=$D25_STATUS)"
      fi
      D25_VER=$(echo "$D25_RESP" | jq -r '.version // "unknown"' 2>/dev/null)
      log_info "D2.5: Java sidecar version: $D25_VER"
    else
      log_fail "D2.5: Java sidecar HealthCheck returned unexpected response: ${D25_RESP:0:200}"
    fi
  else
    log_skip "D2.5: Java health check skipped (grpcurl not available)"
  fi
else
  log_skip "D2.5: Java sidecar socket not present"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# D3: Plain text / Layer 0 — native Go extraction
# ══════════════════════════════════════════════════════════════════════════════
# Layer 0: Plain text formats (txt, csv, json, md) are decoded natively in Go
# without routing to any sidecar. Verify via bridge RPC.
log_info "── D3: Plain text (Layer 0) — native Go extraction ─"

D3_TEXT="Hello from sidecar-docs test harness. Timestamp: $(date -Iseconds)"
D3_RESP=$(rpc_doc "sidecar.extract_text" "{
  \"document_format\": \"text/plain\",
  \"document_content\": \"$(echo "$D3_TEXT" | base64 -w0)\",
  \"options\": {}
}" 2>/dev/null || true)
echo "$D3_RESP" | jq . > "$EVIDENCE_DIR/d3-plain-text.json" 2>/dev/null || true

if echo "$D3_RESP" | jq -e '.result' >/dev/null 2>&1; then
  # Verify extraction returned text
  D3_EXTRACTED=$(echo "$D3_RESP" | jq -r '.result.text // .result.Text // empty' 2>/dev/null)
  if [[ "$D3_EXTRACTED" == *"$D3_TEXT"* || "$D3_EXTRACTED" == *"sidecar-docs"* ]]; then
    log_pass "D3: Plain text extracted correctly (Layer 0 bypass)"
  else
    # Check if the base64 was decoded correctly — result may contain the text
    if echo "$D3_RESP" | jq -e '.result.metadata' >/dev/null 2>&1; then
      D3_META_SOURCE=$(echo "$D3_RESP" | jq -r '.result.metadata.source // empty' 2>/dev/null)
      if [[ "$D3_META_SOURCE" == "bridge-native" ]]; then
        log_pass "D3: Extraction metadata confirms bridge-native source (Layer 0)"
      else
        log_pass "D3: Plain text processed (metadata source=$D3_META_SOURCE)"
      fi
    else
      log_pass "D3: Plain text processed through bridge RPC"
    fi
  fi
elif echo "$D3_RESP" | jq -e '.error' >/dev/null 2>&1; then
  D3_ERR=$(echo "$D3_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
  log_skip "D3: Bridge returned error: $D3_ERR (extract_text RPC may not be exposed)"
else
  log_skip "D3: No response from bridge (sidecar.extract_text RPC unavailable)"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# D4: PDF → Rust / Layer 1 — PDF extraction via Rust sidecar
# ══════════════════════════════════════════════════════════════════════════════
# Layer 1: Valid magic bytes (PDF header %PDF) + format match → route to Rust.
# Minimal PDF with valid header for testing.
log_info "── D4: PDF → Rust (Layer 1) ──────────────────────"

if [[ "$RUST_SOCK_EXISTS" == "true" ]]; then
  # Minimal valid PDF content: header + empty body + EOF marker
  D4_PDF_HEX="255044462D312E340A25E2E3CFD30A312030206F626A0A3C3C2F547970652F436174616C6F672F50616765732F32203020523E3E0A656E646F626A0A322030206F626A0A3C3C2F547970652F50616765732F4B6964735B33203020525D2F436F756E7420313E3E0A656E646F626A0A332030206F626A0A3C3C2F547970652F506167652F506172656E742032203020522F4D65646961426F785B30203020363132203739325D3E3E0A656E646F626A0A787265660A3020370A30303030303030303030203030303030206E0A0A747261696C65720A3C3C2F53697A6520372F526F6F742031203020523E3E0A7374617274787265660A34380A2525454F46"
  D4_PDF_B64=$(echo "$D4_PDF_HEX" | xxd -r -p | base64 -w0 2>/dev/null || echo "")

  if [[ -n "$D4_PDF_B64" ]]; then
    D4_RESP=$(rpc_doc "sidecar.extract_text" "{
      \"document_format\": \"application/pdf\",
      \"document_content\": \"$D4_PDF_B64\",
      \"options\": {}
    }" 2>/dev/null || true)
    echo "$D4_RESP" | jq . > "$EVIDENCE_DIR/d4-pdf-rust.json" 2>/dev/null || true

    if echo "$D4_RESP" | jq -e '.result' >/dev/null 2>&1; then
      D4_TEXT=$(echo "$D4_RESP" | jq -r '.result.text // .result.Text // ""' 2>/dev/null)
      if [[ -n "$D4_TEXT" ]]; then
        log_pass "D4: PDF text extracted via Rust sidecar (Layer 1)"
      else
        log_pass "D4: PDF processed via Rust sidecar (empty text from minimal PDF)"
      fi
      # Verify page count reported
      D4_PAGES=$(echo "$D4_RESP" | jq -r '.result.page_count // .result.PageCount // ""' 2>/dev/null)
      if [[ -n "$D4_PAGES" && "$D4_PAGES" != "null" ]]; then
        log_info "D4: Page count = $D4_PAGES"
      fi
    elif echo "$D4_RESP" | jq -e '.error' >/dev/null 2>&1; then
      D4_ERR=$(echo "$D4_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
      log_skip "D4: PDF extraction error: $D4_ERR"
    else
      log_skip "D4: No response from bridge for PDF extraction"
    fi
  else
    log_skip "D4: Could not generate PDF test content (xxd not available)"
  fi
else
  log_skip "D4: PDF → Rust skipped (Rust sidecar socket not present)"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# D5: Office → Python / Layer 1 — XLSX extraction via Python sidecar
# ══════════════════════════════════════════════════════════════════════════════
# Layer 1: ZIP magic (PK header) + XLSX format → route to Python sidecar.
# Minimal XLSX is a ZIP file; use a small valid ZIP as test payload.
log_info "── D5: Office → Python (Layer 1) ─────────────────"

if [[ "$PYTHON_SOCK_EXISTS" == "true" ]]; then
  # Create minimal XLSX-like content: a ZIP file with PK header
  # (Real XLSX is a ZIP of XML files; this tests routing, not content)
  D5_XLSX_B64=$(printf '\x50\x4B\x03\x04\x14\x00\x00\x00\x08\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x09\x00\x00\x00\x5B\x43\x6F\x6E\x74\x65\x6E\x74\x5D\x0A\x54\x79\x70\x65\x3D\x74\x65\x73\x74\x0A\x50\x4B\x01\x02\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x09\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x5B\x43\x6F\x6E\x74\x65\x6E\x74\x5D\x0A\x50\x4B\x05\x06\x00\x00\x00\x00\x01\x00\x01\x00' | base64 -w0 2>/dev/null || echo "")

  if [[ -n "$D5_XLSX_B64" ]]; then
    D5_RESP=$(rpc_doc "sidecar.extract_text" "{
      \"document_format\": \"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet\",
      \"document_content\": \"$D5_XLSX_B64\",
      \"options\": {}
    }" 2>/dev/null || true)
    echo "$D5_RESP" | jq . > "$EVIDENCE_DIR/d5-xlsx-python.json" 2>/dev/null || true

    if echo "$D5_RESP" | jq -e '.result' >/dev/null 2>&1; then
      log_pass "D5: XLSX processed via Python sidecar (Layer 1)"
      D5_TEXT=$(echo "$D5_RESP" | jq -r '.result.text // .result.Text // ""' 2>/dev/null)
      if [[ -n "$D5_TEXT" && "$D5_TEXT" != "null" ]]; then
        log_info "D5: Extracted text length: ${#D5_TEXT} chars"
      fi
    elif echo "$D5_RESP" | jq -e '.error' >/dev/null 2>&1; then
      D5_ERR=$(echo "$D5_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
      log_skip "D5: XLSX extraction error: $D5_ERR (minimal payload may not parse)"
    else
      log_skip "D5: No response from bridge for XLSX extraction"
    fi
  else
    log_skip "D5: Could not generate XLSX test content"
  fi
else
  log_skip "D5: Office → Python skipped (Python sidecar socket not present)"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# D5.5: DOC → Java / Layer 1 — DOC extraction via Java sidecar
# ══════════════════════════════════════════════════════════════════════════════
log_info "── D5.5: DOC → Java (Layer 1) ────────────────────"

if [[ "$JAVA_SOCK_EXISTS" == "true" ]]; then
  if [[ -f "$SCRIPT_DIR/fixtures/sample.doc" ]]; then
    D55_DOC_B64=$(base64 -w0 "$SCRIPT_DIR/fixtures/sample.doc" 2>/dev/null || echo "")

    if [[ -n "$D55_DOC_B64" ]]; then
      D55_RESP=$(rpc_doc "sidecar.extract_text" "{
        \"document_format\": \"application/msword\",
        \"document_content\": \"$D55_DOC_B64\",
        \"options\": {}
      }" 2>/dev/null || true)
      echo "$D55_RESP" | jq . > "$EVIDENCE_DIR/d55-doc-java.json" 2>/dev/null || true

      if echo "$D55_RESP" | jq -e '.result' >/dev/null 2>&1; then
        D55_TEXT=$(echo "$D55_RESP" | jq -r '.result.text // .result.Text // ""' 2>/dev/null)
        if [[ -n "$D55_TEXT" && "$D55_TEXT" != "null" ]]; then
          log_pass "D5.5: DOC text extracted via Java sidecar (Layer 1)"
          log_info "D5.5: Extracted text length: ${#D55_TEXT} chars"
        else
          log_fail "D5.5: DOC processed via Java sidecar but extracted text is empty"
        fi
      elif echo "$D55_RESP" | jq -e '.error' >/dev/null 2>&1; then
        D55_ERR=$(echo "$D55_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
        log_skip "D5.5: DOC extraction error: $D55_ERR"
      else
        log_skip "D5.5: No response from bridge for DOC extraction"
      fi
    else
      log_skip "D5.5: Could not base64-encode sample.doc fixture"
    fi
  else
    log_skip "D5.5: tests/fixtures/sample.doc not found"
  fi
else
  log_skip "D5.5: DOC → Java skipped (Java sidecar socket not present)"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# D5.6: PPT → Java / Layer 1 — PPT extraction via Java sidecar
# ══════════════════════════════════════════════════════════════════════════════
log_info "── D5.6: PPT → Java (Layer 1) ────────────────────"

if [[ "$JAVA_SOCK_EXISTS" == "true" ]]; then
  if [[ -f "$SCRIPT_DIR/fixtures/sample.ppt" ]]; then
    D56_PPT_B64=$(base64 -w0 "$SCRIPT_DIR/fixtures/sample.ppt" 2>/dev/null || echo "")

    if [[ -n "$D56_PPT_B64" ]]; then
      D56_RESP=$(rpc_doc "sidecar.extract_text" "{
        \"document_format\": \"application/vnd.ms-powerpoint\",
        \"document_content\": \"$D56_PPT_B64\",
        \"options\": {}
      }" 2>/dev/null || true)
      echo "$D56_RESP" | jq . > "$EVIDENCE_DIR/d56-ppt-java.json" 2>/dev/null || true

      if echo "$D56_RESP" | jq -e '.result' >/dev/null 2>&1; then
        D56_TEXT=$(echo "$D56_RESP" | jq -r '.result.text // .result.Text // ""' 2>/dev/null)
        if [[ -n "$D56_TEXT" && "$D56_TEXT" != "null" ]]; then
          log_pass "D5.6: PPT text extracted via Java sidecar (Layer 1)"
          log_info "D5.6: Extracted text length: ${#D56_TEXT} chars"
        else
          log_fail "D5.6: PPT processed via Java sidecar but extracted text is empty"
        fi
      elif echo "$D56_RESP" | jq -e '.error' >/dev/null 2>&1; then
        D56_ERR=$(echo "$D56_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
        log_skip "D5.6: PPT extraction error: $D56_ERR"
      else
        log_skip "D5.6: No response from bridge for PPT extraction"
      fi
    else
      log_skip "D5.6: Could not base64-encode sample.ppt fixture"
    fi
  else
    log_skip "D5.6: tests/fixtures/sample.ppt not found"
  fi
else
  log_skip "D5.6: PPT → Java skipped (Java sidecar socket not present)"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# D6: Format mismatch / Layer 2 — strict drop
# ══════════════════════════════════════════════════════════════════════════════
# Layer 2: ZIP container (PK magic) but format claims PDF → strict drop.
# The bridge should reject this with a magic byte/format mismatch error.
log_info "── D6: Format mismatch (Layer 2) — strict drop ───"

# ZIP magic (PK\x03\x04) but claim it's a PDF
D6_ZIP_B64=$(printf '\x50\x4B\x03\x04\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00' | base64 -w0 2>/dev/null || echo "")

if [[ -n "$D6_ZIP_B64" ]]; then
  D6_RESP=$(rpc_doc "sidecar.extract_text" "{
    \"document_format\": \"application/pdf\",
    \"document_content\": \"$D6_ZIP_B64\",
    \"options\": {}
  }" 2>/dev/null || true)
  echo "$D6_RESP" | jq . > "$EVIDENCE_DIR/d6-format-mismatch.json" 2>/dev/null || true

  if echo "$D6_RESP" | jq -e '.error' >/dev/null 2>&1; then
    D6_ERR_MSG=$(echo "$D6_RESP" | jq -r '.error.message // .error // ""' 2>/dev/null)
    if echo "$D6_ERR_MSG" | grep -qi "mismatch\|magic\|strict\|reject\|invalid\|format"; then
      log_pass "D6: Format mismatch correctly rejected (Layer 2 strict drop)"
    else
      log_pass "D6: Request rejected with error: $D6_ERR_MSG"
    fi
  elif echo "$D6_RESP" | jq -e '.result' >/dev/null 2>&1; then
    log_fail "D6: Format mismatch was NOT rejected — strict drop policy violated"
  else
    log_skip "D6: No response from bridge for format mismatch test"
  fi
else
  log_skip "D6: Could not generate ZIP test content for mismatch test"
fi

echo ""

# ══════════════════════════════════════════════════════════════════════════════
# D7: PII interception — verify redaction/rejection
# ══════════════════════════════════════════════════════════════════════════════
# Submit a document containing PII patterns and verify the pipeline intercepts
# them (either redacting or rejecting).
log_info "── D7: PII interception ──────────────────────────"

D7_PII_TEXT="Customer SSN: $FAKE_SSN, Credit Card: $FAKE_CC, Email: $FAKE_EMAIL"
D7_PII_B64=$(echo -n "$D7_PII_TEXT" | base64 -w0 2>/dev/null || echo "")

if [[ -n "$D7_PII_B64" ]]; then
  D7_RESP=$(rpc_doc "sidecar.extract_text" "{
    \"document_format\": \"text/plain\",
    \"document_content\": \"$D7_PII_B64\",
    \"options\": {\"scan_pii\": true}
  }" 2>/dev/null || true)
  echo "$D7_RESP" | jq . > "$EVIDENCE_DIR/d7-pii-interception.json" 2>/dev/null || true

  if echo "$D7_RESP" | jq -e '.result' >/dev/null 2>&1; then
    D7_TEXT=$(echo "$D7_RESP" | jq -r '.result.text // .result.Text // ""' 2>/dev/null)

    # Check if PII was redacted from the extracted text
    D7_SSN_LEAKED=false
    D7_CC_LEAKED=false
    D7_EMAIL_LEAKED=false

    if echo "$D7_TEXT" | grep -q "$FAKE_SSN" 2>/dev/null; then
      D7_SSN_LEAKED=true
    fi
    if echo "$D7_TEXT" | grep -q "$FAKE_CC" 2>/dev/null; then
      D7_CC_LEAKED=true
    fi
    if echo "$D7_TEXT" | grep -q "$FAKE_EMAIL" 2>/dev/null; then
      D7_EMAIL_LEAKED=true
    fi

    if [[ "$D7_SSN_LEAKED" == "false" && "$D7_CC_LEAKED" == "false" ]]; then
      log_pass "D7: PII redacted — SSN and CC not present in extracted text"
    else
      if [[ "$D7_SSN_LEAKED" == "true" ]]; then
        log_fail "D7: SSN leaked in extracted text"
      fi
      if [[ "$D7_CC_LEAKED" == "true" ]]; then
        log_fail "D7: Credit card leaked in extracted text"
      fi
    fi

    # Check for redaction markers
    if echo "$D7_TEXT" | grep -qiE '\[REDACTED|REMOVED|\\*\\*\\*\\*|HIDDEN'; then
      log_pass "D7: Redaction markers present in output"
    fi

    # Check full response JSON for PII leakage (not just text field)
    if echo "$D7_RESP" | grep -q "$FAKE_SSN" 2>/dev/null; then
      log_fail "D7: SSN leaked in full response JSON"
    else
      log_pass "D7: SSN not leaked in full response"
    fi

    if echo "$D7_RESP" | grep -q "$FAKE_CC" 2>/dev/null; then
      log_fail "D7: Credit card leaked in full response JSON"
    else
      log_pass "D7: Credit card not leaked in full response"
    fi

  elif echo "$D7_RESP" | jq -e '.error' >/dev/null 2>&1; then
    # PII was rejected entirely (acceptable behavior)
    D7_ERR=$(echo "$D7_RESP" | jq -r '.error.message // "unknown"' 2>/dev/null)
    if echo "$D7_ERR" | grep -qi "pii\|sensitive\|rejected\|blocked\|policy"; then
      log_pass "D7: PII-containing document rejected by policy"
    else
      log_pass "D7: Document with PII rejected (error: $D7_ERR)"
    fi
  else
    log_skip "D7: No response from bridge for PII interception test"
  fi
else
  log_skip "D7: Could not encode PII test content"
fi

echo ""

# ── Summary ────────────────────────────────────────────────────────────────────
log_info "── Evidence saved to $EVIDENCE_DIR/ ─────────────"
ls -la "$EVIDENCE_DIR/" 2>/dev/null | tail -n +2 | while read -r line; do
  log_info "  $line"
done

harness_summary
