# EchoChat 日志系统改造计划

## 1. 目标与范围

### 1.1 总体目标

把当前“分散、难串联、难检索”的日志体系改造成：

1. **可读**：同一类事件格式统一，关键字段固定。
2. **可检索**：支持按 `request_id` / `connection_id` / `session_id` / `message_uuid` 快速筛选。
3. **可追踪**：一次请求或一条消息能跨 HTTP -> WebSocket -> Kafka -> 落库链路定位。
4. **可运营**：日志滚动、输出位置稳定，避免无序堆叠。

### 1.2 本次改造范围

1. `pkg/zlog`：日志基础设施与输出规范。
2. `internal/https_server`：HTTP 请求级链路日志中间件。
3. `api/v1`：控制器层请求上下文日志（含 request_id）。
4. `internal/service/chat`：WebSocket / 消息分发关键日志。
5. `internal/service/kafka` 与 `internal/service/chat/kafka_server.go`：Kafka 生产消费关键日志。
6. `internal/service/gorm/message_service.go`：消息查询关键日志（排序依据与结果规模）。
7. 文档与测试：给出检索方式与验证步骤。

## 2. 问题清单（现状）

1. 缺少统一关联 ID：无法把同一次请求的日志串起来。
2. 结构化字段不足：大量仅记录 `err.Error()`，缺少业务关键上下文。
3. 输出风格混杂：`zlog`、`log.Println`、`fmt.Println` 并存。
4. 缺少请求边界日志：HTTP 请求没有统一开始/结束日志与耗时统计。
5. WebSocket 与 Kafka 链路无统一字段：排查跨链路问题成本高。

## 3. 设计原则

1. **先统一主键，再扩展细节**：优先落 `request_id` 与 `connection_id`。
2. **字段优先于文案**：日志检索依赖字段，不依赖自然语言。
3. **关键路径先覆盖**：先覆盖最常用于接口联调的路径。
4. **尽量少侵入业务逻辑**：优先通过中间件与辅助函数承载标准化。
5. **不做过度兜底**：在保证编译和行为正确的前提下保持简单。

## 4. 统一日志规范

### 4.1 统一字段模型

#### 基础字段（所有日志）

1. `event`：事件名（如 `http.request.start`）
2. `module`：模块名（如 `api.contact`）
3. `level`：日志级别（由 zap 自动）
4. `time`：时间戳（由 zap 自动）

#### 链路字段（按场景）

1. HTTP：`request_id` `method` `path` `status_code` `latency_ms` `client_ip`
2. WS：`connection_id` `client_id` `session_id` `message_type`
3. Kafka：`topic` `partition` `offset` `key`
4. 消息业务：`message_uuid` `session_id` `session_seq` `send_id` `receive_id`

### 4.2 事件命名规范

1. 请求入口：`http.request.start`
2. 请求出口：`http.request.finish`
3. WS 建连：`ws.connection.open`
4. WS 收包：`ws.message.in`
5. WS 发包：`ws.message.out`
6. Kafka 生产：`kafka.produce.chat`
7. Kafka 消费：`kafka.consume.chat`
8. 消息落库：`message.persist`
9. 关键异常：`*.error`

## 5. 分层改造方案

### 5.1 基础设施层（zlog）

1. 保持 JSON 输出，确保 stdout + file 双写。
2. 接入标准库日志重定向，减少 `log.Println` 风格分裂。
3. 增加通用字段辅助函数（如 request/connection 相关）。
4. 保留 caller 信息，确保定位到具体文件行号。

### 5.2 HTTP 接入层（Gin 中间件）

1. 新增 Request Logging Middleware。
2. 规则：
3. 优先读取请求头 `X-Request-ID`，无则生成。
4. 写回响应头 `X-Request-ID`，便于客户端与服务端对齐。
5. 请求开始与结束分别落日志（含耗时、状态码）。
6. 将 `request_id` 注入 `gin.Context`，供 controller 使用。

### 5.3 Controller 层

1. 在 `api/v1/controller.go` 提供统一字段提取辅助函数。
2. 控制器异常日志统一带 `request_id` + 业务关键参数。
3. 对高频调试接口优先覆盖：`contact` `session` `message` `ws`。

### 5.4 WebSocket 层

1. 连接建立时生成 `connection_id`（建议格式 `WS-时间+随机`）。
2. 在 `Client` 结构体保存 `ConnectionID`。
3. 收包/发包日志统一包含：
4. `connection_id` `client_id` `session_id` `message_type` `session_seq(若有)`

### 5.5 Kafka 层

1. 生产日志：记录 topic/key/value_size（不打印完整 payload）。
2. 消费日志：记录 topic/partition/offset/key 与消息核心字段。
3. 消费异常日志补齐 `topic` 与错误类型。

### 5.6 消息业务层

1. 落库成功日志记录：`message_uuid` `session_id` `session_seq`。
2. 查询日志记录：条件 + 返回条数 + 排序字段（`session_seq`）。
3. 避免重复打印大对象，改为关键字段摘要。

## 6. 执行步骤

### 阶段 A：文档与中间件

1. 新增并接入请求日志中间件。
2. 接入 `X-Request-ID` 注入与回传。

### 阶段 B：日志基础设施

1. 增强 `pkg/zlog` 的统一输出能力。
2. 重定向标准库日志，减少混杂。

### 阶段 C：业务链路落地

1. 改造控制器关键错误日志（带 request_id）。
2. 改造 WS 与 Kafka 主链路日志（带 connection/message 维度字段）。
3. 改造消息查询/落库关键日志。

### 阶段 D：验证与回归

1. 后端编译：`go build ./cmd/echo_chat_server`
2. 前端构建：`npm run build`（确保无联动破坏）
3. 联调验证：用 `curl` 携带 `X-Request-ID` 发请求并 `grep` 日志验证可追踪性。

## 7. 测试与验收标准

### 7.1 功能验收

1. 每个 HTTP 请求日志都有唯一 `request_id`。
2. 响应头包含 `X-Request-ID`。
3. 同一请求可通过 `request_id` 一次性筛出完整日志链。
4. WS 收发日志包含 `connection_id`。
5. Kafka 消费日志包含 `topic/partition/offset`。

### 7.2 检索验收

1. `grep '<request_id>' <logfile>` 能定位单次请求全链路。
2. `grep '<connection_id>' <logfile>` 能定位单连接消息活动。
3. `grep '<session_id>' <logfile>` 能定位会话相关关键日志。

### 7.3 稳定性验收

1. 代码可编译通过。
2. 不破坏现有接口行为。
3. 日志落盘路径可用，输出不中断。

## 8. 风险与约束

1. 全量替换所有历史 `fmt/log` 调用成本高，本次优先关键链路。
2. 过度打印 payload 会影响性能与敏感信息安全，默认只打摘要字段。
3. request_id 主要覆盖 HTTP 链路；WS 独立使用 connection_id。

## 9. 交付物

1. 本文档：`docs/plan.md`
2. 日志中间件与基础设施改造代码
3. 关键链路日志增强代码
4. 编译与构建验证结果
