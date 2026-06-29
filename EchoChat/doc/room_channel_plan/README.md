# Room + Channel 双向链表方案

## 背景

Kafka 模式下群消息处理的当前链路（`handleConsumedGroupMessage`）存在三个可优化的瓶颈：

1. **每次群消息都查数据库** — 从 group_info 表取成员列表并反序列化
2. **全局锁逐个查找在线成员** — `dispatchToKafkaRecipients` 对每个成员调用 `GetClient()`，抢同一把 `KafkaServer.mutex`
3. **跨实例转发 N 次 Redis 操作** — 每个不在本实例的成员独立走一次 Redis GET + Redis PUBLISH

本方案参考 goim（Terry-Mao/goim）中 `Room` + `Channel` 双向链表的设计，将"每次查库"改为"连接生命周期内维护内存结构"，消除群消息链路上的数据库查询和全局锁竞争。

---

## 数据结构

### Channel（连接）

```go
type Channel struct {
    Uuid     string     // 用户 ID
    Conn     *websocket.Conn
    SendBack chan *MessageBack
    // ...

    rooms   map[string]*Room   // 该用户所属的所有 Room
    mu      sync.RWMutex       // 保护 rooms 的并发安全

    // 用于 Room 链表的指针
    roomNext *Channel    // 在 Room 链表中的后继
    roomPrev *Channel    // 在 Room 链表中的前驱
    room     *Room       // 所属 Room（单链表指针，用于 Del 时回查）
}
```

### Room（群聊房间）

```go
type Room struct {
    ID      string     // 群 UUID（G 开头）
    rLock   sync.RWMutex

    // 双向链表头
    head    *Channel

    // 在线统计
    online  int32      // 本实例在本房间的在线人数

    // 生命周期
    drop    bool       // mark 可清理
}
```

---

## 核心操作

### Put —— 用户加入 Room（登录 / 加群时触发）

```
Channel 插入链表头部，O(1)
     head
      │
      ▼
    ┌──────┐    ┌──────┐    ┌──────┐
    │  C3  │◄──►│  C2  │◄──►│  C1  │
    │(新)  │    │      │    │      │
    └──────┘    └──────┘    └──────┘
```

```go
func (r *Room) Put(ch *Channel) {
    r.rLock.Lock()
    if !r.drop {
        if r.head != nil {
            r.head.roomPrev = ch
        }
        ch.roomNext = r.head
        ch.roomPrev = nil
        ch.room = r
        r.head = ch
        r.online++
    }
    r.rLock.Unlock()
}
```

### Del —— 用户离开 Room（登出 / 退群 / 断线时触发）

```
从链表中摘除节点，O(1)
     head
      │
      ▼
    ┌──────┐    ┌──────┐    ┌──────┐
    │  C3  │◄──►│  C2  │◄──►│  C1  │
    └──────┘    └──╂───┘    └──────┘
                    │
             删除 C2，前后直连
                    │
                    ▼
    ┌──────┐    ┌──────┐    ┌──────┐
    │  C3  │◄──►│  C1  │    │  C2  │(孤立)
    └──────┘    └──────┘    └──────┘
```

```go
func (r *Room) Del(ch *Channel) bool {
    r.rLock.Lock()
    if ch.roomNext != nil {
        ch.roomNext.roomPrev = ch.roomPrev
    }
    if ch.roomPrev != nil {
        ch.roomPrev.roomNext = ch.roomNext
    } else {
        r.head = ch.roomNext
    }
    ch.roomNext = nil
    ch.roomPrev = nil
    ch.room = nil
    r.online--
    r.drop = r.online == 0
    r.rLock.Unlock()
    return r.drop
}
```

### Push —— 群发消息

```go
func (r *Room) Push(jsonMessage []byte) {
    r.rLock.RLock()
    for ch := r.head; ch != nil; ch = ch.roomNext {
        ch.enqueueBack(&MessageBack{
            Message: jsonMessage,
        })
    }
    r.rLock.RUnlock()
}
```

**关键设计**:
- 使用 `RLock`（读锁），允许多个 Push 并发执行
- 不同 Room 的 Push 完全独立，互不阻塞
- 遍历链表推送，不需要查库、不需要取成员列表、不需要全局锁

---

## 生命周期管理

### 用户登录时

```
NewClientInit
  → KafkaServer.Login (channel)
  → 注册到 Clients map
  → 注册 Redis 路由
  → joinUserRooms(client)   ← 新增
      └─ 查一次 DB 获取用户所属的所有群
      └─ 逐个 k.getOrCreateRoom(roomID).Put(client.channel)
```

