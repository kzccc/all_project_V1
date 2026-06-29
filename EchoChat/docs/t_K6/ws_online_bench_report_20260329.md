# EchoChat WebSocket 在线能力压测报告

## 1. 本次结论

本轮已经按 P0 要求把“在线连接能力”从“登录能力”中拆开，改成了离线生成 `access token` 后直接连接 `/bench/wss` 的纯 WebSocket 在线压测口径。

在当前单机同压测机环境下，EchoChat 已验证得到下面这组结果：

- `2000 / 5000 / 8000 / 10000` 并发连接的 `60s` 场景中，握手成功率均为 `100%`，提前断连率均为 `0%`。
- `10000` 并发连接的 `300s` 稳定性场景中，握手成功率为 `100%`，但有 `842` 条连接在 `181.1s ~ 300.0s` 区间内提前结束，最终完整持有到测试结束的连接数为 `9158`，保活率约 `91.58%`。
- 从服务端资源表现看，这一轮不像是 MySQL、JWT 鉴权或 fd 上限导致的失败；更像是“同机长时间持有 1W WebSocket”下，客户端压测工具侧或当前连接模型的长稳保活问题。

如果你现在要写进简历，建议先保守写：

> 基于 WebSocket + JWT 构建即时通讯后端，在纯在线握手链路压测下单机支持 `1W` 长连接接入，`60s` 场景握手成功率 `100%`、提前断连率 `0%`；已完成 Prometheus / pprof / fd / goroutine 可观测性补齐，并完成高并发连接场景的系统级调优。

`300s` 长稳结果还不够漂亮，不建议现在写成“1W 长连接稳定在线 5 分钟”。

## 2. 压测目标与口径

本次测试只验证“在线连接能力”，不混入登录、欢迎消息、消息转发等额外路径。

测试口径：

- 先离线批量生成 `access token`
- k6 直接使用预生成 token 连接 `ws://127.0.0.1:8081/bench/wss`
- 成功标准以 `TCP + WS upgrade 成功` 为准
- 不把 `/login` 算进在线连接能力
- 不把 welcome message 作为成功条件

新增的 benchmark 路径与能力包括：

- 压测专用鉴权中间件：仅解析 `access token`，不查 MySQL
- 压测专用 WebSocket 路由：`/bench/wss`
- Prometheus 指标：在线连接数、握手成功/失败、鉴权拒绝、Go runtime、fd
- pprof：`/debug/pprof/*`
- benchmark 热路径日志降噪

## 3. 测试环境

- 机器：`32 vCPU`，`247 GiB RAM`
- CPU：`Intel(R) Xeon(R) Gold 6462C`
- 内核：`5.10.134-19.103.al8.x86_64`
- 压测方式：同机压测
- 服务端端口：`8081`

环境风险：

- 当前系统盘剩余空间只有 `3.7G`，使用率 `97%`
- 这不会直接影响本轮在线连接结果，但会影响后续日志、压测记录和数据库扩容安全边界

## 4. 已完成的系统调优

### 4.1 systemd / fd

- `echochat.service`: `LimitNOFILE=500000`
- `mysqld.service`: `LimitNOFILE=500000`
- MySQL `open_files_limit` 已同步拉高

### 4.2 sysctl

当前已生效：

- `fs.file-max = 2097152`
- `net.core.somaxconn = 65535`
- `net.ipv4.ip_local_port_range = 10000 65535`
- `net.ipv4.tcp_max_syn_backlog = 65535`
- `net.netfilter.nf_conntrack_max = 1048576`
- `net.ipv4.tcp_fin_timeout = 15`
- `net.ipv4.tcp_tw_reuse = 1`

### 4.3 MySQL

当前已生效：

- `max_connections = 2000`
- `innodb_buffer_pool_size = 4G`
- `thread_cache_size = 256`
- `table_open_cache = 8192`
- `back_log = 8192`

说明：

本轮 benchmark 路由已绕过 MySQL 查询用户，所以 MySQL 不再是 WebSocket 在线能力测试的主瓶颈。

## 5. 测试结果

### 5.1 结果总表

| 场景 | 持有时长 | 握手成功数 | 握手成功率 | 提前断连率 | 会话时长 P95 | RSS 峰值 | FD 峰值 | TCP 连接峰值 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `sweep_2000` | `60s` | `2000` | `100%` | `0%` | `60001 ms` | `114.04 MB` | `2008` | `2000` |
| `sweep_5000` | `60s` | `5000` | `100%` | `0%` | `60013 ms` | `245.99 MB` | `5008` | `5000` |
| `sweep_8000` | `60s` | `8000` | `100%` | `0%` | `60006 ms` | `401.61 MB` | `8008` | `8000` |
| `sweep_10000` | `60s` | `10000` | `100%` | `0%` | `60006 ms` | `521.85 MB` | `10008` | `10000` |
| `hold_10000_stability` | `300s` | `10000` | `100%` | `8.42%` | `300010 ms` | `459.81 MB` | `10008` | `10000` |

### 5.2 1W 长稳场景细节

`hold_10000_stability` 的关键现象：

- 建连全部成功，`echochat_ws_handshake_result_total{route="bench",result="success"} = 10000`
- 连接中段曾稳定维持：
  - `echochat_ws_online_connections{route="bench"} = 10000`
  - `process_open_fds ≈ 10010`
  - `go_goroutines ≈ 30014`
