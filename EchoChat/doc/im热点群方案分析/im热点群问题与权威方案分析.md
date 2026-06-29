# IM 热点群问题与权威方案分析

## 1. 这份文档讲什么

这份文档专门讨论：

**IM 项目怎么解决“热点群 / 大群 / 超大社群”问题。**

这里的“热点群”不是单纯指群成员多，而是指：

- 单群在线人数高
- 单群消息频率高
- 单条消息 fanout 接收者多
- 单群相关状态同步、离线推送、未读更新都会放大

这类群一旦爆起来，系统容易被拖死的点不止一个：

1. 上行写入压力
2. 在线广播压力
3. 离线推送压力
4. 顺序与一致性压力
5. 订阅关系 / 在线状态压力
6. 客户端渲染压力
7. 机房出口带宽压力

我这次主要收集了：

- 大厂官方工程博客
- 官方产品/架构文档
- 云厂商和 IM 厂商的架构实践资料

并把它们整理成一份：

1. 资料来源与要点
2. 热点群问题的主流解法
3. 当下更成熟的一组组合方案
4. 对 `echochat` 最值得借鉴的部分

---

## 2. 先说结论

如果只说一句结论：

**当下解决热点群问题，没有单一银弹，最成熟的方案是一组组合拳：**

1. **把“普通群”和“超大社群”拆成不同产品形态**
2. **把 fanout 从“按用户逐个推”升级成“两级 fanout / 按节点聚合推”**
3. **把成员关系和消息收发从“全群镜像”升级成“server/channel/topic + 订阅模型”**
4. **把所有通知都下发，升级成“按活跃视图推消息、非活跃视图只推未读/状态”**
5. **对热点群做隔离、限流、优先级和降级**
6. **客户端也参与降级，不让百万消息直接把 UI 冲垮**

换句话说，真正成熟的大厂方案不是：

- 继续拿“小群聊架构”硬扩到百万群

而是：

- **到了某个规模，就切产品模型、切广播模型、切订阅模型。**

这点非常关键。

---

## 3. 我这次主要参考了哪些资料

### 3.1 Discord 官方工程博客

1. `How Discord Scaled Elixir to 5,000,000 Concurrent Users`  
   https://discord.com/blog/how-discord-scaled-elixir-to-5-000-000-concurrent-users
2. `Maxjourney: Pushing Discord’s Limits with a Million+ Online Users in a Single Server`  
   https://discord.com/blog/maxjourney-pushing-discords-limits-with-a-million-plus-online-users-in-a-single-server

核心价值：

- 这是最典型的“超大社群、热点 fanout、在线用户极高”的官方工程资料。

### 3.2 Slack 官方工程博客

1. `Real-time Messaging`  
   https://slack.engineering/real-time-messaging/
2. `Flannel: An Application-Level Edge Cache to Make Slack Scale`  
   https://slack.engineering/flannel-an-application-level-edge-cache-to-make-slack-scale/

核心价值：

- 这是比较成熟的“channel server + edge fanout + pub/sub + edge cache”路线。

### 3.3 Ably 官方文档

1. `Scalability of the Ably platform`  
   https://ably.com/docs/platform/architecture/platform-scalability

核心价值：

- 讲得很清楚，尤其适合总结“两级 fanout”这种通用方法论。

### 3.4 腾讯云 IM 官方资料

1. 产品页  
   https://cloud.tencent.com/product/im
2. 群组消息文档  
   https://cloud.tencent.com/document/product/269/3663
3. 群组功能配置  
   https://cloud.tencent.com/document/product/269/38656
4. 服务端 API 调频公告  
   https://cloud.tencent.com/document/product/269/93324

核心价值：

- 官方公开提到了：
  - 社群-分组-话题
  - 快慢通道
  - 两级合并推送
  - 群消息频控
  - 超大群能力

### 3.5 阿里云开发者社区中的大群实践讨论

1. `支持百万人超大群聊的Web端IM架构设计与实践`  
   https://developer.aliyun.com/article/1656799
