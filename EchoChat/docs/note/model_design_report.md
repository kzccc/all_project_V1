# EchoChat 业务模型字段设计报告

<div style="margin: 12px 0 20px; padding: 16px 18px; border-radius: 14px; background: linear-gradient(135deg, #fff7ed 0%, #ffedd5 100%); border: 1px solid #fdba74; color: #7c2d12;">
  <div style="font-size: 22px; font-weight: 800; margin-bottom: 6px;">internal/model 业务模型设计说明</div>
  <div style="font-size: 14px; line-height: 1.8;">
    本文档基于 <code>/workspace/czk/Personal/EchoChat/internal/model</code> 目录下的模型代码生成，聚焦于业务字段、字段取值语义以及模型设计背后的业务意图。
  </div>
</div>

<div style="display:grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin: 20px 0;">
  <div style="padding: 14px; border-radius: 12px; background:#f0fdf4; border:1px solid #86efac;">
    <div style="font-size:13px; color:#166534; font-weight:700;">模型数量</div>
    <div style="font-size:26px; color:#14532d; font-weight:800;">6</div>
  </div>
  <div style="padding: 14px; border-radius: 12px; background:#eff6ff; border:1px solid #93c5fd;">
    <div style="font-size:13px; color:#1d4ed8; font-weight:700;">设计重点</div>
    <div style="font-size:14px; color:#1e3a8a; line-height:1.7;">聊天、联系人、群聊、申请、会话、消息</div>
  </div>
  <div style="padding: 14px; border-radius: 12px; background:#faf5ff; border:1px solid #d8b4fe;">
    <div style="font-size:13px; color:#7e22ce; font-weight:700;">设计风格</div>
    <div style="font-size:14px; color:#581c87; line-height:1.7;">软删除 + 状态字段 + 前缀型业务 ID</div>
  </div>
</div>

## 总体设计特征

<div style="padding: 16px 18px; border-left: 5px solid #f97316; background:#fff7ed; border-radius: 10px; line-height: 1.9; color:#7c2d12;">
  <b>1.</b> 各核心表都同时保留数据库自增主键 <code>id</code> 和业务唯一 ID（如 <code>U...</code>、<code>G...</code>、<code>S...</code>、<code>A...</code>）。<br/>
  <b>2.</b> 多个模型使用 <code>DeletedAt</code> 实现软删除，这说明项目更偏向“业务隐藏”而非物理删除，便于恢复、审计和后台管理。<br/>
  <b>3.</b> 模型中大量使用 <code>status</code> 字段承载业务状态机，例如用户禁用、联系人拉黑、申请拒绝、消息已发送等。<br/>
  <b>4.</b> 群成员列表采用 JSON 存储，而不是拆成独立成员明细表，体现出当前项目优先追求开发成本和实现速度，而不是最强范式化设计。<br/>
  <b>5.</b> 会话表、消息表、联系人表之间形成典型 IM 系统链路：<code>UserInfo -> UserContact -> Session -> Message</code>。
</div>

---

## 1. UserInfo 用户模型

<div style="padding: 14px 16px; border-radius: 12px; background: linear-gradient(135deg, #eff6ff 0%, #dbeafe 100%); border:1px solid #93c5fd; color:#1e3a8a; margin: 14px 0;">
  <div style="font-size:18px; font-weight:800;">表名：<code>user_info</code></div>
  <div style="margin-top:8px; line-height:1.85;">
    该模型承载用户账号体系的核心资料，是所有联系人关系、会话记录、消息发送者信息的上游主表。
  </div>
</div>

### 字段说明

