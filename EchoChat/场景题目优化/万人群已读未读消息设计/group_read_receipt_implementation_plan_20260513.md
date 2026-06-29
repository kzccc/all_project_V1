# EchoChat 万人群已读未读消息设计实施报告

日期：2026-05-13

## 1. 报告目标

这份报告回答四个问题：

1. 当前万人群已读未读消息设计的主流做法是什么。
2. 为什么普通群读回执方案不能直接扩到万人群。
3. EchoChat 当前代码结构下，应该落在哪一版设计上。
4. 如果要实施，建议按什么顺序落地。

---

## 2. 先说最终判断

如果目标是“万人群消息已读未读”：

- 不应该做“每条消息维护全量已读成员名单”的重方案。
- 应该做“消息顺序号 + 成员读游标 + 会话未读数 + 按需查询已读详情”的轻重分层方案。
- 一旦群规模继续上升到热点群/超大社群，应该切到“社群/话题/频道”模型，而不是继续把普通群聊人数调大。

对 EchoChat 来说，最适合的落点不是：

- 微信式强成员已读回执

而是：

- 类 Slack / Discord / Stream / 腾讯云社群的聚合式读状态模型

也就是：

1. 主链路只维护消息顺序和会话摘要
2. 成员侧只维护 `last_read_seq`
3. 列表页和角标只看 `unread_count`
4. 已读明细只在小群或按需查询时提供

---

## 3. 外部方案调研结论

## 3.1 Slack

Slack 官方公开资料的核心思路不是把消息逐用户硬推到底，而是：

- `channel server`
- `gateway / edge fanout`
- `pub/sub`
- `edge cache`

这说明当群或频道规模上来后，消息分发和状态维护必须分层，不能在单个逻辑点上做全量操作。

参考：

- https://slack.engineering/real-time-messaging/
- https://slack.engineering/flannel-an-application-level-edge-cache-to-make-slack-scale/

## 3.2 Discord

Discord 官方在超大 server 的工程实践里，核心也是：

- 房间/频道化
- relay / fanout 拆层
- 只让活跃视图承受完整消息流

它的关键启发是：

- 超大群已经不是“小群成员数调大”
- 必须引入 server/channel/topic 这样的上层产品结构

参考：

- https://discord.com/blog/maxjourney-pushing-discords-limits-with-a-million-plus-online-users-in-a-single-server

## 3.3 Ably

Ably 官方文档对高规模实时系统的描述很清楚：

- 把 fanout 拆成多层
- 前端连接层不直接承担所有上游广播压力

这对应到读状态上，结论也是：

- 读回执必须聚合，不可能对超大群做逐用户强同步

参考：

- https://ably.com/docs/platform/architecture/platform-scalability

## 3.4 腾讯云 IM

腾讯云 IM 的公开资料里有两个非常关键的信号：

1. 社群（Community）是超大群形态，成员上限可到 10 万。
2. 群消息已读回执能力适用群类型有限，并且“群内最大人数为 200 人”。

这说明行业里的成熟产品本身就把：

- 超大群
- 强已读回执

明确拆开了。

参考：

- 产品页：https://cloud.tencent.com/product/im
- 群功能配置：https://cloud.tencent.com/document/product/269/38656
- 群消息已读回执：https://cloud.tencent.com/document/product/269/107478

## 3.5 Sendbird / Stream

这两类云 IM 产品提供了一个很现实的中间态：

- read receipt
- unread count
- read state

它们不是要求每个场景都显示完整已读列表，而是把：

- 会话未读数
- 消息已送达/已读
- 成员读状态

拆成独立能力。

参考：

- Sendbird group channel / supergroup：
  https://docs.sendbird.com/docs/chat/platform-api/v3/channel/creating-a-channel/create-a-group-channel
- Stream read status：
  https://getstream.io/chat/docs/react/message-delivery-and-read-status/

---

## 4. 为什么万人群不能用普通群读回执

## 4.1 写放大

如果一个 1 万人群里每条消息都更新一份成员已读状态：

- 每发一条消息，可能对应 9999 个潜在未读状态变更
- 每次阅读，又可能产生高频成员级回执写入

