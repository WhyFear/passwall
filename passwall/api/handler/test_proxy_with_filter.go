package handler

import (
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
	"passwall/internal/model"
	"passwall/internal/service/proxy"
	"strconv"
	"strings"
)

type TestProxyReq struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
	Type   string `json:"type"`
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

		filter := &proxy.ProxyFilter{}
		if len(req.Status) > 0 {
			statusList := strings.Split(req.Status, ",")
			var statuses []model.ProxyStatus
			for _, status := range statusList {
				// string转int
				status, err := strconv.Atoi(status)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"result":      "error",
						"status_code": http.StatusInternalServerError,
						"status_msg":  "<UNK>: " + err.Error(),
					})
					return
				}
				statuses = append(statuses, model.ProxyStatus(status))
			}
			filter.Status = statuses
		}
		if len(req.Type) > 0 {
			typeList := strings.Split(req.Type, ",")
			var types []model.ProxyType
			for _, t := range typeList {
				types = append(types, model.ProxyType(t))
			}
			filter.Types = types
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
		err := proxyTester.TestProxies(ctx, request)
		if err != nil {
			if err.Error() == "已有其他任务正在运行" {
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
