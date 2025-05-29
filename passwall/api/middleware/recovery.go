package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery 恢复中间件
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 打印错误堆栈信息
				log.Printf("panic: %v\n", err)
				debug.PrintStack()

				// 返回500响应
				c.JSON(http.StatusInternalServerError, gin.H{
					"result":      "fail",
					"status_code": http.StatusInternalServerError,
					"status_msg":  "Internal Server Error",
				})
				c.Abort()
			}
		}()

		c.Next()
	}
}
