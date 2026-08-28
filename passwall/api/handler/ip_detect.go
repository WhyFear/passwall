package handler

import (
	"context"
	"net/http"
	"passwall/internal/detector/unlockchecker"
	"passwall/internal/model"
	"passwall/internal/service"
	"passwall/internal/service/task"
	"strings"

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

type DetectMissingIPRequest struct {
	Type []string `json:"type"`
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

		cfg, err := configService.GetConfig()
		if err != nil {
			log.Errorln("get config failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"result": "fail", "status_code": http.StatusInternalServerError, "status_msg": "获取配置失败",
			})
			return
		}
		ipCheckConfig := cfg.IPCheck
		err = ipDetectorService.BatchDetect(context.Background(), &service.BatchIPDetectorReq{
			ProxyIDList:     []uint{req.ProxyID},
			Enabled:         ipCheckConfig.Enable,
			IPInfoEnable:    ipCheckConfig.IPInfo.Enable,
			APPUnlockEnable: ipCheckConfig.AppUnlock.Enable,
			Refresh:         true,
			Concurrent:      1,
			TaskResourceID:  req.ProxyID,
			Async:           true,
		})
		if err != nil {
			status := http.StatusInternalServerError
			msg := "IP 检测启动失败"
			if task.IsConflictError(err) {
				status = http.StatusConflict
				msg = "已有冲突任务正在运行"
			}
			c.JSON(status, gin.H{"result": "fail", "status_code": status, "status_msg": msg})
			return
		}

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

		cfg, err := configService.GetConfig()
		if err != nil {
			log.Errorln("get config failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"result": "fail", "status_code": http.StatusInternalServerError, "status_msg": "获取配置失败",
			})
			return
		}
		ipCheckConfig := cfg.IPCheck
		err = ipDetectorService.BatchDetect(context.Background(), &service.BatchIPDetectorReq{
			ProxyIDList:     req.ProxyIDList,
			Enabled:         ipCheckConfig.Enable,
			IPInfoEnable:    ipCheckConfig.IPInfo.Enable,
			APPUnlockEnable: ipCheckConfig.AppUnlock.Enable,
			Refresh:         true,
			Concurrent:      ipCheckConfig.Concurrent,
			Async:           true,
		})
		if err != nil {
			status := http.StatusInternalServerError
			msg := "IP 检测启动失败"
			if task.IsConflictError(err) {
				status = http.StatusConflict
				msg = "已有冲突任务正在运行"
			}
			c.JSON(status, gin.H{"result": "fail", "status_code": status, "status_msg": msg})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"result":      "success",
			"status_code": http.StatusOK,
			"status_msg":  "IP IPCheck Started",
		})
	}
}

func DetectMissingIPQuality(ctx context.Context, ipDetectorService service.IPDetectorService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req DetectMissingIPRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "error",
				"status_code": http.StatusBadRequest,
				"status_msg":  "无效的请求参数: " + err.Error(),
			})
			return
		}

		types := make([]model.ProxyType, 0, len(req.Type))
		seen := make(map[string]bool, len(req.Type))
		for _, proxyType := range req.Type {
			proxyType = strings.TrimSpace(proxyType)
			if proxyType == "" || seen[proxyType] {
				continue
			}
			seen[proxyType] = true
			types = append(types, model.ProxyType(proxyType))
		}

		total, err := ipDetectorService.DetectMissing(ctx, types, true)
		if err != nil {
			if task.IsConflictError(err) {
				c.JSON(http.StatusConflict, gin.H{
					"result":      "error",
					"status_code": http.StatusConflict,
					"status_msg":  "已有其他任务正在运行",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "error",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "补全检测信息失败: " + err.Error(),
			})
			return
		}

		statusMsg := "任务已启动"
		if total == 0 {
			statusMsg = "没有需要补全的节点"
		}
		c.JSON(http.StatusOK, gin.H{
			"result":      "success",
			"status_code": http.StatusOK,
			"status_msg":  statusMsg,
			"total":       total,
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

func GetUnlockAppList() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"result":      "success",
			"status_code": http.StatusOK,
			"status_msg":  "success",
			"data":        unlockchecker.SupportedApplications(),
		})
	}
}
