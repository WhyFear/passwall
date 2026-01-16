package handler

import (
	"net/http"
	"passwall/internal/service"
	"passwall/internal/service/proxy"
	"passwall/internal/service/traffic"
	"strconv"
	"strings"
	"time"

	"github.com/metacubex/mihomo/log"

	"github.com/gin-gonic/gin"
)

type ProxyReq struct {
	Page        int    `form:"page" json:"page"`
	PageSize    int    `form:"pageSize" json:"pageSize"`
	Status      string `form:"status"`
	Type        string `form:"type"`
	CountryCode string `form:"country_code"`
	RiskLevel   string `form:"risk_level"`
	SortField   string `form:"sortField"`
	SortOrder   string `form:"sortOrder"`
}

type ProxyResp struct {
	ID              int                   `json:"id"`
	SubscriptionUrl string                `json:"subscription_url"`
	Name            string                `json:"name"`
	Address         string                `json:"address"`
	Type            string                `json:"type"`
	Status          int                   `json:"status"`
	Pinned          bool                  `json:"pinned"`
	Ping            int                   `json:"ping"`
	DownloadSpeed   int                   `json:"download_speed"`
	UploadSpeed     int                   `json:"upload_speed"`
	LatestTestTime  time.Time             `json:"latest_test_time"`
	ShareUrl        string                `json:"share_url"`
	CreatedAt       time.Time             `json:"created_at"`
	SuccessRate     float64               `json:"success_rate"`
	DownloadTotal   int64                 `json:"download_total"`
	UploadTotal     int64                 `json:"upload_total"`
	IPInfo          *service.IPDetectResp `json:"ip_info,omitempty"`
}

// PaginatedResponse 分页响应
type PaginatedResponse struct {
	Total int64       `json:"total"`
	Items []ProxyResp `json:"items"`
}

// GetProxies 获取所有代理
func GetProxies(proxyService proxy.ProxyService, subscriptionManager proxy.SubscriptionManager,
	speedTestHistoryService service.SpeedTestHistoryService, statisticsService *traffic.StatisticsService,
	ipDetectService service.IPDetectorService,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ProxyReq
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "Invalid request parameters",
			})
			return
		}

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
		if len(req.CountryCode) > 0 {
			filters["country_code"] = strings.Split(req.CountryCode, ",")
		}
		if len(req.RiskLevel) > 0 {
			filters["risk_level"] = strings.Split(req.RiskLevel, ",")
		}

		// 获取所有代理
		proxies, total, err := proxyService.GetProxiesByFilters(filters, req.SortField, req.SortOrder, req.Page, req.PageSize)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "Failed to get proxies",
			})
			return
		}

		proxyIDList := make([]uint, 0, len(proxies))
		for _, singleProxy := range proxies {
			proxyIDList = append(proxyIDList, singleProxy.ID)
		}
		speedTestHistoryMap, err := speedTestHistoryService.BatchGetSpeedTestHistoryByProxyIDList(proxyIDList)
		if err != nil {
			log.Infoln("获取代理列表的测速历史记录失败: %v", err)
		}
		trafficMap, err := statisticsService.BatchGetTrafficStatistics(proxyIDList)
		if err != nil {
			log.Infoln("获取代理列表的消耗流量失败: %v", err)
		}

		subscriptionUrls := make(map[uint]string)
		result := make([]ProxyResp, 0, len(proxies))
		for _, singleProxy := range proxies {
			// 获取订阅链接
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
			// 获取节点测试成功率
			successRate := 0.0
			if histories, ok := speedTestHistoryMap[singleProxy.ID]; ok {
				successCount := 0
				for _, history := range histories {
					if history.DownloadSpeed > 0 {
						successCount++
					}
				}
				if successCount == 0 {
					log.Infoln("代理 %s 的测速历史记录中没有成功的记录", singleProxy.Name)
				}
				if len(histories) > 0 {
					// 两位小数
					successRate = float64(successCount) / float64(len(histories)) * 100
					successRate, _ = strconv.ParseFloat(strconv.FormatFloat(successRate, 'f', 2, 64), 64)
				}
			} else {
				log.Infoln("代理 %s 不存在历史测速记录", singleProxy.Name)
			}
			ipInfo, err := ipDetectService.GetInfo(&service.IPDetectorReq{
				ProxyID: singleProxy.ID,
			})
			if err != nil {
				log.Infoln("获取代理 %d 的IP信息失败: %v", singleProxy.ID, err)
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
				CreatedAt:       singleProxy.CreatedAt,
				SuccessRate:     successRate,
			}
			if singleProxy.LatestTestTime != nil {
				tempProxy.LatestTestTime = *singleProxy.LatestTestTime
			}
			if trafficStatistics, ok := trafficMap[singleProxy.ID]; ok {
				tempProxy.DownloadTotal = trafficStatistics.DownloadTotal
				tempProxy.UploadTotal = trafficStatistics.UploadTotal
			}
			if ipInfo != nil {
				tempProxy.IPInfo = ipInfo
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