### 用户登出 / 断线时

```
cleanupDisconnected
  → KafkaServer.Logout (channel)
  → 从 Clients map 移除
  → 从 Redis 路由移除
  → leaveAllRooms(client)   ← 新增
      └─ 遍历 client.rooms
      └─ 逐个 room.Del(channel)
      └─ 如果 room.drop == true，从 KafkaServer.Rooms 删除
```

### 用户加群时

```
/addGroup 等接口成功后
  → k.getOrCreateRoom(groupID).Put(client.channel)
```

### 用户退群时

```
/leaveGroup 等接口成功后
  → room.Del(client.channel)
  → 如果 room.drop == true，从 KafkaServer.Rooms 删除
```

---

## 群消息推送完整新流程

```
Kafka consumer 收到群消息 (handleConsumedGroupMessage)
  │
  ├─ [异步池化] saveKafkaMessage(message)  ← 落库（不变）
  │
  ├─ 构造 messageRsp，json.Marshal
  │
  ├─ Room.Push(jsonMessage)                 ← 新增：本实例在线成员直接推送
  │     └─ RLock 遍历链表 → enqueueBack
  │
  └─ 跨实例：Redis PUBLISH room:G1234        ← 新增：其他实例共享的消息
        └─ 其他实例收到后，本地 Room.Push
```

---

## 跨实例转发方案

当前 `dispatchToKafkaRecipients` 对每个跨实例成员做 `remoteRouteForClient + Publish`。

Room 方案下改为**房间级广播**：

```go
const kafkaWSDispatchRoomChannel = "echochat:kafka:room_dispatch"

// 推送方
func (k *KafkaServer) publishToRoom(roomID string, jsonMessage []byte) {
    envelope := kafkaRoomDispatchEnvelope{
        RoomID:  roomID,
        Message: string(jsonMessage),
    }
    payload, _ := json.Marshal(envelope)
    myredis.Publish(kafkaWSDispatchRoomChannel, string(payload))
}

// 消费方（统一订阅器）
func (k *KafkaServer) startRoomDispatchSubscriber() {
    pubsub := myredis.Subscribe(kafkaWSDispatchRoomChannel)
    for msg := range pubsub.Channel() {
        var envelope kafkaRoomDispatchEnvelope
        json.Unmarshal([]byte(msg.Payload), &envelope)
        if room := k.getRoom(envelope.RoomID); room != nil {
            room.Push([]byte(envelope.Message))
        }
    }
}
```

**收益**: 原来 N 个跨实例成员需要 N 次 Redis GET + N 次 PUBLISH，现在统一为 1 次 PUBLISH（发到房间频道），所有实例各自判断本地是否有在线成员，有则 Push。

---

## 收益对比

以 200 人群、3 个服务实例、本实例 60 人在线为例：

| 操作 | 原来（每消息） | Room 方案后（每消息） |
|------|--------------|---------------------|
| DB 查询 | 1× `SELECT group_info` | **0** |
| JSON Unmarshal | 1× `json.Unmarshal(members)` | **0** |
| `KafkaServer.mutex` 抢锁 | 200×（遍历成员逐个 GetClient） | **0**（不再需要全局锁遍历） |
| Room 级 RLock | 0 | **1 次** |
| 跨实例 Redis GET | ~140×（非本实例成员数） | **0** |
| 跨实例 Redis PUBLISH | ~140× | **1 次**（房间频道广播） |
| 本地 enqueueBack | 60× | 60×（不变） |

---

## 增量迁移方案

1. **Phase 1**: 在 KafkaServer 中新增 `Rooms map[string]*Room`，用户登录时 `joinUserRooms`（异步一次拉群列表）
2. **Phase 2**: 群消息处理走 Room 分流 — 先 Room.Push 本实例，兜底走旧逻辑
3. **Phase 3**: 稳定后去掉旧查库逻辑，上线房间级 Redis PUBLISH

### Rollback

Room 方案出问题时，切回旧逻辑只需：`room == nil` 时走原 `handleConsumedGroupMessage` 查库链路，无需停机。

---

## 参考

- goim `internal/comet/room.go` — Room 双向链表实现
- goim `internal/comet/channel.go` — Channel 数据结构
- goim `internal/comet/bucket.go` — Bucket 分片 + Room 管理
