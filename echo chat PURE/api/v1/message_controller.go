package v1

// 本文件实现 message controller 相关的 HTTP 控制器入口，负责参数绑定、调用 service 并返回统一响应。

import (
	"github.com/gin-gonic/gin"
	"echo_chat_server/internal/dto/request"
	"echo_chat_server/internal/service/gorm"
)

// GetMessageList 获取聊天记录
func GetMessageList(c *gin.Context) {
	var req request.GetMessageListRequest
	if !BindJSONOrError(c, &req, "api.message.list") {
		return
	}
	otherUserID, ok := ensureMessageParticipant(c, req.UserOneId, req.UserTwoId, "api.message.list")
	if !ok {
		return
	}
	message, rsp, ret := gorm.MessageService.GetMessageList(currentActorID(c), otherUserID)
	JsonBack(c, message, ret, rsp)
}

// GetGroupMessageList 获取群聊消息记录
func GetGroupMessageList(c *gin.Context) {
	var req request.GetGroupMessageListRequest
	if !BindJSONOrError(c, &req, "api.message.group_list") {
		return
	}
	if !ensureGroupMember(c, req.GroupId, "api.message.group_list") {
		return
	}
	message, rsp, ret := gorm.MessageService.GetGroupMessageList(req.GroupId)
	JsonBack(c, message, ret, rsp)
}

// UploadAvatar 上传头像
func UploadAvatar(c *gin.Context) {
	message, ret := gorm.MessageService.UploadAvatar(c)
	JsonBack(c, message, ret, nil)
}

// UploadFile 上传头像
func UploadFile(c *gin.Context) {
	message, ret := gorm.MessageService.UploadFile(c)
	JsonBack(c, message, ret, nil)
}
