package handler

import (
	"net/http"
	"passwall/internal/model"
	"passwall/internal/repository"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"gorm.io/gorm"
)

type Page struct {
	Page     int `form:"page" json:"page"`
	PageSize int `form:"page_size" json:"page_size"`
}

type ProxyReq struct {
	Page   Page   `form:"page"`
	Status int    `form:"status"`
	Order  string `form:"order"`
}

type ProxyResp struct {
	ID              int       `json:"id"`
	SubscriptionUrl string    `json:"subscription_url"`
	Name            string    `json:"name"`
	Address         string    `json:"address"`
	Status          int       `json:"status"`
	Ping            int       `json:"ping"`
	DownloadSpeed   int64     `json:"download_speed"`
	UploadSpeed     int64     `json:"upload_speed"`
	TestedAt        time.Time `json:"tested_at"`
	ShareUrl        string    `json:"share_url"`
}

// PaginatedResponse 分页响应
type PaginatedResponse struct {
	Total int64       `json:"total"`
	Items interface{} `json:"items"`
}

// GetProxies 获取所有代理
func GetProxies(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 解析请求参数
		var req ProxyReq
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 设置默认值
		if req.Page.Page <= 0 {
			req.Page.Page = 1
		}
		if req.Page.PageSize <= 0 {
			req.Page.PageSize = 10
		}

		// 构建查询条件
		proxyRepo := repository.NewProxyRepository(db)
		subscriptionRepo := repository.NewSubscriptionRepository(db)

		// 构建过滤条件
		filters := make(map[string]interface{})
		if req.Status != 0 {
			filters["status"] = req.Status
		}

		// 构建分页查询参数
		pageQuery := repository.PageQuery{
			Page:     req.Page.Page,
			PageSize: req.Page.PageSize,
			Filters:  filters,
		}

		// 设置排序
		if req.Order != "" {
			pageQuery.OrderBy = req.Order
		} else {
			pageQuery.OrderBy = "created_at DESC"
		}

		// 执行分页查询
		queryResult, err := proxyRepo.FindPage(pageQuery)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取代理列表失败"})
			return
		}

		result := make([]ProxyResp, 0, len(queryResult.Items))
		for _, proxy := range queryResult.Items {
			subscription, err := subscriptionRepo.FindByID(*proxy.SubscriptionID)
			if err != nil {
				subscription = &model.Subscription{
					URL: "未知",
				}
			}
			result = append(result, ProxyResp{
				ID:              int(proxy.ID),
				SubscriptionUrl: subscription.URL,
				Name:            proxy.Name,
				Address:         proxy.Domain + ":" + strconv.Itoa(proxy.Port),
				Status:          int(proxy.Status),
				Ping:            proxy.Ping,
				DownloadSpeed:   proxy.DownloadSpeed,
				UploadSpeed:     proxy.UploadSpeed,
				TestedAt:        *proxy.LatestTestTime,
			})
		}

		// 返回分页数据
		c.JSON(http.StatusOK, PaginatedResponse{
			Total: queryResult.Total,
			Items: result,
		})
	}
}
