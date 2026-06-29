# OpenIM 本地缓存层设计分析

## 1. 这份文档解决什么问题

这份文档只讲 `open-im-server` 的**本地缓存层设计**，不展开整个 IM 主链路。

目标是两件事：

1. 把它这套本地缓存层到底怎么工作的讲明白。
2. 抽出真正值得 `echochat` 借鉴的关键点，方便后面改你自己的架构。

这里重点分析的是这些模块：

- `pkg/localcache`
- `pkg/localcache/lru`
- `pkg/localcache/link`
- `pkg/rpccache/*`
- `pkg/common/storage/cache/redis/batch_handler.go`

---

## 2. 先说结论

`open-im-server` 的本地缓存层，不是简单的“进程内 map + TTL”。

它实际上是一个三层结构：

1. **本地近端缓存层**
2. **Redis / RocksCache 分布式缓存层**
3. **DB / RPC 回源层**

再叠加两个机制：

1. **跨节点失效通知**
2. **关联 key 联动删除**

所以它的核心设计思路不是：

- 每次都从 Redis 取
- 更新时同步把所有节点都改对

而是：

- 优先命中本地缓存
- miss 时走 Redis/远端
- 变更时以“失效”为主，而不是以“同步重写值”为主

这是它最值得借鉴的地方。

---

## 3. 整体结构

可以先把它抽象成下面这张逻辑图：

```text
业务代码
  -> rpccache 本地代理层
    -> localcache 本地 LRU/Slot LRU
      -> Redis/RocksCache
        -> RPC 或 DB

数据变更时：
  DB/RPC 更新
    -> BatchDeleter 批量标记 Redis 缓存失效
    -> Redis Pub/Sub 广播失效 key
    -> 其他节点 subscriber 收到后删除本地缓存
```

这个结构里，每一层职责都比较清楚：

- `rpccache`：给业务提供“像直接查 RPC 一样”的接口，但内部先查本地缓存。
- `localcache`：负责进程内近端缓存、TTL、slot 分片、关联删除。
- `rocks cache / redis cache`：负责跨节点共享缓存。
- `subscriberRedisDeleteCache`：负责让各节点本地缓存感知远端失效。

---

## 4. 读路径是怎么跑的

### 4.1 业务不是直接查远端，而是先进 `rpccache`

例如会话缓存：

- `ConversationLocalCache.GetConversationIDs`
- `ConversationLocalCache.GetConversation`

见：

- [conversation.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/rpccache/conversation.go:34)
- [conversation.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/rpccache/conversation.go:60)

这些函数看起来像是普通查询，但内部先走：

- `x.local.Get(...)`

也就是说，业务只面对一个“本地缓存代理层”，不需要自己关心：

- 是否命中本地缓存
- 是否回源 RPC
- 是否反序列化

这一层封装做得比较干净。

### 4.2 `localcache.Get` 的实际逻辑

本地缓存对业务暴露的是：

- `Get`
- `GetLink`
- `Del`
- `DelLocal`

见：

- [cache.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/localcache/cache.go:26)

`Get` 最终就是：

- 先查本地 LRU
- miss 时调用 `fetch`
- 取回结果后写回本地缓存

入口在：

- [cache.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/localcache/cache.go:106)

所以它的读路径本质是：

```text
业务 -> localcache.Get -> 命中本地直接返回
                    -> miss 则 fetch(ctx)
                    -> 将结果写回本地
```

### 4.3 本地缓存不是单个 LRU，而是 Slot LRU

如果配置了多个 slot：

- 会创建 `NewSlotLRU`

见：

- [cache.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/localcache/cache.go:47)
- [lru_slot.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/localcache/lru/lru_slot.go:17)

它的意义很直接：

- 把 key 按 hash 打散到多个独立 LRU
- 降低全局锁竞争
- 热 key 不会都卡在同一把大锁上

这一点很适合高并发读场景。

---

## 5. 本地 LRU 是怎么设计的

### 5.1 不是普通 map，而是带 TTL 的 LazyLRU

默认本地缓存常用的是 `LazyLRU`：

- [lru_lazy.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/localcache/lru/lru_lazy.go:31)

它每个缓存项都存：

- `value`
- `err`
- `expires`
- 每项自己的 `lock`

见：

- [lru_lazy.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/localcache/lru/lru_lazy.go:24)

这有两个很重要的点：

1. **成功和失败都有 TTL**
2. **每个 key 自己串行回源**

### 5.2 失败结果也缓存

`LazyLRU` 会区分：

- `successTTL`
- `failedTTL`

见：

- [lru_lazy.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/localcache/lru/lru_lazy.go:42)

也就是说，如果某次回源失败：

- 它不会让后续所有请求立刻继续打远端
- 而是短暂缓存这个失败结果

这能减少热点错误场景下的缓存击穿。

### 5.3 每个 key 自己加锁，避免重复回源

`Get` 时如果 key 已存在但过期了：

- 先拿到这个 key 对应 item
- 再对 item 加锁

见：

- [lru_lazy.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/localcache/lru/lru_lazy.go:58)