这会把系统从“消息系统”变成“消息 + 状态写爆系统”。

## 4.2 查询放大

如果前端希望查看“谁已读、谁未读”：

- 一条消息可能需要查询 1 万成员
- 还要结合在线态、退群态、拉黑态、禁言态

这在热点群里会非常重。

## 4.3 广播放大

如果每个成员读了一条消息，都要通知其他人：

- 已读状态本身会变成一类风暴消息
- 甚至比正文消息还密集

## 4.4 存储放大

如果按消息维度存成员已读名单：

- `消息数 * 成员数`

增长极快，存储和索引都会失控。

## 4.5 产品意义下降

在万人群里，大多数用户并不会真的关心：

- 具体哪 3762 个人已读

真正有价值的通常只有：

- 我有没有未读
- 当前未读多少
- 这条公告大概多少人看了
- 管理员能不能查少量明细

所以产品价值本身也支持“聚合化设计”。

---

## 5. EchoChat 当前代码现状分析

## 5.1 已有能力

EchoChat 现在已经有三块很重要的基础：

1. `message.session_seq`
2. `message.conversation_key`
3. `session.last_message / last_message_at`

这意味着：

- 你已经有“统一聊天流顺序”
- 也有“会话级摘要字段”

这正是做未读/已读模型最重要的底座。

相关代码：

- [message.go](/workspace/czk/Personal/KKK/internal/model/message.go)
- [message_sequence.go](/workspace/czk/Personal/KKK/internal/service/chat/message_sequence.go)
- [session.go](/workspace/czk/Personal/KKK/internal/model/session.go)

## 5.2 当前缺失

当前真正缺的是三层：

1. 成员读游标模型
2. 会话未读数模型
3. 大群读状态聚合模型

具体表现为：

- `session` 没有 `unread_count`
- 没有 `conversation_member_state`
- 消息响应体没有 `read_count`、`is_read`、`last_read_seq`
- 群会话列表响应体没有未读数字段

相关结构：

- [get_group_messagelist_respond.go](/workspace/czk/Personal/KKK/internal/dto/respond/get_group_messagelist_respond.go)
- [get_message_list_respond.go](/workspace/czk/Personal/KKK/internal/dto/respond/get_message_list_respond.go)
- [group_sessionlist_respond.go](/workspace/czk/Personal/KKK/internal/dto/respond/group_sessionlist_respond.go)
- [user_sessionlist_respond.go](/workspace/czk/Personal/KKK/internal/dto/respond/user_sessionlist_respond.go)

## 5.3 当前群结构不适合承载读状态

`group_info.Members` 现在是 JSON。

这适合：

- 创建群
- 取群详情
- 简单遍历成员

但不适合：

- 高频更新成员状态
- 做按成员维度的读游标维护
- 做万人级已读查询分页

相关代码：

- [group_info.go](/workspace/czk/Personal/KKK/internal/model/group_info.go)
- [kafka_server.go](/workspace/czk/Personal/KKK/internal/service/chat/kafka_server.go)

## 5.4 当前消息分发模型不适合万人群

现在群消息处理大致是：

1. 查 `group_info`
2. 解析 `Members`
3. 遍历成员
4. 找在线连接
5. 逐个写 `SendBack`

这套链路在百人群还可以，在万人群会非常重。

特别是：

- 成员查询重
- 遍历重
- 本地 fanout 重
- 读状态维护无承接点

---

## 6. 推荐目标方案

## 6.1 核心原则

目标方案遵守四个原则：

1. 只在主链路维护必要状态
2. 只在成员侧维护读游标，不维护逐消息全量已读名单
3. 列表页和角标优先走会话摘要
4. 已读明细按需查，且仅在小群或公告场景开放

## 6.2 数据模型建议

### 表 1：会话摘要表增强

如果延续现在的 `session`，建议补：

- `conversation_key`
- `last_seq`
- `last_sender_id`
- `unread_count`
- `mute`
- `pin`

说明：

- 单聊和群聊都可以统一用 `conversation_key`
- `unread_count` 是用户视角字段，因此更理想的是放在“用户-会话状态表”

### 表 2：用户-会话状态表

