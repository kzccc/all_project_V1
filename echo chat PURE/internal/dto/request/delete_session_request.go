package request

// DeleteSessionRequest 描述该接口使用的请求参数结构。
type DeleteSessionRequest struct {
	// OwnerId 保存前端传入的 `owner_id` 参数。
	OwnerId string `json:"owner_id"`
	// SessionId 保存前端传入的 `session_id` 参数。
	SessionId string `json:"session_id"`
}
