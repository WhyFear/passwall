package handler

import (
	"net/http"
	"passwall/config"
	"passwall/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/metacubex/mihomo/log"
)

// IPDetectRequest 检测IP质量请求
type IPDetectRequest struct {
	ProxyID uint `json:"proxy_id" form:"proxy_id" binding:"required"`
}

// BatchIPDetectRequest 批量检测IP质量请求
type BatchIPDetectRequest struct {
	ProxyIDList []uint `json:"proxy_id_list" form:"proxy_id_list" binding:"required,min=1,max=1000"`
}

// DetectIPQuality 检测IP质量
func DetectIPQuality(config config.IPCheckConfig, ipDetectorService service.IPDetectorService) gin.HandlerFunc {
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
			defer func() {
				if err := recover(); err != nil {
					log.Errorln("batch detect proxy ip failed, proxy id: %v, err: %v", req.ProxyID, err)
				}
			}()
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
			"status_msg":  "IP IPCheck Started",
		})
	}
}

// BatchDetectIPQuality 检测IP质量
func BatchDetectIPQuality(config config.IPCheckConfig, ipDetectorService service.IPDetectorService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req BatchIPDetectRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "请求参数无效" + err.Error(),
			})
			return
		}

		// 执行IP质量检测
		go func() {
			defer func() {
				if err := recover(); err != nil {
					log.Errorln("batch detect proxy ip failed, proxy id list: %v, err: %v", req.ProxyIDList, err)
				}
			}()
			_ = ipDetectorService.BatchDetect(&service.BatchIPDetectorReq{
				ProxyIDList:     req.ProxyIDList,
				Enabled:         config.Enable,
				IPInfoEnable:    config.IPInfo.Enable,
				APPUnlockEnable: config.AppUnlock.Enable,
				Refresh:         true,
				Concurrent:      config.Concurrent,
			})
		}()

		c.JSON(http.StatusOK, gin.H{
			"result":      "success",
			"status_code": http.StatusOK,
			"status_msg":  "IP IPCheck Started",
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
