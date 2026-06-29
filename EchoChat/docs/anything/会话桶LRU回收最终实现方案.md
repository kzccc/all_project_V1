# 会话桶 LRU 回收最终实现方案

## 1. 目标

本方案用于彻底解决会话桶 `buckets map` 只增不减的问题。  
最终实现目标如下：

1. `buckets map` 不再随着历史总会话数无限增长。
2. 系统优先保留近期活跃会话的 bucket。
3. 长时间不活跃、且当前绝对安全可删除的 bucket，会被自动回收。
4. 回收逻辑不影响当前会话桶的顺序控制和调度语义。
5. bucket 生命周期从“只创建不删除”升级为“创建、复用、淘汰”完整闭环。

---

## 2. 总体方案

最终方案采用：

- `buckets map[string]*conversationBucketState` 作为主索引
- 一条全局双向链表作为 bucket 的 LRU 顺序结构
- 后台周期性 LRU 回收器
- bucket 总量超限时的主动淘汰
- 删除前严格校验 bucket 当前是否绝对空闲

该方案的核心原则是：

**LRU 只负责决定“谁优先被考虑淘汰”，真正是否删除，由 bucket 当前状态决定。**

---

## 3. 数据结构

### 3.1 bucket 结构扩展

在现有 `conversationBucketState` 上增加以下字段：

```go
type conversationBucketState struct {
    key     string
    mu      sync.Mutex
    queue   []*conversationBucketJob
    running bool
    queued  bool

    lastActiveAt int64
    lruElem      *list.Element
}
```

字段含义：

- `lastActiveAt`
  - bucket 最近一次活跃时间，使用 `UnixNano`
- `lruElem`
  - 当前 bucket 在全局 LRU 双向链表中的节点引用

### 3.2 claimRunner 扩展

在 `conversationBucketClaimRunner` 上增加以下字段：

```go
type conversationBucketClaimRunner struct {
    ...
    buckets   map[string]*conversationBucketState
    bucketsMu sync.Mutex

    lruMu      sync.Mutex
    lruList    *list.List
    maxBuckets int
    bucketIdleTTL time.Duration
    gcInterval time.Duration
}
```

字段含义：

- `lruList`
  - bucket LRU 双向链表，链表头表示最近活跃，链表尾表示最久未使用
- `maxBuckets`
  - bucket 最大保留数量
- `bucketIdleTTL`
  - bucket 最长空闲时间
- `gcInterval`
  - 后台回收器扫描周期

---

## 4. 活跃性定义

bucket 的“活跃”定义为：

1. 新消息进入 bucket
2. 已存在 bucket 被命中并复用
3. bucket 被重新放入 ready queue
4. worker 开始处理该 bucket

最终实现中，以 **新消息进入 bucket** 作为强制更新时间点，  
以 **bucket 被命中复用** 作为补充更新时间点。

实现规则：

- 每次 `getOrCreateBucket` 命中已有 bucket 时，刷新其 LRU 位置
- 每次 enqueue 新消息时，刷新 `lastActiveAt`

---

## 5. LRU 维护规则

### 5.1 bucket 创建时

新建 bucket 后：

1. 初始化 `lastActiveAt = now`
2. 插入 `lruList` 头部
3. 把 `lruElem` 指向该链表节点

### 5.2 bucket 被访问时

bucket 被命中或收到新消息时：

1. 更新 `lastActiveAt = now`
2. 将其 `lruElem` 移动到 `lruList` 头部

### 5.3 bucket 被删除时

bucket 被确认可删除后：

1. 从 `buckets map` 删除
2. 从 `lruList` 删除对应节点
3. 将 `bucket.lruElem = nil`

---

## 6. bucket 可删除条件

一个 bucket 可以被删除，当且仅当同时满足以下条件：

1. `len(bucket.queue) == 0`
2. `bucket.running == false`
3. `bucket.queued == false`
4. `now - bucket.lastActiveAt > bucketIdleTTL`

这四个条件全部满足时，bucket 才允许被淘汰。

该规则的含义是：

- 当前没有待处理消息
- 当前不被 worker 处理
- 当前不在 ready queue 中
- 当前已经空闲足够久

也就是说：

**只有“既空闲、又不参与任何调度状态”的 bucket，才允许被回收。**

---

## 7. 锁顺序

为了避免死锁，整个 LRU 方案统一使用固定锁顺序：

1. `bucketsMu`
2. `lruMu`
3. `bucket.mu`

任何涉及：

- map 查询 / 删除
- LRU 链表移动 / 删除
- bucket 状态检查

的逻辑，都必须遵守这个顺序。

禁止出现：

- 先拿 `bucket.mu` 再拿 `bucketsMu`
- 先拿 `bucket.mu` 再拿 `lruMu`
- 先拿 `lruMu` 再拿 `bucketsMu`

这一点作为实现约束强制执行。

---

## 8. getOrCreateBucket 改造

`getOrCreateBucket` 最终逻辑如下：

1. 先锁 `bucketsMu`
2. 查询 map 中是否已有 bucket
3. 若已有：
   - 更新其活跃时间
   - 进入 LRU 头部
   - 返回
