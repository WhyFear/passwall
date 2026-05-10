package handler

import (
	"errors"
	"fmt"
	"net/http"
	"passwall/internal/service"
	"passwall/internal/service/proxy"
	"passwall/internal/service/traffic"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	proxyMetadataMaxIDs       = 100
	proxySuccessRateHistories = 5
	proxyMetadataSuccessRate  = "success_rate"
	proxyMetadataIPInfo       = "ip_info"
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
	ID              int        `json:"id"`
	SubscriptionUrl string     `json:"subscription_url"`
	Name            string     `json:"name"`
	Address         string     `json:"address"`
	Type            string     `json:"type"`
	Status          int        `json:"status"`
	Pinned          bool       `json:"pinned"`
	Ping            int        `json:"ping"`
	DownloadSpeed   int        `json:"download_speed"`
	UploadSpeed     int        `json:"upload_speed"`
	LatestTestTime  *time.Time `json:"latest_test_time"`
	CreatedAt       time.Time  `json:"created_at"`
}

// PaginatedResponse 分页响应
type PaginatedResponse struct {
	Total int64       `json:"total"`
	Items []ProxyResp `json:"items"`
}

type ProxyMetadataReq struct {
	ProxyIDs string `form:"proxy_ids"`
	Include  string `form:"include"`
}

type ProxyMetadataItem struct {
	SuccessRate *float64              `json:"success_rate,omitempty"`
	IPInfo      *service.IPDetectResp `json:"ip_info,omitempty"`
}

type ProxyMetadataResponse struct {
	Items map[string]ProxyMetadataItem `json:"items"`
}

type ProxyTrafficResp struct {
	DownloadTotal int64 `json:"download_total"`
	UploadTotal   int64 `json:"upload_total"`
}

type ProxyDetailsResponse struct {
	Traffic *ProxyTrafficResp     `json:"traffic,omitempty"`
	IPInfo  *service.IPDetectResp `json:"ip_info,omitempty"`
}

// GetProxyList 获取代理主列表。列表页慢数据由 GetProxyMetadata 和 GetProxyDetails 异步加载。
func GetProxyList(proxyService proxy.ProxyService, subscriptionManager proxy.SubscriptionManager) gin.HandlerFunc {
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

		proxies, total, err := proxyService.GetProxiesByFilters(buildProxyFilters(req), req.SortField, req.SortOrder, req.Page, req.PageSize)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "Failed to get proxies",
			})
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

			result = append(result, ProxyResp{
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
				LatestTestTime:  singleProxy.LatestTestTime,
				CreatedAt:       singleProxy.CreatedAt,
			})
		}

		c.JSON(http.StatusOK, PaginatedResponse{
			Total: total,
			Items: result,
		})
	}
}

func GetProxyMetadata(speedTestHistoryService service.SpeedTestHistoryService, ipDetectService service.IPDetectorService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ProxyMetadataReq
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "Invalid request parameters",
			})
			return
		}

		proxyIDs, err := parseProxyIDList(req.ProxyIDs, proxyMetadataMaxIDs)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  err.Error(),
			})
			return
		}

		includes, err := parseProxyMetadataIncludes(req.Include)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  err.Error(),
			})
			return
		}

		items := make(map[string]ProxyMetadataItem, len(proxyIDs))
		if includes[proxyMetadataSuccessRate] {
			successRateMap, err := speedTestHistoryService.GetSuccessRatesByProxyIDList(proxyIDs, proxySuccessRateHistories)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"result":      "fail",
					"status_code": http.StatusInternalServerError,
					"status_msg":  "Failed to get proxy success rates",
				})
				return
			}
			for proxyID, successRate := range successRateMap {
				key := strconv.FormatUint(uint64(proxyID), 10)
				item := items[key]
				value := successRate
				item.SuccessRate = &value
				items[key] = item
			}
		}

		if includes[proxyMetadataIPInfo] {
			ipInfoMap, err := ipDetectService.BatchGetInfo(proxyIDs)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"result":      "fail",
					"status_code": http.StatusInternalServerError,
					"status_msg":  "Failed to get proxy ip info",
				})
				return
			}
			for proxyID, ipInfo := range ipInfoMap {
				key := strconv.FormatUint(uint64(proxyID), 10)
				item := items[key]
				item.IPInfo = ipInfo
				items[key] = item
			}
		}

		c.JSON(http.StatusOK, ProxyMetadataResponse{Items: items})
	}
}

func GetProxyDetails(statisticsService *traffic.StatisticsService, ipDetectService service.IPDetectorService) gin.HandlerFunc {
	return func(c *gin.Context) {
		proxyID, err := parseProxyIDParam(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"result":      "fail",
				"status_code": http.StatusBadRequest,
				"status_msg":  "Invalid proxy ID",
			})
			return
		}

		resp := ProxyDetailsResponse{}
		trafficStatistics, err := statisticsService.GetTrafficStatistics(proxyID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "Failed to get proxy traffic",
			})
			return
		}
		if trafficStatistics != nil {
			resp.Traffic = &ProxyTrafficResp{
				DownloadTotal: trafficStatistics.DownloadTotal,
				UploadTotal:   trafficStatistics.UploadTotal,
			}
		}

		ipInfo, err := ipDetectService.GetInfo(&service.IPDetectorReq{ProxyID: proxyID})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"result":      "fail",
				"status_code": http.StatusInternalServerError,
				"status_msg":  "Failed to get proxy ip info",
			})
			return
		}
		resp.IPInfo = ipInfo

		c.JSON(http.StatusOK, resp)
	}
}

func buildProxyFilters(req ProxyReq) map[string]interface{} {
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
	return filters
}

func parseProxyIDList(value string, maxIDs int) ([]uint, error) {
	if strings.TrimSpace(value) == "" {
		return nil, errors.New("proxy_ids is required")
	}

	parts := strings.Split(value, ",")
	if len(parts) > maxIDs {
		return nil, fmt.Errorf("proxy_ids cannot contain more than %d ids", maxIDs)
	}

	seen := make(map[uint]bool, len(parts))
	proxyIDs := make([]uint, 0, len(parts))
	for _, part := range parts {
		rawID := strings.TrimSpace(part)
		if rawID == "" {
			return nil, errors.New("proxy_ids contains empty id")
		}
		id, err := strconv.ParseUint(rawID, 10, 32)
		if err != nil || id == 0 {
			return nil, errors.New("proxy_ids contains invalid id")
		}
		proxyID := uint(id)
		if !seen[proxyID] {
			seen[proxyID] = true
			proxyIDs = append(proxyIDs, proxyID)
		}
	}
	return proxyIDs, nil
}

func parseProxyMetadataIncludes(value string) (map[string]bool, error) {
	includes := make(map[string]bool)
	if strings.TrimSpace(value) == "" {
		includes[proxyMetadataSuccessRate] = true
		return includes, nil
	}

	for _, part := range strings.Split(value, ",") {
		include := strings.TrimSpace(part)
		switch include {
		case proxyMetadataSuccessRate, proxyMetadataIPInfo:
			includes[include] = true
		default:
			return nil, errors.New("unknown include: " + include)
		}
	}
	return includes, nil
}

func parseProxyIDParam(value string) (uint, error) {
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil || id == 0 {
		if err != nil {
			return 0, err
		}
		return 0, errors.New("proxy id must be greater than 0")
	}
	return uint(id), nil
}