2. `移动端IM中大规模群消息的推送如何保证效率、实时性？`  
   https://developer.aliyun.com/article/633691

核心价值：

- 不是官方产品文档，但对于“热点群为什么炸、怎么做批处理、怎么做消息压缩、怎么做优先级通道”讲得很实在。

### 3.6 网易云信“圈组”相关实践（转载但信息量很高）

1. `实时社群技术专题(二)：百万级成员实时社群技术实现（消息系统篇）`
   https://cloud.tencent.com/developer/article/2302683

核心价值：

- 这份材料虽然发布在腾讯云开发者社区，但内容来源明确标注为网易云信工程师分享。
- 对 `server/channel` 二级结构、订阅模型、未读状态订阅、推送分片、一致性缓存、多级缓存等讲得很完整。

---

## 4. 热点群问题到底难在哪

很多人会把热点群理解成“一个群有很多人”，其实不够。

真正难的是下面这几个放大效应叠加。

### 4.1 fanout 放大

一条群消息，不再是一条消息，而是：

- 对所有在线用户的实时分发
- 对所有离线用户的离线推送判断
- 对所有用户未读状态的影响

如果在线成员 10 万，单条消息就可能瞬间变成 10 万次投递动作。

### 4.2 路由与在线状态放大

不是只知道“这个群有哪些人”，还要知道：

- 这些人当前在哪台 gateway
- 哪些端在线
- 哪些用户只订阅未读，不订阅消息正文

### 4.3 带宽放大

阿里云那篇文章讲得很直接：

- 百万在线时，机房出口带宽本身就是瓶颈

来源：

- https://developer.aliyun.com/article/1656799

### 4.4 客户端渲染放大

这点很容易被忽略。

即使服务端能推出来，客户端也未必能接得住：

- 消息刷屏太快
- CPU 撑不住
- 用户根本看不清

所以热点群问题不是纯服务端问题。

### 4.5 产品语义和小群不一样

Discord / 腾讯社群 / 网易圈组这些方案都在暗示同一件事：

**超大社群已经不是“把普通群人数调大”那么简单。**

需要：

- server / channel / topic 分层
- 订阅模型
- 非活跃频道只推未读

否则整个设计会塌。

---

## 5. 权威方案里反复出现的核心思路

下面是我从这些资料里提炼出的“反复出现”的解法。

## 5.1 两级 fanout：这是最核心的一招

这是几乎最稳定的共识。

### 代表资料

- Slack 官方：channel server 先把消息发到所有订阅该 channel 的 gateway/GS，再由各 GS 发给本机连接  
  https://slack.engineering/real-time-messaging/
- Ably 官方：channel processor -> frontend servers -> connected clients  
  https://ably.com/docs/platform/architecture/platform-scalability

### 核心思想

不是：

- 服务端拿到消息后直接给 10 万个用户逐个推

而是：

1. **先按节点聚合**
2. **每个节点再本地 fanout**

也就是：

```text
消息服务
  -> 找到订阅了该群/频道的 gateway 节点
  -> 每个 gateway 收到一份
  -> gateway 再给本机连接广播
```

### 为什么强

因为它把 fanout 从：

- `消息 -> 用户`

变成：

- `消息 -> 节点 -> 用户`

这样可以大幅减少：

- 跨节点 RPC 次数
- 中央节点 CPU
- 中央节点网络出流

---

## 5.2 热点群不能全压在一个逻辑点上，要做 relay / fanout offload

### 代表资料

- Discord `Maxjourney`
- Discord `How Discord Scaled Elixir to 5,000,000 Concurrent Users`

Discord 非常典型：

- 早期一个 guild process 负责中心路由
- 后来热点大 guild 顶不住
- 引入 `relays`

来源：

- https://discord.com/blog/maxjourney-pushing-discords-limits-with-a-million-plus-online-users-in-a-single-server
- https://discord.com/blog/how-discord-scaled-elixir-to-5-000-000-concurrent-users

