# EchoChat 压测计划

## 1. 目标

本计划的目标不是单纯测几个 HTTP 接口的 QPS，而是围绕 EchoChat 这类即时通讯系统最有简历价值的能力做压测设计，重点覆盖以下四类核心指标：

1. WebSocket 长连接承载能力
2. 单聊 / 群聊消息实时性
3. 消息吞吐能力与高并发稳定性
4. 持续压测下的资源利用与异常控制

本计划以“适合写进简历、适合在面试中讲清楚工程价值”为导向，优先沉淀可量化、可复现、可解释的指标。

## 2. 当前项目压测对象梳理

结合当前项目代码，EchoChat 的压测对象可分为五层：

### 2.1 认证与会话层

- `POST /login`
- `POST /auth/refresh`
- `POST /user/wsLogout`
- 鉴权中间件 `AuthRequired()`

该层负责 access token、refresh token、用户查库和会话有效性，是所有后续请求和 WebSocket 握手的入口。

### 2.2 长连接接入层

- `GET /wss?token=...`
- `api/v1/ws_controller.go`
- `internal/service/chat/client.go`

该层负责 WebSocket 握手、在线连接注册、收发消息循环以及连接下线。

### 2.3 实时消息链路

- WebSocket 消息体：`request.ChatMessageRequest`
- `internal/service/chat/server.go`
- `internal/service/chat/kafka_server.go`

该层是最核心的实时通信路径。当前项目支持两种消息模式：

- `channel` 模式：消息直接进入进程内 `ChatServer.Transmit`
- `kafka` 模式：消息先写 Kafka，再由消费者回推

这意味着压测计划必须分两轮：

1. 先做当前默认 `channel` 模式基线
2. 再做 `kafka` 模式对比，突出架构演进收益

### 2.4 聊天数据查询层

- `POST /message/getMessageList`
- `POST /message/getGroupMessageList`
- `POST /session/getUserSessionList`
- `POST /session/getGroupSessionList`
- `POST /contact/getUserList`

这层更适合展示缓存命中、历史消息查询时延、列表加载体验和数据库压力。

### 2.5 辅助业务层

- `POST /session/openSession`
- `POST /contact/getContactInfo`
- `POST /contact/applyContact`

这部分不是简历最核心的亮点，但适合补齐“完整聊天业务链路”的压测覆盖。

## 3. 适合简历展示的核心指标

如果最后只保留 4 组最有含金量的结果，优先保留下面这些：

1. 单机最大稳定 WebSocket 在线连接数
2. 单聊 / 群聊消息端到端时延的平均值、P95、P99
3. 消息链路吞吐量，单位 `msg/s`
4. 持续压测下的错误率、断连率、资源占用

建议最后整理成类似这样的表达：

> 基于 WebSocket + Kafka 实现即时通讯后端，单机支持 X 个稳定在线连接；在 Y 并发发消息场景下，单聊消息 P95 时延为 N ms；消息链路吞吐达到 Z msg/s；30 分钟持续压测下错误率低于 A%，服务 CPU / 内存保持稳定。

## 4. 压测优先级与模块顺序

按价值和执行顺序，建议按以下顺序一模块一模块推进：

### Phase 0: 压测前基线校验

目标：

- 确认压测环境可重复执行
- 确认测试账号、联系人关系、群组关系、会话数据齐备
- 确认后端当前运行在 `channel` 模式还是 `kafka` 模式

输出：

- 环境基线记录
- 测试用户与测试群组清单
- 当前配置快照

### Phase 1: WebSocket 在线连接能力

目标：

- 测单机最大稳定在线连接数
- 测连接建立成功率
- 测持续在线 10 分钟 / 30 分钟断连率

重点指标：

- 建连成功率
- 在线连接数峰值
- 平均建连耗时
- 压测期间断连率
- 服务 CPU / 内存 / goroutine 数量

简历价值：

- 最能体现即时通讯项目的“实时系统”属性

### Phase 2: 单聊消息实时性

目标：

- 测用户 A 向用户 B 发消息时的端到端时延
- 测不同并发消息数下的平均时延与 P95 / P99

重点指标：

- 平均时延
- P95 / P99 时延
- 发送成功率
- 消息乱序 / 丢失情况

简历价值：

- 最直接体现“实时通信质量”

### Phase 3: 群聊广播能力

目标：

- 测一个群内 10 / 50 / 100 在线成员时的广播时延
- 测群消息扇出后整体时延分布

重点指标：

- 单条群消息广播完成时间
- 群成员接收覆盖率
- P95 / P99 时延
- 高扇出时的错误率

简历价值：

- 这个指标比“普通 WebSocket 聊天”更有区分度

### Phase 4: 消息吞吐能力

目标：

- 测系统在持续消息流下每秒能稳定处理多少条消息
- 分别测 `channel` 模式与 `kafka` 模式

重点指标：

- 峰值吞吐 `msg/s`
- 稳态吞吐 `msg/s`
- 服务端错误率
- 数据落库成功率

简历价值：

- 很适合写“Kafka 异步削峰”“吞吐提升”

### Phase 5: 长稳压测

目标：

- 以中高并发在线用户和持续消息流压 30 分钟以上
- 观察错误率、断连、内存增长、goroutine 增长趋势

重点指标：

