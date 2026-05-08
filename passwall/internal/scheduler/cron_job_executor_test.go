package scheduler

import (
	"context"
	"testing"

	"passwall/config"
	"passwall/internal/model"
	"passwall/internal/service"
	"passwall/internal/service/proxy"
	"passwall/internal/service/task"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCronJobExecutorRunsConfiguredSteps(t *testing.T) {
	taskManager := task.NewTaskManager()
	proxyTester := &fakeCronProxyTester{}
	proxyService := &fakeCronProxyService{proxies: []*model.Proxy{{ID: 7}, {ID: 9}}}
	ipDetector := &fakeCronIPDetector{}
	executor := newCronJobExecutor(taskManager, proxyTester, proxyService, ipDetector)

	executor.Execute(config.CronJob{
		Name: "all",
		TestProxy: config.TestProxyConfig{
			Enable: true,
			Status: "1,2",
		},
		AutoBan: config.BanProxyConfig{
			Enable: true,
		},
		IPCheck: config.IPCheckConfig{
			Enable:     true,
			Concurrent: 3,
			IPInfo:     config.IPInfoConfig{Enable: true},
			AppUnlock:  config.AppUnlockConfig{Enable: true},
			Refresh:    true,
		},
	})

	require.NotNil(t, proxyTester.request)
	require.NotNil(t, proxyTester.request.Filters)
	assert.Equal(t, []model.ProxyStatus{model.ProxyStatusOK, model.ProxyStatusFailed}, proxyTester.request.Filters.Status)
	assert.Equal(t, 5, proxyTester.request.Concurrent)
	assert.Equal(t, 5, proxyService.banReq.TestTimes)
	require.NotNil(t, ipDetector.req)
	assert.Equal(t, []uint{7, 9}, ipDetector.req.ProxyIDList)
	assert.True(t, ipDetector.req.IPInfoEnable)
	assert.True(t, ipDetector.req.APPUnlockEnable)
	assert.True(t, ipDetector.req.Refresh)
	assert.Equal(t, 3, ipDetector.req.Concurrent)
}

func TestCronJobExecutorSkipsWhenAnotherTaskIsRunning(t *testing.T) {
	taskManager := task.NewTaskManager()
	_, started := taskManager.StartTask(context.Background(), task.TaskTypeSpeedTest, 1)
	require.True(t, started)
	proxyTester := &fakeCronProxyTester{}
	executor := newCronJobExecutor(taskManager, proxyTester, &fakeCronProxyService{}, &fakeCronIPDetector{})

	executor.Execute(config.CronJob{
		Name:      "skip",
		TestProxy: config.TestProxyConfig{Enable: true},
	})

	assert.Nil(t, proxyTester.request)
}

func TestBuildProxyFilterIgnoresInvalidStatuses(t *testing.T) {
	filter := buildProxyFilter("bad,1,2")

	require.NotNil(t, filter)
	assert.Equal(t, []model.ProxyStatus{model.ProxyStatusOK, model.ProxyStatusFailed}, filter.Status)
	assert.Nil(t, buildProxyFilter("bad"))
}

type fakeCronProxyTester struct {
	request *proxy.TestRequest
}

func (f *fakeCronProxyTester) TestProxy(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error) {
	return nil, nil
}

func (f *fakeCronProxyTester) TestProxies(ctx context.Context, request *proxy.TestRequest, async bool) error {
	f.request = request
	return nil
}

type fakeCronProxyService struct {
	proxy.ProxyService
	proxies []*model.Proxy
	banReq  proxy.BanProxyReq
}

func (f *fakeCronProxyService) BanProxy(ctx context.Context, req proxy.BanProxyReq) error {
	f.banReq = req
	return nil
}

func (f *fakeCronProxyService) GetProxiesByFilters(filters map[string]interface{}, sort string, sortOrder string, page int, pageSize int) ([]*model.Proxy, int64, error) {
	return f.proxies, int64(len(f.proxies)), nil
}

type fakeCronIPDetector struct {
	service.IPDetectorService
	req *service.BatchIPDetectorReq
}

func (f *fakeCronIPDetector) BatchDetect(ctx context.Context, req *service.BatchIPDetectorReq) error {
	f.req = req
	return nil
}
