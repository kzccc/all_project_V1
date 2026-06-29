package request

// AddGroupListRequest 描述该接口使用的请求参数结构。
type AddGroupListRequest struct {
	// GroupId 保存前端传入的 `group_id` 参数。
	GroupId string `json:"group_id"`
}
