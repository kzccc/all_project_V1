package request

// LoginRequest 描述该接口使用的请求参数结构。
type LoginRequest struct {
	// Telephone 保存前端传入的 `telephone` 参数。
	Telephone string `json:"telephone"`
	// Password 保存前端传入的 `password` 参数。
	Password string `json:"password"`
}
