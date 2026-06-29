package respond

// GroupSessionListRespond 描述该接口返回给前端的数据结构。
type GroupSessionListRespond struct {
	// SessionId 保存响应中的 `session_id` 字段。
	SessionId string `json:"session_id"`
	// GroupName 保存响应中的 `group_name` 字段。
	GroupName string `json:"group_name"`
	// GroupId 保存响应中的 `group_id` 字段。
	GroupId string `json:"group_id"`
	// Avatar 保存响应中的 `avatar` 字段。
	Avatar string `json:"avatar"`
}
