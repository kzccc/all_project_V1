package request

// RefreshTokenRequest 描述刷新 access token 时使用的请求参数结构。
type RefreshTokenRequest struct {
	// RefreshToken 保存前端传入的 `refresh_token` 参数。
	RefreshToken string `json:"refresh_token"`
}
