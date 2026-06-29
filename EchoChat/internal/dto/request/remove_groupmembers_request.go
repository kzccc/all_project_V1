package request

// RemoveGroupMembersRequest 描述该接口使用的请求参数结构。
type RemoveGroupMembersRequest struct {
	// GroupId 保存前端传入的 `group_id` 参数。
	GroupId string `json:"group_id"`
	// OwnerId 保存前端传入的 `owner_id` 参数。
	OwnerId string `json:"owner_id"`
	// UuidList 保存前端传入的 `uuid_list` 参数。
	UuidList []string `json:"uuid_list"`
}
