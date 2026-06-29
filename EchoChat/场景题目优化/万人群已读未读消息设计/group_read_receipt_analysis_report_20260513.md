# 万人群已读未读消息设计分析与实施报告

日期：2026-05-13

## 结论

EchoChat 的万人群不能沿用普通群“每条消息对全体成员做完整已读回执”的做法。更合理的方案是分级：

| 规模 | 推荐策略 | 说明 |
| --- | --- | --- |
| 200 人以内 | 完整已读回执 | 可以保留成员级已读名单或按消息聚合展示 |
| 200 - 1 万 | 游标 + 未读数 | 以 `last_read_seq`、`unread_count` 为主，明细按需查 |
| 1 万以上 | 社群/话题 + 聚合态 | 只保留摘要、未读、读游标和少量聚合指标 |

## 外部方案要点

我参考的官方资料结论一致：

- Slack 采用 `channel server + gateway server + consistent hash` 的两级 fanout。
- Discord 用 `guild process + session + relays` 把 fanout 拆到多机器。
- Ably 明确把高规模分发做成 tiered fanout。
- 腾讯云 IM 已把“社群”定义为 10 万人超大群，并提供话题拆分。
- 腾讯云群消息已读回执能力只支持部分群类型，且群内最大人数为 200。
- Sendbird 的 group channel 默认上限 100，Supergroup 可扩到 tens of thousands，并保留 unread count / read receipts。
- Stream 把 read status、unread counts、read privacy 拆成独立能力。

## 当前 EchoChat 现状

当前代码里已经有会话摘要和消息顺序，但没有真正的万人群读写模型：

- `session` 只有 `last_message` / `last_message_at`，没有 `unread_count` 或 `last_read_seq`。
- `GetMessageListRespond`、`GetGroupMessageListRespond`、`UserSessionListRespond`、`GroupSessionListRespond` 都没有读状态字段。
- 群消息投递仍然是遍历 `group_info.Members` 的本地 fanout，适合小群，不适合万人群。
- `group_info.Members` 用 JSON 快照存成员，无法承载高频读状态更新。

相关代码可见：

- [internal/model/session.go](/workspace/czk/Personal/KKK/internal/model/session.go)
- [internal/service/chat/server.go](/workspace/czk/Personal/KKK/internal/service/chat/server.go)
- [internal/service/chat/kafka_server.go](/workspace/czk/Personal/KKK/internal/service/chat/kafka_server.go)
- [internal/dto/respond/get_group_messagelist_respond.go](/workspace/czk/Personal/KKK/internal/dto/respond/get_group_messagelist_respond.go)

## 推荐实施方案

### 1. 数据模型

新增一层“会话成员状态”，不要把所有已读信息硬塞进 `group_info`：

- `conversation_key`
- `user_id`
- `last_read_seq`
- `last_read_at`
- `unread_count_cache`
- `updated_at`

同时保留一层会话摘要：

- `last_message`
- `last_message_at`
- `last_seq`
- `last_sender_id`

### 2. 读写语义

- 发消息时只更新会话摘要和发送方回显。
- 用户进入群聊或滚动到末尾时，上报 `last_read_seq`。
- `unread_count = last_seq - last_read_seq`，优先走缓存，异步落库。
- 超大群不广播全量“谁读了”，只展示已读人数、最新读游标或少量样本。

### 3. Redis / DB 分工

- Redis 负责热点读写、游标缓存、计数器。
- MySQL 负责最终状态和恢复。
- 读游标更新走异步刷新，避免每条消息都打爆成员表。

### 4. 阈值切换

- `<= 200`：可启用完整成员级已读。
- `200 ~ 10000`：只保留游标、未读数、少量已读摘要。
- `>= 10000`：切换为社群/话题模式，读状态只保留聚合层。

## 落地顺序

1. 给会话摘要补 `last_seq`、`unread_count`、`last_read_seq`。
2. 增加读回执接口，按 `conversation_key + last_read_seq` 上报。
3. 把群聊未读数改成“游标差值”，避免逐成员递增。
4. 把万人群接入社群/话题模型，不再沿用普通群全量成员广播。
5. 最后再补“已读人数 / 已读详情”查询接口和前端展示。

## 参考资料

- Slack: https://slack.engineering/real-time-messaging/
- Slack Flannel: https://slack.engineering/flannel-an-application-level-edge-cache-to-make-slack-scale/
- Discord: https://discord.com/blog/maxjourney-pushing-discords-limits-with-a-million-plus-online-users-in-a-single-server
- Ably: https://ably.com/docs/platform/architecture/platform-scalability
- 腾讯云群消息已读回执: https://cloud.tencent.com/document/product/269/107478
- 腾讯云群功能配置: https://cloud.tencent.com/document/product/269/38656
- Sendbird group / supergroup: https://docs.sendbird.com/docs/chat/platform-api/v3/channel/creating-a-channel/create-a-group-channel
- Stream delivered & read status: https://getstream.io/chat/docs/react/message-delivery-and-read-status/
