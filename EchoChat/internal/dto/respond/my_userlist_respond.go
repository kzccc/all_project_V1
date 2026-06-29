package respond

// MyUserListRespond 描述该接口返回给前端的数据结构。
type MyUserListRespond struct {
	// UserId 保存响应中的 `user_id` 字段。
	UserId string `json:"user_id"`
	// UserName 保存响应中的 `user_name` 字段。
	UserName string `json:"user_name"`
	// Avatar 保存响应中的 `avatar` 字段。
	Avatar string `json:"avatar"`
}
