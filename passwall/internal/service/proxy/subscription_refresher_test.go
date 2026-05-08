package proxy

import (
	"context"
	"errors"
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
		func(url string, options *util.DownloadOptions) ([]byte, error) {
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
		func(url string, options *util.DownloadOptions) ([]byte, error) {
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
		func(url string, options *util.DownloadOptions) ([]byte, error) {
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
		func(url string, options *util.DownloadOptions) ([]byte, error) {
			return nil, errors.New("network failed")
		},
	)

	refresher.RefreshAsync(context.Background(), &model.Subscription{ID: 1, URL: "https://example.test/sub"}, nil)

	require.Eventually(t, func() bool {
		status := taskManager.GetStatus(task.TaskTypeReloadSubs)
		return status != nil && status.State == task.TaskStateFinished
	}, time.Second, 10*time.Millisecond)

	status := taskManager.GetStatus(task.TaskTypeReloadSubs)
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
		func(url string, options *util.DownloadOptions) ([]byte, error) {
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

func TestSubscriptionRefresherSyncsContentAndTriggersPendingTest(t *testing.T) {
	subRepo := &fakeSubscriptionStatusRepository{}
	proxyRepo := &fakeProxySyncRepository{}
	tester := &fakePendingTester{requests: make(chan *TestRequest, 1)}
	refresher := newSubscriptionRefresher(
		subRepo,
		task.NewTaskManager(),
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
		func(url string, options *util.DownloadOptions) ([]byte, error) {
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

func (f *fakePendingTester) TestProxy(proxy *model.Proxy) (*model.SpeedTestResult, error) {
	return nil, nil
}

func (f *fakePendingTester) TestProxies(ctx context.Context, request *TestRequest, async bool) error {
	f.requests <- request
	return nil
}