### 核心思想

保留一个逻辑中心点做：

- 顺序
- 状态
- 权限

但真正的 fanout 工作拆出去给多个 relay / sender worker。

### 为什么强

因为热点群真正烧 CPU 的往往不是“决定谁该收消息”，而是：

- 实际 fanout
- 网络发送
- 权限过滤
- 连接分发

把这部分 offload 掉，中心协调点就能活更久。

---

## 5.3 从“全成员在线广播”切到“订阅模型”

### 代表资料

- 网易云信“圈组”消息系统
- Slack Flannel / pub-sub 思路

最典型的是云信“圈组”：

- 用户登录后订阅感兴趣的 server/channel
- 长连接服务器上保留详细订阅关系
- 后端只维护“哪些链接节点订阅了该频道”的简化视图

来源：

- https://cloud.tencent.com/developer/article/2302683

### 核心思想

不是每来一条消息就：

- 查群成员
- 查所有成员在线状态

而是：

- 提前建立订阅关系
- 广播时直接按订阅关系走

### 为什么强

因为它把热点路径从：

- “查所有群成员”

变成：

- “查哪些节点订阅了这个频道”

复杂度会小很多。

---

## 5.4 超大社群要拆成 server / channel / topic，不要拿传统群硬扩

### 代表资料

- Discord 官方
- 腾讯云 IM 官方社群 / 话题
- 网易云信圈组

腾讯官方文档很明确：

- 社群支持超大规模成员
- 同一社群下可创建多个话题
- 不同话题独立收发消息、互不干扰

来源：

- https://cloud.tencent.com/document/product/269/38656
- https://cloud.tencent.com/product/im

### 核心思想

不要让所有人都在一个大群消息流里。

要拆成：

- 大社群承载成员关系
- 频道/话题承载具体消息流

### 为什么强

因为它直接降低了单条消息的 fanout 面。

这其实是最有杀伤力的方案之一：

**不是把 fanout 做得更快，而是先让不该收到的人根本不用收。**

---

## 5.5 推送不是一刀切：按活跃视图推正文，其他只推未读/状态

### 代表资料

- 网易云信圈组
- Slack pub/sub 演进

云信这点讲得特别清楚：

- 当前停留页面的频道，可以订阅正文消息
- 其他频道只订阅未读计数或未读状态
- 未读到了 99+ 可以进一步简化通知

来源：

- https://cloud.tencent.com/developer/article/2302683

Slack 也在往：

- 客户端只订阅当前视图相关事件

方向走。

来源：

- https://slack.engineering/flannel-an-application-level-edge-cache-to-make-slack-scale/

### 核心思想

大社群下，不可能要求客户端：

- 所有频道都实时收全量正文

更合理的是：

- 活跃频道：推正文
- 非活跃频道：推未读
- 极冷频道：只在拉列表时补

这对热点群很关键。

---

## 5.6 快慢通道、优先级、消息频控是必须的

### 代表资料

- 腾讯云 IM 官方
- 阿里云大群讨论

腾讯云官方产品页明确提到：

- “快慢通道 + 两级合并推送”

来源：

- https://cloud.tencent.com/product/im

腾讯云群消息文档和 API 调频文档也说明：

- 群消息有频控
- 服务端 API 也有调频机制

来源：

- https://cloud.tencent.com/document/product/269/3663
- https://cloud.tencent.com/document/product/269/93324

### 核心思想

热点群消息不能一律同等待遇。

至少要区分：

1. 高优先级
2. 普通正文
3. 瞬时事件
4. 可丢弃通知

### 为什么强

这样在过载时可以：

- 保关键消息
- 降低普通消息 fanout
- 合并或丢弃低价值事件

---

## 5.7 批量路由查询、批量在线态查询、批量推送

### 代表资料

- 阿里云《移动端IM中大规模群消息的推送如何保证效率、实时性？》

这篇文章讲得很接地气：

- 不要逐条查在线态
- 不要逐条查路由
- 不要逐条做 offline push
- 全都应该批量化

