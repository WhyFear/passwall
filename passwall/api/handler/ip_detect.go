package handler

import (
	"net/http"
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
func DetectIPQuality(configService service.ConfigService, ipDetectorService service.IPDetectorService) gin.HandlerFunc {
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

			cfg, err := configService.GetConfig()
			if err != nil {
				log.Errorln("get config failed: %v", err)
				return
			}
			ipCheckConfig := cfg.IPCheck

			_ = ipDetectorService.Detect(&service.IPDetectorReq{
				ProxyID:         req.ProxyID,
				Enabled:         ipCheckConfig.Enable,
				IPInfoEnable:    ipCheckConfig.IPInfo.Enable,
				APPUnlockEnable: ipCheckConfig.AppUnlock.Enable,
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
func BatchDetectIPQuality(configService service.ConfigService, ipDetectorService service.IPDetectorService) gin.HandlerFunc {
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

			cfg, err := configService.GetConfig()
			if err != nil {
				log.Errorln("get config failed: %v", err)
				return
			}
			ipCheckConfig := cfg.IPCheck

			_ = ipDetectorService.BatchDetect(&service.BatchIPDetectorReq{
				ProxyIDList:     req.ProxyIDList,
				Enabled:         ipCheckConfig.Enable,
				IPInfoEnable:    ipCheckConfig.IPInfo.Enable,
				APPUnlockEnable: ipCheckConfig.AppUnlock.Enable,
				Refresh:         true,
				Concurrent:      ipCheckConfig.Concurrent,
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

func GetCountryCodeList(ipDetectorService service.IPDetectorService) gin.HandlerFunc {
	return func(c *gin.Context) {
		countryCodes, err := ipDetectorService.GetDistinctCountryCode()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "get country code failed",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"result":      "success",
			"status_code": http.StatusOK,
			"status_msg":  "success",
			"data":        countryCodes,
		})
	}
}
