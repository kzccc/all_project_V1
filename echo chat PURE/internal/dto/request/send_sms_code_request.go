package request

// SendSmsCodeRequest 描述该接口使用的请求参数结构。
type SendSmsCodeRequest struct {
	// Telephone 保存前端传入的 `telephone` 参数。
	Telephone string `json:"telephone"`
}