来源：

- https://developer.aliyun.com/article/633691

### 核心思想

热点群下一条消息牵涉太多用户，任何“逐个查、逐个推”的代码路径都会死。

所以要：

- 批量查路由
- 批量查在线态
- 按节点聚合批量推

这点其实是工程上最容易落地，也最值得借鉴的。

---

## 5.8 历史消息一般走读扩散，且要有多级缓存

### 代表资料

- 网易云信圈组消息系统

云信明确提到：

- 超大社群历史消息不适合写扩散
- 更偏读扩散
- 最近消息和未读计数放中心缓存
- 热读继续下沉到计算节点内存

来源：

- https://cloud.tencent.com/developer/article/2302683

### 核心思想

大群下不应该把一条消息存成 N 份。

更合理的是：

- 消息正文存一份或少量副本
- 未读/游标单独建模
- 最近热消息多级缓存

这也是为什么大群方案通常会和 seq / hasReadSeq / summary 一起出现。

---

## 5.9 客户端也必须参与降级

### 代表资料

- 阿里云百万群文章
- Slack Flannel
- 云信圈组

这些资料虽然说法不同，但都在表达同一个意思：

**热点群问题不能只靠服务端。**

客户端必须配合：

1. 当前频道优先
2. 非当前频道只维护未读
3. 大群列表分页 / lazy load
4. 消息压缩与轻量编码
5. 渲染节流

阿里云那篇甚至明确指出：

- 即使服务端扛住，客户端也可能因为刷屏太快体验崩掉

来源：

- https://developer.aliyun.com/article/1656799

---

## 6. 当下更成熟的一组“最佳组合方案”

如果让我综合这些资料，抽出“当前更成熟的一组方案”，我会给下面这套组合。

## 6.1 产品层：拆模型

1. 普通群和超大社群分产品形态
2. 超大社群必须有 `server/community -> channel/topic` 二级结构
3. 不让所有消息都落在一个 fanout 面上

## 6.2 控制层：订阅化

1. 用户按频道/话题订阅
2. 后端维护简化订阅视图
3. 广播时按“订阅节点”而不是按“全体成员”查找

## 6.3 数据层：两级 fanout

1. 中央消息服务只找目标 gateway 节点
2. gateway 再给本机连接 fanout
3. 热点群进一步用 relay/shard 拆开 fanout 工作

## 6.4 存储层：读扩散 + 热缓存

1. 消息正文不做用户级大规模写扩散
2. 最近消息、未读计数、游标进缓存
3. 历史消息按 seq 拉取

## 6.5 过载层：优先级与降级

1. 快慢通道
2. 频控
3. 重要消息优先
4. 非活跃频道只推未读
5. 超载时合并、抽样、丢弃低价值事件

## 6.6 客户端层：只接需要的流

1. 当前频道全量
2. 非当前频道摘要/未读
3. 大群成员 lazy load
4. 大群事件 UI 节流

这组组合拳，比“只在服务端多加几台机器”成熟得多。

---

## 7. 这些方案对 `echochat` 的意义

你现在 `echochat` 已经有一些很好的基础：

1. Kafka 主干
2. seq 体系
3. conversation bucket
4. 批量 persist
5. 实例路由

但如果单独看热点群治理，你离这些大厂方案还差几块比较关键的能力。

## 7.1 你已经有的好基础

### 1. 有主干 MQ

这意味着：

- 群聊热点不会直接打穿入口

### 2. 有顺序语义

你有：

- `conversation_key + session_seq`

这让你更容易做：

- 当前频道拉取
- 未读游标

### 3. 有 conversation bucket / worker pool

这已经比很多简单 IM 强了，说明你已经意识到：

- 会话级调度和热点隔离的重要性

## 7.2 你现在最缺的几块

### 1. 缺“真正的两级 fanout”

你现在更多还是：

- consumer 处理后找用户路由

但还不是那种非常明确的：

