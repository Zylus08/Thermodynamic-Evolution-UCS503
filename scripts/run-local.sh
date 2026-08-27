#!/usr/bin/env bash
# Run API and frontend concurrently (Unix/macOS)
# For local development only. Uses ADMIN_PASSKEY=localdevtoken.
set -e
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

# Require ADMIN_PASSKEY to be set (no default)
if [ -z "$ADMIN_PASSKEY" ]; then
  echo "ERROR: ADMIN_PASSKEY is not set. Set it before running, e.g."
  echo "  export ADMIN_PASSKEY=your_local_passkey_here"
  exit 1
fi

# Start API in background
echo "Starting API on port 8080..."
ADMIN_PASSKEY="$ADMIN_PASSKEY" PORT=8080 bash -c "cd \"$ROOT_DIR/api\" && go run main.go" &
API_PID=$!

echo "Starting frontend (Vite) on port 5173..."
cd "$ROOT_DIR/admin-portal"
npm ci
npm run dev -- --host 0.0.0.0 --port 5173 &
FE_PID=$!

echo "API PID: $API_PID, Frontend PID: $FE_PID"
wait $API_PID $FE_PID
