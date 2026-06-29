# echochat Redis Pipeline 分析与优化方案

## 目标

这份文档围绕一个非常具体的主题展开：

- 在 `echochat` 当前架构下，如何借鉴 `Redis pipeline` 的用法优化主链路和周边业务链路

重点不是“把所有 Redis 操作都改成 pipeline”，而是回答：

1. 当前有哪些 Redis 读写是串行的
2. 哪些场景适合改成 pipeline
3. 哪些场景不适合只靠 pipeline，而应该进一步改模型或改异步
4. 按什么优先级改最划算

## 什么是 Redis pipeline

简单说，pipeline 的作用是：

- 把多条 Redis 命令一次性发给 Redis，再统一取结果

它主要优化的是：

- 网络往返次数（RTT）
- 连续多命令调用的额外延迟

它不能解决：

- 单条命令本身太慢
- 业务逻辑里 JSON 序列化/反序列化太重
- 模型设计本身不合理

所以 pipeline 是一个“减少往返成本”的工具，不是万能性能药。

## 当前 echochat 里的 Redis 使用现状

从代码看，你当前 Redis 访问大致分三类：

### 1. 单 key 单命令型

例如：

- `SetKeyEx`
- `GetKeyNilIsErr`
- `SetKey`
- `Publish`
- `Incr`

这些命令本身没问题，单独调用也很正常。

### 2. 连续多个独立 Redis 命令串行执行

这类最适合评估 pipeline。

例如：

- 会话创建后连续删除多个缓存 key
- 群操作后连续删除多个缓存 key
- 用户关系变更后连续删除多个缓存 key
- 后续如果补未读数/last_message/summary，也会形成连续多 key 更新

### 3. 业务逻辑型“Redis 读改写”

典型路径是：

- `GET`
- `json.Unmarshal`
- append
- `json.Marshal`
- `SET`

比如：

- `message_list_xxx`
- `group_messagelist_xxx`

这类路径表面上也有多个 Redis 操作，但性能问题不只是 RTT，而是整个“读出完整缓存对象再重写”的模型过重。

## 当前最典型的串行 Redis 热点

## 热点一：单聊消息列表缓存的同步读改写

相关代码：

- [internal/service/chat/kafka_consumed_decode.go](/workspace/czk/Personal/KKK/internal/service/chat/kafka_consumed_decode.go:197)

当前流程是：

1. `GET message_list_xxx`
2. JSON 反序列化整个消息数组
3. 追加新消息
4. JSON 再序列化
5. `SET message_list_xxx`

这条路径的问题是：

- 有 Redis RTT
- 有大对象 JSON 反序列化
- 有大对象 JSON 序列化
- 还在 consumer 热路径里同步执行

结论：

- 这里不适合只谈 pipeline
- 更应该先判断“这段是否应该留在主链路”

## 热点二：群聊消息列表缓存的同步读改写

相关代码：

- [internal/service/chat/kafka_server.go](/workspace/czk/Personal/KKK/internal/service/chat/kafka_server.go:609)

逻辑和单聊基本一样：

1. `GET group_messagelist_xxx`
2. 反序列化整个数组
3. append
4. 再序列化
5. `SET`

这类逻辑的问题和单聊完全一样。

不过你已经在 group async pipeline 开关打开时，对群聊走了一个更轻量的策略：

- 只删缓存，不同步重写

见：

- [internal/service/chat/kafka_server.go](/workspace/czk/Personal/KKK/internal/service/chat/kafka_server.go:600)

这说明这个方向是对的。

## 热点三：会话/群组/联系人等业务变更后的多 key 连续删除

比如：

- `session_service`
- `group_info_service`
- `user_contact_service`

常见模式是：

1. 删除 `group_session_list_xxx`
2. 再删除 `session_list_xxx`
3. 再删除 `contact_mygroup_list_xxx`
4. 再删除 `my_joined_group_list_xxx`

这类代码在多个 service 里都存在。

比如：

