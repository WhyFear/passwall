package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"passwall/internal/repository"
)

func GetTypes(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		proxyRepo := repository.NewProxyRepository(db)
		var types []string
		err := proxyRepo.GetTypes(&types)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, types)
	}
}
