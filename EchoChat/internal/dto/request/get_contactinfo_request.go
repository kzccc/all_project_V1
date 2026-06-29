package request

// GetContactInfoRequest 描述该接口使用的请求参数结构。
type GetContactInfoRequest struct {
	// ContactId 保存前端传入的 `contact_id` 参数。
	ContactId string `json:"contact_id"`
}
