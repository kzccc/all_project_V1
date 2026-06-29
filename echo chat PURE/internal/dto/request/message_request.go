package request

// MessageRequest 描述该接口使用的请求参数结构。
type MessageRequest struct {
	// Type 保存前端传入的 `type` 参数。
	Type int `json:"type"`
	// Content 保存前端传入的 `content` 参数。
	Content string `json:"content"`
	// Url 保存前端传入的 `url` 参数。
	Url string `json:"url"`
	// SendId 保存前端传入的 `send_id` 参数。
	SendId string `json:"send_id"`
	// ReceiveId 保存前端传入的 `receive_id` 参数。
	ReceiveId string `json:"receive_id"`
	// FileType 保存前端传入的 `file_type` 参数。
	FileType string `json:"file_type"`
	// FileName 保存前端传入的 `file_name` 参数。
	FileName string `json:"file_name"`
	// FileSize 保存前端传入的 `file_size` 参数。
	FileSize int `json:"file_size"`
}
