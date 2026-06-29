# EchoChat 最大吞吐一键压测

这套脚本用于做“版本间可重复对比”的极限吞吐测试。

目标不是跑一档固定压力，而是自动升压，找到当前版本在给定口径下：

1. 单聊最大稳定吞吐
2. 群聊最大稳定吞吐

适合你在每次 Kafka 优化后直接复跑同一套流程，对比吞吐有没有提升。

## 1. 入口脚本

```bash
bash /workspace/czk/Personal/EchoChat/docs/k6_message_test/scripts/run_max_throughput.sh
```

默认跑 `kafka` 模式。

## 2. 它会做什么

脚本会自动完成下面这些事：

1. 生成当前模式的独立测试配置
2. 重建一批固定前缀的压测数据
3. 准备单聊 / 群聊夹具
4. 从低压力开始逐档升压
5. 某一档失败后继续做细化搜索
6. 选出“最大稳定吞吐”对应的最佳档位
7. 输出每一档结果和最终汇总报告

## 3. 判定标准

默认判定为“通过”的条件：

### 单聊

1. `delivery_success_rate >= 0.995`
2. `p95_latency_ms <= 1000`
3. `errors.json` 中错误数为 `0`

### 群聊

1. `delivery_coverage_rate >= 0.995`
2. `full_coverage_message_rate >= 0.99`
3. `receipt p95 <= 1000ms`
4. `errors.json` 中错误数为 `0`

你可以通过环境变量调整这些阈值。

## 4. 输出目录

每次运行会生成：

```text
docs/k6_message_test/records/throughput_capacity_<mode>_<timestamp>[_label]/
```

目录中重点看：

1. `summary.json`
2. `report.md`
3. `single_stage_summary.csv`
4. `group_stage_summary.csv`

每个 stage 目录下还会保存：

1. `summary.json`
2. `deliveries.csv`
3. `errors.json`
4. `metrics.prom`
5. `server.log`

## 5. 最关键的两个吞吐指标

### 单聊

看：

- `observed_throughput_msg_per_sec`

### 群聊

看：

- `observed_delivery_per_sec`

注意：群聊这里统计的是“投递副本吞吐”，不是“群消息条数吞吐”。

## 6. 默认运行命令

```bash
bash /workspace/czk/Personal/EchoChat/docs/k6_message_test/scripts/run_max_throughput.sh
```

## 7. 带版本标签运行

建议你每次优化后都加一个标签，方便结果目录和报告一眼看懂：

```bash
LABEL=v1_kafka_ack \
bash /workspace/czk/Personal/EchoChat/docs/k6_message_test/scripts/run_max_throughput.sh
```

## 8. 常用调参

### 提高单聊并发对数

```bash
SINGLE_PAIR_COUNT=50 \
bash /workspace/czk/Personal/EchoChat/docs/k6_message_test/scripts/run_max_throughput.sh
```

### 提高群成员上限

```bash
GROUP_MEMBER_LIMIT=30 \
bash /workspace/czk/Personal/EchoChat/docs/k6_message_test/scripts/run_max_throughput.sh
```

### 把每档测试持续时间拉长

```bash
SINGLE_MIN_DURATION_SEC=12 \
GROUP_MIN_DURATION_SEC=12 \
bash /workspace/czk/Personal/EchoChat/docs/k6_message_test/scripts/run_max_throughput.sh
```

### 调严格一点的通过标准

```bash
SINGLE_SUCCESS_THRESHOLD=1.0 \
GROUP_COVERAGE_THRESHOLD=1.0 \
GROUP_FULL_COVERAGE_THRESHOLD=1.0 \
bash /workspace/czk/Personal/EchoChat/docs/k6_message_test/scripts/run_max_throughput.sh
```

### 调整 p95 通过阈值

```bash
SINGLE_P95_THRESHOLD_MS=800 \
GROUP_P95_THRESHOLD_MS=800 \
bash /workspace/czk/Personal/EchoChat/docs/k6_message_test/scripts/run_max_throughput.sh
```

## 9. 建议固定口径

为了让每次优化前后的结果可以横向对比，建议你固定下面这些参数，不要每次都改：

1. `SINGLE_PAIR_COUNT`
2. `GROUP_MEMBER_LIMIT`
3. `SINGLE_MIN_DURATION_SEC`
4. `GROUP_MIN_DURATION_SEC`
5. 成功率阈值
6. `p95` 阈值
7. `MODE`

这样你后面看到吞吐变化时，才更容易判断提升来自代码，而不是来自测试口径变化。
