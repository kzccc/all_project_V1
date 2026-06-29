# EchoChat commit 前链路轻量化执行计划

## 1. 这次要解决什么问题

当前单聊 Kafka 链路里，一条消息在 `MarkMessage / Commit` 之前，仍然要同步做完一整串业务步骤。

这会带来两个直接问题：

1. consumer 单条处理时间被拉长，吞吐容易先被同步链路卡住。
2. 只要 commit 前塞了不必要的步骤，这些步骤一变慢，就会直接推高 `consumer lag`、吞吐和 `p95`。

这次改造的目标，不是把链路一下改成“完全异步”，而是先把 commit 前路径收缩到“只保留确认前必须完成的动作”，把不影响可靠性的步骤后移。

本次讨论范围限定为：

- 单机
- 单聊 Kafka 链路
- 重点看 commit 前轻量化

不在这次计划里解决的内容：

- 多机投递
- 群聊 async pipeline
- 完整 outbox / WAL 化彻底异步落库

---

## 2. 当前 commit 前实际包含哪些步骤

按当前实现，一条单聊消息在 commit 前大致会经过下面这些阶段：

1. `deserialize`
   反序列化 Kafka 消息体。
2. `route / validate`
   校验发送者、接收者、消息类型，补 `message_id`。
3. `session_seq`
   生成会话内顺序号。
4. `mysql_persist`
   把消息写入 `message` 表，并等待批量落库 worker 返回结果。
5. `duplicate decision`
   如果遇到重复消息，要判断：
   - 已经 `sent`，直接跳过
   - 还没 `sent`，按“可能没处理完”继续后续分发
6. `websocket_dispatch`
   组装响应体并尝试给在线接收者 / 发送者投递。
7. `redis_read`
   读取单聊消息列表缓存。
8. `redis_write`
   把 append 后的新列表写回 Redis。
9. `MarkMessage`
   把 offset 标成已处理。
10. `Commit`
   批量提交 offset。

也就是说，当前 commit 的语义其实更接近：

**消息已经完成“落库 + 实时分发尝试 + Redis 缓存维护”以后，才允许提交 offset。**

这个语义偏重了。

---

## 3. 这次改造的核心判断标准

先把边界讲清楚。

如果我们把 Kafka 消费成功的最小确认标准定义成：

**消息已经拿到稳定 `message_id`、稳定 `session_seq`、并且已经可靠落库。**

那下面这些动作，其实不一定要留在 commit 前：

- websocket 实时投递
- Redis 消息列表缓存维护
- 一些非关键统计或附加写回

因为它们影响的是：

- 实时性
- 缓存新鲜度
- 用户“立刻看到”的体验

但它们不直接决定：

- 这条消息有没有真正持久化
- 后续能不能通过历史消息补拉回来
- Kafka 这条消息需不需要再重放

所以这次改造的主原则是：

**commit 前只保留“可靠性和幂等判断必须依赖”的步骤，其余全部尽量后移。**

---

## 4. 哪些步骤必须留在 commit 前

这几步当前不建议移出 commit 前。

### 4.1 反序列化与基础校验必须保留

这是最基本的入口过滤。

如果 payload 本身就坏了、字段缺失、类型非法，必须在 commit 前明确判定，否则后面异步化只会把坏数据继续扩散。

### 4.2 `message_id` 归一化必须保留

后面所有幂等判断、重复落库识别、重复分发控制，都依赖稳定唯一键。

如果这一层不稳定，后移任何步骤都会放大重复问题。

### 4.3 `session_seq` 当前版本必须保留

当前消息写库、前端显示顺序、历史消息回放顺序，都依赖 `session_seq`。

如果不重做序号生成模型，这一步还不能简单后移。

换句话说，这次链路轻量化先不碰 `session_seq` 的语义，只把它视为“写库前置条件”。

### 4.4 `mysql_persist` 必须保留

这是当前 commit 前最关键的一步。

只要现在的系统语义还是“至少可靠落库后才能认为消费成功”，那消息持久化就不能放到 commit 后。

而且当前重复消息判断也依赖这里的落库结果：

- 新消息：正常插入
- 1062 重复键：
  - 已经 `sent`，可以直接跳过
  - 未 `sent`，认为“可能没处理完”，需要允许后续补分发

所以在现有语义下，`mysql_persist + duplicate decision` 仍然是 commit 前保留项。

---

## 5. 哪些步骤是本次优先后移对象

### 5.1 第一优先级：Redis 消息列表维护

这是当前最适合先从 commit 前摘出去的一段。

