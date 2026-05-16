package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"passwall/internal/model"
	proxyservice "passwall/internal/service/proxy"
	"passwall/internal/service/task"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyPassesAppUnlockFilterToTester(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tester := &fakeWebProxyTester{}
	ctx := context.WithValue(context.Background(), "concurrent", 3)
	router := gin.New()
	router.POST("/test_proxy_server", TestProxy(ctx, tester))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/test_proxy_server",
		bytes.NewBufferString(`{"status":"1","type":"ss","country_code":"US","risk_level":"low","app_unlock":"Netflix,OpenAI"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, tester.request)
	require.NotNil(t, tester.request.Filters)
	assert.Equal(t, 3, tester.request.Concurrent)
	assert.Equal(t, []model.ProxyStatus{model.ProxyStatusOK}, tester.request.Filters.Status)
	assert.Equal(t, []model.ProxyType{model.ProxyTypeSS}, tester.request.Filters.Types)
	assert.Equal(t, []string{"US"}, tester.request.Filters.CountryCode)
	assert.Equal(t, []string{"low"}, tester.request.Filters.RiskLevel)
	assert.Equal(t, []string{"Netflix", "OpenAI"}, tester.request.Filters.AppUnlock)
}

func TestProxySingleNodeKeepsProxyIDWhenAppUnlockIsPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tester := &fakeWebProxyTester{}
	ctx := context.WithValue(context.Background(), "concurrent", 3)
	router := gin.New()
	router.POST("/test_proxy_server", TestProxy(ctx, tester))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/test_proxy_server",
		bytes.NewBufferString(`{"id":7,"app_unlock":"Netflix"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, tester.request)
	assert.Equal(t, []int64{7}, tester.request.ProxyIDs)
	assert.Equal(t, []string{"Netflix"}, tester.request.Filters.AppUnlock)
}

func TestProxyRejectsInvalidStatusFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.POST("/test_proxy_server", TestProxy(context.WithValue(context.Background(), "concurrent", 3), &fakeWebProxyTester{}))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/test_proxy_server",
		bytes.NewBufferString(`{"status":"bad","app_unlock":"Netflix"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestProxyReportsTaskConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tester := &fakeWebProxyTester{err: task.ErrTaskConflict}
	router := gin.New()
	router.POST("/test_proxy_server", TestProxy(context.WithValue(context.Background(), "concurrent", 3), tester))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/test_proxy_server",
		bytes.NewBufferString(`{"app_unlock":"Netflix"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestProxyReportsTesterError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tester := &fakeWebProxyTester{err: errors.New("boom")}
	router := gin.New()
	router.POST("/test_proxy_server", TestProxy(context.WithValue(context.Background(), "concurrent", 3), tester))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/test_proxy_server",
		bytes.NewBufferString(`{"app_unlock":"Netflix"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

type fakeWebProxyTester struct {
	request *proxyservice.TestRequest
	async   bool
	err     error
}

func (f *fakeWebProxyTester) TestProxy(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error) {
	return nil, nil
}

func (f *fakeWebProxyTester) TestProxies(ctx context.Context, request *proxyservice.TestRequest, async bool) error {
	f.request = request
	f.async = async
	return f.err
}
