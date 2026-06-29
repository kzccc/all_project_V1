# 第二次优化：第二次排查分析 `session_seq`

版本号：`1`

## 1. 这次在查什么

这次只查一个问题：

**`session_seq` 里“每条消息都查一次 MySQL `MAX(session_seq)`”是不是瓶颈。**

当前原逻辑是：

1. 先查 MySQL `MAX(session_seq)`
2. 再走 Redis `SetNX + Incr`

如果这段真是瓶颈，那我把 MySQL 这一步先去掉，吞吐和延迟应该会明显变化。

---

## 2. 这次怎么测的

我做了一个实验开关：

`sessionSeqRedisOnlyExperimental = true`

打开以后，`session_seq` 只走 Redis 递增，不再每条消息查 MySQL floor。

对比方式：

1. 用原版本跑一轮定位压测
2. 用实验版再跑同样的档位
3. 对比吞吐、`p95`、`session_seq` 阶段耗时、`total` 阶段耗时

压测档位还是固定这几档：

### 单聊

1. `240`
2. `960`
3. `2880`

### 群聊

1. `1440`
2. `5760`
3. `11520`

实验结果目录：

`/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/diagnostic_stages_kafka_20260406_222549_sessionseq_redis_only_redo`

另外，单聊 `240` 这一档主实验里有一条异常样本，我又单独复核了一次：

`/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/diagnostic_stages_kafka_20260406_223233_sessionseq_single240_verify`

所以低压档结论以复核结果为准。

---

## 3. 最核心结果

## 3.1 `session_seq` 阶段耗时确实明显下降了

### 单聊 `960`

原版：

1. `session_seq = 3.057 ms`
2. `total = 11.947 ms`

实验版：

1. `session_seq = 0.186 ms`
2. `total = 5.933 ms`

### 群聊 `5760`

原版：

1. `session_seq = 2.526 ms`
2. `total = 11.728 ms`

实验版：

1. `session_seq = 0.074 ms`
2. `total = 5.138 ms`

### 群聊 `11520`

原版：

1. `session_seq = 3.811 ms`
2. `total = 13.744 ms`

实验版：

1. `session_seq = 0.080 ms`
2. `total = 5.100 ms`

这说明：

**`session_seq` 里的 MySQL floor 查询确实很重，不是小噪音。**

---

## 3.2 群聊改善非常明显

### 群聊 `5760`

原版：

1. `observed = 1897.939 delivery/s`
2. `p95 = 13125.0 ms`

实验版：

1. `observed = 3914.862 delivery/s`
2. `p95 = 717.1 ms`

### 群聊 `11520`

原版：

1. `observed = 1681.775 delivery/s`
2. `p95 = 39712.1 ms`

实验版：

1. `observed = 3995.107 delivery/s`
2. `p95 = 6097.0 ms`

这说明：

**群聊平台被 `session_seq` 拖慢得很明显。**

把这一步打掉以后：

1. 吞吐大幅上升
2. 长尾明显下降

---

## 3.3 单聊中压档改善明显

### 单聊 `960`

原版：

1. `observed = 178.607 msg/s`
2. `p95 = 29465.9 ms`

实验版：

1. `observed = 323.551 msg/s`
2. `p95 = 11276.65 ms`

这说明：

**单聊在中压档，`session_seq` 也是重要瓶颈。**

---

## 3.4 单聊高压档没有完全解决

### 单聊 `2880`

原版：

1. `observed = 293.201 msg/s`
2. `success = 0.918542`
3. `p95 = 59489.4 ms`

实验版：

1. `observed = 264.857 msg/s`
2. `success = 0.840458`
3. `p95 = 58829.5 ms`

这说明：

1. `session_seq` 不是单聊高压下的唯一瓶颈
2. 它去掉以后，别的瓶颈立刻顶上来了

从阶段耗时看，下一块更明显的木板就是：

`mysql_persist`

