package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"passwall/internal/model"
	"passwall/internal/service/proxy"
	"passwall/internal/service/task"
)

type QuickWakeProxyReq struct {
	Concurrent int      `json:"concurrent"`
	Type       []string `json:"type"`
}

func QuickWakeProxy(ctx context.Context, service proxy.QuickWakeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req QuickWakeProxyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "error",
				"status_code": http.StatusBadRequest,
				"status_msg":  "无效的请求参数: " + err.Error(),
			})
			return
		}

		types := make([]model.ProxyType, 0, len(req.Type))
		for _, proxyType := range req.Type {
			if proxyType == "" {
				continue
			}
			types = append(types, model.ProxyType(proxyType))
		}

		err := service.WakeBannedProxies(ctx, proxy.QuickWakeRequest{
			Types:      types,
			Concurrent: req.Concurrent,
		}, true)
		if err != nil {
			if task.IsConflictError(err) {
				c.JSON(http.StatusOK, gin.H{
					"result":      "error",
					"status_code": http.StatusOK,
					"status_msg":  "已有其他任务正在运行",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "error",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "快速唤醒失败: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"result":      "success",
			"status_code": http.StatusOK,
			"status_msg":  "任务已启动",
		})
	}
}
