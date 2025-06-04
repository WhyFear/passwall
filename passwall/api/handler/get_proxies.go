package handler

import (
	"net/http"
	"passwall/internal/service"
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
func GetProxies(proxyService service.ProxyService) gin.HandlerFunc {
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
		proxies, err := proxyService.GetAllProxies(filters)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取代理列表失败"})
			return
		}

		// 转换为响应格式
		result := make([]ProxyResp, 0, len(proxies))
		for _, proxy := range proxies {
			subscriptionUrl := "未知"
			if proxy.SubscriptionID != nil {
				// 由于不能直接从service获取subscription信息，这里简化处理
				// 在实际应用中，应该通过SubscriptionService获取订阅信息
				subscriptionUrl = "订阅ID: " + strconv.FormatUint(uint64(*proxy.SubscriptionID), 10)
			}

			tempProxy := ProxyResp{
				ID:              int(proxy.ID),
				SubscriptionUrl: subscriptionUrl,
				Name:            proxy.Name,
				Address:         proxy.Domain + ":" + strconv.Itoa(proxy.Port),
				Type:            string(proxy.Type),
				Status:          int(proxy.Status),
				Ping:            proxy.Ping,
				DownloadSpeed:   proxy.DownloadSpeed,
				UploadSpeed:     proxy.UploadSpeed,
			}
			if proxy.LatestTestTime != nil {
				tempProxy.LatestTestTime = *proxy.LatestTestTime
			}
			result = append(result, tempProxy)
		}

		// 计算分页数据
		total := int64(len(result))
		start := (req.Page - 1) * req.PageSize
		end := start + req.PageSize
		if start >= len(result) {
			result = []ProxyResp{}
		} else if end > len(result) {
			result = result[start:]
		} else {
			result = result[start:end]
		}

		// 返回分页数据
		c.JSON(http.StatusOK, PaginatedResponse{
			Total: total,
			Items: result,
		})
	}
}
