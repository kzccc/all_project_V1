# 第二次优化：第二次排查分析 `mysql_persist`

版本号：`2`

## 1. 这次在查什么

这次只查一个问题：

**`mysql_persist` 这一步是不是当前链路里的大瓶颈。**

这里的 `mysql_persist` 指的是：

1. Kafka 消费到消息后
2. 把消息真正写进 MySQL `message` 表

代码位置：

1. [kafka_server.go](/workspace/czk/Personal/EchoChat/internal/service/chat/kafka_server.go)
2. [kafka_message_support.go](/workspace/czk/Personal/EchoChat/internal/service/chat/kafka_message_support.go)

---

## 2. 这次怎么测的

上一次 `session_seq` 已经证明：

**`session_seq` 是一个重要瓶颈。**

所以这次为了更干净地验证 `mysql_persist`，我不是在原始版本上直接测，而是：

**在 `session_seq` 已经旁路优化的实验基础上，再把 `mysql_persist` 旁路掉。**

也就是这次实验配置同时打开了两个实验开关：

1. `sessionSeqRedisOnlyExperimental = true`
2. `mysqlPersistNoopExperimental = true`

专用配置文件：

[config_local_sessionseq_mysql_noop.toml](/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/config_local_sessionseq_mysql_noop.toml)

这样做的目的很简单：

1. 先把 `session_seq` 这块木板拿掉
2. 再看 `mysql_persist` 拿掉以后，吞吐还会不会继续明显上涨

如果会，就说明 `mysql_persist` 确实是下一块大木板。

---

## 3. 对照怎么做的

这次主要拿两轮结果做对比：

### 对照组

`session_seq` 已旁路，但仍然真实落库：

`/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/diagnostic_stages_kafka_20260406_222549_sessionseq_redis_only_redo`

### 实验组

`session_seq` 已旁路，`mysql_persist` 也旁路：

`/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/diagnostic_stages_kafka_20260406_231120_mysqlpersist_noop`

压测档位还是固定这几档：

### 单聊

1. `240`
2. `960`
3. `2880`

### 群聊

1. `1440`
2. `5760`
3. `11520`

---

## 4. 最核心结果

## 4.1 `mysql_persist` 一旦旁路，consumer 同步阶段几乎直接被打平

### 单聊 `960`

只旁路 `session_seq` 时：

1. `mysql_persist = 5.527 ms`
2. `total = 5.933 ms`

再旁路 `mysql_persist` 后：

1. `mysql_persist = 0`
2. `total = 0.115 ms`

### 单聊 `2880`

只旁路 `session_seq` 时：

1. `mysql_persist = 9.223 ms`
2. `total = 9.996 ms`

再旁路 `mysql_persist` 后：

1. `mysql_persist = 0`
2. `total = 0.109 ms`

### 群聊 `5760`

只旁路 `session_seq` 时：

1. `mysql_persist = 4.654 ms`
2. `total = 5.138 ms`

再旁路 `mysql_persist` 后：

1. `mysql_persist = 0`
2. `total = 0.266 ms`

### 群聊 `11520`

只旁路 `session_seq` 时：

1. `mysql_persist = 4.600 ms`
2. `total = 5.100 ms`

再旁路 `mysql_persist` 后：

1. `mysql_persist = 0`
2. `total = 0.256 ms`

这说明：

**在 `session_seq` 已经被打掉以后，`mysql_persist` 就是同步主链路里最重的一段。**

---

## 4.2 单聊提升非常夸张

### 单聊 `960`

只旁路 `session_seq` 时：

1. `observed = 323.551 msg/s`
2. `p95 = 11276.65 ms`

再旁路 `mysql_persist` 后：

1. `observed = 795.332 msg/s`
2. `p95 = 2.0 ms`

### 单聊 `2880`

只旁路 `session_seq` 时：

1. `observed = 264.857 msg/s`
2. `success = 0.840458`
3. `p95 = 58829.5 ms`

再旁路 `mysql_persist` 后：

1. `observed = 2377.092 msg/s`
2. `success = 1.0`
3. `p95 = 2.0 ms`

