#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WEB_DIR="${ROOT_DIR}/web"
HOST="${HOST:-0.0.0.0}"
PORT="${PORT:-3000}"
BROWSER="${BROWSER:-none}"

cd "${WEB_DIR}"

if [[ ! -d node_modules ]]; then
  echo "node_modules not found, running npm install..."
  npm install
fi

echo "Building web..."
npm run build

echo "Starting web dev server at http://${HOST}:${PORT}"
export HOST PORT BROWSER
exec npm start
