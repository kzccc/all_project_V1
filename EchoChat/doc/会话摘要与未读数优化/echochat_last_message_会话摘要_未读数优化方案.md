# echochat last message / 会话摘要 / 未读数优化方案

## 目标

这份文档只讨论 `echochat` 里和“会话概览层”有关的三件事：

1. `last message`
2. 会话摘要
3. 未读数模型

这里不讨论：

- Kafka consumer 并发
- MySQL 批量落库
- 在线路由表

重点是回答：

1. 当前 `echochat` 到底已经实现了什么
2. 为什么我说它是“有字段 / 半套结构 / 基本没有”
3. 应该怎样优化成一套完整的“会话概览层”

## 三个概念先讲清楚

## 1. `last message`

`last message` 指的是：

- 一个会话最后一条消息的摘要信息

最常见的字段包括：

- `last_message_text`
- `last_message_type`
- `last_message_time`
- `last_sender_id`

它不是完整消息列表，而是“聊天列表页一行里显示的最后一条消息”。

## 2. 会话摘要

会话摘要是比 `last message` 更完整的一层。

它通常包含：

- 会话对象名称
- 头像
- 最后一条消息内容
- 最后一条消息时间
- 未读数
- 置顶/免打扰/草稿等列表页状态

也就是说：

- `last message` 是会话摘要的一部分
- 会话摘要是“聊天列表页一整行的数据”

## 3. 未读数模型

未读数模型指的是：

- 系统如何记录“某个用户在某个会话里还有多少条未读消息”

比如：

- 用户 `U1` 在和 `U2` 的会话里未读 3 条
- 用户 `U1` 在群 `G1` 里未读 15 条

这通常需要独立维护，而不是每次临时现算。

## 当前 echochat 的现状

## 一、`last message`：有字段，但没有形成真正实现

### 现状

`session` 表已经有：

- `LastMessage`
- `LastMessageAt`

相关代码：

- [internal/model/session.go](/workspace/czk/Personal/KKK/internal/model/session.go:25)

这说明从数据模型角度，系统已经预留了“最新消息摘要”的字段。

### 问题

当前主链路里我没有看到：

- 每来一条新消息，就稳定更新 `session.last_message`
- 同步更新 `session.last_message_at`

也就是说：

- 结构在
- 维护逻辑不在主链路里

这会导致两个现实问题：

1. 即使表字段存在，也可能长期不是最新值
2. 会话列表无法把它当成可靠数据源

### 结论

所以我说：

- `last message`：有字段，但没有形成有效实现

## 二、会话摘要：有半套结构，但没有形成“轻量概览层”

### 现状

你现在已经有：

1. `session` 表
2. `session_list_{ownerId}` Redis 缓存
3. `group_session_list_{ownerId}` Redis 缓存

相关代码：

- [internal/service/gorm/session_service.go](/workspace/czk/Personal/KKK/internal/service/gorm/session_service.go:165)
- [internal/service/gorm/session_service.go](/workspace/czk/Personal/KKK/internal/service/gorm/session_service.go:214)

这些缓存当前主要承载：

- `sessionId`
- `avatar`
- `userId/groupId`
- `username/groupName`

### 问题

当前这些结构没有把下面几项真正整合进去：

- `last_message`
- `last_message_time`
- `unread_count`
- 会话排序依据

结果就是：

1. 你有“会话列表”
2. 但没有“会话概览层”

它更像是：

- 会话基本信息缓存

而不是：

- 会话摘要缓存

### 额外问题

当前 `session_list_xxx` 缓存是查列表时临时构建的，而不是消息主链路增量更新出来的。

这意味着：

- 列表数据更偏“读时拼装”
- 而不是“写时维护”

这会让会话列表随着产品需求增加越来越重。

### 结论

所以我说：

- 会话摘要：有半套结构，但没有形成完整可依赖的轻量概览层

## 三、未读数模型：当前基本没有

### 现状

这轮代码检索里，我没有看到当前聊天主链路存在清晰、独立的：

