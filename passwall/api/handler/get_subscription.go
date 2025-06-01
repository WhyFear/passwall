package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"passwall/internal/repository"
	"time"
)

type SubscriptionReq struct {
	ID      int  `form:"id"`
	Content bool `form:"content"`
}
type SubscriptionResp struct {
	ID        int       `json:"id"`
	Url       string    `json:"url"`
	UpdatedAt time.Time `json:"updated_at"`
	Content   string    `json:"content,omitempty"`
}

// GetSubscriptions 获取存储的订阅链接
func GetSubscriptions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

		// 解析请求参数
		var req SubscriptionReq
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		subscriptionRepo := repository.NewSubscriptionRepository(db)
		// 根据入参是否有ID来判断是否需要获取内容，如果ID大于0，则获取内容，否则获取所有订阅
		// 根据content参数来判断是否需要获取内容，如果content为true，则获取内容，否则获取所有订阅
		var results []SubscriptionResp
		if req.ID > 0 {
			subscription, err := subscriptionRepo.FindByID(uint(req.ID))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch subscription"})
				return
			}
			if req.Content {
				results = append(results, SubscriptionResp{
					ID:        int(subscription.ID),
					Url:       subscription.URL,
					UpdatedAt: subscription.UpdatedAt,
					Content:   subscription.Content,
				})
			} else {
				results = append(results, SubscriptionResp{
					ID:        int(subscription.ID),
					Url:       subscription.URL,
					UpdatedAt: subscription.UpdatedAt,
				})
			}
		} else {
			subscriptions, err := subscriptionRepo.FindAll()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch subscriptions"})
				return
			}
			for _, subscription := range subscriptions {
				if req.Content {
					results = append(results, SubscriptionResp{
						ID:        int(subscription.ID),
						Url:       subscription.URL,
						UpdatedAt: subscription.UpdatedAt,
						Content:   subscription.Content,
					})
				} else {
					results = append(results, SubscriptionResp{
						ID:        int(subscription.ID),
						Url:       subscription.URL,
						UpdatedAt: subscription.UpdatedAt,
					})
				}
			}
		}
		c.JSON(http.StatusOK, results)
	}
}
