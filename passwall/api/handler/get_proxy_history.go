package handler

import (
	"net/http"
	"passwall/internal/repository"
	"passwall/internal/service"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// GetProxyHistoryRequest 获取代理历史请求
type GetProxyHistoryRequest struct {
	ProxyID  uint `form:"proxy_id" json:"id" binding:"required"`
	Page     int  `form:"page" json:"page"`
	PageSize int  `form:"pageSize" json:"pageSize"`
}

type GetProxyHistoryResponse struct {
	ID            uint      `json:"id"`
	Ping          int       `json:"ping"`
	DownloadSpeed int       `json:"download_speed"`
	UploadSpeed   int       `json:"upload_speed"`
	TestedAt      time.Time `json:"tested_at"`
}

// GetProxyHistory 获取代理测速历史记录
func GetProxyHistory(speedTestHistoryService service.SpeedTestHistoryService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req GetProxyHistoryRequest

		proxyIDStr := c.Param("id")
		proxyID, err := strconv.ParseUint(proxyIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "Invalid proxy ID",
			})
			return
		}
		req.ProxyID = uint(proxyID)

		// 绑定查询参数到请求结构体
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "Invalid request parameters",
			})
			return
		}

		// 创建分页参数
		page := repository.PageQuery{
			Page:     req.Page,
			PageSize: req.PageSize,
		}

		// 根据参数获取历史记录
		histories, err := speedTestHistoryService.GetSpeedTestHistoryByProxyID(req.ProxyID, &page)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "Failed to get proxy history",
			})
			return
		}
		result := make([]GetProxyHistoryResponse, 0, len(histories))
		for _, history := range histories {
			result = append(result, GetProxyHistoryResponse{
				ID:            history.ID,
				Ping:          history.Ping,
				DownloadSpeed: history.DownloadSpeed,
				UploadSpeed:   history.UploadSpeed,
				TestedAt:      history.TestTime,
			})
		}

		// 直接返回历史记录数组
		c.JSON(http.StatusOK, result)
	}
}
