package handler

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/metacubex/mihomo/log"
	"net/http"
	"passwall/internal/service/proxy"
)

type BanProxyReq struct {
	ID                     uint    `json:"id"`
	SuccessRateThreshold   float32 `json:"success_rate_threshold"`
	DownloadSpeedThreshold int     `json:"download_speed_threshold"`
	UploadSpeedThreshold   int     `json:"upload_speed_threshold"`
	PingThreshold          int     `json:"ping_threshold"`
	TestTimes              int     `json:"test_times"`
}

func BanProxy(ctx context.Context, service proxy.ProxyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req BanProxyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "Invalid request parameters",
			})
			return
		}
		serviceReq := proxy.BanProxyReq{
			ID:                     req.ID,
			SuccessRateThreshold:   req.SuccessRateThreshold,
			DownloadSpeedThreshold: req.DownloadSpeedThreshold,
			UploadSpeedThreshold:   req.UploadSpeedThreshold,
			PingThreshold:          req.PingThreshold,
			TestTimes:              req.TestTimes,
		}
		go func() {
			if err := service.BanProxy(ctx, serviceReq); err != nil {
				log.Errorln("处理代理封禁请求失败：%v", err)
			}
		}()

		c.JSON(http.StatusOK, gin.H{
			"result":      "success",
			"status_code": http.StatusOK,
			"status_msg":  "操作成功",
		})
	}
}
