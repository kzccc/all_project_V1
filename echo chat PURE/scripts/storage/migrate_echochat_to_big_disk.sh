#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  migrate_echochat_to_big_disk.sh --target-repo <path> --runtime-root <path> [--source-repo <path>] [--dry-run] [--copy-tmp] [--copy-local-mysql] [--copy-build-cache]

Example:
  bash scripts/storage/migrate_echochat_to_big_disk.sh \
    --source-repo /workspace/czk/Personal/EchoChat \
    --target-repo /my_storage/echochat/repo/EchoChat \
    --runtime-root /my_storage/echochat/runtime

By default the script does not copy the old tmp tree or build caches.
EOF
}

SOURCE_REPO="$(pwd)"
TARGET_REPO=""
RUNTIME_ROOT=""
DRY_RUN=0
COPY_TMP=0
COPY_LOCAL_MYSQL=0
COPY_BUILD_CACHE=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source-repo)
      SOURCE_REPO="$2"
      shift 2
      ;;
    --target-repo)
      TARGET_REPO="$2"
      shift 2
      ;;
    --runtime-root)
      RUNTIME_ROOT="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --copy-tmp)
      COPY_TMP=1
      shift
      ;;
    --copy-local-mysql)
      COPY_LOCAL_MYSQL=1
      shift
      ;;
    --copy-build-cache)
      COPY_BUILD_CACHE=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown arg: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "$TARGET_REPO" || -z "$RUNTIME_ROOT" ]]; then
  usage >&2
  exit 1
fi

SOURCE_REPO="$(realpath "$SOURCE_REPO")"
TARGET_REPO="$(realpath -m "$TARGET_REPO")"
RUNTIME_ROOT="$(realpath -m "$RUNTIME_ROOT")"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REWRITE_SCRIPT="$SCRIPT_DIR/rewrite_echochat_paths.py"

run_cmd() {
  if [[ "$DRY_RUN" -eq 1 ]]; then
    echo "[dry-run] $*"
  else
    "$@"
  fi
}

echo "source_repo=$SOURCE_REPO"
echo "target_repo=$TARGET_REPO"
echo "runtime_root=$RUNTIME_ROOT"
echo "dry_run=$DRY_RUN"
echo "copy_tmp=$COPY_TMP"
echo "copy_local_mysql=$COPY_LOCAL_MYSQL"
echo "copy_build_cache=$COPY_BUILD_CACHE"

run_cmd mkdir -p "$(dirname "$TARGET_REPO")"
run_cmd mkdir -p "$RUNTIME_ROOT"/{bin,cache/go-build,cache/go-mod,logs,records/k6_message_test,records/mysql_persist_tuning,records/partition_tuning,records/t_K6,tmp,mysql,kafka}

RSYNC_ARGS=(
  -a
  --delete
  --exclude .git/index.lock
  --exclude logs
  --exclude bin
  --exclude tmp
  --exclude docs/k6_message_test/records
  --exclude docs/k6_message_test/mysql_persist_tuning_records
  --exclude docs/k6_message_test/partition_tuning_records
  --exclude docs/t_K6/records
)

run_cmd rsync "${RSYNC_ARGS[@]}" "$SOURCE_REPO"/ "$TARGET_REPO"/
run_cmd rsync -a "$SOURCE_REPO"/logs/ "$RUNTIME_ROOT"/logs/ 2>/dev/null || true
run_cmd rsync -a "$SOURCE_REPO"/bin/ "$RUNTIME_ROOT"/bin/ 2>/dev/null || true
run_cmd rsync -a "$SOURCE_REPO"/docs/k6_message_test/records/ "$RUNTIME_ROOT"/records/k6_message_test/ 2>/dev/null || true
run_cmd rsync -a "$SOURCE_REPO"/docs/k6_message_test/mysql_persist_tuning_records/ "$RUNTIME_ROOT"/records/mysql_persist_tuning/ 2>/dev/null || true
run_cmd rsync -a "$SOURCE_REPO"/docs/k6_message_test/partition_tuning_records/ "$RUNTIME_ROOT"/records/partition_tuning/ 2>/dev/null || true
run_cmd rsync -a "$SOURCE_REPO"/docs/t_K6/records/ "$RUNTIME_ROOT"/records/t_K6/ 2>/dev/null || true
if [[ "$COPY_TMP" -eq 1 ]]; then
  run_cmd rsync -a "$SOURCE_REPO"/tmp/ "$RUNTIME_ROOT"/tmp/ 2>/dev/null || true
