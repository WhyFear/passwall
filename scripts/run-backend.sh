#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="${ROOT_DIR}/passwall"
CONFIG_PATH="${CONFIG_PATH:-${BACKEND_DIR}/config.yaml}"
OUTPUT="${OUTPUT:-${BACKEND_DIR}/build/passwall-server}"

if [[ -z "${PASSWALL_TOKEN:-}" ]]; then
  echo "PASSWALL_TOKEN is required."
  echo "Example: PASSWALL_TOKEN=your_token ./scripts/run-backend.sh"
  exit 1
fi

if [[ ! -f "${CONFIG_PATH}" ]]; then
  echo "Config file not found: ${CONFIG_PATH}"
  echo "Set CONFIG_PATH=/path/to/config.yaml if you want to use another config."
  exit 1
fi

mkdir -p "$(dirname "${OUTPUT}")"

cd "${BACKEND_DIR}"
echo "Building backend..."
go build -o "${OUTPUT}" ./cmd/server

echo "Starting backend with CONFIG_PATH=${CONFIG_PATH}"
exec "${OUTPUT}"
