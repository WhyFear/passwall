package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
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

func AuthReq(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从req的body里面获取token
		authHeader := c.Request.URL.Query().Get("token")

		if authHeader != "" && authHeader == token {
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
