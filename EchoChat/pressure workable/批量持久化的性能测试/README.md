# 批量持久化的性能测试

本目录用于验证 EchoChat Kafka 主链路里 `mysql_persist` 的批量入库设计，相比“单条逐条写库”到底带来了多少收益。

目录结构：

- `configs/`
  存放对照实验配置。
- `scripts/`
  存放实验编排脚本和报告生成脚本。
- `reports/`
  存放本次实验设计说明和最终汇总报告。
- `record/`
  存放每次实验运行的原始产物和中间结果。

本方案的核心原则：

1. 不做脱离业务链路的微基准。
2. 直接复用当前单聊压测 runner，保持 Kafka -> consumer -> `mysql_persist` -> websocket 整条链路一致。
3. 只切换 `mysql_persist` 的批量策略，保证对照组和实验组的差异足够干净。

本次对照口径：

1. `batched`
   使用当前批量持久化配置。
2. `single_insert`
   把 `mysqlPersistBatchSize` 降为 `1`，并把 `firstJobHold`、`flushInterval` 一并降到最小，近似模拟“每条消息单独写库”。

主要输出指标：

1. 客户端吞吐：`observed_throughput_msg_per_sec`
2. 端到端延迟：`latency.avg/p50/p95/p99/max`
3. 服务端关键链路延迟：`server_critical_path_ms`
4. `mysql_persist` 阶段耗时：`mysql_persist_ms`
5. 持久化聚合效果：`flush_count`、`avg_flush_batch_size`、`flush_reason_counts`
6. 积压与背压：`avg_enqueue_queue_depth`
7. 稳定性：`delivery_success_rate`、`drain_recovered_messages`

执行方式：

```bash
python3 "pressure workable/批量持久化的性能测试/scripts/run_batch_persist_benchmark.py" \
  --config "pressure workable/批量持久化的性能测试/configs/batch_persist_benchmark.toml"
```
