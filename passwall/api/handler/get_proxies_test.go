package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service"
	"passwall/internal/service/proxy"
	"passwall/internal/service/traffic"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetProxyListReturnsBaseFieldsOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	latestTestTime := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	proxyService := &fakeListProxyService{
		proxies: []*model.Proxy{{
			ID:             7,
			SubscriptionID: uintPtr(3),
			Name:           "node-1",
			Domain:         "example.com",
			Port:           443,
			Type:           model.ProxyTypeTrojan,
			Status:         model.ProxyStatusOK,
			Pinned:         true,
			Ping:           42,
			DownloadSpeed:  1024,
			UploadSpeed:    512,
			LatestTestTime: &latestTestTime,
			CreatedAt:      latestTestTime,
		}},
		total: 1,
	}
	subscriptionManager := &fakeListSubscriptionManager{
		subscriptions: map[uint]*model.Subscription{
			3: {ID: 3, URL: "https://sub.example/list"},
		},
	}
	router := gin.New()
	router.GET("/proxies", GetProxyList(proxyService, subscriptionManager))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/proxies?page=2&pageSize=20&sortField=ping&sortOrder=ascend&status=1&type=trojan&country_code=US&risk_level=low&app_unlock=Netflix,OpenAI", nil)
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, 2, proxyService.page)
	assert.Equal(t, 20, proxyService.pageSize)
	assert.Equal(t, "ping", proxyService.sort)
	assert.Equal(t, "ascend", proxyService.sortOrder)
	require.NotNil(t, proxyService.filters)
	assert.Equal(t, []model.ProxyStatus{model.ProxyStatusOK}, proxyService.filters.Status)
	assert.Equal(t, []model.ProxyType{model.ProxyTypeTrojan}, proxyService.filters.Types)
	assert.Equal(t, []string{"US"}, proxyService.filters.CountryCode)
	assert.Equal(t, []string{"low"}, proxyService.filters.RiskLevel)
	assert.Equal(t, []string{"Netflix", "OpenAI"}, proxyService.filters.AppUnlock)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	items := body["items"].([]interface{})
	require.Len(t, items, 1)
	item := items[0].(map[string]interface{})
	assert.Equal(t, "https://sub.example/list", item["subscription_url"])
	assert.Equal(t, "example.com:443", item["address"])
	assert.NotContains(t, item, "success_rate")
	assert.NotContains(t, item, "download_total")
	assert.NotContains(t, item, "upload_total")
	assert.NotContains(t, item, "ip_info")
}

func TestGetProxyListRejectsInvalidStatusFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/proxies", GetProxyList(&fakeListProxyService{}, &fakeListSubscriptionManager{}))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/proxies?status=bad", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestGetUnlockAppListReturnsSupportedApps(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/get_unlock_apps", GetUnlockAppList())

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/get_unlock_apps", nil)
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	apps := body["data"].([]interface{})
	assert.Contains(t, apps, "TikTok")
	assert.Contains(t, apps, "Netflix")
	assert.Contains(t, apps, "OpenAI")
}

func TestGetProxyMetadataReturnsRequestedFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	speedService := &fakeMetadataSpeedService{
		rates: map[uint]float64{1: 80, 2: 0},
	}
	ipService := &fakeMetadataIPDetector{
		batchInfo: map[uint]*service.IPDetectResp{
			1: {IPv4: "203.0.113.1", Risk: "low", CountryCode: "US"},
		},
	}
	router := gin.New()
	router.GET("/proxies/metadata", GetProxyMetadata(speedService, ipService))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/proxies/metadata?proxy_ids=1,2&include=success_rate,ip_info", nil)
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, []uint{1, 2}, speedService.proxyIDs)
	assert.Equal(t, proxySuccessRateHistories, speedService.limit)
	assert.Equal(t, []uint{1, 2}, ipService.batchProxyIDs)

	var body ProxyMetadataResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Contains(t, body.Items, "1")
	require.Contains(t, body.Items, "2")
	assert.Equal(t, 80.0, *body.Items["1"].SuccessRate)
	assert.Equal(t, 0.0, *body.Items["2"].SuccessRate)
	require.NotNil(t, body.Items["1"].IPInfo)
	assert.Equal(t, "US", body.Items["1"].IPInfo.CountryCode)
	assert.Nil(t, body.Items["2"].IPInfo)
}

func TestGetProxyMetadataValidatesRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/proxies/metadata", GetProxyMetadata(&fakeMetadataSpeedService{}, &fakeMetadataIPDetector{}))

	tooManyIDs := make([]string, 0, proxyMetadataMaxIDs+1)
	for i := 1; i <= proxyMetadataMaxIDs+1; i++ {
		tooManyIDs = append(tooManyIDs, strconv.Itoa(i))
	}

	tests := []string{
		"/proxies/metadata",
		"/proxies/metadata?proxy_ids=abc",
		"/proxies/metadata?proxy_ids=1&include=unknown",
		"/proxies/metadata?proxy_ids=" + strings.Join(tooManyIDs, ","),
	}

	for _, path := range tests {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusBadRequest, resp.Code, path)
	}
}

func TestGetProxyDetailsReturnsTrafficAndIPInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	statisticsService := traffic.NewTrafficStatisticsService(nil, nil, &fakeDetailsTrafficRepo{
		traffic: &model.TrafficStatistics{ProxyID: 7, DownloadTotal: 1234, UploadTotal: 5678},
	})
	ipService := &fakeMetadataIPDetector{
		info: &service.IPDetectResp{
			IPv4:      "203.0.113.7",
			AppUnlock: []*model.IPUnlockInfo{{AppName: "Netflix", Status: "ok", Region: "US"}},
		},
	}
	router := gin.New()
	router.GET("/proxies/:id/details", GetProxyDetails(&statisticsService, ipService))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/proxies/7/details", nil)
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, uint(7), ipService.infoProxyID)

	var body ProxyDetailsResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.NotNil(t, body.Traffic)
	assert.Equal(t, int64(1234), body.Traffic.DownloadTotal)
	require.NotNil(t, body.IPInfo)
	assert.Equal(t, "203.0.113.7", body.IPInfo.IPv4)
	require.Len(t, body.IPInfo.AppUnlock, 1)
}

type fakeListProxyService struct {
	proxy.ProxyService
	proxies   []*model.Proxy
	total     int64
	filters   *repository.NodeFilter
	sort      string
	sortOrder string
	page      int
	pageSize  int
}

func (f *fakeListProxyService) GetProxiesByFilters(filters *repository.NodeFilter, sort string, sortOrder string, page int, pageSize int) ([]*model.Proxy, int64, error) {
	f.filters = filters
	f.sort = sort
	f.sortOrder = sortOrder
	f.page = page
	f.pageSize = pageSize
	return f.proxies, f.total, nil
}

type fakeListSubscriptionManager struct {
	proxy.SubscriptionManager
	subscriptions map[uint]*model.Subscription
}

func (f *fakeListSubscriptionManager) GetSubscriptionByID(id uint) (*model.Subscription, error) {
	return f.subscriptions[id], nil
}

type fakeMetadataSpeedService struct {
	service.SpeedTestHistoryService
	rates    map[uint]float64
	proxyIDs []uint
	limit    int
}

func (f *fakeMetadataSpeedService) GetSuccessRatesByProxyIDList(proxyIDList []uint, limit int) (map[uint]float64, error) {
	f.proxyIDs = proxyIDList
	f.limit = limit
	return f.rates, nil
}

type fakeMetadataIPDetector struct {
	service.IPDetectorService
	batchInfo     map[uint]*service.IPDetectResp
	info          *service.IPDetectResp
	batchProxyIDs []uint
	infoProxyID   uint
}

func (f *fakeMetadataIPDetector) BatchGetInfo(proxyIDList []uint) (map[uint]*service.IPDetectResp, error) {
	f.batchProxyIDs = proxyIDList
	return f.batchInfo, nil
}

func (f *fakeMetadataIPDetector) GetInfo(req *service.IPDetectorReq) (*service.IPDetectResp, error) {
	f.infoProxyID = req.ProxyID
	return f.info, nil
}

type fakeDetailsTrafficRepo struct {
	repository.TrafficRepository
	traffic *model.TrafficStatistics
}

func (f *fakeDetailsTrafficRepo) FindByProxyID(proxyID uint) (*model.TrafficStatistics, error) {
	return f.traffic, nil
}

func uintPtr(value uint) *uint {
	return &value
}

var _ proxy.ProxyService = (*fakeListProxyService)(nil)
var _ proxy.SubscriptionManager = (*fakeListSubscriptionManager)(nil)
var _ service.SpeedTestHistoryService = (*fakeMetadataSpeedService)(nil)
var _ service.IPDetectorService = (*fakeMetadataIPDetector)(nil)
var _ repository.TrafficRepository = (*fakeDetailsTrafficRepo)(nil)
