#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SCRIPT_PATH="${ROOT_DIR}/docs/k6_message_test/scripts/throughput_capacity_runner.py"

MODE="${MODE:-kafka}"
LABEL="${LABEL:-}"
BASE_CONFIG="${BASE_CONFIG:-${ROOT_DIR}/configs/config_local.toml}"
DATABASE="${DATABASE:-echochat}"
SEED_PREFIX="${SEED_PREFIX:-K6}"
PORT="${PORT:-18082}"
INSTANCE_PORTS="${INSTANCE_PORTS:-}"
CLIENT_INSTANCE_PORTS="${CLIENT_INSTANCE_PORTS:-}"

SINGLE_PAIR_COUNT="${SINGLE_PAIR_COUNT:-30}"
GROUP_MEMBER_LIMIT="${GROUP_MEMBER_LIMIT:-25}"

SINGLE_INITIAL_TARGET="${SINGLE_INITIAL_TARGET:-120}"
GROUP_INITIAL_TARGET="${GROUP_INITIAL_TARGET:-180}"

SINGLE_MIN_DURATION_SEC="${SINGLE_MIN_DURATION_SEC:-8}"
GROUP_MIN_DURATION_SEC="${GROUP_MIN_DURATION_SEC:-8}"
SINGLE_MAX_MESSAGES="${SINGLE_MAX_MESSAGES:-5000}"
GROUP_MAX_MESSAGES="${GROUP_MAX_MESSAGES:-5000}"

MESSAGE_TIMEOUT_MS="${MESSAGE_TIMEOUT_MS:-60000}"
CONNECTION_SETTLE_MS="${CONNECTION_SETTLE_MS:-1500}"
DRAIN_WAIT_MS="${DRAIN_WAIT_MS:-5000}"
DRAIN_IDLE_MS="${DRAIN_IDLE_MS:-1000}"
POST_RUN_SETTLE_MS="${POST_RUN_SETTLE_MS:-1000}"

SINGLE_SUCCESS_THRESHOLD="${SINGLE_SUCCESS_THRESHOLD:-0.995}"
SINGLE_P95_THRESHOLD_MS="${SINGLE_P95_THRESHOLD_MS:-1000}"
GROUP_COVERAGE_THRESHOLD="${GROUP_COVERAGE_THRESHOLD:-0.995}"
GROUP_FULL_COVERAGE_THRESHOLD="${GROUP_FULL_COVERAGE_THRESHOLD:-0.99}"
GROUP_P95_THRESHOLD_MS="${GROUP_P95_THRESHOLD_MS:-1000}"
MAX_ERROR_COUNT="${MAX_ERROR_COUNT:-0}"

MAX_EXPAND_STEPS="${MAX_EXPAND_STEPS:-8}"
MAX_REFINE_STEPS="${MAX_REFINE_STEPS:-6}"
REFINE_RESOLUTION="${REFINE_RESOLUTION:-10}"

python3 "${SCRIPT_PATH}" \
  --mode "${MODE}" \
  --label "${LABEL}" \
  --base-config "${BASE_CONFIG}" \
  --database "${DATABASE}" \
  --seed-prefix "${SEED_PREFIX}" \
  --port "${PORT}" \
  --instance-ports "${INSTANCE_PORTS}" \
  --client-instance-ports "${CLIENT_INSTANCE_PORTS}" \
  --single-pair-count "${SINGLE_PAIR_COUNT}" \
  --group-member-limit "${GROUP_MEMBER_LIMIT}" \
  --single-initial-target "${SINGLE_INITIAL_TARGET}" \
  --group-initial-target "${GROUP_INITIAL_TARGET}" \
  --single-min-duration-sec "${SINGLE_MIN_DURATION_SEC}" \
  --group-min-duration-sec "${GROUP_MIN_DURATION_SEC}" \
  --single-max-messages "${SINGLE_MAX_MESSAGES}" \
  --group-max-messages "${GROUP_MAX_MESSAGES}" \
  --message-timeout-ms "${MESSAGE_TIMEOUT_MS}" \
  --connection-settle-ms "${CONNECTION_SETTLE_MS}" \
  --drain-wait-ms "${DRAIN_WAIT_MS}" \
  --drain-idle-ms "${DRAIN_IDLE_MS}" \
  --post-run-settle-ms "${POST_RUN_SETTLE_MS}" \
  --single-success-threshold "${SINGLE_SUCCESS_THRESHOLD}" \
  --single-p95-threshold-ms "${SINGLE_P95_THRESHOLD_MS}" \
  --group-coverage-threshold "${GROUP_COVERAGE_THRESHOLD}" \
  --group-full-coverage-threshold "${GROUP_FULL_COVERAGE_THRESHOLD}" \
  --group-p95-threshold-ms "${GROUP_P95_THRESHOLD_MS}" \
  --max-error-count "${MAX_ERROR_COUNT}" \
  --max-expand-steps "${MAX_EXPAND_STEPS}" \
  --max-refine-steps "${MAX_REFINE_STEPS}" \
  --refine-resolution "${REFINE_RESOLUTION}"
