# EchoChat 可观测性接入说明

本文档只解决第一步：把 `pprof`、`Prometheus`、`Grafana` 先接到当前项目上，并形成一套本地可跑的最小闭环。

## 1. 先明确三者分工

- `pprof`：Go 进程诊断工具，适合在压测时抓 `cpu`、`heap`、`goroutine`、`block`、`mutex`
- `Prometheus`：持续抓取 `/metrics`，保存时序指标
- `Grafana`：读取 Prometheus 数据并做看板

这三者不是串成一条链。

- `Prometheus -> Grafana` 是常驻链路
- `pprof` 是压测时按需抓样本的诊断链路

对你这个项目来说，最合理的搭配是：

1. 平时一直开 `Prometheus + Grafana`
2. 压测阶段额外打开 `pprof`
3. 看到某个阶段耗时或 lag 异常，再抓对应时刻的 `pprof`

## 2. 你项目当前已经具备的观测基础

服务端已经有以下能力：

- `/metrics`
- `/debug/pprof/*`
- `/readyz`
- `/bench/admin/trace`
- `/bench/admin/metrics_snapshot`

代码位置：

- 指标定义：[internal/observability/metrics.go](/workspace/czk/Personal/EchoChat/internal/observability/metrics.go)
- 暴露路由：[internal/https_server/observability_routes.go](/workspace/czk/Personal/EchoChat/internal/https_server/observability_routes.go)
- HTTP 服务入口：[cmd/echo_chat_server/main.go](/workspace/czk/Personal/EchoChat/cmd/echo_chat_server/main.go)
- 配置开关：[internal/config/config.go](/workspace/czk/Personal/EchoChat/internal/config/config.go)

结论很直接：

- `pprof` 不需要再写代码安装，打开配置即可
- `Prometheus` client 也已经在代码里，不需要再埋基础 SDK
- 你现在缺的是外部部署层和一套初始看板

## 3. 你的压测体系现在是什么结构

仓库里已经有一套偏完整的单聊压测体系：

- 主入口：[pressure testing/scripts/run_single_chat_pressure.py](/workspace/czk/Personal/EchoChat/pressure%20testing/scripts/run_single_chat_pressure.py)
- 关键路径汇总：[pressure testing/scripts/single_chat_critical_path_runner.py](/workspace/czk/Personal/EchoChat/pressure%20testing/scripts/single_chat_critical_path_runner.py)
- 配置样例：[pressure testing/configs/single_chat_pressure.toml](/workspace/czk/Personal/EchoChat/pressure%20testing/configs/single_chat_pressure.toml)

它不是只测“发了多少条”，而是在同时采：

- 单聊目标挡位、持续时间、会话数
- Kafka topic 分区数、consumer 实例数
- `mysql_persist` 的 batch/flush 行为
- benchmark trace 事件
- 关键路径阶段耗时
- 每轮报告与图表产物

这说明你的压测框架已经有“结果产出”能力，但还缺一个“运行中实时看板”。`Grafana` 正好补这块。

## 4. 已经补进仓库的第一步骨架

我新增了一套本地 observability 部署文件：

- Compose：[deploy/observability/docker-compose.yml](/workspace/czk/Personal/EchoChat/deploy/observability/docker-compose.yml)
- Prometheus 配置：[deploy/observability/prometheus/prometheus.yml](/workspace/czk/Personal/EchoChat/deploy/observability/prometheus/prometheus.yml)
- Grafana 数据源：[deploy/observability/grafana/provisioning/datasources/prometheus.yml](/workspace/czk/Personal/EchoChat/deploy/observability/grafana/provisioning/datasources/prometheus.yml)
- Grafana dashboard provider：[deploy/observability/grafana/provisioning/dashboards/dashboards.yml](/workspace/czk/Personal/EchoChat/deploy/observability/grafana/provisioning/dashboards/dashboards.yml)
- 初始看板：[deploy/observability/grafana/dashboards/echochat-overview.json](/workspace/czk/Personal/EchoChat/deploy/observability/grafana/dashboards/echochat-overview.json)

