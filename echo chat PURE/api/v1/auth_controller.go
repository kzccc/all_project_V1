package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"echo_chat_server/internal/dto/request"
	"echo_chat_server/internal/service/gorm"
)

// RefreshToken 使用 refresh token 换取新的 token 对。
func RefreshToken(c *gin.Context) {
	var req request.RefreshTokenRequest
	if !BindJSONOrError(c, &req, "api.auth.refresh") {
		return
	}
	message, data, _, statusCode := gorm.UserInfoService.RefreshToken(req.RefreshToken)
	c.JSON(http.StatusOK, gin.H{
		"code":    statusCode,
		"message": message,
		"data":    data,
	})
}
