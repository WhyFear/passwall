package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"passwall/internal/service/proxy"
	"passwall/internal/service/task"
)

type TestProxyReq struct {
	ID        int64  `json:"id"`
	Status    string `json:"status"`
	Type      string `json:"type"`
	Country   string `json:"country_code"`
	Risk      string `json:"risk_level"`
	AppUnlock string `json:"app_unlock"`
}

func TestProxy(ctx context.Context, proxyTester proxy.Tester) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TestProxyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "error",
				"status_code": http.StatusBadRequest,
				"status_msg":  "无效的请求参数: " + err.Error(),
			})
			return
		}

		filter, err := parseNodeFilter(req.Status, req.Type, req.Country, req.Risk, req.AppUnlock)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "error",
				"status_code": http.StatusBadRequest,
				"status_msg":  "无效的筛选参数: " + err.Error(),
			})
			return
		}

		// 创建测试请求
		request := &proxy.TestRequest{
			Filters:    filter,
			Concurrent: ctx.Value("concurrent").(int),
		}
		if req.ID > 0 {
			request.ProxyIDs = []int64{req.ID}
		}

		// 测试代理
		err = proxyTester.TestProxies(ctx, request, true)
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
				"status_code": http.StatusBadRequest,
				"status_msg":  "测试代理失败: " + err.Error(),
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
