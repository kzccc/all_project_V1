#!/usr/bin/env bash

set -euo pipefail

ulimit -n 500000 || true

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SCRIPT_DIR="${ROOT_DIR}/docs/t_K6/scripts"
RECORD_ROOT="${ROOT_DIR}/docs/t_K6/records"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
RUN_DIR="${RECORD_ROOT}/ws_online_bench_${TIMESTAMP}"
SUMMARY_MD="${RUN_DIR}/summary.md"
TOKEN_FILE="${RUN_DIR}/ws_tokens.json"
TOKEN_FILE_FOR_K6="../records/ws_online_bench_${TIMESTAMP}/ws_tokens.json"

WS_URL="${WS_URL:-ws://127.0.0.1:8081}"
PASSWORD="${PASSWORD:-123456}"
TELEPHONE_START="${TELEPHONE_START:-17620000000}"
USER_PREFIX="${USER_PREFIX:-WS}"
USER_COUNT="${USER_COUNT:-12000}"
MAX_SWEEP_VUS="${MAX_SWEEP_VUS:-10000}"
STABILITY_VUS="${STABILITY_VUS:-10000}"
STABILITY_HOLD_SECONDS="${STABILITY_HOLD_SECONDS:-300}"
SWEEP_HOLD_SECONDS="${SWEEP_HOLD_SECONDS:-60}"
RESOURCE_INTERVAL_SECONDS="${RESOURCE_INTERVAL_SECONDS:-5}"
RUN_SWEEPS="${RUN_SWEEPS:-1}"
RUN_HOLD="${RUN_HOLD:-1}"

mkdir -p "${RUN_DIR}"

count_ws_users() {
  mysql -N -uroot echochat -e "SELECT COUNT(*) FROM user_info WHERE uuid LIKE 'U${USER_PREFIX}%';"
}

ensure_ws_users() {
  local current_count
  current_count="$(count_ws_users)"
  if [[ "${current_count}" -ge "${USER_COUNT}" ]]; then
    return
  fi

  cd "${ROOT_DIR}"
  go run ./cmd/echo_chat_seed \
    -prefix "${USER_PREFIX}" \
    -reset-prefix \
    -user-count "${USER_COUNT}" \
    -admin-count 1 \
    -group-count 0 \
    -group-size 1 \
    -friend-span 1 \
    -pair-messages 0 \
    -group-messages 0 \
    -apply-count 0 \
    -password "${PASSWORD}" \
    -telephone-start "${TELEPHONE_START}" > "${RUN_DIR}/seed_ws_users.json"
}

generate_ws_tokens() {
  cd "${ROOT_DIR}"
  go run ./cmd/echo_chat_ws_tokens \
    -prefix "${USER_PREFIX}" \
    -count "${MAX_SWEEP_VUS}" \
    -output "${TOKEN_FILE}"
}

echochat_pid() {
  systemctl show -p MainPID --value echochat.service
}

sample_cpu_percent() {
  local pid="$1"
  ps -p "${pid}" -o %cpu= 2>/dev/null | awk '
    {
      cpu = $1
    }
    END {
      if (cpu == "" || cpu == "CPU") {
        print "0.00"
      } else {
        printf "%.2f", cpu + 0
      }
    }'
}

sample_thread_count() {
  local pid="$1"
  awk '/^Threads:/ {print $2}' "/proc/${pid}/status" 2>/dev/null || echo 0
}

sample_fd_count() {
  local pid="$1"
  find "/proc/${pid}/fd" -mindepth 1 -maxdepth 1 2>/dev/null | wc -l
}

start_resource_sampler() {
  local output_file="$1"
  local pid="$2"

  (
    echo "timestamp,pid,cpu_percent_inst,rss_kb,vsz_kb,threads,fd_count,established_8081"
    while kill -0 "${K6_PID}" 2>/dev/null; do
      local now cpu rss vsz threads fd_count estab
      now="$(date '+%F %T')"
      read -r _ rss vsz < <(ps -p "${pid}" -o %cpu=,rss=,vsz=)
      cpu="$(sample_cpu_percent "${pid}")"
      threads="$(sample_thread_count "${pid}")"
      fd_count="$(sample_fd_count "${pid}")"
      estab="$(ss -tan state established "( sport = :8081 )" | tail -n +2 | wc -l)"
      echo "${now},${pid},${cpu:-0},${rss:-0},${vsz:-0},${threads:-0},${fd_count:-0},${estab}"
      sleep "${RESOURCE_INTERVAL_SECONDS}"
    done
  ) > "${output_file}" &
  SAMPLER_PID=$!
}

