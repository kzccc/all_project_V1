package respond

// GetGroupMessageListRespond 描述该接口返回给前端的数据结构。
type GetGroupMessageListRespond struct {
	// MessageId 保存响应中的 `message_id` 字段。
	MessageId string `json:"message_id"`
	// SendId 保存响应中的 `send_id` 字段。
	SendId string `json:"send_id"`
	// SendName 保存响应中的 `send_name` 字段。
	SendName string `json:"send_name"`
	// SendAvatar 保存响应中的 `send_avatar` 字段。
	SendAvatar string `json:"send_avatar"`
	// ReceiveId 保存响应中的 `receive_id` 字段。
	ReceiveId string `json:"receive_id"`
	// Type 保存响应中的 `type` 字段。
	Type int8 `json:"type"`
	// Content 保存响应中的 `content` 字段。
	Content string `json:"content"`
	// Url 保存响应中的 `url` 字段。
	Url string `json:"url"`
	// FileType 保存响应中的 `file_type` 字段。
	FileType string `json:"file_type"`
	// FileName 保存响应中的 `file_name` 字段。
	FileName string `json:"file_name"`
	// FileSize 保存响应中的 `file_size` 字段。
	FileSize string `json:"file_size"`
	// SessionSeq 保存响应中的 `session_seq` 字段。
	SessionSeq int64 `json:"session_seq"`
	// CreatedAt 保存响应中的 `created_at` 字段。
	CreatedAt string `json:"created_at"`
}
