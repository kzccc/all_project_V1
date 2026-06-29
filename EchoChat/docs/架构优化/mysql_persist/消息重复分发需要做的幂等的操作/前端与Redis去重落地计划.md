# 消息重复分发去重落地计划（前端 + Redis）

## 1. 背景与问题

在 Kafka 链路存在重复消费/重放时（例如 1062 duplicate key 触发 `duplicate_replayed`），同一条消息可能再次走 websocket 分发与缓存更新路径，带来：

- 前端可能展示重复消息
- Redis `message_list_*` / `group_messagelist_*` 可能被重复 append，缓存污染

目前 `status=sent` 已经具备幂等保护，但前端展示与 Redis 列表不具备稳定去重依据。

## 2. 目标

- 对同一条消息（同一 `message_id/uuid`），做到：
  - 前端展示层幂等：不重复展示
  - Redis 缓存层幂等：不重复写入列表
- 在不改变核心吞吐测试链路口径的前提下，降低重复分发的副作用。

## 3. 现状确认

### 3.1 前端去重困难点

后端写回前端的消息体目前不带 `message_id/uuid`，前端无法稳定去重。

最小闭环：

- 把 `message.Uuid`（或 `MessageId`）加进单聊/群聊/AV 的响应体，让前端用 `message_id` 做集合去重。

退而求其次：

- 只能用 `session_seq` 去重，但遇到重放/乱序时不如 `uuid` 稳。

### 3.2 Redis 缓存写入现状

当前 `message_list_*` / `group_messagelist_*` 的增量维护是典型的：

- `get -> append -> set`

重复分发会把同一条消息重复写进列表，且目前没有去重逻辑。

最小改法：

- 写回前做一次去重：按 `uuid`（最好）或 `session_seq`（次选）过滤，确保列表内唯一。

更稳的做法（后续可选）：

- 缓存结构从“纯数组”改成“索引集合 + 有序列表”，但成本更高。

## 4. 方案设计（最小闭环优先）

### 4.1 协议层：统一补充 `message_id`

改动点：

- 单聊响应结构增加字段：`MessageId string \`json:"message_id"\``
- 群聊响应结构增加字段：`MessageId string \`json:"message_id"\``
- AV 响应结构增加字段：`MessageId string \`json:"message_id"\``

写回消息时赋值：

- `MessageId = message.Uuid`（channel 模式）
- `MessageId = message.Uuid`（kafka 模式，消费得到的 message.Uuid）

前端去重规则（建议）：

- 每个会话维度维护一个 `seen_message_id` 集合
- 若 `message_id` 已存在，则丢弃该条展示事件

容量控制建议：

- `seen_message_id` 只保留最近 N 条（例如 2000）或按时间窗口（例如 10 分钟）淘汰，避免无限增长。

### 4.2 Redis 层：列表写入前做去重

改动点（最小实现）：

- 在 `append` 新消息前，先判断该列表内是否已有相同 `message_id`（首选）或 `session_seq`（次选）。

说明：

- 若采用 `message_id` 去重，必须先落地 4.1，让缓存结构里也能存到该字段。
- 若短期不改协议，也可以用 `session_seq` 去重，但要接受它在重放/乱序下并不完美。

### 4.3 `status=sent` 回刷：保持现状即可

现状已经有两道幂等：

- websocket 写成功侧：同一条 `MessageBack` 通过 CAS 保证只 enqueue 一次
- DB 更新侧：`UPDATE ... WHERE status <> sent` 自带幂等

因此本次改动不以 `status=sent` 为目标，避免扩大范围。

## 5. 实施步骤

### 5.1 后端改动

1. 修改三类响应 DTO，增加 `message_id` 字段。
2. 修改构造响应体的地方，把 `message.Uuid` 填进 `message_id`。
3. 修改 Redis 缓存写回逻辑，在 `append` 前做去重。
4. 补充最小化的观测：
   - 统计 `duplicate_skipped` / `duplicate_replayed` 的次数（已有日志可先用）
   - 统计 Redis 去重命中次数（建议新增 metric 或日志）

### 5.2 前端改动

1. 接收 `message_id` 字段并落到前端消息模型。
2. 在会话维度做 `message_id` 集合去重。
3. 给去重集合加容量控制（N 条或时间窗）。

## 6. 验收方式

功能验收：

- 人工制造重复分发（或在压测时引入重复消费场景），前端不出现重复消息展示。
- Redis `message_list_*` / `group_messagelist_*` 拉取结果不出现重复消息。

指标验收（建议）：

- `duplicate_replayed` 发生时，前端“重复展示”计数为 0。
- Redis 去重命中率在重复场景下明显 > 0。
- 单聊/群聊吞吐与 p95 不因去重逻辑出现明显恶化（避免在热路径上做 O(n) 扫描过大）。

## 7. 风险与回滚

风险：

- Redis 去重如果每次都对全列表扫一遍，列表大时会引入额外 CPU 开销。
- 前端去重集合如果不做容量控制，可能导致内存增长。

回滚：

- 后端新增字段对旧前端通常是向后兼容（忽略字段即可）。
- Redis 去重逻辑可加开关（建议），出现性能回退时可快速关闭。

