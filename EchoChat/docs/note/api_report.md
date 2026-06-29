<style>
  .api-report {
    --ink: #16324a;
    --muted: #57718b;
    --line: rgba(33, 82, 120, 0.14);
    --panel: rgba(255, 255, 255, 0.94);
    --panel-strong: #ffffff;
    --accent: #0f6cab;
    --accent-2: #f97316;
    --accent-3: #14b8a6;
    max-width: 1240px;
    margin: 0 auto;
    padding: 28px 22px 80px;
    color: var(--ink);
    line-height: 1.8;
    font-family: "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif;
    background:
      radial-gradient(circle at top left, rgba(14, 165, 233, 0.12), transparent 26%),
      radial-gradient(circle at top right, rgba(249, 115, 22, 0.12), transparent 24%),
      linear-gradient(180deg, #f3fbff 0%, #fffaf4 46%, #f6fff8 100%);
  }

  .api-report .hero {
    padding: 30px 32px;
    margin-bottom: 28px;
    border: 1px solid rgba(255, 255, 255, 0.72);
    border-radius: 28px;
    background:
      linear-gradient(135deg, rgba(15, 108, 171, 0.96), rgba(20, 184, 166, 0.9)),
      linear-gradient(135deg, #0f6cab, #14b8a6);
    color: #ffffff;
    box-shadow: 0 22px 56px rgba(15, 108, 171, 0.24);
  }

  .api-report .eyebrow {
    display: inline-block;
    padding: 5px 12px;
    border-radius: 999px;
    font-size: 12px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    background: rgba(255, 255, 255, 0.18);
  }

  .api-report .hero h1 {
    margin: 14px 0 12px;
    font-size: 42px;
    line-height: 1.18;
    color: #ffffff;
  }

  .api-report .hero p {
    margin: 0;
    max-width: 880px;
    font-size: 16px;
    color: rgba(255, 255, 255, 0.92);
  }

  .api-report .hero-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(210px, 1fr));
    gap: 14px;
    margin-top: 22px;
  }

  .api-report .hero-card {
    padding: 16px 18px;
    border-radius: 18px;
    background: rgba(255, 255, 255, 0.14);
    border: 1px solid rgba(255, 255, 255, 0.16);
    backdrop-filter: blur(6px);
  }

  .api-report .hero-card span {
    display: block;
    margin-bottom: 6px;
    font-size: 12px;
    color: rgba(255, 255, 255, 0.72);
  }

  .api-report .hero-card strong,
  .api-report .hero-card code {
    font-size: 14px;
    color: #ffffff;
  }

  .api-report .guide {
    margin: 0 0 30px;
    padding: 16px 18px;
    border-left: 6px solid var(--accent-2);
    border-radius: 18px;
    background: linear-gradient(135deg, rgba(255, 244, 231, 0.96), rgba(255, 255, 255, 0.96));
    box-shadow: 0 14px 32px rgba(120, 84, 38, 0.08);
    color: var(--ink);
  }

  .api-report h2 {
    margin: 34px 0 18px;
    padding: 16px 18px;
    border-radius: 22px;
    border: 1px solid rgba(15, 108, 171, 0.1);
    background: linear-gradient(135deg, rgba(15, 108, 171, 0.11), rgba(20, 184, 166, 0.08));
    box-shadow: 0 14px 30px rgba(15, 108, 171, 0.08);
    font-size: 30px;
    line-height: 1.28;
    color: #0d4f85;
  }

  .api-report h3 {
    margin: 28px 0 14px;
    padding: 12px 16px;
    border-left: 5px solid var(--accent-3);
    border-radius: 16px;
    background: rgba(255, 255, 255, 0.88);
    box-shadow: 0 10px 22px rgba(22, 50, 74, 0.06);
    font-size: 22px;
    line-height: 1.45;
    color: #0f5e79;
  }

  .api-report p,
  .api-report li {
    font-size: 15px;
    color: var(--ink);
  }

  .api-report p {
    margin: 10px 0;
  }

  .api-report ul,
  .api-report ol {
    padding-left: 24px;
  }

  .api-report li + li {
    margin-top: 6px;
  }

  .api-report strong {
    color: #0d4f85;
  }

  .api-report a {
    color: var(--accent);
    font-weight: 600;
    text-decoration: none;
  }

  .api-report a:hover {
    text-decoration: underline;
  }

  .api-report code {
    padding: 2px 8px;
    border-radius: 999px;
    background: rgba(15, 108, 171, 0.1);
    color: #0c5c9c;
    font-size: 0.94em;
  }

  .api-report pre {
    padding: 16px;
    overflow-x: auto;
    border-radius: 18px;
    background: #0f172a;
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.06);
  }

  .api-report pre code {
    padding: 0;
    background: transparent;
    color: #e2e8f0;
    border-radius: 0;
  }

  .api-report table {
    width: 100%;
    margin: 16px 0 22px;
    border-collapse: separate;
    border-spacing: 0;
    overflow: hidden;
    border: 1px solid var(--line);
    border-radius: 20px;
    background: var(--panel-strong);
    box-shadow: 0 18px 36px rgba(22, 50, 74, 0.08);
  }

  .api-report thead th {
    padding: 14px 16px;
    background: linear-gradient(135deg, #0f6cab, #14b8a6);
    color: #ffffff;
    font-size: 14px;
    text-align: left;
  }

  .api-report tbody td {
    padding: 13px 16px;
    border-top: 1px solid var(--line);
    vertical-align: top;
  }

  .api-report tbody tr:nth-child(even) {
    background: rgba(15, 108, 171, 0.04);
  }

  .api-report hr {
    border: 0;
    height: 1px;
    margin: 28px 0;
    background: linear-gradient(90deg, transparent, rgba(15, 108, 171, 0.22), transparent);
  }

  .api-report blockquote {
    margin: 18px 0;
    padding: 14px 18px;
    border-left: 5px solid var(--accent-2);
    border-radius: 16px;
    background: rgba(255, 247, 237, 0.9);
    color: #7c4312;
  }

  @media (max-width: 768px) {
    .api-report {
      padding: 18px 12px 60px;
    }

    .api-report .hero {
      padding: 24px 20px;
      border-radius: 22px;
    }

    .api-report .hero h1 {
      font-size: 32px;
    }

    .api-report h2 {
      font-size: 24px;
    }

    .api-report h3 {
      font-size: 19px;
    }
  }
</style>

<div class="api-report">
  <section class="hero">
    <span class="eyebrow">EchoChat Backend API</span>
    <h1>EchoChat API 接口详情报告</h1>
    <p>本文档基于当前项目代码生成，覆盖当前仓库中可见的 HTTP / WebSocket 接口、统一返回结构、缓存路径、数据库写入点与主要业务调用链。</p>
    <div class="hero-grid">
      <div class="hero-card">
        <span>路由入口</span>
        <strong><code>internal/https_server/https_server.go</code></strong>
      </div>
      <div class="hero-card">
        <span>返回封装</span>
        <strong><code>api/v1/controller.go</code></strong>
      </div>
      <div class="hero-card">
        <span>核心业务层</span>
        <strong><code>internal/service/gorm</code></strong>
      </div>
      <div class="hero-card">
        <span>协议范围</span>
        <strong>HTTP + WebSocket</strong>
      </div>
    </div>
  </section>

  <div class="guide">
    <strong>阅读建议：</strong>先看“通用说明”理解统一返回和枚举，再按“用户认证 / 联系人 / 消息 / 群组 / 管理 / WebSocket”的顺序查阅，定位会更快。
  </div>

## 1. 通用说明

### 1.1 统一返回结构

绝大多数 HTTP 接口最终都会走 `api/v1/controller.go` 中的 `JsonBack` 方法返回 JSON。统一格式如下：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `code` | `int` | `200` 表示成功，`400` 表示业务失败，`500` 表示系统错误 |
| `message` | `string` | 接口执行结果描述 |
| `data` | `any` | 成功时按接口返回具体数据；部分接口无该字段 |

说明：

- `ret == 0` 时返回 `code: 200`
- `ret == -2` 时返回 `code: 400`
- `ret == -1` 时返回 `code: 500`

### 1.2 当前项目中的通用技术路径

- HTTP 层使用 Gin。
- 业务层主要在 `internal/service/gorm` 中完成。
- 数据库使用 Gorm 访问 MySQL。
- 部分列表或详情接口会先尝试读 Redis，未命中时再回源 MySQL。
- 文件上传通过 `multipart/form-data` 直接落到本地静态目录。
- WebSocket 连接入口为 `GET /wss`，实际消息收发由 `internal/service/chat` 处理。

### 1.3 枚举说明

常见枚举含义如下：

- 用户状态 `status`：`0=正常`，`1=禁用`
- 群状态 `status`：`0=正常`，`1=禁用`，`2=解散`
- 加群方式 `add_mode`：`0=直接进群`，`1=需要审核`
- 联系人状态 `status`：`0=正常`，`1=被拉黑`，`2=拉黑对方`，`3=被删除`，`4=删除对方`，`5=禁言`，`6=退出群聊`，`7=被踢出群聊`
- 申请状态 `status`：`0=待处理`，`1=已同意`，`2=已拒绝`，`3=已拉黑`
- 消息类型 `type`：`0=文本`，`1=语音`，`2=文件`，`3=音视频通话`

## 2. 用户与认证类接口

### `POST /register`

**用途是什么**

用于新用户注册账号。这个接口负责把前端输入的手机号、密码、昵称和短信验证码组合成一个新的用户记录，并在注册成功后把该用户的基础资料直接返回给前端，方便前端立刻把用户态写入本地缓存并进入聊天页面。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `telephone` | `string` | 是 | 注册手机号 |
| `password` | `string` | 是 | 登录密码 |
| `nickname` | `string` | 是 | 昵称 |
| `sms_code` | `string` | 是 | 短信验证码 |

**传回了什么数据给前端**

成功时 `data` 为 `RegisterRespond`：

| 字段 | 说明 |
| --- | --- |
| `uuid` | 新用户 ID，格式形如 `U...` |
| `nickname` | 昵称 |
| `telephone` | 手机号 |
| `avatar` | 头像地址 |
| `email` | 邮箱 |
| `gender` | 性别 |
| `birthday` | 生日 |
| `signature` | 个性签名 |
| `created_at` | 创建日期文本 |
| `is_admin` | 是否管理员 |
| `status` | 用户状态 |

失败时只返回 `code` 和 `message`。

**接口从调用到传回数据中间调用了什么操作**

控制器先把 JSON 绑定到 `RegisterRequest`，然后调用 `gorm.UserInfoService.Register`。服务层先从 Redis 读取 `auth_code_<telephone>` 验证码键，验证失败直接返回业务错误；验证成功后会删除该验证码键，避免重复使用。随后服务层检查手机号是否已经存在，如果不存在则生成新的用户 UUID，填充默认头像、创建时间、默认状态和管理员标记，再通过 Gorm 写入 `user_info` 表。最后把新建用户转换为 `RegisterRespond` 返回给前端。

### `POST /login`

**用途是什么**

用于账号密码登录。前端通过手机号和密码调用该接口，后端负责校验用户是否存在以及密码是否匹配，并把用户的完整基础资料返回给前端，用于初始化页面和后续 WebSocket 登录。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `telephone` | `string` | 是 | 登录手机号 |
| `password` | `string` | 是 | 登录密码 |

**传回了什么数据给前端**

成功时 `data` 为 `LoginRespond`，字段与注册成功返回基本一致：`uuid`、`nickname`、`telephone`、`avatar`、`email`、`gender`、`birthday`、`signature`、`created_at`、`is_admin`、`status`。

**接口从调用到传回数据中间调用了什么操作**

控制器将请求体绑定为 `LoginRequest` 后交给 `UserInfoService.Login`。服务层直接查询 `user_info` 表中手机号对应的用户记录；如果记录不存在则返回“用户不存在，请注册”，如果密码不匹配则返回“密码不正确，请重试”。校验通过后，服务层把数据库实体转换成 `LoginRespond` 返回。这个接口本身不创建 WebSocket 连接，只负责完成认证和用户资料回传。

### `POST /user/smsLogin`

**用途是什么**

用于手机号加验证码登录。它适用于用户不输入密码、直接依赖短信验证码完成登录的场景。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `telephone` | `string` | 是 | 登录手机号 |
| `sms_code` | `string` | 是 | 短信验证码 |

**传回了什么数据给前端**

成功时 `data` 为 `LoginRespond`，字段与 `/login` 相同。

**接口从调用到传回数据中间调用了什么操作**

控制器把 JSON 绑定到 `SmsLoginRequest`，服务层先查 `user_info` 表确认用户存在，然后从 Redis 读取 `auth_code_<telephone>` 验证码并比较。如果验证码错误则返回业务错误；正确时会删除该验证码键，防止二次复用。最后把用户信息组装成 `LoginRespond` 返回。流程上与密码登录相比，多了一步 Redis 验证码校验。

### `POST /user/sendSmsCode`

**用途是什么**

用于向指定手机号发送登录或注册验证码。该接口是注册和验证码登录的前置步骤。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `telephone` | `string` | 是 | 目标手机号 |

**传回了什么数据给前端**

成功时不返回 `data`，只返回 `message`。当前代码在未配置阿里云短信 AK/SK 时，会把验证码直接拼到 `message` 里返回，例如“验证码已生成，当前开发环境验证码为: 123456”。

**接口从调用到传回数据中间调用了什么操作**

控制器将手机号传给 `UserInfoService.SendSmsCode`，后者继续调用 `sms.VerificationCode`。短信服务先检查 Redis 中是否已有未过期验证码；如果已有，会直接提示用户使用已发送的验证码。如果没有，则生成新的 6 位随机验证码，写入 Redis，过期时间为 1 分钟。若阿里云短信配置为空，则直接走开发环境降级逻辑，把验证码写入返回消息；若配置齐全，则再调用阿里云短信 SDK 发送短信。

### `POST /user/updateUserInfo`

**用途是什么**

用于用户修改自己的资料，包括邮箱、昵称、生日、签名和头像。这个接口适合“个人资料”页面提交保存时调用。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `uuid` | `string` | 是 | 当前用户 ID |
| `email` | `string` | 否 | 新邮箱 |
| `nickname` | `string` | 否 | 新昵称 |
| `birthday` | `string` | 否 | 生日 |
| `signature` | `string` | 否 | 个性签名 |
| `avatar` | `string` | 否 | 头像路径 |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“修改用户信息成功”。

**接口从调用到传回数据中间调用了什么操作**

控制器绑定 `UpdateUserInfoRequest` 后调用 `UserInfoService.UpdateUserInfo`。服务层先按 `uuid` 查出用户记录，然后只更新那些传入值不为空的字段，最后通过 Gorm `Save` 回写到 `user_info` 表。当前实现没有做复杂的字段校验，也没有强制刷新联系人缓存，只是依靠缓存自然过期。

### `POST /user/getUserInfo`

**用途是什么**

用于获取某个用户的完整个人资料，常用于页面刷新后重新拉取登录态信息，或者个人中心初始化展示。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `uuid` | `string` | 是 | 用户 ID |

**传回了什么数据给前端**

成功时 `data` 为 `GetUserInfoRespond`：`uuid`、`nickname`、`telephone`、`avatar`、`email`、`gender`、`birthday`、`signature`、`created_at`、`is_admin`、`status`。

**接口从调用到传回数据中间调用了什么操作**

控制器把请求映射为 `GetUserInfoRequest` 后，服务层先尝试从 Redis 读取 `user_info_<uuid>`。如果 Redis 命中，则直接反序列化返回；如果未命中，则查询 `user_info` 表，组装 `GetUserInfoRespond` 后返回。当前代码中写回 Redis 的逻辑被注释掉了，所以实际运行时多数情况会直接查数据库。

### `POST /user/getUserInfoList`

**用途是什么**

用于管理员获取全站用户列表。这个接口主要服务后台管理功能，例如查看所有用户、判断用户是否被删除、是否为管理员、是否被禁用等。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 当前管理员用户 ID，用于排除自己 |

**传回了什么数据给前端**

成功时 `data` 为数组，每项结构为 `GetUserListRespond`：

| 字段 | 说明 |
| --- | --- |
| `uuid` | 用户 ID |
| `nickname` | 昵称 |
| `telephone` | 手机号 |
| `status` | 用户状态 |
| `is_admin` | 是否管理员 |
| `is_deleted` | 是否已被软删除 |

**接口从调用到传回数据中间调用了什么操作**

控制器读取 `owner_id` 后，服务层直接对 `user_info` 执行 `Unscoped` 查询，这样连已软删除用户也会一起读出来。然后服务层过滤掉当前管理员自己，把每个用户转换成简化列表结构，并根据 `DeletedAt.Valid` 计算 `is_deleted`。这个接口没有使用 Redis 缓存，目的是保证后台管理看到的始终是较新的数据库状态。

### `POST /user/ableUsers`

**用途是什么**

用于管理员批量启用用户，把被禁用的用户恢复到正常状态。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `uuid_list` | `string[]` | 是 | 需要恢复的用户 ID 列表 |
| `is_admin` | `int8` | 否 | 该接口本身不会使用该字段 |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“启用用户成功”。

**接口从调用到传回数据中间调用了什么操作**

控制器绑定 `AbleUsersRequest`，服务层用 `uuid in (?)` 批量查出目标用户，然后逐个把 `status` 改为 `NORMAL` 并保存回数据库。当前实现没有额外恢复历史会话，也没有做缓存清理，只负责把用户本身状态切回可用。

### `POST /user/disableUsers`

**用途是什么**

用于管理员批量禁用用户。用户被禁用后，其他人将无法继续与其发起新会话，现有会话也会被软删除。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `uuid_list` | `string[]` | 是 | 需要禁用的用户 ID 列表 |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“禁用用户成功”。

**接口从调用到传回数据中间调用了什么操作**

控制器把请求交给 `UserInfoService.DisableUsers`。服务层先查询所有目标用户，把每个用户的 `status` 设为 `DISABLE` 并保存；随后再查出所有与该用户有关的会话记录，包括其发出的和接收的会话，把这些会话的 `DeletedAt` 设置为当前时间，从而在业务上隐藏这些会话。这样可以保证用户一旦被禁用，不会继续作为有效聊天对象出现。

### `POST /user/deleteUsers`

**用途是什么**

用于管理员批量删除用户。这里的“删除”是软删除，不会真正物理删除数据，而是统一打上删除时间，用于后台可追踪和前台不可见。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `uuid_list` | `string[]` | 是 | 需要删除的用户 ID 列表 |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“删除用户成功”。

**接口从调用到传回数据中间调用了什么操作**

控制器绑定 `AbleUsersRequest` 后调用 `UserInfoService.DeleteUsers`。服务层会先把 `user_info` 中的用户做软删除，然后继续查找与这些用户有关的所有 `session`、`user_contact` 和 `contact_apply` 记录，统一设置 `DeletedAt`。因此这个接口不仅隐藏用户自己，也会同步清理该用户关联的联系人关系、会话关系和申请记录。

### `POST /user/setAdmin`

**用途是什么**

用于管理员批量设置或取消其他用户的管理员身份，服务于后台权限管理。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `uuid_list` | `string[]` | 是 | 目标用户 ID 列表 |
| `is_admin` | `int8` | 是 | 目标管理员标记，`0` 表示普通用户，`1` 表示管理员 |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“设置管理员成功”。

**接口从调用到传回数据中间调用了什么操作**

控制器读取列表和管理员标记后，服务层按用户 ID 批量查询用户记录，并逐个把 `IsAdmin` 更新成传入值。整个过程只修改 `user_info` 表，不涉及联系人、会话或缓存。

## 3. 群组管理类接口

### `POST /group/createGroup`

**用途是什么**

用于创建新的群聊。这个接口既会创建群本身，也会自动把群主加入群，并建立群主与该群之间的联系人关系。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 群主用户 ID |
| `name` | `string` | 是 | 群名称 |
| `notice` | `string` | 否 | 群公告 |
| `add_mode` | `int8` | 是 | 入群方式，`0=直接进群`，`1=审核` |
| `avatar` | `string` | 否 | 群头像 |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“创建成功”。

**接口从调用到传回数据中间调用了什么操作**

控制器绑定 `CreateGroupRequest` 后，服务层生成群 UUID，构造 `group_info` 记录，初始成员数为 1，并把群主 ID 序列化成 `members` JSON 数组存入数据库。随后再创建一条 `user_contact` 记录，表示群主已经拥有该群联系人关系，联系类型为群聊、状态为正常。最后删除 `contact_mygroup_list_<owner_id>` 相关缓存，让前端下次重新加载时能看到刚创建的群。

### `POST /group/loadMyGroup`

**用途是什么**

用于获取“我创建的群聊”列表，适合联系人页或群管理页左侧列表展示。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 当前用户 ID |

**传回了什么数据给前端**

成功时 `data` 为数组，每项结构为 `LoadMyGroupRespond`：`group_id`、`group_name`、`avatar`。

**接口从调用到传回数据中间调用了什么操作**

服务层先尝试读取 Redis 中的 `contact_mygroup_list_<owner_id>`。如果缓存命中，直接反序列化后返回；如果未命中，则查询 `group_info` 中 `owner_id` 等于当前用户的群聊列表，按创建时间倒序排序，再转换成简化的前端展示结构，并写回 Redis。整个流程是典型的“先缓存、后数据库”的群列表读取模式。

### `POST /group/checkGroupAddMode`

**用途是什么**

用于在申请加群前检查某个群的入群方式，前端可据此决定是直接调用进群接口，还是走申请审核接口。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `group_id` | `string` | 是 | 群聊 ID |

**传回了什么数据给前端**

成功时 `data` 是一个整数：`0` 表示直接进群，`1` 表示审核后进群。

**接口从调用到传回数据中间调用了什么操作**

服务层优先尝试从 Redis 的 `group_info_<group_id>` 中读取群详情；如果缓存未命中，则直接查询 `group_info` 表并取出 `AddMode` 字段。如果缓存命中，就把缓存反序列化成 `GetGroupInfoRespond`，再把其中的 `AddMode` 返回给前端。这个接口不改数据，只做读操作。

### `POST /group/enterGroupDirectly`

**用途是什么**

用于直接进群，适合群设置为“无需审核”的场景。注意当前请求体中的 `owner_id` 在这个接口里实际表示群 ID。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 这里实际传群 ID |
| `contact_id` | `string` | 是 | 要加入群的用户 ID |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“进群成功”。

**接口从调用到传回数据中间调用了什么操作**

服务层先读出目标群记录并解析 `members` JSON 数组，把新用户 ID 追加进去后重新写回群记录，同时将 `member_cnt` 加 1。接着创建一条 `user_contact` 记录，表示该用户已经与群建立联系。最后删除群会话列表和“我加入的群”列表相关缓存，确保前端重新拉取时能看到新群关系。

### `POST /group/leaveGroup`

**用途是什么**

用于成员主动退群。退群后，该成员与群之间的联系人关系、会话关系以及相关申请记录都会被软删除或更新状态。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `user_id` | `string` | 是 | 退群用户 ID |
| `group_id` | `string` | 是 | 目标群 ID |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“退群成功”。

**接口从调用到传回数据中间调用了什么操作**

服务层先读取群记录并解析成员数组，把当前用户从 `members` 中移除，再回写更新后的成员数组与人数。随后它会软删除该用户与群的会话记录，把对应 `user_contact` 的状态改成“退出群聊”，再把该用户对该群的申请记录做软删除。最后清理该用户的群会话缓存和“我加入的群”缓存。

### `POST /group/dismissGroup`

**用途是什么**

用于群主主动解散群聊。解散后，群本身及所有关联关系会被统一软删除。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 群主 ID |
| `group_id` | `string` | 是 | 目标群 ID |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“解散群聊成功”。

**接口从调用到传回数据中间调用了什么操作**

服务层先将 `group_info` 对应群记录做软删除，并更新 `updated_at`。然后继续查出所有 `receive_id` 指向该群的会话，将这些会话全部软删除；再查出所有 `contact_id` 指向该群的联系人关系记录并做软删除；最后查出所有针对该群的申请记录并软删除。流程结束后，服务层还会清理群主的“我创建的群”缓存、群会话缓存以及全局“我加入的群”缓存。

### `POST /group/getGroupInfo`

**用途是什么**

用于获取群详情，供聊天页右侧资料卡、群设置页和管理员页面展示。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `group_id` | `string` | 是 | 群 ID |

**传回了什么数据给前端**

成功时 `data` 为 `GetGroupInfoRespond`：`uuid`、`name`、`notice`、`member_cnt`、`owner_id`、`add_mode`、`status`、`avatar`、`is_deleted`。

**接口从调用到传回数据中间调用了什么操作**

服务层先尝试读取 Redis 中的 `group_info_<group_id>`。如果未命中，则查询 `group_info` 表并组装 `GetGroupInfoRespond`；如果命中则直接反序列化。组装响应时还会根据群记录的 `DeletedAt` 判断 `is_deleted`。当前代码中的写缓存逻辑被注释掉了，所以多数情况下会直接走数据库查询。

### `POST /group/getGroupInfoList`

**用途是什么**

用于管理员查看系统中的全部群聊列表，包括已经软删除的群。这个接口适合后台群管理页面。

**传入什么变量**

无请求体。

**传回了什么数据给前端**

成功时 `data` 为数组，每项结构为 `GetGroupListRespond`：`uuid`、`name`、`owner_id`、`status`、`is_deleted`。

**接口从调用到传回数据中间调用了什么操作**

控制器不做参数绑定，直接调用 `GroupInfoService.GetGroupInfoList`。服务层通过 `Unscoped()` 查询整个 `group_info` 表，把正常群和已软删除群一起读出，然后转换成管理员列表结构，并按 `DeletedAt.Valid` 推导 `is_deleted`。当前实现没有使用缓存。

### `POST /group/deleteGroups`

**用途是什么**

用于管理员批量删除群聊。和群主“解散群聊”不同，这个接口面向后台，支持一次处理多个群 ID。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `uuid_list` | `string[]` | 是 | 群 ID 列表 |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“解散/删除群聊成功”。

**接口从调用到传回数据中间调用了什么操作**

服务层会遍历每个群 ID，依次软删除 `group_info`、所有相关 `session`、所有相关 `user_contact`、以及所有相关 `contact_apply` 记录。因为这是管理员后台接口，所以它是按列表批量执行的。执行结束后还会批量清理“我创建的群”和群会话相关 Redis 缓存，保证后续查询不会读到旧数据。

### `POST /group/setGroupsStatus`

**用途是什么**

用于管理员批量启用或禁用群聊。禁用后，前端在发起会话或申请加群时会被业务校验拦住。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `uuid_list` | `string[]` | 是 | 群 ID 列表 |
| `status` | `int8` | 是 | 目标状态，通常 `0=正常`，`1=禁用` |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“设置成功”。

**接口从调用到传回数据中间调用了什么操作**

服务层遍历所有目标群，逐个更新 `group_info.status`。如果目标状态是禁用，还会进一步查出所有 `receive_id` 指向该群的会话，并将这些会话做软删除，让被禁用的群不会继续出现在有效聊天场景中。这个接口只改状态，不删除群本身。

### `POST /group/updateGroupInfo`

**用途是什么**

用于更新群资料，例如改群名、改头像、改公告、改入群方式。聊天页和群资料页保存时都会依赖它。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 当前操作者，代码里主要用于前端上下文 |
| `uuid` | `string` | 是 | 群 ID |
| `name` | `string` | 否 | 新群名 |
| `avatar` | `string` | 否 | 新头像 |
| `add_mode` | `int8` | 否 | 新入群方式，`-1` 表示不改 |
| `notice` | `string` | 否 | 新公告 |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“更新成功”。

**接口从调用到传回数据中间调用了什么操作**

服务层先按群 ID 查出群记录，然后仅更新那些被传入的字段。群资料保存成功后，还会查出所有 `receive_id` 为该群的会话，把每条会话中的 `ReceiveName` 和 `Avatar` 同步更新为最新群名和群头像，这样用户的会话列表可以直接展示最新信息，而不必重新创建会话。

### `POST /group/getGroupMemberList`

**用途是什么**

用于获取某个群的成员列表，常用于群成员管理弹窗和群资料页。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `group_id` | `string` | 是 | 群 ID |

**传回了什么数据给前端**

成功时 `data` 为数组，每项结构为 `GetGroupMemberListRespond`：`user_id`、`nickname`、`avatar`。

**接口从调用到传回数据中间调用了什么操作**

服务层会先尝试从 Redis 读取 `group_memberlist_<group_id>`；如果未命中，则从 `group_info` 中取出该群的 `members` JSON 数组，再逐个按成员 UUID 查询 `user_info` 表，把用户昵称和头像拼成返回列表。当前代码中的写缓存逻辑被注释掉，所以实际主要依赖数据库查询。

### `POST /group/removeGroupMembers`

**用途是什么**

用于群主从群中移除一个或多个成员。它本质上等价于“管理员踢人”，并且不能移除群主自己。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `group_id` | `string` | 是 | 群 ID |
| `owner_id` | `string` | 是 | 群主 ID |
| `uuid_list` | `string[]` | 是 | 要移除的成员用户 ID 列表 |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“移除群聊成员成功”。如果列表中包含群主自己，会返回业务错误“不能移除群主”。

**接口从调用到传回数据中间调用了什么操作**

服务层先读取群记录并解析成员数组，然后遍历待移除用户列表；如果发现待移除对象就是群主，立刻终止并返回错误。对于每个普通成员，服务层会把该用户从成员数组中移除，递减群人数，并软删除该用户与该群之间的会话、联系人关系以及入群申请记录。所有成员处理完成后再把更新后的成员数组写回群记录，并清理群会话列表和“我加入的群”缓存。

## 4. 会话管理类接口

### `POST /session/checkOpenSessionAllowed`

**用途是什么**

用于在真正打开聊天会话前做前置校验，确认当前用户是否有资格与目标用户或群继续聊天。例如对方是否已禁用、是否存在拉黑关系等。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `send_id` | `string` | 是 | 当前发起方 ID |
| `receive_id` | `string` | 是 | 目标用户或群 ID |

**传回了什么数据给前端**

成功时 `data` 为布尔值：`true` 表示允许发起会话，`false` 表示不允许。若校验失败通常返回 `400` 并带原因消息。

**接口从调用到传回数据中间调用了什么操作**

服务层先查询 `user_contact` 中当前用户和目标对象之间的关系，判断是否是“被对方拉黑”或“已拉黑对方”。如果关系层面可通过，再根据 `receive_id` 的首字母判断目标是用户还是群：若是用户，则检查 `user_info.status` 是否禁用；若是群，则检查 `group_info.status` 是否禁用。全部通过后才返回“可以发起会话”。

### `POST /session/openSession`

**用途是什么**

用于打开某个聊天会话。如果会话已存在则直接返回会话 ID；如果会话不存在则自动创建一个新的会话。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `send_id` | `string` | 是 | 会话创建者 |
| `receive_id` | `string` | 是 | 对方用户或群 |

**传回了什么数据给前端**

成功时 `data` 为会话 ID 字符串，例如 `S2026...`。

**接口从调用到传回数据中间调用了什么操作**

服务层先尝试按 `session_<send_id>_<receive_id>` 前缀从 Redis 查会话；如果缓存没有，再去 `session` 表按 `(send_id, receive_id)` 查找现有会话。若数据库里已有记录，则直接把已有会话 UUID 返回；若数据库中也不存在，则继续调用 `CreateSession` 创建新会话。创建时会读取接收方是用户还是群，并把会话展示名和头像一并写入 `session` 表，同时清理当前用户的个人会话列表和群会话列表缓存。

### `POST /session/getUserSessionList`

**用途是什么**

用于获取当前用户的“私聊会话列表”，也就是联系人维度的聊天会话。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 当前用户 ID |

**传回了什么数据给前端**

成功时 `data` 为数组，每项结构为 `UserSessionListRespond`：`session_id`、`avatar`、`user_id`、`user_name`。

**接口从调用到传回数据中间调用了什么操作**

服务层先读 Redis 的 `session_list_<owner_id>`。未命中时会查询 `session` 表中 `send_id = owner_id` 的所有记录，并按创建时间倒序排序，再筛选出 `receive_id` 以 `U` 开头的私聊会话，转成前端所需的轻量列表结构后写回 Redis。命中缓存时则直接反序列化返回。

### `POST /session/getGroupSessionList`

**用途是什么**

用于获取当前用户的“群聊会话列表”，与私聊会话列表分开展示。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 当前用户 ID |

**传回了什么数据给前端**

成功时 `data` 为数组，每项结构为 `GroupSessionListRespond`：`session_id`、`group_name`、`group_id`、`avatar`。

**接口从调用到传回数据中间调用了什么操作**

逻辑与 `/session/getUserSessionList` 基本一致，只是最终会筛选 `receive_id` 以 `G` 开头的群会话，并缓存到 `group_session_list_<owner_id>`。返回给前端的每条记录都只保留群展示需要的核心字段。

### `POST /session/deleteSession`

**用途是什么**

用于删除某个会话入口。这里的删除也是软删除，会让该会话从会话列表里消失，但不一定物理删除消息历史。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 当前用户 ID |
| `session_id` | `string` | 是 | 会话 ID |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“删除成功”。

**接口从调用到传回数据中间调用了什么操作**

服务层先按会话 UUID 查出目标会话，然后把该会话的 `DeletedAt` 设为当前时间并保存，实现软删除。随后它会删除当前用户的群会话列表缓存和个人会话列表缓存，这样前端下次重新加载会话列表时不会再看到这条会话。

## 5. 联系人与申请类接口

### `POST /contact/getUserList`

**用途是什么**

用于获取当前用户的联系人列表。这里返回的是“用户联系人”，不包含群聊；同时它会尽量保留业务关系信息，所以被禁用或存在拉黑关系的联系人仍可能出现在列表里，由前端在进一步操作时提示不可聊天。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 当前用户 ID |

**传回了什么数据给前端**

成功时 `data` 为数组，每项结构为 `MyUserListRespond`：`user_id`、`user_name`、`avatar`。

**接口从调用到传回数据中间调用了什么操作**

服务层先尝试读取 Redis 中的 `contact_user_list_<owner_id>`。未命中时，会去 `user_contact` 表查出当前用户所有未处于“删除对方”状态的联系人关系，再逐条筛选其中 `contact_type = USER` 的记录，并按联系人 ID 到 `user_info` 表读取昵称和头像，组合成列表。最后该列表会被写入 Redis，供后续读取直接复用。

### `POST /contact/loadMyJoinedGroup`

**用途是什么**

用于获取当前用户加入的群聊列表，但不包含自己创建的群。前端常用它在联系人页中展示“我加入的群”。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 当前用户 ID |

**传回了什么数据给前端**

成功时 `data` 为数组，每项结构为 `LoadMyJoinedGroupRespond`：`group_id`、`group_name`、`avatar`。

**接口从调用到传回数据中间调用了什么操作**

服务层先读 Redis 的 `my_joined_group_list_<owner_id>`。如果没有缓存，则查询 `user_contact` 表中该用户的全部群聊关系，过滤掉“已退群”和“被踢出群聊”状态，再按每条关系中的群 ID 到 `group_info` 表读取群信息。同时代码还会排除群主就是当前用户的群，以保证结果只包含“加入的群”。最后转换成前端列表结构并写入 Redis。

### `POST /contact/getContactInfo`

**用途是什么**

用于获取联系人或群聊的详细资料。前端打开联系人资料卡、群资料卡、群设置面板时都需要这个接口。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `contact_id` | `string` | 是 | 用户 ID 或群 ID，代码通过首字母判断类型 |

**传回了什么数据给前端**

成功时 `data` 为 `GetContactInfoRespond`：

| 字段 | 说明 |
| --- | --- |
| `contact_id` | 联系对象 ID |
| `contact_name` | 联系对象名称 |
| `contact_avatar` | 头像 |
| `contact_phone` | 用户手机号，群场景为空 |
| `contact_email` | 用户邮箱，群场景为空 |
| `contact_gender` | 用户性别，群场景为空 |
| `contact_signature` | 用户签名，群场景为空 |
| `contact_birthday` | 用户生日，群场景为空 |
| `contact_notice` | 群公告，用户场景为空 |
| `contact_members` | 群成员 ID 列表的原始 JSON |
| `contact_member_cnt` | 群成员数 |
| `contact_owner_id` | 群主 ID |
| `contact_add_mode` | 入群方式 |

**接口从调用到传回数据中间调用了什么操作**

服务层会先根据 `contact_id` 首字母判断是群还是用户。如果是群，就查询 `group_info`，并校验群状态不是禁用，再把群名、头像、公告、成员列表、群主、人数和入群方式返回；如果是用户，就查询 `user_info`，并校验用户未被禁用，然后返回昵称、头像、手机号、邮箱、生日、签名等信息。这个接口不依赖 Redis，属于直接详情查询。

### `POST /contact/deleteContact`

**用途是什么**

用于删除好友关系。注意该接口只处理“用户与用户”的联系人关系，不处理群成员关系。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 当前用户 ID |
| `contact_id` | `string` | 是 | 要删除的好友 ID |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“删除联系人成功”。

**接口从调用到传回数据中间调用了什么操作**

服务层会把双方在 `user_contact` 中的关系都做软删除，并分别设置成“删除对方”和“被对方删除”的状态。接着还会软删除双方之间双向的会话记录，以及双方彼此之间的联系人申请记录，目的是让后续重新添加好友时只看新的申请状态。最后删除当前用户的联系人列表缓存。

### `POST /contact/applyContact`

**用途是什么**

用于发起“加好友”或“申请加群”。同一个接口同时覆盖用户申请和群申请，靠 `contact_id` 首字母区分目标类型。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 申请发起人 ID |
| `contact_id` | `string` | 是 | 目标用户 ID 或群 ID |
| `message` | `string` | 否 | 申请附言 |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“申请成功”。如果被拉黑、目标不存在或目标被禁用，则返回业务错误。

**接口从调用到传回数据中间调用了什么操作**

当目标是用户时，服务层先确认该用户存在且未禁用，再检查 `contact_apply` 中是否已有申请记录；没有则新建申请，有则复用原记录。若发现申请状态已是“拉黑”，会直接拒绝当前申请；否则会把状态重置成 `PENDING`，刷新 `last_apply_at` 并保存。目标是群时逻辑类似，只是校验对象改为 `group_info`，申请记录中的 `contact_type` 记为群聊。整个流程只写申请记录，不直接创建联系人关系。

### `POST /contact/getNewContactList`

**用途是什么**

用于获取“新的好友申请”列表，供用户在联系人审批界面查看别人向自己发起的加好友请求。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 当前用户 ID |

**传回了什么数据给前端**

成功时 `data` 为数组，每项结构为 `NewContactListRespond`：`contact_id`、`contact_name`、`contact_avatar`、`message`。

**接口从调用到传回数据中间调用了什么操作**

服务层查询 `contact_apply` 中 `contact_id = owner_id` 且 `status = PENDING` 的记录，也就是“别人申请我”的待处理申请。然后逐条根据申请人的 `user_id` 查询 `user_info` 表，拿到申请人的昵称和头像，再把申请附言包装成展示文案，例如“申请理由：无”或“申请理由：xxx”。最后返回给前端审批列表。

### `POST /contact/passContactApply`

**用途是什么**

用于通过好友申请或加群申请。和申请接口一样，它也是一个双模式接口：`owner_id` 既可能是当前用户 ID，也可能是群 ID。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 用户申请场景下表示当前用户；群申请场景下表示群 ID |
| `contact_id` | `string` | 是 | 申请人用户 ID |

**传回了什么数据给前端**

好友申请通过时返回“已添加该联系人”；加群申请通过时返回“已通过加群申请”。不返回 `data`。

**接口从调用到传回数据中间调用了什么操作**

服务层先从 `contact_apply` 中定位这条申请。如果 `owner_id` 以 `U` 开头，就说明当前是在通过好友申请：服务层先确认申请人用户未被禁用，再把申请状态改成 `AGREE`，然后在 `user_contact` 中创建双向好友关系记录，并清理当前用户的联系人缓存。如果 `owner_id` 不是用户而是群，则说明是在通过加群申请：服务层先检查群状态未禁用，再将申请状态改为 `AGREE`，为申请人创建一条“用户加入群”的联系人关系，随后把申请人追加进群成员列表、重算成员数、回写群记录，并清理“我加入的群”缓存。

### `POST /contact/refuseContactApply`

**用途是什么**

用于拒绝好友申请或加群申请。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 当前用户 ID 或群 ID |
| `contact_id` | `string` | 是 | 申请人用户 ID |

**传回了什么数据给前端**

用户申请场景返回“已拒绝该联系人申请”，群申请场景返回“已拒绝该加群申请”。

**接口从调用到传回数据中间调用了什么操作**

服务层会按 `(contact_id = owner_id, user_id = contact_id)` 找到申请记录，然后把其状态改成 `REFUSE` 并保存。这个接口不会删除申请，也不会创建联系人关系，它只是把待处理申请推进到“已拒绝”状态，供后续前端据此显示处理结果。

### `POST /contact/blackContact`

**用途是什么**

用于把某个好友拉黑。拉黑后，双方的关系状态会分别变为“拉黑对方”和“被对方拉黑”，并且当前会话会被软删除。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 当前用户 ID |
| `contact_id` | `string` | 是 | 被拉黑的好友 ID |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“已拉黑该联系人”。

**接口从调用到传回数据中间调用了什么操作**

服务层会在 `user_contact` 中更新两条对称关系：当前用户侧设为 `BLACK`，对方侧设为 `BE_BLACK`，并同步更新时间。之后它会把当前用户发往该好友的会话记录软删除，使这段关系不再以正常聊天关系继续存在。

### `POST /contact/cancelBlackContact`

**用途是什么**

用于解除对好友的拉黑状态，恢复双方正常联系人关系。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 当前用户 ID |
| `contact_id` | `string` | 是 | 目标好友 ID |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“已解除拉黑该联系人”。如果当前并未处于拉黑关系，会返回业务错误提示无需解除。

**接口从调用到传回数据中间调用了什么操作**

服务层先查当前用户到对方的 `user_contact` 记录，确认其状态确实是 `BLACK`；再查对方到当前用户的记录，确认其状态是 `BE_BLACK`。只有两个条件都满足时，才把这两条关系都恢复为 `NORMAL` 并保存。这个接口不重新创建会话，只恢复联系人状态。

### `POST /contact/getAddGroupList`

**用途是什么**

用于群主查看“待审批的加群申请列表”。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `group_id` | `string` | 是 | 群 ID |

**传回了什么数据给前端**

成功时 `data` 为数组，每项结构为 `AddGroupListRespond`：`contact_id`、`contact_name`、`contact_avatar`、`message`。

**接口从调用到传回数据中间调用了什么操作**

服务层会查询 `contact_apply` 表中 `contact_id = group_id` 且 `status = PENDING` 的申请列表，然后逐条去 `user_info` 表中查申请人的昵称和头像，最后再把附言转成统一展示格式后返回。这个接口本质上就是“群版本的待审批申请列表”。

### `POST /contact/blackApply`

**用途是什么**

用于把某条申请直接拉黑。被拉黑后，该申请不会被视为正常待处理申请，后续重新申请也会受到状态限制。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 当前用户 ID 或群 ID |
| `contact_id` | `string` | 是 | 申请人用户 ID |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“已拉黑该申请”。

**接口从调用到传回数据中间调用了什么操作**

服务层先按申请双方标识找到目标 `contact_apply` 记录，然后直接把其 `status` 更新成 `BLACK` 并保存。这个接口不改联系人关系，也不改会话，只改变申请本身的业务状态。

## 6. 消息与文件类接口

### `POST /message/getMessageList`

**用途是什么**

用于获取两名用户之间的单聊消息历史，供聊天窗口打开时加载历史消息。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `user_one_id` | `string` | 是 | 用户 A ID |
| `user_two_id` | `string` | 是 | 用户 B ID |

**传回了什么数据给前端**

成功时 `data` 为数组，每项结构为 `GetMessageListRespond`：`send_id`、`send_name`、`send_avatar`、`receive_id`、`type`、`content`、`url`、`file_type`、`file_name`、`file_size`、`created_at`。

**接口从调用到传回数据中间调用了什么操作**

服务层会先尝试读取 Redis 的 `message_list_<user_one_id>_<user_two_id>`。若缓存未命中，则直接查询 `message` 表中这两名用户之间双向往来的所有消息，按创建时间升序排列，再将数据库消息对象逐条转换成前端显示结构后返回。当前代码里的写缓存逻辑已注释，因此该接口现状主要依赖数据库查询。

### `POST /message/getGroupMessageList`

**用途是什么**

用于获取某个群聊的消息历史，供群聊天窗口初始化加载。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `group_id` | `string` | 是 | 群 ID |

**传回了什么数据给前端**

成功时 `data` 为数组，每项结构为 `GetGroupMessageListRespond`：`send_id`、`send_name`、`send_avatar`、`receive_id`、`type`、`content`、`url`、`file_type`、`file_name`、`file_size`、`created_at`。

**接口从调用到传回数据中间调用了什么操作**

服务层先尝试从 Redis 读取 `group_messagelist_<group_id>`。如果没有缓存，就查询 `message` 表中 `receive_id = group_id` 的全部记录，并按创建时间升序排序。查到的数据会被转成前端消息展示结构后返回。与私聊历史一样，当前写缓存代码被注释，所以主要是数据库直查。

### `POST /message/uploadAvatar`

**用途是什么**

用于上传头像文件。这个接口既可用于用户头像，也可用于群头像，前端上传成功后通常会把拼出来的静态资源路径再写回资料更新接口。

**传入什么变量**

请求不是 JSON，而是 `multipart/form-data`。代码会遍历整个 multipart 表单中的所有文件字段，因此后端对字段名没有做强约束。

**传回了什么数据给前端**

成功时不返回 `data`，只返回“上传成功”。接口本身不会回传文件 URL。

**接口从调用到传回数据中间调用了什么操作**

服务层直接从 Gin 的 `Context` 中解析 multipart 表单，遍历每个上传文件，确保头像静态目录存在，然后按原始文件名在 `staticAvatarPath` 下创建文件，并把上传内容写入本地磁盘。写入成功后就返回成功消息。需要注意的是，当前实现不会自动重命名文件，前端通常要根据文件名自行拼接 `/static/avatars/<文件名>` 访问地址。

### `POST /message/uploadFile`

**用途是什么**

用于上传聊天中发送的普通文件。上传成功后，前端会把文件名拼成静态文件地址，再作为消息内容发送。

**传入什么变量**

请求格式同样是 `multipart/form-data`，后端会遍历所有文件字段。

**传回了什么数据给前端**

成功时不返回 `data`，只返回“上传成功”。接口本身不返回文件 URL。

**接口从调用到传回数据中间调用了什么操作**

服务层解析 multipart 表单后，确保普通文件静态目录存在，再把每个上传文件按原始文件名写入 `staticFilePath`。和头像上传一样，这里没有额外的业务表写入，也没有重命名或秒传校验，逻辑就是“接收文件并落盘”。

## 7. 聊天室与通话辅助接口

### `POST /chatroom/getCurContactListInChatRoom`

**用途是什么**

用于获取当前聊天室中的联系人列表，主要服务音视频通话或多人实时房间相关场景。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 当前用户 ID |
| `contact_id` | `string` | 是 | 当前会话对象 ID |

**传回了什么数据给前端**

成功时 `data` 为数组，每项结构为 `GetCurContactListInChatRoomRespond`，仅包含一个字段 `contact_id`。

**接口从调用到传回数据中间调用了什么操作**

这个接口不查数据库，也不查 Redis。服务层只是把 `(owner_id, contact_id)` 组合成一个内存 map 键，然后读取全局 `chatRooms` 变量中记录的在线联系人列表，再逐个封装成返回结构。需要额外说明的是，当前代码库里只看到了读取这张内存表的逻辑，没有看到明显的写入入口，因此如果其他流程没有在运行时填充它，该接口返回空数组也是符合当前实现的。

## 8. WebSocket 与在线连接接口

### `GET /wss`

**用途是什么**

这是前端建立 WebSocket 长连接的入口，用于用户登录后接入聊天服务器，接收实时消息、文件消息、通话信令等异步推送。

**传入什么变量**

请求参数使用 QueryString：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `client_id` | `string` | 是 | 当前登录用户 ID |

**传回了什么数据给前端**

这个接口不是普通 JSON 接口。成功时会完成 WebSocket 协议升级，之后通过 WebSocket 文本消息持续收发数据；如果 `client_id` 缺失，则返回普通 JSON：`code=400`，`message=clientId获取失败`。

**接口从调用到传回数据中间调用了什么操作**

控制器先读取 `client_id`。如果参数为空，直接按普通 HTTP 错误返回；如果存在，则调用 `chat.NewClientInit`。该方法会使用 Gorilla WebSocket 完成协议升级，创建 `Client` 对象，并根据当前消息模式把客户端注册到聊天服务器中。随后启动两个 goroutine：`Read` 负责持续读取前端发来的消息并投递到聊天转发链路，`Write` 负责把服务端写回的数据发给前端，同时在消息成功写回后把 `message.status` 更新成已发送。这个入口是整个实时聊天链路的起点。

### `POST /user/wsLogout`

**用途是什么**

用于主动断开某个用户的 WebSocket 连接，通常在前端退出登录、被封禁后强制退出等场景下使用。

**传入什么变量**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner_id` | `string` | 是 | 要退出的用户 ID |

**传回了什么数据给前端**

成功时不返回 `data`，只返回“退出成功”。

**接口从调用到传回数据中间调用了什么操作**

控制器把请求体绑定为 `WsLogoutRequest` 后，调用 `chat.ClientLogout`。该方法会根据当前消息模式找到内存中的在线客户端实例，如果客户端存在，则向聊天服务器发送登出事件、主动关闭 WebSocket 连接，并关闭客户端内部的消息通道 `SendTo` 与 `SendBack`。因此这个接口的作用不是改数据库，而是清理当前用户的在线连接状态。

## 9. 补充说明

### 9.1 代码中明显可见的调用链模式

当前项目的 HTTP 接口大体遵循以下固定链路：

1. `internal/https_server/https_server.go` 注册 Gin 路由。
2. `api/v1/*.go` 控制器负责参数绑定和统一错误返回。
3. `internal/service/gorm/*.go` 负责业务规则、缓存处理和数据读写。
4. `internal/dao/gorm.go` 提供全局 Gorm 数据库连接。
5. `api/v1/controller.go` 中的 `JsonBack` 统一把业务返回值包装为前端可消费的 JSON 结构。

### 9.2 文档适用范围

本文档描述的是当前仓库中的后端接口行为，完全以现有代码实现为准。如果后续你继续调整 DTO、service 逻辑、缓存策略或返回结构，这份文档也需要同步更新。
</div>
