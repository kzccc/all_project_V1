package request

// DismissGroupRequest 描述该接口使用的请求参数结构。
type DismissGroupRequest struct {
	// OwnerId 保存前端传入的 `owner_id` 参数。
	OwnerId string `json:"owner_id"`
	// GroupId 保存前端传入的 `group_id` 参数。
	GroupId string `json:"group_id"`
}