| 字段 | 类型 | 取值/约束说明 | 字段含义 | 设计意图 |
| --- | --- | --- | --- | --- |
| `Id` | `int64` | 主键、自增 | 数据库内部主键 | 用于数据库层面的高效索引和关联，不直接暴露给前端 |
| `Uuid` | `string` | 唯一索引，`char(20)`，业务上通常形如 `U...` | 用户业务 ID | 用业务 ID 替代数据库主键暴露给前端，避免泄露真实表增长信息，也方便跨系统引用 |
| `Nickname` | `string` | `varchar(20)`，非空 | 用户昵称 | 直接用于前端展示、会话显示名、联系人信息卡 |
| `Telephone` | `string` | `char(11)`，索引，非空 | 手机号 | 兼作登录账号和唯一识别入口，符合国内 IM 产品常见做法 |
| `Email` | `string` | `char(30)` | 邮箱 | 作为补充资料字段，不参与主认证链路 |
| `Avatar` | `string` | 非空，默认 URL | 用户头像地址 | 直接存可访问路径，方便前端无需二次拼接复杂资源规则 |
| `Gender` | `int8` | `0=男`，`1=女` | 性别 | 用最轻量的枚举字段承载基础资料，利于前端展示和筛选 |
| `Signature` | `string` | `varchar(100)` | 个性签名 | 满足社交资料展示需求 |
| `Password` | `string` | `char(18)`，非空 | 登录密码 | 当前项目直接存明文/弱处理密码字段，说明更偏教学或演示项目，而非生产级安全实现 |
| `Birthday` | `string` | `char(8)` | 生日 | 用字符串而非日期类型，说明业务更关注展示而不是日期计算 |
| `CreatedAt` | `time.Time` | 非空 | 创建时间 | 用于展示注册时间与后台排序 |
| `DeletedAt` | `gorm.DeletedAt` | 软删除时间 | 用户删除标记 | 支持后台查看已删除用户、避免物理删除造成链路断裂 |
| `LastOnlineAt` | `sql.NullTime` | 可空 | 上次在线时间 | 用于后续扩展在线状态、活跃度分析、最近在线展示 |
| `LastOfflineAt` | `sql.NullTime` | 可空 | 最近离线时间 | 补足用户在线生命周期信息 |
| `IsAdmin` | `int8` | `0=普通用户`，`1=管理员` | 管理员标记 | 允许同一套用户表同时承载普通用户和后台管理用户 |
| `Status` | `int8` | `0=正常`，`1=禁用` | 用户状态 | 用于后台封禁、禁止登录、禁止发起新会话 |

### 字段值重点说明

<div style="padding: 14px 16px; background:#f8fafc; border:1px solid #cbd5e1; border-radius:10px; line-height:1.9;">
  <b><code>Status</code></b><br/>
  <span style="color:#166534; font-weight:700;">0 = NORMAL</span>：用户可正常登录、可被检索、可参与聊天。<br/>
  <span style="color:#b91c1c; font-weight:700;">1 = DISABLE</span>：用户被禁用，其他业务流程会拒绝与其建立新会话。
  <br/><br/>
  <b><code>IsAdmin</code></b><br/>
  <span style="color:#334155; font-weight:700;">0</span>：普通用户。<br/>
  <span style="color:#7c3aed; font-weight:700;">1</span>：管理员，可访问后台用户/群聊管理类接口。
</div>

### 模型设计意图

UserInfo 的设计明显是“单表承载完整账号信息”的方案。它既负责登录认证，又负责社交资料展示，同时承担后台权限控制。这样的设计能显著降低小中型 IM 项目的实现复杂度，因为联系人、会话、消息都只需要引用 `Uuid` 即可。代价是表职责略多，但对当前项目体量来说是合理取舍。

---

## 2. UserContact 联系人关系模型

<div style="padding: 14px 16px; border-radius: 12px; background: linear-gradient(135deg, #f0fdf4 0%, #dcfce7 100%); border:1px solid #86efac; color:#166534; margin: 14px 0;">
  <div style="font-size:18px; font-weight:800;">表名：<code>user_contact</code></div>
  <div style="margin-top:8px; line-height:1.85;">
    该模型不是“用户资料”，而是“用户与联系对象之间的一条关系边”。联系对象既可以是另一个用户，也可以是一个群聊。
  </div>
