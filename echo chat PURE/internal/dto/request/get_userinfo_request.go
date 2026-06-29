package request

// GetUserInfoRequest 描述该接口使用的请求参数结构。
type GetUserInfoRequest struct {
	// Uuid 保存前端传入的 `uuid` 参数。
	Uuid string `json:"uuid"`
}
