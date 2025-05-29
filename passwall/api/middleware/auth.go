package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Auth 认证中间件
func Auth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取请求中的token
		requestToken := c.Query("token")
		if requestToken == "" {
			requestToken = c.PostForm("token")
		}

		// 验证token
		if requestToken == "" || requestToken != token {
			c.JSON(http.StatusUnauthorized, gin.H{
				"result":      "fail",
				"status_code": http.StatusUnauthorized,
				"status_msg":  "Unauthorized",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