fi
if [[ "$COPY_LOCAL_MYSQL" -eq 1 ]]; then
  run_cmd rsync -a "$SOURCE_REPO"/tmp/mysql_sys/ "$RUNTIME_ROOT"/mysql/ 2>/dev/null || true
fi
if [[ "$COPY_BUILD_CACHE" -eq 1 ]]; then
  run_cmd rsync -a "$SOURCE_REPO"/tmp/go-build-cache/ "$RUNTIME_ROOT"/cache/go-build/ 2>/dev/null || true
  run_cmd rsync -a "$SOURCE_REPO"/tmp/go-mod-cache/ "$RUNTIME_ROOT"/cache/go-mod/ 2>/dev/null || true
fi

if [[ "$DRY_RUN" -eq 0 ]]; then
  rm -rf "$TARGET_REPO"/logs "$TARGET_REPO"/bin "$TARGET_REPO"/tmp \
    "$TARGET_REPO"/docs/k6_message_test/records \
    "$TARGET_REPO"/docs/k6_message_test/mysql_persist_tuning_records \
    "$TARGET_REPO"/docs/k6_message_test/partition_tuning_records \
    "$TARGET_REPO"/docs/t_K6/records

  ln -sfn "$RUNTIME_ROOT"/logs "$TARGET_REPO"/logs
  ln -sfn "$RUNTIME_ROOT"/bin "$TARGET_REPO"/bin
  ln -sfn "$RUNTIME_ROOT"/tmp "$TARGET_REPO"/tmp
  mkdir -p "$TARGET_REPO"/docs/k6_message_test "$TARGET_REPO"/docs/t_K6
  ln -sfn "$RUNTIME_ROOT"/records/k6_message_test "$TARGET_REPO"/docs/k6_message_test/records
  ln -sfn "$RUNTIME_ROOT"/records/mysql_persist_tuning "$TARGET_REPO"/docs/k6_message_test/mysql_persist_tuning_records
  ln -sfn "$RUNTIME_ROOT"/records/partition_tuning "$TARGET_REPO"/docs/k6_message_test/partition_tuning_records
  ln -sfn "$RUNTIME_ROOT"/records/t_K6 "$TARGET_REPO"/docs/t_K6/records
fi

run_cmd python3 "$REWRITE_SCRIPT" --repo-root "$TARGET_REPO" --runtime-root "$RUNTIME_ROOT"

ENV_FILE="$TARGET_REPO/.echochat-big-disk.env"
if [[ "$DRY_RUN" -eq 0 ]]; then
  cat > "$ENV_FILE" <<EOF
export ECHOCHAT_REPO_ROOT="$TARGET_REPO"
export ECHOCHAT_RUNTIME_ROOT="$RUNTIME_ROOT"
export ECHOCHAT_LOCAL_MYSQL_ROOT="$RUNTIME_ROOT/mysql"
export ECHOCHAT_RECORD_ROOT_OVERRIDE="$RUNTIME_ROOT/records/k6_message_test"
export ECHOCHAT_PARTITION_TUNING_RECORD_ROOT="$RUNTIME_ROOT/records/partition_tuning"
export ECHOCHAT_MYSQL_PERSIST_TUNING_RECORD_ROOT="$RUNTIME_ROOT/records/mysql_persist_tuning"
export GOCACHE="$RUNTIME_ROOT/cache/go-build"
export GOMODCACHE="$RUNTIME_ROOT/cache/go-mod"
export TMPDIR="$RUNTIME_ROOT/tmp"
EOF
fi

cat <<EOF
Migration scaffold is ready.

Next:
  1. cd "$TARGET_REPO"
  2. source ./.echochat-big-disk.env
  3. export ECHOCHAT_CONFIG="$TARGET_REPO/configs/config_local_singlebroker_part240_mysqlpersist_tune2.toml"
  4. go build ./cmd/echo_chat_server

EOF
