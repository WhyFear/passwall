package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"passwall/internal/detector"
	"passwall/internal/detector/ipbaseinfo"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/task"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIPDetectorBatchDetectTracksProgress(t *testing.T) {
	taskManager := task.NewTaskManager()
	var detected []uint
	var mu sync.Mutex
	detectorService := ipDetectorImpl{
		TaskManager: taskManager,
		detectOne: func(ctx context.Context, req *IPDetectorReq) error {
			mu.Lock()
			defer mu.Unlock()
			detected = append(detected, req.ProxyID)
			return nil
		},
	}

	err := detectorService.BatchDetect(context.Background(), &BatchIPDetectorReq{
		ProxyIDList:     []uint{1, 2, 3},
		Enabled:         true,
		IPInfoEnable:    true,
		APPUnlockEnable: true,
		Refresh:         true,
		Concurrent:      2,
	})

	require.NoError(t, err)
	assert.ElementsMatch(t, []uint{1, 2, 3}, detected)
	status := taskManager.GetStatus(task.TaskTypeCheckIp)
	require.NotNil(t, status)
	assert.Equal(t, task.TaskStateFinished, status.State)
	assert.Equal(t, 3, status.Completed)
	assert.Equal(t, 100, status.Progress)
	assert.Equal(t, "batch detect proxy ip finished", status.Error)
}

