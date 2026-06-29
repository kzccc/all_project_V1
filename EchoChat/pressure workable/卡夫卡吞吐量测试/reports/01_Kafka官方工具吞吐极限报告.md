# Kafka 官方工具吞吐极限报告

口径
- 运行标签：`kafka_tools_throughput_benchmark`
- 生成时间：`2026-06-16T18:03:02`
- 测试拓扑：`single broker / single controller / KRaft`
- Broker：`127.0.0.1:39092`，分区默认数 `producer=24` / `consumer=24` / `e2e=12`
- 测试范围：`producer-perf-test`、`consumer-perf-test`、`kafka-e2e-latency`

吞吐量
- producer 极限吞吐：`264317.181 records/s`，`258.120 MB/s`
- consumer 极限吞吐：`144.605 MB/s`，`148075.025 records/s`
- e2e 请求消息数：`2000` 条，采样点 `sample_points=2`

全链路各模块平均耗时
- ingress_to_produce_ack_ms：本轮未采到
- kafka_queue_wait_ms：本轮未采到
- deserialize_ms：本轮未采到
- session_seq_ms：本轮未采到
- mysql_persist_ms：本轮未采到
- dispatch_after_persist_ms：本轮未采到
- receiver_queue_wait_ms：本轮未采到
- receiver_ws_write_ms：本轮未采到
- server_critical_path_ms：本轮未采到
- end_to_end_ms：avg `0.708` / p50 `0.000` / p95 `本轮未采到` / p99 `1.000` / max `29.652`

mysql_persist 细分
- enqueue_block：本轮未采到
- worker_queue_wait：本轮未采到
- batch_collect_wait：本轮未采到
- sql_exec：本轮未采到
- flush：本轮未采到

每秒读写
- producer 总耗时 `1.135` 秒，对应平均写入 `264317.181` records/s
- consumer 总耗时 `2.026` 秒，对应平均读取 `148075.025` records/s
- 第 1 秒：read 本轮未采到，write 本轮未采到

分区热度
- active partitions：`24`
- hottest partition：本轮未采到
- hottest share：本轮未采到
- heat-shape：官方 perf 工具不输出分区级明细，本轮未采到

consumer 分配
- consumer 实例数：`1`
- 总分配分区：`24`
- 实际活跃分区：`24`
- perf-consumer：assigned `24`，active `24`，consumed `300000`

persist batch
- flush reason：本轮未采到
- batch 分布：本轮未采到
- per reason average batch size：本轮未采到
- flush duration：本轮未采到

总结
- 本轮 Kafka 官方工具三项基准里，producer 上限约为 `264317.181 records/s`，consumer 上限约为 `148075.025 records/s`。
- 端到端单条往返平均 `0.708 ms`，p95 `本轮未采到`，当前瓶颈口径更偏向 broker 自身读写与刷盘能力。
- 这份结果是 Kafka 工具自测基线，不包含 EchoChat 业务链路，所以适合拿来和后续业务压测结果做上限差对比。
