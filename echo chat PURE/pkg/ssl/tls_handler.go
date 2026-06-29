package ssl

// 本文件封装 HTTPS 相关的辅助中间件。

import (
	"github.com/gin-gonic/gin"
	"github.com/unrolled/secure"
	"echo_chat_server/pkg/zlog"
	"strconv"
)

// TlsHandler 返回一个 Gin 中间件，用于把明文请求重定向到 HTTPS 地址。
func TlsHandler(host string, port int) gin.HandlerFunc {
	return func(c *gin.Context) {
		secureMiddleware := secure.New(secure.Options{
			SSLRedirect: true,
			SSLHost:     host + ":" + strconv.Itoa(port),
		})
		err := secureMiddleware.Process(c.Writer, c.Request)

		// 一旦 secure 中间件处理失败，当前请求就不再继续往后执行。
		if err != nil {
			zlog.Fatal(err.Error())
			return
		}

		c.Next()
	}
}
