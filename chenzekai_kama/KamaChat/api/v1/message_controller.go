package v1

import (
	"github.com/gin-gonic/gin"
	"kama_chat_server/internal/dto/request"
	"kama_chat_server/internal/service/gorm"
	"kama_chat_server/pkg/constants"
	"net/http"
)

// GetMessageList 获取聊天记录
// @Summary 获取聊天记录
// @Tags message
// @Accept json
// @Produce json
// @Param body body request.GetMessageListRequest true "获取聊天记录请求"
// @Success 200 {object} SwaggerResponse
// @Router /message/getMessageList [post]
func GetMessageList(c *gin.Context) {
	var req request.GetMessageListRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, rsp, ret := gorm.MessageService.GetMessageList(req.UserOneId, req.UserTwoId)
	JsonBack(c, message, ret, rsp)
}

// GetGroupMessageList 获取群聊消息记录
// @Summary 获取群聊消息记录
// @Tags message
// @Accept json
// @Produce json
// @Param body body request.GetGroupMessageListRequest true "获取群聊消息记录请求"
// @Success 200 {object} SwaggerResponse
// @Router /message/getGroupMessageList [post]
func GetGroupMessageList(c *gin.Context) {
	var req request.GetGroupMessageListRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, rsp, ret := gorm.MessageService.GetGroupMessageList(req.GroupId)
	JsonBack(c, message, ret, rsp)
}

// UploadAvatar 上传头像
// @Summary 上传头像
// @Tags message
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "头像文件"
// @Success 200 {object} SwaggerResponse
// @Router /message/uploadAvatar [post]
func UploadAvatar(c *gin.Context) {
	message, ret := gorm.MessageService.UploadAvatar(c)
	JsonBack(c, message, ret, nil)
}

// UploadFile 上传头像
// @Summary 上传文件
// @Tags message
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "文件"
// @Success 200 {object} SwaggerResponse
// @Router /message/uploadFile [post]
func UploadFile(c *gin.Context) {
	message, ret := gorm.MessageService.UploadFile(c)
	JsonBack(c, message, ret, nil)
}
