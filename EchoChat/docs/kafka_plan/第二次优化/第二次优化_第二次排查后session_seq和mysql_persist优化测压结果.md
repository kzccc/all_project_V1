# 第二次优化：第二次排查后 `session_seq` 和 `mysql_persist` 优化测压结果

## 1. 这次做了什么

这次按落地计划，真正改了两块：

### 1.1 `session_seq` 正式化

改动文件：

1. [message_sequence.go](/workspace/czk/Personal/EchoChat/internal/service/chat/message_sequence.go)
2. [redis_service.go](/workspace/czk/Personal/EchoChat/internal/service/redis/redis_service.go)
3. [conversation_sequence.go](/workspace/czk/Personal/EchoChat/internal/model/conversation_sequence.go)
4. [gorm.go](/workspace/czk/Personal/EchoChat/internal/dao/gorm.go)

实际落地内容：

1. 不再走“每条消息查一次 `message` 表 `MAX(session_seq)`”
2. 改成 Redis 主分配 `session_seq`
3. Redis 初始化改成 Lua 原子脚本
4. 新增 `conversation_sequence` 高水位表
5. 历史会话第一次命中时，只做一次旧表 floor 迁移
6. 后续通过高水位回刷，避免再扫消息大表

### 1.2 `mysql_persist` 优化

改动文件：

1. [kafka_message_support.go](/workspace/czk/Personal/EchoChat/internal/service/chat/kafka_message_support.go)
2. [gorm.go](/workspace/czk/Personal/EchoChat/internal/dao/gorm.go)
3. [config.go](/workspace/czk/Personal/EchoChat/internal/config/config.go)
4. [config_local.toml](/workspace/czk/Personal/EchoChat/configs/config_local.toml)
5. [config.toml](/workspace/czk/Personal/EchoChat/configs/config.toml)

实际落地内容：

1. `gorm` 打开 `SkipDefaultTransaction`
2. `gorm` 打开 `PrepareStmt`
3. `mysql_persist` 改成分 worker 的批量落库器
4. 按会话 scope 把消息分流到不同 worker，避免单 worker 串住多分区单聊
5. 正常路径只走 insert
6. 只有 duplicate 才回退到慢路径查重

---

## 2. 前后的区别是什么

优化前：

1. `session_seq` 每条消息都可能打一次 MySQL floor 查询
2. `mysql_persist` 每条消息都同步单条写库
3. 多分区单聊最终会在 MySQL 固定成本上堆积
4. `message` 大表同时承担历史消息存储和序号恢复职责

优化后：

1. `session_seq` 热路径基本只打 Redis
2. MySQL 不再承担每条消息的序号 floor 查询
3. `mysql_persist` 变成按 worker 分流的更轻写库路径
4. duplicate 才走慢路径，不再每条消息都“插完再查”
5. 旧会话的 floor 迁移从“每条消息一次”变成“每个会话第一次一次”

---

## 3. 这次怎么验收的

代码验收：

1. `gofmt`
2. `go test ./...`
3. `go build ./cmd/echo_chat_server`

压测前定位验收：

1. 跑了一轮定位压测：
   [diagnostic_stages_kafka_20260407_142944_post_fix_check_v3](/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/diagnostic_stages_kafka_20260407_142944_post_fix_check_v3)
2. 单聊 `2880` 恢复到 `353.308 msg/s`
3. 群聊 `5760` 到 `4358.068 delivery/s`
4. 群聊 `11520` 到 `4672.901 delivery/s`

最终常规测压：

1. 跑的是常规容量脚本
2. 结果目录：
   [throughput_capacity_kafka_20260407_143241_second_opt_sessionseq_mysql_final](/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/throughput_capacity_kafka_20260407_143241_second_opt_sessionseq_mysql_final)

---

## 4. 当前可直接记录的容量结论

截至这版压测结果，可以先把当前容量结论记录成：

1. 单聊真实容量：约 `349 msg/s`
2. 群聊真实容量：约 `4746 delivery/s`
3. 群聊折算消息数：约 `198 group_msg/s`

