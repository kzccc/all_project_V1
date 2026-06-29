# EchoChat DLQ 架构设计

## 1. 设计目标

DLQ 在这里不是“把消息丢掉”，而是把主链路暂时处理不了、但又不能丢的消息可靠停放下来。

这套设计要同时满足 4 件事：

1. 主链路不被单条失败消息长期卡住
2. 临时故障可以自动重放
3. 永久故障可以人工治理
4. 全链路有审计、有状态、有恢复手段

---

## 2. 设计原则

### 2.1 主链路先有限重试，再进入 DLQ

临时故障不会立刻进入 DLQ，而是先做主链路有限重试。

建议策略：

- 重试 3 次
- 退避时间：`200ms -> 1s -> 5s`

重试仍失败后，再入 DLQ。

### 2.2 DLQ 只分两类处理

DLQ 中的消息最终只分两类：

1. 自动重放
2. 人工处理

### 2.3 自动重放和主链路重试不是一回事

主链路重试属于正常消费流程内的快速重试。  
DLQ 自动重放属于失败治理流程内的延迟重放。

### 2.4 用户态窗口和 DLQ 治理窗口必须分离

用户态只保留：

1. 发送中
2. 已发送
3. 失败

边界如下：

- `session_seq` 和 `mysql_persist` 失败时，在短窗口内可以显示“发送中”
- 超过用户态窗口后，必须显示“失败”
- `websocket_dispatch` 和 `group_member_query` 若消息已落库成功，则对发送方仍显示“已发送”
- DLQ 可以继续治理，但不能让用户看到几十分钟的“发送中”

---

## 3. 失败分类与处理结果

### 3.1 自动重放

自动重放的 4 类：

1. 会话顺序号生成阶段的临时失败
2. MySQL 持久化阶段的临时失败
3. WebSocket 消息分发阶段的临时失败
4. 群成员查询阶段的临时失败

### 3.2 人工处理

不自动重放的典型情况：

1. 消息反序列化失败
2. 消息路由非法
3. 协议类型不支持
4. 音视频扩展字段损坏
5. 接收方标识非法
6. `conversation_key + session_seq` 唯一键冲突
7. 群成员 JSON 脏数据
8. `conversation bucket worker panic`

### 3.3 不进入 DLQ

1. `uuid` 唯一键冲突
2. `redis_read`
3. `redis_write`

---

## 4. 总体架构

### 4.1 主流程

主链路流程调整为：

1. Kafka 消费消息
2. 主链路按阶段处理
3. 失败时按规则判断临时故障或永久故障
4. 临时故障先有限重试
5. 重试耗尽后写入 DLQ
6. 永久故障直接写入 DLQ
7. DLQ 写入成功后，主链路提交该 Kafka 消息，避免分区长期阻塞

### 4.2 DLQ 后续流程

DLQ 消息后续由两条链路处理：

1. 自动重放调度器
2. 人工治理后台

### 4.3 架构分层

建议新增这些模块：

- `internal/model/dlq_message.go`
- `internal/model/dlq_operation_log.go`
- `internal/service/dlq/dlq_writer.go`
- `internal/service/dlq/dlq_service.go`
- `internal/service/dlq/dlq_replay_scheduler.go`
- `internal/service/dlq/dlq_replay_dispatcher.go`
- `internal/service/dlq/dlq_replay_handlers.go`
- `internal/service/dlq/dlq_admin_service.go`
- `internal/controller/admin/dlq_controller.go`

---

## 5. MySQL 表结构设计

### 5.1 主表：`dlq_message`

这张表保存每条失败消息的当前治理状态。

建议字段如下：

