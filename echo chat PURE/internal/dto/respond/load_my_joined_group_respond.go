package respond

// LoadMyJoinedGroupRespond 描述该接口返回给前端的数据结构。
type LoadMyJoinedGroupRespond struct {
	// GroupId 保存响应中的 `group_id` 字段。
	GroupId string `json:"group_id"`
	// GroupName 保存响应中的 `group_name` 字段。
	GroupName string `json:"group_name"`
	// Avatar 保存响应中的 `avatar` 字段。
	Avatar string `json:"avatar"`
}
