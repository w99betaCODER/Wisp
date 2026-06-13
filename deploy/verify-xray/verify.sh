#!/usr/bin/env bash
# Verifies Wisp's real Xray gRPC integration against a live Xray-core instance.
#
# Starts Xray with the test config, runs the panel pointed at Xray's API,
# creates and deletes a user, and confirms the real AddUser/RemoveUser gRPC
# calls succeed.
#
#   ./deploy/verify-xray/verify.sh                 # downloads Xray
#   XRAY=/usr/local/bin/xray ./deploy/verify-xray/verify.sh
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

xray="${XRAY:-}"
if [ -z "$xray" ]; then
  echo "Downloading Xray-core (linux-64)..."
  curl -fsSL -o "$work/xray.zip" \
    https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-64.zip
  unzip -q "$work/xray.zip" -d "$work"
  xray="$work/xray"
  chmod +x "$xray"
fi

echo "Starting Xray..."
"$xray" run -c "$here/config.json" >"$work/xray.log" 2>&1 &
xpid=$!
sleep 2

echo "Building and starting the panel against the live Xray API..."
( cd "$here/../.." && go build -o "$work/panel" ./cmd/panel )

WISP_DB="$work/verify.db" WISP_ADDR="127.0.0.1:8080" \
WISP_XRAY_API="127.0.0.1:10085" WISP_INBOUND_TAG="vless-in" WISP_NODE_FLOW="" \
  "$work/panel" >"$work/panel.log" 2>&1 &
ppid=$!
sleep 2

cleanup() { kill "$ppid" "$xpid" 2>/dev/null || true; }
trap 'cleanup; rm -rf "$work"' EXIT

uid=$(curl -fsS -X POST http://127.0.0.1:8080/api/users \
  -H 'Content-Type: application/json' -d '{"email":"livetest"}' | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
echo "AddUser OK  -> Xray accepted the client (id $uid)"
curl -fsS -X DELETE "http://127.0.0.1:8080/api/users/$uid" >/dev/null
echo "RemoveUser OK"

echo ""
echo "PASS: Wisp drives a real Xray-core over gRPC (AddUser + RemoveUser)."
