package handler

import (
	"net/http"
	"passwall/internal/model"
	"passwall/internal/service/proxy"
	"time"

	"github.com/metacubex/mihomo/log"

	"github.com/gin-gonic/gin"
)

type SubscriptionReq struct {
	ID       int  `form:"id"`
	Content  bool `form:"content"`
	Page     int  `form:"page"`
	PageSize int  `form:"pageSize"`
}
type SubscriptionResp struct {
	ID          int       `json:"id"`
	Url         string    `json:"url"`
	Status      int       `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ProxyNum    int64     `json:"proxy_num,omitempty"`
	OKProxyNum  int64     `json:"ok_proxy_num,omitempty"`
	AllProxyNum int64     `json:"all_proxy_num,omitempty"`
	Content     string    `json:"content,omitempty"`
}
type SubsPageResp struct {
	Total int64              `json:"total"`
	Items []SubscriptionResp `json:"items"`
}

// GetSubscriptions 获取存储的订阅链接
func GetSubscriptions(subscriptionManager proxy.SubscriptionManager, proxyService proxy.ProxyService) gin.HandlerFunc {
	return func(c *gin.Context) {

		var req SubscriptionReq
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      err.Error(),
				"status_code": http.StatusBadRequest,
				"status_msg":  "Invalid request parameters",
			})
			return
		}

		// 根据入参是否有ID来判断是否需要获取内容，如果ID大于0，则获取内容，否则获取所有订阅
		// 根据content参数来判断是否需要获取内容，如果content为true，则获取内容，否则获取所有订阅
		var items []SubscriptionResp
		var subscriptions []*model.Subscription
		total := int64(1)
		if req.ID > 0 {
			subscription, err := subscriptionManager.GetSubscriptionByID(uint(req.ID))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"result":      err.Error(),
					"status_code": http.StatusInternalServerError,
					"status_msg":  "Failed to fetch subscription",
				})
				return
			}
			subscriptions = append(subscriptions, subscription)
		} else {
			subsReq := proxy.SubsPage{
				Page:     req.Page,
				PageSize: req.PageSize,
			}
			allSubscriptions, subsTotal, err := subscriptionManager.GetSubscriptionsPage(subsReq)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"result":      err.Error(),
					"status_code": http.StatusInternalServerError,
					"status_msg":  "Failed to fetch subscriptions",
				})
				return
			}
			total = subsTotal
			subscriptions = allSubscriptions
		}
		for _, subscription := range subscriptions {
			OKProxyNum, err := proxyService.GetProxyNumBySubscriptionID(subscription.ID, false, true)
			if err != nil {
				log.Infoln("Failed to get proxy num: %v", err)
				OKProxyNum = 0
			}
			// 获取代理数量
			validProxyNum, err := proxyService.GetProxyNumBySubscriptionID(subscription.ID, true, false)
			if err != nil {
				log.Infoln("Failed to get proxy num: %v", err)
				validProxyNum = 0
			}
			proxyNum, err := proxyService.GetProxyNumBySubscriptionID(subscription.ID, false, false)
			if err != nil {
				log.Infoln("Failed to get proxy num: %v", err)
				proxyNum = 0
			}
			tempSubscription := SubscriptionResp{
				ID:          int(subscription.ID),
				Url:         subscription.URL,
				Status:      int(subscription.Status),
				CreatedAt:   subscription.CreatedAt,
				UpdatedAt:   subscription.UpdatedAt,
				OKProxyNum:  OKProxyNum,
				ProxyNum:    validProxyNum,
				AllProxyNum: proxyNum,
			}
			if req.Content {
				tempSubscription.Content = subscription.Content
			}
			items = append(items, tempSubscription)
		}

		c.JSON(http.StatusOK, SubsPageResp{
			Total: total,
			Items: items,
		})
	}
}
