package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"passwall/internal/adapter/generator"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service"
	"passwall/internal/service/proxy"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testShareSlug = "AAAAAAAAAAAAAAAA"

func TestParseNodeFilterIncludesAllSupportedFilters(t *testing.T) {
	filters, err := parseNodeFilter("1,2", "trojan", "US,JP", "low", "Netflix,OpenAI")

	require.NoError(t, err)
	require.NotNil(t, filters)
	assert.Equal(t, []model.ProxyStatus{model.ProxyStatusOK, model.ProxyStatusFailed}, filters.Status)
	assert.Equal(t, []model.ProxyType{model.ProxyTypeTrojan}, filters.Types)
	assert.Equal(t, []string{"US", "JP"}, filters.CountryCode)
	assert.Equal(t, []string{"low"}, filters.RiskLevel)
	assert.Equal(t, []string{"Netflix", "OpenAI"}, filters.AppUnlock)
}

func TestParseNodeFilterReturnsNilForEmptyInput(t *testing.T) {
	filters, err := parseNodeFilter("", "  ", "", "", "")

	require.NoError(t, err)
	assert.Nil(t, filters)
}

func TestParseNodeFilterRejectsInvalidStatus(t *testing.T) {
	filters, err := parseNodeFilter("bad", "trojan", "", "", "")

	require.ErrorIs(t, err, errInvalidNodeFilter)
	assert.Nil(t, filters)
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
	req := httptest.NewRequest(http.MethodGet, "/s/"+testShareSlug, nil)
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, testShareSlug, shareService.slug)
	require.NotNil(t, proxyService.filters)
	assert.Equal(t, []model.ProxyStatus{model.ProxyStatusOK}, proxyService.filters.Status)
	assert.Equal(t, []string{"Netflix", "OpenAI"}, proxyService.filters.AppUnlock)
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
	req := httptest.NewRequest(http.MethodGet, "/s/"+testShareSlug, nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Empty(t, resp.Body.String())
}

func TestGetSharedSubscribeRejectsInvalidSlugBeforeLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	shareService := &fakeSubscribeShareConfigService{config: &model.ShareConfig{Type: "test"}}
	router := gin.New()
	router.GET("/s/:slug", GetSharedSubscribe(shareService, &fakeSubscribeProxyService{}, generator.NewGeneratorFactory()))

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/s/short", nil))

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Empty(t, shareService.slug)
}

func TestGetSharedSubscribeCapsNodesAndRateLimitsBurst(t *testing.T) {
	gin.SetMode(gin.TestMode)
	proxyService := &fakeSubscribeProxyService{proxies: []*model.Proxy{{ID: 1, Name: "node-1"}}}
	generatorFactory := generator.NewGeneratorFactory()
	generatorFactory.RegisterGenerator("test", fakeSubscribeGenerator{})
	router := gin.New()
	router.GET("/s/:slug", GetSharedSubscribe(
		&fakeSubscribeShareConfigService{config: &model.ShareConfig{Type: "test"}},
		proxyService,
		generatorFactory,
	))

	for i := 0; i < 5; i++ {
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/s/"+testShareSlug, nil))
		if i < 4 {
			assert.Equal(t, http.StatusOK, resp.Code)
		} else {
			assert.Equal(t, http.StatusTooManyRequests, resp.Code)
		}
	}
	assert.Equal(t, 1000, proxyService.pageSize)
}

func TestGetSharedSubscribeLimitsConcurrentGeneration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	shareService := &blockingShareConfigService{
		entered: make(chan struct{}, 4),
		release: make(chan struct{}),
	}
	generatorFactory := generator.NewGeneratorFactory()
	generatorFactory.RegisterGenerator("test", fakeSubscribeGenerator{})
	router := gin.New()
	router.GET("/s/:slug", GetSharedSubscribe(
		shareService,
		&fakeSubscribeProxyService{proxies: []*model.Proxy{{ID: 1}}},
		generatorFactory,
	))

	codes := make(chan int, 4)
	for i := 0; i < 4; i++ {
		go func() {
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/s/"+testShareSlug, nil))
			codes <- resp.Code
		}()
	}
	for i := 0; i < 4; i++ {
		<-shareService.entered
	}
	time.Sleep(1100 * time.Millisecond)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/s/"+testShareSlug, nil))
	assert.Equal(t, http.StatusTooManyRequests, resp.Code)
	close(shareService.release)
	for i := 0; i < 4; i++ {
		assert.Equal(t, http.StatusOK, <-codes)
	}
}

func TestGetSubscribeGeneratesContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	proxyService := &fakeSubscribeProxyService{
		proxies: []*model.Proxy{{ID: 1, Name: "node-1"}},
		total:   1,
	}
	generatorFactory := generator.NewGeneratorFactory()
	generatorFactory.RegisterGenerator("test", fakeSubscribeGenerator{})
	router := gin.New()
	router.GET("/subscribe", GetSubscribe(proxyService, generatorFactory))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/subscribe?type=test&status=1&proxy_type=ss&country_code=US&risk_level=low&app_unlock=Netflix", nil)
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, proxyService.filters)
	assert.Equal(t, []model.ProxyStatus{model.ProxyStatusOK}, proxyService.filters.Status)
	assert.Equal(t, []model.ProxyType{model.ProxyTypeSS}, proxyService.filters.Types)
	assert.Equal(t, []string{"US"}, proxyService.filters.CountryCode)
	assert.Equal(t, []string{"low"}, proxyService.filters.RiskLevel)
	assert.Equal(t, []string{"Netflix"}, proxyService.filters.AppUnlock)
	assert.Equal(t, "generated", resp.Body.String())
}

func TestGetSubscribeRejectsInvalidStatusFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	generatorFactory := generator.NewGeneratorFactory()
	generatorFactory.RegisterGenerator("test", fakeSubscribeGenerator{})
	router := gin.New()
	router.GET("/subscribe", GetSubscribe(&fakeSubscribeProxyService{}, generatorFactory))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/subscribe?type=test&status=bad", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestGetSubscribeRejectsLimitOverMaximum(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/subscribe", GetSubscribe(&fakeSubscribeProxyService{}, generator.NewGeneratorFactory()))

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/subscribe?type=test&limit=1001", nil))

	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestGetSubscribeRejectsUnsupportedType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/subscribe", GetSubscribe(&fakeSubscribeProxyService{
		proxies: []*model.Proxy{{ID: 1, Name: "node-1"}},
		total:   1,
	}, generator.NewGeneratorFactory()))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/subscribe?type=missing&status=1", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestGetSubscribeMapsGeneratorNotImplementedError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	generatorFactory := generator.NewGeneratorFactory()
	generatorFactory.RegisterGenerator("test", fakeSubscribeGenerator{err: errors.New("没有可生成分享链接的代理")})
	router := gin.New()
	router.GET("/subscribe", GetSubscribe(&fakeSubscribeProxyService{
		proxies: []*model.Proxy{{ID: 1, Name: "node-1"}},
		total:   1,
	}, generatorFactory))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/subscribe?type=test&status=1", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestGetSubscribeMapsGenericGeneratorError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	generatorFactory := generator.NewGeneratorFactory()
	generatorFactory.RegisterGenerator("test", fakeSubscribeGenerator{err: errors.New("raw-subscription-secret")})
	router := gin.New()
	router.GET("/subscribe", GetSubscribe(&fakeSubscribeProxyService{
		proxies: []*model.Proxy{{ID: 1, Name: "node-1"}},
		total:   1,
	}, generatorFactory))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/subscribe?type=test&status=1", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.NotContains(t, resp.Body.String(), "raw-subscription-secret")
}

func TestGenerateSubscribeContentUsesSingleProxyByID(t *testing.T) {
	proxyService := &fakeSubscribeProxyService{
		proxyByID: &model.Proxy{ID: 7, Name: "single"},
	}
	generatorFactory := generator.NewGeneratorFactory()
	generatorFactory.RegisterGenerator("test", fakeSubscribeGenerator{})

	content, err := GenerateSubscribeContent(SubscribeReq{Type: "test", ID: 7}, proxyService, generatorFactory)

	require.NoError(t, err)
	assert.Equal(t, uint(7), proxyService.proxyID)
	assert.Equal(t, []byte("generated"), content)
}

func TestGenerateSubscribeContentReturnsEmptyWhenSingleProxyMissing(t *testing.T) {
	proxyService := &fakeSubscribeProxyService{err: errors.New("missing")}
	generatorFactory := generator.NewGeneratorFactory()
	generatorFactory.RegisterGenerator("test", fakeSubscribeGenerator{})

	content, err := GenerateSubscribeContent(SubscribeReq{Type: "test", ID: 7}, proxyService, generatorFactory)

	require.NoError(t, err)
	assert.Empty(t, content)
}

