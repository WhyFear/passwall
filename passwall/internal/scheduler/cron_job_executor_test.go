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
	proxyService := &fakeCronProxyService{proxyIDs: []uint{7, 9}}
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

func TestCronJobExecutorDoesNotUseGlobalTaskSkip(t *testing.T) {
	taskManager := task.NewTaskManager()
	_, started := taskManager.StartTask(context.Background(), task.TaskTypeSpeedTest, 1)
	require.True(t, started)
	proxyTester := &fakeCronProxyTester{}
	executor := newCronJobExecutor(taskManager, proxyTester, &fakeCronProxyService{}, &fakeCronIPDetector{})

	executor.Execute(config.CronJob{
		Name:      "skip",
		TestProxy: config.TestProxyConfig{Enable: true},
	})

	require.NotNil(t, proxyTester.request)
}

func TestCronJobPanicDoesNotFinishUnrelatedTask(t *testing.T) {
	taskManager := task.NewTaskManager()
	run, started := task.StartRun(context.Background(), taskManager, task.TaskTypeSpeedTest, 1)
	require.True(t, started)
	proxyTester := &fakeCronProxyTester{panicOnTest: true}
	executor := newCronJobExecutor(taskManager, proxyTester, &fakeCronProxyService{}, &fakeCronIPDetector{})

	executor.Execute(config.CronJob{
		Name:      "panic",
		TestProxy: config.TestProxyConfig{Enable: true},
	})

	assert.True(t, taskManager.IsRunning(task.TaskTypeSpeedTest))
	run.Finish("")
}

func TestCronJobIPConflictStopsBeforeCompletionWebhook(t *testing.T) {
	executor := newCronJobExecutor(
		task.NewTaskManager(),
		&fakeCronProxyTester{},
		&fakeCronProxyService{proxyIDs: []uint{7}},
		&fakeCronIPDetector{err: task.ErrTaskConflict},
	)

	continueJob := executor.executeIPCheck(config.CronJob{
		Name:    "conflict",
		IPCheck: config.IPCheckConfig{Enable: true},
	})

	assert.False(t, continueJob)
}

func TestCronIPCheckUsesKeysetBatches(t *testing.T) {
	proxyIDs := make([]uint, schedulerBatchSize+1)
	for index := range proxyIDs {
		proxyIDs[index] = uint(index + 1)
	}
	proxyService := &fakeCronProxyService{proxyIDs: proxyIDs}
	ipDetector := &fakeCronIPDetector{}
	executor := newCronJobExecutor(task.NewTaskManager(), &fakeCronProxyTester{}, proxyService, ipDetector)

	continueJob := executor.executeIPCheck(config.CronJob{
		Name:    "batch",
		IPCheck: config.IPCheckConfig{Enable: true, Concurrent: 3},
	})

	assert.True(t, continueJob)
	assert.Equal(t, []proxyCursorCall{{0, schedulerBatchSize}, {schedulerBatchSize, schedulerBatchSize}}, proxyService.cursorCalls)
	require.Len(t, ipDetector.reqs, 2)
	assert.Len(t, ipDetector.reqs[0].ProxyIDList, schedulerBatchSize)
	assert.Equal(t, []uint{schedulerBatchSize + 1}, ipDetector.reqs[1].ProxyIDList)
}

func TestBuildProxyFilterIgnoresInvalidStatuses(t *testing.T) {
	filter := buildProxyFilter("bad,1,2")

	require.NotNil(t, filter)
	assert.Equal(t, []model.ProxyStatus{model.ProxyStatusOK, model.ProxyStatusFailed}, filter.Status)
	assert.Nil(t, buildProxyFilter("bad"))
}

type fakeCronProxyTester struct {
	request     *proxy.TestRequest
	panicOnTest bool
}

func (f *fakeCronProxyTester) TestProxy(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error) {
	return nil, nil
}

func (f *fakeCronProxyTester) TestProxies(ctx context.Context, request *proxy.TestRequest, async bool) error {
	if f.panicOnTest {
		panic("test panic")
	}
	f.request = request
	return nil
}

type fakeCronProxyService struct {
	proxy.ProxyService
	proxyIDs    []uint
	cursorCalls []proxyCursorCall
	banReq      proxy.BanProxyReq
}

type proxyCursorCall struct {
	afterID uint
	limit   int
}

func (f *fakeCronProxyService) BanProxy(ctx context.Context, req proxy.BanProxyReq) error {
	f.banReq = req
	return nil
}

func (f *fakeCronProxyService) GetProxyIDsAfter(afterID uint, limit int) ([]uint, error) {
	f.cursorCalls = append(f.cursorCalls, proxyCursorCall{afterID, limit})
	result := make([]uint, 0, limit)
	for _, id := range f.proxyIDs {
		if id > afterID {
			result = append(result, id)
			if len(result) == limit {
				break
			}
		}
	}
	return result, nil
}

type fakeCronIPDetector struct {
	service.IPDetectorService
	req  *service.BatchIPDetectorReq
	reqs []*service.BatchIPDetectorReq
	err  error
}

func (f *fakeCronIPDetector) BatchDetect(ctx context.Context, req *service.BatchIPDetectorReq) error {
	f.req = req
	f.reqs = append(f.reqs, req)
	return f.err
}
