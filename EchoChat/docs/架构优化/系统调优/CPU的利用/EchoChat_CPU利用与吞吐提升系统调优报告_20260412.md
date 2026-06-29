# EchoChat CPU 利用与吞吐提升系统调优报告

生成时间：2026-04-12  
适用范围：`/workspace/czk/Personal/EchoChat`  
目标：在**不先改软件架构**的前提下，分析当前 `EchoChat` 为什么没有把 `32C` 机器的 CPU 能力有效转成吞吐，并给出优先级明确的系统层优化方案。

---

## 1. 这份报告回答什么

这份报告只看一个维度：

1. 当前机器有 `32` 个逻辑 CPU，为什么吞吐没有随之拉高。
2. 在不先动业务架构的前提下，哪些 CPU 相关系统调优最可能带来真实吞吐收益。
3. 哪些 CPU 相关手段看起来“高级”，但对你当前压测形态收益有限。
4. 应该先怎么做实验，才能把“CPU 更忙”转成“吞吐更高、p95 更稳”。

这份报告**不**试图回答：

1. Kafka 分区数的最终工程默认值。
2. 群聊热点单 key 的根治方案。
3. `mysql_persist` 解耦、群聊 key 分片、轻重链路拆分这类架构改造。

这些很重要，但这里先刻意不展开。

---

## 2. 先给结论

先把最核心判断讲清楚：

1. 当前 `EchoChat` 没有把 `32C` 机器吃满，**不是因为 `GOMAXPROCS` 没开够**。
2. 当前更大的问题是：**多个进程都默认认为自己可以吃满 32 核，最终在同一台机器上形成了 CPU 过度竞争和调度噪声。**
3. 真正最值得先做的 CPU 向优化，不是先碰 `IRQ/RPS/XPS/NUMA`，而是：
   - 多实例场景下重新分配每个进程可见 CPU
   - 限制每个 `EchoChat` 实例的 `GOMAXPROCS`
   - 给 `EchoChat / Kafka / MySQL / Redis` 做基础的 CPU 隔离
4. `IRQ 亲和性`、`NIC 队列`、`RPS/XPS` 是正确方向，但对你当前大量**本机回环 / 单机同宿主机压测**场景，收益优先级明显低于前面三项。
5. `NUMA` 很重要，但前提是这台机器真的暴露了多个 NUMA node；如果是单 NUMA 或虚拟化环境已屏蔽，多数 NUMA 调优动作收益有限。

一句话总结：

**当前最有价值的 CPU 优化，不是“让单个进程更能跑”，而是“让整台机器上的多个进程别互相抢坏”。**

---

## 3. 当前仓库里能确认的 CPU 事实

这里不靠想象，先看现有证据。

### 3.1 Go 运行时已经默认把 `GOMAXPROCS` 开到 32

现有 metrics 里已经能看到：

1. `go_sched_gomaxprocs_threads = 32`
2. `go_threads ≈ 38`
3. `go_goroutines ≈ 1746 ~ 1860`

可参考：

1. `docs/k6_message_test/records/diagnostic_stages_kafka_20260409_114356_thirdopt_exec1_baseline_diag/group/target_5040/metrics.prom`
2. `docs/k6_message_test/records/diagnostic_stages_kafka_20260409_142259_thirdopt_exec1_tune2_diag/single/target_2280/metrics.prom`

这说明至少一件事：

1. 单个 `EchoChat` 进程并不是因为 `GOMAXPROCS` 太小而吃不到 CPU。

也就是说，**“把单个进程的 GOMAXPROCS 从 32 改到更大”没有意义。**

### 3.2 当前多实例压测时，单进程仍然默认认为自己有 32 个可并行执行位

第三次优化 `members10` 的压测本质上是：

1. 起 `10` 个 `EchoChat` 进程
2. 它们共享一个 Kafka consumer group
3. 每个进程默认仍然看到 `32` 个 CPU

相关压测结果：

1. `docs/k6_message_test/records/throughput_capacity_kafka_20260409_175745_thirdopt_exec3_members10/summary.json`

这个组合的隐含问题是：

1. `10` 个 Go 进程
2. 每个进程 `GOMAXPROCS = 32`
3. 同时机器上还有 `5` 个 Kafka broker、`MySQL`、`Redis`、日志写盘、压测工具

于是整台机器实际上变成了：

