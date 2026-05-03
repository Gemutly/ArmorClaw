#!/usr/bin/env bash
set -euo pipefail

CONDUIT_URL="${CONDUIT_URL:-http://localhost:6167}"
USERNAME="${E2EE_TEST_USER:-e2ee-test}"
PASSWORD="${E2EE_TEST_PASSWORD:-e2ee-test-password}"
SHARED_SECRET="${CONDUIT_SHARED_SECRET:-}"

echo "Creating test user '$USERNAME' on $CONDUIT_URL ..."

if [ -n "$SHARED_SECRET" ]; then
  BODY=$(cat <<EOF
{
  "username": "$USERNAME",
  "password": "$PASSWORD",
  "auth": {
    "type": "m.login.registration_token",
    "token": "$SHARED_SECRET"
  }
}
EOF
)
else
  BODY=$(cat <<EOF
{
  "username": "$USERNAME",
  "password": "$PASSWORD",
  "auth": {
    "type": "m.login.dummy"
  }
}
EOF
)
fi

HTTP_CODE=$(curl -s -o /tmp/conduit-register-response.json -w "%{http_code}" \
  -X POST "$CONDUIT_URL/_matrix/client/v3/register" \
  -H "Content-Type: application/json" \
  -d "$BODY")

if [ "$HTTP_CODE" = "200" ]; then
  echo "User '$USERNAME' registered successfully."
  cat /tmp/conduit-register-response.json | python3 -m json.tool 2>/dev/null || cat /tmp/conduit-register-response.json
elif [ "$HTTP_CODE" = "400" ]; then
  ERR=$(cat /tmp/conduit-register-response.json)
  if echo "$ERR" | grep -q "M_USER_IN_USE"; then
    echo "User '$USERNAME' already exists — skipping registration."
  else
    echo "Registration failed (400): $ERR"
    exit 1
  fi
else
  echo "Registration failed (HTTP $HTTP_CODE):"
  cat /tmp/conduit-register-response.json
  exit 1
fi