</div>

### 字段说明

| 字段 | 类型 | 取值/约束说明 | 字段含义 | 设计意图 |
| --- | --- | --- | --- | --- |
| `Id` | `int64` | 主键、自增 | 数据库内部主键 | 用于关系表自身管理 |
| `UserId` | `string` | 索引，非空 | 关系拥有者 | 表示“谁在看这条联系人关系” |
| `ContactId` | `string` | 索引，非空 | 对应联系人或群聊 ID | 统一指向用户或群，降低表数量 |
| `ContactType` | `int8` | `0=用户`，`1=群聊` | 联系对象类型 | 允许一张表同时存储好友关系和入群关系 |
| `Status` | `int8` | 多状态枚举 | 当前关系状态 | 支撑拉黑、删除、退群、踢出群等业务分支 |
| `CreatedAt` | `time.Time` | 非空 | 关系创建时间 | 用于联系人排序、群加入时间等扩展能力 |
| `UpdateAt` | `time.Time` | 非空 | 关系更新时间 | 用于记录关系状态最近变化 |
| `DeletedAt` | `gorm.DeletedAt` | 软删除时间 | 关系逻辑删除标记 | 避免硬删除丢失关系历史 |

### 字段值重点说明

<div style="padding: 14px 16px; background:#f8fafc; border:1px solid #cbd5e1; border-radius:10px; line-height:1.9;">
  <b><code>ContactType</code></b><br/>
  <span style="color:#0f766e; font-weight:700;">0</span>：联系对象是用户。<br/>
  <span style="color:#0369a1; font-weight:700;">1</span>：联系对象是群聊。<br/><br/>

  <b><code>Status</code></b><br/>
  <span style="color:#166534; font-weight:700;">0 = NORMAL</span>：正常联系人或正常在群中。<br/>
  <span style="color:#b91c1c; font-weight:700;">1 = BE_BLACK</span>：被对方拉黑。<br/>
  <span style="color:#7f1d1d; font-weight:700;">2 = BLACK</span>：主动拉黑对方。<br/>
  <span style="color:#a16207; font-weight:700;">3 = BE_DELETE</span>：被对方删除。<br/>
  <span style="color:#92400e; font-weight:700;">4 = DELETE</span>：主动删除对方。<br/>
  <span style="color:#475569; font-weight:700;">5 = SILENCE</span>：被禁言，当前项目中预留状态。<br/>
  <span style="color:#1d4ed8; font-weight:700;">6 = QUIT_GROUP</span>：主动退群。<br/>
  <span style="color:#6d28d9; font-weight:700;">7 = KICK_OUT_GROUP</span>：被踢出群聊。
</div>

### 模型设计意图

UserContact 是整个项目里很关键的一张“关系表”。它把好友关系和群成员关系统一抽象成同一种结构，从而让“拉黑”“删除”“退群”“是否可发起会话”等规则都能围绕一套字段实现。这样设计比拆成 `friend_relation` 和 `group_member_relation` 两张表更省开发成本，也更适合教学项目快速迭代。

---

## 3. ContactApply 申请模型

<div style="padding: 14px 16px; border-radius: 12px; background: linear-gradient(135deg, #fefce8 0%, #fef3c7 100%); border:1px solid #fcd34d; color:#854d0e; margin: 14px 0;">
  <div style="font-size:18px; font-weight:800;">表名：<code>contact_apply</code></div>
  <div style="margin-top:8px; line-height:1.85;">
    该模型负责承载好友申请和加群申请的审批流，是联系人关系正式建立前的“待处理状态容器”。
  </div>
</div>

### 字段说明

