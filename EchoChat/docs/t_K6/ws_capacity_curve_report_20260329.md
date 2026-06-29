# EchoChat WebSocket 容量曲线报告

## 1. 结论

本轮已经完成一套“阶梯加压 + 全程采样 + 自动出图”的 WebSocket 在线容量曲线压测。

这轮最重要的结论不是“服务端在 1 万连接失败”，而是：

- EchoChat 服务端已经被验证在 `2000 / 4000 / 6000 / 8000` 连接、每档 `180s` 持有的场景下稳定运行。
- 在这四档健康区内，`握手成功率 100%`、`提前断连率 0%`、`错误率 0%`。
- 当前这套同机压测模型在推进到 `10000` 连接前，压测端 `k6` 先被系统 `OOM Killer` 杀掉，服务端并没有进入退化区或失败区。
- 因此，这轮压测的真实结论应当是：
  - `服务端健康区已验证到 8k`
  - `服务端上限尚未测出`
  - `当前瓶颈先出现在压测端，而不是 EchoChat 服务端`

## 2. 压测口径

本轮沿用了已经改造好的 benchmark 链路，口径如下：

- 使用离线生成好的 `access token`
- WebSocket 直连 `/bench/wss`
- 不走 `/login`
- 不把 welcome message 作为成功标准
- 每个阶段独立运行，阶段之间归零后再升档

阶段配置：

- 起始连接数：`2000`
- 每档增量：`2000`
- 目标上限：`16000`
- 每档持有时长：`180s`
- 采样间隔：`2s`

本轮记录目录：

- [ws_capacity_curve_20260329_202310](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310)

## 3. 区间判定标准

这套方案里我用三段式口径来判定：

- `Healthy Zone`
  握手成功率高，在线峰值能跟上目标，提前断连率低
- `Degradation Zone`
  能连上，但开始出现在线留存下降、握手时延明显抖动、提前断连上升
- `Failure Zone`
  无法完成阶段目标，或者压测执行链路本身先失败

要注意：

这轮的 `Failure Zone` 是“压测端失败”，不是“服务端失败”。

## 4. 阶段结果

原始阶段汇总见：

- [stage_summary.csv](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/stage_summary.csv)

核心结果如下：

| 阶段 | 目标连接数 | 结果区间 | 握手成功率 | 提前断连率 | connect p95 | connect p99 | 在线峰值 | 最后正在线数 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `step_02000` | `2000` | `Healthy` | `100%` | `0%` | `167.94ms` | `201.57ms` | `2000` | `2000` |
| `step_04000` | `4000` | `Healthy` | `100%` | `0%` | `132.80ms` | `177.49ms` | `4000` | `4000` |
| `step_06000` | `6000` | `Healthy` | `100%` | `0%` | `185.64ms` | `214.35ms` | `6000` | `6000` |
| `step_08000` | `8000` | `Healthy` | `100%` | `0%` | `176.73ms` | `198.42ms` | `8000` | `8000` |
| `step_10000` | `10000` | `Loadgen Failure` | `未进入建连` | `未进入建连` | `未产生` | `未产生` | `0` | `0` |

可以看到：

- `2k -> 8k` 全部都是健康区
- `10k` 这一步不是“服务端开始退化”
- 而是压测工具在 VU 初始化阶段就被系统直接杀掉

## 5. 为什么说 1W 失败不是服务端失败

证据链是完整的。

### 5.1 k6 进程被 OOM Killer 杀掉

`dmesg` 明确记录：

- `task=k6`
- `oom-kill`
- `Killed process 1753394 (k6)`

并且它的内存占用达到了非常夸张的量级：

- `anon-rss: 233856148kB`

也就是大约 `223 GiB` 级别。

这说明当前这套 `k6 + 每连接一个 VU` 的压测方式，在 `1W` 阶段先把压测机自己顶爆了。

### 5.2 服务端没有进入 1W 建连

在 `step_10000` 的采样期间：

- `online_connections` 一直是 `0`
- `process_open_fds` 一直是 `10`
- `go_goroutines` 一直在 `14~15`

也就是说：

- 服务端没有开始承受 `1W` 在线连接
- 所以不能把这轮 `1W` 失败解释成“服务端扛不住 1W”

### 5.3 服务端进程始终存活

压测结束后服务状态仍然是：

- `echochat = active`

并且最终指标正常回落到空闲态：

- `echochat_ws_online_connections{route="bench"} = 0`
- `process_open_fds = 10`
- `go_goroutines = 14`

所以这轮失败域很清楚：

- 失败点在 `load generator`
- 不在 `EchoChat server`

## 6. 资源曲线怎么解读

### 6.1 在线连接曲线

图：

- [01_target_vs_actual.png](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/plots/01_target_vs_actual.png)

解读：

- `2k -> 8k` 阶段，实际在线数都能完整追上目标值
- 每档都能完整挂住到阶段结束
- 没有出现“目标继续升，实际在线开始跟不上”的退化拐点

### 6.2 内存曲线

图：

- [02_memory.png](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/plots/02_memory.png)

服务端健康区峰值内存：

- `2k`: `126.99 MB`
- `4k`: `220.24 MB`
- `6k`: `278.25 MB`
- `8k`: `372.45 MB`

解读：

- 整体上仍然是随连接数增长而上升
- 中间有明显的 GC 回落锯齿，这是正常现象
- 但没有出现“失控飙升、无法回收”的内存泄漏特征

### 6.3 goroutine / fd 曲线

图：

- [03_goroutines_fds.png](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/plots/03_goroutines_fds.png)