capture_observability() {
  local case_dir="$1"
  curl -s http://127.0.0.1:8081/metrics > "${case_dir}/metrics.prom" || true
  curl -s http://127.0.0.1:8081/debug/pprof/goroutine?debug=1 > "${case_dir}/goroutine.txt" || true
}

run_case() {
  local case_name="$1"
  local vus="$2"
  local hold_seconds="$3"
  local case_dir="${RUN_DIR}/${case_name}"

  mkdir -p "${case_dir}"

  local summary_json="${case_dir}/summary.json"
  local stdout_file="${case_dir}/stdout.txt"
  local resource_csv="${case_dir}/resource.csv"

  (
    cd "${ROOT_DIR}"
    k6 run "${SCRIPT_DIR}/ws_online_tokens.js" \
      --address "127.0.0.1:0" \
      -u "${vus}" \
      -i "${vus}" \
      --summary-export "${summary_json}" \
      -e WS_URL="${WS_URL}" \
      -e TOKEN_FILE="${TOKEN_FILE_FOR_K6}" \
      -e HOLD_SECONDS="${hold_seconds}"
  ) > "${stdout_file}" 2>&1 &
  K6_PID=$!

  local service_pid
  service_pid="$(echochat_pid)"
  start_resource_sampler "${resource_csv}" "${service_pid}"

  wait "${K6_PID}"
  local k6_status=$?
  wait "${SAMPLER_PID}" || true
  echo "${k6_status}" > "${case_dir}/exit_code.txt"
  capture_observability "${case_dir}"
  return "${k6_status}"
}