- Redis 未读数 key
- DB 会话级未读数字段
- 消息发送后递增、已读后归零或扣减的完整机制

至少没有看到像 `go-chat` 那种明确的：

- `UnreadStorage.Incr`
- `UnreadStorage.PipeIncr`

这样的模型化实现。

### 问题

没有独立未读数模型，后面通常会遇到这些问题：

1. 聊天列表上的红点很难高效展示
2. 未读数要么现算，要么逻辑分散在多个地方
3. 已读逻辑很难和消息顺序、会话列表、推送通知统一

### 结论

所以我说：

- 未读数模型：当前基本没有

## 为什么这三层值得单独建设

因为它们服务的是“会话列表页”，不是“消息详情页”。

你当前消息详情层已经比较完整：

- MySQL `message`
- `conversation_key`
- `session_seq`
- Redis `message_list_xxx`
- Redis `group_messagelist_xxx`

这些适合服务：

- 聊天详情页
- 消息历史查询
- 重建完整消息流

但它们不适合高效服务：

- 最近会话列表
- 最后一条消息摘要
- 未读红点

所以需要单独建设“会话概览层”。

## 当前方案的主要缺点

## 问题一：你在主链路里维护的是完整消息列表缓存，而不是轻量摘要缓存

当前单聊和群聊消费里，都有“如果消息列表缓存存在，就读出来、反序列化、append、再写回”的逻辑。

相关代码：

- [internal/service/chat/kafka_consumed_decode.go](/workspace/czk/Personal/KKK/internal/service/chat/kafka_consumed_decode.go:197)
- [internal/service/chat/kafka_server.go](/workspace/czk/Personal/KKK/internal/service/chat/kafka_server.go:609)

这类缓存适合详情页，不适合列表页。

问题在于：

- 它太重
- 它不是会话摘要
- 它维护成本高

## 问题二：会话列表页缺少稳定的增量维护数据源

如果没有：

- `last_message`
- `last_message_time`
- `unread_count`

那么会话列表就只能依赖：

- 查 `session`
- 查消息表
- 查缓存
- 再拼装

这会随着会话量增大逐渐变重。

## 问题三：未读逻辑未来会成为系统复杂度放大器

未读数如果不单独建模，后面涉及：

- 红点
- 已读回执
- 清零
- 多端同步
- 会话排序

这些功能都会没有统一落点。

早晚会造成：

- 逻辑分散
- 数据不一致
- 维护成本高

## 适合当前 echochat 的优化方向

## 方向一：把 `session.last_message` / `last_message_at` 真正维护起来

这是最小增量且最自然的一步。

### 建议

在消息主链路完成：

1. 持久化成功
2. websocket 分发成功或至少持久化成功

之后，异步更新对应会话的：

- `last_message`
- `last_message_at`

### 维护原则

不要把完整原消息塞进去，而是只存：

- 列表页可展示的摘要文本

例如：

- 文本：直接截断正文
- 图片：`[图片]`
- 文件：`[文件]`
- 语音：`[语音]`
- 视频：`[视频]`

### 效果

这样 `session` 表就真正变成会话摘要的 DB 基础层。

## 方向二：单独建设 `conversation_summary` 轻量缓存层

不要让 `message_list_xxx` 承担会话摘要角色。

### 建议缓存内容

每个用户每个会话维护一条轻量摘要，例如：

```json
{
  "conversation_id": "U1_U2",
  "peer_id": "U2",
  "peer_name": "张三",
  "peer_avatar": "/static/a.png",
  "last_message_text": "今晚开会",
  "last_message_type": "text",
  "last_message_time": "2026-05-07 14:32:10",
  "unread_count": 3
}
```

### 适合的 key 组织

可以按用户维度组织：

- `echochat:conversation_summary:{userID}:{conversationID}`

或按用户 hash：

- `echochat:conversation_summary:{userID}`

field 为 `conversationID`

### 效果

这样会话列表页不必依赖完整消息列表缓存。

## 方向三：补齐未读数模型

这一层建议尽量独立，不要和 `session_list` 混着算。

### 最小方案

