package respond

// RegisterRespond 描述该接口返回给前端的数据结构。
type RegisterRespond struct {
	// AccessToken 保存响应中的 `access_token` 字段。
	AccessToken string `json:"access_token"`
	// RefreshToken 保存响应中的 `refresh_token` 字段。
	RefreshToken string `json:"refresh_token"`
	// Uuid 保存响应中的 `uuid` 字段。
	Uuid string `json:"uuid"`
	// Nickname 保存响应中的 `nickname` 字段。
	Nickname string `json:"nickname"`
	// Telephone 保存响应中的 `telephone` 字段。
	Telephone string `json:"telephone"`
	// Avatar 保存响应中的 `avatar` 字段。
	Avatar string `json:"avatar"`
	// Email 保存响应中的 `email` 字段。
	Email string `json:"email"`
	// Gender 保存响应中的 `gender` 字段。
	Gender int8 `json:"gender"`
	// Birthday 保存响应中的 `birthday` 字段。
	Birthday string `json:"birthday"`
	// Signature 保存响应中的 `signature` 字段。
	Signature string `json:"signature"`
	// CreatedAt 保存响应中的 `created_at` 字段。
	CreatedAt string `json:"created_at"`
	// IsAdmin 保存响应中的 `is_admin` 字段。
	IsAdmin int8 `json:"is_admin"`
	// Status 保存响应中的 `status` 字段。
	Status int8 `json:"status"`
}