这里第 3 条的换算方式还是当前压测口径：

1. 一个 `25` 人群
2. 发送者自己不算接收者
3. 所以 1 条群消息约等于 `24` 个 delivery

因此：

`4745.605 / 24 ≈ 197.7`

也就是约 `198 group_msg/s`。

---

## 5. 和上一版基线比，提升了多少

上一版对外统一口径是：

1. 单聊真实容量：约 `350 msg/s`
2. 群聊真实容量：约 `4050 delivery/s`
3. 群聊折算消息数：约 `169 group_msg/s`

这次新结果对比下来：

1. 单聊：`350 -> 349 msg/s`
2. 单聊变化：约 `-0.7%`
3. 群聊：`4050 -> 4746 delivery/s`
4. 群聊提升：约 `+17.1%`
5. 群聊折算消息数：`169 -> 198 group_msg/s`
6. 群消息提升：约 `+17.1%`

可以直接理解成：

1. 单聊这轮基本持平，没有拿到明显抬升
2. 群聊这轮已经拿到了比较明确的吞吐提升

---

## 6. 为什么会出现这种结果

这次结果的核心特点是：

### 6.1 `session_seq` 确实被明显打轻了

从最终常规测压里看：

1. 单聊 `2880` 档，`session_seq` 已经降到非常低
2. 群聊 `11520` 档，`session_seq` 也已经降到非常低

也就是说：

**`session_seq` 这块优化是生效的。**

### 6.2 群聊的提升更明显

最终常规测压里：

1. 群聊最佳稳定档位从原来的 `5760 offered` 附近，上移到了 `11520 target / 12000 offered`
2. `observed` 从约 `4050 delivery/s` 提到了约 `4746 delivery/s`

这说明：

**群聊这轮的瓶颈确实被向后推了一截。**

### 6.3 单聊没有明显抬升

虽然定位压测里的单聊 `2880` 能跑到 `353.308 msg/s`，但最终常规测压口径下，单聊真实容量还是约 `349 msg/s`，和上一版基本持平。

这说明：

**单聊这轮里，`session_seq + mysql_persist` 虽然已经打轻了，但新的限制项没有被一起拿掉。**

也就是说：

1. 这两块不是没优化成功
2. 而是单聊当前已经被后面的其他固定成本继续卡住了

---

## 7. 这次测压里几个最关键的数字

### 单聊

最终最佳稳定档：

1. `target = 2880`
2. `offered = 3000.0 msg/s`
3. `observed = 348.56 msg/s`
4. `success = 1.0`
5. `p95 = 54038.2 ms`

说明：

1. 单聊真实吞吐平台仍然大约在 `350 msg/s`
2. 单聊长尾依然很重

### 群聊

最终最佳稳定档：

1. `target = 11520`
2. `offered = 12000.0 delivery/s`
3. `observed = 4745.605 delivery/s`
4. `coverage = 1.0`
5. `full_coverage = 1.0`
6. `p95 = 5913.0 ms`

说明：

1. 群聊平台已经从约 `4050 delivery/s` 抬到约 `4746 delivery/s`
2. 更高档 `23040` 时吞吐没有继续明显升高，说明新的平台大约已经出现在 `4700+ delivery/s`

---

## 8. 这次可以收下来的结论

这次可以明确收下这几个结论：

### 结论 1

`session_seq` 正式化方案已经落地成功，而且热路径明显变轻了。

### 结论 2

`mysql_persist` 这一轮改造后，群聊吞吐有了明确提升，说明这条路是对的。

### 结论 3

单聊真实容量这轮没有明显继续抬高，说明单聊当前新的主限制项已经不再只是 `session_seq` 和 `mysql_persist`。

### 结论 4

后续如果要继续抬单聊平台，排查重点应该继续往：

1. 单聊 websocket 分发
2. 单聊缓存读写
3. 单聊后续状态路径
4. 单聊更细的 consumer 主链路固定成本

再往下拆。

---

## 9. 一句话总结

这轮优化真正落地后，结果可以概括成一句话：