先做 Redis 版：

- `echochat:unread:{userID}:{conversationID}` -> int

消息发送成功后：

- 发送者不加
- 接收者对应会话 `INCR`

用户进入会话或已读时：

- `DEL` 或 `SET 0`

### 群聊

群聊未读数按“成员-群会话”维度维护：

- `echochat:unread:{userID}:{groupID}`

### 效果

这样你后续：

- 红点展示
- 会话列表排序
- 已读清零

都会有稳定基础。

## 方向四：主链路不再同步维护完整消息列表缓存，改为“轻摘要优先”

当前主链路里最重的一段非关键后处理，就是同步更新：

- `message_list_xxx`
- `group_messagelist_xxx`

更合理的做法是：

### 主链路保留

1. 持久化
2. 推送
3. 轻摘要更新
4. 未读数更新

### 完整列表缓存改为

1. 缓存失效
2. 或异步 append
3. 或仅在详情页查询时构建

### 效果

把“会话概览层”和“消息详情层”彻底分开。

## 一个适合当前 echochat 的分阶段方案

## 第一阶段：把 `last message` 跑起来

目标：

- 让 `session` 表里的 `last_message` / `last_message_at` 成为可靠数据

动作：

1. 增加消息类型到摘要文本的转换函数
2. 在消息成功持久化后异步更新 `session`
3. 更新会话列表缓存失效策略

## 第二阶段：补轻量会话摘要缓存

目标：

- 会话列表页不依赖完整消息列表缓存

动作：

1. 建 `conversation_summary`
2. 每条消息成功后只更新摘要
3. 会话列表直接读摘要层

## 第三阶段：补未读数模型

目标：

- 红点、清零、列表排序有统一基础

动作：

1. 建 `unread` key 模型
2. 接收侧递增
3. 进入会话时清零

## 第四阶段：瘦掉完整消息列表缓存的主链路更新

目标：

- 把重缓存逻辑从 consumer 热路径移走

动作：

1. `message_list_xxx` 主链路只失效不重写
2. 列表页只看摘要层
3. 详情页才看完整消息层

## 不建议现在做的事

### 1. 不建议直接让 `session_list_xxx` 继续承担所有会话摘要职责

因为它现在更像“会话基础信息缓存”，不是稳定的概览层。

### 2. 不建议用完整消息列表缓存去推导未读数

那样成本太高，而且会和消息详情层耦合过深。

### 3. 不建议先做复杂已读回执，再补未读数基础

顺序应该反过来：

先有稳定的未读数模型，再做复杂已读能力。

## 最终建议

对你当前 `echochat` 来说，最正确的方向不是继续强化“完整消息列表缓存”，而是尽快补一层真正轻量的“会话概览层”。

这层应该由三部分组成：

1. `session` 表里的 `last_message / last_message_at`
2. Redis `conversation_summary`
3. Redis `unread`

这样以后：

- 聊天详情页看消息层
- 会话列表页看概览层

职责会非常清楚。

## 末尾压缩版：每点一句话

1. 当前 `last message` 只有表字段没有稳定维护逻辑；在消息成功后异步更新 `session.last_message` 和 `last_message_at`。
2. 当前会话列表缓存只有基本信息，没有形成真正会话摘要；单独建设 `conversation_summary` 轻量概览层。
3. 当前完整消息列表缓存承担了过多列表职责；把列表页和详情页的数据层彻底拆开。
4. 当前未读数模型基本缺失，后续红点和已读会越来越难做；按“用户-会话”维度独立维护 `unread`。
5. 当前主链路里同步更新完整消息列表缓存成本过高；主链路只更新轻摘要和未读数，完整列表缓存改为失效或异步维护。
6. 当前 `session_list_xxx` 更像会话基础信息缓存，不像完整概览层；让它逐步退化为会话入口，摘要信息交给 `conversation_summary`。
7. 当前最值得先做的是把 `last message` 跑起来，再补会话摘要和未读数；按“字段落地 -> 摘要缓存 -> 未读模型 -> 热路径瘦身”分阶段推进。
