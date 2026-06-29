#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUNTIME_DIR="${ROOT_DIR}/deploy/kafka5/runtime"
BROKER_PORTS=(29092 29093 29094 29095 29096)

echo "Kafka local 5-node status:"
for port in "${BROKER_PORTS[@]}"; do
  if lsof -Pi :"${port}" -sTCP:LISTEN -t >/dev/null 2>&1; then
    pid="$(lsof -Pi :"${port}" -sTCP:LISTEN -t | head -n 1)"
    echo "  port ${port}: up (pid ${pid})"
  else
    echo "  port ${port}: down"
  fi
done

if [[ -x "${RUNTIME_DIR}/dist/kafka_2.13-3.9.2/bin/kafka-broker-api-versions.sh" ]]; then
  echo
  "${RUNTIME_DIR}/dist/kafka_2.13-3.9.2/bin/kafka-broker-api-versions.sh" --bootstrap-server 127.0.0.1:29092 >/dev/null 2>&1 \
    && echo "  bootstrap 127.0.0.1:29092: reachable" \
    || echo "  bootstrap 127.0.0.1:29092: not ready"
fi