| 字段 | 类型 | 取值/约束说明 | 字段含义 | 设计意图 |
| --- | --- | --- | --- | --- |
| `Id` | `int64` | 主键、自增 | 数据库内部主键 | 便于管理和排序 |
| `Uuid` | `string` | 唯一索引，业务上通常形如 `A...` | 申请业务 ID | 申请本身作为独立业务对象存在 |
| `UserId` | `string` | 索引，非空 | 申请人 ID | 谁发起申请 |
| `ContactId` | `string` | 索引，非空 | 被申请对象 ID | 可以是用户，也可以是群 |
| `ContactType` | `int8` | `0=用户`，`1=群聊` | 被申请对象类型 | 用一张表覆盖加好友和加群两类申请 |
| `Status` | `int8` | `0=申请中`，`1=通过`，`2=拒绝`，`3=拉黑` | 审批状态 | 承载审批流状态机 |
| `Message` | `string` | `varchar(100)` | 申请附言 | 让申请行为具备社交说明性 |
| `LastApplyAt` | `time.Time` | 非空 | 最近一次申请时间 | 支持重复申请时刷新时间，而不是一定新建记录 |
| `DeletedAt` | `gorm.DeletedAt` | 软删除时间 | 申请删除标记 | 便于后续重新申请、后台审计 |

### 字段值重点说明

<div style="padding: 14px 16px; background:#f8fafc; border:1px solid #cbd5e1; border-radius:10px; line-height:1.9;">
  <b><code>ContactType</code></b><br/>
  <span style="color:#0f766e; font-weight:700;">0</span>：好友申请。<br/>
  <span style="color:#0369a1; font-weight:700;">1</span>：加群申请。<br/><br/>

  <b><code>Status</code></b><br/>
  <span style="color:#d97706; font-weight:700;">0 = PENDING</span>：待处理。<br/>
  <span style="color:#15803d; font-weight:700;">1 = AGREE</span>：申请已通过。<br/>
  <span style="color:#b45309; font-weight:700;">2 = REFUSE</span>：申请已拒绝。<br/>
  <span style="color:#b91c1c; font-weight:700;">3 = BLACK</span>：申请人被拉黑，后续申请会受限。
</div>

### 模型设计意图

ContactApply 的核心设计思想是把“申请”作为独立业务实体，而不是临时行为。这样一来，系统就可以记录申请历史、允许重复申请、支持拒绝与拉黑，并且在真正建立 `UserContact` 关系之前，先通过审批流做业务判断。这对于 IM 场景中的好友验证、群审核是非常自然的设计。

---

## 4. GroupInfo 群聊模型

<div style="padding: 14px 16px; border-radius: 12px; background: linear-gradient(135deg, #ecfeff 0%, #cffafe 100%); border:1px solid #67e8f9; color:#155e75; margin: 14px 0;">
  <div style="font-size:18px; font-weight:800;">表名：<code>group_info</code></div>
  <div style="margin-top:8px; line-height:1.85;">
    群聊模型保存群本身的基本资料、成员快照、群主和状态，是群会话、加群申请、群资料页的中心数据源。
  </div>
</div>

### 字段说明

| 字段 | 类型 | 取值/约束说明 | 字段含义 | 设计意图 |
| --- | --- | --- | --- | --- |
| `Id` | `int64` | 主键、自增 | 数据库内部主键 | 数据库层面索引 |
| `Uuid` | `string` | 唯一索引，业务上通常形如 `G...` | 群业务 ID | 对外暴露安全、统一的群标识 |
| `Name` | `string` | `varchar(20)`，非空 | 群名称 | 前端展示和会话展示名来源 |
| `Notice` | `string` | `varchar(500)` | 群公告 | 用于群规则、通知展示 |
| `Members` | `json.RawMessage` | JSON 数组 | 群成员列表 | 直接存成员 UUID 快照，减少单独成员表设计复杂度 |
| `MemberCnt` | `int` | 默认 `1` | 群人数 | 避免前端每次都去解析成员 JSON 计数 |
| `OwnerId` | `string` | 非空 | 群主用户 ID | 支撑群管理权限判断 |
| `AddMode` | `int8` | `0=直接`，`1=审核` | 入群方式 | 控制是直接进群还是走申请审批 |
| `Avatar` | `string` | 非空，默认 URL | 群头像 | 直接用于联系人列表与会话列表展示 |
| `Status` | `int8` | `0=正常`，`1=禁用`，`2=解散` | 群状态 | 支撑后台禁用群、群主解散群 |
| `CreatedAt` | `time.Time` | 非空 | 创建时间 | 排序和展示用途 |
| `UpdatedAt` | `time.Time` | 非空 | 更新时间 | 标识最近群资料修改时间 |
| `DeletedAt` | `gorm.DeletedAt` | 软删除时间 | 群逻辑删除标记 | 支持后台查看已删除群和恢复链路一致性 |

