# EchoChat WebSocket 5 万并发评估

## 1. 结论先说

当前这台机器本身不是最先到顶的瓶颈。

已确认的机器基线：

- `32 vCPU`
- `247GiB RAM`
- 当前服务进程 `LimitNOFILE=65535`
- 当前系统临时端口范围 `32768-60999`，单机单源地址对同一目标地址理论上大约只有 `28232` 个可用临时端口

所以，当前第一项压测结果“不正常”，主要不是因为 CPU 或内存不够，而是下面几类问题叠加：

1. 压测结构本身不是“纯 WebSocket 在线能力测试”，而是把 `login + JWT + MySQL 查用户 + WebSocket upgrade + welcome message` 绑在一起测了。
2. MySQL 当前 `max_connections=151`，日志里已经明确出现大量 `Error 1040: Too many connections`，这会直接把登录成功率和 `/wss` 鉴权成功率打穿。
3. 你现在是同机、同 IP、本地回环压测，单机临时端口范围先天就不足以支持真正的 `5 万` 并发长连接。
4. `channel` 模式下，连接注册和欢迎消息发送走单 goroutine 主循环，突发建连时会被串行处理。
5. 当前连接链路日志非常重，且同时写 `stdout + 文件`，对高并发建连场景有明显干扰。

一句话判断：

当前 `1000` 级别压测先失败，主要是测试方法和连接前置链路先到瓶颈，不是 WebSocket 长连接本体已经证明只能到 `1000`。

## 2. 这次为什么没有先看到 CPU 打满

之前的 `resource.csv` 里 CPU 基本是 `0.0`，不是因为服务没有消耗 CPU，而是采样方式有问题。

原脚本用的是：

- `ps -p <pid> -o %cpu`

这个值更接近“进程自启动以来的累计平均 CPU”，而不是压测窗口里的瞬时 CPU。

当前服务进程已经运行了两天以上，像下面这样：

- `ps -p 702068 -o pid,etime,%cpu,%mem,rss,vsz,cmd`
- 返回里 `ELAPSED=2-04:01:31`，`%CPU=0.0`

所以短时间压测产生的 CPU 峰值，会被长时间的空闲期均摊掉，看起来就像没有消耗。

我已经把脚本修了，后续会改成采瞬时 CPU，并额外记录线程数和 fd 数：

- `docs/t_K6/scripts/run_ws_online_suite.sh`

这次修改点：

- 新增 `pidstat` 采样瞬时 CPU
- 新增线程数采样
- 新增 fd 数采样

后面你再复跑，`resource.csv` 会更可信。

## 3. 当前测试结构里，真正先炸的地方

### 3.1 登录和 `/wss` 鉴权都在查 MySQL

当前压测脚本并不是拿预生成 token 直接建连，而是每个 VU 都先：

1. 调 `/login`
2. 拿 `access_token`
3. 再调 `/wss?token=...`

脚本在这里：

- `docs/t_K6/scripts/ws_online_constant.js`

其中：

- `37-76` 行先调用 `/login`
- `85-123` 行再调用 `/wss`

而服务端这两段链路都要查库：

- 登录按手机号查用户：
  - `internal/service/gorm/user_info_service.go:123`
- `/wss` 进入 `AuthRequired()` 后，会按 `claims.UserUUID` 再查一次用户：
  - `internal/https_server/auth_middleware.go:52`
  - `internal/https_server/auth_middleware.go:75`

也就是说，一条连接最少两次 MySQL 读。

这对“在线连接能力压测”非常不友好，因为它把数据库认证链路的上限，混进了长连接上限。

### 3.2 MySQL 已经明确打到连接上限

数据库当前配置：

- `max_connections = 151`
- `innodb_buffer_pool_size = 134217728`，也就是 `128MB`

压测日志已经明确出现：

- `Error 1040: Too many connections`
- `http.auth.user_lookup_failed`

这些错误都在这份日志里能看到：

- `logs/echochat.log`

说明当前 `500/800/1000` 的主要失败原因之一非常清晰：

- 不是 token 签名算法错了
- 不是 WebSocket 协议本身扛不住
- 是登录查库和 `/wss` 鉴权查库把 MySQL 连接池打满了

这一点和已有压测结果是对得上的。

## 4. 为什么“同机压测”天然不适合拿来证明 5 万长连接

当前服务监听：

- `configs/config_local.toml` 里 `host = "127.0.0.1"`

而压测脚本也是直接打本机：

- `BASE_URL=http://127.0.0.1:8081`
- `WS_URL=ws://127.0.0.1:8081`

这样做有两个天然问题。

