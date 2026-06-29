package request

// OpenSessionRequest 描述该接口使用的请求参数结构。
type OpenSessionRequest struct {
	// SendId 保存前端传入的 `send_id` 参数。
	SendId string `json:"send_id"`
	// ReceiveId 保存前端传入的 `receive_id` 参数。
	ReceiveId string `json:"receive_id"`
}
