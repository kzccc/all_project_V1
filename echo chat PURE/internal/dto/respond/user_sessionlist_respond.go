package respond

// UserSessionListRespond 描述该接口返回给前端的数据结构。
type UserSessionListRespond struct {
	// SessionId 保存响应中的 `session_id` 字段。
	SessionId string `json:"session_id"`
	// Avatar 保存响应中的 `avatar` 字段。
	Avatar string `json:"avatar"`
	// UserId 保存响应中的 `user_id` 字段。
	UserId string `json:"user_id"`
	// Username 保存响应中的 `user_name` 字段。
	Username string `json:"user_name"`
}
