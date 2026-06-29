# EchoChat Message Latency Test

这套测试方案不修改业务代码，只通过外部脚本完成下面两类能力验证：

1. 单聊消息端到端时延
2. 群聊消息广播时延

同时支持在相同口径下分别跑 `channel` 和 `kafka` 两种消息模式，并输出对比报告。

## 方案思路

当前后端请求体里没有专门的压测 trace 字段，所以脚本把压测元信息直接写进文本消息 `content`：

- `run_id`
- `scenario`
- `bench_id`
- `send_ts_ms`

服务端会把这段 `content` 原样转发给接收端，因此脚本可以在接收端解析这段内容，计算：

- 单聊端到端时延：`receiver_recv_ts - sender_send_ts`
- 群聊单接收方时延：`member_recv_ts - sender_send_ts`
- 群聊广播完成时延：`最后一个目标成员收到时间 - sender_send_ts`

因为发送端和接收端都由同一台机器上的同一套脚本驱动，所以不需要做跨机时钟同步。

## 目录

- `scripts/make_test_configs.py`
  - 从 `configs/config_local.toml` 生成独立的 `channel` / `kafka` 压测配置
- `scripts/prepare_message_fixtures.py`
  - 从 MySQL 读取 `K6` 测试数据，生成单聊 pair 和群聊成员夹具
- `scripts/message_latency_runner.py`
  - 实际执行单聊/群聊消息时延测试
- `scripts/compare_summaries.py`
  - 对比 `channel` 和 `kafka` 的结果并生成 Markdown 报告
- `scripts/single_chat_stage_runner.py`
  - 当前单聊最新专用脚本，只跑单聊，默认 `60` 对用户，走历史同口径的 `capacity search + refine`，固定 `8s / 5000`
- `scripts/partition_tuning_runner.py`
  - 调整分区专用测压脚本，专门比较不同 partition 数下的吞吐收益和 Kafka / consumer / MySQL / WS / 资源副作用
- `scripts/SINGLE_CHAT_DEDICATED_NOTICE.md`
  - 单聊专用测压的固定口径说明，当前约定为 `1 broker / 10 consumer / tune2 / 240 partitions`
- `scripts/run_mode_compare.sh`
  - 一键串起配置生成、服务启动、两种模式测试和对比报告

## 当前单聊最新专用脚本

当前推荐直接使用：

```bash
python3 /workspace/czk/Personal/EchoChat/docs/k6_message_test/scripts/single_chat_stage_runner.py
```

默认特性：

- 只跑单聊
- `60` 对用户互发
- 使用历史一致的 `capacity search + refine`
- 默认 `8s / 5000`
- 默认走 `1 broker / 10 consumer / tune2 / 240 partitions`
- 如果本机 `3306` 未监听，会优先尝试拉起仓库内的 `tmp/mysql_sys`
- 保留原有 `summary.json`、`deliveries.csv`、`errors.json`、`metrics.prom`、`pprof/*`
- 默认 label 为 `single_dedicated_1broker_10consumer_tune2_part240`

入口约定：

- 当前对外总入口就是 `scripts/single_chat_stage_runner.py`
- 目录里其他脚本暂时保留，但不作为当前单聊专用测压入口
- 后续如果扩展新测压方案，再单独声明新的入口

固定口径说明见：

- `/workspace/czk/Personal/EchoChat/docs/k6_message_test/scripts/SINGLE_CHAT_DEDICATED_NOTICE.md`
- `/workspace/czk/Personal/EchoChat/docs/k6_message_test/调整分区专用测压脚本.md`

## 默认口径

这一节指的是通用消息时延测试方案的默认口径，不是单聊专用容量入口。

单聊：

- 数据集：`UK6...` 用户
- 并发：`30` 对 sender/receiver
- 每个 sender 发送：`20` 条文本消息
- 发送间隔：`40ms`

群聊：

- 数据集：`GK6...` 群组
- 默认群：夹具中的第一个 `GK6...` 群
- 在线成员：最多 `25`
- 发送者：群中第一个成员
- 每轮发送：`16` 条文本消息
- 发送间隔：`80ms`

## 输出

每次执行会在 `docs/k6_message_test/records/<timestamp>/` 下生成：

- `fixtures/message_fixture.json`
- `channel/single/summary.json`
- `channel/group/summary.json`
- `kafka/single/summary.json`
- `kafka/group/summary.json`
- `comparison_report.md`

每个场景还会额外输出：

- `deliveries.csv`
- `errors.json`

## 运行方式

```bash
bash /workspace/czk/Personal/EchoChat/docs/k6_message_test/scripts/run_mode_compare.sh
```

如需调整强度，可以覆盖环境变量：

```bash
PAIR_COUNT=40 \
SINGLE_MESSAGES=30 \
GROUP_MESSAGES=20 \
GROUP_MEMBER_LIMIT=25 \
CONNECTION_SETTLE_MS=1000 \
bash /workspace/czk/Personal/EchoChat/docs/k6_message_test/scripts/run_mode_compare.sh
```

## 注意事项

1. 这套方案测的是“真实业务消息链路”，不是裸 WebSocket RTT。
2. 当前结果会包含消息落库、Redis 缓存更新、群成员查询和回推开销。
3. `kafka` 轮次天然多了一跳 `produce + consume`，所以重点看：
   - `avg / p95 / p99`
   - 发送成功率
   - 群聊覆盖率
   - 观测到的吞吐
4. 两轮测试都会写入 `message` 表，这是预期行为。
