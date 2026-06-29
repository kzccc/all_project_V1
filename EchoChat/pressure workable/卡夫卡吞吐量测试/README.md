# 卡夫卡吞吐量测试

本目录用于补齐 Kafka 官方自带性能工具的独立基准测试。

目录结构：

- `configs/`
  存放测试参数配置。
- `scripts/`
  存放测试执行脚本和报告生成脚本。
- `reports/`
  存放测试设计和最终报告。
- `record/`
  存放每次运行的原始输出、Kafka 临时运行数据和汇总 JSON。

本方案只测 Kafka 工具自身可直接验证的三项能力：

1. `producer-perf-test`
   生产吞吐极限。
2. `consumer-perf-test`
   消费吞吐极限。
3. `kafka-e2e-latency`
   端到端单条往返时延。

当前实现默认在本机启动一个单节点 KRaft Kafka，避免依赖外部容器镜像或业务链路。

执行方式：

```bash
python3 "pressure workable/卡夫卡吞吐量测试/scripts/run_kafka_throughput_benchmark.py" \
  --config "pressure workable/卡夫卡吞吐量测试/configs/kafka_throughput_benchmark.toml"
```
