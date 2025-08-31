package handler

import (
	"net/http"
	"passwall/config"
	"passwall/internal/service"

	"github.com/gin-gonic/gin"
)

// IPDetectRequest 检测IP质量请求
type IPDetectRequest struct {
	ProxyID uint `json:"proxy_id" form:"proxy_id" binding:"required"`
}

// BatchDetectIPQualityRequest 批量检测IP质量请求
type BatchDetectIPQualityRequest struct {
	IPs                    []string `json:"ips" binding:"required,min=1,max=100"`
	EnableGeolocation      *bool    `json:"enable_geolocation,omitempty"`
	EnableRiskAssessment   *bool    `json:"enable_risk_assessment,omitempty"`
	EnableServiceDetection *bool    `json:"enable_service_detection,omitempty"`
	Timeout                *int     `json:"timeout,omitempty"`
	MaxConcurrency         *int     `json:"max_concurrency,omitempty"`
}

// DetectIPQuality 检测IP质量
func DetectIPQuality(config config.CheckConfig, ipDetectorService service.IPDetectorService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req IPDetectRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "请求参数无效",
			})
			return
		}

		// 执行IP质量检测
		go func() {
			_ = ipDetectorService.Detect(&service.IPDetectorReq{
				ProxyID:         req.ProxyID,
				Enabled:         config.Enable,
				IPInfoEnable:    config.IPInfo.Enable,
				APPUnlockEnable: config.AppUnlock.Enable,
				Refresh:         true,
			})
		}()

		c.JSON(http.StatusOK, gin.H{
			"result":      "success",
			"status_code": http.StatusOK,
			"status_msg":  "IP Check Started",
		})
	}
}

// GetIPQuality 获取IP质量信息
func GetIPQuality(ipQualityService service.IPDetectorService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req IPDetectRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "请求参数无效",
			})
			return
		}
		resp, err := ipQualityService.GetInfo(&service.IPDetectorReq{
			ProxyID: req.ProxyID,
		})
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "get ip info failed",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"result":      "success",
			"status_code": http.StatusOK,
			"status_msg":  "get ip info success",
			"data":        resp,
		})
	}
}