### 4.1 单机临时端口范围先卡住

当前系统：

- `net.ipv4.ip_local_port_range = 32768 60999`

总共只有：

- `60999 - 32768 + 1 = 28232`

这意味着，如果你还是“同一台机器、同一个源 IP、连同一个目标地址和端口”，那真正并发 TCP 连接数会先被临时端口数限制住。

所以在当前测试结构下，`5 万` 长连接不是“压出来再看看”，而是理论上就过不去。

### 4.2 当前服务 fd 上限也不够舒服

当前服务：

- `LimitNOFILE=65535`

如果只看服务端，`5 万` 连接加上监听 socket、日志文件、数据库连接、Redis 连接，已经非常贴边。

这还没算压测端自己也需要大量 fd。

所以真要做 `5 万` 在线，至少需要把服务端和压测端的 fd 上限都抬到更高，比如：

- `200000`
- 更稳妥一点是 `500000`

## 5. 代码结构里，会拖累突发建连的地方

### 5.1 `channel` 模式登录注册是单 goroutine 串行处理

`channel` 模式下：

- `ChatServer.Start()` 是一个主循环
- `Login`、`Logout`、`Transmit` 都在这个循环里串行处理

代码位置：

- `internal/service/chat/server.go:114`

更关键的是，登录分支里不仅仅是登记连接，还直接做了欢迎消息写回：

- `internal/service/chat/server.go:120`
- `internal/service/chat/server.go:127`

也就是：

1. 收到一个登录事件
2. 写入 `Clients`
3. 立刻 `client.Conn.WriteMessage("欢迎来到EchoChat聊天服务器")`
4. 然后才处理下一个登录事件

这对“瞬时 1 万、5 万同时建连”的场景非常不友好，因为欢迎消息是串行写出去的。

### 5.2 每条连接至少会拉起 2 到 3 条 goroutine

当前每个 WebSocket 连接会启动：

- `go client.forwardPendingMessages()`，`channel` 模式才有
- `go client.Read()`
- `go client.Write()`

代码位置：

- `internal/service/chat/client.go:362`
- `internal/service/chat/client.go:367`
- `internal/service/chat/client.go:374`
- `internal/service/chat/client.go:375`

这意味着：

- `channel` 模式下每连接约 `3` 条 goroutine
- `5 万` 连接大约就是 `15 万` goroutine

这不是绝对做不到，但会带来明显的调度和内存压力。

### 5.3 每条连接自带 3 个 channel，缓冲还不小

每个连接都有：

- `SendTo`，容量 `CHANNEL_SIZE`
- `SendBack`，容量 `CHANNEL_SIZE`
- `CriticalBack`，容量 `1`

代码位置：

- `internal/service/chat/client.go:350`
- `pkg/constants/constants.go:5`

而当前：

- `CHANNEL_SIZE = 100`

对于大规模在线、但绝大多数连接长期空闲的场景，这个默认缓冲偏大。

在 `5 万` 连接下，这部分内存会变成一个很实际的成本。

### 5.4 日志非常重，而且是双写

当前日志实现是：

- 同时写 `stdout`
- 同时写滚动日志文件

代码位置：

- `pkg/zlog/logger.go:48`
- `pkg/zlog/logger.go:50`
- `pkg/zlog/logger.go:51`

而连接链路上又记录了很多 info 日志：

- `http.request.start`
- `http.request.finish`
- `ws.login.request`
- `ws.connection.open`
- `ws.connection.ready`
- `ws.connection.login`
- `ws.read.start`
- `ws.write.start`

在高并发建连时，这会直接形成非常大的日志 I/O 和 JSON 编码开销。

如果你想验证 `5 万` 在线，必须先把“连接建立路径上的高频 info 日志”压下去，否则你测出来的是“日志系统承载能力”，不是“连接系统承载能力”。

## 6. 对 5 万目标的现实判断

### 6.1 以当前结构直接冲 5 万，不现实

按当前结构，先卡住你的不会是 Go runtime，也不会先是内存，而是：

1. MySQL `max_connections=151`
2. `/login + /wss` 双查库
3. 单机临时端口上限 `28232`
4. 单 goroutine 串行处理欢迎消息
5. 过重日志
6. `LimitNOFILE=65535` 太贴边

### 6.2 如果把目标改成“5 万稳定在线”，理论上是可以往那个方向走的

但前提是测试口径和系统结构都要改。

真正合理的目标应该拆成两步：

1. 先证明“服务端可以承载 5 万已建立连接稳定在线”
2. 再单独测“登录/鉴权/建连洪峰能力”

这是两个不同问题。

