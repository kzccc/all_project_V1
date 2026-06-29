# EchoChat 大盘迁移执行方案

## 目标

把 EchoChat 的源码仓库和运行时重目录一起迁到大盘，避免以后再因为根盘写满导致：

1. 压测任务中断
2. MySQL / Kafka 本地运行失败
3. Go build cache / module cache 把根盘打满
4. 记录目录、日志目录、`tmp/` 目录持续膨胀

## 迁移范围

需要一起迁的不是只有 git 仓库，还包括这些运行时目录：

1. `tmp/`
2. `logs/`
3. `bin/`
4. `docs/k6_message_test/records`
5. `docs/k6_message_test/mysql_persist_tuning_records`
6. `docs/k6_message_test/partition_tuning_records`
7. `docs/t_K6/records`
8. Go 缓存：`GOCACHE` / `GOMODCACHE`
9. 本地 MySQL 目录：`tmp/mysql_sys`

## 目标布局

推荐布局：

1. 大盘源码根：`/my_storage/.../EchoChat`
2. 大盘运行时根：`/my_storage/echochat/runtime`

运行时根下至少包含：

1. `bin/`
2. `cache/go-build`
3. `cache/go-mod`
4. `logs/`
5. `records/k6_message_test`
6. `records/mysql_persist_tuning`
7. `records/partition_tuning`
8. `records/t_K6`
9. `tmp/`
10. `mysql/`
11. `kafka/`

默认迁移策略：

1. 复制源码仓库
2. 复制日志和压测记录
3. 建立新的运行时目录和软链
4. 默认**不复制整个旧 `tmp/`**
5. 默认**不复制旧 Go build/module cache**
6. 本地 MySQL 数据目录按需通过参数再复制

## 这次已经落下的改造

这次执行里已经完成：

1. `internal/config/config.go` 不再只依赖旧绝对路径，会优先尝试：
   - `ECHOCHAT_CONFIG`
   - 当前工作目录向上回溯
   - `ECHOCHAT_REPO_ROOT`
   - 可执行文件路径向上回溯
2. 关键测压脚本不再固定写死旧仓库根路径，支持通过 `ECHOCHAT_REPO_ROOT` 自动切换：
   - `throughput_capacity_runner.py`
   - `diagnostic_stage_runner.py`
   - `partition_tuning_runner.py`
   - `single_chat_stage_runner.py`
   - `mysql_persist_param_tuner.py`
   - `run_ws_capacity_curve.py`
3. `single_chat_stage_runner.py` 和 `mysql_persist_param_tuner.py` 支持通过 `ECHOCHAT_LOCAL_MYSQL_ROOT` 把本地 MySQL 放到大盘
4. 新增迁移脚本：
   - `scripts/storage/migrate_echochat_to_big_disk.sh`
   - `scripts/storage/rewrite_echochat_paths.py`

## 正式迁移步骤

1. 选定源码目标目录和运行时目标目录
2. 执行迁移脚本
3. 在新仓库里 `source .echochat-big-disk.env`
4. 用新仓库路径启动服务、压测和本地 MySQL
5. 确认日志、记录、缓存、`tmp/` 都已经落在大盘
6. 最后再停止旧根盘上的旧任务和旧数据写入

## 推荐执行命令

```bash
bash scripts/storage/migrate_echochat_to_big_disk.sh \
  --source-repo /workspace/czk/Personal/EchoChat \
  --target-repo /my_storage/echochat/repo/EchoChat \
  --runtime-root /my_storage/echochat/runtime
```

如果要先预演：

```bash
bash scripts/storage/migrate_echochat_to_big_disk.sh \
  --source-repo /workspace/czk/Personal/EchoChat \
  --target-repo /tmp/EchoChat-big-disk-dryrun \
  --runtime-root /tmp/EchoChat-runtime-dryrun \
  --dry-run
```

如果要把旧本地 MySQL 一起迁过去，再显式加：

```bash
--copy-local-mysql
```

如果还要连旧 `tmp/` 或旧 Go cache 一起带过去，再显式加：

```bash
--copy-tmp --copy-build-cache
```

## 当前阻塞

当前 Codex 会话只能写工作区和临时目录，不能直接写 `/my_storage`，所以这次已经把“可迁移改造”和“迁移脚本”落地，但真正写入 `/my_storage` 的最后一步需要在有目标目录写权限的 shell 里执行上面的迁移命令。
