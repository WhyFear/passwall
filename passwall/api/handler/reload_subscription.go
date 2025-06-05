package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"passwall/internal/service/proxy"
)

// ReloadSubscriptionRequest 重新加载订阅请求
type ReloadSubscriptionRequest struct {
	ID uint `json:"id" form:"id"` // 订阅ID，为0表示重新加载所有订阅
}

// ReloadSubscription 重新加载订阅处理器
func ReloadSubscription(ctx context.Context, subscriptionManager proxy.SubscriptionManager) gin.HandlerFunc {
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

		var err error
		if req.ID > 0 {
			// 刷新单个订阅
			err = subscriptionManager.RefreshSubscription(ctx, req.ID)
		} else {
			// 刷新所有订阅
			err = subscriptionManager.RefreshAllSubscriptions(ctx)
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