- [internal/service/gorm/session_service.go](/workspace/czk/Personal/KKK/internal/service/gorm/session_service.go:77)
- [internal/service/gorm/group_info_service.go](/workspace/czk/Personal/KKK/internal/service/gorm/group_info_service.go:389)
- [internal/service/gorm/user_contact_service.go](/workspace/czk/Personal/KKK/internal/service/gorm/user_contact_service.go:251)

这类场景非常适合：

- pipeline
- 或者合并成更底层的批量删除方法

因为它们的瓶颈主要就是：

- 多次串行 RTT

## 热点四：在线路由未来如果增加 TTL + active_at，也会形成连续写

你当前在线路由只写一个 key：

- [internal/service/chat/kafka_instance_dispatch.go](/workspace/czk/Personal/KKK/internal/service/chat/kafka_instance_dispatch.go:47)

但如果后续按建议补上：

- TTL
- active_at
- user -> multi-connection 集合

那么一次上线/续租就会变成多条 Redis 写操作。

这类未来非常适合 pipeline。

## 哪些场景最适合直接引入 pipeline

## 场景一：连续多 key 删除 / 失效

这是最适合立刻改的地方。

原因：

1. 逻辑简单
2. 风险低
3. 直接减少 RTT
4. 不改业务语义

典型例子：

- 创建会话后失效多个列表缓存
- 群信息变更后失效多个群相关缓存
- 用户关系变更后失效联系人/会话相关缓存

### 建议

给 Redis service 增加类似：

- `DelKeys(keys ...string)`
- 或 `PipelineDelete(keys ...string)`

统一在内部用 pipeline 执行。

## 场景二：一次业务动作后的多项轻量写入

如果后续补上：

- `unread_count`
- `conversation_summary`
- `last_message`

那么消息成功后的副作用链路可能会同时做：

1. `INCR unread`
2. `HSET conversation_summary`
3. `SET/EXPIRE last_message`

这类非常适合 pipeline。

因为它们有几个特点：

1. 相互独立
2. 都是小对象
3. 都是写操作
4. 同一业务动作触发

这正是 pipeline 最擅长的场景。

## 场景三：批量 publish

这一点 `go-chat` 已经给了一个很明确的借鉴：

- 多个 `Publish` 放 pipeline 一起发

虽然你当前 `echochat` 里 Redis `Publish` 主要是单条跨实例投递：

- [internal/service/chat/kafka_instance_dispatch.go](/workspace/czk/Personal/KKK/internal/service/chat/kafka_instance_dispatch.go:214)

但如果后续出现：

- 批量远端通知
- 批量 side-effect 事件投递

那么 pipeline 也值得用。

## 哪些场景不适合只靠 pipeline

## 场景一：完整消息列表缓存的同步读改写

这是最容易被误判的地方。

像：

- `GET -> unmarshal -> append -> marshal -> SET`

就算你把 `GET` 和 `SET` 放 pipeline，也解决不了主要问题：

- JSON 反序列化重
- JSON 序列化重
- 大对象写回重
- 主链路同步执行重

所以这类场景，真正优化方向应该是：

1. 先判断是否应该留在主链路
2. 如果保留，考虑改为：
   - 只删缓存
   - 异步 append
   - 或改用 list/stream/hash 等更适合增量更新的结构

而不是先上 pipeline。

## 场景二：需要依赖前一个命令结果做复杂业务分支的流程

如果业务逻辑是：

1. 先 `GET`
2. 根据内容做复杂判断
3. 再决定写什么

那 pipeline 只能帮一部分，不会改变本质复杂度。

这时更应该优先考虑：

- 是否能换轻量模型
- 是否能前置判断
- 是否能异步化

## 对当前 echochat 的最优借鉴方式

## 第一层：立即可做的 pipeline 改造

这层不动核心消息模型，优先改边缘高频的多 key 串行操作。

### 建议一：统一封装批量删除

适用场景：

- `session_service`
- `group_info_service`
- `user_contact_service`

把当前这种：

- 删 key A
- 删 key B
- 删 key C

