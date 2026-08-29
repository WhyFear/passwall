package handler

import (
	"net/http"
	"net/url"
	"passwall/internal/model"
	"passwall/internal/service/proxy"
	"time"

	"github.com/gin-gonic/gin"
)

type SubscriptionReq struct {
	ID       int `form:"id"`
	Page     int `form:"page"`
	PageSize int `form:"pageSize"`
}
type SubscriptionResp struct {
	ID          int       `json:"id"`
	Type        string    `json:"type"`
	URL         string    `json:"url,omitempty"`
	Source      string    `json:"source"`
	Refreshable bool      `json:"refreshable"`
	Status      int       `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ProxyNum    int64     `json:"proxy_num,omitempty"`
	OKProxyNum  int64     `json:"ok_proxy_num,omitempty"`
	AllProxyNum int64     `json:"all_proxy_num,omitempty"`
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
		var valid bool
		req.Page, req.PageSize, valid = normalizePagination(req.Page, req.PageSize)
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "Invalid pagination parameters",
			})
			return
		}

		var items []SubscriptionResp
		var subscriptions []*model.Subscription
		total := int64(1)
		if req.ID > 0 {
			subscription, err := subscriptionManager.GetSubscriptionByID(uint(req.ID))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"result":      "fail",
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
					"result":      "fail",
					"status_code": http.StatusInternalServerError,
					"status_msg":  "Failed to fetch subscriptions",
				})
				return
			}
			total = subsTotal
			subscriptions = allSubscriptions
		}
		subscriptionIDs := make([]uint, 0, len(subscriptions))
		for _, subscription := range subscriptions {
			subscriptionIDs = append(subscriptionIDs, subscription.ID)
		}
		proxyCounts, err := proxyService.GetProxyCountsBySubscriptionIDs(subscriptionIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "Failed to count subscription proxies",
			})
			return
		}
		for _, subscription := range subscriptions {
			counts := proxyCounts[subscription.ID]
			refreshable, source := subscriptionSource(subscription.URL)
			tempSubscription := SubscriptionResp{
				ID:          int(subscription.ID),
				Type:        string(subscription.Type),
				Source:      source,
				Refreshable: refreshable,
				Status:      int(subscription.Status),
				CreatedAt:   subscription.CreatedAt,
				UpdatedAt:   subscription.UpdatedAt,
				OKProxyNum:  counts.OKCount,
				ProxyNum:    counts.ValidCount,
				AllProxyNum: counts.AllCount,
			}
			if req.ID > 0 && refreshable {
				tempSubscription.URL = subscription.URL
			}
			items = append(items, tempSubscription)
		}

		c.JSON(http.StatusOK, SubsPageResp{
			Total: total,
			Items: items,
		})
	}
}

func subscriptionSource(rawURL string) (bool, string) {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return false, "本地导入"
	}
	return true, parsed.Hostname()
}
