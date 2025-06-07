package handler

import (
	"net/http"
	"passwall/internal/model"
	"passwall/internal/service/proxy"
	"time"

	"github.com/gin-gonic/gin"
)

type SubscriptionReq struct {
	ID      int  `form:"id"`
	Content bool `form:"content"`
}
type SubscriptionResp struct {
	ID        int       `json:"id"`
	Url       string    `json:"url"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Content   string    `json:"content,omitempty"`
}

// GetSubscriptions 获取存储的订阅链接
func GetSubscriptions(subscriptionManager proxy.SubscriptionManager) gin.HandlerFunc {
	return func(c *gin.Context) {

		// 解析请求参数
		var req SubscriptionReq
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 根据入参是否有ID来判断是否需要获取内容，如果ID大于0，则获取内容，否则获取所有订阅
		// 根据content参数来判断是否需要获取内容，如果content为true，则获取内容，否则获取所有订阅
		var results []SubscriptionResp
		var subscriptions []*model.Subscription
		if req.ID > 0 {
			subscription, err := subscriptionManager.GetSubscriptionByID(uint(req.ID))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch subscription"})
				return
			}
			subscriptions = append(subscriptions, subscription)
		} else {
			allSubscriptions, err := subscriptionManager.GetAllSubscriptions()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch subscriptions"})
				return
			}
			subscriptions = allSubscriptions
		}
		for _, subscription := range subscriptions {
			tempSubscription := SubscriptionResp{
				ID:        int(subscription.ID),
				Url:       subscription.URL,
				Status:    int(subscription.Status),
				CreatedAt: subscription.CreatedAt,
				UpdatedAt: subscription.UpdatedAt,
			}
			if req.Content {
				tempSubscription.Content = subscription.Content
			}
			results = append(results, tempSubscription)
		}
		c.JSON(http.StatusOK, results)
	}
}
