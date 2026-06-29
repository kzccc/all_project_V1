package request

// RegisterRequest 描述该接口使用的请求参数结构。
type RegisterRequest struct {
	// Telephone 保存前端传入的 `telephone` 参数。
	Telephone string `json:"telephone"`
	// Password 保存前端传入的 `password` 参数。
	Password string `json:"password"`
	// Nickname 保存前端传入的 `nickname` 参数。
	Nickname string `json:"nickname"`
	// SmsCode 保存前端传入的 `sms_code` 参数。
	SmsCode string `json:"sms_code"`
}
