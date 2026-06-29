# EchoChat DLQ 开发计划

## 1. 目标

本次开发只解决 3 个核心问题：

1. 在主链路内把失败治理标准化，区分临时故障和永久故障。
2. 用 MySQL 表落地 DLQ，把主链路暂时处理不了、但又不能丢的消息可靠停放下来。
3. 增加一套独立的自动重放和人工治理机制，把失败从“阻塞 Kafka 主消费”改造成“可治理能力”。

本方案严格基于当前项目已有链路设计，重点改造以下位置：

- [kafka_server.go](/workspace/czk/Personal/EchoChat/internal/service/chat/kafka_server.go)
- [kafka_consumed_decode.go](/workspace/czk/Personal/EchoChat/internal/service/chat/kafka_consumed_decode.go)
- [kafka_message_support.go](/workspace/czk/Personal/EchoChat/internal/service/chat/kafka_message_support.go)
- [message_sequence.go](/workspace/czk/Personal/EchoChat/internal/service/chat/message_sequence.go)
- [kafka_group_async_pipeline.go](/workspace/czk/Personal/EchoChat/internal/service/chat/kafka_group_async_pipeline.go)
- [kafka_conversation_bucket.go](/workspace/czk/Personal/EchoChat/internal/service/chat/kafka_conversation_bucket.go)

---

## 2. 范围

本次开发覆盖 4 个部分：

1. DLQ MySQL 表结构与 DAO 落地
2. 主链路失败分类、有限重试、DLQ 入表接入
3. 自动重放调度器、统一重放入口和 4 类 handler
4. 人工治理后台的后端接口与前端页面

本次不做的内容：

- 不做编辑 `raw_payload` 后重放
- 不做复杂审批流
- 不做跨系统统一告警平台，只预留告警埋点
- 不把 `redis_read` / `redis_write` 纳入 DLQ

---

## 3. 失败治理落地规则

### 3.1 自动重放

自动重放只处理 4 类：

1. 会话顺序号生成阶段的临时失败
2. MySQL 持久化阶段的临时失败
3. WebSocket 消息分发阶段的临时失败
4. 群成员查询阶段的临时失败

### 3.2 人工处理

人工处理主要覆盖这些典型情况：

1. 反序列化失败
2. 路由非法或协议不支持
3. 音视频扩展字段损坏
4. 接收方标识非法
5. `conversation_key + session_seq` 唯一键冲突
6. 群成员 JSON 脏数据
7. `conversation bucket worker panic`

### 3.3 不进 DLQ

以下场景不进 DLQ：

1. `uuid` 唯一键冲突
2. `redis_read`
3. `redis_write`

原因：

- `uuid` 冲突按幂等成功处理
- `redis_read` / `redis_write` 走降级继续，不阻断主链路

---

## 4. 用户态窗口与 DLQ 窗口

这里必须拆成两套时间，不允许混为一谈。

### 4.1 用户态发送窗口

用户态窗口必须很短。

建议：

- 主链路快速重试：几百毫秒到几秒
- 主链路 + 短窗口自动补救总时长：控制在 `10s` 左右

含义：

- 在这个窗口内，前端可以显示“发送中”
- 超过窗口还没成功，前端必须显示“失败”

### 4.2 DLQ 治理窗口

DLQ 的窗口可以更长，但它不再服务于“用户还在等发送成功”，而是服务于失败治理。

它的意义主要是：

1. 自动补齐下游动作
2. 为临时故障恢复后再处理保留机会
3. 让人工治理有抓手

结论：

- 主消息成立性错误的自动重放窗口要短
- 下游补偿类错误的自动重放窗口可以更长

---

## 5. 主链路改造计划

### 5.1 第一阶段：失败分类标准化

目标：把当前“只有 retryable / nonRetryable”的结果，升级为“阶段 + 错误码 + 失败类型 + 治理方式”。

开发内容：

1. 增加统一错误分类模块
2. 为每个阶段补结构化 `error_code`
3. 把 `session_seq`、`mysql_persist`、`websocket_dispatch`、`group_member_query` 的错误进一步细分
4. 单独识别 `mysql duplicate session_seq` 和 `mysql duplicate uuid`

预期产出：

- 新的错误分类 helper
- 主链路统一失败治理入口
- 统一的重试策略配置

### 5.2 第二阶段：主链路有限重试

目标：把“无限卡住 Kafka”改成“有限重试后转入 DLQ”。

开发内容：

1. 在消费链路内增加有限重试
2. 主链路只做短平快重试
3. 重试次数和退避时间按阶段配置
4. 重试用尽后写入 DLQ
5. 写入 DLQ 成功后，主链路对该 Kafka 消息做 `MarkMessage`

建议默认值：

- 主链路重试次数：3 次
- 退避：`200ms -> 1s -> 5s`

改造点：

- `ConsumeClaim`
- `handleConsumedMessage`
- `handleDecodedConsumedMessage`
- `group async pipeline`
- `conversation bucket worker`

