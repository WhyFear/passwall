package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger 日志中间件
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 开始时间
		start := time.Now()

		// 处理请求
		c.Next()

		// 结束时间
		end := time.Now()

		// 执行时间
		latency := end.Sub(start)

		// 请求方法
		method := c.Request.Method

		// 请求路由
		uri := c.Request.RequestURI

		// 状态码
		statusCode := c.Writer.Status()

		// 客户端IP
		clientIP := c.ClientIP()

		// 日志格式
		log.Printf("[GIN] %v | %3d | %5v | %15s | %s | %s",
			end.Format("2006/01/02-15:04:05"),
			statusCode,
			latency,
			clientIP,
			method,
			uri,
		)
	}
}
