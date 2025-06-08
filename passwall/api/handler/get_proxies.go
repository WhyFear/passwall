package handler

import (
	"net/http"
	"passwall/internal/service"
	"passwall/internal/service/proxy"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ProxyReq struct {
	Page      int    `form:"page" json:"page"`
	PageSize  int    `form:"pageSize" json:"pageSize"`
	Status    string `form:"status"`
	Type      string `form:"type"`
	SortField string `form:"sortField"`
	SortOrder string `form:"sortOrder"`
}

type ProxyResp struct {
	ID              int       `json:"id"`
	SubscriptionUrl string    `json:"subscription_url"`
	Name            string    `json:"name"`
	Address         string    `json:"address"`
	Type            string    `json:"type"`
	Status          int       `json:"status"`
	Pinned          bool      `json:"pinned"`
	Ping            int       `json:"ping"`
	DownloadSpeed   int       `json:"download_speed"`
	UploadSpeed     int       `json:"upload_speed"`
	LatestTestTime  time.Time `json:"latest_test_time"`
	ShareUrl        string    `json:"share_url"`
}

// PaginatedResponse 分页响应
type PaginatedResponse struct {
	Total int64       `json:"total"`
	Items interface{} `json:"items"`
}

// GetProxies 获取所有代理
func GetProxies(proxyService service.ProxyService, subscriptionManager proxy.SubscriptionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 解析请求参数
		var req ProxyReq
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 设置默认值
		if req.Page <= 0 {
			req.Page = 1
		}
		if req.PageSize <= 0 {
			req.PageSize = 10
		}

		// 构建过滤条件
		filters := make(map[string]interface{})
		if len(req.Status) > 0 {
			filters["status"] = strings.Split(req.Status, ",")
		}
		if len(req.Type) > 0 {
			filters["type"] = strings.Split(req.Type, ",")
		}

		// 获取所有代理
		proxies, total, err := proxyService.GetProxiesByFilters(filters, req.SortField, req.SortOrder, req.Page, req.PageSize)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取代理列表失败"})
			return
		}

		subscriptionUrls := make(map[uint]string)

		result := make([]ProxyResp, 0, len(proxies))
		for _, singleProxy := range proxies {
			subscriptionUrl := "未知"
			if singleProxy.SubscriptionID != nil {
				if url, ok := subscriptionUrls[*singleProxy.SubscriptionID]; ok {
					subscriptionUrl = url
				} else {
					subscription, err := subscriptionManager.GetSubscriptionByID(*singleProxy.SubscriptionID)
					if err == nil && subscription != nil {
						subscriptionUrl = subscription.URL
						subscriptionUrls[*singleProxy.SubscriptionID] = subscriptionUrl
					}
				}
			}

			tempProxy := ProxyResp{
				ID:              int(singleProxy.ID),
				SubscriptionUrl: subscriptionUrl,
				Name:            singleProxy.Name,
				Address:         singleProxy.Domain + ":" + strconv.Itoa(singleProxy.Port),
				Type:            string(singleProxy.Type),
				Status:          int(singleProxy.Status),
				Pinned:          singleProxy.Pinned,
				Ping:            singleProxy.Ping,
				DownloadSpeed:   singleProxy.DownloadSpeed,
				UploadSpeed:     singleProxy.UploadSpeed,
			}
			if singleProxy.LatestTestTime != nil {
				tempProxy.LatestTestTime = *singleProxy.LatestTestTime
			}
			result = append(result, tempProxy)
		}
		// 返回分页数据
		c.JSON(http.StatusOK, PaginatedResponse{
			Total: total,
			Items: result,
		})
	}
}
