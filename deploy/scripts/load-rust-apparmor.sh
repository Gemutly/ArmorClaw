#!/usr/bin/env bash
set -euo pipefail

PROFILE_NAME="armorclaw-rust-worker"
PROFILE_SRC="$(dirname "$0")/../apparmor/${PROFILE_NAME}"
PROFILE_DEST="/etc/apparmor.d/${PROFILE_NAME}"

if [ ! -f "${PROFILE_SRC}" ]; then
    echo "ERROR: Profile source not found at ${PROFILE_SRC}" >&2
    exit 1
fi

echo "Installing ${PROFILE_NAME} AppArmor profile..."
cp "${PROFILE_SRC}" "${PROFILE_DEST}"

echo "Parsing and loading profile..."
apparmor_parser -r "${PROFILE_DEST}"

echo "Verifying profile loaded..."
if aa-status 2>/dev/null | grep -q "${PROFILE_NAME}"; then
    echo "OK: ${PROFILE_NAME} loaded in enforce mode"
else
    echo "ERROR: Profile not found in aa-status output" >&2
    exit 1
fi
