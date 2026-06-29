# Windows 双机 WebSocket 在线承载压测方案

## 1. 目标

这套方案用于完成下面这个场景：

- 压测机和服务端分离
- 压测机是 Windows 笔记本
- 服务端是 Linux 云服务器
- 服务模式切到 `kafka`
- 只测 WebSocket 在线连接承载能力
- 压测期间持续采集服务器数据

这轮压测的口径保持为“纯在线 benchmark”：

- 使用预生成 `access token`
- 直接连接 `/bench/wss`
- 不混入 `/login`
- 不把 welcome message 作为成功条件

## 2. 为什么这轮要切到 Kafka 模式

当前项目里，纯在线 benchmark 链路虽然不发业务消息，但连接建立后仍然会走正式的客户端结构。

两种模式的差别是：

- `channel` 模式下，每连接通常会拉起：
  - `Read`
  - `Write`
  - `forwardPendingMessages`
- `kafka` 模式下，每连接通常只保留：
  - `Read`
  - `Write`

所以在“只测在线承载”的前提下，`kafka` 模式会比 `channel` 模式少一个连接级 goroutine，更适合这轮目标。

## 3. 你当前机器条件下的现实上限

你的测压机是 Windows 笔记本，内存 `16GB`。

这意味着：

- 不适合继续用单台 `k6` 直接冲 `5000+ / 10000+`
- 当前更合理的默认目标是先验证：
  - `100`
  - `300`
  - `500`
  - `800`
  - `1000`

原因不是服务端一定扛不住，而是 `k6` 的“每连接一个 VU”模型会先占掉测压机内存。

所以这套默认脚本会先按 `100,300,500,800,1000` 的档位跑。

如果后续你要继续往更高推，建议：

1. 先验证这台 Windows 笔记本的本机可承载 VU 上限
2. 超过 `1000~1500` 后，改成多台测压机分摊
3. 或者换成专门的 Go WebSocket load generator

## 4. 服务端准备

### 4.1 配置要求

