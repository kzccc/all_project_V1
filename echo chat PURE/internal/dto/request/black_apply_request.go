package request

// BlackApplyRequest 描述该接口使用的请求参数结构。
type BlackApplyRequest struct {
	// OwnerId 保存前端传入的 `owner_id` 参数。
	OwnerId string `json:"owner_id"`
	// ContactId 保存前端传入的 `contact_id` 参数。
	ContactId string `json:"contact_id"`
}
