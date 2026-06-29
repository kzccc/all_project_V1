# 单 broker + 10 consumers + 240 partitions + tune2 复测报告

## 1. 这次复测的目的

这次复测要回答的问题很明确：

1. 不走 `5 brokers` 主线
2. 改成 `单 broker`
3. consumer member 数量固定为 `10`
4. `mysql_persist` 采用第三次优化里已经确认的单机停点 `tune2`
5. 继续跑 `240 partitions`

目标是看在这套单机口径下，当前单聊和群聊的可用容量到底是多少。

---

## 2. 这次复测的实际配置

本次使用的专用配置文件是：

1. [config_local_singlebroker_part240_mysqlpersist_tune2.toml](/workspace/czk/Personal/EchoChat/configs/config_local_singlebroker_part240_mysqlpersist_tune2.toml)

关键配置如下：

1. Kafka：`127.0.0.1:9092`
2. `messageMode = kafka`
3. `chatTopic = chat_message_singlebroker_part240_tune2`
4. `topicPartitions = 240`
5. `topicReplicationFactor = 1`
6. `minInsyncReplicas = 1`
7. `mysqlPersistWorkerCount = 32`
8. `mysqlPersistBatchSize = 256`
9. `mysqlPersistFlushIntervalMs = 5`

本次 consumer member 数量为：

1. `10`

实际实例端口为：

1. `18082`
2. `18083`
3. `18084`
4. `18085`
5. `18086`
6. `18087`
7. `18088`
8. `18089`
9. `18090`
10. `18091`

压测口径沿用第三次优化主线：

1. 单聊：`SINGLE_PAIR_COUNT = 60`
2. 群聊：`GROUP_MEMBER_LIMIT = 25`
3. 判通过标准：`p95 < 1000ms`

结果目录：

1. [throughput_capacity_kafka_20260412_214740_singlebroker_part240_tune2_members10](/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/throughput_capacity_kafka_20260412_214740_singlebroker_part240_tune2_members10)

---

## 3. 本次复测结果

### 3.1 单聊

本次单聊最佳通过档是：

1. `target = 3840`
2. 实际发送速率：`3750.0 msg/s`
3. 实际处理速率：`2503.453 msg/s`
4. `success = 1.0`
5. `p95 = 957.05 ms`

可以记录为：

1. 单聊真实可用容量：约 `2503.453 msg/s`

需要注意：

1. 下一档 `target = 7680` 直接失败
2. 后续 refine 的 `5760 / 4800 / 4320 / 4080` 虽然成功率都过线
3. 但 `p95` 分别到了 `5899.05 / 3688.1 / 2111 / 1790 ms`
4. 所以都按当前口径判失败

也就是说，这一轮单聊的稳定停点不是更高 target，而就是：

1. `3840 offered`
2. `2503.453 observed`
3. `p95 仍勉强压在 1s 内`

### 3.2 群聊

本次群聊最佳通过档是：

1. `target = 5760`
2. 实际发送速率：`6000.0 delivery/s`
3. 实际处理速率：`4451.724 delivery/s`
4. `coverage = 1.0`
5. `full_coverage = 1.0`
6. `p95 = 116 ms`

可以记录为：

1. 群聊真实可用容量：约 `4451.724 delivery/s`

按 `24` 个接收者折算：

1. `4451.724 / 24 = 185.49 group_msg/s`

需要注意：

1. 下一档 `target = 11520`
2. 实际处理速率虽然到 `4839.816 delivery/s`
3. 但 `p95 = 5815 ms`
4. 所以按当前口径直接失败

所以这轮群聊的可用停点仍然记在：

1. `5760 target`
2. `4451.724 delivery/s`
3. `p95 = 116 ms`

---

## 4. 和第三次优化主线结果对比

第三次优化总结里，当前主线推荐口径是：

1. `5 broker + 240 partitions + mysql_persist tune2 + 10 members`
2. 单聊：`2356.15 msg/s`
3. 群聊：`4099.707 delivery/s`
4. 群聊折算：`170.8 group_msg/s`

本次单 broker 复测结果是：

1. 单聊：`2503.453 msg/s`
2. 群聊：`4451.724 delivery/s`
3. 群聊折算：`185.49 group_msg/s`

对比第三次主线：

