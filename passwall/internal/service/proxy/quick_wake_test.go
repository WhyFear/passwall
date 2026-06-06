package proxy

import (
	"context"
	"errors"
	"sync"
	"testing"

	"passwall/internal/adapter/speedtester"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/task"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuickWakeWakesSuccessfulBannedProxies(t *testing.T) {
	taskManager := task.NewTaskManager()
	proxyRepo := &fakeQuickWakeProxyRepo{proxies: []*model.Proxy{
		{ID: 1, Type: model.ProxyTypeSS, Status: model.ProxyStatusBanned},
		{ID: 2, Type: model.ProxyTypeSS, Status: model.ProxyStatusBanned},
	}}
	service := NewQuickWakeService(
		proxyRepo,
		&fakeQuickWakeSpeedFactory{tester: &fakeQuickWakeSpeedTester{
			latencyFunc: func(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error) {
				if proxy.ID == 1 {
					return &model.SpeedTestResult{Ping: 10}, nil
				}
				return &model.SpeedTestResult{Ping: 0, Error: errors.New("probe failed")}, nil
			},
		}},
		taskManager,
	)

	err := service.WakeBannedProxies(context.Background(), QuickWakeRequest{Concurrent: 2}, false)

	require.NoError(t, err)
	assert.Equal(t, []model.ProxyStatus{model.ProxyStatusBanned}, proxyRepo.queryStatuses)
	require.Len(t, proxyRepo.updated, 1)
	assert.Equal(t, uint(1), proxyRepo.updated[0].ID)
	assert.Equal(t, model.ProxyStatusPending, proxyRepo.updated[0].Status)

	status := taskManager.GetStatus(task.TaskTypeQuickWake)
	require.NotNil(t, status)
	assert.Equal(t, task.TaskStateFinished, status.State)
	assert.Equal(t, 2, status.Completed)
	assert.Equal(t, 100, status.Progress)
}

func TestQuickWakePassesTypeFilter(t *testing.T) {
	proxyRepo := &fakeQuickWakeProxyRepo{proxies: []*model.Proxy{
		{ID: 1, Type: model.ProxyTypeSS, Status: model.ProxyStatusBanned},
	}}
	service := NewQuickWakeService(
		proxyRepo,
		&fakeQuickWakeSpeedFactory{tester: &fakeQuickWakeSpeedTester{}},
		task.NewTaskManager(),
	)

	err := service.WakeBannedProxies(context.Background(), QuickWakeRequest{
		Types:      []model.ProxyType{model.ProxyTypeSS, model.ProxyTypeTrojan},
		Concurrent: 1,
	}, false)

	require.NoError(t, err)
	assert.Equal(t, []model.ProxyType{model.ProxyTypeSS, model.ProxyTypeTrojan}, proxyRepo.queryTypes)
}

func TestQuickWakeRejectsConflictingProxyWriteTask(t *testing.T) {
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

	service := NewQuickWakeService(
		&fakeQuickWakeProxyRepo{proxies: []*model.Proxy{{ID: 1, Type: model.ProxyTypeSS, Status: model.ProxyStatusBanned}}},
		&fakeQuickWakeSpeedFactory{tester: &fakeQuickWakeSpeedTester{}},
		taskManager,
	)

	err := service.WakeBannedProxies(context.Background(), QuickWakeRequest{Concurrent: 1}, false)

	require.Error(t, err)
	assert.True(t, errors.Is(err, task.ErrTaskConflict))
}

type fakeQuickWakeProxyRepo struct {
	repository.ProxyRepository
	mu            sync.Mutex
	proxies       []*model.Proxy
	updated       []*model.Proxy
	queryStatuses []model.ProxyStatus
	queryTypes    []model.ProxyType
}

func (r *fakeQuickWakeProxyRepo) FindByStatusAndTypesIncludingBanned(statuses []model.ProxyStatus, types []model.ProxyType) ([]*model.Proxy, error) {
	r.queryStatuses = append([]model.ProxyStatus(nil), statuses...)
	r.queryTypes = append([]model.ProxyType(nil), types...)

	statusSet := make(map[model.ProxyStatus]bool, len(statuses))
	for _, status := range statuses {
		statusSet[status] = true
	}
	typeSet := make(map[model.ProxyType]bool, len(types))
	for _, proxyType := range types {
		typeSet[proxyType] = true
	}

	result := make([]*model.Proxy, 0, len(r.proxies))
	for _, proxy := range r.proxies {
		if len(statusSet) > 0 && !statusSet[proxy.Status] {
			continue
		}
		if len(typeSet) > 0 && !typeSet[proxy.Type] {
			continue
		}
		result = append(result, proxy)
	}
	return result, nil
}

func (r *fakeQuickWakeProxyRepo) UpdateProxyStatus(proxy *model.Proxy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copyProxy := *proxy
	r.updated = append(r.updated, &copyProxy)
	return nil
}

type fakeQuickWakeSpeedFactory struct {
	speedtester.SpeedTesterFactory
	tester speedtester.SpeedTester
}

func (f *fakeQuickWakeSpeedFactory) GetSpeedTester(proxyType model.ProxyType) (speedtester.SpeedTester, error) {
	return f.tester, nil
}

type fakeQuickWakeSpeedTester struct {
	speedtester.SpeedTester
	latencyFunc func(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error)
}

func (t *fakeQuickWakeSpeedTester) Test(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error) {
	return &model.SpeedTestResult{Ping: 10, DownloadSpeed: 1024, UploadSpeed: 512}, nil
}

func (t *fakeQuickWakeSpeedTester) TestLatency(ctx context.Context, proxy *model.Proxy) (*model.SpeedTestResult, error) {
	if t.latencyFunc != nil {
		return t.latencyFunc(ctx, proxy)
	}
	return &model.SpeedTestResult{Ping: 10}, nil
}

func (t *fakeQuickWakeSpeedTester) SupportedTypes() []model.ProxyType {
	return []model.ProxyType{model.ProxyTypeSS}
}