统一改成：

- 一个 pipeline 批量删

### 收益

- 改动小
- 风险低
- 能立刻减少管理类业务的 Redis RTT

## 第二层：为未来的“轻副作用层”提前准备 pipeline 能力

如果后续你按建议补：

- 会话摘要
- 未读数
- last_message

那建议从一开始就设计成：

- 同一条消息触发的多个 Redis 副作用统一 pipeline 写入

例如：

1. `INCR unread`
2. `HSET conversation_summary`
3. `SETEX last_message`

一次 pipeline 做完。

### 收益

- 避免以后补功能后 Redis RTT 线性增加
- 很适合消息系统这种“一次动作带来多个轻写入”的模式

## 第三层：把“重缓存维护”改成“轻缓存维护 + 失效”

这是和 pipeline 同样重要的一层。

当前完整消息列表缓存不是最适合 pipeline 的对象，正确方向是：

### 单聊

- `message_list_xxx` 主链路只失效或异步更新

### 群聊

- `group_messagelist_xxx` 已经部分采用失效优先，建议统一

### 列表页

- 用 `conversation_summary` / `last_message` / `unread` 这类轻量缓存承担

### 结论

pipeline 最适合“轻量多写”，不适合拯救“重对象整包读改写”。

## 适合当前 echochat 的实施顺序

## P0：先补 Redis service 的 pipeline 基础能力

建议新增：

1. `ExecPipeline(fn func(pipe redis.Pipeliner) error) error`
2. `DelKeys(keys ...string) error`
3. 未来可加：
   - `SetMany`
   - `PublishMany`

目标：

- 先把基础设施备好

## P1：把多 key 失效改成 pipeline

优先改：

1. `session_service`
2. `group_info_service`
3. `user_contact_service`

目标：

- 低风险拿到第一波收益

## P2：设计会话摘要 / 未读数 / last_message 的 pipeline 副作用写入

目标：

- 未来每条消息的轻量副作用统一一批写

示意：

1. `INCR unread`
2. `HSET conversation_summary`
3. `SETEX last_message`

## P3：把完整消息列表缓存从主链路里降权

目标：

- 不再试图用 pipeline 优化错误层次的问题

而是直接改模型：

- 重缓存不主链路同步维护
- 轻缓存主链路增量维护

## 一个很重要的判断标准

当你在看某段 Redis 逻辑要不要 pipeline 时，可以用这个标准：

### 如果这段逻辑是

- 多个独立小命令
- 同一业务动作触发
- 不强依赖前一个命令结果

那很适合 pipeline。

### 如果这段逻辑是

- 读出一个大对象
- 本地做复杂变换
- 再整包写回

那问题大概率不在“没用 pipeline”，而在“模型太重”。

## 最终建议

对你当前 `echochat` 来说，借鉴 `go-chat` 的 Redis pipeline 用法，最正确的方式不是“全面替换”，而是：

1. 先把多 key 串行删除/失效统一批量化
2. 再把未来的轻副作用层按 pipeline 设计
3. 最后把完整消息列表缓存从主链路里降权

这样收益最大，风险最小，也最符合你现有 Kafka 主干架构。

## 末尾压缩版：每点一句话

1. 当前很多 Redis 管理类操作是多 key 串行执行，最适合先改成 pipeline 批量删除或批量失效。
2. 当前完整消息列表缓存的性能问题不只是 Redis RTT，而是“整包读改写”模型过重，不适合只靠 pipeline 修补。
3. 后续如果补 `unread + conversation_summary + last_message`，这些轻量副作用非常适合一次 pipeline 统一写入。
4. Pipeline 最适合“多个独立小命令一次发”，不适合解决“大对象先读后改再整包写”的结构性问题。
5. 当前最划算的改法是先补 Redis service 的 pipeline 基础能力，再统一改多 key 失效路径。
6. 真正长期有效的方向不是“让重缓存更快”，而是“让重缓存退出主链路、让轻缓存进入主链路”。