### 字段值重点说明

<div style="padding: 14px 16px; background:#f8fafc; border:1px solid #cbd5e1; border-radius:10px; line-height:1.9;">
  <b><code>AddMode</code></b><br/>
  <span style="color:#15803d; font-weight:700;">0 = DIRECT</span>：可直接进群。<br/>
  <span style="color:#c2410c; font-weight:700;">1 = AUDIT</span>：需要群主审核。<br/><br/>

  <b><code>Status</code></b><br/>
  <span style="color:#166534; font-weight:700;">0 = NORMAL</span>：群可正常使用。<br/>
  <span style="color:#b91c1c; font-weight:700;">1 = DISABLE</span>：群被后台禁用，无法正常会话或加入。<br/>
  <span style="color:#6b21a8; font-weight:700;">2 = DISSOLVE</span>：业务上表示群已解散，当前代码更多通过软删除来隐藏群。
</div>

### 模型设计意图

GroupInfo 采用“群主 + 成员 JSON + 成员数 + 状态”的紧凑设计，是一种工程上较轻量的群模型。它不追求关系型数据库最规范的建模方式，而是把最常用的群资料和成员快照都集中在一行记录中，这让创建群、获取群详情、校验加群方式等业务都更直接。对于中小规模项目或教学项目，这种方式能显著降低理解和开发门槛。

---

## 5. Session 会话模型

<div style="padding: 14px 16px; border-radius: 12px; background: linear-gradient(135deg, #fdf4ff 0%, #f5d0fe 100%); border:1px solid #e879f9; color:#86198f; margin: 14px 0;">
  <div style="font-size:18px; font-weight:800;">表名：<code>session</code></div>
  <div style="margin-top:8px; line-height:1.85;">
    会话模型描述“某个用户打开了与某个对象的聊天入口”，它本质上是聊天列表页的数据基础，而不是消息本身。
  </div>
</div>

### 字段说明

| 字段 | 类型 | 取值/约束说明 | 字段含义 | 设计意图 |
| --- | --- | --- | --- | --- |
| `Id` | `int64` | 主键、自增 | 数据库内部主键 | 数据库索引用途 |
| `Uuid` | `string` | 唯一索引，业务上通常形如 `S...` | 会话业务 ID | 会话对象需要单独业务标识，供前端和消息表引用 |
| `SendId` | `string` | 索引，非空 | 创建该会话入口的用户 ID | 表示“这个聊天入口属于谁” |
| `ReceiveId` | `string` | 索引，非空 | 会话目标对象 ID | 目标可能是用户也可能是群 |
| `ReceiveName` | `string` | 非空 | 会话目标显示名 | 避免前端列表页每次都回查用户/群资料 |
| `Avatar` | `string` | 非空 | 会话目标头像 | 会话列表直接展示用 |
| `LastMessage` | `string` | `TEXT` | 最新消息摘要 | 支撑聊天列表最近消息预览 |
| `LastMessageAt` | `sql.NullTime` | 可空 | 最近消息时间 | 支撑聊天列表按最近活跃排序 |
| `CreatedAt` | `time.Time` | 建立会话时间 | 会话创建时间 | 作为列表排序和生命周期信息 |
| `DeletedAt` | `gorm.DeletedAt` | 软删除时间 | 会话删除标记 | 删除会话入口时不必删除历史消息 |