建议新增：

`conversation_member_state`

字段建议：

| 字段 | 说明 |
| --- | --- |
| `conversation_key` | 统一会话键 |
| `user_id` | 成员 ID |
| `last_read_seq` | 最后已读到的 seq |
| `last_read_at` | 最后已读时间 |
| `unread_count_cache` | 缓存未读数，可选 |
| `mention_seq` | @我 游标，可选 |
| `role` | 管理员/普通成员，可选 |
| `updated_at` | 更新时间 |

这个表是后续万人群读状态的核心。

### 表 3：消息聚合读状态表

只在需要“已读人数”时，增加轻量聚合层：

`message_read_aggregate`

字段建议：

- `message_id`
- `conversation_key`
- `read_count`
- `sample_reader_ids`
- `updated_at`

注意：

- 不建议在万人群里存“完整 reader list”
- `sample_reader_ids` 只做前端少量头像样本

---

## 7. Redis 设计建议

## 7.1 关键缓存键

建议增加如下 Redis key：

- `conv_last_seq:{conversation_key}`
- `conv_member_read_seq:{conversation_key}:{user_id}`
- `conv_member_unread:{conversation_key}:{user_id}`
- `conv_summary:{user_id}:{conversation_key}`

## 7.2 读写规则

发消息时：

1. `conv_last_seq` 更新为最新 `session_seq`
2. 不对所有成员逐个 `INCR unread`
3. 只异步刷新活跃成员摘要，或在读侧按差值计算

用户读到末尾时：

1. 上报 `last_read_seq`
2. 更新 `conv_member_read_seq`
3. 把该用户在该会话的 `unread` 清零

## 7.3 为什么不用“逐成员递增未读”

因为万人群里：

- 每条消息给 1 万成员做一次 `INCR`

成本过高。

更好的方式是：

- `unread = last_seq - last_read_seq`

只有在展示层需要时再算或异步回刷。

---

## 8. 接口设计建议

## 8.1 上报已读

新增接口：

`POST /message/read`

请求体建议：

```json
{
  "conversation_key": "group:G123",
  "session_id": "S123",
  "last_read_seq": 1024
}
```

语义：

- 幂等
- 只允许单调前进
- 小于当前已读游标时直接忽略

## 8.2 会话列表

群会话列表返回建议补：

- `last_message`
- `last_message_time`
- `unread_count`
- `last_read_seq`
- `last_seq`

## 8.3 群消息列表

群消息响应体可选补：

- `read_count`
- `read_state`
- `can_query_read_detail`

说明：

- 不是每条消息都必须带完整读明细
- 只带聚合信息即可

## 8.4 已读详情查询

新增接口：

`GET /group/message/readers`

参数：

- `message_id`
- `page`
- `page_size`

限制：

- 仅限小群
- 或仅限公告/管理员场景

---

## 9. 不同群规模的策略

## 9.1 小群

范围建议：

- 200 人以内

策略：

- 可支持较完整的已读回执
- 可以查已读/未读成员明细

原因：

- 腾讯云 IM 公开文档里，群消息已读回执能力本身就限制在 200 人量级

## 9.2 中群

范围建议：

- 200 到 1 万

策略：

- 保留 `last_read_seq`
- 保留 `unread_count`
- 保留 `read_count`
- 不默认返回全量已读名单

## 9.3 万人群 / 热点群

范围建议：

- 1 万以上

策略：

- 只保留会话未读、消息已读人数、少量样本
- 已读明细按需查，且强限制
- 鼓励转为“社群 + 频道/话题”模型

---

## 10. 与现有 EchoChat 的对接方式

## 10.1 第一阶段：先不动群结构，只补读游标和未读数

这阶段不强行重构 `group_info.Members`。

先做：

1. 新增 `conversation_member_state`
2. 会话列表返回 `unread_count`
3. 增加 `mark read` 接口
4. 用 `last_seq - last_read_seq` 驱动未读

这是性价比最高的一步。

## 10.2 第二阶段：把会话摘要从“读时拼装”改成“写时维护”

当前 `session_list_xxx`、`group_session_list_xxx` 更偏读时回源。