**`session_seq` 和 `mysql_persist` 这两块已经被有效打轻，群聊真实容量从约 `4050 delivery/s` 提升到了约 `4746 delivery/s`，提升约 `17.1%`；单聊真实容量仍然大约在 `350 msg/s`，说明单聊后面还有新的主瓶颈需要继续拆。**

---

## 10. 继续往下拆后，又额外做了什么

在上面这版结果基础上，我继续往下拆，最后又补了两块：

### 10.1 `message` 表查询和索引模型收敛

改动文件：

1. [message.go](/workspace/czk/Personal/EchoChat/internal/model/message.go)
2. [message_service.go](/workspace/czk/Personal/EchoChat/internal/service/gorm/message_service.go)
3. [message_sequence.go](/workspace/czk/Personal/EchoChat/internal/service/chat/message_sequence.go)
4. [server.go](/workspace/czk/Personal/EchoChat/internal/service/chat/server.go)
5. [kafka_server.go](/workspace/czk/Personal/EchoChat/internal/service/chat/kafka_server.go)
6. [kafka_message_support.go](/workspace/czk/Personal/EchoChat/internal/service/chat/kafka_message_support.go)
7. [gorm.go](/workspace/czk/Personal/EchoChat/internal/dao/gorm.go)

实际落地内容：

1. 给 `message` 表新增 `conversation_key`
2. 单聊和群聊查询统一收敛到 `conversation_key + session_seq`
3. 启动时回填旧消息的 `conversation_key`
4. 创建复合索引 `idx_message_conversation_seq`
5. 删除旧的 `send_id / receive_id / session_id / session_seq` 单列索引

### 10.2 `status=sent` 改成异步批量更新

改动文件：

1. [client.go](/workspace/czk/Personal/EchoChat/internal/service/chat/client.go)
2. [status_update.go](/workspace/czk/Personal/EchoChat/internal/service/chat/status_update.go)

实际落地内容：

1. websocket 写协程不再同步执行 `UPDATE message SET status=sent`
2. 改成后台多 worker 批量更新
3. 写协程只负责真正把消息写给前端，再把 uuid 投递给状态更新器

---

## 11. 继续优化后的关键结果

继续优化后，我先跑了定位压测：

[diagnostic_stages_kafka_20260407_152611_convkey_status_async](/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/diagnostic_stages_kafka_20260407_152611_convkey_status_async)

最关键的几个结果是：

1. 单聊 `2880`：`observed = 384.331 msg/s`
2. 群聊 `5760`：`observed = 4482.778 delivery/s`
3. 群聊 `11520`：`observed = 5509.038 delivery/s`

然后又补跑了一轮完整常规测压：

[throughput_capacity_kafka_20260407_152912_second_opt_convkey_status_final](/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/throughput_capacity_kafka_20260407_152912_second_opt_convkey_status_final)

这一轮新的容量口径可以记录成：

1. 单聊真实容量：约 `387 msg/s`
2. 群聊真实容量：约 `5409 delivery/s`
3. 群聊折算消息数：约 `225 group_msg/s`

这里第 3 条换算方式还是：

`5408.771 / 24 ≈ 225.4`

也就是约 `225 group_msg/s`。

---

## 12. 和最初基线比，现在总共提升了多少

最初统一基线口径是：

1. 单聊真实容量：约 `350 msg/s`
2. 群聊真实容量：约 `4050 delivery/s`
3. 群聊折算消息数：约 `169 group_msg/s`

继续优化后的新结果是：

1. 单聊：`350 -> 387 msg/s`
2. 单聊提升：约 `+9.3%`
3. 群聊：`4050 -> 5409 delivery/s`
4. 群聊提升：约 `+20.8%`
5. 群聊折算消息数：`169 -> 225 group_msg/s`
6. 群消息提升：约 `+20.8%`

这说明：

1. `conversation_key` 和异步 `status=sent` 对单聊终于开始产生了明确提升
2. 群聊也继续往上抬了一截，而且提升更明显

---

## 13. 继续拆到这一步后的结论

继续往下拆以后，这次可以把结论收得更完整一点：