原因很直接：

1. 它不决定可靠性。
2. 它不决定幂等主判断。
3. 就算短时间没更新，用户仍然可以从 MySQL 历史消息拉到正确结果。
4. 当前还是 `get -> append -> set`，不仅有额外耗时，还存在重复分发时重复 append 的风险。

所以第一阶段建议非常明确：

**先把单聊 `redis_read / redis_write` 从 commit 前同步链路里完全拿掉。**

改造后语义：

- commit 前不再维护消息列表缓存
- 缓存改成：
  - 读扩散重建
  - 或 commit 后异步刷新
  - 或干脆先关闭增量维护，只保留查询时回填

这是最稳、改动收益比最高的一刀。

### 5.2 第二优先级：websocket 实时分发从“同步投递”改成“提交后异步投递”

当前 `websocket_dispatch` 指标本身不重，但它仍然属于不必绑定在 commit 前的步骤。

只要消息已经落库，实时投递失败并不等于消息丢失，最多表示：

- 在线即时送达失败
- 接收者稍后靠拉历史消息补上

所以从系统语义上讲，websocket 更适合变成：

**commit 前只做“投递任务入队”，不等待实时投递完成，甚至直接放到 commit 后执行。**

但这一刀比 Redis 更敏感，因为它会改变“实时性保障方式”。

因此不建议一上来就直接全量切换，而是分两步：

1. 先把 Redis 拿掉
2. 再把 websocket 从同步路径挪成 post-commit worker

### 5.3 非关键统计、日志、附加写回一律不允许继续长留在 commit 前

所有不影响：

- 幂等判断
- 序号生成
- 持久化确认

的逻辑，都应该从 commit 前清走。

原则很简单：

**commit 前路径里不能再塞“顺手做一下”的事情。**

---

## 6. 本次不建议直接做的事情

### 6.1 不建议直接把 `mysql_persist` 改成 fire-and-forget

如果 consumer 一收到消息就 commit，只把“待落库任务”扔到内存队列里，短期看吞吐会更高，但可靠性会立刻变脆。

因为一旦进程在真正落库前崩掉，就会出现：

- Kafka 已经 commit
- 消息没有真正持久化
- 这条消息无法靠重放找回

所以在没有持久化 outbox / WAL / 第二持久队列之前，不建议直接这么改。

### 6.2 不建议这一轮同时重做 `session_seq`

`session_seq` 确实是同步主路径的一部分，但它不是这次最容易安全摘掉的步骤。

如果同时改：

- 序号生成
- commit 语义
- websocket 分发
- 缓存策略

变量会太多，测压结论会变脏。

所以这一轮建议先只动“commit 前不必要步骤”，把变量收窄。

---

## 7. 推荐改造顺序

这次建议按 3 个阶段推进。

### 阶段 1：先把 Redis 从 commit 前拿掉

目标：

- commit 前链路删除 `redis_read / redis_write`
- 单聊消费者只保留：
  - deserialize
  - validate
  - message_id
  - session_seq
  - mysql_persist
  - duplicate decision
  - websocket_dispatch
  - MarkMessage / Commit

这一阶段预期收益：

1. consumer `total` 均值会进一步下降。
2. `redis_read`、`redis_write` 从主链路 stage 中消失。
3. 代码语义更清楚，重复分发带来的 Redis 污染问题也会收敛。

这一阶段风险最低，建议先做。

### 阶段 2：把 websocket 分发改成 post-commit 异步任务

目标：

- commit 前只做可靠落库和重复判断
- 消息一旦持久化成功，就允许 `MarkMessage`
- websocket 分发改由独立投递 worker 承接

推荐语义：

1. consumer 成功落库后，生成一条“待分发任务”
2. 先允许消息进入待 commit 集合
3. commit 完成后，再由异步 worker 进行在线投递

这里要补两类保障：

1. 分发任务失败重试
2. 重复分发幂等

否则只会把问题从主链路挪到下游。

这一阶段预期收益：

1. consumer `websocket_dispatch` 基本从同步主路径消失。
2. Kafka commit 前单条固定成本进一步下降。
3. 吞吐更有机会继续抬高，尤其是单聊热分区更容易受益。

### 阶段 3：给 post-commit 路径补齐可观测性和补偿能力

如果把 websocket 和缓存都后移了，就必须把这条新链路单独看住。

至少要补齐：

1. post-commit dispatch queue depth
2. dispatch enqueue duration
3. dispatch success / failure / retry total
4. dispatch worker handle duration
5. Redis refresh success / failure total

