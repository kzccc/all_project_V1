# EchoChat

EchoChat 是一个前后端分离的即时通讯项目，包含 Go 后端、Vue 3 前端，以及围绕 Kafka、Redis、MySQL、WebSocket、可观测性和压测的一整套工程化实践。

## 功能概览

- 单聊、群聊、联系人和会话管理
- WebSocket 实时消息收发
- Kafka 消息链路与多实例扩展
- 后台管理能力
- 文件与静态资源支持
- Prometheus / Grafana / pprof 可观测性
- 单聊压测与性能调优文档

## 技术栈

- 后端：Go、Gin、GORM、WebSocket、Redis、Kafka、MySQL
- 前端：Vue 3、Vue Router、Vuex、Element Plus
- 运维与调优：Docker Compose、Prometheus、Grafana、k6、压测脚本

## 目录结构

```text
.
├── api/                 # HTTP / WebSocket 控制器
├── cmd/                 # 后端服务与辅助命令入口
├── configs/             # 服务配置与调优配置
├── deploy/              # Docker Compose、Nginx、观测部署文件
├── internal/            # 核心业务、鉴权、服务、观测逻辑
├── pkg/                 # 通用组件与工具
├── pressure testing/    # 压测配置与脚本
├── web/chat-server/     # Vue 前端
└── docs/ / doc/         # 设计、调优、压测与实施文档
```

## 快速开始

### 1. 准备依赖

- Go 1.23+
- Node.js 18+
- MySQL
- Redis
- Kafka（如果你启用 `kafka` 消息模式）

### 2. 配置后端

编辑 [configs/config.toml](configs/config.toml)，至少补齐这些字段：

- `mysqlConfig`
- `redisConfig`
- `logConfig`
- `jwtConfig`
- `staticSrcConfig`

如果只是本地功能联调，可以先使用：

```toml
[kafkaConfig]
messageMode = "channel"
```

### 3. 启动后端

```bash
go run ./cmd/echo_chat_server
```

默认监听 `0.0.0.0:8000`。

### 4. 启动前端

```bash
cd web/chat-server
npm install
npm run serve
```

前端开发服务默认走 HTTPS `443` 端口，证书读取逻辑在 [web/chat-server/vue.config.js](web/chat-server/vue.config.js)。

## 可观测性与压测

- 可观测性部署文件在 [deploy/observability](deploy/observability)
- 压测入口脚本在 [pressure testing/scripts](pressure%20testing/scripts)
- 相关分析文档在 [docs](docs) 和 [doc](doc)

## 开发建议

- 先用 `channel` 模式跑通业务，再切到 Kafka 模式排查链路问题
- 不要把 `runtime/`、压测记录、日志和本地证书提交进 git
- 变更接口后同步检查前端请求、鉴权中间件和压测脚本
