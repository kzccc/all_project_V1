#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SCRIPT_DIR="${ROOT_DIR}/docs/t_K6/scripts"
RECORD_ROOT="${ROOT_DIR}/docs/t_K6/records"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
RUN_DIR="${RECORD_ROOT}/ws_online_${TIMESTAMP}"
SUMMARY_MD="${RUN_DIR}/summary.md"

BASE_URL="${BASE_URL:-http://127.0.0.1:8081}"
WS_URL="${WS_URL:-ws://127.0.0.1:8081}"
PASSWORD="${PASSWORD:-123456}"
TELEPHONE_START="${TELEPHONE_START:-17620000000}"
USER_PREFIX="${USER_PREFIX:-WS}"
MAX_SWEEP_VUS="${MAX_SWEEP_VUS:-1000}"
STABILITY_VUS="${STABILITY_VUS:-1000}"
STABILITY_HOLD_SECONDS="${STABILITY_HOLD_SECONDS:-600}"
SWEEP_HOLD_SECONDS="${SWEEP_HOLD_SECONDS:-60}"
RESOURCE_INTERVAL_SECONDS="${RESOURCE_INTERVAL_SECONDS:-5}"

mkdir -p "${RUN_DIR}"

count_ws_users() {
  mysql -N -uroot echochat -e "SELECT COUNT(*) FROM user_info WHERE uuid LIKE 'U${USER_PREFIX}%';"
}

ensure_ws_users() {
  local current_count
  current_count="$(count_ws_users)"
  if [[ "${current_count}" -ge "${MAX_SWEEP_VUS}" && "${current_count}" -ge "${STABILITY_VUS}" ]]; then
    return
  fi

  cd "${ROOT_DIR}"
  go run ./cmd/echo_chat_seed \
    -prefix "${USER_PREFIX}" \
    -reset-prefix \
    -user-count 3000 \
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

echochat_pid() {
  systemctl show -p MainPID --value echochat.service
}

sample_cpu_percent() {
  local pid="$1"
  pidstat -u -p "${pid}" 1 1 2>/dev/null | awk -v pid="${pid}" '
    ($1 == "Average:" && $3 == pid) || ($3 == pid && $1 ~ /^[0-9]/) {
      cpu = $8
    }
    END {
      if (cpu == "" || cpu == "UID") {
        print "0.00"
      } else {
        printf "%.2f", cpu
      }
    }
  '
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
      read -r cpu rss vsz < <(ps -p "${pid}" -o %cpu=,rss=,vsz=)
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
    k6 run "${SCRIPT_DIR}/ws_online_constant.js" \
      -u "${vus}" \
      -i "${vus}" \
      --summary-export "${summary_json}" \
      -e BASE_URL="${BASE_URL}" \
      -e WS_URL="${WS_URL}" \
      -e PASSWORD="${PASSWORD}" \
      -e TELEPHONE_START="${TELEPHONE_START}" \
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

write_summary() {
  {
    echo "# WebSocket 在线连接压测结果"
    echo
    echo "- 生成时间：$(date '+%F %T')"
    echo "- 目标服务：${BASE_URL}"
    echo "- 压测用户前缀：${USER_PREFIX}"
    echo "- 测试账号密码：${PASSWORD}"
    echo
    echo "## 账号准备"
    echo
    echo "- 可用压测账号数：$(count_ws_users)"
    echo "- 手机号起始值：${TELEPHONE_START}"
    echo
    echo "## 场景结果"
    echo
    echo "| 场景 | VU | 持续时间 | k6退出码 | 登录成功数 | 登录成功率 | 建连成功数 | 总体建连率 | 升级成功率 | Welcome消息率 | 提前断连率 | 会话时长P95(ms) | HTTP P95(ms) |"
    echo "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |"

    local case summary_json
    for case in sweep_100 sweep_300 sweep_500 sweep_800 sweep_1000 hold_1000_10m; do
      summary_json="${RUN_DIR}/${case}/summary.json"
      if [[ ! -f "${summary_json}" ]]; then
        continue
      fi

      local vus hold exit_code upgrade login welcome early_disconnect session_p95 http_p95
      case "${case}" in
        sweep_100) vus=100; hold="${SWEEP_HOLD_SECONDS}s" ;;
        sweep_300) vus=300; hold="${SWEEP_HOLD_SECONDS}s" ;;
        sweep_500) vus=500; hold="${SWEEP_HOLD_SECONDS}s" ;;
        sweep_800) vus=800; hold="${SWEEP_HOLD_SECONDS}s" ;;
        sweep_1000) vus=1000; hold="${SWEEP_HOLD_SECONDS}s" ;;
        hold_1000_10m) vus="${STABILITY_VUS}"; hold="${STABILITY_HOLD_SECONDS}s" ;;
      esac

      local login_passes upgrade_passes overall_upgrade_rate
      login_passes="$(extract_metric "${summary_json}" "login_success_rate" "passes")"
      login="$(extract_metric "${summary_json}" "login_success_rate" "value")"
      upgrade_passes="$(extract_metric "${summary_json}" "ws_upgrade_success_rate" "passes")"
      upgrade="$(extract_metric "${summary_json}" "ws_upgrade_success_rate" "value")"
      welcome="$(extract_metric "${summary_json}" "ws_welcome_message_rate" "value")"
      early_disconnect="$(extract_metric "${summary_json}" "ws_early_disconnect_rate" "value")"
      session_p95="$(extract_metric "${summary_json}" "ws_session_duration_ms" "p(95)")"
      http_p95="$(extract_metric "${summary_json}" "http_req_duration" "p(95)")"
      exit_code="$(cat "${RUN_DIR}/${case}/exit_code.txt" 2>/dev/null || echo n/a)"
      overall_upgrade_rate="$(awk -v success="${upgrade_passes}" -v total="${vus}" 'BEGIN {
        if (success == "" || success == "n/a" || total == 0) {
          print "n/a"
        } else {
          printf "%.6f", success / total
        }
      }')"
      echo "| ${case} | ${vus} | ${hold} | ${exit_code} | ${login_passes} | $(format_percent "${login}") | ${upgrade_passes} | $(format_percent "${overall_upgrade_rate}") | $(format_percent "${upgrade}") | $(format_percent "${welcome}") | $(format_percent "${early_disconnect}") | ${session_p95} | ${http_p95} |"
    done

    echo
    echo "## 结果文件"
    echo
    echo "- 运行目录：\`${RUN_DIR}\`"
    echo "- 每个场景包含："
    echo "  - `stdout.txt`"
    echo "  - `summary.json`"
    echo "  - `resource.csv`"
  } > "${SUMMARY_MD}"
}

main() {
  ensure_ws_users

  if ! run_case "sweep_100" 100 "${SWEEP_HOLD_SECONDS}"; then true; fi
  if ! run_case "sweep_300" 300 "${SWEEP_HOLD_SECONDS}"; then true; fi
  if ! run_case "sweep_500" 500 "${SWEEP_HOLD_SECONDS}"; then true; fi
  if ! run_case "sweep_800" 800 "${SWEEP_HOLD_SECONDS}"; then true; fi
  if ! run_case "sweep_1000" 1000 "${SWEEP_HOLD_SECONDS}"; then true; fi
  if ! run_case "hold_1000_10m" "${STABILITY_VUS}" "${STABILITY_HOLD_SECONDS}"; then true; fi

  write_summary
  echo "ws online suite records written to ${RUN_DIR}"
}

main "$@"
