package request

// LeaveGroupRequest 描述该接口使用的请求参数结构。
type LeaveGroupRequest struct {
	// UserId 保存前端传入的 `user_id` 参数。
	UserId string `json:"user_id"`
	// GroupId 保存前端传入的 `group_id` 参数。
	GroupId string `json:"group_id"`
}