### 字段值重点说明

<div style="padding: 14px 16px; background:#f8fafc; border:1px solid #cbd5e1; border-radius:10px; line-height:1.9;">
  Session 本身没有复杂枚举字段，它更像是聊天列表的“索引项”。<br/>
  其中 <code>ReceiveId</code> 通过业务前缀区分对象类型：<br/>
  <span style="color:#0f766e; font-weight:700;">U...</span>：私聊对象。<br/>
  <span style="color:#0369a1; font-weight:700;">G...</span>：群聊对象。
</div>

### 模型设计意图

Session 的设计目标不是保存全部消息，而是让“聊天列表页”能快速展示：和谁聊、头像是什么、最近一条消息是什么、最近活跃时间是什么。把这些摘要信息单独做成一张表，可以避免每次打开首页都去消息表做复杂聚合查询，是典型的 IM 系统性能换空间做法。

---

## 6. Message 消息模型

<div style="padding: 14px 16px; border-radius: 12px; background: linear-gradient(135deg, #fff1f2 0%, #ffe4e6 100%); border:1px solid #fda4af; color:#9f1239; margin: 14px 0;">
  <div style="font-size:18px; font-weight:800;">表名：<code>message</code></div>
  <div style="margin-top:8px; line-height:1.85;">
    消息模型是聊天业务的核心载体，负责存储文本、文件、语音、音视频信令等多种消息数据，并记录发送状态与时间。
  </div>
</div>

### 字段说明

| 字段 | 类型 | 取值/约束说明 | 字段含义 | 设计意图 |
| --- | --- | --- | --- | --- |
| `Id` | `int64` | 主键、自增 | 数据库内部主键 | 数据库层索引 |
| `Uuid` | `string` | 唯一索引，业务上通常形如 `M...` 或随机串 | 消息业务 ID | 供前后端唯一标识一条消息 |
| `SessionId` | `string` | 索引，非空 | 所属会话 ID | 将消息挂载到具体会话 |
| `Type` | `int8` | `0=文本`，`1=语音`，`2=文件`，`3=通话` | 消息类型 | 用统一表结构容纳多种消息形态 |
| `Content` | `string` | `TEXT` | 文本内容 | 承载文本消息正文 |
| `Url` | `string` | `char(255)` | 资源地址 | 用于文件、语音或通话相关资源引用 |
| `SendId` | `string` | 索引，非空 | 发送者 ID | 标识是谁发出的消息 |
| `SendName` | `string` | 非空 | 发送者昵称 | 直接冗余展示信息，减少消息读取时的联表 |
| `SendAvatar` | `string` | 非空 | 发送者头像 | 同样是为前端消息列表直接展示做冗余 |
| `ReceiveId` | `string` | 索引，非空 | 接收对象 ID | 既可指向用户，也可指向群 |
| `FileType` | `string` | `char(10)` | 文件类型 | 文件消息扩展字段 |
| `FileName` | `string` | `varchar(50)` | 文件名 | 文件消息展示与下载需要 |
| `FileSize` | `string` | `char(20)` | 文件大小 | 文件消息展示需要 |
| `Status` | `int8` | `0=未发送`，`1=已发送` | 发送状态 | 支撑服务端确认、前端消息状态展示 |
| `CreatedAt` | `time.Time` | 非空 | 生成时间 | 消息创建时间，用于排序 |
| `SendAt` | `sql.NullTime` | 可空 | 真实发送时间 | 用于将来更精细地区分创建与送达 |
| `AVdata` | `string` | 可空 | 音视频传递数据 | 承载 SDP、candidate、通话指令等信令载荷 |

### 字段值重点说明

