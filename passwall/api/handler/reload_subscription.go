package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/metacubex/mihomo/log"

	"passwall/internal/service"
	"passwall/internal/service/proxy"
	"passwall/internal/util"
)

// ReloadSubscriptionRequest 重新加载订阅请求
type ReloadSubscriptionRequest struct {
	ID uint `json:"id" form:"id"` // 订阅ID，为0表示重新加载所有订阅
}

// ReloadSubscription 重新加载订阅处理器
func ReloadSubscription(ctx context.Context, subscriptionManager proxy.SubscriptionManager, configService service.ConfigService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ReloadSubscriptionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "error",
				"status_code": http.StatusBadRequest,
				"status_msg":  "无效的请求参数: " + err.Error(),
			})
			return
		}

		// 获取当前配置以检查是否使用代理
		cfg, err := configService.GetConfig()
		if err != nil {
			log.Errorln("获取配置失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "error",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "内部系统错误",
			})
			return
		}
		var downloadOptions *util.DownloadOptions
		if cfg.Proxy.Enabled && cfg.Proxy.URL != "" {
			downloadOptions = &util.DownloadOptions{
				ProxyURL: cfg.Proxy.URL,
			}
		}

		if req.ID > 0 {
			// 刷新单个订阅
			err = subscriptionManager.RefreshSubscriptionAsync(ctx, req.ID, downloadOptions)
		} else {
			// 刷新所有订阅
			err = subscriptionManager.RefreshAllSubscriptions(ctx, true, downloadOptions)
		}

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "error",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "刷新订阅失败: " + err.Error(),
			})
			return
		}

		// 成功完成
		c.JSON(http.StatusOK, gin.H{
			"result":      "success",
			"status_code": http.StatusOK,
			"status_msg":  "刷新订阅任务已启动",
		})
	}
}
