package proxy

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"passwall/config"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/task"
	"passwall/internal/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionRefresherRejectsEmptyURL(t *testing.T) {
	refresher := newSubscriptionRefresher(
		&fakeSubscriptionStatusRepository{},
		task.NewTaskManager(),
		&fakeConfigProvider{},
		nil,
		newProxySyncer(&fakeParserFactory{parser: &fakeParser{}}, &fakeProxySyncRepository{}),
		func(ctx context.Context, url string, options *util.DownloadOptions) ([]byte, error) {
			t.Fatal("download should not be called for empty URL")
			return nil, nil
		},
	)

	err := refresher.RefreshOne(context.Background(), &model.Subscription{ID: 1}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "订阅为空")
}

func TestSubscriptionRefresherSkipsNonHTTPURL(t *testing.T) {
	called := false
	refresher := newSubscriptionRefresher(
		&fakeSubscriptionStatusRepository{},
		task.NewTaskManager(),
		&fakeConfigProvider{},
		nil,
		newProxySyncer(&fakeParserFactory{parser: &fakeParser{}}, &fakeProxySyncRepository{}),
		func(ctx context.Context, url string, options *util.DownloadOptions) ([]byte, error) {
			called = true
			return nil, nil
		},
	)

	err := refresher.RefreshOne(context.Background(), &model.Subscription{ID: 1, URL: "manual"}, nil)

	require.NoError(t, err)
	assert.False(t, called)
}

func TestSubscriptionRefresherMarksInvalidOnDownloadFailure(t *testing.T) {
	subRepo := &fakeSubscriptionStatusRepository{}
	refresher := newSubscriptionRefresher(
		subRepo,
		task.NewTaskManager(),
		&fakeConfigProvider{},
		nil,
		newProxySyncer(&fakeParserFactory{parser: &fakeParser{}}, &fakeProxySyncRepository{}),
		func(ctx context.Context, url string, options *util.DownloadOptions) ([]byte, error) {
			return nil, errors.New("network failed")
		},
	)
	subscription := &model.Subscription{ID: 1, URL: "https://example.test/sub", Status: model.SubscriptionStatusPending}

	err := refresher.RefreshOne(context.Background(), subscription, nil)

	require.Error(t, err)
	assert.Equal(t, model.SubscriptionStatusInvalid, subRepo.status)
}

