#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="${ROOT:-$(cd "${SCRIPT_DIR}/.." && pwd)}"
WEB_ROOT="${WEB_ROOT:-/usr/share/nginx/html/rk}"

mkdir -p "${WEB_ROOT}"
cp -f "${ROOT}/output/"* "${WEB_ROOT}/"
chmod 644 "${WEB_ROOT}/"*

echo "published to ${WEB_ROOT}"
