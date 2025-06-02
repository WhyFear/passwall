package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"passwall/internal/service"
)

// ReloadSubscriptionRequest 重新加载订阅请求
type ReloadSubscriptionRequest struct {
	ID uint `json:"id" form:"id"` // 订阅ID，为0表示重新加载所有订阅
}

// ReloadSubscription 重新加载订阅处理器
func ReloadSubscription(proxyTester service.ProxyTester) gin.HandlerFunc {
	// TODO 完善逻辑
	return func(c *gin.Context) {
		var req ReloadSubscriptionRequest

		// 绑定请求参数
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "无效的请求参数",
			})
			return
		}

		// 创建测试请求
		testRequest := &service.TestProxyRequest{
			ReloadSubscribeConfig: true,
		}

		// 调用代理测试服务 - 同步执行
		err := proxyTester.TestProxies(testRequest)
		if err != nil {
			// 如果是因为其他任务正在运行而失败
			if err.Error() == "another task is already running" {
				c.JSON(http.StatusConflict, gin.H{
					"result":      "fail",
					"status_code": http.StatusConflict,
					"status_msg":  "已有其他任务正在运行，请稍后再试",
				})
				return
			}

			// 其他错误
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "重新加载订阅失败: " + err.Error(),
			})
			return
		}

		// 成功完成
		c.JSON(http.StatusOK, gin.H{
			"result":      "success",
			"status_code": http.StatusOK,
			"status_msg":  "重新加载订阅成功",
		})
	}
}
