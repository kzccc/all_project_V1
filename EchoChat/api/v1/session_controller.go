package v1

// 本文件实现 session controller 相关的 HTTP 控制器入口，负责参数绑定、调用 service 并返回统一响应。

import (
	"github.com/gin-gonic/gin"
	"echo_chat_server/internal/dto/request"
	"echo_chat_server/internal/service/gorm"
)

// OpenSession 打开会话
func OpenSession(c *gin.Context) {
	var openSessionReq request.OpenSessionRequest
	if !BindJSONOrError(c, &openSessionReq, "api.session.open") {
		return
	}
	openSessionReq.SendId = currentActorID(c)
	message, sessionId, ret := gorm.SessionService.OpenSession(openSessionReq)
	JsonBack(c, message, ret, sessionId)
}

// GetUserSessionList 获取用户会话列表
func GetUserSessionList(c *gin.Context) {
	var getUserSessionListReq request.OwnlistRequest
	if !BindJSONOrError(c, &getUserSessionListReq, "api.session.user_list") {
		return
	}
	message, sessionList, ret := gorm.SessionService.GetUserSessionList(currentActorID(c))
	JsonBack(c, message, ret, sessionList)
}

// GetGroupSessionList 获取群聊会话列表
func GetGroupSessionList(c *gin.Context) {
	var getGroupListReq request.OwnlistRequest
	if !BindJSONOrError(c, &getGroupListReq, "api.session.group_list") {
		return
	}
	message, groupList, ret := gorm.SessionService.GetGroupSessionList(currentActorID(c))
	JsonBack(c, message, ret, groupList)
}

// DeleteSession 删除会话
func DeleteSession(c *gin.Context) {
	var deleteSessionReq request.DeleteSessionRequest
	if !BindJSONOrError(c, &deleteSessionReq, "api.session.delete") {
		return
	}
	message, ret := gorm.SessionService.DeleteSession(currentActorID(c), deleteSessionReq.SessionId)
	JsonBack(c, message, ret, nil)
}

// CheckOpenSessionAllowed 检查是否可以打开会话
func CheckOpenSessionAllowed(c *gin.Context) {
	var req request.CreateSessionRequest
	if !BindJSONOrError(c, &req, "api.session.check_allowed") {
		return
	}
	req.SendId = currentActorID(c)
	message, res, ret := gorm.SessionService.CheckOpenSessionAllowed(req.SendId, req.ReceiveId)
	JsonBack(c, message, ret, res)
}
