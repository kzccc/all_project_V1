#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
TEST_DIR="${ROOT_DIR}/docs/k6_message_test"
SCRIPT_DIR="${TEST_DIR}/scripts"
RECORD_ROOT="${TEST_DIR}/records"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
RUN_DIR="${RECORD_ROOT}/${TIMESTAMP}"
CONFIG_DIR="${RUN_DIR}/configs"
FIXTURE_DIR="${RUN_DIR}/fixtures"
LOG_DIR="${RUN_DIR}/logs"

PAIR_COUNT="${PAIR_COUNT:-30}"
GROUP_MEMBER_LIMIT="${GROUP_MEMBER_LIMIT:-25}"
SINGLE_MESSAGES="${SINGLE_MESSAGES:-20}"
GROUP_MESSAGES="${GROUP_MESSAGES:-16}"
SINGLE_INTERVAL_MS="${SINGLE_INTERVAL_MS:-40}"
GROUP_INTERVAL_MS="${GROUP_INTERVAL_MS:-80}"
MESSAGE_TIMEOUT_MS="${MESSAGE_TIMEOUT_MS:-15000}"
CONNECTION_SETTLE_MS="${CONNECTION_SETTLE_MS:-1000}"
CHANNEL_PORT="${CHANNEL_PORT:-18081}"
KAFKA_PORT="${KAFKA_PORT:-18082}"

mkdir -p "${RUN_DIR}" "${CONFIG_DIR}" "${FIXTURE_DIR}" "${LOG_DIR}"

python3 "${SCRIPT_DIR}/make_test_configs.py" \
  --base-config "${ROOT_DIR}/configs/config_local.toml" \
  --output-dir "${CONFIG_DIR}" \
  --channel-port "${CHANNEL_PORT}" \
  --kafka-port "${KAFKA_PORT}" \
  --channel-log "${LOG_DIR}/channel_server.log" \
  --kafka-log "${LOG_DIR}/kafka_server.log" >/dev/null

python3 "${SCRIPT_DIR}/prepare_message_fixtures.py" \
  --database echochat \
  --pair-count "${PAIR_COUNT}" \
  --group-member-limit "${GROUP_MEMBER_LIMIT}" \
  --output "${FIXTURE_DIR}/message_fixture.json" >/dev/null

cleanup_redis() {
  local pattern
  for pattern in "message_list_UK6*" "group_messagelist_GK6*" "session_list_UK6*" "group_session_list_UK6*" "session_UK6*"; do
    redis-cli --scan --pattern "${pattern}" | xargs -r redis-cli del >/dev/null
  done
}

start_server() {
  local config_path="$1"
  local pid_file="$2"
  local boot_log="$3"
  ECHOCHAT_CONFIG="${config_path}" "${ROOT_DIR}/bin/echo_chat_server" >"${boot_log}" 2>&1 &
  echo $! >"${pid_file}"
}

wait_for_server() {
  local base_url="$1"
  local pid="$2"
  for _ in $(seq 1 60); do
    if ! kill -0 "${pid}" 2>/dev/null; then
      echo "test server exited early: ${pid}" >&2
      return 1
    fi
    if curl -fsS "${base_url}/metrics" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "timeout waiting for ${base_url}" >&2
  return 1
}

stop_server() {
  local pid_file="$1"
  if [[ ! -f "${pid_file}" ]]; then
    return 0
  fi
  local pid
  pid="$(cat "${pid_file}")"
  if kill -0 "${pid}" 2>/dev/null; then
    kill -TERM "${pid}"
    for _ in $(seq 1 30); do
      if ! kill -0 "${pid}" 2>/dev/null; then
        break
      fi
      sleep 1
    done
    if kill -0 "${pid}" 2>/dev/null; then
      kill -KILL "${pid}"
    fi
  fi
}

run_mode() {
  local mode="$1"
  local port="$2"
  local config_path="$3"
  local pid_file="${RUN_DIR}/${mode}.pid"
  local boot_log="${LOG_DIR}/${mode}_boot.log"
  local base_url="http://127.0.0.1:${port}"
  local ws_url="ws://127.0.0.1:${port}"

  cleanup_redis
  start_server "${config_path}" "${pid_file}" "${boot_log}"
  local pid
  pid="$(cat "${pid_file}")"
  wait_for_server "${base_url}" "${pid}"

  python3 "${SCRIPT_DIR}/message_latency_runner.py" \
    --base-url "${base_url}" \
    --ws-base-url "${ws_url}" \
    --fixture "${FIXTURE_DIR}/message_fixture.json" \
    --scenario single \
    --output-dir "${RUN_DIR}/${mode}/single" \
    --messages-per-sender "${SINGLE_MESSAGES}" \
    --send-interval-ms "${SINGLE_INTERVAL_MS}" \
    --message-timeout-ms "${MESSAGE_TIMEOUT_MS}" \
    --connection-settle-ms "${CONNECTION_SETTLE_MS}" \
    --pair-count "${PAIR_COUNT}" \
    --mode-label "${mode}" \
    --server-pid "${pid}" >/dev/null

  python3 "${SCRIPT_DIR}/message_latency_runner.py" \
    --base-url "${base_url}" \
    --ws-base-url "${ws_url}" \
    --fixture "${FIXTURE_DIR}/message_fixture.json" \
    --scenario group \
    --output-dir "${RUN_DIR}/${mode}/group" \
    --messages-per-sender "${GROUP_MESSAGES}" \
    --send-interval-ms "${GROUP_INTERVAL_MS}" \
    --message-timeout-ms "${MESSAGE_TIMEOUT_MS}" \
    --connection-settle-ms "${CONNECTION_SETTLE_MS}" \
    --group-member-limit "${GROUP_MEMBER_LIMIT}" \
    --mode-label "${mode}" \
    --server-pid "${pid}" >/dev/null

  stop_server "${pid_file}"
}

trap 'stop_server "${RUN_DIR}/channel.pid"; stop_server "${RUN_DIR}/kafka.pid"' EXIT

run_mode "channel" "${CHANNEL_PORT}" "${CONFIG_DIR}/channel_test.toml"
run_mode "kafka" "${KAFKA_PORT}" "${CONFIG_DIR}/kafka_test.toml"

python3 "${SCRIPT_DIR}/compare_summaries.py" \
  --channel-single "${RUN_DIR}/channel/single/summary.json" \
  --kafka-single "${RUN_DIR}/kafka/single/summary.json" \
  --channel-group "${RUN_DIR}/channel/group/summary.json" \
  --kafka-group "${RUN_DIR}/kafka/group/summary.json" \
  --output "${RUN_DIR}/comparison_report.md" >/dev/null

echo "${RUN_DIR}"
