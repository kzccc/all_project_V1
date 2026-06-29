package respond

// GetGroupListRespond 描述该接口返回给前端的数据结构。
type GetGroupListRespond struct {
	// Uuid 保存响应中的 `uuid` 字段。
	Uuid string `json:"uuid"`
	// Name 保存响应中的 `name` 字段。
	Name string `json:"name"`
	// OwnerId 保存响应中的 `owner_id` 字段。
	OwnerId string `json:"owner_id"`
	// Status 保存响应中的 `status` 字段。
	Status int8 `json:"status"`
	// IsDeleted 保存响应中的 `is_deleted` 字段。
	IsDeleted bool `json:"is_deleted"`
}
