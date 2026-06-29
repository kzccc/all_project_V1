#!/usr/bin/env bash
set -euo pipefail

IMAGE="${IMAGE:-ubuntu:22.04}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="${ROOT:-$(cd "${SCRIPT_DIR}/.." && pwd)}"

docker run --rm \
  -v "${ROOT}:${ROOT}" \
  -w "${ROOT}" \
  "${IMAGE}" \
  bash -lc 'apt-get update >/dev/null && DEBIAN_FRONTEND=noninteractive apt-get install -y build-essential gcc-aarch64-linux-gnu g++-aarch64-linux-gnu >/dev/null && make clean && make'