### 结论 1

`session_seq` 已经不是当前主瓶颈。

### 结论 2

`message` 表原来的索引和查询模型，确实在放大写入成本。

### 结论 3

单聊新的关键限制项之一，确实就是 websocket 写回后同步 `status=sent` 更新。

### 结论 4

把 `conversation_key` 和异步状态更新一起落地后，单聊平台终于从约 `350 msg/s` 抬到了约 `387 msg/s`。

### 结论 5

群聊平台也从约 `4050 delivery/s` 抬到了约 `5409 delivery/s`，已经比最初基线高出约 `20.8%`。

---

## 14. 继续优化 `mysql_persist` 调度后，最新结果

在上面这版基础上，我又继续把 `mysql_persist` 的调度参数做了两项调整：

1. `mysqlPersistFlushIntervalMs` 从 `5ms` 降到 `1ms`
2. `mysqlPersistWorkerCount` 调到 `8`

目的很直接：

1. 降低单聊消息在批量落库器里的等待时间
2. 让单聊多分区、多会话压力下有更高的并发落库能力

对应代码和配置位置：

1. [config.go](/workspace/czk/Personal/EchoChat/internal/config/config.go)
2. [kafka_message_support.go](/workspace/czk/Personal/EchoChat/internal/service/chat/kafka_message_support.go)
3. [config_local.toml](/workspace/czk/Personal/EchoChat/configs/config_local.toml)
4. [config.toml](/workspace/czk/Personal/EchoChat/configs/config.toml)

---

## 15. 这轮调优后的定位压测结果

定位压测目录：

[diagnostic_stages_kafka_20260407_165845_persist_tune_v2](/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/diagnostic_stages_kafka_20260407_165845_persist_tune_v2)

关键结果：

1. 单聊 `2880`：`417.562 msg/s`
2. 单聊 `3360`：`415.268 msg/s`
3. 单聊 `3840`：`421.473 msg/s`
4. 群聊 `5760`：`4482.565 delivery/s`
5. 群聊 `11520`：`5356.01 delivery/s`

从这里已经能看出来：

1. 单聊平台已经开始明显从 `380+` 往 `420` 左右抬
2. 群聊高压平台也依然保持在 `5300+ delivery/s`

---

## 16. 最新常规测压容量结论

我又补跑了一轮完整常规测压：

[throughput_capacity_kafka_20260407_170438_second_opt_persist_tune_final](/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/throughput_capacity_kafka_20260407_170438_second_opt_persist_tune_final)

截至这轮结果，可以把当前最新容量记录成：

1. 单聊真实容量：约 `417 msg/s`
2. 群聊真实容量：约 `5462 delivery/s`
3. 群聊折算消息数：约 `228 group_msg/s`

这里第 3 条还是按当前压测口径换算：

`5462.239 / 24 ≈ 227.6`

也就是约 `228 group_msg/s`。

---

## 17. 和最初基线比，现在总共提升了多少

最初统一基线是：

1. 单聊真实容量：约 `350 msg/s`
2. 群聊真实容量：约 `4050 delivery/s`
3. 群聊折算消息数：约 `169 group_msg/s`

当前最新结果是：

1. 单聊：`350 -> 417 msg/s`
2. 单聊提升：约 `+15.5%`
3. 群聊：`4050 -> 5462 delivery/s`
4. 群聊提升：约 `+33.7%`
5. 群聊折算消息数：`169 -> 228 group_msg/s`
6. 群消息提升：约 `+33.7%`

这说明：

1. 单聊这一轮终于从原来长期卡住的 `350` 平台，抬到了 `410+`
2. 群聊相比最初基线，已经抬到了 `5400+ delivery/s`

---

## 18. 到这一步，当前新的判断

到这一步可以把当前判断收成这样：

### 结论 1

`session_seq` 已经基本退出主瓶颈位置。

### 结论 2

`message` 表模型、同步 `status=sent` 更新、以及 `mysql_persist` 的等待时间，这三块连续打下来以后，单聊和群聊都被继续抬上去了。

### 结论 3

