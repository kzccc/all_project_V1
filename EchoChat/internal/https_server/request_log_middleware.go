package https_server

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"echo_chat_server/internal/pressure"
	"echo_chat_server/pkg/constants"
	"echo_chat_server/pkg/util/random"
	"echo_chat_server/pkg/zlog"
)

func newRequestID() string {
	return "REQ" + random.GetNowAndLenRandomString(6)
}

func actorIDFromRequest(c *gin.Context) string {
	headerActorID := strings.TrimSpace(c.GetHeader(constants.ACTOR_ID_HEADER))
	if headerActorID != "" {
		return headerActorID
	}
	candidates := []string{"client_id", "owner_id", "user_id", "uuid", "telephone"}
	for _, key := range candidates {
		value := strings.TrimSpace(c.Query(key))
		if value != "" {
			return value
		}
	}
	return ""
}

// RequestLogMiddleware 负责给每个请求注入 request_id，并输出统一的开始/结束日志。
func RequestLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(constants.REQUEST_ID_HEADER))
		if requestID == "" {
			requestID = newRequestID()
		}
		c.Set(constants.REQUEST_ID_CONTEXT_KEY, requestID)
		c.Writer.Header().Set(constants.REQUEST_ID_HEADER, requestID)
		if actorID := actorIDFromRequest(c); actorID != "" {
			c.Set(constants.ACTOR_ID_CONTEXT_KEY, actorID)
		}
		if pressure.ShouldSkipRequestLog(c.Request.URL.Path) {
			c.Next()
			return
		}

		start := time.Now()
		startFields := []zap.Field{
			zap.String("event", "http.request.start"),
			zap.String("module", "http"),
			zap.String("request_id", requestID),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
		}
		if actorID, ok := c.Get(constants.ACTOR_ID_CONTEXT_KEY); ok {
			if value, ok := actorID.(string); ok && strings.TrimSpace(value) != "" {
				startFields = append(startFields, zap.String("actor_id", value))
			}
		}
		zlog.Info(
			"http.request.start",
			startFields...,
		)

		c.Next()

		latency := time.Since(start)
		errorText := c.Errors.ByType(gin.ErrorTypePrivate).String()
		if errorText == "" {
			errorText = c.Errors.String()
		}
		fields := []zap.Field{
			zap.String("event", "http.request.finish"),
			zap.String("module", "http"),
			zap.String("request_id", requestID),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status_code", c.Writer.Status()),
			zap.Int64("latency_ms", latency.Milliseconds()),
		}
		if actorID, ok := c.Get(constants.ACTOR_ID_CONTEXT_KEY); ok {
			if value, ok := actorID.(string); ok && strings.TrimSpace(value) != "" {
				fields = append(fields, zap.String("actor_id", value))
			}
		}
		if errorText != "" {
			fields = append(fields, zap.String("error", errorText))
		}
		zlog.Info("http.request.finish", fields...)
	}
}
