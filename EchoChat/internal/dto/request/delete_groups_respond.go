package request

// DeleteGroupsRequest 描述该接口使用的请求参数结构。
type DeleteGroupsRequest struct {
	// UuidList 保存前端传入的 `uuid_list` 参数。
	UuidList []string `json:"uuid_list"`
}
