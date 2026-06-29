#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
RECORD_DIR="${ROOT_DIR}/docs/t_K6/records"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
OUTPUT_FILE="${RECORD_DIR}/seed_testdata_${TIMESTAMP}.json"

mkdir -p "${RECORD_DIR}"

cd "${ROOT_DIR}"

go run ./cmd/echo_chat_seed \
  -prefix K6 \
  -reset-prefix \
  -user-count 200 \
  -admin-count 1 \
  -group-count 12 \
  -group-size 25 \
  -friend-span 10 \
  -pair-messages 30 \
  -group-messages 120 \
  -apply-count 40 \
  -password 123456 \
  -telephone-start 17610000000 | tee "${OUTPUT_FILE}"

echo
echo "seed summary written to ${OUTPUT_FILE}"
