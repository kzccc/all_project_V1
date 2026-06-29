# EchoChat MySQL 事务补齐计划

## 1. 目标

本计划只讨论一个问题：

**当前 EchoChat 项目里，哪些业务函数在一次业务动作中会连续修改多张表或多条记录，但尚未放入显式 MySQL 事务；这些函数应该补齐事务。**

这里不做优先级拆分，也不区分“立刻补”还是“以后补”，只整理**应该补事务**的代码点，作为后续改造清单。

---

## 2. 判断标准

以下场景纳入“应该补事务”的范围：

1. 一个函数里存在多条数据库写操作，这些写操作共同表达一个业务动作。
2. 这些写操作如果只成功一部分，会留下明显的脏数据或中间态。
3. 这些写操作之间存在强业务一致性要求，即：
   - 要么全部成功；
   - 要么全部失败并回滚。

以下情况不纳入本计划：

1. 只有单条 SQL 写入。
2. 只有查询，没有写入。
3. Redis 删除缓存等旁路动作。

---

## 3. 当前全局背景

当前数据库初始化配置里开启了：

```go
SkipDefaultTransaction: true
```

位置：

- `internal/dao/gorm.go`

这意味着：

1. 普通 `Create/Save/Update/Updates/Delete` 不会自动包事务。
2. 只有显式调用 `dao.GormDB.Transaction(...)` 的代码，才真正具备事务语义。

因此，下面列出的函数如果没有显式事务，实际都是“多条 SQL 分别提交”的状态。

---

## 4. 应该补事务的函数清单

### 4.1 联系人与申请相关

#### 4.1.1 `PassContactApply`

位置：

- `internal/service/gorm/user_contact_service.go:432`

涉及写操作：

1. 更新 `contact_apply.status`
2. 创建 `user_contact` 记录
3. 在用户加好友分支中，再创建反向 `user_contact`
4. 在加群分支中，修改 `group_info.members`
5. 在加群分支中，修改 `group_info.member_cnt`

涉及表：

1. `contact_apply`
2. `user_contact`
3. `group_info`

为什么应该补事务：

1. “通过申请”是一个完整业务动作。
2. 如果申请状态已经改成同意，但联系人关系没建成功，会产生脏状态。
3. 如果加群联系人关系建好了，但群成员列表没更新成功，会出现“关系表已入群、群成员快照未入群”的不一致状态。

事务要保证的原子性：

1. 申请状态变更
2. 联系人关系创建
3. 群成员信息更新

必须整体成功或整体回滚。

---

#### 4.1.2 `DeleteContact`

位置：

- `internal/service/gorm/user_contact_service.go:212`

涉及写操作：

1. 更新 owner -> contact 的 `user_contact`
2. 更新 contact -> owner 的 `user_contact`
3. 软删 owner -> contact 的 `session`
4. 软删 contact -> owner 的 `session`
5. 软删双方相关 `contact_apply`

涉及表：

1. `user_contact`
2. `session`
3. `contact_apply`

为什么应该补事务：

1. “删除联系人”不是单表动作，而是联系人关系、会话、申请记录的联合变更。
2. 如果只删了一边联系人关系，会出现 A 视角和 B 视角状态不一致。
3. 如果联系人关系删了，但会话没删掉，会出现前端行为与真实关系冲突。

事务要保证的原子性：

1. 双向联系人状态一致更新
2. 双向会话一致删除
3. 关联申请记录一致失效

---

#### 4.1.3 `BlackContact`

位置：

- `internal/service/gorm/user_contact_service.go:549`

涉及写操作：

1. 更新 owner -> contact 为 `BLACK`
2. 更新 contact -> owner 为 `BE_BLACK`
3. 软删相关 `session`

涉及表：

1. `user_contact`
2. `session`

为什么应该补事务：

1. “拉黑联系人”是双边关系状态切换。
2. 如果只改了一侧状态，会出现一边已拉黑、一边未被拉黑的脏状态。
3. 如果状态改了但会话没删，会出现行为和关系不一致。

---

#### 4.1.4 `CancelBlackContact`

位置：

- `internal/service/gorm/user_contact_service.go:578`

涉及写操作：

1. 更新 owner -> contact 为 `NORMAL`
2. 更新 contact -> owner 为 `NORMAL`

涉及表：

1. `user_contact`

为什么应该补事务：

1. 解除拉黑本质上是双向关系状态恢复。
2. 如果只恢复一边，关系语义会被破坏。

---

### 4.2 群组相关

#### 4.2.1 `CreateGroup`

位置：

- `internal/service/gorm/group_info_service.go:73`

涉及写操作：

1. 创建 `group_info`
2. 创建群主对应的 `user_contact`

涉及表：

1. `group_info`
2. `user_contact`

为什么应该补事务：

1. 建群成功后，群主必须天然是该群联系人。
2. 如果群创建成功，但群主联系人关系没建成功，会留下“群存在但群主关系不完整”的中间态。

---

#### 4.2.2 `LeaveGroup`

位置：

- `internal/service/gorm/group_info_service.go:258`

涉及写操作：

1. 更新 `group_info.members`
2. 更新 `group_info.member_cnt`
3. 软删该用户与群的 `session`
4. 更新该用户与群的 `user_contact`
5. 软删该用户与群的 `contact_apply`

涉及表：

1. `group_info`
2. `session`
3. `user_contact`
4. `contact_apply`