1. 大量进程都在竞争同一组 CPU
2. Linux 调度器需要持续在这些 runnable task 之间做切换
3. 用户态逻辑并没有因此获得等比例吞吐收益，反而更容易把 `p95` 拉坏

这就是当前 CPU 维度最值得盯的点。

### 3.3 当前吞吐平台更像“等待链路被打满”，不是“纯计算核被打满”

现有链路和指标都说明：

1. 单聊可用容量大约在 `2.3k msg/s`
2. 群聊可用容量大约在 `4.1k delivery/s`
3. `mysql_persist` 在很多 partition 上几乎占据了 `total` 的绝大部分时间

例如在：

1. `docs/k6_message_test/records/diagnostic_stages_kafka_20260409_142259_thirdopt_exec1_tune2_diag/single/target_2280/metrics.prom`

里可以看到多个 partition 上：

1. `mysql_persist stage sum` 非常接近 `total stage sum`
2. 这说明该消息处理并不是在大量做纯 CPU 计算
3. 而是在等待落库、批量写、下游处理完成

因此这里有个很重要的判断：

**你当前的 CPU 优化，目标不是“把单条消息算得更快”，而是“减少无意义调度竞争，让 CPU 时间更集中地花在真正有效的 runnable 路径上”。**

---

## 4. 为什么 32 核没有自然变成更高吞吐

这里把原因拆成 5 个层次。

### 4.1 多实例默认全核可见，导致 CPU 超卖

这是当前最可能直接影响吞吐的 CPU 问题。

在单机上同时运行：

1. `10` 个 `EchoChat`
2. `5` 个 Kafka broker
3. `MySQL`
4. `Redis`
5. 压测工具和采样工具

如果这些进程都默认可见全部 `32` 核，那么调度器视角就是：

1. 每个进程都可以把任何工作扔给任何核
2. 同类工作不会自然聚集
3. cache locality 变差
4. 上下文切换和 runqueue 抖动变多
5. 热核和冷核分布不稳定

这类问题在“吞吐看着还能跑、但 p95 难压稳”的场景里尤其常见。

### 4.2 Go 运行时线程数不高，不代表 CPU 利用质量高

从现有指标看：

1. `go_threads ≈ 38`
2. 并没有爆出几百上千线程

这说明：

1. 问题不是 Go runtime 线程失控

但它**不**说明：

1. 这 38 个线程已经以最高质量利用了 32 核

因为 CPU 利用质量还受到：

1. 线程被调度到哪些核
2. 是否频繁跨核迁移
3. 进程之间是否在抢共享 cache
4. 是否和 Kafka / MySQL / Redis 抢同一批核
5. runqueue 是否出现局部热点

### 4.3 当前链路是“阻塞型多阶段链路”，不是纯 CPU 型任务

你现在的主链路里有：

1. Kafka produce / consume
2. Redis
3. MySQL
4. WebSocket
5. JSON

这意味着：

1. 很多 goroutine 是 runnable 和 waiting 混合
2. CPU 空闲不等于还能继续线性扩吞吐
3. CPU 忙也不等于忙在最值得的路径上

所以 CPU 调优的重点必须从：

1. “让 CPU 百分比看起来更高”

转到：

1. “让最热路径拿到更稳定、更低噪声的 CPU 时间片”

### 4.4 当前同机压测对网卡路径优化不敏感

`IRQ 亲和性`、`RSS/RPS/XPS`、NIC 队列优化这些都很有价值，但要看流量形态。

如果你当前很多压测流量是：

1. `127.0.0.1`
2. 单机回环
3. 本机多个进程互打

那么：

1. 真实物理网卡收包路径并不是主角
2. NIC 队列和中断分发收益会被显著削弱

所以：

1. 对当前同机压测，优先级应低于 `cpuset + GOMAXPROCS + 进程隔离`
2. 对未来跨机压测或正式环境流量，优先级会升高

### 4.5 NUMA 可能重要，但不是默认有收益

NUMA 调优只有在下面条件成立时才很值钱：

1. 机器确实有多个 NUMA node
2. `EchoChat / Kafka / MySQL` 会明显跨 node 分配内存
3. 远端内存访问已带来可观延迟

如果当前机器是：

1. 单 NUMA
2. 或云厂商已经把 CPU / memory topology 屏蔽得很平

那么直接上 NUMA 绑定，不一定有收益。

所以 NUMA 应该是：

1. 先确认
2. 再行动