否则主链路虽然变轻了，但你看不见新瓶颈在哪。

---

## 8. 每个阶段具体要看哪些指标

这次不是“改完感觉更快了”就算结束，必须按指标判定。

### 8.1 主链路指标

每个阶段都要重点看：

1. `consumer total`
2. `mysql_persist`
3. `session_seq`
4. `websocket_dispatch`
5. `redis_read`
6. `redis_write`
7. `consumer lag`
8. `offset commit duration`
9. `offset commit batch size`

判断方式：

- 如果某一步摘除成功，它对应 stage 应该从主链路显著下降或直接消失。
- 如果 `total` 没明显下降，说明摘掉的不是当前有效负担。

### 8.2 压测结果指标

只看你当前最关心的两类指标：

1. 吞吐
2. `p95`

判断口径继续沿用你现在的“最大稳定吞吐”标准，不额外引入新的业务 SLA 口径。

但每一阶段都要回答两个问题：

1. 最大稳定吞吐有没有提升
2. 同等吞吐下 `p95` 有没有恶化

### 8.3 新增异步路径指标

从阶段 2 开始，至少新增：

1. `post_commit_dispatch_queue_depth`
2. `post_commit_dispatch_enqueue_block_duration`
3. `post_commit_dispatch_duration`
4. `post_commit_dispatch_total{result=success|failure|retry}`
5. `post_commit_redis_refresh_total{result=success|failure}`

因为后移不是白送收益，它只是把压力搬家了。

如果不把新队列看住，很容易出现：

- consumer 吞吐上来了
- 但 post-commit 队列持续积压
- 最终用户端时延反而变差

---

## 9. 测压验证方式

这次建议仍然沿用你当前的固定并发、逐档抬 `target_rate` 的方式，不要另起一套测压框架。

验证步骤建议如下。

### 9.1 阶段 1 验证

目标：

- 验证“去掉 Redis 同步维护”以后，主链路有没有变轻

看 4 个现象：

1. `redis_read / redis_write` 是否从 consumer 主链路中消失
2. 单聊最大稳定吞吐是否上升
3. 同档位 `p95` 是否至少不恶化
4. 历史消息读取是否仍然正确

### 9.2 阶段 2 验证

目标：

- 验证“websocket 改成 post-commit 异步投递”以后，吞吐是否继续抬高

看 5 个现象：

1. `websocket_dispatch` 是否从同步主链路显著变轻
2. `consumer total` 是否继续下降
3. 最大稳定吞吐是否继续上升
4. post-commit dispatch backlog 是否持续增长
5. 在线即时送达是否出现明显漏投或明显延迟恶化

### 9.3 成功判定

这次链路轻量化成功，至少要满足：

1. commit 前主链路 stage 数明显减少
2. 单聊最大稳定吞吐比改造前更高
3. 你的现有稳定判定标准下，结果仍然通过
4. post-commit 队列没有长期失控积压

---

## 10. 风险点

### 10.1 Redis 后移后，缓存新鲜度会下降

这个是预期内代价，不是异常。

只要历史查询仍然能回源拿到正确数据，这个代价是可以接受的。

### 10.2 websocket 后移后，实时性保障方式会改变

以前是：

- 消费线程亲自完成分发尝试后再 commit

改完后会变成：

- 先确认消息已经持久化
- 实时分发由后置队列继续处理

这会让系统语义从“强绑定实时尝试”变成“强绑定持久化，实时投递异步补做”。

这个方向是对吞吐更友好的，但要补好重试和幂等。

### 10.3 如果只做后移，不补监控，后面会变成黑盒

主链路变轻以后，问题不会消失，只会转移。

所以这次执行里，埋点不是附属品，而是主任务之一。

---

## 11. 最终建议

如果按投入产出比排优先级，这次建议顺序非常明确：

1. 先把 Redis 消息列表维护完全移出 commit 前路径。
2. 跑一轮同口径压测，确认主链路 `total`、吞吐、`p95` 的真实变化。
3. 如果收益成立，再推进 websocket post-commit 异步投递。
4. 在 websocket 后移前，先把前端去重、Redis 去重、`status=sent` 幂等这三道保护补齐。
5. 在没有持久化 outbox 之前，不要把 `mysql_persist` 直接改成 commit 前 fire-and-forget。

一句话总结这次执行计划：

**先把 commit 前路径收缩到“序号 + 落库 + 幂等判断”这条最小可靠闭环，再把实时分发和缓存维护逐步后移，用吞吐、`p95` 和 backlog 指标验证收益。**
