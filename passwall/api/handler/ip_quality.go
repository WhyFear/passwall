package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"passwall/internal/service"

	"github.com/gin-gonic/gin"
)

// DetectIPQualityRequest 检测IP质量请求
type DetectIPQualityRequest struct {
	IP                     string `json:"ip" binding:"required"`
	EnableGeolocation      *bool  `json:"enable_geolocation,omitempty"`
	EnableRiskAssessment   *bool  `json:"enable_risk_assessment,omitempty"`
	EnableServiceDetection *bool  `json:"enable_service_detection,omitempty"`
	Timeout                *int   `json:"timeout,omitempty"` // 超时时间（秒）
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

// BatchDetectResponse 批量检测响应
type BatchDetectResponse struct {
	TotalCount   int                        `json:"total_count"`
	SuccessCount int                        `json:"success_count"`
	FailureCount int                        `json:"failure_count"`
	Results      []*service.IPQualityResult `json:"results"`
	Failures     []BatchDetectFailure       `json:"failures,omitempty"`
	Duration     time.Duration              `json:"duration"`
}

// BatchDetectFailure 批量检测失败项
type BatchDetectFailure struct {
	IP    string `json:"ip"`
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// DetectIPQuality 检测IP质量
func DetectIPQuality(ipQualityService *service.IPQualityService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req DetectIPQualityRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "请求参数无效",
			})
			return
		}

		// 创建上下文
		ctx := c.Request.Context()
		if req.Timeout != nil && *req.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, time.Duration(*req.Timeout)*time.Second)
			defer cancel()
		}

		// 执行IP质量检测
		result, err := ipQualityService.DetectIPQuality(ctx, req.IP)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "IP质量检测失败",
			})
			return
		}

		c.JSON(http.StatusOK, result)
	}
}

// GetIPQuality 获取IP质量信息
func GetIPQuality(ipQualityService *service.IPQualityService) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.Param("ip")
		if ip == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "缺少IP地址参数",
			})
			return
		}

		// 解析查询参数
		timeout := 60 // 默认60秒超时
		if timeoutStr := c.Query("timeout"); timeoutStr != "" {
			if t, err := strconv.Atoi(timeoutStr); err == nil && t > 0 {
				timeout = t
			}
		}

		// 创建上下文
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(timeout)*time.Second)
		defer cancel()

		// 执行IP质量检测
		result, err := ipQualityService.DetectIPQuality(ctx, ip)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "IP质量检测失败",
			})
			return
		}

		c.JSON(http.StatusOK, result)
	}
}
