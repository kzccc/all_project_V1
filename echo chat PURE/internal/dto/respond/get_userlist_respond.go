package respond

// GetUserListRespond 描述该接口返回给前端的数据结构。
type GetUserListRespond struct {
	// Uuid 保存响应中的 `uuid` 字段。
	Uuid string `json:"uuid"`
	// Nickname 保存响应中的 `nickname` 字段。
	Nickname string `json:"nickname"`
	// Telephone 保存响应中的 `telephone` 字段。
	Telephone string `json:"telephone"`
	// Status 保存响应中的 `status` 字段。
	Status int8 `json:"status"`
	// IsAdmin 保存响应中的 `is_admin` 字段。
	IsAdmin int8 `json:"is_admin"`
	// IsDeleted 保存响应中的 `is_deleted` 字段。
	IsDeleted bool `json:"is_deleted"`
}
