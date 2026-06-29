#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_FILE="${LOG_FILE:-$ROOT_DIR/logs/echochat.log}"

if [[ ! -f "$LOG_FILE" ]]; then
  echo "日志文件不存在: $LOG_FILE"
  exit 1
fi

usage() {
  cat <<'EOF'
用法:
  bash scripts/logs.sh tail [行数]
  bash scripts/logs.sh actor <actor_id>
  bash scripts/logs.sh req <request_id>
  bash scripts/logs.sh error
  bash scripts/logs.sh grep <关键词>

示例:
  bash scripts/logs.sh tail 200
  bash scripts/logs.sh actor 17603055719
  bash scripts/logs.sh req login-test-001
EOF
}

cmd="${1:-}"
case "$cmd" in
  tail)
    lines="${2:-200}"
    tail -n "$lines" -f "$LOG_FILE" | jq -R 'fromjson? // {raw:.}' -C
    ;;
  actor)
    actor_id="${2:-}"
    if [[ -z "$actor_id" ]]; then
      usage
      exit 1
    fi
    jq -c --arg actor "$actor_id" 'select(.actor_id==$actor)' "$LOG_FILE" | jq -C '.'
    ;;
  req)
    request_id="${2:-}"
    if [[ -z "$request_id" ]]; then
      usage
      exit 1
    fi
    jq -c --arg rid "$request_id" 'select(.request_id==$rid)' "$LOG_FILE" | jq -C '.'
    ;;
  error)
    rg --color=always -n '"level":"error"|"level":"fatal"|panic' "$LOG_FILE"
    ;;
  grep)
    keyword="${2:-}"
    if [[ -z "$keyword" ]]; then
      usage
      exit 1
    fi
    rg --color=always -n "$keyword" "$LOG_FILE"
    ;;
  *)
    usage
    exit 1
    ;;
esac
