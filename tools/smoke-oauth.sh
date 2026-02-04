#!/usr/bin/env bash
set -euo pipefail

API_URL=${API_URL:-http://localhost:3000}
BIN=${BIN:-/tmp/sure-cli}
DEVICE_TYPE=${DEVICE_TYPE:-android} # some Sure builds reject "web"

echo "[smoke] building sure-cli -> $BIN"
cd "$(dirname "$0")/.."
go build -o "$BIN" .

EMAIL=${EMAIL:-"clitest+$(date +%s)@example.com"}
PASS=${PASS:-"Aa1!aaaa"}

echo "[smoke] api_url=$API_URL"
echo "[smoke] email=$EMAIL"

# Create user via Sure signup (no real data)
# If signup is disabled/invite-only, this may fail; you can instead export EMAIL/PASS for an existing user.
set +e
RESP=$(curl -sS -X POST "$API_URL/api/v1/auth/signup" \
  -H 'Content-Type: application/json' \
  -d "{\"user\":{\"email\":\"$EMAIL\",\"password\":\"$PASS\",\"first_name\":\"CLI\",\"last_name\":\"Test\"},\"device\":{\"device_id\":\"sure-cli-test\",\"device_name\":\"sure-cli-test\",\"device_type\":\"$DEVICE_TYPE\",\"os_version\":\"macOS\",\"app_version\":\"sure-cli\"}}")
RC=$?
set -e
if [[ $RC -ne 0 ]]; then
  echo "[smoke] signup request failed (curl rc=$RC)" >&2
  exit 1
fi

echo "[smoke] signup response (trunc): ${RESP:0:120}"

# Configure CLI
"$BIN" config set api_url "$API_URL" >/dev/null

# Login
"$BIN" login --email "$EMAIL" --password "$PASS" --format=json | head -c 300; echo

# whoami should show oauth
"$BIN" whoami --format=json | head -c 240; echo

# refresh should work (and persist)
"$BIN" refresh --format=json | head -c 200; echo

echo "[smoke] OK"
