package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// Auth 认证中间件
func Auth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从header里面获取token，格式为：Authorization: Bearer token
		authHeader := c.Request.Header.Get("Authorization")

		if authHeader != "" && authHeader == "Bearer "+token {
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{
				"result":      "fail",
				"status_code": http.StatusUnauthorized,
				"status_msg":  "Unauthorized",
			})
			c.Abort()
		}
	}
}