单聊当前真实平台已经从最初约 `350 msg/s` 上移到约 `417 msg/s`。

### 结论 4

群聊当前真实平台已经从最初约 `4050 delivery/s` 上移到约 `5462 delivery/s`。

### 结论 5

当前如果还要继续提单聊，下一步就要继续盯：

1. `mysql_persist` 本身剩下的固定 insert 成本
2. 高频单聊下的长尾积压问题
3. 是否要进一步把消息落库从“同步成功路径”再往外拆

---

## 19. 继续拆解后，为什么单聊还是停在 `417 msg/s` 左右

这一步我又把最新常规测压继续往下拆，重点看了两件事：

1. 单聊消息在 3 个 Kafka 分区上的分布
2. 每个分区里 `session_seq / mysql_persist / total` 的平均耗时

先看最新最佳稳定档：

1. 档位：`single/step_006_expand_target_3840`
2. offered：`3750.0 msg/s`
3. observed：`417.449 msg/s`
4. success：`1.0`

### 19.1 单聊 30 对会话没有均匀落到 3 个分区

在这轮压测里，单聊消息分区分布长期都很稳定：

1. `partition 0`：约 `26.67%`
2. `partition 1`：约 `46.67%`
3. `partition 2`：约 `26.67%`

以最佳稳定档为例：

1. `partition 0`：`8000`
2. `partition 1`：`14000`
3. `partition 2`：`8000`

也就是当前 `30` 对单聊会话，经 `session_id` 哈希以后，实际落成了：

1. 一边各 `8` 对
2. 中间一个热分区 `14` 对

所以单聊不是“3 个分区一起平均打满”，而是：

**先由吃流量最多的那个热分区先打到头。**

### 19.2 `session_seq` 已经不是主耗时了

还是看 `single/step_006_expand_target_3840`：

1. `partition 0`：`session_seq avg = 0.019 ms`
2. `partition 1`：`session_seq avg = 0.018 ms`
3. `partition 2`：`session_seq avg = 0.019 ms`

这个量级已经很低了，说明：

**`session_seq` 现在基本已经退出主瓶颈。**

### 19.3 当前单聊主要还是卡在 `mysql_persist`

同一个档位里：

1. `partition 0`：`mysql_persist avg = 5.891 ms`，`total avg = 5.963 ms`
2. `partition 1`：`mysql_persist avg = 4.846 ms`，`total avg = 4.913 ms`
3. `partition 2`：`mysql_persist avg = 6.172 ms`，`total avg = 6.243 ms`

可以直接看出：

1. `total` 几乎就等于 `mysql_persist`
2. `session_seq` 在总耗时里占比已经很小

这说明当前单聊 consumer 主链路里，真正还在顶着平台的，主要就是：

**`mysql_persist` 的固定写库成本。**

### 19.4 为什么这个结果会刚好落在 `417` 附近

热分区 `partition 1` 当前承担了约 `46.67%` 的单聊流量。

它自己的 `total avg` 大约是 `4.913 ms`，换算成这个分区自己大致能承受的处理能力，大约是：

`1000 / 4.913 ≈ 203.5 msg/s`

再考虑它要扛掉全局约 `46.67%` 的流量，那么反推整个单聊场景的大致平台大约就是：

`203.5 / 0.4667 ≈ 436 msg/s`

这个值和实际压测看到的：

1. 最佳稳定档 `417.449 msg/s`
2. 失败前档位 `4320 -> 436.308 msg/s`
3. 更高档位虽然瞬时 `480+`，但成功率已经掉线

是非常接近的。

所以现在可以把单聊平台的原因收得更明确一点：

**单聊不是又回到了 `session_seq` 问题，而是被“热分区先打满 + `mysql_persist` 还存在固定成本”一起卡在了 `410~430 msg/s` 这一段。**

### 19.5 到这一步，当前最实用的判断

到这里可以把单聊剩余瓶颈总结成一句话：

1. 代码层面，`session_seq` 这条链已经基本压下去了
2. 当前单聊平台更像是 `mysql_persist` 固定成本问题
3. 同时又被 `30` 对会话在 `3` 分区下的 `8 : 14 : 8` 分布放大了