| 字段名 | 类型 | 说明 |
| --- | --- | --- |
| `id` | `bigint` | 主键 |
| `message_id` | `varchar(64)` | 业务消息唯一 ID，通常是 `uuid` |
| `conversation_key` | `varchar(128)` | 会话标识 |
| `session_seq` | `bigint` | 会话内顺序号 |
| `raw_payload` | `longtext` | Kafka 原始消息体 |
| `payload_snapshot` | `json` | 当前可重放的业务快照 |
| `context_snapshot` | `json` | 当前可重放的上下文快照 |
| `topic` | `varchar(128)` | 来源 topic |
| `partition_id` | `int` | Kafka 分区 |
| `offset_id` | `bigint` | Kafka offset |
| `stage` | `varchar(64)` | 失败阶段 |
| `error_code` | `varchar(128)` | 结构化错误码 |
| `last_error` | `text` | 最后错误信息 |
| `failure_type` | `varchar(16)` | `temporary` / `permanent` |
| `handle_type` | `varchar(16)` | `auto` / `manual` |
| `status` | `varchar(16)` | 自动治理状态：`pending` / `retrying` / `done` / `manual` |
| `manual_status` | `varchar(16)` | 人工治理状态：`open` / `in_progress` / `closed` |
| `close_reason` | `varchar(32)` | 人工关闭原因：`discarded` / `externally_fixed` / `expected` / `merged_into_incident` |
| `attempt_count` | `int` | 已自动治理次数 |
| `max_attempt_count` | `int` | 最大自动治理次数 |
| `next_retry_at` | `datetime` | 下一次允许自动重放的时间 |
| `assignee` | `varchar(64)` | 当前处理人 |
| `claimed_at` | `datetime` | 人工接手时间 |
| `remark` | `text` | 人工治理备注 |
| `first_failed_at` | `datetime` | 第一次失败时间 |
| `last_failed_at` | `datetime` | 最近一次失败时间 |
| `resolved_at` | `datetime` | 完成处理时间 |
| `created_at` | `datetime` | 创建时间 |
| `updated_at` | `datetime` | 更新时间 |

### 5.2 核心字段语义

#### `status`

表示这条 DLQ 记录当前的自动治理状态：

- `pending`：等待下一次重放
- `retrying`：已被某个 worker 抢占，正在处理中
- `done`：已经处理完成
- `manual`：自动治理停止，转人工

#### `manual_status`

表示这条记录在人工治理流程中的位置：

- `open`：已进入人工治理池，还没人接手
- `in_progress`：已有人接手，正在处理
- `closed`：人工治理已结束

#### `close_reason`

表示人工治理结束的原因：

- `discarded`：确认不再处理，直接放弃
- `externally_fixed`：根因已在外部修复，这条记录关闭归档
- `expected`：后来确认不是问题，或属于预期现象
- `merged_into_incident`：并入某个批量事故单，不再单独跟踪

#### `attempt_count`

表示这条 DLQ 记录已经自动重放了多少次。

注意：

- 它统计的是 DLQ 自动治理次数
- 不是 Kafka 主链路里的那几次快速重试

#### `max_attempt_count`

表示自动治理最多还能试几次。  
达到阈值后，不再自动重放，直接转 `manual`。

#### `next_retry_at`

表示这条记录“下一次最早什么时候允许被扫描器捞出来”。

举例：

- 第一次失败后设置成 `now + 10s`
- 扫描器只有在到这个时间点之后才会重放它

#### `pending + next_retry_at 到期`

意思是：

- 这条记录当前状态是 `pending`
- 并且 `next_retry_at <= now()`

也就是“这条消息现在轮到它可以被自动重放了”。

### 5.3 建议索引

1. `idx_dlq_status_next_retry`：`status, next_retry_at`
2. `idx_dlq_stage_status`：`stage, status`
3. `idx_dlq_message_id`：`message_id`
4. `idx_dlq_conversation_key`：`conversation_key`
5. `uniq_dlq_source`：`topic, partition_id, offset_id, stage`

### 5.4 操作日志表：`dlq_operation_log`

这张表保存自动动作和人工动作的审计记录。

建议字段如下：

| 字段名 | 类型 | 说明 |
| --- | --- | --- |
| `id` | `bigint` | 主键 |
| `dlq_id` | `bigint` | 对应 DLQ 主表 ID |
| `action` | `varchar(32)` | `create` / `auto_retry` / `manual_retry` / `mark_manual` / `discard` / `done` |
| `operator` | `varchar(64)` | 操作人，自动任务可记为 `system` |
| `remark` | `text` | 备注 |
| `before_status` | `varchar(16)` | 操作前状态 |
| `after_status` | `varchar(16)` | 操作后状态 |
| `created_at` | `datetime` | 创建时间 |

### 5.5 建议 DDL

