package respond

// NewContactListRespond 描述该接口返回给前端的数据结构。
type NewContactListRespond struct {
	// ContactId 保存响应中的 `contact_id` 字段。
	ContactId string `json:"contact_id"`
	// ContactName 保存响应中的 `contact_name` 字段。
	ContactName string `json:"contact_name"`
	// ContactAvatar 保存响应中的 `contact_avatar` 字段。
	ContactAvatar string `json:"contact_avatar"`
	// Message 保存响应中的 `message` 字段。
	Message string `json:"message"`
}