确保服务端配置文件 [config_local.toml](/workspace/czk/Personal/EchoChat/configs/config_local.toml#L1) 至少满足：

- `kafkaConfig.messageMode = "kafka"`
- `pressureTestConfig.enableBenchmarkRoutes = true`
- `pressureTestConfig.disableBenchmarkRequestLog = true`
- `pressureTestConfig.disableBenchmarkHotPathLog = true`
- `observabilityConfig.enableMetrics = true`
- `observabilityConfig.enablePprof = true`

改完后要重启服务。

### 4.2 网络要求

确保 Windows 笔记本可以访问：

- WebSocket 地址，例如：
  - `ws://<server-ip>:8081`
  - 或 `wss://<domain>`
- SSH
- `/metrics`
- `/debug/pprof/goroutine?debug=1`

如果服务挂在公网云服务器上，通常需要：

- 放行服务端口
- 放行 SSH 端口
- 如果走 Nginx / TLS，确认 `wss` 反代正常

### 4.3 服务端软件要求

服务端需要可用：

- `python3`
- `go`
- `systemctl`
- `ss`

这套脚本会通过 SSH 在服务端执行：

- `go run ./cmd/echo_chat_seed`
- `go run ./cmd/echo_chat_ws_tokens`
- `python3 docs/t_K6/scripts/collect_server_metrics.py`

## 5. Windows 测压机准备

Windows 机器需要安装：

- `k6`
- `ssh`
- `scp`

推荐直接使用 Windows 自带 OpenSSH Client 和单独安装的 `k6`。

## 6. 本次新增脚本

### 6.1 Windows 发压脚本

- [Start-EchoWsOnlineBench.ps1](/workspace/czk/Personal/EchoChat/docs/t_K6/scripts/Start-EchoWsOnlineBench.ps1)

职责：

- 通过 SSH 在服务端准备 benchmark 用户和 token
- 拉取 token 到 Windows 本地
- 按目标并发逐档执行本地 `k6`
- 每档启动并停止服务端采样
- 每档拉回服务端观测文件
- 生成本地 `stage_summary.csv`

### 6.2 服务端持续采样脚本

- [collect_server_metrics.py](/workspace/czk/Personal/EchoChat/docs/t_K6/scripts/collect_server_metrics.py)

职责：

- 持续采集服务端进程瞬时 CPU
- RSS / VSZ
- 线程数
- fd 数
- ESTABLISHED TCP 数
- Prometheus 指标：
  - benchmark 在线连接数
  - open fds
  - goroutines
  - heap
  - process memory
  - 网络收发总量和速率
  - benchmark 握手成功/失败计数

结束后还会额外落盘：

- `metrics.prom`
- `goroutine.txt`
- `collector_status.json`

### 6.3 k6 WebSocket 脚本增强

- [ws_online_tokens.js](/workspace/czk/Personal/EchoChat/docs/t_K6/scripts/ws_online_tokens.js#L1)

这次补了两个参数：

- `WS_PATH`
- `PING_INTERVAL_SECONDS`

这样双机场景下可以：

- 保持 `/bench/wss` 路径可配置
- 对长时间持有连接场景发送心跳 ping，降低中间链路空闲断开干扰

## 7. 推荐执行命令

在 Windows PowerShell 里执行：

```powershell
Set-ExecutionPolicy -Scope Process Bypass

.\docs\t_K6\scripts\Start-EchoWsOnlineBench.ps1 `
  -ServerHost "你的云服务器IP或域名" `
  -ServerUser "你的SSH用户名" `
  -ServerSshPort 22 `
  -WsUrl "ws://你的云服务器IP:8081" `
  -TargetVusList 100,300,500,800,1000 `
  -HoldSeconds 180 `
  -PingIntervalSeconds 30 `
  -CollectorInterval 2 `
  -Prefix "WSKAF" `
  -TelephoneStart 17630000000
```

如果你走 HTTPS / WSS，并且证书是自签名或测试证书：

```powershell
.\docs\t_K6\scripts\Start-EchoWsOnlineBench.ps1 `
  -ServerHost "你的域名" `
  -ServerUser "你的SSH用户名" `
  -WsUrl "wss://你的域名" `
  -InsecureSkipTlsVerify
```

## 8. 每一档执行时会发生什么

以 `step_00500` 为例：

1. Windows 脚本通过 SSH 在服务端启动 `collect_server_metrics.py`
2. Windows 本地运行 `k6`，500 个 VU 直连 `/bench/wss`
3. 服务端持续采样 CPU、内存、fd、goroutine、在线连接数
4. `k6` 结束后，Windows 通过 SSH 停止服务端采样
5. Windows 把服务端该档位的 `samples.csv / metrics.prom / goroutine.txt` 拉回本地
6. Windows 解析 `summary.json` 和 `samples.csv`，追加到 `stage_summary.csv`

## 9. 结果目录

Windows 本地结果会落在：

- `docs/t_K6/records/windows_ws_online_kafka_<timestamp>`

目录里主要有：

- `ws_tokens.json`
- `stage_summary.csv`
- `summary.md`
- 每档目录：
  - `summary.json`
  - `stdout.txt`
  - `exit_code.txt`
  - `server/samples.csv`
  - `server/metrics.prom`
  - `server/goroutine.txt`
  - `server/collector_status.json`

服务端远程结果会落在：

- `docs/t_K6/records/windows_remote_ws_online_kafka_<timestamp>`

## 10. 你这一轮应该怎么跑

推荐你按下面顺序做：

1. 先确认服务端已经切到 `kafka` 模式
2. 重启服务
3. 先跑一个 smoke：
   - `TargetVusList 20`
   - `HoldSeconds 30`
4. smoke 通过后，再跑：
   - `100,300,500,800,1000`
   - `HoldSeconds 180`
5. 如果 `1000` 也稳，再尝试：
   - `1200`
   - `1500`

但这一步要非常注意 Windows 笔记本内存。

## 11. 我对你这台 16G Windows 笔记本的建议

这一轮不要把目标写成“我就要从单台 Windows 笔记本直接打到 1 万”。

更稳妥的目标是：

- 先用这套双机脚本把 `1000` 左右的在线连接承载压测流程跑顺
- 把服务端观测数据沉淀完整
- 证明双机、Kafka、benchmark 直连链路下，结果是可信的

之后如果你要冲更高：

1. 再增加第 2 台 load generator
2. 或者直接改成自定义 Go 压测客户端

## 12. 当前方案的边界

这套方案已经解决了下面这些问题：

- 压测机和服务端不再抢同机资源
- 不再混入 `/login`
- 可以持续采集服务端指标
- 结果可按档位自动归档

但它还没有解决：

- 单台 `k6` 在 16GB Windows 笔记本上的高并发内存上限
- 多压测机聚合汇总
- 专门的 Go 级别高密度 WebSocket load generator

所以这套方案是你当前设备条件下的“可直接落地版”，不是“最终 1 万以上极限版”。
