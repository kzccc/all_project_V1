package https_server

import (
	"net/http"
	"net/http/pprof"
	"strings"

	"github.com/gin-gonic/gin"

	"echo_chat_server/internal/config"
	"echo_chat_server/internal/observability"
	"echo_chat_server/internal/pressure"
	"echo_chat_server/internal/service/chat"
	myredis "echo_chat_server/internal/service/redis"
)

func registerObservabilityRoutes(engine *gin.Engine) {
	conf := config.GetConfig()

	engine.GET("/readyz", func(c *gin.Context) {
		switch conf.KafkaConfig.MessageMode {
		case "kafka":
			if chat.KafkaChatServer.IsShuttingDown() {
				c.JSON(http.StatusServiceUnavailable, gin.H{"ready": false, "mode": "kafka", "reason": "shutting_down"})
				return
			}
			if !chat.KafkaChatServer.IsConsumerReady() {
				c.JSON(http.StatusServiceUnavailable, gin.H{"ready": false, "mode": "kafka", "reason": "consumer_not_ready"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"ready": true, "mode": "kafka"})
		default:
			if chat.ChatServer.IsShuttingDown() {
				c.JSON(http.StatusServiceUnavailable, gin.H{"ready": false, "mode": "channel", "reason": "shutting_down"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"ready": true, "mode": "channel"})
		}
	})

	if conf.ObservabilityConfig.EnableMetrics {
		engine.GET("/metrics", gin.WrapH(observability.MetricsHandler()))
	}
	if conf.PressureTestConfig.EnableBenchmarkRoutes {
		engine.POST("/bench/admin/reset", func(c *gin.Context) {
			pressure.ResetBenchmarkTrace()
			c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok"})
		})
		engine.POST("/bench/admin/session_seq/reset_state", func(c *gin.Context) {
			chat.ResetConversationSequenceBenchState()
			c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok"})
		})
		engine.POST("/bench/admin/session_seq/flush_state", func(c *gin.Context) {
			if err := chat.FlushConversationSequenceBenchState(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok"})
		})
		engine.POST("/bench/admin/session_seq/redis_key/delete", func(c *gin.Context) {
			type requestBody struct {
				ConversationKey string `json:"conversation_key"`
			}
			var req requestBody
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
				return
			}
			conversationKey := strings.TrimSpace(req.ConversationKey)
			if conversationKey == "" {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "conversation_key is required"})
				return
			}
			redisKey := "message_session_seq_" + conversationKey
			if err := myredis.DelKeyIfExists(redisKey); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": gin.H{"redis_key": redisKey}})
		})
		engine.GET("/bench/admin/session_seq/redis_key_status", func(c *gin.Context) {
			conversationKey := strings.TrimSpace(c.Query("conversation_key"))
			if conversationKey == "" {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "conversation_key is required"})
				return
			}
			redisKey := "message_session_seq_" + conversationKey
			exists, err := myredis.KeyExists(redisKey)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
				return
			}
			value, err := myredis.GetKey(redisKey)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"redis_key": redisKey, "exists": exists, "value": value}})
		})
		engine.GET("/bench/admin/trace", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"code": 200, "data": pressure.SnapshotBenchmarkTrace()})
		})
		engine.GET("/bench/admin/metrics_snapshot", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"code": 200, "data": pressure.SnapshotBenchmarkMetrics()})
		})
	}
	if !conf.ObservabilityConfig.EnablePprof {
		return
	}

	engine.GET("/debug/pprof/", gin.WrapF(pprof.Index))
	engine.GET("/debug/pprof/cmdline", gin.WrapF(pprof.Cmdline))
	engine.GET("/debug/pprof/profile", gin.WrapF(pprof.Profile))
	engine.GET("/debug/pprof/symbol", gin.WrapF(pprof.Symbol))
	engine.POST("/debug/pprof/symbol", gin.WrapF(pprof.Symbol))
	engine.GET("/debug/pprof/trace", gin.WrapF(pprof.Trace))
	engine.GET("/debug/pprof/allocs", gin.WrapH(pprof.Handler("allocs")))
	engine.GET("/debug/pprof/block", gin.WrapH(pprof.Handler("block")))
	engine.GET("/debug/pprof/goroutine", gin.WrapH(pprof.Handler("goroutine")))
	engine.GET("/debug/pprof/heap", gin.WrapH(pprof.Handler("heap")))
	engine.GET("/debug/pprof/mutex", gin.WrapH(pprof.Handler("mutex")))
	engine.GET("/debug/pprof/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
}
