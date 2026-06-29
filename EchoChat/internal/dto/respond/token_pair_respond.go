package respond

// TokenPairRespond 描述 token 刷新接口返回的数据结构。
type TokenPairRespond struct {
	// AccessToken 保存响应中的 `access_token` 字段。
	AccessToken string `json:"access_token"`
	// RefreshToken 保存响应中的 `refresh_token` 字段。
	RefreshToken string `json:"refresh_token"`
}
