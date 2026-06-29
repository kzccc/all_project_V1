package gorm

// 本文件实现 chatroom service 相关的业务服务，负责组织数据库、缓存和业务规则。

import "echo_chat_server/internal/dto/respond"

type chatRoomService struct {
}

var ChatRoomService = new(chatRoomService)

type chatRoomKey struct {
	ownerId   string
	contactId string
}

// map 类型是 {string, string}: []string该怎么写
var chatRooms = make(map[chatRoomKey][]string)

// GetCurContactListInChatRoom 获取当前聊天室联系人列表
func (c *chatRoomService) GetCurContactListInChatRoom(ownerId string, contactId string) (string, []respond.GetCurContactListInChatRoomRespond, int) {
	var rspList []respond.GetCurContactListInChatRoomRespond
	for _, contactId := range chatRooms[chatRoomKey{ownerId, contactId}] {
		rspList = append(rspList, respond.GetCurContactListInChatRoomRespond{
			ContactId: contactId,
		})
	}
	return "获取聊天室联系人列表成功", rspList, 0
}
