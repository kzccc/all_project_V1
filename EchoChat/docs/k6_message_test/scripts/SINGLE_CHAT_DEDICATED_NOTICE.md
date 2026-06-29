# 单聊专用测压说明

只要提到“单聊专用测压”，统一指下面这套方案，不再混用旧入口：

- 压测脚本：`/workspace/czk/Personal/EchoChat/docs/k6_message_test/scripts/single_chat_stage_runner.py`
- 基础配置：`/workspace/czk/Personal/EchoChat/configs/config_local_singlebroker_part240_mysqlpersist_tune2.toml`

当前约定：

- 对外总入口只保留 `single_chat_stage_runner.py`
- 其他脚本可以继续保留，作为内部依赖、历史脚本或后续扩展基础
- 日常提“跑单聊专用测压”，默认就是执行这一个入口脚本
- 这次确认过的测压配置已经固化成这个入口的默认值

## 固定口径

单聊专用测压默认讨论口径固定为：

- `1 broker`
- `10 consumer`
- `tune2`
- `240 partitions`

这里的含义是：

- `1 broker / tune2 / 240 partitions` 由基础配置文件承载
- `10 consumer` 是被测服务部署前提，不是压测脚本自己启动出来的数量
- 压测脚本负责发压、采集端到端时延、抓取 metrics / pprof、汇总每个 consumer stage 的热点

## 推荐命令

```bash
python3 /workspace/czk/Personal/EchoChat/docs/k6_message_test/scripts/single_chat_stage_runner.py
```

上面这条默认就等价于：

- `mode = kafka`
- `label = single_dedicated_1broker_10consumer_tune2_part240`
- `base-config = config_local_singlebroker_part240_mysqlpersist_tune2.toml`
- `instance-ports = 18082 ~ 18091`
- `single-pair-count = 60`
- `single-min-duration-sec = 8`
- `single-max-messages = 5000`

## 解释边界

这套脚本默认：

- 只跑单聊
- 默认 `60` 对用户互发
- 使用历史同口径的 `capacity search + refine` 搜索亚秒级稳定吞吐
- 默认 `8s / 5000`，和历史容量口径一致
- 默认起本机 `10` 个服务实例：`18082 ~ 18091`
- 如果本机 `3306` 未监听，会优先尝试拉起仓库内的 `tmp/mysql_sys`
- 底层复用 `throughput_capacity_runner.py`，但固定为 `--single-only`
- 保留原来的 `summary.json`、`deliveries.csv`、`errors.json`、`metrics.prom`、`pprof/*`

其他脚本的定位：

- 可以存在
- 但不作为当前单聊专用测压入口
- 如果后面扩展新方案，再单独声明新的入口和适用范围

## 结果口径

你会同时看到两类吞吐：

- `actual_offered_rate`
  当前通过档位实际打出去的发送速率，基本可以理解成当前发消息接口等效 qps
- `observed_throughput_msg_per_sec`
  链路实际观测到的吞吐，按收到的消息数和持续时间计算

## 使用约定

后续文档、讨论、复盘里如果出现下面这些说法，默认都指这套方案：

- “单聊专用测压”
- “单聊最新专用脚本”
- “单聊容量搜索压测”
- “单聊亚秒级可用容量压测”

如果要偏离这套口径，比如不是 `1 broker / 10 consumer / tune2 / 240 partitions`，需要在文档或命令里显式写出来。