### 5.3 第三阶段：DLQ 落表接入

目标：主链路失败消息可靠入 MySQL DLQ 表。

开发内容：

1. 增加 DLQ Model、DAO、Service
2. 增加 DLQ 写入器
3. 在失败点采集 `raw_payload`、`message_id`、`conversation_key`、`session_seq`、`stage`、`error_code`
4. 自动重放类和人工处理类设置不同 `status`
5. 为重放准备 `payload_snapshot` 和 `context_snapshot`

预期结果：

- 临时故障：`pending`
- 永久故障：`manual`
- 幂等恢复：不落 DLQ

---

## 6. 自动重放机制开发计划

### 6.1 目标

自动重放从主链路独立出来，不再依赖 Kafka 原地反复消费。

### 6.2 运作方式

采用“定时扫描 + 批量抢占 + 并发重放 + 状态回写”的模型。

自动重放服务做 5 件事：

1. 独立 goroutine 定时扫描 MySQL DLQ 表
2. 捞出 `status=pending` 且 `next_retry_at <= now()` 的记录
3. 抢占这些记录，把状态改成 `retrying`
4. 根据 `stage` 调用统一重放入口
5. 更新 `attempt_count`、`next_retry_at`、`status`

### 6.3 默认参数

建议第一版默认值：

- 扫描周期：`10s`
- 扫描批大小：`100`
- 重放 worker 并发：`2~4`
- 自动重放最大次数：`5`
- 主消息成立性错误自动重放窗口：短窗口，建议不超过 `10s~30s`
- 下游补偿错误自动重放窗口：可放宽，建议 `5min~30min`

### 6.4 代码拆分

建议新增这些模块：

- `internal/service/dlq/dlq_service.go`
- `internal/service/dlq/dlq_writer.go`
- `internal/service/dlq/dlq_replay_scheduler.go`
- `internal/service/dlq/dlq_replay_dispatcher.go`
- `internal/service/dlq/dlq_replay_handlers.go`

### 6.5 自动重放规则

1. `session_seq` 临时失败：自动重放
2. `mysql_persist` 普通临时异常：自动重放
3. `websocket_dispatch` 临时失败：只重放分发动作
4. `group_member_query` 临时失败：自动重放下游群扇出

### 6.6 自动重放状态

`dlq_message.status` 建议只用这 5 个：

1. `pending`
2. `retrying`
3. `done`
4. `manual`
5. `discarded`

字段语义：

- `pending`：等待下一次重放
- `retrying`：已被某个 worker 抢占，正在处理中
- `done`：处理完成
- `manual`：自动机制停止，转人工
- `discarded`：人工确认无需再处理

### 6.7 自动重放终止条件

以下任一条件满足则不再自动重放：

1. 达到最大自动重放次数
2. 超过最大自动重放时间窗口
3. 错误被重新识别为永久故障

处理结果：

- 转 `manual`
- 记录最后失败信息
- 记录操作日志

---

## 7. 重放 handler 开发计划

不为每种错误写一套独立调度器。  
采用“统一入口 + 4 个 stage handler”。

### 7.1 统一入口

统一入口负责：

1. 接收 DLQ 记录
2. 按 `stage` 分发
3. 返回标准化重放结果

建议抽象：

```go
func HandleDLQRecord(record *DLQMessage) ReplayResult
```

### 7.2 四类 handler

#### `handleSessionSeqReplay`

步骤：

1. 从 `raw_payload` 或 `payload_snapshot` 还原原始请求
2. 重新生成 `session_seq`
3. 重新构造 `model.Message`
4. 继续走后续持久化和分发

#### `handleMysqlPersistReplay`

步骤：

1. 从 `payload_snapshot` 还原已构造好的消息
2. 保持原 `session_seq`
3. 重新执行 MySQL 持久化
4. 成功后继续后续分发

关键点：

- 不能重新申请新的 `session_seq`

#### `handleWebsocketDispatchReplay`

步骤：

1. 还原待分发消息
2. 还原接收方或分发上下文
3. 只重放 WebSocket 分发动作

关键点：

- 不重新落库
- 不重新生成 `session_seq`

#### `handleGroupMemberQueryReplay`

步骤：

1. 还原已落库消息
2. 重新查群成员
3. 重新构造群扇出
4. 只补群消息分发

关键点：

- 不重新走消息主存储

---

## 8. 人工治理后台开发计划

### 8.1 后端接口

需要提供这些接口：

1. `GET /admin/dlq/messages`
2. `GET /admin/dlq/messages/:id`
3. `GET /admin/dlq/stats`
4. `POST /admin/dlq/messages/:id/replay`
5. `POST /admin/dlq/messages/batch-replay`
6. `POST /admin/dlq/messages/:id/mark-manual`
7. `POST /admin/dlq/messages/:id/discard`
8. `GET /admin/dlq/messages/:id/logs`

### 8.2 前端页面

