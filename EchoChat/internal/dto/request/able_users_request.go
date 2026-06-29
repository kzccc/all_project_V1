package request

// AbleUsersRequest 描述该接口使用的请求参数结构。
type AbleUsersRequest struct {
	// UuidList 保存前端传入的 `uuid_list` 参数。
	UuidList []string `json:"uuid_list"`
	// IsAdmin 保存前端传入的 `is_admin` 参数。
	IsAdmin int8 `json:"is_admin"`
}
