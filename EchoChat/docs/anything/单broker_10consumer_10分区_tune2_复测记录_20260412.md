# 单 broker + 10 consumers + 10 partitions + tune2 复测记录

## 1. 这次复测的目标

在上一轮已经完成：

1. `单 broker`
2. `10 consumers`
3. `240 partitions`
4. `mysql_persist tune2`

的正式复测之后，这次继续按同样口径，只把分区数改成：

1. `10 partitions`

其余配置保持不变。

专用配置文件：

1. [config_local_singlebroker_part10_mysqlpersist_tune2.toml](/workspace/czk/Personal/EchoChat/configs/config_local_singlebroker_part10_mysqlpersist_tune2.toml)

---

## 2. 本次实际配置

固定项如下：

1. Kafka：`127.0.0.1:9092`
2. `messageMode = kafka`
3. `topicPartitions = 10`
4. `topicReplicationFactor = 1`
5. `minInsyncReplicas = 1`
6. `mysqlPersistWorkerCount = 32`
7. `mysqlPersistBatchSize = 256`
8. `mysqlPersistFlushIntervalMs = 5`
9. consumer members：`10`
10. 单聊：`SINGLE_PAIR_COUNT = 60`
11. 群聊：`GROUP_MEMBER_LIMIT = 25`
12. 判通过标准：`p95 < 1000ms`

本次结果目录：

1. [throughput_capacity_kafka_20260412_221954_singlebroker_part10_tune2_members10](/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/throughput_capacity_kafka_20260412_221954_singlebroker_part10_tune2_members10)

---

## 3. 当前已确认结果

### 3.1 单聊

单聊完整搜索已经跑完。

各关键阶段结果如下：

1. `target=120`
   `98.882 msg/s`
   `p95 = 38 ms`
2. `target=240`
   `197.441 msg/s`
   `p95 = 43 ms`
3. `target=480`
   `395.064 msg/s`
   `p95 = 46 ms`
4. `target=960`
   `791.823 msg/s`
   `p95 = 47 ms`
5. `target=1920`
   `1067.443 msg/s`
   `p95 = 3261 ms`
   失败
6. refine `target=1440`
   `1005.276 msg/s`
   `p95 = 586 ms`
   通过
7. refine `target=1680`
   `1072.136 msg/s`
   `p95 = 1806 ms`
   失败
8. refine `target=1560`
   `1039.636 msg/s`
   `p95 = 1524 ms`
   失败
9. refine `target=1500`
   `983.184 msg/s`
   `p95 = 1027.05 ms`
   失败
10. refine `target=1470`
    `1018.126 msg/s`
    `p95 = 851.1 ms`
    通过

所以当前可以确认的单聊结果是：

1. 单聊真实可用容量：约 `1018.126 msg/s`
2. 对应档位：`target = 1470`
3. `p95 = 851.1 ms`

### 3.2 群聊

群聊这轮没有完整收口。

目前已经写出的阶段结果只有前四档：

1. `target=180`
   `151.122 delivery/s`
   `p95 = 5 ms`
2. `target=360`
   `298.677 delivery/s`
   `p95 = 5 ms`
3. `target=720`
   `603.105 delivery/s`
   `p95 = 5 ms`
4. `target=1440`
   `1109.222 delivery/s`
   `p95 = 5 ms`

也就是说，目前只能确认：

1. 群聊至少能稳定通过到 `1109.222 delivery/s`
2. 但更高档位没有完整结果，不能据此认定群聊最终容量

---

## 4. 为什么这轮没有完整收口

这次 10 分区复测发生了两个问题：

1. 第一轮压测运行时间超过当前命令超时，只留下了阶段目录，没有自动生成总报告
2. 在尝试补跑时，MySQL 先出现 `lock wait timeout`，随后 mysqld 进程退出，导致后续群聊结果无法继续补齐

实际遇到的异常包括：

1. `Error 1205 (HY000): Lock wait timeout exceeded`
2. 随后本机 `mysqld` 不再监听 `3306`
3. 再补跑时 seed 和 server 初始化都无法继续依赖 MySQL

所以这次不是“10 分区一定很差”，而是：

**单聊结果已经测实，但群聊完整结果被数据库故障打断。**

---

## 5. 当前能先得出的结论

即使只看已经确认的单聊结果，这次 10 分区也已经说明一件事：

1. 在 `单 broker + 10 consumers + tune2` 不变时
2. 分区数从 `240` 降到 `10`
3. 单聊可用容量从上一轮的 `2503.453 msg/s`
4. 掉到这轮已确认的 `1018.126 msg/s`

这说明：

1. `10 partitions` 明显吃不住 `10 consumers`
2. 单聊并行度被严重压缩
3. 单聊容量相比 `240 partitions` 明显回退

所以即使这轮群聊没完整收口，单聊这一条结论已经足够明确：

**在当前单 broker + 10 consumers + tune2 口径下，`10 partitions` 对单聊来说明显偏少。**

---

## 6. 当前建议

如果后面继续补这条线，建议顺序是：

1. 先恢复 MySQL 服务
2. 再把 `10 partitions` 这轮群聊补完整
3. 然后把 `10 / 20 / 60 / 240` 做成同口径对照

当前这份记录先保留为：

1. `10 partitions` 单聊结果已确认
2. 群聊完整结果待补