健康区峰值：

- `2k`: `goroutines≈6015`, `open_fds≈2010`
- `4k`: `goroutines≈12015`, `open_fds≈4010`
- `6k`: `goroutines≈18015`, `open_fds≈6010`
- `8k`: `goroutines≈24015`, `open_fds≈8010`

这个结果很有价值：

- `open_fds` 近似等于 `在线连接数 + 10`
- `goroutines` 近似等于 `在线连接数 * 3 + 常数`

这基本把当前连接模型的结构特征量化出来了：

- fd 成本接近线性
- goroutine 成本也近似线性，但系数偏高

这对后面冲更高连接数是一个很强的信号：

- fd 不是现在的瓶颈
- goroutine 模型会更早成为上限压力

### 6.4 错误率图

图：

- [04_error_rates.png](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/plots/04_error_rates.png)

解读：

- 健康区内 `early_disconnect_rate = 0`
- `ws_error_rate = 0`
- 没有出现服务端侧退化抬头

### 6.5 握手时延图

图：

- [05_connect_latency.png](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/plots/05_connect_latency.png)

健康区内握手时延大致范围：

- `p95`: `132ms ~ 186ms`
- `p99`: `177ms ~ 214ms`

解读：

- 目前 `2k -> 8k` 区间里，握手时延有波动，但还没有出现明显失控拐点
- 这仍然属于“服务能接、时延可控”的健康区

### 6.6 容量曲线

图：

- [06_capacity_curves.png](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/plots/06_capacity_curves.png)

解读：

- 随在线峰值上升，`resident_memory / goroutines / open_fds` 都呈近似线性增长
- 当前图里还没有出现明显的“弯折点”
- 这反过来说明：在服务端真正进入容量极限之前，压测端先到头了

### 6.7 关闭原因分布

图：

- [07_close_reasons.png](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/plots/07_close_reasons.png)

结果：

- 健康区各阶段关闭原因都表现为 `read:close_1001`

这说明：

- 已完成阶段的连接结束基本都是压测端按预期主动关闭
- 不是异常断链主导

## 7. 当前能对外怎么讲

如果你现在要拿去面试或写简历，建议讲成这样：

> 为即时通讯后端设计并实现了 WebSocket 容量曲线压测方案，完成阶梯加压、Prometheus 指标采集、自动绘图与容量报告；在纯在线 benchmark 链路中，单机已验证 `8k` WebSocket 连接健康承载，`180s` 持有场景握手成功率 `100%`、提前断连率 `0%`，并定位到当前 `1w` 压测失败先发生在压测端 `k6` 的 OOM，而非服务端瓶颈。

这个表述比“压到了 1 万”更专业，因为它体现了：

- 你会设计压测方法
- 你会区分服务端瓶颈和压测端瓶颈
- 你能做容量曲线分析，而不是只看一个 QPS 数字

## 8. 这轮方案还需要怎么升级

这轮方案已经够用来做一版比较像样的容量分析，但如果目标是继续往 `1W+ / 2W+ / 5W` 推，下一步建议是：

### 8.1 先换压测模型

当前 `k6` 的问题很明显：

- `1W` 阶段在初始化 VU 时就吃掉了 `223GiB` 级别匿名内存
- 说明“每连接一个 VU”的模型不适合继续往高连接数推进

下一步更适合：

- 用更轻量的 WebSocket 压测器
- 或者自己写一个 Go 版长连接压测器
- 或者多压测机分布式打

### 8.2 再测服务端真正的退化区

现在还没有真正测到：

- 服务端在哪一档开始掉线
- 哪一档开始握手时延明显抬升
- 哪一档开始出现在线数追不上目标

也就是说：

- `Degradation Zone` 这轮还没有落到服务端身上
- `Failure Zone` 也还没有落到服务端身上

### 8.3 benchmark 连接模型还可以继续减重

从当前图上看，服务端 goroutine 增长大致是 `3 * online_connections`。

这说明 benchmark 路径虽然已经避开了数据库和业务消息链路，但连接模型仍然偏重。

如果后面要追更高承载，优先级很高的优化是：

- 做一个更极简的 benchmark-only 连接模型
- 进一步减少每连接 goroutine 数和 channel 分配

## 9. 产物清单

### 主要报告

- [ws_capacity_curve_report_20260329.md](/workspace/czk/Personal/EchoChat/docs/t_K6/ws_capacity_curve_report_20260329.md)

### 运行记录

- [ws_capacity_curve_20260329_202310](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310)

### 图表

- [dashboard.png](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/plots/dashboard.png)
- [01_target_vs_actual.png](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/plots/01_target_vs_actual.png)
- [02_memory.png](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/plots/02_memory.png)
- [03_goroutines_fds.png](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/plots/03_goroutines_fds.png)
- [04_error_rates.png](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/plots/04_error_rates.png)
- [05_connect_latency.png](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/plots/05_connect_latency.png)
- [06_capacity_curves.png](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/plots/06_capacity_curves.png)
- [07_close_reasons.png](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/plots/07_close_reasons.png)

### 原始数据

- [all_samples.csv](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/all_samples.csv)
- [stage_summary.csv](/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_capacity_curve_20260329_202310/stage_summary.csv)

## 10. 一句话收尾

这轮压测已经把 EchoChat 的服务端健康承载区间明确地画到了 `8k`，并且进一步证明了下一步真正该优化的不是服务端 fd 或 JWT 鉴权，而是：

- `压测端模型`
- `benchmark 连接模型`
- `更高连接规模下的真实服务端退化区定位`
