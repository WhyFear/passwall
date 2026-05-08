package proxy

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"passwall/internal/adapter/speedtester"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/task"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTesterTracksProgress(t *testing.T) {
	taskManager := task.NewTaskManager()
	tester := NewTester(
		&fakeTesterProxyRepo{proxies: []*model.Proxy{
			{ID: 1, Type: model.ProxyTypeVMess},
			{ID: 2, Type: model.ProxyTypeVMess},
		}},
		&fakeTesterHistoryRepo{},
		&fakeTesterSpeedFactory{tester: &fakeTesterSpeedTester{}},
		taskManager,
	)

	err := tester.TestProxies(context.Background(), &TestRequest{Concurrent: 2}, false)

	require.NoError(t, err)
	status := taskManager.GetStatus(task.TaskTypeSpeedTest)
	require.NotNil(t, status)
	assert.Equal(t, task.TaskStateFinished, status.State)
	assert.Equal(t, 2, status.Completed)
	assert.Equal(t, 100, status.Progress)
	assert.Empty(t, status.Error)
}

func TestTesterCancellationStopsPendingTests(t *testing.T) {
	taskManager := task.NewTaskManager()
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	tester := NewTester(
		&fakeTesterProxyRepo{proxies: []*model.Proxy{
			{ID: 1, Type: model.ProxyTypeVMess},
			{ID: 2, Type: model.ProxyTypeVMess},
			{ID: 3, Type: model.ProxyTypeVMess},
		}},
		&fakeTesterHistoryRepo{},
		&fakeTesterSpeedFactory{tester: &fakeTesterSpeedTester{
			testFunc: func(proxy *model.Proxy) (*model.SpeedTestResult, error) {
				if calls.Add(1) == 1 {
					close(started)
					<-release
				}
				return &model.SpeedTestResult{Ping: 10, DownloadSpeed: 1024, UploadSpeed: 512}, nil
			},
		}},
		taskManager,
	)

	err := tester.TestProxies(context.Background(), &TestRequest{Concurrent: 1}, true)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		select {
		case <-started:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	cancelled, timedOut := taskManager.CancelTask(task.TaskTypeSpeedTest, false)
	require.True(t, cancelled)
	require.False(t, timedOut)
	close(release)

	require.Eventually(t, func() bool {
		status := taskManager.GetStatus(task.TaskTypeSpeedTest)
		return status != nil && status.State == task.TaskStateFinished
	}, time.Second, 10*time.Millisecond)

	status := taskManager.GetStatus(task.TaskTypeSpeedTest)
	require.NotNil(t, status)
	assert.Equal(t, task.TaskCanceledMessage, status.Error)
	assert.Equal(t, 1, status.Completed)
	assert.Equal(t, int32(1), calls.Load())
}

type fakeTesterProxyRepo struct {
	repository.ProxyRepository
	mu      sync.Mutex
	proxies []*model.Proxy
	updated []*model.Proxy
}

func (r *fakeTesterProxyRepo) FindAll() ([]*model.Proxy, error) {
	return r.proxies, nil
}

func (r *fakeTesterProxyRepo) UpdateSpeedTestInfo(proxy *model.Proxy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updated = append(r.updated, proxy)
	return nil
}

type fakeTesterHistoryRepo struct {
	repository.SpeedTestHistoryRepository
}

func (r *fakeTesterHistoryRepo) Create(history *model.SpeedTestHistory) error {
	return nil
}

type fakeTesterSpeedFactory struct {
	speedtester.SpeedTesterFactory
	tester speedtester.SpeedTester
}

func (f *fakeTesterSpeedFactory) GetSpeedTester(proxyType model.ProxyType) (speedtester.SpeedTester, error) {
	return f.tester, nil
}

type fakeTesterSpeedTester struct {
	speedtester.SpeedTester
	testFunc func(proxy *model.Proxy) (*model.SpeedTestResult, error)
}

func (t *fakeTesterSpeedTester) Test(proxy *model.Proxy) (*model.SpeedTestResult, error) {
	if t.testFunc != nil {
		return t.testFunc(proxy)
	}
	return &model.SpeedTestResult{Ping: 10, DownloadSpeed: 1024, UploadSpeed: 512}, nil
}

func (t *fakeTesterSpeedTester) SupportedTypes() []model.ProxyType {
	return []model.ProxyType{model.ProxyTypeVMess}
}