```sql
CREATE TABLE dlq_message (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  message_id VARCHAR(64) NOT NULL,
  conversation_key VARCHAR(128) NOT NULL DEFAULT '',
  session_seq BIGINT NOT NULL DEFAULT 0,
  raw_payload LONGTEXT NOT NULL,
  payload_snapshot JSON NULL,
  context_snapshot JSON NULL,
  topic VARCHAR(128) NOT NULL,
  partition_id INT NOT NULL,
  offset_id BIGINT NOT NULL,
  stage VARCHAR(64) NOT NULL,
  error_code VARCHAR(128) NOT NULL,
  last_error TEXT NOT NULL,
  failure_type VARCHAR(16) NOT NULL,
  handle_type VARCHAR(16) NOT NULL,
  status VARCHAR(16) NOT NULL,
  manual_status VARCHAR(16) NOT NULL DEFAULT 'open',
  close_reason VARCHAR(32) NULL,
  attempt_count INT NOT NULL DEFAULT 0,
  max_attempt_count INT NOT NULL DEFAULT 0,
  next_retry_at DATETIME NULL,
  assignee VARCHAR(64) NOT NULL DEFAULT '',
  claimed_at DATETIME NULL,
  remark TEXT NULL,
  first_failed_at DATETIME NOT NULL,
  last_failed_at DATETIME NOT NULL,
  resolved_at DATETIME NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE KEY uniq_dlq_source (topic, partition_id, offset_id, stage),
  KEY idx_dlq_status_next_retry (status, next_retry_at),
  KEY idx_dlq_stage_status (stage, status),
  KEY idx_dlq_message_id (message_id),
  KEY idx_dlq_conversation_key (conversation_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE dlq_operation_log (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  dlq_id BIGINT NOT NULL,
  action VARCHAR(32) NOT NULL,
  operator VARCHAR(64) NOT NULL,
  remark TEXT NULL,
  before_status VARCHAR(16) NOT NULL,
  after_status VARCHAR(16) NOT NULL,
  created_at DATETIME NOT NULL,
  KEY idx_dlq_log_dlq_id (dlq_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

---

## 6. 为什么需要 `payload_snapshot`

只存 `raw_payload` 不够。

原因：

1. `session_seq` 失败时，重放可以从原始请求重新走
2. `mysql_persist` 失败时，不能重新分配新的 `session_seq`
3. `websocket_dispatch` 失败时，不应该再重复写 MySQL
4. `group_member_query` 失败时，应直接补群扇出，不应重新走完整主链路

因此需要补两个快照字段：

1. `payload_snapshot`
2. `context_snapshot`

建议约定：

- `session_seq` 失败：`payload_snapshot` 存解码后的请求体
- `mysql_persist` 失败：`payload_snapshot` 存构造完成的 `model.Message`
- `websocket_dispatch` 失败：`payload_snapshot` 存待分发消息体，`context_snapshot` 存接收方集合或发送方头像等上下文
- `group_member_query` 失败：`payload_snapshot` 存已持久化消息，`context_snapshot` 存群发相关上下文

---

## 7. 主链路改造设计

### 7.1 统一失败治理入口

主链路增加统一治理入口，职责如下：

1. 识别 `stage`
2. 识别 `error_code`
3. 判断临时故障还是永久故障
4. 判断是否自动重放
5. 决定是否写入 DLQ

建议抽象：

```go
type DLQDecision struct {
    FailureType string
    HandleType  string
    ShouldDLQ   bool
    ErrorCode   string
}
```

### 7.2 主链路有限重试

主链路内对临时故障做有限重试。

建议流程：

1. 初次失败
2. 判断是否临时故障
3. 若是，按配置做 3 次重试
4. 若成功，正常结束
5. 若失败，写入 DLQ
6. DLQ 写入成功后提交 Kafka 消息

### 7.3 各阶段接入点

#### `deserialize`

处理方式：

- 直接落 DLQ
- `failure_type=permanent`
- `handle_type=manual`
- `status=manual`

#### `route`

处理方式同上：

- 直接落 DLQ
- 人工治理

#### `session_seq`

处理方式：

- 主链路有限重试
- 失败后进入 DLQ
- `failure_type=temporary`
- `handle_type=auto`
- `status=pending`

#### `mysql_persist`

分 3 类：

1. 普通临时异常：自动重放
2. `conversation_key + session_seq` 冲突：人工处理
3. `uuid` 冲突：幂等成功，不进 DLQ

#### `websocket_dispatch`

处理方式：

- 若消息已持久化，则只补分发动作
- 进入自动重放
- 不改变消息主状态

#### `group_member_query`

分 2 类：

1. 数据库短时失败：自动重放
2. 群成员 JSON 脏数据：人工处理

#### `redis_read` / `redis_write`

处理方式：

- 不进 DLQ
- 继续走降级

#### `conversation bucket worker panic`

处理方式：

- 直接落 DLQ
- 直接告警
- 标记 `manual`

---

## 8. 自动重放架构

### 8.1 独立扫描器

自动重放不跑在 Kafka 主消费 goroutine 里，而是独立后台 goroutine。

第一版建议：

- 服务启动时起一个独立扫描 goroutine
- 每隔 `10s` 扫一次 `dlq_message`

如果后续量大，再扩展成：

- 一个调度 goroutine 扫表
- 多个 worker goroutine 并发执行重放

### 8.2 统一运作模型

运作方式是：

1. 定时扫描
2. 捞出到期记录
3. 抢占记录
4. 调统一重放入口
5. 回写状态

### 8.3 扫描条件

扫描器只捞这些记录：

1. `status = pending`
2. `handle_type = auto`
3. `next_retry_at <= now()`

也就是“当前允许自动重放的待处理记录”。

### 8.4 锁定成 `retrying` 的含义

“锁定成 `retrying`” 的意思不是处理成功，而是：

- 某个 worker 已经把这条记录拿走了
- 其他 worker 不能再重复处理同一条

建议实现：

- 事务内查询
- `FOR UPDATE SKIP LOCKED`
- 然后把记录更新成 `retrying`

### 8.5 重放失败后的处理

重放失败后：

1. `attempt_count + 1`
2. 重新计算 `next_retry_at`
3. 如果还没超限，状态改回 `pending`
4. 如果超限，状态改成 `manual`

### 8.6 默认参数

第一版建议：

- 扫描周期：`10s`
- 每批拉取：`100`
- worker 并发：`2~4`
- 最大自动重放次数：`5`

---

## 9. 统一重放入口与 4 类 handler

### 9.1 统一重放入口

不为每种错误写独立调度程序。  
采用“统一扫描器 + 统一重放入口 + 分阶段 handler”。

建议入口：

```go
func HandleDLQRecord(record *DLQMessage) ReplayResult
```

内部按 `stage` 分发：

- `handleSessionSeqReplay`
- `handleMysqlPersistReplay`
- `handleWebsocketDispatchReplay`
- `handleGroupMemberQueryReplay`

### 9.2 `handleSessionSeqReplay`

适用：

- `session_seq` 临时失败

步骤：

1. 从 `raw_payload` 或 `payload_snapshot` 还原原始请求
2. 重新执行顺序号生成
3. 重新构造 `model.Message`
4. 继续走后续持久化和分发

特点：

- 从比较前面的阶段重新走

### 9.3 `handleMysqlPersistReplay`

适用：

- `mysql_persist` 普通临时异常

步骤：

1. 从 `payload_snapshot` 还原已构造好的消息
2. 保持原 `session_seq`
3. 重新执行 MySQL 持久化
4. 成功后继续后续分发

关键点：

- 不能重新申请新的 `session_seq`

### 9.4 `handleWebsocketDispatchReplay`

适用：

- `websocket_dispatch` 临时失败

步骤：

1. 还原待分发消息
2. 还原分发上下文
3. 只补分发动作

关键点：

- 不碰 MySQL
- 不重新生成 `session_seq`

### 9.5 `handleGroupMemberQueryReplay`

适用：

- `group_member_query` 临时失败

步骤：

1. 还原已落库消息
2. 重新查群成员
3. 重新做群扇出

关键点：

- 不重新走消息主存储

### 9.6 结果回写

统一入口返回结果后：

- 成功：`status=done`
- 失败可重试：`attempt_count+1`，`status=pending`，写 `next_retry_at`
- 失败不可重试：`status=manual`

---

## 10. 状态机设计

`dlq_message.status` 只描述自动治理状态，建议只用这 4 个值：

1. `pending`
2. `retrying`
3. `done`
4. `manual`

状态流转：

1. 自动重放类入库后进入 `pending`
2. 扫描器取到任务后改成 `retrying`
3. 重放成功改成 `done`
4. 重放失败但还能再试，回到 `pending`
5. 重放失败且达到上限，改成 `manual`

人工治理状态单独使用：

1. `open`
2. `in_progress`
3. `closed`

---

## 11. 人工治理后台设计

### 11.1 使用角色

建议只开放给：

1. 研发
2. 值班同学
3. 运维/SRE

### 11.2 后端接口

#### 查询类

1. `GET /admin/dlq/messages`
2. `GET /admin/dlq/messages/:id`
3. `GET /admin/dlq/messages/:id/logs`
4. `GET /admin/dlq/stats`

#### 操作类

1. `POST /admin/dlq/messages/:id/claim`
2. `POST /admin/dlq/messages/:id/reopen`
3. `POST /admin/dlq/messages/:id/close`
4. `POST /admin/dlq/messages/:id/remark`

### 11.3 前端页面

#### 列表页

字段展示：

1. 消息 ID
2. 会话标识
3. `stage`
4. `error_code`
5. 自动治理状态 `status`
6. 人工治理状态 `manual_status`
7. `attempt_count`
8. 最近失败时间

筛选能力：

1. 阶段
2. 错误码
3. 自动治理状态
4. 人工治理状态
5. 消息 ID
6. 会话标识
7. 时间范围

#### 详情页

展示：

1. 原始 payload
2. 业务快照
3. 上下文快照
4. 最后错误
5. Kafka 来源信息
6. 操作日志

#### 操作面板

支持：

1. 标记 `open`
2. 标记 `in_progress`
3. 标记 `closed`
4. 选择关闭原因：`discarded`、`externally_fixed`、`expected`、`merged_into_incident`
5. 填写备注
6. 关联事故单或 bug 单

### 11.4 审计要求

人工治理的每个动作都要写 `dlq_operation_log`。

至少记录：

1. 谁操作
2. 何时操作
3. 操作前状态
4. 操作后状态
5. 备注

人工治理设计的核心不是“人工补发消息”，而是：

1. 看清失败信息
2. 明确是否已有人接手
3. 记录关闭原因
4. 形成故障治理闭环

---

## 12. 关键实现细节

### 12.1 `mysql_persist` 冲突处理

你的项目当前已经具备一部分基础能力：

- 批量插入失败时识别 `1062`
- 拆成单条插入
- `conversation_key + session_seq` 冲突判成确定性错误
- `uuid` 冲突按幂等恢复

代码位置：

- [kafka_message_support.go](/workspace/czk/Personal/EchoChat/internal/service/chat/kafka_message_support.go:416)
- [kafka_message_support.go](/workspace/czk/Personal/EchoChat/internal/service/chat/kafka_message_support.go:457)

本次要补的，是把这类结果进一步接入 DLQ 体系：

1. 普通 MySQL 临时异常：自动重放
2. `session_seq` 冲突：人工处理
3. `uuid` 冲突：成功返回，不入 DLQ

### 12.2 `group_member_query` 分类修正

当前群成员 JSON 解析失败仍按重试错误返回。  
这类应改成永久故障，直接进人工治理，不应自动反复重试。

### 12.3 `conversation bucket worker panic`

这类不能只打日志。  
应补一条 DLQ 故障记录，并触发告警。

### 12.4 主消息成立性错误与下游补偿错误的窗口差异

建议区分：

1. `session_seq`、`mysql_persist`
   这是主消息成立性错误，自动重放窗口要短
2. `websocket_dispatch`、`group_member_query`
   这是下游补偿错误，自动重放窗口可以更长

---

## 13. 上线顺序建议

### 第一步

只上线：

1. DLQ 表
2. 主链路有限重试
3. 主链路写 DLQ

目的：

- 先把“无限重试阻塞”问题解决掉

### 第二步

上线：

1. 自动重放扫描器
2. 统一重放入口
3. 4 类自动重放 handler
4. 状态流转和操作日志

### 第三步

上线：

1. 人工治理后台
2. 审计日志
3. 批量处理能力

---

## 14. 结论

这套 DLQ 架构的核心不是“再搞一层重试”，而是把你当前主链路中的失败从“卡住消费”改造成“分阶段治理”。

最终形成的能力是：

1. 主链路有限重试
2. MySQL 型 DLQ 可靠停放
3. 独立 goroutine 驱动的自动重放扫描器
4. 统一重放入口和 4 类 handler
5. 人工治理后台闭环处理

这样主链路负责“尽快处理”，DLQ 负责“后续治理”，两者职责才会清晰。
