# 第二次优化：第二次排查后 `session_seq` 和 `mysql_persist` 优化落地计划安排

## 1. 这份计划是要解决什么问题

前面的第二次排查已经把两件事基本坐实了：

1. `session_seq` 是当前链路里的一个大瓶颈
2. `mysql_persist` 是 `session_seq` 后面的下一块大木板

也就是说，当前 Kafka 吞吐上不去，核心已经不是：

1. Kafka 有没有拉到消息
2. consumer 有没有 ready
3. websocket 写回是不是第一主瓶颈

而是：

**Kafka 拉到消息以后，业务主链路里“会话序号分配”和“消息落库”这两段同步成本太高。**

这份文档的目标不是再做一轮分析，而是把后面真正要落地的优化工作安排清楚。

---

## 2. 当前代码里的真实问题点

当前热路径主要在：

1. [message_sequence.go](/workspace/czk/Personal/EchoChat/internal/service/chat/message_sequence.go)
2. [kafka_message_support.go](/workspace/czk/Personal/EchoChat/internal/service/chat/kafka_message_support.go)
3. [kafka_server.go](/workspace/czk/Personal/EchoChat/internal/service/chat/kafka_server.go)
4. [client.go](/workspace/czk/Personal/EchoChat/internal/service/chat/client.go)
5. [message.go](/workspace/czk/Personal/EchoChat/internal/model/message.go)

当前一条普通消息的大致同步路径是：

1. Kafka 消费
2. 计算 `session_seq`
3. `Create(message)` 写 MySQL
4. websocket 分发
5. 首次写回成功后，再 `Update status=sent`

这里的问题非常直接：

### 2.1 `session_seq` 的问题

当前原始逻辑是：

1. 先查 MySQL `MAX(session_seq)`
2. 再走 Redis `SetNX + Incr`

这意味着：

1. 每条消息都要多打一趟 MySQL
2. 单聊查询里还有 `(A->B) OR (B->A)`，高压下很吃索引和 CPU
3. `message` 大表被拿来做“序号分配地板查询”，成本很不划算

### 2.2 `mysql_persist` 的问题

当前原始逻辑是：

1. 每条消息都同步 `Create(message)`
2. 落库完成后才继续后面的消费处理
3. websocket 首次写回成功后，还会再做一次 `status=sent` 更新

这意味着：

1. consumer 主链路被单条 insert 卡住
2. `message` 表每条写入都要承担索引维护成本
3. 整条链路存在“insert 一次 + update 一次”的写放大

---

## 3. 这次优化的总原则

这次优化不建议靠“调几个 MySQL 参数”或者“继续加机器”来顶。

总原则应该是：

1. 不再让 `message` 大表承担高频 `session_seq` 分配职责
2. 不再让 consumer 主线程按“每条消息一次 insert”的方式同步落库
3. 尽量把“高频单条写”改成“轻量原子分配 + 批量持久化”
4. 尽量减少 `message` 表上的无效索引和重复写

一句话就是：

**把当前“每条消息同步打 MySQL”的模型，改成“Redis 快速分配 + MySQL 低频/批量持久化”的模型。**

---

## 4. `session_seq` 优化落地方向

## 4.1 目标

把当前：

1. 每条消息查一次 `MAX(session_seq)`
2. 再 Redis 递增

改成：

1. Redis 作为主分配器
2. MySQL 只保存会话级高水位
3. 不再从 `message` 表查 `MAX(session_seq)`

---

## 4.2 具体改造步骤

### 步骤 1：把 Redis-only 方案从实验开关升级成正式路径

当前已经有实验开关：

`sessionSeqRedisOnlyExperimental`

这说明代码路径已经验证过方向是对的。

下一步要做的是：

1. 不再把 Redis-only 当实验旁路
2. 把它整理成正式实现
3. 保留降级开关，但默认走新路径

### 步骤 2：把 `SetNX + Incr` 改成 Lua 单次原子操作

当前 Redis 里是两次请求：

1. `SetNX`
2. `Incr`

这还能继续优化。

应该改成：

1. 用 Lua 脚本一次完成“若不存在则初始化 floor，再自增”
2. 降低 Redis 往返次数
3. 保证逻辑更原子、更稳定

### 步骤 3：新增会话级高水位表

建议新增一张轻量表，例如：

`conversation_sequence`

字段建议：

1. `conversation_key`
2. `last_seq`
3. `updated_at`

其中 `conversation_key` 直接复用当前：

[message_sequence.go](/workspace/czk/Personal/EchoChat/internal/service/chat/message_sequence.go)

里的 `buildConversationSequenceScope()` 规则。

### 步骤 4：Redis miss 时不再查 `message` 表

后面 Redis 如果 miss：

1. 不再查 `message` 表 `MAX(session_seq)`
2. 只查 `conversation_sequence.last_seq`

这样可以彻底把 `message` 大表从序号分配路径里移出去。

### 步骤 5：做低频高水位回刷

Redis 里的当前序号，需要周期性回刷 MySQL。

建议做法：

1. 定时回刷
2. 或者每累计一批消息回刷一次
3. 回刷目标只写 `conversation_sequence`

这样做的意义是：

1. Redis 负责高频
2. MySQL 负责恢复点
3. 重启后可从高水位恢复，不需要扫消息大表

---

## 4.3 `session_seq` 这一块的验收标准

这一块做完后，要重点看：

1. `session_seq` 阶段耗时是否稳定降到很低
2. 单聊和群聊高压下 `total` 是否明显下降
3. `session_seq` 不再出现 MySQL 查询放大
4. 历史消息顺序是否仍然正确
5. 重启后序号是否仍然连续、无回退

---

