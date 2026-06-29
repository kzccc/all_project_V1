package request

// SmsLoginRequest 描述该接口使用的请求参数结构。
type SmsLoginRequest struct {
	// Telephone 保存前端传入的 `telephone` 参数。
	Telephone string `json:"telephone"`
	// SmsCode 保存前端传入的 `sms_code` 参数。
	SmsCode string `json:"sms_code"`
}
