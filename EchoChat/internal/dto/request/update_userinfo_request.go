package request

// UpdateUserInfoRequest 描述该接口使用的请求参数结构。
type UpdateUserInfoRequest struct {
	// Uuid 保存前端传入的 `uuid` 参数。
	Uuid string `json:"uuid"`
	// Email 保存前端传入的 `email` 参数。
	Email string `json:"email"`
	// Nickname 保存前端传入的 `nickname` 参数。
	Nickname string `json:"nickname"`
	// Birthday 保存前端传入的 `birthday` 参数。
	Birthday string `json:"birthday"`
	// Signature 保存前端传入的 `signature` 参数。
	Signature string `json:"signature"`
	// Avatar 保存前端传入的 `avatar` 参数。
	Avatar string `json:"avatar"`
}