4. 若没有：
   - 创建新 bucket
   - 设置 `lastActiveAt`
   - 插入 LRU 头部
   - 放入 map
5. 解锁
6. 如果 bucket 总量超过 `maxBuckets`，触发一次主动淘汰

---

## 9. enqueue 路径改造

消息 enqueue 到 bucket 时：

1. 更新 `bucket.lastActiveAt`
2. 将 bucket 移动到 LRU 头部
3. 将 job 放入 `bucket.queue`
4. 后续继续走原有 `enqueueBucketReady`

这样可以保证：

- 正在持续接收消息的 bucket 始终位于 LRU 前端
- 最近活跃会话不会被误删

---

## 10. 后台回收器

新增一个后台 goroutine：

```go
func (r *conversationBucketClaimRunner) runBucketGC()
```

启动时机：

- 在 `conversationBucketClaimRunner.run()` 启动 worker 后立即启动

运行规则：

1. 每隔 `gcInterval` 触发一次
2. 从 `lruList` 尾部开始扫描
3. 每轮最多回收固定数量 bucket，例如 `maxGCPerRun`
4. 对每个 bucket 按可删除条件检查
5. 满足条件则删除
6. 不满足则跳过，继续往前找

后台回收器不会全量扫描 map，而是：

**始终从“最久未使用 bucket”开始尝试回收。**

---

## 11. 超量主动淘汰

除了后台回收器，还必须有总量超限保护。

规则：

- 每次创建新 bucket 后，如果 `len(buckets) > maxBuckets`
- 立即触发 `evictBucketsUntilWithinLimit()`

该逻辑会：

1. 从 LRU 链表尾部开始
2. 优先淘汰最久未使用 bucket
3. 直到 `len(buckets) <= maxBuckets`
4. 若尾部 bucket 因为状态不安全不可删，则继续尝试前一个

如果本轮可删 bucket 不足，则允许短暂超限，但保留后台 GC 持续回收。

---

## 12. LRU 回收器实现逻辑

最终实现引入两个核心函数：

### 12.1 `tryEvictOneFromTail`

逻辑：

1. 从 `lruList.Back()` 取尾部 bucket
2. 拿 `bucket.mu` 检查：
   - queue 是否为空
   - running 是否为 false
   - queued 是否为 false
   - idleTTL 是否满足
3. 若满足：
   - 从 map 删除
   - 从链表删除
   - 返回 true
4. 若不满足：
   - 返回 false

### 12.2 `evictBucketsUntilWithinLimit`

逻辑：

1. 检查当前 bucket 数量
2. 若超限，从尾部开始循环调用 `tryEvictOneFromTail`
3. 直到恢复到阈值内或本轮找不到可删 bucket

---

## 13. 参数固定方案

本方案使用以下固定参数：

### 13.1 `maxBuckets`

bucket 最大数量由配置给出，例如：

- `maxBuckets = 100000`

### 13.2 `bucketIdleTTL`

bucket 空闲 TTL 固定为：

- `bucketIdleTTL = 5 * time.Minute`

### 13.3 `gcInterval`

后台回收器扫描周期固定为：

- `gcInterval = 1 * time.Minute`

### 13.4 `maxGCPerRun`

每轮后台回收最多回收：

- `maxGCPerRun = 1024`

这样可以避免一次 GC 清理过猛，影响正常调度。

---

## 14. 指标

新增以下指标：

1. `conversation_bucket_total`
   - 当前 bucket 总数
2. `conversation_bucket_lru_evict_total`
   - LRU 淘汰总数
3. `conversation_bucket_idle_evict_total`
   - 因空闲 TTL 回收的 bucket 数
4. `conversation_bucket_gc_scan_total`
   - GC 扫描轮次
5. `conversation_bucket_gc_skipped_total`
   - 因状态不安全而跳过删除的 bucket 数

这些指标用于验证：

- bucket 是否真的在回收
- 回收速度是否足够
- 是否经常因为 `running/queued` 跳过删除

---

## 15. 正常状态下的行为

在此方案下，系统行为为：

- 活跃会话 bucket 始终留在 LRU 前端
- 不活跃会话 bucket 逐步退到尾部
- 长时间无消息的 bucket 被后台自动删除
- bucket 数过多时，从最久未使用会话开始回收

最终表现为：

**bucket 总量主要与近期活跃会话数相关，而不再与历史总会话数线性增长。**

---

## 16. 不允许的行为

本方案明确禁止以下做法：

1. 仅因 bucket 在 LRU 尾部就直接删除
2. 不检查 `running/queued/queue` 状态就删除 bucket
3. 用全量 map 扫描替代 LRU 作为主回收路径
4. 在没有统一锁顺序的情况下同时改 map 和 LRU 链表
5. 创建 bucket 后不纳入 LRU 管理

---

## 17. 最终实现结果

实现完成后，会话桶生命周期将从：

- 创建
- 复用

升级为：

- 创建
- 复用
- LRU 更新
- 空闲回收
- 超量淘汰

这是本方案的最终目标。

一句话总结：

**最终方案使用“map + LRU 双向链表 + 安全删除条件 + 后台回收器 + 超量主动淘汰”的方式，把 bucket 管理从无上限驻留改造成按最近活跃程度自动回收。**
