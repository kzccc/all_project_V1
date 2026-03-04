package v1

import (
	"github.com/gin-gonic/gin"
	"kama_chat_server/internal/dto/request"
	"kama_chat_server/internal/service/chat"
	"kama_chat_server/pkg/constants"
	"kama_chat_server/pkg/zlog"
	"net/http"
)

// WsLogin wss登录 Get
// @Summary WebSocket 登录
// @Tags ws
// @Produce json
// @Param client_id query string true "客户端ID"
// @Success 200 {object} SwaggerResponse
// @Router /wss [get]
func WsLogin(c *gin.Context) {
	clientId := c.Query("client_id")
	if clientId == "" {
		zlog.Error("clientId获取失败")
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "clientId获取失败",
		})
		return
	}
	chat.NewClientInit(c, clientId)
}

// WsLogout wss登出
// @Summary WebSocket 登出
// @Tags ws
// @Accept json
// @Produce json
// @Param body body request.WsLogoutRequest true "登出请求"
// @Success 200 {object} SwaggerResponse
// @Router /user/wsLogout [post]
func WsLogout(c *gin.Context) {
	var req request.WsLogoutRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, ret := chat.ClientLogout(req.OwnerId)
	JsonBack(c, message, ret, nil)
}