建议升级成：

- 新消息到达时，增量更新摘要缓存
- 读回执到达时，增量更新用户会话未读

这样列表页才会真正轻。

## 10.3 第三阶段：群成员模型拆表

如果真的要承载万人群：

- `group_info.Members` 不能继续是主成员关系载体

建议拆成：

- `group_member`

这样才能支持：

- 成员分页
- 活跃成员查询
- 成员读状态 join

## 10.4 第四阶段：热点群切产品模型

到这个阶段，就不建议继续叫“普通群聊”。

建议升级成：

- 社群
- 频道
- 话题

并采用：

- 只对当前活跃频道推正文
- 非活跃频道只推未读变化

这和你仓库里已有的热点群分析文档是一致的。

---

## 11. 风险点

## 11.1 逐成员未读递增会炸

如果你在发群消息时对所有成员做：

- Redis `INCR`
- DB `UPDATE`

万人群下会直接成为热点。

## 11.2 JSON 成员表会卡死后续演进

`group_info.Members` 在读状态模型里会成为明显瓶颈。

## 11.3 已读明细接口容易被滥用

如果开放全量“谁已读谁未读”，需要严格限制：

- 群规模
- 查询频率
- 管理员权限

## 11.4 读状态事件本身可能形成风暴

如果每个用户每次滚动都上报已读，需要：

- 幂等
- 防抖
- 只上报更大的 `last_read_seq`

---

## 12. 建议的实施顺序

### 阶段 A：两周内可做的最小闭环

1. 新增 `conversation_member_state`
2. 新增 `mark read` 接口
3. 会话列表补 `unread_count`
4. 群消息接口补 `read_count` 占位字段

### 阶段 B：补会话摘要缓存

1. 写时更新会话摘要
2. 读时只做轻量回源
3. 未读统一从游标模型取数

### 阶段 C：群成员拆表

1. 建 `group_member`
2. 群发和成员查询从 JSON 迁移
3. 读状态与成员态解耦

### 阶段 D：热点群专用模式

1. 社群/频道/话题
2. 活跃频道推正文
3. 非活跃频道只推未读
4. 已读只保留聚合层

---

## 13. 最终建议

对 EchoChat，这次题目最合理的答案应该是：

1. 万人群不采用全量已读名单模型。
2. 采用 `session_seq + last_read_seq + unread_count` 的读游标方案。
3. 已读明细只在小群或特定消息类型开放。
4. 真正的热点群必须升级成“社群/话题”产品模型。

也就是说：

这题的重点不是“怎么把已读回执也做成万人级”。

而是：

“哪些读状态应该保留，哪些必须降级，哪些要通过产品模型切换来解决。”

---

## 14. 参考资料

### 官方工程资料

- Slack Real-time Messaging  
  https://slack.engineering/real-time-messaging/
- Slack Flannel  
  https://slack.engineering/flannel-an-application-level-edge-cache-to-make-slack-scale/
- Discord Maxjourney  
  https://discord.com/blog/maxjourney-pushing-discords-limits-with-a-million-plus-online-users-in-a-single-server
- Ably Platform Scalability  
  https://ably.com/docs/platform/architecture/platform-scalability

### 官方产品文档

- 腾讯云 IM 产品页  
  https://cloud.tencent.com/product/im
- 腾讯云 IM 群功能配置  
  https://cloud.tencent.com/document/product/269/38656
- 腾讯云 IM 群消息已读回执  
  https://cloud.tencent.com/document/product/269/107478
- Sendbird Group Channel / Supergroup  
  https://docs.sendbird.com/docs/chat/platform-api/v3/channel/creating-a-channel/create-a-group-channel
- Stream Read Status  
  https://getstream.io/chat/docs/react/message-delivery-and-read-status/

### 仓库内已有相关文档

- [im热点群问题与权威方案分析](/workspace/czk/Personal/KKK/doc/im热点群方案分析/im热点群问题与权威方案分析.md)
- [echochat_last_message_会话摘要_未读数优化方案](/workspace/czk/Personal/KKK/doc/会话摘要与未读数优化/echochat_last_message_会话摘要_未读数优化方案.md)