也就是说，后面如果还想继续把单聊从 `417` 往上抬，最有效的方向优先是：

1. 继续降低 `mysql_persist` 单条消息固定成本
2. 验证更均匀的会话分布时，单聊平台能不能继续上移
3. 评估是否增加 topic 分区数，或者调整压测会话规模，减少热分区过早打满

---

## 20. 验证实验：把单聊会话数提到 `60`，Kafka 分区提到 `6`

为了验证上一节的判断，我这次把两个措施一起做了：

1. 压测单聊会话数从 `30` 对提到 `60` 对
2. Kafka `chat_message` topic 分区数从 `3` 提到 `6`

这轮真实压测目录：

[throughput_capacity_kafka_20260408_100431_part6_pair60](/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/throughput_capacity_kafka_20260408_100431_part6_pair60)

### 20.1 这轮压测结果

新的最佳稳定结果是：

1. 单聊真实容量：约 `606 msg/s`
2. 群聊真实容量：约 `5475 delivery/s`
3. 群聊折算消息数：约 `228 group_msg/s`

这里第 3 条还是按当前压测口径换算：

`5474.776 / 24 ≈ 228.1`

也就是约 `228 group_msg/s`。

### 20.2 和上一轮最新结果比，提升了多少

上一轮最新口径是：

1. 单聊真实容量：约 `417 msg/s`
2. 群聊真实容量：约 `5462 delivery/s`
3. 群聊折算消息数：约 `228 group_msg/s`

这次对比下来：

1. 单聊：`417 -> 606 msg/s`
2. 单聊提升：约 `+45.1%`
3. 群聊：`5462 -> 5475 delivery/s`
4. 群聊变化：约 `+0.2%`
5. 群聊折算消息数：`228 -> 228 group_msg/s`
6. 群消息变化：约 `+0.2%`

这说明很清楚：

1. 这两个措施对单聊提升非常明显
2. 对群聊几乎没有影响

### 20.3 为什么这次单聊能明显上去

这轮单聊最佳稳定档是：

1. `target = 4800`
2. `offered = 5000.0 msg/s`
3. `observed = 605.653 msg/s`
4. `success = 1.0`

同时，单聊消息在 `6` 个分区上的分布变成了：

1. `partition 0`：`16.67%`
2. `partition 1`：`25.00%`
3. `partition 2`：`18.33%`
4. `partition 3`：`15.00%`
5. `partition 4`：`16.67%`
6. `partition 5`：`8.33%`

和之前 `3` 分区下的 `26.67% / 46.67% / 26.67%` 相比，最热分区占比已经从 `46.67%` 降到了 `25.00%`。

也就是说：

**热分区先打满的问题，确实被明显缓解了。**

### 20.4 这轮里真正发生了什么

这轮里 `session_seq` 还是很轻：

1. 各分区 `session_seq avg` 基本都在 `0.020 ms` 左右

`mysql_persist` 仍然是主耗时：

1. 最热分区 `partition 1`：`mysql_persist avg = 6.295 ms`
2. 最热分区 `partition 1`：`total avg = 6.371 ms`

虽然单条消息的写库固定成本还在，但因为：

1. 分区更多了
2. 会话更多了
3. 流量分布更均匀了

所以全局单聊平台还是被明显抬高了。

按最热分区反推：

`1000 / 6.371 / 0.25 ≈ 628 msg/s`

这个值和实际压测看到的 `605.653 msg/s` 也很接近。

### 20.5 这轮实验可以收下来的结论

这轮实验已经足够说明：

1. 单聊之前卡在 `417 msg/s`，确实和热分区过热有关
2. 提高 topic 分区数、增加单聊会话规模以后，单聊平台可以明显上移
3. `session_seq` 不是当前主瓶颈
4. `mysql_persist` 仍然是单聊链路里的主要固定成本
5. 群聊因为本质上是单个群会话，同一轮压测还是集中在一个分区，所以这两个措施对群聊提升不明显