<div style="padding: 14px 16px; background:#f8fafc; border:1px solid #cbd5e1; border-radius:10px; line-height:1.9;">
  <b><code>Type</code></b><br/>
  <span style="color:#166534; font-weight:700;">0 = Text</span>：普通文本消息。<br/>
  <span style="color:#0369a1; font-weight:700;">1 = Voice</span>：语音消息。<br/>
  <span style="color:#7c3aed; font-weight:700;">2 = File</span>：文件消息。<br/>
  <span style="color:#b91c1c; font-weight:700;">3 = AudioOrVideo</span>：音视频通话信令或通话类消息。<br/><br/>

  <b><code>Status</code></b><br/>
  <span style="color:#d97706; font-weight:700;">0 = Unsent</span>：消息已入库但尚未确认成功发送到前端。<br/>
  <span style="color:#15803d; font-weight:700;">1 = Sent</span>：服务端已经通过 WebSocket 成功写回前端。
</div>

### 模型设计意图

Message 采用“一张表承载所有消息类型”的设计，非常符合 IM 业务的快速迭代需求。文本、文件、语音、通话虽然结构不同，但它们都共享“谁发的、发给谁、在哪个会话里、什么时候发、状态如何”这些公共属性，因此统一建模可以显著降低代码复杂度。额外的文件字段和 `AVdata` 则体现了这个模型的扩展性：当消息类型不同，只用填充对应字段即可。

---

## 7. 模型之间的关系理解

<div style="padding: 16px 18px; border-radius: 12px; background:#111827; color:#f9fafb; line-height:1.9; font-family: ui-monospace, SFMono-Regular, Menlo, monospace;">
UserInfo            -- 用户基础资料主表<br/>
UserContact         -- 用户与用户 / 用户与群 的关系表<br/>
ContactApply        -- 正式关系建立前的申请流表<br/>
GroupInfo           -- 群资料与成员快照表<br/>
Session             -- 用户视角的聊天入口索引表<br/>
Message             -- 会话内的消息明细表
</div>

<div style="margin-top: 14px; padding: 16px 18px; border-left: 5px solid #2563eb; background:#eff6ff; border-radius: 10px; color:#1e3a8a; line-height:1.9;">
  <b>关系脉络可以概括为：</b><br/>
  用户先存在于 <code>UserInfo</code> 中；<br/>
  想建立好友或入群关系时，先产生 <code>ContactApply</code>；<br/>
  审批通过后，生成 <code>UserContact</code>；<br/>
  用户真正打开聊天窗口时，会创建或读取 <code>Session</code>；<br/>
  会话中的每一条聊天内容，最终都落到 <code>Message</code>。
</div>

## 8. 设计优缺点总结

### 优点

- 结构清晰，围绕 IM 核心链路展开，容易理解和维护。
- 大部分业务状态都可以通过 `status` 字段快速判断，控制器和 service 写起来直接。
- 使用软删除避免了直接物理删除带来的链路断裂问题。
- 多种消息类型统一建模，前后端联调成本低。

### 需要注意的点

- `Password` 当前设计偏教学风格，若用于生产需要改成哈希存储。
- `GroupInfo.Members` 使用 JSON 而不是独立成员表，在成员数很大时不利于复杂查询。
- `Session` 与 `Message` 中有一定冗余字段，这提升了读取效率，但需要在资料更新时做同步维护。
- 枚举值大量散落在代码和注释中，如果后续系统变大，建议抽成更统一的文档或字典层。

---

<div style="margin-top: 24px; padding: 14px 16px; border-radius: 12px; background: linear-gradient(135deg, #faf5ff 0%, #f3e8ff 100%); border:1px solid #d8b4fe; color:#581c87;">
  <div style="font-size:16px; font-weight:800; margin-bottom:6px;">报告结论</div>
  <div style="line-height:1.85;">
    这套模型设计不是为了做最复杂、最规范的 IM 数据库，而是为了在较低复杂度下完整支撑“注册登录、好友关系、群聊、会话、消息、审批、后台管理”这一整套业务闭环。从工程目标来看，这是一套非常典型的教学型、实战导向型即时通讯模型方案。
  </div>
</div>