extract_metric() {
  local summary_json="$1"
  local metric_name="$2"
  local field_name="$3"
  jq -r --arg metric_name "${metric_name}" --arg field_name "${field_name}" '
    .metrics[$metric_name] as $metric
    | if ($metric | type) != "object" then
        "n/a"
      elif $metric[$field_name] != null then
        $metric[$field_name]
      elif (($metric.values // null) | type) == "object" and $metric.values[$field_name] != null then
        $metric.values[$field_name]
      elif $field_name == "value" and $metric.value != null then
        $metric.value
      else
        "n/a"
      end
  ' "${summary_json}"
}

format_percent() {
  local value="${1:-n/a}"
  awk -v v="${value}" 'BEGIN {
    if (v == "" || v == "n/a") {
      print "n/a"
    } else {
      printf "%.2f%%", v * 100
    }
  }'
}

extract_resource_stat() {
  local resource_csv="$1"
  local field="$2"
  awk -F, -v field="${field}" '
    NR == 1 {
      for (i = 1; i <= NF; i++) {
        if ($i == field) {
          idx = i
        }
      }
      next
    }
    idx > 0 && $idx + 0 > max {
      max = $idx + 0
    }
    END {
      if (idx == 0) {
        print "n/a"
      } else {
        print max + 0
      }
    }
  ' "${resource_csv}"
}

write_summary() {
  {
    echo "# WebSocket 纯在线压测结果"
    echo
    echo "- 生成时间：$(date '+%F %T')"
    echo "- WebSocket 目标：${WS_URL}${WS_URL:+/bench/wss}"
    echo "- 压测用户前缀：${USER_PREFIX}"
    echo "- token 文件：${TOKEN_FILE}"
    echo "- 压测口径：离线 access token 直连 benchmark 握手链路，不经过 /login，不以 welcome 消息为成功条件"
    echo
    echo "## 账号准备"
    echo
    echo "- 可用压测账号数：$(count_ws_users)"
    echo "- token 数量：$(jq 'length' "${TOKEN_FILE}")"
    echo
    echo "## 场景结果"
    echo
    echo "| 场景 | VU | 持续时间 | k6退出码 | 建连成功数 | 建连成功率 | 提前断连率 | 会话时长P95(ms) | 瞬时CPU峰值(%) | RSS峰值(MB) | 线程峰值 | FD峰值 | TCP连接采样峰值 |"
    echo "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |"

    local case summary_json resource_csv
    for case in sweep_2000 sweep_5000 sweep_8000 sweep_10000 hold_10000_stability; do
      summary_json="${RUN_DIR}/${case}/summary.json"
      resource_csv="${RUN_DIR}/${case}/resource.csv"
      if [[ ! -f "${summary_json}" ]]; then
        continue
      fi

      local vus hold exit_code upgrade_passes upgrade session_p95 early_disconnect cpu_peak rss_peak threads_peak fd_peak estab_peak
      case "${case}" in
        sweep_2000) vus=2000; hold="${SWEEP_HOLD_SECONDS}s" ;;
        sweep_5000) vus=5000; hold="${SWEEP_HOLD_SECONDS}s" ;;
        sweep_8000) vus=8000; hold="${SWEEP_HOLD_SECONDS}s" ;;
        sweep_10000) vus=10000; hold="${SWEEP_HOLD_SECONDS}s" ;;
        hold_10000_stability) vus="${STABILITY_VUS}"; hold="${STABILITY_HOLD_SECONDS}s" ;;
      esac

      upgrade_passes="$(extract_metric "${summary_json}" "ws_upgrade_success_rate" "passes")"
      upgrade="$(extract_metric "${summary_json}" "ws_upgrade_success_rate" "value")"
      early_disconnect="$(extract_metric "${summary_json}" "ws_early_disconnect_rate" "value")"
      session_p95="$(extract_metric "${summary_json}" "ws_session_duration_ms" "p(95)")"
      exit_code="$(cat "${RUN_DIR}/${case}/exit_code.txt" 2>/dev/null || echo n/a)"

      cpu_peak="$(extract_resource_stat "${resource_csv}" "cpu_percent_inst")"
      rss_peak="$(extract_resource_stat "${resource_csv}" "rss_kb")"
      threads_peak="$(extract_resource_stat "${resource_csv}" "threads")"
      fd_peak="$(extract_resource_stat "${resource_csv}" "fd_count")"
      estab_peak="$(extract_resource_stat "${resource_csv}" "established_8081")"
      rss_peak="$(awk -v kb="${rss_peak}" 'BEGIN {printf "%.2f", kb / 1024}')"

      echo "| ${case} | ${vus} | ${hold} | ${exit_code} | ${upgrade_passes} | $(format_percent "${upgrade}") | $(format_percent "${early_disconnect}") | ${session_p95} | ${cpu_peak} | ${rss_peak} | ${threads_peak} | ${fd_peak} | ${estab_peak} |"
    done

    echo
    echo "## 结果文件"
    echo
    echo "- 运行目录：\`${RUN_DIR}\`"
    echo "- 每个场景包含：\`stdout.txt\`、\`summary.json\`、\`resource.csv\`、\`metrics.prom\`、\`goroutine.txt\`"
  } > "${SUMMARY_MD}"
}

main() {
  ensure_ws_users
  generate_ws_tokens

  if [[ "${RUN_SWEEPS}" == "1" ]]; then
    if ! run_case "sweep_2000" 2000 "${SWEEP_HOLD_SECONDS}"; then true; fi
    if ! run_case "sweep_5000" 5000 "${SWEEP_HOLD_SECONDS}"; then true; fi
    if ! run_case "sweep_8000" 8000 "${SWEEP_HOLD_SECONDS}"; then true; fi
    if ! run_case "sweep_10000" 10000 "${SWEEP_HOLD_SECONDS}"; then true; fi
  fi
  if [[ "${RUN_HOLD}" == "1" ]]; then
    if ! run_case "hold_10000_stability" "${STABILITY_VUS}" "${STABILITY_HOLD_SECONDS}"; then true; fi
  fi

  write_summary
  echo "ws online bench suite records written to ${RUN_DIR}"
}

main "$@"