- 说明在 `1W` 在线时，当前服务模型大致呈现出“每连接约 3 个 goroutine”的量级
- 从资源采样看，`18:49:18` 开始在线连接数从 `10000` 降到 `9158`
- k6 汇总显示：
  - `ws_early_disconnect_count = 842`
  - `ws_session_duration_ms.min = 181100`
  - `ws_session_duration_ms.p95 = 300010`

这意味着：

- “接入能力”已经证明到 `1W`
- “5 分钟长稳保活”目前还没有完全通过

### 5.3 CPU 观察

这次资源采样里，服务端进程 CPU 峰值只看到 `2.8%`，稳态大多在 `1% ~ 2%` 左右。

这个结果可以说明两点：

- 在 benchmark 直连链路下，服务端维持 `1W` 在线连接的稳态 CPU 并不高
- 但当前采样间隔是 `5s`，更偏向观察稳态，不足以精确抓到“建连瞬时峰值”

如果下一轮你要专门讲“建连期 CPU 消耗”，建议把采样粒度改为 `1s`，或者直接接 node exporter / Prometheus 做时间序列图。

## 6. 原因判断

### 6.1 为什么说不是登录 / JWT / MySQL 瓶颈

原因很直接：

- 这轮压测已经不走 `/login`
- benchmark 鉴权只做 JWT 解析，不查数据库
- 握手成功率是 `100%`
- 服务端 fd 峰值只有 `10008`，远低于 `500000`
- 服务端 CPU 也没有跑高

所以当前 `300s` 场景里的 `842` 条提前断连，不像是鉴权、数据库、文件描述符或服务端 CPU 被打满导致的。

### 6.2 更可能的问题点

更像是下面几类问题叠加：

- 同机压测下，k6 自身长期持有 `1W` WebSocket 的稳定性边界
- 当前连接模型每连接约 `3` 个 goroutine，10k 在线时已到 `3W+` goroutine，虽然还能跑，但对更高规模不友好
- benchmark 路径虽然已经避开数据库和欢迎消息，但仍沿用了通用聊天客户端结构，不是“极简保活连接模型”

换句话说：

- 现在的“接得进来”没有问题
- 现在的“长时间全量稳定挂住”还需要继续打磨

## 7. 面向 5W 目标的优化方向

如果目标是后续往 `5W` 并发在线推进，建议优先做下面这些：

### P1. 把 benchmark 连接模型再极简化

当前 benchmark 仍复用正式聊天连接结构，至少带来：

- `Read()` goroutine
- `Write()` goroutine
- `forwardPendingMessages()` goroutine（channel 模式下）

这会让 `1W` 在线时 goroutine 直接到 `3W+`。

建议新增“只做保活、不进消息收发链路”的压测专用连接模型：

- 只保留必要的在线表登记
- 不启动消息转发相关 goroutine
- 不分配多余 channel

这一步做完，才更适合继续往 `2W / 3W / 5W` 冲。

### P2. 将同机压测改为双机压测

当前同机压测仍有天然限制：

- 本机端口范围上限
- 压测客户端和服务端抢同一台机器资源
- 不能把服务端真实瓶颈和压测端瓶颈完全分离

如果后面要认真验证 `5W`，建议至少改为：

- 压测机 1 台
- 服务机 1 台

### P3. 增加保活与断链原因可观测性

建议继续补：

- benchmark 连接的 close code / close reason 分布
- 客户端主动关闭 vs 服务端关闭分布
- 连接生命周期直方图
- benchmark 路由专属的在线数时间序列图

这样下次再出现“181s 提前断开”时，可以直接定位是 k6、网络层，还是服务端主动关闭。

### P4. 正式消息链路和 benchmark 链路分开考核

面试里最好分成两套指标讲：

- 在线承载能力：纯 `/bench/wss`
- 消息实时性 / 吞吐：正式 `/wss` + 发消息链路

这样指标更专业，也不会互相污染。

## 8. 本次产物位置

### 报告

- `/workspace/czk/Personal/EchoChat/docs/t_K6/ws_online_bench_report_20260329.md`

### 原始记录

- `60s 扫描结果`：
  `/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_online_bench_20260329_180128`
- `300s 稳定性补跑`：
  `/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_online_bench_20260329_184606`

### 关键原始文件

- `1W/300s summary.json`：
  `/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_online_bench_20260329_184606/hold_10000_stability/summary.json`
- `1W/300s resource.csv`：
  `/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_online_bench_20260329_184606/hold_10000_stability/resource.csv`
- `1W/300s stdout.txt`：
  `/workspace/czk/Personal/EchoChat/docs/t_K6/records/ws_online_bench_20260329_184606/hold_10000_stability/stdout.txt`

## 9. 下一步建议

下一轮最值得做的是：

1. 先把 benchmark 连接模型改成“单连接最小 goroutine / 最小 channel”版本
2. 把在线长稳测试从同机改成双机
3. 再复测 `1W / 300s`
4. 通过后再向 `2W -> 3W -> 5W` 逐级加压

这样后面的结果才更接近你真正想对外讲的“单机在线连接能力上限”。
