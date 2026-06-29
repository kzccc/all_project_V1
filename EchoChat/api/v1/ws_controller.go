package v1

// 本文件实现 ws controller 相关的 HTTP 控制器入口，负责参数绑定、调用 service 并返回统一响应。

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"echo_chat_server/internal/auth"
	"echo_chat_server/internal/dto/request"
	"echo_chat_server/internal/pressure"
	"echo_chat_server/internal/service/chat"
	"echo_chat_server/pkg/constants"
	"echo_chat_server/pkg/zlog"
	"net/http"
)

// WsLogin wss登录 Get
func WsLogin(c *gin.Context) {
	clientId := currentActorID(c)
	if clientId == "" {
		zlog.Error("ws.login.invalid_client_id", RequestFields(c, zap.String("event", "ws.login.invalid_client_id"))...)
		c.JSON(http.StatusOK, gin.H{
			"code":    401,
			"message": "登录已失效，请重新登录",
		})
		return
	}
	c.Set(constants.ACTOR_ID_CONTEXT_KEY, clientId)
	if pressure.ShouldLogHotPath(pressure.IsBenchmarkPath(c.Request.URL.Path)) {
		zlog.Info("ws.login.request", RequestFields(c, zap.String("event", "ws.login.request"), zap.String("client_id", clientId))...)
	}
	chat.NewClientInit(c, clientId)
}

// WsLogout wss登出
func WsLogout(c *gin.Context) {
	var req request.WsLogoutRequest
	if !BindJSONOrError(c, &req, "api.ws.logout") {
		return
	}
	if claims := CurrentClaims(c); claims != nil {
		if err := auth.DeleteRefreshToken(claims.SessionID); err != nil {
			zlog.Error("ws.logout.refresh_token_delete_failed", RequestFields(c, zap.String("event", "ws.logout.refresh_token_delete_failed"), zap.String("session_id", claims.SessionID), zap.String("error", err.Error()))...)
		}
	}
	message, ret := chat.ClientLogout(currentActorID(c))
	JsonBack(c, message, ret, nil)
}
