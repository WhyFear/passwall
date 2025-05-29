package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"passwall/internal/repository"
)

// GetProxyHistoryRequest 获取代理历史请求
type GetProxyHistoryRequest struct {
	ProxyID   uint      `form:"proxy_id" binding:"required"`
	StartTime time.Time `form:"start_time"`
	EndTime   time.Time `form:"end_time"`
	Limit     int       `form:"limit" default:"10"`
}

// GetProxyHistory 获取代理测速历史记录
func GetProxyHistory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req GetProxyHistoryRequest

		// 从URL参数中获取代理ID
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

		// 从查询参数中获取其他可选参数
		limitStr := c.DefaultQuery("limit", "10")
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			limit = 10
		}
		req.Limit = limit

		// 检查是否提供了时间范围
		startTimeStr := c.Query("start_time")
		endTimeStr := c.Query("end_time")

		// 创建仓库
		proxyRepo := repository.NewProxyRepository(db)
		speedTestHistoryRepo := repository.NewSpeedTestHistoryRepository(db)

		// 检查代理是否存在
		proxy, err := proxyRepo.FindByID(req.ProxyID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"result":      "fail",
				"status_code": http.StatusNotFound,
				"status_msg":  "Proxy not found",
			})
			return
		}

		var histories []*struct {
			ID            uint      `json:"id"`
			Ping          int       `json:"ping"`
			DownloadSpeed int64     `json:"download_speed"`
			UploadSpeed   int64     `json:"upload_speed"`
			TestTime      time.Time `json:"test_time"`
		}

		// 根据参数获取历史记录
		if startTimeStr != "" && endTimeStr != "" {
			startTime, err := time.Parse(time.RFC3339, startTimeStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"result":      "fail",
					"status_code": http.StatusBadRequest,
					"status_msg":  "Invalid start time format",
				})
				return
			}

			endTime, err := time.Parse(time.RFC3339, endTimeStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"result":      "fail",
					"status_code": http.StatusBadRequest,
					"status_msg":  "Invalid end time format",
				})
				return
			}

			// 根据时间范围获取历史记录
			rawHistories, err := speedTestHistoryRepo.FindByTimeRange(req.ProxyID, startTime, endTime)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"result":      "fail",
					"status_code": http.StatusInternalServerError,
					"status_msg":  "Failed to fetch proxy history: " + err.Error(),
				})
				return
			}

			// 转换为简化格式
			for _, h := range rawHistories {
				histories = append(histories, &struct {
					ID            uint      `json:"id"`
					Ping          int       `json:"ping"`
					DownloadSpeed int64     `json:"download_speed"`
					UploadSpeed   int64     `json:"upload_speed"`
					TestTime      time.Time `json:"test_time"`
				}{
					ID:            h.ID,
					Ping:          h.Ping,
					DownloadSpeed: h.DownloadSpeed,
					UploadSpeed:   h.UploadSpeed,
					TestTime:      h.TestTime,
				})
			}
		} else {
			// 根据限制获取最近的历史记录
			rawHistories, err := speedTestHistoryRepo.FindByProxyID(req.ProxyID, req.Limit)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"result":      "fail",
					"status_code": http.StatusInternalServerError,
					"status_msg":  "Failed to fetch proxy history: " + err.Error(),
				})
				return
			}

			// 转换为简化格式
			for _, h := range rawHistories {
				histories = append(histories, &struct {
					ID            uint      `json:"id"`
					Ping          int       `json:"ping"`
					DownloadSpeed int64     `json:"download_speed"`
					UploadSpeed   int64     `json:"upload_speed"`
					TestTime      time.Time `json:"test_time"`
				}{
					ID:            h.ID,
					Ping:          h.Ping,
					DownloadSpeed: h.DownloadSpeed,
					UploadSpeed:   h.UploadSpeed,
					TestTime:      h.TestTime,
				})
			}
		}

		// 返回代理信息和历史记录
		c.JSON(http.StatusOK, gin.H{
			"result":      "success",
			"status_code": http.StatusOK,
			"data": gin.H{
				"proxy": gin.H{
					"id":             proxy.ID,
					"name":           proxy.Name,
					"type":           proxy.Type,
					"domain":         proxy.Domain,
					"port":           proxy.Port,
					"ping":           proxy.Ping,
					"download_speed": proxy.DownloadSpeed,
					"upload_speed":   proxy.UploadSpeed,
					"status":         proxy.Status,
				},
				"history": histories,
			},
		})
	}
}