1. 单聊：`2356.15 -> 2503.453`
   提升约 `6.25%`
2. 群聊：`4099.707 -> 4451.724`
   提升约 `8.59%`
3. 群聊折算：`170.8 -> 185.49`
   提升约 `8.60%`

这说明至少在当前这次复测环境下：

1. `单 broker + 10 consumers + tune2 + 240 partitions`
2. 并没有比第三次 `kafka5` 主线更差
3. 单聊和群聊的过线值都略高一些

但这里要强调：

1. 这不是严格意义上的“单 broker 必然优于 5 brokers”结论
2. 它更像是当前仓库状态、当前单机环境、当前这轮复测条件下的一次实测结果
3. 真正可靠的工程判断，仍然要结合后续重复跑和诊断指标看

---

## 5. 和第二次优化末尾 `240` 分区结果对比

第二次优化末尾，在 `240` 分区常规一键脚本 + `p95 < 1s` 口径下，记录的是：

1. 单聊：`2372.169 msg/s`
2. 群聊：`4464.231 delivery/s`
3. 群聊折算：约 `186 group_msg/s`

本次结果：

1. 单聊：`2503.453 msg/s`
2. 群聊：`4451.724 delivery/s`
3. 群聊折算：`185.49 group_msg/s`

对比第二次那组 `240` 分区结果：

1. 单聊：`2372.169 -> 2503.453`
   提升约 `5.53%`
2. 群聊：`4464.231 -> 4451.724`
   下降约 `0.28%`

这说明：

1. 单聊这次确实比第二次末尾的 `240` 分区结果更高
2. 群聊则基本打平

所以如果只看“当前单 broker + 10 consumers”这条线，可以先记成：

1. 单聊有新增量
2. 群聊基本回到第二次 `240` 分区那组水平

---

## 6. 这次结果该怎么理解

### 6.1 单聊

单聊最重要的现象是：

1. 在 `3840 offered` 档
2. 系统还能稳定处理到 `2503.453 msg/s`
3. 但 `p95` 已经到 `957.05 ms`

这说明：

1. 单聊这轮已经很接近当前 `p95 < 1s` 口径下的边界
2. 再往上 target 虽然还能送达很多消息
3. 但尾延迟会迅速爆开

也就是说，这一轮单聊不是“完全还有很大余量”，而是：

**已经把当前可用平台打得比较实了。**

### 6.2 群聊

群聊最重要的现象是：

1. `5760` 档通过时，`p95` 只有 `116 ms`
2. 但一到 `11520` 档，`p95` 直接跳到 `5815 ms`

这说明：

1. 群聊当前不是逐步缓慢恶化
2. 而是到某个压力点后，会明显进入排队/积压区间

所以这轮群聊的“可用容量”也很好记：

1. `4451.724 delivery/s`
2. 折算约 `185.49 group_msg/s`

---

## 7. 这次复测的结论

如果把这次复测压成几句话，就是：

1. 在 `单 broker + 10 consumers + 240 partitions + mysql_persist tune2` 口径下，本次单聊真实可用容量测到 `2503.453 msg/s @ p95 957.05ms`
2. 同口径下，本次群聊真实可用容量测到 `4451.724 delivery/s @ p95 116ms`，折算约 `185.49 group_msg/s`
3. 相比第三次优化主线 `5 broker + 10 members`，这次单聊提升约 `6.25%`，群聊提升约 `8.59%`
4. 相比第二次优化末尾 `240` 分区结果，这次单聊提升约 `5.53%`，群聊基本打平

如果只看当前这一次实测结果，可以先把这条线记成：

1. 单 broker 版本并没有天然比 `kafka5` 主线差
2. 在当前机器和当前仓库状态下，它反而给出了更高一点的单聊过线值
3. 群聊则大体回到了第二次 `240` 分区工程口径的水平

---

## 8. 结果引用

本次正式结果引用文件：

1. [report.md](/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/throughput_capacity_kafka_20260412_214740_singlebroker_part240_tune2_members10/report.md)
2. [summary.json](/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/throughput_capacity_kafka_20260412_214740_singlebroker_part240_tune2_members10/summary.json)
3. [metadata.json](/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/throughput_capacity_kafka_20260412_214740_singlebroker_part240_tune2_members10/metadata.json)