这意味着同一个 key 在并发 miss 时，不会每个请求都自己回源，而是：

- 一个请求去 fetch
- 其他请求等这个 key 的 item 锁

这本质上就是一个**单 key 级别的请求合并**。

这个点很关键，很多简单本地缓存都没有做。

### 5.4 批量获取时还支持批量 miss 合并

`GetBatch` 会：

1. 先扫一遍本地缓存，收集 miss key
2. 把 miss key 一次性传给 `fetch(keys []K)`
3. 再把结果批量回填本地缓存

见：

- [lru_lazy.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/localcache/lru/lru_lazy.go:91)

这非常值得你关注，因为它不是只优化“单 key 命中”，还优化了：

- 会话列表批量查询
- 批量用户信息查询
- 批量在线态查询

这种“多 key 读”的效率。

---

## 6. 关联删除机制是这套设计里很容易被忽略的亮点

### 6.1 `GetLink` 支持建立 key 之间的关联

本地缓存除了 `Get`，还有 `GetLink`：

- [cache.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/localcache/cache.go:110)

当一个缓存值依赖其他 key 时，可以把它们 link 起来。

比如逻辑上可能出现：

- `conversation_ids:{user}`
- `conversation:{user}:{conversationID}`

如果列表 key 失效，相关单项 key 也应该联动清掉，或者反过来。

### 6.2 `link` 组件做的是图状联动删除

`pkg/localcache/link` 里维护的是：

- key -> 关联 key 集合

见：

- [link.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/localcache/link/link.go:23)

删除时不是只删当前 key，而是：

- 递归把关联 key 一起收集出来删掉

见：

- [link.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/localcache/link/link.go:89)

这点的价值在于：

- 你不需要每次手工推导“改了 A 还要删哪些 B/C”
- 本地缓存内部能保证关联对象不残留旧值

### 6.3 它还特意规避了 eviction 回调里的死锁风险

当本地 LRU eviction 发生时，它不会直接在当前锁内递归删 linked key，而是：

- 先取出 link 集合
- 再起 goroutine 异步删 linked key

见：

- [cache.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/localcache/cache.go:73)

注释里也直接说明了原因：

- 避免在底层 LRU 还持锁时重入同一 slot 导致死锁

这说明这套缓存层不是“能跑就行”，而是考虑过高并发下的锁行为。

---

## 7. 分布式缓存失效是怎么做的

这部分是整套机制真正闭环的地方。

### 7.1 Redis/RocksCache 这一层不是直接覆盖写，而是偏“失效”

`getCache` 的核心逻辑是：

- 先走 `Fetch2`
- miss 时执行回源函数
- 序列化结果写回缓存

见：

- [batch_handler.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/common/storage/cache/redis/batch_handler.go:113)

而数据更新时，更常见的不是“直接改缓存内容”，而是：

- `TagAsDeletedBatch2`

见：

- [batch_handler.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/common/storage/cache/redis/batch_handler.go:56)

它的思路更偏：

- 更新 DB / 远端后
- 把 Redis 缓存标成失效
- 下次读再重建

这和你当前 `echochat` 里很多“同步重写完整缓存内容”的思路不同。

### 7.2 失效不是单机行为，而是带跨节点广播

批量删除缓存后，它还会：

- 按 topic 组织要广播的 key
- Redis `Publish`

见：

- [batch_handler.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/common/storage/cache/redis/batch_handler.go:62)

也就是说：

- Redis / RocksCache 负责分布式共享缓存
- Pub/Sub 负责告诉其他节点“这些 key 本地也该删了”

### 7.3 每个节点本地都有订阅器，收到后删本地缓存

`subscriberRedisDeleteCache` 就干这一件事：

- 订阅某个 channel
- 解析失效 key 列表
- 调 `DelLocal`

见：

- [subscriber.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/rpccache/subscriber.go:10)

所以完整链路是：

```text
写请求更新数据
  -> 批量标记 Redis 缓存失效
  -> publish 失效 key
  -> 其他节点 subscriber 收到
  -> 删除各自本地缓存
```

这就是它为什么能大胆地大量使用本地缓存，因为它不是“每个节点各缓存各的没人管”，而是有一套统一失效同步机制。

---

## 8. `rpccache` 这一层的意义

如果只看 `localcache`，你会觉得这只是个通用本地缓存组件。

真正让它在项目里落地的是 `rpccache` 这一层。

例如：

- `ConversationLocalCache`
- `UserLocalCache`
- `GroupLocalCache`
- `OnlineCache`

它们做的是：

1. 把“本地缓存 + RPC 回源 + 本地失效订阅”封在一个对象里
2. 业务方只调一个语义化接口，不自己管缓存细节

以 `ConversationLocalCache` 为例：

- 初始化时订阅 Redis 删除 topic。[conversation.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/rpccache/conversation.go:24)
- 读取时统一先进本地缓存。[conversation.go](/workspace/czk/Personal/IMproject/open-im-server/pkg/rpccache/conversation.go:60)

