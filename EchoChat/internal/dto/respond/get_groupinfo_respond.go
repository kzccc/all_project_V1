package respond

// GetGroupInfoRespond 描述该接口返回给前端的数据结构。
type GetGroupInfoRespond struct {
	// Uuid 保存响应中的 `uuid` 字段。
	Uuid string `json:"uuid"`
	// Name 保存响应中的 `name` 字段。
	Name string `json:"name"`
	// Notice 保存响应中的 `notice` 字段。
	Notice string `json:"notice"`
	// MemberCnt 保存响应中的 `member_cnt` 字段。
	MemberCnt int `json:"member_cnt"`
	// OwnerId 保存响应中的 `owner_id` 字段。
	OwnerId string `json:"owner_id"`
	// AddMode 保存响应中的 `add_mode` 字段。
	AddMode int8 `json:"add_mode"`
	// Status 保存响应中的 `status` 字段。
	Status int8 `json:"status"`
	// Avatar 保存响应中的 `avatar` 字段。
	Avatar string `json:"avatar"`
	// IsDeleted 保存响应中的 `is_deleted` 字段。
	IsDeleted bool `json:"is_deleted"`
}