你现在这份压测，把它们混在一起了。

## 7. 优化方向，按优先级排

### P0：先把测试方法改对

这是最高优先级。

1. 把“在线连接能力”从“登录能力”里拆出来。
   做法：
   - 先离线批量生成 access token
   - WebSocket 压测直接使用预生成 token
   - 不要每个连接现调 `/login`

2. 不要继续用“同机同 IP 打同机服务”证明 `5 万`。
   做法：
   - 把服务监听从 `127.0.0.1` 改成 `0.0.0.0`
   - 使用独立压测机
   - 最好用多台压测机分摊连接

3. “在线连接能力”压测不要把 welcome message 作为核心成功条件。
   当前脚本会顺带看 welcome message 是否收到，这会把单 goroutine 欢迎消息路径也掺进去。
   更合理的在线测试口径是：
   - TCP / WS upgrade 成功
   - 握手完成后连接稳定存活 N 分钟

### P1：先把系统硬上限抬起来

这是第二优先级，不然你还没进应用层就先撞墙。

1. 提升 `LimitNOFILE`
   - 服务端建议至少 `200000`
   - 压测端也要同步提高

2. 提升内核网络参数
   重点看：
   - `net.ipv4.ip_local_port_range`
   - `net.core.somaxconn`
   - `net.ipv4.tcp_max_syn_backlog`
   - `net.netfilter.nf_conntrack_max`，如果后面经由 NAT / 防火墙

3. 提升 MySQL 并发能力
   重点看：
   - `max_connections`
   - `innodb_buffer_pool_size`
   - thread cache

但这里要强调：

如果在线连接测试已经改成“预生成 token 直连”，那 MySQL 就不该再成为 WebSocket 在线压测的主瓶颈。

### P2：优化建连热路径

这是第三优先级，属于代码层。

1. `/wss` 鉴权避免每次握手都查 MySQL
   方向：
   - JWT 验签通过后，优先只做 session / refresh 维度校验
   - 用户资料改为 Redis 缓存或延迟加载
   - 至少不要把“用户存在性确认”放在每次握手热路径上查 MySQL

2. 把欢迎消息从 `ChatServer.Start()` 的单线程主循环里拿出去
   方向：
   - 登录事件只负责登记在线连接
   - 欢迎消息交给连接自己的写协程异步处理

3. 降低每连接内存占用
   方向：
   - 下调 `CHANNEL_SIZE`
   - 评估是否真的需要每连接 3 个 channel
   - 对“仅在线不发消息”的连接采用更轻量结构

4. 减少锁竞争
   方向：
   - `Clients` 可考虑分片 map 或其他并发结构
   - 但这一项不是当前最先炸的点，优先级低于前面几项

### P3：降低日志干扰

这是高并发建连的必做项。

1. 压测场景下关闭或采样以下 info 日志：
   - 请求开始/结束日志
   - `ws.connection.open`
   - `ws.connection.ready`
   - `ws.read.start`
   - `ws.write.start`
   - `ws.connection.login`

2. 对高频日志做采样或降级到 debug

3. 压测期间尽量避免同时 `stdout + file` 双写

### P4：补齐可观测性

建议加：

1. `pprof`
2. Prometheus 指标
3. goroutine 数
4. heap / alloc / gc pause
5. fd 数
6. 在线连接数
7. 握手成功率
8. 鉴权失败原因分布

## 8. 我建议你下一步怎么做

如果目标是拿去校招面试，最稳的路线不是直接怼 `5 万`，而是按下面顺序推进：

1. 先做一版“纯 WebSocket 在线压测”
   - 预生成 token
   - 只测 upgrade + 在线稳定性

2. 再做一版“登录 / 鉴权 / 建连洪峰压测”
   - 单独测 `/login`
   - 单独测 `/wss` 鉴权
   - 单独测登录后建连

3. 再做“多机压测”
   - 至少把服务端和压测端分开

4. 在此之前先做两类基础优化：
   - 调高 MySQL / systemd / sysctl 上限
   - 关掉连接路径上的高频日志

## 9. 最后给你的判断

你现在这份项目，不是“只能做 1000 长连接”，而是“当前这套压测结构无法证明它能到更高”。

真正把结果压坏的首要原因是：

1. MySQL `Too many connections`
2. 同机同 IP 压测的临时端口上限
3. 建连链路把登录和查库绑进来了
4. `channel` 模式下欢迎消息串行发送
5. 日志太重

如果你的目标是“校招简历里写一个漂亮的在线连接指标”，正确方向不是现在就拿这份数据硬写，而是先把压测口径拆对，再去冲一个可信的稳定在线数字。
