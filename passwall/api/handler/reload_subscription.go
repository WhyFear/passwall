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
func ReloadSubscription(subscriptionService service.SubscriptionService) gin.HandlerFunc {
	// TODO 完善逻辑
	// ID=0 表示重新加载所有订阅
	return func(c *gin.Context) {

		// 成功完成
		c.JSON(http.StatusOK, gin.H{
			"result":      "success",
			"status_code": http.StatusOK,
			"status_msg":  "重新加载订阅成功",
		})
	}
}
