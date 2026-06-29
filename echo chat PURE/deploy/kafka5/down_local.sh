#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUNTIME_DIR="${ROOT_DIR}/deploy/kafka5/runtime"
PIDS_FILE="${RUNTIME_DIR}/pids"
BROKER_PORTS=(29092 29093 29094 29095 29096)
CONTROLLER_PORTS=(39092 39093 39094 39095 39096)

if [[ -f "${PIDS_FILE}" ]]; then
  tac "${PIDS_FILE}" | while read -r pid; do
    if [[ -n "${pid}" ]] && kill -0 "${pid}" >/dev/null 2>&1; then
      kill "${pid}" >/dev/null 2>&1 || true
    fi
  done
  rm -f "${PIDS_FILE}"
fi

for port in "${BROKER_PORTS[@]}"; do
  if lsof -Pi :"${port}" -sTCP:LISTEN -t >/dev/null 2>&1; then
    lsof -Pi :"${port}" -sTCP:LISTEN -t | xargs -r kill >/dev/null 2>&1 || true
  fi
done

for port in "${CONTROLLER_PORTS[@]}"; do
  if lsof -Pi :"${port}" -sTCP:LISTEN -t >/dev/null 2>&1; then
    lsof -Pi :"${port}" -sTCP:LISTEN -t | xargs -r kill >/dev/null 2>&1 || true
  fi
done