至少做 4 个页面或模块：

1. DLQ 列表页
2. DLQ 详情页
3. 批量治理操作面板
4. 操作日志面板

### 8.3 列表页能力

支持按这些维度筛选：

1. `stage`
2. `error_code`
3. `status`
4. `message_id`
5. `conversation_key`
6. 时间范围
7. 是否自动重放

### 8.4 人工治理状态

第一版人工治理不把“重放”作为通用动作开放，而是采用更清晰的治理状态：

1. `open`
2. `in_progress`
3. `closed`

同时补一个关闭原因字段：

1. `discarded`
2. `externally_fixed`
3. `expected`
4. `merged_into_incident`

含义如下：

| 治理状态 | 含义 | 价值 |
| --- | --- | --- |
| `open` | 这条消息已经进入人工治理池，但还没人接手 | 能看到积压、统计 backlog、做告警 |
| `in_progress` | 已经有人接手，正在查根因、修数据、看日志、关联事故 | 避免多人重复处理，体现“正在处理”，便于审计 |
| `closed` | 这条治理记录已经结束，不再继续跟踪 | 能形成闭环，便于归档和复盘 |

### 8.5 人工页面展示与操作

人工治理页面至少要展示这些信息：

1. 消息信息：`message_id`、`conversation_key`、`session_seq`
2. 来源信息：`topic`、`partition`、`offset`
3. 失败信息：`stage`、`error_code`、`last_error`
4. 快照信息：`raw_payload`、`payload_snapshot`、`context_snapshot`
5. 治理信息：`status`、`attempt_count`、`next_retry_at`、`first_failed_at`、`last_failed_at`
6. 处理信息：处理人、处理时间、历史操作日志、备注

第一版人工页面只提供这些动作：

1. 标记 `open`
2. 标记 `in_progress`
3. 标记 `closed`
4. 选择关闭原因：`discarded`、`externally_fixed`、`expected`、`merged_into_incident`
5. 填写备注
6. 关联事故单或 bug 单
7. 查看历史操作记录

不开放直接修改业务 payload，也不开放对主消息的通用“人工重放”按钮。

---

## 9. 数据结构开发计划

本次至少落 2 张表：

1. `dlq_message`
2. `dlq_operation_log`

第一期先不拆独立任务表，直接用 `dlq_message` 自身的 `status`、`attempt_count`、`next_retry_at` 驱动自动重放。

核心字段的开发要求：

- `status`：当前处理状态
- `attempt_count`：已经自动治理了多少次
- `max_attempt_count`：最多允许自动治理多少次
- `next_retry_at`：下一次允许扫描器重放的时间
- `payload_snapshot`：重放所需业务快照
- `context_snapshot`：重放所需上下文快照

第一版人工治理还建议补这些字段：

- `assignee`：当前处理人
- `claimed_at`：接手时间
- `close_reason`：关闭原因
- `remark`：人工备注

---

## 10. 测试计划

### 10.1 单元测试

覆盖：

1. 错误分类
2. 重试策略计算
3. DLQ 写入
4. 自动重放状态流转
5. `uuid` 冲突幂等恢复
6. `conversation_key + session_seq` 冲突转人工
7. 4 类 replay handler 的输入输出

### 10.2 集成测试

覆盖：

1. `session_seq` Redis 临时失败后进入 DLQ，再自动重放成功
2. `mysql_persist` 超时后进入 DLQ，再自动重放成功
3. `websocket_dispatch` 失败后只重放分发动作
4. `group_member_query` 临时失败后自动重放成功
5. 永久故障直接进人工治理
6. 扫描器把 `pending` 抢占为 `retrying`

### 10.3 人工回归

重点验证：

1. Kafka 主消费不再被单条坏消息长期卡住
2. DLQ 入库信息完整
3. 自动重放不会重复分配错误 `session_seq`
4. 手动操作有审计记录
5. 用户态 `sending` 不会被 DLQ 长窗口拖成几十分钟

---

## 11. 交付顺序

建议按下面顺序开发：

### 第一批

1. DLQ 表结构
2. 主链路错误分类
3. 主链路有限重试
4. DLQ 写入

### 第二批

1. 自动重放调度器
2. 统一重放入口
3. 4 类自动重放 handler
4. 状态流转和操作日志

### 第三批

1. 管理后台后端接口
2. 前端列表页和详情页
3. 批量治理能力
4. 审计日志

---

## 12. 最终交付物

本次最终要落地这些内容：

1. 一套 MySQL 型 DLQ 数据结构
2. 一套主链路失败治理改造
3. 一套独立 goroutine 驱动的自动重放机制
4. 一套统一重放入口和 4 类重放 handler
5. 一套人工治理后台
6. 一套测试和上线校验方案

一句话总结：

这次开发的目标不是把所有失败都继续堵在 Kafka 主链路里重试，而是把失败改造成“主链路短重试、DLQ 可停放、自动可重放、人工可治理”的标准化能力。