func TestGenerateSubscribeContentReturnsQueryError(t *testing.T) {
	proxyService := &fakeSubscribeProxyService{err: errors.New("query failed")}
	generatorFactory := generator.NewGeneratorFactory()
	generatorFactory.RegisterGenerator("test", fakeSubscribeGenerator{})

	content, err := GenerateSubscribeContent(SubscribeReq{Type: "test", StatusStr: "1"}, proxyService, generatorFactory)

	require.Error(t, err)
	assert.Nil(t, content)
}

func TestGenerateSubscribeContentReturnsEmptyWhenNoProxiesMatch(t *testing.T) {
	proxyService := &fakeSubscribeProxyService{}
	generatorFactory := generator.NewGeneratorFactory()
	generatorFactory.RegisterGenerator("test", fakeSubscribeGenerator{})

	content, err := GenerateSubscribeContent(SubscribeReq{Type: "test", StatusStr: "1"}, proxyService, generatorFactory)

	require.NoError(t, err)
	assert.Empty(t, content)
}

func TestGenerateSubscribeContentEncodesStableIDsInRuntimeNames(t *testing.T) {
	proxies := []*model.Proxy{
		{ID: 1, Name: "same", Config: `{"name":"old-1"}`},
		{ID: 2, Name: "same", Config: `{"name":"old-2"}`},
	}
	proxyService := &fakeSubscribeProxyService{proxies: proxies, total: 1}
	generated := make([]*model.Proxy, 0, len(proxies))
	generatorFactory := generator.NewGeneratorFactory()
	generatorFactory.RegisterGenerator(SubscribeTypeClash, capturingSubscribeGenerator{generated: &generated})

	content, err := GenerateSubscribeContent(SubscribeReq{Type: SubscribeTypeClash, WithIndex: true}, proxyService, generatorFactory)

	require.NoError(t, err)
	assert.Equal(t, []byte("generated"), content)
	require.Len(t, generated, 2)
	assert.Equal(t, "[pw:1]-[1]-same", generated[0].Name)
	assert.Equal(t, "[pw:2]-[2]-same", generated[1].Name)
	assert.Contains(t, generated[0].Config, `"name":"[pw:1]-[1]-same"`)
	assert.Contains(t, generated[1].Config, `"name":"[pw:2]-[2]-same"`)
	assert.Equal(t, "same", proxies[0].Name, "generation must not mutate repository models")
	assert.Equal(t, `{"name":"old-1"}`, proxies[0].Config)
}

type fakeSubscribeShareConfigService struct {
	service.ShareConfigService
	config *model.ShareConfig
	err    error
	slug   string
}

type blockingShareConfigService struct {
	service.ShareConfigService
	entered chan struct{}
	release chan struct{}
}

func (f *blockingShareConfigService) GetEnabledBySlug(string) (*model.ShareConfig, error) {
	f.entered <- struct{}{}
	<-f.release
	return &model.ShareConfig{Type: "test", Limit: 1}, nil
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
	proxyByID *model.Proxy
	total     int64
	err       error
	filters   *repository.NodeFilter
	proxyID   uint
	sort      string
	sortOrder string
	page      int
	pageSize  int
}

func (f *fakeSubscribeProxyService) GetProxyByID(id uint) (*model.Proxy, error) {
	f.proxyID = id
	if f.err != nil {
		return nil, f.err
	}
	return f.proxyByID, nil
}

func (f *fakeSubscribeProxyService) GetProxiesByFilters(filters *repository.NodeFilter, sort string, sortOrder string, page int, pageSize int) ([]*model.Proxy, int64, error) {
	if f.err != nil {
		return nil, 0, f.err
	}
	f.filters = filters
	f.sort = sort
	f.sortOrder = sortOrder
	f.page = page
	f.pageSize = pageSize
	return f.proxies, f.total, nil
}

type fakeSubscribeGenerator struct {
	err error
}

func (g fakeSubscribeGenerator) Generate(_ []*model.Proxy) ([]byte, error) {
	if g.err != nil {
		return nil, g.err
	}
	return []byte("generated"), nil
}

func (fakeSubscribeGenerator) Format() string {
	return "test"
}

type capturingSubscribeGenerator struct {
	generated *[]*model.Proxy
}

func (g capturingSubscribeGenerator) Generate(proxies []*model.Proxy) ([]byte, error) {
	for _, proxy := range proxies {
		copy := *proxy
		*g.generated = append(*g.generated, &copy)
	}
	return []byte("generated"), nil
}

func (capturingSubscribeGenerator) Format() string { return SubscribeTypeClash }
