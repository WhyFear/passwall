package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"passwall/internal/adapter/generator"
	"passwall/internal/model"
	"passwall/internal/service"
	"passwall/internal/service/proxy"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSubscribeFiltersIncludesAppUnlock(t *testing.T) {
	filters := buildSubscribeFilters(SubscribeReq{
		StatusStr:   "1,2",
		ProxyType:   "trojan",
		CountryCode: "US,JP",
		RiskLevel:   "low",
		AppUnlock:   "Netflix,OpenAI",
	})

	assert.Equal(t, []string{"1", "2"}, filters["status"])
	assert.Equal(t, []string{"trojan"}, filters["type"])
	assert.Equal(t, []string{"US", "JP"}, filters["country_code"])
	assert.Equal(t, []string{"low"}, filters["risk_level"])
	assert.Equal(t, []string{"Netflix", "OpenAI"}, filters["app_unlock"])
}

func TestGetSharedSubscribeReplaysAppUnlockFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	shareService := &fakeSubscribeShareConfigService{
		config: &model.ShareConfig{
			Type:      "test",
			Status:    "1",
			AppUnlock: "Netflix,OpenAI",
			Sort:      "download_speed",
			SortOrder: "descend",
			Limit:     10,
		},
	}
	proxyService := &fakeSubscribeProxyService{
		proxies: []*model.Proxy{{ID: 1, Name: "node-1"}},
		total:   1,
	}
	generatorFactory := generator.NewGeneratorFactory()
	generatorFactory.RegisterGenerator("test", fakeSubscribeGenerator{})

	router := gin.New()
	router.GET("/s/:slug", GetSharedSubscribe(shareService, proxyService, generatorFactory))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/s/demo", nil)
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "demo", shareService.slug)
	assert.Equal(t, []string{"Netflix", "OpenAI"}, proxyService.filters["app_unlock"])
	assert.Equal(t, "download_speed", proxyService.sort)
	assert.Equal(t, "descend", proxyService.sortOrder)
	assert.Equal(t, 1, proxyService.page)
	assert.Equal(t, 10, proxyService.pageSize)
	assert.Equal(t, "generated", resp.Body.String())
}

func TestGetSharedSubscribeReturnsNotFoundForMissingConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/s/:slug", GetSharedSubscribe(
		&fakeSubscribeShareConfigService{err: errors.New("not found")},
		&fakeSubscribeProxyService{},
		generator.NewGeneratorFactory(),
	))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/s/missing", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Empty(t, resp.Body.String())
}

type fakeSubscribeShareConfigService struct {
	service.ShareConfigService
	config *model.ShareConfig
	err    error
	slug   string
}

func (f *fakeSubscribeShareConfigService) GetEnabledBySlug(slug string) (*model.ShareConfig, error) {
	f.slug = slug
	if f.err != nil {
		return nil, f.err
	}
	return f.config, nil
}

type fakeSubscribeProxyService struct {
	proxy.ProxyService
	proxies   []*model.Proxy
	total     int64
	filters   map[string]interface{}
	sort      string
	sortOrder string
	page      int
	pageSize  int
}

func (f *fakeSubscribeProxyService) GetProxiesByFilters(filters map[string]interface{}, sort string, sortOrder string, page int, pageSize int) ([]*model.Proxy, int64, error) {
	f.filters = filters
	f.sort = sort
	f.sortOrder = sortOrder
	f.page = page
	f.pageSize = pageSize
	return f.proxies, f.total, nil
}

type fakeSubscribeGenerator struct{}

func (fakeSubscribeGenerator) Generate(_ []*model.Proxy) ([]byte, error) {
	return []byte("generated"), nil
}

func (fakeSubscribeGenerator) Format() string {
	return "test"
}
