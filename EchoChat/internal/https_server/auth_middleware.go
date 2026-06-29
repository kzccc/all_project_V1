package https_server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"echo_chat_server/internal/auth"
	"echo_chat_server/internal/dao"
	"echo_chat_server/internal/model"
	"echo_chat_server/internal/observability"
	"echo_chat_server/pkg/constants"
	"echo_chat_server/pkg/enum/user_info/user_status_enum"
	"echo_chat_server/pkg/zlog"
)

func abortAuth(c *gin.Context, code int, message string, logEvent string) {
	observability.ObserveAuthReject(c.Request.URL.Path, logEvent)
	zlog.Error(
		logEvent,
		zap.String("event", logEvent),
		zap.String("module", "http.auth"),
		zap.String("request_id", c.GetString(constants.REQUEST_ID_CONTEXT_KEY)),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method),
		zap.String("client_ip", c.ClientIP()),
	)
	c.JSON(http.StatusOK, gin.H{
		"code":    code,
		"message": message,
	})
	c.Abort()
}

func tokenFromRequest(c *gin.Context) string {
	authHeader := strings.TrimSpace(c.GetHeader(constants.AUTHORIZATION_HEADER))
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}
	return strings.TrimSpace(c.Query("token"))
}

// AuthRequired 校验请求是否携带了有效的 access token，并把当前用户信息注入上下文。
//
// 该中间件主要负责三件事：
// 1. 从请求头或 query 参数中提取 token
// 2. 校验 token 是否是合法的 access token
// 3. 根据 token 中的用户标识加载当前用户，并写入 gin.Context
//
// 校验通过后，后续控制器和中间件可以直接从上下文中读取当前登录用户和 token claims。
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 先从请求中提取 token。
		// 普通 HTTP 请求通常从 Authorization: Bearer <token> 中获取，
		// WebSocket 握手这类场景则可能从 query 参数中获取。
		token := tokenFromRequest(c)
		if token == "" {
			abortAuth(c, 401, "登录已失效，请重新登录", "http.auth.token_missing")
			return
		}

		// 将 token 按 access token 的规则进行解析和校验。
		// 这里不仅会校验 JWT 签名、过期时间、session_id 等内容，
		// 还会额外确认这个 token 的类型确实是 access。
		claims, err := auth.ParseTokenAs(token, auth.AccessTokenType)
		if err != nil {
			abortAuth(c, 401, "登录已失效，请重新登录", "http.auth.token_invalid")
			return
		}

		// 根据 token 中的用户 UUID 查询数据库，确认当前用户仍然存在。
		// 即使 token 本身合法，如果对应用户已经被删除或数据异常，
		// 当前请求也不能继续向下执行业务逻辑。
		var user model.UserInfo
		if err := dao.GormDB.Where("uuid = ?", claims.UserUUID).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				abortAuth(c, 401, "登录已失效，请重新登录", "http.auth.user_not_found")
				return
			}
			abortAuth(c, 500, constants.SYSTEM_ERROR, "http.auth.user_lookup_failed")
			return
		}

		// 将当前用户、当前 token claims 和 actor_id 写入上下文，
		// 供后续中间件和控制器复用，避免重复解析 token 或重复查库。
		c.Set(constants.CURRENT_USER_CONTEXT_KEY, &user)
		c.Set(constants.CURRENT_TOKEN_CLAIMS_CONTEXT_KEY, claims)
		c.Set(constants.ACTOR_ID_CONTEXT_KEY, user.Uuid)

		// 所有认证步骤都通过后，继续执行后续处理链路。
		c.Next()
	}
}

// BenchmarkAuthRequired 仅针对压测专用握手链路校验 access token，不回源数据库查询用户。
// 这条链路的目标是把“在线连接能力”从“登录/查库能力”里拆出来。
func BenchmarkAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := tokenFromRequest(c)
		if token == "" {
			abortAuth(c, 401, "登录已失效，请重新登录", "http.auth.token_missing")
			return
		}

		claims, err := auth.ParseTokenAs(token, auth.AccessTokenType)
		if err != nil {
			abortAuth(c, 401, "登录已失效，请重新登录", "http.auth.token_invalid")
			return
		}

		user := &model.UserInfo{
			Uuid:      claims.UserUUID,
			Telephone: claims.Telephone,
			IsAdmin:   claims.IsAdmin,
			Status:    user_status_enum.NORMAL,
		}
		c.Set(constants.CURRENT_USER_CONTEXT_KEY, user)
		c.Set(constants.CURRENT_TOKEN_CLAIMS_CONTEXT_KEY, claims)
		c.Set(constants.ACTOR_ID_CONTEXT_KEY, claims.UserUUID)
		c.Next()
	}
}

func RequireActiveUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUser, ok := c.Get(constants.CURRENT_USER_CONTEXT_KEY)
		if !ok {
			abortAuth(c, 401, "登录已失效，请重新登录", "http.auth.user_missing")
			return
		}
		user, ok := currentUser.(*model.UserInfo)
		if !ok || user == nil {
			abortAuth(c, 401, "登录已失效，请重新登录", "http.auth.user_invalid")
			return
		}
		if user.Status != user_status_enum.NORMAL {
			abortAuth(c, 403, "账号已被禁用，请联系管理员", "http.auth.user_disabled")
			return
		}
		c.Next()
	}
}

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUser, ok := c.Get(constants.CURRENT_USER_CONTEXT_KEY)
		if !ok {
			abortAuth(c, 401, "登录已失效，请重新登录", "http.auth.user_missing")
			return
		}
		user, ok := currentUser.(*model.UserInfo)
		if !ok || user == nil {
			abortAuth(c, 401, "登录已失效，请重新登录", "http.auth.user_invalid")
			return
		}
		if user.Status != user_status_enum.NORMAL {
			abortAuth(c, 403, "账号已被禁用，请联系管理员", "http.auth.user_disabled")
			return
		}
		if user.IsAdmin != 1 {
			abortAuth(c, 403, "无权访问该接口", "http.auth.admin_required")
			return
		}
		c.Next()
	}
}