而不是先配。

---

## 5. 哪些 CPU 调优最可能带来吞吐收益

这里按收益优先级排序。

## 5.1 第一优先级：给 `EchoChat` 实例降 `GOMAXPROCS`

这是最值得立刻验证的 CPU 优化项。

### 为什么它可能有效

当前单机 `members10` 场景里：

1. `10` 个 `EchoChat` 进程都默认 `GOMAXPROCS=32`
2. 但机器总共也就 `32` 核
3. 同时还要留核给 Kafka / MySQL / Redis / 系统线程

所以这更像是：

1. 同一台机器上有大量 Go 进程都在“过度乐观地并行”
2. 结果不是吞吐倍增，而是调度竞争上升

### 为什么它可能比“继续加实例”更值钱

如果不先约束每实例的 CPU 并发度：

1. 继续加 member 很容易先加出调度噪声
2. 而不是加出稳定吞吐

这时你会看到：

1. 成功率仍然过线
2. 吞吐小涨或不涨
3. `p95/p99` 反而更容易抖

### 建议起手实验

对于单机全栈环境，建议不要再默认 `32`。

可以做一轮固定其他变量不变的实验：

1. `members = 10`
2. 分别测试 `GOMAXPROCS = 1 / 2 / 3 / 4`
3. 每轮都记录：
   - 单聊吞吐
   - 群聊吞吐
   - `p95/p99`
   - `process_cpu_percent`
   - `go_threads`
   - `run queue` 相关系统指标

### 我的预判

在你当前这种：

1. 多进程
2. 重 IO
3. 同机 Kafka + MySQL + Redis

的组合里，`GOMAXPROCS=2~4` 很可能比 `32` 更合理。

尤其是：

1. 单进程并不长期做纯计算
2. 主要是在多段链路中等待和搬运

这时过高的 `GOMAXPROCS` 更容易放大调度成本。

### 落地方式

最简单的是：

1. 启动前设置环境变量 `GOMAXPROCS`

例如：

```bash
GOMAXPROCS=2 ./bin/echo_chat_server
```

如果你走 systemd：

```ini
[Service]
Environment=GOMAXPROCS=2
```

---

## 5.2 第二优先级：给 `EchoChat / Kafka / MySQL / Redis` 做 CPU 隔离

### 为什么它比 IRQ / NIC 更优先

因为你当前同机压测的更大问题是：

1. 多类进程在抢同一组核

而不是：

1. 物理网卡收包队列不够多

### 目标

让这几类进程尽量少互抢：

1. `EchoChat`
2. `Kafka brokers`
3. `MySQL`
4. `Redis`
5. 系统与中断线程

### 一个适合当前单机实验的示例分配

假设总共 `32` 个逻辑核，先给一个**实验用**而不是绝对正确的示例：

1. `CPU 0-1`：系统、ksoftirq、杂项后台线程
2. `CPU 2-7`：MySQL
3. `CPU 8-15`：Kafka 五个 broker
4. `CPU 16-17`：Redis 与轻量后台任务
5. `CPU 18-31`：`EchoChat` 多实例

这不是唯一答案，但它有两个价值：

1. 把最核心的冲突先拆开
2. 让后续观察更可解释

### `EchoChat` 多实例如何分

如果 `EchoChat` 有 `10` 个实例，而你给了它 `14` 个核：

1. 可以先不做“每实例独占固定 1 核”的死绑定
2. 而是让它们共享 `18-31`
3. 同时把 `GOMAXPROCS` 限在 `1~2`

这样通常比：

1. 所有实例全机 32 核可见

更容易得到稳定吞吐。

### 落地方式

临时实验可以用：

```bash
taskset -c 18-31 GOMAXPROCS=2 ./bin/echo_chat_server
```

长期运行建议改 systemd：

```ini
[Service]
CPUAffinity=18 19 20 21 22 23 24 25 26 27 28 29 30 31
Environment=GOMAXPROCS=2
```

Kafka / MySQL / Redis 也可以分别配置 `CPUAffinity`。

### 预期收益

这类优化最可能带来的不是“峰值吞吐暴涨”，而是：

1. `p95/p99` 稳定
2. 多实例下吞吐不再轻易回退
3. CPU 使用率更接近“热核持续忙、冷核有节制”，而不是全机乱抖

---

## 5.3 第三优先级：按实例数重算“每个实例应该吃多少核”

