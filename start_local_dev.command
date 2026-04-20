#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$SCRIPT_DIR"
START_SCRIPT="${REPO_ROOT}/tools/local_dev.sh"

if [ ! -x "$START_SCRIPT" ]; then
  chmod +x "$START_SCRIPT"
fi

echo "Starting Sub2API local dev services..."
echo ""
"$START_SCRIPT" start
echo ""
echo "Frontend: http://127.0.0.1:3000/"
echo "Backend health: http://127.0.0.1:8080/health"
echo ""
echo "Press Enter to close this window."
read -r
