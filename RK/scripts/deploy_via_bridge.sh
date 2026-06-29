#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="${ROOT:-$(cd "${SCRIPT_DIR}/.." && pwd)}"
BOARD_HOST="${BOARD_HOST:-192.168.1.21}"
BOARD_USER="${BOARD_USER:-root}"
REMOTE_ROOT="${REMOTE_ROOT:-/userdata/vision}"

ssh "${BOARD_USER}@${BOARD_HOST}" "mkdir -p '${REMOTE_ROOT}/bin' '${REMOTE_ROOT}/res' '${REMOTE_ROOT}/model'"
scp "${ROOT}/output/"* "${BOARD_USER}@${BOARD_HOST}:${REMOTE_ROOT}/bin/"
ssh "${BOARD_USER}@${BOARD_HOST}" "chmod +x '${REMOTE_ROOT}/bin/'*"

echo "deployed to ${BOARD_USER}@${BOARD_HOST}:${REMOTE_ROOT}/bin"
