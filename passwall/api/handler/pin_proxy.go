package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"passwall/internal/service"
)

type PinProxyReq struct {
	ID     uint `json:"id" binding:"required"`
	Pinned bool `json:"pinned"`
}

func PinProxy(service service.ProxyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req PinProxyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "Invalid request parameters",
			})
			return
		}
		if err := service.PinProxy(req.ID, req.Pinned); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "Failed to pin proxy",
			})
		}
		c.JSON(http.StatusOK, gin.H{
			"result":      "success",
			"status_code": http.StatusOK,
			"status_msg":  "操作成功",
		})
	}
}
