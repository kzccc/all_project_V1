package respond

// GetCurContactListInChatRoomRespond 描述该接口返回给前端的数据结构。
type GetCurContactListInChatRoomRespond struct {
	// ContactId 保存响应中的 `contact_id` 字段。
	ContactId string `json:"contact_id"`
}