因为实验版单聊 `2880` 下：

1. `mysql_persist = 9.223 ms`
2. `total = 9.996 ms`

已经非常接近主耗时了。

---

## 3.5 单聊 `240` 主实验那条异常，不算正式结论

主实验里的单聊 `240` 跑出过一次异常：

1. `success = 0.6`
2. `observed = 16.23 msg/s`

这个结果和整体趋势完全不一致，所以我补跑了单档复核。

复核结果是：

1. `observed = 191.079 msg/s`
2. `success = 1.0`
3. `p95 = 62.05 ms`

所以这里可以明确：

**单聊 `240` 主实验那条是异常样本，不作为正式结论。**

正式结论以复核结果为准。

---

## 4. 这次结论

这次排查可以直接下这几个结论：

### 结论 1

`session_seq` 里的 MySQL `MAX(session_seq)` 查询，已经被坐实是当前瓶颈之一。

### 结论 2

它对群聊影响尤其大，群聊吞吐和 `p95` 改善非常明显。

### 结论 3

它对单聊中压档也有明显帮助。

### 结论 4

单聊高压档没有被彻底救回来，说明 `mysql_persist` 已经成为下一块主要木板。

---

## 5. 下一步怎么做

下一步不要继续纠结 `session_seq` 了，方向已经很清楚：

**直接继续查 `mysql_persist`。**

建议下一步做同样的实验法：

1. 先旁路真实落库
2. 跑同样的定位档位
3. 看吞吐和 `p95` 能不能继续明显改善

如果改善明显，就可以把下一块主瓶颈继续坐实。

---

## 6. 一句话总结

这次实验说明：`session_seq` 里“每条消息查一次 MySQL `MAX(session_seq)`”确实在拖慢系统，已经被坐实是瓶颈之一；但它不是最后一块木板，特别是在单聊高压下，下一块更明显的瓶颈已经变成了 `mysql_persist`。

---

## 7. 本实验对照结论

这里要特别说明：

**这一节不是在更新全局“当前系统真实容量”。**

它只是在说明：

**在这次 `session_seq` 对照实验里，只旁路 `session_seq` 之后，实验组相对本轮对照组提升到了什么水平。**

如果看“当前系统到目前为止对外统一记录的容量口径”，还是以这份总报告为准：

[第一次优化Kafka优化阶段总结第一次.md](/workspace/czk/Personal/EchoChat/docs/kafka_plan/第一次优化Kafka优化阶段总结第一次.md)

也就是：

1. 单聊真实容量：约 `350 msg/s`
2. 群聊真实容量：约 `4050 delivery/s`
3. 群聊折算消息数：约 `169 group_msg/s`

这次 `session_seq` 实验本身，可以先记成下面这组“实验对照结果”：

1. 单聊真实容量：约 `324 msg/s`
2. 相比这轮原版单聊平台约 `293 msg/s`，提升约 `10.4%`
3. 群聊真实容量：约 `3995 delivery/s`
4. 相比这轮原版群聊平台约 `1898 delivery/s`，提升约 `110.5%`
5. 群聊折算消息数：约 `166 group_msg/s`
6. 相比这轮原版群消息平台约 `79 group_msg/s`，提升约 `110.5%`

这里第 5 条的换算方式还是按当前压测口径：

1. 一个 `25` 人群
2. 发送者自己不算接收者
3. 所以 1 条群消息约等于 `24` 个 delivery

因此：

`3995 / 24 ≈ 166`

所以这里更准确的理解应该是：

1. `session_seq` 这一步确实把群聊平台抬高了很多
2. 单聊也有提升，但提升幅度没有群聊那么大
3. 单聊高压档之所以没彻底起来，是因为后面的 `mysql_persist` 很快又顶上来了
4. 它不是在推翻前面已经得到的 `350 / 4050 / 169`，而是在说明 `session_seq` 单独拿掉以后，对照实验里贡献了多少提升