为什么应该补事务：

1. 退群是一个强一致性动作。
2. 如果成员列表已删除该用户，但联系人关系没删，会出现逻辑冲突。
3. 如果计数和成员 JSON 更新不同步，也会产生群状态异常。

---

#### 4.2.3 `DismissGroup`

位置：

- `internal/service/gorm/group_info_service.go:327`

涉及写操作：

1. 软删 `group_info`
2. 批量软删相关 `session`
3. 批量软删相关 `user_contact`
4. 批量软删相关 `contact_apply`

涉及表：

1. `group_info`
2. `session`
3. `user_contact`
4. `contact_apply`

为什么应该补事务：

1. 解散群聊是典型的级联失效动作。
2. 如果群已删，但相关联系人/会话残留，会导致大量孤儿数据。
3. 这是典型的“主对象删除 + 依赖对象清理”场景。

---

#### 4.2.4 `DeleteGroups`

位置：

- `internal/service/gorm/group_info_service.go:402`

涉及写操作：

1. 软删 `group_info`
2. 批量软删相关 `session`
3. 批量软删相关 `user_contact`
4. 批量软删相关 `contact_apply`

涉及表：

1. `group_info`
2. `session`
3. `user_contact`
4. `contact_apply`

为什么应该补事务：

1. 业务语义与 `DismissGroup` 本质一致，只是入口是管理员批量操作。
2. 每个群的删除动作都应该在单群事务内原子完成。

---

#### 4.2.5 `RemoveGroupMembers`

位置：

- `internal/service/gorm/group_info_service.go:686`

涉及写操作：

1. 修改内存中的 `members`
2. 更新 `group_info.member_cnt`
3. 软删被移除成员与群的 `session`
4. 软删被移除成员与群的 `user_contact`
5. 软删被移除成员与群的 `contact_apply`
6. 最终回写 `group_info.members`

涉及表：

1. `group_info`
2. `session`
3. `user_contact`
4. `contact_apply`

为什么应该补事务：

1. 移除成员是一个多表同步动作。
2. 如果成员列表已经删除，但会话和联系人关系没清掉，会产生不一致。
3. 当前函数还会循环处理多人，至少应该保证“单次调用整体原子”或“单成员原子”。

---

#### 4.2.6 `EnterGroupDirectly`

位置：

- `internal/service/gorm/group_info_service.go:498`

涉及写操作：

1. 更新 `group_info.members`
2. 更新 `group_info.member_cnt`
3. 创建加入群的 `user_contact`

涉及表：

1. `group_info`
2. `user_contact`

为什么应该补事务：

1. 直接入群与“通过加群申请”本质一样，都是建立成员关系。
2. 如果群成员快照和联系人关系不同步，会出现入群中间态。

---

### 4.3 用户状态与删除相关

#### 4.3.1 `DisableUsers`

位置：

- `internal/service/gorm/user_info_service.go:399`

涉及写操作：

1. 更新 `user_info.status`
2. 查询用户相关 `session`
3. 批量软删相关 `session`

涉及表：

1. `user_info`
2. `session`

为什么应该补事务：

1. 用户禁用后，相关会话要同步失效，属于同一个业务动作。
2. 如果用户已禁用，但会话没删掉，会出现禁用态下仍残留历史行为入口的问题。

---

#### 4.3.2 `DeleteUsers`

位置：

- `internal/service/gorm/user_info_service.go:436`

涉及写操作：

1. 软删 `user_info`
2. 批量软删相关 `session`
3. 批量软删相关 `user_contact`
4. 批量软删相关 `contact_apply`

涉及表：

1. `user_info`
2. `session`
3. `user_contact`
4. `contact_apply`

为什么应该补事务：

1. 删除用户是典型级联软删动作。
2. 如果用户删掉了，但联系人或申请记录残留，会产生大量孤儿数据。
3. 这是需要事务保护的多表一致性场景。

---

## 5. 推荐的落地方式

对于上述函数，统一建议：

1. 使用 `dao.GormDB.Transaction(func(tx *gorm.DB) error { ... })`
2. 把所有必须一起成功的 MySQL 写操作放进同一个事务
3. Redis 缓存删除放在事务成功提交之后执行，不放进 MySQL 事务
4. 对批量操作函数，明确事务边界：
   - 是整次调用一个事务
   - 还是每个对象一个事务

需要在具体改造时按函数体量和锁范围单独判断，但至少要先建立显式事务边界。

---

## 6. 风险提醒

补事务时需要额外注意：

1. 事务里不要放 Redis 删除、日志打印、网络调用这类外部慢操作。
2. 涉及循环批量更新的函数，要控制事务范围，避免一个事务过长。
3. 涉及群成员 JSON 更新的函数，未来如果并发更高，可能还需要进一步引入行锁或版本控制。

---

## 7. 本计划覆盖的函数汇总

1. `userContactService.DeleteContact`
2. `userContactService.PassContactApply`
3. `userContactService.BlackContact`
4. `userContactService.CancelBlackContact`
5. `groupInfoService.CreateGroup`
6. `groupInfoService.LeaveGroup`
7. `groupInfoService.DismissGroup`
8. `groupInfoService.DeleteGroups`
9. `groupInfoService.EnterGroupDirectly`
10. `groupInfoService.RemoveGroupMembers`
11. `userInfoService.DisableUsers`
12. `userInfoService.DeleteUsers`

