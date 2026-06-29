package request

// WsLogoutRequest 描述该接口使用的请求参数结构。
type WsLogoutRequest struct {
	// OwnerId 保存前端传入的 `owner_id` 参数。
	OwnerId string `json:"owner_id"`
}
