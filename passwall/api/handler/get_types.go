package handler

import (
	"net/http"
	"passwall/internal/service/proxy"

	"github.com/gin-gonic/gin"
)

func GetTypes(proxyService proxy.ProxyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		types, err := proxyService.GetTypes()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "Failed to get proxy types",
			})
			return
		}
		c.JSON(http.StatusOK, types)
	}
}
