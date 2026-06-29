package request

// UpdateGroupInfoRequest 描述该接口使用的请求参数结构。
type UpdateGroupInfoRequest struct {
	// OwnerId 保存前端传入的 `owner_id` 参数。
	OwnerId string `json:"owner_id"`
	// Uuid 保存前端传入的 `uuid` 参数。
	Uuid string `json:"uuid"`
	// Name 保存前端传入的 `name` 参数。
	Name string `json:"name"`
	// Avatar 保存前端传入的 `avatar` 参数。
	Avatar string `json:"avatar"`
	// AddMode 保存前端传入的 `add_mode` 参数。
	AddMode int8 `json:"add_mode"`
	// Notice 保存前端传入的 `notice` 参数。
	Notice string `json:"notice"`
}
