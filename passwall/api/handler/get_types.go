package handler

import (
	"net/http"
	"passwall/internal/service"

	"github.com/gin-gonic/gin"
)

func GetTypes(proxyService service.ProxyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		types, err := proxyService.GetTypes()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, types)
	}
}
