# JWT 双 Token 改造计划

## 目标

把当前 EchoChat 的单 JWT 鉴权改造成经典双 token 方案：

- `access_token`：
  - 用于访问受保护的 HTTP / WebSocket 资源
  - 生命周期短
  - 不落库
- `refresh_token`：
  - 仅用于换取新的 token 对
  - 生命周期长
  - 服务端落 Redis，支持主动失效和轮换

## 现状评估

当前项目已经完成了基础 JWT 鉴权，但仍是单 token 模式：

1. 登录、注册、短信登录只返回一个 `token`
2. 后端中间件把这个 token 同时用于所有访问场景
3. 前端把单 token 放在 `sessionStorage`
4. token 失效后只能强制重新登录
5. 服务端无法精细撤销某次登录会话

这套实现已经解决了“接口不能裸奔”的问题，但对长期登录体验和会话治理还不够。

## 目标方案

### Token 职责拆分

#### Access Token

- 用途：访问业务接口、建立 WebSocket 连接
- 有效期：建议 15 分钟
- 内容：
  - 用户基础身份
  - `token_type=access`
  - `session_id`
- 存储位置：
  - 前端 `sessionStorage`
  - 不进入 Redis

#### Refresh Token

- 用途：只用于刷新 token
- 有效期：建议 7 天
- 内容：
  - 用户基础身份
  - `token_type=refresh`
  - `session_id`
- 存储位置：
  - 前端 `sessionStorage`
  - Redis 按 `session_id` 保存当前有效 refresh token

## 服务端改造计划

### 1. 配置扩展

扩展 `jwtConfig`：

- `accessExpireMinutes`
- `refreshExpireHours`
- `issuer`
- `subject`
- `key`

同时更新 `config.toml` 和 `config_local.toml`。

### 2. JWT 模型重构

重构 `internal/auth/jwt.go`：

- 增加统一 claims
- 增加字段：
  - `token_type`
  - `session_id`
- 提供：
  - 生成 access token
  - 生成 refresh token
  - 解析 token
  - 校验 token 类型
  - 生成 token 对

### 3. Refresh Token 持久化

基于 Redis 维护 refresh 会话：

- key 设计：
  - `refresh_token:{session_id}`
- value：
  - 当前有效 refresh token 字符串
- TTL：
  - 与 refresh token 过期时间一致

这样可以做到：

- 服务端校验 refresh token 是否仍然有效
- 用户登出时删除 Redis key
- refresh 轮换时替换 Redis value

### 4. 新增接口

新增公开接口：

- `POST /auth/refresh`

请求体：

- `refresh_token`

响应体：

- `access_token`
- `refresh_token`

行为规则：

1. 校验 refresh token 签名、过期时间、类型
2. 校验 Redis 中该 `session_id` 的 refresh token 与当前传入值一致
3. 成功后签发新的 access/refresh token
4. 用新的 refresh token 覆盖 Redis

### 5. 登录链路改造

修改：

- 登录
- 注册
- 短信登录

统一返回：

- `access_token`
- `refresh_token`
- 用户信息

登录成功后把 refresh token 写入 Redis。

### 6. 中间件改造

`AuthRequired()` 只接受 access token：

- header `Authorization: Bearer <access_token>`
- WebSocket `?token=<access_token>`

如果传入 refresh token，当作非法访问处理。

### 7. 登出改造

登出时：

1. 从当前 access token 中取出 `session_id`
2. 删除 Redis 中对应 refresh token
3. 关闭当前 WebSocket 连接

这样该次登录会话会被整体撤销。

## 前端改造计划

### 1. 会话存储拆分

把当前单 `token` 改成：

- `accessToken`
- `refreshToken`
- `userInfo`

### 2. 登录态接入

登录/注册/短信登录后保存 token 对。

### 3. Axios 自动续签

响应拦截器遇到 `401` 时：

1. 如果当前请求不是 refresh 请求，则尝试调用 `/auth/refresh`
2. refresh 成功：
   - 更新本地 token 对
   - 重放原请求
3. refresh 失败：
   - 清空登录态
   - 跳回登录页

### 4. WebSocket 续连

WebSocket 继续使用 access token 建连：

- 新建连接时使用最新 access token
- refresh 成功后，如果需要重建连接，则使用新 access token 重连

### 5. 上传接口

上传组件继续补 `Authorization` 头，但从新的 `accessToken` 读取。

## 测试计划

### 后端验证

1. 登录返回双 token
2. refresh 接口可正常换新 token 对
3. access token 访问接口成功
4. refresh token 访问业务接口失败
5. 已轮换的旧 refresh token 不能再次使用
6. 登出后 refresh token 失效
7. WebSocket 使用 access token 可连接

### 前端验证

1. 登录后页面正常进入系统
2. access token 失效后能自动 refresh
3. refresh 失效后自动回到登录页
4. 上传接口仍可正常工作
5. WebSocket 在刷新后的新 access token 下仍能正常连接

## 执行顺序

1. 扩展配置与 JWT 基础能力
2. 新增 refresh token Redis 管理
3. 改登录/注册/短信登录返回结构
4. 新增 `/auth/refresh`
5. 改中间件为 access token-only
6. 改登出逻辑
7. 改前端存储与自动 refresh
8. 跑 Go 测试、前端构建和联调验证

## 风险与注意事项

1. 当前仓库工作区本身已有较多未提交改动，改造时只能沿现有状态继续推进，不能粗暴回滚
2. 前端拦截器要避免 refresh 死循环
3. WebSocket 连接依赖 access token，token 更新后要确保重连逻辑一致
4. Redis 中 refresh token 的 key 设计要稳定，避免误删其他业务缓存
