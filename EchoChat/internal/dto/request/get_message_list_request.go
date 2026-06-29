package request

// GetMessageListRequest 描述该接口使用的请求参数结构。
type GetMessageListRequest struct {
	// UserOneId 保存前端传入的 `user_one_id` 参数。
	UserOneId string `json:"user_one_id"`
	// UserTwoId 保存前端传入的 `user_two_id` 参数。
	UserTwoId string `json:"user_two_id"`
}