这两个结果已经非常直接了：

**单聊高压下，`mysql_persist` 就是当前最关键的瓶颈之一，而且影响非常大。**

---

## 4.3 群聊也继续提升

### 群聊 `5760`

只旁路 `session_seq` 时：

1. `observed = 3914.862 delivery/s`
2. `p95 = 717.1 ms`

再旁路 `mysql_persist` 后：

1. `observed = 4467.238 delivery/s`
2. `p95 = 2.0 ms`

### 群聊 `11520`

只旁路 `session_seq` 时：

1. `observed = 3995.107 delivery/s`
2. `p95 = 6097.0 ms`

再旁路 `mysql_persist` 后：

1. `observed = 6605.234 delivery/s`
2. `p95 = 4.0 ms`

说明：

**群聊在高压档里，`mysql_persist` 也明显在拖平台。**

特别是 `11520` 这档，吞吐继续大幅抬升，说明群聊在 `session_seq` 后面的下一块大木板，同样就是 `mysql_persist`。

---

## 5. 这次结论

这次可以直接下这几个结论：

### 结论 1

`mysql_persist` 已经被坐实是当前主瓶颈之一。

### 结论 2

在 `session_seq` 去掉以后，`mysql_persist` 就成了单聊和群聊同步主链路里最重的一段。

### 结论 3

单聊高压下，`mysql_persist` 对吞吐和长尾延迟的影响非常大。

### 结论 4

群聊高压下，`mysql_persist` 也会明显限制平台，尤其在更高档位下影响非常明显。

---

## 6. 要注意的一点

这次实验是“旁路真实落库”实验。

所以它带来的收益，不只是：

1. 去掉了 MySQL insert

还包括：

2. 因为没有真实消息记录，所以后面的 `status=sent` 更新也不会执行

所以这次实验的结果，要理解成：

**这是“消息落库整段成本”的上限影响。**

也就是说：

1. 它非常适合用来判断 `mysql_persist` 值不值得优先优化
2. 但不能直接把这次实验值当成“真实可上线效果”

不过即便带着这个前提，这次结果依然足够说明：

**当前消息落库相关成本，确实是核心瓶颈。**

---

## 7. 下一步怎么做

方向已经很清楚了：

不要再继续猜了，下一步应该直接开始打真实的消息落库链路。

---

## 8. 补充验证：三个开关同时开启

这次又额外做了一轮补充验证，把下面三个开关同时打开：

1. `sessionSeqRedisOnlyExperimental = true`
2. `mysqlPersistNoopExperimental = true`
3. `statusUpdateNoopExperimental = true`

三开关配置文件：

[config_local_sessionseq_mysql_status_noop.toml](/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/config_local_sessionseq_mysql_status_noop.toml)

对比对象还是上一轮“双开关”实验：

1. 双开关：`session_seq` 旁路 + `mysql_persist` 旁路
2. 三开关：再额外把 `status=sent` 更新也旁路

---

## 9. 三开关和双开关对比结果

### 单聊

#### `960`

双开关：

1. `observed = 795.332 msg/s`
2. `p95 = 2.0 ms`

三开关：

1. `observed = 798.355 msg/s`
2. `p95 = 2.0 ms`

提升只有大约 `+3 msg/s`。

#### `2880`

双开关：

1. `observed = 2377.092 msg/s`
2. `p95 = 2.0 ms`

三开关：

1. `observed = 2382.114 msg/s`
2. `p95 = 2.0 ms`

提升只有大约 `+5 msg/s`。

### 群聊

#### `5760`

双开关：

1. `observed = 4467.238 delivery/s`
2. `p95 = 2.0 ms`

三开关：

1. `observed = 4479.341 delivery/s`
2. `p95 = 2.0 ms`

提升只有大约 `+12 delivery/s`。

#### `11520`

双开关：

1. `observed = 6605.234 delivery/s`
2. `p95 = 4.0 ms`

三开关：

1. `observed = 6616.392 delivery/s`
2. `p95 = 4.0 ms`