func TestSubscriptionRefresherRefreshAsyncKeepsFailureMessage(t *testing.T) {
	taskManager := task.NewTaskManager()
	refresher := newSubscriptionRefresher(
		&fakeSubscriptionStatusRepository{},
		taskManager,
		&fakeConfigProvider{},
		nil,
		newProxySyncer(&fakeParserFactory{parser: &fakeParser{}}, &fakeProxySyncRepository{}),
		func(ctx context.Context, url string, options *util.DownloadOptions) ([]byte, error) {
			return nil, errors.New("network failed")
		},
	)

	refresher.RefreshAsync(context.Background(), &model.Subscription{ID: 1, URL: "https://example.test/sub"}, nil)

	require.Eventually(t, func() bool {
		allStatus := taskManager.GetAllStatus()
		for _, s := range allStatus {
			if s.Type == task.TaskTypeReloadSubs && s.ResourceID == 1 && s.State == task.TaskStateFinished {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)

	allStatus := taskManager.GetAllStatus()
	var status *task.TaskStatus
	for _, s := range allStatus {
		if s.Type == task.TaskTypeReloadSubs && s.ResourceID == 1 {
			status = s
			break
		}
	}
	require.NotNil(t, status)
	assert.Contains(t, status.Error, "network failed")
}

func TestSubscriptionRefresherRefreshManyKeepsCancellationMessage(t *testing.T) {
	taskManager := task.NewTaskManager()
	refresher := newSubscriptionRefresher(
		&fakeSubscriptionStatusRepository{},
		taskManager,
		&fakeConfigProvider{},
		nil,
		newProxySyncer(&fakeParserFactory{parser: &fakeParser{}}, &fakeProxySyncRepository{}),
		func(ctx context.Context, url string, options *util.DownloadOptions) ([]byte, error) {
			t.Fatal("download should not be called after cancellation")
			return nil, nil
		},
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	refresher.RefreshMany(ctx, []*model.Subscription{{ID: 1, URL: "https://example.test/sub"}}, nil, false)

	status := taskManager.GetStatus(task.TaskTypeReloadSubs)
	require.NotNil(t, status)
	assert.Equal(t, task.TaskStateFinished, status.State)
	assert.Equal(t, "任务被取消", status.Error)
}

func TestSubscriptionRefresherCancellationStopsInFlightDownload(t *testing.T) {
	taskManager := task.NewTaskManager()
	subRepo := &fakeSubscriptionStatusRepository{}
	started := make(chan struct{})
	refresher := newSubscriptionRefresher(
		subRepo,
		taskManager,
		&fakeConfigProvider{},
		nil,
		newProxySyncer(&fakeParserFactory{parser: &fakeParser{}}, &fakeProxySyncRepository{}),
		func(ctx context.Context, url string, options *util.DownloadOptions) ([]byte, error) {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		},
	)

	refresher.RefreshMany(context.Background(), []*model.Subscription{{
		ID:     1,
		URL:    "https://example.test/sub",
		Status: model.SubscriptionStatusPending,
	}}, nil, true)

	require.Eventually(t, func() bool {
		select {
		case <-started:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	cancelled, timedOut := taskManager.CancelTask(task.TaskTypeReloadSubs, false)
	require.True(t, cancelled)
	require.False(t, timedOut)

	require.Eventually(t, func() bool {
		status := taskManager.GetStatus(task.TaskTypeReloadSubs)
		return status != nil && status.State == task.TaskStateFinished
	}, time.Second, 10*time.Millisecond)

	status := taskManager.GetStatus(task.TaskTypeReloadSubs)
	require.NotNil(t, status)
	assert.Equal(t, task.TaskCanceledMessage, status.Error)
	assert.NotEqual(t, model.SubscriptionStatusInvalid, subRepo.status)
}

func TestSubscriptionRefresherSyncsContentAndTriggersPendingTest(t *testing.T) {
	subRepo := &fakeSubscriptionStatusRepository{}
	proxyRepo := &fakeProxySyncRepository{}
	taskManager := task.NewTaskManager()
	tester := &lockingPendingTester{
		taskManager: taskManager,
		requests:    make(chan *TestRequest, 1),
		errors:      make(chan error, 1),
	}
	refresher := newSubscriptionRefresher(
		subRepo,
		taskManager,
		&fakeConfigProvider{cfg: &config.Config{Concurrent: 7}},
		tester,
		newProxySyncer(&fakeParserFactory{parser: &fakeParser{proxies: []*model.Proxy{{
			Name:     "created",
			Domain:   "created.example",
			Port:     443,
			Password: "secret",
			Type:     model.ProxyTypeTrojan,
			Config:   `{"name":"created","server":"created.example","port":443}`,
		}}}}, proxyRepo),
		func(ctx context.Context, url string, options *util.DownloadOptions) ([]byte, error) {
			require.Equal(t, "socks5://127.0.0.1:7890", options.ProxyURL)
			return []byte("subscription-content"), nil
		},
	)
	subscription := &model.Subscription{ID: 10, URL: "https://example.test/sub", Type: model.SubscriptionTypeClash}

	err := refresher.RefreshOne(context.Background(), subscription, &util.DownloadOptions{ProxyURL: "socks5://127.0.0.1:7890"})

	require.NoError(t, err)
	assert.Equal(t, model.SubscriptionStatusOK, subRepo.status)
	assert.Equal(t, "subscription-content", subRepo.content)
	require.Len(t, proxyRepo.created, 1)
	select {
	case req := <-tester.requests:
		require.NotNil(t, req.Filters)
		assert.Equal(t, []model.ProxyStatus{model.ProxyStatusPending}, req.Filters.Status)
		assert.Equal(t, 7, req.Concurrent)
	case <-time.After(time.Second):
		t.Fatal("expected pending proxy test to be triggered")
	}
	select {
	case err := <-tester.errors:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("expected pending proxy test result")
	}
}

func TestSubscriptionRefresherReturnsConflictWhenProxyWriteTaskIsActive(t *testing.T) {
	taskManager := task.NewTaskManager()
	_, started := taskManager.StartTaskWithSpec(context.Background(), task.TaskSpec{
		Type:  task.TaskTypeSpeedTest,
		Total: 1,
		Accesses: []task.TaskAccess{{
			Resource: task.ResourceProxies,
			Mode:     task.AccessModeWrite,
		}},
	})
	require.True(t, started)
	refresher := newSubscriptionRefresher(
		&fakeSubscriptionStatusRepository{},
		taskManager,
		&fakeConfigProvider{},
		nil,
		newProxySyncer(&fakeParserFactory{parser: &fakeParser{}}, &fakeProxySyncRepository{}),
		func(ctx context.Context, url string, options *util.DownloadOptions) ([]byte, error) {
			t.Fatal("download should not be called while a conflicting task is active")
			return nil, nil
		},
	)

	err := refresher.RefreshOne(context.Background(), &model.Subscription{ID: 1, URL: "https://example.test/sub"}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "存在冲突任务")
}

type fakeSubscriptionStatusRepository struct {
	repository.SubscriptionRepository
	status  model.SubscriptionStatus
	content string
}

func (r *fakeSubscriptionStatusRepository) UpdateStatus(subscription *model.Subscription) error {
	r.status = subscription.Status
	return nil
}

func (r *fakeSubscriptionStatusRepository) UpdateStatusAndContent(subscription *model.Subscription) error {
	r.status = subscription.Status
	r.content = subscription.Content
	return nil
}

type fakeConfigProvider struct {
	cfg *config.Config
	err error
}

func (f *fakeConfigProvider) GetConfig() (*config.Config, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.cfg != nil {
		return f.cfg, nil
	}
	return &config.Config{}, nil
}

type fakePendingTester struct {
	requests chan *TestRequest
}

func (f *fakePendingTester) TestProxy(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error) {
	return nil, nil
}

func (f *fakePendingTester) TestProxies(ctx context.Context, request *TestRequest, async bool) error {
	f.requests <- request
	return nil
}

type lockingPendingTester struct {
	taskManager task.TaskManager
	requests    chan *TestRequest
	errors      chan error
	calls       atomic.Int32
}

func (f *lockingPendingTester) TestProxy(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error) {
	return nil, nil
}

func (f *lockingPendingTester) TestProxies(ctx context.Context, request *TestRequest, async bool) error {
	f.calls.Add(1)
	taskRun, started := task.StartRunWithSpec(ctx, f.taskManager, task.TaskSpec{
		Type:  task.TaskTypeSpeedTest,
		Total: 1,
		Accesses: []task.TaskAccess{
			{Resource: task.ResourceProxies, Mode: task.AccessModeWrite},
			{Resource: task.ResourceSpeedHistory, Mode: task.AccessModeWrite},
		},
	})

	var err error
	if !started {
		err = task.ErrTaskConflict
	} else {
		taskRun.Finish("")
	}

	if f.requests != nil {
		f.requests <- request
	}
	if f.errors != nil {
		f.errors <- err
	}
	return err
}

func TestSubscriptionRefresherAllowsConcurrentDifferentSubIDs(t *testing.T) {
	taskManager := task.NewTaskManager()
	started1 := make(chan struct{})
	started2 := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32

	sampleProxy := &model.Proxy{Name: "test", Type: model.ProxyTypeVMess}
	refresher := newSubscriptionRefresher(
		&fakeSubscriptionStatusRepository{},
		taskManager,
		&fakeConfigProvider{},
		nil,
		newProxySyncer(&fakeParserFactory{parser: &fakeParser{proxies: []*model.Proxy{sampleProxy}}}, &fakeProxySyncRepository{}),
		func(ctx context.Context, url string, options *util.DownloadOptions) ([]byte, error) {
			if calls.Add(1) == 1 {
				close(started1)
			} else {
				close(started2)
			}
			<-release
			return []byte("content"), nil
		},
	)

	err1 := make(chan error, 1)
	err2 := make(chan error, 1)
	go func() {
		err1 <- refresher.RefreshOne(context.Background(), &model.Subscription{ID: 1, URL: "https://example.test/sub1"}, nil)
	}()
	go func() {
		err2 <- refresher.RefreshOne(context.Background(), &model.Subscription{ID: 2, URL: "https://example.test/sub2"}, nil)
	}()

	<-started1
	<-started2
	close(release)

	require.NoError(t, <-err1)
	require.NoError(t, <-err2)
	assert.Equal(t, int32(2), calls.Load())
}

func TestSubscriptionRefresherRejectsSameSubID(t *testing.T) {
	taskManager := task.NewTaskManager()
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32

	sampleProxy := &model.Proxy{Name: "test", Type: model.ProxyTypeVMess}
	refresher := newSubscriptionRefresher(
		&fakeSubscriptionStatusRepository{},
		taskManager,
		&fakeConfigProvider{},
		nil,
		newProxySyncer(&fakeParserFactory{parser: &fakeParser{proxies: []*model.Proxy{sampleProxy}}}, &fakeProxySyncRepository{}),
		func(ctx context.Context, url string, options *util.DownloadOptions) ([]byte, error) {
			if calls.Add(1) == 1 {
				close(started)
				<-release
			}
			return []byte("content"), nil
		},
	)

	done := make(chan error, 1)
	go func() {
		done <- refresher.RefreshOne(context.Background(), &model.Subscription{ID: 1, URL: "https://example.test/sub"}, nil)
	}()

	<-started

	err := refresher.RefreshOne(context.Background(), &model.Subscription{ID: 1, URL: "https://example.test/sub"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "正在刷新或存在冲突任务")

	close(release)
	require.NoError(t, <-done)
	assert.Equal(t, int32(1), calls.Load())
}

func TestSubscriptionRefresherMutualExclusionGlobalVsResource(t *testing.T) {
	sampleProxy := &model.Proxy{Name: "test", Type: model.ProxyTypeVMess}

	t.Run("global refresh blocks single sub refresh", func(t *testing.T) {
		taskManager := task.NewTaskManager()
		started := make(chan struct{})
		release := make(chan struct{})
		var calls atomic.Int32

		tester := &fakePendingTester{requests: make(chan *TestRequest, 1)}
		refresher := newSubscriptionRefresher(
			&fakeSubscriptionStatusRepository{},
			taskManager,
			&fakeConfigProvider{},
			tester,
			newProxySyncer(&fakeParserFactory{parser: &fakeParser{proxies: []*model.Proxy{sampleProxy}}}, &fakeProxySyncRepository{}),
			func(ctx context.Context, url string, options *util.DownloadOptions) ([]byte, error) {
				if calls.Add(1) == 1 {
					close(started)
					<-release
				}
				return []byte("content"), nil
			},
		)

		done := make(chan struct{})
		go func() {
			refresher.RefreshMany(context.Background(), []*model.Subscription{
				{ID: 10, URL: "https://example.test/sub1"},
			}, nil, false)
			close(done)
		}()

		<-started

		err := refresher.RefreshOne(context.Background(), &model.Subscription{ID: 2, URL: "https://example.test/sub2"}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "存在冲突任务")

		close(release)
		<-done
	})

	t.Run("single sub refresh blocks global refresh", func(t *testing.T) {
		taskManager := task.NewTaskManager()
		started := make(chan struct{})
		release := make(chan struct{})
		var calls atomic.Int32

		tester := &fakePendingTester{requests: make(chan *TestRequest, 1)}
		refresher := newSubscriptionRefresher(
			&fakeSubscriptionStatusRepository{},
			taskManager,
			&fakeConfigProvider{},
			tester,
			newProxySyncer(&fakeParserFactory{parser: &fakeParser{proxies: []*model.Proxy{sampleProxy}}}, &fakeProxySyncRepository{}),
			func(ctx context.Context, url string, options *util.DownloadOptions) ([]byte, error) {
				if calls.Add(1) == 1 {
					close(started)
					<-release
				}
				return []byte("content"), nil
			},
		)

		done := make(chan error, 1)
		go func() {
			done <- refresher.RefreshOne(context.Background(), &model.Subscription{ID: 1, URL: "https://example.test/sub"}, nil)
		}()

		<-started

		refresher.RefreshMany(context.Background(), []*model.Subscription{
			{ID: 2, URL: "https://example.test/sub2"},
		}, nil, false)
		assert.False(t, taskManager.IsRunning(task.TaskTypeReloadSubs))
		assert.Equal(t, int32(1), calls.Load())

		close(release)
		require.NoError(t, <-done)
	})
}

func TestSubscriptionRefresherRefreshManyReturnsImmediatelyWhenGlobalTaskRunning(t *testing.T) {
	taskManager := task.NewTaskManager()

	// Pre-acquire a global reload_subs task with write access
	_, started := taskManager.StartTaskWithSpec(context.Background(), task.TaskSpec{
		Type:  task.TaskTypeReloadSubs,
		Total: 1,
		Accesses: []task.TaskAccess{
			{Resource: task.ResourceSubscriptions, Mode: task.AccessModeWrite},
		},
	})
	require.True(t, started)

	sampleProxy := &model.Proxy{Name: "test", Type: model.ProxyTypeVMess}
	downloadCalled := false
	tester := &fakePendingTester{requests: make(chan *TestRequest, 1)}
	refresher := newSubscriptionRefresher(
		&fakeSubscriptionStatusRepository{},
		taskManager,
		&fakeConfigProvider{},
		tester,
		newProxySyncer(&fakeParserFactory{parser: &fakeParser{proxies: []*model.Proxy{sampleProxy}}}, &fakeProxySyncRepository{}),
		func(ctx context.Context, url string, options *util.DownloadOptions) ([]byte, error) {
			downloadCalled = true
			return []byte("content"), nil
		},
	)

	assert.NotPanics(t, func() {
		refresher.RefreshMany(context.Background(), []*model.Subscription{
			{ID: 1, URL: "https://example.test/sub"},
		}, nil, false)
	})
	assert.False(t, downloadCalled, "download should not be called when task lock is unavailable")
}

func TestSubscriptionRefresherRefreshManyTriggersPendingTestAfterGlobalTaskFinished(t *testing.T) {
	taskManager := task.NewTaskManager()
	tester := &lockingPendingTester{
		taskManager: taskManager,
		requests:    make(chan *TestRequest, 2),
		errors:      make(chan error, 2),
	}
	sampleProxy := &model.Proxy{Name: "test", Type: model.ProxyTypeVMess}
	refresher := newSubscriptionRefresher(
		&fakeSubscriptionStatusRepository{},
		taskManager,
		&fakeConfigProvider{cfg: &config.Config{Concurrent: 3}},
		tester,
		newProxySyncer(&fakeParserFactory{parser: &fakeParser{proxies: []*model.Proxy{sampleProxy}}}, &fakeProxySyncRepository{}),
		func(ctx context.Context, url string, options *util.DownloadOptions) ([]byte, error) {
			return []byte("content"), nil
		},
	)

	refresher.RefreshMany(context.Background(), []*model.Subscription{
		{ID: 1, URL: "https://example.test/sub1"},
		{ID: 2, URL: "https://example.test/sub2"},
	}, nil, false)

	select {
	case req := <-tester.requests:
		require.NotNil(t, req.Filters)
		assert.Equal(t, []model.ProxyStatus{model.ProxyStatusPending}, req.Filters.Status)
		assert.Equal(t, 3, req.Concurrent)
	case <-time.After(time.Second):
		t.Fatal("expected pending proxy test to be triggered")
	}
	select {
	case err := <-tester.errors:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("expected pending proxy test result")
	}
	assert.Equal(t, int32(1), tester.calls.Load())

	select {
	case <-tester.requests:
		t.Fatal("pending proxy test should be triggered only once for refresh many")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSubscriptionRefresherRefreshOneRecoversFromPanic(t *testing.T) {
	taskManager := task.NewTaskManager()
	refresher := newSubscriptionRefresher(
		&fakeSubscriptionStatusRepository{},
		taskManager,
		&fakeConfigProvider{},
		nil,
		newProxySyncer(&fakeParserFactory{parser: &fakeParser{}}, &fakeProxySyncRepository{}),
		func(ctx context.Context, url string, options *util.DownloadOptions) ([]byte, error) {
			panic("download exploded")
		},
	)

	err := refresher.RefreshOne(context.Background(),
		&model.Subscription{ID: 1, URL: "https://example.test/sub"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "panic")

	allStatus := taskManager.GetAllStatus()
	var taskStatus *task.TaskStatus
	for _, s := range allStatus {
		if s.Type == task.TaskTypeReloadSubs && s.ResourceID == 1 {
			taskStatus = s
			break
		}
	}
	require.NotNil(t, taskStatus)
	assert.Equal(t, task.TaskStateFinished, taskStatus.State)
	assert.Contains(t, taskStatus.Error, "panic")

	// A new refresh can start — no leftover Running task blocking it
	err = refresher.RefreshOne(context.Background(),
		&model.Subscription{ID: 1, URL: "manual"}, nil)
	require.NoError(t, err, "second refresh should succeed without conflict")
}

func TestSubscriptionRefresherRefreshOneUpdatesProgress(t *testing.T) {
	taskManager := task.NewTaskManager()
	sampleProxy := &model.Proxy{Name: "test", Type: model.ProxyTypeVMess}
	refresher := newSubscriptionRefresher(
		&fakeSubscriptionStatusRepository{},
		taskManager,
		&fakeConfigProvider{},
		nil,
		newProxySyncer(&fakeParserFactory{parser: &fakeParser{proxies: []*model.Proxy{sampleProxy}}}, &fakeProxySyncRepository{}),
		func(ctx context.Context, url string, options *util.DownloadOptions) ([]byte, error) {
			return []byte("content"), nil
		},
	)

	err := refresher.RefreshOne(context.Background(),
		&model.Subscription{ID: 1, URL: "https://example.test/sub"}, nil)
	require.NoError(t, err)

	allStatus := taskManager.GetAllStatus()
	var taskStatus *task.TaskStatus
	for _, s := range allStatus {
		if s.Type == task.TaskTypeReloadSubs && s.ResourceID == 1 {
			taskStatus = s
			break
		}
	}
	require.NotNil(t, taskStatus)
	assert.Equal(t, 1, taskStatus.Completed)
	assert.Equal(t, 1, taskStatus.Total)
	assert.Equal(t, 100, taskStatus.Progress)
	assert.Equal(t, task.TaskStateFinished, taskStatus.State)
}
