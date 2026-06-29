package https_server

import (
	"net/http"
	"net/http/pprof"

	"github.com/gin-gonic/gin"

	"echo_chat_server/internal/config"
	"echo_chat_server/internal/service/chat"
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
