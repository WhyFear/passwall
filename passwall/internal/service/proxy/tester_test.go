package proxy

import (
	"context"
	"errors"
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
	assert.Nil(t, taskManager.GetStatus(task.TaskTypeSpeedTest))
}

func TestTesterCancellationStopsPendingTests(t *testing.T) {
	taskManager := task.NewTaskManager()
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	proxyRepo := &fakeTesterProxyRepo{proxies: []*model.Proxy{
		{ID: 1, Type: model.ProxyTypeVMess},
		{ID: 2, Type: model.ProxyTypeVMess},
		{ID: 3, Type: model.ProxyTypeVMess},
	}}
	historyRepo := &fakeTesterHistoryRepo{}
	tester := NewTester(
		proxyRepo,
		historyRepo,
		&fakeTesterSpeedFactory{tester: &fakeTesterSpeedTester{
			testFunc: func(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error) {
				if calls.Add(1) == 1 {
					close(started)
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-release:
					}
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

	require.Eventually(t, func() bool {
		return !taskManager.IsRunning(task.TaskTypeSpeedTest)
	}, time.Second, 10*time.Millisecond)

	assert.Nil(t, taskManager.GetStatus(task.TaskTypeSpeedTest))
	assert.Equal(t, int32(1), calls.Load())
	assert.Empty(t, proxyRepo.updated)
	assert.Empty(t, historyRepo.created)
}

func TestTesterRejectsWhenProxyWriteTaskIsActive(t *testing.T) {
	taskManager := task.NewTaskManager()
	_, started := taskManager.StartTaskWithSpec(context.Background(), task.TaskSpec{
		Type:  task.TaskTypeBanProxy,
		Total: 1,
		Accesses: []task.TaskAccess{{
			Resource: task.ResourceProxies,
			Mode:     task.AccessModeWrite,
		}},
	})
	require.True(t, started)

	tester := NewTester(
		&fakeTesterProxyRepo{proxies: []*model.Proxy{{ID: 1, Type: model.ProxyTypeVMess}}},
		&fakeTesterHistoryRepo{},
		&fakeTesterSpeedFactory{tester: &fakeTesterSpeedTester{}},
		taskManager,
	)

	err := tester.TestProxies(context.Background(), &TestRequest{Concurrent: 1}, false)

	require.Error(t, err)
	assert.True(t, errors.Is(err, task.ErrTaskConflict), "expected ErrTaskConflict, got: %v", err)
}

func TestTesterRecordsFailedAttempt(t *testing.T) {
	proxyToTest := &model.Proxy{ID: 1, Type: model.ProxyTypeVMess, Ping: 50, DownloadSpeed: 1024, UploadSpeed: 512}
	proxyRepo := &fakeTesterProxyRepo{proxies: []*model.Proxy{proxyToTest}}
	historyRepo := &fakeTesterHistoryRepo{}
	tester := NewTester(
		proxyRepo,
		historyRepo,
		&fakeTesterSpeedFactory{tester: &fakeTesterSpeedTester{
			testFunc: func(context.Context, *model.Proxy) (*model.SpeedTestResult, error) {
				return nil, errors.New("connection failed")
			},
		}},
		task.NewTaskManager(),
	)

	require.NoError(t, tester.TestProxies(context.Background(), &TestRequest{Concurrent: 1}, false))
	require.Len(t, historyRepo.created, 1)
	assert.Zero(t, historyRepo.created[0].DownloadSpeed)
	assert.Zero(t, historyRepo.created[0].UploadSpeed)
	require.Len(t, proxyRepo.updated, 1)
	assert.Equal(t, model.ProxyStatusFailed, proxyRepo.updated[0].Status)
	assert.NotNil(t, proxyRepo.updated[0].LatestTestTime)
}

func TestTesterRecordsPanickingAttempt(t *testing.T) {
	proxyRepo := &fakeTesterProxyRepo{proxies: []*model.Proxy{{ID: 1, Type: model.ProxyTypeVMess}}}
	historyRepo := &fakeTesterHistoryRepo{}
	tester := NewTester(
		proxyRepo,
		historyRepo,
		&fakeTesterSpeedFactory{tester: &fakeTesterSpeedTester{
			testFunc: func(context.Context, *model.Proxy) (*model.SpeedTestResult, error) {
				panic("speed tester panic")
			},
		}},
		task.NewTaskManager(),
	)

	require.NoError(t, tester.TestProxies(context.Background(), &TestRequest{Concurrent: 1}, false))
	require.Len(t, historyRepo.created, 1)
	assert.Zero(t, historyRepo.created[0].DownloadSpeed)
	require.Len(t, proxyRepo.updated, 1)
	assert.Equal(t, model.ProxyStatusFailed, proxyRepo.updated[0].Status)
}

func TestTesterPassesAppUnlockFilterToRepository(t *testing.T) {
	taskManager := task.NewTaskManager()
	proxyRepo := &fakeTesterProxyRepo{}
	tester := NewTester(
		proxyRepo,
		&fakeTesterHistoryRepo{},
		&fakeTesterSpeedFactory{tester: &fakeTesterSpeedTester{}},
		taskManager,
	)

	err := tester.TestProxies(context.Background(), &TestRequest{
		Filters: &repository.NodeFilter{
			Status:      []model.ProxyStatus{model.ProxyStatusOK},
			Types:       []model.ProxyType{model.ProxyTypeSS},
			CountryCode: []string{"US"},
			RiskLevel:   []string{"low"},
			AppUnlock:   []string{"Netflix", "OpenAI"},
		},
		Concurrent: 1,
	}, false)

	require.NoError(t, err)
	require.NotNil(t, proxyRepo.filter)
	assert.Equal(t, []model.ProxyStatus{model.ProxyStatusOK}, proxyRepo.filter.Status)
	assert.Equal(t, []model.ProxyType{model.ProxyTypeSS}, proxyRepo.filter.Types)
	assert.Equal(t, []string{"US"}, proxyRepo.filter.CountryCode)
	assert.Equal(t, []string{"low"}, proxyRepo.filter.RiskLevel)
	assert.Equal(t, []string{"Netflix", "OpenAI"}, proxyRepo.filter.AppUnlock)
}

type fakeTesterProxyRepo struct {
	repository.ProxyRepository
	mu      sync.Mutex
	proxies []*model.Proxy
	updated []*model.Proxy
	filter  *repository.NodeFilter
}

func (r *fakeTesterProxyRepo) FindByID(id uint) (*model.Proxy, error) {
	for _, p := range r.proxies {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}

func (r *fakeTesterProxyRepo) FindAll() ([]*model.Proxy, error) {
	return r.proxies, nil
}

func (r *fakeTesterProxyRepo) FindByFilter(filter *repository.NodeFilter) ([]*model.Proxy, error) {
	r.filter = filter
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
	mu      sync.Mutex
	created []*model.SpeedTestHistory
}

func (r *fakeTesterHistoryRepo) Create(history *model.SpeedTestHistory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.created = append(r.created, history)
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
	testFunc func(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error)
}

func (t *fakeTesterSpeedTester) Test(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error) {
	if t.testFunc != nil {
		return t.testFunc(ctx, proxy)
	}
	return &model.SpeedTestResult{Ping: 10, DownloadSpeed: 1024, UploadSpeed: 512}, nil
}

func (t *fakeTesterSpeedTester) SupportedTypes() []model.ProxyType {
	return []model.ProxyType{model.ProxyTypeVMess}
}

func TestTesterAllowsConcurrentDifferentProxyIDs(t *testing.T) {
	taskManager := task.NewTaskManager()
	started1 := make(chan struct{})
	started2 := make(chan struct{})
	release := make(chan struct{})
	done1 := make(chan struct{})
	done2 := make(chan struct{})
	var calls atomic.Int32

	tester := NewTester(
		&fakeTesterProxyRepo{proxies: []*model.Proxy{
			{ID: 1, Type: model.ProxyTypeVMess},
			{ID: 2, Type: model.ProxyTypeVMess},
		}},
		&fakeTesterHistoryRepo{},
		&fakeTesterSpeedFactory{tester: &fakeTesterSpeedTester{
			testFunc: func(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error) {
				if calls.Add(1) == 1 {
					close(started1)
				} else {
					close(started2)
				}
				<-release
				return &model.SpeedTestResult{Ping: 10, DownloadSpeed: 1024, UploadSpeed: 512}, nil
			},
		}},
		taskManager,
	)

	go func() {
		_ = tester.TestProxies(context.Background(), &TestRequest{ProxyIDs: []int64{1}, Concurrent: 1}, false)
		close(done1)
	}()
	go func() {
		_ = tester.TestProxies(context.Background(), &TestRequest{ProxyIDs: []int64{2}, Concurrent: 1}, false)
		close(done2)
	}()

	<-started1
	<-started2
	assert.Equal(t, int32(2), calls.Load())

	close(release)
	<-done1
	<-done2
}

func TestTesterRejectsSameProxyID(t *testing.T) {
	taskManager := task.NewTaskManager()
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	var calls atomic.Int32

	tester := NewTester(
		&fakeTesterProxyRepo{proxies: []*model.Proxy{
			{ID: 1, Type: model.ProxyTypeVMess},
		}},
		&fakeTesterHistoryRepo{},
		&fakeTesterSpeedFactory{tester: &fakeTesterSpeedTester{
			testFunc: func(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error) {
				if calls.Add(1) == 1 {
					close(started)
					<-release
				}
				return &model.SpeedTestResult{Ping: 10, DownloadSpeed: 1024, UploadSpeed: 512}, nil
			},
		}},
		taskManager,
	)

	go func() {
		_ = tester.TestProxies(context.Background(), &TestRequest{ProxyIDs: []int64{1}, Concurrent: 1}, false)
		close(done)
	}()

	<-started

	err := tester.TestProxies(context.Background(), &TestRequest{ProxyIDs: []int64{1}, Concurrent: 1}, false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, task.ErrTaskConflict))

	assert.Equal(t, int32(1), calls.Load())

	close(release)
	<-done
}

func TestTesterMutualExclusionGlobalVsResource(t *testing.T) {
	t.Run("global running blocks resource", func(t *testing.T) {
		taskManager := task.NewTaskManager()
		started := make(chan struct{})
		release := make(chan struct{})
		done := make(chan struct{})
		var calls atomic.Int32

		tester := NewTester(
			&fakeTesterProxyRepo{proxies: []*model.Proxy{
				{ID: 1, Type: model.ProxyTypeVMess},
				{ID: 2, Type: model.ProxyTypeVMess},
			}},
			&fakeTesterHistoryRepo{},
			&fakeTesterSpeedFactory{tester: &fakeTesterSpeedTester{
				testFunc: func(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error) {
					if calls.Add(1) == 1 {
						close(started)
						<-release
					}
					return &model.SpeedTestResult{Ping: 10, DownloadSpeed: 1024, UploadSpeed: 512}, nil
				},
			}},
			taskManager,
		)

		go func() {
			_ = tester.TestProxies(context.Background(), &TestRequest{Concurrent: 1}, false) // global (FindAll)
			close(done)
		}()

		<-started

		err := tester.TestProxies(context.Background(), &TestRequest{ProxyIDs: []int64{2}, Concurrent: 1}, false) // resource
		require.Error(t, err)
		assert.True(t, errors.Is(err, task.ErrTaskConflict))
		assert.Equal(t, int32(1), calls.Load())

		close(release)
		<-done
	})

	t.Run("resource running blocks global", func(t *testing.T) {
		taskManager := task.NewTaskManager()
		started := make(chan struct{})
		release := make(chan struct{})
		done := make(chan struct{})
		var calls atomic.Int32

		tester := NewTester(
			&fakeTesterProxyRepo{proxies: []*model.Proxy{
				{ID: 1, Type: model.ProxyTypeVMess},
				{ID: 2, Type: model.ProxyTypeVMess},
			}},
			&fakeTesterHistoryRepo{},
			&fakeTesterSpeedFactory{tester: &fakeTesterSpeedTester{
				testFunc: func(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error) {
					if calls.Add(1) == 1 {
						close(started)
						<-release
					}
					return &model.SpeedTestResult{Ping: 10, DownloadSpeed: 1024, UploadSpeed: 512}, nil
				},
			}},
			taskManager,
		)

		go func() {
			_ = tester.TestProxies(context.Background(), &TestRequest{ProxyIDs: []int64{1}, Concurrent: 1}, false) // resource
			close(done)
		}()

		<-started

		err := tester.TestProxies(context.Background(), &TestRequest{Concurrent: 1}, false) // global (FindAll)
		require.Error(t, err)
		assert.True(t, errors.Is(err, task.ErrTaskConflict))
		assert.Equal(t, int32(1), calls.Load())

		close(release)
		<-done
	})
}
