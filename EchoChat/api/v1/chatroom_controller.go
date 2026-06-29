package v1

// 本文件实现 chatroom controller 相关的 HTTP 控制器入口，负责参数绑定、调用 service 并返回统一响应。

import (
	"github.com/gin-gonic/gin"
	"echo_chat_server/internal/dto/request"
	"echo_chat_server/internal/service/gorm"
)

// GetCurContactListInChatRoom 获取当前聊天室联系人列表
func GetCurContactListInChatRoom(c *gin.Context) {
	var req request.GetCurContactListInChatRoomRequest
	if !BindJSONOrError(c, &req, "api.chatroom.current_contact_list") {
		return
	}
	message, rspList, ret := gorm.ChatRoomService.GetCurContactListInChatRoom(currentActorID(c), req.ContactId)
	JsonBack(c, message, ret, rspList)
}