默认行为：

- Prometheus 抓本机 `8000` 端口的 `/metrics`
- Grafana 自动连 Prometheus
- Grafana 首次启动就会加载 `EchoChat Overview`

## 5. 第一阶段你该怎么装

### 第一步：打开 EchoChat 的 metrics 和 pprof

你的配置文件里确认或修改：

```toml
[observabilityConfig]
enablePprof = true
enableMetrics = true
```

如果你是本地默认配置启动，通常改这里：

- [configs/config.toml](/workspace/czk/Personal/EchoChat/configs/config.toml)

如果你是压测脚本生成临时后端配置，那么要确认脚本生成出的那份后端配置也带上这两个开关。

### 第二步：启动 EchoChat 服务

示例：

```bash
go run ./cmd/echo_chat_server
```

启动后先验证：

```bash
curl http://127.0.0.1:8000/readyz
curl http://127.0.0.1:8000/metrics | head
curl http://127.0.0.1:8000/debug/pprof/
```

### 第三步：启动 Prometheus 和 Grafana

在仓库根目录执行：

```bash
docker compose -f deploy/observability/docker-compose.yml up -d
```

访问：

- Prometheus: `http://127.0.0.1:9090`
- Grafana: `http://127.0.0.1:3000`

Grafana 默认账号密码：

- `admin`
- `admin`

### 第四步：确认 Grafana 已经看到 EchoChat 指标

先在 Prometheus 里直接搜：

- `echochat_ws_online_connections`
- `echochat_kafka_producer_messages_total`
- `echochat_kafka_consumer_handled_total`
- `echochat_mysql_open_connections`
- `echochat_mysql_persist_flush_duration_seconds`

如果这些指标能查到，Grafana 看板就会出图。

## 6. 为什么这是最适合你的第一步

因为你项目现阶段的瓶颈排查，本质上是三层证据配合：

1. `压测脚本产物`
2. `Prometheus 时序指标`
3. `pprof 剖析样本`

只看压测报告，你能知道“慢了”

只看 `pprof`，你常常只能知道“哪儿热”

只有把 `Prometheus` 实时指标补上，你才能回答：

- 是 Kafka consumer lag 先涨，还是 mysql_persist 先抖
- 是 websocket write 先变慢，还是 status update 先堆积
- 是某几个 partition 热，还是整体都饱和

## 7. 你这个项目下一步最值得补的面板

当前初始 dashboard 只是让系统先通。下一轮建议重点补这几类 panel：

- Kafka producer QPS / fail rate
- Kafka consumer handled rate / failure stage
- consumer lag by partition
- `kafka_consumer_stage_duration_seconds` 的 p50/p95
- `mysql_persist_flush_duration_seconds` 的 p95
- `mysql_persist_flush_batch_size`
- `echochat_mysql_wait_count_total`
- `echochat_ws_write_duration_seconds`
- `echochat_ws_status_update_duration_seconds`

## 8. pprof 在你这里的正确用法

建议不要长期高频抓 `pprof`，而是压测时按挡位抓。

常用命令：

```bash
go tool pprof http://127.0.0.1:8000/debug/pprof/profile?seconds=15
go tool pprof http://127.0.0.1:8000/debug/pprof/heap
curl http://127.0.0.1:8000/debug/pprof/goroutine?debug=1
curl http://127.0.0.1:8000/debug/pprof/block?debug=1
curl http://127.0.0.1:8000/debug/pprof/mutex?debug=1
```

压测观察顺序建议固定成：

1. 先看 Grafana 哪条曲线先坏
2. 再在那一段时间抓 `pprof`
3. 最后对照你的 benchmark trace 和离线报告确认根因

## 9. 当前这一套的限制

这次只做了“先装起来”的最小闭环，还没做：

- Kafka / MySQL / Redis exporter
- 更细的业务 dashboard
- 压测脚本自动抓取 Prometheus 区间数据
- `pprof` 采样自动归档到每轮压测结果目录

这些是第二步，不该和第一步混在一起做。