这件事和 `GOMAXPROCS` 有关，但比单纯降它更进一步。

### 核心思想

`consumer members` 增加时，不能只扩大实例数，还要同步思考：

1. 每个实例分到多少 partition
2. 每个实例实际应该拥有多少 CPU 执行预算

否则容易出现：

1. member 变多了
2. claim 变细了
3. 但每实例仍然全核并发
4. 最终 CPU 调度成本抵消了 claim 变细的收益

### 建议公式

对于单机全栈实验，先估算：

1. `chat_cpu_budget = total_cpu - mysql_budget - kafka_budget - redis_budget - system_budget`
2. `per_instance_budget = floor(chat_cpu_budget / chat_instance_count)`

然后让：

1. `GOMAXPROCS <= per_instance_budget`

例如：

1. 总核数 `32`
2. MySQL 预留 `6`
3. Kafka 预留 `8`
4. Redis + 系统预留 `4`
5. 剩给 `EchoChat` 约 `14`
6. 如果 `members=10`
7. 那每实例预算只有 `1~2` 核

这时每实例继续放 `32` 个 P，明显是不合理的。

---

## 5.4 第四优先级：确认并优化 NUMA 本地性

### 什么时候值得做

先执行：

```bash
numactl --hardware
lscpu | rg -n "NUMA|Socket|Core|CPU\\(s\\)"
```

如果看到：

1. `NUMA node(s): 2` 或更多

那这件事就值得做。

### 为什么它会影响吞吐

多 NUMA 机器上，若进程：

1. 在 node 0 上跑
2. 但大量内存页在 node 1

那会出现：

1. 远端内存访问
2. cache miss 成本变高
3. 对重 IO + 高 goroutine 数场景更不友好

### 建议动作

如果确认多 NUMA：

1. `MySQL` 优先单独绑到一个 node
2. `Kafka brokers` 尽量局部化
3. `EchoChat` 多实例按 node 切分

例如：

```bash
numactl --cpunodebind=0 --membind=0 ./bin/echo_chat_server
```

### 预期收益

NUMA 调优通常不会给你“翻倍吞吐”，但会带来：

1. 更稳的尾延迟
2. 更少的跨 node 抖动
3. 多进程并跑时更清晰的 CPU 行为

---

## 5.5 第五优先级：IRQ 亲和性、RSS、RPS、XPS

这块是对的，但当前优先级没那么高。

### 它们解决什么

这些优化主要是让：

1. 收包中断
2. NIC queue
3. 网络软中断处理

更均匀地落到多个核上，减少单核网络热点。

### 为什么当前不是最先做

因为你当前很多实验是：

1. 本机回环
2. 单机多进程
3. 很多链路成本还在 Kafka / MySQL / WebSocket / Redis

所以：

1. 真实物理 NIC 路径不是当前吞吐的主瓶颈

### 什么时候应升优先级

如果你后面切到：

1. 多压测机
2. 跨机真实 TCP / WebSocket 流量
3. 机器对外承接大量网络收发

那这块优先级会明显提高。

### 建议动作

届时可以重点看：

1. `ethtool -l <nic>`
2. `ethtool -x <nic>`
3. `/proc/interrupts`
4. `/sys/class/net/<nic>/queues/rx-*/rps_cpus`
5. `/sys/class/net/<nic>/queues/tx-*/xps_cpus`

然后做：

1. 网卡队列数与 CPU 核数匹配
2. IRQ 分散到不同核
3. 把网络软中断核与 `EchoChat` 热核错开

---

## 6. 哪些 CPU 调优收益有限，别先花太多时间

## 6.1 盲目提高单进程 `GOMAXPROCS`

当前不是 `GOMAXPROCS` 太小，而是多实例太大。

所以：

1. 把单进程从 `32` 改到更大没有价值
2. 在多实例场景下继续保持 `32` 往往更容易放大噪声

## 6.2 过早做非常细的 Go 调度器玄学参数优化

例如：

1. 先去猜 GC 一定是主因
2. 先去改一堆非常细的 runtime 参数

当前都不是第一优先级。

现有指标里：

1. RSS 并不高
2. 堆也没有显示出明显失控
3. 当前更大问题还是多进程竞争、链路等待、下游阻塞

## 6.3 在同机回环压测上深挖 NIC 调优

这不是说它没价值，而是：

1. 当前投入产出比不高

同机实验里先把：