- 中央消息服务 -> 节点聚合 -> 节点本地 fanout

### 2. 缺“超大社群的产品拆分”

你当前群聊仍然更像：

- 传统群

而不是：

- `community/server + topic/channel`

### 3. 缺“订阅模型”

你现在更像消息来了再找谁在线，而不是：

- 用户预先按频道订阅
- 后端只按订阅关系 fanout

### 4. 缺“非活跃频道只推未读”的策略

这点在大社群里非常关键。

### 5. 缺“快慢通道 + 群消息频控 + 优先级”

这块你现在可以补，而且应该尽早补。

---

## 8. 我对 `echochat` 的建议

我按收益和改造成本给一版顺序。

## 第一阶段：先补最值的工程手段

这阶段不改产品形态，只先改善热点群技术路径。

### 建议 1：做按节点聚合的群 fanout

目标：

- 同一条群消息先按 `instance_id` 聚合
- 一次 RPC / 一次本地分发打一个节点

而不是在中央路径里对每个在线成员逐个决定投递。

### 建议 2：批量查群成员在线态和路由

不要逐个查 Redis / 路由表。

### 建议 3：给群消息加频控和优先级

至少分：

1. 普通群聊正文
2. @ 消息
3. 系统广播
4. 可合并事件

### 建议 4：群热点时关闭重缓存更新

热点群路径里：

- 不要同步重写完整消息列表缓存
- 只做失效或轻摘要

---

## 第二阶段：补“订阅式视图”

这一步开始往大社群方向演进。

### 建议 1：当前活跃群/频道订阅正文

只有用户当前打开的群/频道，才需要全量实时正文。

### 建议 2：非活跃群只推未读变化

这样能大幅降低：

- 无意义的 fanout
- 客户端消息洪泛

### 建议 3：为群列表和未读做独立模型

这一步和你前面准备补的：

- `last_message`
- `summary`
- `unread`

是契合的。

---

## 第三阶段：如果真要做超大社群，别继续拿“群聊”硬扛

当你目标变成：

- 10w+
- 50w+
- 百万社群

那就该新建：

- `community/server`
- `topic/channel`

而不是继续让一个 `group_id` 承担所有消息流。

这是我从腾讯云社群、Discord、云信圈组三家资料里得到的最一致判断。

---

## 第四阶段：真正的热点群专项治理

如果后面你还要继续打更高规模，才值得再做：

1. relay/fanout shard
2. 热点群独立资源池
3. 快慢通道
4. 大群视图分层
5. 更激进的 pull/push 混合

---

## 9. 一句话总结

当下 IM 项目解决热点群问题的成熟做法，不是继续把普通群聊架构横向放大，而是**用“二级群模型 + 订阅模型 + 两级 fanout + 热点隔离 + 只对活跃视图推正文 + 快慢通道和频控”这组组合拳**；对 `echochat` 来说，最先该借鉴的是按节点聚合 fanout、群消息批量路由/在线态查询、群消息优先级与频控，以及非活跃频道只推未读的思路。

---

## 10. 压缩版关键点

1. 热点群问题本质上是 fanout、在线态、离线推送、带宽和客户端渲染的放大叠加。
2. 大厂主流共识是两级 fanout：中央服务先发节点，节点再本地发连接。
3. Discord 代表的是“热点 guild relay 化、fanout offload、状态瘦身”路线。
4. Slack 代表的是“channel server + consistent hash + edge fanout + pub/sub + edge cache”路线。
5. 腾讯云 IM 代表的是“社群-分组-话题 + 快慢通道 + 两级合并推送 + 频控”路线。
6. 网易云信圈组代表的是“server/channel 二级结构 + 订阅模型 + 非活跃频道只推未读 + 推送任务分片”路线。
7. 超大社群不能继续拿普通群聊硬扩，必须拆成更细的消息流单元。
8. 对 `echochat` 最值的借鉴点是节点聚合 fanout、批量在线态/路由查询、群消息频控优先级和订阅式未读视图。
