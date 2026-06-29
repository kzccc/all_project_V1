package v1

// 本文件实现 controller 相关的 HTTP 控制器入口，负责参数绑定、调用 service 并返回统一响应。

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"echo_chat_server/pkg/constants"
	"echo_chat_server/pkg/zlog"
	"net/http"
	"reflect"
	"strings"
)

// JsonBack 把 service 层约定的 ret 码统一映射成前端消费的 JSON 结构。
func JsonBack(c *gin.Context, message string, ret int, data interface{}) {
	responseCode := 200
	if ret == 0 {
		// 成功时仅在 data 不为空时返回 data 字段，避免空对象污染响应。
		if data != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": message,
				"data":    data,
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": message,
			})
		}
	} else if ret == -2 {
		// -2 表示业务校验失败，由前端按普通错误提示处理即可。
		responseCode = 400
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": message,
		})
	} else if ret == -1 {
		// -1 表示系统级错误，通常需要结合服务端日志进一步排查。
		responseCode = 500
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": message,
		})
	}
	zlog.Info(
		"http.response",
		zap.String("event", "http.response"),
		zap.String("module", "api"),
		zap.String("request_id", RequestID(c)),
		zap.String("actor_id", ActorID(c)),
		zap.String("path", c.Request.URL.Path),
		zap.Int("code", responseCode),
		zap.Int("ret", ret),
	)
}

func RequestID(c *gin.Context) string {
	if requestID, ok := c.Get(constants.REQUEST_ID_CONTEXT_KEY); ok {
		if value, ok := requestID.(string); ok {
			return value
		}
	}
	return ""
}

func RequestFields(c *gin.Context, fields ...zap.Field) []zap.Field {
	base := []zap.Field{
		zap.String("request_id", RequestID(c)),
		zap.String("actor_id", ActorID(c)),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method),
	}
	return append(base, fields...)
}

func ActorID(c *gin.Context) string {
	if actorID, ok := c.Get(constants.ACTOR_ID_CONTEXT_KEY); ok {
		if value, ok := actorID.(string); ok {
			return value
		}
	}
	return ""
}

func setActorID(c *gin.Context, actorID string) {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return
	}
	if existing := ActorID(c); existing != "" {
		return
	}
	c.Set(constants.ACTOR_ID_CONTEXT_KEY, actorID)
}

func findActorID(req any) string {
	value := reflect.ValueOf(req)
	if !value.IsValid() {
		return ""
	}
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return ""
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return ""
	}
	actorFieldPriority := []string{
		"OwnerId", "OwnerID",
		"SendId", "SendID",
		"UserId", "UserID",
		"Uuid", "UUID",
		"ClientId", "ClientID",
		"Telephone",
		"UserOneId", "UserOneID",
	}
	for _, name := range actorFieldPriority {
		field := value.FieldByName(name)
		if field.IsValid() && field.Kind() == reflect.String {
			actorID := strings.TrimSpace(field.String())
			if actorID != "" {
				return actorID
			}
		}
	}

	actorTagPriority := map[string]struct{}{
		"owner_id": {}, "ownerId": {},
		"send_id": {}, "sendId": {},
		"user_id": {}, "userId": {},
		"uuid": {}, "client_id": {}, "clientId": {},
		"telephone": {}, "user_one_id": {}, "userOneId": {},
	}
	typeOfReq := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		if field.Kind() != reflect.String {
			continue
		}
		jsonTag := strings.Split(typeOfReq.Field(i).Tag.Get("json"), ",")[0]
		if _, ok := actorTagPriority[jsonTag]; !ok {
			continue
		}
		actorID := strings.TrimSpace(field.String())
		if actorID != "" {
			return actorID
		}
	}
	return ""
}

func BindJSONOrError(c *gin.Context, req any, api string) bool {
	if err := c.BindJSON(req); err != nil {
		fields := RequestFields(
			c,
			zap.String("event", "http.request.bind_failed"),
			zap.String("module", api),
			zap.String("error", err.Error()),
		)
		zlog.Error("http.request.bind_failed", fields...)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return false
	}
	setActorID(c, findActorID(req))
	return true
}