func TestIPDetectorBatchDetectCancellationStopsPendingDetects(t *testing.T) {
	taskManager := task.NewTaskManager()
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	var calls atomic.Int32
	detectorService := ipDetectorImpl{
		TaskManager: taskManager,
		detectOne: func(ctx context.Context, req *IPDetectorReq) error {
			if calls.Add(1) == 1 {
				close(started)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-release:
				}
			}
			return nil
		},
	}

	go func() {
		done <- detectorService.BatchDetect(context.Background(), &BatchIPDetectorReq{
			ProxyIDList: []uint{1, 2, 3},
			Enabled:     true,
			Concurrent:  1,
		})
	}()

	require.Eventually(t, func() bool {
		select {
		case <-started:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	cancelled, timedOut := taskManager.CancelTask(task.TaskTypeCheckIp, false)
	require.True(t, cancelled)
	require.False(t, timedOut)

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("batch detect did not finish after cancellation")
	}

	status := taskManager.GetStatus(task.TaskTypeCheckIp)
	require.NotNil(t, status)
	assert.Equal(t, task.TaskStateFinished, status.State)
	assert.Equal(t, task.TaskCanceledMessage, status.Error)
	assert.Equal(t, int32(1), calls.Load())
}

func TestIPDetectorBatchDetectUsesDefaultConcurrency(t *testing.T) {
	taskManager := task.NewTaskManager()
	detectorService := ipDetectorImpl{
		TaskManager: taskManager,
		detectOne: func(ctx context.Context, req *IPDetectorReq) error {
			return nil
		},
	}
	req := &BatchIPDetectorReq{
		ProxyIDList: []uint{1},
		Enabled:     true,
	}

	err := detectorService.BatchDetect(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, 20, req.Concurrent)
}

func TestIPDetectorBatchDetectSkipsWhenProxyWriteTaskIsActive(t *testing.T) {
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
	called := false
	detectorService := ipDetectorImpl{
		TaskManager: taskManager,
		detectOne: func(ctx context.Context, req *IPDetectorReq) error {
			called = true
			return nil
		},
	}

	err := detectorService.BatchDetect(context.Background(), &BatchIPDetectorReq{
		ProxyIDList: []uint{1},
		Enabled:     true,
	})

	require.NoError(t, err)
	assert.False(t, called)
	assert.Nil(t, taskManager.GetStatus(task.TaskTypeCheckIp))
}

func TestIPDetectorDetectRefreshFalseSkipsWhenProxyAlreadyHasIPRecord(t *testing.T) {
	detectorService := newDetectTestService()
	detectorService.ProxyIPAddress = &fakeDetectProxyIPRepo{
		records: []*model.ProxyIPAddress{{ProxyID: 1, IPAddressesID: 10, IPType: 4}},
	}
	called := false
	detectorService.detectAll = func(ctx context.Context, proxy *model.IPProxy, ipInfoEnable bool, appUnlockEnable bool) (*detector.DetectionResult, error) {
		called = true
		return nil, nil
	}

	err := detectorService.Detect(context.Background(), &IPDetectorReq{
		ProxyID: 1,
		Enabled: true,
		Refresh: false,
	})

	require.NoError(t, err)
	assert.False(t, called)
}

func TestIPDetectorDetectRefreshFalseReusesKnownIPv4Address(t *testing.T) {
	proxyIPRepo := &fakeDetectProxyIPRepo{}
	addressRepo := &fakeDetectAddressRepo{
		byIP: map[string]*model.IPAddress{
			"203.0.113.10": {ID: 5, IP: "203.0.113.10", IPType: 4},
		},
	}
	detectorService := newDetectTestService()
	detectorService.ProxyIPAddress = proxyIPRepo
	detectorService.IPAddressRepo = addressRepo
	detectorService.detectAll = func(ctx context.Context, proxy *model.IPProxy, ipInfoEnable bool, appUnlockEnable bool) (*detector.DetectionResult, error) {
		assert.False(t, ipInfoEnable)
		assert.False(t, appUnlockEnable)
		return &detector.DetectionResult{
			BaseInfo: &ipbaseinfo.IPBaseInfo{IPV4: "203.0.113.10"},
		}, nil
	}

	err := detectorService.Detect(context.Background(), &IPDetectorReq{
		ProxyID: 1,
		Enabled: true,
		Refresh: false,
	})

	require.NoError(t, err)
	require.Len(t, proxyIPRepo.saved, 1)
	assert.Equal(t, uint(1), proxyIPRepo.saved[0].ProxyID)
	assert.Equal(t, uint(5), proxyIPRepo.saved[0].IPAddressesID)
	assert.Equal(t, uint(4), proxyIPRepo.saved[0].IPType)
}

func TestIPDetectorDetectRefreshFalseFallsBackToKnownIPv6Address(t *testing.T) {
	proxyIPRepo := &fakeDetectProxyIPRepo{}
	addressRepo := &fakeDetectAddressRepo{
		byIP: map[string]*model.IPAddress{
			"2001:db8::10": {ID: 6, IP: "2001:db8::10", IPType: 6},
		},
	}
	detectorService := newDetectTestService()
	detectorService.ProxyIPAddress = proxyIPRepo
	detectorService.IPAddressRepo = addressRepo
	detectorService.detectAll = func(ctx context.Context, proxy *model.IPProxy, ipInfoEnable bool, appUnlockEnable bool) (*detector.DetectionResult, error) {
		return &detector.DetectionResult{
			BaseInfo: &ipbaseinfo.IPBaseInfo{
				IPV4: "203.0.113.10",
				IPV6: "2001:db8::10",
			},
		}, nil
	}

	err := detectorService.Detect(context.Background(), &IPDetectorReq{
		ProxyID: 1,
		Enabled: true,
		Refresh: false,
	})

	require.NoError(t, err)
	require.Len(t, proxyIPRepo.saved, 1)
	assert.Equal(t, uint(6), proxyIPRepo.saved[0].IPAddressesID)
	assert.Equal(t, uint(6), proxyIPRepo.saved[0].IPType)
}

func TestIPDetectorDetectRefreshFalseContinuesToFullDetectWhenNoKnownAddressExists(t *testing.T) {
	proxyIPRepo := &fakeDetectProxyIPRepo{}
	addressRepo := &fakeDetectAddressRepo{byIP: map[string]*model.IPAddress{}}
	detectorService := newDetectTestService()
	detectorService.ProxyIPAddress = proxyIPRepo
	detectorService.IPAddressRepo = addressRepo
	detectorService.Persister = newIPDetectPersister(addressRepo, proxyIPRepo, &fakeDetectBaseInfoRepo{}, &fakeDetectIPInfoRepo{}, &fakeDetectUnlockInfoRepo{})
	var calls []struct {
		ipInfoEnable    bool
		appUnlockEnable bool
	}
	detectorService.detectAll = func(ctx context.Context, proxy *model.IPProxy, ipInfoEnable bool, appUnlockEnable bool) (*detector.DetectionResult, error) {
		calls = append(calls, struct {
			ipInfoEnable    bool
			appUnlockEnable bool
		}{ipInfoEnable: ipInfoEnable, appUnlockEnable: appUnlockEnable})
		if len(calls) == 1 {
			return &detector.DetectionResult{
				BaseInfo: &ipbaseinfo.IPBaseInfo{IPV4: "203.0.113.10"},
			}, nil
		}
		return &detector.DetectionResult{
			BaseInfo: &ipbaseinfo.IPBaseInfo{IPV4: "198.51.100.20"},
		}, nil
	}

	err := detectorService.Detect(context.Background(), &IPDetectorReq{
		ProxyID:         1,
		Enabled:         true,
		Refresh:         false,
		IPInfoEnable:    true,
		APPUnlockEnable: true,
	})

	require.NoError(t, err)
	require.Len(t, calls, 2)
	assert.False(t, calls[0].ipInfoEnable)
	assert.False(t, calls[0].appUnlockEnable)
	assert.True(t, calls[1].ipInfoEnable)
	assert.True(t, calls[1].appUnlockEnable)
	require.Len(t, proxyIPRepo.saved, 1)
	require.Len(t, addressRepo.created, 1)
	assert.Equal(t, "198.51.100.20", addressRepo.created[0].IP)
}

func TestIPDetectorDetectReturnsRepositoryErrorBeforeDetection(t *testing.T) {
	detectorService := newDetectTestService()
	detectorService.ProxyIPAddress = &fakeDetectProxyIPRepo{findErr: errors.New("repo down")}
	called := false
	detectorService.detectAll = func(ctx context.Context, proxy *model.IPProxy, ipInfoEnable bool, appUnlockEnable bool) (*detector.DetectionResult, error) {
		called = true
		return nil, nil
	}

	err := detectorService.Detect(context.Background(), &IPDetectorReq{
		ProxyID: 1,
		Enabled: true,
		Refresh: false,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "repo down")
	assert.False(t, called)
}

func TestIPDetectorDetectSkipsMissingProxy(t *testing.T) {
	detectorService := newDetectTestService()
	detectorService.ProxyRepo = &fakeDetectProxyRepo{}
	called := false
	detectorService.detectAll = func(ctx context.Context, proxy *model.IPProxy, ipInfoEnable bool, appUnlockEnable bool) (*detector.DetectionResult, error) {
		called = true
		return nil, nil
	}

	err := detectorService.Detect(context.Background(), &IPDetectorReq{
		ProxyID: 404,
		Enabled: true,
		Refresh: true,
	})

	require.NoError(t, err)
	assert.False(t, called)
}

func TestIPDetectorBatchGetInfoUsesIPv4BaseInfoFirst(t *testing.T) {
	detectorService := newDetectTestService()
	detectorService.IPUnlockInfoRepo = &fakeDetectUnlockInfoRepo{
		byIPAddressID: map[uint][]*model.IPUnlockInfo{
			10: {{IPAddressesID: 10, AppName: "Netflix", Status: "fail"}},
			11: {{IPAddressesID: 11, AppName: "Netflix", Status: "unlock", Region: "US"}},
			20: {{IPAddressesID: 20, AppName: "OpenAI", Status: "forbidden"}},
		},
	}
	detectorService.ProxyIPAddress = &fakeDetectProxyIPRepo{
		latestRecords: []*model.ProxyIPAddress{
			{
				ProxyID: 1, IPAddressesID: 10, IPType: 6, Latest: true,
				IPAddress: model.IPAddress{ID: 10, IP: "2001:db8::1", IPType: 6, IPBaseInfo: model.IPBaseInfo{ID: 10, CountryCode: "JP", RiskLevel: "high"}},
			},
			{
				ProxyID: 1, IPAddressesID: 11, IPType: 4, Latest: true,
				IPAddress: model.IPAddress{ID: 11, IP: "203.0.113.1", IPType: 4, IPBaseInfo: model.IPBaseInfo{ID: 11, CountryCode: "US", RiskLevel: "low"}},
			},
			{
				ProxyID: 2, IPAddressesID: 20, IPType: 6, Latest: true,
				IPAddress: model.IPAddress{ID: 20, IP: "2001:db8::2", IPType: 6, IPBaseInfo: model.IPBaseInfo{ID: 20, CountryCode: "SG", RiskLevel: "medium"}},
			},
		},
	}

	result, err := detectorService.BatchGetInfo([]uint{1, 2, 3})

	require.NoError(t, err)
	require.Contains(t, result, uint(1))
	assert.Equal(t, "203.0.113.1", result[1].IPv4)
	assert.Equal(t, "2001:db8::1", result[1].IPv6)
	assert.Equal(t, "US", result[1].CountryCode)
	assert.Equal(t, "low", result[1].Risk)
	require.Len(t, result[1].AppUnlock, 1)
	assert.Equal(t, "Netflix", result[1].AppUnlock[0].AppName)
	assert.Equal(t, "unlock", result[1].AppUnlock[0].Status)
	require.Contains(t, result, uint(2))
	assert.Equal(t, "2001:db8::2", result[2].IPv6)
	assert.Equal(t, "SG", result[2].CountryCode)
	require.Len(t, result[2].AppUnlock, 1)
	assert.Equal(t, "OpenAI", result[2].AppUnlock[0].AppName)
	assert.NotContains(t, result, uint(3))
}

func TestIPDetectorBatchGetInfoAggregatesUnlockAcrossLatestIPs(t *testing.T) {
	detectorService := newDetectTestService()
	unlockRepo := &fakeDetectUnlockInfoRepo{
		byIPAddressID: map[uint][]*model.IPUnlockInfo{
			10: {
				{IPAddressesID: 10, AppName: "Netflix", Status: "fail"},
				{IPAddressesID: 10, AppName: "OpenAI", Status: "unlock"},
			},
			11: {
				{IPAddressesID: 11, AppName: "Netflix", Status: "unlock", Region: "US"},
				{IPAddressesID: 11, AppName: "Claude", Status: "rateLimit"},
			},
		},
	}
	detectorService.IPUnlockInfoRepo = unlockRepo
	detectorService.ProxyIPAddress = &fakeDetectProxyIPRepo{
		latestRecords: []*model.ProxyIPAddress{
			{ProxyID: 1, IPAddressesID: 10, IPType: 4, Latest: true, IPAddress: model.IPAddress{ID: 10, IP: "203.0.113.1", IPType: 4}},
			{ProxyID: 1, IPAddressesID: 11, IPType: 6, Latest: true, IPAddress: model.IPAddress{ID: 11, IP: "2001:db8::1", IPType: 6}},
		},
	}

	result, err := detectorService.BatchGetInfo([]uint{1})

	require.NoError(t, err)
	require.True(t, unlockRepo.batchCalled)
	assert.ElementsMatch(t, []uint{10, 11}, unlockRepo.batchIPAddressIDs)
	require.Len(t, result[1].AppUnlock, 3)
	assert.Equal(t, "Claude", result[1].AppUnlock[0].AppName)
	assert.Equal(t, "rateLimit", result[1].AppUnlock[0].Status)
	assert.Equal(t, "Netflix", result[1].AppUnlock[1].AppName)
	assert.Equal(t, "unlock", result[1].AppUnlock[1].Status)
	assert.Equal(t, "OpenAI", result[1].AppUnlock[2].AppName)
	assert.Equal(t, "unlock", result[1].AppUnlock[2].Status)
}

func TestAppUnlockStatusRankOrdersKnownStatuses(t *testing.T) {
	assert.Greater(t, appUnlockStatusRank("unlock"), appUnlockStatusRank("forbidden"))
	assert.Greater(t, appUnlockStatusRank("forbidden"), appUnlockStatusRank("rateLimit"))
	assert.Greater(t, appUnlockStatusRank("rateLimit"), appUnlockStatusRank("fail"))
	assert.Greater(t, appUnlockStatusRank("fail"), appUnlockStatusRank("unknown"))
}

func TestIPDetectorBatchGetInfoEmptyInputDoesNotQuery(t *testing.T) {
	repo := &fakeDetectProxyIPRepo{}
	detectorService := newDetectTestService()
	detectorService.ProxyIPAddress = repo

	result, err := detectorService.BatchGetInfo(nil)

	require.NoError(t, err)
	assert.Empty(t, result)
	assert.False(t, repo.latestCalled)
}

func newDetectTestService() ipDetectorImpl {
	proxyIPRepo := &fakeDetectProxyIPRepo{}
	addressRepo := &fakeDetectAddressRepo{byIP: map[string]*model.IPAddress{}}
	unlockRepo := &fakeDetectUnlockInfoRepo{}
	return ipDetectorImpl{
		ProxyRepo:        &fakeDetectProxyRepo{proxy: &model.Proxy{ID: 1, Type: model.ProxyTypeVMess}},
		ProxyIPAddress:   proxyIPRepo,
		IPAddressRepo:    addressRepo,
		IPUnlockInfoRepo: unlockRepo,
		TaskManager:      task.NewTaskManager(),
		Persister:        newIPDetectPersister(addressRepo, proxyIPRepo, &fakeDetectBaseInfoRepo{}, &fakeDetectIPInfoRepo{}, unlockRepo),
	}
}

type fakeDetectProxyRepo struct {
	repository.ProxyRepository
	proxy *model.Proxy
	err   error
}

func (r *fakeDetectProxyRepo) FindByID(id uint) (*model.Proxy, error) {
	return r.proxy, r.err
}

type fakeDetectProxyIPRepo struct {
	repository.ProxyIPAddressRepository
	records       []*model.ProxyIPAddress
	latestRecords []*model.ProxyIPAddress
	saved         []*model.ProxyIPAddress
	findErr       error
	latestCalled  bool
}

func (r *fakeDetectProxyIPRepo) FindByProxyID(proxyID uint) ([]*model.ProxyIPAddress, error) {
	return r.records, r.findErr
}

func (r *fakeDetectProxyIPRepo) FindLatestByProxyIDList(proxyIDList []uint) ([]*model.ProxyIPAddress, error) {
	r.latestCalled = true
	return r.latestRecords, r.findErr
}

func (r *fakeDetectProxyIPRepo) CreateOrUpdate(proxyIPAddress *model.ProxyIPAddress) error {
	r.saved = append(r.saved, proxyIPAddress)
	return nil
}

type fakeDetectAddressRepo struct {
	repository.IPAddressRepository
	byIP    map[string]*model.IPAddress
	created []*model.IPAddress
	nextID  uint
	findErr error
}

func (r *fakeDetectAddressRepo) FindByIP(ip string) (*model.IPAddress, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.byIP[ip], nil
}

func (r *fakeDetectAddressRepo) CreateOrIgnore(ipAddress *model.IPAddress) error {
	if existing := r.byIP[ipAddress.IP]; existing != nil {
		ipAddress.ID = existing.ID
		return nil
	}
	r.nextID++
	ipAddress.ID = r.nextID
	r.byIP[ipAddress.IP] = ipAddress
	r.created = append(r.created, ipAddress)
	return nil
}

type fakeDetectBaseInfoRepo struct {
	repository.IPBaseInfoRepository
}

type fakeDetectIPInfoRepo struct {
	repository.IPInfoRepository
}

type fakeDetectUnlockInfoRepo struct {
	repository.IPUnlockInfoRepository
	byIPAddressID     map[uint][]*model.IPUnlockInfo
	batchIPAddressIDs []uint
	batchCalled       bool
}

func (r *fakeDetectUnlockInfoRepo) FindByIPAddressIDs(ipAddressIDs []uint) ([]*model.IPUnlockInfo, error) {
	r.batchCalled = true
	r.batchIPAddressIDs = append([]uint(nil), ipAddressIDs...)
	result := make([]*model.IPUnlockInfo, 0)
	for _, ipAddressID := range ipAddressIDs {
		result = append(result, r.byIPAddressID[ipAddressID]...)
	}
	return result, nil
}

func TestIPDetectorBatchDetectAllowsConcurrentDifferentResourceIDs(t *testing.T) {
	taskManager := task.NewTaskManager()
	started1 := make(chan struct{})
	started2 := make(chan struct{})
	release := make(chan struct{})
	done1 := make(chan struct{})
	done2 := make(chan struct{})
	var calls atomic.Int32

	detectorService := ipDetectorImpl{
		TaskManager: taskManager,
		detectOne: func(ctx context.Context, req *IPDetectorReq) error {
			if calls.Add(1) == 1 {
				close(started1)
			} else {
				close(started2)
			}
			<-release
			return nil
		},
	}

	go func() {
		_ = detectorService.BatchDetect(context.Background(), &BatchIPDetectorReq{
			ProxyIDList:    []uint{1},
			Enabled:        true,
			Concurrent:     1,
			TaskResourceID: 1,
		})
		close(done1)
	}()
	go func() {
		_ = detectorService.BatchDetect(context.Background(), &BatchIPDetectorReq{
			ProxyIDList:    []uint{2},
			Enabled:        true,
			Concurrent:     1,
			TaskResourceID: 2,
		})
		close(done2)
	}()

	<-started1
	<-started2
	assert.Equal(t, int32(2), calls.Load())

	close(release)
	<-done1
	<-done2
}

func TestIPDetectorBatchDetectRejectsSameResourceID(t *testing.T) {
	taskManager := task.NewTaskManager()
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	var calls atomic.Int32

	detectorService := ipDetectorImpl{
		TaskManager: taskManager,
		detectOne: func(ctx context.Context, req *IPDetectorReq) error {
			if calls.Add(1) == 1 {
				close(started)
				<-release
			}
			return nil
		},
	}

	go func() {
		_ = detectorService.BatchDetect(context.Background(), &BatchIPDetectorReq{
			ProxyIDList:    []uint{1},
			Enabled:        true,
			Concurrent:     1,
			TaskResourceID: 100,
		})
		close(done)
	}()

	<-started

	_ = detectorService.BatchDetect(context.Background(), &BatchIPDetectorReq{
		ProxyIDList:    []uint{2},
		Enabled:        true,
		Concurrent:     1,
		TaskResourceID: 100,
	})

	assert.Equal(t, int32(1), calls.Load())

	close(release)
	<-done
}

func TestIPDetectorBatchDetectMutualExclusionGlobalVsResource(t *testing.T) {
	taskManager := task.NewTaskManager()

	t.Run("global running blocks resource", func(t *testing.T) {
		started := make(chan struct{})
		release := make(chan struct{})
		done := make(chan struct{})
		var calls atomic.Int32

		detectorService := ipDetectorImpl{
			TaskManager: taskManager,
			detectOne: func(ctx context.Context, req *IPDetectorReq) error {
				if calls.Add(1) == 1 {
					close(started)
					<-release
				}
				return nil
			},
		}

		go func() {
			_ = detectorService.BatchDetect(context.Background(), &BatchIPDetectorReq{
				ProxyIDList:    []uint{1},
				Enabled:        true,
				Concurrent:     1,
				TaskResourceID: 0,
			})
			close(done)
		}()

		<-started

		_ = detectorService.BatchDetect(context.Background(), &BatchIPDetectorReq{
			ProxyIDList:    []uint{2},
			Enabled:        true,
			Concurrent:     1,
			TaskResourceID: 200,
		})
		assert.Equal(t, int32(1), calls.Load())

		close(release)
		<-done
	})

	t.Run("resource running blocks global", func(t *testing.T) {
		started := make(chan struct{})
		release := make(chan struct{})
		done := make(chan struct{})
		var calls atomic.Int32

		detectorService := ipDetectorImpl{
			TaskManager: taskManager,
			detectOne: func(ctx context.Context, req *IPDetectorReq) error {
				if calls.Add(1) == 1 {
					close(started)
					<-release
				}
				return nil
			},
		}

		go func() {
			_ = detectorService.BatchDetect(context.Background(), &BatchIPDetectorReq{
				ProxyIDList:    []uint{1},
				Enabled:        true,
				Concurrent:     1,
				TaskResourceID: 300,
			})
			close(done)
		}()

		<-started

		_ = detectorService.BatchDetect(context.Background(), &BatchIPDetectorReq{
			ProxyIDList:    []uint{2},
			Enabled:        true,
			Concurrent:     1,
			TaskResourceID: 0,
		})
		assert.Equal(t, int32(1), calls.Load())

		close(release)
		<-done
	})
}