## 5. `mysql_persist` 优化落地方向

## 5.1 目标

把当前：

1. 每条消息同步 `Create(message)`
2. insert 后再补一次 `status=sent`

改成：

1. 批量 insert
2. 尽量减少 `message` 表写放大
3. 逐步弱化同步 `status=sent` 更新

---

## 5.2 具体改造步骤

### 步骤 1：先把单条 insert 改成批量 insert

这是最优先的一步。

建议直接在 Kafka consumer 路径里做批量缓冲：

1. 攒够 `N` 条就 flush
2. 或者到 `T` 毫秒就 flush
3. 用 `CreateInBatches` 统一写入

建议第一版参数可以先从下面起步：

1. `batch_size = 50 ~ 200`
2. `flush_interval = 5 ~ 10ms`

第一版先不要追求特别复杂，先把“每条一写”改成“批量一写”。

### 步骤 2：把幂等控制保留在 `uuid`

当前已经有：

1. `uuid` 唯一约束
2. duplicate 检测逻辑

批量写后，这套幂等模型还要保留。

也就是说：

1. 批量写不能牺牲去重能力
2. 重复消息仍然要能识别
3. consumer retry 时不能因为批量化而把一致性打坏

### 步骤 3：减少 `message` 表索引写成本

当前 `message` 表承载了不少索引。

后面要按真实查询模式收索引。

建议方向：

1. 保留 `uuid` 唯一索引
2. 新增或改成更贴合查询模式的复合索引
3. 逐步淘汰命中差、但写入很贵的单列索引

这一块不要盲删，必须先按查询路径来做。

### 步骤 4：给消息增加 `conversation_key`

这是和 `session_seq` 联动的一步。

建议给 `message` 表增加：

`conversation_key`

后面消息查询统一往这个方向收：

1. 单聊不再走 `(A->B) OR (B->A)`
2. 单聊和群聊统一按 `conversation_key`
3. 历史查询统一按 `conversation_key + session_seq`

这样能同时带来两件好事：

1. 查询更简单
2. insert 端索引模型更清晰

### 步骤 5：弱化同步 `status=sent` 更新

虽然旁路实验说明：

`status=sent` 不是当前最大的瓶颈

但在真实链路里，它依然属于额外写放大。

建议后续方向：

1. 如果业务允许，入库时直接使用最终状态
2. 如果必须保留 `unsent -> sent` 语义，就改成异步批量更新
3. 至少不要把它放在 websocket 成功路径上同步单条 update

这一步不是第一优先级，但应该纳入本轮改造范围。

---

## 5.3 `mysql_persist` 这一块的验收标准

这一块做完后，要重点看：

1. `mysql_persist` 阶段耗时是否明显下降
2. 单聊高压档吞吐是否明显上升
3. 群聊高压档吞吐平台是否继续上移
4. `p95` 是否同步下降
5. 批量写后是否仍然保持幂等和顺序正确

---

## 6. 建议的实际落地顺序

如果按收益和风险平衡，我建议后面按这个顺序推进：

### 第一阶段：先做 `session_seq` 正式化

先完成：

1. Redis 主分配
2. Lua 原子脚本
3. `conversation_sequence` 高水位表
4. Redis miss 时只查高水位表

这一阶段做完后，先跑一轮对照压测，确认 `session_seq` 真正从热路径里拿掉。

### 第二阶段：再做 `mysql_persist` 批量化

然后完成：

1. consumer 批量缓冲
2. `CreateInBatches`
3. 批量路径下的幂等处理

这一阶段大概率会直接抬单聊高压平台。

### 第三阶段：统一消息模型和索引

再往后做：

1. `conversation_key`
2. 查询路径收敛
3. `message` 表索引瘦身

这一阶段是结构性优化，收益会更稳。

### 第四阶段：处理 `status=sent` 双写

最后再看：

1. 是否改成异步
2. 是否改成批量
3. 是否可以简化状态模型

---

## 7. 本轮不建议先做的事情

为了避免走偏，这几件事现在不建议先做：

1. 先去调 Kafka broker 参数
2. 先去盲目扩大 MySQL 连接池
3. 先靠加更多 consumer 实例硬顶
4. 在没有改模型前先删一堆索引试运气

原因很简单：

**当前瓶颈已经不是“配置不够大”，而是热路径模型本身太重。**

---

## 8. 本轮每一步做完后都要怎么验证

每做完一块，都继续用现在这一套固定档位测：

### 单聊

1. `240`
2. `960`
3. `2880`

### 群聊

1. `1440`
2. `5760`
3. `11520`

每一轮都固定看这几项：

1. `observed`
2. `success / coverage`
3. `p95`
4. `session_seq`
5. `mysql_persist`
6. `total`

这样后面每改一版，才方便横向比较指标是不是上去了。

---

## 9. 这份计划最后要达到什么结果

这份计划最终想达到的，不只是“再涨一点吞吐”，而是把系统从当前这种：

1. 每条消息同步查 MySQL
2. 每条消息同步插 MySQL
3. 每条消息再补一次 update

改成更合理的模型：

1. `session_seq` 用轻量分配
2. 落库用批量化持久化
3. 查询和索引按 `conversation_key + session_seq` 收敛
4. 减少不必要的重复写

如果这几步能落地，后面你再继续做 Kafka 吞吐优化，才会是真正有效的优化。

---

## 10. 一句话总结

下一步最值得做的，不是继续猜瓶颈，也不是继续做旁路实验，而是正式把：

1. `session_seq` 改成 Redis 主分配 + MySQL 高水位恢复
2. `mysql_persist` 改成批量 insert + 更轻的消息模型

这两件事真正落到代码里。