提升只有大约 `+11 delivery/s`。

---

## 10. 这个结果说明什么

结论很明确：

**三开关同时开，并没有再带来明显提升。**

原因也比较直接：

当 `mysqlPersistNoopExperimental = true` 时，消息本身已经不再真实落库，`messageBackUUID` 也不会形成正常的落库后回写数据。

这样一来，后面的 `status=sent` 更新本来就基本不会真正成为主要执行成本。

所以这次三开关实验本质上是在验证一件事：

**`status=sent` 不是在这条链路里被漏掉的“大瓶颈”。**

也就是说：

1. 前面把吞吐拉起来的关键，不是 `status=sent`
2. 真正大的收益，主要还是来自 `session_seq` 和 `mysql_persist` 两段被拿掉
3. `status=sent` 在 `mysql_persist` 已经旁路的情况下，额外收益几乎可以忽略

---

## 11. 最终结论

到这一步可以把结论收得更准一点：

1. `session_seq` 是第一个已坐实的大瓶颈
2. `mysql_persist` 是第二个已坐实的大瓶颈
3. `status=sent` 不是当前这轮吞吐平台的主要限制项

所以下一步优化重点，不应该再放在继续旁路 `status=sent` 上，而是应该继续深挖：

1. 真实消息落库怎么降成本
2. 是否要把落库改成更轻的批量化/异步化路径
3. 是否要把“消费成功”和“重持久化动作”进一步拆开

---

## 12. 本实验上限结论

这里也要特别说明：

**这一节不是在更新全局“当前系统真实容量”。**

它只是在说明：

**在 `session_seq` 已旁路的基础上，再把 `mysql_persist` 旁路后，实验上限大概能抬到哪里。**

如果看“当前系统到目前为止对外统一记录的容量口径”，还是以这份总报告为准：

[第一次优化Kafka优化阶段总结第一次.md](/workspace/czk/Personal/EchoChat/docs/kafka_plan/第一次优化Kafka优化阶段总结第一次.md)

也就是：

1. 单聊真实容量：约 `350 msg/s`
2. 群聊真实容量：约 `4050 delivery/s`
3. 群聊折算消息数：约 `169 group_msg/s`

这次 `mysql_persist` 旁路实验本身，可以先记成下面这组“实验上限结果”：

1. 单聊实验容量：约 `2382 msg/s`
2. 相比上一阶段 `session_seq` 版单聊容量约 `324 msg/s`，提升约 `636.2%`
3. 群聊实验容量：约 `6616 delivery/s`
4. 相比上一阶段 `session_seq` 版群聊容量约 `3995 delivery/s`，提升约 `65.6%`
5. 群聊折算消息数：约 `276 group_msg/s`
6. 相比上一阶段 `session_seq` 版群消息容量约 `166 group_msg/s`，提升约 `65.6%`

这里第 5 条还是按当前压测口径换算：

1. 一个 `25` 人群
2. 发送者自己不算接收者
3. 所以 1 条群消息约等于 `24` 个 delivery

因此：

`6616 / 24 ≈ 275.7`

也就是约 `276 group_msg/s`。

要特别注意：

1. 这一版是旁路实验容量，不是可直接上线宣称的真实业务容量
2. 因为这版把真实消息落库整段成本都拿掉了，所以它更像“上限参考值”
3. 它最重要的意义，是把 `mysql_persist` 坐实成当前主瓶颈之一
4. 它也不是在推翻前面已经记录的 `350 / 4050 / 169`，而是在告诉我们：如果把 `mysql_persist` 这块大木板拿掉，上限能抬高多少

建议优先考虑的方向：

1. 降低每条消息落库成本
2. 减少落库后附带的状态更新成本
3. 看是否能把部分同步落库改成更轻的路径
4. 继续拆 insert 本身和后续 update 的占比

---

## 13. 一句话总结

这次实验已经把 `mysql_persist` 坐实了：在 `session_seq` 之后，当前真正卡住单聊和群聊平台的下一块大木板，就是消息落库这一步。
