package v1

// SwaggerResponse 通用响应结构
type SwaggerResponse struct {
	Code    int         `json:"code" example:"200"`
	Message string      `json:"message" example:"ok"`
	Data    interface{} `json:"data,omitempty"`
}