这层的价值非常大，因为它决定了缓存不会在业务代码里散掉。

---

## 9. 这套设计为什么性能好

可以压成 6 个原因。

### 9.1 热读尽量止于进程内

命中本地缓存时：

- 不打 Redis
- 不打 RPC
- 不走 DB

这是延迟最低、开销最小的路径。

### 9.2 slot 分片降低锁竞争

不是一个全局大缓存锁，而是：

- hash 到多个 LRU slot

高并发下可扩展性更好。

### 9.3 单 key miss 不会形成回源风暴

每个 key 自己有 item lock，天然合并并发 miss。

### 9.4 多 key 查询支持批量 miss 合并

会话列表、批量用户信息、批量在线态这类查询不会一把一把回源。

### 9.5 失效优先，避免主链路重写大对象

更新动作不强求“同步把每层缓存改成新值”，而是优先：

- 删除
- 标失效
- 下次读再构建

这能显著减轻写路径负担。

### 9.6 本地缓存和分布式缓存是联动的

这不是“单机小技巧”，而是：

- 本地缓存
- Redis/RocksCache
- Pub/Sub 失效广播

三层联动，所以规模上去以后仍然能用。

---

## 10. 对 `echochat` 最有借鉴价值的点

这里不讲泛泛而谈，只讲你后面最可能真改的地方。

### 10.1 先补一层本地近端缓存，而不是所有读都直打 Redis

适合先做本地缓存的对象：

- 用户在线状态
- 会话摘要
- 用户基础资料
- 会话配置项

这些对象：

- 读多写少
- 热点明显
- 可接受短 TTL

### 10.2 本地缓存不要直接上一个全局 map，至少做 slot 分片

如果你后面自己做：

- 推荐 `slot LRU`
- 不建议一个全局 `map + mutex`

因为 IM 场景的读热点很集中，全局锁很容易成为瓶颈。

### 10.3 缓存更新优先走“失效”，不要优先走“重写完整对象”

这个对你当前最关键。

你现在很多地方的问题不是 Redis RTT，而是：

- 主链路里在同步重写完整缓存对象

`open-im-server` 给你的启发是：

- 更新 DB / 核心状态
- 删对应缓存
- 下次读再重建

特别适合你现在的：

- `message_list_xxx`
- `group_messagelist_xxx`

这类大对象缓存。

### 10.4 本地缓存最好配一个“失效广播”机制

如果你只加本地缓存，不加跨节点失效同步，后面一定会脏。

最小可行方案就是：

- Redis Pub/Sub 广播失效 key
- 各实例删本地缓存

这个方案工程成本不高，但收益很大。

### 10.5 给缓存对象建“关联删除”关系

如果后面你会同时缓存：

- `conversation_summary:{user}`
- `conversation:{user}:{conversationID}`
- `session_list:{user}`

那最好建立 key link，而不是每次手工枚举要删哪些 key。

这会让本地缓存结构稳定很多。

---

## 11. 我建议你在 `echochat` 里怎么分阶段落地

### 第一阶段：最小增量

先做：

1. 本地在线态缓存
2. 本地会话摘要缓存
3. Redis Pub/Sub 本地失效订阅

不要一上来就缓存消息列表。

### 第二阶段：把列表类读请求从“远端/Redis直读”切到“本地缓存代理层”

可以抽一个类似 `rpccache` 的层，比如：

- `local_session_cache`
- `local_online_cache`
- `local_user_cache`

业务层统一从代理读。

### 第三阶段：补 key 关联删除

当本地缓存对象开始变多时，再引入 link 机制。

### 第四阶段：评估是否需要把消息明细也做成本地近端缓存

这一步要谨慎。

我的建议是：

- 优先缓存摘要、配置、在线态
- 不优先缓存完整消息列表

因为完整消息列表在你当前系统里体积大、更新频繁、热路径敏感。

---

## 12. 一句话总结

`open-im-server` 的本地缓存层，核心不是“加了个 LRU”，而是把**本地近端缓存、分布式缓存、批量失效、订阅删除、关联 key 联动删除**组合成了一套完整机制；这对 `echochat` 最值得借鉴的是“本地缓存代理层 + 失效优先 + Pub/Sub 同步删除”，而不是去同步重写大缓存对象。

---

## 13. 压缩版关键点

1. 本地缓存不是普通 map，而是带 TTL、支持批量回源的 `Slot LRU`。
2. 同一个 key 并发 miss 时只会有一个请求真正回源，避免缓存击穿。
3. 批量查询支持 `GetBatch`，会把 miss key 合并后一次性回源。
4. 数据更新时以“批量失效”优先，不在主链路同步重写大对象缓存。
5. Redis Pub/Sub 只广播“哪些 key 失效”，各节点收到后删除自己的本地缓存。
6. `link` 机制可以把关联 key 联动删除，减少本地缓存残留脏数据。
7. `rpccache` 这一层把缓存细节包起来，业务读接口不需要自己管理缓存。
8. 对 `echochat` 最适合先落地的是在线态、会话摘要、用户资料这类小对象本地缓存。