1. `GOMAXPROCS`
2. `CPUAffinity`
3. `MySQL/Kafka/EchoChat` 核隔离

做好，往往比先调 NIC 队列更能看到变化。

---

## 7. 我对当前项目的 CPU 优化优先级判断

如果只允许做 CPU 维度的系统调优，我会按下面顺序来。

### P0：立刻做

1. 给 `members10` 这类多实例实验补 `GOMAXPROCS` 扫描
2. 给 `EchoChat / Kafka / MySQL / Redis` 做粗粒度 `CPUAffinity`
3. 记录 `pidstat -u -t -p <pid> 1`、`mpstat -P ALL 1`、`vmstat 1`

### P1：确认后再做

1. 查 NUMA 拓扑
2. 如果是多 NUMA，再做 node 级绑定

### P2：切到跨机流量后做

1. IRQ 亲和性
2. RSS / RPS / XPS
3. NIC queue 调优

---

## 8. 建议你怎么做实验

不要一上来同时改很多项，CPU 维度最怕结论变脏。

建议实验矩阵如下。

### 8.1 第一轮：只扫 `GOMAXPROCS`

固定：

1. `members = 10`
2. `topicPartitions = 240`
3. `mysqlPersist` 相关参数不变
4. `Kafka / MySQL / Redis` 先不绑核

扫描：

1. `GOMAXPROCS = 1`
2. `GOMAXPROCS = 2`
3. `GOMAXPROCS = 3`
4. `GOMAXPROCS = 4`

目标：

1. 找到吞吐和 `p95` 的第一合理停点

### 8.2 第二轮：在最佳 `GOMAXPROCS` 上做 CPU 隔离

固定：

1. 使用第一轮最优 `GOMAXPROCS`

对照：

1. 无绑核
2. 仅 `EchoChat` 绑核
3. `EchoChat + Kafka + MySQL + Redis` 全部粗粒度绑核

目标：

1. 看吞吐是否继续上涨
2. 看 `p95/p99` 是否明显更稳

### 8.3 第三轮：如果机器是多 NUMA，再补 NUMA 实验

对照：

1. 无 NUMA 绑定
2. 只绑 CPU
3. CPU + memory 同 node

目标：

1. 看尾延迟是否继续下降

---

## 9. 推荐观测指标

CPU 调优不能只看吞吐，必须同时看系统采样。

### 必采命令

```bash
pidstat -u -t -p <echochat_pid> 1
pidstat -u -t -p <mysql_pid> 1
pidstat -u -t -p <kafka_pid> 1
mpstat -P ALL 1
vmstat 1
```

如果做磁盘关联判断，再补：

```bash
iostat -x 1
```

### 必看指标

1. 每个 `EchoChat` 进程的瞬时 CPU
2. 每核 CPU 利用率是否极不均匀
3. `run queue` 是否抖动
4. `go_threads`
5. `go_goroutines`
6. 吞吐
7. `p95/p99`
8. Kafka consumer stage，尤其 `mysql_persist` 与 `total`

### 判断标准

好的 CPU 调优结果应该是：

1. 吞吐上升或持平
2. `p95/p99` 下降
3. 每核分布更稳定
4. 没有把 MySQL / Kafka 挤到长期饥饿

而不是：

1. CPU 百分比更高
2. 但吞吐没涨
3. `p95` 更差

---

## 10. 最终建议

对你当前 `EchoChat` 来说，CPU 维度最值得收下来的判断是：

1. **当前不是单进程 CPU 不够，而是单机多进程 CPU 竞争太乱。**
2. **最可能带来真实吞吐收益的第一动作，是在多实例场景下降低每实例 `GOMAXPROCS`。**
3. **第二动作是给 `EchoChat / Kafka / MySQL / Redis` 做粗粒度 CPU 隔离。**
4. **NUMA 要先确认拓扑后再做。**
5. **IRQ / RSS / RPS / XPS 对未来跨机流量重要，但对你当前同机压测不是第一优先级。**

如果只允许我选一条最先做的 CPU 优化实验，我会选：

1. `members10`
2. `topicPartitions=240`
3. 固定其他参数
4. 扫 `GOMAXPROCS=1/2/3/4`
5. 记录吞吐、`p95`、`pidstat`、`mpstat`

因为这一轮最有可能直接回答：

**你现在的吞吐上限，到底是“实例不够”，还是“实例太会抢 CPU”。**

