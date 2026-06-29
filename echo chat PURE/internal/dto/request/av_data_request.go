package request

// AVData 描述该接口使用的请求参数结构。
type AVData struct {
	// MessageId 保存前端传入的 `messageId` 参数。
	MessageId string `json:"messageId"`
	// Type 保存前端传入的 `type` 参数。
	Type string `json:"type"`
}
