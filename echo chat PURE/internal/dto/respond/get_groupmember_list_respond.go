package respond

// GetGroupMemberListRespond 描述该接口返回给前端的数据结构。
type GetGroupMemberListRespond struct {
	// UserId 保存响应中的 `user_id` 字段。
	UserId string `json:"user_id"`
	// Nickname 保存响应中的 `nickname` 字段。
	Nickname string `json:"nickname"`
	// Avatar 保存响应中的 `avatar` 字段。
	Avatar string `json:"avatar"`
}
