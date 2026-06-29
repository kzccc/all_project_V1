# 顺序号分发的相关测试

本目录用于验证 EchoChat 主链路中 `session_seq` 顺序号分发的真实耗时，并分别统计以下三种情况：

1. `cold_start`
   首次进入会话顺序号分发，走 `InitFloorAndIncr`。
2. `hot_path`
   会话顺序号已初始化完成，走普通 `IncrKey`。
3. `redis_floor_recovery`
   会话顺序号已初始化，但 Redis key 丢失后第一次请求返回 `1`，触发 floor 恢复并走 `EnsureMinAndIncr`。

目录结构：

- `configs/`
  存放测试配置。
- `scripts/`
  存放测试编排脚本、结果分析脚本、报告生成脚本。
- `reports/`
  存放测试设计说明和最终汇总报告。
- `record/`
  存放每次测试运行的原始产物和中间结果。

核心指标：

1. 主链路 `session_seq` 平均耗时。
2. `cold_start` 平均耗时。
3. `hot_path` 平均耗时。
4. `redis_floor_recovery` 平均耗时。

执行方式：

```bash
python3 "pressure workable/顺序号分发的相关测试/scripts/run_sequence_dispatch_benchmark.py" \
  --config "pressure workable/顺序号分发的相关测试/configs/sequence_dispatch_benchmark.toml"
```
