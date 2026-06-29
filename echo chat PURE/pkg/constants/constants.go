package constants

// 本文件集中定义项目中复用的常量配置，避免业务代码硬编码。

const (
	// CHANNEL_SIZE 是聊天服务内部各类 channel 的默认缓冲长度。
	CHANNEL_SIZE = 100
	// SYSTEM_ERROR 是统一返回给前端的通用系统错误文案。
	SYSTEM_ERROR = "系统错误，请联系工作人员"
	// FILE_MAX_SIZE 是 multipart 表单解析时允许的最大文件体积。
	FILE_MAX_SIZE = 50000
	// REDIS_TIMEOUT 是当前项目写入 Redis 时常用的过期分钟数。
	REDIS_TIMEOUT = 1
	// REQUEST_ID_HEADER 是客户端可透传的请求链路 ID 头。
	REQUEST_ID_HEADER = "X-Request-ID"
	// ACTOR_ID_HEADER 是客户端可透传的请求发起者标识头。
	ACTOR_ID_HEADER = "X-Actor-ID"
	// AUTHORIZATION_HEADER 是标准 Bearer Token 头。
	AUTHORIZATION_HEADER = "Authorization"
	// REQUEST_ID_CONTEXT_KEY 是 Gin Context 中保存 request_id 的键名。
	REQUEST_ID_CONTEXT_KEY = "request_id"
	// ACTOR_ID_CONTEXT_KEY 是 Gin Context 中保存请求发起者标识的键名。
	ACTOR_ID_CONTEXT_KEY = "actor_id"
	// CURRENT_USER_CONTEXT_KEY 是 Gin Context 中保存当前登录用户的键名。
	CURRENT_USER_CONTEXT_KEY = "current_user"
	// CURRENT_TOKEN_CLAIMS_CONTEXT_KEY 是 Gin Context 中保存当前 token claims 的键名。
	CURRENT_TOKEN_CLAIMS_CONTEXT_KEY = "current_token_claims"
)