- 持续压测错误率
- 断连率
- goroutine 是否持续增长
- 内存是否持续上涨
- 服务是否出现明显抖动

简历价值：

- 能证明项目不仅“能跑”，而且“能稳”

### Phase 6: 关键 HTTP 接口性能

目标：

- 给登录、刷新 token、会话列表、消息列表等接口补充性能数据

建议覆盖：

- `POST /login`
- `POST /auth/refresh`
- `POST /contact/getUserList`
- `POST /session/getUserSessionList`
- `POST /message/getMessageList`
- `POST /message/getGroupMessageList`

重点指标：

- 平均响应时间
- P95 / P99
- 错误率

简历价值：

- 作为补充项，不作为主卖点

## 5. 推荐最终展示的指标矩阵

建议最终至少沉淀下面这 6 组数据：

| 模块 | 关键指标 | 是否建议写入简历 |
| --- | --- | --- |
| WebSocket 在线连接 | 最大稳定在线连接数、建连成功率、断连率 | 是 |
| 单聊实时通信 | 平均时延、P95、P99 | 是 |
| 群聊广播 | 广播完成时延、接收覆盖率 | 是 |
| 消息吞吐 | 峰值 / 稳态 `msg/s` | 是 |
| 长稳压测 | 错误率、goroutine / 内存趋势 | 是 |
| HTTP 接口 | 登录 / 消息列表 / 会话列表 RT | 选写 |

## 6. 执行策略

### 6.1 先做 channel 模式基线

当前 `config_local.toml` 中配置的是：

- `mainConfig.port = 8081`
- `kafkaConfig.messageMode = "channel"`

因此第一轮压测先基于 `channel` 模式完成基线记录。

这样做的好处是：

- 环境简单，便于先把脚本和统计口径跑通
- 可以先暴露本地通道容量、数据库落库和连接治理的瓶颈

### 6.2 再做 kafka 模式对比

第二轮将 `messageMode` 切换为 `kafka` 后重复 Phase 2 ~ Phase 5 的核心场景，重点比较：

- 消息吞吐是否提升
- P95 / P99 是否改善或恶化
- 高峰下服务资源抖动是否更平稳

这一轮的目标不是“绝对数值最好看”，而是证明架构升级带来的收益和代价。

## 7. 工具建议

### 7.1 核心压测工具

建议主工具使用 `k6`，原因如下：

- HTTP 压测和场景编排能力成熟
- 脚本化能力强，适合把登录、会话打开、消息查询串成完整流程
- 可以组织阶段式场景，适合做简历向的压测记录

### 7.2 WebSocket 压测

如果当前本地 `k6` 版本支持 WebSocket 模块，则统一用 `k6`。

如果不支持，则采用双工具策略：

- `k6` 负责 HTTP 场景、认证、列表、历史消息
- 专门的 WebSocket 压测脚本负责长连接与实时消息

但最终记录仍统一沉淀在 `docs/t_K6`。

### 7.3 系统资源观测

压测同时记录：

- CPU
- 内存
- goroutine 数量
- WebSocket 连接数
- MySQL / Redis 连接数
- 服务日志中的错误事件

建议观测方式：

- `systemctl status echochat.service`
- `journalctl -u echochat.service`
- `ss -antp`
- `top` / `pidstat` / `vmstat`
- 必要时增加 `pprof`

## 8. 数据准备策略

压测前需要准备一批固定测试数据，至少包含：

1. 多个可登录账号
2. 一组双向好友关系
3. 至少一个中型群组
4. 稳定可复用的会话数据

建议准备：

- 20 个基础测试账号
- 1 个主发送账号
- 1 个主接收账号
- 1 个 20 人群
- 1 个 50 人群
- 1 个 100 人群

这样后续压测脚本能稳定复用，不会每轮都重新造数据。

## 9. 记录规范

后续所有压测脚本和记录统一放在：

- `/workspace/czk/Personal/EchoChat/docs/t_K6`

建议目录约定如下：

```text
docs/t_K6/
  k6_plan.md
  scripts/
  records/
  baselines/
  summaries/
```

记录规范建议：

- 一个场景一份脚本
- 一次压测一份记录
- 一类结论一份总结

记录内容至少包括：

1. 时间
2. 服务配置
3. 数据规模
4. 并发设置
5. 执行时长
6. 核心指标
7. 异常现象
8. 结论与下一步建议

## 10. 第一阶段落地建议

下一步不直接从“登录接口压测”开始，而是按下面顺序落脚本：

1. WebSocket 建连脚本
2. 单聊消息时延脚本
3. 群聊广播脚本
4. 长稳压测脚本
5. HTTP 关键接口补充脚本

其中第一批最值得先做的是：

1. WebSocket 在线连接能力
2. 单聊消息端到端时延
3. 消息吞吐能力

## 11. 当前阶段结论

从当前项目结构看，EchoChat 最值得凸显的不是“接口多”，而是下面这几个工程点：

1. JWT 双 token 鉴权
2. WebSocket 长连接在线通信
3. 聊天消息实时推送与落库
4. `channel / kafka` 双消息模式
5. 缓存、会话、消息列表的读写路径
6. 优雅关机与连接治理

因此压测策略也必须围绕“实时性、吞吐、稳定性、资源利用”展开，而不是只测一个登录接口 QPS。

后续执行时，优先把结果打磨成可直接写进简历的表达。
